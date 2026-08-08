package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (r *Runner) persistActionEvent(
	turnIndex int,
	stepIndex int,
	actorRole string,
	actionType string,
	payload map[string]any,
	res map[string]any,
) error {
	if err := r.store.AppendEvent(r.cfg.RunID, turnIndex, stepIndex, actorRole, actionType, payload, res); err != nil {
		return err
	}
	if r.cfg.EventsPath == "" {
		return nil
	}
	line := map[string]any{
		"timestamp": time.Now().Format("2006-01-02 15:04:05.000"),
		"turn":      turnIndex,
		"step":      stepIndex,
		"role":      actorRole,
		"action":    actionType,
		"payload":   payload,
		"response":  res,
	}
	return appendEventLine(r.cfg.EventsPath, line)
}

func (r *Runner) persistAgentEvent(
	turnIndex int,
	sequence int,
	actorRole string,
	eventType string,
	payload map[string]any,
) error {
	stepIndex := -sequence
	if err := r.store.AppendEvent(r.cfg.RunID, turnIndex, stepIndex, actorRole, eventType, payload, map[string]any{}); err != nil {
		return err
	}
	if r.cfg.EventsPath == "" {
		return nil
	}
	line := map[string]any{
		"timestamp":   time.Now().Format("2006-01-02 15:04:05.000"),
		"turn":        turnIndex,
		"step":        stepIndex,
		"role":        actorRole,
		"agent_event": eventType,
		"payload":     payload,
	}
	return appendEventLine(r.cfg.EventsPath, line)
}

func (r *Runner) persistAgentCompletionResult(
	turnIndex int,
	sequence int,
	actorRole string,
	payload map[string]any,
) error {
	return r.persistAgentEvent(turnIndex, sequence, actorRole, "agent_completion_result", payload)
}

func appendEventLine(path string, line map[string]any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events path: %w", err)
	}
	enc, err := json.Marshal(line)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("marshal events line: %w", err)
	}
	if _, err := f.Write(append(enc, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write events path: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close events path: %w", err)
	}
	return nil
}

func resetEventLog(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create events directory: %w", err)
		}
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return fmt.Errorf("reset events path: %w", err)
	}
	return nil
}

func (r *Runner) writeEvidenceManifest() error {
	if r.cfg.OutputPath == "" {
		return nil
	}
	outputDir := filepath.Dir(r.cfg.OutputPath)
	caseObj, _ := r.state["case"].(map[string]any)
	if caseObj == nil {
		return fmt.Errorf("state.case missing")
	}
	files, _ := caseObj["case_files"].([]any)
	evidence := make([]map[string]any, 0, len(files))
	for _, raw := range files {
		fileObj, _ := raw.(map[string]any)
		if fileObj == nil {
			continue
		}
		item, err := r.evidenceManifestItem(outputDir, caseObj, fileObj)
		if err != nil {
			return err
		}
		evidence = append(evidence, item)
	}
	manifest := map[string]any{
		"schema_version": "adc.evidence-manifest.v0",
		"created_at":     time.Now().UTC().Format(time.RFC3339),
		"evidence":       evidence,
		"evidence_count": len(evidence),
	}
	if err := writeJSONFileAtomic(filepath.Join(outputDir, "evidence-manifest.json"), manifest); err != nil {
		return fmt.Errorf("write evidence manifest: %w", err)
	}
	return nil
}

func (r *Runner) evidenceManifestItem(outputDir string, caseObj map[string]any, fileObj map[string]any) (map[string]any, error) {
	fileID := strings.TrimSpace(stringOrDefault(fileObj["file_id"], ""))
	if fileID == "" {
		return nil, fmt.Errorf("case file missing file_id")
	}
	storedPath := strings.TrimSpace(stringOrDefault(fileObj["storage_relpath"], ""))
	if storedPath == "" {
		return nil, fmt.Errorf("case file %s has no storage_relpath", fileID)
	}
	src := resolveStoredCaseFilePath(storedPath, r.cfg.ScenarioBaseDir)
	name := manifestCaseFileName(fileObj)
	if err := copyFileAtomic(src, filepath.Join(outputDir, "submitted-evidence", name)); err != nil {
		return nil, fmt.Errorf("copy case file %s into submitted evidence: %w", fileID, err)
	}
	item := map[string]any{
		"evidence_id":     fileID,
		"file_id":         fileID,
		"name":            name,
		"original_name":   strings.TrimSpace(stringOrDefault(fileObj["original_name"], "")),
		"label":           strings.TrimSpace(stringOrDefault(fileObj["label"], "")),
		"storage_relpath": storedPath,
		"sha256":          strings.TrimSpace(stringOrDefault(fileObj["sha256"], "")),
		"size_bytes":      intFromAny(fileObj["size_bytes"]),
		"mime_type":       caseFileMIMEType(fileObj),
		"text_readable":   isReadableCaseTextExtension(caseFileExtension(fileObj)),
		"uses":            caseFileUses(caseObj, fileID),
	}
	if importedAt := strings.TrimSpace(stringOrDefault(fileObj["imported_at"], "")); importedAt != "" {
		item["imported_at"] = importedAt
	}
	if importedBy := strings.TrimSpace(stringOrDefault(fileObj["imported_by"], "")); importedBy != "" {
		item["imported_by"] = importedBy
	}
	return item, nil
}

func manifestCaseFileName(fileObj map[string]any) string {
	fileID := strings.TrimSpace(stringOrDefault(fileObj["file_id"], "case-file"))
	name := strings.TrimSpace(stringOrDefault(fileObj["original_name"], ""))
	if name == "" {
		name = strings.TrimSpace(stringOrDefault(fileObj["label"], ""))
	}
	if name == "" {
		name = strings.TrimSpace(filepath.Base(stringOrDefault(fileObj["storage_relpath"], "")))
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "case-file.bin"
	}
	return fileID + "-" + sanitizeUploadedCaseFilename(name)
}

func writeJSONFileAtomic(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write([]byte("\n")); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func copyFileAtomic(src string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (r *Runner) writeEvidence(result Result) error {
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	if r.cfg.OutputPath == "" {
		return nil
	}
	if err := os.WriteFile(r.cfg.OutputPath, raw, 0o644); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}
	if err := writeJSONFileAtomic(filepath.Join(filepath.Dir(r.cfg.OutputPath), "state.json"), result.FinalState); err != nil {
		return fmt.Errorf("write final state: %w", err)
	}
	if err := r.writeReplayCertificate(result); err != nil {
		return fmt.Errorf("write replay certificate: %w", err)
	}
	if err := exportExternalWorkProduct(filepath.Dir(r.cfg.OutputPath), r.workProductDirs); err != nil {
		return err
	}
	if err := r.writeEvidenceManifest(); err != nil {
		return err
	}
	return nil
}

func exportExternalWorkProduct(outputDir string, workProductDirs map[string]string) error {
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
