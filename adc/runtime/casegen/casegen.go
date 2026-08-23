package casegen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentcourt/adj/adc/runtime/courts"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/common/openai"
)

var markdownLinkPattern = regexp.MustCompile(`!?\[([^\]]*)\]\(([^)]+)\)`)

const (
	defaultJudgeName     = "Judge A. Neutral"
	defaultRuntimeModel  = "gpt-4.1-mini"
	defaultPlannerModel  = "gpt-4.1-mini"
	defaultNonJurorModel = "gpt-5.4"
	maxPlannerAttempts   = 3
)

func DefaultRuntimeModel() string {
	return defaultRuntimeModel
}

func DefaultPlannerModel() string {
	return defaultPlannerModel
}

func DefaultNonJurorModel() string {
	return defaultNonJurorModel
}

type LinkedFile struct {
	Label         string `json:"label"`
	ReferencePath string `json:"reference_path,omitempty"`
	OriginalPath  string `json:"original_path"`
	OriginalName  string `json:"original_name"`
	StagedRelPath string `json:"staged_relpath,omitempty"`
	StagedAbsPath string `json:"-"`
	PreviewKind   string `json:"preview_kind"`
	Preview       string `json:"preview"`
}

type ComplaintInput struct {
	OriginalPath  string       `json:"original_path"`
	StagedRelPath string       `json:"staged_relpath,omitempty"`
	StagedAbsPath string       `json:"-"`
	Markdown      string       `json:"markdown"`
	LinkedFiles   []LinkedFile `json:"linked_files"`
}

type CasePacket struct {
	Error                    string         `json:"error"`
	Caption                  string         `json:"caption"`
	PlaintiffName            string         `json:"plaintiff_name"`
	DefendantName            string         `json:"defendant_name"`
	ComplaintSummary         string         `json:"complaint_summary"`
	RequestedRelief          string         `json:"requested_relief"`
	TrialModeRecommendation  string         `json:"trial_mode_recommendation"`
	JurisdictionBasis        string         `json:"jurisdiction_basis"`
	JurisdictionalStatement  string         `json:"jurisdictional_statement"`
	InjuryStatement          string         `json:"injury_statement"`
	CausationStatement       string         `json:"causation_statement"`
	RedressabilityStatement  string         `json:"redressability_statement"`
	RipenessStatement        string         `json:"ripeness_statement"`
	LiveControversyStatement string         `json:"live_controversy_statement"`
	PlaintiffCitizenship     string         `json:"plaintiff_citizenship,omitempty"`
	DefendantCitizenship     string         `json:"defendant_citizenship,omitempty"`
	AmountInControversy      string         `json:"amount_in_controversy,omitempty"`
	Claim                    spec.ClaimSpec `json:"claim"`
}

type Plan struct {
	Packet            CasePacket `json:"packet"`
	PlaintiffStrategy string     `json:"plaintiff_strategy"`
	DefenseStrategy   string     `json:"defense_strategy"`
}

type ScenarioOptions struct {
	RuntimeModel        string
	Temperature         *float64
	NonJurorTemperature *float64
	PlaintiffModel      string
	DefendantModel      string
	JudgeModel          string
	ClerkModel          string
	Court               courts.Profile
	JudgeName           string
	FiledOn             string
	TrialModeOverride   string
	SkipVoirDire        bool
	JurorCount          int
	MinimumConcurring   int
	UnanimousRequired   *bool
	PromptDir           string
	PromptFiles         map[string]string
}

type PlanningOptions struct {
	PromptDir   string
	PromptFiles map[string]string
}

func LoadComplaint(path string) (ComplaintInput, error) {
	return LoadSourceMarkdown(path)
}

func LoadSourceMarkdown(path string) (ComplaintInput, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ComplaintInput{}, fmt.Errorf("resolve markdown path: %w", err)
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return ComplaintInput{}, fmt.Errorf("read markdown: %w", err)
	}
	markdown := string(raw)
	if strings.TrimSpace(markdown) == "" {
		return ComplaintInput{}, fmt.Errorf("markdown is empty")
	}
	linked, err := extractLinkedFiles(markdown, filepath.Dir(absPath))
	if err != nil {
		return ComplaintInput{}, err
	}
	return ComplaintInput{
		OriginalPath: absPath,
		Markdown:     markdown,
		LinkedFiles:  linked,
	}, nil
}

