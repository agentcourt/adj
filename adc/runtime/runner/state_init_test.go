package runner

import (
	"testing"

	"github.com/jsmorph/adj/adc/runtime/courts"
	"github.com/jsmorph/adj/adc/runtime/spec"
)

func TestBuildSingleClaimDefaultsToCivilDamages(t *testing.T) {
	claim := buildSingleClaim(nil)
	if declaratoryOnly, ok := claim["declaratory_only"].(bool); !ok || declaratoryOnly {
		t.Fatalf("declaratory_only = %#v, want false", claim["declaratory_only"])
	}
}

func TestBuildInitialStateStartsWithPendingResolution(t *testing.T) {
	t.Parallel()

	state := buildInitialState(spec.FormalScenario{}, courts.PropositionTribunal())
	caseState, ok := state["case"].(map[string]any)
	if !ok {
		t.Fatalf("case state = %#v", state["case"])
	}
	if got := caseState["resolution"]; got != "pending" {
		t.Fatalf("resolution = %#v, want pending", got)
	}
}

func TestBuildSingleClaimPreservesDeclaratoryOnly(t *testing.T) {
	claim := buildSingleClaim([]spec.ClaimSpec{{DeclaratoryOnly: true}})
	if declaratoryOnly, ok := claim["declaratory_only"].(bool); !ok || !declaratoryOnly {
		t.Fatalf("declaratory_only = %#v, want true", claim["declaratory_only"])
	}
}

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
