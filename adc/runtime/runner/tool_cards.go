package runner

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed tool_cards/*/*.md
var toolCardFS embed.FS

func buildTurnPrompt(roleName string, basePrompt string, allowedTools []string) string {
	return buildTurnPromptWithResolver(roleName, basePrompt, allowedTools, toolSchema)
}

func (r *Runner) buildTurnPrompt(roleName string, basePrompt string, allowedTools []string) string {
	return buildTurnPromptWithResolver(roleName, basePrompt, allowedTools, r.toolSchema)
}

func buildTurnPromptWithResolver(roleName string, basePrompt string, allowedTools []string, resolveSchema func(string) map[string]any) string {
	basePrompt = strings.TrimSpace(basePrompt)
	schemaLines := toolSchemaPromptLinesWithResolver(allowedTools, resolveSchema)
	cards := collectToolCards(roleName, allowedTools)
	if len(schemaLines) == 0 && len(cards) == 0 {
		return basePrompt
	}
	parts := []string{basePrompt}
	if len(schemaLines) > 0 {
		parts = append(parts, "Tool payloads:")
		parts = append(parts, schemaLines...)
	}
	if len(cards) > 0 {
		parts = append(parts, "Applicable tool guidance:")
		parts = append(parts, cards...)
	}
	return strings.Join(parts, "\n\n")
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

func collectToolCards(roleName string, allowedTools []string) []string {
	roleName = strings.TrimSpace(roleName)
	seen := map[string]bool{}
	cards := make([]string, 0, len(allowedTools))
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		card := lookupToolCard(roleName, toolName)
		if card == "" {
			continue
		}
		cards = append(cards, fmt.Sprintf("Tool `%s`:\n%s", toolName, card))
	}
	return cards
}

func lookupToolCard(roleName string, toolName string) string {
	paths := []string{
		fmt.Sprintf("tool_cards/%s/%s.md", roleName, toolName),
		fmt.Sprintf("tool_cards/shared/%s.md", toolName),
	}
	for _, path := range paths {
		raw, err := toolCardFS.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(raw))
		if text != "" {
			return text
		}
	}
	return ""
}
