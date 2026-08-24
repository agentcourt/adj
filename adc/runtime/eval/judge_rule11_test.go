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

func TestLoadJudgeRule11Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r11-test","tier":1,"issue_family":"frivolous_legal","case_theme":"Foreclosed claim.","movant":"defendant","target_party":"plaintiff","challenged_filing":"Complaint","filing_text":"Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.","notice_text":"Defendant served notice identifying the release and demanding withdrawal.","notice_served_at":"2026-07-01","motion_filed_at":"2026-07-25","motion_text":"Defendant moves for Rule 11 sanctions after no correction.","opposition_text":"Plaintiff argues the release should be ignored.","expected_granted":true,"expected_sanction_type":"admonition","expected_reason_tags":["frivolous_legal","proportional_sanction"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule11Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule11Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r11-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule11StateCreatesMotionDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule11Fixture(true, "admonition", 0, "frivolous_legal")
	state := BuildJudgeRule11State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["auto_rule11"] != true {
		t.Fatalf("auto_rule11 = %v", caseObj["auto_rule11"])
	}
	docket, _ := caseObj["docket"].([]any)
	found := false
	for _, entry := range docket {
		m, _ := entry.(map[string]any)
		if m["title"] == "Rule 11 Motion" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docket missing Rule 11 Motion: %+v", docket)
	}
}

func TestScoreJudgeRule11ResponseRejectsDeniedSanction(t *testing.T) {
	t.Parallel()

	fixture := testRule11Fixture(false, "none", 0, "weak_merits")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule11Tool,
			Arguments: map[string]any{
				"motion_index":    0,
				"granted":         false,
				"sanction_type":   "none",
				"sanction_detail": "",
				"reasoning":       "The filing was weak but not sanctionable.",
			},
		}},
	}
	result := scoreJudgeRule11Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
	if result.InvalidReason != "denied_with_sanction_type" {
		t.Fatalf("InvalidReason = %q, want denied_with_sanction_type", result.InvalidReason)
	}
}

func TestRunJudgeRule11DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r11-dry","tier":1,"issue_family":"frivolous_legal","case_theme":"Foreclosed claim.","movant":"defendant","target_party":"plaintiff","challenged_filing":"Complaint","filing_text":"Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.","notice_text":"Defendant served notice identifying the release and demanding withdrawal.","notice_served_at":"2026-07-01","motion_filed_at":"2026-07-25","motion_text":"Defendant moves for Rule 11 sanctions after no correction.","opposition_text":"Plaintiff argues the release should be ignored.","expected_granted":true,"expected_sanction_type":"admonition","expected_reason_tags":["frivolous_legal","proportional_sanction"],"severity":5}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule11Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule11(nil, JudgeRule11Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule11 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule11Summary
	if err := json.Unmarshal(rawSummary, &parsed); err != nil {
		t.Fatalf("Unmarshal summary error = %v", err)
	}
	if parsed.Total != 1 || parsed.WeightedAccuracy != 1 {
		t.Fatalf("parsed summary = %+v", parsed)
	}
	if parsed.ExecutionMode != "production" || parsed.CounterfactualModel {
		t.Fatalf("summary execution mode = %q, counterfactual = %v", parsed.ExecutionMode, parsed.CounterfactualModel)
	}
	if parsed.Provenance.SummaryMode != "run" || parsed.Provenance.ExecutionMode != "production" || parsed.Provenance.Court == nil || parsed.Provenance.Online == nil || !parsed.Provenance.DryRun || len(parsed.Provenance.Engine) == 0 {
		t.Fatalf("summary provenance = %+v", parsed.Provenance)
	}
	if parsed.Provenance.CandidatePromptSource != "production" || parsed.Provenance.CandidatePromptName != "production" {
		t.Fatalf("summary provenance = %+v", parsed.Provenance)
	}
	rawResults, err := os.ReadFile(filepath.Join(outDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile results error = %v", err)
	}
	if !strings.Contains(string(rawResults), `"lean_accepted":true`) {
		t.Fatalf("results missing accepted Lean decision: %s", rawResults)
	}
	var result JudgeRule11Result
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

func TestRunJudgeRule11DeterministicDoesNotRequireProviderCredentials(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r11-dry","tier":1,"issue_family":"frivolous_legal","case_theme":"Foreclosed claim.","movant":"defendant","target_party":"plaintiff","challenged_filing":"Complaint","filing_text":"Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.","notice_text":"Defendant served notice identifying the release and demanding withdrawal.","notice_served_at":"2026-07-01","motion_filed_at":"2026-07-25","motion_text":"Defendant moves for Rule 11 sanctions after no correction.","opposition_text":"Plaintiff argues the release should be ignored.","expected_granted":true,"expected_sanction_type":"admonition","expected_reason_tags":["frivolous_legal","proportional_sanction"],"severity":5}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule11(nil, JudgeRule11Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{writeFakeJudgeRule11Engine(t)}),
		Model:        "openai/gpt-5.6",
		Online:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule11 error = %v", err)
	}
	if summary.Correct != 1 || summary.ExecutionMode != "production" {
		t.Fatalf("summary = %+v", summary)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile results error = %v", err)
	}
	var result JudgeRule11Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}
	if result.Provider.RequestCount != 0 || len(result.ResponseExchanges) != 0 {
		t.Fatalf("deterministic result used provider: provider = %+v, exchanges = %+v", result.Provider, result.ResponseExchanges)
	}
}

