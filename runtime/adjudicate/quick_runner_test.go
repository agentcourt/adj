package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentcourt/adj/internal/launcherprompt"
	lawyerlaunch "github.com/agentcourt/adj/internal/lawyer"
	"github.com/agentcourt/adj/internal/mcpcap"
	"github.com/agentcourt/adj/internal/mcpchild"
)

type fakeQuickCore struct {
	done chan coreProcessOutcome
}

func (p *fakeQuickCore) Done() <-chan coreProcessOutcome { return p.done }

type fakeQuickMCP struct {
	address string
	done    chan error
}

func (p *fakeQuickMCP) Address() string    { return p.address }
func (p *fakeQuickMCP) Done() <-chan error { return p.done }

type fakeQuickSupervisor struct {
	mu          sync.Mutex
	assignments []lawyerlaunch.Assignment
	errors      chan error
	onStart     func()
	waitErr     error
	stopErr     error
}

func (s *fakeQuickSupervisor) Start(_ context.Context, _ lawyerlaunch.Profile, assignment lawyerlaunch.Assignment) error {
	s.mu.Lock()
	s.assignments = append(s.assignments, assignment)
	count := len(s.assignments)
	s.mu.Unlock()
	if count == 2 && s.onStart != nil {
		s.onStart()
	}
	return nil
}

func (s *fakeQuickSupervisor) Errors() <-chan error       { return s.errors }
func (s *fakeQuickSupervisor) Wait(context.Context) error { return s.waitErr }
func (s *fakeQuickSupervisor) Stop() error                { return s.stopErr }

func writeQuickTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("write test JSON: %v", err)
	}
}

