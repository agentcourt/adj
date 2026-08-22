package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	adcprompts "github.com/jsmorph/adj/adc/runtime/prompts"
	"github.com/jsmorph/adj/adc/runtime/spec"
)

const (
	DefaultCaseAPIAddr       = "127.0.0.1:0"
	roleAPIBasePath          = "/roleapi/v1"
	defaultRoleAPIWait       = 30 * time.Second
	maxRoleAPIWait           = 30 * time.Second
	roleAPIDeadlineCheck     = time.Second
	maxRoleAPIRequestBytes   = 64 << 20
	roleAPIWorkNotesFilename = "work-notes.ndjson"
)

type caseAPIServer struct {
	server        *http.Server
	ln            net.Listener
	baseURL       string
	serveDone     chan error
	responseErrMu sync.Mutex
	responseErr   error
}

type roleAPIServer struct {
	r *Runner

	mu      sync.Mutex
	cond    *sync.Cond
	version uint64
	active  *externalOpportunityTurn

	terminal       bool
	terminalResult Result
	terminalError  string
}

type externalOpportunityTurn struct {
	turnIndex    int
	role         spec.RoleSpec
	principalID  string
	opportunity  leanOpportunity
	rolesPayload []map[string]any
	stateVersion int
	prompt       string
	view         map[string]any
	deadline     time.Time
	timeout      time.Duration

	attemptsMax        int
	attemptsRemaining  int
	invalidReasons     []string
	supportBudget      int
	supportUsed        int
	availableToolSpecs []map[string]any
	stepsUsed          int
	transcript         []map[string]any
	completed          bool
	done               chan externalOpportunityResult
}

type externalOpportunityResult struct {
	log TurnLog
	err error
}

type roleAPIRequest struct {
	CaseID        string         `json:"case_id"`
	RoleID        string         `json:"role_id"`
	PrincipalID   string         `json:"principal_id,omitempty"`
	OpportunityID string         `json:"opportunity_id,omitempty"`
	Tool          string         `json:"tool,omitempty"`
	Arguments     map[string]any `json:"arguments,omitempty"`
	TimeoutMS     int            `json:"timeout_ms,omitempty"`
	Message       string         `json:"message,omitempty"`
}

type workNoteRecord struct {
	Timestamp     string `json:"timestamp"`
	CaseID        string `json:"case_id"`
	RunID         string `json:"run_id"`
	RoleID        string `json:"role_id"`
	PrincipalID   string `json:"principal_id,omitempty"`
	OpportunityID string `json:"opportunity_id,omitempty"`
	Notes         string `json:"notes"`
}

func startCaseAPIServer(r *Runner) (*caseAPIServer, error) {
	addr := strings.TrimSpace(r.cfg.CaseAPIAddr)
	if addr == "" {
		addr = DefaultCaseAPIAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("start case API listener: %w", err)
	}
	roleAPI := newRoleAPIServer(r)
	r.roleAPI = roleAPI
	api := &caseAPIServer{
		ln:        ln,
		baseURL:   "http://" + listenerHostPort(ln.Addr()),
		serveDone: make(chan error, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		handleCaseAPIHealth(w, req, r.cfg.CaseID, r.cfg.RunID)
	})
	roleAPI.register(mux)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mux.ServeHTTP(&adcResponseErrorWriter{ResponseWriter: w, api: api}, req)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	api.server = server
	go func() {
		failure := serveCaseAPI(server, ln)
		if failure != nil {
			if persistErr := r.persistAgentEvent(0, 0, "system", "caseapi_error", map[string]any{"error": failure.Error()}); persistErr != nil {
				failure = errors.Join(failure, fmt.Errorf("persist case API error event: %w", persistErr))
			}
			roleAPI.setTerminal(Result{}, failure)
		}
		api.serveDone <- failure
	}()
	if _, err := fmt.Fprintf(os.Stderr, "adc case api listening on %s\n", api.baseURL); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return nil, errors.Join(fmt.Errorf("write case API address: %w", err), api.Close(shutdownCtx))
	}
	return api, nil
}

func serveCaseAPI(server *http.Server, ln net.Listener) error {
	if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("case API server failed: %w", err)
	}
	return nil
}

func handleCaseAPIHealth(w http.ResponseWriter, r *http.Request, caseID, runID string) {
	if r.Method != http.MethodGet {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": roleAPIError("method_not_allowed", "use GET"),
		})
		return
	}
	writeRoleAPIJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"case_id": caseID,
		"run_id":  runID,
	})
}

