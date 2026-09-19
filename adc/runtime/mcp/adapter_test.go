package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/agentcourt/adj/common/mcpbridge"
)

func TestADCSubmissionsIncludeStateVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		response := map[string]any{"ok": true}
		if r.URL.Path == roleAPIPath+"/get" {
			response["opportunity"] = map[string]any{"opportunity_id": "o1", "state_version": 42}
		} else if body["opportunity_id"] != "o1" || body["state_version"] != float64(42) {
			t.Errorf("unversioned submission: %#v", body)
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := mcpbridge.NewAPIClient(server.URL, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := mcpbridge.LoadPromptCatalog(mcpbridge.PromptCatalogOptions{Procedure: "adc-version-test"}, promptDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	a := &adapter{client: client, prompts: catalog}
	for _, name := range []string{"send_work_notes", "report_failure"} {
		result, err := a.CallTool(context.Background(), mcpbridge.Profile{CaseID: "test", AssignmentType: "juror", PrincipalID: "J1"}, name, map[string]any{"notes": "test", "message": "test"})
		if err != nil || result.IsError {
			t.Fatalf("%s: %#v, %v", name, result, err)
		}
	}
}

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
