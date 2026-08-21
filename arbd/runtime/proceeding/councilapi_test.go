package proceeding

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCouncilWaitReturnsReadyOpportunity(t *testing.T) {
	api, _ := testCouncilAPIWithTurn(t)

	status, got := callCouncilAPIWait(t, api, "case_id=arbd-1&member_id=C1&timeout_ms=100")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "ready" {
		t.Fatalf("status field = %#v, want ready", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "ready" {
		t.Fatalf("wait.reason = %#v, want ready", wait["reason"])
	}
	turn, ok := got["turn"].(map[string]any)
	if !ok {
		t.Fatalf("turn = %#v, want object", got["turn"])
	}
	if turn["opportunity_id"] != "deliberation:1:C1" {
		t.Fatalf("opportunity_id = %#v, want deliberation:1:C1", turn["opportunity_id"])
	}
}

func TestCouncilAPIRejectsMismatchedCaseID(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)
	api.rc.cfg.CaseID = "case-123"

	status, got := callCouncilAPIWait(t, api, "case_id=case-456&member_id=C1&timeout_ms=100")
	if status != http.StatusNotFound {
		t.Fatalf("wait status = %d, want %d", status, http.StatusNotFound)
	}
	if code := councilAPIErrorCode(t, got); code != "unknown_case" {
		t.Fatalf("wait error code = %q, want unknown_case", code)
	}

	status, got = callCouncilAPIDo(t, api, map[string]any{
		"case_id":        "case-456",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"tool":           "get_case",
		"arguments":      map[string]any{},
	})
	if status != http.StatusNotFound {
		t.Fatalf("do status = %d, want %d", status, http.StatusNotFound)
	}
	if code := councilAPIErrorCode(t, got); code != "unknown_case" {
		t.Fatalf("do error code = %q, want unknown_case", code)
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
}

func TestCouncilWaitReturnsDoneOnTerminalCase(t *testing.T) {
	api, _ := testCouncilAPIWithTurn(t)
	api.rc.mu.Lock()
	api.rc.setRoleAPIsTerminalLocked("threshold_met")
	api.rc.mu.Unlock()

	status, got := callCouncilAPIWait(t, api, "case_id=arbd-1&member_id=C1&after=deliberation:1:C1&timeout_ms=1000")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "done" {
		t.Fatalf("status field = %#v, want done", got["status"])
	}
	if got["final_reason"] != "threshold_met" {
		t.Fatalf("final_reason = %#v, want threshold_met", got["final_reason"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "done" {
		t.Fatalf("wait.reason = %#v, want done", wait["reason"])
	}
}

func TestCouncilWaitReturnsFailedForFailedMember(t *testing.T) {
	api, _ := testCouncilAPIWithTurn(t)
	api.active = nil
	caseObj := mapAny(api.rc.state["case"])
	caseObj["council_members"] = []map[string]any{{
		"member_id":              "C1",
		"status":                 "failed",
		"failure_reason":         opportunityFailureAttemptsExhausted,
		"failure_opportunity_id": "deliberation:1:C1",
		"failure_message":        "Council member C1 exhausted attempts.",
	}}

	status, got := callCouncilAPIWait(t, api, "case_id=arbd-1&member_id=C1&timeout_ms=100")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "failed" {
		t.Fatalf("status field = %#v, want failed", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "failed" {
		t.Fatalf("wait = %#v, want reason failed", got["wait"])
	}
	failure, ok := got["failure"].(map[string]any)
	if !ok || failure["reason"] != opportunityFailureAttemptsExhausted || failure["member_id"] != "C1" {
		t.Fatalf("failure = %#v", got["failure"])
	}
	tools := mapList(got["tools"])
	if len(tools) != 0 {
		t.Fatalf("tools = %#v, want none", tools)
	}
}

func TestCouncilDoRequiresActiveOpportunityID(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)

	for _, tc := range []struct {
		name          string
		opportunityID string
		wantCode      string
	}{
		{name: "missing", wantCode: "missing_opportunity_id"},
		{name: "stale", opportunityID: "deliberation:1:C2", wantCode: "stale_opportunity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"case_id":   "arbd-1",
				"member_id": "C1",
				"tool":      "get_case",
				"arguments": map[string]any{},
			}
			if tc.opportunityID != "" {
				body["opportunity_id"] = tc.opportunityID
			}
			status, got := callCouncilAPIDo(t, api, body)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if got["ok"] != false {
				t.Fatalf("ok = %#v, want false", got["ok"])
			}
			if code := councilAPIErrorCode(t, got); code != tc.wantCode {
				t.Fatalf("error code = %q, want %q", code, tc.wantCode)
			}
			if turn.attemptsRemaining != turn.attemptsMax {
				t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
			}
		})
	}
}

func TestCouncilPostRejectsTrailingJSONValueWithoutCountingAttempt(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{
			name: "do",
			path: councilAPIBasePath + "/do",
			body: `{"case_id":"arbd-1","member_id":"C1","opportunity_id":"deliberation:1:C1","tool":"get_case","arguments":{}} {}`,
		},
		{
			name: "fail",
			path: councilAPIBasePath + "/fail",
			body: `{"case_id":"arbd-1","member_id":"C1","opportunity_id":"deliberation:1:C1"} {}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api, turn := testCouncilAPIWithTurn(t)
			handler := api.handleDo
			if tc.name == "fail" {
				handler = api.handleFail
			}
			status, got := callCouncilAPIPostRaw(t, handler, tc.path, tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
			}
			if code := councilAPIErrorCode(t, got); code != "bad_json" {
				t.Fatalf("error code = %q, want bad_json", code)
			}
			if turn.attemptsRemaining != turn.attemptsMax {
				t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
			}
			if turn.completed {
				t.Fatal("trailing JSON completed the turn")
			}
		})
	}
}

func TestCouncilDoRejectsUnknownToolArgumentFields(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tool      string
		arguments map[string]any
		unknown   string
	}{
		{name: "get case", tool: "get_case", arguments: map[string]any{"unexpected": true}, unknown: "unexpected"},
		{name: "list evidence", tool: "list_evidence", arguments: map[string]any{"unexpected": true}, unknown: "unexpected"},
		{name: "stat evidence", tool: "stat_evidence", arguments: map[string]any{"evidence_id": "ev-1", "unexpected": true}, unknown: "unexpected"},
		{name: "read evidence", tool: "read_evidence_range", arguments: map[string]any{"evidence_id": "ev-1", "offset": 0, "length": 1, "unexpected": true}, unknown: "unexpected"},
		{name: "answer", tool: "submit_council_answer", arguments: map[string]any{"answer": 72, "rationale": "Sufficient.", "unexpected": true}, unknown: "unexpected"},
		{name: "caller member id", tool: "submit_council_answer", arguments: map[string]any{"answer": 72, "rationale": "Sufficient.", "member_id": "C2"}, unknown: "member_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, turn := testCouncilAPIWithTurn(t)
			status, got := callCouncilAPIDo(t, api, map[string]any{
				"case_id":        "arbd-1",
				"member_id":      "C1",
				"opportunity_id": "deliberation:1:C1",
				"tool":           tc.tool,
				"arguments":      tc.arguments,
			})
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if got["ok"] != false || councilAPIErrorCode(t, got) != "tool_failed" {
				t.Fatalf("response = %#v, want tool_failed", got)
			}
			if message := mapString(mapAny(got["error"])["message"]); !strings.Contains(message, "unknown field \""+tc.unknown+"\"") {
				t.Fatalf("error message = %q, want unknown field", message)
			}
			if turn.attemptsRemaining != turn.attemptsMax-1 {
				t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax-1)
			}
			if turn.completed {
				t.Fatal("unknown tool argument completed the turn")
			}
		})
	}
}

func TestCouncilDoRejectsOtherMemberWithoutCountingAttempt(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)

	status, got := callCouncilAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C2",
		"opportunity_id": "deliberation:1:C1",
		"tool":           "get_case",
		"arguments":      map[string]any{},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %#v, want false", got["ok"])
	}
	if code := councilAPIErrorCode(t, got); code != "not_current_turn" {
		t.Fatalf("error code = %q, want not_current_turn", code)
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
}

func TestCouncilSubmitAnswerAcceptsCurrentOpportunity(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)

	status, got := callCouncilAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"tool":           "submit_council_answer",
		"arguments": map[string]any{
			"answer":    72,
			"rationale": "record sufficient",
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true in %#v", got["ok"], got)
	}
	if !turn.completed {
		t.Fatalf("turn.completed = false, want true")
	}
	caseObj := mapAny(api.rc.state["case"])
	answers := mapList(caseObj["council_answers"])
	if len(answers) != 1 {
		t.Fatalf("answer count = %d, want 1", len(answers))
	}
	if got := mapString(answers[0]["member_id"]); got != "C1" {
		t.Fatalf("answer member_id = %q, want C1", got)
	}
	if got := intNumber(answers[0]["answer"]); got != 72 {
		t.Fatalf("answer = %d, want 72", got)
	}
}

func TestCouncilSubmitAnswerEngineCallCrossingDeadlineRecordsDeadlineFailure(t *testing.T) {
	api, turn := testCouncilAPIWithTurnFailureReason(t, opportunityFailureDeadline)
	marker := filepath.Join(t.TempDir(), "engine-entered")
	api.rc.cfg.Engine.Command = []string{"/bin/sh", "-c", blockingCouncilEngineScript(opportunityFailureDeadline), "blocking-engine", marker}
	turn.deadline = time.Now().Add(500 * time.Millisecond)

	status, got := callCouncilAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": turn.opportunity.ID,
		"tool":           "submit_council_answer",
		"arguments": map[string]any{
			"answer":    72,
			"rationale": "Record sufficient.",
		},
	})
	if status != http.StatusOK || got["ok"] != false || councilAPIErrorCode(t, got) != "turn_timeout" {
		t.Fatalf("status = %d, response = %#v, want turn_timeout", status, got)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("blocking engine did not start: %v", err)
	}
	assertCouncilAPIDeadlineFailure(t, api, turn)
}

func TestCouncilFailRecordsMemberFailure(t *testing.T) {
	api, turn := testCouncilAPIWithTurnFailureReason(t, opportunityFailureAgentExited)

	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"message":        "Council member C1 agent process exited before submitting an answer.",
		"details":        map[string]any{"exit_status": "0"},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true in %#v", got["ok"], got)
	}
	if !turn.completed {
		t.Fatalf("turn.completed = false, want true")
	}
	caseObj := mapAny(api.rc.state["case"])
	members := mapList(caseObj["council_members"])
	if len(members) == 0 {
		t.Fatalf("missing council members")
	}
	member := members[0]
	if mapString(member["status"]) != "failed" {
		t.Fatalf("member status = %#v, want failed", member["status"])
	}
	if mapString(member["failure_reason"]) != opportunityFailureAgentExited {
		t.Fatalf("failure_reason = %#v, want %s", member["failure_reason"], opportunityFailureAgentExited)
	}
	if mapString(member["failure_opportunity_id"]) != "deliberation:1:C1" {
		t.Fatalf("failure_opportunity_id = %#v", member["failure_opportunity_id"])
	}
	failure := mapAny(got["failure"])
	if mapString(failure["member_id"]) != "C1" || mapString(failure["reason"]) != opportunityFailureAgentExited {
		t.Fatalf("failure = %#v", got["failure"])
	}
}

func TestCouncilFailEngineCallCrossingDeadlineRecordsDeadlineFailure(t *testing.T) {
	api, turn := testCouncilAPIWithTurnFailureReason(t, opportunityFailureDeadline)
	marker := filepath.Join(t.TempDir(), "engine-entered")
	api.rc.cfg.Engine.Command = []string{"/bin/sh", "-c", blockingCouncilEngineScript(opportunityFailureDeadline), "blocking-engine", marker}
	turn.deadline = time.Now().Add(500 * time.Millisecond)

	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": turn.opportunity.ID,
		"reason":         opportunityFailureAgentExited,
		"message":        "agent exited",
	})
	if status != http.StatusOK || got["ok"] != false || councilAPIErrorCode(t, got) != "turn_timeout" {
		t.Fatalf("status = %d, response = %#v, want turn_timeout", status, got)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("blocking engine did not start: %v", err)
	}
	assertCouncilAPIDeadlineFailure(t, api, turn)
}

func TestCouncilFailPreservesAuthoritativePayloadFields(t *testing.T) {
	api, turn := testCouncilAPIWithTurnFailureReason(t, opportunityFailureAgentExited)
	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": turn.opportunity.ID,
		"reason":         opportunityFailureAgentExited,
		"message":        "reported exit",
		"details": map[string]any{
			"type":           "forged_type",
			"role":           "plaintiff",
			"phase":          "openings",
			"opportunity_id": "forged_opportunity",
			"reason":         opportunityFailureDeadline,
			"message":        "forged message",
			"member_id":      "C9",
			"exit_status":    "17",
		},
	})
	if status != http.StatusOK || got["ok"] != true {
		t.Fatalf("status = %d, response = %#v", status, got)
	}
	if len(api.rc.events) == 0 || api.rc.events[0].Type != "opportunity_failed" {
		t.Fatalf("events = %#v, want opportunity_failed", api.rc.events)
	}
	payload := api.rc.events[0].Payload
	want := map[string]any{
		"type":           "opportunity_failed",
		"role":           "council",
		"phase":          "deliberation",
		"opportunity_id": turn.opportunity.ID,
		"reason":         opportunityFailureAgentExited,
		"message":        "reported exit",
		"member_id":      "C1",
		"exit_status":    "17",
	}
	for key, value := range want {
		if payload[key] != value {
			t.Fatalf("payload[%q] = %#v, want %#v in %#v", key, payload[key], value, payload)
		}
	}
}

func TestCouncilFailRecordsOutputLimitReason(t *testing.T) {
	api, turn := testCouncilAPIWithTurnFailureReason(t, opportunityFailureAgentOutputLimit)

	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"reason":         opportunityFailureAgentOutputLimit,
		"message":        "Council member C1 agent process exceeded the output limit.",
		"details": map[string]any{
			"output_bytes":       11,
			"output_limit_bytes": 10,
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true in %#v", got["ok"], got)
	}
	if !turn.completed {
		t.Fatalf("turn.completed = false, want true")
	}
	caseObj := mapAny(api.rc.state["case"])
	members := mapList(caseObj["council_members"])
	if len(members) == 0 {
		t.Fatalf("missing council members")
	}
	member := members[0]
	if mapString(member["failure_reason"]) != opportunityFailureAgentOutputLimit {
		t.Fatalf("failure_reason = %#v, want %s", member["failure_reason"], opportunityFailureAgentOutputLimit)
	}
	failure := mapAny(got["failure"])
	if mapString(failure["reason"]) != opportunityFailureAgentOutputLimit {
		t.Fatalf("failure = %#v", got["failure"])
	}
}

func TestCouncilFailAfterDeadlineRecordsDeadlineExpiration(t *testing.T) {
	api, turn := testCouncilAPIWithTurnFailureReason(t, opportunityFailureDeadline)
	turn.deadline = time.Now().Add(-time.Second)

	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": turn.opportunity.ID,
		"reason":         opportunityFailureAgentExited,
		"message":        "agent exited after its deadline",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != false || councilAPIErrorCode(t, got) != "turn_timeout" {
		t.Fatalf("response = %#v, want turn_timeout", got)
	}
	turnPayload := mapAny(got["turn"])
	if turnPayload["completed"] != true || intNumber(turnPayload["attempts_remaining"]) != turn.attemptsMax {
		t.Fatalf("turn = %#v, want completed without attempt charge", turnPayload)
	}
	caseObj := mapAny(api.rc.state["case"])
	members := mapList(caseObj["council_members"])
	if len(members) != 1 || mapString(members[0]["failure_reason"]) != opportunityFailureDeadline {
		t.Fatalf("council members = %#v, want deadline failure", members)
	}
	select {
	case err := <-turn.done:
		if err != nil {
			t.Fatalf("deadline completion error = %v, want nil", err)
		}
	default:
		t.Fatal("deadline transition did not complete the turn")
	}
}

func TestCouncilFailRejectsWrongCurrentTurn(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)

	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C2",
		"opportunity_id": "deliberation:1:C1",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %#v, want false", got["ok"])
	}
	if code := councilAPIErrorCode(t, got); code != "not_current_turn" {
		t.Fatalf("error code = %q, want not_current_turn", code)
	}
	if turn.completed {
		t.Fatalf("turn.completed = true, want false")
	}
}

func TestCouncilFailRejectsStaleOpportunity(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)

	status, got := callCouncilAPIFail(t, api, map[string]any{
		"case_id":        "arbd-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C2",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %#v, want false", got["ok"])
	}
	if code := councilAPIErrorCode(t, got); code != "stale_opportunity" {
		t.Fatalf("error code = %q, want stale_opportunity", code)
	}
	if turn.completed {
		t.Fatalf("turn.completed = true, want false")
	}
}

func testCouncilAPIWithTurn(t *testing.T) (*councilAPIServer, *councilTurn) {
	t.Helper()
	return testCouncilAPIWithTurnFailureReason(t, "")
}

func testCouncilAPIWithTurnFailureReason(t *testing.T, failureReason string) (*councilAPIServer, *councilTurn) {
	t.Helper()
	rc := newCouncilOpportunityTestContext(t, failureReason)
	turn := &councilTurn{
		opportunity: Opportunity{
			ID:           "deliberation:1:C1",
			StateVersion: 1,
			Role:         "council",
			Phase:        "deliberation",
			MemberID:     "C1",
			AllowedTools: []string{"submit_council_answer"},
		},
		seat:              rc.council[0],
		turnNumber:        1,
		prompt:            "Council prompt.",
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       3,
		attemptsRemaining: 3,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
	api := &councilAPIServer{rc: rc, active: turn, version: 1}
	rc.councilAPI = api
	return api, turn
}

func assertCouncilAPIDeadlineFailure(t *testing.T, api *councilAPIServer, turn *councilTurn) {
	t.Helper()
	if !turn.completed {
		t.Fatal("turn remained active after deadline transition")
	}
	if len(api.rc.certificateActions) != 1 {
		t.Fatalf("certificate action count = %d, want one deadline transition", len(api.rc.certificateActions))
	}
	if answers := mapList(mapAny(api.rc.state["case"])["council_answers"]); len(answers) != 0 {
		t.Fatalf("council answers = %#v, want none", answers)
	}
	assertFailedCouncilMember(t, api.rc, opportunityFailureDeadline)
	select {
	case err := <-turn.done:
		if err != nil {
			t.Fatalf("deadline completion error = %v, want nil", err)
		}
	default:
		t.Fatal("deadline transition did not publish turn completion")
	}
}

func callCouncilAPIDo(t *testing.T, api *councilAPIServer, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return callCouncilAPIPostRaw(t, api.handleDo, councilAPIBasePath+"/do", string(raw))
}

func callCouncilAPIFail(t *testing.T, api *councilAPIServer, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return callCouncilAPIPostRaw(t, api.handleFail, councilAPIBasePath+"/fail", string(raw))
}

func callCouncilAPIPostRaw(t *testing.T, handler http.HandlerFunc, path string, raw string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func callCouncilAPIWait(t *testing.T, api *councilAPIServer, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, councilAPIBasePath+"/wait?"+query, nil)
	rec := httptest.NewRecorder()
	api.handleWait(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func councilAPIErrorCode(t *testing.T, got map[string]any) string {
	t.Helper()
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want object", got["error"])
	}
	return mapString(errObj["code"])
}
