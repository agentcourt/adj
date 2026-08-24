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
	result := scoreJudgeRule37Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
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

func TestRunJudgeRule37DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r37-dry","tier":1,"issue_family":"no_response","case_theme":"No interrogatory response.","movant":"plaintiff","target_party":"defendant","discovery_type":"interrogatories","set_index":0,"request_text":"Identify witnesses with knowledge of the delivery failure.","response_text":"No response served by the deadline.","motion_text":"Plaintiff moves to compel complete answers and requests $750 in fees.","opposition_text":"Defendant offers no justification for missing the deadline.","expected_granted":true,"expected_sanction_type":"fees","expected_sanction_amount":750,"expected_reason_tags":["no_response","fees"],"severity":5}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule37Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule37(nil, JudgeRule37Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule37 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule37Summary
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
	var result JudgeRule37Result
	if err := json.Unmarshal(rawResults, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}
	if result.Provider.RequestCount != 0 || len(result.ResponseExchanges) != 0 {
		t.Fatalf("deterministic result used provider: provider = %+v, exchanges = %+v", result.Provider, result.ResponseExchanges)
	}
	if result.FinalState == nil {
		t.Fatalf("deterministic result missing final state")
	}
	if result.ExecutionMode != "production" || result.CounterfactualModel {
		t.Fatalf("execution mode = %q, counterfactual = %v", result.ExecutionMode, result.CounterfactualModel)
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

func writeFakeJudgeRule37Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"discovery","kind":"turn","may_pass":true,"actor_message":"Current discovery opportunity for judge: act on this objective now.","objective":"For case 0, decide Rule 37 motion_index 0 and include sanction decision.","allowed_tools":["decide_rule37_motion"],"step_budget":1,"priority":100,"constraints":{"required_payload":{"motion_index":0}},"deterministic_action":{"kind":"single_tool","action_type":"decide_rule37_motion","payload":{"motion_index":0,"granted":true,"sanction_type":"fees","sanction_amount":750,"order_text":"motion granted; compel discovery response and award fees","reasoning":"gold tags: no_response, fees"}}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"decide_rule37_motion"}}'
  ;;
*'"action_type":"decide_rule37_motion"'*)
  printf '%s' '{"ok":true,"state":{"state_version":1,"case":{"status":"pretrial","phase":"discovery"}}}'
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