func TestQuickRunnerStartsLawyersSequentiallyAndMapsResult(t *testing.T) {
	t.Setenv("QUICK_OPENROUTER_KEY", "openrouter-secret")
	t.Setenv("OPENAI_API_KEY", "unselected-openai-secret")
	recordDir := t.TempDir()
	for _, name := range []string{"core", "logs", filepath.Join("inputs", "documents")} {
		if err := os.MkdirAll(filepath.Join(recordDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	core := &fakeQuickCore{done: make(chan coreProcessOutcome, 1)}
	supervisor := &fakeQuickSupervisor{errors: make(chan error, 1)}
	supervisor.onStart = func() {
		core.done <- coreProcessOutcome{result: []byte(`{"status":"ok","phase":"closed","resolution":"demonstrated","arguments":[{"role":"plaintiff","opportunity_id":"arguments:plaintiff"},{"role":"defendant","opportunity_id":"arguments:defendant"}],"provider":{"request_count":3,"usage_observed_count":0,"cost_observed_count":0}}`)}
	}
	var coreRequest coreProcessRequest
	var mcpRequest mcpchild.Request
	var supervisorRuntime lawyerlaunch.Runtime
	var coreCaseAPITokenMu sync.RWMutex
	var coreCaseAPIToken string
	var coreCaseAPITokenPath string
	defendantWaitStarted := make(chan struct{})
	releaseDefendantWait := make(chan struct{})
	var defendantWaitStartedOnce sync.Once
	var releaseDefendantWaitOnce sync.Once
	var waitMu sync.Mutex
	waitCalls := 0
	caseAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/lawyerapi/v1/wait" {
			http.NotFound(w, req)
			return
		}
		coreCaseAPITokenMu.RLock()
		wantAuthorization := "Bearer " + coreCaseAPIToken
		coreCaseAPITokenMu.RUnlock()
		if req.Header.Get("Authorization") != wantAuthorization {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if req.URL.Query().Get("case_id") != "case-1" || req.URL.Query().Get("role_id") != "defendant" || req.URL.Query().Get("timeout_ms") != "30000" {
			t.Errorf("wait query = %q", req.URL.RawQuery)
		}
		waitMu.Lock()
		waitCalls++
		call := waitCalls
		waitMu.Unlock()
		switch call {
		case 1:
			if got := req.URL.Query().Get("after_version"); got != "" {
				t.Errorf("first wait after_version = %q", got)
			}
			writeQuickTestJSON(t, w, map[string]any{
				"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 11, "status": "waiting",
				"turn": map[string]any{"role_id": "plaintiff", "opportunity_id": "arguments:plaintiff", "completed": false},
			})
		case 2:
			if got := req.URL.Query().Get("after_version"); got != "11" {
				t.Errorf("second wait after_version = %q, want 11", got)
			}
			defendantWaitStartedOnce.Do(func() { close(defendantWaitStarted) })
			<-releaseDefendantWait
			writeQuickTestJSON(t, w, map[string]any{
				"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 12, "status": "ready",
				"turn": map[string]any{"role_id": "defendant", "opportunity_id": "arguments:defendant", "completed": false},
			})
		default:
			t.Errorf("wait request count = %d", call)
			http.Error(w, "unexpected wait", http.StatusInternalServerError)
		}
	}))
	defer caseAPI.Close()
	defer releaseDefendantWaitOnce.Do(func() { close(releaseDefendantWait) })
	runner := QuickRunner{
		startCore: func(_ context.Context, request coreProcessRequest) (coreProcessWaiter, error) {
			coreRequest = request
			var ok bool
			coreCaseAPITokenPath, ok = argumentValue(request.Args, "--lawyerapi-bearer-token-file")
			if !ok {
				return nil, errors.New("core bearer-token file argument is absent")
			}
			info, err := os.Stat(coreCaseAPITokenPath)
			if err != nil {
				return nil, err
			}
			if info.Mode().Perm() != 0o600 {
				return nil, errors.New("core bearer-token file permissions are not 0600")
			}
			raw, err := os.ReadFile(coreCaseAPITokenPath)
			if err != nil {
				return nil, err
			}
			coreCaseAPITokenMu.Lock()
			coreCaseAPIToken = string(raw)
			coreCaseAPITokenMu.Unlock()
			if len(string(raw)) < 40 || slices.Contains(request.Args, string(raw)) {
				return nil, errors.New("core bearer token has invalid length or appears in argv")
			}
			runtime := quickRuntimeRecord{CaseID: "case-1", RunID: "run-1", CaseAPIBase: caseAPI.URL}
			if err := WriteJSONAtomic(filepath.Join(recordDir, "core", "runtime.json"), runtime); err != nil {
				return nil, err
			}
			return core, nil
		},
		startMCP: func(ctx context.Context, request mcpchild.Request) (quickMCPProcess, error) {
			mcpRequest = request
			process := &fakeQuickMCP{address: "127.0.0.1:3210", done: make(chan error, 1)}
			go func() {
				<-ctx.Done()
				process.done <- nil
			}()
			return process, nil
		},
		newSupervisor: func(runtime lawyerlaunch.Runtime) (lawyerSupervisor, error) {
			supervisorRuntime = runtime
			return supervisor, nil
		},
		probeHealth: func(context.Context, string) error { return nil },
		probeCore:   func(context.Context, string, string, string) error { return nil },
	}
	settings := ResolvedSettings{
		Common: CommonSettings{
			EvidenceStandard: "preponderance_of_the_evidence",
			CouncilPool:      "/pool.jsonl",
			CouncilSize:      3,
			RequiredVotes:    2,
			DocumentLimits:   DocumentLimits{Count: 2, PerFile: 1024, Total: 2048},
			ProviderCredentials: map[string]CredentialMetadata{
				"openai":     {Source: AuthAPIKey, EnvironmentVariable: "QUICK_OPENAI_KEY"},
				"openrouter": {Source: AuthAPIKey, EnvironmentVariable: "QUICK_OPENROUTER_KEY"},
			},
		},
		AgentProfiles: map[string]ResolvedAgentProfile{
			"lawyer": {Runner: RunnerCodex, Resume: true, Authentication: Authentication{Source: AuthSubscription, Path: "/credentials/codex.json"}},
		},
		Procedure: ResolvedProcedureSettings{Quick: &ResolvedQuickSettings{
			CoreCommand:      "quick",
			MCPCommand:       "/bin/quick-mcp",
			MCPWorkingDir:    "/work/mcp",
			PlaintiffProfile: "lawyer",
			DefendantProfile: "lawyer",
			WebSearch:        true,
			PromptDir:        "/prompts/quick",
			PromptFiles: PromptFilePaths{
				"lawyer.common":            "/prompts/common.md",
				"lawyer.proponent":         "/prompts/for.md",
				"lawyer.opponent":          "/prompts/against.md",
				"search.enabled":           "/prompts/search-on.md",
				"search.disabled":          "/prompts/search-off.md",
				"council.system":           "/prompts/council.md",
				"mcp.session.instructions": "/prompts/mcp-session.md",
				"mcp.tool.get_case":        "/prompts/mcp-get-case.md",
			},
		}},
	}
	type runResult struct {
		outcome ProcedureOutcome
		err     error
	}
	runDone := make(chan runResult, 1)
	go func() {
		outcome, err := runner.Run(context.Background(), ProcedureRequest{
			Request:    Request{CaseID: "case-1", RunID: "run-1", Proposition: "P"},
			Settings:   settings,
			RecordDir:  recordDir,
			CoreDir:    filepath.Join(recordDir, "core"),
			LogsDir:    filepath.Join(recordDir, "logs"),
			SessionDir: filepath.Join(recordDir, "session"),
		})
		runDone <- runResult{outcome: outcome, err: err}
	}()
	select {
	case <-defendantWaitStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("Quick runner did not wait for the defendant opportunity")
	}
	supervisor.mu.Lock()
	assignmentsBeforeReady := append([]lawyerlaunch.Assignment(nil), supervisor.assignments...)
	supervisor.mu.Unlock()
	if len(assignmentsBeforeReady) != 1 || assignmentsBeforeReady[0].RoleID != "plaintiff" {
		t.Fatalf("assignments before defendant readiness = %+v", assignmentsBeforeReady)
	}
	releaseDefendantWaitOnce.Do(func() { close(releaseDefendantWait) })
	var completed runResult
	select {
	case completed = <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Quick runner did not finish after defendant readiness")
	}
	if completed.err != nil {
		t.Fatal(completed.err)
	}
	outcome := completed.outcome
	if outcome.Status != StatusOK || outcome.Phase != "closed" || outcome.Decision == nil || outcome.Decision.Value != "demonstrated" {
		t.Fatalf("outcome = %+v", outcome)
	}
	if outcome.Provider.RequestCount != 3 {
		t.Fatalf("provider accounting = %#v", outcome.Provider)
	}
	supervisor.mu.Lock()
	assignments := append([]lawyerlaunch.Assignment(nil), supervisor.assignments...)
	supervisor.mu.Unlock()
	if len(assignments) != 2 {
		t.Fatalf("assignment count = %d", len(assignments))
	}
	if assignments[0].RoleID != "plaintiff" || assignments[1].RoleID != "defendant" {
		t.Fatalf("assignment roles = %q, %q", assignments[0].RoleID, assignments[1].RoleID)
	}
	if assignments[1].MCP.URL != "http://127.0.0.1:3210/mcp" || assignments[1].VerifyExit == nil {
		t.Fatalf("defendant MCP URL = %q; exit verifier present = %t", assignments[1].MCP.URL, assignments[1].VerifyExit != nil)
	}
	if assignments[0].MCP.URL != assignments[1].MCP.URL || assignments[0].MCP.BearerToken == assignments[1].MCP.BearerToken {
		t.Fatal("quick MCP assignments do not have one endpoint and distinct capabilities")
	}
	for _, assignment := range assignments {
		wantCapability, err := mcpcap.Issue(mcpRequest.SigningKey, mcpcap.Claims{
			Audience:       "quick",
			CaseID:         "case-1",
			AssignmentType: "lawyer",
			PrincipalID:    assignment.RoleID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if assignment.MCP.BearerToken != wantCapability {
			t.Fatalf("%s capability does not bind its assignment", assignment.RoleID)
		}
	}
	for _, assignment := range assignments {
		if assignment.RunID != "run-1" {
			t.Fatalf("%s assignment run ID = %q", assignment.RoleID, assignment.RunID)
		}
		if assignment.WebSearch == nil || !*assignment.WebSearch {
			t.Fatalf("%s assignment web search = %#v", assignment.RoleID, assignment.WebSearch)
		}
		if !strings.Contains(assignment.Prompt, "wait_for_opportunity") {
			t.Fatalf("%s assignment lacks startup instructions: %q", assignment.RoleID, assignment.Prompt)
		}
		if !strings.Contains(assignment.Prompt, assignment.WorkDir) || !strings.Contains(assignment.Prompt, assignment.EvidenceDir) {
			t.Fatalf("%s assignment lacks its workspace or evidence path: %q", assignment.RoleID, assignment.Prompt)
		}
	}
	wantEvidenceDir := filepath.Join(recordDir, "inputs", "documents")
	if assignments[0].StateDir != filepath.Join(recordDir, "session", "lawyer", "plaintiff") || assignments[0].WorkDir != filepath.Join(recordDir, "work", "plaintiff") || assignments[0].EvidenceDir != wantEvidenceDir {
		t.Fatalf("plaintiff state directory = %q, work directory = %q", assignments[0].StateDir, assignments[0].WorkDir)
	}
	if assignments[1].StateDir != filepath.Join(recordDir, "session", "lawyer", "defendant") || assignments[1].WorkDir != filepath.Join(recordDir, "work", "defendant") || assignments[1].EvidenceDir != wantEvidenceDir {
		t.Fatalf("defendant state directory = %q, work directory = %q", assignments[1].StateDir, assignments[1].WorkDir)
	}
	if slices.Contains(coreRequest.Args, "--parallel-council") {
		t.Fatalf("default quick args enable parallel council: %v", coreRequest.Args)
	}
	for flag, want := range map[string]string{
		"--council-pool":           "/pool.jsonl",
		"--council-size":           "3",
		"--required-votes":         "2",
		"--evidence-standard":      "preponderance_of_the_evidence",
		"--lawyer-web-search=true": "",
		"--prompt-dir":             "/prompts/quick",
	} {
		if want == "" {
			if !slices.Contains(coreRequest.Args, flag) {
				t.Fatalf("%s is absent from %v", flag, coreRequest.Args)
			}
			continue
		}
		if got, ok := argumentValue(coreRequest.Args, flag); !ok || got != want {
			t.Fatalf("%s = %q, present = %t, want %q", flag, got, ok, want)
		}
	}
	for _, value := range []string{
		"council.system=/prompts/council.md",
		"lawyer.common=/prompts/common.md",
		"lawyer.opponent=/prompts/against.md",
		"lawyer.proponent=/prompts/for.md",
		"search.disabled=/prompts/search-off.md",
		"search.enabled=/prompts/search-on.md",
	} {
		if !slices.Contains(coreRequest.Args, value) {
			t.Fatalf("prompt override %q is absent from %v", value, coreRequest.Args)
		}
	}
	for _, value := range []string{"mcp.session.instructions=/prompts/mcp-session.md", "mcp.tool.get_case=/prompts/mcp-get-case.md"} {
		if slices.Contains(coreRequest.Args, value) {
			t.Fatalf("MCP prompt override %q was sent to the Quick core: %v", value, coreRequest.Args)
		}
	}
	if mcpRequest.PromptFiles["mcp.session.instructions"] != "/prompts/mcp-session.md" || mcpRequest.PromptFiles["mcp.tool.get_case"] != "/prompts/mcp-get-case.md" {
		t.Fatalf("MCP prompt overrides = %#v", mcpRequest.PromptFiles)
	}
	if mcpRequest.Command != "/bin/quick-mcp" || mcpRequest.WorkingDir != "/work/mcp" || mcpRequest.Name != "quick-mcp" || !slices.Equal(mcpRequest.CommandArgs, []string{"serve"}) {
		t.Fatalf("MCP process command = %q, args = %v, directory = %q, name = %q", mcpRequest.Command, mcpRequest.CommandArgs, mcpRequest.WorkingDir, mcpRequest.Name)
	}
	if mcpRequest.ListenAddr != "127.0.0.1:0" || mcpRequest.CaseAPIBase != caseAPI.URL || mcpRequest.PromptDir != "/prompts/quick" || len(mcpRequest.SigningKey) != 32 {
		t.Fatalf("MCP listen address = %q, case API base = %q, prompt directory = %q, signing-key length = %d", mcpRequest.ListenAddr, mcpRequest.CaseAPIBase, mcpRequest.PromptDir, len(mcpRequest.SigningKey))
	}
	if string(mcpRequest.CaseAPIBearerToken) != coreCaseAPIToken {
		t.Fatal("Quick MCP did not receive the core bearer token")
	}
	if _, err := os.Stat(coreCaseAPITokenPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("core bearer-token file remains after core readiness: %v", err)
	}
	for _, assignment := range assignments {
		if assignment.MCP.BearerToken == coreCaseAPIToken || strings.Contains(assignment.Prompt, coreCaseAPIToken) {
			t.Fatalf("core bearer token was exposed to %s lawyer", assignment.RoleID)
		}
	}
	for _, name := range []string{"QUICK_OPENROUTER_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		if _, ok := environmentValue(mcpRequest.Environment, name); ok {
			t.Fatalf("MCP environment contains provider credential %s", name)
		}
	}
	for _, path := range []string{filepath.Join(recordDir, "core", "quick-mcp.token"), filepath.Join(recordDir, "core", "quick-mcp-ready.json")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ephemeral MCP file %q remains: %v", path, err)
		}
	}
	if value, ok := environmentValue(coreRequest.Env, "OPENAI_API_KEY"); ok {
		t.Fatalf("unselected OPENAI_API_KEY remains in the environment with value %q", value)
	}
	if value, ok := environmentValue(coreRequest.Env, "OPENROUTER_API_KEY"); !ok || value != "openrouter-secret" {
		t.Fatalf("OPENROUTER_API_KEY was not selected")
	}
	for _, name := range []string{"QUICK_OPENROUTER_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		if _, ok := environmentValue(supervisorRuntime.BaseEnvironment, name); ok {
			t.Fatalf("participant environment contains provider credential %s", name)
		}
	}
}

