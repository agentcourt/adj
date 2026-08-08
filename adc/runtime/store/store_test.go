package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(filepath.Join(t.TempDir(), "run.db"))
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("Close error = %v", err)
		}
	})
	return s
}

func TestStoreLifecyclePersistsRunAndEvents(t *testing.T) {
	t.Parallel()

	s := openTempStore(t)
	if err := s.CreateRun("run-1", "ex1"); err != nil {
		t.Fatalf("CreateRun error = %v", err)
	}
	if err := s.AppendEvent(
		"run-1",
		2,
		1,
		"judge",
		"issue_order",
		map[string]any{"title": "Scheduling order"},
		map[string]any{"ok": true},
	); err != nil {
		t.Fatalf("AppendEvent error = %v", err)
	}
	if err := s.FinishRun(
		"run-1",
		"ok",
		map[string]any{"case": map[string]any{"case_id": "case-1", "caption": "Peter v. Samantha"}},
		map[string]any{"digest": "done"},
	); err != nil {
		t.Fatalf("FinishRun error = %v", err)
	}

	var startedAt, finishedAt, status, finalStateJSON, evidenceJSON string
	err := s.db.QueryRow(
		`SELECT started_at, finished_at, status, final_state_json, evidence_json FROM runs WHERE run_id=?`,
		"run-1",
	).Scan(&startedAt, &finishedAt, &status, &finalStateJSON, &evidenceJSON)
	if err != nil {
		t.Fatalf("QueryRow runs error = %v", err)
	}
	if startedAt == "" || finishedAt == "" {
		t.Fatalf("timestamps = (%q, %q), want both set", startedAt, finishedAt)
	}
	if status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if !strings.Contains(finalStateJSON, "\"case_id\":\"case-1\"") {
		t.Fatalf("final_state_json = %s", finalStateJSON)
	}
	if !strings.Contains(evidenceJSON, "\"digest\":\"done\"") {
		t.Fatalf("evidence_json = %s", evidenceJSON)
	}

	var turnIndex, stepIndex int
	var role, actionType, payloadJSON, responseJSON string
	err = s.db.QueryRow(
		`SELECT turn_index, step_index, role, action_type, payload_json, response_json FROM events WHERE run_id=?`,
		"run-1",
	).Scan(&turnIndex, &stepIndex, &role, &actionType, &payloadJSON, &responseJSON)
	if err != nil {
		t.Fatalf("QueryRow events error = %v", err)
	}
	if turnIndex != 2 || stepIndex != 1 || role != "judge" || actionType != "issue_order" {
		t.Fatalf("event row = (%d, %d, %q, %q)", turnIndex, stepIndex, role, actionType)
	}
	if !strings.Contains(payloadJSON, "Scheduling order") || !strings.Contains(responseJSON, "\"ok\":true") {
		t.Fatalf("event JSON = (%s, %s)", payloadJSON, responseJSON)
	}
}

func TestLoadLatestCaseChoosesNewestCompletedCase(t *testing.T) {
	t.Parallel()

	s := openTempStore(t)
	insertRun := func(runID, scenario, startedAt, finishedAt, finalState string) {
		t.Helper()
		_, err := s.db.Exec(
			`INSERT INTO runs(run_id, scenario_name, started_at, finished_at, status, final_state_json) VALUES(?,?,?,?,?,?)`,
			runID,
			scenario,
			startedAt,
			finishedAt,
			"ok",
			finalState,
		)
		if err != nil {
			t.Fatalf("insert run %s error = %v", runID, err)
		}
	}

	insertRun("bad-json", "ex1", "2026-03-15T10:00:00Z", "2026-03-15T10:10:00Z", "{")
	insertRun("missing-case", "ex1", "2026-03-15T11:00:00Z", "2026-03-15T11:10:00Z", `{"status":"ok"}`)
	insertRun("older", "ex1", "2026-03-15T12:00:00Z", "2026-03-15T12:10:00Z", `{"case":{"case_id":"case-1","caption":"Old","judge":"J","status":"open","trial_mode":"jury","phase":"pretrial"}}`)
	insertRun("newer", "ex2", "2026-03-15T13:00:00Z", "2026-03-15T13:10:00Z", `{"case":{"case_id":"case-2","caption":"New","judge":"Judge Ada","status":"trial","trial_mode":"jury","phase":"evidence"}}`)

	got, err := s.LoadLatestCase("")
	if err != nil {
		t.Fatalf("LoadLatestCase(\"\") error = %v", err)
	}
	if got.RunID != "newer" || got.CaseID != "case-2" || got.Caption != "New" {
		t.Fatalf("LoadLatestCase(\"\") = %+v", got)
	}
	if got.Meta["scenario_name"] != "ex2" {
		t.Fatalf("Meta = %+v", got.Meta)
	}

	filtered, err := s.LoadLatestCase("case-1")
	if err != nil {
		t.Fatalf("LoadLatestCase(case-1) error = %v", err)
	}
	if filtered.RunID != "older" {
		t.Fatalf("LoadLatestCase(case-1) = %+v", filtered)
	}
}

