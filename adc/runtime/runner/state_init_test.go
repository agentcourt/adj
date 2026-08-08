package runner

import "testing"

func TestBuildInitialPolicyNormalizesJuryConfiguration(t *testing.T) {
	policy := buildInitialPolicy(map[string]any{
		"jury_juror_count":        8,
		"jury_unanimous_required": false,
		"jury_minimum_concurring": 6,
	})

	if got := policy["jury_juror_count"]; got != 8 {
		t.Fatalf("jury_juror_count = %#v, want 8", got)
	}
	if got := policy["jury_unanimous_required"]; got != 0 {
		t.Fatalf("jury_unanimous_required = %#v, want 0", got)
	}
	if got := policy["jury_minimum_concurring"]; got != 6 {
		t.Fatalf("jury_minimum_concurring = %#v, want 6", got)
	}
}
