package proceeding

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/arb/runtime/lean"
)

func TestRoleAPIParticipantInputConsumesOneAttempt(t *testing.T) {
	t.Run("lawyer malformed decision", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		turn.opportunity = Opportunity{
			ID:           "arguments:plaintiff",
			Role:         "plaintiff",
			Phase:        "arguments",
			AllowedTools: []string{"submit_argument"},
		}
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "submit_decision",
			"arguments": map[string]any{
				"kind":      "tool",
				"tool_name": "submit_argument",
				"payload": map[string]any{
					"text":             "Argument.",
					"offered_evidence": nil,
				},
			},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertParticipantInputResponse(t, body, turn.attemptsMax-1, turn.done)
	})

	t.Run("lawyer evidence range", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		meta := installTestEvidence(t, api.rc, []byte("record"))
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "read_evidence_range",
			"arguments": map[string]any{
				"evidence_id": meta.EvidenceID,
				"offset":      meta.SizeBytes + 1,
				"length":      1,
			},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertParticipantInputResponse(t, body, turn.attemptsMax-1, turn.done)
	})

	t.Run("lawyer unknown upload", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		turn.opportunity = Opportunity{ID: "arguments:plaintiff", Role: "plaintiff", Phase: "arguments"}
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "write_evidence_chunk",
			"arguments": map[string]any{
				"upload_id":      "unknown",
				"offset":         0,
				"content_base64": "YQ==",
			},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertParticipantInputResponse(t, body, turn.attemptsMax-1, turn.done)
	})

	t.Run("council invalid vote", func(t *testing.T) {
		api, turn := testCouncilAPIWithTurn(t)
		status, body := callCouncilAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"member_id":      "C1",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "submit_council_vote",
			"arguments": map[string]any{
				"vote":      "abstain",
				"rationale": "Insufficient basis.",
			},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertParticipantInputResponse(t, body, turn.attemptsMax-1, turn.done)
	})
}

func TestLawyerRoleAPIRuntimeFailuresCompleteTurn(t *testing.T) {
	t.Run("work notes persistence", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		outputFile := filepath.Join(t.TempDir(), "output-file")
		if err := os.WriteFile(outputFile, []byte("not a directory"), 0o644); err != nil {
			t.Fatalf("write output blocker: %v", err)
		}
		api.rc.cfg.OutputDir = outputFile
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "send_work_notes",
			"arguments":      map[string]any{"notes": "Check the record."},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		err := assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
		if !strings.Contains(err.Error(), "work-notes.ndjson") {
			t.Fatalf("runtime error = %q, want work-notes path", err)
		}
	})

	t.Run("missing nonempty upload staging file", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		turn.opportunity = Opportunity{ID: "arguments:plaintiff", Role: "plaintiff", Phase: "arguments"}
		session := &EvidenceUploadSession{
			UploadID:          "upl_missing",
			Role:              "plaintiff",
			Phase:             "arguments",
			ExpectedSizeBytes: 2,
			ReceivedBytes:     1,
			Path:              filepath.Join(t.TempDir(), "missing.part"),
		}
		api.rc.uploadSessions[session.UploadID] = session
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "write_evidence_chunk",
			"arguments": map[string]any{
				"upload_id":      session.UploadID,
				"offset":         1,
				"content_base64": "YQ==",
			},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		err := assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
		if !strings.Contains(err.Error(), "stat evidence upload") {
			t.Fatalf("runtime error = %q, want staging stat failure", err)
		}
	})

	t.Run("evidence manifest publication", func(t *testing.T) {
		dir := t.TempDir()
		api, turn := newSubmissionTransactionTestAPI(t, dir)
		manifestPath := filepath.Join(dir, "evidence-manifest.json")
		if err := os.Mkdir(manifestPath, 0o755); err != nil {
			t.Fatalf("block evidence manifest path: %v", err)
		}
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "submit_evidence",
			"arguments":      transactionEvidenceArgs(),
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		err := assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
		if !strings.Contains(err.Error(), "evidence-manifest.json") {
			t.Fatalf("runtime error = %q, want evidence manifest path", err)
		}
	})
}