func StageComplaintAssets(outDir string, in ComplaintInput) (ComplaintInput, error) {
	if strings.TrimSpace(outDir) == "" {
		return ComplaintInput{}, fmt.Errorf("outDir is required")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return ComplaintInput{}, fmt.Errorf("create out dir: %w", err)
	}
	staged := in
	complaintRel := "complaint.md"
	complaintDst := filepath.Join(outDir, complaintRel)
	if err := copyFile(staged.OriginalPath, complaintDst); err != nil {
		return ComplaintInput{}, fmt.Errorf("stage complaint: %w", err)
	}
	staged.StagedRelPath = complaintRel
	staged.StagedAbsPath = complaintDst

	inputDir := filepath.Join(outDir, "input-files")
	if len(staged.LinkedFiles) > 0 {
		if err := os.MkdirAll(inputDir, 0o755); err != nil {
			return ComplaintInput{}, fmt.Errorf("create input-files dir: %w", err)
		}
	}
	for i := range staged.LinkedFiles {
		name := staged.LinkedFiles[i].OriginalName
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if base == "" {
			base = "file"
		}
		stagedName := fmt.Sprintf("%02d-%s%s", i+1, slugify(base), filepath.Ext(name))
		rel := filepath.ToSlash(filepath.Join("input-files", stagedName))
		dst := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := copyFile(staged.LinkedFiles[i].OriginalPath, dst); err != nil {
			return ComplaintInput{}, fmt.Errorf("stage linked file %s: %w", staged.LinkedFiles[i].OriginalPath, err)
		}
		staged.LinkedFiles[i].StagedRelPath = rel
		staged.LinkedFiles[i].StagedAbsPath = dst
	}
	return staged, nil
}

func CreatePlan(ctx context.Context, client *openai.Client, plannerModel string, complaint ComplaintInput, court courts.Profile) (Plan, error) {
	return CreatePlanWithOptions(ctx, client, plannerModel, complaint, court, PlanningOptions{})
}

func CreatePlanWithOptions(ctx context.Context, client *openai.Client, plannerModel string, complaint ComplaintInput, court courts.Profile, opts PlanningOptions) (Plan, error) {
	if client == nil {
		return Plan{}, fmt.Errorf("planner client is nil")
	}
	model := strings.TrimSpace(plannerModel)
	if model == "" {
		return Plan{}, fmt.Errorf("planner model is required")
	}
	temp := 0.2
	promptCatalog, err := adcprompts.Load(adcprompts.Options{PromptDir: opts.PromptDir, PromptFiles: opts.PromptFiles})
	if err != nil {
		return Plan{}, err
	}

	packet, err := planCasePacket(ctx, client, model, complaint, court, &temp, promptCatalog)
	if err != nil {
		return Plan{}, err
	}
	plaintiffPrompt, err := buildStrategyPrompt(promptCatalog, adcprompts.PlaintiffStrategyUserID, packet, complaint, court)
	if err != nil {
		return Plan{}, err
	}
	plaintiffSystem, err := promptCatalog.Text(adcprompts.PlaintiffStrategySystemID)
	if err != nil {
		return Plan{}, err
	}

	plaintiffStrategy, err := planStrategyMemo(
		ctx,
		client,
		model,
		"plaintiff",
		plaintiffSystem,
		plaintiffPrompt,
		&temp,
	)
	if err != nil {
		return Plan{}, err
	}
	defensePrompt, err := buildStrategyPrompt(promptCatalog, adcprompts.DefenseStrategyUserID, packet, complaint, court)
	if err != nil {
		return Plan{}, err
	}
	defenseSystem, err := promptCatalog.Text(adcprompts.DefenseStrategySystemID)
	if err != nil {
		return Plan{}, err
	}
	defenseStrategy, err := planStrategyMemo(
		ctx,
		client,
		model,
		"defendant",
		defenseSystem,
		defensePrompt,
		&temp,
	)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		Packet:            packet,
		PlaintiffStrategy: plaintiffStrategy,
		DefenseStrategy:   defenseStrategy,
	}, nil
}

