package proceeding

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/casemanifest"
	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/common/recordio"
)

func Run(ctx context.Context, opts Options) (Result, error) {
	return RunWithClientFactory(ctx, opts, directClientFactory{})
}

func RunWithClientFactory(ctx context.Context, opts Options, factory ClientFactory) (Result, error) {
	startedAt := time.Now().UTC()
	resolved, spec, runtime, err := resolveOptions(opts, startedAt)
	if err != nil {
		return Result{}, err
	}
	if factory == nil {
		return Result{}, fmt.Errorf("response client factory is required")
	}
	if err := createEmptyOutputDir(resolved.OutputDir); err != nil {
		return Result{}, err
	}
	manifest := casemanifest.New(casemanifest.ProcedureSimple, resolved.CaseID, resolved.RunID, startedAt)
	if err := casemanifest.WriteAtomic(resolved.OutputDir, manifest); err != nil {
		return Result{}, fmt.Errorf("write case manifest: %w", err)
	}
	if err := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "input.json"), InputRecord{
		SchemaVersion:    InputSchemaVersion,
		CaseID:           resolved.CaseID,
		RunID:            resolved.RunID,
		Proposition:      resolved.Proposition,
		EvidenceStandard: resolved.EvidenceStandard,
		DocumentsPresent: resolved.DocumentsRoot != "",
		DocumentsRoot:    resolved.DocumentsRoot,
	}); err != nil {
		return Result{}, err
	}
	if err := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "runtime.json"), runtime); err != nil {
		return Result{}, err
	}

	recorder := &eventRecorder{path: filepath.Join(resolved.OutputDir, "events.ndjson")}
	documentManifest, err := importDocuments(resolved)
	if err != nil {
		documentManifest = documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}}
		writeErr := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "documents.json"), documentManifest)
		requestRecord := ModelRequestRecord{
			SchemaVersion:    RequestSchemaVersion,
			RequestSpec:      redactRequestSpec(spec),
			DeveloperPrompt:  developerPrompt(resolved.EvidenceStandard),
			Proposition:      resolved.Proposition,
			EvidenceStandard: resolved.EvidenceStandard,
			Documents:        documentManifest.Files,
			Tools:            decisionTools(),
			Error:            err.Error(),
		}
		writeErr = errors.Join(writeErr, recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-request.json"), requestRecord))
		writeErr = errors.Join(writeErr, recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), ModelResponseRecord{SchemaVersion: ResponseSchemaVersion, Status: "not_sent", Error: err.Error(), ErrorClass: "input"}))
		if writeErr != nil {
			return Result{}, errors.Join(err, writeErr)
		}
		return finishError(resolved, startedAt, documentManifest, openaiapi.Response{}, "input", err, recorder)
	}
	if err := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "documents.json"), documentManifest); err != nil {
		return Result{}, err
	}
	if err := recorder.append("run_initialized", map[string]any{
		"case_id":        resolved.CaseID,
		"run_id":         resolved.RunID,
		"document_count": len(documentManifest.Files),
	}); err != nil {
		return Result{}, err
	}

	tools := decisionTools()
	inputItems, requestErr := buildInputItems(resolved.Proposition, resolved.EvidenceStandard, filepath.Join(resolved.OutputDir, "documents"), documentManifest)
	requestRecord := ModelRequestRecord{
		SchemaVersion:    RequestSchemaVersion,
		RequestSpec:      redactRequestSpec(spec),
		DeveloperPrompt:  developerPrompt(resolved.EvidenceStandard),
		Proposition:      resolved.Proposition,
		EvidenceStandard: resolved.EvidenceStandard,
		Documents:        documentManifest.Files,
		Tools:            tools,
	}
	if requestErr != nil {
		requestRecord.Error = requestErr.Error()
	}
	if err := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-request.json"), requestRecord); err != nil {
		return Result{}, err
	}
	if requestErr != nil {
		responseRecord := ModelResponseRecord{SchemaVersion: ResponseSchemaVersion, Status: "not_sent", Error: requestErr.Error(), ErrorClass: "input"}
		if err := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), responseRecord); err != nil {
			return Result{}, errors.Join(requestErr, err)
		}
		return finishError(resolved, startedAt, documentManifest, openaiapi.Response{}, "input", requestErr, recorder)
	}

	client, err := factory.New(spec, time.Duration(runtime.ProviderTimeoutSeconds)*time.Second)
	if err != nil {
		errorClass := errorClass(err)
		responseRecord := ModelResponseRecord{SchemaVersion: ResponseSchemaVersion, Status: "error", Error: err.Error(), ErrorClass: errorClass}
		if writeErr := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), responseRecord); writeErr != nil {
			return Result{}, errors.Join(err, writeErr)
		}
		return finishError(resolved, startedAt, documentManifest, openaiapi.Response{}, errorClass, err, recorder)
	}
	if client == nil {
		err := fmt.Errorf("response client factory returned a nil client")
		if writeErr := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), ModelResponseRecord{SchemaVersion: ResponseSchemaVersion, Status: "error", Error: err.Error()}); writeErr != nil {
			return Result{}, errors.Join(err, writeErr)
		}
		return finishError(resolved, startedAt, documentManifest, openaiapi.Response{}, "", err, recorder)
	}
	if err := recorder.append("provider_request_started", map[string]any{"model": spec.RuntimeModel()}); err != nil {
		writeErr := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), ModelResponseRecord{SchemaVersion: ResponseSchemaVersion, Status: "not_sent", Error: err.Error(), ErrorClass: "storage"})
		if writeErr != nil {
			return Result{}, errors.Join(err, writeErr)
		}
		return finishError(resolved, startedAt, documentManifest, openaiapi.Response{}, "storage", err, recorder)
	}
	response, providerErr := client.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, "")
	responseRecord := modelResponseRecord(response, providerErr)
	if err := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), responseRecord); err != nil {
		return Result{}, errors.Join(providerErr, err)
	}
	if providerErr != nil {
		return finishError(resolved, startedAt, documentManifest, response, errorClass(providerErr), providerErr, recorder)
	}
	if err := recorder.append("provider_response_received", map[string]any{"response_id": response.ResponseID}); err != nil {
		return finishError(resolved, startedAt, documentManifest, response, "storage", err, recorder)
	}
	decision, err := parseDecision(response)
	if err != nil {
		responseRecord = modelResponseRecord(response, err)
		if writeErr := recordio.WriteJSON(filepath.Join(resolved.OutputDir, "model-response.json"), responseRecord); writeErr != nil {
			return Result{}, errors.Join(err, writeErr)
		}
		return finishError(resolved, startedAt, documentManifest, response, errorClass(err), err, recorder)
	}
	return finishSuccess(resolved, startedAt, documentManifest, response, decision, recorder)
}

