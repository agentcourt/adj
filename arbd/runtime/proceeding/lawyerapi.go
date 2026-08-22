package proceeding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const lawyerAPIBasePath = "/lawyerapi/v1"

const (
	defaultLawyerAPIWaitTimeout = 30 * time.Second
	maxLawyerAPIWaitTimeout     = 5 * time.Minute
)

type lawyerAPIServer struct {
	rc *runContext

	cond          *sync.Cond
	version       uint64
	active        *lawyerTurn
	evidenceFiles evidenceFileOperations
}

type lawyerTurn struct {
	opportunity       Opportunity
	turnNumber        int
	prompt            string
	deadline          time.Time
	attemptsMax       int
	attemptsRemaining int
	invalidReasons    []string
	evidenceBudget    *evidenceReadBudget
	completed         bool
	done              chan error
}

type lawyerDoRequest struct {
	CaseID        string         `json:"case_id"`
	RoleID        string         `json:"role_id"`
	OpportunityID string         `json:"opportunity_id,omitempty"`
	Tool          string         `json:"tool"`
	Arguments     map[string]any `json:"arguments"`
	CallID        string         `json:"call_id,omitempty"`
}

func newLawyerAPIServer(rc *runContext) *lawyerAPIServer {
	api := &lawyerAPIServer{
		rc: rc,
	}
	api.cond = sync.NewCond(&rc.mu)
	return api
}

func (api *lawyerAPIServer) register(mux *http.ServeMux, caseCtx context.Context) {
	mux.HandleFunc(lawyerAPIBasePath+"/get", api.handleGet)
	mux.HandleFunc(lawyerAPIBasePath+"/wait", api.handleWait)
	mux.HandleFunc(lawyerAPIBasePath+"/status", api.handleStatus)
	mux.HandleFunc(lawyerAPIBasePath+"/result", api.handleResult)
	mux.HandleFunc(lawyerAPIBasePath+"/do", func(w http.ResponseWriter, r *http.Request) {
		api.handleDo(caseCtx, w, r)
	})
}

func (api *lawyerAPIServer) startTurnLocked(turn *lawyerTurn) error {
	if api.active != nil && !api.active.completed {
		return fmt.Errorf("lawyerapi already has an active turn")
	}
	api.active = turn
	api.signalChangedLocked()
	return nil
}

func (api *lawyerAPIServer) clearTurn(turn *lawyerTurn) {
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	if api.active == turn {
		api.active = nil
		api.signalChangedLocked()
	}
}

func (api *lawyerAPIServer) signalChangedLocked() {
	api.version++
	api.ensureCondLocked().Broadcast()
}

func (api *lawyerAPIServer) ensureCondLocked() *sync.Cond {
	if api.cond == nil {
		api.cond = sync.NewCond(&api.rc.mu)
	}
	return api.cond
}

func (rc *runContext) executeAttorneyOpportunity(ctx context.Context, _ any, opportunity Opportunity) error {
	if err := validateAttorneyRole(opportunity.Role); err != nil {
		return err
	}
	rc.mu.Lock()
	api := rc.lawyerAPI
	if api == nil {
		rc.mu.Unlock()
		return fmt.Errorf("lawyerapi server is not running")
	}
	prompt, err := rc.buildAttorneyPrompt(opportunity)
	if err != nil {
		rc.mu.Unlock()
		return err
	}
	timeout := rc.cfg.Runtime.LawyerTurnTimeout()
	turn := &lawyerTurn{
		opportunity:       opportunity,
		turnNumber:        rc.turn,
		prompt:            prompt,
		deadline:          time.Now().Add(timeout),
		attemptsMax:       rc.cfg.Runtime.InvalidAttemptLimit,
		attemptsRemaining: rc.cfg.Runtime.InvalidAttemptLimit,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
	if err := api.startTurnLocked(turn); err != nil {
		rc.mu.Unlock()
		return err
	}
	rc.mu.Unlock()
	defer api.clearTurn(turn)
	timer := time.NewTimer(time.Until(turn.deadline))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return api.timeoutTurn(ctx, turn, timeout)
	case err := <-turn.done:
		return err
	}
}

func (api *lawyerAPIServer) timeoutTurn(caseCtx context.Context, turn *lawyerTurn, timeout time.Duration) error {
	api.rc.mu.Lock()
	if turn == nil || api.active != turn {
		api.rc.mu.Unlock()
		return fmt.Errorf("lawyerapi timed-out turn is no longer active")
	}
	if turn.completed {
		done := turn.done
		api.rc.mu.Unlock()
		return <-done
	}
	opportunity := turn.opportunity
	err := fmt.Errorf("%s lawyer opportunity timed out after %s", opportunity.Role, timeout)
	if failErr := api.expireTurnLocked(caseCtx, turn, err.Error()); failErr != nil {
		api.rc.mu.Unlock()
		return failErr
	}
	api.rc.mu.Unlock()
	return nil
}

func (api *lawyerAPIServer) finishTurn(turn *lawyerTurn, err error) {
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	api.finishTurnLocked(turn, err)
}

func (api *lawyerAPIServer) finishTurnLocked(turn *lawyerTurn, err error) {
	api.completeTurnLocked(turn, err, true)
}

func (api *lawyerAPIServer) finishTurnAfterSignalLocked(turn *lawyerTurn, err error) {
	api.completeTurnLocked(turn, err, false)
}

func (api *lawyerAPIServer) completeTurnLocked(turn *lawyerTurn, err error, signal bool) {
	if turn == nil || turn.completed {
		return
	}
	turn.completed = true
	if signal {
		api.signalChangedLocked()
	}
	select {
	case turn.done <- err:
	default:
	}
}

func (api *lawyerAPIServer) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLawyerJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role_id"))
	caseID := strings.TrimSpace(r.URL.Query().Get("case_id"))
	if caseID == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(caseID) {
		api.writeCaseMismatch(w, caseID, role)
		return
	}
	if role == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_role_id", "role_id is required"),
		})
		return
	}
	if role == "observer" {
		api.rc.mu.Lock()
		response := api.statusResponseLocked(caseID, role)
		api.rc.mu.Unlock()
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	if err := validateAttorneyRole(role); err != nil {
		writeLawyerJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"case_id": caseID,
			"role_id": role,
			"error":   apiError("invalid_role", err.Error()),
		})
		return
	}
	api.rc.mu.Lock()
	response := api.statusResponseLocked(caseID, role)
	api.rc.mu.Unlock()
	writeLawyerJSON(w, http.StatusOK, response)
}

func (api *lawyerAPIServer) handleWait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLawyerJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role_id"))
	caseID := strings.TrimSpace(r.URL.Query().Get("case_id"))
	after := strings.TrimSpace(r.URL.Query().Get("after"))
	afterVersion, hasAfterVersion, err := parseOptionalUintQuery(r.URL.Query().Get("after_version"), "after_version")
	if err != nil {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_after_version", err.Error()),
		})
		return
	}
	timeout, err := parseLawyerAPIWaitTimeout(r.URL.Query().Get("timeout_ms"))
	if err != nil {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_timeout", err.Error()),
		})
		return
	}
	if caseID == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(caseID) {
		api.writeCaseMismatch(w, caseID, role)
		return
	}
	if role == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_role_id", "role_id is required"),
		})
		return
	}
	if role != "observer" {
		if err := validateAttorneyRole(role); err != nil {
			writeLawyerJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"case_id": caseID,
				"role_id": role,
				"error":   apiError("invalid_role", err.Error()),
			})
			return
		}
	}

	api.rc.mu.Lock()
	cond := api.ensureCondLocked()
	baseline := api.version
	if hasAfterVersion {
		baseline = afterVersion
	}
	deadline := time.Now().Add(timeout)
	timer := time.AfterFunc(timeout, func() {
		api.rc.mu.Lock()
		api.ensureCondLocked().Broadcast()
		api.rc.mu.Unlock()
	})
	defer timer.Stop()
	if done := r.Context().Done(); done != nil {
		go func() {
			<-done
			api.rc.mu.Lock()
			api.ensureCondLocked().Broadcast()
			api.rc.mu.Unlock()
		}()
	}
	for {
		if r.Context().Err() != nil {
			api.rc.mu.Unlock()
			return
		}
		if response, reason, ready := api.waitResponseLocked(caseID, role, after, baseline); ready {
			response["wait"] = api.waitPayloadLocked(reason)
			api.rc.mu.Unlock()
			writeLawyerJSON(w, http.StatusOK, response)
			return
		}
		if !time.Now().Before(deadline) {
			response := api.statusResponseLocked(caseID, role)
			response["wait"] = api.waitPayloadLocked("timeout")
			api.rc.mu.Unlock()
			writeLawyerJSON(w, http.StatusOK, response)
			return
		}
		cond.Wait()
	}
}

