package runner

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/adc/runtime/spec"
)

func TestCaseAPIHealthIdentifiesRun(t *testing.T) {
	response := httptest.NewRecorder()
	handleCaseAPIHealth(response, httptest.NewRequest(http.MethodGet, "/health", nil), "case-1", "run-1")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "case-1") || !strings.Contains(response.Body.String(), "run-1") {
		t.Fatalf("health response: %d %s", response.Code, response.Body.String())
	}
}

func TestObserverCannotActOnActiveTurn(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	status, err := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "observer"})
	if err != nil {
		t.Fatalf("statusResponseLocked error = %v", err)
	}
	if status["status"] != "active" {
		t.Fatalf("observer status = %#v, want active read access", status)
	}

	response, statusCode := api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "observer",
		OpportunityID: "opp-1",
		Tool:          "send_work_notes",
		Arguments:     map[string]any{"notes": "observer should not be able to write notes for plaintiff"},
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("observer do status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != false {
		t.Fatalf("observer do response = %#v", response)
	}
	if turn.completed {
		t.Fatalf("observer do completed the active turn")
	}

	response, statusCode = api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "observer",
		OpportunityID: "opp-1",
		Tool:          "submit_decision",
		Arguments:     map[string]any{"kind": "pass", "reason": "observer should not be able to pass for plaintiff"},
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("observer submit status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != false {
		t.Fatalf("observer submit response = %#v", response)
	}
	if turn.completed {
		t.Fatalf("observer submit completed the active turn")
	}
}

func TestCaseAPIServerReportsServeFailure(t *testing.T) {
	err := serveCaseAPI(&http.Server{}, adcFailedListener{err: errors.New("accept failed")})
	if err == nil || !strings.Contains(err.Error(), "case API server failed") {
		t.Fatalf("serveCaseAPI error = %v", err)
	}
}

func TestCaseAPIServerRecordsResponseFailure(t *testing.T) {
	api := &caseAPIServer{}
	w := &adcResponseErrorWriter{
		ResponseWriter: &adcFailedResponseWriter{header: make(http.Header), err: errors.New("write failed")},
		api:            api,
	}
	writeRoleAPIJSON(w, http.StatusOK, map[string]any{"ok": true})
	err := api.takeResponseError()
	if err == nil || !strings.Contains(err.Error(), "write case API response") {
		t.Fatalf("response error = %v", err)
	}
	if err := api.takeResponseError(); err != nil {
		t.Fatalf("response error was not cleared: %v", err)
	}
}

type adcFailedListener struct {
	err error
}

func (ln adcFailedListener) Accept() (net.Conn, error) { return nil, ln.err }
func (adcFailedListener) Close() error                 { return nil }
func (adcFailedListener) Addr() net.Addr               { return adcFailedAddr("failed") }

type adcFailedAddr string

func (addr adcFailedAddr) Network() string { return string(addr) }
func (addr adcFailedAddr) String() string  { return string(addr) }

type adcFailedResponseWriter struct {
	header http.Header
	err    error
}

func (w *adcFailedResponseWriter) Header() http.Header       { return w.header }
func (*adcFailedResponseWriter) WriteHeader(int)             {}
func (w *adcFailedResponseWriter) Write([]byte) (int, error) { return 0, w.err }

