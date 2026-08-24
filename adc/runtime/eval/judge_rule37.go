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

const JudgeRule37Tool = "decide_rule37_motion"

type JudgeRule37Fixture struct {
	ID                     string   `json:"id"`
	Tier                   int      `json:"tier"`
	IssueFamily            string   `json:"issue_family"`
	CaseTheme              string   `json:"case_theme"`
	Movant                 string   `json:"movant"`
	TargetParty            string   `json:"target_party"`
	DiscoveryType          string   `json:"discovery_type"`
	SetIndex               int      `json:"set_index"`
	RequestText            string   `json:"request_text"`
	ResponseText           string   `json:"response_text"`
	MeetAndConferText      string   `json:"meet_and_confer_text,omitempty"`
	MotionText             string   `json:"motion_text"`
	OppositionText         string   `json:"opposition_text"`
	ReplyText              string   `json:"reply_text,omitempty"`
	ExpectedGranted        bool     `json:"expected_granted"`
	ExpectedSanctionType   string   `json:"expected_sanction_type"`
	ExpectedSanctionAmount float64  `json:"expected_sanction_amount,omitempty"`
	ExpectedReasonTags     []string `json:"expected_reason_tags"`
	Severity               float64  `json:"severity"`
	ContextNotes           string   `json:"context_notes,omitempty"`
}

type JudgeRule37Options struct {
	FixturesPath          string
	OutputDir             string
	OpportunityPromptPath string
	OpportunityPromptName string
	Engine                lean.Engine
	Model                 string
	Online                bool
	Limit                 int
	Timeout               time.Duration
	Temperature           *float64
	PromptDir             string
	PromptFiles           map[string]string
	Court                 courts.Profile
	CounterfactualModel   bool
}

type JudgeRule37RescoreOptions struct {
	ResultsPath string
	OutputDir   string
}

type JudgeRule37Summary struct {
	Evaluation          string                      `json:"evaluation"`
	Model               string                      `json:"model"`
	ExecutionMode       string                      `json:"execution_mode"`
	CounterfactualModel bool                        `json:"counterfactual_model"`
	PromptSource        string                      `json:"prompt_source"`
	PromptName          string                      `json:"prompt_name"`
	PromptPath          string                      `json:"prompt_path,omitempty"`
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
	ByReasonTag         map[string]JudgeRule37Slice `json:"by_reason_tag"`
	ByIssueFamily       map[string]JudgeRule37Slice `json:"by_issue_family"`
	ByTier              map[string]JudgeRule37Slice `json:"by_tier"`
	ByMovant            map[string]JudgeRule37Slice `json:"by_movant"`
	ByExpectedSanction  map[string]JudgeRule37Slice `json:"by_expected_sanction"`
	GeneratedAt         string                      `json:"generated_at"`
}

