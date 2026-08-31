package caserecord

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testStart = time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)

func TestBuildSimpleDirectRecord(t *testing.T) {
	dir := t.TempDir()
	writeTestManifest(t, dir, "simple")
	writeTestFile(t, filepath.Join(dir, "documents", "record.txt"), "evidence")
	writeTestJSON(t, filepath.Join(dir, "documents.json"), map[string]any{
		"files": []any{map[string]any{"path": "record.txt", "sha256": "abc", "media_type": "text/plain"}},
	})
	writeTestJSON(t, filepath.Join(dir, "run.json"), map[string]any{
		"finished_at": testStart.Add(time.Minute),
		"decision":    map[string]any{"value": "demonstrated"},
	})

	record, err := Build(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if record.Procedure != "simple" || len(record.Docket) != 2 {
		t.Fatalf("record identity or docket = %#v", record)
	}
	if record.Docket[0].Kind != "evidence" || record.Docket[0].TimestampSource != "case_manifest" || record.Docket[1].Kind != "decision" || record.Docket[1].TimestampSource != "record" {
		t.Fatalf("docket = %#v", record.Docket)
	}
	artifact := findArtifact(t, record, "record", "documents/record.txt")
	if artifact.RecordedSHA256 != "abc" || artifact.Timestamp.IsZero() {
		t.Fatalf("document artifact = %#v", artifact)
	}
}

func TestBuildQuickUnifiedPrivacyOptions(t *testing.T) {
	dir := t.TempDir()
	core := filepath.Join(dir, "core")
	writeTestManifest(t, core, "quick")
	writeTestJSON(t, filepath.Join(core, "documents.json"), map[string]any{"files": []any{}})
	writeTestJSON(t, filepath.Join(core, "run.json"), map[string]any{
		"arguments": []any{
			map[string]any{"role": "plaintiff", "submitted_at": testStart.Add(time.Minute)},
			map[string]any{"role": "defendant", "submitted_at": testStart.Add(2 * time.Minute)},
		},
		"votes":       []any{map[string]any{"member_id": "C1", "vote": "demonstrated", "submitted_at": testStart.Add(3 * time.Minute)}},
		"resolution":  "demonstrated",
		"finished_at": testStart.Add(4 * time.Minute),
	})
	writeTestNDJSON(t, filepath.Join(core, "work-notes.jsonl"), map[string]any{
		"timestamp": testStart.Add(90 * time.Second), "role_id": "plaintiff", "notes": "private",
	})
	writeTestFile(t, filepath.Join(dir, "logs", "quick.stdout"), "log")
	writeTestFile(t, filepath.Join(dir, "work", "plaintiff", "notes.md"), "workspace")
	writeTestFile(t, filepath.Join(dir, "agents", "plaintiff", "codex", "session.jsonl"), "session")
	writeTestFile(t, filepath.Join(dir, "agents", "plaintiff", "work", "analysis.md"), "workspace")
	writeTestFile(t, filepath.Join(core, "work-product", "plaintiff", "analysis.md"), "private")

	sessionDir := t.TempDir()
	writeTestFile(t, filepath.Join(sessionDir, "session.jsonl"), "session")
	writeTestJSON(t, filepath.Join(dir, "run.json"), map[string]any{
		"management": map[string]any{"participants": []any{map[string]any{"role": "plaintiff", "state_dir": sessionDir}}},
	})

	public, err := Build(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Build public: %v", err)
	}
	if countKind(public, "decision") != 1 {
		t.Fatalf("public docket lacks decision: %#v", public.Docket)
	}
	if hasArtifact(public, "work-notes.jsonl") || hasCategory(public, "process_log") || hasCategory(public, "participant_session") || hasArtifact(public, "notes.md") || hasArtifact(public, "analysis.md") {
		t.Fatalf("public artifacts include private records: %#v", public.Artifacts)
	}

	private, err := Build(Options{Dir: dir, IncludeWorkNotes: true, IncludeSessions: true})
	if err != nil {
		t.Fatalf("Build private: %v", err)
	}
	if countKind(private, "work_note") != 1 || !hasCategory(private, "process_log") || !hasCategory(private, "participant_session") {
		t.Fatalf("private record lacks requested records: %#v", private)
	}
	if hasArtifact(private, "notes.md") || hasArtifact(private, "analysis.md") {
		t.Fatalf("private record includes work directories: %#v", private.Artifacts)
	}
}

func TestBuildArbitrationDockets(t *testing.T) {
	for _, test := range []struct {
		procedure string
		field     string
		eventType string
		value     any
		kind      string
	}{
		{procedure: "arb", field: "council_votes", eventType: "council_vote", value: "demonstrated", kind: "council_vote"},
		{procedure: "arbd", field: "council_answers", eventType: "council_answer", value: 72, kind: "council_answer"},
	} {
		t.Run(test.procedure, func(t *testing.T) {
			dir := t.TempDir()
			writeTestManifest(t, dir, test.procedure)
			writeTestFile(t, filepath.Join(dir, "complaint.md"), "complaint")
			writeTestJSON(t, filepath.Join(dir, "evidence-manifest.json"), map[string]any{"evidence": []any{}})
			decisionField := "vote"
			if test.procedure == "arbd" {
				decisionField = "answer"
			}
			writeTestNDJSON(t, filepath.Join(dir, "events.ndjson"),
				map[string]any{
					"timestamp": testStart.Add(time.Minute), "type": "attorney_action", "role": "plaintiff", "phase": "openings",
					"payload": map[string]any{"action_type": "record_opening_statement", "payload": map[string]any{"text": "Opening"}},
				},
				map[string]any{
					"timestamp": testStart.Add(2 * time.Minute), "type": test.eventType, "role": "council", "phase": "deliberation",
					"payload": map[string]any{"member_id": "C1", "payload": map[string]any{decisionField: test.value}},
				},
			)
			caseState := map[string]any{
				"openings":           []any{map[string]any{"role": "plaintiff", "text": "Opening"}},
				"arguments":          []any{},
				"rebuttals":          []any{},
				"surrebuttals":       []any{},
				"closings":           []any{},
				"offered_evidence":   []any{},
				"technical_reports":  []any{},
				"submitted_evidence": []any{},
				test.field:           []any{map[string]any{"member_id": "C1", decisionField: test.value}},
			}
			writeTestJSON(t, filepath.Join(dir, "state.json"), map[string]any{"case": caseState})
			run := map[string]any{"finished_at": testStart.Add(3 * time.Minute)}
			if test.procedure == "arb" {
				run["resolution"] = "demonstrated"
			} else {
				run["answers"] = map[string]any{"C1": test.value}
			}
			writeTestJSON(t, filepath.Join(dir, "run.json"), run)

			record, err := Build(Options{Dir: dir})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if len(record.Docket) != 4 || countKind(record, "opening_statement") != 1 || countKind(record, test.kind) != 1 || countKind(record, "decision") != 1 {
				t.Fatalf("docket = %#v", record.Docket)
			}
			if record.Docket[1].TimestampSource != "event" || record.Docket[2].TimestampSource != "event" {
				t.Fatalf("event timestamps = %#v", record.Docket)
			}
			if record.Docket[3].TimestampSource != "record" {
				t.Fatalf("decision timestamp = %#v", record.Docket[3])
			}
		})
	}
}

func TestBuildFormalLauncherLayouts(t *testing.T) {
	for _, test := range []struct {
		procedure string
		child     string
	}{
		{procedure: "arb", child: "aar-output"},
		{procedure: "arbd", child: "aard-output"},
		{procedure: "adc", child: "adc-output"},
	} {
		t.Run(test.procedure, func(t *testing.T) {
			dir := t.TempDir()
			writeTestManifest(t, filepath.Join(dir, test.child), test.procedure)

			record, err := Build(Options{Dir: dir})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if record.Procedure != test.procedure {
				t.Fatalf("procedure = %q", record.Procedure)
			}
		})
	}
}

func TestBuildADCDocketUsesActionTimes(t *testing.T) {
	dir := t.TempDir()
	writeTestManifest(t, dir, "adc")
	first := map[string]any{"title": "Complaint filed", "description": "Plaintiff filed the complaint."}
	second := map[string]any{"title": "Answer filed", "description": "Defendant answered."}
	writeTestNDJSON(t, filepath.Join(dir, "events.ndjson"),
		map[string]any{
			"timestamp": testStart.Add(time.Minute).In(time.Local).Format("2006-01-02 15:04:05.000"), "turn": 1, "role": "plaintiff", "action": "file_complaint",
			"response": map[string]any{"state": map[string]any{"case": map[string]any{"docket": []any{first}}}},
		},
		map[string]any{
			"timestamp": testStart.Add(2 * time.Minute).In(time.Local).Format("2006-01-02 15:04:05.000"), "turn": 2, "role": "defendant", "action": "file_answer",
			"response": map[string]any{"state": map[string]any{"case": map[string]any{"docket": []any{first, second}}}},
		},
	)
	writeTestJSON(t, filepath.Join(dir, "state.json"), map[string]any{"case": map[string]any{"docket": []any{first, second}}})
	writeTestJSON(t, filepath.Join(dir, "certificate.json"), map[string]any{
		"initialize_request": map[string]any{"initialize_case": map[string]any{"attachments": []any{}}},
	})

	record, err := Build(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(record.Docket) != 2 || record.Docket[0].Title != "Complaint filed" || record.Docket[1].Title != "Answer filed" {
		t.Fatalf("docket = %#v", record.Docket)
	}
	if record.Docket[0].TimestampSource != "case_manifest" || record.Docket[1].TimestampSource != "event" {
		t.Fatalf("ADC timestamp sources = %#v", record.Docket)
	}
	for _, item := range record.Docket {
		if item.Description == "" {
			t.Fatalf("ADC docket item = %#v", item)
		}
	}
}

func writeTestManifest(t *testing.T, dir, procedure string) {
	t.Helper()
	writeTestJSON(t, filepath.Join(dir, "case-manifest.json"), map[string]any{
		"schema_version": "adj.case-manifest.v1",
		"procedure":      procedure,
		"case_id":        procedure + "-case",
		"run_id":         procedure + "-run",
		"started_at":     testStart,
		"core_version":   "test",
	})
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestNDJSON(t *testing.T, path string, values ...map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	var raw []byte
	for _, value := range values {
		line, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", path, err)
		}
		raw = append(raw, line...)
		raw = append(raw, '\n')
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findArtifact(t *testing.T, record Record, sourceID, path string) Artifact {
	t.Helper()
	for _, artifact := range record.Artifacts {
		if artifact.SourceID == sourceID && artifact.Path == path {
			return artifact
		}
	}
	t.Fatalf("artifact %s:%s not found", sourceID, path)
	return Artifact{}
}

func hasArtifact(record Record, base string) bool {
	for _, artifact := range record.Artifacts {
		if filepath.Base(artifact.Path) == base {
			return true
		}
	}
	return false
}

func hasCategory(record Record, category string) bool {
	for _, artifact := range record.Artifacts {
		if artifact.Category == category {
			return true
		}
	}
	return false
}

func countKind(record Record, kind string) int {
	count := 0
	for _, item := range record.Docket {
		if item.Kind == kind {
			count++
		}
	}
	return count
}
