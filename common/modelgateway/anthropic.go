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

const anthropicMessagesURL = "https://api.anthropic.com/v1/messages"

type anthropicConversation struct {
	system   []map[string]any
	messages []map[string]any
}

type anthropicClient struct {
	apiKey string
	http   rawHTTPClient

	mu     sync.Mutex
	states map[string]anthropicConversation
}

func newAnthropicClient(apiKey string, timeout time.Duration, maxAttempts int) (*anthropicClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorAuthentication,
			Err:   fmt.Errorf("ANTHROPIC_API_KEY is required for anthropic models"),
		}
	}
	return &anthropicClient{
		apiKey: apiKey,
		http:   newRawHTTPClient(timeout, maxAttempts),
		states: map[string]anthropicConversation{},
	}, nil
}

func (c *anthropicClient) CreateResponseWithRequestSpec(
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
	if err := appendAnthropicInput(&conversation, inputItems, previousResponseID == ""); err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	request, err := anthropicRequest(spec, conversation, tools)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	headers := make(http.Header)
	headers.Set("x-api-key", c.apiKey)
	headers.Set("anthropic-version", "2023-06-01")
	raw, err := c.http.postJSON(ctx, anthropicMessagesURL, headers, request)
	if err != nil {
		return modelapi.Response{}, err
	}
	response, assistantContent, err := parseAnthropicResponse(raw)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorProtocol, Err: err}
	}
	conversation.messages = append(conversation.messages, map[string]any{
		"role":    "assistant",
		"content": assistantContent,
	})
	c.mu.Lock()
	c.states[response.ResponseID] = conversation
	c.mu.Unlock()
	return response, nil
}

func (c *anthropicClient) conversation(previousResponseID string) (anthropicConversation, error) {
	if strings.TrimSpace(previousResponseID) == "" {
		return anthropicConversation{}, nil
	}
	c.mu.Lock()
	stored, ok := c.states[previousResponseID]
	c.mu.Unlock()
	if !ok {
		return anthropicConversation{}, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("unknown Anthropic response %q", previousResponseID),
		}
	}
	return cloneAnthropicConversation(stored)
}

func appendAnthropicInput(conversation *anthropicConversation, inputItems []map[string]any, allowSystem bool) error {
	toolResults := make([]map[string]any, 0)
	flushToolResults := func() {
		if len(toolResults) == 0 {
			return
		}
		conversation.messages = append(conversation.messages, map[string]any{
			"role":    "user",
			"content": toolResults,
		})
		toolResults = nil
	}
	for _, item := range inputItems {
		itemType, _ := item["type"].(string)
		if itemType == "function_call_output" {
			callID, _ := item["call_id"].(string)
			output, _ := item["output"].(string)
			if strings.TrimSpace(callID) == "" {
				return fmt.Errorf("function_call_output requires call_id")
			}
			toolResults = append(toolResults, map[string]any{
				"type":        "tool_result",
				"tool_use_id": callID,
				"content":     output,
			})
			continue
		}
		flushToolResults()
		role, _ := item["role"].(string)
		role = strings.ToLower(strings.TrimSpace(role))
		content, err := anthropicContent(item)
		if err != nil {
			return err
		}
		switch role {
		case "system", "developer":
			if !allowSystem {
				return fmt.Errorf("continued Anthropic request contains a system message")
			}
			conversation.system = append(conversation.system, content...)
		case "user", "assistant":
			conversation.messages = append(conversation.messages, map[string]any{"role": role, "content": content})
		default:
			return fmt.Errorf("unsupported Anthropic message role %q", role)
		}
	}
	flushToolResults()
	return nil
}

func anthropicContent(item map[string]any) ([]map[string]any, error) {
	if raw, ok := item["content"].(string); ok {
		return []map[string]any{{"type": "text", "text": raw}}, nil
	}
	raw, ok := item["content_items"]
	if !ok {
		return nil, fmt.Errorf("Anthropic message requires content or content_items")
	}
	items, err := contentItems(raw)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		switch itemType, _ := item["type"].(string); itemType {
		case "input_text":
			text, _ := item["text"].(string)
			out = append(out, map[string]any{"type": "text", "text": text})
		case "input_image":
			imageURL, _ := item["image_url"].(string)
			data, err := parseDataURL(imageURL)
			if err != nil {
				return nil, fmt.Errorf("Anthropic input image: %w", err)
			}
			out = append(out, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": data.mediaType,
					"data":       data.data,
				},
			})
		case "input_file":
			fileData, _ := item["file_data"].(string)
			data, err := parseDataURL(fileData)
			if err != nil {
				return nil, fmt.Errorf("Anthropic input document: %w", err)
			}
			document := map[string]any{
				"type": "document",
				"source": map[string]any{
					"type":       "base64",
					"media_type": data.mediaType,
					"data":       data.data,
				},
			}
			if filename, _ := item["filename"].(string); strings.TrimSpace(filename) != "" {
				document["title"] = filename
			}
			out = append(out, document)
		default:
			return nil, fmt.Errorf("unsupported Anthropic content item %q", itemType)
		}
	}
	return out, nil
}

