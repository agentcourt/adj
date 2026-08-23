package proceeding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/arbd/runtime/lean"
)

func TestLawyerDoRequiresActiveOpportunityID(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()

	for _, tc := range []struct {
		name          string
		opportunityID string
		wantCode      string
	}{
		{name: "missing", wantCode: "missing_opportunity_id"},
		{name: "stale", opportunityID: "openings:defendant", wantCode: "stale_opportunity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"case_id":   "arbd-1",
				"role_id":   "plaintiff",
				"tool":      "get_case",
				"arguments": map[string]any{},
			}
			if tc.opportunityID != "" {
				body["opportunity_id"] = tc.opportunityID
			}
			status, got := callLawyerAPIDo(t, api, body)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if got["ok"] != false {
				t.Fatalf("ok = %#v, want false", got["ok"])
			}
			if code := lawyerAPIErrorCode(t, got); code != tc.wantCode {
				t.Fatalf("error code = %q, want %q", code, tc.wantCode)
			}
			if turn.attemptsRemaining != turn.attemptsMax {
				t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
			}
		})
	}

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"role_id":        "plaintiff",
		"opportunity_id": "openings:plaintiff",
		"tool":           "get_case",
		"arguments":      map[string]any{},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true", got["ok"])
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
}

func TestLawyerDoRejectsCanceledCaseBeforeToolMutation(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.OutputDir = t.TempDir()
	caseCtx, cancelCase := context.WithCancelCause(context.Background())
	wantErr := errors.New("case canceled before lawyer tool call")
	cancelCase(wantErr)
	body := map[string]any{
		"case_id":        "arbd-1",
		"role_id":        "plaintiff",
		"opportunity_id": turn.opportunity.ID,
		"tool":           "send_work_notes",
		"arguments":      map[string]any{"notes": "must not be written"},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	api.handleDo(caseCtx, rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := lawyerAPIErrorCode(t, got); code != "runtime_failure" {
		t.Fatalf("error code = %q, want runtime_failure", code)
	}
	if message := mapString(mapAny(got["error"])["message"]); message != wantErr.Error() {
		t.Fatalf("error message = %q, want %q", message, wantErr)
	}
	if !turn.completed || turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("turn completed=%t attempts=%d, want true and %d", turn.completed, turn.attemptsRemaining, turn.attemptsMax)
	}
	entries, err := os.ReadDir(api.rc.cfg.OutputDir)
	if err != nil {
		t.Fatalf("read output directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("canceled tool wrote output entries: %#v", entries)
	}
}

func TestLawyerDoRejectsTrailingJSONValueWithoutCountingAttempt(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	raw := `{"case_id":"arbd-1","role_id":"plaintiff","opportunity_id":"openings:plaintiff","tool":"get_case","arguments":{}} {}`

	status, got := callLawyerAPIDoRaw(t, api, raw)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if code := lawyerAPIErrorCode(t, got); code != "bad_json" {
		t.Fatalf("error code = %q, want bad_json", code)
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
}

func TestLawyerDoRejectsUnknownToolArgumentFields(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
		phase     string
	}{
		{name: "get case", tool: "get_case", arguments: map[string]any{"unexpected": true}},
		{name: "work notes", tool: "send_work_notes", arguments: map[string]any{"notes": "notes", "unexpected": true}},
		{name: "list evidence", tool: "list_evidence", arguments: map[string]any{"unexpected": true}},
		{name: "stat evidence", tool: "stat_evidence", arguments: map[string]any{"evidence_id": "ev-1", "unexpected": true}},
		{name: "read evidence", tool: "read_evidence_range", arguments: map[string]any{"evidence_id": "ev-1", "offset": 0, "length": 1, "unexpected": true}},
		{name: "begin evidence upload", tool: "begin_evidence_upload", phase: "arguments", arguments: map[string]any{
			"title": "Source", "mime_type": "text/plain", "expected_size_bytes": 1, "relevance": "Relevant", "unexpected": true,
		}},
		{name: "write evidence chunk", tool: "write_evidence_chunk", phase: "arguments", arguments: map[string]any{
			"upload_id": "upload-1", "offset": 0, "content_base64": "YQ==", "unexpected": true,
		}},
		{name: "commit evidence upload", tool: "commit_evidence_upload", phase: "arguments", arguments: map[string]any{
			"upload_id": "upload-1", "unexpected": true,
		}},
		{name: "submit evidence", tool: "submit_evidence", phase: "arguments", arguments: map[string]any{
			"title": "Source", "mime_type": "text/plain", "relevance": "Relevant", "content": "a", "unexpected": true,
		}},
		{name: "submit decision", tool: "submit_decision", arguments: map[string]any{
			"kind": "tool", "tool_name": "record_opening_statement", "payload": map[string]any{"text": "Opening."}, "unexpected": true,
		}},
		{name: "decision payload", tool: "submit_decision", arguments: map[string]any{
			"kind": "tool", "tool_name": "record_opening_statement", "payload": map[string]any{"text": "Opening.", "unexpected": true},
		}},
		{name: "offered evidence entry", tool: "submit_decision", phase: "arguments", arguments: map[string]any{
			"kind": "tool", "tool_name": "submit_argument", "payload": map[string]any{
				"text": "Argument.", "offered_evidence": []any{map[string]any{"evidence_id": "ev-1", "label": "PX-1", "unexpected": true}},
			},
		}},
		{name: "technical report entry", tool: "submit_decision", phase: "arguments", arguments: map[string]any{
			"kind": "tool", "tool_name": "submit_argument", "payload": map[string]any{
				"text": "Argument.", "technical_reports": []any{map[string]any{"title": "Report", "summary": "Summary", "unexpected": true}},
			},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api, turn := testLawyerAPIWithTurn()
			if tc.phase != "" {
				turn.opportunity.ID = tc.phase + ":plaintiff"
				turn.opportunity.Phase = tc.phase
				turn.opportunity.AllowedTools = []string{"submit_argument"}
			}
			status, got := callLawyerAPIDo(t, api, map[string]any{
				"case_id":        "arbd-1",
				"role_id":        "plaintiff",
				"opportunity_id": turn.opportunity.ID,
				"tool":           tc.tool,
				"arguments":      tc.arguments,
			})
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if got["ok"] != false || lawyerAPIErrorCode(t, got) != "tool_failed" {
				t.Fatalf("response = %#v, want tool_failed", got)
			}
			if message := mapString(mapAny(got["error"])["message"]); !strings.Contains(message, "unknown field \"unexpected\"") {
				t.Fatalf("error message = %q, want unknown field", message)
			}
			if turn.attemptsRemaining != turn.attemptsMax-1 {
				t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax-1)
			}
			if turn.completed {
				t.Fatal("unknown tool argument completed the turn")
			}
		})
	}
}

func TestLawyerPreTurnAndObserverToolsRejectUnknownArgumentFields(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":   "arbd-1",
		"role_id":   "plaintiff",
		"tool":      "case_status",
		"arguments": map[string]any{"unexpected": true},
	})
	if status != http.StatusOK || got["ok"] != false || lawyerAPIErrorCode(t, got) != "tool_failed" {
		t.Fatalf("case_status response = %#v with status %d, want tool_failed", got, status)
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("case_status attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}

	for _, tc := range []struct {
		tool      string
		arguments map[string]any
	}{
		{tool: "case_status", arguments: map[string]any{"unexpected": true}},
		{tool: "get_case", arguments: map[string]any{"unexpected": true}},
		{tool: "get_turn", arguments: map[string]any{"unexpected": true}},
		{tool: "list_events", arguments: map[string]any{"offset": 0, "limit": 1, "unexpected": true}},
		{tool: "list_evidence", arguments: map[string]any{"unexpected": true}},
		{tool: "stat_evidence", arguments: map[string]any{"evidence_id": "ev-1", "unexpected": true}},
		{tool: "read_evidence_range", arguments: map[string]any{"evidence_id": "ev-1", "offset": 0, "length": 1, "unexpected": true}},
	} {
		t.Run("observer "+tc.tool, func(t *testing.T) {
			status, got := callLawyerAPIDo(t, api, map[string]any{
				"case_id":   "arbd-1",
				"role_id":   "observer",
				"tool":      tc.tool,
				"arguments": tc.arguments,
			})
			if status != http.StatusOK || got["ok"] != false || lawyerAPIErrorCode(t, got) != "tool_failed" {
				t.Fatalf("response = %#v with status %d, want tool_failed", got, status)
			}
		})
	}
}

func TestLawyerAPIRejectsMismatchedCaseID(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.CaseID = "case-123"

	status, got := callLawyerAPIGet(t, api, "case_id=case-456&role_id=plaintiff")
	if status != http.StatusNotFound {
		t.Fatalf("get status = %d, want %d", status, http.StatusNotFound)
	}
	if code := lawyerAPIErrorCode(t, got); code != "unknown_case" {
		t.Fatalf("get error code = %q, want unknown_case", code)
	}

	status, got = callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "case-456",
		"role_id":        "plaintiff",
		"opportunity_id": "openings:plaintiff",
		"tool":           "get_case",
		"arguments":      map[string]any{},
	})
	if status != http.StatusNotFound {
		t.Fatalf("do status = %d, want %d", status, http.StatusNotFound)
	}
	if code := lawyerAPIErrorCode(t, got); code != "unknown_case" {
		t.Fatalf("do error code = %q, want unknown_case", code)
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
}

