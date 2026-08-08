package runner

import (
	"fmt"
	"strings"
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
