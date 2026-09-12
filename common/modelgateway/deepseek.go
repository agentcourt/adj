package modelgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/modelapi"
	"github.com/agentcourt/adj/common/modelrequest"
)

const (
	deepSeekChatURL     = "https://api.deepseek.com/chat/completions"
	deepSeekBetaChatURL = "https://api.deepseek.com/beta/chat/completions"
)

type deepSeekConversation struct {
	messages []map[string]any
}

type deepSeekClient struct {
	apiKey string
	http   rawHTTPClient

	mu     sync.Mutex
	states map[string]deepSeekConversation
}

func newDeepSeekClient(apiKey string, timeout time.Duration, maxAttempts int) (*deepSeekClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorAuthentication,
			Err:   fmt.Errorf("DEEPSEEK_API_KEY is required for deepseek models"),
		}
	}
	return &deepSeekClient{
		apiKey: apiKey,
		http:   newRawHTTPClient(timeout, maxAttempts),
		states: map[string]deepSeekConversation{},
	}, nil
}

func (c *deepSeekClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (modelapi.Response, error) {
	conversation, err := c.conversation(previousResponseID)
	if err != nil {
		return modelapi.Response{}, err
	}
	if err := appendDeepSeekInput(&conversation, inputItems); err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	request, strictTools, err := deepSeekRequest(spec, conversation, tools)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	headers := make(http.Header)
	headers.Set("authorization", "Bearer "+c.apiKey)
	endpoint := deepSeekChatURL
	if strictTools {
		endpoint = deepSeekBetaChatURL
	}
	raw, err := c.http.postJSON(ctx, endpoint, headers, request)
	if err != nil {
		return modelapi.Response{}, err
	}
	response, assistant, err := parseDeepSeekResponse(raw)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorProtocol, Err: err}
	}
	conversation.messages = append(conversation.messages, assistant)
	c.mu.Lock()
	c.states[response.ResponseID] = conversation
	c.mu.Unlock()
	return response, nil
}

func (c *deepSeekClient) conversation(previousResponseID string) (deepSeekConversation, error) {
	if strings.TrimSpace(previousResponseID) == "" {
		return deepSeekConversation{}, nil
	}
	c.mu.Lock()
	stored, ok := c.states[previousResponseID]
	c.mu.Unlock()
	if !ok {
		return deepSeekConversation{}, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("unknown DeepSeek response %q", previousResponseID),
		}
	}
	return cloneDeepSeekConversation(stored)
}

func appendDeepSeekInput(conversation *deepSeekConversation, inputItems []map[string]any) error {
	for _, item := range inputItems {
		if itemType, _ := item["type"].(string); itemType == "function_call_output" {
			callID, _ := item["call_id"].(string)
			output, _ := item["output"].(string)
			if strings.TrimSpace(callID) == "" {
				return fmt.Errorf("function_call_output requires call_id")
			}
			conversation.messages = append(conversation.messages, map[string]any{
				"role": "tool", "tool_call_id": callID, "content": output,
			})
			continue
		}
		role, _ := item["role"].(string)
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "developer" {
			role = "system"
		}
		if role != "system" && role != "user" && role != "assistant" {
			return fmt.Errorf("unsupported DeepSeek message role %q", role)
		}
		content, err := deepSeekTextContent(item)
		if err != nil {
			return err
		}
		conversation.messages = append(conversation.messages, map[string]any{"role": role, "content": content})
	}
	return nil
}

func deepSeekTextContent(item map[string]any) (string, error) {
	if content, ok := item["content"].(string); ok {
		return content, nil
	}
	items, err := contentItems(item["content_items"])
	if err != nil {
		return "", fmt.Errorf("DeepSeek message content: %w", err)
	}
	var text strings.Builder
	for _, item := range items {
		itemType, _ := item["type"].(string)
		if itemType != "input_text" {
			return "", fmt.Errorf("DeepSeek direct chat does not support content item %q", itemType)
		}
		value, _ := item["text"].(string)
		text.WriteString(value)
	}
	return text.String(), nil
}

func deepSeekRequest(spec modelrequest.Spec, conversation deepSeekConversation, tools []map[string]any) (map[string]any, bool, error) {
	request := map[string]any{
		"model":    spec.UpstreamModel(),
		"messages": conversation.messages,
	}
	if spec.Request.Temperature != nil {
		request["temperature"] = *spec.Request.Temperature
	}
	if spec.Request.TopP != nil {
		request["top_p"] = *spec.Request.TopP
	}
	if maxTokens := spec.MaxOutputTokens(); maxTokens != nil {
		request["max_tokens"] = *maxTokens
	}
	if effort := spec.ReasoningEffort(); effort != "" {
		switch effort {
		case "none":
			request["thinking"] = map[string]any{"type": "disabled"}
		case "low", "high", "max":
			request["thinking"] = map[string]any{"type": "enabled"}
			request["reasoning_effort"] = effort
		case "medium", "xhigh":
			request["thinking"] = map[string]any{"type": "enabled"}
			request["reasoning_effort"] = "high"
		default:
			return nil, false, fmt.Errorf("DeepSeek does not support reasoning effort %q", effort)
		}
	}
	converted, strict, err := deepSeekTools(tools)
	if err != nil {
		return nil, false, err
	}
	if len(converted) > 0 {
		request["tools"] = converted
	}
	return request, strict, nil
}

