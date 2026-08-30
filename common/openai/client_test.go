package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/common/modelrequest"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestNewRejectsMissingConfig(t *testing.T) {
	t.Parallel()

	if _, err := New("", "https://api.openai.com/v1", false, time.Second); err == nil {
		t.Fatalf("New missing api key error = nil, want error")
	}
	if _, err := New("key", "", false, time.Second); err == nil {
		t.Fatalf("New missing base URL error = nil, want error")
	}
}

func TestClientRecordsLogicalProviderRequest(t *testing.T) {
	client, err := New("key", "https://provider.test/v1", false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
  "id":"resp-1",
  "object":"response",
  "status":"completed",
  "output":[],
  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15,"cost":0.125}
}`)),
		}, nil
	})
	client.client = openaisdk.NewClient(
		option.WithAPIKey("key"),
		option.WithBaseURL("https://provider.test/v1"),
		option.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if _, err := client.CreateResponse(context.Background(), "model", []map[string]any{{"role": "user", "content": "test"}}, nil, "", nil); err != nil {
		t.Fatal(err)
	}
	accounting := client.Accounting()
	if accounting.RequestCount != 1 || accounting.UsageObservedCount != 1 || accounting.Usage == nil || accounting.Usage.TotalTokens != 15 || accounting.CostObservedCount != 1 || accounting.CostUSD == nil || *accounting.CostUSD != 0.125 {
		t.Fatalf("accounting = %#v", accounting)
	}
}

func TestClientRetriesOpenRouterInvalidPrompt(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	client, err := New("key", "https://openrouter.ai/api/v1", false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetMaxAttempts(2); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"invalid_prompt","message":"Invalid Responses API request"}}`)),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
  "id":"resp-2",
  "object":"response",
  "status":"completed",
  "output":[],
  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
}`)),
			Request: request,
		}, nil
	})
	client.client = openaisdk.NewClient(
		option.WithAPIKey("key"),
		option.WithBaseURL("https://openrouter.ai/api/v1"),
		option.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if _, err := client.CreateResponse(context.Background(), "model", []map[string]any{{"role": "user", "content": "test"}}, nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestClientExhaustsOpenRouterInvalidPromptAttempts(t *testing.T) {
	client, err := New("key", "https://openrouter.ai/api/v1", false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetMaxAttempts(2); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"invalid_prompt","message":"Invalid Responses API request"}}`)),
			Request:    request,
		}, nil
	})
	client.client = openaisdk.NewClient(
		option.WithAPIKey("key"),
		option.WithBaseURL("https://openrouter.ai/api/v1"),
		option.WithHTTPClient(&http.Client{Transport: transport}),
	)
	_, err = client.CreateResponse(context.Background(), "model", []map[string]any{{"role": "user", "content": "test"}}, nil, "", nil)
	if err == nil {
		t.Fatal("CreateResponse error = nil")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if got := ErrorClass(err); got != ProviderErrorTransient {
		t.Fatalf("ErrorClass = %q, want %q", got, ProviderErrorTransient)
	}
	for _, text := range []string{"400 Bad Request", `"code":"invalid_prompt"`} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("error %q omits %q", err, text)
		}
	}
}

func TestNewParsesDefaultTemperatureFromEnv(t *testing.T) {
	t.Setenv("OPENAI_TEMPERATURE", "0.7")

	client, err := New("key", "https://api.openai.com/v1", false, time.Second)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if client.defaultTemperature == nil || *client.defaultTemperature != 0.7 {
		t.Fatalf("defaultTemperature = %v, want 0.7", client.defaultTemperature)
	}
}

func TestNewFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing keys",
			env:     map[string]string{},
			wantErr: "OPENAI_API_KEY or OPENROUTER_API_KEY is required",
		},
		{
			name: "openai key",
			env:  map[string]string{"OPENAI_API_KEY": "oa-key"},
		},
		{
			name: "openrouter key",
			env:  map[string]string{"OPENROUTER_API_KEY": "or-key"},
		},
		{
			name: "explicit base url",
			env:  map[string]string{"OPENAI_API_KEY": "oa-key", "OPENAI_BASE_URL": "https://proxy.local/v1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "OPENAI_BASE_URL", "OPENAI_TEMPERATURE"} {
				t.Setenv(name, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			client, err := NewFromEnv(false, time.Second)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NewFromEnv error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NewFromEnv error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewFromEnv error = %v", err)
			}
			if client == nil {
				t.Fatalf("NewFromEnv returned nil client")
			}
		})
	}
}

