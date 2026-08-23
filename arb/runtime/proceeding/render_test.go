package proceeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/arb/runtime/spec"
)

func (rc *runContext) recordEventAtTurn(turn int, eventType string, role string, phase string, payload map[string]any) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.recordEventAtTurnLocked(turn, eventType, role, phase, payload)
}

func TestRenderTranscriptOmitsRawEvents(t *testing.T) {
	result, rc := sampleRenderResult()
	out := renderTranscript(result, rc.fileByID)
	if strings.Contains(out, "## Events") {
		t.Fatalf("transcript still includes raw events:\n%s", out)
	}
	if !strings.Contains(out, "## Proceeding") {
		t.Fatalf("transcript missing proceeding section:\n%s", out)
	}
	if !strings.Contains(out, "## Council Deliberation") {
		t.Fatalf("transcript missing council deliberation:\n%s", out)
	}
	if !strings.Contains(out, "#### Plaintiff Argument") {
		t.Fatalf("transcript missing plaintiff argument heading:\n%s", out)
	}
	if !strings.Contains(out, "Exhibits offered:\n- PX-1: instructions.txt") {
		t.Fatalf("transcript missing inline exhibit index:\n%s", out)
	}
	if !strings.Contains(out, "## Exhibits") || !strings.Contains(out, "instructions body") {
		t.Fatalf("transcript missing exhibit appendix:\n%s", out)
	}
}

func TestRenderDigestUsesExhibitIndex(t *testing.T) {
	result, rc := sampleRenderResult()
	out := renderDigest(result, rc.fileByID)
	if strings.Contains(out, "instructions body") {
		t.Fatalf("digest should not inline exhibit body:\n%s", out)
	}
	if !strings.Contains(out, "[plaintiff arguments] PX-1: instructions.txt") {
		t.Fatalf("digest missing exhibit index entry:\n%s", out)
	}
	if !strings.Contains(out, "Tally: 2 demonstrated, 1 not_demonstrated") {
		t.Fatalf("digest missing vote tally:\n%s", out)
	}
}

func TestExportAttorneyWorkProduct(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "plaintiff-work")
	if err := os.MkdirAll(filepath.Join(src, "notes"), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "notes", "timeline.md"), []byte("line one\n"), 0o644); err != nil {
		t.Fatalf("write source note: %v", err)
	}
	workProductDirs := map[string]string{
		"plaintiff": src,
	}
	if err := exportAttorneyWorkProduct(filepath.Join(dir, "out"), workProductDirs); err != nil {
		t.Fatalf("exportAttorneyWorkProduct returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "out", "work-product", "plaintiff", "notes", "timeline.md"))
	if err != nil {
		t.Fatalf("read exported note: %v", err)
	}
	if string(raw) != "line one\n" {
		t.Fatalf("exported note = %q, want %q", string(raw), "line one\n")
	}
}

func TestRecordEventUsesUTCTimestamp(t *testing.T) {
	rc := &runContext{
		cfg: Config{OutputDir: t.TempDir()},
	}
	if err := rc.recordEventAtTurn(1, "test_event", "system", "openings", nil); err != nil {
		t.Fatalf("recordEventAtTurn returned error: %v", err)
	}
	if len(rc.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(rc.events))
	}
	timestamp := rc.events[0].Timestamp
	if !strings.HasSuffix(timestamp, "Z") {
		t.Fatalf("timestamp = %q, want UTC suffix", timestamp)
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z07:00", timestamp); err != nil {
		t.Fatalf("timestamp = %q, want millisecond UTC timestamp: %v", timestamp, err)
	}
}

