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
	caseCtx := ctx
	memberID := councilMemberIDFromOpportunity(opportunity)
	rc.mu.Lock()
	seat, ok := rc.findCouncilSeatLocked(memberID)
	rc.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown council member %q", memberID)
	}
	switch NormalizeCouncilBackend(rc.cfg.CouncilBackend) {
	case councilBackendAPI:
		return rc.executeCouncilAPIOpportunity(ctx, opportunity, seat)
	}
	turnCtx, cancel := withTimeout(caseCtx, rc.cfg.Runtime.CouncilTimeout())
	defer cancel()
	turnDeadline, ok := turnCtx.Deadline()
	if !ok {
		return fmt.Errorf("council turn context has no deadline")
	}

	rc.mu.Lock()
	prompt, err := rc.buildCouncilPrompt(seat, opportunity)
	if err != nil {
		rc.mu.Unlock()
		return err
	}
	if err := rc.writeCouncilTurnSnapshot(&councilTurn{
		opportunity:       opportunity,
		seat:              seat,
		turnNumber:        rc.turn,
		prompt:            prompt,
		deadline:          turnDeadline,
		attemptsMax:       rc.cfg.Runtime.InvalidAttemptLimit,
		attemptsRemaining: rc.cfg.Runtime.InvalidAttemptLimit,
		evidenceBudget:    &evidenceReadBudget{},
	}, prompt); err != nil {
		rc.mu.Unlock()
		return err
	}
	rc.mu.Unlock()
	requestPrompt, err := rc.councilDirectRequestPrompt(seat, opportunity)
	if err != nil {
		return err
	}
	inputItems := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": requestPrompt},
	}
	tools := []map[string]any{
		{
			"type":        "function",
			"name":        "submit_council_vote",
			"description": rc.cfg.modelToolDescription("Submit one council vote for the current deliberation opportunity."),
			"parameters":  submitCouncilVoteSchema(),
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
		resp, err := rc.createCouncilResponse(turnCtx, client, seat, inputItems, tools, prevID, maxOutputTokens)
		if err != nil {
			if cause := context.Cause(caseCtx); cause != nil {
				return cause
			}
			if isFunctionArgumentParseError(err) {
				recordInvalidAttempt(err.Error())
				repair, renderErr := rc.councilDirectRepairPrompt(seat, opportunity, "malformed_arguments", "", 0, 0)
				if renderErr != nil {
					return renderErr
				}
				inputItems = append(inputItems, map[string]any{
					"role":    "user",
					"content": repair,
				})
				continue
			}
			if isCouncilTimeoutError(err) || errors.Is(context.Cause(turnCtx), context.DeadlineExceeded) {
				return rc.removeTimedOutCouncilMember(caseCtx, turnDeadline, opportunity, seat, err)
			}
			if isCouncilRequestError(err) {
				return rc.removeRequestFailedCouncilMember(caseCtx, turnDeadline, opportunity, seat, err)
			}
			return err
		}
		if size, err := jsonPayloadSize(resp); err != nil {
			return err
		} else if size > rc.cfg.Runtime.MaxResponseBytes {
			recordInvalidAttempt(councilResponseOversizeReason(size, rc.cfg.Runtime.MaxResponseBytes))
			repair, err := rc.councilDirectRepairPrompt(seat, opportunity, "response_too_large", "", size, rc.cfg.Runtime.MaxResponseBytes)
			if err != nil {
				return err
			}
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": repair,
			})
			continue
		}
		prevID = resp.ResponseID
		if len(resp.ToolCalls) != 1 {
			recordInvalidAttempt("Call submit_council_vote exactly once.")
			repair, err := rc.councilDirectRepairPrompt(seat, opportunity, "tool_call_count", "", 0, 0)
			if err != nil {
				return err
			}
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": repair,
			})
			continue
		}
		call := resp.ToolCalls[0]
		if call.Name != "submit_council_vote" {
			recordInvalidAttempt("The only allowed tool is submit_council_vote.")
			repair, err := rc.councilDirectRepairPrompt(seat, opportunity, "wrong_tool", "", 0, 0)
			if err != nil {
				return err
			}
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": repair,
			})
			continue
		}
		payload := cloneMap(call.Arguments)
		if err := validateCouncilVotePayload(payload); err != nil {
			reason := ensureTerminalPeriod(err.Error())
			recordInvalidAttempt(reason)
			repair, renderErr := rc.councilDirectRepairPrompt(seat, opportunity, "invalid_arguments", reason, 0, 0)
			if renderErr != nil {
				return renderErr
			}
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": repair,
			})
			continue
		}
		payload["member_id"] = memberID
		rc.mu.Lock()
		stepResp, replayAction, err := rc.evaluateTurnStepLocked(caseCtx, turnDeadline, opportunity, "submit_council_vote", "council", payload)
		if err != nil {
			if errors.Is(err, errTurnDeadlineExceeded) {
				deadlineErr := rc.failCouncilOpportunityLocked(caseCtx, time.Time{}, opportunity, seat, opportunityFailureDeadline, rc.directCouncilDeadlineError(opportunity, seat))
				rc.mu.Unlock()
				return deadlineErr
			}
			rc.mu.Unlock()
			return err
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			reason := mapString(stepResp["error"])
			rc.mu.Unlock()
			return fmt.Errorf("%s", reason)
		}
		nextState, _, err := acceptedStepState(stepResp, opportunity.StateVersion)
		if err != nil {
			rc.mu.Unlock()
			return err
		}
		if err := turnStepError(caseCtx, turnDeadline, nil); err != nil {
			if errors.Is(err, errTurnDeadlineExceeded) {
				err = rc.failCouncilOpportunityLocked(caseCtx, time.Time{}, opportunity, seat, opportunityFailureDeadline, rc.directCouncilDeadlineError(opportunity, seat))
			}
			rc.mu.Unlock()
			return err
		}
		rc.state = nextState
		rc.certificateActions = append(rc.certificateActions, replayAction)
		rc.signalRoleAPIsLocked()
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
		eventErr := rc.recordEventLocked("council_vote", "council", opportunity.Phase, eventPayload)
		rc.mu.Unlock()
		return eventErr
	}
	limitErr := formatInvalidAttemptLimitError(fmt.Sprintf("council member %s", memberID), invalidAttemptReasons)
	return rc.removeInvalidResponseCouncilMember(caseCtx, turnDeadline, opportunity, seat, limitErr)
}

