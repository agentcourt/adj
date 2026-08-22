package mcp

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jsmorph/adj/common/mcpbridge"
)

func TestADCAssignmentProfiles(t *testing.T) {
	catalog, err := mcpbridge.LoadPromptCatalog(mcpbridge.PromptCatalogOptions{Procedure: "adc-profile-test"}, promptDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	client, err := mcpbridge.NewAPIClient("http://127.0.0.1:1", "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	serverAdapter := &adapter{client: client, prompts: catalog}
	jurorAssignment, err := NewAssignment("case-1", "juror", "juror-1")
	if err != nil {
		t.Fatal(err)
	}
	juror, err := serverAdapter.OpenSession(jurorAssignment)
	if err != nil {
		t.Fatal(err)
	}
	if juror.AssignmentType != "juror" || juror.PrincipalID != "juror-1" {
		t.Fatalf("juror profile = %#v", juror)
	}
	jurorTools, err := serverAdapter.Tools(juror)
	if err != nil {
		t.Fatal(err)
	}
	if !containsTool(jurorTools, "get_juror_context") || !containsTool(jurorTools, "submit_decision") {
		t.Fatalf("juror tools = %#v", names(jurorTools))
	}
	observerAssignment, err := NewAssignment("case-1", "observer", "")
	if err != nil {
		t.Fatal(err)
	}
	observer, err := serverAdapter.OpenSession(observerAssignment)
	if err != nil {
		t.Fatal(err)
	}
	observerTools, err := serverAdapter.Tools(observer)
	if err != nil {
		t.Fatal(err)
	}
	wantObserver := []string{"case_status", "get_case_result", "get_current_opportunity", "wait_for_opportunity"}
	if got := names(observerTools); !reflect.DeepEqual(got, wantObserver) {
		t.Fatalf("observer tools = %#v", got)
	}
	if _, err := NewAssignment("case-1", "juror", ""); err == nil {
		t.Fatal("juror session without principal_id succeeded")
	}
}

func names(tools []mcpbridge.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func containsTool(tools []mcpbridge.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
