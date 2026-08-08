package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBlackBoxLawyerAttemptFailureDirectCase(t *testing.T) {
	fx := newBlackBoxFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	caseID := "bb-lawyer-direct"
	outDir := filepath.Join(fx.dir, "case-out")
	proc := fx.startAAR(t, ctx, "case",
		"--case-id", caseID,
		"--run-id", "run-"+caseID,
		"--complaint", fx.complaintPath,
		"--out-dir", outDir,
		"--engine", fx.enginePath,
		"--policy", fx.policyPath,
		"--council-pool", fx.councilPoolPath,
		"--common-root", fx.commonRoot,
		"--caseapi-addr", "127.0.0.1:0",
		"--invalid-attempt-limit", "1",
		"--lawyer-timeout-seconds", "30",
		"--timeout-seconds", "10",
	)
	defer proc.kill(t)

	caseBase := proc.waitForStderrPrefix(ctx, t, "caseapi listening on ")
	lawyerBase := caseBase + "/lawyerapi/v1"
	ready := waitLawyerReady(ctx, t, lawyerBase, caseID, "plaintiff")
	postLawyerTool(ctx, t, lawyerBase, map[string]any{
		"case_id":        caseID,
		"role_id":        "plaintiff",
		"opportunity_id": stringAt(ready, "turn", "opportunity_id"),
		"tool":           "submit_decision",
		"arguments": map[string]any{
			"kind":      "tool",
			"tool_name": "record_opening_statement",
			"payload":   map[string]any{},
		},
	}, false)

	if err := proc.wait(); err != nil {
		t.Fatalf("aard case exited with error: %v\nstderr:\n%s\nstdout:\n%s", err, proc.stderrString(), proc.stdoutString())
	}
	summary := lastJSONLine(t, proc.stdoutString())
	assertString(t, summary, "status", "failed")
	assertFailure(t, mapAny(summary["failure"]), "plaintiff", "attempts_exhausted")

	run := readJSONFile(t, filepath.Join(outDir, "run.json"))
	assertString(t, run, "status", "failed")
	caseObj := mapAny(mapAny(run["final_state"])["case"])
	assertString(t, caseObj, "status", "failed")
	assertEventTypes(t, filepath.Join(outDir, "events.ndjson"), "opportunity_failed")
}

func TestBlackBoxLawyerDeadlineFailureDirectCase(t *testing.T) {
	fx := newBlackBoxFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	caseID := "bb-lawyer-deadline"
	outDir := filepath.Join(fx.dir, "case-deadline-out")
	proc := fx.startAAR(t, ctx, "case",
		"--case-id", caseID,
		"--run-id", "run-"+caseID,
		"--complaint", fx.complaintPath,
		"--out-dir", outDir,
		"--engine", fx.enginePath,
		"--policy", fx.policyPath,
		"--council-pool", fx.councilPoolPath,
		"--common-root", fx.commonRoot,
		"--caseapi-addr", "127.0.0.1:0",
		"--invalid-attempt-limit", "2",
		"--lawyer-timeout-seconds", "1",
		"--timeout-seconds", "10",
	)
	defer proc.kill(t)

	caseBase := proc.waitForStderrPrefix(ctx, t, "caseapi listening on ")
	lawyerBase := caseBase + "/lawyerapi/v1"
	waitLawyerReady(ctx, t, lawyerBase, caseID, "plaintiff")

	if err := proc.wait(); err != nil {
		t.Fatalf("aard case exited with error: %v\nstderr:\n%s\nstdout:\n%s", err, proc.stderrString(), proc.stdoutString())
	}
	summary := lastJSONLine(t, proc.stdoutString())
	assertString(t, summary, "status", "failed")
	assertFailure(t, mapAny(summary["failure"]), "plaintiff", "deadline_expired")

	run := readJSONFile(t, filepath.Join(outDir, "run.json"))
	assertString(t, run, "status", "failed")
	caseObj := mapAny(mapAny(run["final_state"])["case"])
	assertString(t, caseObj, "status", "failed")
	assertEventTypes(t, filepath.Join(outDir, "events.ndjson"), "opportunity_failed")
}

