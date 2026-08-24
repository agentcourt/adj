package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
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

const JudgeRule11Tool = "decide_rule11_motion"

type JudgeRule11Fixture struct {
	ID                     string   `json:"id"`
	Tier                   int      `json:"tier"`
	IssueFamily            string   `json:"issue_family"`
	CaseTheme              string   `json:"case_theme"`
	Movant                 string   `json:"movant"`
	TargetParty            string   `json:"target_party"`
	ChallengedFiling       string   `json:"challenged_filing"`
	FilingText             string   `json:"filing_text"`
	NoticeText             string   `json:"notice_text"`
	NoticeServedAt         string   `json:"notice_served_at"`
	MotionFiledAt          string   `json:"motion_filed_at"`
	CorrectionText         string   `json:"correction_text,omitempty"`
	MotionText             string   `json:"motion_text"`
	OppositionText         string   `json:"opposition_text"`
	ExpectedGranted        bool     `json:"expected_granted"`
	ExpectedSanctionType   string   `json:"expected_sanction_type"`
	ExpectedSanctionAmount float64  `json:"expected_sanction_amount,omitempty"`
	ExpectedReasonTags     []string `json:"expected_reason_tags"`
	Severity               float64  `json:"severity"`
	ContextNotes           string   `json:"context_notes,omitempty"`
}

type JudgeRule11Options struct {
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
	CounterfactualModel   bool
}

type JudgeRule11RescoreOptions struct {
	ResultsPath       string
	SourceSummaryPath string
	OutputDir         string
}

type JudgeRule11Summary struct {
	RunID               string                      `json:"run_id"`
	Evaluation          string                      `json:"evaluation"`
	Model               string                      `json:"model"`
	DryRun              bool                        `json:"dry_run"`
	Provenance          JudgeEvalProvenance         `json:"provenance"`
	ExecutionMode       string                      `json:"execution_mode"`
	CounterfactualModel bool                        `json:"counterfactual_model"`
	PromptSource        string                      `json:"prompt_source"`
	PromptName          string                      `json:"prompt_name"`
	PromptPath          string                      `json:"prompt_path,omitempty"`
	PromptCopyPath      string                      `json:"prompt_copy_path,omitempty"`
	FixturesPath        string                      `json:"fixtures_path"`
	OutputDir           string                      `json:"output_dir"`
	ResultsPath         string                      `json:"results_path"`
	SummaryPath         string                      `json:"summary_path"`
	Total               int                         `json:"total"`
	Correct             int                         `json:"correct"`
	GrantCorrect        int                         `json:"grant_correct"`
	ReasonCorrect       int                         `json:"reason_correct"`
	Invalid             int                         `json:"invalid"`
	FalseGrants         int                         `json:"false_grants"`
	FalseDenials        int                         `json:"false_denials"`
	SanctionMismatches  int                         `json:"sanction_mismatches"`
	Accuracy            float64                     `json:"accuracy"`
	GrantAccuracy       float64                     `json:"grant_accuracy"`
	WeightedAccuracy    float64                     `json:"weighted_accuracy"`
	FalseGrantRate      float64                     `json:"false_grant_rate"`
	FalseDenialRate     float64                     `json:"false_denial_rate"`
	InvalidRate         float64                     `json:"invalid_rate"`
	ByReasonTag         map[string]JudgeRule11Slice `json:"by_reason_tag"`
	ByIssueFamily       map[string]JudgeRule11Slice `json:"by_issue_family"`
	ByTier              map[string]JudgeRule11Slice `json:"by_tier"`
	ByMovant            map[string]JudgeRule11Slice `json:"by_movant"`
	ByExpectedSanction  map[string]JudgeRule11Slice `json:"by_expected_sanction"`
	GeneratedAt         string                      `json:"generated_at"`
}

type JudgeRule11Slice struct {
	Total              int     `json:"total"`
	Correct            int     `json:"correct"`
	GrantCorrect       int     `json:"grant_correct"`
	FalseGrants        int     `json:"false_grants"`
	FalseDenials       int     `json:"false_denials"`
	SanctionMismatches int     `json:"sanction_mismatches"`
	Invalid            int     `json:"invalid"`
	Weight             float64 `json:"weight"`
	CorrectWeight      float64 `json:"correct_weight"`
	Accuracy           float64 `json:"accuracy"`
	GrantAccuracy      float64 `json:"grant_accuracy"`
	WeightedAccuracy   float64 `json:"weighted_accuracy"`
}

