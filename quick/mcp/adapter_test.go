package mcp

import (
	"reflect"
	"sort"
	"testing"

	"github.com/agentcourt/adj/common/mcpbridge"
)

func TestQuickToolProfiles(t *testing.T) {
	catalog, err := mcpbridge.LoadPromptCatalog(mcpbridge.PromptCatalogOptions{Procedure: "quick-test"}, promptDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	serverAdapter := &adapter{prompts: catalog}
	lawyerAssignment, err := NewAssignment("case-1", "plaintiff")
	if err != nil {
		t.Fatal(err)
	}
	lawyer, err := serverAdapter.OpenSession(lawyerAssignment)
	if err != nil {
		t.Fatal(err)
	}
	observerAssignment, err := NewAssignment("case-1", "observer")
	if err != nil {
		t.Fatal(err)
	}
	observer, err := serverAdapter.OpenSession(observerAssignment)
	if err != nil {
		t.Fatal(err)
	}
	wantObserver := []string{
		"case_status",
		"get_case",
		"get_case_result",
		"get_current_opportunity",
		"list_evidence",
		"read_evidence_range",
		"stat_evidence",
		"wait_for_opportunity",
	}
	wantLawyer := append(append([]string(nil), wantObserver...), "send_work_notes", "submit_decision")
	sort.Strings(wantLawyer)
	observerTools, err := serverAdapter.Tools(observer)
	if err != nil {
		t.Fatal(err)
	}
	if got := toolNames(observerTools); !reflect.DeepEqual(got, wantObserver) {
		t.Fatalf("observer tools = %#v", got)
	}
	lawyerTools, err := serverAdapter.Tools(lawyer)
	if err != nil {
		t.Fatal(err)
	}
	if got := toolNames(lawyerTools); !reflect.DeepEqual(got, wantLawyer) {
		t.Fatalf("lawyer tools = %#v", got)
	}
	decision := findTool(t, lawyerTools, "submit_decision")
	properties, _ := decision.InputSchema["properties"].(map[string]any)
	toolName, _ := properties["tool_name"].(map[string]any)
	if got, _ := toolName["enum"].([]string); !reflect.DeepEqual(got, []string{"submit_argument"}) {
		t.Fatalf("submit_decision tool_name enum = %#v", toolName["enum"])
	}
}

func toolNames(tools []mcpbridge.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func findTool(t *testing.T, tools []mcpbridge.Tool, name string) mcpbridge.Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %s was not found", name)
	return mcpbridge.Tool{}
}
