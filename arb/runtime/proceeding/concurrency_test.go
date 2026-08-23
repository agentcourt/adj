package proceeding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentcourt/adj/arb/runtime/lean"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

func TestRoleAPIWaitersShareCaseNotification(t *testing.T) {
	rc, lawyerAPI, councilAPI := testRoleAPIs(t)
	rc.mu.Lock()
	councilAPI.active = nil
	rc.mu.Unlock()
	type response struct {
		body map[string]any
		err  error
	}
	lawyerDone := make(chan response, 1)
	councilDone := make(chan response, 1)
	go func() {
		body, err := runAPIRequest(lawyerAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=observer&after_version=1&timeout_ms=1000",
			nil,
		))
		lawyerDone <- response{body: body, err: err}
	}()
	go func() {
		body, err := runAPIRequest(councilAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			councilAPIBasePath+"/wait?case_id=arb-1&member_id=C1&after_version=1&timeout_ms=1000",
			nil,
		))
		councilDone <- response{body: body, err: err}
	}()

	rc.mu.Lock()
	rc.state["state_version"] = 2
	rc.signalRoleAPIsLocked()
	rc.mu.Unlock()

	for name, done := range map[string]<-chan response{
		"lawyer":  lawyerDone,
		"council": councilDone,
	} {
		select {
		case got := <-done:
			if got.err != nil {
				t.Fatalf("%s wait: %v", name, got.err)
			}
			wait := mapAny(got.body["wait"])
			if wait["reason"] != "changed" || intNumber(wait["state_version"]) != 2 {
				t.Fatalf("%s wait = %#v, want changed state version 2", name, got.body["wait"])
			}
		case <-time.After(time.Second):
			t.Fatalf("%s wait did not receive the shared notification", name)
		}
	}
}

func TestRoleAPIsPublishTerminalStateTogether(t *testing.T) {
	rc, lawyerAPI, councilAPI := testRoleAPIs(t)
	rc.mu.Lock()
	councilAPI.active = nil
	rc.mu.Unlock()
	type response struct {
		body map[string]any
		err  error
	}
	lawyerDone := make(chan response, 1)
	councilDone := make(chan response, 1)
	go func() {
		body, err := runAPIRequest(lawyerAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=observer&after_version=1&timeout_ms=1000",
			nil,
		))
		lawyerDone <- response{body: body, err: err}
	}()
	go func() {
		body, err := runAPIRequest(councilAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			councilAPIBasePath+"/wait?case_id=arb-1&member_id=C1&after_version=1&timeout_ms=1000",
			nil,
		))
		councilDone <- response{body: body, err: err}
	}()

	rc.mu.Lock()
	rc.setRoleAPIsTerminalLocked("closed")
	rc.mu.Unlock()
	for name, done := range map[string]<-chan response{
		"lawyer":  lawyerDone,
		"council": councilDone,
	} {
		select {
		case got := <-done:
			if got.err != nil {
				t.Fatalf("%s terminal wait: %v", name, got.err)
			}
			wait := mapAny(got.body["wait"])
			if got.body["status"] != "done" || got.body["final_reason"] != "closed" || wait["reason"] != "done" {
				t.Fatalf("%s terminal response = %#v", name, got.body)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s terminal wait did not finish", name)
		}
	}
}

func TestLawyerActionPublishesCouncilAPIChange(t *testing.T) {
	lawyerAPI := testLawyerDecisionAPI(t)
	rc := lawyerAPI.rc
	councilAPI := newCouncilAPIServer(rc)
	councilAPI.version = 1
	rc.mu.Lock()
	rc.councilAPI = councilAPI
	rc.mu.Unlock()
	waitDone := make(chan map[string]any, 1)
	waitErr := make(chan error, 1)
	go func() {
		body, err := runAPIRequest(councilAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			councilAPIBasePath+"/wait?case_id=arb-1&member_id=C1&after_version=1&timeout_ms=1000",
			nil,
		))
		if err != nil {
			waitErr <- err
			return
		}
		waitDone <- body
	}()
	body, err := runAPIRequest(lawyerDoHandler(context.Background(), lawyerAPI), lawyerDecisionRequest())
	if err != nil {
		t.Fatalf("lawyer action: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("lawyer action response = %#v", body)
	}
	assertChangedWait(t, "council", waitDone, waitErr)
}

func TestCouncilActionPublishesLawyerAPIChange(t *testing.T) {
	_, lawyerAPI, councilAPI := testRoleAPIs(t)
	waitDone := make(chan map[string]any, 1)
	waitErr := make(chan error, 1)
	go func() {
		body, err := runAPIRequest(lawyerAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=observer&after_version=1&timeout_ms=1000",
			nil,
		))
		if err != nil {
			waitErr <- err
			return
		}
		waitDone <- body
	}()
	raw, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"tool":           "submit_council_vote",
		"arguments": map[string]any{
			"vote":      "demonstrated",
			"rationale": "record sufficient",
		},
	})
	if err != nil {
		t.Fatalf("marshal council action: %v", err)
	}
	body, err := runAPIRequest(councilAPI.handleDo, httptest.NewRequest(http.MethodPost, councilAPIBasePath+"/do", bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("council action: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("council action response = %#v", body)
	}
	assertChangedWait(t, "lawyer", waitDone, waitErr)
}

func TestCouncilFailurePublishesLawyerAPIChange(t *testing.T) {
	_, lawyerAPI, councilAPI := testRoleAPIs(t)
	waitDone := make(chan map[string]any, 1)
	waitErr := make(chan error, 1)
	go func() {
		body, err := runAPIRequest(lawyerAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=observer&after_version=1&timeout_ms=1000",
			nil,
		))
		if err != nil {
			waitErr <- err
			return
		}
		waitDone <- body
	}()
	raw, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"message":        "agent exited",
	})
	if err != nil {
		t.Fatalf("marshal council failure: %v", err)
	}
	body, err := runAPIRequest(councilAPI.handleFail, httptest.NewRequest(http.MethodPost, councilAPIBasePath+"/fail", bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("council failure: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("council failure response = %#v", body)
	}
	assertChangedWait(t, "lawyer", waitDone, waitErr)
}

func TestRoleAPIObserversReadWhileCaseChanges(t *testing.T) {
	rc, lawyerAPI, councilAPI := testRoleAPIs(t)
	opening := map[string]any{"role": "plaintiff", "text": "initial"}
	eventDetails := map[string]any{"value": "initial"}
	rc.mu.Lock()
	mapAny(rc.state["case"])["openings"] = []map[string]any{opening}
	rc.events = append(rc.events, Event{Turn: 1, Type: "initial", Payload: map[string]any{"details": eventDetails}})
	rc.mu.Unlock()
	lawyerBody, err := json.Marshal(map[string]any{
		"case_id": "arb-1",
		"role_id": "observer",
		"tool":    "get_case",
	})
	if err != nil {
		t.Fatalf("marshal lawyer request: %v", err)
	}
	lawyerRoleBody, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"role_id":        "plaintiff",
		"opportunity_id": "openings:plaintiff",
		"tool":           "get_case",
	})
	if err != nil {
		t.Fatalf("marshal active lawyer request: %v", err)
	}
	councilBody, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"tool":           "get_case",
	})
	if err != nil {
		t.Fatalf("marshal council request: %v", err)
	}
	eventsBody, err := json.Marshal(map[string]any{
		"case_id": "arb-1",
		"role_id": "observer",
		"tool":    "list_events",
		"arguments": map[string]any{
			"limit": 1000,
		},
	})
	if err != nil {
		t.Fatalf("marshal event request: %v", err)
	}

	errCh := make(chan error, 4)
	go func() {
		for i := 0; i < 100; i++ {
			for _, requestBody := range [][]byte{lawyerBody, lawyerRoleBody} {
				req := httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(requestBody))
				body, err := runAPIRequest(lawyerDoHandler(context.Background(), lawyerAPI), req)
				if err != nil {
					errCh <- err
					return
				}
				if body["ok"] != true {
					errCh <- fmt.Errorf("lawyer case response: %#v", body)
					return
				}
			}
		}
		errCh <- nil
	}()
	go func() {
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest(http.MethodPost, councilAPIBasePath+"/do", bytes.NewReader(councilBody))
			body, err := runAPIRequest(councilAPI.handleDo, req)
			if err != nil {
				errCh <- err
				return
			}
			if body["ok"] != true {
				errCh <- fmt.Errorf("council observer response: %#v", body)
				return
			}
		}
		errCh <- nil
	}()
	go func() {
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(eventsBody))
			body, err := runAPIRequest(lawyerDoHandler(context.Background(), lawyerAPI), req)
			if err != nil {
				errCh <- err
				return
			}
			if body["ok"] != true {
				errCh <- fmt.Errorf("lawyer event response: %#v", body)
				return
			}
		}
		errCh <- nil
	}()
	go func() {
		for i := 0; i < 100; i++ {
			rc.mu.Lock()
			rc.state["state_version"] = i + 2
			opening["text"] = fmt.Sprintf("opening %d", i)
			eventDetails["value"] = fmt.Sprintf("event %d", i)
			rc.council[0].Model = fmt.Sprintf("model-%d", i)
			rc.events = append(rc.events, Event{Turn: i + 1, Type: "case_changed"})
			rc.signalRoleAPIsLocked()
			rc.mu.Unlock()
		}
		errCh <- nil
	}()

	for i := 0; i < 4; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

