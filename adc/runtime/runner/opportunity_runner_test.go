package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type opportunityTestRequest struct {
	model              string
	requestSpec        *modelrequest.Spec
	inputItems         []map[string]any
	tools              []map[string]any
	previousResponseID string
	temperature        *float64
}

type opportunityTestClient struct {
	mu         sync.Mutex
	responses  []openaiapi.Response
	errors     []error
	requests   []opportunityTestRequest
	accounting openaiapi.AccountingRecorder
}

func (c *opportunityTestClient) CreateResponse(
	_ context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
) (openaiapi.Response, error) {
	return c.next(opportunityTestRequest{
		model:              model,
		inputItems:         cloneRunnerMapList(inputItems),
		tools:              cloneRunnerMapList(tools),
		previousResponseID: previousResponseID,
		temperature:        cloneFloat64Pointer(temperature),
	})
}

func (c *opportunityTestClient) CreateResponseWithRequestSpec(
	_ context.Context,
	requestSpec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	return c.next(opportunityTestRequest{
		requestSpec:        &requestSpec,
		inputItems:         cloneRunnerMapList(inputItems),
		tools:              cloneRunnerMapList(tools),
		previousResponseID: previousResponseID,
	})
}

func (c *opportunityTestClient) next(request opportunityTestRequest) (openaiapi.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, request)
	index := len(c.requests) - 1
	var response openaiapi.Response
	if index < len(c.responses) {
		response = c.responses[index]
	}
	var err error
	if index < len(c.errors) {
		err = c.errors[index]
	}
	c.accounting.Record(response)
	return response, err
}

func (c *opportunityTestClient) Accounting() openaiapi.Accounting {
	return c.accounting.Snapshot()
}

func TestNewRejectsNilStore(t *testing.T) {
	_, err := New(nil, lean.New([]string{"unused"}), nil, nil, Config{ScenarioPath: "unused"})
	if err == nil || !strings.Contains(err.Error(), "store is nil") {
		t.Fatalf("New error = %v, want nil-store error", err)
	}
}

func TestPersistenceRejectsMissingStoreOutsideOpportunityRunner(t *testing.T) {
	r := &Runner{}
	if err := r.persistActionEvent(1, 1, "judge", "test", map[string]any{}, map[string]any{}); err == nil || !strings.Contains(err.Error(), "store is nil") {
		t.Fatalf("persistActionEvent error = %v, want nil-store error", err)
	}
	if err := r.persistAgentEvent(1, 1, "judge", "test", map[string]any{}); err == nil || !strings.Contains(err.Error(), "store is nil") {
		t.Fatalf("persistAgentEvent error = %v, want nil-store error", err)
	}
}

