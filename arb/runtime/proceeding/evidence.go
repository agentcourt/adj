package proceeding

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
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

type evidenceFileSnapshot struct {
	meta EvidenceMeta
	path string
}

type evidenceReadReservation struct {
	file          evidenceFileSnapshot
	offset        int64
	length        int
	budget        *evidenceReadBudget
	reservedBytes int
	active        bool
}

type evidenceFileOperations struct {
	verify func(evidenceFileSnapshot) error
	read   func(evidenceReadReservation) (map[string]any, int, error)
}

func (ops evidenceFileOperations) verifyFile(file evidenceFileSnapshot) error {
	if ops.verify != nil {
		return ops.verify(file)
	}
	return verifyEvidenceFile(file)
}

func (ops evidenceFileOperations) readRange(reservation evidenceReadReservation) (map[string]any, int, error) {
	if ops.read != nil {
		return ops.read(reservation)
	}
	return readReservedEvidenceRange(reservation)
}

func utcTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func sha256File(path string) (digest string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s after hashing: %w", path, closeErr))
		}
	}()
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

func evidenceStorageName(sha string) (string, error) {
	if len(sha) != sha256.Size*2 {
		return "", fmt.Errorf("invalid evidence sha256 %q", sha)
	}
	return filepath.ToSlash(filepath.Join(sha[:2], sha)), nil
}

func verifyPublishedFile(path string, sha string, size int) error {
	_, err := readVerifiedEvidenceFile(evidenceFileSnapshot{
		meta: EvidenceMeta{SHA256: sha, SizeBytes: size},
		path: path,
	}, 0, 0)
	return err
}

func openPublishedEvidenceFile(file evidenceFileSnapshot) (f *os.File, err error) {
	pathInfo, err := os.Lstat(file.path)
	if err != nil {
		return nil, fmt.Errorf("stat published evidence %s: %w", file.path, err)
	}
	if !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("published evidence is not a regular file: %s", file.path)
	}
	f, err = os.Open(file.path)
	if err != nil {
		return nil, fmt.Errorf("open published evidence %s: %w", file.path, err)
	}
	opened := f
	valid := false
	defer func() {
		if valid {
			return
		}
		if closeErr := opened.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close published evidence %s: %w", file.path, closeErr))
		}
	}()
	openedInfo, err := opened.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat open published evidence %s: %w", file.path, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("open published evidence is not a regular file: %s", file.path)
	}
	if !os.SameFile(pathInfo, openedInfo) {
		return nil, fmt.Errorf("published evidence changed while opening: %s", file.path)
	}
	if openedInfo.Size() != int64(file.meta.SizeBytes) {
		return nil, fmt.Errorf("published evidence %s size mismatch: got %d, want %d", file.path, openedInfo.Size(), file.meta.SizeBytes)
	}
	valid = true
	return opened, nil
}

func readVerifiedOpenEvidenceFile(f *os.File, file evidenceFileSnapshot, offset int64, length int) ([]byte, error) {
	if offset < 0 || length < 0 {
		return nil, fmt.Errorf("invalid evidence range: offset %d, length %d", offset, length)
	}
	end := offset + int64(length)
	if end < offset {
		return nil, fmt.Errorf("invalid evidence range: offset %d, length %d", offset, length)
	}
	capacity := 0
	if offset < int64(file.meta.SizeBytes) {
		capacity = min(length, file.meta.SizeBytes-int(offset))
	}
	raw := make([]byte, 0, capacity)
	h := sha256.New()
	buf := make([]byte, 32*1024)
	var position int64
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if _, err := h.Write(chunk); err != nil {
				return nil, fmt.Errorf("hash published evidence %s: %w", file.path, err)
			}
			chunkEnd := position + int64(n)
			if position < end && chunkEnd > offset {
				startIndex := max(offset-position, 0)
				endIndex := min(end-position, int64(n))
				raw = append(raw, chunk[int(startIndex):int(endIndex)]...)
			}
			position = chunkEnd
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read published evidence %s: %w", file.path, readErr)
		}
	}
	if position != int64(file.meta.SizeBytes) {
		return nil, fmt.Errorf("published evidence %s size mismatch while reading: got %d, want %d", file.path, position, file.meta.SizeBytes)
	}
	actualSHA := hex.EncodeToString(h.Sum(nil))
	if actualSHA != file.meta.SHA256 {
		return nil, fmt.Errorf("published evidence %s sha256 mismatch: got %s, want %s", file.path, actualSHA, file.meta.SHA256)
	}
	return raw, nil
}

func readVerifiedEvidenceFile(file evidenceFileSnapshot, offset int64, length int) (raw []byte, err error) {
	f, err := openPublishedEvidenceFile(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close published evidence %s: %w", file.path, closeErr))
		}
	}()
	return readVerifiedOpenEvidenceFile(f, file, offset, length)
}