func (api *lawyerAPIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLawyerJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role_id"))
	caseID := strings.TrimSpace(r.URL.Query().Get("case_id"))
	if caseID == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(caseID) {
		api.writeCaseMismatch(w, caseID, role)
		return
	}
	if role == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": caseID,
			"error":   apiError("missing_role_id", "role_id is required"),
		})
		return
	}
	if role != "observer" {
		if err := validateAttorneyRole(role); err != nil {
			writeLawyerJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"case_id": caseID,
				"role_id": role,
				"error":   apiError("invalid_role", err.Error()),
			})
			return
		}
	}
	api.rc.mu.Lock()
	response, err := api.caseStatusResponseLocked(caseID, role)
	if err != nil {
		response = api.responseBaseLocked(caseID, role)
		response["ok"] = false
		response["error"] = apiError("runtime_failure", err.Error())
	}
	api.rc.mu.Unlock()
	if err != nil {
		writeLawyerJSON(w, http.StatusInternalServerError, response)
		return
	}
	writeLawyerJSON(w, http.StatusOK, response)
}

func (api *lawyerAPIServer) handleResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLawyerJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role_id"))
	caseID := strings.TrimSpace(r.URL.Query().Get("case_id"))
	if caseID == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(caseID) {
		api.writeCaseMismatch(w, caseID, role)
		return
	}
	if role == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": caseID,
			"error":   apiError("missing_role_id", "role_id is required"),
		})
		return
	}
	if role != "observer" {
		if err := validateAttorneyRole(role); err != nil {
			writeLawyerJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"case_id": caseID,
				"role_id": role,
				"error":   apiError("invalid_role", err.Error()),
			})
			return
		}
	}
	api.rc.mu.Lock()
	response := api.caseResultResponseLocked(caseID, role)
	api.rc.mu.Unlock()
	writeLawyerJSON(w, http.StatusOK, response)
}

func (api *lawyerAPIServer) handleDo(caseCtx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLawyerJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use POST"),
		})
		return
	}
	requestBodyLimit, err := lawyerDoRequestBodyLimit(
		int64(api.rc.cfg.Runtime.MaxResponseBytes),
		int64(api.rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes),
	)
	if err != nil {
		writeLawyerJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": apiError("runtime_failure", err.Error()),
		})
		return
	}
	var req lawyerDoRequest
	body := http.MaxBytesReader(w, r.Body, requestBodyLimit)
	dec := json.NewDecoder(body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			err = fmt.Errorf("request body is required")
		}
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_json", err.Error()),
		})
		return
	}
	if err := requireJSONEOF(dec); err != nil {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_json", err.Error()),
		})
		return
	}
	req.RoleID = strings.TrimSpace(req.RoleID)
	req.OpportunityID = strings.TrimSpace(req.OpportunityID)
	req.Tool = strings.TrimSpace(req.Tool)
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	req.CaseID = strings.TrimSpace(req.CaseID)
	if req.CaseID == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(req.CaseID) {
		api.writeCaseMismatch(w, req.CaseID, req.RoleID)
		return
	}
	if req.RoleID == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": req.CaseID,
			"error":   apiError("missing_role_id", "role_id is required"),
		})
		return
	}
	if req.Tool == "" {
		writeLawyerJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": strings.TrimSpace(req.CaseID),
			"role_id": req.RoleID,
			"error":   apiError("missing_tool", "tool is required"),
		})
		return
	}
	if req.RoleID == "observer" {
		response := api.handleObserverDo(req)
		if r.Context().Err() == nil {
			writeLawyerJSON(w, http.StatusOK, response)
		}
		return
	}
	status, response := api.handleLawyerDo(caseCtx, req)
	if r.Context().Err() == nil {
		writeLawyerJSON(w, status, response)
	}
}

func lawyerDoRequestBodyLimit(maxResponseBytes int64, maxDirectEvidenceBytes int64) (int64, error) {
	if maxResponseBytes <= 0 {
		return 0, fmt.Errorf("runtime.max_response_bytes must be positive")
	}
	if maxDirectEvidenceBytes <= 0 {
		return 0, fmt.Errorf("policy.max_direct_submitted_evidence_bytes must be positive")
	}
	const maxInt64 = uint64(1<<63 - 1)
	encodedEvidenceBytes := ((uint64(maxDirectEvidenceBytes) + 2) / 3) * 4
	if encodedEvidenceBytes > maxInt64 || uint64(maxResponseBytes) > maxInt64-encodedEvidenceBytes {
		return 0, fmt.Errorf(
			"lawyer /do request body limit exceeds the maximum supported size: runtime.max_response_bytes=%d, policy.max_direct_submitted_evidence_bytes=%d",
			maxResponseBytes,
			maxDirectEvidenceBytes,
		)
	}
	return int64(uint64(maxResponseBytes) + encodedEvidenceBytes), nil
}

func requireJSONEOF(dec *json.Decoder) error {
	var trailing any
	if err := dec.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return fmt.Errorf("request body must contain exactly one JSON value")
}

func (api *lawyerAPIServer) caseIDMatches(caseID string) bool {
	return strings.TrimSpace(caseID) == normalizeCaseID(api.rc.cfg.CaseID)
}

func (api *lawyerAPIServer) writeCaseMismatch(w http.ResponseWriter, caseID string, roleID string) {
	response := map[string]any{
		"ok":      false,
		"case_id": strings.TrimSpace(caseID),
		"error":   apiError("unknown_case", "case_id does not match this case process"),
	}
	if strings.TrimSpace(roleID) != "" {
		response["role_id"] = strings.TrimSpace(roleID)
	}
	writeLawyerJSON(w, http.StatusNotFound, response)
}

func (api *lawyerAPIServer) handleLawyerDo(caseCtx context.Context, req lawyerDoRequest) (int, map[string]any) {
	if err := validateAttorneyRole(req.RoleID); err != nil {
		return http.StatusForbidden, map[string]any{
			"ok":      false,
			"case_id": req.CaseID,
			"role_id": req.RoleID,
			"error":   apiError("invalid_role", err.Error()),
		}
	}
	if evidenceFileTool(req.Tool) {
		return http.StatusOK, api.lawyerEvidenceDoResponse(caseCtx, req)
	}
	api.rc.mu.Lock()
	response := api.lawyerDoResponseLocked(caseCtx, req)
	api.rc.mu.Unlock()
	return http.StatusOK, response
}

func (api *lawyerAPIServer) lawyerDoResponseLocked(caseCtx context.Context, req lawyerDoRequest) map[string]any {
	if req.Tool == "case_status" {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		if err := requireAllowedKeys(req.Arguments, "case_status arguments"); err != nil {
			response["ok"] = false
			response["error"] = apiError("tool_failed", err.Error())
			return response
		}
		result, err := api.caseStatusPayloadLocked(req.RoleID)
		if err != nil {
			response["ok"] = false
			response["error"] = apiError("runtime_failure", err.Error())
			return response
		}
		response["ok"] = true
		response["result"] = result
		return response
	}
	turn, response := api.lawyerRequestTurnLocked(caseCtx, req)
	if response != nil {
		return response
	}
	result, decisionAttempt, err := api.callLawyerToolLocked(caseCtx, turn, req.Tool, req.Arguments, req.CallID)
	if err != nil {
		return api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, decisionAttempt)
	}
	if !turn.completed && req.Tool != "submit_evidence" && req.Tool != "commit_evidence_upload" {
		if err := turnStepError(caseCtx, turn.deadline, nil); err != nil {
			return api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, false)
		}
	}
	response = api.responseBaseLocked(req.CaseID, req.RoleID)
	response["ok"] = true
	response["result"] = result
	return response
}

func (api *lawyerAPIServer) lawyerToolErrorResponseLocked(caseCtx context.Context, req lawyerDoRequest, turn *lawyerTurn, err error, decisionAttempt bool) map[string]any {
	if isParticipantInput(err) {
		if timingErr := turnStepError(caseCtx, turn.deadline, nil); timingErr != nil {
			err = timingErr
		} else {
			err = api.consumeAttemptLocked(caseCtx, turn, err, decisionAttempt)
		}
	}
	if errors.Is(err, errTurnDeadlineExceeded) {
		return api.lawyerDeadlineResponseLocked(caseCtx, req, turn)
	}
	code := "runtime_failure"
	if isParticipantInput(err) {
		code = "tool_failed"
	} else if !turn.completed {
		api.finishTurnLocked(turn, err)
	}
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	response["ok"] = false
	response["error"] = apiError(code, err.Error())
	return response
}

