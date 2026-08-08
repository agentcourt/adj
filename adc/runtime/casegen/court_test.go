package casegen

import (
	"testing"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func TestValidateCasePacketAllowsGeneralCivilInInternationalClaw(t *testing.T) {
	t.Parallel()

	packet := CasePacket{
		Caption:                  "Peter v. Samantha",
		PlaintiffName:            "Peter",
		DefendantName:            "Samantha",
		ComplaintSummary:         "Breach of contract over a bad essay.",
		RequestedRelief:          "$108 in damages.",
		TrialModeRecommendation:  "jury",
		JurisdictionBasis:        "general_civil",
		JurisdictionalStatement:  "The International Claw District hears this civil dispute.",
		InjuryStatement:          "Peter lost money printing the essay.",
		CausationStatement:       "The bad essay caused the printing loss.",
		RedressabilityStatement:  "Damages would redress the loss.",
		RipenessStatement:        "The dispute is present and concrete.",
		LiveControversyStatement: "The parties dispute liability and relief.",
		Claim:                    specClaim(),
	}
	if err := validateCasePacket(packet, testInternationalClawProfile()); err != nil {
		t.Fatalf("validateCasePacket() error = %v", err)
	}
}

func TestValidateCasePacketRejectsSmallDiversityAmountInUSDistrict(t *testing.T) {
	t.Parallel()

	packet := CasePacket{
		Caption:                  "Peter v. Samantha",
		PlaintiffName:            "Peter",
		DefendantName:            "Samantha",
		ComplaintSummary:         "Breach of contract over a bad essay.",
		RequestedRelief:          "$108 in damages.",
		TrialModeRecommendation:  "jury",
		JurisdictionBasis:        "diversity",
		JurisdictionalStatement:  "The parties are citizens of different States.",
		InjuryStatement:          "Peter lost money printing the essay.",
		CausationStatement:       "The bad essay caused the printing loss.",
		RedressabilityStatement:  "Damages would redress the loss.",
		RipenessStatement:        "The dispute is present and concrete.",
		LiveControversyStatement: "The parties dispute liability and relief.",
		PlaintiffCitizenship:     "Texas",
		DefendantCitizenship:     "Massachusetts",
		AmountInControversy:      "108",
		Claim:                    specClaim(),
	}
	if err := validateCasePacket(packet, testUSDistrictProfile()); err == nil {
		t.Fatalf("validateCasePacket() error = nil, want amount threshold error")
	}
}

func specClaim() spec.ClaimSpec {
	return spec.ClaimSpec{
		ClaimID:         "claim-1",
		Label:           "Breach of contract",
		LegalTheory:     "breach_of_contract",
		StandardOfProof: "preponderance_of_the_evidence",
		BurdenHolder:    "plaintiff",
		Elements:        []string{"contract", "breach", "damages"},
		DamagesQuestion: "What damages, if any, are proven?",
	}
}
