package runner

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func publicJurorRecord(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	return map[string]any{
		"juror_id": stringOrDefault(raw["juror_id"], ""),
		"name":     stringOrDefault(raw["name"], ""),
		"status":   stringOrDefault(raw["status"], ""),
	}
}

func filterJurorQuestionnaireResponses(responses []any, jurorID string) []any {
	filtered := make([]any, 0)
	for _, raw := range responses {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(entry["juror_id"], "")) == strings.TrimSpace(jurorID) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterVoirDireExchanges(exchanges []any, jurorID string) []any {
	filtered := make([]any, 0)
	for _, raw := range exchanges {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(entry["juror_id"], "")) == strings.TrimSpace(jurorID) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterForCauseChallenges(challenges []any, jurorID string) []any {
	filtered := make([]any, 0)
	for _, raw := range challenges {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(entry["juror_id"], "")) == strings.TrimSpace(jurorID) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func sanitizeUploadedCaseFilename(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "uploaded-file.bin"
	}
	return name
}

func (r *Runner) uploadedCaseFilesDir() string {
	if strings.TrimSpace(r.cfg.OutputPath) != "" {
		return filepath.Join(filepath.Dir(r.cfg.OutputPath), "uploaded-case-files")
	}
	return filepath.Join(r.cfg.ScenarioBaseDir, "uploaded-case-files")
}

func (r *Runner) storeUploadedCaseFile(fileID, originalName string, raw []byte) (string, string, error) {
	dir := r.uploadedCaseFilesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create uploaded case files dir: %w", err)
	}
	storedName := fileID + "-" + sanitizeUploadedCaseFilename(originalName)
	absPath := filepath.Join(dir, storedName)
	if err := os.WriteFile(absPath, raw, 0o644); err != nil {
		return "", "", fmt.Errorf("write uploaded case file: %w", err)
	}
	return absPath, storedName, nil
}

func uploadedCaseFilePayload(payload map[string]any) (string, string, []byte, error) {
	originalName := strings.TrimSpace(stringOrDefault(payload["original_name"], ""))
	if originalName == "" {
		return "", "", nil, fmt.Errorf("original_name is required when importing uploaded file content")
	}
	contentBase64 := strings.TrimSpace(stringOrDefault(payload["content_base64"], ""))
	if contentBase64 == "" {
		return "", "", nil, fmt.Errorf("content_base64 is required when importing uploaded file content")
	}
	raw, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return "", "", nil, fmt.Errorf("decode content_base64: %w", err)
	}
	return originalName, strings.TrimSpace(stringOrDefault(payload["label"], "")), raw, nil
}

func (r *Runner) visibleCaseForRole(actorRole string) (map[string]any, error) {
	resp, err := r.lean.View(r.state, actorRole)
	if err != nil {
		return nil, err
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return nil, fmt.Errorf("role view failed for %s: %s", actorRole, strings.TrimSpace(stringOrDefault(resp["error"], "unknown error")))
	}
	view, _ := resp["view"].(map[string]any)
	if view == nil {
		return nil, fmt.Errorf("role view missing payload for %s", actorRole)
	}
	stateObj, _ := view["state"].(map[string]any)
	if stateObj == nil {
		return nil, fmt.Errorf("role view missing state for %s", actorRole)
	}
	caseObj, _ := stateObj["case"].(map[string]any)
	if caseObj == nil {
		return nil, fmt.Errorf("role view missing case for %s", actorRole)
	}
	return caseObj, nil
}

func (r *Runner) visibleCaseFilesForRole(actorRole string) ([]any, error) {
	caseObj, err := r.visibleCaseForRole(actorRole)
	if err != nil {
		return nil, err
	}
	caseFiles, _ := caseObj["case_files"].([]any)
	return caseFiles, nil
}

