package localrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	"github.com/agentcourt/adj/internal/launcherprompt"
	headless "github.com/agentcourt/adj/runtime/agent"
)

var pairedCoreBinDir = flag.String("core-bin-dir", "", "Directory containing paired core executables")
var pairedCoreRoot = flag.String("core-root", "", "Paired core checkout root")

func mustARBDLauncherPrompts(t *testing.T, overrides map[string]string) launcherprompt.Sources {
	t.Helper()
	prompts, err := launcherprompt.Resolve("arbd", "", overrides)
	if err != nil {
		t.Fatal(err)
	}
	return prompts
}

func TestRenderInstructionsUsesTemplateData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lawyer.md.tmpl")
	if err := os.WriteFile(path, []byte("case={{CASE_ID}} role={{ROLE_ID}} server={{MCP_SERVER}} workspace={{WORKSPACE}} search={{SEARCH_INSTRUCTIONS}}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	prompts, err := launcherprompt.Resolve("arbd", "", map[string]string{"participant.openclaw": path})
	if err != nil {
		t.Fatal(err)
	}
	state := &runState{launcherPrompts: prompts}
	got, err := state.renderLauncherPrompt("participant.openclaw", instructionData{
		CaseID:             "case-1",
		RoleID:             "plaintiff",
		MCPServer:          "aard-case-1-plaintiff",
		MCPURL:             "http://example/mcp",
		Workspace:          "/home/node/work",
		SearchInstructions: "search enabled",
	})
	if err != nil {
		t.Fatalf("render instructions: %v", err)
	}
	for _, want := range []string{"case=case-1", "role=plaintiff", "server=aard-case-1-plaintiff", "workspace=/home/node/work", "search=search enabled"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered instructions missing %q: %s", want, got)
		}
	}
}

func TestCoreCaseArgsUseProcessInterface(t *testing.T) {
	args := coreCaseArgs(Options{
		ComplaintPath:    "/case/complaint.md",
		CaseFiles:        []string{"/case/source-1", "/case/source-2"},
		OutputDir:        "/out",
		CoreOutputDir:    "/out/aard-output",
		PolicyPath:       "/case/policy.json",
		CouncilSize:      3,
		JudgmentStandard: "score from 0 through 100",
		PromptDir:        "/case/prompts",
		PromptFiles: map[string]string{
			"attorney.common":    "/case/common.md",
			"attorney.arguments": "/case/arguments.md",
			"attorney.rebuttals": "/case/rebuttals.md",
		},
		CommonRoot:              "/common",
		CouncilPoolPath:         "/case/pool.jsonl",
		CouncilAllowedEndpoints: []string{"openai", "anthropic"},
		CouncilMinEndpoints:     2,
		CouncilTimeoutSeconds:   90,
		LawyerTimeoutSeconds:    60,
		MaxResponseBytes:        4096,
		InvalidAttemptLimit:     2,
		EnginePath:              "/bin/aardengine",
		RunID:                   "run-1",
		CaseID:                  "case-1",
	}, "127.0.0.1:9001")
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"case", "--complaint\x00/case/complaint.md", "--file\x00/case/source-1",
		"--file\x00/case/source-2", "--out-dir\x00/out/aard-output", "--case-id\x00case-1",
		"--run-id\x00run-1", "--caseapi-addr\x00127.0.0.1:9001",
		"--council-backend\x00councilapi", "--policy\x00/case/policy.json",
		"--council-endpoint\x00openai", "--council-endpoint\x00anthropic",
		"--minimum-distinct-council-endpoints\x002",
		"--judgment-standard\x00score from 0 through 100", "--engine\x00/bin/aardengine",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("core args lack %q: %#v", want, args)
		}
	}
}

