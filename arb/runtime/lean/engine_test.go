package lean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStepEncodesOpportunityAuthority(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	requestPath := filepath.Join(dir, "request.json")
	script := `#!/bin/sh
request=$(cat)
printf '%s' "$request" > "$1"
printf '%s\n' '{"ok":true,"state":{"state_version":8}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	authority := OpportunityAuthority{
		OpportunityID:        "deliberation:2:C3",
		ExpectedStateVersion: 7,
		Role:                 "council",
		Phase:                "deliberation",
		MemberID:             "C3",
	}
	engine := New([]string{enginePath, requestPath})
	if _, err := engine.Step(
		map[string]any{"state_version": 7},
		"submit_council_vote",
		"council",
		authority,
		map[string]any{"member_id": "C3"},
	); err != nil {
		t.Fatalf("step: %v", err)
	}
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	var request struct {
		Action struct {
			Authority OpportunityAuthority `json:"authority"`
		} `json:"action"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if request.Action.Authority != authority {
		t.Fatalf("authority = %#v, want %#v", request.Action.Authority, authority)
	}
}

func TestEngineEnforcesOpportunityAuthority(t *testing.T) {
	enginePath := filepath.Clean(filepath.Join("..", "..", ".bin", "aarengine"))
	if _, err := os.Stat(enginePath); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("%s is required", enginePath)
		}
		t.Fatalf("stat engine: %v", err)
	}
	engine := New([]string{enginePath})
	initial := testInitialState()
	members := []map[string]any{
		testCouncilMember("C1"),
		testCouncilMember("C2"),
		testCouncilMember("C3"),
	}
	initializedResponse, err := engine.InitializeCase(initial, "The proposition is demonstrated.", members)
	if err != nil {
		t.Fatalf("initialize case: %v", err)
	}
	if ok, _ := initializedResponse["ok"].(bool); !ok {
		t.Fatalf("initialize case rejected: %v", initializedResponse["error"])
	}
	initialized := requireTestMap(t, initializedResponse["state"], "initialized state")
	correct := OpportunityAuthority{
		OpportunityID:        "openings:plaintiff",
		ExpectedStateVersion: 1,
		Role:                 "plaintiff",
		Phase:                "openings",
	}
	accepted, err := engine.Step(initialized, "record_opening_statement", "plaintiff", correct, map[string]any{"text": "Opening."})
	if err != nil {
		t.Fatalf("correct authority step: %v", err)
	}
	if ok, _ := accepted["ok"].(bool); !ok {
		t.Fatalf("correct authority rejected: %v", accepted["error"])
	}
	stale := correct
	stale.ExpectedStateVersion = 0
	assertEngineAuthorityRejected(t, engine, initialized, "record_opening_statement", "plaintiff", stale, map[string]any{"text": "Opening."}, "stale opportunity state_version=0 current=1")

	caseState := requireTestMap(t, initialized["case"], "initialized case")
	caseState["phase"] = "deliberation"
	wrongMember := OpportunityAuthority{
		OpportunityID:        "deliberation:1:C2",
		ExpectedStateVersion: 1,
		Role:                 "council",
		Phase:                "deliberation",
		MemberID:             "C2",
	}
	assertEngineAuthorityRejected(t, engine, initialized, "submit_council_vote", "council", wrongMember, map[string]any{
		"member_id": "C2",
		"vote":      "demonstrated",
		"rationale": "Reason.",
	}, "does not match current opportunity deliberation:1:C1")
	correctMember := wrongMember
	correctMember.OpportunityID = "deliberation:1:C1"
	correctMember.MemberID = "C1"
	assertEngineAuthorityRejected(t, engine, initialized, "submit_council_vote", "council", correctMember, map[string]any{
		"member_id": "C2",
		"vote":      "demonstrated",
		"rationale": "Reason.",
	}, "action member_id C2 does not match current member_id C1")
}

func testInitialState() map[string]any {
	return map[string]any{
		"schema_version": "v1",
		"forum_name":     "Authority Test Forum",
		"case": map[string]any{
			"case_id":            "authority-test",
			"caption":            "Claimant v. Respondent",
			"proposition":        "",
			"status":             "draft",
			"phase":              "draft",
			"council_members":    []map[string]any{},
			"openings":           []map[string]any{},
			"arguments":          []map[string]any{},
			"rebuttals":          []map[string]any{},
			"surrebuttals":       []map[string]any{},
			"closings":           []map[string]any{},
			"offered_evidence":   []map[string]any{},
			"technical_reports":  []map[string]any{},
			"submitted_evidence": []map[string]any{},
			"deliberation_round": 1,
			"council_votes":      []map[string]any{},
			"resolution":         "",
			"failure":            nil,
		},
		"policy": map[string]any{
			"council_size":                    3,
			"evidence_standard":               "Preponderance of the evidence.",
			"required_votes_for_decision":     2,
			"max_opening_chars":               4_000,
			"max_argument_chars":              6_000,
			"max_rebuttal_chars":              4_000,
			"max_surrebuttal_chars":           4_000,
			"max_closing_chars":               5_000,
			"max_deliberation_rounds":         3,
			"max_exhibits_per_filing":         9,
			"max_exhibits_per_side":           12,
			"max_exhibit_bytes":               131_072,
			"max_reports_per_filing":          3,
			"max_reports_per_side":            4,
			"max_report_title_bytes":          256,
			"max_report_summary_bytes":        8_192,
			"max_submitted_evidence_per_side": 8,
			"max_submitted_evidence_bytes":    131_072,
		},
		"state_version": 0,
	}
}

func testCouncilMember(memberID string) map[string]any {
	return map[string]any{
		"member_id":              memberID,
		"model":                  "",
		"persona_filename":       "",
		"status":                 "seated",
		"failure_reason":         "",
		"failure_opportunity_id": "",
		"failure_message":        "",
	}
}

func assertEngineAuthorityRejected(t *testing.T, engine Engine, state map[string]any, actionType string, actorRole string, authority OpportunityAuthority, payload map[string]any, want string) {
	t.Helper()
	response, err := engine.Step(state, actionType, actorRole, authority, payload)
	if err != nil {
		t.Fatalf("step transport: %v", err)
	}
	if ok, _ := response["ok"].(bool); ok {
		t.Fatalf("step accepted authority %#v", authority)
	}
	got, ok := response["error"].(string)
	if !ok {
		t.Fatalf("error = %#v, want string", response["error"])
	}
	if !strings.Contains(got, want) {
		t.Fatalf("error = %q, want substring %q", got, want)
	}
}

func requireTestMap(t *testing.T, value any, label string) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", label, value)
	}
	return result
}
