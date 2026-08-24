package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/common/modelrequest"
	"github.com/agentcourt/adj/common/openai"
)

const JudgeRule12Tool = "decide_rule12_motion"

type JudgeRule12Fixture struct {
	ID                                string            `json:"id"`
	Tier                              int               `json:"tier"`
	IssueFamily                       string            `json:"issue_family"`
	CaseTheme                         string            `json:"case_theme"`
	Ground                            string            `json:"ground"`
	ComplaintText                     string            `json:"complaint_text"`
	MotionText                        string            `json:"motion_text"`
	OppositionText                    string            `json:"opposition_text"`
	ReplyText                         string            `json:"reply_text,omitempty"`
	JurisdictionalAllegations         map[string]string `json:"jurisdictional_allegations,omitempty"`
	ExpectedDisposition               string            `json:"expected_disposition"`
	ExpectedWithPrejudice             bool              `json:"expected_with_prejudice"`
	ExpectedLeaveToAmend              bool              `json:"expected_leave_to_amend"`
	ExpectedMissingElements           []string          `json:"expected_missing_elements,omitempty"`
	ExpectedJurisdictionBasisRejected string            `json:"expected_jurisdiction_basis_rejected,omitempty"`
	ExpectedInjuryMissing             bool              `json:"expected_injury_missing"`
	ExpectedTraceabilityMissing       bool              `json:"expected_traceability_missing"`
	ExpectedRedressabilityMissing     bool              `json:"expected_redressability_missing"`
	ExpectedReasonTags                []string          `json:"expected_reason_tags"`
	Severity                          float64           `json:"severity"`
	ContextNotes                      string            `json:"context_notes,omitempty"`
}

type JudgeRule12Options struct {
	FixturesPath          string
	OutputDir             string
	OpportunityPromptPath string
	OpportunityPromptName string
	Engine                lean.Engine
	Model                 string
	Online                bool
	DryRun                bool
	Limit                 int
	Timeout               time.Duration
	Temperature           *float64
	PromptDir             string
	PromptFiles           map[string]string
	Court                 courts.Profile
}

type JudgeRule12RescoreOptions struct {
	ResultsPath       string
	SourceSummaryPath string
	OutputDir         string
}

type JudgeRule12Summary struct {
	RunID              string                      `json:"run_id"`
	Evaluation         string                      `json:"evaluation"`
	Model              string                      `json:"model"`
	DryRun             bool                        `json:"dry_run"`
	ExecutionMode      string                      `json:"execution_mode"`
	CounterfactualModel bool                       `json:"counterfactual_model"`
	Provenance         JudgeEvalProvenance         `json:"provenance"`
	PromptSource       string                      `json:"prompt_source"`
	PromptName         string                      `json:"prompt_name"`
	PromptPath         string                      `json:"prompt_path,omitempty"`
	PromptCopyPath     string                      `json:"prompt_copy_path,omitempty"`
	FixturesPath       string                      `json:"fixtures_path"`
	OutputDir          string                      `json:"output_dir"`
	ResultsPath        string                      `json:"results_path"`
	SummaryPath        string                      `json:"summary_path"`
	Total              int                         `json:"total"`
	Correct            int                         `json:"correct"`
	ReasonCorrect      int                         `json:"reason_correct"`
	Invalid            int                         `json:"invalid"`
	FalseDismissals    int                         `json:"false_dismissals"`
	FalseDenials       int                         `json:"false_denials"`
	PostureMismatches  int                         `json:"posture_mismatches"`
	Accuracy           float64                     `json:"accuracy"`
	WeightedAccuracy   float64                     `json:"weighted_accuracy"`
	FalseDismissalRate float64                     `json:"false_dismissal_rate"`
	FalseDenialRate    float64                     `json:"false_denial_rate"`
	InvalidRate        float64                     `json:"invalid_rate"`
	ByReasonTag        map[string]JudgeRule12Slice `json:"by_reason_tag"`
	ByIssueFamily      map[string]JudgeRule12Slice `json:"by_issue_family"`
	ByGround           map[string]JudgeRule12Slice `json:"by_ground"`
	ByTier             map[string]JudgeRule12Slice `json:"by_tier"`
	GeneratedAt        string                      `json:"generated_at"`
}

