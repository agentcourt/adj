package runner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func evaluateAssertions(assertions []spec.AssertionSpec, state map[string]any, turnLogs []TurnLog) []map[string]any {
	results := make([]map[string]any, 0, len(assertions))
	caseObj, _ := state["case"].(map[string]any)
	for _, a := range assertions {
		passed := false
		details := ""
		switch a.Type {
		case "case_status":
			v, _ := caseObj["status"].(string)
			passed = v == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, v)
		case "trial_mode":
			v, _ := caseObj["trial_mode"].(string)
			passed = v == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, v)
		case "trial_phase":
			v, _ := caseObj["phase"].(string)
			passed = v == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, v)
		case "jury_verdict_for":
			jury, _ := caseObj["jury_verdict"].(map[string]any)
			v, _ := jury["verdict_for"].(string)
			passed = v == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, v)
		case "jury_verdict_votes_equals":
			jury, _ := caseObj["jury_verdict"].(map[string]any)
			votes := intFromAny(jury["votes_for_verdict"])
			passed = votes == a.Votes
			details = fmt.Sprintf("expected=%d got=%d", a.Votes, votes)
		case "jury_outcome_recorded":
			jury, _ := caseObj["jury_verdict"].(map[string]any)
			hung, _ := caseObj["hung_jury"].(map[string]any)
			if len(jury) > 0 {
				passed = true
				details = "jury verdict recorded"
			} else if len(hung) > 0 {
				passed = true
				details = "hung jury recorded"
			} else {
				passed = false
				details = "no jury outcome recorded"
			}
		case "decision_trace_contains_action":
			traces, _ := caseObj["decision_traces"].([]any)
			found := false
			for _, t := range traces {
				obj, _ := t.(map[string]any)
				if action, _ := obj["action"].(string); action == a.Action {
					found = true
					break
				}
			}
			passed = found
			details = fmt.Sprintf("action=%s found=%v", a.Action, found)
		case "decision_trace_contains_citation":
			traces, _ := caseObj["decision_traces"].([]any)
			found := false
			for _, t := range traces {
				obj, _ := t.(map[string]any)
				citations, _ := obj["citations"].([]any)
				for _, c := range citations {
					if cs, _ := c.(string); citationEquivalent(cs, a.Citation) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			passed = found
			details = fmt.Sprintf("citation=%s found=%v", a.Citation, found)
		case "decision_trace_action_count_min":
			traces, _ := caseObj["decision_traces"].([]any)
			count := 0
			for _, t := range traces {
				obj, _ := t.(map[string]any)
				if action, _ := obj["action"].(string); action == a.Action {
					count++
				}
			}
			passed = count >= a.MinCount
			details = fmt.Sprintf("action=%s count=%d min=%d", a.Action, count, a.MinCount)
		case "docket_entry_title_contains":
			entries, _ := caseObj["docket"].([]any)
			found := false
			for _, e := range entries {
				obj, _ := e.(map[string]any)
				title, _ := obj["title"].(string)
				if strings.Contains(title, a.Equals) {
					found = true
					break
				}
			}
			passed = found
			details = fmt.Sprintf("title_contains=%q found=%v", a.Equals, found)
		case "monetary_judgment_equals":
			amt := floatFromAny(caseObj["monetary_judgment"])
			if amt == 0 {
				jury, _ := caseObj["jury_verdict"].(map[string]any)
				amt = floatFromAny(jury["damages"])
			}
			passed = amt == a.Amount
			details = fmt.Sprintf("expected=%0.2f got=%0.2f", a.Amount, amt)
		case "judgment_count_min":
			traces, _ := caseObj["decision_traces"].([]any)
			count := 0
			for _, t := range traces {
				obj, _ := t.(map[string]any)
				if action, _ := obj["action"].(string); action == "enter_judgment" {
					count++
				}
			}
			min := a.MinCount
			if a.Count != nil && *a.Count > min {
				min = *a.Count
			}
			passed = count >= min
			details = fmt.Sprintf("count=%d min=%d", count, min)
		case "judgment_count_equals":
			traces, _ := caseObj["decision_traces"].([]any)
			count := 0
			for _, t := range traces {
				obj, _ := t.(map[string]any)
				if action, _ := obj["action"].(string); action == "enter_judgment" {
					count++
				}
			}
			expected := 0
			if a.Count != nil {
				expected = *a.Count
			}
			passed = count == expected
			details = fmt.Sprintf("count=%d expected=%d", count, expected)
		case "rule68_offer_status":
			offers, _ := caseObj["rule68_offers"].([]any)
			if len(offers) == 0 {
				passed = false
				details = "no rule68 offers recorded"
				break
			}
			targetIdx := 0
			if a.OfferIndex != nil {
				targetIdx = *a.OfferIndex
			}
			if strings.TrimSpace(a.OfferID) != "" {
				foundIdx := -1
				for i, raw := range offers {
					obj, _ := raw.(map[string]any)
					offerID, _ := obj["offer_id"].(string)
					if strings.TrimSpace(offerID) == strings.TrimSpace(a.OfferID) {
						foundIdx = i
						break
					}
				}
				if foundIdx < 0 {
					passed = false
					details = fmt.Sprintf("offer_id=%s not found", a.OfferID)
					break
				}
				targetIdx = foundIdx
			}
			if targetIdx < 0 || targetIdx >= len(offers) {
				passed = false
				details = fmt.Sprintf("offer_index=%d out of range len=%d", targetIdx, len(offers))
				break
			}
			obj, _ := offers[targetIdx].(map[string]any)
			status, _ := obj["status"].(string)
			passed = status == a.Equals
			details = fmt.Sprintf("expected=%s got=%s offer_index=%d", a.Equals, status, targetIdx)
		case "filing_document_type_contains":
			docs, _ := caseObj["filing_documents"].([]any)
			found := false
			target := strings.ToLower(strings.TrimSpace(a.Equals))
			for _, d := range docs {
				obj, _ := d.(map[string]any)
				typ, _ := obj["filing_type"].(string)
				if strings.Contains(strings.ToLower(typ), target) {
					found = true
					break
				}
			}
			if !found {
				entries, _ := caseObj["docket"].([]any)
				for _, e := range entries {
					obj, _ := e.(map[string]any)
					title, _ := obj["title"].(string)
					desc, _ := obj["description"].(string)
					hay := strings.ToLower(strings.TrimSpace(title + " " + desc))
					if strings.Contains(hay, target) {
						found = true
						break
					}
				}
			}
			passed = found
			details = fmt.Sprintf("filing_type_contains=%q found=%v", a.Equals, found)
		case "rule12_motion_disposition":
			orders := rule12OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := orders[idx]["disposition"]
			passed = got == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, got)
		case "rule12_motion_with_prejudice":
			orders := rule12OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := parseBoolField(orders[idx], "with_prejudice", false)
			want := false
			if a.Truth != nil {
				want = *a.Truth
			}
			passed = got == want
			details = fmt.Sprintf("expected=%v got=%v", want, got)
		case "rule12_motion_leave_to_amend":
			orders := rule12OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := parseBoolField(orders[idx], "leave_to_amend", false)
			want := false
			if a.Truth != nil {
				want = *a.Truth
			}
			passed = got == want
			details = fmt.Sprintf("expected=%v got=%v", want, got)
		case "rule37_motion_disposition":
			orders := rule37OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := orders[idx]["disposition"]
			passed = got == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, got)
		case "rule37_motion_sanction_type":
			orders := rule37OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := orders[idx]["sanction_type"]
			passed = got == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, got)
		case "rule11_motion_disposition":
			orders := rule11OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := orders[idx]["disposition"]
			passed = got == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, got)
		case "rule11_motion_sanction_type":
			orders := rule11OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := orders[idx]["sanction_type"]
			passed = got == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, got)
		case "rule56_motion_disposition":
			orders := rule56OrderRecords(caseObj)
			idx := 0
			if a.MotionIndex != nil {
				idx = *a.MotionIndex
			}
			if idx < 0 || idx >= len(orders) {
				passed = false
				details = fmt.Sprintf("motion_index=%d out of range len=%d", idx, len(orders))
				break
			}
			got := orders[idx]["disposition"]
			passed = got == a.Equals
			details = fmt.Sprintf("expected=%s got=%s", a.Equals, got)
		case "turns_all_autopilot_source_equals":
			if len(turnLogs) == 0 {
				passed = false
				details = "turn log is empty"
				break
			}
			mismatches := 0
			for _, turn := range turnLogs {
				if strings.TrimSpace(turn.Source) != strings.TrimSpace(a.Equals) {
					mismatches++
				}
			}
			passed = mismatches == 0
			details = fmt.Sprintf("expected_source=%s mismatches=%d turns=%d", a.Equals, mismatches, len(turnLogs))
		default:
			details = "unsupported assertion type in initial Go runner"
		}
		results = append(results, map[string]any{
			"type":    a.Type,
			"passed":  passed,
			"details": details,
		})
	}
	return results
}

