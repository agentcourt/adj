package quick

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/agentcourt/adj/common/documents"
)

const (
	lawyerAPIBasePath = "/lawyerapi/v1"
	maxWaitTimeout    = 5 * time.Minute
	defaultWait       = 30 * time.Second
	maxReadBytes      = 256 << 10
)

type lawyerTurn struct {
	role              string
	opportunityID     string
	prompt            string
	deadline          time.Time
	attemptsMax       int
	attemptsRemaining int
	completed         bool
	done              chan error
}

type doRequest struct {
	CaseID        string         `json:"case_id"`
	RoleID        string         `json:"role_id"`
	OpportunityID string         `json:"opportunity_id"`
	Tool          string         `json:"tool"`
	Arguments     map[string]any `json:"arguments"`
	CallID        string         `json:"call_id,omitempty"`
}

type failRequest struct {
	CaseID        string         `json:"case_id"`
	RoleID        string         `json:"role_id"`
	OpportunityID string         `json:"opportunity_id"`
	Reason        string         `json:"reason,omitempty"`
	Message       string         `json:"message,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

type caseAPI struct {
	runner    *runner
	server    *http.Server
	listener  net.Listener
	baseURL   string
	serveDone chan error
	closeOnce sync.Once
	closeErr  error

	errMu      sync.Mutex
	runtimeErr error
}

func startCaseAPI(runner *runner) (*caseAPI, error) {
	listener, err := net.Listen("tcp", runner.cfg.CaseAPIAddr)
	if err != nil {
		return nil, fmt.Errorf("start quick case API: %w", err)
	}
	api := &caseAPI{
		runner:    runner,
		listener:  listener,
		baseURL:   "http://" + displayAddress(listener.Addr()),
		serveDone: make(chan error, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.handleHealth)
	mux.HandleFunc(lawyerAPIBasePath+"/get", api.handleGet)
	mux.HandleFunc(lawyerAPIBasePath+"/wait", api.handleWait)
	mux.HandleFunc(lawyerAPIBasePath+"/status", api.handleStatus)
	mux.HandleFunc(lawyerAPIBasePath+"/result", api.handleResult)
	mux.HandleFunc(lawyerAPIBasePath+"/do", api.handleDo)
	mux.HandleFunc(lawyerAPIBasePath+"/fail", api.handleFail)
	api.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if (req.URL.Path == lawyerAPIBasePath || strings.HasPrefix(req.URL.Path, lawyerAPIBasePath+"/")) && !api.authorized(req) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, errorResponse("unauthorized", "valid bearer token required"))
			return
		}
		mux.ServeHTTP(&responseErrorWriter{ResponseWriter: w, api: api}, req)
	})}
	go func() {
		err := api.server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("serve quick case API: %w", err)
		}
		api.serveDone <- err
	}()
	return api, nil
}

func (api *caseAPI) authorized(req *http.Request) bool {
	values := req.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
		return false
	}
	candidate := strings.TrimPrefix(values[0], "Bearer ")
	if candidate == "" || strings.TrimSpace(candidate) != candidate {
		return false
	}
	expected := api.runner.cfg.LawyerAPIBearerToken
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1
}

func displayAddress(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func (api *caseAPI) Close(ctx context.Context) error {
	api.closeOnce.Do(func() {
		shutdownErr := api.server.Shutdown(ctx)
		var forceCloseErr error
		if shutdownErr != nil {
			forceCloseErr = api.server.Close()
		}
		serveErr := <-api.serveDone
		api.errMu.Lock()
		runtimeErr := api.runtimeErr
		api.runtimeErr = nil
		api.errMu.Unlock()
		api.closeErr = errors.Join(shutdownErr, forceCloseErr, serveErr, runtimeErr)
	})
	return api.closeErr
}

func (api *caseAPI) recordRuntimeError(err error) {
	if err == nil {
		return
	}
	api.errMu.Lock()
	defer api.errMu.Unlock()
	api.runtimeErr = errors.Join(api.runtimeErr, err)
}

type responseErrorWriter struct {
	http.ResponseWriter
	api *caseAPI
}

func (w *responseErrorWriter) recordResponseError(err error) {
	w.api.recordRuntimeError(fmt.Errorf("write quick case API response: %w", err))
}

func (api *caseAPI) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method_not_allowed", "use GET"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"case_id": api.runner.cfg.CaseID,
		"run_id":  api.runner.cfg.RunID,
	})
}

func (api *caseAPI) handleGet(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method_not_allowed", "use GET"))
		return
	}
	caseID, role, ok := api.queryIdentity(w, req)
	if !ok {
		return
	}
	api.runner.mu.Lock()
	response := api.statusResponseLocked(caseID, role)
	api.runner.mu.Unlock()
	writeJSON(w, http.StatusOK, response)
}

func (api *caseAPI) handleStatus(w http.ResponseWriter, req *http.Request) {
	api.handleGet(w, req)
}

func (api *caseAPI) handleResult(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method_not_allowed", "use GET"))
		return
	}
	caseID, role, ok := api.queryIdentity(w, req)
	if !ok {
		return
	}
	api.runner.mu.Lock()
	response := api.responseBaseLocked(caseID, role)
	response["ok"] = true
	response["status"] = "pending"
	if api.runner.terminal {
		if api.runner.terminalErr != nil {
			response["status"] = "failed"
			response["error"] = apiError("case_failed", api.runner.terminalErr.Error())
		} else {
			response["status"] = "done"
		}
	}
	forVotes, againstVotes := countVotes(api.runner.transcript.Votes)
	response["result"] = map[string]any{
		"phase":             api.runner.phase,
		"proposition":       api.runner.cfg.Proposition,
		"evidence_standard": api.runner.cfg.EvidenceStandard,
		"resolution":        resolutionFor(forVotes, againstVotes, api.runner.cfg.RequiredVotes, councilComplete(api.runner.transcript, api.runner.cfg.CouncilSize)),
		"votes_for":         forVotes,
		"votes_against":     againstVotes,
		"required_votes":    api.runner.cfg.RequiredVotes,
		"arguments":         append([]Argument(nil), api.runner.transcript.Arguments...),
		"votes":             append([]Vote(nil), api.runner.transcript.Votes...),
		"council_failures":  append([]CouncilMemberFailure(nil), api.runner.transcript.CouncilFailures...),
	}
	api.runner.mu.Unlock()
	writeJSON(w, http.StatusOK, response)
}

func (api *caseAPI) handleWait(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method_not_allowed", "use GET"))
		return
	}
	caseID, role, ok := api.queryIdentity(w, req)
	if !ok {
		return
	}
	afterVersion, hasAfterVersion, err := optionalUint(req.URL.Query().Get("after_version"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_after_version", err.Error()))
		return
	}
	after := strings.TrimSpace(req.URL.Query().Get("after"))
	timeout, err := waitTimeout(req.URL.Query().Get("timeout_ms"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_timeout", err.Error()))
		return
	}
	runner := api.runner
	runner.mu.Lock()
	baseline := runner.version
	if hasAfterVersion {
		baseline = afterVersion
	}
	timedOut := false
	timer := time.AfterFunc(timeout, func() {
		runner.mu.Lock()
		timedOut = true
		runner.cond.Broadcast()
		runner.mu.Unlock()
	})
	defer timer.Stop()
	if done := req.Context().Done(); done != nil {
		go func() {
			<-done
			runner.mu.Lock()
			runner.cond.Broadcast()
			runner.mu.Unlock()
		}()
	}
	for {
		if req.Context().Err() != nil {
			runner.mu.Unlock()
			return
		}
		response := api.statusResponseLocked(caseID, role)
		reason := ""
		ready := false
		if runner.terminal {
			ready = true
			if runner.terminalErr != nil {
				reason = "failed"
			} else {
				reason = "done"
			}
		} else if runner.active != nil && !runner.active.completed && runner.active.role == role && (after == "" || after != runner.active.opportunityID) {
			ready, reason = true, "ready"
		} else if runner.version != baseline {
			ready, reason = true, "changed"
		} else if timedOut {
			ready, reason = true, "timeout"
		}
		if ready {
			response["wait"] = map[string]any{"reason": reason, "version": runner.version, "state_version": runner.version}
			runner.mu.Unlock()
			writeJSON(w, http.StatusOK, response)
			return
		}
		runner.cond.Wait()
	}
}

func (api *caseAPI) handleDo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method_not_allowed", "use POST"))
		return
	}
	var value doRequest
	if err := decodeRequest(w, req, api.runner.cfg.MaxResponseBytes, &value); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_json", err.Error()))
		return
	}
	value.CaseID = strings.TrimSpace(value.CaseID)
	value.RoleID = strings.TrimSpace(value.RoleID)
	value.OpportunityID = strings.TrimSpace(value.OpportunityID)
	value.Tool = strings.TrimSpace(value.Tool)
	if value.Arguments == nil {
		value.Arguments = map[string]any{}
	}
	if !api.validateIdentity(w, value.CaseID, value.RoleID) {
		return
	}
	result, err := api.executeTool(value)
	api.runner.mu.Lock()
	response := api.responseBaseLocked(value.CaseID, value.RoleID)
	api.runner.mu.Unlock()
	if err != nil {
		response["ok"] = false
		response["error"] = apiError("tool_failed", err.Error())
	} else {
		response["ok"] = true
		response["result"] = result
	}
	writeJSON(w, http.StatusOK, response)
}

func (api *caseAPI) handleFail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method_not_allowed", "use POST"))
		return
	}
	var value failRequest
	if err := decodeRequest(w, req, api.runner.cfg.MaxResponseBytes, &value); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_json", err.Error()))
		return
	}
	value.CaseID = strings.TrimSpace(value.CaseID)
	value.RoleID = strings.TrimSpace(value.RoleID)
	value.OpportunityID = strings.TrimSpace(value.OpportunityID)
	value.Reason = strings.TrimSpace(value.Reason)
	value.Message = strings.TrimSpace(value.Message)
	if !api.validateIdentity(w, value.CaseID, value.RoleID) {
		return
	}
	runner := api.runner
	runner.mu.Lock()
	turn := runner.active
	accepted := false
	var responseErr map[string]any
	if turn == nil || turn.completed {
		responseErr = apiError("no_active_turn", "no lawyer turn is active")
	} else if turn.role != value.RoleID {
		responseErr = apiError("not_current_turn", fmt.Sprintf("current lawyer turn belongs to %s", turn.role))
	} else if value.OpportunityID == "" || value.OpportunityID != turn.opportunityID {
		responseErr = apiError("stale_opportunity", "opportunity_id does not match the active opportunity")
	} else if expiryErr := lawyerTurnExpiryError(turn, runner.cfg.LawyerTimeout); expiryErr != nil {
		runner.completeLawyerTurnLocked(turn, expiryErr)
		responseErr = apiError("turn_expired", expiryErr.Error())
	} else {
		message := value.Message
		if message == "" {
			message = value.Reason
		}
		if message == "" {
			message = "lawyer agent failed"
		}
		runner.completeLawyerTurnLocked(turn, fmt.Errorf("%s lawyer: %s", value.RoleID, message))
		accepted = true
	}
	response := api.responseBaseLocked(value.CaseID, value.RoleID)
	response["ok"] = accepted
	if accepted {
		response["result"] = map[string]any{"accepted": true}
	} else {
		response["error"] = responseErr
	}
	runner.mu.Unlock()
	writeJSON(w, http.StatusOK, response)
}

func (api *caseAPI) queryIdentity(w http.ResponseWriter, req *http.Request) (string, string, bool) {
	caseID := strings.TrimSpace(req.URL.Query().Get("case_id"))
	role := strings.TrimSpace(req.URL.Query().Get("role_id"))
	return caseID, role, api.validateIdentity(w, caseID, role)
}

func (api *caseAPI) validateIdentity(w http.ResponseWriter, caseID, role string) bool {
	if caseID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("missing_case_id", "case_id is required"))
		return false
	}
	if caseID != api.runner.cfg.CaseID {
		response := errorResponse("unknown_case", "case_id does not match this case process")
		response["case_id"] = caseID
		writeJSON(w, http.StatusNotFound, response)
		return false
	}
	if role == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("missing_role_id", "role_id is required"))
		return false
	}
	if role != "plaintiff" && role != "defendant" && role != "observer" {
		writeJSON(w, http.StatusForbidden, errorResponse("invalid_role", "role_id must be plaintiff, defendant, or observer"))
		return false
	}
	return true
}

func (api *caseAPI) statusResponseLocked(caseID, role string) map[string]any {
	runner := api.runner
	response := api.responseBaseLocked(caseID, role)
	response["ok"] = true
	if runner.terminal {
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		if runner.terminalErr != nil {
			response["status"] = "failed"
			response["error"] = apiError("case_failed", runner.terminalErr.Error())
		} else {
			response["status"] = "done"
		}
		return response
	}
	if role == "observer" {
		response["status"] = "observing"
		response["prompt"] = runner.observerPrompt()
		response["tools"] = observerToolSpecs(runner)
		return response
	}
	turn := runner.active
	if turn == nil || turn.completed || turn.role != role {
		response["status"] = "waiting"
		response["prompt"] = ""
		response["tools"] = []map[string]any{caseStatusToolSpec(runner)}
		return response
	}
	response["status"] = "ready"
	response["prompt"] = turn.prompt
	response["tools"] = lawyerToolSpecs(runner)
	response["limits"] = map[string]any{
		"max_response_bytes":      runner.cfg.MaxResponseBytes,
		"max_argument_chars":      runner.cfg.MaxArgumentChars,
		"attempts_max":            turn.attemptsMax,
		"attempts_remaining":      turn.attemptsRemaining,
		"max_evidence_read_bytes": maxReadBytes,
	}
	return response
}

func (api *caseAPI) responseBaseLocked(caseID, role string) map[string]any {
	return map[string]any{
		"case_id":       caseID,
		"role_id":       role,
		"state_version": api.runner.version,
		"turn":          api.turnPayloadLocked(),
	}
}

func (api *caseAPI) turnPayloadLocked() map[string]any {
	turn := api.runner.active
	if turn == nil {
		return nil
	}
	remaining := time.Until(turn.deadline).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"role_id":            turn.role,
		"phase":              "arguments",
		"opportunity_id":     turn.opportunityID,
		"turn_number":        len(api.runner.transcript.Arguments) + 1,
		"deadline":           turn.deadline.UTC().Format(time.RFC3339Nano),
		"remaining_ms":       remaining,
		"attempts_max":       turn.attemptsMax,
		"attempts_remaining": turn.attemptsRemaining,
		"completed":          turn.completed,
	}
}

func (api *caseAPI) executeTool(req doRequest) (map[string]any, error) {
	switch req.Tool {
	case "case_status":
		api.runner.mu.Lock()
		defer api.runner.mu.Unlock()
		return api.caseViewLocked(), nil
	case "get_case":
		api.runner.mu.Lock()
		defer api.runner.mu.Unlock()
		return map[string]any{"case": api.caseViewLocked()}, nil
	case "get_case_result":
		api.runner.mu.Lock()
		defer api.runner.mu.Unlock()
		forVotes, againstVotes := countVotes(api.runner.transcript.Votes)
		return map[string]any{
			"status":            api.runner.phase,
			"evidence_standard": api.runner.cfg.EvidenceStandard,
			"resolution":        resolutionFor(forVotes, againstVotes, api.runner.cfg.RequiredVotes, councilComplete(api.runner.transcript, api.runner.cfg.CouncilSize)),
			"votes_for":         forVotes,
			"votes_against":     againstVotes,
			"required_votes":    api.runner.cfg.RequiredVotes,
			"council_failures":  append([]CouncilMemberFailure(nil), api.runner.transcript.CouncilFailures...),
		}, nil
	case "send_work_notes":
		if req.RoleID == "observer" {
			return nil, fmt.Errorf("observer work notes are unavailable")
		}
		if len(req.Arguments) != 1 {
			return nil, fmt.Errorf("send_work_notes accepts only arguments.notes")
		}
		notes, ok := req.Arguments["notes"].(string)
		if !ok || strings.TrimSpace(notes) == "" {
			return nil, fmt.Errorf("arguments.notes must be a non-empty string")
		}
		if err := api.runner.records.appendWorkNote(map[string]any{
			"timestamp":      time.Now().UTC(),
			"role_id":        req.RoleID,
			"opportunity_id": req.OpportunityID,
			"call_id":        strings.TrimSpace(req.CallID),
			"notes":          notes,
		}); err != nil {
			return nil, err
		}
		return map[string]any{"accepted": true, "byte_count": len([]byte(notes))}, nil
	case "list_evidence":
		evidence := make([]map[string]any, 0, len(api.runner.documents.Files))
		for _, file := range api.runner.documents.Files {
			evidence = append(evidence, evidenceMetadata(file))
		}
		return map[string]any{"evidence": evidence}, nil
	case "stat_evidence":
		file, err := api.document(strings.TrimSpace(stringValue(req.Arguments["evidence_id"])))
		if err != nil {
			return nil, err
		}
		return map[string]any{"evidence": evidenceMetadata(file), "limits": map[string]any{"max_read_bytes": maxReadBytes}}, nil
	case "read_evidence_range":
		return api.readDocument(req.Arguments)
	case "submit_decision":
		return api.submitArgument(req)
	default:
		return nil, fmt.Errorf("tool %q is unavailable in quick adjudication", req.Tool)
	}
}

func (api *caseAPI) caseViewLocked() map[string]any {
	return map[string]any{
		"case_id":           api.runner.cfg.CaseID,
		"phase":             api.runner.phase,
		"proposition":       api.runner.cfg.Proposition,
		"arguments":         append([]Argument(nil), api.runner.transcript.Arguments...),
		"documents":         api.runner.documents,
		"council_size":      api.runner.cfg.CouncilSize,
		"required_votes":    api.runner.cfg.RequiredVotes,
		"evidence_standard": api.runner.cfg.EvidenceStandard,
		"council_failures":  append([]CouncilMemberFailure(nil), api.runner.transcript.CouncilFailures...),
	}
}

func (api *caseAPI) document(id string) (documents.File, error) {
	for _, file := range api.runner.documents.Files {
		if file.Path == id {
			return file, nil
		}
	}
	return documents.File{}, fmt.Errorf("unknown evidence_id %q", id)
}

func evidenceMetadata(file documents.File) map[string]any {
	return map[string]any{
		"evidence_id":   file.Path,
		"name":          file.Path,
		"size_bytes":    file.SizeBytes,
		"sha256":        file.SHA256,
		"mime_type":     file.MediaType,
		"text_readable": strings.HasPrefix(file.MediaType, "text/") || file.MediaType == "application/json",
	}
}

func (api *caseAPI) readDocument(args map[string]any) (map[string]any, error) {
	id := strings.TrimSpace(stringValue(args["evidence_id"]))
	file, err := api.document(id)
	if err != nil {
		return nil, err
	}
	offset, err := integerValue(args["offset"], "offset")
	if err != nil {
		return nil, err
	}
	length, err := integerValue(args["length"], "length")
	if err != nil {
		return nil, err
	}
	if offset < 0 || length < 1 || length > maxReadBytes {
		return nil, fmt.Errorf("offset must be non-negative and length must be between 1 and %d", maxReadBytes)
	}
	if int64(offset) > file.SizeBytes {
		return nil, fmt.Errorf("offset %d exceeds evidence size %d", offset, file.SizeBytes)
	}
	root := filepath.Join(api.runner.cfg.OutputDir, "documents")
	raw, err := documents.ReadVerified(root, file)
	if err != nil {
		return nil, err
	}
	end := offset + length
	if end < offset || end > len(raw) {
		end = len(raw)
	}
	data := raw[offset:end]
	return map[string]any{
		"evidence_id":    id,
		"offset":         offset,
		"length":         len(data),
		"content_base64": base64.StdEncoding.EncodeToString(data),
		"eof":            end == len(raw),
	}, nil
}

func (api *caseAPI) submitArgument(req doRequest) (map[string]any, error) {
	runner := api.runner
	runner.mu.Lock()
	defer runner.mu.Unlock()
	turn := runner.active
	if turn == nil || turn.completed {
		return nil, fmt.Errorf("no lawyer turn is active")
	}
	if turn.role != req.RoleID {
		return nil, fmt.Errorf("current lawyer turn belongs to %s", turn.role)
	}
	if req.OpportunityID == "" || req.OpportunityID != turn.opportunityID {
		return nil, fmt.Errorf("opportunity_id does not match the active opportunity")
	}
	if expiryErr := lawyerTurnExpiryError(turn, runner.cfg.LawyerTimeout); expiryErr != nil {
		runner.completeLawyerTurnLocked(turn, expiryErr)
		return nil, expiryErr
	}
	kind := strings.TrimSpace(stringValue(req.Arguments["kind"]))
	toolName := strings.TrimSpace(stringValue(req.Arguments["tool_name"]))
	payload, _ := req.Arguments["payload"].(map[string]any)
	text := strings.TrimSpace(stringValue(payload["text"]))
	validationErr := error(nil)
	if len(req.Arguments) != 3 {
		validationErr = fmt.Errorf("submit_decision accepts only kind, tool_name, and payload")
	} else if kind != "tool" || toolName != "submit_argument" {
		validationErr = fmt.Errorf("submit_decision requires kind=tool and tool_name=submit_argument")
	} else if len(payload) != 1 {
		validationErr = fmt.Errorf("submit_argument payload accepts only text")
	} else if text == "" {
		validationErr = fmt.Errorf("submit_argument payload.text must be non-empty")
	} else if utf8.RuneCountInString(text) > runner.cfg.MaxArgumentChars {
		validationErr = fmt.Errorf("argument has %d characters; limit is %d", utf8.RuneCountInString(text), runner.cfg.MaxArgumentChars)
	}
	if validationErr != nil {
		turn.attemptsRemaining--
		if turn.attemptsRemaining == 0 {
			runner.completeLawyerTurnLocked(turn, fmt.Errorf("%s lawyer exhausted invalid attempts: %w", turn.role, validationErr))
		}
		return nil, validationErr
	}
	argument := Argument{
		Role:          turn.role,
		Text:          text,
		SubmittedAt:   time.Now().UTC(),
		OpportunityID: turn.opportunityID,
	}
	runner.transcript.Arguments = append(runner.transcript.Arguments, argument)
	if err := runner.records.writeTranscript(cloneTranscript(runner.transcript)); err != nil {
		runner.transcript.Arguments = runner.transcript.Arguments[:len(runner.transcript.Arguments)-1]
		runner.completeLawyerTurnLocked(turn, err)
		return nil, err
	}
	if err := runner.appendEventLocked("argument_submitted", argument.Role, map[string]any{
		"opportunity_id": argument.OpportunityID,
		"text":           argument.Text,
	}); err != nil {
		runner.completeLawyerTurnLocked(turn, err)
		return nil, err
	}
	runner.completeLawyerTurnLocked(turn, nil)
	return map[string]any{"accepted": true, "role_id": argument.Role, "opportunity_id": argument.OpportunityID}, nil
}

func (r *runner) runLawyerTurn(ctx context.Context, role string) error {
	r.mu.Lock()
	if r.active != nil && !r.active.completed {
		r.mu.Unlock()
		return fmt.Errorf("a lawyer turn is already active")
	}
	prompt, err := r.lawyerPromptLocked(role)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	turn := &lawyerTurn{
		role:              role,
		opportunityID:     "arguments:" + role,
		prompt:            prompt,
		attemptsMax:       r.cfg.InvalidAttemptLimit,
		attemptsRemaining: r.cfg.InvalidAttemptLimit,
		done:              make(chan error, 1),
	}
	if err := ctx.Err(); err != nil {
		r.mu.Unlock()
		return err
	}
	if err := r.appendEventLocked("lawyer_turn_started", role, map[string]any{"opportunity_id": turn.opportunityID}); err != nil {
		r.mu.Unlock()
		return err
	}
	turn.deadline = time.Now().Add(r.cfg.LawyerTimeout)
	r.phase = "arguments"
	r.active = turn
	r.version++
	r.cond.Broadcast()
	r.mu.Unlock()
	timer := time.AfterFunc(time.Until(turn.deadline), func() {
		r.completeLawyerTurn(turn, lawyerTurnTimeoutError(turn, r.cfg.LawyerTimeout))
	})
	stopCancellation := context.AfterFunc(ctx, func() {
		r.completeLawyerTurn(turn, ctx.Err())
	})
	turnErr := <-turn.done
	timer.Stop()
	stopCancellation()
	r.mu.Lock()
	if r.active == turn {
		r.active = nil
		r.version++
		r.cond.Broadcast()
	}
	r.mu.Unlock()
	return turnErr
}

func (r *runner) completeLawyerTurn(turn *lawyerTurn, err error) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completeLawyerTurnLocked(turn, err)
}

func (r *runner) completeLawyerTurnLocked(turn *lawyerTurn, err error) bool {
	if turn == nil || r.active != turn || turn.completed {
		return false
	}
	turn.completed = true
	r.version++
	r.cond.Broadcast()
	turn.done <- err
	return true
}

func lawyerTurnExpiryError(turn *lawyerTurn, timeout time.Duration) error {
	if turn == nil || turn.deadline.IsZero() || time.Now().Before(turn.deadline) {
		return nil
	}
	return lawyerTurnTimeoutError(turn, timeout)
}

func lawyerTurnTimeoutError(turn *lawyerTurn, timeout time.Duration) error {
	return fmt.Errorf("%s lawyer turn timed out after %s", turn.role, timeout)
}

func lawyerToolSpecs(runner *runner) []map[string]any {
	return append(lawyerSupportToolSpecs(runner), map[string]any{
		"name":        "submit_decision",
		"description": runner.toolPrompt("submit_argument"),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":      map[string]any{"type": "string", "enum": []string{"tool"}},
				"tool_name": map[string]any{"type": "string", "enum": []string{"submit_argument"}},
				"payload": map[string]any{
					"type":                 "object",
					"properties":           map[string]any{"text": map[string]any{"type": "string"}},
					"required":             []string{"text"},
					"additionalProperties": false,
				},
			},
			"required":             []string{"kind", "tool_name", "payload"},
			"additionalProperties": false,
		},
		"read_only": false,
	})
}

func lawyerSupportToolSpecs(runner *runner) []map[string]any {
	return append(observerToolSpecs(runner),
		httpToolSpec("send_work_notes", runner.toolPrompt("send_work_notes"), map[string]any{"type": "object", "properties": map[string]any{"notes": map[string]any{"type": "string"}}, "required": []string{"notes"}, "additionalProperties": false}, false),
	)
}

func observerToolSpecs(runner *runner) []map[string]any {
	return []map[string]any{
		caseStatusToolSpec(runner),
		httpToolSpec("get_case", runner.toolPrompt("get_case"), emptySchema(), true),
		httpToolSpec("get_case_result", runner.toolPrompt("get_case_result"), emptySchema(), true),
		httpToolSpec("list_evidence", runner.toolPrompt("list_evidence"), emptySchema(), true),
		httpToolSpec("stat_evidence", runner.toolPrompt("stat_evidence"), evidenceIDSchema(), true),
		httpToolSpec("read_evidence_range", runner.toolPrompt("read_evidence_range"), evidenceRangeSchema(), true),
	}
}

func caseStatusToolSpec(runner *runner) map[string]any {
	return httpToolSpec("case_status", runner.toolPrompt("case_status"), emptySchema(), true)
}

func httpToolSpec(name, description string, schema map[string]any, readOnly bool) map[string]any {
	return map[string]any{"name": name, "description": description, "input_schema": schema, "read_only": readOnly}
}

func emptySchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}

func evidenceIDSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"evidence_id": map[string]any{"type": "string"}}, "required": []string{"evidence_id"}, "additionalProperties": false}
}

func evidenceRangeSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"evidence_id": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer", "minimum": 0}, "length": map[string]any{"type": "integer", "minimum": 1, "maximum": maxReadBytes}}, "required": []string{"evidence_id", "offset", "length"}, "additionalProperties": false}
}

func decodeRequest(w http.ResponseWriter, req *http.Request, maxBytes int, target any) error {
	body := http.MaxBytesReader(w, req.Body, int64(maxBytes))
	decoder := json.NewDecoder(body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("request body is required")
		}
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON value")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		if recorder, ok := w.(interface{ recordResponseError(error) }); ok {
			recorder.recordResponseError(err)
			return
		}
		panic(err)
	}
}

func errorResponse(code, message string) map[string]any {
	return map[string]any{"ok": false, "error": apiError(code, message)}
}

func apiError(code, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

func waitTimeout(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultWait, nil
	}
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds <= 0 {
		return 0, fmt.Errorf("timeout_ms must be a positive integer")
	}
	timeout := time.Duration(milliseconds) * time.Millisecond
	if timeout > maxWaitTimeout {
		return 0, fmt.Errorf("timeout_ms must be at most %d", maxWaitTimeout.Milliseconds())
	}
	return timeout, nil
}

func optionalUint(value string) (uint64, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("after_version must be an unsigned integer")
	}
	return parsed, true, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func integerValue(value any, name string) (int, error) {
	switch number := value.(type) {
	case int:
		return number, nil
	case int64:
		return int(number), nil
	case float64:
		if number != float64(int(number)) {
			return 0, fmt.Errorf("%s must be an integer", name)
		}
		return int(number), nil
	case json.Number:
		parsed, err := strconv.Atoi(number.String())
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", name)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%s must be an integer", name)
	}
}
