package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/adc/runtime/store"
	"github.com/agentcourt/adj/common/casemanifest"
	"github.com/agentcourt/adj/common/councilsample"
	"github.com/agentcourt/adj/common/openai"
)

type Config struct {
	ScenarioPath            string
	ScenarioBaseDir         string
	OutputPath              string
	EventsPath              string
	RunID                   string
	CaseID                  string
	CaseAPIAddr             string
	ExternalRoles           []string
	Model                   string
	Temperature             *float64
	JurorTemperature        *float64
	JurorPersonasPath       string
	CouncilAllowedEndpoints []string
	CouncilMinEndpoints     int
	Offline                 bool
	Runtime                 RuntimeLimits
	PolicyOverrides         map[string]any
	PromptDir               string
	PromptFiles             map[string]string
}

type TurnLog struct {
	External           bool             `json:"external,omitempty"`
	Source             string           `json:"source,omitempty"`
	ActionID           string           `json:"action_id,omitempty"`
	OpportunityID      string           `json:"opportunity_id,omitempty"`
	OpportunityPhase   string           `json:"opportunity_phase,omitempty"`
	OpportunityKind    string           `json:"opportunity_kind,omitempty"`
	OpportunityMessage string           `json:"opportunity_message,omitempty"`
	MayPass            bool             `json:"may_pass,omitempty"`
	Role               string           `json:"role"`
	Prompt             string           `json:"prompt"`
	Steps              int              `json:"steps"`
	Transcript         []map[string]any `json:"transcript"`
}

type Result struct {
	Scenario   string            `json:"scenario"`
	Assertions []map[string]any  `json:"assertions"`
	TurnLogs   []TurnLog         `json:"turn_logs"`
	FinalState map[string]any    `json:"final_state"`
	Provider   openai.Accounting `json:"provider"`
}

type ActionExecution struct {
	Result             map[string]any
	FollowupInputItems []map[string]any
}

type Runner struct {
	scenario                spec.FormalScenario
	lean                    lean.Engine
	store                   *store.Store
	client                  ResponseClient
	jurorClient             ResponseClient
	sharedClientAccounting  bool
	allowNoStorePersistence bool
	cfg                     Config
	state                   map[string]any
	lastOpportunityView     map[string]any
	roles                   map[string]spec.RoleSpec
	courtProfile            courts.Profile
	certificateInit         ReplayInitializeRequest
	certificateTransitions  []ReplayTransition
	workProductDirs         map[string]string
	jurorPersonaPool        *jurorPersonaPool
	jurorPersonaAssignments map[string]jurorPersonaPair
	externalRoles           map[string]bool
	roleAPI                 *roleAPIServer
	prompts                 *adcprompts.Catalog
	promptRenderer          *PromptRenderer
	schemaDescriptions      map[string]map[string]string
}

func (r *Runner) RequiresLLMTurns() bool {
	if r.scenario.LoopPolicy != nil && strings.TrimSpace(r.scenario.LoopPolicy.Type) == "autopilot_trial" {
		return true
	}
	for _, turn := range r.scenario.Turns {
		if turn.DeterministicAction == nil {
			return true
		}
	}
	return false
}

