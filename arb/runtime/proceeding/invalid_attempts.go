package proceeding

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	charLimitWithCountPattern = regexp.MustCompile(`^(.+) exceeds character limit of ([0-9]+) \(got ([0-9]+)\)$`)
	charLimitPattern          = regexp.MustCompile(`^(.+) exceeds character limit of ([0-9]+)$`)
)

func formatInvalidAttemptLimitError(subject string, reasons []string) error {
	subject = strings.TrimSpace(subject)
	attemptReasons := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		attemptReasons = append(attemptReasons, fmt.Sprintf("attempt %d: %s", len(attemptReasons)+1, reason))
	}
	if len(attemptReasons) == 0 {
		return fmt.Errorf("%s exceeded invalid-attempt limit", subject)
	}
	submissions := "submissions"
	if len(attemptReasons) == 1 {
		submissions = "submission"
	}
	return fmt.Errorf(
		"%s exceeded invalid-attempt limit after %d invalid %s: %s",
		subject,
		len(attemptReasons),
		submissions,
		strings.Join(attemptReasons, "; "),
	)
}

func formatAttorneyInvalidDecisionError(opportunity Opportunity, policy Policy, reasons []string, attemptLimit int) error {
	if len(reasons) == 0 {
		return fmt.Errorf("%s submission is invalid", strings.TrimSpace(opportunity.Role))
	}
	reasonText, corrective := attorneyInvalidReasonText(strings.TrimSpace(reasons[len(reasons)-1]), opportunity, policy)
	attempt := len(reasons)
	remaining := attemptLimit - attempt
	lines := []string{reasonText}
	if remaining > 0 {
		lines = append(lines, fmt.Sprintf(
			"This is invalid submission %d of %d for this opportunity. You have %d invalid %s remaining.",
			attempt,
			attemptLimit,
			remaining,
			invalidSubmissionWord(remaining),
		))
		if corrective != "" {
			lines = append(lines, corrective)
		}
		lines = append(lines, "If you exhaust the remaining invalid submissions, this opportunity will fail and the run will end with an error.")
		return fmt.Errorf("%s", strings.Join(lines, "\n"))
	}
	lines = append(lines, fmt.Sprintf(
		"This is invalid submission %d of %d for this opportunity. No invalid submissions remain.",
		attempt,
		attemptLimit,
	))
	lines = append(lines, "This opportunity has failed, and the run is ending with an error.")
	if history := formatAttorneyInvalidSubmissionHistory(reasons, opportunity, policy); history != "" {
		lines = append(lines, "Invalid submission history: "+history)
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func formatAttorneyInvalidSubmissionHistory(reasons []string, opportunity Opportunity, policy Policy) string {
	if len(reasons) <= 1 {
		return ""
	}
	attemptReasons := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reasonText, _ := attorneyInvalidReasonText(strings.TrimSpace(reason), opportunity, policy)
		if reasonText == "" {
			continue
		}
		attemptReasons = append(attemptReasons, fmt.Sprintf("attempt %d: %s", len(attemptReasons)+1, reasonText))
	}
	return strings.Join(attemptReasons, "; ")
}

func attorneyInvalidReasonText(reason string, opportunity Opportunity, policy Policy) (string, string) {
	reason = strings.TrimSpace(reason)
	if label, limit, got, ok := parseCharLimitReason(reason); ok {
		return fmt.Sprintf(
				"%s exceeds the character limit: %d characters submitted, %d allowed.",
				upperFirstASCII(label),
				got,
				limit,
			), fmt.Sprintf(
				"Resubmit at %d characters or fewer. Count characters, not tokens.",
				resubmissionCharLimit(limit),
			)
	}
	if label, limit, ok := parseCharLimitReasonWithoutCount(reason, opportunity, policy); ok {
		return fmt.Sprintf(
				"%s exceeds the character limit: %d allowed.",
				upperFirstASCII(label),
				limit,
			), fmt.Sprintf(
				"Resubmit at %d characters or fewer. Count characters, not tokens.",
				resubmissionCharLimit(limit),
			)
	}
	if strings.HasPrefix(reason, "offered_evidence for this side exceed limit of") ||
		strings.HasPrefix(reason, "technical_reports for this side exceed limit of") ||
		strings.HasPrefix(reason, "offered_evidence exceed per-filing limit of") ||
		strings.HasPrefix(reason, "technical_reports exceed per-filing limit of") {
		return ensureTerminalPeriod(reason), "Remove the overflow and resubmit within the stated limit."
	}
	if strings.Contains(reason, "offered_evidence must use visible case evidence_id values") {
		return ensureTerminalPeriod(reason), "Use only visible case evidence_id values in offered_evidence and resubmit."
	}
	if reason == "payload.text is required" {
		return "payload.text is required.", "Provide payload.text and resubmit."
	}
	if strings.Contains(reason, "entry requires") {
		return ensureTerminalPeriod(reason), "Add the missing required field and resubmit."
	}
	if strings.Contains(reason, "is not allowed in this opportunity") ||
		strings.Contains(reason, "passing is not allowed") ||
		reason == "submit_decision kind must be tool or pass" {
		return ensureTerminalPeriod(reason), "Use an allowed legal act for this opportunity and resubmit."
	}
	return ensureTerminalPeriod(reason), "Correct the submission and resubmit."
}

func parseCharLimitReason(reason string) (string, int, int, bool) {
	match := charLimitWithCountPattern.FindStringSubmatch(reason)
	if len(match) != 4 {
		return "", 0, 0, false
	}
	limit, err := strconv.Atoi(match[2])
	if err != nil {
		return "", 0, 0, false
	}
	got, err := strconv.Atoi(match[3])
	if err != nil {
		return "", 0, 0, false
	}
	return strings.TrimSpace(match[1]), limit, got, true
}

func parseCharLimitReasonWithoutCount(reason string, opportunity Opportunity, policy Policy) (string, int, bool) {
	match := charLimitPattern.FindStringSubmatch(reason)
	if len(match) == 3 {
		limit, err := strconv.Atoi(match[2])
		if err == nil {
			return strings.TrimSpace(match[1]), limit, true
		}
	}
	if !strings.Contains(reason, "exceeds character limit") {
		return "", 0, false
	}
	limit := phaseTextCharLimit(policy, opportunity.Phase)
	if limit <= 0 {
		return "", 0, false
	}
	return filingLabelForPhase(opportunity.Phase), limit, true
}

func resubmissionCharLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	return targetSubmissionCharLimit(limit)
}

func invalidSubmissionWord(count int) string {
	if count == 1 {
		return "submission"
	}
	return "submissions"
}

func upperFirstASCII(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func ensureTerminalPeriod(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	last := value[len(value)-1]
	if last == '.' || last == '!' || last == '?' {
		return value
	}
	return value + "."
}

func filingLabelForPhase(phase string) string {
	switch phase {
	case "openings":
		return "opening statement"
	case "arguments":
		return "argument"
	case "rebuttals":
		return "rebuttal"
	case "surrebuttals":
		return "surrebuttal"
	case "closings":
		return "closing statement"
	default:
		return "submission"
	}
}