func TestStartCoreCaseReadsFreshResult(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	coreOutputDir := filepath.Join(dir, "aard-output")
	if err := os.Mkdir(coreOutputDir, 0o755); err != nil {
		t.Fatalf("mkdir core output: %v", err)
	}
	core := filepath.Join(dir, "aard-core")
	script := `#!/bin/sh
set -eu
out_dir=
case_id=
run_id=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out-dir) out_dir=$2; shift 2 ;;
    --case-id) case_id=$2; shift 2 ;;
    --run-id) run_id=$2; shift 2 ;;
    *) shift ;;
  esac
done
if find "$out_dir" -mindepth 1 -print -quit | grep -q .; then
  exit 41
fi
printf '{"case_id":"%s","run_id":"%s","status":"ok","answers":{"C1":72},"extra":"preserved"}\n' "$case_id" "$run_id" > "$out_dir/run.json"
printf '{"status":"ok","answers":{"C1":72}}\n'
`
	if err := os.WriteFile(core, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake core: %v", err)
	}
	done, err := startCoreCase(context.Background(), Options{
		CoreCommand:   core,
		ComplaintPath: filepath.Join(dir, "complaint.md"),
		OutputDir:     dir,
		CoreOutputDir: coreOutputDir,
		CaseID:        "case-1",
		RunID:         "run-1",
	}, "127.0.0.1:9001", logDir)
	if err != nil {
		t.Fatalf("start core case: %v", err)
	}
	outcome := <-done
	if outcome.err != nil {
		t.Fatalf("core outcome: %v", outcome.err)
	}
	if outcome.result.Status != "ok" || outcome.result.Answers["C1"] != 72 {
		t.Fatalf("result = %#v", outcome.result)
	}
	raw, err := json.Marshal(outcome.result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(raw), `"extra":"preserved"`) {
		t.Fatalf("marshaled result lost core fields: %s", raw)
	}
	for _, name := range []string{"aard.stdout", "aard.stderr"} {
		if _, err := os.Stat(filepath.Join(logDir, name)); err != nil {
			t.Fatalf("stat service log %s: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(coreOutputDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("core output contains service log %s: %v", name, err)
		}
	}
}

func TestPairedCoreCaseAPI(t *testing.T) {
	binDir := strings.TrimSpace(*pairedCoreBinDir)
	coreRoot := strings.TrimSpace(*pairedCoreRoot)
	if binDir == "" || coreRoot == "" {
		t.Skip("-core-bin-dir and -core-root are not set")
	}
	coreCommand := filepath.Join(binDir, "aard")
	enginePath := filepath.Join(coreRoot, "arbd", "engine", ".lake", "build", "bin", "aardengine")
	for _, path := range []string{coreCommand, enginePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat paired core path %s: %v", path, err)
		}
	}
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	coreOutputDir := filepath.Join(dir, "aard-output")
	if err := os.Mkdir(coreOutputDir, 0o755); err != nil {
		t.Fatalf("mkdir core output: %v", err)
	}
	complaintPath := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaintPath, []byte("# Question\n\nHow strongly does the record support the claim?\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id": "paired-response", "object": "response", "status": "completed",
			"model": "paired-council",
			"output": []map[string]any{{
				"id": "paired-message", "type": "message", "status": "completed", "role": "assistant",
				"content": []map[string]any{{"type": "output_text", "text": "ready", "annotations": []any{}}},
			}},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
		}); err != nil {
			t.Errorf("write paired provider response: %v", err)
		}
	}))
	defer provider.Close()
	t.Setenv("OPENAI_API_KEY", "paired-key")
	t.Setenv("OPENAI_BASE_URL", provider.URL+"/v1")
	policyPath := filepath.Join(dir, "policy.json")
	if err := writeJSONFile(policyPath, map[string]any{
		"council_size":      1,
		"judgment_standard": "Answer with one integer from 0 through 100.",
	}); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	poolDir := filepath.Join(dir, "pool")
	if err := os.MkdirAll(poolDir, 0o755); err != nil {
		t.Fatalf("mkdir pool: %v", err)
	}
	if err := os.WriteFile(filepath.Join(poolDir, "c1.txt"), []byte("Paired council persona.\n"), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	poolPath := filepath.Join(poolDir, "pool.jsonl")
	if err := os.WriteFile(poolPath, []byte(`{"endpoint":"openai","model":"paired-council","persona":"c1.txt"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write pool: %v", err)
	}
	caseAPIAddr, err := resolveListenAddr("127.0.0.1:0", "127.0.0.1")
	if err != nil {
		t.Fatalf("resolve case API address: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done, err := startCoreCase(ctx, Options{
		CoreCommand:           coreCommand,
		CoreWorkingDir:        filepath.Join(coreRoot, "arbd"),
		ComplaintPath:         complaintPath,
		OutputDir:             dir,
		CoreOutputDir:         coreOutputDir,
		PolicyPath:            policyPath,
		CommonRoot:            filepath.Join(coreRoot, "common"),
		CouncilPoolPath:       poolPath,
		EnginePath:            enginePath,
		CouncilTimeoutSeconds: 30,
		LawyerTimeoutSeconds:  30,
		RunID:                 "run-paired-arbd",
		CaseID:                "paired-arbd",
	}, caseAPIAddr, logDir)
	if err != nil {
		cancel()
		t.Fatalf("start paired core: %v", err)
	}
	baseURL := "http://" + caseAPIAddr
	if err := waitForCaseHealth(ctx, baseURL+"/health", "paired-arbd", "run-paired-arbd", 20*time.Second); err != nil {
		cancel()
		outcome := <-done
		t.Fatalf("wait for paired core API: %v; core outcome: %v", err, outcome.err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/lawyerapi/v1/status?case_id=paired-arbd&role_id=plaintiff", nil)
	if err != nil {
		cancel()
		t.Fatalf("build status request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("read paired core status: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		cancel()
		t.Fatalf("close paired core status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("paired core status HTTP = %d", resp.StatusCode)
	}
	completions := launcherCompletions{caseDone: done, casePending: true}
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- completions.shutdown(cancel)
	}()
	select {
	case shutdownErr := <-shutdownDone:
		if shutdownErr == nil {
			t.Fatalf("canceled paired core returned no process error")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("launcher shutdown did not drain paired core")
	}
}

func TestWritePiConfigFromRosterEntry(t *testing.T) {
	maxTokens := int64(1234)
	temperature := 0.2
	topP := 0.8
	allowFallbacks := false
	home := t.TempDir()
	spec := modelrequest.Spec{
		Endpoint: "openrouter",
		Model:    "anthropic/claude-sonnet-4",
		Provider: &modelrequest.ProviderConstraints{
			Only:           []string{"anthropic"},
			AllowFallbacks: &allowFallbacks,
			Quantizations:  []string{"bf16"},
		},
		Request: modelrequest.RequestParameters{
			Temperature:     &temperature,
			TopP:            &topP,
			MaxOutputTokens: &maxTokens,
		},
		Headers: map[string]string{"X-Test-Request": "arbd"},
	}
	model, err := writePiConfig(home, councilRosterEntry{
		MemberID:    "C1",
		RequestSpec: &spec,
	}, spec, "http://127.0.0.1:18888/v1", modelgateway.Binding{Token: "local-token", Model: "adj-model-1"}, "aard-case-C1", "http://127.0.0.1:19780/mcp", "adjmcp1.test.signature")
	if err != nil {
		t.Fatalf("write Pi config: %v", err)
	}
	if model != "adj-model-1" {
		t.Fatalf("model = %q", model)
	}
	settings := readJSONMap(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	if settings["defaultProvider"] != "adj" || settings["defaultModel"] != "adj-model-1" {
		t.Fatalf("settings = %#v", settings)
	}
	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	providers := models["providers"].(map[string]any)
	provider := providers["adj"].(map[string]any)
	if provider["baseUrl"] != "http://127.0.0.1:18888/v1" || provider["apiKey"] != "$ADJ_MODEL_API_KEY" {
		t.Fatalf("provider = %#v", provider)
	}
	modelList := provider["models"].([]any)
	modelEntry := modelList[0].(map[string]any)
	if modelEntry["maxTokens"] != float64(maxTokens) {
		t.Fatalf("model entry maxTokens = %#v", modelEntry["maxTokens"])
	}
	sampling := modelEntry["samplingParams"].(map[string]any)
	if sampling["temperature"] != temperature || sampling["top_p"] != topP {
		t.Fatalf("samplingParams = %#v", sampling)
	}
	compat := modelEntry["compat"].(map[string]any)
	routing := compat["openRouterRouting"].(map[string]any)
	if routing["allow_fallbacks"] != false {
		t.Fatalf("routing = %#v", routing)
	}
	quantizations := routing["quantizations"].([]any)
	if len(quantizations) != 1 || quantizations[0] != "bf16" {
		t.Fatalf("routing quantizations = %#v", routing["quantizations"])
	}
	if _, ok := provider["headers"]; ok {
		t.Fatalf("provider contains upstream headers: %#v", provider)
	}
	mcpPath := filepath.Join(home, ".mcp.json")
	mcpConfig := readJSONMap(t, mcpPath)
	mcpInfo, err := os.Stat(mcpPath)
	if err != nil {
		t.Fatalf("stat MCP config: %v", err)
	}
	if mcpInfo.Mode().Perm() != 0o600 {
		t.Fatalf("MCP config mode = %04o", mcpInfo.Mode().Perm())
	}
	servers := mcpConfig["mcpServers"].(map[string]any)
	server := servers["aard-case-C1"].(map[string]any)
	if server["url"] != "http://127.0.0.1:19780/mcp" {
		t.Fatalf("mcp server = %#v", server)
	}
	headers := server["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer adjmcp1.test.signature" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestWritePiConfigAddsDefaultMaxTokens(t *testing.T) {
	home := t.TempDir()
	spec := modelrequest.Spec{
		Endpoint: "openrouter",
		Model:    "anthropic/claude-opus-4.6-fast",
	}.WithFallbackMaxOutputTokens(DefaultCouncilMaxOutputTokens)
	_, err := writePiConfig(home, councilRosterEntry{
		MemberID:    "C1",
		RequestSpec: &spec,
	}, spec, "http://127.0.0.1:18888/v1", modelgateway.Binding{Token: "local-token", Model: "adj-model-1"}, "aard-case-C1", "http://127.0.0.1:19780/mcp", "adjmcp1.test.signature")
	if err != nil {
		t.Fatalf("write Pi config: %v", err)
	}
	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	providers := models["providers"].(map[string]any)
	provider := providers["adj"].(map[string]any)
	modelList := provider["models"].([]any)
	modelEntry := modelList[0].(map[string]any)
	want := float64(DefaultCouncilMaxOutputTokens)
	if modelEntry["maxTokens"] != want {
		t.Fatalf("model entry maxTokens = %#v, want %#v", modelEntry["maxTokens"], want)
	}
}

func TestValidatedPiRequestRejectsMissingRequestSpec(t *testing.T) {
	_, _, err := validatedPiRequest(councilRosterEntry{
		MemberID: "C1",
		Model:    "openrouter://anthropic/claude-sonnet-4",
	})
	if err == nil || !strings.Contains(err.Error(), "request_spec") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveOpenClawAuthDefaultsToCodexAuth(t *testing.T) {
	path := writeCodexAuth(t, t.TempDir())
	auth, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath: path,
		ParticipantEnvironment: []string{
			"HOME=" + t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("resolve OpenClaw auth: %v", err)
	}
	if auth.Mode != "codex" || auth.CodexAuthPath != path {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestResolveOpenClawAuthDoesNotFallBackToAPIKey(t *testing.T) {
	_, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath: filepath.Join(t.TempDir(), "missing-auth.json"),
		ParticipantEnvironment: []string{
			"OPENAI_API_KEY=api-key",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "read Codex auth file") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveOpenClawAuthRequiresSelectedCredential(t *testing.T) {
	_, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath:  filepath.Join(t.TempDir(), "missing-auth.json"),
		ParticipantEnvironment: []string{},
	})
	if err == nil || !strings.Contains(err.Error(), "read Codex auth file") {
		t.Fatalf("error = %v", err)
	}
	_, err = resolveOpenClawAuth(Options{OpenClawAuth: "api-key", ParticipantEnvironment: []string{}})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("api-key error = %v", err)
	}
	_, err = resolveOpenClawAuth(Options{OpenClawAuth: "auto"})
	if err == nil || !strings.Contains(err.Error(), "expected codex or api-key") {
		t.Fatalf("auto error = %v", err)
	}
}

func TestLawyerEnvironmentSeparatesRoleCredentials(t *testing.T) {
	opts := Options{
		PlaintiffLawyer: LawyerProfile{Runner: LawyerPi, AuthMode: headless.AuthAPIKey, APIKeyEnv: "PLAINTIFF_KEY"},
		DefendantLawyer: LawyerProfile{Runner: LawyerClaude, AuthMode: headless.AuthAPIKey, APIKeyEnv: "DEFENDANT_KEY"},
	}
	base := []string{"PATH=/bin", "PLAINTIFF_KEY=plaintiff", "DEFENDANT_KEY=defendant", "OPENROUTER_API_KEY=council"}
	plaintiff := lawyerEnvironment(opts.PlaintiffLawyer, opts, base)
	if value, ok := environmentValue(plaintiff, "PLAINTIFF_KEY"); !ok || value != "plaintiff" {
		t.Fatalf("plaintiff credential = %q, %t", value, ok)
	}
	for _, name := range []string{"DEFENDANT_KEY", "OPENROUTER_API_KEY"} {
		if _, ok := environmentValue(plaintiff, name); ok {
			t.Fatalf("plaintiff environment contains %s", name)
		}
	}
}

func TestResolveOpenClawLawyerAuthUsesRoleProfile(t *testing.T) {
	credentials := writeCodexAuth(t, t.TempDir())
	baseEnvironment := []string{"ROLE_OPENAI_KEY=selected"}
	opts := applyDefaults(Options{ParticipantEnvironment: baseEnvironment})
	auth, err := resolveOpenClawLawyerAuth(LawyerProfile{
		Runner:          LawyerOpenClaw,
		AuthMode:        headless.AuthSubscription,
		CredentialsFile: credentials,
	}, opts, opts.ParticipantEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Mode != "codex" || auth.CodexAuthPath != credentials {
		t.Fatalf("subscription auth = %#v", auth)
	}
	auth, err = resolveOpenClawLawyerAuth(LawyerProfile{
		Runner:    LawyerOpenClaw,
		AuthMode:  headless.AuthAPIKey,
		APIKeyEnv: "ROLE_OPENAI_KEY",
	}, opts, opts.ParticipantEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Mode != "api-key" || auth.APIKeyEnv != "ROLE_OPENAI_KEY" {
		t.Fatalf("API-key auth = %#v", auth)
	}
}

func TestApplyDefaultsUsesIndependentLawyerProfiles(t *testing.T) {
	opts := applyDefaults(Options{})
	if opts.PlaintiffLawyer.Runner != LawyerOpenClaw || opts.DefendantLawyer.Runner != LawyerOpenClaw {
		t.Fatalf("default lawyer profiles = %#v, %#v", opts.PlaintiffLawyer, opts.DefendantLawyer)
	}
	opts = applyDefaults(Options{
		PlaintiffLawyer: LawyerProfile{Runner: LawyerPi},
		DefendantLawyer: LawyerProfile{Runner: LawyerClaude},
	})
	if opts.PlaintiffLawyer.Runner != LawyerPi || opts.DefendantLawyer.Runner != LawyerClaude {
		t.Fatalf("selected lawyer profiles = %#v, %#v", opts.PlaintiffLawyer, opts.DefendantLawyer)
	}
}

func TestHeadlessMCPServerNameIsValid(t *testing.T) {
	got := headlessMCPServerName("case/with spaces.and:punctuation", "plaintiff")
	if got != "aard-case-with-spaces-and-punctuation-plaintiff" {
		t.Fatalf("headless MCP server name = %q", got)
	}
}

func TestValidateLawyerProfileAuthentication(t *testing.T) {
	home := t.TempDir()
	codexAuth := filepath.Join(home, "codex-auth.json")
	piCodexAuth := filepath.Join(home, "pi-codex-auth.json")
	claudeAuth := filepath.Join(home, "claude-credentials.json")
	if err := os.WriteFile(codexAuth, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piCodexAuth, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"e30.eyJleHAiOjQxMDI0NDQ4MDB9.signature","refresh_token":"refresh","account_id":"account"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeAuth, []byte(`{"claudeAiOauth":{"accessToken":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := applyDefaults(Options{})
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerCodex, CredentialsFile: codexAuth}, opts, []string{"HOME=" + home}); err != nil {
		t.Fatalf("Codex profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerClaude, CredentialsFile: claudeAuth}, opts, []string{"HOME=" + home}); err != nil {
		t.Fatalf("Claude profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{
		Runner:          LawyerPi,
		Provider:        headless.ProviderOpenAI,
		Model:           "openai/gpt-5.6-sol",
		ReasoningEffort: "xhigh",
		AuthMode:        headless.AuthSubscription,
		CredentialsFile: piCodexAuth,
	}, opts, []string{"HOME=" + home, "OPENAI_API_KEY=unselected"}); err != nil {
		t.Fatalf("Pi profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{
		Runner:          LawyerPi,
		Provider:        headless.ProviderOpenRouter,
		Model:           "openrouter/openai/gpt-5.6-sol",
		AuthMode:        headless.AuthSubscription,
		CredentialsFile: piCodexAuth,
	}, opts, []string{"HOME=" + home}); err == nil || !strings.Contains(err.Error(), "supports only provider") {
		t.Fatalf("unsupported Pi subscription provider error = %v", err)
	}
}

func TestEffectiveLawyerTurnTimeoutSeconds(t *testing.T) {
	if got := effectiveLawyerTurnTimeoutSeconds(Options{}); got != DefaultRunLawyerTimeoutSeconds {
		t.Fatalf("default timeout = %d", got)
	}
	if got := effectiveLawyerTurnTimeoutSeconds(Options{LawyerTimeoutSeconds: 123}); got != 123 {
		t.Fatalf("override timeout = %d", got)
	}
}

func TestApplyDefaultsOpenClawStartDelay(t *testing.T) {
	if got := applyDefaults(Options{OpenClawStartDelaySeconds: -1}).OpenClawStartDelaySeconds; got != defaultOpenClawStartDelay {
		t.Fatalf("default start delay = %d", got)
	}
	if got := applyDefaults(Options{OpenClawStartDelaySeconds: 0}).OpenClawStartDelaySeconds; got != 0 {
		t.Fatalf("zero start delay = %d", got)
	}
	if got := applyDefaults(Options{OpenClawStartDelaySeconds: 27}).OpenClawStartDelaySeconds; got != 27 {
		t.Fatalf("override start delay = %d", got)
	}
}

func TestApplyDefaultsRunTurnTimeouts(t *testing.T) {
	opts := applyDefaults(Options{})
	if opts.CouncilTimeoutSeconds != DefaultRunCouncilTimeoutSeconds {
		t.Fatalf("default council timeout = %d", opts.CouncilTimeoutSeconds)
	}
	if opts.LawyerTimeoutSeconds != DefaultRunLawyerTimeoutSeconds {
		t.Fatalf("default lawyer timeout = %d", opts.LawyerTimeoutSeconds)
	}
	opts = applyDefaults(Options{CouncilTimeoutSeconds: 123, LawyerTimeoutSeconds: 456})
	if opts.CouncilTimeoutSeconds != 123 || opts.LawyerTimeoutSeconds != 456 {
		t.Fatalf("timeouts = council %d lawyer %d", opts.CouncilTimeoutSeconds, opts.LawyerTimeoutSeconds)
	}
}

func TestPrepareOutputLayoutRejectsSymlinkChildren(t *testing.T) {
	for _, child := range []string{"aard-output", "logs"} {
		t.Run(child, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, child)); err != nil {
				t.Fatalf("create symlink: %v", err)
			}
			err := prepareOutputLayout(root, filepath.Join(root, "aard-output"), filepath.Join(root, "logs"))
			if err == nil {
				t.Fatal("output preparation accepted a symlink child")
			}
		})
	}
}