type JudgeRule12Slice struct {
	Total             int     `json:"total"`
	Correct           int     `json:"correct"`
	FalseDismissals   int     `json:"false_dismissals"`
	FalseDenials      int     `json:"false_denials"`
	PostureMismatches int     `json:"posture_mismatches"`
	Invalid           int     `json:"invalid"`
	Weight            float64 `json:"weight"`
	CorrectWeight     float64 `json:"correct_weight"`
	Accuracy          float64 `json:"accuracy"`
	WeightedAccuracy  float64 `json:"weighted_accuracy"`
}

type JudgeRule12Result struct {
	RunID                             string            `json:"run_id"`
	Evaluation                        string            `json:"evaluation"`
	ID                                string            `json:"id"`
	Tier                              int               `json:"tier"`
	IssueFamily                       string            `json:"issue_family"`
	CaseTheme                         string            `json:"case_theme"`
	Ground                            string            `json:"ground"`
	ComplaintText                     string            `json:"complaint_text"`
	MotionText                        string            `json:"motion_text"`
	OppositionText                    string            `json:"opposition_text"`
	ReplyText                         string            `json:"reply_text,omitempty"`
	JurisdictionalAllegations         map[string]string `json:"jurisdictional_allegations,omitempty"`
	ExpectedDisposition               string            `json:"expected_disposition"`
	ExpectedWithPrejudice             bool              `json:"expected_with_prejudice"`
	ExpectedLeaveToAmend              bool              `json:"expected_leave_to_amend"`
	ExpectedMissingElements           []string          `json:"expected_missing_elements,omitempty"`
	ExpectedJurisdictionBasisRejected string            `json:"expected_jurisdiction_basis_rejected,omitempty"`
	ExpectedInjuryMissing             bool              `json:"expected_injury_missing"`
	ExpectedTraceabilityMissing       bool              `json:"expected_traceability_missing"`
	ExpectedRedressabilityMissing     bool              `json:"expected_redressability_missing"`
	ExpectedReasonTags                []string          `json:"expected_reason_tags"`
	Severity                          float64           `json:"severity"`
	ContextNotes                      string            `json:"context_notes,omitempty"`
	Model                             string            `json:"model"`
	DryRun                            bool              `json:"dry_run"`
	ExecutionMode                    string            `json:"execution_mode"`
	CounterfactualModel              bool              `json:"counterfactual_model"`
	PromptSource                      string            `json:"prompt_source"`
	PromptName                        string            `json:"prompt_name"`
	PromptPath                        string            `json:"prompt_path,omitempty"`
	State                             map[string]any    `json:"state"`
	View                              map[string]any    `json:"view"`
	Opportunity                       map[string]any    `json:"opportunity"`
	Input                             []map[string]any  `json:"input"`
	RawResponse                       map[string]any    `json:"raw_response"`
	ResponseExchanges                 []map[string]any  `json:"response_exchanges,omitempty"`
	TurnLog                           runner.TurnLog    `json:"turn_log"`
	FinalState                        map[string]any    `json:"final_state,omitempty"`
	Provider                          openai.Accounting `json:"provider"`
	ToolPayload                       map[string]any    `json:"tool_payload,omitempty"`
	Disposition                       string            `json:"disposition,omitempty"`
	WithPrejudice                     bool              `json:"with_prejudice,omitempty"`
	LeaveToAmend                      bool              `json:"leave_to_amend,omitempty"`
	AmendmentDeadlineDays             int               `json:"amendment_deadline_days,omitempty"`
	JurisdictionBasisRejected         string            `json:"jurisdiction_basis_rejected,omitempty"`
	InjuryMissing                     bool              `json:"injury_missing,omitempty"`
	TraceabilityMissing               bool              `json:"traceability_missing,omitempty"`
	RedressabilityMissing             bool              `json:"redressability_missing,omitempty"`
	MissingElements                   []string          `json:"missing_elements,omitempty"`
	Reasoning                         string            `json:"reasoning,omitempty"`
	MatchedReasonTags                 []string          `json:"matched_reason_tags"`
	OutcomeCorrect                    bool              `json:"outcome_correct"`
	ReasonCorrect                     bool              `json:"reason_correct"`
	InvalidReason                     string            `json:"invalid_reason,omitempty"`
	LeanAccepted                      bool              `json:"lean_accepted"`
	StepAccepted                      bool              `json:"step_accepted"`
	LeanError                         string            `json:"lean_error,omitempty"`
}