func BuildScenario(plan Plan, complaint ComplaintInput, opts ScenarioOptions) (spec.FormalScenario, error) {
	if err := opts.Court.Validate(); err != nil {
		return spec.FormalScenario{}, err
	}
	if err := validateCasePacket(plan.Packet, opts.Court); err != nil {
		return spec.FormalScenario{}, err
	}
	filedOn := strings.TrimSpace(opts.FiledOn)
	if filedOn == "" {
		filedOn = time.Now().UTC().Format("2006-01-02")
	}
	trialMode, err := resolveTrialMode(plan.Packet.TrialModeRecommendation, opts.TrialModeOverride)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	courtName := strings.TrimSpace(opts.Court.Name)
	judgeName := strings.TrimSpace(opts.JudgeName)
	if judgeName == "" {
		judgeName = defaultJudgeName
	}
	model := strings.TrimSpace(opts.RuntimeModel)
	if model == "" {
		return spec.FormalScenario{}, fmt.Errorf("runtime model is required")
	}
	plaintiffModel := strings.TrimSpace(opts.PlaintiffModel)
	if plaintiffModel == "" {
		return spec.FormalScenario{}, fmt.Errorf("plaintiff model is required")
	}
	defendantModel := strings.TrimSpace(opts.DefendantModel)
	if defendantModel == "" {
		return spec.FormalScenario{}, fmt.Errorf("defendant model is required")
	}
	judgeModel := strings.TrimSpace(opts.JudgeModel)
	if judgeModel == "" {
		return spec.FormalScenario{}, fmt.Errorf("judge model is required")
	}
	clerkModel := strings.TrimSpace(opts.ClerkModel)
	if clerkModel == "" {
		return spec.FormalScenario{}, fmt.Errorf("clerk model is required")
	}
	promptCatalog, err := adcprompts.Load(adcprompts.Options{PromptDir: opts.PromptDir, PromptFiles: opts.PromptFiles})
	if err != nil {
		return spec.FormalScenario{}, err
	}
	plaintiffInstructions, err := promptCatalog.Text(adcprompts.PlaintiffInstructionsID)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	plaintiffPreamble, err := promptCatalog.Render(adcprompts.PlaintiffRuntimeID, map[string]string{
		"{{COURT_RULES}}": strings.TrimSpace(opts.Court.RulesMarkdown),
		"{{STRATEGY}}":    strings.TrimSpace(plan.PlaintiffStrategy),
	})
	if err != nil {
		return spec.FormalScenario{}, err
	}
	defendantInstructions, err := promptCatalog.Text(adcprompts.DefendantInstructionsID)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	defendantPreamble, err := promptCatalog.Render(adcprompts.DefendantRuntimeID, map[string]string{
		"{{COURT_RULES}}": strings.TrimSpace(opts.Court.RulesMarkdown),
		"{{STRATEGY}}":    strings.TrimSpace(plan.DefenseStrategy),
	})
	if err != nil {
		return spec.FormalScenario{}, err
	}
	clerkInstructions, err := promptCatalog.Text(adcprompts.ClerkInstructionsID)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	clerkPreamble, err := promptCatalog.Render(adcprompts.ClerkRuntimeID, map[string]string{
		"{{COURT_RULES}}": strings.TrimSpace(opts.Court.RulesMarkdown),
	})
	if err != nil {
		return spec.FormalScenario{}, err
	}
	judgeInstructions, err := promptCatalog.Text(adcprompts.JudgeInstructionsID)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	judgePreamble, err := promptCatalog.Render(adcprompts.JudgeRuntimeID, map[string]string{
		"{{COURT_RULES}}": strings.TrimSpace(opts.Court.RulesMarkdown),
	})
	if err != nil {
		return spec.FormalScenario{}, err
	}

	roles := []spec.RoleSpec{
		{
			Name:           "plaintiff",
			Model:          plaintiffModel,
			Temperature:    opts.NonJurorTemperature,
			Instructions:   plaintiffInstructions,
			PromptPreamble: plaintiffPreamble,
			AllowedTools:   plaintiffTools(trialMode, opts.Court),
		},
		{
			Name:           "defendant",
			Model:          defendantModel,
			Temperature:    opts.NonJurorTemperature,
			Instructions:   defendantInstructions,
			PromptPreamble: defendantPreamble,
			AllowedTools:   defendantTools(trialMode, opts.Court),
		},
		{
			Name:           "clerk",
			Model:          clerkModel,
			Temperature:    opts.NonJurorTemperature,
			Instructions:   clerkInstructions,
			PromptPreamble: clerkPreamble,
			AllowedTools:   clerkTools(trialMode),
		},
		{
			Name:           "judge",
			Model:          judgeModel,
			Temperature:    opts.NonJurorTemperature,
			Instructions:   judgeInstructions,
			PromptPreamble: judgePreamble,
			AllowedTools:   judgeTools(trialMode, opts.Court),
		},
	}
	if trialMode == "jury" {
		jurorInstructions, err := promptCatalog.Text(adcprompts.JurorInstructionsID)
		if err != nil {
			return spec.FormalScenario{}, err
		}
		jurorPreamble, err := promptCatalog.Text(adcprompts.JurorRuntimeID)
		if err != nil {
			return spec.FormalScenario{}, err
		}
		roles = append(roles, spec.RoleSpec{
			Name:           "juror",
			Instructions:   jurorInstructions,
			PromptPreamble: jurorPreamble,
			AllowedTools: []string{
				"answer_juror_questionnaire",
				"answer_voir_dire_question",
				"submit_juror_vote",
			},
		})
	}

	caseInit, err := buildCaseInitialization(plan.Packet, complaint, filedOn, trialMode)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	assertions := []spec.AssertionSpec{
		{Type: "trial_mode", CaseIndex: 0, Equals: trialMode},
	}
	policy := map[string]any{}
	if opts.SkipVoirDire {
		policy["skip_voir_dire"] = 1
	}
	if opts.JurorCount > 0 {
		policy["jury_juror_count"] = opts.JurorCount
	}
	if opts.MinimumConcurring > 0 {
		policy["jury_minimum_concurring"] = opts.MinimumConcurring
	}
	if opts.UnanimousRequired != nil {
		if *opts.UnanimousRequired {
			policy["jury_unanimous_required"] = 1
		} else {
			policy["jury_unanimous_required"] = 0
		}
	}
	if trialMode == "jury" {
		assertions = append(assertions, spec.AssertionSpec{Type: "jury_outcome_recorded", CaseIndex: 0})
	} else {
		assertions = append(assertions,
			spec.AssertionSpec{Type: "case_status", CaseIndex: 0, Equals: "judgment_entered"},
			spec.AssertionSpec{Type: "decision_trace_contains_action", CaseIndex: 0, Action: "enter_judgment"},
			spec.AssertionSpec{Type: "judgment_count_min", CaseIndex: 0, MinCount: 1},
		)
	}

	return spec.FormalScenario{
		Name:        "go_case_" + slugify(strings.TrimSuffix(filepath.Base(complaint.OriginalPath), filepath.Ext(complaint.OriginalPath))),
		CourtName:   courtName,
		Court:       &opts.Court,
		Model:       model,
		Temperature: opts.Temperature,
		InitialCases: []map[string]any{
			{
				"caption":  plan.Packet.Caption,
				"judge":    judgeName,
				"filed_on": filedOn,
			},
		},
		Claims:   []spec.ClaimSpec{plan.Packet.Claim},
		Policy:   policy,
		CaseInit: caseInit,
		Roles:    roles,
		Turns:    nil,
		LoopPolicy: &spec.LoopPolicySpec{
			Type:             "autopilot_trial",
			MaxStepsPerTurn:  5,
			MaxTurns:         180,
			StopOnCaseStatus: "judgment_entered",
			StopCaseIndex:    0,
		},
		Assertions: assertions,
	}, nil
}