func argumentValue(arguments []string, flag string) (string, bool) {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == flag {
			return arguments[index+1], true
		}
	}
	return "", false
}

func TestMapQuickCoreOutcomePreservesProviderClass(t *testing.T) {
	providerErr := errors.New("process failed")
	_, err := mapQuickCoreOutcome(coreProcessOutcome{
		result: []byte(`{"status":"failed","phase":"failed","error":"authentication failed","error_class":"provider_authentication","provider":{"request_count":1,"usage_observed_count":0,"cost_observed_count":0}}`),
		err:    providerErr,
	})
	var coreErr *CoreRunError
	if !errors.As(err, &coreErr) || coreErr.ErrorClass() != "provider_authentication" || !errors.Is(err, providerErr) {
		t.Fatalf("error = %v", err)
	}
	if coreErr.Provider.RequestCount != 1 {
		t.Fatalf("provider accounting = %#v", coreErr.Provider)
	}
}

func TestWaitForQuickRuntimeRetainsLastProbeError(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "runtime.json")
	if err := WriteJSONAtomic(runtimePath, quickRuntimeRecord{
		CaseID:      "case-1",
		RunID:       "run-1",
		CaseAPIBase: "http://127.0.0.1:1",
	}); err != nil {
		t.Fatal(err)
	}
	probeErr := errors.New("health identity mismatch")
	ctx, cancel := context.WithCancel(context.Background())
	runner := QuickRunner{probeCore: func(context.Context, string, string, string) error {
		cancel()
		return probeErr
	}}
	_, _, err := runner.waitForQuickRuntime(ctx, make(chan coreProcessOutcome), runtimePath, "case-1", "run-1")
	if !errors.Is(err, probeErr) || !errors.Is(err, context.Canceled) {
		t.Fatalf("startup error = %v", err)
	}
}

