package mcp

import "github.com/agentcourt/adj/common/mcpbridge"

const PromptSessionInstructions = "mcp.session.instructions"

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
			Fallback:     "This MCP session is bound to ADC case {{CASE_ID}} for the {{ASSIGNMENT_TYPE}} assignment held by {{PRINCIPAL_ID}} with role {{ROLE_ID}}.  Inspect the current state with get_current_opportunity.  Call wait_for_opportunity until it returns ready, done, failed, or error.  After a ready response, read the returned prompt and available tools, then submit one legal decision for that opportunity.  Stop after a done, failed, or error response.  An observer may inspect status and results through the available read-only tools.",
			Tokens:       []string{"{{CASE_ID}}", "{{ASSIGNMENT_TYPE}}", "{{ROLE_ID}}", "{{PRINCIPAL_ID}}"},
		},
		{ID: "mcp.wait.ready", RelativePath: "mcp/wait/ready.md", Fallback: "An opportunity is ready. Read its prompt, use support tools as needed, record work notes when useful, and submit one decision."},
		{ID: "mcp.wait.waiting", RelativePath: "mcp/wait/waiting.md", Fallback: "No opportunity is ready. Call wait_for_opportunity again."},
		{ID: "mcp.wait.done", RelativePath: "mcp/wait/done.md", Fallback: "The case is done. Stop acting on this assignment."},
		{ID: "mcp.wait.failed", RelativePath: "mcp/wait/failed.md", Fallback: "The case failed. Stop acting on this assignment."},
		{ID: "mcp.wait.error", RelativePath: "mcp/wait/error.md", Fallback: "This assignment cannot continue without operator attention."},
	}
	descriptions := map[string]string{
		"get_current_opportunity": "Return the current prompt, opportunity, tools, limits, remaining time, and attempts.",
		"wait_for_opportunity":    "Wait up to 30 seconds for this role to receive an opportunity or terminal case status.",
		"case_status":             "Return current case status and current-turn information.",
		"get_case":                "Return the current case view visible to this role.",
		"get_case_result":         "Return final case results or pending status.",
		"explain_decisions":       "Return decision traces visible to this role.",
		"list_case_files":         "List visible case file identifiers and metadata.",
		"read_case_text_file":     "Read a visible text case file by file_id.",
		"request_case_file":       "Return a visible case file as model content items.",
		"read_case_file_bytes":    "Read a visible case file as base64 bytes.",
		"get_juror_context":       "Return questionnaire and voir dire context for one juror.",
		"send_work_notes":         "Send private work notes outside the case record.",
		"submit_decision":         "Submit one legal decision for the current opportunity.",
		"report_failure":          "Report that this agent cannot continue the active opportunity.",
	}
	for _, name := range []string{
		"get_current_opportunity",
		"wait_for_opportunity",
		"case_status",
		"get_case",
		"get_case_result",
		"explain_decisions",
		"list_case_files",
		"read_case_text_file",
		"request_case_file",
		"read_case_file_bytes",
		"get_juror_context",
		"send_work_notes",
		"submit_decision",
		"report_failure",
	} {
		definitions = append(definitions, mcpbridge.PromptDefinition{
			ID:           "mcp.tool." + name,
			RelativePath: "mcp/tools/" + name + ".md",
			Fallback:     descriptions[name],
		})
	}
	return definitions
}
