package proceeding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jsmorph/adj/arbd/runtime/lean"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
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
			lawyerAPIBasePath+"/wait?case_id=arbd-1&role_id=observer&after_version=1&timeout_ms=1000",
			nil,
		))
		lawyerDone <- response{body: body, err: err}
	}()
	go func() {
		body, err := runAPIRequest(councilAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			councilAPIBasePath+"/wait?case_id=arbd-1&member_id=C1&after_version=1&timeout_ms=1000",
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
			lawyerAPIBasePath+"/wait?case_id=arbd-1&role_id=observer&after_version=1&timeout_ms=1000",
			nil,
		))
		lawyerDone <- response{body: body, err: err}
	}()
	go func() {
		body, err := runAPIRequest(councilAPI.handleWait, httptest.NewRequest(
			http.MethodGet,
			councilAPIBasePath+"/wait?case_id=arbd-1&member_id=C1&after_version=1&timeout_ms=1000",
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

func TestRoleAPIHTTPWritesDoNotHoldCaseMutex(t *testing.T) {
	_, lawyerAPI, councilAPI := testRoleAPIs(t)
	lawyerRequest, err := json.Marshal(map[string]any{
		"case_id": "arbd-1",
		"role_id": "observer",
		"tool":    "get_case",
	})
	if err != nil {
		t.Fatalf("marshal lawyer request: %v", err)
	}
	councilRequest, err := json.Marshal(map[string]any{
		"case_id":        "arbd-1",
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
	rc.cfg.PromptDir = filepath.Join("..", "..", "prompts")
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
	if stateVersion != 2 || actionCount != 1 || len(events) != 1 || events[0].Type != "council_answer" {
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
		"case_id":        "arbd-1",
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
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("evidence read did not begin")
		}
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
		select {
		case result := <-requestDone:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if code := mapString(mapAny(result.body["error"])["code"]); code != "stale_opportunity" {
				t.Fatalf("read response = %#v, want stale_opportunity", result.body)
			}
		case <-time.After(time.Second):
			t.Fatal("evidence request did not finish")
		}
		if turn.evidenceBudget.reads != 0 || turn.evidenceBudget.bytes != 0 {
			t.Fatalf("stale read budget = %#v, want zero", turn.evidenceBudget)
		}
	})

	t.Run("council", func(t *testing.T) {
		rc, _, api := testRoleAPIs(t)
		rc.cfg.OutputDir = t.TempDir()
		rc.cfg.Engine = roleAPITestEngine(`{"ok":true,"state":{"case":{"status":"failed","phase":"deliberation"},"state_version":2}}`)
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
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("evidence read did not begin")
		}
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
		select {
		case result := <-requestDone:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if code := mapString(mapAny(result.body["error"])["code"]); code != "stale_opportunity" {
				t.Fatalf("read response = %#v, want stale_opportunity", result.body)
			}
		case <-time.After(time.Second):
			t.Fatal("evidence request did not finish")
		}
		if turn.evidenceBudget.reads != 0 || turn.evidenceBudget.bytes != 0 {
			t.Fatalf("stale read budget = %#v, want zero", turn.evidenceBudget)
		}
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
	api.rc.cfg.CaseID = "arbd-1"
	api.rc.cfg.OutputDir = dir
	api.rc.cfg.Engine = lean.New([]string{enginePath})
	api.rc.lawyerAPI = api
}

func testRoleAPIs(t *testing.T) (*runContext, *lawyerAPIServer, *councilAPIServer) {
	t.Helper()
	policy := DefaultPolicy()
	state := initialState(policy, "arbd-1", nil)
	state["state_version"] = 1
	caseObj := mapAny(state["case"])
	caseObj["status"] = "active"
	caseObj["phase"] = "deliberation"
	caseObj["council_members"] = []map[string]any{{"member_id": "C1", "status": "seated"}}
	rc := &runContext{
		cfg:     Config{CaseID: "arbd-1", Policy: policy, Runtime: DefaultRuntimeLimits()},
		state:   state,
		council: []CouncilSeat{{MemberID: "C1"}},
	}
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

func testLawyerDecisionAPI(t *testing.T) *lawyerAPIServer {
	t.Helper()
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	engineScript := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '{\"ok\":true,\"state\":{\"case\":{\"status\":\"active\",\"phase\":\"arguments\"},\"state_version\":1}}'\n"
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	policy := DefaultPolicy()
	state := initialState(policy, "arbd-1", nil)
	caseObj := mapAny(state["case"])
	caseObj["status"] = "active"
	caseObj["phase"] = "openings"
	rc := &runContext{
		cfg: Config{
			CaseID:    "arbd-1",
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
		bytes.NewBufferString(`{"case_id":"arbd-1","role_id":"plaintiff","opportunity_id":"openings:plaintiff","tool":"submit_decision","arguments":{"kind":"tool","tool_name":"record_opening_statement","payload":{"text":"Opening."}}}`),
	)
}

func setupBlockedLawyerEvidenceRequest(t *testing.T, stat bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request) {
	t.Helper()
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.CaseID = "arbd-1"
	api.rc.cfg.OutputDir = t.TempDir()
	api.rc.lawyerAPI = api
	meta := installTestEvidence(t, api.rc, []byte("lawyer evidence"))
	tool, arguments := evidenceRequest(meta.EvidenceID, stat)
	return api.rc, &api.evidenceFiles, lawyerDoHandler(context.Background(), api), lawyerToolRequest(t, turn.opportunity.Role, turn.opportunity.ID, tool, arguments)
}

func setupBlockedObserverEvidenceRequest(t *testing.T, stat bool) (*runContext, *evidenceFileOperations, http.HandlerFunc, *http.Request) {
	t.Helper()
	api, _ := testLawyerAPIWithTurn()
	api.rc.cfg.CaseID = "arbd-1"
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

func lawyerToolRequest(t *testing.T, role string, opportunityID string, tool string, arguments map[string]any) *http.Request {
	t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
	raw, err := json.Marshal(map[string]any{
		"case_id":        "arbd-1",
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
		"case_id":        "arbd-1",
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
		RecordVisibility:    "council_visible",
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
				Name: "submit_council_answer",
				Arguments: map[string]any{
					"answer":    72,
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
