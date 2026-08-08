package lean

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Engine struct {
	Command []string
}

func New(command []string) Engine {
	if len(command) == 0 {
		command = []string{"lake", "exe", "adcengine"}
	}
	return Engine{Command: command}
}

func (e Engine) Call(request map[string]any) (map[string]any, error) {
	if len(e.Command) == 0 {
		return nil, fmt.Errorf("lean command is empty")
	}
	wire, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	cmd := exec.Command(e.Command[0], e.Command[1:]...)
	cmd.Stdin = bytes.NewReader(wire)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("lean process failed: %w stderr=%s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return nil, fmt.Errorf("lean returned empty response")
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse lean json: %w", err)
	}
	return out, nil
}

func (e Engine) Step(state map[string]any, actionType, actorRole string, payload map[string]any) (map[string]any, error) {
	return e.Call(map[string]any{
		"state": state,
		"action": map[string]any{
			"action_type": actionType,
			"actor_role":  actorRole,
			"payload":     payload,
		},
	})
}

func (e Engine) View(state map[string]any, role string) (map[string]any, error) {
	return e.Call(map[string]any{
		"request_type": "role_view",
		"state":        state,
		"role":         role,
	})
}

func (e Engine) NextOpportunity(state map[string]any, roles []map[string]any, maxStepsPerTurn int) (map[string]any, error) {
	return e.Call(map[string]any{
		"request_type":       "next_opportunity",
		"state":              state,
		"roles":              roles,
		"max_steps_per_turn": maxStepsPerTurn,
	})
}

func (e Engine) ApplyDecision(
	state map[string]any,
	stateVersion int,
	opportunityID string,
	role string,
	decision map[string]any,
	roles []map[string]any,
	maxStepsPerTurn int,
) (map[string]any, error) {
	return e.Call(map[string]any{
		"request_type":       "apply_decision",
		"state":              state,
		"state_version":      stateVersion,
		"opportunity_id":     opportunityID,
		"role":               role,
		"decision":           decision,
		"roles":              roles,
		"max_steps_per_turn": maxStepsPerTurn,
	})
}

func (e Engine) InitializeCase(
	state map[string]any,
	complaintSummary, filedBy, juryDemandedOn string,
	jurisdictionalAllegations map[string]any,
	attachments []map[string]any,
) (map[string]any, error) {
	request := map[string]any{
		"request_type":               "initialize_case",
		"state":                      state,
		"complaint_summary":          complaintSummary,
		"filed_by":                   filedBy,
		"jurisdictional_allegations": jurisdictionalAllegations,
		"attachments":                attachments,
	}
	if juryDemandedOn != "" {
		request["jury_demanded_on"] = juryDemandedOn
	}
	return e.Call(request)
}