func listenerHostPort(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	switch host {
	case "", "::", "0.0.0.0", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func (s *caseAPIServer) Close(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	shutdownErr := s.server.Shutdown(ctx)
	return errors.Join(shutdownErr, <-s.serveDone, s.takeResponseError())
}

type adcResponseErrorWriter struct {
	http.ResponseWriter
	api *caseAPIServer
}

func (w *adcResponseErrorWriter) recordResponseError(err error) {
	w.api.recordResponseError(err)
}

func (s *caseAPIServer) recordResponseError(err error) {
	if err == nil {
		return
	}
	s.responseErrMu.Lock()
	defer s.responseErrMu.Unlock()
	s.responseErr = errors.Join(s.responseErr, fmt.Errorf("write case API response: %w", err))
}

func (s *caseAPIServer) takeResponseError() error {
	s.responseErrMu.Lock()
	defer s.responseErrMu.Unlock()
	err := s.responseErr
	s.responseErr = nil
	return err
}

func newRoleAPIServer(r *Runner) *roleAPIServer {
	api := &roleAPIServer{r: r}
	api.cond = sync.NewCond(&api.mu)
	return api
}

func (api *roleAPIServer) register(mux *http.ServeMux) {
	mux.HandleFunc(roleAPIBasePath+"/get", api.handleGet)
	mux.HandleFunc(roleAPIBasePath+"/wait_for_opportunity", api.handleWait)
	mux.HandleFunc(roleAPIBasePath+"/status", api.handleStatus)
	mux.HandleFunc(roleAPIBasePath+"/result", api.handleResult)
	mux.HandleFunc(roleAPIBasePath+"/do", api.handleDo)
	mux.HandleFunc(roleAPIBasePath+"/fail", api.handleFail)
}

func externalRoleSet(roles []string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range roles {
		role := strings.TrimSpace(raw)
		if role == "" {
			continue
		}
		out[role] = true
	}
	return out
}

func (r *Runner) roleIsExternal(role string) bool {
	if r == nil {
		return false
	}
	return r.externalRoles[strings.TrimSpace(role)]
}

func (r *Runner) executeExternalOpportunityTurn(
	ctx context.Context,
	turnIndex int,
	role spec.RoleSpec,
	opportunity leanOpportunity,
	rolesPayload []map[string]any,
	stateVersion int,
) (TurnLog, error) {
	if r.roleAPI == nil {
		return TurnLog{}, fmt.Errorf("role API server is not running")
	}
	view, err := r.lean.View(r.state, role.Name)
	if err != nil {
		return TurnLog{}, err
	}
	timeout := time.Duration(r.cfg.Runtime.Normalized().RoleAPITimeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	attemptsMax := r.cfg.Runtime.Normalized().InvalidAttemptLimit
	supportBudget := supportToolBudget(r.state)
	availableToolSpecs, err := r.roleAPIToolSpecs(role, opportunity)
	if err != nil {
		return TurnLog{}, err
	}
	prompt, err := r.buildRoleAPIPrompt(role, view, opportunity, deadline, timeout, attemptsMax, supportBudget, availableToolSpecs)
	if err != nil {
		return TurnLog{}, err
	}
	turn := &externalOpportunityTurn{
		turnIndex:          turnIndex,
		role:               role,
		principalID:        principalIDForOpportunity(role.Name, opportunity),
		opportunity:        opportunity,
		rolesPayload:       rolesPayload,
		stateVersion:       stateVersion,
		prompt:             prompt,
		view:               view,
		deadline:           deadline,
		timeout:            timeout,
		attemptsMax:        attemptsMax,
		attemptsRemaining:  attemptsMax,
		supportBudget:      supportBudget,
		availableToolSpecs: availableToolSpecs,
		done:               make(chan externalOpportunityResult, 1),
	}
	if err := r.roleAPI.startTurn(turn); err != nil {
		return TurnLog{}, err
	}
	defer r.roleAPI.clearTurn(turn)
	deadlineCheck := time.NewTicker(roleAPIDeadlineCheck)
	defer deadlineCheck.Stop()
	for {
		if r.roleAPI.expireTurnIfDue(turn, time.Now()) {
			continue
		}
		select {
		case <-ctx.Done():
			return TurnLog{}, ctx.Err()
		case <-deadlineCheck.C:
			if r.roleAPI.expireTurnIfDue(turn, time.Now()) {
				continue
			}
		case result := <-turn.done:
			return result.log, result.err
		}
	}
}

func (api *roleAPIServer) startTurn(turn *externalOpportunityTurn) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active != nil && !api.active.completed {
		return fmt.Errorf("role API already has an active opportunity")
	}
	api.active = turn
	api.signalChangedLocked()
	return nil
}

func (api *roleAPIServer) clearTurn(turn *externalOpportunityTurn) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active == turn {
		api.active = nil
		api.signalChangedLocked()
	}
}

func (api *roleAPIServer) setTerminal(result Result, err error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.terminal = true
	api.terminalResult = result
	if err != nil {
		api.terminalError = err.Error()
	}
	api.active = nil
	api.signalChangedLocked()
}

func (api *roleAPIServer) signalChangedLocked() {
	api.version++
	api.cond.Broadcast()
}

func (api *roleAPIServer) expireTurnIfDue(turn *externalOpportunityTurn, now time.Time) bool {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active != turn || turn == nil || turn.completed || !turnDeadlineExpired(turn, now) {
		return false
	}
	api.expireTurnLocked(turn)
	return true
}

func (api *roleAPIServer) expireActiveTurnLocked(now time.Time) bool {
	turn := api.active
	if turn == nil || turn.completed || !turnDeadlineExpired(turn, now) {
		return false
	}
	api.expireTurnLocked(turn)
	return true
}

func turnDeadlineExpired(turn *externalOpportunityTurn, now time.Time) bool {
	if turn == nil || turn.deadline.IsZero() {
		return false
	}
	return now.UnixNano() >= turn.deadline.UnixNano()
}

func (api *roleAPIServer) expireTurnLocked(turn *externalOpportunityTurn) {
	if turn == nil || turn.completed {
		return
	}
	err := turnTimeoutError(turn)
	if turn.role.Name == "juror" {
		log, handled, handleErr := api.r.handleOpportunityResponseError(turn.turnIndex, turn.role, turn.opportunity, "", err)
		if handled {
			api.finishTurnLocked(turn, log, handleErr)
			return
		}
	}
	api.finishTurnLocked(turn, TurnLog{}, err)
}

func turnTimeoutError(turn *externalOpportunityTurn) error {
	if turn == nil {
		return fmt.Errorf("opportunity timed out")
	}
	if turn.timeout > 0 {
		return fmt.Errorf("%s opportunity timed out after %s", turn.role.Name, turn.timeout)
	}
	return fmt.Errorf("%s opportunity timed out", turn.role.Name)
}

func (api *roleAPIServer) handleGet(w http.ResponseWriter, r *http.Request) {
	req, err := api.requestFromHTTP(w, r)
	if err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("bad_request", err.Error())})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": roleAPIError("method_not_allowed", "use GET or POST")})
		return
	}
	api.mu.Lock()
	response := api.statusResponseLocked(req)
	api.mu.Unlock()
	writeRoleAPIJSON(w, http.StatusOK, response)
}