func unknownCaseFileResult(fileID string, visibleFiles []any) map[string]any {
	choices := make([]string, 0, len(visibleFiles))
	for _, entry := range visibleFiles {
		fileObj, _ := entry.(map[string]any)
		if fileObj == nil {
			continue
		}
		summary := summarizeCaseFileChoice(fileObj)
		if summary == "" {
			continue
		}
		choices = append(choices, summary)
	}
	actorMessage := fmt.Sprintf("Use a case file identifier, not a filename. %q is not a known file_id.", fileID)
	if len(choices) > 0 {
		actorMessage += " Available file_id values: " + strings.Join(choices, ", ") + "."
	}
	return map[string]any{
		"ok":            false,
		"error":         "unknown file_id: " + fileID,
		"code":          "UNKNOWN_CASE_FILE_ID",
		"details":       map[string]any{"file_id": fileID},
		"actor_message": actorMessage,
	}
}

func visibleCaseFileByID(visibleFiles []any, fileID string) map[string]any {
	for _, entry := range visibleFiles {
		fileObj, _ := entry.(map[string]any)
		if fileObj == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(fileObj["file_id"], "")) == fileID {
			return fileObj
		}
	}
	return nil
}

func caseFileExtension(fileObj map[string]any) string {
	name := strings.TrimSpace(stringOrDefault(fileObj["original_name"], ""))
	if name == "" {
		name = strings.TrimSpace(stringOrDefault(fileObj["label"], ""))
	}
	if name == "" {
		name = strings.TrimSpace(stringOrDefault(fileObj["storage_relpath"], ""))
	}
	return strings.ToLower(filepath.Ext(name))
}

func isReadableCaseTextExtension(extension string) bool {
	switch strings.ToLower(strings.TrimSpace(extension)) {
	case ".md", ".txt", ".pem", ".b64":
		return true
	default:
		return false
	}
}

func caseFileMIMEType(fileObj map[string]any) string {
	extension := caseFileExtension(fileObj)
	if extension == "" {
		return ""
	}
	return strings.TrimSpace(mime.TypeByExtension(extension))
}

func caseFileUses(caseObj map[string]any, fileID string) []string {
	seen := map[string]bool{}
	uses := make([]string, 0)
	fileEvents, _ := caseObj["file_events"].([]any)
	for _, raw := range fileEvents {
		event, _ := raw.(map[string]any)
		if event == nil || strings.TrimSpace(stringOrDefault(event["file_id"], "")) != fileID {
			continue
		}
		action := strings.TrimSpace(stringOrDefault(event["action"], ""))
		details := strings.TrimSpace(stringOrDefault(event["details"], ""))
		switch action {
		case "filed_with_complaint":
			uses = appendIfMissing(uses, "complaint_attachment")
			seen["complaint_attachment"] = true
		case "produce_case_file":
			uses = appendIfMissing(uses, "produced")
			seen["produced"] = true
		case "offer_case_file_as_exhibit":
			exhibitID := ""
			for _, field := range strings.Fields(details) {
				if strings.HasPrefix(field, "exhibit_id=") {
					exhibitID = strings.TrimPrefix(field, "exhibit_id=")
					break
				}
			}
			if exhibitID == "" {
				uses = appendIfMissing(uses, "exhibit")
				seen["exhibit"] = true
				continue
			}
			offered := "exhibit:" + exhibitID
			uses = appendIfMissing(uses, offered)
			seen[offered] = true
			if strings.Contains(details, "admitted=true") {
				admitted := "admitted_exhibit:" + exhibitID
				uses = appendIfMissing(uses, admitted)
				seen[admitted] = true
			}
		}
	}
	reports, _ := caseObj["technical_reports"].([]any)
	for _, raw := range reports {
		report, _ := raw.(map[string]any)
		if report == nil || strings.TrimSpace(stringOrDefault(report["file_id"], "")) != fileID {
			continue
		}
		reportID := strings.TrimSpace(stringOrDefault(report["report_id"], ""))
		if reportID == "" {
			uses = appendIfMissing(uses, "technical_report")
			continue
		}
		uses = appendIfMissing(uses, "technical_report:"+reportID)
	}
	return uses
}

