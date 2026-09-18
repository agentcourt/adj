package mcpbridge

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ProtocolVersion               = "2025-06-18"
	DefaultSessionTTL             = 30 * time.Minute
	DefaultSessionCleanupInterval = time.Minute
	defaultServerVersion          = "0.1.0"
	MaxRequestBytes               = 4 << 20
)

type Profile struct {
	CaseID         string
	AssignmentType string
	PrincipalID    string
	Instructions   string
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	ReadOnly    bool
}

type CallResult struct {
	StructuredContent map[string]any
	IsError           bool
}

type Adapter interface {
	OpenSession(Assignment) (Profile, error)
	Tools(Profile) ([]Tool, error)
	CallTool(context.Context, Profile, string, map[string]any) (CallResult, error)
}

type Options struct {
	ListenAddr             string
	Audience               string
	SigningKey             []byte
	SessionTTL             time.Duration
	DisableSessionExpiry   bool
	SessionCleanupInterval time.Duration
	AllowedOrigins         []string
	Log                    io.Writer
	ListenerReady          func(string) error
	ServerName             string
	ServerVersion          string
	Adapter                Adapter
}

func Run(ctx context.Context, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	listenAddr := strings.TrimSpace(opts.ListenAddr)
	if listenAddr == "" {
		return fmt.Errorf("listen address is required")
	}
	serverName := strings.TrimSpace(opts.ServerName)
	if serverName == "" {
		return fmt.Errorf("server name is required")
	}
	serverVersion := strings.TrimSpace(opts.ServerVersion)
	if serverVersion == "" {
		serverVersion = defaultServerVersion
	}
	if opts.Adapter == nil {
		return fmt.Errorf("MCP adapter is required")
	}
	audience := strings.TrimSpace(opts.Audience)
	if err := validateAudience(audience); err != nil {
		return err
	}
	if err := validateSigningKey(opts.SigningKey); err != nil {
		return err
	}
	sessionTTL := opts.SessionTTL
	if opts.DisableSessionExpiry {
		sessionTTL = 0
	} else if sessionTTL == 0 {
		sessionTTL = DefaultSessionTTL
	}
	if sessionTTL < 0 {
		return fmt.Errorf("session TTL must be non-negative")
	}
	cleanupInterval := opts.SessionCleanupInterval
	if cleanupInterval == 0 {
		cleanupInterval = DefaultSessionCleanupInterval
	}
	if sessionTTL > 0 && cleanupInterval <= 0 {
		return fmt.Errorf("session cleanup interval must be positive when session expiry is enabled")
	}
	allowedOrigins, err := makeAllowedOrigins(opts.AllowedOrigins)
	if err != nil {
		return err
	}
	logWriter := opts.Log
	if logWriter == nil {
		logWriter = io.Discard
	}
	server := &server{
		name:           serverName,
		version:        serverVersion,
		audience:       audience,
		signingKey:     append([]byte(nil), opts.SigningKey...),
		allowedOrigins: allowedOrigins,
		adapter:        opts.Adapter,
		log:            logWriter,
		sessionTTL:     sessionTTL,
		sessions:       map[string]*session{},
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen for %s MCP: %w", serverName, err)
	}
	if opts.ListenerReady != nil {
		if err := opts.ListenerReady(DisplayAddress(listener.Addr().String())); err != nil {
			return errors.Join(err, listener.Close())
		}
	}
	cleanupCtx, stopCleanup := context.WithCancel(context.Background())
	defer stopCleanup()
	if sessionTTL > 0 {
		go server.expireSessionsLoop(cleanupCtx, cleanupInterval)
	}
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 10 * time.Second}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- httpServer.Serve(listener)
	}()
	server.logf("%s mcp listening on http://%s/mcp", serverName, DisplayAddress(listener.Addr().String()))
	select {
	case serveErr := <-serveDone:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(serveErr, server.takeRuntimeError())
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := httpServer.Shutdown(shutdownCtx)
		cancel()
		var closeErr error
		if shutdownErr != nil {
			closeErr = httpServer.Close()
		}
		serveErr := <-serveDone
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, closeErr, serveErr, server.takeRuntimeError())
	}
}

