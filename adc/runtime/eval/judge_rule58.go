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

const JudgeRule58Tool = "enter_judgment"

type JudgeRule58Fixture struct {
	ID                      string   `json:"id"`
	Tier                    int      `json:"tier"`
	IssueFamily             string   `json:"issue_family"`
	CaseTheme               string   `json:"case_theme"`
	TrialMode               string   `json:"trial_mode"`
	VerdictFor              string   `json:"verdict_for,omitempty"`
	VerdictDamages          float64  `json:"verdict_damages,omitempty"`
	BenchOpinionText        string   `json:"bench_opinion_text,omitempty"`
	BenchJudgmentAmount     float64  `json:"bench_judgment_amount,omitempty"`
	ExpectedClaimID         string   `json:"expected_claim_id"`
	ExpectedBasis           string   `json:"expected_basis"`
	ExpectedAmount          float64  `json:"expected_amount"`
	RequiredBasisConcepts   []string `json:"required_basis_concepts"`
	ProhibitedBasisConcepts []string `json:"prohibited_basis_concepts,omitempty"`
	ExpectedReasonTags      []string `json:"expected_reason_tags"`
	Severity                float64  `json:"severity"`
	ContextNotes            string   `json:"context_notes,omitempty"`
}

type JudgeRule58Options struct {
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

type JudgeRule58Summary struct {
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
	ClaimCorrect        int                         `json:"claim_correct"`
	BasisCorrect        int                         `json:"basis_correct"`
	AmountCorrect       int                         `json:"amount_correct"`
	StatusCorrect       int                         `json:"status_correct"`
	ReasonCorrect       int                         `json:"reason_correct"`
	Invalid             int                         `json:"invalid"`
	LeanRejected        int                         `json:"lean_rejected"`
	StepRejected        int                         `json:"step_rejected"`
	Accuracy            float64                     `json:"accuracy"`
	WeightedAccuracy    float64                     `json:"weighted_accuracy"`
	InvalidRate         float64                     `json:"invalid_rate"`
	LeanRejectionRate   float64                     `json:"lean_rejection_rate"`
	ByReasonTag         map[string]JudgeRule58Slice `json:"by_reason_tag"`
	ByIssueFamily       map[string]JudgeRule58Slice `json:"by_issue_family"`
	ByTier              map[string]JudgeRule58Slice `json:"by_tier"`
	ByTrialMode         map[string]JudgeRule58Slice `json:"by_trial_mode"`
	GeneratedAt         string                      `json:"generated_at"`
}

type JudgeRule58Slice struct {
	Total            int     `json:"total"`
	Correct          int     `json:"correct"`
	Invalid          int     `json:"invalid"`
	LeanRejected     int     `json:"lean_rejected"`
	StepRejected     int     `json:"step_rejected"`
	Weight           float64 `json:"weight"`
	CorrectWeight    float64 `json:"correct_weight"`
	Accuracy         float64 `json:"accuracy"`
	WeightedAccuracy float64 `json:"weighted_accuracy"`
}

type JudgeRule58Result struct {
	ID                        string            `json:"id"`
	Tier                      int               `json:"tier"`
	IssueFamily               string            `json:"issue_family"`
	CaseTheme                 string            `json:"case_theme"`
	TrialMode                 string            `json:"trial_mode"`
	VerdictFor                string            `json:"verdict_for,omitempty"`
	VerdictDamages            float64           `json:"verdict_damages,omitempty"`
	BenchOpinionText          string            `json:"bench_opinion_text,omitempty"`
	BenchJudgmentAmount       float64           `json:"bench_judgment_amount,omitempty"`
	ExpectedClaimID           string            `json:"expected_claim_id"`
	ExpectedBasis             string            `json:"expected_basis"`
	ExpectedAmount            float64           `json:"expected_amount"`
	RequiredBasisConcepts     []string          `json:"required_basis_concepts"`
	ProhibitedBasisConcepts   []string          `json:"prohibited_basis_concepts,omitempty"`
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
	ClaimID                   string            `json:"claim_id,omitempty"`
	Basis                     string            `json:"basis,omitempty"`
	MissingBasisConcepts      []string          `json:"missing_basis_concepts,omitempty"`
	PresentProhibitedConcepts []string          `json:"present_prohibited_concepts,omitempty"`
	MatchedReasonTags         []string          `json:"matched_reason_tags,omitempty"`
	AppliedState              map[string]any    `json:"applied_state,omitempty"`
	AppliedCaseStatus         string            `json:"applied_case_status,omitempty"`
	AppliedMonetaryJudgment   float64           `json:"applied_monetary_judgment,omitempty"`
	ClaimCorrect              bool              `json:"claim_correct"`
	BasisCorrect              bool              `json:"basis_correct"`
	AmountCorrect             bool              `json:"amount_correct"`
	StatusCorrect             bool              `json:"status_correct"`
	OutcomeCorrect            bool              `json:"outcome_correct"`
	ReasonCorrect             bool              `json:"reason_correct"`
	InvalidReason             string            `json:"invalid_reason,omitempty"`
	LeanAccepted              bool              `json:"lean_accepted"`
	StepAccepted              bool              `json:"step_accepted"`
	LeanError                 string            `json:"lean_error,omitempty"`
}

type judgeRule58PromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
	Renderer *runner.PromptRenderer
}

