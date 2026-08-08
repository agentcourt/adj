package proceeding

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func attorneyDecision(opportunity Opportunity, params map[string]any, fileByID map[string]CaseFile, policy Policy) (string, map[string]any, error) {
	kind := mapString(params["kind"])
	switch kind {
	case "pass":
		if !opportunity.MayPass {
			return "", nil, fmt.Errorf("passing is not allowed in this opportunity")
		}
		switch opportunity.Phase {
		case "rebuttals", "surrebuttals":
			return "pass_phase_opportunity", map[string]any{}, nil
		default:
			return "", nil, fmt.Errorf("passing is not allowed in phase %q", opportunity.Phase)
		}
	case "tool":
		toolName := mapString(params["tool_name"])
		if toolName == "" {
			return "", nil, fmt.Errorf("submit_decision tool_name is required when kind is tool")
		}
		if !submitDecisionTool(toolName) {
			return "", nil, fmt.Errorf("tool %q must be called directly, not through submit_decision", toolName)
		}
		if !slices.Contains(opportunity.AllowedTools, toolName) {
			return "", nil, fmt.Errorf("tool %q is not allowed in this opportunity", toolName)
		}
		payload := normalizePayload(params["payload"])
		if err := validateAttorneyPayload(toolName, payload, fileByID, policy); err != nil {
			return "", nil, err
		}
		return toolName, payload, nil
	default:
		return "", nil, fmt.Errorf("submit_decision kind must be tool or pass")
	}
}

func validateAttorneyPayload(actionType string, payload map[string]any, fileByID map[string]CaseFile, policy Policy) error {
	switch actionType {
	case "record_opening_statement":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
	case "deliver_closing_statement":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if len(listOfMaps(payload["offered_evidence"])) != 0 {
			return fmt.Errorf("offered_evidence are allowed only in arguments, rebuttals, and surrebuttals")
		}
		if len(listOfMaps(payload["technical_reports"])) != 0 {
			return fmt.Errorf("technical_reports are allowed only in arguments, rebuttals, and surrebuttals")
		}
	case "submit_argument":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if err := validateOfferedEvidence(payload["offered_evidence"], fileByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload["technical_reports"], policy); err != nil {
			return err
		}
	case "submit_rebuttal":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if err := validateOfferedEvidence(payload["offered_evidence"], fileByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload["technical_reports"], policy); err != nil {
			return err
		}
	case "submit_surrebuttal":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if err := validateOfferedEvidence(payload["offered_evidence"], fileByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload["technical_reports"], policy); err != nil {
			return err
		}
	case "pass_phase_opportunity":
	default:
		return fmt.Errorf("unsupported action type %q", actionType)
	}
	return nil
}

func validateOfferedEvidence(value any, fileByID map[string]CaseFile, policy Policy) error {
	entries := listOfMaps(value)
	if len(entries) > policy.MaxExhibitsPerFiling {
		return fmt.Errorf("offered_evidence exceed per-filing limit of %d (attempted %d)", policy.MaxExhibitsPerFiling, len(entries))
	}
	for _, entry := range entries {
		evidenceID := mapString(entry["evidence_id"])
		if evidenceID == "" {
			return fmt.Errorf("offered_evidence entry requires evidence_id")
		}
		file, ok := fileByID[evidenceID]
		if !ok {
			return fmt.Errorf("unknown offered file %q; offered_evidence must use visible case evidence_id values, not workspace paths or downloaded filenames", evidenceID)
		}
		if file.SizeBytes > policy.MaxExhibitBytes {
			return fmt.Errorf("offered file %q exceeds byte limit of %d", evidenceID, policy.MaxExhibitBytes)
		}
	}
	return nil
}

