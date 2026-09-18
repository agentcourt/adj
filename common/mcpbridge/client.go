package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CallHTTPTool calls one tool on this repository's JSON-response MCP server.
func CallHTTPTool(ctx context.Context, endpoint, token, name string, arguments map[string]any) (content map[string]any, returnErr error) {
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	sessionID := ""
	request := func(ctx context.Context, method string, body []byte) ([]byte, error) {
		if len(body) > MaxRequestBytes {
			return nil, fmt.Errorf("MCP request exceeds %d bytes", MaxRequestBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(resp.Body)
		if err := errors.Join(readErr, resp.Body.Close()); err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("MCP HTTP %d: %s", resp.StatusCode, raw)
		}
		if value := resp.Header.Get("Mcp-Session-Id"); value != "" {
			sessionID = value
		}
		return raw, nil
	}
	post := func(method string, params any, id int) (map[string]any, error) {
		message := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
		if id != 0 {
			message["id"] = id
		}
		body, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}
		raw, err := request(ctx, http.MethodPost, body)
		if err != nil || id == 0 {
			return nil, err
		}
		var response rpcResponse
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode MCP %s response: %w", method, err)
		}
		if response.Error != nil {
			return nil, fmt.Errorf("MCP %s: %s", method, response.Error.Message)
		}
		if response.JSONRPC != "2.0" || response.ID == nil || string(*response.ID) != fmt.Sprint(id) {
			return nil, fmt.Errorf("invalid MCP %s response identity", method)
		}
		result, ok := response.Result.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("MCP %s returned no result object", method)
		}
		return result, nil
	}
	initial, err := post("initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "adj-file-import", "version": "1"},
	}, 1)
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		return nil, fmt.Errorf("MCP initialization returned no session ID")
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, err := request(cleanupCtx, http.MethodDelete, nil)
		if err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close MCP session: %w", err))
		}
	}()
	if initial["protocolVersion"] != ProtocolVersion {
		return nil, fmt.Errorf("unsupported MCP protocol version %v", initial["protocolVersion"])
	}
	if _, err := post("notifications/initialized", map[string]any{}, 0); err != nil {
		return nil, err
	}
	result, err := post("tools/call", map[string]any{"name": name, "arguments": arguments}, 2)
	if err != nil {
		return nil, err
	}
	content, ok := result["structuredContent"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("MCP %s returned no structured content", name)
	}
	if result["isError"] == true || content["ok"] == false {
		detail, err := json.Marshal(content)
		if err != nil {
			return nil, err
		}
		return content, fmt.Errorf("MCP %s failed: %s", name, detail)
	}
	return content, nil
}
