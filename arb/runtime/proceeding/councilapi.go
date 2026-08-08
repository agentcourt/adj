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

	mu      sync.Mutex
	cond    *sync.Cond
	version uint64
	active  *councilTurn

	terminal       bool
	terminalReason string
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
	api.cond = sync.NewCond(&api.mu)
	return api
}

func (api *councilAPIServer) register(mux *http.ServeMux) {
	mux.HandleFunc(councilAPIBasePath+"/get", api.handleGet)
	mux.HandleFunc(councilAPIBasePath+"/wait", api.handleWait)
	mux.HandleFunc(councilAPIBasePath+"/do", api.handleDo)
	mux.HandleFunc(councilAPIBasePath+"/fail", api.handleFail)
}

func (api *councilAPIServer) startTurn(turn *councilTurn) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active != nil && !api.active.completed {
		return fmt.Errorf("councilapi already has an active turn")
	}
	api.active = turn
	api.signalChangedLocked()
	return nil
}

func (api *councilAPIServer) clearTurn(turn *councilTurn) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active == turn {
		api.active = nil
		api.signalChangedLocked()
	}
}

func (api *councilAPIServer) setTerminal(reason string) {
	if api == nil {
		return
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.terminal = true
	api.terminalReason = strings.TrimSpace(reason)
	api.active = nil
	api.signalChangedLocked()
}

func (api *councilAPIServer) signalChanged() {
	if api == nil {
		return
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.signalChangedLocked()
}

func (api *councilAPIServer) signalChangedLocked() {
	api.version++
	api.ensureCondLocked().Broadcast()
}

func (api *councilAPIServer) ensureCondLocked() *sync.Cond {
	if api.cond == nil {
		api.cond = sync.NewCond(&api.mu)
	}
	return api.cond
}

func (rc *runContext) executeCouncilAPIOpportunity(ctx context.Context, opportunity Opportunity, seat CouncilSeat) error {
	if rc.councilAPI == nil {
		return fmt.Errorf("councilapi server is not running")
	}
	prompt, err := rc.buildCouncilAPIPrompt(seat, opportunity)
	if err != nil {
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
		return err
	}
	if err := rc.councilAPI.startTurn(turn); err != nil {
		return err
	}
	defer rc.councilAPI.clearTurn(turn)
	timer := time.NewTimer(time.Until(turn.deadline))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		err := fmt.Errorf("council member %s opportunity timed out after %s", seat.MemberID, rc.cfg.Runtime.CouncilTimeout())
		rc.councilAPI.finishTurn(turn, err)
		return rc.removeTimedOutCouncilMember(opportunity, seat, err)
	case err := <-turn.done:
		if err != nil {
			return rc.removeInvalidResponseCouncilMember(opportunity, seat, err)
		}
		return nil
	}
}

func (api *councilAPIServer) finishTurn(turn *councilTurn, err error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.finishTurnLocked(turn, err)
}

func (api *councilAPIServer) finishTurnLocked(turn *councilTurn, err error) {
	if turn == nil || turn.completed {
		return
	}
	turn.completed = true
	api.signalChangedLocked()
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
	api.mu.Lock()
	response := api.statusResponseLocked(caseID, memberID)
	api.mu.Unlock()
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

	api.mu.Lock()
	cond := api.ensureCondLocked()
	baseline := api.version
	if hasAfterVersion {
		baseline = afterVersion
	}
	deadline := time.Now().Add(timeout)
	timer := time.AfterFunc(timeout, func() {
		api.mu.Lock()
		api.ensureCondLocked().Broadcast()
		api.mu.Unlock()
	})
	defer timer.Stop()
	if done := r.Context().Done(); done != nil {
		go func() {
			<-done
			api.mu.Lock()
			api.ensureCondLocked().Broadcast()
			api.mu.Unlock()
		}()
	}
	for {
		if r.Context().Err() != nil {
			api.mu.Unlock()
			return
		}
		if response, reason, ready := api.waitResponseLocked(caseID, memberID, after, baseline); ready {
			response["wait"] = api.waitPayloadLocked(reason)
			api.mu.Unlock()
			writeCouncilJSON(w, http.StatusOK, response)
			return
		}
		if !time.Now().Before(deadline) {
			response := api.statusResponseLocked(caseID, memberID)
			response["wait"] = api.waitPayloadLocked("timeout")
			api.mu.Unlock()
			writeCouncilJSON(w, http.StatusOK, response)
			return
		}
		cond.Wait()
	}
}

func (api *councilAPIServer) handleDo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCouncilJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use POST"),
		})
		return
	}
	var req councilDoRequest
	body := http.MaxBytesReader(w, r.Body, int64(api.rc.cfg.Runtime.MaxResponseBytes))
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			err = fmt.Errorf("request body is required")
		}
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
	api.handleCouncilDo(w, req)
}