func TestNewForEndpointUsesOpenAIBaseURL(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("OPENAI_BASE_URL", "https://proxy.local/v1")

	client, err := NewForEndpoint("openai", false, time.Second)
	if err != nil {
		t.Fatalf("NewForEndpoint error = %v", err)
	}
	if client.baseURL != "https://proxy.local/v1" {
		t.Fatalf("baseURL = %q, want explicit OPENAI_BASE_URL", client.baseURL)
	}
}

func TestConvertInputItemsSupportsMessagesAndToolOutputs(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"role": "system", "content": "Federal rules apply."},
		{"role": "user", "content_items": []any{
			map[string]any{"type": "input_text", "text": "Review the confession."},
			map[string]any{"type": "input_file", "file_id": "file_123", "filename": "confession.txt"},
		}},
		{"type": "function_call_output", "call_id": "call_1", "output": "done"},
	}

	converted, err := convertInputItems(items)
	if err != nil {
		t.Fatalf("convertInputItems error = %v", err)
	}
	raw, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	text := string(raw)
	for _, needle := range []string{"Federal rules apply.", "Review the confession.", "file_123", "call_1", "done"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("convertInputItems JSON missing %q\n%s", needle, text)
		}
	}
}

func TestConvertInputItemsRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item []map[string]any
		want string
	}{
		{
			name: "missing role",
			item: []map[string]any{{"content": "hello"}},
			want: "unsupported input item shape",
		},
		{
			name: "bad content type",
			item: []map[string]any{{"role": "user", "content": 7}},
			want: "input item content must be string",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := convertInputItems(tt.item)
			if err == nil {
				t.Fatalf("convertInputItems error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("convertInputItems error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestConvertContentItems(t *testing.T) {
	t.Parallel()

	content, err := convertContentItems([]any{
		map[string]any{"type": "input_text", "text": "hello"},
		map[string]any{"type": "input_image", "image_url": "https://example.com/image.png"},
		map[string]any{"type": "input_file", "file_id": "file_123", "filename": "confession.txt"},
	})
	if err != nil {
		t.Fatalf("convertContentItems error = %v", err)
	}
	raw, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	text := string(raw)
	for _, needle := range []string{"hello", "https://example.com/image.png", "file_123", "confession.txt"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("convertContentItems JSON missing %q\n%s", needle, text)
		}
	}

	if _, err := convertContentItems([]any{map[string]any{"type": "unknown"}}); err == nil {
		t.Fatalf("convertContentItems unsupported type error = nil")
	}
}

func TestConvertTools(t *testing.T) {
	t.Parallel()

	tools, err := convertTools([]map[string]any{
		{"type": "function", "name": "issue_order", "description": "Issue the selected order.", "parameters": map[string]any{"type": "object"}, "strict": true},
		{"type": "function", "name": "read_record", "parameters": map[string]any{"type": "object"}},
	}, true, false)
	if err != nil {
		t.Fatalf("convertTools error = %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("len(tools) = %d, want 3", len(tools))
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	text := string(raw)
	for _, needle := range []string{"issue_order", "Issue the selected order.", "read_record", `"strict":true`, `"strict":false`, `"type":"web_search"`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("convertTools JSON missing %q\n%s", needle, text)
		}
	}
	if strings.Contains(text, "web_search_preview") {
		t.Fatalf("convertTools used legacy web search\n%s", text)
	}
	params, err := json.Marshal(responseParams("test-model", nil, tools, "", nil, nil, nil, nil))
	if err != nil {
		t.Fatalf("marshal response params: %v", err)
	}
	if !strings.Contains(string(params), `"include":["web_search_call.action.sources"]`) {
		t.Fatalf("response params omit web search sources\n%s", params)
	}

	explicit, err := convertTools([]map[string]any{{"type": "web_search"}}, false, false)
	if err != nil {
		t.Fatalf("convertTools explicit web_search error = %v", err)
	}
	if len(explicit) != 1 {
		t.Fatalf("len(explicit) = %d, want 1", len(explicit))
	}

	openRouter, err := convertTools([]map[string]any{
		{"type": "function", "name": "issue_order", "parameters": map[string]any{"type": "object"}},
		{"type": "web_search"},
	}, false, true)
	if err != nil {
		t.Fatalf("convertTools OpenRouter error = %v", err)
	}
	openRouterWire, err := json.Marshal(responseParams("openai/gpt-4.1", nil, openRouter, "", nil, nil, nil, nil))
	if err != nil {
		t.Fatalf("marshal OpenRouter response params: %v", err)
	}
	if !strings.Contains(string(openRouterWire), `"type":"openrouter:web_search"`) || strings.Contains(string(openRouterWire), `"include"`) {
		t.Fatalf("OpenRouter response params use the wrong web-search form\n%s", openRouterWire)
	}

	if _, err := convertTools([]map[string]any{{"type": "unsupported"}}, false, false); err == nil {
		t.Fatalf("convertTools unsupported type error = nil")
	}
}

func TestParseResponse(t *testing.T) {
	t.Parallel()

	var resp responses.Response
	if err := json.Unmarshal([]byte(`{
  "id":"resp_123",
  "output":[
    {
      "type":"web_search_call",
      "id":"ws_1",
      "status":"completed",
      "action":{
        "type":"search",
        "queries":["current fact"],
        "sources":[{"type":"url","url":"https://example.test/source"}]
      }
    },
    {
      "type":"message",
      "content":[
        {
          "type":"output_text",
          "text":"hello ",
          "annotations":[{
            "type":"url_citation",
            "url":"https://example.test/source",
            "title":"Example source",
            "start_index":0,
            "end_index":6
          }]
        },
        {"type":"output_text","text":"world","annotations":[]}
      ]
    },
    {
      "type":"function_call",
      "call_id":"call_1",
      "name":"get_case",
      "arguments":"{\"case_id\":\"case-1\"}"
    }
  ]
}`), &resp); err != nil {
		t.Fatal(err)
	}

	got, err := parseResponse(&resp)
	if err != nil {
		t.Fatalf("parseResponse error = %v", err)
	}
	if got.ResponseID != "resp_123" || got.Text != "hello world" {
		t.Fatalf("parseResponse = %+v", got)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Arguments["case_id"] != "case-1" {
		t.Fatalf("ToolCalls = %+v", got.ToolCalls)
	}
	if got.ToolCalls[0].RawArguments != `{"case_id":"case-1"}` {
		t.Fatalf("RawArguments = %q", got.ToolCalls[0].RawArguments)
	}
	if got.ToolCalls[0].ArgumentsError != "" {
		t.Fatalf("ArgumentsError = %q", got.ToolCalls[0].ArgumentsError)
	}
	if len(got.WebSearchCalls) != 1 || got.WebSearchCalls[0].ID != "ws_1" || len(got.WebSearchCalls[0].Queries) != 1 || got.WebSearchCalls[0].Queries[0] != "current fact" {
		t.Fatalf("WebSearchCalls = %+v", got.WebSearchCalls)
	}
	if len(got.WebSearchCalls[0].Sources) != 1 || got.WebSearchCalls[0].Sources[0].URL != "https://example.test/source" {
		t.Fatalf("WebSearchCalls sources = %+v", got.WebSearchCalls[0].Sources)
	}
	if len(got.URLCitations) != 1 || got.URLCitations[0].Title != "Example source" || got.URLCitations[0].EndIndex != 6 {
		t.Fatalf("URLCitations = %+v", got.URLCitations)
	}

	var bad responses.Response
	if err := json.Unmarshal([]byte(`{
  "output":[{
    "type":"function_call",
    "call_id":"call_2",
    "name":"bad",
    "arguments":"{"
  }]
}`), &bad); err != nil {
		t.Fatal(err)
	}
	badResp, err := parseResponse(&bad)
	if err != nil {
		t.Fatalf("parseResponse bad arguments error = %v", err)
	}
	if len(badResp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(badResp.ToolCalls))
	}
	if badResp.ToolCalls[0].Arguments != nil {
		t.Fatalf("Arguments = %#v, want nil", badResp.ToolCalls[0].Arguments)
	}
	if badResp.ToolCalls[0].RawArguments != "{" {
		t.Fatalf("RawArguments = %q, want {", badResp.ToolCalls[0].RawArguments)
	}
	if badResp.ToolCalls[0].ArgumentsError == "" {
		t.Fatalf("ArgumentsError = empty, want parse error")
	}
}

func TestParseResponseRetainsUsageAndInlineCost(t *testing.T) {
	t.Parallel()

	var response responses.Response
	if err := json.Unmarshal([]byte(`{
  "id":"gen-1",
  "output":[],
  "usage":{
    "input_tokens":386,
    "input_tokens_details":{"cached_tokens":17},
    "output_tokens":445,
    "output_tokens_details":{"reasoning_tokens":334},
    "total_tokens":831,
    "cost":0.0001018248
  }
}`), &response); err != nil {
		t.Fatal(err)
	}
	got, err := parseResponse(&response)
	if err != nil {
		t.Fatal(err)
	}
	wantUsage := Usage{InputTokens: 386, CachedInputTokens: 17, OutputTokens: 445, ReasoningTokens: 334, TotalTokens: 831}
	if got.Usage != wantUsage {
		t.Fatalf("usage = %+v, want %+v", got.Usage, wantUsage)
	}
	if usage := got.TokenUsage(); usage == nil || *usage != wantUsage {
		t.Fatalf("TokenUsage = %+v, want %+v", usage, wantUsage)
	}
	if cost := got.CostUSD(); cost == nil || *cost != 0.0001018248 {
		t.Fatalf("cost = %v, want 0.0001018248", cost)
	}
}

func TestResponseParamsSetsMaxOutputTokens(t *testing.T) {
	t.Parallel()

	maxOutputTokens := int64(800)
	params := responseParams("openai://gpt-5", nil, nil, "", nil, nil, &maxOutputTokens, nil)
	wire, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	text := string(wire)
	if !strings.Contains(text, `"max_output_tokens":800`) {
		t.Fatalf("responseParams JSON missing max_output_tokens:\n%s", text)
	}
}

func TestResponseParamsSetsReasoningEffort(t *testing.T) {
	t.Parallel()

	effort := modelrequest.ReasoningEffortXHigh
	spec := modelrequest.Spec{
		Endpoint: "openai",
		Model:    "gpt-5.4-mini",
		Request:  modelrequest.RequestParameters{ReasoningEffort: &effort},
	}
	params := responseParams(spec.RuntimeModel(), nil, nil, "", nil, nil, nil, &spec)
	wire, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if !strings.Contains(string(wire), `"reasoning":{"effort":"xhigh"}`) {
		t.Fatalf("responseParams JSON missing reasoning effort:\n%s", wire)
	}

	params = responseParams("openai://gpt-5.4-mini", nil, nil, "", nil, nil, nil, nil)
	wire, err = json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal without spec error = %v", err)
	}
	if strings.Contains(string(wire), `"reasoning"`) {
		t.Fatalf("responseParams JSON includes unrequested reasoning effort:\n%s", wire)
	}
}

func TestResponseParamsSetsMaxToolCalls(t *testing.T) {
	t.Parallel()

	maxToolCalls := int64(8)
	spec := modelrequest.Spec{
		Endpoint: "openai",
		Model:    "gpt-5.4-mini",
		Request:  modelrequest.RequestParameters{MaxToolCalls: &maxToolCalls},
	}
	params := responseParams(spec.RuntimeModel(), nil, nil, "", nil, nil, nil, &spec)
	wire, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if !strings.Contains(string(wire), `"max_tool_calls":8`) {
		t.Fatalf("responseParams JSON missing max_tool_calls:\n%s", wire)
	}

	params = responseParams("openai://gpt-5.4-mini", nil, nil, "", nil, nil, nil, nil)
	wire, err = json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal without spec error = %v", err)
	}
	if strings.Contains(string(wire), `"max_tool_calls"`) {
		t.Fatalf("responseParams JSON includes unrequested max_tool_calls:\n%s", wire)
	}
}

func TestResponseParamsAddsOpenRouterProviderSpec(t *testing.T) {
	t.Parallel()

	spec, err := modelrequest.ParseJSON([]byte(`{
		"openrouter_model_id":"deepseek/deepseek-v4-flash",
		"endpoint_tag":"deepinfra/fp4",
		"quantization":"fp4",
		"request":{"temperature":0,"top_p":1,"max_tokens":32},
		"persona":"p.txt"
	}`))
	if err != nil {
		t.Fatalf("ParseJSON error = %v", err)
	}
	params := responseParams(spec.RuntimeModel(), nil, nil, "", spec.Request.Temperature, nil, spec.MaxOutputTokens(), &spec)
	wire, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	text := string(wire)
	for _, needle := range []string{
		`"model":"openrouter://deepseek/deepseek-v4-flash"`,
		`"temperature":0`,
		`"top_p":1`,
		`"max_output_tokens":32`,
		`"provider"`,
		`"only":["deepinfra/fp4"]`,
		`"allow_fallbacks":false`,
		`"require_parameters":true`,
		`"quantizations":["fp4"]`,
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("responseParams JSON missing %q:\n%s", needle, text)
		}
	}
}

func TestToMessageRole(t *testing.T) {
	t.Parallel()

	for _, role := range []string{"user", "assistant", "system", "developer"} {
		if _, err := toMessageRole(strings.ToUpper(role)); err != nil {
			t.Fatalf("toMessageRole(%q) error = %v", role, err)
		}
	}
	if _, err := toMessageRole("judge"); err == nil {
		t.Fatalf("toMessageRole unsupported role error = nil")
	}
}

func TestRetryHelpers(t *testing.T) {
	t.Parallel()

	client := &Client{retryDelays: []time.Duration{0, 10 * time.Millisecond}}
	apiErr := &openaisdk.Error{
		StatusCode: 429,
		Request:    &http.Request{Method: http.MethodPost, URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/v1/responses"}},
		Response:   &http.Response{StatusCode: 429},
	}
	if !client.shouldRetry(apiErr, 0, 3) {
		t.Fatalf("shouldRetry apiErr = false, want true")
	}
	if retryStatusCode(apiErr) != "429" {
		t.Fatalf("retryStatusCode = %q", retryStatusCode(apiErr))
	}
	invalidPrompt := &openaisdk.Error{
		Code:       "invalid_prompt",
		Message:    "Invalid Responses API request",
		StatusCode: http.StatusBadRequest,
		Request:    &http.Request{Method: http.MethodPost, URL: &url.URL{Scheme: "https", Host: "openrouter.ai", Path: "/api/v1/responses"}},
		Response:   &http.Response{StatusCode: http.StatusBadRequest},
	}
	client.baseURL = "https://openrouter.ai/api/v1"
	if !client.shouldRetry(invalidPrompt, 0, 3) {
		t.Fatal("shouldRetry OpenRouter invalid_prompt = false, want true")
	}
	if got := client.providerFailureClass(invalidPrompt); got != ProviderErrorTransient {
		t.Fatalf("providerFailureClass OpenRouter invalid_prompt = %q, want %q", got, ProviderErrorTransient)
	}
	if got := client.providerFailureClass(errors.Join(context.Canceled, invalidPrompt)); got != "" {
		t.Fatalf("providerFailureClass canceled OpenRouter invalid_prompt = %q, want empty", got)
	}
	client.baseURL = "https://api.openai.com/v1"
	if client.shouldRetry(invalidPrompt, 0, 3) {
		t.Fatal("shouldRetry OpenAI invalid_prompt = true, want false")
	}
	client.baseURL = "https://openrouter.ai/api/v1"
	invalidPrompt.Code = "invalid_request_error"
	if client.shouldRetry(invalidPrompt, 0, 3) {
		t.Fatal("shouldRetry OpenRouter invalid_request_error = true, want false")
	}
	invalidPrompt.Code = "invalid_prompt"
	invalidPrompt.Message = "Invalid request"
	if client.shouldRetry(invalidPrompt, 0, 3) {
		t.Fatal("shouldRetry OpenRouter invalid_prompt with different message = true, want false")
	}
	invalidPrompt.Message = "Invalid Responses API request"
	invalidPrompt.Request.URL.Path = "/api/v1/chat/completions"
	if client.shouldRetry(invalidPrompt, 0, 3) {
		t.Fatal("shouldRetry OpenRouter invalid_prompt outside Responses API = true, want false")
	}

	var netErr net.Error = timeoutError{}
	if !client.shouldRetry(netErr, 0, 3) {
		t.Fatalf("shouldRetry timeout error = false, want true")
	}
	if client.shouldRetry(errors.New("bad request"), 2, 3) {
		t.Fatalf("shouldRetry final attempt = true, want false")
	}
	if !client.shouldRetry(errors.New("temporary failure in name resolution"), 0, 3) {
		t.Fatalf("shouldRetry name resolution error = false, want true")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.sleepBeforeRetry(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("sleepBeforeRetry canceled error = %v, want context.Canceled", err)
	}
	if err := client.sleepBeforeRetry(context.Background(), 0); err != nil {
		t.Fatalf("sleepBeforeRetry zero delay error = %v", err)
	}
}

func TestProviderAttemptsClassAndCost(t *testing.T) {
	client := &Client{retryDelays: append([]time.Duration(nil), defaultRetryDelays...)}
	if err := client.SetMaxAttempts(1); err != nil {
		t.Fatalf("SetMaxAttempts error = %v", err)
	}
	if len(client.retryDelays) != 0 {
		t.Fatalf("retryDelays length = %d, want 0", len(client.retryDelays))
	}
	if err := client.SetMaxAttempts(0); err == nil {
		t.Fatal("SetMaxAttempts accepted zero attempts")
	}
	if got := providerFailureClass(context.Canceled); got != "" {
		t.Fatalf("providerFailureClass(context.Canceled) = %q, want empty", got)
	}
	canceled := &ProviderError{Class: ProviderErrorTransient, Err: context.Canceled}
	if got := ErrorClass(canceled); got != "" {
		t.Fatalf("ErrorClass(wrapped context.Canceled) = %q, want empty", got)
	}

	request := &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/v1/responses"},
	}
	classes := map[int]ProviderErrorClass{
		http.StatusUnauthorized:        ProviderErrorAuthentication,
		http.StatusBadRequest:          ProviderErrorRequest,
		http.StatusTooManyRequests:     ProviderErrorTransient,
		http.StatusInternalServerError: ProviderErrorTransient,
	}
	for status, want := range classes {
		err := &openaisdk.Error{
			StatusCode: status,
			Request:    request,
			Response:   &http.Response{StatusCode: status},
		}
		if got := providerFailureClass(err); got != want {
			t.Errorf("providerFailureClass(%d) = %q, want %q", status, got, want)
		}
	}

	payload := map[string]any{"data": map[string]any{"total_cost": 0.0125}}
	if got, want := openRouterGenerationCost(payload), 0.0125; got != want {
		t.Fatalf("openRouterGenerationCost = %v, want %v", got, want)
	}
}

func TestOpenRouterConfigurationError(t *testing.T) {
	request := &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Scheme: "https", Host: "openrouter.ai", Path: "/api/v1/responses"},
	}
	apiErr := &openaisdk.Error{StatusCode: http.StatusUnauthorized, Request: request}
	if err := apiErr.UnmarshalJSON([]byte(`{"message":"Provider returned error","code":401,"metadata":{"provider_name":"AtlasCloud","is_byok":false}}`)); err != nil {
		t.Fatal(err)
	}
	wrapped := &ProviderError{Class: ProviderErrorAuthentication, Err: apiErr}
	if !IsOpenRouterConfigurationError(wrapped) {
		t.Fatal("shared provider error was not identified")
	}

	byokErr := &openaisdk.Error{StatusCode: http.StatusUnauthorized, Request: request}
	if err := byokErr.UnmarshalJSON([]byte(`{"message":"Provider returned error","code":401,"metadata":{"provider_name":"AtlasCloud","is_byok":true}}`)); err != nil {
		t.Fatal(err)
	}
	if IsOpenRouterConfigurationError(byokErr) {
		t.Fatal("BYOK error was identified as a shared provider error")
	}

	forbidden := &openaisdk.Error{StatusCode: http.StatusForbidden, Request: request}
	if !IsOpenRouterConfigurationError(forbidden) {
		t.Fatal("OpenRouter forbidden error was not identified")
	}
}
