// Package agent prepares isolated invocations of supported headless agents.
package agent

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Runner string

const (
	RunnerPi     Runner = "pi"
	RunnerCodex  Runner = "codex"
	RunnerClaude Runner = "claude"
)

type Provider string

const (
	ProviderOpenAI     Provider = "openai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenRouter Provider = "openrouter"
	ProviderGoogle     Provider = "google"
)

type AuthMode string

const (
	AuthSubscription AuthMode = "subscription"
	AuthAPIKey       AuthMode = "api_key"
)

// Auth selects subscription credentials or an explicitly named API-key source.
type Auth struct {
	Mode            AuthMode `json:"mode,omitempty"`
	CredentialsFile string   `json:"credentials_file,omitempty"`
	APIKeyEnv       string   `json:"api_key_env,omitempty"`
}

// Profile contains reusable settings for one headless agent command.
type Profile struct {
	Runner             Runner   `json:"runner"`
	Provider           Provider `json:"provider,omitempty"`
	Command            string   `json:"command,omitempty"`
	Model              string   `json:"model,omitempty"`
	ReasoningEffort    string   `json:"reasoning_effort,omitempty"`
	MCPAdapter         string   `json:"mcp_adapter,omitempty"`
	WebSearchExtension string   `json:"web_search_extension,omitempty"`
	Auth               Auth     `json:"auth,omitempty"`
	Resume             *bool    `json:"resume,omitempty"`
}

// MCPServer identifies the sole Streamable HTTP server available to an agent.
type MCPServer struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	BearerToken string `json:"bearer_token,omitempty"`
}

// Assignment contains the state and request for one agent invocation.
type Assignment struct {
	StateDir    string    `json:"state_dir"`
	WorkDir     string    `json:"work_dir"`
	EvidenceDir string    `json:"evidence_dir,omitempty"`
	Prompt      string    `json:"prompt"`
	MCP         MCPServer `json:"mcp"`
	WebSearch   *bool     `json:"web_search,omitempty"`
}

// Invocation describes a command prepared for os/exec.
type Invocation struct {
	Command     string
	Args        []string
	Env         []string
	Dir         string
	StateDir    string
	EvidenceDir string
	Resumed     bool
	SessionID   string

	credentialEnv       string
	credentialSourceEnv string
	successMarker       string
	secretFiles         []string
	secretEnv           []string
	usageCheckpoint     string
	usageBaseline       *usageCheckpoint
}

// CredentialEnvironmentNames returns the child provider variable and configured source variable.
// Both are empty for subscription authentication.
func (i *Invocation) CredentialEnvironmentNames() (target, source string) {
	if i == nil {
		return "", ""
	}
	return i.credentialEnv, i.credentialSourceEnv
}

const mcpBearerTokenEnv = "ADJ_AGENT_MCP_BEARER_TOKEN"

