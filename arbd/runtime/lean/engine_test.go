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
		"submit_council_answer",
		"council",
		authority,
		map[string]any{"member_id": "C3", "answer": 75},
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

func TestInitializeCaseUsesQuestion(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	requestPath := filepath.Join(dir, "request.json")
	script := `#!/bin/sh
request=$(cat)
printf '%s' "$request" > "$1"
printf '%s\n' '{"ok":true,"state":{"state_version":1}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	engine := New([]string{enginePath, requestPath})
	if _, err := engine.InitializeCase(context.Background(), map[string]any{"state_version": 0}, "Question?", nil); err != nil {
		t.Fatalf("initialize case: %v", err)
	}
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	var request map[string]any
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if request["question"] != "Question?" {
		t.Fatalf("question = %#v, want Question?", request["question"])
	}
	if _, ok := request["proposition"]; ok {
		t.Fatalf("request contains proposition: %#v", request)
	}
}

func TestCallClassifiesNonzeroExit(t *testing.T) {
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

	t.Run("exit one rejection", func(t *testing.T) {
		engine := New([]string{enginePath, `{"ok":false,"error":"rejected"}`, "protocol stderr", "1"})
		response, err := engine.Call(context.Background(), request)
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if ok, valid := response["ok"].(bool); !valid || ok {
			t.Fatalf("ok = %#v, want false", response["ok"])
		}
	})

	t.Run("rejection on another exit", func(t *testing.T) {
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
			t.Fatalf("error = %q, want rejection details", err)
		}
		if !strings.Contains(err.Error(), "stderr=protocol stderr") {
			t.Fatalf("error = %q, want stderr", err)
		}
	})

	t.Run("rejection after signal", func(t *testing.T) {
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
			t.Fatalf("error = %q, want rejection details", err)
		}
	})

	t.Run("success body on process failure", func(t *testing.T) {
		engine := New([]string{enginePath, `{"ok":true}`, "process stderr", "7"})
		response, err := engine.Call(context.Background(), request)
		if err == nil {
			t.Fatalf("call response = %#v, want process error", response)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
			t.Fatalf("error = %v, want exit status 7", err)
		}
		if !strings.Contains(err.Error(), "stderr=process stderr") {
			t.Fatalf("error = %q, want stderr", err)
		}
	})

	t.Run("invalid JSON on process failure", func(t *testing.T) {
		engine := New([]string{enginePath, `{`, "parse stderr", "8"})
		response, err := engine.Call(context.Background(), request)
		if err == nil {
			t.Fatalf("call response = %#v, want parse and process errors", response)
		}
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("error = %v, want JSON syntax error", err)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 8 {
			t.Fatalf("error = %v, want exit status 8", err)
		}
		if !strings.Contains(err.Error(), "stderr=parse stderr") {
			t.Fatalf("error = %q, want stderr", err)
		}
	})

	for _, test := range []struct {
		name   string
		stdout string
	}{
		{name: "missing ok", stdout: `{"state":{}}`},
		{name: "nonboolean ok", stdout: `{"ok":"false"}`},
		{name: "rejection missing error", stdout: `{"ok":false}`},
		{name: "rejection blank error", stdout: `{"ok":false,"error":" "}`},
		{name: "rejection nonstring error", stdout: `{"ok":false,"error":7}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := New([]string{enginePath, test.stdout, "synthetic stderr", "7"})
			response, err := engine.Call(context.Background(), request)
			if err == nil {
				t.Fatalf("call response = %#v, want process error", response)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
				t.Fatalf("error = %v, want exit status 7", err)
			}
			want := "lean process failed for next_opportunity: exit status 7 stderr=synthetic stderr"
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want substring %q", err, want)
			}
		})
	}
}

func TestCallPreservesZeroExitResponseWithoutOKField(t *testing.T) {
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
			t.Fatalf("error = %v, want JSON syntax error", err)
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
	childPID := readTestPID(t, childPath)
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
	childPID := readTestPID(t, childPath)
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
		t.Fatalf("error = %q, want rejection details", err)
	}
	waitForTestProcessTermination(t, childPID)
}

