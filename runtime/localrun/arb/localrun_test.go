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

	"github.com/agentcourt/adj/internal/launcherprompt"
	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/modelrequest"
)

var pairedCoreBinDir = flag.String("core-bin-dir", "", "Directory containing paired core executables")
var pairedCoreRoot = flag.String("core-root", "", "Paired core checkout root")

func mustARBLauncherPrompts(t *testing.T, overrides map[string]string) launcherprompt.Sources {
	t.Helper()
	prompts, err := launcherprompt.Resolve("arb", "", overrides)
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
	prompts, err := launcherprompt.Resolve("arb", "", map[string]string{"participant.headless": path})
	if err != nil {
		t.Fatal(err)
	}
	state := &runState{launcherPrompts: prompts}
	got, err := state.renderLauncherPrompt("participant.headless", instructionData{
		CaseID:             "case-1",
		RoleID:             "plaintiff",
		MCPServer:          "aar-case-1-plaintiff",
		MCPURL:             "http://example/mcp",
		Workspace:          "/work/plaintiff",
		SearchInstructions: "search enabled",
	})
	if err != nil {
		t.Fatalf("render instructions: %v", err)
	}
	for _, want := range []string{"case=case-1", "role=plaintiff", "server=aar-case-1-plaintiff", "workspace=/work/plaintiff", "search=search enabled"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered instructions missing %q: %s", want, got)
		}
	}
}

func TestHandleLawyerExitAllowsDeliberation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lawyerapi/v1/status" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("case_id") != "case-1" || r.URL.Query().Get("role_id") != "defendant" {
			t.Fatalf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"waiting","phase":"deliberation","case_status":"open","current_opportunity":null}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	state := &runState{
		opts:     Options{CaseID: "case-1"},
		caseBase: server.URL,
	}
	if err := state.handleLawyerExit(context.Background(), "defendant", "openclaw-defendant"); err != nil {
		t.Fatalf("handle exit: %v", err)
	}
}

func TestHandleLawyerExitRejectsAttorneyPhase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"waiting","phase":"closings","case_status":"open","current_opportunity":{"opportunity_id":"closings:plaintiff","role_id":"plaintiff","phase":"closings"}}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	state := &runState{
		opts:     Options{CaseID: "case-1"},
		caseBase: server.URL,
	}
	err := state.handleLawyerExit(context.Background(), "defendant", "openclaw-defendant")
	if err == nil || !strings.Contains(err.Error(), "exited before case completion") {
		t.Fatalf("error = %v", err)
	}
}

func TestWaitForCaseOutcomeReturnsDelayedOutcome(t *testing.T) {
	caseDone := make(chan caseOutcome, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		caseDone <- caseOutcome{result: Result{Status: "ok", Resolution: "demonstrated"}}
	}()
	outcome, ok := waitForCaseOutcome(caseDone, time.Second)
	if !ok {
		t.Fatalf("waitForCaseOutcome returned no outcome")
	}
	if outcome.result.Status != "ok" || outcome.result.Resolution != "demonstrated" {
		t.Fatalf("outcome = %#v", outcome.result)
	}
}

func TestWaitForCaseOutcomeReturnsFalseOnTimeout(t *testing.T) {
	caseDone := make(chan caseOutcome, 1)
	if outcome, ok := waitForCaseOutcome(caseDone, 10*time.Millisecond); ok {
		t.Fatalf("waitForCaseOutcome returned %#v", outcome)
	}
}

