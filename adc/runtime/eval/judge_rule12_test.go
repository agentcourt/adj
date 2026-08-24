package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeRule12Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r12-test","tier":1,"issue_family":"missing_element","case_theme":"theme","ground":"failure_to_state_a_claim","complaint_text":"Plaintiff alleges a contract and breach but no damages.","motion_text":"Defendant moves because damages are missing.","opposition_text":"Plaintiff says damages can be inferred.","expected_disposition":"granted","expected_leave_to_amend":true,"expected_missing_elements":["damages"],"expected_reason_tags":["missing_element","amendable_defect"],"severity":3}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule12Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule12Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r12-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule12StateCreatesMotionDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule12Fixture("denied", "factual_dispute")
	state := BuildJudgeRule12State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["status"] != "filed" {
		t.Fatalf("case status = %v", caseObj["status"])
	}
	docket, _ := caseObj["docket"].([]any)
	var hasMotion bool
	for _, item := range docket {
		entry, _ := item.(map[string]any)
		if entry["title"] == "Rule 12 Motion" {
			hasMotion = true
			desc := entry["description"].(string)
			if !strings.Contains(desc, "ground=failure_to_state_a_claim") || !strings.Contains(desc, fixture.MotionText) {
				t.Fatalf("motion description = %q", desc)
			}
		}
	}
	if !hasMotion {
		t.Fatalf("docket missing Rule 12 Motion: %+v", docket)
	}
}

func TestScoreJudgeRule12ResponseDetectsFalseDismissal(t *testing.T) {
	t.Parallel()

	fixture := testRule12Fixture("denied", "factual_dispute")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule12Tool,
			Arguments: map[string]any{
				"motion_index":     0,
				"ground":           "failure_to_state_a_claim",
				"disposition":      "granted",
				"leave_to_amend":   true,
				"missing_elements": []any{"breach"},
				"reasoning":        "The facts are disputed, so the complaint is weak.",
			},
		}},
	}
	result := scoreJudgeRule12Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
	summary := JudgeRule12Summary{
		ByReasonTag:   map[string]JudgeRule12Slice{},
		ByIssueFamily: map[string]JudgeRule12Slice{},
		ByGround:      map[string]JudgeRule12Slice{},
		ByTier:        map[string]JudgeRule12Slice{},
	}
	applyRule12SummaryResult(&summary, result, 1)
	if summary.FalseDismissals != 1 {
		t.Fatalf("FalseDismissals = %d, want 1", summary.FalseDismissals)
	}
}

func TestRule12OutcomeScoringAcceptsEquivalentElementLabels(t *testing.T) {
	t.Parallel()

	result := JudgeRule12Result{
		Ground:                  "failure_to_state_a_claim",
		ExpectedDisposition:     "granted",
		Disposition:             "granted",
		ExpectedLeaveToAmend:    true,
		LeaveToAmend:            true,
		ExpectedMissingElements: []string{"breach", "damages"},
		MissingElements:         []string{"contract term", "facts constituting breach", "damages"},
	}
	if !rule12OutcomeCorrect(result) {
		t.Fatalf("rule12OutcomeCorrect = false, want true")
	}
}

func TestRule12OutcomeScoringAcceptsOmittedJurisdictionBasis(t *testing.T) {
	t.Parallel()

	result := JudgeRule12Result{
		Ground:                            "lack_subject_matter_jurisdiction",
		ExpectedDisposition:               "granted",
		Disposition:                       "granted",
		ExpectedLeaveToAmend:              true,
		LeaveToAmend:                      true,
		ExpectedJurisdictionBasisRejected: "unspecified",
		JurisdictionBasisRejected:         "No 1331 federal question alleged; no 1332 diversity allegations, citizenship, or amount in controversy pled.",
	}
	if !rule12OutcomeCorrect(result) {
		t.Fatalf("rule12OutcomeCorrect = false, want true")
	}
}

func TestRule12ReasonTagsMatchLiveWording(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag    string
		reason string
	}{
		{
			tag:    "accept_pled_facts",
			reason: "The pleaded facts must be accepted as true at this stage.",
		},
		{
			tag:    "missing_element",
			reason: "The complaint fails to allege damages, an essential element.",
		},
		{
			tag:    "standing_traceability",
			reason: "The alleged injury is not fairly traceable to the defendant.",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			matches := matchedRule12ReasonTags(tt.reason, []string{tt.tag})
			if len(matches) != 1 || matches[0] != tt.tag {
				t.Fatalf("matchedRule12ReasonTags(%q, %q) = %v", tt.reason, tt.tag, matches)
			}
		})
	}
}

