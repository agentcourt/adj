package runner

import "strings"

func (r *Runner) legalToolSchemaLines(allowedTools []string) []string {
	lines := make([]string, 0, len(allowedTools))
	seen := map[string]bool{}
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		schema := r.toolSchema(toolName)
		if schema == nil {
			continue
		}
		lines = append(lines, "- "+toolName+": "+marshalString(schema))
	}
	return lines
}

func issueText(issue correctionIssue) string {
	if strings.TrimSpace(issue.ActorMessage) != "" {
		return strings.TrimSpace(issue.ActorMessage)
	}
	if strings.TrimSpace(issue.Error) != "" {
		return strings.TrimSpace(issue.Error)
	}
	return "request failed"
}