func TestObserverDoDoesNotRequireOpportunityID(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":   "arbd-1",
		"role_id":   "observer",
		"tool":      "get_turn",
		"arguments": map[string]any{},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true", got["ok"])
	}
	result, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", got["result"])
	}
	turn, ok := result["turn"].(map[string]any)
	if !ok {
		t.Fatalf("result.turn = %#v, want object", result["turn"])
	}
	if turn["opportunity_id"] != "openings:plaintiff" {
		t.Fatalf("opportunity_id = %#v, want openings:plaintiff", turn["opportunity_id"])
	}
}

func TestLawyerSendWorkNotesWritesOffRecordLog(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.OutputDir = t.TempDir()
	notes := strings.Repeat("plan source-chain OCR adverse checks\n", 128)

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"role_id":        "plaintiff",
		"opportunity_id": "openings:plaintiff",
		"tool":           "send_work_notes",
		"call_id":        "notes-1",
		"arguments": map[string]any{
			"notes": notes,
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true", got["ok"])
	}
	if turn.completed {
		t.Fatalf("send_work_notes completed the turn")
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
	if len(api.rc.events) != 0 {
		t.Fatalf("send_work_notes recorded case events: %#v", api.rc.events)
	}
	raw, err := os.ReadFile(api.rc.cfg.OutputDir + "/work-notes.ndjson")
	if err != nil {
		t.Fatalf("read work-notes.ndjson: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("work note lines = %d, want 1: %s", len(lines), raw)
	}
	var note WorkNote
	if err := json.Unmarshal([]byte(lines[0]), &note); err != nil {
		t.Fatalf("decode work note: %v", err)
	}
	if note.Notes != notes {
		t.Fatalf("notes were not preserved exactly")
	}
	if note.Role != "plaintiff" || note.Phase != "openings" || note.OpportunityID != "openings:plaintiff" || note.CallID != "notes-1" {
		t.Fatalf("unexpected note metadata: %#v", note)
	}
}

func TestLawyerSubmitEvidenceWritesLiveManifest(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	engineScript := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '{\"ok\":true,\"state\":{\"case\":{\"status\":\"open\",\"phase\":\"arguments\",\"submitted_evidence\":[]},\"state_version\":1}}'\n"
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	api.rc.cfg.OutputDir = dir
	api.rc.cfg.Engine = lean.Engine{Command: []string{enginePath}}
	api.rc.evidenceStoreDir = filepath.Join(dir, "evidence-store")
	turn.opportunity = Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		AllowedTools: []string{"submit_evidence", "submit_argument"},
	}

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"role_id":        "plaintiff",
		"opportunity_id": "arguments:plaintiff",
		"tool":           "submit_evidence",
		"arguments": map[string]any{
			"title":                  "Source",
			"mime_type":              "text/plain",
			"source_url":             "https://example.test/source",
			"relevance":              "Shows the disputed fact.",
			"content":                "source evidence\n",
			"retrieval_timestamp":    "2026-07-09T12:00:00Z",
			"preferred_filename_ext": "txt",
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, body = %#v", got["ok"], got)
	}
	if turn.opportunity.StateVersion != 1 {
		t.Fatalf("turn state version = %d, want 1", turn.opportunity.StateVersion)
	}
	result := got["result"].(map[string]any)
	evidenceID := mapString(result["evidence_id"])
	if evidenceID == "" {
		t.Fatalf("missing evidence_id in %#v", result)
	}
	rawManifest, err := os.ReadFile(filepath.Join(dir, "evidence-manifest.json"))
	if err != nil {
		t.Fatalf("read evidence manifest: %v", err)
	}
	var manifest struct {
		EvidenceCount int            `json:"evidence_count"`
		Evidence      []EvidenceMeta `json:"evidence"`
	}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode evidence manifest: %v", err)
	}
	if manifest.EvidenceCount != 1 || len(manifest.Evidence) != 1 || manifest.Evidence[0].EvidenceID != evidenceID {
		t.Fatalf("manifest evidence = %#v, want %q", manifest, evidenceID)
	}
}