func anthropicRequest(spec modelrequest.Spec, conversation anthropicConversation, tools []map[string]any) (map[string]any, error) {
	maxTokens := int64(4096)
	if configured := spec.MaxOutputTokens(); configured != nil {
		maxTokens = *configured
	}
	request := map[string]any{
		"model":      spec.UpstreamModel(),
		"max_tokens": maxTokens,
		"messages":   conversation.messages,
	}
	if len(conversation.system) > 0 {
		request["system"] = conversation.system
	}
	if spec.Request.Temperature != nil {
		request["temperature"] = *spec.Request.Temperature
	}
	if spec.Request.TopP != nil {
		request["top_p"] = *spec.Request.TopP
	}
	if effort := spec.ReasoningEffort(); effort != "" {
		switch effort {
		case "none":
			request["thinking"] = map[string]any{"type": "disabled"}
		case "low", "medium", "high", "xhigh", "max":
			request["thinking"] = map[string]any{"type": "adaptive"}
			request["output_config"] = map[string]any{"effort": effort}
		default:
			return nil, fmt.Errorf("Anthropic does not support reasoning effort %q", effort)
		}
	}
	convertedTools, err := anthropicTools(tools)
	if err != nil {
		return nil, err
	}
	if len(convertedTools) > 0 {
		request["tools"] = convertedTools
	}
	return request, nil
}

func anthropicTools(tools []map[string]any) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		if toolType != "function" {
			return nil, fmt.Errorf("Anthropic juror supports function tools; got %q", toolType)
		}
		name, _ := tool["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("function tool requires a name")
		}
		inputSchema, err := copyObject(tool["parameters"])
		if err != nil {
			return nil, fmt.Errorf("Anthropic tool %s parameters: %w", name, err)
		}
		converted := map[string]any{"name": name, "input_schema": inputSchema}
		if description, _ := tool["description"].(string); strings.TrimSpace(description) != "" {
			converted["description"] = description
		}
		if strict, ok := tool["strict"].(bool); ok {
			converted["strict"] = strict
		}
		out = append(out, converted)
	}
	return out, nil
}

func parseAnthropicResponse(raw []byte) (modelapi.Response, []any, error) {
	var envelope struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []any  `json:"content"`
		Usage   struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			OutputTokensDetails      struct {
				ThinkingTokens int64 `json:"thinking_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return modelapi.Response{}, nil, fmt.Errorf("decode Anthropic response: %w", err)
	}
	if strings.TrimSpace(envelope.ID) == "" {
		return modelapi.Response{}, nil, fmt.Errorf("Anthropic response omitted id")
	}
	response := modelapi.Response{
		ResponseID:    envelope.ID,
		ReturnedModel: envelope.Model,
		RawJSON:       string(raw),
		UsageKnown:    true,
		Usage: modelapi.Usage{
			InputTokens:       envelope.Usage.InputTokens + envelope.Usage.CacheCreationInputTokens,
			CachedInputTokens: envelope.Usage.CacheReadInputTokens,
			OutputTokens:      envelope.Usage.OutputTokens,
			ReasoningTokens:   envelope.Usage.OutputTokensDetails.ThinkingTokens,
			TotalTokens: envelope.Usage.InputTokens + envelope.Usage.CacheCreationInputTokens +
				envelope.Usage.CacheReadInputTokens + envelope.Usage.OutputTokens,
		},
	}
	text := make([]string, 0)
	for _, rawBlock := range envelope.Content {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			return modelapi.Response{}, nil, fmt.Errorf("Anthropic response content block is not an object")
		}
		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			value, _ := block["text"].(string)
			text = append(text, value)
		case "tool_use":
			callID, _ := block["id"].(string)
			name, _ := block["name"].(string)
			if strings.TrimSpace(callID) == "" || strings.TrimSpace(name) == "" {
				return modelapi.Response{}, nil, fmt.Errorf("Anthropic tool call requires id and name")
			}
			arguments, ok := block["input"].(map[string]any)
			if !ok {
				return modelapi.Response{}, nil, fmt.Errorf("Anthropic tool call %q input is not an object", name)
			}
			rawArguments, err := json.Marshal(arguments)
			if err != nil {
				return modelapi.Response{}, nil, fmt.Errorf("encode Anthropic tool call %q: %w", name, err)
			}
			response.ToolCalls = append(response.ToolCalls, modelapi.ToolCall{
				CallID:       callID,
				Name:         name,
				Arguments:    arguments,
				RawArguments: string(rawArguments),
			})
		case "thinking", "redacted_thinking":
		default:
			return modelapi.Response{}, nil, fmt.Errorf("unsupported Anthropic response content type %q", blockType)
		}
	}
	response.Text = strings.Join(text, "")
	return response, envelope.Content, nil
}

func cloneAnthropicConversation(value anthropicConversation) (anthropicConversation, error) {
	wire, err := json.Marshal(struct {
		System   []map[string]any `json:"system"`
		Messages []map[string]any `json:"messages"`
	}{value.system, value.messages})
	if err != nil {
		return anthropicConversation{}, err
	}
	var clone struct {
		System   []map[string]any `json:"system"`
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(wire, &clone); err != nil {
		return anthropicConversation{}, err
	}
	return anthropicConversation{system: clone.System, messages: clone.Messages}, nil
}
