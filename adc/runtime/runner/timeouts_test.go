package runner

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/adc/runtime/store"
)

type stubTimeoutError struct{}

func (stubTimeoutError) Error() string   { return "timed out" }
func (stubTimeoutError) Timeout() bool   { return true }
func (stubTimeoutError) Temporary() bool { return true }

func TestIsTimeoutError(t *testing.T) {
	t.Parallel()

	var netErr net.Error = stubTimeoutError{}
	if !isTimeoutError(netErr) {
		t.Fatalf("isTimeoutError(net timeout) = false, want true")
	}
	if !isTimeoutError(context.DeadlineExceeded) {
		t.Fatalf("isTimeoutError(context deadline exceeded) = false, want true")
	}
	if isTimeoutError(nil) {
		t.Fatalf("isTimeoutError(nil) = true, want false")
	}
	if isTimeoutError(assertionError("different failure")) {
		t.Fatalf("isTimeoutError(non-timeout) = true, want false")
	}
}

func TestJurorTimeoutOpportunityKinds(t *testing.T) {
	t.Parallel()

	if !isCandidateJurorOpportunity(leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"answer_juror_questionnaire"},
	}) {
		t.Fatalf("candidate voir dire opportunity not recognized")
	}
	if !isCandidateJurorOpportunity(leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"answer_voir_dire_question"},
	}) {
		t.Fatalf("candidate voir dire answer opportunity not recognized")
	}
	if isCandidateJurorOpportunity(leanOpportunity{
		Phase:        "deliberation",
		AllowedTools: []string{"submit_juror_vote"},
	}) {
		t.Fatalf("deliberation opportunity misclassified as candidate timeout")
	}
	if !isDeliberationJurorOpportunity(leanOpportunity{
		Phase:        "deliberation",
		AllowedTools: []string{"submit_juror_vote"},
	}) {
		t.Fatalf("deliberation opportunity not recognized")
	}
	if isDeliberationJurorOpportunity(leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"answer_juror_questionnaire"},
	}) {
		t.Fatalf("voir dire opportunity misclassified as deliberation timeout")
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }

func newTimeoutTestRunner(t *testing.T) *Runner {
	t.Helper()

	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "run.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})
	if err := st.CreateRun("run-1", "timeout-test"); err != nil {
		t.Fatalf("create run: %v", err)
	}
	enginePath, err := filepath.Abs(filepath.Join("..", "..", "engine", ".lake", "build", "bin", "adcengine"))
	if err != nil {
		t.Fatalf("resolve engine path: %v", err)
	}
	return &Runner{
		lean:  lean.New([]string{enginePath}),
		store: st,
		cfg: Config{
			RunID: "run-1",
		},
		state: buildInitialState(spec.FormalScenario{CourtName: "Test Court"}, courts.Profile{}),
	}
}

func TestHandleCandidateJurorTimeoutReplacesJuror(t *testing.T) {
	r := newTimeoutTestRunner(t)
	caseObj := r.state["case"].(map[string]any)
	caseObj["status"] = "trial"
	caseObj["trial_mode"] = "jury"
	caseObj["phase"] = "voir_dire"
	caseObj["jury_configuration"] = map[string]any{
		"juror_count":        6,
		"unanimous_required": true,
		"minimum_concurring": 6,
	}
	caseObj["jurors"] = []any{
		map[string]any{"juror_id": "J1", "name": "Juror 1", "status": "candidate", "note": "", "model": "", "persona_filename": ""},
	}
	opportunity := leanOpportunity{
		OpportunityID: "opp-1",
		Phase:         "voir_dire",
		Objective:     "Answer the juror questionnaire.",
		AllowedTools:  []string{"answer_juror_questionnaire"},
		Constraints: map[string]any{
			"required_payload": map[string]any{"juror_id": "J1"},
		},
	}
	log, err := r.handleCandidateJurorTimeout(24, opportunity, "openrouter://model", "J1", context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("handleCandidateJurorTimeout error = %v", err)
	}
	if log.Steps != 2 {
		t.Fatalf("Steps = %d, want 2", log.Steps)
	}
	caseObj = r.state["case"].(map[string]any)
	jurors := caseObj["jurors"].([]any)
	if len(jurors) != 2 {
		t.Fatalf("juror count = %d, want 2", len(jurors))
	}
	first := jurors[0].(map[string]any)
	second := jurors[1].(map[string]any)
	if first["juror_id"] != "J1" || first["status"] != "timed_out" {
		t.Fatalf("first juror = %#v", first)
	}
	if second["juror_id"] != "J2" || second["status"] != "candidate" {
		t.Fatalf("replacement juror = %#v", second)
	}
}

