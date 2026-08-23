package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentcourt/adj/adc/runtime/lean"
)

const (
	ReplayCertificateSchemaVersion = "adc.replay-certificate.v0"
	ReplayCertificateFileName      = "certificate.json"
)

type ReplayCertificate struct {
	SchemaVersion           string                  `json:"schema_version"`
	Procedure               string                  `json:"procedure"`
	Engine                  []string                `json:"engine"`
	CaseID                  string                  `json:"case_id"`
	RunID                   string                  `json:"run_id,omitempty"`
	InitializeRequest       ReplayInitializeRequest `json:"initialize_request"`
	Transitions             []ReplayTransition      `json:"transitions"`
	ClaimedFinalState       map[string]any          `json:"claimed_final_state"`
	ClaimedFinalStateSHA256 string                  `json:"claimed_final_state_sha256"`
}

type ReplayInitializeRequest struct {
	State          map[string]any               `json:"state"`
	InitializeCase *ReplayInitializeCaseRequest `json:"initialize_case,omitempty"`
}

type ReplayInitializeCaseRequest struct {
	ComplaintSummary          string           `json:"complaint_summary"`
	FiledBy                   string           `json:"filed_by"`
	JuryDemandedOn            string           `json:"jury_demanded_on,omitempty"`
	JurisdictionalAllegations map[string]any   `json:"jurisdictional_allegations"`
	Attachments               []map[string]any `json:"attachments"`
}

type ReplayTransition struct {
	Kind          string                         `json:"kind"`
	Step          *ReplayStepTransition          `json:"step,omitempty"`
	ApplyDecision *ReplayApplyDecisionTransition `json:"apply_decision,omitempty"`
}

type ReplayStepTransition struct {
	ActionType string         `json:"action_type"`
	ActorRole  string         `json:"actor_role"`
	Payload    map[string]any `json:"payload"`
}

type ReplayApplyDecisionTransition struct {
	StateVersion    int              `json:"state_version"`
	OpportunityID   string           `json:"opportunity_id"`
	Role            string           `json:"role"`
	Decision        map[string]any   `json:"decision"`
	Roles           []map[string]any `json:"roles"`
	MaxStepsPerTurn int              `json:"max_steps_per_turn"`
}

type VerifyReplayCertificateOptions struct {
	CertificatePath string
	StatePath       string
	Engine          lean.Engine
}

type VerifyReplayCertificateResult struct {
	Status                  string `json:"status"`
	CaseID                  string `json:"case_id"`
	RunID                   string `json:"run_id,omitempty"`
	TransitionCount         int    `json:"transition_count"`
	ClaimedFinalStateSHA256 string `json:"claimed_final_state_sha256"`
}

func newReplayInitializeRequest(state map[string]any) (ReplayInitializeRequest, error) {
	stateCopy, err := cloneMapJSON(state)
	if err != nil {
		return ReplayInitializeRequest{}, fmt.Errorf("clone certificate initial state: %w", err)
	}
	return ReplayInitializeRequest{State: stateCopy}, nil
}

