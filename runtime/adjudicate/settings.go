package adjudicate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/modelgateway"
)

type SettingsFile struct {
	SchemaVersion string                  `json:"schema_version"`
	Common        CommonSettings          `json:"common"`
	AgentProfiles map[string]AgentProfile `json:"agent_profiles,omitempty"`
	Procedures    ProcedureSettings       `json:"procedures"`
}

type CommonSettings struct {
	OutputRoot              string                        `json:"output_root,omitempty"`
	AgentStateRoot          string                        `json:"agent_state_root,omitempty"`
	LawyerProfile           string                        `json:"lawyer_profile,omitempty"`
	EvidenceStandard        string                        `json:"evidence_standard"`
	CouncilPool             string                        `json:"council_pool,omitempty"`
	CouncilAllowedEndpoints []string                      `json:"council_allowed_endpoints,omitempty"`
	CouncilMinEndpoints     int                           `json:"council_minimum_distinct_endpoints,omitempty"`
	CouncilSize             int                           `json:"council_size,omitempty"`
	RequiredVotes           int                           `json:"required_votes,omitempty"`
	DocumentLimits          DocumentLimits                `json:"document_limits,omitempty"`
	AllowAPIKey             *bool                         `json:"allow_api_key"`
	ProviderCredentials     map[string]CredentialMetadata `json:"provider_credentials"`
}

type DocumentLimits struct {
	Count   int   `json:"count,omitempty"`
	PerFile int64 `json:"per_file_bytes,omitempty"`
	Total   int64 `json:"total_bytes,omitempty"`
}

type ProcedureSettings struct {
	ARB    *ARBSettings    `json:"arb,omitempty"`
	ARBD   *ARBDSettings   `json:"arbd,omitempty"`
	ADC    *ADCSettings    `json:"adc,omitempty"`
	Simple *SimpleSettings `json:"simple,omitempty"`
	Quick  *QuickSettings  `json:"quick,omitempty"`
}

type ARBDSettings struct {
	CoreCommand         string          `json:"core_command,omitempty"`
	CoreWorkingDir      string          `json:"core_working_directory,omitempty"`
	MCPCommand          string          `json:"mcp_command,omitempty"`
	MCPWorkingDir       string          `json:"mcp_working_directory,omitempty"`
	MCPListenAddr       string          `json:"mcp_listen,omitempty"`
	MCPPublicBaseURL    string          `json:"mcp_public_base_url,omitempty"`
	AutoLawyers         string          `json:"auto_lawyers,omitempty"`
	PlaintiffProfile    string          `json:"plaintiff_profile"`
	DefendantProfile    string          `json:"defendant_profile"`
	JudgmentStandard    string          `json:"judgment_standard"`
	WebSearch           *bool           `json:"web_search,omitempty"`
	PromptDir           string          `json:"prompt_dir,omitempty"`
	PromptFiles         PromptFilePaths `json:"prompt_files,omitempty"`
	LauncherPromptDir   string          `json:"launcher_prompt_dir,omitempty"`
	LauncherPromptFiles PromptFilePaths `json:"launcher_prompt_files,omitempty"`
	Timeout             Duration        `json:"timeout,omitempty"`
}

type ARBSettings struct {
	CoreCommand         string          `json:"core_command,omitempty"`
	CoreWorkingDir      string          `json:"core_working_directory,omitempty"`
	MCPCommand          string          `json:"mcp_command,omitempty"`
	MCPWorkingDir       string          `json:"mcp_working_directory,omitempty"`
	MCPListenAddr       string          `json:"mcp_listen,omitempty"`
	MCPPublicBaseURL    string          `json:"mcp_public_base_url,omitempty"`
	AutoLawyers         string          `json:"auto_lawyers,omitempty"`
	PlaintiffProfile    string          `json:"plaintiff_profile"`
	DefendantProfile    string          `json:"defendant_profile"`
	WebSearch           *bool           `json:"web_search,omitempty"`
	PromptDir           string          `json:"prompt_dir,omitempty"`
	PromptFiles         PromptFilePaths `json:"prompt_files,omitempty"`
	LauncherPromptDir   string          `json:"launcher_prompt_dir,omitempty"`
	LauncherPromptFiles PromptFilePaths `json:"launcher_prompt_files,omitempty"`
	Timeout             Duration        `json:"timeout,omitempty"`
}

type ADCSettings struct {
	CoreCommand         string          `json:"core_command,omitempty"`
	CoreWorkingDir      string          `json:"core_working_directory,omitempty"`
	MCPCommand          string          `json:"mcp_command,omitempty"`
	MCPWorkingDir       string          `json:"mcp_working_directory,omitempty"`
	MCPListenAddr       string          `json:"mcp_listen,omitempty"`
	MCPPublicBaseURL    string          `json:"mcp_public_base_url,omitempty"`
	AutoLawyers         string          `json:"auto_lawyers,omitempty"`
	PlaintiffProfile    string          `json:"plaintiff_profile"`
	DefendantProfile    string          `json:"defendant_profile"`
	TrialMode           string          `json:"trial_mode,omitempty"`
	WebSearch           *bool           `json:"web_search,omitempty"`
	PromptDir           string          `json:"prompt_dir,omitempty"`
	PromptFiles         PromptFilePaths `json:"prompt_files,omitempty"`
	LauncherPromptDir   string          `json:"launcher_prompt_dir,omitempty"`
	LauncherPromptFiles PromptFilePaths `json:"launcher_prompt_files,omitempty"`
	Timeout             Duration        `json:"timeout,omitempty"`
}

type SimpleSettings struct {
	CoreCommand     string          `json:"core_command,omitempty"`
	CoreWorkingDir  string          `json:"core_working_directory,omitempty"`
	Model           string          `json:"model"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	MaxOutputTokens int64           `json:"max_output_tokens,omitempty"`
	MaxToolCalls    int64           `json:"max_tool_calls,omitempty"`
	WebSearch       *bool           `json:"web_search,omitempty"`
	PromptDir       string          `json:"prompt_dir,omitempty"`
	PromptFiles     PromptFilePaths `json:"prompt_files,omitempty"`
	Timeout         Duration        `json:"timeout,omitempty"`
}

type ResolvedSimpleSettings struct {
	CoreCommand     string          `json:"core_command"`
	CoreWorkingDir  string          `json:"core_working_directory,omitempty"`
	Model           string          `json:"model"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	MaxOutputTokens int64           `json:"max_output_tokens,omitempty"`
	MaxToolCalls    int64           `json:"max_tool_calls,omitempty"`
	WebSearch       bool            `json:"web_search"`
	PromptDir       string          `json:"prompt_dir,omitempty"`
	PromptFiles     PromptFilePaths `json:"prompt_files,omitempty"`
	Timeout         Duration        `json:"timeout,omitempty"`
}