func TestCoreCaseArgsUseProcessInterface(t *testing.T) {
	args := coreCaseArgs(Options{
		ComplaintPath:    "/case/complaint.md",
		CaseFiles:        []string{"/case/source-1", "/case/source-2"},
		OutputDir:        "/out",
		CoreOutputDir:    "/out/aar-output",
		PolicyPath:       "/case/policy.json",
		CouncilSize:      3,
		RequiredVotes:    2,
		EvidenceStandard: "preponderance",
		PromptDir:        "/case/prompts",
		PromptFiles: map[string]string{
			"attorney.common":    "/case/common.md",
			"attorney.arguments": "/case/arguments.md",
			"attorney.rebuttals": "/case/rebuttals.md",
		},
		CommonRoot:            "/common",
		CouncilPoolPath:       "/case/pool.jsonl",
		CouncilTimeoutSeconds: 90,
		LawyerTimeoutSeconds:  60,
		MaxResponseBytes:      4096,
		InvalidAttemptLimit:   2,
		EnginePath:            "/bin/aarengine",
		RunID:                 "run-1",
		CaseID:                "case-1",
	}, "127.0.0.1:9001")
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"case", "--complaint\x00/case/complaint.md", "--file\x00/case/source-1",
		"--file\x00/case/source-2", "--out-dir\x00/out/aar-output", "--case-id\x00case-1",
		"--run-id\x00run-1", "--caseapi-addr\x00127.0.0.1:9001",
		"--council-backend\x00councilapi", "--policy\x00/case/policy.json",
		"--required-votes\x002",
		"--engine\x00/bin/aarengine",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("core args lack %q: %#v", want, args)
		}
	}
}

func TestStartCoreCaseUsesEmptyDedicatedOutputDir(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	coreOutputDir := filepath.Join(dir, "aar-output")
	if err := os.MkdirAll(coreOutputDir, 0o755); err != nil {
		t.Fatalf("mkdir core output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local-run.json"), []byte("service file\n"), 0o644); err != nil {
		t.Fatalf("write service file: %v", err)
	}
	core := filepath.Join(dir, "aar-core")
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
if [ -n "$(find "$out_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
  echo "core output directory was not empty" >&2
  exit 73
fi
printf '{"case_id":"%s","run_id":"%s","status":"ok","resolution":"demonstrated","extra":"preserved","selected_environment":"%s"}\n' "$case_id" "$run_id" "${SELECTED_CORE_ENV:-missing}" > "$out_dir/run.json"
printf '{"status":"ok","result":"demonstrated"}\n'
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
		CoreEnvironment: []string{
			"SELECTED_CORE_ENV=present",
		},
	}, "127.0.0.1:9001", logDir)
	if err != nil {
		t.Fatalf("start core case: %v", err)
	}
	outcome := <-done
	if outcome.err != nil {
		t.Fatalf("core outcome: %v", outcome.err)
	}
	if outcome.result.Status != "ok" || outcome.result.Resolution != "demonstrated" {
		t.Fatalf("result = %#v", outcome.result)
	}
	raw, err := json.Marshal(outcome.result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(raw), `"extra":"preserved"`) {
		t.Fatalf("marshaled result lost core fields: %s", raw)
	}
	if !strings.Contains(string(raw), `"selected_environment":"present"`) {
		t.Fatalf("core process did not receive selected environment: %s", raw)
	}
	entries, err := os.ReadDir(coreOutputDir)
	if err != nil {
		t.Fatalf("read core output: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "run.json" {
		t.Fatalf("core output entries = %#v", entries)
	}
}

