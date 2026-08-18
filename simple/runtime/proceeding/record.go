package proceeding

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/documents"
	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/common/recordio"
)

func finishSuccess(opts Options, startedAt time.Time, manifest documents.Manifest, response openaiapi.Response, decision Decision, recorder *eventRecorder) (Result, error) {
	result := terminalResult(opts, startedAt, manifest, response, "ok", "closed", &decision, "", "")
	if err := recorder.append("decision_submitted", map[string]any{"decision": decision.Value}); err != nil {
		return finishError(opts, startedAt, manifest, response, "storage", err, recorder)
	}
	if err := recorder.append("run_completed", map[string]any{"status": result.Status}); err != nil {
		return finishError(opts, startedAt, manifest, response, "storage", err, recorder)
	}
	if err := writeTerminalRecords(opts.OutputDir, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func finishError(opts Options, startedAt time.Time, manifest documents.Manifest, response openaiapi.Response, class string, runErr error, recorder *eventRecorder) (Result, error) {
	result := terminalResult(opts, startedAt, manifest, response, "error", "error", nil, runErr.Error(), class)
	recordErr := errors.Join(
		recorder.append("run_failed", map[string]any{"error": runErr.Error(), "error_class": class}),
		writeTerminalRecords(opts.OutputDir, result),
	)
	if recordErr != nil {
		return result, errors.Join(runErr, recordErr)
	}
	return result, runErr
}

func terminalResult(opts Options, startedAt time.Time, manifest documents.Manifest, response openaiapi.Response, status, phase string, decision *Decision, errorMessage, errorClass string) Result {
	return Result{
		SchemaVersion:    RunSchemaVersion,
		Procedure:        "simple",
		CaseID:           opts.CaseID,
		RunID:            opts.RunID,
		StartedAt:        startedAt.Format(time.RFC3339Nano),
		FinishedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		Status:           status,
		Phase:            phase,
		Proposition:      opts.Proposition,
		EvidenceStandard: opts.EvidenceStandard,
		Decision:         decision,
		Error:            strings.TrimSpace(errorMessage),
		ErrorClass:       strings.TrimSpace(errorClass),
		ResponseID:       response.ResponseID,
		ProviderCostUSD:  response.OpenRouterCostUSD,
		Documents:        manifest,
	}
}

func writeTerminalRecords(outputDir string, result Result) error {
	decisionRecord := DecisionRecord{
		SchemaVersion: DecisionSchemaVersion,
		Status:        result.Status,
		Decision:      result.Decision,
		Error:         result.Error,
		ErrorClass:    result.ErrorClass,
	}
	if err := recordio.WriteJSON(filepath.Join(outputDir, "decision.json"), decisionRecord); err != nil {
		return err
	}
	state := StateRecord{
		SchemaVersion:    StateSchemaVersion,
		CaseID:           result.CaseID,
		RunID:            result.RunID,
		Status:           result.Status,
		Phase:            result.Phase,
		Proposition:      result.Proposition,
		EvidenceStandard: result.EvidenceStandard,
		Documents:        result.Documents,
		Decision:         result.Decision,
		Error:            result.Error,
		ErrorClass:       result.ErrorClass,
	}
	if err := recordio.WriteJSON(filepath.Join(outputDir, "state.json"), state); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "transcript.md"), []byte(renderTranscript(result)), 0o644); err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "digest.md"), []byte(renderDigest(result)), 0o644); err != nil {
		return fmt.Errorf("write digest: %w", err)
	}
	if err := recordio.WriteJSONAtomic(filepath.Join(outputDir, "run.json"), result); err != nil {
		return err
	}
	return nil
}

func renderTranscript(result Result) string {
	var b strings.Builder
	b.WriteString("# Simple Adjudication Transcript\n\n")
	b.WriteString("## Proposition\n\n")
	b.WriteString(result.Proposition)
	b.WriteString("\n\n## Evidence Standard\n\n")
	b.WriteString(result.EvidenceStandard)
	b.WriteString("\n\n## Documents\n\n")
	if len(result.Documents.Files) == 0 {
		b.WriteString("No documents were imported.\n")
	} else {
		for _, document := range result.Documents.Files {
			fmt.Fprintf(&b, "- `%s`: %d bytes, `%s`, SHA-256 `%s`\n", document.Path, document.SizeBytes, document.MediaType, document.SHA256)
		}
	}
	b.WriteString("\n## Decision\n\n")
	if result.Decision != nil {
		fmt.Fprintf(&b, "Decision: `%s`\n\n%s\n", result.Decision.Value, result.Decision.Rationale)
	} else {
		fmt.Fprintf(&b, "Status: `%s`\n\nError: %s\n", result.Status, result.Error)
	}
	return b.String()
}

func renderDigest(result Result) string {
	var b strings.Builder
	b.WriteString("# Simple Adjudication Digest\n\n")
	fmt.Fprintf(&b, "Case: `%s`\n\nRun: `%s`\n\nStatus: `%s`\n\n", result.CaseID, result.RunID, result.Status)
	b.WriteString("## Proposition\n\n")
	b.WriteString(result.Proposition)
	b.WriteString("\n\n## Evidence Standard\n\n")
	b.WriteString(result.EvidenceStandard)
	b.WriteString("\n\n## Result\n\n")
	if result.Decision != nil {
		fmt.Fprintf(&b, "Decision: `%s`\n\n%s\n", result.Decision.Value, result.Decision.Rationale)
	} else {
		fmt.Fprintf(&b, "Error class: `%s`\n\nError: %s\n", result.ErrorClass, result.Error)
	}
	return b.String()
}
