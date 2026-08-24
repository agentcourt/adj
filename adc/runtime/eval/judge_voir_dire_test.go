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
	result := scoreJudgeVoirDireResponse(fixture, "test-model", false, nil, nil, nil, nil, resp)
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
	result := scoreJudgeVoirDireResponse(fixture, "test-model", false, nil, nil, nil, nil, resp)
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

func TestRunJudgeVoirDireDryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"jvd-dry","tier":1,"question_family":"proper_bias_probe","case_theme":"theme","asked_by":"plaintiff","juror_id":"J1","question":"Can you be fair to both sides?","expected_allowed":true,"expected_reason_tags":["proper_bias_probe"],"severity":1}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeVoirDireEngine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeVoirDire(nil, JudgeVoirDireOptions{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeVoirDire error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeVoirDireSummary
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

func TestRunJudgeVoirDireDryRunUsesPromptOverride(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"jvd-prompt","tier":1,"question_family":"proper_bias_probe","case_theme":"theme","asked_by":"plaintiff","juror_id":"J1","question":"Can you be fair to both sides?","expected_allowed":true,"expected_reason_tags":["proper_bias_probe"],"severity":1}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	promptPath := filepath.Join(t.TempDir(), "candidate.md")
	promptText := "Variant prompt for {{question}} by {{asked_by}} to {{juror_id}} in {{case_theme}}. Production was {{production_objective}}."
	if err := os.WriteFile(promptPath, []byte(promptText), 0o644); err != nil {
		t.Fatalf("WriteFile prompt error = %v", err)
	}
	engineScript := writeFakeJudgeVoirDireEngine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeVoirDire(nil, JudgeVoirDireOptions{
		FixturesPath:          fixturePath,
		OutputDir:             outDir,
		OpportunityPromptPath: promptPath,
		OpportunityPromptName: "candidate-test",
		Engine:                lean.New([]string{engineScript}),
		Model:                 "dry-model",
		DryRun:                true,
		Timeout:               time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeVoirDire error = %v", err)
	}
	if summary.PromptName != "candidate-test" || summary.PromptSource != "file:"+promptPath {
		t.Fatalf("prompt summary = source %q name %q", summary.PromptSource, summary.PromptName)
	}
	copiedPrompt, err := os.ReadFile(filepath.Join(outDir, "opportunity_prompt.md"))
	if err != nil {
		t.Fatalf("ReadFile copied prompt error = %v", err)
	}
	if string(copiedPrompt) != promptText {
		t.Fatalf("copied prompt = %q, want %q", copiedPrompt, promptText)
	}
	rawResults, err := os.ReadFile(filepath.Join(outDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile results error = %v", err)
	}
	resultText := string(rawResults)
	if !strings.Contains(resultText, "Variant prompt for Can you be fair to both sides? by plaintiff to J1 in theme") {
		t.Fatalf("results missing rendered prompt: %s", resultText)
	}
	if !strings.Contains(resultText, `"prompt_name":"candidate-test"`) {
		t.Fatalf("results missing prompt metadata: %s", resultText)
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

func writeFakeJudgeVoirDireEngine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"voir_dire","kind":"turn","may_pass":false,"actor_message":"Current voir_dire opportunity for judge: act on this objective now.","objective":"For case 0, rule on the pending voir dire question by plaintiff to juror_id J1. Allow a narrow question that tests bias. Disallow a precommitment.","allowed_tools":["decide_voir_dire_question"],"step_budget":3,"priority":100,"constraints":{"required_payload":{"exchange_id":"vx-1","juror_id":"J1","asked_by":"plaintiff"}}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"decide_voir_dire_question"}}'
  ;;
*'"action_type":"decide_voir_dire_question"'*)
  printf '%s' '{"ok":true,"state":{"state_version":1,"case":{"status":"jury_selection","phase":"voir_dire"}}}'
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
