package proceeding

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/agentcourt/adj/arbd/runtime/spec"
)

func TestWriteCouncilTurnSnapshotPreservesAARDDomain(t *testing.T) {
	dir := t.TempDir()
	policy := DefaultPolicy()
	policy.JudgmentStandard = "Assess the degree supported by the record."
	state := initialState(policy, "arbd-snapshot", nil)
	state["state_version"] = 4
	caseObj := mapAny(state["case"])
	caseObj["phase"] = "deliberation"
	caseObj["council_answers"] = []map[string]any{{
		"round": 1, "member_id": "C2", "answer": 61, "rationale": "Prior answer.",
	}}
	rc := &runContext{
		cfg: Config{
			CaseID: "arbd-snapshot", RunID: "run-snapshot", OutputDir: dir,
			Policy: policy, Runtime: DefaultRuntimeLimits(),
		},
		complaint: spec.Complaint{Question: "What degree does the record support?"},
		state:     state,
		evidence: []EvidenceMeta{
			{EvidenceID: "ev_visible", SHA256: "abc", SizeBytes: 3, RecordVisibility: "council_visible", AdmissibilityStatus: "case_packet"},
			{EvidenceID: "ev_private", SHA256: "def", SizeBytes: 3, RecordVisibility: "system_private", AdmissibilityStatus: "work_product"},
		},
	}
	turn := &councilTurn{
		opportunity: Opportunity{
			ID: "deliberation:2:C1", StateVersion: 4, Role: "council", Phase: "deliberation", MemberID: "C1",
			Objective: "answer", AllowedTools: []string{"submit_council_answer"},
		},
		seat:       CouncilSeat{MemberID: "C1", Model: "model", PersonaFile: "personas/c1.txt"},
		turnNumber: 2, deadline: time.Now().Add(time.Minute), attemptsMax: 3, attemptsRemaining: 2,
		evidenceBudget: &evidenceReadBudget{},
	}
	const prompt = "Council prompt."
	rc.mu.Lock()
	err := rc.writeCouncilTurnSnapshot(turn, prompt)
	rc.mu.Unlock()
	if err != nil {
		t.Fatalf("writeCouncilTurnSnapshot returned error: %v", err)
	}

	path := filepath.Join(dir, "council-turns", "turn-000002-C1", "input.json")
	var snapshot councilTurnSnapshot
	if err := readJSON(path, &snapshot); err != nil {
		t.Fatalf("read council snapshot: %v", err)
	}
	if snapshot.SchemaVersion != "aard.council-turn-snapshot.v0" || snapshot.CaseID != "arbd-snapshot" || snapshot.MemberID != "C1" {
		t.Fatalf("snapshot identity = %#v", snapshot)
	}
	if snapshot.Complaint.Question != rc.complaint.Question || snapshot.Policy.JudgmentStandard != policy.JudgmentStandard {
		t.Fatalf("snapshot question or judgment standard = %#v, %#v", snapshot.Complaint, snapshot.Policy)
	}
	if snapshot.CaseView["question"] != rc.complaint.Question || snapshot.CaseView["judgment_standard"] != policy.JudgmentStandard {
		t.Fatalf("case view = %#v", snapshot.CaseView)
	}
	priorAnswers := mapList(mapAny(snapshot.CaseView["record"])["prior_council_answers"])
	if len(priorAnswers) != 1 || intNumber(priorAnswers[0]["answer"]) != 61 {
		t.Fatalf("prior council answers = %#v", priorAnswers)
	}
	if len(snapshot.Evidence) != 1 || snapshot.Evidence[0].EvidenceID != "ev_visible" {
		t.Fatalf("visible evidence = %#v", snapshot.Evidence)
	}
	manifestEvidence := mapList(snapshot.EvidenceManifest["evidence"])
	if len(manifestEvidence) != 2 || snapshot.EvidenceManifest["schema_version"] != "aar.evidence-manifest.v0" {
		t.Fatalf("evidence manifest = %#v", snapshot.EvidenceManifest)
	}
}