func validateReports(value any, policy Policy) error {
	entries := listOfMaps(value)
	if len(entries) > policy.MaxReportsPerFiling {
		return fmt.Errorf("technical_reports exceed per-filing limit of %d (attempted %d)", policy.MaxReportsPerFiling, len(entries))
	}
	for _, entry := range entries {
		title := mapString(entry["title"])
		summary := mapString(entry["summary"])
		if title == "" {
			return fmt.Errorf("technical_reports entry requires title")
		}
		if summary == "" {
			return fmt.Errorf("technical_reports entry requires summary")
		}
		if len([]byte(title)) > policy.MaxReportTitleBytes {
			return fmt.Errorf("technical_reports title exceeds byte limit of %d", policy.MaxReportTitleBytes)
		}
		if len([]byte(summary)) > policy.MaxReportSummaryBytes {
			return fmt.Errorf("technical_reports summary exceeds byte limit of %d", policy.MaxReportSummaryBytes)
		}
	}
	return nil
}

func normalizePayload(value any) map[string]any {
	payload := mapAny(value)
	if len(payload) == 0 {
		return map[string]any{}
	}
	return cloneMap(payload)
}

func jsonPayloadSize(value any) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("marshal response payload size: %w", err)
	}
	return len(raw), nil
}

func listOfMaps(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, raw := range v {
			entry, _ := raw.(map[string]any)
			if entry != nil {
				out = append(out, entry)
			}
		}
		return out
	default:
		return nil
	}
}

func decisionToolEnum(allowedTools []string) []string {
	fallback := []string{
		"record_opening_statement",
		"submit_argument",
		"submit_rebuttal",
		"submit_surrebuttal",
		"deliver_closing_statement",
		"pass_phase_opportunity",
	}
	if len(allowedTools) == 0 {
		return fallback
	}
	out := make([]string, 0, len(allowedTools))
	for _, tool := range allowedTools {
		tool = strings.TrimSpace(tool)
		if tool != "" && submitDecisionTool(tool) && !slices.Contains(out, tool) {
			out = append(out, tool)
		}
	}
	return out
}

func submitDecisionTool(tool string) bool {
	switch tool {
	case "record_opening_statement", "submit_argument", "submit_rebuttal", "submit_surrebuttal", "deliver_closing_statement", "pass_phase_opportunity":
		return true
	default:
		return false
	}
}

func attorneyPayloadSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":              map[string]any{"type": "string"},
			"offered_evidence":  offeredEvidenceSchema(),
			"technical_reports": technicalReportsSchema(),
		},
		"additionalProperties": false,
	}
}

func offeredEvidenceSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"evidence_id": map[string]any{"type": "string"},
				"label":       map[string]any{"type": "string"},
			},
			"required":             []string{"evidence_id", "label"},
			"additionalProperties": false,
		},
	}
}

func technicalReportsSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":   map[string]any{"type": "string"},
				"summary": map[string]any{"type": "string"},
			},
			"required":             []string{"title", "summary"},
			"additionalProperties": false,
		},
	}
}

func submittedEvidenceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":                  map[string]any{"type": "string"},
			"source_url":             map[string]any{"type": "string"},
			"source_description":     map[string]any{"type": "string"},
			"retrieval_timestamp":    map[string]any{"type": "string"},
			"mime_type":              map[string]any{"type": "string"},
			"relevance":              map[string]any{"type": "string"},
			"content":                map[string]any{"type": "string"},
			"content_base64":         map[string]any{"type": "string"},
			"preferred_filename_ext": map[string]any{"type": "string"},
		},
		"required":             []string{"title", "mime_type", "relevance"},
		"additionalProperties": false,
	}
}

func evidenceReadAllowed(opportunity Opportunity) bool {
	switch opportunity.Phase {
	case "openings", "arguments", "rebuttals", "surrebuttals", "closings":
		return true
	default:
		return false
	}
}

func evidenceSubmissionAllowed(opportunity Opportunity) bool {
	return opportunity.Phase == "arguments" || opportunity.Phase == "rebuttals" || opportunity.Phase == "surrebuttals"
}

