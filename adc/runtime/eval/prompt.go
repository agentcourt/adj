package eval

import (
	"fmt"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/common/promptfile"
)

func loadJudgePromptRenderer(court courts.Profile, promptDir string, promptFiles map[string]string) (*runner.PromptRenderer, error) {
	renderer, err := runner.NewPromptRenderer(runner.PromptRendererOptions{
		Court:       court,
		PromptDir:   promptDir,
		PromptFiles: promptFiles,
	})
	if err != nil {
		return nil, fmt.Errorf("load judge eval prompts: %w", err)
	}
	return renderer, nil
}

func resolveJudgeEvalCourt(court courts.Profile) (courts.Profile, error) {
	if strings.TrimSpace(court.Name) == "" {
		return courts.Resolve("")
	}
	if err := court.Validate(); err != nil {
		return courts.Profile{}, err
	}
	return court, nil
}

func applyJudgeEvalCourt(state map[string]any, court courts.Profile) {
	state["court_name"] = court.Name
	state["court_profile"] = court
}

func buildJudgeEvalInput(
	renderer *runner.PromptRenderer,
	toolName string,
	view map[string]any,
	opportunity map[string]any,
	objective string,
) ([]map[string]any, error) {
	if renderer == nil {
		return nil, fmt.Errorf("judge eval prompt renderer is nil")
	}
	role, err := renderer.JudgeRole([]string{toolName})
	if err != nil {
		return nil, err
	}
	promptOpportunity := runner.PromptOpportunityFromMap(opportunity)
	promptOpportunity.Objective = strings.TrimSpace(objective)
	return renderer.RenderOpportunityInput(role, view, promptOpportunity)
}

func buildJudgeEvalTools(renderer *runner.PromptRenderer, toolName string, opportunity map[string]any) ([]map[string]any, error) {
	if renderer == nil {
		return nil, fmt.Errorf("judge eval prompt renderer is nil")
	}
	role, err := renderer.JudgeRole([]string{toolName})
	if err != nil {
		return nil, err
	}
	return renderer.OpportunityTools(role, runner.PromptOpportunityFromMap(opportunity))
}

func renderJudgeCandidatePrompt(name string, template string, replacements ...string) (string, error) {
	return promptfile.Render(name, template, replacements...)
}
