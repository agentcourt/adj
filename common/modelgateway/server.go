package modelgateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/modelapi"
	"github.com/agentcourt/adj/common/modelrequest"
)

const maxChatRequestBytes = 32 * 1024 * 1024

type ResponseExecutor interface {
	CheckEndpoint(string) error
	CreateResponseWithRequestSpec(context.Context, modelrequest.Spec, []map[string]any, []map[string]any, string) (modelapi.Response, error)
}

type ServerOptions struct {
	ListenAddress string
	RecordPath    string
}

type Binding struct {
	Token string
	Model string
}

type Server struct {
	executor ResponseExecutor

	mu       sync.RWMutex
	bindings map[string]*chatBinding
	listener net.Listener
	http     *http.Server
	serveErr chan error
	address  string

	recordMu sync.Mutex
	record   *os.File

	handlerErrMu sync.Mutex
	handlerErr   error
}

type chatBinding struct {
	name  string
	alias string
	spec  modelrequest.Spec

	mu                 sync.Mutex
	previousResponseID string
	inputItems         []map[string]any
	expectedMessages   []map[string]any
}

type chatRequest struct {
	Model      string           `json:"model"`
	Messages   []map[string]any `json:"messages"`
	Tools      []map[string]any `json:"tools"`
	ToolChoice any              `json:"tool_choice"`
	Stream     bool             `json:"stream"`
}