func TestBlackBoxRuntimeFailureUsesNonzeroExit(t *testing.T) {
	fx := newBlackBoxFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	proc := fx.startAAR(t, ctx, "case",
		"--case-id", "bb-runtime-failure",
		"--run-id", "run-bb-runtime-failure",
		"--complaint", filepath.Join(fx.dir, "missing-complaint.md"),
		"--out-dir", filepath.Join(fx.dir, "runtime-failure-out"),
		"--engine", fx.enginePath,
	)
	if err := proc.wait(); err == nil {
		t.Fatalf("aard case exit = 0, want nonzero\nstdout:\n%s\nstderr:\n%s", proc.stdoutString(), proc.stderrString())
	}
	summary := lastJSONLine(t, proc.stdoutString())
	assertString(t, summary, "status", "error")
	if strings.Contains(proc.stderrString(), "caseapi listening on ") {
		t.Fatalf("runtime failure unexpectedly started case API\nstderr:\n%s", proc.stderrString())
	}
}

type blackBoxFixture struct {
	dir             string
	arbRoot         string
	aarBin          string
	enginePath      string
	commonRoot      string
	complaintPath   string
	policyPath      string
	councilPoolPath string
	provider        *httptest.Server
}

func newBlackBoxFixture(t *testing.T) *blackBoxFixture {
	t.Helper()
	arbRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve arbd root: %v", err)
	}
	aarBin := filepath.Join(arbRoot, ".bin", "aard")
	enginePath := filepath.Join(arbRoot, ".bin", "aardengine")
	if _, err := os.Stat(aarBin); err != nil {
		t.Skipf("%s is required; run make build in arbd first", aarBin)
	}
	if _, err := os.Stat(enginePath); err != nil {
		t.Skipf("%s is required; run make build in arbd first", enginePath)
	}
	provider := newFakeResponsesServer(t)
	dir, err := os.MkdirTemp("", "aard-blackbox-"+safeTestName(t.Name())+"-")
	if err != nil {
		t.Fatalf("create black-box fixture dir: %v", err)
	}
	t.Logf("black-box fixture directory: %s", dir)
	t.Cleanup(func() {
		provider.Close()
		if t.Failed() {
			t.Logf("retained black-box fixture directory: %s", dir)
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove black-box fixture dir %s: %v", dir, err)
		}
	})
	complaintPath := filepath.Join(dir, "case", "complaint.md")
	mustWriteFile(t, complaintPath, "# Question\n\nThe question is true for this process and HTTP test.\n")
	policyPath := filepath.Join(dir, "policy.json")
	mustWriteJSON(t, policyPath, map[string]any{
		"council_size":          3,
		"judgment_standard":     "Answer with one integer from 0 through 100.",
		"max_opening_chars":     1000,
		"max_argument_chars":    1000,
		"max_rebuttal_chars":    1000,
		"max_surrebuttal_chars": 1000,
		"max_closing_chars":     1000,
	})
	poolDir := filepath.Join(dir, "pool")
	for _, name := range []string{"c1.txt", "c2.txt", "c3.txt"} {
		mustWriteFile(t, filepath.Join(poolDir, name), "Process test council persona.\n")
	}
	councilPoolPath := filepath.Join(poolDir, "pool.jsonl")
	mustWriteFile(t, councilPoolPath, strings.Join([]string{
		`{"endpoint":"openai","model":"blackbox-council","persona":"c1.txt"}`,
		`{"endpoint":"openai","model":"blackbox-council","persona":"c2.txt"}`,
		`{"endpoint":"openai","model":"blackbox-council","persona":"c3.txt"}`,
		"",
	}, "\n"))
	return &blackBoxFixture{
		dir:             dir,
		arbRoot:         arbRoot,
		aarBin:          aarBin,
		enginePath:      enginePath,
		commonRoot:      filepath.Join(filepath.Dir(arbRoot), "common"),
		complaintPath:   complaintPath,
		policyPath:      policyPath,
		councilPoolPath: councilPoolPath,
		provider:        provider,
	}
}

func newFakeResponsesServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read fake provider request: %v", err)
			http.Error(w, "read request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id":         "resp_blackbox",
			"object":     "response",
			"created_at": time.Now().Unix(),
			"status":     "completed",
			"model":      "blackbox-council",
			"output": []map[string]any{
				{
					"id":     "msg_blackbox",
					"type":   "message",
					"status": "completed",
					"role":   "assistant",
					"content": []map[string]any{
						{
							"type":        "output_text",
							"text":        "ready",
							"annotations": []any{},
						},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}); err != nil {
			t.Errorf("write fake provider response: %v", err)
		}
	}))
}

func (fx *blackBoxFixture) startAAR(t *testing.T, ctx context.Context, args ...string) *testProcess {
	t.Helper()
	cmd := exec.CommandContext(ctx, fx.aarBin, args...)
	cmd.Dir = fx.arbRoot
	cmd.Env = mergedEnv(map[string]string{
		"OPENAI_API_KEY":  "blackbox-key",
		"OPENAI_BASE_URL": fx.provider.URL + "/v1",
	})
	return startTestProcess(t, cmd, filepath.Join(fx.dir, "stdout.log"), filepath.Join(fx.dir, "stderr.log"))
}

type testProcess struct {
	cmd    *exec.Cmd
	stdout lockedBuffer
	stderr lockedBuffer
	done   chan processResult
}

type processResult struct {
	waitErr  error
	closeErr error
}

func startTestProcess(t *testing.T, cmd *exec.Cmd, stdoutLogPath string, stderrLogPath string) *testProcess {
	t.Helper()
	stdoutLog := createLogFile(t, stdoutLogPath)
	stderrLog := createLogFile(t, stderrLogPath)
	proc := &testProcess{cmd: cmd, done: make(chan processResult, 1)}
	cmd.Stdout = io.MultiWriter(&proc.stdout, stdoutLog)
	cmd.Stderr = io.MultiWriter(&proc.stderr, stderrLog)
	if err := cmd.Start(); err != nil {
		closeErr := errors.Join(closeLog(stdoutLog, "stdout"), closeLog(stderrLog, "stderr"))
		t.Fatalf("start process: %v", errors.Join(err, closeErr))
	}
	go func() {
		waitErr := cmd.Wait()
		closeErr := errors.Join(closeLog(stdoutLog, "stdout"), closeLog(stderrLog, "stderr"))
		proc.done <- processResult{waitErr: waitErr, closeErr: closeErr}
	}()
	return proc
}

func createLogFile(t *testing.T, path string) *os.File {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create process log directory: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create process log: %v", err)
	}
	return f
}

func closeLog(f *os.File, name string) error {
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s log: %w", name, err)
	}
	return nil
}

func (p *testProcess) waitForStderrPrefix(ctx context.Context, t *testing.T, prefix string) string {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if value, ok := findLinePrefix(p.stderrString(), prefix); ok {
			return value
		}
		select {
		case result := <-p.done:
			t.Fatalf("process exited before %q: %v\nstderr:\n%s\nstdout:\n%s", prefix, errors.Join(result.waitErr, result.closeErr), p.stderrString(), p.stdoutString())
		case <-ctx.Done():
			t.Fatalf("timeout waiting for %q\nstderr:\n%s\nstdout:\n%s", prefix, p.stderrString(), p.stdoutString())
		case <-ticker.C:
		}
	}
}

func (p *testProcess) wait() error {
	result := <-p.done
	return errors.Join(result.waitErr, result.closeErr)
}

func (p *testProcess) kill(t *testing.T) {
	t.Helper()
	if p.cmd.Process == nil {
		return
	}
	err := p.cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return
	}
	if err != nil {
		t.Errorf("kill process: %v", err)
		return
	}
	result := <-p.done
	if result.closeErr != nil {
		t.Errorf("close process logs after killing process: %v", result.closeErr)
	}
	var exitErr *exec.ExitError
	if result.waitErr != nil && !errors.As(result.waitErr, &exitErr) {
		t.Errorf("wait after killing process: %v", result.waitErr)
	}
}

func (p *testProcess) stdoutString() string {
	return p.stdout.String()
}

