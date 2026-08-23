package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/lean"
)

func TestStepForCertificateRecordsOnlyAcceptedSteps(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	requestPath := filepath.Join(tmpDir, "request.json")
	enginePath := filepath.Join(tmpDir, "engine.sh")
	script := `#!/bin/sh
cat > "$1"
if grep -q reject_me "$1"; then
  printf '%s\n' '{"ok":false,"error":"rejected"}'
else
  printf '%s\n' '{"ok":true,"state":{"case":{"case_id":"case-1","status":"next"}}}'
fi
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	r := &Runner{
		lean:  lean.New([]string{enginePath, requestPath}),
		state: map[string]any{"case": map[string]any{"case_id": "case-1"}},
	}

	if _, err := r.stepForCertificate("file_answer", "defendant", map[string]any{"summary": "answer"}); err != nil {
		t.Fatalf("accepted step returned error: %v", err)
	}
	if _, err := r.stepForCertificate("reject_me", "defendant", map[string]any{}); err != nil {
		t.Fatalf("rejected step returned process error: %v", err)
	}
	if len(r.certificateTransitions) != 1 {
		t.Fatalf("recorded transitions = %d, want 1", len(r.certificateTransitions))
	}
	transition := r.certificateTransitions[0]
	if transition.Kind != "step" || transition.Step == nil || transition.Step.ActionType != "file_answer" {
		t.Fatalf("transition = %#v", transition)
	}
}

func TestVerifyReplayCertificateReplaysStep(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	finalState := map[string]any{
		"case": map[string]any{
			"case_id": "case-1",
			"status":  "answered",
		},
	}
	hash, err := canonicalJSONSHA256(finalState)
	if err != nil {
		t.Fatalf("hash final state: %v", err)
	}
	cert := ReplayCertificate{
		SchemaVersion: ReplayCertificateSchemaVersion,
		Procedure:     "adc",
		CaseID:        "case-1",
		RunID:         "run-1",
		InitializeRequest: ReplayInitializeRequest{
			State: map[string]any{
				"case": map[string]any{
					"case_id": "case-1",
					"status":  "filed",
				},
			},
		},
		Transitions: []ReplayTransition{{
			Kind: "step",
			Step: &ReplayStepTransition{
				ActionType: "file_answer",
				ActorRole:  "defendant",
				Payload:    map[string]any{"summary": "answer"},
			},
		}},
		ClaimedFinalState:       finalState,
		ClaimedFinalStateSHA256: hash,
	}
	certPath := filepath.Join(tmpDir, ReplayCertificateFileName)
	statePath := filepath.Join(tmpDir, "state.json")
	writeCertificateTestJSON(t, certPath, cert)
	writeCertificateTestJSON(t, statePath, finalState)

	enginePath := filepath.Join(tmpDir, "engine.sh")
	script := `#!/bin/sh
cat >/dev/null
printf '%s\n' '{"ok":true,"state":{"case":{"case_id":"case-1","status":"answered"}}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	result, err := VerifyReplayCertificate(VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
	})
	if err != nil {
		t.Fatalf("VerifyReplayCertificate returned error: %v", err)
	}
	if result.Status != "ok" || result.CaseID != "case-1" || result.RunID != "run-1" || result.TransitionCount != 1 {
		t.Fatalf("verification result = %#v", result)
	}
}

func writeCertificateTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
