package proceeding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jsmorph/adj/arb/runtime/lean"
)

func (rc *runContext) stepForCertificate(opportunity Opportunity, actionType string, actorRole string, payload map[string]any) (map[string]any, error) {
	stepResp, action, err := rc.evaluateStep(context.Background(), opportunity, actionType, actorRole, payload)
	if err != nil {
		return nil, err
	}
	if ok, _ := stepResp["ok"].(bool); ok {
		if _, _, err := acceptedStepState(stepResp, opportunity.StateVersion); err != nil {
			return nil, err
		}
		rc.certificateActions = append(rc.certificateActions, action)
	}
	return stepResp, nil
}

func TestVerifyReplayCertificateAcceptsMatchingPacket(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	initialState := map[string]any{
		"case":          map[string]any{"phase": "draft"},
		"state_version": 0,
	}
	finalState := certificateTestFinalState()
	hash, err := canonicalJSONSHA256(finalState)
	if err != nil {
		t.Fatalf("hash final state: %v", err)
	}
	cert := ReplayCertificate{
		SchemaVersion: ReplayCertificateSchemaVersion,
		Procedure:     "aar",
		Engine:        []string{enginePath},
		CaseID:        "cert-case",
		RunID:         "run-cert-case",
		InitializeRequest: ReplayInitializeRequest{
			State:          initialState,
			Proposition:    "The proposition is true.",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		Actions: []ReplayAction{{
			ActionType: "record_opening_statement",
			ActorRole:  "plaintiff",
			Authority:  certificateTestAuthority(),
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
	result, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
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
	hash, err := canonicalJSONSHA256(finalState)
	if err != nil {
		t.Fatalf("hash final state: %v", err)
	}
	cert := ReplayCertificate{
		SchemaVersion: ReplayCertificateSchemaVersion,
		Procedure:     "aar",
		CaseID:        "cert-case",
		InitializeRequest: ReplayInitializeRequest{
			State:          map[string]any{"case": map[string]any{"phase": "draft"}},
			Proposition:    "The proposition is true.",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		Actions: []ReplayAction{{
			ActionType: "record_opening_statement",
			ActorRole:  "plaintiff",
			Authority:  certificateTestAuthority(),
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
	if err := writeJSONFile(statePath, map[string]any{"case": map[string]any{"phase": "closed", "resolution": "not_demonstrated"}}); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err = VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "packet final state mismatch") {
		t.Fatalf("error = %v, want packet final state mismatch", err)
	}
}

func TestVerifyReplayCertificateRejectsClaimHashMismatch(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.ClaimedFinalStateSHA256 = "wrong"
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "certificate final state hash mismatch") {
		t.Fatalf("error = %v, want certificate final state hash mismatch", err)
	}
}

func TestVerifyReplayCertificateRejectsMissingAction(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.Actions = nil
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "replayed final state mismatch") {
		t.Fatalf("error = %v, want replayed final state mismatch", err)
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
		Authority:  certificateTestAuthority(),
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
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "certificate action 1 (reject_action) rejected") {
		t.Fatalf("error = %v, want rejected replay action", err)
	}
}

func TestVerifyReplayCertificateRejectsAlteredPayload(t *testing.T) {
	dir := t.TempDir()
	enginePath := writePayloadSensitiveCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.Actions[0].Payload["text"] = "Changed."
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "replayed final state mismatch") {
		t.Fatalf("error = %v, want replayed final state mismatch", err)
	}
}

func TestVerifyReplayCertificateRejectsTamperedAuthority(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeAuthoritySensitiveCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.Actions[0].Authority.OpportunityID = "openings:defendant"
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "authority rejected for test") {
		t.Fatalf("error = %v, want authority rejection", err)
	}
}

func TestVerifyReplayCertificateRejectsWrongCouncilMember(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCouncilAuthorityCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.Actions[0] = ReplayAction{
		ActionType: "submit_council_vote",
		ActorRole:  "council",
		Authority: OpportunityAuthority{
			OpportunityID:        "deliberation:1:C1",
			ExpectedStateVersion: 1,
			Role:                 "council",
			Phase:                "deliberation",
			MemberID:             "C2",
		},
		Payload: map[string]any{"member_id": "C2", "vote": "demonstrated", "rationale": "Reason."},
	}
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "wrong council member") {
		t.Fatalf("error = %v, want wrong council member rejection", err)
	}
}

func TestVerifyReplayCertificateValidatesAuthorityFields(t *testing.T) {
	dir := t.TempDir()
	enginePath := writeCertificateTestEngine(t, dir)
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	cert.Actions[0].Authority.Phase = ""
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "phase is required") {
		t.Fatalf("error = %v, want missing authority phase", err)
	}
}

func TestVerifyReplayCertificateRequiresEngineTimeout(t *testing.T) {
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: "certificate.json",
		StatePath:       "state.json",
		Engine:          lean.New([]string{"engine"}),
	})
	if err == nil || !strings.Contains(err.Error(), "engine timeout must be positive") {
		t.Fatalf("error = %v, want positive engine timeout", err)
	}
}