func TestRoleAPIHTTPWritesDoNotHoldCaseMutex(t *testing.T) {
	_, lawyerAPI, councilAPI := testRoleAPIs(t)
	lawyerRequest, err := json.Marshal(map[string]any{
		"case_id": "arb-1",
		"role_id": "observer",
		"tool":    "get_case",
	})
	if err != nil {
		t.Fatalf("marshal lawyer request: %v", err)
	}
	councilRequest, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"message":        "agent exited",
	})
	if err != nil {
		t.Fatalf("marshal council request: %v", err)
	}
	tests := []struct {
		name    string
		rc      *runContext
		handler http.HandlerFunc
		request *http.Request
	}{
		{
			name:    "lawyer do",
			rc:      lawyerAPI.rc,
			handler: lawyerDoHandler(context.Background(), lawyerAPI),
			request: httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(lawyerRequest)),
		},
		{
			name:    "council fail",
			rc:      councilAPI.rc,
			handler: councilAPI.handleFail,
			request: httptest.NewRequest(http.MethodPost, councilAPIBasePath+"/fail", bytes.NewReader(councilRequest)),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writer := newBlockingResponseWriter()
			done := make(chan struct{})
			go func() {
				tc.handler(writer, tc.request)
				close(done)
			}()
			select {
			case <-writer.entered:
			case <-time.After(time.Second):
				t.Fatal("HTTP response write did not begin")
			}
			acquired := make(chan struct{})
			go func() {
				tc.rc.mu.Lock()
				tc.rc.mu.Unlock()
				close(acquired)
			}()
			select {
			case <-acquired:
			case <-time.After(time.Second):
				close(writer.release)
				t.Fatal("HTTP response write held the case mutex")
			}
			close(writer.release)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("HTTP handler did not finish")
			}
		})
	}
}

func TestDirectCouncilProviderCallDoesNotHoldCaseMutex(t *testing.T) {
	rc := newCouncilOpportunityTestContext(t, "")
	rc.cfg.PromptDir = testPromptDir()
	client := &blockingCouncilResponseClient{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	done := make(chan error, 1)
	go func() {
		done <- rc.executeCouncilOpportunity(context.Background(), client, Opportunity{
			ID:           "deliberation:1:C1",
			StateVersion: 1,
			Role:         "council",
			Phase:        "deliberation",
			MemberID:     "C1",
		})
	}()
	select {
	case <-client.entered:
	case <-time.After(time.Second):
		t.Fatal("direct council provider call did not begin")
	}
	acquired := make(chan struct{})
	go func() {
		rc.mu.Lock()
		rc.mu.Unlock()
		close(acquired)
	}()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		close(client.release)
		t.Fatal("direct council provider call held the case mutex")
	}
	close(client.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("execute council opportunity: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("direct council opportunity did not finish")
	}
	rc.mu.Lock()
	stateVersion := intNumber(rc.state["state_version"])
	actionCount := len(rc.certificateActions)
	events := append([]Event(nil), rc.events...)
	rc.mu.Unlock()
	if stateVersion != 2 || actionCount != 1 || len(events) != 1 || events[0].Type != "council_vote" {
		t.Fatalf("accepted council commit: state_version=%d actions=%d events=%#v", stateVersion, actionCount, events)
	}
}

func TestConcurrentDuplicateLawyerDecisionCommitsOnce(t *testing.T) {
	api := testLawyerDecisionAPI(t)
	responses := make(chan map[string]any, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerDecisionRequest())
			if err != nil {
				errs <- err
				return
			}
			responses <- body
		}()
	}
	successes := 0
	failures := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			t.Fatal(err)
		case body := <-responses:
			if body["ok"] == true {
				successes++
			} else {
				failures++
			}
		case <-time.After(time.Second):
			t.Fatal("duplicate lawyer decision did not finish")
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("duplicate decision results: %d successes, %d failures", successes, failures)
	}
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	if len(api.rc.certificateActions) != 1 || len(api.rc.events) != 1 || !api.active.completed {
		t.Fatalf("duplicate decision state: actions=%d events=%d completed=%v", len(api.rc.certificateActions), len(api.rc.events), api.active.completed)
	}
}

func TestBlockedLawyerActionExpiresTurnAndCommitsDeadlineFailure(t *testing.T) {
	api := testLawyerDecisionAPI(t)
	enteredPath, releasePath, callsPath := configureBlockingLawyerEngine(t, api)
	api.active.deadline = time.Now().Add(150 * time.Millisecond)
	defer releaseBlockingEngine(t, releasePath)

	done := make(chan apiCallResult, 1)
	go func() {
		body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerDecisionRequest())
		done <- apiCallResult{body: body, err: err}
	}()
	waitForTestPath(t, enteredPath)

	var result apiCallResult
	select {
	case result = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked lawyer action did not finish after its deadline")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if code := mapString(mapAny(result.body["error"])["code"]); code != "turn_timeout" {
		t.Fatalf("error code = %q, want turn_timeout: %#v", code, result.body)
	}
	api.rc.mu.Lock()
	failure := caseFailure(api.rc.state)
	actions := append([]ReplayAction(nil), api.rc.certificateActions...)
	completed := api.active.completed
	api.rc.mu.Unlock()
	if mapString(failure["reason"]) != opportunityFailureDeadline {
		t.Fatalf("case failure = %#v, want deadline_expired", failure)
	}
	if !completed {
		t.Fatal("deadline failure left the lawyer turn active")
	}
	if len(actions) != 1 || actions[0].ActionType != "fail_opportunity" {
		t.Fatalf("certificate actions = %#v, want one fail_opportunity", actions)
	}
	if calls := readTestLines(t, callsPath); len(calls) != 2 || calls[0] != "record_opening_statement" || calls[1] != "fail_opportunity" {
		t.Fatalf("engine calls = %#v, want action then one deadline failure", calls)
	}
}