func (api *lawyerAPIServer) lawyerDeadlineResponseLocked(caseCtx context.Context, req lawyerDoRequest, turn *lawyerTurn) map[string]any {
	err := fmt.Errorf("%s lawyer opportunity timed out", turn.opportunity.Role)
	code := "turn_timeout"
	responseErr := err
	if failErr := api.expireTurnLocked(caseCtx, turn, err.Error()); failErr != nil {
		code = "runtime_failure"
		responseErr = failErr
	}
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	response["ok"] = false
	response["error"] = apiError(code, responseErr.Error())
	return response
}

func (api *lawyerAPIServer) expireTurnLocked(caseCtx context.Context, turn *lawyerTurn, message string) error {
	if turn == nil || turn.completed {
		return nil
	}
	if err := api.rc.failOpportunityLocked(caseCtx, time.Time{}, turn.opportunity, opportunityFailureDeadline, message, nil); err != nil {
		api.finishTurnLocked(turn, err)
		return err
	}
	api.finishTurnAfterSignalLocked(turn, nil)
	return nil
}

func (api *lawyerAPIServer) lawyerRequestTurnLocked(caseCtx context.Context, req lawyerDoRequest) (*lawyerTurn, map[string]any) {
	turn := api.active
	if turn == nil || turn.completed {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("no_active_turn", "no lawyer turn is active")
		return nil, response
	}
	if err := context.Cause(caseCtx); err != nil {
		api.finishTurnLocked(turn, err)
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("runtime_failure", err.Error())
		return nil, response
	}
	if !time.Now().Before(turn.deadline) {
		return nil, api.lawyerDeadlineResponseLocked(caseCtx, req, turn)
	}
	if turn.opportunity.Role != req.RoleID {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("not_current_turn", fmt.Sprintf("current turn belongs to %s", turn.opportunity.Role))
		return nil, response
	}
	if req.OpportunityID == "" {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("missing_opportunity_id", "opportunity_id is required for lawyer tool calls")
		return nil, response
	}
	if req.OpportunityID != turn.opportunity.ID {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("stale_opportunity", fmt.Sprintf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, turn.opportunity.ID))
		return nil, response
	}
	return turn, nil
}

func (api *lawyerAPIServer) handleObserverDo(req lawyerDoRequest) map[string]any {
	if evidenceFileTool(req.Tool) {
		return api.observerEvidenceDoResponse(req)
	}
	api.rc.mu.Lock()
	response := api.observerDoResponseLocked(req)
	api.rc.mu.Unlock()
	return response
}

func (api *lawyerAPIServer) observerDoResponseLocked(req lawyerDoRequest) map[string]any {
	result, err := api.callObserverToolLocked(req.Tool, req.Arguments)
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	if err != nil {
		response["ok"] = false
		response["error"] = apiError(roleAPIToolErrorCode(err), err.Error())
		return response
	}
	response["ok"] = true
	response["result"] = result
	return response
}

func evidenceFileTool(tool string) bool {
	return tool == "stat_evidence" || tool == "read_evidence_range"
}

func (api *lawyerAPIServer) lawyerEvidenceDoResponse(caseCtx context.Context, req lawyerDoRequest) map[string]any {
	api.rc.mu.Lock()
	turn, response := api.lawyerRequestTurnLocked(caseCtx, req)
	if response != nil {
		api.rc.mu.Unlock()
		return response
	}
	if !evidenceReadAllowed(turn.opportunity) {
		response := api.lawyerToolErrorResponseLocked(caseCtx, req, turn, participantInput(fmt.Errorf("evidence access is not allowed in phase %q", turn.opportunity.Phase)), false)
		api.rc.mu.Unlock()
		return response
	}
	if err := validateEvidenceFileToolArguments(req.Tool, req.Arguments); err != nil {
		response := api.lawyerToolErrorResponseLocked(caseCtx, req, turn, participantInput(err), false)
		api.rc.mu.Unlock()
		return response
	}
	if req.Tool == "stat_evidence" {
		file, err := api.rc.evidenceFileSnapshotLocked(mapString(req.Arguments["evidence_id"]))
		if err != nil {
			response := api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, false)
			api.rc.mu.Unlock()
			return response
		}
		api.rc.mu.Unlock()
		err = api.evidenceFiles.verifyFile(file)
		api.rc.mu.Lock()
		defer api.rc.mu.Unlock()
		if response := api.revalidateLawyerEvidenceTurnLocked(caseCtx, req, turn, nil); response != nil {
			return response
		}
		if err != nil {
			return api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, false)
		}
		response = api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = true
		response["result"] = map[string]any{
			"evidence": file.meta,
			"limits":   api.evidenceReadLimitsLocked(turn),
		}
		return response
	}

	offset, err := requiredIntParam(req.Arguments, "offset")
	if err != nil {
		response := api.lawyerToolErrorResponseLocked(caseCtx, req, turn, participantInput(err), false)
		api.rc.mu.Unlock()
		return response
	}
	length, err := requiredIntParam(req.Arguments, "length")
	if err != nil {
		response := api.lawyerToolErrorResponseLocked(caseCtx, req, turn, participantInput(err), false)
		api.rc.mu.Unlock()
		return response
	}
	reservation, err := api.rc.reserveEvidenceReadLocked(mapString(req.Arguments["evidence_id"]), int64(offset), length, turn.evidenceBudget)
	if err != nil {
		response := api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, false)
		api.rc.mu.Unlock()
		return response
	}
	api.rc.mu.Unlock()
	result, bytesRead, readErr := api.evidenceFiles.readRange(reservation)
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	if response := api.revalidateLawyerEvidenceTurnLocked(caseCtx, req, turn, &reservation); response != nil {
		return response
	}
	if readErr != nil {
		rollbackEvidenceReadLocked(&reservation)
		return api.lawyerToolErrorResponseLocked(caseCtx, req, turn, readErr, false)
	}
	finalizeEvidenceReadLocked(&reservation, bytesRead)
	result["remaining_read_bytes_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes)
	result["remaining_reads_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads)
	if err := api.rc.recordEventAtTurnLocked(turn.turnNumber, "evidence_read", turn.opportunity.Role, turn.opportunity.Phase, map[string]any{
		"evidence_id": result["evidence_id"],
		"offset":      result["offset"],
		"length":      result["length"],
		"byte_count":  result["length"],
	}); err != nil {
		api.rc.signalRoleAPIsLocked()
		api.finishTurnAfterSignalLocked(turn, err)
		return api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, false)
	}
	api.rc.signalRoleAPIsLocked()
	response = api.responseBaseLocked(req.CaseID, req.RoleID)
	response["ok"] = true
	response["result"] = result
	return response
}

func (api *lawyerAPIServer) revalidateLawyerEvidenceTurnLocked(caseCtx context.Context, req lawyerDoRequest, turn *lawyerTurn, reservation *evidenceReadReservation) map[string]any {
	if api.active != turn || turn.completed {
		rollbackEvidenceReadLocked(reservation)
		return api.staleLawyerEvidenceResponseLocked(req, turn)
	}
	if err := context.Cause(caseCtx); err != nil {
		rollbackEvidenceReadLocked(reservation)
		return api.lawyerToolErrorResponseLocked(caseCtx, req, turn, err, false)
	}
	if !time.Now().Before(turn.deadline) {
		rollbackEvidenceReadLocked(reservation)
		_, response := api.lawyerRequestTurnLocked(caseCtx, req)
		return response
	}
	return nil
}

func (api *lawyerAPIServer) staleLawyerEvidenceResponseLocked(req lawyerDoRequest, turn *lawyerTurn) map[string]any {
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	response["ok"] = false
	response["error"] = apiError("stale_opportunity", fmt.Sprintf("evidence file operation completed after opportunity %q ended", turn.opportunity.ID))
	return response
}

