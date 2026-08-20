package proceeding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jsmorph/adj/arb/runtime/lean"
)

func TestInitialEvidenceCatalogUsesStoredSnapshots(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.txt")
	textPath := filepath.Join(dir, "record.txt")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatalf("write empty input: %v", err)
	}
	if err := os.WriteFile(textPath, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("write text input: %v", err)
	}
	rc := &runContext{
		cfg: Config{OutputDir: dir, Policy: DefaultPolicy()},
		caseFiles: []CaseFile{
			{EvidenceID: "record.txt", Name: "record.txt", Path: textPath, MimeType: "text/plain", TextReadable: true, SizeBytes: 9, Text: "original\n"},
			{EvidenceID: "empty.txt", Name: "empty.txt", Path: emptyPath, MimeType: "text/plain", TextReadable: true},
		},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		t.Fatalf("initializeEvidenceRegistry: %v", err)
	}
	if err := os.WriteFile(textPath, []byte("mutated\n"), 0o644); err != nil {
		t.Fatalf("mutate source: %v", err)
	}
	if len(rc.caseFiles) != 2 {
		t.Fatalf("case files = %d, want 2", len(rc.caseFiles))
	}
	for _, file := range rc.caseFiles {
		rel, err := filepath.Rel(rc.evidenceStoreDir, file.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("case file path %q is outside evidence store %q", file.Path, rc.evidenceStoreDir)
		}
		if file.Name == "record.txt" && file.Text != "original\n" {
			t.Fatalf("stored record text = %q, want original snapshot", file.Text)
		}
		if file.Name == "empty.txt" && file.SizeBytes != 0 {
			t.Fatalf("empty input size = %d, want 0", file.SizeBytes)
		}
	}
	commitments := rc.initialEvidenceCommitments()
	if len(commitments) != 2 || commitments[0].EvidenceID > commitments[1].EvidenceID {
		t.Fatalf("commitments are not sorted: %#v", commitments)
	}
	state := initialState(DefaultPolicy(), "case", []EvidenceCommitment{commitments[1], commitments[0]})
	got, ok := state["evidence_catalog"].([]EvidenceCommitment)
	if !ok || !reflect.DeepEqual(got, commitments) {
		t.Fatalf("initial evidence_catalog = %#v, want %#v", state["evidence_catalog"], commitments)
	}
}

func TestVerifyPublishedFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("record")
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	sum := sha256.Sum256(raw)
	if err := verifyPublishedFile(link, hex.EncodeToString(sum[:]), len(raw)); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("verifyPublishedFile error = %v, want symlink rejection", err)
	}
}

func TestVerifyPublishedFileRejectsSizeMismatch(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("record")
	path := filepath.Join(dir, "record")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write record: %v", err)
	}
	sum := sha256.Sum256(raw)
	err := verifyPublishedFile(path, hex.EncodeToString(sum[:]), len(raw)+1)
	if err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("verifyPublishedFile error = %v, want size mismatch", err)
	}
}

func TestEvidenceConsumersRejectSymlinkReplacement(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("record")
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	target := filepath.Join(dir, "target")
	path := filepath.Join(dir, "stored")
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("create stored-evidence symlink: %v", err)
	}
	meta := EvidenceMeta{
		EvidenceID: "ev_record", SHA256: sha, SizeBytes: len(raw), MimeType: "text/plain",
		StorageName: path, OriginalName: "record.txt", TextReadable: true,
	}
	rc := &runContext{evidenceStoreDir: dir}
	if _, err := rc.caseFileFromEvidence(meta); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("caseFileFromEvidence error = %v, want symlink rejection", err)
	}
	reservation := evidenceReadReservation{
		file:   evidenceFileSnapshot{meta: meta, path: path},
		offset: 0,
		length: len(raw),
	}
	if _, _, err := readReservedEvidenceRange(reservation); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("readReservedEvidenceRange error = %v, want symlink rejection", err)
	}
}

func TestVerifiedEvidenceRangeUsesOpenedFileAfterReplacement(t *testing.T) {
	dir := t.TempDir()
	original := []byte("abcdef")
	sum := sha256.Sum256(original)
	path := filepath.Join(dir, "stored")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}
	file := evidenceFileSnapshot{
		meta: EvidenceMeta{EvidenceID: "ev_original", SHA256: hex.EncodeToString(sum[:]), SizeBytes: len(original)},
		path: path,
	}
	f, err := openPublishedEvidenceFile(file)
	if err != nil {
		t.Fatalf("openPublishedEvidenceFile: %v", err)
	}
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(replacement, []byte("uvwxyz"), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("replace evidence path: %v", err)
	}
	got, readErr := readVerifiedOpenEvidenceFile(f, file, 1, 3)
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("verified read errors = %v, %v", readErr, closeErr)
	}
	if string(got) != "bcd" {
		t.Fatalf("verified range = %q, want bcd", got)
	}
}

func TestReadReservedEvidenceRangeHandlesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	sum := sha256.Sum256(nil)
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty evidence: %v", err)
	}
	reservation := evidenceReadReservation{
		file: evidenceFileSnapshot{
			meta: EvidenceMeta{EvidenceID: "ev_empty", SHA256: hex.EncodeToString(sum[:]), MimeType: "text/plain"},
			path: path,
		},
		offset: 0,
		length: 8,
	}
	result, bytesRead, err := readReservedEvidenceRange(reservation)
	if err != nil {
		t.Fatalf("readReservedEvidenceRange: %v", err)
	}
	if bytesRead != 0 || result["length"] != 0 || result["content_base64"] != "" {
		t.Fatalf("empty evidence result = %#v, bytes = %d", result, bytesRead)
	}
}

func TestOfferedEvidenceUsesVisibleRecordMetadata(t *testing.T) {
	policy := DefaultPolicy()
	payload := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "record", "label": "PX-1"},
		},
	}
	record := EvidenceMeta{EvidenceID: "record", SizeBytes: 12, RecordVisibility: "juror_visible", AdmissibilityStatus: "case_packet"}
	if err := validateAttorneyPayload("submit_argument", payload, map[string]EvidenceMeta{"record": record}, policy); err != nil {
		t.Fatalf("visible record metadata was rejected without a CaseFile: %v", err)
	}
	for name, meta := range map[string]EvidenceMeta{
		"private":   {EvidenceID: "record", SizeBytes: 12, RecordVisibility: "system_private", AdmissibilityStatus: "case_packet"},
		"nonrecord": {EvidenceID: "record", SizeBytes: 12, RecordVisibility: "juror_visible", AdmissibilityStatus: "work_product"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateAttorneyPayload("submit_argument", payload, map[string]EvidenceMeta{"record": meta}, policy); err == nil {
				t.Fatal("non-record offered evidence was accepted")
			}
		})
	}
}