func TestCaseCancellationStopsBlockedLawyerAction(t *testing.T) {
	api := testLawyerDecisionAPI(t)
	enteredPath, releasePath, callsPath := configureBlockingLawyerEngine(t, api)
	api.active.deadline = time.Now().Add(5 * time.Second)
	defer releaseBlockingEngine(t, releasePath)
	caseCtx, cancelCase := context.WithCancelCause(context.Background())
	wantErr := errors.New("case canceled during lawyer action")

	done := make(chan apiCallResult, 1)
	go func() {
		body, err := runAPIRequest(lawyerDoHandler(caseCtx, api), lawyerDecisionRequest())
		done <- apiCallResult{body: body, err: err}
	}()
	waitForTestPath(t, enteredPath)
	cancelCase(wantErr)

	var result apiCallResult
	select {
	case result = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked lawyer action did not finish after case cancellation")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if code := mapString(mapAny(result.body["error"])["code"]); code != "runtime_failure" {
		t.Fatalf("error code = %q, want runtime_failure: %#v", code, result.body)
	}
	if message := mapString(mapAny(result.body["error"])["message"]); !strings.Contains(message, wantErr.Error()) {
		t.Fatalf("error message = %q, want custom cancellation cause", message)
	}
	api.rc.mu.Lock()
	stateVersion, err := requiredStateVersion(api.rc.state)
	actionCount := len(api.rc.certificateActions)
	completed := api.active.completed
	api.rc.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if stateVersion != 0 || actionCount != 0 || !completed {
		t.Fatalf("canceled action state_version=%d actions=%d completed=%t, want 0, 0, true", stateVersion, actionCount, completed)
	}
	if err := <-api.active.done; !errors.Is(err, wantErr) {
		t.Fatalf("turn error = %v, want %v", err, wantErr)
	}
	if calls := readTestLines(t, callsPath); len(calls) != 1 || calls[0] != "record_opening_statement" {
		t.Fatalf("engine calls = %#v, want one canceled action", calls)
	}
}

func TestClientDisconnectDoesNotCancelValidatedLawyerAction(t *testing.T) {
	api := testLawyerDecisionAPI(t)
	enteredPath, releasePath, callsPath := configureBlockingLawyerEngine(t, api)
	api.active.deadline = time.Now().Add(5 * time.Second)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	request := lawyerDecisionRequest().WithContext(requestCtx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		api.handleDo(context.Background(), recorder, request)
		close(done)
	}()
	waitForTestPath(t, enteredPath)
	cancelRequest()
	releaseBlockingEngine(t, releasePath)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("validated lawyer action stopped after client disconnect")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("response body after client disconnect = %q, want empty", recorder.Body.String())
	}
	api.rc.mu.Lock()
	stateVersion, err := requiredStateVersion(api.rc.state)
	actions := append([]ReplayAction(nil), api.rc.certificateActions...)
	completed := api.active.completed
	api.rc.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if stateVersion != 1 || len(actions) != 1 || actions[0].ActionType != "record_opening_statement" || !completed {
		t.Fatalf("disconnected action state_version=%d actions=%#v completed=%t", stateVersion, actions, completed)
	}
	if calls := readTestLines(t, callsPath); len(calls) != 1 || calls[0] != "record_opening_statement" {
		t.Fatalf("engine calls = %#v, want one completed action", calls)
	}
}

func TestLawyerEngineTimeoutReleasesCaseMutex(t *testing.T) {
	api := testLawyerDecisionAPI(t)
	enteredPath, releasePath, callsPath := configureBlockingLawyerEngine(t, api)
	api.rc.cfg.Runtime.EngineCallTimeoutSeconds = 1
	api.active.deadline = time.Now().Add(5 * time.Second)
	defer releaseBlockingEngine(t, releasePath)

	done := make(chan apiCallResult, 1)
	go func() {
		body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerDecisionRequest())
		done <- apiCallResult{body: body, err: err}
	}()
	waitForTestPath(t, enteredPath)

	var result apiCallResult
	select {
	case result = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked lawyer action did not finish after the engine timeout")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if code := mapString(mapAny(result.body["error"])["code"]); code != "runtime_failure" {
		t.Fatalf("error code = %q, want runtime_failure: %#v", code, result.body)
	}
	acquired := make(chan struct{})
	go func() {
		api.rc.mu.Lock()
		api.rc.mu.Unlock()
		close(acquired)
	}()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("case mutex remained locked after the bounded engine call returned")
	}
	if calls := readTestLines(t, callsPath); len(calls) != 1 || calls[0] != "record_opening_statement" {
		t.Fatalf("engine calls = %#v, want one timed-out action", calls)
	}
}

func TestConcurrentSameOffsetEvidenceChunksWriteOnce(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.OutputDir = t.TempDir()
	caseObj := mapAny(api.rc.state["case"])
	caseObj["phase"] = "arguments"
	turn.opportunity = Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		AllowedTools: []string{"write_evidence_chunk"},
	}
	api.rc.mu.Lock()
	session, err := api.rc.beginEvidenceUpload(turn.opportunity, map[string]any{
		"title":               "Concurrent upload",
		"mime_type":           "text/plain",
		"expected_size_bytes": 3,
		"source_description":  "test",
		"relevance":           "test",
	})
	api.rc.mu.Unlock()
	if err != nil {
		t.Fatalf("begin upload: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"role_id":        "plaintiff",
		"opportunity_id": "arguments:plaintiff",
		"tool":           "write_evidence_chunk",
		"arguments": map[string]any{
			"upload_id":      session.UploadID,
			"offset":         0,
			"content_base64": "YWJj",
		},
	})
	if err != nil {
		t.Fatalf("marshal chunk request: %v", err)
	}
	responses := make(chan map[string]any, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(raw)))
			if err != nil {
				errs <- err
				return
			}
			responses <- body
		}()
	}
	successes := 0
	failures := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			t.Fatal(err)
		case body := <-responses:
			if body["ok"] == true {
				successes++
			} else {
				failures++
			}
		case <-time.After(time.Second):
			t.Fatal("same-offset evidence chunks did not finish")
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("same-offset chunk results: %d successes, %d failures", successes, failures)
	}
	rawFile, err := os.ReadFile(session.Path)
	if err != nil {
		t.Fatalf("read upload file: %v", err)
	}
	api.rc.mu.Lock()
	received := session.ReceivedBytes
	api.rc.mu.Unlock()
	if string(rawFile) != "abc" || received != 3 {
		t.Fatalf("upload content=%q received=%d, want abc and 3", rawFile, received)
	}
}

