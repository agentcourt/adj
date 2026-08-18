package runner

import (
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func TestObserverCannotActOnActiveTurn(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	status := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "observer"})
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

func TestJurorRoleAPISpecsIncludeRecordReaders(t *testing.T) {
	r := &Runner{}
	specs := r.roleAPIToolSpecs(spec.RoleSpec{Name: "juror"}, leanOpportunity{})
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
		state: map[string]any{
			"case": map[string]any{"status": "active"},
		},
	}
	deadline := time.Date(2026, 6, 6, 10, 30, 0, 0, time.UTC)
	prompt := r.buildRoleAPIPrompt(
		spec.RoleSpec{Name: "plaintiff"},
		map[string]any{"role": "plaintiff"},
		leanOpportunity{
			OpportunityID: "opp-1",
			Objective:     "Submit a technical report.",
			AllowedTools:  []string{"submit_technical_report"},
			MayPass:       true,
		},
		deadline,
		30*time.Minute,
		3,
		30,
	)
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

	response := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "plaintiff"})
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
	response := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "observer"})
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
		cfg: Config{CaseID: "case-1", ScenarioBaseDir: t.TempDir()},
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
