package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jsmorph/adj/adc/runtime/store"
	"github.com/jsmorph/adj/common/openai"
)

func TestPersistAgentEventWritesNDJSONAndSQLite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "events.ndjson")
	dbPath := filepath.Join(tmpDir, "run.db")

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()
	if err := st.CreateRun("run-1", "scenario"); err != nil {
		t.Fatalf("create run: %v", err)
	}

	r := &Runner{
		store: st,
		cfg: Config{
			RunID:      "run-1",
			EventsPath: eventsPath,
		},
	}
	payload := map[string]any{
		"tool_call_id": "call-1",
		"title":        "bash",
		"raw_input":    map[string]any{"cmd": "openssl dgst -sha256 -verify key.pem -signature sig msg.txt"},
	}
	if err := r.persistAgentEvent(7, 2, "plaintiff", "agent_tool_call", payload); err != nil {
		t.Fatalf("persistAgentEvent returned error: %v", err)
	}

	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"\"timestamp\":",
		"\"agent_event\":\"agent_tool_call\"",
		"\"step\":-2",
		"openssl dgst -sha256 -verify key.pem -signature sig msg.txt",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("events log missing %q\n%s", needle, text)
		}
	}
}

func TestExportExternalWorkProduct(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "home", "work-product")
	if err := os.MkdirAll(filepath.Join(src, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir work product: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "notes", "timeline.md"), []byte("timeline\n"), 0o644); err != nil {
		t.Fatalf("write work product: %v", err)
	}
	if err := exportExternalWorkProduct(filepath.Join(tmpDir, "out"), map[string]string{"plaintiff": src}); err != nil {
		t.Fatalf("exportExternalWorkProduct returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(tmpDir, "out", "work-product", "plaintiff", "notes", "timeline.md"))
	if err != nil {
		t.Fatalf("read exported work product: %v", err)
	}
	if string(raw) != "timeline\n" {
		t.Fatalf("exported work product = %q", string(raw))
	}
}

func TestWriteEvidenceManifestUsesEmptyArray(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	r := &Runner{
		cfg: Config{OutputPath: filepath.Join(tmpDir, "run.json")},
		state: map[string]any{
			"case": map[string]any{
				"case_files": []any{},
			},
		},
	}

	if err := r.writeEvidenceManifest(); err != nil {
		t.Fatalf("writeEvidenceManifest returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(tmpDir, "evidence-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var got struct {
		Evidence []map[string]any `json:"evidence"`
		Count    int              `json:"evidence_count"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if got.Evidence == nil {
		t.Fatalf("manifest evidence array is nil: %s", string(raw))
	}
	if len(got.Evidence) != 0 || got.Count != 0 {
		t.Fatalf("manifest = %s", string(raw))
	}
}

func TestWriteEvidenceWritesStateArtifact(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	finalState := map[string]any{
		"case": map[string]any{
			"case_id":    "case-1",
			"case_files": []any{},
		},
		"status": "judgment_entered",
	}
	r := &Runner{
		cfg: Config{
			OutputPath: filepath.Join(tmpDir, "run.json"),
			CaseID:     "case-1",
			RunID:      "run-1",
		},
		state: finalState,
		certificateInit: ReplayInitializeRequest{
			State: map[string]any{
				"case": map[string]any{
					"case_id": "case-1",
				},
			},
		},
	}

	if err := r.writeEvidence(Result{Scenario: "scenario-1", FinalState: finalState}); err != nil {
		t.Fatalf("writeEvidence returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("read state artifact: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal state artifact: %v", err)
	}
	if stringOrDefault(got["status"], "") != "judgment_entered" {
		t.Fatalf("state artifact = %s", string(raw))
	}
	caseObj, _ := got["case"].(map[string]any)
	if stringOrDefault(caseObj["case_id"], "") != "case-1" {
		t.Fatalf("state artifact case = %s", string(raw))
	}
	raw, err = os.ReadFile(filepath.Join(tmpDir, ReplayCertificateFileName))
	if err != nil {
		t.Fatalf("read certificate: %v", err)
	}
	var cert ReplayCertificate
	if err := json.Unmarshal(raw, &cert); err != nil {
		t.Fatalf("unmarshal certificate: %v", err)
	}
	if cert.CaseID != "case-1" || cert.RunID != "run-1" || cert.ClaimedFinalStateSHA256 == "" {
		t.Fatalf("certificate = %#v", cert)
	}
}

func TestRefreshProviderAccountingUpdatesRunRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "run.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	if err := st.CreateRun("run-1", "scenario-1"); err != nil {
		t.Fatal(err)
	}
	r := &Runner{store: st, cfg: Config{RunID: "run-1", OutputPath: filepath.Join(dir, "run.json")}}
	result := Result{
		Scenario:   "scenario-1",
		Assertions: []map[string]any{},
		FinalState: map[string]any{"case": map[string]any{"status": "closed"}},
		Provider:   openai.Accounting{RequestCount: 99},
	}
	if err := r.RefreshProviderAccounting(&result); err != nil {
		t.Fatal(err)
	}
	if result.Provider != (openai.Accounting{}) {
		t.Fatalf("provider accounting = %#v", result.Provider)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded Result
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded.Provider != (openai.Accounting{}) {
		t.Fatalf("recorded provider accounting = %#v", recorded.Provider)
	}
}

func TestWriteEvidenceManifestCopiesCaseFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(src, []byte("case file text\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	r := &Runner{
		cfg: Config{OutputPath: filepath.Join(tmpDir, "out", "run.json")},
		state: map[string]any{
			"case": map[string]any{
				"case_files": []any{
					map[string]any{
						"file_id":         "file-0001",
						"label":           "Record",
						"original_name":   "record.txt",
						"storage_relpath": src,
						"sha256":          "sha-test",
						"size_bytes":      15,
					},
				},
				"file_events": []any{
					map[string]any{
						"action":  "offer_case_file_as_exhibit",
						"file_id": "file-0001",
						"actor":   "plaintiff",
					},
				},
			},
		},
	}

	if err := r.writeEvidenceManifest(); err != nil {
		t.Fatalf("writeEvidenceManifest returned error: %v", err)
	}
	copied, err := os.ReadFile(filepath.Join(tmpDir, "out", "submitted-evidence", "file-0001-record.txt"))
	if err != nil {
		t.Fatalf("read copied evidence: %v", err)
	}
	if string(copied) != "case file text\n" {
		t.Fatalf("copied evidence = %q", string(copied))
	}
	raw, err := os.ReadFile(filepath.Join(tmpDir, "out", "evidence-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var got struct {
		Evidence []map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if len(got.Evidence) != 1 {
		t.Fatalf("evidence count = %d manifest=%s", len(got.Evidence), string(raw))
	}
	item := got.Evidence[0]
	if item["evidence_id"] != "file-0001" || item["name"] != "file-0001-record.txt" || item["mime_type"] != "text/plain; charset=utf-8" {
		t.Fatalf("manifest item = %#v", item)
	}
	if uses, ok := item["uses"].([]any); !ok || len(uses) != 1 {
		t.Fatalf("uses = %#v", item["uses"])
	}
}

func TestPersistAgentCompletionResultWritesNDJSONAndSQLite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "events.ndjson")
	dbPath := filepath.Join(tmpDir, "run.db")

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()
	if err := st.CreateRun("run-1", "scenario"); err != nil {
		t.Fatalf("create run: %v", err)
	}

	r := &Runner{
		store: st,
		cfg: Config{
			RunID:      "run-1",
			EventsPath: eventsPath,
		},
	}
	payload := map[string]any{
		"model":            "openrouter://google/gemini-2.5-flash",
		"opportunity_id":   "opp-24",
		"status":           "rejected",
		"invalid_attempt":  2,
		"response_text":    "I think I can be fair.",
		"rejection_reason": "Choose one allowed action or use a reference tool now.",
	}
	if err := r.persistAgentCompletionResult(24, 3, "juror", payload); err != nil {
		t.Fatalf("persistAgentCompletionResult returned error: %v", err)
	}

	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"\"timestamp\":",
		"\"agent_event\":\"agent_completion_result\"",
		"\"turn\":24",
		"\"role\":\"juror\"",
		"\"response_text\":\"I think I can be fair.\"",
		"Choose one allowed action or use a reference tool now.",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("events log missing %q\n%s", needle, text)
		}
	}
}