func TestLawyerTimeoutUsesRefreshedEvidenceStateVersion(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	engineScript := `#!/bin/sh
request=$(cat)
case "$request" in
  *\"action_type\":\"submit_evidence\"*\"expected_state_version\":0*)
    printf '%s\n' '{"ok":true,"state":{"case":{"status":"active","phase":"arguments","submitted_evidence":[]},"state_version":1}}'
    ;;
  *\"action_type\":\"fail_opportunity\"*\"expected_state_version\":1*)
    printf '%s\n' '{"ok":true,"state":{"case":{"status":"failed","phase":"arguments","submitted_evidence":[]},"state_version":2}}'
    ;;
  *)
    printf '%s\n' '{"ok":false,"error":"unexpected authority"}'
    ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(engineScript), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	api.rc.cfg.OutputDir = dir
	api.rc.cfg.Engine = lean.New([]string{enginePath})
	api.rc.evidenceStoreDir = filepath.Join(dir, "evidence-store")
	caseObj := mapAny(api.rc.state["case"])
	caseObj["status"] = "active"
	caseObj["phase"] = "arguments"
	turn.opportunity = Opportunity{
		ID:           "arguments:plaintiff",
		StateVersion: 0,
		Role:         "plaintiff",
		Phase:        "arguments",
		AllowedTools: []string{"submit_evidence", "submit_argument"},
	}

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arbd-1",
		"role_id":        "plaintiff",
		"opportunity_id": "arguments:plaintiff",
		"tool":           "submit_evidence",
		"arguments": map[string]any{
			"title":               "Source",
			"mime_type":           "text/plain",
			"source_url":          "https://example.test/source",
			"relevance":           "Shows the disputed fact.",
			"content":             "source evidence\n",
			"retrieval_timestamp": "2026-07-09T12:00:00Z",
		},
	})
	if status != http.StatusOK || got["ok"] != true {
		t.Fatalf("submit evidence status = %d, response = %#v", status, got)
	}
	if turn.opportunity.StateVersion != 1 {
		t.Fatalf("turn state version = %d, want 1", turn.opportunity.StateVersion)
	}
	if err := api.timeoutTurn(context.Background(), turn, time.Second); err != nil {
		t.Fatalf("time out refreshed turn: %v", err)
	}
	if !turn.completed {
		t.Fatal("timed-out turn remains open")
	}
	if got := mapString(mapAny(api.rc.state["case"])["status"]); got != "failed" {
		t.Fatalf("case status = %q, want failed", got)
	}
	if len(api.rc.certificateActions) != 2 {
		t.Fatalf("certificate actions = %d, want 2", len(api.rc.certificateActions))
	}
	if got := api.rc.certificateActions[1].Authority.ExpectedStateVersion; got != 1 {
		t.Fatalf("failure authority state version = %d, want 1", got)
	}
}

func TestLawyerTimeoutPreservesCompletedTurnResult(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	want := errors.New("completed turn result")
	api.finishTurn(turn, want)

	if err := api.timeoutTurn(context.Background(), turn, time.Second); !errors.Is(err, want) {
		t.Fatalf("timeout result = %v, want %v", err, want)
	}
	if len(api.rc.certificateActions) != 0 {
		t.Fatalf("certificate actions = %d, want 0", len(api.rc.certificateActions))
	}
}

func TestLawyerToolSpecsIncludeStatusAndWorkNotesEveryPhase(t *testing.T) {
	for _, phase := range []string{"openings", "arguments", "rebuttals", "surrebuttals", "closings"} {
		tools := lawyerToolSpecs(Opportunity{Phase: phase})
		if !hasLawyerTool(tools, "case_status") {
			t.Fatalf("%s tools missing case_status: %#v", phase, tools)
		}
		if !hasLawyerTool(tools, "send_work_notes") {
			t.Fatalf("%s tools missing send_work_notes: %#v", phase, tools)
		}
	}
}

func TestLawyerStatusReportsWaitingRoleAndActiveTurn(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	caseObj := mapAny(api.rc.state["case"])
	caseObj["status"] = "open"
	caseObj["phase"] = "openings"
	caseObj["deliberation_round"] = 1
	caseObj["arguments"] = []map[string]any{{"role": "plaintiff"}}
	api.rc.events = []Event{{Type: "run_initialized"}}

	status, got := callLawyerAPIStatus(t, api, "case_id=arbd-1&role_id=defendant")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "waiting" {
		t.Fatalf("status field = %#v, want waiting", got["status"])
	}
	if got["phase"] != "openings" || got["case_status"] != "open" {
		t.Fatalf("case fields = phase %#v status %#v", got["phase"], got["case_status"])
	}
	turn, ok := got["turn"].(map[string]any)
	if !ok {
		t.Fatalf("turn = %#v, want object", got["turn"])
	}
	if turn["role_id"] != "plaintiff" || turn["opportunity_id"] != "openings:plaintiff" {
		t.Fatalf("turn = %#v", turn)
	}
	current, ok := got["current_opportunity"].(map[string]any)
	if !ok {
		t.Fatalf("current_opportunity = %#v, want object", got["current_opportunity"])
	}
	if current["role_id"] != "plaintiff" {
		t.Fatalf("current_opportunity = %#v", current)
	}
	if _, ok := current["allowed_operations"]; ok {
		t.Fatalf("current_opportunity used obsolete allowed_operations field: %#v", current)
	}
	actions := stringList(current["final_filing_actions"])
	if len(actions) != 1 || actions[0] != "record_opening_statement" {
		t.Fatalf("current_opportunity final_filing_actions = %#v", current["final_filing_actions"])
	}
	access, ok := current["evidence_access"].(map[string]any)
	if !ok || access["read"] != true || access["submit"] != false {
		t.Fatalf("current_opportunity evidence_access = %#v", current["evidence_access"])
	}
	counts, ok := got["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts = %#v, want object", got["counts"])
	}
	if intNumber(counts["events"]) != 1 || intNumber(counts["arguments"]) != 1 {
		t.Fatalf("counts = %#v", counts)
	}

	_, toolGot := callLawyerAPIDo(t, api, map[string]any{
		"case_id":   "arbd-1",
		"role_id":   "defendant",
		"tool":      "case_status",
		"arguments": map[string]any{},
	})
	if toolGot["ok"] != true {
		t.Fatalf("case_status tool ok = %#v in %#v", toolGot["ok"], toolGot)
	}
	result, ok := toolGot["result"].(map[string]any)
	if !ok {
		t.Fatalf("tool result = %#v, want object", toolGot["result"])
	}
	if result["status"] != "waiting" {
		t.Fatalf("tool result status = %#v, want waiting", result["status"])
	}
}

func TestLawyerGetWaitingAdvertisesCaseStatus(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIGet(t, api, "case_id=arbd-1&role_id=defendant")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "waiting" {
		t.Fatalf("status field = %#v, want waiting", got["status"])
	}
	tools := mapList(got["tools"])
	if !hasLawyerTool(tools, "case_status") {
		t.Fatalf("waiting tools missing case_status: %#v", tools)
	}
	if hasLawyerTool(tools, "submit_decision") {
		t.Fatalf("waiting tools should not include submit_decision: %#v", tools)
	}
}

func TestLawyerWaitReturnsReadyOpportunity(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&timeout_ms=100")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "ready" {
		t.Fatalf("status field = %#v, want ready", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "ready" {
		t.Fatalf("wait.reason = %#v, want ready", wait["reason"])
	}
	turn, ok := got["turn"].(map[string]any)
	if !ok {
		t.Fatalf("turn = %#v, want object", got["turn"])
	}
	if turn["opportunity_id"] != "openings:plaintiff" {
		t.Fatalf("opportunity_id = %#v, want openings:plaintiff", turn["opportunity_id"])
	}
}

func TestLawyerWaitBlocksUntilLaterOpportunity(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	done := make(chan map[string]any, 1)
	go func() {
		_, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=1000")
		done <- got
	}()
	select {
	case got := <-done:
		t.Fatalf("wait returned before a later opportunity: %#v", got)
	case <-time.After(20 * time.Millisecond):
	}
	api.rc.mu.Lock()
	api.active = testLawyerTurn("arguments:plaintiff", "plaintiff", "arguments")
	api.signalChangedLocked()
	api.rc.mu.Unlock()
	select {
	case got := <-done:
		if got["status"] != "ready" {
			t.Fatalf("status field = %#v, want ready", got["status"])
		}
		wait, ok := got["wait"].(map[string]any)
		if !ok {
			t.Fatalf("wait = %#v, want object", got["wait"])
		}
		if wait["reason"] != "ready" {
			t.Fatalf("wait.reason = %#v, want ready", wait["reason"])
		}
		turn, ok := got["turn"].(map[string]any)
		if !ok {
			t.Fatalf("turn = %#v, want object", got["turn"])
		}
		if turn["opportunity_id"] != "arguments:plaintiff" {
			t.Fatalf("opportunity_id = %#v, want arguments:plaintiff", turn["opportunity_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not return after later opportunity")
	}
}

func TestLawyerWaitReturnsOnCaseChange(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	done := make(chan map[string]any, 1)
	go func() {
		_, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=1000")
		done <- got
	}()
	select {
	case got := <-done:
		t.Fatalf("wait returned before case change: %#v", got)
	case <-time.After(20 * time.Millisecond):
	}
	api.rc.mu.Lock()
	api.signalChangedLocked()
	api.rc.mu.Unlock()
	select {
	case got := <-done:
		if got["status"] != "ready" {
			t.Fatalf("status field = %#v, want ready", got["status"])
		}
		wait, ok := got["wait"].(map[string]any)
		if !ok {
			t.Fatalf("wait = %#v, want object", got["wait"])
		}
		if wait["reason"] != "changed" {
			t.Fatalf("wait.reason = %#v, want changed", wait["reason"])
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not return after case change")
	}
}

func TestLawyerWaitReturnsForOlderVersion(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&after=openings:plaintiff&after_version=0&timeout_ms=1000")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "changed" {
		t.Fatalf("wait.reason = %#v, want changed", wait["reason"])
	}
}

func TestLawyerWaitReturnsDoneOnTerminalCase(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	api.rc.lawyerAPI = api
	api.rc.mu.Lock()
	api.rc.setRoleAPIsTerminalLocked("threshold_met")
	api.rc.mu.Unlock()

	status, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=1000")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "done" {
		t.Fatalf("status field = %#v, want done", got["status"])
	}
	if got["final_reason"] != "threshold_met" {
		t.Fatalf("final_reason = %#v, want threshold_met", got["final_reason"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "done" {
		t.Fatalf("wait.reason = %#v, want done", wait["reason"])
	}
}

func TestLawyerAPIsReportFailedCase(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	caseObj := mapAny(api.rc.state["case"])
	caseObj["status"] = "failed"
	caseObj["phase"] = "arguments"
	caseObj["failure"] = map[string]any{
		"failure_type":   "opportunity_failed",
		"role":           "plaintiff",
		"phase":          "arguments",
		"opportunity_id": "arguments:plaintiff",
		"reason":         opportunityFailureDeadline,
		"message":        "Plaintiff lawyer opportunity arguments:plaintiff failed because the deadline expired.",
	}

	status, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&timeout_ms=100")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "failed" {
		t.Fatalf("status field = %#v, want failed", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "failed" {
		t.Fatalf("wait = %#v, want reason failed", got["wait"])
	}
	failure, ok := got["failure"].(map[string]any)
	if !ok || failure["type"] != "opportunity_failed" || failure["reason"] != opportunityFailureDeadline {
		t.Fatalf("failure = %#v", got["failure"])
	}

	_, got = callLawyerAPIResult(t, api, "case_id=arbd-1&role_id=observer")
	if got["status"] != "failed" {
		t.Fatalf("result status = %#v, want failed", got["status"])
	}
	if got["result"] != nil {
		t.Fatalf("result = %#v, want nil for failed case", got["result"])
	}
}

func TestLawyerResultReportsPendingCase(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIResult(t, api, "case_id=arbd-1&role_id=observer")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "pending" {
		t.Fatalf("status field = %#v, want pending", got["status"])
	}
	if got["result"] != nil {
		t.Fatalf("result = %#v, want nil for pending case", got["result"])
	}
	if got["message"] != "The case is still pending." {
		t.Fatalf("message = %#v", got["message"])
	}
}

func TestLawyerResultReportsFinalCouncilAnswers(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	api.rc.lawyerAPI = api
	caseObj := mapAny(api.rc.state["case"])
	caseObj["status"] = "closed"
	caseObj["phase"] = "closed"
	caseObj["deliberation_round"] = 1
	caseObj["council_answers"] = []map[string]any{
		{"round": 1, "member_id": "C1", "answer": 72, "rationale": "official record proves control"},
		{"round": 1, "member_id": "C2", "answer": 28, "rationale": "intent not proven"},
	}
	api.rc.mu.Lock()
	api.rc.setRoleAPIsTerminalLocked("threshold_met")
	api.rc.mu.Unlock()

	status, got := callLawyerAPIResult(t, api, "case_id=arbd-1&role_id=plaintiff")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "done" {
		t.Fatalf("status field = %#v, want done", got["status"])
	}
	result, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", got["result"])
	}
	answers, ok := result["answers"].(map[string]any)
	if !ok || intNumber(answers["C1"]) != 72 || intNumber(answers["C2"]) != 28 {
		t.Fatalf("answers = %#v", result["answers"])
	}
	councilAnswers := mapList(result["council_answers"])
	if len(councilAnswers) != 2 {
		t.Fatalf("council_answers = %#v, want 2", result["council_answers"])
	}
	if councilAnswers[0]["member_id"] != "C1" || intNumber(councilAnswers[0]["answer"]) != 72 || councilAnswers[0]["rationale"] != "official record proves control" {
		t.Fatalf("first council answer = %#v", councilAnswers[0])
	}
	summary, ok := result["answer_summary"].(map[string]any)
	if !ok {
		t.Fatalf("answer_summary = %#v, want object", result["answer_summary"])
	}
	final := mapAny(summary["final"])
	if intNumber(final["count"]) != 2 || intNumber(final["min"]) != 28 || intNumber(final["max"]) != 72 || final["mean"] != float64(50) {
		t.Fatalf("answer_summary = %#v", summary)
	}
}

func TestLawyerWaitTimeout(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIWait(t, api, "case_id=arbd-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=10")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "timeout" {
		t.Fatalf("wait.reason = %#v, want timeout", wait["reason"])
	}
}

func testLawyerAPIWithTurn() (*lawyerAPIServer, *lawyerTurn) {
	policy := DefaultPolicy()
	turn := testLawyerTurn("openings:plaintiff", "plaintiff", "openings")
	rc := &runContext{
		cfg: Config{
			Policy:  policy,
			Runtime: DefaultRuntimeLimits(),
		},
		state:          initialState(policy, "", nil),
		fileByID:       map[string]CaseFile{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	return &lawyerAPIServer{rc: rc, active: turn, version: 1}, turn
}

func testLawyerTurn(id string, role string, phase string) *lawyerTurn {
	return &lawyerTurn{
		opportunity: Opportunity{
			ID:           id,
			Role:         role,
			Phase:        phase,
			AllowedTools: []string{"record_opening_statement"},
		},
		turnNumber:        1,
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       3,
		attemptsRemaining: 3,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
}

func callLawyerAPIDo(t *testing.T, api *lawyerAPIServer, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return callLawyerAPIDoRaw(t, api, string(raw))
}

func callLawyerAPIDoRaw(t *testing.T, api *lawyerAPIServer, raw string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.handleDo(context.Background(), rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func callLawyerAPIWait(t *testing.T, api *lawyerAPIServer, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, lawyerAPIBasePath+"/wait?"+query, nil)
	rec := httptest.NewRecorder()
	api.handleWait(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func callLawyerAPIGet(t *testing.T, api *lawyerAPIServer, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, lawyerAPIBasePath+"/get?"+query, nil)
	rec := httptest.NewRecorder()
	api.handleGet(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func callLawyerAPIStatus(t *testing.T, api *lawyerAPIServer, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, lawyerAPIBasePath+"/status?"+query, nil)
	rec := httptest.NewRecorder()
	api.handleStatus(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func callLawyerAPIResult(t *testing.T, api *lawyerAPIServer, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, lawyerAPIBasePath+"/result?"+query, nil)
	rec := httptest.NewRecorder()
	api.handleResult(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func hasLawyerTool(tools []map[string]any, name string) bool {
	for _, tool := range tools {
		if mapString(tool["name"]) == name {
			return true
		}
	}
	return false
}

func lawyerAPIErrorCode(t *testing.T, got map[string]any) string {
	t.Helper()
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want object", got["error"])
	}
	return mapString(errObj["code"])
}