func DisplayAddress(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func EmptyObjectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func makeAllowedOrigins(values []string) (map[string]struct{}, error) {
	origins := map[string]struct{}{}
	for _, value := range values {
		origin := strings.TrimSpace(value)
		if origin == "" {
			return nil, fmt.Errorf("allowed origin must not be empty")
		}
		origins[origin] = struct{}{}
	}
	return origins, nil
}

type server struct {
	name           string
	version        string
	audience       string
	signingKey     []byte
	allowedOrigins map[string]struct{}
	adapter        Adapter
	log            io.Writer
	sessionTTL     time.Duration

	mu         sync.Mutex
	sessions   map[string]*session
	errMu      sync.Mutex
	runtimeErr error
}

type session struct {
	assignment Assignment
	profile    Profile
	lastSeen   time.Time
}

type rpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments map[string]any  `json:"arguments"`
	Meta      requestMetadata `json:"_meta,omitempty"`
}

type requestMetadata map[string]json.RawMessage

func (m *requestMetadata) UnmarshalJSON(data []byte) error {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("_meta must be an object: %w", err)
	}
	if value == nil {
		return fmt.Errorf("_meta must be an object")
	}
	*m = value
	return nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w = &responseErrorWriter{ResponseWriter: w, server: s}
	if req.URL.Path == "/health" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.URL.Path != "/mcp" {
		http.NotFound(w, req)
		return
	}
	if req.URL.RawQuery != "" {
		http.Error(w, "MCP endpoint does not accept query parameters", http.StatusBadRequest)
		return
	}
	assignment, ok := s.authorized(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.originAllowed(req.Header.Get("Origin")) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	switch req.Method {
	case http.MethodPost:
		s.handlePost(w, req, assignment)
	case http.MethodDelete:
		s.handleDelete(w, req, assignment)
	case http.MethodGet:
		w.Header().Set("Allow", "POST, GET, DELETE")
		http.Error(w, "server-sent event stream is not supported", http.StatusMethodNotAllowed)
	default:
		w.Header().Set("Allow", "POST, GET, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) authorized(req *http.Request) (Assignment, bool) {
	authorization := strings.TrimSpace(req.Header.Get("Authorization"))
	token, found := strings.CutPrefix(authorization, "Bearer ")
	token = strings.TrimSpace(token)
	if !found || token == "" {
		return Assignment{}, false
	}
	assignment, err := VerifyCapability(s.signingKey, token, s.audience)
	if err != nil {
		return Assignment{}, false
	}
	return assignment, true
}

func (s *server) originAllowed(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true
	}
	if _, ok := s.allowedOrigins[origin]; ok {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch parsed.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func (s *server) handlePost(w http.ResponseWriter, req *http.Request, assignment Assignment) {
	body := http.MaxBytesReader(w, req.Body, MaxRequestBytes)
	var message rpcMessage
	decoder := json.NewDecoder(body)
	decoder.UseNumber()
	if err := decoder.Decode(&message); err != nil {
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		messageText := "request body must contain one JSON value"
		if err != nil {
			messageText = err.Error()
		}
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32600, Message: messageText}})
		return
	}
	if message.JSONRPC != "2.0" || strings.TrimSpace(message.Method) == "" {
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32600, Message: "invalid JSON-RPC request"}})
		return
	}
	var stored *session
	if message.Method != "initialize" {
		var ok bool
		stored, ok = s.requireSession(w, req, message, assignment)
		if !ok {
			return
		}
	}
	if message.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch message.Method {
	case "initialize":
		s.handleInitialize(w, req, message, assignment)
	case "ping":
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Result: map[string]any{}})
	case "tools/list":
		tools, err := s.adapter.Tools(stored.profile)
		if err != nil {
			writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32603, Message: err.Error()}})
			return
		}
		wireTools := make([]map[string]any, 0, len(tools))
		for _, tool := range tools {
			wireTools = append(wireTools, toolSpec(tool))
		}
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Result: map[string]any{"tools": wireTools}})
	case "tools/call":
		result, err := s.callTool(req.Context(), stored.profile, message.Params)
		if err != nil {
			writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
			return
		}
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Result: toolResult(result)})
	default:
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
	}
}