func RunJudgeRule58(ctx context.Context, opts JudgeRule58Options) (resultValue JudgeRule58Summary, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule58Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule58Summary{}, fmt.Errorf("output directory is required")
	}
	if strings.TrimSpace(opts.OpportunityPromptPath) != "" && !opts.CounterfactualModel {
		return JudgeRule58Summary{}, fmt.Errorf("opportunity prompt file requires counterfactual model mode")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	var err error
	opts.Court, err = resolveJudgeEvalCourt(opts.Court)
	if err != nil {
		return JudgeRule58Summary{}, err
	}
	fixtures, err := LoadJudgeRule58Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule58Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule58Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if opts.CounterfactualModel && !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule58Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule58Summary{}, err
		}
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule58Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule58PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeRule58Summary{}, err
	}
	promptVariant.Renderer, err = loadJudgePromptRenderer(opts.Court, opts.PromptDir, opts.PromptFiles)
	if err != nil {
		return JudgeRule58Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule58Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer closeEvalFile(resultsFile, resultsPath, &returnErr)

	summary := newJudgeRule58Summary(opts, promptVariant, resultsPath, summaryPath)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule58Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule58Summary{}, err
		}
		if err := encoder.Encode(result); err != nil {
			return JudgeRule58Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule58SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule58Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule58Summary{}, err
	}
	return summary, nil
}

func newJudgeRule58Summary(opts JudgeRule58Options, promptVariant judgeRule58PromptVariant, resultsPath string, summaryPath string) JudgeRule58Summary {
	return JudgeRule58Summary{
		Evaluation:          "judge_rule58",
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
		ByReasonTag:         map[string]JudgeRule58Slice{},
		ByIssueFamily:       map[string]JudgeRule58Slice{},
		ByTier:              map[string]JudgeRule58Slice{},
		ByTrialMode:         map[string]JudgeRule58Slice{},
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeRule58Fixture(
	ctx context.Context,
	opts JudgeRule58Options,
	promptVariant judgeRule58PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule58Fixture,
) (JudgeRule58Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule58Result{}, err
	}
	state := BuildJudgeRule58State(fixture)
	applyJudgeEvalCourt(state, opts.Court)
	rolesPayload := judgeRule58Roles()
	roles, err := judgeEvalRoleSpecs(promptVariant.Renderer, JudgeRule58Tool)
	if err != nil {
		return JudgeRule58Result{}, fmt.Errorf("fixture %s roles: %w", fixture.ID, err)
	}
	executionModel := opts.Model
	if opts.CounterfactualModel && !opts.DryRun {
		executionModel = modelRef.Model
	}
	responseClient := judgeEvalResponseClient(opts.DryRun, client, dryRunJudgeRule58Response(fixture))
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
			return renderJudgeRule58PromptTemplate(promptVariant.Text, fixture, opportunity)
		},
	})
	cancel()
	decision, decisionErr := judgeEvalDecisionForScoring(execution, JudgeRule58Tool, executionErr)
	if decisionErr != nil {
		return JudgeRule58Result{}, fmt.Errorf("fixture %s execute opportunity: %w", fixture.ID, decisionErr)
	}
	result := scoreJudgeRule58Response(fixture, opts.Model, opts.DryRun, state, execution.View, execution.Opportunity, decision.Input, decision.ScoringResponse)
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
	result.LeanAccepted, result.StepAccepted = judgeEvalExecutionAcceptance(execution, JudgeRule58Tool)
	result.AppliedState = execution.FinalState
	if appliedCase := judgeRule58AppliedCase(execution.FinalState); appliedCase != nil {
		result.AppliedCaseStatus = stringField(appliedCase, "status")
		result.AppliedMonetaryJudgment = judgeRule58FloatField(appliedCase, "monetary_judgment")
	}
	if executionErr != nil {
		result.LeanError = executionErr.Error()
	}
	finalizeJudgeRule58Result(&result)
	result.OutcomeCorrect = result.OutcomeCorrect && executionErr == nil
	return result, nil
}