var (
	mcpNamePattern      = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	envNamePattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	piProviderAPIKeyEnv = map[string]string{
		"anthropic":  "ANTHROPIC_API_KEY",
		"google":     "GEMINI_API_KEY",
		"openai":     "OPENAI_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}
	piAPIKeyEnvironmentVariables = []string{
		"AI_GATEWAY_API_KEY",
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_OAUTH_TOKEN",
		"AZURE_OPENAI_API_KEY",
		"CEREBRAS_API_KEY",
		"CLOUDFLARE_API_KEY",
		"DEEPSEEK_API_KEY",
		"EXA_API_KEY",
		"FIREWORKS_API_KEY",
		"GEMINI_API_KEY",
		"GROQ_API_KEY",
		"KIMI_API_KEY",
		"MINIMAX_API_KEY",
		"MISTRAL_API_KEY",
		"MOONSHOT_API_KEY",
		"OPENAI_API_KEY",
		"OPENCODE_API_KEY",
		"OPENROUTER_API_KEY",
		"XAI_API_KEY",
		"XIAOMI_API_KEY",
		"ZAI_API_KEY",
	}
	claudeProviderEnvironmentVariables = []string{
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_BASE_URL",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY",
		"CLAUDE_CODE_SUBAGENT_MODEL",
		"ANTHROPIC_DEFAULT_FABLE_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	}
)

// Prepare validates an assignment, stages authentication, and constructs a command.
func Prepare(profile Profile, assignment Assignment, baseEnv []string) (*Invocation, error) {
	if err := validateAssignment(assignment); err != nil {
		return nil, err
	}
	if err := ValidateProfile(profile, baseEnv); err != nil {
		return nil, err
	}

	evidenceDir := strings.TrimSpace(assignment.EvidenceDir)
	if evidenceDir != "" {
		var err error
		evidenceDir, err = filepath.Abs(evidenceDir)
		if err != nil {
			return nil, fmt.Errorf("resolve agent evidence directory %q: %w", assignment.EvidenceDir, err)
		}
	}
	runnerDir := filepath.Join(assignment.StateDir, string(profile.Runner))
	if err := os.MkdirAll(runnerDir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s state directory %q: %w", profile.Runner, runnerDir, err)
	}
	if err := os.Chmod(runnerDir, 0o700); err != nil {
		return nil, fmt.Errorf("set permissions on %s state directory %q: %w", profile.Runner, runnerDir, err)
	}

	invocation := &Invocation{
		Command:       strings.TrimSpace(profile.Command),
		Dir:           assignment.WorkDir,
		StateDir:      runnerDir,
		EvidenceDir:   evidenceDir,
		successMarker: filepath.Join(runnerDir, "successful-invocation"),
	}
	if invocation.Command == "" {
		invocation.Command = string(profile.Runner)
	}

	resume := profile.Resume == nil || *profile.Resume
	_, markerErr := os.Stat(invocation.successMarker)
	switch {
	case markerErr == nil:
		invocation.Resumed = resume
	case errors.Is(markerErr, os.ErrNotExist):
	case markerErr != nil:
		return nil, fmt.Errorf("inspect successful invocation marker %q: %w", invocation.successMarker, markerErr)
	}

	mode, err := profileAuthMode(profile)
	if err != nil {
		return nil, errors.Join(err, invocation.Cleanup())
	}

	switch profile.Runner {
	case RunnerPi:
		err = preparePi(invocation, profile, assignment, mode, baseEnv)
	case RunnerCodex:
		err = prepareCodex(invocation, profile, assignment, mode, baseEnv)
	case RunnerClaude:
		err = prepareClaude(invocation, profile, assignment, mode, baseEnv)
	}
	if err != nil {
		return nil, errors.Join(err, invocation.Cleanup())
	}
	return invocation, nil
}

// ValidateProfile checks authentication and runner settings without creating state.
func ValidateProfile(profile Profile, baseEnv []string) error {
	if profile.Runner != RunnerPi && profile.Runner != RunnerCodex && profile.Runner != RunnerClaude {
		return fmt.Errorf("unsupported agent runner %q", profile.Runner)
	}
	if effort := strings.TrimSpace(profile.ReasoningEffort); effort != "" && effort != "low" && effort != "medium" && effort != "high" && effort != "xhigh" {
		return fmt.Errorf("reasoning effort must be low, medium, high, or xhigh, got %q", effort)
	}
	mode, err := profileAuthMode(profile)
	if err != nil {
		return err
	}
	provider, err := resolveProfileProvider(profile)
	if err != nil {
		return err
	}
	if mode == AuthSubscription && strings.TrimSpace(profile.Auth.APIKeyEnv) != "" {
		return errors.New("subscription authentication cannot name an API-key source environment variable")
	}
	if mode == AuthAPIKey && strings.TrimSpace(profile.Auth.CredentialsFile) != "" {
		return errors.New("API-key authentication cannot name a subscription credential file")
	}
	if profile.Runner == RunnerPi {
		if _, _, _, err := resolvePiProfile(profile); err != nil {
			return err
		}
		if mode == AuthAPIKey {
			if _, err := explicitAPIKey(profile.Auth.APIKeyEnv, baseEnv); err != nil {
				return fmt.Errorf("select Pi API-key authentication: %w", err)
			}
			return nil
		}
		if provider != ProviderOpenAI {
			return fmt.Errorf("Pi subscription authentication supports only provider %q, got %q", ProviderOpenAI, provider)
		}
		source, err := credentialsSource(profile.Auth.CredentialsFile, baseEnv, filepath.Join(".codex", "auth.json"))
		if err != nil {
			return fmt.Errorf("resolve Pi subscription credentials: %w", err)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read credentials %q for Pi subscription authentication: %w", source, err)
		}
		if _, err := piCodexCredentials(data); err != nil {
			return fmt.Errorf("validate credentials %q for Pi subscription authentication: %w", source, err)
		}
		return nil
	}
	if profile.Runner == RunnerClaude && provider == ProviderOpenRouter && mode != AuthAPIKey {
		return errors.New("Claude with provider openrouter requires API-key authentication")
	}
	if mode == AuthAPIKey {
		if _, err := explicitAPIKey(profile.Auth.APIKeyEnv, baseEnv); err != nil {
			return fmt.Errorf("select %s API-key authentication: %w", profile.Runner, err)
		}
		return nil
	}
	relativeDefault := filepath.Join(".codex", "auth.json")
	validate := validateCodexCredentials
	if profile.Runner == RunnerClaude {
		relativeDefault = filepath.Join(".claude", ".credentials.json")
		validate = validateClaudeCredentials
	}
	source, err := credentialsSource(profile.Auth.CredentialsFile, baseEnv, relativeDefault)
	if err != nil {
		return fmt.Errorf("resolve %s subscription credentials: %w", profile.Runner, err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read credentials %q for %s subscription authentication: %w", source, profile.Runner, err)
	}
	if err := validate(data); err != nil {
		return fmt.Errorf("validate credentials %q for %s subscription authentication: %w", source, profile.Runner, err)
	}
	return nil
}

func profileAuthMode(profile Profile) (AuthMode, error) {
	mode := profile.Auth.Mode
	if mode == "" {
		mode = AuthSubscription
	}
	if mode != AuthSubscription && mode != AuthAPIKey {
		return "", fmt.Errorf("unsupported authentication mode %q", mode)
	}
	return mode, nil
}

func preparePi(invocation *Invocation, profile Profile, assignment Assignment, mode AuthMode, baseEnv []string) error {
	provider, modelID, providerEnv, err := resolvePiProfile(profile)
	if err != nil {
		return err
	}
	if mode == AuthSubscription && provider != string(ProviderOpenAI) {
		return fmt.Errorf("Pi subscription authentication supports only provider %q, got %q", ProviderOpenAI, provider)
	}
	configDir := filepath.Join(invocation.StateDir, "config")
	sessionDir := filepath.Join(invocation.StateDir, "sessions")
	homeDir := filepath.Join(invocation.StateDir, "home")
	for _, dir := range []string{configDir, sessionDir, homeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create Pi state directory %q: %w", dir, err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("set permissions on Pi state directory %q: %w", dir, err)
		}
	}

	mcpPath := filepath.Join(configDir, "mcp.json")
	mcpConfig, err := piMCPConfig(assignment.MCP)
	if err != nil {
		return err
	}
	if err := writePrivateFile(mcpPath, mcpConfig); err != nil {
		return fmt.Errorf("write Pi MCP configuration %q: %w", mcpPath, err)
	}
	authPath := filepath.Join(configDir, "auth.json")
	invocation.secretFiles = []string{mcpPath, authPath}
	settingsPath := filepath.Join(configDir, "settings.json")
	existingSettings, err := os.ReadFile(settingsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read Pi settings %q: %w", settingsPath, err)
	}
	settings, err := piSettingsConfig(existingSettings)
	if err != nil {
		return err
	}
	if err := writePrivateFile(settingsPath, settings); err != nil {
		return fmt.Errorf("write Pi settings %q: %w", settingsPath, err)
	}
	invocation.secretEnv = append([]string{}, piAPIKeyEnvironmentVariables...)
	if sourceEnv := strings.TrimSpace(profile.Auth.APIKeyEnv); sourceEnv != "" {
		invocation.secretEnv = append(invocation.secretEnv, sourceEnv)
	}
	webSearchExtension := strings.TrimSpace(profile.WebSearchExtension)
	if assignment.WebSearch != nil && *assignment.WebSearch {
		if webSearchExtension == "" {
			return errors.New("Pi web search extension path is required when web search is enabled")
		}
		searchConfigPath := filepath.Join(configDir, "web-search.json")
		searchConfig, err := piWebSearchConfig()
		if err != nil {
			return err
		}
		if err := writePrivateFile(searchConfigPath, searchConfig); err != nil {
			return fmt.Errorf("write Pi web search configuration %q: %w", searchConfigPath, err)
		}
		invocation.secretFiles = append(invocation.secretFiles, searchConfigPath)
		invocation.secretEnv = append(invocation.secretEnv,
			"PI_ALLOW_BROWSER_COOKIES",
			"FEYNMAN_ALLOW_BROWSER_COOKIES",
		)
	}
	env := filterEnv(baseEnv, invocation.secretEnv...)
	env = setEnv(env, "HOME", homeDir)
	env = setEnv(env, "PI_CODING_AGENT_DIR", configDir)
	env = setEnv(env, "PI_CODING_AGENT_SESSION_DIR", sessionDir)
	if mode == AuthSubscription {
		source, err := credentialsSource(profile.Auth.CredentialsFile, baseEnv, filepath.Join(".codex", "auth.json"))
		if err != nil {
			return fmt.Errorf("resolve Pi subscription credentials: %w", err)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read Pi subscription credentials %q: %w", source, err)
		}
		credentials, err := piCodexCredentials(data)
		if err != nil {
			return fmt.Errorf("convert Pi subscription credentials %q: %w", source, err)
		}
		if err := writePrivateFile(authPath, credentials); err != nil {
			return fmt.Errorf("write Pi subscription credentials %q: %w", authPath, err)
		}
		provider = "openai-codex"
		providerEnv = ""
	} else {
		if err := removeIfPresent(authPath); err != nil {
			return fmt.Errorf("remove Pi subscription credentials %q: %w", authPath, err)
		}
		key, err := explicitAPIKey(profile.Auth.APIKeyEnv, baseEnv)
		if err != nil {
			return fmt.Errorf("select Pi API-key authentication: %w", err)
		}
		env = setEnv(env, providerEnv, key)
		invocation.credentialEnv = providerEnv
		invocation.credentialSourceEnv = strings.TrimSpace(profile.Auth.APIKeyEnv)
	}

	invocation.Env = env
	invocation.Args = []string{
		"--provider", provider,
		"--model", modelID,
		"--mode", "json",
		"--print",
		"--no-prompt-templates",
		"--no-context-files",
		"--extension", strings.TrimSpace(profile.MCPAdapter),
		"--session-dir", sessionDir,
	}
	if effort := strings.TrimSpace(profile.ReasoningEffort); effort != "" {
		invocation.Args = append(invocation.Args, "--thinking", effort)
	}
	if assignment.WebSearch != nil && *assignment.WebSearch {
		invocation.Args = append(invocation.Args, "--extension", webSearchExtension)
	}
	if invocation.Resumed {
		invocation.Args = append(invocation.Args, "--continue")
	}
	invocation.Args = append(invocation.Args, assignment.Prompt)
	return nil
}

func resolvePiProfile(profile Profile) (provider, modelID, providerEnv string, err error) {
	model := strings.TrimSpace(profile.Model)
	provider, modelID, ok := strings.Cut(model, "/")
	if !ok || strings.TrimSpace(provider) == "" || strings.TrimSpace(modelID) == "" {
		return "", "", "", errors.New("Pi model must include its provider, for example openrouter/anthropic/claude-sonnet-4")
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	modelID = strings.TrimSpace(modelID)
	configured := strings.ToLower(strings.TrimSpace(string(profile.Provider)))
	if configured != "" && configured != provider {
		return "", "", "", fmt.Errorf("Pi provider %q does not match model provider %q", configured, provider)
	}
	providerEnv, ok = piProviderAPIKeyEnv[provider]
	if !ok {
		return "", "", "", fmt.Errorf("unsupported Pi provider %q", provider)
	}
	if strings.TrimSpace(profile.MCPAdapter) == "" {
		return "", "", "", errors.New("Pi MCP adapter path is required")
	}
	return provider, modelID, providerEnv, nil
}

// MarkSuccessful enables session resumption for the next prepared invocation.
func (i *Invocation) MarkSuccessful() error {
	if i == nil || i.successMarker == "" {
		return errors.New("invocation has no success marker")
	}
	if err := writePrivateFile(i.successMarker, []byte("success\n")); err != nil {
		return fmt.Errorf("write successful invocation marker %q: %w", i.successMarker, err)
	}
	return nil
}

// MarkUnsuccessful prevents a later invocation from resuming this session.
func (i *Invocation) MarkUnsuccessful() error {
	if i == nil || i.successMarker == "" {
		return errors.New("invocation has no success marker")
	}
	if err := removeIfPresent(i.successMarker); err != nil {
		return fmt.Errorf("remove successful invocation marker %q: %w", i.successMarker, err)
	}
	return nil
}

// Cleanup removes staged credentials and configuration while retaining sessions.
func (i *Invocation) Cleanup() error {
	if i == nil {
		return nil
	}
	var errs []error
	for _, path := range i.secretFiles {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove staged secret %q: %w", path, err))
		}
	}
	i.Env = filterEnv(i.Env, i.secretEnv...)
	return errors.Join(errs...)
}

func prepareCodex(invocation *Invocation, profile Profile, assignment Assignment, mode AuthMode, baseEnv []string) error {
	codexHome := filepath.Join(invocation.StateDir, "home")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return fmt.Errorf("create Codex home %q: %w", codexHome, err)
	}
	if err := os.Chmod(codexHome, 0o700); err != nil {
		return fmt.Errorf("set permissions on Codex home %q: %w", codexHome, err)
	}
	invocation.usageCheckpoint = filepath.Join(invocation.StateDir, "usage.json")
	if invocation.Resumed {
		checkpoint, err := readUsageCheckpoint(invocation.usageCheckpoint)
		if err != nil {
			return fmt.Errorf("read Codex usage checkpoint: %w", err)
		}
		invocation.usageBaseline = &checkpoint
	}

	authPath := filepath.Join(codexHome, "auth.json")
	configPath := filepath.Join(codexHome, "config.toml")
	invocation.secretFiles = []string{authPath, configPath}
	invocation.secretEnv = []string{"OPENAI_API_KEY", "CODEX_API_KEY", mcpBearerTokenEnv}
	env := filterEnv(baseEnv, invocation.secretEnv...)
	env = setEnv(env, "CODEX_HOME", codexHome)

	config := strings.Builder{}
	config.WriteString("cli_auth_credentials_store = \"file\"\n")
	config.WriteString("sandbox_mode = \"workspace-write\"\n")
	if effort := strings.TrimSpace(profile.ReasoningEffort); effort != "" {
		fmt.Fprintf(&config, "model_reasoning_effort = %q\n", effort)
	}
	if assignment.WebSearch != nil {
		mode := "disabled"
		if *assignment.WebSearch {
			mode = "live"
		}
		fmt.Fprintf(&config, "web_search = %q\n", mode)
	}
	if mode == AuthSubscription {
		config.WriteString("forced_login_method = \"chatgpt\"\n")
		source, err := credentialsSource(profile.Auth.CredentialsFile, baseEnv, filepath.Join(".codex", "auth.json"))
		if err != nil {
			return fmt.Errorf("resolve Codex subscription credentials: %w", err)
		}
		if err := stageCredentials(source, authPath, validateCodexCredentials); err != nil {
			return fmt.Errorf("stage Codex subscription credentials: %w", err)
		}
	} else {
		config.WriteString("forced_login_method = \"api\"\n")
		key, err := explicitAPIKey(profile.Auth.APIKeyEnv, baseEnv)
		if err != nil {
			return fmt.Errorf("select Codex API-key authentication: %w", err)
		}
		env = filterEnv(env, profile.Auth.APIKeyEnv)
		credentials, err := json.Marshal(struct {
			AuthMode string `json:"auth_mode"`
			APIKey   string `json:"OPENAI_API_KEY"`
			Tokens   any    `json:"tokens"`
		}{AuthMode: "apikey", APIKey: key})
		if err != nil {
			return fmt.Errorf("encode Codex API-key credentials: %w", err)
		}
		if err := writePrivateFile(authPath, append(credentials, '\n')); err != nil {
			return fmt.Errorf("stage Codex API-key credentials: %w", err)
		}
	}

	config.WriteString("\n[sandbox_workspace_write]\n")
	config.WriteString("network_access = true\n")
	config.WriteString("\n[features]\n")
	for _, feature := range []string{
		"apps",
		"plugins",
		"remote_plugin",
		"recommended_plugins",
		"plugin_sharing",
		"workspace_dependencies",
	} {
		fmt.Fprintf(&config, "%s = false\n", feature)
	}
	writeCodexMCPConfig(&config, assignment.MCP)
	if err := writePrivateFile(configPath, []byte(config.String())); err != nil {
		return fmt.Errorf("write Codex configuration %q: %w", configPath, err)
	}
	if assignment.MCP.BearerToken != "" {
		env = setEnv(env, mcpBearerTokenEnv, assignment.MCP.BearerToken)
	}

	invocation.Env = env
	if invocation.Resumed {
		invocation.Args = []string{"--strict-config", "exec", "resume", "--json"}
		if model := strings.TrimSpace(profile.Model); model != "" {
			invocation.Args = append(invocation.Args, "--model", model)
		}
		invocation.Args = append(invocation.Args, "--last", "--all", assignment.Prompt)
	} else {
		invocation.Args = []string{"--strict-config", "exec", "--json"}
		if model := strings.TrimSpace(profile.Model); model != "" {
			invocation.Args = append(invocation.Args, "--model", model)
		}
		invocation.Args = append(invocation.Args, assignment.Prompt)
	}
	return nil
}

func prepareClaude(invocation *Invocation, profile Profile, assignment Assignment, mode AuthMode, baseEnv []string) error {
	provider, err := resolveProfileProvider(profile)
	if err != nil {
		return err
	}
	claudeHome := filepath.Join(invocation.StateDir, "home")
	claudeConfigDir := filepath.Join(claudeHome, ".claude")
	if err := os.MkdirAll(claudeConfigDir, 0o700); err != nil {
		return fmt.Errorf("create Claude configuration directory %q: %w", claudeConfigDir, err)
	}
	if err := os.Chmod(claudeConfigDir, 0o700); err != nil {
		return fmt.Errorf("set permissions on Claude configuration directory %q: %w", claudeConfigDir, err)
	}

	authPath := filepath.Join(claudeConfigDir, ".credentials.json")
	mcpConfigPath := filepath.Join(invocation.StateDir, "mcp.json")
	invocation.secretFiles = []string{authPath, mcpConfigPath}
	invocation.secretEnv = append([]string(nil), claudeProviderEnvironmentVariables...)
	env := filterEnv(baseEnv, invocation.secretEnv...)
	env = setEnv(env, "HOME", claudeHome)

	if mode == AuthSubscription {
		source, err := credentialsSource(profile.Auth.CredentialsFile, baseEnv, filepath.Join(".claude", ".credentials.json"))
		if err != nil {
			return fmt.Errorf("resolve Claude subscription credentials: %w", err)
		}
		if err := stageCredentials(source, authPath, validateClaudeCredentials); err != nil {
			return fmt.Errorf("stage Claude subscription credentials: %w", err)
		}
	} else {
		if err := removeIfPresent(authPath); err != nil {
			return fmt.Errorf("remove staged Claude subscription credentials: %w", err)
		}
		key, err := explicitAPIKey(profile.Auth.APIKeyEnv, baseEnv)
		if err != nil {
			return fmt.Errorf("select Claude API-key authentication: %w", err)
		}
		env = filterEnv(env, profile.Auth.APIKeyEnv)
		if provider == ProviderOpenRouter {
			model := strings.TrimSpace(profile.Model)
			env = setEnv(env, "ANTHROPIC_BASE_URL", "https://openrouter.ai/api")
			env = setEnv(env, "ANTHROPIC_AUTH_TOKEN", key)
			env = setEnv(env, "ANTHROPIC_API_KEY", "")
			for _, name := range []string{
				"CLAUDE_CODE_SUBAGENT_MODEL",
				"ANTHROPIC_DEFAULT_FABLE_MODEL",
				"ANTHROPIC_DEFAULT_OPUS_MODEL",
				"ANTHROPIC_DEFAULT_SONNET_MODEL",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL",
			} {
				env = setEnv(env, name, model)
			}
		} else {
			env = setEnv(env, "ANTHROPIC_API_KEY", key)
		}
	}

	mcpConfig, err := claudeMCPConfig(assignment.MCP)
	if err != nil {
		return err
	}
	if err := writePrivateFile(mcpConfigPath, mcpConfig); err != nil {
		return fmt.Errorf("write Claude MCP configuration %q: %w", mcpConfigPath, err)
	}
	settings, err := claudeInvocationSettings(assignment.WorkDir, authPath, mcpConfigPath)
	if err != nil {
		return err
	}

	sessionPath := filepath.Join(invocation.StateDir, "session-id")
	sessionID, err := loadOrCreateSessionID(sessionPath)
	if err != nil {
		return fmt.Errorf("prepare Claude session: %w", err)
	}
	invocation.SessionID = sessionID
	invocation.Env = env
	invocation.Args = []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--tools", "default",
		"--mcp-config", mcpConfigPath,
		"--strict-mcp-config",
		"--settings", settings,
		"--permission-mode", "dontAsk",
	}
	if profile.Resume != nil && !*profile.Resume {
		invocation.Args = append(invocation.Args, "--no-session-persistence")
	}
	if assignment.WebSearch != nil && !*assignment.WebSearch {
		invocation.Args = append(invocation.Args, "--disallowedTools", "WebSearch")
	}
	invocation.Args = append(invocation.Args, "--allowedTools", claudeAllowedTools(assignment))
	if model := strings.TrimSpace(profile.Model); model != "" {
		invocation.Args = append(invocation.Args, "--model", model)
	}
	if effort := strings.TrimSpace(profile.ReasoningEffort); effort != "" {
		invocation.Args = append(invocation.Args, "--effort", effort)
	}
	if invocation.Resumed {
		invocation.Args = append(invocation.Args, "--resume", sessionID)
	} else {
		invocation.Args = append(invocation.Args, "--session-id", sessionID)
	}
	invocation.Args = append(invocation.Args, assignment.Prompt)
	return nil
}