func TestApplyDefaultsOpenClawNetworkHostMCPHost(t *testing.T) {
	opts := applyDefaults(Options{OpenClawNetwork: "host"})
	if opts.DockerMCPHost != "127.0.0.1" {
		t.Fatalf("DockerMCPHost = %q", opts.DockerMCPHost)
	}
	opts = applyDefaults(Options{OpenClawNetwork: "host", DockerMCPHost: "custom"})
	if opts.DockerMCPHost != "custom" {
		t.Fatalf("custom DockerMCPHost = %q", opts.DockerMCPHost)
	}
	opts = applyDefaults(Options{})
	if opts.DockerMCPHost != "host.docker.internal" {
		t.Fatalf("default DockerMCPHost = %q", opts.DockerMCPHost)
	}
}

func TestValidateOptionsRejectsInvalidOpenClawNetwork(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENROUTER_API_KEY", "key")
	err := validateOptions(applyDefaults(Options{
		ComplaintPath:           "complaint.md",
		OutputDir:               dir,
		CaseID:                  "case",
		AutoLawyers:             DefaultAutoLawyers,
		CouncilOutputLimitBytes: 1,
		OpenClawNetwork:         "bridge",
	}))
	if err == nil || !strings.Contains(err.Error(), "invalid OpenClaw network") {
		t.Fatalf("validateOptions error = %v", err)
	}
}

