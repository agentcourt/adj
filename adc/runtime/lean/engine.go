package lean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
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
	return e.CallContext(context.Background(), request)
}

func (e Engine) CallContext(ctx context.Context, request map[string]any) (map[string]any, error) {
	if len(e.Command) == 0 {
		return nil, fmt.Errorf("lean command is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	wire, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	cmd := exec.CommandContext(ctx, e.Command[0], e.Command[1:]...)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Stdin = bytes.NewReader(wire)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("lean process failed: %w stderr=%s", ctxErr, bytes.TrimSpace(stderr.Bytes()))
		}
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
	return e.StepContext(context.Background(), state, actionType, actorRole, payload)
}

func (e Engine) StepContext(ctx context.Context, state map[string]any, actionType, actorRole string, payload map[string]any) (map[string]any, error) {
	return e.CallContext(ctx, map[string]any{
		"state": state,
		"action": map[string]any{
			"action_type": actionType,
			"actor_role":  actorRole,
			"payload":     payload,
		},
	})
}

func (e Engine) View(state map[string]any, role string) (map[string]any, error) {
	return e.ViewContext(context.Background(), state, role)
}

func (e Engine) ViewContext(ctx context.Context, state map[string]any, role string) (map[string]any, error) {
	return e.CallContext(ctx, map[string]any{
		"request_type": "role_view",
		"state":        state,
		"role":         role,
	})
}

func (e Engine) NextOpportunity(state map[string]any, roles []map[string]any, maxStepsPerTurn int) (map[string]any, error) {
	return e.NextOpportunityContext(context.Background(), state, roles, maxStepsPerTurn)
}

func (e Engine) NextOpportunityContext(ctx context.Context, state map[string]any, roles []map[string]any, maxStepsPerTurn int) (map[string]any, error) {
	return e.CallContext(ctx, map[string]any{
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
	return e.ApplyDecisionContext(context.Background(), state, stateVersion, opportunityID, role, decision, roles, maxStepsPerTurn)
}

func (e Engine) ApplyDecisionContext(
	ctx context.Context,
	state map[string]any,
	stateVersion int,
	opportunityID string,
	role string,
	decision map[string]any,
	roles []map[string]any,
	maxStepsPerTurn int,
) (map[string]any, error) {
	return e.CallContext(ctx, map[string]any{
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
	return e.InitializeCaseContext(context.Background(), state, complaintSummary, filedBy, juryDemandedOn, jurisdictionalAllegations, attachments)
}

func (e Engine) InitializeCaseContext(
	ctx context.Context,
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
	return e.CallContext(ctx, request)
}