func TestRoleAPIEngineFailuresDoNotConsumeAttempts(t *testing.T) {
	responses := []struct {
		name     string
		response string
	}{
		{name: "Lean rejection", response: `{"ok":false,"error":"engine rejected valid action"}`},
		{name: "malformed accepted state", response: `{"ok":true,"state":{"case":{"status":"active"}}}`},
	}
	for _, tc := range responses {
		t.Run("lawyer "+tc.name, func(t *testing.T) {
			api, turn := testLawyerAPIWithTurn()
			api.rc.cfg.Engine = roleAPITestEngine(tc.response)
			status, body := callLawyerAPIDo(t, api, map[string]any{
				"case_id":        "arb-1",
				"role_id":        "plaintiff",
				"opportunity_id": turn.opportunity.ID,
				"tool":           "submit_decision",
				"arguments": map[string]any{
					"kind":      "tool",
					"tool_name": "record_opening_statement",
					"payload":   map[string]any{"text": "Opening."},
				},
			})
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
			if len(api.rc.certificateActions) != 0 {
				t.Fatalf("certificate actions = %d, want 0", len(api.rc.certificateActions))
			}
		})

		t.Run("council "+tc.name, func(t *testing.T) {
			api, turn := testCouncilAPIWithTurn(t)
			api.rc.cfg.Engine = roleAPITestEngine(tc.response)
			status, body := callCouncilAPIDo(t, api, map[string]any{
				"case_id":        "arb-1",
				"member_id":      "C1",
				"opportunity_id": turn.opportunity.ID,
				"tool":           "submit_council_vote",
				"arguments": map[string]any{
					"vote":      "demonstrated",
					"rationale": "Record sufficient.",
				},
			})
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
			if len(api.rc.certificateActions) != 0 {
				t.Fatalf("certificate actions = %d, want 0", len(api.rc.certificateActions))
			}
		})
	}
}

func TestCouncilAcceptedVoteEventFailureCompletesWithoutAttemptCharge(t *testing.T) {
	api, turn := testCouncilAPIWithTurn(t)
	outputFile := filepath.Join(t.TempDir(), "output-file")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write output blocker: %v", err)
	}
	api.rc.cfg.OutputDir = outputFile
	status, body := callCouncilAPIDo(t, api, map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": turn.opportunity.ID,
		"tool":           "submit_council_vote",
		"arguments": map[string]any{
			"vote":      "demonstrated",
			"rationale": "Record sufficient.",
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
	if len(api.rc.certificateActions) != 1 || len(api.rc.events) != 1 {
		t.Fatalf("accepted vote commit = %d actions and %d events, want 1 and 1", len(api.rc.certificateActions), len(api.rc.events))
	}
	votes := mapList(mapAny(api.rc.state["case"])["council_votes"])
	if len(votes) != 1 || mapString(votes[0]["vote"]) != "demonstrated" {
		t.Fatalf("committed votes = %#v, want one demonstrated vote", votes)
	}
}

func TestCouncilAPIRuntimeFailurePropagatesFromOpportunity(t *testing.T) {
	rc := newCouncilOpportunityTestContext(t, "")
	rc.cfg.CaseID = "arb-1"
	rc.cfg.PromptDir = testPromptDir()
	rc.cfg.Engine = roleAPITestEngine(`{"ok":false,"error":"engine rejected valid vote"}`)
	api := newCouncilAPIServer(rc)
	rc.councilAPI = api
	opportunity := Opportunity{ID: "deliberation:1:C1", StateVersion: 1, Role: "council", Phase: "deliberation", MemberID: "C1"}
	executionDone := make(chan error, 1)
	go func() {
		executionDone <- rc.executeCouncilAPIOpportunity(context.Background(), opportunity, rc.council[0])
	}()

	deadline := time.Now().Add(time.Second)
	var turn *councilTurn
	for turn == nil && time.Now().Before(deadline) {
		rc.mu.Lock()
		turn = api.active
		rc.mu.Unlock()
		if turn == nil {
			time.Sleep(time.Millisecond)
		}
	}
	if turn == nil {
		t.Fatal("council opportunity did not become active")
	}
	status, body := callCouncilAPIDo(t, api, map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": opportunity.ID,
		"tool":           "submit_council_vote",
		"arguments": map[string]any{
			"vote":      "demonstrated",
			"rationale": "Record sufficient.",
		},
	})
	if status != http.StatusOK || mapString(mapAny(body["error"])["code"]) != "runtime_failure" {
		t.Fatalf("status = %d, response = %#v", status, body)
	}
	turnPayload := mapAny(body["turn"])
	if turnPayload["completed"] != true || intNumber(turnPayload["attempts_remaining"]) != turn.attemptsMax {
		t.Fatalf("runtime turn = %#v, want completed without attempt charge", turnPayload)
	}
	select {
	case err := <-executionDone:
		if err == nil || err.Error() != mapString(mapAny(body["error"])["message"]) {
			t.Fatalf("opportunity error = %v, response = %#v", err, body["error"])
		}
	case <-time.After(time.Second):
		t.Fatal("council opportunity did not return the handler error")
	}
}