func TestMapQuickCoreOutcomePreservesAccountingForInvalidResult(t *testing.T) {
	_, err := mapQuickCoreOutcome(coreProcessOutcome{
		result: []byte(`{"status":"ok","phase":"closed","resolution":"invalid","provider":{"request_count":2,"usage_observed_count":0,"cost_observed_count":0}}`),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid resolution") {
		t.Fatalf("error = %v", err)
	}
	if providerAccountingFromError(err).RequestCount != 2 {
		t.Fatalf("provider accounting = %#v", providerAccountingFromError(err))
	}
}

func TestQuickResultContainsFiledRole(t *testing.T) {
	direct := []byte(`{"arguments":[{"role":"plaintiff","opportunity_id":"arguments:plaintiff"}]}`)
	plaintiff, err := quickResultContainsArgument(direct, "plaintiff")
	if err != nil {
		t.Fatal(err)
	}
	defendant, err := quickResultContainsArgument(direct, "defendant")
	if err != nil {
		t.Fatal(err)
	}
	if !plaintiff || defendant {
		t.Fatalf("direct result role detection failed")
	}
	wrapped := []byte(`{"result":{"arguments":[{"role":"defendant","opportunity_id":"arguments:defendant"}]}}`)
	defendant, err = quickResultContainsArgument(wrapped, "defendant")
	if err != nil {
		t.Fatal(err)
	}
	if !defendant {
		t.Fatalf("wrapped result role detection failed")
	}
	wrongOpportunity := []byte(`{"arguments":[{"role":"plaintiff","opportunity_id":"arguments:defendant"}]}`)
	plaintiff, err = quickResultContainsArgument(wrongOpportunity, "plaintiff")
	if err != nil {
		t.Fatal(err)
	}
	if plaintiff {
		t.Fatalf("wrong opportunity was accepted")
	}
}

func TestReadQuickResultUsesCoreBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer core-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	raw, err := readQuickResult(context.Background(), server.URL, "core-secret")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("result = %s", raw)
	}
}

