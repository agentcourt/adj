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

const JudgeRule51Tool = "settle_jury_instructions"

type JudgeRule51Fixture struct {
	ID                      string   `json:"id"`
	Tier                    int      `json:"tier"`
	IssueFamily             string   `json:"issue_family"`
	CaseTheme               string   `json:"case_theme"`
	ClaimSummary            string   `json:"claim_summary"`
	PlaintiffInstruction    string   `json:"plaintiff_instruction"`
	DefendantInstruction    string   `json:"defendant_instruction"`
	PlaintiffObjection      string   `json:"plaintiff_objection,omitempty"`
	DefendantObjection      string   `json:"defendant_objection,omitempty"`
	EvidenceSummary         string   `json:"evidence_summary,omitempty"`
	ExpectedRequiredTerms   []string `json:"expected_required_terms"`
	ExpectedProhibitedTerms []string `json:"expected_prohibited_terms,omitempty"`
	ExpectedReasonTags      []string `json:"expected_reason_tags"`
	Severity                float64  `json:"severity"`
	ContextNotes            string   `json:"context_notes,omitempty"`
}

type JudgeRule51Options struct {
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

type JudgeRule51RescoreOptions struct {
	ResultsPath string
	OutputDir   string
}

type JudgeRule51Summary struct {
	Evaluation            string                      `json:"evaluation"`
	Model                 string                      `json:"model"`
	ExecutionMode         string                      `json:"execution_mode"`
	CounterfactualModel   bool                        `json:"counterfactual_model"`
	PromptSource          string                      `json:"prompt_source"`
	PromptName            string                      `json:"prompt_name"`
	PromptPath            string                      `json:"prompt_path,omitempty"`
	FixturesPath          string                      `json:"fixtures_path"`
	OutputDir             string                      `json:"output_dir"`
	ResultsPath           string                      `json:"results_path"`
	SummaryPath           string                      `json:"summary_path"`
	Total                 int                         `json:"total"`
	Correct               int                         `json:"correct"`
	ReasonCorrect         int                         `json:"reason_correct"`
	Invalid               int                         `json:"invalid"`
	MissingRequired       int                         `json:"missing_required"`
	ProhibitedIncluded    int                         `json:"prohibited_included"`
	Accuracy              float64                     `json:"accuracy"`
	WeightedAccuracy      float64                     `json:"weighted_accuracy"`
	InvalidRate           float64                     `json:"invalid_rate"`
	MissingRequiredRate   float64                     `json:"missing_required_rate"`
	ProhibitedIncludeRate float64                     `json:"prohibited_include_rate"`
	ByReasonTag           map[string]JudgeRule51Slice `json:"by_reason_tag"`
	ByIssueFamily         map[string]JudgeRule51Slice `json:"by_issue_family"`
	ByTier                map[string]JudgeRule51Slice `json:"by_tier"`
	GeneratedAt           string                      `json:"generated_at"`
}

type JudgeRule51Slice struct {
	Total              int     `json:"total"`
	Correct            int     `json:"correct"`
	MissingRequired    int     `json:"missing_required"`
	ProhibitedIncluded int     `json:"prohibited_included"`
	Invalid            int     `json:"invalid"`
	Weight             float64 `json:"weight"`
	CorrectWeight      float64 `json:"correct_weight"`
	Accuracy           float64 `json:"accuracy"`
	WeightedAccuracy   float64 `json:"weighted_accuracy"`
}

type JudgeRule51Result struct {
	ID                      string            `json:"id"`
	Tier                    int               `json:"tier"`
	IssueFamily             string            `json:"issue_family"`
	CaseTheme               string            `json:"case_theme"`
	ClaimSummary            string            `json:"claim_summary"`
	PlaintiffInstruction    string            `json:"plaintiff_instruction"`
	DefendantInstruction    string            `json:"defendant_instruction"`
	PlaintiffObjection      string            `json:"plaintiff_objection,omitempty"`
	DefendantObjection      string            `json:"defendant_objection,omitempty"`
	EvidenceSummary         string            `json:"evidence_summary,omitempty"`
	ExpectedRequiredTerms   []string          `json:"expected_required_terms"`
	ExpectedProhibitedTerms []string          `json:"expected_prohibited_terms,omitempty"`
	ExpectedReasonTags      []string          `json:"expected_reason_tags"`
	Severity                float64           `json:"severity"`
	ContextNotes            string            `json:"context_notes,omitempty"`
	Model                   string            `json:"model"`
	ExecutionMode           string            `json:"execution_mode"`
	CounterfactualModel     bool              `json:"counterfactual_model"`
	PromptSource            string            `json:"prompt_source"`
	PromptName              string            `json:"prompt_name"`
	PromptPath              string            `json:"prompt_path,omitempty"`
	State                   map[string]any    `json:"state"`
	View                    map[string]any    `json:"view"`
	Opportunity             map[string]any    `json:"opportunity"`
	Input                   []map[string]any  `json:"input"`
	RawResponse             map[string]any    `json:"raw_response"`
	ResponseExchanges       []map[string]any  `json:"response_exchanges,omitempty"`
	TurnLog                 runner.TurnLog    `json:"turn_log"`
	FinalState              map[string]any    `json:"final_state,omitempty"`
	Provider                openai.Accounting `json:"provider"`
	ToolPayload             map[string]any    `json:"tool_payload,omitempty"`
	Summary                 string            `json:"summary,omitempty"`
	MissingRequiredTerms    []string          `json:"missing_required_terms,omitempty"`
	PresentProhibitedTerms  []string          `json:"present_prohibited_terms,omitempty"`
	MatchedReasonTags       []string          `json:"matched_reason_tags"`
	OutcomeCorrect          bool              `json:"outcome_correct"`
	ReasonCorrect           bool              `json:"reason_correct"`
	InvalidReason           string            `json:"invalid_reason,omitempty"`
	LeanAccepted            bool              `json:"lean_accepted"`
	StepAccepted            bool              `json:"step_accepted"`
	LeanError               string            `json:"lean_error,omitempty"`
}

type judgeRule51PromptVariant struct {
	Source   string
	Name     string
	Path     string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeRule51(ctx context.Context, opts JudgeRule51Options) (resultValue JudgeRule51Summary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule51Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule51Summary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeRule51Summary{}, err
	}
	fixtures, err := LoadJudgeRule51Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule51Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule51Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef, err := modelrequest.ParseModelRef(opts.Model)
	if err != nil {
		return JudgeRule51Summary{}, fmt.Errorf("parse --model: %w", err)
	}
	client, err := openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
	if err != nil {
		return JudgeRule51Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule51Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule51PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName)
	if err != nil {
		return JudgeRule51Summary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeRule51Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule51Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule51Summary{
		Evaluation:          "judge_rule51",
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
		ByReasonTag:         map[string]JudgeRule51Slice{},
		ByIssueFamily:       map[string]JudgeRule51Slice{},
		ByTier:              map[string]JudgeRule51Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule51Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule51Summary{}, err
		}
		result.ExecutionMode = summary.ExecutionMode
		result.CounterfactualModel = summary.CounterfactualModel
		if err := encoder.Encode(result); err != nil {
			return JudgeRule51Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyRule51SummaryResult(&summary, result, weight)
	}
	finalizeRule51Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule51Summary{}, err
	}
	return summary, nil
}

func RescoreJudgeRule51(opts JudgeRule51RescoreOptions) (resultValue JudgeRule51Summary, returnErr error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeRule51Summary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule51Summary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeRule51Results(opts.ResultsPath)
	if err != nil {
		return JudgeRule51Summary{}, err
	}
	if len(results) == 0 {
		return JudgeRule51Summary{}, fmt.Errorf("no results loaded from %s", opts.ResultsPath)
	}
	if err := validateJudgeEvalRescoreOutputPath(opts.ResultsPath, opts.OutputDir); err != nil {
		return JudgeRule51Summary{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule51Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule51Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := JudgeRule51Summary{
		Evaluation:          "judge_rule51",
		Model:               results[0].Model,
		ExecutionMode:       results[0].ExecutionMode,
		CounterfactualModel: results[0].CounterfactualModel,
		PromptSource:        resultRule51PromptSource(results[0]),
		PromptName:          resultRule51PromptName(results[0]),
		PromptPath:          results[0].PromptPath,
		FixturesPath:        "rescored from " + opts.ResultsPath,
		OutputDir:           opts.OutputDir,
		ResultsPath:         resultsPath,
		SummaryPath:         summaryPath,
		ByReasonTag:         map[string]JudgeRule51Slice{},
		ByIssueFamily:       map[string]JudgeRule51Slice{},
		ByTier:              map[string]JudgeRule51Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeRule51Result(&result)
		result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted
		if err := encoder.Encode(result); err != nil {
			return JudgeRule51Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyRule51SummaryResult(&summary, result, weight)
	}
	finalizeRule51Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule51Summary{}, err
	}
	return summary, nil
}

func runJudgeRule51Fixture(
	ctx context.Context,
	opts JudgeRule51Options,
	promptVariant judgeRule51PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule51Fixture,
) (JudgeRule51Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule51Result{}, err
	}
	state := BuildJudgeRule51State(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeRule51Roles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeRule51Tool)
	if err != nil {
		return JudgeRule51Result{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
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
			return renderJudgeRule51PromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeRule51Tool, executionErr)
	if decisionErr != nil {
		return JudgeRule51Result{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeRule51Response(fixture, opts.Model, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
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
	result.LeanAccepted, result.StepAccepted = judgeEvalExecutionAcceptance(execution, JudgeRule51Tool)
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	result.OutcomeCorrect = result.OutcomeCorrect && result.LeanAccepted && result.StepAccepted && executionErr == nil
	return result, nil
}

func LoadJudgeRule51Fixtures(path string) (resultValue []JudgeRule51Fixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule51Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule51Fixture
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

func readJudgeRule51Results(path string) (resultValue []JudgeRule51Result, returnErr error) {
	return readJudgeEvalJSONL[JudgeRule51Result](path)
}

func loadJudgeRule51PromptVariant(path string, name string) (judgeRule51PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule51PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule51PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule51PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	return judgeRule51PromptVariant{Source: "file:" + path, Name: name, Path: path, Text: text}, nil
}

func (f JudgeRule51Fixture) Validate() error {
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
	if strings.TrimSpace(f.ClaimSummary) == "" {
		return fmt.Errorf("fixture %s missing claim_summary", f.ID)
	}
	if strings.TrimSpace(f.PlaintiffInstruction) == "" {
		return fmt.Errorf("fixture %s missing plaintiff_instruction", f.ID)
	}
	if strings.TrimSpace(f.DefendantInstruction) == "" {
		return fmt.Errorf("fixture %s missing defendant_instruction", f.ID)
	}
	if len(f.ExpectedRequiredTerms) == 0 {
		return fmt.Errorf("fixture %s missing expected_required_terms", f.ID)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule51State(f JudgeRule51Fixture) map[string]any {
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        map[string]any{},
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-rule51-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "trial",
			"resolution":                    "pending",
			"trial_mode":                    "jury",
			"phase":                         "jury_charge",
			"last_pleading_served_on":       "",
			"jury_demanded_on":              "2026-07-14",
			"jury_configuration":            map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6},
			"single_claim":                  defaultJudgeEvalClaim(),
			"jurisdictional_allegations":    map[string]any{},
			"jurors":                        judgeRule51Jurors(),
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
			"docket":                        judgeRule51Docket(f),
			"decision_traces": []any{
				map[string]any{"action": "propose_jury_instruction", "outcome": "filed", "citations": []any{"FRCP 51(a)"}},
				map[string]any{"action": "object_jury_instruction", "outcome": "filed", "citations": []any{"FRCP 51(c)"}},
				map[string]any{"action": "deliver_closing_argument", "outcome": "completed", "citations": []any{}},
			},
		},
	}
}

func judgeRule51Docket(f JudgeRule51Fixture) []any {
	entries := []any{
		map[string]any{"title": "Complaint filed", "description": "claim=" + strings.TrimSpace(f.ClaimSummary)},
		map[string]any{"title": "Trial evidence summary", "description": strings.TrimSpace(f.EvidenceSummary)},
		map[string]any{"title": "Proposed jury instruction - plaintiff", "description": "instruction_id=PI-1 text=" + strings.TrimSpace(f.PlaintiffInstruction)},
		map[string]any{"title": "Proposed jury instruction - defendant", "description": "instruction_id=DI-1 text=" + strings.TrimSpace(f.DefendantInstruction)},
	}
	if strings.TrimSpace(f.PlaintiffObjection) != "" {
		entries = append(entries, map[string]any{"title": "Jury instruction objection - plaintiff", "description": "instruction_id=DI-1 grounds=" + strings.TrimSpace(f.PlaintiffObjection)})
	}
	if strings.TrimSpace(f.DefendantObjection) != "" {
		entries = append(entries, map[string]any{"title": "Jury instruction objection - defendant", "description": "instruction_id=PI-1 grounds=" + strings.TrimSpace(f.DefendantObjection)})
	}
	entries = append(entries,
		map[string]any{"title": "Plaintiff closing argument", "description": "completed"},
		map[string]any{"title": "Defense closing argument", "description": "completed"},
	)
	return entries
}

func judgeRule51Jurors() []any {
	out := make([]any, 0, 6)
	for i := 1; i <= 6; i++ {
		out = append(out, map[string]any{
			"juror_id":         fmt.Sprintf("J%d", i),
			"status":           "sworn",
			"name":             fmt.Sprintf("Juror %d", i),
			"note":             "",
			"model":            "eval-model",
			"persona_filename": "eval-persona",
		})
	}
	return out
}

func buildJudgeRule51Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule51Fixture,
	promptVariant judgeRule51PromptVariant,
) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeRule51PromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeRule51Tool, view, opportunity, objective)
}

func scoreJudgeRule51Response(
	fixture JudgeRule51Fixture,
	model string,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule51Result {
	result := JudgeRule51Result{
		ID:                      fixture.ID,
		Tier:                    fixture.Tier,
		IssueFamily:             fixture.IssueFamily,
		CaseTheme:               strings.TrimSpace(fixture.CaseTheme),
		ClaimSummary:            strings.TrimSpace(fixture.ClaimSummary),
		PlaintiffInstruction:    strings.TrimSpace(fixture.PlaintiffInstruction),
		DefendantInstruction:    strings.TrimSpace(fixture.DefendantInstruction),
		PlaintiffObjection:      strings.TrimSpace(fixture.PlaintiffObjection),
		DefendantObjection:      strings.TrimSpace(fixture.DefendantObjection),
		EvidenceSummary:         strings.TrimSpace(fixture.EvidenceSummary),
		ExpectedRequiredTerms:   append([]string{}, fixture.ExpectedRequiredTerms...),
		ExpectedProhibitedTerms: append([]string{}, fixture.ExpectedProhibitedTerms...),
		ExpectedReasonTags:      append([]string{}, fixture.ExpectedReasonTags...),
		Severity:                normalizedSeverity(fixture.Severity),
		ContextNotes:            strings.TrimSpace(fixture.ContextNotes),
		Model:                   model,
		State:                   state,
		View:                    view,
		Opportunity:             opportunity,
		Input:                   input,
		RawResponse:             responseJSON(resp),
	}
	payload, invalid := extractJudgeRule51Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	result.Summary = strings.TrimSpace(stringField(payload, "summary"))
	if result.Summary == "" {
		result.InvalidReason = "empty_summary"
		return result
	}
	result.MissingRequiredTerms = missingRule51Terms(result.Summary, fixture.ExpectedRequiredTerms)
	result.PresentProhibitedTerms = presentRule51Terms(result.Summary, fixture.ExpectedProhibitedTerms)
	result.OutcomeCorrect = len(result.MissingRequiredTerms) == 0 && len(result.PresentProhibitedTerms) == 0
	result.MatchedReasonTags = matchedRule51ReasonTags(result.Summary, fixture.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
	return result
}

func rescoreJudgeRule51Result(result *JudgeRule51Result) {
	if result == nil || result.InvalidReason != "" {
		return
	}
	result.MissingRequiredTerms = missingRule51Terms(result.Summary, result.ExpectedRequiredTerms)
	result.PresentProhibitedTerms = presentRule51Terms(result.Summary, result.ExpectedProhibitedTerms)
	result.OutcomeCorrect = len(result.MissingRequiredTerms) == 0 && len(result.PresentProhibitedTerms) == 0
	result.MatchedReasonTags = matchedRule51ReasonTags(result.Summary, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeRule51Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule51Tool {
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

func renderJudgeRule51PromptTemplate(template string, fixture JudgeRule51Fixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeRule51PromptTemplate", template,
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{claim_summary}}", strings.TrimSpace(fixture.ClaimSummary),
		"{{plaintiff_instruction}}", strings.TrimSpace(fixture.PlaintiffInstruction),
		"{{defendant_instruction}}", strings.TrimSpace(fixture.DefendantInstruction),
		"{{plaintiff_objection}}", strings.TrimSpace(fixture.PlaintiffObjection),
		"{{defendant_objection}}", strings.TrimSpace(fixture.DefendantObjection),
		"{{evidence_summary}}", strings.TrimSpace(fixture.EvidenceSummary),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeRule51Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule51Tool}}}
}

func missingRule51Terms(summary string, terms []string) []string {
	out := make([]string, 0)
	for _, term := range terms {
		if !rule51ContainsTerm(summary, term) {
			out = append(out, strings.TrimSpace(term))
		}
	}
	sort.Strings(out)
	return out
}

func presentRule51Terms(summary string, terms []string) []string {
	out := make([]string, 0)
	for _, term := range terms {
		if rule51ContainsProhibitedTerm(summary, term) {
			out = append(out, strings.TrimSpace(term))
		}
	}
	sort.Strings(out)
	return out
}

func rule51ContainsTerm(summary string, term string) bool {
	summary = normalizeReasonText(summary)
	term = normalizeReasonText(term)
	if term == "" {
		return true
	}
	if strings.Contains(summary, term) {
		return true
	}
	switch term {
	case "admitted evidence":
		return strings.Contains(summary, "evidence admitted") || strings.Contains(summary, "admitted exhibits")
	case "causation":
		return strings.Contains(summary, "caused") || strings.Contains(summary, "cause")
	case "may infer":
		return strings.Contains(summary, "may") && strings.Contains(summary, "infer")
	case "no adverse inference":
		return strings.Contains(summary, "no adverse inference") ||
			(strings.Contains(summary, "must not draw") && strings.Contains(summary, "adverse inference"))
	default:
		return false
	}
}

func rule51ContainsProhibitedTerm(summary string, term string) bool {
	summary = normalizeReasonText(summary)
	term = normalizeReasonText(term)
	if term == "" {
		return false
	}
	searchAt := 0
	for {
		idx := strings.Index(summary[searchAt:], term)
		if idx < 0 {
			return false
		}
		start := searchAt + idx
		end := start + len(term)
		contextStart := start - 140
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := end + 140
		if contextEnd > len(summary) {
			contextEnd = len(summary)
		}
		if !rule51RejectedOrNegatedContext(summary[contextStart:contextEnd]) {
			return true
		}
		searchAt = end
		if searchAt >= len(summary) {
			return false
		}
	}
}

func rule51RejectedOrNegatedContext(context string) bool {
	for _, keyword := range []string{
		"rejected",
		"refused",
		"sustained",
		"denied",
		"not applicable",
		"will not",
		"do not",
		"must not",
		"not to",
		"not part",
		"no instruction",
		"excluded",
		"cannot",
		"without",
		"not required",
	} {
		if strings.Contains(context, keyword) {
			return true
		}
	}
	return false
}

func applyRule51SummaryResult(summary *JudgeRule51Summary, result JudgeRule51Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else if result.OutcomeCorrect {
		summary.Correct++
	} else {
		classifyRule51Error(result, &summary.MissingRequired, &summary.ProhibitedIncluded)
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateRule51Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateRule51Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateRule51Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
}

func updateRule51Slice(m map[string]JudgeRule51Slice, key string, result JudgeRule51Result, weight float64) {
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
		classifyRule51Error(result, &s.MissingRequired, &s.ProhibitedIncluded)
	}
	m[key] = s
}

func classifyRule51Error(result JudgeRule51Result, missingRequired *int, prohibitedIncluded *int) {
	if len(result.MissingRequiredTerms) > 0 {
		(*missingRequired)++
	}
	if len(result.PresentProhibitedTerms) > 0 {
		(*prohibitedIncluded)++
	}
}

func finalizeRule51Summary(summary *JudgeRule51Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
		summary.MissingRequiredRate = float64(summary.MissingRequired) / float64(summary.Total)
		summary.ProhibitedIncludeRate = float64(summary.ProhibitedIncluded) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeRule51Slices(summary.ByReasonTag)
	finalizeRule51Slices(summary.ByIssueFamily)
	finalizeRule51Slices(summary.ByTier)
}

func finalizeRule51Slices(m map[string]JudgeRule51Slice) {
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

func matchedRule51ReasonTags(summary string, expected []string) []string {
	summary = normalizeReasonText(summary)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if rule51SummaryMatchesTag(summary, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func rule51SummaryMatchesTag(summary string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(summary, normalizedTag) {
		return true
	}
	for _, keyword := range rule51ReasonTagKeywords()[tag] {
		if strings.Contains(summary, keyword) {
			return true
		}
	}
	return false
}

func rule51ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"burden_standard":      {"preponderance", "burden of proof", "plaintiff must prove"},
		"burden_shift":         {"burden remains", "does not shift", "plaintiff bears"},
		"claim_elements":       {"contract", "breach", "causation", "damages", "element"},
		"argumentative":        {"must decide the facts", "advocacy", "argumentative"},
		"assumes_fact":         {"if you find", "do not assume", "disputed fact"},
		"excluded_evidence":    {"excluded", "not consider", "outside the record"},
		"damages":              {"damages", "reasonable certainty", "caused by breach"},
		"credibility":          {"credibility", "witness", "believe"},
		"limiting_instruction": {"limited purpose", "only for", "not for liability"},
		"adverse_inference":    {"adverse inference", "may infer", "not required"},
		"digital_evidence":     {"digital", "electronic", "authenticated"},
		"sympathy":             {"sympathy", "prejudice", "bias"},
	}
}

func resultRule51PromptSource(result JudgeRule51Result) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultRule51PromptName(result JudgeRule51Result) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}
