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

func attorneyDecision(opportunity Opportunity, params map[string]any, evidenceByID map[string]EvidenceMeta, policy Policy) (string, map[string]any, error) {
	if err := requireAllowedKeys(params, "submit_decision arguments", "kind", "tool_name", "payload"); err != nil {
		return "", nil, err
	}
	payload := map[string]any{}
	if rawPayload, supplied := params["payload"]; supplied {
		var ok bool
		payload, ok = rawPayload.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("submit_decision payload must be an object")
		}
	}
	kind, kindOK := params["kind"].(string)
	kind = strings.TrimSpace(kind)
	if !kindOK {
		return "", nil, fmt.Errorf("submit_decision kind must be tool or pass")
	}
	switch kind {
	case "pass":
		if err := validateAttorneyPayloadKeys(payload); err != nil {
			return "", nil, err
		}
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
		toolName, toolNameOK := params["tool_name"].(string)
		toolName = strings.TrimSpace(toolName)
		if !toolNameOK || toolName == "" {
			return "", nil, fmt.Errorf("submit_decision tool_name is required when kind is tool")
		}
		if !submitDecisionTool(toolName) {
			return "", nil, fmt.Errorf("tool %q must be called directly, not through submit_decision", toolName)
		}
		if !slices.Contains(opportunity.AllowedTools, toolName) {
			return "", nil, fmt.Errorf("tool %q is not allowed in this opportunity", toolName)
		}
		payload = normalizeAttorneyPayload(payload)
		if err := validateAttorneyPayload(toolName, payload, evidenceByID, policy); err != nil {
			return "", nil, err
		}
		return toolName, payload, nil
	default:
		return "", nil, fmt.Errorf("submit_decision kind must be tool or pass")
	}
}

func validateAttorneyPayload(actionType string, payload map[string]any, evidenceByID map[string]EvidenceMeta, policy Policy) error {
	if err := validateAttorneyPayloadKeys(payload); err != nil {
		return err
	}
	switch actionType {
	case "record_opening_statement":
		if _, err := requiredStringField(payload, "text", "payload.text is required"); err != nil {
			return err
		}
		return validateNoSupplementalMaterials(payload)
	case "deliver_closing_statement":
		if _, err := requiredStringField(payload, "text", "payload.text is required"); err != nil {
			return err
		}
		return validateNoSupplementalMaterials(payload)
	case "submit_argument":
		if _, err := requiredStringField(payload, "text", "payload.text is required"); err != nil {
			return err
		}
		if err := validateOfferedEvidence(payload, evidenceByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload, policy); err != nil {
			return err
		}
	case "submit_rebuttal":
		if _, err := requiredStringField(payload, "text", "payload.text is required"); err != nil {
			return err
		}
		if err := validateOfferedEvidence(payload, evidenceByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload, policy); err != nil {
			return err
		}
	case "submit_surrebuttal":
		if _, err := requiredStringField(payload, "text", "payload.text is required"); err != nil {
			return err
		}
		if err := validateOfferedEvidence(payload, evidenceByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload, policy); err != nil {
			return err
		}
	case "pass_phase_opportunity":
	default:
		return fmt.Errorf("unsupported action type %q", actionType)
	}
	return nil
}

func validateAttorneyPayloadKeys(payload map[string]any) error {
	if err := requireAllowedKeys(payload, "submit_decision payload", "text", "offered_evidence", "technical_reports"); err != nil {
		return err
	}
	offered, err := strictObjectList(payload, "offered_evidence")
	if err != nil {
		return err
	}
	for _, entry := range offered {
		if err := requireAllowedKeys(entry, "offered_evidence entry", "evidence_id", "label"); err != nil {
			return err
		}
	}
	reports, err := strictObjectList(payload, "technical_reports")
	if err != nil {
		return err
	}
	for _, entry := range reports {
		if err := requireAllowedKeys(entry, "technical_reports entry", "title", "summary"); err != nil {
			return err
		}
	}
	return nil
}

