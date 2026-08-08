package runner

import (
	"testing"

	"github.com/jsmorph/adj/adc/runtime/courts"
)

func TestRule12ToolSchemaOmitsJurisdictionGroundInInternationalClaw(t *testing.T) {
	t.Parallel()

	r := &Runner{
		courtProfile: courts.Profile{
			Name:                       courts.InternationalClawDistrictName,
			RulesMarkdown:              "No jurisdiction screen.",
			AllowedJurisdictionBases:   []string{"general_civil"},
			PreferredJurisdictionBasis: "general_civil",
		},
	}
	schema := r.toolSchema("file_rule12_motion")
	properties, _ := schema["properties"].(map[string]any)
	ground, _ := properties["ground"].(map[string]any)
	enumVals, _ := ground["enum"].([]any)
	for _, item := range enumVals {
		if item == "lack_subject_matter_jurisdiction" {
			t.Fatalf("unexpected jurisdiction ground in schema: %v", enumVals)
		}
	}
}

func TestRule12ToolSchemaIncludesJurisdictionGroundInUSDistrict(t *testing.T) {
	t.Parallel()

	r := &Runner{
		courtProfile: courts.Profile{
			Name:                     courts.DefaultCourtName,
			RulesMarkdown:            "Federal jurisdiction screen applies.",
			JurisdictionScreen:       true,
			AllowedJurisdictionBases: []string{"federal_question", "diversity", "unspecified"},
		},
	}
	schema := r.toolSchema("file_rule12_motion")
	properties, _ := schema["properties"].(map[string]any)
	ground, _ := properties["ground"].(map[string]any)
	enumVals, _ := ground["enum"].([]any)
	found := false
	for _, item := range enumVals {
		if item == "lack_subject_matter_jurisdiction" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing jurisdiction ground in schema: %v", enumVals)
	}
}
