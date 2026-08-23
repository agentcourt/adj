package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agentcourt/adj/arbd/runtime/proceeding"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestRunCaseReportsJSONError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runCase(context.Background(), nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("runCase returned nil error, want failure")
	}
	if !isReportedError(err) {
		t.Fatalf("runCase error = %T, want reported error", err)
	}

	var summary caseRunSummary
	if decodeErr := json.Unmarshal(stdout.Bytes(), &summary); decodeErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", decodeErr, stdout.String())
	}
	if summary.Status != "error" {
		t.Fatalf("summary status = %q, want error", summary.Status)
	}
	if !strings.Contains(summary.Error, "--complaint and --out-dir are required") {
		t.Fatalf("summary error = %q, want missing-args message", summary.Error)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCaseRejectsInvalidCouncilBackend(t *testing.T) {
	dir := t.TempDir()
	complaintPath := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaintPath, []byte("# Question\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runCase(context.Background(), []string{
		"--complaint", complaintPath,
		"--out-dir", filepath.Join(dir, "out"),
		"--council-backend", "browser",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runCase returned nil error, want failure")
	}
	if !isReportedError(err) {
		t.Fatalf("runCase error = %T, want reported error", err)
	}
	var summary caseRunSummary
	if decodeErr := json.Unmarshal(stdout.Bytes(), &summary); decodeErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", decodeErr, stdout.String())
	}
	if summary.Status != "error" {
		t.Fatalf("summary status = %q, want error", summary.Status)
	}
	if !strings.Contains(summary.Error, "council backend must be direct or councilapi") {
		t.Fatalf("summary error = %q, want invalid council backend message", summary.Error)
	}
}

func TestRunCaseRejectsNegativeEngineTimeout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runCase(context.Background(), []string{
		"--complaint", "complaint.md",
		"--out-dir", "out",
		"--engine-timeout-seconds", "-1",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runCase returned nil error, want failure")
	}
	if !isReportedError(err) {
		t.Fatalf("runCase error = %T, want reported error", err)
	}
	var summary caseRunSummary
	if decodeErr := json.Unmarshal(stdout.Bytes(), &summary); decodeErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", decodeErr, stdout.String())
	}
	if !strings.Contains(summary.Error, "engine call timeout must be positive when set") {
		t.Fatalf("summary error = %q, want engine timeout validation message", summary.Error)
	}
}

func TestRunCaseRejectsEngineTimeoutOverflow(t *testing.T) {
	dir := t.TempDir()
	complaintPath := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaintPath, []byte("# Question\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runCase(context.Background(), []string{
		"--complaint", complaintPath,
		"--out-dir", filepath.Join(dir, "out"),
		"--engine-timeout-seconds", strconv.FormatInt(proceeding.MaxEngineCallTimeoutSeconds+1, 10),
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runCase returned nil error, want failure")
	}
	if !isReportedError(err) {
		t.Fatalf("runCase error = %T, want reported error", err)
	}
	var summary caseRunSummary
	if decodeErr := json.Unmarshal(stdout.Bytes(), &summary); decodeErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", decodeErr, stdout.String())
	}
	if !strings.Contains(summary.Error, "runtime.engine_call_timeout_seconds must not exceed") {
		t.Fatalf("summary error = %q, want bounded engine timeout message", summary.Error)
	}
}

func TestReportedErrorWrapsOriginalError(t *testing.T) {
	base := errors.New("boom")
	err := &reportedError{err: base}
	if !errors.Is(err, base) {
		t.Fatal("reportedError does not unwrap to original error")
	}
}

func TestReportCaseErrorPreservesOriginalErrorWhenSummaryWriteFails(t *testing.T) {
	caseErr := errors.New("case failed")
	writeErr := errors.New("summary write failed")
	err := reportCaseError(failingWriter{err: writeErr}, caseErr)
	if !errors.Is(err, caseErr) {
		t.Fatal("reportCaseError does not preserve the original case error")
	}
	if !errors.Is(err, writeErr) {
		t.Fatal("reportCaseError does not preserve the summary write error")
	}
}
