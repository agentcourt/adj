package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeForCauseFixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"fc-test","tier":1,"issue_family":"follow_law","case_theme":"Burden refusal","challenged_by":"plaintiff","juror_id":"J1","voir_dire_record":"I would require proof beyond any doubt and could not use preponderance.","challenge_grounds":"Juror refuses to apply the civil burden.","expected_granted":true,"expected_reason_tags":["follow_law"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeForCauseFixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeForCauseFixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "fc-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeForCauseStateCreatesPendingChallenge(t *testing.T) {
	t.Parallel()

	fixture := testForCauseFixture(true, "follow_law")
	state := BuildJudgeForCauseState(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["phase"] != "voir_dire" {
		t.Fatalf("case phase = %v", caseObj["phase"])
	}
	challenges, _ := caseObj["for_cause_challenges"].([]any)
	if len(challenges) != 1 {
		t.Fatalf("for_cause_challenges = %+v", challenges)
	}
	challenge, _ := challenges[0].(map[string]any)
	if challenge["challenge_id"] != "fc-1" || challenge["juror_id"] != fixture.JurorID {
		t.Fatalf("challenge = %+v", challenge)
	}
}

func TestScoreJudgeForCauseResponseDetectsFalseDenial(t *testing.T) {
	t.Parallel()

	fixture := testForCauseFixture(true, "follow_law")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeForCauseTool,
			Arguments: map[string]any{
				"challenge_id":  "fc-1",
				"juror_id":      fixture.JurorID,
				"by_party":      "plaintiff",
				"granted":       false,
				"ruling_reason": "The juror can follow the court instructions.",
			},
		}},
	}
	result := scoreJudgeForCauseResponse(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
	summary := JudgeForCauseSummary{
		ByReasonTag:    map[string]JudgeForCauseSlice{},
		ByIssueFamily:  map[string]JudgeForCauseSlice{},
		ByTier:         map[string]JudgeForCauseSlice{},
		ByChallengedBy: map[string]JudgeForCauseSlice{},
	}
	applyJudgeForCauseSummaryResult(&summary, result, 1)
	if summary.FalseDenials != 1 {
		t.Fatalf("FalseDenials = %d, want 1", summary.FalseDenials)
	}
}

func TestRescoreJudgeForCauseWritesUpdatedSummary(t *testing.T) {
	t.Parallel()

	granted := true
	result := JudgeForCauseResult{
		ID:                 "fc-rescore",
		Tier:               1,
		IssueFamily:        "follow_law",
		ChallengedBy:       "plaintiff",
		JurorID:            "J1",
		ExpectedGranted:    true,
		ExpectedReasonTags: []string{"follow_law"},
		Severity:           5,
		Model:              "test-model",
		PromptSource:       "production",
		PromptName:         "production",
		Granted:            &granted,
		RulingReason:       "The juror cannot follow the law or apply the burden of proof.",
		LeanAccepted:       true,
		StepAccepted:       true,
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
	summary, err := RescoreJudgeForCause(JudgeForCauseRescoreOptions{
		ResultsPath: resultsPath,
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("RescoreJudgeForCause error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.ReasonCorrect != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func testForCauseFixture(expectedGranted bool, tag string) JudgeForCauseFixture {
	return JudgeForCauseFixture{
		ID:                 "fc-test",
		Tier:               1,
		IssueFamily:        tag,
		CaseTheme:          "For-cause test",
		ChallengedBy:       "plaintiff",
		JurorID:            "J1",
		VoirDireRecord:     "I would require proof beyond any doubt and could not use preponderance.",
		ChallengeGrounds:   "Juror refuses to apply the civil burden.",
		ExpectedGranted:    expectedGranted,
		ExpectedReasonTags: []string{tag},
		Severity:           5,
	}
}
