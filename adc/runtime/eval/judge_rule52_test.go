package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/common/openai"
)

func TestLoadJudgeRule52Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r52-test","tier":1,"issue_family":"breach_proved","case_theme":"Bench trial on unpaid goods.","complaint_text":"Plaintiff seeks payment for delivered goods.","answer_text":"Defendant denies delivery.","plaintiff_theory":"The purchase order, delivery log, and invoice prove breach.","defendant_theory":"Delivery did not occur.","admitted_evidence":["Purchase order PO-17 for $12,000.","Delivery log signed by defendant.","Unpaid invoice for $12,000."],"plaintiff_closing":"The admitted record proves delivery and nonpayment.","defendant_closing":"The delivery log should receive little weight.","expected_winner":"plaintiff","expected_amount_text":"12000","required_concepts":["purchase order","delivery log","nonpayment","12000"],"expected_reason_tags":["breach_proved","damages_proved","fact_law_separation"],"severity":5}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule52Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule52Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r52-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule52StateCreatesBenchVerdictPosture(t *testing.T) {
	t.Parallel()

	fixture := testRule52Fixture("plaintiff")
	state := BuildJudgeRule52State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["status"] != "trial" || caseObj["trial_mode"] != "bench" || caseObj["phase"] != "verdict_return" {
		t.Fatalf("case posture = status %v mode %v phase %v", caseObj["status"], caseObj["trial_mode"], caseObj["phase"])
	}
	docket, _ := caseObj["docket"].([]any)
	found := false
	for _, entry := range docket {
		m, _ := entry.(map[string]any)
		if m["title"] == "Admitted bench evidence 1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docket missing admitted evidence: %+v", docket)
	}
}

func TestScoreJudgeRule52ResponseRejectsProhibitedConcept(t *testing.T) {
	t.Parallel()

	fixture := testRule52Fixture("defendant")
	fixture.ProhibitedConcepts = []string{"excluded screenshot proves breach"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule52Tool,
			Arguments: map[string]any{
				"text":        "Findings of Fact: the excluded screenshot proves breach. Conclusions of Law: plaintiff failed to prove admissible evidence. Judgment: judgment for defendant.",
				"verdict_for": "defendant",
			},
		}},
	}
	result := scoreJudgeRule52Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.ProhibitedCorrect {
		t.Fatalf("ProhibitedCorrect = true, want false")
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
}

func TestScoreJudgeRule52ResponseAcceptsEquivalentConceptLanguage(t *testing.T) {
	t.Parallel()

	fixture := testRule52Fixture("plaintiff")
	fixture.ExpectedAmountText = "6000"
	fixture.RequiredConcepts = []string{
		"signed service agreement",
		"authenticated audit log",
		"prior orders",
		"extra freight",
		"damages not proved",
		"6000",
	}
	fixture.ExpectedReasonTags = []string{"breach_proved", "damages_proved", "fact_law_separation"}
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule52Tool,
			Arguments: map[string]any{
				"text":        "Findings of Fact: the parties executed Service Agreement SA-5. The audit log (AL-3) was authenticated. Defendant previously accepted two prior Lee-initiated orders. Plaintiff paid additional freight charges. The separate lost-profit estimate did not prove any damages amount. Conclusions of Law: plaintiff proved breach and damages. Judgment: judgment for plaintiff in the amount of $6,000.",
				"verdict_for": "plaintiff",
			},
		}},
	}
	result := scoreJudgeRule52Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if !result.RequiredCorrect {
		t.Fatalf("MissingRequiredConcepts = %v", result.MissingRequiredConcepts)
	}
	if !result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = false, result = %+v", result)
	}
}

func TestJudgeRule52AmountCorrectUsesExactMonetaryValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		expected string
		want     bool
	}{
		{name: "currency and commas", text: "Judgment is entered for $12,000.", expected: "12000", want: true},
		{name: "equivalent cents", text: "Judgment is entered for 12000.00 dollars.", expected: "$12,000", want: true},
		{name: "larger amount", text: "Judgment is entered for $112,000.", expected: "12000", want: false},
		{name: "different cents", text: "Judgment is entered for $12,000.01.", expected: "12000", want: false},
		{name: "concatenated numbers", text: "Exhibits 12 and 000 establish liability.", expected: "12000", want: false},
		{name: "partial long decimal", text: "The calculation is 12000.001.", expected: "12000", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := judgeRule52AmountCorrect(test.text, test.expected); got != test.want {
				t.Fatalf("judgeRule52AmountCorrect(%q, %q) = %t, want %t", test.text, test.expected, got, test.want)
			}
		})
	}
}