func TestSubmittedEvidenceLineageDerivesParentCommitment(t *testing.T) {
	dir := t.TempDir()
	parentPath := filepath.Join(dir, "parent.txt")
	if err := os.WriteFile(parentPath, []byte("parent\n"), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}
	rc := &runContext{
		cfg: Config{OutputDir: dir, Policy: DefaultPolicy()},
		caseFiles: []CaseFile{{
			EvidenceID: "parent.txt", Name: "parent.txt", Path: parentPath, MimeType: "text/plain", TextReadable: true, SizeBytes: 7, Text: "parent\n",
		}},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		t.Fatalf("initialize parent evidence: %v", err)
	}
	parent := rc.evidence[0]
	params := map[string]any{
		"title":               "Derived source",
		"source_description":  "test derivation",
		"mime_type":           "text/plain",
		"relevance":           "derived fact",
		"content":             "child\n",
		"parent_evidence_id":  parent.EvidenceID,
		"derivation_method":   "normalized line endings",
		"retrieval_timestamp": "2026-08-19T12:00:00Z",
	}
	meta, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, params)
	if err != nil {
		t.Fatalf("prepareSubmittedEvidence: %v", err)
	}
	if meta.ParentEvidenceID != parent.EvidenceID || meta.ParentSHA256 != parent.SHA256 || meta.DerivationMethod != "normalized line endings" {
		t.Fatalf("lineage = %#v, want parent %#v", meta, parent)
	}
	payload := submittedEvidencePayload(meta)
	for key, want := range map[string]string{
		"parent_evidence_id": parent.EvidenceID,
		"parent_sha256":      parent.SHA256,
		"derivation_method":  "normalized line endings",
	} {
		if payload[key] != want {
			t.Fatalf("payload[%q] = %#v, want %q", key, payload[key], want)
		}
	}
	upload, err := rc.beginEvidenceUpload(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title": "Derived upload", "mime_type": "text/plain", "expected_size_bytes": 1,
		"source_description": "test", "relevance": "derived fact", "parent_evidence_id": parent.EvidenceID,
		"derivation_method": "transcoded",
	})
	if err != nil {
		t.Fatalf("begin derived upload: %v", err)
	}
	if upload.ParentSHA256 != parent.SHA256 {
		t.Fatalf("upload parent sha = %q, want %q", upload.ParentSHA256, parent.SHA256)
	}
	if _, _, err := rc.writeEvidenceChunk(Opportunity{Role: "plaintiff", Phase: "arguments"}, upload.UploadID, 0, "eA=="); err != nil {
		t.Fatalf("write derived upload: %v", err)
	}
	uploadMeta, err := rc.prepareEvidenceUploadCommit(Opportunity{Role: "plaintiff", Phase: "arguments"}, upload, "txt", "")
	if err != nil {
		t.Fatalf("prepare derived upload: %v", err)
	}
	if uploadMeta.ParentEvidenceID != parent.EvidenceID || uploadMeta.ParentSHA256 != parent.SHA256 || uploadMeta.DerivationMethod != "transcoded" {
		t.Fatalf("prepared upload lineage = %#v", uploadMeta)
	}

	bad := cloneMap(params)
	delete(bad, "derivation_method")
	if _, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, bad); err == nil {
		t.Fatal("partial lineage was accepted")
	}
	bad = cloneMap(params)
	bad["parent_sha256"] = parent.SHA256
	if _, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, bad); err == nil || !strings.Contains(err.Error(), "must not be supplied") {
		t.Fatalf("caller parent_sha256 error = %v", err)
	}
	bad = cloneMap(params)
	bad["parent_evidence_id"] = "missing"
	if _, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, bad); err == nil {
		t.Fatal("unknown lineage parent was accepted")
	}
	for name, parentMeta := range map[string]EvidenceMeta{
		"private":   {EvidenceID: "private", RecordVisibility: "system_private", AdmissibilityStatus: "case_packet"},
		"nonrecord": {EvidenceID: "nonrecord", RecordVisibility: "juror_visible", AdmissibilityStatus: "work_product"},
	} {
		rc.evidenceByID[name] = parentMeta
		bad = cloneMap(params)
		bad["parent_evidence_id"] = name
		if _, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, bad); err == nil {
			t.Fatalf("%s lineage parent was accepted", name)
		}
	}

	baseParams := cloneMap(params)
	delete(baseParams, "parent_evidence_id")
	delete(baseParams, "derivation_method")
	base, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, baseParams)
	if err != nil {
		t.Fatalf("prepare collision candidate: %v", err)
	}
	rc.evidenceByID[base.EvidenceID] = EvidenceMeta{
		EvidenceID: base.EvidenceID, RecordVisibility: "juror_visible", AdmissibilityStatus: "submitted_evidence",
	}
	self := cloneMap(baseParams)
	self["parent_evidence_id"] = base.EvidenceID
	self["derivation_method"] = "self"
	if _, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, self); err == nil || !strings.Contains(err.Error(), "must not equal") {
		t.Fatalf("self-parent error = %v", err)
	}
	if _, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, baseParams); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("submitted-ID collision error = %v", err)
	}
}

func TestOptionalLineageArgumentsRejectNonStrings(t *testing.T) {
	invalidValues := map[string]any{
		"number":  float64(1),
		"boolean": true,
		"array":   []any{"parent"},
		"object":  map[string]any{"evidence_id": "parent"},
		"null":    nil,
	}
	fields := map[string]string{
		"parent_evidence_id": "derivation_method",
		"derivation_method":  "parent_evidence_id",
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}

	for field, companion := range fields {
		for valueName, value := range invalidValues {
			t.Run("direct/"+field+"/"+valueName, func(t *testing.T) {
				rc := &runContext{cfg: Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()}}
				params := map[string]any{
					"title": "Evidence", "mime_type": "text/plain", "source_description": "test",
					"relevance": "test", "content": "content", field: value, companion: "value",
				}
				_, _, err := rc.prepareSubmittedEvidence(opportunity, params)
				if err == nil || err.Error() != field+" must be a string" {
					t.Fatalf("error = %v, want %s type error", err, field)
				}
			})

			t.Run("chunked/"+field+"/"+valueName, func(t *testing.T) {
				rc := &runContext{cfg: Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()}}
				params := map[string]any{
					"title": "Evidence", "mime_type": "text/plain", "expected_size_bytes": 1,
					"source_description": "test", "relevance": "test", field: value, companion: "value",
				}
				_, err := rc.beginEvidenceUpload(opportunity, params)
				if err == nil || err.Error() != field+" must be a string" {
					t.Fatalf("error = %v, want %s type error", err, field)
				}
			})
		}
	}
}