type QuickSettings struct {
	CoreCommand         string          `json:"core_command,omitempty"`
	CoreWorkingDir      string          `json:"core_working_directory,omitempty"`
	MCPCommand          string          `json:"mcp_command,omitempty"`
	MCPWorkingDir       string          `json:"mcp_working_directory,omitempty"`
	MCPListenAddr       string          `json:"mcp_listen,omitempty"`
	MCPPublicBaseURL    string          `json:"mcp_public_base_url,omitempty"`
	AutoLawyers         string          `json:"auto_lawyers,omitempty"`
	PlaintiffProfile    string          `json:"plaintiff_profile"`
	DefendantProfile    string          `json:"defendant_profile"`
	WebSearch           *bool           `json:"web_search,omitempty"`
	PromptDir           string          `json:"prompt_dir,omitempty"`
	PromptFiles         PromptFilePaths `json:"prompt_files,omitempty"`
	LauncherPromptDir   string          `json:"launcher_prompt_dir,omitempty"`
	LauncherPromptFiles PromptFilePaths `json:"launcher_prompt_files,omitempty"`
	Timeout             Duration        `json:"timeout,omitempty"`
	ParallelCouncil     bool            `json:"parallel_council,omitempty"`
}

type ResolvedQuickSettings struct {
	CoreCommand         string          `json:"core_command"`
	CoreWorkingDir      string          `json:"core_working_directory,omitempty"`
	MCPCommand          string          `json:"mcp_command"`
	MCPWorkingDir       string          `json:"mcp_working_directory,omitempty"`
	MCPListenAddr       string          `json:"mcp_listen,omitempty"`
	MCPPublicBaseURL    string          `json:"mcp_public_base_url,omitempty"`
	AutoLawyers         string          `json:"auto_lawyers"`
	PlaintiffProfile    string          `json:"plaintiff_profile"`
	DefendantProfile    string          `json:"defendant_profile"`
	WebSearch           bool            `json:"web_search"`
	PromptDir           string          `json:"prompt_dir,omitempty"`
	PromptFiles         PromptFilePaths `json:"prompt_files,omitempty"`
	LauncherPromptDir   string          `json:"launcher_prompt_dir,omitempty"`
	LauncherPromptFiles PromptFilePaths `json:"launcher_prompt_files,omitempty"`
	Timeout             Duration        `json:"timeout,omitempty"`
	ParallelCouncil     bool            `json:"parallel_council"`
}

type PromptFilePaths map[string]string

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	if parsed <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

type AgentRunner string

const (
	RunnerOpenClaw AgentRunner = "openclaw"
	RunnerPi       AgentRunner = "pi"
	RunnerCodex    AgentRunner = "codex"
	RunnerClaude   AgentRunner = "claude"
)

type AgentProvider string

const (
	ProviderOpenAI     AgentProvider = "openai"
	ProviderAnthropic  AgentProvider = "anthropic"
	ProviderOpenRouter AgentProvider = "openrouter"
	ProviderGoogle     AgentProvider = "google"
)

type AgentProfile struct {
	Runner          AgentRunner    `json:"runner"`
	Provider        AgentProvider  `json:"provider,omitempty"`
	Model           string         `json:"model,omitempty"`
	Command         string         `json:"command,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	Resume          *bool          `json:"resume,omitempty"`
	Authentication  Authentication `json:"authentication,omitempty"`
}

type Authentication struct {
	Source              AuthenticationSource `json:"source,omitempty"`
	Path                string               `json:"path,omitempty"`
	EnvironmentVariable string               `json:"environment_variable,omitempty"`
}

type CredentialMetadata struct {
	Source              AuthenticationSource `json:"source"`
	EnvironmentVariable string               `json:"environment_variable,omitempty"`
}

type AuthenticationSource string

const (
	AuthSubscription AuthenticationSource = "subscription"
	AuthAPIKey       AuthenticationSource = "api_key"
)

type ResolvedSettings struct {
	SchemaVersion string                          `json:"schema_version"`
	Common        CommonSettings                  `json:"common"`
	AgentProfiles map[string]ResolvedAgentProfile `json:"agent_profiles,omitempty"`
	Procedure     ResolvedProcedureSettings       `json:"procedure"`
}

type ResolvedAgentProfile struct {
	Runner          AgentRunner    `json:"runner"`
	Provider        AgentProvider  `json:"provider"`
	Model           string         `json:"model,omitempty"`
	Command         string         `json:"command,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	Resume          bool           `json:"resume"`
	Authentication  Authentication `json:"authentication"`
}

type ResolvedProcedureSettings struct {
	Name   Procedure               `json:"name"`
	ARB    *ARBSettings            `json:"arb,omitempty"`
	ARBD   *ARBDSettings           `json:"arbd,omitempty"`
	ADC    *ADCSettings            `json:"adc,omitempty"`
	Simple *ResolvedSimpleSettings `json:"simple,omitempty"`
	Quick  *ResolvedQuickSettings  `json:"quick,omitempty"`
}

type resolvedProcedureCatalog struct {
	ARB    *ARBSettings
	ARBD   *ARBDSettings
	ADC    *ADCSettings
	Simple *ResolvedSimpleSettings
	Quick  *ResolvedQuickSettings
}

func LoadSettings(path string, selected Procedure) (ResolvedSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ResolvedSettings{}, fmt.Errorf("read settings %q: %w", path, err)
	}

	var settings SettingsFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		return ResolvedSettings{}, fmt.Errorf("decode settings %q: %w", path, err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ResolvedSettings{}, fmt.Errorf("decode settings %q: unexpected JSON value after settings object", path)
		}
		return ResolvedSettings{}, fmt.Errorf("decode settings %q after settings object: %w", path, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ResolvedSettings{}, fmt.Errorf("resolve settings path %q: %w", path, err)
	}
	home := ""
	if settings.needsHomeDirectory() {
		home, err = os.UserHomeDir()
		if err != nil {
			return ResolvedSettings{}, fmt.Errorf("resolve user home: %w", err)
		}
	}
	return settings.resolve(selected, filepath.Dir(absPath), home)
}

