package proceeding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	openaiapi "github.com/jsmorph/adj/common/openai"
)

func (rc *runContext) executeCouncilOpportunity(ctx context.Context, client councilResponseClient, opportunity Opportunity) error {
	memberID := councilMemberIDFromOpportunity(opportunity)
	seat, ok := rc.findCouncilSeat(memberID)
	if !ok {
		return fmt.Errorf("unknown council member %q", memberID)
	}
	switch NormalizeCouncilBackend(rc.cfg.CouncilBackend) {
	case councilBackendAPI:
		return rc.executeCouncilAPIOpportunity(ctx, opportunity, seat)
	}
	ctx, cancel := withTimeout(ctx, rc.cfg.Runtime.CouncilTimeout())
	defer cancel()

	prompt, err := rc.buildCouncilPrompt(seat, opportunity)
	if err != nil {
		return err
	}
	if err := rc.writeCouncilTurnSnapshot(&councilTurn{
		opportunity:       opportunity,
		seat:              seat,
		turnNumber:        rc.turn,
		prompt:            prompt,
		deadline:          time.Now().Add(rc.cfg.Runtime.CouncilTimeout()),
		attemptsMax:       rc.cfg.Runtime.InvalidAttemptLimit,
		attemptsRemaining: rc.cfg.Runtime.InvalidAttemptLimit,
		evidenceBudget:    &evidenceReadBudget{},
	}, prompt); err != nil {
		return err
	}
	inputItems := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": "Call submit_council_vote exactly once for this opportunity."},
	}
	tools := []map[string]any{
		{
			"type":        "function",
			"name":        "submit_council_vote",
			"description": "Submit one council vote for the current deliberation opportunity.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"member_id": map[string]any{"type": "string"},
					"vote":      map[string]any{"type": "string", "enum": []string{"demonstrated", "not_demonstrated"}},
					"rationale": map[string]any{"type": "string"},
				},
				"required":             []string{"member_id", "vote", "rationale"},
				"additionalProperties": false,
			},
		},
	}
	prevID := ""
	invalidAttempts := 0
	invalidAttemptReasons := make([]string, 0)
	recordInvalidAttempt := func(reason string) {
		invalidAttempts++
		invalidAttemptReasons = append(invalidAttemptReasons, strings.TrimSpace(reason))
	}
	maxOutputTokens := rc.cfg.Runtime.CouncilMaxOutputTokens
	for invalidAttempts < rc.cfg.Runtime.InvalidAttemptLimit {
		resp, err := rc.createCouncilResponse(ctx, client, seat, inputItems, tools, prevID, maxOutputTokens)
		if err != nil {
			if isFunctionArgumentParseError(err) {
				recordInvalidAttempt(err.Error())
				inputItems = append(inputItems, map[string]any{
					"role":    "user",
					"content": "The previous tool call arguments were malformed. Call submit_council_vote exactly once with valid JSON arguments and keep the rationale brief.",
				})
				continue
			}
			if isCouncilTimeoutError(err) {
				return rc.removeTimedOutCouncilMember(opportunity, seat, err)
			}
			if isCouncilRequestError(err) {
				return rc.removeRequestFailedCouncilMember(opportunity, seat, err)
			}
			return err
		}
		if size, err := jsonPayloadSize(resp); err != nil {
			return err
		} else if size > rc.cfg.Runtime.MaxResponseBytes {
			recordInvalidAttempt(councilResponseOversizeReason(size, rc.cfg.Runtime.MaxResponseBytes))
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": councilResponseOversizeCorrection(size, rc.cfg.Runtime.MaxResponseBytes),
			})
			continue
		}
		prevID = resp.ResponseID
		if len(resp.ToolCalls) != 1 {
			recordInvalidAttempt("Call submit_council_vote exactly once.")
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": "Call submit_council_vote exactly once.",
			})
			continue
		}
		call := resp.ToolCalls[0]
		if call.Name != "submit_council_vote" {
			recordInvalidAttempt("The only allowed tool is submit_council_vote.")
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": "The only allowed tool is submit_council_vote.",
			})
			continue
		}
		payload := cloneMap(call.Arguments)
		payload["member_id"] = memberID
		if mapString(payload["vote"]) == "" || mapString(payload["rationale"]) == "" {
			recordInvalidAttempt("submit_council_vote requires vote and rationale.")
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": "submit_council_vote requires vote and rationale.",
			})
			continue
		}
		stepResp, err := rc.stepForCertificate("submit_council_vote", "council", payload)
		if err != nil {
			return err
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			reason := mapString(stepResp["error"])
			recordInvalidAttempt(reason)
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": reason,
			})
			continue
		}
		rc.state = mapAny(stepResp["state"])
		if rc.lawyerAPI != nil {
			rc.lawyerAPI.signalChanged()
		}
		if rc.councilAPI != nil {
			rc.councilAPI.signalChanged()
		}
		eventPayload := map[string]any{
			"member_id": memberID,
			"model":     seat.Model,
			"payload":   payload,
		}
		if resp.ResponseID != "" {
			eventPayload["response_id"] = resp.ResponseID
		}
		if seat.RequestSpec != nil {
			eventPayload["request_spec"] = seat.RequestSpec
		}
		if resp.OpenRouterMetadata != nil {
			eventPayload["openrouter_metadata"] = resp.OpenRouterMetadata
		}
		if resp.OpenRouterGeneration != nil {
			eventPayload["openrouter_generation"] = resp.OpenRouterGeneration
		}
		if resp.OpenRouterGenerationError != "" {
			eventPayload["openrouter_generation_error"] = resp.OpenRouterGenerationError
		}
		return rc.recordEvent("council_vote", "council", opportunity.Phase, eventPayload)
	}
	limitErr := formatInvalidAttemptLimitError(fmt.Sprintf("council member %s", memberID), invalidAttemptReasons)
	return rc.removeInvalidResponseCouncilMember(opportunity, seat, limitErr)
}

