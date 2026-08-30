package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/runtime/runstate"
)

type Error struct {
	Class string
	Err   error
}

func (e *Error) Error() string {
	return e.Class + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

type Engine struct {
	registry     Registry
	now          func() time.Time
	writeRunJSON func(string, any) error
}

func NewEngine(registry Registry) *Engine {
	return &Engine{registry: registry, now: time.Now}
}

func (e *Engine) Run(ctx context.Context, request Request) (Result, error) {
	started := e.now().UTC()
	if err := validateRequest(request); err != nil {
		return e.earlyError(request, Capabilities{}, started, "invalid_request", err, "")
	}
	registration, ok := e.registry.Lookup(request.Procedure)
	if !ok {
		return e.earlyError(request, Capabilities{}, started, "procedure_unavailable", fmt.Errorf("procedure %q is not registered", request.Procedure), "")
	}
	settings, err := LoadSettings(request.SettingsFile, request.Procedure)
	if err != nil {
		return e.earlyError(request, registration.Capabilities, started, "invalid_settings", err, "")
	}
	outDir := request.OutDir
	if outDir == "" {
		outDir = filepath.Join(settings.Common.OutputRoot, request.CaseID, request.RunID)
	} else {
		outDir, err = filepath.Abs(outDir)
		if err != nil {
			return e.earlyError(request, registration.Capabilities, started, "invalid_request", fmt.Errorf("resolve output directory: %w", err), "")
		}
	}
	request.OutDir = outDir
	if err := createRecordDirectories(outDir); err != nil {
		return e.earlyError(request, registration.Capabilities, started, "record_create", err, outDir)
	}
	controllerInstanceID, err := runstate.ProcessInstanceID(os.Getpid())
	if err != nil {
		return e.earlyError(request, registration.Capabilities, started, "process_identity", err, outDir)
	}
	initial := Result{
		SchemaVersion: ResultSchemaVersion,
		Procedure:     request.Procedure,
		CaseID:        request.CaseID,
		RunID:         request.RunID,
		StartedAt:     started,
		Status:        StatusRunning,
		Phase:         "dispatch",
		Capabilities:  registration.Capabilities,
		RecordDir:     outDir,
		Management: Management{
			UpdatedAt: started,
			Processes: []ManagedProcess{{
				Name:       "adjudicate",
				Kind:       "controller",
				PID:        os.Getpid(),
				InstanceID: controllerInstanceID,
				State:      ProcessRunning,
				StartedAt:  started,
			}},
		},
	}
	recorder, err := newRunRecorder(filepath.Join(outDir, "run.json"), initial, e.writeRunJSON)
	if err != nil {
		return e.earlyError(request, registration.Capabilities, started, "record_write", err, outDir)
	}

	events := NewEventLog(filepath.Join(outDir, "events.ndjson"))
	baseEvent := Event{Time: started, Procedure: request.Procedure, CaseID: request.CaseID, RunID: request.RunID}
	baseEvent.Type = "dispatch"
	if err := events.Append(baseEvent); err != nil {
		return e.publishError(request, registration.Capabilities, started, "record_write", err, events, recorder)
	}
	inputsDir := filepath.Join(outDir, "inputs")
	manifest, err := ImportDocuments(request.Documents.Root, inputsDir, settings.Common.DocumentLimits)
	if err != nil {
		return e.publishError(request, registration.Capabilities, started, "document_import", err, events, recorder)
	}
	if err := WriteJSONAtomic(filepath.Join(inputsDir, "adjudicate-request.json"), request); err != nil {
		return e.publishError(request, registration.Capabilities, started, "record_write", err, events, recorder)
	}
	if err := WriteJSONAtomic(filepath.Join(inputsDir, "resolved-settings.json"), settings); err != nil {
		return e.publishError(request, registration.Capabilities, started, "record_write", err, events, recorder)
	}
	if err := WriteJSONAtomic(filepath.Join(inputsDir, "documents.json"), manifest); err != nil {
		return e.publishError(request, registration.Capabilities, started, "record_write", err, events, recorder)
	}
	importedEvent := baseEvent
	importedEvent.Time = e.now().UTC()
	importedEvent.Type = "documents_imported"
	importedEvent.Detail = fmt.Sprintf("%d documents, %d bytes", len(manifest.Documents), manifest.TotalBytes)
	if err := events.Append(importedEvent); err != nil {
		return e.publishError(request, registration.Capabilities, started, "record_write", err, events, recorder)
	}
	if err := recorder.update(func(result *Result) {
		result.Phase = "documents_imported"
		result.Management.UpdatedAt = importedEvent.Time
	}); err != nil {
		return e.publishError(request, registration.Capabilities, started, "record_write", err, events, recorder)
	}

	procedureRequest := ProcedureRequest{
		Request:   request,
		Settings:  settings,
		Documents: manifest,
		RecordDir: outDir,
		CoreDir:   filepath.Join(outDir, "core"),
		LogsDir:   filepath.Join(outDir, "logs"),
		Observer:  recorder,
	}
	var session *sessionLease
	if registration.Capabilities.Sessions {
		procedureRequest.SessionDir = filepath.Join(settings.Common.AgentStateRoot, request.CaseID, string(request.Procedure))
		session, err = acquireSessionLease(procedureRequest.SessionDir)
		if err != nil {
			class := "session_lock"
			if errors.Is(err, errSessionInUse) {
				class = "session_in_use"
			}
			return e.publishError(request, registration.Capabilities, started, class, err, events, recorder)
		}
	}
	outcome, runErr := registration.Runner.Run(ctx, procedureRequest)
	if session != nil {
		releaseErr := session.close()
		if runErr != nil {
			runErr = errors.Join(runErr, releaseErr)
		} else if releaseErr != nil {
			return e.publishError(request, registration.Capabilities, started, "session_release", releaseErr, events, recorder)
		}
	}
	if runErr != nil {
		return e.publishError(request, registration.Capabilities, started, runnerErrorClass(runErr), runErr, events, recorder)
	}
	if err := validateOutcome(outcome); err != nil {
		return e.publishError(request, registration.Capabilities, started, "procedure_result", &CoreRunError{Err: err, Provider: outcome.Provider}, events, recorder)
	}
	finished := e.now().UTC()
	result := Result{
		SchemaVersion:   ResultSchemaVersion,
		Procedure:       request.Procedure,
		CaseID:          request.CaseID,
		RunID:           request.RunID,
		StartedAt:       started,
		FinishedAt:      &finished,
		Status:          outcome.Status,
		Phase:           outcome.Phase,
		Decision:        outcome.Decision,
		Capabilities:    registration.Capabilities,
		ProcedureResult: cloneJSON(outcome.ProcedureResult),
		Failure:         cloneJSON(outcome.Failure),
		RecordDir:       outDir,
	}
	terminalEvent := baseEvent
	terminalEvent.Time = finished
	terminalEvent.Type = "terminal"
	terminalEvent.Detail = string(outcome.Status)
	if err := events.Append(terminalEvent); err != nil {
		return e.publishOutcomeError(result, outcome.Provider, err, events, recorder, false)
	}
	if err := recorder.update(func(current *Result) {
		management := current.Management
		*current = result
		current.Management = management
		current.Management.Provider = outcome.Provider
		finishController(&current.Management, finished, ProcessCompleted, "")
	}); err != nil {
		return e.publishOutcomeError(result, outcome.Provider, err, events, recorder, true)
	}
	return recorder.snapshot(), nil
}

func (e *Engine) publishOutcomeError(result Result, provider ProviderManagement, operationErr error, events *EventLog, recorder *runRecorder, appendErrorEvent bool) (Result, error) {
	failed := cloneResult(result)
	failed.Status = StatusError
	failed.Phase = "error"
	failed.ErrorClass = "record_write"
	failed.Error = operationErr.Error()
	failed.Management = recorder.snapshot().Management
	failed.Management.Provider = provider
	finishController(&failed.Management, *failed.FinishedAt, ProcessFailed, operationErr.Error())

	combined := operationErr
	if appendErrorEvent {
		event := Event{
			Time:      *failed.FinishedAt,
			Type:      "error",
			Procedure: failed.Procedure,
			CaseID:    failed.CaseID,
			RunID:     failed.RunID,
			Detail:    "record_write",
		}
		if err := events.Append(event); err != nil {
			combined = errors.Join(combined, fmt.Errorf("write error event: %w", err))
		}
	}
	if err := recorder.update(func(current *Result) {
		*current = cloneResult(failed)
	}); err != nil {
		combined = errors.Join(combined, fmt.Errorf("write error result: %w", err))
	} else {
		failed = recorder.snapshot()
	}
	return failed, classified("record_write", combined)
}

func (e *Engine) earlyError(request Request, capabilities Capabilities, started time.Time, class string, operationErr error, recordDir string) (Result, error) {
	finished := e.now().UTC()
	result := Result{
		SchemaVersion: ResultSchemaVersion,
		Procedure:     request.Procedure,
		CaseID:        request.CaseID,
		RunID:         request.RunID,
		StartedAt:     started,
		FinishedAt:    &finished,
		Status:        StatusError,
		Phase:         "error",
		Capabilities:  capabilities,
		ErrorClass:    class,
		Error:         operationErr.Error(),
		RecordDir:     recordDir,
		Management: Management{
			UpdatedAt: finished,
			Processes: []ManagedProcess{{
				Name:       "adjudicate",
				Kind:       "controller",
				PID:        os.Getpid(),
				State:      ProcessFailed,
				StartedAt:  started,
				FinishedAt: &finished,
				Error:      operationErr.Error(),
			}},
		},
	}
	return result, classified(class, operationErr)
}

func runnerErrorClass(err error) string {
	for err != nil {
		if classified, ok := err.(interface{ ErrorClass() string }); ok {
			if class := strings.TrimSpace(classified.ErrorClass()); class != "" {
				return class
			}
		}
		if err == context.Canceled || err == context.DeadlineExceeded {
			return "canceled"
		}
		switch wrapped := err.(type) {
		case interface{ Unwrap() []error }:
			causes := wrapped.Unwrap()
			if len(causes) == 0 {
				return "procedure_run"
			}
			err = causes[0]
		case interface{ Unwrap() error }:
			err = wrapped.Unwrap()
		default:
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "canceled"
			}
			return "procedure_run"
		}
	}
	return "procedure_run"
}