func plaintiffTools(trialMode string, _ courts.Profile) []string {
	tools := []string{
		"get_case",
		"list_case_files",
		"read_case_text_file",
		"request_case_file",
		"explain_decisions",
		"file_amended_complaint",
		"withdraw_or_correct_filing",
		"serve_initial_disclosures",
		"serve_interrogatories",
		"serve_request_for_production",
		"serve_requests_for_admission",
		"file_rule37_motion",
		"oppose_rule12_motion",
		"oppose_rule56_motion",
		"import_case_file",
		"produce_case_file",
		"submit_technical_report",
		"record_opening_statement",
		"submit_trial_theory",
		"offer_case_file_as_exhibit",
		"rest_case",
		"offer_exhibit",
		"deliver_closing_argument",
	}
	if trialMode == "jury" {
		tools = append(tools,
			"get_juror_context",
			"record_voir_dire_question",
			"challenge_juror_for_cause",
			"strike_juror_peremptorily",
			"propose_jury_instruction",
			"object_jury_instruction",
		)
	}
	return tools
}

func defendantTools(trialMode string, _ courts.Profile) []string {
	tools := []string{
		"get_case",
		"list_case_files",
		"read_case_text_file",
		"request_case_file",
		"explain_decisions",
		"file_answer",
		"file_rule12_motion",
		"reply_rule12_motion",
		"serve_rule11_safe_harbor_notice",
		"file_rule11_motion",
		"serve_initial_disclosures",
		"respond_interrogatories",
		"respond_request_for_production",
		"respond_requests_for_admission",
		"file_rule56_motion",
		"reply_rule56_motion",
		"import_case_file",
		"produce_case_file",
		"submit_technical_report",
		"record_opening_statement",
		"submit_trial_theory",
		"offer_case_file_as_exhibit",
		"rest_case",
		"offer_exhibit",
		"deliver_closing_argument",
	}
	if trialMode == "jury" {
		tools = append(tools,
			"get_juror_context",
			"record_voir_dire_question",
			"challenge_juror_for_cause",
			"strike_juror_peremptorily",
			"propose_jury_instruction",
			"object_jury_instruction",
		)
	}
	return tools
}

func clerkTools(trialMode string) []string {
	tools := []string{"get_case", "list_case_files", "read_case_text_file", "request_case_file", "set_last_pleading_served_on"}
	if trialMode == "jury" {
		tools = append(tools, "record_jury_demand", "set_jury_configuration", "add_juror")
	}
	return tools
}

func judgeTools(trialMode string, court courts.Profile) []string {
	tools := []string{
		"get_case",
		"list_case_files",
		"read_case_text_file",
		"request_case_file",
		"explain_decisions",
		"get_juror_context",
		"resolve_trial_mode",
		"transition_case",
		"decide_rule11_motion",
		"decide_rule12_motion",
		"decide_rule37_motion",
		"decide_rule56_motion",
		"enter_pretrial_order",
		"advance_trial_phase",
		"enter_judgment",
	}
	if court.JurisdictionScreen {
		tools = append(tools, "dismiss_for_lack_of_subject_matter_jurisdiction")
	}
	if trialMode == "jury" {
		tools = append(tools, "issue_juror_questionnaire", "decide_voir_dire_question", "decide_juror_for_cause_challenge", "empanel_jury", "settle_jury_instructions", "deliver_jury_instructions")
	} else {
		tools = append(tools, "file_bench_opinion")
	}
	return tools
}

