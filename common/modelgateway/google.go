package modelgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/modelapi"
	"github.com/agentcourt/adj/common/modelrequest"
)

const googleGenerateContentBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

type googleConversation struct {
	system    []map[string]any
	contents  []map[string]any
	toolNames map[string]string
}

type googleClient struct {
	apiKey string
	http   rawHTTPClient

	mu     sync.Mutex
	states map[string]googleConversation
}

func newGoogleClient(apiKey string, timeout time.Duration, maxAttempts int) (*googleClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorAuthentication,
			Err:   fmt.Errorf("GEMINI_API_KEY is required for google models"),
		}
	}
	return &googleClient{
		apiKey: apiKey,
		http:   newRawHTTPClient(timeout, maxAttempts),
		states: map[string]googleConversation{},
	}, nil
}

func (c *googleClient) CreateResponseWithRequestSpec(
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
	if err := appendGoogleInput(&conversation, inputItems, previousResponseID == ""); err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	request, err := googleRequest(spec, conversation, tools)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	headers := make(http.Header)
	headers.Set("x-goog-api-key", c.apiKey)
	endpoint := googleGenerateContentBaseURL + url.PathEscape(spec.UpstreamModel()) + ":generateContent"
	raw, err := c.http.postJSON(ctx, endpoint, headers, request)
	if err != nil {
		return modelapi.Response{}, err
	}
	response, content, toolNames, err := parseGoogleResponse(raw, spec.UpstreamModel())
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorProtocol, Err: err}
	}
	conversation.contents = append(conversation.contents, content)
	for id, name := range toolNames {
		conversation.toolNames[id] = name
	}
	c.mu.Lock()
	c.states[response.ResponseID] = conversation
	c.mu.Unlock()
	return response, nil
}

func (c *googleClient) conversation(previousResponseID string) (googleConversation, error) {
	if strings.TrimSpace(previousResponseID) == "" {
		return googleConversation{toolNames: map[string]string{}}, nil
	}
	c.mu.Lock()
	stored, ok := c.states[previousResponseID]
	c.mu.Unlock()
	if !ok {
		return googleConversation{}, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("unknown Google response %q", previousResponseID),
		}
	}
	return cloneGoogleConversation(stored)
}

func appendGoogleInput(conversation *googleConversation, inputItems []map[string]any, allowSystem bool) error {
	functionResponses := make([]map[string]any, 0)
	flushFunctionResponses := func() {
		if len(functionResponses) == 0 {
			return
		}
		conversation.contents = append(conversation.contents, map[string]any{
			"role":  "user",
			"parts": functionResponses,
		})
		functionResponses = nil
	}
	for _, item := range inputItems {
		itemType, _ := item["type"].(string)
		if itemType == "function_call_output" {
			callID, _ := item["call_id"].(string)
			output, _ := item["output"].(string)
			name := conversation.toolNames[callID]
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("Google function result refers to unknown call %q", callID)
			}
			result := any(map[string]any{"result": output})
			var decoded any
			if json.Unmarshal([]byte(output), &decoded) == nil {
				if object, ok := decoded.(map[string]any); ok {
					result = object
				}
			}
			functionResponses = append(functionResponses, map[string]any{
				"functionResponse": map[string]any{
					"id":       callID,
					"name":     name,
					"response": result,
				},
			})
			continue
		}
		flushFunctionResponses()
		role, _ := item["role"].(string)
		role = strings.ToLower(strings.TrimSpace(role))
		parts, err := googleParts(item)
		if err != nil {
			return err
		}
		switch role {
		case "system", "developer":
			if !allowSystem {
				return fmt.Errorf("continued Google request contains a system message")
			}
			conversation.system = append(conversation.system, parts...)
		case "user":
			conversation.contents = append(conversation.contents, map[string]any{"role": "user", "parts": parts})
		case "assistant":
			conversation.contents = append(conversation.contents, map[string]any{"role": "model", "parts": parts})
		default:
			return fmt.Errorf("unsupported Google message role %q", role)
		}
	}
	flushFunctionResponses()
	return nil
}

func googleParts(item map[string]any) ([]map[string]any, error) {
	if raw, ok := item["content"].(string); ok {
		return []map[string]any{{"text": raw}}, nil
	}
	raw, ok := item["content_items"]
	if !ok {
		return nil, fmt.Errorf("Google message requires content or content_items")
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
			out = append(out, map[string]any{"text": text})
		case "input_image":
			imageURL, _ := item["image_url"].(string)
			data, err := parseDataURL(imageURL)
			if err != nil {
				return nil, fmt.Errorf("Google input image: %w", err)
			}
			out = append(out, map[string]any{"inlineData": map[string]any{"mimeType": data.mediaType, "data": data.data}})
		case "input_file":
			fileData, _ := item["file_data"].(string)
			data, err := parseDataURL(fileData)
			if err != nil {
				return nil, fmt.Errorf("Google input document: %w", err)
			}
			out = append(out, map[string]any{"inlineData": map[string]any{"mimeType": data.mediaType, "data": data.data}})
		default:
			return nil, fmt.Errorf("unsupported Google content item %q", itemType)
		}
	}
	return out, nil
}

