package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

func TestScriptedResponseClientRecordsRequestResponseSequence(t *testing.T) {
	t.Parallel()

	temperature := 0.25
	scriptErr := errors.New("scripted provider failure")
	client := NewScriptedResponseClient([]ScriptedResponse{
		{Response: openaiapi.Response{
			ResponseID: "response-1",
			ToolCalls: []openaiapi.ToolCall{{
				CallID:    "call-1",
				Name:      "get_case",
				Arguments: map[string]any{"scope": "visible"},
			}},
			UsageKnown: true,
			Usage:      openaiapi.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		}},
		{Err: scriptErr},
	})
	input := []map[string]any{{"role": "user", "content": "first"}}
	tools := []map[string]any{{"type": "function", "name": "get_case"}}
	if _, err := client.CreateResponse(context.Background(), "model-1", input, tools, "", &temperature); err != nil {
		t.Fatalf("first CreateResponse error = %v", err)
	}
	input[0]["content"] = "caller mutation"
	spec := modelrequest.Spec{Endpoint: "openrouter", Model: "model-2", Headers: map[string]string{"x-test": "one"}}
	_, err := client.CreateResponseWithRequestSpec(context.Background(), spec, nil, nil, "response-1")
	if !errors.Is(err, scriptErr) {
		t.Fatalf("second response error = %v", err)
	}
	spec.Headers["x-test"] = "caller mutation"

	exchanges := client.Exchanges()
	if len(exchanges) != 2 || exchanges[0].Request.Sequence != 1 || exchanges[1].Request.Sequence != 2 {
		t.Fatalf("exchanges = %#v", exchanges)
	}
	if exchanges[0].Request.InputItems[0]["content"] != "first" {
		t.Fatalf("recorded input = %#v", exchanges[0].Request.InputItems)
	}
	if exchanges[1].Request.RequestSpec.Headers["x-test"] != "one" || exchanges[1].Error != scriptErr.Error() {
		t.Fatalf("second exchange = %#v", exchanges[1])
	}
	exchanges[0].Request.InputItems[0]["content"] = "result mutation"
	if client.Exchanges()[0].Request.InputItems[0]["content"] != "first" {
		t.Fatal("Exchanges returned a mutable alias")
	}
	if client.Remaining() != 0 {
		t.Fatalf("Remaining = %d, want 0", client.Remaining())
	}
	accounting := client.Accounting()
	if accounting.RequestCount != 2 || accounting.UsageObservedCount != 1 || accounting.Usage == nil || accounting.Usage.TotalTokens != 15 {
		t.Fatalf("Accounting = %#v", accounting)
	}
}

func TestRecordingResponseClientDelegatesAccountingAndRecords(t *testing.T) {
	t.Parallel()

	base := NewScriptedResponseClient([]ScriptedResponse{{Response: openaiapi.Response{ResponseID: "response-1"}}})
	recorder, err := NewRecordingResponseClient(base)
	if err != nil {
		t.Fatalf("NewRecordingResponseClient error = %v", err)
	}
	if _, err := recorder.CreateResponse(context.Background(), "model-1", nil, nil, "", nil); err != nil {
		t.Fatalf("CreateResponse error = %v", err)
	}
	if len(recorder.Exchanges()) != 1 {
		t.Fatalf("recorded exchanges = %#v", recorder.Exchanges())
	}
	if recorder.Accounting().RequestCount != 1 {
		t.Fatalf("Accounting = %#v", recorder.Accounting())
	}
}