func TestObserverCannotReportFailureForActiveTurn(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	response, statusCode := api.failLocked(roleAPIRequest{
		CaseID:  "case-1",
		RoleID:  "observer",
		Message: "observer should not be able to fail plaintiff",
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("observer fail status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != false {
		t.Fatalf("observer fail response = %#v", response)
	}
	if turn.completed {
		t.Fatalf("observer fail completed the active turn")
	}
	select {
	case result := <-turn.done:
		t.Fatalf("observer fail sent turn result: %#v", result)
	default:
	}
}

func TestRoleAPIRejectsStaleOpportunity(t *testing.T) {
	for _, operation := range []string{"do", "fail"} {
		for _, staleField := range []string{"state_version", "opportunity_id"} {
			t.Run(operation+"/"+staleField, func(t *testing.T) {
				api, turn := testRoleAPIWithActiveTurn(t)
				turn.stateVersion = 2
				version := 2
				req := roleAPIRequest{CaseID: "case-1", RoleID: "plaintiff", OpportunityID: "opp-1", StateVersion: &version, Tool: "send_work_notes", Arguments: map[string]any{"notes": "stale"}}
				if staleField == "state_version" {
					version = 1
				} else {
					req.OpportunityID = "old"
				}
				call := api.doLocked
				if operation == "fail" {
					call = api.failLocked
				}
				response, code := call(req)
				apiError, _ := response["error"].(map[string]any)
				if code != http.StatusConflict || apiError["code"] != "wrong_opportunity" || turn.completed {
					t.Fatalf("stale request: %d %#v, completed=%v", code, response, turn.completed)
				}
				if version := api.currentTurnPayloadLocked(turn)["state_version"]; version != 2 {
					t.Fatalf("state_version = %v", version)
				}
			})
		}
	}
}

func TestRoleAPIRejectsMalformedRule60GrantedBeforeEngine(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)
	turn.opportunity.AllowedTools = []string{"resolve_rule60_motion"}

	response, statusCode := api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "plaintiff",
		OpportunityID: "opp-1",
		Tool:          "submit_decision",
		Arguments: map[string]any{
			"kind":      "tool",
			"tool_name": "resolve_rule60_motion",
			"payload":   map[string]any{"motion_index": 0},
		},
	})
	if statusCode != http.StatusOK || response["ok"] != false {
		t.Fatalf("response = (%d, %#v)", statusCode, response)
	}
	errorObject, _ := response["error"].(map[string]any)
	if errorObject["code"] != "invalid_decision" || errorObject["message"] != "required field granted must be a Boolean" {
		t.Fatalf("error = %#v", errorObject)
	}
	if turn.attemptsRemaining != 2 || turn.completed {
		t.Fatalf("turn = %+v", turn)
	}
}

func TestWorkNotesStayOutOfTurnTranscript(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	response, statusCode := api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "plaintiff",
		OpportunityID: "opp-1",
		Tool:          "send_work_notes",
		Arguments:     map[string]any{"notes": "plan, work log, and evidence analysis"},
	})
	if statusCode != http.StatusOK {
		t.Fatalf("send notes status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != true {
		t.Fatalf("send notes response = %#v", response)
	}
	if len(turn.transcript) != 0 {
		t.Fatalf("transcript = %#v, want no work-note entry", turn.transcript)
	}
	raw, err := os.ReadFile(api.r.workNotesPath())
	if err != nil {
		t.Fatalf("read work notes: %v", err)
	}
	if !strings.Contains(string(raw), "plan, work log, and evidence analysis") {
		t.Fatalf("work notes = %s", string(raw))
	}
	if turn.completed {
		t.Fatalf("send_work_notes completed the active turn")
	}
}