func TestRoleAPIErrorResponsesReflectCompletedTransitions(t *testing.T) {
	t.Run("lawyer attempts exhausted", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		configureLawyerFailureEngine(t, api)
		turn.attemptsMax = 1
		turn.attemptsRemaining = 1
		body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, "plaintiff", "openings:plaintiff", "unknown", nil))
		if err != nil {
			t.Fatal(err)
		}
		assertCompletedAttemptResponse(t, body, 0)
		if got := mapString(mapAny(api.rc.state["case"])["status"]); got != "failed" {
			t.Fatalf("case status = %q, want failed", got)
		}
	})

	t.Run("lawyer deadline", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		configureLawyerFailureEngine(t, api)
		turn.deadline = time.Now().Add(-time.Second)
		body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, "plaintiff", "openings:plaintiff", "get_case", nil))
		if err != nil {
			t.Fatal(err)
		}
		assertCompletedAttemptResponse(t, body, turn.attemptsMax)
		if code := mapString(mapAny(body["error"])["code"]); code != "turn_timeout" {
			t.Fatalf("error code = %q, want turn_timeout", code)
		}
	})

	t.Run("council attempts exhausted", func(t *testing.T) {
		_, _, api := testRoleAPIs(t)
		api.active.attemptsMax = 1
		api.active.attemptsRemaining = 1
		body, err := runAPIRequest(api.handleDo, councilToolRequest(t, "unknown", nil))
		if err != nil {
			t.Fatal(err)
		}
		assertCompletedAttemptResponse(t, body, 0)
	})

	t.Run("council deadline", func(t *testing.T) {
		_, _, api := testRoleAPIs(t)
		api.active.deadline = time.Now().Add(-time.Second)
		body, err := runAPIRequest(api.handleDo, councilToolRequest(t, "get_case", nil))
		if err != nil {
			t.Fatal(err)
		}
		assertCompletedAttemptResponse(t, body, api.active.attemptsMax)
		if code := mapString(mapAny(body["error"])["code"]); code != "turn_timeout" {
			t.Fatalf("error code = %q, want turn_timeout", code)
		}
	})
}

func TestAcceptedClosingCouncilVoteMakesBothAPIsTerminal(t *testing.T) {
	rc, lawyerAPI, councilAPI := testRoleAPIs(t)
	enginePath := filepath.Join(rc.cfg.OutputDir, "closed-engine.sh")
	engineScript := `#!/bin/sh
cat >/dev/null
printf '%s\n' '{"ok":true,"state":{"case":{"status":"closed","phase":"closed","resolution":"demonstrated","deliberation_round":1,"council_members":[{"member_id":"C1","status":"seated"}],"council_votes":[{"round":1,"member_id":"C1","vote":"demonstrated","rationale":"record sufficient"}]},"state_version":2}}'
`
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write closed engine: %v", err)
	}
	rc.cfg.Engine = lean.New([]string{enginePath})

	type response struct {
		body map[string]any
		err  error
	}
	lawyerDone := make(chan response, 1)
	councilDone := make(chan response, 1)
	go func() {
		body, err := runAPIRequest(lawyerAPI.handleWait, httptest.NewRequest(http.MethodGet, lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=observer&after_version=1&timeout_ms=1000", nil))
		lawyerDone <- response{body: body, err: err}
	}()
	go func() {
		body, err := runAPIRequest(councilAPI.handleWait, httptest.NewRequest(http.MethodGet, councilAPIBasePath+"/wait?case_id=arb-1&member_id=C1&after=deliberation:1:C1&after_version=1&timeout_ms=1000", nil))
		councilDone <- response{body: body, err: err}
	}()

	body, err := runAPIRequest(councilAPI.handleDo, councilToolRequest(t, "submit_council_vote", map[string]any{
		"vote":      "demonstrated",
		"rationale": "record sufficient",
	}))
	if err != nil {
		t.Fatalf("submit closing council vote: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("vote response = %#v", body)
	}
	rc.mu.Lock()
	if rc.terminal {
		rc.mu.Unlock()
		t.Fatal("run loop terminal flags were set during the accepted vote")
	}
	rc.mu.Unlock()

	for name, done := range map[string]<-chan response{"lawyer": lawyerDone, "council": councilDone} {
		select {
		case got := <-done:
			if got.err != nil {
				t.Fatalf("%s wait: %v", name, got.err)
			}
			if got.body["status"] != "done" || mapString(mapAny(got.body["wait"])["reason"]) != "done" {
				t.Fatalf("%s terminal wait = %#v", name, got.body)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s wait did not report the closed Lean state", name)
		}
	}
}

func TestEvidenceFileIODoesNotHoldCaseMutex(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request)
		stat  bool
	}{
		{name: "lawyer stat", setup: setupBlockedLawyerEvidenceRequest, stat: true},
		{name: "lawyer read", setup: setupBlockedLawyerEvidenceRequest},
		{name: "observer stat", setup: setupBlockedObserverEvidenceRequest, stat: true},
		{name: "observer read", setup: setupBlockedObserverEvidenceRequest},
		{name: "council stat", setup: setupBlockedCouncilEvidenceRequest, stat: true},
		{name: "council read", setup: setupBlockedCouncilEvidenceRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, operations, handler, request := tc.setup(t, tc.stat)
			entered := make(chan struct{})
			release := make(chan struct{})
			if tc.stat {
				operations.verify = func(file evidenceFileSnapshot) error {
					close(entered)
					<-release
					return verifyEvidenceFile(file)
				}
			} else {
				operations.read = func(reservation evidenceReadReservation) (map[string]any, int, error) {
					close(entered)
					<-release
					return readReservedEvidenceRange(reservation)
				}
			}
			done := make(chan map[string]any, 1)
			errCh := make(chan error, 1)
			go func() {
				body, err := runAPIRequest(handler, request)
				if err != nil {
					errCh <- err
					return
				}
				done <- body
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("evidence file operation did not begin")
			}
			acquired := make(chan struct{})
			go func() {
				rc.mu.Lock()
				rc.mu.Unlock()
				close(acquired)
			}()
			select {
			case <-acquired:
			case <-time.After(time.Second):
				close(release)
				t.Fatal("evidence file operation held the case mutex")
			}
			close(release)
			select {
			case err := <-errCh:
				t.Fatal(err)
			case body := <-done:
				if body["ok"] != true {
					t.Fatalf("evidence response = %#v", body)
				}
			case <-time.After(time.Second):
				t.Fatal("evidence request did not finish")
			}
		})
	}
}