func (r *Runner) stepForCertificate(actionType string, actorRole string, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	resp, err := r.lean.Step(r.state, actionType, actorRole, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := resp["ok"].(bool); ok {
		payloadCopy, err := cloneMapJSON(payload)
		if err != nil {
			return nil, fmt.Errorf("clone certificate step payload: %w", err)
		}
		r.certificateTransitions = append(r.certificateTransitions, ReplayTransition{
			Kind: "step",
			Step: &ReplayStepTransition{
				ActionType: actionType,
				ActorRole:  actorRole,
				Payload:    payloadCopy,
			},
		})
	}
	return resp, nil
}

func (r *Runner) recordApplyDecisionForCertificate(
	stateVersion int,
	opportunityID string,
	role string,
	decision map[string]any,
	roles []map[string]any,
	maxStepsPerTurn int,
) error {
	decisionCopy, err := cloneMapJSON(decision)
	if err != nil {
		return fmt.Errorf("clone certificate decision: %w", err)
	}
	rolesCopy, err := cloneMapListJSON(roles)
	if err != nil {
		return fmt.Errorf("clone certificate roles: %w", err)
	}
	r.certificateTransitions = append(r.certificateTransitions, ReplayTransition{
		Kind: "apply_decision",
		ApplyDecision: &ReplayApplyDecisionTransition{
			StateVersion:    stateVersion,
			OpportunityID:   opportunityID,
			Role:            role,
			Decision:        decisionCopy,
			Roles:           rolesCopy,
			MaxStepsPerTurn: maxStepsPerTurn,
		},
	})
	return nil
}

func (r *Runner) writeReplayCertificate(result Result) error {
	if r.cfg.OutputPath == "" {
		return nil
	}
	cert, err := r.replayCertificate(result.FinalState)
	if err != nil {
		return err
	}
	return writeJSONFileAtomic(filepath.Join(filepath.Dir(r.cfg.OutputPath), ReplayCertificateFileName), cert)
}

func (r *Runner) replayCertificate(finalState map[string]any) (ReplayCertificate, error) {
	if r.certificateInit.State == nil {
		return ReplayCertificate{}, fmt.Errorf("certificate initialize state is missing")
	}
	finalStateCopy, err := cloneMapJSON(finalState)
	if err != nil {
		return ReplayCertificate{}, fmt.Errorf("clone certificate final state: %w", err)
	}
	finalStateHash, err := canonicalJSONSHA256(finalStateCopy)
	if err != nil {
		return ReplayCertificate{}, fmt.Errorf("hash certificate final state: %w", err)
	}
	transitions := make([]ReplayTransition, len(r.certificateTransitions))
	copy(transitions, r.certificateTransitions)
	return ReplayCertificate{
		SchemaVersion:           ReplayCertificateSchemaVersion,
		Procedure:               "adc",
		Engine:                  append([]string(nil), r.lean.Command...),
		CaseID:                  r.cfg.CaseID,
		RunID:                   r.cfg.RunID,
		InitializeRequest:       r.certificateInit,
		Transitions:             transitions,
		ClaimedFinalState:       finalStateCopy,
		ClaimedFinalStateSHA256: finalStateHash,
	}, nil
}

func VerifyReplayCertificate(opts VerifyReplayCertificateOptions) (VerifyReplayCertificateResult, error) {
	if opts.CertificatePath == "" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate path is required")
	}
	if opts.StatePath == "" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("state path is required")
	}
	if len(opts.Engine.Command) == 0 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("lean engine command is required")
	}
	var cert ReplayCertificate
	if err := readJSON(opts.CertificatePath, &cert); err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	if cert.SchemaVersion != ReplayCertificateSchemaVersion {
		return VerifyReplayCertificateResult{}, fmt.Errorf("unsupported certificate schema_version %q", cert.SchemaVersion)
	}
	if cert.Procedure != "adc" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("unsupported certificate procedure %q", cert.Procedure)
	}
	if cert.InitializeRequest.State == nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate initialize_request.state is required")
	}
	if cert.ClaimedFinalState == nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate claimed_final_state is required")
	}
	claimedHash, err := canonicalJSONSHA256(cert.ClaimedFinalState)
	if err != nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("hash certificate claimed_final_state: %w", err)
	}
	if claimedHash != cert.ClaimedFinalStateSHA256 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate final state hash mismatch: claimed %s, computed %s", cert.ClaimedFinalStateSHA256, claimedHash)
	}
	var packetState map[string]any
	if err := readJSON(opts.StatePath, &packetState); err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	packetHash, err := canonicalJSONSHA256(packetState)
	if err != nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("hash packet state: %w", err)
	}
	if packetHash != cert.ClaimedFinalStateSHA256 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("packet final state mismatch: state.json hash %s, certificate %s", packetHash, cert.ClaimedFinalStateSHA256)
	}
	replayedState, err := replayCertificateTransitions(opts.Engine, cert)
	if err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	replayedHash, err := canonicalJSONSHA256(replayedState)
	if err != nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("hash replayed final state: %w", err)
	}
	if replayedHash != cert.ClaimedFinalStateSHA256 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("replayed final state mismatch: replay hash %s, certificate %s", replayedHash, cert.ClaimedFinalStateSHA256)
	}
	return VerifyReplayCertificateResult{
		Status:                  "ok",
		CaseID:                  cert.CaseID,
		RunID:                   cert.RunID,
		TransitionCount:         len(cert.Transitions),
		ClaimedFinalStateSHA256: cert.ClaimedFinalStateSHA256,
	}, nil
}