func (s SettingsFile) resolve(selected Procedure, baseDir, homeDir string) (ResolvedSettings, error) {
	if s.SchemaVersion != SettingsSchemaVersion {
		return ResolvedSettings{}, fmt.Errorf("settings schema_version must be %q", SettingsSchemaVersion)
	}
	if !selected.Valid() {
		return ResolvedSettings{}, fmt.Errorf("invalid procedure %q", selected)
	}
	common := s.Common
	if common.OutputRoot == "" {
		common.OutputRoot = "out"
	}
	common.OutputRoot = resolvePath(baseDir, homeDir, common.OutputRoot)
	if common.AgentStateRoot == "" {
		common.AgentStateRoot = filepath.Join(common.OutputRoot, ".agents")
	} else {
		common.AgentStateRoot = resolvePath(baseDir, homeDir, common.AgentStateRoot)
	}
	if common.DocumentLimits.Count <= 0 || common.DocumentLimits.PerFile <= 0 || common.DocumentLimits.Total <= 0 {
		return ResolvedSettings{}, fmt.Errorf("document limits must be positive")
	}
	if common.DocumentLimits.PerFile > common.DocumentLimits.Total {
		return ResolvedSettings{}, fmt.Errorf("document per-file limit exceeds total limit")
	}
	if err := resolveCommonAdjudicationSettings(&common, s.Procedures, baseDir, homeDir); err != nil {
		return ResolvedSettings{}, err
	}
	if err := validateCommonProviderCredentials(common); err != nil {
		return ResolvedSettings{}, err
	}

	profiles := make(map[string]ResolvedAgentProfile, len(s.AgentProfiles))
	for name, profile := range s.AgentProfiles {
		if strings.TrimSpace(name) != name || !singlePathComponent(name) {
			return ResolvedSettings{}, fmt.Errorf("agent profile name %q must be one path component without surrounding whitespace", name)
		}
		resolved, err := profile.resolve(baseDir, homeDir)
		if err != nil {
			return ResolvedSettings{}, fmt.Errorf("agent profile %q: %w", name, err)
		}
		profiles[name] = resolved
	}
	if common.LawyerProfile != "" {
		if _, ok := profiles[common.LawyerProfile]; !ok {
			return ResolvedSettings{}, fmt.Errorf("common lawyer profile %q is not defined", common.LawyerProfile)
		}
	}
	procedures := withCommonLawyerProfile(s.Procedures, common.LawyerProfile)
	if err := requireProcedureProviderCredentials(procedures, common.ProviderCredentials); err != nil {
		return ResolvedSettings{}, err
	}
	resolvedProcedures, err := resolveProcedures(procedures, profiles, baseDir, homeDir)
	if err != nil {
		return ResolvedSettings{}, err
	}
	procedure, err := selectProcedure(selected, resolvedProcedures)
	if err != nil {
		return ResolvedSettings{}, err
	}
	return ResolvedSettings{
		SchemaVersion: SettingsSchemaVersion,
		Common:        common,
		AgentProfiles: profiles,
		Procedure:     procedure,
	}, nil
}

func resolveCommonAdjudicationSettings(common *CommonSettings, procedures ProcedureSettings, baseDir, homeDir string) error {
	common.EvidenceStandard = strings.TrimSpace(common.EvidenceStandard)
	usesEvidenceStandard := procedures.ARB != nil || procedures.ADC != nil || procedures.Simple != nil || procedures.Quick != nil
	if usesEvidenceStandard {
		switch common.EvidenceStandard {
		case "preponderance_of_the_evidence", "clear_and_convincing":
		case "":
			return fmt.Errorf("common evidence_standard is required")
		default:
			return fmt.Errorf("common evidence_standard must be preponderance_of_the_evidence or clear_and_convincing")
		}
	}

	common.CouncilPool = strings.TrimSpace(common.CouncilPool)
	adcNeedsCouncil := procedures.ADC != nil && strings.ToLower(strings.TrimSpace(procedures.ADC.TrialMode)) != "bench"
	councilRequired := procedures.ARBD != nil || procedures.ARB != nil || procedures.Quick != nil || adcNeedsCouncil
	votesRequired := procedures.ARB != nil || procedures.Quick != nil || adcNeedsCouncil
	councilConfigured := common.CouncilPool != "" || common.CouncilSize != 0 || common.RequiredVotes != 0
	if !councilRequired && !councilConfigured {
		return nil
	}
	if common.CouncilPool == "" {
		return fmt.Errorf("common council_pool is required")
	}
	common.CouncilPool = resolvePath(baseDir, homeDir, common.CouncilPool)
	if common.CouncilSize <= 0 {
		return fmt.Errorf("common council_size must be positive")
	}
	common.CouncilAllowedEndpoints = normalizeEndpointList(common.CouncilAllowedEndpoints)
	if common.CouncilMinEndpoints < 0 {
		return fmt.Errorf("common council_minimum_distinct_endpoints must be between 0 and council_size")
	}
	completedCouncilMinimum := procedures.ARBD != nil || procedures.ARB != nil || procedures.Quick != nil
	if completedCouncilMinimum && common.CouncilMinEndpoints > common.CouncilSize {
		return fmt.Errorf("common council_minimum_distinct_endpoints must be between 0 and council_size")
	}
	if votesRequired && (common.RequiredVotes <= common.CouncilSize/2 || common.RequiredVotes > common.CouncilSize) {
		return fmt.Errorf("common required_votes must be a majority between %d and council_size", common.CouncilSize/2+1)
	}
	if adcNeedsCouncil {
		if common.CouncilSize < 6 || common.CouncilSize > 12 {
			return fmt.Errorf("ADC jury council_size must be between 6 and 12")
		}
		if common.RequiredVotes < 6 {
			return fmt.Errorf("ADC jury required_votes must be at least 6")
		}
	}
	return nil
}

