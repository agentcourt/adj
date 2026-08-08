package proceeding

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jsmorph/adj/arbd/runtime/spec"
)

func (rc *runContext) recordEvent(eventType string, role string, phase string, payload map[string]any) error {
	return rc.recordEventAtTurn(rc.turn, eventType, role, phase, payload)
}

func (rc *runContext) recordEventAtTurn(turn int, eventType string, role string, phase string, payload map[string]any) error {
	event := Event{
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		Turn:      turn,
		Role:      role,
		Phase:     phase,
		Type:      eventType,
		Payload:   payload,
	}
	rc.events = append(rc.events, event)
	return appendJSONLine(filepath.Join(rc.cfg.OutputDir, "events.ndjson"), event)
}

func (rc *runContext) recordWorkNotesAtTurn(turn int, opportunity Opportunity, callID string, notes string) error {
	note := WorkNote{
		Timestamp:     time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		Turn:          turn,
		Role:          opportunity.Role,
		Phase:         opportunity.Phase,
		OpportunityID: opportunity.ID,
		CallID:        strings.TrimSpace(callID),
		Notes:         notes,
	}
	return appendJSONLine(filepath.Join(rc.cfg.OutputDir, "work-notes.ndjson"), note)
}

func writeEvidence(cfg Config, result Result, rc *runContext) error {
	if err := exportAttorneyWorkProduct(cfg.OutputDir, rc.workProductDirs); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "complaint.md"), []byte(spec.ComplaintMarkdown(result.Complaint)), 0o644); err != nil {
		return fmt.Errorf("write complaint copy: %w", err)
	}
	if err := writeJSONFile(filepath.Join(cfg.OutputDir, "policy.json"), cfg.Policy); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(cfg.OutputDir, "runtime.json"), cfg.Runtime); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(cfg.OutputDir, "run.json"), result); err != nil {
		return err
	}
	if err := rc.writeEvidenceManifest(); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(cfg.OutputDir, "state.json"), result.FinalState); err != nil {
		return err
	}
	if err := writeReplayCertificate(cfg, result, rc); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(cfg.OutputDir, "council.json"), result.Council); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "transcript.md"), []byte(renderTranscript(result, rc)), 0o644); err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "digest.md"), []byte(renderDigest(result, rc)), 0o644); err != nil {
		return fmt.Errorf("write digest: %w", err)
	}
	return nil
}

func exportAttorneyWorkProduct(outputDir string, workProductDirs map[string]string) error {
	if len(workProductDirs) == 0 {
		return nil
	}
	workRoot := filepath.Join(outputDir, "work-product")
	roles := make([]string, 0, len(workProductDirs))
	for role := range workProductDirs {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		src := strings.TrimSpace(workProductDirs[role])
		if src == "" {
			continue
		}
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat work-product dir for %s: %w", role, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("work-product path for %s is not a directory", role)
		}
		dst := filepath.Join(workRoot, role)
		if err := copyTree(dst, src); err != nil {
			return fmt.Errorf("export work product for %s: %w", role, err)
		}
	}
	return nil
}

func renderTranscript(result Result, rc *runContext) string {
	caseObj := mapAny(result.FinalState["case"])
	var b strings.Builder
	b.WriteString("# Degree Arbitration Transcript\n\n")
	appendComplaintSection(&b, result)
	appendCouncilSection(&b, result, true)
	b.WriteString("## Proceeding\n\n")
	appendTranscriptPhase(&b, "Openings", "openings", mapList(caseObj["openings"]), caseObj, rc)
	appendTranscriptPhase(&b, "Arguments", "arguments", mapList(caseObj["arguments"]), caseObj, rc)
	appendTranscriptPhase(&b, "Rebuttals", "rebuttals", mapList(caseObj["rebuttals"]), caseObj, rc)
	appendTranscriptPhase(&b, "Surrebuttals", "surrebuttals", mapList(caseObj["surrebuttals"]), caseObj, rc)
	appendTranscriptPhase(&b, "Closings", "closings", mapList(caseObj["closings"]), caseObj, rc)
	b.WriteString("## Council Answers\n\n")
	b.WriteString(renderAnswerRounds(mapList(caseObj["council_answers"])))
	b.WriteString("\n\n## Exhibits\n\n")
	b.WriteString(rc.renderExhibitBodies(mapList(caseObj["offered_evidence"])))
	b.WriteString("\n\n## Submitted Evidence\n\n")
	b.WriteString(renderSubmittedEvidence(mapList(caseObj["submitted_evidence"])))
	b.WriteString("\n\n## Technical Reports\n\n")
	b.WriteString(renderReports(mapList(caseObj["technical_reports"])))
	b.WriteString("\n\n## Answers\n\n")
	b.WriteString(renderAnswerMap(result.Answers))
	b.WriteString("\n\n## Final State\n\n")
	b.WriteString(fmt.Sprintf("Final phase: `%s`\n", mapString(caseObj["phase"])))
	return b.String()
}