func deepSeekTools(tools []map[string]any) ([]map[string]any, bool, error) {
	converted := make([]map[string]any, 0, len(tools))
	strictTools := false
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		if toolType != "function" {
			return nil, false, fmt.Errorf("DeepSeek juror supports function tools; got %q", toolType)
		}
		name, _ := tool["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, false, fmt.Errorf("function tool requires a name")
		}
		parameters, err := copyObject(tool["parameters"])
		if err != nil {
			return nil, false, fmt.Errorf("DeepSeek tool %s parameters: %w", name, err)
		}
		function := map[string]any{"name": name, "parameters": parameters}
		if description, _ := tool["description"].(string); strings.TrimSpace(description) != "" {
			function["description"] = description
		}
		if strict, ok := tool["strict"].(bool); ok {
			function["strict"] = strict
			strictTools = strictTools || strict
		}
		converted = append(converted, map[string]any{"type": "function", "function": function})
	}
	return converted, strictTools, nil
}

func parseDeepSeekResponse(raw []byte) (modelapi.Response, map[string]any, error) {
	var envelope struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message map[string]any `json:"message"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return modelapi.Response{}, nil, fmt.Errorf("decode DeepSeek response: %w", err)
	}
	if strings.TrimSpace(envelope.ID) == "" {
		return modelapi.Response{}, nil, fmt.Errorf("DeepSeek response omitted id")
	}
	if len(envelope.Choices) != 1 {
		return modelapi.Response{}, nil, fmt.Errorf("DeepSeek response contains %d choices; expected one", len(envelope.Choices))
	}
	assistant := envelope.Choices[0].Message
	response := modelapi.Response{
		ResponseID:    envelope.ID,
		ReturnedModel: envelope.Model,
		RawJSON:       string(raw),
	}
	response.Text, _ = assistant["content"].(string)
	if calls, ok := assistant["tool_calls"].([]any); ok {
		for _, rawCall := range calls {
			call, ok := rawCall.(map[string]any)
			if !ok {
				return modelapi.Response{}, nil, fmt.Errorf("DeepSeek tool call is not an object")
			}
			function, ok := call["function"].(map[string]any)
			if !ok {
				return modelapi.Response{}, nil, fmt.Errorf("DeepSeek tool call omitted function")
			}
			callID, _ := call["id"].(string)
			name, _ := function["name"].(string)
			if strings.TrimSpace(callID) == "" || strings.TrimSpace(name) == "" {
				return modelapi.Response{}, nil, fmt.Errorf("DeepSeek tool call requires id and name")
			}
			rawArguments, _ := function["arguments"].(string)
			arguments := map[string]any{}
			argumentsError := ""
			if err := json.Unmarshal([]byte(rawArguments), &arguments); err != nil {
				arguments = nil
				argumentsError = err.Error()
			}
			response.ToolCalls = append(response.ToolCalls, modelapi.ToolCall{
				CallID: callID, Name: name, Arguments: arguments, RawArguments: rawArguments, ArgumentsError: argumentsError,
			})
		}
	}
	inputTokens, inputOK := int64Value(envelope.Usage["prompt_tokens"])
	outputTokens, outputOK := int64Value(envelope.Usage["completion_tokens"])
	totalTokens, totalOK := int64Value(envelope.Usage["total_tokens"])
	if inputOK && outputOK && totalOK {
		response.UsageKnown = true
		response.Usage = modelapi.Usage{InputTokens: inputTokens, OutputTokens: outputTokens, TotalTokens: totalTokens}
		if details, ok := envelope.Usage["prompt_tokens_details"].(map[string]any); ok {
			response.Usage.CachedInputTokens, _ = int64Value(details["cached_tokens"])
		}
		if details, ok := envelope.Usage["completion_tokens_details"].(map[string]any); ok {
			response.Usage.ReasoningTokens, _ = int64Value(details["reasoning_tokens"])
		}
	}
	return response, assistant, nil
}

func cloneDeepSeekConversation(value deepSeekConversation) (deepSeekConversation, error) {
	raw, err := json.Marshal(value.messages)
	if err != nil {
		return deepSeekConversation{}, fmt.Errorf("copy DeepSeek conversation: %w", err)
	}
	var messages []map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		return deepSeekConversation{}, fmt.Errorf("copy DeepSeek conversation: %w", err)
	}
	return deepSeekConversation{messages: messages}, nil
}
