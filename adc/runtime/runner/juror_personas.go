package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/common/councilsample"
	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	"github.com/agentcourt/adj/common/persona"
)

type jurorPersonaPair struct {
	Model       string
	PersonaText string
	PersonaFile string
	RequestSpec *modelrequest.Spec
}

type jurorPersonaPool struct {
	pairs     []jurorPersonaPair
	selector  *councilsample.Selector
	available map[int]bool
}

func (p *jurorPersonaPool) findPair(model string, personaFile string) (jurorPersonaPair, bool) {
	if p == nil {
		return jurorPersonaPair{}, false
	}
	model = strings.TrimSpace(model)
	personaFile = strings.TrimSpace(personaFile)
	for _, pair := range p.pairs {
		if strings.TrimSpace(pair.Model) == model && strings.TrimSpace(pair.PersonaFile) == personaFile {
			return pair, true
		}
	}
	return jurorPersonaPair{}, false
}

func loadJurorPersonaPool(path string, scenarioBaseDir string) (*jurorPersonaPool, error) {
	return loadJurorPersonaPoolWithOptions(path, scenarioBaseDir, councilsample.Options{})
}

func loadJurorPersonaPoolWithOptions(path string, scenarioBaseDir string, opts councilsample.Options) (*jurorPersonaPool, error) {
	resolvedPairsPath := resolveScenarioRelativePath(path, scenarioBaseDir)
	raw, err := os.ReadFile(resolvedPairsPath)
	if err != nil {
		return nil, fmt.Errorf("read juror personas file: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	pairs := make([]jurorPersonaPair, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		spec, err := persona.ParseRecord(line, filepath.Dir(resolvedPairsPath))
		if err != nil {
			return nil, err
		}
		var requestSpec *modelrequest.Spec
		if spec.RequestSpec != nil {
			copied := *spec.RequestSpec
			requestSpec = &copied
		} else {
			return nil, fmt.Errorf("juror persona record must be a JSONL request spec: %s", line)
		}
		pairs = append(pairs, jurorPersonaPair{
			Model:       spec.Model,
			PersonaText: spec.Text,
			PersonaFile: spec.File,
			RequestSpec: requestSpec,
		})
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("juror personas file contains no usable pairs: %s", path)
	}
	endpoints := make([]string, len(pairs))
	for index, pair := range pairs {
		endpoints[index] = pair.RequestSpec.Endpoint
	}
	selector, err := councilsample.New(endpoints, opts)
	if err != nil {
		return nil, err
	}
	return &jurorPersonaPool{pairs: pairs, selector: selector, available: map[int]bool{}}, nil
}

func (p *jurorPersonaPool) samplePair() (jurorPersonaPair, error) {
	return p.sampleAvailablePair(context.Background(), nil)
}

func (p *jurorPersonaPool) sampleAvailablePair(ctx context.Context, check func(context.Context, jurorPersonaPair) error) (jurorPersonaPair, error) {
	if p == nil || len(p.pairs) == 0 {
		return jurorPersonaPair{}, fmt.Errorf("juror persona pool is empty")
	}
	if p.selector == nil {
		endpoints := make([]string, len(p.pairs))
		for index, pair := range p.pairs {
			if pair.RequestSpec != nil {
				endpoints[index] = pair.RequestSpec.Endpoint
			}
			if strings.TrimSpace(endpoints[index]) == "" {
				model, err := modelrequest.ParseModelRef(pair.Model)
				if err != nil {
					return jurorPersonaPair{}, fmt.Errorf("juror persona pair %d: %w", index+1, err)
				}
				endpoints[index] = model.Endpoint
			}
		}
		selector, err := councilsample.New(endpoints, councilsample.Options{})
		if err != nil {
			return jurorPersonaPair{}, err
		}
		p.selector = selector
	}
	if p.available == nil {
		p.available = map[int]bool{}
	}
	var lastErr error
	for {
		pairIndex, err := p.selector.Draw()
		if err != nil {
			if lastErr != nil {
				return jurorPersonaPair{}, fmt.Errorf("sample available juror persona pair: %w", errors.Join(lastErr, err))
			}
			return jurorPersonaPair{}, fmt.Errorf("sample juror persona pair: %w", err)
		}
		pair := p.pairs[pairIndex]
		if check != nil && !p.available[pairIndex] {
			if err := check(ctx, pair); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return jurorPersonaPair{}, ctxErr
				}
				if errors.Is(err, context.Canceled) {
					return jurorPersonaPair{}, err
				}
				lastErr = err
				if endpoint, ok := modelgateway.CredentialFailureEndpoint(err); ok {
					if rejectErr := p.selector.RejectEndpoint(endpoint); rejectErr != nil {
						return jurorPersonaPair{}, errors.Join(lastErr, rejectErr)
					}
				} else if rejectErr := p.selector.Reject(pairIndex); rejectErr != nil {
					return jurorPersonaPair{}, errors.Join(lastErr, rejectErr)
				}
				continue
			}
			p.available[pairIndex] = true
		}
		if err := p.selector.Accept(pairIndex); err != nil {
			return jurorPersonaPair{}, err
		}
		return pair, nil
	}
}

