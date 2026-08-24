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

const JudgeForCauseTool = "decide_juror_for_cause_challenge"

type JudgeForCauseFixture struct {
	ID                 string   `json:"id"`
	Tier               int      `json:"tier"`
	IssueFamily        string   `json:"issue_family"`
	CaseTheme          string   `json:"case_theme"`
	ChallengedBy       string   `json:"challenged_by"`
	JurorID            string   `json:"juror_id"`
	VoirDireRecord     string   `json:"voir_dire_record"`
	ChallengeGrounds   string   `json:"challenge_grounds"`
	ExpectedGranted    bool     `json:"expected_granted"`
	ExpectedReasonTags []string `json:"expected_reason_tags"`
	Severity           float64  `json:"severity"`
	ContextNotes       string   `json:"context_notes,omitempty"`
}

type JudgeForCauseOptions struct {
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
}

type JudgeForCauseRescoreOptions struct {
	ResultsPath string
	OutputDir   string
}

type JudgeForCauseSummary struct {
	Evaluation          string                        `json:"evaluation"`
	Model               string                        `json:"model"`
	ExecutionMode       string                        `json:"execution_mode"`
	CounterfactualModel bool                          `json:"counterfactual_model"`
	PromptSource        string                        `json:"prompt_source"`
	PromptName          string                        `json:"prompt_name"`
	PromptPath          string                        `json:"prompt_path,omitempty"`
	FixturesPath        string                        `json:"fixtures_path"`
	OutputDir           string                        `json:"output_dir"`
	ResultsPath         string                        `json:"results_path"`
	SummaryPath         string                        `json:"summary_path"`
	Total               int                           `json:"total"`
	Correct             int                           `json:"correct"`
	ReasonCorrect       int                           `json:"reason_correct"`
	Invalid             int                           `json:"invalid"`
	FalseGrants         int                           `json:"false_grants"`
	FalseDenials        int                           `json:"false_denials"`
	Accuracy            float64                       `json:"accuracy"`
	WeightedAccuracy    float64                       `json:"weighted_accuracy"`
	FalseGrantRate      float64                       `json:"false_grant_rate"`
	FalseDenialRate     float64                       `json:"false_denial_rate"`
	InvalidRate         float64                       `json:"invalid_rate"`
	ByReasonTag         map[string]JudgeForCauseSlice `json:"by_reason_tag"`
	ByIssueFamily       map[string]JudgeForCauseSlice `json:"by_issue_family"`
	ByTier              map[string]JudgeForCauseSlice `json:"by_tier"`
	ByChallengedBy      map[string]JudgeForCauseSlice `json:"by_challenged_by"`
	GeneratedAt         string                        `json:"generated_at"`
}

type JudgeForCauseSlice struct {
	Total            int     `json:"total"`
	Correct          int     `json:"correct"`
	FalseGrants      int     `json:"false_grants"`
	FalseDenials     int     `json:"false_denials"`
	Invalid          int     `json:"invalid"`
	Weight           float64 `json:"weight"`
	CorrectWeight    float64 `json:"correct_weight"`
	Accuracy         float64 `json:"accuracy"`
	WeightedAccuracy float64 `json:"weighted_accuracy"`
}