func TestBlockedEvidenceReadsDoNotBlockDeadlinesAndRollback(t *testing.T) {
	t.Run("lawyer", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		configureLawyerFailureEngine(t, api)
		meta := installTestEvidence(t, api.rc, []byte("lawyer deadline evidence"))
		entered := make(chan struct{})
		release := make(chan struct{})
		api.evidenceFiles.read = func(reservation evidenceReadReservation) (map[string]any, int, error) {
			close(entered)
			<-release
			return readReservedEvidenceRange(reservation)
		}
		request := lawyerToolRequest(t, "plaintiff", "openings:plaintiff", "read_evidence_range", map[string]any{
			"evidence_id": meta.EvidenceID,
			"offset":      0,
			"length":      4,
		})
		requestDone := make(chan apiCallResult, 1)
		go func() {
			body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), request)
			requestDone <- apiCallResult{body: body, err: err}
		}()
		<-entered
		deadlineDone := make(chan error, 1)
		go func() { deadlineDone <- api.timeoutTurn(context.Background(), turn, time.Second) }()
		select {
		case err := <-deadlineDone:
			if err != nil {
				t.Fatalf("deadline transition: %v", err)
			}
		case <-time.After(time.Second):
			close(release)
			t.Fatal("deadline transition waited for evidence I/O")
		}
		close(release)
		result := <-requestDone
		if result.err != nil {
			t.Fatal(result.err)
		}
		body := result.body
		if code := mapString(mapAny(body["error"])["code"]); code != "stale_opportunity" {
			t.Fatalf("read response = %#v, want stale_opportunity", body)
		}
		if turn.evidenceBudget.reads != 0 || turn.evidenceBudget.bytes != 0 {
			t.Fatalf("stale read budget = %#v, want zero", turn.evidenceBudget)
		}
	})

	t.Run("council", func(t *testing.T) {
		rc, _, api := testRoleAPIs(t)
		turn := api.active
		meta := installTestEvidence(t, rc, []byte("council deadline evidence"))
		entered := make(chan struct{})
		release := make(chan struct{})
		api.evidenceFiles.read = func(reservation evidenceReadReservation) (map[string]any, int, error) {
			close(entered)
			<-release
			return readReservedEvidenceRange(reservation)
		}
		request := councilToolRequest(t, "read_evidence_range", map[string]any{
			"evidence_id": meta.EvidenceID,
			"offset":      0,
			"length":      4,
		})
		requestDone := make(chan apiCallResult, 1)
		go func() {
			body, err := runAPIRequest(api.handleDo, request)
			requestDone <- apiCallResult{body: body, err: err}
		}()
		<-entered
		deadlineDone := make(chan error, 1)
		go func() { deadlineDone <- api.timeoutTurn(turn, time.Second) }()
		select {
		case err := <-deadlineDone:
			if err != nil {
				t.Fatalf("deadline transition: %v", err)
			}
		case <-time.After(time.Second):
			close(release)
			t.Fatal("deadline transition waited for evidence I/O")
		}
		close(release)
		result := <-requestDone
		if result.err != nil {
			t.Fatal(result.err)
		}
		body := result.body
		if code := mapString(mapAny(body["error"])["code"]); code != "stale_opportunity" {
			t.Fatalf("read response = %#v, want stale_opportunity", body)
		}
		if turn.evidenceBudget.reads != 0 || turn.evidenceBudget.bytes != 0 {
			t.Fatalf("stale read budget = %#v, want zero", turn.evidenceBudget)
		}
	})
}

func TestEvidenceReadFailureRollsBackReservation(t *testing.T) {
	wantErr := errors.New("evidence reader failed")
	t.Run("lawyer", func(t *testing.T) {
		rc, operations, handler, request := setupBlockedLawyerEvidenceRequest(t, false)
		turn := rc.lawyerAPI.active
		operations.read = func(evidenceReadReservation) (map[string]any, int, error) {
			return nil, 0, wantErr
		}
		body, err := runAPIRequest(handler, request)
		if err != nil {
			t.Fatal(err)
		}
		if got := mapString(mapAny(body["error"])["message"]); got != wantErr.Error() {
			t.Fatalf("read error = %q, want %q", got, wantErr)
		}
		if err := assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done); !errors.Is(err, wantErr) {
			t.Fatalf("turn error = %v, want %v", err, wantErr)
		}
		rc.mu.Lock()
		budget := turn.evidenceBudget
		rc.mu.Unlock()
		if budget.reads != 0 || budget.bytes != 0 {
			t.Fatalf("failed read budget = %#v, want zero", budget)
		}
	})

	t.Run("council", func(t *testing.T) {
		rc, operations, handler, request := setupBlockedCouncilEvidenceRequest(t, false)
		turn := rc.councilAPI.active
		operations.read = func(evidenceReadReservation) (map[string]any, int, error) {
			return nil, 0, wantErr
		}
		body, err := runAPIRequest(handler, request)
		if err != nil {
			t.Fatal(err)
		}
		if got := mapString(mapAny(body["error"])["message"]); got != wantErr.Error() {
			t.Fatalf("read error = %q, want %q", got, wantErr)
		}
		if err := assertRuntimeFailureResponse(t, body, turn.attemptsMax, turn.done); !errors.Is(err, wantErr) {
			t.Fatalf("turn error = %v, want %v", err, wantErr)
		}
		rc.mu.Lock()
		budget := turn.evidenceBudget
		rc.mu.Unlock()
		if budget.reads != 0 || budget.bytes != 0 {
			t.Fatalf("failed read budget = %#v, want zero", budget)
		}
	})
}

func TestCouncilEvidenceReadStopsOnCaseCancellation(t *testing.T) {
	rc, operations, _, request := setupBlockedCouncilEvidenceRequest(t, false)
	api := rc.councilAPI
	turn := api.active
	caseCtx, cancelCase := context.WithCancelCause(context.Background())
	wantErr := errors.New("case canceled during council evidence read")
	entered := make(chan struct{})
	release := make(chan struct{})
	operations.read = func(reservation evidenceReadReservation) (map[string]any, int, error) {
		close(entered)
		<-release
		return readReservedEvidenceRange(reservation)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.handleDoContext(caseCtx, w, r)
	})
	done := make(chan apiCallResult, 1)
	go func() {
		body, err := runAPIRequest(handler, request)
		done <- apiCallResult{body: body, err: err}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("council evidence read did not begin")
	}
	cancelCase(wantErr)
	close(release)
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	if code := mapString(mapAny(result.body["error"])["code"]); code != "runtime_failure" {
		t.Fatalf("error code = %q, want runtime_failure: %#v", code, result.body)
	}
	if err := assertRuntimeFailureResponse(t, result.body, turn.attemptsMax, turn.done); !errors.Is(err, wantErr) {
		t.Fatalf("turn error = %v, want %v", err, wantErr)
	}
	rc.mu.Lock()
	budget := *turn.evidenceBudget
	events := len(rc.events)
	rc.mu.Unlock()
	if budget.reads != 0 || budget.bytes != 0 || events != 0 {
		t.Fatalf("canceled read budget=%#v events=%d, want zero", budget, events)
	}
}

func TestEvidenceIOErrorRevalidatesTurn(t *testing.T) {
	wantErr := errors.New("evidence file operation failed")
	tests := []struct {
		name       string
		role       string
		stat       bool
		deadline   bool
		setup      func(*testing.T, bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request)
		wantCode   string
		wantBudget bool
	}{
		{name: "lawyer read after completion", role: "lawyer", setup: setupBlockedLawyerEvidenceRequest, wantCode: "stale_opportunity", wantBudget: true},
		{name: "lawyer stat after deadline", role: "lawyer", stat: true, deadline: true, setup: setupBlockedLawyerEvidenceRequest, wantCode: "turn_timeout"},
		{name: "council read after completion", role: "council", setup: setupBlockedCouncilEvidenceRequest, wantCode: "stale_opportunity", wantBudget: true},
		{name: "council stat after deadline", role: "council", stat: true, deadline: true, setup: setupBlockedCouncilEvidenceRequest, wantCode: "turn_timeout"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, operations, handler, request := tc.setup(t, tc.stat)
			if tc.deadline && tc.role == "lawyer" {
				configureLawyerFailureEngine(t, rc.lawyerAPI)
			}
			entered := make(chan struct{})
			release := make(chan struct{})
			if tc.stat {
				operations.verify = func(evidenceFileSnapshot) error {
					close(entered)
					<-release
					return wantErr
				}
			} else {
				operations.read = func(evidenceReadReservation) (map[string]any, int, error) {
					close(entered)
					<-release
					return nil, 0, wantErr
				}
			}
			done := make(chan apiCallResult, 1)
			go func() {
				body, err := runAPIRequest(handler, request)
				done <- apiCallResult{body: body, err: err}
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("evidence file operation did not begin")
			}
			if tc.role == "lawyer" {
				turn := rc.lawyerAPI.active
				if tc.deadline {
					rc.mu.Lock()
					turn.deadline = time.Now().Add(-time.Second)
					rc.mu.Unlock()
				} else {
					rc.lawyerAPI.finishTurn(turn, nil)
				}
			} else {
				turn := rc.councilAPI.active
				if tc.deadline {
					rc.mu.Lock()
					turn.deadline = time.Now().Add(-time.Second)
					rc.mu.Unlock()
				} else {
					rc.councilAPI.finishTurn(turn, nil)
				}
			}
			close(release)
			select {
			case result := <-done:
				if result.err != nil {
					t.Fatal(result.err)
				}
				if code := mapString(mapAny(result.body["error"])["code"]); code != tc.wantCode {
					t.Fatalf("response = %#v, want error code %q", result.body, tc.wantCode)
				}
			case <-time.After(time.Second):
				t.Fatal("evidence request did not finish")
			}
			if tc.wantBudget {
				rc.mu.Lock()
				var budget *evidenceReadBudget
				if tc.role == "lawyer" {
					budget = rc.lawyerAPI.active.evidenceBudget
				} else {
					budget = rc.councilAPI.active.evidenceBudget
				}
				reads, bytes := budget.reads, budget.bytes
				rc.mu.Unlock()
				if reads != 0 || bytes != 0 {
					t.Fatalf("stale read budget = %d reads and %d bytes, want zero", reads, bytes)
				}
			}
		})
	}
}

