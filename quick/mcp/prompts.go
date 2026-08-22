package mcp

import "github.com/jsmorph/adj/common/mcpbridge"

const PromptSessionInstructions = "mcp.session.instructions"

func toolPromptID(name string) string {
	return "mcp.tool." + name
}

func waitPromptID(state string) string {
	switch state {
	case "ready", "done", "failed", "error":
		return "mcp.wait." + state
	default:
		return "mcp.wait.waiting"
	}
}

func promptDefinitions() []mcpbridge.PromptDefinition {
	definitions := []mcpbridge.PromptDefinition{
		{
			ID:           PromptSessionInstructions,
			RelativePath: "mcp/session.md",
			Fallback:     "This MCP session is bound to quick case {{CASE_ID}} for the {{ASSIGNMENT_TYPE}} assignment held by {{PRINCIPAL_ID}}.  Inspect the current state with get_current_opportunity.  When the assignment participates in a turn, call wait_for_opportunity until it returns ready, done, failed, or error.  After a waiting response, call it again with after_version.  After a ready response, use the returned prompt, turn, limits, and tools to complete that opportunity.  Stop after a done, failed, or error response.  An observer may inspect the case through the available read-only tools.",
			Tokens:       []string{"{{CASE_ID}}", "{{ASSIGNMENT_TYPE}}", "{{PRINCIPAL_ID}}"},
		},
		{ID: "mcp.wait.ready", RelativePath: "mcp/wait/ready.md", Fallback: "An opportunity is ready. Use its prompt, turn, limits, and tools to act."},
		{ID: "mcp.wait.waiting", RelativePath: "mcp/wait/waiting.md", Fallback: "No opportunity is ready. Call wait_for_opportunity again with after_version."},
		{ID: "mcp.wait.done", RelativePath: "mcp/wait/done.md", Fallback: "The case is done. Stop acting on this assignment."},
		{ID: "mcp.wait.failed", RelativePath: "mcp/wait/failed.md", Fallback: "The case failed. Stop acting on this assignment."},
		{ID: "mcp.wait.error", RelativePath: "mcp/wait/error.md", Fallback: "This assignment cannot continue without operator attention."},
	}
	toolDescriptions := map[string]string{
		"get_current_opportunity": "Return the current prompt, turn, tools, limits, remaining time, and attempts for this assignment.",
		"wait_for_opportunity":    "Wait up to 30 seconds for a ready opportunity or case-status change. If the state is waiting, call this tool again with after_version.",
		"case_status":             "Return the current quick-adjudication phase and turn.",
		"get_case":                "Return the visible quick-adjudication record.",
		"get_case_result":         "Return the final result or pending status.",
		"list_evidence":           "List immutable case documents.",
		"stat_evidence":           "Return metadata for one case document.",
		"read_evidence_range":     "Read a bounded document byte range as base64.",
		"send_work_notes":         "Record private lawyer work notes.",
		"submit_decision":         "Submit the argument for the current quick-adjudication opportunity.",
	}
	for _, name := range []string{
		"get_current_opportunity",
		"wait_for_opportunity",
		"case_status",
		"get_case",
		"get_case_result",
		"list_evidence",
		"stat_evidence",
		"read_evidence_range",
		"send_work_notes",
		"submit_decision",
	} {
		definitions = append(definitions, mcpbridge.PromptDefinition{
			ID:           toolPromptID(name),
			RelativePath: "mcp/tools/" + name + ".md",
			Fallback:     toolDescriptions[name],
		})
	}
	return definitions
}
