package courts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultCourtName              = "United States District"
	InternationalClawDistrictName = "International Claw District"
)

type Profile struct {
	Name                         string   `json:"name"`
	RulesMarkdown                string   `json:"rules_markdown,omitempty"`
	RulesMarkdownFile            string   `json:"rules_markdown_file,omitempty"`
	JurisdictionScreen           bool     `json:"jurisdiction_screen"`
	AllowedJurisdictionBases     []string `json:"allowed_jurisdiction_bases"`
	PreferredJurisdictionBasis   string   `json:"preferred_jurisdiction_basis,omitempty"`
	RequireJurisdictionStatement bool     `json:"require_jurisdiction_statement"`
	RequireDiversityCitizenship  bool     `json:"require_diversity_citizenship"`
	RequireAmountInControversy   bool     `json:"require_amount_in_controversy"`
	MinimumAmountInControversy   int      `json:"minimum_amount_in_controversy"`
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("court profile missing name")
	}
	if len(p.AllowedJurisdictionBases) == 0 {
		return fmt.Errorf("court profile %q missing allowed_jurisdiction_bases", p.Name)
	}
	seen := map[string]bool{}
	for _, raw := range p.AllowedJurisdictionBases {
		basis := normalizeBasis(raw)
		if basis == "" {
			return fmt.Errorf("court profile %q contains blank jurisdiction basis", p.Name)
		}
		if seen[basis] {
			return fmt.Errorf("court profile %q repeats jurisdiction basis %q", p.Name, basis)
		}
		seen[basis] = true
	}
	if p.MinimumAmountInControversy < 0 {
		return fmt.Errorf("court profile %q has negative minimum_amount_in_controversy", p.Name)
	}
	if p.PreferredJurisdictionBasis != "" && !containsBasis(p.AllowedJurisdictionBases, p.PreferredJurisdictionBasis) {
		return fmt.Errorf("court profile %q preferred_jurisdiction_basis %q is not allowed", p.Name, p.PreferredJurisdictionBasis)
	}
	if strings.TrimSpace(p.RulesMarkdown) == "" {
		return fmt.Errorf("court profile %q missing rules markdown", p.Name)
	}
	return nil
}

func (p Profile) AllowsJurisdictionBasis(basis string) bool {
	return containsBasis(p.AllowedJurisdictionBases, basis)
}

func Resolve(ref string) (Profile, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = DefaultCourtName
	}
	if builtinPath, ok := builtinCourtPath(ref); ok {
		return loadFile(resolveBuiltinCourtPath(builtinPath))
	}
	return loadFile(ref)
}

func builtinCourtPath(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case DefaultCourtName:
		return filepath.FromSlash("etc/courts/united-states-district.json"), true
	case InternationalClawDistrictName:
		return filepath.FromSlash("etc/courts/international-claw-district.json"), true
	default:
		return "", false
	}
}

func resolveBuiltinCourtPath(rel string) string {
	rel = filepath.FromSlash(rel)
	cwd, err := os.Getwd()
	if err != nil {
		return rel
	}
	for {
		for _, candidate := range []string{
			filepath.Join(cwd, rel),
			filepath.Join(cwd, "adc", rel),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return rel
		}
		cwd = parent
	}
}

func loadFile(path string) (Profile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Profile{}, fmt.Errorf("resolve court profile path: %w", err)
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return Profile{}, fmt.Errorf("read court profile %s: %w", path, err)
	}
	var profile Profile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse court profile %s: %w", path, err)
	}
	profile.Name = strings.TrimSpace(profile.Name)
	profile.PreferredJurisdictionBasis = normalizeBasis(profile.PreferredJurisdictionBasis)
	for i := range profile.AllowedJurisdictionBases {
		profile.AllowedJurisdictionBases[i] = normalizeBasis(profile.AllowedJurisdictionBases[i])
	}
	if strings.TrimSpace(profile.RulesMarkdown) == "" && strings.TrimSpace(profile.RulesMarkdownFile) != "" {
		rulesPath := strings.TrimSpace(profile.RulesMarkdownFile)
		if !filepath.IsAbs(rulesPath) {
			rulesPath = filepath.Join(filepath.Dir(absPath), rulesPath)
		}
		rulesRaw, err := os.ReadFile(rulesPath)
		if err != nil {
			return Profile{}, fmt.Errorf("read court rules %s: %w", rulesPath, err)
		}
		profile.RulesMarkdown = strings.TrimSpace(string(rulesRaw))
	}
	if err := profile.Validate(); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func containsBasis(bases []string, target string) bool {
	target = normalizeBasis(target)
	for _, basis := range bases {
		if normalizeBasis(basis) == target {
			return true
		}
	}
	return false
}

func normalizeBasis(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
