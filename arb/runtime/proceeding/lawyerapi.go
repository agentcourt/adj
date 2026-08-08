package proceeding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

	mu      sync.Mutex
	cond    *sync.Cond
	version uint64
	active  *lawyerTurn

	terminal       bool
	terminalReason string
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
	api.cond = sync.NewCond(&api.mu)
	return api
}

func (api *lawyerAPIServer) register(mux *http.ServeMux) {
	mux.HandleFunc(lawyerAPIBasePath+"/get", api.handleGet)
	mux.HandleFunc(lawyerAPIBasePath+"/wait", api.handleWait)
	mux.HandleFunc(lawyerAPIBasePath+"/status", api.handleStatus)
	mux.HandleFunc(lawyerAPIBasePath+"/result", api.handleResult)
	mux.HandleFunc(lawyerAPIBasePath+"/do", api.handleDo)
}

func (api *lawyerAPIServer) startTurn(turn *lawyerTurn) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active != nil && !api.active.completed {
		return fmt.Errorf("lawyerapi already has an active turn")
	}
	api.active = turn
	api.signalChangedLocked()
	return nil
}

func (api *lawyerAPIServer) clearTurn(turn *lawyerTurn) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.active == turn {
		api.active = nil
		api.signalChangedLocked()
	}
}

func (api *lawyerAPIServer) setTerminal(reason string) {
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

func (api *lawyerAPIServer) signalChanged() {
	if api == nil {
		return
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.signalChangedLocked()
}

func (api *lawyerAPIServer) signalChangedLocked() {
	api.version++
	api.ensureCondLocked().Broadcast()
}

func (api *lawyerAPIServer) ensureCondLocked() *sync.Cond {
	if api.cond == nil {
		api.cond = sync.NewCond(&api.mu)
	}
	return api.cond
}

func (rc *runContext) executeAttorneyOpportunity(ctx context.Context, _ any, opportunity Opportunity) error {
	if err := validateAttorneyRole(opportunity.Role); err != nil {
		return err
	}
	if rc.lawyerAPI == nil {
		return fmt.Errorf("lawyerapi server is not running")
	}
	prompt, err := rc.buildAttorneyPrompt(opportunity)
	if err != nil {
		return err
	}
	turn := &lawyerTurn{
		opportunity:       opportunity,
		turnNumber:        rc.turn,
		prompt:            prompt,
		deadline:          time.Now().Add(rc.cfg.Runtime.LawyerTurnTimeout()),
		attemptsMax:       rc.cfg.Runtime.InvalidAttemptLimit,
		attemptsRemaining: rc.cfg.Runtime.InvalidAttemptLimit,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
	if err := rc.lawyerAPI.startTurn(turn); err != nil {
		return err
	}
	defer rc.lawyerAPI.clearTurn(turn)
	timer := time.NewTimer(time.Until(turn.deadline))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		err := fmt.Errorf("%s lawyer opportunity timed out after %s", opportunity.Role, rc.cfg.Runtime.LawyerTurnTimeout())
		failErr := rc.failOpportunity(opportunity, opportunityFailureDeadline, err.Error(), nil)
		rc.lawyerAPI.finishTurn(turn, failErr)
		return failErr
	case err := <-turn.done:
		return err
	}
}

func (api *lawyerAPIServer) finishTurn(turn *lawyerTurn, err error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.finishTurnLocked(turn, err)
}

func (api *lawyerAPIServer) finishTurnLocked(turn *lawyerTurn, err error) {
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
		api.mu.Lock()
		response := api.statusResponseLocked(caseID, role)
		api.mu.Unlock()
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
	api.mu.Lock()
	response := api.statusResponseLocked(caseID, role)
	api.mu.Unlock()
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
		if response, reason, ready := api.waitResponseLocked(caseID, role, after, baseline); ready {
			response["wait"] = api.waitPayloadLocked(reason)
			api.mu.Unlock()
			writeLawyerJSON(w, http.StatusOK, response)
			return
		}
		if !time.Now().Before(deadline) {
			response := api.statusResponseLocked(caseID, role)
			response["wait"] = api.waitPayloadLocked("timeout")
			api.mu.Unlock()
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
	api.mu.Lock()
	response := api.caseStatusResponseLocked(caseID, role)
	api.mu.Unlock()
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
	api.mu.Lock()
	response := api.caseResultResponseLocked(caseID, role)
	api.mu.Unlock()
	writeLawyerJSON(w, http.StatusOK, response)
}

func (api *lawyerAPIServer) handleDo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLawyerJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use POST"),
		})
		return
	}
	var req lawyerDoRequest
	dec := json.NewDecoder(r.Body)
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
		api.handleObserverDo(w, req)
		return
	}
	api.handleLawyerDo(w, req)
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

