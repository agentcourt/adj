package adjudicate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/runtime/runstate"
)

type runRecorder struct {
	mu        sync.Mutex
	path      string
	result    Result
	writeJSON func(string, any) error
}

func newRunRecorder(path string, initial Result, writeJSON func(string, any) error) (*runRecorder, error) {
	if writeJSON == nil {
		writeJSON = WriteJSONAtomic
	}
	recorder := &runRecorder{path: path, result: cloneResult(initial), writeJSON: writeJSON}
	if err := recorder.writeJSON(path, initial); err != nil {
		return nil, err
	}
	return recorder, nil
}

func (r *runRecorder) update(change func(*Result)) error {
	if r == nil {
		return fmt.Errorf("run recorder is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneResult(r.result)
	change(&next)
	if err := r.writeJSON(r.path, next); err != nil {
		return err
	}
	r.result = next
	return nil
}

func (r *runRecorder) snapshot() Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneResult(r.result)
}

func (r *runRecorder) StartProcess(start runstate.ProcessStart) (runstate.FinishProcess, error) {
	if strings.TrimSpace(start.Name) == "" {
		return nil, fmt.Errorf("managed process name is empty")
	}
	if strings.TrimSpace(start.Kind) == "" {
		return nil, fmt.Errorf("managed process %q kind is empty", start.Name)
	}
	if start.PID <= 0 {
		return nil, fmt.Errorf("managed process %q PID must be positive", start.Name)
	}
	if start.StartedAt.IsZero() {
		return nil, fmt.Errorf("managed process %q start time is empty", start.Name)
	}
	index := -1
	if err := r.update(func(result *Result) {
		index = len(result.Management.Processes)
		result.Management.Processes = append(result.Management.Processes, ManagedProcess{
			Name:       start.Name,
			Kind:       start.Kind,
			Role:       start.Role,
			PID:        start.PID,
			InstanceID: start.InstanceID,
			State:      ProcessRunning,
			StartedAt:  start.StartedAt.UTC(),
		})
		advanceManagementTime(&result.Management, start.StartedAt)
	}); err != nil {
		return nil, err
	}
	handle := &processHandle{recorder: r, index: index, name: start.Name}
	return handle.finish, nil
}

func (r *runRecorder) StartParticipant(start runstate.ParticipantStart) (runstate.FinishParticipant, error) {
	role := strings.TrimSpace(start.Role)
	profile := strings.TrimSpace(start.Profile)
	runner := AgentRunner(strings.TrimSpace(start.Runner))
	if role == "" {
		return nil, fmt.Errorf("managed participant role is empty")
	}
	if profile == "" {
		return nil, fmt.Errorf("managed participant %q profile is empty", role)
	}
	if runner != RunnerOpenClaw && runner != RunnerPi && runner != RunnerCodex && runner != RunnerClaude {
		return nil, fmt.Errorf("managed participant %q runner %q is invalid", role, runner)
	}
	reasoningEffort := strings.TrimSpace(start.ReasoningEffort)
	if reasoningEffort != "" && reasoningEffort != "low" && reasoningEffort != "medium" && reasoningEffort != "high" && reasoningEffort != "xhigh" {
		return nil, fmt.Errorf("managed participant %q reasoning effort %q is invalid", role, reasoningEffort)
	}
	if start.StartedAt.IsZero() {
		return nil, fmt.Errorf("managed participant %q start time is empty", role)
	}
	var webSearch *ParticipantWebSearch
	if start.WebSearch != nil {
		mechanism := strings.TrimSpace(start.WebSearch.Mechanism)
		providers := make([]string, len(start.WebSearch.Providers))
		for index, provider := range start.WebSearch.Providers {
			providers[index] = strings.TrimSpace(provider)
			if providers[index] == "" {
				return nil, fmt.Errorf("managed participant %q web search provider %d is empty", role, index)
			}
		}
		if start.WebSearch.Enabled && (mechanism == "" || len(providers) == 0) {
			return nil, fmt.Errorf("managed participant %q enabled web search requires a mechanism and providers", role)
		}
		if !start.WebSearch.Enabled && (mechanism != "" || len(providers) != 0) {
			return nil, fmt.Errorf("managed participant %q disabled web search must not name a mechanism or providers", role)
		}
		webSearch = &ParticipantWebSearch{
			Enabled:   start.WebSearch.Enabled,
			Mechanism: mechanism,
			Providers: providers,
		}
	}
	index := -1
	if err := r.update(func(result *Result) {
		index = len(result.Management.Participants)
		result.Management.Participants = append(result.Management.Participants, ManagedParticipant{
			Role:            role,
			Profile:         profile,
			Runner:          runner,
			ReasoningEffort: reasoningEffort,
			Resumed:         start.Resumed,
			StateDir:        strings.TrimSpace(start.StateDir),
			WorkDir:         strings.TrimSpace(start.WorkDir),
			WebSearch:       webSearch,
		})
		advanceManagementTime(&result.Management, start.StartedAt)
	}); err != nil {
		return nil, err
	}
	handle := &participantHandle{recorder: r, index: index, role: role}
	return handle.finish, nil
}

type processHandle struct {
	mu       sync.Mutex
	recorder *runRecorder
	index    int
	name     string
	finished bool
}

type participantHandle struct {
	mu       sync.Mutex
	recorder *runRecorder
	index    int
	role     string
	finished bool
}

func (h *participantHandle) finish(finish runstate.ParticipantFinish) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.finished {
		return fmt.Errorf("managed participant %q is already finished", h.role)
	}
	if finish.FinishedAt.IsZero() {
		return fmt.Errorf("managed participant %q finish time is empty", h.role)
	}
	if finish.StateBytes != nil && *finish.StateBytes < 0 {
		return fmt.Errorf("managed participant %q state size is negative", h.role)
	}
	if finish.Usage != nil {
		usage := finish.Usage
		if usage.InputTokens < 0 || usage.CachedInputTokens < 0 || usage.CacheWriteInputTokens < 0 || usage.OutputTokens < 0 || usage.ReasoningTokens < 0 || usage.TotalTokens < 0 {
			return fmt.Errorf("managed participant %q usage contains a negative value", h.role)
		}
	}
	if finish.Refusal != nil {
		if err := validateParticipantRefusal(*finish.Refusal); err != nil {
			return fmt.Errorf("managed participant %q refusal: %w", h.role, err)
		}
	}
	err := h.recorder.finishParticipant(h.index, h.role, finish)
	if err == nil {
		h.finished = true
	}
	return err
}