func New(st *store.Store, le lean.Engine, client ResponseClient, jurorClient ResponseClient, cfg Config) (*Runner, error) {
	if st == nil {
		return nil, fmt.Errorf("runner store is nil")
	}
	scenario, err := spec.Load(cfg.ScenarioPath)
	if err != nil {
		return nil, err
	}
	applyPolicyOverrides(&scenario, cfg.PolicyOverrides)
	if strings.TrimSpace(cfg.ScenarioBaseDir) == "" {
		cfg.ScenarioBaseDir = filepath.Dir(cfg.ScenarioPath)
	}
	courtProfile, err := resolveScenarioCourtProfile(scenario)
	if err != nil {
		return nil, err
	}
	r, err := newRunnerCore(
		st,
		le,
		client,
		jurorClient,
		client != nil && client == jurorClient,
		cfg,
		scenario.Roles,
		courtProfile,
		false,
	)
	if err != nil {
		return nil, err
	}
	r.scenario = scenario
	if strings.TrimSpace(cfg.JurorPersonasPath) != "" {
		pool, err := loadJurorPersonaPoolWithOptions(cfg.JurorPersonasPath, cfg.ScenarioBaseDir, councilsample.Options{
			AllowedEndpoints:         cfg.CouncilAllowedEndpoints,
			MinimumDistinctEndpoints: cfg.CouncilMinEndpoints,
		})
		if err != nil {
			return nil, err
		}
		r.jurorPersonaPool = pool
	}
	if err := validateScenarioActions(scenario, r.roles); err != nil {
		return nil, err
	}
	initialState := buildInitialState(scenario, courtProfile)
	certificateInit, err := newReplayInitializeRequest(initialState)
	if err != nil {
		return nil, err
	}
	r.certificateInit = certificateInit
	r.state = initialState
	if scenario.CaseInit != nil {
		initReq := replayInitializeCaseRequest(*scenario.CaseInit)
		r.certificateInit.InitializeCase = &initReq
		state, err := initializeSeededCase(le, r.state, initReq)
		if err != nil {
			return nil, err
		}
		r.state = state
	}
	return r, nil
}

func applyPolicyOverrides(scenario *spec.FormalScenario, overrides map[string]any) {
	if len(overrides) == 0 {
		return
	}
	if scenario.Policy == nil {
		scenario.Policy = map[string]any{}
	}
	for key, value := range overrides {
		scenario.Policy[key] = value
	}
}

func resolveScenarioCourtProfile(scenario spec.FormalScenario) (courts.Profile, error) {
	if scenario.Court != nil {
		return *scenario.Court, nil
	}
	return courts.Resolve(strings.TrimSpace(scenario.CourtName))
}

func replayInitializeCaseRequest(init spec.CaseInitializationSpec) ReplayInitializeCaseRequest {
	attachments := make([]map[string]any, 0, len(init.Attachments))
	for _, attachment := range init.Attachments {
		attachments = append(attachments, map[string]any{
			"file_id":         attachment.FileID,
			"label":           attachment.Label,
			"original_name":   attachment.OriginalName,
			"storage_relpath": attachment.StorageRelPath,
			"sha256":          attachment.Sha256,
			"size_bytes":      attachment.SizeBytes,
		})
	}
	jurisdictionalAllegations := map[string]any{}
	addString := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			jurisdictionalAllegations[key] = strings.TrimSpace(value)
		}
	}
	addString("jurisdiction_basis", init.JurisdictionBasis)
	addString("jurisdictional_statement", init.JurisdictionalStatement)
	addString("injury_statement", init.InjuryStatement)
	addString("causation_statement", init.CausationStatement)
	addString("redressability_statement", init.RedressabilityStatement)
	addString("ripeness_statement", init.RipenessStatement)
	addString("live_controversy_statement", init.LiveControversyStatement)
	addString("plaintiff_citizenship", init.PlaintiffCitizenship)
	addString("defendant_citizenship", init.DefendantCitizenship)
	addString("amount_in_controversy", init.AmountInControversy)
	return ReplayInitializeCaseRequest{
		ComplaintSummary:          strings.TrimSpace(init.ComplaintSummary),
		FiledBy:                   strings.TrimSpace(init.FiledBy),
		JuryDemandedOn:            strings.TrimSpace(init.JuryDemandedOn),
		JurisdictionalAllegations: jurisdictionalAllegations,
		Attachments:               attachments,
	}
}

func initializeSeededCase(le lean.Engine, state map[string]any, init ReplayInitializeCaseRequest) (map[string]any, error) {
	resp, err := le.InitializeCase(
		state,
		init.ComplaintSummary,
		init.FiledBy,
		init.JuryDemandedOn,
		init.JurisdictionalAllegations,
		init.Attachments,
	)
	if err != nil {
		return nil, fmt.Errorf("lean initialize_case failed: %w", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return nil, fmt.Errorf("lean initialize_case error: %s", marshalString(resp))
	}
	nextState, _ := resp["state"].(map[string]any)
	if nextState == nil {
		return nil, fmt.Errorf("lean initialize_case missing state")
	}
	return nextState, nil
}

func loadRolePromptPreambles(roles map[string]spec.RoleSpec, scenarioBaseDir string) error {
	for roleName, role := range roles {
		path := strings.TrimSpace(role.PromptPreambleFile)
		if path == "" {
			continue
		}
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(scenarioBaseDir, path)
		}
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("read prompt preamble file role=%s path=%s: %w", roleName, path, err)
		}
		fileText := strings.TrimSpace(string(raw))
		if fileText == "" {
			continue
		}
		if strings.TrimSpace(role.PromptPreamble) == "" {
			role.PromptPreamble = fileText
		} else {
			role.PromptPreamble = strings.TrimSpace(role.PromptPreamble) + "\n\n" + fileText
		}
		roles[roleName] = role
	}
	return nil
}