func publishReaderAtomic(dst string, source io.Reader, sha string, size int, mode os.FileMode) (err error) {
	if statErr := verifyPublishedFile(dst, sha, size); statErr == nil {
		return nil
	} else if _, err := os.Lstat(dst); err == nil {
		return statErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat publication target %s: %w", dst, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create publication directory for %s: %w", dst, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create publication temp file for %s: %w", dst, err)
	}
	tmpName := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := tmp.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close publication temp file %s: %w", tmpName, closeErr))
			}
		}
		if removeErr := os.Remove(tmpName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove publication temp file %s: %w", tmpName, removeErr))
		}
	}()
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, h), source)
	if copyErr != nil {
		return fmt.Errorf("write publication temp file %s: %w", tmpName, copyErr)
	}
	if written != int64(size) {
		return fmt.Errorf("publication source size mismatch for %s: got %d, want %d", dst, written, size)
	}
	actualSHA := hex.EncodeToString(h.Sum(nil))
	if actualSHA != sha {
		return fmt.Errorf("publication source sha256 mismatch for %s: got %s, want %s", dst, actualSHA, sha)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("chmod publication temp file %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync publication temp file %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return fmt.Errorf("close publication temp file %s: %w", tmpName, err)
	}
	closed = true
	if err := os.Link(tmpName, dst); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("publish %s: %w", dst, err)
		}
		if err := verifyPublishedFile(dst, sha, size); err != nil {
			return fmt.Errorf("publication target collision at %s: %w", dst, err)
		}
		return nil
	}
	if err := verifyPublishedFile(dst, sha, size); err != nil {
		if removeErr := os.Remove(dst); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove invalid publication %s: %w", dst, removeErr))
		}
		return err
	}
	return nil
}

func publishFileAtomic(src string, dst string, sha string, size int, mode os.FileMode) (err error) {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open publication source %s: %w", src, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close publication source %s: %w", src, closeErr))
		}
	}()
	return publishReaderAtomic(dst, f, sha, size, mode)
}

func publishBytesAtomic(raw []byte, dst string, sha string, mode os.FileMode) error {
	return publishReaderAtomic(dst, bytes.NewReader(raw), sha, len(raw), mode)
}

func (rc *runContext) initializeEvidenceRegistry() error {
	rc.evidenceStoreDir = filepath.Join(rc.cfg.OutputDir, "evidence-store")
	candidate := &runContext{evidence: []EvidenceMeta{}, evidenceByID: map[string]EvidenceMeta{}}
	caseFiles := make([]CaseFile, 0, len(rc.caseFiles))
	fileByID := map[string]CaseFile{}
	for _, file := range rc.caseFiles {
		evidence, err := rc.caseFileEvidenceMeta(file)
		if err != nil {
			return err
		}
		if _, err := candidate.addEvidence(evidence); err != nil {
			return err
		}
		storedFile, err := rc.caseFileFromEvidence(evidence)
		if err != nil {
			return err
		}
		caseFiles = append(caseFiles, storedFile)
		fileByID[storedFile.EvidenceID] = storedFile
	}
	if err := rc.writeEvidenceManifestCandidate(candidate.evidence); err != nil {
		return err
	}
	rc.evidence = candidate.evidence
	rc.evidenceByID = candidate.evidenceByID
	rc.caseFiles = caseFiles
	rc.fileByID = fileByID
	return nil
}

func (rc *runContext) caseFileEvidenceMeta(file CaseFile) (EvidenceMeta, error) {
	return rc.buildEvidenceMeta(file.Path, EvidenceMeta{
		EvidenceID:          file.EvidenceID,
		Title:               file.EvidenceID,
		OriginalName:        file.Name,
		MimeType:            file.MimeType,
		SourceDescription:   "Initial case packet file",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "juror_visible",
		TextReadable:        file.TextReadable,
	})
}

func (rc *runContext) buildEvidenceMeta(path string, meta EvidenceMeta) (EvidenceMeta, error) {
	storageName, sha, size, err := captureInitialEvidence(path, rc.evidenceStoreDir)
	if err != nil {
		return EvidenceMeta{}, err
	}
	meta.EvidenceID = canonicalEvidenceID(sha, meta.EvidenceID)
	meta.SHA256 = sha
	meta.SizeBytes = size
	if strings.TrimSpace(meta.MimeType) == "" {
		meta.MimeType = "application/octet-stream"
	}
	meta.StorageName = storageName
	meta.CreatedAt = utcTimestamp()
	return meta, nil
}

