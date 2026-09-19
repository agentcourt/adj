package runner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/adc/runtime/store"
)

type OpportunityRunnerOptions struct {
	State           map[string]any
	Roles           []spec.RoleSpec
	Engine          lean.Engine
	Client          ResponseClient
	Court           courts.Profile
	Model           string
	Temperature     *float64
	Runtime         RuntimeLimits
	ScenarioBaseDir string
	EventsPath      string
	PromptDir       string
	PromptFiles     map[string]string
}

type DirectOpportunityRequest struct {
	Opportunity       map[string]any
	StateVersion      int
	RolesPayload      []map[string]any
	TurnIndex         int
	ObjectiveOverride string
}

type ProceduralFailureKind string

const (
	ProceduralFailureInvalidAttemptLimit ProceduralFailureKind = "invalid_attempt_limit"
	ProceduralFailureDecisionBudget      ProceduralFailureKind = "decision_budget_exhausted"
)

type ProceduralTurnFailure struct {
	Kind    ProceduralFailureKind
	Partial TurnLog
	Err     error
}

func (f *ProceduralTurnFailure) Error() string {
	if f == nil || f.Err == nil {
		return "procedural turn failed"
	}
	return f.Err.Error()
}

func (f *ProceduralTurnFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Err
}

func NewOpportunityRunner(opts OpportunityRunnerOptions) (*Runner, error) {
	if len(opts.State) == 0 {
		return nil, fmt.Errorf("opportunity runner state is empty")
	}
	if len(opts.Roles) == 0 {
		return nil, fmt.Errorf("opportunity runner roles are empty")
	}
	if len(opts.Engine.Command) == 0 {
		return nil, fmt.Errorf("opportunity runner Lean command is empty")
	}
	court := cloneCourtProfile(opts.Court)
	if courtProfileProvided(court) {
		if err := court.Validate(); err != nil {
			return nil, err
		}
	} else {
		resolved, err := courts.Resolve("")
		if err != nil {
			return nil, err
		}
		court = resolved
	}
	state := cloneRunnerMap(opts.State)
	cfg := Config{
		ScenarioBaseDir: strings.TrimSpace(opts.ScenarioBaseDir),
		EventsPath:      strings.TrimSpace(opts.EventsPath),
		Model:           strings.TrimSpace(opts.Model),
		Temperature:     cloneFloat64Pointer(opts.Temperature),
		Runtime:         opts.Runtime,
		PromptDir:       strings.TrimSpace(opts.PromptDir),
		PromptFiles:     cloneStringMap(opts.PromptFiles),
	}
	if cfg.ScenarioBaseDir == "" {
		cfg.ScenarioBaseDir = "."
	}
	r, err := newRunnerCore(
		nil,
		opts.Engine,
		opts.Client,
		opts.Client,
		true,
		cfg,
		opts.Roles,
		court,
		true,
	)
	if err != nil {
		return nil, err
	}
	r.scenario = spec.FormalScenario{
		CourtName:   court.Name,
		Court:       &court,
		Model:       cfg.Model,
		Temperature: cloneFloat64Pointer(cfg.Temperature),
		Roles:       cloneRoleSpecs(opts.Roles),
	}
	if err := validateScenarioActions(r.scenario, r.roles); err != nil {
		return nil, err
	}
	r.state = state
	certificateInit, err := newReplayInitializeRequest(state)
	if err != nil {
		return nil, err
	}
	r.certificateInit = certificateInit
	if err := resetEventLog(cfg.EventsPath); err != nil {
		return nil, err
	}
	return r, nil
}

func newRunnerCore(
	st *store.Store,
	le lean.Engine,
	client ResponseClient,
	jurorClient ResponseClient,
	sharedClientAccounting bool,
	cfg Config,
	roleSpecs []spec.RoleSpec,
	court courts.Profile,
	allowNoStorePersistence bool,
) (*Runner, error) {
	cfg.Runtime = cfg.Runtime.Normalized()
	cfg.ExternalRoles = append([]string(nil), cfg.ExternalRoles...)
	cfg.PromptFiles = cloneStringMap(cfg.PromptFiles)
	roles := make(map[string]spec.RoleSpec, len(roleSpecs))
	for _, role := range cloneRoleSpecs(roleSpecs) {
		roles[role.Name] = role
	}
	if err := loadRolePromptPreambles(roles, cfg.ScenarioBaseDir); err != nil {
		return nil, err
	}
	promptCatalog, err := adcprompts.Load(adcprompts.Options{PromptDir: cfg.PromptDir, PromptFiles: cfg.PromptFiles})
	if err != nil {
		return nil, err
	}
	schemaDescriptions, err := loadSchemaPropertyDescriptions(promptCatalog)
	if err != nil {
		return nil, err
	}
	return &Runner{
		lean:                    le,
		store:                   st,
		client:                  client,
		jurorClient:             jurorClient,
		sharedClientAccounting:  sharedClientAccounting,
		allowNoStorePersistence: allowNoStorePersistence,
		cfg:                     cfg,
		roles:                   roles,
		courtProfile:            cloneCourtProfile(court),
		workProductDirs:         map[string]string{},
		jurorPersonaAssignments: map[string]jurorPersonaPair{},
		externalRoles:           externalRoleSet(cfg.ExternalRoles),
		prompts:                 promptCatalog,
		promptRenderer:          newPromptRenderer(promptCatalog, schemaDescriptions, court),
		schemaDescriptions:      schemaDescriptions,
	}, nil
}