type JudgeRule11Result struct {
	RunID                  string            `json:"run_id"`
	Evaluation             string            `json:"evaluation"`
	ID                     string            `json:"id"`
	Tier                   int               `json:"tier"`
	IssueFamily            string            `json:"issue_family"`
	CaseTheme              string            `json:"case_theme"`
	Movant                 string            `json:"movant"`
	TargetParty            string            `json:"target_party"`
	ChallengedFiling       string            `json:"challenged_filing"`
	FilingText             string            `json:"filing_text"`
	NoticeText             string            `json:"notice_text"`
	NoticeServedAt         string            `json:"notice_served_at"`
	MotionFiledAt          string            `json:"motion_filed_at"`
	CorrectionText         string            `json:"correction_text,omitempty"`
	MotionText             string            `json:"motion_text"`
	OppositionText         string            `json:"opposition_text"`
	ExpectedGranted        bool              `json:"expected_granted"`
	ExpectedSanctionType   string            `json:"expected_sanction_type"`
	ExpectedSanctionAmount float64           `json:"expected_sanction_amount,omitempty"`
	ExpectedReasonTags     []string          `json:"expected_reason_tags"`
	Severity               float64           `json:"severity"`
	ContextNotes           string            `json:"context_notes,omitempty"`
	Model                  string            `json:"model"`
	DryRun                 bool              `json:"dry_run"`
	ExecutionMode          string            `json:"execution_mode"`
	CounterfactualModel    bool              `json:"counterfactual_model"`
	PromptSource           string            `json:"prompt_source"`
	PromptName             string            `json:"prompt_name"`
	PromptPath             string            `json:"prompt_path,omitempty"`
	State                  map[string]any    `json:"state"`
	View                   map[string]any    `json:"view"`
	Opportunity            map[string]any    `json:"opportunity"`
	Input                  []map[string]any  `json:"input"`
	RawResponse            map[string]any    `json:"raw_response"`
	ResponseExchanges      []map[string]any  `json:"response_exchanges,omitempty"`
	TurnLog                runner.TurnLog    `json:"turn_log"`
	FinalState             map[string]any    `json:"final_state,omitempty"`
	Provider               openai.Accounting `json:"provider"`
	ToolPayload            map[string]any    `json:"tool_payload,omitempty"`
	MotionIndex            int               `json:"motion_index,omitempty"`
	Granted                *bool             `json:"granted,omitempty"`
	SanctionType           string            `json:"sanction_type,omitempty"`
	SanctionAmount         *float64          `json:"sanction_amount,omitempty"`
	SanctionDetail         string            `json:"sanction_detail"`
	Reasoning              string            `json:"reasoning,omitempty"`
	MatchedReasonTags      []string          `json:"matched_reason_tags"`
	GrantCorrect           bool              `json:"grant_correct"`
	SanctionCorrect        bool              `json:"sanction_correct"`
	OutcomeCorrect         bool              `json:"outcome_correct"`
	ReasonCorrect          bool              `json:"reason_correct"`
	InvalidReason          string            `json:"invalid_reason,omitempty"`
	LeanAccepted           bool              `json:"lean_accepted"`
	StepAccepted           bool              `json:"step_accepted"`
	LeanError              string            `json:"lean_error,omitempty"`
}

type judgeRule11PromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeRule11(ctx context.Context, opts JudgeRule11Options) (resultValue JudgeRule11Summary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule11Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule11Summary{}, fmt.Errorf("output directory is required")
	}
	if strings.TrimSpace(opts.OpportunityPromptPath) != "" && !opts.CounterfactualModel {
		return JudgeRule11Summary{}, fmt.Errorf("opportunity prompt file requires counterfactual model mode")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	fixtures, err := LoadJudgeRule11Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule11Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if opts.CounterfactualModel && !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule11Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule11Summary{}, err
		}
	}
	runID, err := newJudgeEvalRunID()
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule11Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule11PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule11Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := newJudgeRule11Summary(opts, promptVariant, resultsPath, summaryPath, runID)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule11Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule11Summary{}, err
		}
		result.RunID = runID
		result.Evaluation = summary.Evaluation
		if err := encoder.Encode(result); err != nil {
			return JudgeRule11Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule11SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule11Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule11Summary{}, err
	}
	return summary, nil
}