func TestRescoreJudgeRule11WritesUpdatedSummary(t *testing.T) {
	t.Parallel()

	granted := true
	result := JudgeRule11Result{
		ID:                   "r11-rescore",
		Tier:                 1,
		IssueFamily:          "frivolous_legal",
		Movant:               "defendant",
		TargetParty:          "plaintiff",
		ExpectedGranted:      true,
		ExpectedSanctionType: "admonition",
		ExpectedReasonTags:   []string{"frivolous_legal"},
		Severity:             5,
		Model:                "test-model",
		PromptSource:         "production",
		PromptName:           "production",
		Granted:              &granted,
		SanctionType:         "admonition",
		SanctionDetail:       "Admonition for a filing with no legal basis.",
		Reasoning:            "The filing was frivolous and no legal basis existed.",
		LeanAccepted:         true,
		StepAccepted:         true,
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
	summary, err := RescoreJudgeRule11(JudgeRule11RescoreOptions{
		ResultsPath: resultsPath,
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("RescoreJudgeRule11 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.ReasonCorrect != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule11Summary
	if err := json.Unmarshal(rawSummary, &parsed); err != nil {
		t.Fatalf("Unmarshal summary error = %v", err)
	}
	if parsed.Provenance.SummaryMode != "rescore" || parsed.Provenance.ExecutionMode != "production" || parsed.Provenance.CandidatePromptSource != "production" || parsed.Provenance.CandidatePromptName != "production" {
		t.Fatalf("rescore provenance = %+v", parsed.Provenance)
	}
}

func testRule11Fixture(expectedGranted bool, sanctionType string, sanctionAmount float64, tags ...string) JudgeRule11Fixture {
	return JudgeRule11Fixture{
		ID:                     "r11-test",
		Tier:                   1,
		IssueFamily:            "frivolous_legal",
		CaseTheme:              "Foreclosed claim.",
		Movant:                 "defendant",
		TargetParty:            "plaintiff",
		ChallengedFiling:       "Complaint",
		FilingText:             "Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.",
		NoticeText:             "Defendant served notice identifying the release and demanding withdrawal.",
		NoticeServedAt:         "2026-07-01",
		MotionFiledAt:          "2026-07-25",
		MotionText:             "Defendant moves for Rule 11 sanctions after no correction.",
		OppositionText:         "Plaintiff argues the release should be ignored.",
		ExpectedGranted:        expectedGranted,
		ExpectedSanctionType:   sanctionType,
		ExpectedSanctionAmount: sanctionAmount,
		ExpectedReasonTags:     tags,
		Severity:               5,
	}
}

func writeFakeJudgeRule11Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"pleadings","kind":"turn","may_pass":false,"actor_message":"Current pleadings opportunity for judge: act on this objective now.","objective":"For case 0, decide Rule 11 motion_index 0 and state sanctions if granted.","allowed_tools":["decide_rule11_motion"],"step_budget":1,"priority":100,"constraints":{"required_payload":{"motion_index":0}},"deterministic_action":{"kind":"single_tool","action_type":"decide_rule11_motion","payload":{"motion_index":0,"granted":true,"sanction_type":"admonition","sanction_amount":0,"sanction_detail":"gold sanction: admonition","reasoning":"gold tags: frivolous_legal, proportional_sanction"}}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"decide_rule11_motion"}}'
  ;;
*'"action_type":"decide_rule11_motion"'*)
  printf '%s' '{"ok":true,"state":{"state_version":1,"case":{"status":"pretrial","phase":"pleadings"}}}'
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