func captureInitialEvidence(path string, storeDir string) (storageName string, sha string, size int, err error) {
	pathInfo, err := os.Stat(path)
	if err != nil {
		return "", "", 0, fmt.Errorf("stat evidence source %s: %w", path, err)
	}
	if !pathInfo.Mode().IsRegular() {
		return "", "", 0, fmt.Errorf("evidence source is not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", "", 0, fmt.Errorf("open evidence source %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close evidence source %s: %w", path, closeErr))
		}
	}()
	openedInfo, err := f.Stat()
	if err != nil {
		return "", "", 0, fmt.Errorf("stat open evidence source %s: %w", path, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return "", "", 0, fmt.Errorf("open evidence source is not a regular file: %s", path)
	}
	if !os.SameFile(pathInfo, openedInfo) {
		return "", "", 0, fmt.Errorf("evidence source changed while opening: %s", path)
	}
	h := sha256.New()
	written, err := io.Copy(h, f)
	if err != nil {
		return "", "", 0, fmt.Errorf("hash evidence source %s: %w", path, err)
	}
	if written != openedInfo.Size() {
		return "", "", 0, fmt.Errorf("evidence source %s size changed while hashing: got %d, want %d", path, written, openedInfo.Size())
	}
	size = int(written)
	if int64(size) != written {
		return "", "", 0, fmt.Errorf("evidence source %s is too large", path)
	}
	sha = hex.EncodeToString(h.Sum(nil))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", "", 0, fmt.Errorf("rewind evidence source %s: %w", path, err)
	}
	storageName, err = evidenceStorageName(sha)
	if err != nil {
		return "", "", 0, err
	}
	dst := filepath.Join(storeDir, filepath.FromSlash(storageName))
	if err := publishReaderAtomic(dst, f, sha, size, 0o444); err != nil {
		return "", "", 0, fmt.Errorf("publish evidence store file: %w", err)
	}
	return storageName, sha, size, nil
}

func (rc *runContext) caseFileFromEvidence(meta EvidenceMeta) (CaseFile, error) {
	path, err := rc.evidencePath(meta)
	if err != nil {
		return CaseFile{}, err
	}
	length := 0
	if meta.TextReadable {
		length = meta.SizeBytes
	}
	raw, err := readVerifiedEvidenceFile(evidenceFileSnapshot{meta: meta, path: path}, 0, length)
	if err != nil {
		return CaseFile{}, fmt.Errorf("verify stored evidence %s: %w", meta.EvidenceID, err)
	}
	file := CaseFile{
		EvidenceID:   meta.EvidenceID,
		Name:         meta.OriginalName,
		Path:         path,
		MimeType:     meta.MimeType,
		TextReadable: meta.TextReadable,
		SizeBytes:    meta.SizeBytes,
	}
	if file.TextReadable {
		file.Text = string(raw)
	}
	return file, nil
}

func submittedEvidenceRecordMeta(meta SubmittedEvidenceMeta, textReadable bool) (EvidenceMeta, error) {
	storageName, err := evidenceStorageName(meta.SHA256)
	if err != nil {
		return EvidenceMeta{}, err
	}
	if meta.EvidenceID == "" || canonicalEvidenceID(meta.SHA256, meta.EvidenceID) != meta.EvidenceID {
		return EvidenceMeta{}, fmt.Errorf("invalid submitted evidence_id %q for sha256 %s", meta.EvidenceID, meta.SHA256)
	}
	return EvidenceMeta{
		EvidenceID:          meta.EvidenceID,
		SHA256:              meta.SHA256,
		SizeBytes:           meta.SizeBytes,
		MimeType:            meta.MimeType,
		StorageName:         storageName,
		CreatedAt:           utcTimestamp(),
		AdmissibilityStatus: "submitted_evidence",
		RecordVisibility:    "juror_visible",
		Title:               meta.Title,
		OriginalName:        meta.Name,
		SourceURL:           meta.SourceURL,
		SourceDescription:   meta.SourceDescription,
		RetrievalTimestamp:  meta.RetrievalTimestamp,
		SubmittedByRole:     meta.Role,
		SubmittedPhase:      meta.Phase,
		ParentEvidenceID:    meta.ParentEvidenceID,
		ParentSHA256:        meta.ParentSHA256,
		DerivationMethod:    meta.DerivationMethod,
		Relevance:           meta.Relevance,
		TextReadable:        textReadable,
	}, nil
}

func (rc *runContext) candidateEvidenceRegistry(meta EvidenceMeta) ([]EvidenceMeta, map[string]EvidenceMeta, error) {
	if err := rc.validateSubmittedEvidenceID(meta.EvidenceID); err != nil {
		return nil, nil, err
	}
	evidence := append([]EvidenceMeta(nil), rc.evidence...)
	byID := make(map[string]EvidenceMeta, len(rc.evidenceByID)+1)
	for evidenceID, existing := range rc.evidenceByID {
		byID[evidenceID] = existing
	}
	candidate := &runContext{evidence: evidence, evidenceByID: byID}
	if _, err := candidate.addEvidence(meta); err != nil {
		return nil, nil, err
	}
	return candidate.evidence, candidate.evidenceByID, nil
}

func submittedEvidenceCopyPath(outputDir string, meta SubmittedEvidenceMeta) (string, error) {
	name := filepath.Base(meta.Name)
	if name == "" || name == "." || name == string(filepath.Separator) || name != meta.Name {
		return "", fmt.Errorf("invalid submitted evidence filename %q", meta.Name)
	}
	return filepath.Join(outputDir, "submitted-evidence", name), nil
}

