package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeRule56Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r56-test","tier":1,"issue_family":"missing_element","case_theme":"theme","moving_party":"defendant","request_text":"Defendant seeks summary judgment.","statement_of_undisputed_facts":"No record evidence supports causation.","opposition_text":"Plaintiff identifies no causation evidence.","expected_disposition":"granted","expected_reason_tags":["missing_element"],"severity":1}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule56Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule56Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r56-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule56StateCreatesMotionDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule56Fixture("denied", "credibility_dispute")
	state := BuildJudgeRule56State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["phase"] != "pretrial" || caseObj["status"] != "pretrial" {
		t.Fatalf("case phase/status = %v/%v", caseObj["phase"], caseObj["status"])
	}
	docket, _ := caseObj["docket"].([]any)
	var hasMotion bool
	var hasOpposition bool
	for _, item := range docket {
		entry, _ := item.(map[string]any)
		if entry["title"] == "Rule 56 Motion" {
			hasMotion = true
			if !strings.Contains(entry["description"].(string), fixture.RequestText) {
				t.Fatalf("motion description = %q", entry["description"])
			}
		}
		if entry["title"] == "Rule 56 Opposition" {
			hasOpposition = true
		}
	}
	if !hasMotion || !hasOpposition {
		t.Fatalf("docket missing motion or opposition: %+v", docket)
	}
}

func TestScoreJudgeRule56ResponseDetectsFalseGrant(t *testing.T) {
	t.Parallel()

	fixture := testRule56Fixture("denied", "credibility_dispute")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule56Tool,
			Arguments: map[string]any{
				"motion_index": 0,
				"disposition":  "granted",
				"reasoning":    "The movant says the testimony is not credible.",
			},
		}},
	}
	result := scoreJudgeRule56Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
	summary := JudgeRule56Summary{
		ByReasonTag:   map[string]JudgeRule56Slice{},
		ByIssueFamily: map[string]JudgeRule56Slice{},
		ByTier:        map[string]JudgeRule56Slice{},
		ByMovingParty: map[string]JudgeRule56Slice{},
	}
	applyRule56SummaryResult(&summary, result, 1)
	if summary.FalseGrants != 1 {
		t.Fatalf("FalseGrants = %d, want 1", summary.FalseGrants)
	}
}

func TestScoreJudgeRule56ResponseComparesSurvivingIssuesExactlyAfterNormalization(t *testing.T) {
	t.Parallel()

	fixture := testRule56Fixture("partial", "partial_element")
	fixture.ExpectedSurviving = []string{"Causation", "damages"}
	response := func(issues []any) openai.Response {
		return openai.Response{ToolCalls: []openai.ToolCall{{
			Name: JudgeRule56Tool,
			Arguments: map[string]any{
				"motion_index":     0,
				"disposition":      "partial",
				"surviving_issues": issues,
				"reasoning":        "The undisputed record supports partial relief on the element.",
			},
		}}}
	}

	normalizedMatch := scoreJudgeRule56Response(fixture, "test-model", nil, nil, nil, nil, response([]any{"  DAMAGES ", "causation"}))
	if !normalizedMatch.SurvivingCorrect || !normalizedMatch.OutcomeCorrect {
		t.Fatalf("normalized match = %+v", normalizedMatch)
	}

	mismatch := scoreJudgeRule56Response(fixture, "test-model", nil, nil, nil, nil, response([]any{"causation"}))
	if mismatch.SurvivingCorrect || mismatch.OutcomeCorrect {
		t.Fatalf("mismatch = %+v", mismatch)
	}
	summary := JudgeRule56Summary{
		ByReasonTag:   map[string]JudgeRule56Slice{},
		ByIssueFamily: map[string]JudgeRule56Slice{},
		ByTier:        map[string]JudgeRule56Slice{},
		ByMovingParty: map[string]JudgeRule56Slice{},
	}
	applyRule56SummaryResult(&summary, mismatch, 1)
	if summary.PartialMismatches != 1 {
		t.Fatalf("PartialMismatches = %d, want 1", summary.PartialMismatches)
	}
}

func TestScoreJudgeRule56ResponseRejectsUnexpectedSurvivingIssue(t *testing.T) {
	t.Parallel()

	fixture := testRule56Fixture("granted", "missing_element")
	result := scoreJudgeRule56Response(fixture, "test-model", nil, nil, nil, nil, openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule56Tool,
			Arguments: map[string]any{
				"motion_index":     0,
				"disposition":      "granted",
				"surviving_issues": []any{"damages"},
				"reasoning":        "The undisputed record establishes the claim.",
			},
		}},
	})
	if result.SurvivingCorrect || result.OutcomeCorrect {
		t.Fatalf("result = %+v", result)
	}
}

func TestRule56ReasonTagsMatchLiveWording(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag    string
		reason string
	}{
		{
			tag:    "credibility_dispute",
			reason: "Denied because the motion asks the court to weigh witness credibility.",
		},
		{
			tag:    "competing_inference",
			reason: "Denied because a reasonable jury could draw competing inferences from the record.",
		},
		{
			tag:    "no_genuine_dispute",
			reason: "Granted because the undisputed record leaves no genuine dispute and the movant is entitled to judgment as a matter of law.",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			matches := matchedRule56ReasonTags(tt.reason, []string{tt.tag})
			if len(matches) != 1 || matches[0] != tt.tag {
				t.Fatalf("matchedRule56ReasonTags(%q, %q) = %v", tt.reason, tt.tag, matches)
			}
		})
	}
}

func testRule56Fixture(expectedDisposition string, tag string) JudgeRule56Fixture {
	return JudgeRule56Fixture{
		ID:                  "r56-test",
		Tier:                1,
		IssueFamily:         tag,
		CaseTheme:           "summary judgment test",
		MovingParty:         "defendant",
		RequestText:         "Defendant seeks summary judgment.",
		StatementOfFacts:    "Defendant says the record is undisputed.",
		OppositionText:      "Plaintiff identifies contrary sworn testimony.",
		ExpectedDisposition: expectedDisposition,
		ExpectedReasonTags:  []string{tag},
		Severity:            1,
	}
}