func TestFinalOutputSnapshotOwnsMutableData(t *testing.T) {
	rc := &runContext{
		evidence: []EvidenceMeta{{EvidenceID: "ev_1", Title: "Original evidence"}},
		fileByID: map[string]CaseFile{
			"ev_1": {EvidenceID: "ev_1", Text: "original file"},
		},
		workProductDirs: map[string]string{"plaintiff": "original-work"},
		certificateInit: ReplayInitializeRequest{
			State:          map[string]any{"case": map[string]any{"phase": "openings"}},
			Proposition:    "P",
			CouncilMembers: []map[string]any{{"member_id": "C1"}},
		},
		certificateActions: []ReplayAction{{
			ActionType: "record_opening_statement",
			Payload:    map[string]any{"nested": map[string]any{"text": "original action"}},
		}},
	}
	rc.mu.Lock()
	snapshot, err := rc.finalOutputSnapshotLocked()
	rc.mu.Unlock()
	if err != nil {
		t.Fatalf("final output snapshot: %v", err)
	}
	rc.evidence[0].Title = "changed evidence"
	rc.fileByID["ev_1"] = CaseFile{EvidenceID: "ev_1", Text: "changed file"}
	rc.workProductDirs["plaintiff"] = "changed-work"
	mapAny(rc.certificateInit.State["case"])["phase"] = "changed"
	rc.certificateInit.CouncilMembers[0]["member_id"] = "changed"
	mapAny(rc.certificateActions[0].Payload["nested"])["text"] = "changed action"

	if snapshot.evidence[0].Title != "Original evidence" || snapshot.fileByID["ev_1"].Text != "original file" {
		t.Fatalf("record snapshot changed: evidence=%#v file=%#v", snapshot.evidence, snapshot.fileByID)
	}
	if snapshot.workProductDirs["plaintiff"] != "original-work" {
		t.Fatalf("work-product snapshot = %#v", snapshot.workProductDirs)
	}
	if mapString(mapAny(snapshot.certificateInit.State["case"])["phase"]) != "openings" ||
		mapString(snapshot.certificateInit.CouncilMembers[0]["member_id"]) != "C1" ||
		mapString(mapAny(snapshot.certificateActions[0].Payload["nested"])["text"]) != "original action" {
		t.Fatalf("certificate snapshot changed: init=%#v actions=%#v", snapshot.certificateInit, snapshot.certificateActions)
	}
}

func sampleRenderResult() (Result, *runContext) {
	rc := &runContext{
		fileByID: map[string]CaseFile{
			"instructions.txt": {
				EvidenceID:   "instructions.txt",
				Name:         "instructions.txt",
				TextReadable: true,
				Text:         "instructions body",
			},
		},
	}
	result := Result{
		Complaint:        spec.Complaint{Proposition: "P"},
		EvidenceStandard: "Preponderance of the evidence.",
		Council: []CouncilSeat{
			{MemberID: "C1", Model: "m1", PersonaFile: "p1"},
			{MemberID: "C2", Model: "m2", PersonaFile: "p2"},
			{MemberID: "C3", Model: "m3", PersonaFile: "p3"},
		},
		FinalState: map[string]any{
			"case": map[string]any{
				"resolution": "demonstrated",
				"phase":      "closed",
				"openings": []any{
					map[string]any{"role": "plaintiff", "text": "opening"},
				},
				"arguments": []any{
					map[string]any{"phase": "arguments", "role": "plaintiff", "text": "argument"},
				},
				"rebuttals":        []any{},
				"surrebuttals":     []any{},
				"closings":         []any{},
				"offered_evidence": []any{map[string]any{"phase": "arguments", "role": "plaintiff", "evidence_id": "instructions.txt", "label": "PX-1"}},
				"technical_reports": []any{
					map[string]any{"phase": "arguments", "role": "plaintiff", "title": "Verification", "summary": "Verified OK."},
				},
				"council_votes": []any{
					map[string]any{"round": 1, "member_id": "C1", "vote": "demonstrated", "rationale": "R1"},
					map[string]any{"round": 1, "member_id": "C2", "vote": "demonstrated", "rationale": "R2"},
					map[string]any{"round": 1, "member_id": "C3", "vote": "not_demonstrated", "rationale": "R3"},
				},
			},
		},
	}
	return result, rc
}
