package mcp

import "github.com/agentcourt/adj/common/mcpbridge"

const PromptSessionInstructions = "mcp.session.instructions"

func promptDefinitions() []mcpbridge.PromptDefinition {
	definitions := []mcpbridge.PromptDefinition{
		{
			ID:           PromptSessionInstructions,
			RelativePath: "mcp/session.md",
			Fallback:     "This MCP session is bound to ARBD case {{CASE_ID}} for the {{ASSIGNMENT_TYPE}} assignment held by {{PRINCIPAL_ID}}.  Inspect the current state with get_current_opportunity.  Call wait_for_opportunity until it returns ready, done, failed, or error.  After a waiting response, call it again with after_version.  After a ready response, follow the returned prompt and use the available tools for that opportunity.  Stop after a done, failed, or error response.  An observer may inspect the record through the available read-only tools.",
			Tokens:       []string{"{{CASE_ID}}", "{{ASSIGNMENT_TYPE}}", "{{PRINCIPAL_ID}}"},
		},
		{ID: "mcp.wait.ready", RelativePath: "mcp/wait/ready.md", Fallback: "An opportunity is ready. Use its prompt, turn, limits, and tools to act."},
		{ID: "mcp.wait.waiting", RelativePath: "mcp/wait/waiting.md", Fallback: "No opportunity is ready. Call wait_for_opportunity again with after_version."},
		{ID: "mcp.wait.done", RelativePath: "mcp/wait/done.md", Fallback: "The case is done. Stop acting on this assignment."},
		{ID: "mcp.wait.failed", RelativePath: "mcp/wait/failed.md", Fallback: "The case failed. Stop acting on this assignment."},
		{ID: "mcp.wait.error", RelativePath: "mcp/wait/error.md", Fallback: "This assignment cannot continue without operator attention."},
	}
	descriptions := map[string]string{
		"get_current_opportunity": "Return the current prompt, turn, tools, limits, remaining time, and attempts for this assignment.",
		"wait_for_opportunity":    "Wait up to 30 seconds for a ready opportunity or case-status change. If the state is waiting, call this tool again with after_version.",
		"case_status":             "Return the current case phase, active turn, assignment status, and case counts.",
		"get_case":                "Return the arbitration record visible to this assignment.",
		"get_case_result":         "Return final case results, including council answers and rationales, or pending status.",
		"get_turn":                "Return the current turn role, phase, deadline, and attempts.",
		"list_events":             "List recorded case events.",
		"send_work_notes":         "Send private work notes for off-record analysis.",
		"list_evidence":           "List visible immutable record evidence.",
		"stat_evidence":           "Return metadata for one visible evidence item.",
		"read_evidence_range":     "Read a bounded byte range from one visible evidence item as base64.",
		"begin_evidence_upload":   "Begin a chunked evidence upload.",
		"write_evidence_chunk":    "Write one base64 chunk into an upload session.",
		"commit_evidence_upload":  "Verify and admit a completed evidence upload.",
		"submit_evidence":         "Submit source evidence with provenance.",
		"submit_decision":         "Submit the legal act for the current opportunity.",
		"submit_council_answer":   "Submit one council answer for the current deliberation opportunity.",
	}
	for _, name := range []string{
		"get_current_opportunity",
		"wait_for_opportunity",
		"case_status",
		"get_case",
		"get_case_result",
		"get_turn",
		"list_events",
		"send_work_notes",
		"list_evidence",
		"stat_evidence",
		"read_evidence_range",
		"begin_evidence_upload",
		"write_evidence_chunk",
		"commit_evidence_upload",
		"submit_evidence",
		"submit_decision",
		"submit_council_answer",
	} {
		definitions = append(definitions, mcpbridge.PromptDefinition{
			ID:           "mcp.tool." + name,
			RelativePath: "mcp/tools/" + name + ".md",
			Fallback:     descriptions[name],
		})
	}
	return definitions
}
