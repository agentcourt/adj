package adjudicate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/runtime/runstate"
)

func TestCoreProcessRecordsLifecycle(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.Mkdir(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	recorder, err := newRunRecorder(filepath.Join(dir, "common-run.json"), Result{Management: Management{UpdatedAt: time.Now().UTC()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "core-run.json")
	raw, err := runCoreProcess(context.Background(), coreProcessRequest{
		Command:    os.Args[0],
		Args:       []string{"-test.run=^TestCoreProcessHelper$", "--", resultPath},
		Env:        append(os.Environ(), "ADJ_CORE_PROCESS_HELPER=1"),
		LogsDir:    logsDir,
		LogPrefix:  "simple",
		ResultPath: resultPath,
		Observer:   recorder,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"status":"ok"}` {
		t.Fatalf("result = %q", raw)
	}
	management := recorder.snapshot().Management
	if len(management.Processes) != 1 {
		t.Fatalf("processes = %#v", management.Processes)
	}
	process := management.Processes[0]
	if process.Name != "simple-core" || process.Kind != "core" || process.State != ProcessCompleted || process.ExitCode == nil || *process.ExitCode != 0 {
		t.Fatalf("process = %#v", process)
	}
}

func TestCoreProcessReturnsObserverStartError(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.Mkdir(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := errors.New("record failed")
	_, err := runCoreProcess(context.Background(), coreProcessRequest{
		Command:    os.Args[0],
		Args:       []string{"-test.run=^TestCoreProcessHelper$", "--", filepath.Join(dir, "core-run.json")},
		Env:        append(os.Environ(), "ADJ_CORE_PROCESS_HELPER=1"),
		LogsDir:    logsDir,
		LogPrefix:  "simple",
		ResultPath: filepath.Join(dir, "core-run.json"),
		Observer:   processObserverFunc(func(runstate.ProcessStart) (runstate.FinishProcess, error) { return nil, want }),
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "record core process start") {
		t.Fatalf("error = %v", err)
	}
}

func TestCoreProcessReturnsObserverFinishError(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.Mkdir(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := errors.New("finish record failed")
	_, err := runCoreProcess(context.Background(), coreProcessRequest{
		Command:    os.Args[0],
		Args:       []string{"-test.run=^TestCoreProcessHelper$", "--", filepath.Join(dir, "core-run.json")},
		Env:        append(os.Environ(), "ADJ_CORE_PROCESS_HELPER=1"),
		LogsDir:    logsDir,
		LogPrefix:  "simple",
		ResultPath: filepath.Join(dir, "core-run.json"),
		Observer: processObserverFunc(func(runstate.ProcessStart) (runstate.FinishProcess, error) {
			return func(runstate.ProcessFinish) error { return want }, nil
		}),
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestCoreProcessHelper(t *testing.T) {
	if os.Getenv("ADJ_CORE_PROCESS_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	if err := os.WriteFile(path, []byte(`{"status":"ok"}`), 0o644); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

type processObserverFunc func(runstate.ProcessStart) (runstate.FinishProcess, error)

func (f processObserverFunc) StartProcess(start runstate.ProcessStart) (runstate.FinishProcess, error) {
	return f(start)
}