func (s *server) handleInitialize(w http.ResponseWriter, req *http.Request, message rpcMessage, assignment Assignment) {
	if strings.TrimSpace(req.Header.Get("Mcp-Session-Id")) != "" {
		writeRPC(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32600, Message: "Mcp-Session-Id is not allowed during initialize"}})
		return
	}
	profile, err := s.adapter.OpenSession(assignment)
	if err != nil {
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}
	profile.CaseID = strings.TrimSpace(profile.CaseID)
	profile.AssignmentType = strings.TrimSpace(profile.AssignmentType)
	profile.PrincipalID = strings.TrimSpace(profile.PrincipalID)
	profile.Instructions = strings.TrimSpace(profile.Instructions)
	if profile.CaseID != assignment.CaseID || profile.AssignmentType != assignment.AssignmentType || profile.PrincipalID != assignment.PrincipalID || profile.Instructions == "" {
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32603, Message: "adapter returned an incomplete session profile"}})
		return
	}
	sessionID, err := randomSessionID()
	if err != nil {
		writeRPC(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32603, Message: err.Error()}})
		return
	}
	now := time.Now()
	stored := &session{assignment: assignment, profile: profile, lastSeen: now}
	s.mu.Lock()
	s.sessions[sessionID] = stored
	s.mu.Unlock()
	s.logf("mcp_session_created session_id=%s case_id=%s assignment_type=%s principal=%s", sessionID, profile.CaseID, profile.AssignmentType, profile.PrincipalID)
	w.Header().Set("Mcp-Session-Id", sessionID)
	writeRPC(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.name, "version": s.version},
			"instructions":    profile.Instructions,
		},
	})
}

func (s *server) callTool(ctx context.Context, profile Profile, raw json.RawMessage) (CallResult, error) {
	var call toolCallParams
	if len(raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&call); err != nil {
			return CallResult{}, fmt.Errorf("decode tool call parameters: %w", err)
		}
	}
	call.Name = strings.TrimSpace(call.Name)
	if call.Name == "" {
		return CallResult{}, fmt.Errorf("tool name is required")
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	result, err := s.adapter.CallTool(ctx, profile, call.Name, call.Arguments)
	if err != nil {
		return CallResult{}, err
	}
	if result.StructuredContent == nil {
		return CallResult{}, fmt.Errorf("tool %s returned no structured content", call.Name)
	}
	return result, nil
}

func (s *server) requireSession(w http.ResponseWriter, req *http.Request, message rpcMessage, assignment Assignment) (*session, bool) {
	sessionID := strings.TrimSpace(req.Header.Get("Mcp-Session-Id"))
	if sessionID == "" {
		writeRPC(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32600, Message: "Mcp-Session-Id is required after initialize"}})
		return nil, false
	}
	now := time.Now()
	s.mu.Lock()
	stored := s.sessions[sessionID]
	expired := false
	if stored != nil && stored.assignment != assignment {
		s.mu.Unlock()
		http.Error(w, "MCP capability does not match session", http.StatusUnauthorized)
		return nil, false
	}
	if stored != nil {
		if s.sessionTTL > 0 && !stored.lastSeen.Add(s.sessionTTL).After(now) {
			delete(s.sessions, sessionID)
			stored = nil
			expired = true
		} else {
			stored.lastSeen = now
		}
	}
	s.mu.Unlock()
	if stored != nil {
		return stored, true
	}
	if expired {
		s.logf("mcp_session_deleted session_id=%s reason=expired", sessionID)
		http.Error(w, "expired MCP session", http.StatusNotFound)
	} else {
		http.Error(w, "unknown MCP session", http.StatusNotFound)
	}
	return nil, false
}

