package agent

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareSubscriptionDefaults(t *testing.T) {
	t.Run("codex", func(t *testing.T) {
		home := t.TempDir()
		writeTestFile(t, filepath.Join(home, ".codex", "auth.json"), `{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`)
		assignment := testAssignment(t)
		assignment.MCP.BearerToken = "court-secret"
		webSearch := true
		assignment.WebSearch = &webSearch

		invocation, err := Prepare(Profile{Runner: RunnerCodex, ReasoningEffort: "xhigh"}, assignment, []string{
			"HOME=" + home,
			"PATH=/usr/bin",
			"OPENAI_API_KEY=billable",
			"CODEX_API_KEY=also-billable",
		})
		if err != nil {
			t.Fatal(err)
		}
		if invocation.Command != "codex" {
			t.Fatalf("Command = %q, want codex", invocation.Command)
		}
		if want := []string{"--strict-config", "exec", "--json", "--skip-git-repo-check", assignment.Prompt}; !reflect.DeepEqual(invocation.Args, want) {
			t.Fatalf("Args = %#v, want %#v", invocation.Args, want)
		}
		if invocation.Dir != assignment.WorkDir {
			t.Fatalf("Dir = %q, want %q", invocation.Dir, assignment.WorkDir)
		}
		assertEnvAbsent(t, invocation.Env, "OPENAI_API_KEY", "CODEX_API_KEY")
		if got := envValue(t, invocation.Env, mcpBearerTokenEnv); got != "court-secret" {
			t.Fatalf("MCP bearer token = %q", got)
		}
		codexHome := envValue(t, invocation.Env, "CODEX_HOME")
		if want := filepath.Join(assignment.StateDir, "codex", "home"); codexHome != want {
			t.Fatalf("CODEX_HOME = %q, want %q", codexHome, want)
		}
		config := readTestFile(t, filepath.Join(codexHome, "config.toml"))
		for _, required := range []string{
			`forced_login_method = "chatgpt"`,
			`sandbox_mode = "workspace-write"`,
			"[sandbox_workspace_write]\nnetwork_access = true",
			`model_reasoning_effort = "xhigh"`,
			`web_search = "live"`,
			`[features]`,
			`apps = false`,
			`plugins = false`,
			`remote_plugin = false`,
			`recommended_plugins = false`,
			`plugin_sharing = false`,
			`workspace_dependencies = false`,
			`[mcp_servers."court"]`,
			`url = "https://court.example.test/mcp"`,
			`required = true`,
			`bearer_token_env_var = "` + mcpBearerTokenEnv + `"`,
		} {
			if !strings.Contains(config, required) {
				t.Errorf("config does not contain %q:\n%s", required, config)
			}
		}
		if strings.Contains(config, "court-secret") {
			t.Fatal("Codex configuration contains the MCP bearer token")
		}
		if got := readTestFile(t, filepath.Join(codexHome, "auth.json")); !strings.Contains(got, `"auth_mode":"chatgpt"`) {
			t.Fatalf("staged credentials = %q", got)
		}
	})

	t.Run("claude", func(t *testing.T) {
		home := t.TempDir()
		writeTestFile(t, filepath.Join(home, ".claude", ".credentials.json"), `{"claudeAiOauth":{"accessToken":"token"}}`)
		assignment := testAssignment(t)
		assignment.MCP.BearerToken = "court-secret"
		webSearch := true
		assignment.WebSearch = &webSearch

		invocation, err := Prepare(Profile{Runner: RunnerClaude, ReasoningEffort: "xhigh"}, assignment, []string{
			"HOME=" + home,
			"PATH=/usr/bin",
			"ANTHROPIC_API_KEY=billable",
			"ANTHROPIC_AUTH_TOKEN=also-billable",
		})
		if err != nil {
			t.Fatal(err)
		}
		if invocation.Command != "claude" {
			t.Fatalf("Command = %q, want claude", invocation.Command)
		}
		assertEnvAbsent(t, invocation.Env, "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN")
		claudeHome := envValue(t, invocation.Env, "HOME")
		if want := filepath.Join(assignment.StateDir, "claude", "home"); claudeHome != want {
			t.Fatalf("HOME = %q, want %q", claudeHome, want)
		}
		if invocation.SessionID == "" {
			t.Fatal("SessionID is empty")
		}
		assertArgsContainSequence(t, invocation.Args, "--session-id", invocation.SessionID)
		assertArgsContainSequence(t, invocation.Args, "--effort", "xhigh")
		assertArgsContainSequence(t, invocation.Args, "--mcp-config", filepath.Join(assignment.StateDir, "claude", "mcp.json"))
		assertArgsContainSequence(t, invocation.Args, "--tools", "default")
		assertArgsContainSequence(t, invocation.Args, "--allowedTools", "mcp__court__*,Bash,Edit,Glob,Grep,Read,Write,WebFetch,NotebookEdit,Agent,WebSearch")
		if slicesContain(invocation.Args, "--disallowedTools") {
			t.Fatalf("Claude search-enabled arguments deny a built-in tool: %#v", invocation.Args)
		}
		if slicesContain(invocation.Args, "--bare") || slicesContain(invocation.Args, "--safe-mode") || slicesContain(invocation.Args, "--no-chrome") || slicesContain(invocation.Args, "--no-session-persistence") {
			t.Fatalf("Claude subscription arguments disable OAuth or session persistence: %#v", invocation.Args)
		}
		assertClaudeIsolationSettings(t, invocation.Args, assignment)
		if invocation.Args[len(invocation.Args)-1] != assignment.Prompt {
			t.Fatalf("last argument = %q, want prompt", invocation.Args[len(invocation.Args)-1])
		}
		config := readTestFile(t, filepath.Join(assignment.StateDir, "claude", "mcp.json"))
		for _, required := range []string{`"type":"http"`, `"url":"https://court.example.test/mcp"`, `"Authorization":"Bearer court-secret"`} {
			if !strings.Contains(config, required) {
				t.Errorf("MCP config does not contain %q: %s", required, config)
			}
		}
		if got := readTestFile(t, filepath.Join(claudeHome, ".claude", ".credentials.json")); !strings.Contains(got, "claudeAiOauth") {
			t.Fatalf("staged credentials = %q", got)
		}
	})
}

