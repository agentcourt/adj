package councilsample

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

type Options struct {
	Count                    int
	AllowedEndpoints         []string
	MinimumDistinctEndpoints int
}

type Selector struct {
	endpoints      []string
	eligible       []bool
	seats          map[string]int
	candidateSeats []int
	minimum        int
	count          int
	accepted       int
	current        int
	currentPending bool
}

func New(endpoints []string, opts Options) (*Selector, error) {
	if opts.Count < 0 {
		return nil, fmt.Errorf("council size must not be negative")
	}
	if opts.MinimumDistinctEndpoints < 0 {
		return nil, fmt.Errorf("minimum distinct council endpoints must not be negative")
	}
	if opts.Count > 0 && opts.MinimumDistinctEndpoints > opts.Count {
		return nil, fmt.Errorf("minimum distinct council endpoints %d exceeds council size %d", opts.MinimumDistinctEndpoints, opts.Count)
	}
	allowed := make(map[string]struct{}, len(opts.AllowedEndpoints))
	for _, raw := range opts.AllowedEndpoints {
		endpoint := normalizeEndpoint(raw)
		if endpoint == "" {
			return nil, fmt.Errorf("allowed council endpoint must not be empty")
		}
		allowed[endpoint] = struct{}{}
	}
	selector := &Selector{
		endpoints:      append([]string(nil), endpoints...),
		eligible:       make([]bool, len(endpoints)),
		seats:          make(map[string]int),
		candidateSeats: make([]int, len(endpoints)),
		minimum:        opts.MinimumDistinctEndpoints,
		count:          opts.Count,
	}
	available := make(map[string]struct{})
	for index, raw := range selector.endpoints {
		endpoint := normalizeEndpoint(raw)
		if endpoint == "" {
			return nil, fmt.Errorf("council candidate %d has no endpoint", index+1)
		}
		selector.endpoints[index] = endpoint
		if len(allowed) > 0 {
			if _, ok := allowed[endpoint]; !ok {
				continue
			}
		}
		selector.eligible[index] = true
		available[endpoint] = struct{}{}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("council pool contains no configurations for the allowed endpoints")
	}
	if selector.minimum > len(available) {
		return nil, fmt.Errorf("minimum distinct council endpoints %d exceeds available endpoint count %d", selector.minimum, len(available))
	}
	return selector, nil
}

func (s *Selector) Draw() (int, error) {
	if s == nil {
		return 0, fmt.Errorf("council selector is nil")
	}
	if s.currentPending {
		return 0, fmt.Errorf("council selection must be accepted or rejected before drawing again")
	}
	if s.count > 0 && s.accepted >= s.count {
		return 0, fmt.Errorf("council already contains %d members", s.count)
	}
	byEndpoint := make(map[string][]int)
	minimumSeats := -1
	for index, eligible := range s.eligible {
		if !eligible {
			continue
		}
		endpoint := s.endpoints[index]
		seatCount := s.seats[endpoint]
		if minimumSeats < 0 || seatCount < minimumSeats {
			minimumSeats = seatCount
		}
		byEndpoint[endpoint] = append(byEndpoint[endpoint], index)
	}
	if len(byEndpoint) == 0 {
		return 0, fmt.Errorf("no council configurations remain")
	}
	leastUsed := make([]string, 0, len(byEndpoint))
	for endpoint := range byEndpoint {
		if s.seats[endpoint] == minimumSeats {
			leastUsed = append(leastUsed, endpoint)
		}
	}
	endpointIndex, err := randomIndex(len(leastUsed))
	if err != nil {
		return 0, err
	}
	candidates := byEndpoint[leastUsed[endpointIndex]]
	minimumCandidateSeats := -1
	leastUsedCandidates := make([]int, 0, len(candidates))
	for _, index := range candidates {
		count := s.candidateSeats[index]
		if minimumCandidateSeats < 0 || count < minimumCandidateSeats {
			minimumCandidateSeats = count
			leastUsedCandidates = leastUsedCandidates[:0]
		}
		if count == minimumCandidateSeats {
			leastUsedCandidates = append(leastUsedCandidates, index)
		}
	}
	candidateIndex, err := randomIndex(len(leastUsedCandidates))
	if err != nil {
		return 0, err
	}
	s.current = leastUsedCandidates[candidateIndex]
	s.currentPending = true
	return s.current, nil
}

func (s *Selector) Accept(index int) error {
	if err := s.checkCurrent(index); err != nil {
		return err
	}
	s.seats[s.endpoints[index]]++
	s.candidateSeats[index]++
	s.accepted++
	s.currentPending = false
	return nil
}

func (s *Selector) Reject(index int) error {
	if err := s.checkCurrent(index); err != nil {
		return err
	}
	s.eligible[index] = false
	s.currentPending = false
	return nil
}

func (s *Selector) RejectEndpoint(endpoint string) error {
	if s == nil {
		return fmt.Errorf("council selector is nil")
	}
	endpoint = normalizeEndpoint(endpoint)
	if endpoint == "" {
		return fmt.Errorf("rejected council endpoint must not be empty")
	}
	for index, candidateEndpoint := range s.endpoints {
		if candidateEndpoint == endpoint {
			s.eligible[index] = false
		}
	}
	s.currentPending = false
	return nil
}

func (s *Selector) Validate() error {
	if s == nil {
		return fmt.Errorf("council selector is nil")
	}
	if s.currentPending {
		return fmt.Errorf("council selection has not been accepted or rejected")
	}
	if s.count > 0 && s.accepted != s.count {
		return fmt.Errorf("council contains %d of %d required members", s.accepted, s.count)
	}
	distinct := 0
	for _, count := range s.seats {
		if count > 0 {
			distinct++
		}
	}
	if distinct < s.minimum {
		return fmt.Errorf("council uses %d distinct endpoints; at least %d required", distinct, s.minimum)
	}
	return nil
}

func (s *Selector) checkCurrent(index int) error {
	if s == nil {
		return fmt.Errorf("council selector is nil")
	}
	if !s.currentPending {
		return fmt.Errorf("no council selection is pending")
	}
	if index != s.current {
		return fmt.Errorf("council candidate %d is pending, not %d", s.current, index)
	}
	return nil
}

func normalizeEndpoint(endpoint string) string {
	return strings.ToLower(strings.TrimSpace(endpoint))
}

func randomIndex(upperBound int) (int, error) {
	if upperBound <= 0 {
		return 0, fmt.Errorf("random index upper bound must be positive")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(upperBound)))
	if err != nil {
		return 0, fmt.Errorf("read cryptographic randomness: %w", err)
	}
	return int(value.Int64()), nil
}