func RescoreJudgeRule11(opts JudgeRule11RescoreOptions) (resultValue JudgeRule11Summary, returnErr error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeRule11Summary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule11Summary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeRule11Results(opts.ResultsPath)
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	source, sourceSummaryPath, err := loadJudgeEvalSourceSummary(opts.ResultsPath, opts.SourceSummaryPath, "judge_rule11")
	if err != nil {
		return JudgeRule11Summary{}, err
	}
	if err := validateJudgeEvalRescoreRows(opts.ResultsPath, source, results, judgeRule11ResultIdentity, validateJudgeRule11RescoreResult); err != nil {
		return JudgeRule11Summary{}, err
	}
	if err := validateJudgeEvalRescoreOutputPaths(opts.ResultsPath, sourceSummaryPath, opts.OutputDir); err != nil {
		return JudgeRule11Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule11Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule11Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule11Summary{
		RunID:               source.RunID,
		Evaluation:          "judge_rule11",
		Model:               source.Model,
		DryRun:              source.DryRun,
		Provenance:          newJudgeEvalRescoreProvenance(source, opts.ResultsPath, sourceSummaryPath),
		ExecutionMode:       source.ExecutionMode,
		CounterfactualModel: source.CounterfactualModel,
		PromptSource:        source.PromptSource,
		PromptName:          source.PromptName,
		PromptPath:          source.PromptPath,
		FixturesPath:        "rescored from " + opts.ResultsPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeRule11Slice{},
		ByIssueFamily:       map[string]JudgeRule11Slice{},
		ByTier:              map[string]JudgeRule11Slice{},
		ByMovant:            map[string]JudgeRule11Slice{},
		ByExpectedSanction:  map[string]JudgeRule11Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeRule11Result(&result)
		result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted
		if err := encoder.Encode(result); err != nil {
			return JudgeRule11Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule11SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule11Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule11Summary{}, err
	}
	return summary, nil
}

func newJudgeRule11Summary(opts JudgeRule11Options, promptVariant judgeRule11PromptVariant, resultsPath string, summaryPath string, runID string) JudgeRule11Summary {
	return JudgeRule11Summary{
		RunID:               runID,
		Evaluation:          "judge_rule11",
		Model:               opts.Model,
		DryRun:              opts.DryRun,
		Provenance:          newJudgeEvalProvenance(opts.Court, opts.PromptDir, opts.PromptFiles, promptVariant.Source, promptVariant.Name, promptVariant.Path, opts.Temperature, opts.Online, opts.DryRun, opts.Engine, judgeEvalExecutionMode(opts.CounterfactualModel)),
		ExecutionMode:       judgeEvalExecutionMode(opts.CounterfactualModel),
		CounterfactualModel: opts.CounterfactualModel,
		PromptSource:        promptVariant.Source,
		PromptName:          promptVariant.Name,
		PromptPath:          promptVariant.Path,
		PromptCopyPath:      promptVariant.CopyPath,
		FixturesPath:        opts.FixturesPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeRule11Slice{},
		ByIssueFamily:       map[string]JudgeRule11Slice{},
		ByTier:              map[string]JudgeRule11Slice{},
		ByMovant:            map[string]JudgeRule11Slice{},
		ByExpectedSanction:  map[string]JudgeRule11Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeRule11Fixture(
	ctx context.Context,
	opts JudgeRule11Options,
	promptVariant judgeRule11PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule11Fixture,
) (JudgeRule11Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule11Result{}, err
	}
	state := BuildJudgeRule11State(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeRule11Roles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeRule11Tool)
	if err != nil {
		return JudgeRule11Result{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
	}
	executionModel := opts.Model
	if opts.CounterfactualModel && !opts.DryRun {
		executionModel = modelRef.Model
	}
	responseClient := judgeEvalResponseClient(opts.DryRun, client, dryRunJudgeRule11Response(fixture))
	callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	execution, executionErr := executeJudgeOpportunity(callCtx, judgeOpportunityExecutionOptions{
		Engine:                     opts.Engine,
		State:                      state,
		Roles:                      roles,
		RolesPayload:               rolesPayload,
		Client:                     responseClient,
		Court:                      opts.Court,
		Model:                      executionModel,
		Temperature:                opts.Temperature,
		PromptDir:                  opts.PromptDir,
		PromptFiles:                opts.PromptFiles,
		MaxStepsPerTurn:            3,
		TurnIndex:                  1,
		CounterfactualModel:        opts.CounterfactualModel,
		RequireDeterministicAction: true,
		Objective: func(opportunity map[string]any) (string, error) {
			if strings.TrimSpace(promptVariant.Text) == "" {
				return stringField(opportunity, "objective"), nil
			}
			return renderJudgeRule11PromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeRule11Tool, executionErr)
	if decisionErr != nil {
		return JudgeRule11Result{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeRule11Response(fixture, opts.Model, opts.DryRun, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
	result.RawResponse = responseJSON(decision.RawResponse)
	result.ResponseExchanges = judgeEvalResponseExchangeJSON(execution.Exchanges)
	result.TurnLog = execution.TurnLog
	result.FinalState = execution.FinalState
	result.Provider = execution.Provider
	result.PromptSource = promptVariant.Source
	result.PromptName = promptVariant.Name
	result.PromptPath = promptVariant.Path
	result.ExecutionMode = judgeEvalExecutionMode(opts.CounterfactualModel)
	result.CounterfactualModel = opts.CounterfactualModel
	if result.InvalidReason == "" {
		result.InvalidReason = judgeEvalProceduralInvalidReason(executionErr)
	}
	leanAccepted, stepAccepted := judgeEvalExecutionAcceptance(execution, JudgeRule11Tool)
	result.LeanAccepted = leanAccepted
	result.StepAccepted = stepAccepted
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && stepAccepted && executionErr == nil
	return result, nil
}

func LoadJudgeRule11Fixtures(path string) (resultValue []JudgeRule11Fixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule11Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule11Fixture
		raw := []byte(line)
		if err := json.Unmarshal(raw, &fixture); err != nil {
			return nil, fmt.Errorf("parse fixtures %s line %d: %w", path, lineNo, err)
		}
		if err := requireJSONBooleanFields(raw, "expected_granted"); err != nil {
			return nil, fmt.Errorf("fixtures %s line %d: %w", path, lineNo, err)
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

func readJudgeRule11Results(path string) (resultValue []JudgeRule11Result, returnErr error) {
	return readJudgeEvalJSONL[JudgeRule11Result](path, judgeEvalRequiredResultFields(
		"tier", "issue_family", "case_theme", "movant", "target_party", "challenged_filing", "filing_text",
		"notice_text", "notice_served_at", "motion_filed_at", "motion_text", "opposition_text", "expected_granted",
		"expected_sanction_type", "expected_reason_tags", "severity", "grant_correct", "sanction_correct", "matched_reason_tags",
	)...)
}

func judgeRule11ResultIdentity(result JudgeRule11Result) judgeEvalResultIdentity {
	return judgeEvalResultIdentity{
		RunID:               result.RunID,
		Evaluation:          result.Evaluation,
		ID:                  result.ID,
		Model:               result.Model,
		DryRun:              result.DryRun,
		PromptSource:        resultJudgeRule11PromptSource(result),
		PromptName:          resultJudgeRule11PromptName(result),
		PromptPath:          result.PromptPath,
		ExecutionMode:       result.ExecutionMode,
		CounterfactualModel: result.CounterfactualModel,
	}
}

func validateJudgeRule11RescoreResult(result JudgeRule11Result) error {
	return (JudgeRule11Fixture{
		ID:                     result.ID,
		Tier:                   result.Tier,
		IssueFamily:            result.IssueFamily,
		CaseTheme:              result.CaseTheme,
		Movant:                 result.Movant,
		TargetParty:            result.TargetParty,
		ChallengedFiling:       result.ChallengedFiling,
		FilingText:             result.FilingText,
		NoticeText:             result.NoticeText,
		NoticeServedAt:         result.NoticeServedAt,
		MotionFiledAt:          result.MotionFiledAt,
		CorrectionText:         result.CorrectionText,
		MotionText:             result.MotionText,
		OppositionText:         result.OppositionText,
		ExpectedGranted:        result.ExpectedGranted,
		ExpectedSanctionType:   result.ExpectedSanctionType,
		ExpectedSanctionAmount: result.ExpectedSanctionAmount,
		ExpectedReasonTags:     result.ExpectedReasonTags,
		Severity:               result.Severity,
		ContextNotes:           result.ContextNotes,
	}).Validate()
}

func loadJudgeRule11PromptVariant(path string, name string, outputDir string) (judgeRule11PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule11PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule11PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule11PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeRule11PromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeRule11PromptVariant{Source: "file:" + path, Name: name, Path: path, CopyPath: copyPath, Text: text}, nil
}

func (f JudgeRule11Fixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if normalizeParty(f.Movant) == "" {
		return fmt.Errorf("fixture %s invalid movant %q", f.ID, f.Movant)
	}
	if normalizeParty(f.TargetParty) == "" {
		return fmt.Errorf("fixture %s invalid target_party %q", f.ID, f.TargetParty)
	}
	if normalizeParty(f.Movant) == normalizeParty(f.TargetParty) {
		return fmt.Errorf("fixture %s movant and target_party must differ", f.ID)
	}
	if strings.TrimSpace(f.ChallengedFiling) == "" {
		return fmt.Errorf("fixture %s missing challenged_filing", f.ID)
	}
	if strings.TrimSpace(f.FilingText) == "" {
		return fmt.Errorf("fixture %s missing filing_text", f.ID)
	}
	if strings.TrimSpace(f.NoticeText) == "" {
		return fmt.Errorf("fixture %s missing notice_text", f.ID)
	}
	if strings.TrimSpace(f.MotionText) == "" {
		return fmt.Errorf("fixture %s missing motion_text", f.ID)
	}
	if strings.TrimSpace(f.OppositionText) == "" {
		return fmt.Errorf("fixture %s missing opposition_text", f.ID)
	}
	if !validJudgeRule11ExpectedSanctionType(f.ExpectedSanctionType) {
		return fmt.Errorf("fixture %s invalid expected_sanction_type %q", f.ID, f.ExpectedSanctionType)
	}
	if f.ExpectedGranted && f.ExpectedSanctionType == "none" {
		return fmt.Errorf("fixture %s granted motion requires non-none expected_sanction_type", f.ID)
	}
	if !f.ExpectedGranted && f.ExpectedSanctionType != "none" {
		return fmt.Errorf("fixture %s denied motion must expect none sanction", f.ID)
	}
	if judgeRule11SanctionTypeIsMonetary(f.ExpectedSanctionType) && f.ExpectedSanctionAmount <= 0 {
		return fmt.Errorf("fixture %s monetary expected_sanction_amount must be positive", f.ID)
	}
	if !judgeRule11SanctionTypeIsMonetary(f.ExpectedSanctionType) && f.ExpectedSanctionAmount != 0 {
		return fmt.Errorf("fixture %s non-monetary expected_sanction_amount must be zero", f.ID)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule11State(f JudgeRule11Fixture) map[string]any {
	movant := normalizeParty(f.Movant)
	target := normalizeParty(f.TargetParty)
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-rule11-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   true,
			"status":                        "filed",
			"resolution":                    "pending",
			"trial_mode":                    "jury",
			"phase":                         "none",
			"last_pleading_served_on":       "2026-07-01",
			"jury_demanded_on":              "2026-07-01",
			"jury_configuration":            map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6},
			"single_claim":                  defaultJudgeEvalClaim(),
			"jurisdictional_allegations":    nil,
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
			"docket":                        judgeRule11Docket(f, movant, target),
			"decision_traces":               judgeRule11DecisionTraces(f, movant, target),
		},
	}
}

func judgeRule11Docket(f JudgeRule11Fixture, movant string, target string) []any {
	entries := []any{
		map[string]any{"title": "Complaint filed", "description": "plaintiff: " + strings.TrimSpace(f.CaseTheme)},
		map[string]any{"title": "Challenged Filing", "description": strings.TrimSpace(f.ChallengedFiling) + ": " + strings.TrimSpace(f.FilingText)},
		map[string]any{"title": "Rule 11 Safe Harbor Notice", "description": movant + " to " + target + " served_at " + strings.TrimSpace(f.NoticeServedAt) + ": " + strings.TrimSpace(f.NoticeText)},
	}
	if strings.TrimSpace(f.CorrectionText) != "" {
		entries = append(entries, map[string]any{"title": "Withdrawal or Correction", "description": target + ": " + strings.TrimSpace(f.CorrectionText)})
	}
	entries = append(entries, map[string]any{"title": "Rule 11 Motion", "description": movant + " filed_at " + strings.TrimSpace(f.MotionFiledAt) + ": " + strings.TrimSpace(f.MotionText)})
	entries = append(entries, map[string]any{"title": "Rule 11 Opposition", "description": target + ": " + strings.TrimSpace(f.OppositionText)})
	return entries
}

func judgeRule11DecisionTraces(f JudgeRule11Fixture, movant string, target string) []any {
	traces := []any{
		map[string]any{"action": "file_complaint", "outcome": "filed", "citations": []any{"FRCP 3", "FRCP 8(a)"}},
		map[string]any{"action": "serve_rule11_safe_harbor_notice", "outcome": movant + ":served:" + target, "citations": []any{"FRCP 11(c)(2)"}},
	}
	if strings.TrimSpace(f.CorrectionText) != "" {
		traces = append(traces, map[string]any{"action": "withdraw_or_correct_filing", "outcome": target + ":corrected", "citations": []any{"FRCP 11(c)(2)"}})
	}
	traces = append(traces, map[string]any{"action": "file_rule11_motion", "outcome": movant + ":filed", "citations": []any{"FRCP 11(c)(2)"}})
	return traces
}

func buildJudgeRule11Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule11Fixture,
	promptVariant judgeRule11PromptVariant,
) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeRule11PromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeRule11Tool, view, opportunity, objective)
}

func scoreJudgeRule11Response(
	fixture JudgeRule11Fixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule11Result {
	result := JudgeRule11Result{
		ID:                     fixture.ID,
		Tier:                   fixture.Tier,
		IssueFamily:            strings.TrimSpace(fixture.IssueFamily),
		CaseTheme:              strings.TrimSpace(fixture.CaseTheme),
		Movant:                 normalizeParty(fixture.Movant),
		TargetParty:            normalizeParty(fixture.TargetParty),
		ChallengedFiling:       strings.TrimSpace(fixture.ChallengedFiling),
		FilingText:             strings.TrimSpace(fixture.FilingText),
		NoticeText:             strings.TrimSpace(fixture.NoticeText),
		NoticeServedAt:         strings.TrimSpace(fixture.NoticeServedAt),
		MotionFiledAt:          strings.TrimSpace(fixture.MotionFiledAt),
		CorrectionText:         strings.TrimSpace(fixture.CorrectionText),
		MotionText:             strings.TrimSpace(fixture.MotionText),
		OppositionText:         strings.TrimSpace(fixture.OppositionText),
		ExpectedGranted:        fixture.ExpectedGranted,
		ExpectedSanctionType:   strings.TrimSpace(fixture.ExpectedSanctionType),
		ExpectedSanctionAmount: fixture.ExpectedSanctionAmount,
		ExpectedReasonTags:     append([]string{}, fixture.ExpectedReasonTags...),
		Severity:               normalizedSeverity(fixture.Severity),
		ContextNotes:           strings.TrimSpace(fixture.ContextNotes),
		Model:                  model,
		DryRun:                 dryRun,
		State:                  state,
		View:                   view,
		Opportunity:            opportunity,
		Input:                  input,
		RawResponse:            responseJSON(resp),
	}
	payload, invalid := extractJudgeRule11Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	if got := intField(payload, "motion_index"); got != 0 {
		result.InvalidReason = "wrong_motion_index"
		return result
	}
	result.MotionIndex = 0
	granted, ok := payload["granted"].(bool)
	if !ok {
		result.InvalidReason = "malformed_granted"
		return result
	}
	result.Granted = &granted
	sanctionType, hasSanctionType := stringFieldPresence(payload, "sanction_type")
	result.SanctionType = strings.TrimSpace(sanctionType)
	amount, hasAmount, amountOK := optionalFloatField(payload, "sanction_amount")
	if !amountOK {
		result.InvalidReason = "malformed_sanction_amount"
		return result
	}
	if hasAmount {
		result.SanctionAmount = &amount
	}
	detail, hasDetail := stringFieldPresence(payload, "sanction_detail")
	if !hasDetail {
		result.InvalidReason = "missing_sanction_detail"
		return result
	}
	result.SanctionDetail = strings.TrimSpace(detail)
	result.Reasoning = strings.TrimSpace(stringField(payload, "reasoning"))
	if result.Reasoning == "" {
		result.InvalidReason = "empty_reasoning"
		return result
	}
	if granted {
		if !hasSanctionType || result.SanctionType == "" {
			result.InvalidReason = "granted_missing_sanction_type"
			return result
		}
		if !validJudgeRule11GrantedSanctionType(result.SanctionType) {
			result.InvalidReason = "invalid_sanction_type"
			return result
		}
		if result.SanctionDetail == "" {
			result.InvalidReason = "empty_sanction_detail"
			return result
		}
		if judgeRule11SanctionTypeIsMonetary(result.SanctionType) {
			if !hasAmount {
				result.InvalidReason = "monetary_missing_amount"
				return result
			}
			if amount <= 0 {
				result.InvalidReason = "monetary_nonpositive_amount"
				return result
			}
		} else if hasAmount && amount != 0 {
			result.InvalidReason = "non_monetary_with_amount"
			return result
		}
	} else {
		if hasSanctionType && result.SanctionType != "" {
			result.InvalidReason = "denied_with_sanction_type"
			return result
		}
		if hasAmount && amount != 0 {
			result.InvalidReason = "denied_with_sanction_amount"
			return result
		}
		if result.SanctionDetail != "" {
			result.InvalidReason = "denied_with_sanction_detail"
			return result
		}
	}
	rescoreJudgeRule11Result(&result)
	return result
}

func rescoreJudgeRule11Result(result *JudgeRule11Result) {
	if result == nil || result.InvalidReason != "" || result.Granted == nil {
		return
	}
	result.GrantCorrect = *result.Granted == result.ExpectedGranted
	result.SanctionCorrect = judgeRule11SanctionCorrect(*result)
	result.OutcomeCorrect = result.GrantCorrect && result.SanctionCorrect
	result.MatchedReasonTags = matchedJudgeRule11ReasonTags(result.Reasoning+" "+result.SanctionDetail, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeRule11Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule11Tool {
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

func dryRunJudgeRule11Response(f JudgeRule11Fixture) openai.Response {
	payload := map[string]any{
		"motion_index":    0,
		"granted":         f.ExpectedGranted,
		"sanction_detail": dryRunJudgeRule11SanctionDetail(f),
		"reasoning":       "gold tags: " + strings.Join(f.ExpectedReasonTags, ", "),
	}
	if f.ExpectedGranted {
		payload["sanction_type"] = strings.TrimSpace(f.ExpectedSanctionType)
		if judgeRule11SanctionTypeIsMonetary(f.ExpectedSanctionType) {
			payload["sanction_amount"] = f.ExpectedSanctionAmount
		} else {
			payload["sanction_amount"] = 0
		}
	}
	return openai.Response{
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID:    "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:      JudgeRule11Tool,
			Arguments: payload,
		}},
	}
}

func dryRunJudgeRule11SanctionDetail(f JudgeRule11Fixture) string {
	if !f.ExpectedGranted {
		return ""
	}
	return "gold sanction: " + strings.TrimSpace(f.ExpectedSanctionType)
}

func renderJudgeRule11PromptTemplate(template string, fixture JudgeRule11Fixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeRule11PromptTemplate", template,
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{movant}}", normalizeParty(fixture.Movant),
		"{{target_party}}", normalizeParty(fixture.TargetParty),
		"{{challenged_filing}}", strings.TrimSpace(fixture.ChallengedFiling),
		"{{filing_text}}", strings.TrimSpace(fixture.FilingText),
		"{{notice_text}}", strings.TrimSpace(fixture.NoticeText),
		"{{notice_served_at}}", strings.TrimSpace(fixture.NoticeServedAt),
		"{{motion_filed_at}}", strings.TrimSpace(fixture.MotionFiledAt),
		"{{correction_text}}", strings.TrimSpace(fixture.CorrectionText),
		"{{motion_text}}", strings.TrimSpace(fixture.MotionText),
		"{{opposition_text}}", strings.TrimSpace(fixture.OppositionText),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeRule11Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule11Tool}}}
}

func applyJudgeRule11SummaryResult(summary *JudgeRule11Summary, result JudgeRule11Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else {
		if result.GrantCorrect {
			summary.GrantCorrect++
		}
		if result.OutcomeCorrect {
			summary.Correct++
		} else if result.GrantCorrect && !result.SanctionCorrect {
			summary.SanctionMismatches++
		} else {
			classifyJudgeRule11Error(result, &summary.FalseGrants, &summary.FalseDenials)
		}
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateJudgeRule11Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeRule11Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeRule11Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeRule11Slice(summary.ByMovant, result.Movant, result, weight)
	updateJudgeRule11Slice(summary.ByExpectedSanction, result.ExpectedSanctionType, result, weight)
}

func updateJudgeRule11Slice(m map[string]JudgeRule11Slice, key string, result JudgeRule11Result, weight float64) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unspecified"
	}
	s := m[key]
	s.Total++
	s.Weight += weight
	if result.InvalidReason != "" {
		s.Invalid++
	} else {
		if result.GrantCorrect {
			s.GrantCorrect++
		}
		if result.OutcomeCorrect {
			s.Correct++
			s.CorrectWeight += weight
		} else if result.GrantCorrect && !result.SanctionCorrect {
			s.SanctionMismatches++
		} else {
			classifyJudgeRule11Error(result, &s.FalseGrants, &s.FalseDenials)
		}
	}
	m[key] = s
}

func classifyJudgeRule11Error(result JudgeRule11Result, falseGrants *int, falseDenials *int) {
	if result.Granted == nil {
		return
	}
	if *result.Granted && !result.ExpectedGranted {
		(*falseGrants)++
	}
	if !*result.Granted && result.ExpectedGranted {
		(*falseDenials)++
	}
}

func finalizeJudgeRule11Summary(summary *JudgeRule11Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.GrantAccuracy = float64(summary.GrantCorrect) / float64(summary.Total)
		summary.FalseGrantRate = float64(summary.FalseGrants) / float64(summary.Total)
		summary.FalseDenialRate = float64(summary.FalseDenials) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeJudgeRule11Slices(summary.ByReasonTag)
	finalizeJudgeRule11Slices(summary.ByIssueFamily)
	finalizeJudgeRule11Slices(summary.ByTier)
	finalizeJudgeRule11Slices(summary.ByMovant)
	finalizeJudgeRule11Slices(summary.ByExpectedSanction)
}

func finalizeJudgeRule11Slices(m map[string]JudgeRule11Slice) {
	for key, s := range m {
		if s.Total > 0 {
			s.Accuracy = float64(s.Correct) / float64(s.Total)
			s.GrantAccuracy = float64(s.GrantCorrect) / float64(s.Total)
		}
		if s.Weight > 0 {
			s.WeightedAccuracy = s.CorrectWeight / s.Weight
		}
		m[key] = s
	}
}

func judgeRule11SanctionCorrect(result JudgeRule11Result) bool {
	if result.ExpectedSanctionType == "none" {
		if result.SanctionType != "" {
			return false
		}
		if result.SanctionDetail != "" {
			return false
		}
		return result.SanctionAmount == nil || math.Abs(*result.SanctionAmount) <= 0.01
	}
	if strings.TrimSpace(result.SanctionType) != strings.TrimSpace(result.ExpectedSanctionType) {
		return false
	}
	if result.SanctionDetail == "" {
		return false
	}
	if judgeRule11SanctionTypeIsMonetary(result.ExpectedSanctionType) {
		if result.SanctionAmount == nil {
			return false
		}
		return math.Abs(*result.SanctionAmount-result.ExpectedSanctionAmount) <= 0.01
	}
	return result.SanctionAmount == nil || math.Abs(*result.SanctionAmount) <= 0.01
}

func matchedJudgeRule11ReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if judgeRule11ReasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeRule11ReasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range judgeRule11ReasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func judgeRule11ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"frivolous_legal":        {"frivolous", "no legal basis", "foreclosed", "not warranted by law"},
		"no_evidentiary_support": {"no evidentiary support", "no factual support", "without evidentiary support", "no investigation", "available records contradicted"},
		"improper_purpose":       {"improper purpose", "harass", "delay", "increase costs"},
		"reasonable_inquiry":     {"reasonable inquiry", "investigated", "reviewed records", "reasonable investigation"},
		"reasonable_extension":   {"extension", "nonfrivolous argument", "change in law", "good faith argument"},
		"likely_support":         {"likely have support", "after discovery", "information and belief", "needs discovery"},
		"safe_harbor_defect":     {"safe harbor", "21 day", "twenty one", "premature"},
		"corrected":              {"corrected", "withdrawn", "cured", "safe harbor correction"},
		"discovery_exclusion":    {"discovery", "rule 37", "not rule 11"},
		"weak_merits":            {"losing position", "weak merits", "merits dispute", "not sanctionable"},
		"proportional_sanction":  {"proportionate", "admonition", "directive", "fee", "penalty"},
	}
}

func validJudgeRule11ExpectedSanctionType(value string) bool {
	if value == "none" {
		return true
	}
	return validJudgeRule11GrantedSanctionType(value)
}

func validJudgeRule11GrantedSanctionType(value string) bool {
	switch strings.TrimSpace(value) {
	case "admonition", "non_monetary_directive", "monetary_penalty", "fee_shift":
		return true
	default:
		return false
	}
}

func judgeRule11SanctionTypeIsMonetary(value string) bool {
	switch strings.TrimSpace(value) {
	case "monetary_penalty", "fee_shift":
		return true
	default:
		return false
	}
}

func stringFieldPresence(m map[string]any, key string) (string, bool) {
	value, ok := m[key]
	if !ok || value == nil {
		return "", false
	}
	s, ok := value.(string)
	if !ok {
		return "", true
	}
	return s, true
}

func resultJudgeRule11PromptSource(result JudgeRule11Result) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultJudgeRule11PromptName(result JudgeRule11Result) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}