func TestHandleCandidateJurorFailureReplacesJuror(t *testing.T) {
	r := newTimeoutTestRunner(t)
	caseObj := r.state["case"].(map[string]any)
	caseObj["status"] = "trial"
	caseObj["trial_mode"] = "jury"
	caseObj["phase"] = "voir_dire"
	caseObj["jury_configuration"] = map[string]any{
		"juror_count":        6,
		"unanimous_required": true,
		"minimum_concurring": 6,
	}
	caseObj["jurors"] = []any{
		map[string]any{"juror_id": "J1", "name": "Juror 1", "status": "candidate", "note": "", "model": "", "persona_filename": ""},
	}
	opportunity := leanOpportunity{
		OpportunityID: "opp-1",
		Phase:         "voir_dire",
		Objective:     "Answer the juror questionnaire.",
		AllowedTools:  []string{"answer_juror_questionnaire"},
		Constraints: map[string]any{
			"required_payload": map[string]any{"juror_id": "J1"},
		},
	}
	log, handled, err := r.handleOpportunityResponseError(24, spec.RoleSpec{Name: "juror"}, opportunity, "openrouter://model", assertionError("agent output limit exceeded"))
	if !handled {
		t.Fatalf("handled = false, want true")
	}
	if err != nil {
		t.Fatalf("handleOpportunityResponseError error = %v", err)
	}
	if log.Steps != 2 {
		t.Fatalf("Steps = %d, want 2", log.Steps)
	}
	caseObj = r.state["case"].(map[string]any)
	jurors := caseObj["jurors"].([]any)
	first := jurors[0].(map[string]any)
	second := jurors[1].(map[string]any)
	if first["juror_id"] != "J1" || first["status"] != "timed_out" {
		t.Fatalf("first juror = %#v", first)
	}
	if second["juror_id"] != "J2" || second["status"] != "candidate" {
		t.Fatalf("replacement juror = %#v", second)
	}
}

func TestHandleDeliberatingJurorTimeoutAllowsVerdictFromRemainingJurors(t *testing.T) {
	r := newTimeoutTestRunner(t)
	caseObj := r.state["case"].(map[string]any)
	caseObj["status"] = "trial"
	caseObj["trial_mode"] = "jury"
	caseObj["phase"] = "deliberation"
	caseObj["jury_configuration"] = map[string]any{
		"juror_count":        6,
		"unanimous_required": true,
		"minimum_concurring": 6,
	}
	caseObj["jurors"] = []any{
		map[string]any{"juror_id": "J1", "name": "Juror 1", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J2", "name": "Juror 2", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J3", "name": "Juror 3", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J4", "name": "Juror 4", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J5", "name": "Juror 5", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J6", "name": "Juror 6", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
	}
	caseObj["juror_votes"] = []any{
		map[string]any{"juror_id": "J2", "round": 1, "vote": "plaintiff", "damages": 100.0, "confidence": "high", "explanation": "e2", "submitted_at": "2026-06-06"},
		map[string]any{"juror_id": "J3", "round": 1, "vote": "plaintiff", "damages": 100.0, "confidence": "high", "explanation": "e3", "submitted_at": "2026-06-06"},
		map[string]any{"juror_id": "J4", "round": 1, "vote": "plaintiff", "damages": 100.0, "confidence": "high", "explanation": "e4", "submitted_at": "2026-06-06"},
		map[string]any{"juror_id": "J5", "round": 1, "vote": "plaintiff", "damages": 100.0, "confidence": "high", "explanation": "e5", "submitted_at": "2026-06-06"},
		map[string]any{"juror_id": "J6", "round": 1, "vote": "plaintiff", "damages": 100.0, "confidence": "high", "explanation": "e6", "submitted_at": "2026-06-06"},
	}
	opportunity := leanOpportunity{
		OpportunityID: "opp-2",
		Phase:         "deliberation",
		Objective:     "Submit the juror vote.",
		AllowedTools:  []string{"submit_juror_vote"},
		Constraints: map[string]any{
			"required_payload": map[string]any{"juror_id": "J1"},
		},
	}
	log, err := r.handleDeliberatingJurorTimeout(69, opportunity, "openrouter://model", "J1", context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("handleDeliberatingJurorTimeout error = %v", err)
	}
	if log.Steps != 1 {
		t.Fatalf("Steps = %d, want 1", log.Steps)
	}
	caseObj = r.state["case"].(map[string]any)
	jurors := caseObj["jurors"].([]any)
	first := jurors[0].(map[string]any)
	if first["status"] != "timed_out" {
		t.Fatalf("timed out juror = %#v", first)
	}
	if hung, _ := caseObj["hung_jury"].(map[string]any); hung != nil {
		t.Fatalf("hung_jury = %#v, want nil", hung)
	}
	verdict, _ := caseObj["jury_verdict"].(map[string]any)
	if verdict == nil {
		t.Fatalf("jury_verdict = nil, want verdict")
	}
	if verdict["verdict_for"] != "plaintiff" {
		t.Fatalf("verdict_for = %#v", verdict["verdict_for"])
	}
	if toInt(verdict["votes_for_verdict"]) != 5 || toInt(verdict["required_votes"]) != 5 {
		t.Fatalf("verdict counts = %#v", verdict)
	}
}
