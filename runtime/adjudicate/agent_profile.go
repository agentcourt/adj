package adjudicate

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/modelgateway"
	lawyerlaunch "github.com/agentcourt/adj/internal/lawyer"
	headless "github.com/agentcourt/adj/runtime/agent"
)

func participantStateDir(sessionDir, profile, role string) string {
	if strings.TrimSpace(sessionDir) == "" {
		return ""
	}
	return filepath.Join(sessionDir, profile, role)
}

func participantWorkDir(recordDir, role string) string {
	return filepath.Join(recordDir, "work", role)
}

const (
	defaultDockerCommand = "docker"
	defaultPodmanCommand = "podman"
	defaultOpenClawImage = "ghcr.io/openclaw/openclaw:latest"
	defaultOpenClawModel = "gpt-5.5"
	defaultPiImage       = "agentcourt-pi-sandbox"
	defaultPiMCPAdapter  = "/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter"
	defaultPiWebSearch   = "/opt/pi-extensions/pi-web-access/node_modules/pi-web-access/index.ts"
)

func resolvedLawyerLaunchProfile(profile ResolvedAgentProfile, timeout Duration) (lawyerlaunch.Profile, error) {
	resolvedTimeout := 15 * time.Minute
	if timeout != 0 {
		resolvedTimeout = time.Duration(timeout)
	}
	seconds := int(resolvedTimeout / time.Second)
	if resolvedTimeout <= 0 || time.Duration(seconds)*time.Second != resolvedTimeout {
		return lawyerlaunch.Profile{}, fmt.Errorf("lawyer timeout must be a positive whole number of seconds")
	}

	resume := profile.Resume
	launch := lawyerlaunch.Profile{Runner: lawyerlaunch.Runner(profile.Runner)}
	if profile.Runner == RunnerOpenClaw {
		model := strings.TrimSpace(profile.Model)
		if model == "" {
			model = defaultOpenClawModel
		}
		provider := strings.ToLower(strings.TrimSpace(string(profile.Provider)))
		if provider == "" {
			provider = string(ProviderOpenAI)
		}
		if !openClawModelSupported(provider, model) {
			return lawyerlaunch.Profile{}, fmt.Errorf("OpenClaw model %q does not match provider %q", model, provider)
		}
		auth := lawyerlaunch.OpenClawAuth{}
		switch profile.Authentication.Source {
		case AuthSubscription:
			if provider != string(ProviderOpenAI) {
				return lawyerlaunch.Profile{}, fmt.Errorf("OpenClaw provider %q does not support subscription authentication", provider)
			}
			auth.Mode = lawyerlaunch.OpenClawAuthCodex
			auth.CodexAuthPath = profile.Authentication.Path
		case AuthAPIKey:
			auth.Mode = lawyerlaunch.OpenClawAuthAPIKey
			auth.APIKeyEnv = profile.Authentication.EnvironmentVariable
		default:
			return lawyerlaunch.Profile{}, fmt.Errorf("unsupported OpenClaw authentication source %q", profile.Authentication.Source)
		}
		launch.OpenClaw = lawyerlaunch.OpenClawProfile{
			Command:                  defaultDockerCommand,
			Image:                    defaultOpenClawImage,
			Network:                  "host",
			Provider:                 provider,
			Model:                    model,
			Thinking:                 openClawReasoningEffort(profile.ReasoningEffort),
			AgentTimeoutSeconds:      seconds,
			LawyerTurnTimeoutSeconds: seconds,
			Auth:                     auth,
		}
		return launch, nil
	}

	auth := headless.Auth{}
	switch profile.Authentication.Source {
	case AuthSubscription:
		auth.Mode = headless.AuthSubscription
		auth.CredentialsFile = profile.Authentication.Path
	case AuthAPIKey:
		auth.Mode = headless.AuthAPIKey
		auth.APIKeyEnv = profile.Authentication.EnvironmentVariable
	default:
		return lawyerlaunch.Profile{}, fmt.Errorf("unsupported %s authentication source %q", profile.Runner, profile.Authentication.Source)
	}
	launch.Headless = headless.Profile{
		Runner:          headless.Runner(profile.Runner),
		Provider:        headless.Provider(profile.Provider),
		Command:         profile.Command,
		Model:           profile.Model,
		ReasoningEffort: profile.ReasoningEffort,
		MCPAdapter:      defaultPiMCPAdapter,
		Auth:            auth,
		Resume:          &resume,
	}
	if profile.Runner == RunnerPi {
		launch.Headless.WebSearchExtension = defaultPiWebSearch
	}
	return launch, nil
}

func openClawReasoningEffort(configured string) string {
	if effort := strings.TrimSpace(configured); effort != "" {
		return effort
	}
	return "low"
}

func openClawModelSupported(provider, model string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	switch provider {
	case "", string(ProviderOpenAI):
		return !strings.Contains(model, "/") || strings.HasPrefix(strings.ToLower(model), "openai/")
	case string(ProviderAnthropic):
		return strings.HasPrefix(strings.ToLower(model), "anthropic/") && len(model) > len("anthropic/")
	default:
		return false
	}
}

func participantEnvironmentFor(base []string, settings ResolvedSettings, profileNames ...string) ([]string, error) {
	environment := credentialFreeEnvironment(base, settings)
	for _, name := range profileNames {
		profile, ok := settings.AgentProfiles[name]
		if !ok {
			return nil, fmt.Errorf("agent profile %q is absent", name)
		}
		if profile.Authentication.Source != AuthAPIKey {
			continue
		}
		sourceName := strings.TrimSpace(profile.Authentication.EnvironmentVariable)
		value, ok := environmentValue(base, sourceName)
		if !ok || value == "" {
			return nil, fmt.Errorf("agent profile %q API-key source environment variable %s is absent or empty", name, sourceName)
		}
		environment = removeEnvironment(environment, sourceName)
		environment = append(environment, sourceName+"="+value)
	}
	return environment, nil
}

func credentialFreeEnvironment(base []string, settings ResolvedSettings) []string {
	remove := append(modelgateway.CredentialEnvironmentNames(), credentialEnvironmentNames(settings)...)
	return removeEnvironment(base, remove...)
}
