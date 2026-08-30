package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type coreProcessFunc func(context.Context, coreProcessRequest) ([]byte, error)

type SimpleRunner struct {
	runProcess coreProcessFunc
}

type simpleCoreResult struct {
	Status     string `json:"status"`
	Phase      string `json:"phase"`
	Error      string `json:"error"`
	ErrorClass string `json:"error_class"`
	Decision   *struct {
		Value     string `json:"value"`
		Rationale string `json:"rationale"`
	} `json:"decision"`
	Provider ProviderManagement `json:"provider"`
}

type CoreRunError struct {
	Class    string
	Err      error
	Provider ProviderManagement
}

func (e *CoreRunError) Error() string {
	return e.Err.Error()
}

func (e *CoreRunError) Unwrap() error { return e.Err }

func (e *CoreRunError) ErrorClass() string { return strings.TrimSpace(e.Class) }

func (e *CoreRunError) ProviderAccounting() ProviderManagement { return e.Provider }

func (r SimpleRunner) Run(ctx context.Context, request ProcedureRequest) (ProcedureOutcome, error) {
	settings := request.Settings.Procedure.Simple
	if settings == nil {
		return ProcedureOutcome{}, fmt.Errorf("resolved simple settings are absent")
	}
	env, err := directProviderEnvironment(settings.Model, request.Settings, os.Environ())
	if err != nil {
		return ProcedureOutcome{}, err
	}
	args := []string{
		"case",
		"--proposition", request.Request.Proposition,
		"--documents", filepath.Join(request.RecordDir, "inputs", "documents"),
		"--out-dir", request.CoreDir,
		"--case-id", request.Request.CaseID,
		"--run-id", request.Request.RunID,
		"--model", settings.Model,
		"--web-search=" + strconv.FormatBool(settings.WebSearch),
		"--evidence-standard", request.Settings.Common.EvidenceStandard,
		"--allow-api-key",
		"--max-documents", strconv.Itoa(request.Settings.Common.DocumentLimits.Count),
		"--max-document-bytes", strconv.FormatInt(request.Settings.Common.DocumentLimits.PerFile, 10),
		"--max-documents-bytes", strconv.FormatInt(request.Settings.Common.DocumentLimits.Total, 10),
	}
	if settings.ReasoningEffort != "" {
		args = append(args, "--reasoning-effort", settings.ReasoningEffort)
	}
	if settings.MaxOutputTokens > 0 {
		args = append(args, "--max-output-tokens", strconv.FormatInt(settings.MaxOutputTokens, 10))
	}
	if settings.MaxToolCalls > 0 {
		args = append(args, "--max-tool-calls", strconv.FormatInt(settings.MaxToolCalls, 10))
	}
	args = appendCorePromptArgs(args, settings.PromptDir, settings.PromptFiles)
	if settings.Timeout != 0 {
		seconds, err := wholeSeconds(settings.Timeout)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("simple timeout: %w", err)
		}
		args = append(args, "--timeout-seconds", strconv.FormatInt(seconds, 10))
	}
	runProcess := r.runProcess
	if runProcess == nil {
		runProcess = runCoreProcess
	}
	raw, processErr := runProcess(ctx, coreProcessRequest{
		Command:    settings.CoreCommand,
		Args:       args,
		Dir:        settings.CoreWorkingDir,
		Env:        env,
		LogsDir:    request.LogsDir,
		LogPrefix:  "simple",
		ResultPath: filepath.Join(request.CoreDir, "run.json"),
		Observer:   request.Observer,
	})
	var result simpleCoreResult
	decodeErr := decodeCoreResult(raw, &result)
	if decodeErr != nil {
		return ProcedureOutcome{}, errors.Join(processErr, decodeErr)
	}
	if result.Status != "ok" {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = fmt.Sprintf("simple core returned status %q", result.Status)
		}
		return ProcedureOutcome{}, &CoreRunError{Class: result.ErrorClass, Err: errors.Join(errors.New(message), processErr), Provider: result.Provider}
	}
	if processErr != nil {
		return ProcedureOutcome{}, &CoreRunError{Err: processErr, Provider: result.Provider}
	}
	if result.Decision == nil {
		return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("simple core result has no decision"), Provider: result.Provider}
	}
	return ProcedureOutcome{
		Status: StatusOK,
		Phase:  result.Phase,
		Decision: &Decision{
			Kind:      "binary",
			Value:     result.Decision.Value,
			Rationale: result.Decision.Rationale,
		},
		ProcedureResult: append(json.RawMessage(nil), raw...),
		Provider:        result.Provider,
	}, nil
}

