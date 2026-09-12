package proceeding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/councilsample"
	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
	"github.com/agentcourt/adj/common/persona"
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
	candidates := councilCandidates(specs)
	if NormalizeCouncilBackend(cfg.CouncilBackend) == councilBackendAPI {
		return preflightCouncilCandidatesWithOptions(ctx, candidates, councilsample.Options{
			Count:                    cfg.Policy.CouncilSize,
			AllowedEndpoints:         cfg.CouncilAllowedEndpoints,
			MinimumDistinctEndpoints: cfg.CouncilMinEndpoints,
		}, func(context.Context, CouncilSeat) error {
			return nil
		})
	}
	check := func(ctx context.Context, seat CouncilSeat) error {
		return checkCouncilSeatAvailableWithConfig(ctx, cfg, client, seat)
	}
	return preflightCouncilCandidatesWithOptions(ctx, candidates, councilsample.Options{
		Count:                    cfg.Policy.CouncilSize,
		AllowedEndpoints:         cfg.CouncilAllowedEndpoints,
		MinimumDistinctEndpoints: cfg.CouncilMinEndpoints,
	}, check)
}

func councilCandidates(specs []persona.Spec) []CouncilSeat {
	out := make([]CouncilSeat, 0, len(specs))
	for _, spec := range specs {
		out = append(out, CouncilSeat{
			Model:       spec.Model,
			PersonaFile: spec.File,
			Status:      "seated",
			RequestSpec: spec.RequestSpec,
			PersonaText: spec.Text,
		})
	}
	return out
}

func preflightCouncilCandidates(
	ctx context.Context,
	candidates []CouncilSeat,
	count int,
	check func(context.Context, CouncilSeat) error,
) ([]CouncilSeat, []councilPreflightReplacement, error) {
	return preflightCouncilCandidatesWithOptions(ctx, candidates, councilsample.Options{Count: count}, check)
}

func preflightCouncilCandidatesWithOptions(
	ctx context.Context,
	candidates []CouncilSeat,
	opts councilsample.Options,
	check func(context.Context, CouncilSeat) error,
) ([]CouncilSeat, []councilPreflightReplacement, error) {
	endpoints := make([]string, len(candidates))
	for index, candidate := range candidates {
		endpoints[index] = councilSeatEndpoint(candidate)
	}
	selector, err := councilsample.New(endpoints, opts)
	if err != nil {
		return nil, nil, err
	}
	seated := make([]CouncilSeat, 0, opts.Count)
	replacements := make([]councilPreflightReplacement, 0)
	available := make(map[int]bool)
	for i := 0; i < opts.Count; i++ {
		memberID := fmt.Sprintf("C%d", i+1)
		failed := make([]councilPreflightReplacement, 0)
		for {
			candidateIndex, drawErr := selector.Draw()
			if drawErr != nil {
				if len(failed) == 0 {
					return nil, nil, fmt.Errorf("council preflight could not seat %s: %w", memberID, drawErr)
				}
				last := failed[len(failed)-1]
				return nil, nil, fmt.Errorf("council preflight could not seat %s after %d unavailable candidate(s); last unavailable model %s from %s: %w", memberID, len(failed), last.UnavailableModel, last.UnavailablePersonaFile, drawErr)
			}
			candidate := candidates[candidateIndex]
			candidate.MemberID = memberID
			candidate.Status = "seated"
			if available[candidateIndex] {
				if err := selector.Accept(candidateIndex); err != nil {
					return nil, nil, err
				}
				seated = append(seated, candidate)
				for _, replacement := range failed {
					replacement.ReplacementModel = candidate.Model
					replacement.ReplacementPersonaFile = candidate.PersonaFile
					replacements = append(replacements, replacement)
				}
				break
			}
			if err := check(ctx, candidate); err != nil {
				failed = append(failed, councilPreflightReplacement{
					MemberID:               memberID,
					UnavailableModel:       candidate.Model,
					UnavailablePersonaFile: candidate.PersonaFile,
					Cause:                  err.Error(),
				})
				if failedEndpoint, ok := modelgateway.CredentialFailureEndpoint(err); ok {
					if rejectErr := selector.RejectEndpoint(failedEndpoint); rejectErr != nil {
						return nil, nil, rejectErr
					}
				} else if rejectErr := selector.Reject(candidateIndex); rejectErr != nil {
					return nil, nil, rejectErr
				}
				continue
			}
			available[candidateIndex] = true
			if err := selector.Accept(candidateIndex); err != nil {
				return nil, nil, err
			}
			seated = append(seated, candidate)
			for _, replacement := range failed {
				replacement.ReplacementModel = candidate.Model
				replacement.ReplacementPersonaFile = candidate.PersonaFile
				replacements = append(replacements, replacement)
			}
			break
		}
	}
	if err := selector.Validate(); err != nil {
		return nil, nil, err
	}
	return seated, replacements, nil
}

func councilSeatEndpoint(seat CouncilSeat) string {
	if seat.RequestSpec != nil {
		if endpoint := strings.TrimSpace(seat.RequestSpec.Endpoint); endpoint != "" {
			return endpoint
		}
	}
	model, err := modelrequest.ParseModelRef(seat.Model)
	if err == nil {
		return model.Endpoint
	}
	return seat.Model
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
