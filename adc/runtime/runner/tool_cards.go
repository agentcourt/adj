package runner

import (
	"fmt"
	"strings"

	adcprompts "github.com/jsmorph/adj/adc/runtime/prompts"
)

func (r *Runner) buildTurnPrompt(roleName string, basePrompt string, allowedTools []string) (string, error) {
	schemaLines := toolSchemaPromptLinesWithResolver(allowedTools, r.toolSchema)
	cards, err := r.collectToolCards(roleName, allowedTools)
	if err != nil {
		return "", err
	}
	return r.prompts.Render(adcprompts.RuntimeTurnID, map[string]string{
		"{{BASE_PROMPT}}":   promptValue(basePrompt),
		"{{TOOL_SCHEMAS}}":  promptSections(schemaLines),
		"{{TOOL_GUIDANCE}}": promptSections(cards),
	})
}

func toolSchemaPromptLines(allowedTools []string) []string {
	return toolSchemaPromptLinesWithResolver(allowedTools, toolSchema)
}

func toolSchemaPromptLinesWithResolver(allowedTools []string, resolveSchema func(string) map[string]any) []string {
	seen := map[string]bool{}
	lines := make([]string, 0, len(allowedTools))
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		schema := resolveSchema(toolName)
		if schema == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("Tool `%s` payload: %s", toolName, marshalString(schema)))
	}
	return lines
}

func (r *Runner) collectToolCards(roleName string, allowedTools []string) ([]string, error) {
	roleName = strings.TrimSpace(roleName)
	seen := map[string]bool{}
	cards := make([]string, 0, len(allowedTools))
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		card, err := r.prompts.ToolCard(roleName, toolName)
		if err != nil {
			return nil, err
		}
		cards = append(cards, fmt.Sprintf("Tool `%s`:\n%s", toolName, card))
	}
	return cards, nil
}
