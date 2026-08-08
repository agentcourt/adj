package proceeding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/jsmorph/adj/arb/runtime/lean"
)

const (
	ReplayCertificateSchemaVersion = "aar.replay-certificate.v0"
	ReplayCertificateFileName      = "certificate.json"
)

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
	Proposition    string           `json:"proposition"`
	CouncilMembers []map[string]any `json:"council_members"`
}

type ReplayAction struct {
	ActionType string         `json:"action_type"`
	ActorRole  string         `json:"actor_role"`
	Payload    map[string]any `json:"payload"`
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
	ActionCount             int    `json:"action_count"`
	ClaimedFinalStateSHA256 string `json:"claimed_final_state_sha256"`
}

func newReplayInitializeRequest(state map[string]any, proposition string, councilMembers []map[string]any) (ReplayInitializeRequest, error) {
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
		Proposition:    proposition,
		CouncilMembers: membersCopy,
	}, nil
}

func (rc *runContext) stepForCertificate(actionType string, actorRole string, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	stepResp, err := rc.cfg.Engine.Step(rc.state, actionType, actorRole, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); ok {
		payloadCopy, err := cloneMapJSON(payload)
		if err != nil {
			return nil, fmt.Errorf("clone certificate action payload: %w", err)
		}
		rc.certificateActions = append(rc.certificateActions, ReplayAction{
			ActionType: actionType,
			ActorRole:  actorRole,
			Payload:    payloadCopy,
		})
	}
	return stepResp, nil
}

func writeReplayCertificate(cfg Config, result Result, rc *runContext) error {
	cert, err := rc.replayCertificate(result.FinalState)
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(cfg.OutputDir, ReplayCertificateFileName), cert)
}

func (rc *runContext) replayCertificate(finalState map[string]any) (ReplayCertificate, error) {
	finalStateCopy, err := cloneMapJSON(finalState)
	if err != nil {
		return ReplayCertificate{}, fmt.Errorf("clone certificate final state: %w", err)
	}
	finalStateHash, err := canonicalJSONSHA256(finalStateCopy)
	if err != nil {
		return ReplayCertificate{}, fmt.Errorf("hash certificate final state: %w", err)
	}
	actions := make([]ReplayAction, len(rc.certificateActions))
	copy(actions, rc.certificateActions)
	return ReplayCertificate{
		SchemaVersion:           ReplayCertificateSchemaVersion,
		Procedure:               "aar",
		Engine:                  append([]string(nil), rc.cfg.Engine.Command...),
		CaseID:                  rc.cfg.CaseID,
		RunID:                   rc.cfg.RunID,
		InitializeRequest:       rc.certificateInit,
		Actions:                 actions,
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
	if cert.Procedure != "aar" {
		return VerifyReplayCertificateResult{}, fmt.Errorf("unsupported certificate procedure %q", cert.Procedure)
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
	replayedState, err := replayCertificateActions(opts.Engine, cert)
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
		ActionCount:             len(cert.Actions),
		ClaimedFinalStateSHA256: cert.ClaimedFinalStateSHA256,
	}, nil
}

func replayCertificateActions(engine lean.Engine, cert ReplayCertificate) (map[string]any, error) {
	initResp, err := engine.InitializeCase(cert.InitializeRequest.State, cert.InitializeRequest.Proposition, cert.InitializeRequest.CouncilMembers)
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
		payload := action.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		stepResp, err := engine.Step(state, action.ActionType, action.ActorRole, payload)
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
