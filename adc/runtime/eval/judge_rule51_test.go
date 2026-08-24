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

func TestLoadJudgeRule51Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r51-test","tier":1,"issue_family":"burden_standard","case_theme":"Burden instruction","claim_summary":"Contract claim.","plaintiff_instruction":"Plaintiff proposes preponderance of the evidence.","defendant_instruction":"Defendant proposes clear and convincing evidence.","expected_required_terms":["preponderance","burden"],"expected_prohibited_terms":["clear and convincing"],"expected_reason_tags":["burden_standard"],"severity":3}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule51Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule51Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r51-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule51StateCreatesChargeDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule51Fixture()
	state := BuildJudgeRule51State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["phase"] != "jury_charge" || caseObj["trial_mode"] != "jury" {
		t.Fatalf("case phase/trial_mode = %v/%v", caseObj["phase"], caseObj["trial_mode"])
	}
	docket, _ := caseObj["docket"].([]any)
	var hasProposal bool
	for _, item := range docket {
		entry, _ := item.(map[string]any)
		if entry["title"] == "Proposed jury instruction - plaintiff" && strings.Contains(entry["description"].(string), "PI-1") {
			hasProposal = true
		}
	}
	if !hasProposal {
		t.Fatalf("docket missing plaintiff proposal: %+v", docket)
	}
}

func TestScoreJudgeRule51ResponseDetectsProhibitedTerm(t *testing.T) {
	t.Parallel()

	fixture := testRule51Fixture()
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule51Tool,
			Arguments: map[string]any{
				"summary": "The final instructions use preponderance of the evidence and tell the jury the defendant breached.",
			},
		}},
	}
	result := scoreJudgeRule51Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
	if len(result.PresentProhibitedTerms) != 1 || result.PresentProhibitedTerms[0] != "defendant breached" {
		t.Fatalf("PresentProhibitedTerms = %v", result.PresentProhibitedTerms)
	}
}

func TestRule51ProhibitedTermIgnoresRejectedContext(t *testing.T) {
	t.Parallel()

	summary := "The objection is sustained and the instruction saying defendant breached is rejected. Final instructions use if you find breach."
	present := presentRule51Terms(summary, []string{"defendant breached"})
	if len(present) != 0 {
		t.Fatalf("presentRule51Terms = %v, want none", present)
	}
}

func TestRule51RequiredTermEquivalents(t *testing.T) {
	t.Parallel()

	if !rule51ContainsTerm("The breach caused the plaintiff's claimed loss.", "causation") {
		t.Fatalf("causation equivalent was not accepted")
	}
	if !rule51ContainsTerm("Decide the case on the evidence admitted at trial.", "admitted evidence") {
		t.Fatalf("admitted evidence equivalent was not accepted")
	}
	if !rule51ContainsTerm("You may but are not required to infer that the tickets were unfavorable.", "may infer") {
		t.Fatalf("may infer equivalent was not accepted")
	}
	if !rule51ContainsTerm("You must not draw any adverse inference from the missing draft.", "no adverse inference") {
		t.Fatalf("no adverse inference equivalent was not accepted")
	}
}

func TestRunJudgeRule51DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r51-dry","tier":1,"issue_family":"burden_standard","case_theme":"Burden instruction","claim_summary":"Contract claim.","plaintiff_instruction":"Plaintiff proposes preponderance of the evidence.","defendant_instruction":"Defendant proposes clear and convincing evidence.","defendant_objection":"Clear and convincing is not the civil burden.","expected_required_terms":["preponderance","burden"],"expected_prohibited_terms":["clear and convincing"],"expected_reason_tags":["burden_standard"],"severity":3}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule51Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule51(nil, JudgeRule51Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule51 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule51Summary
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

func TestRescoreJudgeRule51WritesUpdatedSummary(t *testing.T) {
	t.Parallel()

	result := JudgeRule51Result{
		ID:                      "r51-rescore",
		Tier:                    1,
		IssueFamily:             "argumentative",
		ExpectedRequiredTerms:   []string{"jury", "facts"},
		ExpectedProhibitedTerms: []string{"defendant lied"},
		ExpectedReasonTags:      []string{"argumentative"},
		Severity:                5,
		Model:                   "test-model",
		PromptSource:            "production",
		PromptName:              "production",
		Summary:                 "The objection is sustained because the instruction saying defendant lied is argumentative and rejected. Final instruction: the jury decides the facts.",
		LeanAccepted:            true,
		StepAccepted:            true,
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
	summary, err := RescoreJudgeRule51(JudgeRule51RescoreOptions{
		ResultsPath: resultsPath,
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("RescoreJudgeRule51 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.ReasonCorrect != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(outDir, "summary.json")); err != nil {
		t.Fatalf("Stat summary error = %v", err)
	}
}

func testRule51Fixture() JudgeRule51Fixture {
	return JudgeRule51Fixture{
		ID:                      "r51-test",
		Tier:                    1,
		IssueFamily:             "assumes_fact",
		CaseTheme:               "Disputed breach instruction",
		ClaimSummary:            "Plaintiff claims defendant breached a delivery contract.",
		PlaintiffInstruction:    "Because defendant breached, award damages.",
		DefendantInstruction:    "If you find breach, consider damages.",
		DefendantObjection:      "Plaintiff's instruction assumes a disputed breach.",
		ExpectedRequiredTerms:   []string{"if you find", "preponderance"},
		ExpectedProhibitedTerms: []string{"defendant breached"},
		ExpectedReasonTags:      []string{"assumes_fact"},
		Severity:                5,
	}
}

func writeFakeJudgeRule51Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"jury_charge","kind":"turn","may_pass":true,"actor_message":"Current jury_charge opportunity for judge: act on this objective now.","objective":"For case 0, settle jury instructions with a concise summary of the final instruction set.","allowed_tools":["settle_jury_instructions"],"step_budget":1,"priority":100}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"settle_jury_instructions"}}'
  ;;
*'"action_type":"settle_jury_instructions"'*)
  printf '%s' '{"ok":true,"state":{"state_version":1,"case":{"status":"trial","phase":"jury_charge"}}}'
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
