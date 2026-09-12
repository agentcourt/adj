package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeRule11Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r11-test","tier":1,"issue_family":"frivolous_legal","case_theme":"Foreclosed claim.","movant":"defendant","target_party":"plaintiff","challenged_filing":"Complaint","filing_text":"Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.","notice_text":"Defendant served notice identifying the release and demanding withdrawal.","notice_served_at":"2026-07-01","motion_filed_at":"2026-07-25","motion_text":"Defendant moves for Rule 11 sanctions after no correction.","opposition_text":"Plaintiff argues the release should be ignored.","expected_granted":true,"expected_sanction_type":"admonition","expected_reason_tags":["frivolous_legal","proportional_sanction"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule11Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule11Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r11-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule11StateCreatesMotionDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule11Fixture(true, "admonition", 0, "frivolous_legal")
	state := BuildJudgeRule11State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["auto_rule11"] != true {
		t.Fatalf("auto_rule11 = %v", caseObj["auto_rule11"])
	}
	docket, _ := caseObj["docket"].([]any)
	found := false
	for _, entry := range docket {
		m, _ := entry.(map[string]any)
		if m["title"] == "Rule 11 Motion" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docket missing Rule 11 Motion: %+v", docket)
	}
}

func TestScoreJudgeRule11ResponseRejectsDeniedSanction(t *testing.T) {
	t.Parallel()

	fixture := testRule11Fixture(false, "none", 0, "weak_merits")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule11Tool,
			Arguments: map[string]any{
				"motion_index":    0,
				"granted":         false,
				"sanction_type":   "none",
				"sanction_detail": "",
				"reasoning":       "The filing was weak but not sanctionable.",
			},
		}},
	}
	result := scoreJudgeRule11Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.InvalidReason != "denied_with_sanction_type" {
		t.Fatalf("InvalidReason = %q, want denied_with_sanction_type", result.InvalidReason)
	}
}

func TestRescoreJudgeRule11WritesUpdatedSummary(t *testing.T) {
	t.Parallel()

	granted := true
	result := JudgeRule11Result{
		ID:                   "r11-rescore",
		Tier:                 1,
		IssueFamily:          "frivolous_legal",
		Movant:               "defendant",
		TargetParty:          "plaintiff",
		ExpectedGranted:      true,
		ExpectedSanctionType: "admonition",
		ExpectedReasonTags:   []string{"frivolous_legal"},
		Severity:             5,
		Model:                "test-model",
		PromptSource:         "production",
		PromptName:           "production",
		Granted:              &granted,
		SanctionType:         "admonition",
		SanctionDetail:       "Admonition for a filing with no legal basis.",
		Reasoning:            "The filing was frivolous and no legal basis existed.",
		LeanAccepted:         true,
		StepAccepted:         true,
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
	summary, err := RescoreJudgeRule11(JudgeRule11RescoreOptions{
		ResultsPath: resultsPath,
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("RescoreJudgeRule11 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.ReasonCorrect != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule11Summary
	if err := json.Unmarshal(rawSummary, &parsed); err != nil {
		t.Fatalf("Unmarshal summary error = %v", err)
	}
}

func testRule11Fixture(expectedGranted bool, sanctionType string, sanctionAmount float64, tags ...string) JudgeRule11Fixture {
	return JudgeRule11Fixture{
		ID:                     "r11-test",
		Tier:                   1,
		IssueFamily:            "frivolous_legal",
		CaseTheme:              "Foreclosed claim.",
		Movant:                 "defendant",
		TargetParty:            "plaintiff",
		ChallengedFiling:       "Complaint",
		FilingText:             "Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.",
		NoticeText:             "Defendant served notice identifying the release and demanding withdrawal.",
		NoticeServedAt:         "2026-07-01",
		MotionFiledAt:          "2026-07-25",
		MotionText:             "Defendant moves for Rule 11 sanctions after no correction.",
		OppositionText:         "Plaintiff argues the release should be ignored.",
		ExpectedGranted:        expectedGranted,
		ExpectedSanctionType:   sanctionType,
		ExpectedSanctionAmount: sanctionAmount,
		ExpectedReasonTags:     tags,
		Severity:               5,
	}
}