func (api *lawyerAPIServer) observerEvidenceDoResponse(req lawyerDoRequest) map[string]any {
	api.rc.mu.Lock()
	if err := validateEvidenceFileToolArguments(req.Tool, req.Arguments); err != nil {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
		api.rc.mu.Unlock()
		return response
	}
	if req.Tool == "stat_evidence" {
		file, err := api.rc.evidenceFileSnapshotLocked(mapString(req.Arguments["evidence_id"]))
		if err != nil {
			response := api.responseBaseLocked(req.CaseID, req.RoleID)
			response["ok"] = false
			response["error"] = apiError(roleAPIToolErrorCode(err), err.Error())
			api.rc.mu.Unlock()
			return response
		}
		api.rc.mu.Unlock()
		err = api.evidenceFiles.verifyFile(file)
		api.rc.mu.Lock()
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		api.rc.mu.Unlock()
		if err != nil {
			response["ok"] = false
			response["error"] = apiError(roleAPIToolErrorCode(err), err.Error())
			return response
		}
		response["ok"] = true
		response["result"] = map[string]any{"evidence": file.meta}
		return response
	}

	offset, err := requiredIntParam(req.Arguments, "offset")
	if err != nil {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
		api.rc.mu.Unlock()
		return response
	}
	length, err := requiredIntParam(req.Arguments, "length")
	if err != nil {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
		api.rc.mu.Unlock()
		return response
	}
	reservation, err := api.rc.reserveEvidenceReadLocked(mapString(req.Arguments["evidence_id"]), int64(offset), length, nil)
	if err != nil {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError(roleAPIToolErrorCode(err), err.Error())
		api.rc.mu.Unlock()
		return response
	}
	api.rc.mu.Unlock()
	result, _, readErr := api.evidenceFiles.readRange(reservation)
	api.rc.mu.Lock()
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	api.rc.mu.Unlock()
	if readErr != nil {
		response["ok"] = false
		response["error"] = apiError("runtime_failure", readErr.Error())
		return response
	}
	response["ok"] = true
	response["result"] = result
	return response
}

func (api *lawyerAPIServer) callLawyerToolLocked(caseCtx context.Context, turn *lawyerTurn, tool string, args map[string]any, callID string) (map[string]any, bool, error) {
	switch tool {
	case "case_status":
		if err := requireAllowedKeys(args, "case_status arguments"); err != nil {
			return nil, false, participantInput(err)
		}
		result, err := api.caseStatusPayloadLocked(turn.opportunity.Role)
		return result, false, err
	case "get_case":
		if err := requireAllowedKeys(args, "get_case arguments"); err != nil {
			return nil, false, participantInput(err)
		}
		view := api.rc.attorneyView(turn.opportunity)
		return map[string]any{"case": view}, false, nil
	case "send_work_notes":
		if err := requireAllowedKeys(args, "send_work_notes arguments", "notes"); err != nil {
			return nil, false, participantInput(err)
		}
		notes, ok := args["notes"].(string)
		if !ok {
			return nil, false, participantInput(fmt.Errorf("arguments.notes is required and must be a string"))
		}
		if err := api.rc.recordWorkNotesAtTurn(turn.turnNumber, turn.opportunity, callID, notes); err != nil {
			return nil, false, err
		}
		return map[string]any{
			"text":       "Work notes accepted off record.",
			"byte_count": len([]byte(notes)),
		}, false, nil
	case "list_evidence":
		if err := requireAllowedKeys(args, "list_evidence arguments"); err != nil {
			return nil, false, participantInput(err)
		}
		if !evidenceReadAllowed(turn.opportunity) {
			return nil, false, participantInput(fmt.Errorf("evidence access is not allowed in phase %q", turn.opportunity.Phase))
		}
		return map[string]any{"evidence": api.rc.listVisibleEvidence()}, false, nil
	case "stat_evidence", "read_evidence_range":
		return nil, false, fmt.Errorf("evidence file tool %q requires unlocked execution", tool)
	case "begin_evidence_upload":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, false, participantInput(fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase))
		}
		session, err := api.rc.beginEvidenceUpload(turn.opportunity, args)
		if err != nil {
			return nil, false, err
		}
		return map[string]any{
			"upload_id":              session.UploadID,
			"max_chunk_bytes":        api.rc.cfg.Policy.MaxEvidenceChunkBytes,
			"remaining_upload_bytes": session.ExpectedSizeBytes,
		}, false, nil
	case "write_evidence_chunk":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, false, participantInput(fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase))
		}
		if err := requireAllowedKeys(args, "write_evidence_chunk arguments", "upload_id", "offset", "content_base64"); err != nil {
			return nil, false, participantInput(err)
		}
		offset, err := requiredIntParam(args, "offset")
		if err != nil {
			return nil, false, participantInput(err)
		}
		uploadID, err := optionalStringParam(args, "upload_id")
		if err != nil {
			return nil, false, participantInput(err)
		}
		contentBase64, err := optionalStringParam(args, "content_base64")
		if err != nil {
			return nil, false, participantInput(err)
		}
		session, n, err := api.rc.writeEvidenceChunk(turn.opportunity, uploadID, offset, contentBase64)
		if err != nil {
			return nil, false, err
		}
		return map[string]any{
			"upload_id":              session.UploadID,
			"accepted_offset":        session.ReceivedBytes - n,
			"accepted_length":        n,
			"received_bytes":         session.ReceivedBytes,
			"remaining_upload_bytes": remainingCapacity(session.ExpectedSizeBytes, session.ReceivedBytes),
		}, false, nil
	case "commit_evidence_upload":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, false, participantInput(fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase))
		}
		result, err := api.commitEvidenceUploadLocked(caseCtx, turn, args)
		return result, false, err
	case "submit_evidence":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, false, participantInput(fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase))
		}
		result, err := api.submitEvidenceLocked(caseCtx, turn, args)
		return result, false, err
	case "submit_decision":
		result, err := api.submitDecisionLocked(caseCtx, turn, args)
		return result, true, err
	default:
		return nil, false, participantInput(fmt.Errorf("unknown tool %q", tool))
	}
}

func (api *lawyerAPIServer) commitEvidenceUploadLocked(caseCtx context.Context, turn *lawyerTurn, args map[string]any) (map[string]any, error) {
	if err := requireAllowedKeys(args, "commit_evidence_upload arguments", "upload_id", "expected_sha256", "preferred_filename_ext"); err != nil {
		return nil, participantInput(err)
	}
	uploadID, err := optionalStringParam(args, "upload_id")
	if err != nil {
		return nil, participantInput(err)
	}
	preferredExt, err := optionalStringParam(args, "preferred_filename_ext")
	if err != nil {
		return nil, participantInput(err)
	}
	expectedSHA256, err := optionalStringParam(args, "expected_sha256")
	if err != nil {
		return nil, participantInput(err)
	}
	session := api.rc.uploadSessions[uploadID]
	if session == nil {
		return nil, participantInput(fmt.Errorf("unknown upload_id %q", uploadID))
	}
	meta, err := api.rc.prepareEvidenceUploadCommit(
		turn.opportunity,
		session,
		preferredExt,
		expectedSHA256,
	)
	if err != nil {
		return nil, err
	}
	evidence, err := api.commitEvidenceSubmissionLocked(caseCtx, turn, meta, nil, session.Path, session.UploadID)
	if err != nil {
		return nil, err
	}
	if err := api.recordSubmittedEvidenceEventLocked(turn, meta); err != nil {
		api.finishTurnAfterSignalLocked(turn, err)
		return nil, err
	}
	return map[string]any{
		"text":        fmt.Sprintf("Evidence upload accepted as evidence_id %s. Cite this evidence_id in offered_evidence if you want it admitted as an exhibit.", meta.EvidenceID),
		"evidence_id": meta.EvidenceID,
		"evidence":    evidence,
	}, nil
}

func (api *lawyerAPIServer) submitEvidenceLocked(caseCtx context.Context, turn *lawyerTurn, args map[string]any) (map[string]any, error) {
	meta, raw, err := api.rc.prepareSubmittedEvidence(turn.opportunity, args)
	if err != nil {
		return nil, err
	}
	evidence, err := api.commitEvidenceSubmissionLocked(caseCtx, turn, meta, raw, "", "")
	if err != nil {
		return nil, err
	}
	if err := api.recordSubmittedEvidenceEventLocked(turn, meta); err != nil {
		api.finishTurnAfterSignalLocked(turn, err)
		return nil, err
	}
	return map[string]any{
		"text":        fmt.Sprintf("Evidence accepted as evidence_id %s. Cite this evidence_id in offered_evidence if you want it admitted as an exhibit.", meta.EvidenceID),
		"evidence_id": meta.EvidenceID,
		"evidence":    evidence,
	}, nil
}

