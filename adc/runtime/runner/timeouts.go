package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "deadline exceeded")
}

func isCandidateJurorOpportunity(opportunity leanOpportunity) bool {
	return opportunity.Phase == "voir_dire" &&
		(contains(opportunity.AllowedTools, "answer_juror_questionnaire") ||
			contains(opportunity.AllowedTools, "answer_voir_dire_question"))
}

func isDeliberationJurorOpportunity(opportunity leanOpportunity) bool {
	return opportunity.Phase == "deliberation" &&
		contains(opportunity.AllowedTools, "submit_juror_vote")
}

func (r *Runner) handleOpportunityResponseError(
	turnIndex int,
	role spec.RoleSpec,
	opportunity leanOpportunity,
	model string,
	err error,
) (TurnLog, bool, error) {
	if role.Name != "juror" || err == nil {
		return TurnLog{}, false, nil
	}
	jurorID := strings.TrimSpace(opportunityConstraintString(opportunity, "juror_id"))
	if jurorID == "" {
		return TurnLog{}, false, nil
	}
	if isCandidateJurorOpportunity(opportunity) {
		log, handleErr := r.handleCandidateJurorTimeout(turnIndex, opportunity, model, jurorID, err)
		return log, true, handleErr
	}
	if isDeliberationJurorOpportunity(opportunity) {
		log, handleErr := r.handleDeliberatingJurorTimeout(turnIndex, opportunity, model, jurorID, err)
		return log, true, handleErr
	}
	return TurnLog{}, false, nil
}

func (r *Runner) handleCandidateJurorTimeout(
	turnIndex int,
	opportunity leanOpportunity,
	model string,
	jurorID string,
	cause error,
) (TurnLog, error) {
	transcript := make([]map[string]any, 0, 2)
	timeoutRes, err := r.executeAction(turnIndex, 1, "system", "process_juror_timeout", map[string]any{"juror_id": jurorID})
	if err != nil {
		return TurnLog{}, err
	}
	if ok, _ := timeoutRes.Result["ok"].(bool); !ok {
		return TurnLog{}, fmt.Errorf("%s", issueText(issueFromResult("process_juror_timeout", timeoutRes.Result)))
	}
	transcript = append(transcript, map[string]any{
		"action": "process_juror_timeout",
		"arguments": map[string]any{
			"juror_id": jurorID,
		},
		"result": timeoutRes.Result,
	})
	replacementNumber := nextJurorNumber(r.state)
	replacementID := fmt.Sprintf("J%d", replacementNumber)
	addRes, err := r.executeAction(turnIndex, 2, "clerk", "add_juror", map[string]any{
		"juror_id": replacementID,
		"name":     fmt.Sprintf("Juror %d", replacementNumber),
	})
	if err != nil {
		return TurnLog{}, err
	}
	if ok, _ := addRes.Result["ok"].(bool); !ok {
		return TurnLog{}, fmt.Errorf("%s", issueText(issueFromResult("add_juror", addRes.Result)))
	}
	transcript = append(transcript, map[string]any{
		"action": "add_juror",
		"arguments": map[string]any{
			"juror_id": replacementID,
			"name":     fmt.Sprintf("Juror %d", replacementNumber),
		},
		"result": addRes.Result,
	})
	if err := r.persistAgentEvent(turnIndex, 1, "juror", "agent_timeout", map[string]any{
		"model":                strings.TrimSpace(model),
		"opportunity_id":       opportunity.OpportunityID,
		"phase":                opportunity.Phase,
		"juror_id":             jurorID,
		"status":               "timed_out",
		"outcome":              "candidate_replaced",
		"replacement_juror_id": replacementID,
		"error":                strings.TrimSpace(cause.Error()),
	}); err != nil {
		return TurnLog{}, err
	}
	return TurnLog{
		Role:       "juror",
		Prompt:     opportunity.Objective,
		Steps:      2,
		Transcript: transcript,
	}, nil
}

func (r *Runner) handleDeliberatingJurorTimeout(
	turnIndex int,
	opportunity leanOpportunity,
	model string,
	jurorID string,
	cause error,
) (TurnLog, error) {
	transcript := make([]map[string]any, 0, 1)
	timeoutRes, err := r.executeAction(turnIndex, 1, "system", "process_juror_timeout", map[string]any{"juror_id": jurorID})
	if err != nil {
		return TurnLog{}, err
	}
	if ok, _ := timeoutRes.Result["ok"].(bool); !ok {
		return TurnLog{}, fmt.Errorf("%s", issueText(issueFromResult("process_juror_timeout", timeoutRes.Result)))
	}
	transcript = append(transcript, map[string]any{
		"action": "process_juror_timeout",
		"arguments": map[string]any{
			"juror_id": jurorID,
		},
		"result": timeoutRes.Result,
	})
	payload := map[string]any{
		"model":          strings.TrimSpace(model),
		"opportunity_id": opportunity.OpportunityID,
		"phase":          opportunity.Phase,
		"juror_id":       jurorID,
		"status":         "timed_out",
		"outcome":        "deliberating_juror_removed",
		"error":          strings.TrimSpace(cause.Error()),
	}
	caseObj, _ := r.state["case"].(map[string]any)
	if caseObj != nil {
		if _, ok := caseObj["hung_jury"].(map[string]any); ok {
			payload["hung_jury_declared"] = true
		}
		if _, ok := caseObj["jury_verdict"].(map[string]any); ok {
			payload["jury_verdict_derived"] = true
		}
	}
	if err := r.persistAgentEvent(turnIndex, 1, "juror", "agent_timeout", payload); err != nil {
		return TurnLog{}, err
	}
	return TurnLog{
		Role:       "juror",
		Prompt:     opportunity.Objective,
		Steps:      1,
		Transcript: transcript,
	}, nil
}
