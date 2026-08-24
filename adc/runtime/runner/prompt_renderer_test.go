package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
)

func TestPromptRendererRendersProbeWithoutDefaultCourtAsset(t *testing.T) {
	dir := t.TempDir()
	identityPath := filepath.Join(dir, "identity.md")
	if err := os.WriteFile(identityPath, []byte("Assigned identity: {{PERSONA}}"), 0o644); err != nil {
		t.Fatalf("write probe prompt: %v", err)
	}
	t.Chdir(dir)

	renderer, err := NewPromptRenderer(PromptRendererOptions{PromptFiles: map[string]string{
		adcprompts.ProbeJurorIdentityID: identityPath,
	}})
	if err != nil {
		t.Fatalf("NewPromptRenderer: %v", err)
	}
	got, err := renderer.RenderJurorProbeIdentity("requires corroboration")
	if err != nil {
		t.Fatalf("RenderJurorProbeIdentity: %v", err)
	}
	if got != "Assigned identity: requires corroboration" {
		t.Fatalf("RenderJurorProbeIdentity = %q", got)
	}
	if _, err := renderer.BuildTools([]string{"submit_juror_vote"}); err != nil {
		t.Fatalf("BuildTools for court-independent tool: %v", err)
	}
}

func TestPromptRendererJudgeRoleReportsMissingDefaultCourt(t *testing.T) {
	renderer, err := NewPromptRenderer(PromptRendererOptions{})
	if err != nil {
		t.Fatalf("NewPromptRenderer: %v", err)
	}
	renderer.resolveCourt = func(string) (courts.Profile, error) {
		return courts.Profile{}, fmt.Errorf("read court profile: missing")
	}
	_, err = renderer.JudgeRole(nil)
	if err == nil || !strings.Contains(err.Error(), "read court profile") {
		t.Fatalf("JudgeRole error = %v", err)
	}
	if _, err := renderer.BuildTools([]string{"decide_rule12_motion"}); err == nil || !strings.Contains(err.Error(), "read court profile") {
		t.Fatalf("BuildTools Rule 12 error = %v", err)
	}
}

func TestNewPromptRendererValidatesProvidedCourt(t *testing.T) {
	t.Parallel()

	_, err := NewPromptRenderer(PromptRendererOptions{Court: courts.Profile{
		RulesMarkdown: "rules without a court name",
	}})
	if err == nil || !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("NewPromptRenderer error = %v", err)
	}
}
