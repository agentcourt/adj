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

	"github.com/jsmorph/adj/arb/runtime/proceeding"
	openaiapi "github.com/jsmorph/adj/common/openai"
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

func TestCaseSummariesCarryProviderFailureData(t *testing.T) {
	result := proceeding.Result{
		Status:     "ok",
		Resolution: "no_majority",
		ErrorClass: "provider_transient",
		Provider: openaiapi.Accounting{
			RequestCount:      1,
			CostObservedCount: 1,
			CostUSD:           float64Pointer(0.025),
		},
	}
	summary := buildCaseSuccessSummary(result, "out")
	if summary.ErrorClass != "provider_transient" || summary.Provider.CostUSD == nil || *summary.Provider.CostUSD != 0.025 {
		t.Fatalf("success summary = %+v", summary)
	}

	classes := []openaiapi.ProviderErrorClass{
		openaiapi.ProviderErrorAuthentication,
		openaiapi.ProviderErrorTransient,
		openaiapi.ProviderErrorRequest,
		openaiapi.ProviderErrorProtocol,
	}
	for _, class := range classes {
		providerErr := &openaiapi.ProviderError{Class: class, Err: errors.New("provider failed")}
		preflightErr := &proceeding.CouncilPreflightError{
			MemberID:                   "C1",
			UnavailableCandidates:      1,
			LastUnavailableModel:       "model",
			LastUnavailablePersonaFile: "persona.md",
			Cause:                      providerErr.Error(),
			Err:                        providerErr,
		}
		summary = buildCaseErrorSummary(preflightErr)
		if summary.ErrorClass != string(class) {
			t.Fatalf("error summary error_class = %q, want %q", summary.ErrorClass, class)
		}
	}
}

func float64Pointer(value float64) *float64 {
	return &value
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
	if err := os.WriteFile(complaintPath, []byte("# Proposition\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runCase(context.Background(), []string{
		"--complaint", complaintPath,
		"--out-dir", filepath.Join(dir, "out"),
		"--engine-timeout-seconds", strconv.FormatInt(proceeding.MaxRuntimeTimeoutSeconds+1, 10),
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

func TestRunCaseAppliesRequiredVotesOverride(t *testing.T) {
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
		"--council-size", "5",
		"--required-votes", "2",
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
	if !strings.Contains(summary.Error, "required_votes_for_decision must be a strict majority") {
		t.Fatalf("summary error = %q, want required-votes validation message", summary.Error)
	}
}

func TestReportedErrorWrapsOriginalError(t *testing.T) {
	base := errors.New("boom")
	err := &reportedError{err: base}
	if !errors.Is(err, base) {
		t.Fatal("reportedError does not unwrap to original error")
	}
}