func (rc *runContext) publishSubmittedEvidence(meta SubmittedEvidenceMeta, evidence EvidenceMeta, raw []byte, sourcePath string) (CaseFile, string, error) {
	storePath, err := rc.evidencePath(evidence)
	if err != nil {
		return CaseFile{}, "", err
	}
	copyPath, err := submittedEvidenceCopyPath(rc.cfg.OutputDir, meta)
	if err != nil {
		return CaseFile{}, "", err
	}
	publish := func(dst string, mode os.FileMode) error {
		if sourcePath != "" {
			return publishFileAtomic(sourcePath, dst, meta.SHA256, meta.SizeBytes, mode)
		}
		return publishBytesAtomic(raw, dst, meta.SHA256, mode)
	}
	if err := publish(storePath, 0o444); err != nil {
		return CaseFile{}, "", fmt.Errorf("publish immutable evidence %s: %w", meta.EvidenceID, err)
	}
	if err := publish(copyPath, 0o644); err != nil {
		return CaseFile{}, "", fmt.Errorf("publish submitted evidence copy %s: %w", meta.EvidenceID, err)
	}
	file, err := rc.caseFileFromEvidence(evidence)
	if err != nil {
		return CaseFile{}, "", err
	}
	return file, copyPath, nil
}

func (rc *runContext) initialEvidenceCommitments() []EvidenceCommitment {
	out := make([]EvidenceCommitment, 0, len(rc.evidence))
	for _, evidence := range rc.evidence {
		out = append(out, EvidenceCommitment{
			EvidenceID: evidence.EvidenceID,
			SHA256:     evidence.SHA256,
			SizeBytes:  evidence.SizeBytes,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EvidenceID < out[j].EvidenceID })
	return out
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

func evidenceManifest(evidence []EvidenceMeta) map[string]any {
	evidence = append([]EvidenceMeta(nil), evidence...)
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

func (rc *runContext) evidenceManifest() map[string]any {
	return evidenceManifest(rc.evidence)
}

func (rc *runContext) writeEvidenceManifestCandidate(evidence []EvidenceMeta) error {
	return writeJSONFileAtomic(filepath.Join(rc.cfg.OutputDir, "evidence-manifest.json"), evidenceManifest(evidence))
}

func visibleEvidence(meta EvidenceMeta) bool {
	return meta.RecordVisibility != "system_private"
}

func recordEvidence(meta EvidenceMeta) bool {
	if !visibleEvidence(meta) {
		return false
	}
	return meta.AdmissibilityStatus == "case_packet" || meta.AdmissibilityStatus == "submitted_evidence"
}

func (rc *runContext) listVisibleEvidence() []EvidenceMeta {
	out := make([]EvidenceMeta, 0, len(rc.evidence))
	for _, evidence := range rc.evidence {
		if !visibleEvidence(evidence) {
			continue
		}
		out = append(out, evidence)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EvidenceID < out[j].EvidenceID })
	return out
}

func (rc *runContext) evidenceFileSnapshotLocked(evidenceID string) (evidenceFileSnapshot, error) {
	evidenceID = strings.TrimSpace(evidenceID)
	if evidenceID == "" {
		return evidenceFileSnapshot{}, participantInput(fmt.Errorf("evidence_id is required"))
	}
	meta, ok := rc.evidenceByID[evidenceID]
	if !ok || !visibleEvidence(meta) {
		return evidenceFileSnapshot{}, participantInput(fmt.Errorf("unknown evidence %q", evidenceID))
	}
	path, err := rc.evidencePath(meta)
	if err != nil {
		return evidenceFileSnapshot{}, err
	}
	return evidenceFileSnapshot{meta: meta, path: path}, nil
}

func validateEvidenceFileToolArguments(tool string, args map[string]any) error {
	switch tool {
	case "stat_evidence":
		return requireAllowedKeys(args, "stat_evidence arguments", "evidence_id")
	case "read_evidence_range":
		return requireAllowedKeys(args, "read_evidence_range arguments", "evidence_id", "offset", "length")
	default:
		return fmt.Errorf("unsupported evidence file tool %q", tool)
	}
}

func verifyEvidenceFile(file evidenceFileSnapshot) error {
	evidenceID := file.meta.EvidenceID
	if err := verifyPublishedFile(file.path, file.meta.SHA256, file.meta.SizeBytes); err != nil {
		return fmt.Errorf("verify evidence %s: %w", evidenceID, err)
	}
	return nil
}

func (rc *runContext) reserveEvidenceReadLocked(evidenceID string, offset int64, length int, budget *evidenceReadBudget) (evidenceReadReservation, error) {
	if offset < 0 {
		return evidenceReadReservation{}, participantInput(fmt.Errorf("offset must be non-negative"))
	}
	if length <= 0 {
		return evidenceReadReservation{}, participantInput(fmt.Errorf("length must be positive"))
	}
	if length > rc.cfg.Policy.MaxEvidenceReadBytes {
		return evidenceReadReservation{}, participantInput(fmt.Errorf("evidence_read_limit_exceeded: requested %d, max %d", length, rc.cfg.Policy.MaxEvidenceReadBytes))
	}
	if budget != nil {
		if budget.reads >= rc.cfg.Policy.MaxEvidenceReadsPerOpportunity {
			return evidenceReadReservation{}, participantInput(fmt.Errorf("evidence_read_limit_exceeded: read count limit %d", rc.cfg.Policy.MaxEvidenceReadsPerOpportunity))
		}
		if budget.bytes+length > rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity {
			return evidenceReadReservation{}, participantInput(fmt.Errorf("evidence_read_limit_exceeded: opportunity byte budget %d", rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity))
		}
	}
	file, err := rc.evidenceFileSnapshotLocked(evidenceID)
	if err != nil {
		return evidenceReadReservation{}, err
	}
	if offset > int64(file.meta.SizeBytes) {
		return evidenceReadReservation{}, participantInput(fmt.Errorf("invalid_evidence_range: offset %d exceeds size %d", offset, file.meta.SizeBytes))
	}
	reservation := evidenceReadReservation{
		file:          file,
		offset:        offset,
		length:        length,
		budget:        budget,
		reservedBytes: length,
	}
	if budget != nil {
		budget.reads++
		budget.bytes += length
		reservation.active = true
	}
	return reservation, nil
}

func rollbackEvidenceReadLocked(reservation *evidenceReadReservation) {
	if reservation == nil || !reservation.active || reservation.budget == nil {
		return
	}
	reservation.budget.reads--
	reservation.budget.bytes -= reservation.reservedBytes
	reservation.active = false
}

func finalizeEvidenceReadLocked(reservation *evidenceReadReservation, bytesRead int) {
	if reservation == nil || !reservation.active || reservation.budget == nil {
		return
	}
	reservation.budget.bytes -= reservation.reservedBytes - bytesRead
	reservation.active = false
}

func readReservedEvidenceRange(reservation evidenceReadReservation) (result map[string]any, bytesRead int, err error) {
	file := reservation.file
	if reservation.offset > int64(file.meta.SizeBytes) {
		return nil, 0, fmt.Errorf("invalid_evidence_range: offset %d exceeds size %d", reservation.offset, file.meta.SizeBytes)
	}
	raw, err := readVerifiedEvidenceFile(file, reservation.offset, reservation.length)
	if err != nil {
		return nil, 0, fmt.Errorf("verify evidence %s: %w", file.meta.EvidenceID, err)
	}
	n := len(raw)
	return map[string]any{
		"evidence_id":      file.meta.EvidenceID,
		"offset":           reservation.offset,
		"length":           n,
		"total_size_bytes": file.meta.SizeBytes,
		"sha256":           file.meta.SHA256,
		"mime_type":        file.meta.MimeType,
		"content_base64":   base64.StdEncoding.EncodeToString(raw),
	}, n, nil
}

func randomUploadID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return "upl_" + hex.EncodeToString(buf), nil
}

func (rc *runContext) resolveEvidenceLineage(parentEvidenceID string, derivationMethod string, childEvidenceID string) (string, string, string, error) {
	parentEvidenceID = strings.TrimSpace(parentEvidenceID)
	derivationMethod = strings.TrimSpace(derivationMethod)
	if (parentEvidenceID == "") != (derivationMethod == "") {
		return "", "", "", participantInput(fmt.Errorf("parent_evidence_id and derivation_method must be supplied together"))
	}
	if parentEvidenceID == "" {
		return "", "", "", nil
	}
	if parentEvidenceID == strings.TrimSpace(childEvidenceID) {
		return "", "", "", participantInput(fmt.Errorf("parent_evidence_id must not equal submitted evidence_id %q", childEvidenceID))
	}
	parentMetadata, ok := rc.evidenceByID[parentEvidenceID]
	if !ok || !recordEvidence(parentMetadata) {
		return "", "", "", participantInput(fmt.Errorf("invalid parent_evidence_id %q: unknown record evidence", parentEvidenceID))
	}
	parentFile, err := rc.evidenceFileSnapshotLocked(parentEvidenceID)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid parent_evidence_id %q: %w", parentEvidenceID, err)
	}
	if err := verifyEvidenceFile(parentFile); err != nil {
		return "", "", "", fmt.Errorf("invalid parent_evidence_id %q: %w", parentEvidenceID, err)
	}
	parent := parentFile.meta
	return parent.EvidenceID, parent.SHA256, derivationMethod, nil
}