func TestEvidenceUploadSessionBindsRoleAndPhase(t *testing.T) {
	rc := &runContext{
		cfg:            Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()},
		evidenceByID:   map[string]EvidenceMeta{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	session, err := rc.beginEvidenceUpload(opportunity, map[string]any{
		"title": "Upload", "mime_type": "text/plain", "expected_size_bytes": 3,
		"source_description": "test", "relevance": "test",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload: %v", err)
	}
	if _, _, err := rc.writeEvidenceChunk(Opportunity{Role: "defendant", Phase: "arguments"}, session.UploadID, 0, "YWJj"); err == nil {
		t.Fatal("another role wrote the upload")
	}
	if _, _, err := rc.writeEvidenceChunk(Opportunity{Role: "plaintiff", Phase: "rebuttals"}, session.UploadID, 0, "YWJj"); err == nil {
		t.Fatal("another phase wrote the upload")
	}
	if session.ReceivedBytes != 0 {
		t.Fatalf("received bytes changed after rejected writes: %d", session.ReceivedBytes)
	}
	if _, _, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 0, "YWJj"); err != nil {
		t.Fatalf("writeEvidenceChunk: %v", err)
	}
	if _, err := rc.prepareEvidenceUploadCommit(Opportunity{Role: "defendant", Phase: "arguments"}, session, "txt", ""); err == nil {
		t.Fatal("another role prepared the upload commit")
	}
	wantExpected := session.ExpectedSHA256
	if _, err := rc.prepareEvidenceUploadCommit(opportunity, session, "txt", strings.Repeat("0", 64)); err == nil {
		t.Fatal("incorrect commit hash was accepted")
	}
	if session.ExpectedSHA256 != wantExpected {
		t.Fatalf("failed commit mutated session expected sha: %q", session.ExpectedSHA256)
	}
	if _, err := rc.prepareEvidenceUploadCommit(opportunity, session, "txt", ""); err != nil {
		t.Fatalf("prepareEvidenceUploadCommit: %v", err)
	}
}

func TestEvidenceUploadCommitRechecksSubmittedEvidenceCount(t *testing.T) {
	dir := t.TempDir()
	policy := DefaultPolicy()
	policy.MaxSubmittedEvidencePerSide = 1
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    policy,
		},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	params := map[string]any{
		"title":               "Source",
		"mime_type":           "text/plain",
		"expected_size_bytes": 1,
		"source_description":  "Submitted during argument.",
		"relevance":           "Record evidence.",
	}
	first, err := rc.beginEvidenceUpload(opportunity, params)
	if err != nil {
		t.Fatalf("begin first upload: %v", err)
	}
	second, err := rc.beginEvidenceUpload(opportunity, params)
	if err != nil {
		t.Fatalf("begin second upload: %v", err)
	}
	for _, session := range []*EvidenceUploadSession{first, second} {
		if _, _, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 0, "eA=="); err != nil {
			t.Fatalf("write upload %s: %v", session.UploadID, err)
		}
	}
	firstMeta, err := rc.prepareEvidenceUploadCommit(opportunity, first, "txt", "")
	if err != nil {
		t.Fatalf("prepare first commit: %v", err)
	}
	rc.submittedEvidence = append(rc.submittedEvidence, firstMeta)
	_, err = rc.prepareEvidenceUploadCommit(opportunity, second, "txt", "")
	if err == nil || !isParticipantInput(err) || !strings.Contains(err.Error(), "submitted_evidence for this side exceed limit of 1") {
		t.Fatalf("prepare second commit error = %v, want participant count error", err)
	}
}