func TestPrepareClaudeAPIKeyRetainsStandardToolsAndDisablesPersistence(t *testing.T) {
	assignment := testAssignment(t)
	assignment.MCP.BearerToken = "court-secret"
	webSearch := true
	assignment.WebSearch = &webSearch
	resume := false
	invocation, err := Prepare(Profile{
		Runner: RunnerClaude,
		Auth:   Auth{Mode: AuthAPIKey, APIKeyEnv: "SELECTED_CLAUDE_KEY"},
		Resume: &resume,
	}, assignment, []string{
		"HOME=" + t.TempDir(),
		"SELECTED_CLAUDE_KEY=selected",
		"ANTHROPIC_API_KEY=unselected",
		"ANTHROPIC_AUTH_TOKEN=unselected-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if slicesContain(invocation.Args, "--bare") || slicesContain(invocation.Args, "--no-chrome") {
		t.Fatalf("Claude API-key arguments suppress standard tools: %#v", invocation.Args)
	}
	if slicesContain(invocation.Args, "--safe-mode") {
		t.Fatalf("Claude API-key arguments disable explicit MCP configuration: %#v", invocation.Args)
	}
	assertArgsContainSequence(t, invocation.Args, "--mcp-config", filepath.Join(assignment.StateDir, "claude", "mcp.json"), "--strict-mcp-config")
	assertArgsContainSequence(t, invocation.Args, "--no-session-persistence")
	assertArgsContainSequence(t, invocation.Args, "--tools", "default")
	assertArgsContainSequence(t, invocation.Args, "--allowedTools", "mcp__court__*,Bash,Edit,Glob,Grep,Read,Write,WebFetch,NotebookEdit,Agent,WebSearch")
	assertClaudeIsolationSettings(t, invocation.Args, assignment)
	if got := envValue(t, invocation.Env, "ANTHROPIC_API_KEY"); got != "selected" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want selected", got)
	}
	assertEnvAbsent(t, invocation.Env, "SELECTED_CLAUDE_KEY", "ANTHROPIC_AUTH_TOKEN")
}

func TestPrepareClaudeOpenRouterGateway(t *testing.T) {
	assignment := testAssignment(t)
	invocation, err := Prepare(Profile{
		Runner:   RunnerClaude,
		Provider: ProviderOpenRouter,
		Model:    "openai/gpt-5.6-sol",
		Auth:     Auth{Mode: AuthAPIKey, APIKeyEnv: "SELECTED_OPENROUTER_KEY"},
	}, assignment, []string{
		"HOME=" + t.TempDir(),
		"SELECTED_OPENROUTER_KEY=selected",
		"ANTHROPIC_API_KEY=unselected-anthropic",
		"ANTHROPIC_AUTH_TOKEN=unselected-token",
		"ANTHROPIC_BASE_URL=https://unselected.example.test",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1",
		"CLAUDE_CODE_SUBAGENT_MODEL=unselected-subagent",
		"ANTHROPIC_DEFAULT_FABLE_MODEL=unselected-fable",
		"ANTHROPIC_DEFAULT_OPUS_MODEL=unselected-opus",
		"ANTHROPIC_DEFAULT_SONNET_MODEL=unselected-sonnet",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL=unselected-haiku",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := envValue(t, invocation.Env, "ANTHROPIC_BASE_URL"); got != "https://openrouter.ai/api" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q", got)
	}
	if got := envValue(t, invocation.Env, "ANTHROPIC_AUTH_TOKEN"); got != "selected" {
		t.Fatalf("ANTHROPIC_AUTH_TOKEN = %q", got)
	}
	if got := envValue(t, invocation.Env, "ANTHROPIC_API_KEY"); got != "" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want empty", got)
	}
	assertEnvAbsent(t, invocation.Env, "SELECTED_OPENROUTER_KEY")
	assertEnvAbsent(t, invocation.Env, "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY")
	for _, name := range []string{
		"CLAUDE_CODE_SUBAGENT_MODEL",
		"ANTHROPIC_DEFAULT_FABLE_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	} {
		if got := envValue(t, invocation.Env, name); got != "openai/gpt-5.6-sol" {
			t.Fatalf("%s = %q", name, got)
		}
	}
	assertArgsContainSequence(t, invocation.Args, "--model", "openai/gpt-5.6-sol")
}

