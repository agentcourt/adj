package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/spec"
)

type leanOpportunity struct {
	OpportunityID       string
	Role                string
	Phase               string
	Kind                string
	MayPass             bool
	ActorMessage        string
	Objective           string
	AllowedTools        []string
	StepBudget          int
	Constraints         map[string]any
	DeterministicAction *spec.DeterministicAction
}

func (r *Runner) runAutopilot(ctx context.Context, startTurnIndex int) ([]TurnLog, error) {
	loop := r.scenario.LoopPolicy
	if loop == nil {
		return nil, fmt.Errorf("autopilot requires loop_policy")
	}
	if strings.TrimSpace(loop.Type) != "autopilot_trial" {
		return nil, fmt.Errorf("unsupported loop_policy.type: %s", loop.Type)
	}
	rolesPayload := r.autopilotRolesPayload()
	logs := make([]TurnLog, 0, loop.MaxTurns)
	for autoIndex := 0; autoIndex < loop.MaxTurns; autoIndex++ {
		turnIndex := startTurnIndex + autoIndex
		if r.shouldStopOnCaseStatus(loop.StopOnCaseStatus) {
			if _, err := fmt.Fprintf(os.Stderr, "autopilot stop turn=%d reason=case_status status=%s\n", turnIndex, strings.TrimSpace(loop.StopOnCaseStatus)); err != nil {
				return nil, fmt.Errorf("write autopilot status: %w", err)
			}
			return logs, nil
		}
		resp, err := r.lean.NextOpportunity(r.state, rolesPayload, loop.MaxStepsPerTurn)
		if err != nil {
			return nil, fmt.Errorf("lean next_opportunity failed: %w", err)
		}
		if ok, _ := resp["ok"].(bool); !ok {
			return nil, fmt.Errorf("lean next_opportunity error: %s", stringOrDefault(resp["error"], "unknown error"))
		}
		if terminal, _ := resp["terminal"].(bool); terminal {
			if _, err := fmt.Fprintf(os.Stderr, "autopilot stop turn=%d reason=%s\n", turnIndex, stringOrDefault(resp["reason"], "terminal")); err != nil {
				return nil, fmt.Errorf("write autopilot status: %w", err)
			}
			return logs, nil
		}
		stateVersion := intFromAny(resp["state_version"])
		opportunityPayload, _ := resp["opportunity"].(map[string]any)
		if len(opportunityPayload) == 0 {
			return nil, fmt.Errorf("lean next_opportunity returned empty opportunity")
		}
		opportunity, err := parseLeanOpportunity(opportunityPayload)
		if err != nil {
			return nil, err
		}
		role, ok := r.roles[opportunity.Role]
		if !ok {
			return nil, fmt.Errorf("lean next_opportunity returned unknown role: %s", opportunity.Role)
		}
		if _, err := fmt.Fprintf(
			os.Stderr,
			"agent call turn=%d source=next_opportunity role=%s opportunity_id=%s phase=%s kind=%s may_pass=%t why=%s allowed=%s\n",
			turnIndex,
			role.Name,
			opportunity.OpportunityID,
			opportunity.Phase,
			opportunity.Kind,
			opportunity.MayPass,
			opportunity.Objective,
			strings.Join(opportunity.AllowedTools, ","),
		); err != nil {
			return nil, fmt.Errorf("write agent call: %w", err)
		}
		var turnLog TurnLog
		if opportunity.DeterministicAction != nil {
			turn := spec.TurnSpec{
				Role:                opportunity.Role,
				Prompt:              opportunity.Objective,
				MaxSteps:            opportunity.StepBudget,
				AllowedTools:        opportunity.AllowedTools,
				DeterministicAction: opportunity.DeterministicAction,
				RequireSuccess:      true,
			}
			turnLog, err = r.executeTurn(ctx, turnIndex, role, turn, opportunity.AllowedTools)
			if err != nil {
				return nil, err
			}
		} else {
			if r.roleIsExternal(role.Name) {
				turnLog, err = r.executeExternalOpportunityTurn(ctx, turnIndex, role, opportunity, rolesPayload, stateVersion)
				if err != nil {
					return nil, err
				}
			} else {
				turnLog, err = r.executeOpportunityTurn(ctx, turnIndex, role, opportunity, rolesPayload, stateVersion)
				if err != nil {
					return nil, err
				}
			}
		}
		turnLog.Source = "next_opportunity"
		turnLog.ActionID = opportunity.OpportunityID
		turnLogsApplyOpportunity(&turnLog, opportunity)
		logs = append(logs, turnLog)
	}
	return nil, fmt.Errorf("autopilot exhausted max_turns=%d without stop", loop.MaxTurns)
}