func decodeCoreResult(raw []byte, destination any) error {
	if len(raw) == 0 {
		return fmt.Errorf("core result is empty")
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return fmt.Errorf("decode core result: %w", err)
	}
	return nil
}

func directProviderEnvironment(model string, settings ResolvedSettings, base []string) ([]string, error) {
	endpoint, _, ok := strings.Cut(strings.TrimSpace(model), "://")
	if !ok {
		return nil, fmt.Errorf("model %q must be endpoint://model", model)
	}
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	canonical := ""
	switch endpoint {
	case "openai":
		canonical = "OPENAI_API_KEY"
	case "openrouter":
		canonical = "OPENROUTER_API_KEY"
	default:
		return nil, fmt.Errorf("unsupported direct-provider endpoint %q", endpoint)
	}
	env, err := coreProviderEnvironmentFor(settings, base, endpoint)
	if err != nil {
		return nil, err
	}
	value, ok := environmentValue(env, canonical)
	if !ok || value == "" {
		return nil, fmt.Errorf("provider credential %q was not installed", endpoint)
	}
	return env, nil
}

func coreProviderEnvironmentFor(settings ResolvedSettings, base []string, providers ...string) ([]string, error) {
	return directProviderEnvironmentRemoving(settings.Common.ProviderCredentials, base, credentialEnvironmentNames(settings), providers...)
}

func directProviderEnvironmentRemoving(sources map[string]CredentialMetadata, base, remove []string, providers ...string) ([]string, error) {
	providers = append([]string(nil), providers...)
	for index := range providers {
		providers[index] = strings.ToLower(strings.TrimSpace(providers[index]))
	}
	sort.Strings(providers)

	values := make(map[string]string, len(providers))
	remove = append([]string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"}, remove...)
	for _, provider := range providers {
		canonical := canonicalProviderEnvironment(provider)
		if canonical == "" {
			return nil, fmt.Errorf("unsupported direct-provider endpoint %q", provider)
		}
		source, ok := sources[provider]
		if !ok {
			return nil, fmt.Errorf("provider credential %q is absent", provider)
		}
		if source.Source != AuthAPIKey {
			return nil, fmt.Errorf("provider credential %q must use api_key", provider)
		}
		sourceName := strings.TrimSpace(source.EnvironmentVariable)
		if sourceName == "" {
			return nil, fmt.Errorf("provider credential %q environment variable is empty", provider)
		}
		value, ok := environmentValue(base, sourceName)
		if !ok || value == "" {
			return nil, fmt.Errorf("API-key source environment variable %s is absent or empty", sourceName)
		}
		values[provider] = value
		remove = append(remove, sourceName)
	}
	env := removeEnvironment(base, remove...)
	for _, provider := range providers {
		env = append(env, canonicalProviderEnvironment(provider)+"="+values[provider])
	}
	return env, nil
}

func credentialEnvironmentNames(settings ResolvedSettings) []string {
	names := make([]string, 0, len(settings.Common.ProviderCredentials)+len(settings.AgentProfiles))
	for _, credential := range settings.Common.ProviderCredentials {
		if name := strings.TrimSpace(credential.EnvironmentVariable); name != "" {
			names = append(names, name)
		}
	}
	for _, profile := range settings.AgentProfiles {
		if profile.Authentication.Source != AuthAPIKey {
			continue
		}
		if name := strings.TrimSpace(profile.Authentication.EnvironmentVariable); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func canonicalProviderEnvironment(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	default:
		return ""
	}
}

func wholeSeconds(duration Duration) (int64, error) {
	value := time.Duration(duration)
	if value <= 0 || value%time.Second != 0 {
		return 0, fmt.Errorf("duration must be a positive whole number of seconds")
	}
	return int64(value / time.Second), nil
}

func environmentValue(env []string, name string) (string, bool) {
	for index := len(env) - 1; index >= 0; index-- {
		key, value, ok := strings.Cut(env[index], "=")
		if ok && key == name {
			return value, true
		}
	}
	return "", false
}

func removeEnvironment(env []string, names ...string) []string {
	removed := make(map[string]struct{}, len(names))
	for _, name := range names {
		removed[name] = struct{}{}
	}
	result := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if _, remove := removed[name]; ok && remove {
			continue
		}
		result = append(result, entry)
	}
	return result
}
