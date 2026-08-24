package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
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
	schema, err := r.toolSchema("file_rule12_motion")
	if err != nil {
		t.Fatalf("toolSchema error = %v", err)
	}
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
	schema, err := r.toolSchema("file_rule12_motion")
	if err != nil {
		t.Fatalf("toolSchema error = %v", err)
	}
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

func TestToolSchemaReportsPromptCatalogError(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompts", "adc", "runtime", "system.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(promptPath, []byte(" \n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	t.Chdir(dir)

	_, err := (&Runner{}).toolSchema("file_rule12_motion")
	if err == nil || !strings.Contains(err.Error(), "prompt is empty") {
		t.Fatalf("toolSchema error = %v", err)
	}
}
