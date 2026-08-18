package runner

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jsmorph/adj/adc/runtime/lean"
	"github.com/jsmorph/adj/adc/runtime/spec"
)

func TestReadCaseFileVerifiesComplaintAttachment(t *testing.T) {
	t.Parallel()

	r, fileObj, path := testRunnerWithAttachment(t, []byte("record text\n"))
	raw, err := r.readCaseFile(fileObj)
	if err != nil {
		t.Fatalf("readCaseFile() error = %v", err)
	}
	if string(raw) != "record text\n" {
		t.Fatalf("readCaseFile() = %q", raw)
	}

	if err := os.WriteFile(path, []byte("record next\n"), 0o644); err != nil {
		t.Fatalf("replace attachment bytes: %v", err)
	}
	_, err = r.readCaseFile(fileObj)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("readCaseFile() changed-byte error = %v", err)
	}
}

func TestReadCaseFileRejectsComplaintAttachmentSizeDrift(t *testing.T) {
	t.Parallel()

	r, fileObj, path := testRunnerWithAttachment(t, []byte("record text\n"))
	if err := os.WriteFile(path, []byte("record text with additional bytes\n"), 0o644); err != nil {
		t.Fatalf("replace attachment bytes: %v", err)
	}
	_, err := r.readCaseFile(fileObj)
	if err == nil || !strings.Contains(err.Error(), "recorded size") {
		t.Fatalf("readCaseFile() size error = %v", err)
	}
}

func TestReadCaseFileRejectsComplaintAttachmentSymlinkReplacement(t *testing.T) {
	t.Parallel()

	r, fileObj, path := testRunnerWithAttachment(t, []byte("record text\n"))
	target := filepath.Join(filepath.Dir(path), "replacement.txt")
	if err := os.WriteFile(target, []byte("record text\n"), 0o644); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove attachment: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("replace attachment with symlink: %v", err)
	}
	_, err := r.readCaseFile(fileObj)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("readCaseFile() symlink error = %v", err)
	}
}

func TestComplaintAttachmentReadsReachPromptAndRoleAPI(t *testing.T) {
	t.Parallel()

	contents := []byte("record text\n")
	r, fileObj, _ := testRunnerWithAttachment(t, contents)
	setAttachmentRoleView(t, r, fileObj)

	promptResult, handled, err := r.executeLocalAction("plaintiff", "request_case_file", map[string]any{"file_id": "file-0001"})
	if err != nil {
		t.Fatalf("request_case_file error = %v", err)
	}
	if !handled || len(promptResult.FollowupInputItems) != 1 {
		t.Fatalf("request_case_file result = %+v handled=%v", promptResult, handled)
	}
	items, ok := promptResult.FollowupInputItems[0]["content_items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("prompt content items = %#v", promptResult.FollowupInputItems[0]["content_items"])
	}
	if items[1]["type"] != "input_file" || items[1]["filename"] != "record.txt" {
		t.Fatalf("prompt file item = %#v", items[1])
	}
	fileData, _ := items[1]["file_data"].(string)
	promptBytes, err := base64.StdEncoding.DecodeString(fileData)
	if err != nil {
		t.Fatalf("decode prompt file_data: %v", err)
	}
	if string(promptBytes) != string(contents) {
		t.Fatalf("prompt bytes = %q", promptBytes)
	}

	apiResult := r.readCaseFileBytes("plaintiff", "file-0001")
	if ok, _ := apiResult["ok"].(bool); !ok {
		t.Fatalf("readCaseFileBytes() = %#v", apiResult)
	}
	apiBytes, err := base64.StdEncoding.DecodeString(stringOrDefault(apiResult["content_base64"], ""))
	if err != nil {
		t.Fatalf("decode API content_base64: %v", err)
	}
	if string(apiBytes) != string(contents) {
		t.Fatalf("API bytes = %q", apiBytes)
	}
}

func TestEvidenceManifestVerifiesComplaintAttachment(t *testing.T) {
	t.Parallel()

	r, _, path := testRunnerWithAttachment(t, []byte("record text\n"))
	r.cfg.OutputPath = filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(path, []byte("record next\n"), 0o644); err != nil {
		t.Fatalf("replace attachment bytes: %v", err)
	}
	if err := r.writeEvidenceManifest(); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("writeEvidenceManifest() error = %v", err)
	}
}

func testRunnerWithAttachment(t *testing.T, contents []byte) (*Runner, map[string]any, string) {
	t.Helper()

	root := t.TempDir()
	path := filepath.Join(root, "documents", "record.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create attachment directory: %v", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	digest := sha256.Sum256(contents)
	attachment := spec.ComplaintAttachmentSpec{
		FileID:         "file-0001",
		Label:          "Record",
		OriginalName:   "record.txt",
		StorageRelPath: "documents/record.txt",
		Sha256:         hex.EncodeToString(digest[:]),
		SizeBytes:      len(contents),
	}
	fileObj := map[string]any{
		"file_id":         attachment.FileID,
		"label":           attachment.Label,
		"original_name":   attachment.OriginalName,
		"storage_relpath": attachment.StorageRelPath,
		"sha256":          attachment.Sha256,
		"size_bytes":      attachment.SizeBytes,
	}
	r := &Runner{
		scenario: spec.FormalScenario{CaseInit: &spec.CaseInitializationSpec{Attachments: []spec.ComplaintAttachmentSpec{attachment}}},
		cfg:      Config{ScenarioBaseDir: root},
		state: map[string]any{
			"case": map[string]any{"case_files": []any{fileObj}},
		},
	}
	return r, fileObj, path
}

func setAttachmentRoleView(t *testing.T, r *Runner, fileObj map[string]any) {
	t.Helper()

	response := map[string]any{
		"ok": true,
		"view": map[string]any{
			"state": map[string]any{
				"case": map[string]any{"case_files": []any{fileObj}},
			},
		},
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal role view: %v", err)
	}
	path := filepath.Join(t.TempDir(), "role-view.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write role view: %v", err)
	}
	r.lean = lean.New([]string{"cat", path})
}