func (rc *runContext) councilDirectRequestPrompt(seat CouncilSeat, opportunity Opportunity) (string, error) {
	return rc.cfg.renderPromptFile(promptCouncilDirectRequest, map[string]string{
		"COUNCIL_TOOL":   "submit_council_vote",
		"MEMBER_ID":      seat.MemberID,
		"OPPORTUNITY_ID": opportunity.ID,
	})
}

func (rc *runContext) councilDirectRepairPrompt(seat CouncilSeat, opportunity Opportunity, kind string, reason string, size int, limit int) (string, error) {
	values := map[string]string{
		"REPAIR_KIND":       kind,
		"COUNCIL_TOOL":      "submit_council_vote",
		"SUBMISSION_FIELDS": "vote and rationale",
		"MEMBER_ID":         seat.MemberID,
		"OPPORTUNITY_ID":    opportunity.ID,
		"SIZE_BYTES":        fmt.Sprintf("%d", size),
		"LIMIT_BYTES":       fmt.Sprintf("%d", limit),
		"REASON":            reason,
	}
	componentID := ""
	switch kind {
	case "malformed_arguments":
		componentID = promptCouncilRepairMalformed
	case "response_too_large":
		componentID = promptCouncilRepairOversize
	case "tool_call_count":
		componentID = promptCouncilRepairCallCount
	case "wrong_tool":
		componentID = promptCouncilRepairWrongTool
	case "invalid_arguments":
		componentID = promptCouncilRepairArguments
	default:
		return "", fmt.Errorf("unknown council direct repair kind %q", kind)
	}
	correction, err := rc.cfg.renderPromptFile(componentID, values)
	if err != nil {
		return "", err
	}
	values["CORRECTION"] = correction
	return rc.cfg.renderPromptFile(promptCouncilDirectRepair, values)
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

func (rc *runContext) removeTimedOutCouncilMember(ctx context.Context, commitDeadline time.Time, opportunity Opportunity, seat CouncilSeat, cause error) error {
	return rc.removeCouncilMember(ctx, commitDeadline, opportunity, seat, opportunityFailureDeadline, cause)
}

func (rc *runContext) removeRequestFailedCouncilMember(ctx context.Context, commitDeadline time.Time, opportunity Opportunity, seat CouncilSeat, cause error) error {
	return rc.removeCouncilMember(ctx, commitDeadline, opportunity, seat, opportunityFailureRequestFailed, cause)
}

func (rc *runContext) removeInvalidResponseCouncilMember(ctx context.Context, commitDeadline time.Time, opportunity Opportunity, seat CouncilSeat, cause error) error {
	return rc.removeCouncilMember(ctx, commitDeadline, opportunity, seat, opportunityFailureAttemptsExhausted, cause)
}

func (rc *runContext) removeCouncilMember(ctx context.Context, commitDeadline time.Time, opportunity Opportunity, seat CouncilSeat, reason string, cause error) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.failCouncilOpportunityLocked(ctx, commitDeadline, opportunity, seat, reason, cause)
}

