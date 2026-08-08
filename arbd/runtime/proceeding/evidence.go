package proceeding

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const evidenceManifestSchemaVersion = "aar.evidence-manifest.v0"
const evidenceIDPrefix = "ev_"

type evidenceReadBudget struct {
	bytes int
	reads int
}

func utcTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func evidenceIDForFile(sha string, evidenceID string) string {
	stem := strings.TrimSuffix(filepath.ToSlash(evidenceID), filepath.Ext(evidenceID))
	stem = strings.ToLower(strings.TrimSpace(stem))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		slug = "evidence"
	}
	return fmt.Sprintf("%s%s_%s", evidenceIDPrefix, sha[:12], slug)
}

func canonicalEvidenceID(sha string, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if strings.HasPrefix(candidate, evidenceIDPrefix+sha[:12]+"_") {
		return candidate
	}
	return evidenceIDForFile(sha, candidate)
}

func copyToEvidenceStore(path string, storeDir string, sha string) (string, error) {
	storageName := filepath.ToSlash(filepath.Join(sha[:2], sha))
	dst := filepath.Join(storeDir, filepath.FromSlash(storageName))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("create evidence store dir: %w", err)
	}
	if _, err := os.Stat(dst); err == nil {
		existing, err := sha256File(dst)
		if err != nil {
			return "", err
		}
		if existing != sha {
			return "", fmt.Errorf("evidence store collision at %s", dst)
		}
		return storageName, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat evidence store file %s: %w", dst, err)
	}
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open evidence source %s: %w", path, err)
	}
	defer src.Close()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create evidence store file %s: %w", dst, err)
	}
	_, copyErr := io.Copy(dstFile, src)
	closeErr := dstFile.Close()
	if copyErr != nil {
		return "", fmt.Errorf("copy evidence bytes to %s: %w", dst, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close evidence store file %s: %w", dst, closeErr)
	}
	return storageName, nil
}

func (rc *runContext) initializeEvidenceRegistry() error {
	rc.evidenceStoreDir = filepath.Join(rc.cfg.OutputDir, "evidence-store")
	rc.evidenceByID = map[string]EvidenceMeta{}
	rc.evidence = []EvidenceMeta{}
	fileByID := map[string]CaseFile{}
	for i, file := range rc.caseFiles {
		evidence, err := rc.registerCaseFileEvidence(file)
		if err != nil {
			return err
		}
		file.EvidenceID = evidence.EvidenceID
		rc.caseFiles[i] = file
		fileByID[file.EvidenceID] = file
	}
	rc.fileByID = fileByID
	return rc.writeEvidenceManifest()
}

func (rc *runContext) registerCaseFileEvidence(file CaseFile) (EvidenceMeta, error) {
	meta, err := rc.buildEvidenceMeta(file.Path, EvidenceMeta{
		EvidenceID:          file.EvidenceID,
		Title:               file.EvidenceID,
		OriginalName:        file.Name,
		MimeType:            file.MimeType,
		SourceDescription:   "Initial case packet file",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "council_visible",
		TextReadable:        file.TextReadable,
	})
	if err != nil {
		return EvidenceMeta{}, err
	}
	return rc.addEvidence(meta)
}

func (rc *runContext) registerSubmittedEvidenceEvidence(meta SubmittedEvidenceMeta, file CaseFile) (EvidenceMeta, error) {
	return rc.registerSubmittedEvidenceEvidenceWithDerivation(meta, file, "", "", "")
}

func (rc *runContext) registerSubmittedEvidenceEvidenceWithDerivation(meta SubmittedEvidenceMeta, file CaseFile, parentEvidenceID string, parentSHA256 string, derivationMethod string) (EvidenceMeta, error) {
	evidence, err := rc.buildEvidenceMeta(file.Path, EvidenceMeta{
		EvidenceID:          meta.EvidenceID,
		Title:               meta.Title,
		OriginalName:        meta.Name,
		MimeType:            meta.MimeType,
		SourceURL:           meta.SourceURL,
		SourceDescription:   meta.SourceDescription,
		RetrievalTimestamp:  meta.RetrievalTimestamp,
		SubmittedByRole:     meta.Role,
		SubmittedPhase:      meta.Phase,
		AdmissibilityStatus: "submitted_evidence",
		RecordVisibility:    "council_visible",
		Relevance:           meta.Relevance,
		TextReadable:        file.TextReadable,
		ParentEvidenceID:    strings.TrimSpace(parentEvidenceID),
		ParentSHA256:        strings.TrimSpace(parentSHA256),
		DerivationMethod:    strings.TrimSpace(derivationMethod),
	})
	if err != nil {
		return EvidenceMeta{}, err
	}
	return rc.addEvidence(evidence)
}