func replayCertificateTransitions(engine lean.Engine, cert ReplayCertificate) (map[string]any, error) {
	state := cert.InitializeRequest.State
	if cert.InitializeRequest.InitializeCase != nil {
		init := cert.InitializeRequest.InitializeCase
		resp, err := engine.InitializeCase(
			state,
			init.ComplaintSummary,
			init.FiledBy,
			init.JuryDemandedOn,
			init.JurisdictionalAllegations,
			init.Attachments,
		)
		if err != nil {
			return nil, fmt.Errorf("initialize_case failed: %w", err)
		}
		if ok, _ := resp["ok"].(bool); !ok {
			return nil, fmt.Errorf("initialize_case rejected: %s", stringFromAny(resp["error"]))
		}
		state = mapFromAny(resp["state"])
		if len(state) == 0 {
			return nil, fmt.Errorf("initialize_case returned empty state")
		}
	}
	for i, transition := range cert.Transitions {
		switch transition.Kind {
		case "step":
			if transition.Step == nil {
				return nil, fmt.Errorf("certificate transition %d step is missing", i+1)
			}
			next, err := replayStepTransition(engine, state, i+1, *transition.Step)
			if err != nil {
				return nil, err
			}
			state = next
		case "apply_decision":
			if transition.ApplyDecision == nil {
				return nil, fmt.Errorf("certificate transition %d apply_decision is missing", i+1)
			}
			next, err := replayApplyDecisionTransition(engine, state, i+1, *transition.ApplyDecision)
			if err != nil {
				return nil, err
			}
			state = next
		default:
			return nil, fmt.Errorf("certificate transition %d has unsupported kind %q", i+1, transition.Kind)
		}
	}
	return state, nil
}

func replayStepTransition(engine lean.Engine, state map[string]any, index int, step ReplayStepTransition) (map[string]any, error) {
	if step.ActionType == "" {
		return nil, fmt.Errorf("certificate transition %d step has empty action_type", index)
	}
	if step.ActorRole == "" {
		return nil, fmt.Errorf("certificate transition %d step has empty actor_role", index)
	}
	payload := step.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	resp, err := engine.Step(state, step.ActionType, step.ActorRole, payload)
	if err != nil {
		return nil, fmt.Errorf("certificate transition %d step %s failed: %w", index, step.ActionType, err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return nil, fmt.Errorf("certificate transition %d step %s rejected: %s", index, step.ActionType, stringFromAny(resp["error"]))
	}
	next := mapFromAny(resp["state"])
	if len(next) == 0 {
		return nil, fmt.Errorf("certificate transition %d step %s returned empty state", index, step.ActionType)
	}
	return next, nil
}

func replayApplyDecisionTransition(engine lean.Engine, state map[string]any, index int, transition ReplayApplyDecisionTransition) (map[string]any, error) {
	resp, err := engine.ApplyDecision(
		state,
		transition.StateVersion,
		transition.OpportunityID,
		transition.Role,
		transition.Decision,
		transition.Roles,
		transition.MaxStepsPerTurn,
	)
	if err != nil {
		return nil, fmt.Errorf("certificate transition %d apply_decision failed: %w", index, err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return nil, fmt.Errorf("certificate transition %d apply_decision rejected: %s", index, stringFromAny(resp["error"]))
	}
	resultKind := stringFromAny(resp["result_kind"])
	if resultKind != "pass_recorded" {
		return nil, fmt.Errorf("certificate transition %d apply_decision returned unsupported result_kind: %s", index, resultKind)
	}
	next := mapFromAny(resp["state"])
	if len(next) == 0 {
		return nil, fmt.Errorf("certificate transition %d apply_decision returned empty state", index)
	}
	return next, nil
}

func canonicalJSONSHA256(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func cloneMapJSON(in map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func cloneMapListJSON(in []map[string]any) ([]map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func mapFromAny(value any) map[string]any {
	out, _ := value.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