type JudgeForCauseResult struct {
	ID                  string            `json:"id"`
	Tier                int               `json:"tier"`
	IssueFamily         string            `json:"issue_family"`
	CaseTheme           string            `json:"case_theme"`
	ChallengedBy        string            `json:"challenged_by"`
	JurorID             string            `json:"juror_id"`
	VoirDireRecord      string            `json:"voir_dire_record"`
	ChallengeGrounds    string            `json:"challenge_grounds"`
	ExpectedGranted     bool              `json:"expected_granted"`
	ExpectedReasonTags  []string          `json:"expected_reason_tags"`
	Severity            float64           `json:"severity"`
	ContextNotes        string            `json:"context_notes,omitempty"`
	Model               string            `json:"model"`
	ExecutionMode       string            `json:"execution_mode"`
	CounterfactualModel bool              `json:"counterfactual_model"`
	PromptSource        string            `json:"prompt_source"`
	PromptName          string            `json:"prompt_name"`
	PromptPath          string            `json:"prompt_path,omitempty"`
	State               map[string]any    `json:"state"`
	View                map[string]any    `json:"view"`
	Opportunity         map[string]any    `json:"opportunity"`
	Input               []map[string]any  `json:"input"`
	RawResponse         map[string]any    `json:"raw_response"`
	ResponseExchanges   []map[string]any  `json:"response_exchanges,omitempty"`
	TurnLog             runner.TurnLog    `json:"turn_log"`
	FinalState          map[string]any    `json:"final_state,omitempty"`
	Provider            openai.Accounting `json:"provider"`
	ToolPayload         map[string]any    `json:"tool_payload,omitempty"`
	ChallengeID         string            `json:"challenge_id,omitempty"`
	Granted             *bool             `json:"granted,omitempty"`
	RulingReason        string            `json:"ruling_reason,omitempty"`
	MatchedReasonTags   []string          `json:"matched_reason_tags"`
	OutcomeCorrect      bool              `json:"outcome_correct"`
	ReasonCorrect       bool              `json:"reason_correct"`
	InvalidReason       string            `json:"invalid_reason,omitempty"`
	LeanAccepted        bool              `json:"lean_accepted"`
	StepAccepted        bool              `json:"step_accepted"`
	LeanError           string            `json:"lean_error,omitempty"`
}

type judgeForCausePromptVariant struct {
	Source   string
	Name     string
	Path     string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeForCause(ctx context.Context, opts JudgeForCauseOptions) (resultValue JudgeForCauseSummary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	fixtures, err := LoadJudgeForCauseFixtures(opts.FixturesPath)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeForCauseSummary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef, err := modelrequest.ParseModelRef(opts.Model)
	if err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("parse --model: %w", err)
	}
	client, err := openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeForCausePromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := newJudgeForCauseSummary(opts, promptVariant, resultsPath, summaryPath)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeForCauseFixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeForCauseSummary{}, err
		}
		result.ExecutionMode = summary.ExecutionMode
		result.CounterfactualModel = summary.CounterfactualModel
		if err := encoder.Encode(result); err != nil {
			return JudgeForCauseSummary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeForCauseSummaryResult(&summary, result, weight)
	}
	finalizeJudgeForCauseSummary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeForCauseSummary{}, err
	}
	return summary, nil
}

