package runner

import (
	"testing"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func TestEffectiveRoleTemperatureUsesJurorOverrideOnlyForJurors(t *testing.T) {
	t.Parallel()

	general := 0.2
	juror := 0.8
	r := &Runner{
		cfg: Config{
			Temperature:      &general,
			JurorTemperature: &juror,
		},
	}

	gotJuror := r.effectiveRoleTemperature(spec.RoleSpec{Name: "juror"})
	if gotJuror == nil || *gotJuror != juror {
		t.Fatalf("juror temperature = %v, want %v", gotJuror, juror)
	}

	gotJudge := r.effectiveRoleTemperature(spec.RoleSpec{Name: "judge"})
	if gotJudge == nil || *gotJudge != general {
		t.Fatalf("judge temperature = %v, want %v", gotJudge, general)
	}
}
