package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/runner"
)

func TestExternalActivityDistinguishesRoleAPIFromDirectModel(t *testing.T) {
	turn := runner.TurnLog{
		Role: "plaintiff", OpportunityPhase: "pretrial",
		Transcript: []map[string]any{
			{"decision": map[string]any{"kind": "tool"}, "acceptance": map[string]any{"ok": true}},
			{"action": "import_case_file"},
		},
	}
	if got := collectExternalActivities([]runner.TurnLog{turn}); len(got) != 0 {
		t.Fatalf("direct model turn identified as external: %+v", got)
	}
	turn.External = true
	got := renderExternalActivityTable([]runner.TurnLog{turn})
	if !strings.Contains(got, "| 1 | plaintiff | pretrial | submit_decision | not recorded in turn | import_case_file |") {
		t.Fatalf("missing external import: %s", got)
	}
}

func TestAppendPiToolActivity(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "pi-plaintiff.stdout")
	body := `{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"private reasoning"}}
{"type":"tool_execution_start","toolCallId":"1","toolName":"bash","args":{"command":"uv run calculate.py\nprintf done"}}
{"type":"tool_execution_end","toolCallId":"1","toolName":"bash","isError":false,"result":{"content":[{"type":"text","text":"Total: 364.50\n"}]}}
{"type":"tool_execution_start","toolCallId":"2","toolName":"bash","args":{"command":"curl example.invalid"}}
{"type":"tool_execution_end","toolCallId":"2","toolName":"bash","isError":true,"result":{"content":[{"type":"text","text":"Could not resolve host"}]}}
{"type":"tool_execution_start","toolCallId":"3","toolName":"mcp","args":{"tool":"court_send_work_notes","args":{"notes":"private notes"}}}
{"type":"tool_execution_end","toolCallId":"3","toolName":"mcp","isError":false,"result":{"content":[{"type":"text","text":"private case response"}]}}
{"type":"tool_execution_start","toolCallId":"4","toolName":"bash","args":{"command":"uv run pending.py"}}
`
	if err := os.WriteFile(log, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := filepath.Join(dir, "digest.md")
	if err := os.WriteFile(digest, []byte("# Case Digest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendPiToolActivity(digest, []PiToolLog{{Participant: "plaintiff", Path: log}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(digest)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Case Digest", "Source: `pi-plaintiff.stdout`", "| bash | 1 | 1 | 1 |",
		"| mcp: court_send_work_notes | 1 | 0 | 0 |",
		"| 1 | succeeded | uv run calculate.py printf done | Total: 364.50 |",
		"| 2 | failed | curl example.invalid | Could not resolve host |",
		"| 4 | no result | uv run pending.py | n/a |",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q in digest:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "private") {
		t.Fatalf("digest included thinking, notes, or MCP response text: %s", data)
	}
	if err := os.WriteFile(log, []byte(body+`{"type":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPiToolCalls(log); err == nil {
		t.Fatal("expected error for truncated log")
	}
}
