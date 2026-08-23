package runner

import (
	"strings"

	"github.com/agentcourt/adj/adc/runtime/spec"
)

var referenceToolSet = map[string]bool{
	"get_case":            true,
	"explain_decisions":   true,
	"list_case_files":     true,
	"read_case_text_file": true,
	"request_case_file":   true,
	"get_juror_context":   true,
}

var referenceToolNames = []string{
	"get_case",
	"explain_decisions",
	"list_case_files",
	"read_case_text_file",
	"request_case_file",
	"get_juror_context",
}

func isReferenceTool(name string) bool {
	return referenceToolSet[strings.TrimSpace(name)]
}

func referenceToolsForRole(role spec.RoleSpec) []string {
	tools := make([]string, len(referenceToolNames))
	copy(tools, referenceToolNames)
	return tools
}

func opportunityCallableTools(role spec.RoleSpec, opportunity leanOpportunity) []string {
	names := append([]string{}, opportunity.AllowedTools...)
	for _, name := range referenceToolsForRole(role) {
		names = appendIfMissing(names, name)
	}
	if opportunity.MayPass {
		names = appendIfMissing(names, "pass_turn")
	}
	return names
}