func TestEvidenceUploadCommitRejectsChangedExpectedSHAWithoutMutatingSession(t *testing.T) {
	raw := []byte("abc")
	commitSum := sha256.Sum256(raw)
	commitSHA := hex.EncodeToString(commitSum[:])
	beginSum := sha256.Sum256([]byte("different commitment"))
	beginSHA := hex.EncodeToString(beginSum[:])
	rc := &runContext{
		cfg:            Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()},
		evidenceByID:   map[string]EvidenceMeta{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	session, err := rc.beginEvidenceUpload(opportunity, map[string]any{
		"title": "Upload", "mime_type": "text/plain", "expected_size_bytes": len(raw),
		"expected_sha256": beginSHA, "source_description": "test", "relevance": "test",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload: %v", err)
	}
	if _, _, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 0, "YWJj"); err != nil {
		t.Fatalf("writeEvidenceChunk: %v", err)
	}
	wantSession := *session
	if _, err := rc.prepareEvidenceUploadCommit(opportunity, session, "txt", commitSHA); err == nil {
		t.Fatal("commit replaced the begin-time expected sha256")
	}
	if !reflect.DeepEqual(*session, wantSession) {
		t.Fatalf("failed commit mutated upload session: got %#v, want %#v", *session, wantSession)
	}
	if rc.uploadSessions[session.UploadID] != session {
		t.Fatal("failed commit replaced or removed the upload session")
	}
}

func TestEvidenceChunkRetryReconcilesStagingFileAheadOfSession(t *testing.T) {
	rc := &runContext{
		cfg:            Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()},
		evidenceByID:   map[string]EvidenceMeta{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	session, err := rc.beginEvidenceUpload(opportunity, map[string]any{
		"title": "Upload", "mime_type": "text/plain", "expected_size_bytes": 6,
		"source_description": "test", "relevance": "test",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload: %v", err)
	}
	if _, _, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 0, "YWJj"); err != nil {
		t.Fatalf("write initial chunk: %v", err)
	}
	if err := os.WriteFile(session.Path, []byte("abcd"), 0o644); err != nil {
		t.Fatalf("write staging prefix ahead of session: %v", err)
	}
	if session.ReceivedBytes != 3 {
		t.Fatalf("received bytes = %d before retry, want 3", session.ReceivedBytes)
	}
	if _, _, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 3, "ZGVm"); err == nil || !strings.Contains(err.Error(), "next valid offset is 4") {
		t.Fatalf("stale retry error = %v, want next valid offset 4", err)
	}
	if session.ReceivedBytes != 4 {
		t.Fatalf("received bytes after reconciliation = %d, want 4", session.ReceivedBytes)
	}
	gotSession, written, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 4, "ZWY=")
	if err != nil {
		t.Fatalf("retry evidence chunk at reconciled offset: %v", err)
	}
	if gotSession != session || written != 2 || session.ReceivedBytes != 6 {
		t.Fatalf("retry result: session=%p, want %p; written=%d, want 2; received=%d, want 6", gotSession, session, written, session.ReceivedBytes)
	}
	got, err := os.ReadFile(session.Path)
	if err != nil {
		t.Fatalf("read reconciled staging file: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("reconciled staging content = %q, want %q", got, "abcdef")
	}
}

func TestEvidenceSubmissionSchemasAcceptCallerLineageInputsOnly(t *testing.T) {
	for name, schema := range map[string]map[string]any{
		"direct": submittedEvidenceSchema(),
		"upload": beginEvidenceUploadSchema(),
	} {
		properties := mapAny(schema["properties"])
		if properties["parent_evidence_id"] == nil || properties["derivation_method"] == nil {
			t.Fatalf("%s schema lacks caller lineage fields: %#v", name, properties)
		}
		if properties["parent_sha256"] != nil {
			t.Fatalf("%s schema permits caller-supplied parent_sha256", name)
		}
	}
}

func TestDirectEvidencePublicationCanReuseFilesAfterManifestFailure(t *testing.T) {
	dir := t.TempDir()
	api, turn := newSubmissionTransactionTestAPI(t, dir)
	args := transactionEvidenceArgs()
	manifestPath := filepath.Join(dir, "evidence-manifest.json")
	if err := os.Mkdir(manifestPath, 0o755); err != nil {
		t.Fatalf("block manifest path: %v", err)
	}
	initialStateHash, err := canonicalJSONSHA256(api.rc.state)
	if err != nil {
		t.Fatalf("hash initial state: %v", err)
	}
	api.rc.mu.Lock()
	_, err = api.submitEvidenceLocked(context.Background(), turn, args)
	api.rc.mu.Unlock()
	if err == nil {
		t.Fatal("submission succeeded with blocked manifest path")
	}
	assertSubmissionUncommitted(t, api, turn, initialStateHash)
	entries, err := os.ReadDir(filepath.Join(dir, "submitted-evidence"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("orphan submitted copy count = %d, %v", len(entries), err)
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("unblock manifest path: %v", err)
	}
	api.rc.mu.Lock()
	result, err := api.submitEvidenceLocked(context.Background(), turn, args)
	api.rc.mu.Unlock()
	if err != nil {
		t.Fatalf("retry submission: %v", err)
	}
	if result["evidence_id"] == "" || len(api.rc.evidence) != 1 || len(api.rc.certificateActions) != 1 {
		t.Fatalf("retry result=%#v evidence=%#v actions=%#v", result, api.rc.evidence, api.rc.certificateActions)
	}
}

func TestUploadPublicationCanReuseSessionAfterManifestFailure(t *testing.T) {
	dir := t.TempDir()
	api, turn := newSubmissionTransactionTestAPI(t, dir)
	session, err := api.rc.beginEvidenceUpload(turn.opportunity, map[string]any{
		"title": "Upload", "mime_type": "text/plain", "expected_size_bytes": 3,
		"source_description": "test", "relevance": "test",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload: %v", err)
	}
	if _, _, err := api.rc.writeEvidenceChunk(turn.opportunity, session.UploadID, 0, "YWJj"); err != nil {
		t.Fatalf("writeEvidenceChunk: %v", err)
	}
	stagingPath := session.Path
	manifestPath := filepath.Join(dir, "evidence-manifest.json")
	if err := os.Mkdir(manifestPath, 0o755); err != nil {
		t.Fatalf("block manifest path: %v", err)
	}
	initialStateHash, err := canonicalJSONSHA256(api.rc.state)
	if err != nil {
		t.Fatalf("hash initial state: %v", err)
	}
	api.rc.mu.Lock()
	_, err = api.commitEvidenceUploadLocked(context.Background(), turn, map[string]any{"upload_id": session.UploadID, "preferred_filename_ext": "txt"})
	api.rc.mu.Unlock()
	if err == nil {
		t.Fatal("upload commit succeeded with blocked manifest path")
	}
	assertSubmissionUncommitted(t, api, turn, initialStateHash)
	if api.rc.uploadSessions[session.UploadID] != session {
		t.Fatal("failed upload commit removed the session")
	}
	if session.Path == stagingPath || !strings.Contains(session.Path, "submitted-evidence") {
		t.Fatalf("retry path = %q, staging path = %q", session.Path, stagingPath)
	}
	if _, err := os.Stat(session.Path); err != nil {
		t.Fatalf("retry source is unavailable: %v", err)
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("unblock manifest path: %v", err)
	}
	api.rc.mu.Lock()
	result, err := api.commitEvidenceUploadLocked(context.Background(), turn, map[string]any{"upload_id": session.UploadID, "preferred_filename_ext": "txt"})
	api.rc.mu.Unlock()
	if err != nil {
		t.Fatalf("retry upload commit: %v", err)
	}
	if result["evidence_id"] == "" || api.rc.uploadSessions[session.UploadID] != nil || len(api.rc.certificateActions) != 1 {
		t.Fatalf("retry result=%#v sessions=%#v actions=%#v", result, api.rc.uploadSessions, api.rc.certificateActions)
	}
}

func TestSubmittedCopyPublicationFailureDoesNotCommit(t *testing.T) {
	dir := t.TempDir()
	api, turn := newSubmissionTransactionTestAPI(t, dir)
	args := transactionEvidenceArgs()
	meta, _, err := api.rc.prepareSubmittedEvidence(turn.opportunity, args)
	if err != nil {
		t.Fatalf("prepare submitted evidence: %v", err)
	}
	copyPath, err := submittedEvidenceCopyPath(dir, meta)
	if err != nil {
		t.Fatalf("submittedEvidenceCopyPath: %v", err)
	}
	if err := os.MkdirAll(copyPath, 0o755); err != nil {
		t.Fatalf("block submitted-copy path: %v", err)
	}
	initialStateHash, err := canonicalJSONSHA256(api.rc.state)
	if err != nil {
		t.Fatalf("hash initial state: %v", err)
	}
	api.rc.mu.Lock()
	_, err = api.submitEvidenceLocked(context.Background(), turn, args)
	api.rc.mu.Unlock()
	if err == nil || !strings.Contains(err.Error(), "submitted evidence copy") {
		t.Fatalf("publication error = %v", err)
	}
	assertSubmissionUncommitted(t, api, turn, initialStateHash)
}

func TestAtomicPublicationRejectsChangedSourceCommitment(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "published")
	wrong := sha256.Sum256([]byte("different"))
	if err := publishBytesAtomic([]byte("source"), dst, hex.EncodeToString(wrong[:]), 0o444); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("publishBytesAtomic error = %v, want sha256 mismatch", err)
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Fatalf("invalid publication exists at %s: %v", dst, err)
	}
}

func newSubmissionTransactionTestAPI(t *testing.T, dir string) (*lawyerAPIServer, *lawyerTurn) {
	t.Helper()
	enginePath := filepath.Join(dir, "engine.sh")
	responsePath := filepath.Join(dir, "engine-response.json")
	script := "#!/bin/sh\ncat >/dev/null\ncat \"$1\"\nprintf '\\n'\n"
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	writeSubmissionTransactionEngineResponse(t, responsePath, map[string]any{
		"case": map[string]any{
			"status":             "active",
			"phase":              "arguments",
			"submitted_evidence": []map[string]any{},
		},
		"state_version": 1,
	})
	policy := DefaultPolicy()
	state := initialState(policy, "arb-1", nil)
	caseObj := mapAny(state["case"])
	caseObj["status"] = "active"
	caseObj["phase"] = "arguments"
	rc := &runContext{
		cfg: Config{
			CaseID: "arb-1", OutputDir: dir, Policy: policy, Runtime: DefaultRuntimeLimits(),
			Engine: lean.New([]string{enginePath, responsePath}),
		},
		state:              state,
		caseFiles:          []CaseFile{},
		fileByID:           map[string]CaseFile{},
		submittedEvidence:  []SubmittedEvidenceMeta{},
		evidence:           []EvidenceMeta{},
		evidenceByID:       map[string]EvidenceMeta{},
		evidenceStoreDir:   filepath.Join(dir, "evidence-store"),
		uploadSessions:     map[string]*EvidenceUploadSession{},
		certificateActions: []ReplayAction{},
	}
	turn := testLawyerTurn("arguments:plaintiff", "plaintiff", "arguments")
	turn.opportunity.StateVersion = 0
	turn.opportunity.AllowedTools = []string{"submit_evidence", "submit_argument"}
	api := newLawyerAPIServer(rc)
	api.active = turn
	return api, turn
}

func writeSubmissionTransactionEngineResponse(t *testing.T, path string, state map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"ok": true, "state": state})
	if err != nil {
		t.Fatalf("encode engine response: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write engine response: %v", err)
	}
}