type judgeRule12PromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeRule12(ctx context.Context, opts JudgeRule12Options) (resultValue JudgeRule12Summary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule12Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule12Summary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	fixtures, err := LoadJudgeRule12Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule12Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule12Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule12Summary{}, err
		}
	}
	runID, err := newJudgeEvalRunID()
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule12Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule12PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule12Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule12Summary{
		RunID:          runID,
		Evaluation:     "judge_rule12",
		Model:          opts.Model,
		DryRun:         opts.DryRun,
		ExecutionMode:  "production",
		CounterfactualModel: false,
		Provenance:     newJudgeEvalProvenance(opts.Court, opts.PromptDir, opts.PromptFiles, promptVariant.Source, promptVariant.Name, promptVariant.Path, opts.Temperature, opts.Online, opts.DryRun, opts.Engine, "production"),
		PromptSource:   promptVariant.Source,
		PromptName:     promptVariant.Name,
		PromptPath:     promptVariant.Path,
		PromptCopyPath: promptVariant.CopyPath,
		FixturesPath:   opts.FixturesPath,
		OutputDir:      opts.OutputDir,
		ResultsPath:    resultsPath,
		SummaryPath:    summaryPath,
		ByReasonTag:    map[string]JudgeRule12Slice{},
		ByIssueFamily:  map[string]JudgeRule12Slice{},
		ByGround:       map[string]JudgeRule12Slice{},
		ByTier:         map[string]JudgeRule12Slice{},
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule12Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule12Summary{}, err
		}
		result.RunID = runID
		result.Evaluation = summary.Evaluation
		result.ExecutionMode = summary.ExecutionMode
		result.CounterfactualModel = summary.CounterfactualModel
		if err := encoder.Encode(result); err != nil {
			return JudgeRule12Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyRule12SummaryResult(&summary, result, weight)
	}
	finalizeRule12Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule12Summary{}, err
	}
	return summary, nil
}