func googleRequest(spec modelrequest.Spec, conversation googleConversation, tools []map[string]any) (map[string]any, error) {
	request := map[string]any{"contents": conversation.contents}
	if len(conversation.system) > 0 {
		request["systemInstruction"] = map[string]any{"parts": conversation.system}
	}
	generation := map[string]any{}
	if spec.Request.Temperature != nil {
		generation["temperature"] = *spec.Request.Temperature
	}
	if spec.Request.TopP != nil {
		generation["topP"] = *spec.Request.TopP
	}
	if maxTokens := spec.MaxOutputTokens(); maxTokens != nil {
		generation["maxOutputTokens"] = *maxTokens
	}
	if effort := spec.ReasoningEffort(); effort != "" {
		switch effort {
		case "minimal", "low", "medium", "high":
			generation["thinkingConfig"] = map[string]any{"thinkingLevel": strings.ToUpper(effort)}
		default:
			return nil, fmt.Errorf("Google does not support reasoning effort %q through the direct API", effort)
		}
	}
	if len(generation) > 0 {
		request["generationConfig"] = generation
	}
	declarations, err := googleTools(tools)
	if err != nil {
		return nil, err
	}
	if len(declarations) > 0 {
		request["tools"] = []map[string]any{{"functionDeclarations": declarations}}
	}
	return request, nil
}

func googleTools(tools []map[string]any) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		if toolType != "function" {
			return nil, fmt.Errorf("Google juror supports function tools; got %q", toolType)
		}
		name, _ := tool["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("function tool requires a name")
		}
		parameters, err := copyObject(tool["parameters"])
		if err != nil {
			return nil, fmt.Errorf("Google tool %s parameters: %w", name, err)
		}
		converted := map[string]any{"name": name, "parametersJsonSchema": parameters}
		if description, _ := tool["description"].(string); strings.TrimSpace(description) != "" {
			converted["description"] = description
		}
		out = append(out, converted)
	}
	return out, nil
}

func parseGoogleResponse(raw []byte, requestedModel string) (modelapi.Response, map[string]any, map[string]string, error) {
	var envelope struct {
		ResponseID   string `json:"responseId"`
		ModelVersion string `json:"modelVersion"`
		Candidates   []struct {
			Content map[string]any `json:"content"`
		} `json:"candidates"`
		Usage map[string]any `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return modelapi.Response{}, nil, nil, fmt.Errorf("decode Google response: %w", err)
	}
	if len(envelope.Candidates) != 1 {
		return modelapi.Response{}, nil, nil, fmt.Errorf("Google response contains %d candidates; expected one", len(envelope.Candidates))
	}
	content := envelope.Candidates[0].Content
	parts, err := contentItems(content["parts"])
	if err != nil {
		return modelapi.Response{}, nil, nil, fmt.Errorf("decode Google response parts: %w", err)
	}
	responseID := strings.TrimSpace(envelope.ResponseID)
	if responseID == "" {
		responseID, err = newResponseID("google-")
		if err != nil {
			return modelapi.Response{}, nil, nil, err
		}
	}
	response := modelapi.Response{
		ResponseID:    responseID,
		ReturnedModel: envelope.ModelVersion,
		RawJSON:       string(raw),
	}
	if response.ReturnedModel == "" {
		response.ReturnedModel = requestedModel
	}
	toolNames := map[string]string{}
	text := make([]string, 0)
	for index, part := range parts {
		if value, ok := part["text"].(string); ok {
			if thought, _ := part["thought"].(bool); !thought {
				text = append(text, value)
			}
		}
		call, ok := part["functionCall"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := call["name"].(string)
		if strings.TrimSpace(name) == "" {
			return modelapi.Response{}, nil, nil, fmt.Errorf("Google function call requires a name")
		}
		callID, _ := call["id"].(string)
		if strings.TrimSpace(callID) == "" {
			callID = fmt.Sprintf("%s-%d", responseID, index)
		}
		arguments, ok := call["args"].(map[string]any)
		if !ok {
			return modelapi.Response{}, nil, nil, fmt.Errorf("Google function call %q args are not an object", name)
		}
		rawArguments, err := json.Marshal(arguments)
		if err != nil {
			return modelapi.Response{}, nil, nil, err
		}
		response.ToolCalls = append(response.ToolCalls, modelapi.ToolCall{
			CallID:       callID,
			Name:         name,
			Arguments:    arguments,
			RawArguments: string(rawArguments),
		})
		toolNames[callID] = name
	}
	response.Text = strings.Join(text, "")
	input, inputOK := int64Value(envelope.Usage["promptTokenCount"])
	output, outputOK := int64Value(envelope.Usage["candidatesTokenCount"])
	total, totalOK := int64Value(envelope.Usage["totalTokenCount"])
	if inputOK || outputOK || totalOK {
		cached, _ := int64Value(envelope.Usage["cachedContentTokenCount"])
		reasoning, _ := int64Value(envelope.Usage["thoughtsTokenCount"])
		if !totalOK {
			total = input + output + reasoning
		}
		response.UsageKnown = true
		response.Usage = modelapi.Usage{
			InputTokens:       input,
			CachedInputTokens: cached,
			OutputTokens:      output,
			ReasoningTokens:   reasoning,
			TotalTokens:       total,
		}
	}
	return response, content, toolNames, nil
}

func cloneGoogleConversation(value googleConversation) (googleConversation, error) {
	wire, err := json.Marshal(struct {
		System    []map[string]any  `json:"system"`
		Contents  []map[string]any  `json:"contents"`
		ToolNames map[string]string `json:"tool_names"`
	}{value.system, value.contents, value.toolNames})
	if err != nil {
		return googleConversation{}, err
	}
	var clone struct {
		System    []map[string]any  `json:"system"`
		Contents  []map[string]any  `json:"contents"`
		ToolNames map[string]string `json:"tool_names"`
	}
	if err := json.Unmarshal(wire, &clone); err != nil {
		return googleConversation{}, err
	}
	return googleConversation{system: clone.System, contents: clone.Contents, toolNames: clone.ToolNames}, nil
}
