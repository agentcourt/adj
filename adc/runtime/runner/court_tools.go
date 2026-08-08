package runner

import (
	"fmt"
	"strings"
)

func (r *Runner) rule12Grounds() []string {
	if r.courtProfile.JurisdictionScreen {
		return []string{
			"lack_subject_matter_jurisdiction",
			"no_standing",
			"not_ripe",
			"moot",
			"failure_to_state_a_claim",
		}
	}
	return []string{
		"no_standing",
		"not_ripe",
		"moot",
		"failure_to_state_a_claim",
	}
}

func (r *Runner) toolSchema(name string) map[string]any {
	base := toolSchema(name)
	if base == nil {
		return nil
	}
	schema := cloneJSONMap(base)
	switch name {
	case "file_rule12_motion", "decide_rule12_motion":
		properties, _ := schema["properties"].(map[string]any)
		ground, _ := properties["ground"].(map[string]any)
		if properties == nil || ground == nil {
			return schema
		}
		enumVals := make([]any, 0, len(r.rule12Grounds()))
		for _, groundName := range r.rule12Grounds() {
			enumVals = append(enumVals, groundName)
		}
		ground["enum"] = enumVals
		properties["ground"] = ground
		schema["properties"] = properties
	}
	return schema
}

func (r *Runner) buildTools(allowed []string) ([]map[string]any, error) {
	tools := make([]map[string]any, 0, len(allowed))
	missing := make([]string, 0)
	for _, name := range allowed {
		params := r.toolSchema(name)
		if params == nil {
			missing = append(missing, name)
			continue
		}
		tools = append(tools, map[string]any{
			"type":        "function",
			"name":        name,
			"description": "Execute " + name,
			"parameters":  params,
		})
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing tool schemas for actions: %s", strings.Join(missing, ", "))
	}
	return tools, nil
}

func (r *Runner) buildOpportunityTools(allowed []string, reference []string, mayPass bool) ([]map[string]any, error) {
	names := append([]string{}, allowed...)
	for _, name := range reference {
		if !contains(names, name) {
			names = append(names, name)
		}
	}
	if mayPass {
		names = append(names, "pass_turn")
	}
	return r.buildTools(names)
}