func TestWaitForQuickLawyerOpportunityUsesStateVersion(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer core-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if req.URL.Path != "/lawyerapi/v1/wait" || req.URL.Query().Get("case_id") != "case-1" || req.URL.Query().Get("role_id") != "defendant" || req.URL.Query().Get("timeout_ms") != "30000" {
			t.Errorf("wait request = %s?%s", req.URL.Path, req.URL.RawQuery)
		}
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		switch call {
		case 1:
			if got := req.URL.Query().Get("after_version"); got != "" {
				t.Errorf("first after_version = %q", got)
			}
			writeQuickTestJSON(t, w, map[string]any{
				"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 8, "status": "waiting",
				"turn": map[string]any{"role_id": "plaintiff", "opportunity_id": "arguments:plaintiff", "completed": false},
			})
		case 2:
			if got := req.URL.Query().Get("after_version"); got != "8" {
				t.Errorf("second after_version = %q, want 8", got)
			}
			writeQuickTestJSON(t, w, map[string]any{
				"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 9, "status": "ready",
				"turn": map[string]any{"role_id": "defendant", "opportunity_id": "arguments:defendant", "completed": false},
			})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	if err := waitForQuickLawyerOpportunity(context.Background(), server.URL, "case-1", "defendant", "core-secret"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("wait calls = %d, want 2", calls)
	}
}

func TestWaitForQuickLawyerOpportunityRetriesRequestDeadline(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		switch call {
		case 1:
			writeQuickTestJSON(t, w, map[string]any{
				"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 8, "status": "waiting",
				"turn": map[string]any{"role_id": "plaintiff", "opportunity_id": "arguments:plaintiff", "completed": false},
			})
		case 2:
			if got := req.URL.Query().Get("after_version"); got != "8" {
				t.Errorf("timed-out wait after_version = %q, want 8", got)
			}
			<-req.Context().Done()
		case 3:
			if got := req.URL.Query().Get("after_version"); got != "8" {
				t.Errorf("retried wait after_version = %q, want 8", got)
			}
			writeQuickTestJSON(t, w, map[string]any{
				"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 9, "status": "ready",
				"turn": map[string]any{"role_id": "defendant", "opportunity_id": "arguments:defendant", "completed": false},
			})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitForQuickLawyerOpportunityWithTimeouts(ctx, server.URL, "case-1", "defendant", "core-secret", 10*time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Fatalf("wait calls = %d, want 3", calls)
	}
}

func TestWaitForQuickLawyerOpportunityHonorsParentDeadline(t *testing.T) {
	requestStarted := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		once.Do(func() { close(requestStarted) })
		<-req.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- waitForQuickLawyerOpportunityWithTimeouts(ctx, server.URL, "case-1", "defendant", "core-secret", 50*time.Millisecond, time.Second)
	}()
	<-requestStarted
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("parent deadline error = %v", err)
	}
}

