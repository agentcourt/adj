package proceeding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jsmorph/adj/arbd/runtime/spec"
)

const councilTurnSnapshotSchemaVersion = "aard.council-turn-snapshot.v0"

type councilTurnSnapshot struct {
	SchemaVersion    string                  `json:"schema_version"`
	CreatedAt        string                  `json:"created_at"`
	SourceOutputDir  string                  `json:"source_output_dir,omitempty"`
	CaseID           string                  `json:"case_id"`
	RunID            string                  `json:"run_id,omitempty"`
	MemberID         string                  `json:"member_id"`
	TurnNumber       int                     `json:"turn_number"`
	Opportunity      Opportunity             `json:"opportunity"`
	Seat             councilTurnSnapshotSeat `json:"seat"`
	Policy           Policy                  `json:"policy"`
	Runtime          RuntimeLimits           `json:"runtime"`
	Complaint        spec.Complaint          `json:"complaint"`
	State            map[string]any          `json:"state"`
	Prompt           string                  `json:"prompt"`
	Tools            []map[string]any        `json:"tools"`
	Limits           map[string]any          `json:"limits"`
	CaseView         map[string]any          `json:"case_view"`
	Evidence         []EvidenceMeta          `json:"evidence"`
	EvidenceManifest map[string]any          `json:"evidence_manifest"`
}

type councilTurnSnapshotSeat struct {
	MemberID    string `json:"member_id"`
	Model       string `json:"model"`
	PersonaFile string `json:"persona_file"`
	PersonaText string `json:"persona_text,omitempty"`
	RequestSpec any    `json:"request_spec,omitempty"`
}

func (rc *runContext) writeCouncilTurnSnapshot(turn *councilTurn, prompt string) error {
	if turn == nil {
		return fmt.Errorf("council turn is required")
	}
	state, err := cloneMapJSON(rc.state)
	if err != nil {
		return err
	}
	snapshot := councilTurnSnapshot{
		SchemaVersion:    councilTurnSnapshotSchemaVersion,
		CreatedAt:        utcTimestamp(),
		SourceOutputDir:  rc.cfg.OutputDir,
		CaseID:           normalizeCaseID(rc.cfg.CaseID),
		RunID:            rc.cfg.RunID,
		MemberID:         turn.seat.MemberID,
		TurnNumber:       turn.turnNumber,
		Opportunity:      turn.opportunity,
		Seat:             snapshotCouncilSeat(turn.seat),
		Policy:           rc.cfg.Policy,
		Runtime:          rc.cfg.Runtime,
		Complaint:        rc.complaint,
		State:            state,
		Prompt:           prompt,
		Tools:            councilToolSpecs(),
		Limits:           councilTurnLimits(rc.cfg.Policy, rc.cfg.Runtime, turn),
		CaseView:         rc.councilView(turn.seat, turn.opportunity),
		Evidence:         rc.listVisibleEvidence(),
		EvidenceManifest: rc.evidenceManifest(),
	}
	dir := filepath.Join(rc.cfg.OutputDir, "council-turns", fmt.Sprintf("turn-%06d-%s", turn.turnNumber, safePathComponent(turn.seat.MemberID)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create council snapshot dir: %w", err)
	}
	if err := writeJSONFile(filepath.Join(dir, "input.json"), snapshot); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("write council prompt snapshot: %w", err)
	}
	return nil
}

func councilTurnLimits(policy Policy, runtime RuntimeLimits, turn *councilTurn) map[string]any {
	return map[string]any{
		"max_response_bytes":                            runtime.MaxResponseBytes,
		"attempts_max":                                  turn.attemptsMax,
		"attempts_remaining":                            turn.attemptsRemaining,
		"max_evidence_read_bytes":                       policy.MaxEvidenceReadBytes,
		"max_evidence_reads_per_opportunity":            policy.MaxEvidenceReadsPerOpportunity,
		"max_evidence_read_bytes_per_opportunity":       policy.MaxEvidenceReadBytesPerOpportunity,
		"remaining_evidence_reads_for_opportunity":      remainingCapacity(policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads),
		"remaining_evidence_read_bytes_for_opportunity": remainingCapacity(policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes),
	}
}

func snapshotCouncilSeat(seat CouncilSeat) councilTurnSnapshotSeat {
	return councilTurnSnapshotSeat{
		MemberID:    seat.MemberID,
		Model:       seat.Model,
		PersonaFile: seat.PersonaFile,
		PersonaText: seat.PersonaText,
		RequestSpec: seat.RequestSpec,
	}
}

func safePathComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "member"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "member"
	}
	return out
}
