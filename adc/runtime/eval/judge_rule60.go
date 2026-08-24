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

const JudgeRule60Tool = "resolve_rule60_motion"

type JudgeRule60Fixture struct {
	ID                 string   `json:"id"`
	Tier               int      `json:"tier"`
	IssueFamily        string   `json:"issue_family"`
	CaseTheme          string   `json:"case_theme"`
	JudgmentSummary    string   `json:"judgment_summary"`
	MotionGround       string   `json:"motion_ground"`
	MotionText         string   `json:"motion_text"`
	OppositionText     string   `json:"opposition_text"`
	DefaultJudgment    bool     `json:"default_judgment,omitempty"`
	ExpectedGranted    bool     `json:"expected_granted"`
	RequiredConcepts   []string `json:"required_concepts"`
	ProhibitedConcepts []string `json:"prohibited_concepts,omitempty"`
	ExpectedReasonTags []string `json:"expected_reason_tags"`
	Severity           float64  `json:"severity"`
	ContextNotes       string   `json:"context_notes,omitempty"`
}

type JudgeRule60Options struct {
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

type JudgeRule60RescoreOptions struct {
	ResultsPath       string
	SourceSummaryPath string
	OutputDir         string
}

type JudgeRule60Summary struct {
	RunID                 string                      `json:"run_id"`
	Evaluation            string                      `json:"evaluation"`
	Model                 string                      `json:"model"`
	DryRun                bool                        `json:"dry_run"`
	ExecutionMode         string                      `json:"execution_mode"`
	CounterfactualModel   bool                        `json:"counterfactual_model"`
	Provenance            JudgeEvalProvenance         `json:"provenance"`
	PromptSource          string                      `json:"prompt_source"`
	PromptName            string                      `json:"prompt_name"`
	PromptPath            string                      `json:"prompt_path,omitempty"`
	PromptCopyPath        string                      `json:"prompt_copy_path,omitempty"`
	FixturesPath          string                      `json:"fixtures_path"`
	OutputDir             string                      `json:"output_dir"`
	ResultsPath           string                      `json:"results_path"`
	SummaryPath           string                      `json:"summary_path"`
	Total                 int                         `json:"total"`
	Correct               int                         `json:"correct"`
	GrantCorrect          int                         `json:"grant_correct"`
	RequiredCorrect       int                         `json:"required_correct"`
	ProhibitedCorrect     int                         `json:"prohibited_correct"`
	ReasonCorrect         int                         `json:"reason_correct"`
	Invalid               int                         `json:"invalid"`
	FalseGrants           int                         `json:"false_grants"`
	FalseDenials          int                         `json:"false_denials"`
	LeanRejected          int                         `json:"lean_rejected"`
	StepRejected          int                         `json:"step_rejected"`
	Accuracy              float64                     `json:"accuracy"`
	WeightedAccuracy      float64                     `json:"weighted_accuracy"`
	FalseGrantRate        float64                     `json:"false_grant_rate"`
	FalseDenialRate       float64                     `json:"false_denial_rate"`
	InvalidRate           float64                     `json:"invalid_rate"`
	LeanRejectionRate     float64                     `json:"lean_rejection_rate"`
	ByReasonTag           map[string]JudgeRule60Slice `json:"by_reason_tag"`
	ByIssueFamily         map[string]JudgeRule60Slice `json:"by_issue_family"`
	ByTier                map[string]JudgeRule60Slice `json:"by_tier"`
	ByExpectedDisposition map[string]JudgeRule60Slice `json:"by_expected_disposition"`
	GeneratedAt           string                      `json:"generated_at"`
}

type JudgeRule60Slice struct {
	Total            int     `json:"total"`
	Correct          int     `json:"correct"`
	FalseGrants      int     `json:"false_grants"`
	FalseDenials     int     `json:"false_denials"`
	Invalid          int     `json:"invalid"`
	LeanRejected     int     `json:"lean_rejected"`
	StepRejected     int     `json:"step_rejected"`
	Weight           float64 `json:"weight"`
	CorrectWeight    float64 `json:"correct_weight"`
	Accuracy         float64 `json:"accuracy"`
	WeightedAccuracy float64 `json:"weighted_accuracy"`
}

type JudgeRule60Result struct {
	RunID                     string            `json:"run_id"`
	Evaluation                string            `json:"evaluation"`
	ID                        string            `json:"id"`
	Tier                      int               `json:"tier"`
	IssueFamily               string            `json:"issue_family"`
	CaseTheme                 string            `json:"case_theme"`
	JudgmentSummary           string            `json:"judgment_summary"`
	MotionGround              string            `json:"motion_ground"`
	MotionText                string            `json:"motion_text"`
	OppositionText            string            `json:"opposition_text"`
	DefaultJudgment           bool              `json:"default_judgment"`
	ExpectedGranted           bool              `json:"expected_granted"`
	RequiredConcepts          []string          `json:"required_concepts"`
	ProhibitedConcepts        []string          `json:"prohibited_concepts,omitempty"`
	ExpectedReasonTags        []string          `json:"expected_reason_tags"`
	Severity                  float64           `json:"severity"`
	ContextNotes              string            `json:"context_notes,omitempty"`
	Model                     string            `json:"model"`
	DryRun                    bool              `json:"dry_run"`
	ExecutionMode             string            `json:"execution_mode"`
	CounterfactualModel       bool              `json:"counterfactual_model"`
	PromptSource              string            `json:"prompt_source"`
	PromptName                string            `json:"prompt_name"`
	PromptPath                string            `json:"prompt_path,omitempty"`
	State                     map[string]any    `json:"state"`
	View                      map[string]any    `json:"view"`
	Opportunity               map[string]any    `json:"opportunity"`
	Input                     []map[string]any  `json:"input"`
	RawResponse               map[string]any    `json:"raw_response"`
	ResponseExchanges         []map[string]any  `json:"response_exchanges,omitempty"`
	TurnLog                   runner.TurnLog    `json:"turn_log"`
	FinalState                map[string]any    `json:"final_state,omitempty"`
	Provider                  openai.Accounting `json:"provider"`
	ToolPayload               map[string]any    `json:"tool_payload,omitempty"`
	MotionIndex               int               `json:"motion_index"`
	Granted                   bool              `json:"granted"`
	ReliefSummary             string            `json:"relief_summary,omitempty"`
	MissingRequiredConcepts   []string          `json:"missing_required_concepts,omitempty"`
	PresentProhibitedConcepts []string          `json:"present_prohibited_concepts,omitempty"`
	MatchedReasonTags         []string          `json:"matched_reason_tags"`
	GrantCorrect              bool              `json:"grant_correct"`
	RequiredCorrect           bool              `json:"required_correct"`
	ProhibitedCorrect         bool              `json:"prohibited_correct"`
	OutcomeCorrect            bool              `json:"outcome_correct"`
	ReasonCorrect             bool              `json:"reason_correct"`
	InvalidReason             string            `json:"invalid_reason,omitempty"`
	LeanAccepted              bool              `json:"lean_accepted"`
	StepAccepted              bool              `json:"step_accepted"`
	LeanError                 string            `json:"lean_error,omitempty"`
}

type judgeRule60PromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeRule60(ctx context.Context, opts JudgeRule60Options) (resultValue JudgeRule60Summary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule60Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule60Summary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	fixtures, err := LoadJudgeRule60Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule60Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule60Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule60Summary{}, err
		}
	}
	runID, err := newJudgeEvalRunID()
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule60Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule60PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule60Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := newJudgeRule60Summary(opts, promptVariant, resultsPath, summaryPath, runID)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule60Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule60Summary{}, err
		}
		result.RunID = runID
		result.Evaluation = summary.Evaluation
		result.ExecutionMode = summary.ExecutionMode
		result.CounterfactualModel = summary.CounterfactualModel
		if err := encoder.Encode(result); err != nil {
			return JudgeRule60Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule60SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule60Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule60Summary{}, err
	}
	return summary, nil
}

