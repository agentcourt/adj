package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type testAdapter struct{}

func (testAdapter) OpenSession(assignment Assignment) (Profile, error) {
	return Profile{
		CaseID:         assignment.CaseID,
		AssignmentType: assignment.AssignmentType,
		PrincipalID:    assignment.PrincipalID,
		Instructions:   "Bound session for " + assignment.PrincipalID,
	}, nil
}

func (testAdapter) Tools(profile Profile) ([]Tool, error) {
	return []Tool{{
		Name:        "inspect",
		Description: "Inspect the bound profile.",
		InputSchema: EmptyObjectSchema(),
		ReadOnly:    true,
	}}, nil
}

func (testAdapter) CallTool(_ context.Context, profile Profile, name string, arguments map[string]any) (CallResult, error) {
	if name != "inspect" {
		return CallResult{}, fmt.Errorf("unknown tool %q", name)
	}
	return CallResult{StructuredContent: map[string]any{
		"ok":           true,
		"case_id":      profile.CaseID,
		"principal_id": profile.PrincipalID,
		"arguments":    arguments,
	}}, nil
}

func TestCallToolAcceptsRequestMetadata(t *testing.T) {
	server := &server{adapter: testAdapter{}}
	profile := Profile{CaseID: "case-7", PrincipalID: "plaintiff"}
	tests := map[string]string{
		"empty":                         `{"name":"inspect","arguments":{"detail":"full"},"_meta":{}}`,
		"standard and extension fields": `{"name":"inspect","arguments":{"detail":"full"},"_meta":{"progressToken":"progress-1","example.test/trace":{"id":7}}}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := server.callTool(context.Background(), profile, json.RawMessage(raw))
			if err != nil {
				t.Fatal(err)
			}
			arguments, ok := result.StructuredContent["arguments"].(map[string]any)
			if !ok {
				t.Fatalf("arguments = %#v", result.StructuredContent["arguments"])
			}
			if len(arguments) != 1 || arguments["detail"] != "full" {
				t.Fatalf("arguments = %#v", arguments)
			}
		})
	}
}

func TestCallToolRejectsInvalidEnvelopeFields(t *testing.T) {
	server := &server{adapter: testAdapter{}}
	profile := Profile{CaseID: "case-7", PrincipalID: "plaintiff"}
	tests := map[string]struct {
		raw       string
		wantError string
	}{
		"unknown field": {
			raw:       `{"name":"inspect","arguments":{},"unexpected":true}`,
			wantError: `unknown field "unexpected"`,
		},
		"null metadata": {
			raw:       `{"name":"inspect","arguments":{},"_meta":null}`,
			wantError: "_meta must be an object",
		},
		"array metadata": {
			raw:       `{"name":"inspect","arguments":{},"_meta":[]}`,
			wantError: "_meta must be an object",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := server.callTool(context.Background(), profile, json.RawMessage(test.raw))
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want text %q", err, test.wantError)
			}
		})
	}
}

func TestTransportSessionLifecycleAndProfile(t *testing.T) {
	key := goldenCapabilityKey()
	assignment := Assignment{Audience: "quick", CaseID: "case-7", AssignmentType: "lawyer", PrincipalID: "plaintiff"}
	token, err := IssueCapability(key, assignment)
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := IssueCapability(key, Assignment{Audience: "quick", CaseID: "case-7", AssignmentType: "lawyer", PrincipalID: "defendant"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			ListenAddr:           "127.0.0.1:0",
			Audience:             "quick",
			SigningKey:           key,
			DisableSessionExpiry: true,
			AllowedOrigins:       []string{"https://client.example"},
			ServerName:           "test",
			Adapter:              testAdapter{},
			ListenerReady: func(addr string) error {
				ready <- addr
				return nil
			},
		})
	}()
	addr := <-ready
	baseURL := "http://" + addr + "/mcp"

	queried := postRPC(t, baseURL+"?case_id=case-7&role_id=plaintiff", token, "", "", "initialize", map[string]any{})
	if queried.status != http.StatusBadRequest {
		t.Fatalf("identity query status = %d", queried.status)
	}
	unauthorized := postRPC(t, baseURL, "", "", "", "initialize", map[string]any{})
	if unauthorized.status != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.status)
	}
	initialized := postRPC(t, baseURL, token, "https://client.example", "", "initialize", map[string]any{})
	if initialized.status != http.StatusOK || initialized.sessionID == "" {
		t.Fatalf("initialize status = %d, session = %q, body = %s", initialized.status, initialized.sessionID, initialized.body)
	}
	var initResponse rpcResponse
	if err := json.Unmarshal(initialized.body, &initResponse); err != nil {
		t.Fatal(err)
	}
	result, _ := initResponse.Result.(map[string]any)
	if result["instructions"] != "Bound session for plaintiff" {
		t.Fatalf("initialize result = %#v", result)
	}

	listed := postRPC(t, baseURL, token, "https://client.example", initialized.sessionID, "tools/list", map[string]any{})
	var listResponse struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listed.body, &listResponse); err != nil {
		t.Fatal(err)
	}
	if len(listResponse.Result.Tools) != 1 || listResponse.Result.Tools[0]["name"] != "inspect" {
		t.Fatalf("tools = %#v", listResponse.Result.Tools)
	}

	mismatched := postRPC(t, baseURL, otherToken, "https://client.example", initialized.sessionID, "tools/list", map[string]any{})
	if mismatched.status != http.StatusUnauthorized {
		t.Fatalf("mismatched capability status = %d", mismatched.status)
	}

	called := postRPC(t, baseURL, token, "https://client.example", initialized.sessionID, "tools/call", map[string]any{
		"name":      "inspect",
		"arguments": map[string]any{"detail": "full"},
		"_meta":     map[string]any{"progressToken": "progress-1"},
	})
	var callResponse struct {
		Result struct {
			Structured map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(called.body, &callResponse); err != nil {
		t.Fatal(err)
	}
	if callResponse.Result.Structured["case_id"] != "case-7" || callResponse.Result.Structured["principal_id"] != "plaintiff" {
		t.Fatalf("structured result = %#v", callResponse.Result.Structured)
	}

	if status := deleteMCPSession(t, baseURL, otherToken, "https://client.example", initialized.sessionID); status != http.StatusUnauthorized {
		t.Fatalf("mismatched delete status = %d", status)
	}
	retained := postRPC(t, baseURL, token, "https://client.example", initialized.sessionID, "tools/list", map[string]any{})
	if retained.status != http.StatusOK {
		t.Fatalf("session status after mismatched delete = %d", retained.status)
	}
	if status := deleteMCPSession(t, baseURL, token, "https://client.example", initialized.sessionID); status != http.StatusNoContent {
		t.Fatalf("delete status = %d", status)
	}
	expired := postRPC(t, baseURL, token, "https://client.example", initialized.sessionID, "tools/list", map[string]any{})
	if expired.status != http.StatusNotFound {
		t.Fatalf("deleted session status = %d", expired.status)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MCP server did not stop")
	}
}

func deleteMCPSession(t *testing.T, rawURL, token, origin, sessionID string) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodDelete, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Origin", origin)
	request.Header.Set("Mcp-Session-Id", sessionID)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode
}

type testHTTPResponse struct {
	status    int
	sessionID string
	body      []byte
}

func postRPC(t *testing.T, rawURL, token, origin, sessionID, method string, params map[string]any) testHTTPResponse {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return testHTTPResponse{
		status:    response.StatusCode,
		sessionID: response.Header.Get("Mcp-Session-Id"),
		body:      responseBody,
	}
}
