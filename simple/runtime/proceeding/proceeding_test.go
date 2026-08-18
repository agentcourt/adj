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

	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
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
		OpenRouterCostUSD:    0.0125,
	}
	client := &fakeResponseClient{response: response}
	factory := &fakeClientFactory{client: client}
	opts := testOptions(filepath.Join(base, "out"))
	opts.DocumentsRoot = documentsRoot
	opts.RequestSpecPath = requestSpecPath
	opts.Model = ""
	result, err := RunWithClientFactory(context.Background(), opts, factory)
	if err != nil {
		t.Fatalf("RunWithClientFactory error = %v", err)
	}
	if result.Status != "ok" || result.Decision == nil || result.Decision.Value != "demonstrated" {
		t.Fatalf("result = %#v", result)
	}
	if result.ProviderCostUSD != 0.0125 || result.ResponseID != "resp-1" {
		t.Fatalf("provider result = %#v", result)
	}
	if factory.newCalls != 1 || client.calls != 1 {
		t.Fatalf("factory calls = %d, provider calls = %d, want 1, 1", factory.newCalls, client.calls)
	}
	if factory.timeout != DefaultTimeoutSeconds*time.Second {
		t.Fatalf("timeout = %s", factory.timeout)
	}
	if client.spec.MaxOutputTokens() == nil || *client.spec.MaxOutputTokens() != DefaultMaxOutputTokens {
		t.Fatalf("max output tokens = %v", client.spec.MaxOutputTokens())
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
	for _, required := range []string{"[redacted]", "facts.txt", "sha256", opts.EvidenceStandard} {
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
	responseWire, err := os.ReadFile(filepath.Join(opts.OutputDir, "model-response.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"resp-1", `\"id\":\"resp-1\"`, "provider_metadata", "0.0125"} {
		if !strings.Contains(string(responseWire), required) {
			t.Fatalf("model response omits %q\n%s", required, responseWire)
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
	items, err := buildInputItems("P", "preponderance of the evidence", destination, manifest)
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