func RescoreJudgeRule12(opts JudgeRule12RescoreOptions) (resultValue JudgeRule12Summary, returnErr error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeRule12Summary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule12Summary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeRule12Results(opts.ResultsPath)
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	source, sourceSummaryPath, err := loadJudgeEvalSourceSummary(opts.ResultsPath, opts.SourceSummaryPath, "judge_rule12")
	if err != nil {
		return JudgeRule12Summary{}, err
	}
	if err := validateJudgeEvalRescoreRows(opts.ResultsPath, source, results, judgeRule12ResultIdentity, validateJudgeRule12RescoreResult); err != nil {
		return JudgeRule12Summary{}, err
	}
	if err := validateJudgeEvalRescoreOutputPaths(opts.ResultsPath, sourceSummaryPath, opts.OutputDir); err != nil {
		return JudgeRule12Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule12Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule12Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule12Summary{
		RunID:         source.RunID,
		Evaluation:    "judge_rule12",
		Model:         source.Model,
		DryRun:        source.DryRun,
		ExecutionMode: source.ExecutionMode,
		CounterfactualModel: source.CounterfactualModel,
		Provenance:    newJudgeEvalRescoreProvenance(source, opts.ResultsPath, sourceSummaryPath),
		PromptSource:  source.PromptSource,
		PromptName:    source.PromptName,
		PromptPath:    source.PromptPath,
		FixturesPath:  "rescored from " + opts.ResultsPath,
		OutputDir:     opts.OutputDir,
		ResultsPath:   resultsPath,
		SummaryPath:   summaryPath,
		ByReasonTag:   map[string]JudgeRule12Slice{},
		ByIssueFamily: map[string]JudgeRule12Slice{},
		ByGround:      map[string]JudgeRule12Slice{},
		ByTier:        map[string]JudgeRule12Slice{},
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeRule12Result(&result)
		result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted
		if err := encoder.Encode(result); err != nil {
			return JudgeRule12Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyRule12SummaryResult(&summary, result, weight)
	}
	finalizeRule12Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule12Summary{}, err
	}
	return summary, nil
}

func runJudgeRule12Fixture(
	ctx context.Context,
	opts JudgeRule12Options,
	promptVariant judgeRule12PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule12Fixture,
) (JudgeRule12Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule12Result{}, err
	}
	state := BuildJudgeRule12State(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeRule12Roles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeRule12Tool)
	if err != nil {
		return JudgeRule12Result{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
	}
	executionModel := opts.Model
	if !opts.DryRun {
		executionModel = modelRef.Model
	}
	responseClient := judgeEvalResponseClient(opts.DryRun, client, dryRunJudgeRule12Response(fixture))
	callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	execution, executionErr := executeJudgeOpportunity(callCtx, judgeOpportunityExecutionOptions{
		Engine:          opts.Engine,
		State:           state,
		Roles:           roles,
		RolesPayload:    rolesPayload,
		Client:          responseClient,
		Court:           opts.Court,
		Model:           executionModel,
		Temperature:     opts.Temperature,
		PromptDir:       opts.PromptDir,
		PromptFiles:     opts.PromptFiles,
		MaxStepsPerTurn: 3,
		TurnIndex:       1,
		Objective: func(opportunity map[string]any) (string, error) {
			if strings.TrimSpace(promptVariant.Text) == "" {
				return stringField(opportunity, "objective"), nil
			}
			return renderJudgeRule12PromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeRule12Tool, executionErr)
	if decisionErr != nil {
		return JudgeRule12Result{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeRule12Response(fixture, opts.Model, opts.DryRun, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
	result.RawResponse = responseJSON(decision.RawResponse)
	result.ResponseExchanges = judgeEvalResponseExchangeJSON(execution.Exchanges)
	result.TurnLog = execution.TurnLog
	result.FinalState = execution.FinalState
	result.Provider = execution.Provider
	result.PromptSource = promptVariant.Source
	result.PromptName = promptVariant.Name
	result.PromptPath = promptVariant.Path
	if result.InvalidReason == "" {
		result.InvalidReason = judgeEvalProceduralInvalidReason(executionErr)
	}
	result.LeanAccepted, result.StepAccepted = judgeEvalExecutionAcceptance(execution, JudgeRule12Tool)
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted && executionErr == nil
	return result, nil
}

func LoadJudgeRule12Fixtures(path string) (resultValue []JudgeRule12Fixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule12Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule12Fixture
		if err := json.Unmarshal([]byte(line), &fixture); err != nil {
			return nil, fmt.Errorf("parse fixtures %s line %d: %w", path, lineNo, err)
		}
		if err := fixture.Validate(); err != nil {
			return nil, fmt.Errorf("fixtures %s line %d: %w", path, lineNo, err)
		}
		out = append(out, fixture)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan fixtures %s: %w", path, err)
	}
	return out, nil
}

func readJudgeRule12Results(path string) (resultValue []JudgeRule12Result, returnErr error) {
	return readJudgeEvalJSONL[JudgeRule12Result](path, judgeEvalRequiredResultFields(
		"tier", "issue_family", "case_theme", "ground", "complaint_text", "motion_text", "opposition_text",
		"expected_disposition", "expected_with_prejudice", "expected_leave_to_amend", "expected_injury_missing",
		"expected_traceability_missing", "expected_redressability_missing", "expected_reason_tags", "severity", "matched_reason_tags",
	)...)
}

func judgeRule12ResultIdentity(result JudgeRule12Result) judgeEvalResultIdentity {
	return judgeEvalResultIdentity{
		RunID:         result.RunID,
		Evaluation:    result.Evaluation,
		ID:            result.ID,
		Model:         result.Model,
		DryRun:        result.DryRun,
		PromptSource:  resultRule12PromptSource(result),
		PromptName:    resultRule12PromptName(result),
		PromptPath:    result.PromptPath,
		ExecutionMode: result.ExecutionMode,
		CounterfactualModel: result.CounterfactualModel,
	}
}

func validateJudgeRule12RescoreResult(result JudgeRule12Result) error {
	return (JudgeRule12Fixture{
		ID:                                result.ID,
		Tier:                              result.Tier,
		IssueFamily:                       result.IssueFamily,
		CaseTheme:                         result.CaseTheme,
		Ground:                            result.Ground,
		ComplaintText:                     result.ComplaintText,
		MotionText:                        result.MotionText,
		OppositionText:                    result.OppositionText,
		ReplyText:                         result.ReplyText,
		JurisdictionalAllegations:         result.JurisdictionalAllegations,
		ExpectedDisposition:               result.ExpectedDisposition,
		ExpectedWithPrejudice:             result.ExpectedWithPrejudice,
		ExpectedLeaveToAmend:              result.ExpectedLeaveToAmend,
		ExpectedMissingElements:           result.ExpectedMissingElements,
		ExpectedJurisdictionBasisRejected: result.ExpectedJurisdictionBasisRejected,
		ExpectedInjuryMissing:             result.ExpectedInjuryMissing,
		ExpectedTraceabilityMissing:       result.ExpectedTraceabilityMissing,
		ExpectedRedressabilityMissing:     result.ExpectedRedressabilityMissing,
		ExpectedReasonTags:                result.ExpectedReasonTags,
		Severity:                          result.Severity,
		ContextNotes:                      result.ContextNotes,
	}).Validate()
}

func loadJudgeRule12PromptVariant(path string, name string, outputDir string) (judgeRule12PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule12PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule12PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule12PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeRule12PromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeRule12PromptVariant{Source: "file:" + path, Name: name, Path: path, CopyPath: copyPath, Text: text}, nil
}

func (f JudgeRule12Fixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if !validRule12GroundName(f.Ground) {
		return fmt.Errorf("fixture %s invalid ground %q", f.ID, f.Ground)
	}
	if strings.TrimSpace(f.ComplaintText) == "" {
		return fmt.Errorf("fixture %s missing complaint_text", f.ID)
	}
	if strings.TrimSpace(f.MotionText) == "" {
		return fmt.Errorf("fixture %s missing motion_text", f.ID)
	}
	if strings.TrimSpace(f.OppositionText) == "" {
		return fmt.Errorf("fixture %s missing opposition_text", f.ID)
	}
	if !validRule12Disposition(f.ExpectedDisposition) {
		return fmt.Errorf("fixture %s invalid expected_disposition %q", f.ID, f.ExpectedDisposition)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule12State(f JudgeRule12Fixture) map[string]any {
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        map[string]any{"jurisdiction_screen": true, "require_jurisdiction_statement": true},
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-rule12-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "filed",
			"resolution":                    "pending",
			"trial_mode":                    "unset",
			"phase":                         "none",
			"last_pleading_served_on":       "",
			"jury_demanded_on":              "",
			"jury_configuration":            nil,
			"single_claim":                  defaultJudgeEvalClaim(),
			"jurisdictional_allegations":    judgeRule12JurisdictionalAllegations(f),
			"jurors":                        []any{},
			"juror_questionnaire":           []any{},
			"juror_questionnaire_responses": []any{},
			"voir_dire_exchanges":           []any{},
			"for_cause_challenges":          []any{},
			"deliberation_round":            1,
			"juror_votes":                   []any{},
			"jury_verdict":                  nil,
			"hung_jury":                     nil,
			"contempt_counts":               []any{},
			"protective_orders":             []any{},
			"bench_findings":                []any{},
			"bench_conclusions":             []any{},
			"juror_explanations":            []any{},
			"local_rule_overrides":          []any{},
			"limit_usage":                   []any{},
			"rule56_window_closed_for":      []any{},
			"case_files":                    []any{},
			"file_events":                   []any{},
			"rule68_offers":                 []any{},
			"technical_reports":             []any{},
			"monetary_judgment":             0.0,
			"docket":                        judgeRule12Docket(f),
			"decision_traces": []any{
				map[string]any{"action": "file_complaint", "outcome": "filed", "citations": []any{"FRCP 3", "FRCP 8(a)"}},
				map[string]any{"action": "file_rule12_motion", "outcome": "filed", "citations": []any{"FRCP 12(b)"}},
				map[string]any{"action": "oppose_rule12_motion", "outcome": "filed", "citations": []any{"FRCP 12"}},
			},
		},
	}
}

func judgeRule12Docket(f JudgeRule12Fixture) []any {
	entries := []any{
		map[string]any{"title": "Complaint filed", "description": "plaintiff: " + strings.TrimSpace(f.ComplaintText)},
		map[string]any{"title": "Rule 12 Motion", "description": "defendant: ground=" + normalizeRule12Ground(f.Ground) + " summary=" + strings.TrimSpace(f.MotionText)},
		map[string]any{"title": "Rule 12 Opposition", "description": "plaintiff: " + strings.TrimSpace(f.OppositionText)},
	}
	if strings.TrimSpace(f.ReplyText) != "" {
		entries = append(entries, map[string]any{"title": "Rule 12 Reply", "description": "defendant: " + strings.TrimSpace(f.ReplyText)})
	}
	return entries
}

func judgeRule12JurisdictionalAllegations(f JudgeRule12Fixture) map[string]any {
	defaults := map[string]string{
		"jurisdiction_basis":         "diversity",
		"jurisdictional_statement":   "Plaintiff and defendant are citizens of different states and the amount in controversy exceeds $75,000.",
		"plaintiff_citizenship":      "Illinois",
		"defendant_citizenship":      "Delaware",
		"amount_in_controversy":      "$120,000",
		"injury_statement":           "Plaintiff lost money from the disputed transaction.",
		"causation_statement":        "Defendant's conduct caused the alleged loss.",
		"redressability_statement":   "Money damages would redress the injury.",
		"ripeness_statement":         "The loss has already occurred.",
		"live_controversy_statement": "The parties continue to dispute liability and damages.",
	}
	for key, value := range f.JurisdictionalAllegations {
		defaults[key] = value
	}
	out := make(map[string]any, len(defaults))
	for key, value := range defaults {
		out[key] = value
	}
	return out
}

func buildJudgeRule12Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule12Fixture,
	promptVariant judgeRule12PromptVariant,
) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeRule12PromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeRule12Tool, view, opportunity, objective)
}

func scoreJudgeRule12Response(
	fixture JudgeRule12Fixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule12Result {
	result := JudgeRule12Result{
		ID:                                fixture.ID,
		Tier:                              fixture.Tier,
		IssueFamily:                       fixture.IssueFamily,
		CaseTheme:                         fixture.CaseTheme,
		Ground:                            normalizeRule12Ground(fixture.Ground),
		ComplaintText:                     strings.TrimSpace(fixture.ComplaintText),
		MotionText:                        strings.TrimSpace(fixture.MotionText),
		OppositionText:                    strings.TrimSpace(fixture.OppositionText),
		ReplyText:                         strings.TrimSpace(fixture.ReplyText),
		JurisdictionalAllegations:         copyStringMap(fixture.JurisdictionalAllegations),
		ExpectedDisposition:               normalizeRule12Disposition(fixture.ExpectedDisposition),
		ExpectedWithPrejudice:             fixture.ExpectedWithPrejudice,
		ExpectedLeaveToAmend:              fixture.ExpectedLeaveToAmend,
		ExpectedMissingElements:           append([]string{}, fixture.ExpectedMissingElements...),
		ExpectedJurisdictionBasisRejected: strings.TrimSpace(fixture.ExpectedJurisdictionBasisRejected),
		ExpectedInjuryMissing:             fixture.ExpectedInjuryMissing,
		ExpectedTraceabilityMissing:       fixture.ExpectedTraceabilityMissing,
		ExpectedRedressabilityMissing:     fixture.ExpectedRedressabilityMissing,
		ExpectedReasonTags:                append([]string{}, fixture.ExpectedReasonTags...),
		Severity:                          normalizedSeverity(fixture.Severity),
		ContextNotes:                      strings.TrimSpace(fixture.ContextNotes),
		Model:                             model,
		DryRun:                            dryRun,
		State:                             state,
		View:                              view,
		Opportunity:                       opportunity,
		Input:                             input,
		RawResponse:                       responseJSON(resp),
	}
	payload, invalid := extractJudgeRule12Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	if intField(payload, "motion_index") != 0 {
		result.InvalidReason = "wrong_motion_index"
		return result
	}
	ground := normalizeRule12Ground(stringField(payload, "ground"))
	if ground != result.Ground {
		result.InvalidReason = "wrong_ground"
		return result
	}
	disposition := normalizeRule12Disposition(stringField(payload, "disposition"))
	if !validRule12Disposition(disposition) {
		result.InvalidReason = "invalid_disposition"
		return result
	}
	result.Disposition = disposition
	result.WithPrejudice = boolField(payload, "with_prejudice")
	result.LeaveToAmend = boolField(payload, "leave_to_amend")
	result.AmendmentDeadlineDays = intField(payload, "amendment_deadline_days")
	result.JurisdictionBasisRejected = strings.TrimSpace(stringField(payload, "jurisdiction_basis_rejected"))
	result.InjuryMissing = boolField(payload, "injury_missing")
	result.TraceabilityMissing = boolField(payload, "traceability_missing")
	result.RedressabilityMissing = boolField(payload, "redressability_missing")
	result.MissingElements = stringsFromAnySlice(payload["missing_elements"])
	result.Reasoning = strings.TrimSpace(stringField(payload, "reasoning"))
	if result.Reasoning == "" {
		result.InvalidReason = "empty_reasoning"
		return result
	}
	result.OutcomeCorrect = rule12OutcomeCorrect(result)
	result.MatchedReasonTags = matchedRule12ReasonTags(result.Reasoning, fixture.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
	return result
}

func rescoreJudgeRule12Result(result *JudgeRule12Result) {
	if result == nil || result.InvalidReason != "" {
		return
	}
	result.OutcomeCorrect = rule12OutcomeCorrect(*result)
	result.MatchedReasonTags = matchedRule12ReasonTags(result.Reasoning, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeRule12Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule12Tool {
		return nil, "wrong_tool"
	}
	if strings.TrimSpace(call.ArgumentsError) != "" {
		return nil, "malformed_arguments"
	}
	if call.Arguments == nil {
		return nil, "missing_arguments"
	}
	return call.Arguments, ""
}

func dryRunJudgeRule12Response(f JudgeRule12Fixture) openai.Response {
	args := map[string]any{
		"motion_index": 0,
		"ground":       normalizeRule12Ground(f.Ground),
		"disposition":  normalizeRule12Disposition(f.ExpectedDisposition),
		"reasoning":    "gold tags: " + strings.Join(f.ExpectedReasonTags, ", "),
	}
	if f.ExpectedWithPrejudice {
		args["with_prejudice"] = true
	}
	if f.ExpectedLeaveToAmend {
		args["leave_to_amend"] = true
		args["amendment_deadline_days"] = 21
	}
	if len(f.ExpectedMissingElements) > 0 {
		args["missing_elements"] = append([]string{}, f.ExpectedMissingElements...)
	}
	if strings.TrimSpace(f.ExpectedJurisdictionBasisRejected) != "" {
		args["jurisdiction_basis_rejected"] = strings.TrimSpace(f.ExpectedJurisdictionBasisRejected)
	}
	if f.ExpectedInjuryMissing {
		args["injury_missing"] = true
	}
	if f.ExpectedTraceabilityMissing {
		args["traceability_missing"] = true
	}
	if f.ExpectedRedressabilityMissing {
		args["redressability_missing"] = true
	}
	return openai.Response{
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID:    "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:      JudgeRule12Tool,
			Arguments: args,
		}},
	}
}

func renderJudgeRule12PromptTemplate(template string, fixture JudgeRule12Fixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeRule12PromptTemplate", template,
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{ground}}", normalizeRule12Ground(fixture.Ground),
		"{{complaint_text}}", strings.TrimSpace(fixture.ComplaintText),
		"{{motion_text}}", strings.TrimSpace(fixture.MotionText),
		"{{opposition_text}}", strings.TrimSpace(fixture.OppositionText),
		"{{reply_text}}", strings.TrimSpace(fixture.ReplyText),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeRule12Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule12Tool}}}
}

