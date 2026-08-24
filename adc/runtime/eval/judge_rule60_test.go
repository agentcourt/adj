package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeRule60Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r60-test","tier":1,"issue_family":"excusable_neglect","case_theme":"Default judgment after service routing error.","judgment_summary":"Default judgment entered for plaintiff.","motion_ground":"60b1_mistake","motion_text":"Defendant missed the answer deadline because service was routed to a closed mailbox and appeared promptly.","opposition_text":"Plaintiff argues the neglect was careless.","default_judgment":true,"expected_granted":true,"required_concepts":["excusable neglect","prompt action"],"expected_reason_tags":["mistake_excusable_neglect"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule60Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule60Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r60-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule60StateCreatesPendingMotionPosture(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(true)
	state := BuildJudgeRule60State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["status"] != "judgment_entered" {
		t.Fatalf("status = %v", caseObj["status"])
	}
	docket, _ := caseObj["docket"].([]any)
	found := false
	for _, entry := range docket {
		m, _ := entry.(map[string]any)
		if m["title"] == "Rule 60 Motion" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docket missing Rule 60 Motion: %+v", docket)
	}
}

func TestScoreJudgeRule60ResponseRejectsWrongGrant(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        true,
				"relief_summary": "ordinary litigation argument repeats trial arguments",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.GrantCorrect {
		t.Fatalf("GrantCorrect = true, want false")
	}
}

func TestScoreJudgeRule60ResponseAcceptsOrdinaryLitigationReason(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	fixture.ExpectedReasonTags = []string{"ordinary_reargument"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        false,
				"relief_summary": "ordinary litigation argument",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.ReasonCorrect {
		t.Fatalf("ReasonCorrect = false, matched tags = %+v", result.MatchedReasonTags)
	}
}

func TestScoreJudgeRule60ResponseAllowsNegatedProhibitedConcept(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	fixture.RequiredConcepts = []string{"no extraordinary circumstances"}
	fixture.ProhibitedConcepts = []string{"extraordinary circumstances"}
	fixture.ExpectedReasonTags = []string{"extraordinary"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        false,
				"relief_summary": "no extraordinary circumstances",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.ProhibitedCorrect {
		t.Fatalf("ProhibitedCorrect = false, present prohibited concepts = %+v", result.PresentProhibitedConcepts)
	}
}

func TestScoreJudgeRule60ResponseAllowsStandardStatement(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	fixture.RequiredConcepts = []string{"no extraordinary circumstances", "ordinary litigation argument"}
	fixture.ProhibitedConcepts = []string{"extraordinary circumstances"}
	fixture.ExpectedReasonTags = []string{"extraordinary", "ordinary_reargument"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        false,
				"relief_summary": "Denied. Rule 60(b)(6) requires extraordinary circumstances; movant shows only regret and payment inconvenience.",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.RequiredCorrect || !result.ProhibitedCorrect || !result.ReasonCorrect {
		t.Fatalf("result = %+v", result)
	}
}

func TestScoreJudgeRule60ResponseAllowsNoFraudShowing(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	fixture.RequiredConcepts = []string{"ordinary litigation argument"}
	fixture.ProhibitedConcepts = []string{"fraud"}
	fixture.ExpectedReasonTags = []string{"fraud", "ordinary_reargument"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        false,
				"relief_summary": "Denied. The asserted minor inconsistency is impeachment only and does not show by clear and convincing evidence fraud, misrepresentation, or misconduct under Rule 60(b)(3).",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.ProhibitedCorrect {
		t.Fatalf("present prohibited concepts = %+v", result.PresentProhibitedConcepts)
	}
}

func TestScoreJudgeRule60ResponseAllowsNoExtraordinaryShowing(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	fixture.RequiredConcepts = []string{"ordinary litigation argument", "no extraordinary circumstances"}
	fixture.ProhibitedConcepts = []string{"extraordinary circumstances"}
	fixture.ExpectedReasonTags = []string{"extraordinary", "ordinary_reargument"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        false,
				"relief_summary": "Denied under Rule 60(b)(6). The motion re-urges arguments already rejected and shows no intervening change of law, new evidence, or extraordinary circumstance.",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.RequiredCorrect || !result.ProhibitedCorrect {
		t.Fatalf("result = %+v", result)
	}
}

func TestScoreJudgeRule60ResponseAllowsNoGroundList(t *testing.T) {
	t.Parallel()

	fixture := testRule60Fixture(false)
	fixture.RequiredConcepts = []string{"ordinary litigation argument"}
	fixture.ProhibitedConcepts = []string{"extraordinary circumstances"}
	fixture.ExpectedReasonTags = []string{"ordinary_reargument"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule60Tool,
			Arguments: map[string]any{
				"motion_index":   0,
				"granted":        false,
				"relief_summary": "Denied. The motion seeks to relitigate witness credibility and reweigh the trial evidence. No mistake, newly discovered evidence, fraud, void judgment, satisfaction, or extraordinary circumstances are shown.",
			},
		}},
	}
	result := scoreJudgeRule60Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.ProhibitedCorrect {
		t.Fatalf("present prohibited concepts = %+v", result.PresentProhibitedConcepts)
	}
}

func testRule60Fixture(expectedGranted bool) JudgeRule60Fixture {
	return JudgeRule60Fixture{
		ID:                 "r60-test",
		Tier:               1,
		IssueFamily:        "excusable_neglect",
		CaseTheme:          "Default judgment after service routing error.",
		JudgmentSummary:    "Default judgment entered for plaintiff.",
		MotionGround:       "60b1_mistake",
		MotionText:         "Defendant missed the answer deadline because service was routed to a closed mailbox and appeared promptly.",
		OppositionText:     "Plaintiff argues the neglect was careless.",
		DefaultJudgment:    true,
		ExpectedGranted:    expectedGranted,
		RequiredConcepts:   []string{"excusable neglect", "prompt action"},
		ExpectedReasonTags: []string{"mistake_excusable_neglect"},
		Severity:           5,
	}
}