func RescoreJudgeRule60(opts JudgeRule60RescoreOptions) (resultValue JudgeRule60Summary, returnErr error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeRule60Summary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule60Summary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeRule60Results(opts.ResultsPath)
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	source, sourceSummaryPath, err := loadJudgeEvalSourceSummary(opts.ResultsPath, opts.SourceSummaryPath, "judge_rule60")
	if err != nil {
		return JudgeRule60Summary{}, err
	}
	if err := validateJudgeEvalRescoreRows(opts.ResultsPath, source, results, judgeRule60ResultIdentity, validateJudgeRule60RescoreResult); err != nil {
		return JudgeRule60Summary{}, err
	}
	if err := validateJudgeEvalRescoreOutputPaths(opts.ResultsPath, sourceSummaryPath, opts.OutputDir); err != nil {
		return JudgeRule60Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule60Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule60Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule60Summary{
		RunID:                 source.RunID,
		Evaluation:            "judge_rule60",
		Model:                 source.Model,
		DryRun:                source.DryRun,
		ExecutionMode:         source.ExecutionMode,
		CounterfactualModel:   source.CounterfactualModel,
		Provenance:            newJudgeEvalRescoreProvenance(source, opts.ResultsPath, sourceSummaryPath),
		PromptSource:          source.PromptSource,
		PromptName:            source.PromptName,
		PromptPath:            source.PromptPath,
		FixturesPath:          "rescored from " + opts.ResultsPath,
		OutputDir:             opts.OutputDir,
		ResultsPath:           resultsPath,
		SummaryPath:           summaryPath,
		ByReasonTag:           map[string]JudgeRule60Slice{},
		ByIssueFamily:         map[string]JudgeRule60Slice{},
		ByTier:                map[string]JudgeRule60Slice{},
		ByExpectedDisposition: map[string]JudgeRule60Slice{},
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeRule60Result(&result)
		if err := encoder.Encode(result); err != nil {
			return JudgeRule60Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule60SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule60Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule60Summary{}, err
	}
	return summary, nil
}

func newJudgeRule60Summary(opts JudgeRule60Options, promptVariant judgeRule60PromptVariant, resultsPath string, summaryPath string, runID string) JudgeRule60Summary {
	return JudgeRule60Summary{
		RunID:                 runID,
		Evaluation:            "judge_rule60",
		Model:                 opts.Model,
		DryRun:                opts.DryRun,
		ExecutionMode:         "production",
		CounterfactualModel:   false,
		Provenance:            newJudgeEvalProvenance(opts.Court, opts.PromptDir, opts.PromptFiles, promptVariant.Source, promptVariant.Name, promptVariant.Path, opts.Temperature, opts.Online, opts.DryRun, opts.Engine, "production"),
		PromptSource:          promptVariant.Source,
		PromptName:            promptVariant.Name,
		PromptPath:            promptVariant.Path,
		PromptCopyPath:        promptVariant.CopyPath,
		FixturesPath:          opts.FixturesPath,
		OutputDir:             opts.OutputDir,
		ResultsPath:           resultsPath,
		SummaryPath:           summaryPath,
		ByReasonTag:           map[string]JudgeRule60Slice{},
		ByIssueFamily:         map[string]JudgeRule60Slice{},
		ByTier:                map[string]JudgeRule60Slice{},
		ByExpectedDisposition: map[string]JudgeRule60Slice{},
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeRule60Fixture(
	ctx context.Context,
	opts JudgeRule60Options,
	promptVariant judgeRule60PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule60Fixture,
) (JudgeRule60Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule60Result{}, err
	}
	state := BuildJudgeRule60State(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeRule60Roles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeRule60Tool)
	if err != nil {
		return JudgeRule60Result{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
	}
	executionModel := opts.Model
	if !opts.DryRun {
		executionModel = modelRef.Model
	}
	responseClient := judgeEvalResponseClient(opts.DryRun, client, dryRunJudgeRule60Response(fixture))
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
			return renderJudgeRule60PromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeRule60Tool, executionErr)
	if decisionErr != nil {
		return JudgeRule60Result{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeRule60Response(fixture, opts.Model, opts.DryRun, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
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
	result.LeanAccepted, result.StepAccepted = judgeEvalExecutionAcceptance(execution, JudgeRule60Tool)
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	rescoreJudgeRule60Result(&result)
	result.OutcomeCorrect = result.OutcomeCorrect && executionErr == nil
	return result, nil
}

func LoadJudgeRule60Fixtures(path string) (resultValue []JudgeRule60Fixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule60Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule60Fixture
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

func readJudgeRule60Results(path string) (resultValue []JudgeRule60Result, returnErr error) {
	return readJudgeEvalJSONL[JudgeRule60Result](path, judgeEvalRequiredResultFields(
		"tier", "issue_family", "case_theme", "judgment_summary", "motion_ground", "motion_text", "opposition_text",
		"default_judgment", "expected_granted", "required_concepts", "expected_reason_tags", "severity", "granted",
		"grant_correct", "required_correct", "prohibited_correct", "matched_reason_tags",
	)...)
}

func judgeRule60ResultIdentity(result JudgeRule60Result) judgeEvalResultIdentity {
	return judgeEvalResultIdentity{
		RunID:               result.RunID,
		Evaluation:          result.Evaluation,
		ID:                  result.ID,
		Model:               result.Model,
		DryRun:              result.DryRun,
		PromptSource:        resultJudgeRule60PromptSource(result),
		PromptName:          resultJudgeRule60PromptName(result),
		PromptPath:          result.PromptPath,
		ExecutionMode:       result.ExecutionMode,
		CounterfactualModel: result.CounterfactualModel,
	}
}

func validateJudgeRule60RescoreResult(result JudgeRule60Result) error {
	return (JudgeRule60Fixture{
		ID:                 result.ID,
		Tier:               result.Tier,
		IssueFamily:        result.IssueFamily,
		CaseTheme:          result.CaseTheme,
		JudgmentSummary:    result.JudgmentSummary,
		MotionGround:       result.MotionGround,
		MotionText:         result.MotionText,
		OppositionText:     result.OppositionText,
		DefaultJudgment:    result.DefaultJudgment,
		ExpectedGranted:    result.ExpectedGranted,
		RequiredConcepts:   result.RequiredConcepts,
		ProhibitedConcepts: result.ProhibitedConcepts,
		ExpectedReasonTags: result.ExpectedReasonTags,
		Severity:           result.Severity,
		ContextNotes:       result.ContextNotes,
	}).Validate()
}

func loadJudgeRule60PromptVariant(path string, name string, outputDir string) (judgeRule60PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule60PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule60PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule60PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeRule60PromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeRule60PromptVariant{Source: "file:" + path, Name: name, Path: path, CopyPath: copyPath, Text: text}, nil
}

func resultJudgeRule60PromptSource(result JudgeRule60Result) string {
	if strings.TrimSpace(result.PromptSource) != "" {
		return strings.TrimSpace(result.PromptSource)
	}
	if strings.TrimSpace(result.PromptPath) != "" {
		return "file:" + strings.TrimSpace(result.PromptPath)
	}
	return "production"
}

func resultJudgeRule60PromptName(result JudgeRule60Result) string {
	if strings.TrimSpace(result.PromptName) != "" {
		return strings.TrimSpace(result.PromptName)
	}
	return "unknown"
}

func (f JudgeRule60Fixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if strings.TrimSpace(f.CaseTheme) == "" {
		return fmt.Errorf("fixture %s missing case_theme", f.ID)
	}
	if strings.TrimSpace(f.JudgmentSummary) == "" {
		return fmt.Errorf("fixture %s missing judgment_summary", f.ID)
	}
	if strings.TrimSpace(f.MotionGround) == "" {
		return fmt.Errorf("fixture %s missing motion_ground", f.ID)
	}
	if strings.TrimSpace(f.MotionText) == "" {
		return fmt.Errorf("fixture %s missing motion_text", f.ID)
	}
	if strings.TrimSpace(f.OppositionText) == "" {
		return fmt.Errorf("fixture %s missing opposition_text", f.ID)
	}
	if len(nonemptyRule52Strings(f.RequiredConcepts)) == 0 {
		return fmt.Errorf("fixture %s missing required_concepts", f.ID)
	}
	if len(nonemptyRule52Strings(f.ExpectedReasonTags)) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule60State(f JudgeRule60Fixture) map[string]any {
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-rule60-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "judgment_entered",
			"resolution":                    "pending",
			"trial_mode":                    "jury",
			"phase":                         "post_verdict",
			"last_pleading_served_on":       "2026-07-01",
			"jury_demanded_on":              "2026-07-02",
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
			"jury_verdict":                  map[string]any{"verdict_for": "plaintiff", "votes_for_verdict": 6, "required_votes": 6, "damages": 12000.0},
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
			"monetary_judgment":             12000.0,
			"docket":                        judgeRule60Docket(f),
			"decision_traces":               judgeRule60DecisionTraces(f),
		},
	}
}

func judgeRule60Docket(f JudgeRule60Fixture) []any {
	entries := []any{
		map[string]any{"title": "Judgment entered", "description": strings.TrimSpace(f.JudgmentSummary)},
	}
	if f.DefaultJudgment {
		entries = append(entries, map[string]any{"title": "Default judgment entered", "description": strings.TrimSpace(f.JudgmentSummary)})
	}
	entries = append(entries,
		map[string]any{"title": "Rule 60 Motion", "description": "ground=" + strings.TrimSpace(f.MotionGround) + " motion: " + strings.TrimSpace(f.MotionText)},
		map[string]any{"title": "Rule 60 Opposition", "description": strings.TrimSpace(f.OppositionText)},
	)
	return entries
}

func judgeRule60DecisionTraces(f JudgeRule60Fixture) []any {
	traces := []any{
		map[string]any{"action": "enter_judgment", "outcome": "jury verdict", "citations": []any{"FRCP 58"}},
	}
	if f.DefaultJudgment {
		traces = append(traces, map[string]any{"action": "enter_default_judgment", "outcome": "entered", "citations": []any{"FRCP 55(b)"}})
	}
	traces = append(traces, map[string]any{"action": "file_rule60_motion", "outcome": "filed", "citations": []any{"FRCP 60(b)"}})
	return traces
}

func buildJudgeRule60Input(view map[string]any, opportunity map[string]any, fixture JudgeRule60Fixture, promptVariant judgeRule60PromptVariant) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeRule60PromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeRule60Tool, view, opportunity, objective)
}

func scoreJudgeRule60Response(fixture JudgeRule60Fixture, model string, dryRun bool, state map[string]any, view map[string]any, opportunity map[string]any, input []map[string]any, resp openai.Response) JudgeRule60Result {
	result := JudgeRule60Result{
		ID:                 fixture.ID,
		Tier:               fixture.Tier,
		IssueFamily:        strings.TrimSpace(fixture.IssueFamily),
		CaseTheme:          strings.TrimSpace(fixture.CaseTheme),
		JudgmentSummary:    strings.TrimSpace(fixture.JudgmentSummary),
		MotionGround:       strings.TrimSpace(fixture.MotionGround),
		MotionText:         strings.TrimSpace(fixture.MotionText),
		OppositionText:     strings.TrimSpace(fixture.OppositionText),
		DefaultJudgment:    fixture.DefaultJudgment,
		ExpectedGranted:    fixture.ExpectedGranted,
		RequiredConcepts:   nonemptyRule52Strings(fixture.RequiredConcepts),
		ProhibitedConcepts: nonemptyRule52Strings(fixture.ProhibitedConcepts),
		ExpectedReasonTags: nonemptyRule52Strings(fixture.ExpectedReasonTags),
		Severity:           normalizedSeverity(fixture.Severity),
		ContextNotes:       strings.TrimSpace(fixture.ContextNotes),
		Model:              model,
		DryRun:             dryRun,
		State:              state,
		View:               view,
		Opportunity:        opportunity,
		Input:              input,
		RawResponse:        responseJSON(resp),
	}
	payload, invalid := extractJudgeRule60Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	result.MotionIndex = intField(payload, "motion_index")
	result.Granted, _ = payload["granted"].(bool)
	result.ReliefSummary = strings.TrimSpace(stringField(payload, "relief_summary"))
	if result.MotionIndex != 0 {
		result.InvalidReason = "wrong_motion_index"
		return result
	}
	if result.ReliefSummary == "" {
		result.InvalidReason = "missing_relief_summary"
		return result
	}
	scoreJudgeRule60Payload(&result)
	return result
}

func scoreJudgeRule60Payload(result *JudgeRule60Result) {
	result.GrantCorrect = result.Granted == result.ExpectedGranted
	result.MissingRequiredConcepts = missingJudgeRule60Concepts(result.ReliefSummary, result.RequiredConcepts)
	result.PresentProhibitedConcepts = presentJudgeRule60Concepts(result.ReliefSummary, result.ProhibitedConcepts)
	result.RequiredCorrect = len(result.MissingRequiredConcepts) == 0
	result.ProhibitedCorrect = len(result.PresentProhibitedConcepts) == 0
	result.MatchedReasonTags = matchedJudgeRule60ReasonTags(result.ReliefSummary, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func finalizeJudgeRule60Result(result *JudgeRule60Result) {
	if result == nil || result.InvalidReason != "" {
		return
	}
	result.OutcomeCorrect = result.GrantCorrect &&
		result.RequiredCorrect &&
		result.ProhibitedCorrect &&
		result.ReasonCorrect &&
		result.LeanAccepted &&
		result.StepAccepted
}

func rescoreJudgeRule60Result(result *JudgeRule60Result) {
	if result == nil || result.InvalidReason != "" {
		return
	}
	result.MissingRequiredConcepts = nil
	result.PresentProhibitedConcepts = nil
	result.MatchedReasonTags = nil
	result.GrantCorrect = false
	result.RequiredCorrect = false
	result.ProhibitedCorrect = false
	result.ReasonCorrect = false
	result.OutcomeCorrect = false
	scoreJudgeRule60Payload(result)
	finalizeJudgeRule60Result(result)
}

func extractJudgeRule60Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule60Tool {
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

func dryRunJudgeRule60Response(f JudgeRule60Fixture) openai.Response {
	summary := strings.Join(nonemptyRule52Strings(f.RequiredConcepts), "; ")
	return openai.Response{
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID: "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:   JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        f.ExpectedGranted,
				"relief_summary": summary,
			},
		}},
	}
}

func renderJudgeRule60PromptTemplate(template string, fixture JudgeRule60Fixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeRule60PromptTemplate", template,
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{judgment_summary}}", strings.TrimSpace(fixture.JudgmentSummary),
		"{{motion_ground}}", strings.TrimSpace(fixture.MotionGround),
		"{{motion_text}}", strings.TrimSpace(fixture.MotionText),
		"{{opposition_text}}", strings.TrimSpace(fixture.OppositionText),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeRule60Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule60Tool}}}
}