func TestPairedCoreCaseAPI(t *testing.T) {
	binDir := strings.TrimSpace(*pairedCoreBinDir)
	coreRoot := strings.TrimSpace(*pairedCoreRoot)
	if binDir == "" || coreRoot == "" {
		t.Skip("-core-bin-dir and -core-root are not set")
	}
	coreCommand := filepath.Join(binDir, "aar")
	enginePath := filepath.Join(coreRoot, "arb", "engine", ".lake", "build", "bin", "aarengine")
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
	complaintPath := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaintPath, []byte("# Proposition\n\nThe paired core API starts.\n"), 0o644); err != nil {
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
		"council_size": 1, "required_votes_for_decision": 1, "max_deliberation_rounds": 1,
		"max_opening_chars": 1000, "max_argument_chars": 1000, "max_rebuttal_chars": 1000,
		"max_surrebuttal_chars": 1000, "max_closing_chars": 1000,
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
		CoreWorkingDir:        filepath.Join(coreRoot, "arb"),
		ComplaintPath:         complaintPath,
		OutputDir:             dir,
		CoreOutputDir:         filepath.Join(dir, "aar-output"),
		PolicyPath:            policyPath,
		CommonRoot:            filepath.Join(coreRoot, "common"),
		CouncilPoolPath:       poolPath,
		EnginePath:            enginePath,
		CouncilTimeoutSeconds: 30,
		LawyerTimeoutSeconds:  30,
		RunID:                 "run-paired-arb",
		CaseID:                "paired-arb",
	}, caseAPIAddr, logDir)
	if err != nil {
		cancel()
		t.Fatalf("start paired core: %v", err)
	}
	baseURL := "http://" + caseAPIAddr
	if err := waitForCaseHealth(ctx, baseURL+"/health", "paired-arb", "run-paired-arb", 20*time.Second); err != nil {
		cancel()
		outcome := <-done
		t.Fatalf("wait for paired core API: %v; core outcome: %v", err, outcome.err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/lawyerapi/v1/status?case_id=paired-arb&role_id=plaintiff", nil)
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
	model, err := writePiConfig(home, councilRosterEntry{
		MemberID: "C1",
		RequestSpec: &modelrequest.Spec{
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
			Headers: map[string]string{"X-Test-Request": "arb"},
		},
	}, "aar-case-C1", "http://127.0.0.1:19780/mcp", "adjmcp1.test.signature")
	if err != nil {
		t.Fatalf("write Pi config: %v", err)
	}
	if model != "anthropic/claude-sonnet-4" {
		t.Fatalf("model = %q", model)
	}
	settings := readJSONMap(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	if settings["defaultProvider"] != "openrouter" || settings["defaultModel"] != "anthropic/claude-sonnet-4" {
		t.Fatalf("settings = %#v", settings)
	}
	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	providers := models["providers"].(map[string]any)
	openrouter := providers["openrouter"].(map[string]any)
	modelList := openrouter["models"].([]any)
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
	requestHeaders := openrouter["headers"].(map[string]any)
	if requestHeaders["X-Test-Request"] != "arb" {
		t.Fatalf("provider headers = %#v", requestHeaders)
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
	server := servers["aar-case-C1"].(map[string]any)
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
	_, err := writePiConfig(home, councilRosterEntry{
		MemberID: "C1",
		RequestSpec: &modelrequest.Spec{
			Endpoint: "openrouter",
			Model:    "anthropic/claude-opus-4.6-fast",
		},
	}, "aar-case-C1", "http://127.0.0.1:19780/mcp", "adjmcp1.test.signature")
	if err != nil {
		t.Fatalf("write Pi config: %v", err)
	}
	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	providers := models["providers"].(map[string]any)
	openrouter := providers["openrouter"].(map[string]any)
	modelList := openrouter["models"].([]any)
	modelEntry := modelList[0].(map[string]any)
	want := float64(DefaultCouncilMaxOutputTokens)
	if modelEntry["maxTokens"] != want {
		t.Fatalf("model entry maxTokens = %#v, want %#v", modelEntry["maxTokens"], want)
	}
}

func TestWritePiConfigRejectsMissingRequestSpec(t *testing.T) {
	_, err := writePiConfig(t.TempDir(), councilRosterEntry{
		MemberID: "C1",
		Model:    "openrouter://anthropic/claude-sonnet-4",
	}, "server", "http://example/mcp", "token")
	if err == nil || !strings.Contains(err.Error(), "request_spec") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveOpenClawAuthDefaultsToCodexAuth(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-key")
	path := writeCodexAuth(t, t.TempDir())
	auth, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath: path,
	})
	if err != nil {
		t.Fatalf("resolve OpenClaw auth: %v", err)
	}
	if auth.Mode != "codex" || auth.CodexAuthPath != path {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestResolveOpenClawAuthDoesNotFallBackToAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-key")
	_, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath: filepath.Join(t.TempDir(), "missing-auth.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "read Codex auth file") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveOpenClawAuthRequiresSelectedCredential(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath: filepath.Join(t.TempDir(), "missing-auth.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "read Codex auth file") {
		t.Fatalf("error = %v", err)
	}
	_, err = resolveOpenClawAuth(Options{OpenClawAuth: "api-key"})
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
		PlaintiffLawyer: LawyerProfile{Runner: LawyerCodex, AuthMode: headless.AuthAPIKey, APIKeyEnv: "PLAINTIFF_KEY"},
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

func TestApplyDefaultsUsesIndependentOpenClawLawyers(t *testing.T) {
	opts := applyDefaults(Options{})
	if opts.PlaintiffLawyer.Runner != LawyerOpenClaw || opts.DefendantLawyer.Runner != LawyerOpenClaw {
		t.Fatalf("default lawyer profiles = %#v, %#v", opts.PlaintiffLawyer, opts.DefendantLawyer)
	}
	opts = applyDefaults(Options{
		PlaintiffLawyer: LawyerProfile{Runner: LawyerCodex},
		DefendantLawyer: LawyerProfile{Runner: LawyerClaude},
	})
	if opts.PlaintiffLawyer.Runner != LawyerCodex || opts.DefendantLawyer.Runner != LawyerClaude {
		t.Fatalf("selected lawyer profiles = %#v, %#v", opts.PlaintiffLawyer, opts.DefendantLawyer)
	}
	if !hasAutomaticLawyerRunner(opts, LawyerCodex) || !hasAutomaticLawyerRunner(opts, LawyerClaude) || hasAutomaticLawyerRunner(opts, LawyerOpenClaw) {
		t.Fatalf("automatic runner selection is wrong")
	}
}

func TestApplyDefaultsUsesAndCopiesEnvironments(t *testing.T) {
	participantEnvironment := []string{
		"HOME=/selected/home",
		"PI_CONTAINER_IMAGE=selected-pi",
	}
	coreEnvironment := []string{"OPENROUTER_API_KEY=core"}
	opts := applyDefaults(Options{CoreEnvironment: coreEnvironment, ParticipantEnvironment: participantEnvironment})
	participantEnvironment[0] = "HOME=/changed"
	coreEnvironment[0] = "OPENROUTER_API_KEY=changed"
	if opts.OpenClawCodexAuthPath != "/selected/home/.codex/auth.json" {
		t.Fatalf("Codex auth path = %q", opts.OpenClawCodexAuthPath)
	}
	if opts.PiImage != "selected-pi" {
		t.Fatalf("Pi image = %q", opts.PiImage)
	}
	if got, _ := environmentValue(opts.ParticipantEnvironment, "HOME"); got != "/selected/home" {
		t.Fatalf("copied HOME = %q", got)
	}
	if got, _ := environmentValue(opts.CoreEnvironment, "OPENROUTER_API_KEY"); got != "core" {
		t.Fatalf("copied core credential = %q", got)
	}
}

func TestHeadlessMCPServerNameIsValid(t *testing.T) {
	got := headlessMCPServerName("case/with spaces.and:punctuation", "plaintiff")
	if got != "aar-case-with-spaces-and-punctuation-plaintiff" {
		t.Fatalf("headless MCP server name = %q", got)
	}
}

func TestValidateLawyerProfileAuthentication(t *testing.T) {
	home := t.TempDir()
	codexAuth := filepath.Join(home, "codex-auth.json")
	claudeAuth := filepath.Join(home, "claude-credentials.json")
	if err := os.WriteFile(codexAuth, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeAuth, []byte(`{"claudeAiOauth":{"accessToken":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := applyDefaults(Options{})
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerCodex, CredentialsFile: codexAuth}, opts, []string{"HOME=" + home, "OPENAI_API_KEY=unselected"}); err != nil {
		t.Fatalf("Codex profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerClaude, CredentialsFile: claudeAuth}, opts, []string{"HOME=" + home, "ANTHROPIC_API_KEY=unselected"}); err != nil {
		t.Fatalf("Claude profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{
		Runner:    LawyerPi,
		Model:     "openrouter/anthropic/claude-sonnet-4",
		AuthMode:  headless.AuthAPIKey,
		APIKeyEnv: "SELECTED_PI_KEY",
	}, opts, []string{"HOME=" + home, "SELECTED_PI_KEY=selected"}); err != nil {
		t.Fatalf("Pi profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerPi}, opts, []string{"HOME=" + home, "OPENROUTER_API_KEY=present"}); err == nil || !strings.Contains(err.Error(), "explicit API-key") {
		t.Fatalf("implicit Pi authentication error = %v", err)
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

func TestPrepareOutputLayoutRejectsSymlinkChildren(t *testing.T) {
	for _, child := range []string{"aar-output", "logs"} {
		t.Run(child, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, child)); err != nil {
				t.Fatalf("create symlink: %v", err)
			}
			err := prepareOutputLayout(root, filepath.Join(root, "aar-output"), filepath.Join(root, "logs"))
			if err == nil {
				t.Fatal("output preparation accepted a symlink child")
			}
		})
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

func TestApplyDefaultsCouncilOutputLimit(t *testing.T) {
	if got := applyDefaults(Options{}).CouncilOutputLimitBytes; got != DefaultCouncilOutputLimitBytes {
		t.Fatalf("default council output limit = %d", got)
	}
	if got := applyDefaults(Options{CouncilOutputLimitBytes: 123}).CouncilOutputLimitBytes; got != 123 {
		t.Fatalf("council output limit override = %d", got)
	}
}

func TestPiRunArgsIncludesContainerName(t *testing.T) {
	opts := applyDefaults(Options{PiImage: "agentcourt-pi-sandbox"})
	args := piRunArgs(opts, "aar-case-1-c1", "/tmp/pi-C1", "model-1", "instructions")
	got := "\x00" + strings.Join(args, "\x00") + "\x00"
	for _, want := range []string{
		"\x00run\x00",
		"\x00--rm\x00",
		"\x00--name\x00aar-case-1-c1\x00",
		"\x00--network\x00host\x00",
		"\x00agentcourt-pi-sandbox\x00",
		"\x00--model\x00model-1\x00",
		"\x00-e\x00/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter\x00",
		"\x00-p\x00instructions\x00",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Pi args missing %q in %#v", want, args)
		}
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

func TestCouncilAgentProcessErrorExtractsJSONErrorMessage(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "pi-C1.stdout")
	stderrPath := filepath.Join(dir, "pi-C1.stderr")
	const message = "404 No endpoints found for anthropic/claude-opus-4.6-fast."
	line := fmt.Sprintf(`{"type":"message_start","message":{"stopReason":"error","errorMessage":%q}}`+"\n", message)
	if err := os.WriteFile(stdoutPath, []byte(line), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	if err := os.WriteFile(stderrPath, nil, 0o644); err != nil {
		t.Fatalf("write stderr: %v", err)
	}
	got, details := councilAgentProcessError(&processRecord{
		name:       "pi-C1",
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
	})
	if got != message {
		t.Fatalf("agent error = %q", got)
	}
	if details["agent_error_stream"] != "stdout" || details["agent_error"] != message {
		t.Fatalf("details = %#v", details)
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
	containerID := strings.Repeat("c", 64)
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, "aar-case-1-c1")
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
		stopCommand:     fakeContainerRuntime(t, 0, ""),
		containerIDPath: containerIDPath,
		containerIDDir:  containerIDDir,
		stdoutPath:      stdoutPath,
		stderrPath:      stderrPath,
		finished:        make(chan struct{}),
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
	assertFakeRuntimeLog(t, "container rm -f "+containerID)
}

func TestStopAgentsRemovesOwnedContainerAndPreservesPriorExit(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "client.stdout")
	stderrPath := filepath.Join(dir, "client.stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	cmd := exec.Command("false")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start false: %v", err)
	}
	containerID := strings.Repeat("d", 64)
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, "aar-case-1-c4")
	if err != nil {
		t.Fatalf("create container ID path: %v", err)
	}
	if err := os.WriteFile(containerIDPath, []byte(containerID+"\n"), 0o600); err != nil {
		t.Fatalf("write container ID: %v", err)
	}
	runtimePath := fakeContainerRuntime(t, 0, "")
	proc := &processRecord{
		name:            "pi-C4",
		kind:            "podman",
		command:         cmd,
		done:            make(chan processExit, 1),
		stopCommand:     runtimePath,
		containerIDPath: containerIDPath,
		containerIDDir:  containerIDDir,
		stdoutPath:      stdoutPath,
		stderrPath:      stderrPath,
		finished:        make(chan struct{}),
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
	select {
	case <-proc.finished:
	case <-time.After(2 * time.Second):
		t.Fatalf("process did not exit")
	}
	state := &runState{processes: []*processRecord{proc}}
	if err := state.stopAgents(); err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("stopAgents error = %v", err)
	}
	assertFakeRuntimeLog(t, "container rm -f "+containerID)

	collisionIDPath, collisionIDDir, err := createContainerIDPath(dir, "aar-case-1-c4")
	if err != nil {
		t.Fatalf("reserve collision container ID path: %v", err)
	}
	collisionCmd := exec.Command("sleep", "60")
	if err := collisionCmd.Start(); err != nil {
		t.Fatalf("start collision client: %v", err)
	}
	collision := &processRecord{
		name:            "pi-C4-collision",
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
	assertFakeRuntimeLog(t, "container rm -f "+containerID)
}

func TestStopContainerProcessAcceptsExitedAutoRemovedContainer(t *testing.T) {
	dir := t.TempDir()
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, "aar-case-1-c1")
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
		stopCommand:     fakeContainerRuntime(t, 0, ""),
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

func TestContainerRemovalMissingOutput(t *testing.T) {
	for _, message := range []string{
		"Error response from daemon: No such container: aar-case-1-c1",
		"Error: no container with name or ID \"aar-case-1-c1\" found: no such container",
		"container aar-case-1-c1 does not exist",
	} {
		if !containerRemovalMissingOutput(message) {
			t.Fatalf("missing container output not recognized: %q", message)
		}
	}
	if containerRemovalMissingOutput("permission denied") {
		t.Fatalf("unexpected missing-container match")
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
	t.Setenv("FAKE_CONTAINER_PID", fmt.Sprintf("%d", cmd.Process.Pid))
	runtimePath := fakeContainerRuntime(t, 0, "")
	containerID := strings.Repeat("e", 64)
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, "aar-case-1-c1")
	if err != nil {
		t.Fatalf("create container ID path: %v", err)
	}
	if err := os.WriteFile(containerIDPath, []byte(containerID+"\n"), 0o600); err != nil {
		t.Fatalf("write container ID: %v", err)
	}
	proc := &processRecord{
		name:            "pi-C1-first",
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
	assertFakeRuntimeLog(t, "container rm -f "+containerID)
	if err := state.stopAgents(); err != nil {
		t.Fatalf("stop agents after replacement: %v", err)
	}
}

func fakeContainerRuntime(t *testing.T, exitCode int, output string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "runtime.log")
	t.Setenv("FAKE_CONTAINER_RUNTIME_LOG", logPath)
	t.Setenv("FAKE_CONTAINER_RUNTIME_EXIT", fmt.Sprintf("%d", exitCode))
	t.Setenv("FAKE_CONTAINER_RUNTIME_OUTPUT", output)
	path := filepath.Join(dir, "container-runtime")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_CONTAINER_RUNTIME_LOG"
if [ -n "$FAKE_CONTAINER_RUNTIME_OUTPUT" ]; then
    printf '%s\n' "$FAKE_CONTAINER_RUNTIME_OUTPUT" >&2
fi
if [ -n "${FAKE_CONTAINER_PID:-}" ]; then
    kill "$FAKE_CONTAINER_PID"
fi
exit "$FAKE_CONTAINER_RUNTIME_EXIT"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	return path
}

func assertFakeRuntimeLog(t *testing.T, want string) {
	t.Helper()
	raw, err := os.ReadFile(os.Getenv("FAKE_CONTAINER_RUNTIME_LOG"))
	if err != nil {
		t.Fatalf("read fake runtime log: %v", err)
	}
	got := strings.TrimSpace(string(raw))
	if got != want {
		t.Fatalf("runtime log = %q, want %q", got, want)
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
	logFilter, ok := event["aar_log_filter"].(map[string]any)
	if !ok {
		t.Fatalf("missing aar_log_filter: %#v", event)
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

func TestPiTailLogWriterCounterCountsFilteredBytes(t *testing.T) {
	var out bytes.Buffer
	counter := newProcessOutputCounter(&out)
	writer := newPiTailLogWriter(counter)
	prefix := strings.Repeat("a", 4096)
	first := fmt.Sprintf(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":%q}]}}}`, prefix)
	second := fmt.Sprintf(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"partial":{"responseId":"r1","content":[{"type":"thinking","thinking":%q}]}}}`, prefix+"def")
	raw := []byte(first + "\n" + second + "\n")
	if _, err := writer.Write(raw); err != nil {
		t.Fatalf("write raw log: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush log: %v", err)
	}
	if counter.Size() != int64(out.Len()) {
		t.Fatalf("counter size = %d, log size = %d", counter.Size(), out.Len())
	}
	if counter.Size() >= int64(len(raw)) {
		t.Fatalf("counter counted raw bytes: counter=%d raw=%d", counter.Size(), len(raw))
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
	base, err := publicMCPBase("http://aar.example:8001/", "0.0.0.0:1234")
	if err != nil {
		t.Fatalf("public base: %v", err)
	}
	if base != "http://aar.example:8001" {
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
	templatePath := filepath.Join(dir, "remote.md.tmpl")
	if err := os.WriteFile(templatePath, []byte("case={{CASE_ID}} role={{ROLE_ID}} server={{MCP_SERVER}} url={{MCP_URL}} json={{MCP_JSON}} search={{SEARCH_INSTRUCTIONS}}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	state := &runState{
		opts: Options{
			CaseID:    "case-1",
			OutputDir: dir,
		},
		launcherPrompts: mustARBLauncherPrompts(t, map[string]string{"skill.openclaw": templatePath}),
		mcpPublicBase:   "http://aar.example:8001",
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
	for _, want := range []string{"case=case-1", "role=plaintiff", "http://aar.example:8001/mcp", "Bearer adjmcp1.", "search=Use web search"} {
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
	}, Options{OpenClawStartDelaySeconds: 15, AutoLawyers: "defendant", MCPPublicBaseURL: "http://aar.example:8001"}); err != nil {
		t.Fatalf("write run summary: %v", err)
	}
	summary := readJSONMap(t, filepath.Join(dir, "local-run.json"))
	if summary["openclaw_lawyer_start_delay_seconds"] != float64(15) {
		t.Fatalf("summary = %#v", summary)
	}
	if summary["auto_lawyers"] != "defendant" || summary["mcp_public_base_url"] != "http://aar.example:8001" {
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
	state.launcherPrompts = mustARBLauncherPrompts(t, nil)
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
	got := containerName("AAR/case with spaces and @ symbols " + strings.Repeat("x", 100))
	if len(got) > 63 {
		t.Fatalf("container name length = %d", len(got))
	}
	if strings.ContainsAny(got, "/ @") {
		t.Fatalf("container name = %q", got)
	}
	longPrefix := "aar-" + strings.Repeat("x", 100)
	if containerName(longPrefix+"-a") == containerName(longPrefix+"-b") {
		t.Fatal("distinct long container inputs have the same name")
	}
	args, err := addContainerIDPath([]string{"run", "--name", got}, "/owned/container.cid")
	if err != nil {
		t.Fatalf("add container ID path: %v", err)
	}
	if len(args) < 3 || args[0] != "run" || args[1] != "--cidfile" || args[2] != "/owned/container.cid" {
		t.Fatalf("container args do not contain the container ID path: %#v", args)
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