func resolveProfileProvider(profile Profile) (Provider, error) {
	provider := Provider(strings.ToLower(strings.TrimSpace(string(profile.Provider))))
	switch profile.Runner {
	case RunnerCodex:
		if provider == "" {
			provider = ProviderOpenAI
		}
		if provider != ProviderOpenAI {
			return "", fmt.Errorf("Codex supports only provider %q, got %q", ProviderOpenAI, provider)
		}
	case RunnerClaude:
		if provider == "" {
			provider = ProviderAnthropic
		}
		if provider != ProviderAnthropic && provider != ProviderOpenRouter {
			return "", fmt.Errorf("Claude provider must be %q or %q, got %q", ProviderAnthropic, ProviderOpenRouter, provider)
		}
		if provider == ProviderOpenRouter {
			modelProvider, modelID, ok := strings.Cut(strings.TrimSpace(profile.Model), "/")
			if !ok || strings.TrimSpace(modelProvider) == "" || strings.TrimSpace(modelID) == "" {
				return "", errors.New("Claude with provider openrouter requires an OpenRouter model slug in provider/model form")
			}
		}
	case RunnerPi:
		modelProvider, _, _, err := resolvePiProfile(profile)
		if err != nil {
			return "", err
		}
		provider = Provider(modelProvider)
	}
	return provider, nil
}