func TestMalformedAcceptedFailureStateIsRuntimeFailure(t *testing.T) {
	engine := roleAPITestEngine(`{"ok":true,"state":{"case":{"status":"failed"}}}`)

	t.Run("lawyer deadline", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		api.rc.cfg.Engine = engine
		stateBefore := cloneJSONLikeMap(api.rc.state)
		turn.deadline = time.Now().Add(-time.Second)
		status, body := callLawyerAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"role_id":        "plaintiff",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "get_case",
			"arguments":      map[string]any{},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
		assertFailedTransitionUnchanged(t, api.rc, stateBefore, 0)
	})

	t.Run("council attempts exhausted", func(t *testing.T) {
		api, turn := testCouncilAPIWithTurn(t)
		api.rc.cfg.Engine = engine
		turn.attemptsMax = 1
		turn.attemptsRemaining = 1
		stateBefore := cloneJSONLikeMap(api.rc.state)
		status, body := callCouncilAPIDo(t, api, map[string]any{
			"case_id":        "arb-1",
			"member_id":      "C1",
			"opportunity_id": turn.opportunity.ID,
			"tool":           "unknown",
			"arguments":      map[string]any{},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertRuntimeFailureResponse(t, body, 0, turn.done)
		assertFailedTransitionUnchanged(t, api.rc, stateBefore, 0)
	})

	t.Run("council fail", func(t *testing.T) {
		api, turn := testCouncilAPIWithTurn(t)
		api.rc.cfg.Engine = engine
		stateBefore := cloneJSONLikeMap(api.rc.state)
		status, body := callCouncilAPIFail(t, api, map[string]any{
			"case_id":        "arb-1",
			"member_id":      "C1",
			"opportunity_id": turn.opportunity.ID,
			"reason":         opportunityFailureAgentExited,
			"message":        "agent exited",
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done)
		assertFailedTransitionUnchanged(t, api.rc, stateBefore, 0)
	})
}

func TestFailureTransitionRejectsIncompleteOrWrongVersionState(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response string
		want     string
	}{
		{name: "missing case", response: `{"ok":true,"state":{"state_version":2}}`, want: "empty case state"},
		{name: "empty case", response: `{"ok":true,"state":{"case":{},"state_version":2}}`, want: "empty case state"},
		{name: "unchanged version", response: `{"ok":true,"state":{"case":{"status":"failed"},"state_version":1}}`, want: "want 2"},
		{name: "jumped version", response: `{"ok":true,"state":{"case":{"status":"failed"},"state_version":3}}`, want: "want 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc := newCouncilOpportunityTestContext(t, "")
			rc.cfg.Engine = roleAPITestEngine(tc.response)
			opportunity := Opportunity{
				ID:           "deliberation:1:C1",
				StateVersion: 1,
				Role:         "council",
				Phase:        "deliberation",
				MemberID:     "C1",
			}
			stateBefore := cloneJSONLikeMap(rc.state)
			err := rc.failOpportunityLocked(context.Background(), time.Time{}, opportunity, opportunityFailureAgentExited, "agent exited", map[string]any{"member_id": "C1"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("failure transition error = %v, want %q", err, tc.want)
			}
			assertFailedTransitionUnchanged(t, rc, stateBefore, 0)
		})
	}
}

func TestObserverRoleAPIClassifiesToolAndStorageErrors(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	status, body := callLawyerAPIDo(t, api, map[string]any{
		"case_id":   "arb-1",
		"role_id":   "observer",
		"tool":      "unknown",
		"arguments": map[string]any{},
	})
	if status != http.StatusOK || lawyerAPIErrorCode(t, body) != "tool_failed" {
		t.Fatalf("unknown tool status = %d, response = %#v", status, body)
	}

	rc, operations, handler, request := setupBlockedObserverEvidenceRequest(t, true)
	wantErr := errors.New("observer evidence verification failed")
	operations.verify = func(evidenceFileSnapshot) error { return wantErr }
	body, err := runAPIRequest(handler, request)
	if err != nil {
		t.Fatal(err)
	}
	if code := mapString(mapAny(body["error"])["code"]); code != "runtime_failure" {
		t.Fatalf("observer storage response = %#v", body)
	}
	if message := mapString(mapAny(body["error"])["message"]); message != wantErr.Error() {
		t.Fatalf("observer storage error = %q, want %q", message, wantErr)
	}
	if turn.completed || turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("observer errors changed active lawyer turn: %#v", turn)
	}
	rc.mu.Lock()
	storageTurn := rc.lawyerAPI.active
	storageCompleted := storageTurn.completed
	storageAttempts := storageTurn.attemptsRemaining
	rc.mu.Unlock()
	if storageCompleted || storageAttempts != storageTurn.attemptsMax {
		t.Fatalf("observer storage error changed active lawyer turn: %#v", storageTurn)
	}
}

