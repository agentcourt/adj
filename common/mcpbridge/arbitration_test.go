package mcpbridge

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestArbitrationAssignmentProfiles(t *testing.T) {
	toolNames := []string{
		"get_current_opportunity", "wait_for_opportunity", "case_status", "get_case", "get_case_result",
		"get_turn", "list_events", "send_work_notes", "list_evidence", "stat_evidence", "read_evidence_range",
		"begin_evidence_upload", "write_evidence_chunk", "commit_evidence_upload", "submit_evidence", "submit_decision",
		"submit_council_vote",
	}
	definitions := []PromptDefinition{{
		ID:       "mcp.session.instructions",
		Fallback: "{{CASE_ID}} {{ASSIGNMENT_TYPE}} {{PRINCIPAL_ID}}",
		Tokens:   []string{"{{CASE_ID}}", "{{ASSIGNMENT_TYPE}}", "{{PRINCIPAL_ID}}"},
	}}
	definitions[0].RelativePath = "mcp/session.md"
	for _, name := range toolNames {
		definitions = append(definitions, PromptDefinition{ID: "mcp.tool." + name, RelativePath: "mcp/tools/" + name + ".md", Fallback: name})
	}
	catalog, err := LoadPromptCatalog(PromptCatalogOptions{Procedure: "arbitration-profile-test"}, definitions)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewAPIClient("http://127.0.0.1:1", "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	serverAdapter, err := NewArbitrationAdapter(ArbitrationAdapterOptions{
		Client: client, Prompts: catalog, CouncilSubmissionTool: "submit_council_vote", CouncilSubmissionSchema: EmptyObjectSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		assignment Assignment
		principal  string
		tools      []string
	}{
		{
			name: "lawyer", assignment: Assignment{Audience: "aar", CaseID: "case-1", AssignmentType: "lawyer", PrincipalID: "plaintiff"}, principal: "plaintiff",
			tools: []string{"begin_evidence_upload", "case_status", "commit_evidence_upload", "get_case", "get_case_result", "get_current_opportunity", "list_evidence", "read_evidence_range", "send_work_notes", "stat_evidence", "submit_decision", "submit_evidence", "wait_for_opportunity", "write_evidence_chunk"},
		},
		{
			name: "observer", assignment: Assignment{Audience: "aar", CaseID: "case-1", AssignmentType: "observer", PrincipalID: "observer"}, principal: "observer",
			tools: []string{"case_status", "get_case", "get_case_result", "get_current_opportunity", "get_turn", "list_events", "list_evidence", "read_evidence_range", "stat_evidence", "wait_for_opportunity"},
		},
		{
			name: "council", assignment: Assignment{Audience: "aar", CaseID: "case-1", AssignmentType: "council", PrincipalID: "member-1"}, principal: "member-1",
			tools: []string{"get_case", "get_current_opportunity", "list_evidence", "read_evidence_range", "stat_evidence", "submit_council_vote", "wait_for_opportunity"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile, err := serverAdapter.OpenSession(test.assignment)
			if err != nil {
				t.Fatal(err)
			}
			if profile.AssignmentType != test.assignment.AssignmentType || profile.PrincipalID != test.principal {
				t.Fatalf("profile = %#v", profile)
			}
			tools, err := serverAdapter.Tools(profile)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(tools))
			for _, tool := range tools {
				got = append(got, tool.Name)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, test.tools) {
				t.Fatalf("tools = %#v", got)
			}
		})
	}
}