func claudeInvocationSettings(workDir, authPath, mcpConfigPath string) (string, error) {
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("resolve Claude work directory: %w", err)
	}
	absAuthPath, err := filepath.Abs(authPath)
	if err != nil {
		return "", fmt.Errorf("resolve staged Claude credential path: %w", err)
	}
	absMCPConfigPath, err := filepath.Abs(mcpConfigPath)
	if err != nil {
		return "", fmt.Errorf("resolve Claude MCP configuration path: %w", err)
	}
	deniedPaths := []string{absAuthPath, absMCPConfigPath}
	settings := map[string]any{
		"permissions": map[string]any{
			"deny": []string{
				claudePermissionPath("Read", absAuthPath),
				claudePermissionPath("Edit", absAuthPath),
				claudePermissionPath("Read", absMCPConfigPath),
				claudePermissionPath("Edit", absMCPConfigPath),
			},
		},
		"sandbox": map[string]any{
			"enabled":                  true,
			"failIfUnavailable":        true,
			"allowUnsandboxedCommands": false,
			"excludedCommands":         []string{},
			"filesystem": map[string]any{
				"allowWrite": []string{absWorkDir},
				"denyRead":   deniedPaths,
				"denyWrite":  deniedPaths,
			},
			"network": map[string]any{
				"allowedDomains":      []string{"*"},
				"allowUnixSockets":    []string{},
				"allowAllUnixSockets": false,
				"allowLocalBinding":   false,
			},
		},
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("encode Claude invocation settings: %w", err)
	}
	return string(raw), nil
}