func LoadJudgeRule58Fixtures(path string) (resultValue []JudgeRule58Fixture, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule58Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule58Fixture
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

func loadJudgeRule58PromptVariant(path string, name string, outputDir string) (judgeRule58PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule58PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule58PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule58PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeRule58PromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeRule58PromptVariant{Source: "file:" + path, Name: name, Path: path, CopyPath: copyPath, Text: text}, nil
}

func (f JudgeRule58Fixture) Validate() error {
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
	if f.TrialMode != "jury" && f.TrialMode != "bench" {
		return fmt.Errorf("fixture %s invalid trial_mode %q", f.ID, f.TrialMode)
	}
	if f.TrialMode == "jury" && f.VerdictFor != "plaintiff" && f.VerdictFor != "defendant" {
		return fmt.Errorf("fixture %s invalid verdict_for %q", f.ID, f.VerdictFor)
	}
	if f.TrialMode == "bench" && strings.TrimSpace(f.BenchOpinionText) == "" {
		return fmt.Errorf("fixture %s missing bench_opinion_text", f.ID)
	}
	if strings.TrimSpace(f.ExpectedClaimID) == "" {
		return fmt.Errorf("fixture %s missing expected_claim_id", f.ID)
	}
	if strings.TrimSpace(f.ExpectedBasis) == "" {
		return fmt.Errorf("fixture %s missing expected_basis", f.ID)
	}
	if len(nonemptyRule52Strings(f.RequiredBasisConcepts)) == 0 {
		return fmt.Errorf("fixture %s missing required_basis_concepts", f.ID)
	}
	if len(nonemptyRule52Strings(f.ExpectedReasonTags)) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule58State(f JudgeRule58Fixture) map[string]any {
	caseObj := map[string]any{
		"case_id":                       "judge-rule58-" + strings.TrimSpace(f.ID),
		"caption":                       strings.TrimSpace(f.CaseTheme),
		"judge":                         "Judge Eval",
		"filed_on":                      "2026-07-14",
		"auto_rule11":                   false,
		"status":                        "trial",
		"resolution":                    "pending",
		"trial_mode":                    strings.TrimSpace(f.TrialMode),
		"phase":                         "post_verdict",
		"last_pleading_served_on":       "2026-07-01",
		"jury_demanded_on":              judgeRule58JuryDemandedOn(f),
		"jury_configuration":            judgeRule58JuryConfiguration(f),
		"single_claim":                  defaultJudgeEvalClaim(),
		"jurisdictional_allegations":    nil,
		"jurors":                        []any{},
		"juror_questionnaire":           []any{},
		"juror_questionnaire_responses": []any{},
		"voir_dire_exchanges":           []any{},
		"for_cause_challenges":          []any{},
		"deliberation_round":            1,
		"juror_votes":                   []any{},
		"jury_verdict":                  judgeRule58JuryVerdict(f),
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
		"monetary_judgment":             judgeRule58StartingJudgmentAmount(f),
		"docket":                        judgeRule58Docket(f),
		"decision_traces":               judgeRule58DecisionTraces(f),
	}
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case":                 caseObj,
	}
}

func judgeRule58JuryDemandedOn(f JudgeRule58Fixture) string {
	if f.TrialMode == "jury" {
		return "2026-07-02"
	}
	return ""
}

func judgeRule58JuryConfiguration(f JudgeRule58Fixture) any {
	if f.TrialMode != "jury" {
		return nil
	}
	return map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6}
}

