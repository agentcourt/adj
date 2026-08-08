package proceeding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRenderPromptFileUsesPromptDirAndFallsBackToDefault(t *testing.T) {
	defaultDir := t.TempDir()
	overrideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(defaultDir, "attorney-common.md"), []byte("default common"), 0o644); err != nil {
		t.Fatalf("write default common: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "attorney-arguments.md"), []byte("default arguments"), 0o644); err != nil {
		t.Fatalf("write default arguments: %v", err)
	}
	if err := os.WriteFile(filepath.Join(overrideDir, "attorney-arguments.md"), []byte("override arguments"), 0o644); err != nil {
		t.Fatalf("write override arguments: %v", err)
	}

	origPromptBaseDir := promptBaseDir
	promptBaseDir = defaultDir
	defer func() { promptBaseDir = origPromptBaseDir }()

	cfg := Config{PromptDir: overrideDir}
	got, err := cfg.renderPromptFile("attorney-arguments.md", nil)
	if err != nil {
		t.Fatalf("render arguments prompt: %v", err)
	}
	if got != "override arguments" {
		t.Fatalf("arguments prompt = %q, want override", got)
	}
	got, err = cfg.renderPromptFile("attorney-common.md", nil)
	if err != nil {
		t.Fatalf("render common prompt: %v", err)
	}
	if got != "default common" {
		t.Fatalf("common prompt = %q, want default fallback", got)
	}
}

func TestConfigRenderPromptFileUsesPerFileOverrideBeforePromptDir(t *testing.T) {
	defaultDir := t.TempDir()
	overrideDir := t.TempDir()
	specific := filepath.Join(t.TempDir(), "argument.md")
	if err := os.WriteFile(filepath.Join(defaultDir, "attorney-arguments.md"), []byte("default arguments"), 0o644); err != nil {
		t.Fatalf("write default arguments: %v", err)
	}
	if err := os.WriteFile(filepath.Join(overrideDir, "attorney-arguments.md"), []byte("dir arguments"), 0o644); err != nil {
		t.Fatalf("write dir arguments: %v", err)
	}
	if err := os.WriteFile(specific, []byte("specific arguments"), 0o644); err != nil {
		t.Fatalf("write specific arguments: %v", err)
	}

	origPromptBaseDir := promptBaseDir
	promptBaseDir = defaultDir
	defer func() { promptBaseDir = origPromptBaseDir }()

	cfg := Config{PromptDir: overrideDir, AttorneyArgumentPromptPath: specific}
	got, err := cfg.renderPromptFile("attorney-arguments.md", nil)
	if err != nil {
		t.Fatalf("render arguments prompt: %v", err)
	}
	if got != "specific arguments" {
		t.Fatalf("arguments prompt = %q, want specific override", got)
	}
}