func (api *councilAPIServer) handleFail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCouncilJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use POST"),
		})
		return
	}
	var req councilFailRequest
	body := http.MaxBytesReader(w, r.Body, int64(api.rc.cfg.Runtime.MaxResponseBytes))
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			err = fmt.Errorf("request body is required")
		}
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
	api.handleCouncilFail(w, req)
}

func (api *councilAPIServer) handleCouncilFail(w http.ResponseWriter, req councilFailRequest) {
	api.mu.Lock()
	defer api.mu.Unlock()
	turn := api.active
	response := api.responseBaseLocked(req.CaseID, req.MemberID)
	if turn == nil || turn.completed {
		response["ok"] = false
		response["error"] = apiError("no_active_turn", "no council turn is active")
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	if turn.seat.MemberID != req.MemberID {
		response["ok"] = false
		response["error"] = apiError("not_current_turn", fmt.Sprintf("current council turn belongs to %s", turn.seat.MemberID))
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	if req.OpportunityID != turn.opportunity.ID {
		response["ok"] = false
		response["error"] = apiError("stale_opportunity", fmt.Sprintf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, turn.opportunity.ID))
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	details := cloneMap(req.Details)
	details["member_id"] = turn.seat.MemberID
	details["model"] = turn.seat.Model
	if failErr := api.rc.failOpportunity(turn.opportunity, req.Reason, req.Message, details); failErr != nil {
		api.finishTurnLocked(turn, failErr)
		response["ok"] = false
		response["error"] = apiError("member_failure_failed", failErr.Error())
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	api.finishTurnLocked(turn, nil)
	response["ok"] = true
	response["result"] = map[string]any{"text": "Council member failure recorded."}
	if failure := api.failedCouncilMemberPayloadLocked(req.MemberID); failure != nil {
		response["failure"] = failure
	}
	writeCouncilJSON(w, http.StatusOK, response)
}

func (api *councilAPIServer) handleCouncilDo(w http.ResponseWriter, req councilDoRequest) {
	api.mu.Lock()
	defer api.mu.Unlock()
	turn := api.active
	if turn == nil || turn.completed {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("no_active_turn", "no council turn is active")
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	if time.Now().After(turn.deadline) {
		err := fmt.Errorf("council member %s opportunity timed out", turn.seat.MemberID)
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		if failErr := api.rc.failOpportunity(turn.opportunity, opportunityFailureDeadline, err.Error(), map[string]any{
			"member_id": turn.seat.MemberID,
			"model":     turn.seat.Model,
		}); failErr != nil {
			api.finishTurnLocked(turn, failErr)
			response["error"] = apiError("member_failure_failed", failErr.Error())
		} else {
			api.finishTurnLocked(turn, nil)
			response["error"] = apiError("turn_timeout", err.Error())
		}
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	if turn.seat.MemberID != req.MemberID {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("not_current_turn", fmt.Sprintf("current council turn belongs to %s", turn.seat.MemberID))
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	if req.OpportunityID == "" {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("missing_opportunity_id", "opportunity_id is required for council tool calls")
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	if req.OpportunityID != turn.opportunity.ID {
		response := api.responseBaseLocked(req.CaseID, req.MemberID)
		response["ok"] = false
		response["error"] = apiError("stale_opportunity", fmt.Sprintf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, turn.opportunity.ID))
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	result, countAttempt, err := api.callCouncilToolLocked(turn, req.Tool, req.Arguments)
	response := api.responseBaseLocked(req.CaseID, req.MemberID)
	if err != nil {
		if countAttempt {
			err = api.consumeAttemptLocked(turn, err)
		}
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
		writeCouncilJSON(w, http.StatusOK, response)
		return
	}
	response["ok"] = true
	response["result"] = result
	writeCouncilJSON(w, http.StatusOK, response)
}

func (api *councilAPIServer) callCouncilToolLocked(turn *councilTurn, tool string, args map[string]any) (map[string]any, bool, error) {
	switch tool {
	case "get_case":
		return map[string]any{"case": api.rc.councilView(turn.seat, turn.opportunity)}, false, nil
	case "list_evidence":
		evidence := api.rc.listVisibleEvidence()
		return map[string]any{"evidence": evidence}, false, nil
	case "stat_evidence":
		evidence, err := api.rc.statEvidence(mapString(args["evidence_id"]))
		if err != nil {
			return nil, false, err
		}
		return map[string]any{"evidence": evidence, "limits": api.evidenceReadLimitsLocked(turn)}, false, nil
	case "read_evidence_range":
		offset, err := requiredIntParam(args, "offset")
		if err != nil {
			return nil, false, err
		}
		length, err := requiredIntParam(args, "length")
		if err != nil {
			return nil, false, err
		}
		result, err := api.rc.readEvidenceRange(mapString(args["evidence_id"]), int64(offset), length, turn.evidenceBudget)
		if err != nil {
			return nil, false, err
		}
		result["remaining_read_bytes_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes)
		result["remaining_reads_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads)
		if err := api.rc.recordEventAtTurn(turn.turnNumber, "evidence_read", "council", turn.opportunity.Phase, map[string]any{
			"member_id":   turn.seat.MemberID,
			"evidence_id": result["evidence_id"],
			"offset":      result["offset"],
			"length":      result["length"],
			"byte_count":  result["length"],
		}); err != nil {
			return nil, false, err
		}
		return result, false, nil
	case "submit_council_vote":
		result, err := api.submitCouncilVoteLocked(turn, args)
		return result, err != nil, err
	default:
		return nil, true, fmt.Errorf("unknown tool %q", tool)
	}
}

func (api *councilAPIServer) submitCouncilVoteLocked(turn *councilTurn, args map[string]any) (map[string]any, error) {
	if turn.completed {
		return nil, fmt.Errorf("council vote already submitted for this opportunity")
	}
	payload := cloneMap(args)
	payload["member_id"] = turn.seat.MemberID
	if mapString(payload["vote"]) == "" || mapString(payload["rationale"]) == "" {
		return nil, fmt.Errorf("submit_council_vote requires vote and rationale")
	}
	stepResp, err := api.rc.stepForCertificate("submit_council_vote", "council", payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return nil, fmt.Errorf("%s", mapString(stepResp["error"]))
	}
	api.rc.state = mapAny(stepResp["state"])
	api.signalChangedLocked()
	if api.rc.lawyerAPI != nil {
		api.rc.lawyerAPI.signalChanged()
	}
	if err := api.rc.recordEventAtTurn(turn.turnNumber, "council_vote", "council", turn.opportunity.Phase, map[string]any{
		"member_id": turn.seat.MemberID,
		"model":     turn.seat.Model,
		"backend":   councilBackendAPI,
		"payload":   payload,
	}); err != nil {
		return nil, err
	}
	api.finishTurnLocked(turn, nil)
	return map[string]any{"text": "Council vote accepted."}, nil
}

func (api *councilAPIServer) consumeAttemptLocked(turn *councilTurn, err error) error {
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
		if failErr := api.rc.failOpportunity(turn.opportunity, opportunityFailureAttemptsExhausted, feedback.Error(), details); failErr != nil {
			feedback = errors.Join(feedback, failErr)
			api.finishTurnLocked(turn, feedback)
		} else {
			api.finishTurnLocked(turn, nil)
		}
	}
	return feedback
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
	if api.terminal {
		response := api.responseBaseLocked(caseID, memberID)
		if mapString(mapAny(api.rc.state["case"])["status"]) == "failed" {
			response["status"] = "failed"
			response["failure"] = caseFailure(api.rc.state)
			response["error"] = caseFailureError(api.rc.state)
		} else {
			response["status"] = "done"
		}
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		if api.terminalReason != "" {
			response["final_reason"] = api.terminalReason
		}
		return response
	}
	response := api.responseBaseLocked(caseID, memberID)
	if mapString(mapAny(api.rc.state["case"])["status"]) == "failed" {
		response["status"] = "failed"
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		response["failure"] = caseFailure(api.rc.state)
		response["error"] = caseFailureError(api.rc.state)
		return response
	}
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
	if api.terminal {
		if response["status"] == "failed" {
			return response, "failed", true
		}
		return response, "done", true
	}
	if response["status"] == "failed" {
		return response, "failed", true
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
