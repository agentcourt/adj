package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	result := scoreJudgeRule12Response(fixture, "test-model", nil, nil, nil, nil, resp)
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