func (rc *runContext) validateSubmittedEvidenceLineage(meta SubmittedEvidenceMeta) error {
	allEmpty := meta.ParentEvidenceID == "" && meta.ParentSHA256 == "" && meta.DerivationMethod == ""
	allPresent := meta.ParentEvidenceID != "" && meta.ParentSHA256 != "" && meta.DerivationMethod != ""
	if !allEmpty && !allPresent {
		return fmt.Errorf("parent_evidence_id, parent_sha256, and derivation_method must all be present or all be empty")
	}
	if allEmpty {
		return nil
	}
	parentID, parentSHA, method, err := rc.resolveEvidenceLineage(meta.ParentEvidenceID, meta.DerivationMethod, meta.EvidenceID)
	if err != nil {
		var inputErr *participantInputError
		if errors.As(err, &inputErr) {
			return inputErr.Unwrap()
		}
		return err
	}
	if parentID != meta.ParentEvidenceID || parentSHA != meta.ParentSHA256 || method != meta.DerivationMethod {
		return fmt.Errorf("submitted evidence lineage changed before commit")
	}
	return nil
}

func (rc *runContext) validateSubmittedEvidenceID(evidenceID string) error {
	evidenceID = strings.TrimSpace(evidenceID)
	if evidenceID == "" {
		return fmt.Errorf("submitted evidence_id is required")
	}
	if _, exists := rc.evidenceByID[evidenceID]; exists {
		return fmt.Errorf("submitted evidence_id collision %q", evidenceID)
	}
	for _, evidence := range rc.evidence {
		if evidence.EvidenceID == evidenceID {
			return fmt.Errorf("submitted evidence_id collision %q", evidenceID)
		}
	}
	for _, evidence := range rc.submittedEvidence {
		if evidence.EvidenceID == evidenceID {
			return fmt.Errorf("submitted evidence_id collision %q", evidenceID)
		}
	}
	return nil
}

