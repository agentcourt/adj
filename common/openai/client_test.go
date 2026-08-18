package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jsmorph/adj/common/modelrequest"

	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

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
		{"type": "function", "name": "issue_order", "parameters": map[string]any{"type": "object"}},
	}, true)
	if err != nil {
		t.Fatalf("convertTools error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(tools))
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	text := string(raw)
	for _, needle := range []string{"issue_order", "web_search_preview"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("convertTools JSON missing %q\n%s", needle, text)
		}
	}

	explicit, err := convertTools([]map[string]any{{"type": "web_search"}}, true)
	if err != nil {
		t.Fatalf("convertTools explicit web_search error = %v", err)
	}
	if len(explicit) != 1 {
		t.Fatalf("len(explicit) = %d, want 1", len(explicit))
	}

	if _, err := convertTools([]map[string]any{{"type": "unsupported"}}, false); err == nil {
		t.Fatalf("convertTools unsupported type error = nil")
	}
}

func TestParseResponse(t *testing.T) {
	t.Parallel()

	resp := &responses.Response{
		ID: "resp_123",
		Output: []responses.ResponseOutputItemUnion{
			{
				Type: "message",
				Content: []responses.ResponseOutputMessageContentUnion{
					{Type: "output_text", Text: "hello "},
					{Type: "output_text", Text: "world"},
				},
			},
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "get_case",
				Arguments: `{"case_id":"case-1"}`,
			},
		},
	}

	got, err := parseResponse(resp)
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

	bad := &responses.Response{
		Output: []responses.ResponseOutputItemUnion{{
			Type:      "function_call",
			CallID:    "call_2",
			Name:      "bad",
			Arguments: "{",
		}},
	}
	badResp, err := parseResponse(bad)
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