func TestEvidenceReadEventFailurePublishesCommittedReadAndCompletesTurn(t *testing.T) {
	for _, role := range []string{"lawyer", "council"} {
		t.Run(role, func(t *testing.T) {
			rc, lawyerAPI, councilAPI := testRoleAPIs(t)
			meta := installTestEvidence(t, rc, []byte(role+" event failure evidence"))
			badOutput := filepath.Join(t.TempDir(), "output-file")
			if err := os.WriteFile(badOutput, []byte("not a directory"), 0o644); err != nil {
				t.Fatalf("write output blocker: %v", err)
			}
			rc.mu.Lock()
			rc.cfg.OutputDir = badOutput
			lawyerVersion := lawyerAPI.version
			councilVersion := councilAPI.version
			eventCount := len(rc.events)
			rc.mu.Unlock()

			arguments := map[string]any{"evidence_id": meta.EvidenceID, "offset": 0, "length": 4}
			var body map[string]any
			var err error
			if role == "lawyer" {
				body, err = runAPIRequest(lawyerDoHandler(context.Background(), lawyerAPI), lawyerToolRequest(t, "plaintiff", "openings:plaintiff", "read_evidence_range", arguments))
			} else {
				body, err = runAPIRequest(councilAPI.handleDo, councilToolRequest(t, "read_evidence_range", arguments))
			}
			if err != nil {
				t.Fatal(err)
			}
			var attempts int
			var done <-chan error
			if role == "lawyer" {
				attempts = lawyerAPI.active.attemptsMax
				done = lawyerAPI.active.done
			} else {
				attempts = councilAPI.active.attemptsMax
				done = councilAPI.active.done
			}
			assertRuntimeFailureResponse(t, body, attempts, done)

			rc.mu.Lock()
			gotLawyerVersion := lawyerAPI.version
			gotCouncilVersion := councilAPI.version
			gotEventCount := len(rc.events)
			var budget *evidenceReadBudget
			if role == "lawyer" {
				budget = lawyerAPI.active.evidenceBudget
			} else {
				budget = councilAPI.active.evidenceBudget
			}
			reads, bytes := budget.reads, budget.bytes
			rc.mu.Unlock()
			if gotLawyerVersion != lawyerVersion+1 || gotCouncilVersion != councilVersion+1 {
				t.Fatalf("versions = lawyer %d, council %d; want %d and %d", gotLawyerVersion, gotCouncilVersion, lawyerVersion+1, councilVersion+1)
			}
			if gotEventCount != eventCount+1 {
				t.Fatalf("in-memory event count = %d, want %d", gotEventCount, eventCount+1)
			}
			if reads != 1 || bytes != 4 {
				t.Fatalf("committed read budget = %d reads and %d bytes, want 1 and 4", reads, bytes)
			}
		})
	}
}

func TestConcurrentEvidenceReadsRespectOpportunityBudget(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request)
	}{
		{name: "lawyer", setup: setupBlockedLawyerEvidenceRequest},
		{name: "council", setup: setupBlockedCouncilEvidenceRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, operations, handler, firstRequest := tc.setup(t, false)
			rc.mu.Lock()
			rc.cfg.Policy.MaxEvidenceReadsPerOpportunity = 1
			rc.mu.Unlock()
			entered := make(chan struct{})
			release := make(chan struct{})
			operations.read = func(reservation evidenceReadReservation) (map[string]any, int, error) {
				close(entered)
				<-release
				return readReservedEvidenceRange(reservation)
			}
			firstDone := make(chan apiCallResult, 1)
			go func() {
				body, err := runAPIRequest(handler, firstRequest)
				firstDone <- apiCallResult{body: body, err: err}
			}()
			<-entered
			rc.mu.Lock()
			evidenceID := rc.evidence[0].EvidenceID
			var secondRequest *http.Request
			if tc.name == "lawyer" {
				turn := rc.lawyerAPI.active
				secondRequest = lawyerToolRequest(t, turn.opportunity.Role, turn.opportunity.ID, "read_evidence_range", map[string]any{
					"evidence_id": evidenceID,
					"offset":      0,
					"length":      4,
				})
			} else {
				secondRequest = councilToolRequest(t, "read_evidence_range", map[string]any{
					"evidence_id": evidenceID,
					"offset":      0,
					"length":      4,
				})
			}
			rc.mu.Unlock()
			secondBody, err := runAPIRequest(handler, secondRequest)
			if err != nil {
				t.Fatal(err)
			}
			if secondBody["ok"] != false || !strings.Contains(mapString(mapAny(secondBody["error"])["message"]), "read count limit 1") {
				t.Fatalf("second read response = %#v", secondBody)
			}
			close(release)
			firstResult := <-firstDone
			if firstResult.err != nil {
				t.Fatal(firstResult.err)
			}
			firstBody := firstResult.body
			if firstBody["ok"] != true {
				t.Fatalf("first read response = %#v", firstBody)
			}
			rc.mu.Lock()
			var budget *evidenceReadBudget
			if tc.name == "lawyer" {
				budget = rc.lawyerAPI.active.evidenceBudget
			} else {
				budget = rc.councilAPI.active.evidenceBudget
			}
			eventCount := len(rc.events)
			rc.mu.Unlock()
			if budget.reads != 1 || budget.bytes != 4 {
				t.Fatalf("read budget = %#v, want one four-byte read", budget)
			}
			if eventCount != 1 {
				t.Fatalf("evidence read events = %d, want 1", eventCount)
			}
		})
	}
}

