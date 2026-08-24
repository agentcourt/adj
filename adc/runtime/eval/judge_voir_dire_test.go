package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeVoirDireFixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"jvd-test","tier":1,"question_family":"proper_bias_probe","case_theme":"theme","asked_by":"plaintiff","juror_id":"J1","question":"Can you be fair?","expected_allowed":true,"expected_reason_tags":["proper_bias_probe"],"severity":1}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeVoirDireFixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeVoirDireFixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "jvd-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeVoirDireStateCreatesPendingExchange(t *testing.T) {
	t.Parallel()

	fixture := testFixture(false, "specific_evidence_sufficiency")
	state := BuildJudgeVoirDireState(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["phase"] != "voir_dire" || caseObj["status"] != "trial" {
		t.Fatalf("case phase/status = %v/%v", caseObj["phase"], caseObj["status"])
	}
	exchanges, _ := caseObj["voir_dire_exchanges"].([]any)
	if len(exchanges) != 1 {
		t.Fatalf("voir_dire_exchanges len = %d, want 1", len(exchanges))
	}
	exchange, _ := exchanges[0].(map[string]any)
	if exchange["judge_allowed"] != nil {
		t.Fatalf("judge_allowed = %v, want nil", exchange["judge_allowed"])
	}
	if exchange["question"] != fixture.Question {
		t.Fatalf("question = %q, want %q", exchange["question"], fixture.Question)
	}
}

func TestScoreJudgeVoirDireResponseDetectsFalseAllow(t *testing.T) {
	t.Parallel()

	fixture := testFixture(false, "precommitment_liability")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeVoirDireTool,
			Arguments: map[string]any{
				"exchange_id":   "vx-1",
				"juror_id":      "J1",
				"allowed":       true,
				"ruling_reason": "This is a permissible bias question.",
			},
		}},
	}
	result := scoreJudgeVoirDireResponse(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
	if result.Allowed == nil || !*result.Allowed {
		t.Fatalf("Allowed = %v, want true", result.Allowed)
	}
}

func TestScoreJudgeVoirDireResponseMatchesReasonTag(t *testing.T) {
	t.Parallel()

	fixture := testFixture(false, "precommitment_liability")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeVoirDireTool,
			Arguments: map[string]any{
				"exchange_id":   "vx-1",
				"juror_id":      "J1",
				"allowed":       false,
				"ruling_reason": "The question asks the juror to precommit to a liability verdict.",
			},
		}},
	}
	result := scoreJudgeVoirDireResponse(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.OutcomeCorrect || !result.ReasonCorrect {
		t.Fatalf("score = outcome %v reason %v matches %v", result.OutcomeCorrect, result.ReasonCorrect, result.MatchedReasonTags)
	}
}

func TestReasonTagsMatchLiveAllowedWording(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag    string
		reason string
	}{
		{
			tag:    "proper_attention_probe",
			reason: "Allowed. The question narrowly tests the juror's ability to attend to and fairly consider documentary records.",
		},
		{
			tag:    "proper_follow_instructions_probe",
			reason: "Allowed. The question neutrally tests the juror's ability to follow limiting instructions.",
		},
		{
			tag:    "proper_follow_instructions_probe",
			reason: "Allowed. This is a narrow question testing the juror's ability to follow a limiting instruction.",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			matches := matchedReasonTags(tt.reason, []string{tt.tag})
			if len(matches) != 1 || matches[0] != tt.tag {
				t.Fatalf("matchedReasonTags(%q, %q) = %v", tt.reason, tt.tag, matches)
			}
		})
	}
}

func testFixture(expectedAllowed bool, tag string) JudgeVoirDireFixture {
	return JudgeVoirDireFixture{
		ID:                 "jvd-test",
		Tier:               1,
		QuestionFamily:     tag,
		CaseTheme:          "theme",
		AskedBy:            "plaintiff",
		JurorID:            "J1",
		Question:           "Would this proof be enough?",
		ExpectedAllowed:    expectedAllowed,
		ExpectedReasonTags: []string{tag},
		Severity:           1,
	}
}
