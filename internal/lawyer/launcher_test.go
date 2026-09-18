package lawyer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/runstate"
)

func TestHeadlessSuccessMarkerRequiresVerifiedCompletion(t *testing.T) {
	dir := t.TempDir()
	credentials := writeCodexCredentials(t, dir)
	supervisor := newTestSupervisor(t, dir, []string{"HOME=" + dir})
	verificationErr := errors.New("role obligation remains open")
	profile := Profile{
		Runner: RunnerCodex,
		Headless: headless.Profile{
			Runner:  headless.RunnerCodex,
			Command: "true",
			Auth:    headless.Auth{CredentialsFile: credentials},
		},
	}
	assignment := testAssignment("plaintiff", func(context.Context, string, string) error {
		return verificationErr
	})
	if err := supervisor.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-supervisor.Errors():
		if !errors.Is(err, verificationErr) {
			t.Fatalf("process error %v does not preserve verification error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("premature lawyer exit produced no error")
	}
	if err := supervisor.Stop(); !errors.Is(err, verificationErr) {
		t.Fatalf("stop error = %v", err)
	}

	marker := filepath.Join(dir, "agents", "plaintiff", "codex", "successful-invocation")
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("premature lawyer exit created marker %s: %v", marker, err)
	}
	second, err := headless.Prepare(profile.Headless, headless.Assignment{
		StateDir: filepath.Join(dir, "agents", "plaintiff"),
		WorkDir:  filepath.Join(dir, "agents", "plaintiff", "work"),
		Prompt:   assignment.Prompt,
		MCP:      assignment.MCP,
	}, []string{"HOME=" + dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := second.Cleanup(); err != nil {
			t.Errorf("cleanup second invocation: %v", err)
		}
	})
	if second.Resumed {
		t.Fatal("premature lawyer exit made the next invocation resumable")
	}
}

func TestHeadlessSuccessfulExitRetainsSessionAndCleansSecrets(t *testing.T) {
	dir := t.TempDir()
	credentials := writeCodexCredentials(t, dir)
	command := fakeCodexCommand(t)
	supervisor := newTestSupervisor(t, dir, []string{"HOME=" + dir})
	verified := make(chan struct{}, 1)
	profile := Profile{
		Runner: RunnerCodex,
		Headless: headless.Profile{
			Runner:  headless.RunnerCodex,
			Command: command,
			Auth:    headless.Auth{CredentialsFile: credentials},
		},
	}
	assignment := testAssignment("defendant", func(_ context.Context, role, name string) error {
		if role != "defendant" || name != "codex-defendant" {
			return fmt.Errorf("verification identity = %s, %s", role, name)
		}
		verified <- struct{}{}
		return nil
	})
	if err := supervisor.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	waitForProcesses(t, supervisor)
	if err := supervisor.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-verified:
	default:
		t.Fatal("exit verifier was not called")
	}
	select {
	case err := <-supervisor.Errors():
		t.Fatalf("unexpected process error: %v", err)
	default:
	}

	runnerDir := filepath.Join(dir, "agents", "defendant", "codex")
	if _, err := os.Stat(filepath.Join(runnerDir, "successful-invocation")); err != nil {
		t.Fatalf("inspect success marker: %v", err)
	}
	for _, secret := range []string{filepath.Join(runnerDir, "home", "auth.json"), filepath.Join(runnerDir, "home", "config.toml")} {
		if _, err := os.Stat(secret); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("staged secret remains at %s: %v", secret, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "defendant", "work")); err != nil {
		t.Fatalf("retained work directory: %v", err)
	}
}

func TestHeadlessStateIsSeparatedByRole(t *testing.T) {
	dir := t.TempDir()
	credentials := writeCodexCredentials(t, dir)
	command := fakeCodexCommand(t)
	supervisor := newTestSupervisor(t, dir, []string{"HOME=" + dir})
	profile := Profile{
		Runner: RunnerCodex,
		Headless: headless.Profile{
			Runner:  headless.RunnerCodex,
			Command: command,
			Auth:    headless.Auth{CredentialsFile: credentials},
		},
	}
	for _, role := range []string{"plaintiff", "defendant"} {
		if err := supervisor.Start(context.Background(), profile, testAssignment(role, func(context.Context, string, string) error { return nil })); err != nil {
			t.Fatalf("start %s: %v", role, err)
		}
	}
	waitForProcesses(t, supervisor)
	if err := supervisor.Stop(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"plaintiff", "defendant"} {
		marker := filepath.Join(dir, "agents", role, "codex", "successful-invocation")
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("inspect %s marker: %v", role, err)
		}
	}
}

func TestHeadlessUsesAssignmentEnvironment(t *testing.T) {
	dir := t.TempDir()
	credentials := writeCodexCredentials(t, dir)
	captured := filepath.Join(dir, "child.env")
	command := filepath.Join(dir, "codex")
	script := "#!/bin/sh\nenv > \"$CAPTURE_ENV_PATH\"\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}'\n"
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	supervisor := newTestSupervisor(t, dir, []string{
		"HOME=" + dir,
		"OPPOSING_LAWYER_SECRET=exposed",
	})
	profile := Profile{
		Runner: RunnerCodex,
		Headless: headless.Profile{
			Runner:  headless.RunnerCodex,
			Command: command,
			Auth:    headless.Auth{CredentialsFile: credentials},
		},
	}
	assignment := testAssignment("plaintiff", func(context.Context, string, string) error { return nil })
	assignment.Environment = []string{
		"HOME=" + dir,
		"CAPTURE_ENV_PATH=" + captured,
	}
	if err := supervisor.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	waitForProcesses(t, supervisor)
	if err := supervisor.Stop(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(captured)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "OPPOSING_LAWYER_SECRET=") {
		t.Fatalf("child environment contains opposing lawyer credential: %s", raw)
	}
}