func TestLawyerDoLimitsRequestBodyWithoutConsumingAttempt(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.Runtime.MaxResponseBytes = 64
	requestBodyLimit, err := lawyerDoRequestBodyLimit(
		int64(api.rc.cfg.Runtime.MaxResponseBytes),
		int64(api.rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes),
	)
	if err != nil {
		t.Fatal(err)
	}
	status, body := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arb-1",
		"role_id":        "plaintiff",
		"opportunity_id": turn.opportunity.ID,
		"tool":           "send_work_notes",
		"arguments":      map[string]any{"notes": strings.Repeat("x", int(requestBodyLimit)+1)},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if code := lawyerAPIErrorCode(t, body); code != "bad_json" {
		t.Fatalf("error code = %q, want bad_json", code)
	}
	if turn.completed || turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("oversize request changed turn: %#v", turn)
	}
}

func TestLawyerDoAllowsMaximumDirectEvidenceBody(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.Runtime.MaxResponseBytes = 512
	api.rc.cfg.Policy.MaxSubmittedEvidencePerSide = 0
	turn.opportunity = Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		AllowedTools: []string{"submit_evidence"},
	}
	content := make([]byte, api.rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes)
	status, body := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arb-1",
		"role_id":        "plaintiff",
		"opportunity_id": turn.opportunity.ID,
		"tool":           "submit_evidence",
		"arguments": map[string]any{
			"title":              "Source",
			"mime_type":          "application/octet-stream",
			"source_description": "Maximum-size direct evidence test.",
			"relevance":          "Exercises request parsing.",
			"content_base64":     base64.StdEncoding.EncodeToString(content),
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d; response = %#v", status, http.StatusOK, body)
	}
	if code := lawyerAPIErrorCode(t, body); code != "tool_failed" {
		t.Fatalf("error code = %q, want tool_failed; response = %#v", code, body)
	}
	if message := mapString(mapAny(body["error"])["message"]); !strings.Contains(message, "submitted_evidence for this side exceed limit of 0") {
		t.Fatalf("error message = %q, want submission-count validation", message)
	}
	if turn.completed || turn.attemptsRemaining != turn.attemptsMax-1 {
		t.Fatalf("validated request changed turn incorrectly: %#v", turn)
	}
}

func TestLawyerDoRequestBodyLimitRejectsOverflow(t *testing.T) {
	const maxInt64 = int64(1<<63 - 1)
	_, err := lawyerDoRequestBodyLimit(1, maxInt64)
	if err == nil || !strings.Contains(err.Error(), "request body limit exceeds the maximum supported size") {
		t.Fatalf("lawyerDoRequestBodyLimit error = %v, want overflow error", err)
	}
}

func assertParticipantInputResponse(t *testing.T, body map[string]any, attemptsRemaining int, done <-chan error) {
	t.Helper()
	if body["ok"] != false || mapString(mapAny(body["error"])["code"]) != "tool_failed" {
		t.Fatalf("participant-input response = %#v", body)
	}
	turn := mapAny(body["turn"])
	if turn["completed"] != false || intNumber(turn["attempts_remaining"]) != attemptsRemaining {
		t.Fatalf("participant-input turn = %#v, want incomplete with %d attempts", turn, attemptsRemaining)
	}
	select {
	case err := <-done:
		t.Fatalf("participant input completed turn with %v", err)
	default:
	}
}

func assertRuntimeFailureResponse(t *testing.T, body map[string]any, attemptsRemaining int, done <-chan error) error {
	t.Helper()
	if body["ok"] != false || mapString(mapAny(body["error"])["code"]) != "runtime_failure" {
		t.Fatalf("runtime response = %#v", body)
	}
	turn := mapAny(body["turn"])
	if turn["completed"] != true || intNumber(turn["attempts_remaining"]) != attemptsRemaining {
		t.Fatalf("runtime turn = %#v, want completed with %d attempts", turn, attemptsRemaining)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("runtime failure completed turn with nil error")
		}
		if message := mapString(mapAny(body["error"])["message"]); message != err.Error() {
			t.Fatalf("response error = %q, done error = %q", message, err)
		}
		return err
	default:
		t.Fatal("runtime failure did not publish a turn result")
		return nil
	}
}

func roleAPITestEngine(response string) lean.Engine {
	return lean.New([]string{"/bin/sh", "-c", `cat >/dev/null; printf '%s\n' "$1"`, "role-api-test", response})
}

func assertFailedTransitionUnchanged(t *testing.T, rc *runContext, stateBefore map[string]any, actionsBefore int) {
	t.Helper()
	if !reflect.DeepEqual(rc.state, stateBefore) {
		t.Fatalf("failure transition changed state: got %#v, want %#v", rc.state, stateBefore)
	}
	if len(rc.certificateActions) != actionsBefore {
		t.Fatalf("certificate actions = %d, want %d", len(rc.certificateActions), actionsBefore)
	}
	if len(rc.events) != 0 {
		t.Fatalf("failure transition recorded %d events, want 0", len(rc.events))
	}
}
