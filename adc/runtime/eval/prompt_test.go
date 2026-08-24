package eval

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/runner"
)

func TestJudgeEvalInputMatchesProductionPromptRenderer(t *testing.T) {
	t.Parallel()

	court := testEvalCourt()
	renderer, err := runner.NewPromptRenderer(runner.PromptRendererOptions{Court: court})
	if err != nil {
		t.Fatalf("NewPromptRenderer error = %v", err)
	}
	fixture := testRule52Fixture("plaintiff")
	opportunity := map[string]any{
		"actor_message": "Issue the bench opinion.",
		"objective":     "Production objective.",
		"phase":         "verdict_return",
		"allowed_tools": []any{JudgeRule52Tool},
		"constraints":   map[string]any{"case_index": 0},
		"may_pass":      true,
	}
	variant := judgeRule52PromptVariant{
		Text:     "Candidate objective for {{fixture_id}}: {{production_objective}}",
		Renderer: renderer,
	}
	view := map[string]any{
		"ok": true,
		"view": map[string]any{
			"role": "judge",
			"case": map[string]any{"status": "trial"},
		},
	}
	got, err := buildJudgeRule52Input(view, opportunity, fixture, variant)
	if err != nil {
		t.Fatalf("buildJudgeRule52Input error = %v", err)
	}

	role, err := renderer.JudgeRole([]string{JudgeRule52Tool})
	if err != nil {
		t.Fatalf("JudgeRole error = %v", err)
	}
	promptOpportunity := runner.PromptOpportunityFromMap(opportunity)
	promptOpportunity.Objective, err = renderJudgeRule52PromptTemplate(variant.Text, fixture, opportunity)
	if err != nil {
		t.Fatalf("renderJudgeRule52PromptTemplate error = %v", err)
	}
	want, err := renderer.RenderOpportunityInput(role, view, promptOpportunity)
	if err != nil {
		t.Fatalf("RenderOpportunityInput error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("eval input differs from production renderer\ngot:  %#v\nwant: %#v", got, want)
	}
	systemPrompt, _ := got[0]["content"].(string)
	if !strings.Contains(systemPrompt, `"ok":true`) || !strings.Contains(systemPrompt, `"view":{`) {
		t.Fatalf("system prompt lacks full role_view response: %s", systemPrompt)
	}
}

func TestJudgeCandidatePromptRejectsUnknownToken(t *testing.T) {
	t.Parallel()

	_, err := renderJudgeRule52PromptTemplate(
		"Decide {{fixture_id}} and {{unknown_token}}.",
		testRule52Fixture("plaintiff"),
		map[string]any{"objective": "Decide."},
	)
	if err == nil || !strings.Contains(err.Error(), "unresolved token {{unknown_token}}") {
		t.Fatalf("render error = %v", err)
	}
}

func TestJudgeEvalInputUsesPromptOverrideAndCourt(t *testing.T) {
	t.Parallel()

	overridePath := filepath.Join(t.TempDir(), "system.md")
	override := "EVAL SYSTEM {{ROLE}} | {{PREAMBLE}} | {{INSTRUCTIONS}} | {{ALLOWED_ACTIONS}} | {{VIEW}}"
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	court := testEvalCourt()
	renderer, err := loadJudgePromptRenderer(court, "", map[string]string{
		adcprompts.RuntimeSystemID: overridePath,
	})
	if err != nil {
		t.Fatalf("loadJudgePromptRenderer error = %v", err)
	}
	opportunity := map[string]any{
		"objective":     "Decide.",
		"allowed_tools": []any{JudgeRule52Tool},
	}
	input, err := buildJudgeEvalInput(renderer, JudgeRule52Tool, map[string]any{"role": "judge"}, opportunity, "Decide.")
	if err != nil {
		t.Fatalf("buildJudgeEvalInput error = %v", err)
	}
	systemPrompt, _ := input[0]["content"].(string)
	for _, text := range []string{"EVAL SYSTEM judge", court.RulesMarkdown} {
		if !strings.Contains(systemPrompt, text) {
			t.Fatalf("system prompt missing %q: %s", text, systemPrompt)
		}
	}
	tools, err := renderer.BuildTools([]string{JudgeRule12Tool})
	if err != nil {
		t.Fatalf("BuildTools error = %v", err)
	}
	parameters, _ := tools[0]["parameters"].(map[string]any)
	properties, _ := parameters["properties"].(map[string]any)
	ground, _ := properties["ground"].(map[string]any)
	grounds, _ := ground["enum"].([]any)
	for _, value := range grounds {
		if value == "lack_subject_matter_jurisdiction" {
			t.Fatalf("custom court Rule 12 grounds = %v", grounds)
		}
	}
}

func testEvalCourt() courts.Profile {
	return courts.Profile{
		Name:                     "Eval Court",
		RulesMarkdown:            "Apply the Eval Court rules.",
		JurisdictionScreen:       false,
		AllowedJurisdictionBases: []string{"federal_question"},
	}
}
