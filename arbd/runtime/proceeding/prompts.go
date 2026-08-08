package proceeding

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var promptBaseDir string

var promptPlaceholderPattern = regexp.MustCompile(`\{\{([A-Z0-9_]+)\}\}`)

func renderPromptFile(name string, values map[string]string) (string, error) {
	path, err := defaultPromptPath(name)
	if err != nil {
		return "", err
	}
	return renderPromptPath(path, name, values)
}

func (cfg Config) renderPromptFile(name string, values map[string]string) (string, error) {
	path, err := cfg.promptPath(name)
	if err != nil {
		return "", err
	}
	return renderPromptPath(path, name, values)
}

func renderPromptPath(path string, name string, values map[string]string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt %s: %w", path, err)
	}
	rendered := string(raw)
	matches := promptPlaceholderPattern.FindAllStringSubmatch(rendered, -1)
	required := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		key := match[1]
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		required = append(required, key)
	}
	sort.Strings(required)
	for _, key := range required {
		value, ok := values[key]
		if !ok {
			return "", fmt.Errorf("prompt %s requires placeholder %s", name, key)
		}
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}
	if leftover := promptPlaceholderPattern.FindStringSubmatch(rendered); len(leftover) == 2 {
		return "", fmt.Errorf("prompt %s left placeholder %s unresolved", name, leftover[1])
	}
	return strings.TrimSpace(rendered), nil
}

func defaultPromptPath(name string) (string, error) {
	root, err := promptDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func (cfg Config) promptPath(name string) (string, error) {
	if override := strings.TrimSpace(cfg.promptFileOverride(name)); override != "" {
		return override, nil
	}
	if dir := strings.TrimSpace(cfg.PromptDir); dir != "" {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("stat prompt %s: %w", candidate, err)
		}
	}
	return defaultPromptPath(name)
}

func (cfg Config) promptFileOverride(name string) string {
	switch name {
	case "attorney-common.md":
		return cfg.AttorneyCommonPromptPath
	case "attorney-openings.md":
		return cfg.AttorneyOpeningPromptPath
	case "attorney-arguments.md":
		return cfg.AttorneyArgumentPromptPath
	case "attorney-rebuttals.md":
		return cfg.AttorneyRebuttalPromptPath
	case "attorney-surrebuttals.md":
		return cfg.AttorneySurrebuttalPromptPath
	case "attorney-closings.md":
		return cfg.AttorneyClosingPromptPath
	default:
		return ""
	}
}

func promptDir() (string, error) {
	if promptBaseDir != "" {
		return promptBaseDir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}
	return filepath.Join(cwd, "prompts"), nil
}

func attorneyPromptFile(phase string) (string, error) {
	switch phase {
	case "openings":
		return "attorney-openings.md", nil
	case "arguments":
		return "attorney-arguments.md", nil
	case "rebuttals":
		return "attorney-rebuttals.md", nil
	case "surrebuttals":
		return "attorney-surrebuttals.md", nil
	case "closings":
		return "attorney-closings.md", nil
	default:
		return "", fmt.Errorf("unsupported attorney prompt phase %q", phase)
	}
}
