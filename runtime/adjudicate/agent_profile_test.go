package adjudicate

import "testing"

func TestOpenClawProfileSupportsAnthropicProvider(t *testing.T) {
	profile, err := resolvedLawyerLaunchProfile(ResolvedAgentProfile{
		Runner:   RunnerOpenClaw,
		Provider: ProviderAnthropic,
		Model:    "anthropic/claude-opus-4-8",
		Authentication: Authentication{
			Source:              AuthAPIKey,
			EnvironmentVariable: "ANTHROPIC_API_KEY",
		},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if profile.OpenClaw.Provider != "anthropic" || profile.OpenClaw.Model != "anthropic/claude-opus-4-8" {
		t.Fatalf("OpenClaw profile = %#v", profile.OpenClaw)
	}
}

func TestClaudeProfileSupportsOpenRouterProvider(t *testing.T) {
	profile, err := resolvedLawyerLaunchProfile(ResolvedAgentProfile{
		Runner:   RunnerClaude,
		Provider: ProviderOpenRouter,
		Model:    "openai/gpt-5.6-sol",
		Authentication: Authentication{
			Source:              AuthAPIKey,
			EnvironmentVariable: "OPENROUTER_API_KEY",
		},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Headless.Provider != "openrouter" || profile.Headless.Model != "openai/gpt-5.6-sol" {
		t.Fatalf("Claude profile = %#v", profile.Headless)
	}
}

func TestCoreAndParticipantCredentialsRemainSeparate(t *testing.T) {
	base := []string{
		"DIRECT_OPENROUTER_KEY=core-key",
		"DIRECT_OPENAI_KEY=other-core-key",
		"OPENROUTER_API_KEY=participant-key",
		"OPENAI_API_KEY=unselected-key",
		"GEMINI_API_KEY=undeclared-key",
		"OTHER_ROLE_KEY=other-role-key",
	}
	settings := ResolvedSettings{
		Common: CommonSettings{ProviderCredentials: map[string]CredentialMetadata{
			"openrouter": {Source: AuthAPIKey, EnvironmentVariable: "DIRECT_OPENROUTER_KEY"},
			"openai":     {Source: AuthAPIKey, EnvironmentVariable: "DIRECT_OPENAI_KEY"},
		}},
		AgentProfiles: map[string]ResolvedAgentProfile{
			"lawyer": {
				Runner: RunnerPi,
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OPENROUTER_API_KEY",
				},
			},
			"other": {
				Runner: RunnerClaude,
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OTHER_ROLE_KEY",
				},
			},
		},
	}

	core, err := coreProviderEnvironmentFor(settings, base, "openrouter")
	if err != nil {
		t.Fatal(err)
	}
	participants, err := participantEnvironmentFor(base, settings, "lawyer")
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := environmentValue(core, "OPENROUTER_API_KEY"); !ok || value != "core-key" {
		t.Fatalf("core OPENROUTER_API_KEY = %q, present = %t", value, ok)
	}
	for _, name := range []string{"DIRECT_OPENROUTER_KEY", "DIRECT_OPENAI_KEY", "OTHER_ROLE_KEY"} {
		if _, ok := environmentValue(core, name); ok {
			t.Fatalf("core environment contains credential source %s", name)
		}
	}
	if value, ok := environmentValue(participants, "OPENROUTER_API_KEY"); !ok || value != "participant-key" {
		t.Fatalf("participant OPENROUTER_API_KEY = %q, present = %t", value, ok)
	}
	for _, name := range []string{"DIRECT_OPENROUTER_KEY", "DIRECT_OPENAI_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "OTHER_ROLE_KEY"} {
		if _, ok := environmentValue(participants, name); ok {
			t.Fatalf("participant environment contains %s", name)
		}
	}
}
