package proceeding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsmorph/adj/arbd/runtime/lean"
)

const (
	ReplayCertificateSchemaVersion = "aard.replay-certificate.v1"
	ReplayCertificateFileName      = "certificate.json"
)

var errTurnDeadlineExceeded = errors.New("opportunity deadline expired during engine call")

type ReplayCertificate struct {
	SchemaVersion           string                  `json:"schema_version"`
	Procedure               string                  `json:"procedure"`
	Engine                  []string                `json:"engine"`
	CaseID                  string                  `json:"case_id"`
	RunID                   string                  `json:"run_id,omitempty"`
	InitializeRequest       ReplayInitializeRequest `json:"initialize_request"`
	Actions                 []ReplayAction          `json:"actions"`
	ClaimedFinalState       map[string]any          `json:"claimed_final_state"`
	ClaimedFinalStateSHA256 string                  `json:"claimed_final_state_sha256"`
}

type ReplayInitializeRequest struct {
	State          map[string]any   `json:"state"`
	Question       string           `json:"question"`
	CouncilMembers []map[string]any `json:"council_members"`
}

type OpportunityAuthority = lean.OpportunityAuthority

type ReplayAction struct {
	ActionType string               `json:"action_type"`
	ActorRole  string               `json:"actor_role"`
	Authority  OpportunityAuthority `json:"authority"`
	Payload    map[string]any       `json:"payload"`
}

type VerifyReplayCertificateOptions struct {
	CertificatePath string
	StatePath       string
	Engine          lean.Engine
	EngineTimeout   time.Duration
}

type VerifyReplayCertificateResult struct {
	Status                  string `json:"status"`
	CaseID                  string `json:"case_id"`
	RunID                   string `json:"run_id,omitempty"`
	ActionCount             int    `json:"action_count"`
	ClaimedFinalStateSHA256 string `json:"claimed_final_state_sha256"`
}

func newReplayInitializeRequest(state map[string]any, question string, councilMembers []map[string]any) (ReplayInitializeRequest, error) {
	stateCopy, err := cloneMapJSON(state)
	if err != nil {
		return ReplayInitializeRequest{}, fmt.Errorf("clone certificate initial state: %w", err)
	}
	membersCopy, err := cloneMapListJSON(councilMembers)
	if err != nil {
		return ReplayInitializeRequest{}, fmt.Errorf("clone certificate council members: %w", err)
	}
	return ReplayInitializeRequest{
		State:          stateCopy,
		Question:       question,
		CouncilMembers: membersCopy,
	}, nil
}

func (rc *runContext) evaluateStep(ctx context.Context, opportunity Opportunity, actionType string, actorRole string, payload map[string]any) (map[string]any, ReplayAction, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	engineTimeout := rc.cfg.Runtime.EngineCallTimeout()
	if engineTimeout <= 0 {
		return nil, ReplayAction{}, fmt.Errorf("runtime.engine_call_timeout_seconds must be positive")
	}
	authority, err := authorityForOpportunity(rc.state, opportunity)
	if err != nil {
		return nil, ReplayAction{}, err
	}
	payloadCopy, err := cloneMapJSON(payload)
	if err != nil {
		return nil, ReplayAction{}, fmt.Errorf("clone certificate action payload: %w", err)
	}
	stepCtx, cancel := context.WithTimeout(ctx, engineTimeout)
	defer cancel()
	stepResp, err := rc.cfg.Engine.Step(stepCtx, rc.state, actionType, actorRole, authority, payload)
	if err != nil {
		return nil, ReplayAction{}, err
	}
	return stepResp, ReplayAction{
		ActionType: actionType,
		ActorRole:  actorRole,
		Authority:  authority,
		Payload:    payloadCopy,
	}, nil
}

func turnStepContext(caseCtx context.Context, turnDeadline time.Time, engineTimeout time.Duration) (context.Context, context.CancelFunc) {
	if engineTimeout <= 0 {
		return context.WithCancel(caseCtx)
	}
	engineDeadline := time.Now().Add(engineTimeout)
	if turnDeadline.IsZero() || engineDeadline.Before(turnDeadline) {
		return context.WithDeadlineCause(caseCtx, engineDeadline, context.DeadlineExceeded)
	}
	return context.WithDeadlineCause(caseCtx, turnDeadline, errTurnDeadlineExceeded)
}

