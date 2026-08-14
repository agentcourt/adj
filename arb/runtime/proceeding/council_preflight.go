package proceeding

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/common/persona"
)

type councilResponseClient interface {
	CreateResponseWithRequestSpec(
		ctx context.Context,
		spec modelrequest.Spec,
		inputItems []map[string]any,
		tools []map[string]any,
		previousResponseID string,
	) (openaiapi.Response, error)
}

type councilPreflightReplacement struct {
	MemberID               string
	UnavailableModel       string
	UnavailablePersonaFile string
	ReplacementModel       string
	ReplacementPersonaFile string
	Cause                  string
	err                    error
}

type CouncilPreflightError struct {
	MemberID                   string
	UnavailableCandidates      int
	LastUnavailableModel       string
	LastUnavailablePersonaFile string
	Cause                      string
	Err                        error
}

func (e *CouncilPreflightError) Error() string {
	return fmt.Sprintf(
		"council preflight could not seat %s after %d unavailable candidate(s); last unavailable model %s from %s: %s",
		e.MemberID,
		e.UnavailableCandidates,
		e.LastUnavailableModel,
		e.LastUnavailablePersonaFile,
		e.Cause,
	)
}

func (e *CouncilPreflightError) Unwrap() error { return e.Err }

func sampleAvailableCouncil(ctx context.Context, cfg Config, client councilResponseClient) ([]CouncilSeat, []councilPreflightReplacement, error) {
	specs, err := councilPoolMeta(cfg.CouncilPoolPath, cfg.CommonRoot)
	if err != nil {
		return nil, nil, err
	}
	if cfg.Policy.CouncilSize <= 0 {
		return nil, nil, fmt.Errorf("council size must be positive")
	}
	if cfg.Policy.CouncilSize > len(specs) {
		return nil, nil, fmt.Errorf("council size %d exceeds available pool %d", cfg.Policy.CouncilSize, len(specs))
	}
	candidates, err := shuffledCouncilCandidates(specs)
	if err != nil {
		return nil, nil, err
	}
	check := func(ctx context.Context, seat CouncilSeat) error {
		return checkCouncilSeatAvailable(ctx, cfg.Runtime, client, seat)
	}
	return preflightCouncilCandidates(ctx, candidates, cfg.Policy.CouncilSize, check)
}

func shuffledCouncilCandidates(specs []persona.Spec) ([]CouncilSeat, error) {
	indexes := make([]int, len(specs))
	for i := range specs {
		indexes[i] = i
	}
	out := make([]CouncilSeat, 0, len(specs))
	for len(indexes) > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(indexes))))
		if err != nil {
			return nil, fmt.Errorf("sample council pool: %w", err)
		}
		pick := int(n.Int64())
		spec := specs[indexes[pick]]
		indexes = append(indexes[:pick], indexes[pick+1:]...)
		out = append(out, CouncilSeat{
			Model:       spec.Model,
			PersonaFile: spec.File,
			RequestSpec: spec.RequestSpec,
			PersonaText: spec.Text,
		})
	}
	return out, nil
}

func preflightCouncilCandidates(
	ctx context.Context,
	candidates []CouncilSeat,
	count int,
	check func(context.Context, CouncilSeat) error,
) ([]CouncilSeat, []councilPreflightReplacement, error) {
	if count <= 0 {
		return nil, nil, fmt.Errorf("council size must be positive")
	}
	if count > len(candidates) {
		return nil, nil, fmt.Errorf("council size %d exceeds available pool %d", count, len(candidates))
	}
	candidates = append([]CouncilSeat(nil), candidates...)
	seated := make([]CouncilSeat, 0, count)
	replacements := make([]councilPreflightReplacement, 0)
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		memberID := fmt.Sprintf("C%d", i+1)
		failed := make([]councilPreflightReplacement, 0)
		for len(candidates) > 0 {
			candidate := candidates[0]
			candidates = candidates[1:]
			candidate.MemberID = memberID
			if err := check(ctx, candidate); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return nil, nil, ctxErr
				}
				if errors.Is(err, context.Canceled) {
					return nil, nil, err
				}
				failed = append(failed, councilPreflightReplacement{
					MemberID:               memberID,
					UnavailableModel:       candidate.Model,
					UnavailablePersonaFile: candidate.PersonaFile,
					Cause:                  err.Error(),
					err:                    err,
				})
				if openaiapi.ErrorClass(err) == openaiapi.ProviderErrorAuthentication {
					return nil, nil, newCouncilPreflightError(memberID, failed)
				}
				continue
			}
			seated = append(seated, candidate)
			for _, replacement := range failed {
				replacement.ReplacementModel = candidate.Model
				replacement.ReplacementPersonaFile = candidate.PersonaFile
				replacements = append(replacements, replacement)
			}
			break
		}
		if len(seated) != i+1 {
			if len(failed) == 0 {
				return nil, nil, fmt.Errorf("council preflight could not seat %s: no candidates remained", memberID)
			}
			return nil, nil, newCouncilPreflightError(memberID, failed)
		}
	}
	return seated, replacements, nil
}

func newCouncilPreflightError(memberID string, failed []councilPreflightReplacement) *CouncilPreflightError {
	last := failed[len(failed)-1]
	return &CouncilPreflightError{
		MemberID:                   memberID,
		UnavailableCandidates:      len(failed),
		LastUnavailableModel:       last.UnavailableModel,
		LastUnavailablePersonaFile: last.UnavailablePersonaFile,
		Cause:                      last.Cause,
		Err:                        last.err,
	}
}

func checkCouncilSeatAvailable(ctx context.Context, limits RuntimeLimits, client councilResponseClient, seat CouncilSeat) error {
	ctx, cancel := withTimeout(ctx, councilPreflightTimeout(limits))
	defer cancel()
	maxOutputTokens := int64(16)
	_, err := createCouncilAvailabilityResponse(
		ctx,
		client,
		seat,
		[]map[string]any{
			{"role": "system", "content": "You are being checked for availability as an Agent Arbitration council member. Reply with the exact word ready."},
			{"role": "user", "content": "Availability check. Reply ready."},
		},
		&maxOutputTokens,
	)
	if err != nil {
		return err
	}
	return nil
}

func createCouncilAvailabilityResponse(
	ctx context.Context,
	client councilResponseClient,
	seat CouncilSeat,
	inputItems []map[string]any,
	maxOutputTokens *int64,
) (openaiapi.Response, error) {
	if seat.RequestSpec == nil {
		return openaiapi.Response{}, fmt.Errorf("council member %s has no request_spec; JSONL council pool records are required", seat.MemberID)
	}
	spec := seat.RequestSpec.WithFallbackMaxOutputTokens(0)
	if maxOutputTokens != nil {
		spec = spec.WithFallbackMaxOutputTokens(*maxOutputTokens)
	}
	return client.CreateResponseWithRequestSpec(ctx, spec, inputItems, nil, "")
}

func councilPreflightTimeout(limits RuntimeLimits) time.Duration {
	timeout := limits.CouncilRequestTimeout()
	if timeout <= 0 || timeout > 20*time.Second {
		return 20 * time.Second
	}
	return timeout
}
