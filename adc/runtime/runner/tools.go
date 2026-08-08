package runner

import (
	"fmt"
	"sort"
	"strings"
)

var actionSchemas = buildActionSchemas()

func buildTools(allowed []string) ([]map[string]any, error) {
	tools := make([]map[string]any, 0, len(allowed))
	missing := make([]string, 0)
	for _, name := range allowed {
		params := toolSchema(name)
		if params == nil {
			missing = append(missing, name)
			continue
		}
		tools = append(tools, map[string]any{
			"type":        "function",
			"name":        name,
			"description": "Execute " + name,
			"parameters":  params,
		})
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing tool schemas for actions: %s", strings.Join(missing, ", "))
	}
	return tools, nil
}

func BuildTools(allowed []string) ([]map[string]any, error) {
	return buildTools(allowed)
}

func buildOpportunityTools(allowed []string, reference []string, mayPass bool) ([]map[string]any, error) {
	names := append([]string{}, allowed...)
	for _, name := range reference {
		if !contains(names, name) {
			names = append(names, name)
		}
	}
	if mayPass {
		names = append(names, "pass_turn")
	}
	return buildTools(names)
}

func supportedActions() []string {
	keys := make([]string, 0, len(actionSchemas))
	for k := range actionSchemas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toolSchema(name string) map[string]any {
	return actionSchemas[name]
}

func cloneJSONValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneJSONMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = cloneJSONValue(typed[i])
		}
		return out
	default:
		return typed
	}
}

func cloneJSONMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneJSONValue(v)
	}
	return out
}