func buildCaseInitialization(packet CasePacket, complaint ComplaintInput, filedOn string, trialMode string) (*spec.CaseInitializationSpec, error) {
	attachments := make([]spec.ComplaintAttachmentSpec, 0, len(complaint.LinkedFiles))
	for i, linked := range complaint.LinkedFiles {
		if strings.TrimSpace(linked.StagedRelPath) == "" {
			return nil, fmt.Errorf("linked file %s missing staged path", linked.OriginalName)
		}
		stagedPath := linked.StagedAbsPath
		if strings.TrimSpace(stagedPath) == "" {
			return nil, fmt.Errorf("linked file %s missing staged absolute path", linked.OriginalName)
		}
		raw, err := os.ReadFile(stagedPath)
		if err != nil {
			return nil, fmt.Errorf("read staged linked file %s: %w", stagedPath, err)
		}
		digest := sha256.Sum256(raw)
		attachments = append(attachments, spec.ComplaintAttachmentSpec{
			FileID:         fmt.Sprintf("file-%04d", i+1),
			Label:          strings.TrimSpace(linked.Label),
			OriginalName:   strings.TrimSpace(linked.OriginalName),
			StorageRelPath: filepath.ToSlash(linked.StagedRelPath),
			Sha256:         hex.EncodeToString(digest[:]),
			SizeBytes:      len(raw),
		})
	}
	caseInit := &spec.CaseInitializationSpec{
		ComplaintSummary:         strings.TrimSpace(packet.ComplaintSummary),
		FiledBy:                  "plaintiff",
		JurisdictionBasis:        strings.TrimSpace(packet.JurisdictionBasis),
		JurisdictionalStatement:  strings.TrimSpace(packet.JurisdictionalStatement),
		InjuryStatement:          strings.TrimSpace(packet.InjuryStatement),
		CausationStatement:       strings.TrimSpace(packet.CausationStatement),
		RedressabilityStatement:  strings.TrimSpace(packet.RedressabilityStatement),
		RipenessStatement:        strings.TrimSpace(packet.RipenessStatement),
		LiveControversyStatement: strings.TrimSpace(packet.LiveControversyStatement),
		PlaintiffCitizenship:     strings.TrimSpace(packet.PlaintiffCitizenship),
		DefendantCitizenship:     strings.TrimSpace(packet.DefendantCitizenship),
		AmountInControversy:      strings.TrimSpace(packet.AmountInControversy),
		Attachments:              attachments,
	}
	if trialMode == "jury" {
		caseInit.JuryDemandedOn = filedOn
	}
	return caseInit, nil
}

func buildAnswerSummary(packet CasePacket) string {
	parts := []string{"Defendant denies liability and demands strict proof of every required element."}
	if len(packet.Claim.Defenses) > 0 {
		parts = append(parts, "Defendant asserts defenses including "+commaJoin(packet.Claim.Defenses)+".")
	}
	if strings.TrimSpace(packet.RequestedRelief) != "" {
		parts = append(parts, "Defendant disputes entitlement to the requested relief.")
	}
	return strings.Join(parts, " ")
}

func buildCasePacketPrompt(complaint ComplaintInput, court courts.Profile) (string, error) {
	promptCatalog, err := adcprompts.Load(adcprompts.Options{})
	if err != nil {
		return "", err
	}
	return buildCasePacketPromptWithCatalog(promptCatalog, complaint, court)
}

func buildCasePacketPromptWithCatalog(promptCatalog *adcprompts.Catalog, complaint ComplaintInput, court courts.Profile) (string, error) {
	courtContext, err := renderCourtContext(court)
	if err != nil {
		return "", err
	}
	return promptCatalog.Render(adcprompts.CasePacketUserID, map[string]string{
		"{{COURT}}":        courtContext,
		"{{COMPLAINT}}":    complaint.Markdown,
		"{{LINKED_FILES}}": renderLinkedFileContext(complaint.LinkedFiles),
	})
}

func buildStrategyPrompt(promptCatalog *adcprompts.Catalog, promptID string, packet CasePacket, complaint ComplaintInput, court courts.Profile) (string, error) {
	packetJSON, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode case packet: %w", err)
	}
	courtContext, err := renderCourtContext(court)
	if err != nil {
		return "", err
	}
	plaintiffToolList := plaintiffTools(packet.TrialModeRecommendation, court)
	defendantToolList := defendantTools(packet.TrialModeRecommendation, court)
	return promptCatalog.Render(promptID, map[string]string{
		"{{COURT}}":           courtContext,
		"{{CASE_PACKET}}":     string(packetJSON),
		"{{COMPLAINT}}":       complaint.Markdown,
		"{{LINKED_FILES}}":    renderLinkedFileContext(complaint.LinkedFiles),
		"{{PLAINTIFF_TOOLS}}": strings.Join(plaintiffToolList, ", "),
		"{{DEFENDANT_TOOLS}}": strings.Join(defendantToolList, ", "),
	})
}