func judgeRule58JuryVerdict(f JudgeRule58Fixture) any {
	if f.TrialMode != "jury" {
		return nil
	}
	return map[string]any{
		"verdict_for":       strings.TrimSpace(f.VerdictFor),
		"votes_for_verdict": 6,
		"required_votes":    6,
		"damages":           f.VerdictDamages,
	}
}

func judgeRule58StartingJudgmentAmount(f JudgeRule58Fixture) float64 {
	if f.TrialMode == "bench" {
		return f.BenchJudgmentAmount
	}
	return 0
}

func judgeRule58Docket(f JudgeRule58Fixture) []any {
	entries := []any{
		map[string]any{"title": "Complaint filed", "description": "single breach claim"},
		map[string]any{"title": "Answer filed", "description": "liability and damages disputed"},
		map[string]any{"title": "Trial mode resolved", "description": strings.TrimSpace(f.TrialMode)},
	}
	if f.TrialMode == "jury" {
		entries = append(entries,
			map[string]any{"title": "Jury verdict derived", "description": fmt.Sprintf("verdict_for=%s votes_for_verdict=6 required_votes=6 damages=%s", f.VerdictFor, judgeRule58FormatAmount(f.VerdictDamages))},
			map[string]any{"title": "Trial phase advanced", "description": "post_verdict"},
		)
	} else {
		entries = append(entries,
			map[string]any{"title": "Bench Opinion", "description": strings.TrimSpace(f.BenchOpinionText)},
			map[string]any{"title": "Trial phase advanced", "description": "post_verdict"},
		)
	}
	return entries
}

func judgeRule58DecisionTraces(f JudgeRule58Fixture) []any {
	traces := []any{
		map[string]any{"action": "file_complaint", "outcome": "filed", "citations": []any{"FRCP 3", "FRCP 8(a)"}},
		map[string]any{"action": "file_answer", "outcome": "filed", "citations": []any{"FRCP 8(b)"}},
		map[string]any{"action": "resolve_trial_mode", "outcome": strings.TrimSpace(f.TrialMode), "citations": []any{"FRCP 38", "FRCP 39"}},
	}
	if f.TrialMode == "jury" {
		traces = append(traces,
			map[string]any{"action": "deliver_jury_instructions", "outcome": "delivered", "citations": []any{"FRCP 51"}},
			map[string]any{"action": "record_jury_verdict", "outcome": strings.TrimSpace(f.VerdictFor), "citations": []any{"FRCP 48"}},
		)
	} else {
		traces = append(traces, map[string]any{"action": "file_bench_opinion", "outcome": "entered", "citations": []any{"FRCP 52(a)(1)"}})
	}
	traces = append(traces, map[string]any{"action": "advance_trial_phase", "outcome": "post_verdict", "citations": []any{}})
	return traces
}

func buildJudgeRule58Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule58Fixture,
	promptVariant judgeRule58PromptVariant,
) ([]map[string]any, error) {
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		var err error
		objective, err = renderJudgeRule58PromptTemplate(promptVariant.Text, fixture, opportunity)
		if err != nil {
			return nil, err
		}
	}
	return buildJudgeEvalInput(promptVariant.Renderer, JudgeRule58Tool, view, opportunity, objective)
}