func (api *lawyerAPIServer) commitEvidenceSubmissionLocked(caseCtx context.Context, turn *lawyerTurn, meta SubmittedEvidenceMeta, raw []byte, sourcePath string, uploadID string) (evidence EvidenceMeta, err error) {
	if meta.Role != turn.opportunity.Role || meta.Phase != turn.opportunity.Phase {
		return EvidenceMeta{}, fmt.Errorf("submitted evidence belongs to role %s in phase %s, not role %s in phase %s", meta.Role, meta.Phase, turn.opportunity.Role, turn.opportunity.Phase)
	}
	if err := api.rc.validateSubmittedEvidenceID(meta.EvidenceID); err != nil {
		return EvidenceMeta{}, err
	}
	if err := api.rc.validateSubmittedEvidenceLineage(meta); err != nil {
		return EvidenceMeta{}, err
	}
	_, readableKind := caseFileKind(meta.Name)
	textReadable := (readableKind || strings.HasPrefix(strings.ToLower(meta.MimeType), "text/") || strings.EqualFold(meta.MimeType, "application/json")) && meta.SizeBytes <= api.rc.cfg.Policy.MaxEvidenceReadBytes
	evidence, err = submittedEvidenceRecordMeta(meta, textReadable)
	if err != nil {
		return EvidenceMeta{}, err
	}
	candidateEvidence, candidateEvidenceByID, err := api.rc.candidateEvidenceRegistry(evidence)
	if err != nil {
		return EvidenceMeta{}, err
	}
	payload := submittedEvidencePayload(meta)
	stepResp, replayAction, err := api.rc.evaluateTurnStepLocked(caseCtx, turn.deadline, turn.opportunity, "submit_evidence", turn.opportunity.Role, payload)
	if err != nil {
		return EvidenceMeta{}, err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return EvidenceMeta{}, fmt.Errorf("%s", mapString(stepResp["error"]))
	}
	nextState, nextVersion, err := acceptedStepState(stepResp, turn.opportunity.StateVersion)
	if err != nil {
		return EvidenceMeta{}, err
	}
	file, submittedCopyPath, err := api.rc.publishSubmittedEvidence(meta, evidence, raw, sourcePath)
	if err != nil {
		return EvidenceMeta{}, err
	}
	if uploadID != "" {
		session := api.rc.uploadSessions[uploadID]
		if session == nil || session.Path != sourcePath {
			return EvidenceMeta{}, fmt.Errorf("upload session %s changed during commit", uploadID)
		}
		if sourcePath != submittedCopyPath {
			if removeErr := os.Remove(sourcePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return EvidenceMeta{}, fmt.Errorf("remove published upload staging file %s: %w", uploadID, removeErr)
			}
		}
		session.Path = submittedCopyPath
	}
	candidateCaseFiles := append(append([]CaseFile(nil), api.rc.caseFiles...), file)
	candidateFileByID := make(map[string]CaseFile, len(api.rc.fileByID)+1)
	for evidenceID, existing := range api.rc.fileByID {
		candidateFileByID[evidenceID] = existing
	}
	candidateFileByID[file.EvidenceID] = file
	candidateSubmittedEvidence := append(append([]SubmittedEvidenceMeta(nil), api.rc.submittedEvidence...), meta)
	if err := api.rc.writeEvidenceManifestCandidate(candidateEvidence); err != nil {
		return EvidenceMeta{}, err
	}
	if err := turnStepError(caseCtx, turn.deadline, nil); err != nil {
		restoreErr := api.rc.writeEvidenceManifestCandidate(api.rc.evidence)
		if restoreErr != nil {
			restoreFailure := fmt.Errorf("restore evidence manifest after canceled submission: %w", restoreErr)
			runtimeErr := errors.Join(err, restoreFailure)
			if errors.Is(err, errTurnDeadlineExceeded) {
				message := fmt.Sprintf("%s lawyer opportunity timed out", turn.opportunity.Role)
				failErr := api.rc.failOpportunityLocked(caseCtx, time.Time{}, turn.opportunity, opportunityFailureDeadline, message, nil)
				runtimeErr = errors.Join(errors.New("evidence submission exceeded its opportunity deadline"), restoreFailure, failErr)
				if failErr == nil {
					api.finishTurnAfterSignalLocked(turn, runtimeErr)
				} else {
					api.finishTurnLocked(turn, runtimeErr)
				}
			}
			return EvidenceMeta{}, runtimeErr
		}
		return EvidenceMeta{}, err
	}

	api.rc.state = nextState
	turn.opportunity.StateVersion = nextVersion
	api.rc.evidence = candidateEvidence
	api.rc.evidenceByID = candidateEvidenceByID
	api.rc.caseFiles = candidateCaseFiles
	api.rc.fileByID = candidateFileByID
	api.rc.submittedEvidence = candidateSubmittedEvidence
	api.rc.certificateActions = append(api.rc.certificateActions, replayAction)
	if uploadID != "" {
		delete(api.rc.uploadSessions, uploadID)
	}
	api.rc.signalRoleAPIsLocked()
	return evidence, err
}

func acceptedStepState(stepResp map[string]any, sourceVersion int) (map[string]any, int, error) {
	state := mapAny(stepResp["state"])
	if len(state) == 0 {
		return nil, 0, fmt.Errorf("accepted Lean step returned empty state")
	}
	if len(mapAny(state["case"])) == 0 {
		return nil, 0, fmt.Errorf("accepted Lean step returned empty case state")
	}
	version, err := requiredStateVersion(state)
	if err != nil {
		return nil, 0, fmt.Errorf("accepted Lean step state: %w", err)
	}
	if version != sourceVersion+1 {
		return nil, 0, fmt.Errorf("accepted Lean step state_version=%d, want %d", version, sourceVersion+1)
	}
	return state, version, nil
}

func (api *lawyerAPIServer) submitDecisionLocked(caseCtx context.Context, turn *lawyerTurn, args map[string]any) (map[string]any, error) {
	if turn.completed {
		return nil, fmt.Errorf("decision already submitted for this opportunity")
	}
	actionType, payload, err := attorneyDecision(turn.opportunity, args, api.rc.evidenceByID, api.rc.cfg.Policy)
	if err != nil {
		return nil, participantInput(err)
	}
	if err := api.rc.validateAttorneyPayloadAgainstState(turn.opportunity, actionType, payload); err != nil {
		return nil, participantInput(err)
	}
	stepResp, replayAction, err := api.rc.evaluateTurnStepLocked(caseCtx, turn.deadline, turn.opportunity, actionType, turn.opportunity.Role, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return nil, fmt.Errorf("%s", mapString(stepResp["error"]))
	}
	nextState, nextVersion, err := acceptedStepState(stepResp, turn.opportunity.StateVersion)
	if err != nil {
		return nil, err
	}
	if err := turnStepError(caseCtx, turn.deadline, nil); err != nil {
		return nil, err
	}
	api.rc.state = nextState
	turn.opportunity.StateVersion = nextVersion
	api.rc.certificateActions = append(api.rc.certificateActions, replayAction)
	api.rc.signalRoleAPIsLocked()
	if err := api.rc.recordEventAtTurnLocked(turn.turnNumber, "attorney_action", turn.opportunity.Role, turn.opportunity.Phase, map[string]any{
		"opportunity_id": turn.opportunity.ID,
		"action_type":    actionType,
		"payload":        payload,
	}); err != nil {
		api.finishTurnAfterSignalLocked(turn, err)
		return nil, err
	}
	api.finishTurnAfterSignalLocked(turn, nil)
	return map[string]any{"text": "Decision accepted."}, nil
}

func (api *lawyerAPIServer) recordSubmittedEvidenceEventLocked(turn *lawyerTurn, meta SubmittedEvidenceMeta) error {
	return api.rc.recordEventAtTurnLocked(turn.turnNumber, "submitted_evidence", turn.opportunity.Role, turn.opportunity.Phase, map[string]any{
		"evidence_id":         meta.EvidenceID,
		"title":               meta.Title,
		"source_url":          meta.SourceURL,
		"source_description":  meta.SourceDescription,
		"mime_type":           meta.MimeType,
		"retrieval_timestamp": meta.RetrievalTimestamp,
		"relevance":           meta.Relevance,
		"sha256":              meta.SHA256,
		"size_bytes":          meta.SizeBytes,
		"parent_evidence_id":  meta.ParentEvidenceID,
		"parent_sha256":       meta.ParentSHA256,
		"derivation_method":   meta.DerivationMethod,
	})
}