func rule12OutcomeCorrect(result JudgeRule12Result) bool {
	if result.Disposition != result.ExpectedDisposition {
		return false
	}
	if result.WithPrejudice != result.ExpectedWithPrejudice || result.LeaveToAmend != result.ExpectedLeaveToAmend {
		return false
	}
	if result.ExpectedDisposition == "denied" {
		return true
	}
	switch result.Ground {
	case "failure_to_state_a_claim":
		return containsAllElementLabels(result.MissingElements, result.ExpectedMissingElements)
	case "lack_subject_matter_jurisdiction":
		return jurisdictionBasisMatches(result.JurisdictionBasisRejected, result.ExpectedJurisdictionBasisRejected)
	case "no_standing":
		if result.ExpectedInjuryMissing && !result.InjuryMissing {
			return false
		}
		if result.ExpectedTraceabilityMissing && !result.TraceabilityMissing {
			return false
		}
		if result.ExpectedRedressabilityMissing && !result.RedressabilityMissing {
			return false
		}
		return result.ExpectedInjuryMissing || result.ExpectedTraceabilityMissing || result.ExpectedRedressabilityMissing
	default:
		return true
	}
}

func applyRule12SummaryResult(summary *JudgeRule12Summary, result JudgeRule12Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else if result.OutcomeCorrect {
		summary.Correct++
	} else {
		classifyRule12Error(result, &summary.FalseDismissals, &summary.FalseDenials, &summary.PostureMismatches)
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateRule12Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateRule12Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateRule12Slice(summary.ByGround, result.Ground, result, weight)
	updateRule12Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
}

func updateRule12Slice(m map[string]JudgeRule12Slice, key string, result JudgeRule12Result, weight float64) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unspecified"
	}
	s := m[key]
	s.Total++
	s.Weight += weight
	if result.InvalidReason != "" {
		s.Invalid++
	} else if result.OutcomeCorrect {
		s.Correct++
		s.CorrectWeight += weight
	} else {
		classifyRule12Error(result, &s.FalseDismissals, &s.FalseDenials, &s.PostureMismatches)
	}
	m[key] = s
}