func renderDigest(result Result, rc *runContext) string {
	caseObj := mapAny(result.FinalState["case"])
	var b strings.Builder
	b.WriteString("# Degree Arbitration Digest\n\n")
	b.WriteString("## Answers\n\n")
	b.WriteString(renderAnswerMap(result.Answers))
	b.WriteString(fmt.Sprintf("Final phase: `%s`\n\n", mapString(caseObj["phase"])))
	appendComplaintSection(&b, result)
	appendCouncilSection(&b, result, true)
	b.WriteString("\n## Filings\n\n")
	appendFilingSection(&b, "Openings", mapList(caseObj["openings"]))
	appendFilingSection(&b, "Arguments", mapList(caseObj["arguments"]))
	appendFilingSection(&b, "Rebuttals", mapList(caseObj["rebuttals"]))
	appendFilingSection(&b, "Surrebuttals", mapList(caseObj["surrebuttals"]))
	appendFilingSection(&b, "Closings", mapList(caseObj["closings"]))
	b.WriteString("## Exhibits\n\n")
	b.WriteString(rc.renderExhibitIndex(mapList(caseObj["offered_evidence"])))
	b.WriteString("\n\n## Submitted Evidence\n\n")
	b.WriteString(renderSubmittedEvidence(mapList(caseObj["submitted_evidence"])))
	b.WriteString("\n\n## Technical Reports\n\n")
	b.WriteString(renderReports(mapList(caseObj["technical_reports"])))
	b.WriteString("\n\n## Council Answers\n\n")
	b.WriteString(renderAnswerRounds(mapList(caseObj["council_answers"])))
	return b.String()
}