func TestLoadLatestCaseNotFound(t *testing.T) {
	t.Parallel()

	s := openTempStore(t)
	if _, err := s.LoadLatestCase("missing"); err == nil {
		t.Fatalf("LoadLatestCase(missing) error = nil, want error")
	}
}

func TestBuildPacerDocumentsAndFind(t *testing.T) {
	t.Parallel()

	caseObj := map[string]any{
		"docket": []any{
			map[string]any{"title": "Complaint", "description": "Filed by Peter"},
			map[string]any{"title": "Answer", "description": "Filed by Samantha"},
		},
		"filing_documents": []any{
			map[string]any{
				"title":       "Complaint",
				"filing_type": "complaint",
				"filed_at":    "2026-03-15T10:00:00Z",
				"filed_by":    "plaintiff",
				"body":        "Counts and relief",
			},
		},
	}

	docs := BuildPacerDocuments(caseObj)
	if len(docs) != 3 {
		t.Fatalf("len(docs) = %d, want 3", len(docs))
	}
	if docs[0].DocumentID != "docket-0001" || docs[2].DocumentID != "filing-0001" {
		t.Fatalf("documents = %+v", docs)
	}
	if docs[2].Metadata["filed_by"] != "plaintiff" {
		t.Fatalf("filing metadata = %+v", docs[2].Metadata)
	}

	doc, ok := FindPacerDocument(docs, "filing-0001")
	if !ok {
		t.Fatalf("FindPacerDocument filing-0001 = not found")
	}
	if doc.Body != "Counts and relief" {
		t.Fatalf("Body = %q", doc.Body)
	}
	if _, ok := FindPacerDocument(docs, "missing"); ok {
		t.Fatalf("FindPacerDocument missing = found, want not found")
	}
}

func TestFinishRunStoresValidJSON(t *testing.T) {
	t.Parallel()

	s := openTempStore(t)
	if err := s.CreateRun("run-2", "ex1"); err != nil {
		t.Fatalf("CreateRun error = %v", err)
	}
	finalState := map[string]any{"case": map[string]any{"case_id": "case-9"}}
	evidence := map[string]any{"digest": map[string]any{"status": "ok"}}
	if err := s.FinishRun("run-2", "dismissed", finalState, evidence); err != nil {
		t.Fatalf("FinishRun error = %v", err)
	}

	var finalStateJSON, evidenceJSON string
	if err := s.db.QueryRow(`SELECT final_state_json, evidence_json FROM runs WHERE run_id=?`, "run-2").Scan(&finalStateJSON, &evidenceJSON); err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}

	var gotState map[string]any
	if err := json.Unmarshal([]byte(finalStateJSON), &gotState); err != nil {
		t.Fatalf("final_state_json unmarshal error = %v", err)
	}
	var gotEvidence map[string]any
	if err := json.Unmarshal([]byte(evidenceJSON), &gotEvidence); err != nil {
		t.Fatalf("evidence_json unmarshal error = %v", err)
	}
}

var _ = sql.ErrNoRows
