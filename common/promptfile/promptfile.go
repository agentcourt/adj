package promptfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var unresolvedTokenPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

type Spec struct {
	ID               string
	Name             string
	ConventionalPath string
	RelativePath     string
	Fallback         string
	Tokens           []string
}

type Assignments struct {
	values map[string]string
}

func (a *Assignments) Set(value string) error {
	id, path, ok := strings.Cut(value, "=")
	id = strings.TrimSpace(id)
	path = strings.TrimSpace(path)
	if !ok || id == "" || path == "" {
		return fmt.Errorf("prompt file must use ID=PATH")
	}
	if a.values == nil {
		a.values = make(map[string]string)
	}
	if _, exists := a.values[id]; exists {
		return fmt.Errorf("prompt file %q is assigned more than once", id)
	}
	a.values[id] = path
	return nil
}

func (a *Assignments) String() string {
	if a == nil || len(a.values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(a.values))
	for key := range a.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+a.values[key])
	}
	return strings.Join(parts, ",")
}

func (a *Assignments) Values() map[string]string {
	if a == nil || len(a.values) == 0 {
		return nil
	}
	values := make(map[string]string, len(a.values))
	for id, path := range a.values {
		values[id] = path
	}
	return values
}

func Resolve(specs []Spec, overrides map[string]string, promptDir string) (map[string]string, error) {
	byID := make(map[string]Spec, len(specs))
	for _, spec := range specs {
		if strings.TrimSpace(spec.ID) == "" {
			return nil, fmt.Errorf("prompt specification has an empty ID")
		}
		if _, exists := byID[spec.ID]; exists {
			return nil, fmt.Errorf("prompt specification repeats ID %q", spec.ID)
		}
		byID[spec.ID] = spec
	}
	for id, path := range overrides {
		if _, exists := byID[id]; !exists {
			return nil, fmt.Errorf("unknown prompt ID %q", id)
		}
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("prompt file %q has an empty path", id)
		}
	}
	promptDir = strings.TrimSpace(promptDir)
	resolved := make(map[string]string, len(specs))
	for _, spec := range specs {
		explicitPath := strings.TrimSpace(overrides[spec.ID])
		if explicitPath == "" && promptDir != "" {
			explicitPath = filepath.Join(promptDir, spec.RelativePath)
		}
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			name = spec.ID
		}
		source, err := Load(name, explicitPath, spec.ConventionalPath, spec.Fallback)
		if err != nil {
			return nil, err
		}
		if err := Validate(name, source, spec.Tokens...); err != nil {
			return nil, err
		}
		resolved[spec.ID] = source
	}
	return resolved, nil
}

func Load(name, explicitPath, conventionalPath, fallback string) (string, error) {
	path := strings.TrimSpace(explicitPath)
	explicit := path != ""
	if !explicit {
		path = conventionalPath
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		return string(raw), nil
	}
	if !explicit && errors.Is(err, fs.ErrNotExist) {
		return fallback, nil
	}
	return "", fmt.Errorf("read %s prompt %s: %w", name, path, err)
}

func Render(name, source string, replacements ...string) (string, error) {
	if len(replacements)%2 != 0 {
		return "", fmt.Errorf("render %s prompt: replacement list must contain token-value pairs", name)
	}
	markers := make([]string, 0, len(replacements))
	for index := 0; index < len(replacements); index += 2 {
		markers = append(markers, replacements[index], "")
	}
	remaining := strings.NewReplacer(markers...).Replace(source)
	if unresolved := unresolvedTokenPattern.FindString(remaining); unresolved != "" {
		return "", fmt.Errorf("render %s prompt: unresolved token %s", name, unresolved)
	}
	if strings.Contains(remaining, "{{") {
		return "", fmt.Errorf("render %s prompt: unmatched template delimiter", name)
	}
	rendered := strings.NewReplacer(replacements...).Replace(source)
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return "", fmt.Errorf("render %s prompt: prompt is empty", name)
	}
	return rendered, nil
}

func Validate(name, source string, tokens ...string) error {
	replacements := make([]string, 0, len(tokens)*2)
	for _, token := range tokens {
		replacements = append(replacements, token, "value")
	}
	_, err := Render(name, source, replacements...)
	return err
}
