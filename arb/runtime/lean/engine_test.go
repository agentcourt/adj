package lean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStepEncodesOpportunityAuthority(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	requestPath := filepath.Join(dir, "request.json")
	script := `#!/bin/sh
request=$(cat)
printf '%s' "$request" > "$1"
printf '%s\n' '{"ok":true,"state":{"state_version":8}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	authority := OpportunityAuthority{
		OpportunityID:        "deliberation:2:C3",
		ExpectedStateVersion: 7,
		Role:                 "council",
		Phase:                "deliberation",
		MemberID:             "C3",
	}
	engine := New([]string{enginePath, requestPath})
	if _, err := engine.Step(
		context.Background(),
		map[string]any{"state_version": 7},
		"submit_council_vote",
		"council",
		authority,
		map[string]any{"member_id": "C3"},
	); err != nil {
		t.Fatalf("step: %v", err)
	}
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	var request struct {
		Action struct {
			Authority OpportunityAuthority `json:"authority"`
		} `json:"action"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if request.Action.Authority != authority {
		t.Fatalf("authority = %#v, want %#v", request.Action.Authority, authority)
	}
}

func TestCallHandlesNonzeroProcessExit(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
printf '%s' "$1"
printf '%s' "$2" >&2
exit "$3"
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	request := map[string]any{"request_type": "next_opportunity"}

	t.Run("protocol rejection", func(t *testing.T) {
		engine := New([]string{enginePath, `{"ok":false,"error":"rejected"}`, "protocol stderr", "1"})
		response, err := engine.Call(context.Background(), request)
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if ok, valid := response["ok"].(bool); !valid || ok {
			t.Fatalf("ok = %#v, want false", response["ok"])
		}
	})

	t.Run("protocol rejection with wrong exit status", func(t *testing.T) {
		engine := New([]string{enginePath, `{"ok":false,"error":"rejected"}`, "protocol stderr", "7"})
		response, err := engine.Call(context.Background(), request)
		if err == nil {
			t.Fatalf("call response = %#v, want process error", response)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
			t.Fatalf("error = %v, want exit status 7", err)
		}
		if !strings.Contains(err.Error(), "lean rejected next_opportunity: rejected") {
			t.Fatalf("error = %q, want protocol rejection message", err)
		}
	})

	t.Run("protocol rejection after signal", func(t *testing.T) {
		engine := New([]string{"/bin/sh", "-c", `printf '%s' '{"ok":false,"error":"rejected"}'; kill -TERM $$`})
		response, err := engine.Call(context.Background(), request)
		if err == nil {
			t.Fatalf("call response = %#v, want process error", response)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != -1 {
			t.Fatalf("error = %v, want signal termination", err)
		}
		if !strings.Contains(err.Error(), "lean rejected next_opportunity: rejected") {
			t.Fatalf("error = %q, want protocol rejection message", err)
		}
	})

	for _, tt := range []struct {
		name   string
		stdout string
	}{
		{name: "ok true", stdout: `{"ok":true}`},
		{name: "missing ok", stdout: `{"state":{}}`},
		{name: "nonboolean ok", stdout: `{"ok":"false"}`},
		{name: "rejection missing error", stdout: `{"ok":false}`},
		{name: "rejection blank error", stdout: `{"ok":false,"error":" "}`},
		{name: "rejection nonstring error", stdout: `{"ok":false,"error":7}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			engine := New([]string{enginePath, tt.stdout, "synthetic stderr", "7"})
			response, err := engine.Call(context.Background(), request)
			if err == nil {
				t.Fatalf("call response = %#v, want process error", response)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("error = %v, want wrapped process exit", err)
			}
			want := "lean process failed for next_opportunity: exit status 7 stderr=synthetic stderr"
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want substring %q", err, want)
			}
		})
	}
}

func TestCallPreservesJSONParseErrorAfterNonzeroExit(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
printf '%s' '{'
printf '%s' 'synthetic stderr' >&2
exit 8
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	response, err := New([]string{enginePath}).Call(context.Background(), map[string]any{"request_type": "next_opportunity"})
	if err == nil {
		t.Fatalf("call response = %#v, want JSON parse error", response)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want wrapped process exit", err)
	}
	if !strings.Contains(err.Error(), "parse lean json:") {
		t.Fatalf("error = %q, want JSON parse context", err)
	}
	if !strings.Contains(err.Error(), "lean process failed for next_opportunity: exit status 8 stderr=synthetic stderr") {
		t.Fatalf("error = %q, want process and stderr context", err)
	}
}

func TestCallPreservesZeroExitBehaviorWithoutOKField(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
printf '%s' '{"state":{"state_version":1}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	response, err := New([]string{enginePath}).Call(context.Background(), map[string]any{"request_type": "initialize_case"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if _, ok := response["state"].(map[string]any); !ok {
		t.Fatalf("state = %#v, want object", response["state"])
	}
}

func TestCallHonorsContextDeadline(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
printf '%s' 'before deadline' >&2
exec sleep 30
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	response, err := New([]string{enginePath}).Call(ctx, map[string]any{"request_type": "next_opportunity"})
	if err == nil {
		t.Fatalf("call response = %#v, want deadline error", response)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	if !strings.HasPrefix(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("error = %q, want deadline cause first", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("call returned after %s, want no more than 3s", elapsed)
	}
	if !strings.Contains(err.Error(), "stderr=before deadline") {
		t.Fatalf("error = %q, want process stderr", err)
	}
}

func TestCallPreservesCustomCancellationCause(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	readyPath := filepath.Join(dir, "ready")
	script := `#!/bin/sh
printf '%s' ready > "$1"
printf '%s' '{'
printf '%s' 'before cancellation' >&2
exec sleep 30
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(context.Canceled)
	result := make(chan error, 1)
	go func() {
		_, err := New([]string{enginePath, readyPath}).Call(ctx, map[string]any{"request_type": "initialize_case"})
		result <- err
	}()
	waitForTestFile(t, readyPath)
	cause := errors.New("case stopped")
	cancel(cause)
	select {
	case err := <-result:
		if !errors.Is(err, cause) {
			t.Fatalf("error = %v, want custom cancellation cause", err)
		}
		if !strings.HasPrefix(err.Error(), cause.Error()) {
			t.Fatalf("error = %q, want custom cause first", err)
		}
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("error = %v, want wrapped JSON syntax error", err)
		}
		if !strings.Contains(err.Error(), "stderr=before cancellation") {
			t.Fatalf("error = %q, want process stderr", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("call did not return after cancellation")
	}
}

func TestCallCancellationKillsChildProcessGroup(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	childPath := filepath.Join(dir, "child-pid")
	script := `#!/bin/sh
sleep 30 &
child=$!
printf '%s' "$child" > "$1"
wait "$child"
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := New([]string{enginePath, childPath}).Call(ctx, map[string]any{"request_type": "next_opportunity"})
		result <- err
	}()
	waitForTestFile(t, childPath)
	rawPID, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatalf("parse child pid: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
		waitForTestProcessTermination(t, childPID)
	case <-time.After(3 * time.Second):
		t.Fatal("call did not return after process-group cancellation")
	}
}

func TestCallPreservesDescendantPipeCleanupError(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	childPath := filepath.Join(dir, "child-pid")
	script := `#!/bin/sh
sleep 30 &
child=$!
printf '%s' "$child" > "$1"
printf '%s' '{"ok":false,"error":"rejected"}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	response, err := New([]string{enginePath, childPath}).Call(context.Background(), map[string]any{"request_type": "next_opportunity"})
	rawPID, readErr := os.ReadFile(childPath)
	if readErr != nil {
		t.Fatalf("read child pid: %v", readErr)
	}
	childPID, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if parseErr != nil {
		t.Fatalf("parse child pid: %v", parseErr)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})
	if err == nil {
		t.Fatalf("call response = %#v, want pipe cleanup error", response)
	}
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("error = %v, want exec.ErrWaitDelay", err)
	}
	if !errors.Is(err, errLeanDescendantsAfterExit) {
		t.Fatalf("error = %v, want descendant cleanup error", err)
	}
	if !strings.Contains(err.Error(), "lean rejected next_opportunity: rejected") {
		t.Fatalf("error = %q, want protocol rejection message", err)
	}
	waitForTestProcessTermination(t, childPID)
}

func TestCallKillsDescendantAfterExitError(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	childPath := filepath.Join(dir, "child-pid")
	script := `#!/bin/sh
sleep 30 &
child=$!
printf '%s' "$child" > "$1"
printf '%s' 'descendant stderr' >&2
printf '%s' '{"ok":false,"error":"rejected"}'
exit 1
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	response, err := New([]string{enginePath, childPath}).Call(context.Background(), map[string]any{"request_type": "next_opportunity"})
	rawPID, readErr := os.ReadFile(childPath)
	if readErr != nil {
		t.Fatalf("read child pid: %v", readErr)
	}
	childPID, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if parseErr != nil {
		t.Fatalf("parse child pid: %v", parseErr)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})
	if err == nil {
		t.Fatalf("call response = %#v, want process-control error", response)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("error = %v, want exit status 1", err)
	}
	if !errors.Is(err, errLeanDescendantsAfterExit) {
		t.Fatalf("error = %v, want descendant cleanup error", err)
	}
	if !strings.Contains(err.Error(), "stderr=descendant stderr") {
		t.Fatalf("error = %q, want process stderr", err)
	}
	if !strings.Contains(err.Error(), "lean rejected next_opportunity: rejected") {
		t.Fatalf("error = %q, want protocol rejection message", err)
	}
	waitForTestProcessTermination(t, childPID)
}

func waitForTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForTestProcessTermination(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		state, err := testProcessState(pid)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err != nil {
			t.Fatalf("read process %d state: %v", pid, err)
		}
		if state == 'Z' {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %d remained in state %c", pid, state)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func testProcessState(pid int) (byte, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	stat := string(raw)
	separator := strings.LastIndex(stat, ") ")
	if separator < 0 || separator+2 >= len(stat) {
		return 0, fmt.Errorf("parse /proc/%d/stat", pid)
	}
	return stat[separator+2], nil
}

func TestEngineEnforcesOpportunityAuthority(t *testing.T) {
	engine := testRealEngine(t)
	initial := testInitialState()
	members := []map[string]any{
		testCouncilMember("C1"),
		testCouncilMember("C2"),
		testCouncilMember("C3"),
	}
	initializedResponse, err := engine.InitializeCase(context.Background(), initial, "The proposition is demonstrated.", members)
	if err != nil {
		t.Fatalf("initialize case: %v", err)
	}
	if ok, _ := initializedResponse["ok"].(bool); !ok {
		t.Fatalf("initialize case rejected: %v", initializedResponse["error"])
	}
	initialized := requireTestMap(t, initializedResponse["state"], "initialized state")
	correct := OpportunityAuthority{
		OpportunityID:        "openings:plaintiff",
		ExpectedStateVersion: 1,
		Role:                 "plaintiff",
		Phase:                "openings",
	}
	accepted, err := engine.Step(context.Background(), initialized, "record_opening_statement", "plaintiff", correct, map[string]any{"text": "Opening."})
	if err != nil {
		t.Fatalf("correct authority step: %v", err)
	}
	if ok, _ := accepted["ok"].(bool); !ok {
		t.Fatalf("correct authority rejected: %v", accepted["error"])
	}
	for _, field := range []string{"offered_evidence", "technical_reports"} {
		response, err := engine.Step(context.Background(), initialized, "record_opening_statement", "plaintiff", correct, map[string]any{
			"text": "Opening.",
			field:  []any{map[string]any{}},
		})
		if err != nil {
			t.Fatalf("opening with %s: %v", field, err)
		}
		if ok, _ := response["ok"].(bool); ok {
			t.Fatalf("opening with %s was accepted", field)
		}
		if got, _ := response["error"].(string); !strings.Contains(got, field+" are allowed only") {
			t.Fatalf("opening with %s error = %q", field, got)
		}
	}
	stale := correct
	stale.ExpectedStateVersion = 0
	assertEngineAuthorityRejected(t, engine, initialized, "record_opening_statement", "plaintiff", stale, map[string]any{"text": "Opening."}, "stale opportunity state_version=0 current=1")

	caseState := requireTestMap(t, initialized["case"], "initialized case")
	caseState["phase"] = "deliberation"
	wrongMember := OpportunityAuthority{
		OpportunityID:        "deliberation:1:C2",
		ExpectedStateVersion: 1,
		Role:                 "council",
		Phase:                "deliberation",
		MemberID:             "C2",
	}
	assertEngineAuthorityRejected(t, engine, initialized, "submit_council_vote", "council", wrongMember, map[string]any{
		"member_id": "C2",
		"vote":      "demonstrated",
		"rationale": "Reason.",
	}, "does not match current opportunity deliberation:1:C1")
	correctMember := wrongMember
	correctMember.OpportunityID = "deliberation:1:C1"
	correctMember.MemberID = "C1"
	assertEngineAuthorityRejected(t, engine, initialized, "submit_council_vote", "council", correctMember, map[string]any{
		"member_id": "C2",
		"vote":      "demonstrated",
		"rationale": "Reason.",
	}, "action member_id C2 does not match current member_id C1")
}

func TestEnginePreservesInitialEvidenceCatalogIncludingZeroSize(t *testing.T) {
	engine := testRealEngine(t)
	catalog := []map[string]any{
		{"evidence_id": "E-second", "sha256": strings.Repeat("b", 64), "size_bytes": 0},
		{"evidence_id": "E-first", "sha256": strings.Repeat("a", 64), "size_bytes": 17},
	}
	initial := testInitialState()
	initial["evidence_catalog"] = catalog

	response, err := engine.InitializeCase(context.Background(), initial, "The proposition is demonstrated.", testCouncilMembers())
	if err != nil {
		t.Fatalf("initialize case: %v", err)
	}
	state := requireEngineAcceptedState(t, response, "initialize case")
	requireTestStateVersion(t, state, 1)
	requireTestJSONEqual(t, state["evidence_catalog"], catalog, "evidence_catalog")
}

func TestEngineRejectsInvalidInitialEvidenceCatalog(t *testing.T) {
	engine := testRealEngine(t)
	valid := map[string]any{
		"evidence_id": "E-valid",
		"sha256":      strings.Repeat("a", 64),
		"size_bytes":  1,
	}
	tests := []struct {
		name    string
		catalog []map[string]any
		want    string
	}{
		{
			name: "duplicate identifier",
			catalog: []map[string]any{
				valid,
				{"evidence_id": "E-valid", "sha256": strings.Repeat("b", 64), "size_bytes": 2},
			},
			want: "evidence_catalog contains duplicate evidence_id",
		},
		{
			name:    "empty identifier",
			catalog: []map[string]any{{"evidence_id": "", "sha256": strings.Repeat("a", 64), "size_bytes": 1}},
			want:    "evidence_catalog evidence_id must be nonempty and trimmed",
		},
		{
			name:    "untrimmed identifier",
			catalog: []map[string]any{{"evidence_id": " E-valid", "sha256": strings.Repeat("a", 64), "size_bytes": 1}},
			want:    "evidence_catalog evidence_id must be nonempty and trimmed",
		},
		{
			name:    "empty digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": "", "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "untrimmed digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 64) + " ", "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "short digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 63), "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "long digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 65), "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "uppercase digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("A", 64), "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "nonhex digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 63) + "g", "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "missing size",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 64)}},
			want:    "size_bytes",
		},
		{
			name:    "wrong-type size",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 64), "size_bytes": "1"}},
			want:    "size_bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial := testInitialState()
			initial["evidence_catalog"] = tt.catalog
			response, err := engine.InitializeCase(context.Background(), initial, "The proposition is demonstrated.", testCouncilMembers())
			if err != nil {
				t.Fatalf("initialize case transport: %v", err)
			}
			requireEngineRejected(t, response, tt.want)
		})
	}
}

func TestEngineRejectsInvalidOfferedEvidenceLabelTypes(t *testing.T) {
	engine := testRealEngine(t)
	initial := testInitialState()
	initial["evidence_catalog"] = []map[string]any{
		{"evidence_id": "E-known", "sha256": strings.Repeat("a", 64), "size_bytes": 16},
	}
	arguments := initializeTestArguments(t, engine, initial)
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}
	for _, tt := range []struct {
		name  string
		value any
	}{
		{name: "null", value: nil},
		{name: "number", value: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response, err := engine.Step(context.Background(), arguments, "submit_argument", "plaintiff", authority, map[string]any{
				"text": "Argument.",
				"offered_evidence": []map[string]any{
					{"evidence_id": "E-known", "label": tt.value},
				},
			})
			if err != nil {
				t.Fatalf("submit argument transport: %v", err)
			}
			requireEngineRejected(t, response, "label must be a string")
		})
	}
}

func TestEngineRejectsInvalidSubmittedEvidenceOptionalStringTypes(t *testing.T) {
	engine := testRealEngine(t)
	arguments := initializeTestArguments(t, engine, testInitialState())
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}
	fields := []string{
		"source_url",
		"source_description",
		"retrieval_timestamp",
		"parent_evidence_id",
		"parent_sha256",
		"derivation_method",
	}
	values := []struct {
		name  string
		value any
	}{
		{name: "null", value: nil},
		{name: "number", value: 1},
	}
	for _, field := range fields {
		for _, value := range values {
			t.Run(field+"/"+value.name, func(t *testing.T) {
				payload := testSubmittedEvidencePayload("E-invalid-"+field+"-"+value.name, strings.Repeat("b", 64))
				payload[field] = value.value
				response, err := engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, payload)
				if err != nil {
					t.Fatalf("submit evidence transport: %v", err)
				}
				requireEngineRejected(t, response, field+" must be a string")
			})
		}
	}
}

func TestEngineRejectsInvalidCouncilRationaleTypes(t *testing.T) {
	engine := testRealEngine(t)
	state := initializeTestArguments(t, engine, testInitialState())
	caseState := requireTestMap(t, state["case"], "case")
	caseState["phase"] = "deliberation"
	authority := OpportunityAuthority{
		OpportunityID:        "deliberation:1:C1",
		ExpectedStateVersion: 3,
		Role:                 "council",
		Phase:                "deliberation",
		MemberID:             "C1",
	}
	for _, tt := range []struct {
		name  string
		value any
	}{
		{name: "null", value: nil},
		{name: "number", value: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response, err := engine.Step(context.Background(), state, "submit_council_vote", "council", authority, map[string]any{
				"member_id": "C1",
				"vote":      "demonstrated",
				"rationale": tt.value,
			})
			if err != nil {
				t.Fatalf("submit council vote transport: %v", err)
			}
			requireEngineRejected(t, response, "rationale must be a string")
		})
	}
}

func TestEngineRejectsInvalidFailureOptionalStringTypes(t *testing.T) {
	engine := testRealEngine(t)
	state := initializeTestArguments(t, engine, testInitialState())
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}
	fields := []string{"message", "member_id", "model"}
	values := []struct {
		name  string
		value any
	}{
		{name: "null", value: nil},
		{name: "number", value: 1},
	}
	for _, field := range fields {
		for _, value := range values {
			t.Run(field+"/"+value.name, func(t *testing.T) {
				payload := map[string]any{
					"opportunity_id": "arguments:plaintiff",
					"role":           "plaintiff",
					"phase":          "arguments",
					"reason":         "agent_error",
					field:            value.value,
				}
				response, err := engine.Step(context.Background(), state, "fail_opportunity", "system", authority, payload)
				if err != nil {
					t.Fatalf("fail opportunity transport: %v", err)
				}
				requireEngineRejected(t, response, field+" must be a string")
			})
		}
	}
}

func TestEngineValidatesOfferedEvidenceAndReportBytes(t *testing.T) {
	engine := testRealEngine(t)
	catalog := []map[string]any{
		{"evidence_id": "E-known", "sha256": strings.Repeat("a", 64), "size_bytes": 16},
		{"evidence_id": "E-oversized", "sha256": strings.Repeat("b", 64), "size_bytes": 17},
	}
	initial := testInitialState()
	initial["evidence_catalog"] = catalog
	policy := requireTestMap(t, initial["policy"], "initial policy")
	policy["max_exhibit_bytes"] = 16
	policy["max_report_title_bytes"] = 5
	arguments := initializeTestArguments(t, engine, initial)
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}

	accepted, err := engine.Step(context.Background(), arguments, "submit_argument", "plaintiff", authority, map[string]any{
		"text": "Argument.",
		"offered_evidence": []map[string]any{
			{"evidence_id": "E-known", "label": "Known exhibit"},
		},
	})
	if err != nil {
		t.Fatalf("known evidence transport: %v", err)
	}
	acceptedState := requireEngineAcceptedState(t, accepted, "known evidence")
	requireTestStateVersion(t, acceptedState, 4)

	for _, tt := range []struct {
		name       string
		evidenceID string
	}{
		{name: "unknown", evidenceID: "E-unknown"},
		{name: "oversized", evidenceID: "E-oversized"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response, err := engine.Step(context.Background(), arguments, "submit_argument", "plaintiff", authority, map[string]any{
				"text":             "Argument.",
				"offered_evidence": []map[string]any{{"evidence_id": tt.evidenceID}},
			})
			if err != nil {
				t.Fatalf("submit argument transport: %v", err)
			}
			requireEngineRejected(t, response, "offered_evidence contains an unknown or oversized evidence_id")
		})
	}

	title := "界界"
	if characters, bytes := len([]rune(title)), len([]byte(title)); characters >= 5 || bytes <= 5 {
		t.Fatalf("test title has %d characters and %d bytes", characters, bytes)
	}
	reportResponse, err := engine.Step(context.Background(), arguments, "submit_argument", "plaintiff", authority, map[string]any{
		"text": "Argument.",
		"technical_reports": []map[string]any{
			{"title": title, "summary": "Summary."},
		},
	})
	if err != nil {
		t.Fatalf("technical report transport: %v", err)
	}
	requireEngineRejected(t, reportResponse, "technical_reports exceed a UTF-8 byte limit")
}

func TestEngineValidatesSubmittedEvidenceLineage(t *testing.T) {
	engine := testRealEngine(t)
	parentSHA := strings.Repeat("a", 64)
	derivedSHA := strings.Repeat("b", 64)
	initial := testInitialState()
	initial["evidence_catalog"] = []map[string]any{
		{"evidence_id": "E-parent", "sha256": parentSHA, "size_bytes": 12},
	}
	arguments := initializeTestArguments(t, engine, initial)
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}
	valid := testSubmittedEvidencePayload("E-derived", derivedSHA)
	valid["parent_evidence_id"] = "E-parent"
	valid["parent_sha256"] = parentSHA
	valid["derivation_method"] = "excerpt"
	response, err := engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, valid)
	if err != nil {
		t.Fatalf("valid submission transport: %v", err)
	}
	accepted := requireEngineAcceptedState(t, response, "valid submitted evidence")
	requireTestStateVersion(t, accepted, 4)
	caseState := requireTestMap(t, accepted["case"], "accepted case")
	submitted := requireTestSlice(t, caseState["submitted_evidence"], "submitted_evidence")
	if len(submitted) != 1 {
		t.Fatalf("submitted_evidence length = %d, want 1", len(submitted))
	}
	requireTestJSONEqual(t, submitted[0], map[string]any{
		"phase":               "arguments",
		"role":                "plaintiff",
		"evidence_id":         "E-derived",
		"title":               "Derived evidence",
		"source_url":          "",
		"source_description":  "Test source",
		"mime_type":           "text/plain",
		"retrieval_timestamp": "",
		"relevance":           "Test relevance",
		"sha256":              derivedSHA,
		"size_bytes":          8,
		"parent_evidence_id":  "E-parent",
		"parent_sha256":       parentSHA,
		"derivation_method":   "excerpt",
	}, "submitted evidence")

	tests := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{
			name: "missing parent digest",
			payload: func() map[string]any {
				payload := testSubmittedEvidencePayload("E-missing-parent-digest", derivedSHA)
				payload["parent_evidence_id"] = "E-parent"
				payload["derivation_method"] = "excerpt"
				return payload
			}(),
			want: "submitted evidence parent derivation requires parent_evidence_id, parent_sha256, and derivation_method",
		},
		{
			name: "mismatched parent digest",
			payload: func() map[string]any {
				payload := testSubmittedEvidencePayload("E-mismatched-parent-digest", derivedSHA)
				payload["parent_evidence_id"] = "E-parent"
				payload["parent_sha256"] = strings.Repeat("c", 64)
				payload["derivation_method"] = "excerpt"
				return payload
			}(),
			want: "submitted evidence parent derivation does not match an existing commitment",
		},
		{
			name: "malformed parent digest",
			payload: func() map[string]any {
				payload := testSubmittedEvidencePayload("E-malformed-parent-digest", derivedSHA)
				payload["parent_evidence_id"] = "E-parent"
				payload["parent_sha256"] = strings.Repeat("a", 63)
				payload["derivation_method"] = "excerpt"
				return payload
			}(),
			want: "submitted evidence parent_sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name: "self parent",
			payload: func() map[string]any {
				payload := testSubmittedEvidencePayload("E-self", derivedSHA)
				payload["parent_evidence_id"] = "E-self"
				payload["parent_sha256"] = derivedSHA
				payload["derivation_method"] = "excerpt"
				return payload
			}(),
			want: "submitted evidence parent_evidence_id must differ from evidence_id",
		},
		{
			name:    "initial catalog identifier collision",
			payload: testSubmittedEvidencePayload("E-parent", derivedSHA),
			want:    "submitted evidence_id collides with evidence_catalog: E-parent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, tt.payload)
			if err != nil {
				t.Fatalf("submit evidence transport: %v", err)
			}
			requireEngineRejected(t, response, tt.want)
		})
	}

	postSubmissionAuthority := authority
	postSubmissionAuthority.ExpectedStateVersion = 4
	grandchild := testSubmittedEvidencePayload("E-grandchild", strings.Repeat("c", 64))
	grandchild["parent_evidence_id"] = "E-derived"
	grandchild["parent_sha256"] = derivedSHA
	grandchild["derivation_method"] = "excerpt"
	grandchildResponse, err := engine.Step(context.Background(), accepted, "submit_evidence", "plaintiff", postSubmissionAuthority, grandchild)
	if err != nil {
		t.Fatalf("prior-submission parent transport: %v", err)
	}
	grandchildState := requireEngineAcceptedState(t, grandchildResponse, "prior-submission parent")
	requireTestStateVersion(t, grandchildState, 5)

	offeredResponse, err := engine.Step(context.Background(), accepted, "submit_argument", "plaintiff", postSubmissionAuthority, map[string]any{
		"text":             "Argument citing submitted evidence.",
		"offered_evidence": []map[string]any{{"evidence_id": "E-derived"}},
	})
	if err != nil {
		t.Fatalf("submitted-evidence offer transport: %v", err)
	}
	offeredState := requireEngineAcceptedState(t, offeredResponse, "submitted-evidence offer")
	requireTestStateVersion(t, offeredState, 5)

	duplicateResponse, err := engine.Step(context.Background(), accepted, "submit_evidence", "plaintiff", postSubmissionAuthority, valid)
	if err != nil {
		t.Fatalf("duplicate submission transport: %v", err)
	}
	requireEngineRejected(t, duplicateResponse, "duplicate submitted evidence_id: E-derived")
}

func TestEngineRejectsInvalidSubmittedEvidenceSHA256(t *testing.T) {
	engine := testRealEngine(t)
	arguments := initializeTestArguments(t, engine, testInitialState())
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}
	tests := []struct {
		name   string
		digest string
	}{
		{name: "short", digest: strings.Repeat("a", 63)},
		{name: "long", digest: strings.Repeat("a", 65)},
		{name: "uppercase", digest: strings.Repeat("A", 64)},
		{name: "nonhex", digest: strings.Repeat("a", 63) + "g"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := testSubmittedEvidencePayload("E-invalid-"+tt.name, tt.digest)
			response, err := engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, payload)
			if err != nil {
				t.Fatalf("submit evidence transport: %v", err)
			}
			requireEngineRejected(t, response, "submitted evidence sha256 must contain exactly 64 lowercase hexadecimal characters")
		})
	}
}

func testInitialState() map[string]any {
	return map[string]any{
		"schema_version": "v1",
		"forum_name":     "Authority Test Forum",
		"case": map[string]any{
			"case_id":            "authority-test",
			"caption":            "Claimant v. Respondent",
			"proposition":        "",
			"status":             "draft",
			"phase":              "draft",
			"council_members":    []map[string]any{},
			"openings":           []map[string]any{},
			"arguments":          []map[string]any{},
			"rebuttals":          []map[string]any{},
			"surrebuttals":       []map[string]any{},
			"closings":           []map[string]any{},
			"offered_evidence":   []map[string]any{},
			"technical_reports":  []map[string]any{},
			"submitted_evidence": []map[string]any{},
			"deliberation_round": 1,
			"council_votes":      []map[string]any{},
			"resolution":         "",
			"failure":            nil,
		},
		"policy": map[string]any{
			"council_size":                    3,
			"evidence_standard":               "Preponderance of the evidence.",
			"required_votes_for_decision":     2,
			"max_opening_chars":               4_000,
			"max_argument_chars":              6_000,
			"max_rebuttal_chars":              4_000,
			"max_surrebuttal_chars":           4_000,
			"max_closing_chars":               5_000,
			"max_deliberation_rounds":         3,
			"max_exhibits_per_filing":         9,
			"max_exhibits_per_side":           12,
			"max_exhibit_bytes":               131_072,
			"max_reports_per_filing":          3,
			"max_reports_per_side":            4,
			"max_report_title_bytes":          256,
			"max_report_summary_bytes":        8_192,
			"max_submitted_evidence_per_side": 8,
			"max_submitted_evidence_bytes":    131_072,
		},
		"evidence_catalog": []map[string]any{},
		"state_version":    0,
	}
}

func testCouncilMember(memberID string) map[string]any {
	return map[string]any{
		"member_id":              memberID,
		"model":                  "",
		"persona_filename":       "",
		"status":                 "seated",
		"failure_reason":         "",
		"failure_opportunity_id": "",
		"failure_message":        "",
	}
}

func testCouncilMembers() []map[string]any {
	return []map[string]any{
		testCouncilMember("C1"),
		testCouncilMember("C2"),
		testCouncilMember("C3"),
	}
}

func testRealEngine(t *testing.T) Engine {
	t.Helper()
	enginePath := filepath.Clean(filepath.Join("..", "..", ".bin", "aarengine"))
	if _, err := os.Stat(enginePath); err != nil {
		t.Fatalf("required engine %s is unavailable: %v", enginePath, err)
	}
	return New([]string{enginePath})
}

func initializeTestArguments(t *testing.T, engine Engine, initial map[string]any) map[string]any {
	t.Helper()
	response, err := engine.InitializeCase(context.Background(), initial, "The proposition is demonstrated.", testCouncilMembers())
	if err != nil {
		t.Fatalf("initialize case transport: %v", err)
	}
	state := requireEngineAcceptedState(t, response, "initialize case")
	requireTestStateVersion(t, state, 1)

	for _, step := range []struct {
		role    string
		version int
	}{
		{role: "plaintiff", version: 1},
		{role: "defendant", version: 2},
	} {
		authority := OpportunityAuthority{
			OpportunityID:        "openings:" + step.role,
			ExpectedStateVersion: step.version,
			Role:                 step.role,
			Phase:                "openings",
		}
		response, err = engine.Step(context.Background(), state, "record_opening_statement", step.role, authority, map[string]any{"text": "Opening."})
		if err != nil {
			t.Fatalf("%s opening transport: %v", step.role, err)
		}
		state = requireEngineAcceptedState(t, response, step.role+" opening")
		requireTestStateVersion(t, state, step.version+1)
	}
	caseState := requireTestMap(t, state["case"], "arguments case")
	if phase, _ := caseState["phase"].(string); phase != "arguments" {
		t.Fatalf("case phase = %q, want arguments", phase)
	}
	return state
}

func testSubmittedEvidencePayload(evidenceID, sha256 string) map[string]any {
	return map[string]any{
		"evidence_id":        evidenceID,
		"title":              "Derived evidence",
		"source_description": "Test source",
		"mime_type":          "text/plain",
		"relevance":          "Test relevance",
		"sha256":             sha256,
		"size_bytes":         8,
	}
}

func assertEngineAuthorityRejected(t *testing.T, engine Engine, state map[string]any, actionType string, actorRole string, authority OpportunityAuthority, payload map[string]any, want string) {
	t.Helper()
	response, err := engine.Step(context.Background(), state, actionType, actorRole, authority, payload)
	if err != nil {
		t.Fatalf("step transport: %v", err)
	}
	if ok, _ := response["ok"].(bool); ok {
		t.Fatalf("step accepted authority %#v", authority)
	}
	got, ok := response["error"].(string)
	if !ok {
		t.Fatalf("error = %#v, want string", response["error"])
	}
	if !strings.Contains(got, want) {
		t.Fatalf("error = %q, want substring %q", got, want)
	}
}

func requireEngineAcceptedState(t *testing.T, response map[string]any, label string) map[string]any {
	t.Helper()
	if ok, _ := response["ok"].(bool); !ok {
		t.Fatalf("%s rejected: %v", label, response["error"])
	}
	return requireTestMap(t, response["state"], label+" state")
}

func requireEngineRejected(t *testing.T, response map[string]any, want string) {
	t.Helper()
	if ok, _ := response["ok"].(bool); ok {
		t.Fatalf("engine accepted request, want error containing %q", want)
	}
	got, ok := response["error"].(string)
	if !ok {
		t.Fatalf("error = %#v, want string containing %q", response["error"], want)
	}
	if !strings.Contains(got, want) {
		t.Fatalf("error = %q, want substring %q", got, want)
	}
}

func requireTestStateVersion(t *testing.T, state map[string]any, want int) {
	t.Helper()
	got, ok := state["state_version"].(float64)
	if !ok || got != float64(want) {
		t.Fatalf("state_version = %#v, want %d", state["state_version"], want)
	}
}

func requireTestJSONEqual(t *testing.T, got, want any, label string) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal %s: %v", label, err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal expected %s: %v", label, err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("%s = %s, want %s", label, gotJSON, wantJSON)
	}
}

func requireTestSlice(t *testing.T, value any, label string) []any {
	t.Helper()
	result, ok := value.([]any)
	if !ok {
		t.Fatalf("%s = %#v, want array", label, value)
	}
	return result
}

func requireTestMap(t *testing.T, value any, label string) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", label, value)
	}
	return result
}
