package lean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEngineScript(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	return path
}

func readCapturedRequest(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	return out
}

func TestNewDefaultsToLakeExe(t *testing.T) {
	t.Parallel()

	engine := New(nil)
	if strings.Join(engine.Command, " ") != "lake exe adcengine" {
		t.Fatalf("Command = %v", engine.Command)
	}
}

func TestCallWritesRequestAndParsesResponse(t *testing.T) {
	t.Parallel()

	requestPath := filepath.Join(t.TempDir(), "request.json")
	script := writeEngineScript(t, "#!/bin/sh\ncat >\"$1\"\nprintf '%s' '{\"ok\":true}'\n")
	engine := New([]string{script, requestPath})

	out, err := engine.Call(map[string]any{"request_type": "ping"})
	if err != nil {
		t.Fatalf("Call error = %v", err)
	}
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("Call output = %+v", out)
	}
	request := readCapturedRequest(t, requestPath)
	if request["request_type"] != "ping" {
		t.Fatalf("captured request = %+v", request)
	}
}

func TestCallRejectsBadProcessOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		want   string
		reqArg bool
	}{
		{name: "empty response", body: "#!/bin/sh\ncat >/dev/null\n", want: "empty response"},
		{name: "invalid json", body: "#!/bin/sh\ncat >/dev/null\nprintf '%s' 'not-json'\n", want: "parse lean json"},
		{name: "process failure", body: "#!/bin/sh\ncat >/dev/null\necho boom >&2\nexit 7\n", want: "stderr=boom"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			script := writeEngineScript(t, tt.body)
			engine := New([]string{script})
			_, err := engine.Call(map[string]any{"request_type": "ping"})
			if err == nil {
				t.Fatalf("Call error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Call error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestCallRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	engine := Engine{}
	_, err := engine.Call(map[string]any{"request_type": "ping"})
	if err == nil {
		t.Fatalf("Call error = nil, want error")
	}
	if !strings.Contains(err.Error(), "lean command is empty") {
		t.Fatalf("Call error = %v", err)
	}
}

func TestHelperMethodsBuildExpectedRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		call  func(Engine) (map[string]any, error)
		check func(*testing.T, map[string]any)
	}{
		{
			name: "step",
			call: func(engine Engine) (map[string]any, error) {
				return engine.Step(map[string]any{"case": "state"}, "issue_order", "judge", map[string]any{"title": "Order"})
			},
			check: func(t *testing.T, request map[string]any) {
				action, _ := request["action"].(map[string]any)
				if action["action_type"] != "issue_order" || action["actor_role"] != "judge" {
					t.Fatalf("step request = %+v", request)
				}
			},
		},
		{
			name: "view",
			call: func(engine Engine) (map[string]any, error) {
				return engine.View(map[string]any{"case": "state"}, "plaintiff")
			},
			check: func(t *testing.T, request map[string]any) {
				if request["request_type"] != "role_view" || request["role"] != "plaintiff" {
					t.Fatalf("view request = %+v", request)
				}
			},
		},
		{
			name: "next opportunity",
			call: func(engine Engine) (map[string]any, error) {
				return engine.NextOpportunity(map[string]any{"case": "state"}, []map[string]any{{"name": "judge"}}, 4)
			},
			check: func(t *testing.T, request map[string]any) {
				if request["request_type"] != "next_opportunity" || int(request["max_steps_per_turn"].(float64)) != 4 {
					t.Fatalf("next opportunity request = %+v", request)
				}
			},
		},
		{
			name: "apply decision",
			call: func(engine Engine) (map[string]any, error) {
				return engine.ApplyDecision(map[string]any{"case": "state"}, 7, "o1", "judge", map[string]any{"action_type": "issue_order"}, []map[string]any{{"name": "judge"}}, 3)
			},
			check: func(t *testing.T, request map[string]any) {
				if request["request_type"] != "apply_decision" || request["opportunity_id"] != "o1" || request["role"] != "judge" {
					t.Fatalf("apply decision request = %+v", request)
				}
			},
		},
		{
			name: "initialize case",
			call: func(engine Engine) (map[string]any, error) {
				return engine.InitializeCase(map[string]any{"case": "state"}, "summary", "plaintiff", "2026-03-15", map[string]any{"basis": "diversity"}, []map[string]any{{"name": "confession.txt"}})
			},
			check: func(t *testing.T, request map[string]any) {
				if request["request_type"] != "initialize_case" || request["jury_demanded_on"] != "2026-03-15" {
					t.Fatalf("initialize case request = %+v", request)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requestPath := filepath.Join(t.TempDir(), "request.json")
			script := writeEngineScript(t, "#!/bin/sh\ncat >\"$1\"\nprintf '%s' '{}'\n")
			engine := New([]string{script, requestPath})
			if _, err := tt.call(engine); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			tt.check(t, readCapturedRequest(t, requestPath))
		})
	}
}