func claudePermissionPath(tool, path string) string {
	return tool + "(" + filepath.ToSlash(path) + ")"
}

func claudeAllowedTools(assignment Assignment) string {
	tools := "mcp__" + assignment.MCP.Name + "__*," + claudeLocalToolList
	if assignment.WebSearch != nil && *assignment.WebSearch {
		tools += ",WebSearch"
	}
	return tools
}

const claudeLocalToolList = "Bash,Edit,Glob,Grep,Read,Write,WebFetch,NotebookEdit,Agent"

func validateAssignment(assignment Assignment) error {
	if strings.TrimSpace(assignment.StateDir) == "" {
		return errors.New("agent state directory is required")
	}
	if strings.TrimSpace(assignment.WorkDir) == "" {
		return errors.New("agent working directory is required")
	}
	info, err := os.Stat(assignment.WorkDir)
	if err != nil {
		return fmt.Errorf("inspect agent working directory %q: %w", assignment.WorkDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("agent working directory %q is not a directory", assignment.WorkDir)
	}
	if evidenceDir := strings.TrimSpace(assignment.EvidenceDir); evidenceDir != "" {
		info, err := os.Stat(evidenceDir)
		if err != nil {
			return fmt.Errorf("inspect agent evidence directory %q: %w", evidenceDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("agent evidence directory %q is not a directory", evidenceDir)
		}
	}
	if strings.TrimSpace(assignment.Prompt) == "" {
		return errors.New("agent prompt is required")
	}
	if !mcpNamePattern.MatchString(assignment.MCP.Name) {
		return fmt.Errorf("MCP server name %q must contain only letters, digits, underscores, or hyphens", assignment.MCP.Name)
	}
	parsed, err := url.Parse(assignment.MCP.URL)
	if err != nil {
		return fmt.Errorf("parse MCP server URL %q: %w", assignment.MCP.URL, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("MCP server URL %q must be an absolute HTTP or HTTPS URL", assignment.MCP.URL)
	}
	if parsed.User != nil {
		return fmt.Errorf("MCP server URL %q must not contain user information", assignment.MCP.URL)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("MCP server URL %q must not contain a fragment", assignment.MCP.URL)
	}
	return nil
}

func credentialsSource(configured string, env []string, relativeDefault string) (string, error) {
	if path := strings.TrimSpace(configured); path != "" {
		return path, nil
	}
	home, ok := lookupEnv(env, "HOME")
	if !ok || strings.TrimSpace(home) == "" {
		return "", errors.New("HOME is absent from the base environment")
	}
	return filepath.Join(home, relativeDefault), nil
}

func explicitAPIKey(name string, env []string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("API-key mode requires an explicit source environment-variable name")
	}
	if !envNamePattern.MatchString(name) {
		return "", fmt.Errorf("API-key source environment-variable name %q is invalid", name)
	}
	value, ok := lookupEnv(env, name)
	if !ok || value == "" {
		return "", fmt.Errorf("API-key source environment variable %s is absent or empty", name)
	}
	return value, nil
}

func stageCredentials(source, destination string, validate func([]byte) error) error {
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("staged credential path %q is not a regular file", destination)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect staged credentials %q: %w", destination, err)
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read credentials %q: %w", source, err)
	}
	if err := validate(data); err != nil {
		return fmt.Errorf("validate credentials %q: %w", source, err)
	}
	if err := writePrivateFile(destination, data); err != nil {
		return fmt.Errorf("copy credentials from %q to %q: %w", source, destination, err)
	}
	return nil
}

func validateCodexCredentials(data []byte) error {
	var document struct {
		AuthMode string          `json:"auth_mode"`
		Tokens   json.RawMessage `json:"tokens"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if document.AuthMode != "chatgpt" {
		return fmt.Errorf("auth_mode is %q; subscription authentication requires %q", document.AuthMode, "chatgpt")
	}
	if err := requireJSONObject(document.Tokens, "tokens"); err != nil {
		return err
	}
	return nil
}

func piCodexCredentials(data []byte) ([]byte, error) {
	var source struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if source.AuthMode != "chatgpt" {
		return nil, fmt.Errorf("auth_mode is %q; subscription authentication requires %q", source.AuthMode, "chatgpt")
	}
	accessToken := strings.TrimSpace(source.Tokens.AccessToken)
	refreshToken := strings.TrimSpace(source.Tokens.RefreshToken)
	accountID := strings.TrimSpace(source.Tokens.AccountID)
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "tokens.access_token", value: accessToken},
		{name: "tokens.refresh_token", value: refreshToken},
		{name: "tokens.account_id", value: accountID},
	} {
		if field.value == "" {
			return nil, fmt.Errorf("%s is required", field.name)
		}
	}
	expires, err := jwtExpirationMilliseconds(accessToken)
	if err != nil {
		return nil, fmt.Errorf("tokens.access_token: %w", err)
	}
	type oauthCredentials struct {
		Type      string `json:"type"`
		Access    string `json:"access"`
		Refresh   string `json:"refresh"`
		Expires   int64  `json:"expires"`
		AccountID string `json:"accountId"`
	}
	document := map[string]oauthCredentials{
		"openai-codex": {
			Type:      "oauth",
			Access:    accessToken,
			Refresh:   refreshToken,
			Expires:   expires,
			AccountID: accountID,
		},
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Pi credentials: %w", err)
	}
	return append(encoded, '\n'), nil
}

func jwtExpirationMilliseconds(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, errors.New("must be a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims struct {
		Expires int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("decode JWT claims: %w", err)
	}
	if claims.Expires <= 0 {
		return 0, errors.New("JWT exp claim must be a positive integer")
	}
	if claims.Expires > (1<<63-1)/1000 {
		return 0, errors.New("JWT exp claim is too large")
	}
	return claims.Expires * 1000, nil
}

func validateClaudeCredentials(data []byte) error {
	var document struct {
		ClaudeAIOAuth json.RawMessage `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := requireJSONObject(document.ClaudeAIOAuth, "claudeAiOauth"); err != nil {
		return err
	}
	return nil
}

func requireJSONObject(data json.RawMessage, field string) error {
	if len(data) == 0 {
		return fmt.Errorf("%s object is required", field)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("%s must be an object: %w", field, err)
	}
	if len(object) == 0 {
		return fmt.Errorf("%s object is empty", field)
	}
	return nil
}

func writeCodexMCPConfig(config *strings.Builder, server MCPServer) {
	config.WriteString("\n[mcp_servers.")
	config.WriteString(strconv.Quote(server.Name))
	config.WriteString("]\nurl = ")
	config.WriteString(strconv.Quote(server.URL))
	config.WriteString("\nrequired = true\ndefault_tools_approval_mode = \"approve\"\n")
	if server.BearerToken != "" {
		config.WriteString("bearer_token_env_var = ")
		config.WriteString(strconv.Quote(mcpBearerTokenEnv))
		config.WriteByte('\n')
	}
}

func claudeMCPConfig(server MCPServer) ([]byte, error) {
	type claudeServer struct {
		Type    string            `json:"type"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers,omitempty"`
	}
	configured := claudeServer{Type: "http", URL: server.URL}
	if server.BearerToken != "" {
		configured.Headers = map[string]string{"Authorization": "Bearer " + server.BearerToken}
	}
	document := struct {
		MCPServers map[string]claudeServer `json:"mcpServers"`
	}{MCPServers: map[string]claudeServer{server.Name: configured}}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Claude MCP configuration: %w", err)
	}
	return append(data, '\n'), nil
}

func piMCPConfig(server MCPServer) ([]byte, error) {
	type piServer struct {
		URL         string `json:"url"`
		Lifecycle   string `json:"lifecycle"`
		Auth        string `json:"auth,omitempty"`
		BearerToken string `json:"bearerToken,omitempty"`
	}
	configured := piServer{
		URL:       server.URL,
		Lifecycle: "keep-alive",
	}
	if server.BearerToken != "" {
		configured.Auth = "bearer"
		configured.BearerToken = server.BearerToken
	}
	document := struct {
		MCPServers map[string]piServer `json:"mcpServers"`
	}{MCPServers: map[string]piServer{
		server.Name: configured,
	}}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Pi MCP configuration: %w", err)
	}
	return append(data, '\n'), nil
}