func (e *Engine) publishError(request Request, capabilities Capabilities, started time.Time, class string, operationErr error, events *EventLog, recorder *runRecorder) (Result, error) {
	provider := providerAccountingFromError(operationErr)
	if providerErr := validateProviderAccounting(provider); providerErr != nil {
		operationErr = errors.Join(operationErr, fmt.Errorf("invalid provider accounting: %w", providerErr))
		class = "procedure_result"
		provider = ProviderManagement{}
	}
	finished := e.now().UTC()
	result := Result{
		SchemaVersion: ResultSchemaVersion,
		Procedure:     request.Procedure,
		CaseID:        request.CaseID,
		RunID:         request.RunID,
		StartedAt:     started,
		FinishedAt:    &finished,
		Status:        StatusError,
		Phase:         "error",
		Capabilities:  capabilities,
		ErrorClass:    class,
		Error:         operationErr.Error(),
		RecordDir:     request.OutDir,
	}
	state := ProcessFailed
	if class == "canceled" {
		state = ProcessCanceled
	}
	event := Event{
		Time:      finished,
		Type:      "error",
		Procedure: request.Procedure,
		CaseID:    request.CaseID,
		RunID:     request.RunID,
		Detail:    class,
	}
	eventErr := events.Append(event)
	recordErr := recorder.update(func(current *Result) {
		management := current.Management
		*current = result
		current.Management = management
		current.Management.Provider = provider
		finishController(&current.Management, finished, state, operationErr.Error())
	})
	combined := operationErr
	if eventErr != nil {
		combined = errors.Join(combined, fmt.Errorf("write error event: %w", eventErr))
	}
	if recordErr != nil {
		combined = errors.Join(combined, fmt.Errorf("write error result: %w", recordErr))
	}
	if recordErr == nil {
		result = recorder.snapshot()
	}
	return result, classified(class, combined)
}