func TestJudgeRule52FixtureRejectsMalformedExpectedAmount(t *testing.T) {
	t.Parallel()

	fixture := testRule52Fixture("plaintiff")
	fixture.ExpectedAmountText = "12,00"
	if err := fixture.Validate(); err == nil || !strings.Contains(err.Error(), "invalid expected_amount_text") {
		t.Fatalf("Validate error = %v, want invalid expected_amount_text", err)
	}
}

func TestScoreJudgeRule52ResponseRequiresVerdictFor(t *testing.T) {
	t.Parallel()

	fixture := testRule52Fixture("plaintiff")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule52Tool,
			Arguments: map[string]any{
				"text": "Findings of Fact: purchase order and delivery log. Conclusions of Law: breach proved. Judgment: judgment for plaintiff in the amount of $12,000.",
			},
		}},
	}
	result := scoreJudgeRule52Response(fixture, "test-model", nil, nil, nil, nil, resp)
	if result.InvalidReason != "malformed_verdict_for" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
}

func TestRescoreJudgeRule52UsesVerdictFor(t *testing.T) {
	t.Parallel()

	fixture := testRule52Fixture("defendant")
	result := JudgeRule52Result{
		ExpectedWinner:     fixture.ExpectedWinner,
		ExpectedAmountText: "",
		RequiredConcepts:   nil,
		ProhibitedConcepts: nil,
		ExpectedReasonTags: nil,
		OpinionText:        "Findings of Fact: none. Conclusions of Law: claim failed. Judgment text contains no party label.",
		ToolPayload: map[string]any{
			"verdict_for": "defendant",
		},
	}
	rescoreJudgeRule52Result(&result)
	if result.VerdictFor != "defendant" || !result.WinnerCorrect {
		t.Fatalf("rescored result = %+v", result)
	}
}

func TestJudgeRule52ConceptAliasesCoverBenchOpinionPhrases(t *testing.T) {
	t.Parallel()

	text := strings.Join([]string{
		"Exhibit DO-2 is a draft order that lists price and quantity but contains no completed signature block.",
		"Plaintiff has not proved consequential damages.",
		"The summary spreadsheet does not identify source data or tie line items to transactions.",
		"Audit log AL-3 was authenticated by the system custodian and admitted into evidence.",
		"Defendant accepted two prior Lee orders under the same process.",
		"Plaintiff did not meet the contractual written-notice requirement.",
		"The contemporaneous service log and receiving message were more reliable than later memory testimony.",
	}, " ")
	missing := missingJudgeRule52Concepts(text, []string{
		"unsigned draft",
		"consequential damages not proved",
		"no source data",
		"authenticated audit log",
		"prior orders",
		"notice condition failed",
		"credible contemporaneous",
	})
	if len(missing) != 0 {
		t.Fatalf("missing concepts = %v", missing)
	}
}

func testRule52Fixture(expectedWinner string) JudgeRule52Fixture {
	return JudgeRule52Fixture{
		ID:                 "r52-test",
		Tier:               1,
		IssueFamily:        "breach_proved",
		CaseTheme:          "Bench trial on unpaid goods.",
		ComplaintText:      "Plaintiff seeks payment for delivered goods.",
		AnswerText:         "Defendant denies delivery.",
		PlaintiffTheory:    "The purchase order, delivery log, and invoice prove breach.",
		DefendantTheory:    "Delivery did not occur.",
		AdmittedEvidence:   []string{"Purchase order PO-17 for $12,000.", "Delivery log signed by defendant.", "Unpaid invoice for $12,000."},
		PlaintiffClosing:   "The admitted record proves delivery and nonpayment.",
		DefendantClosing:   "The delivery log should receive little weight.",
		ExpectedWinner:     expectedWinner,
		ExpectedAmountText: "12000",
		RequiredConcepts:   []string{"purchase order", "delivery log", "12000"},
		ExpectedReasonTags: []string{"breach_proved", "damages_proved", "fact_law_separation"},
		Severity:           5,
	}
}