type JudgeRule37Slice struct {
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

type JudgeRule37Result struct {
	ID                     string            `json:"id"`
	Tier                   int               `json:"tier"`
	IssueFamily            string            `json:"issue_family"`
	CaseTheme              string            `json:"case_theme"`
	Movant                 string            `json:"movant"`
	TargetParty            string            `json:"target_party"`
	DiscoveryType          string            `json:"discovery_type"`
	SetIndex               int               `json:"set_index"`
	RequestText            string            `json:"request_text"`
	ResponseText           string            `json:"response_text"`
	MeetAndConferText      string            `json:"meet_and_confer_text,omitempty"`
	MotionText             string            `json:"motion_text"`
	OppositionText         string            `json:"opposition_text"`
	ReplyText              string            `json:"reply_text,omitempty"`
	ExpectedGranted        bool              `json:"expected_granted"`
	ExpectedSanctionType   string            `json:"expected_sanction_type"`
	ExpectedSanctionAmount float64           `json:"expected_sanction_amount,omitempty"`
	ExpectedReasonTags     []string          `json:"expected_reason_tags"`
	Severity               float64           `json:"severity"`
	ContextNotes           string            `json:"context_notes,omitempty"`
	Model                  string            `json:"model"`
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
	OrderText              string            `json:"order_text,omitempty"`
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

type judgeRule37PromptVariant struct {
	Source   string
	Name     string
	Path     string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeRule37(ctx context.Context, opts JudgeRule37Options) (resultValue JudgeRule37Summary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule37Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule37Summary{}, fmt.Errorf("output directory is required")
	}
	if strings.TrimSpace(opts.OpportunityPromptPath) != "" && !opts.CounterfactualModel {
		return JudgeRule37Summary{}, fmt.Errorf("opportunity prompt file requires counterfactual model mode")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeRule37Summary{}, err
	}
	fixtures, err := LoadJudgeRule37Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule37Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule37Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if opts.CounterfactualModel {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule37Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule37Summary{}, err
		}
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule37Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule37PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName)
	if err != nil {
		return JudgeRule37Summary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeRule37Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule37Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := newJudgeRule37Summary(opts, promptVariant, resultsPath, summaryPath)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule37Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule37Summary{}, err
		}
		if err := encoder.Encode(result); err != nil {
			return JudgeRule37Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule37SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule37Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule37Summary{}, err
	}
	return summary, nil
}

func RescoreJudgeRule37(opts JudgeRule37RescoreOptions) (resultValue JudgeRule37Summary, returnErr error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeRule37Summary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule37Summary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeRule37Results(opts.ResultsPath)
	if err != nil {
		return JudgeRule37Summary{}, err
	}
	if len(results) == 0 {
		return JudgeRule37Summary{}, fmt.Errorf("no results loaded from %s", opts.ResultsPath)
	}
	if err := validateJudgeEvalRescoreOutputPath(opts.ResultsPath, opts.OutputDir); err != nil {
		return JudgeRule37Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule37Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule37Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule37Summary{
		Evaluation:          "judge_rule37",
		Model:               results[0].Model,
		ExecutionMode:       results[0].ExecutionMode,
		CounterfactualModel: results[0].CounterfactualModel,
		PromptSource:        resultJudgeRule37PromptSource(results[0]),
		PromptName:          resultJudgeRule37PromptName(results[0]),
		PromptPath:          results[0].PromptPath,
		FixturesPath:        "rescored from " + opts.ResultsPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeRule37Slice{},
		ByIssueFamily:       map[string]JudgeRule37Slice{},
		ByTier:              map[string]JudgeRule37Slice{},
		ByMovant:            map[string]JudgeRule37Slice{},
		ByExpectedSanction:  map[string]JudgeRule37Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeRule37Result(&result)
		result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted
		if err := encoder.Encode(result); err != nil {
			return JudgeRule37Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule37SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule37Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule37Summary{}, err
	}
	return summary, nil
}

func newJudgeRule37Summary(opts JudgeRule37Options, promptVariant judgeRule37PromptVariant, resultsPath string, summaryPath string) JudgeRule37Summary {
	return JudgeRule37Summary{
		Evaluation:          "judge_rule37",
		Model:               opts.Model,
		ExecutionMode:       judgeEvalExecutionMode(opts.CounterfactualModel),
		CounterfactualModel: opts.CounterfactualModel,
		PromptSource:        promptVariant.Source,
		PromptName:          promptVariant.Name,
		PromptPath:          promptVariant.Path,
		FixturesPath:        opts.FixturesPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeRule37Slice{},
		ByIssueFamily:       map[string]JudgeRule37Slice{},
		ByTier:              map[string]JudgeRule37Slice{},
		ByMovant:            map[string]JudgeRule37Slice{},
		ByExpectedSanction:  map[string]JudgeRule37Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeRule37Fixture(
	ctx context.Context,
	opts JudgeRule37Options,
	promptVariant judgeRule37PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule37Fixture,
) (JudgeRule37Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule37Result{}, err
	}
	state := BuildJudgeRule37State(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeRule37Roles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeRule37Tool)
	if err != nil {
		return JudgeRule37Result{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
	}
	executionModel := opts.Model
	if opts.CounterfactualModel {
		executionModel = modelRef.Model
	}
	callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	execution, executionErr := executeJudgeOpportunity(callCtx, judgeOpportunityExecutionOptions{
		Engine:                     opts.Engine,
		State:                      state,
		Roles:                      roles,
		RolesPayload:               rolesPayload,
		Client:                     client,
		Court:                      opts.Court,
		Model:                      executionModel,
		Temperature:                opts.Temperature,
		PromptDir:                  opts.PromptDir,
		PromptFiles:                opts.PromptFiles,
		CounterfactualModel:        opts.CounterfactualModel,
		RequireDeterministicAction: true,
		Objective: func(opportunity map[string]any) (string, error) {
			if strings.TrimSpace(promptVariant.Text) == "" {
				return stringField(opportunity, "objective"), nil
			}
			return renderJudgeRule37PromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeRule37Tool, executionErr)
	if decisionErr != nil {
		return JudgeRule37Result{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeRule37Response(fixture, opts.Model, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
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
	leanAccepted, stepAccepted := judgeEvalExecutionAcceptance(execution, JudgeRule37Tool)
	result.LeanAccepted = leanAccepted
	result.StepAccepted = stepAccepted
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && stepAccepted && executionErr == nil
	return result, nil
}

func LoadJudgeRule37Fixtures(path string) (resultValue []JudgeRule37Fixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule37Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule37Fixture
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

func readJudgeRule37Results(path string) (resultValue []JudgeRule37Result, returnErr error) {
	return readJudgeEvalJSONL[JudgeRule37Result](path)
}

func loadJudgeRule37PromptVariant(path string, name string) (judgeRule37PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule37PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule37PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule37PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	return judgeRule37PromptVariant{Source: "file:" + path, Name: name, Path: path, Text: text}, nil
}

func (f JudgeRule37Fixture) Validate() error {
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
	if !validJudgeRule37DiscoveryType(f.DiscoveryType) {
		return fmt.Errorf("fixture %s invalid discovery_type %q", f.ID, f.DiscoveryType)
	}
	if f.SetIndex < 0 {
		return fmt.Errorf("fixture %s set_index must be nonnegative", f.ID)
	}
	if strings.TrimSpace(f.RequestText) == "" {
		return fmt.Errorf("fixture %s missing request_text", f.ID)
	}
	if strings.TrimSpace(f.ResponseText) == "" {
		return fmt.Errorf("fixture %s missing response_text", f.ID)
	}
	if strings.TrimSpace(f.MotionText) == "" {
		return fmt.Errorf("fixture %s missing motion_text", f.ID)
	}
	if strings.TrimSpace(f.OppositionText) == "" {
		return fmt.Errorf("fixture %s missing opposition_text", f.ID)
	}
	if !validJudgeRule37SanctionType(f.ExpectedSanctionType) {
		return fmt.Errorf("fixture %s invalid expected_sanction_type %q", f.ID, f.ExpectedSanctionType)
	}
	if f.ExpectedSanctionType == "fees" && f.ExpectedSanctionAmount <= 0 {
		return fmt.Errorf("fixture %s fees expected_sanction_amount must be positive", f.ID)
	}
	if f.ExpectedSanctionType == "none" && f.ExpectedSanctionAmount != 0 {
		return fmt.Errorf("fixture %s none expected_sanction_amount must be zero", f.ID)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule37State(f JudgeRule37Fixture) map[string]any {
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
			"case_id":                       "judge-rule37-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "pretrial",
			"resolution":                    "pending",
			"trial_mode":                    "jury",
			"phase":                         "discovery",
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
			"docket":                        judgeRule37Docket(f, movant, target),
			"decision_traces": []any{
				map[string]any{"action": "serve_discovery", "outcome": f.DiscoveryType, "citations": []any{"FRCP 26", "FRCP 33", "FRCP 34", "FRCP 36"}},
				map[string]any{"action": "file_rule37_motion", "outcome": movant + ":requested:" + target, "citations": []any{"FRCP 37(a)"}},
			},
		},
	}
}

func judgeRule37Docket(f JudgeRule37Fixture, movant string, target string) []any {
	entries := []any{
		map[string]any{"title": "Complaint", "description": "plaintiff: " + strings.TrimSpace(f.CaseTheme)},
		map[string]any{"title": "Answer", "description": "defendant: denies liability and demands proof."},
		map[string]any{"title": "Discovery Request", "description": movant + " served " + strings.TrimSpace(f.DiscoveryType) + " set_index " + strconv.Itoa(f.SetIndex) + ": " + strings.TrimSpace(f.RequestText)},
		map[string]any{"title": "Discovery Response", "description": target + ": " + strings.TrimSpace(f.ResponseText)},
	}
	if strings.TrimSpace(f.MeetAndConferText) != "" {
		entries = append(entries, map[string]any{"title": "Rule 37 Meet and Confer", "description": strings.TrimSpace(f.MeetAndConferText)})
	}
	entries = append(entries, map[string]any{"title": "Rule 37 Motion", "description": movant + ": " + strings.TrimSpace(f.MotionText)})
	entries = append(entries, map[string]any{"title": "Rule 37 Opposition", "description": target + ": " + strings.TrimSpace(f.OppositionText)})
	if strings.TrimSpace(f.ReplyText) != "" {
		entries = append(entries, map[string]any{"title": "Rule 37 Reply", "description": movant + ": " + strings.TrimSpace(f.ReplyText)})
	}
	return entries
}

func buildJudgeRule37Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule37Fixture,
	promptVariant judgeRule37PromptVariant,
) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeRule37PromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeRule37Tool, view, opportunity, objective)
}

func scoreJudgeRule37Response(
	fixture JudgeRule37Fixture,
	model string,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule37Result {
	result := JudgeRule37Result{
		ID:                     fixture.ID,
		Tier:                   fixture.Tier,
		IssueFamily:            strings.TrimSpace(fixture.IssueFamily),
		CaseTheme:              strings.TrimSpace(fixture.CaseTheme),
		Movant:                 normalizeParty(fixture.Movant),
		TargetParty:            normalizeParty(fixture.TargetParty),
		DiscoveryType:          strings.TrimSpace(fixture.DiscoveryType),
		SetIndex:               fixture.SetIndex,
		RequestText:            strings.TrimSpace(fixture.RequestText),
		ResponseText:           strings.TrimSpace(fixture.ResponseText),
		MeetAndConferText:      strings.TrimSpace(fixture.MeetAndConferText),
		MotionText:             strings.TrimSpace(fixture.MotionText),
		OppositionText:         strings.TrimSpace(fixture.OppositionText),
		ReplyText:              strings.TrimSpace(fixture.ReplyText),
		ExpectedGranted:        fixture.ExpectedGranted,
		ExpectedSanctionType:   strings.TrimSpace(fixture.ExpectedSanctionType),
		ExpectedSanctionAmount: fixture.ExpectedSanctionAmount,
		ExpectedReasonTags:     append([]string{}, fixture.ExpectedReasonTags...),
		Severity:               normalizedSeverity(fixture.Severity),
		ContextNotes:           strings.TrimSpace(fixture.ContextNotes),
		Model:                  model,
		State:                  state,
		View:                   view,
		Opportunity:            opportunity,
		Input:                  input,
		RawResponse:            responseJSON(resp),
	}
	payload, invalid := extractJudgeRule37Payload(resp)
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
	result.SanctionType = strings.TrimSpace(stringField(payload, "sanction_type"))
	if !validJudgeRule37SanctionType(result.SanctionType) {
		result.InvalidReason = "invalid_sanction_type"
		return result
	}
	amount, hasAmount, amountOK := optionalFloatField(payload, "sanction_amount")
	if !amountOK {
		result.InvalidReason = "malformed_sanction_amount"
		return result
	}
	if hasAmount {
		result.SanctionAmount = &amount
	}
	if !granted && result.SanctionType != "none" {
		result.InvalidReason = "denied_with_sanction"
		return result
	}
	if result.SanctionType == "none" && hasAmount && amount != 0 {
		result.InvalidReason = "none_with_sanction_amount"
		return result
	}
	if result.SanctionType == "fees" {
		if !hasAmount {
			result.InvalidReason = "fees_missing_amount"
			return result
		}
		if amount <= 0 {
			result.InvalidReason = "fees_nonpositive_amount"
			return result
		}
	}
	result.OrderText = strings.TrimSpace(stringField(payload, "order_text"))
	result.Reasoning = strings.TrimSpace(stringField(payload, "reasoning"))
	if result.Reasoning == "" {
		result.InvalidReason = "empty_reasoning"
		return result
	}
	rescoreJudgeRule37Result(&result)
	return result
}

func rescoreJudgeRule37Result(result *JudgeRule37Result) {
	if result == nil || result.InvalidReason != "" || result.Granted == nil {
		return
	}
	result.GrantCorrect = *result.Granted == result.ExpectedGranted
	result.SanctionCorrect = judgeRule37SanctionCorrect(*result)
	result.OutcomeCorrect = result.GrantCorrect && result.SanctionCorrect
	result.MatchedReasonTags = matchedJudgeRule37ReasonTags(result.Reasoning+" "+result.OrderText, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeRule37Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule37Tool {
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

func renderJudgeRule37PromptTemplate(template string, fixture JudgeRule37Fixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeRule37PromptTemplate", template,
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
		"{{discovery_type}}", strings.TrimSpace(fixture.DiscoveryType),
		"{{set_index}}", strconv.Itoa(fixture.SetIndex),
		"{{request_text}}", strings.TrimSpace(fixture.RequestText),
		"{{response_text}}", strings.TrimSpace(fixture.ResponseText),
		"{{meet_and_confer_text}}", strings.TrimSpace(fixture.MeetAndConferText),
		"{{motion_text}}", strings.TrimSpace(fixture.MotionText),
		"{{opposition_text}}", strings.TrimSpace(fixture.OppositionText),
		"{{reply_text}}", strings.TrimSpace(fixture.ReplyText),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeRule37Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule37Tool}}}
}

func applyJudgeRule37SummaryResult(summary *JudgeRule37Summary, result JudgeRule37Result, weight float64) {
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
			classifyJudgeRule37Error(result, &summary.FalseGrants, &summary.FalseDenials)
		}
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateJudgeRule37Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeRule37Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeRule37Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeRule37Slice(summary.ByMovant, result.Movant, result, weight)
	updateJudgeRule37Slice(summary.ByExpectedSanction, result.ExpectedSanctionType, result, weight)
}

func updateJudgeRule37Slice(m map[string]JudgeRule37Slice, key string, result JudgeRule37Result, weight float64) {
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
			classifyJudgeRule37Error(result, &s.FalseGrants, &s.FalseDenials)
		}
	}
	m[key] = s
}

func classifyJudgeRule37Error(result JudgeRule37Result, falseGrants *int, falseDenials *int) {
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

func finalizeJudgeRule37Summary(summary *JudgeRule37Summary, totalWeight float64, correctWeight float64) {
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
	finalizeJudgeRule37Slices(summary.ByReasonTag)
	finalizeJudgeRule37Slices(summary.ByIssueFamily)
	finalizeJudgeRule37Slices(summary.ByTier)
	finalizeJudgeRule37Slices(summary.ByMovant)
	finalizeJudgeRule37Slices(summary.ByExpectedSanction)
}

func finalizeJudgeRule37Slices(m map[string]JudgeRule37Slice) {
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

func judgeRule37SanctionCorrect(result JudgeRule37Result) bool {
	if strings.TrimSpace(result.SanctionType) != strings.TrimSpace(result.ExpectedSanctionType) {
		return false
	}
	if result.ExpectedSanctionType == "none" {
		return result.SanctionAmount == nil || math.Abs(*result.SanctionAmount) <= 0.01
	}
	if result.ExpectedSanctionType == "fees" {
		if result.SanctionAmount == nil {
			return false
		}
		return math.Abs(*result.SanctionAmount-result.ExpectedSanctionAmount) <= 0.01
	}
	return false
}

func matchedJudgeRule37ReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if judgeRule37ReasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeRule37ReasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range judgeRule37ReasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func judgeRule37ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"no_response":                {"no response", "failed to respond", "never responded", "remains incomplete"},
		"complete_response":          {"complete response", "complete non evasive", "answer is complete", "complete for the request", "provided the identities", "fully answered", "already answered", "existing answer", "sufficient", "no further response", "produced all", "no withholding", "no unresolved"},
		"evasive_incomplete":         {"evasive", "incomplete", "nonresponsive", "missing responsive"},
		"justified_objection":        {"justified objection", "substantially justified", "privilege", "work product"},
		"overbroad_request":          {"overbroad", "unduly broad", "not proportional", "sweeping"},
		"proportionality":            {"proportional", "burden", "expense", "narrower request"},
		"harmless_cure":              {"harmless", "cured", "supplemented", "no prejudice", "before the motion", "production preceded"},
		"order_violation":            {"violated", "failed to obey", "prior order", "court order"},
		"disclosure_failure":         {"initial disclosure", "disclose", "identify witness", "rule 26"},
		"rfa_nonresponse":            {"request for admission", "rfa", "admit", "nonresponse"},
		"rfa_deemed_admitted":        {"deemed admitted", "matter is admitted", "matters are admitted", "rule 36"},
		"rfp_failure":                {"request for production", "production", "documents", "files"},
		"premature_motion":           {"premature", "deadline", "not expired", "time to respond"},
		"fees":                       {"fees", "expenses", "fee award", "costs"},
		"no_fees_substantial_reason": {"no fees", "without fees", "substantially justified", "award unjust"},
	}
}

func validJudgeRule37DiscoveryType(value string) bool {
	switch strings.TrimSpace(value) {
	case "interrogatories", "rfp", "rfa", "initial_disclosures":
		return true
	default:
		return false
	}
}

func validJudgeRule37SanctionType(value string) bool {
	switch strings.TrimSpace(value) {
	case "none", "fees":
		return true
	default:
		return false
	}
}

func optionalFloatField(m map[string]any, key string) (float64, bool, bool) {
	value, ok := m[key]
	if !ok || value == nil {
		return 0, false, true
	}
	switch v := value.(type) {
	case float64:
		return v, true, true
	case float32:
		return float64(v), true, true
	case int:
		return float64(v), true, true
	case int64:
		return float64(v), true, true
	case json.Number:
		n, err := v.Float64()
		if err != nil {
			return 0, true, false
		}
		return n, true, true
	default:
		return 0, true, false
	}
}

func resultJudgeRule37PromptSource(result JudgeRule37Result) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultJudgeRule37PromptName(result JudgeRule37Result) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}
