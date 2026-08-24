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
	result := scoreJudgeRule56Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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

	normalizedMatch := scoreJudgeRule56Response(fixture, "test-model", false, nil, nil, nil, nil, response([]any{"  DAMAGES ", "causation"}))
	if !normalizedMatch.SurvivingCorrect || !normalizedMatch.OutcomeCorrect {
		t.Fatalf("normalized match = %+v", normalizedMatch)
	}

	mismatch := scoreJudgeRule56Response(fixture, "test-model", false, nil, nil, nil, nil, response([]any{"causation"}))
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
	result := scoreJudgeRule56Response(fixture, "test-model", false, nil, nil, nil, nil, openai.Response{
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

func TestRunJudgeRule56DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r56-dry","tier":1,"issue_family":"missing_element","case_theme":"theme","moving_party":"defendant","request_text":"Defendant seeks summary judgment on causation.","statement_of_undisputed_facts":"Plaintiff has no causation evidence.","opposition_text":"Plaintiff concedes the record has no causation witness or document.","expected_disposition":"granted","expected_reason_tags":["missing_element"],"severity":1}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule56Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule56(nil, JudgeRule56Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule56 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule56Summary
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
	var parsedResult JudgeRule56Result
	if err := json.Unmarshal(rawResults, &parsedResult); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}
	if ok, _ := parsedResult.View["ok"].(bool); !ok {
		t.Fatalf("recorded view lacks ok wrapper: %#v", parsedResult.View)
	}
	if _, ok := parsedResult.View["view"].(map[string]any); !ok {
		t.Fatalf("recorded view lacks nested view: %#v", parsedResult.View)
	}
	systemPrompt, _ := parsedResult.Input[0]["content"].(string)
	if !strings.Contains(systemPrompt, `"ok":true`) || !strings.Contains(systemPrompt, `"view":{`) {
		t.Fatalf("prompt lacks full role_view response: %s", systemPrompt)
	}
}

func TestRunJudgeRule56RejectedApplyCannotScoreCorrect(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r56-rejected","tier":1,"issue_family":"missing_element","case_theme":"theme","moving_party":"defendant","request_text":"Defendant seeks summary judgment on causation.","statement_of_undisputed_facts":"Plaintiff has no causation evidence.","opposition_text":"Plaintiff identifies no causation evidence.","expected_disposition":"granted","expected_reason_tags":["missing_element"],"severity":1}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule56(nil, JudgeRule56Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{writeFakeJudgeRule56EngineWithApply(t, false)}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule56 error = %v", err)
	}
	if summary.Correct != 0 {
		t.Fatalf("Correct = %d, want 0", summary.Correct)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "results.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var result JudgeRule56Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.OutcomeCorrect || result.LeanAccepted || result.StepAccepted {
		t.Fatalf("result = %+v", result)
	}
	if result.InvalidReason != "procedural_invalid_attempt_limit" || !strings.Contains(result.LeanError, "rejected for test") {
		t.Fatalf("result = %+v", result)
	}
	if len(result.ResponseExchanges) != 3 || result.Provider.RequestCount != 3 {
		t.Fatalf("result = %+v", result)
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

func writeFakeJudgeRule56Engine(t *testing.T) string {
	return writeFakeJudgeRule56EngineWithApply(t, true)
}

func writeFakeJudgeRule56EngineWithApply(t *testing.T, accept bool) string {
	t.Helper()

	applyResponse := `{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"decide_rule56_motion"}}`
	if !accept {
		applyResponse = `{"ok":false,"error":"rejected for test"}`
	}
	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"pretrial","kind":"turn","may_pass":false,"actor_message":"Current pretrial opportunity for judge: act on this objective now.","objective":"For case 0, decide Rule 56 motion_index 0 with disposition granted, denied, or partial, and explain the decisive record-based reason.","allowed_tools":["decide_rule56_motion"],"step_budget":3,"priority":100,"constraints":{}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '` + applyResponse + `'
  ;;
*'"action_type":"decide_rule56_motion"'*)
  printf '%s' '{"ok":true,"state":{"state_version":1,"case":{"status":"pretrial","phase":"pretrial"}}}'
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
