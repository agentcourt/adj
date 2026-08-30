package adjudicate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentcourt/adj/runtime/runstate"
)

func TestRunRecorderRecordsProcessLifecycle(t *testing.T) {
	dir := t.TempDir()
	started := time.Date(2026, 8, 18, 14, 0, 0, 0, time.UTC)
	recorder, err := newRunRecorder(dir+"/run.json", Result{
		SchemaVersion: ResultSchemaVersion,
		Management:    Management{UpdatedAt: started},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := recorder.StartProcess(runstate.ProcessStart{
		Name:      "simple-core",
		Kind:      "core",
		PID:       123,
		StartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	live := recorder.snapshot()
	if len(live.Management.Processes) != 1 || live.Management.Processes[0].State != ProcessRunning {
		t.Fatalf("live management = %#v", live.Management)
	}
	finished := started.Add(time.Second)
	exitCode := 0
	if err := finish(runstate.ProcessFinish{
		State:      runstate.Completed,
		FinishedAt: finished,
		ExitCode:   &exitCode,
	}); err != nil {
		t.Fatal(err)
	}
	terminal := recorder.snapshot()
	process := terminal.Management.Processes[0]
	if process.State != ProcessCompleted || process.FinishedAt == nil || !process.FinishedAt.Equal(finished) || process.ExitCode == nil || *process.ExitCode != 0 {
		t.Fatalf("terminal process = %#v", process)
	}
	if err := finish(runstate.ProcessFinish{State: runstate.Completed, FinishedAt: finished}); err == nil || !strings.Contains(err.Error(), "already finished") {
		t.Fatalf("second finish error = %v", err)
	}
}

func TestRunRecorderSerializesConcurrentProcesses(t *testing.T) {
	recorder, err := newRunRecorder(t.TempDir()+"/run.json", Result{Management: Management{UpdatedAt: time.Now().UTC()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	const count = 8
	errs := make(chan error, count)
	var group sync.WaitGroup
	for index := range count {
		group.Add(1)
		go func() {
			defer group.Done()
			name := "process-" + string(rune('a'+index))
			finish, err := recorder.StartProcess(runstate.ProcessStart{Name: name, Kind: "test", PID: 100 + index, StartedAt: time.Now().UTC()})
			if err == nil {
				err = finish(runstate.ProcessFinish{State: runstate.Completed, FinishedAt: time.Now().UTC()})
			}
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	result := recorder.snapshot()
	if len(result.Management.Processes) != count {
		t.Fatalf("process count = %d, want %d", len(result.Management.Processes), count)
	}
	for _, process := range result.Management.Processes {
		if process.State != ProcessCompleted {
			t.Fatalf("process = %#v", process)
		}
	}
}

func TestRunRecorderRejectsInvalidProcess(t *testing.T) {
	recorder, err := newRunRecorder(t.TempDir()+"/run.json", Result{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recorder.StartProcess(runstate.ProcessStart{Kind: "core", PID: 1, StartedAt: time.Now()}); err == nil || !strings.Contains(err.Error(), "name is empty") {
		t.Fatalf("invalid process error = %v", err)
	}
}

func TestRunRecorderRecordsParticipant(t *testing.T) {
	started := time.Date(2026, 8, 18, 15, 0, 0, 0, time.UTC)
	recorder, err := newRunRecorder(t.TempDir()+"/run.json", Result{Management: Management{UpdatedAt: started}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := recorder.StartParticipant(runstate.ParticipantStart{
		Role:            "plaintiff",
		Profile:         "usual-lawyer",
		Runner:          "codex",
		ReasoningEffort: "xhigh",
		Resumed:         true,
		StateDir:        "/state/plaintiff/codex",
		WorkDir:         "/work/plaintiff",
		WebSearch:       &runstate.WebSearch{Enabled: true, Mechanism: "native", Providers: []string{"openai"}},
		StartedAt:       started.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	live := recorder.snapshot()
	if len(live.Management.Participants) != 1 || !live.Management.Participants[0].Resumed {
		t.Fatalf("live management = %#v", live.Management)
	}
	stateBytes := int64(2048)
	usage := runstate.TokenUsage{InputTokens: 100, CachedInputTokens: 40, OutputTokens: 10, TotalTokens: 110}
	finished := started.Add(2 * time.Second)
	if err := finish(runstate.ParticipantFinish{FinishedAt: finished, StateBytes: &stateBytes, Usage: &usage}); err != nil {
		t.Fatal(err)
	}
	participant := recorder.snapshot().Management.Participants[0]
	if participant.Role != "plaintiff" || participant.Profile != "usual-lawyer" || participant.Runner != RunnerCodex || participant.ReasoningEffort != "xhigh" || participant.StateDir != "/state/plaintiff/codex" || participant.WorkDir != "/work/plaintiff" || participant.WebSearch == nil || !participant.WebSearch.Enabled || participant.WebSearch.Mechanism != "native" || !slices.Equal(participant.WebSearch.Providers, []string{"openai"}) || participant.StateBytes == nil || *participant.StateBytes != stateBytes || participant.Usage == nil || participant.Usage.TotalTokens != 110 {
		t.Fatalf("participant = %#v", participant)
	}
	if err := finish(runstate.ParticipantFinish{FinishedAt: finished}); err == nil || !strings.Contains(err.Error(), "already finished") {
		t.Fatalf("second finish error = %v", err)
	}
}

func TestRunRecorderRecordsParticipantObservationError(t *testing.T) {
	started := time.Now().UTC()
	recorder, err := newRunRecorder(t.TempDir()+"/run.json", Result{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := recorder.StartParticipant(runstate.ParticipantStart{Role: "defendant", Profile: "claude-lawyer", Runner: "claude", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	if err := finish(runstate.ParticipantFinish{FinishedAt: started.Add(time.Second), Error: "parse usage: malformed JSON"}); err != nil {
		t.Fatal(err)
	}
	management := recorder.snapshot().Management
	if len(management.Errors) != 1 || management.Errors[0].Operation != "participant_observation" || management.Errors[0].Process != "defendant" || management.Participants[0].Error == "" {
		t.Fatalf("management = %#v", management)
	}
}

func TestRunRecorderRecordsParticipantRefusal(t *testing.T) {
	started := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "run.json")
	recorder, err := newRunRecorder(path, Result{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := recorder.StartParticipant(runstate.ParticipantStart{Role: "defendant", Profile: "pi-opus", Runner: "pi", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	usage := runstate.TokenUsage{InputTokens: 20, OutputTokens: 2, TotalTokens: 22}
	refusal := runstate.ParticipantRefusal{
		Provider:         "anthropic",
		Model:            "claude-opus-4-8",
		StopReason:       "error",
		RawStopReason:    "refusal",
		WillRetry:        false,
		Category:         " cyber ",
		Explanation:      "provider explanation",
		SessionID:        "session-1",
		RequestID:        " request-1 ",
		ResponseID:       "response-1",
		RefusedMessageID: " message-1 ",
		StdoutArtifact:   "logs/pi-defendant.stdout",
		SessionPath:      "/state/pi/sessions/session-1.jsonl",
	}
	if err := finish(runstate.ParticipantFinish{FinishedAt: started.Add(time.Second), Usage: &usage, Refusal: &refusal}); err != nil {
		t.Fatal(err)
	}
	refusal.Provider = "changed"
	first := recorder.snapshot()
	participant := first.Management.Participants[0]
	if participant.Refusal == nil || participant.Refusal.Provider != "anthropic" || participant.Refusal.Model != "claude-opus-4-8" || participant.Refusal.RawStopReason != "refusal" || participant.Refusal.WillRetry || participant.Refusal.Category != "cyber" || participant.Refusal.SessionID != "session-1" || participant.Refusal.RequestID != "request-1" || participant.Refusal.ResponseID != "response-1" || participant.Refusal.RefusedMessageID != "message-1" || participant.Refusal.StdoutArtifact != "logs/pi-defendant.stdout" || participant.Refusal.SessionPath != "/state/pi/sessions/session-1.jsonl" {
		t.Fatalf("participant = %#v", participant)
	}
	if participant.Usage == nil || participant.Usage.TotalTokens != 22 || participant.Error != "" || len(first.Management.Errors) != 0 {
		t.Fatalf("management = %#v", first.Management)
	}
	participant.Refusal.RequestID = "mutated snapshot"
	if got := recorder.snapshot().Management.Participants[0].Refusal.RequestID; got != "request-1" {
		t.Fatalf("cloned refusal request ID = %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var recorded Result
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded.Management.Participants[0].Refusal == nil || recorded.Management.Participants[0].Refusal.Category != "cyber" || recorded.Management.Participants[0].Refusal.RequestID != "request-1" || recorded.Management.Participants[0].Refusal.ResponseID != "response-1" || recorded.Management.Participants[0].Refusal.RefusedMessageID != "message-1" {
		t.Fatalf("recorded result = %#v", recorded)
	}
	if err := finish(runstate.ParticipantFinish{FinishedAt: started.Add(2 * time.Second)}); err == nil || !strings.Contains(err.Error(), "already finished") {
		t.Fatalf("second finish error = %v", err)
	}
}

func TestParticipantRefusalOmitsUnavailableProviderIdentifiers(t *testing.T) {
	refusal := participantRefusal(runstate.ParticipantRefusal{
		Provider:       "anthropic",
		Model:          "claude-opus-4-8",
		StopReason:     "error",
		RawStopReason:  "refusal",
		StdoutArtifact: "logs/claude-plaintiff.stdout",
	})
	data, err := json.Marshal(refusal)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"category"`, `"request_id"`, `"response_id"`, `"refused_message_id"`} {
		if strings.Contains(string(data), field) {
			t.Fatalf("optional field %s appears in %s", field, data)
		}
	}
}

func TestRunRecorderAcceptsClaudeRefusalWithoutRawStopReason(t *testing.T) {
	started := time.Now().UTC()
	recorder, err := newRunRecorder(filepath.Join(t.TempDir(), "run.json"), Result{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := recorder.StartParticipant(runstate.ParticipantStart{Role: "plaintiff", Profile: "claude-opus", Runner: "claude", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	refusal := runstate.ParticipantRefusal{
		Provider:       "anthropic",
		Model:          "claude-opus-4-8",
		StopReason:     "refusal",
		StdoutArtifact: "logs/claude-plaintiff.stdout",
	}
	if err := finish(runstate.ParticipantFinish{FinishedAt: started.Add(time.Second), Refusal: &refusal}); err != nil {
		t.Fatal(err)
	}
	got := recorder.snapshot().Management.Participants[0].Refusal
	if got == nil || got.StopReason != "refusal" || got.RawStopReason != "" {
		t.Fatalf("refusal = %#v", got)
	}
}

func TestRunRecorderRejectsInvalidParticipantRefusal(t *testing.T) {
	valid := runstate.ParticipantRefusal{
		Provider:       "anthropic",
		Model:          "claude-opus-4-8",
		StopReason:     "error",
		RawStopReason:  "refusal",
		StdoutArtifact: "logs/pi-defendant.stdout",
		SessionPath:    "/state/session.jsonl",
	}
	tests := []struct {
		name   string
		change func(*runstate.ParticipantRefusal)
	}{
		{name: "provider", change: func(value *runstate.ParticipantRefusal) { value.Provider = "" }},
		{name: "stop reasons", change: func(value *runstate.ParticipantRefusal) { value.StopReason, value.RawStopReason = "", "" }},
		{name: "stdout artifact", change: func(value *runstate.ParticipantRefusal) { value.StdoutArtifact = "../outside.stdout" }},
		{name: "session path", change: func(value *runstate.ParticipantRefusal) { value.SessionPath = "sessions/session.jsonl" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			refusal := valid
			test.change(&refusal)
			started := time.Now().UTC()
			recorder, err := newRunRecorder(filepath.Join(t.TempDir(), "run.json"), Result{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			finish, err := recorder.StartParticipant(runstate.ParticipantStart{Role: "defendant", Profile: "pi-opus", Runner: "pi", StartedAt: started})
			if err != nil {
				t.Fatal(err)
			}
			if err := finish(runstate.ParticipantFinish{FinishedAt: started.Add(time.Second), Refusal: &refusal}); err == nil {
				t.Fatalf("invalid refusal was accepted: %#v", refusal)
			}
		})
	}
}

func TestRunRecorderRejectsInvalidParticipant(t *testing.T) {
	recorder, err := newRunRecorder(t.TempDir()+"/run.json", Result{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, start := range []runstate.ParticipantStart{
		{Profile: "lawyer", Runner: "codex", StartedAt: time.Now()},
		{Role: "plaintiff", Runner: "codex", StartedAt: time.Now()},
		{Role: "plaintiff", Profile: "lawyer", Runner: "unknown", StartedAt: time.Now()},
		{Role: "plaintiff", Profile: "lawyer", Runner: "codex", ReasoningEffort: "maximum", StartedAt: time.Now()},
		{Role: "plaintiff", Profile: "lawyer", Runner: "codex"},
	} {
		if _, err := recorder.StartParticipant(start); err == nil {
			t.Fatalf("invalid participant was accepted: %#v", start)
		}
	}
}

func TestTerminalPublicationPreservesCurrentProcessState(t *testing.T) {
	started := time.Date(2026, 8, 18, 14, 0, 0, 0, time.UTC)
	recorder, err := newRunRecorder(t.TempDir()+"/run.json", Result{
		Status: StatusRunning,
		Management: Management{UpdatedAt: started, Processes: []ManagedProcess{{
			Name:      "adjudicate",
			Kind:      "controller",
			PID:       1,
			State:     ProcessRunning,
			StartedAt: started,
		}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := recorder.StartProcess(runstate.ProcessStart{Name: "simple-core", Kind: "core", PID: 2, StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	stale := recorder.snapshot()
	finished := started.Add(time.Second)
	if err := finish(runstate.ProcessFinish{State: runstate.Completed, FinishedAt: finished}); err != nil {
		t.Fatal(err)
	}
	terminal := Result{Status: StatusOK, FinishedAt: &finished}
	if err := recorder.update(func(current *Result) {
		management := current.Management
		*current = terminal
		current.Management = management
		finishController(&current.Management, finished, ProcessCompleted, "")
	}); err != nil {
		t.Fatal(err)
	}
	result := recorder.snapshot()
	if result.Status != StatusOK || len(result.Management.Processes) != 2 || result.Management.Processes[1].State != ProcessCompleted {
		t.Fatalf("terminal result = %#v; stale snapshot = %#v", result, stale)
	}
}