func normalizeEndpointList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validateCommonProviderCredentials(common CommonSettings) error {
	if common.AllowAPIKey == nil {
		return fmt.Errorf("common allow_api_key is required")
	}
	if !*common.AllowAPIKey {
		return fmt.Errorf("current adjudication procedures require common allow_api_key")
	}
	if len(common.ProviderCredentials) == 0 {
		return fmt.Errorf("common provider_credentials is empty")
	}
	for name, credential := range common.ProviderCredentials {
		provider := strings.ToLower(strings.TrimSpace(name))
		if _, ok := modelgateway.CredentialEnvironmentName(provider); provider != name || !ok {
			return fmt.Errorf("common provider_credentials contains unsupported provider %q", name)
		}
		if credential.Source != AuthAPIKey {
			return fmt.Errorf("common provider credential %q must use api_key", provider)
		}
		if !validEnvironmentVariableName(credential.EnvironmentVariable) {
			return fmt.Errorf("common provider credential %q has invalid environment_variable %q", provider, credential.EnvironmentVariable)
		}
	}
	return nil
}

func requireProcedureProviderCredentials(procedures ProcedureSettings, credentials map[string]CredentialMetadata) error {
	require := func(procedure, provider string) error {
		if _, ok := credentials[provider]; !ok {
			return fmt.Errorf("procedure %s requires common provider credential %q", procedure, provider)
		}
		return nil
	}
	if procedures.ADC != nil {
		if err := require("adc", "openai"); err != nil {
			return err
		}
	}
	if procedures.Simple != nil {
		endpoint, _, ok := strings.Cut(strings.ToLower(strings.TrimSpace(procedures.Simple.Model)), "://")
		if ok {
			if err := require("simple", endpoint); err != nil {
				return err
			}
		}
	}
	return nil
}