func RescoreJudgeForCause(opts JudgeForCauseRescoreOptions) (resultValue JudgeForCauseSummary, returnErr error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeForCauseResults(opts.ResultsPath)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	if len(results) == 0 {
		return JudgeForCauseSummary{}, fmt.Errorf("no results loaded from %s", opts.ResultsPath)
	}
	if err := validateJudgeEvalRescoreOutputPath(opts.ResultsPath, opts.OutputDir); err != nil {
		return JudgeForCauseSummary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeForCauseSummary{
		Evaluation:          "judge_for_cause",
		Model:               results[0].Model,
		ExecutionMode:       results[0].ExecutionMode,
		CounterfactualModel: results[0].CounterfactualModel,
		PromptSource:        resultJudgeForCausePromptSource(results[0]),
		PromptName:          resultJudgeForCausePromptName(results[0]),
		PromptPath:          results[0].PromptPath,
		FixturesPath:        "rescored from " + opts.ResultsPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeForCauseSlice{},
		ByIssueFamily:       map[string]JudgeForCauseSlice{},
		ByTier:              map[string]JudgeForCauseSlice{},
		ByChallengedBy:      map[string]JudgeForCauseSlice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeForCauseResult(&result)
		result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted
		if err := encoder.Encode(result); err != nil {
			return JudgeForCauseSummary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeForCauseSummaryResult(&summary, result, weight)
	}
	finalizeJudgeForCauseSummary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeForCauseSummary{}, err
	}
	return summary, nil
}

func newJudgeForCauseSummary(opts JudgeForCauseOptions, promptVariant judgeForCausePromptVariant, resultsPath string, summaryPath string) JudgeForCauseSummary {
	return JudgeForCauseSummary{
		Evaluation:          "judge_for_cause",
		Model:               opts.Model,
		ExecutionMode:       "production",
		CounterfactualModel: false,
		PromptSource:        promptVariant.Source,
		PromptName:          promptVariant.Name,
		PromptPath:          promptVariant.Path,
		FixturesPath:        opts.FixturesPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeForCauseSlice{},
		ByIssueFamily:       map[string]JudgeForCauseSlice{},
		ByTier:              map[string]JudgeForCauseSlice{},
		ByChallengedBy:      map[string]JudgeForCauseSlice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeForCauseFixture(
	ctx context.Context,
	opts JudgeForCauseOptions,
	promptVariant judgeForCausePromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeForCauseFixture,
) (JudgeForCauseResult, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeForCauseResult{}, err
	}
	state := BuildJudgeForCauseState(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeForCauseRoles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeForCauseTool)
	if err != nil {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
	}
	callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	execution, executionErr := executeJudgeOpportunity(callCtx, judgeOpportunityExecutionOptions{
		Engine:       opts.Engine,
		State:        state,
		Roles:        roles,
		RolesPayload: rolesPayload,
		Client:       client,
		Court:        opts.Court,
		Model:        modelRef.Model,
		Temperature:  opts.Temperature,
		PromptDir:    opts.PromptDir,
		PromptFiles:  opts.PromptFiles,
		Objective: func(opportunity map[string]any) (string, error) {
			if strings.TrimSpace(promptVariant.Text) == "" {
				return stringField(opportunity, "objective"), nil
			}
			return renderJudgeForCausePromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeForCauseTool, executionErr)
	if decisionErr != nil {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeForCauseResponse(fixture, opts.Model, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
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
	result.LeanAccepted, result.StepAccepted = judgeEvalExecutionAcceptance(execution, JudgeForCauseTool)
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted && executionErr == nil
	return result, nil
}

func LoadJudgeForCauseFixtures(path string) (resultValue []JudgeForCauseFixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeForCauseFixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeForCauseFixture
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

func readJudgeForCauseResults(path string) (resultValue []JudgeForCauseResult, returnErr error) {
	return readJudgeEvalJSONL[JudgeForCauseResult](path)
}

func loadJudgeForCausePromptVariant(path string, name string) (judgeForCausePromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeForCausePromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeForCausePromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeForCausePromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	return judgeForCausePromptVariant{Source: "file:" + path, Name: name, Path: path, Text: text}, nil
}

func (f JudgeForCauseFixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if normalizeParty(f.ChallengedBy) == "" {
		return fmt.Errorf("fixture %s invalid challenged_by %q", f.ID, f.ChallengedBy)
	}
	if strings.TrimSpace(f.JurorID) == "" {
		return fmt.Errorf("fixture %s missing juror_id", f.ID)
	}
	if strings.TrimSpace(f.VoirDireRecord) == "" {
		return fmt.Errorf("fixture %s missing voir_dire_record", f.ID)
	}
	if strings.TrimSpace(f.ChallengeGrounds) == "" {
		return fmt.Errorf("fixture %s missing challenge_grounds", f.ID)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeForCauseState(f JudgeForCauseFixture) map[string]any {
	jurorID := strings.TrimSpace(f.JurorID)
	byParty := normalizeParty(f.ChallengedBy)
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeVoirDirePolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-for-cause-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "trial",
			"resolution":                    "pending",
			"trial_mode":                    "jury",
			"phase":                         "voir_dire",
			"last_pleading_served_on":       "2026-07-01",
			"jury_demanded_on":              "2026-07-01",
			"jury_configuration":            map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6},
			"single_claim":                  defaultJudgeVoirDireClaim(),
			"jurisdictional_allegations":    nil,
			"jurors":                        []any{map[string]any{"juror_id": jurorID, "name": "Juror " + jurorID, "status": "candidate", "note": "", "model": "eval-model", "persona_filename": "eval-persona"}},
			"juror_questionnaire":           []any{map[string]any{"question_id": "q1", "question": "Can you follow the court's instructions and decide the case from the record?"}},
			"juror_questionnaire_responses": []any{map[string]any{"juror_id": jurorID, "submitted_at": "2026-07-14", "answers": []any{map[string]any{"question_id": "q1", "answer": strings.TrimSpace(f.VoirDireRecord)}}}},
			"voir_dire_exchanges": []any{map[string]any{
				"exchange_id":   "vx-1",
				"juror_id":      jurorID,
				"asked_by":      byParty,
				"question":      "Counsel explored whether this candidate can follow the law and decide from the record.",
				"judge_allowed": true,
				"ruling_reason": "Permitted bias and qualification inquiry.",
				"response":      strings.TrimSpace(f.VoirDireRecord),
				"asked_at":      "2026-07-14",
				"ruled_at":      "2026-07-14",
				"answered_at":   "2026-07-14",
			}},
			"for_cause_challenges": []any{map[string]any{
				"challenge_id":  "fc-1",
				"juror_id":      jurorID,
				"by_party":      byParty,
				"grounds":       strings.TrimSpace(f.ChallengeGrounds),
				"requested_at":  "2026-07-14",
				"granted":       nil,
				"decided_at":    nil,
				"ruling_reason": "",
			}},
			"deliberation_round":       1,
			"juror_votes":              []any{},
			"jury_verdict":             nil,
			"hung_jury":                nil,
			"contempt_counts":          []any{},
			"protective_orders":        []any{},
			"bench_findings":           []any{},
			"bench_conclusions":        []any{},
			"juror_explanations":       []any{},
			"local_rule_overrides":     []any{},
			"limit_usage":              []any{},
			"rule56_window_closed_for": []any{},
			"case_files":               []any{},
			"file_events":              []any{},
			"rule68_offers":            []any{},
			"technical_reports":        []any{},
			"monetary_judgment":        0.0,
			"docket": []any{
				map[string]any{"title": "Voir dire answer", "description": jurorID + ": " + strings.TrimSpace(f.VoirDireRecord)},
				map[string]any{"title": "For-cause challenge requested", "description": byParty + " requested excusal of " + jurorID + ": " + strings.TrimSpace(f.ChallengeGrounds)},
			},
			"decision_traces": []any{
				map[string]any{"action": "answer_voir_dire_question", "outcome": jurorID, "citations": []any{"FRCP 47(a)"}},
				map[string]any{"action": "challenge_juror_for_cause", "outcome": byParty + ":requested:" + jurorID, "citations": []any{"FRCP 47(a)"}},
			},
		},
	}
}

func buildJudgeForCauseInput(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeForCauseFixture,
	promptVariant judgeForCausePromptVariant,
) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeForCausePromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeForCauseTool, view, opportunity, objective)
}

func scoreJudgeForCauseResponse(
	fixture JudgeForCauseFixture,
	model string,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeForCauseResult {
	result := JudgeForCauseResult{
		ID:                 fixture.ID,
		Tier:               fixture.Tier,
		IssueFamily:        fixture.IssueFamily,
		CaseTheme:          strings.TrimSpace(fixture.CaseTheme),
		ChallengedBy:       normalizeParty(fixture.ChallengedBy),
		JurorID:            strings.TrimSpace(fixture.JurorID),
		VoirDireRecord:     strings.TrimSpace(fixture.VoirDireRecord),
		ChallengeGrounds:   strings.TrimSpace(fixture.ChallengeGrounds),
		ExpectedGranted:    fixture.ExpectedGranted,
		ExpectedReasonTags: append([]string{}, fixture.ExpectedReasonTags...),
		Severity:           normalizedSeverity(fixture.Severity),
		ContextNotes:       strings.TrimSpace(fixture.ContextNotes),
		Model:              model,
		State:              state,
		View:               view,
		Opportunity:        opportunity,
		Input:              input,
		RawResponse:        responseJSON(resp),
	}
	payload, invalid := extractJudgeForCausePayload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	if got := strings.TrimSpace(stringField(payload, "challenge_id")); got != "fc-1" {
		result.InvalidReason = "wrong_challenge_id"
		return result
	}
	if got := strings.TrimSpace(stringField(payload, "juror_id")); got != result.JurorID {
		result.InvalidReason = "wrong_juror_id"
		return result
	}
	if got := normalizeParty(stringField(payload, "by_party")); got != result.ChallengedBy {
		result.InvalidReason = "wrong_by_party"
		return result
	}
	granted, ok := payload["granted"].(bool)
	if !ok {
		result.InvalidReason = "malformed_granted"
		return result
	}
	result.Granted = &granted
	result.RulingReason = strings.TrimSpace(stringField(payload, "ruling_reason"))
	if result.RulingReason == "" {
		result.InvalidReason = "empty_ruling_reason"
		return result
	}
	rescoreJudgeForCauseResult(&result)
	return result
}

func rescoreJudgeForCauseResult(result *JudgeForCauseResult) {
	if result == nil || result.InvalidReason != "" || result.Granted == nil {
		return
	}
	result.OutcomeCorrect = *result.Granted == result.ExpectedGranted
	result.MatchedReasonTags = matchedJudgeForCauseReasonTags(result.RulingReason, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeForCausePayload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeForCauseTool {
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

func renderJudgeForCausePromptTemplate(template string, fixture JudgeForCauseFixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeForCausePromptTemplate", template,
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{challenged_by}}", normalizeParty(fixture.ChallengedBy),
		"{{juror_id}}", strings.TrimSpace(fixture.JurorID),
		"{{voir_dire_record}}", strings.TrimSpace(fixture.VoirDireRecord),
		"{{challenge_grounds}}", strings.TrimSpace(fixture.ChallengeGrounds),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeForCauseRoles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeForCauseTool}}}
}

func applyJudgeForCauseSummaryResult(summary *JudgeForCauseSummary, result JudgeForCauseResult, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else if result.OutcomeCorrect {
		summary.Correct++
	} else {
		classifyJudgeForCauseError(result, &summary.FalseGrants, &summary.FalseDenials)
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateJudgeForCauseSlice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeForCauseSlice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeForCauseSlice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeForCauseSlice(summary.ByChallengedBy, result.ChallengedBy, result, weight)
}

func updateJudgeForCauseSlice(m map[string]JudgeForCauseSlice, key string, result JudgeForCauseResult, weight float64) {
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
		classifyJudgeForCauseError(result, &s.FalseGrants, &s.FalseDenials)
	}
	m[key] = s
}

func classifyJudgeForCauseError(result JudgeForCauseResult, falseGrants *int, falseDenials *int) {
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

func finalizeJudgeForCauseSummary(summary *JudgeForCauseSummary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.FalseGrantRate = float64(summary.FalseGrants) / float64(summary.Total)
		summary.FalseDenialRate = float64(summary.FalseDenials) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeJudgeForCauseSlices(summary.ByReasonTag)
	finalizeJudgeForCauseSlices(summary.ByIssueFamily)
	finalizeJudgeForCauseSlices(summary.ByTier)
	finalizeJudgeForCauseSlices(summary.ByChallengedBy)
}

func finalizeJudgeForCauseSlices(m map[string]JudgeForCauseSlice) {
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

func matchedJudgeForCauseReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if judgeForCauseReasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeForCauseReasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range judgeForCauseReasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func judgeForCauseReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"fixed_bias":            {"cannot be impartial", "fixed opinion", "already decided", "bias"},
		"follow_law":            {"follow the law", "follow instructions", "follow the instructions", "court instructions", "burden of proof", "preponderance"},
		"damages_precommitment": {"damages", "minimum", "floor", "cap"},
		"digital_evidence":      {"digital", "electronic", "documentary", "records"},
		"hardship":              {"hardship", "unable to serve", "attention"},
		"relationship_interest": {"relationship", "employed", "financial interest", "stock"},
		"rehabilitated":         {"rehabilitated", "rehabilitation", "can be fair", "can serve", "credible assurance", "assurance"},
		"lawful_attitude":       {"general concern", "lawful attitude", "skepticism", "general preference", "not disqualifying", "can follow"},
		"sympathy_bias":         {"sympathy", "favor", "small business"},
		"language_attention":    {"language", "understand", "attention"},
	}
}

func resultJudgeForCausePromptSource(result JudgeForCauseResult) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultJudgeForCausePromptName(result JudgeForCauseResult) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}
