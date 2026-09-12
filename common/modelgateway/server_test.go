package modelgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentcourt/adj/common/modelapi"
	"github.com/agentcourt/adj/common/modelrequest"
)

type recordedExecution struct {
	spec               modelrequest.Spec
	input              []map[string]any
	tools              []map[string]any
	previousResponseID string
}

type fakeExecutor struct {
	responses []modelapi.Response
	calls     []recordedExecution
}

func (f *fakeExecutor) CheckEndpoint(string) error { return nil }

func (f *fakeExecutor) CreateResponseWithRequestSpec(
	_ context.Context,
	spec modelrequest.Spec,
	input []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (modelapi.Response, error) {
	f.calls = append(f.calls, recordedExecution{spec: spec, input: input, tools: tools, previousResponseID: previousResponseID})
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

func TestChatServerPreservesProviderConversation(t *testing.T) {
	executor := &fakeExecutor{responses: []modelapi.Response{
		{
			ResponseID: "response-1",
			ToolCalls: []modelapi.ToolCall{{
				CallID:       "call-1",
				Name:         "submit_council_vote",
				Arguments:    map[string]any{"verdict": "for"},
				RawArguments: `{"verdict":"for"}`,
			}},
			UsageKnown: true,
			Usage:      modelapi.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
		},
		{ResponseID: "response-2", Text: "recorded"},
	}}
	server, err := NewServer(executor)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := server.Bind("juror-1", modelrequest.Spec{Endpoint: "anthropic", Model: "model-1"})
	if err != nil {
		t.Fatal(err)
	}
	tools := []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name": "submit_council_vote",
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{"verdict": map[string]any{"type": "string"}},
			},
		},
	}}
	firstMessages := []map[string]any{
		{"role": "system", "content": "Decide the question."},
		{"role": "user", "content": "Record a vote."},
	}
	first := runChatRequest(t, server, binding, map[string]any{
		"model": binding.Model, "messages": firstMessages, "tools": tools, "stream": true,
	})
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"name":"submit_council_vote"`) || !strings.Contains(first.Body.String(), "data: [DONE]") {
		t.Fatalf("first response: status=%d body=%s", first.Code, first.Body.String())
	}
	assistant := chatAssistantMessage(modelapi.Response{
		ToolCalls: []modelapi.ToolCall{{CallID: "call-1", Name: "submit_council_vote", Arguments: map[string]any{"verdict": "for"}, RawArguments: `{"verdict":"for"}`}},
	})
	secondMessages := append(append([]map[string]any(nil), firstMessages...), assistant, map[string]any{
		"role": "tool", "tool_call_id": "call-1", "content": `{"accepted":true}`,
	})
	second := runChatRequest(t, server, binding, map[string]any{
		"model": binding.Model, "messages": secondMessages, "tools": tools,
	})
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"content":"recorded"`) {
		t.Fatalf("second response: status=%d body=%s", second.Code, second.Body.String())
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	if executor.calls[1].previousResponseID != "response-1" {
		t.Fatalf("previous response = %q", executor.calls[1].previousResponseID)
	}
	if len(executor.calls[1].input) != 3 || executor.calls[1].input[2]["type"] != "function_call_output" {
		t.Fatalf("second input = %#v", executor.calls[1].input)
	}
	if len(executor.calls[0].tools) != 1 || executor.calls[0].tools[0]["name"] != "submit_council_vote" {
		t.Fatalf("converted tools = %#v", executor.calls[0].tools)
	}
}

func TestChatServerRejectsChangedHistory(t *testing.T) {
	executor := &fakeExecutor{responses: []modelapi.Response{{ResponseID: "response-1"}}}
	server, err := NewServer(executor)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := server.Bind("juror-1", modelrequest.Spec{Endpoint: "openai", Model: "model-1"})
	if err != nil {
		t.Fatal(err)
	}
	messages := []map[string]any{{"role": "user", "content": "question"}}
	response := runChatRequest(t, server, binding, map[string]any{"model": binding.Model, "messages": messages})
	if response.Code != http.StatusOK {
		t.Fatalf("first response: %d %s", response.Code, response.Body.String())
	}
	changed := []map[string]any{{"role": "user", "content": "changed"}, {"role": "assistant", "content": ""}}
	response = runChatRequest(t, server, binding, map[string]any{"model": binding.Model, "messages": changed})
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "changed prior message 0") {
		t.Fatalf("changed response: %d %s", response.Code, response.Body.String())
	}
}

func TestChatServerRejectsToolChoice(t *testing.T) {
	executor := &fakeExecutor{responses: []modelapi.Response{{ResponseID: "response-1"}}}
	server, err := NewServer(executor)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := server.Bind("juror-1", modelrequest.Spec{Endpoint: "openai", Model: "model-1"})
	if err != nil {
		t.Fatal(err)
	}
	response := runChatRequest(t, server, binding, map[string]any{
		"model": binding.Model, "messages": []map[string]any{{"role": "user", "content": "vote"}}, "tool_choice": "auto",
	})
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "tool_choice is unsupported") {
		t.Fatalf("response: %d %s", response.Code, response.Body.String())
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %d", len(executor.calls))
	}
}

func TestChatServerUnbindRevokesToken(t *testing.T) {
	executor := &fakeExecutor{}
	server, err := NewServer(executor)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := server.Bind("juror-1", modelrequest.Spec{Endpoint: "openai", Model: "model-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Unbind(binding.Token); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer "+binding.Token)
	if _, err := server.bindingForRequest(request); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("bindingForRequest error = %v", err)
	}
}

func runChatRequest(t *testing.T, server *Server, binding Binding, value map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(raw))
	request.Header.Set("Authorization", "Bearer "+binding.Token)
	response := httptest.NewRecorder()
	server.handleChatCompletions(response, request)
	return response
}
