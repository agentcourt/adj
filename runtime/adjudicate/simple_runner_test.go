package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSimpleRunnerInvokesCoreWithSelectedCredential(t *testing.T) {
	t.Setenv("ADJ_TEST_OPENAI_KEY", "selected-secret")
	t.Setenv("OPENAI_API_KEY", "unselected-openai-secret")
	t.Setenv("OPENROUTER_API_KEY", "unselected-openrouter-secret")

	root := t.TempDir()
	request := simpleProcedureRequest(root)
	var captured coreProcessRequest
	runner := SimpleRunner{runProcess: func(_ context.Context, process coreProcessRequest) ([]byte, error) {
		captured = process
		return []byte(`{"status":"ok","phase":"closed","decision":{"value":"demonstrated","rationale":"The record supports it."},"provider":{"request_count":1,"usage_observed_count":1,"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15},"cost_observed_count":0}}`), nil
	}}
	outcome, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != StatusOK || outcome.Phase != "closed" || outcome.Decision == nil || outcome.Decision.Value != "demonstrated" {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Provider.RequestCount != 1 || outcome.Provider.Usage == nil || outcome.Provider.Usage.TotalTokens != 15 {
		t.Fatalf("provider accounting = %#v", outcome.Provider)
	}
	if captured.Command != "/opt/adj/simple" || captured.Dir != filepath.Join(root, "core-work") {
		t.Fatalf("process command = %q, directory = %q", captured.Command, captured.Dir)
	}
	wantArgs := []string{
		"case",
		"--proposition", "The proposition",
		"--documents", filepath.Join(root, "record", "inputs", "documents"),
		"--out-dir", filepath.Join(root, "record", "core"),
		"--case-id", "case-1",
		"--run-id", "run-1",
		"--model", "openai://test-model",
		"--web-search=false",
		"--evidence-standard", "preponderance_of_the_evidence",
		"--allow-api-key",
		"--max-documents", "7",
		"--max-document-bytes", "1000",
		"--max-documents-bytes", "5000",
		"--reasoning-effort", "xhigh",
		"--max-output-tokens", "16384",
		"--max-tool-calls", "8",
		"--prompt-dir", "/prompts/simple",
		"--prompt-file", "decision=/prompts/decision.md",
		"--prompt-file", "search.disabled=/prompts/search-off.md",
		"--prompt-file", "search.enabled=/prompts/search-on.md",
		"--timeout-seconds", "30",
	}
	if !reflect.DeepEqual(captured.Args, wantArgs) {
		t.Fatalf("arguments = %#v, want %#v", captured.Args, wantArgs)
	}
	if value, ok := environmentValue(captured.Env, "OPENAI_API_KEY"); !ok || value != "selected-secret" {
		t.Fatalf("OPENAI_API_KEY = %q, present = %t", value, ok)
	}
	for _, name := range []string{"ADJ_TEST_OPENAI_KEY", "OPENROUTER_API_KEY"} {
		if value, ok := environmentValue(captured.Env, name); ok {
			t.Fatalf("environment contains %s=%q", name, value)
		}
	}
	if !json.Valid(outcome.ProcedureResult) {
		t.Fatalf("procedure result = %q", outcome.ProcedureResult)
	}
}

func TestSimpleRunnerPreservesCoreErrorClassAndProcessError(t *testing.T) {
	t.Setenv("ADJ_TEST_OPENAI_KEY", "selected-secret")
	request := simpleProcedureRequest(t.TempDir())
	processErr := errors.New("exit status 1")
	runner := SimpleRunner{runProcess: func(context.Context, coreProcessRequest) ([]byte, error) {
		return []byte(`{"status":"error","phase":"error","error":"credential rejected","error_class":"provider_authentication","provider":{"request_count":1,"usage_observed_count":0,"cost_observed_count":0}}`), processErr
	}}
	_, err := runner.Run(context.Background(), request)
	if err == nil || !errors.Is(err, processErr) || !strings.Contains(err.Error(), "credential rejected") {
		t.Fatalf("error = %v", err)
	}
	if class := runnerErrorClass(err); class != "provider_authentication" {
		t.Fatalf("error class = %q", class)
	}
	if provider := providerAccountingFromError(err); provider.RequestCount != 1 {
		t.Fatalf("provider accounting = %#v", provider)
	}
}

func TestSimpleRunnerPreservesAccountingForInvalidResult(t *testing.T) {
	t.Setenv("ADJ_TEST_OPENAI_KEY", "selected-secret")
	runner := SimpleRunner{runProcess: func(context.Context, coreProcessRequest) ([]byte, error) {
		return []byte(`{"status":"ok","phase":"closed","provider":{"request_count":1,"usage_observed_count":0,"cost_observed_count":0}}`), nil
	}}
	_, err := runner.Run(context.Background(), simpleProcedureRequest(t.TempDir()))
	if err == nil || !strings.Contains(err.Error(), "has no decision") {
		t.Fatalf("error = %v", err)
	}
	if providerAccountingFromError(err).RequestCount != 1 {
		t.Fatalf("provider accounting = %#v", providerAccountingFromError(err))
	}
}

func TestCoreRunErrorClassIsPrintedOnce(t *testing.T) {
	coreErr := &CoreRunError{Class: "provider_transient", Err: errors.New("provider request failed")}
	if got := coreErr.Error(); got != "provider request failed" {
		t.Fatalf("core error = %q", got)
	}
	outer := classified(runnerErrorClass(coreErr), coreErr)
	if got := outer.Error(); got != "provider_transient: provider request failed" {
		t.Fatalf("classified error = %q", got)
	}
}

func TestDirectProviderEnvironmentRequiresExplicitSource(t *testing.T) {
	_, err := directProviderEnvironment("openai://model", ResolvedSettings{Common: CommonSettings{ProviderCredentials: map[string]CredentialMetadata{"openai": {Source: AuthSubscription}}}}, []string{"OPENAI_API_KEY=secret"})
	if err == nil || !strings.Contains(err.Error(), "must use api_key") {
		t.Fatalf("error = %v", err)
	}
	_, err = directProviderEnvironment("anthropic://model", ResolvedSettings{Common: CommonSettings{ProviderCredentials: map[string]CredentialMetadata{"anthropic": {Source: AuthAPIKey, EnvironmentVariable: "KEY"}}}}, []string{"KEY=secret"})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunnerErrorClassRecognizesCancellation(t *testing.T) {
	if class := runnerErrorClass(context.Canceled); class != "canceled" {
		t.Fatalf("error class = %q", class)
	}
	cleanupErr := errors.New("cleanup failed")
	if class := runnerErrorClass(errors.Join(fmt.Errorf("wait: %w", context.DeadlineExceeded), cleanupErr)); class != "canceled" {
		t.Fatalf("primary cancellation class = %q", class)
	}
	processErr := errors.New("lawyer process failed")
	if class := runnerErrorClass(errors.Join(processErr, context.Canceled)); class != "procedure_run" {
		t.Fatalf("primary process-failure class = %q", class)
	}
	providerErr := &CoreRunError{Class: "provider_authentication", Err: errors.New("credential rejected")}
	if class := runnerErrorClass(errors.Join(providerErr, context.Canceled)); class != "provider_authentication" {
		t.Fatalf("primary classified-failure class = %q", class)
	}
}

func simpleProcedureRequest(root string) ProcedureRequest {
	return ProcedureRequest{
		Request: Request{
			Procedure:   ProcedureSimple,
			CaseID:      "case-1",
			RunID:       "run-1",
			Proposition: "The proposition",
		},
		Settings: ResolvedSettings{
			Common: CommonSettings{
				EvidenceStandard: "preponderance_of_the_evidence",
				DocumentLimits:   DocumentLimits{Count: 7, PerFile: 1000, Total: 5000},
				ProviderCredentials: map[string]CredentialMetadata{
					"openai": {Source: AuthAPIKey, EnvironmentVariable: "ADJ_TEST_OPENAI_KEY"},
				},
			},
			Procedure: ResolvedProcedureSettings{Simple: &ResolvedSimpleSettings{
				CoreCommand:     "/opt/adj/simple",
				CoreWorkingDir:  filepath.Join(root, "core-work"),
				Model:           "openai://test-model",
				ReasoningEffort: "xhigh",
				MaxOutputTokens: 16384,
				MaxToolCalls:    8,
				WebSearch:       false,
				PromptDir:       "/prompts/simple",
				PromptFiles: PromptFilePaths{
					"decision":        "/prompts/decision.md",
					"search.enabled":  "/prompts/search-on.md",
					"search.disabled": "/prompts/search-off.md",
				},
				Timeout: Duration(30 * time.Second),
			}},
		},
		RecordDir: filepath.Join(root, "record"),
		CoreDir:   filepath.Join(root, "record", "core"),
		LogsDir:   filepath.Join(root, "record", "logs"),
	}
}
