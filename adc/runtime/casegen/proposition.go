package casegen

import (
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/courts"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
	"github.com/agentcourt/adj/common/documents"
)

const (
	EvidenceStandardPreponderance   = "preponderance_of_the_evidence"
	EvidenceStandardClearConvincing = "clear_and_convincing"
)

func CreatePropositionPlan(proposition, evidenceStandard, trialMode string) (Plan, error) {
	return CreatePropositionPlanWithOptions(proposition, evidenceStandard, trialMode, PlanningOptions{})
}

func CreatePropositionPlanWithOptions(proposition, evidenceStandard, trialMode string, opts PlanningOptions) (Plan, error) {
	proposition = strings.TrimSpace(proposition)
	if proposition == "" {
		return Plan{}, fmt.Errorf("proposition is required")
	}
	evidenceStandard = strings.TrimSpace(evidenceStandard)
	if err := ValidateEvidenceStandard(evidenceStandard); err != nil {
		return Plan{}, err
	}
	trialMode = strings.ToLower(strings.TrimSpace(trialMode))
	if trialMode != "jury" && trialMode != "bench" {
		return Plan{}, fmt.Errorf("proposition trial mode must be jury or bench")
	}

	packet := CasePacket{
		Caption:                  "Proponent v. Opponent",
		PlaintiffName:            "Proponent",
		DefendantName:            "Opponent",
		ComplaintSummary:         proposition,
		RequestedRelief:          "A declaration whether the submitted proposition is demonstrated under the configured evidence standard.",
		TrialModeRecommendation:  trialMode,
		JurisdictionBasis:        courts.PropositionAdjudicationBasis,
		JurisdictionalStatement:  "The Proposition Tribunal has jurisdiction to adjudicate the submitted proposition.",
		InjuryStatement:          "The submitted proposition remains disputed.",
		CausationStatement:       "The parties maintain opposing positions on the submitted proposition.",
		RedressabilityStatement:  "A declaration will resolve whether the submitted proposition is demonstrated.",
		RipenessStatement:        "The proposition has been submitted for present adjudication.",
		LiveControversyStatement: "The Proponent and Opponent dispute whether the submitted proposition is demonstrated.",
		Claim: spec.ClaimSpec{
			ClaimID:         "proposition-1",
			Label:           proposition,
			LegalTheory:     courts.PropositionAdjudicationBasis,
			StandardOfProof: evidenceStandard,
			BurdenHolder:    "plaintiff",
			Elements:        []string{"The submitted proposition is demonstrated."},
			Defenses:        []string{},
			DamagesQuestion: "Monetary damages are unavailable; any damages amount must be zero.",
			DeclaratoryOnly: true,
		},
	}
	if err := validateCasePacket(packet, courts.PropositionTribunal()); err != nil {
		return Plan{}, err
	}
	promptCatalog, err := adcprompts.Load(adcprompts.Options{PromptDir: opts.PromptDir, PromptFiles: opts.PromptFiles})
	if err != nil {
		return Plan{}, err
	}
	strategyValues := map[string]string{
		"{{PROPOSITION}}":       proposition,
		"{{EVIDENCE_STANDARD}}": evidenceStandard,
	}
	proponentStrategy, err := promptCatalog.Render(adcprompts.PropositionProponentStrategyID, strategyValues)
	if err != nil {
		return Plan{}, err
	}
	opponentStrategy, err := promptCatalog.Render(adcprompts.PropositionOpponentStrategyID, strategyValues)
	if err != nil {
		return Plan{}, err
	}

	return Plan{
		Packet:            packet,
		PlaintiffStrategy: proponentStrategy,
		DefenseStrategy:   opponentStrategy,
	}, nil
}

func BuildPropositionScenario(plan Plan, manifest documents.Manifest, documentsRoot, documentsRelPath string, opts ScenarioOptions) (spec.FormalScenario, error) {
	if opts.Court.Name != courts.PropositionTribunalName {
		return spec.FormalScenario{}, fmt.Errorf("proposition scenario requires %s", courts.PropositionTribunalName)
	}
	attachments, err := propositionAttachments(manifest, documentsRoot, documentsRelPath)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	scenario, err := BuildScenario(plan, ComplaintInput{OriginalPath: "proposition.md"}, opts)
	if err != nil {
		return spec.FormalScenario{}, err
	}
	scenario.Name = "go_case_proposition_adjudication"
	scenario.CaseInit.Attachments = attachments
	return scenario, nil
}

func ValidateEvidenceStandard(standard string) error {
	switch standard {
	case EvidenceStandardPreponderance, EvidenceStandardClearConvincing:
		return nil
	default:
		return fmt.Errorf("evidence standard must be %s or %s", EvidenceStandardPreponderance, EvidenceStandardClearConvincing)
	}
}

func propositionAttachments(manifest documents.Manifest, documentsRoot, documentsRelPath string) ([]spec.ComplaintAttachmentSpec, error) {
	if manifest.SchemaVersion != documents.SchemaVersion {
		return nil, fmt.Errorf("document manifest schema_version must be %q", documents.SchemaVersion)
	}
	documentsRoot = strings.TrimSpace(documentsRoot)
	if documentsRoot == "" {
		return nil, fmt.Errorf("documents root is required")
	}
	documentsRelPath = filepath.ToSlash(strings.TrimSpace(documentsRelPath))
	if documentsRelPath == "" || filepath.IsAbs(filepath.FromSlash(documentsRelPath)) || filepath.ToSlash(filepath.Clean(filepath.FromSlash(documentsRelPath))) != documentsRelPath || documentsRelPath == ".." || strings.HasPrefix(documentsRelPath, "../") {
		return nil, fmt.Errorf("documents relative path must remain within the scenario directory")
	}

	attachments := make([]spec.ComplaintAttachmentSpec, 0, len(manifest.Files))
	var total int64
	seen := make(map[string]struct{}, len(manifest.Files))
	for i, file := range manifest.Files {
		if _, ok := seen[file.Path]; ok {
			return nil, fmt.Errorf("document manifest repeats path %q", file.Path)
		}
		seen[file.Path] = struct{}{}
		if file.SizeBytes < 0 || file.SizeBytes > int64(math.MaxInt) {
			return nil, fmt.Errorf("document %q size cannot be represented by ADC", file.Path)
		}
		decoded, err := hex.DecodeString(file.SHA256)
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("document %q has invalid SHA-256", file.Path)
		}
		if _, err := documents.ReadVerified(documentsRoot, file); err != nil {
			return nil, fmt.Errorf("verify document %q: %w", file.Path, err)
		}
		if total > math.MaxInt64-file.SizeBytes {
			return nil, fmt.Errorf("document manifest total byte count overflows")
		}
		total += file.SizeBytes
		attachments = append(attachments, spec.ComplaintAttachmentSpec{
			FileID:         fmt.Sprintf("file-%04d", i+1),
			Label:          file.Path,
			OriginalName:   filepath.Base(filepath.FromSlash(file.Path)),
			StorageRelPath: filepath.ToSlash(filepath.Join(filepath.FromSlash(documentsRelPath), filepath.FromSlash(file.Path))),
			Sha256:         file.SHA256,
			SizeBytes:      int(file.SizeBytes),
		})
	}
	if total != manifest.TotalBytes {
		return nil, fmt.Errorf("document manifest total_bytes is %d, calculated %d", manifest.TotalBytes, total)
	}
	return attachments, nil
}