func (rc *runContext) createCouncilResponse(
	ctx context.Context,
	client councilResponseClient,
	seat CouncilSeat,
	inputItems []map[string]any,
	tools []map[string]any,
	prevID string,
	defaultMaxOutputTokens int64,
) (openaiapi.Response, error) {
	if seat.RequestSpec == nil {
		return openaiapi.Response{}, fmt.Errorf("council member %s has no request_spec; JSONL council pool records are required", seat.MemberID)
	}
	spec := seat.RequestSpec.WithFallbackMaxOutputTokens(defaultMaxOutputTokens)
	return client.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, prevID)
}

func (rc *runContext) removeTimedOutCouncilMember(opportunity Opportunity, seat CouncilSeat, cause error) error {
	return rc.removeCouncilMember(opportunity, seat, opportunityFailureDeadline, cause)
}

func (rc *runContext) removeRequestFailedCouncilMember(opportunity Opportunity, seat CouncilSeat, cause error) error {
	return rc.removeCouncilMember(opportunity, seat, opportunityFailureRequestFailed, cause)
}

func (rc *runContext) removeInvalidResponseCouncilMember(opportunity Opportunity, seat CouncilSeat, cause error) error {
	return rc.removeCouncilMember(opportunity, seat, opportunityFailureAttemptsExhausted, cause)
}

func (rc *runContext) removeCouncilMember(opportunity Opportunity, seat CouncilSeat, reason string, cause error) error {
	memberID := seat.MemberID
	details := map[string]any{
		"member_id": memberID,
		"model":     seat.Model,
	}
	if class := openaiapi.ErrorClass(cause); class != "" {
		details["error_class"] = string(class)
		rc.providerErrorClass = string(class)
	}
	if err := rc.failOpportunity(opportunity, reason, cause.Error(), details); err != nil {
		return err
	}
	rc.signalRoleAPIs()
	return nil
}

func (rc *runContext) findCouncilSeat(memberID string) (CouncilSeat, bool) {
	for _, seat := range rc.council {
		if seat.MemberID == memberID {
			return seat, true
		}
	}
	return CouncilSeat{}, false
}

func councilMemberIDFromOpportunity(opportunity Opportunity) string {
	parts := strings.Split(opportunity.ID, ":")
	if len(parts) == 3 {
		return strings.TrimSpace(parts[2])
	}
	return ""
}

func (rc *runContext) buildCouncilPrompt(seat CouncilSeat, _ Opportunity) (string, error) {
	personaSection := ""
	if strings.TrimSpace(seat.PersonaText) != "" {
		personaSection = "Persona:\n" + strings.TrimSpace(seat.PersonaText) + "\n"
	}
	return rc.cfg.renderPromptFile("council.md", map[string]string{
		"MEMBER_ID":          seat.MemberID,
		"DELIBERATION_ROUND": fmt.Sprintf("%v", mapAny(rc.state["case"])["deliberation_round"]),
		"PROPOSITION":        rc.complaint.Proposition,
		"EVIDENCE_STANDARD":  currentEvidenceStandard(rc.state, rc.cfg.Policy),
		"PERSONA_SECTION":    personaSection,
		"RECORD":             rc.renderCouncilRecord(),
	})
}

func isFunctionArgumentParseError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "parse function arguments")
}

func isCouncilTimeoutError(err error) bool {
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
	return strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out")
}

func isCouncilRequestError(err error) bool {
	return openaiapi.ErrorClass(err) != ""
}

func councilResponseOversizeReason(size int, limit int) string {
	return fmt.Sprintf("council response exceeded byte limit of %d bytes (got %d)", limit, size)
}

func councilResponseOversizeCorrection(size int, limit int) string {
	return fmt.Sprintf("Your response payload was %d bytes; the limit is %d bytes. Call submit_council_vote exactly once with only vote and a concise rationale. Do not include analysis outside the tool call.", size, limit)
}