func (s *server) handleDelete(w http.ResponseWriter, req *http.Request, assignment Assignment) {
	sessionID := strings.TrimSpace(req.Header.Get("Mcp-Session-Id"))
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	stored, existed := s.sessions[sessionID]
	if existed && stored.assignment != assignment {
		s.mu.Unlock()
		http.Error(w, "MCP capability does not match session", http.StatusUnauthorized)
		return
	}
	if existed {
		delete(s.sessions, sessionID)
	}
	s.mu.Unlock()
	if existed {
		s.logf("mcp_session_deleted session_id=%s reason=delete", sessionID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) expireSessionsLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.expireIdleSessions(now)
		}
	}
}

func (s *server) expireIdleSessions(now time.Time) int {
	if s.sessionTTL <= 0 {
		return 0
	}
	var expired []string
	s.mu.Lock()
	for id, stored := range s.sessions {
		if !stored.lastSeen.Add(s.sessionTTL).After(now) {
			delete(s.sessions, id)
			expired = append(expired, id)
		}
	}
	s.mu.Unlock()
	sort.Strings(expired)
	for _, id := range expired {
		s.logf("mcp_session_deleted session_id=%s reason=expired", id)
	}
	return len(expired)
}

func toolSpec(tool Tool) map[string]any {
	spec := map[string]any{
		"name":        tool.Name,
		"description": tool.Description,
		"inputSchema": tool.InputSchema,
	}
	if tool.ReadOnly {
		spec["annotations"] = map[string]any{"readOnlyHint": true}
	}
	return spec
}

func toolResult(result CallResult) map[string]any {
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": toolResultText(result.StructuredContent)}},
		"structuredContent": result.StructuredContent,
		"isError":           result.IsError,
	}
}

func toolResultText(value map[string]any) string {
	var summary strings.Builder
	for _, key := range []string{"ok", "status", "state", "message", "after_version", "after_opportunity_id", "role_id", "member_id", "case_id"} {
		if text := NumberString(value[key]); text != "" {
			summary.WriteString(key)
			summary.WriteString(": ")
			summary.WriteString(text)
			summary.WriteByte('\n')
		}
	}
	if wait, ok := value["wait"].(map[string]any); ok {
		if reason := String(wait["reason"]); reason != "" {
			summary.WriteString("wait_reason: ")
			summary.WriteString(reason)
			summary.WriteByte('\n')
		}
	}
	if turn, ok := value["turn"].(map[string]any); ok {
		for _, key := range []string{"phase", "opportunity_id", "remaining_ms", "attempts_remaining"} {
			if text := NumberString(turn[key]); text != "" {
				summary.WriteString(key)
				summary.WriteString(": ")
				summary.WriteString(text)
				summary.WriteByte('\n')
			}
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		if summary.Len() == 0 {
			return "json_error: " + err.Error()
		}
		summary.WriteString("json_error: ")
		summary.WriteString(err.Error())
		return strings.TrimSpace(summary.String())
	}
	if summary.Len() == 0 {
		return string(raw)
	}
	summary.WriteString("json: ")
	summary.Write(raw)
	return strings.TrimSpace(summary.String())
}

func String(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprintf("%v", value)
}

func NumberString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", typed)
	default:
		return ""
	}
}

func writeRPC(w http.ResponseWriter, status int, response rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		recorder, ok := w.(interface{ recordResponseError(error) })
		if !ok {
			panic(err)
		}
		recorder.recordResponseError(fmt.Errorf("encode MCP response: %w", err))
	}
}

func randomSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate MCP session ID: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *server) logf(format string, args ...any) {
	if _, err := fmt.Fprintf(s.log, format+"\n", args...); err != nil {
		s.recordRuntimeError(fmt.Errorf("write MCP log: %w", err))
	}
}

type responseErrorWriter struct {
	http.ResponseWriter
	server *server
}

func (w *responseErrorWriter) recordResponseError(err error) {
	w.server.recordRuntimeError(err)
}

func (s *server) recordRuntimeError(err error) {
	if err == nil {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	s.runtimeErr = errors.Join(s.runtimeErr, err)
}

func (s *server) takeRuntimeError() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	err := s.runtimeErr
	s.runtimeErr = nil
	return err
}