func TestNonterminalInvalidAttemptPublishesRoleAPIChange(t *testing.T) {
	t.Run("lawyer", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		api.rc.cfg.CaseID = "arb-1"
		api.rc.lawyerAPI = api
		waitDone := make(chan map[string]any, 1)
		waitErr := make(chan error, 1)
		go func() {
			body, err := runAPIRequest(api.handleWait, httptest.NewRequest(
				http.MethodGet,
				lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&after_version=1&timeout_ms=1000",
				nil,
			))
			if err != nil {
				waitErr <- err
				return
			}
			waitDone <- body
		}()
		body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, "plaintiff", "openings:plaintiff", "unknown", nil))
		if err != nil {
			t.Fatal(err)
		}
		if got := intNumber(mapAny(body["turn"])["attempts_remaining"]); got != turn.attemptsMax-1 {
			t.Fatalf("response attempts_remaining = %d, want %d", got, turn.attemptsMax-1)
		}
		assertChangedWait(t, "lawyer", waitDone, waitErr)
	})

	t.Run("council", func(t *testing.T) {
		_, _, api := testRoleAPIs(t)
		turn := api.active
		waitDone := make(chan map[string]any, 1)
		waitErr := make(chan error, 1)
		go func() {
			body, err := runAPIRequest(api.handleWait, httptest.NewRequest(
				http.MethodGet,
				councilAPIBasePath+"/wait?case_id=arb-1&member_id=C1&after=deliberation:1:C1&after_version=1&timeout_ms=1000",
				nil,
			))
			if err != nil {
				waitErr <- err
				return
			}
			waitDone <- body
		}()
		body, err := runAPIRequest(api.handleDo, councilToolRequest(t, "unknown", nil))
		if err != nil {
			t.Fatal(err)
		}
		if got := intNumber(mapAny(body["turn"])["attempts_remaining"]); got != turn.attemptsMax-1 {
			t.Fatalf("response attempts_remaining = %d, want %d", got, turn.attemptsMax-1)
		}
		assertChangedWait(t, "council", waitDone, waitErr)
	})
}

