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

func TestLoadJudgeRule58Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r58-test","tier":1,"issue_family":"jury_plaintiff","case_theme":"Jury verdict for plaintiff.","trial_mode":"jury","verdict_for":"plaintiff","verdict_damages":12000,"expected_claim_id":"claim-1","expected_basis":"jury verdict","expected_amount":12000,"required_basis_concepts":["jury verdict"],"expected_reason_tags":["jury_verdict"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule58Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule58Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r58-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule58StateCreatesJuryPostVerdictPosture(t *testing.T) {
	t.Parallel()

	fixture := testRule58JuryFixture()
	state := BuildJudgeRule58State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["status"] != "trial" || caseObj["trial_mode"] != "jury" || caseObj["phase"] != "post_verdict" {
		t.Fatalf("case posture = status %v mode %v phase %v", caseObj["status"], caseObj["trial_mode"], caseObj["phase"])
	}
	verdict, _ := caseObj["jury_verdict"].(map[string]any)
	if verdict["verdict_for"] != "plaintiff" || verdict["damages"] != 12000.0 {
		t.Fatalf("jury_verdict = %+v", verdict)
	}
}

func TestScoreJudgeRule58ResponseRejectsWrongClaim(t *testing.T) {
	t.Parallel()

	fixture := testRule58JuryFixture()
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule58Tool,
			Arguments: map[string]any{
				"claim_id": "claim-2",
				"basis":    "jury verdict",
			},
		}},
	}
	result := scoreJudgeRule58Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.ClaimCorrect {
		t.Fatalf("ClaimCorrect = true, want false")
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
}

func TestFinalizeJudgeRule58ResultChecksAppliedAmount(t *testing.T) {
	t.Parallel()

	fixture := testRule58JuryFixture()
	resp := openai.Response{ToolCalls: []openai.ToolCall{{
		Name: JudgeRule58Tool,
		Arguments: map[string]any{
			"claim_id": fixture.ExpectedClaimID,
			"basis":    fixture.ExpectedBasis,
		},
	}}}
	result := scoreJudgeRule58Response(fixture, "test-model", nil, nil, nil, nil, resp)
	result.LeanAccepted = true
	result.StepAccepted = true
	result.AppliedCaseStatus = "judgment_entered"
	result.AppliedMonetaryJudgment = 12000
	finalizeJudgeRule58Result(&result)
	if !result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = false, result = %+v", result)
	}
	result.AppliedMonetaryJudgment = 11000
	finalizeJudgeRule58Result(&result)
	if result.AmountCorrect {
		t.Fatalf("AmountCorrect = true after wrong amount")
	}
}

func TestRunJudgeRule58DeterministicWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r58-dry","tier":1,"issue_family":"jury_plaintiff","case_theme":"Jury verdict for plaintiff.","trial_mode":"jury","verdict_for":"plaintiff","verdict_damages":12000,"expected_claim_id":"claim-1","expected_basis":"jury verdict","expected_amount":12000,"required_basis_concepts":["jury verdict"],"expected_reason_tags":["jury_verdict"],"severity":5}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule58Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule58(nil, JudgeRule58Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "test-model",
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule58 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule58Summary
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
	if !strings.Contains(string(rawResults), `"applied_case_status":"judgment_entered"`) {
		t.Fatalf("results missing applied judgment state: %s", rawResults)
	}
	var result JudgeRule58Result
	if err := json.Unmarshal(rawResults, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}
	if result.Provider.RequestCount != 0 || len(result.ResponseExchanges) != 0 {
		t.Fatalf("deterministic result used provider: provider = %+v, exchanges = %+v", result.Provider, result.ResponseExchanges)
	}
	if result.ExecutionMode != "production" || result.CounterfactualModel {
		t.Fatalf("execution mode = %q, counterfactual = %v", result.ExecutionMode, result.CounterfactualModel)
	}
}

func testRule58JuryFixture() JudgeRule58Fixture {
	return JudgeRule58Fixture{
		ID:                    "r58-test",
		Tier:                  1,
		IssueFamily:           "jury_plaintiff",
		CaseTheme:             "Jury verdict for plaintiff.",
		TrialMode:             "jury",
		VerdictFor:            "plaintiff",
		VerdictDamages:        12000,
		ExpectedClaimID:       "claim-1",
		ExpectedBasis:         "jury verdict",
		ExpectedAmount:        12000,
		RequiredBasisConcepts: []string{"jury verdict"},
		ExpectedReasonTags:    []string{"jury_verdict"},
		Severity:              5,
	}
}

func writeFakeJudgeRule58Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"post_verdict","kind":"turn","may_pass":true,"actor_message":"Current post_verdict opportunity for judge: consider this objective and either act now or pass.","objective":"For case 0, enter judgment with basis jury verdict.","allowed_tools":["enter_judgment"],"constraints":{"required_payload":{"case_id":"judge-rule58-r58-dry","claim_id":"claim-1","basis":"jury verdict"}},"step_budget":1,"priority":100,"deterministic_action":{"kind":"single_tool","action_type":"enter_judgment","payload":{"claim_id":"claim-1","basis":"jury verdict"}}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","action":{"action_type":"enter_judgment","actor_role":"judge","payload":{"claim_id":"claim-1","basis":"jury verdict"}}}'
  ;;
*'"action_type":"enter_judgment"'*)
  printf '%s' '{"ok":true,"state":{"case":{"status":"judgment_entered","monetary_judgment":12000}}}'
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
