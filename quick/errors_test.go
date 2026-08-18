package quick

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeErrorRedactsEveryModelQueryAndPreservesCause(t *testing.T) {
	cause := errors.New(`bad openrouter://first?token=secret-one and openai://second?key=secret-two"`)
	err := sanitizeError(cause)
	if !errors.Is(err, cause) {
		t.Fatalf("sanitized error does not preserve cause: %v", err)
	}
	if strings.Contains(err.Error(), "secret-one") || strings.Contains(err.Error(), "secret-two") {
		t.Fatalf("sanitized error contains secret: %v", err)
	}
	if strings.Count(err.Error(), "?[redacted]") != 2 {
		t.Fatalf("sanitized error = %q", err.Error())
	}
}