func (api *lawyerAPIServer) callObserverToolLocked(tool string, args map[string]any) (map[string]any, error) {
	switch tool {
	case "case_status":
		if err := requireAllowedKeys(args, "case_status arguments"); err != nil {
			return nil, participantInput(err)
		}
		return api.caseStatusPayloadLocked("observer")
	case "get_case":
		if err := requireAllowedKeys(args, "get_case arguments"); err != nil {
			return nil, participantInput(err)
		}
		return map[string]any{"case": api.observerViewLocked()}, nil
	case "get_turn":
		if err := requireAllowedKeys(args, "get_turn arguments"); err != nil {
			return nil, participantInput(err)
		}
		return map[string]any{"turn": api.turnPayloadLocked(api.active)}, nil
	case "list_events":
		if err := requireAllowedKeys(args, "list_events arguments", "offset", "limit"); err != nil {
			return nil, participantInput(err)
		}
		offset, err := optionalIntParam(args, "offset", 0)
		if err != nil {
			return nil, participantInput(err)
		}
		limit, err := optionalIntParam(args, "limit", 100)
		if err != nil {
			return nil, participantInput(err)
		}
		if offset < 0 {
			return nil, participantInput(fmt.Errorf("offset must be non-negative"))
		}
		if limit <= 0 || limit > 1000 {
			return nil, participantInput(fmt.Errorf("limit must be between 1 and 1000"))
		}
		total := len(api.rc.events)
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		events := cloneAPIEvents(api.rc.events[offset:end])
		return map[string]any{"events": events, "offset": offset, "limit": limit, "total": total}, nil
	case "list_evidence":
		if err := requireAllowedKeys(args, "list_evidence arguments"); err != nil {
			return nil, participantInput(err)
		}
		return map[string]any{"evidence": api.rc.listVisibleEvidence()}, nil
	case "stat_evidence", "read_evidence_range":
		return nil, fmt.Errorf("evidence file tool %q requires unlocked execution", tool)
	default:
		return nil, participantInput(fmt.Errorf("unknown observer tool %q", tool))
	}
}

func (api *lawyerAPIServer) observerViewLocked() map[string]any {
	caseObj := mapAny(api.rc.state["case"])
	return map[string]any{
		"question":          api.rc.complaint.Question,
		"judgment_standard": currentJudgmentStandard(api.rc.state, api.rc.cfg.Policy),
		"phase":             currentPhase(api.rc.state),
		"record": map[string]any{
			"evidence":           api.rc.listVisibleEvidence(),
			"openings":           cloneJSONLikeMapList(mapList(caseObj["openings"])),
			"arguments":          cloneJSONLikeMapList(mapList(caseObj["arguments"])),
			"rebuttals":          cloneJSONLikeMapList(mapList(caseObj["rebuttals"])),
			"surrebuttals":       cloneJSONLikeMapList(mapList(caseObj["surrebuttals"])),
			"closings":           cloneJSONLikeMapList(mapList(caseObj["closings"])),
			"submitted_evidence": cloneJSONLikeMapList(mapList(caseObj["submitted_evidence"])),
			"exhibits":           api.rc.attorneyExhibits(),
			"technical_reports":  cloneJSONLikeMapList(mapList(caseObj["technical_reports"])),
			"council_answers":    cloneJSONLikeMapList(mapList(caseObj["council_answers"])),
		},
		"turn":   api.turnPayloadLocked(api.active),
		"events": len(api.rc.events),
		"policy": api.rc.cfg.Policy.StateMap(),
	}
}

func cloneAPIEvents(events []Event) []Event {
	out := make([]Event, len(events))
	for i, event := range events {
		event.Payload = cloneJSONLikeMap(event.Payload)
		out[i] = event
	}
	return out
}

func (api *lawyerAPIServer) caseStatusResponseLocked(caseID string, roleID string) (map[string]any, error) {
	response := api.responseBaseLocked(caseID, roleID)
	payload, err := api.caseStatusPayloadLocked(roleID)
	if err != nil {
		return nil, err
	}
	for key, value := range payload {
		response[key] = value
	}
	return response, nil
}

func (api *lawyerAPIServer) caseStatusPayloadLocked(roleID string) (map[string]any, error) {
	caseObj := mapAny(api.rc.state["case"])
	phase := currentPhase(api.rc.state)
	caseStatus := mapString(caseObj["status"])
	roleStatus := api.caseRoleStatusLocked(roleID, caseObj)
	councilRoster, err := councilSeatRoster(api.rc.council, mapList(caseObj["council_members"]))
	if err != nil {
		return nil, fmt.Errorf("build case-status council roster: %w", err)
	}
	payload := map[string]any{
		"status":              roleStatus,
		"phase":               phase,
		"case_status":         caseStatus,
		"council_backend":     api.rc.cfg.CouncilBackend,
		"council_roster":      councilRoster,
		"deliberation_round":  intNumber(caseObj["deliberation_round"]),
		"state_version":       mapAny(api.rc.state)["state_version"],
		"turn":                api.turnPayloadLocked(api.active),
		"current_opportunity": api.currentOpportunityPayloadLocked(),
		"counts":              api.caseStatusCountsLocked(caseObj),
		"message":             caseStatusMessage(roleStatus),
	}
	if api.rc.terminalReason != "" {
		payload["final_reason"] = api.rc.terminalReason
	}
	if caseStatus == "failed" {
		failure := caseFailure(api.rc.state)
		payload["failure"] = failure
		payload["error"] = caseFailureError(api.rc.state)
	}
	if roleID == "observer" {
		payload["limits"] = observerLimits(api.rc)
	} else if api.active != nil && !api.active.completed && api.active.opportunity.Role == roleID {
		payload["limits"] = api.lawyerLimitsLocked(api.active)
	}
	return payload, nil
}

func (api *lawyerAPIServer) caseRoleStatusLocked(roleID string, caseObj map[string]any) string {
	if mapString(caseObj["status"]) == "failed" {
		return "failed"
	}
	if api.caseIsFinalLocked(caseObj) {
		return "done"
	}
	if roleID == "observer" {
		return "observing"
	}
	if api.active != nil && !api.active.completed && api.active.opportunity.Role == roleID {
		return "ready"
	}
	return "waiting"
}

func (api *lawyerAPIServer) currentOpportunityPayloadLocked() map[string]any {
	turn := api.active
	if turn == nil || turn.completed {
		return nil
	}
	return map[string]any{
		"opportunity_id":       turn.opportunity.ID,
		"role_id":              turn.opportunity.Role,
		"phase":                turn.opportunity.Phase,
		"objective":            turn.opportunity.Objective,
		"may_pass":             turn.opportunity.MayPass,
		"final_filing_actions": decisionToolEnum(turn.opportunity.AllowedTools),
		"evidence_access": map[string]any{
			"read":   evidenceReadAllowed(turn.opportunity),
			"submit": evidenceSubmissionAllowed(turn.opportunity),
		},
		"remaining_ms":       remainingTurnMilliseconds(turn),
		"attempts_remaining": turn.attemptsRemaining,
		"attempts_max":       turn.attemptsMax,
	}
}

func (api *lawyerAPIServer) caseStatusCountsLocked(caseObj map[string]any) map[string]any {
	return map[string]any{
		"events":             len(api.rc.events),
		"visible_evidence":   len(api.rc.listVisibleEvidence()),
		"submitted_evidence": len(mapList(caseObj["submitted_evidence"])),
		"offered_evidence":   len(mapList(caseObj["offered_evidence"])),
		"technical_reports":  len(mapList(caseObj["technical_reports"])),
		"openings":           len(mapList(caseObj["openings"])),
		"arguments":          len(mapList(caseObj["arguments"])),
		"rebuttals":          len(mapList(caseObj["rebuttals"])),
		"surrebuttals":       len(mapList(caseObj["surrebuttals"])),
		"closings":           len(mapList(caseObj["closings"])),
		"council_answers":    len(mapList(caseObj["council_answers"])),
	}
}

func caseStatusMessage(roleStatus string) string {
	switch roleStatus {
	case "ready":
		return "This role has the active lawyer opportunity."
	case "waiting":
		return "This role has no active opportunity."
	case "observing":
		return "Observer access is read-only."
	case "done":
		return "The case is done."
	case "failed":
		return "The case failed."
	default:
		return "Case status is available."
	}
}

func (api *lawyerAPIServer) caseResultResponseLocked(caseID string, roleID string) map[string]any {
	response := api.responseBaseLocked(caseID, roleID)
	caseObj := mapAny(api.rc.state["case"])
	phase := currentPhase(api.rc.state)
	caseStatus := mapString(caseObj["status"])
	response["phase"] = phase
	response["case_status"] = caseStatus
	if !api.caseIsFinalLocked(caseObj) {
		response["status"] = "pending"
		response["message"] = "The case is still pending."
		return response
	}
	if caseStatus == "failed" {
		response["status"] = "failed"
		response["error"] = caseFailureError(api.rc.state)
		response["failure"] = caseFailure(api.rc.state)
		return response
	}
	answers := normalizeCouncilAnswers(mapList(caseObj["council_answers"]))
	response["status"] = "done"
	if api.rc.terminalReason != "" {
		response["final_reason"] = api.rc.terminalReason
	}
	response["result"] = map[string]any{
		"phase":              phase,
		"case_status":        caseStatus,
		"final_reason":       api.rc.terminalReason,
		"answers":            currentAnswers(api.rc.state),
		"council_answers":    answers,
		"answer_summary":     councilAnswerSummary(answers),
		"deliberation_round": intNumber(caseObj["deliberation_round"]),
	}
	return response
}

