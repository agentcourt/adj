package modelgateway

import (
	"testing"

	"github.com/agentcourt/adj/common/modelrequest"
)

func TestAnthropicRequestAndContinuation(t *testing.T) {
	conversation := anthropicConversation{}
	if err := appendAnthropicInput(&conversation, []map[string]any{
		{"role": "system", "content": "Decide the case."},
		{"role": "user", "content": "Vote."},
	}, true); err != nil {
		t.Fatal(err)
	}
	effort := modelrequest.ReasoningEffortXHigh
	request, err := anthropicRequest(modelrequest.Spec{
		Model:   "claude-opus-4-8",
		Request: modelrequest.RequestParameters{ReasoningEffort: &effort},
	}, conversation, testCouncilTools())
	if err != nil {
		t.Fatal(err)
	}
	thinking := request["thinking"].(map[string]any)
	output := request["output_config"].(map[string]any)
	if thinking["type"] != "adaptive" || output["effort"] != "xhigh" {
		t.Fatalf("Anthropic thinking settings = %#v, %#v", thinking, output)
	}
	tools := request["tools"].([]map[string]any)
	if strict, ok := tools[0]["strict"].(bool); !ok || !strict {
		t.Fatalf("Anthropic tool = %#v", tools[0])
	}

	raw := []byte(`{
  "id":"msg_1",
  "model":"claude-opus-4-8",
  "content":[
    {"type":"thinking","thinking":"considered","signature":"signature-1"},
    {"type":"tool_use","id":"call_1","name":"submit_council_vote","input":{"vote":"demonstrated","rationale":"Enough."}}
  ],
  "usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4,"output_tokens_details":{"thinking_tokens":2}}
}`)
	response, assistant, err := parseAnthropicResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if response.Usage.InputTokens != 12 || response.Usage.CachedInputTokens != 3 || response.Usage.ReasoningTokens != 2 || response.Usage.TotalTokens != 19 {
		t.Fatalf("Anthropic usage = %#v", response.Usage)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].CallID != "call_1" {
		t.Fatalf("Anthropic tool calls = %#v", response.ToolCalls)
	}
	conversation.messages = append(conversation.messages, map[string]any{"role": "assistant", "content": assistant})
	if err := appendAnthropicInput(&conversation, []map[string]any{
		{"type": "function_call_output", "call_id": "call_1", "output": `{"accepted":true}`},
		{"type": "function_call_output", "call_id": "call_2", "output": `{"accepted":true}`},
	}, false); err != nil {
		t.Fatal(err)
	}
	last := conversation.messages[len(conversation.messages)-1]
	results := last["content"].([]map[string]any)
	if last["role"] != "user" || len(results) != 2 {
		t.Fatalf("Anthropic tool-result message = %#v", last)
	}
	thinkingBlock := assistant[0].(map[string]any)
	if thinkingBlock["signature"] != "signature-1" {
		t.Fatalf("Anthropic thinking block = %#v", thinkingBlock)
	}
}

func TestGoogleRequestAndContinuation(t *testing.T) {
	conversation := googleConversation{toolNames: map[string]string{"call_1": "first", "call_2": "second"}}
	if err := appendGoogleInput(&conversation, []map[string]any{{"role": "user", "content": "Vote."}}, true); err != nil {
		t.Fatal(err)
	}
	effort := modelrequest.ReasoningEffortHigh
	request, err := googleRequest(modelrequest.Spec{
		Model:   "gemini-3-pro-preview",
		Request: modelrequest.RequestParameters{ReasoningEffort: &effort},
	}, conversation, testCouncilTools())
	if err != nil {
		t.Fatal(err)
	}
	generation := request["generationConfig"].(map[string]any)
	thinking := generation["thinkingConfig"].(map[string]any)
	if thinking["thinkingLevel"] != "HIGH" {
		t.Fatalf("Google thinking settings = %#v", thinking)
	}
	declarations := request["tools"].([]map[string]any)[0]["functionDeclarations"].([]map[string]any)
	schema := declarations[0]["parametersJsonSchema"].(map[string]any)
	if schema["additionalProperties"] != false {
		t.Fatalf("Google tool schema = %#v", schema)
	}

	raw := []byte(`{
  "responseId":"response-1",
  "modelVersion":"gemini-3-pro-preview",
  "candidates":[{"content":{"role":"model","parts":[
    {"thought":true,"text":"considered","thoughtSignature":"signature-1"},
    {"functionCall":{"id":"call_3","name":"submit_council_vote","args":{"vote":"not_demonstrated","rationale":"Insufficient."}},"thoughtSignature":"signature-2"}
  ]}}],
  "usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4,"thoughtsTokenCount":3,"totalTokenCount":17}
}`)
	response, content, names, err := parseGoogleResponse(raw, "gemini-3-pro-preview")
	if err != nil {
		t.Fatal(err)
	}
	if response.Usage.ReasoningTokens != 3 || response.Usage.TotalTokens != 17 {
		t.Fatalf("Google usage = %#v", response.Usage)
	}
	if len(response.ToolCalls) != 1 || names["call_3"] != "submit_council_vote" {
		t.Fatalf("Google tool calls = %#v, names = %#v", response.ToolCalls, names)
	}
	parts := content["parts"].([]any)
	thought := parts[0].(map[string]any)
	if thought["thoughtSignature"] != "signature-1" {
		t.Fatalf("Google thought part = %#v", thought)
	}

	if err := appendGoogleInput(&conversation, []map[string]any{
		{"type": "function_call_output", "call_id": "call_1", "output": `{"accepted":true}`},
		{"type": "function_call_output", "call_id": "call_2", "output": `{"accepted":false}`},
	}, false); err != nil {
		t.Fatal(err)
	}
	last := conversation.contents[len(conversation.contents)-1]
	responses := last["parts"].([]map[string]any)
	if last["role"] != "user" || len(responses) != 2 {
		t.Fatalf("Google function-response content = %#v", last)
	}
}

