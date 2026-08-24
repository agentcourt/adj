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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeRule60Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
	if !result.ProhibitedCorrect {
		t.Fatalf("present prohibited concepts = %+v", result.PresentProhibitedConcepts)
	}
}

func TestRunJudgeRule60DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r60-dry","tier":1,"issue_family":"excusable_neglect","case_theme":"Default judgment after service routing error.","judgment_summary":"Default judgment entered for plaintiff.","motion_ground":"60b1_mistake","motion_text":"Defendant missed the answer deadline because service was routed to a closed mailbox and appeared promptly.","opposition_text":"Plaintiff argues the neglect was careless.","default_judgment":true,"expected_granted":true,"required_concepts":["excusable neglect","prompt action"],"expected_reason_tags":["mistake_excusable_neglect"],"severity":5}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule60Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule60(nil, JudgeRule60Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule60 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule60Summary
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
	if !strings.Contains(string(rawResults), `"step_accepted":true`) {
		t.Fatalf("results missing accepted step: %s", rawResults)
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

func writeFakeJudgeRule60Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"post_verdict","kind":"turn","may_pass":true,"actor_message":"Current post_verdict opportunity for judge: consider this objective and either act now or pass.","objective":"For case 0, resolve Rule 60 motion_index 0 as granted or denied with a short relief summary.","allowed_tools":["resolve_rule60_motion"],"step_budget":3,"priority":100}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","action":{"action_type":"resolve_rule60_motion","actor_role":"judge","payload":{"motion_index":0,"granted":true,"relief_summary":"excusable neglect; prompt action"}}}'
  ;;
*'"action_type":"resolve_rule60_motion"'*)
  printf '%s' '{"ok":true,"state":{"case":{"status":"judgment_entered"}}}'
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
