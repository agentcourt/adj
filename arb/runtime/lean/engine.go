package lean

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Engine struct {
	Command []string
}

func New(command []string) Engine {
	if len(command) == 0 {
		command = []string{"lake", "exe", "aarengine"}
	}
	return Engine{Command: command}
}

func (e Engine) Call(request map[string]any) (map[string]any, error) {
	if len(e.Command) == 0 {
		return nil, fmt.Errorf("lean command is empty")
	}
	wire, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	cmd := exec.Command(e.Command[0], e.Command[1:]...)
	cmd.Stdin = bytes.NewReader(wire)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		reqLabel := requestLabel(request)
		if runErr != nil {
			return nil, fmt.Errorf("lean process failed for %s: %w stderr=%s", reqLabel, runErr, bytes.TrimSpace(stderr.Bytes()))
		}
		return nil, fmt.Errorf("lean returned empty response for %s", reqLabel)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse lean json: %w", err)
	}
	if runErr != nil {
		return out, nil
	}
	return out, nil
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

func (e Engine) InitializeCase(state map[string]any, proposition string, councilMembers []map[string]any) (map[string]any, error) {
	return e.Call(map[string]any{
		"request_type":    "initialize_case",
		"state":           state,
		"proposition":     proposition,
		"council_members": councilMembers,
	})
}

func (e Engine) Step(state map[string]any, actionType string, actorRole string, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	return e.Call(map[string]any{
		"state": state,
		"action": map[string]any{
			"action_type": actionType,
			"actor_role":  actorRole,
			"payload":     payload,
		},
	})
}

func (e Engine) NextOpportunity(state map[string]any) (map[string]any, error) {
	return e.Call(map[string]any{
		"request_type": "next_opportunity",
		"state":        state,
	})
}