func TestRunJudgeRule12DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r12-dry","tier":1,"issue_family":"missing_element","case_theme":"theme","ground":"failure_to_state_a_claim","complaint_text":"Plaintiff alleges breach but no damages.","motion_text":"Defendant moves because damages are missing.","opposition_text":"Plaintiff requests leave to amend.","expected_disposition":"granted","expected_leave_to_amend":true,"expected_missing_elements":["damages"],"expected_reason_tags":["missing_element","amendable_defect"],"severity":3}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule12Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule12(nil, JudgeRule12Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule12 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule12Summary
	if err := json.Unmarshal(rawSummary, &parsed); err != nil {
		t.Fatalf("Unmarshal summary error = %v", err)
	}
	if parsed.Total != 1 || parsed.WeightedAccuracy != 1 {
		t.Fatalf("parsed summary = %+v", parsed)
	}
	rawResults, err := os.ReadFile(filepath.Join(outDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile results error = %v", err)
	}
	if !strings.Contains(string(rawResults), `"lean_accepted":true`) {
		t.Fatalf("results missing accepted Lean decision: %s", rawResults)
	}
}

func TestRescoreJudgeRule12WritesUpdatedSummary(t *testing.T) {
	t.Parallel()

	result := JudgeRule12Result{
		ID:                                "r12-rescore",
		Tier:                              1,
		IssueFamily:                       "jurisdiction_defect",
		Ground:                            "lack_subject_matter_jurisdiction",
		ExpectedDisposition:               "granted",
		ExpectedLeaveToAmend:              true,
		ExpectedJurisdictionBasisRejected: "unspecified",
		ExpectedReasonTags:                []string{"jurisdiction_defect", "amendable_defect"},
		Severity:                          3,
		Model:                             "test-model",
		PromptSource:                      "production",
		PromptName:                        "production",
		Disposition:                       "granted",
		LeaveToAmend:                      true,
		JurisdictionBasisRejected:         "No 1331 federal question alleged; no 1332 diversity allegations.",
		Reasoning:                         "The complaint omits subject matter jurisdiction allegations, and the defect can be cured by amendment.",
		LeanAccepted:                      true,
		StepAccepted:                      true,
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal result error = %v", err)
	}
	resultsPath := filepath.Join(t.TempDir(), "results.jsonl")
	if err := os.WriteFile(resultsPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile results error = %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RescoreJudgeRule12(JudgeRule12RescoreOptions{
		ResultsPath: resultsPath,
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("RescoreJudgeRule12 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.ReasonCorrect != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(outDir, "summary.json")); err != nil {
		t.Fatalf("Stat summary error = %v", err)
	}
}

func testRule12Fixture(expectedDisposition string, tag string) JudgeRule12Fixture {
	return JudgeRule12Fixture{
		ID:                  "r12-test",
		Tier:                1,
		IssueFamily:         tag,
		CaseTheme:           "Rule 12 test",
		Ground:              "failure_to_state_a_claim",
		ComplaintText:       "Plaintiff alleges a contract, a specific breach, and damages.",
		MotionText:          "Defendant moves by disputing the alleged breach facts.",
		OppositionText:      "Plaintiff argues the pleaded facts must be accepted as true.",
		ExpectedDisposition: expectedDisposition,
		ExpectedReasonTags:  []string{tag},
		Severity:            1,
	}
}

func writeFakeJudgeRule12Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"none","kind":"turn","may_pass":false,"actor_message":"Current none opportunity for judge: act on this objective now.","objective":"For case 0, decide Rule 12 motion_index 0 on the ground failure_to_state_a_claim. Apply the standard for that ground. Grant only if that ground is established on the pleadings or jurisdictional allegations. If granted, set with_prejudice and leave_to_amend consistently and state the decisive reason.","allowed_tools":["decide_rule12_motion"],"step_budget":3,"priority":100,"constraints":{"required_payload":{"motion_index":0,"ground":"failure_to_state_a_claim"}}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"decide_rule12_motion"}}'
  ;;
*'"action_type":"decide_rule12_motion"'*)
  printf '%s' '{"ok":true,"state":{"state_version":1,"case":{"status":"pretrial","phase":"pleadings"}}}'
  ;;
*)
  printf '%s' '{"ok":false,"error":"unexpected request"}'
  ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("WriteFile engine error = %v", err)
	}
	return path
}