func classifyRule12Error(result JudgeRule12Result, falseDismissals *int, falseDenials *int, postureMismatches *int) {
	got := normalizeRule12Disposition(result.Disposition)
	want := normalizeRule12Disposition(result.ExpectedDisposition)
	if got == "granted" && want == "denied" {
		(*falseDismissals)++
		return
	}
	if got == "denied" && want == "granted" {
		(*falseDenials)++
		return
	}
	(*postureMismatches)++
}

func finalizeRule12Summary(summary *JudgeRule12Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.FalseDismissalRate = float64(summary.FalseDismissals) / float64(summary.Total)
		summary.FalseDenialRate = float64(summary.FalseDenials) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeRule12Slices(summary.ByReasonTag)
	finalizeRule12Slices(summary.ByIssueFamily)
	finalizeRule12Slices(summary.ByGround)
	finalizeRule12Slices(summary.ByTier)
}

func finalizeRule12Slices(m map[string]JudgeRule12Slice) {
	for key, s := range m {
		if s.Total > 0 {
			s.Accuracy = float64(s.Correct) / float64(s.Total)
		}
		if s.Weight > 0 {
			s.WeightedAccuracy = s.CorrectWeight / s.Weight
		}
		m[key] = s
	}
}

func matchedRule12ReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if rule12ReasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func rule12ReasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range rule12ReasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func rule12ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"accept_pled_facts":       {"accept", "accepted as true", "pleaded facts", "as pleaded"},
		"factual_dispute":         {"factual dispute", "disputed fact", "fact dispute", "rule 12"},
		"missing_element":         {"missing element", "fails to allege", "does not allege", "element"},
		"conclusory":              {"conclusory", "labels", "formulaic", "unsupported conclusion"},
		"standing_injury":         {"injury", "concrete", "particularized"},
		"standing_traceability":   {"traceability", "fairly traceable", "causation"},
		"standing_redressability": {"redressability", "redress", "relief would not"},
		"jurisdiction_defect":     {"subject matter", "jurisdiction", "diversity", "amount in controversy"},
		"amendable_defect":        {"leave to amend", "amend", "curable", "without prejudice"},
		"futile_defect":           {"with prejudice", "futile", "cannot be cured"},
		"live_controversy":        {"live controversy", "moot", "ongoing"},
		"ripeness":                {"ripe", "ripeness", "contingent", "premature"},
	}
}

