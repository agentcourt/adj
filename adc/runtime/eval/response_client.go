package eval

import (
	"context"
	"fmt"
	"sync"

	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type ResponseRequest struct {
	Sequence           int                `json:"sequence"`
	Method             string             `json:"method"`
	Model              string             `json:"model,omitempty"`
	RequestSpec        *modelrequest.Spec `json:"request_spec,omitempty"`
	InputItems         []map[string]any   `json:"input_items,omitempty"`
	Tools              []map[string]any   `json:"tools,omitempty"`
	PreviousResponseID string             `json:"previous_response_id,omitempty"`
	Temperature        *float64           `json:"temperature,omitempty"`
}

type ResponseExchange struct {
	Request  ResponseRequest    `json:"request"`
	Response openaiapi.Response `json:"response"`
	Error    string             `json:"error,omitempty"`
}

type ScriptedResponse struct {
	Response openaiapi.Response
	Err      error
}

type ScriptedResponseClient struct {
	mu         sync.Mutex
	script     []ScriptedResponse
	next       int
	exchanges  []ResponseExchange
	accounting openaiapi.AccountingRecorder
}

func NewScriptedResponseClient(script []ScriptedResponse) *ScriptedResponseClient {
	copy := make([]ScriptedResponse, len(script))
	for i, step := range script {
		copy[i] = ScriptedResponse{Response: cloneEvalResponse(step.Response), Err: step.Err}
	}
	return &ScriptedResponseClient{script: copy}
}

func (c *ScriptedResponseClient) CreateResponse(
	_ context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
) (openaiapi.Response, error) {
	return c.respond(ResponseRequest{
		Method:             "model",
		Model:              model,
		InputItems:         cloneEvalMapList(inputItems),
		Tools:              cloneEvalMapList(tools),
		PreviousResponseID: previousResponseID,
		Temperature:        cloneEvalFloat64Pointer(temperature),
	})
}

func (c *ScriptedResponseClient) CreateResponseWithRequestSpec(
	_ context.Context,
	requestSpec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	specCopy := cloneEvalRequestSpec(requestSpec)
	return c.respond(ResponseRequest{
		Method:             "request_spec",
		RequestSpec:        &specCopy,
		InputItems:         cloneEvalMapList(inputItems),
		Tools:              cloneEvalMapList(tools),
		PreviousResponseID: previousResponseID,
	})
}

func (c *ScriptedResponseClient) respond(request ResponseRequest) (openaiapi.Response, error) {
	if c == nil {
		return openaiapi.Response{}, fmt.Errorf("scripted response client is nil")
	}
	c.mu.Lock()
	request.Sequence = c.next + 1
	if c.next >= len(c.script) {
		err := fmt.Errorf("scripted response client exhausted after %d requests", c.next)
		c.exchanges = append(c.exchanges, ResponseExchange{Request: request, Error: err.Error()})
		c.next++
		c.mu.Unlock()
		c.accounting.Record(openaiapi.Response{})
		return openaiapi.Response{}, err
	}
	step := c.script[c.next]
	c.next++
	response := cloneEvalResponse(step.Response)
	exchange := ResponseExchange{Request: request, Response: cloneEvalResponse(response)}
	if step.Err != nil {
		exchange.Error = step.Err.Error()
	}
	c.exchanges = append(c.exchanges, exchange)
	c.mu.Unlock()
	c.accounting.Record(response)
	return response, step.Err
}

func (c *ScriptedResponseClient) Exchanges() []ResponseExchange {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneEvalExchanges(c.exchanges)
}

func (c *ScriptedResponseClient) Remaining() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := len(c.script) - c.next
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (c *ScriptedResponseClient) Accounting() openaiapi.Accounting {
	if c == nil {
		return openaiapi.Accounting{}
	}
	return c.accounting.Snapshot()
}

type RecordingResponseClient struct {
	client    runner.ResponseClient
	mu        sync.Mutex
	exchanges []ResponseExchange
}

func NewRecordingResponseClient(client runner.ResponseClient) (*RecordingResponseClient, error) {
	if client == nil {
		return nil, fmt.Errorf("response client is nil")
	}
	return &RecordingResponseClient{client: client}, nil
}

func (c *RecordingResponseClient) CreateResponse(
	ctx context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
) (openaiapi.Response, error) {
	request := ResponseRequest{
		Method:             "model",
		Model:              model,
		InputItems:         cloneEvalMapList(inputItems),
		Tools:              cloneEvalMapList(tools),
		PreviousResponseID: previousResponseID,
		Temperature:        cloneEvalFloat64Pointer(temperature),
	}
	sequence := c.reserve(request)
	response, err := c.client.CreateResponse(ctx, model, inputItems, tools, previousResponseID, temperature)
	c.finish(sequence, response, err)
	return response, err
}

func (c *RecordingResponseClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	requestSpec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	specCopy := cloneEvalRequestSpec(requestSpec)
	request := ResponseRequest{
		Method:             "request_spec",
		RequestSpec:        &specCopy,
		InputItems:         cloneEvalMapList(inputItems),
		Tools:              cloneEvalMapList(tools),
		PreviousResponseID: previousResponseID,
	}
	sequence := c.reserve(request)
	response, err := c.client.CreateResponseWithRequestSpec(ctx, requestSpec, inputItems, tools, previousResponseID)
	c.finish(sequence, response, err)
	return response, err
}

func (c *RecordingResponseClient) reserve(request ResponseRequest) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	request.Sequence = len(c.exchanges) + 1
	c.exchanges = append(c.exchanges, ResponseExchange{Request: request})
	return request.Sequence
}

