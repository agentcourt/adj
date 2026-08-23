package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agentcourt/adj/arb/runtime/proceeding"
)

func TestRunVerifyCertificatePrintsJSON(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeVerifyCertificateTestEngine(t, dir)
	finalState := verifyCertificateTestFinalState()
	cert := verifyCertificateTestCertificate(t, enginePath, finalState)
	if err := writeVerifyCertificateJSON(filepath.Join(dir, proceeding.ReplayCertificateFileName), cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeVerifyCertificateJSON(filepath.Join(dir, "state.json"), finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runVerifyCertificate(context.Background(), []string{"--dir", dir, "--engine", enginePath}, &stdout, &stderr); err != nil {
		t.Fatalf("runVerifyCertificate returned error: %v\nstderr=%s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got["status"] != "ok" || got["case_id"] != "cert-case" || int(got["action_count"].(float64)) != 1 {
		t.Fatalf("verification result = %#v", got)
	}
}

func TestRunVerifyCertificateReportsFailure(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeVerifyCertificateTestEngine(t, dir)
	finalState := verifyCertificateTestFinalState()
	cert := verifyCertificateTestCertificate(t, enginePath, finalState)
	cert.ClaimedFinalStateSHA256 = "wrong"
	if err := writeVerifyCertificateJSON(filepath.Join(dir, proceeding.ReplayCertificateFileName), cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeVerifyCertificateJSON(filepath.Join(dir, "state.json"), finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runVerifyCertificate(context.Background(), []string{"--dir", dir, "--engine", enginePath}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "certificate final state hash mismatch") {
		t.Fatalf("error = %v, want certificate hash mismatch", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunVerifyCertificateRejectsNonpositiveEngineTimeout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runVerifyCertificate(context.Background(), []string{
		"--certificate", "certificate.json",
		"--state", "state.json",
		"--engine-timeout-seconds", "0",
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--engine-timeout-seconds must be positive") {
		t.Fatalf("error = %v, want positive engine timeout", err)
	}
}

func TestRunVerifyCertificateRejectsEngineTimeoutOverflow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runVerifyCertificate(context.Background(), []string{
		"--certificate", "certificate.json",
		"--state", "state.json",
		"--engine-timeout-seconds", strconv.FormatInt(proceeding.MaxRuntimeTimeoutSeconds+1, 10),
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--engine-timeout-seconds must be at most") {
		t.Fatalf("error = %v, want bounded engine timeout", err)
	}
}

func verifyCertificateTestCertificate(t *testing.T, enginePath string, finalState map[string]any) proceeding.ReplayCertificate {
	t.Helper()
	hash, err := compactJSONSHA256(finalState)
	if err != nil {
		t.Fatalf("hash final state: %v", err)
	}
	return proceeding.ReplayCertificate{
		SchemaVersion: proceeding.ReplayCertificateSchemaVersion,
		Procedure:     "aar",
		Engine:        []string{enginePath},
		CaseID:        "cert-case",
		RunID:         "run-cert-case",
		InitializeRequest: proceeding.ReplayInitializeRequest{
			State:          map[string]any{"case": map[string]any{"phase": "draft"}, "state_version": 0},
			Proposition:    "The proposition is true.",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		Actions: []proceeding.ReplayAction{{
			ActionType: "record_opening_statement",
			ActorRole:  "plaintiff",
			Authority: proceeding.OpportunityAuthority{
				OpportunityID:        "openings:plaintiff",
				ExpectedStateVersion: 1,
				Role:                 "plaintiff",
				Phase:                "openings",
			},
			Payload: map[string]any{"text": "Opening."},
		}},
		ClaimedFinalState:       finalState,
		ClaimedFinalStateSHA256: hash,
	}
}

func verifyCertificateTestFinalState() map[string]any {
	return map[string]any{
		"case": map[string]any{
			"phase":      "closed",
			"resolution": "demonstrated",
		},
		"state_version": 2,
	}
}

func compactJSONSHA256(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func writeVerifyCertificateJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func writeVerifyCertificateTestEngine(t *testing.T, dir string) string {
	t.Helper()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *initialize_case*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"},"state_version":1}}' ;;
  *record_opening_statement*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","resolution":"demonstrated"},"state_version":2}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"unexpected request"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return enginePath
}