func (r *Runner) prepareActionPayload(ctx context.Context, actionType string, payload map[string]any) (map[string]any, error) {
	payload = applyActionEventTime(actionType, payload, time.Now().UTC().Format(time.RFC3339), false)
	if actionType != "add_juror" {
		return payload, nil
	}
	return r.applyJurorPersonaDefaultsContext(ctx, payload)
}

func (r *Runner) applyJurorPersonaDefaults(payload map[string]any) (map[string]any, error) {
	return r.applyJurorPersonaDefaultsContext(context.Background(), payload)
}

func (r *Runner) applyJurorPersonaDefaultsContext(ctx context.Context, payload map[string]any) (map[string]any, error) {
	cloned := clonePayload(payload)
	jurorID := strings.TrimSpace(stringOrDefault(cloned["juror_id"], ""))
	if jurorID == "" {
		return cloned, nil
	}
	if pair, ok := r.jurorPersonaAssignments[jurorID]; ok {
		if strings.TrimSpace(stringOrDefault(cloned["model"], "")) == "" && strings.TrimSpace(pair.Model) != "" {
			cloned["model"] = pair.Model
		}
		if strings.TrimSpace(stringOrDefault(cloned["persona_filename"], "")) == "" && strings.TrimSpace(pair.PersonaFile) != "" {
			cloned["persona_filename"] = pair.PersonaFile
		}
		return cloned, nil
	}
	if r.jurorPersonaPool == nil {
		return cloned, nil
	}
	hasModel := strings.TrimSpace(stringOrDefault(cloned["model"], "")) != ""
	hasPersona := strings.TrimSpace(stringOrDefault(cloned["persona_filename"], "")) != ""
	if hasModel && hasPersona {
		if pair, ok := r.jurorPersonaPool.findPair(stringOrDefault(cloned["model"], ""), stringOrDefault(cloned["persona_filename"], "")); ok {
			r.jurorPersonaAssignments[jurorID] = pair
		}
		return cloned, nil
	}
	var check func(context.Context, jurorPersonaPair) error
	if r.jurorClient != nil {
		check = r.preflightJurorPair
	}
	pair, err := r.jurorPersonaPool.sampleAvailablePair(ctx, check)
	if err != nil {
		return nil, err
	}
	cloned["model"] = pair.Model
	cloned["persona_filename"] = pair.PersonaFile
	r.jurorPersonaAssignments[jurorID] = pair
	return cloned, nil
}

const jurorPreflightMaxOutputTokens int64 = 1024

