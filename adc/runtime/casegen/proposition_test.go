package casegen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/common/documents"
)

func TestCreatePropositionPlan(t *testing.T) {
	t.Parallel()

	const proposition = "The sky is blue."
	plan, err := CreatePropositionPlan(proposition, EvidenceStandardPreponderance, "jury")
	if err != nil {
		t.Fatalf("CreatePropositionPlan() error = %v", err)
	}
	packet := plan.Packet
	if packet.Caption != "Proponent v. Opponent" || packet.PlaintiffName != "Proponent" || packet.DefendantName != "Opponent" {
		t.Fatalf("parties = %q, %q, %q", packet.Caption, packet.PlaintiffName, packet.DefendantName)
	}
	if packet.ComplaintSummary != proposition || packet.Claim.Label != proposition {
		t.Fatalf("proposition was not preserved: %+v", packet)
	}
	if packet.Claim.StandardOfProof != EvidenceStandardPreponderance || packet.Claim.BurdenHolder != "plaintiff" {
		t.Fatalf("claim burden = %+v", packet.Claim)
	}
	if len(packet.Claim.Elements) != 1 || !strings.Contains(packet.RequestedRelief, "declaration") || !strings.Contains(packet.Claim.DamagesQuestion, "zero") {
		t.Fatalf("proposition claim = %+v", packet)
	}
	if !packet.Claim.DeclaratoryOnly {
		t.Fatalf("proposition claim permits monetary damages: %+v", packet.Claim)
	}
	if packet.Claim.LegalTheory != courts.PropositionAdjudicationBasis || packet.JurisdictionBasis != courts.PropositionAdjudicationBasis {
		t.Fatalf("proposition basis = %+v", packet)
	}
	wantPlaintiffStrategy := "# Proponent Strategy\n\nProposition:\n\nThe sky is blue.\n\nEvidence standard: `preponderance_of_the_evidence`.  The Proponent bears the burden to demonstrate the proposition.  Use the imported documents and the trial record to address the proposition, and seek declaratory judgment with no monetary damages."
	if plan.PlaintiffStrategy != wantPlaintiffStrategy {
		t.Fatalf("plaintiff strategy = %q, want %q", plan.PlaintiffStrategy, wantPlaintiffStrategy)
	}
	wantDefenseStrategy := "# Opponent Strategy\n\nProposition:\n\nThe sky is blue.\n\nEvidence standard: `preponderance_of_the_evidence`.  The Opponent may prevail by showing that the Proponent has not met the burden.  Use the imported documents and the trial record to address the proposition, and seek declaratory judgment with no monetary damages."
	if plan.DefenseStrategy != wantDefenseStrategy {
		t.Fatalf("defense strategy = %q, want %q", plan.DefenseStrategy, wantDefenseStrategy)
	}
}

func TestCreatePropositionPlanValidatesInputs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		proposition string
		standard    string
		trialMode   string
		want        string
	}{
		{name: "proposition", proposition: " ", standard: EvidenceStandardPreponderance, trialMode: "jury", want: "proposition is required"},
		{name: "standard", proposition: "P", standard: "balance", trialMode: "jury", want: "evidence standard"},
		{name: "trial mode", proposition: "P", standard: EvidenceStandardPreponderance, trialMode: "auto", want: "trial mode"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := CreatePropositionPlan(test.proposition, test.standard, test.trialMode)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("CreatePropositionPlan() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestBuildPropositionScenarioPreservesDocumentManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	documentsRoot := filepath.Join(dir, "documents")
	if err := os.Mkdir(documentsRoot, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	contents := []byte("source material\n")
	if err := os.WriteFile(filepath.Join(documentsRoot, "record.txt"), contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	digest := sha256.Sum256(contents)
	manifest := documents.Manifest{
		SchemaVersion: documents.SchemaVersion,
		TotalBytes:    int64(len(contents)),
		Files: []documents.File{{
			Path:      "record.txt",
			SizeBytes: int64(len(contents)),
			SHA256:    hex.EncodeToString(digest[:]),
			MediaType: "text/plain",
		}},
	}
	plan, err := CreatePropositionPlan("The record is accurate.", EvidenceStandardClearConvincing, "bench")
	if err != nil {
		t.Fatalf("CreatePropositionPlan() error = %v", err)
	}
	scenario, err := BuildPropositionScenario(plan, manifest, documentsRoot, "documents", ScenarioOptions{
		RuntimeModel:        "runtime",
		PlaintiffModel:      "plaintiff",
		DefendantModel:      "defendant",
		JudgeModel:          "judge",
		ClerkModel:          "clerk",
		Court:               courts.PropositionTribunal(),
		FiledOn:             "2026-08-18",
		TrialModeOverride:   "bench",
		NonJurorTemperature: nil,
	})
	if err != nil {
		t.Fatalf("BuildPropositionScenario() error = %v", err)
	}
	if scenario.Name != "go_case_proposition_adjudication" || scenario.CourtName != courts.PropositionTribunalName {
		t.Fatalf("scenario identity = %q, %q", scenario.Name, scenario.CourtName)
	}
	if scenario.LoopPolicy == nil || scenario.LoopPolicy.MaxTurns != 500 {
		t.Fatalf("loop policy = %+v", scenario.LoopPolicy)
	}
	if scenario.CaseInit == nil || len(scenario.CaseInit.Attachments) != 1 {
		t.Fatalf("case init = %+v", scenario.CaseInit)
	}
	if scenario.CaseInit.FiledBy != "plaintiff" || scenario.CaseInit.JuryDemandedOn != "" {
		t.Fatalf("bench case initialization = %+v", scenario.CaseInit)
	}
	if len(scenario.Claims) != 1 || scenario.Claims[0].BurdenHolder != "plaintiff" || !scenario.Claims[0].DeclaratoryOnly {
		t.Fatalf("claims = %+v", scenario.Claims)
	}
	attachment := scenario.CaseInit.Attachments[0]
	if attachment.StorageRelPath != "documents/record.txt" || attachment.Sha256 != manifest.Files[0].SHA256 || attachment.SizeBytes != len(contents) {
		t.Fatalf("attachment = %+v", attachment)
	}
}

func TestBuildPropositionScenarioRejectsChangedDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "record.txt")
	contents := []byte("changed")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manifest := documents.Manifest{
		SchemaVersion: documents.SchemaVersion,
		TotalBytes:    int64(len(contents)),
		Files: []documents.File{{
			Path:      "record.txt",
			SizeBytes: int64(len(contents)),
			SHA256:    strings.Repeat("0", 64),
		}},
	}
	plan, err := CreatePropositionPlan("P", EvidenceStandardPreponderance, "bench")
	if err != nil {
		t.Fatalf("CreatePropositionPlan() error = %v", err)
	}
	_, err = BuildPropositionScenario(plan, manifest, dir, "documents", ScenarioOptions{Court: courts.PropositionTribunal()})
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("BuildPropositionScenario() error = %v", err)
	}
}
