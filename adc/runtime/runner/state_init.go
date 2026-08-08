package runner

import (
	"time"

	"github.com/jsmorph/adj/adc/runtime/courts"
	"github.com/jsmorph/adj/adc/runtime/spec"
)

func buildInitialState(scenario spec.FormalScenario, courtProfile courts.Profile) map[string]any {
	initialCase := scenario.InitialCase
	if len(initialCase) == 0 && len(scenario.InitialCases) > 0 {
		initialCase = scenario.InitialCases[0]
	}
	caseID := deriveCaseID(initialCase)
	caption, _ := initialCase["caption"].(string)
	judge, _ := initialCase["judge"].(string)
	filedOn := deriveFiledOn(initialCase)

	policy := buildInitialPolicy(scenario.Policy)
	claim := buildSingleClaim(scenario.Claims)
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           scenario.CourtName,
		"court_profile":        courtProfile,
		"policy":               policy,
		"pleadings":            scenario.Pleadings,
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       caseID,
			"caption":                       caption,
			"judge":                         judge,
			"filed_on":                      filedOn,
			"auto_rule11":                   boolOrDefault(initialCase["auto_rule11"], false),
			"status":                        "filed",
			"trial_mode":                    "unset",
			"phase":                         "none",
			"last_pleading_served_on":       "",
			"jury_demanded_on":              "",
			"jury_configuration":            nil,
			"single_claim":                  claim,
			"jurisdictional_allegations":    nil,
			"jurors":                        []any{},
			"juror_questionnaire":           []any{},
			"juror_questionnaire_responses": []any{},
			"voir_dire_exchanges":           []any{},
			"for_cause_challenges":          []any{},
			"deliberation_round":            1,
			"juror_votes":                   []any{},
			"jury_verdict":                  nil,
			"hung_jury":                     nil,
			"contempt_counts":               []any{},
			"docket":                        []any{},
			"decision_traces":               []any{},
			"local_rule_overrides":          []any{},
			"limit_usage":                   []any{},
			"monetary_judgment":             0.0,
			"initial_disclosures":           []any{},
			"case_files":                    []any{},
			"file_events":                   []any{},
			"rule68_offers":                 []any{},
			"technical_reports":             []any{},
			"protective_orders":             []any{},
			"rule56_window_closed_for":      []any{},
			"filing_documents":              []any{},
			"juror_explanations":            []any{},
			"bench_findings":                []any{},
			"bench_conclusions":             []any{},
		},
	}
}

func boolOrDefault(v any, fallback bool) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	default:
		return fallback
	}
}

func deriveCaseID(initialCase map[string]any) string {
	caseID, _ := initialCase["case_id"].(string)
	if caseID != "" {
		return caseID
	}
	if filedOn, _ := initialCase["filed_on"].(string); filedOn != "" {
		return filedOn + "-0001"
	}
	return time.Now().UTC().Format("2006-01-02") + "-0001"
}

func deriveFiledOn(initialCase map[string]any) string {
	filedOn, _ := initialCase["filed_on"].(string)
	if filedOn != "" {
		return filedOn
	}
	return time.Now().UTC().Format("2006-01-02")
}

func buildSingleClaim(claims []spec.ClaimSpec) map[string]any {
	claim := map[string]any{
		"claim_id":          "claim-1",
		"label":             "Civil claim",
		"legal_theory":      "civil_claim",
		"standard_of_proof": "preponderance_of_the_evidence",
		"burden_holder":     "plaintiff",
		"elements":          []string{"duty", "breach", "causation", "damages"},
		"defenses":          []string{},
		"damages_question":  "What damages, if any, are proven?",
	}
	if len(claims) == 0 {
		return claim
	}
	c := claims[0]
	return map[string]any{
		"claim_id":          c.ClaimID,
		"label":             c.Label,
		"legal_theory":      c.LegalTheory,
		"standard_of_proof": c.StandardOfProof,
		"burden_holder":     c.BurdenHolder,
		"elements":          c.Elements,
		"defenses":          c.Defenses,
		"damages_question":  c.DamagesQuestion,
	}
}

func buildInitialPolicy(overrides map[string]any) map[string]any {
	policy := map[string]any{
		"max_opening_chars":                           6000,
		"max_trial_theory_chars":                      4000,
		"max_closing_chars":                           8000,
		"max_exhibits_per_side":                       20,
		"max_support_tool_calls_per_opportunity":      30,
		"max_jury_note_chars":                         3000,
		"skip_voir_dire":                              0,
		"jury_juror_count":                            6,
		"jury_unanimous_required":                     1,
		"jury_minimum_concurring":                     6,
		"voir_dire_candidate_count":                   10,
		"max_voir_dire_questions_per_side_per_juror":  1,
		"max_disallowed_voir_dire_questions_per_side": 3,
		"max_for_cause_challenges_per_side":           1,
		"max_peremptory_challenges_per_side":          1,
		"max_deliberation_rounds":                     3,
		"max_dispositive_motions_per_side_pretrial":   2,
		"max_interrogatories_per_set":                 5,
		"max_interrogatory_sets_per_side":             2,
		"max_rfp_requests_per_set":                    40,
		"max_rfp_sets_per_side":                       2,
		"max_rfa_requests_per_set":                    40,
		"max_rfa_sets_per_side":                       2,
		"max_discovery_response_deadline_days":        30,
		"max_rule12_summary_chars":                    5000,
		"max_rule56_summary_chars":                    10000,
		"max_rule56_reply_chars":                      4000,
		"max_technical_reports_per_side":              3,
		"max_technical_report_summary_chars":          5000,
	}
	for k, v := range overrides {
		policy[k] = v
	}
	if v, ok := policy["max_argument_chars"]; ok {
		if _, exists := policy["max_opening_chars"]; !exists {
			policy["max_opening_chars"] = v
		}
	}
	return normalizePolicy(policy)
}