func (api *lawyerAPIServer) handleLawyerDo(w http.ResponseWriter, req lawyerDoRequest) {
	if err := validateAttorneyRole(req.RoleID); err != nil {
		writeLawyerJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"case_id": req.CaseID,
			"role_id": req.RoleID,
			"error":   apiError("invalid_role", err.Error()),
		})
		return
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if req.Tool == "case_status" {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = true
		response["result"] = api.caseStatusPayloadLocked(req.RoleID)
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	turn := api.active
	if turn == nil || turn.completed {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("no_active_turn", "no lawyer turn is active")
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	if time.Now().After(turn.deadline) {
		err := fmt.Errorf("%s lawyer opportunity timed out", turn.opportunity.Role)
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		if failErr := api.rc.failOpportunity(turn.opportunity, opportunityFailureDeadline, err.Error(), nil); failErr != nil {
			api.finishTurnLocked(turn, failErr)
			response["error"] = apiError("case_failure_failed", failErr.Error())
		} else {
			api.finishTurnLocked(turn, nil)
			response["error"] = apiError("turn_timeout", err.Error())
		}
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	if turn.opportunity.Role != req.RoleID {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("not_current_turn", fmt.Sprintf("current turn belongs to %s", turn.opportunity.Role))
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	if req.OpportunityID == "" {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("missing_opportunity_id", "opportunity_id is required for lawyer tool calls")
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	if req.OpportunityID != turn.opportunity.ID {
		response := api.responseBaseLocked(req.CaseID, req.RoleID)
		response["ok"] = false
		response["error"] = apiError("stale_opportunity", fmt.Sprintf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, turn.opportunity.ID))
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	result, countAttempt, decisionAttempt, err := api.callLawyerToolLocked(turn, req.Tool, req.Arguments, req.CallID)
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	if err != nil {
		if countAttempt {
			err = api.consumeAttemptLocked(turn, err, decisionAttempt)
		}
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	response["ok"] = true
	response["result"] = result
	writeLawyerJSON(w, http.StatusOK, response)
}

func (api *lawyerAPIServer) handleObserverDo(w http.ResponseWriter, req lawyerDoRequest) {
	api.mu.Lock()
	defer api.mu.Unlock()
	result, err := api.callObserverToolLocked(req.Tool, req.Arguments)
	response := api.responseBaseLocked(req.CaseID, req.RoleID)
	if err != nil {
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
		writeLawyerJSON(w, http.StatusOK, response)
		return
	}
	response["ok"] = true
	response["result"] = result
	writeLawyerJSON(w, http.StatusOK, response)
}

func (api *lawyerAPIServer) callLawyerToolLocked(turn *lawyerTurn, tool string, args map[string]any, callID string) (map[string]any, bool, bool, error) {
	switch tool {
	case "case_status":
		return api.caseStatusPayloadLocked(turn.opportunity.Role), false, false, nil
	case "get_case":
		view := api.rc.attorneyView(turn.opportunity)
		return map[string]any{"case": view}, false, false, nil
	case "send_work_notes":
		notes, ok := args["notes"].(string)
		if !ok {
			return nil, false, false, fmt.Errorf("arguments.notes is required and must be a string")
		}
		if err := api.rc.recordWorkNotesAtTurn(turn.turnNumber, turn.opportunity, callID, notes); err != nil {
			return nil, false, false, err
		}
		return map[string]any{
			"text":       "Work notes accepted off record.",
			"byte_count": len([]byte(notes)),
		}, false, false, nil
	case "list_evidence":
		if !evidenceReadAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence access is not allowed in phase %q", turn.opportunity.Phase)
		}
		return map[string]any{"evidence": api.rc.listVisibleEvidence()}, false, false, nil
	case "stat_evidence":
		if !evidenceReadAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence access is not allowed in phase %q", turn.opportunity.Phase)
		}
		evidence, err := api.rc.statEvidence(mapString(args["evidence_id"]))
		if err != nil {
			return nil, false, false, err
		}
		return map[string]any{"evidence": evidence, "limits": api.evidenceReadLimitsLocked(turn)}, false, false, nil
	case "read_evidence_range":
		if !evidenceReadAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence access is not allowed in phase %q", turn.opportunity.Phase)
		}
		offset, err := requiredIntParam(args, "offset")
		if err != nil {
			return nil, false, false, err
		}
		length, err := requiredIntParam(args, "length")
		if err != nil {
			return nil, false, false, err
		}
		result, err := api.rc.readEvidenceRange(mapString(args["evidence_id"]), int64(offset), length, turn.evidenceBudget)
		if err != nil {
			return nil, false, false, err
		}
		result["remaining_read_bytes_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes)
		result["remaining_reads_for_opportunity"] = remainingCapacity(api.rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads)
		if err := api.rc.recordEventAtTurn(turn.turnNumber, "evidence_read", turn.opportunity.Role, turn.opportunity.Phase, map[string]any{
			"evidence_id": result["evidence_id"],
			"offset":      result["offset"],
			"length":      result["length"],
			"byte_count":  result["length"],
		}); err != nil {
			return nil, false, false, err
		}
		return result, false, false, nil
	case "begin_evidence_upload":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase)
		}
		session, err := api.rc.beginEvidenceUpload(turn.opportunity, args)
		if err != nil {
			return nil, true, false, err
		}
		return map[string]any{
			"upload_id":              session.UploadID,
			"max_chunk_bytes":        api.rc.cfg.Policy.MaxEvidenceChunkBytes,
			"remaining_upload_bytes": session.ExpectedSizeBytes,
		}, false, false, nil
	case "write_evidence_chunk":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase)
		}
		offset, err := requiredIntParam(args, "offset")
		if err != nil {
			return nil, true, false, err
		}
		session, n, err := api.rc.writeEvidenceChunk(mapString(args["upload_id"]), offset, mapString(args["content_base64"]))
		if err != nil {
			return nil, true, false, err
		}
		return map[string]any{
			"upload_id":              session.UploadID,
			"accepted_offset":        session.ReceivedBytes - n,
			"accepted_length":        n,
			"received_bytes":         session.ReceivedBytes,
			"remaining_upload_bytes": remainingCapacity(session.ExpectedSizeBytes, session.ReceivedBytes),
		}, false, false, nil
	case "commit_evidence_upload":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase)
		}
		result, err := api.commitEvidenceUploadLocked(turn, args)
		return result, err != nil, false, err
	case "submit_evidence":
		if !evidenceSubmissionAllowed(turn.opportunity) {
			return nil, true, false, fmt.Errorf("evidence submission is not allowed in phase %q", turn.opportunity.Phase)
		}
		result, err := api.submitEvidenceLocked(turn, args)
		return result, err != nil, false, err
	case "submit_decision":
		result, err := api.submitDecisionLocked(turn, args)
		return result, err != nil, true, err
	default:
		return nil, true, false, fmt.Errorf("unknown tool %q", tool)
	}
}