func (rc *runContext) buildEvidenceMeta(path string, meta EvidenceMeta) (EvidenceMeta, error) {
	info, err := os.Stat(path)
	if err != nil {
		return EvidenceMeta{}, fmt.Errorf("stat evidence source %s: %w", path, err)
	}
	if info.IsDir() {
		return EvidenceMeta{}, fmt.Errorf("evidence source is a directory: %s", path)
	}
	sha, err := sha256File(path)
	if err != nil {
		return EvidenceMeta{}, err
	}
	storageName, err := copyToEvidenceStore(path, rc.evidenceStoreDir, sha)
	if err != nil {
		return EvidenceMeta{}, err
	}
	meta.EvidenceID = canonicalEvidenceID(sha, meta.EvidenceID)
	meta.SHA256 = sha
	meta.SizeBytes = int(info.Size())
	if strings.TrimSpace(meta.MimeType) == "" {
		meta.MimeType = "application/octet-stream"
	}
	meta.StorageName = storageName
	meta.CreatedAt = utcTimestamp()
	return meta, nil
}

func (rc *runContext) addEvidence(meta EvidenceMeta) (EvidenceMeta, error) {
	if meta.EvidenceID == "" {
		return EvidenceMeta{}, fmt.Errorf("evidence_id is required")
	}
	if rc.evidenceByID == nil {
		rc.evidenceByID = map[string]EvidenceMeta{}
	}
	if existing, ok := rc.evidenceByID[meta.EvidenceID]; ok {
		if conflict := evidenceMetadataConflict(existing, meta); conflict != "" {
			return EvidenceMeta{}, fmt.Errorf("evidence_id collision %s: %s", meta.EvidenceID, conflict)
		}
		return existing, nil
	}
	rc.evidence = append(rc.evidence, meta)
	rc.evidenceByID[meta.EvidenceID] = meta
	return meta, nil
}

func evidenceMetadataConflict(existing EvidenceMeta, incoming EvidenceMeta) string {
	left := existing
	right := incoming
	left.CreatedAt = ""
	right.CreatedAt = ""
	if left == right {
		return ""
	}
	if existing.SHA256 != incoming.SHA256 || existing.SizeBytes != incoming.SizeBytes || existing.StorageName != incoming.StorageName {
		return fmt.Sprintf("existing sha=%s size=%d storage=%s, incoming sha=%s size=%d storage=%s", existing.SHA256, existing.SizeBytes, existing.StorageName, incoming.SHA256, incoming.SizeBytes, incoming.StorageName)
	}
	return "metadata differs for existing evidence_id"
}

func (rc *runContext) evidencePath(meta EvidenceMeta) (string, error) {
	storageName := strings.TrimSpace(meta.StorageName)
	if storageName == "" {
		return "", fmt.Errorf("evidence %s has no storage_name", meta.EvidenceID)
	}
	if filepath.IsAbs(storageName) {
		return storageName, nil
	}
	return filepath.Join(rc.evidenceStoreDir, filepath.FromSlash(storageName)), nil
}

func (rc *runContext) evidenceManifest() map[string]any {
	evidence := append([]EvidenceMeta(nil), rc.evidence...)
	if evidence == nil {
		evidence = []EvidenceMeta{}
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].EvidenceID < evidence[j].EvidenceID })
	return map[string]any{
		"schema_version": evidenceManifestSchemaVersion,
		"created_at":     utcTimestamp(),
		"evidence_count": len(evidence),
		"evidence":       evidence,
	}
}

func (rc *runContext) writeEvidenceManifest() error {
	return writeJSONFileAtomic(filepath.Join(rc.cfg.OutputDir, "evidence-manifest.json"), rc.evidenceManifest())
}

func (rc *runContext) listVisibleEvidence() []EvidenceMeta {
	out := make([]EvidenceMeta, 0, len(rc.evidence))
	for _, evidence := range rc.evidence {
		if evidence.RecordVisibility == "system_private" {
			continue
		}
		out = append(out, evidence)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EvidenceID < out[j].EvidenceID })
	return out
}

