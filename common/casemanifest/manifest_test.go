package casemanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAtomicCreatesAndReplacesManifest(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "case")
	startedAt := time.Date(2026, 8, 14, 12, 30, 0, 123, time.FixedZone("test", -5*60*60))
	manifest := New(ProcedureARB, "case-1", "run-1", startedAt)
	if err := WriteAtomic(dir, manifest); err != nil {
		t.Fatalf("write initial manifest: %v", err)
	}

	manifest.CaseAPIBase = "http://127.0.0.1:12345"
	if err := WriteAtomic(dir, manifest); err != nil {
		t.Fatalf("replace manifest: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var got Manifest
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", got.SchemaVersion, SchemaVersion)
	}
	if got.Procedure != ProcedureARB || got.CaseID != "case-1" || got.RunID != "run-1" {
		t.Fatalf("identity = %#v", got)
	}
	if got.CaseAPIBase != manifest.CaseAPIBase {
		t.Fatalf("case_api_base = %q, want %q", got.CaseAPIBase, manifest.CaseAPIBase)
	}
	if !got.StartedAt.Equal(startedAt) {
		t.Fatalf("started_at = %s, want %s", got.StartedAt, startedAt)
	}
	if got.CoreVersion == "" {
		t.Fatal("core_version is empty")
	}
}

func TestWriteAtomicRejectsInvalidManifest(t *testing.T) {
	t.Parallel()
	manifest := New(ProcedureADC, "", "run-1", time.Now())
	if err := WriteAtomic(t.TempDir(), manifest); err == nil {
		t.Fatal("expected missing case_id error")
	}
}

func TestWriteAtomicAcceptsSimpleProcedure(t *testing.T) {
	t.Parallel()
	manifest := New(ProcedureSimple, "case-simple", "run-simple", time.Now())
	if err := WriteAtomic(t.TempDir(), manifest); err != nil {
		t.Fatalf("write simple manifest: %v", err)
	}
}

func TestWriteAtomicAcceptsQuickProcedure(t *testing.T) {
	t.Parallel()
	manifest := New(ProcedureQuick, "case-quick", "run-quick", time.Now())
	if err := WriteAtomic(t.TempDir(), manifest); err != nil {
		t.Fatalf("write quick manifest: %v", err)
	}
}