func transactionEvidenceArgs() map[string]any {
	return map[string]any{
		"title": "Source", "source_url": "https://example.test/source", "mime_type": "text/plain",
		"relevance": "Shows the fact.", "content": "source evidence\n", "preferred_filename_ext": "txt",
	}
}

func assertSubmissionUncommitted(t *testing.T, api *lawyerAPIServer, turn *lawyerTurn, initialStateHash string) {
	t.Helper()
	stateHash, err := canonicalJSONSHA256(api.rc.state)
	if err != nil {
		t.Fatalf("hash failed-submission state: %v", err)
	}
	if stateHash != initialStateHash {
		t.Fatal("failed submission replaced state")
	}
	if turn.opportunity.StateVersion != 0 {
		t.Fatalf("failed submission changed turn state version to %d", turn.opportunity.StateVersion)
	}
	if len(api.rc.certificateActions) != 0 || len(api.rc.evidence) != 0 || len(api.rc.evidenceByID) != 0 || len(api.rc.caseFiles) != 0 || len(api.rc.fileByID) != 0 || len(api.rc.submittedEvidence) != 0 {
		t.Fatalf("failed submission mutated registries: actions=%#v evidence=%#v evidenceByID=%#v caseFiles=%#v fileByID=%#v submitted=%#v", api.rc.certificateActions, api.rc.evidence, api.rc.evidenceByID, api.rc.caseFiles, api.rc.fileByID, api.rc.submittedEvidence)
	}
	if len(api.rc.events) != 0 {
		t.Fatalf("failed submission recorded events: %#v", api.rc.events)
	}
}