func (api *lawyerAPIServer) commitEvidenceUploadLocked(turn *lawyerTurn, args map[string]any) (map[string]any, error) {
	uploadID := mapString(args["upload_id"])
	session := api.rc.uploadSessions[uploadID]
	if session == nil {
		return nil, fmt.Errorf("unknown upload_id %q", uploadID)
	}
	if expected := strings.ToLower(mapString(args["expected_sha256"])); expected != "" {
		session.ExpectedSHA256 = expected
	}
	meta, err := api.rc.prepareEvidenceUploadCommit(session, mapString(args["preferred_filename_ext"]))
	if err != nil {
		return nil, err
	}
	payload := submittedEvidencePayload(meta)
	stepResp, err := api.rc.stepForCertificate("submit_evidence", turn.opportunity.Role, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return nil, fmt.Errorf("%s", mapString(stepResp["error"]))
	}
	meta, file, evidence, err := api.rc.finalizeEvidenceUpload(session, meta)
	if err != nil {
		return nil, err
	}
	api.rc.state = mapAny(stepResp["state"])
	api.signalChangedLocked()
	if api.rc.councilAPI != nil {
		api.rc.councilAPI.signalChanged()
	}
	api.rc.caseFiles = append(api.rc.caseFiles, file)
	api.rc.fileByID[file.EvidenceID] = file
	api.rc.submittedEvidence = append(api.rc.submittedEvidence, meta)
	if err := api.rc.writeEvidenceManifest(); err != nil {
		return nil, err
	}
	if err := api.recordSubmittedEvidenceEventLocked(turn, meta); err != nil {
		return nil, err
	}
	return map[string]any{
		"text":        fmt.Sprintf("Evidence upload accepted as evidence_id %s. Cite this evidence_id in offered_evidence if you want it admitted as an exhibit.", meta.EvidenceID),
		"evidence_id": meta.EvidenceID,
		"evidence":    evidence,
	}, nil
}