func TestWaitForQuickLawyerOpportunityRejectsInvalidResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "not ok", body: `{"ok":false,"case_id":"case-1","role_id":"defendant","status":"waiting","state_version":1}`, want: "not ok"},
		{name: "wrong identity", body: `{"ok":true,"case_id":"other","role_id":"defendant","status":"waiting","state_version":1}`, want: "identifies case"},
		{name: "ready without turn", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"ready"}`, want: "without a turn"},
		{name: "wrong turn role", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"ready","turn":{"role_id":"plaintiff","opportunity_id":"arguments:defendant","completed":false}}`, want: "turn role"},
		{name: "wrong opportunity", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"ready","turn":{"role_id":"defendant","opportunity_id":"arguments:plaintiff","completed":false}}`, want: "turn role"},
		{name: "missing completed", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"ready","turn":{"role_id":"defendant","opportunity_id":"arguments:defendant"}}`, want: "no active turn"},
		{name: "completed turn", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"ready","turn":{"role_id":"defendant","opportunity_id":"arguments:defendant","completed":true}}`, want: "no active turn"},
		{name: "waiting without version", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"waiting"}`, want: "without state_version"},
		{name: "invalid status", body: `{"ok":true,"case_id":"case-1","role_id":"defendant","status":"observing"}`, want: "invalid status"},
		{name: "malformed JSON", body: `{`, want: "decode quick defendant lawyer wait response"},
		{name: "HTTP error", statusCode: http.StatusUnauthorized, body: `unauthorized`, want: "returned HTTP 401"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				statusCode := test.statusCode
				if statusCode == 0 {
					statusCode = http.StatusOK
				}
				w.WriteHeader(statusCode)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := waitForQuickLawyerOpportunity(ctx, server.URL, "case-1", "defendant", "core-secret")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestWaitForQuickLawyerOpportunityLeavesTerminalResultToCore(t *testing.T) {
	responseSent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeQuickTestJSON(t, w, map[string]any{
			"ok": true, "case_id": "case-1", "role_id": "defendant", "state_version": 3, "status": "failed",
		})
		close(responseSent)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- waitForQuickLawyerOpportunity(ctx, server.URL, "case-1", "defendant", "core-secret")
	}()
	<-responseSent
	select {
	case err := <-done:
		t.Fatalf("terminal wait returned before core lifecycle ended: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("terminal wait error = %v", err)
	}
}

func TestWaitForQuickOpportunityStageHandlesLifecycle(t *testing.T) {
	newChannels := func() (chan coreProcessOutcome, chan error, chan error, chan error) {
		return make(chan coreProcessOutcome, 1), make(chan error, 1), make(chan error, 1), make(chan error, 1)
	}
	t.Run("ready", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		ready <- nil
		result := waitForQuickOpportunityStage(context.Background(), coreDone, mcpDone, supervisorErrors, ready)
		if result.coreFinal != nil || result.mcpExited || result.err != nil {
			t.Fatalf("stage result = %+v", result)
		}
	})
	t.Run("readiness error", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		want := errors.New("wait failed")
		ready <- want
		result := waitForQuickOpportunityStage(context.Background(), coreDone, mcpDone, supervisorErrors, ready)
		if !errors.Is(result.err, want) {
			t.Fatalf("stage error = %v", result.err)
		}
	})
	t.Run("core exit", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		want := coreProcessOutcome{result: []byte(`{"status":"failed"}`), err: errors.New("core failed")}
		coreDone <- want
		result := waitForQuickOpportunityStage(context.Background(), coreDone, mcpDone, supervisorErrors, ready)
		if result.coreFinal == nil || !errors.Is(result.coreFinal.err, want.err) || result.mcpExited || result.err != nil {
			t.Fatalf("stage result = %+v", result)
		}
	})
	t.Run("MCP exit", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		mcpDone <- nil
		result := waitForQuickOpportunityStage(context.Background(), coreDone, mcpDone, supervisorErrors, ready)
		if !result.mcpExited || result.err == nil || result.err.Error() != "quick MCP exited before core completion" {
			t.Fatalf("stage result = %+v", result)
		}
	})
	t.Run("MCP failure", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		want := errors.New("MCP failed")
		mcpDone <- want
		result := waitForQuickOpportunityStage(context.Background(), coreDone, mcpDone, supervisorErrors, ready)
		if !result.mcpExited || !errors.Is(result.err, want) {
			t.Fatalf("stage result = %+v", result)
		}
	})
	t.Run("supervisor error", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		want := errors.New("lawyer failed")
		supervisorErrors <- want
		result := waitForQuickOpportunityStage(context.Background(), coreDone, mcpDone, supervisorErrors, ready)
		if !errors.Is(result.err, want) || result.mcpExited || result.coreFinal != nil {
			t.Fatalf("stage result = %+v", result)
		}
	})
	t.Run("context cancellation", func(t *testing.T) {
		coreDone, mcpDone, supervisorErrors, ready := newChannels()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result := waitForQuickOpportunityStage(ctx, coreDone, mcpDone, supervisorErrors, ready)
		if !errors.Is(result.err, context.Canceled) || result.mcpExited || result.coreFinal != nil {
			t.Fatalf("stage result = %+v", result)
		}
	})
}

