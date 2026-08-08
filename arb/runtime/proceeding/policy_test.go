package proceeding

import (
	"testing"
	"time"
)

func TestCouncilRequestTimeoutIsShorterThanTurnTimeout(t *testing.T) {
	limits := DefaultRuntimeLimits()
	if got, want := limits.CouncilTimeout(), 240*time.Second; got != want {
		t.Fatalf("CouncilTimeout = %s, want %s", got, want)
	}
	if got, want := limits.CouncilRequestTimeout(), 90*time.Second; got != want {
		t.Fatalf("CouncilRequestTimeout = %s, want %s", got, want)
	}
	limits.CouncilLLMTimeoutSeconds = 60
	if got, want := limits.CouncilRequestTimeout(), 60*time.Second; got != want {
		t.Fatalf("CouncilRequestTimeout with 60-second budget = %s, want %s", got, want)
	}
}
