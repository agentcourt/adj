package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalVoteCountsUsesFinalRound(t *testing.T) {
	state := map[string]any{
		"case": map[string]any{
			"deliberation_round": 2,
			"council_votes": []any{
				map[string]any{"round": 1, "vote": "demonstrated"},
				map[string]any{"round": 1, "vote": "not_demonstrated"},
				map[string]any{"round": 2, "vote": "demonstrated"},
				map[string]any{"round": 2, "vote": "demonstrated"},
				map[string]any{"round": 2, "vote": "not_demonstrated"},
			},
		},
	}

	votesFor, votesAgainst := finalVoteCounts(state)
	if votesFor != 2 || votesAgainst != 1 {
		t.Fatalf("finalVoteCounts = (%d, %d), want (2, 1)", votesFor, votesAgainst)
	}
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
	if err := os.WriteFile(complaintPath, []byte("# Proposition\n\nP\n"), 0o644); err != nil {
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

func TestReportedErrorWrapsOriginalError(t *testing.T) {
	base := errors.New("boom")
	err := &reportedError{err: base}
	if !errors.Is(err, base) {
		t.Fatal("reportedError does not unwrap to original error")
	}
}