func (rc *runContext) attorneyView(opportunity Opportunity) map[string]any {
	limits := rc.attorneyLimits(opportunity)
	return map[string]any{
		"question":          rc.complaint.Question,
		"judgment_standard": currentJudgmentStandard(rc.state, rc.cfg.Policy),
		"phase":             currentPhase(rc.state),
		"opportunity": map[string]any{
			"id":            opportunity.ID,
			"role":          opportunity.Role,
			"phase":         opportunity.Phase,
			"objective":     opportunity.Objective,
			"allowed_tools": opportunity.AllowedTools,
			"may_pass":      opportunity.MayPass,
		},
		"record": map[string]any{
			"evidence":           rc.listVisibleEvidence(),
			"openings":           mapList(mapAny(rc.state["case"])["openings"]),
			"arguments":          mapList(mapAny(rc.state["case"])["arguments"]),
			"rebuttals":          mapList(mapAny(rc.state["case"])["rebuttals"]),
			"surrebuttals":       mapList(mapAny(rc.state["case"])["surrebuttals"]),
			"closings":           mapList(mapAny(rc.state["case"])["closings"]),
			"submitted_evidence": mapList(mapAny(rc.state["case"])["submitted_evidence"]),
			"exhibits":           rc.attorneyExhibits(),
			"technical_reports":  mapList(mapAny(rc.state["case"])["technical_reports"]),
		},
		"limits":  limits,
		"council": rc.council,
	}
}

func (rc *runContext) attorneyCapabilitySection(role string, opportunityID string) string {
	return fmt.Sprintf("Use the Lawyer API as role %s. GET returns the current prompt, available tools, opportunity id, live deadline, and attempts left. POST executes one tool call and must include the current turn.opportunity_id. For this turn, opportunity_id is %s.", role, opportunityID)
}

func (rc *runContext) buildAttorneyPrompt(opportunity Opportunity) (string, error) {
	view := rc.attorneyView(opportunity)
	visibleFilesSection := ""
	workspaceSection := "Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter. Do not reconstruct byte-sensitive evidence by hand. Use evidence_id plus hash as record identity.\n"
	workProductSection := ""
	if opportunity.Phase == "arguments" || opportunity.Phase == "rebuttals" || opportunity.Phase == "surrebuttals" {
		visibleFilesSection = "Visible evidence:\n" + marshalIndented(rc.listVisibleEvidence()) + "\n"
	}
	common, err := rc.cfg.renderPromptFile("attorney-common.md", map[string]string{
		"ROLE":                       opportunity.Role,
		"PHASE":                      opportunity.Phase,
		"OBJECTIVE":                  opportunity.Objective,
		"OPPORTUNITY_ID":             opportunity.ID,
		"QUESTION":                   rc.complaint.Question,
		"JUDGMENT_STANDARD":          currentJudgmentStandard(rc.state, rc.cfg.Policy),
		"MODEL_CAPABILITIES_SECTION": rc.attorneyCapabilitySection(opportunity.Role, opportunity.ID),
		"CURRENT_RECORD":             marshalIndented(view["record"]),
		"LIMITS_SECTION":             rc.attorneyLimitsSection(opportunity),
		"COUNCIL":                    marshalIndented(view["council"]),
		"VISIBLE_CASE_FILES_SECTION": visibleFilesSection,
		"WORKSPACE_SECTION":          workspaceSection,
		"WORK_PRODUCT_SECTION":       workProductSection,
		"ALLOWED_TOOLS":              strings.Join(decisionToolEnum(opportunity.AllowedTools), ", "),
		"DECISION_TOOLS":             strings.Join(decisionToolEnum(opportunity.AllowedTools), ", "),
	})
	if err != nil {
		return "", err
	}
	phaseFile, err := attorneyPromptFile(opportunity.Phase)
	if err != nil {
		return "", err
	}
	phaseText, err := rc.cfg.renderPromptFile(phaseFile, nil)
	if err != nil {
		return "", err
	}
	return common + "\n\n" + phaseText + "\n\nSubmit the legal act with submit_decision before the deadline.", nil
}