func TestRoleAPIExhaustedDecisionAttempts(t *testing.T) {
	for _, tc := range []struct {
		name, role, phase, tool, jurorID string
		wantError                        bool
	}{
		{"questionnaire", "juror", "voir_dire", "answer_juror_questionnaire", "J1", false},
		{"follow-up", "juror", "voir_dire", "answer_voir_dire_question", "J1", false},
		{"vote", "juror", "deliberation", "submit_juror_vote", "J1", false},
		{"replacement-error", "juror", "voir_dire", "answer_voir_dire_question", "missing", true},
		{"lawyer", "plaintiff", "trial", "record_opening_statement", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, turn := testRoleAPIWithActiveTurn(t)
			api.r = newTimeoutTestRunner(t)
			turn.role.Name = tc.role
			turn.principalID = tc.jurorID
			turn.opportunity.Phase = tc.phase
			turn.opportunity.AllowedTools = []string{tc.tool}
			turn.opportunity.Constraints = map[string]any{"required_payload": map[string]any{"juror_id": tc.jurorID}}
			caseObj := api.r.state["case"].(map[string]any)
			caseObj["status"], caseObj["trial_mode"], caseObj["phase"] = "trial", "jury", tc.phase
			caseObj["jury_configuration"] = map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6}
			jurors, votes := []any{}, []any{}
			for _, id := range []string{"J1", "J2", "J3", "J4", "J5", "J6"} {
				status := "candidate"
				if tc.phase == "deliberation" {
					status = "sworn"
					if id != "J1" {
						votes = append(votes, map[string]any{"juror_id": id, "round": 1, "vote": "plaintiff", "damages": 0.0, "confidence": "high", "explanation": "The record supports the claim.", "submitted_at": "2026-09-19"})
					}
				}
				jurors = append(jurors, map[string]any{"juror_id": id, "name": id, "status": status, "note": "", "model": "", "persona_filename": ""})
			}
			caseObj["jurors"], caseObj["juror_votes"] = jurors, votes
			for attempt := 1; attempt <= 3; attempt++ {
				response, code := api.doLocked(roleAPIRequest{
					RoleID: tc.role, PrincipalID: tc.jurorID, OpportunityID: "opp-1",
					Tool: "submit_decision", Arguments: map[string]any{},
				})
				if code != http.StatusOK || response["ok"] != false || response["attempts_remaining"] != 3-attempt {
					t.Fatalf("attempt %d: %d %#v", attempt, code, response)
				}
				if turn.completed != (attempt == 3) {
					t.Fatalf("attempt %d: completed=%v", attempt, turn.completed)
				}
				if attempt == 3 {
					apiError := response["error"].(map[string]any)
					if response["status"] != "failed" || apiError["code"] != "attempts_exhausted" {
						t.Fatalf("exhausted response = %#v", response)
					}
				}
			}
			var result externalOpportunityResult
			select {
			case result = <-turn.done:
			default:
				t.Fatal("no turn result")
			}
			if (result.err != nil) != tc.wantError {
				t.Fatalf("turn error = %v, wantError=%v", result.err, tc.wantError)
			}
			if tc.wantError {
				return
			}
			caseObj = api.r.state["case"].(map[string]any)
			jurors = caseObj["jurors"].([]any)
			if jurors[0].(map[string]any)["status"] != "timed_out" || !result.log.External {
				t.Fatalf("failed juror=%#v, external=%v", jurors[0], result.log.External)
			}
			if tc.phase == "voir_dire" {
				if len(jurors) != 7 || result.log.Steps != 2 {
					t.Fatalf("replacement: jurors=%d, steps=%d", len(jurors), result.log.Steps)
				}
				replacement := jurors[6].(map[string]any)
				if replacement["juror_id"] != "J7" || replacement["status"] != "candidate" {
					t.Fatalf("replacement=%#v", replacement)
				}
			} else {
				verdict, _ := caseObj["jury_verdict"].(map[string]any)
				if len(jurors) != 6 || result.log.Steps != 1 || verdict["verdict_for"] != "plaintiff" || toInt(verdict["required_votes"]) != 5 {
					t.Fatalf("remaining jury: jurors=%d, steps=%d, verdict=%#v", len(jurors), result.log.Steps, verdict)
				}
			}
		})
	}
}

func TestJurorRoleAPISpecsIncludeRecordReaders(t *testing.T) {
	r := &Runner{prompts: testPromptCatalog(t)}
	specs, err := r.roleAPIToolSpecs(spec.RoleSpec{Name: "juror"}, leanOpportunity{})
	if err != nil {
		t.Fatalf("roleAPIToolSpecs: %v", err)
	}
	names := map[string]bool{}
	for _, toolSpec := range specs {
		names[strings.TrimSpace(stringOrDefault(toolSpec["name"], ""))] = true
	}
	for _, name := range []string{"get_case", "list_case_files", "read_case_text_file", "request_case_file", "read_case_file_bytes"} {
		if !names[name] {
			t.Fatalf("juror tool specs missing %s: %#v", name, specs)
		}
	}
}

func TestRoleAPIPromptIncludesDeadlineAndBudgets(t *testing.T) {
	r := &Runner{
		prompts: testPromptCatalog(t),
		state: map[string]any{
			"case": map[string]any{"status": "active"},
		},
	}
	deadline := time.Date(2026, 6, 6, 10, 30, 0, 0, time.UTC)
	opportunity := leanOpportunity{
		OpportunityID: "opp-1",
		Objective:     "Submit a technical report.",
		AllowedTools:  []string{"submit_technical_report"},
		MayPass:       true,
	}
	availableToolSpecs, err := r.roleAPIToolSpecs(spec.RoleSpec{Name: "plaintiff"}, opportunity)
	if err != nil {
		t.Fatalf("roleAPIToolSpecs: %v", err)
	}
	prompt, err := r.buildRoleAPIPrompt(
		spec.RoleSpec{Name: "plaintiff"},
		map[string]any{"role": "plaintiff"},
		opportunity,
		deadline,
		30*time.Minute,
		3,
		30,
		availableToolSpecs,
	)
	if err != nil {
		t.Fatalf("buildRoleAPIPrompt: %v", err)
	}
	for _, want := range []string{
		"Deadline: submit this turn before 2026-06-06 10:30:00 UTC.",
		"The remaining_time_ms field in each response is live.",
		"Decision attempts: 3.",
		"Support tool calls: 30 per turn.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n%s", want, prompt)
		}
	}
}

