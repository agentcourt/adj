package prompts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCatalogResolutionAndValidation(t *testing.T) {
	_, sourcePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller did not return the test source path")
	}
	promptDir := filepath.Clean(filepath.Join(filepath.Dir(sourcePath), "..", "..", "..", ConventionalDir))
	catalog, err := Load(Options{PromptDir: promptDir})
	if err != nil {
		t.Fatalf("load complete conventional catalog: %v", err)
	}
	if got, want := len(Entries()), 211; got != want {
		t.Fatalf("catalog entry count = %d, want %d", got, want)
	}
	seenPaths := map[string]string{}
	for _, entry := range Entries() {
		if prior := seenPaths[entry.RelativePath]; prior != "" {
			t.Fatalf("prompts %s and %s share relative path %s", prior, entry.ID, entry.RelativePath)
		}
		seenPaths[entry.RelativePath] = entry.ID
		if _, err := os.Stat(filepath.Join(promptDir, entry.RelativePath)); err != nil {
			t.Fatalf("prompt %s file: %v", entry.ID, err)
		}
	}

	overridePath := filepath.Join(t.TempDir(), "repair.md")
	if err := os.WriteFile(overridePath, []byte("Repair this error: {{ERROR}}"), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}
	catalog, err = Load(Options{PromptDir: promptDir, PromptFiles: map[string]string{ComplaintRepairID: overridePath}})
	if err != nil {
		t.Fatalf("load partial override: %v", err)
	}
	rendered, err := catalog.Render(ComplaintRepairID, map[string]string{"{{ERROR}}": "bad {{RUNTIME_VALUE}}"})
	if err != nil {
		t.Fatalf("render runtime braces: %v", err)
	}
	if !strings.Contains(rendered, "bad {{RUNTIME_VALUE}}") {
		t.Fatalf("rendered prompt = %q", rendered)
	}

	invalidCases := []struct {
		name  string
		files map[string]string
		body  string
		want  string
	}{
		{name: "unknown id", files: map[string]string{"unknown": overridePath}, want: "unknown prompt ID"},
		{name: "unreadable", files: map[string]string{ComplaintRepairID: filepath.Join(t.TempDir(), "missing.md")}, want: "read complaint.repair prompt"},
		{name: "empty", files: map[string]string{ComplaintRepairID: overridePath}, body: " \n", want: "prompt is empty"},
		{name: "unknown token", files: map[string]string{ComplaintRepairID: overridePath}, body: "{{UNDECLARED}}", want: "unresolved token"},
	}
	for _, test := range invalidCases {
		t.Run(test.name, func(t *testing.T) {
			if test.body != "" {
				if err := os.WriteFile(overridePath, []byte(test.body), 0o644); err != nil {
					t.Fatalf("write invalid override: %v", err)
				}
			}
			_, err := Load(Options{PromptDir: promptDir, PromptFiles: test.files})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load error = %v, want %q", err, test.want)
			}
		})
	}
}