func (rc *runContext) prepareSubmittedEvidence(opportunity Opportunity, params map[string]any) (SubmittedEvidenceMeta, []byte, error) {
	if !evidenceSubmissionAllowed(opportunity) {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence is allowed only in arguments, rebuttals, and surrebuttals")
	}
	title := mapString(params["title"])
	mimeType := mapString(params["mime_type"])
	relevance := mapString(params["relevance"])
	sourceURL := mapString(params["source_url"])
	sourceDescription := mapString(params["source_description"])
	retrievalTimestamp := mapString(params["retrieval_timestamp"])
	if title == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires title")
	}
	if sourceURL == "" && sourceDescription == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires source_url or source_description")
	}
	if mimeType == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires mime_type")
	}
	if relevance == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires relevance")
	}
	raw, err := submittedEvidenceContent(params)
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, err
	}
	if len(raw) == 0 {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence content must not be empty")
	}
	if len(raw) > rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("direct submitted evidence exceeds byte limit of %d", rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes)
	}
	if len(raw) > rc.cfg.Policy.MaxSubmittedEvidenceBytes {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence exceeds byte limit of %d", rc.cfg.Policy.MaxSubmittedEvidenceBytes)
	}
	if submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedEvidencePerSide {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted_evidence for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedEvidencePerSide)
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	name := submittedEvidenceFilename(len(rc.submittedEvidence)+1, opportunity.Role, sha, mimeType, mapString(params["preferred_filename_ext"]))
	return SubmittedEvidenceMeta{
		Phase:              opportunity.Phase,
		Role:               opportunity.Role,
		EvidenceID:         evidenceIDForFile(sha, name),
		Name:               name,
		Title:              title,
		SourceURL:          sourceURL,
		SourceDescription:  sourceDescription,
		MimeType:           mimeType,
		RetrievalTimestamp: retrievalTimestamp,
		Relevance:          relevance,
		SHA256:             sha,
		SizeBytes:          len(raw),
	}, raw, nil
}

func submittedEvidenceContent(params map[string]any) ([]byte, error) {
	content, hasContent := rawStringParam(params, "content")
	contentBase64 := mapString(params["content_base64"])
	if hasContent && contentBase64 != "" {
		return nil, fmt.Errorf("use content or content_base64, not both")
	}
	if contentBase64 != "" {
		raw, err := base64.StdEncoding.DecodeString(contentBase64)
		if err != nil {
			return nil, fmt.Errorf("decode content_base64: %w", err)
		}
		return raw, nil
	}
	if !hasContent {
		return nil, fmt.Errorf("submitted evidence requires content or content_base64")
	}
	return []byte(content), nil
}

func rawStringParam(params map[string]any, key string) (string, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return "", false
	}
	s, ok := value.(string)
	return s, ok
}

func submittedEvidenceFilename(index int, role string, sha string, mimeType string, preferredExt string) string {
	ext := sanitizeEvidenceExtension(preferredExt)
	if ext == "" {
		ext = evidenceExtensionForMIME(mimeType)
	}
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("submitted-evidence-%02d-%s-%s%s", index, sanitizeEvidenceComponent(role), sha[:12], ext)
}

func sanitizeEvidenceComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

func sanitizeEvidenceExtension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	if filepath.Base("x"+value) != "x"+value {
		return ""
	}
	for _, r := range value[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return value
}

func evidenceExtensionForMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "text/plain":
		return ".txt"
	case "text/markdown":
		return ".md"
	case "text/html":
		return ".html"
	case "application/json":
		return ".json"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func submittedEvidencePayload(meta SubmittedEvidenceMeta) map[string]any {
	payload := map[string]any{
		"evidence_id":         meta.EvidenceID,
		"title":               meta.Title,
		"source_url":          meta.SourceURL,
		"source_description":  meta.SourceDescription,
		"mime_type":           meta.MimeType,
		"retrieval_timestamp": meta.RetrievalTimestamp,
		"relevance":           meta.Relevance,
		"sha256":              meta.SHA256,
		"size_bytes":          meta.SizeBytes,
	}
	if meta.EvidenceID != "" {
		payload["evidence_id"] = meta.EvidenceID
	}
	return payload
}

func (rc *runContext) writeSubmittedEvidenceFile(meta SubmittedEvidenceMeta, raw []byte) (CaseFile, error) {
	dir := filepath.Join(rc.cfg.OutputDir, "submitted-evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CaseFile{}, fmt.Errorf("create submitted evidence dir: %w", err)
	}
	name := filepath.Base(meta.Name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return CaseFile{}, fmt.Errorf("invalid submitted evidence filename %q", meta.Name)
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return CaseFile{}, fmt.Errorf("submitted evidence file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return CaseFile{}, fmt.Errorf("stat submitted evidence file %s: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return CaseFile{}, fmt.Errorf("write submitted evidence %s: %w", path, err)
	}
	_, readable := caseFileKind(name)
	file := CaseFile{
		EvidenceID:   meta.EvidenceID,
		Name:         name,
		Path:         path,
		MimeType:     meta.MimeType,
		TextReadable: readable || strings.HasPrefix(strings.ToLower(meta.MimeType), "text/") || strings.EqualFold(meta.MimeType, "application/json"),
		SizeBytes:    len(raw),
	}
	if file.TextReadable {
		file.Text = string(raw)
	}
	return file, nil
}

func (rc *runContext) attorneyExhibits() []map[string]any {
	items := mapList(mapAny(rc.state["case"])["offered_evidence"])
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		evidenceID := mapString(item["evidence_id"])
		label := mapString(item["label"])
		entry := map[string]any{
			"phase":       mapString(item["phase"]),
			"role":        mapString(item["role"]),
			"evidence_id": evidenceID,
			"label":       label,
		}
		if file, ok := rc.fileByID[evidenceID]; ok {
			if file.TextReadable {
				entry["text"] = file.Text
			} else {
				entry["text"] = "(binary or non-text file)"
			}
		} else {
			entry["text"] = "(unavailable file)"
		}
		out = append(out, entry)
	}
	return out
}

func (rc *runContext) attorneyLimits(opportunity Opportunity) map[string]any {
	caseObj := mapAny(rc.state["case"])
	usedExhibits := filingCountForRole(mapList(caseObj["offered_evidence"]), opportunity.Role)
	usedReports := filingCountForRole(mapList(caseObj["technical_reports"]), opportunity.Role)
	limits := map[string]any{
		"text_char_limit": phaseTextCharLimit(rc.cfg.Policy, opportunity.Phase),
	}
	if evidenceSubmissionAllowed(opportunity) {
		limits["max_exhibits_per_filing"] = rc.cfg.Policy.MaxExhibitsPerFiling
		limits["max_exhibits_per_side"] = rc.cfg.Policy.MaxExhibitsPerSide
		limits["used_exhibits_for_side"] = usedExhibits
		limits["remaining_exhibits_for_side"] = remainingCapacity(rc.cfg.Policy.MaxExhibitsPerSide, usedExhibits)
		limits["max_reports_per_filing"] = rc.cfg.Policy.MaxReportsPerFiling
		limits["max_reports_per_side"] = rc.cfg.Policy.MaxReportsPerSide
		limits["used_reports_for_side"] = usedReports
		limits["remaining_reports_for_side"] = remainingCapacity(rc.cfg.Policy.MaxReportsPerSide, usedReports)
		usedSubmittedEvidence := submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role)
		limits["max_submitted_evidence_per_side"] = rc.cfg.Policy.MaxSubmittedEvidencePerSide
		limits["max_submitted_evidence_bytes"] = rc.cfg.Policy.MaxSubmittedEvidenceBytes
		limits["max_direct_submitted_evidence_bytes"] = rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes
		limits["max_evidence_upload_bytes"] = rc.cfg.Policy.MaxEvidenceUploadBytes
		limits["max_evidence_chunk_bytes"] = rc.cfg.Policy.MaxEvidenceChunkBytes
		limits["max_evidence_read_bytes"] = rc.cfg.Policy.MaxEvidenceReadBytes
		limits["max_evidence_reads_per_opportunity"] = rc.cfg.Policy.MaxEvidenceReadsPerOpportunity
		limits["max_evidence_read_bytes_per_opportunity"] = rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity
		limits["used_submitted_evidence_for_side"] = usedSubmittedEvidence
		limits["remaining_submitted_evidence_for_side"] = remainingCapacity(rc.cfg.Policy.MaxSubmittedEvidencePerSide, usedSubmittedEvidence)
		limits["offered_evidence_rule"] = "Use only visible case evidence_id values in offered_evidence. Submit new evidence first with submit_evidence, then cite the returned evidence_id in offered_evidence."
		limits["outside_material_rule"] = "Outside source material belongs in submitted evidence when the source content matters, or in technical_reports when only attorney analysis is being offered."
	}
	return limits
}