func turnStepError(caseCtx context.Context, turnDeadline time.Time, stepErr error) error {
	if cause := context.Cause(caseCtx); cause != nil {
		if stepErr == nil || !errors.Is(stepErr, cause) {
			return errors.Join(cause, stepErr)
		}
		return stepErr
	}
	if !turnDeadline.IsZero() && !time.Now().Before(turnDeadline) {
		if stepErr == nil || !errors.Is(stepErr, errTurnDeadlineExceeded) {
			return errors.Join(errTurnDeadlineExceeded, stepErr)
		}
		return stepErr
	}
	return stepErr
}

func turnEngineStepError(caseCtx context.Context, stepCtx context.Context, turnDeadline time.Time, stepErr error) error {
	if cause := context.Cause(caseCtx); cause != nil {
		if stepErr == nil || !errors.Is(stepErr, cause) {
			return errors.Join(cause, stepErr)
		}
		return stepErr
	}
	if cause := context.Cause(stepCtx); cause != nil {
		if stepErr == nil || !errors.Is(stepErr, cause) {
			return errors.Join(cause, stepErr)
		}
		return stepErr
	}
	return turnStepError(caseCtx, turnDeadline, stepErr)
}

func (rc *runContext) evaluateTurnStepLocked(caseCtx context.Context, turnDeadline time.Time, opportunity Opportunity, actionType string, actorRole string, payload map[string]any) (map[string]any, ReplayAction, error) {
	stepCtx, cancel := turnStepContext(caseCtx, turnDeadline, rc.cfg.Runtime.EngineCallTimeout())
	defer cancel()
	stepResp, replayAction, err := rc.evaluateStep(stepCtx, opportunity, actionType, actorRole, payload)
	if err := turnEngineStepError(caseCtx, stepCtx, turnDeadline, err); err != nil {
		return nil, ReplayAction{}, err
	}
	return stepResp, replayAction, nil
}

func authorityForOpportunity(state map[string]any, opportunity Opportunity) (OpportunityAuthority, error) {
	stateVersion, err := requiredStateVersion(state)
	if err != nil {
		return OpportunityAuthority{}, fmt.Errorf("read action authority state_version: %w", err)
	}
	if opportunity.StateVersion != stateVersion {
		return OpportunityAuthority{}, fmt.Errorf("stale opportunity state_version=%d current=%d", opportunity.StateVersion, stateVersion)
	}
	authority := OpportunityAuthority{
		OpportunityID:        opportunity.ID,
		ExpectedStateVersion: opportunity.StateVersion,
		Role:                 opportunity.Role,
		Phase:                opportunity.Phase,
		MemberID:             opportunity.MemberID,
	}
	if err := validateOpportunityAuthority(authority); err != nil {
		return OpportunityAuthority{}, fmt.Errorf("construct action authority: %w", err)
	}
	return authority, nil
}

func validateOpportunityAuthority(authority OpportunityAuthority) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "opportunity_id", value: authority.OpportunityID},
		{name: "role", value: authority.Role},
		{name: "phase", value: authority.Phase},
	}
	for _, field := range fields {
		name, value := field.name, field.value
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if strings.TrimSpace(value) != value {
			return fmt.Errorf("%s must not contain surrounding whitespace", name)
		}
	}
	if authority.ExpectedStateVersion < 0 {
		return fmt.Errorf("expected_state_version must be nonnegative")
	}
	if strings.TrimSpace(authority.MemberID) != authority.MemberID {
		return fmt.Errorf("member_id must not contain surrounding whitespace")
	}
	if authority.Role == "council" && authority.MemberID == "" {
		return fmt.Errorf("member_id is required for council authority")
	}
	if authority.Role != "council" && authority.MemberID != "" {
		return fmt.Errorf("member_id must be empty for %s authority", authority.Role)
	}
	return nil
}