func applyJudgeRule60SummaryResult(summary *JudgeRule60Summary, result JudgeRule60Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else {
		if result.GrantCorrect {
			summary.GrantCorrect++
		} else if result.Granted && !result.ExpectedGranted {
			summary.FalseGrants++
		} else if !result.Granted && result.ExpectedGranted {
			summary.FalseDenials++
		}
		if result.RequiredCorrect {
			summary.RequiredCorrect++
		}
		if result.ProhibitedCorrect {
			summary.ProhibitedCorrect++
		}
		if !result.LeanAccepted {
			summary.LeanRejected++
		}
		if result.LeanAccepted && !result.StepAccepted {
			summary.StepRejected++
		}
		if result.OutcomeCorrect {
			summary.Correct++
		}
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateJudgeRule60Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeRule60Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeRule60Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeRule60Slice(summary.ByExpectedDisposition, judgeRule60DispositionKey(result.ExpectedGranted), result, weight)
}

func updateJudgeRule60Slice(m map[string]JudgeRule60Slice, key string, result JudgeRule60Result, weight float64) {
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
		if !result.GrantCorrect {
			if result.Granted && !result.ExpectedGranted {
				s.FalseGrants++
			} else if !result.Granted && result.ExpectedGranted {
				s.FalseDenials++
			}
		}
		if !result.LeanAccepted {
			s.LeanRejected++
		}
		if result.LeanAccepted && !result.StepAccepted {
			s.StepRejected++
		}
		if result.OutcomeCorrect {
			s.Correct++
			s.CorrectWeight += weight
		}
	}
	m[key] = s
}

