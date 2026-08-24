package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/adc/runtime/spec"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

func TestExecuteJudgeOpportunityRetainsDeterministicActionByDefault(t *testing.T) {
	t.Parallel()

	engine := writeEvalOpportunityEngine(t)
	result, err := executeJudgeOpportunity(context.Background(), judgeOpportunityExecutionOptions{
		Engine:       engine,
		State:        evalOpportunityState(),
		Roles:        evalOpportunityRoles(),
		RolesPayload: evalOpportunityRolesPayload(),
		Court:        courts.PropositionTribunal(),
		Model:        "test-model",
	})
	if err != nil {
		t.Fatalf("executeJudgeOpportunity error = %v", err)
	}
	if _, ok := result.Opportunity["deterministic_action"]; !ok {
		t.Fatalf("opportunity = %#v, want deterministic_action", result.Opportunity)
	}
	if len(result.Exchanges) != 0 || result.Provider.RequestCount != 0 {
		t.Fatalf("model exchanges = %#v, accounting = %#v", result.Exchanges, result.Provider)
	}
	if result.View != nil {
		t.Fatalf("deterministic execution view = %#v, want nil", result.View)
	}
	if result.FinalState["case"].(map[string]any)["status"] != "motion_filed" {
		t.Fatalf("final state = %#v", result.FinalState)
	}
}

func TestExecuteJudgeOpportunityRejectsNilClientForModelPath(t *testing.T) {
	t.Parallel()

	_, err := executeJudgeOpportunity(context.Background(), judgeOpportunityExecutionOptions{
		Engine:              writeEvalOpportunityEngine(t),
		State:               evalOpportunityState(),
		Roles:               evalOpportunityRoles(),
		RolesPayload:        evalOpportunityRolesPayload(),
		Court:               courts.PropositionTribunal(),
		Model:               "test-model",
		CounterfactualModel: true,
	})
	if err == nil || err.Error() != "model-backed judge opportunity requires a response client" {
		t.Fatalf("executeJudgeOpportunity error = %v", err)
	}
}

func TestExecuteJudgeOpportunityCounterfactualUsesModelPath(t *testing.T) {
	t.Parallel()

	engine := writeEvalOpportunityEngine(t)
	client := newTestResponseClient(testResponseStep{response: openaiapi.Response{
		ResponseID: "response-1",
		ToolCalls: []openaiapi.ToolCall{{
			CallID: "decision-1",
			Name:   "file_rule12_motion",
			Arguments: map[string]any{
				"motion_id": "motion-1",
			},
		}},
	}})
	result, err := executeJudgeOpportunity(context.Background(), judgeOpportunityExecutionOptions{
		Engine:              engine,
		State:               evalOpportunityState(),
		Roles:               evalOpportunityRoles(),
		RolesPayload:        evalOpportunityRolesPayload(),
		Client:              client,
		Court:               courts.PropositionTribunal(),
		Model:               "test-model",
		CounterfactualModel: true,
		Objective: func(opportunity map[string]any) (string, error) {
			if _, ok := opportunity["deterministic_action"]; ok {
				t.Fatalf("objective callback received deterministic_action: %#v", opportunity)
			}
			opportunity["role"] = "callback mutation"
			return "Evaluate the counterfactual model decision.", nil
		},
	})
	if err != nil {
		t.Fatalf("executeJudgeOpportunity error = %v", err)
	}
	if _, ok := result.Opportunity["deterministic_action"]; ok {
		t.Fatalf("counterfactual opportunity = %#v", result.Opportunity)
	}
	if len(result.Exchanges) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if result.Opportunity["role"] != "judge" || result.TurnLog.Prompt != "Evaluate the counterfactual model decision." {
		t.Fatalf("objective result = %#v", result)
	}
	if ok, _ := result.View["ok"].(bool); !ok {
		t.Fatalf("full view wrapper = %#v", result.View)
	}
	if result.FinalState["case"].(map[string]any)["status"] != "motion_filed" {
		t.Fatalf("final state = %#v", result.FinalState)
	}
}

