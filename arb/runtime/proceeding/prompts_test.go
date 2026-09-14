package proceeding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRenderPromptFileUsesCompletePromptDir(t *testing.T) {
	overrideDir := t.TempDir()
	writeCompletePromptDir(t, overrideDir)
	if err := os.WriteFile(filepath.Join(overrideDir, "attorney", "phase", "arguments.md"), []byte("override arguments"), 0o644); err != nil {
		t.Fatalf("overwrite arguments: %v", err)
	}

	cfg := Config{PromptDir: overrideDir}
	got, err := cfg.renderPromptFile(promptAttorneyArguments, nil)
	if err != nil {
		t.Fatalf("render arguments prompt: %v", err)
	}
	if got != "override arguments" {
		t.Fatalf("arguments prompt = %q, want override", got)
	}
	got, err = cfg.renderPromptFile(promptAttorneyCommon, nil)
	if err != nil {
		t.Fatalf("render common prompt: %v", err)
	}
	if got != promptAttorneyCommon {
		t.Fatalf("common prompt = %q, want prompt-dir source", got)
	}

	partialDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(partialDir, "attorney"), 0o755); err != nil {
		t.Fatalf("create partial prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(partialDir, "attorney", "wrapper.md"), []byte("partial"), 0o644); err != nil {
		t.Fatalf("write partial prompt dir: %v", err)
	}
	if _, err := (Config{PromptDir: partialDir}).renderPromptFile(promptAttorney, nil); err == nil {
		t.Fatal("incomplete prompt dir was accepted")
	}
}

func TestConfigRenderPromptFileUsesPerFileOverrideBeforePromptDir(t *testing.T) {
	overrideDir := t.TempDir()
	writeCompletePromptDir(t, overrideDir)
	specific := filepath.Join(t.TempDir(), "argument.md")
	if err := os.WriteFile(filepath.Join(overrideDir, "attorney", "phase", "arguments.md"), []byte("dir arguments"), 0o644); err != nil {
		t.Fatalf("write dir arguments: %v", err)
	}
	if err := os.WriteFile(specific, []byte("specific arguments"), 0o644); err != nil {
		t.Fatalf("write specific arguments: %v", err)
	}

	cfg := Config{PromptDir: overrideDir, PromptFiles: map[string]string{promptAttorneyArguments: specific}}
	got, err := cfg.renderPromptFile(promptAttorneyArguments, nil)
	if err != nil {
		t.Fatalf("render arguments prompt: %v", err)
	}
	if got != "specific arguments" {
		t.Fatalf("arguments prompt = %q, want specific override", got)
	}

	toolPath := filepath.Join(t.TempDir(), "list-evidence.md")
	if err := os.WriteFile(toolPath, []byte("Custom list-evidence description."), 0o644); err != nil {
		t.Fatalf("write tool prompt: %v", err)
	}
	notesPath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(notesPath, []byte("Custom notes property description."), 0o644); err != nil {
		t.Fatalf("write notes property prompt: %v", err)
	}
	prepared, err := (Config{PromptFiles: map[string]string{
		"tool.list_evidence":                  toolPath,
		"tool.send_work_notes.property.notes": notesPath,
	}}).preparePrompts()
	if err != nil {
		t.Fatalf("prepare partial tool prompt override: %v", err)
	}
	if got := prepared.modelToolDescription("List visible immutable record evidence."); got != "Custom list-evidence description." {
		t.Fatalf("list-evidence description = %q", got)
	}
	if got := prepared.modelToolDescription("A short update on the lawyer's findings, uncertainty, attempted approaches, and next steps."); got != "Custom notes property description." {
		t.Fatalf("notes property description = %q", got)
	}
	tools := prepared.lawyerToolSpecs(Opportunity{})
	for _, tool := range tools {
		if tool["name"] != "send_work_notes" {
			continue
		}
		schema := tool["input_schema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		notes := properties["notes"].(map[string]any)
		if got := notes["description"]; got != "Custom notes property description." {
			t.Fatalf("send_work_notes notes description = %#v", got)
		}
		return
	}
	t.Fatal("send_work_notes tool was not returned")
}

func writeCompletePromptDir(t *testing.T, dir string) {
	t.Helper()
	for _, definition := range promptDefinitions {
		path := filepath.Join(dir, definition.relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create directory for %s: %v", definition.relativePath, err)
		}
		if err := os.WriteFile(path, []byte(definition.id), 0o644); err != nil {
			t.Fatalf("write %s: %v", definition.relativePath, err)
		}
	}
}

func testPromptDir() string {
	return filepath.Join("..", "..", "..", "prompts", "arb")
}