func TestSuccessfulEvidenceReadPublishesRoleAPIChange(t *testing.T) {
	t.Run("lawyer", func(t *testing.T) {
		api, turn := testLawyerAPIWithTurn()
		api.rc.cfg.CaseID = "arb-1"
		api.rc.cfg.OutputDir = t.TempDir()
		api.rc.lawyerAPI = api
		meta := installTestEvidence(t, api.rc, []byte("lawyer wait evidence"))
		waitDone := make(chan map[string]any, 1)
		waitErr := make(chan error, 1)
		go func() {
			body, err := runAPIRequest(api.handleWait, httptest.NewRequest(
				http.MethodGet,
				lawyerAPIBasePath+"/wait?case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&after_version=1&timeout_ms=1000",
				nil,
			))
			if err != nil {
				waitErr <- err
				return
			}
			waitDone <- body
		}()
		body, err := runAPIRequest(lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, turn.opportunity.Role, turn.opportunity.ID, "read_evidence_range", map[string]any{
			"evidence_id": meta.EvidenceID,
			"offset":      0,
			"length":      4,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if body["ok"] != true {
			t.Fatalf("read response = %#v", body)
		}
		assertChangedWait(t, "lawyer", waitDone, waitErr)
	})

	t.Run("council", func(t *testing.T) {
		rc, _, api := testRoleAPIs(t)
		meta := installTestEvidence(t, rc, []byte("council wait evidence"))
		waitDone := make(chan map[string]any, 1)
		waitErr := make(chan error, 1)
		go func() {
			body, err := runAPIRequest(api.handleWait, httptest.NewRequest(
				http.MethodGet,
				councilAPIBasePath+"/wait?case_id=arb-1&member_id=C1&after=deliberation:1:C1&after_version=1&timeout_ms=1000",
				nil,
			))
			if err != nil {
				waitErr <- err
				return
			}
			waitDone <- body
		}()
		body, err := runAPIRequest(api.handleDo, councilToolRequest(t, "read_evidence_range", map[string]any{
			"evidence_id": meta.EvidenceID,
			"offset":      0,
			"length":      4,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if body["ok"] != true {
			t.Fatalf("read response = %#v", body)
		}
		assertChangedWait(t, "council", waitDone, waitErr)
	})
}

type apiCallResult struct {
	body map[string]any
	err  error
}

func configureLawyerFailureEngine(t *testing.T, api *lawyerAPIServer) {
	t.Helper()
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "failure-engine.sh")
	engineScript := `#!/bin/sh
cat >/dev/null
printf '%s\n' '{"ok":true,"state":{"case":{"status":"failed","phase":"openings","failure":{"failure_type":"opportunity_failed","role":"plaintiff","reason":"test","message":"failed"}},"state_version":1}}'
`
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write failure engine: %v", err)
	}
	api.rc.cfg.CaseID = "arb-1"
	api.rc.cfg.OutputDir = dir
	api.rc.cfg.Engine = lean.New([]string{enginePath})
	api.rc.lawyerAPI = api
}

func configureBlockingLawyerEngine(t *testing.T, api *lawyerAPIServer) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "blocking-engine.sh")
	enteredPath := filepath.Join(dir, "entered")
	releasePath := filepath.Join(dir, "release")
	callsPath := filepath.Join(dir, "calls")
	engineScript := `#!/bin/sh
request=$(cat)
case "$request" in
  *\"action_type\":\"fail_opportunity\"*)
    printf '%s\n' 'fail_opportunity' >> "$3"
    printf '%s\n' '{"ok":true,"state":{"case":{"status":"failed","phase":"openings","failure":{"failure_type":"opportunity_failed","role":"plaintiff","phase":"openings","opportunity_id":"openings:plaintiff","reason":"deadline_expired","message":"Plaintiff lawyer opportunity timed out."}},"state_version":1}}'
    ;;
  *)
    printf '%s\n' 'record_opening_statement' >> "$3"
    : > "$1"
    while [ ! -e "$2" ]; do
      sleep 0.01
    done
    printf '%s\n' '{"ok":true,"state":{"case":{"status":"active","phase":"arguments"},"state_version":1}}'
    ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write blocking lawyer engine: %v", err)
	}
	api.rc.cfg.OutputDir = dir
	api.rc.cfg.Engine = lean.New([]string{enginePath, enteredPath, releasePath, callsPath})
	api.rc.cfg.Runtime.EngineCallTimeoutSeconds = 2
	api.rc.lawyerAPI = api
	return enteredPath, releasePath, callsPath
}

func waitForTestPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat test path %s: %v", path, err)
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("test path was not created: %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func releaseBlockingEngine(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("release blocking engine: %v", err)
	}
}

func readTestLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test lines %s: %v", path, err)
	}
	return strings.Fields(string(raw))
}

func assertCompletedAttemptResponse(t *testing.T, body map[string]any, attemptsRemaining int) {
	t.Helper()
	if body["ok"] != false {
		t.Fatalf("response ok = %#v, want false: %#v", body["ok"], body)
	}
	turn := mapAny(body["turn"])
	if turn["completed"] != true {
		t.Fatalf("response turn completed = %#v, want true: %#v", turn["completed"], body)
	}
	if got := intNumber(turn["attempts_remaining"]); got != attemptsRemaining {
		t.Fatalf("response attempts_remaining = %d, want %d: %#v", got, attemptsRemaining, body)
	}
}

func lawyerToolRequest(t *testing.T, role string, opportunityID string, tool string, arguments map[string]any) *http.Request {
	t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
	raw, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"role_id":        role,
		"opportunity_id": opportunityID,
		"tool":           tool,
		"arguments":      arguments,
	})
	if err != nil {
		t.Fatalf("marshal lawyer tool request: %v", err)
	}
	return httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(raw))
}

func councilToolRequest(t *testing.T, tool string, arguments map[string]any) *http.Request {
	t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
	raw, err := json.Marshal(map[string]any{
		"case_id":        "arb-1",
		"member_id":      "C1",
		"opportunity_id": "deliberation:1:C1",
		"tool":           tool,
		"arguments":      arguments,
	})
	if err != nil {
		t.Fatalf("marshal council tool request: %v", err)
	}
	return httptest.NewRequest(http.MethodPost, councilAPIBasePath+"/do", bytes.NewReader(raw))
}

func setupBlockedLawyerEvidenceRequest(t *testing.T, stat bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request) {
	t.Helper()
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.CaseID = "arb-1"
	api.rc.cfg.OutputDir = t.TempDir()
	api.rc.lawyerAPI = api
	meta := installTestEvidence(t, api.rc, []byte("lawyer evidence"))
	tool, arguments := evidenceRequest(meta.EvidenceID, stat)
	return api.rc, &api.evidenceFiles, lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, turn.opportunity.Role, turn.opportunity.ID, tool, arguments)
}

func setupBlockedObserverEvidenceRequest(t *testing.T, stat bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request) {
	t.Helper()
	api, _ := testLawyerAPIWithTurn()
	api.rc.cfg.CaseID = "arb-1"
	api.rc.cfg.OutputDir = t.TempDir()
	api.rc.lawyerAPI = api
	meta := installTestEvidence(t, api.rc, []byte("observer evidence"))
	tool, arguments := evidenceRequest(meta.EvidenceID, stat)
	return api.rc, &api.evidenceFiles, lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, "observer", "", tool, arguments)
}

func setupBlockedCouncilEvidenceRequest(t *testing.T, stat bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request) {
	t.Helper()
	rc, _, api := testRoleAPIs(t)
	meta := installTestEvidence(t, rc, []byte("council evidence"))
	tool, arguments := evidenceRequest(meta.EvidenceID, stat)
	return rc, &api.evidenceFiles, api.handleDo, councilToolRequest(t, tool, arguments)
}

func evidenceRequest(evidenceID string, stat bool) (string, map[string]any) {
	if stat {
		return "stat_evidence", map[string]any{"evidence_id": evidenceID}
	}
	return "read_evidence_range", map[string]any{"evidence_id": evidenceID, "offset": 0, "length": 4}
}

func installTestEvidence(t *testing.T, rc *runContext, raw []byte) EvidenceMeta {
	t.Helper()
	if rc.cfg.OutputDir == "" {
		rc.cfg.OutputDir = t.TempDir()
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	storageName, err := evidenceStorageName(digest)
	if err != nil {
		t.Fatalf("evidence storage name: %v", err)
	}
	rc.evidenceStoreDir = filepath.Join(rc.cfg.OutputDir, "evidence-store")
	path := filepath.Join(rc.evidenceStoreDir, filepath.FromSlash(storageName))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o444); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	meta := EvidenceMeta{
		EvidenceID:          canonicalEvidenceID(digest, "test.txt"),
		SHA256:              digest,
		SizeBytes:           len(raw),
		MimeType:            "text/plain",
		StorageName:         storageName,
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "juror_visible",
	}
	rc.mu.Lock()
	rc.evidence = append(rc.evidence, meta)
	if rc.evidenceByID == nil {
		rc.evidenceByID = map[string]EvidenceMeta{}
	}
	rc.evidenceByID[meta.EvidenceID] = meta
	rc.mu.Unlock()
	return meta
}

func testRoleAPIs(t *testing.T) (*runContext, *lawyerAPIServer, *councilAPIServer) {
	t.Helper()
	rc := newCouncilOpportunityTestContext(t, "")
	rc.cfg.CaseID = "arb-1"
	lawyerAPI := newLawyerAPIServer(rc)
	lawyerAPI.active = testLawyerTurn("openings:plaintiff", "plaintiff", "openings")
	lawyerAPI.version = 1
	councilAPI := newCouncilAPIServer(rc)
	councilAPI.active = &councilTurn{
		opportunity: Opportunity{
			ID:           "deliberation:1:C1",
			StateVersion: 1,
			Role:         "council",
			Phase:        "deliberation",
			MemberID:     "C1",
			AllowedTools: []string{"submit_council_vote"},
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
	councilAPI.version = 1
	rc.lawyerAPI = lawyerAPI
	rc.councilAPI = councilAPI
	return rc, lawyerAPI, councilAPI
}

func runAPIRequest(handler http.HandlerFunc, request *http.Request) (map[string]any, error) {
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", recorder.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return body, nil
}

func lawyerDoHandler(caseCtx context.Context, api *lawyerAPIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		api.handleDo(caseCtx, w, r)
	}
}

func assertChangedWait(t *testing.T, name string, done <-chan map[string]any, errCh <-chan error) {
	t.Helper()
	select {
	case err := <-errCh:
		t.Fatalf("%s wait: %v", name, err)
	case body := <-done:
		wait := mapAny(body["wait"])
		if wait["reason"] != "changed" {
			t.Fatalf("%s wait = %#v, want changed", name, body["wait"])
		}
	case <-time.After(time.Second):
		t.Fatalf("%s wait did not receive the cross-API notification", name)
	}
}

func testLawyerDecisionAPI(t *testing.T) *lawyerAPIServer {
	t.Helper()
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	engineScript := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '{\"ok\":true,\"state\":{\"case\":{\"status\":\"active\",\"phase\":\"arguments\"},\"state_version\":1}}'\n"
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	policy := DefaultPolicy()
	state := initialState(policy, "arb-1", nil)
	caseObj := mapAny(state["case"])
	caseObj["status"] = "active"
	caseObj["phase"] = "openings"
	rc := &runContext{
		cfg: Config{
			CaseID:    "arb-1",
			OutputDir: dir,
			Engine:    lean.New([]string{enginePath}),
			Policy:    policy,
			Runtime:   DefaultRuntimeLimits(),
		},
		state:          state,
		fileByID:       map[string]CaseFile{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	turn := testLawyerTurn("openings:plaintiff", "plaintiff", "openings")
	turn.opportunity.StateVersion = 0
	api := newLawyerAPIServer(rc)
	api.active = turn
	api.version = 1
	rc.lawyerAPI = api
	return api
}

func lawyerDecisionRequest() *http.Request {
	return httptest.NewRequest(
		http.MethodPost,
		lawyerAPIBasePath+"/do",
		bytes.NewBufferString(`{"case_id":"arb-1","role_id":"plaintiff","opportunity_id":"openings:plaintiff","tool":"submit_decision","arguments":{"kind":"tool","tool_name":"record_opening_statement","payload":{"text":"Opening."}}}`),
	)
}

type blockingCouncilResponseClient struct {
	entered chan struct{}
	release chan struct{}
}

func (c *blockingCouncilResponseClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	_ modelrequest.Spec,
	_ []map[string]any,
	_ []map[string]any,
	_ string,
) (openaiapi.Response, error) {
	close(c.entered)
	select {
	case <-c.release:
		return openaiapi.Response{
			ToolCalls: []openaiapi.ToolCall{{
				Name: "submit_council_vote",
				Arguments: map[string]any{
					"vote":      "demonstrated",
					"rationale": "record sufficient",
				},
			}},
		}, nil
	case <-ctx.Done():
		return openaiapi.Response{}, ctx.Err()
	}
}

type blockingResponseWriter struct {
	header  http.Header
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingResponseWriter() *blockingResponseWriter {
	return &blockingResponseWriter{
		header:  make(http.Header),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (w *blockingResponseWriter) Header() http.Header {
	return w.header
}

func (w *blockingResponseWriter) WriteHeader(int) {}

func (w *blockingResponseWriter) Write(data []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(data), nil
}