func requireAllowedKeys(values map[string]any, object string, allowed ...string) error {
	unknown := make([]string, 0)
	for key := range values {
		if !slices.Contains(allowed, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	if len(unknown) == 1 {
		return fmt.Errorf("%s contains unknown field %q", object, unknown[0])
	}
	return fmt.Errorf("%s contains unknown fields %q", object, unknown)
}

func validateNoSupplementalMaterials(payload map[string]any) error {
	offered, err := strictObjectList(payload, "offered_evidence")
	if err != nil {
		return err
	}
	if len(offered) != 0 {
		return fmt.Errorf("offered_evidence are allowed only in arguments, rebuttals, and surrebuttals")
	}
	reports, err := strictObjectList(payload, "technical_reports")
	if err != nil {
		return err
	}
	if len(reports) != 0 {
		return fmt.Errorf("technical_reports are allowed only in arguments, rebuttals, and surrebuttals")
	}
	return nil
}

func validateOfferedEvidence(payload map[string]any, evidenceByID map[string]EvidenceMeta, policy Policy) error {
	entries, err := strictObjectList(payload, "offered_evidence")
	if err != nil {
		return err
	}
	if len(entries) > policy.MaxExhibitsPerFiling {
		return fmt.Errorf("offered_evidence exceed per-filing limit of %d (attempted %d)", policy.MaxExhibitsPerFiling, len(entries))
	}
	for _, entry := range entries {
		evidenceID, err := requiredStringField(entry, "evidence_id", "offered_evidence entry requires evidence_id")
		if err != nil {
			return err
		}
		if _, err := requiredStringField(entry, "label", "offered_evidence entry requires label"); err != nil {
			return err
		}
		evidence, ok := evidenceByID[evidenceID]
		if !ok || !recordEvidence(evidence) {
			return fmt.Errorf("unknown offered file %q; offered_evidence must use visible case evidence_id values, not workspace paths or downloaded filenames", evidenceID)
		}
		if evidence.SizeBytes > policy.MaxExhibitBytes {
			return fmt.Errorf("offered file %q exceeds byte limit of %d", evidenceID, policy.MaxExhibitBytes)
		}
	}
	return nil
}

func validateReports(payload map[string]any, policy Policy) error {
	entries, err := strictObjectList(payload, "technical_reports")
	if err != nil {
		return err
	}
	if len(entries) > policy.MaxReportsPerFiling {
		return fmt.Errorf("technical_reports exceed per-filing limit of %d (attempted %d)", policy.MaxReportsPerFiling, len(entries))
	}
	for _, entry := range entries {
		title, err := requiredStringField(entry, "title", "technical_reports entry requires title")
		if err != nil {
			return err
		}
		summary, err := requiredStringField(entry, "summary", "technical_reports entry requires summary")
		if err != nil {
			return err
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

func requiredStringField(values map[string]any, key string, message string) (string, error) {
	value, ok := values[key].(string)
	value = trimLeanWhitespace(value)
	if !ok || value == "" {
		return "", fmt.Errorf("%s", message)
	}
	return value, nil
}

func normalizeAttorneyPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	out := cloneJSONLikeMap(payload)
	trimAttorneyStringField(out, "text")
	for _, entry := range listOfMaps(out["offered_evidence"]) {
		trimAttorneyStringField(entry, "evidence_id")
		trimAttorneyStringField(entry, "label")
	}
	for _, entry := range listOfMaps(out["technical_reports"]) {
		trimAttorneyStringField(entry, "title")
		trimAttorneyStringField(entry, "summary")
	}
	return out
}

func trimAttorneyStringField(values map[string]any, key string) {
	if value, ok := values[key].(string); ok {
		values[key] = trimLeanWhitespace(value)
	}
}

func trimLeanWhitespace(value string) string {
	return strings.Trim(value, " \t\r\n")
}

func strictObjectList(values map[string]any, field string) ([]map[string]any, error) {
	value, supplied := values[field]
	if !supplied {
		return nil, nil
	}
	switch entries := value.(type) {
	case []map[string]any:
		return entries, nil
	case []any:
		out := make([]map[string]any, 0, len(entries))
		for _, raw := range entries {
			entry, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s entries must be objects", field)
			}
			out = append(out, entry)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array", field)
	}
}

func jsonPayloadSize(value any) (int, error) {
	wire, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("marshal response payload size: %w", err)
	}
	return len(wire), nil
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
			"parent_evidence_id":     map[string]any{"type": "string"},
			"derivation_method":      map[string]any{"type": "string"},
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
	caseObj := mapAny(rc.state["case"])
	return map[string]any{
		"question":          rc.complaint.Question,
		"judgment_standard": currentJudgmentStandard(rc.state, rc.cfg.Policy),
		"phase":             currentPhase(rc.state),
		"opportunity": map[string]any{
			"id":            opportunity.ID,
			"role":          opportunity.Role,
			"phase":         opportunity.Phase,
			"objective":     opportunity.Objective,
			"allowed_tools": append([]string(nil), opportunity.AllowedTools...),
			"may_pass":      opportunity.MayPass,
		},
		"record": map[string]any{
			"evidence":           rc.listVisibleEvidence(),
			"openings":           cloneJSONLikeMapList(mapList(caseObj["openings"])),
			"arguments":          cloneJSONLikeMapList(mapList(caseObj["arguments"])),
			"rebuttals":          cloneJSONLikeMapList(mapList(caseObj["rebuttals"])),
			"surrebuttals":       cloneJSONLikeMapList(mapList(caseObj["surrebuttals"])),
			"closings":           cloneJSONLikeMapList(mapList(caseObj["closings"])),
			"submitted_evidence": cloneJSONLikeMapList(mapList(caseObj["submitted_evidence"])),
			"exhibits":           rc.attorneyExhibits(),
			"technical_reports":  cloneJSONLikeMapList(mapList(caseObj["technical_reports"])),
		},
		"limits":  limits,
		"council": append([]CouncilSeat(nil), rc.council...),
	}
}

func (rc *runContext) buildAttorneyPrompt(opportunity Opportunity) (string, error) {
	view := rc.attorneyView(opportunity)
	visibleFilesSection := ""
	workProductSection := ""
	if opportunity.Phase == "arguments" || opportunity.Phase == "rebuttals" || opportunity.Phase == "surrebuttals" {
		visibleEvidence, err := marshalIndented("visible evidence", rc.listVisibleEvidence())
		if err != nil {
			return "", err
		}
		visibleFilesSection = "Visible evidence:\n" + visibleEvidence + "\n"
	}
	record, err := marshalIndented("current record", view["record"])
	if err != nil {
		return "", err
	}
	council, err := marshalIndented("council", view["council"])
	if err != nil {
		return "", err
	}
	componentValues := map[string]string{
		"ROLE":           opportunity.Role,
		"PHASE":          opportunity.Phase,
		"OPPORTUNITY_ID": opportunity.ID,
	}
	capabilitiesSection, err := rc.cfg.renderPromptFile(promptAttorneyCapabilities, componentValues)
	if err != nil {
		return "", err
	}
	workspaceSection, err := rc.cfg.renderPromptFile(promptAttorneyWorkspace, componentValues)
	if err != nil {
		return "", err
	}
	limitsSection, err := rc.attorneyLimitsSection(opportunity)
	if err != nil {
		return "", err
	}
	values := map[string]string{
		"ROLE":                       opportunity.Role,
		"PHASE":                      opportunity.Phase,
		"OBJECTIVE":                  opportunity.Objective,
		"OPPORTUNITY_ID":             opportunity.ID,
		"QUESTION":                   rc.complaint.Question,
		"JUDGMENT_STANDARD":          currentJudgmentStandard(rc.state, rc.cfg.Policy),
		"MODEL_CAPABILITIES_SECTION": capabilitiesSection,
		"CURRENT_RECORD":             record,
		"LIMITS_SECTION":             limitsSection,
		"COUNCIL":                    council,
		"VISIBLE_CASE_FILES_SECTION": visibleFilesSection,
		"WORKSPACE_SECTION":          workspaceSection,
		"WORK_PRODUCT_SECTION":       workProductSection,
		"DECISION_TOOLS":             strings.Join(decisionToolEnum(opportunity.AllowedTools), ", "),
	}
	standing, err := rc.cfg.renderPromptFile(promptAttorneyStanding, values)
	if err != nil {
		return "", err
	}
	common, err := rc.cfg.renderPromptFile(promptAttorneyCommon, values)
	if err != nil {
		return "", err
	}
	phaseID, err := attorneyPromptID(opportunity.Phase)
	if err != nil {
		return "", err
	}
	phaseText, err := rc.cfg.renderPromptFile(phaseID, values)
	if err != nil {
		return "", err
	}
	finalize, err := rc.cfg.renderPromptFile(promptAttorneyFinalize, values)
	if err != nil {
		return "", err
	}
	return rc.cfg.renderPromptFile(promptAttorney, map[string]string{
		"ATTORNEY_STANDING": standing,
		"ATTORNEY_COMMON":   common,
		"ATTORNEY_PHASE":    phaseText,
		"ATTORNEY_FINALIZE": finalize,
		"ROLE":              opportunity.Role,
		"PHASE":             opportunity.Phase,
		"OPPORTUNITY_ID":    opportunity.ID,
	})
}

func (rc *runContext) prepareSubmittedEvidence(opportunity Opportunity, params map[string]any) (SubmittedEvidenceMeta, []byte, error) {
	if !evidenceSubmissionAllowed(opportunity) {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence is allowed only in arguments, rebuttals, and surrebuttals"))
	}
	if err := rejectCallerParentSHA(params); err != nil {
		return SubmittedEvidenceMeta{}, nil, err
	}
	if err := requireAllowedKeys(
		params,
		"submit_evidence arguments",
		"title",
		"source_url",
		"source_description",
		"retrieval_timestamp",
		"mime_type",
		"relevance",
		"content",
		"content_base64",
		"preferred_filename_ext",
		"parent_evidence_id",
		"derivation_method",
	); err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	title, err := optionalStringParam(params, "title")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	mimeType, err := optionalStringParam(params, "mime_type")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	relevance, err := optionalStringParam(params, "relevance")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	sourceURL, err := optionalStringParam(params, "source_url")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	sourceDescription, err := optionalStringParam(params, "source_description")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	retrievalTimestamp, err := optionalStringParam(params, "retrieval_timestamp")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	preferredExt, err := optionalStringParam(params, "preferred_filename_ext")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	if title == "" {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence requires title"))
	}
	if sourceURL == "" && sourceDescription == "" {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence requires source_url or source_description"))
	}
	if mimeType == "" {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence requires mime_type"))
	}
	if relevance == "" {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence requires relevance"))
	}
	raw, err := submittedEvidenceContent(params)
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	if len(raw) == 0 {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence content must not be empty"))
	}
	if len(raw) > rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("direct submitted evidence exceeds byte limit of %d", rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes))
	}
	if len(raw) > rc.cfg.Policy.MaxSubmittedEvidenceBytes {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted evidence exceeds byte limit of %d", rc.cfg.Policy.MaxSubmittedEvidenceBytes))
	}
	if submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedEvidencePerSide {
		return SubmittedEvidenceMeta{}, nil, participantInput(fmt.Errorf("submitted_evidence for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedEvidencePerSide))
	}
	parentEvidenceIDArg, err := optionalStringParam(params, "parent_evidence_id")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	derivationMethodArg, err := optionalStringParam(params, "derivation_method")
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	name := submittedEvidenceFilename(len(rc.submittedEvidence)+1, opportunity.Role, sha, mimeType, preferredExt)
	meta := SubmittedEvidenceMeta{
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
	}
	parentEvidenceID, parentSHA256, derivationMethod, err := rc.resolveEvidenceLineage(
		parentEvidenceIDArg,
		derivationMethodArg,
		meta.EvidenceID,
	)
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, err
	}
	meta.ParentEvidenceID = parentEvidenceID
	meta.ParentSHA256 = parentSHA256
	meta.DerivationMethod = derivationMethod
	if err := rc.validateSubmittedEvidenceID(meta.EvidenceID); err != nil {
		return SubmittedEvidenceMeta{}, nil, participantInput(err)
	}
	return meta, raw, nil
}

func submittedEvidenceContent(params map[string]any) ([]byte, error) {
	rawContent, hasContent := params["content"]
	content, contentOK := rawContent.(string)
	if hasContent && !contentOK {
		return nil, fmt.Errorf("content must be a string")
	}
	contentBase64, err := optionalStringParam(params, "content_base64")
	if err != nil {
		return nil, err
	}
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
	if meta.ParentEvidenceID != "" || meta.ParentSHA256 != "" || meta.DerivationMethod != "" {
		payload["parent_evidence_id"] = meta.ParentEvidenceID
		payload["parent_sha256"] = meta.ParentSHA256
		payload["derivation_method"] = meta.DerivationMethod
	}
	return payload
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

func (rc *runContext) attorneyLimitsSection(opportunity Opportunity) (string, error) {
	limits := rc.attorneyLimits(opportunity)
	values := map[string]string{
		"ROLE":           opportunity.Role,
		"PHASE":          opportunity.Phase,
		"OPPORTUNITY_ID": opportunity.ID,
	}
	textLimitsSection := ""
	if limit, _ := limits["text_char_limit"].(int); limit > 0 {
		values["TEXT_CHAR_LIMIT"] = fmt.Sprintf("%d", limit)
		values["TARGET_TEXT_CHAR_LIMIT"] = fmt.Sprintf("%d", targetSubmissionCharLimit(limit))
		var err error
		textLimitsSection, err = rc.cfg.renderPromptFile(promptAttorneyTextLimits, values)
		if err != nil {
			return "", err
		}
	}
	evidenceLimitsSection := ""
	switch opportunity.Phase {
	case "arguments", "rebuttals", "surrebuttals":
		for token, key := range map[string]string{
			"MAX_EXHIBITS_PER_FILING":                 "max_exhibits_per_filing",
			"USED_EXHIBITS_FOR_SIDE":                  "used_exhibits_for_side",
			"MAX_EXHIBITS_PER_SIDE":                   "max_exhibits_per_side",
			"REMAINING_EXHIBITS_FOR_SIDE":             "remaining_exhibits_for_side",
			"MAX_REPORTS_PER_FILING":                  "max_reports_per_filing",
			"USED_REPORTS_FOR_SIDE":                   "used_reports_for_side",
			"MAX_REPORTS_PER_SIDE":                    "max_reports_per_side",
			"REMAINING_REPORTS_FOR_SIDE":              "remaining_reports_for_side",
			"MAX_SUBMITTED_EVIDENCE_BYTES":            "max_submitted_evidence_bytes",
			"MAX_DIRECT_SUBMITTED_EVIDENCE_BYTES":     "max_direct_submitted_evidence_bytes",
			"MAX_EVIDENCE_UPLOAD_BYTES":               "max_evidence_upload_bytes",
			"MAX_EVIDENCE_CHUNK_BYTES":                "max_evidence_chunk_bytes",
			"USED_SUBMITTED_EVIDENCE_FOR_SIDE":        "used_submitted_evidence_for_side",
			"MAX_SUBMITTED_EVIDENCE_PER_SIDE":         "max_submitted_evidence_per_side",
			"REMAINING_SUBMITTED_EVIDENCE_FOR_SIDE":   "remaining_submitted_evidence_for_side",
			"MAX_EVIDENCE_READ_BYTES":                 "max_evidence_read_bytes",
			"MAX_EVIDENCE_READS_PER_OPPORTUNITY":      "max_evidence_reads_per_opportunity",
			"MAX_EVIDENCE_READ_BYTES_PER_OPPORTUNITY": "max_evidence_read_bytes_per_opportunity",
		} {
			values[token] = fmt.Sprintf("%d", limits[key].(int))
		}
		var err error
		evidenceLimitsSection, err = rc.cfg.renderPromptFile(promptAttorneyEvidenceLimits, values)
		if err != nil {
			return "", err
		}
	}
	values["TEXT_LIMITS_SECTION"] = textLimitsSection
	values["EVIDENCE_LIMITS_SECTION"] = evidenceLimitsSection
	return rc.cfg.renderPromptFile(promptAttorneyLimits, values)
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
	text, _ := payload["text"].(string)
	text = trimLeanWhitespace(text)
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

func marshalIndented(label string, value any) (string, error) {
	wire, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", label, err)
	}
	return string(wire), nil
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
