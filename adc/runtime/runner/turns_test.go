package runner

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	openaiapi "github.com/agentcourt/adj/common/openai"
)

func TestChooseRandomCandidateJurorIDs(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	selected, err := chooseRandomCandidateJurorIDs(rng, []string{"J1", "J2", "J3", "J4", "J5"}, 3)
	if err != nil {
		t.Fatalf("chooseRandomCandidateJurorIDs returned error: %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf("chooseRandomCandidateJurorIDs returned %d ids, want 3", len(selected))
	}
	seen := map[string]bool{}
	for _, jurorID := range selected {
		if seen[jurorID] {
			t.Fatalf("chooseRandomCandidateJurorIDs returned duplicate id %q in %v", jurorID, selected)
		}
		seen[jurorID] = true
		switch jurorID {
		case "J1", "J2", "J3", "J4", "J5":
		default:
			t.Fatalf("chooseRandomCandidateJurorIDs returned out-of-panel id %q in %v", jurorID, selected)
		}
	}
}

func TestCandidateJurorIDs(t *testing.T) {
	state := map[string]any{
		"case": map[string]any{
			"jurors": []any{
				map[string]any{"juror_id": "J1", "status": "candidate"},
				map[string]any{"juror_id": "J2", "status": "sworn"},
				map[string]any{"juror_id": "J3", "status": "candidate"},
				map[string]any{"juror_id": "", "status": "candidate"},
			},
		},
	}
	got := candidateJurorIDs(state)
	want := []string{"J1", "J3"}
	if len(got) != len(want) {
		t.Fatalf("candidateJurorIDs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("candidateJurorIDs = %v, want %v", got, want)
		}
	}
}

func TestParseLiteralToolCall(t *testing.T) {
	allowed := []string{"submit_juror_vote", "pass_turn"}
	text := `submit_juror_vote Republike {"juror_id":"J8","vote":"defendant","damages":5000,"confidence":"high"}`
	name, arguments, ok := parseLiteralToolCall(text, allowed)
	if !ok {
		t.Fatalf("parseLiteralToolCall(%q) = not ok, want ok", text)
	}
	if name != "submit_juror_vote" {
		t.Fatalf("parseLiteralToolCall(%q) name = %q, want submit_juror_vote", text, name)
	}
	want := map[string]any{
		"juror_id":   "J8",
		"vote":       "defendant",
		"damages":    float64(5000),
		"confidence": "high",
	}
	if !reflect.DeepEqual(arguments, want) {
		t.Fatalf("parseLiteralToolCall(%q) arguments = %#v, want %#v", text, arguments, want)
	}
}

func TestParseLiteralToolCallPassTurnWithoutArguments(t *testing.T) {
	name, arguments, ok := parseLiteralToolCall("pass_turn", []string{"pass_turn"})
	if !ok {
		t.Fatalf("parseLiteralToolCall(pass_turn) = not ok, want ok")
	}
	if name != "pass_turn" {
		t.Fatalf("parseLiteralToolCall(pass_turn) name = %q, want pass_turn", name)
	}
	if len(arguments) != 0 {
		t.Fatalf("parseLiteralToolCall(pass_turn) arguments = %#v, want empty map", arguments)
	}
}

func TestParseLiteralToolCallRejectsTrailingGarbage(t *testing.T) {
	text := `submit_juror_vote {"juror_id":"J8"} extra`
	if _, _, ok := parseLiteralToolCall(text, []string{"submit_juror_vote"}); ok {
		t.Fatalf("parseLiteralToolCall(%q) = ok, want not ok", text)
	}
}

func TestRecoverLiteralToolCall(t *testing.T) {
	resp := recoverLiteralToolCall(openaiResponse(`submit_juror_vote ব্যক{"juror_id":"J8","vote":"plaintiff","damages":5000}`), []string{"submit_juror_vote"})
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("recoverLiteralToolCall tool calls = %#v, want one call", resp.ToolCalls)
	}
	call := resp.ToolCalls[0]
	if call.Name != "submit_juror_vote" {
		t.Fatalf("recoverLiteralToolCall name = %q, want submit_juror_vote", call.Name)
	}
	if call.Arguments["juror_id"] != "J8" || call.Arguments["vote"] != "plaintiff" {
		t.Fatalf("recoverLiteralToolCall arguments = %#v", call.Arguments)
	}
}

func TestIssueFromMalformedToolCall(t *testing.T) {
	call := openaiapi.ToolCall{
		Name:           "submit_juror_vote",
		RawArguments:   "{",
		ArgumentsError: "unexpected end of JSON input",
	}
	r := &Runner{prompts: testPromptCatalog(t)}
	issue, err := r.issueFromMalformedToolCall(call)
	if err != nil {
		t.Fatalf("issueFromMalformedToolCall() error = %v", err)
	}
	if issue.Tool != "submit_juror_vote" {
		t.Fatalf("Tool = %q, want submit_juror_vote", issue.Tool)
	}
	if issue.Error != "malformed JSON arguments: unexpected end of JSON input" {
		t.Fatalf("Error = %q", issue.Error)
	}
	if issue.ActorMessage == "" {
		t.Fatalf("ActorMessage = empty")
	}
}