func (r *Runner) preflightJurorPair(ctx context.Context, pair jurorPersonaPair) error {
	if pair.RequestSpec == nil {
		return fmt.Errorf("juror persona pair has no request specification")
	}
	identity, err := r.promptRenderer.RenderJurorProbeIdentity(pair.PersonaText)
	if err != nil {
		return err
	}
	toolCheck, err := r.promptRenderer.JurorProbeToolCheck()
	if err != nil {
		return err
	}
	request, err := r.prompts.Text(adcprompts.ProbeJurorPreflightID)
	if err != nil {
		return err
	}
	tools, err := r.promptRenderer.BuildTools([]string{"submit_juror_vote"})
	if err != nil {
		return err
	}
	spec := pair.RequestSpec.WithFallbackMaxOutputTokens(jurorPreflightMaxOutputTokens)
	response, err := r.jurorClient.CreateResponseWithRequestSpec(ctx, spec, []map[string]any{
		{"role": "system", "content": identity},
		{"role": "system", "content": toolCheck},
		{"role": "user", "content": request},
	}, tools, "")
	if err != nil {
		return err
	}
	if len(response.ToolCalls) != 1 {
		return fmt.Errorf("juror preflight returned %d tool calls; expected one", len(response.ToolCalls))
	}
	call := response.ToolCalls[0]
	if call.Name != "submit_juror_vote" {
		return fmt.Errorf("juror preflight called %q; expected submit_juror_vote", call.Name)
	}
	if call.ArgumentsError != "" {
		return fmt.Errorf("juror preflight returned malformed tool arguments: %s", call.ArgumentsError)
	}
	for _, field := range []string{"juror_id", "vote", "damages", "confidence", "explanation"} {
		if _, ok := call.Arguments[field]; !ok {
			return fmt.Errorf("juror preflight omitted %s", field)
		}
	}
	return nil
}

var jurorIDPattern = regexp.MustCompile(`\bJ(\d+)\b`)

func opportunityConstraintString(opportunity leanOpportunity, key string) string {
	required, _ := opportunity.Constraints["required_payload"].(map[string]any)
	if required != nil {
		if value := strings.TrimSpace(stringOrDefault(required[key], "")); value != "" {
			return value
		}
	}
	defaults, _ := opportunity.Constraints["payload_defaults"].(map[string]any)
	if defaults != nil {
		if value := strings.TrimSpace(stringOrDefault(defaults[key], "")); value != "" {
			return value
		}
	}
	return ""
}

func countCandidateJurors(state map[string]any) int {
	caseObj, _ := state["case"].(map[string]any)
	if caseObj == nil {
		return 0
	}
	jurors, _ := caseObj["jurors"].([]any)
	count := 0
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if juror == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(juror["status"], "")) == "candidate" {
			count++
		}
	}
	return count
}

func nextJurorNumber(state map[string]any) int {
	caseObj, _ := state["case"].(map[string]any)
	if caseObj == nil {
		return 1
	}
	jurors, _ := caseObj["jurors"].([]any)
	maxSeen := 0
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if juror == nil {
			continue
		}
		jurorID := strings.TrimSpace(stringOrDefault(juror["juror_id"], ""))
		match := jurorIDPattern.FindStringSubmatch(jurorID)
		if len(match) != 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if n > maxSeen {
			maxSeen = n
		}
	}
	if maxSeen == 0 {
		return 1
	}
	return maxSeen + 1
}

func targetJurorIDForOpportunity(opportunity leanOpportunity) string {
	if jurorID := opportunityConstraintString(opportunity, "juror_id"); jurorID != "" {
		return jurorID
	}
	match := jurorIDPattern.FindStringSubmatch(opportunity.ActorMessage)
	if len(match) != 2 {
		return ""
	}
	return "J" + match[1]
}

func (r *Runner) jurorOpportunityPromptContext(opportunity leanOpportunity) (string, string) {
	jurorID := targetJurorIDForOpportunity(opportunity)
	if jurorID == "" {
		return "", ""
	}
	pair, ok := r.jurorPersonaAssignments[jurorID]
	if !ok {
		return "", ""
	}
	model := strings.TrimSpace(pair.Model)
	context := strings.TrimSpace(pair.PersonaText)
	if context == "" {
		return model, ""
	}
	return model, context
}

func (r *Runner) jurorOpportunityRequestSpec(opportunity leanOpportunity) *modelrequest.Spec {
	jurorID := targetJurorIDForOpportunity(opportunity)
	if jurorID == "" {
		return nil
	}
	pair, ok := r.jurorPersonaAssignments[jurorID]
	if !ok || pair.RequestSpec == nil {
		return nil
	}
	spec := *pair.RequestSpec
	return &spec
}

func (r *Runner) jurorResponseClient(model string) (ResponseClient, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		if r.client == nil {
			return nil, fmt.Errorf("llm client is nil")
		}
		return r.client, nil
	}
	if r.jurorClient == nil {
		if r.client == nil {
			return nil, fmt.Errorf("llm client is nil")
		}
		return r.client, nil
	}
	return r.jurorClient, nil
}