func TestExecuteJudgeOpportunityRequiresOriginalDeterministicActionBeforeClientUse(t *testing.T) {
	t.Parallel()

	for _, counterfactual := range []bool{false, true} {
		counterfactual := counterfactual
		t.Run(map[bool]string{false: "production", true: "counterfactual"}[counterfactual], func(t *testing.T) {
			t.Parallel()
			client := newTestResponseClient(testResponseStep{response: openaiapi.Response{
				ResponseID: "response-1",
				ToolCalls:  []openaiapi.ToolCall{{CallID: "decision-1", Name: "file_rule12_motion"}},
			}})
			_, err := executeJudgeOpportunity(context.Background(), judgeOpportunityExecutionOptions{
				Engine:                     writeEvalOpportunityEngineWithoutDeterministicAction(t),
				State:                      evalOpportunityState(),
				Roles:                      evalOpportunityRoles(),
				RolesPayload:               evalOpportunityRolesPayload(),
				Client:                     client,
				Court:                      courts.PropositionTribunal(),
				Model:                      "test-model",
				CounterfactualModel:        counterfactual,
				RequireDeterministicAction: true,
			})
			if err == nil || err.Error() != "judge opportunity requires a deterministic action" {
				t.Fatalf("executeJudgeOpportunity error = %v", err)
			}
			if got := client.Accounting().RequestCount; got != 0 {
				t.Fatalf("provider request count = %d, want 0", got)
			}
		})
	}
}

func TestJudgeEvalDecisionForScoringUsesFinalSuccessfulPass(t *testing.T) {
	t.Parallel()

	execution := judgeOpportunityExecution{Exchanges: []ResponseExchange{
		{
			Request: ResponseRequest{InputItems: []map[string]any{{"content": "rejected target"}}},
			Response: openaiapi.Response{
				ResponseID: "rejected-target-response",
				ToolCalls:  []openaiapi.ToolCall{{Name: "target_tool", Arguments: map[string]any{"value": "rejected"}}},
			},
		},
		{
			Request: ResponseRequest{InputItems: []map[string]any{{"content": "accepted pass"}}},
			Response: openaiapi.Response{
				ResponseID: "accepted-pass-response",
				ToolCalls:  []openaiapi.ToolCall{{Name: "pass_turn", Arguments: map[string]any{"reason": "defer"}}},
			},
		},
	}}
	decision, err := judgeEvalDecisionForScoring(execution, "target_tool", nil)
	if err != nil {
		t.Fatalf("judgeEvalDecisionForScoring error = %v", err)
	}
	if decision.RawResponse.ResponseID != "accepted-pass-response" || decision.ScoringResponse.ResponseID != "accepted-pass-response" {
		t.Fatalf("decision = %+v", decision)
	}
	if len(decision.ScoringResponse.ToolCalls) != 1 || decision.ScoringResponse.ToolCalls[0].Name != "pass_turn" {
		t.Fatalf("scoring response = %+v, want terminal pass_turn", decision.ScoringResponse)
	}
	if got := decision.Input[0]["content"]; got != "accepted pass" {
		t.Fatalf("decision input content = %v, want accepted pass", got)
	}
}

func TestJudgeEvalDecisionForScoringUsesTerminalProceduralExchange(t *testing.T) {
	t.Parallel()

	execution := judgeOpportunityExecution{Exchanges: []ResponseExchange{
		{
			Request: ResponseRequest{InputItems: []map[string]any{{"content": "earlier"}}},
			Response: openaiapi.Response{
				ResponseID: "earlier-response",
				ToolCalls:  []openaiapi.ToolCall{{Name: "target_tool", Arguments: map[string]any{"value": "earlier"}}},
			},
		},
		{
			Request:  ResponseRequest{InputItems: []map[string]any{{"content": "terminal"}}},
			Response: openaiapi.Response{ResponseID: "terminal-response", ToolCalls: []openaiapi.ToolCall{{Name: "wrong_tool"}}},
		},
	}}
	failure := &runner.ProceduralTurnFailure{
		Kind: runner.ProceduralFailureInvalidAttemptLimit,
		Err:  errors.New("invalid-attempt limit"),
	}
	decision, err := judgeEvalDecisionForScoring(execution, "target_tool", failure)
	if err != nil {
		t.Fatalf("judgeEvalDecisionForScoring error = %v", err)
	}
	if decision.RawResponse.ResponseID != "terminal-response" || decision.ScoringResponse.ResponseID != "terminal-response" {
		t.Fatalf("decision = %+v", decision)
	}
	if got := decision.Input[0]["content"]; got != "terminal" {
		t.Fatalf("decision input content = %v, want terminal", got)
	}
	if got := judgeEvalProceduralInvalidReason(failure); got != "procedural_invalid_attempt_limit" {
		t.Fatalf("invalid reason = %q", got)
	}
}

