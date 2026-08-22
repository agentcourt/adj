package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jsmorph/adj/arb/runtime/proceeding"
	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/common/promptfile"
)

type caseRunSummary struct {
	Status       string               `json:"status"`
	Result       string               `json:"result,omitempty"`
	VotesFor     *int                 `json:"votes_for,omitempty"`
	VotesAgainst *int                 `json:"votes_against,omitempty"`
	RunID        string               `json:"run_id,omitempty"`
	OutputDir    string               `json:"out_dir,omitempty"`
	Error        string               `json:"error,omitempty"`
	ErrorClass   string               `json:"error_class,omitempty"`
	Failure      map[string]any       `json:"failure,omitempty"`
	Provider     openaiapi.Accounting `json:"provider"`
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
	requiredVotes := fs.Int("required-votes", 0, "Override policy required_votes_for_decision")
	evidenceStandard := fs.String("evidence-standard", "", "Override policy evidence_standard")
	promptDir := fs.String("prompt-dir", "", "Complete prompt directory. Every non-overridden catalog file is required")
	fs.Var(&promptFiles, "prompt-file", "Prompt file override as ID=PATH. May be repeated")
	commonRoot := fs.String("common-root", proceeding.DefaultCommonRoot(), "Path to the sibling shared common directory")
	councilPool := fs.String("council-pool", "", "Council JSONL request-spec pool file. Default: ./pool.jsonl when present, else <common-root>/data/personas/pool.jsonl")
	caseAPIAddr := fs.String("caseapi-addr", proceeding.DefaultCaseAPIAddr, "Private case API listen address")
	councilBackend := fs.String("council-backend", proceeding.DefaultCouncilBackend, "Council backend: direct or councilapi")
	timeoutSeconds := fs.Int("timeout-seconds", 0, "Override runtime council LLM timeout in seconds")
	councilRequestAttempts := fs.Int("council-request-attempts", 0, "Override council provider attempts per request: 1 through 4")
	lawyerTimeoutSeconds := fs.Int("lawyer-timeout-seconds", 0, "Override runtime lawyer turn timeout in seconds")
	engineCallTimeoutSeconds := fs.Int("engine-timeout-seconds", 0, "Override Lean engine call timeout in seconds")
	maxResponseBytes := fs.Int("max-response-bytes", 0, "Override runtime max parsed response bytes")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", 0, "Override runtime invalid-attempt limit")
	enginePath := fs.String("engine", proceeding.DefaultEnginePath(), "Lean engine binary")
	runID := fs.String("run-id", "", "Run ID override")
	caseID := fs.String("case-id", proceeding.DefaultCaseID, "Case ID")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: aar case --complaint FILE --out-dir DIR\n\n")
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
		return reportCaseError(stdout, fmt.Errorf("aar case accepts no positional arguments"))
	}
	if strings.TrimSpace(*complaintPath) == "" || strings.TrimSpace(*outDir) == "" {
		return reportCaseError(stdout, fmt.Errorf("--complaint and --out-dir are required"))
	}
	commonRootValue := strings.TrimSpace(*commonRoot)
	opts := proceeding.Options{
		ComplaintPath:            *complaintPath,
		CaseFiles:                caseFiles.values,
		OutputDir:                *outDir,
		PolicyPath:               *policyPath,
		CouncilSize:              *councilSize,
		RequiredVotes:            *requiredVotes,
		EvidenceStandard:         *evidenceStandard,
		PromptDir:                *promptDir,
		PromptFiles:              promptFiles.Values(),
		CommonRoot:               commonRootValue,
		CouncilPoolPath:          *councilPool,
		CaseAPIAddr:              *caseAPIAddr,
		CouncilBackend:           *councilBackend,
		CouncilTimeoutSeconds:    *timeoutSeconds,
		CouncilRequestAttempts:   *councilRequestAttempts,
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
	votesFor, votesAgainst := finalVoteCounts(result.FinalState)
	return caseRunSummary{
		Status:       strings.TrimSpace(result.Status),
		Result:       strings.TrimSpace(result.Resolution),
		VotesFor:     &votesFor,
		VotesAgainst: &votesAgainst,
		RunID:        strings.TrimSpace(result.RunID),
		OutputDir:    strings.TrimSpace(outDir),
		Error:        strings.TrimSpace(result.Error),
		ErrorClass:   strings.TrimSpace(result.ErrorClass),
		Failure:      result.Failure,
		Provider:     result.Provider,
	}
}

func buildCaseErrorSummary(err error) caseRunSummary {
	return caseRunSummary{
		Status:     "error",
		Error:      strings.TrimSpace(err.Error()),
		ErrorClass: string(openaiapi.ErrorClass(err)),
		Provider:   proceeding.CouncilAccounting(err),
	}
}

func reportCaseError(stdout io.Writer, err error) error {
	if writeErr := writeCaseSummary(stdout, buildCaseErrorSummary(err)); writeErr != nil {
		return writeErr
	}
	return &reportedError{err: err}
}

func writeCaseSummary(w io.Writer, summary caseRunSummary) error {
	wire, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal case summary: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(wire)); err != nil {
		return fmt.Errorf("write case summary: %w", err)
	}
	return nil
}

func finalVoteCounts(state map[string]any) (int, int) {
	caseObj := mapStringAny(state["case"])
	if len(caseObj) == 0 {
		return 0, 0
	}
	targetRound := caseSummaryIntValue(caseObj["deliberation_round"])
	votes := mapListAny(caseObj["council_votes"])
	if targetRound <= 0 {
		for _, vote := range votes {
			if round := caseSummaryIntValue(vote["round"]); round > targetRound {
				targetRound = round
			}
		}
	}
	var votesFor int
	var votesAgainst int
	for _, vote := range votes {
		if targetRound > 0 && caseSummaryIntValue(vote["round"]) != targetRound {
			continue
		}
		switch strings.TrimSpace(fmt.Sprintf("%v", vote["vote"])) {
		case "demonstrated":
			votesFor++
		case "not_demonstrated":
			votesAgainst++
		}
	}
	return votesFor, votesAgainst
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