func (api *lawyerAPIServer) caseIsFinalLocked(caseObj map[string]any) bool {
	if api.rc.terminal {
		return true
	}
	if mapString(caseObj["status"]) == "failed" {
		return true
	}
	if mapString(caseObj["status"]) == "closed" {
		return true
	}
	if currentPhase(api.rc.state) == "closed" {
		return true
	}
	return false
}

func normalizeCouncilAnswers(answers []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(answers))
	for _, answer := range answers {
		out = append(out, map[string]any{
			"round":     intNumber(answer["round"]),
			"member_id": mapString(answer["member_id"]),
			"answer":    intNumber(answer["answer"]),
			"rationale": mapString(answer["rationale"]),
		})
	}
	return out
}

func councilAnswerSummary(answers []map[string]any) map[string]any {
	type stats struct {
		count int
		sum   int
		min   int
		max   int
	}
	rounds := map[int]stats{}
	finalRound := 0
	for _, answer := range answers {
		round := intNumber(answer["round"])
		if round <= 0 {
			round = 1
		}
		if round > finalRound {
			finalRound = round
		}
		value := intNumber(answer["answer"])
		s := rounds[round]
		if s.count == 0 || value < s.min {
			s.min = value
		}
		if s.count == 0 || value > s.max {
			s.max = value
		}
		s.count++
		s.sum += value
		rounds[round] = s
	}
	byRound := make([]map[string]any, 0, len(rounds))
	for round, s := range rounds {
		byRound = append(byRound, map[string]any{
			"round": round,
			"count": s.count,
			"min":   s.min,
			"max":   s.max,
			"mean":  float64(s.sum) / float64(s.count),
		})
	}
	sort.Slice(byRound, func(i, j int) bool {
		return intNumber(byRound[i]["round"]) < intNumber(byRound[j]["round"])
	})
	final := map[string]any{}
	if finalRound > 0 {
		s := rounds[finalRound]
		final = map[string]any{
			"round": finalRound,
			"count": s.count,
			"min":   s.min,
			"max":   s.max,
			"mean":  float64(s.sum) / float64(s.count),
		}
	}
	return map[string]any{
		"final_round": finalRound,
		"final":       final,
		"rounds":      byRound,
	}
}

func (api *lawyerAPIServer) consumeAttemptLocked(caseCtx context.Context, turn *lawyerTurn, err error, decisionAttempt bool) error {
	if turn.attemptsRemaining > 0 {
		turn.attemptsRemaining--
	}
	reason := strings.TrimSpace(err.Error())
	if reason == "" {
		reason = "invalid tool call"
	}
	turn.invalidReasons = append(turn.invalidReasons, reason)
	var feedback error
	if decisionAttempt {
		feedback = formatAttorneyInvalidDecisionError(turn.opportunity, api.rc.cfg.Policy, turn.invalidReasons, turn.attemptsMax)
	} else if turn.attemptsRemaining > 0 {
		feedback = fmt.Errorf(
			"%s\nInvalid tool call %d of %d for this opportunity. %d invalid %s remain.",
			ensureTerminalPeriod(reason),
			len(turn.invalidReasons),
			turn.attemptsMax,
			turn.attemptsRemaining,
			invalidSubmissionWord(turn.attemptsRemaining),
		)
	} else {
		feedback = formatInvalidAttemptLimitError(turn.opportunity.Role+" lawyer turn", turn.invalidReasons)
	}
	if turn.attemptsRemaining <= 0 {
		details := map[string]any{"invalid_reasons": append([]string(nil), turn.invalidReasons...)}
		if failErr := api.rc.failOpportunityLocked(caseCtx, turn.deadline, turn.opportunity, opportunityFailureAttemptsExhausted, feedback.Error(), details); failErr != nil {
			if errors.Is(failErr, errTurnDeadlineExceeded) {
				return failErr
			}
			runtimeErr := errors.Join(feedback, failErr)
			api.finishTurnLocked(turn, runtimeErr)
			return runtimeErr
		} else {
			api.finishTurnAfterSignalLocked(turn, nil)
		}
	} else {
		api.rc.signalRoleAPIsLocked()
	}
	return participantInput(feedback)
}

func (api *lawyerAPIServer) statusResponseLocked(caseID string, roleID string) map[string]any {
	terminalStatus := roleAPITerminalStatus(api.rc.state)
	if api.rc.terminal || terminalStatus != "" {
		response := api.responseBaseLocked(caseID, roleID)
		if terminalStatus == "failed" {
			response["status"] = "failed"
			response["failure"] = caseFailure(api.rc.state)
			response["error"] = caseFailureError(api.rc.state)
		} else {
			response["status"] = "done"
		}
		response["prompt"] = ""
		response["tools"] = []map[string]any{api.rc.cfg.caseStatusHTTPToolSpec()}
		if api.rc.terminalReason != "" {
			response["final_reason"] = api.rc.terminalReason
		}
		return response
	}
	if roleID == "observer" {
		response := api.responseBaseLocked(caseID, roleID)
		response["status"] = "observing"
		response["prompt"] = api.rc.cfg.observerPrompt
		response["tools"] = api.rc.cfg.observerToolSpecs()
		response["limits"] = observerLimits(api.rc)
		return response
	}
	response := api.responseBaseLocked(caseID, roleID)
	turn := api.active
	if turn == nil || turn.completed || turn.opportunity.Role != roleID {
		response["status"] = "waiting"
		response["prompt"] = ""
		response["tools"] = []map[string]any{api.rc.cfg.caseStatusHTTPToolSpec()}
		return response
	}
	response["status"] = "ready"
	response["prompt"] = turn.prompt
	response["tools"] = api.rc.cfg.lawyerToolSpecs(turn.opportunity)
	response["limits"] = api.lawyerLimitsLocked(turn)
	return response
}

func (api *lawyerAPIServer) waitResponseLocked(caseID string, roleID string, after string, baseline uint64) (map[string]any, string, bool) {
	response := api.statusResponseLocked(caseID, roleID)
	if response["status"] == "failed" {
		return response, "failed", true
	}
	if response["status"] == "done" {
		return response, "done", true
	}
	turn := api.active
	if turn != nil && !turn.completed && turn.opportunity.Role == roleID {
		if after == "" || turn.opportunity.ID != after {
			return response, "ready", true
		}
	}
	if api.version != baseline {
		return response, "changed", true
	}
	return response, "", false
}

func roleAPITerminalStatus(state map[string]any) string {
	caseObj := mapAny(state["case"])
	if mapString(caseObj["status"]) == "failed" {
		return "failed"
	}
	if mapString(caseObj["status"]) == "closed" || currentPhase(state) == "closed" {
		return "done"
	}
	return ""
}

func (api *lawyerAPIServer) waitPayloadLocked(reason string) map[string]any {
	return map[string]any{
		"reason":        reason,
		"version":       api.version,
		"state_version": mapAny(api.rc.state)["state_version"],
	}
}

func (api *lawyerAPIServer) responseBaseLocked(caseID string, roleID string) map[string]any {
	return map[string]any{
		"ok":      true,
		"case_id": strings.TrimSpace(caseID),
		"role_id": strings.TrimSpace(roleID),
		"turn":    api.turnPayloadLocked(api.active),
	}
}

func (api *lawyerAPIServer) turnPayloadLocked(turn *lawyerTurn) map[string]any {
	if turn == nil {
		return nil
	}
	return map[string]any{
		"role_id":            turn.opportunity.Role,
		"phase":              turn.opportunity.Phase,
		"opportunity_id":     turn.opportunity.ID,
		"turn_number":        turn.turnNumber,
		"deadline":           turn.deadline.UTC().Format(time.RFC3339Nano),
		"remaining_ms":       remainingTurnMilliseconds(turn),
		"attempts_max":       turn.attemptsMax,
		"attempts_remaining": turn.attemptsRemaining,
		"completed":          turn.completed,
	}
}