func (r *Runner) autopilotRolesPayload() []map[string]any {
	rolesPayload := make([]map[string]any, 0, len(r.scenario.Roles))
	for _, role := range r.scenario.Roles {
		rolesPayload = append(rolesPayload, map[string]any{
			"role":          role.Name,
			"allowed_tools": role.EffectiveAllowedActions(),
		})
	}
	return rolesPayload
}

func parseLeanOpportunity(payload map[string]any) (leanOpportunity, error) {
	opportunityID := strings.TrimSpace(stringOrDefault(payload["opportunity_id"], ""))
	role := strings.TrimSpace(stringOrDefault(payload["role"], ""))
	if opportunityID == "" {
		return leanOpportunity{}, fmt.Errorf("lean next_opportunity returned missing opportunity_id")
	}
	if role == "" {
		return leanOpportunity{}, fmt.Errorf("lean next_opportunity returned missing role")
	}
	allowedRaw, _ := payload["allowed_tools"].([]any)
	allowed := make([]string, 0, len(allowedRaw))
	for _, item := range allowedRaw {
		value := strings.TrimSpace(stringOrDefault(item, ""))
		if value == "" {
			continue
		}
		allowed = append(allowed, value)
	}
	if len(allowed) == 0 {
		return leanOpportunity{}, fmt.Errorf("lean next_opportunity returned empty allowed_tools opportunity_id=%s", opportunityID)
	}
	stepBudget := intFromAny(payload["step_budget"])
	if stepBudget <= 0 {
		stepBudget = 1
	}
	var deterministic *spec.DeterministicAction
	if raw, ok := payload["deterministic_action"].(map[string]any); ok {
		kind, _ := raw["kind"].(string)
		actionType, _ := raw["action_type"].(string)
		argPayload, _ := raw["payload"].(map[string]any)
		if argPayload == nil {
			argPayload = map[string]any{}
		}
		for key, value := range raw {
			if key == "kind" || key == "action_type" || key == "payload" {
				continue
			}
			if _, exists := argPayload[key]; !exists {
				argPayload[key] = value
			}
		}
		deterministic = &spec.DeterministicAction{
			Kind:       kind,
			ActionType: actionType,
			Payload:    argPayload,
		}
	}
	return leanOpportunity{
		OpportunityID:       opportunityID,
		Role:                role,
		Phase:               strings.TrimSpace(stringOrDefault(payload["phase"], "")),
		Kind:                strings.TrimSpace(stringOrDefault(payload["kind"], "")),
		MayPass:             boolFromAny(payload["may_pass"]),
		ActorMessage:        strings.TrimSpace(stringOrDefault(payload["actor_message"], "")),
		Objective:           strings.TrimSpace(stringOrDefault(payload["objective"], "")),
		AllowedTools:        allowed,
		StepBudget:          stepBudget,
		Constraints:         mapOrEmpty(payload["constraints"]),
		DeterministicAction: deterministic,
	}, nil
}

func turnLogsApplyOpportunity(log *TurnLog, opportunity leanOpportunity) {
	if log == nil {
		return
	}
	log.Prompt = opportunity.Objective
	log.OpportunityID = opportunity.OpportunityID
	log.OpportunityPhase = opportunity.Phase
	log.OpportunityKind = opportunity.Kind
	log.OpportunityMessage = opportunity.ActorMessage
	log.MayPass = opportunity.MayPass
	if log.ActionID == "" {
		log.ActionID = opportunity.OpportunityID
	}
}

func (r *Runner) shouldStopOnCaseStatus(status string) bool {
	status = strings.TrimSpace(status)
	if status == "" {
		return false
	}
	caseObj, _ := r.state["case"].(map[string]any)
	if caseObj == nil {
		return false
	}
	current, _ := caseObj["status"].(string)
	return strings.TrimSpace(current) == status
}

func boolFromAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	default:
		return false
	}
}

func mapOrEmpty(v any) map[string]any {
	out, _ := v.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}