func TestQuickParticipantPromptFilesAndReplacement(t *testing.T) {
	dir := t.TempDir()
	participantPath := filepath.Join(dir, "participant.md")
	if err := os.WriteFile(participantPath, []byte("role={{ROLE}} case={{CASE}} server={{SERVER}} workspace={{WORKSPACE}} evidence={{EVIDENCE_DIR}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompts, err := launcherprompt.Resolve("quick", "", map[string]string{"participant": participantPath})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := quickParticipantPrompt(prompts, "plaintiff", "case-1", "server-1", "/host/work", "/host/evidence", false)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "role=plaintiff case=case-1 server=server-1 workspace=/host/work evidence=/host/evidence" {
		t.Fatalf("prompt = %q", prompt)
	}

	if err := os.WriteFile(participantPath, []byte("{{UNKNOWN}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := launcherprompt.Resolve("quick", "", map[string]string{"participant": participantPath}); err == nil || !strings.Contains(err.Error(), "unsupported token") {
		t.Fatalf("unsupported-token error = %v", err)
	}
	if _, err := launcherprompt.Resolve("quick", "", map[string]string{"participant": filepath.Join(dir, "missing.md")}); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing explicit file error = %v", err)
	}

	prompts, err = launcherprompt.Resolve("quick", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := quickParticipantPrompt(prompts, "defendant", "case-{{record}}", "server-2", "/work", "/evidence", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallback, "defendant participant") || !strings.Contains(fallback, "case-{{record}}") || !strings.Contains(fallback, "/work") || !strings.Contains(fallback, "/evidence") {
		t.Fatalf("fallback prompt = %q", fallback)
	}

	for _, test := range []struct {
		runner        AgentRunner
		wantWorkspace string
		wantEvidence  string
	}{
		{runner: RunnerCodex, wantWorkspace: "/host/work", wantEvidence: "/host/evidence"},
		{runner: RunnerClaude, wantWorkspace: "/host/work", wantEvidence: "/host/evidence"},
		{runner: RunnerPi, wantWorkspace: lawyerlaunch.PiWorkspacePath, wantEvidence: lawyerlaunch.PiEvidencePath},
		{runner: RunnerOpenClaw, wantWorkspace: lawyerlaunch.OpenClawWorkspacePath, wantEvidence: lawyerlaunch.OpenClawEvidencePath},
	} {
		workspace, evidence := quickLauncherPaths(test.runner, "/host/work", "/host/evidence")
		if workspace != test.wantWorkspace || evidence != test.wantEvidence {
			t.Fatalf("%s launcher paths = %q, %q", test.runner, workspace, evidence)
		}
	}
}

func TestQuickRemoteLawyerSkill(t *testing.T) {
	prompts, err := launcherprompt.Resolve("quick", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	path, err := writeQuickRemoteLawyerSkill(t.TempDir(), "defendant", "case-1", "http://127.0.0.1:3210", "capability-1", true, prompts)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("remote skill mode = %o", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"http://127.0.0.1:3210/mcp", "Bearer capability-1", "Use web search"} {
		if !strings.Contains(string(raw), required) {
			t.Fatalf("remote skill lacks %q: %s", required, raw)
		}
	}

	if _, err := quickPublicMCPBase("", "0.0.0.0:3210", "none"); err == nil {
		t.Fatal("wildcard manual MCP address was accepted without a public base URL")
	}
	base, err := quickPublicMCPBase("", "127.0.0.1:3210", "none")
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://127.0.0.1:3210" {
		t.Fatalf("manual MCP base = %q", base)
	}
}

func TestResolvedLawyerLaunchProfileMapsAuthentication(t *testing.T) {
	profile, err := resolvedLawyerLaunchProfile(ResolvedAgentProfile{
		Runner:          RunnerCodex,
		Command:         "/bin/codex",
		Model:           "gpt-5.5",
		ReasoningEffort: "xhigh",
		Resume:          true,
		Authentication:  Authentication{Source: AuthSubscription, Path: "/credentials/codex.json"},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Headless.Command != "/bin/codex" || profile.Headless.ReasoningEffort != "xhigh" || profile.Headless.Auth.CredentialsFile != "/credentials/codex.json" || profile.Headless.Resume == nil || !*profile.Headless.Resume {
		t.Fatalf("Codex launch profile = %+v", profile.Headless)
	}
	openclaw, err := resolvedLawyerLaunchProfile(ResolvedAgentProfile{
		Runner:          RunnerOpenClaw,
		ReasoningEffort: "xhigh",
		Authentication:  Authentication{Source: AuthAPIKey, EnvironmentVariable: "SELECTED_OPENAI_KEY"},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if openclaw.OpenClaw.Auth.Mode != lawyerlaunch.OpenClawAuthAPIKey || openclaw.OpenClaw.Auth.APIKeyEnv != "SELECTED_OPENAI_KEY" || openclaw.OpenClaw.Network != "host" || openclaw.OpenClaw.Thinking != "xhigh" {
		t.Fatalf("OpenClaw launch profile = %+v", openclaw.OpenClaw)
	}
	if got := openClawReasoningEffort(""); got != "low" {
		t.Fatalf("default OpenClaw reasoning effort = %q", got)
	}
}

func TestQuickSettingsDecodeParallelCouncil(t *testing.T) {
	var settings QuickSettings
	if err := json.Unmarshal([]byte(`{"parallel_council":true}`), &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.ParallelCouncil {
		t.Fatal("parallel_council did not decode")
	}
}
