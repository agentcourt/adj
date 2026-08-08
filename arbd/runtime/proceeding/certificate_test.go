package proceeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jsmorph/adj/arbd/runtime/lean"
)

func TestVerifyReplayCertificateAcceptsMatchingPacket(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	hash, err := canonicalJSONSHA256(finalState)
	if err != nil {
		t.Fatalf("hash final state: %v", err)
	}
	cert := ReplayCertificate{
		SchemaVersion: ReplayCertificateSchemaVersion,
		Procedure:     "aard",
		Engine:        []string{enginePath},
		CaseID:        "cert-case",
		RunID:         "run-cert-case",
		InitializeRequest: ReplayInitializeRequest{
			State:          map[string]any{"case": map[string]any{"phase": "draft"}, "state_version": 0},
			Question:       "What degree is supported?",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		Actions: []ReplayAction{{
			ActionType: "record_opening_statement",
			ActorRole:  "plaintiff",
			Payload:    map[string]any{"text": "Opening."},
		}},
		ClaimedFinalState:       finalState,
		ClaimedFinalStateSHA256: hash,
	}
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	result, err := VerifyReplayCertificate(VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
	})
	if err != nil {
		t.Fatalf("verify certificate: %v", err)
	}
	if result.Status != "ok" || result.CaseID != "cert-case" || result.ActionCount != 1 {
		t.Fatalf("unexpected verification result: %#v", result)
	}
}

func TestVerifyReplayCertificateRejectsPacketStateMismatch(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, map[string]any{"case": map[string]any{"phase": "closed", "answers": map[string]any{"C1": 3}}, "state_version": 2}); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
	})
	if err == nil || !strings.Contains(err.Error(), "packet final state mismatch") {
		t.Fatalf("error = %v, want packet final state mismatch", err)
	}
}

func TestVerifyReplayCertificateRejectsReplayAction(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.Actions = []ReplayAction{{
		ActionType: "reject_action",
		ActorRole:  "plaintiff",
		Payload:    map[string]any{"text": "Opening."},
	}}
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
	})
	if err == nil || !strings.Contains(err.Error(), "certificate action 1 (reject_action) rejected") {
		t.Fatalf("error = %v, want rejected replay action", err)
	}
}

func TestStepForCertificateRecordsAcceptedStepsOnly(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *reject_me*) printf '%s\n' '{"ok":false,"error":"rejected"}' ;;
  *) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"},"state_version":1}}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	rc := &runContext{
		cfg: Config{
			Engine: lean.New([]string{enginePath}),
		},
		state: map[string]any{"case": map[string]any{"phase": "draft"}},
	}
	payload := map[string]any{"text": "accepted", "nested": map[string]any{"value": "original"}}
	if _, err := rc.stepForCertificate("record_opening_statement", "plaintiff", payload); err != nil {
		t.Fatalf("accepted step: %v", err)
	}
	payload["text"] = "mutated"
	mapAny(payload["nested"])["value"] = "mutated"
	if _, err := rc.stepForCertificate("reject_me", "plaintiff", map[string]any{}); err != nil {
		t.Fatalf("rejected step transport: %v", err)
	}
	if len(rc.certificateActions) != 1 {
		t.Fatalf("recorded actions = %d, want 1", len(rc.certificateActions))
	}
	recorded := rc.certificateActions[0]
	if recorded.ActionType != "record_opening_statement" || mapString(recorded.Payload["text"]) != "accepted" {
		t.Fatalf("recorded action = %#v", recorded)
	}
	if mapString(mapAny(recorded.Payload["nested"])["value"]) != "original" {
		t.Fatalf("recorded payload was not cloned: %#v", recorded.Payload)
	}
}

func certificateTestCertificate(t *testing.T, enginePath string, finalState map[string]any) ReplayCertificate {
	t.Helper()
	hash, err := canonicalJSONSHA256(finalState)
	if err != nil {
		t.Fatalf("hash final state: %v", err)
	}
	return ReplayCertificate{
		SchemaVersion: ReplayCertificateSchemaVersion,
		Procedure:     "aard",
		Engine:        []string{enginePath},
		CaseID:        "cert-case",
		RunID:         "run-cert-case",
		InitializeRequest: ReplayInitializeRequest{
			State:          map[string]any{"case": map[string]any{"phase": "draft"}, "state_version": 0},
			Question:       "What degree is supported?",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		Actions: []ReplayAction{{
			ActionType: "record_opening_statement",
			ActorRole:  "plaintiff",
			Payload:    map[string]any{"text": "Opening."},
		}},
		ClaimedFinalState:       finalState,
		ClaimedFinalStateSHA256: hash,
	}
}

func certificateTestFinalState() map[string]any {
	return map[string]any{
		"case": map[string]any{
			"phase":   "closed",
			"answers": map[string]any{"C1": 72},
		},
		"state_version": 2,
	}
}

func writeCertificateTestEngine(t *testing.T, dir string) string {
	t.Helper()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *initialize_case*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"},"state_version":1}}' ;;
  *reject_action*) printf '%s\n' '{"ok":false,"error":"rejected for test"}' ;;
  *record_opening_statement*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","answers":{"C1":72}},"state_version":2}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"unexpected request"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return enginePath
}
