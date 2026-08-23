package runner

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/common/documents"
)

func (r *Runner) readCaseFile(fileObj map[string]any) ([]byte, error) {
	fileID := strings.TrimSpace(stringOrDefault(fileObj["file_id"], ""))
	if fileID == "" {
		return nil, fmt.Errorf("case file missing file_id")
	}
	attachment, found, err := r.scenarioAttachment(fileID)
	if err != nil {
		return nil, err
	}
	if found {
		raw, err := readVerifiedAttachment(r.cfg.ScenarioBaseDir, attachment)
		if err != nil {
			return nil, fmt.Errorf("read complaint attachment %s: %w", fileID, err)
		}
		return raw, nil
	}
	storedPath := strings.TrimSpace(stringOrDefault(fileObj["storage_relpath"], ""))
	if storedPath == "" {
		return nil, fmt.Errorf("case file %s has no storage_relpath", fileID)
	}
	raw, err := os.ReadFile(resolveStoredCaseFilePath(storedPath, r.cfg.ScenarioBaseDir))
	if err != nil {
		return nil, fmt.Errorf("read case file %s: %w", fileID, err)
	}
	return raw, nil
}

func (r *Runner) scenarioAttachment(fileID string) (spec.ComplaintAttachmentSpec, bool, error) {
	if r.scenario.CaseInit == nil {
		return spec.ComplaintAttachmentSpec{}, false, nil
	}
	var found *spec.ComplaintAttachmentSpec
	for i := range r.scenario.CaseInit.Attachments {
		attachment := &r.scenario.CaseInit.Attachments[i]
		if strings.TrimSpace(attachment.FileID) != fileID {
			continue
		}
		if found != nil {
			return spec.ComplaintAttachmentSpec{}, false, fmt.Errorf("formal scenario repeats complaint attachment file_id %q", fileID)
		}
		found = attachment
	}
	if found == nil {
		return spec.ComplaintAttachmentSpec{}, false, nil
	}
	return *found, true, nil
}

func readVerifiedAttachment(root string, attachment spec.ComplaintAttachmentSpec) ([]byte, error) {
	path := strings.TrimSpace(attachment.StorageRelPath)
	if path == "" {
		return nil, fmt.Errorf("complaint attachment %q has no storage_relpath", attachment.FileID)
	}
	return documents.ReadVerified(root, documents.File{
		Path:      path,
		SizeBytes: int64(attachment.SizeBytes),
		SHA256:    strings.TrimSpace(attachment.Sha256),
	})
}
