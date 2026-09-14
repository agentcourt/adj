package proceeding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agentcourt/adj/arbd/runtime/lean"
	"github.com/agentcourt/adj/arbd/runtime/spec"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

func TestLoadCaseFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("situation.md", "# Question\n\nP\n")
	write("complaint.md", "# Question\n\nP\n")
	write("instructions.txt", "hello")
	write("samantha_public.pem", "pem")

	files, err := loadCaseFiles(dir, filepath.Join(dir, "complaint.md"))
	if err != nil {
		t.Fatalf("loadCaseFiles returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("loadCaseFiles returned %d files, want 2", len(files))
	}
	if files[0].EvidenceID != "instructions.txt" || files[1].EvidenceID != "samantha_public.pem" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestSampleCouncilCarriesJSONRequestSpec(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "personas"), 0o755); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "personas", "juror.txt"), []byte("skeptical juror"), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	pool := filepath.Join(dir, "pool.jsonl")
	line := `{"openrouter_model_id":"deepseek/deepseek-v4-flash","endpoint_tag":"deepinfra/fp4","quantization":"fp4","request":{"temperature":0,"top_p":1,"max_tokens":32},"persona":"personas/juror.txt"}`
	if err := os.WriteFile(pool, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write pool: %v", err)
	}

	council, err := sampleCouncil(pool, dir, 1)
	if err != nil {
		t.Fatalf("sampleCouncil error = %v", err)
	}
	seat := council[0]
	if seat.Model != "openrouter://deepseek/deepseek-v4-flash" {
		t.Fatalf("seat.Model = %q", seat.Model)
	}
	if seat.RequestSpec == nil {
		t.Fatalf("seat.RequestSpec = nil")
	}
	provider := seat.RequestSpec.ProviderBody()
	if provider["only"].([]string)[0] != "deepinfra/fp4" {
		t.Fatalf("provider = %#v", provider)
	}
	if seat.PersonaFile != "personas/juror.txt" || seat.PersonaText != "skeptical juror" {
		t.Fatalf("persona = %q/%q", seat.PersonaFile, seat.PersonaText)
	}
}

func TestSampleCouncilRejectsLegacyPoolRecord(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "personas"), 0o755); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "personas", "juror.txt"), []byte("skeptical juror"), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	pool := filepath.Join(dir, "pool.csv")
	if err := os.WriteFile(pool, []byte("openrouter://openai/gpt-5,personas/juror.txt\n"), 0o644); err != nil {
		t.Fatalf("write pool: %v", err)
	}

	_, err := sampleCouncil(pool, dir, 1)
	if err == nil || !strings.Contains(err.Error(), "request_spec") {
		t.Fatalf("sampleCouncil error = %v, want request_spec error", err)
	}
}

func makeTempCouncilPool(t *testing.T, line string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "personas"), 0o755); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "personas", "juror.txt"), []byte("skeptical juror"), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	pool := filepath.Join(dir, "pool.jsonl")
	if err := os.WriteFile(pool, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write pool: %v", err)
	}
	return pool
}

func TestEvidenceRegistryStoresCaseFilesAndReadsBoundedRanges(t *testing.T) {
	dir := t.TempDir()
	casePath := filepath.Join(dir, "source.txt")
	body := []byte("abcdef")
	if err := os.WriteFile(casePath, body, 0o644); err != nil {
		t.Fatalf("write case file: %v", err)
	}
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		caseFiles: []CaseFile{{
			EvidenceID:   "source.txt",
			Name:         "source.txt",
			Path:         casePath,
			MimeType:     "text/plain",
			TextReadable: true,
			SizeBytes:    len(body),
			Text:         string(body),
		}},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		t.Fatalf("initializeEvidenceRegistry returned error: %v", err)
	}
	if len(rc.evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(rc.evidence))
	}
	evidence := rc.evidence[0]
	if !strings.HasPrefix(evidence.EvidenceID, "ev_") || evidence.SHA256 == "" || evidence.StorageName == "" {
		t.Fatalf("evidence metadata = %#v", evidence)
	}
	if rc.caseFiles[0].EvidenceID != evidence.EvidenceID {
		t.Fatalf("case file evidence id = %q, want %q", rc.caseFiles[0].EvidenceID, evidence.EvidenceID)
	}
	if _, ok := rc.fileByID[evidence.EvidenceID]; !ok {
		t.Fatalf("fileByID missing canonical evidence id %q", evidence.EvidenceID)
	}
	if _, ok := rc.fileByID["source.txt"]; ok {
		t.Fatalf("fileByID retained filename key after canonical evidence registration")
	}
	rawManifest, err := os.ReadFile(filepath.Join(dir, "evidence-manifest.json"))
	if err != nil {
		t.Fatalf("read evidence manifest: %v", err)
	}
	var manifest struct {
		EvidenceCount int            `json:"evidence_count"`
		Evidence      []EvidenceMeta `json:"evidence"`
	}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode evidence manifest: %v", err)
	}
	if manifest.EvidenceCount != 1 || len(manifest.Evidence) != 1 || manifest.Evidence[0].EvidenceID != evidence.EvidenceID {
		t.Fatalf("manifest evidence = %#v, want %q", manifest, evidence.EvidenceID)
	}
	budget := &evidenceReadBudget{}
	rc.mu.Lock()
	reservation, err := rc.reserveEvidenceReadLocked(evidence.EvidenceID, 1, 3, budget)
	rc.mu.Unlock()
	if err != nil {
		t.Fatalf("reserveEvidenceReadLocked returned error: %v", err)
	}
	got, bytesRead, err := readReservedEvidenceRange(reservation)
	rc.mu.Lock()
	if err != nil {
		rollbackEvidenceReadLocked(&reservation)
	} else {
		finalizeEvidenceReadLocked(&reservation, bytesRead)
	}
	rc.mu.Unlock()
	if err != nil {
		t.Fatalf("readReservedEvidenceRange returned error: %v", err)
	}
	if got["content_base64"] != "YmNk" || got["length"] != 3 {
		t.Fatalf("read result = %#v", got)
	}
}