func TestValidateOptionsAllowsCouncilWithoutOpenRouterKey(t *testing.T) {
	dir := t.TempDir()
	err := validateOptions(applyDefaults(Options{
		ComplaintPath:          "complaint.md",
		OutputDir:              dir,
		CaseID:                 "case",
		AutoLawyers:            "none",
		CoreEnvironment:        []string{},
		MCPEnvironment:         []string{},
		ParticipantEnvironment: []string{},
	}))
	if err != nil {
		t.Fatalf("validateOptions error = %v", err)
	}
}

func TestApplyDefaultsCouncilOutputLimit(t *testing.T) {
	if got := applyDefaults(Options{}).CouncilOutputLimitBytes; got != DefaultCouncilOutputLimitBytes {
		t.Fatalf("default council output limit = %d", got)
	}
	if got := applyDefaults(Options{CouncilOutputLimitBytes: 123}).CouncilOutputLimitBytes; got != 123 {
		t.Fatalf("council output limit override = %d", got)
	}
}

func TestCouncilProcessOutputSizeCountsLogs(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "pi-C1.stdout")
	stderrPath := filepath.Join(dir, "pi-C1.stderr")
	if err := os.WriteFile(stdoutPath, []byte("abcdef"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	if err := os.WriteFile(stderrPath, []byte("xyz"), 0o644); err != nil {
		t.Fatalf("write stderr: %v", err)
	}
	size, err := councilProcessOutputSize(&processRecord{
		name:       "pi-C1",
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
	})
	if err != nil {
		t.Fatalf("council process output size: %v", err)
	}
	if size.Stdout != 6 || size.Stderr != 3 || size.Total != 9 {
		t.Fatalf("size = %#v", size)
	}
}

func TestCouncilProcessOutputSizeUsesStdoutCounter(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "pi-C1.stdout")
	stderrPath := filepath.Join(dir, "pi-C1.stderr")
	if err := os.WriteFile(stdoutPath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	if err := os.WriteFile(stderrPath, []byte("xy"), 0o644); err != nil {
		t.Fatalf("write stderr: %v", err)
	}
	var out bytes.Buffer
	counter := newProcessOutputCounter(&out)
	if _, err := counter.Write([]byte("abcdef")); err != nil {
		t.Fatalf("write counter: %v", err)
	}
	size, err := councilProcessOutputSize(&processRecord{
		name:          "pi-C1",
		stdoutPath:    stdoutPath,
		stderrPath:    stderrPath,
		stdoutCounter: counter,
	})
	if err != nil {
		t.Fatalf("council process output size: %v", err)
	}
	if size.Stdout != 6 || size.Stderr != 2 || size.Total != 8 {
		t.Fatalf("size = %#v", size)
	}
}

func TestMonitorCouncilOutputKillsProcessOverLimit(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "pi-C1.stdout")
	stderrPath := filepath.Join(dir, "pi-C1.stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	if _, err := stdout.WriteString("abcdef"); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	cmd := exec.Command("sleep", "60")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	proc := &processRecord{
		name:       "pi-C1",
		kind:       "podman",
		command:    cmd,
		done:       make(chan processExit, 1),
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
		finished:   make(chan struct{}),
	}
	go func() {
		exit := processExit{
			waitErr:   cmd.Wait(),
			stdoutErr: stdout.Close(),
			stderrErr: stderr.Close(),
		}
		proc.markExited()
		proc.done <- exit
	}()
	t.Cleanup(func() {
		if !proc.isExited() {
			_ = cmd.Process.Kill()
			<-proc.finished
		}
	})

	state := &runState{
		opts:      Options{CouncilOutputLimitBytes: 5},
		agentErrs: make(chan error, 1),
	}
	state.monitorCouncilOutput(context.Background(), proc, councilProcessTarget{
		memberID:      "C1",
		opportunityID: "deliberation:1:C1",
	}, 10*time.Millisecond)
	select {
	case <-proc.finished:
	case err := <-state.agentErrs:
		t.Fatalf("agent error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatalf("process was not killed")
	}
	reason, message, details := proc.forcedFailure()
	if reason != councilFailureOutputLimit {
		t.Fatalf("forced reason = %q", reason)
	}
	if !strings.Contains(message, "exceeded the output limit") {
		t.Fatalf("message = %q", message)
	}
	if details["output_bytes"] != int64(6) || details["output_limit_bytes"] != int64(5) {
		t.Fatalf("details = %#v", details)
	}
}

func TestCouncilProcessReplacementReapsPriorProcess(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "pi-C1.stdout")
	stderrPath := filepath.Join(dir, "pi-C1.stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	cmd := exec.Command("sleep", "60")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	runtimeLog := filepath.Join(dir, "runtime.log")
	runtimePath := filepath.Join(dir, "podman")
	t.Setenv("FAKE_CONTAINER_PID", fmt.Sprintf("%d", cmd.Process.Pid))
	t.Setenv("FAKE_CONTAINER_LOG", runtimeLog)
	runtimeScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$FAKE_CONTAINER_LOG\"\nkill \"$FAKE_CONTAINER_PID\"\n"
	if err := os.WriteFile(runtimePath, []byte(runtimeScript), 0o755); err != nil {
		t.Fatalf("write fake container runtime: %v", err)
	}
	containerID := strings.Repeat("b", 64)
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, "aard-case-1-c1")
	if err != nil {
		t.Fatalf("create container ID path: %v", err)
	}
	if err := os.WriteFile(containerIDPath, []byte(containerID+"\n"), 0o600); err != nil {
		t.Fatalf("write container ID: %v", err)
	}
	proc := &processRecord{
		name:            "pi-C1",
		kind:            "podman",
		command:         cmd,
		done:            make(chan processExit, 1),
		stopCommand:     runtimePath,
		containerIDPath: containerIDPath,
		containerIDDir:  containerIDDir,
		councilTarget: &councilProcessTarget{
			memberID:      "C1",
			opportunityID: "opportunity-1",
		},
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
		finished:   make(chan struct{}),
	}
	go func() {
		exit := processExit{
			waitErr:   cmd.Wait(),
			stdoutErr: stdout.Close(),
			stderrErr: stderr.Close(),
		}
		proc.markExited()
		proc.done <- exit
	}()
	t.Cleanup(func() {
		if !proc.isExited() {
			_ = cmd.Process.Kill()
			<-proc.finished
		}
	})

	state := &runState{processes: []*processRecord{proc}}
	if err := state.stopPriorCouncilProcesses("C1", "opportunity-1"); err != nil {
		t.Fatalf("retain current council process: %v", err)
	}
	if proc.isExited() || len(state.processes) != 1 {
		t.Fatal("current opportunity process was stopped")
	}
	if err := state.stopPriorCouncilProcesses("C1", "opportunity-2"); err != nil {
		t.Fatalf("stop prior council process: %v", err)
	}
	select {
	case <-proc.finished:
	default:
		t.Fatal("prior council process was not reaped")
	}
	if len(state.processes) != 0 {
		t.Fatalf("active processes = %#v", state.processes)
	}
	if err := state.stopAgents(); err != nil {
		t.Fatalf("stop agents after replacement: %v", err)
	}
	raw, err := os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read fake container runtime log: %v", err)
	}
	if strings.TrimSpace(string(raw)) != "container rm -f "+containerID {
		t.Fatalf("container runtime args = %q", raw)
	}

	collisionIDPath, collisionIDDir, err := createContainerIDPath(dir, "aard-case-1-c1")
	if err != nil {
		t.Fatalf("reserve collision container ID path: %v", err)
	}
	collisionCmd := exec.Command("sleep", "60")
	if err := collisionCmd.Start(); err != nil {
		t.Fatalf("start collision client: %v", err)
	}
	collision := &processRecord{
		name:            "pi-C1-collision",
		kind:            "podman",
		command:         collisionCmd,
		done:            make(chan processExit, 1),
		stopCommand:     runtimePath,
		containerIDPath: collisionIDPath,
		containerIDDir:  collisionIDDir,
		finished:        make(chan struct{}),
	}
	go func() {
		exit := processExit{waitErr: collisionCmd.Wait()}
		collision.markExited()
		collision.done <- exit
	}()
	if err := stopContainerProcess(collision); err == nil || !strings.Contains(err.Error(), "container ID") {
		t.Fatalf("stop collision client error = %v", err)
	}
	select {
	case <-collision.finished:
	default:
		t.Fatal("collision client was not reaped")
	}
	raw, err = os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read fake container runtime log after collision: %v", err)
	}
	if strings.TrimSpace(string(raw)) != "container rm -f "+containerID {
		t.Fatalf("name collision caused an unowned container removal: %q", raw)
	}
}