func TestValidateProfileProviderConsistency(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{
			name: "Claude OpenRouter requires API key",
			profile: Profile{
				Runner:   RunnerClaude,
				Provider: ProviderOpenRouter,
				Model:    "openai/gpt-5.6-sol",
			},
			want: "requires API-key authentication",
		},
		{
			name: "Pi provider conflicts with model",
			profile: Profile{
				Runner:     RunnerPi,
				Provider:   ProviderAnthropic,
				Model:      "openrouter/anthropic/claude-opus-4-8",
				MCPAdapter: "/opt/pi-mcp-adapter",
				Auth:       Auth{Mode: AuthAPIKey, APIKeyEnv: "SELECTED_KEY"},
			},
			want: `Pi provider "anthropic" does not match model provider "openrouter"`,
		},
		{
			name: "Pi subscription requires OpenAI",
			profile: Profile{
				Runner:     RunnerPi,
				Provider:   ProviderAnthropic,
				Model:      "anthropic/claude-opus-4-8",
				MCPAdapter: "/opt/pi-mcp-adapter",
				Auth:       Auth{Mode: AuthSubscription},
			},
			want: `supports only provider "openai"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateProfile(test.profile, []string{"HOME=" + t.TempDir(), "SELECTED_KEY=selected"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateProfile() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestPrepareDisablesWebSearch(t *testing.T) {
	webSearch := false
	t.Run("codex", func(t *testing.T) {
		home := t.TempDir()
		writeTestFile(t, filepath.Join(home, ".codex", "auth.json"), `{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`)
		assignment := testAssignment(t)
		assignment.WebSearch = &webSearch
		invocation, err := Prepare(Profile{Runner: RunnerCodex}, assignment, []string{"HOME=" + home})
		if err != nil {
			t.Fatal(err)
		}
		config := readTestFile(t, filepath.Join(envValue(t, invocation.Env, "CODEX_HOME"), "config.toml"))
		if !strings.Contains(config, `web_search = "disabled"`) {
			t.Fatalf("Codex configuration does not disable web search:\n%s", config)
		}
	})

	t.Run("claude", func(t *testing.T) {
		home := t.TempDir()
		writeTestFile(t, filepath.Join(home, ".claude", ".credentials.json"), `{"claudeAiOauth":{"accessToken":"token"}}`)
		assignment := testAssignment(t)
		assignment.WebSearch = &webSearch
		invocation, err := Prepare(Profile{Runner: RunnerClaude}, assignment, []string{"HOME=" + home})
		if err != nil {
			t.Fatal(err)
		}
		assertArgsContainSequence(t, invocation.Args, "--tools", "default")
		assertArgsContainSequence(t, invocation.Args, "--allowedTools", "mcp__court__*,Bash,Edit,Glob,Grep,Read,Write,WebFetch,NotebookEdit,Agent")
		assertArgsContainSequence(t, invocation.Args, "--disallowedTools", "WebSearch")
	})

	t.Run("pi", func(t *testing.T) {
		assignment := testAssignment(t)
		assignment.WebSearch = &webSearch
		invocation, err := Prepare(Profile{
			Runner:             RunnerPi,
			Model:              "openrouter/anthropic/claude-sonnet-4",
			MCPAdapter:         "/opt/pi-mcp-adapter",
			WebSearchExtension: "/opt/pi-web-access/index.ts",
			Auth:               Auth{Mode: AuthAPIKey, APIKeyEnv: "SELECTED_PI_KEY"},
		}, assignment, []string{"HOME=" + t.TempDir(), "SELECTED_PI_KEY=selected"})
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"--no-builtin-tools", "--tools", "--no-extensions", "--no-skills"} {
			if slicesContain(invocation.Args, forbidden) {
				t.Fatalf("Pi search-disabled arguments contain %s: %#v", forbidden, invocation.Args)
			}
		}
		if slicesContain(invocation.Args, "/opt/pi-web-access/index.ts") {
			t.Fatalf("Pi arguments load web search: %#v", invocation.Args)
		}
		configDir := envValue(t, invocation.Env, "PI_CODING_AGENT_DIR")
		if _, err := os.Stat(filepath.Join(configDir, "web-search.json")); !os.IsNotExist(err) {
			t.Fatalf("Pi search configuration exists or stat failed: %v", err)
		}
	})
}

func TestPreparePiUsesExplicitAPIKeyAndIsolatedState(t *testing.T) {
	assignment := testAssignment(t)
	assignment.MCP.BearerToken = "court-secret"
	webSearch := true
	assignment.WebSearch = &webSearch
	invocation, err := Prepare(Profile{
		Runner:             RunnerPi,
		Model:              "openrouter/anthropic/claude-sonnet-4",
		ReasoningEffort:    "high",
		MCPAdapter:         "/opt/pi-mcp-adapter",
		WebSearchExtension: "/opt/pi-web-access/index.ts",
		Auth: Auth{
			Mode:      AuthAPIKey,
			APIKeyEnv: "SELECTED_PI_KEY",
		},
	}, assignment, []string{
		"HOME=" + t.TempDir(),
		"PATH=/usr/bin",
		"SELECTED_PI_KEY=selected",
		"OPENROUTER_API_KEY=unselected",
		"OPENAI_API_KEY=unselected-openai",
		"ANTHROPIC_API_KEY=unselected-anthropic",
		"EXA_API_KEY=unselected-exa",
		"PI_ALLOW_BROWSER_COOKIES=1",
		"FEYNMAN_ALLOW_BROWSER_COOKIES=1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Command != "pi" {
		t.Fatalf("Command = %q, want pi", invocation.Command)
	}
	if invocation.Resumed {
		t.Fatal("first Pi invocation resumed")
	}
	assertArgsContainSequence(t, invocation.Args, "--provider", "openrouter")
	assertArgsContainSequence(t, invocation.Args, "--model", "anthropic/claude-sonnet-4")
	assertArgsContainSequence(t, invocation.Args, "--thinking", "high")
	for _, forbidden := range []string{"--no-builtin-tools", "--tools", "--no-extensions", "--no-skills"} {
		if slicesContain(invocation.Args, forbidden) {
			t.Fatalf("Pi arguments contain %s: %#v", forbidden, invocation.Args)
		}
	}
	assertArgsContainSequence(t, invocation.Args, "--extension", "/opt/pi-mcp-adapter")
	assertArgsContainSequence(t, invocation.Args, "--extension", "/opt/pi-web-access/index.ts")
	if slicesContain(invocation.Args, "--continue") {
		t.Fatalf("first Pi invocation arguments contain --continue: %#v", invocation.Args)
	}
	if got := envValue(t, invocation.Env, "OPENROUTER_API_KEY"); got != "selected" {
		t.Fatalf("OPENROUTER_API_KEY = %q, want selected", got)
	}
	assertEnvAbsent(t, invocation.Env, "SELECTED_PI_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "EXA_API_KEY", "PI_ALLOW_BROWSER_COOKIES", "FEYNMAN_ALLOW_BROWSER_COOKIES")
	configDir := envValue(t, invocation.Env, "PI_CODING_AGENT_DIR")
	sessionDir := envValue(t, invocation.Env, "PI_CODING_AGENT_SESSION_DIR")
	if configDir != filepath.Join(assignment.StateDir, "pi", "config") {
		t.Fatalf("PI_CODING_AGENT_DIR = %q", configDir)
	}
	assertArgsContainSequence(t, invocation.Args, "--session-dir", sessionDir)
	settings := readTestFile(t, filepath.Join(configDir, "settings.json"))
	if settings != `{"defaultTools":["read","bash","edit","write","grep","find","ls"]}`+"\n" {
		t.Fatalf("Pi settings = %s", settings)
	}
	mcp := readTestFile(t, filepath.Join(configDir, "mcp.json"))
	for _, want := range []string{`"court"`, `"auth":"bearer"`, `"bearerToken":"court-secret"`} {
		if !strings.Contains(mcp, want) {
			t.Fatalf("Pi MCP configuration does not contain %q: %s", want, mcp)
		}
	}
	webConfig := readTestFile(t, filepath.Join(configDir, "web-search.json"))
	for _, want := range []string{
		`"searchRouting":{"providers":["openai","exa"],"fallbackOn":["transient","quota","network","invalid-response"]}`,
		`"workflow":"none"`,
		`"autoOpenBrowser":false`,
		`"allowBrowserCookies":false`,
		`"webSearch":{"enabled":true}`,
		`"sourceCheck":{"enabled":true}`,
		`"fetchContent":{"enabled":true}`,
		`"getSearchContent":{"enabled":true}`,
		`"image":{"enabled":true}`,
		`"githubClone":{"enabled":true}`,
		`"youtube":{"enabled":true}`,
		`"video":{"enabled":true}`,
		`"pdf":{"enabled":true}`,
	} {
		if !strings.Contains(webConfig, want) {
			t.Fatalf("Pi web search configuration does not contain %q: %s", want, webConfig)
		}
	}
	if err := invocation.MarkSuccessful(); err != nil {
		t.Fatal(err)
	}
	if err := invocation.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("Pi MCP secret remains or stat failed: %v", err)
	}
	writeTestFile(t, filepath.Join(configDir, "settings.json"), `{"packages":["npm:installed-analysis-tool"],"defaultTools":["read"]}`)
	retainedProgram := filepath.Join(invocation.StateDir, "home", "bin", "analysis-tool")
	retainedOutput := filepath.Join(assignment.WorkDir, "analysis", "result.txt")
	writeTestFile(t, retainedProgram, "program")
	writeTestFile(t, retainedOutput, "result")

	assignment.Prompt = "second opportunity"
	second, err := Prepare(Profile{
		Runner:             RunnerPi,
		Model:              "openrouter/anthropic/claude-sonnet-4",
		MCPAdapter:         "/opt/pi-mcp-adapter",
		WebSearchExtension: "/opt/pi-web-access/index.ts",
		Auth:               Auth{Mode: AuthAPIKey, APIKeyEnv: "SELECTED_PI_KEY"},
	}, assignment, []string{"HOME=" + t.TempDir(), "SELECTED_PI_KEY=selected"})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Resumed || !slicesContain(second.Args, "--continue") {
		t.Fatalf("second Pi invocation did not continue: resumed=%v args=%#v", second.Resumed, second.Args)
	}
	retainedSettings := readTestFile(t, filepath.Join(configDir, "settings.json"))
	if !strings.Contains(retainedSettings, `"packages":["npm:installed-analysis-tool"]`) || !strings.Contains(retainedSettings, `"defaultTools":["read","bash","edit","write","grep","find","ls"]`) {
		t.Fatalf("retained Pi settings = %s", retainedSettings)
	}
	for _, path := range []string{retainedProgram, retainedOutput} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("inspect retained Pi path %q: %v", path, err)
		}
	}
}

func TestPreparePiSubscriptionUsesCodexCredentials(t *testing.T) {
	home := t.TempDir()
	sourcePath := filepath.Join(home, ".codex", "auth.json")
	sourceContents := testPiCodexCredentials(t, 4_102_444_800)
	writeTestFile(t, sourcePath, sourceContents)
	assignment := testAssignment(t)
	webSearch := true
	assignment.WebSearch = &webSearch
	invocation, err := Prepare(Profile{
		Runner:             RunnerPi,
		Provider:           ProviderOpenAI,
		Model:              "openai/gpt-5.6-sol",
		ReasoningEffort:    "xhigh",
		MCPAdapter:         "/opt/pi-mcp-adapter",
		WebSearchExtension: "/opt/pi-web-access/index.ts",
		Auth:               Auth{Mode: AuthSubscription},
	}, assignment, []string{
		"HOME=" + home,
		"OPENAI_API_KEY=unselected-openai",
		"OPENROUTER_API_KEY=unselected-openrouter",
		"EXA_API_KEY=unselected-exa",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertArgsContainSequence(t, invocation.Args, "--provider", "openai-codex")
	assertArgsContainSequence(t, invocation.Args, "--model", "gpt-5.6-sol")
	assertArgsContainSequence(t, invocation.Args, "--thinking", "xhigh")
	assertArgsContainSequence(t, invocation.Args, "--extension", "/opt/pi-web-access/index.ts")
	assertEnvAbsent(t, invocation.Env, "OPENAI_API_KEY", "OPENROUTER_API_KEY", "EXA_API_KEY")
	if target, source := invocation.CredentialEnvironmentNames(); target != "" || source != "" {
		t.Fatalf("credential environment names = %q, %q", target, source)
	}

	authPath := filepath.Join(envValue(t, invocation.Env, "PI_CODING_AGENT_DIR"), "auth.json")
	var document map[string]struct {
		Type      string `json:"type"`
		Access    string `json:"access"`
		Refresh   string `json:"refresh"`
		Expires   int64  `json:"expires"`
		AccountID string `json:"accountId"`
	}
	if err := json.Unmarshal([]byte(readTestFile(t, authPath)), &document); err != nil {
		t.Fatal(err)
	}
	credentials, ok := document["openai-codex"]
	if !ok {
		t.Fatalf("Pi credentials lack openai-codex entry: %#v", document)
	}
	if credentials.Type != "oauth" || credentials.Access != testPiAccessToken(t, 4_102_444_800) || credentials.Refresh != "test-refresh" || credentials.Expires != 4_102_444_800_000 || credentials.AccountID != "test-account" {
		t.Fatalf("Pi OAuth credential fields = %#v", credentials)
	}
	info, err := os.Stat(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("Pi credential mode = %o", info.Mode().Perm())
	}
	if got := readTestFile(t, sourcePath); got != sourceContents {
		t.Fatal("source Codex credentials changed")
	}
	if err := invocation.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("Pi subscription credentials remain or stat failed: %v", err)
	}
}

func TestValidatePiSubscriptionCredentials(t *testing.T) {
	validToken := testPiAccessToken(t, 4_102_444_800)
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "malformed JSON", data: `{`, want: "decode JSON"},
		{name: "API authentication", data: `{"auth_mode":"apikey","tokens":{}}`, want: "subscription authentication requires"},
		{name: "missing access token", data: `{"auth_mode":"chatgpt","tokens":{"refresh_token":"refresh","account_id":"account"}}`, want: "tokens.access_token is required"},
		{name: "missing refresh token", data: `{"auth_mode":"chatgpt","tokens":{"access_token":"` + validToken + `","account_id":"account"}}`, want: "tokens.refresh_token is required"},
		{name: "missing account ID", data: `{"auth_mode":"chatgpt","tokens":{"access_token":"` + validToken + `","refresh_token":"refresh"}}`, want: "tokens.account_id is required"},
		{name: "access token is not JWT", data: `{"auth_mode":"chatgpt","tokens":{"access_token":"token","refresh_token":"refresh","account_id":"account"}}`, want: "must be a JWT"},
		{name: "missing expiration", data: `{"auth_mode":"chatgpt","tokens":{"access_token":"e30.e30.signature","refresh_token":"refresh","account_id":"account"}}`, want: "exp claim"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "auth.json")
			writeTestFile(t, path, test.data)
			err := ValidateProfile(Profile{
				Runner:     RunnerPi,
				Provider:   ProviderOpenAI,
				Model:      "openai/gpt-5.6-sol",
				MCPAdapter: "/opt/pi-mcp-adapter",
				Auth:       Auth{Mode: AuthSubscription, CredentialsFile: path},
			}, []string{"HOME=" + t.TempDir()})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateProfile() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestPiMCPConfigAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		want        string
	}{
		{
			name:        "bearer token",
			bearerToken: "court-secret",
			want:        `{"mcpServers":{"court":{"url":"https://court.example.test/mcp","lifecycle":"keep-alive","auth":"bearer","bearerToken":"court-secret"}}}` + "\n",
		},
		{
			name: "no token",
			want: `{"mcpServers":{"court":{"url":"https://court.example.test/mcp","lifecycle":"keep-alive"}}}` + "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := piMCPConfig(MCPServer{
				Name:        "court",
				URL:         "https://court.example.test/mcp",
				BearerToken: test.bearerToken,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := string(config); got != test.want {
				t.Fatalf("Pi MCP configuration = %s, want %s", got, test.want)
			}
		})
	}
}

func TestPreparePiRequiresExplicitSettings(t *testing.T) {
	assignment := testAssignment(t)
	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{name: "authentication", profile: Profile{Runner: RunnerPi, Provider: ProviderOpenAI, Model: "openai/model", MCPAdapter: "adapter"}, want: "read credentials"},
		{name: "model", profile: Profile{Runner: RunnerPi, Auth: Auth{Mode: AuthAPIKey, APIKeyEnv: "PI_KEY"}, MCPAdapter: "adapter"}, want: "model must include its provider"},
		{name: "provider", profile: Profile{Runner: RunnerPi, Model: "unknown/model", Auth: Auth{Mode: AuthAPIKey, APIKeyEnv: "PI_KEY"}, MCPAdapter: "adapter"}, want: "unsupported Pi provider"},
		{name: "adapter", profile: Profile{Runner: RunnerPi, Model: "openrouter/model", Auth: Auth{Mode: AuthAPIKey, APIKeyEnv: "PI_KEY"}}, want: "adapter path is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Prepare(test.profile, assignment, []string{"HOME=" + t.TempDir(), "PI_KEY=selected"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestPrepareAPIKeyRequiresExplicitSource(t *testing.T) {
	for _, runner := range []Runner{RunnerCodex, RunnerClaude} {
		t.Run(string(runner), func(t *testing.T) {
			assignment := testAssignment(t)
			_, err := Prepare(Profile{Runner: runner, Auth: Auth{Mode: AuthAPIKey}}, assignment, []string{"HOME=" + t.TempDir()})
			if err == nil || !strings.Contains(err.Error(), "explicit source environment-variable name") {
				t.Fatalf("error = %v", err)
			}

			_, err = Prepare(Profile{Runner: runner, Auth: Auth{Mode: AuthAPIKey, APIKeyEnv: "BILLABLE_KEY"}}, assignment, []string{"HOME=" + t.TempDir()})
			if err == nil || !strings.Contains(err.Error(), "absent or empty") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateProfileRejectsMixedAuthenticationSettings(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{
			name: "subscription with key source",
			profile: Profile{
				Runner: RunnerCodex,
				Auth:   Auth{Mode: AuthSubscription, APIKeyEnv: "OPENAI_API_KEY"},
			},
			want: "subscription authentication cannot name",
		},
		{
			name: "API key with credential file",
			profile: Profile{
				Runner: RunnerClaude,
				Auth:   Auth{Mode: AuthAPIKey, CredentialsFile: "/tmp/credentials", APIKeyEnv: "ANTHROPIC_API_KEY"},
			},
			want: "API-key authentication cannot name",
		},
		{
			name:    "invalid reasoning effort",
			profile: Profile{Runner: RunnerCodex, ReasoningEffort: "maximum"},
			want:    "reasoning effort must be low, medium, high, or xhigh",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateProfile(test.profile, []string{"HOME=" + t.TempDir(), "ANTHROPIC_API_KEY=key"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestPrepareAppliesCodexAndClaudeModels(t *testing.T) {
	tests := []struct {
		runner   Runner
		relative string
		data     string
	}{
		{RunnerCodex, filepath.Join(".codex", "auth.json"), `{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`},
		{RunnerClaude, filepath.Join(".claude", ".credentials.json"), `{"claudeAiOauth":{"accessToken":"token"}}`},
	}
	for _, test := range tests {
		t.Run(string(test.runner), func(t *testing.T) {
			home := t.TempDir()
			writeTestFile(t, filepath.Join(home, test.relative), test.data)
			invocation, err := Prepare(Profile{Runner: test.runner, Model: "selected-model"}, testAssignment(t), []string{"HOME=" + home})
			if err != nil {
				t.Fatal(err)
			}
			assertArgsContainSequence(t, invocation.Args, "--model", "selected-model")
		})
	}
}

func TestSubscriptionDoesNotFallBackToAPIKey(t *testing.T) {
	for _, test := range []struct {
		runner Runner
		key    string
	}{
		{RunnerCodex, "OPENAI_API_KEY=billable"},
		{RunnerClaude, "ANTHROPIC_API_KEY=billable"},
	} {
		t.Run(string(test.runner), func(t *testing.T) {
			_, err := Prepare(Profile{Runner: test.runner}, testAssignment(t), []string{
				"HOME=" + t.TempDir(),
				test.key,
			})
			if err == nil || !strings.Contains(err.Error(), "read credentials") {
				t.Fatalf("error = %v", err)
			}
		})
	}
	t.Run("pi", func(t *testing.T) {
		_, err := Prepare(Profile{
			Runner:     RunnerPi,
			Provider:   ProviderOpenAI,
			Model:      "openai/gpt-5.6-sol",
			MCPAdapter: "/opt/pi-mcp-adapter",
		}, testAssignment(t), []string{
			"HOME=" + t.TempDir(),
			"OPENAI_API_KEY=billable",
		})
		if err == nil || !strings.Contains(err.Error(), "read credentials") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestPrepareAPIKeyUsesOnlyExplicitSource(t *testing.T) {
	for _, test := range []struct {
		runner       Runner
		canonicalKey string
	}{
		{RunnerCodex, "OPENAI_API_KEY"},
		{RunnerClaude, "ANTHROPIC_API_KEY"},
	} {
		t.Run(string(test.runner), func(t *testing.T) {
			assignment := testAssignment(t)
			invocation, err := Prepare(Profile{
				Runner: test.runner,
				Auth:   Auth{Mode: AuthAPIKey, APIKeyEnv: "BILLABLE_KEY"},
			}, assignment, []string{
				"HOME=" + t.TempDir(),
				"BILLABLE_KEY=selected",
				"OPENAI_API_KEY=unselected-openai",
				"CODEX_API_KEY=unselected-codex",
				"ANTHROPIC_API_KEY=unselected-anthropic",
				"ANTHROPIC_AUTH_TOKEN=unselected-token",
			})
			if err != nil {
				t.Fatal(err)
			}
			assertEnvAbsent(t, invocation.Env, "BILLABLE_KEY")
			if test.runner == RunnerCodex {
				assertEnvAbsent(t, invocation.Env, "OPENAI_API_KEY", "CODEX_API_KEY")
				config := readTestFile(t, filepath.Join(assignment.StateDir, "codex", "home", "config.toml"))
				if !strings.Contains(config, `forced_login_method = "api"`) {
					t.Fatalf("Codex configuration does not force API login:\n%s", config)
				}
				credentials := readTestFile(t, filepath.Join(assignment.StateDir, "codex", "home", "auth.json"))
				if !strings.Contains(credentials, `"auth_mode":"apikey"`) || !strings.Contains(credentials, `"OPENAI_API_KEY":"selected"`) {
					t.Fatalf("Codex API-key credentials have the wrong shape: %s", credentials)
				}
			} else {
				if got := envValue(t, invocation.Env, test.canonicalKey); got != "selected" {
					t.Fatalf("%s = %q, want selected", test.canonicalKey, got)
				}
				assertEnvAbsent(t, invocation.Env, "ANTHROPIC_AUTH_TOKEN")
			}
		})
	}
}

func TestPrepareRejectsInvalidSubscriptionCredentials(t *testing.T) {
	tests := []struct {
		name    string
		runner  Runner
		relPath string
		data    string
		want    string
	}{
		{"codex malformed JSON", RunnerCodex, filepath.Join(".codex", "auth.json"), `{`, "decode JSON"},
		{"codex API authentication", RunnerCodex, filepath.Join(".codex", "auth.json"), `{"auth_mode":"api","tokens":{"access_token":"token"}}`, "subscription authentication requires"},
		{"codex missing tokens", RunnerCodex, filepath.Join(".codex", "auth.json"), `{"auth_mode":"chatgpt"}`, "tokens object is required"},
		{"claude malformed JSON", RunnerClaude, filepath.Join(".claude", ".credentials.json"), `{`, "decode JSON"},
		{"claude missing OAuth", RunnerClaude, filepath.Join(".claude", ".credentials.json"), `{"apiKey":"key"}`, "claudeAiOauth object is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeTestFile(t, filepath.Join(home, test.relPath), test.data)
			_, err := Prepare(Profile{Runner: test.runner}, testAssignment(t), []string{"HOME=" + home})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestStageCredentialsRefreshesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.json")
	destination := filepath.Join(dir, "staged", "auth.json")
	writeTestFile(t, source, `{"auth_mode":"chatgpt","tokens":{"access_token":"current"}}`)
	writeTestFile(t, destination, `{"auth_mode":"chatgpt","tokens":{"access_token":"stale"}}`)

	if err := stageCredentials(source, destination, validateCodexCredentials); err != nil {
		t.Fatal(err)
	}
	got := readTestFile(t, destination)
	if !strings.Contains(got, `"access_token":"current"`) || strings.Contains(got, `"access_token":"stale"`) {
		t.Fatalf("staged credentials were not refreshed: %s", got)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("staged credential mode = %o", info.Mode().Perm())
	}
}

func TestCodexResumesOnlyAfterMarkedSuccess(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".codex", "auth.json"), `{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`)
	assignment := testAssignment(t)
	profile := Profile{Runner: RunnerCodex}

	first, err := Prepare(profile, assignment, []string{"HOME=" + home})
	if err != nil {
		t.Fatal(err)
	}
	if first.Resumed {
		t.Fatal("first invocation resumed")
	}
	usagePath := filepath.Join(t.TempDir(), "codex.stdout")
	writeTestFile(t, usagePath, `{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":20,"reasoning_output_tokens":5}}`)
	usage, err := first.FinishUsage(RunnerCodex, usagePath)
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 100 || usage.TotalTokens != 120 {
		t.Fatalf("usage = %#v", usage)
	}
	if err := first.MarkSuccessful(); err != nil {
		t.Fatal(err)
	}
	persistentSessionFile := filepath.Join(first.StateDir, "sessions", "case-session.jsonl")
	writeTestFile(t, persistentSessionFile, "session")
	if err := first.Cleanup(); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{
		filepath.Join(first.StateDir, "home", "auth.json"),
		filepath.Join(first.StateDir, "home", "config.toml"),
	} {
		if _, err := os.Stat(secret); !os.IsNotExist(err) {
			t.Fatalf("secret %q remains or stat failed: %v", secret, err)
		}
	}
	for _, retained := range []string{filepath.Join(first.StateDir, "successful-invocation"), persistentSessionFile} {
		if _, err := os.Stat(retained); err != nil {
			t.Fatalf("retained state %q: %v", retained, err)
		}
	}

	assignment.Prompt = "second opportunity"
	second, err := Prepare(profile, assignment, []string{"HOME=" + home})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--strict-config", "exec", "resume", "--json", "--skip-git-repo-check", "--last", "--all", assignment.Prompt}
	if !reflect.DeepEqual(second.Args, want) {
		t.Fatalf("Args = %#v, want %#v", second.Args, want)
	}
	if !second.Resumed {
		t.Fatal("second invocation did not resume")
	}
}

func TestCodexResumeRequiresUsageCheckpoint(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".codex", "auth.json"), `{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`)
	assignment := testAssignment(t)
	profile := Profile{Runner: RunnerCodex}

	first, err := Prepare(profile, assignment, []string{"HOME=" + home})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.MarkSuccessful(); err != nil {
		t.Fatal(err)
	}
	if err := first.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(profile, assignment, []string{"HOME=" + home}); err == nil || !strings.Contains(err.Error(), "read Codex usage checkpoint") {
		t.Fatalf("missing-checkpoint error = %v", err)
	}
	if err := first.MarkUnsuccessful(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(first.StateDir, "successful-invocation")); !os.IsNotExist(err) {
		t.Fatalf("success marker remains or stat failed: %v", err)
	}
}

func TestClaudeRetainsSessionAndCleanupRemovesSecrets(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude", ".credentials.json"), `{"claudeAiOauth":{"accessToken":"token"}}`)
	assignment := testAssignment(t)
	assignment.MCP.BearerToken = "court-secret"
	profile := Profile{Runner: RunnerClaude}

	first, err := Prepare(profile, assignment, []string{
		"HOME=" + home,
		"ANTHROPIC_API_KEY=billable",
		"ANTHROPIC_AUTH_TOKEN=billable-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertArgsContainSequence(t, first.Args, "--session-id", first.SessionID)
	if slicesContain(first.Args, "--no-session-persistence") {
		t.Fatalf("initial resumable Claude arguments disable persistence: %#v", first.Args)
	}
	if err := first.MarkSuccessful(); err != nil {
		t.Fatal(err)
	}
	persistentSessionFile := filepath.Join(first.StateDir, "home", ".claude", "projects", "case-session.jsonl")
	writeTestFile(t, persistentSessionFile, "session")
	if err := first.Cleanup(); err != nil {
		t.Fatal(err)
	}
	assertEnvAbsent(t, first.Env, "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN")
	for _, secret := range []string{
		filepath.Join(first.StateDir, "home", ".claude", ".credentials.json"),
		filepath.Join(first.StateDir, "mcp.json"),
	} {
		if _, err := os.Stat(secret); !os.IsNotExist(err) {
			t.Fatalf("secret %q remains or stat failed: %v", secret, err)
		}
	}
	for _, retained := range []string{
		filepath.Join(first.StateDir, "session-id"),
		filepath.Join(first.StateDir, "successful-invocation"),
		persistentSessionFile,
	} {
		if _, err := os.Stat(retained); err != nil {
			t.Fatalf("retained state %q: %v", retained, err)
		}
	}

	assignment.Prompt = "second opportunity"
	second, err := Prepare(profile, assignment, []string{"HOME=" + home})
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("SessionID = %q, want %q", second.SessionID, first.SessionID)
	}
	if !second.Resumed {
		t.Fatal("second invocation did not resume")
	}
	assertArgsContainSequence(t, second.Args, "--resume", first.SessionID)
	if slicesContain(second.Args, "--no-session-persistence") {
		t.Fatalf("resumed Claude arguments disable persistence: %#v", second.Args)
	}
}

func TestResumeCanBeDisabled(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".codex", "auth.json"), `{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`)
	assignment := testAssignment(t)
	first, err := Prepare(Profile{Runner: RunnerCodex}, assignment, []string{"HOME=" + home})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.MarkSuccessful(); err != nil {
		t.Fatal(err)
	}
	resume := false
	second, err := Prepare(Profile{Runner: RunnerCodex, Resume: &resume}, assignment, []string{"HOME=" + home})
	if err != nil {
		t.Fatal(err)
	}
	if second.Resumed {
		t.Fatal("invocation resumed with Resume set to false")
	}
	want := []string{"--strict-config", "exec", "--json", "--skip-git-repo-check", assignment.Prompt}
	if !reflect.DeepEqual(second.Args, want) {
		t.Fatalf("Args = %#v, want %#v", second.Args, want)
	}
}

func TestPrepareRejectsMalformedClaudeSessionID(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude", ".credentials.json"), `{"claudeAiOauth":{"accessToken":"token"}}`)
	assignment := testAssignment(t)
	writeTestFile(t, filepath.Join(assignment.StateDir, "claude", "session-id"), "malformed")
	_, err := Prepare(Profile{Runner: RunnerClaude}, assignment, []string{"HOME=" + home})
	if err == nil || !strings.Contains(err.Error(), "not a UUID") {
		t.Fatalf("error = %v", err)
	}
	for _, secret := range []string{
		filepath.Join(assignment.StateDir, "claude", "home", ".claude", ".credentials.json"),
		filepath.Join(assignment.StateDir, "claude", "mcp.json"),
	} {
		if _, err := os.Stat(secret); !os.IsNotExist(err) {
			t.Fatalf("failed preparation retained secret %q or stat failed: %v", secret, err)
		}
	}
}

func TestPrepareRejectsInvalidMCPServer(t *testing.T) {
	assignment := testAssignment(t)
	assignment.MCP.URL = "file:///tmp/mcp.sock"
	_, err := Prepare(Profile{Runner: RunnerCodex}, assignment, []string{"HOME=" + t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "absolute HTTP or HTTPS URL") {
		t.Fatalf("error = %v", err)
	}

	assignment = testAssignment(t)
	assignment.MCP.Name = "court server"
	_, err = Prepare(Profile{Runner: RunnerCodex}, assignment, []string{"HOME=" + t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "only letters") {
		t.Fatalf("error = %v", err)
	}
}

func testAssignment(t *testing.T) Assignment {
	t.Helper()
	return Assignment{
		StateDir: t.TempDir(),
		WorkDir:  t.TempDir(),
		Prompt:   "decide the current opportunity",
		MCP: MCPServer{
			Name: "court",
			URL:  "https://court.example.test/mcp",
		},
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func testPiCodexCredentials(t *testing.T, expires int64) string {
	t.Helper()
	document := struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
	}{AuthMode: "chatgpt"}
	document.Tokens.AccessToken = testPiAccessToken(t, expires)
	document.Tokens.RefreshToken = "test-refresh"
	document.Tokens.AccountID = "test-account"
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func testPiAccessToken(t *testing.T, expires int64) string {
	t.Helper()
	payload, err := json.Marshal(struct {
		Expires int64 `json:"exp"`
	}{Expires: expires})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func envValue(t *testing.T, env []string, name string) string {
	t.Helper()
	value, ok := lookupEnv(env, name)
	if !ok {
		t.Fatalf("environment variable %s is absent from %#v", name, env)
	}
	return value
}

func assertEnvAbsent(t *testing.T, env []string, names ...string) {
	t.Helper()
	for _, name := range names {
		if value, ok := lookupEnv(env, name); ok {
			t.Errorf("environment variable %s remains with value %q", name, value)
		}
	}
}

func assertArgsContainSequence(t *testing.T, args []string, sequence ...string) {
	t.Helper()
	for index := 0; index+len(sequence) <= len(args); index++ {
		if reflect.DeepEqual(args[index:index+len(sequence)], sequence) {
			return
		}
	}
	t.Fatalf("arguments %#v do not contain sequence %#v", args, sequence)
}

func assertClaudeIsolationSettings(t *testing.T, args []string, assignment Assignment) {
	t.Helper()
	settingsJSON := argumentValue(t, args, "--settings")
	if strings.Contains(settingsJSON, assignment.MCP.BearerToken) {
		t.Fatal("Claude settings contain the MCP bearer token")
	}
	var settings struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
		Sandbox struct {
			Enabled                  *bool    `json:"enabled"`
			FailIfUnavailable        *bool    `json:"failIfUnavailable"`
			AllowUnsandboxedCommands *bool    `json:"allowUnsandboxedCommands"`
			ExcludedCommands         []string `json:"excludedCommands"`
			Filesystem               struct {
				AllowWrite []string `json:"allowWrite"`
				DenyRead   []string `json:"denyRead"`
				DenyWrite  []string `json:"denyWrite"`
			} `json:"filesystem"`
			Network struct {
				AllowedDomains      []string `json:"allowedDomains"`
				AllowUnixSockets    []string `json:"allowUnixSockets"`
				AllowAllUnixSockets *bool    `json:"allowAllUnixSockets"`
				AllowLocalBinding   *bool    `json:"allowLocalBinding"`
			} `json:"network"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		t.Fatalf("decode Claude settings: %v", err)
	}
	if settings.Sandbox.Enabled == nil || !*settings.Sandbox.Enabled || settings.Sandbox.FailIfUnavailable == nil || !*settings.Sandbox.FailIfUnavailable {
		t.Fatalf("Claude sandbox is not required: %s", settingsJSON)
	}
	if settings.Sandbox.AllowUnsandboxedCommands == nil || *settings.Sandbox.AllowUnsandboxedCommands {
		t.Fatalf("Claude sandbox permits unsandboxed commands: %s", settingsJSON)
	}
	if settings.Sandbox.ExcludedCommands == nil || len(settings.Sandbox.ExcludedCommands) != 0 {
		t.Fatalf("Claude sandbox excluded commands = %#v", settings.Sandbox.ExcludedCommands)
	}
	absWorkDir, err := filepath.Abs(assignment.WorkDir)
	if err != nil {
		t.Fatal(err)
	}
	authPath, err := filepath.Abs(filepath.Join(assignment.StateDir, "claude", "home", ".claude", ".credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcpPath, err := filepath.Abs(filepath.Join(assignment.StateDir, "claude", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings.Sandbox.Filesystem.AllowWrite, []string{absWorkDir}) {
		t.Fatalf("Claude sandbox write paths = %#v, want %q", settings.Sandbox.Filesystem.AllowWrite, absWorkDir)
	}
	for name, got := range map[string][]string{
		"denyRead":  settings.Sandbox.Filesystem.DenyRead,
		"denyWrite": settings.Sandbox.Filesystem.DenyWrite,
	} {
		if !reflect.DeepEqual(got, []string{authPath, mcpPath}) {
			t.Fatalf("Claude sandbox %s = %#v", name, got)
		}
	}
	wantPermissionDeny := []string{
		"Read(" + filepath.ToSlash(authPath) + ")",
		"Edit(" + filepath.ToSlash(authPath) + ")",
		"Read(" + filepath.ToSlash(mcpPath) + ")",
		"Edit(" + filepath.ToSlash(mcpPath) + ")",
	}
	if !reflect.DeepEqual(settings.Permissions.Deny, wantPermissionDeny) {
		t.Fatalf("Claude permission denies = %#v, want %#v", settings.Permissions.Deny, wantPermissionDeny)
	}
	if settings.Sandbox.Network.AllowUnixSockets == nil || len(settings.Sandbox.Network.AllowUnixSockets) != 0 {
		t.Fatalf("Claude sandbox Unix socket allowlist = %#v", settings.Sandbox.Network.AllowUnixSockets)
	}
	if !reflect.DeepEqual(settings.Sandbox.Network.AllowedDomains, []string{"*"}) {
		t.Fatalf("Claude sandbox allowed domains = %#v, want all domains", settings.Sandbox.Network.AllowedDomains)
	}
	if settings.Sandbox.Network.AllowAllUnixSockets == nil || *settings.Sandbox.Network.AllowAllUnixSockets || settings.Sandbox.Network.AllowLocalBinding == nil || *settings.Sandbox.Network.AllowLocalBinding {
		t.Fatalf("Claude sandbox permits Unix sockets or local binding: %s", settingsJSON)
	}
	if strings.Contains(settingsJSON, `"strictAllowlist"`) {
		t.Fatalf("Claude sandbox enables a second network allowlist: %s", settingsJSON)
	}
}

func argumentValue(t *testing.T, args []string, name string) string {
	t.Helper()
	for index := 0; index < len(args); index++ {
		if args[index] == name {
			if index+1 >= len(args) {
				t.Fatalf("argument %s has no value in %#v", name, args)
			}
			return args[index+1]
		}
	}
	t.Fatalf("argument %s is absent from %#v", name, args)
	return ""
}

func slicesContain(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
