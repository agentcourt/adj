package runstate

import "time"

type State string

const (
	Running   State = "running"
	Completed State = "completed"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

type ProcessStart struct {
	Name       string
	Kind       string
	Role       string
	PID        int
	InstanceID string
	StartedAt  time.Time
}

type ProcessFinish struct {
	State      State
	FinishedAt time.Time
	ExitCode   *int
	Error      string
}

type FinishProcess func(ProcessFinish) error

type ParticipantStart struct {
	Role            string
	Profile         string
	Runner          string
	ReasoningEffort string
	Resumed         bool
	StateDir        string
	WorkDir         string
	WebSearch       *WebSearch
	StartedAt       time.Time
}

type WebSearch struct {
	Enabled   bool
	Mechanism string
	Providers []string
}

type ParticipantFinish struct {
	FinishedAt time.Time
	StateBytes *int64
	Usage      *TokenUsage
	Refusal    *ParticipantRefusal
	Error      string
}

type ParticipantRefusal struct {
	Provider         string
	Model            string
	StopReason       string
	RawStopReason    string
	WillRetry        bool
	Category         string
	Explanation      string
	SessionID        string
	RequestID        string
	ResponseID       string
	RefusedMessageID string
	StdoutArtifact   string
	SessionPath      string
}

type FinishParticipant func(ParticipantFinish) error

type Observer interface {
	StartProcess(ProcessStart) (FinishProcess, error)
}

type ParticipantObserver interface {
	StartParticipant(ParticipantStart) (FinishParticipant, error)
}

func Start(observer Observer, start ProcessStart) (FinishProcess, error) {
	if observer == nil {
		return nil, nil
	}
	if start.InstanceID == "" {
		instanceID, err := ProcessInstanceID(start.PID)
		if err != nil {
			return nil, err
		}
		start.InstanceID = instanceID
	}
	return observer.StartProcess(start)
}

func Finish(finish FinishProcess, state ProcessFinish) error {
	if finish == nil {
		return nil
	}
	return finish(state)
}

func StartParticipant(observer Observer, start ParticipantStart) (FinishParticipant, error) {
	if observer == nil {
		return nil, nil
	}
	participantObserver, ok := observer.(ParticipantObserver)
	if !ok {
		return nil, nil
	}
	return participantObserver.StartParticipant(start)
}

func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