func (p *testProcess) stderrString() string {
	return p.stderr.String()
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func findLinePrefix(text string, prefix string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

func waitLawyerReady(ctx context.Context, t *testing.T, base string, caseID string, role string) map[string]any {
	t.Helper()
	endpoint := fmt.Sprintf("%s/wait?case_id=%s&role_id=%s&timeout_ms=5000", base, url.QueryEscape(caseID), url.QueryEscape(role))
	for {
		resp := getJSON(ctx, t, endpoint)
		switch mapString(resp["status"]) {
		case "ready":
			return resp
		case "failed", "done":
			t.Fatalf("lawyer %s status = %s, want ready: %#v", role, resp["status"], resp)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for lawyer %s ready", role)
		default:
		}
	}
}

func postLawyerTool(ctx context.Context, t *testing.T, base string, body map[string]any, wantOK bool) map[string]any {
	t.Helper()
	resp := postJSON(ctx, t, base+"/do", body)
	if got := boolAt(resp, "ok"); got != wantOK {
		t.Fatalf("POST %s/do ok = %t, want %t; body=%#v response=%#v", base, got, wantOK, body, resp)
	}
	return resp
}

func getJSON(ctx context.Context, t *testing.T, endpoint string) map[string]any {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", endpoint, err)
	}
	return decodeHTTPJSON(t, resp, endpoint)
}

func postJSON(ctx context.Context, t *testing.T, endpoint string, body map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal POST body: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", endpoint, err)
	}
	return decodeHTTPJSON(t, resp, endpoint)
}

func decodeHTTPJSON(t *testing.T, resp *http.Response, endpoint string) map[string]any {
	t.Helper()
	raw, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read %s response: %v", endpoint, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s returned non-JSON HTTP %d: %s", endpoint, resp.StatusCode, string(raw))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s returned HTTP %d: %#v", endpoint, resp.StatusCode, out)
	}
	return out
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

func lastJSONLine(t *testing.T, text string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(line), &out); err == nil {
			return out
		}
	}
	t.Fatalf("no JSON line in:\n%s", text)
	return nil
}

func assertEventTypes(t *testing.T, path string, want ...string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	found := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("parse event line %q: %v", line, err)
		}
		found[mapString(event["type"])] = true
	}
	for _, eventType := range want {
		if !found[eventType] {
			t.Fatalf("events in %s missing %q; found=%v", path, eventType, found)
		}
	}
}

func assertFailure(t *testing.T, failure map[string]any, role string, reason string) {
	t.Helper()
	if len(failure) == 0 {
		t.Fatalf("failure object is empty")
	}
	if got := mapString(failure["type"]); got != "" && got != "opportunity_failed" {
		t.Fatalf("failure.type = %q, want opportunity_failed", got)
	}
	if got := mapString(failure["failure_type"]); got != "" && got != "opportunity_failed" {
		t.Fatalf("failure.failure_type = %q, want opportunity_failed", got)
	}
	assertString(t, failure, "role", role)
	assertString(t, failure, "reason", reason)
	if mapString(failure["opportunity_id"]) == "" {
		t.Fatalf("failure missing opportunity_id: %#v", failure)
	}
}

func assertString(t *testing.T, m map[string]any, key string, want string) {
	t.Helper()
	if got := mapString(m[key]); got != want {
		t.Fatalf("%s = %q, want %q in %#v", key, got, want, m)
	}
}

func stringAt(m map[string]any, keys ...string) string {
	var value any = m
	for _, key := range keys {
		value = mapAny(value)[key]
	}
	return mapString(value)
}

func boolAt(m map[string]any, key string) bool {
	value, _ := m[key].(bool)
	return value
}

func mapAny(value any) map[string]any {
	out, _ := value.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func mapString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func safeTestName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "test"
	}
	return out
}

func mustWriteFile(t *testing.T, path string, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	mustWriteFile(t, path, string(data)+"\n")
}

func mergedEnv(overrides map[string]string) []string {
	env := os.Environ()
	for key, value := range overrides {
		prefix := key + "="
		replaced := false
		for i, existing := range env {
			if strings.HasPrefix(existing, prefix) {
				env[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}
