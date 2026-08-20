package proceeding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const councilAPIBasePath = "/councilapi/v1"

const (
	councilBackendAPI            = "councilapi"
	defaultCouncilAPIWaitTimeout = 30 * time.Second
	maxCouncilAPIWaitTimeout     = 5 * time.Minute
)

type councilAPIServer struct {
	rc *runContext

	cond          *sync.Cond
	version       uint64
	active        *councilTurn
	evidenceFiles evidenceFileOperations
}

type councilTurn struct {
	opportunity       Opportunity
	seat              CouncilSeat
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

type councilDoRequest struct {
	CaseID        string         `json:"case_id"`
	MemberID      string         `json:"member_id"`
	OpportunityID string         `json:"opportunity_id,omitempty"`
	Tool          string         `json:"tool"`
	Arguments     map[string]any `json:"arguments"`
	CallID        string         `json:"call_id,omitempty"`
}

type councilFailRequest struct {
	CaseID        string         `json:"case_id"`
	MemberID      string         `json:"member_id"`
	OpportunityID string         `json:"opportunity_id"`
	Reason        string         `json:"reason,omitempty"`
	Message       string         `json:"message,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

func newCouncilAPIServer(rc *runContext) *councilAPIServer {
	api := &councilAPIServer{
		rc: rc,
	}
	api.cond = sync.NewCond(&rc.mu)
	return api
}

func (api *councilAPIServer) register(mux *http.ServeMux, caseCtx context.Context) {
	if caseCtx == nil {
		caseCtx = context.Background()
	}
	mux.HandleFunc(councilAPIBasePath+"/get", api.handleGet)
	mux.HandleFunc(councilAPIBasePath+"/wait", api.handleWait)
	mux.HandleFunc(councilAPIBasePath+"/do", func(w http.ResponseWriter, r *http.Request) {
		api.handleDoContext(caseCtx, w, r)
	})
	mux.HandleFunc(councilAPIBasePath+"/fail", func(w http.ResponseWriter, r *http.Request) {
		api.handleFailContext(caseCtx, w, r)
	})
}

func (api *councilAPIServer) startTurnLocked(turn *councilTurn) error {
	if api.active != nil && !api.active.completed {
		return fmt.Errorf("councilapi already has an active turn")
	}
	api.active = turn
	api.signalChangedLocked()
	return nil
}

func (api *councilAPIServer) clearTurn(turn *councilTurn) {
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	if api.active == turn {
		api.active = nil
		api.signalChangedLocked()
	}
}

func (api *councilAPIServer) signalChangedLocked() {
	api.version++
	api.ensureCondLocked().Broadcast()
}

func (api *councilAPIServer) ensureCondLocked() *sync.Cond {
	if api.cond == nil {
		api.cond = sync.NewCond(&api.rc.mu)
	}
	return api.cond
}

func (rc *runContext) executeCouncilAPIOpportunity(ctx context.Context, opportunity Opportunity, seat CouncilSeat) error {
	rc.mu.Lock()
	api := rc.councilAPI
	if api == nil {
		rc.mu.Unlock()
		return fmt.Errorf("councilapi server is not running")
	}
	prompt, err := rc.buildCouncilAPIPrompt(seat, opportunity)
	if err != nil {
		rc.mu.Unlock()
		return err
	}
	turn := &councilTurn{
		opportunity:       opportunity,
		seat:              seat,
		turnNumber:        rc.turn,
		prompt:            prompt,
		deadline:          time.Now().Add(rc.cfg.Runtime.CouncilTimeout()),
		attemptsMax:       rc.cfg.Runtime.InvalidAttemptLimit,
		attemptsRemaining: rc.cfg.Runtime.InvalidAttemptLimit,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
	if err := rc.writeCouncilTurnSnapshot(turn, prompt); err != nil {
		rc.mu.Unlock()
		return err
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
		return api.timeoutTurnContext(ctx, turn, rc.cfg.Runtime.CouncilTimeout())
	case err := <-turn.done:
		return err
	}
}

func (api *councilAPIServer) timeoutTurn(turn *councilTurn, timeout time.Duration) error {
	return api.timeoutTurnContext(context.Background(), turn, timeout)
}

func (api *councilAPIServer) timeoutTurnContext(caseCtx context.Context, turn *councilTurn, timeout time.Duration) error {
	api.rc.mu.Lock()
	if turn == nil || api.active != turn {
		api.rc.mu.Unlock()
		return fmt.Errorf("councilapi timed-out turn is no longer active")
	}
	if turn.completed {
		done := turn.done
		api.rc.mu.Unlock()
		return <-done
	}
	err := fmt.Errorf("council member %s opportunity timed out after %s", turn.seat.MemberID, timeout)
	details := map[string]any{
		"member_id": turn.seat.MemberID,
		"model":     turn.seat.Model,
	}
	if failErr := api.rc.failOpportunityLocked(caseCtx, time.Time{}, turn.opportunity, opportunityFailureDeadline, err.Error(), details); failErr != nil {
		api.finishTurnLocked(turn, failErr)
		api.rc.mu.Unlock()
		return failErr
	}
	api.finishTurnAfterSignalLocked(turn, nil)
	api.rc.mu.Unlock()
	return nil
}

func (api *councilAPIServer) finishTurn(turn *councilTurn, err error) {
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	api.finishTurnLocked(turn, err)
}

func (api *councilAPIServer) finishTurnLocked(turn *councilTurn, err error) {
	api.completeTurnLocked(turn, err, true)
}

func (api *councilAPIServer) finishTurnAfterSignalLocked(turn *councilTurn, err error) {
	api.completeTurnLocked(turn, err, false)
}

func (api *councilAPIServer) completeTurnLocked(turn *councilTurn, err error, signal bool) {
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

func (api *councilAPIServer) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCouncilJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	caseID, memberID, ok := api.parseGetWaitIdentity(w, r)
	if !ok {
		return
	}
	api.rc.mu.Lock()
	response := api.statusResponseLocked(caseID, memberID)
	api.rc.mu.Unlock()
	writeCouncilJSON(w, http.StatusOK, response)
}

func (api *councilAPIServer) handleWait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCouncilJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	caseID, memberID, ok := api.parseGetWaitIdentity(w, r)
	if !ok {
		return
	}
	after := strings.TrimSpace(r.URL.Query().Get("after"))
	afterVersion, hasAfterVersion, err := parseOptionalUintQuery(r.URL.Query().Get("after_version"), "after_version")
	if err != nil {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_after_version", err.Error()),
		})
		return
	}
	timeout, err := parseCouncilAPIWaitTimeout(r.URL.Query().Get("timeout_ms"))
	if err != nil {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_timeout", err.Error()),
		})
		return
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
		if response, reason, ready := api.waitResponseLocked(caseID, memberID, after, baseline); ready {
			response["wait"] = api.waitPayloadLocked(reason)
			api.rc.mu.Unlock()
			writeCouncilJSON(w, http.StatusOK, response)
			return
		}
		if !time.Now().Before(deadline) {
			response := api.statusResponseLocked(caseID, memberID)
			response["wait"] = api.waitPayloadLocked("timeout")
			api.rc.mu.Unlock()
			writeCouncilJSON(w, http.StatusOK, response)
			return
		}
		cond.Wait()
	}
}

func (api *councilAPIServer) handleDo(w http.ResponseWriter, r *http.Request) {
	api.handleDoContext(context.Background(), w, r)
}

func (api *councilAPIServer) handleDoContext(caseCtx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCouncilJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use POST"),
		})
		return
	}
	var req councilDoRequest
	body := http.MaxBytesReader(w, r.Body, int64(api.rc.cfg.Runtime.MaxResponseBytes))
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			err = fmt.Errorf("request body is required")
		}
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_json", err.Error()),
		})
		return
	}
	if err := requireJSONEOF(dec); err != nil {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_json", err.Error()),
		})
		return
	}
	req.CaseID = strings.TrimSpace(req.CaseID)
	req.MemberID = strings.TrimSpace(req.MemberID)
	req.OpportunityID = strings.TrimSpace(req.OpportunityID)
	req.Tool = strings.TrimSpace(req.Tool)
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	if req.CaseID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(req.CaseID) {
		api.writeCaseMismatch(w, req.CaseID, req.MemberID)
		return
	}
	if req.MemberID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": req.CaseID,
			"error":   apiError("missing_member_id", "member_id is required"),
		})
		return
	}
	if req.Tool == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":        false,
			"case_id":   req.CaseID,
			"member_id": req.MemberID,
			"error":     apiError("missing_tool", "tool is required"),
		})
		return
	}
	api.handleCouncilDo(caseCtx, r.Context(), w, req)
}

func (api *councilAPIServer) handleFail(w http.ResponseWriter, r *http.Request) {
	api.handleFailContext(context.Background(), w, r)
}

func (api *councilAPIServer) handleFailContext(caseCtx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCouncilJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use POST"),
		})
		return
	}
	var req councilFailRequest
	body := http.MaxBytesReader(w, r.Body, int64(api.rc.cfg.Runtime.MaxResponseBytes))
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			err = fmt.Errorf("request body is required")
		}
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_json", err.Error()),
		})
		return
	}
	if err := requireJSONEOF(dec); err != nil {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("bad_json", err.Error()),
		})
		return
	}
	req.CaseID = strings.TrimSpace(req.CaseID)
	req.MemberID = strings.TrimSpace(req.MemberID)
	req.OpportunityID = strings.TrimSpace(req.OpportunityID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Message = strings.TrimSpace(req.Message)
	if req.CaseID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return
	}
	if !api.caseIDMatches(req.CaseID) {
		api.writeCaseMismatch(w, req.CaseID, req.MemberID)
		return
	}
	if req.MemberID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": req.CaseID,
			"error":   apiError("missing_member_id", "member_id is required"),
		})
		return
	}
	if req.OpportunityID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":        false,
			"case_id":   req.CaseID,
			"member_id": req.MemberID,
			"error":     apiError("missing_opportunity_id", "opportunity_id is required"),
		})
		return
	}
	if req.Reason == "" {
		req.Reason = opportunityFailureAgentExited
	}
	if req.Reason != opportunityFailureAgentExited && req.Reason != opportunityFailureAgentOutputLimit {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":        false,
			"case_id":   req.CaseID,
			"member_id": req.MemberID,
			"error":     apiError("invalid_reason", "reason must be agent_exited or agent_output_limit_exceeded"),
		})
		return
	}
	api.handleCouncilFail(caseCtx, r.Context(), w, req)
}

func (api *councilAPIServer) handleCouncilFail(caseCtx context.Context, requestCtx context.Context, w http.ResponseWriter, req councilFailRequest) {
	api.rc.mu.Lock()
	response := api.councilFailResponseLocked(caseCtx, req)
	api.rc.mu.Unlock()
	if requestCtx.Err() != nil {
		return
	}
	writeCouncilJSON(w, http.StatusOK, response)
}

func (api *councilAPIServer) councilFailResponseLocked(caseCtx context.Context, req councilFailRequest) map[string]any {
	turn := api.active
	if turn == nil || turn.completed {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("no_active_turn", "no council turn is active")
		return response
	}
	if turn.seat.MemberID != req.MemberID {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("not_current_turn", fmt.Sprintf("current council turn belongs to %s", turn.seat.MemberID))
		return response
	}
	if req.OpportunityID != turn.opportunity.ID {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("stale_opportunity", fmt.Sprintf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, turn.opportunity.ID))
		return response
	}
	if !time.Now().Before(turn.deadline) {
		err := api.councilTurnDeadlineError(turn)
		if failErr := api.expireCouncilTurnLocked(caseCtx, turn); failErr != nil {
			api.finishTurnLocked(turn, failErr)
			response := api.responseBaseLocked(req.CaseID, req.MemberID)
			response["ok"] = false
			response["error"] = apiError("runtime_failure", failErr.Error())
			return response
		}
		api.finishTurnAfterSignalLocked(turn, nil)
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("turn_timeout", err.Error())
		return response
	}
	details := cloneMap(req.Details)
	details["member_id"] = turn.seat.MemberID
	details["model"] = turn.seat.Model
	if failErr := api.rc.failOpportunityLocked(caseCtx, turn.deadline, turn.opportunity, req.Reason, req.Message, details); failErr != nil {
		if errors.Is(failErr, errTurnDeadlineExceeded) {
			deadlineErr := api.councilTurnDeadlineError(turn)
			if transitionErr := api.expireCouncilTurnLocked(caseCtx, turn); transitionErr != nil {
				api.finishTurnLocked(turn, transitionErr)
				response := api.responseBaseLocked(req.CaseID, req.MemberID)
				response["ok"] = false
				response["error"] = apiError("runtime_failure", transitionErr.Error())
				return response
			}
			api.finishTurnAfterSignalLocked(turn, nil)
			response := api.responseBaseLocked(req.CaseID, req.MemberID)
			response["ok"] = false
			response["error"] = apiError("turn_timeout", deadlineErr.Error())
			return response
		}
		api.finishTurnLocked(turn, failErr)
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("runtime_failure", failErr.Error())
		return response
	}
	api.finishTurnAfterSignalLocked(turn, nil)
	response := api.responseBaseLocked(req.CaseID, req.MemberID)
	response["ok"] = true
	response["result"] = map[string]any{"text": "Council member failure recorded."}
	if failure := api.failedCouncilMemberPayloadLocked(req.MemberID); failure != nil {
		response["failure"] = failure
	}
	return response
}

func (api *councilAPIServer) councilTurnDeadlineError(turn *councilTurn) error {
	return fmt.Errorf("council member %s opportunity timed out: %w", turn.seat.MemberID, errTurnDeadlineExceeded)
}

func (api *councilAPIServer) expireCouncilTurnLocked(caseCtx context.Context, turn *councilTurn) error {
	details := map[string]any{
		"member_id": turn.seat.MemberID,
		"model":     turn.seat.Model,
	}
	return api.rc.failOpportunityLocked(
		caseCtx,
		time.Time{},
		turn.opportunity,
		opportunityFailureDeadline,
		opportunityFailureMessage(turn.opportunity, opportunityFailureDeadline),
		details,
	)
}

func (api *councilAPIServer) handleCouncilDo(caseCtx context.Context, requestCtx context.Context, w http.ResponseWriter, req councilDoRequest) {
	if evidenceFileTool(req.Tool) {
		response := api.councilEvidenceDoResponse(caseCtx, req)
		if requestCtx.Err() != nil {
			return
		}
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	api.rc.mu.Lock()
	response := api.councilDoResponseLocked(caseCtx, req)
	api.rc.mu.Unlock()
	if requestCtx.Err() != nil {
		return
	}
	writeCouncilJSON(w, http.StatusOK, response)
}

func (api *councilAPIServer) councilDoResponseLocked(caseCtx context.Context, req councilDoRequest) map[string]any {
	turn, response := api.councilRequestTurnLocked(caseCtx, req)
	if response != nil {
		return response
	}
	result, err := api.callCouncilToolLocked(caseCtx, turn, req.Tool, req.Arguments)
	if err != nil {
		return api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
	}
	if !turn.completed {
		if err := turnStepError(caseCtx, turn.deadline, nil); err != nil {
			return api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
		}
	}
	response = api.responseBaseLocked(req.CaseID, req.MemberID)
	response["ok"] = true
	response["result"] = result
	return response
}

func (api *councilAPIServer) councilToolErrorResponseLocked(caseCtx context.Context, req councilDoRequest, turn *councilTurn, err error) map[string]any {
	if isParticipantInput(err) {
		if timingErr := turnStepError(caseCtx, turn.deadline, nil); timingErr != nil {
			err = timingErr
		} else {
			err = api.consumeAttemptLocked(caseCtx, turn, err)
		}
	}
	if errors.Is(err, errTurnDeadlineExceeded) {
		return api.councilDeadlineResponseLocked(caseCtx, req, turn)
	}
	code := "runtime_failure"
	if isParticipantInput(err) {
		code = "tool_failed"
	} else if !turn.completed {
		api.finishTurnLocked(turn, err)
	}
	response := api.responseBaseLocked(req.CaseID, req.MemberID)
	response["ok"] = false
	response["error"] = apiError(code, err.Error())
	return response
}

func (api *councilAPIServer) councilDeadlineResponseLocked(caseCtx context.Context, req councilDoRequest, turn *councilTurn) map[string]any {
	err := api.councilTurnDeadlineError(turn)
	code := "turn_timeout"
	responseErr := err
	if !turn.completed {
		if failErr := api.expireCouncilTurnLocked(caseCtx, turn); failErr != nil {
			api.finishTurnLocked(turn, failErr)
			code = "runtime_failure"
			responseErr = failErr
		} else {
			api.finishTurnAfterSignalLocked(turn, nil)
		}
	}
	response := api.responseBaseLocked(req.CaseID, req.MemberID)
	response["ok"] = false
	response["error"] = apiError(code, responseErr.Error())
	return response
}

func (api *councilAPIServer) councilRequestTurnLocked(caseCtx context.Context, req councilDoRequest) (*councilTurn, map[string]any) {
	turn := api.active
	if turn == nil || turn.completed {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("no_active_turn", "no council turn is active")
		return nil, response
	}
	if err := context.Cause(caseCtx); err != nil {
		api.finishTurnLocked(turn, err)
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("runtime_failure", err.Error())
		return nil, response
	}
	if time.Now().After(turn.deadline) {
		err := fmt.Errorf("council member %s opportunity timed out", turn.seat.MemberID)
		code := "turn_timeout"
		responseErr := err
		if failErr := api.rc.failOpportunityLocked(caseCtx, time.Time{}, turn.opportunity, opportunityFailureDeadline, err.Error(), map[string]any{
			"member_id": turn.seat.MemberID,
			"model":     turn.seat.Model,
		}); failErr != nil {
			api.finishTurnLocked(turn, failErr)
			code = "runtime_failure"
			responseErr = failErr
		} else {
			api.finishTurnAfterSignalLocked(turn, nil)
		}
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError(code, responseErr.Error())
		return nil, response
	}
	if turn.seat.MemberID != req.MemberID {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("not_current_turn", fmt.Sprintf("current council turn belongs to %s", turn.seat.MemberID))
		return nil, response
	}
	if req.OpportunityID == "" {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("missing_opportunity_id", "opportunity_id is required for council tool calls")
		return nil, response
	}
	if req.OpportunityID != turn.opportunity.ID {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("stale_opportunity", fmt.Sprintf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, turn.opportunity.ID))
		return nil, response
	}
	return turn, nil
}

func (api *councilAPIServer) councilEvidenceDoResponse(caseCtx context.Context, req councilDoRequest) map[string]any {
	api.rc.mu.Lock()
	turn, response := api.councilRequestTurnLocked(caseCtx, req)
	if response != nil {
		api.rc.mu.Unlock()
		return response
	}
	if err := validateEvidenceFileToolArguments(req.Tool, req.Arguments); err != nil {
		response := api.councilToolErrorResponseLocked(caseCtx, req, turn, participantInput(err))
		api.rc.mu.Unlock()
		return response
	}
	if req.Tool == "stat_evidence" {
		file, err := api.rc.evidenceFileSnapshotLocked(mapString(req.Arguments["evidence_id"]))
		if err != nil {
			response := api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
			api.rc.mu.Unlock()
			return response
		}
		api.rc.mu.Unlock()
		err = api.evidenceFiles.verifyFile(file)
		api.rc.mu.Lock()
		defer api.rc.mu.Unlock()
		if response := api.revalidateCouncilEvidenceTurnLocked(caseCtx, req, turn, nil); response != nil {
			return response
		}
		if err != nil {
			return api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
		}
		response = api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = true
		response["result"] = map[string]any{
			"evidence": file.meta,
			"limits":   api.evidenceReadLimitsLocked(turn),
		}
		return response
	}

	offset, err := requiredIntParam(req.Arguments, "offset")
	if err != nil {
		response := api.councilToolErrorResponseLocked(caseCtx, req, turn, participantInput(err))
		api.rc.mu.Unlock()
		return response
	}
	length, err := requiredIntParam(req.Arguments, "length")
	if err != nil {
		response := api.councilToolErrorResponseLocked(caseCtx, req, turn, participantInput(err))
		api.rc.mu.Unlock()
		return response
	}
	reservation, err := api.rc.reserveEvidenceReadLocked(mapString(req.Arguments["evidence_id"]), int64(offset), length, turn.evidenceBudget)
	if err != nil {
		response := api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
		api.rc.mu.Unlock()
		return response
	}
	api.rc.mu.Unlock()
	result, bytesRead, readErr := api.evidenceFiles.readRange(reservation)
	api.rc.mu.Lock()
	defer api.rc.mu.Unlock()
	if response := api.revalidateCouncilEvidenceTurnLocked(caseCtx, req, turn, &reservation); response != nil {
		return response
	}
	if readErr != nil {
		rollbackEvidenceReadLocked(&reservation)
		return api.councilToolErrorResponseLocked(caseCtx, req, turn, readErr)
	}
	finalizeEvidenceReadLocked(&reservation, bytesRead)
	result["remaining_read_bytes_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes)
	result["remaining_reads_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads)
	if err := api.rc.recordEventAtTurnLocked(turn.turnNumber, "evidence_read", "council", turn.opportunity.Phase, map[string]any{
		"member_id":   turn.seat.MemberID,
		"evidence_id": result["evidence_id"],
		"offset":      result["offset"],
		"length":      result["length"],
		"byte_count":  result["length"],
	}); err != nil {
		api.rc.signalRoleAPIsLocked()
		api.finishTurnAfterSignalLocked(turn, err)
		return api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
	}
	api.rc.signalRoleAPIsLocked()
	response = api.responseBaseLocked(req.CaseID, req.MemberID)
	response["ok"] = true
	response["result"] = result
	return response
}

func (api *councilAPIServer) revalidateCouncilEvidenceTurnLocked(caseCtx context.Context, req councilDoRequest, turn *councilTurn, reservation *evidenceReadReservation) map[string]any {
	if api.active != turn || turn.completed {
		rollbackEvidenceReadLocked(reservation)
		return api.staleCouncilEvidenceResponseLocked(req, turn)
	}
	if err := context.Cause(caseCtx); err != nil {
		rollbackEvidenceReadLocked(reservation)
		return api.councilToolErrorResponseLocked(caseCtx, req, turn, err)
	}
	if time.Now().After(turn.deadline) {
		rollbackEvidenceReadLocked(reservation)
		_, response := api.councilRequestTurnLocked(caseCtx, req)
		return response
	}
	return nil
}

func (api *councilAPIServer) staleCouncilEvidenceResponseLocked(req councilDoRequest, turn *councilTurn) map[string]any {
	response := api.responseBaseLocked(req.CaseID, req.MemberID)
	response["ok"] = false
	response["error"] = apiError("stale_opportunity", fmt.Sprintf("evidence file operation completed after opportunity %q ended", turn.opportunity.ID))
	return response
}

func (api *councilAPIServer) callCouncilToolLocked(caseCtx context.Context, turn *councilTurn, tool string, args map[string]any) (map[string]any, error) {
	switch tool {
	case "get_case":
		if err := requireAllowedKeys(args, "get_case arguments"); err != nil {
			return nil, participantInput(err)
		}
		return map[string]any{"case": api.rc.councilView(turn.seat, turn.opportunity)}, nil
	case "list_evidence":
		if err := requireAllowedKeys(args, "list_evidence arguments"); err != nil {
			return nil, participantInput(err)
		}
		evidence := api.rc.listVisibleEvidence()
		return map[string]any{"evidence": evidence}, nil
	case "stat_evidence", "read_evidence_range":
		return nil, fmt.Errorf("evidence file tool %q requires unlocked execution", tool)
	case "submit_council_vote":
		return api.submitCouncilVoteLocked(caseCtx, turn, args)
	default:
		return nil, participantInput(fmt.Errorf("unknown tool %q", tool))
	}
}

func (api *councilAPIServer) submitCouncilVoteLocked(caseCtx context.Context, turn *councilTurn, args map[string]any) (map[string]any, error) {
	if turn.completed {
		return nil, fmt.Errorf("council vote already submitted for this opportunity")
	}
	payload := cloneMap(args)
	if err := validateCouncilVotePayload(payload); err != nil {
		return nil, participantInput(err)
	}
	payload["member_id"] = turn.seat.MemberID
	stepResp, replayAction, err := api.rc.evaluateTurnStepLocked(caseCtx, turn.deadline, turn.opportunity, "submit_council_vote", "council", payload)
	if err != nil {
		if errors.Is(err, errTurnDeadlineExceeded) {
			if failErr := api.expireCouncilTurnLocked(caseCtx, turn); failErr != nil {
				api.finishTurnLocked(turn, failErr)
				return nil, failErr
			}
			api.finishTurnAfterSignalLocked(turn, nil)
			return nil, api.councilTurnDeadlineError(turn)
		}
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
	if err := api.rc.recordEventAtTurnLocked(turn.turnNumber, "council_vote", "council", turn.opportunity.Phase, map[string]any{
		"member_id": turn.seat.MemberID,
		"model":     turn.seat.Model,
		"backend":   councilBackendAPI,
		"payload":   payload,
	}); err != nil {
		api.finishTurnAfterSignalLocked(turn, err)
		return nil, err
	}
	api.finishTurnAfterSignalLocked(turn, nil)
	return map[string]any{"text": "Council vote accepted."}, nil
}

func validateCouncilVotePayload(payload map[string]any) error {
	if err := requireAllowedKeys(payload, "submit_council_vote arguments", "vote", "rationale"); err != nil {
		return err
	}
	vote, voteOK := payload["vote"].(string)
	rationale, rationaleOK := payload["rationale"].(string)
	vote = strings.TrimSpace(vote)
	rationale = strings.TrimSpace(rationale)
	if !voteOK || vote == "" || !rationaleOK || rationale == "" {
		return fmt.Errorf("submit_council_vote requires vote and rationale")
	}
	if vote != "demonstrated" && vote != "not_demonstrated" {
		return fmt.Errorf("submit_council_vote vote must be demonstrated or not_demonstrated")
	}
	payload["vote"] = vote
	payload["rationale"] = rationale
	return nil
}

func (api *councilAPIServer) consumeAttemptLocked(caseCtx context.Context, turn *councilTurn, err error) error {
	if turn.attemptsRemaining > 0 {
		turn.attemptsRemaining--
	}
	reason := strings.TrimSpace(err.Error())
	if reason == "" {
		reason = "invalid tool call"
	}
	turn.invalidReasons = append(turn.invalidReasons, reason)
	var feedback error
	if turn.attemptsRemaining > 0 {
		feedback = fmt.Errorf(
			"%s\nInvalid tool call %d of %d for this opportunity. %d invalid %s remain.",
			ensureTerminalPeriod(reason),
			len(turn.invalidReasons),
			turn.attemptsMax,
			turn.attemptsRemaining,
			invalidSubmissionWord(turn.attemptsRemaining),
		)
	} else {
		feedback = formatInvalidAttemptLimitError("council member "+turn.seat.MemberID, turn.invalidReasons)
	}
	if turn.attemptsRemaining <= 0 {
		details := map[string]any{
			"member_id":       turn.seat.MemberID,
			"model":           turn.seat.Model,
			"invalid_reasons": append([]string(nil), turn.invalidReasons...),
		}
		if failErr := api.rc.failOpportunityLocked(caseCtx, turn.deadline, turn.opportunity, opportunityFailureAttemptsExhausted, feedback.Error(), details); failErr != nil {
			if errors.Is(failErr, errTurnDeadlineExceeded) {
				if deadlineErr := api.expireCouncilTurnLocked(caseCtx, turn); deadlineErr != nil {
					api.finishTurnLocked(turn, deadlineErr)
					return deadlineErr
				}
				api.finishTurnAfterSignalLocked(turn, nil)
				return api.councilTurnDeadlineError(turn)
			}
			feedback = errors.Join(feedback, failErr)
			api.finishTurnLocked(turn, feedback)
			return feedback
		} else {
			api.finishTurnAfterSignalLocked(turn, nil)
		}
	} else {
		api.rc.signalRoleAPIsLocked()
	}
	return participantInput(feedback)
}

func (api *councilAPIServer) parseGetWaitIdentity(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	caseID := strings.TrimSpace(r.URL.Query().Get("case_id"))
	memberID := strings.TrimSpace(r.URL.Query().Get("member_id"))
	if caseID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": apiError("missing_case_id", "case_id is required"),
		})
		return "", "", false
	}
	if !api.caseIDMatches(caseID) {
		api.writeCaseMismatch(w, caseID, memberID)
		return "", "", false
	}
	if memberID == "" {
		writeCouncilJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"case_id": caseID,
			"error":   apiError("missing_member_id", "member_id is required"),
		})
		return "", "", false
	}
	return caseID, memberID, true
}

func (api *councilAPIServer) caseIDMatches(caseID string) bool {
	return strings.TrimSpace(caseID) == normalizeCaseID(api.rc.cfg.CaseID)
}

func (api *councilAPIServer) writeCaseMismatch(w http.ResponseWriter, caseID string, memberID string) {
	response := map[string]any{
		"ok":      false,
		"case_id": strings.TrimSpace(caseID),
		"error":   apiError("unknown_case", "case_id does not match this case process"),
	}
	if strings.TrimSpace(memberID) != "" {
		response["member_id"] = strings.TrimSpace(memberID)
	}
	writeCouncilJSON(w, http.StatusNotFound, response)
}

func (api *councilAPIServer) statusResponseLocked(caseID string, memberID string) map[string]any {
	terminalStatus := roleAPITerminalStatus(api.rc.state)
	if api.rc.terminal || terminalStatus != "" {
		response := api.responseBaseLocked(caseID, memberID)
		if terminalStatus == "failed" {
			response["status"] = "failed"
			response["failure"] = caseFailure(api.rc.state)
			response["error"] = caseFailureError(api.rc.state)
		} else {
			response["status"] = "done"
		}
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		if api.rc.terminalReason != "" {
			response["final_reason"] = api.rc.terminalReason
		}
		return response
	}
	response := api.responseBaseLocked(caseID, memberID)
	if failure := api.failedCouncilMemberPayloadLocked(memberID); failure != nil {
		response["status"] = "failed"
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		response["failure"] = failure
		response["error"] = mapString(failure["message"])
		return response
	}
	turn := api.active
	if turn == nil || turn.completed || turn.seat.MemberID != memberID {
		response["status"] = "waiting"
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		return response
	}
	response["status"] = "ready"
	response["prompt"] = turn.prompt
	response["tools"] = councilToolSpecs()
	response["limits"] = api.councilLimitsLocked(turn)
	return response
}

func (api *councilAPIServer) waitResponseLocked(caseID string, memberID string, after string, baseline uint64) (map[string]any, string, bool) {
	response := api.statusResponseLocked(caseID, memberID)
	if response["status"] == "failed" {
		return response, "failed", true
	}
	if response["status"] == "done" {
		return response, "done", true
	}
	turn := api.active
	if turn != nil && !turn.completed && turn.seat.MemberID == memberID {
		if after == "" || turn.opportunity.ID != after {
			return response, "ready", true
		}
	}
	if api.version != baseline {
		return response, "changed", true
	}
	return response, "", false
}

func (api *councilAPIServer) failedCouncilMemberPayloadLocked(memberID string) map[string]any {
	caseObj := mapAny(api.rc.state["case"])
	for _, member := range mapList(caseObj["council_members"]) {
		if mapString(member["member_id"]) != memberID {
			continue
		}
		if mapString(member["status"]) != "failed" {
			return nil
		}
		return map[string]any{
			"type":           "opportunity_failed",
			"role":           "council",
			"member_id":      memberID,
			"opportunity_id": mapString(member["failure_opportunity_id"]),
			"reason":         mapString(member["failure_reason"]),
			"message":        mapString(member["failure_message"]),
		}
	}
	return nil
}

func (api *councilAPIServer) waitPayloadLocked(reason string) map[string]any {
	return map[string]any{
		"reason":        reason,
		"version":       api.version,
		"state_version": mapAny(api.rc.state)["state_version"],
	}
}

func (api *councilAPIServer) responseBaseLocked(caseID string, memberID string) map[string]any {
	return map[string]any{
		"ok":        true,
		"case_id":   strings.TrimSpace(caseID),
		"member_id": strings.TrimSpace(memberID),
		"turn":      api.turnPayloadLocked(api.active),
	}
}

func (api *councilAPIServer) turnPayloadLocked(turn *councilTurn) map[string]any {
	if turn == nil {
		return nil
	}
	remaining := time.Until(turn.deadline).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"role_id":            "council",
		"member_id":          turn.seat.MemberID,
		"phase":              turn.opportunity.Phase,
		"opportunity_id":     turn.opportunity.ID,
		"turn_number":        turn.turnNumber,
		"deliberation_round": mapAny(api.rc.state["case"])["deliberation_round"],
		"deadline":           turn.deadline.UTC().Format(time.RFC3339Nano),
		"remaining_ms":       remaining,
		"attempts_max":       turn.attemptsMax,
		"attempts_remaining": turn.attemptsRemaining,
		"completed":          turn.completed,
	}
}

func (api *councilAPIServer) councilLimitsLocked(turn *councilTurn) map[string]any {
	limits := map[string]any{
		"max_response_bytes":                            api.rc.cfg.Runtime.MaxResponseBytes,
		"attempts_max":                                  turn.attemptsMax,
		"attempts_remaining":                            turn.attemptsRemaining,
		"max_evidence_read_bytes":                       api.rc.cfg.Policy.MaxEvidenceReadBytes,
		"max_evidence_reads_per_opportunity":            api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity,
		"max_evidence_read_bytes_per_opportunity":       api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity,
		"remaining_evidence_reads_for_opportunity":      remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads),
		"remaining_evidence_read_bytes_for_opportunity": remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes),
	}
	return limits
}

func (api *councilAPIServer) evidenceReadLimitsLocked(turn *councilTurn) map[string]any {
	return map[string]any{
		"max_read_bytes":                       api.rc.cfg.Policy.MaxEvidenceReadBytes,
		"max_reads_per_opportunity":            api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity,
		"max_read_bytes_per_opportunity":       api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity,
		"remaining_read_bytes_for_opportunity": remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes),
		"remaining_reads_for_opportunity":      remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads),
	}
}

func councilToolSpecs() []map[string]any {
	return []map[string]any{
		httpToolSpec("get_case", "Return the current visible arbitration record for this council member.", emptyObjectSchema(), true),
		httpToolSpec("list_evidence", "List visible immutable record evidence.", emptyObjectSchema(), true),
		httpToolSpec("stat_evidence", "Return metadata and read limits for one visible evidence item.", evidenceIDSchema(), true),
		httpToolSpec("read_evidence_range", "Read a bounded byte range from one visible evidence item as base64.", readEvidenceRangeSchema(), true),
		httpToolSpec("submit_council_vote", "Submit one council vote for the current deliberation opportunity.", submitCouncilVoteSchema(), false),
	}
}

func submitCouncilVoteSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vote":      map[string]any{"type": "string", "enum": []string{"demonstrated", "not_demonstrated"}},
			"rationale": map[string]any{"type": "string"},
		},
		"required":             []string{"vote", "rationale"},
		"additionalProperties": false,
	}
}

func parseCouncilAPIWaitTimeout(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultCouncilAPIWaitTimeout, nil
	}
	ms, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("timeout_ms must be an integer")
	}
	if ms <= 0 {
		return 0, fmt.Errorf("timeout_ms must be positive")
	}
	timeout := time.Duration(ms) * time.Millisecond
	if timeout > maxCouncilAPIWaitTimeout {
		return 0, fmt.Errorf("timeout_ms must be at most %d", maxCouncilAPIWaitTimeout.Milliseconds())
	}
	return timeout, nil
}

func writeCouncilJSON(w http.ResponseWriter, status int, value map[string]any) {
	writeCaseAPIJSON(w, status, value)
}

func (rc *runContext) buildCouncilAPIPrompt(seat CouncilSeat, opportunity Opportunity) (string, error) {
	base, err := rc.buildCouncilPrompt(seat, opportunity)
	if err != nil {
		return "", err
	}
	return base + "\n\nCouncil API instructions:\n" +
		"You are a council member. Decide the proposition from the admitted record.\n" +
		"You may examine admitted evidence through read-only tools when exact bytes, metadata, or exhibit contents matter.\n" +
		"Do not search the web, introduce new facts, create new evidence, or upload evidence.\n" +
		"When ready, call submit_council_vote exactly once with vote=demonstrated or vote=not_demonstrated and a concise rationale.\n", nil
}
