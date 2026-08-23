package proceeding

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/documents"
	openaiapi "github.com/agentcourt/adj/common/openai"
	"github.com/agentcourt/adj/common/recordio"
)

func finishSuccess(opts Options, startedAt time.Time, manifest documents.Manifest, response openaiapi.Response, decision Decision, recorder *eventRecorder) (Result, error) {
	provider := providerAccounting(response)
	result := terminalResult(opts, startedAt, manifest, response, provider, "ok", "closed", &decision, "", "")
	if err := recorder.append("decision_submitted", map[string]any{"decision": decision.Value}); err != nil {
		return finishError(opts, startedAt, manifest, response, provider, "storage", err, recorder)
	}
	if err := recorder.append("run_completed", map[string]any{"status": result.Status}); err != nil {
		return finishError(opts, startedAt, manifest, response, provider, "storage", err, recorder)
	}
	if err := writeTerminalRecords(opts.OutputDir, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func finishError(opts Options, startedAt time.Time, manifest documents.Manifest, response openaiapi.Response, provider openaiapi.Accounting, class string, runErr error, recorder *eventRecorder) (Result, error) {
	result := terminalResult(opts, startedAt, manifest, response, provider, "error", "error", nil, runErr.Error(), class)
	recordErr := errors.Join(
		recorder.append("run_failed", map[string]any{"error": runErr.Error(), "error_class": class}),
		writeTerminalRecords(opts.OutputDir, result),
	)
	if recordErr != nil {
		return result, errors.Join(runErr, recordErr)
	}
	return result, runErr
}

func terminalResult(opts Options, startedAt time.Time, manifest documents.Manifest, response openaiapi.Response, provider openaiapi.Accounting, status, phase string, decision *Decision, errorMessage, errorClass string) Result {
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
		WebSearch:        summarizeWebSearch(opts, response),
		ReasoningEffort:  opts.ReasoningEffort,
		MaxOutputTokens:  opts.MaxOutputTokens,
		MaxToolCalls:     opts.MaxToolCalls,
		Provider:         provider,
		Documents:        manifest,
	}
}

func summarizeWebSearch(opts Options, response openaiapi.Response) WebSearchSummary {
	enabled := true
	if opts.WebSearch != nil {
		enabled = *opts.WebSearch
	}
	return WebSearchSummary{
		Enabled:       enabled,
		CallCount:     len(response.WebSearchCalls),
		SourceCount:   webSearchSourceCount(response.WebSearchCalls),
		CitationCount: len(response.URLCitations),
	}
}

func providerAccounting(response openaiapi.Response) openaiapi.Accounting {
	recorder := &openaiapi.AccountingRecorder{}
	recorder.Record(response)
	return recorder.Snapshot()
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
		WebSearch:        result.WebSearch,
		ReasoningEffort:  result.ReasoningEffort,
		MaxOutputTokens:  result.MaxOutputTokens,
		MaxToolCalls:     result.MaxToolCalls,
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
	fmt.Fprintf(&b, "\n## Web Search\n\nEnabled: `%t`\n\nCalls: %d\n\nSources: %d\n\nCitations: %d\n", result.WebSearch.Enabled, result.WebSearch.CallCount, result.WebSearch.SourceCount, result.WebSearch.CitationCount)
	fmt.Fprintf(&b, "\n## Model Request\n\nReasoning effort: `%s`\n\nMaximum output tokens: %d\n\nMaximum tool calls: %s\n", displayReasoningEffort(result.ReasoningEffort), result.MaxOutputTokens, displayMaxToolCalls(result.MaxToolCalls))
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
	fmt.Fprintf(&b, "\n\n## Web Search\n\nEnabled: `%t`\n\nCalls: %d\n\nSources: %d\n\nCitations: %d\n", result.WebSearch.Enabled, result.WebSearch.CallCount, result.WebSearch.SourceCount, result.WebSearch.CitationCount)
	fmt.Fprintf(&b, "\n\n## Model Request\n\nReasoning effort: `%s`\n\nMaximum output tokens: %d\n\nMaximum tool calls: %s\n", displayReasoningEffort(result.ReasoningEffort), result.MaxOutputTokens, displayMaxToolCalls(result.MaxToolCalls))
	return b.String()
}

func displayReasoningEffort(value string) string {
	if value == "" {
		return "provider default"
	}
	return value
}

func displayMaxToolCalls(value int64) string {
	if value == 0 {
		return "provider default"
	}
	return fmt.Sprintf("%d", value)
}
