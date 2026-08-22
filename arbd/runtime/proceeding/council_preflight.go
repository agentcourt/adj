package proceeding

import (
	"context"
	"crypto/rand"
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
}

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
	if NormalizeCouncilBackend(cfg.CouncilBackend) == councilBackendAPI {
		return preflightCouncilCandidates(ctx, candidates, cfg.Policy.CouncilSize, func(context.Context, CouncilSeat) error {
			return nil
		})
	}
	check := func(ctx context.Context, seat CouncilSeat) error {
		return checkCouncilSeatAvailableWithConfig(ctx, cfg, client, seat)
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
			Status:      "seated",
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
		memberID := fmt.Sprintf("C%d", i+1)
		failed := make([]councilPreflightReplacement, 0)
		for len(candidates) > 0 {
			candidate := candidates[0]
			candidates = candidates[1:]
			candidate.MemberID = memberID
			candidate.Status = "seated"
			if err := check(ctx, candidate); err != nil {
				failed = append(failed, councilPreflightReplacement{
					MemberID:               memberID,
					UnavailableModel:       candidate.Model,
					UnavailablePersonaFile: candidate.PersonaFile,
					Cause:                  err.Error(),
				})
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
			last := failed[len(failed)-1]
			return nil, nil, fmt.Errorf("council preflight could not seat %s after %d unavailable candidate(s); last unavailable model %s from %s: %s", memberID, len(failed), last.UnavailableModel, last.UnavailablePersonaFile, last.Cause)
		}
	}
	return seated, replacements, nil
}

func checkCouncilSeatAvailable(ctx context.Context, limits RuntimeLimits, client councilResponseClient, seat CouncilSeat) error {
	return checkCouncilSeatAvailableWithConfig(ctx, Config{Runtime: limits}, client, seat)
}

func checkCouncilSeatAvailableWithConfig(ctx context.Context, cfg Config, client councilResponseClient, seat CouncilSeat) error {
	ctx, cancel := withTimeout(ctx, councilPreflightTimeout(cfg.Runtime))
	defer cancel()
	inputItems, err := cfg.councilPreflightInput(seat)
	if err != nil {
		return err
	}
	maxOutputTokens := int64(16)
	_, err = createCouncilAvailabilityResponse(
		ctx,
		client,
		seat,
		inputItems,
		&maxOutputTokens,
	)
	if err != nil {
		return err
	}
	return nil
}

func (cfg Config) councilPreflightInput(seat CouncilSeat) ([]map[string]any, error) {
	values := map[string]string{
		"MEMBER_ID":    seat.MemberID,
		"MODEL":        seat.Model,
		"PERSONA_FILE": seat.PersonaFile,
	}
	systemPrompt, err := cfg.renderPromptFile(promptCouncilPreflightSystem, values)
	if err != nil {
		return nil, err
	}
	userPrompt, err := cfg.renderPromptFile(promptCouncilPreflightUser, values)
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}, nil
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
