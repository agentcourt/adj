package lean

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type Engine struct {
	Command []string
}

var errLeanDescendantsAfterExit = errors.New("lean descendants remained after command exit")

type OpportunityAuthority struct {
	OpportunityID        string `json:"opportunity_id"`
	ExpectedStateVersion int    `json:"expected_state_version"`
	Role                 string `json:"role"`
	Phase                string `json:"phase"`
	MemberID             string `json:"member_id"`
}

func New(command []string) Engine {
	if len(command) == 0 {
		command = []string{"lake", "exe", "aarengine"}
	}
	return Engine{Command: command}
}

func (e Engine) Call(ctx context.Context, request map[string]any) (map[string]any, error) {
	if len(e.Command) == 0 {
		return nil, fmt.Errorf("lean command is empty")
	}
	wire, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	cmd := exec.CommandContext(ctx, e.Command[0], e.Command[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var cancellationRequested atomic.Bool
	cmd.Cancel = func() error {
		cancellationRequested.Store(true)
		err := killLeanProcessGroup(cmd.Process)
		if err == nil {
			return nil
		}
		if errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return fmt.Errorf("kill lean process group: %w", err)
	}
	cmd.WaitDelay = time.Second
	cmd.Stdin = bytes.NewReader(wire)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	commandErr := cmd.Run()
	runErr := commandErr
	processGroupAbsent := false
	cleanupErr := killLeanProcessGroup(cmd.Process)
	switch {
	case cleanupErr == nil:
		runErr = errors.Join(runErr, errLeanDescendantsAfterExit)
	case errors.Is(cleanupErr, os.ErrProcessDone):
		processGroupAbsent = true
	default:
		runErr = errors.Join(runErr, fmt.Errorf("kill lean process group after command exit: %w", cleanupErr))
	}
	processErr := leanProcessError(request, runErr, stderr.Bytes())
	canceledByContext := context.Cause(ctx) != nil && (cancellationRequested.Load() || cmd.Process == nil)
	if runErr != nil && canceledByContext {
		processErr = errors.Join(context.Cause(ctx), processErr)
	}
	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		reqLabel := requestLabel(request)
		if runErr != nil {
			return nil, processErr
		}
		return nil, fmt.Errorf("lean returned empty response for %s", reqLabel)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		parseErr := fmt.Errorf("parse lean json: %w", err)
		if runErr != nil {
			if canceledByContext {
				return nil, errors.Join(processErr, parseErr)
			}
			return nil, errors.Join(parseErr, processErr)
		}
		return nil, parseErr
	}
	if runErr != nil {
		if canceledByContext {
			return nil, processErr
		}
		ok, validOK := out["ok"].(bool)
		message, validMessage := out["error"].(string)
		validRejection := validOK && !ok && validMessage && strings.TrimSpace(message) != ""
		if validRejection && processGroupAbsent && isLeanProtocolRejectionExit(commandErr) {
			return out, nil
		}
		if !validRejection {
			return nil, processErr
		}
		return nil, errors.Join(processErr, fmt.Errorf("lean rejected %s: %s", requestLabel(request), strings.TrimSpace(message)))
	}
	return out, nil
}

func isLeanProtocolRejectionExit(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	return ok && exitErr.ExitCode() == 1
}

func killLeanProcessGroup(process *os.Process) error {
	if process == nil {
		return os.ErrProcessDone
	}
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}

func leanProcessError(request map[string]any, runErr error, stderr []byte) error {
	if runErr == nil {
		return nil
	}
	return fmt.Errorf("lean process failed for %s: %w stderr=%s", requestLabel(request), runErr, bytes.TrimSpace(stderr))
}

func requestLabel(request map[string]any) string {
	if value := fmt.Sprintf("%v", request["request_type"]); strings.TrimSpace(value) != "" && value != "<nil>" {
		return value
	}
	action, _ := request["action"].(map[string]any)
	if action != nil {
		if actionType := fmt.Sprintf("%v", action["action_type"]); strings.TrimSpace(actionType) != "" && actionType != "<nil>" {
			return "step:" + actionType
		}
	}
	return "request"
}

func (e Engine) InitializeCase(ctx context.Context, state map[string]any, proposition string, councilMembers []map[string]any) (map[string]any, error) {
	return e.Call(ctx, map[string]any{
		"request_type":    "initialize_case",
		"state":           state,
		"proposition":     proposition,
		"council_members": councilMembers,
	})
}

func (e Engine) Step(ctx context.Context, state map[string]any, actionType string, actorRole string, authority OpportunityAuthority, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	return e.Call(ctx, map[string]any{
		"state": state,
		"action": map[string]any{
			"action_type": actionType,
			"actor_role":  actorRole,
			"authority":   authority,
			"payload":     payload,
		},
	})
}

func (e Engine) NextOpportunity(ctx context.Context, state map[string]any) (map[string]any, error) {
	return e.Call(ctx, map[string]any{
		"request_type": "next_opportunity",
		"state":        state,
	})
}