func scoreJudgeRule58Response(
	fixture JudgeRule58Fixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule58Result {
	result := JudgeRule58Result{
		ID:                      fixture.ID,
		Tier:                    fixture.Tier,
		IssueFamily:             strings.TrimSpace(fixture.IssueFamily),
		CaseTheme:               strings.TrimSpace(fixture.CaseTheme),
		TrialMode:               strings.TrimSpace(fixture.TrialMode),
		VerdictFor:              strings.TrimSpace(fixture.VerdictFor),
		VerdictDamages:          fixture.VerdictDamages,
		BenchOpinionText:        strings.TrimSpace(fixture.BenchOpinionText),
		BenchJudgmentAmount:     fixture.BenchJudgmentAmount,
		ExpectedClaimID:         strings.TrimSpace(fixture.ExpectedClaimID),
		ExpectedBasis:           strings.TrimSpace(fixture.ExpectedBasis),
		ExpectedAmount:          fixture.ExpectedAmount,
		RequiredBasisConcepts:   nonemptyRule52Strings(fixture.RequiredBasisConcepts),
		ProhibitedBasisConcepts: nonemptyRule52Strings(fixture.ProhibitedBasisConcepts),
		ExpectedReasonTags:      nonemptyRule52Strings(fixture.ExpectedReasonTags),
		Severity:                normalizedSeverity(fixture.Severity),
		ContextNotes:            strings.TrimSpace(fixture.ContextNotes),
		Model:                   model,
		DryRun:                  dryRun,
		State:                   state,
		View:                    view,
		Opportunity:             opportunity,
		Input:                   input,
		RawResponse:             responseJSON(resp),
	}
	payload, invalid := extractJudgeRule58Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	result.ClaimID = strings.TrimSpace(stringField(payload, "claim_id"))
	result.Basis = strings.TrimSpace(stringField(payload, "basis"))
	if result.ClaimID == "" {
		result.InvalidReason = "missing_claim_id"
		return result
	}
	if result.Basis == "" {
		result.InvalidReason = "missing_basis"
		return result
	}
	scoreJudgeRule58Payload(&result)
	return result
}

func scoreJudgeRule58Payload(result *JudgeRule58Result) {
	result.ClaimCorrect = result.ClaimID == result.ExpectedClaimID
	result.MissingBasisConcepts = missingJudgeRule58Concepts(result.Basis, result.RequiredBasisConcepts)
	result.PresentProhibitedConcepts = presentJudgeRule58Concepts(result.Basis, result.ProhibitedBasisConcepts)
	result.BasisCorrect = len(result.MissingBasisConcepts) == 0 && len(result.PresentProhibitedConcepts) == 0
	result.MatchedReasonTags = matchedJudgeRule58ReasonTags(result.Basis, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func finalizeJudgeRule58Result(result *JudgeRule58Result) {
	if result == nil || result.InvalidReason != "" {
		return
	}
	result.StatusCorrect = result.AppliedCaseStatus == "judgment_entered"
	result.AmountCorrect = judgeRule58AmountsEqual(result.AppliedMonetaryJudgment, result.ExpectedAmount)
	result.OutcomeCorrect = result.ClaimCorrect &&
		result.BasisCorrect &&
		result.ReasonCorrect &&
		result.LeanAccepted &&
		result.StepAccepted &&
		result.StatusCorrect &&
		result.AmountCorrect
}

func extractJudgeRule58Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule58Tool {
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

func dryRunJudgeRule58Response(f JudgeRule58Fixture) openai.Response {
	return openai.Response{
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID: "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:   JudgeRule58Tool,
			Arguments: map[string]any{
				"claim_id": strings.TrimSpace(f.ExpectedClaimID),
				"basis":    strings.TrimSpace(f.ExpectedBasis),
			},
		}},
	}
}

func renderJudgeRule58PromptTemplate(template string, fixture JudgeRule58Fixture, opportunity map[string]any) (string, error) {
	return renderJudgeCandidatePrompt("renderJudgeRule58PromptTemplate", template,
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{trial_mode}}", strings.TrimSpace(fixture.TrialMode),
		"{{verdict_for}}", strings.TrimSpace(fixture.VerdictFor),
		"{{verdict_damages}}", judgeRule58FormatAmount(fixture.VerdictDamages),
		"{{bench_opinion_text}}", strings.TrimSpace(fixture.BenchOpinionText),
		"{{bench_judgment_amount}}", judgeRule58FormatAmount(fixture.BenchJudgmentAmount),
		"{{expected_claim_id}}", strings.TrimSpace(fixture.ExpectedClaimID),
		"{{expected_basis}}", strings.TrimSpace(fixture.ExpectedBasis),
		"{{expected_amount}}", judgeRule58FormatAmount(fixture.ExpectedAmount),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
}

func judgeRule58Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule58Tool}}}
}