func (rc *runContext) attorneyLimitsSection(opportunity Opportunity) string {
	limits := rc.attorneyLimits(opportunity)
	lines := []string{}
	if limit, _ := limits["text_char_limit"].(int); limit > 0 {
		lines = append(lines, fmt.Sprintf("Text limit for this submission: %d characters.", limit))
		lines = append(lines, fmt.Sprintf("Target length for the first submission: %d characters or less.", targetSubmissionCharLimit(limit)))
	}
	switch opportunity.Phase {
	case "arguments", "rebuttals", "surrebuttals":
		lines = append(lines,
			fmt.Sprintf(
				"Exhibits: at most %d in this filing. This side has used %d of %d total, with %d left.",
				limits["max_exhibits_per_filing"].(int),
				limits["used_exhibits_for_side"].(int),
				limits["max_exhibits_per_side"].(int),
				limits["remaining_exhibits_for_side"].(int),
			),
		)
		lines = append(lines,
			fmt.Sprintf(
				"Technical reports: at most %d in this filing. This side has used %d of %d total, with %d left.",
				limits["max_reports_per_filing"].(int),
				limits["used_reports_for_side"].(int),
				limits["max_reports_per_side"].(int),
				limits["remaining_reports_for_side"].(int),
			),
		)
		lines = append(lines,
			fmt.Sprintf(
				"Submitted evidence: admitted items may be at most %d bytes. Direct submit_evidence items may be at most %d bytes; chunked evidence uploads may be at most %d bytes with %d-byte chunks. This side has submitted %d of %d total, with %d left.",
				limits["max_submitted_evidence_bytes"].(int),
				limits["max_direct_submitted_evidence_bytes"].(int),
				limits["max_evidence_upload_bytes"].(int),
				limits["max_evidence_chunk_bytes"].(int),
				limits["used_submitted_evidence_for_side"].(int),
				limits["max_submitted_evidence_per_side"].(int),
				limits["remaining_submitted_evidence_for_side"].(int),
			),
		)
		lines = append(lines, fmt.Sprintf("Evidence reads: at most %d bytes per read, %d reads per opportunity, and %d bytes total per opportunity.", limits["max_evidence_read_bytes"].(int), limits["max_evidence_reads_per_opportunity"].(int), limits["max_evidence_read_bytes_per_opportunity"].(int)))
		lines = append(lines, "Use only visible case evidence_id values in offered_evidence. Submit new source material first with submit_evidence, then cite the returned evidence_id in offered_evidence. Use evidence_id and hash for custody checks and exact byte inspection.")
		lines = append(lines, "Use technical_reports for attorney analysis or synthesized work product, not as a substitute for source evidence when exact source content matters.")
	}
	return strings.Join(lines, "\n")
}

func targetSubmissionCharLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	target := (limit * 3) / 4
	if target <= 0 {
		target = limit
	}
	return target
}

func phaseTextCharLimit(policy Policy, phase string) int {
	switch phase {
	case "openings":
		return policy.MaxOpeningChars
	case "arguments":
		return policy.MaxArgumentChars
	case "rebuttals":
		return policy.MaxRebuttalChars
	case "surrebuttals":
		return policy.MaxSurrebuttalChars
	case "closings":
		return policy.MaxClosingChars
	default:
		return 0
	}
}

func filingCountForRole(items []map[string]any, role string) int {
	count := 0
	for _, item := range items {
		if mapString(item["role"]) == role {
			count++
		}
	}
	return count
}

func submittedEvidenceCountForRole(items []SubmittedEvidenceMeta, role string) int {
	count := 0
	for _, item := range items {
		if item.Role == role {
			count++
		}
	}
	return count
}

func remainingCapacity(limit int, used int) int {
	if limit-used < 0 {
		return 0
	}
	return limit - used
}

func (rc *runContext) validateAttorneyPayloadAgainstState(opportunity Opportunity, actionType string, payload map[string]any) error {
	text := strings.TrimSpace(mapString(payload["text"]))
	if limit := phaseTextCharLimit(rc.cfg.Policy, opportunity.Phase); limit > 0 {
		charCount := len([]rune(text))
		if charCount > limit {
			return fmt.Errorf("%s exceeds character limit of %d (got %d)", filingLabel(actionType), limit, charCount)
		}
	}
	switch actionType {
	case "submit_argument", "submit_rebuttal", "submit_surrebuttal":
		caseObj := mapAny(rc.state["case"])
		usedExhibits := filingCountForRole(mapList(caseObj["offered_evidence"]), opportunity.Role)
		attemptedExhibits := len(listOfMaps(payload["offered_evidence"]))
		if usedExhibits+attemptedExhibits > rc.cfg.Policy.MaxExhibitsPerSide {
			return fmt.Errorf(
				"offered_evidence for this side exceed limit of %d (%d already used, %d attempted, %d remaining)",
				rc.cfg.Policy.MaxExhibitsPerSide,
				usedExhibits,
				attemptedExhibits,
				remainingCapacity(rc.cfg.Policy.MaxExhibitsPerSide, usedExhibits),
			)
		}
		usedReports := filingCountForRole(mapList(caseObj["technical_reports"]), opportunity.Role)
		attemptedReports := len(listOfMaps(payload["technical_reports"]))
		if usedReports+attemptedReports > rc.cfg.Policy.MaxReportsPerSide {
			return fmt.Errorf(
				"technical_reports for this side exceed limit of %d (%d already used, %d attempted, %d remaining)",
				rc.cfg.Policy.MaxReportsPerSide,
				usedReports,
				attemptedReports,
				remainingCapacity(rc.cfg.Policy.MaxReportsPerSide, usedReports),
			)
		}
	}
	return nil
}

func filingLabel(actionType string) string {
	switch actionType {
	case "record_opening_statement":
		return "opening statement"
	case "submit_argument":
		return "argument"
	case "submit_rebuttal":
		return "rebuttal"
	case "submit_surrebuttal":
		return "surrebuttal"
	case "deliver_closing_statement":
		return "closing statement"
	default:
		return "submission"
	}
}

func marshalInline(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(raw)
}

func marshalIndented(value any) string {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(raw)
}

func copyTree(dstRoot string, srcRoot string) error {
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}
		dstPath := dstRoot
		if rel != "." {
			dstPath = filepath.Join(dstRoot, rel)
		}
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", dstPath, err)
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink work product is not allowed: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported work-product entry %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.WriteFile(dstPath, raw, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", dstPath, err)
		}
		return nil
	})
}
