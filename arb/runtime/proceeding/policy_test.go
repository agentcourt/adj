package proceeding

import (
	"testing"
	"time"
)

func TestCouncilRequestTimeoutIsShorterThanTurnTimeout(t *testing.T) {
	limits := DefaultRuntimeLimits()
	if got, want := limits.CouncilRequestAttempts, 4; got != want {
		t.Fatalf("CouncilRequestAttempts = %d, want %d", got, want)
	}
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

func TestRuntimeTimeouts(t *testing.T) {
	limits := DefaultRuntimeLimits()
	if got, want := limits.EngineCallTimeout(), 30*time.Second; got != want {
		t.Fatalf("EngineCallTimeout = %s, want %s", got, want)
	}
	maxSeconds := int64(MaxRuntimeTimeoutSeconds)
	if int64(int(maxSeconds)) != maxSeconds {
		t.Skip("int cannot represent the maximum runtime timeout")
	}
	timeouts := []struct {
		name string
		set  func(*RuntimeLimits, int)
	}{
		{name: "council", set: func(limits *RuntimeLimits, seconds int) {
			limits.CouncilLLMTimeoutSeconds = seconds
		}},
		{name: "lawyer", set: func(limits *RuntimeLimits, seconds int) {
			limits.LawyerTurnTimeoutSeconds = seconds
		}},
		{name: "engine", set: func(limits *RuntimeLimits, seconds int) {
			limits.EngineCallTimeoutSeconds = seconds
		}},
	}
	for _, timeout := range timeouts {
		t.Run(timeout.name, func(t *testing.T) {
			zero := DefaultRuntimeLimits()
			timeout.set(&zero, 0)
			if err := ValidateRuntimeLimits(zero); err == nil {
				t.Fatal("ValidateRuntimeLimits accepted zero timeout")
			}

			maximum := DefaultRuntimeLimits()
			timeout.set(&maximum, int(maxSeconds))
			if err := ValidateRuntimeLimits(maximum); err != nil {
				t.Fatalf("ValidateRuntimeLimits rejected maximum timeout: %v", err)
			}

			tooLarge := MaxRuntimeTimeoutSeconds + 1
			if int64(int(tooLarge)) != tooLarge {
				t.Skip("int cannot represent an overflowing runtime timeout")
			}
			overflow := DefaultRuntimeLimits()
			timeout.set(&overflow, int(tooLarge))
			if err := ValidateRuntimeLimits(overflow); err == nil {
				t.Fatal("ValidateRuntimeLimits accepted overflowing timeout")
			}
		})
	}
	limits.EngineCallTimeoutSeconds = int(maxSeconds)
	if got, want := limits.EngineCallTimeout(), time.Duration(MaxRuntimeTimeoutSeconds)*time.Second; got != want || got <= 0 {
		t.Fatalf("EngineCallTimeout at maximum = %s, want %s", got, want)
	}
}