func finalizeJudgeRule60Summary(summary *JudgeRule60Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
		summary.FalseGrantRate = float64(summary.FalseGrants) / float64(summary.Total)
		summary.FalseDenialRate = float64(summary.FalseDenials) / float64(summary.Total)
		summary.LeanRejectionRate = float64(summary.LeanRejected) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeJudgeRule60Slices(summary.ByReasonTag)
	finalizeJudgeRule60Slices(summary.ByIssueFamily)
	finalizeJudgeRule60Slices(summary.ByTier)
	finalizeJudgeRule60Slices(summary.ByExpectedDisposition)
}

func finalizeJudgeRule60Slices(m map[string]JudgeRule60Slice) {
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

func missingJudgeRule60Concepts(text string, concepts []string) []string {
	normalized := normalizeReasonText(text)
	missing := make([]string, 0)
	for _, concept := range nonemptyRule52Strings(concepts) {
		if !judgeRule60ConceptPresent(normalized, concept) {
			missing = append(missing, concept)
		}
	}
	sort.Strings(missing)
	return missing
}

func presentJudgeRule60Concepts(text string, concepts []string) []string {
	normalized := normalizeReasonText(text)
	present := make([]string, 0)
	for _, concept := range nonemptyRule52Strings(concepts) {
		if judgeRule60ProhibitedConceptPresent(normalized, concept) {
			present = append(present, concept)
		}
	}
	sort.Strings(present)
	return present
}

func judgeRule60ConceptPresent(normalizedText string, concept string) bool {
	normalizedConcept := normalizeReasonText(concept)
	if strings.Contains(normalizedText, normalizedConcept) {
		return true
	}
	for _, alias := range judgeRule60ConceptAliases()[normalizedConcept] {
		if strings.Contains(normalizedText, alias) {
			return true
		}
	}
	return false
}

func judgeRule60ProhibitedConceptPresent(normalizedText string, concept string) bool {
	normalizedConcept := normalizeReasonText(concept)
	if phraseAppearsUnnegated(normalizedText, normalizedConcept) {
		return true
	}
	for _, alias := range judgeRule60ConceptAliases()[normalizedConcept] {
		if phraseAppearsUnnegated(normalizedText, alias) {
			return true
		}
	}
	return false
}

func phraseAppearsUnnegated(normalizedText string, phrase string) bool {
	phrase = normalizeReasonText(phrase)
	if phrase == "" {
		return false
	}
	for _, idx := range phraseIndexes(normalizedText, phrase) {
		if !phraseIsNegatedAt(normalizedText, idx) {
			return true
		}
	}
	return false
}

func phraseIndexes(text string, phrase string) []int {
	indexes := make([]int, 0)
	offset := 0
	for {
		idx := strings.Index(text[offset:], phrase)
		if idx < 0 {
			return indexes
		}
		absolute := offset + idx
		beforeOK := absolute == 0 || text[absolute-1] == ' '
		after := absolute + len(phrase)
		afterOK := after == len(text) || text[after] == ' '
		if beforeOK && afterOK {
			indexes = append(indexes, absolute)
		}
		offset = absolute + len(phrase)
	}
}

func phraseIsNegatedAt(text string, phraseStart int) bool {
	prefix := strings.TrimSpace(text[:phraseStart])
	if prefix == "" {
		return false
	}
	fields := strings.Fields(prefix)
	if len(fields) == 0 {
		return false
	}
	last := fields[len(fields)-1]
	if last == "no" || last == "not" || last == "without" || last == "requires" || last == "require" || last == "required" {
		return true
	}
	if len(fields) >= 2 {
		two := fields[len(fields)-2] + " " + fields[len(fields)-1]
		if two == "does not" || two == "did not" || two == "do not" {
			return true
		}
	}
	windowStart := len(fields) - 16
	if windowStart < 0 {
		windowStart = 0
	}
	window := strings.Join(fields[windowStart:], " ")
	for _, marker := range []string{
		"does not show",
		"does not establish",
		"does not constitute",
		"no clear and convincing showing",
		"no clear showing",
		"not clear and convincing evidence",
		"shows no",
		"no basis",
		"not a proper basis",
	} {
		if strings.Contains(window, marker) {
			return true
		}
	}
	suffixFields := strings.Fields(text[phraseStart:])
	if len(suffixFields) > 8 {
		suffixFields = suffixFields[:8]
	}
	suffix := strings.Join(suffixFields, " ")
	if strings.Contains(window, "no") && (strings.Contains(suffix, "are shown") || strings.Contains(suffix, "is shown")) {
		return true
	}
	if len(fields) >= 3 {
		three := fields[len(fields)-3] + " " + fields[len(fields)-2] + " " + fields[len(fields)-1]
		if three == "is not a" || three == "is not an" || three == "was not a" || three == "was not an" {
			return true
		}
	}
	return false
}

func matchedJudgeRule60ReasonTags(text string, expected []string) []string {
	normalized := normalizeReasonText(text)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if strings.Contains(normalized, normalizeReasonText(tag)) {
			matches = append(matches, tag)
			continue
		}
		for _, keyword := range judgeRule60ReasonTagKeywords()[tag] {
			if strings.Contains(normalized, keyword) {
				matches = append(matches, tag)
				break
			}
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeRule60ConceptAliases() map[string][]string {
	return map[string][]string{
		"excusable neglect":              {"service routing error", "service was misrouted", "misrouted service", "missed deadline because"},
		"prompt action":                  {"prompt corrective appearance", "acted promptly", "quickly moved", "appeared eight days", "appeared promptly", "prompt appearance"},
		"no meritorious defense":         {"does not identify a defense", "no defense to the claim"},
		"new evidence":                   {"newly discovered", "newly discovered evidence", "not available earlier", "unavailable despite", "recovered after judgment"},
		"not available earlier":          {"not reasonably obtainable before judgment", "unavailable despite", "unavailable despite diligence", "despite subpoenas", "could not have been found earlier", "recovered after judgment"},
		"could have been discovered":     {"available before judgment", "reasonable diligence", "known before judgment", "had the records", "cross examined on"},
		"fraud":                          {"fabricated", "misrepresentation"},
		"ordinary litigation argument":   {"ordinary trial arguments", "repeats trial arguments", "reargues trial evidence", "reweigh the trial evidence", "matters for appeal", "re argues", "re urges arguments", "relitigation", "relitigate witness credibility", "relitigation of credibility determinations", "relitigate the merits", "disagreement with the result", "impeachment only", "mere impeachment", "minor inconsistency", "substitute for appeal", "regret", "payment inconvenience", "known before judgment"},
		"void judgment":                  {"judgment is void", "lack of service", "no valid pre judgment service", "no personal jurisdiction", "lack of personal jurisdiction"},
		"lack of service":                {"lack of valid service", "no valid pre judgment service", "proper pre judgment service", "served on an unrelated address", "sent to an unrelated address", "no notice before default", "no notice before judgment"},
		"satisfied judgment":             {"judgment has been satisfied", "judgment satisfied", "mark the judgment satisfied", "satisfaction of judgment", "full satisfaction"},
		"paid in full":                   {"full payment", "paid in full", "payment was received in full", "acknowledgment of full payment", "filed acknowledgment", "judgment has been satisfied"},
		"untimely":                       {"filed fourteen months after", "outside the one year", "deadline is absolute", "one year limit"},
		"reasonable time":                {"timely", "filed promptly"},
		"extraordinary circumstances":    {"extraordinary circumstance", "extraordinary reason", "exceptional circumstances", "significant change in law", "changed legal circumstance", "post judgment higher court injunction", "unlawful and impossible to enforce", "prospective enforcement inequitable"},
		"no extraordinary circumstances": {"no extraordinary circumstance", "no extraordinary reason", "no intervening change", "no new law facts or events", "offers no new law", "shows only regret", "only regret", "payment inconvenience", "no basis to set aside"},
		"prospective enforcement":        {"prospective enforcement inequitable", "prospective reporting duties", "reporting obligations"},
		"mistake":                        {"amend the judgment to correct", "correct the judgment", "to conform to the verdict", "nunc pro tunc", "naming error"},
		"correct party name":             {"amend the judgment to correct", "correct the defendant", "correct the judgment to identify", "wrong party name", "naming error", "delta supplies llc", "not delta supply"},
	}
}

func judgeRule60ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"mistake_excusable_neglect": {"excusable neglect", "mistake", "service was misrouted", "amend the judgment", "correct the judgment", "nunc pro tunc", "naming error"},
		"new_evidence":              {"new evidence", "newly discovered", "could have been discovered", "available before judgment", "reasonable diligence", "unavailable despite diligence", "not reasonably obtainable before judgment", "material to the access issue", "likely to change"},
		"fraud":                     {"fraud", "fabricated", "misrepresentation"},
		"void_judgment":             {"void", "jurisdiction", "service"},
		"satisfied_judgment":        {"satisfied", "paid", "full payment", "full satisfaction"},
		"ordinary_reargument":       {"ordinary litigation argument", "ordinary trial arguments", "reargue", "re argues", "re urges", "relitigation", "relitigate", "reweigh", "matters for appeal", "disagreement", "impeachment only", "mere impeachment", "substitute for appeal", "regret", "known before judgment"},
		"timeliness":                {"timely", "untimely", "reasonable time", "one year"},
		"extraordinary":             {"extraordinary", "significant change in law", "changed legal circumstance", "post judgment higher court injunction", "prospective enforcement inequitable"},
	}
}

func judgeRule60DispositionKey(granted bool) string {
	if granted {
		return "granted"
	}
	return "denied"
}

func judgeRule60ActionPayload(applyResp map[string]any, fallback map[string]any) map[string]any {
	action, _ := applyResp["action"].(map[string]any)
	if action == nil {
		return fallback
	}
	payload, _ := action["payload"].(map[string]any)
	if payload == nil {
		return fallback
	}
	return payload
}