func TestStopContainerProcessAcceptsExitedAutoRemovedContainer(t *testing.T) {
	dir := t.TempDir()
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, "aard-case-1-c1")
	if err != nil {
		t.Fatalf("create container ID path: %v", err)
	}
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start client: %v", err)
	}
	proc := &processRecord{
		name:            "pi-C1",
		kind:            "podman",
		command:         cmd,
		done:            make(chan processExit, 1),
		stopCommand:     filepath.Join(dir, "podman"),
		containerIDPath: containerIDPath,
		containerIDDir:  containerIDDir,
		finished:        make(chan struct{}),
	}
	go func() {
		exit := processExit{waitErr: cmd.Wait()}
		proc.markExited()
		proc.done <- exit
	}()
	select {
	case <-proc.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("client did not exit")
	}
	if err := stopContainerProcess(proc); err != nil {
		t.Fatalf("stop exited client: %v", err)
	}
}

func TestPiMessageUpdateTailFilterCompactsAccumulatedThinking(t *testing.T) {
	var filter piMessageUpdateTailFilter
	first := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":"abc","thinkingSignature":"reasoning"}]}},"message":{"responseId":"r1","content":[{"type":"thinking","thinking":"abc","thinkingSignature":"reasoning"}]}}`)
	if got := filter.filterLine(first); string(got) != string(first) {
		t.Fatalf("first update changed:\n%s", got)
	}
	second := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"def","partial":{"responseId":"r1","content":[{"type":"thinking","thinking":"abcdef","thinkingSignature":"reasoning"}]}},"message":{"responseId":"r1","content":[{"type":"thinking","thinking":"abcdef","thinkingSignature":"reasoning"}]}}`)
	got := filter.filterLine(second)
	if string(got) == string(second) {
		t.Fatalf("second update was not compacted")
	}

	var event map[string]any
	if err := json.Unmarshal(got, &event); err != nil {
		t.Fatalf("unmarshal filtered event: %v", err)
	}
	logFilter, ok := event["aard_log_filter"].(map[string]any)
	if !ok {
		t.Fatalf("missing aard_log_filter: %#v", event)
	}
	if logFilter["message"] != repeatedMessageUpdateLogFilterMessage {
		t.Fatalf("filter message = %#v", logFilter["message"])
	}
	if logFilter["dropped_prefix_bytes"] != float64(3) || logFilter["tail_bytes"] != float64(3) {
		t.Fatalf("filter details = %#v", logFilter)
	}

	assistantEvent, ok := piMapValue(event["assistantMessageEvent"])
	if !ok {
		t.Fatalf("missing assistantMessageEvent")
	}
	partial, ok := piMapValue(assistantEvent["partial"])
	if !ok {
		t.Fatalf("missing partial")
	}
	_, value, ok := piContentString(partial, 0)
	if !ok || value != "def" {
		t.Fatalf("partial content = %q, %v", value, ok)
	}
	message, ok := piMapValue(event["message"])
	if !ok {
		t.Fatalf("missing message")
	}
	_, value, ok = piContentString(message, 0)
	if !ok || value != "def" {
		t.Fatalf("message content = %q, %v", value, ok)
	}
}

func TestPiMessageUpdateTailFilterLeavesNonPrefixUpdate(t *testing.T) {
	var filter piMessageUpdateTailFilter
	first := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":"abc"}]}}}`)
	second := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":"zabc"}]}}}`)
	_ = filter.filterLine(first)
	if got := filter.filterLine(second); string(got) != string(second) {
		t.Fatalf("non-prefix update changed:\n%s", got)
	}
}