func TestCurrentTurnPayloadIncludesDeadlineAt(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)
	turn.deadline = time.Date(2026, 6, 6, 10, 30, 0, 0, time.UTC)

	payload := api.currentTurnPayloadLocked(turn)
	if payload["deadline_at"] != "2026-06-06T10:30:00Z" {
		t.Fatalf("deadline_at = %#v", payload["deadline_at"])
	}
	if toInt(payload["remaining_time_ms"]) < 0 {
		t.Fatalf("remaining_time_ms = %#v", payload["remaining_time_ms"])
	}
}

func TestRoleAPIDoRejectsExpiredTurn(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)
	turn.deadline = time.Now().Add(-time.Second)
	turn.timeout = time.Second

	response, statusCode := api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "plaintiff",
		OpportunityID: "opp-1",
		Tool:          "send_work_notes",
		Arguments:     map[string]any{"notes": "late notes"},
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("expired do status = %d, response = %#v", statusCode, response)
	}
	if response["status"] != "expired" || response["ok"] != false {
		t.Fatalf("expired do response = %#v", response)
	}
	if !turn.completed {
		t.Fatalf("expired do did not complete the turn")
	}
	if len(turn.transcript) != 0 {
		t.Fatalf("transcript = %#v, want no late tool execution", turn.transcript)
	}
	select {
	case result := <-turn.done:
		if result.err == nil || !strings.Contains(result.err.Error(), "plaintiff opportunity timed out after 1s") {
			t.Fatalf("turn result error = %v", result.err)
		}
	default:
		t.Fatalf("expired do did not publish turn result")
	}
}

func TestRoleAPIStatusExpiresPastDeadline(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)
	turn.deadline = time.Now().Add(-time.Second)
	turn.timeout = time.Second

	response, err := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "plaintiff"})
	if err != nil {
		t.Fatalf("statusResponseLocked error = %v", err)
	}
	if response["status"] != "waiting" {
		t.Fatalf("expired status response = %#v", response)
	}
	if !turn.completed {
		t.Fatalf("expired status did not complete the turn")
	}
	select {
	case result := <-turn.done:
		if result.err == nil || !strings.Contains(result.err.Error(), "plaintiff opportunity timed out after 1s") {
			t.Fatalf("turn result error = %v", result.err)
		}
	default:
		t.Fatalf("expired status did not publish turn result")
	}
}

func TestRoleAPIStatusIncludesAdjudicationResolution(t *testing.T) {
	t.Parallel()

	r := &Runner{
		cfg: Config{CaseID: "case-1"},
		state: map[string]any{
			"case": map[string]any{
				"status":     "judgment_entered",
				"phase":      "post_verdict",
				"resolution": "demonstrated",
			},
		},
	}
	api := newRoleAPIServer(r)
	response, err := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "observer"})
	if err != nil {
		t.Fatalf("statusResponseLocked error = %v", err)
	}
	caseStatus, ok := response["case_status"].(map[string]any)
	if !ok {
		t.Fatalf("case_status = %#v", response["case_status"])
	}
	if got := caseStatus["resolution"]; got != "demonstrated" {
		t.Fatalf("resolution = %#v, want demonstrated", got)
	}
}

func testRoleAPIWithActiveTurn(t *testing.T) (*roleAPIServer, *externalOpportunityTurn) {
	t.Helper()

	r := &Runner{
		cfg:     Config{CaseID: "case-1", ScenarioBaseDir: t.TempDir()},
		prompts: testPromptCatalog(t),
		state: map[string]any{
			"case": map[string]any{"status": "active", "phase": "trial"},
		},
	}
	api := newRoleAPIServer(r)
	r.roleAPI = api
	turn := &externalOpportunityTurn{
		turnIndex: 1,
		role:      spec.RoleSpec{Name: "plaintiff"},
		opportunity: leanOpportunity{
			OpportunityID: "opp-1",
			Phase:         "trial",
			Kind:          "plaintiff_opening",
			MayPass:       true,
		},
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       3,
		attemptsRemaining: 3,
		supportBudget:     30,
		view:              map[string]any{"role": "plaintiff"},
		done:              make(chan externalOpportunityResult, 1),
	}
	api.active = turn
	return api, turn
}
