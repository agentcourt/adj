package localrun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const DefaultCourtSubmissionErrorLimit = 10

const courtSubmissionErrorReason = "court_submission_error_limit"

type courtSubmissionCall struct {
	tool          string
	malformedArgs bool
}

type courtSubmissionErrors struct {
	server  string
	pending map[string]courtSubmissionCall
	onError func(string)
}

func (c *courtSubmissionErrors) observe(line []byte) {
	var event struct {
		Type       string `json:"type"`
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
		IsError    bool   `json:"isError"`
		Args       struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		} `json:"args"`
		Result struct {
			Details struct {
				Error     string          `json:"error"`
				MCPResult json.RawMessage `json:"mcpResult"`
			} `json:"details"`
		} `json:"result"`
	}
	if json.Unmarshal(line, &event) != nil {
		return
	}
	switch event.Type {
	case "tool_execution_start":
		tool := event.ToolName
		if tool == "mcp" {
			tool = event.Args.Tool
		}
		name, found := strings.CutPrefix(tool, c.server+"_")
		if !found || !courtSubmissionTool(name) {
			return
		}
		if c.pending == nil {
			c.pending = make(map[string]courtSubmissionCall)
		}
		c.pending[event.ToolCallID] = courtSubmissionCall{
			tool: tool, malformedArgs: event.ToolName != "mcp" || malformedPiArgs(event.Args.Args),
		}
	case "tool_execution_end":
		call, found := c.pending[event.ToolCallID]
		delete(c.pending, event.ToolCallID)
		if !found || len(event.Result.Details.MCPResult) > 0 {
			return
		}
		switch event.Result.Details.Error {
		case "tool_not_found", "tool_not_found_after_reconnect", "ambiguous_tool":
			c.onError(call.tool)
		default:
			if event.IsError && call.malformedArgs {
				c.onError(call.tool)
			}
		}
	}
}

func malformedPiArgs(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return true
	}
	if text, ok := value.(string); ok {
		if text == "" {
			return false
		}
		if json.Unmarshal([]byte(text), &value) != nil {
			return true
		}
	}
	_, object := value.(map[string]any)
	return !object
}

func courtSubmissionTool(name string) bool {
	switch name {
	case "", "get_current_opportunity", "wait_for_opportunity", "case_status",
		"get_case", "get_case_result", "explain_decisions", "list_case_files",
		"read_case_text_file", "request_case_file", "read_case_file_bytes",
		"get_juror_context", "send_work_notes", "report_failure":
		return false
	default:
		return true
	}
}

func courtSubmissionFailure(count, limit int, tool string) (string, map[string]any) {
	return fmt.Sprintf("Court submission error limit reached: %d errors, limit %d; last tool %s", count, limit, tool), map[string]any{
		"court_submission_errors": count, "court_submission_error_limit": limit, "last_tool": tool,
	}
}

func (s *runState) lawyerCourtSubmissionObserver(ctx context.Context, role, server string) func([]byte) {
	var previous jurorTurn
	count := 0
	finished := false
	tracker := &courtSubmissionErrors{server: server}
	tracker.onError = func(tool string) {
		if finished || ctx.Err() != nil {
			return
		}
		err := func() error {
			status, err := s.lawyerStatus(ctx, role)
			if err != nil {
				return fmt.Errorf("check %s court submission opportunity: %w", role, err)
			}
			if status.Status != "active" || status.CurrentTurn == nil {
				return nil
			}
			turn := jurorTurn{opportunityID: status.CurrentTurn.OpportunityID, stateVersion: status.CurrentTurn.StateVersion}
			if turn != previous {
				previous, count = turn, 0
			}
			count++
			if count < s.opts.CourtSubmissionErrorLimit {
				return nil
			}
			finished = true
			message, details := courtSubmissionFailure(count, s.opts.CourtSubmissionErrorLimit, tool)
			return s.reportParticipantFailure(ctx, role, turn, courtSubmissionErrorReason, message, details)
		}()
		if err != nil {
			finished = true
			select {
			case s.agentErrs <- err:
			case <-ctx.Done():
			}
		}
	}
	return tracker.observe
}