func (api *roleAPIServer) handleWait(w http.ResponseWriter, r *http.Request) {
	req, err := api.requestFromHTTP(w, r)
	if err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("bad_request", err.Error())})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": roleAPIError("method_not_allowed", "use GET or POST")})
		return
	}
	timeout := roleAPIWaitDuration(req.TimeoutMS)
	api.mu.Lock()
	deadline := time.Now().Add(timeout)
	timer := time.AfterFunc(timeout, func() {
		api.mu.Lock()
		api.cond.Broadcast()
		api.mu.Unlock()
	})
	defer timer.Stop()
	go func() {
		<-r.Context().Done()
		api.mu.Lock()
		api.cond.Broadcast()
		api.mu.Unlock()
	}()
	for {
		if r.Context().Err() != nil {
			api.mu.Unlock()
			return
		}
		response := api.statusResponseLocked(req)
		status := strings.TrimSpace(stringOrDefault(response["status"], ""))
		if status == "active" || status == "done" || status == "failed" || !time.Now().Before(deadline) {
			if status == "waiting" {
				response["wait"] = map[string]any{"status": "waiting", "timeout_ms": int(timeout / time.Millisecond)}
			}
			api.mu.Unlock()
			writeRoleAPIJSON(w, http.StatusOK, response)
			return
		}
		api.cond.Wait()
	}
}

func (api *roleAPIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": roleAPIError("method_not_allowed", "use GET or POST")})
		return
	}
	req, err := api.requestFromHTTP(w, r)
	if err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("bad_request", err.Error())})
		return
	}
	api.mu.Lock()
	response := api.statusResponseLocked(req)
	api.mu.Unlock()
	writeRoleAPIJSON(w, http.StatusOK, response)
}

func (api *roleAPIServer) handleResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": roleAPIError("method_not_allowed", "use GET or POST")})
		return
	}
	req, err := api.requestFromHTTP(w, r)
	if err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("bad_request", err.Error())})
		return
	}
	if err := api.validateCaseID(req.CaseID); err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("case_mismatch", err.Error())})
		return
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if !api.terminal {
		writeRoleAPIJSON(w, http.StatusOK, map[string]any{"ok": true, "case_id": api.caseID(), "status": "pending"})
		return
	}
	status := "done"
	if strings.TrimSpace(api.terminalError) != "" {
		status = "failed"
	}
	writeRoleAPIJSON(w, http.StatusOK, map[string]any{
		"ok":      strings.TrimSpace(api.terminalError) == "",
		"case_id": api.caseID(),
		"status":  status,
		"error":   api.terminalError,
		"result":  api.terminalResult,
	})
}

func (api *roleAPIServer) handleDo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": roleAPIError("method_not_allowed", "use POST")})
		return
	}
	req, err := api.requestFromHTTP(w, r)
	if err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("bad_request", err.Error())})
		return
	}
	api.mu.Lock()
	response, status := api.doLocked(req)
	api.mu.Unlock()
	writeRoleAPIJSON(w, status, response)
}

func (api *roleAPIServer) handleFail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRoleAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": roleAPIError("method_not_allowed", "use POST")})
		return
	}
	req, err := api.requestFromHTTP(w, r)
	if err != nil {
		writeRoleAPIJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": roleAPIError("bad_request", err.Error())})
		return
	}
	api.mu.Lock()
	response, status := api.failLocked(req)
	api.mu.Unlock()
	writeRoleAPIJSON(w, status, response)
}

func (api *roleAPIServer) requestFromHTTP(w http.ResponseWriter, r *http.Request) (roleAPIRequest, error) {
	var req roleAPIRequest
	switch r.Method {
	case http.MethodGet:
		req.CaseID = strings.TrimSpace(r.URL.Query().Get("case_id"))
		req.RoleID = strings.TrimSpace(r.URL.Query().Get("role_id"))
		req.PrincipalID = strings.TrimSpace(r.URL.Query().Get("principal_id"))
		req.OpportunityID = strings.TrimSpace(r.URL.Query().Get("opportunity_id"))
	case http.MethodPost:
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRoleAPIRequestBytes))
		dec.UseNumber()
		if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			return roleAPIRequest{}, err
		}
	default:
	}
	req.CaseID = strings.TrimSpace(req.CaseID)
	req.RoleID = strings.TrimSpace(req.RoleID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	req.OpportunityID = strings.TrimSpace(req.OpportunityID)
	req.Tool = strings.TrimSpace(req.Tool)
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	if req.CaseID == "" {
		return roleAPIRequest{}, fmt.Errorf("case_id is required")
	}
	if req.RoleID == "" {
		return roleAPIRequest{}, fmt.Errorf("role_id is required")
	}
	if err := api.validateCaseID(req.CaseID); err != nil {
		return roleAPIRequest{}, err
	}
	return req, nil
}