func TestOpenClawRetainsWorkspaceWithoutHostState(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "docker")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s\\n' '{\"meta\":{\"completion\":{\"stopReason\":\"stop\",\"finishReason\":\"stop\",\"refusal\":false}}}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(dir, "state")
	workDir := filepath.Join(dir, "work")
	evidenceDir := filepath.Join(dir, "evidence")
	for _, path := range []string{workDir, evidenceDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	workFile := filepath.Join(workDir, "analysis.txt")
	if err := os.WriteFile(workFile, []byte("analysis"), 0o600); err != nil {
		t.Fatal(err)
	}

	observer := &capturedManagementObserver{}
	supervisor := newTestSupervisor(t, dir, []string{"SELECTED_OPENCLAW_KEY=selected"})
	supervisor.runtime.Observer = observer
	assignment := testAssignment("plaintiff", func(context.Context, string, string) error { return nil })
	assignment.RunID = "run-1"
	assignment.StateDir = stateRoot
	assignment.WorkDir = workDir
	assignment.EvidenceDir = evidenceDir
	profile := Profile{
		Runner: RunnerOpenClaw,
		OpenClaw: OpenClawProfile{
			Command:                  command,
			Image:                    "openclaw-image",
			Model:                    "gpt-5.5",
			Thinking:                 "xhigh",
			AgentTimeoutSeconds:      900,
			LawyerTurnTimeoutSeconds: 900,
			Auth: OpenClawAuth{
				Mode:      OpenClawAuthAPIKey,
				APIKeyEnv: "SELECTED_OPENCLAW_KEY",
			},
		},
	}
	if err := supervisor.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	observer.mu.Lock()
	starts := append([]runstate.ParticipantStart(nil), observer.participantStarts...)
	finishes := append([]runstate.ParticipantFinish(nil), observer.participantFinishes...)
	observer.mu.Unlock()
	if len(starts) != 1 || starts[0].StateDir != "" || starts[0].WorkDir != workDir {
		t.Fatalf("participant starts = %#v", starts)
	}
	if len(finishes) != 1 || finishes[0].StateBytes != nil {
		t.Fatalf("participant finishes = %#v", finishes)
	}
	if _, err := os.Stat(workFile); err != nil {
		t.Fatalf("inspect retained file %q: %v", workFile, err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, string(RunnerOpenClaw))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("host OpenClaw state exists: %v", err)
	}
}

func TestValidateAssignmentRejectsInvalidMCPServer(t *testing.T) {
	assignment := testAssignment("plaintiff", func(context.Context, string, string) error { return nil })
	for _, test := range []struct {
		name string
		mcp  headless.MCPServer
		want string
	}{
		{name: "name", mcp: headless.MCPServer{Name: "invalid name", URL: assignment.MCP.URL}, want: "server name"},
		{name: "scheme", mcp: headless.MCPServer{Name: assignment.MCP.Name, URL: "file:///tmp/mcp"}, want: "absolute HTTP or HTTPS"},
		{name: "userinfo", mcp: headless.MCPServer{Name: assignment.MCP.Name, URL: "https://user@example.test/mcp"}, want: "user information"},
		{name: "fragment", mcp: headless.MCPServer{Name: assignment.MCP.Name, URL: "https://example.test/mcp#fragment"}, want: "fragment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := assignment
			candidate.MCP = test.mcp
			if err := validateAssignment(candidate); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
}

func TestProcessExitPreservesEveryError(t *testing.T) {
	terminalErr := errors.New("terminal failed")
	waitErr := errors.New("wait failed")
	stopErr := errors.New("stop failed")
	stdoutErr := errors.New("stdout close failed")
	stderrErr := errors.New("stderr close failed")
	cleanupErr := errors.New("cleanup failed")
	statusErr := errors.New("status failed")
	markerErr := errors.New("marker failed")
	participantErr := errors.New("participant failed")
	recordErr := errors.New("record failed")
	exit := processExit{
		terminalErr:    terminalErr,
		waitErr:        waitErr,
		stopErr:        stopErr,
		stdoutErr:      stdoutErr,
		stderrErr:      stderrErr,
		cleanupErr:     cleanupErr,
		statusErr:      statusErr,
		markerErr:      markerErr,
		participantErr: participantErr,
		recordErr:      recordErr,
	}
	for _, want := range []error{terminalErr, waitErr, stopErr, stdoutErr, stderrErr, cleanupErr, statusErr, markerErr, participantErr, recordErr} {
		if !errors.Is(exit.err(), want) {
			t.Fatalf("combined error %v does not preserve %v", exit.err(), want)
		}
	}
	causes := exit.err().(interface{ Unwrap() []error }).Unwrap()
	if len(causes) == 0 || causes[0] != terminalErr {
		t.Fatalf("first combined error = %v, want %v", causes, terminalErr)
	}
	if errors.Is(exit.finalizationErr(), waitErr) {
		t.Fatalf("forced-stop finalization error includes process wait error: %v", exit.finalizationErr())
	}
	if !errors.Is(exit.finalizationErr(), terminalErr) {
		t.Fatalf("forced-stop finalization error omits terminal error: %v", exit.finalizationErr())
	}
	if !errors.Is(exit.finalizationErr(), stopErr) {
		t.Fatalf("forced-stop finalization error omits container stop error: %v", exit.finalizationErr())
	}
}

func TestHeadlessParticipantObservationAndResumption(t *testing.T) {
	dir := t.TempDir()
	credentials := writeCodexCredentials(t, dir)
	command := filepath.Join(dir, "fake-codex")
	script := "#!/bin/sh\nprintf 'session' > \"$CODEX_HOME/session.json\"\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":100,\"cached_input_tokens\":40,\"output_tokens\":20,\"reasoning_output_tokens\":5}}'\n"
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	profile := Profile{
		Runner: RunnerCodex,
		Headless: headless.Profile{
			Runner:          headless.RunnerCodex,
			Command:         command,
			ReasoningEffort: "xhigh",
			Auth:            headless.Auth{CredentialsFile: credentials},
		},
	}
	assignment := testAssignment("plaintiff", func(context.Context, string, string) error { return nil })
	assignment.Profile = "usual-lawyer"
	assignment.StateDir = filepath.Join(dir, "retained", "plaintiff")

	firstObserver := &capturedManagementObserver{}
	first := newTestSupervisor(t, filepath.Join(dir, "first"), []string{"HOME=" + dir})
	first.runtime.Observer = firstObserver
	if err := first.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	waitForProcesses(t, first)
	if err := first.Stop(); err != nil {
		t.Fatal(err)
	}
	firstObserver.mu.Lock()
	starts := append([]runstate.ParticipantStart(nil), firstObserver.participantStarts...)
	finishes := append([]runstate.ParticipantFinish(nil), firstObserver.participantFinishes...)
	firstObserver.mu.Unlock()
	if len(starts) != 1 || starts[0].Role != "plaintiff" || starts[0].Profile != "usual-lawyer" || starts[0].Runner != "codex" || starts[0].ReasoningEffort != "xhigh" || starts[0].Resumed {
		t.Fatalf("participant starts = %#v", starts)
	}
	wantStateDir := filepath.Join(assignment.StateDir, "codex")
	if starts[0].StateDir != wantStateDir {
		t.Fatalf("state directory = %q, want %q", starts[0].StateDir, wantStateDir)
	}
	wantWorkDir := filepath.Join(assignment.StateDir, "work")
	if starts[0].WorkDir != wantWorkDir {
		t.Fatalf("work directory = %q, want %q", starts[0].WorkDir, wantWorkDir)
	}
	retainedBytes, err := regularFileBytes(wantStateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(finishes) != 1 || finishes[0].Usage == nil || finishes[0].Usage.InputTokens != 100 || finishes[0].Usage.TotalTokens != 120 || finishes[0].StateBytes == nil || *finishes[0].StateBytes != retainedBytes || finishes[0].Error != "" {
		t.Fatalf("participant finishes = %#v", finishes)
	}

	secondObserver := &capturedManagementObserver{}
	second := newTestSupervisor(t, filepath.Join(dir, "second"), []string{"HOME=" + dir})
	second.runtime.Observer = secondObserver
	if err := second.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	waitForProcesses(t, second)
	if err := second.Stop(); err != nil {
		t.Fatal(err)
	}
	secondObserver.mu.Lock()
	secondStarts := append([]runstate.ParticipantStart(nil), secondObserver.participantStarts...)
	secondFinishes := append([]runstate.ParticipantFinish(nil), secondObserver.participantFinishes...)
	secondObserver.mu.Unlock()
	if len(secondStarts) != 1 || !secondStarts[0].Resumed {
		t.Fatalf("second participant starts = %#v", secondStarts)
	}
	if len(secondFinishes) != 1 || secondFinishes[0].Usage == nil || secondFinishes[0].Usage.TotalTokens != 0 || secondFinishes[0].Error != "" {
		t.Fatalf("second participant finishes = %#v", secondFinishes)
	}

	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s\\n' '{\"type\":\"thread.started\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	third := newTestSupervisor(t, filepath.Join(dir, "third"), []string{"HOME=" + dir})
	if err := third.Start(context.Background(), profile, assignment); err != nil {
		t.Fatal(err)
	}
	waitForProcesses(t, third)
	if err := third.Stop(); err == nil || !strings.Contains(err.Error(), "usage stream has no turn.completed") {
		t.Fatalf("third stop error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(wantStateDir, "successful-invocation")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed invocation retained success marker: %v", err)
	}
}

func TestParticipantObservationErrorFailsProcess(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	observer := &capturedManagementObserver{}
	supervisor := newTestSupervisor(t, dir, nil)
	supervisor.runtime.Observer = observer
	process, err := supervisor.startProcess(context.Background(), "codex-plaintiff", "codex", "true", nil, "", processStartOptions{
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
		participant: &runstate.ParticipantStart{
			Role:     "plaintiff",
			Profile:  "usual-lawyer",
			Runner:   "codex",
			StateDir: stateDir,
		},
		readUsage: func(path string) (*runstate.TokenUsage, error) {
			return headless.ReadUsageFile(headless.RunnerCodex, path)
		},
		stateDir: stateDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	if err := supervisor.Wait(context.Background()); err == nil || !strings.Contains(err.Error(), "usage stream has no turn.completed") {
		t.Fatalf("wait error = %v", err)
	}
	if err := supervisor.Stop(); err == nil || !strings.Contains(err.Error(), "usage stream has no turn.completed") {
		t.Fatalf("stop error = %v", err)
	}
	observer.mu.Lock()
	finishes := append([]runstate.ParticipantFinish(nil), observer.participantFinishes...)
	processFinishes := append([]runstate.ProcessFinish(nil), observer.finishes...)
	observer.mu.Unlock()
	if len(finishes) != 1 || !strings.Contains(finishes[0].Error, "usage stream has no turn.completed") {
		t.Fatalf("participant finishes = %#v", finishes)
	}
	if len(processFinishes) != 1 || processFinishes[0].State != runstate.Failed || !strings.Contains(processFinishes[0].Error, "usage stream has no turn.completed") {
		t.Fatalf("process finishes = %#v", processFinishes)
	}
}

func TestProviderRefusalPrecedesRoleVerificationAndRecordsContext(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state", "pi")
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(sessionsDir, "2026-08-23T12-00-00-000Z_session-1.jsonl")
	if err := os.WriteFile(sessionPath, []byte("session\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	observer := &capturedManagementObserver{}
	supervisor := newTestSupervisor(t, dir, nil)
	supervisor.runtime.Observer = observer
	verified := false
	markedSuccessful := false
	markedUnsuccessful := false
	usage := &runstate.TokenUsage{InputTokens: 20, OutputTokens: 2, TotalTokens: 22}
	providerErr := &headless.ProviderRefusalError{
		Provider:         "anthropic",
		Model:            "claude-opus-4-8",
		StopReason:       "error",
		RawStopReason:    "refusal",
		WillRetry:        false,
		Category:         "cyber",
		Explanation:      "provider explanation",
		SessionID:        "session-1",
		RequestID:        "request-1",
		ResponseID:       "response-1",
		RefusedMessageID: "message-1",
	}
	process, err := supervisor.startProcess(context.Background(), "pi-defendant", "podman", "true", nil, "", processStartOptions{
		roleID: "defendant",
		verifyExit: func(context.Context, string, string) error {
			verified = true
			return errors.New("derivative missing argument error")
		},
		markSuccessful: func() error {
			markedSuccessful = true
			return nil
		},
		markUnsuccessful: func() error {
			markedUnsuccessful = true
			return nil
		},
		participant: &runstate.ParticipantStart{
			Role:     "defendant",
			Profile:  "pi-opus",
			Runner:   "pi",
			StateDir: stateDir,
		},
		readUsage: func(path string) (*runstate.TokenUsage, error) {
			if path != filepath.Join(dir, "logs", "pi-defendant.stdout") {
				t.Fatalf("usage path = %q", path)
			}
			return usage, providerErr
		},
		stateDir: stateDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	waitErr := supervisor.Wait(context.Background())
	var refusalErr *headless.ProviderRefusalError
	if !errors.As(waitErr, &refusalErr) || refusalErr != providerErr {
		t.Fatalf("wait error = %v", waitErr)
	}
	if verified {
		t.Fatal("role verifier ran after a provider refusal")
	}
	if markedSuccessful || !markedUnsuccessful {
		t.Fatalf("marker state: successful = %t, unsuccessful = %t", markedSuccessful, markedUnsuccessful)
	}
	observer.mu.Lock()
	participantFinishes := append([]runstate.ParticipantFinish(nil), observer.participantFinishes...)
	processFinishes := append([]runstate.ProcessFinish(nil), observer.finishes...)
	observer.mu.Unlock()
	if len(participantFinishes) != 1 {
		t.Fatalf("participant finishes = %#v", participantFinishes)
	}
	finish := participantFinishes[0]
	if finish.Usage == nil || finish.Usage.TotalTokens != 22 || finish.Error != "" || finish.Refusal == nil {
		t.Fatalf("participant finish = %#v", finish)
	}
	if finish.Refusal.Provider != "anthropic" || finish.Refusal.Model != "claude-opus-4-8" || finish.Refusal.RawStopReason != "refusal" || finish.Refusal.WillRetry || finish.Refusal.Category != "cyber" || finish.Refusal.SessionID != "session-1" || finish.Refusal.RequestID != "request-1" || finish.Refusal.ResponseID != "response-1" || finish.Refusal.RefusedMessageID != "message-1" || finish.Refusal.StdoutArtifact != "logs/pi-defendant.stdout" || finish.Refusal.SessionPath != sessionPath {
		t.Fatalf("participant refusal = %#v", finish.Refusal)
	}
	if len(processFinishes) != 1 || processFinishes[0].State != runstate.Failed || processFinishes[0].ExitCode == nil || *processFinishes[0].ExitCode != 0 || !strings.Contains(processFinishes[0].Error, "provider explanation") || strings.Contains(processFinishes[0].Error, "derivative missing argument") {
		t.Fatalf("process finishes = %#v", processFinishes)
	}
	if err := supervisor.Stop(); !errors.As(err, &refusalErr) {
		t.Fatalf("stop error = %v", err)
	}
}

func TestRetainedStateMeasurementErrorFailsProcess(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "missing-state")
	observer := &capturedManagementObserver{}
	supervisor := newTestSupervisor(t, dir, nil)
	supervisor.runtime.Observer = observer
	markedSuccessful := false
	markedUnsuccessful := false
	process, err := supervisor.startProcess(context.Background(), "codex-plaintiff", "codex", "true", nil, "", processStartOptions{
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
		participant: &runstate.ParticipantStart{
			Role:     "plaintiff",
			Profile:  "usual-lawyer",
			Runner:   "codex",
			StateDir: stateDir,
		},
		stateDir: stateDir,
		markSuccessful: func() error {
			markedSuccessful = true
			return nil
		},
		markUnsuccessful: func() error {
			markedUnsuccessful = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	if err := supervisor.Wait(context.Background()); err == nil || !strings.Contains(err.Error(), "measure retained state") {
		t.Fatalf("wait error = %v", err)
	}
	if !markedSuccessful || !markedUnsuccessful {
		t.Fatalf("marker state: successful = %t, unsuccessful = %t", markedSuccessful, markedUnsuccessful)
	}
	observer.mu.Lock()
	participantFinishes := append([]runstate.ParticipantFinish(nil), observer.participantFinishes...)
	processFinishes := append([]runstate.ProcessFinish(nil), observer.finishes...)
	observer.mu.Unlock()
	if len(participantFinishes) != 1 || !strings.Contains(participantFinishes[0].Error, "measure retained state") {
		t.Fatalf("participant finishes = %#v", participantFinishes)
	}
	if len(processFinishes) != 1 || processFinishes[0].State != runstate.Failed || !strings.Contains(processFinishes[0].Error, "measure retained state") {
		t.Fatalf("process finishes = %#v", processFinishes)
	}
}

func TestStartProcessRecordsLifecycleWithoutPIDFile(t *testing.T) {
	dir := t.TempDir()
	observer := &capturedProcessObserver{}
	supervisor := newTestSupervisor(t, dir, nil)
	supervisor.runtime.Observer = observer
	process, err := supervisor.startProcess(context.Background(), "agent", "test", "true", nil, "", processStartOptions{
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	if err := supervisor.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	observer.mu.Lock()
	starts := append([]runstate.ProcessStart(nil), observer.starts...)
	finishes := append([]runstate.ProcessFinish(nil), observer.finishes...)
	observer.mu.Unlock()
	if len(starts) != 1 || starts[0].Name != "agent" || starts[0].Role != "plaintiff" || starts[0].PID <= 0 {
		t.Fatalf("starts = %#v", starts)
	}
	if len(finishes) != 1 || finishes[0].State != runstate.Completed || finishes[0].ExitCode == nil || *finishes[0].ExitCode != 0 {
		t.Fatalf("finishes = %#v", finishes)
	}
	if _, err := os.Stat(filepath.Join(dir, "agent.pid")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("PID file stat error = %v", err)
	}
}

type capturedProcessObserver struct {
	mu       sync.Mutex
	starts   []runstate.ProcessStart
	finishes []runstate.ProcessFinish
}

type capturedManagementObserver struct {
	capturedProcessObserver
	participantStarts   []runstate.ParticipantStart
	participantFinishes []runstate.ParticipantFinish
	participantFinish   func()
}

func (o *capturedManagementObserver) StartParticipant(start runstate.ParticipantStart) (runstate.FinishParticipant, error) {
	o.mu.Lock()
	o.participantStarts = append(o.participantStarts, start)
	o.mu.Unlock()
	return func(finish runstate.ParticipantFinish) error {
		o.mu.Lock()
		o.participantFinishes = append(o.participantFinishes, finish)
		hook := o.participantFinish
		o.mu.Unlock()
		if hook != nil {
			hook()
		}
		return nil
	}, nil
}

func (o *capturedProcessObserver) StartProcess(start runstate.ProcessStart) (runstate.FinishProcess, error) {
	o.mu.Lock()
	o.starts = append(o.starts, start)
	o.mu.Unlock()
	return func(finish runstate.ProcessFinish) error {
		o.mu.Lock()
		o.finishes = append(o.finishes, finish)
		o.mu.Unlock()
		return nil
	}, nil
}

func TestStartProcessReturnsCleanupAndMarkerErrors(t *testing.T) {
	dir := t.TempDir()
	supervisor := newTestSupervisor(t, dir, nil)
	cleanupErr := errors.New("cleanup failed")
	statusCalled := false
	process, err := supervisor.startProcess(context.Background(), "agent", "test", "true", nil, "", processStartOptions{
		roleID:         "plaintiff",
		verifyExit:     func(context.Context, string, string) error { statusCalled = true; return nil },
		cleanup:        func() error { return cleanupErr },
		markSuccessful: func() error { return errors.New("marker must not run") },
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	select {
	case err := <-supervisor.Errors():
		if !errors.Is(err, cleanupErr) || strings.Contains(err.Error(), "marker must not run") {
			t.Fatalf("process error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup error was not reported")
	}
	if err := supervisor.Stop(); !errors.Is(err, cleanupErr) {
		t.Fatalf("stop error = %v", err)
	}
	if statusCalled {
		t.Fatal("completion verifier ran after cleanup failed")
	}

	markerErr := errors.New("marker failed")
	dir = t.TempDir()
	supervisor = newTestSupervisor(t, dir, nil)
	process, err = supervisor.startProcess(context.Background(), "agent", "test", "true", nil, "", processStartOptions{
		roleID:         "plaintiff",
		verifyExit:     func(context.Context, string, string) error { return nil },
		cleanup:        noCleanup,
		markSuccessful: func() error { return markerErr },
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	select {
	case err := <-supervisor.Errors():
		if !errors.Is(err, markerErr) {
			t.Fatalf("process error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("marker error was not reported")
	}
	if err := supervisor.Stop(); !errors.Is(err, markerErr) {
		t.Fatalf("stop error = %v", err)
	}
}

func TestWaitAndStopReuseSuccessfulProcessResult(t *testing.T) {
	dir := t.TempDir()
	supervisor := newTestSupervisor(t, dir, nil)
	process, err := supervisor.startProcess(context.Background(), "agent", "test", "true", nil, "", processStartOptions{
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
		cleanup:    noCleanup,
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	if err := supervisor.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if err := supervisor.Stop(); err != nil {
		t.Fatalf("Stop() after Wait() error = %v", err)
	}
}

func TestCancellationDoesNotMarkInvocationSuccessful(t *testing.T) {
	dir := t.TempDir()
	supervisor := newTestSupervisor(t, dir, nil)
	ctx, cancel := context.WithCancel(context.Background())
	marked := false
	usageInspected := false
	process, err := supervisor.startProcess(ctx, "agent", "test", "sleep", []string{"60"}, "", processStartOptions{
		roleID:         "plaintiff",
		verifyExit:     func(context.Context, string, string) error { return nil },
		cleanup:        noCleanup,
		markSuccessful: func() error { marked = true; return nil },
		readUsage: func(string) (*runstate.TokenUsage, error) {
			usageInspected = true
			return nil, errors.New("terminal output is incomplete")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	cancel()
	select {
	case <-process.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled process did not exit")
	}
	if err := supervisor.Stop(); err != nil {
		t.Fatal(err)
	}
	if marked {
		t.Fatal("canceled invocation was marked successful")
	}
	if usageInspected {
		t.Fatal("canceled invocation output was inspected for terminal usage")
	}
}

func TestCancellationDuringParticipantFinalizationRemovesSuccessMarker(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	observer := &capturedManagementObserver{participantFinish: cancel}
	supervisor := newTestSupervisor(t, dir, nil)
	supervisor.runtime.Observer = observer
	markerPresent := false
	markedSuccessful := 0
	markedUnsuccessful := 0
	process, err := supervisor.startProcess(ctx, "agent", "test", "true", nil, "", processStartOptions{
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
		cleanup:    noCleanup,
		participant: &runstate.ParticipantStart{
			Role:    "plaintiff",
			Profile: "lawyer",
			Runner:  "codex",
		},
		markSuccessful: func() error {
			markedSuccessful++
			markerPresent = true
			return nil
		},
		markUnsuccessful: func() error {
			markedUnsuccessful++
			markerPresent = false
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	if err := supervisor.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if markerPresent || markedSuccessful != 1 || markedUnsuccessful != 1 {
		t.Fatalf("marker state: present = %t, successful calls = %d, unsuccessful calls = %d", markerPresent, markedSuccessful, markedUnsuccessful)
	}
	observer.mu.Lock()
	finishes := append([]runstate.ProcessFinish(nil), observer.finishes...)
	observer.mu.Unlock()
	if len(finishes) != 1 || finishes[0].State != runstate.Canceled {
		t.Fatalf("process finishes = %#v", finishes)
	}
}

func TestPiContainerPassesOnlySelectedProviderKey(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	workDir := filepath.Join(dir, "work")
	evidenceDir := filepath.Join(dir, "evidence")
	for _, path := range []string{workDir, evidenceDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	baseEnv := []string{
		"HOME=" + dir,
		"SELECTED_PI_KEY=selected",
		"ADJ_MCP_COMMAND=/usr/local/bin/adc-mcp",
		"ADJ_MCP_URL=http://127.0.0.1:19000/mcp",
		"ADJ_MCP_BEARER_TOKEN=court-secret",
		"OPENROUTER_API_KEY=unselected",
		"OPENAI_API_KEY=unselected-openai",
		"EXA_API_KEY=unselected-exa",
		"PI_ALLOW_BROWSER_COOKIES=1",
		"FEYNMAN_ALLOW_BROWSER_COOKIES=1",
	}
	invocation, err := headless.Prepare(headless.Profile{
		Runner:     headless.RunnerPi,
		Model:      "openrouter/anthropic/claude-sonnet-4",
		MCPAdapter: "/adapter",
		Auth:       headless.Auth{Mode: headless.AuthAPIKey, APIKeyEnv: "SELECTED_PI_KEY"},
	}, headless.Assignment{
		StateDir:    stateDir,
		WorkDir:     workDir,
		EvidenceDir: evidenceDir,
		Prompt:      "serve the plaintiff",
		MCP: headless.MCPServer{
			Name:        "aar-case-1-plaintiff",
			URL:         "http://127.0.0.1:19000/mcp",
			BearerToken: "court-secret",
		},
	}, baseEnv)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := invocation.Cleanup(); err != nil {
			t.Errorf("cleanup invocation: %v", err)
		}
	})
	args, processEnv, err := piContainer(invocation, "aar-case-1-plaintiff-pi", "pi-image", baseEnv)
	if err != nil {
		t.Fatal(err)
	}
	joined := "\x00" + strings.Join(args, "\x00") + "\x00"
	for _, want := range []string{
		"\x00--name\x00aar-case-1-plaintiff-pi\x00",
		"\x00--user\x000:0\x00",
		"\x00-e\x00OPENROUTER_API_KEY\x00",
		"\x00-v\x00/usr/local/bin/adc-mcp:/opt/adj/mcp:ro\x00",
		"\x00-e\x00ADJ_MCP_COMMAND=/opt/adj/mcp\x00",
		"\x00-e\x00ADJ_MCP_URL\x00",
		"\x00-e\x00ADJ_MCP_BEARER_TOKEN\x00",
		"\x00-v\x00" + invocation.StateDir + ":/home/user/state\x00",
		"\x00-v\x00" + invocation.Dir + ":/home/user/work\x00",
		"\x00-v\x00" + invocation.EvidenceDir + ":/home/user/evidence:ro\x00",
		"\x00-w\x00/home/user/work\x00",
		"\x00pi-image\x00",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("Pi lawyer arguments lack %q: %#v", want, args)
		}
	}
	if strings.Contains(joined, "SELECTED_PI_KEY") || strings.Contains(joined, "OPENAI_API_KEY") || strings.Contains(joined, "selected") {
		t.Fatalf("Pi lawyer arguments expose an unselected variable or key: %#v", args)
	}
	if got, ok := environmentValue(processEnv, "OPENROUTER_API_KEY"); !ok || got != "selected" {
		t.Fatalf("selected provider key = %q, %v", got, ok)
	}
	if got, ok := environmentValue(processEnv, "ADJ_MCP_BEARER_TOKEN"); !ok || got != "court-secret" || strings.Contains(joined, "court-secret") {
		t.Fatal("MCP capability must pass through the environment")
	}
	for _, name := range []string{"SELECTED_PI_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "EXA_API_KEY", "PI_ALLOW_BROWSER_COOKIES", "FEYNMAN_ALLOW_BROWSER_COOKIES"} {
		if value, ok := environmentValue(processEnv, name); ok {
			t.Fatalf("process environment contains %s=%q", name, value)
		}
	}
}

func TestPiContainerSubscriptionUsesStagedCredentials(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	credentials := writePiCodexCredentials(t, dir)
	baseEnv := []string{
		"HOME=" + dir,
		"OPENAI_API_KEY=unselected-openai",
		"OPENROUTER_API_KEY=unselected-openrouter",
		"EXA_API_KEY=unselected-exa",
	}
	invocation, err := headless.Prepare(headless.Profile{
		Runner:     headless.RunnerPi,
		Provider:   headless.ProviderOpenAI,
		Model:      "openai/gpt-5.6-sol",
		MCPAdapter: "/adapter",
		Auth: headless.Auth{
			Mode:            headless.AuthSubscription,
			CredentialsFile: credentials,
		},
	}, headless.Assignment{
		StateDir: stateDir,
		WorkDir:  workDir,
		Prompt:   "serve the plaintiff",
		MCP: headless.MCPServer{
			Name: "aar-case-1-plaintiff",
			URL:  "http://127.0.0.1:19000/mcp",
		},
	}, baseEnv)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := invocation.Cleanup(); err != nil {
			t.Errorf("cleanup invocation: %v", err)
		}
	})
	args, processEnv, err := piContainer(invocation, "aar-case-1-plaintiff-pi", "pi-image", baseEnv)
	if err != nil {
		t.Fatal(err)
	}
	joined := "\x00" + strings.Join(args, "\x00") + "\x00"
	for _, want := range []string{
		"\x00--provider\x00openai-codex\x00",
		"\x00--model\x00gpt-5.6-sol\x00",
		"\x00-v\x00" + invocation.StateDir + ":/home/user/state\x00",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("Pi lawyer arguments lack %q: %#v", want, args)
		}
	}
	for _, forbidden := range []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "EXA_API_KEY"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("Pi lawyer arguments contain %s: %#v", forbidden, args)
		}
		if value, ok := environmentValue(processEnv, forbidden); ok {
			t.Fatalf("Pi process environment contains %s=%q", forbidden, value)
		}
	}
	authPath := filepath.Join(invocation.StateDir, "config", "auth.json")
	if _, err := os.Stat(authPath); err != nil {
		t.Fatalf("inspect staged Pi credentials: %v", err)
	}
}

func TestOpenClawAuthenticationIsStagedAndCleaned(t *testing.T) {
	dir := t.TempDir()
	source := writeCodexCredentials(t, dir)
	supervisor := newTestSupervisor(t, dir, []string{})
	args, prefix, processEnv, cleanup, err := supervisor.openClawAuth("openai", OpenClawAuth{
		Mode:          OpenClawAuthCodex,
		CodexAuthPath: source,
	}, "plaintiff", supervisor.runtime.BaseEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CODEX_HOME", "tokens.access_token", "openclaw models auth paste-token", "unset codex_token"} {
		if !strings.Contains(prefix, want) {
			t.Fatalf("authentication command lacks %q:\n%s", want, prefix)
		}
	}
	joined := strings.Join(args, "\n")
	if strings.Contains(joined, "OPENAI_API_KEY") || !strings.Contains(joined, "CODEX_HOME=/aar-codex") {
		t.Fatalf("authentication arguments = %#v", args)
	}
	if _, ok := environmentValue(processEnv, "OPENAI_API_KEY"); ok {
		t.Fatalf("subscription process environment contains OPENAI_API_KEY")
	}
	staged := filepath.Join(dir, "openclaw-plaintiff-codex", "auth.json")
	if !filepath.IsAbs(strings.SplitN(args[1], ":", 2)[0]) {
		t.Fatalf("mount path is not absolute: %q", args[1])
	}
	info, err := os.Stat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("staged auth mode = %o", info.Mode().Perm())
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged credentials remain: %v", err)
	}

	args, _, processEnv, cleanup, err = supervisor.openClawAuth("openai", OpenClawAuth{
		Mode:      OpenClawAuthAPIKey,
		APIKeyEnv: "SELECTED_OPENCLAW_KEY",
	}, "defendant", supervisor.runtime.BaseEnvironment)
	if err == nil {
		t.Fatal("missing selected API key was accepted")
	}
	_ = args
	_ = processEnv
	_ = cleanup

	supervisor.runtime.BaseEnvironment = []string{
		"SELECTED_OPENCLAW_KEY=selected",
		"OPENAI_API_KEY=unselected",
		"ANTHROPIC_API_KEY=unselected",
		"ANTHROPIC_BASE_URL=https://unselected.example.test",
	}
	args, _, processEnv, cleanup, err = supervisor.openClawAuth("openai", OpenClawAuth{
		Mode:      OpenClawAuthAPIKey,
		APIKeyEnv: "SELECTED_OPENCLAW_KEY",
	}, "defendant", supervisor.runtime.BaseEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(args, "\x00") != "-e\x00OPENAI_API_KEY" {
		t.Fatalf("API-key arguments = %#v", args)
	}
	if value, ok := environmentValue(processEnv, "OPENAI_API_KEY"); !ok || value != "selected" {
		t.Fatalf("OpenClaw API key = %q, %v", value, ok)
	}
	for _, name := range []string{"SELECTED_OPENCLAW_KEY", "ANTHROPIC_API_KEY"} {
		if _, ok := environmentValue(processEnv, name); ok {
			t.Fatalf("process environment contains %s", name)
		}
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}

	args, _, processEnv, cleanup, err = supervisor.openClawAuth("anthropic", OpenClawAuth{
		Mode:      OpenClawAuthAPIKey,
		APIKeyEnv: "SELECTED_OPENCLAW_KEY",
	}, "defendant", supervisor.runtime.BaseEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(args, "\x00") != "-e\x00ANTHROPIC_API_KEY" {
		t.Fatalf("Anthropic API-key arguments = %#v", args)
	}
	if value, ok := environmentValue(processEnv, "ANTHROPIC_API_KEY"); !ok || value != "selected" {
		t.Fatalf("OpenClaw Anthropic API key = %q, %v", value, ok)
	}
	for _, name := range []string{"SELECTED_OPENCLAW_KEY", "OPENAI_API_KEY", "ANTHROPIC_BASE_URL"} {
		if _, ok := environmentValue(processEnv, name); ok {
			t.Fatalf("Anthropic process environment contains %s", name)
		}
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenClawCommandConfiguration(t *testing.T) {
	webSearch := true
	command, err := openClawConfigPatchCommand("openai", 900, &webSearch)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(command, "\n")
	end := strings.Index(command, "\nJSON\n")
	if start < 0 || end <= start {
		t.Fatalf("configuration command has no JSON document: %q", command)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(command[start+1:end]), &patch); err != nil {
		t.Fatal(err)
	}
	appServer := patch["plugins"].(map[string]any)["entries"].(map[string]any)["codex"].(map[string]any)["config"].(map[string]any)["appServer"].(map[string]any)
	for _, name := range []string{"turnCompletionIdleTimeoutMs", "postToolRawAssistantCompletionIdleTimeoutMs"} {
		if appServer[name] != float64(900000) {
			t.Fatalf("%s = %#v", name, appServer[name])
		}
	}
	search := patch["tools"].(map[string]any)["web"].(map[string]any)["search"].(map[string]any)
	if search["enabled"] != true || len(search) != 4 {
		t.Fatalf("OpenClaw web search configuration = %#v", search)
	}
	openaiCodex := search["openaiCodex"].(map[string]any)
	if openaiCodex["enabled"] != true || openaiCodex["mode"] != "live" || len(openaiCodex) != 5 {
		t.Fatalf("OpenClaw live search configuration = %#v", search)
	}
	for _, name := range []string{"allowedDomains", "contextSize", "userLocation"} {
		if value, ok := openaiCodex[name]; !ok || value != nil {
			t.Fatalf("OpenClaw native search %s reset = %#v, present = %t", name, value, ok)
		}
	}
	for _, name := range []string{"provider", "apiKey"} {
		if value, ok := search[name]; !ok || value != nil {
			t.Fatalf("OpenClaw managed search %s reset = %#v, present = %t", name, value, ok)
		}
	}
	duckDuckGo := patch["plugins"].(map[string]any)["entries"].(map[string]any)["duckduckgo"].(map[string]any)
	if duckDuckGo["enabled"] != false {
		t.Fatalf("OpenClaw DuckDuckGo plugin configuration = %#v", duckDuckGo)
	}
	agents := patch["agents"].(map[string]any)["defaults"].(map[string]any)
	if agents["workspace"] != OpenClawWorkspacePath {
		t.Fatalf("OpenClaw workspace = %#v", agents["workspace"])
	}
	tools := patch["tools"].(map[string]any)
	if tools["profile"] != "full" || tools["exec"].(map[string]any)["mode"] != "full" {
		t.Fatalf("OpenClaw tool configuration = %#v", tools)
	}
	for _, name := range []string{"allow", "alsoAllow", "deny"} {
		if value, ok := tools[name]; !ok || value != nil {
			t.Fatalf("OpenClaw %s reset = %#v, present = %t", name, value, ok)
		}
	}
	webSearch = false
	disabledCommand, err := openClawConfigPatchCommand("openai", 900, &webSearch)
	if err != nil {
		t.Fatal(err)
	}
	disabledStart := strings.Index(disabledCommand, "\n")
	disabledEnd := strings.Index(disabledCommand, "\nJSON\n")
	var disabledPatch map[string]any
	if err := json.Unmarshal([]byte(disabledCommand[disabledStart+1:disabledEnd]), &disabledPatch); err != nil {
		t.Fatal(err)
	}
	disabledSearch := disabledPatch["tools"].(map[string]any)["web"].(map[string]any)["search"].(map[string]any)
	if disabledSearch["enabled"] != false || len(disabledSearch) != 4 {
		t.Fatalf("disabled OpenClaw web search configuration = %#v", disabledSearch)
	}
	for _, name := range []string{"provider", "apiKey", "openaiCodex"} {
		if value, ok := disabledSearch[name]; !ok || value != nil {
			t.Fatalf("disabled OpenClaw search %s reset = %#v, present = %t", name, value, ok)
		}
	}
	disabledDuckDuckGo := disabledPatch["plugins"].(map[string]any)["entries"].(map[string]any)["duckduckgo"].(map[string]any)
	if disabledDuckDuckGo["enabled"] != false {
		t.Fatalf("disabled OpenClaw DuckDuckGo plugin configuration = %#v", disabledDuckDuckGo)
	}

	webSearch = true
	anthropicCommand, err := openClawConfigPatchCommand("anthropic", 900, &webSearch)
	if err != nil {
		t.Fatal(err)
	}
	anthropicStart := strings.Index(anthropicCommand, "\n")
	anthropicEnd := strings.Index(anthropicCommand, "\nJSON\n")
	var anthropicPatch map[string]any
	if err := json.Unmarshal([]byte(anthropicCommand[anthropicStart+1:anthropicEnd]), &anthropicPatch); err != nil {
		t.Fatal(err)
	}
	anthropicSearch := anthropicPatch["tools"].(map[string]any)["web"].(map[string]any)["search"].(map[string]any)
	if anthropicSearch["enabled"] != true || anthropicSearch["provider"] != "duckduckgo" || len(anthropicSearch) != 4 {
		t.Fatalf("OpenClaw Anthropic search configuration = %#v", anthropicSearch)
	}
	for _, name := range []string{"apiKey", "openaiCodex"} {
		if value, ok := anthropicSearch[name]; !ok || value != nil {
			t.Fatalf("OpenClaw Anthropic search %s reset = %#v, present = %t", name, value, ok)
		}
	}
	duckDuckGo = anthropicPatch["plugins"].(map[string]any)["entries"].(map[string]any)["duckduckgo"].(map[string]any)
	if duckDuckGo["enabled"] != true {
		t.Fatalf("OpenClaw DuckDuckGo plugin configuration = %#v", duckDuckGo)
	}
	agentCommand := openClawAgentCommand("auth-prefix\n", "config-prefix\n", "gpt-5.5", "xhigh", 3600)
	for _, want := range []string{
		"auth-prefix\nconfig-prefix\nopenclaw mcp set",
		"AAR_OPENCLAW_AGENT_ATTEMPTS",
		`openclaw agent --local --model "gpt-5.5" --thinking "xhigh" --timeout 3600`,
		`grep -Eq 'stream disconnected before completion|LLM request timed out'`,
	} {
		if !strings.Contains(agentCommand, want) {
			t.Fatalf("agent command lacks %q:\n%s", want, agentCommand)
		}
	}
	args := openClawContainerArgs(OpenClawProfile{Network: "host"}, "aar-test", "/work", "/evidence")
	joined := "\x00" + strings.Join(args, "\x00") + "\x00"
	for _, want := range []string{
		"\x00--network\x00host\x00",
		"\x00--user\x000:0\x00",
		"\x00-e\x00HOME=/home/node\x00",
		"\x00-e\x00OPENCLAW_STATE_DIR=/home/node/.openclaw\x00",
		"\x00-e\x00OPENCLAW_CONFIG_PATH=/home/node/.openclaw/openclaw.json\x00",
		"\x00-e\x00AAR_RETAINED_WORKSPACE=/home/node/work\x00",
		"\x00-v\x00/work:/home/node/work\x00",
		"\x00-v\x00/evidence:/home/node/evidence:ro\x00",
		"\x00-w\x00/home/node/work\x00",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("OpenClaw container arguments lack %q: %#v", want, args)
		}
	}
	if strings.Contains(joined, ":"+openClawContainerState+"\x00") {
		t.Fatalf("OpenClaw state is mounted from the host: %#v", args)
	}
	if strings.Contains(joined, "host.docker.internal") {
		t.Fatalf("host-network arguments = %#v", args)
	}
	if got := openClawSessionKey("quick", Assignment{CaseID: "case-1", RunID: "run-2", RoleID: "plaintiff"}); got != "agent:quick:case-1:run-2:plaintiff" {
		t.Fatalf("OpenClaw session key = %q", got)
	}
}

func TestOpenClawSearchPatchClearsRetainedProviderState(t *testing.T) {
	config := map[string]any{
		"plugins": map[string]any{
			"entries": map[string]any{
				"codex": map[string]any{
					"enabled": false,
					"config":  map[string]any{"retained": true},
				},
				"duckduckgo": map[string]any{
					"enabled": true,
					"config":  map[string]any{"webSearch": map[string]any{"region": "us-en"}},
				},
			},
		},
		"tools": map[string]any{
			"web": map[string]any{
				"search": map[string]any{
					"enabled":         true,
					"provider":        "brave",
					"apiKey":          "retained-secret",
					"maxResults":      float64(6),
					"timeoutSeconds":  float64(12),
					"cacheTtlMinutes": float64(60),
					"openaiCodex": map[string]any{
						"enabled":        false,
						"mode":           "cached",
						"allowedDomains": []any{"retained.example"},
						"contextSize":    "low",
						"userLocation":   map[string]any{"country": "US"},
					},
				},
			},
		},
	}

	webSearch := true
	command, err := openClawConfigPatchCommand("openai", 900, &webSearch)
	if err != nil {
		t.Fatal(err)
	}
	applyOpenClawConfigPatch(config, decodeOpenClawConfigPatch(t, command))
	search := openClawSearchConfig(t, config)
	if _, ok := search["provider"]; ok {
		t.Fatalf("OpenAI search retained managed provider: %#v", search)
	}
	if _, ok := search["apiKey"]; ok {
		t.Fatalf("OpenAI search retained managed credential: %#v", search)
	}
	if got := search["openaiCodex"]; !reflect.DeepEqual(got, map[string]any{"enabled": true, "mode": "live"}) {
		t.Fatalf("OpenAI native search configuration = %#v", got)
	}
	assertRetainedOpenClawSearchPreferences(t, search)
	entries := config["plugins"].(map[string]any)["entries"].(map[string]any)
	if entries["duckduckgo"].(map[string]any)["enabled"] != false {
		t.Fatalf("OpenAI search retained enabled DuckDuckGo plugin: %#v", entries["duckduckgo"])
	}
	if entries["codex"].(map[string]any)["enabled"] != true || entries["codex"].(map[string]any)["config"].(map[string]any)["retained"] != true {
		t.Fatalf("OpenAI search did not preserve the Codex plugin: %#v", entries["codex"])
	}

	command, err = openClawConfigPatchCommand("anthropic", 900, &webSearch)
	if err != nil {
		t.Fatal(err)
	}
	applyOpenClawConfigPatch(config, decodeOpenClawConfigPatch(t, command))
	search = openClawSearchConfig(t, config)
	if search["provider"] != "duckduckgo" {
		t.Fatalf("Anthropic search provider = %#v", search["provider"])
	}
	for _, name := range []string{"apiKey", "openaiCodex"} {
		if _, ok := search[name]; ok {
			t.Fatalf("Anthropic search retained %s: %#v", name, search)
		}
	}
	assertRetainedOpenClawSearchPreferences(t, search)
	if entries["duckduckgo"].(map[string]any)["enabled"] != true {
		t.Fatalf("Anthropic search did not enable DuckDuckGo: %#v", entries["duckduckgo"])
	}

	webSearch = false
	command, err = openClawConfigPatchCommand("anthropic", 900, &webSearch)
	if err != nil {
		t.Fatal(err)
	}
	applyOpenClawConfigPatch(config, decodeOpenClawConfigPatch(t, command))
	search = openClawSearchConfig(t, config)
	if search["enabled"] != false {
		t.Fatalf("disabled search configuration = %#v", search)
	}
	for _, name := range []string{"provider", "apiKey", "openaiCodex"} {
		if _, ok := search[name]; ok {
			t.Fatalf("disabled search retained %s: %#v", name, search)
		}
	}
	assertRetainedOpenClawSearchPreferences(t, search)
	if entries["duckduckgo"].(map[string]any)["enabled"] != false {
		t.Fatalf("disabled search retained enabled DuckDuckGo: %#v", entries["duckduckgo"])
	}
}

func decodeOpenClawConfigPatch(t *testing.T, command string) map[string]any {
	t.Helper()
	start := strings.Index(command, "\n")
	end := strings.Index(command, "\nJSON\n")
	if start < 0 || end <= start {
		t.Fatalf("configuration command has no JSON document: %q", command)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(command[start+1:end]), &patch); err != nil {
		t.Fatal(err)
	}
	return patch
}

func applyOpenClawConfigPatch(config, patch map[string]any) {
	for name, value := range patch {
		if value == nil {
			delete(config, name)
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			config[name] = value
			continue
		}
		current, ok := config[name].(map[string]any)
		if !ok {
			current = map[string]any{}
			config[name] = current
		}
		applyOpenClawConfigPatch(current, object)
	}
}

func openClawSearchConfig(t *testing.T, config map[string]any) map[string]any {
	t.Helper()
	tools, ok := config["tools"].(map[string]any)
	if !ok {
		t.Fatalf("OpenClaw tools configuration = %#v", config["tools"])
	}
	web, ok := tools["web"].(map[string]any)
	if !ok {
		t.Fatalf("OpenClaw web configuration = %#v", tools["web"])
	}
	search, ok := web["search"].(map[string]any)
	if !ok {
		t.Fatalf("OpenClaw search configuration = %#v", web["search"])
	}
	return search
}

func assertRetainedOpenClawSearchPreferences(t *testing.T, search map[string]any) {
	t.Helper()
	for name, want := range map[string]any{
		"maxResults":      float64(6),
		"timeoutSeconds":  float64(12),
		"cacheTtlMinutes": float64(60),
	} {
		if search[name] != want {
			t.Fatalf("OpenClaw search preference %s = %#v, want %#v", name, search[name], want)
		}
	}
}

func TestOpenClawAgentCommandPreservesFailureStatus(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"openclaw": "#!/bin/sh\nif [ \"$1\" = mcp ]; then exit 0; fi\nprintf 'fatal agent error\\n' >&2\nexit 7\n",
	} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("sh", "-c", openClawAgentCommand("", "", "gpt-5.5", "xhigh", 900))
	command.Env = []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"AAR_RETAINED_WORKSPACE=/work",
		"AAR_MCP_NAME=court",
		"AAR_MCP_JSON={}",
		"AAR_SESSION_KEY=agent:quick:case:run:plaintiff",
		"AAR_ASSIGNMENT=analyze",
	}
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run OpenClaw command error = %v, output = %s", err, output)
	}
	if exitErr.ExitCode() != 7 {
		t.Fatalf("exit code = %d, want 7; output = %s", exitErr.ExitCode(), output)
	}
	if !strings.Contains(string(output), "fatal agent error") {
		t.Fatalf("output = %q", output)
	}
}

func TestOpenClawAgentCommandRetriesModelTimeout(t *testing.T) {
	tests := []struct {
		name              string
		succeedingAttempt string
		wantExit          int
	}{
		{name: "recovers", succeedingAttempt: "2"},
		{name: "exhausts attempts", succeedingAttempt: "0", wantExit: 9},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			binDir := filepath.Join(dir, "bin")
			if err := os.Mkdir(binDir, 0o700); err != nil {
				t.Fatal(err)
			}
			for name, script := range map[string]string{
				"openclaw": `#!/bin/sh
if [ "$1" = mcp ]; then
    exit 0
fi
attempt=0
if [ -f "$OPENCLAW_ATTEMPT_LOG" ]; then
    attempt="$(cat "$OPENCLAW_ATTEMPT_LOG")"
fi
attempt=$((attempt + 1))
printf '%s\n' "$attempt" > "$OPENCLAW_ATTEMPT_LOG"
if [ "$OPENCLAW_SUCCEEDING_ATTEMPT" -gt 0 ] && [ "$attempt" -ge "$OPENCLAW_SUCCEEDING_ATTEMPT" ]; then
    exit 0
fi
printf 'FailoverError: LLM request timed out.\n' >&2
exit 9
`,
				"sleep": "#!/bin/sh\nexit 0\n",
			} {
				if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			attemptLog := filepath.Join(dir, "attempts")
			command := exec.Command("sh", "-c", openClawAgentCommand("", "", "gpt-5.5", "xhigh", 900))
			command.Env = []string{
				"PATH=" + binDir + ":/usr/bin:/bin",
				"OPENCLAW_ATTEMPT_LOG=" + attemptLog,
				"OPENCLAW_SUCCEEDING_ATTEMPT=" + test.succeedingAttempt,
				"AAR_OPENCLAW_AGENT_ATTEMPTS=2",
				"AAR_RETAINED_WORKSPACE=/work",
				"AAR_MCP_NAME=court",
				"AAR_MCP_JSON={}",
				"AAR_SESSION_KEY=agent:quick:case:run:plaintiff",
				"AAR_ASSIGNMENT=analyze",
			}
			output, err := command.CombinedOutput()
			if test.wantExit == 0 {
				if err != nil {
					t.Fatalf("run OpenClaw command: %v: %s", err, output)
				}
			} else {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != test.wantExit {
					t.Fatalf("run OpenClaw command error = %v, want exit %d; output = %s", err, test.wantExit, output)
				}
			}
			raw, err := os.ReadFile(attemptLog)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(string(raw)); got != "2" {
				t.Fatalf("OpenClaw attempts = %q, want 2", got)
			}
			if !strings.Contains(string(output), "recoverable model transport error; retrying") {
				t.Fatalf("output = %q", output)
			}
		})
	}
}

func TestCanceledContainerRestoresWorkspaceOwnership(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	lockedWorkDir := filepath.Join(workDir, "quick")
	if err := os.MkdirAll(lockedWorkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	workFile := filepath.Join(lockedWorkDir, "analysis.txt")
	if err := os.WriteFile(workFile, []byte("retained work"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(lockedWorkDir, 0o000); err != nil {
		t.Fatal(err)
	}

	stopFile := filepath.Join(dir, "container-stopped")
	mainExitFile := filepath.Join(dir, "main-exited")
	cleanupArgsFile := filepath.Join(dir, "cleanup-args")
	for name, value := range map[string]string{
		"FAKE_STOP_FILE":       stopFile,
		"FAKE_MAIN_EXIT_FILE":  mainExitFile,
		"FAKE_CLEANUP_ARGS":    cleanupArgsFile,
		"FAKE_LOCKED_WORK_DIR": lockedWorkDir,
	} {
		t.Setenv(name, value)
	}
	runtimeCommand := filepath.Join(dir, "fake-docker")
	runtimeScript := `#!/bin/sh
set -eu
case "$1" in
    wait)
        while [ ! -e "$FAKE_STOP_FILE" ]; do
            sleep 0.01
        done
        : > "$FAKE_MAIN_EXIT_FILE"
        exit 137
        ;;
    container)
        if [ "$2" != "rm" ] || [ "$3" != "-f" ]; then
            echo "unexpected container command: $*" >&2
            exit 31
        fi
        : > "$FAKE_STOP_FILE"
        ;;
    run)
        if [ ! -e "$FAKE_MAIN_EXIT_FILE" ]; then
            echo "ownership cleanup ran before the main container exited" >&2
            exit 32
		fi
		printf '%s\n' "$@" > "$FAKE_CLEANUP_ARGS"
		chmod 700 "$FAKE_LOCKED_WORK_DIR"
        ;;
    *)
        echo "unexpected fake runtime command: $*" >&2
        exit 33
        ;;
esac
`
	if err := os.WriteFile(runtimeCommand, []byte(runtimeScript), 0o755); err != nil {
		t.Fatal(err)
	}

	observer := &capturedManagementObserver{}
	supervisor := newTestSupervisor(t, dir, os.Environ())
	supervisor.runtime.Observer = observer
	ctx, cancel := context.WithCancel(context.Background())
	process, err := supervisor.startProcess(ctx, "openclaw-plaintiff", "docker", runtimeCommand, []string{"wait"}, "case-plaintiff", processStartOptions{
		env:        os.Environ(),
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
		afterExit: func() error {
			return restoreOpenClawWorkspaceOwnership(runtimeCommand, "openclaw-image", workDir, os.Geteuid(), os.Getegid())
		},
		participant: &runstate.ParticipantStart{
			Role:    "plaintiff",
			Profile: "openclaw-lawyer",
			Runner:  "openclaw",
			WorkDir: workDir,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	cancel()
	if err := supervisor.Stop(); err != nil {
		t.Fatal(err)
	}

	observer.mu.Lock()
	participantFinishes := append([]runstate.ParticipantFinish(nil), observer.participantFinishes...)
	processFinishes := append([]runstate.ProcessFinish(nil), observer.finishes...)
	observer.mu.Unlock()
	if len(participantFinishes) != 1 || participantFinishes[0].StateBytes != nil || participantFinishes[0].Error != "" {
		t.Fatalf("participant finishes = %#v", participantFinishes)
	}
	if len(processFinishes) != 1 || processFinishes[0].State != runstate.Canceled {
		t.Fatalf("process finishes = %#v", processFinishes)
	}
	if _, err := os.ReadFile(workFile); err != nil {
		t.Fatalf("read retained file %q: %v", workFile, err)
	}
	cleanupArgsRaw, err := os.ReadFile(cleanupArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	cleanupArgs := string(cleanupArgsRaw)
	for _, want := range []string{
		"--network\nnone\n",
		"--user\n0:0\n",
		"-v\n" + workDir + ":" + openClawCleanupWorkspace + "\n",
		"--entrypoint\n/usr/bin/chown\nopenclaw-image\n-R\n--\n" + fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()) + "\n" + openClawCleanupWorkspace + "\n",
	} {
		if !strings.Contains(cleanupArgs, want) {
			t.Fatalf("ownership cleanup arguments lack %q:\n%s", want, cleanupArgs)
		}
	}
}

func TestWebSearchObservationRecordsRunnerRoute(t *testing.T) {
	enabled := true
	tests := []struct {
		name      string
		profile   Profile
		mechanism string
		providers []string
	}{
		{name: "codex", profile: Profile{Runner: RunnerCodex}, mechanism: "native", providers: []string{"openai"}},
		{name: "claude", profile: Profile{Runner: RunnerClaude}, mechanism: "native", providers: []string{"anthropic"}},
		{name: "Claude OpenRouter", profile: Profile{Runner: RunnerClaude, Headless: headless.Profile{Provider: headless.ProviderOpenRouter}}, mechanism: "gateway", providers: []string{"openrouter"}},
		{name: "pi", profile: Profile{Runner: RunnerPi}, mechanism: "pi_web_access", providers: []string{"openai", "exa"}},
		{name: "OpenClaw Codex", profile: Profile{Runner: RunnerOpenClaw, OpenClaw: OpenClawProfile{Auth: OpenClawAuth{Mode: OpenClawAuthCodex}}}, mechanism: "native", providers: []string{"openai"}},
		{name: "OpenClaw API key", profile: Profile{Runner: RunnerOpenClaw, OpenClaw: OpenClawProfile{Auth: OpenClawAuth{Mode: OpenClawAuthAPIKey}}}, mechanism: "native", providers: []string{"openai"}},
		{name: "OpenClaw Anthropic", profile: Profile{Runner: RunnerOpenClaw, OpenClaw: OpenClawProfile{Provider: "anthropic", Auth: OpenClawAuth{Mode: OpenClawAuthAPIKey}}}, mechanism: "managed", providers: []string{"duckduckgo"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := webSearchObservation(test.profile, &enabled)
			if observation == nil || !observation.Enabled || observation.Mechanism != test.mechanism || !slices.Equal(observation.Providers, test.providers) {
				t.Fatalf("observation = %#v", observation)
			}
		})
	}

	disabled := false
	observation := webSearchObservation(Profile{Runner: RunnerCodex}, &disabled)
	if observation == nil || observation.Enabled || observation.Mechanism != "" || len(observation.Providers) != 0 {
		t.Fatalf("disabled observation = %#v", observation)
	}
	if webSearchObservation(Profile{Runner: RunnerCodex}, nil) != nil {
		t.Fatal("unconfigured assignment has a web search observation")
	}
}

func TestStopReturnsContainerAndCleanupErrors(t *testing.T) {
	dir := t.TempDir()
	runtimeCommand := fakeContainerRuntime(t, 7, "permission denied")
	supervisor := newTestSupervisor(t, dir, nil)
	afterExitErr := errors.New("retained ownership cleanup failed")
	cleanupErr := errors.New("cleanup failed")
	process, err := supervisor.startProcess(context.Background(), "pi-plaintiff", "podman", "sleep", []string{"60"}, "case-plaintiff", processStartOptions{
		roleID:     "plaintiff",
		verifyExit: func(context.Context, string, string) error { return nil },
		afterExit:  func() error { return afterExitErr },
		cleanup:    func() error { return cleanupErr },
	})
	if err != nil {
		t.Fatal(err)
	}
	process.containerStop.command = runtimeCommand
	supervisor.mu.Lock()
	supervisor.processes = append(supervisor.processes, process)
	supervisor.mu.Unlock()
	err = supervisor.Stop()
	if !errors.Is(err, afterExitErr) || !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("stop error = %v", err)
	}
}

func newTestSupervisor(t *testing.T, dir string, env []string) *Supervisor {
	t.Helper()
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	supervisor, err := New(Runtime{
		OutputDir:       dir,
		LogsDir:         logs,
		ContainerPrefix: "aar",
		PodmanCommand:   "podman",
		PiImage:         "pi-image",
		BaseEnvironment: env,
	})
	if err != nil {
		t.Fatal(err)
	}
	return supervisor
}

func testAssignment(role string, verify func(context.Context, string, string) error) Assignment {
	return Assignment{
		CaseID: "case-1",
		RoleID: role,
		Prompt: "complete the assigned lawyer obligation",
		MCP: headless.MCPServer{
			Name: "aar-case-1-" + role,
			URL:  "http://127.0.0.1:19000/mcp",
		},
		VerifyExit: verify,
	}
}

func writeCodexCredentials(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-auth.json")
	if err := os.WriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePiCodexCredentials(t *testing.T, dir string) string {
	t.Helper()
	payload, err := json.Marshal(struct {
		Expires int64 `json:"exp"`
	}{Expires: 4_102_444_800})
	if err != nil {
		t.Fatal(err)
	}
	accessToken := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	document := struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
	}{AuthMode: "chatgpt"}
	document.Tokens.AccessToken = accessToken
	document.Tokens.RefreshToken = "test-refresh"
	document.Tokens.AccountID = "test-account"
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pi-codex-auth.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeCodexCommand(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	script := "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":100,\"cached_input_tokens\":40,\"output_tokens\":20}}'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForProcesses(t *testing.T, supervisor *Supervisor) {
	t.Helper()
	supervisor.mu.Lock()
	processes := append([]*processRecord(nil), supervisor.processes...)
	supervisor.mu.Unlock()
	for _, process := range processes {
		select {
		case <-process.finished:
		case <-time.After(2 * time.Second):
			t.Fatalf("process %s did not exit", process.name)
		}
	}
}

func fakeContainerRuntime(t *testing.T, exitCode int, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "container-runtime")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q >&2\nexit %d\n", output, exitCode)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
