package proceeding

import (
	"fmt"
	"strings"
)

const (
	opportunityFailureDeadline          = "deadline_expired"
	opportunityFailureAttemptsExhausted = "attempts_exhausted"
	opportunityFailureRequestFailed     = "request_failed"
	opportunityFailureAgentExited       = "agent_exited"
	opportunityFailureAgentOutputLimit  = "agent_output_limit_exceeded"
)

func (rc *runContext) failOpportunity(opportunity Opportunity, reason string, message string, details map[string]any) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("opportunity failure reason is required")
	}
	payload := map[string]any{
		"type":           "opportunity_failed",
		"role":           opportunity.Role,
		"phase":          opportunity.Phase,
		"opportunity_id": opportunity.ID,
		"reason":         reason,
		"message":        strings.TrimSpace(message),
	}
	if payload["message"] == "" {
		payload["message"] = opportunityFailureMessage(opportunity, reason)
	}
	if opportunity.Role == "council" {
		memberID := councilMemberIDFromOpportunity(opportunity)
		if memberID == "" {
			memberID = mapString(details["member_id"])
		}
		payload["member_id"] = memberID
	}
	for key, value := range details {
		if strings.TrimSpace(key) != "" && value != nil {
			payload[key] = value
		}
	}
	stepResp, err := rc.stepForCertificate("fail_opportunity", "system", payload)
	if err != nil {
		return err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return fmt.Errorf("fail_opportunity rejected: %s", mapString(stepResp["error"]))
	}
	rc.state = mapAny(stepResp["state"])
	if err := rc.recordEvent("opportunity_failed", opportunity.Role, opportunity.Phase, payload); err != nil {
		return err
	}
	if opportunity.Role == "council" {
		eventPayload := map[string]any{
			"member_id":      mapString(payload["member_id"]),
			"status":         "failed",
			"failure_reason": reason,
			"cause":          mapString(payload["message"]),
		}
		if err := rc.recordEvent("council_member_removed", "system", opportunity.Phase, eventPayload); err != nil {
			return err
		}
	}
	return nil
}

func (rc *runContext) signalRoleAPIs() {
	if rc.lawyerAPI != nil {
		rc.lawyerAPI.signalChanged()
	}
	if rc.councilAPI != nil {
		rc.councilAPI.signalChanged()
	}
}

func opportunityFailureMessage(opportunity Opportunity, reason string) string {
	role := strings.TrimSpace(opportunity.Role)
	if role == "" {
		role = "unknown"
	}
	phase := strings.TrimSpace(opportunity.Phase)
	if phase == "" {
		phase = "unknown"
	}
	opportunityID := strings.TrimSpace(opportunity.ID)
	if opportunityID == "" {
		opportunityID = phase + ":" + role
	}
	switch reason {
	case opportunityFailureDeadline:
		return fmt.Sprintf("%s opportunity %s failed because the deadline expired.", titleCase(role), opportunityID)
	case opportunityFailureAttemptsExhausted:
		return fmt.Sprintf("%s opportunity %s failed because attempts were exhausted.", titleCase(role), opportunityID)
	case opportunityFailureRequestFailed:
		return fmt.Sprintf("%s opportunity %s failed because the request failed.", titleCase(role), opportunityID)
	case opportunityFailureAgentExited:
		return fmt.Sprintf("%s opportunity %s failed because the assigned agent process exited before completing the opportunity.", titleCase(role), opportunityID)
	case opportunityFailureAgentOutputLimit:
		return fmt.Sprintf("%s opportunity %s failed because the assigned agent process exceeded the output limit before completing the opportunity.", titleCase(role), opportunityID)
	default:
		return fmt.Sprintf("%s opportunity %s failed: %s.", titleCase(role), opportunityID, reason)
	}
}

func caseFailure(state map[string]any) map[string]any {
	raw := mapAny(mapAny(state["case"])["failure"])
	if len(raw) == 0 {
		return raw
	}
	out := cloneMap(raw)
	if out["type"] == nil && out["failure_type"] != nil {
		out["type"] = out["failure_type"]
		delete(out, "failure_type")
	}
	return out
}

func caseFailureError(state map[string]any) string {
	failure := caseFailure(state)
	if message := mapString(failure["message"]); message != "" {
		return message
	}
	if reason := mapString(failure["reason"]); reason != "" {
		return "Case failed: " + reason + "."
	}
	return ""
}