func (api *lawyerAPIServer) submitEvidenceLocked(turn *lawyerTurn, args map[string]any) (map[string]any, error) {
	meta, raw, err := api.rc.prepareSubmittedEvidence(turn.opportunity, args)
	if err != nil {
		return nil, err
	}
	payload := submittedEvidencePayload(meta)
	stepResp, err := api.rc.stepForCertificate("submit_evidence", turn.opportunity.Role, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return nil, fmt.Errorf("%s", mapString(stepResp["error"]))
	}
	file, err := api.rc.writeSubmittedEvidenceFile(meta, raw)
	if err != nil {
		return nil, err
	}
	evidence, err := api.rc.registerSubmittedEvidenceEvidence(meta, file)
	if err != nil {
		return nil, err
	}
	meta.EvidenceID = evidence.EvidenceID
	api.rc.state = mapAny(stepResp["state"])
	api.signalChangedLocked()
	if api.rc.councilAPI != nil {
		api.rc.councilAPI.signalChanged()
	}
	api.rc.caseFiles = append(api.rc.caseFiles, file)
	api.rc.fileByID[file.EvidenceID] = file
	api.rc.submittedEvidence = append(api.rc.submittedEvidence, meta)
	if err := api.rc.writeEvidenceManifest(); err != nil {
		return nil, err
	}
	if err := api.recordSubmittedEvidenceEventLocked(turn, meta); err != nil {
		return nil, err
	}
	return map[string]any{
		"text":        fmt.Sprintf("Evidence accepted as evidence_id %s. Cite this evidence_id in offered_evidence if you want it admitted as an exhibit.", meta.EvidenceID),
		"evidence_id": meta.EvidenceID,
		"evidence":    evidence,
	}, nil
}

func (api *lawyerAPIServer) submitDecisionLocked(turn *lawyerTurn, args map[string]any) (map[string]any, error) {
	if turn.completed {
		return nil, fmt.Errorf("decision already submitted for this opportunity")
	}
	actionType, payload, err := attorneyDecision(turn.opportunity, args, api.rc.fileByID, api.rc.cfg.Policy)
	if err != nil {
		return nil, err
	}
	if err := api.rc.validateAttorneyPayloadAgainstState(turn.opportunity, actionType, payload); err != nil {
		return nil, err
	}
	stepResp, err := api.rc.stepForCertificate(actionType, turn.opportunity.Role, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); !ok {
		return nil, fmt.Errorf("%s", mapString(stepResp["error"]))
	}
	api.rc.state = mapAny(stepResp["state"])
	api.signalChangedLocked()
	if api.rc.councilAPI != nil {
		api.rc.councilAPI.signalChanged()
	}
	if err := api.rc.recordEventAtTurn(turn.turnNumber, "attorney_action", turn.opportunity.Role, turn.opportunity.Phase, map[string]any{
		"opportunity_id": turn.opportunity.ID,
		"action_type":    actionType,
		"payload":        payload,
	}); err != nil {
		return nil, err
	}
	api.finishTurnLocked(turn, nil)
	return map[string]any{"text": "Decision accepted."}, nil
}