func TestVerifyReplayCertificateHonorsEngineTimeout(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	if err := os.WriteFile(enginePath, []byte("#!/bin/sh\nsleep 10\n"), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	finalState := certificateTestFinalState()
	cert := certificateTestCertificate(t, enginePath, finalState)
	certPath := filepath.Join(dir, ReplayCertificateFileName)
	statePath := filepath.Join(dir, "state.json")
	if err := writeJSONFile(certPath, cert); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := writeJSONFile(statePath, finalState); err != nil {
		t.Fatalf("write state: %v", err)
	}

	started := time.Now()
	_, err := VerifyReplayCertificate(context.Background(), VerifyReplayCertificateOptions{
		CertificatePath: certPath,
		StatePath:       statePath,
		Engine:          lean.New([]string{enginePath}),
		EngineTimeout:   20 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("certificate replay returned after %s, want at most 1s", elapsed)
	}
}

func TestStepForCertificateRecordsAcceptedStepsOnly(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *reject_me*) printf '%s\n' '{"ok":false,"error":"rejected"}' ;;
  *malformed_accept*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"}}}' ;;
  *) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"},"state_version":2}}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	rc := &runContext{
		cfg: Config{
			Engine:  lean.New([]string{enginePath}),
			Runtime: DefaultRuntimeLimits(),
		},
		state: map[string]any{"case": map[string]any{"phase": "openings"}, "state_version": 1},
	}
	opportunity := Opportunity{ID: "openings:plaintiff", StateVersion: 1, Role: "plaintiff", Phase: "openings"}
	payload := map[string]any{"text": "accepted", "nested": map[string]any{"value": "original"}}
	if _, err := rc.stepForCertificate(opportunity, "record_opening_statement", "plaintiff", payload); err != nil {
		t.Fatalf("accepted step: %v", err)
	}
	payload["text"] = "mutated"
	mapAny(payload["nested"])["value"] = "mutated"
	if _, err := rc.stepForCertificate(opportunity, "reject_me", "plaintiff", map[string]any{}); err != nil {
		t.Fatalf("rejected step transport: %v", err)
	}
	if _, err := rc.stepForCertificate(opportunity, "malformed_accept", "plaintiff", map[string]any{}); err == nil || !strings.Contains(err.Error(), "state_version") {
		t.Fatalf("malformed accepted step error = %v, want state_version", err)
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
	if recorded.Authority != certificateTestAuthority() {
		t.Fatalf("recorded authority = %#v, want %#v", recorded.Authority, certificateTestAuthority())
	}
}