func TestOpportunityRunnerExecutesReferenceToolAndDecision(t *testing.T) {
	t.Parallel()

	engine, requestLog := writeOpportunityRunnerEngine(t)
	eventsPath := filepath.Join(t.TempDir(), "events.jsonl")
	client := &opportunityTestClient{responses: []openaiapi.Response{
		{
			ResponseID: "response-1",
			ToolCalls: []openaiapi.ToolCall{{
				CallID:    "support-1",
				Name:      "get_case",
				Arguments: map[string]any{},
			}},
		},
		{
			ResponseID: "response-2",
			ToolCalls: []openaiapi.ToolCall{{
				CallID: "decision-1",
				Name:   "file_rule12_motion",
				Arguments: map[string]any{
					"motion_id": "motion-1",
					"grounds":   []any{"failure_to_state_claim"},
					"summary":   "The complaint omits a required element.",
				},
			}},
		},
	}}
	initialState := opportunityRunnerTestState()
	r, err := NewOpportunityRunner(OpportunityRunnerOptions{
		State:      initialState,
		Roles:      opportunityRunnerTestRoles(),
		Engine:     engine,
		Client:     client,
		Court:      courts.PropositionTribunal(),
		Model:      "test-model",
		EventsPath: eventsPath,
	})
	if err != nil {
		t.Fatalf("NewOpportunityRunner error = %v", err)
	}
	initialState["state_version"] = 99
	initialState["case"].(map[string]any)["status"] = "caller-mutated"

	opportunity := opportunityRunnerTestOpportunity()
	opportunityBefore := cloneRunnerMap(opportunity)
	log, err := r.ExecuteDirectOpportunity(context.Background(), DirectOpportunityRequest{
		Opportunity:  opportunity,
		StateVersion: 7,
		RolesPayload: opportunityRunnerTestRolesPayload(),
		TurnIndex:    4,
	})
	if err != nil {
		t.Fatalf("ExecuteDirectOpportunity error = %v", err)
	}
	if !reflect.DeepEqual(opportunity, opportunityBefore) {
		t.Fatalf("ExecuteDirectOpportunity mutated opportunity:\n got %#v\nwant %#v", opportunity, opportunityBefore)
	}
	if log.Source != "next_opportunity" || log.OpportunityID != "opportunity-1" || log.ActionID != "opportunity-1" || log.Role != "judge" {
		t.Fatalf("turn log metadata = %#v", log)
	}
	if log.Steps != 2 || len(log.Transcript) != 3 {
		t.Fatalf("turn log = %#v", log)
	}
	if len(client.requests) != 2 {
		t.Fatalf("model requests = %d, want 2", len(client.requests))
	}
	systemPrompt, _ := client.requests[0].inputItems[0]["content"].(string)
	userPrompt, _ := client.requests[0].inputItems[1]["content"].(string)
	if !strings.Contains(systemPrompt, `"ok":true`) || !strings.Contains(userPrompt, "Resolve the pending Rule 12 motion.") {
		t.Fatalf("first model input = %#v", client.requests[0].inputItems)
	}
	if client.requests[1].previousResponseID != "response-1" {
		t.Fatalf("second previous_response_id = %q", client.requests[1].previousResponseID)
	}
	if output, _ := client.requests[1].inputItems[0]["output"].(string); !strings.Contains(output, "case-1") {
		t.Fatalf("support-tool continuation = %#v", client.requests[1].inputItems)
	}
	if got := r.ProviderAccounting().RequestCount; got != 2 {
		t.Fatalf("provider request count = %d, want 2", got)
	}
	state := r.State()
	if state["state_version"] != float64(8) || state["case"].(map[string]any)["status"] != "motion_filed" {
		t.Fatalf("runner state = %#v", state)
	}
	state["case"].(map[string]any)["status"] = "result-mutated"
	if r.State()["case"].(map[string]any)["status"] != "motion_filed" {
		t.Fatal("State returned a mutable alias")
	}
	view := r.LastOpportunityView()
	view["view"].(map[string]any)["state"] = map[string]any{"caller": "mutation"}
	if _, ok := r.LastOpportunityView()["view"].(map[string]any)["state"].(map[string]any)["caller"]; ok {
		t.Fatal("LastOpportunityView returned a mutable alias")
	}
	requestsRaw, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatalf("read engine request log: %v", err)
	}
	requestsText := string(requestsRaw)
	for _, requestType := range []string{`"request_type":"role_view"`, `"request_type":"apply_decision"`, `"action_type":"file_rule12_motion"`} {
		if !strings.Contains(requestsText, requestType) {
			t.Fatalf("engine request log lacks %s:\n%s", requestType, requestsText)
		}
	}
	eventsRaw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, eventType := range []string{`"action":"get_case"`, `"agent_event":"agent_completion_result"`, `"action":"file_rule12_motion"`} {
		if !strings.Contains(string(eventsRaw), eventType) {
			t.Fatalf("events lack %s:\n%s", eventType, string(eventsRaw))
		}
	}
}

func TestOpportunityRunnerReturnsTypedProceduralFailure(t *testing.T) {
	t.Parallel()

	engine, _ := writeOpportunityRunnerEngine(t)
	client := &opportunityTestClient{responses: []openaiapi.Response{
		{ResponseID: "response-1", Text: "No tool call."},
		{ResponseID: "response-2", Text: "Still no tool call."},
	}}
	r, err := NewOpportunityRunner(OpportunityRunnerOptions{
		State:  opportunityRunnerTestState(),
		Roles:  opportunityRunnerTestRoles(),
		Engine: engine,
		Client: client,
		Court:  courts.PropositionTribunal(),
		Model:  "test-model",
		Runtime: RuntimeLimits{
			InvalidAttemptLimit: 2,
		},
	})
	if err != nil {
		t.Fatalf("NewOpportunityRunner error = %v", err)
	}
	log, err := r.ExecuteDirectOpportunity(context.Background(), DirectOpportunityRequest{
		Opportunity:  opportunityRunnerTestOpportunity(),
		StateVersion: 7,
		RolesPayload: opportunityRunnerTestRolesPayload(),
		TurnIndex:    6,
	})
	var failure *ProceduralTurnFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %T %v, want ProceduralTurnFailure", err, err)
	}
	if failure.Kind != ProceduralFailureInvalidAttemptLimit {
		t.Fatalf("failure kind = %q", failure.Kind)
	}
	if failure.Partial.Steps != 2 || failure.Partial.OpportunityID != "opportunity-1" {
		t.Fatalf("partial log = %#v", failure.Partial)
	}
	if !reflect.DeepEqual(log, failure.Partial) {
		t.Fatalf("returned log = %#v, partial = %#v", log, failure.Partial)
	}
}