func (rc *runContext) renderCouncilRecord() string {
	caseObj := mapAny(rc.state["case"])
	sections := []string{
		"Openings:\n" + renderFilingList(mapList(caseObj["openings"])),
		"Arguments:\n" + renderFilingList(mapList(caseObj["arguments"])),
		"Rebuttals:\n" + renderFilingList(mapList(caseObj["rebuttals"])),
		"Surrebuttals:\n" + renderFilingList(mapList(caseObj["surrebuttals"])),
		"Closings:\n" + renderFilingList(mapList(caseObj["closings"])),
		"Exhibits:\n" + rc.renderExhibits(mapList(caseObj["offered_evidence"])),
		"Submitted evidence:\n" + renderSubmittedEvidence(mapList(caseObj["submitted_evidence"])),
		"Technical reports:\n" + renderReports(mapList(caseObj["technical_reports"])),
	}
	prior := rc.renderPriorVotes(mapList(caseObj["council_votes"]), intNumber(caseObj["deliberation_round"]))
	if prior != "" {
		sections = append(sections, "Prior rounds:\n"+prior)
	}
	return strings.Join(sections, "\n\n")
}

func (rc *runContext) councilView(seat CouncilSeat, opportunity Opportunity) map[string]any {
	caseObj := mapAny(rc.state["case"])
	return map[string]any{
		"proposition":       rc.complaint.Proposition,
		"evidence_standard": currentEvidenceStandard(rc.state, rc.cfg.Policy),
		"phase":             currentPhase(rc.state),
		"member": map[string]any{
			"member_id":        seat.MemberID,
			"model":            seat.Model,
			"persona_filename": seat.PersonaFile,
		},
		"opportunity": map[string]any{
			"id":            opportunity.ID,
			"role":          opportunity.Role,
			"phase":         opportunity.Phase,
			"objective":     opportunity.Objective,
			"allowed_tools": opportunity.AllowedTools,
			"may_pass":      opportunity.MayPass,
		},
		"record": map[string]any{
			"evidence":           rc.listVisibleEvidence(),
			"openings":           mapList(caseObj["openings"]),
			"arguments":          mapList(caseObj["arguments"]),
			"rebuttals":          mapList(caseObj["rebuttals"]),
			"surrebuttals":       mapList(caseObj["surrebuttals"]),
			"closings":           mapList(caseObj["closings"]),
			"submitted_evidence": mapList(caseObj["submitted_evidence"]),
			"exhibits":           rc.attorneyExhibits(),
			"technical_reports":  mapList(caseObj["technical_reports"]),
			"prior_council_votes": mapList(
				caseObj["council_votes"],
			),
		},
	}
}

func renderFilingList(items []map[string]any) string {
	if len(items) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("[%s] %s", mapString(item["role"]), mapString(item["text"])))
	}
	return strings.Join(lines, "\n\n")
}

func renderReports(items []map[string]any) string {
	if len(items) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("[%s] %s\n%s", mapString(item["role"]), mapString(item["title"]), mapString(item["summary"])))
	}
	return strings.Join(lines, "\n\n")
}

func (rc *runContext) renderExhibits(items []map[string]any) string {
	return rc.renderExhibitBodies(items)
}

func (rc *runContext) renderExhibitBodies(items []map[string]any) string {
	if len(items) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		evidenceID := mapString(item["evidence_id"])
		label := mapString(item["label"])
		if label == "" {
			label = evidenceID
		}
		file, ok := rc.fileByID[evidenceID]
		if !ok {
			lines = append(lines, fmt.Sprintf("[%s] %s\n(unavailable file)", mapString(item["role"]), label))
			continue
		}
		body := "(binary or non-text file)"
		if file.TextReadable {
			body = file.Text
		}
		lines = append(lines, fmt.Sprintf("[%s] %s\n%s", mapString(item["role"]), label, body))
	}
	return strings.Join(lines, "\n\n")
}

func (rc *runContext) renderExhibitIndex(items []map[string]any) string {
	if len(items) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		evidenceID := mapString(item["evidence_id"])
		label := mapString(item["label"])
		phase := mapString(item["phase"])
		role := mapString(item["role"])
		name := evidenceID
		if file, ok := rc.fileByID[evidenceID]; ok && strings.TrimSpace(file.Name) != "" {
			name = file.Name
		}
		if label == "" {
			lines = append(lines, fmt.Sprintf("[%s %s] %s", role, phase, name))
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s %s] %s: %s", role, phase, label, name))
	}
	return strings.Join(lines, "\n")
}

func (rc *runContext) renderPriorVotes(votes []map[string]any, currentRound int) string {
	if currentRound <= 1 {
		return ""
	}
	lines := make([]string, 0)
	for _, vote := range votes {
		round := intNumber(vote["round"])
		if round >= currentRound {
			continue
		}
		lines = append(lines, fmt.Sprintf("Round %d [%s] %s\n%s", round, mapString(vote["member_id"]), mapString(vote["vote"]), mapString(vote["rationale"])))
	}
	return strings.Join(lines, "\n\n")
}

func intNumber(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
		return 0
	default:
		return 0
	}
}
