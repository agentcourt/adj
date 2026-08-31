package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/common/caserecord"
)

func TestRunCaseRecordWritesJSONOutsideCaseDirectory(t *testing.T) {
	root := t.TempDir()
	recordDir := filepath.Join(root, "record")
	writeCommandTestJSON(t, filepath.Join(recordDir, "case-manifest.json"), map[string]any{
		"schema_version": "adj.case-manifest.v1",
		"procedure":      "simple",
		"case_id":        "case-1",
		"run_id":         "run-1",
		"started_at":     time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
		"core_version":   "test",
	})
	writeCommandTestJSON(t, filepath.Join(recordDir, "run.json"), map[string]any{
		"finished_at": "2026-08-31T12:01:00Z",
		"decision":    map[string]any{"value": "demonstrated"},
	})

	var stdout bytes.Buffer
	if err := runCaseRecord([]string{"--dir", recordDir}, &stdout, io.Discard); err != nil {
		t.Fatalf("runCaseRecord stdout: %v", err)
	}
	var record caserecord.Record
	if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}
	if record.Procedure != "simple" || len(record.Docket) != 1 {
		t.Fatalf("stdout record = %#v", record)
	}

	output := filepath.Join(root, "listing.json")
	if err := runCaseRecord([]string{"--dir", recordDir, "--output", output}, io.Discard, io.Discard); err != nil {
		t.Fatalf("runCaseRecord file: %v", err)
	}
	if info, err := os.Stat(output); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("output file: info=%v error=%v", info, err)
	}
	if err := runCaseRecord([]string{"--dir", recordDir, "--output", output}, io.Discard, io.Discard); err == nil {
		t.Fatal("existing output was replaced")
	}

	inside := filepath.Join(recordDir, "listing.json")
	err := runCaseRecord([]string{"--dir", recordDir, "--output", inside}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "outside the case record") {
		t.Fatalf("inside output error = %v", err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("inside output was created: %v", err)
	}
}

func TestRunCaseRecordPublishesSelectedArtifacts(t *testing.T) {
	root := t.TempDir()
	recordDir := filepath.Join(root, "record")
	writeCommandTestJSON(t, filepath.Join(recordDir, "case-manifest.json"), map[string]any{
		"schema_version": "adj.case-manifest.v1",
		"procedure":      "simple",
		"case_id":        "case-1",
		"run_id":         "run-1",
		"started_at":     time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
		"core_version":   "test",
	})
	writeCommandTestJSON(t, filepath.Join(recordDir, "run.json"), map[string]any{
		"finished_at": "2026-08-31T12:01:00Z",
		"decision":    map[string]any{"value": "demonstrated"},
	})

	target := filepath.Join(root, "published")
	var stdout bytes.Buffer
	if err := runCaseRecord([]string{"--dir", recordDir, "--publish-dir", target}, &stdout, io.Discard); err != nil {
		t.Fatalf("runCaseRecord publish: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("publish stdout = %q", stdout.String())
	}
	raw, err := os.ReadFile(filepath.Join(target, caserecord.PublishedIndexName))
	if err != nil {
		t.Fatalf("read published index: %v", err)
	}
	var record caserecord.Record
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("decode published index: %v", err)
	}
	if record.Sources[0].PathBase != caserecord.SourcePathBaseIndex {
		t.Fatalf("published path base = %q", record.Sources[0].PathBase)
	}
	if _, err := os.Stat(filepath.Join(target, "files", "record", "run.json")); err != nil {
		t.Fatalf("published run record: %v", err)
	}

	err = runCaseRecord([]string{"--dir", recordDir, "--output", filepath.Join(root, "index.json"), "--publish-dir", filepath.Join(root, "other")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("combined output error = %v", err)
	}
}

func writeCommandTestJSON(t *testing.T, path string, value any) {
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