func applyJudgeRule58SummaryResult(summary *JudgeRule58Summary, result JudgeRule58Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else {
		if result.ClaimCorrect {
			summary.ClaimCorrect++
		}
		if result.BasisCorrect {
			summary.BasisCorrect++
		}
		if result.AmountCorrect {
			summary.AmountCorrect++
		}
		if result.StatusCorrect {
			summary.StatusCorrect++
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
		updateJudgeRule58Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeRule58Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeRule58Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeRule58Slice(summary.ByTrialMode, result.TrialMode, result, weight)
}

func updateJudgeRule58Slice(m map[string]JudgeRule58Slice, key string, result JudgeRule58Result, weight float64) {
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

func finalizeJudgeRule58Summary(summary *JudgeRule58Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
		summary.LeanRejectionRate = float64(summary.LeanRejected) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeJudgeRule58Slices(summary.ByReasonTag)
	finalizeJudgeRule58Slices(summary.ByIssueFamily)
	finalizeJudgeRule58Slices(summary.ByTier)
	finalizeJudgeRule58Slices(summary.ByTrialMode)
}

func finalizeJudgeRule58Slices(m map[string]JudgeRule58Slice) {
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

func missingJudgeRule58Concepts(text string, concepts []string) []string {
	normalized := normalizeReasonText(text)
	missing := make([]string, 0)
	for _, concept := range nonemptyRule52Strings(concepts) {
		if !strings.Contains(normalized, normalizeReasonText(concept)) {
			missing = append(missing, concept)
		}
	}
	sort.Strings(missing)
	return missing
}

func presentJudgeRule58Concepts(text string, concepts []string) []string {
	normalized := normalizeReasonText(text)
	present := make([]string, 0)
	for _, concept := range nonemptyRule52Strings(concepts) {
		if strings.Contains(normalized, normalizeReasonText(concept)) {
			present = append(present, concept)
		}
	}
	sort.Strings(present)
	return present
}

func matchedJudgeRule58ReasonTags(text string, expected []string) []string {
	normalized := normalizeReasonText(text)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if judgeRule58ReasonMatchesTag(normalized, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeRule58ReasonMatchesTag(normalizedText string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(normalizedText, normalizedTag) {
		return true
	}
	for _, keyword := range judgeRule58ReasonTagKeywords()[tag] {
		if strings.Contains(normalizedText, keyword) {
			return true
		}
	}
	return false
}

func judgeRule58ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"jury_verdict":  {"jury verdict", "verdict"},
		"bench_verdict": {"bench verdict", "bench opinion"},
	}
}

func judgeRule58AppliedCase(state map[string]any) map[string]any {
	if state == nil {
		return nil
	}
	if caseObj, _ := state["case"].(map[string]any); caseObj != nil {
		return caseObj
	}
	if cases, _ := state["cases"].([]any); len(cases) > 0 {
		caseObj, _ := cases[0].(map[string]any)
		return caseObj
	}
	return nil
}

func judgeRule58ActionPayload(applyResp map[string]any, fallback map[string]any) map[string]any {
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

func judgeRule58FloatField(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		out, _ := v.Float64()
		return out
	default:
		return 0
	}
}

func judgeRule58AmountsEqual(got float64, want float64) bool {
	return math.Abs(got-want) < 0.000001
}

func judgeRule58FormatAmount(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.000001 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strconv.FormatFloat(value, 'f', 2, 64)
}
