package proceeding

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/common/documents"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type fakeClientFactory struct {
	newCalls int
	client   ResponseClient
	err      error
	spec     modelrequest.Spec
	timeout  time.Duration
}

func (f *fakeClientFactory) New(spec modelrequest.Spec, timeout time.Duration) (ResponseClient, error) {
	f.newCalls++
	f.spec = spec
	f.timeout = timeout
	return f.client, f.err
}

type fakeResponseClient struct {
	calls      int
	response   openaiapi.Response
	err        error
	spec       modelrequest.Spec
	inputItems []map[string]any
	tools      []map[string]any
}

func (c *fakeResponseClient) CreateResponseWithRequestSpec(_ context.Context, spec modelrequest.Spec, inputItems []map[string]any, tools []map[string]any, _ string) (openaiapi.Response, error) {
	c.calls++
	c.spec = spec
	c.inputItems = inputItems
	c.tools = tools
	return c.response, c.err
}

func TestRunWritesCompleteRecord(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	documentsRoot := filepath.Join(base, "source")
	if err := os.Mkdir(documentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(documentsRoot, "facts.txt"), []byte("The observed sky was blue.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requestSpecPath := filepath.Join(base, "request.json")
	if err := os.WriteFile(requestSpecPath, []byte(`{
  "endpoint":"openai",
  "model":"test-model",
  "headers":{"Authorization":"Bearer secret-value","X-Trace":"trace-value"}
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	response := openaiapi.Response{
		Text:       "submitted",
		ResponseID: "resp-1",
		RawJSON:    `{"id":"resp-1","output":[]}`,
		ToolCalls: []openaiapi.ToolCall{{
			CallID:       "call-1",
			Name:         "submit_simple_decision",
			Arguments:    map[string]any{"decision": "demonstrated", "rationale": "The observation supports the proposition."},
			RawArguments: `{"decision":"demonstrated","rationale":"The observation supports the proposition."}`,
		}},
		OpenRouterMetadata:   map[string]any{"provider": "test"},
		OpenRouterGeneration: map[string]any{"data": map[string]any{"total_cost": 0.0125}},
		WebSearchCalls: []openaiapi.WebSearchCall{{
			ID:      "ws-1",
			Status:  "completed",
			Action:  "search",
			Queries: []string{"observed sky color"},
			Sources: []openaiapi.WebSearchSource{{Type: "url", URL: "https://example.test/sky"}},
		}},
		URLCitations: []openaiapi.URLCitation{{
			URL:        "https://example.test/sky",
			Title:      "Sky observation",
			StartIndex: 0,
			EndIndex:   10,
		}},
		Usage:               openaiapi.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		UsageKnown:          true,
		OpenRouterCostUSD:   0.0125,
		OpenRouterCostKnown: true,
	}
	client := &fakeResponseClient{response: response}
	factory := &fakeClientFactory{client: client}
	opts := testOptions(filepath.Join(base, "out"))
	opts.DocumentsRoot = documentsRoot
	opts.RequestSpecPath = requestSpecPath
	opts.Model = ""
	opts.ReasoningEffort = "xhigh"
	opts.MaxOutputTokens = 8000
	opts.MaxToolCalls = 8
	result, err := RunWithClientFactory(context.Background(), opts, factory)
	if err != nil {
		t.Fatalf("RunWithClientFactory error = %v", err)
	}
	if result.Status != "ok" || result.Decision == nil || result.Decision.Value != "demonstrated" {
		t.Fatalf("result = %#v", result)
	}
	if result.Provider.RequestCount != 1 || result.Provider.UsageObservedCount != 1 || result.Provider.CostObservedCount != 1 || result.Provider.CostUSD == nil || *result.Provider.CostUSD != 0.0125 || result.ResponseID != "resp-1" {
		t.Fatalf("provider result = %#v", result)
	}
	if !result.WebSearch.Enabled || result.WebSearch.CallCount != 1 || result.WebSearch.SourceCount != 1 || result.WebSearch.CitationCount != 1 {
		t.Fatalf("web search result = %#v", result.WebSearch)
	}
	if result.ReasoningEffort != "xhigh" || result.MaxOutputTokens != 8000 || result.MaxToolCalls != 8 {
		t.Fatalf("model request result = %#v", result)
	}
	if factory.newCalls != 1 || client.calls != 1 {
		t.Fatalf("factory calls = %d, provider calls = %d, want 1, 1", factory.newCalls, client.calls)
	}
	if factory.timeout != DefaultTimeoutSeconds*time.Second {
		t.Fatalf("timeout = %s", factory.timeout)
	}
	if client.spec.MaxOutputTokens() == nil || *client.spec.MaxOutputTokens() != 8000 {
		t.Fatalf("max output tokens = %v", client.spec.MaxOutputTokens())
	}
	if client.spec.ReasoningEffort() != "xhigh" {
		t.Fatalf("reasoning effort = %q", client.spec.ReasoningEffort())
	}
	if client.spec.MaxToolCalls() == nil || *client.spec.MaxToolCalls() != 8 {
		t.Fatalf("max tool calls = %v", client.spec.MaxToolCalls())
	}
	requestWire, err := os.ReadFile(filepath.Join(opts.OutputDir, "model-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	requestText := string(requestWire)
	for _, forbidden := range []string{"secret-value", "trace-value", "The observed sky was blue.", "data:"} {
		if strings.Contains(requestText, forbidden) {
			t.Fatalf("model request record contains %q", forbidden)
		}
	}
	for _, required := range []string{"[redacted]", "facts.txt", "sha256", opts.EvidenceStandard, `"web_search_enabled": true`, `"reasoning_effort": "xhigh"`, `"max_output_tokens": 8000`, `"max_tool_calls": 8`, `"type": "web_search"`} {
		if !strings.Contains(requestText, required) {
			t.Fatalf("model request record omits %q\n%s", required, requestText)
		}
	}
	inputWire, err := json.Marshal(client.inputItems)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(inputWire), "The observed sky was blue.") || !strings.Contains(string(inputWire), opts.EvidenceStandard) {
		t.Fatalf("provider input omits document or evidence standard: %s", inputWire)
	}
	toolWire, err := json.Marshal(client.tools)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toolWire), `"type":"web_search"`) {
		t.Fatalf("provider tools omit default web search: %s", toolWire)
	}
	responseWire, err := os.ReadFile(filepath.Join(opts.OutputDir, "model-response.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"resp-1", `\"id\":\"resp-1\"`, "provider_metadata", "0.0125", "ws-1", "https://example.test/sky", "Sky observation"} {
		if !strings.Contains(string(responseWire), required) {
			t.Fatalf("model response omits %q\n%s", required, responseWire)
		}
	}
	events, err := os.ReadFile(filepath.Join(opts.OutputDir, "events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`"provider_usage"`, `"input_tokens":10`, `"provider_cost_usd":0.0125`, `"web_search_call_count":1`, `"web_search_source_count":1`, `"web_search_citation_count":1`, `"reasoning_effort":"xhigh"`, `"max_output_tokens":8000`, `"max_tool_calls":8`} {
		if !strings.Contains(string(events), required) {
			t.Fatalf("events omit %q\n%s", required, events)
		}
	}
	for _, name := range []string{
		"case-manifest.json", "input.json", "runtime.json", "documents.json",
		"model-request.json", "model-response.json", "decision.json", "state.json",
		"events.ndjson", "transcript.md", "digest.md", "run.json",
	} {
		if _, err := os.Stat(filepath.Join(opts.OutputDir, name)); err != nil {
			t.Errorf("record %s: %v", name, err)
		}
	}
	if temporary, err := filepath.Glob(filepath.Join(opts.OutputDir, ".run.json.*.tmp")); err != nil || len(temporary) != 0 {
		t.Fatalf("temporary run records = %v, error = %v", temporary, err)
	}
	runtimeWire, err := os.ReadFile(filepath.Join(opts.OutputDir, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`"web_search_enabled": true`, `"reasoning_effort": "xhigh"`, `"max_output_tokens": 8000`, `"max_tool_calls": 8`} {
		if !strings.Contains(string(runtimeWire), required) {
			t.Fatalf("runtime omits %q\n%s", required, runtimeWire)
		}
	}
	for _, name := range []string{"state.json", "run.json"} {
		wire, err := os.ReadFile(filepath.Join(opts.OutputDir, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{`"reasoning_effort": "xhigh"`, `"max_output_tokens": 8000`, `"max_tool_calls": 8`} {
			if !strings.Contains(string(wire), required) {
				t.Fatalf("%s omits %q\n%s", name, required, wire)
			}
		}
	}
	for _, name := range []string{"transcript.md", "digest.md"} {
		wire, err := os.ReadFile(filepath.Join(opts.OutputDir, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"Reasoning effort: `xhigh`", "Maximum output tokens: 8000", "Maximum tool calls: 8"} {
			if !strings.Contains(string(wire), required) {
				t.Fatalf("%s omits %q\n%s", name, required, wire)
			}
		}
	}

	disabledResponse := response
	disabledResponse.WebSearchCalls = nil
	disabledResponse.URLCitations = nil
	disabledClient := &fakeResponseClient{response: disabledResponse}
	disabledFactory := &fakeClientFactory{client: disabledClient}
	disabled := false
	disabledOpts := opts
	disabledOpts.OutputDir = filepath.Join(base, "out-search-disabled")
	disabledOpts.WebSearch = &disabled
	disabledResult, err := RunWithClientFactory(context.Background(), disabledOpts, disabledFactory)
	if err != nil {
		t.Fatalf("RunWithClientFactory with search disabled: %v", err)
	}
	if disabledResult.WebSearch.Enabled {
		t.Fatalf("disabled web search result = %#v", disabledResult.WebSearch)
	}
	disabledRequest, err := os.ReadFile(filepath.Join(disabledOpts.OutputDir, "model-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(disabledRequest), `"web_search_enabled": false`) || strings.Contains(string(disabledRequest), `"type": "web_search"`) {
		t.Fatalf("disabled model request = %s", disabledRequest)
	}
	disabledTools, err := json.Marshal(disabledClient.tools)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(disabledTools), `"type":"web_search"`) {
		t.Fatalf("disabled provider tools = %s", disabledTools)
	}
}

func TestRunRejectsMalformedDecisionWithoutAnotherRequest(t *testing.T) {
	t.Parallel()

	client := &fakeResponseClient{response: openaiapi.Response{
		ResponseID: "resp-bad",
		RawJSON:    `{"id":"resp-bad"}`,
		ToolCalls: []openaiapi.ToolCall{{
			Name:         "submit_simple_decision",
			RawArguments: `{"decision":"demonstrated","rationale":""}`,
		}},
	}}
	factory := &fakeClientFactory{client: client}
	opts := testOptions(filepath.Join(t.TempDir(), "out"))
	result, err := RunWithClientFactory(context.Background(), opts, factory)
	if err == nil {
		t.Fatal("RunWithClientFactory accepted an empty rationale")
	}
	if client.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", client.calls)
	}
	if result.ErrorClass != string(openaiapi.ProviderErrorProtocol) {
		t.Fatalf("error class = %q, want %q", result.ErrorClass, openaiapi.ProviderErrorProtocol)
	}
	raw, readErr := os.ReadFile(filepath.Join(opts.OutputDir, "model-response.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(raw), string(openaiapi.ProviderErrorProtocol)) || !strings.Contains(string(raw), "resp-bad") {
		t.Fatalf("model response did not preserve protocol error and response: %s", raw)
	}
	if strings.Contains(string(raw), "recovered_cost_usd") {
		t.Fatalf("model response represented an unknown provider cost: %s", raw)
	}
	if strings.Contains(string(raw), "provider_usage") {
		t.Fatalf("model response represented unknown provider usage: %s", raw)
	}
	runRecord, readErr := os.ReadFile(filepath.Join(opts.OutputDir, "run.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(runRecord), "provider_cost_usd") {
		t.Fatalf("run record represented an unknown provider cost: %s", runRecord)
	}
	if strings.Contains(string(runRecord), "provider_usage") {
		t.Fatalf("run record represented unknown provider usage: %s", runRecord)
	}
}

func TestRunRejectsUnsupportedMediaBeforeClientCreation(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	documentsRoot := filepath.Join(base, "source")
	if err := os.Mkdir(documentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(documentsRoot, "binary.bin"), []byte{0, 0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}
	factory := &fakeClientFactory{client: &fakeResponseClient{}}
	opts := testOptions(filepath.Join(base, "out"))
	opts.DocumentsRoot = documentsRoot
	result, err := RunWithClientFactory(context.Background(), opts, factory)
	if err == nil {
		t.Fatal("RunWithClientFactory accepted unsupported media")
	}
	if factory.newCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", factory.newCalls)
	}
	if result.Status != "error" || result.ErrorClass != "input" {
		t.Fatalf("result = %#v", result)
	}
	for _, name := range []string{"model-response.json", "decision.json", "state.json", "run.json"} {
		if _, err := os.Stat(filepath.Join(opts.OutputDir, name)); err != nil {
			t.Errorf("terminal record %s: %v", name, err)
		}
	}
}

func TestRunRecordsDocumentImportFailure(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	documentsRoot := filepath.Join(base, "source")
	if err := os.Mkdir(documentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(documentsRoot, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	factory := &fakeClientFactory{client: &fakeResponseClient{}}
	opts := testOptions(filepath.Join(base, "out"))
	opts.DocumentsRoot = documentsRoot
	opts.MaxDocuments = 1
	result, err := RunWithClientFactory(context.Background(), opts, factory)
	if err == nil {
		t.Fatal("RunWithClientFactory accepted too many documents")
	}
	if factory.newCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", factory.newCalls)
	}
	if result.Status != "error" || result.ErrorClass != "input" {
		t.Fatalf("result = %#v", result)
	}
	for _, name := range []string{"documents.json", "model-request.json", "model-response.json", "decision.json", "state.json", "run.json"} {
		if _, err := os.Stat(filepath.Join(opts.OutputDir, name)); err != nil {
			t.Errorf("failure record %s: %v", name, err)
		}
	}
}

func TestRunPreservesTypedProviderFailure(t *testing.T) {
	t.Parallel()

	providerErr := &openaiapi.ProviderError{Class: openaiapi.ProviderErrorAuthentication, Err: errors.New("credential rejected")}
	client := &fakeResponseClient{err: providerErr}
	factory := &fakeClientFactory{client: client}
	opts := testOptions(filepath.Join(t.TempDir(), "out"))
	result, err := RunWithClientFactory(context.Background(), opts, factory)
	if err == nil {
		t.Fatal("RunWithClientFactory accepted provider failure")
	}
	if result.ErrorClass != string(openaiapi.ProviderErrorAuthentication) {
		t.Fatalf("error class = %q", result.ErrorClass)
	}
	if client.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", client.calls)
	}
	raw, readErr := os.ReadFile(filepath.Join(opts.OutputDir, "run.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(raw), string(openaiapi.ProviderErrorAuthentication)) || !strings.Contains(string(raw), "credential rejected") {
		t.Fatalf("run record = %s", raw)
	}
}

func TestRunRequiresExplicitAPIKeyPermission(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	factory := &fakeClientFactory{client: &fakeResponseClient{}}
	opts := testOptions(filepath.Join(base, "out"))
	opts.AllowAPIKey = false
	_, err := RunWithClientFactory(context.Background(), opts, factory)
	if err == nil || !strings.Contains(err.Error(), "--allow-api-key") {
		t.Fatalf("error = %v", err)
	}
	if factory.newCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", factory.newCalls)
	}
	if _, err := os.Stat(opts.OutputDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output directory stat error = %v, want not exist", err)
	}
}

func TestRunRejectsInvalidModelRequestControlsBeforeOutput(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name            string
		reasoningEffort string
		maxOutputTokens int64
		maxToolCalls    int64
	}{
		{name: "reasoning effort", reasoningEffort: "extreme"},
		{name: "max output tokens", maxOutputTokens: -1},
		{name: "max tool calls", maxToolCalls: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &fakeClientFactory{client: &fakeResponseClient{}}
			outputDir := filepath.Join(t.TempDir(), "out")
			opts := testOptions(outputDir)
			opts.ReasoningEffort = test.reasoningEffort
			opts.MaxOutputTokens = test.maxOutputTokens
			opts.MaxToolCalls = test.maxToolCalls
			if _, err := RunWithClientFactory(context.Background(), opts, factory); err == nil {
				t.Fatal("RunWithClientFactory accepted invalid request control")
			}
			if factory.newCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", factory.newCalls)
			}
			if _, err := os.Stat(outputDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("output directory stat error = %v, want not exist", err)
			}
		})
	}
}

func TestRunRequiresExactlyOneRequestSpecSource(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	requestSpecPath := filepath.Join(base, "request.json")
	if err := os.WriteFile(requestSpecPath, []byte(`{"endpoint":"openai","model":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name            string
		model           string
		requestSpecPath string
	}{
		{name: "neither"},
		{name: "both", model: "openai://test", requestSpecPath: requestSpecPath},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &fakeClientFactory{client: &fakeResponseClient{}}
			opts := testOptions(filepath.Join(base, test.name))
			opts.Model = test.model
			opts.RequestSpecPath = test.requestSpecPath
			_, err := RunWithClientFactory(context.Background(), opts, factory)
			if err == nil || !strings.Contains(err.Error(), "exactly one") {
				t.Fatalf("error = %v", err)
			}
			if factory.newCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", factory.newCalls)
			}
		})
	}
}

func TestResolveOptionsModelRequestControls(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Unix(1, 0)

	defaults := testOptions(filepath.Join(base, "default"))
	resolved, spec, runtime, err := resolveOptions(defaults, now)
	if err != nil {
		t.Fatalf("resolveOptions defaults: %v", err)
	}
	if resolved.ReasoningEffort != "" || runtime.ReasoningEffort != "" || spec.ReasoningEffort() != "" {
		t.Fatalf("default reasoning effort = %q, %q, %q", resolved.ReasoningEffort, runtime.ReasoningEffort, spec.ReasoningEffort())
	}
	if resolved.MaxOutputTokens != DefaultMaxOutputTokens || runtime.MaxOutputTokens != DefaultMaxOutputTokens || spec.MaxOutputTokens() == nil || *spec.MaxOutputTokens() != DefaultMaxOutputTokens {
		t.Fatalf("default max output tokens = %d, %d, %v", resolved.MaxOutputTokens, runtime.MaxOutputTokens, spec.MaxOutputTokens())
	}
	if resolved.MaxToolCalls != 0 || runtime.MaxToolCalls != 0 || spec.MaxToolCalls() != nil {
		t.Fatalf("default max tool calls = %d, %d, %v", resolved.MaxToolCalls, runtime.MaxToolCalls, spec.MaxToolCalls())
	}

	requestSpecPath := filepath.Join(base, "request.json")
	if err := os.WriteFile(requestSpecPath, []byte(`{
		"endpoint":"openai",
		"model":"test-model",
		"request":{"reasoning_effort":"low","max_output_tokens":6000,"max_tool_calls":6}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fromSpec := testOptions(filepath.Join(base, "from-spec"))
	fromSpec.Model = ""
	fromSpec.RequestSpecPath = requestSpecPath
	resolved, spec, runtime, err = resolveOptions(fromSpec, now)
	if err != nil {
		t.Fatalf("resolveOptions request spec: %v", err)
	}
	if resolved.ReasoningEffort != "low" || runtime.ReasoningEffort != "low" || spec.ReasoningEffort() != "low" {
		t.Fatalf("request-spec reasoning effort = %q, %q, %q", resolved.ReasoningEffort, runtime.ReasoningEffort, spec.ReasoningEffort())
	}
	if resolved.MaxOutputTokens != 6000 || runtime.MaxOutputTokens != 6000 || spec.MaxOutputTokens() == nil || *spec.MaxOutputTokens() != 6000 {
		t.Fatalf("request-spec max output tokens = %d, %d, %v", resolved.MaxOutputTokens, runtime.MaxOutputTokens, spec.MaxOutputTokens())
	}
	if resolved.MaxToolCalls != 6 || runtime.MaxToolCalls != 6 || spec.MaxToolCalls() == nil || *spec.MaxToolCalls() != 6 {
		t.Fatalf("request-spec max tool calls = %d, %d, %v", resolved.MaxToolCalls, runtime.MaxToolCalls, spec.MaxToolCalls())
	}

	override := fromSpec
	override.OutputDir = filepath.Join(base, "override")
	override.ReasoningEffort = "xhigh"
	override.MaxOutputTokens = 8000
	override.MaxToolCalls = 8
	resolved, spec, runtime, err = resolveOptions(override, now)
	if err != nil {
		t.Fatalf("resolveOptions override: %v", err)
	}
	if resolved.ReasoningEffort != "xhigh" || runtime.ReasoningEffort != "xhigh" || spec.ReasoningEffort() != "xhigh" {
		t.Fatalf("overridden reasoning effort = %q, %q, %q", resolved.ReasoningEffort, runtime.ReasoningEffort, spec.ReasoningEffort())
	}
	if resolved.MaxOutputTokens != 8000 || runtime.MaxOutputTokens != 8000 || spec.MaxOutputTokens() == nil || *spec.MaxOutputTokens() != 8000 {
		t.Fatalf("overridden max output tokens = %d, %d, %v", resolved.MaxOutputTokens, runtime.MaxOutputTokens, spec.MaxOutputTokens())
	}
	if resolved.MaxToolCalls != 8 || runtime.MaxToolCalls != 8 || spec.MaxToolCalls() == nil || *spec.MaxToolCalls() != 8 {
		t.Fatalf("overridden max tool calls = %d, %d, %v", resolved.MaxToolCalls, runtime.MaxToolCalls, spec.MaxToolCalls())
	}

	invalidEffort := defaults
	invalidEffort.ReasoningEffort = "extreme"
	if _, _, _, err := resolveOptions(invalidEffort, now); err == nil || !strings.Contains(err.Error(), "reasoning effort") {
		t.Fatalf("invalid reasoning effort error = %v", err)
	}
	negativeMax := defaults
	negativeMax.MaxOutputTokens = -1
	if _, _, _, err := resolveOptions(negativeMax, now); err == nil || !strings.Contains(err.Error(), "--max-output-tokens") {
		t.Fatalf("negative max output tokens error = %v", err)
	}
	negativeToolMax := defaults
	negativeToolMax.MaxToolCalls = -1
	if _, _, _, err := resolveOptions(negativeToolMax, now); err == nil || !strings.Contains(err.Error(), "--max-tool-calls") {
		t.Fatalf("negative max tool calls error = %v", err)
	}
}

func TestBuildInputItemsSupportsTextImagesAndPDF(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	source := filepath.Join(base, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"facts.txt":  []byte("text evidence"),
		"photo.png":  append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...),
		"report.pdf": []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n"),
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(source, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(base, "documents")
	manifest, err := documents.Import(source, destination, documents.Limits{MaxFiles: 3, MaxFileBytes: 1024, MaxTotalBytes: 2048})
	if err != nil {
		t.Fatal(err)
	}
	items, err := buildInputItems("P", "preponderance of the evidence", "developer prompt", destination, manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{"text evidence", "data:image/png;base64,", "input_image", "data:application/pdf;base64,", "input_file", "report.pdf"} {
		if !strings.Contains(text, required) {
			t.Fatalf("request omits %q\n%s", required, text)
		}
	}
}

func TestBuildInputItemsWithoutDocumentsAllowsEstablishedKnowledge(t *testing.T) {
	t.Parallel()

	prompt, err := resolveDeveloperPrompt(Options{EvidenceStandard: "preponderance of the evidence"}, false)
	if err != nil {
		t.Fatal(err)
	}
	items, err := buildInputItems(
		"A square has four sides.",
		"preponderance of the evidence",
		prompt,
		t.TempDir(),
		documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{"relevant established knowledge", "absence of documents does not decide the proposition"} {
		if !strings.Contains(text, required) {
			t.Fatalf("request omits %q\n%s", required, text)
		}
	}
	if strings.Contains(text, "supplied material demonstrates") {
		t.Fatalf("request makes supplied documents dispositive\n%s", text)
	}
}

func TestResolveDeveloperPromptFiles(t *testing.T) {
	root := t.TempDir()
	decisionPath := filepath.Join(root, "decision.md")
	searchPath := filepath.Join(root, "search.md")
	if err := os.WriteFile(decisionPath, []byte("standard={{EVIDENCE_STANDARD}}\n{{WEB_SEARCH}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(searchPath, []byte("search standard={{EVIDENCE_STANDARD}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{
		EvidenceStandard: "clear and convincing",
		PromptFiles: map[string]string{
			"decision":       decisionPath,
			"search.enabled": searchPath,
		},
	}
	prompt, err := resolveDeveloperPrompt(opts, true)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "standard=clear and convincing\nsearch standard=clear and convincing" {
		t.Fatalf("prompt = %q", prompt)
	}

	opts.PromptFiles["decision"] = filepath.Join(root, "missing.md")
	if _, err := resolveDeveloperPrompt(opts, true); err == nil || !strings.Contains(err.Error(), "read simple decision prompt") {
		t.Fatalf("missing explicit prompt error = %v", err)
	}
	if err := os.WriteFile(decisionPath, []byte("{{UNKNOWN}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.PromptFiles["decision"] = decisionPath
	if _, err := resolveDeveloperPrompt(opts, true); err == nil || !strings.Contains(err.Error(), "unresolved token {{UNKNOWN}}") {
		t.Fatalf("unresolved token error = %v", err)
	}
	if err := os.WriteFile(decisionPath, []byte(" \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveDeveloperPrompt(opts, true); err == nil || !strings.Contains(err.Error(), "prompt is empty") {
		t.Fatalf("empty prompt error = %v", err)
	}
	duplicate := testOptions(t.TempDir())
	duplicate.PromptFiles = map[string]string{"decision": decisionPath, " decision ": searchPath}
	if _, _, _, err := resolveOptions(duplicate, time.Unix(1, 0)); err == nil || !strings.Contains(err.Error(), "repeated after trimming") {
		t.Fatalf("trimmed duplicate prompt ID error = %v", err)
	}
}

func TestParseDecisionRejectsMalformedCalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		calls []openaiapi.ToolCall
	}{
		{name: "none"},
		{name: "wrong tool", calls: []openaiapi.ToolCall{{Name: "other", RawArguments: `{}`}}},
		{name: "unknown field", calls: []openaiapi.ToolCall{{Name: "submit_simple_decision", RawArguments: `{"decision":"demonstrated","rationale":"r","extra":true}`}}},
		{name: "trailing JSON", calls: []openaiapi.ToolCall{{Name: "submit_simple_decision", RawArguments: `{"decision":"demonstrated","rationale":"r"}{}`}}},
		{name: "bad decision", calls: []openaiapi.ToolCall{{Name: "submit_simple_decision", RawArguments: `{"decision":"maybe","rationale":"r"}`}}},
		{name: "two calls", calls: []openaiapi.ToolCall{{Name: "submit_simple_decision", RawArguments: `{}`}, {Name: "submit_simple_decision", RawArguments: `{}`}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDecision(openaiapi.Response{ToolCalls: test.calls})
			if err == nil {
				t.Fatal("parseDecision accepted malformed response")
			}
			if openaiapi.ErrorClass(err) != openaiapi.ProviderErrorProtocol {
				t.Fatalf("error class = %q", openaiapi.ErrorClass(err))
			}
		})
	}
}

func testOptions(outputDir string) Options {
	return Options{
		Proposition:       "The sky is blue.",
		OutputDir:         outputDir,
		CaseID:            "case-1",
		RunID:             "run-1",
		Model:             "openai://test-model",
		EvidenceStandard:  "preponderance of the evidence",
		AllowAPIKey:       true,
		MaxDocuments:      10,
		MaxDocumentBytes:  1024,
		MaxDocumentsBytes: 4096,
	}
}
