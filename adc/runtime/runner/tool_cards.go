package runner

import (
	"fmt"
	"strings"
)

func (r *Runner) buildTurnPrompt(roleName string, basePrompt string, allowedTools []string) (string, error) {
	renderer, err := r.runtimePromptRenderer()
	if err != nil {
		return "", err
	}
	return renderer.RenderTurnPrompt(roleName, basePrompt, allowedTools)
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

func toolSchemaPromptLinesWithErrorResolver(allowedTools []string, resolveSchema func(string) (map[string]any, error)) ([]string, error) {
	seen := map[string]bool{}
	lines := make([]string, 0, len(allowedTools))
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		schema, err := resolveSchema(toolName)
		if err != nil {
			return nil, err
		}
		if schema == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("Tool `%s` payload: %s", toolName, marshalString(schema)))
	}
	return lines, nil
}

func (r *Runner) collectToolCards(roleName string, allowedTools []string) ([]string, error) {
	renderer, err := r.runtimePromptRenderer()
	if err != nil {
		return nil, err
	}
	return renderer.collectToolCards(roleName, allowedTools)
}