func roleAPIWaitDuration(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return defaultRoleAPIWait
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout > maxRoleAPIWait {
		return maxRoleAPIWait
	}
	if timeout <= 0 {
		return defaultRoleAPIWait
	}
	return timeout
}

func (api *roleAPIServer) validateCaseID(caseID string) error {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return fmt.Errorf("case_id is required")
	}
	if caseID != api.caseID() {
		return fmt.Errorf("unknown case_id %q", caseID)
	}
	return nil
}

func (api *roleAPIServer) caseID() string {
	caseID := strings.TrimSpace(api.r.cfg.CaseID)
	if caseID != "" {
		return caseID
	}
	runID := strings.TrimSpace(api.r.cfg.RunID)
	if runID != "" {
		return runID
	}
	return strings.TrimSpace(api.r.scenario.Name)
}

func (api *roleAPIServer) statusResponseLocked(req roleAPIRequest) map[string]any {
	api.expireActiveTurnLocked(time.Now())
	response := map[string]any{
		"ok":           true,
		"case_id":      api.caseID(),
		"role_id":      strings.TrimSpace(req.RoleID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
		"version":      api.version,
		"case_status":  api.caseStatusLocked(),
	}
	if api.terminal {
		status := "done"
		if strings.TrimSpace(api.terminalError) != "" {
			status = "failed"
			response["ok"] = false
			response["error"] = roleAPIError("case_failed", api.terminalError)
		}
		response["status"] = status
		response["result"] = api.terminalResult
		return response
	}
	if api.active == nil || api.active.completed {
		response["status"] = "waiting"
		return response
	}
	response["current_turn"] = api.currentTurnPayloadLocked(api.active)
	if !api.turnVisibleToRequest(api.active, req) {
		response["status"] = "waiting"
		return response
	}
	response["status"] = "active"
	response["opportunity"] = api.opportunityPayloadLocked(api.active, true)
	return response
}

func (api *roleAPIServer) caseStatusLocked() map[string]any {
	caseObj, _ := api.r.state["case"].(map[string]any)
	if caseObj == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for _, key := range []string{"status", "phase", "resolution", "current_phase"} {
		if value, ok := caseObj[key]; ok {
			out[key] = value
		}
	}
	return out
}

func (api *roleAPIServer) currentTurnPayloadLocked(turn *externalOpportunityTurn) map[string]any {
	if turn == nil {
		return nil
	}
	payload := map[string]any{
		"role_id":             turn.role.Name,
		"principal_id":        turn.principalID,
		"opportunity_id":      turn.opportunity.OpportunityID,
		"phase":               turn.opportunity.Phase,
		"kind":                turn.opportunity.Kind,
		"deadline_at":         turn.deadline.UTC().Format(time.RFC3339),
		"remaining_time_ms":   remainingMillis(turn.deadline),
		"attempts_remaining":  turn.attemptsRemaining,
		"attempts_max":        turn.attemptsMax,
		"support_calls_left":  maxInt(0, turn.supportBudget-turn.supportUsed),
		"support_calls_limit": turn.supportBudget,
	}
	return payload
}

func (api *roleAPIServer) opportunityPayloadLocked(turn *externalOpportunityTurn, includePrompt bool) map[string]any {
	payload := api.currentTurnPayloadLocked(turn)
	payload["objective"] = turn.opportunity.Objective
	payload["actor_message"] = turn.opportunity.ActorMessage
	payload["may_pass"] = turn.opportunity.MayPass
	payload["allowed_legal_tools"] = append([]string{}, turn.opportunity.AllowedTools...)
	payload["legal_tool_specs"] = api.r.legalToolSpecs(turn.opportunity.AllowedTools)
	payload["available_tool_specs"] = append([]map[string]any(nil), turn.availableToolSpecs...)
	payload["constraints"] = cloneJSONMap(turn.opportunity.Constraints)
	payload["view"] = turn.view
	if agent := api.r.jurorAgentPayload(turn); agent != nil {
		payload["agent"] = agent
	}
	if includePrompt {
		payload["prompt"] = turn.prompt
	}
	return payload
}

func remainingMillis(deadline time.Time) int64 {
	if deadline.IsZero() {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return int64(remaining / time.Millisecond)
}

func (api *roleAPIServer) turnVisibleToRequest(turn *externalOpportunityTurn, req roleAPIRequest) bool {
	if turn == nil {
		return false
	}
	if strings.TrimSpace(req.RoleID) == "observer" {
		return true
	}
	return api.turnMatchesActor(turn, req)
}

func (api *roleAPIServer) turnMatchesActor(turn *externalOpportunityTurn, req roleAPIRequest) bool {
	if turn == nil {
		return false
	}
	if strings.TrimSpace(req.RoleID) != strings.TrimSpace(turn.role.Name) {
		return false
	}
	if strings.TrimSpace(turn.principalID) == "" {
		return strings.TrimSpace(req.PrincipalID) == ""
	}
	return strings.TrimSpace(req.PrincipalID) == strings.TrimSpace(turn.principalID)
}

func (api *roleAPIServer) doLocked(req roleAPIRequest) (map[string]any, int) {
	if req.Tool == "" {
		return map[string]any{"ok": false, "error": roleAPIError("missing_tool", "tool is required")}, http.StatusBadRequest
	}
	if req.Tool == "case_status" {
		response := api.statusResponseLocked(req)
		response["result"] = map[string]any{"case_status": response["case_status"], "current_turn": response["current_turn"]}
		return response, http.StatusOK
	}
	if api.expireActiveTurnLocked(time.Now()) {
		return map[string]any{"ok": false, "case_id": api.caseID(), "status": "expired", "error": roleAPIError("opportunity_expired", "the active opportunity deadline has passed")}, http.StatusConflict
	}
	if api.active == nil || api.active.completed {
		return map[string]any{"ok": false, "case_id": api.caseID(), "status": "waiting", "error": roleAPIError("no_active_opportunity", "no active opportunity for this role")}, http.StatusConflict
	}
	turn := api.active
	if !api.turnMatchesActor(turn, req) {
		return map[string]any{"ok": false, "case_id": api.caseID(), "status": "waiting", "current_turn": api.currentTurnPayloadLocked(turn), "error": roleAPIError("wrong_turn", "current opportunity belongs to another role or principal")}, http.StatusConflict
	}
	if req.OpportunityID != "" && req.OpportunityID != turn.opportunity.OpportunityID {
		return map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("wrong_opportunity", "opportunity_id does not match the active opportunity")}, http.StatusConflict
	}
	switch req.Tool {
	case "send_work_notes":
		result, err := api.r.writeWorkNotes(turn, stringOrDefault(req.Arguments["notes"], ""))
		if err != nil {
			return map[string]any{"ok": false, "error": roleAPIError("write_notes_failed", err.Error())}, http.StatusInternalServerError
		}
		return map[string]any{"ok": true, "case_id": api.caseID(), "status": "active", "result": result, "opportunity": api.opportunityPayloadLocked(turn, false)}, http.StatusOK
	case "submit_decision":
		result, errResponse := api.submitDecisionLocked(turn, req.Arguments)
		if errResponse != nil {
			return errResponse, http.StatusOK
		}
		return map[string]any{"ok": true, "case_id": api.caseID(), "status": "accepted", "result": result}, http.StatusOK
	default:
		result, status := api.executeSupportToolLocked(turn, req.Tool, req.Arguments)
		ok := status == http.StatusOK
		if resultOK, exists := result["ok"].(bool); exists {
			ok = ok && resultOK
		}
		return map[string]any{"ok": ok, "case_id": api.caseID(), "status": "active", "result": result, "opportunity": api.opportunityPayloadLocked(turn, false)}, status
	}
}

func (api *roleAPIServer) failLocked(req roleAPIRequest) (map[string]any, int) {
	if api.expireActiveTurnLocked(time.Now()) {
		return map[string]any{"ok": false, "case_id": api.caseID(), "status": "expired", "error": roleAPIError("opportunity_expired", "the active opportunity deadline has passed")}, http.StatusConflict
	}
	if api.active == nil || api.active.completed {
		return map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("no_active_opportunity", "no active opportunity")}, http.StatusConflict
	}
	turn := api.active
	if !api.turnMatchesActor(turn, req) {
		return map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("wrong_turn", "current opportunity belongs to another role or principal")}, http.StatusConflict
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = "external agent reported failure"
	}
	err := fmt.Errorf("%s", message)
	if turn.role.Name == "juror" {
		log, handled, handleErr := api.r.handleOpportunityResponseError(turn.turnIndex, turn.role, turn.opportunity, "", err)
		if handled {
			api.finishTurnLocked(turn, log, handleErr)
			return map[string]any{"ok": handleErr == nil, "case_id": api.caseID(), "status": "accepted", "error": errorString(handleErr)}, http.StatusOK
		}
	}
	api.finishTurnLocked(turn, TurnLog{}, fmt.Errorf("external role %s failed: %s", turn.role.Name, message))
	return map[string]any{"ok": true, "case_id": api.caseID(), "status": "failed_recorded"}, http.StatusOK
}

