package runner

import (
	"strings"

	"github.com/agentcourt/adj/adc/runtime/spec"
)

type ScenarioValidation struct {
	ScenarioName       string   `json:"scenario_name"`
	UnknownRoles       []string `json:"unknown_roles"`
	MissingActionTypes []int    `json:"missing_action_type_turns"`
	UnsupportedActions []string `json:"unsupported_actions"`
	RequiresLLM        bool     `json:"requires_llm"`
}

func (v ScenarioValidation) Valid() bool {
	return len(v.UnknownRoles) == 0 && len(v.MissingActionTypes) == 0 && len(v.UnsupportedActions) == 0
}

func ValidateScenarioFile(path string) (ScenarioValidation, error) {
	scenario, err := spec.Load(path)
	if err != nil {
		return ScenarioValidation{}, err
	}
	return ValidateScenario(scenario), nil
}

func ValidateScenario(scenario spec.FormalScenario) ScenarioValidation {
	roles := make(map[string]spec.RoleSpec, len(scenario.Roles))
	for _, role := range scenario.Roles {
		roles[role.Name] = role
	}
	missingActionType := make([]int, 0)
	unknownRoles := make([]string, 0)
	unsupported := map[string]bool{}
	requiresLLM := false

	if scenario.LoopPolicy != nil && strings.TrimSpace(scenario.LoopPolicy.Type) == "autopilot_trial" {
		requiresLLM = true
	}
	for _, role := range scenario.Roles {
		for _, action := range role.EffectiveAllowedActions() {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
			}
			if toolSchema(action) == nil {
				unsupported[action] = true
			}
		}
	}

	for i, turn := range scenario.Turns {
		role, ok := roles[turn.Role]
		if !ok {
			unknownRoles = appendIfMissing(unknownRoles, turn.Role)
			continue
		}
		if turn.DeterministicAction == nil {
			requiresLLM = true
		}
		if turn.DeterministicAction != nil {
			action := strings.TrimSpace(turn.DeterministicAction.ActionType)
			if action == "" {
				missingActionType = append(missingActionType, i+1)
				continue
			}
			if toolSchema(action) == nil {
				unsupported[action] = true
			}
			continue
		}
		for _, action := range turn.EffectiveAllowedActions(role) {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
			}
			if toolSchema(action) == nil {
				unsupported[action] = true
			}
		}
	}
	return ScenarioValidation{
		ScenarioName:       scenario.Name,
		UnknownRoles:       unknownRoles,
		MissingActionTypes: missingActionType,
		UnsupportedActions: sortedKeys(unsupported),
		RequiresLLM:        requiresLLM,
	}
}