func (r *runRecorder) finishParticipant(index int, role string, finish runstate.ParticipantFinish) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneResult(r.result)
	if index < 0 || index >= len(next.Management.Participants) || next.Management.Participants[index].Role != role {
		return fmt.Errorf("managed participant %q record is absent", role)
	}
	participant := &next.Management.Participants[index]
	if participant.StateBytes != nil || participant.Usage != nil || participant.Refusal != nil || participant.Error != "" {
		return fmt.Errorf("managed participant %q record is already finished", role)
	}
	if finish.StateBytes != nil {
		stateBytes := *finish.StateBytes
		participant.StateBytes = &stateBytes
	}
	if finish.Usage != nil {
		usage := *finish.Usage
		participant.Usage = &usage
	}
	if finish.Refusal != nil {
		participant.Refusal = participantRefusal(*finish.Refusal)
	}
	participant.Error = finish.Error
	if finish.Error != "" {
		next.Management.Errors = append(next.Management.Errors, ManagementError{
			Operation: "participant_observation",
			Process:   role,
			Error:     finish.Error,
		})
	}
	advanceManagementTime(&next.Management, finish.FinishedAt)
	if err := r.writeJSON(r.path, next); err != nil {
		return err
	}
	r.result = next
	return nil
}

func validateParticipantRefusal(refusal runstate.ParticipantRefusal) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "provider", value: refusal.Provider},
		{name: "model", value: refusal.Model},
		{name: "stdout artifact", value: refusal.StdoutArtifact},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is empty", field.name)
		}
	}
	if strings.TrimSpace(refusal.StopReason) == "" && strings.TrimSpace(refusal.RawStopReason) == "" {
		return errors.New("stop reason and raw stop reason are empty")
	}
	artifact := filepath.Clean(strings.TrimSpace(refusal.StdoutArtifact))
	if filepath.IsAbs(artifact) || artifact == "." || artifact == ".." || strings.HasPrefix(artifact, ".."+string(filepath.Separator)) {
		return fmt.Errorf("stdout artifact %q is not relative to the run record", refusal.StdoutArtifact)
	}
	if refusal.SessionPath != "" && !filepath.IsAbs(strings.TrimSpace(refusal.SessionPath)) {
		return fmt.Errorf("session path %q is not absolute", refusal.SessionPath)
	}
	return nil
}

