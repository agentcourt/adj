package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/agentcourt/adj/arbd/runtime/proceeding"
	"github.com/agentcourt/adj/common/promptfile"
)

type caseRunSummary struct {
	Status    string         `json:"status"`
	Answers   map[string]int `json:"answers,omitempty"`
	RunID     string         `json:"run_id,omitempty"`
	OutputDir string         `json:"out_dir,omitempty"`
	Error     string         `json:"error,omitempty"`
	Failure   map[string]any `json:"failure,omitempty"`
}

func runCase(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := newCommandFlagSet("case", stderr)
	var caseFiles explicitFileList
	var promptFiles promptfile.Assignments
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	fs.Var(&caseFiles, "file", "Explicit case file path or glob. May be repeated. Overrides automatic complaint-directory scanning")
	outDir := fs.String("out-dir", "", "Output directory")
	policyPath := fs.String("policy", "", "Policy JSON file. Default: ./etc/policy.json when present")
	councilSize := fs.Int("council-size", 0, "Override policy council_size")
	judgmentStandard := fs.String("judgment-standard", "", "Override policy judgment_standard")
	promptDir := fs.String("prompt-dir", "", "Complete prompt directory. Every non-overridden catalog file is required")
	fs.Var(&promptFiles, "prompt-file", "Prompt file override as ID=PATH. May be repeated")
	commonRoot := fs.String("common-root", proceeding.DefaultCommonRoot(), "Path to the sibling shared common directory")
	councilPool := fs.String("council-pool", "", "Council JSONL request-spec pool file. Default: ./pool.jsonl when present, else <common-root>/data/personas/pool.jsonl")
	caseAPIAddr := fs.String("caseapi-addr", proceeding.DefaultCaseAPIAddr, "Private case API listen address")
	councilBackend := fs.String("council-backend", proceeding.DefaultCouncilBackend, "Council backend: direct or councilapi")
	timeoutSeconds := fs.Int("timeout-seconds", 0, "Override runtime council LLM timeout in seconds")
	lawyerTimeoutSeconds := fs.Int("lawyer-timeout-seconds", 0, "Override runtime lawyer turn timeout in seconds")
	engineCallTimeoutSeconds := fs.Int("engine-timeout-seconds", 0, "Override Lean engine call timeout in seconds")
	maxResponseBytes := fs.Int("max-response-bytes", 0, "Override runtime max parsed response bytes")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", 0, "Override runtime invalid-attempt limit")
	enginePath := fs.String("engine", proceeding.DefaultEnginePath(), "Lean engine binary")
	runID := fs.String("run-id", "", "Run ID override")
	caseID := fs.String("case-id", proceeding.DefaultCaseID, "Case ID")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: aard case --complaint FILE --out-dir DIR\n\n")
		fs.PrintDefaults()
	}
	help, parseErr := parseCommandFlags(fs, args)
	if parseErr != nil {
		return reportCaseError(stdout, parseErr)
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return reportCaseError(stdout, fmt.Errorf("aard case accepts no positional arguments"))
	}
	if strings.TrimSpace(*complaintPath) == "" || strings.TrimSpace(*outDir) == "" {
		return reportCaseError(stdout, fmt.Errorf("--complaint and --out-dir are required"))
	}
	opts := proceeding.Options{
		ComplaintPath:            *complaintPath,
		CaseFiles:                caseFiles.values,
		OutputDir:                *outDir,
		PolicyPath:               *policyPath,
		CouncilSize:              *councilSize,
		JudgmentStandard:         *judgmentStandard,
		PromptDir:                *promptDir,
		PromptFiles:              promptFiles.Values(),
		CommonRoot:               strings.TrimSpace(*commonRoot),
		CouncilPoolPath:          *councilPool,
		CaseAPIAddr:              *caseAPIAddr,
		CouncilBackend:           *councilBackend,
		CouncilTimeoutSeconds:    *timeoutSeconds,
		LawyerTimeoutSeconds:     *lawyerTimeoutSeconds,
		EngineCallTimeoutSeconds: *engineCallTimeoutSeconds,
		MaxResponseBytes:         *maxResponseBytes,
		InvalidAttemptLimit:      *invalidAttemptLimit,
		EnginePath:               *enginePath,
		RunID:                    *runID,
		CaseID:                   *caseID,
	}
	result, err := proceeding.Run(ctx, opts)
	if err != nil {
		return reportCaseError(stdout, err)
	}
	return writeCaseSummary(stdout, buildCaseSuccessSummary(result, opts.OutputDir))
}

type explicitFileList struct {
	values []string
}

func (f *explicitFileList) String() string {
	return strings.Join(f.values, ",")
}

func (f *explicitFileList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("--file must not be empty")
	}
	f.values = append(f.values, value)
	return nil
}

func buildCaseSuccessSummary(result proceeding.Result, outDir string) caseRunSummary {
	return caseRunSummary{
		Status:    strings.TrimSpace(result.Status),
		Answers:   result.Answers,
		RunID:     strings.TrimSpace(result.RunID),
		OutputDir: strings.TrimSpace(outDir),
		Error:     strings.TrimSpace(result.Error),
		Failure:   result.Failure,
	}
}

func buildCaseErrorSummary(err error) caseRunSummary {
	return caseRunSummary{
		Status: "error",
		Error:  strings.TrimSpace(err.Error()),
	}
}

func reportCaseError(stdout io.Writer, err error) error {
	if writeErr := writeCaseSummary(stdout, buildCaseErrorSummary(err)); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	return &reportedError{err: err}
}

func writeCaseSummary(w io.Writer, summary caseRunSummary) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal case summary: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(raw)); err != nil {
		return fmt.Errorf("write case summary: %w", err)
	}
	return nil
}

func mapStringAny(value any) map[string]any {
	out, _ := value.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func mapListAny(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, raw := range v {
			entry, _ := raw.(map[string]any)
			if entry != nil {
				out = append(out, entry)
			}
		}
		return out
	default:
		return nil
	}
}

func caseSummaryIntValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