func (rc *runContext) statEvidence(evidenceID string) (EvidenceMeta, error) {
	evidenceID = strings.TrimSpace(evidenceID)
	if evidenceID == "" {
		return EvidenceMeta{}, fmt.Errorf("evidence_id is required")
	}
	meta, ok := rc.evidenceByID[evidenceID]
	if !ok || meta.RecordVisibility == "system_private" {
		return EvidenceMeta{}, fmt.Errorf("unknown evidence %q", evidenceID)
	}
	path, err := rc.evidencePath(meta)
	if err != nil {
		return EvidenceMeta{}, err
	}
	sha, err := sha256File(path)
	if err != nil {
		return EvidenceMeta{}, err
	}
	if sha != meta.SHA256 {
		return EvidenceMeta{}, fmt.Errorf("evidence %s sha256 mismatch", evidenceID)
	}
	info, err := os.Stat(path)
	if err != nil {
		return EvidenceMeta{}, fmt.Errorf("stat evidence %s: %w", evidenceID, err)
	}
	if int(info.Size()) != meta.SizeBytes {
		return EvidenceMeta{}, fmt.Errorf("evidence %s size mismatch", evidenceID)
	}
	return meta, nil
}

func (rc *runContext) readEvidenceRange(evidenceID string, offset int64, length int, budget *evidenceReadBudget) (map[string]any, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}
	if length > rc.cfg.Policy.MaxEvidenceReadBytes {
		return nil, fmt.Errorf("evidence_read_limit_exceeded: requested %d, max %d", length, rc.cfg.Policy.MaxEvidenceReadBytes)
	}
	if budget != nil {
		if budget.reads >= rc.cfg.Policy.MaxEvidenceReadsPerOpportunity {
			return nil, fmt.Errorf("evidence_read_limit_exceeded: read count limit %d", rc.cfg.Policy.MaxEvidenceReadsPerOpportunity)
		}
		if budget.bytes+length > rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity {
			return nil, fmt.Errorf("evidence_read_limit_exceeded: opportunity byte budget %d", rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity)
		}
	}
	meta, err := rc.statEvidence(evidenceID)
	if err != nil {
		return nil, err
	}
	path, err := rc.evidencePath(meta)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open evidence %s: %w", evidenceID, err)
	}
	defer f.Close()
	if offset > int64(meta.SizeBytes) {
		return nil, fmt.Errorf("invalid_evidence_range: offset %d exceeds size %d", offset, meta.SizeBytes)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek evidence %s: %w", evidenceID, err)
	}
	raw := make([]byte, length)
	n, err := f.Read(raw)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read evidence %s: %w", evidenceID, err)
	}
	raw = raw[:n]
	if budget != nil {
		budget.reads++
		budget.bytes += n
	}
	return map[string]any{
		"evidence_id":      meta.EvidenceID,
		"offset":           offset,
		"length":           n,
		"total_size_bytes": meta.SizeBytes,
		"sha256":           meta.SHA256,
		"mime_type":        meta.MimeType,
		"content_base64":   base64.StdEncoding.EncodeToString(raw),
	}, nil
}

func safeMaterializedEvidenceName(meta EvidenceMeta) string {
	name := filepath.Base(strings.TrimSpace(meta.OriginalName))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = meta.EvidenceID
	}
	return meta.EvidenceID + "-" + name
}

func (rc *runContext) materializeEvidence(workspaceDir string, evidenceID string) (map[string]any, error) {
	meta, err := rc.statEvidence(evidenceID)
	if err != nil {
		return nil, err
	}
	path, err := rc.evidencePath(meta)
	if err != nil {
		return nil, err
	}
	evidenceDir := filepath.Join(workspaceDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create evidence workspace dir: %w", err)
	}
	dst := filepath.Join(evidenceDir, safeMaterializedEvidenceName(meta))
	if _, err := os.Stat(dst); err == nil {
		existing, err := sha256File(dst)
		if err != nil {
			return nil, err
		}
		if existing != meta.SHA256 {
			return nil, fmt.Errorf("materialization target exists with different bytes: %s", dst)
		}
	} else if os.IsNotExist(err) {
		if err := copyFile(dst, path); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("stat materialized evidence %s: %w", dst, err)
	}
	remote := filepath.ToSlash(filepath.Join(defaultRemoteSessionCwd, "evidence", filepath.Base(dst)))
	return map[string]any{
		"evidence_id":    meta.EvidenceID,
		"workspace_path": remote,
		"sha256":         meta.SHA256,
		"size_bytes":     meta.SizeBytes,
		"mime_type":      meta.MimeType,
	}, nil
}