func participantRefusal(refusal runstate.ParticipantRefusal) *ParticipantRefusal {
	return &ParticipantRefusal{
		Provider:         strings.TrimSpace(refusal.Provider),
		Model:            strings.TrimSpace(refusal.Model),
		StopReason:       strings.TrimSpace(refusal.StopReason),
		RawStopReason:    strings.TrimSpace(refusal.RawStopReason),
		WillRetry:        refusal.WillRetry,
		Category:         strings.TrimSpace(refusal.Category),
		Explanation:      strings.TrimSpace(refusal.Explanation),
		SessionID:        strings.TrimSpace(refusal.SessionID),
		RequestID:        strings.TrimSpace(refusal.RequestID),
		ResponseID:       strings.TrimSpace(refusal.ResponseID),
		RefusedMessageID: strings.TrimSpace(refusal.RefusedMessageID),
		StdoutArtifact:   filepath.ToSlash(filepath.Clean(strings.TrimSpace(refusal.StdoutArtifact))),
		SessionPath:      strings.TrimSpace(refusal.SessionPath),
	}
}

func (h *processHandle) finish(finish runstate.ProcessFinish) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.finished {
		return fmt.Errorf("managed process %q is already finished", h.name)
	}
	if finish.State != runstate.Completed && finish.State != runstate.Failed && finish.State != runstate.Canceled {
		return fmt.Errorf("managed process %q has invalid terminal state %q", h.name, finish.State)
	}
	if finish.FinishedAt.IsZero() {
		return fmt.Errorf("managed process %q finish time is empty", h.name)
	}
	err := h.recorder.finishProcess(h.index, h.name, finish)
	if err == nil {
		h.finished = true
	}
	return err
}

func (r *runRecorder) finishProcess(index int, name string, finish runstate.ProcessFinish) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneResult(r.result)
	if index < 0 || index >= len(next.Management.Processes) || next.Management.Processes[index].Name != name {
		return fmt.Errorf("managed process %q record is absent", name)
	}
	process := &next.Management.Processes[index]
	if process.State != ProcessRunning {
		return fmt.Errorf("managed process %q record has state %q", name, process.State)
	}
	finished := finish.FinishedAt.UTC()
	process.State = finish.State
	process.FinishedAt = &finished
	process.Error = finish.Error
	if finish.ExitCode != nil {
		exitCode := *finish.ExitCode
		process.ExitCode = &exitCode
	}
	advanceManagementTime(&next.Management, finish.FinishedAt)
	if err := r.writeJSON(r.path, next); err != nil {
		return err
	}
	r.result = next
	return nil
}

func advanceManagementTime(management *Management, value time.Time) {
	value = value.UTC()
	if value.After(management.UpdatedAt) {
		management.UpdatedAt = value
	}
}

func cloneResult(result Result) Result {
	result.ProcedureResult = cloneJSON(result.ProcedureResult)
	result.Failure = cloneJSON(result.Failure)
	result.Management = cloneManagement(result.Management)
	return result
}

func cloneManagement(management Management) Management {
	management.Processes = append([]ManagedProcess(nil), management.Processes...)
	for index := range management.Processes {
		if management.Processes[index].FinishedAt != nil {
			finished := *management.Processes[index].FinishedAt
			management.Processes[index].FinishedAt = &finished
		}
		if management.Processes[index].ExitCode != nil {
			exitCode := *management.Processes[index].ExitCode
			management.Processes[index].ExitCode = &exitCode
		}
	}
	management.Participants = append([]ManagedParticipant(nil), management.Participants...)
	for index := range management.Participants {
		if management.Participants[index].WebSearch != nil {
			webSearch := *management.Participants[index].WebSearch
			webSearch.Providers = append([]string(nil), webSearch.Providers...)
			management.Participants[index].WebSearch = &webSearch
		}
		if management.Participants[index].StateBytes != nil {
			stateBytes := *management.Participants[index].StateBytes
			management.Participants[index].StateBytes = &stateBytes
		}
		if management.Participants[index].Usage != nil {
			usage := *management.Participants[index].Usage
			management.Participants[index].Usage = &usage
		}
		if management.Participants[index].Refusal != nil {
			refusal := *management.Participants[index].Refusal
			management.Participants[index].Refusal = &refusal
		}
	}
	management.Errors = append([]ManagementError(nil), management.Errors...)
	if management.Provider.Usage != nil {
		usage := *management.Provider.Usage
		management.Provider.Usage = &usage
	}
	if management.Provider.CostUSD != nil {
		cost := *management.Provider.CostUSD
		management.Provider.CostUSD = &cost
	}
	return management
}