func (api *roleAPIServer) executeSupportToolLocked(turn *externalOpportunityTurn, tool string, args map[string]any) (map[string]any, int) {
	if !contains(referenceToolsForRole(turn.role), tool) && tool != "read_case_file_bytes" {
		return map[string]any{"ok": false, "error": "tool is not available for this role", "tool": tool}, http.StatusForbidden
	}
	if turn.supportUsed >= turn.supportBudget {
		actorMessage, err := api.r.prompts.Text(adcprompts.RuntimeCorrectionSupportID)
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, http.StatusInternalServerError
		}
		return map[string]any{
			"ok":            false,
			"error":         "support-tool budget exhausted",
			"actor_message": actorMessage,
		}, http.StatusOK
	}
	turn.supportUsed++
	turn.stepsUsed++
	var execRes ActionExecution
	var err error
	if tool == "read_case_file_bytes" {
		result, readErr := api.r.readCaseFileBytes(turn.role.Name, strings.TrimSpace(stringOrDefault(args["file_id"], "")))
		if readErr != nil {
			return map[string]any{"ok": false, "error": readErr.Error()}, http.StatusInternalServerError
		}
		execRes = ActionExecution{Result: result}
	} else {
		execRes, err = api.r.executeAction(turn.turnIndex, turn.stepsUsed, turn.role.Name, tool, args)
	}
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, http.StatusInternalServerError
	}
	result := execRes.Result
	if len(execRes.FollowupInputItems) > 0 {
		result = cloneJSONMap(result)
		result["content_items"] = execRes.FollowupInputItems
	}
	turn.transcript = append(turn.transcript, map[string]any{"action": tool, "arguments": args, "result": result})
	return result, http.StatusOK
}