func TestCallRejectsSurvivingProcessGroupMember(t *testing.T) {
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
	childPID := readTestPID(t, childPath)
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
		t.Fatalf("error = %q, want rejection details", err)
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

func readTestPID(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse child pid: %v", err)
	}
	return pid
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
	initialized := initializeTestCase(t, engine, testInitialState())
	correct := OpportunityAuthority{
		OpportunityID:        "openings:plaintiff",
		ExpectedStateVersion: 1,
		Role:                 "plaintiff",
		Phase:                "openings",
	}
	response, err := engine.Step(context.Background(), initialized, "record_opening_statement", "plaintiff", correct, map[string]any{"text": "Opening."})
	if err != nil {
		t.Fatalf("correct authority step: %v", err)
	}
	requireEngineAcceptedState(t, response, "correct authority")

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
	assertEngineAuthorityRejected(t, engine, initialized, "submit_council_answer", "council", wrongMember, map[string]any{
		"member_id": "C2",
		"answer":    75,
		"rationale": "Reason.",
	}, "does not match current opportunity deliberation:1:C1")
	correctMember := wrongMember
	correctMember.OpportunityID = "deliberation:1:C1"
	correctMember.MemberID = "C1"
	assertEngineAuthorityRejected(t, engine, initialized, "submit_council_answer", "council", correctMember, map[string]any{
		"member_id": "C2",
		"answer":    75,
		"rationale": "Reason.",
	}, "action member_id C2 does not match current member_id C1")
}

func TestEnginePreservesAndValidatesInitialEvidenceCatalog(t *testing.T) {
	engine := testRealEngine(t)
	catalog := []map[string]any{
		{"evidence_id": "E-second", "sha256": strings.Repeat("b", 64), "size_bytes": 0},
		{"evidence_id": "E-first", "sha256": strings.Repeat("a", 64), "size_bytes": 17},
	}
	initial := testInitialState()
	initial["evidence_catalog"] = catalog
	state := initializeTestCase(t, engine, initial)
	requireTestJSONEqual(t, state["evidence_catalog"], catalog, "evidence_catalog")

	tests := []struct {
		name    string
		catalog []map[string]any
		want    string
	}{
		{
			name: "duplicate identifier",
			catalog: []map[string]any{
				{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 64), "size_bytes": 1},
				{"evidence_id": "E-valid", "sha256": strings.Repeat("b", 64), "size_bytes": 2},
			},
			want: "evidence_catalog contains duplicate evidence_id",
		},
		{
			name:    "untrimmed identifier",
			catalog: []map[string]any{{"evidence_id": " E-valid", "sha256": strings.Repeat("a", 64), "size_bytes": 1}},
			want:    "evidence_catalog evidence_id must be nonempty and trimmed",
		},
		{
			name:    "uppercase digest",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("A", 64), "size_bytes": 1}},
			want:    "evidence_catalog sha256 must contain exactly 64 lowercase hexadecimal characters",
		},
		{
			name:    "wrong size type",
			catalog: []map[string]any{{"evidence_id": "E-valid", "sha256": strings.Repeat("a", 64), "size_bytes": "1"}},
			want:    "size_bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial := testInitialState()
			initial["evidence_catalog"] = tt.catalog
			response, err := engine.InitializeCase(context.Background(), initial, "The question is proved.", testCouncilMembers())
			if err != nil {
				t.Fatalf("initialize case transport: %v", err)
			}
			requireEngineRejected(t, response, tt.want)
		})
	}
}