func writeReplayCertificate(cfg Config, result Result, initialize ReplayInitializeRequest, actions []ReplayAction) error {
	cert, err := replayCertificate(cfg, initialize, actions, result.FinalState)
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(cfg.OutputDir, ReplayCertificateFileName), cert)
}

func replayCertificate(cfg Config, initialize ReplayInitializeRequest, actions []ReplayAction, finalState map[string]any) (ReplayCertificate, error) {
	finalStateCopy, err := cloneMapJSON(finalState)
	if err != nil {
		return ReplayCertificate{}, fmt.Errorf("clone certificate final state: %w", err)
	}
	finalStateHash, err := canonicalJSONSHA256(finalStateCopy)
	if err != nil {
		return ReplayCertificate{}, fmt.Errorf("hash certificate final state: %w", err)
	}
	actions = append([]ReplayAction(nil), actions...)
	return ReplayCertificate{
		SchemaVersion:           ReplayCertificateSchemaVersion,
		Procedure:               "aard",
		Engine:                  append([]string(nil), cfg.Engine.Command...),
		CaseID:                  cfg.CaseID,
		RunID:                   cfg.RunID,
		InitializeRequest:       initialize,
		Actions:                 actions,
		ClaimedFinalState:       finalStateCopy,
		ClaimedFinalStateSHA256: finalStateHash,
	}, nil
}

func VerifyReplayCertificate(ctx context.Context, opts VerifyReplayCertificateOptions) (VerifyReplayCertificateResult, error) {
	if opts.CertificatePath == "" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate path is required")
	}
	if opts.StatePath == "" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("state path is required")
	}
	if len(opts.Engine.Command) == 0 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("lean engine command is required")
	}
	if opts.EngineTimeout <= 0 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("engine timeout must be positive")
	}
	var cert ReplayCertificate
	if err := readJSON(opts.CertificatePath, &cert); err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	if cert.SchemaVersion != ReplayCertificateSchemaVersion {
		return VerifyReplayCertificateResult{}, fmt.Errorf("unsupported certificate schema_version %q", cert.SchemaVersion)
	}
	if cert.Procedure != "aard" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("unsupported certificate procedure %q", cert.Procedure)
	}
	if strings.TrimSpace(cert.CaseID) == "" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate case_id is required")
	}
	if cert.InitializeRequest.State == nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate initialize_request.state is required")
	}
	if cert.InitializeRequest.CouncilMembers == nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate initialize_request.council_members is required")
	}
	if cert.ClaimedFinalState == nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("certificate claimed_final_state is required")
	}
	if err := requireCertificateCaseID(cert.CaseID, cert.InitializeRequest.State, "certificate initialize_request.state"); err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	if err := requireCertificateCaseID(cert.CaseID, cert.ClaimedFinalState, "certificate claimed_final_state"); err != nil {
		return VerifyReplayCertificateResult{}, err
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
	if err := requireCertificateCaseID(cert.CaseID, packetState, "packet state.json"); err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	packetHash, err := canonicalJSONSHA256(packetState)
	if err != nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("hash packet state: %w", err)
	}
	if packetHash != cert.ClaimedFinalStateSHA256 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("packet final state mismatch: state.json hash %s, certificate %s", packetHash, cert.ClaimedFinalStateSHA256)
	}
	replayedState, err := replayCertificateActions(ctx, opts.Engine, opts.EngineTimeout, cert)
	if err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	if err := requireCertificateCaseID(cert.CaseID, replayedState, "replayed state"); err != nil {
		return VerifyReplayCertificateResult{}, err
	}
	replayedHash, err := canonicalJSONSHA256(replayedState)
	if err != nil {
		return VerifyReplayCertificateResult{}, fmt.Errorf("hash replayed final state: %w", err)
	}
	if replayedHash != cert.ClaimedFinalStateSHA256 {
		return VerifyReplayCertificateResult{}, fmt.Errorf("replayed final state mismatch: replay hash %s, certificate %s", replayedHash, cert.ClaimedFinalStateSHA256)
	}
	status := mapString(mapAny(replayedState["case"])["status"])
	if status != "closed" && status != "failed" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("replayed final state case.status %q is not terminal; want closed or failed", status)
	}
	return VerifyReplayCertificateResult{
		Status:                  "ok",
		CaseID:                  cert.CaseID,
		RunID:                   cert.RunID,
		ActionCount:             len(cert.Actions),
		ClaimedFinalStateSHA256: cert.ClaimedFinalStateSHA256,
	}, nil
}