func countExhibitsOfferedByParty(caseObj map[string]any, party string) int {
	fileEvents, _ := caseObj["file_events"].([]any)
	count := 0
	for _, raw := range fileEvents {
		event, _ := raw.(map[string]any)
		if event == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(event["action"], "")) != "offer_case_file_as_exhibit" {
			continue
		}
		if strings.TrimSpace(stringOrDefault(event["actor"], "")) != party {
			continue
		}
		count++
	}
	return count
}

func nextExhibitIDForParty(caseObj map[string]any, party string) string {
	prefix := "X"
	switch strings.TrimSpace(party) {
	case "plaintiff":
		prefix = "PX"
	case "defendant":
		prefix = "DX"
	}
	return fmt.Sprintf("%s-%d", prefix, countExhibitsOfferedByParty(caseObj, party)+1)
}

func caseFileAlreadyOfferedByParty(caseObj map[string]any, party, fileID string) bool {
	fileEvents, _ := caseObj["file_events"].([]any)
	for _, raw := range fileEvents {
		event, _ := raw.(map[string]any)
		if event == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(event["action"], "")) != "offer_case_file_as_exhibit" {
			continue
		}
		if strings.TrimSpace(stringOrDefault(event["actor"], "")) != party {
			continue
		}
		if strings.TrimSpace(stringOrDefault(event["file_id"], "")) != fileID {
			continue
		}
		return true
	}
	return false
}

func enrichVisibleCaseFile(caseObj map[string]any, visibleFile map[string]any) map[string]any {
	enriched := cloneMap(visibleFile)
	fileID := strings.TrimSpace(stringOrDefault(visibleFile["file_id"], ""))
	internalFile := findCaseFile(caseObj, fileID)
	enriched["extension"] = caseFileExtension(internalFile)
	enriched["mime_type"] = caseFileMIMEType(internalFile)
	enriched["uses"] = caseFileUses(caseObj, fileID)
	return enriched
}

func caseFileAttachmentContent(fileObj map[string]any, raw []byte) ([]map[string]any, error) {
	filename := strings.TrimSpace(stringOrDefault(fileObj["original_name"], ""))
	if filename == "" {
		filename = strings.TrimSpace(stringOrDefault(fileObj["label"], ""))
	}
	if filename == "" {
		filename = strings.TrimSpace(stringOrDefault(fileObj["file_id"], ""))
	}
	if filename == "" {
		filename = "case-file"
	}
	extension := caseFileExtension(fileObj)
	mimeType := caseFileMIMEType(fileObj)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	summary := "Requested case file " + filename + ". Review it and continue with the current opportunity."
	items := []map[string]any{
		{
			"type": "input_text",
			"text": summary,
		},
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	switch extension {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		items = append(items, map[string]any{
			"type":      "input_image",
			"detail":    "auto",
			"image_url": "data:" + mimeType + ";base64," + encoded,
		})
	default:
		items = append(items, map[string]any{
			"type":      "input_file",
			"file_data": encoded,
			"filename":  filename,
		})
	}
	return items, nil
}

func resolveStoredCaseFilePath(storageRelPath string, scenarioBaseDir string) string {
	if filepath.IsAbs(storageRelPath) {
		return filepath.Clean(storageRelPath)
	}
	return resolveScenarioRelativePath(storageRelPath, scenarioBaseDir)
}

func jurorContextPayload(caseObj map[string]any, jurorID string) map[string]any {
	var selectedJuror map[string]any
	jurors, _ := caseObj["jurors"].([]any)
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if strings.TrimSpace(stringOrDefault(juror["juror_id"], "")) == strings.TrimSpace(jurorID) {
			selectedJuror = juror
			break
		}
	}
	questionnaireResponses, _ := caseObj["juror_questionnaire_responses"].([]any)
	voirDireExchanges, _ := caseObj["voir_dire_exchanges"].([]any)
	forCauseChallenges, _ := caseObj["for_cause_challenges"].([]any)
	return map[string]any{
		"juror_id":                      jurorID,
		"juror":                         publicJurorRecord(selectedJuror),
		"juror_questionnaire":           caseObj["juror_questionnaire"],
		"juror_questionnaire_responses": filterJurorQuestionnaireResponses(questionnaireResponses, jurorID),
		"voir_dire_exchanges":           filterVoirDireExchanges(voirDireExchanges, jurorID),
		"for_cause_challenges":          filterForCauseChallenges(forCauseChallenges, jurorID),
	}
}