func TestEngineAcceptsSurrebuttalMaterials(t *testing.T) {
	engine := testRealEngine(t)
	initial := testInitialState()
	initial["evidence_catalog"] = []map[string]any{
		{"evidence_id": "E-known", "sha256": strings.Repeat("a", 64), "size_bytes": 16},
	}
	state := advanceTestCaseToSurrebuttal(t, engine, initial)
	authority := OpportunityAuthority{
		OpportunityID:        "surrebuttals:defendant",
		ExpectedStateVersion: 6,
		Role:                 "defendant",
		Phase:                "surrebuttals",
	}
	response, err := engine.Step(context.Background(), state, "submit_surrebuttal", "defendant", authority, map[string]any{
		"text": "Surrebuttal.",
		"offered_evidence": []map[string]any{
			{"evidence_id": "E-known", "label": "Known exhibit"},
		},
		"technical_reports": []map[string]any{
			{"title": "Report", "summary": "Summary."},
		},
	})
	if err != nil {
		t.Fatalf("submit surrebuttal transport: %v", err)
	}
	accepted := requireEngineAcceptedState(t, response, "surrebuttal materials")
	requireTestStateVersion(t, accepted, 7)
	caseState := requireTestMap(t, accepted["case"], "accepted case")
	if got := len(requireTestSlice(t, caseState["offered_evidence"], "offered_evidence")); got != 1 {
		t.Fatalf("offered_evidence length = %d, want 1", got)
	}
	if got := len(requireTestSlice(t, caseState["technical_reports"], "technical_reports")); got != 1 {
		t.Fatalf("technical_reports length = %d, want 1", got)
	}

	evidenceResponse, err := engine.Step(context.Background(), state, "submit_evidence", "defendant", authority, testSubmittedEvidencePayload("E-surrebuttal", strings.Repeat("b", 64)))
	if err != nil {
		t.Fatalf("submit surrebuttal evidence transport: %v", err)
	}
	evidenceState := requireEngineAcceptedState(t, evidenceResponse, "surrebuttal evidence")
	requireTestStateVersion(t, evidenceState, 7)
	evidenceCase := requireTestMap(t, evidenceState["case"], "evidence case")
	if got := len(requireTestSlice(t, evidenceCase["submitted_evidence"], "submitted_evidence")); got != 1 {
		t.Fatalf("submitted_evidence length = %d, want 1", got)
	}
}

func TestEngineValidatesSubmittedEvidenceLineage(t *testing.T) {
	engine := testRealEngine(t)
	parentSHA := strings.Repeat("a", 64)
	derivedSHA := strings.Repeat("b", 64)
	initial := testInitialState()
	initial["evidence_catalog"] = []map[string]any{
		{"evidence_id": "E-parent", "sha256": parentSHA, "size_bytes": 12},
	}
	arguments := advanceTestCaseToArguments(t, engine, initial)
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

	missingDigest := testSubmittedEvidencePayload("E-missing-digest", derivedSHA)
	missingDigest["parent_evidence_id"] = "E-parent"
	missingDigest["derivation_method"] = "excerpt"
	response, err = engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, missingDigest)
	if err != nil {
		t.Fatalf("invalid lineage transport: %v", err)
	}
	requireEngineRejected(t, response, "submitted evidence parent derivation requires parent_evidence_id, parent_sha256, and derivation_method")

	collision := testSubmittedEvidencePayload("E-parent", derivedSHA)
	response, err = engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, collision)
	if err != nil {
		t.Fatalf("identifier collision transport: %v", err)
	}
	requireEngineRejected(t, response, "submitted evidence_id collides with evidence_catalog: E-parent")
}

func TestEngineRejectsInvalidOptionalStringTypes(t *testing.T) {
	engine := testRealEngine(t)
	arguments := advanceTestCaseToArguments(t, engine, testInitialState())
	authority := OpportunityAuthority{
		OpportunityID:        "arguments:plaintiff",
		ExpectedStateVersion: 3,
		Role:                 "plaintiff",
		Phase:                "arguments",
	}

	response, err := engine.Step(context.Background(), arguments, "submit_argument", "plaintiff", authority, map[string]any{
		"text": "Argument.",
		"offered_evidence": []map[string]any{
			{"evidence_id": "E-unknown", "label": 1},
		},
	})
	if err != nil {
		t.Fatalf("invalid label transport: %v", err)
	}
	requireEngineRejected(t, response, "label must be a string")

	payload := testSubmittedEvidencePayload("E-invalid", strings.Repeat("b", 64))
	payload["source_url"] = 1
	response, err = engine.Step(context.Background(), arguments, "submit_evidence", "plaintiff", authority, payload)
	if err != nil {
		t.Fatalf("invalid source URL transport: %v", err)
	}
	requireEngineRejected(t, response, "source_url must be a string")
}