func (api *lawyerAPIServer) recordSubmittedEvidenceEventLocked(turn *lawyerTurn, meta SubmittedEvidenceMeta) error {
	return api.rc.recordEventAtTurn(turn.turnNumber, "submitted_evidence", turn.opportunity.Role, turn.opportunity.Phase, map[string]any{
		"evidence_id":         meta.EvidenceID,
		"title":               meta.Title,
		"source_url":          meta.SourceURL,
		"source_description":  meta.SourceDescription,
		"mime_type":           meta.MimeType,
		"retrieval_timestamp": meta.RetrievalTimestamp,
		"relevance":           meta.Relevance,
		"sha256":              meta.SHA256,
		"size_bytes":          meta.SizeBytes,
	})
}

func (api *lawyerAPIServer) callObserverToolLocked(tool string, args map[string]any) (map[string]any, error) {
	switch tool {
	case "case_status":
		return api.caseStatusPayloadLocked("observer"), nil
	case "get_case":
		return map[string]any{"case": api.observerViewLocked()}, nil
	case "get_turn":
		return map[string]any{"turn": api.turnPayloadLocked(api.active)}, nil
	case "list_events":
		offset, err := optionalIntParam(args, "offset", 0)
		if err != nil {
			return nil, err
		}
		limit, err := optionalIntParam(args, "limit", 100)
		if err != nil {
			return nil, err
		}
		if offset < 0 {
			return nil, fmt.Errorf("offset must be non-negative")
		}
		if limit <= 0 || limit > 1000 {
			return nil, fmt.Errorf("limit must be between 1 and 1000")
		}
		events := append([]Event(nil), api.rc.events...)
		if offset > len(events) {
			offset = len(events)
		}
		end := offset + limit
		if end > len(events) {
			end = len(events)
		}
		return map[string]any{"events": events[offset:end], "offset": offset, "limit": limit, "total": len(events)}, nil
	case "list_evidence":
		return map[string]any{"evidence": api.rc.listVisibleEvidence()}, nil
	case "stat_evidence":
		evidence, err := api.rc.statEvidence(mapString(args["evidence_id"]))
		if err != nil {
			return nil, err
		}
		return map[string]any{"evidence": evidence}, nil
	case "read_evidence_range":
		offset, err := requiredIntParam(args, "offset")
		if err != nil {
			return nil, err
		}
		length, err := requiredIntParam(args, "length")
		if err != nil {
			return nil, err
		}
		return api.rc.readEvidenceRange(mapString(args["evidence_id"]), int64(offset), length, nil)
	default:
		return nil, fmt.Errorf("unknown observer tool %q", tool)
	}
}

func (api *lawyerAPIServer) observerViewLocked() map[string]any {
	caseObj := mapAny(api.rc.state["case"])
	return map[string]any{
		"proposition":       api.rc.complaint.Proposition,
		"evidence_standard": currentEvidenceStandard(api.rc.state, api.rc.cfg.Policy),
		"phase":             currentPhase(api.rc.state),
		"resolution":        currentResolution(api.rc.state),
		"record": map[string]any{
			"evidence":           api.rc.listVisibleEvidence(),
			"openings":           mapList(caseObj["openings"]),
			"arguments":          mapList(caseObj["arguments"]),
			"rebuttals":          mapList(caseObj["rebuttals"]),
			"surrebuttals":       mapList(caseObj["surrebuttals"]),
			"closings":           mapList(caseObj["closings"]),
			"submitted_evidence": mapList(caseObj["submitted_evidence"]),
			"exhibits":           api.rc.attorneyExhibits(),
			"technical_reports":  mapList(caseObj["technical_reports"]),
			"council_votes":      mapList(caseObj["council_votes"]),
		},
		"turn":   api.turnPayloadLocked(api.active),
		"events": len(api.rc.events),
		"policy": api.rc.cfg.Policy.StateMap(),
	}
}

func (api *lawyerAPIServer) caseStatusResponseLocked(caseID string, roleID string) map[string]any {
	response := api.responseBaseLocked(caseID, roleID)
	for key, value := range api.caseStatusPayloadLocked(roleID) {
		response[key] = value
	}
	return response
}