func resolveOptions(opts Options, now time.Time) (Options, modelrequest.Spec, Runtime, error) {
	opts.Proposition = strings.TrimSpace(opts.Proposition)
	opts.DocumentsRoot = strings.TrimSpace(opts.DocumentsRoot)
	opts.OutputDir = strings.TrimSpace(opts.OutputDir)
	opts.CaseID = strings.TrimSpace(opts.CaseID)
	opts.RunID = strings.TrimSpace(opts.RunID)
	opts.RequestSpecPath = strings.TrimSpace(opts.RequestSpecPath)
	opts.Model = strings.TrimSpace(opts.Model)
	opts.EvidenceStandard = strings.TrimSpace(opts.EvidenceStandard)
	if opts.Proposition == "" {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("proposition is required")
	}
	if opts.OutputDir == "" {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("output directory is required")
	}
	if opts.EvidenceStandard == "" {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("evidence standard is required")
	}
	if !opts.AllowAPIKey {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("--allow-api-key is required before a provider request")
	}
	if (opts.RequestSpecPath == "") == (opts.Model == "") {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("exactly one of --request-spec or --model is required")
	}
	if opts.MaxDocumentBytes <= 0 {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("--max-document-bytes must be positive")
	}
	if opts.MaxDocuments <= 0 {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("--max-documents must be positive")
	}
	if opts.MaxDocumentsBytes <= 0 {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("--max-documents-bytes must be positive")
	}
	if opts.TimeoutSeconds < 0 {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("--timeout-seconds must be positive when set")
	}
	if opts.TimeoutSeconds == 0 {
		opts.TimeoutSeconds = DefaultTimeoutSeconds
	}
	if opts.CaseID == "" {
		opts.CaseID = DefaultCaseID
	}
	if opts.RunID == "" {
		opts.RunID = fmt.Sprintf("run-%d", now.UnixNano())
	}
	spec, err := loadRequestSpec(opts)
	if err != nil {
		return Options{}, modelrequest.Spec{}, Runtime{}, err
	}
	if supported, known := spec.SupportsParameter("tools"); known && !supported {
		return Options{}, modelrequest.Spec{}, Runtime{}, fmt.Errorf("request spec endpoint metadata omits required parameter tools")
	}
	spec = spec.WithFallbackMaxOutputTokens(DefaultMaxOutputTokens)
	runtime := Runtime{
		SchemaVersion:          RuntimeSchemaVersion,
		EvidenceStandard:       opts.EvidenceStandard,
		AllowAPIKey:            opts.AllowAPIKey,
		MaxDocuments:           opts.MaxDocuments,
		MaxDocumentBytes:       opts.MaxDocumentBytes,
		MaxDocumentsBytes:      opts.MaxDocumentsBytes,
		ProviderTimeoutSeconds: opts.TimeoutSeconds,
		ProviderAttempts:       1,
	}
	return opts, spec, runtime, nil
}