func TestStepForCertificateUsesRefreshedVersionForRepeatedSubmitEvidence(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *\"expected_state_version\":1*\"state_version\":1*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"arguments"},"state_version":2}}' ;;
  *\"expected_state_version\":2*\"state_version\":2*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"arguments"},"state_version":3}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"authority state version mismatch"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	rc := &runContext{
		cfg:   Config{Engine: lean.New([]string{enginePath}), Runtime: DefaultRuntimeLimits()},
		state: map[string]any{"case": map[string]any{"phase": "arguments"}, "state_version": 1},
	}
	opportunity := Opportunity{ID: "arguments:plaintiff", StateVersion: 1, Role: "plaintiff", Phase: "arguments"}
	for _, evidenceID := range []string{"E1", "E2"} {
		stepResp, err := rc.stepForCertificate(opportunity, "submit_evidence", "plaintiff", map[string]any{"evidence_id": evidenceID})
		if err != nil {
			t.Fatalf("submit evidence %s: %v", evidenceID, err)
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			t.Fatalf("submit evidence %s rejected: %v", evidenceID, stepResp["error"])
		}
		rc.state = mapAny(stepResp["state"])
		opportunity.StateVersion++
	}
	if len(rc.certificateActions) != 2 {
		t.Fatalf("recorded actions = %d, want 2", len(rc.certificateActions))
	}
	if rc.certificateActions[0].Authority.ExpectedStateVersion != 1 || rc.certificateActions[1].Authority.ExpectedStateVersion != 2 {
		t.Fatalf("recorded authority versions = %d, %d; want 1, 2", rc.certificateActions[0].Authority.ExpectedStateVersion, rc.certificateActions[1].Authority.ExpectedStateVersion)
	}
}

func TestStepForCertificateRejectsStaleOpportunity(t *testing.T) {
	rc := &runContext{
		cfg:   Config{Runtime: DefaultRuntimeLimits()},
		state: map[string]any{"case": map[string]any{"phase": "arguments"}, "state_version": 2},
	}
	opportunity := Opportunity{ID: "arguments:plaintiff", StateVersion: 1, Role: "plaintiff", Phase: "arguments"}
	_, err := rc.stepForCertificate(opportunity, "submit_evidence", "plaintiff", map[string]any{"evidence_id": "E2"})
	if err == nil || !strings.Contains(err.Error(), "stale opportunity state_version=1 current=2") {
		t.Fatalf("error = %v, want stale opportunity error", err)
	}
	if len(rc.certificateActions) != 0 {
		t.Fatalf("recorded actions = %d, want 0", len(rc.certificateActions))
	}
}

func TestStepForCertificateRequiresCurrentStateVersion(t *testing.T) {
	rc := &runContext{cfg: Config{Runtime: DefaultRuntimeLimits()}, state: map[string]any{"case": map[string]any{"phase": "openings"}}}
	opportunity := Opportunity{ID: "openings:plaintiff", Role: "plaintiff", Phase: "openings"}
	_, err := rc.stepForCertificate(opportunity, "record_opening_statement", "plaintiff", map[string]any{"text": "Opening."})
	if err == nil || !strings.Contains(err.Error(), "state_version is required") {
		t.Fatalf("error = %v, want required state_version", err)
	}
}

func TestNextOpportunityParsesAuthorityFields(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
printf '%s\n' '{"ok":true,"terminal":false,"state_version":7,"opportunity":{"opportunity_id":"deliberation:2:C3","role":"council","phase":"deliberation","member_id":"C3","objective":"vote","allowed_tools":["submit_council_vote"]}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	opportunity, terminal, _, err := nextOpportunity(context.Background(), lean.New([]string{enginePath}), time.Second, map[string]any{"state_version": 7})
	if err != nil {
		t.Fatalf("next opportunity: %v", err)
	}
	if terminal {
		t.Fatal("next opportunity returned terminal")
	}
	if opportunity.ID != "deliberation:2:C3" || opportunity.StateVersion != 7 || opportunity.Role != "council" || opportunity.Phase != "deliberation" || opportunity.MemberID != "C3" {
		t.Fatalf("opportunity = %#v", opportunity)
	}
}