func TestPiTailLogWriterHandlesChunkedLines(t *testing.T) {
	var out bytes.Buffer
	writer := newPiTailLogWriter(&out)
	first := `{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":"abc"}]}}}`
	second := `{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":"abcdef"}]}}}`
	raw := []byte(first + "\n" + second + "\n")
	if _, err := writer.Write(raw[:17]); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	if _, err := writer.Write(raw[17:]); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, repeatedMessageUpdateLogFilterMessage) {
		t.Fatalf("filtered log missing marker:\n%s", got)
	}
	if !strings.Contains(got, `"thinking":"def"`) {
		t.Fatalf("filtered log missing tail content:\n%s", got)
	}
	if !strings.Contains(got, first) {
		t.Fatalf("first line changed:\n%s", got)
	}
}

func TestAutoLawyerRoles(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{mode: "both", want: "plaintiff,defendant"},
		{mode: "plaintiff", want: "plaintiff"},
		{mode: "defendant", want: "defendant"},
		{mode: "none", want: ""},
	}
	for _, tc := range tests {
		got, err := autoLawyerRoles(tc.mode)
		if err != nil {
			t.Fatalf("%s: %v", tc.mode, err)
		}
		if strings.Join(got, ",") != tc.want {
			t.Fatalf("%s roles = %#v", tc.mode, got)
		}
	}
	if _, err := autoLawyerRoles("other"); err == nil {
		t.Fatalf("expected invalid mode error")
	}
	if got := strings.Join(manualLawyerRoles("none"), ","); got != "plaintiff,defendant" {
		t.Fatalf("manual roles = %q", got)
	}
	if got := strings.Join(manualLawyerRoles("defendant"), ","); got != "plaintiff" {
		t.Fatalf("manual roles = %q", got)
	}
}

