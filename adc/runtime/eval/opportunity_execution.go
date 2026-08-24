package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/adc/runtime/spec"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type judgeOpportunityExecutionOptions struct {
	Engine                     lean.Engine
	State                      map[string]any
	Roles                      []spec.RoleSpec
	RolesPayload               []map[string]any
	Client                     runner.ResponseClient
	Court                      courts.Profile
	Model                      string
	Temperature                *float64
	PromptDir                  string
	PromptFiles                map[string]string
	Objective                  func(map[string]any) (string, error)
	CounterfactualModel        bool
	RequireDeterministicAction bool
}

type judgeOpportunityExecution struct {
	Opportunity map[string]any
	View        map[string]any
	TurnLog     runner.TurnLog
	FinalState  map[string]any
	Provider    openaiapi.Accounting
	Exchanges   []ResponseExchange
}

func executeJudgeOpportunity(ctx context.Context, opts judgeOpportunityExecutionOptions) (judgeOpportunityExecution, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(opts.State) == 0 {
		return judgeOpportunityExecution{}, fmt.Errorf("judge opportunity state is empty")
	}
	if len(opts.Roles) == 0 {
		return judgeOpportunityExecution{}, fmt.Errorf("judge opportunity roles are empty")
	}
	if len(opts.RolesPayload) == 0 {
		return judgeOpportunityExecution{}, fmt.Errorf("judge opportunity roles payload is empty")
	}
	opportunityResponse, err := opts.Engine.NextOpportunityContext(ctx, opts.State, opts.RolesPayload, 3)
	if err != nil {
		return judgeOpportunityExecution{}, fmt.Errorf("next opportunity: %w", err)
	}
	if ok, _ := opportunityResponse["ok"].(bool); !ok {
		return judgeOpportunityExecution{}, fmt.Errorf("next opportunity rejected: %s", strings.TrimSpace(executionStringField(opportunityResponse, "error")))
	}
	if terminal, _ := opportunityResponse["terminal"].(bool); terminal {
		return judgeOpportunityExecution{}, fmt.Errorf("next opportunity returned terminal state: %s", strings.TrimSpace(executionStringField(opportunityResponse, "reason")))
	}
	opportunity, _ := opportunityResponse["opportunity"].(map[string]any)
	if len(opportunity) == 0 {
		return judgeOpportunityExecution{}, fmt.Errorf("next opportunity returned empty opportunity")
	}
	if opts.RequireDeterministicAction {
		if deterministic, ok := opportunity["deterministic_action"].(map[string]any); !ok || deterministic == nil {
			return judgeOpportunityExecution{}, fmt.Errorf("judge opportunity requires a deterministic action")
		}
	}
	if opts.CounterfactualModel {
		delete(opportunity, "deterministic_action")
	}
	if _, deterministic := opportunity["deterministic_action"]; !deterministic && opts.Client == nil {
		return judgeOpportunityExecution{}, fmt.Errorf("model-backed judge opportunity requires a response client")
	}
	objectiveOverride := ""
	if opts.Objective != nil {
		objectiveOverride, err = opts.Objective(cloneEvalMap(opportunity))
		if err != nil {
			return judgeOpportunityExecution{}, fmt.Errorf("render opportunity objective: %w", err)
		}
		objectiveOverride = strings.TrimSpace(objectiveOverride)
		if objectiveOverride == "" {
			return judgeOpportunityExecution{}, fmt.Errorf("render opportunity objective returned empty text")
		}
	}
	var recordingClient *RecordingResponseClient
	responseClient := opts.Client
	if opts.Client != nil {
		recordingClient, err = NewRecordingResponseClient(opts.Client)
		if err != nil {
			return judgeOpportunityExecution{}, err
		}
		responseClient = recordingClient
	}
	opportunityRunner, err := runner.NewOpportunityRunner(runner.OpportunityRunnerOptions{
		State:       opts.State,
		Roles:       opts.Roles,
		Engine:      opts.Engine,
		Client:      responseClient,
		Court:       opts.Court,
		Model:       opts.Model,
		Temperature: opts.Temperature,
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	})
	if err != nil {
		return judgeOpportunityExecution{}, err
	}
	result := judgeOpportunityExecution{
		Opportunity: cloneEvalMap(opportunity),
	}
	result.TurnLog, err = opportunityRunner.ExecuteDirectOpportunity(ctx, runner.DirectOpportunityRequest{
		Opportunity:       opportunity,
		StateVersion:      executionIntField(opportunityResponse, "state_version"),
		RolesPayload:      opts.RolesPayload,
		TurnIndex:         1,
		ObjectiveOverride: objectiveOverride,
	})
	result.View = opportunityRunner.LastOpportunityView()
	result.FinalState = opportunityRunner.State()
	result.Provider = opportunityRunner.ProviderAccounting()
	if recordingClient != nil {
		result.Exchanges = recordingClient.Exchanges()
	}
	return result, err
}

func executionStringField(value map[string]any, key string) string {
	text, _ := value[key].(string)
	return text
}

func executionIntField(value map[string]any, key string) int {
	switch number := value[key].(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	default:
		return 0
	}
}
