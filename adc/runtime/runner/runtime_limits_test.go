package runner

import "testing"

func TestRuntimeLimitsNormalized(t *testing.T) {
	t.Parallel()

	got := (RuntimeLimits{}).Normalized()
	if got.LLMTimeoutSeconds != DefaultLLMTimeoutSeconds {
		t.Fatalf("LLMTimeoutSeconds = %d, want %d", got.LLMTimeoutSeconds, DefaultLLMTimeoutSeconds)
	}
	if got.RoleAPITimeoutSeconds != DefaultRoleAPITimeoutSeconds {
		t.Fatalf("RoleAPITimeoutSeconds = %d, want %d", got.RoleAPITimeoutSeconds, DefaultRoleAPITimeoutSeconds)
	}
	if got.MaxResponseBytes != DefaultMaxResponseBytes {
		t.Fatalf("MaxResponseBytes = %d, want %d", got.MaxResponseBytes, DefaultMaxResponseBytes)
	}
	if got.InvalidAttemptLimit != DefaultInvalidAttemptLimit {
		t.Fatalf("InvalidAttemptLimit = %d, want %d", got.InvalidAttemptLimit, DefaultInvalidAttemptLimit)
	}
	if got.JurorMaxOutputTokens != DefaultJurorMaxOutputTokens {
		t.Fatalf("JurorMaxOutputTokens = %d, want %d", got.JurorMaxOutputTokens, DefaultJurorMaxOutputTokens)
	}
}
