package runner

import (
	"strings"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
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

func (r *Runner) buildSystemPrompt(role spec.RoleSpec, view map[string]any) (string, error) {
	return r.prompts.Render(adcprompts.RuntimeSystemID, map[string]string{
		"{{ROLE}}":            strings.TrimSpace(role.Name),
		"{{PREAMBLE}}":        promptValue(role.PromptPreamble),
		"{{INSTRUCTIONS}}":    promptValue(role.Instructions),
		"{{ALLOWED_ACTIONS}}": promptList(role.EffectiveAllowedActions()),
		"{{VIEW}}":            marshalString(view),
	})
}

func (r *Runner) buildOpportunityPrompt(role spec.RoleSpec, opportunity leanOpportunity) (string, error) {
	return r.prompts.Render(adcprompts.RuntimeOpportunityID, map[string]string{
		"{{ACTOR_MESSAGE}}":   promptValue(opportunity.ActorMessage),
		"{{OBJECTIVE}}":       promptValue(opportunity.Objective),
		"{{PHASE}}":           promptValue(opportunity.Phase),
		"{{ALLOWED_ACTIONS}}": promptList(opportunity.AllowedTools),
		"{{REFERENCE_TOOLS}}": promptList(referenceToolsForRole(role)),
		"{{CONSTRAINTS}}":     promptJSON(opportunity.Constraints),
		"{{PASS_ACTION}}":     passAction(opportunity.MayPass),
	})
}

func promptValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(none)"
	}
	return value
}

func promptList(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

func promptSections(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, "\n\n")
}

func promptJSON(value map[string]any) string {
	if len(value) == 0 {
		return "{}"
	}
	return marshalString(value)
}

func passAction(mayPass bool) string {
	if mayPass {
		return "pass_turn"
	}
	return "(unavailable)"
}