func testInitialState() map[string]any {
	return map[string]any{
		"schema_version": "v1",
		"forum_name":     "AARD Test Forum",
		"case": map[string]any{
			"case_id":            "aard-test",
			"caption":            "Plaintiff v. Defendant",
			"question":           "",
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
			"council_answers":    []map[string]any{},
			"failure":            nil,
		},
		"policy": map[string]any{
			"council_size":                    3,
			"judgment_standard":               "Answer with one integer from 0 through 100.",
			"max_opening_chars":               4_000,
			"max_argument_chars":              6_000,
			"max_rebuttal_chars":              4_000,
			"max_surrebuttal_chars":           4_000,
			"max_closing_chars":               5_000,
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
	enginePath := filepath.Clean(filepath.Join("..", "..", ".bin", "aardengine"))
	if _, err := os.Stat(enginePath); err != nil {
		t.Fatalf("required engine %s is unavailable: %v", enginePath, err)
	}
	return New([]string{enginePath})
}

func initializeTestCase(t *testing.T, engine Engine, initial map[string]any) map[string]any {
	t.Helper()
	response, err := engine.InitializeCase(context.Background(), initial, "The question is proved.", testCouncilMembers())
	if err != nil {
		t.Fatalf("initialize case transport: %v", err)
	}
	state := requireEngineAcceptedState(t, response, "initialize case")
	requireTestStateVersion(t, state, 1)
	return state
}

func advanceTestCaseToArguments(t *testing.T, engine Engine, initial map[string]any) map[string]any {
	t.Helper()
	state := initializeTestCase(t, engine, initial)
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
		response, err := engine.Step(context.Background(), state, "record_opening_statement", step.role, authority, map[string]any{"text": "Opening."})
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

func advanceTestCaseToSurrebuttal(t *testing.T, engine Engine, initial map[string]any) map[string]any {
	t.Helper()
	state := advanceTestCaseToArguments(t, engine, initial)
	for _, step := range []struct {
		role    string
		version int
	}{
		{role: "plaintiff", version: 3},
		{role: "defendant", version: 4},
	} {
		authority := OpportunityAuthority{
			OpportunityID:        "arguments:" + step.role,
			ExpectedStateVersion: step.version,
			Role:                 step.role,
			Phase:                "arguments",
		}
		response, err := engine.Step(context.Background(), state, "submit_argument", step.role, authority, map[string]any{"text": "Argument."})
		if err != nil {
			t.Fatalf("%s argument transport: %v", step.role, err)
		}
		state = requireEngineAcceptedState(t, response, step.role+" argument")
		requireTestStateVersion(t, state, step.version+1)
	}
	passAuthority := OpportunityAuthority{
		OpportunityID:        "rebuttals:plaintiff",
		ExpectedStateVersion: 5,
		Role:                 "plaintiff",
		Phase:                "rebuttals",
	}
	response, err := engine.Step(context.Background(), state, "pass_phase_opportunity", "plaintiff", passAuthority, map[string]any{})
	if err != nil {
		t.Fatalf("pass rebuttal transport: %v", err)
	}
	state = requireEngineAcceptedState(t, response, "pass rebuttal")
	requireTestStateVersion(t, state, 6)
	caseState := requireTestMap(t, state["case"], "surrebuttal case")
	if phase, _ := caseState["phase"].(string); phase != "surrebuttals" {
		t.Fatalf("case phase = %q, want surrebuttals", phase)
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
	requireEngineRejected(t, response, want)
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
