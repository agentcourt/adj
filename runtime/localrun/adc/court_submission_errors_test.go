package localrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCourtSubmissionErrors(t *testing.T) {
	for _, test := range []struct {
		name, tool string
		args       any
		isError    bool
		details    map[string]any
		want       int
	}{
		{name: "malformed JSON", tool: "adc_submit_decision", args: `{`, isError: true, want: 1},
		{name: "invented legal tool", tool: "adc_answer_juror_questionnaire", args: `{}`, details: map[string]any{"error": "tool_not_found"}, want: 1},
		{name: "non-object", tool: "adc_submit_decision", args: `[]`, isError: true, want: 1},
		{name: "object arguments", tool: "adc_submit_decision", args: map[string]any{"kind": "pass"}},
		{name: "court decision rejection", tool: "adc_submit_decision", args: `{}`, isError: true, details: map[string]any{"error": "tool_error", "mcpResult": map[string]any{"isError": true}}},
		{name: "read error", tool: "adc_get_case", args: `{`, isError: true},
		{name: "file error", tool: "adc_request_case_file", args: `{`, isError: true},
		{name: "notes error", tool: "adc_send_work_notes", args: `{`, isError: true},
		{name: "other server", tool: "search_submit_decision", args: `{`, isError: true},
		{name: "transport error", tool: "adc_submit_decision", args: `{}`, isError: true, details: map[string]any{"error": "call_failed"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			count := 0
			tracker := courtSubmissionErrors{server: "adc", onError: func(string) { count++ }}
			feedSubmission(t, tracker.observe, test.tool, test.args, test.isError, test.details)
			if count != test.want || len(tracker.pending) != 0 {
				t.Fatalf("count=%d, pending=%d; want count=%d", count, len(tracker.pending), test.want)
			}
		})
	}
}

func TestLawyerCourtSubmissionLimitResetsPerOpportunity(t *testing.T) {
	version, failures := 10, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{"ok": true}
		if r.URL.Path == "/roleapi/v1/status" {
			response["status"] = "active"
			response["current_turn"] = map[string]any{"opportunity_id": "o1", "state_version": version}
		} else if r.URL.Path == "/roleapi/v1/fail" {
			var request struct {
				Role    string `json:"role_id"`
				Version int    `json:"state_version"`
				Reason  string `json:"reason"`
				Details struct {
					Count int `json:"court_submission_errors"`
				} `json:"details"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Role != "plaintiff" || request.Version != 11 || request.Reason != courtSubmissionErrorReason || request.Details.Count != 2 {
				t.Errorf("failure = %+v", request)
			}
			failures++
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	state := runState{caseBase: server.URL, opts: Options{CaseID: "test", CourtSubmissionErrorLimit: 2}, agentErrs: make(chan error, 1)}
	observe := state.lawyerCourtSubmissionObserver(context.Background(), "plaintiff", "adc")
	feedSubmission(t, observe, "adc_submit_decision", `{`, true, nil)
	version = 11
	feedSubmission(t, observe, "adc_submit_decision", `{`, true, nil)
	if failures != 0 {
		t.Fatal("error counter carried into the next opportunity")
	}
	feedSubmission(t, observe, "adc_submit_decision", `{`, true, nil)
	feedSubmission(t, observe, "adc_submit_decision", `{`, true, nil)
	if failures != 1 {
		t.Fatalf("failure reports = %d, want 1", failures)
	}
	select {
	case err := <-state.agentErrs:
		t.Fatal(err)
	default:
	}
}

func feedSubmission(t *testing.T, observe func([]byte), tool string, args any, isError bool, details map[string]any) {
	t.Helper()
	for _, event := range []map[string]any{
		{"type": "tool_execution_start", "toolName": "mcp", "toolCallId": "call-1", "args": map[string]any{"tool": tool, "args": args}},
		{"type": "tool_execution_end", "toolName": "mcp", "toolCallId": "call-1", "isError": isError, "result": map[string]any{"details": details}},
	} {
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		observe(raw)
	}
}