func randomUploadID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return "upl_" + hex.EncodeToString(buf), nil
}

func (rc *runContext) beginEvidenceUpload(opportunity Opportunity, params map[string]any) (*EvidenceUploadSession, error) {
	if opportunity.Phase != "arguments" && opportunity.Phase != "rebuttals" {
		return nil, fmt.Errorf("evidence upload is allowed only in arguments and rebuttals")
	}
	title := mapString(params["title"])
	mimeType := mapString(params["mime_type"])
	relevance := mapString(params["relevance"])
	sourceURL := mapString(params["source_url"])
	sourceDescription := mapString(params["source_description"])
	expectedSize, err := requiredIntParam(params, "expected_size_bytes")
	if err != nil {
		return nil, err
	}
	if title == "" {
		return nil, fmt.Errorf("evidence upload requires title")
	}
	if sourceURL == "" && sourceDescription == "" {
		return nil, fmt.Errorf("evidence upload requires source_url or source_description")
	}
	if mimeType == "" {
		return nil, fmt.Errorf("evidence upload requires mime_type")
	}
	if relevance == "" {
		return nil, fmt.Errorf("evidence upload requires relevance")
	}
	if expectedSize <= 0 {
		return nil, fmt.Errorf("evidence upload requires positive expected_size_bytes")
	}
	if expectedSize > rc.cfg.Policy.MaxEvidenceUploadBytes {
		return nil, fmt.Errorf("evidence upload exceeds byte limit of %d", rc.cfg.Policy.MaxEvidenceUploadBytes)
	}
	if submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedEvidencePerSide {
		return nil, fmt.Errorf("submitted_evidence for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedEvidencePerSide)
	}
	parentEvidenceID := strings.TrimSpace(mapString(params["parent_evidence_id"]))
	parentSHA256 := ""
	if parentEvidenceID != "" {
		parent, err := rc.statEvidence(parentEvidenceID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent_evidence_id %q: %w", parentEvidenceID, err)
		}
		parentEvidenceID = parent.EvidenceID
		parentSHA256 = parent.SHA256
	}
	uploadID, err := randomUploadID()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(rc.cfg.OutputDir, "evidence-uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create evidence upload dir: %w", err)
	}
	session := &EvidenceUploadSession{
		UploadID:           uploadID,
		Role:               opportunity.Role,
		Phase:              opportunity.Phase,
		Title:              title,
		MimeType:           mimeType,
		ExpectedSizeBytes:  expectedSize,
		ExpectedSHA256:     strings.ToLower(mapString(params["expected_sha256"])),
		SourceURL:          sourceURL,
		SourceDescription:  sourceDescription,
		RetrievalTimestamp: mapString(params["retrieval_timestamp"]),
		Relevance:          relevance,
		ParentEvidenceID:   parentEvidenceID,
		ParentSHA256:       parentSHA256,
		DerivationMethod:   mapString(params["derivation_method"]),
		Path:               filepath.Join(dir, uploadID+".part"),
	}
	if rc.uploadSessions == nil {
		rc.uploadSessions = map[string]*EvidenceUploadSession{}
	}
	rc.uploadSessions[uploadID] = session
	return session, nil
}

func (rc *runContext) writeEvidenceChunk(uploadID string, offset int, contentBase64 string) (*EvidenceUploadSession, int, error) {
	session := rc.uploadSessions[strings.TrimSpace(uploadID)]
	if session == nil {
		return nil, 0, fmt.Errorf("unknown upload_id %q", uploadID)
	}
	if offset != session.ReceivedBytes {
		return nil, 0, fmt.Errorf("evidence upload offset %d does not match received byte count %d", offset, session.ReceivedBytes)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(contentBase64))
	if err != nil {
		return nil, 0, fmt.Errorf("decode evidence chunk: %w", err)
	}
	if len(raw) == 0 {
		return nil, 0, fmt.Errorf("evidence chunk must not be empty")
	}
	if len(raw) > rc.cfg.Policy.MaxEvidenceChunkBytes {
		return nil, 0, fmt.Errorf("evidence chunk exceeds byte limit of %d", rc.cfg.Policy.MaxEvidenceChunkBytes)
	}
	if session.ReceivedBytes+len(raw) > session.ExpectedSizeBytes {
		return nil, 0, fmt.Errorf("evidence upload exceeds expected size of %d", session.ExpectedSizeBytes)
	}
	f, err := os.OpenFile(session.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, 0, fmt.Errorf("open evidence upload %s: %w", session.UploadID, err)
	}
	written, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil {
		return nil, 0, fmt.Errorf("write evidence upload %s: %w", session.UploadID, writeErr)
	}
	if written != len(raw) {
		return nil, 0, fmt.Errorf("short evidence upload write %s: wrote %d of %d bytes", session.UploadID, written, len(raw))
	}
	if closeErr != nil {
		return nil, 0, fmt.Errorf("close evidence upload %s: %w", session.UploadID, closeErr)
	}
	session.ReceivedBytes += len(raw)
	return session, len(raw), nil
}