func TestDeepSeekRequestAndResponse(t *testing.T) {
	effort := modelrequest.ReasoningEffortHigh
	request, strict, err := deepSeekRequest(modelrequest.Spec{
		Model:   "deepseek-chat",
		Request: modelrequest.RequestParameters{ReasoningEffort: &effort},
	}, deepSeekConversation{messages: []map[string]any{{"role": "user", "content": "Vote."}}}, testCouncilTools())
	if err != nil {
		t.Fatal(err)
	}
	thinking, _ := request["thinking"].(map[string]any)
	if request["reasoning_effort"] != "high" || thinking["type"] != "enabled" || !strict {
		t.Fatalf("DeepSeek request = %#v, strict = %v", request, strict)
	}
	xhigh := modelrequest.ReasoningEffortXHigh
	request, _, err = deepSeekRequest(modelrequest.Spec{Request: modelrequest.RequestParameters{ReasoningEffort: &xhigh}}, deepSeekConversation{}, nil)
	if err != nil || request["reasoning_effort"] != "high" {
		t.Fatalf("DeepSeek xhigh mapping = %#v, %v", request, err)
	}
	none := modelrequest.ReasoningEffortNone
	request, _, err = deepSeekRequest(modelrequest.Spec{Request: modelrequest.RequestParameters{ReasoningEffort: &none}}, deepSeekConversation{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	thinking, _ = request["thinking"].(map[string]any)
	if thinking["type"] != "disabled" || request["reasoning_effort"] != nil {
		t.Fatalf("DeepSeek disabled thinking = %#v", request)
	}

	raw := []byte(`{
  "id":"chat-1",
  "model":"deepseek-chat",
  "choices":[{"message":{"role":"assistant","content":null,"reasoning_content":"considered","tool_calls":[{"id":"call_1","type":"function","function":{"name":"submit_council_vote","arguments":"{\"vote\":\"demonstrated\",\"rationale\":\"Enough.\"}"}}]}}],
  "usage":{"prompt_tokens":10,"completion_tokens":7,"total_tokens":17,"completion_tokens_details":{"reasoning_tokens":3}}
}`)
	response, assistant, err := parseDeepSeekResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ToolCalls) != 1 || response.Usage.ReasoningTokens != 3 {
		t.Fatalf("DeepSeek response = %#v", response)
	}
	if assistant["reasoning_content"] != "considered" {
		t.Fatalf("DeepSeek assistant state = %#v", assistant)
	}
}

func TestRequestSpecRejectsUnsupportedRoutingAndCredentialHeaders(t *testing.T) {
	provider := &modelrequest.ProviderConstraints{Only: []string{"anthropic"}}
	if err := validateRequestSpec(modelrequest.Spec{Endpoint: "anthropic", Model: "claude-opus-4-8", Provider: provider}); err == nil {
		t.Fatal("Anthropic request accepted OpenRouter routing constraints")
	}
	if err := validateRequestSpec(modelrequest.Spec{Endpoint: "openai", Model: "gpt-5", Headers: map[string]string{"Authorization": "Bearer other"}}); err == nil {
		t.Fatal("OpenAI request accepted a credential header override")
	}
	if err := validateRequestSpec(modelrequest.Spec{Endpoint: "openrouter", Model: "openai/gpt-5", Provider: provider}); err != nil {
		t.Fatalf("OpenRouter request: %v", err)
	}
}

func testCouncilTools() []map[string]any {
	return []map[string]any{{
		"type":        "function",
		"name":        "submit_council_vote",
		"description": "Submit one vote.",
		"strict":      true,
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"vote": map[string]any{"type": "string"},
			},
			"required":             []any{"vote"},
			"additionalProperties": false,
		},
	}}
}