func buildActionSchemas() map[string]map[string]any {
	m := make(map[string]map[string]any)
	register := func(schema map[string]any, names ...string) {
		for _, name := range names {
			m[name] = schema
		}
	}

	register(schemaObj(map[string]any{}), "get_case", "explain_decisions", "list_case_files")
	register(schemaObj(map[string]any{"file_id": map[string]any{"type": "string"}}, "file_id"), "read_case_text_file")
	register(schemaObj(map[string]any{"file_id": map[string]any{"type": "string"}}, "file_id"), "request_case_file")
	register(schemaObj(map[string]any{"reason": map[string]any{"type": "string"}}), "pass_turn")

	register(schemaObj(
		map[string]any{
			"summary":          map[string]any{"type": "string"},
			"filed_by":         map[string]any{"type": "string"},
			"served_on":        map[string]any{"type": "string"},
			"jury_demanded_on": map[string]any{"type": "string"},
		},
		"summary",
	), "file_complaint", "file_answer", "file_amended_complaint")

	register(schemaObj(
		map[string]any{
			"party":   map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"summary": map[string]any{"type": "string"},
		},
		"party",
		"summary",
	), "serve_initial_disclosures")

	register(schemaObj(
		map[string]any{
			"source_filename": map[string]any{"type": "string", "description": "Host path for local runner use only"},
			"original_name":   map[string]any{"type": "string", "description": "Original filename when uploading file content"},
			"content_base64":  map[string]any{"type": "string", "description": "Base64-encoded file content"},
			"label":           map[string]any{"type": "string"},
		},
	), "import_case_file")

	register(schemaObj(
		map[string]any{
			"file_id":     map[string]any{"type": "string"},
			"produced_by": map[string]any{"type": "string"},
			"produced_to": map[string]any{"type": "string"},
			"request_ref": map[string]any{"type": "string"},
		},
		"file_id",
		"produced_by",
		"produced_to",
	), "produce_case_file")

	register(schemaObj(
		map[string]any{
			"file_id":     map[string]any{"type": "string"},
			"exhibit_id":  map[string]any{"type": "string"},
			"admitted":    map[string]any{"type": "boolean"},
			"description": map[string]any{"type": "string"},
		},
		"file_id",
	), "offer_case_file_as_exhibit")

	register(schemaObj(
		map[string]any{},
	), "rest_case")

	register(schemaObj(
		map[string]any{
			"scope":         map[string]any{"type": "string"},
			"target":        map[string]any{"type": "string"},
			"allowed_roles": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"note":          map[string]any{"type": "string"},
			"order_id":      map[string]any{"type": "string"},
		},
		"scope",
		"target",
		"allowed_roles",
		"note",
	), "enter_protective_order")

	register(schemaObj(
		map[string]any{
			"order_id": map[string]any{"type": "string"},
			"note":     map[string]any{"type": "string"},
		},
		"order_id",
	), "lift_protective_order")

	register(schemaObj(
		map[string]any{
			"summary":          map[string]any{"type": "string"},
			"amount":           map[string]any{"type": "number"},
			"consent_judgment": map[string]any{"type": "boolean"},
		},
		"summary",
	), "enter_settlement")

	register(schemaObj(
		map[string]any{
			"against_party": map[string]any{"type": "string"},
			"reason":        map[string]any{"type": "string"},
		},
		"against_party",
	), "enter_default")

	register(schemaObj(
		map[string]any{
			"against_party":   map[string]any{"type": "string"},
			"monetary_amount": map[string]any{"type": "number"},
			"reason":          map[string]any{"type": "string"},
		},
		"against_party",
	), "enter_default_judgment")

	register(schemaObj(
		map[string]any{
			"motion_index":   map[string]any{"type": "integer", "minimum": 0},
			"granted":        map[string]any{"type": "boolean"},
			"relief_summary": map[string]any{"type": "string"},
		},
		"motion_index",
		"granted",
	), "resolve_rule60_motion")

	register(schemaObj(
		map[string]any{
			"issue":   map[string]any{"type": "string"},
			"grounds": map[string]any{"type": "string"},
			"ruling":  map[string]any{"type": "string"},
		},
		"issue",
		"grounds",
	), "object_to_evidence")

	register(schemaObj(map[string]any{"juror_id": map[string]any{"type": "string"}}, "juror_id"), "get_juror_context")

	register(schemaObj(map[string]any{}), "issue_juror_questionnaire")

	register(schemaObj(
		map[string]any{
			"juror_id": map[string]any{"type": "string"},
			"answers": map[string]any{
				"type": "array",
				"items": schemaObj(
					map[string]any{
						"question_id": map[string]any{"type": "string"},
						"answer":      map[string]any{"type": "string"},
					},
					"question_id",
					"answer",
				),
			},
		},
		"juror_id",
		"answers",
	), "answer_juror_questionnaire")

	register(schemaObj(
		map[string]any{
			"exchange_id": map[string]any{"type": "string"},
			"juror_id":    map[string]any{"type": "string"},
			"response":    map[string]any{"type": "string"},
		},
		"exchange_id",
		"juror_id",
		"response",
	), "answer_voir_dire_question")

	register(schemaObj(
		map[string]any{
			"juror_id":    map[string]any{"type": "string"},
			"damages":     map[string]any{"type": "number"},
			"vote":        map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"confidence":  map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
			"explanation": map[string]any{"type": "string"},
		},
		"juror_id",
		"damages",
		"vote",
		"confidence",
		"explanation",
	), "submit_juror_vote")

	register(schemaObj(
		map[string]any{
			"asked_by": map[string]any{"type": "string"},
			"juror_id": map[string]any{"type": "string"},
			"question": map[string]any{"type": "string"},
		},
		"asked_by",
		"juror_id",
		"question",
	), "record_voir_dire_question")

	register(schemaObj(
		map[string]any{
			"exchange_id":   map[string]any{"type": "string"},
			"juror_id":      map[string]any{"type": "string"},
			"allowed":       map[string]any{"type": "boolean"},
			"ruling_reason": map[string]any{"type": "string"},
		},
		"exchange_id",
		"juror_id",
		"allowed",
		"ruling_reason",
	), "decide_voir_dire_question")

	register(schemaObj(
		map[string]any{
			"juror_id": map[string]any{"type": "string"},
			"grounds":  map[string]any{"type": "string"},
		},
		"juror_id",
		"grounds",
	), "challenge_juror_for_cause")

	register(schemaObj(
		map[string]any{
			"challenge_id":  map[string]any{"type": "string"},
			"juror_id":      map[string]any{"type": "string"},
			"by_party":      map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"granted":       map[string]any{"type": "boolean"},
			"ruling_reason": map[string]any{"type": "string"},
		},
		"challenge_id",
		"juror_id",
		"by_party",
		"granted",
		"ruling_reason",
	), "decide_juror_for_cause_challenge")

	register(schemaObj(
		map[string]any{
			"juror_id": map[string]any{"type": "string"},
			"party":    map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"reason":   map[string]any{"type": "string"},
		},
		"juror_id",
		"party",
	), "strike_juror_peremptorily")

	register(schemaObj(
		map[string]any{
			"issue":   map[string]any{"type": "string"},
			"finding": map[string]any{"type": "string"},
		},
		"issue",
		"finding",
	), "add_bench_finding")

	register(schemaObj(
		map[string]any{
			"issue":      map[string]any{"type": "string"},
			"conclusion": map[string]any{"type": "string"},
		},
		"issue",
		"conclusion",
	), "add_bench_conclusion")

	register(schemaObj(map[string]any{"served_on": map[string]any{"type": "string", "format": "date"}}, "served_on"), "set_last_pleading_served_on")
	register(schemaObj(map[string]any{"demanded_on": map[string]any{"type": "string", "format": "date"}}, "demanded_on"), "record_jury_demand")

	register(schemaObj(
		map[string]any{
			"parties_stipulate_nonjury": map[string]any{"type": "boolean"},
			"court_orders_jury":         map[string]any{"type": "boolean"},
		},
		"parties_stipulate_nonjury",
		"court_orders_jury",
	), "resolve_trial_mode")

	register(schemaObj(
		map[string]any{
			"next_status": map[string]any{
				"type": "string",
				"enum": []string{"filed", "pretrial", "trial", "judgment_entered", "closed"},
			},
		},
		"next_status",
	), "transition_case")

	register(schemaObj(
		map[string]any{
			"juror_count":        map[string]any{"type": "integer", "minimum": 6, "maximum": 12},
			"unanimous_required": map[string]any{"type": "boolean"},
			"minimum_concurring": map[string]any{"type": "integer", "minimum": 6, "maximum": 12},
		},
		"juror_count",
	), "set_jury_configuration")

	register(schemaObj(
		map[string]any{
			"juror_id":         map[string]any{"type": "string"},
			"name":             map[string]any{"type": "string"},
			"model":            map[string]any{"type": "string"},
			"persona_filename": map[string]any{"type": "string"},
		},
		"juror_id",
		"name",
	), "add_juror")

	register(schemaObj(
		map[string]any{
			"juror_ids": map[string]any{
				"type":     "array",
				"items":    map[string]any{"type": "string"},
				"minItems": 1,
			},
		},
		"juror_ids",
	), "empanel_jury")

	register(schemaObj(
		map[string]any{
			"phase": map[string]any{
				"type": "string",
				"enum": []string{
					"voir_dire",
					"openings",
					"plaintiff_case",
					"defense_case",
					"plaintiff_rebuttal",
					"defense_surrebuttal",
					"charge_conference",
					"closings",
					"jury_charge",
					"deliberation",
					"verdict_return",
					"post_verdict",
				},
			},
		},
		"phase",
	), "advance_trial_phase")

	register(schemaObj(
		map[string]any{
			"party":   map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"summary": map[string]any{"type": "string"},
		},
		"party",
		"summary",
	), "record_opening_statement")

	register(schemaObj(
		map[string]any{
			"party":  map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"theory": map[string]any{"type": "string"},
		},
		"party",
		"theory",
	), "submit_trial_theory")

	register(schemaObj(
		map[string]any{
			"party":        map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"report_id":    map[string]any{"type": "string"},
			"title":        map[string]any{"type": "string"},
			"summary":      map[string]any{"type": "string"},
			"method_notes": map[string]any{"type": "string"},
			"limitations":  map[string]any{"type": "string"},
			"file_id":      map[string]any{"type": "string"},
		},
		"party",
		"report_id",
		"title",
		"summary",
	), "submit_technical_report")

	register(schemaObj(
		map[string]any{
			"party":          map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"instruction_id": map[string]any{"type": "string"},
			"text":           map[string]any{"type": "string"},
			"rationale":      map[string]any{"type": "string"},
		},
		"party",
		"instruction_id",
		"text",
	), "propose_jury_instruction")

	register(schemaObj(
		map[string]any{
			"party":          map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"instruction_id": map[string]any{"type": "string"},
			"grounds":        map[string]any{"type": "string"},
		},
		"party",
		"instruction_id",
		"grounds",
	), "object_jury_instruction")

	register(schemaObj(
		map[string]any{
			"summary": map[string]any{"type": "string"},
		},
		"summary",
	), "settle_jury_instructions")

	register(schemaObj(
		map[string]any{
			"text": map[string]any{"type": "string"},
		},
		"text",
	), "deliver_jury_instructions")

	register(schemaObj(
		map[string]any{
			"party":       map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"exhibit_id":  map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"admitted":    map[string]any{"type": "boolean"},
		},
		"party",
		"exhibit_id",
		"description",
		"admitted",
	), "offer_exhibit")

	register(schemaObj(
		map[string]any{
			"party":    map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"argument": map[string]any{"type": "string"},
		},
		"party",
		"argument",
	), "deliver_closing_argument")

	register(schemaObj(map[string]any{"text": map[string]any{"type": "string"}}, "text"), "file_bench_opinion")

	register(schemaObj(
		map[string]any{
			"basis":    map[string]any{"type": "string"},
			"claim_id": map[string]any{"type": "string"},
		},
		"basis",
		"claim_id",
	), "enter_judgment")

	register(schemaObj(
		map[string]any{
			"claim_id":                map[string]any{"type": "string"},
			"verdict_for":             map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"damages":                 map[string]any{"type": "number"},
			"comparative_fault_used":  map[string]any{"type": "boolean"},
			"plaintiff_fault_pct":     map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
			"defendant_fault_pct":     map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
			"interrogatories":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"interrogatory_responses": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"claim_id",
		"verdict_for",
		"damages",
	), "record_general_verdict_with_interrogatories")

	register(schemaObj(
		map[string]any{
			"served_by":         map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"target_party":      map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"challenged_filing": map[string]any{"type": "string"},
			"grounds":           map[string]any{"type": "string"},
			"served_at":         map[string]any{"type": "string"},
		},
		"served_by",
		"target_party",
		"challenged_filing",
		"grounds",
	), "serve_rule11_safe_harbor_notice")

	register(schemaObj(
		map[string]any{
			"notice_index":       map[string]any{"type": "integer", "minimum": 0},
			"by_party":           map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"resolution_summary": map[string]any{"type": "string"},
			"resolved_at":        map[string]any{"type": "string"},
		},
		"notice_index",
		"by_party",
	), "withdraw_or_correct_filing")

	register(schemaObj(
		map[string]any{
			"movant":       map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"notice_index": map[string]any{"type": "integer", "minimum": 0},
			"filed_at":     map[string]any{"type": "string"},
		},
		"movant",
		"notice_index",
	), "file_rule11_motion")

	register(schemaObj(
		map[string]any{
			"motion_index":    map[string]any{"type": "integer", "minimum": 0},
			"granted":         map[string]any{"type": "boolean"},
			"sanction_type":   map[string]any{"type": "string", "enum": []string{"none", "admonition", "non_monetary_directive", "monetary_penalty", "fee_shift"}},
			"sanction_amount": map[string]any{"type": "number"},
			"sanction_detail": map[string]any{"type": "string"},
			"reasoning":       map[string]any{"type": "string"},
		},
		"motion_index",
		"granted",
		"sanction_detail",
		"reasoning",
	), "decide_rule11_motion")

	register(schemaObj(
		map[string]any{
			"movant":         map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"target_party":   map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"discovery_type": map[string]any{"type": "string", "enum": []string{"interrogatories", "rfp", "rfa", "initial_disclosures"}},
			"set_index":      map[string]any{"type": "integer", "minimum": 0},
			"relief_sought":  map[string]any{"type": "string"},
			"summary":        map[string]any{"type": "string"},
		},
		"movant",
		"target_party",
		"discovery_type",
		"set_index",
		"relief_sought",
		"summary",
	), "file_rule37_motion")

	register(schemaObj(
		map[string]any{
			"motion_index":    map[string]any{"type": "integer", "minimum": 0},
			"granted":         map[string]any{"type": "boolean"},
			"sanction_type":   map[string]any{"type": "string", "enum": []string{"none", "fees"}},
			"sanction_amount": map[string]any{"type": "number"},
			"order_text":      map[string]any{"type": "string"},
			"reasoning":       map[string]any{"type": "string"},
		},
		"motion_index",
		"granted",
		"sanction_type",
		"reasoning",
	), "decide_rule37_motion")

	register(schemaObj(
		map[string]any{
			"movant": map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"ground": map[string]any{"type": "string", "enum": []string{
				"lack_subject_matter_jurisdiction",
				"no_standing",
				"not_ripe",
				"moot",
				"failure_to_state_a_claim",
			}},
			"summary": map[string]any{"type": "string"},
		},
		"movant",
		"ground",
		"summary",
	), "file_rule12_motion")

	register(schemaObj(
		map[string]any{
			"motion_index": map[string]any{"type": "integer", "minimum": 0},
			"party":        map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"summary":      map[string]any{"type": "string"},
		},
		"motion_index",
		"party",
		"summary",
	), "oppose_rule12_motion", "reply_rule12_motion")

	register(schemaObj(
		map[string]any{
			"motion_index": map[string]any{"type": "integer", "minimum": 0},
			"ground": map[string]any{"type": "string", "enum": []string{
				"lack_subject_matter_jurisdiction",
				"no_standing",
				"not_ripe",
				"moot",
				"failure_to_state_a_claim",
			}},
			"disposition":                 map[string]any{"type": "string", "enum": []string{"granted", "denied"}},
			"with_prejudice":              map[string]any{"type": "boolean"},
			"leave_to_amend":              map[string]any{"type": "boolean"},
			"amendment_deadline_days":     map[string]any{"type": "integer", "minimum": 0},
			"jurisdiction_basis_rejected": map[string]any{"type": "string"},
			"injury_missing":              map[string]any{"type": "boolean"},
			"traceability_missing":        map[string]any{"type": "boolean"},
			"redressability_missing":      map[string]any{"type": "boolean"},
			"missing_elements":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"reasoning":                   map[string]any{"type": "string"},
		},
		"motion_index",
		"ground",
		"disposition",
		"reasoning",
	), "decide_rule12_motion")

	register(schemaObj(
		map[string]any{
			"jurisdiction_basis_rejected": map[string]any{"type": "string"},
			"leave_to_amend":              map[string]any{"type": "boolean"},
			"reasoning":                   map[string]any{"type": "string"},
		},
		"jurisdiction_basis_rejected",
		"reasoning",
	), "dismiss_for_lack_of_subject_matter_jurisdiction")

	register(schemaObj(
		map[string]any{
			"movant":                        map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"scope":                         map[string]any{"type": "string"},
			"statement_of_undisputed_facts": map[string]any{"type": "string"},
			"evidence_refs":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"movant",
		"scope",
		"statement_of_undisputed_facts",
	), "file_rule56_motion")

	register(schemaObj(
		map[string]any{
			"motion_index": map[string]any{"type": "integer", "minimum": 0},
			"party":        map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"summary":      map[string]any{"type": "string"},
		},
		"motion_index",
		"party",
		"summary",
	), "oppose_rule56_motion", "reply_rule56_motion")

	register(schemaObj(
		map[string]any{
			"motion_index":     map[string]any{"type": "integer", "minimum": 0},
			"disposition":      map[string]any{"type": "string", "enum": []string{"granted", "denied", "partial"}},
			"surviving_issues": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"reasoning":        map[string]any{"type": "string"},
		},
		"motion_index",
		"disposition",
		"reasoning",
	), "decide_rule56_motion")

	register(schemaObj(
		map[string]any{
			"served_by": map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"served_on": map[string]any{"type": "string", "format": "date"},
			"questions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"served_at": map[string]any{"type": "string"},
		},
		"served_by",
		"served_on",
		"questions",
	), "serve_interrogatories")

	register(schemaObj(
		map[string]any{
			"responding_party": map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"set_index":        map[string]any{"type": "integer", "minimum": 0},
			"answers":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"objections":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"responded_at":     map[string]any{"type": "string"},
		},
		"responding_party",
		"set_index",
	), "respond_interrogatories")

	register(schemaObj(
		map[string]any{
			"served_by": map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"served_on": map[string]any{"type": "string", "format": "date"},
			"requests":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"served_by",
		"served_on",
		"requests",
	), "serve_request_for_production", "serve_requests_for_admission")

	register(schemaObj(
		map[string]any{
			"responding_party":  map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"set_index":         map[string]any{"type": "integer", "minimum": 0},
			"responses":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"produced_file_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"responded_at":      map[string]any{"type": "string"},
		},
		"responding_party",
		"set_index",
	), "respond_request_for_production")

	register(schemaObj(
		map[string]any{
			"responding_party": map[string]any{"type": "string", "enum": []string{"plaintiff", "defendant"}},
			"set_index":        map[string]any{"type": "integer", "minimum": 0},
			"responses":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"responded_at":     map[string]any{"type": "string"},
		},
		"responding_party",
		"set_index",
	), "respond_requests_for_admission")

	register(schemaObj(
		map[string]any{
			"motion_type": map[string]any{"type": "string"},
			"filed_at":    map[string]any{"type": "string"},
		},
		"motion_type",
	), "file_rule59_motion")

	register(schemaObj(
		map[string]any{
			"ground":   map[string]any{"type": "string"},
			"filed_at": map[string]any{"type": "string"},
		},
		"ground",
	), "file_rule60_motion")

	register(schemaObj(
		map[string]any{
			"limit_key":      map[string]any{"type": "string"},
			"override_value": map[string]any{"type": "integer"},
			"reason":         map[string]any{"type": "string"},
		},
		"limit_key",
		"override_value",
		"reason",
	), "enter_local_rule_override")

	register(schemaObj(
		map[string]any{
			"target_role": map[string]any{"type": "string"},
			"reason":      map[string]any{"type": "string"},
			"severity":    map[string]any{"type": "string"},
		},
		"target_role",
		"reason",
		"severity",
	), "hold_in_contempt")

	register(schemaObj(
		map[string]any{
			"offer_id":    map[string]any{"type": "string"},
			"offeree":     map[string]any{"type": "string"},
			"amount":      map[string]any{"type": "number"},
			"terms":       map[string]any{"type": "string"},
			"served_at":   map[string]any{"type": "string"},
			"expires_at":  map[string]any{"type": "string"},
			"served_by":   map[string]any{"type": "string"},
			"claim_scope": map[string]any{"type": "string"},
		},
		"offeree",
		"amount",
	), "make_rule68_offer")

	register(schemaObj(
		map[string]any{
			"offer_id":    map[string]any{"type": "string"},
			"offer_index": map[string]any{"type": "integer", "minimum": 0},
			"accepted_at": map[string]any{"type": "string"},
		},
	), "accept_rule68_offer")

	register(schemaObj(
		map[string]any{
			"as_of": map[string]any{"type": "string"},
		},
	), "expire_rule68_offers")

	register(schemaObj(
		map[string]any{
			"offer_id":    map[string]any{"type": "string"},
			"offer_index": map[string]any{"type": "integer", "minimum": 0},
			"awarded_to":  map[string]any{"type": "string"},
			"amount":      map[string]any{"type": "number"},
			"reason":      map[string]any{"type": "string"},
		},
	), "evaluate_rule68_cost_shift")

	register(schemaObj(
		map[string]any{
			"with_prejudice": map[string]any{"type": "boolean"},
			"reason":         map[string]any{"type": "string"},
		},
		"with_prejudice",
	), "dismiss_case_rule41")

	register(schemaObj(
		map[string]any{
			"issues_resolved": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"amount":          map[string]any{"type": "number"},
			"basis":           map[string]any{"type": "string"},
		},
		"issues_resolved",
	), "enter_partial_judgment")

	register(schemaObj(
		map[string]any{
			"text": map[string]any{"type": "string"},
		},
		"text",
	), "enter_pretrial_order")

	register(schemaObj(
		map[string]any{
			"set_index":     map[string]any{"type": "integer", "minimum": 0},
			"responses_due": map[string]any{"type": "string"},
		},
		"set_index",
	), "finalize_interrogatory_responses")

	register(schemaObj(
		map[string]any{
			"set_index":  map[string]any{"type": "integer", "minimum": 0},
			"item_index": map[string]any{"type": "integer", "minimum": 0},
			"answer":     map[string]any{"type": "string"},
			"objection":  map[string]any{"type": "string"},
		},
		"set_index",
		"item_index",
	), "respond_interrogatory_item")

	register(schemaObj(
		map[string]any{
			"reason":      map[string]any{"type": "string"},
			"scope":       map[string]any{"type": "string"},
			"entered_at":  map[string]any{"type": "string"},
			"duration":    map[string]any{"type": "string"},
			"stay_type":   map[string]any{"type": "string"},
			"order_text":  map[string]any{"type": "string"},
			"target_case": map[string]any{"type": "string"},
		},
		"reason",
	), "order_discretionary_stay")

	register(schemaObj(
		map[string]any{
			"reason":    map[string]any{"type": "string"},
			"lifted_at": map[string]any{"type": "string"},
		},
	), "lift_stay")

	register(schemaObj(
		map[string]any{
			"amount":     map[string]any{"type": "number"},
			"bond_id":    map[string]any{"type": "string"},
			"posted_by":  map[string]any{"type": "string"},
			"posted_at":  map[string]any{"type": "string"},
			"conditions": map[string]any{"type": "string"},
		},
		"amount",
	), "post_supersedeas_bond")

	register(schemaObj(
		map[string]any{
			"party":   map[string]any{"type": "string"},
			"issue":   map[string]any{"type": "string"},
			"summary": map[string]any{"type": "string"},
		},
		"party",
		"summary",
	), "record_offer_of_proof")

	register(schemaObj(
		map[string]any{
			"motion_index":   map[string]any{"type": "integer", "minimum": 0},
			"granted":        map[string]any{"type": "boolean"},
			"relief_summary": map[string]any{"type": "string"},
		},
		"motion_index",
		"granted",
	), "resolve_rule59_motion")

	return m
}

func schemaObj(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	req := required
	if req == nil {
		req = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             req,
		"additionalProperties": false,
	}
}