func (r *Runner) executeLocalAction(actorRole, actionType string, payload map[string]any) (ActionExecution, bool, error) {
	caseObj, _ := r.state["case"].(map[string]any)
	if caseObj == nil {
		return ActionExecution{}, false, fmt.Errorf("state.case missing")
	}
	switch actionType {
	case "get_case":
		visibleCase, err := r.visibleCaseForRole(actorRole)
		if err != nil {
			return ActionExecution{}, true, err
		}
		return ActionExecution{Result: map[string]any{"ok": true, "case": visibleCase}}, true, nil
	case "explain_decisions":
		traces, _ := caseObj["decision_traces"].([]any)
		return ActionExecution{Result: map[string]any{"ok": true, "decision_traces": traces}}, true, nil
	case "import_case_file":
		importedBy := actorRole
		label, _ := payload["label"].(string)
		sourceFilename, _ := payload["source_filename"].(string)
		var (
			raw          []byte
			sourcePath   string
			originalName string
		)
		if strings.TrimSpace(sourceFilename) != "" {
			sourcePath = sourceFilename
			if !filepath.IsAbs(sourcePath) {
				sourcePath = resolveScenarioRelativePath(sourceFilename, r.cfg.ScenarioBaseDir)
			}
			info, err := os.Stat(sourcePath)
			if err != nil || info.IsDir() {
				return ActionExecution{Result: map[string]any{"ok": false, "error": "source filename must be a regular file"}}, true, nil
			}
			raw, err = os.ReadFile(sourcePath)
			if err != nil {
				return ActionExecution{}, true, fmt.Errorf("read source file: %w", err)
			}
			originalName = filepath.Base(sourcePath)
		} else {
			name, uploadLabel, uploadRaw, err := uploadedCaseFilePayload(payload)
			if err != nil {
				return ActionExecution{Result: map[string]any{
					"ok":            false,
					"error":         err.Error(),
					"actor_message": "To import a new file, submit original_name and base64-encoded content in content_base64. Do not refer to a host path.",
				}}, true, nil
			}
			originalName = name
			raw = uploadRaw
			if strings.TrimSpace(label) == "" {
				label = uploadLabel
			}
		}
		digest := sha256.Sum256(raw)
		caseFiles, _ := caseObj["case_files"].([]any)
		fileID := fmt.Sprintf("file-%04d", len(caseFiles)+1)
		if strings.TrimSpace(sourcePath) == "" {
			storedPath, _, err := r.storeUploadedCaseFile(fileID, originalName, raw)
			if err != nil {
				return ActionExecution{}, true, err
			}
			sourcePath = storedPath
		}
		record := map[string]any{
			"file_id":         fileID,
			"imported_at":     time.Now().UTC().Format(time.RFC3339),
			"imported_by":     importedBy,
			"label":           strings.TrimSpace(label),
			"original_name":   originalName,
			"storage_relpath": sourcePath,
			"sha256":          hex.EncodeToString(digest[:]),
			"size_bytes":      len(raw),
		}
		leanRes, err := r.stepForCertificate("import_case_file", actorRole, record)
		if err != nil {
			return ActionExecution{}, true, err
		}
		if ok, _ := leanRes["ok"].(bool); ok {
			nextState, _ := leanRes["state"].(map[string]any)
			if nextState == nil {
				return ActionExecution{}, true, fmt.Errorf("lean response missing state for import_case_file")
			}
			r.state = nextState
			leanRes["state"] = nextState
			leanRes["file"] = record
			if err := r.writeEvidenceManifest(); err != nil {
				return ActionExecution{}, true, err
			}
		}
		return ActionExecution{Result: leanRes}, true, nil
	case "list_case_files":
		caseFiles, err := r.visibleCaseFilesForRole(actorRole)
		if err != nil {
			return ActionExecution{}, true, err
		}
		files := make([]any, 0, len(caseFiles))
		for _, entry := range caseFiles {
			fileObj, _ := entry.(map[string]any)
			if fileObj == nil {
				continue
			}
			files = append(files, enrichVisibleCaseFile(caseObj, fileObj))
		}
		return ActionExecution{Result: map[string]any{"ok": true, "files": files}}, true, nil
	case "read_case_text_file":
		fileID := strings.TrimSpace(stringOrDefault(payload["file_id"], ""))
		if fileID == "" {
			return ActionExecution{Result: map[string]any{"ok": false, "error": "file_id is required"}}, true, nil
		}
		visibleFiles, err := r.visibleCaseFilesForRole(actorRole)
		if err != nil {
			return ActionExecution{}, true, err
		}
		visibleFile := visibleCaseFileByID(visibleFiles, fileID)
		if visibleFile == nil {
			return ActionExecution{Result: unknownCaseFileResult(fileID, visibleFiles)}, true, nil
		}
		internalFile := findCaseFile(caseObj, fileID)
		if internalFile == nil {
			return ActionExecution{}, true, fmt.Errorf("internal case file missing for visible file_id=%s", fileID)
		}
		extension := caseFileExtension(internalFile)
		if !isReadableCaseTextExtension(extension) {
			return ActionExecution{Result: map[string]any{
				"ok":            false,
				"error":         fmt.Sprintf("read_case_text_file only supports .md, .txt, .pem, and .b64 files; got %s", extension),
				"code":          "UNSUPPORTED_CASE_TEXT_EXTENSION",
				"details":       map[string]any{"file_id": fileID, "extension": extension},
				"actor_message": fmt.Sprintf("read_case_text_file only supports .md, .txt, .pem, and .b64 files. %s has extension %s.", summarizeCaseFileChoice(enrichVisibleCaseFile(caseObj, visibleFile)), extension),
			}}, true, nil
		}
		storedPath := strings.TrimSpace(stringOrDefault(internalFile["storage_relpath"], ""))
		if storedPath == "" {
			return ActionExecution{Result: map[string]any{
				"ok":            false,
				"error":         "stored path missing for case file",
				"code":          "CASE_FILE_PATH_MISSING",
				"details":       map[string]any{"file_id": fileID},
				"actor_message": "This case file has no stored path and cannot be read.",
			}}, true, nil
		}
		resolvedPath := resolveStoredCaseFilePath(storedPath, r.cfg.ScenarioBaseDir)
		raw, err := os.ReadFile(resolvedPath)
		if err != nil {
			return ActionExecution{}, true, fmt.Errorf("read case file %s: %w", fileID, err)
		}
		if !utf8.Valid(raw) {
			return ActionExecution{Result: map[string]any{
				"ok":            false,
				"error":         "case file is not valid UTF-8 text",
				"code":          "CASE_FILE_NOT_UTF8",
				"details":       map[string]any{"file_id": fileID},
				"actor_message": fmt.Sprintf("%s could not be read as UTF-8 text.", summarizeCaseFileChoice(enrichVisibleCaseFile(caseObj, visibleFile))),
			}}, true, nil
		}
		return ActionExecution{Result: map[string]any{
			"ok":   true,
			"file": enrichVisibleCaseFile(caseObj, visibleFile),
			"text": string(raw),
		}}, true, nil
	case "request_case_file":
		fileID := strings.TrimSpace(stringOrDefault(payload["file_id"], ""))
		if fileID == "" {
			return ActionExecution{Result: map[string]any{"ok": false, "error": "file_id is required"}}, true, nil
		}
		visibleFiles, err := r.visibleCaseFilesForRole(actorRole)
		if err != nil {
			return ActionExecution{}, true, err
		}
		visibleFile := visibleCaseFileByID(visibleFiles, fileID)
		if visibleFile == nil {
			return ActionExecution{Result: unknownCaseFileResult(fileID, visibleFiles)}, true, nil
		}
		internalFile := findCaseFile(caseObj, fileID)
		if internalFile == nil {
			return ActionExecution{}, true, fmt.Errorf("internal case file missing for visible file_id=%s", fileID)
		}
		storedPath := strings.TrimSpace(stringOrDefault(internalFile["storage_relpath"], ""))
		if storedPath == "" {
			return ActionExecution{Result: map[string]any{
				"ok":            false,
				"error":         "stored path missing for case file",
				"code":          "CASE_FILE_PATH_MISSING",
				"details":       map[string]any{"file_id": fileID},
				"actor_message": "This case file has no stored path and cannot be attached.",
			}}, true, nil
		}
		resolvedPath := resolveStoredCaseFilePath(storedPath, r.cfg.ScenarioBaseDir)
		raw, err := os.ReadFile(resolvedPath)
		if err != nil {
			return ActionExecution{}, true, fmt.Errorf("read case file %s: %w", fileID, err)
		}
		contentItems, err := caseFileAttachmentContent(internalFile, raw)
		if err != nil {
			return ActionExecution{}, true, err
		}
		return ActionExecution{
			Result: map[string]any{
				"ok":       true,
				"file":     enrichVisibleCaseFile(caseObj, visibleFile),
				"attached": true,
			},
			FollowupInputItems: []map[string]any{
				{
					"role":          "user",
					"content_items": contentItems,
				},
			},
		}, true, nil
	case "produce_case_file":
		fileID, _ := payload["file_id"].(string)
		producedBy := actorRole
		producedTo := stringOrDefault(payload["produced_to"], "")
		if strings.TrimSpace(fileID) == "" || strings.TrimSpace(producedTo) == "" {
			return ActionExecution{Result: map[string]any{"ok": false, "error": "file_id and produced_to are required"}}, true, nil
		}
		leanPayload := map[string]any{
			"file_id":     fileID,
			"produced_by": producedBy,
			"produced_to": producedTo,
			"produced_at": time.Now().UTC().Format(time.RFC3339),
		}
		if requestRef, _ := payload["request_ref"].(string); strings.TrimSpace(requestRef) != "" {
			leanPayload["request_ref"] = requestRef
		}
		leanRes, err := r.stepForCertificate("produce_case_file", actorRole, leanPayload)
		if err != nil {
			return ActionExecution{}, true, err
		}
		if ok, _ := leanRes["ok"].(bool); ok {
			nextState, _ := leanRes["state"].(map[string]any)
			if nextState == nil {
				return ActionExecution{}, true, fmt.Errorf("lean response missing state for produce_case_file")
			}
			r.state = nextState
			leanRes["state"] = nextState
			if err := r.writeEvidenceManifest(); err != nil {
				return ActionExecution{}, true, err
			}
		}
		return ActionExecution{Result: leanRes}, true, nil
	case "offer_case_file_as_exhibit":
		fileID, _ := payload["file_id"].(string)
		party := actorRole
		exhibitID, _ := payload["exhibit_id"].(string)
		if strings.TrimSpace(fileID) == "" || strings.TrimSpace(party) == "" {
			return ActionExecution{Result: map[string]any{"ok": false, "error": "file_id is required"}}, true, nil
		}
		if !hasCaseFile(caseObj, fileID) {
			visibleFiles, err := r.visibleCaseFilesForRole(actorRole)
			if err != nil {
				return ActionExecution{}, true, err
			}
			return ActionExecution{Result: unknownCaseFileResult(fileID, visibleFiles)}, true, nil
		}
		if caseFileAlreadyOfferedByParty(caseObj, party, fileID) {
			return ActionExecution{Result: map[string]any{
				"ok":            false,
				"error":         "file already offered by party",
				"code":          "FILE_ALREADY_OFFERED",
				"details":       map[string]any{"file_id": fileID, "party": party},
				"actor_message": "Choose a case file that this side has not already offered as an exhibit.",
			}}, true, nil
		}
		if strings.TrimSpace(exhibitID) == "" {
			exhibitID = nextExhibitIDForParty(caseObj, party)
		}
		admitted := true
		if raw, ok := payload["admitted"].(bool); ok {
			admitted = raw
		}
		description, _ := payload["description"].(string)
		if strings.TrimSpace(description) == "" {
			fileObj := findCaseFile(caseObj, fileID)
			if fileObj != nil {
				name := caseFileDisplayName(fileObj)
				if name == "" || name == fileID {
					description = fileID
				} else {
					description = name + " (" + fileID + ")"
				}
			} else {
				description = "Case file " + fileID
			}
		}
		leanPayload := map[string]any{
			"party":       party,
			"file_id":     fileID,
			"exhibit_id":  exhibitID,
			"description": description,
			"admitted":    admitted,
			"offered_at":  time.Now().UTC().Format(time.RFC3339),
		}
		leanRes, err := r.stepForCertificate("offer_exhibit", actorRole, leanPayload)
		if err != nil {
			return ActionExecution{}, true, err
		}
		if ok, _ := leanRes["ok"].(bool); ok {
			nextState, _ := leanRes["state"].(map[string]any)
			if nextState == nil {
				return ActionExecution{}, true, fmt.Errorf("lean response missing state for offer_exhibit")
			}
			r.state = nextState
			leanRes["state"] = nextState
			if err := r.writeEvidenceManifest(); err != nil {
				return ActionExecution{}, true, err
			}
		}
		return ActionExecution{Result: leanRes}, true, nil
	case "rest_case":
		leanRes, err := r.stepForCertificate("rest_case", actorRole, map[string]any{})
		if err != nil {
			return ActionExecution{}, true, err
		}
		if ok, _ := leanRes["ok"].(bool); ok {
			nextState, _ := leanRes["state"].(map[string]any)
			if nextState == nil {
				return ActionExecution{}, true, fmt.Errorf("lean response missing state for rest_case")
			}
			r.state = nextState
			leanRes["state"] = nextState
		}
		return ActionExecution{Result: leanRes}, true, nil
	case "get_juror_context":
		jurorID := stringOrDefault(payload["juror_id"], "")
		ctx := jurorContextPayload(caseObj, jurorID)
		return ActionExecution{Result: map[string]any{"ok": true, "context": ctx}}, true, nil
	default:
		return ActionExecution{}, false, nil
	}
}

func resolveScenarioRelativePath(sourceFilename, scenarioBaseDir string) string {
	cleaned := filepath.Clean(sourceFilename)
	if _, err := os.Stat(cleaned); err == nil {
		return cleaned
	}
	base := filepath.Clean(scenarioBaseDir)
	if !filepath.IsAbs(base) {
		if absBase, err := filepath.Abs(base); err == nil {
			base = absBase
		}
	}
	for {
		candidate := filepath.Clean(filepath.Join(base, sourceFilename))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		next := filepath.Dir(base)
		if next == base {
			break
		}
		base = next
	}
	return cleaned
}
