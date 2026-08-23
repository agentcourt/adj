package mcpbridge

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentcourt/adj/common/promptfile"
)

type PromptDefinition struct {
	ID           string
	RelativePath string
	Fallback     string
	Tokens       []string
}

type PromptCatalogOptions struct {
	Procedure   string
	PromptDir   string
	PromptFiles map[string]string
}

type PromptCatalog struct {
	procedure   string
	sources     map[string]string
	definitions map[string]PromptDefinition
}

func LoadPromptCatalog(opts PromptCatalogOptions, definitions []PromptDefinition) (*PromptCatalog, error) {
	procedure := strings.TrimSpace(opts.Procedure)
	if procedure == "" {
		return nil, fmt.Errorf("prompt catalog procedure is required")
	}
	defined := make(map[string]PromptDefinition, len(definitions))
	for _, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.RelativePath = filepath.Clean(strings.TrimSpace(definition.RelativePath))
		if definition.ID == "" {
			return nil, fmt.Errorf("%s MCP prompt ID is required", procedure)
		}
		if !strings.HasPrefix(definition.ID, "mcp.") {
			return nil, fmt.Errorf("%s MCP prompt ID %q must begin with mcp.", procedure, definition.ID)
		}
		if _, exists := defined[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate %s MCP prompt ID %q", procedure, definition.ID)
		}
		if definition.RelativePath == "." || filepath.IsAbs(definition.RelativePath) || escapesDirectory(definition.RelativePath) || !withinMCPPromptDirectory(definition.RelativePath) {
			return nil, fmt.Errorf("%s MCP prompt %s has invalid relative path %q", procedure, definition.ID, definition.RelativePath)
		}
		defined[definition.ID] = definition
	}
	if len(defined) == 0 {
		return nil, fmt.Errorf("%s MCP prompt catalog is empty", procedure)
	}
	promptFiles, err := validatePromptFiles(opts.PromptFiles, defined)
	if err != nil {
		return nil, fmt.Errorf("configure %s MCP prompts: %w", procedure, err)
	}
	promptDir := strings.TrimSpace(opts.PromptDir)
	sources := make(map[string]string, len(defined))
	ids := make([]string, 0, len(defined))
	for id := range defined {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		definition := defined[id]
		explicitPath := strings.TrimSpace(promptFiles[id])
		if explicitPath == "" && promptDir != "" {
			explicitPath = filepath.Join(promptDir, definition.RelativePath)
		}
		conventionalPath := filepath.Join("prompts", procedure, definition.RelativePath)
		source, err := promptfile.Load(procedure+" MCP "+id, explicitPath, conventionalPath, definition.Fallback)
		if err != nil {
			return nil, err
		}
		if err := promptfile.Validate(procedure+" MCP "+id, source, definition.Tokens...); err != nil {
			return nil, err
		}
		sources[id] = source
	}
	return &PromptCatalog{procedure: procedure, sources: sources, definitions: defined}, nil
}

func (c *PromptCatalog) Render(id string, values map[string]string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("MCP prompt catalog is nil")
	}
	definition, ok := c.definitions[id]
	if !ok {
		return "", fmt.Errorf("unknown %s MCP prompt ID %q", c.procedure, id)
	}
	if values == nil {
		values = map[string]string{}
	}
	allowed := make(map[string]struct{}, len(definition.Tokens))
	replacements := make([]string, 0, len(definition.Tokens)*2)
	for _, token := range definition.Tokens {
		allowed[token] = struct{}{}
		value, exists := values[token]
		if !exists {
			return "", fmt.Errorf("render %s MCP prompt %s: value for %s is required", c.procedure, id, token)
		}
		replacements = append(replacements, token, value)
	}
	for token := range values {
		if _, ok := allowed[token]; !ok {
			return "", fmt.Errorf("render %s MCP prompt %s: token %s is unavailable", c.procedure, id, token)
		}
	}
	return promptfile.Render(c.procedure+" MCP "+id, c.sources[id], replacements...)
}

func (c *PromptCatalog) Text(id string) (string, error) {
	return c.Render(id, nil)
}

func validatePromptFiles(files map[string]string, definitions map[string]PromptDefinition) (map[string]string, error) {
	validated := make(map[string]string, len(files))
	for id, path := range files {
		id = strings.TrimSpace(id)
		path = strings.TrimSpace(path)
		if _, ok := definitions[id]; !ok {
			return nil, fmt.Errorf("unknown prompt ID %q", id)
		}
		if path == "" {
			return nil, fmt.Errorf("prompt file path for %s is empty", id)
		}
		validated[id] = path
	}
	return validated, nil
}

func escapesDirectory(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func withinMCPPromptDirectory(path string) bool {
	return strings.HasPrefix(path, "mcp"+string(filepath.Separator))
}

type PromptFiles struct {
	values map[string]string
}

func (f *PromptFiles) Set(value string) error {
	id, path, ok := strings.Cut(value, "=")
	id = strings.TrimSpace(id)
	path = strings.TrimSpace(path)
	if !ok || id == "" || path == "" {
		return fmt.Errorf("prompt file must have the form ID=PATH")
	}
	if f.values == nil {
		f.values = map[string]string{}
	}
	if _, exists := f.values[id]; exists {
		return fmt.Errorf("prompt file ID %q was repeated", id)
	}
	f.values[id] = path
	return nil
}

func (f *PromptFiles) String() string {
	if f == nil || len(f.values) == 0 {
		return ""
	}
	values := make([]string, 0, len(f.values))
	for id, path := range f.values {
		values = append(values, id+"="+path)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func (f *PromptFiles) Map() map[string]string {
	if f == nil {
		return nil
	}
	result := make(map[string]string, len(f.values))
	for id, path := range f.values {
		result[id] = path
	}
	return result
}