func TestEvidenceManifestUsesEmptyArrayForNoEvidence(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		t.Fatalf("initializeEvidenceRegistry returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "evidence-manifest.json"))
	if err != nil {
		t.Fatalf("read evidence manifest: %v", err)
	}
	if strings.Contains(string(raw), `"evidence": null`) {
		t.Fatalf("manifest used null evidence array: %s", raw)
	}
	var manifest struct {
		EvidenceCount int            `json:"evidence_count"`
		Evidence      []EvidenceMeta `json:"evidence"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode evidence manifest: %v", err)
	}
	if manifest.EvidenceCount != 0 || len(manifest.Evidence) != 0 {
		t.Fatalf("manifest evidence = %#v, want empty", manifest)
	}
}

func TestPrepareSubmittedEvidencePreservesContentAndBuildsVisibleFile(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		submittedEvidence: []SubmittedEvidenceMeta{},
		evidenceStoreDir:  filepath.Join(dir, "evidence-store"),
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	content := "  exact text\n"
	meta, raw, err := rc.prepareSubmittedEvidence(opportunity, map[string]any{
		"title":                  "Source post",
		"source_url":             "https://example.test/post",
		"mime_type":              "text/plain",
		"relevance":              "Shows the disputed announcement.",
		"content":                content,
		"retrieval_timestamp":    "2026-05-14T23:00:00Z",
		"preferred_filename_ext": "txt",
	})
	if err != nil {
		t.Fatalf("prepareSubmittedEvidence returned error: %v", err)
	}
	if string(raw) != content {
		t.Fatalf("raw content = %q, want %q", string(raw), content)
	}
	sum := sha256.Sum256([]byte(content))
	wantSHA := hex.EncodeToString(sum[:])
	if meta.SHA256 != wantSHA {
		t.Fatalf("sha = %s, want %s", meta.SHA256, wantSHA)
	}
	evidence, err := submittedEvidenceRecordMeta(meta, true)
	if err != nil {
		t.Fatalf("submittedEvidenceRecordMeta returned error: %v", err)
	}
	file, _, err := rc.publishSubmittedEvidence(meta, evidence, raw, "")
	if err != nil {
		t.Fatalf("publishSubmittedEvidence returned error: %v", err)
	}
	if file.EvidenceID != meta.EvidenceID || !file.TextReadable || file.Text != content {
		t.Fatalf("written file metadata = %#v", file)
	}
	written, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("read written evidence: %v", err)
	}
	if string(written) != content {
		t.Fatalf("written content = %q, want %q", string(written), content)
	}
}

func TestChunkedEvidenceUploadPreparesSubmittedEvidence(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		evidenceByID:     map[string]EvidenceMeta{},
		evidenceStoreDir: filepath.Join(dir, "evidence-store"),
		uploadSessions:   map[string]*EvidenceUploadSession{},
	}
	raw := []byte("abcdef")
	sha := sha256.Sum256(raw)
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	session, err := rc.beginEvidenceUpload(opportunity, map[string]any{
		"title":               "Binary source",
		"mime_type":           "application/octet-stream",
		"expected_size_bytes": int64(len(raw)),
		"expected_sha256":     hex.EncodeToString(sha[:]),
		"source_description":  "test source",
		"relevance":           "test relevance",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload returned error: %v", err)
	}
	if _, n, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 0, "YWJj"); err != nil || n != 3 {
		t.Fatalf("write first chunk = session, %d, %v", n, err)
	}
	if _, n, err := rc.writeEvidenceChunk(opportunity, session.UploadID, 3, "ZGVm"); err != nil || n != 3 {
		t.Fatalf("write second chunk = session, %d, %v", n, err)
	}
	meta, err := rc.prepareEvidenceUploadCommit(opportunity, session, "bin", "")
	if err != nil {
		t.Fatalf("prepareEvidenceUploadCommit returned error: %v", err)
	}
	fileMeta := submittedEvidencePayload(meta)
	if fileMeta["evidence_id"] != meta.EvidenceID {
		t.Fatalf("submitted evidence payload missing evidence_id: %#v", fileMeta)
	}
	evidence, err := submittedEvidenceRecordMeta(meta, false)
	if err != nil {
		t.Fatalf("submittedEvidenceRecordMeta returned error: %v", err)
	}
	file, _, err := rc.publishSubmittedEvidence(meta, evidence, nil, session.Path)
	if err != nil {
		t.Fatalf("publishSubmittedEvidence returned error: %v", err)
	}
	if meta.EvidenceID == "" || file.EvidenceID != meta.EvidenceID || evidence.EvidenceID != meta.EvidenceID {
		t.Fatalf("meta=%#v file=%#v evidence=%#v", meta, file, evidence)
	}
	if got, err := os.ReadFile(file.Path); err != nil || string(got) != string(raw) {
		t.Fatalf("uploaded file = %q, %v", string(got), err)
	}
}

func TestSubmittedEvidenceRegistersEvidence(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		evidenceByID:     map[string]EvidenceMeta{},
		evidenceStoreDir: filepath.Join(dir, "evidence-store"),
	}
	sha := sha256.Sum256([]byte("source"))
	shaHex := hex.EncodeToString(sha[:])
	name := "submitted-evidence-01-plaintiff-abcd.txt"
	meta := SubmittedEvidenceMeta{
		Phase:              "arguments",
		Role:               "plaintiff",
		EvidenceID:         evidenceIDForFile(shaHex, name),
		Name:               name,
		Title:              "Source",
		SourceURL:          "https://example.test/source",
		MimeType:           "text/plain",
		RetrievalTimestamp: "2026-05-21T12:00:00Z",
		Relevance:          "Shows the fact.",
		SHA256:             shaHex,
		SizeBytes:          len("source"),
	}
	file := CaseFile{EvidenceID: meta.EvidenceID, Name: meta.Name, Path: filepath.Join(dir, meta.Name), MimeType: meta.MimeType, TextReadable: true, Text: "source"}
	if err := os.WriteFile(file.Path, []byte(file.Text), 0o644); err != nil {
		t.Fatalf("write evidence file: %v", err)
	}
	evidence, err := submittedEvidenceRecordMeta(meta, true)
	if err != nil {
		t.Fatalf("submittedEvidenceRecordMeta returned error: %v", err)
	}
	if _, _, err := rc.publishSubmittedEvidence(meta, evidence, nil, file.Path); err != nil {
		t.Fatalf("publishSubmittedEvidence returned error: %v", err)
	}
	if evidence.AdmissibilityStatus != "submitted_evidence" || evidence.SubmittedByRole != "plaintiff" || evidence.EvidenceID != meta.EvidenceID {
		t.Fatalf("evidence metadata = %#v", evidence)
	}
	if _, err := os.Stat(filepath.Join(dir, "evidence-store", filepath.FromSlash(evidence.StorageName))); err != nil {
		t.Fatalf("stored evidence not found: %v", err)
	}
}

func TestAddEvidenceRejectsSameIDForDifferentBytes(t *testing.T) {
	rc := &runContext{}
	_, err := rc.addEvidence(EvidenceMeta{EvidenceID: "ev_same", SHA256: "aaa", SizeBytes: 3, StorageName: "aa/aaa"})
	if err != nil {
		t.Fatalf("add first evidence returned error: %v", err)
	}
	_, err = rc.addEvidence(EvidenceMeta{EvidenceID: "ev_same", SHA256: "bbb", SizeBytes: 3, StorageName: "bb/bbb"})
	if err == nil || !strings.Contains(err.Error(), "evidence_id collision") {
		t.Fatalf("add conflicting evidence error = %v, want collision", err)
	}
	_, err = rc.addEvidence(EvidenceMeta{EvidenceID: "ev_same", SHA256: "aaa", SizeBytes: 3, StorageName: "aa/aaa", ParentEvidenceID: "parent"})
	if err == nil || !strings.Contains(err.Error(), "metadata differs") {
		t.Fatalf("add same-byte metadata conflict error = %v, want metadata conflict", err)
	}
	if rc.evidenceByID["ev_same"].ParentEvidenceID != "" {
		t.Fatalf("evidence metadata was overwritten: %#v", rc.evidenceByID["ev_same"])
	}
}

func TestAddEvidenceAllowsIdempotentRegistration(t *testing.T) {
	rc := &runContext{}
	meta := EvidenceMeta{
		EvidenceID:          "ev_abc123_source",
		SHA256:              "abc123",
		SizeBytes:           6,
		MimeType:            "text/plain",
		StorageName:         "ab/abc123",
		CreatedAt:           "2026-05-21T20:00:00Z",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "council_visible",
		Title:               "source.txt",
		OriginalName:        "source.txt",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		TextReadable:        true,
	}
	first, err := rc.addEvidence(meta)
	if err != nil {
		t.Fatalf("first addEvidence returned error: %v", err)
	}
	meta.CreatedAt = "2026-05-21T20:01:00Z"
	second, err := rc.addEvidence(meta)
	if err != nil {
		t.Fatalf("second addEvidence returned error: %v", err)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("idempotent registration replaced existing metadata: first=%#v second=%#v", first, second)
	}
	if len(rc.evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(rc.evidence))
	}
}

func TestAddEvidenceRejectsMetadataConflict(t *testing.T) {
	rc := &runContext{}
	meta := EvidenceMeta{
		EvidenceID:          "ev_abc123_source",
		SHA256:              "abc123",
		SizeBytes:           6,
		MimeType:            "text/plain",
		StorageName:         "ab/abc123",
		CreatedAt:           "2026-05-21T20:00:00Z",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "council_visible",
		Title:               "source.txt",
		OriginalName:        "source.txt",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		TextReadable:        true,
	}
	if _, err := rc.addEvidence(meta); err != nil {
		t.Fatalf("first addEvidence returned error: %v", err)
	}
	conflicting := meta
	conflicting.Title = "different title"
	if _, err := rc.addEvidence(conflicting); err == nil || !strings.Contains(err.Error(), "metadata differs") {
		t.Fatalf("conflicting addEvidence error = %v, want metadata conflict", err)
	}
}

func TestBeginEvidenceUploadRejectsNonIntegerSize(t *testing.T) {
	rc := &runContext{cfg: Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()}}
	_, err := rc.beginEvidenceUpload(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":               "Bad size",
		"mime_type":           "text/plain",
		"expected_size_bytes": "12",
		"source_description":  "test source",
		"relevance":           "test relevance",
	})
	if err == nil || !strings.Contains(err.Error(), "expected_size_bytes must be an integer") {
		t.Fatalf("beginEvidenceUpload error = %v, want integer error", err)
	}
}

func TestPrepareSubmittedEvidenceHonorsDirectByteLimit(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxDirectSubmittedEvidenceBytes = 4
	policy.MaxSubmittedEvidenceBytes = 8
	rc := &runContext{cfg: Config{OutputDir: t.TempDir(), Policy: policy}}
	_, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":              "Too large direct source",
		"source_description": "test source",
		"mime_type":          "text/plain",
		"relevance":          "test relevance",
		"content":            "12345",
	})
	if err == nil || !strings.Contains(err.Error(), "direct submitted evidence exceeds byte limit") {
		t.Fatalf("prepareSubmittedEvidence error = %v, want direct limit error", err)
	}
}

func TestValidatePolicyKeepsUploadLimitWithinRecordEvidenceLimit(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxSubmittedEvidenceBytes = 8
	policy.MaxDirectSubmittedEvidenceBytes = 4
	policy.MaxEvidenceUploadBytes = 9
	policy.MaxEvidenceChunkBytes = 4
	if err := ValidatePolicy(policy); err == nil || !strings.Contains(err.Error(), "max_evidence_upload_bytes") {
		t.Fatalf("ValidatePolicy error = %v, want upload limit error", err)
	}
}

func TestLoadCaseFilesDefersTextReadUntilSnapshot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "situation.md"), []byte("# Question\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write situation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "complaint.md"), []byte("# Question\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	body := "line one\nline two\n"
	if err := os.WriteFile(filepath.Join(dir, "confession.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write confession: %v", err)
	}

	files, err := loadCaseFiles(dir, filepath.Join(dir, "complaint.md"))
	if err != nil {
		t.Fatalf("loadCaseFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("loadCaseFiles returned %d files, want 1", len(files))
	}
	if files[0].Text != "" {
		t.Fatalf("file text = %q, want deferred read", files[0].Text)
	}
}

func TestLoadCaseFilesAllowsNoUsableFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "situation.md"), []byte("# Question\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write situation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "complaint.md"), []byte("# Question\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("note\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md~"), []byte("backup\n"), 0o644); err != nil {
		t.Fatalf("write readme backup: %v", err)
	}

	files, err := loadCaseFiles(dir, filepath.Join(dir, "complaint.md"))
	if err != nil {
		t.Fatalf("loadCaseFiles returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("loadCaseFiles returned %d files, want 0", len(files))
	}
}

func TestLoadCaseFilesFromPaths(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "instructions.txt")
	pemPath := filepath.Join(dir, "samantha_public.pem")
	if err := os.WriteFile(txtPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}
	if err := os.WriteFile(pemPath, []byte("pem"), 0o644); err != nil {
		t.Fatalf("write pem: %v", err)
	}

	files, err := loadCaseFilesFromPaths([]string{pemPath, txtPath})
	if err != nil {
		t.Fatalf("loadCaseFilesFromPaths returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("loadCaseFilesFromPaths returned %d files, want 2", len(files))
	}
	if files[0].EvidenceID != "instructions.txt" || files[1].EvidenceID != "samantha_public.pem" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if files[0].Text != "" {
		t.Fatalf("instructions text = %q, want deferred read", files[0].Text)
	}
}

func TestLoadCaseFilesFromPathsRejectsDuplicateBaseNames(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "a")
	right := filepath.Join(dir, "b")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("mkdir left: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("mkdir right: %v", err)
	}
	leftPath := filepath.Join(left, "shared.txt")
	rightPath := filepath.Join(right, "shared.txt")
	if err := os.WriteFile(leftPath, []byte("left"), 0o644); err != nil {
		t.Fatalf("write left: %v", err)
	}
	if err := os.WriteFile(rightPath, []byte("right"), 0o644); err != nil {
		t.Fatalf("write right: %v", err)
	}

	_, err := loadCaseFilesFromPaths([]string{leftPath, rightPath})
	if err == nil || !strings.Contains(err.Error(), "duplicate case file name") {
		t.Fatalf("loadCaseFilesFromPaths error = %v, want duplicate name error", err)
	}
}

func TestValidateAttorneyPayload(t *testing.T) {
	policy := DefaultPolicy()
	evidenceByID := map[string]EvidenceMeta{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128, RecordVisibility: "council_visible", AdmissibilityStatus: "case_packet"},
	}
	valid := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt", "label": "PX-1"},
		},
		"technical_reports": []any{
			map[string]any{"title": "Verification", "summary": "Verified OK."},
		},
	}
	if err := validateAttorneyPayload("submit_argument", valid, evidenceByID, policy); err != nil {
		t.Fatalf("validateAttorneyPayload returned error: %v", err)
	}
	invalid := map[string]any{
		"text": "",
	}
	if err := validateAttorneyPayload("submit_argument", invalid, evidenceByID, policy); err == nil {
		t.Fatalf("expected validation error for empty text")
	}
	badFile := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "missing.txt"},
		},
	}
	if err := validateAttorneyPayload("submit_argument", badFile, evidenceByID, policy); err == nil {
		t.Fatalf("expected validation error for missing file")
	}
}

func TestCouncilMemberIDFromOpportunity(t *testing.T) {
	opportunity := Opportunity{ID: "deliberation:2:C4", MemberID: "C4"}
	if got := councilMemberIDFromOpportunity(opportunity); got != "C4" {
		t.Fatalf("councilMemberIDFromOpportunity = %q, want C4", got)
	}
}

func TestPreflightCouncilCandidatesReplacesUnavailableSeat(t *testing.T) {
	candidates := []CouncilSeat{
		{Model: "bad-model", PersonaFile: "bad.md", PersonaText: "bad"},
		{Model: "good-a", PersonaFile: "good-a.md", PersonaText: "good a"},
		{Model: "good-b", PersonaFile: "good-b.md", PersonaText: "good b"},
	}
	checked := []string{}
	seated, replacements, err := preflightCouncilCandidates(context.Background(), candidates, 3, func(_ context.Context, seat CouncilSeat) error {
		checked = append(checked, seat.MemberID+":"+seat.Model)
		if seat.Model == "bad-model" {
			return fmt.Errorf("404 model unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("preflightCouncilCandidates returned error: %v", err)
	}
	checkedModels := make(map[string]int)
	for _, value := range checked {
		_, model, _ := strings.Cut(value, ":")
		checkedModels[model]++
	}
	for _, model := range []string{"bad-model", "good-a", "good-b"} {
		if checkedModels[model] != 1 {
			t.Fatalf("checked = %#v", checked)
		}
	}
	if len(seated) != 3 {
		t.Fatalf("seated %d council members, want 3", len(seated))
	}
	for index, seat := range seated {
		if seat.MemberID != fmt.Sprintf("C%d", index+1) || seat.Model == "bad-model" {
			t.Fatalf("seated council = %#v", seated)
		}
	}
	if len(replacements) != 1 {
		t.Fatalf("replacements = %#v, want one replacement", replacements)
	}
	replacement := replacements[0]
	if replacement.UnavailableModel != "bad-model" || replacement.ReplacementModel == "bad-model" || !strings.Contains(replacement.Cause, "404") {
		t.Fatalf("replacement = %#v", replacement)
	}
}

func TestPreflightCouncilCandidatesFailsWhenAvailablePoolExhausted(t *testing.T) {
	candidates := []CouncilSeat{
		{Model: "bad-a", PersonaFile: "bad-a.md"},
		{Model: "bad-b", PersonaFile: "bad-b.md"},
	}
	_, _, err := preflightCouncilCandidates(context.Background(), candidates, 1, func(_ context.Context, seat CouncilSeat) error {
		return fmt.Errorf("%s unavailable", seat.Model)
	})
	if err == nil || !strings.Contains(err.Error(), "could not seat C1") {
		t.Fatalf("preflightCouncilCandidates error = %v, want seating failure", err)
	}
}

func TestValidateAttorneyPayloadAllowsSupplementalMaterialsInRebuttal(t *testing.T) {
	policy := DefaultPolicy()
	evidenceByID := map[string]EvidenceMeta{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128, RecordVisibility: "council_visible", AdmissibilityStatus: "case_packet"},
	}
	rebuttal := map[string]any{
		"text": "reply",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt", "label": "PX-1"},
		},
		"technical_reports": []any{
			map[string]any{"title": "Check", "summary": "Done."},
		},
	}
	if err := validateAttorneyPayload("submit_rebuttal", rebuttal, evidenceByID, policy); err != nil {
		t.Fatalf("expected rebuttal supplemental materials to be accepted: %v", err)
	}
}

func TestValidateAttorneyPayloadAllowsSupplementalMaterialsInSurrebuttal(t *testing.T) {
	policy := DefaultPolicy()
	evidenceByID := map[string]EvidenceMeta{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128, RecordVisibility: "council_visible", AdmissibilityStatus: "case_packet"},
	}
	surrebuttal := map[string]any{
		"text": "reply",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt", "label": "PX-1"},
		},
		"technical_reports": []any{
			map[string]any{"title": "Check", "summary": "Done."},
		},
	}
	if err := validateAttorneyPayload("submit_surrebuttal", surrebuttal, evidenceByID, policy); err != nil {
		t.Fatalf("expected surrebuttal supplemental materials to be accepted: %v", err)
	}
}

func TestValidateAttorneyPayloadRejectsSupplementalMaterialsInClosing(t *testing.T) {
	policy := DefaultPolicy()
	evidenceByID := map[string]EvidenceMeta{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128, RecordVisibility: "council_visible", AdmissibilityStatus: "case_packet"},
	}
	closing := map[string]any{
		"text": "closing",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt"},
		},
	}
	if err := validateAttorneyPayload("deliver_closing_statement", closing, evidenceByID, policy); err == nil {
		t.Fatalf("expected closing offered_evidence to be rejected")
	}
	closing = map[string]any{
		"text": "closing",
		"technical_reports": []any{
			map[string]any{"title": "Late report", "summary": "New analysis."},
		},
	}
	if err := validateAttorneyPayload("deliver_closing_statement", closing, evidenceByID, policy); err == nil {
		t.Fatalf("expected closing technical_reports to be rejected")
	}
}

func TestValidateAttorneyPayloadRejectsOversizeExhibit(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxExhibitBytes = 16
	evidenceByID := map[string]EvidenceMeta{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 32, RecordVisibility: "council_visible", AdmissibilityStatus: "case_packet"},
	}
	payload := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt"},
		},
	}
	if err := validateAttorneyPayload("submit_argument", payload, evidenceByID, policy); err == nil {
		t.Fatalf("expected oversize exhibit to be rejected")
	}
}

func TestValidateAttorneyPayloadRejectsTooManyReports(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxReportsPerFiling = 1
	evidenceByID := map[string]EvidenceMeta{}
	payload := map[string]any{
		"text": "argument",
		"technical_reports": []any{
			map[string]any{"title": "One", "summary": "A"},
			map[string]any{"title": "Two", "summary": "B"},
		},
	}
	if err := validateAttorneyPayload("submit_argument", payload, evidenceByID, policy); err == nil {
		t.Fatalf("expected per-filing report limit to be enforced")
	}
}

func TestFormatInvalidAttemptLimitErrorIncludesAttemptReasons(t *testing.T) {
	err := formatInvalidAttemptLimitError("plaintiff", []string{
		"opening statement exceeds character limit of 4000 (got 4687)",
		"payload.text is required",
	})
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "plaintiff exceeded invalid-attempt limit after 2 invalid submissions") {
		t.Fatalf("unexpected invalid-attempt summary: %s", got)
	}
	if !strings.Contains(got, "attempt 1: opening statement exceeds character limit of 4000 (got 4687)") {
		t.Fatalf("missing first attempt reason: %s", got)
	}
	if !strings.Contains(got, "attempt 2: payload.text is required") {
		t.Fatalf("missing second attempt reason: %s", got)
	}
}

func TestFormatInvalidAttemptLimitErrorFallsBackWithoutReasons(t *testing.T) {
	err := formatInvalidAttemptLimitError("plaintiff", []string{"", "  "})
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	if got := err.Error(); got != "plaintiff exceeded invalid-attempt limit" {
		t.Fatalf("unexpected fallback invalid-attempt error: %s", got)
	}
}

func TestFormatAttorneyInvalidDecisionErrorGuidesLengthResubmission(t *testing.T) {
	err := formatAttorneyInvalidDecisionError(
		Opportunity{Role: "plaintiff", Phase: "openings"},
		DefaultPolicy(),
		[]string{"opening statement exceeds character limit of 4000 (got 4687)"},
		3,
	)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "Opening statement exceeds the character limit: 4687 characters submitted, 4000 allowed.") {
		t.Fatalf("missing length reason: %s", got)
	}
	if !strings.Contains(got, "This is invalid submission 1 of 3 for this opportunity. You have 2 invalid submissions remaining.") {
		t.Fatalf("missing invalid-submission count: %s", got)
	}
	if !strings.Contains(got, "Resubmit at 3000 characters or fewer. Count characters, not tokens.") {
		t.Fatalf("missing resubmission target: %s", got)
	}
	if !strings.Contains(got, "If you exhaust the remaining invalid submissions, this opportunity will fail and the run will end with an error.") {
		t.Fatalf("missing exhaustion warning: %s", got)
	}
}

func TestFormatAttorneyInvalidDecisionErrorGuidesOverflowResubmission(t *testing.T) {
	err := formatAttorneyInvalidDecisionError(
		Opportunity{Role: "plaintiff", Phase: "rebuttals"},
		DefaultPolicy(),
		[]string{"technical_reports for this side exceed limit of 4 (3 already used, 2 attempted, 1 remaining)"},
		3,
	)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "technical_reports for this side exceed limit of 4 (3 already used, 2 attempted, 1 remaining).") {
		t.Fatalf("missing overflow reason: %s", got)
	}
	if !strings.Contains(got, "Remove the overflow and resubmit within the stated limit.") {
		t.Fatalf("missing overflow guidance: %s", got)
	}
}

func TestFormatAttorneyInvalidDecisionErrorExplainsFinalFailure(t *testing.T) {
	err := formatAttorneyInvalidDecisionError(
		Opportunity{Role: "plaintiff", Phase: "openings"},
		DefaultPolicy(),
		[]string{
			"opening statement exceeds character limit of 4000 (got 4687)",
			"payload.text is required",
			"payload.text is required",
		},
		3,
	)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "This is invalid submission 3 of 3 for this opportunity. No invalid submissions remain.") {
		t.Fatalf("missing final invalid-submission count: %s", got)
	}
	if !strings.Contains(got, "This opportunity has failed, and the run is ending with an error.") {
		t.Fatalf("missing terminal failure line: %s", got)
	}
	if !strings.Contains(got, "Invalid submission history: attempt 1: Opening statement exceeds the character limit: 4687 characters submitted, 4000 allowed.; attempt 2: payload.text is required.; attempt 3: payload.text is required.") {
		t.Fatalf("missing invalid-submission history: %s", got)
	}
}

func TestValidateAttorneyPayloadAgainstStateRejectsOverlongRebuttal(t *testing.T) {
	policy := DefaultPolicy()
	rc := &runContext{
		cfg: Config{Policy: policy},
		state: map[string]any{
			"case": map[string]any{
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	payload := map[string]any{
		"text": strings.Repeat("a", policy.MaxRebuttalChars+1),
	}
	err := rc.validateAttorneyPayloadAgainstState(Opportunity{
		Role:  "plaintiff",
		Phase: "rebuttals",
	}, "submit_rebuttal", payload)
	if err == nil {
		t.Fatalf("expected rebuttal length error")
	}
	if !strings.Contains(err.Error(), "rebuttal exceeds character limit") || !strings.Contains(err.Error(), fmt.Sprintf("got %d", policy.MaxRebuttalChars+1)) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttorneyPayloadAgainstStateRejectsSideReportOverflow(t *testing.T) {
	policy := DefaultPolicy()
	existing := []map[string]any{
		{"role": "plaintiff", "title": "One", "summary": "A"},
		{"role": "plaintiff", "title": "Two", "summary": "B"},
		{"role": "plaintiff", "title": "Three", "summary": "C"},
	}
	rc := &runContext{
		cfg: Config{Policy: policy},
		state: map[string]any{
			"case": map[string]any{
				"offered_evidence":  []map[string]any{},
				"technical_reports": existing,
			},
		},
	}
	payload := map[string]any{
		"text": "reply",
		"technical_reports": []any{
			map[string]any{"title": "Four", "summary": "D"},
			map[string]any{"title": "Five", "summary": "E"},
		},
	}
	err := rc.validateAttorneyPayloadAgainstState(Opportunity{
		Role:  "plaintiff",
		Phase: "rebuttals",
	}, "submit_rebuttal", payload)
	if err == nil {
		t.Fatalf("expected side report overflow error")
	}
	if !strings.Contains(err.Error(), "technical_reports for this side exceed limit of 4 (3 already used, 2 attempted, 1 remaining)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePolicyRejectsMissingJudgmentStandard(t *testing.T) {
	policy := DefaultPolicy()
	policy.JudgmentStandard = " "
	err := ValidatePolicy(policy)
	if err == nil {
		t.Fatalf("expected policy validation error")
	}
	if got := err.Error(); got != "policy.judgment_standard is required" {
		t.Fatalf("unexpected validation error: %s", got)
	}
}

func TestValidateRuntimeLimitsRejectsZeroResponseLimit(t *testing.T) {
	runtime := DefaultRuntimeLimits()
	runtime.MaxResponseBytes = 0
	if err := ValidateRuntimeLimits(runtime); err == nil {
		t.Fatalf("expected runtime validation error")
	}
}

func TestBuildAttorneyPromptStatesCouncilForum(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "openings",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "openings:plaintiff",
		Role:         "plaintiff",
		Phase:        "openings",
		Objective:    "plaintiff opening statement",
		AllowedTools: []string{"record_opening_statement"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "The participants are the two lawyers and the council.") {
		t.Fatalf("prompt did not state the forum shape:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Address the council, not a judge.") {
		t.Fatalf("prompt did not direct counsel to address the council:\n%s", prompt)
	}
	if !strings.Contains(prompt, "The record may include case-packet files") {
		t.Fatalf("prompt did not state that openings may inspect case-packet evidence:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Analyze the case-packet evidence available at the opening.") {
		t.Fatalf("prompt did not instruct counsel to analyze opening evidence:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do not submit evidence, offer evidence, or file technical reports in this phase.") {
		t.Fatalf("prompt did not state the opening filing limit:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do not invent facts, sources, quotations, files, analyses, or results.") {
		t.Fatalf("prompt did not forbid fabrication:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Read any returned error and address the stated defect before another attempt.") {
		t.Fatalf("prompt did not instruct counsel to respond to tool errors:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Text limit for this submission: 5000 characters.") {
		t.Fatalf("prompt did not state the opening text limit:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Target length for the first submission: 3750 characters or less.") {
		t.Fatalf("prompt did not state the opening target length:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use the Lawyer API as role plaintiff.") {
		t.Fatalf("prompt did not state the Lawyer API role:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Opportunity id: openings:plaintiff") {
		t.Fatalf("prompt did not state the current opportunity id:\n%s", prompt)
	}
	if strings.Contains(prompt, "Visible case files:") {
		t.Fatalf("opening prompt should not list visible case files:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptIncludesWorkGuidanceEveryTurn(t *testing.T) {

	opportunities := []Opportunity{
		{
			ID:           "openings:plaintiff",
			Role:         "plaintiff",
			Phase:        "openings",
			Objective:    "plaintiff opening statement",
			AllowedTools: []string{"record_opening_statement"},
		},
		{
			ID:           "arguments:plaintiff",
			Role:         "plaintiff",
			Phase:        "arguments",
			Objective:    "plaintiff merits argument",
			AllowedTools: []string{"submit_argument"},
		},
		{
			ID:           "rebuttals:plaintiff",
			Role:         "plaintiff",
			Phase:        "rebuttals",
			Objective:    "plaintiff rebuttal",
			AllowedTools: []string{"submit_rebuttal", "pass_phase_opportunity"},
		},
		{
			ID:           "surrebuttals:defendant",
			Role:         "defendant",
			Phase:        "surrebuttals",
			Objective:    "defendant surrebuttal",
			AllowedTools: []string{"submit_surrebuttal", "pass_phase_opportunity"},
		},
		{
			ID:           "closings:plaintiff",
			Role:         "plaintiff",
			Phase:        "closings",
			Objective:    "plaintiff closing statement",
			AllowedTools: []string{"deliver_closing_statement"},
		},
	}
	required := []string{
		"The plaintiff argues for the strongest supported higher score.",
		"The defendant tests that position and argues for the strongest supported lower score or narrower range.",
		"After initial orientation, send a short note through send_work_notes",
		"Send further notes as material findings, failed approaches, or changes in theory occur",
		"At the start of each opportunity, inspect the current record and evidence list",
		"Check adverse evidence, conflicting sources, missing context, and later corrections.",
		"Use the analysis, execution, and computer-use tools supplied by your harness",
		"Install tools when needed",
		"extracting PDF text, inspecting images, transcribing media, checking signatures, or writing and running programs",
		"Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter.",
	}

	for _, opportunity := range opportunities {
		t.Run(opportunity.ID, func(t *testing.T) {
			rc := &runContext{
				cfg: Config{
					PromptDir: testPromptDir(),
					Policy:    DefaultPolicy(),
				},
				complaint: spec.Complaint{
					Question: "P",
				},
				caseFiles: []CaseFile{{EvidenceID: "case-file.txt", Name: "case-file.txt", MimeType: "text/plain", TextReadable: true}},
				state: map[string]any{
					"policy": map[string]any{
						"judgment_standard": "preponderance",
					},
					"case": map[string]any{
						"phase":              opportunity.Phase,
						"openings":           []map[string]any{},
						"arguments":          []map[string]any{},
						"rebuttals":          []map[string]any{},
						"surrebuttals":       []map[string]any{},
						"closings":           []map[string]any{},
						"offered_evidence":   []map[string]any{},
						"submitted_evidence": []map[string]any{},
						"technical_reports":  []map[string]any{},
					},
				},
			}
			prompt, err := rc.buildAttorneyPrompt(opportunity)
			if err != nil {
				t.Fatalf("buildAttorneyPrompt returned error: %v", err)
			}
			for _, want := range required {
				if !strings.Contains(prompt, want) {
					t.Fatalf("prompt missing %q:\n%s", want, prompt)
				}
			}
		})
	}
}

func TestLawyerToolSpecsArePhaseSpecific(t *testing.T) {
	openingSpecs := lawyerToolSpecs(Opportunity{Phase: "openings", AllowedTools: []string{"record_opening_statement"}})
	openingTools := make([]string, 0, len(openingSpecs))
	for _, spec := range openingSpecs {
		openingTools = append(openingTools, mapString(spec["name"]))
	}
	if slices.Contains(openingTools, "begin_evidence_upload") || slices.Contains(openingTools, "write_evidence_chunk") || slices.Contains(openingTools, "commit_evidence_upload") || slices.Contains(openingTools, "submit_evidence") {
		t.Fatalf("opening tools exposed evidence submission: %#v", openingTools)
	}
	if !slices.Contains(openingTools, "list_evidence") || !slices.Contains(openingTools, "stat_evidence") || !slices.Contains(openingTools, "read_evidence_range") {
		t.Fatalf("opening tools did not expose evidence reads: %#v", openingTools)
	}
	argumentSpecs := lawyerToolSpecs(Opportunity{Phase: "arguments", AllowedTools: []string{"submit_argument"}})
	argumentTools := make([]string, 0, len(argumentSpecs))
	for _, spec := range argumentSpecs {
		argumentTools = append(argumentTools, mapString(spec["name"]))
	}
	if !slices.Contains(argumentTools, "list_evidence") || !slices.Contains(argumentTools, "stat_evidence") || !slices.Contains(argumentTools, "read_evidence_range") || !slices.Contains(argumentTools, "begin_evidence_upload") || !slices.Contains(argumentTools, "write_evidence_chunk") || !slices.Contains(argumentTools, "commit_evidence_upload") || !slices.Contains(argumentTools, "submit_evidence") {
		t.Fatalf("argument tools did not expose evidence access: %#v", argumentTools)
	}
	rebuttalSpecs := lawyerToolSpecs(Opportunity{Phase: "rebuttals", AllowedTools: []string{"submit_rebuttal"}})
	rebuttalTools := make([]string, 0, len(rebuttalSpecs))
	for _, spec := range rebuttalSpecs {
		rebuttalTools = append(rebuttalTools, mapString(spec["name"]))
	}
	if !slices.Contains(rebuttalTools, "list_evidence") || !slices.Contains(rebuttalTools, "stat_evidence") || !slices.Contains(rebuttalTools, "read_evidence_range") || !slices.Contains(rebuttalTools, "begin_evidence_upload") || !slices.Contains(rebuttalTools, "write_evidence_chunk") || !slices.Contains(rebuttalTools, "commit_evidence_upload") || !slices.Contains(rebuttalTools, "submit_evidence") {
		t.Fatalf("rebuttal tools did not expose evidence access: %#v", rebuttalTools)
	}
	surrebuttalSpecs := lawyerToolSpecs(Opportunity{Phase: "surrebuttals", AllowedTools: []string{"submit_surrebuttal", "pass_phase_opportunity"}})
	surrebuttalTools := make([]string, 0, len(surrebuttalSpecs))
	for _, spec := range surrebuttalSpecs {
		surrebuttalTools = append(surrebuttalTools, mapString(spec["name"]))
	}
	if !slices.Contains(surrebuttalTools, "list_evidence") || !slices.Contains(surrebuttalTools, "stat_evidence") || !slices.Contains(surrebuttalTools, "read_evidence_range") || !slices.Contains(surrebuttalTools, "begin_evidence_upload") || !slices.Contains(surrebuttalTools, "write_evidence_chunk") || !slices.Contains(surrebuttalTools, "commit_evidence_upload") || !slices.Contains(surrebuttalTools, "submit_evidence") {
		t.Fatalf("surrebuttal tools did not expose evidence access: %#v", surrebuttalTools)
	}
	var submitSpec map[string]any
	for _, spec := range argumentSpecs {
		if mapString(spec["name"]) == "submit_decision" {
			submitSpec = spec
			break
		}
	}
	if submitSpec == nil {
		t.Fatalf("missing submit_decision spec")
	}
	properties := mapAny(mapAny(submitSpec["input_schema"])["properties"])
	if _, ok := properties["reason"]; ok {
		t.Fatalf("submit_decision should not advertise a reason field: %#v", properties)
	}
	payload := mapAny(properties["payload"])
	if mapString(payload["type"]) != "object" {
		t.Fatalf("payload schema type = %#v, want object", payload["type"])
	}
	payloadProps := mapAny(payload["properties"])
	offeredEvidence := mapAny(payloadProps["offered_evidence"])
	if mapString(offeredEvidence["type"]) != "array" {
		t.Fatalf("offered_evidence schema type = %#v, want array", offeredEvidence["type"])
	}
	offeredItemProps := mapAny(mapAny(offeredEvidence["items"])["properties"])
	if _, ok := offeredItemProps["evidence_id"]; !ok {
		t.Fatalf("offered_evidence items missing evidence_id: %#v", offeredItemProps)
	}
	if _, ok := offeredItemProps["label"]; !ok {
		t.Fatalf("offered_evidence items missing label: %#v", offeredItemProps)
	}
	reports := mapAny(payloadProps["technical_reports"])
	if mapString(reports["type"]) != "array" {
		t.Fatalf("technical_reports schema type = %#v, want array", reports["type"])
	}
	reportItemProps := mapAny(mapAny(reports["items"])["properties"])
	if _, ok := reportItemProps["title"]; !ok {
		t.Fatalf("technical_reports items missing title: %#v", reportItemProps)
	}
	if _, ok := reportItemProps["summary"]; !ok {
		t.Fatalf("technical_reports items missing summary: %#v", reportItemProps)
	}
	openingSubmitSpec := findHTTPToolSpec(openingSpecs, "submit_decision")
	openingEnum, _ := mapAny(mapAny(openingSubmitSpec["input_schema"])["properties"])["tool_name"].(map[string]any)["enum"].([]string)
	if len(openingEnum) != 1 || openingEnum[0] != "record_opening_statement" {
		t.Fatalf("opening submit_decision enum = %#v, want record_opening_statement only", openingEnum)
	}
	argumentSubmitSpec := findHTTPToolSpec(argumentSpecs, "submit_decision")
	argumentEnum, _ := mapAny(mapAny(argumentSubmitSpec["input_schema"])["properties"])["tool_name"].(map[string]any)["enum"].([]string)
	if len(argumentEnum) != 1 || argumentEnum[0] != "submit_argument" {
		t.Fatalf("argument submit_decision enum = %#v, want submit_argument only", argumentEnum)
	}
	argumentSpecsWithEvidenceAction := lawyerToolSpecs(Opportunity{Phase: "arguments", AllowedTools: []string{"submit_evidence", "submit_argument"}})
	if findHTTPToolSpec(argumentSpecsWithEvidenceAction, "submit_evidence") == nil {
		t.Fatalf("argument tools missing direct submit_evidence")
	}
	argumentSubmitSpec = findHTTPToolSpec(argumentSpecsWithEvidenceAction, "submit_decision")
	argumentEnum, _ = mapAny(mapAny(argumentSubmitSpec["input_schema"])["properties"])["tool_name"].(map[string]any)["enum"].([]string)
	if len(argumentEnum) != 1 || argumentEnum[0] != "submit_argument" {
		t.Fatalf("argument submit_decision enum with evidence action = %#v, want submit_argument only", argumentEnum)
	}
	evidenceOnlySpecs := lawyerToolSpecs(Opportunity{Phase: "arguments", AllowedTools: []string{"submit_evidence"}})
	evidenceOnlySubmitSpec := findHTTPToolSpec(evidenceOnlySpecs, "submit_decision")
	evidenceOnlyEnum, _ := mapAny(mapAny(evidenceOnlySubmitSpec["input_schema"])["properties"])["tool_name"].(map[string]any)["enum"].([]string)
	if len(evidenceOnlyEnum) != 0 {
		t.Fatalf("submit_decision enum for evidence-only action = %#v, want empty", evidenceOnlyEnum)
	}
}

func findHTTPToolSpec(specs []map[string]any, name string) map[string]any {
	for _, spec := range specs {
		if mapString(spec["name"]) == name {
			return spec
		}
	}
	return nil
}

func TestCouncilBackendValidation(t *testing.T) {
	for _, backend := range []string{"", "direct", "councilapi", "COUNCILAPI"} {
		if err := ValidateCouncilBackend(backend); err != nil {
			t.Fatalf("ValidateCouncilBackend(%q) returned error: %v", backend, err)
		}
	}
	if got := NormalizeCouncilBackend(""); got != "direct" {
		t.Fatalf("NormalizeCouncilBackend empty = %q, want direct", got)
	}
	if err := ValidateCouncilBackend("pi"); err == nil {
		t.Fatalf("ValidateCouncilBackend accepted removed pi backend")
	}
	if err := ValidateCouncilBackend("browser"); err == nil {
		t.Fatalf("ValidateCouncilBackend accepted unknown backend")
	}
}

func TestCouncilSeatRosterIncludesRequestSpec(t *testing.T) {
	pool := makeTempCouncilPool(t, `{"openrouter_model_id":"deepseek/deepseek-v4-flash","endpoint_tag":"deepinfra/fp4","quantization":"fp4","request":{"temperature":0,"top_p":1,"max_tokens":32},"persona":"personas/juror.txt"}`)
	council, err := sampleCouncil(pool, filepath.Dir(pool), 1)
	if err != nil {
		t.Fatalf("sampleCouncil error = %v", err)
	}
	roster, err := councilSeatRoster(council, []map[string]any{{"member_id": "C1", "status": "removed"}})
	if err != nil {
		t.Fatalf("councilSeatRoster error = %v", err)
	}
	if len(roster) != 1 {
		t.Fatalf("roster length = %d, want 1", len(roster))
	}
	member := roster[0]
	if member["status"] != "removed" || member["persona_filename"] != "personas/juror.txt" {
		t.Fatalf("member metadata = %#v", member)
	}
	requestSpec := mapAny(member["request_spec"])
	if requestSpec["endpoint"] != "openrouter" || requestSpec["model"] != "deepseek/deepseek-v4-flash" {
		t.Fatalf("request_spec model = %#v", requestSpec)
	}
	provider := mapAny(requestSpec["provider"])
	only := stringList(provider["only"])
	if len(only) != 1 || only[0] != "deepinfra/fp4" {
		t.Fatalf("provider.only = %#v", provider["only"])
	}
	quantizations := stringList(provider["quantizations"])
	if len(quantizations) != 1 || quantizations[0] != "fp4" {
		t.Fatalf("provider.quantizations = %#v", provider["quantizations"])
	}
	request := mapAny(requestSpec["request"])
	if request["temperature"] == nil || request["top_p"] == nil || request["max_tokens"] == nil {
		t.Fatalf("request = %#v", request)
	}
}

func TestFinalCouncilCarriesFailureStatusAndRequestSpec(t *testing.T) {
	pool := makeTempCouncilPool(t, `{"openrouter_model_id":"deepseek/deepseek-v4-flash","endpoint_tag":"deepinfra/fp4","quantization":"fp4","request":{"temperature":0,"top_p":1,"max_tokens":32},"persona":"personas/juror.txt"}`)
	council, err := sampleCouncil(pool, filepath.Dir(pool), 1)
	if err != nil {
		t.Fatalf("sampleCouncil error = %v", err)
	}
	state := map[string]any{
		"case": map[string]any{
			"council_members": []map[string]any{{
				"member_id":              "C1",
				"status":                 "failed",
				"failure_reason":         opportunityFailureAgentExited,
				"failure_opportunity_id": "deliberation:1:C1",
				"failure_message":        "agent exited",
			}},
		},
	}
	final := finalCouncil(council, state)
	if len(final) != 1 {
		t.Fatalf("final council length = %d, want 1", len(final))
	}
	seat := final[0]
	if seat.Status != "failed" || seat.FailureReason != opportunityFailureAgentExited || seat.FailureOpportunityID != "deliberation:1:C1" {
		t.Fatalf("final seat failure fields = %#v", seat)
	}
	if seat.RequestSpec == nil {
		t.Fatalf("final seat lost request spec")
	}
	if seat.Model != council[0].Model || seat.PersonaFile != council[0].PersonaFile {
		t.Fatalf("final seat lost sampled metadata: %#v", seat)
	}
}

func TestBuildAttorneyPromptConstrainsArgumentExperiments(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		caseFiles: []CaseFile{{EvidenceID: "instructions.txt", Name: "instructions.txt", MimeType: "text/plain", TextReadable: true}},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "arguments",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		Objective:    "plaintiff merits argument",
		AllowedTools: []string{"submit_evidence", "submit_argument"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Use this phase to file the merits submission for your side.") {
		t.Fatalf("argument prompt did not define the court-owned phase objective:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Exhibits: at most 9 in this filing. This side has used 0 of 12 total, with 12 left.") {
		t.Fatalf("argument prompt did not state exhibit limits:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Technical reports: at most 3 in this filing. This side has used 0 of 4 total, with 4 left.") {
		t.Fatalf("argument prompt did not state report limits:\n%s", prompt)
	}
	if !strings.Contains(prompt, "submit its content and provenance with the direct submit_evidence tool") {
		t.Fatalf("argument prompt did not require outside source material to enter as submitted evidence:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do not call submit_decision with tool_name set to submit_evidence") {
		t.Fatalf("argument prompt did not forbid wrapping submit_evidence in submit_decision:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Final filing actions for submit_decision: submit_argument") {
		t.Fatalf("argument prompt did not restrict submit_decision to final filing actions:\n%s", prompt)
	}
	if strings.Contains(prompt, "Legal operations allowed for this opportunity") {
		t.Fatalf("argument prompt used obsolete opportunity wording:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Evidence submission, offered evidence, and technical reports are available during arguments, rebuttals, and surrebuttals") {
		t.Fatalf("argument prompt did not state the phase rule for evidence submission:\n%s", prompt)
	}
	if strings.Contains(prompt, "Allowed legal acts for submit_decision") {
		t.Fatalf("argument prompt used obsolete submit_decision wording:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter.") {
		t.Fatalf("argument prompt did not instruct counsel to use evidence read tools:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use only visible case evidence_id values in offered_evidence. Submit new source material first with submit_evidence") {
		t.Fatalf("argument prompt did not restrict offered_evidence to visible evidence ids:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use technical_reports for attorney analysis or synthesized work product") {
		t.Fatalf("argument prompt did not distinguish technical reports from source evidence:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptConstrainsArgumentExperimentsWithoutSearch(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		caseFiles: []CaseFile{{EvidenceID: "instructions.txt", Name: "instructions.txt", MimeType: "text/plain", TextReadable: true}},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "arguments",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		Objective:    "plaintiff merits argument",
		AllowedTools: []string{"submit_argument"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Use the Lawyer API as role plaintiff.") {
		t.Fatalf("argument prompt did not state the Lawyer API role:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptAllowsRebuttalSupplementalMaterials(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":            "rebuttals",
				"openings":         []map[string]any{},
				"arguments":        []map[string]any{},
				"rebuttals":        []map[string]any{},
				"surrebuttals":     []map[string]any{},
				"closings":         []map[string]any{},
				"offered_evidence": []map[string]any{},
				"technical_reports": []map[string]any{
					{"role": "plaintiff", "title": "One", "summary": "A"},
					{"role": "plaintiff", "title": "Two", "summary": "B"},
					{"role": "plaintiff", "title": "Three", "summary": "C"},
				},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "rebuttals:plaintiff",
		Role:         "plaintiff",
		Phase:        "rebuttals",
		Objective:    "plaintiff rebuttal",
		AllowedTools: []string{"submit_rebuttal", "pass_phase_opportunity"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Offer exhibits, submitted evidence, and technical reports only if they directly answer the opposing argument.") {
		t.Fatalf("rebuttal prompt did not allow targeted supplemental materials:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Text limit for this submission: 4000 characters.") {
		t.Fatalf("rebuttal prompt did not state the rebuttal text limit:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Target length for the first submission: 3000 characters or less.") {
		t.Fatalf("rebuttal prompt did not state the rebuttal target length:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Technical reports: at most 3 in this filing. This side has used 3 of 4 total, with 1 left.") {
		t.Fatalf("rebuttal prompt did not state remaining report capacity:\n%s", prompt)
	}
	if !strings.Contains(prompt, "\"offered_evidence\"") || !strings.Contains(prompt, "\"technical_reports\"") {
		t.Fatalf("rebuttal example payload did not show supplemental materials:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter.") {
		t.Fatalf("rebuttal prompt did not instruct counsel to use evidence read tools:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use offered_evidence only for visible evidence, by evidence_id.") {
		t.Fatalf("rebuttal prompt did not restrict offered_evidence to visible evidence ids:\n%s", prompt)
	}
	if !strings.Contains(prompt, "submit its content and provenance with the direct submit_evidence tool") {
		t.Fatalf("rebuttal prompt did not require outside source material to enter as submitted evidence:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptAllowsSurrebuttalSupplementalMaterials(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":              "surrebuttals",
				"openings":           []map[string]any{},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"submitted_evidence": []map[string]any{},
				"technical_reports":  []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "surrebuttals:defendant",
		Role:         "defendant",
		Phase:        "surrebuttals",
		Objective:    "defendant surrebuttal",
		AllowedTools: []string{"submit_surrebuttal", "pass_phase_opportunity"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Final filing actions for submit_decision: submit_surrebuttal, pass_phase_opportunity") {
		t.Fatalf("surrebuttal prompt did not state final filing actions:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Evidence submission, offered evidence, and technical reports are available during arguments, rebuttals, and surrebuttals") {
		t.Fatalf("surrebuttal prompt did not state the phase rule for evidence submission:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Submitted evidence: admitted items may be at most") {
		t.Fatalf("surrebuttal prompt did not state submitted-evidence limits:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Submit any material source through the direct submit_evidence tool") {
		t.Fatalf("surrebuttal prompt did not tell counsel to submit source material:\n%s", prompt)
	}
	if strings.Contains(prompt, "Legal operations allowed for this opportunity") {
		t.Fatalf("surrebuttal prompt used obsolete opportunity wording:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptConstrainsRebuttalWithoutSearch(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "rebuttals",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "rebuttals:plaintiff",
		Role:         "plaintiff",
		Phase:        "rebuttals",
		Objective:    "plaintiff rebuttal",
		AllowedTools: []string{"submit_rebuttal", "pass_phase_opportunity"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Use the Lawyer API as role plaintiff.") {
		t.Fatalf("rebuttal prompt did not state the Lawyer API role:\n%s", prompt)
	}
}

func TestBuildCouncilPromptIncludesPersonaAndRecord(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Question: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"judgment_standard": "preponderance",
			},
			"case": map[string]any{
				"deliberation_round": 2,
				"openings":           []map[string]any{{"role": "plaintiff", "text": "opening"}},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"council_answers":    []map[string]any{{"round": 1, "member_id": "C1", "answer": 72, "rationale": "r"}},
			},
		},
	}
	prompt, err := rc.buildCouncilPrompt(CouncilSeat{
		MemberID:    "C2",
		PersonaText: "Skeptical but concise.",
	}, Opportunity{ID: "deliberation:2:C2", Role: "council", Phase: "deliberation"})
	if err != nil {
		t.Fatalf("buildCouncilPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Persona:\nSkeptical but concise.") {
		t.Fatalf("prompt did not include persona:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Prior rounds:\nRound 1 [C1] 72") {
		t.Fatalf("prompt did not include prior rounds:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Call submit_council_answer with answer as a JSON integer from 0 through 100") {
		t.Fatalf("prompt did not include council instruction:\n%s", prompt)
	}
}

func TestBuildCouncilPromptUsesPromptFileOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "council.md")
	if err := os.WriteFile(path, []byte("custom council prompt for {{MEMBER_ID}} on {{QUESTION}}\n{{RECORD}}\n"), 0o644); err != nil {
		t.Fatalf("write council prompt: %v", err)
	}
	rc := &runContext{
		cfg: Config{
			PromptFiles: map[string]string{promptCouncilSystem: path},
			Policy:      DefaultPolicy(),
		},
		complaint: spec.Complaint{Question: "Degree question?"},
		state: map[string]any{
			"case": map[string]any{
				"deliberation_round": 1,
				"openings":           []map[string]any{},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"council_answers":    []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildCouncilPrompt(CouncilSeat{MemberID: "C1"}, Opportunity{ID: "deliberation:1:C1", Role: "council", Phase: "deliberation"})
	if err != nil {
		t.Fatalf("buildCouncilPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "custom council prompt for C1 on Degree question?") {
		t.Fatalf("prompt did not use prompt-file override:\n%s", prompt)
	}
}

func TestIsFunctionArgumentParseError(t *testing.T) {
	t.Parallel()

	if isFunctionArgumentParseError(os.ErrInvalid) {
		t.Fatalf("unexpected parse-error match for os.ErrInvalid")
	}
	if !isFunctionArgumentParseError(fmt.Errorf("parse function arguments for submit_council_answer: unexpected end of JSON input")) {
		t.Fatalf("expected parse function arguments error to match")
	}
}

func TestIsCouncilTimeoutError(t *testing.T) {
	t.Parallel()

	if isCouncilTimeoutError(fmt.Errorf("provider failed")) {
		t.Fatalf("unexpected timeout match for generic error")
	}
	if !isCouncilTimeoutError(context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded to count as timeout")
	}
	if !isCouncilTimeoutError(fmt.Errorf("responses request canceled during backoff: %w", context.DeadlineExceeded)) {
		t.Fatalf("expected wrapped deadline exceeded to count as timeout")
	}
	if !isCouncilTimeoutError(fmt.Errorf("responses request failed: request timed out")) {
		t.Fatalf("expected timed out message to count as timeout")
	}
}

func TestIsCouncilRequestError(t *testing.T) {
	t.Parallel()

	if isCouncilRequestError(fmt.Errorf("parse function arguments for submit_council_answer: bad json")) {
		t.Fatalf("unexpected request-error match for tool argument parse error")
	}
	if isCouncilRequestError(context.Canceled) {
		t.Fatalf("unexpected request-error match for context cancellation")
	}
	if isCouncilRequestError(&openaiapi.ProviderError{Class: openaiapi.ProviderErrorTransient, Err: context.Canceled}) {
		t.Fatalf("unexpected request-error match for wrapped context cancellation")
	}
	providerErr := &openaiapi.ProviderError{
		Class: openaiapi.ProviderErrorRequest,
		Err:   fmt.Errorf("responses request failed: 404 model not found"),
	}
	if !isCouncilRequestError(providerErr) {
		t.Fatalf("expected typed provider failure to count as request error")
	}
}

func TestExecuteCouncilOpportunityRetriesAfterOversizeResponse(t *testing.T) {

	rc := newCouncilOpportunityTestContext(t, "")
	repairPath := filepath.Join(t.TempDir(), "response-too-large.md")
	if err := os.WriteFile(repairPath, []byte("custom repair {{SIZE_BYTES}}/{{LIMIT_BYTES}} using {{COUNCIL_TOOL}} with {{SUBMISSION_FIELDS}}"), 0o644); err != nil {
		t.Fatalf("write repair prompt: %v", err)
	}
	rc.cfg.PromptFiles = map[string]string{promptCouncilRepairOversize: repairPath}
	client := &fakeCouncilResponseClient{
		responses: []openaiapi.Response{
			{Text: strings.Repeat("x", 4096), ResponseID: "oversize"},
			{ToolCalls: []openaiapi.ToolCall{{Name: "submit_council_answer", Arguments: map[string]any{"answer": 72, "rationale": "record sufficient"}}}, ResponseID: "valid"},
		},
	}
	oversizeBytes, err := jsonPayloadSize(client.responses[0])
	if err != nil {
		t.Fatalf("measure oversize response: %v", err)
	}
	if err := rc.executeCouncilOpportunity(context.Background(), client, Opportunity{ID: "deliberation:1:C1", StateVersion: 1, Role: "council", Phase: "deliberation", MemberID: "C1"}); err != nil {
		t.Fatalf("executeCouncilOpportunity returned error: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("client calls = %d, want 2", client.calls)
	}
	if got, want := client.inputs[1][len(client.inputs[1])-1]["content"].(string), fmt.Sprintf("custom repair %d/2048 using submit_council_answer with answer and rationale", oversizeBytes); got != want {
		t.Fatalf("second prompt = %q", got)
	}
	caseObj := mapAny(rc.state["case"])
	answers := mapList(caseObj["council_answers"])
	if len(answers) != 1 || intNumber(answers[0]["answer"]) != 72 {
		t.Fatalf("answers = %#v, want one answer of 72", answers)
	}
}

func TestExecuteCouncilOpportunityFailsMemberAfterRepeatedOversizeResponses(t *testing.T) {

	rc := newCouncilOpportunityTestContext(t, opportunityFailureAttemptsExhausted)
	rc.cfg.Runtime.InvalidAttemptLimit = 2
	client := &fakeCouncilResponseClient{
		responses: []openaiapi.Response{
			{Text: strings.Repeat("x", 4096), ResponseID: "oversize-1"},
			{Text: strings.Repeat("y", 4096), ResponseID: "oversize-2"},
		},
	}
	if err := rc.executeCouncilOpportunity(context.Background(), client, Opportunity{ID: "deliberation:1:C1", StateVersion: 1, Role: "council", Phase: "deliberation", MemberID: "C1"}); err != nil {
		t.Fatalf("executeCouncilOpportunity returned error: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("client calls = %d, want 2", client.calls)
	}
	assertFailedCouncilMember(t, rc, opportunityFailureAttemptsExhausted)
	if got := mapString(rc.events[1].Payload["cause"]); !strings.Contains(got, "exceeded invalid-attempt limit") || !strings.Contains(got, "byte limit") {
		t.Fatalf("cause = %q, want invalid-attempt byte-limit cause", got)
	}
}

func TestRemoveTimedOutCouncilMemberRecordsEvent(t *testing.T) {
	t.Parallel()

	rc := newCouncilRemovalTestContext(t, opportunityFailureDeadline)
	opportunity := Opportunity{ID: "deliberation:1:C1", StateVersion: 1, Role: "council", Phase: "deliberation", MemberID: "C1"}
	seat := CouncilSeat{MemberID: "C1", Model: "openrouter://openai/gpt-4o"}
	if err := rc.removeTimedOutCouncilMember(context.Background(), time.Time{}, opportunity, seat, context.DeadlineExceeded); err != nil {
		t.Fatalf("removeTimedOutCouncilMember returned error: %v", err)
	}
	assertFailedCouncilMember(t, rc, opportunityFailureDeadline)
}

func TestRemoveRequestFailedCouncilMemberRecordsEvent(t *testing.T) {
	t.Parallel()

	rc := newCouncilRemovalTestContext(t, opportunityFailureRequestFailed)
	opportunity := Opportunity{ID: "deliberation:1:C1", StateVersion: 1, Role: "council", Phase: "deliberation", MemberID: "C1"}
	seat := CouncilSeat{MemberID: "C1", Model: "openrouter://anthropic/claude-3.7-sonnet"}
	if err := rc.removeRequestFailedCouncilMember(context.Background(), time.Time{}, opportunity, seat, fmt.Errorf("responses request failed: 404 model not found")); err != nil {
		t.Fatalf("removeRequestFailedCouncilMember returned error: %v", err)
	}
	assertFailedCouncilMember(t, rc, opportunityFailureRequestFailed)
	if got := mapString(rc.events[1].Payload["cause"]); !strings.Contains(got, "404") {
		t.Fatalf("cause = %q, want 404 marker", got)
	}
}

type fakeCouncilResponseClient struct {
	responses []openaiapi.Response
	errs      []error
	inputs    [][]map[string]any
	calls     int
}

func (c *fakeCouncilResponseClient) CreateResponseWithRequestSpec(_ context.Context, _ modelrequest.Spec, inputItems []map[string]any, _ []map[string]any, _ string) (openaiapi.Response, error) {
	c.inputs = append(c.inputs, append([]map[string]any(nil), inputItems...))
	call := c.calls
	c.calls++
	if call < len(c.errs) && c.errs[call] != nil {
		return openaiapi.Response{}, c.errs[call]
	}
	if call < len(c.responses) {
		return c.responses[call], nil
	}
	return openaiapi.Response{}, fmt.Errorf("unexpected fake council client call %d", call+1)
}

func newCouncilOpportunityTestContext(t *testing.T, failureReason string) *runContext {
	t.Helper()

	dir := t.TempDir()
	script := councilEngineScript(failureReason)
	runtimeLimits := DefaultRuntimeLimits()
	runtimeLimits.MaxResponseBytes = 2048
	runtimeLimits.InvalidAttemptLimit = 3
	return &runContext{
		cfg: Config{
			Engine:    lean.Engine{Command: []string{"/bin/sh", "-c", script}},
			OutputDir: dir,
			PromptDir: testPromptDir(),
			Policy:    DefaultPolicy(),
			Runtime:   runtimeLimits,
		},
		complaint: spec.Complaint{Question: "P"},
		state: map[string]any{
			"policy":        DefaultPolicy().StateMap(),
			"state_version": 1,
			"case": map[string]any{
				"phase":              "deliberation",
				"deliberation_round": 1,
				"openings":           []map[string]any{},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"submitted_evidence": []map[string]any{},
				"council_answers":    []map[string]any{},
				"council_members":    []map[string]any{{"member_id": "C1", "status": "seated"}},
				"answers":            "",
			},
		},
		council: []CouncilSeat{councilTestSeat("C1")},
	}
}

func councilTestSeat(memberID string) CouncilSeat {
	spec := modelrequest.Spec{
		Endpoint: "openai",
		Model:    "gpt-4o",
		Persona:  "personas/test.txt",
	}
	return CouncilSeat{
		MemberID:    memberID,
		Model:       spec.RuntimeModel(),
		PersonaFile: spec.Persona,
		RequestSpec: &spec,
		PersonaText: "Concise.",
	}
}

func newCouncilRemovalTestContext(t *testing.T, failureReason string) *runContext {
	t.Helper()

	dir := t.TempDir()
	script := councilEngineScript(failureReason)
	return &runContext{
		cfg: Config{
			Engine:    lean.Engine{Command: []string{"/bin/sh", "-c", script}},
			OutputDir: dir,
			Runtime:   DefaultRuntimeLimits(),
		},
		state: map[string]any{
			"state_version": 1,
			"case": map[string]any{
				"phase": "deliberation",
			},
		},
	}
}

func councilEngineScript(failureReason string) string {
	failureState := fmt.Sprintf(`{"ok":true,"state":{"case":{"phase":"deliberation","answers":"","council_members":[{"member_id":"C1","status":"failed","failure_reason":"%s","failure_opportunity_id":"deliberation:1:C1","failure_message":"member failed"}]},"state_version":2}}`, failureReason)
	answerState := `{"ok":true,"state":{"case":{"phase":"deliberation","answers":"","council_members":[{"member_id":"C1","status":"seated"}],"council_answers":[{"round":1,"member_id":"C1","answer":72,"rationale":"record sufficient"}]},"state_version":2}}`
	return fmt.Sprintf(`#!/bin/sh
request=$(cat)
case "$request" in
  *fail_opportunity*) printf '%%s\n' '%s' ;;
  *) printf '%%s\n' '%s' ;;
esac
`, failureState, answerState)
}

func blockingCouncilEngineScript(failureReason string) string {
	failureState := fmt.Sprintf(`{"ok":true,"state":{"case":{"phase":"deliberation","council_members":[{"member_id":"C1","status":"failed","failure_reason":"%s","failure_opportunity_id":"deliberation:1:C1","failure_message":"member failed"}]},"state_version":2}}`, failureReason)
	return fmt.Sprintf(`#!/bin/sh
request=$(cat)
case "$request" in
  *'"reason":"deadline_expired"'*) printf '%%s\n' '%s' ;;
  *)
    if [ -n "$1" ]; then
      printf 'entered\n' > "$1"
    fi
    exec sleep 60
    ;;