func appendFilingSection(b *strings.Builder, title string, items []map[string]any) {
	b.WriteString("### ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(renderFilingList(items))
	b.WriteString("\n\n")
}

func appendComplaintSection(b *strings.Builder, result Result) {
	b.WriteString("## Complaint\n\n")
	b.WriteString("### Question\n\n")
	b.WriteString(result.Complaint.Question)
	b.WriteString("\n\n### Judgment Standard\n\n")
	b.WriteString(result.JudgmentStandard)
}

func appendCouncilSection(b *strings.Builder, result Result, includePersona bool) {
	b.WriteString("\n\n## Council\n\n")
	for _, seat := range result.Council {
		status := strings.TrimSpace(seat.Status)
		if status == "" {
			status = "seated"
		}
		if includePersona {
			b.WriteString(fmt.Sprintf("- %s: %s (%s), status: %s", seat.MemberID, seat.Model, seat.PersonaFile, status))
			if status == "failed" && strings.TrimSpace(seat.FailureReason) != "" {
				b.WriteString(fmt.Sprintf(", failure: %s", seat.FailureReason))
			}
			b.WriteString("\n")
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %s, status: %s", seat.MemberID, seat.Model, status))
		if status == "failed" && strings.TrimSpace(seat.FailureReason) != "" {
			b.WriteString(fmt.Sprintf(", failure: %s", seat.FailureReason))
		}
		b.WriteString("\n")
	}
}

func appendTranscriptPhase(b *strings.Builder, title string, phase string, items []map[string]any, caseObj map[string]any, rc *runContext) {
	if len(items) == 0 {
		return
	}
	b.WriteString("### ")
	b.WriteString(title)
	b.WriteString("\n\n")
	for _, item := range items {
		role := mapString(item["role"])
		b.WriteString("#### ")
		b.WriteString(transcriptEntryTitle(role, phase))
		b.WriteString("\n\n")
		b.WriteString(mapString(item["text"]))
		b.WriteString("\n\n")
		exhibits := filterEvidence(mapList(caseObj["offered_evidence"]), phase, role)
		if len(exhibits) > 0 {
			b.WriteString("Exhibits offered:\n")
			b.WriteString(renderInlineExhibitIndex(exhibits, rc.fileByID))
			b.WriteString("\n\n")
		}
		reports := filterEvidence(mapList(caseObj["technical_reports"]), phase, role)
		if len(reports) > 0 {
			b.WriteString("Technical reports:\n")
			b.WriteString(renderInlineReportIndex(reports))
			b.WriteString("\n\n")
		}
	}
}

func transcriptEntryTitle(role string, phase string) string {
	role = titleCase(role)
	switch phase {
	case "openings":
		return role + " Opening"
	case "arguments":
		return role + " Argument"
	case "rebuttals":
		return role + " Rebuttal"
	case "surrebuttals":
		return role + " Surrebuttal"
	case "closings":
		return role + " Closing"
	default:
		return role
	}
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func filterEvidence(items []map[string]any, phase string, role string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, item := range items {
		if mapString(item["phase"]) != phase || mapString(item["role"]) != role {
			continue
		}
		out = append(out, item)
	}
	return out
}

func renderInlineExhibitIndex(items []map[string]any, fileByID map[string]CaseFile) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		evidenceID := mapString(item["evidence_id"])
		label := mapString(item["label"])
		name := evidenceID
		if file, ok := fileByID[evidenceID]; ok && strings.TrimSpace(file.Name) != "" {
			name = file.Name
		}
		if label == "" {
			lines = append(lines, "- "+name)
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", label, name))
	}
	return strings.Join(lines, "\n")
}

func renderInlineReportIndex(items []map[string]any) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s\n  %s", mapString(item["title"]), mapString(item["summary"])))
	}
	return strings.Join(lines, "\n")
}

func renderSubmittedEvidence(items []map[string]any) string {
	if len(items) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		source := mapString(item["source_url"])
		if source == "" {
			source = mapString(item["source_description"])
		}
		lines = append(lines, fmt.Sprintf(
			"[%s %s] %s (%s)\nSource: %s\nSHA-256: `%s`\nBytes: %s\nRelevance: %s",
			mapString(item["role"]),
			mapString(item["phase"]),
			mapString(item["title"]),
			mapString(item["evidence_id"]),
			source,
			mapString(item["sha256"]),
			mapString(item["size_bytes"]),
			mapString(item["relevance"]),
		))
	}
	return strings.Join(lines, "\n\n")
}

func renderAnswerRounds(items []map[string]any) string {
	if len(items) == 0 {
		return "(none)"
	}
	rounds := make(map[int][]map[string]any)
	order := make([]int, 0)
	for _, item := range items {
		round := intNumber(item["round"])
		if _, ok := rounds[round]; !ok {
			order = append(order, round)
		}
		rounds[round] = append(rounds[round], item)
	}
	var b strings.Builder
	for i, round := range order {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(fmt.Sprintf("### Round %d\n\n", round))
		for j, item := range rounds[round] {
			if j > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(fmt.Sprintf("[%s] %d\n%s", mapString(item["member_id"]), intNumber(item["answer"]), mapString(item["rationale"])))
		}
	}
	return b.String()
}

func renderAnswerMap(answers map[string]int) string {
	if len(answers) == 0 {
		return "(none)"
	}
	memberIDs := make([]string, 0, len(answers))
	for memberID := range answers {
		memberIDs = append(memberIDs, memberID)
	}
	sort.Strings(memberIDs)
	lines := make([]string, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		lines = append(lines, fmt.Sprintf("- %s: %d", memberID, answers[memberID]))
	}
	return strings.Join(lines, "\n")
}
