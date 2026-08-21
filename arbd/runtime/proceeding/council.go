package proceeding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
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
	inputItems := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": "Call submit_council_answer exactly once for this opportunity."},
	}
	tools := []map[string]any{
		{
			"type":        "function",
			"name":        "submit_council_answer",
			"description": "Submit one council answer for the current deliberation opportunity.",
			"parameters":  submitCouncilAnswerSchema(),
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
				inputItems = append(inputItems, map[string]any{
					"role":    "user",
					"content": "The previous tool call arguments were malformed. Call submit_council_answer exactly once with valid JSON arguments and keep the rationale brief.",
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
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": councilResponseOversizeCorrection(size, rc.cfg.Runtime.MaxResponseBytes),
			})
			continue
		}
		prevID = resp.ResponseID
		if len(resp.ToolCalls) != 1 {
			recordInvalidAttempt("Call submit_council_answer exactly once.")
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": "Call submit_council_answer exactly once.",
			})
			continue
		}
		call := resp.ToolCalls[0]
		if call.Name != "submit_council_answer" {
			recordInvalidAttempt("The only allowed tool is submit_council_answer.")
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": "The only allowed tool is submit_council_answer.",
			})
			continue
		}
		payload := cloneMap(call.Arguments)
		if err := requireAllowedKeys(payload, "submit_council_answer arguments", "answer", "rationale"); err != nil {
			reason := ensureTerminalPeriod(err.Error())
			recordInvalidAttempt(reason)
			inputItems = append(inputItems, map[string]any{
				"role":    "user",
				"content": reason,
			})
			continue
		}
		payload["member_id"] = memberID
		payload, err = normalizeCouncilAnswerPayload(payload)
		if err != nil {
			reason := ensureTerminalPeriod(err.Error())
			recordInvalidAttempt(reason)
			inputItems = append(inputItems, map[string]any{"role": "user", "content": reason})
			continue
		}
		rc.mu.Lock()
		stepResp, replayAction, err := rc.evaluateTurnStepLocked(caseCtx, turnDeadline, opportunity, "submit_council_answer", "council", payload)
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
		eventErr := rc.recordEventLocked("council_answer", "council", opportunity.Phase, eventPayload)
		rc.mu.Unlock()
		return eventErr
	}
	limitErr := formatInvalidAttemptLimitError(fmt.Sprintf("council member %s", memberID), invalidAttemptReasons)
	return rc.removeInvalidResponseCouncilMember(caseCtx, turnDeadline, opportunity, seat, limitErr)
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

func normalizeCouncilAnswerPayload(payload map[string]any) (map[string]any, error) {
	answer, err := normalizeCouncilAnswerValue(payload["answer"])
	if err != nil {
		return nil, err
	}
	rationale, ok := payload["rationale"].(string)
	rationale = strings.TrimSpace(rationale)
	if !ok || rationale == "" {
		return nil, fmt.Errorf("submit_council_answer requires a rationale")
	}
	payload["answer"] = answer
	payload["rationale"] = rationale
	return payload, nil
}

func normalizeCouncilAnswerValue(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return validateCouncilAnswerRange(v)
	case int64:
		return validateCouncilAnswerRange(int(v))
	case float64:
		if math.Trunc(v) != v {
			return 0, fmt.Errorf("submit_council_answer answer must be a whole number from 0 through 100")
		}
		return validateCouncilAnswerRange(int(v))
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, fmt.Errorf("submit_council_answer answer must be a whole number from 0 through 100")
		}
		return validateCouncilAnswerRange(n)
	default:
		return 0, fmt.Errorf("submit_council_answer answer must be a whole number from 0 through 100")
	}
}

func validateCouncilAnswerRange(answer int) (int, error) {
	if answer < 0 || answer > 100 {
		return 0, fmt.Errorf("submit_council_answer answer must be a whole number from 0 through 100")
	}
	return answer, nil
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

func (rc *runContext) buildCouncilPrompt(seat CouncilSeat, _ Opportunity) (string, error) {
	personaSection := ""
	if strings.TrimSpace(seat.PersonaText) != "" {
		personaSection = "Persona:\n" + strings.TrimSpace(seat.PersonaText) + "\n"
	}
	return rc.cfg.renderPromptFile("council.md", map[string]string{
		"MEMBER_ID":          seat.MemberID,
		"DELIBERATION_ROUND": fmt.Sprintf("%v", mapAny(rc.state["case"])["deliberation_round"]),
		"QUESTION":           rc.complaint.Question,
		"JUDGMENT_STANDARD":  currentJudgmentStandard(rc.state, rc.cfg.Policy),
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
	return fmt.Sprintf("Your response payload was %d bytes; the limit is %d bytes. Call submit_council_answer exactly once with only answer and a concise rationale. Do not include analysis outside the tool call.", size, limit)
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
	prior := rc.renderPriorAnswers(mapList(caseObj["council_answers"]), intNumber(caseObj["deliberation_round"]))
	if prior != "" {
		sections = append(sections, "Prior rounds:\n"+prior)
	}
	return strings.Join(sections, "\n\n")
}

func (rc *runContext) councilView(seat CouncilSeat, opportunity Opportunity) map[string]any {
	caseObj := mapAny(rc.state["case"])
	return map[string]any{
		"question":          rc.complaint.Question,
		"judgment_standard": currentJudgmentStandard(rc.state, rc.cfg.Policy),
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
			"evidence":              rc.listVisibleEvidence(),
			"openings":              cloneJSONLikeMapList(mapList(caseObj["openings"])),
			"arguments":             cloneJSONLikeMapList(mapList(caseObj["arguments"])),
			"rebuttals":             cloneJSONLikeMapList(mapList(caseObj["rebuttals"])),
			"surrebuttals":          cloneJSONLikeMapList(mapList(caseObj["surrebuttals"])),
			"closings":              cloneJSONLikeMapList(mapList(caseObj["closings"])),
			"submitted_evidence":    cloneJSONLikeMapList(mapList(caseObj["submitted_evidence"])),
			"exhibits":              rc.attorneyExhibits(),
			"technical_reports":     cloneJSONLikeMapList(mapList(caseObj["technical_reports"])),
			"prior_council_answers": cloneJSONLikeMapList(mapList(caseObj["council_answers"])),
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

func (rc *runContext) renderPriorAnswers(answers []map[string]any, currentRound int) string {
	if currentRound <= 1 {
		return ""
	}
	lines := make([]string, 0)
	for _, answer := range answers {
		round := intNumber(answer["round"])
		if round >= currentRound {
			continue
		}
		lines = append(lines, fmt.Sprintf("Round %d [%s] %d\n%s", round, mapString(answer["member_id"]), intNumber(answer["answer"]), mapString(answer["rationale"])))
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