func rule12OrderRecords(caseObj map[string]any) []map[string]string {
	return docketOrderRecords(caseObj, "Rule 12 Order")
}

func rule37OrderRecords(caseObj map[string]any) []map[string]string {
	return docketOrderRecords(caseObj, "Rule 37 Order")
}

func rule11OrderRecords(caseObj map[string]any) []map[string]string {
	return docketOrderRecords(caseObj, "Rule 11 Order")
}

func rule56OrderRecords(caseObj map[string]any) []map[string]string {
	return docketOrderRecords(caseObj, "Rule 56 Order")
}

func docketOrderRecords(caseObj map[string]any, titleFilter string) []map[string]string {
	entries, _ := caseObj["docket"].([]any)
	out := make([]map[string]string, 0)
	for _, e := range entries {
		obj, _ := e.(map[string]any)
		title, _ := obj["title"].(string)
		if title != titleFilter {
			continue
		}
		desc, _ := obj["description"].(string)
		out = append(out, parseKVFields(desc))
	}
	return out
}

func parseKVFields(s string) map[string]string {
	out := map[string]string{}
	for _, tok := range strings.Fields(s) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		out[k] = strings.TrimSpace(v)
	}
	return out
}

func parseBoolField(m map[string]string, key string, d bool) bool {
	raw, ok := m[key]
	if !ok {
		return d
	}
	v, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil {
		return d
	}
	return v
}