func TestOpportunityRunnerClassifiesDecisionBudgetExhaustion(t *testing.T) {
	t.Parallel()

	engine, _ := writeOpportunityRunnerEngine(t)
	client := &opportunityTestClient{responses: []openaiapi.Response{{ResponseID: "response-1", Text: "No tool call."}}}
	r, err := NewOpportunityRunner(OpportunityRunnerOptions{
		State:  opportunityRunnerTestState(),
		Roles:  opportunityRunnerTestRoles(),
		Engine: engine,
		Client: client,
		Court:  courts.PropositionTribunal(),
		Model:  "test-model",
		Runtime: RuntimeLimits{
			InvalidAttemptLimit: 3,
		},
	})
	if err != nil {
		t.Fatalf("NewOpportunityRunner error = %v", err)
	}
	opportunity := opportunityRunnerTestOpportunity()
	opportunity["step_budget"] = 1
	_, err = r.ExecuteDirectOpportunity(context.Background(), DirectOpportunityRequest{
		Opportunity:  opportunity,
		StateVersion: 7,
		RolesPayload: opportunityRunnerTestRolesPayload(),
		TurnIndex:    1,
	})
	var failure *ProceduralTurnFailure
	if !errors.As(err, &failure) || failure.Kind != ProceduralFailureDecisionBudget {
		t.Fatalf("error = %#v, want decision-budget procedural failure", err)
	}
}

func TestOpportunityRunnerContextBoundsLeanStep(t *testing.T) {
	t.Parallel()

	engine := writeSlowStepOpportunityRunnerEngine(t)
	client := &opportunityTestClient{responses: []openaiapi.Response{{
		ResponseID: "response-1",
		ToolCalls: []openaiapi.ToolCall{{
			CallID:    "decision-1",
			Name:      "file_rule12_motion",
			Arguments: map[string]any{"motion_id": "motion-1"},
		}},
	}}}
	r, err := NewOpportunityRunner(OpportunityRunnerOptions{
		State:  opportunityRunnerTestState(),
		Roles:  opportunityRunnerTestRoles(),
		Engine: engine,
		Client: client,
		Court:  courts.PropositionTribunal(),
		Model:  "test-model",
	})
	if err != nil {
		t.Fatalf("NewOpportunityRunner error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = r.ExecuteDirectOpportunity(ctx, DirectOpportunityRequest{
		Opportunity:  opportunityRunnerTestOpportunity(),
		StateVersion: 7,
		RolesPayload: opportunityRunnerTestRolesPayload(),
		TurnIndex:    1,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecuteDirectOpportunity error = %v, want deadline exceeded", err)
	}
	var procedural *ProceduralTurnFailure
	if errors.As(err, &procedural) {
		t.Fatalf("Lean timeout classified as procedural failure: %#v", procedural)
	}
}

func opportunityRunnerTestState() map[string]any {
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

func opportunityRunnerTestRoles() []spec.RoleSpec {
	return []spec.RoleSpec{{
		Name:           "judge",
		Instructions:   "Decide the pending procedural question.",
		AllowedActions: []string{"file_rule12_motion"},
	}}
}

func opportunityRunnerTestRolesPayload() []map[string]any {
	return []map[string]any{{
		"role":          "judge",
		"allowed_tools": []any{"file_rule12_motion"},
	}}
}

func opportunityRunnerTestOpportunity() map[string]any {
	return map[string]any{
		"opportunity_id": "opportunity-1",
		"role":           "judge",
		"phase":          "pleading",
		"kind":           "rule12_decision",
		"actor_message":  "Decide whether the complaint states a claim.",
		"objective":      "Resolve the pending Rule 12 motion.",
		"allowed_tools":  []any{"file_rule12_motion"},
		"step_budget":    3,
		"constraints":    map[string]any{},
	}
}

func writeOpportunityRunnerEngine(t *testing.T) (lean.Engine, string) {
	t.Helper()
	dir := t.TempDir()
	requestLog := filepath.Join(dir, "requests.jsonl")
	path := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
printf '%s\n' "$request" >> "$1"
case "$request" in
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
	return lean.New([]string{path, requestLog}), requestLog
}

func writeSlowStepOpportunityRunnerEngine(t *testing.T) lean.Engine {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *'"request_type":"role_view"'*)
    printf '%s\n' '{"ok":true,"view":{"state":{"state_version":7,"case":{"case_id":"case-1","status":"pleading","case_files":[],"decision_traces":[]}}}}'
    ;;
  *'"request_type":"apply_decision"'*)
    printf '%s\n' '{"ok":true,"result_kind":"execute_tool","action":{"action_type":"file_rule12_motion","actor_role":"judge","payload":{"motion_id":"motion-1"}}}'
    ;;
  *'"action_type":"file_rule12_motion"'*)
    exec sleep 5
    ;;
  *)
    printf '%s\n' '{"ok":false,"error":"unexpected request"}'
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write slow fake engine: %v", err)
	}
	return lean.New([]string{path})
}