func (c *RecordingResponseClient) finish(sequence int, response openaiapi.Response, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	exchange := &c.exchanges[sequence-1]
	exchange.Response = cloneEvalResponse(response)
	if err != nil {
		exchange.Error = err.Error()
	}
}

func (c *RecordingResponseClient) Exchanges() []ResponseExchange {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneEvalExchanges(c.exchanges)
}

func (c *RecordingResponseClient) Accounting() openaiapi.Accounting {
	if c == nil || c.client == nil {
		return openaiapi.Accounting{}
	}
	return c.client.Accounting()
}

func cloneEvalExchanges(in []ResponseExchange) []ResponseExchange {
	out := make([]ResponseExchange, len(in))
	for i, exchange := range in {
		out[i] = ResponseExchange{
			Request:  cloneEvalRequest(exchange.Request),
			Response: cloneEvalResponse(exchange.Response),
			Error:    exchange.Error,
		}
	}
	return out
}

func cloneEvalRequest(in ResponseRequest) ResponseRequest {
	out := in
	out.InputItems = cloneEvalMapList(in.InputItems)
	out.Tools = cloneEvalMapList(in.Tools)
	out.Temperature = cloneEvalFloat64Pointer(in.Temperature)
	if in.RequestSpec != nil {
		copy := cloneEvalRequestSpec(*in.RequestSpec)
		out.RequestSpec = &copy
	}
	return out
}

func cloneEvalResponse(in openaiapi.Response) openaiapi.Response {
	out := in
	out.ToolCalls = make([]openaiapi.ToolCall, len(in.ToolCalls))
	for i, call := range in.ToolCalls {
		out.ToolCalls[i] = call
		out.ToolCalls[i].Arguments = cloneEvalMap(call.Arguments)
	}
	out.WebSearchCalls = make([]openaiapi.WebSearchCall, len(in.WebSearchCalls))
	for i, call := range in.WebSearchCalls {
		out.WebSearchCalls[i] = call
		out.WebSearchCalls[i].Queries = append([]string(nil), call.Queries...)
		out.WebSearchCalls[i].Sources = append([]openaiapi.WebSearchSource(nil), call.Sources...)
	}
	out.URLCitations = append([]openaiapi.URLCitation(nil), in.URLCitations...)
	out.OpenRouterMetadata = cloneEvalMap(in.OpenRouterMetadata)
	out.OpenRouterGeneration = cloneEvalMap(in.OpenRouterGeneration)
	return out
}

func cloneEvalRequestSpec(in modelrequest.Spec) modelrequest.Spec {
	out := in
	out.Headers = cloneEvalStringMap(in.Headers)
	out.VariantMetadata = cloneEvalMap(in.VariantMetadata)
	if in.Provider != nil {
		provider := *in.Provider
		provider.Only = append([]string(nil), in.Provider.Only...)
		provider.Quantizations = append([]string(nil), in.Provider.Quantizations...)
		provider.AllowFallbacks = cloneEvalBoolPointer(in.Provider.AllowFallbacks)
		provider.RequireParameters = cloneEvalBoolPointer(in.Provider.RequireParameters)
		out.Provider = &provider
	}
	out.Request.Temperature = cloneEvalFloat64Pointer(in.Request.Temperature)
	out.Request.TopP = cloneEvalFloat64Pointer(in.Request.TopP)
	out.Request.MaxTokens = cloneEvalInt64Pointer(in.Request.MaxTokens)
	out.Request.MaxOutputTokens = cloneEvalInt64Pointer(in.Request.MaxOutputTokens)
	out.Request.MaxToolCalls = cloneEvalInt64Pointer(in.Request.MaxToolCalls)
	if in.Request.ReasoningEffort != nil {
		effort := *in.Request.ReasoningEffort
		out.Request.ReasoningEffort = &effort
	}
	return out
}

func cloneEvalMapList(in []map[string]any) []map[string]any {
	if in == nil {
		return nil
	}
	out := make([]map[string]any, len(in))
	for i, item := range in {
		out[i] = cloneEvalMap(item)
	}
	return out
}

func cloneEvalMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneEvalValue(value)
	}
	return out
}

func cloneEvalValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneEvalMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneEvalValue(item)
		}
		return out
	case []map[string]any:
		return cloneEvalMapList(typed)
	case []string:
		return append([]string(nil), typed...)
	case map[string]string:
		return cloneEvalStringMap(typed)
	default:
		return typed
	}
}

func cloneEvalStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneEvalFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneEvalInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneEvalBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

var _ runner.ResponseClient = (*ScriptedResponseClient)(nil)
var _ runner.ResponseClient = (*RecordingResponseClient)(nil)
