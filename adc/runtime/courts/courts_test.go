package courts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeCourtProfile(t *testing.T, profile map[string]any) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "court.json")
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	return path
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestProfileValidateRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{name: "missing name", profile: Profile{RulesMarkdown: "Rules", AllowedJurisdictionBases: []string{"diversity"}}, want: "missing name"},
		{name: "missing bases", profile: Profile{Name: "Court", RulesMarkdown: "Rules"}, want: "missing allowed_jurisdiction_bases"},
		{name: "duplicate basis", profile: Profile{Name: "Court", RulesMarkdown: "Rules", AllowedJurisdictionBases: []string{" diversity ", "diversity"}}, want: "repeats jurisdiction basis"},
		{name: "negative amount", profile: Profile{Name: "Court", RulesMarkdown: "Rules", AllowedJurisdictionBases: []string{"diversity"}, MinimumAmountInControversy: -1}, want: "negative minimum_amount_in_controversy"},
		{name: "bad preferred basis", profile: Profile{Name: "Court", RulesMarkdown: "Rules", AllowedJurisdictionBases: []string{"diversity"}, PreferredJurisdictionBasis: "federal_question"}, want: "preferred_jurisdiction_basis"},
		{name: "missing rules", profile: Profile{Name: "Court", AllowedJurisdictionBases: []string{"diversity"}}, want: "missing rules markdown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.profile.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestResolveLoadsRulesFileAndNormalizesBases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.md")
	if err := os.WriteFile(rulesPath, []byte(" Local rules apply. \n"), 0o644); err != nil {
		t.Fatalf("WriteFile rules error = %v", err)
	}
	profilePath := writeCourtProfile(t, map[string]any{
		"name":                           "Example Court",
		"rules_markdown_file":            "rules.md",
		"jurisdiction_screen":            false,
		"allowed_jurisdiction_bases":     []string{" General_Civil ", " Diversity "},
		"preferred_jurisdiction_basis":   " Diversity ",
		"require_jurisdiction_statement": true,
	})
	if err := os.Rename(profilePath, filepath.Join(dir, "court.json")); err != nil {
		t.Fatalf("Rename error = %v", err)
	}

	got, err := Resolve(filepath.Join(dir, "court.json"))
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if got.RulesMarkdown != "Local rules apply." {
		t.Fatalf("RulesMarkdown = %q", got.RulesMarkdown)
	}
	if !got.AllowsJurisdictionBasis("general_civil") || !got.AllowsJurisdictionBasis("DIVERSITY") {
		t.Fatalf("AllowsJurisdictionBasis failed for normalized bases: %+v", got.AllowedJurisdictionBases)
	}
	if got.PreferredJurisdictionBasis != "diversity" {
		t.Fatalf("PreferredJurisdictionBasis = %q", got.PreferredJurisdictionBasis)
	}
}

func TestResolveBuiltInProfiles(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, name := range []string{DefaultCourtName, InternationalClawDistrictName} {
		rel, ok := builtinCourtPath(name)
		if !ok {
			t.Fatalf("builtinCourtPath(%q) not found", name)
		}
		profile, err := loadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("loadFile(%q) error = %v", name, err)
		}
		if profile.Name != name {
			t.Fatalf("profile.Name = %q, want %q", profile.Name, name)
		}
		if strings.TrimSpace(profile.RulesMarkdown) == "" {
			t.Fatalf("profile %q missing RulesMarkdown", name)
		}
		if len(profile.AllowedJurisdictionBases) == 0 {
			t.Fatalf("profile %q missing allowed bases", name)
		}
	}
}

func TestResolveDefaultBuiltInFromRepoRoot(t *testing.T) {
	t.Chdir(repoRoot(t))

	profile, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\") error = %v", err)
	}
	if profile.Name != DefaultCourtName {
		t.Fatalf("Resolve(\"\") name = %q, want %q", profile.Name, DefaultCourtName)
	}
}