func loadRequestSpec(opts Options) (modelrequest.Spec, error) {
	if opts.RequestSpecPath != "" {
		raw, err := os.ReadFile(opts.RequestSpecPath)
		if err != nil {
			return modelrequest.Spec{}, fmt.Errorf("read request spec %s: %w", opts.RequestSpecPath, err)
		}
		spec, err := modelrequest.ParseJSON(raw)
		if err != nil {
			return modelrequest.Spec{}, fmt.Errorf("parse request spec %s: %w", opts.RequestSpecPath, err)
		}
		return spec, nil
	}
	model, err := modelrequest.ParseModelRef(opts.Model)
	if err != nil {
		return modelrequest.Spec{}, err
	}
	modelID := model.Model
	if model.Query != "" {
		modelID += "?" + model.Query
	}
	return modelrequest.Spec{Endpoint: model.Endpoint, Model: modelID}, nil
}

func createEmptyOutputDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create output directory %s: %w", path, err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read output directory %s: %w", path, err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory %s is not empty", path)
	}
	return nil
}

func importDocuments(opts Options) (documents.Manifest, error) {
	destination := filepath.Join(opts.OutputDir, "documents")
	if opts.DocumentsRoot == "" {
		if err := os.Mkdir(destination, 0o755); err != nil {
			return documents.Manifest{}, fmt.Errorf("create empty documents directory: %w", err)
		}
		return documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}}, nil
	}
	return documents.Import(opts.DocumentsRoot, destination, documents.Limits{
		MaxFiles:      opts.MaxDocuments,
		MaxFileBytes:  opts.MaxDocumentBytes,
		MaxTotalBytes: opts.MaxDocumentsBytes,
	})
}

type eventRecorder struct {
	path     string
	sequence int
}

func (r *eventRecorder) append(eventType string, payload map[string]any) error {
	sequence := r.sequence + 1
	if err := recordio.AppendJSONLine(r.path, Event{
		SchemaVersion: EventSchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Sequence:      sequence,
		Type:          eventType,
		Payload:       payload,
	}); err != nil {
		return err
	}
	r.sequence = sequence
	return nil
}

func redactRequestSpec(spec modelrequest.Spec) modelrequest.Spec {
	redacted := spec
	if len(spec.Headers) != 0 {
		redacted.Headers = make(map[string]string, len(spec.Headers))
		for name := range spec.Headers {
			redacted.Headers[name] = "[redacted]"
		}
	}
	return redacted
}

func modelResponseRecord(response openaiapi.Response, err error) ModelResponseRecord {
	status := "ok"
	if err != nil {
		status = "error"
	}
	return ModelResponseRecord{
		SchemaVersion:           ResponseSchemaVersion,
		Status:                  status,
		ResponseID:              response.ResponseID,
		Text:                    response.Text,
		ToolCalls:               recordedToolCalls(response.ToolCalls),
		RawResponse:             response.RawJSON,
		ProviderMetadata:        response.OpenRouterMetadata,
		ProviderGeneration:      response.OpenRouterGeneration,
		ProviderGenerationError: response.OpenRouterGenerationError,
		RecoveredCostUSD:        response.OpenRouterCostUSD,
		Error:                   errorText(err),
		ErrorClass:              errorClass(err),
	}
}

func recordedToolCalls(calls []openaiapi.ToolCall) []ToolCallRecord {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCallRecord, 0, len(calls))
	for _, call := range calls {
		out = append(out, ToolCallRecord{
			CallID:         call.CallID,
			Name:           call.Name,
			Arguments:      call.Arguments,
			RawArguments:   call.RawArguments,
			ArgumentsError: call.ArgumentsError,
		})
	}
	return out
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func errorClass(err error) string {
	if err == nil {
		return ""
	}
	if class := openaiapi.ErrorClass(err); class != "" {
		return string(class)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "canceled"
	}
	return ""
}
