package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/common/openai"
)

func TestRunLLMHelpIncludesPromptOptions(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunLLM(context.Background(), []string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunLLM: %v", err)
	}
	for _, option := range []string{"-input-file", "-prompt-dir", "-prompt-file"} {
		if !strings.Contains(stderr.String(), option) {
			t.Errorf("help omits %q:\n%s", option, stderr.String())
		}
	}
}

func TestRunLLMRejectsUnknownPromptID(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunLLM(context.Background(), []string{"--prompt-file", "unknown=prompt.md"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), `unknown ADC prompt id "unknown"`) {
		t.Fatalf("RunLLM error = %v", err)
	}
}

func TestLoadPromptText(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(path, []byte("from file\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := loadPromptText("", path)
	if err != nil {
		t.Fatalf("loadPromptText file: %v", err)
	}
	if got != "from file\n" {
		t.Fatalf("loadPromptText file = %q", got)
	}
	got, err = loadPromptText("inline", "")
	if err != nil {
		t.Fatalf("loadPromptText inline: %v", err)
	}
	if got != "inline" {
		t.Fatalf("loadPromptText inline = %q", got)
	}
	if _, err := loadPromptText("inline", path); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("loadPromptText mutually exclusive error = %v", err)
	}
}

func TestProbePromptFileOverrides(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	identityPath := filepath.Join(root, "identity.md")
	toolCheckPath := filepath.Join(root, "tool-check.md")
	if err := os.WriteFile(identityPath, []byte("Identity: {{PERSONA}}"), 0o644); err != nil {
		t.Fatalf("WriteFile identity: %v", err)
	}
	if err := os.WriteFile(toolCheckPath, []byte("Use the vote tool."), 0o644); err != nil {
		t.Fatalf("WriteFile tool check: %v", err)
	}
	var overrides promptFileFlag
	if err := overrides.Set(adcprompts.ProbeJurorIdentityID + "=" + identityPath); err != nil {
		t.Fatalf("Set identity override: %v", err)
	}
	if err := overrides.Set(adcprompts.ProbeJurorToolCheckID + "=" + toolCheckPath); err != nil {
		t.Fatalf("Set tool-check override: %v", err)
	}
	renderer, err := newProbePromptRenderer("", overrides)
	if err != nil {
		t.Fatalf("newProbePromptRenderer: %v", err)
	}
	identity, err := renderer.RenderJurorProbeIdentity("requires corroboration")
	if err != nil {
		t.Fatalf("RenderJurorProbeIdentity: %v", err)
	}
	if identity != "Identity: requires corroboration" {
		t.Fatalf("identity = %q", identity)
	}
	toolCheck, err := renderer.JurorProbeToolCheck()
	if err != nil {
		t.Fatalf("JurorProbeToolCheck: %v", err)
	}
	if toolCheck != "Use the vote tool." {
		t.Fatalf("tool check = %q", toolCheck)
	}
}

func TestResolveLLMPersonaSpecRandom(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "common", "etc", "personas", "persons"), 0o755); err != nil {
		t.Fatalf("MkdirAll persona error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, "common", "data", "personas"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	personaPath := filepath.Join(cwd, "common", "etc", "personas", "persons", "a.txt")
	if err := os.WriteFile(personaPath, []byte("skeptical of unsigned documents"), 0o644); err != nil {
		t.Fatalf("WriteFile persona error = %v", err)
	}
	recordsPath := filepath.Join(cwd, "common", "data", "personas", "pool.jsonl")
	record := `{"openrouter_model_id":"openai/gpt-5","provider":{"only":["openai"],"allow_fallbacks":false,"require_parameters":true},"request":{"temperature":0,"max_tokens":32},"persona":"personas/persons/a.txt"}` + "\n"
	if err := os.WriteFile(recordsPath, []byte(record), 0o644); err != nil {
		t.Fatalf("WriteFile records error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "common", "etc", "personas.csv"), []byte(record), 0o644); err != nil {
		t.Fatalf("WriteFile marker records error = %v", err)
	}

	spec, sampled, err := resolveLLMPersonaSpec("random", cwd)
	if err != nil {
		t.Fatalf("resolveLLMPersonaSpec random error = %v", err)
	}
	if !sampled {
		t.Fatalf("resolveLLMPersonaSpec random sampled = false, want true")
	}
	if spec.File != "personas/persons/a.txt" {
		t.Fatalf("resolveLLMPersonaSpec random file = %q", spec.File)
	}
}

func TestResolveLLMPersonaSpecFallsBackToEtcBase(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "etc", "personas", "persons"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	personaPath := filepath.Join(cwd, "etc", "personas", "persons", "b.txt")
	if err := os.WriteFile(personaPath, []byte("requires corroboration"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	spec, sampled, err := resolveLLMPersonaSpec("openrouter://openai/gpt-5-mini,personas/persons/b.txt", cwd)
	if err != nil {
		t.Fatalf("resolveLLMPersonaSpec fallback error = %v", err)
	}
	if sampled {
		t.Fatalf("resolveLLMPersonaSpec fallback sampled = true, want false")
	}
	if spec.File != "personas/persons/b.txt" {
		t.Fatalf("resolveLLMPersonaSpec fallback file = %q", spec.File)
	}
}

func TestExtractToolCheckArguments(t *testing.T) {
	t.Parallel()

	resp := openai.Response{
		ToolCalls: []openai.ToolCall{
			{
				Name: llmToolCheckName,
				Arguments: map[string]any{
					"juror_id":    "J1",
					"damages":     float64(1),
					"vote":        "plaintiff",
					"confidence":  "high",
					"explanation": "defendant admitted breach",
				},
			},
		},
	}
	got, err := extractToolCheckArguments(resp)
	if err != nil {
		t.Fatalf("extractToolCheckArguments error = %v", err)
	}
	if got != `{"confidence":"high","damages":1,"explanation":"defendant admitted breach","juror_id":"J1","vote":"plaintiff"}` {
		t.Fatalf("extractToolCheckArguments = %q", got)
	}
}

func TestExtractToolCheckArgumentsRejectsMissingRequiredTool(t *testing.T) {
	t.Parallel()

	_, err := extractToolCheckArguments(openai.Response{Text: "plain text"})
	if err == nil {
		t.Fatalf("extractToolCheckArguments error = nil")
	}
}
