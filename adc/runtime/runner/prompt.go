package runner

import (
	"strings"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func (r *Runner) roleSpec(roleName string) spec.RoleSpec {
	if role, ok := r.roles[roleName]; ok {
		return role
	}
	return spec.RoleSpec{Name: roleName}
}

func (r *Runner) effectiveRoleModel(role spec.RoleSpec) string {
	if strings.TrimSpace(role.Model) != "" {
		return strings.TrimSpace(role.Model)
	}
	if strings.TrimSpace(r.cfg.Model) != "" {
		return strings.TrimSpace(r.cfg.Model)
	}
	return strings.TrimSpace(r.scenario.Model)
}

func (r *Runner) effectiveRoleModelByName(roleName string) string {
	return r.effectiveRoleModel(r.roleSpec(roleName))
}

func (r *Runner) effectiveRoleTemperature(role spec.RoleSpec) *float64 {
	if role.Name == "juror" && r.cfg.JurorTemperature != nil {
		return r.cfg.JurorTemperature
	}
	if role.Temperature != nil {
		return role.Temperature
	}
	if r.cfg.Temperature != nil {
		return r.cfg.Temperature
	}
	return r.scenario.Temperature
}

func (r *Runner) effectiveRoleTemperatureByName(roleName string) *float64 {
	return r.effectiveRoleTemperature(r.roleSpec(roleName))
}

func buildSystemPrompt(role spec.RoleSpec, view map[string]any) string {
	payload := marshalString(view)
	allowed := role.EffectiveAllowedActions()
	preamble := ""
	if strings.TrimSpace(role.PromptPreamble) != "" {
		preamble = "\nRole prompt preamble: " + role.PromptPreamble
	}
	return "Role: " + role.Name +
		preamble +
		"\nInstructions: " + role.Instructions +
		"\nAllowed actions: " + strings.Join(allowed, ", ") +
		"\nUse only listed tools with precise payloads." +
		"\nWhen you decide to act, call exactly one tool rather than replying with prose." +
		"\nCurrent view:\n" + payload
}

func buildOpportunityPrompt(role spec.RoleSpec, opportunity leanOpportunity) string {
	referenceTools := referenceToolsForRole(role)
	lines := []string{
		"Current opportunity:",
		opportunity.ActorMessage,
		"Objective: " + opportunity.Objective,
		"Phase: " + opportunity.Phase,
		"Allowed actions: " + strings.Join(opportunity.AllowedTools, ", "),
	}
	if len(referenceTools) > 0 {
		lines = append(lines, "Reference tools: "+strings.Join(referenceTools, ", "))
	}
	if len(opportunity.Constraints) > 0 {
		lines = append(lines, "Opportunity constraints: "+marshalString(opportunity.Constraints))
	}
	if opportunity.MayPass {
		lines = append(lines, "You may decline this opportunity by calling pass_turn.")
	} else {
		lines = append(lines, "You must choose one allowed action now.")
	}
	return strings.Join(lines, "\n")
}
