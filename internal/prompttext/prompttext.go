package prompttext

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var tokenPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

func Render(name, source string, values map[string]string) (string, error) {
	tokens := make([]string, 0, len(values))
	for token := range values {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	withoutValues := make([]string, 0, len(tokens)*2)
	replacements := make([]string, 0, len(tokens)*2)
	for _, token := range tokens {
		withoutValues = append(withoutValues, token, "")
		replacements = append(replacements, token, values[token])
	}
	remaining := strings.NewReplacer(withoutValues...).Replace(source)
	if unresolved := tokenPattern.FindString(remaining); unresolved != "" {
		return "", fmt.Errorf("render %s prompt: unsupported token %s", name, unresolved)
	}
	if strings.Contains(remaining, "{{") {
		return "", fmt.Errorf("render %s prompt: unmatched template delimiter", name)
	}
	rendered := strings.TrimSpace(strings.NewReplacer(replacements...).Replace(source))
	if rendered == "" {
		return "", fmt.Errorf("render %s prompt: prompt is empty", name)
	}
	return rendered, nil
}
