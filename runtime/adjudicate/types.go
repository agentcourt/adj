package adjudicate

import (
	"encoding/json"
	"time"

	"github.com/agentcourt/adj/runtime/runstate"
)

const (
	RequestSchemaVersion  = "adjudicate.request.v1"
	SettingsSchemaVersion = "adjudicate.settings.v1"
	ResultSchemaVersion   = "adjudicate.result.v1"
)

type Procedure string

const (
	ProcedureARB    Procedure = "arb"
	ProcedureARBD   Procedure = "arbd"
	ProcedureADC    Procedure = "adc"
	ProcedureSimple Procedure = "simple"
	ProcedureQuick  Procedure = "quick"
)

func (p Procedure) Valid() bool {
	switch p {
	case ProcedureARB, ProcedureARBD, ProcedureADC, ProcedureSimple, ProcedureQuick:
		return true
	default:
		return false
	}
}

type Request struct {
	SchemaVersion string        `json:"schema_version"`
	Procedure     Procedure     `json:"procedure"`
	CaseID        string        `json:"case_id"`
	RunID         string        `json:"run_id"`
	Proposition   string        `json:"proposition"`
	Documents     DocumentInput `json:"documents"`
	SettingsFile  string        `json:"settings_file"`
	OutDir        string        `json:"out_dir,omitempty"`
}

type DocumentInput struct {
	Root string `json:"root,omitempty"`
}

type Decision struct {
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	Rationale string `json:"rationale,omitempty"`
}

type Capabilities struct {
	Documents      bool `json:"documents"`
	ParticipantAPI bool `json:"participant_api"`
	LeanReplay     bool `json:"lean_replay"`
	Sessions       bool `json:"sessions"`
	Council        bool `json:"council"`
}

type Status string

const (
	StatusRunning Status = "running"
	StatusOK      Status = "ok"
	StatusFailed  Status = "failed"
	StatusError   Status = "error"
)

type ProcessState = runstate.State

const (
	ProcessRunning   = runstate.Running
	ProcessCompleted = runstate.Completed
	ProcessFailed    = runstate.Failed
	ProcessCanceled  = runstate.Canceled
)

type ManagedProcess struct {
	Name       string       `json:"name"`
	Kind       string       `json:"kind"`
	Role       string       `json:"role,omitempty"`
	PID        int          `json:"pid"`
	InstanceID string       `json:"instance_id,omitempty"`
	State      ProcessState `json:"state"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	ExitCode   *int         `json:"exit_code,omitempty"`
	Error      string       `json:"error,omitempty"`
}

type TokenUsage = runstate.TokenUsage

type ProviderManagement = runstate.ProviderAccounting

type ManagedParticipant struct {
	Role            string                `json:"role"`
	Profile         string                `json:"profile"`
	Runner          AgentRunner           `json:"runner"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
	Resumed         bool                  `json:"resumed"`
	StateDir        string                `json:"state_dir"`
	WorkDir         string                `json:"work_dir,omitempty"`
	WebSearch       *ParticipantWebSearch `json:"web_search,omitempty"`
	StateBytes      *int64                `json:"state_bytes,omitempty"`
	Usage           *TokenUsage           `json:"usage,omitempty"`
	Refusal         *ParticipantRefusal   `json:"refusal,omitempty"`
	Error           string                `json:"error,omitempty"`
}

type ParticipantRefusal struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	StopReason       string `json:"stop_reason"`
	RawStopReason    string `json:"raw_stop_reason"`
	WillRetry        bool   `json:"will_retry"`
	Category         string `json:"category,omitempty"`
	Explanation      string `json:"explanation,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
	ResponseID       string `json:"response_id,omitempty"`
	RefusedMessageID string `json:"refused_message_id,omitempty"`
	StdoutArtifact   string `json:"stdout_artifact"`
	SessionPath      string `json:"session_path,omitempty"`
}

type ParticipantWebSearch struct {
	Enabled   bool     `json:"enabled"`
	Mechanism string   `json:"mechanism,omitempty"`
	Providers []string `json:"providers,omitempty"`
}

type ManagementError struct {
	Operation string `json:"operation"`
	Process   string `json:"process,omitempty"`
	Error     string `json:"error"`
}

type Management struct {
	UpdatedAt    time.Time            `json:"updated_at"`
	Processes    []ManagedProcess     `json:"processes,omitempty"`
	Participants []ManagedParticipant `json:"participants,omitempty"`
	Provider     ProviderManagement   `json:"provider"`
	Errors       []ManagementError    `json:"errors,omitempty"`
}

type Result struct {
	SchemaVersion   string          `json:"schema_version"`
	Procedure       Procedure       `json:"procedure"`
	CaseID          string          `json:"case_id"`
	RunID           string          `json:"run_id"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	Status          Status          `json:"status"`
	Phase           string          `json:"phase"`
	Decision        *Decision       `json:"decision,omitempty"`
	Capabilities    Capabilities    `json:"capabilities"`
	ProcedureResult json.RawMessage `json:"procedure_result,omitempty"`
	Failure         json.RawMessage `json:"failure,omitempty"`
	ErrorClass      string          `json:"error_class,omitempty"`
	Error           string          `json:"error,omitempty"`
	RecordDir       string          `json:"record_dir"`
	Management      Management      `json:"management"`
}

type ProcedureOutcome struct {
	Status          Status
	Phase           string
	Decision        *Decision
	ProcedureResult json.RawMessage
	Failure         json.RawMessage
	Provider        ProviderManagement
}

type ProcedureRequest struct {
	Request    Request
	Settings   ResolvedSettings
	Documents  DocumentManifest
	RecordDir  string
	CoreDir    string
	LogsDir    string
	SessionDir string
	Observer   runstate.Observer
}