func (rc *runContext) failCouncilOpportunityLocked(ctx context.Context, commitDeadline time.Time, opportunity Opportunity, seat CouncilSeat, reason string, cause error) error {
	memberID := seat.MemberID
	details := map[string]any{
		"member_id": memberID,
		"model":     seat.Model,
	}
	if class := openaiapi.ErrorClass(cause); class != "" {
		details["error_class"] = string(class)
		rc.providerErrorClass = string(class)
	}
	if err := rc.failOpportunityLocked(ctx, commitDeadline, opportunity, reason, cause.Error(), details); err != nil {
		if !errors.Is(err, errTurnDeadlineExceeded) {
			return err
		}
		deadlineErr := rc.directCouncilDeadlineError(opportunity, seat)
		if deadlineFailureErr := rc.failOpportunityLocked(ctx, time.Time{}, opportunity, opportunityFailureDeadline, deadlineErr.Error(), details); deadlineFailureErr != nil {
			return errors.Join(err, deadlineFailureErr)
		}
	}
	return nil
}

func (rc *runContext) directCouncilDeadlineError(opportunity Opportunity, seat CouncilSeat) error {
	return fmt.Errorf("council member %s opportunity %s timed out: %w", seat.MemberID, opportunity.ID, errTurnDeadlineExceeded)
}

func (rc *runContext) findCouncilSeatLocked(memberID string) (CouncilSeat, bool) {
	for _, seat := range rc.council {
		if seat.MemberID == memberID {
			return seat, true
		}
	}
	return CouncilSeat{}, false
}

func councilMemberIDFromOpportunity(opportunity Opportunity) string {
	return strings.TrimSpace(opportunity.MemberID)
}

func (rc *runContext) buildCouncilPrompt(seat CouncilSeat, opportunity Opportunity) (string, error) {
	personaSection := ""
	if strings.TrimSpace(seat.PersonaText) != "" {
		var err error
		personaSection, err = rc.cfg.renderPromptFile(promptCouncilPersona, map[string]string{
			"PERSONA":        strings.TrimSpace(seat.PersonaText),
			"MEMBER_ID":      seat.MemberID,
			"MODEL":          seat.Model,
			"PERSONA_FILE":   seat.PersonaFile,
			"OPPORTUNITY_ID": opportunity.ID,
		})
		if err != nil {
			return "", err
		}
	}
	return rc.cfg.renderPromptFile(promptCouncilSystem, map[string]string{
		"MEMBER_ID":          seat.MemberID,
		"DELIBERATION_ROUND": fmt.Sprintf("%v", mapAny(rc.state["case"])["deliberation_round"]),
		"PROPOSITION":        rc.complaint.Proposition,
		"EVIDENCE_STANDARD":  currentEvidenceStandard(rc.state, rc.cfg.Policy),
		"PERSONA_SECTION":    personaSection,
		"RECORD":             rc.renderCouncilRecord(),
		"OPPORTUNITY_ID":     opportunity.ID,
		"OBJECTIVE":          opportunity.Objective,
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
			"allowed_tools": append([]string(nil), opportunity.AllowedTools...),
			"may_pass":      opportunity.MayPass,
		},
		"record": map[string]any{
			"evidence":            rc.listVisibleEvidence(),
			"openings":            cloneJSONLikeMapList(mapList(caseObj["openings"])),
			"arguments":           cloneJSONLikeMapList(mapList(caseObj["arguments"])),
			"rebuttals":           cloneJSONLikeMapList(mapList(caseObj["rebuttals"])),
			"surrebuttals":        cloneJSONLikeMapList(mapList(caseObj["surrebuttals"])),
			"closings":            cloneJSONLikeMapList(mapList(caseObj["closings"])),
			"submitted_evidence":  cloneJSONLikeMapList(mapList(caseObj["submitted_evidence"])),
			"exhibits":            rc.attorneyExhibits(),
			"technical_reports":   cloneJSONLikeMapList(mapList(caseObj["technical_reports"])),
			"prior_council_votes": cloneJSONLikeMapList(mapList(caseObj["council_votes"])),
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
	return renderExhibitBodies(items, rc.fileByID)
}

func renderExhibitBodies(items []map[string]any, fileByID map[string]CaseFile) string {
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
		file, ok := fileByID[evidenceID]
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

func renderExhibitIndex(items []map[string]any, fileByID map[string]CaseFile) string {
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
		if file, ok := fileByID[evidenceID]; ok && strings.TrimSpace(file.Name) != "" {
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
