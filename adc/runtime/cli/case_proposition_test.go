package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/casegen"
	"github.com/agentcourt/adj/adc/runtime/courts"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/common/documents"
)

func TestValidateCaseInputFlags(t *testing.T) {
	t.Parallel()

	validLimits := caseInputFlags{
		Proposition:            "P",
		EvidenceStandard:       casegen.EvidenceStandardPreponderance,
		MaxDocumentFiles:       1,
		MaxDocumentFileBytes:   1,
		MaxDocumentsTotalBytes: 1,
	}
	tests := []struct {
		name  string
		flags caseInputFlags
		want  string
	}{
		{name: "neither", want: "exactly one"},
		{name: "both", flags: caseInputFlags{ComplaintPath: "complaint.md", Proposition: "P"}, want: "exactly one"},
		{name: "standard", flags: caseInputFlags{Proposition: "P", MaxDocumentFiles: 1, MaxDocumentFileBytes: 1, MaxDocumentsTotalBytes: 1}, want: "--evidence-standard"},
		{name: "unsupported standard", flags: caseInputFlags{Proposition: "P", EvidenceStandard: "balance", MaxDocumentFiles: 1, MaxDocumentFileBytes: 1, MaxDocumentsTotalBytes: 1}, want: "invalid --evidence-standard"},
		{name: "file count", flags: caseInputFlags{Proposition: "P", EvidenceStandard: casegen.EvidenceStandardPreponderance, MaxDocumentFileBytes: 1, MaxDocumentsTotalBytes: 1}, want: "--max-document-files"},
		{name: "file bytes", flags: caseInputFlags{Proposition: "P", EvidenceStandard: casegen.EvidenceStandardPreponderance, MaxDocumentFiles: 1, MaxDocumentsTotalBytes: 1}, want: "--max-document-file-bytes"},
		{name: "total bytes", flags: caseInputFlags{Proposition: "P", EvidenceStandard: casegen.EvidenceStandardPreponderance, MaxDocumentFiles: 1, MaxDocumentFileBytes: 1}, want: "--max-documents-total-bytes"},
		{name: "court", flags: caseInputFlags{Proposition: "P", EvidenceStandard: casegen.EvidenceStandardPreponderance, MaxDocumentFiles: 1, MaxDocumentFileBytes: 1, MaxDocumentsTotalBytes: 1, CourtSet: true}, want: "--court cannot be used"},
		{name: "planner model", flags: caseInputFlags{Proposition: "P", EvidenceStandard: casegen.EvidenceStandardPreponderance, MaxDocumentFiles: 1, MaxDocumentFileBytes: 1, MaxDocumentsTotalBytes: 1, PlannerModelSet: true}, want: "--planner-model cannot be used"},
		{name: "complaint proposition flag", flags: caseInputFlags{ComplaintPath: "complaint.md", DocumentsDir: "documents"}, want: "require --proposition"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateCaseInputFlags(test.flags)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateCaseInputFlags() error = %v, want %q", err, test.want)
			}
		})
	}
	if err := validateCaseInputFlags(caseInputFlags{ComplaintPath: "complaint.md"}); err != nil {
		t.Fatalf("complaint flags error = %v", err)
	}
	if err := validateCaseInputFlags(validLimits); err != nil {
		t.Fatalf("proposition flags error = %v", err)
	}
}