func TestNextOpportunityRequiresStateVersion(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
printf '%s\n' '{"ok":true,"terminal":false,"opportunity":{"opportunity_id":"openings:plaintiff","role":"plaintiff","phase":"openings","objective":"open","allowed_tools":["record_opening_statement"]}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	_, _, _, err := nextOpportunity(context.Background(), lean.New([]string{enginePath}), time.Second, map[string]any{"state_version": 1})
	if err == nil || !strings.Contains(err.Error(), "state_version is required") {
		t.Fatalf("error = %v, want required state_version", err)
	}
}

func TestNextOpportunityHonorsTimeout(t *testing.T) {
	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
sleep 10
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	started := time.Now()
	_, _, _, err := nextOpportunity(context.Background(), lean.New([]string{enginePath}), 20*time.Millisecond, map[string]any{"state_version": 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("next opportunity returned after %s, want at most 1s", elapsed)
	}
}

func TestTurnEngineStepErrorPreservesEarlierEngineTimeout(t *testing.T) {
	caseCtx := context.Background()
	stepCtx, cancel := context.WithCancelCause(caseCtx)
	cancel(context.DeadlineExceeded)
	err := turnEngineStepError(caseCtx, stepCtx, time.Now().Add(-time.Second), errors.New("process cleanup finished late"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want engine deadline", err)
	}
	if errors.Is(err, errTurnDeadlineExceeded) {
		t.Fatalf("error = %v, engine deadline became turn deadline", err)
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
		Procedure:     "aar",
		Engine:        []string{enginePath},
		CaseID:        "cert-case",
		RunID:         "run-cert-case",
		InitializeRequest: ReplayInitializeRequest{
			State:          map[string]any{"case": map[string]any{"phase": "draft"}, "state_version": 0},
			Proposition:    "The proposition is true.",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		Actions: []ReplayAction{{
			ActionType: "record_opening_statement",
			ActorRole:  "plaintiff",
			Authority:  certificateTestAuthority(),
			Payload:    map[string]any{"text": "Opening."},
		}},
		ClaimedFinalState:       finalState,
		ClaimedFinalStateSHA256: hash,
	}
}

func certificateTestAuthority() OpportunityAuthority {
	return OpportunityAuthority{
		OpportunityID:        "openings:plaintiff",
		ExpectedStateVersion: 1,
		Role:                 "plaintiff",
		Phase:                "openings",
	}
}

func certificateTestFinalState() map[string]any {
	return map[string]any{
		"case": map[string]any{
			"phase":      "closed",
			"resolution": "demonstrated",
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
  *record_opening_statement*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","resolution":"demonstrated"},"state_version":2}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"unexpected request"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return enginePath
}

func writePayloadSensitiveCertificateTestEngine(t *testing.T, dir string) string {
	t.Helper()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *initialize_case*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"},"state_version":1}}' ;;
  *Changed*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","resolution":"not_demonstrated"},"state_version":2}}' ;;
  *record_opening_statement*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","resolution":"demonstrated"},"state_version":2}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"unexpected request"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return enginePath
}

func writeAuthoritySensitiveCertificateTestEngine(t *testing.T, dir string) string {
	t.Helper()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *initialize_case*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"openings"},"state_version":1}}' ;;
  *\"opportunity_id\":\"openings:plaintiff\"*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","resolution":"demonstrated"},"state_version":2}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"authority rejected for test"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return enginePath
}

func writeCouncilAuthorityCertificateTestEngine(t *testing.T, dir string) string {
	t.Helper()
	enginePath := filepath.Join(dir, "engine.sh")
	script := `#!/bin/sh
request=$(cat)
case "$request" in
  *initialize_case*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"deliberation"},"state_version":1}}' ;;
  *\"opportunity_id\":\"deliberation:1:C1\"*\"member_id\":\"C1\"*) printf '%s\n' '{"ok":true,"state":{"case":{"phase":"closed","resolution":"demonstrated"},"state_version":2}}' ;;
  *) printf '%s\n' '{"ok":false,"error":"wrong council member"}' ;;
esac
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return enginePath
}