func planCasePacket(
	ctx context.Context,
	client *openai.Client,
	model string,
	complaint ComplaintInput,
	court courts.Profile,
	temperature *float64,
	promptCatalog *adcprompts.Catalog,
) (CasePacket, error) {
	prompt, err := buildCasePacketPromptWithCatalog(promptCatalog, complaint, court)
	if err != nil {
		return CasePacket{}, err
	}
	systemPrompt, err := promptCatalog.Text(adcprompts.CasePacketSystemID)
	if err != nil {
		return CasePacket{}, err
	}
	baseMessages := []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": prompt},
	}
	messages := append([]map[string]any(nil), baseMessages...)
	for attempt := 1; attempt <= maxPlannerAttempts; attempt++ {
		resp, err := client.CreateResponse(ctx, model, messages, nil, "", temperature)
		if err != nil {
			return CasePacket{}, fmt.Errorf("plan case packet: %w", err)
		}
		packet, err := parseCasePacket(resp.Text)
		if err == nil {
			if strings.TrimSpace(packet.Error) != "" {
				return CasePacket{}, fmt.Errorf("case packet planner returned error: %s", strings.TrimSpace(packet.Error))
			}
			if validateErr := validateCasePacket(packet, court); validateErr == nil {
				return packet, nil
			} else {
				err = validateErr
			}
		}
		if attempt == maxPlannerAttempts {
			return CasePacket{}, fmt.Errorf("plan case packet: %w", err)
		}
		fmt.Fprintf(
			os.Stderr,
			"planner correction kind=case_packet attempt=%d/%d reason=%s\n",
			attempt,
			maxPlannerAttempts,
			err.Error(),
		)
		correctionPrompt, renderErr := promptCatalog.Render(adcprompts.CasePacketRepairID, map[string]string{"{{ERROR}}": err.Error()})
		if renderErr != nil {
			return CasePacket{}, renderErr
		}
		messages = append(
			append([]map[string]any(nil), baseMessages...),
			map[string]any{"role": "assistant", "content": strings.TrimSpace(resp.Text)},
			map[string]any{"role": "user", "content": correctionPrompt},
		)
	}
	return CasePacket{}, fmt.Errorf("plan case packet: exhausted planner attempts")
}

func planStrategyMemo(
	ctx context.Context,
	client *openai.Client,
	model string,
	side string,
	systemPrompt string,
	basePrompt string,
	temperature *float64,
) (string, error) {
	messages := []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": basePrompt},
	}
	resp, err := client.CreateResponse(ctx, model, messages, nil, "", temperature)
	if err != nil {
		return "", fmt.Errorf("plan %s strategy: %w", side, err)
	}
	return strings.TrimSpace(resp.Text), nil
}

func renderCourtContext(court courts.Profile) (string, error) {
	profileJSON, err := json.MarshalIndent(court, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode court profile: %w", err)
	}
	return string(profileJSON), nil
}

var amountTokenPattern = regexp.MustCompile(`([0-9][0-9,]*(?:\.[0-9]{1,2})?)`)

func parseAmountString(raw string) (int, bool) {
	match := amountTokenPattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return 0, false
	}
	whole := strings.SplitN(match[1], ".", 2)[0]
	whole = strings.ReplaceAll(whole, ",", "")
	value, err := strconv.Atoi(whole)
	if err != nil {
		return 0, false
	}
	return value, true
}

func formatWholeDollarAmount(value int) string {
	if value <= 0 {
		return "$0"
	}
	raw := strconv.Itoa(value)
	chunks := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		chunks = append([]string{raw[len(raw)-3:]}, chunks...)
		raw = raw[:len(raw)-3]
	}
	chunks = append([]string{raw}, chunks...)
	return "$" + strings.Join(chunks, ",")
}