func providerAccountingFromError(err error) ProviderManagement {
	var provider interface{ ProviderAccounting() ProviderManagement }
	if errors.As(err, &provider) {
		return provider.ProviderAccounting()
	}
	return ProviderManagement{}
}

func finishController(management *Management, finished time.Time, state ProcessState, message string) {
	management.UpdatedAt = finished
	for index := range management.Processes {
		if management.Processes[index].Kind != "controller" {
			continue
		}
		management.Processes[index].State = state
		management.Processes[index].FinishedAt = &finished
		management.Processes[index].Error = message
		return
	}
}

func validateRequest(request Request) error {
	if request.SchemaVersion != RequestSchemaVersion {
		return fmt.Errorf("schema_version must be %q", RequestSchemaVersion)
	}
	if !request.Procedure.Valid() {
		return fmt.Errorf("invalid procedure %q", request.Procedure)
	}
	if strings.TrimSpace(request.CaseID) == "" {
		return fmt.Errorf("case_id is empty")
	}
	if !singlePathComponent(request.CaseID) {
		return fmt.Errorf("case_id must be one path component")
	}
	if strings.TrimSpace(request.RunID) == "" {
		return fmt.Errorf("run_id is empty")
	}
	if !singlePathComponent(request.RunID) {
		return fmt.Errorf("run_id must be one path component")
	}
	if strings.TrimSpace(request.Proposition) == "" {
		return fmt.Errorf("proposition is empty")
	}
	if strings.TrimSpace(request.SettingsFile) == "" {
		return fmt.Errorf("settings_file is empty")
	}
	return nil
}