func (api *roleAPIServer) submitDecisionLocked(turn *externalOpportunityTurn, args map[string]any) (map[string]any, map[string]any) {
	if turn.attemptsRemaining <= 0 {
		return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("attempts_exhausted", "no decision attempts remain")}
	}
	turn.stepsUsed++
	decision, err := roleAPIDecisionFromParams(args)
	if err != nil {
		return nil, api.rejectDecisionLocked(turn, err)
	}
	if strings.TrimSpace(stringOrDefault(decision["kind"], "")) == "tool" {
		toolName := strings.TrimSpace(stringOrDefault(decision["tool_name"], ""))
		payload, _ := decision["payload"].(map[string]any)
		merged, issue, renderErr := api.r.applyOpportunityPayloadDefaults(toolName, payload, turn.opportunity)
		if renderErr != nil {
			return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("prompt_render_failed", renderErr.Error())}
		}
		if issue != nil {
			return nil, api.rejectDecisionLocked(turn, fmt.Errorf("%s", issueText(*issue)))
		}
		decision["payload"] = merged
	}
	acceptResp, err := api.r.lean.ApplyDecision(api.r.state, turn.stateVersion, turn.opportunity.OpportunityID, turn.role.Name, decision, turn.rolesPayload, turn.opportunity.StepBudget)
	if err != nil {
		return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("apply_decision_failed", err.Error())}
	}
	turn.transcript = append(turn.transcript, map[string]any{"decision": decision, "acceptance": acceptResp})
	if ok, _ := acceptResp["ok"].(bool); !ok {
		return nil, api.rejectDecisionLocked(turn, fmt.Errorf("%s", issueText(issueFromResult("submit_decision", acceptResp))))
	}
	resultKind := strings.TrimSpace(stringOrDefault(acceptResp["result_kind"], ""))
	switch resultKind {
	case "pass_recorded":
		state, _ := acceptResp["state"].(map[string]any)
		if state == nil {
			return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("bad_lean_response", "pass_recorded missing state")}
		}
		if err := api.r.recordApplyDecisionForCertificate(turn.stateVersion, turn.opportunity.OpportunityID, turn.role.Name, decision, turn.rolesPayload, turn.opportunity.StepBudget); err != nil {
			return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("certificate_record_failed", err.Error())}
		}
		api.r.state = mergeLocalCaseExtensions(api.r.state, state)
		if err := api.r.persistActionEvent(turn.turnIndex, turn.stepsUsed, turn.role.Name, "pass_turn", args, acceptResp); err != nil {
			return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("persist_action_failed", err.Error())}
		}
		turn.transcript = append(turn.transcript, map[string]any{"action": "pass_turn", "arguments": args, "result": acceptResp})
		log := TurnLog{Role: turn.role.Name, Prompt: turn.opportunity.Objective, Steps: turn.stepsUsed, Transcript: turn.transcript}
		api.finishTurnLocked(turn, log, nil)
		return map[string]any{"acceptance": acceptResp}, nil
	case "execute_tool":
		action, _ := acceptResp["action"].(map[string]any)
		if action == nil {
			return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("bad_lean_response", "execute_tool missing action")}
		}
		actionType := strings.TrimSpace(stringOrDefault(action["action_type"], ""))
		actorRole := strings.TrimSpace(stringOrDefault(action["actor_role"], turn.role.Name))
		payload, _ := action["payload"].(map[string]any)
		if payload == nil {
			payload = map[string]any{}
		}
		execRes, err := api.r.executeAction(turn.turnIndex, turn.stepsUsed, actorRole, actionType, payload)
		if err != nil {
			return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("execute_action_failed", err.Error())}
		}
		res := execRes.Result
		turn.transcript = append(turn.transcript, map[string]any{"action": actionType, "arguments": payload, "result": res})
		if ok, _ := res["ok"].(bool); !ok {
			return nil, api.rejectDecisionLocked(turn, fmt.Errorf("%s", issueText(issueFromResult(actionType, res))))
		}
		log := TurnLog{Role: turn.role.Name, Prompt: turn.opportunity.Objective, Steps: turn.stepsUsed, Transcript: turn.transcript}
		api.finishTurnLocked(turn, log, nil)
		return map[string]any{"acceptance": acceptResp, "result": res}, nil
	default:
		return nil, map[string]any{"ok": false, "case_id": api.caseID(), "error": roleAPIError("bad_lean_response", "unsupported result_kind: "+resultKind)}
	}
}

