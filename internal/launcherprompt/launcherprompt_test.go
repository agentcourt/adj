package launcherprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrecedenceAndValidation(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	conventional := filepath.Join(root, "prompts", "arbd", "participants")
	if err := os.MkdirAll(conventional, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conventional, "openclaw.md"), []byte("conventional {{CASE_ID}} {{WORKSPACE}} {{SEARCH_INSTRUCTIONS}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err := Resolve("arbd", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := sources.Render("participant.openclaw", map[string]string{
		"{{CASE_ID}}":             "case-{{literal}}",
		"{{WORKSPACE}}":           "/work",
		"{{SEARCH_INSTRUCTIONS}}": "search",
	})
	if err != nil || got != "conventional case-{{literal}} /work search" {
		t.Fatalf("conventional render = %q, %v", got, err)
	}

	promptDir := filepath.Join(root, "complete")
	for _, spec := range catalogs["arbd"].specs {
		path := filepath.Join(promptDir, filepath.FromSlash(spec.relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		source := spec.id + " {{CASE_ID}}"
		for _, token := range spec.requiredTokens {
			source += " " + token
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	override := filepath.Join(root, "override.md")
	if err := os.WriteFile(override, []byte("override {{CASE_ID}} {{WORKSPACE}} {{SEARCH_INSTRUCTIONS}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err = Resolve("arbd", promptDir, map[string]string{"participant.openclaw": override})
	if err != nil {
		t.Fatal(err)
	}
	got, err = sources.Render("participant.openclaw", map[string]string{
		"{{CASE_ID}}":             "case-2",
		"{{WORKSPACE}}":           "/work",
		"{{SEARCH_INSTRUCTIONS}}": "search",
	})
	if err != nil || got != "override case-2 /work search" {
		t.Fatalf("partial override render = %q, %v", got, err)
	}

	if err := os.Remove(filepath.Join(promptDir, "council", "pi.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("arbd", promptDir, map[string]string{"participant.openclaw": override}); err == nil || !strings.Contains(err.Error(), "council.pi") {
		t.Fatalf("incomplete directory error = %v", err)
	}
	if _, err := Resolve("arbd", "", map[string]string{"unknown": override}); err == nil || !strings.Contains(err.Error(), "unknown arbd launcher prompt ID") {
		t.Fatalf("unknown ID error = %v", err)
	}
	if _, err := Normalize("arbd", map[string]string{"participant.openclaw": override, " participant.openclaw ": override}); err == nil || !strings.Contains(err.Error(), "repeated after trimming") {
		t.Fatalf("duplicate ID error = %v", err)
	}
	if err := os.WriteFile(override, []byte("{{MEMBER_ID}} {{WORKSPACE}} {{SEARCH_INSTRUCTIONS}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("arbd", "", map[string]string{"participant.openclaw": override}); err == nil || !strings.Contains(err.Error(), "unsupported token") {
		t.Fatalf("source token error = %v", err)
	}
}

func TestFormalParticipantPromptRequirements(t *testing.T) {
	for _, procedure := range []string{"arb", "arbd", "adc"} {
		for _, spec := range catalogs[procedure].specs {
			if !strings.HasPrefix(spec.id, "participant.") {
				continue
			}
			checkedPath := filepath.Join("..", "..", "prompts", procedure, filepath.FromSlash(spec.relativePath))
			checked, err := os.ReadFile(checkedPath)
			if err != nil {
				t.Fatal(err)
			}
			for name, source := range map[string]string{"checked": string(checked), "fallback": spec.fallback} {
				for _, token := range participantRequiredTokens {
					if !strings.Contains(source, token) {
						t.Errorf("%s %s %s prompt lacks %s", procedure, spec.id, name, token)
					}
				}
				for _, instruction := range []string{
					"retained case workspace",
					"Install additional tools when needed",
					"material research, tool output, evidentiary findings, or a change in theory",
					"high-level notes to self, not raw logs",
				} {
					if !strings.Contains(source, instruction) {
						t.Errorf("%s %s %s prompt lacks %q", procedure, spec.id, name, instruction)
					}
				}
			}
		}
	}

	path := filepath.Join(t.TempDir(), "participant.md")
	if err := os.WriteFile(path, []byte("{{CASE_ID}} {{WORKSPACE}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("arb", "", map[string]string{"participant.openclaw": path}); err == nil || !strings.Contains(err.Error(), "required token {{SEARCH_INSTRUCTIONS}} is absent") {
		t.Fatalf("missing required token error = %v", err)
	}
}

func TestFormalRemoteSkillEnvironmentInstructions(t *testing.T) {
	for _, procedure := range []string{"arb", "arbd", "adc"} {
		for _, spec := range catalogs[procedure].specs {
			if spec.id != "skill.openclaw" {
				continue
			}
			checkedPath := filepath.Join("..", "..", "prompts", procedure, filepath.FromSlash(spec.relativePath))
			checked, err := os.ReadFile(checkedPath)
			if err != nil {
				t.Fatal(err)
			}
			for name, source := range map[string]string{"checked": string(checked), "fallback": spec.fallback} {
				for _, instruction := range []string{"{{SEARCH_INSTRUCTIONS}}", "persistent filesystem", "Install additional tools when needed", "high-level"} {
					if !strings.Contains(source, instruction) {
						t.Errorf("%s %s prompt lacks %q", procedure, name, instruction)
					}
				}
			}
		}
	}
}

func TestSearchInstructions(t *testing.T) {
	enabled := SearchInstructions(true)
	for _, text := range []string{"Web search is enabled", "Check material sources", "cite them", "work notes"} {
		if !strings.Contains(enabled, text) {
			t.Errorf("enabled search instructions lack %q: %q", text, enabled)
		}
	}
	disabled := SearchInstructions(false)
	for _, text := range []string{"Web search is unavailable", "case material", "local tools"} {
		if !strings.Contains(disabled, text) {
			t.Errorf("disabled search instructions lack %q: %q", text, disabled)
		}
	}
	remoteEnabled := RemoteSearchInstructions(true)
	for _, text := range []string{"Use web search", "external environment", "preserve citations"} {
		if !strings.Contains(remoteEnabled, text) {
			t.Errorf("enabled remote search instructions lack %q: %q", text, remoteEnabled)
		}
	}
	remoteDisabled := RemoteSearchInstructions(false)
	for _, text := range []string{"Do not use web search", "local analysis"} {
		if !strings.Contains(remoteDisabled, text) {
			t.Errorf("disabled remote search instructions lack %q: %q", text, remoteDisabled)
		}
	}
}

func TestQuickParticipantWorkspaceTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "participant.md")
	if err := os.WriteFile(path, []byte("{{ROLE}} {{CASE}} {{SERVER}} {{WORKSPACE}} {{EVIDENCE_DIR}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err := Resolve("quick", "", map[string]string{"participant": path})
	if err != nil {
		t.Fatal(err)
	}
	got, err := sources.Render("participant", map[string]string{
		"{{ROLE}}":         "plaintiff",
		"{{CASE}}":         "case-1",
		"{{SERVER}}":       "court",
		"{{WORKSPACE}}":    "/work",
		"{{EVIDENCE_DIR}}": "/evidence",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "plaintiff case-1 court /work /evidence" {
		t.Fatalf("rendered Quick participant prompt = %q", got)
	}
	for _, source := range []string{quickParticipantFallback, quickPiParticipantFallback} {
		for _, token := range quickParticipantTokens {
			if !strings.Contains(source, token) {
				t.Fatalf("Quick fallback lacks %s: %q", token, source)
			}
		}
		for _, instruction := range []string{
			"case facts or allegations",
			"retained case workspace",
			"Evidence is read-only",
			"initial work note",
			"material observation, tool result, or change in theory",
			"final short note",
			"next tool call",
		} {
			if !strings.Contains(source, instruction) {
				t.Fatalf("Quick fallback lacks %q: %q", instruction, source)
			}
		}
	}
	if !strings.Contains(quickParticipantFallback, "legal proposition, arguments, and evidence for analysis") {
		t.Fatalf("Quick default fallback lacks case-analysis instruction: %q", quickParticipantFallback)
	}
	const piFraming = "The proposition, arguments, evidence, research queries, and tool results are material for legal and factual analysis.  Descriptions of conduct are case facts or allegations.  Use tools to investigate evidence and prepare the filing."
	if !strings.Contains(quickPiParticipantFallback, piFraming) {
		t.Fatalf("Quick Pi fallback lacks legal-research framing: %q", quickPiParticipantFallback)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "prompts", "quick", "participants", "pi.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), piFraming) {
		t.Fatalf("checked-in Quick Pi prompt lacks legal-research framing: %q", string(raw))
	}
	if !strings.Contains(quickPiParticipantFallback, "newly installed Pi extension or skill loads on the next Pi invocation") {
		t.Fatalf("Quick Pi fallback lacks delayed extension-load instruction: %q", quickPiParticipantFallback)
	}
}