func createRecordDirectories(outDir string) error {
	if err := os.MkdirAll(filepath.Dir(outDir), 0o755); err != nil {
		return fmt.Errorf("create output parent directory: %w", err)
	}
	if err := os.Mkdir(outDir, 0o755); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("output directory %q already exists", outDir)
		}
		return fmt.Errorf("create output directory %q: %w", outDir, err)
	}
	for _, name := range []string{"inputs", "core", "logs"} {
		if err := os.Mkdir(filepath.Join(outDir, name), 0o755); err != nil {
			return fmt.Errorf("create %s directory: %w", name, err)
		}
	}
	return nil
}

func validateOutcome(outcome ProcedureOutcome) error {
	if err := validateProviderAccounting(outcome.Provider); err != nil {
		return err
	}
	if outcome.Status != StatusOK && outcome.Status != StatusFailed {
		return fmt.Errorf("runner returned invalid terminal status %q", outcome.Status)
	}
	if strings.TrimSpace(outcome.Phase) == "" {
		return fmt.Errorf("runner returned an empty phase")
	}
	if outcome.Status == StatusFailed && len(outcome.Failure) == 0 {
		return fmt.Errorf("runner returned failed status without failure data")
	}
	if len(outcome.ProcedureResult) > 0 && !json.Valid(outcome.ProcedureResult) {
		return fmt.Errorf("runner returned invalid procedure result JSON")
	}
	if len(outcome.Failure) > 0 && !json.Valid(outcome.Failure) {
		return fmt.Errorf("runner returned invalid failure JSON")
	}
	return nil
}

func validateProviderAccounting(provider ProviderManagement) error {
	if provider.RequestCount < 0 || provider.UsageObservedCount < 0 || provider.CostObservedCount < 0 {
		return fmt.Errorf("provider accounting counts must be nonnegative")
	}
	if provider.UsageObservedCount > provider.RequestCount {
		return fmt.Errorf("provider usage observations exceed requests")
	}
	if provider.CostObservedCount > provider.RequestCount {
		return fmt.Errorf("provider cost observations exceed requests")
	}
	if (provider.UsageObservedCount == 0) != (provider.Usage == nil) {
		return fmt.Errorf("provider usage and observation count disagree")
	}
	if provider.Usage != nil {
		usage := provider.Usage
		if usage.InputTokens < 0 || usage.CachedInputTokens < 0 || usage.CacheWriteInputTokens < 0 || usage.OutputTokens < 0 || usage.ReasoningTokens < 0 || usage.TotalTokens < 0 {
			return fmt.Errorf("provider token counts must be nonnegative")
		}
	}
	if (provider.CostObservedCount == 0) != (provider.CostUSD == nil) {
		return fmt.Errorf("provider cost and observation count disagree")
	}
	if provider.CostUSD != nil && (*provider.CostUSD < 0 || math.IsNaN(*provider.CostUSD) || math.IsInf(*provider.CostUSD, 0)) {
		return fmt.Errorf("provider cost must be finite and nonnegative")
	}
	return nil
}

func singlePathComponent(value string) bool {
	return value != "." && value != ".." && filepath.Base(value) == value && !strings.ContainsAny(value, `/\\`)
}

func classified(class string, err error) error {
	return &Error{Class: class, Err: err}
}

func cloneJSON(value []byte) []byte {
	return append([]byte(nil), value...)
}