func (r *Runner) ExecuteDirectOpportunity(ctx context.Context, req DirectOpportunityRequest) (TurnLog, error) {
	if r == nil {
		return TurnLog{}, fmt.Errorf("runner is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req.TurnIndex <= 0 {
		return TurnLog{}, fmt.Errorf("opportunity turn index must be positive")
	}
	opportunityPayload := cloneRunnerMap(req.Opportunity)
	opportunity, err := parseLeanOpportunity(opportunityPayload)
	if err != nil {
		return TurnLog{}, err
	}
	if override := strings.TrimSpace(req.ObjectiveOverride); override != "" {
		opportunity.Objective = override
	}
	role, ok := r.roles[opportunity.Role]
	if !ok {
		return TurnLog{}, fmt.Errorf("opportunity returned unknown role: %s", opportunity.Role)
	}
	rolesPayload := cloneRunnerMapList(req.RolesPayload)
	if len(rolesPayload) == 0 {
		return TurnLog{}, fmt.Errorf("opportunity roles payload is empty")
	}
	r.lastOpportunityView = nil
	log, err := r.executeParsedOpportunity(ctx, req.TurnIndex, role, opportunity, rolesPayload, req.StateVersion)
	log.Source = "next_opportunity"
	turnLogsApplyOpportunity(&log, opportunity)
	if err != nil {
		var procedural *ProceduralTurnFailure
		if errors.As(err, &procedural) {
			procedural.Partial.Source = "next_opportunity"
			turnLogsApplyOpportunity(&procedural.Partial, opportunity)
			log = procedural.Partial
		}
		return log, err
	}
	return log, nil
}

func (r *Runner) executeParsedOpportunity(
	ctx context.Context,
	turnIndex int,
	role spec.RoleSpec,
	opportunity leanOpportunity,
	rolesPayload []map[string]any,
	stateVersion int,
) (TurnLog, error) {
	if opportunity.DeterministicAction != nil {
		turn := spec.TurnSpec{
			Role:                opportunity.Role,
			Prompt:              opportunity.Objective,
			MaxSteps:            opportunity.StepBudget,
			AllowedTools:        append([]string(nil), opportunity.AllowedTools...),
			DeterministicAction: opportunity.DeterministicAction,
			RequireSuccess:      true,
		}
		return r.executeTurn(ctx, turnIndex, role, turn, opportunity.AllowedTools)
	}
	if r.roleIsExternal(role.Name) {
		return r.executeExternalOpportunityTurn(ctx, turnIndex, role, opportunity, rolesPayload, stateVersion)
	}
	return r.executeOpportunityTurn(ctx, turnIndex, role, opportunity, rolesPayload, stateVersion)
}

func (r *Runner) State() map[string]any {
	if r == nil {
		return nil
	}
	return cloneRunnerMap(r.state)
}

func (r *Runner) LastOpportunityView() map[string]any {
	if r == nil {
		return nil
	}
	return cloneRunnerMap(r.lastOpportunityView)
}

func cloneRoleSpecs(in []spec.RoleSpec) []spec.RoleSpec {
	out := make([]spec.RoleSpec, len(in))
	for i, role := range in {
		out[i] = role
		out[i].AllowedActions = append([]string(nil), role.AllowedActions...)
		out[i].AllowedTools = append([]string(nil), role.AllowedTools...)
		out[i].Temperature = cloneFloat64Pointer(role.Temperature)
	}
	return out
}

func cloneFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneCourtProfile(in courts.Profile) courts.Profile {
	out := in
	out.AllowedJurisdictionBases = append([]string(nil), in.AllowedJurisdictionBases...)
	return out
}

func cloneRunnerMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneRunnerValue(value)
	}
	return out
}

func cloneRunnerMapList(in []map[string]any) []map[string]any {
	if in == nil {
		return nil
	}
	out := make([]map[string]any, len(in))
	for i, value := range in {
		out[i] = cloneRunnerMap(value)
	}
	return out
}

func cloneRunnerValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneRunnerMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneRunnerValue(item)
		}
		return out
	case []map[string]any:
		return cloneRunnerMapList(typed)
	case []string:
		return slices.Clone(typed)
	case map[string]string:
		return cloneStringMap(typed)
	case courts.Profile:
		return cloneCourtProfile(typed)
	default:
		return typed
	}
}