func configuredProviderNames(credentials map[string]CredentialMetadata) []string {
	names := make([]string, 0, len(credentials))
	for name := range credentials {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validEnvironmentVariableName(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func (p AgentProfile) resolve(baseDir, homeDir string) (ResolvedAgentProfile, error) {
	if p.Runner != RunnerOpenClaw && p.Runner != RunnerPi && p.Runner != RunnerCodex && p.Runner != RunnerClaude {
		return ResolvedAgentProfile{}, fmt.Errorf("invalid runner %q", p.Runner)
	}
	resume := p.Runner != RunnerOpenClaw
	if p.Resume != nil {
		resume = *p.Resume
	}
	if p.Runner == RunnerOpenClaw && resume {
		return ResolvedAgentProfile{}, fmt.Errorf("openclaw does not support session resumption")
	}
	command := strings.TrimSpace(p.Command)
	if command != "" && (p.Runner == RunnerOpenClaw || p.Runner == RunnerPi) {
		return ResolvedAgentProfile{}, fmt.Errorf("%s does not accept a command", p.Runner)
	}
	command = resolveCommand(baseDir, homeDir, command, "")
	model := strings.TrimSpace(p.Model)
	provider, err := resolveAgentProvider(p.Runner, p.Provider, model)
	if err != nil {
		return ResolvedAgentProfile{}, err
	}
	reasoningEffort := strings.TrimSpace(p.ReasoningEffort)
	if reasoningEffort != "" && reasoningEffort != "low" && reasoningEffort != "medium" && reasoningEffort != "high" && reasoningEffort != "xhigh" {
		return ResolvedAgentProfile{}, fmt.Errorf("reasoning_effort must be low, medium, high, or xhigh, got %q", reasoningEffort)
	}
	auth := p.Authentication
	if auth.Source == "" {
		switch p.Runner {
		case RunnerOpenClaw:
			if provider != ProviderOpenAI {
				return ResolvedAgentProfile{}, fmt.Errorf("openclaw with provider %q requires explicit api_key authentication", provider)
			}
			auth.Source = AuthSubscription
		case RunnerCodex:
			auth.Source = AuthSubscription
		case RunnerClaude:
			if provider != ProviderAnthropic {
				return ResolvedAgentProfile{}, fmt.Errorf("claude with provider %q requires explicit api_key authentication", provider)
			}
			auth.Source = AuthSubscription
		case RunnerPi:
			if provider != ProviderOpenAI {
				return ResolvedAgentProfile{}, fmt.Errorf("pi with provider %q requires explicit api_key authentication", provider)
			}
			auth.Source = AuthSubscription
		}
	}
	switch auth.Source {
	case AuthSubscription:
		if (p.Runner == RunnerOpenClaw && provider != ProviderOpenAI) || (p.Runner == RunnerClaude && provider != ProviderAnthropic) || (p.Runner == RunnerPi && provider != ProviderOpenAI) {
			return ResolvedAgentProfile{}, fmt.Errorf("%s provider %q does not support subscription authentication", p.Runner, provider)
		}
		if auth.EnvironmentVariable != "" {
			return ResolvedAgentProfile{}, fmt.Errorf("subscription authentication cannot name an environment variable")
		}
		if auth.Path == "" {
			switch p.Runner {
			case RunnerOpenClaw, RunnerPi, RunnerCodex:
				auth.Path = filepath.Join(homeDir, ".codex", "auth.json")
			case RunnerClaude:
				auth.Path = filepath.Join(homeDir, ".claude", ".credentials.json")
			}
		} else {
			auth.Path = resolvePath(baseDir, homeDir, auth.Path)
		}
	case AuthAPIKey:
		if auth.Path != "" {
			return ResolvedAgentProfile{}, fmt.Errorf("api_key authentication cannot name a credential path")
		}
		if strings.TrimSpace(auth.EnvironmentVariable) == "" {
			return ResolvedAgentProfile{}, fmt.Errorf("api_key authentication requires environment_variable")
		}
	default:
		return ResolvedAgentProfile{}, fmt.Errorf("invalid authentication source %q", auth.Source)
	}

	resolved := ResolvedAgentProfile{
		Runner:          p.Runner,
		Provider:        provider,
		Model:           model,
		Command:         command,
		ReasoningEffort: reasoningEffort,
		Resume:          resume,
		Authentication:  auth,
	}
	return resolved, nil
}

func resolveAgentProvider(runner AgentRunner, configured AgentProvider, model string) (AgentProvider, error) {
	provider := AgentProvider(strings.ToLower(strings.TrimSpace(string(configured))))
	switch runner {
	case RunnerCodex:
		if provider == "" {
			provider = ProviderOpenAI
		}
		if provider != ProviderOpenAI {
			return "", fmt.Errorf("codex supports only provider %q, got %q", ProviderOpenAI, provider)
		}
	case RunnerClaude:
		if provider == "" {
			provider = ProviderAnthropic
		}
		if provider != ProviderAnthropic && provider != ProviderOpenRouter {
			return "", fmt.Errorf("claude provider must be %q or %q, got %q", ProviderAnthropic, ProviderOpenRouter, provider)
		}
		if provider == ProviderOpenRouter {
			modelProvider, modelID, ok := strings.Cut(model, "/")
			if !ok || strings.TrimSpace(modelProvider) == "" || strings.TrimSpace(modelID) == "" {
				return "", fmt.Errorf("claude with provider %q requires an OpenRouter model slug in provider/model form", provider)
			}
		}
	case RunnerOpenClaw:
		if provider == "" {
			provider = ProviderOpenAI
		}
		if provider != ProviderOpenAI && provider != ProviderAnthropic {
			return "", fmt.Errorf("openclaw provider must be %q or %q, got %q", ProviderOpenAI, ProviderAnthropic, provider)
		}
		if !openClawModelSupported(string(provider), model) {
			return "", fmt.Errorf("OpenClaw model %q does not match provider %q", model, provider)
		}
	case RunnerPi:
		modelProvider, modelID, ok := strings.Cut(model, "/")
		modelProvider = strings.ToLower(strings.TrimSpace(modelProvider))
		if !ok || modelProvider == "" || strings.TrimSpace(modelID) == "" {
			if provider == "" {
				return "", fmt.Errorf("Pi model must use provider/model form")
			}
			return "", fmt.Errorf("Pi model must include provider %q", provider)
		}
		if provider == "" {
			provider = AgentProvider(modelProvider)
		}
		if provider != ProviderOpenAI && provider != ProviderAnthropic && provider != ProviderOpenRouter && provider != ProviderGoogle {
			return "", fmt.Errorf("unsupported Pi provider %q", provider)
		}
		if modelProvider != string(provider) {
			return "", fmt.Errorf("Pi provider %q does not match model provider %q", provider, modelProvider)
		}
	}
	return provider, nil
}

func withCommonLawyerProfile(procedures ProcedureSettings, profile string) ProcedureSettings {
	resolved := procedures
	if procedures.ARB != nil {
		settings := *procedures.ARB
		if settings.PlaintiffProfile == "" {
			settings.PlaintiffProfile = profile
		}
		if settings.DefendantProfile == "" {
			settings.DefendantProfile = profile
		}
		resolved.ARB = &settings
	}
	if procedures.ARBD != nil {
		settings := *procedures.ARBD
		if settings.PlaintiffProfile == "" {
			settings.PlaintiffProfile = profile
		}
		if settings.DefendantProfile == "" {
			settings.DefendantProfile = profile
		}
		resolved.ARBD = &settings
	}
	if procedures.ADC != nil {
		settings := *procedures.ADC
		if settings.PlaintiffProfile == "" {
			settings.PlaintiffProfile = profile
		}
		if settings.DefendantProfile == "" {
			settings.DefendantProfile = profile
		}
		resolved.ADC = &settings
	}
	if procedures.Quick != nil {
		settings := *procedures.Quick
		if settings.PlaintiffProfile == "" {
			settings.PlaintiffProfile = profile
		}
		if settings.DefendantProfile == "" {
			settings.DefendantProfile = profile
		}
		resolved.Quick = &settings
	}
	if procedures.Simple != nil {
		settings := *procedures.Simple
		resolved.Simple = &settings
	}
	return resolved
}

func (s SettingsFile) needsHomeDirectory() bool {
	for _, path := range []string{s.Common.OutputRoot, s.Common.AgentStateRoot, s.Common.CouncilPool} {
		if usesHomeDirectory(path) {
			return true
		}
	}
	for _, profile := range s.AgentProfiles {
		if usesHomeDirectory(profile.Command) || usesHomeDirectory(profile.Authentication.Path) {
			return true
		}
		provider := AgentProvider(strings.ToLower(strings.TrimSpace(string(profile.Provider))))
		defaultSubscription := profile.Authentication.Source == "" && (profile.Runner == RunnerCodex || (profile.Runner == RunnerOpenClaw && (provider == "" || provider == ProviderOpenAI)) || (profile.Runner == RunnerClaude && (provider == "" || provider == ProviderAnthropic)) || (profile.Runner == RunnerPi && (provider == ProviderOpenAI || (provider == "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(profile.Model)), "openai/")))))
		if (profile.Authentication.Source == AuthSubscription || defaultSubscription) && profile.Authentication.Path == "" {
			return true
		}
	}
	if s.Procedures.ARB != nil && (usesHomeDirectory(s.Procedures.ARB.CoreCommand) || usesHomeDirectory(s.Procedures.ARB.CoreWorkingDir) || usesHomeDirectory(s.Procedures.ARB.MCPCommand) || usesHomeDirectory(s.Procedures.ARB.MCPWorkingDir) || usesHomeDirectory(s.Procedures.ARB.PromptDir) || promptFilesUseHome(s.Procedures.ARB.PromptFiles) || usesHomeDirectory(s.Procedures.ARB.LauncherPromptDir) || promptFilesUseHome(s.Procedures.ARB.LauncherPromptFiles)) {
		return true
	}
	if s.Procedures.ARBD != nil && (usesHomeDirectory(s.Procedures.ARBD.CoreCommand) || usesHomeDirectory(s.Procedures.ARBD.CoreWorkingDir) || usesHomeDirectory(s.Procedures.ARBD.MCPCommand) || usesHomeDirectory(s.Procedures.ARBD.MCPWorkingDir) || usesHomeDirectory(s.Procedures.ARBD.PromptDir) || promptFilesUseHome(s.Procedures.ARBD.PromptFiles) || usesHomeDirectory(s.Procedures.ARBD.LauncherPromptDir) || promptFilesUseHome(s.Procedures.ARBD.LauncherPromptFiles)) {
		return true
	}
	if s.Procedures.ADC != nil && (usesHomeDirectory(s.Procedures.ADC.CoreCommand) || usesHomeDirectory(s.Procedures.ADC.CoreWorkingDir) || usesHomeDirectory(s.Procedures.ADC.MCPCommand) || usesHomeDirectory(s.Procedures.ADC.MCPWorkingDir) || usesHomeDirectory(s.Procedures.ADC.PromptDir) || promptFilesUseHome(s.Procedures.ADC.PromptFiles) || usesHomeDirectory(s.Procedures.ADC.LauncherPromptDir) || promptFilesUseHome(s.Procedures.ADC.LauncherPromptFiles)) {
		return true
	}
	if s.Procedures.Simple != nil {
		paths := []string{
			s.Procedures.Simple.CoreCommand,
			s.Procedures.Simple.CoreWorkingDir,
			s.Procedures.Simple.PromptDir,
		}
		paths = append(paths, promptFilePathValues(s.Procedures.Simple.PromptFiles)...)
		for _, path := range paths {
			if usesHomeDirectory(path) {
				return true
			}
		}
	}
	if s.Procedures.Quick == nil {
		return false
	}
	paths := []string{
		s.Procedures.Quick.CoreCommand,
		s.Procedures.Quick.CoreWorkingDir,
		s.Procedures.Quick.MCPCommand,
		s.Procedures.Quick.MCPWorkingDir,
		s.Procedures.Quick.PromptDir,
		s.Procedures.Quick.LauncherPromptDir,
	}
	paths = append(paths, promptFilePathValues(s.Procedures.Quick.PromptFiles)...)
	paths = append(paths, promptFilePathValues(s.Procedures.Quick.LauncherPromptFiles)...)
	for _, path := range paths {
		if usesHomeDirectory(path) {
			return true
		}
	}
	return false
}

func usesHomeDirectory(path string) bool {
	path = strings.TrimSpace(path)
	return path == "~" || strings.HasPrefix(path, "~/")
}

func resolveProcedures(procedures ProcedureSettings, profiles map[string]ResolvedAgentProfile, baseDir, homeDir string) (resolvedProcedureCatalog, error) {
	resolved := resolvedProcedureCatalog{}
	if procedures.ARB != nil {
		settings := *procedures.ARB
		if err := resolveARB(&settings, profiles, baseDir, homeDir); err != nil {
			return resolvedProcedureCatalog{}, fmt.Errorf("procedure arb: %w", err)
		}
		resolved.ARB = &settings
	}
	if procedures.ARBD != nil {
		settings := *procedures.ARBD
		if err := resolveARBD(&settings, profiles, baseDir, homeDir); err != nil {
			return resolvedProcedureCatalog{}, fmt.Errorf("procedure arbd: %w", err)
		}
		resolved.ARBD = &settings
	}
	if procedures.ADC != nil {
		settings := *procedures.ADC
		if err := resolveADC(&settings, profiles, baseDir, homeDir); err != nil {
			return resolvedProcedureCatalog{}, fmt.Errorf("procedure adc: %w", err)
		}
		resolved.ADC = &settings
	}
	if procedures.Simple != nil {
		settings := *procedures.Simple
		simple, err := resolveSimple(&settings, profiles, baseDir, homeDir)
		if err != nil {
			return resolvedProcedureCatalog{}, fmt.Errorf("procedure simple: %w", err)
		}
		resolved.Simple = &simple
	}
	if procedures.Quick != nil {
		settings := *procedures.Quick
		quick, err := resolveQuick(&settings, profiles, baseDir, homeDir)
		if err != nil {
			return resolvedProcedureCatalog{}, fmt.Errorf("procedure quick: %w", err)
		}
		resolved.Quick = &quick
	}
	return resolved, nil
}

func selectProcedure(selected Procedure, procedures resolvedProcedureCatalog) (ResolvedProcedureSettings, error) {
	resolved := ResolvedProcedureSettings{Name: selected}
	switch selected {
	case ProcedureARB:
		if procedures.ARB == nil {
			return resolved, fmt.Errorf("settings do not configure selected procedure %q", selected)
		}
		resolved.ARB = procedures.ARB
	case ProcedureARBD:
		if procedures.ARBD == nil {
			return resolved, fmt.Errorf("settings do not configure selected procedure %q", selected)
		}
		resolved.ARBD = procedures.ARBD
	case ProcedureADC:
		if procedures.ADC == nil {
			return resolved, fmt.Errorf("settings do not configure selected procedure %q", selected)
		}
		resolved.ADC = procedures.ADC
	case ProcedureSimple:
		if procedures.Simple == nil {
			return resolved, fmt.Errorf("settings do not configure selected procedure %q", selected)
		}
		resolved.Simple = procedures.Simple
	case ProcedureQuick:
		if procedures.Quick == nil {
			return resolved, fmt.Errorf("settings do not configure selected procedure %q", selected)
		}
		resolved.Quick = procedures.Quick
	}
	return resolved, nil
}

func defaultEnabled(configured *bool) *bool {
	if configured != nil {
		return configured
	}
	enabled := true
	return &enabled
}

func resolveAutoLawyers(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "both", nil
	}
	switch value {
	case "both", "plaintiff", "defendant", "none":
		return value, nil
	default:
		return "", fmt.Errorf("auto_lawyers must be both, plaintiff, defendant, or none")
	}
}

func automaticLawyerEnabled(mode, role string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	return mode == "" || mode == "both" || mode == role
}

func automaticProfileNames(mode, plaintiff, defendant string) []string {
	names := make([]string, 0, 2)
	if automaticLawyerEnabled(mode, "plaintiff") {
		names = append(names, plaintiff)
	}
	if automaticLawyerEnabled(mode, "defendant") {
		names = append(names, defendant)
	}
	return names
}

func resolveARB(s *ARBSettings, profiles map[string]ResolvedAgentProfile, baseDir, homeDir string) error {
	s.WebSearch = defaultEnabled(s.WebSearch)
	s.CoreCommand = resolveCommand(baseDir, homeDir, s.CoreCommand, "aar")
	s.CoreWorkingDir = resolveOptionalPath(baseDir, homeDir, s.CoreWorkingDir)
	s.MCPCommand = resolveCommand(baseDir, homeDir, s.MCPCommand, "aar-mcp")
	s.MCPWorkingDir = resolveOptionalPath(baseDir, homeDir, s.MCPWorkingDir)
	s.MCPListenAddr = strings.TrimSpace(s.MCPListenAddr)
	s.MCPPublicBaseURL = strings.TrimRight(strings.TrimSpace(s.MCPPublicBaseURL), "/")
	var err error
	s.AutoLawyers, err = resolveAutoLawyers(s.AutoLawyers)
	if err != nil {
		return err
	}
	if s.MCPWorkingDir == "" {
		s.MCPWorkingDir = s.CoreWorkingDir
	}
	s.PromptDir = resolveOptionalPath(baseDir, homeDir, s.PromptDir)
	s.LauncherPromptDir = resolveOptionalPath(baseDir, homeDir, s.LauncherPromptDir)
	if err := s.PromptFiles.resolve(baseDir, homeDir); err != nil {
		return fmt.Errorf("prompt files: %w", err)
	}
	if _, _, err := splitFormalPromptFiles("arb", s.PromptFiles); err != nil {
		return err
	}
	if err := s.LauncherPromptFiles.resolve(baseDir, homeDir); err != nil {
		return fmt.Errorf("launcher prompt files: %w", err)
	}
	if err := requireProfiles(profiles, automaticProfileNames(s.AutoLawyers, s.PlaintiffProfile, s.DefendantProfile)...); err != nil {
		return err
	}
	return nil
}

func resolveARBD(s *ARBDSettings, profiles map[string]ResolvedAgentProfile, baseDir, homeDir string) error {
	s.WebSearch = defaultEnabled(s.WebSearch)
	s.CoreCommand = resolveCommand(baseDir, homeDir, s.CoreCommand, "aard")
	s.CoreWorkingDir = resolveOptionalPath(baseDir, homeDir, s.CoreWorkingDir)
	s.MCPCommand = resolveCommand(baseDir, homeDir, s.MCPCommand, "aard-mcp")
	s.MCPWorkingDir = resolveOptionalPath(baseDir, homeDir, s.MCPWorkingDir)
	s.MCPListenAddr = strings.TrimSpace(s.MCPListenAddr)
	s.MCPPublicBaseURL = strings.TrimRight(strings.TrimSpace(s.MCPPublicBaseURL), "/")
	var err error
	s.AutoLawyers, err = resolveAutoLawyers(s.AutoLawyers)
	if err != nil {
		return err
	}
	if s.MCPWorkingDir == "" {
		s.MCPWorkingDir = s.CoreWorkingDir
	}
	s.PromptDir = resolveOptionalPath(baseDir, homeDir, s.PromptDir)
	s.LauncherPromptDir = resolveOptionalPath(baseDir, homeDir, s.LauncherPromptDir)
	if err := s.PromptFiles.resolve(baseDir, homeDir); err != nil {
		return fmt.Errorf("prompt files: %w", err)
	}
	if _, _, err := splitFormalPromptFiles("arbd", s.PromptFiles); err != nil {
		return err
	}
	if err := s.LauncherPromptFiles.resolve(baseDir, homeDir); err != nil {
		return fmt.Errorf("launcher prompt files: %w", err)
	}
	s.JudgmentStandard = strings.TrimSpace(s.JudgmentStandard)
	if s.JudgmentStandard == "" {
		return fmt.Errorf("judgment_standard is required")
	}
	if err := requireProfiles(profiles, automaticProfileNames(s.AutoLawyers, s.PlaintiffProfile, s.DefendantProfile)...); err != nil {
		return err
	}
	return nil
}

func resolveADC(s *ADCSettings, profiles map[string]ResolvedAgentProfile, baseDir, homeDir string) error {
	s.WebSearch = defaultEnabled(s.WebSearch)
	s.CoreCommand = resolveCommand(baseDir, homeDir, s.CoreCommand, "adc")
	s.CoreWorkingDir = resolveOptionalPath(baseDir, homeDir, s.CoreWorkingDir)
	s.MCPCommand = resolveCommand(baseDir, homeDir, s.MCPCommand, "adc-mcp")
	s.MCPWorkingDir = resolveOptionalPath(baseDir, homeDir, s.MCPWorkingDir)
	s.MCPListenAddr = strings.TrimSpace(s.MCPListenAddr)
	s.MCPPublicBaseURL = strings.TrimRight(strings.TrimSpace(s.MCPPublicBaseURL), "/")
	var err error
	s.AutoLawyers, err = resolveAutoLawyers(s.AutoLawyers)
	if err != nil {
		return err
	}
	if s.MCPWorkingDir == "" {
		s.MCPWorkingDir = s.CoreWorkingDir
	}
	s.PromptDir = resolveOptionalPath(baseDir, homeDir, s.PromptDir)
	s.LauncherPromptDir = resolveOptionalPath(baseDir, homeDir, s.LauncherPromptDir)
	if err := s.PromptFiles.resolve(baseDir, homeDir); err != nil {
		return fmt.Errorf("prompt files: %w", err)
	}
	if _, _, err := splitFormalPromptFiles("adc", s.PromptFiles); err != nil {
		return err
	}
	if err := s.LauncherPromptFiles.resolve(baseDir, homeDir); err != nil {
		return fmt.Errorf("launcher prompt files: %w", err)
	}
	if err := requireProfiles(profiles, automaticProfileNames(s.AutoLawyers, s.PlaintiffProfile, s.DefendantProfile)...); err != nil {
		return err
	}
	s.TrialMode = strings.ToLower(strings.TrimSpace(s.TrialMode))
	if s.TrialMode == "" {
		s.TrialMode = "auto"
	}
	if s.TrialMode != "auto" && s.TrialMode != "jury" && s.TrialMode != "bench" {
		return fmt.Errorf("trial_mode must be auto, jury, or bench")
	}
	return nil
}

func resolveSimple(s *SimpleSettings, profiles map[string]ResolvedAgentProfile, baseDir, homeDir string) (ResolvedSimpleSettings, error) {
	s.CoreCommand = resolveCommand(baseDir, homeDir, s.CoreCommand, "simple")
	s.CoreWorkingDir = resolveOptionalPath(baseDir, homeDir, s.CoreWorkingDir)
	s.PromptDir = resolveOptionalPath(baseDir, homeDir, s.PromptDir)
	if err := s.PromptFiles.resolve(baseDir, homeDir); err != nil {
		return ResolvedSimpleSettings{}, err
	}
	s.Model = strings.TrimSpace(s.Model)
	if s.Model == "" {
		return ResolvedSimpleSettings{}, fmt.Errorf("model is empty")
	}
	if strings.Contains(s.Model, "?") {
		return ResolvedSimpleSettings{}, fmt.Errorf("model must not include a query")
	}
	if strings.Contains(s.Model, "#") {
		return ResolvedSimpleSettings{}, fmt.Errorf("model must not include a fragment")
	}
	endpoint, _, ok := strings.Cut(s.Model, "://")
	if !ok || strings.TrimSpace(endpoint) == "" {
		return ResolvedSimpleSettings{}, fmt.Errorf("model must be endpoint://model")
	}
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "openai", "openrouter":
	default:
		return ResolvedSimpleSettings{}, fmt.Errorf("unsupported direct-provider endpoint %q", endpoint)
	}
	s.ReasoningEffort = strings.TrimSpace(s.ReasoningEffort)
	switch s.ReasoningEffort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh", "max":
	default:
		return ResolvedSimpleSettings{}, fmt.Errorf("reasoning_effort must be none, minimal, low, medium, high, xhigh, or max, got %q", s.ReasoningEffort)
	}
	if s.MaxOutputTokens < 0 {
		return ResolvedSimpleSettings{}, fmt.Errorf("max_output_tokens must not be negative")
	}
	if s.MaxToolCalls < 0 {
		return ResolvedSimpleSettings{}, fmt.Errorf("max_tool_calls must not be negative")
	}
	webSearch := true
	if s.WebSearch != nil {
		webSearch = *s.WebSearch
	}
	return ResolvedSimpleSettings{
		CoreCommand:     s.CoreCommand,
		CoreWorkingDir:  s.CoreWorkingDir,
		Model:           s.Model,
		ReasoningEffort: s.ReasoningEffort,
		MaxOutputTokens: s.MaxOutputTokens,
		MaxToolCalls:    s.MaxToolCalls,
		WebSearch:       webSearch,
		PromptDir:       s.PromptDir,
		PromptFiles:     s.PromptFiles,
		Timeout:         s.Timeout,
	}, nil
}

func resolveQuick(s *QuickSettings, profiles map[string]ResolvedAgentProfile, baseDir, homeDir string) (ResolvedQuickSettings, error) {
	s.CoreCommand = resolveCommand(baseDir, homeDir, s.CoreCommand, "quick")
	s.CoreWorkingDir = resolveOptionalPath(baseDir, homeDir, s.CoreWorkingDir)
	s.MCPCommand = resolveCommand(baseDir, homeDir, s.MCPCommand, "quick-mcp")
	s.MCPWorkingDir = resolveOptionalPath(baseDir, homeDir, s.MCPWorkingDir)
	s.MCPListenAddr = strings.TrimSpace(s.MCPListenAddr)
	s.MCPPublicBaseURL = strings.TrimRight(strings.TrimSpace(s.MCPPublicBaseURL), "/")
	var err error
	s.AutoLawyers, err = resolveAutoLawyers(s.AutoLawyers)
	if err != nil {
		return ResolvedQuickSettings{}, err
	}
	if s.MCPWorkingDir == "" {
		s.MCPWorkingDir = s.CoreWorkingDir
	}
	s.PromptDir = resolveOptionalPath(baseDir, homeDir, s.PromptDir)
	s.LauncherPromptDir = resolveOptionalPath(baseDir, homeDir, s.LauncherPromptDir)
	if err := s.PromptFiles.resolve(baseDir, homeDir); err != nil {
		return ResolvedQuickSettings{}, fmt.Errorf("prompt files: %w", err)
	}
	if _, _, err := splitQuickPromptFiles(s.PromptFiles); err != nil {
		return ResolvedQuickSettings{}, err
	}
	if err := s.LauncherPromptFiles.resolve(baseDir, homeDir); err != nil {
		return ResolvedQuickSettings{}, fmt.Errorf("launcher prompt files: %w", err)
	}
	if err := requireProfiles(profiles, automaticProfileNames(s.AutoLawyers, s.PlaintiffProfile, s.DefendantProfile)...); err != nil {
		return ResolvedQuickSettings{}, err
	}
	webSearch := true
	if s.WebSearch != nil {
		webSearch = *s.WebSearch
	}
	return ResolvedQuickSettings{
		CoreCommand:         s.CoreCommand,
		CoreWorkingDir:      s.CoreWorkingDir,
		MCPCommand:          s.MCPCommand,
		MCPWorkingDir:       s.MCPWorkingDir,
		MCPListenAddr:       s.MCPListenAddr,
		MCPPublicBaseURL:    s.MCPPublicBaseURL,
		AutoLawyers:         s.AutoLawyers,
		PlaintiffProfile:    s.PlaintiffProfile,
		DefendantProfile:    s.DefendantProfile,
		WebSearch:           webSearch,
		PromptDir:           s.PromptDir,
		PromptFiles:         s.PromptFiles,
		LauncherPromptDir:   s.LauncherPromptDir,
		LauncherPromptFiles: s.LauncherPromptFiles,
		Timeout:             s.Timeout,
		ParallelCouncil:     s.ParallelCouncil,
	}, nil
}

func promptFilePathValues(paths PromptFilePaths) []string {
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		values = append(values, path)
	}
	return values
}

func promptFilesUseHome(paths PromptFilePaths) bool {
	for _, path := range paths {
		if usesHomeDirectory(path) {
			return true
		}
	}
	return false
}

func (p PromptFilePaths) resolve(baseDir, homeDir string) error {
	resolved := make(PromptFilePaths, len(p))
	for rawID, rawPath := range p {
		id := strings.TrimSpace(rawID)
		path := strings.TrimSpace(rawPath)
		if id == "" {
			return fmt.Errorf("prompt ID is empty")
		}
		if path == "" {
			return fmt.Errorf("prompt %q path is empty", id)
		}
		if _, exists := resolved[id]; exists {
			return fmt.Errorf("prompt ID %q is repeated after trimming", id)
		}
		resolved[id] = resolveOptionalPath(baseDir, homeDir, path)
	}
	for id := range p {
		delete(p, id)
	}
	for id, path := range resolved {
		p[id] = path
	}
	return nil
}

func requireProfiles(profiles map[string]ResolvedAgentProfile, names ...string) error {
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("agent profile reference is empty")
		}
		if profiles != nil {
			if _, ok := profiles[name]; !ok {
				return fmt.Errorf("agent profile %q is not defined", name)
			}
		}
	}
	return nil
}

func resolveOptionalPath(baseDir, homeDir, path string) string {
	if path == "" {
		return ""
	}
	return resolvePath(baseDir, homeDir, path)
}

func resolveCommand(baseDir, homeDir, command, defaultCommand string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return defaultCommand
	}
	if filepath.IsAbs(command) || strings.ContainsAny(command, `/\\`) || command == "~" || strings.HasPrefix(command, "~/") {
		return resolvePath(baseDir, homeDir, command)
	}
	return command
}

func resolvePath(baseDir, homeDir, path string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(baseDir, path)
}