func validRule12GroundName(s string) bool {
	switch normalizeRule12Ground(s) {
	case "lack_subject_matter_jurisdiction", "no_standing", "not_ripe", "moot", "failure_to_state_a_claim":
		return true
	default:
		return false
	}
}

func normalizeRule12Ground(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validRule12Disposition(s string) bool {
	switch normalizeRule12Disposition(s) {
	case "granted", "denied":
		return true
	default:
		return false
	}
}

func normalizeRule12Disposition(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func boolField(m map[string]any, key string) bool {
	value, _ := m[key].(bool)
	return value
}

func containsAllElementLabels(got []string, want []string) bool {
	gotNormalized := make([]string, 0, len(got))
	for _, item := range got {
		item = normalizeReasonText(item)
		if item != "" {
			gotNormalized = append(gotNormalized, item)
		}
	}
	for _, item := range want {
		wantItem := normalizeReasonText(item)
		if wantItem == "" {
			continue
		}
		var matched bool
		for _, gotItem := range gotNormalized {
			if gotItem == wantItem || strings.Contains(gotItem, wantItem) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func jurisdictionBasisMatches(got string, want string) bool {
	got = normalizeReasonText(got)
	want = normalizeReasonText(want)
	if want == "" {
		return false
	}
	if got == want || strings.Contains(got, want) {
		return true
	}
	if want == "unspecified" {
		hasNoFederalQuestion := strings.Contains(got, "no federal question") || strings.Contains(got, "no 1331") || strings.Contains(got, "1331 federal question")
		hasNoDiversity := strings.Contains(got, "no diversity") || strings.Contains(got, "no 1332") || strings.Contains(got, "1332 diversity") || strings.Contains(got, "diversity allegations")
		return hasNoFederalQuestion && hasNoDiversity
	}
	return false
}

func resultRule12PromptSource(result JudgeRule12Result) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultRule12PromptName(result JudgeRule12Result) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
