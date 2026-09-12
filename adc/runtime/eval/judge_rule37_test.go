package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeRule37Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r37-test","tier":1,"issue_family":"no_response","case_theme":"No interrogatory response.","movant":"plaintiff","target_party":"defendant","discovery_type":"interrogatories","set_index":0,"request_text":"Identify witnesses with knowledge of the delivery failure.","response_text":"No response served by the deadline.","motion_text":"Plaintiff moves to compel complete answers and requests $750 in fees.","opposition_text":"Defendant offers no justification for missing the deadline.","expected_granted":true,"expected_sanction_type":"fees","expected_sanction_amount":750,"expected_reason_tags":["no_response","fees"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule37Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule37Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r37-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule37StateCreatesMotionDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule37Fixture(true, "fees", 750, "no_response", "fees")
	state := BuildJudgeRule37State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["phase"] != "discovery" {
		t.Fatalf("case phase = %v", caseObj["phase"])
	}
	docket, _ := caseObj["docket"].([]any)
	found := false
	for _, entry := range docket {
		m, _ := entry.(map[string]any)
		if m["title"] == "Rule 37 Motion" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docket missing Rule 37 Motion: %+v", docket)
	}
}

func TestScoreJudgeRule37ResponseDetectsSanctionMismatch(t *testing.T) {
	t.Parallel()

	fixture := testRule37Fixture(true, "fees", 750, "no_response", "fees")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule37Tool,
			Arguments: map[string]any{
				"motion_index":    0,
				"granted":         true,
				"sanction_type":   "none",
				"sanction_amount": 0,
				"order_text":      "motion granted; compel complete answers",
				"reasoning":       "Defendant failed to respond to interrogatories.",
			},
		}},
	}
	result := scoreJudgeRule37Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if !result.GrantCorrect {
		t.Fatalf("GrantCorrect = false, want true")
	}
	if result.SanctionCorrect {
		t.Fatalf("SanctionCorrect = true, want false")
	}
	summary := JudgeRule37Summary{
		ByReasonTag:        map[string]JudgeRule37Slice{},
		ByIssueFamily:      map[string]JudgeRule37Slice{},
		ByTier:             map[string]JudgeRule37Slice{},
		ByMovant:           map[string]JudgeRule37Slice{},
		ByExpectedSanction: map[string]JudgeRule37Slice{},
	}
	applyJudgeRule37SummaryResult(&summary, result, 1)
	if summary.SanctionMismatches != 1 {
		t.Fatalf("SanctionMismatches = %d, want 1", summary.SanctionMismatches)
	}
}

func TestRescoreJudgeRule37WritesUpdatedSummary(t *testing.T) {
	t.Parallel()

	granted := true
	amount := 750.0
	result := JudgeRule37Result{
		ID:                     "r37-rescore",
		Tier:                   1,
		IssueFamily:            "no_response",
		Movant:                 "plaintiff",
		TargetParty:            "defendant",
		DiscoveryType:          "interrogatories",
		ExpectedGranted:        true,
		ExpectedSanctionType:   "fees",
		ExpectedSanctionAmount: 750,
		ExpectedReasonTags:     []string{"no_response", "fees"},
		Severity:               5,
		Model:                  "test-model",
		PromptSource:           "production",
		PromptName:             "production",
		Granted:                &granted,
		SanctionType:           "fees",
		SanctionAmount:         &amount,
		Reasoning:              "Defendant failed to respond, so fees are awarded.",
		LeanAccepted:           true,
		StepAccepted:           true,
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
	summary, err := RescoreJudgeRule37(JudgeRule37RescoreOptions{
		ResultsPath: resultsPath,
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("RescoreJudgeRule37 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.ReasonCorrect != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func testRule37Fixture(expectedGranted bool, sanctionType string, sanctionAmount float64, tags ...string) JudgeRule37Fixture {
	return JudgeRule37Fixture{
		ID:                     "r37-test",
		Tier:                   1,
		IssueFamily:            "no_response",
		CaseTheme:              "No interrogatory response.",
		Movant:                 "plaintiff",
		TargetParty:            "defendant",
		DiscoveryType:          "interrogatories",
		SetIndex:               0,
		RequestText:            "Identify witnesses with knowledge of the delivery failure.",
		ResponseText:           "No response served by the deadline.",
		MotionText:             "Plaintiff moves to compel complete answers and requests $750 in fees.",
		OppositionText:         "Defendant offers no justification for missing the deadline.",
		ExpectedGranted:        expectedGranted,
		ExpectedSanctionType:   sanctionType,
		ExpectedSanctionAmount: sanctionAmount,
		ExpectedReasonTags:     tags,
		Severity:               5,
	}
}