func renderLinkedFileContext(files []LinkedFile) string {
	if len(files) == 0 {
		return "(none)\n"
	}
	var b strings.Builder
	for _, file := range files {
		b.WriteString("- label: ")
		b.WriteString(file.Label)
		b.WriteString("\n")
		b.WriteString("  original_name: ")
		b.WriteString(file.OriginalName)
		b.WriteString("\n")
		if strings.TrimSpace(file.ReferencePath) != "" {
			b.WriteString("  reference_path: ")
			b.WriteString(file.ReferencePath)
			b.WriteString("\n")
		}
		if strings.TrimSpace(file.StagedRelPath) != "" {
			b.WriteString("  staged_path: ")
			b.WriteString(file.StagedRelPath)
			b.WriteString("\n")
		}
		b.WriteString("  preview_kind: ")
		b.WriteString(file.PreviewKind)
		b.WriteString("\n")
		if strings.TrimSpace(file.Preview) != "" {
			b.WriteString("  preview:\n")
			b.WriteString(indentLines(file.Preview, "    "))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func parseCasePacket(raw string) (CasePacket, error) {
	var packet CasePacket
	text := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(text), &packet); err != nil {
		return CasePacket{}, fmt.Errorf("expected strict JSON object from planner: %w; response=%q", err, truncateForError(text))
	}
	packet = normalizeCasePacket(packet)
	return packet, nil
}

func normalizeCasePacket(packet CasePacket) CasePacket {
	packet.Error = strings.TrimSpace(packet.Error)
	packet.Caption = strings.TrimSpace(packet.Caption)
	packet.PlaintiffName = strings.TrimSpace(packet.PlaintiffName)
	packet.DefendantName = strings.TrimSpace(packet.DefendantName)
	packet.ComplaintSummary = oneLine(packet.ComplaintSummary)
	packet.RequestedRelief = oneLine(packet.RequestedRelief)
	packet.TrialModeRecommendation = strings.ToLower(strings.TrimSpace(packet.TrialModeRecommendation))
	packet.JurisdictionBasis = strings.ToLower(strings.TrimSpace(packet.JurisdictionBasis))
	packet.JurisdictionalStatement = oneLine(packet.JurisdictionalStatement)
	packet.InjuryStatement = oneLine(packet.InjuryStatement)
	packet.CausationStatement = oneLine(packet.CausationStatement)
	packet.RedressabilityStatement = oneLine(packet.RedressabilityStatement)
	packet.RipenessStatement = oneLine(packet.RipenessStatement)
	packet.LiveControversyStatement = oneLine(packet.LiveControversyStatement)
	packet.PlaintiffCitizenship = oneLine(packet.PlaintiffCitizenship)
	packet.DefendantCitizenship = oneLine(packet.DefendantCitizenship)
	packet.AmountInControversy = oneLine(packet.AmountInControversy)
	packet.Claim.ClaimID = strings.TrimSpace(packet.Claim.ClaimID)
	packet.Claim.Label = strings.TrimSpace(packet.Claim.Label)
	packet.Claim.LegalTheory = strings.TrimSpace(packet.Claim.LegalTheory)
	packet.Claim.StandardOfProof = strings.TrimSpace(packet.Claim.StandardOfProof)
	packet.Claim.BurdenHolder = strings.TrimSpace(packet.Claim.BurdenHolder)
	packet.Claim.DamagesQuestion = oneLine(packet.Claim.DamagesQuestion)
	packet.Claim.Elements = compactStrings(packet.Claim.Elements)
	packet.Claim.Defenses = compactStrings(packet.Claim.Defenses)
	return packet
}

func validateCasePacket(packet CasePacket, court courts.Profile) error {
	if packet.Caption == "" {
		return fmt.Errorf("case packet missing caption")
	}
	if packet.PlaintiffName == "" {
		return fmt.Errorf("case packet missing plaintiff_name")
	}
	if packet.DefendantName == "" {
		return fmt.Errorf("case packet missing defendant_name")
	}
	if packet.ComplaintSummary == "" {
		return fmt.Errorf("case packet missing complaint_summary")
	}
	if packet.RequestedRelief == "" {
		return fmt.Errorf("case packet missing requested_relief")
	}
	if packet.TrialModeRecommendation != "jury" && packet.TrialModeRecommendation != "bench" {
		return fmt.Errorf("case packet trial_mode_recommendation must be jury or bench")
	}
	if !court.AllowsJurisdictionBasis(packet.JurisdictionBasis) {
		return fmt.Errorf("case packet jurisdiction_basis %q is not allowed in %s", packet.JurisdictionBasis, court.Name)
	}
	if court.RequireJurisdictionStatement && packet.JurisdictionalStatement == "" {
		return fmt.Errorf("case packet missing jurisdictional_statement")
	}
	if packet.InjuryStatement == "" {
		return fmt.Errorf("case packet missing injury_statement")
	}
	if packet.CausationStatement == "" {
		return fmt.Errorf("case packet missing causation_statement")
	}
	if packet.RedressabilityStatement == "" {
		return fmt.Errorf("case packet missing redressability_statement")
	}
	if packet.RipenessStatement == "" {
		return fmt.Errorf("case packet missing ripeness_statement")
	}
	if packet.LiveControversyStatement == "" {
		return fmt.Errorf("case packet missing live_controversy_statement")
	}
	if packet.JurisdictionBasis == "diversity" && court.RequireDiversityCitizenship {
		if packet.PlaintiffCitizenship == "" {
			return fmt.Errorf("case packet missing plaintiff_citizenship for diversity jurisdiction")
		}
		if packet.DefendantCitizenship == "" {
			return fmt.Errorf("case packet missing defendant_citizenship for diversity jurisdiction")
		}
	}
	if packet.JurisdictionBasis == "diversity" && court.RequireAmountInControversy {
		if packet.AmountInControversy == "" {
			return fmt.Errorf("case packet missing amount_in_controversy for diversity jurisdiction")
		}
		amount, ok := parseAmountString(packet.AmountInControversy)
		if !ok {
			return fmt.Errorf("case packet amount_in_controversy must contain a dollar amount for diversity jurisdiction")
		}
		if amount <= court.MinimumAmountInControversy {
			return fmt.Errorf("case packet amount_in_controversy must exceed %s for diversity jurisdiction in %s", formatWholeDollarAmount(court.MinimumAmountInControversy), court.Name)
		}
	}
	if packet.Claim.ClaimID == "" {
		return fmt.Errorf("case packet missing claim.claim_id")
	}
	if packet.Claim.Label == "" {
		return fmt.Errorf("case packet missing claim.label")
	}
	if packet.Claim.LegalTheory == "" {
		return fmt.Errorf("case packet missing claim.legal_theory")
	}
	if len(packet.Claim.LegalTheory) > 80 {
		return fmt.Errorf("case packet claim.legal_theory is too long")
	}
	if packet.Claim.StandardOfProof == "" {
		return fmt.Errorf("case packet missing claim.standard_of_proof")
	}
	if packet.Claim.BurdenHolder == "" {
		return fmt.Errorf("case packet missing claim.burden_holder")
	}
	if len(packet.Claim.Elements) == 0 {
		return fmt.Errorf("case packet missing claim.elements")
	}
	for _, element := range packet.Claim.Elements {
		if len(element) > 120 {
			return fmt.Errorf("case packet claim element is too long: %q", element)
		}
	}
	for _, defense := range packet.Claim.Defenses {
		if len(defense) > 120 {
			return fmt.Errorf("case packet claim defense is too long: %q", defense)
		}
	}
	if packet.Claim.DamagesQuestion == "" {
		return fmt.Errorf("case packet missing claim.damages_question")
	}
	return nil
}

func resolveTrialMode(recommended string, override string) (string, error) {
	override = strings.ToLower(strings.TrimSpace(override))
	if override == "" || override == "auto" {
		recommended = strings.ToLower(strings.TrimSpace(recommended))
		if recommended == "jury" || recommended == "bench" {
			return recommended, nil
		}
		return "", fmt.Errorf("planner trial mode recommendation must be jury or bench")
	}
	if override != "jury" && override != "bench" {
		return "", fmt.Errorf("trial mode override must be auto, jury, or bench")
	}
	return override, nil
}

func extractLinkedFiles(markdown string, baseDir string) ([]LinkedFile, error) {
	matches := markdownLinkPattern.FindAllStringSubmatch(markdown, -1)
	files := make([]LinkedFile, 0)
	seen := map[string]bool{}
	for _, match := range matches {
		label := strings.TrimSpace(match[1])
		target := strings.TrimSpace(match[2])
		if ignoreLinkTarget(target) {
			continue
		}
		resolved, err := resolveMarkdownLink(baseDir, target)
		if err != nil {
			return nil, err
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("stat linked file %s: %w", resolved, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("linked path is a directory: %s", resolved)
		}
		previewKind, preview, err := previewFile(resolved)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(resolved)
		if label == "" {
			label = name
		}
		files = append(files, LinkedFile{
			Label:         label,
			ReferencePath: target,
			OriginalPath:  resolved,
			OriginalName:  name,
			PreviewKind:   previewKind,
			Preview:       preview,
		})
	}
	return files, nil
}

func ignoreLinkTarget(target string) bool {
	target = strings.TrimSpace(target)
	return target == "" ||
		strings.HasPrefix(target, "#") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.Contains(target, "://")
}

func resolveMarkdownLink(baseDir string, target string) (string, error) {
	target = strings.TrimSpace(strings.Trim(target, "<>"))
	candidates := []string{target}
	if i := strings.LastIndex(target, "\""); i > 0 && strings.HasSuffix(target, "\"") {
		candidates = append(candidates, strings.TrimSpace(target[:i]))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path := candidate
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, candidate)
		}
		if _, err := os.Stat(path); err == nil {
			abs, absErr := filepath.Abs(path)
			if absErr != nil {
				return "", fmt.Errorf("resolve linked file path %s: %w", path, absErr)
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("linked file not found from markdown target %q", target)
}

func previewFile(path string) (kind string, text string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("open linked file %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close linked file %s: %w", path, closeErr))
		}
	}()
	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", "", fmt.Errorf("read linked file %s: %w", path, err)
	}
	buf = buf[:n]
	if len(buf) == 0 {
		return "empty", "", nil
	}
	if !utf8.Valid(buf) {
		return "binary", "", nil
	}
	text = string(buf)
	text = strings.TrimSpace(text)
	info, err := f.Stat()
	if err != nil {
		return "", "", fmt.Errorf("stat linked file %s: %w", path, err)
	}
	if info.Size() > int64(len(buf)) {
		text += "\n[truncated]"
	}
	return "text_excerpt", text, nil
}

func copyFile(src string, dst string) (err error) {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if filepath.Clean(srcAbs) == filepath.Clean(dstAbs) {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close source file %s: %w", src, closeErr))
		}
	}()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		if copyErr != nil {
			err = errors.Join(err, fmt.Errorf("copy %s to %s: %w", src, dst, copyErr))
		}
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close destination file %s: %w", dst, closeErr))
		}
		if removeErr := os.Remove(dst); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove incomplete destination file %s: %w", dst, removeErr))
		}
		return err
	}
	return nil
}

func truncateForError(s string) string {
	if len(s) <= 240 {
		return s
	}
	return s[:240]
}

func excerptAround(s string, needle string) string {
	idx := strings.Index(s, needle)
	if idx < 0 {
		return truncateForError(s)
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(needle) + 40
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

func compactStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func commaJoin(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "case"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "case"
	}
	return out
}

func indentLines(s string, prefix string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
