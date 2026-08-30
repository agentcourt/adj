package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	headless "github.com/agentcourt/adj/runtime/agent"
)

type fakeRunner struct {
	request ProcedureRequest
	outcome ProcedureOutcome
	err     error
	calls   int
	inspect func(ProcedureRequest) error
}

func (f *fakeRunner) Run(_ context.Context, request ProcedureRequest) (ProcedureOutcome, error) {
	f.calls++
	f.request = request
	if f.inspect != nil {
		if err := f.inspect(request); err != nil {
			return ProcedureOutcome{}, err
		}
	}
	return f.outcome, f.err
}

func TestEngineDispatchesThroughRegistryAndPublishesResult(t *testing.T) {
	providerCost := 0.125
	runner := &fakeRunner{outcome: ProcedureOutcome{
		Status:          StatusOK,
		Phase:           "closed",
		Decision:        &Decision{Kind: "binary", Value: "demonstrated", Rationale: "record supports it"},
		ProcedureResult: json.RawMessage(`{"native":true}`),
		Provider: ProviderManagement{
			RequestCount:       2,
			UsageObservedCount: 1,
			Usage:              &TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			CostObservedCount:  1,
			CostUSD:            &providerCost,
		},
	}}
	runner.inspect = func(request ProcedureRequest) error {
		data, err := os.ReadFile(filepath.Join(request.RecordDir, "run.json"))
		if err != nil {
			return err
		}
		var live Result
		if err := json.Unmarshal(data, &live); err != nil {
			return err
		}
		if live.Status != StatusRunning || live.Phase != "documents_imported" || live.FinishedAt != nil {
			return errors.New("live run record has invalid state")
		}
		if len(live.Management.Processes) != 1 || live.Management.Processes[0].State != ProcessRunning || live.Management.Processes[0].Kind != "controller" {
			return errors.New("live run record has invalid controller process")
		}
		return nil
	}
	capabilities := Capabilities{Documents: true, Sessions: true}
	registry, err := NewRegistry(map[Procedure]Registration{
		ProcedureSimple: {Capabilities: capabilities, Runner: runner},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(registry)
	times := []time.Time{
		time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 18, 12, 0, 1, 0, time.UTC),
		time.Date(2026, 8, 18, 12, 0, 2, 0, time.UTC),
	}
	engine.now = func() time.Time {
		value := times[0]
		times = times[1:]
		return value
	}
	documents := t.TempDir()
	if err := os.WriteFile(filepath.Join(documents, "fact.txt"), []byte("fact"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "new", "case")
	request := Request{
		SchemaVersion: RequestSchemaVersion,
		Procedure:     ProcedureSimple,
		CaseID:        "case-1",
		RunID:         "run-1",
		Proposition:   "The proposition",
		Documents:     DocumentInput{Root: documents},
		SettingsFile:  simpleSettingsFile(t),
		OutDir:        outDir,
	}
	result, err := engine.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d", runner.calls)
	}
	if runner.request.CoreDir != filepath.Join(outDir, "core") || runner.request.LogsDir != filepath.Join(outDir, "logs") {
		t.Fatalf("runner directories = %#v", runner.request)
	}
	if len(runner.request.Documents.Documents) != 1 || runner.request.Documents.Documents[0].Path != "fact.txt" {
		t.Fatalf("runner documents = %#v", runner.request.Documents)
	}
	if result.Status != StatusOK || result.Decision.Value != "demonstrated" || !reflect.DeepEqual(result.Capabilities, capabilities) {
		t.Fatalf("result = %#v", result)
	}
	for _, path := range []string{
		"inputs/adjudicate-request.json",
		"inputs/resolved-settings.json",
		"inputs/documents.json",
		"inputs/documents/fact.txt",
		"events.ndjson",
		"run.json",
	} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(path))); err != nil {
			t.Errorf("record %q: %v", path, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(outDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded Result
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded.Status != result.Status || recorded.RecordDir != outDir {
		t.Fatalf("recorded result = %#v", recorded)
	}
	if recorded.FinishedAt == nil || len(recorded.Management.Processes) != 1 || recorded.Management.Processes[0].State != ProcessCompleted {
		t.Fatalf("recorded management = %#v", recorded.Management)
	}
	if recorded.Management.Provider.RequestCount != 2 || recorded.Management.Provider.UsageObservedCount != 1 || recorded.Management.Provider.CostUSD == nil || *recorded.Management.Provider.CostUSD != providerCost {
		t.Fatalf("recorded provider accounting = %#v", recorded.Management.Provider)
	}
	events, err := os.ReadFile(filepath.Join(outDir, "events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(events)), "\n")
	if len(lines) != 3 {
		t.Fatalf("event lines = %d: %s", len(lines), events)
	}
	for _, line := range lines {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("event %q: %v", line, err)
		}
	}
}

func TestEnginePreservesOutcomeWhenFinalRecordWriteFails(t *testing.T) {
	writeErr := errors.New("final record write failed")
	providerCost := 0.25
	outcome := ProcedureOutcome{
		Status:          StatusOK,
		Phase:           "closed",
		Decision:        &Decision{Kind: "binary", Value: "demonstrated", Rationale: "record supports it"},
		ProcedureResult: json.RawMessage(`{"native":true}`),
		Provider: ProviderManagement{
			RequestCount:       2,
			UsageObservedCount: 1,
			Usage:              &TokenUsage{InputTokens: 8, OutputTokens: 3, TotalTokens: 11},
			CostObservedCount:  1,
			CostUSD:            &providerCost,
		},
	}
	runner := &fakeRunner{outcome: outcome}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(registry)
	failedOnce := false
	engine.writeRunJSON = func(path string, value any) error {
		result, ok := value.(Result)
		if ok && result.Status == StatusOK && !failedOnce {
			failedOnce = true
			return writeErr
		}
		return WriteJSONAtomic(path, value)
	}
	outDir := filepath.Join(t.TempDir(), "case")
	result, err := engine.Run(context.Background(), validSimpleRequest(t, outDir))
	assertClass(t, err, "record_write")
	if !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want final write error", err)
	}
	assertPreservedOutcomeAfterRecordError(t, result, outcome)

	data, err := os.ReadFile(filepath.Join(outDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded Result
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	assertPreservedOutcomeAfterRecordError(t, recorded, outcome)

	eventData, err := os.ReadFile(filepath.Join(outDir, "events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(eventData)), "\n")
	if len(lines) != 4 {
		t.Fatalf("event lines = %d, want 4: %s", len(lines), eventData)
	}
	var terminal, publicationError Event
	if err := json.Unmarshal([]byte(lines[len(lines)-2]), &terminal); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &publicationError); err != nil {
		t.Fatal(err)
	}
	if terminal.Type != "terminal" || publicationError.Type != "error" || publicationError.Detail != "record_write" {
		t.Fatalf("terminal events = %#v, %#v", terminal, publicationError)
	}
}

func TestEngineReturnsOutcomeWhenTerminalRecordCannotBeWritten(t *testing.T) {
	writeErr := errors.New("terminal records unavailable")
	outcome := ProcedureOutcome{
		Status:          StatusOK,
		Phase:           "closed",
		Decision:        &Decision{Kind: "binary", Value: "not_demonstrated"},
		ProcedureResult: json.RawMessage(`{"native":true}`),
		Provider:        ProviderManagement{RequestCount: 1},
	}
	runner := &fakeRunner{outcome: outcome}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(registry)
	engine.writeRunJSON = func(path string, value any) error {
		result, ok := value.(Result)
		if ok && result.Status != StatusRunning {
			return writeErr
		}
		return WriteJSONAtomic(path, value)
	}
	outDir := filepath.Join(t.TempDir(), "case")
	result, err := engine.Run(context.Background(), validSimpleRequest(t, outDir))
	assertClass(t, err, "record_write")
	if !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want terminal write error", err)
	}
	assertPreservedOutcomeAfterRecordError(t, result, outcome)

	data, err := os.ReadFile(filepath.Join(outDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded Result
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded.Status != StatusRunning || recorded.Phase != "documents_imported" {
		t.Fatalf("last durable result = %#v", recorded)
	}
}

func assertPreservedOutcomeAfterRecordError(t *testing.T, result Result, outcome ProcedureOutcome) {
	t.Helper()
	if result.Status != StatusError || result.Phase != "error" || result.ErrorClass != "record_write" || result.Error == "" {
		t.Fatalf("result error fields = %#v", result)
	}
	if !reflect.DeepEqual(result.Decision, outcome.Decision) || !equalJSON(result.ProcedureResult, outcome.ProcedureResult) || !equalJSON(result.Failure, outcome.Failure) {
		t.Fatalf("preserved outcome = %#v, want %#v", result, outcome)
	}
	if !reflect.DeepEqual(result.Management.Provider, outcome.Provider) {
		t.Fatalf("provider accounting = %#v, want %#v", result.Management.Provider, outcome.Provider)
	}
	if len(result.Management.Processes) != 1 || result.Management.Processes[0].State != ProcessFailed || result.Management.Processes[0].FinishedAt == nil {
		t.Fatalf("management = %#v", result.Management)
	}
}

func equalJSON(left, right json.RawMessage) bool {
	if len(left) == 0 || len(right) == 0 {
		return len(left) == len(right)
	}
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func TestEngineDefaultOutputIncludesCaseAndRun(t *testing.T) {
	runner := &fakeRunner{outcome: ProcedureOutcome{Status: StatusOK, Phase: "closed"}}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := simpleSettingsFile(t)
	request := Request{
		SchemaVersion: RequestSchemaVersion,
		Procedure:     ProcedureSimple,
		CaseID:        "case-1",
		RunID:         "run-2",
		Proposition:   "The proposition",
		SettingsFile:  settingsPath,
	}
	result, err := NewEngine(registry).Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(settingsPath), "out", "case-1", "run-2")
	if result.RecordDir != want || runner.request.RecordDir != want {
		t.Fatalf("record directories = %q and %q, want %q", result.RecordDir, runner.request.RecordDir, want)
	}
}

func TestEngineHoldsExclusiveProcedureSession(t *testing.T) {
	settingsPath := simpleSettingsFile(t)
	wantSession := filepath.Join(filepath.Dir(settingsPath), "out", ".agents", "case-1", "simple")
	runner := &fakeRunner{
		outcome: ProcedureOutcome{Status: StatusOK, Phase: "closed"},
		inspect: func(request ProcedureRequest) error {
			if request.SessionDir != wantSession {
				return fmt.Errorf("session directory = %q, want %q", request.SessionDir, wantSession)
			}
			second, err := acquireSessionLease(request.SessionDir)
			if second != nil {
				return errors.Join(errors.New("second lease succeeded"), second.close())
			}
			if err == nil || !strings.Contains(err.Error(), "is in use") {
				return fmt.Errorf("second lease error = %v", err)
			}
			return nil
		},
	}
	registry, err := NewRegistry(map[Procedure]Registration{
		ProcedureSimple: {Capabilities: Capabilities{Sessions: true}, Runner: runner},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := validSimpleRequest(t, filepath.Join(t.TempDir(), "case"))
	request.SettingsFile = settingsPath
	if _, err := NewEngine(registry).Run(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireSessionLease(wantSession)
	if err != nil {
		t.Fatalf("acquire released session: %v", err)
	}
	if err := lease.close(); err != nil {
		t.Fatalf("close reacquired session: %v", err)
	}
}

func TestEngineRecordsSessionInUse(t *testing.T) {
	settingsPath := simpleSettingsFile(t)
	sessionDir := filepath.Join(filepath.Dir(settingsPath), "out", ".agents", "case-1", "simple")
	lease, err := acquireSessionLease(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.close(); err != nil {
			t.Errorf("close session lease: %v", err)
		}
	}()
	runner := &fakeRunner{}
	registry, err := NewRegistry(map[Procedure]Registration{
		ProcedureSimple: {Capabilities: Capabilities{Sessions: true}, Runner: runner},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := validSimpleRequest(t, filepath.Join(t.TempDir(), "case"))
	request.SettingsFile = settingsPath
	result, err := NewEngine(registry).Run(context.Background(), request)
	assertClass(t, err, "session_in_use")
	if runner.calls != 0 || result.Status != StatusError || result.ErrorClass != "session_in_use" {
		t.Fatalf("runner calls = %d, result = %#v", runner.calls, result)
	}
}

func TestEngineRequiresFreshOutputDirectory(t *testing.T) {
	runner := &fakeRunner{}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	request := validSimpleRequest(t, outDir)
	result, err := NewEngine(registry).Run(context.Background(), request)
	assertClass(t, err, "record_create")
	if result.SchemaVersion != ResultSchemaVersion || result.Status != StatusError || result.ErrorClass != "record_create" || result.RecordDir != outDir {
		t.Fatalf("result = %#v", result)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d", runner.calls)
	}
}

func TestEnginePublishesRunnerErrors(t *testing.T) {
	providerCost := 0.25
	runner := &fakeRunner{err: &CoreRunError{
		Err: errors.New("runner failed"),
		Provider: ProviderManagement{
			RequestCount:      1,
			CostObservedCount: 1,
			CostUSD:           &providerCost,
		},
	}}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "case")
	result, err := NewEngine(registry).Run(context.Background(), validSimpleRequest(t, outDir))
	assertClass(t, err, "procedure_run")
	if result.Status != StatusError || result.ErrorClass != "procedure_run" || !strings.Contains(result.Error, "runner failed") {
		t.Fatalf("result = %#v", result)
	}
	data, readErr := os.ReadFile(filepath.Join(outDir, "run.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var recorded Result
	if unmarshalErr := json.Unmarshal(data, &recorded); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	if recorded.ErrorClass != "procedure_run" {
		t.Fatalf("recorded result = %#v", recorded)
	}
	if len(recorded.Management.Processes) != 1 || recorded.Management.Processes[0].State != ProcessFailed || recorded.Management.Processes[0].Error != "runner failed" {
		t.Fatalf("recorded management = %#v", recorded.Management)
	}
	if recorded.Management.Provider.RequestCount != 1 || recorded.Management.Provider.CostUSD == nil || *recorded.Management.Provider.CostUSD != providerCost {
		t.Fatalf("recorded provider accounting = %#v", recorded.Management.Provider)
	}
}

func TestEnginePublishesProviderRefusalClass(t *testing.T) {
	runner := &fakeRunner{err: &headless.ProviderRefusalError{
		Provider:      "anthropic",
		Model:         "claude-opus-4-8",
		StopReason:    "error",
		RawStopReason: "refusal",
		Explanation:   "provider explanation",
	}}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "case")
	result, err := NewEngine(registry).Run(context.Background(), validSimpleRequest(t, outDir))
	assertClass(t, err, "provider_refusal")
	if result.Status != StatusError || result.ErrorClass != "provider_refusal" || result.Failure != nil || !strings.Contains(result.Error, "provider explanation") {
		t.Fatalf("result = %#v", result)
	}
	data, readErr := os.ReadFile(filepath.Join(outDir, "events.ndjson"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var event Event
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "error" || event.Detail != "provider_refusal" {
		t.Fatalf("terminal event = %#v", event)
	}
}

func TestEngineRejectsInvalidProviderAccounting(t *testing.T) {
	runner := &fakeRunner{outcome: ProcedureOutcome{
		Status:   StatusOK,
		Phase:    "closed",
		Provider: ProviderManagement{RequestCount: 1, UsageObservedCount: 2},
	}}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(registry).Run(context.Background(), validSimpleRequest(t, filepath.Join(t.TempDir(), "case")))
	assertClass(t, err, "procedure_result")
	if result.Status != StatusError || !strings.Contains(result.Error, "usage observations exceed requests") {
		t.Fatalf("result = %#v", result)
	}
}

func TestEngineRejectsInvalidRunnerOutcome(t *testing.T) {
	runner := &fakeRunner{outcome: ProcedureOutcome{Status: StatusOK}}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(registry).Run(context.Background(), validSimpleRequest(t, filepath.Join(t.TempDir(), "case")))
	assertClass(t, err, "procedure_result")
	if result.Status != StatusError {
		t.Fatalf("result status = %q", result.Status)
	}
}

func TestEngineRejectsInvalidRunnerOutcomeJSON(t *testing.T) {
	runner := &fakeRunner{outcome: ProcedureOutcome{
		Status:          StatusOK,
		Phase:           "closed",
		ProcedureResult: json.RawMessage(`{"broken"`),
	}}
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: runner}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(registry).Run(context.Background(), validSimpleRequest(t, filepath.Join(t.TempDir(), "case")))
	assertClass(t, err, "procedure_result")
	if result.Status != StatusError || !strings.Contains(result.Error, "invalid procedure result JSON") {
		t.Fatalf("result = %#v", result)
	}
}

type cancellationRunner struct{}

func (cancellationRunner) Run(ctx context.Context, _ ProcedureRequest) (ProcedureOutcome, error) {
	return ProcedureOutcome{}, ctx.Err()
}

func TestEnginePropagatesCancellationToRunner(t *testing.T) {
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: cancellationRunner{}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := NewEngine(registry).Run(ctx, validSimpleRequest(t, filepath.Join(t.TempDir(), "case")))
	assertClass(t, err, "canceled")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if result.Status != StatusError || result.ErrorClass != "canceled" || result.Error != context.Canceled.Error() {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Management.Processes) != 1 || result.Management.Processes[0].State != ProcessCanceled {
		t.Fatalf("management = %#v", result.Management)
	}
}

type failedRunnerWithCanceledCleanup struct {
	processErr error
	cleanupErr error
}

func (r failedRunnerWithCanceledCleanup) Run(context.Context, ProcedureRequest) (ProcedureOutcome, error) {
	return ProcedureOutcome{}, errors.Join(r.processErr, context.Canceled, r.cleanupErr)
}

func TestEngineKeepsPrimaryFailureWhenCleanupIsCanceled(t *testing.T) {
	processErr := errors.New("lawyer process failed")
	cleanupErr := errors.New("core cleanup failed")
	registry, err := NewRegistry(map[Procedure]Registration{ProcedureSimple: {Runner: failedRunnerWithCanceledCleanup{processErr: processErr, cleanupErr: cleanupErr}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(registry).Run(context.Background(), validSimpleRequest(t, filepath.Join(t.TempDir(), "case")))
	assertClass(t, err, "procedure_run")
	if !errors.Is(err, processErr) || !errors.Is(err, context.Canceled) || !errors.Is(err, cleanupErr) {
		t.Fatalf("error = %v, want primary and cleanup causes", err)
	}
	if result.Status != StatusError || result.ErrorClass != "procedure_run" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Management.Processes) != 1 || result.Management.Processes[0].State != ProcessFailed {
		t.Fatalf("management = %#v", result.Management)
	}
}

func TestRegistryRejectsInvalidRegistrations(t *testing.T) {
	if _, err := NewRegistry(map[Procedure]Registration{"other": {Runner: &fakeRunner{}}}); err == nil {
		t.Fatal("NewRegistry accepted an invalid procedure")
	}
	if _, err := NewRegistry(map[Procedure]Registration{ProcedureARB: {}}); err == nil {
		t.Fatal("NewRegistry accepted a nil runner")
	}
}

func TestEngineRejectsUnregisteredProcedureBeforeCreatingOutput(t *testing.T) {
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "case")
	request := validSimpleRequest(t, outDir)
	result, err := NewEngine(registry).Run(context.Background(), request)
	assertClass(t, err, "procedure_unavailable")
	if result.SchemaVersion != ResultSchemaVersion || result.Status != StatusError || result.ErrorClass != "procedure_unavailable" || result.RecordDir != "" {
		t.Fatalf("result = %#v", result)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Fatalf("output directory stat error = %v", statErr)
	}
}

func validSimpleRequest(t *testing.T, outDir string) Request {
	t.Helper()
	return Request{
		SchemaVersion: RequestSchemaVersion,
		Procedure:     ProcedureSimple,
		CaseID:        "case-1",
		RunID:         "run-1",
		Proposition:   "The proposition",
		SettingsFile:  simpleSettingsFile(t),
		OutDir:        outDir,
	}
}

func simpleSettingsFile(t *testing.T) string {
	t.Helper()
	return writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":10,"per_file_bytes":100,"total_bytes":1000}},
  "procedures":{"simple":{"model":"openai://model"}}
}`)
}

func assertClass(t *testing.T, err error, class string) {
	t.Helper()
	var classifiedErr *Error
	if !errors.As(err, &classifiedErr) || classifiedErr.Class != class {
		t.Fatalf("error = %v, want class %q", err, class)
	}
}