func (rc *runContext) prepareEvidenceUploadCommit(session *EvidenceUploadSession, preferredExt string) (SubmittedEvidenceMeta, error) {
	if session == nil {
		return SubmittedEvidenceMeta{}, fmt.Errorf("upload session is required")
	}
	if session.ReceivedBytes != session.ExpectedSizeBytes {
		return SubmittedEvidenceMeta{}, fmt.Errorf("evidence upload incomplete: received %d of %d bytes", session.ReceivedBytes, session.ExpectedSizeBytes)
	}
	sha, err := sha256File(session.Path)
	if err != nil {
		return SubmittedEvidenceMeta{}, err
	}
	if session.ExpectedSHA256 != "" && session.ExpectedSHA256 != sha {
		return SubmittedEvidenceMeta{}, fmt.Errorf("evidence upload sha256 mismatch: expected %s, got %s", session.ExpectedSHA256, sha)
	}
	name := submittedEvidenceFilename(len(rc.submittedEvidence)+1, session.Role, sha, session.MimeType, preferredExt)
	return SubmittedEvidenceMeta{
		Phase:              session.Phase,
		Role:               session.Role,
		EvidenceID:         evidenceIDForFile(sha, name),
		Name:               name,
		Title:              session.Title,
		SourceURL:          session.SourceURL,
		SourceDescription:  session.SourceDescription,
		MimeType:           session.MimeType,
		RetrievalTimestamp: session.RetrievalTimestamp,
		Relevance:          session.Relevance,
		SHA256:             sha,
		SizeBytes:          session.ReceivedBytes,
	}, nil
}

func (rc *runContext) finalizeEvidenceUpload(session *EvidenceUploadSession, meta SubmittedEvidenceMeta) (SubmittedEvidenceMeta, CaseFile, EvidenceMeta, error) {
	file, err := rc.moveUploadedSubmittedEvidenceFile(meta, session.Path)
	if err != nil {
		return SubmittedEvidenceMeta{}, CaseFile{}, EvidenceMeta{}, err
	}
	evidence, err := rc.registerSubmittedEvidenceEvidenceWithDerivation(meta, file, session.ParentEvidenceID, session.ParentSHA256, session.DerivationMethod)
	if err != nil {
		return SubmittedEvidenceMeta{}, CaseFile{}, EvidenceMeta{}, err
	}
	meta.EvidenceID = evidence.EvidenceID
	delete(rc.uploadSessions, session.UploadID)
	return meta, file, evidence, nil
}

func (rc *runContext) moveUploadedSubmittedEvidenceFile(meta SubmittedEvidenceMeta, src string) (CaseFile, error) {
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
	if err := os.Rename(src, path); err != nil {
		return CaseFile{}, fmt.Errorf("move submitted evidence %s: %w", path, err)
	}
	_, readableKind := caseFileKind(name)
	textReadable := readableKind || strings.HasPrefix(strings.ToLower(meta.MimeType), "text/") || strings.EqualFold(meta.MimeType, "application/json")
	file := CaseFile{EvidenceID: meta.EvidenceID, Name: name, Path: path, MimeType: meta.MimeType, TextReadable: textReadable && meta.SizeBytes <= rc.cfg.Policy.MaxEvidenceReadBytes, SizeBytes: meta.SizeBytes}
	if file.TextReadable {
		raw, err := os.ReadFile(path)
		if err != nil {
			return CaseFile{}, fmt.Errorf("read text uploaded evidence %s: %w", path, err)
		}
		file.Text = string(raw)
	}
	return file, nil
}

func copyFile(dst string, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", dst, closeErr)
	}
	return nil
}