func requireCertificateCaseID(caseID string, state map[string]any, label string) error {
	stateCaseID := mapString(mapAny(state["case"])["case_id"])
	if strings.TrimSpace(stateCaseID) == "" {
		return fmt.Errorf("%s case.case_id is required", label)
	}
	if stateCaseID != caseID {
		return fmt.Errorf("%s case.case_id %q does not match certificate case_id %q", label, stateCaseID, caseID)
	}
	return nil
}

func replayCertificateActions(ctx context.Context, engine lean.Engine, engineTimeout time.Duration, cert ReplayCertificate) (map[string]any, error) {
	initCtx, cancel := context.WithTimeout(ctx, engineTimeout)
	initResp, err := engine.InitializeCase(initCtx, cert.InitializeRequest.State, cert.InitializeRequest.Question, cert.InitializeRequest.CouncilMembers)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("initialize_case failed: %w", err)
	}
	if ok, _ := initResp["ok"].(bool); !ok {
		return nil, fmt.Errorf("initialize_case rejected: %s", mapString(initResp["error"]))
	}
	state := mapAny(initResp["state"])
	if len(state) == 0 {
		return nil, fmt.Errorf("initialize_case returned empty state")
	}
	for i, action := range cert.Actions {
		if action.ActionType == "" {
			return nil, fmt.Errorf("certificate action %d has empty action_type", i+1)
		}
		if action.ActorRole == "" {
			return nil, fmt.Errorf("certificate action %d (%s) has empty actor_role", i+1, action.ActionType)
		}
		if err := validateOpportunityAuthority(action.Authority); err != nil {
			return nil, fmt.Errorf("certificate action %d (%s) has invalid authority: %w", i+1, action.ActionType, err)
		}
		stateVersion, err := requiredStateVersion(state)
		if err != nil {
			return nil, fmt.Errorf("certificate action %d (%s) state: %w", i+1, action.ActionType, err)
		}
		if action.Authority.ExpectedStateVersion != stateVersion {
			return nil, fmt.Errorf("certificate action %d (%s) authority expected_state_version %d does not match replay state_version %d", i+1, action.ActionType, action.Authority.ExpectedStateVersion, stateVersion)
		}
		payload := action.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		if action.Authority.Role == "council" && actionUsesCouncilMember(action.ActionType) && mapString(payload["member_id"]) != action.Authority.MemberID {
			return nil, fmt.Errorf("certificate action %d (%s) payload member_id %q does not match authority member_id %q", i+1, action.ActionType, mapString(payload["member_id"]), action.Authority.MemberID)
		}
		stepCtx, cancel := context.WithTimeout(ctx, engineTimeout)
		stepResp, err := engine.Step(stepCtx, state, action.ActionType, action.ActorRole, action.Authority, payload)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("certificate action %d (%s) failed: %w", i+1, action.ActionType, err)
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			return nil, fmt.Errorf("certificate action %d (%s) rejected: %s", i+1, action.ActionType, mapString(stepResp["error"]))
		}
		state = mapAny(stepResp["state"])
		if len(state) == 0 {
			return nil, fmt.Errorf("certificate action %d (%s) returned empty state", i+1, action.ActionType)
		}
	}
	return state, nil
}

func actionUsesCouncilMember(actionType string) bool {
	switch actionType {
	case "submit_council_answer", "remove_council_member", "fail_opportunity":
		return true
	default:
		return false
	}
}

func canonicalJSONSHA256(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
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
	return out, nil
}