func validateScenarioActions(scenario spec.FormalScenario, roles map[string]spec.RoleSpec) error {
	missing := make([]string, 0)
	seen := map[string]bool{}
	for _, role := range scenario.Roles {
		for _, action := range role.EffectiveAllowedActions() {
			action = strings.TrimSpace(action)
			if action == "" || seen[action] || toolSchema(action) != nil {
				continue
			}
			missing = append(missing, action)
			seen[action] = true
		}
	}
	for i, turn := range scenario.Turns {
		role, ok := roles[turn.Role]
		if !ok {
			return fmt.Errorf("turn %d uses unknown role: %s", i+1, turn.Role)
		}
		if turn.DeterministicAction != nil {
			action := strings.TrimSpace(turn.DeterministicAction.ActionType)
			if action == "" {
				return fmt.Errorf("turn %d deterministic action missing action_type", i+1)
			}
			if toolSchema(action) == nil && !seen[action] {
				missing = append(missing, action)
				seen[action] = true
			}
			continue
		}
		for _, action := range turn.EffectiveAllowedActions(role) {
			if toolSchema(action) != nil || seen[action] {
				continue
			}
			missing = append(missing, action)
			seen[action] = true
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"scenario contains actions not implemented in go runner: %s",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func (r *Runner) Run(ctx context.Context) (result Result, err error) {
	if r == nil {
		return Result{}, fmt.Errorf("runner is nil")
	}
	if r.store == nil {
		return Result{}, fmt.Errorf("runner store is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	startedAt := time.Now().UTC()
	manifest := casemanifest.New(casemanifest.ProcedureADC, r.cfg.CaseID, r.cfg.RunID, startedAt)
	if err := r.writeCaseManifest(manifest); err != nil {
		return Result{}, err
	}
	var api *caseAPIServer
	if strings.TrimSpace(r.cfg.CaseAPIAddr) != "" {
		api, err = startCaseAPIServer(r)
		if err != nil {
			return Result{}, err
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err = errors.Join(err, api.Close(shutdownCtx))
		}()
		manifest.CaseAPIBase = api.baseURL
		if err := r.writeCaseManifest(manifest); err != nil {
			return Result{}, err
		}
	}
	defer func() {
		if r.roleAPI != nil {
			r.roleAPI.setTerminal(result, err)
		}
	}()
	if err := resetEventLog(r.cfg.EventsPath); err != nil {
		return Result{}, err
	}
	if err := r.writeEvidenceManifest(); err != nil {
		return Result{}, err
	}
	if err := r.store.CreateRun(r.cfg.RunID, r.scenario.Name); err != nil {
		return Result{}, err
	}
	turnLogs := make([]TurnLog, 0, len(r.scenario.Turns)+16)
	for i, turn := range r.scenario.Turns {
		role, ok := r.roles[turn.Role]
		if !ok {
			return Result{}, fmt.Errorf("unknown role: %s", turn.Role)
		}
		allowed := turn.EffectiveAllowedActions(role)
		if len(allowed) == 0 {
			return Result{}, fmt.Errorf("turn role=%s has no allowed actions", turn.Role)
		}
		log, err := r.executeTurn(ctx, i+1, role, turn, allowed)
		if err != nil {
			return Result{}, err
		}
		turnLogs = append(turnLogs, log)
	}
	if r.scenario.LoopPolicy != nil && strings.TrimSpace(r.scenario.LoopPolicy.Type) == "autopilot_trial" {
		logs, err := r.runAutopilot(ctx, len(turnLogs)+1)
		if err != nil {
			return Result{}, err
		}
		turnLogs = append(turnLogs, logs...)
	}
	assertions := evaluateAssertions(r.scenario.Assertions, r.state, turnLogs)
	result = Result{
		Scenario:   r.scenario.Name,
		Assertions: assertions,
		TurnLogs:   turnLogs,
		FinalState: r.state,
		Provider:   r.ProviderAccounting(),
	}
	if err := r.writeEvidence(result); err != nil {
		return Result{}, err
	}
	status := "ok"
	for _, a := range assertions {
		if passed, _ := a["passed"].(bool); !passed {
			status = "assertion_failed"
			break
		}
	}
	evidenceMap, err := resultMap(result)
	if err != nil {
		return Result{}, err
	}
	if err := r.store.FinishRun(r.cfg.RunID, status, r.state, evidenceMap); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r *Runner) ProviderAccounting() openai.Accounting {
	if r == nil {
		return openai.Accounting{}
	}
	if r.sharedClientAccounting {
		if r.client == nil {
			return openai.Accounting{}
		}
		return r.client.Accounting()
	}
	accounting := make([]openai.Accounting, 0, 2)
	if r.client != nil {
		accounting = append(accounting, r.client.Accounting())
	}
	if r.jurorClient != nil {
		accounting = append(accounting, r.jurorClient.Accounting())
	}
	return openai.MergeAccounting(accounting...)
}

func (r *Runner) writeCaseManifest(manifest casemanifest.Manifest) error {
	if strings.TrimSpace(r.cfg.OutputPath) == "" {
		return nil
	}
	if err := casemanifest.WriteAtomic(filepath.Dir(r.cfg.OutputPath), manifest); err != nil {
		return fmt.Errorf("write case manifest: %w", err)
	}
	return nil
}

func resultMap(result Result) (map[string]any, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal run result for storage: %w", err)
	}
	var evidenceMap map[string]any
	if err := json.Unmarshal(raw, &evidenceMap); err != nil {
		return nil, fmt.Errorf("decode run result for storage: %w", err)
	}
	return evidenceMap, nil
}

func (r *Runner) executeAction(turnIndex, stepIndex int, actorRole, actionType string, payload map[string]any) (ActionExecution, error) {
	return r.executeActionContext(context.Background(), turnIndex, stepIndex, actorRole, actionType, payload)
}

func (r *Runner) executeActionContext(ctx context.Context, turnIndex, stepIndex int, actorRole, actionType string, payload map[string]any) (ActionExecution, error) {
	if actionType == "import_case_file" {
		prepared, issue, err := r.prepareCaseFileImport(actorRole, payload)
		if err != nil {
			return ActionExecution{}, err
		}
		if issue != nil {
			return ActionExecution{Result: map[string]any{"ok": false, "error": issue.Error, "actor_message": issue.ActorMessage}}, nil
		}
		payload = prepared
	}
	return r.executePreparedActionContext(ctx, turnIndex, stepIndex, actorRole, actionType, payload)
}

func (r *Runner) executePreparedActionContext(ctx context.Context, turnIndex, stepIndex int, actorRole, actionType string, payload map[string]any) (ActionExecution, error) {
	preparedPayload, err := r.prepareActionPayload(ctx, actionType, payload)
	if err != nil {
		return ActionExecution{}, err
	}
	payload = preparedPayload
	execRes, handled, err := r.executeLocalActionContext(ctx, actorRole, actionType, payload)
	if err != nil {
		return ActionExecution{}, err
	}
	if !handled {
		res, err := r.stepForCertificateContext(ctx, actionType, actorRole, payload)
		if err != nil {
			return ActionExecution{}, err
		}
		execRes = ActionExecution{Result: res}
	}
	res := execRes.Result
	if ok, _ := res["ok"].(bool); ok {
		if state, ok := res["state"].(map[string]any); ok {
			if handled {
				r.state = state
			} else {
				r.state = mergeLocalCaseExtensions(r.state, state)
			}
		}
	}
	if err := r.persistActionEvent(turnIndex, stepIndex, actorRole, actionType, payload, res); err != nil {
		return ActionExecution{}, err
	}
	return execRes, nil
}