func (api *roleAPIServer) rejectDecisionLocked(turn *externalOpportunityTurn, err error) map[string]any {
	reason := strings.TrimSpace(errorString(err))
	if reason == "" {
		reason = "invalid decision"
	}
	turn.invalidReasons = append(turn.invalidReasons, reason)
	turn.attemptsRemaining--
	response := map[string]any{
		"ok":                 false,
		"case_id":            api.caseID(),
		"status":             "active",
		"attempts_remaining": turn.attemptsRemaining,
		"attempts_max":       turn.attemptsMax,
		"error":              roleAPIError("invalid_decision", reason),
	}
	if turn.attemptsRemaining <= 0 {
		limitErr := formatInvalidAttemptLimitError(fmt.Sprintf("role API role=%s", turn.role.Name), turn.invalidReasons)
		api.finishTurnLocked(turn, TurnLog{}, limitErr)
		response["status"] = "failed"
		response["error"] = roleAPIError("attempts_exhausted", limitErr.Error())
	}
	return response
}

func (api *roleAPIServer) finishTurnLocked(turn *externalOpportunityTurn, log TurnLog, err error) {
	if turn == nil || turn.completed {
		return
	}
	turn.completed = true
	api.signalChangedLocked()
	select {
	case turn.done <- externalOpportunityResult{log: log, err: err}:
	default:
	}
}

func roleAPIDecisionFromParams(params map[string]any) (map[string]any, error) {
	kind := strings.TrimSpace(stringOrDefault(params["kind"], ""))
	switch kind {
	case "pass":
		return map[string]any{"kind": "pass", "reason": strings.TrimSpace(stringOrDefault(params["reason"], ""))}, nil
	case "tool":
		toolName := strings.TrimSpace(stringOrDefault(params["tool_name"], ""))
		if toolName == "" {
			return nil, fmt.Errorf("submit_decision requires tool_name when kind=tool")
		}
		payload, _ := params["payload"].(map[string]any)
		if payload == nil {
			payload = map[string]any{}
		}
		return map[string]any{"kind": "tool", "tool_name": toolName, "payload": payload}, nil
	default:
		return nil, fmt.Errorf("submit_decision kind must be tool or pass")
	}
}

func (r *Runner) buildRoleAPIPrompt(role spec.RoleSpec, view map[string]any, opportunity leanOpportunity, deadline time.Time, timeout time.Duration, attemptsMax int, supportBudget int, availableToolSpecs []map[string]any) (string, error) {
	systemPrompt, err := r.buildSystemPrompt(role, view)
	if err != nil {
		return "", err
	}
	if role.Name == "juror" {
		caseObj, _ := r.state["case"].(map[string]any)
		_, jurorPersona := r.jurorOpportunityPromptContext(opportunity)
		systemPrompt, err = r.buildJurorSystemPrompt(role, opportunity, jurorPersona, caseObj)
		if err != nil {
			return "", err
		}
	}
	opportunityPrompt, err := r.buildOpportunityPrompt(role, opportunity)
	if err != nil {
		return "", err
	}
	deadlineText := "(none)"
	if !deadline.IsZero() {
		deadlineText = deadline.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	legalSchemaLines := r.legalToolSchemaLines(opportunity.AllowedTools)
	cards, err := r.collectToolCards(role.Name, opportunity.AllowedTools)
	if err != nil {
		return "", err
	}
	toolLines := make([]string, 0, len(availableToolSpecs))
	for _, spec := range availableToolSpecs {
		name := strings.TrimSpace(stringOrDefault(spec["name"], ""))
		description := strings.TrimSpace(stringOrDefault(spec["description"], ""))
		if name != "" && description != "" {
			toolLines = append(toolLines, "- "+name+": "+description)
		}
	}
	return r.prompts.Render(adcprompts.RuntimeExternalRoleID, map[string]string{
		"{{SYSTEM_PROMPT}}":       systemPrompt,
		"{{OPPORTUNITY_PROMPT}}":  opportunityPrompt,
		"{{DEADLINE}}":            deadlineText,
		"{{TIMEOUT}}":             timeout.String(),
		"{{DECISION_ATTEMPTS}}":   fmt.Sprintf("%d", attemptsMax),
		"{{SUPPORT_BUDGET}}":      fmt.Sprintf("%d", supportBudget),
		"{{LEGAL_TOOLS}}":         promptList(opportunity.AllowedTools),
		"{{CONSTRAINTS}}":         promptJSON(opportunity.Constraints),
		"{{PASS_ACTION}}":         passAction(opportunity.MayPass),
		"{{LEGAL_TOOL_SCHEMAS}}":  promptSections(legalSchemaLines),
		"{{LEGAL_TOOL_GUIDANCE}}": promptSections(cards),
		"{{SUPPORT_TOOLS}}":       promptSections(toolLines),
	})
}

func (r *Runner) jurorAgentPayload(turn *externalOpportunityTurn) map[string]any {
	if turn == nil || turn.role.Name != "juror" {
		return nil
	}
	jurorID := strings.TrimSpace(turn.principalID)
	if jurorID == "" {
		return nil
	}
	pair, ok := r.jurorPersonaAssignments[jurorID]
	if !ok {
		return nil
	}
	payload := map[string]any{
		"juror_id":     jurorID,
		"model":        pair.Model,
		"persona_file": pair.PersonaFile,
	}
	if pair.RequestSpec != nil {
		payload["request_spec"] = pair.RequestSpec
	}
	return payload
}

func (r *Runner) roleAPIToolSpecs(role spec.RoleSpec, opportunity leanOpportunity) ([]map[string]any, error) {
	names := []string{"case_status", "send_work_notes"}
	for _, name := range referenceToolsForRole(role) {
		names = appendIfMissing(names, name)
	}
	names = appendIfMissing(names, "read_case_file_bytes")
	names = appendIfMissing(names, "submit_decision")
	specs := make([]map[string]any, 0, len(names))
	for _, name := range names {
		spec, err := r.roleAPIToolSpec(name, opportunity.MayPass)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func (r *Runner) roleAPIToolSpec(name string, mayPass bool) (map[string]any, error) {
	descriptionID, ok := adcprompts.RoleAPIToolDescriptionID(name)
	if !ok {
		return nil, fmt.Errorf("missing role API tool description prompt for %q", name)
	}
	descriptionValues := map[string]string{}
	if name == "submit_decision" {
		descriptionValues["{{PASS_ACTION}}"] = passAction(mayPass)
	}
	description, err := r.prompts.Render(descriptionID, descriptionValues)
	if err != nil {
		return nil, err
	}
	switch name {
	case "case_status":
		return simpleToolSpec(name, description, map[string]any{}), nil
	case "send_work_notes":
		return simpleToolSpec(name, description, map[string]any{"notes": map[string]any{"type": "string"}}), nil
	case "read_case_file_bytes":
		return simpleToolSpec(name, description, map[string]any{"file_id": map[string]any{"type": "string"}}), nil
	case "submit_decision":
		return simpleToolSpec(name, description, map[string]any{
			"kind":      map[string]any{"type": "string", "enum": []string{"tool", "pass"}},
			"tool_name": map[string]any{"type": "string"},
			"payload":   map[string]any{"type": "object"},
			"reason":    map[string]any{"type": "string"},
		}), nil
	default:
		schema := toolSchema(name)
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
		}
		return map[string]any{"name": name, "description": description, "parameters": schema}, nil
	}
}

func simpleToolSpec(name string, description string, properties map[string]any) map[string]any {
	required := make([]string, 0)
	for key := range properties {
		if key != "reason" && key != "payload" && key != "tool_name" {
			required = append(required, key)
		}
	}
	sort.Strings(required)
	return map[string]any{
		"name":        name,
		"description": description,
		"parameters": map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
	}
}

func (r *Runner) legalToolSpecs(allowedTools []string) []map[string]any {
	specs := make([]map[string]any, 0, len(allowedTools))
	seen := map[string]bool{}
	for _, name := range allowedTools {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		schema := r.toolSchema(name)
		if schema == nil {
			continue
		}
		specs = append(specs, map[string]any{"name": name, "parameters": schema})
	}
	return specs
}

func (r *Runner) writeWorkNotes(turn *externalOpportunityTurn, notes string) (result map[string]any, err error) {
	if turn == nil {
		return nil, fmt.Errorf("active turn is required")
	}
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return nil, fmt.Errorf("notes are required")
	}
	path := r.workNotesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create work notes dir: %w", err)
	}
	record := workNoteRecord{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		CaseID:        r.roleAPI.caseID(),
		RunID:         strings.TrimSpace(r.cfg.RunID),
		RoleID:        turn.role.Name,
		PrincipalID:   turn.principalID,
		OpportunityID: turn.opportunity.OpportunityID,
		Notes:         notes,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open work notes: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close work notes: %w", closeErr))
		}
	}()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return nil, fmt.Errorf("write work notes: %w", err)
	}
	return map[string]any{"ok": true, "path": path}, nil
}