esac
`, failureState)
}

func assertFailedCouncilMember(t *testing.T, rc *runContext, reason string) {
	t.Helper()

	caseObj := mapAny(rc.state["case"])
	members := mapList(caseObj["council_members"])
	if len(members) != 1 {
		t.Fatalf("council member count = %d, want 1", len(members))
	}
	if got := mapString(members[0]["status"]); got != "failed" {
		t.Fatalf("member status = %q, want failed", got)
	}
	if got := mapString(members[0]["failure_reason"]); got != reason {
		t.Fatalf("failure_reason = %q, want %s", got, reason)
	}
	if len(rc.events) != 2 {
		t.Fatalf("event count = %d, want 2", len(rc.events))
	}
	event := rc.events[0]
	if event.Type != "opportunity_failed" {
		t.Fatalf("first event type = %q, want opportunity_failed", event.Type)
	}
	event = rc.events[1]
	if event.Type != "council_member_removed" {
		t.Fatalf("event type = %q, want council_member_removed", event.Type)
	}
	if got := mapString(event.Payload["member_id"]); got != "C1" {
		t.Fatalf("member_id = %q, want C1", got)
	}
	if got := mapString(event.Payload["status"]); got != "failed" {
		t.Fatalf("status = %q, want failed", got)
	}
	if got := mapString(event.Payload["failure_reason"]); got != reason {
		t.Fatalf("failure_reason = %q, want %s", got, reason)
	}
}