func TestSubmittedEvidenceLineagePersistsInStateManifestEventAndCertificateAction(t *testing.T) {
	dir := t.TempDir()
	api, turn := newSubmissionTransactionTestAPI(t, dir)
	parentRaw := []byte("parent")
	parentSum := sha256.Sum256(parentRaw)
	parentSHA := hex.EncodeToString(parentSum[:])
	storageName, err := evidenceStorageName(parentSHA)
	if err != nil {
		t.Fatalf("evidenceStorageName: %v", err)
	}
	parent := EvidenceMeta{
		EvidenceID: "ev_parent", SHA256: parentSHA, SizeBytes: len(parentRaw), StorageName: storageName,
		AdmissibilityStatus: "case_packet", RecordVisibility: "juror_visible",
	}
	parentPath := filepath.Join(api.rc.evidenceStoreDir, filepath.FromSlash(storageName))
	if err := publishBytesAtomic(parentRaw, parentPath, parentSHA, 0o444); err != nil {
		t.Fatalf("publish parent: %v", err)
	}
	api.rc.evidence = []EvidenceMeta{parent}
	api.rc.evidenceByID[parent.EvidenceID] = parent
	api.rc.state["evidence_catalog"] = []EvidenceCommitment{{
		EvidenceID: parent.EvidenceID,
		SHA256:     parent.SHA256,
		SizeBytes:  parent.SizeBytes,
	}}
	args := transactionEvidenceArgs()
	args["parent_evidence_id"] = parent.EvidenceID
	args["derivation_method"] = "excerpt"
	wantSubmitted, _, err := api.rc.prepareSubmittedEvidence(turn.opportunity, args)
	if err != nil {
		t.Fatalf("prepare expected submitted evidence: %v", err)
	}
	acceptedState, err := cloneMapJSON(api.rc.state)
	if err != nil {
		t.Fatalf("clone accepted state: %v", err)
	}
	acceptedState["state_version"] = 1
	acceptedSubmitted := submittedEvidencePayload(wantSubmitted)
	acceptedSubmitted["phase"] = wantSubmitted.Phase
	acceptedSubmitted["role"] = wantSubmitted.Role
	mapAny(acceptedState["case"])["submitted_evidence"] = []map[string]any{acceptedSubmitted}
	writeSubmissionTransactionEngineResponse(t, filepath.Join(dir, "engine-response.json"), acceptedState)
	api.rc.mu.Lock()
	result, err := api.submitEvidenceLocked(context.Background(), turn, args)
	api.rc.mu.Unlock()
	if err != nil {
		t.Fatalf("submit derived evidence: %v", err)
	}
	evidenceID := mapString(result["evidence_id"])
	if evidenceID != wantSubmitted.EvidenceID {
		t.Fatalf("accepted evidence_id = %q, want %q", evidenceID, wantSubmitted.EvidenceID)
	}
	meta := api.rc.evidenceByID[evidenceID]
	if meta.ParentEvidenceID != parent.EvidenceID || meta.ParentSHA256 != parentSHA || meta.DerivationMethod != "excerpt" {
		t.Fatalf("registry lineage = %#v", meta)
	}
	if len(api.rc.submittedEvidence) != 1 ||
		api.rc.submittedEvidence[0].ParentEvidenceID != parent.EvidenceID ||
		api.rc.submittedEvidence[0].ParentSHA256 != parentSHA ||
		api.rc.submittedEvidence[0].DerivationMethod != "excerpt" {
		t.Fatalf("submitted metadata lineage = %#v", api.rc.submittedEvidence)
	}
	stateSubmitted := mapList(mapAny(api.rc.state["case"])["submitted_evidence"])
	if len(stateSubmitted) != 1 ||
		mapString(stateSubmitted[0]["parent_evidence_id"]) != parent.EvidenceID ||
		mapString(stateSubmitted[0]["parent_sha256"]) != parentSHA ||
		mapString(stateSubmitted[0]["derivation_method"]) != "excerpt" {
		t.Fatalf("accepted state lineage = %#v", stateSubmitted)
	}
	if len(api.rc.certificateActions) != 1 {
		t.Fatalf("certificate actions = %d, want 1", len(api.rc.certificateActions))
	}
	assertLineage := func(label string, payload map[string]any) {
		t.Helper()
		if payload["parent_evidence_id"] != parent.EvidenceID || payload["parent_sha256"] != parentSHA || payload["derivation_method"] != "excerpt" {
			t.Fatalf("%s lineage = %#v", label, payload)
		}
	}
	assertLineage("certificate action", api.rc.certificateActions[0].Payload)
	if len(api.rc.events) != 1 {
		t.Fatalf("event lineage = %#v", api.rc.events)
	}
	assertLineage("in-memory event", api.rc.events[0].Payload)
	rawEvents, err := os.ReadFile(filepath.Join(dir, "events.ndjson"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var durableEvent Event
	if err := json.Unmarshal(rawEvents, &durableEvent); err != nil {
		t.Fatalf("decode durable event: %v", err)
	}
	if durableEvent.Type != "submitted_evidence" {
		t.Fatalf("durable event type = %q, want submitted_evidence", durableEvent.Type)
	}
	assertLineage("durable event", durableEvent.Payload)
	rawManifest, err := os.ReadFile(filepath.Join(dir, "evidence-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Evidence []EvidenceMeta `json:"evidence"`
	}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	var submitted EvidenceMeta
	for _, item := range manifest.Evidence {
		if item.EvidenceID == evidenceID {
			submitted = item
		}
	}
	if len(manifest.Evidence) != 2 || submitted.ParentEvidenceID != parent.EvidenceID || submitted.ParentSHA256 != parentSHA || submitted.DerivationMethod != "excerpt" {
		t.Fatalf("manifest lineage = %#v", manifest.Evidence)
	}
}