func piSettingsConfig(existing []byte) ([]byte, error) {
	document := map[string]any{}
	if len(bytes.TrimSpace(existing)) != 0 {
		if err := json.Unmarshal(existing, &document); err != nil {
			return nil, fmt.Errorf("decode retained Pi settings: %w", err)
		}
		if document == nil {
			return nil, errors.New("retained Pi settings must be a JSON object")
		}
	}
	document["defaultTools"] = []string{"read", "bash", "edit", "write", "grep", "find", "ls"}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Pi settings: %w", err)
	}
	return append(data, '\n'), nil
}

func piWebSearchConfig() ([]byte, error) {
	type enabled struct {
		Enabled bool `json:"enabled"`
	}
	type searchRouting struct {
		Providers  []string `json:"providers"`
		FallbackOn []string `json:"fallbackOn"`
	}
	document := struct {
		SearchRouting       searchRouting      `json:"searchRouting"`
		Workflow            string             `json:"workflow"`
		AutoOpenBrowser     bool               `json:"autoOpenBrowser"`
		AllowBrowserCookies bool               `json:"allowBrowserCookies"`
		Tools               map[string]enabled `json:"tools"`
		Commands            map[string]enabled `json:"commands"`
		Image               enabled            `json:"image"`
		GitHubClone         enabled            `json:"githubClone"`
		YouTube             enabled            `json:"youtube"`
		Video               enabled            `json:"video"`
		PDF                 enabled            `json:"pdf"`
	}{
		SearchRouting: searchRouting{
			Providers:  []string{"openai", "exa"},
			FallbackOn: []string{"transient", "quota", "network", "invalid-response"},
		},
		Workflow:            "none",
		AutoOpenBrowser:     false,
		AllowBrowserCookies: false,
		Tools: map[string]enabled{
			"webSearch":        {Enabled: true},
			"sourceCheck":      {Enabled: true},
			"fetchContent":     {Enabled: true},
			"getSearchContent": {Enabled: true},
		},
		Commands: map[string]enabled{
			"websearch":      {Enabled: false},
			"curator":        {Enabled: false},
			"search":         {Enabled: false},
			"google-account": {Enabled: false},
		},
		Image:       enabled{Enabled: true},
		GitHubClone: enabled{Enabled: true},
		YouTube:     enabled{Enabled: true},
		Video:       enabled{Enabled: true},
		PDF:         enabled{Enabled: true},
	}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Pi web search configuration: %w", err)
	}
	return append(data, '\n'), nil
}

