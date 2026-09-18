package runner

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
)

func TestLawyerImportProductionAndExhibit(t *testing.T) {
	r := newTimeoutTestRunner(t)
	r.cfg.OutputPath = filepath.Join(t.TempDir(), "run.json")
	var err error
	r.prompts, err = adcprompts.Load(adcprompts.Options{})
	if err != nil {
		t.Fatal(err)
	}
	caseObj := r.state["case"].(map[string]any)
	caseObj["status"] = "pretrial"
	caseObj["phase"] = "discovery"
	res, err := r.executeAction(0, 0, "defendant", "serve_request_for_production", map[string]any{
		"served_by": "defendant", "served_on": "plaintiff", "requests": []string{"Produce the source records."},
	})
	if err != nil || res.Result["ok"] != true {
		t.Fatalf("serve production request: %v, %v", res.Result, err)
	}
	roles := []map[string]any{{"role": "plaintiff", "allowed_tools": []string{"submit_technical_report", "import_case_file", "produce_case_file", "offer_exhibit", "rest_case"}}}
	api := newRoleAPIServer(r)
	toolSpecs, err := r.legalToolSpecs([]string{"import_case_file"})
	if err != nil {
		t.Fatal(err)
	}
	description, err := r.prompts.Text(adcprompts.DirectToolDescriptionPrefix + "import_case_file")
	if err != nil || len(toolSpecs) != 1 || toolSpecs[0]["description"] != description {
		t.Fatalf("Role API import guidance: %v, %v", toolSpecs, err)
	}
	decide := func(tool string, payload map[string]any) map[string]any {
		t.Helper()
		before := cloneJSONMap(r.state)
		next, err := r.lean.NextOpportunity(r.state, roles, 8)
		if err != nil {
			t.Fatal(err)
		}
		opportunity, err := parseLeanOpportunity(mapFromAny(next["opportunity"]))
		if err != nil {
			t.Fatalf("next opportunity: %v: %v", next, err)
		}
		if len(opportunity.AllowedTools) < 2 || !contains(opportunity.AllowedTools, "import_case_file") {
			t.Fatalf("expected import alongside the ordinary action: %v", opportunity.AllowedTools)
		}
		turn := &externalOpportunityTurn{
			role: spec.RoleSpec{Name: "plaintiff"}, opportunity: opportunity,
			stateVersion: int(next["state_version"].(float64)), rolesPayload: roles,
			attemptsMax: 3, attemptsRemaining: 3, done: make(chan externalOpportunityResult, 1),
		}
		args := map[string]any{"kind": "tool", "tool_name": tool, "payload": payload}
		if tool == "submit_technical_report" {
			invalid := map[string]any{"kind": "tool", "tool_name": tool, "payload": map[string]any{"party": "defendant", "report_id": "TR-D1"}}
			if _, failure := api.submitDecisionLocked(turn, invalid); failure == nil {
				t.Fatal("report accepted another party's fixed fields")
			}
		}
		response, failure := api.submitDecisionLocked(turn, args)
		if failure != nil {
			t.Fatalf("%s at %v: %v", tool, opportunity.AllowedTools, failure)
		}
		transition := r.certificateTransitions[len(r.certificateTransitions)-1]
		if transition.ApplyDecision == nil {
			t.Fatalf("missing authorized transition for %s", tool)
		}
		if tool == "import_case_file" {
			for _, key := range []string{"party", "report_id", "produced_to", "request_ref"} {
				if _, exists := transition.ApplyDecision.ExecutedStep.Payload[key]; exists {
					t.Fatalf("import inherited %s from another action", key)
				}
			}
		}
		replayed, err := replayApplyDecisionTransition(r.lean, before, 1, *transition.ApplyDecision)
		if err != nil || !reflect.DeepEqual(replayed, r.state) {
			t.Fatalf("replay %s: %v; states equal=%v", tool, err, reflect.DeepEqual(replayed, r.state))
		}
		return response
	}
	for i := 1; i <= 2; i++ {
		text := fmt.Sprintf("Source record %d\n", i)
		response := decide("import_case_file", map[string]any{
			"original_name": fmt.Sprintf("source-%d.txt", i), "label": "Retrieved source",
			"content_base64": base64.StdEncoding.EncodeToString([]byte(text)),
		})
		file := mapFromAny(mapFromAny(response["result"])["file"])
		if file["file_id"] != fmt.Sprintf("file-%04d", i) {
			t.Fatalf("imported file: %v", file)
		}
		data, err := os.ReadFile(file["storage_relpath"].(string))
		if err != nil || string(data) != text {
			t.Fatalf("stored file: %q, %v", data, err)
		}
	}
	decide("submit_technical_report", map[string]any{"title": "Source review", "summary": "Two source records imported."})
	for i := 1; i <= 2; i++ {
		response := decide("produce_case_file", map[string]any{})
		action := mapFromAny(mapFromAny(response["acceptance"])["action"])
		if mapFromAny(action["payload"])["file_id"] != fmt.Sprintf("file-%04d", i) {
			t.Fatalf("production did not select remaining file: %v", action)
		}
	}
	caseObj = r.state["case"].(map[string]any)
	caseObj["status"] = "trial"
	caseObj["phase"] = "plaintiff_evidence"
	r.state["passed_opportunities"] = []any{}
	decide("import_case_file", map[string]any{
		"original_name": "trial-source.txt", "content_base64": base64.StdEncoding.EncodeToString([]byte("Trial source\n")),
	})
	decide("offer_exhibit", map[string]any{
		"party": "plaintiff", "file_id": "file-0003", "exhibit_id": "PX-1",
		"description": "Trial source", "admitted": true,
	})
	read, err := r.executeAction(0, 0, "juror", "read_case_text_file", map[string]any{"file_id": "file-0003"})
	if err != nil || read.Result["ok"] != true || read.Result["text"] != "Trial source\n" {
		t.Fatalf("juror read: %v, %v", read.Result, err)
	}
}