type requestRecord struct {
	Name          string          `json:"name"`
	Endpoint      string          `json:"endpoint"`
	Model         string          `json:"model"`
	ReturnedModel string          `json:"returned_model,omitempty"`
	ResponseID    string          `json:"response_id,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   time.Time       `json:"completed_at"`
	Usage         *modelapi.Usage `json:"usage,omitempty"`
	ProviderError string          `json:"provider_error,omitempty"`
	ProviderClass string          `json:"provider_error_class,omitempty"`
}

func NewServer(executor ResponseExecutor) (*Server, error) {
	if executor == nil {
		return nil, fmt.Errorf("model executor is required")
	}
	return &Server{executor: executor, bindings: map[string]*chatBinding{}}, nil
}

func (s *Server) Start(options ServerOptions) error {
	if s == nil {
		return fmt.Errorf("model server is nil")
	}
	listenAddress := strings.TrimSpace(options.ListenAddress)
	if listenAddress == "" {
		listenAddress = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return fmt.Errorf("parse model server listen address: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("model server must listen on a loopback address")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return fmt.Errorf("model server is already running")
	}
	if recordPath := strings.TrimSpace(options.RecordPath); recordPath != "" {
		if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
			return fmt.Errorf("create model request record directory: %w", err)
		}
		record, err := os.OpenFile(recordPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open model request record: %w", err)
		}
		s.record = record
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		closeErr := s.closeRecord()
		return errors.Join(fmt.Errorf("listen for model requests: %w", err), closeErr)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	s.listener = listener
	s.address = listener.Addr().String()
	s.serveErr = make(chan error, 1)
	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		err := s.http.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		s.serveErr <- err
	}()
	return nil
}

func (s *Server) Bind(name string, spec modelrequest.Spec) (Binding, error) {
	if s == nil {
		return Binding{}, fmt.Errorf("model server is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Binding{}, fmt.Errorf("model binding name is required")
	}
	if err := s.executor.CheckEndpoint(spec.Endpoint); err != nil {
		return Binding{}, err
	}
	token, err := randomIdentifier(32)
	if err != nil {
		return Binding{}, fmt.Errorf("create model binding token: %w", err)
	}
	aliasPart, err := randomIdentifier(12)
	if err != nil {
		return Binding{}, fmt.Errorf("create model alias: %w", err)
	}
	alias := "adj-" + aliasPart
	s.mu.Lock()
	s.bindings[token] = &chatBinding{name: name, alias: alias, spec: spec}
	s.mu.Unlock()
	return Binding{Token: token, Model: alias}, nil
}

func (s *Server) Unbind(token string) error {
	if s == nil {
		return fmt.Errorf("model server is nil")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("model binding token is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bindings[token] == nil {
		return fmt.Errorf("model binding token is unknown")
	}
	delete(s.bindings, token)
	return nil
}

func (s *Server) URL() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.address == "" {
		return ""
	}
	return "http://" + s.address
}

func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	httpServer := s.http
	serveErr := s.serveErr
	s.mu.RUnlock()
	var shutdownErr error
	if httpServer != nil {
		shutdownErr = httpServer.Shutdown(ctx)
	}
	var runErr error
	if serveErr != nil {
		runErr = <-serveErr
	}
	recordErr := s.closeRecord()
	s.handlerErrMu.Lock()
	handlerErr := s.handlerErr
	s.handlerErrMu.Unlock()
	return errors.Join(shutdownErr, runErr, recordErr, handlerErr)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, request *http.Request) {
	binding, err := s.bindingForRequest(request)
	if err != nil {
		s.recordHandlerError(writeChatError(w, http.StatusUnauthorized, err))
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxChatRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.UseNumber()
	var chat chatRequest
	if err := decoder.Decode(&chat); err != nil {
		s.recordHandlerError(writeChatError(w, http.StatusBadRequest, fmt.Errorf("decode chat request: %w", err)))
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		s.recordHandlerError(writeChatError(w, http.StatusBadRequest, err))
		return
	}
	response, err := s.executeChat(request.Context(), binding, chat)
	if err != nil {
		status := http.StatusBadGateway
		if class := modelapi.ErrorClass(err); class == modelapi.ProviderErrorRequest || class == modelapi.ProviderErrorProtocol {
			status = http.StatusBadRequest
		}
		s.recordHandlerError(writeChatError(w, status, err))
		return
	}
	if chat.Stream {
		s.recordHandlerError(writeChatStream(w, chat.Model, response))
		return
	}
	s.recordHandlerError(writeChatResponse(w, chat.Model, response))
}

func (s *Server) bindingForRequest(request *http.Request) (*chatBinding, error) {
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	scheme, token, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(strings.TrimSpace(scheme), "Bearer") || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("bearer token is required")
	}
	s.mu.RLock()
	binding := s.bindings[strings.TrimSpace(token)]
	s.mu.RUnlock()
	if binding == nil {
		return nil, fmt.Errorf("bearer token is invalid")
	}
	return binding, nil
}

func (s *Server) executeChat(ctx context.Context, binding *chatBinding, chat chatRequest) (modelapi.Response, error) {
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if strings.TrimSpace(chat.Model) == "" {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: fmt.Errorf("chat model is required")}
	}
	if chat.Model != binding.alias {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: fmt.Errorf("chat model does not match the authorized model")}
	}
	if len(chat.Messages) == 0 {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: fmt.Errorf("chat messages are required")}
	}
	if chat.ToolChoice != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: fmt.Errorf("chat tool_choice is unsupported")}
	}
	if err := checkMessagePrefix(chat.Messages, binding.expectedMessages); err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	newMessages := chat.Messages[len(binding.expectedMessages):]
	newInput, err := chatInputItems(newMessages)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	tools, err := chatTools(chat.Tools)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	input := append(append([]map[string]any(nil), binding.inputItems...), newInput...)
	started := time.Now().UTC()
	response, requestErr := s.executor.CreateResponseWithRequestSpec(ctx, binding.spec, input, tools, binding.previousResponseID)
	record := requestRecord{
		Name:          binding.name,
		Endpoint:      binding.spec.Endpoint,
		Model:         binding.spec.UpstreamModel(),
		ReturnedModel: response.ReturnedModel,
		ResponseID:    response.ResponseID,
		StartedAt:     started,
		CompletedAt:   time.Now().UTC(),
		Usage:         response.TokenUsage(),
	}
	if requestErr != nil {
		record.ProviderError = requestErr.Error()
		record.ProviderClass = string(modelapi.ErrorClass(requestErr))
	}
	if err := s.writeRecord(record); err != nil {
		return modelapi.Response{}, err
	}
	if requestErr != nil {
		return modelapi.Response{}, requestErr
	}
	binding.inputItems = input
	binding.previousResponseID = response.ResponseID
	binding.expectedMessages = append(append([]map[string]any(nil), chat.Messages...), chatAssistantMessage(response))
	return response, nil
}

func checkMessagePrefix(messages, expected []map[string]any) error {
	if len(messages) < len(expected) {
		return fmt.Errorf("chat request removed prior messages")
	}
	for index := range expected {
		messageJSON, err := json.Marshal(messages[index])
		if err != nil {
			return fmt.Errorf("encode chat message %d: %w", index, err)
		}
		expectedJSON, err := json.Marshal(expected[index])
		if err != nil {
			return fmt.Errorf("encode prior chat message %d: %w", index, err)
		}
		if !bytes.Equal(messageJSON, expectedJSON) {
			return fmt.Errorf("chat request changed prior message %d", index)
		}
	}
	return nil
}

func chatInputItems(messages []map[string]any) ([]map[string]any, error) {
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		role, _ := message["role"].(string)
		role = strings.ToLower(strings.TrimSpace(role))
		switch role {
		case "system", "developer", "user":
			item, err := chatMessageInput(role, message["content"])
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		case "tool":
			callID, _ := message["tool_call_id"].(string)
			if strings.TrimSpace(callID) == "" {
				return nil, fmt.Errorf("tool message requires tool_call_id")
			}
			output, err := chatTextContent(message["content"])
			if err != nil {
				return nil, fmt.Errorf("tool message content: %w", err)
			}
			items = append(items, map[string]any{"type": "function_call_output", "call_id": callID, "output": output})
		case "assistant":
			return nil, fmt.Errorf("new chat messages contain an assistant message")
		default:
			return nil, fmt.Errorf("unsupported chat message role %q", role)
		}
	}
	return items, nil
}

func chatMessageInput(role string, content any) (map[string]any, error) {
	if text, ok := content.(string); ok {
		return map[string]any{"role": role, "content": text}, nil
	}
	parts, ok := content.([]any)
	if !ok {
		return nil, fmt.Errorf("%s message content must be a string or array", role)
	}
	contentItems := make([]map[string]any, 0, len(parts))
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s message content contains a non-object part", role)
		}
		partType, _ := part["type"].(string)
		switch partType {
		case "text", "input_text":
			text, _ := part["text"].(string)
			contentItems = append(contentItems, map[string]any{"type": "input_text", "text": text})
		case "image_url":
			image, ok := part["image_url"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s image_url part requires an object", role)
			}
			imageURL, _ := image["url"].(string)
			if strings.TrimSpace(imageURL) == "" {
				return nil, fmt.Errorf("%s image_url part requires url", role)
			}
			contentItems = append(contentItems, map[string]any{"type": "input_image", "image_url": imageURL})
		default:
			return nil, fmt.Errorf("unsupported %s message content type %q", role, partType)
		}
	}
	return map[string]any{"role": role, "content_items": contentItems}, nil
}

func chatTextContent(content any) (string, error) {
	if text, ok := content.(string); ok {
		return text, nil
	}
	parts, ok := content.([]any)
	if !ok {
		return "", fmt.Errorf("content must be a string or array")
	}
	var text strings.Builder
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			return "", fmt.Errorf("content contains a non-object part")
		}
		partText, ok := part["text"].(string)
		if !ok {
			return "", fmt.Errorf("content part requires text")
		}
		text.WriteString(partText)
	}
	return text.String(), nil
}

func chatTools(tools []map[string]any) ([]map[string]any, error) {
	converted := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		function, ok := tool["function"].(map[string]any)
		if toolType != "function" || !ok {
			return nil, fmt.Errorf("chat tools must use type function")
		}
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("chat function tool requires a name")
		}
		parameters, err := copyObject(function["parameters"])
		if err != nil {
			return nil, fmt.Errorf("chat function tool %s parameters: %w", name, err)
		}
		item := map[string]any{"type": "function", "name": name, "parameters": parameters}
		if description, _ := function["description"].(string); strings.TrimSpace(description) != "" {
			item["description"] = description
		}
		if strict, ok := function["strict"].(bool); ok {
			item["strict"] = strict
		}
		converted = append(converted, item)
	}
	return converted, nil
}

func chatAssistantMessage(response modelapi.Response) map[string]any {
	message := map[string]any{"role": "assistant", "content": response.Text}
	if len(response.ToolCalls) == 0 {
		return message
	}
	if response.Text == "" {
		message["content"] = nil
	}
	calls := make([]map[string]any, 0, len(response.ToolCalls))
	for _, call := range response.ToolCalls {
		arguments := call.RawArguments
		if strings.TrimSpace(arguments) == "" {
			raw, err := json.Marshal(call.Arguments)
			if err == nil {
				arguments = string(raw)
			}
		}
		calls = append(calls, map[string]any{
			"id":       call.CallID,
			"type":     "function",
			"function": map[string]any{"name": call.Name, "arguments": arguments},
		})
	}
	message["tool_calls"] = calls
	return message
}

func writeChatStream(w http.ResponseWriter, model string, response modelapi.Response) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	created := time.Now().Unix()
	delta := map[string]any{"role": "assistant"}
	if response.Text != "" {
		delta["content"] = response.Text
	}
	if len(response.ToolCalls) > 0 {
		calls := make([]map[string]any, 0, len(response.ToolCalls))
		for index, call := range response.ToolCalls {
			arguments := call.RawArguments
			if strings.TrimSpace(arguments) == "" {
				raw, err := json.Marshal(call.Arguments)
				if err == nil {
					arguments = string(raw)
				}
			}
			calls = append(calls, map[string]any{
				"index": index,
				"id":    call.CallID,
				"type":  "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": arguments,
				},
			})
		}
		delta["tool_calls"] = calls
	}
	finishReason := "stop"
	if len(response.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}
	if err := writeSSEJSON(w, map[string]any{
		"id": response.ResponseID, "object": "chat.completion.chunk", "created": created, "model": model,
		"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finishReason}},
	}); err != nil {
		return err
	}
	if response.UsageKnown {
		if err := writeSSEJSON(w, map[string]any{
			"id": response.ResponseID, "object": "chat.completion.chunk", "created": created, "model": model,
			"choices": []any{}, "usage": chatUsage(response.Usage),
		}); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("write chat stream terminator: %w", err)
	}
	return nil
}

func writeChatResponse(w http.ResponseWriter, model string, response modelapi.Response) error {
	w.Header().Set("Content-Type", "application/json")
	finishReason := "stop"
	if len(response.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}
	body := map[string]any{
		"id": response.ResponseID, "object": "chat.completion", "created": time.Now().Unix(), "model": model,
		"choices": []map[string]any{{"index": 0, "message": chatAssistantMessage(response), "finish_reason": finishReason}},
	}
	if response.UsageKnown {
		body["usage"] = chatUsage(response.Usage)
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return fmt.Errorf("write chat response: %w", err)
	}
	return nil
}

func chatUsage(usage modelapi.Usage) map[string]any {
	return map[string]any{
		"prompt_tokens":             usage.InputTokens,
		"completion_tokens":         usage.OutputTokens,
		"total_tokens":              usage.TotalTokens,
		"prompt_tokens_details":     map[string]any{"cached_tokens": usage.CachedInputTokens},
		"completion_tokens_details": map[string]any{"reasoning_tokens": usage.ReasoningTokens},
	}
}

func writeSSEJSON(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode chat stream event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
		return fmt.Errorf("write chat stream event: %w", err)
	}
	return nil
}

func writeChatError(w http.ResponseWriter, status int, requestErr error) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": requestErr.Error(), "type": "adj_model_gateway_error"}}); err != nil {
		return fmt.Errorf("write chat error response: %w", err)
	}
	return nil
}

func (s *Server) recordHandlerError(err error) {
	if err == nil {
		return
	}
	s.handlerErrMu.Lock()
	s.handlerErr = errors.Join(s.handlerErr, err)
	s.handlerErrMu.Unlock()
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing chat request data: %w", err)
	}
	return fmt.Errorf("chat request contains trailing JSON data")
}

func randomIdentifier(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Server) writeRecord(record requestRecord) error {
	s.recordMu.Lock()
	defer s.recordMu.Unlock()
	if s.record == nil {
		return nil
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode model request record: %w", err)
	}
	raw = append(raw, '\n')
	if _, err := s.record.Write(raw); err != nil {
		return fmt.Errorf("write model request record: %w", err)
	}
	return nil
}

func (s *Server) closeRecord() error {
	s.recordMu.Lock()
	defer s.recordMu.Unlock()
	if s.record == nil {
		return nil
	}
	err := s.record.Close()
	s.record = nil
	if err != nil {
		return fmt.Errorf("close model request record: %w", err)
	}
	return nil
}