func TestRunCaseRejectsConflictingInputsBeforeProviderSetup(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunCase(context.Background(), []string{"--complaint", "complaint.md", "--proposition", "P"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("RunCase() error = %v", err)
	}
}

func TestRunCaseHelpIncludesPropositionFlags(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunCase(context.Background(), []string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunCase() error = %v", err)
	}
	for _, flag := range []string{"--proposition", "--evidence-standard", "--documents", "--max-document-files", "--max-document-file-bytes", "--max-documents-total-bytes", "--prompt-dir", "--prompt-file"} {
		if !strings.Contains(stderr.String(), strings.TrimPrefix(flag, "-")) {
			t.Fatalf("help omitted %s: %s", flag, stderr.String())
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	var promptFiles promptFileFlag
	if err := promptFiles.Set("complaint.system=one.md"); err != nil {
		t.Fatalf("first prompt override: %v", err)
	}
	if err := promptFiles.Set("complaint.system=two.md"); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("duplicate prompt override error = %v", err)
	}
	if err := promptFiles.Set("unknown=one.md"); err == nil || !strings.Contains(err.Error(), "unknown ADC prompt id") {
		t.Fatalf("unknown prompt override error = %v", err)
	}
}

func TestPreparePropositionScenarioImportsDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	contents := []byte("record contents\n")
	if err := os.WriteFile(filepath.Join(source, "nested", "record.txt"), contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	proponentPromptPath := filepath.Join(root, "proponent.md")
	if err := os.WriteFile(proponentPromptPath, []byte("Custom proponent strategy for {{PROPOSITION}} under {{EVIDENCE_STANDARD}}."), 0o644); err != nil {
		t.Fatalf("WriteFile(proponent prompt) error = %v", err)
	}
	outDir := filepath.Join(root, "out")
	result, err := preparePropositionScenario(propositionSetupOptions{
		Proposition:      "The record is accurate.",
		EvidenceStandard: casegen.EvidenceStandardClearConvincing,
		DocumentsDir:     source,
		DocumentLimits: documents.Limits{
			MaxFiles:      2,
			MaxFileBytes:  1024,
			MaxTotalBytes: 2048,
		},
		OutDir:            outDir,
		TrialModeOverride: "auto",
		PromptFiles: map[string]string{
			adcprompts.PropositionProponentStrategyID: proponentPromptPath,
		},
	})
	if err != nil {
		t.Fatalf("preparePropositionScenario() error = %v", err)
	}

	raw, err := os.ReadFile(result.DocumentManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(documents.json) error = %v", err)
	}
	var manifest documents.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("Unmarshal(documents.json) error = %v", err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "nested/record.txt" || manifest.TotalBytes != int64(len(contents)) {
		t.Fatalf("manifest = %+v", manifest)
	}
	copied, err := os.ReadFile(filepath.Join(result.DocumentsPath, "nested", "record.txt"))
	if err != nil {
		t.Fatalf("ReadFile(imported document) error = %v", err)
	}
	if !bytes.Equal(copied, contents) {
		t.Fatalf("imported contents = %q", copied)
	}

	scenario, err := spec.Load(result.ScenarioPath)
	if err != nil {
		t.Fatalf("spec.Load() error = %v", err)
	}
	if scenario.CourtName != courts.PropositionTribunalName {
		t.Fatalf("CourtName = %q", scenario.CourtName)
	}
	if scenario.CaseInit == nil || scenario.CaseInit.JuryDemandedOn == "" {
		t.Fatalf("auto trial mode did not resolve to jury: %+v", scenario.CaseInit)
	}
	if len(scenario.CaseInit.Attachments) != 1 || scenario.CaseInit.Attachments[0].Sha256 != manifest.Files[0].SHA256 {
		t.Fatalf("attachments = %+v", scenario.CaseInit.Attachments)
	}
	if len(scenario.Claims) != 1 || scenario.Claims[0].StandardOfProof != casegen.EvidenceStandardClearConvincing {
		t.Fatalf("claims = %+v", scenario.Claims)
	}
	plaintiffStrategy, err := os.ReadFile(result.PlaintiffStrategyPath)
	if err != nil {
		t.Fatalf("ReadFile(plaintiff strategy) error = %v", err)
	}
	if got, want := string(plaintiffStrategy), "Custom proponent strategy for The record is accurate. under clear_and_convincing.\n"; got != want {
		t.Fatalf("plaintiff strategy = %q, want %q", got, want)
	}
	for _, path := range []string{result.NormalizedCasePath, result.PlaintiffStrategyPath, result.DefenseStrategyPath, result.ScenarioPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%s) error = %v", path, err)
		}
	}
}

func TestPreparePropositionScenarioCreatesEmptyDocumentRecord(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "out")
	result, err := preparePropositionScenario(propositionSetupOptions{
		Proposition:      "P",
		EvidenceStandard: casegen.EvidenceStandardPreponderance,
		DocumentLimits: documents.Limits{
			MaxFiles:      1,
			MaxFileBytes:  1,
			MaxTotalBytes: 1,
		},
		OutDir:            outDir,
		TrialModeOverride: "bench",
	})
	if err != nil {
		t.Fatalf("preparePropositionScenario() error = %v", err)
	}
	raw, err := os.ReadFile(result.DocumentManifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var manifest documents.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if manifest.SchemaVersion != documents.SchemaVersion || len(manifest.Files) != 0 || manifest.TotalBytes != 0 {
		t.Fatalf("manifest = %+v", manifest)
	}
	info, err := os.Stat(result.DocumentsPath)
	if err != nil || !info.IsDir() {
		t.Fatalf("documents directory: info=%v error=%v", info, err)
	}
}

func TestPreparePropositionScenarioRejectsInvalidConfigurationBeforeCreatingOutput(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "out")
	_, err := preparePropositionScenario(propositionSetupOptions{
		Proposition:      "P",
		EvidenceStandard: "unsupported",
		DocumentLimits: documents.Limits{
			MaxFiles:      1,
			MaxFileBytes:  1,
			MaxTotalBytes: 1,
		},
		OutDir:            outDir,
		TrialModeOverride: "jury",
	})
	if err == nil || !strings.Contains(err.Error(), "evidence standard") {
		t.Fatalf("preparePropositionScenario() error = %v", err)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Fatalf("output directory exists after validation failure: %v", statErr)
	}
}
