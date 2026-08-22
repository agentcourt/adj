package runner

import (
	"encoding/json"
	"strings"
	"testing"

	adcprompts "github.com/jsmorph/adj/adc/runtime/prompts"
	"github.com/jsmorph/adj/adc/runtime/spec"
)

func testPromptCatalog(t *testing.T) *adcprompts.Catalog {
	t.Helper()
	catalog, err := adcprompts.Load(adcprompts.Options{})
	if err != nil {
		t.Fatalf("load prompt catalog: %v", err)
	}
	return catalog
}

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

func TestMarshalStringReportsEncodingFailure(t *testing.T) {
	got := marshalString(map[string]any{"unsupported": make(chan int)})
	if !json.Valid([]byte(got)) || !strings.Contains(got, "encode JSON") {
		t.Fatalf("marshalString = %q", got)
	}
}