func loadOrCreateSessionID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if !validUUID(id) {
			return "", fmt.Errorf("stored session ID %q is not a UUID", id)
		}
		return id, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read session ID %q: %w", path, err)
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	id := formatUUID(bytes)
	if err := writePrivateFile(path, []byte(id+"\n")); err != nil {
		return "", fmt.Errorf("write session ID %q: %w", path, err)
	}
	return id, nil
}

func formatUUID(value []byte) string {
	encoded := hex.EncodeToString(value)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	return err == nil && len(decoded) == 16
}

func writePrivateFile(path string, data []byte) (returnErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create parent directory %q: %w", dir, err)
	}
	temporary, err := os.CreateTemp(dir, ".agent-secret-*")
	if err != nil {
		return fmt.Errorf("create temporary file in %q: %w", dir, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary file %q: %w", temporaryPath, err))
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return errors.Join(fmt.Errorf("set permissions on temporary file %q: %w", temporaryPath, err), temporary.Close())
	}
	if _, err := temporary.Write(data); err != nil {
		return errors.Join(fmt.Errorf("write temporary file %q: %w", temporaryPath, err), temporary.Close())
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file %q: %w", temporaryPath, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace private file %q: %w", path, err)
	}
	return nil
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func lookupEnv(env []string, name string) (string, bool) {
	for index := len(env) - 1; index >= 0; index-- {
		key, value, ok := strings.Cut(env[index], "=")
		if ok && key == name {
			return value, true
		}
	}
	return "", false
}

func filterEnv(env []string, names ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && slices.Contains(names, key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func setEnv(env []string, name, value string) []string {
	env = filterEnv(env, name)
	return append(env, name+"="+value)
}