func (r *Runner) workNotesPath() string {
	if strings.TrimSpace(r.cfg.OutputPath) != "" {
		return filepath.Join(filepath.Dir(r.cfg.OutputPath), roleAPIWorkNotesFilename)
	}
	return filepath.Join(r.cfg.ScenarioBaseDir, roleAPIWorkNotesFilename)
}

func (r *Runner) readCaseFileBytes(actorRole string, fileID string) (map[string]any, error) {
	if strings.TrimSpace(fileID) == "" {
		return map[string]any{"ok": false, "error": "file_id is required"}, nil
	}
	caseObj, _ := r.state["case"].(map[string]any)
	if caseObj == nil {
		return map[string]any{"ok": false, "error": "state.case missing"}, nil
	}
	visibleFiles, err := r.visibleCaseFilesForRole(actorRole)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	visibleFile := visibleCaseFileByID(visibleFiles, fileID)
	if visibleFile == nil {
		return r.unknownCaseFileResult(fileID, visibleFiles)
	}
	internalFile := findCaseFile(caseObj, fileID)
	if internalFile == nil {
		return map[string]any{"ok": false, "error": "internal case file missing for visible file_id=" + fileID}, nil
	}
	storedPath := strings.TrimSpace(stringOrDefault(internalFile["storage_relpath"], ""))
	if storedPath == "" {
		return map[string]any{"ok": false, "error": "stored path missing for case file"}, nil
	}
	raw, err := r.readCaseFile(internalFile)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	return map[string]any{
		"ok":             true,
		"file":           enrichVisibleCaseFile(caseObj, visibleFile),
		"content_base64": base64.StdEncoding.EncodeToString(raw),
		"size_bytes":     len(raw),
		"mime_type":      caseFileMIMEType(internalFile),
	}, nil
}

func principalIDForOpportunity(role string, opportunity leanOpportunity) string {
	if strings.TrimSpace(role) != "juror" {
		return ""
	}
	return targetJurorIDForOpportunity(opportunity)
}

func roleAPIError(code string, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

func writeRoleAPIJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		recorder, ok := w.(interface{ recordResponseError(error) })
		if !ok {
			panic(fmt.Errorf("write case API response: %w", err))
		}
		recorder.recordResponseError(err)
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