func TestCompletionResultPayloadIncludesMalformedToolCallDetails(t *testing.T) {
	resp := openaiapi.Response{
		ToolCalls: []openaiapi.ToolCall{{
			CallID:         "call_1",
			Name:           "submit_juror_vote",
			RawArguments:   "{",
			ArgumentsError: "unexpected end of JSON input",
		}},
	}
	payload := completionResultPayload(
		"openrouter://model",
		leanOpportunity{OpportunityID: "opp-1", Phase: "deliberation"},
		resp,
		"rejected",
		nil,
		1,
	)
	toolCalls, ok := payload["tool_calls"].([]map[string]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("tool_calls = %#v", payload["tool_calls"])
	}
	if toolCalls[0]["raw_arguments"] != "{" {
		t.Fatalf("raw_arguments = %#v", toolCalls[0]["raw_arguments"])
	}
	if toolCalls[0]["arguments_error"] != "unexpected end of JSON input" {
		t.Fatalf("arguments_error = %#v", toolCalls[0]["arguments_error"])
	}
}

func TestApplyOpportunityPayloadDefaultsRequiresRule60Granted(t *testing.T) {
	r := &Runner{}
	tests := []struct {
		name      string
		arguments map[string]any
		wantIssue bool
	}{
		{name: "missing", arguments: map[string]any{}, wantIssue: true},
		{name: "non-Boolean", arguments: map[string]any{"granted": "false"}, wantIssue: true},
		{name: "false", arguments: map[string]any{"granted": false}},
		{name: "true", arguments: map[string]any{"granted": true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, issue, err := r.applyOpportunityPayloadDefaults("resolve_rule60_motion", test.arguments, leanOpportunity{})
			if err != nil {
				t.Fatalf("applyOpportunityPayloadDefaults error = %v", err)
			}
			if got := issue != nil; got != test.wantIssue {
				t.Fatalf("issue = %#v, want issue %t", issue, test.wantIssue)
			}
			if test.wantIssue {
				if issue.Error != "required field granted must be a Boolean" {
					t.Fatalf("issue error = %q", issue.Error)
				}
				if payload != nil {
					t.Fatalf("payload = %#v, want nil", payload)
				}
				return
			}
			if payload["granted"] != test.arguments["granted"] {
				t.Fatalf("payload = %#v", payload)
			}
		})
	}
}

func TestApplyOpportunityPayloadDefaultsUsesRuntimeEventTime(t *testing.T) {
	r := &Runner{}
	tests := []struct {
		action string
		field  string
	}{
		{action: "produce_case_file", field: "produced_at"},
		{action: "offer_exhibit", field: "offered_at"},
	}
	for _, test := range tests {
		t.Run(test.action, func(t *testing.T) {
			payload, issue, err := r.applyOpportunityPayloadDefaults(test.action, map[string]any{
				test.field: "1900-01-01T00:00:00Z",
			}, leanOpportunity{})
			if err != nil {
				t.Fatalf("applyOpportunityPayloadDefaults error = %v", err)
			}
			if issue != nil {
				t.Fatalf("issue = %#v", issue)
			}
			got, _ := payload[test.field].(string)
			if got == "1900-01-01T00:00:00Z" {
				t.Fatalf("%s retained participant value", test.field)
			}
			if _, err := time.Parse(time.RFC3339, got); err != nil {
				t.Fatalf("%s = %q: %v", test.field, got, err)
			}
		})
	}
}

func TestApplyActionEventTimePreservesDirectActionTime(t *testing.T) {
	payload := map[string]any{"produced_at": "2026-09-13T12:00:00Z"}
	got := applyActionEventTime("produce_case_file", payload, "2026-09-13T13:00:00Z", false)
	if got["produced_at"] != payload["produced_at"] {
		t.Fatalf("produced_at = %#v, want %#v", got["produced_at"], payload["produced_at"])
	}
}

func TestEnforceResponseSizeLimit(t *testing.T) {
	resp := openaiapi.Response{Text: "0123456789"}
	if err := enforceResponseSizeLimit(resp, 0); err != nil {
		t.Fatalf("enforceResponseSizeLimit unlimited error = %v", err)
	}
	if err := enforceResponseSizeLimit(resp, 1024); err != nil {
		t.Fatalf("enforceResponseSizeLimit roomy limit error = %v", err)
	}
	if err := enforceResponseSizeLimit(resp, 4); err == nil {
		t.Fatalf("enforceResponseSizeLimit tight limit error = nil")
	}
}

func openaiResponse(text string) openaiapi.Response {
	return openaiapi.Response{Text: text}
}