func rejectCallerParentSHA(params map[string]any) error {
	if _, supplied := params["parent_sha256"]; supplied {
		return participantInput(fmt.Errorf("parent_sha256 is derived from parent_evidence_id and must not be supplied"))
	}
	return nil
}

func validateUploadSessionAuthority(opportunity Opportunity, session *EvidenceUploadSession) error {
	if session == nil {
		return participantInput(fmt.Errorf("upload session is required"))
	}
	if session.Role != opportunity.Role || session.Phase != opportunity.Phase {
		return participantInput(fmt.Errorf("upload session %s belongs to role %s in phase %s, not role %s in phase %s", session.UploadID, session.Role, session.Phase, opportunity.Role, opportunity.Phase))
	}
	return nil
}

func (rc *runContext) beginEvidenceUpload(opportunity Opportunity, params map[string]any) (*EvidenceUploadSession, error) {
	if !evidenceSubmissionAllowed(opportunity) {
		return nil, participantInput(fmt.Errorf("evidence upload is allowed only in arguments, rebuttals, and surrebuttals"))
	}
	if err := rejectCallerParentSHA(params); err != nil {
		return nil, err
	}
	if err := requireAllowedKeys(
		params,
		"begin_evidence_upload arguments",
		"title",
		"mime_type",
		"expected_size_bytes",
		"expected_sha256",
		"source_url",
		"source_description",
		"retrieval_timestamp",
		"relevance",
		"parent_evidence_id",
		"derivation_method",
	); err != nil {
		return nil, participantInput(err)
	}
	title, err := optionalStringParam(params, "title")
	if err != nil {
		return nil, participantInput(err)
	}
	mimeType, err := optionalStringParam(params, "mime_type")
	if err != nil {
		return nil, participantInput(err)
	}
	relevance, err := optionalStringParam(params, "relevance")
	if err != nil {
		return nil, participantInput(err)
	}
	sourceURL, err := optionalStringParam(params, "source_url")
	if err != nil {
		return nil, participantInput(err)
	}
	sourceDescription, err := optionalStringParam(params, "source_description")
	if err != nil {
		return nil, participantInput(err)
	}
	retrievalTimestamp, err := optionalStringParam(params, "retrieval_timestamp")
	if err != nil {
		return nil, participantInput(err)
	}
	expectedSHA256, err := optionalStringParam(params, "expected_sha256")
	if err != nil {
		return nil, participantInput(err)
	}
	expectedSize, err := requiredIntParam(params, "expected_size_bytes")
	if err != nil {
		return nil, participantInput(err)
	}
	if title == "" {
		return nil, participantInput(fmt.Errorf("evidence upload requires title"))
	}
	if sourceURL == "" && sourceDescription == "" {
		return nil, participantInput(fmt.Errorf("evidence upload requires source_url or source_description"))
	}
	if mimeType == "" {
		return nil, participantInput(fmt.Errorf("evidence upload requires mime_type"))
	}
	if relevance == "" {
		return nil, participantInput(fmt.Errorf("evidence upload requires relevance"))
	}
	if expectedSize <= 0 {
		return nil, participantInput(fmt.Errorf("evidence upload requires positive expected_size_bytes"))
	}
	if expectedSize > rc.cfg.Policy.MaxEvidenceUploadBytes {
		return nil, participantInput(fmt.Errorf("evidence upload exceeds byte limit of %d", rc.cfg.Policy.MaxEvidenceUploadBytes))
	}
	if submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedEvidencePerSide {
		return nil, participantInput(fmt.Errorf("submitted_evidence for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedEvidencePerSide))
	}
	parentEvidenceIDArg, err := optionalStringParam(params, "parent_evidence_id")
	if err != nil {
		return nil, participantInput(err)
	}
	derivationMethodArg, err := optionalStringParam(params, "derivation_method")
	if err != nil {
		return nil, participantInput(err)
	}
	parentEvidenceID, parentSHA256, derivationMethod, err := rc.resolveEvidenceLineage(
		parentEvidenceIDArg,
		derivationMethodArg,
		"",
	)
	if err != nil {
		return nil, err
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
		ExpectedSHA256:     strings.ToLower(expectedSHA256),
		SourceURL:          sourceURL,
		SourceDescription:  sourceDescription,
		RetrievalTimestamp: retrievalTimestamp,
		Relevance:          relevance,
		ParentEvidenceID:   parentEvidenceID,
		ParentSHA256:       parentSHA256,
		DerivationMethod:   derivationMethod,
		Path:               filepath.Join(dir, uploadID+".part"),
	}
	if rc.uploadSessions == nil {
		rc.uploadSessions = map[string]*EvidenceUploadSession{}
	}
	rc.uploadSessions[uploadID] = session
	return session, nil
}

func (rc *runContext) writeEvidenceChunk(opportunity Opportunity, uploadID string, offset int, contentBase64 string) (*EvidenceUploadSession, int, error) {
	session := rc.uploadSessions[strings.TrimSpace(uploadID)]
	if session == nil {
		return nil, 0, participantInput(fmt.Errorf("unknown upload_id %q", uploadID))
	}
	if err := validateUploadSessionAuthority(opportunity, session); err != nil {
		return nil, 0, err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(contentBase64))
	if err != nil {
		return nil, 0, participantInput(fmt.Errorf("decode evidence chunk: %w", err))
	}
	if len(raw) == 0 {
		return nil, 0, participantInput(fmt.Errorf("evidence chunk must not be empty"))
	}
	if len(raw) > rc.cfg.Policy.MaxEvidenceChunkBytes {
		return nil, 0, participantInput(fmt.Errorf("evidence chunk exceeds byte limit of %d", rc.cfg.Policy.MaxEvidenceChunkBytes))
	}
	visibleBytes, err := reconcileEvidenceUploadFile(session, true)
	if err != nil {
		return nil, 0, err
	}
	if offset != visibleBytes {
		return nil, 0, participantInput(fmt.Errorf("evidence upload offset %d does not match stored byte count %d; next valid offset is %d", offset, visibleBytes, visibleBytes))
	}
	if visibleBytes+len(raw) > session.ExpectedSizeBytes {
		return nil, 0, participantInput(fmt.Errorf("evidence upload exceeds expected size of %d", session.ExpectedSizeBytes))
	}
	f, err := os.OpenFile(session.Path, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, 0, fmt.Errorf("open evidence upload %s: %w", session.UploadID, err)
	}
	info, statErr := f.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() != int64(offset) {
		var validationErr error
		switch {
		case statErr != nil:
			validationErr = fmt.Errorf("stat open evidence upload %s: %w", session.UploadID, statErr)
		case !info.Mode().IsRegular():
			validationErr = fmt.Errorf("open evidence upload %s is not a regular file", session.UploadID)
		default:
			validationErr = fmt.Errorf("open evidence upload %s has %d bytes, expected %d", session.UploadID, info.Size(), offset)
		}
		closeErr := f.Close()
		_, reconcileErr := reconcileEvidenceUploadFile(session, false)
		err := errors.Join(
			validationErr,
			wrapEvidenceUploadError("close", session.UploadID, closeErr),
			reconcileErr,
		)
		return session, 0, fmt.Errorf("write evidence upload %s failed; next valid offset is %d: %w", session.UploadID, session.ReceivedBytes, err)
	}
	written, writeErr := f.WriteAt(raw, int64(offset))
	syncErr := f.Sync()
	closeErr := f.Close()
	if written > 0 {
		session.ReceivedBytes = offset + written
	}
	observedBytes, reconcileErr := reconcileEvidenceUploadFile(session, false)
	var shortWriteErr error
	if written != len(raw) {
		shortWriteErr = fmt.Errorf("short evidence upload write %s: wrote %d of %d bytes", session.UploadID, written, len(raw))
	}
	var lengthErr error
	if reconcileErr == nil && observedBytes != offset+written {
		lengthErr = fmt.Errorf("evidence upload %s stored byte count %d does not match written prefix %d", session.UploadID, observedBytes, offset+written)
	}
	operationErr := errors.Join(
		wrapEvidenceUploadError("write", session.UploadID, writeErr),
		shortWriteErr,
		wrapEvidenceUploadError("sync", session.UploadID, syncErr),
		wrapEvidenceUploadError("close", session.UploadID, closeErr),
		reconcileErr,
		lengthErr,
	)
	if operationErr != nil {
		return session, written, fmt.Errorf("write evidence upload %s failed; next valid offset is %d: %w", session.UploadID, session.ReceivedBytes, operationErr)
	}
	return session, written, nil
}

func reconcileEvidenceUploadFile(session *EvidenceUploadSession, allowMissing bool) (int, error) {
	info, err := os.Lstat(session.Path)
	if err != nil {
		if allowMissing && session.ReceivedBytes == 0 && errors.Is(err, os.ErrNotExist) {
			session.ReceivedBytes = 0
			return 0, nil
		}
		return session.ReceivedBytes, fmt.Errorf("stat evidence upload %s: %w", session.UploadID, err)
	}
	if !info.Mode().IsRegular() {
		return session.ReceivedBytes, fmt.Errorf("evidence upload %s is not a regular file", session.UploadID)
	}
	if info.Size() > int64(session.ExpectedSizeBytes) {
		return session.ReceivedBytes, fmt.Errorf("evidence upload %s has %d bytes, exceeding expected size %d", session.UploadID, info.Size(), session.ExpectedSizeBytes)
	}
	visibleBytes := int(info.Size())
	if visibleBytes < session.ReceivedBytes {
		return session.ReceivedBytes, fmt.Errorf("evidence upload %s has %d bytes, fewer than accepted byte count %d", session.UploadID, visibleBytes, session.ReceivedBytes)
	}
	session.ReceivedBytes = visibleBytes
	return visibleBytes, nil
}

func wrapEvidenceUploadError(operation string, uploadID string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s evidence upload %s: %w", operation, uploadID, err)
}

func (rc *runContext) prepareEvidenceUploadCommit(opportunity Opportunity, session *EvidenceUploadSession, preferredExt string, expectedSHA256 string) (SubmittedEvidenceMeta, error) {
	if session == nil {
		return SubmittedEvidenceMeta{}, fmt.Errorf("upload session is required")
	}
	if current := rc.uploadSessions[session.UploadID]; current != session {
		return SubmittedEvidenceMeta{}, fmt.Errorf("unknown upload_id %q", session.UploadID)
	}
	if err := validateUploadSessionAuthority(opportunity, session); err != nil {
		return SubmittedEvidenceMeta{}, err
	}
	if submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedEvidencePerSide {
		return SubmittedEvidenceMeta{}, participantInput(fmt.Errorf("submitted_evidence for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedEvidencePerSide))
	}
	if session.ReceivedBytes != session.ExpectedSizeBytes {
		return SubmittedEvidenceMeta{}, participantInput(fmt.Errorf("evidence upload incomplete: received %d of %d bytes", session.ReceivedBytes, session.ExpectedSizeBytes))
	}
	info, err := os.Stat(session.Path)
	if err != nil {
		return SubmittedEvidenceMeta{}, fmt.Errorf("stat evidence upload %s: %w", session.UploadID, err)
	}
	if !info.Mode().IsRegular() || info.Size() != int64(session.ReceivedBytes) {
		return SubmittedEvidenceMeta{}, fmt.Errorf("evidence upload %s byte count does not match session", session.UploadID)
	}
	sha, err := sha256File(session.Path)
	if err != nil {
		return SubmittedEvidenceMeta{}, err
	}
	commitExpectedSHA256 := strings.ToLower(strings.TrimSpace(expectedSHA256))
	sessionExpectedSHA256 := strings.ToLower(strings.TrimSpace(session.ExpectedSHA256))
	if sessionExpectedSHA256 != "" && commitExpectedSHA256 != "" && sessionExpectedSHA256 != commitExpectedSHA256 {
		return SubmittedEvidenceMeta{}, participantInput(fmt.Errorf("commit expected_sha256 %s does not match upload session expected_sha256 %s", commitExpectedSHA256, sessionExpectedSHA256))
	}
	expectedSHA256 = sessionExpectedSHA256
	if expectedSHA256 == "" {
		expectedSHA256 = commitExpectedSHA256
	}
	if expectedSHA256 != "" && expectedSHA256 != sha {
		return SubmittedEvidenceMeta{}, participantInput(fmt.Errorf("evidence upload sha256 mismatch: expected %s, got %s", expectedSHA256, sha))
	}
	name := submittedEvidenceFilename(len(rc.submittedEvidence)+1, session.Role, sha, session.MimeType, preferredExt)
	meta := SubmittedEvidenceMeta{
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
		ParentEvidenceID:   session.ParentEvidenceID,
		ParentSHA256:       session.ParentSHA256,
		DerivationMethod:   session.DerivationMethod,
	}
	if err := rc.validateSubmittedEvidenceID(meta.EvidenceID); err != nil {
		return SubmittedEvidenceMeta{}, participantInput(err)
	}
	if err := rc.validateSubmittedEvidenceLineage(meta); err != nil {
		return SubmittedEvidenceMeta{}, err
	}
	return meta, nil
}