func TestPublicMCPBaseAndManualAddressValidation(t *testing.T) {
	base, err := publicMCPBase("http://aard.example:8001/", "0.0.0.0:1234")
	if err != nil {
		t.Fatalf("public base: %v", err)
	}
	if base != "http://aard.example:8001" {
		t.Fatalf("base = %q", base)
	}
	if err := validateManualLawyerAddress("", "0.0.0.0:1234"); err == nil {
		t.Fatalf("expected wildcard listen error")
	}
	if err := validateManualLawyerAddress("", "192.0.2.10:1234"); err != nil {
		t.Fatalf("non-wildcard listen: %v", err)
	}
}

func TestWriteRemoteLawyerSkill(t *testing.T) {
	dir := t.TempDir()
	searchPath := filepath.Join(dir, "search.md")
	if err := os.WriteFile(searchPath, []byte("Custom remote search instructions."), 0o644); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(dir, "remote.md.tmpl")
	if err := os.WriteFile(templatePath, []byte("case={{CASE_ID}} role={{ROLE_ID}} server={{MCP_SERVER}} url={{MCP_URL}} json={{MCP_JSON}} search={{SEARCH_INSTRUCTIONS}}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	state := &runState{
		opts: Options{
			CaseID:    "case-1",
			OutputDir: dir,
		},
		launcherPrompts: mustARBDLauncherPrompts(t, map[string]string{"skill.openclaw": templatePath, "search.remote.enabled": searchPath}),
		mcpPublicBase:   "http://aard.example:8001",
		signingKey:      []byte("01234567890123456789012345678901"),
	}
	if err := state.writeRemoteLawyerSkill("plaintiff"); err != nil {
		t.Fatalf("write remote skill: %v", err)
	}
	path := filepath.Join(dir, "openclaw-plaintiff-lawyer-skill.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	text := string(raw)
	for _, want := range []string{"case=case-1", "role=plaintiff", "http://aard.example:8001/mcp", "Bearer adjmcp1.", "search=Custom remote search instructions."} {
		if !strings.Contains(text, want) {
			t.Fatalf("skill missing %q: %s", want, text)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat skill: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("skill mode = %o", info.Mode().Perm())
	}
	if err := state.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup remote skill: %v", err)
	}
	failureDir := t.TempDir()
	failureState := &runState{
		opts:            state.opts,
		launcherPrompts: state.launcherPrompts,
		mcpPublicBase:   state.mcpPublicBase,
		signingKey:      state.signingKey,
	}
	failureState.opts.OutputDir = failureDir
	failureState.opts.Log = failedLocalRunWriter{}
	if err := failureState.writeRemoteLawyerSkill("plaintiff"); err == nil || !strings.Contains(err.Error(), "write remote lawyer skill log") {
		t.Fatalf("log error = %v", err)
	}
	if err := failureState.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup secrets: %v", err)
	}
	failurePath := filepath.Join(failureDir, "openclaw-plaintiff-lawyer-skill.md")
	if _, err := os.Stat(failurePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remote lawyer skill remains after cleanup: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("unchanged\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatalf("create skill symlink: %v", err)
	}
	symlinkState := &runState{opts: state.opts, launcherPrompts: state.launcherPrompts, mcpPublicBase: state.mcpPublicBase, signingKey: state.signingKey}
	if err := symlinkState.writeRemoteLawyerSkill("plaintiff"); err == nil {
		t.Fatal("remote lawyer skill write followed a symlink")
	}
	raw, err = os.ReadFile(outside)
	if err != nil || string(raw) != "unchanged\n" {
		t.Fatalf("outside file = %q, error = %v", raw, err)
	}
	if err := symlinkState.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup rejected skill: %v", err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("rejected skill path changed: info=%v error=%v", info, err)
	}
}

type failedLocalRunWriter struct{}

func (failedLocalRunWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteRunSummaryIncludesOpenClawStartDelay(t *testing.T) {
	dir := t.TempDir()
	if err := writeRunSummary(dir, Result{
		CaseID: "case-1",
		RunID:  "run-1",
		Status: "ok",
	}, Options{OpenClawStartDelaySeconds: 15, AutoLawyers: "defendant", MCPPublicBaseURL: "http://aard.example:8001"}); err != nil {
		t.Fatalf("write run summary: %v", err)
	}
	summary := readJSONMap(t, filepath.Join(dir, "local-run.json"))
	if summary["openclaw_lawyer_start_delay_seconds"] != float64(15) {
		t.Fatalf("summary = %#v", summary)
	}
	if summary["auto_lawyers"] != "defendant" || summary["mcp_public_base_url"] != "http://aard.example:8001" {
		t.Fatalf("summary = %#v", summary)
	}
	if err := writeRunSummary(dir, Result{CaseID: "changed", RunID: "changed", Status: "failed"}, Options{}); err == nil {
		t.Fatal("writeRunSummary replaced an existing summary")
	}
	summary = readJSONMap(t, filepath.Join(dir, "local-run.json"))
	if summary["case_id"] != "case-1" || summary["status"] != "ok" {
		t.Fatalf("summary changed after rejected replacement: %#v", summary)
	}
}

func TestOutputSubdirReturnsAbsolutePath(t *testing.T) {
	got, err := outputSubdir("relative-out", "pi-C1")
	if err != nil {
		t.Fatalf("output subdir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("path is not absolute: %q", got)
	}
	if !strings.HasSuffix(got, filepath.Join("relative-out", "pi-C1")) {
		t.Fatalf("path = %q", got)
	}
}

func TestStartPiCouncilRejectsPathMemberIDBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	logsDir := t.TempDir()
	state := &runState{opts: Options{OutputDir: outputDir, LogsDir: logsDir}, signingKey: []byte("01234567890123456789012345678901")}
	err := state.startPiCouncil(context.Background(), councilRosterEntry{MemberID: "/../../outside"}, "19780", "opportunity-1")
	if err == nil || !strings.Contains(err.Error(), "member_id") {
		t.Fatalf("error = %v", err)
	}
	for _, dir := range []string{outputDir, logsDir} {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatalf("read %s: %v", dir, readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("invalid member wrote files under %s: %#v", dir, entries)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(outputDir, "pi-C1")); err != nil {
		t.Fatalf("create Pi home symlink: %v", err)
	}
	state.launcherPrompts = mustARBDLauncherPrompts(t, nil)
	state.opts.CaseID = "case-1"
	state.opts.PodmanMCPHost = "127.0.0.1"
	err = state.startPiCouncil(context.Background(), councilRosterEntry{MemberID: "C1"}, "19780", "opportunity-1")
	if err == nil || !strings.Contains(err.Error(), "Pi home") {
		t.Fatalf("symlink Pi home error = %v", err)
	}
	entries, readErr := os.ReadDir(outside)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("outside Pi home entries = %#v, error = %v", entries, readErr)
	}
	owned, err := state.councilHome("C2")
	if err != nil {
		t.Fatalf("claim Pi home: %v", err)
	}
	reused, err := state.councilHome("C2")
	if err != nil || reused != owned {
		t.Fatalf("reuse Pi home = %q, error = %v", reused, err)
	}
	if err := os.Rename(owned, owned+"-replaced"); err != nil {
		t.Fatalf("replace Pi home: %v", err)
	}
	if err := os.Mkdir(owned, 0o755); err != nil {
		t.Fatalf("create replacement Pi home: %v", err)
	}
	if _, err := state.councilHome("C2"); err == nil || !strings.Contains(err.Error(), "was replaced") {
		t.Fatalf("replacement error = %v", err)
	}
}

func TestResolveListenAddrAllocatesPort(t *testing.T) {
	addr, err := resolveListenAddr("0.0.0.0:0", "127.0.0.1")
	if err != nil {
		t.Fatalf("resolve listen addr: %v", err)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split listen addr: %v", err)
	}
	if host != "0.0.0.0" || port == "0" || port == "" {
		t.Fatalf("addr = %q", addr)
	}
}

func TestContainerNameSanitizesAndBounds(t *testing.T) {
	got := piContainerName("case with spaces", "C1@"+strings.Repeat("x", 100), "opportunity-1")
	if len(got) > 63 {
		t.Fatalf("container name length = %d", len(got))
	}
	if strings.ContainsAny(got, "/ @") {
		t.Fatalf("container name = %q", got)
	}
	longPrefix := "case-" + strings.Repeat("x", 100)
	if piContainerName(longPrefix+"-a", "C1", "opportunity-1") == piContainerName(longPrefix+"-b", "C1", "opportunity-1") {
		t.Fatal("distinct long container inputs have the same name")
	}
	args := piRunArgs(Options{PiImage: "pi-image", PiMCPAdapter: "adapter"}, got, "/home", "model", "instructions")
	if !strings.Contains(strings.Join(args, "\n"), "--name\n"+got) {
		t.Fatalf("Pi args do not name container %q: %#v", got, args)
	}
	args, err := addContainerIDPath(args, "/owned/container.cid")
	if err != nil {
		t.Fatalf("add container ID path: %v", err)
	}
	if len(args) < 3 || args[0] != "run" || args[1] != "--cidfile" || args[2] != "/owned/container.cid" {
		t.Fatalf("Pi args do not contain the container ID path: %#v", args)
	}
	if !canonicalContainerID(strings.Repeat("a", 64)) || canonicalContainerID("abc123") || canonicalContainerID(strings.Repeat("A", 64)) {
		t.Fatal("container ID validation accepted a noncanonical value")
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return out
}

func writeCodexAuth(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "auth.json")
	raw := []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"test","refresh_token":"test"}}` + "\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write Codex auth: %v", err)
	}
	return path
}