func TestJudgeEvalProceduralInvalidReasonCoversDecisionBudget(t *testing.T) {
	t.Parallel()

	failure := &runner.ProceduralTurnFailure{
		Kind: runner.ProceduralFailureDecisionBudget,
		Err:  errors.New("decision budget exhausted"),
	}
	if got := judgeEvalProceduralInvalidReason(failure); got != "procedural_decision_budget_exhausted" {
		t.Fatalf("invalid reason = %q", got)
	}
}

func TestJudgeEvalDecisionForScoringReturnsProviderError(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("provider failed")
	client := newTestResponseClient(testResponseStep{err: providerErr})
	execution, executionErr := executeJudgeOpportunity(context.Background(), judgeOpportunityExecutionOptions{
		Engine:              writeEvalOpportunityEngine(t),
		State:               evalOpportunityState(),
		Roles:               evalOpportunityRoles(),
		RolesPayload:        evalOpportunityRolesPayload(),
		Client:              client,
		Court:               courts.PropositionTribunal(),
		Model:               "test-model",
		CounterfactualModel: true,
	})
	if !errors.Is(executionErr, providerErr) {
		t.Fatalf("executeJudgeOpportunity error = %v", executionErr)
	}
	_, err := judgeEvalDecisionForScoring(execution, "file_rule12_motion", executionErr)
	if !errors.Is(err, providerErr) {
		t.Fatalf("judgeEvalDecisionForScoring error = %v", err)
	}
}

func evalOpportunityState() map[string]any {
	return map[string]any{
		"state_version": 7,
		"policy": map[string]any{
			"max_support_tool_calls_per_opportunity": 3,
		},
		"case": map[string]any{
			"case_id":         "case-1",
			"status":          "pleading",
			"case_files":      []any{},
			"decision_traces": []any{},
		},
	}
}

func evalOpportunityRoles() []spec.RoleSpec {
	return []spec.RoleSpec{{
		Name:           "judge",
		Instructions:   "Decide the pending procedural question.",
		AllowedActions: []string{"file_rule12_motion"},
	}}
}

func evalOpportunityRolesPayload() []map[string]any {
	return []map[string]any{{
		"role":          "judge",
		"allowed_tools": []any{"file_rule12_motion"},
	}}
}

func writeEvalOpportunityEngine(t *testing.T) lean.Engine {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *'"request_type":"next_opportunity"'*)
    printf '%s\n' '{"ok":true,"state_version":7,"opportunity":{"opportunity_id":"opportunity-1","role":"judge","phase":"pleading","kind":"rule12_decision","actor_message":"Decide the pending motion.","objective":"Resolve the Rule 12 motion.","allowed_tools":["file_rule12_motion"],"step_budget":3,"constraints":{},"deterministic_action":{"kind":"single_tool","action_type":"file_rule12_motion","payload":{"motion_id":"motion-1"}}}}'
    ;;
  *'"request_type":"role_view"'*)
    printf '%s\n' '{"ok":true,"view":{"state":{"state_version":7,"case":{"case_id":"case-1","status":"pleading","case_files":[],"decision_traces":[]}}}}'
    ;;
  *'"request_type":"apply_decision"'*)
    printf '%s\n' '{"ok":true,"result_kind":"execute_tool","action":{"action_type":"file_rule12_motion","actor_role":"judge","payload":{"motion_id":"motion-1"}}}'
    ;;
  *'"action_type":"file_rule12_motion"'*)
    printf '%s\n' '{"ok":true,"state":{"state_version":8,"case":{"case_id":"case-1","status":"motion_filed","case_files":[],"decision_traces":[]}}}'
    ;;
  *)
    printf '%s\n' '{"ok":false,"error":"unexpected request"}'
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	return lean.New([]string{path})
}

func writeEvalOpportunityEngineWithoutDeterministicAction(t *testing.T) lean.Engine {
	t.Helper()
	path := filepath.Join(t.TempDir(), "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *'"request_type":"next_opportunity"'*)
    printf '%s\n' '{"ok":true,"state_version":7,"opportunity":{"opportunity_id":"opportunity-1","role":"judge","phase":"pleading","kind":"rule12_decision","actor_message":"Decide the pending motion.","objective":"Resolve the Rule 12 motion.","allowed_tools":["file_rule12_motion"],"step_budget":3,"constraints":{}}}'
    ;;
  *)
    printf '%s\n' '{"ok":false,"error":"unexpected request"}'
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	return lean.New([]string{path})
}