func (api *lawyerAPIServer) caseStatusPayloadLocked(roleID string) map[string]any {
	caseObj := mapAny(api.rc.state["case"])
	phase := currentPhase(api.rc.state)
	caseStatus := mapString(caseObj["status"])
	resolution := currentResolution(api.rc.state)
	roleStatus := api.caseRoleStatusLocked(roleID, caseObj)
	payload := map[string]any{
		"status":              roleStatus,
		"phase":               phase,
		"case_status":         caseStatus,
		"resolution":          resolution,
		"council_backend":     api.rc.cfg.CouncilBackend,
		"council_roster":      councilSeatRoster(api.rc.council, mapList(caseObj["council_members"])),
		"deliberation_round":  intNumber(caseObj["deliberation_round"]),
		"state_version":       mapAny(api.rc.state)["state_version"],
		"turn":                api.turnPayloadLocked(api.active),
		"current_opportunity": api.currentOpportunityPayloadLocked(),
		"counts":              api.caseStatusCountsLocked(caseObj),
		"message":             caseStatusMessage(roleStatus),
	}
	if api.terminalReason != "" {
		payload["final_reason"] = api.terminalReason
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
	return payload
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
		"council_votes":      len(mapList(caseObj["council_votes"])),
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
	resolution := currentResolution(api.rc.state)
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
	votes := normalizeCouncilVotes(mapList(caseObj["council_votes"]))
	response["status"] = "done"
	if api.terminalReason != "" {
		response["final_reason"] = api.terminalReason
	}
	response["result"] = map[string]any{
		"resolution":         resolution,
		"phase":              phase,
		"case_status":        caseStatus,
		"final_reason":       api.terminalReason,
		"council_votes":      votes,
		"vote_tally":         councilVoteTally(votes),
		"deliberation_round": intNumber(caseObj["deliberation_round"]),
	}
	return response
}

func (api *lawyerAPIServer) caseIsFinalLocked(caseObj map[string]any) bool {
	if api.terminal {
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

func normalizeCouncilVotes(votes []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(votes))
	for _, vote := range votes {
		out = append(out, map[string]any{
			"round":     intNumber(vote["round"]),
			"member_id": mapString(vote["member_id"]),
			"vote":      mapString(vote["vote"]),
			"rationale": mapString(vote["rationale"]),
		})
	}
	return out
}

func councilVoteTally(votes []map[string]any) map[string]any {
	rounds := map[int]map[string]int{}
	finalRound := 0
	for _, vote := range votes {
		round := intNumber(vote["round"])
		if round <= 0 {
			round = 1
		}
		if round > finalRound {
			finalRound = round
		}
		if rounds[round] == nil {
			rounds[round] = map[string]int{}
		}
		rounds[round][mapString(vote["vote"])]++
	}
	byRound := make([]map[string]any, 0, len(rounds))
	for round, counts := range rounds {
		byRound = append(byRound, map[string]any{
			"round":            round,
			"demonstrated":     counts["demonstrated"],
			"not_demonstrated": counts["not_demonstrated"],
		})
	}
	sort.Slice(byRound, func(i, j int) bool {
		return intNumber(byRound[i]["round"]) < intNumber(byRound[j]["round"])
	})
	finalCounts := map[string]int{}
	if finalRound > 0 {
		finalCounts = rounds[finalRound]
	}
	return map[string]any{
		"final_round":      finalRound,
		"demonstrated":     finalCounts["demonstrated"],
		"not_demonstrated": finalCounts["not_demonstrated"],
		"rounds":           byRound,
	}
}

func (api *lawyerAPIServer) consumeAttemptLocked(turn *lawyerTurn, err error, decisionAttempt bool) error {
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
		if failErr := api.rc.failOpportunity(turn.opportunity, opportunityFailureAttemptsExhausted, feedback.Error(), details); failErr != nil {
			feedback = errors.Join(feedback, failErr)
			api.finishTurnLocked(turn, feedback)
		} else {
			api.finishTurnLocked(turn, nil)
		}
	}
	return feedback
}

func (api *lawyerAPIServer) statusResponseLocked(caseID string, roleID string) map[string]any {
	if api.terminal {
		response := api.responseBaseLocked(caseID, roleID)
		caseObj := mapAny(api.rc.state["case"])
		if mapString(caseObj["status"]) == "failed" {
			response["status"] = "failed"
			response["failure"] = caseFailure(api.rc.state)
			response["error"] = caseFailureError(api.rc.state)
		} else {
			response["status"] = "done"
		}
		response["prompt"] = ""
		response["tools"] = []map[string]any{caseStatusHTTPToolSpec()}
		if api.terminalReason != "" {
			response["final_reason"] = api.terminalReason
		}
		return response
	}
	if mapString(mapAny(api.rc.state["case"])["status"]) == "failed" {
		response := api.responseBaseLocked(caseID, roleID)
		response["status"] = "failed"
		response["prompt"] = ""
		response["tools"] = []map[string]any{caseStatusHTTPToolSpec()}
		response["failure"] = caseFailure(api.rc.state)
		response["error"] = caseFailureError(api.rc.state)
		return response
	}
	if roleID == "observer" {
		response := api.responseBaseLocked(caseID, roleID)
		response["status"] = "observing"
		response["prompt"] = "Observe the arbitration record. Observer tools are read-only."
		response["tools"] = observerToolSpecs()
		response["limits"] = observerLimits(api.rc)
		return response
	}
	response := api.responseBaseLocked(caseID, roleID)
	turn := api.active
	if turn == nil || turn.completed || turn.opportunity.Role != roleID {
		response["status"] = "waiting"
		response["prompt"] = ""
		response["tools"] = []map[string]any{caseStatusHTTPToolSpec()}
		return response
	}
	response["status"] = "ready"
	response["prompt"] = turn.prompt
	response["tools"] = lawyerToolSpecs(turn.opportunity)
	response["limits"] = api.lawyerLimitsLocked(turn)
	return response
}

func (api *lawyerAPIServer) waitResponseLocked(caseID string, roleID string, after string, baseline uint64) (map[string]any, string, bool) {
	response := api.statusResponseLocked(caseID, roleID)
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
	specs := []map[string]any{
		caseStatusHTTPToolSpec(),
		httpToolSpec("get_case", "Return the current visible arbitration record.", emptyObjectSchema(), true),
		httpToolSpec("send_work_notes", "Send private work notes for off-record operator analysis. This does not create evidence, a filing, a technical report, or a case event.", workNotesSchema(), false),
	}
	if evidenceReadAllowed(opportunity) {
		specs = append(specs,
			httpToolSpec("list_evidence", "List visible immutable record evidence.", emptyObjectSchema(), true),
			httpToolSpec("stat_evidence", "Return metadata and read limits for one visible evidence item.", evidenceIDSchema(), true),
			httpToolSpec("read_evidence_range", "Read a bounded byte range from one visible evidence item as base64.", readEvidenceRangeSchema(), true),
		)
	}
	if evidenceSubmissionAllowed(opportunity) {
		specs = append(specs,
			httpToolSpec("begin_evidence_upload", "Begin a chunked evidence upload.", beginEvidenceUploadSchema(), false),
			httpToolSpec("write_evidence_chunk", "Write one base64 chunk into an upload session.", writeEvidenceChunkSchema(), false),
			httpToolSpec("commit_evidence_upload", "Verify and admit a completed evidence upload.", commitEvidenceUploadSchema(), false),
			httpToolSpec("submit_evidence", "Submit source evidence with provenance.", submittedEvidenceSchema(), false),
		)
	}
	specs = append(specs, httpToolSpec("submit_decision", "Submit the legal act for the current opportunity.", submitDecisionHTTPSchema(opportunity.AllowedTools), false))
	return specs
}

func observerToolSpecs() []map[string]any {
	return []map[string]any{
		caseStatusHTTPToolSpec(),
		httpToolSpec("get_case", "Return the current arbitration record.", emptyObjectSchema(), true),
		httpToolSpec("get_turn", "Return the current turn role, phase, deadline, and attempts.", emptyObjectSchema(), true),
		httpToolSpec("list_events", "List recorded case events.", listEventsSchema(), true),
		httpToolSpec("list_evidence", "List visible immutable record evidence.", emptyObjectSchema(), true),
		httpToolSpec("stat_evidence", "Return metadata for one visible evidence item.", evidenceIDSchema(), true),
		httpToolSpec("read_evidence_range", "Read a bounded byte range from one visible evidence item as base64.", readEvidenceRangeSchema(), true),
	}
}

func caseStatusHTTPToolSpec() map[string]any {
	return httpToolSpec("case_status", "Return the current case phase, active turn, role status, and case counts.", emptyObjectSchema(), true)
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
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"notes": map[string]any{
				"type":        "string",
				"description": "Accumulated private work notes for this lawyer turn.",
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