func remainingTurnMilliseconds(turn *lawyerTurn) int64 {
	if turn == nil {
		return 0
	}
	remaining := time.Until(turn.deadline).Milliseconds()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (api *lawyerAPIServer) lawyerLimitsLocked(turn *lawyerTurn) map[string]any {
	limits := api.rc.attorneyLimits(turn.opportunity)
	limits["max_response_bytes"] = api.rc.cfg.Runtime.MaxResponseBytes
	limits["attempts_max"] = turn.attemptsMax
	limits["attempts_remaining"] = turn.attemptsRemaining
	if evidenceReadAllowed(turn.opportunity) {
		limits["remaining_evidence_reads_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads)
		limits["remaining_evidence_read_bytes_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes)
	}
	return limits
}

func (api *lawyerAPIServer) evidenceReadLimitsLocked(turn *lawyerTurn) map[string]any {
	return map[string]any{
		"max_read_bytes":                       api.rc.cfg.Policy.MaxEvidenceReadBytes,
		"max_reads_per_opportunity":            api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity,
		"max_read_bytes_per_opportunity":       api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity,
		"remaining_read_bytes_for_opportunity": remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes),
		"remaining_reads_for_opportunity":      remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads),
	}
}

func observerLimits(rc *runContext) map[string]any {
	return map[string]any{
		"max_evidence_read_bytes": rc.cfg.Policy.MaxEvidenceReadBytes,
	}
}

func parseLawyerAPIWaitTimeout(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultLawyerAPIWaitTimeout, nil
	}
	ms, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("timeout_ms must be an integer")
	}
	if ms <= 0 {
		return 0, fmt.Errorf("timeout_ms must be positive")
	}
	timeout := time.Duration(ms) * time.Millisecond
	if timeout > maxLawyerAPIWaitTimeout {
		return 0, fmt.Errorf("timeout_ms must be at most %d", maxLawyerAPIWaitTimeout.Milliseconds())
	}
	return timeout, nil
}

func parseOptionalUintQuery(value string, name string) (uint64, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be an unsigned integer", name)
	}
	return parsed, true, nil
}

func writeLawyerJSON(w http.ResponseWriter, status int, value map[string]any) {
	writeCaseAPIJSON(w, status, value)
}

func apiError(code string, message string) map[string]any {
	return map[string]any{
		"code":    strings.TrimSpace(code),
		"message": strings.TrimSpace(message),
	}
}

func optionalIntParam(params map[string]any, key string, fallback int) (int, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback, nil
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case json.Number:
		i, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func lawyerToolSpecs(opportunity Opportunity) []map[string]any {
	return lawyerToolSpecsWithDescription(opportunity, func(description string) string { return description })
}

func (cfg Config) lawyerToolSpecs(opportunity Opportunity) []map[string]any {
	return lawyerToolSpecsWithDescription(opportunity, cfg.modelToolDescription)
}

func lawyerToolSpecsWithDescription(opportunity Opportunity, describe func(string) string) []map[string]any {
	specs := []map[string]any{
		caseStatusHTTPToolSpecWithDescription(describe),
		httpToolSpec("get_case", describe("Return the current visible arbitration record."), emptyObjectSchema(), true),
		httpToolSpec("send_work_notes", describe("Send private work notes for off-record operator analysis. This does not create evidence, a filing, a technical report, or a case event."), workNotesSchemaWithDescription(describe), false),
	}
	if evidenceReadAllowed(opportunity) {
		specs = append(specs,
			httpToolSpec("list_evidence", describe("List visible immutable record evidence."), emptyObjectSchema(), true),
			httpToolSpec("stat_evidence", describe("Return metadata and read limits for one visible evidence item."), evidenceIDSchema(), true),
			httpToolSpec("read_evidence_range", describe("Read a bounded byte range from one visible evidence item as base64."), readEvidenceRangeSchema(), true),
		)
	}
	if evidenceSubmissionAllowed(opportunity) {
		specs = append(specs,
			httpToolSpec("begin_evidence_upload", describe("Begin a chunked evidence upload."), beginEvidenceUploadSchema(), false),
			httpToolSpec("write_evidence_chunk", describe("Write one base64 chunk into an upload session."), writeEvidenceChunkSchema(), false),
			httpToolSpec("commit_evidence_upload", describe("Verify and admit a completed evidence upload."), commitEvidenceUploadSchema(), false),
			httpToolSpec("submit_evidence", describe("Submit source evidence with provenance."), submittedEvidenceSchema(), false),
		)
	}
	specs = append(specs, httpToolSpec("submit_decision", describe("Submit the legal act for the current opportunity."), submitDecisionHTTPSchema(opportunity.AllowedTools), false))
	return specs
}

func observerToolSpecs() []map[string]any {
	return observerToolSpecsWithDescription(func(description string) string { return description })
}

func (cfg Config) observerToolSpecs() []map[string]any {
	return observerToolSpecsWithDescription(cfg.modelToolDescription)
}

func observerToolSpecsWithDescription(describe func(string) string) []map[string]any {
	return []map[string]any{
		caseStatusHTTPToolSpecWithDescription(describe),
		httpToolSpec("get_case", describe("Return the current arbitration record."), emptyObjectSchema(), true),
		httpToolSpec("get_turn", describe("Return the current turn role, phase, deadline, and attempts."), emptyObjectSchema(), true),
		httpToolSpec("list_events", describe("List recorded case events."), listEventsSchema(), true),
		httpToolSpec("list_evidence", describe("List visible immutable record evidence."), emptyObjectSchema(), true),
		httpToolSpec("stat_evidence", describe("Return metadata for one visible evidence item."), evidenceIDSchema(), true),
		httpToolSpec("read_evidence_range", describe("Read a bounded byte range from one visible evidence item as base64."), readEvidenceRangeSchema(), true),
	}
}

func caseStatusHTTPToolSpec() map[string]any {
	return caseStatusHTTPToolSpecWithDescription(func(description string) string { return description })
}

func (cfg Config) caseStatusHTTPToolSpec() map[string]any {
	return caseStatusHTTPToolSpecWithDescription(cfg.modelToolDescription)
}

func caseStatusHTTPToolSpecWithDescription(describe func(string) string) map[string]any {
	return httpToolSpec("case_status", describe("Return the current case phase, active turn, role status, and case counts."), emptyObjectSchema(), true)
}

func httpToolSpec(name string, description string, schema map[string]any, readOnly bool) map[string]any {
	return map[string]any{
		"name":         name,
		"description":  description,
		"input_schema": schema,
		"read_only":    readOnly,
	}
}

func emptyObjectSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}

func evidenceIDSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"evidence_id": map[string]any{"type": "string"},
		},
		"required":             []string{"evidence_id"},
		"additionalProperties": false,
	}
}

func readEvidenceRangeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"evidence_id": map[string]any{"type": "string"},
			"offset":      map[string]any{"type": "integer", "minimum": 0},
			"length":      map[string]any{"type": "integer", "minimum": 1},
		},
		"required":             []string{"evidence_id", "offset", "length"},
		"additionalProperties": false,
	}
}

func workNotesSchema() map[string]any {
	return workNotesSchemaWithDescription(func(description string) string { return description })
}

func workNotesSchemaWithDescription(describe func(string) string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"notes": map[string]any{
				"type":        "string",
				"description": describe("Accumulated private work notes for this lawyer turn."),
			},
		},
		"required":             []string{"notes"},
		"additionalProperties": false,
	}
}

func beginEvidenceUploadSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":               map[string]any{"type": "string"},
			"mime_type":           map[string]any{"type": "string"},
			"expected_size_bytes": map[string]any{"type": "integer", "minimum": 1},
			"expected_sha256":     map[string]any{"type": "string"},
			"source_url":          map[string]any{"type": "string"},
			"source_description":  map[string]any{"type": "string"},
			"retrieval_timestamp": map[string]any{"type": "string"},
			"relevance":           map[string]any{"type": "string"},
			"parent_evidence_id":  map[string]any{"type": "string"},
			"derivation_method":   map[string]any{"type": "string"},
		},
		"required":             []string{"title", "mime_type", "expected_size_bytes", "relevance"},
		"additionalProperties": false,
	}
}

func writeEvidenceChunkSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"upload_id":      map[string]any{"type": "string"},
			"offset":         map[string]any{"type": "integer", "minimum": 0},
			"content_base64": map[string]any{"type": "string"},
		},
		"required":             []string{"upload_id", "offset", "content_base64"},
		"additionalProperties": false,
	}
}

func commitEvidenceUploadSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"upload_id":              map[string]any{"type": "string"},
			"expected_sha256":        map[string]any{"type": "string"},
			"preferred_filename_ext": map[string]any{"type": "string"},
		},
		"required":             []string{"upload_id"},
		"additionalProperties": false,
	}
}

func submitDecisionHTTPSchema(allowedTools []string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{"tool", "pass"}},
			"tool_name": map[string]any{
				"type": "string",
				"enum": decisionToolEnum(allowedTools),
			},
			"payload": attorneyPayloadSchema(),
		},
		"required":             []string{"kind"},
		"additionalProperties": false,
	}
}

func listEventsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"offset": map[string]any{"type": "integer", "minimum": 0},
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
		},
		"additionalProperties": false,
	}
}
