package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/common/persona"
)

func TestRunJurorHelpIncludesPromptOptions(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunJuror(context.Background(), []string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunJuror: %v", err)
	}
	for _, option := range []string{"-input-file", "-prompt-dir", "-prompt-file"} {
		if !strings.Contains(stderr.String(), option) {
			t.Errorf("help omits %q:\n%s", option, stderr.String())
		}
	}
}

func TestRunJurorRejectsUnknownPromptID(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunJuror(context.Background(), []string{"--prompt-file", "unknown=prompt.md"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), `unknown ADC prompt id "unknown"`) {
		t.Fatalf("RunJuror error = %v", err)
	}
}

func writeJurorTestPool(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	personaDir := filepath.Join(root, "persons")
	if err := os.MkdirAll(personaDir, 0o755); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(personaDir, "j1.txt"), []byte("skeptical of screenshots"), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	if err := os.WriteFile(filepath.Join(personaDir, "j2.txt"), []byte("insists on corroboration"), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	poolPath := filepath.Join(root, "pool.jsonl")
	records := []string{
		`{"openrouter_model_id":"openai/gpt-5","provider":{"only":["openai"]},"persona":"persons/j1.txt"}`,
		`{"openrouter_model_id":"deepseek/deepseek-r1","provider":{"only":["novita"],"quantizations":["fp8"]},"persona":"persons/j2.txt"}`,
		`{"openrouter_model_id":"deepseek/deepseek-r1","provider":{"only":["siliconflow"]},"persona":"persons/j2.txt"}`,
	}
	if err := os.WriteFile(poolPath, []byte(strings.Join(records, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write pool: %v", err)
	}
	return poolPath
}

func TestSelectJurorMember(t *testing.T) {
	poolPath := writeJurorTestPool(t)
	specs, err := persona.LoadRecordsFile(poolPath, "")
	if err != nil {
		t.Fatalf("load pool: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("loaded %d specs, want 3", len(specs))
	}

	byIndex, err := selectJurorMember(specs, "2")
	if err != nil {
		t.Fatalf("select by index: %v", err)
	}
	if byIndex.File != "persons/j2.txt" {
		t.Fatalf("index selection returned %q", byIndex.File)
	}

	bySubstring, err := selectJurorMember(specs, "gpt-5")
	if err != nil {
		t.Fatalf("select by substring: %v", err)
	}
	if bySubstring.File != "persons/j1.txt" {
		t.Fatalf("substring selection returned %q", bySubstring.File)
	}

	byProvider, err := selectJurorMember(specs, "novita")
	if err != nil {
		t.Fatalf("select by provider: %v", err)
	}
	if !strings.Contains(jurorMemberLabel(byProvider), "quant=fp8") {
		t.Fatalf("provider selection returned %q", jurorMemberLabel(byProvider))
	}

	if _, err := selectJurorMember(specs, "deepseek"); err == nil {
		t.Fatalf("ambiguous selector did not error")
	}
	if _, err := selectJurorMember(specs, "no-such-model"); err == nil {
		t.Fatalf("unmatched selector did not error")
	}
	if _, err := selectJurorMember(specs, "4"); err == nil {
		t.Fatalf("out-of-range index did not error")
	}
	if _, err := selectJurorMember(specs, ""); err == nil {
		t.Fatalf("empty selector did not error")
	}
}

func TestJurorTranscriptRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.ndjson")

	entries, err := readJurorTranscript(path, "member-a")
	if err != nil {
		t.Fatalf("read missing transcript: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing transcript returned %d entries", len(entries))
	}

	first := []jurorTranscriptEntry{
		{Time: "2026-07-14T00:00:00Z", Member: "member-a", Role: "user", Content: "question one"},
		{Time: "2026-07-14T00:00:00Z", Member: "member-a", Role: "assistant", Content: "answer one"},
	}
	if err := appendJurorTranscript(path, first); err != nil {
		t.Fatalf("append transcript: %v", err)
	}
	second := []jurorTranscriptEntry{
		{Time: "2026-07-14T00:01:00Z", Member: "member-a", Role: "user", Content: "question two"},
		{Time: "2026-07-14T00:01:00Z", Member: "member-a", Role: "assistant", Content: "answer two"},
	}
	if err := appendJurorTranscript(path, second); err != nil {
		t.Fatalf("append transcript again: %v", err)
	}

	entries, err = readJurorTranscript(path, "member-a")
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("read %d entries, want 4", len(entries))
	}
	if entries[3].Content != "answer two" || entries[3].Role != "assistant" {
		t.Fatalf("last entry wrong: %+v", entries[3])
	}

	if _, err := readJurorTranscript(path, "member-b"); err == nil {
		t.Fatalf("member mismatch did not error")
	}
}
