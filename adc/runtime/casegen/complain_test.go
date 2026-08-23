package casegen

import (
	"strings"
	"testing"

	"github.com/agentcourt/adj/adc/runtime/courts"
)

func testUSDistrictProfile() courts.Profile {
	return courts.Profile{
		Name:                         courts.DefaultCourtName,
		RulesMarkdown:                "Federal civil rules apply.",
		JurisdictionScreen:           true,
		AllowedJurisdictionBases:     []string{"federal_question", "diversity", "unspecified"},
		RequireJurisdictionStatement: true,
		RequireDiversityCitizenship:  true,
		RequireAmountInControversy:   true,
		MinimumAmountInControversy:   75000,
	}
}

func testInternationalClawProfile() courts.Profile {
	return courts.Profile{
		Name:                         courts.InternationalClawDistrictName,
		RulesMarkdown:                "General civil jurisdiction applies.",
		JurisdictionScreen:           false,
		AllowedJurisdictionBases:     []string{"general_civil"},
		PreferredJurisdictionBasis:   "general_civil",
		RequireJurisdictionStatement: true,
	}
}

func TestBuildComplaintDraftPromptIncludesSituationAndLinks(t *testing.T) {
	t.Parallel()

	source := ComplaintInput{
		Markdown: "# Situation\n\nA dispute.",
		LinkedFiles: []LinkedFile{
			{
				Label:         "instructions",
				ReferencePath: "./instructions.txt",
				OriginalName:  "instructions.txt",
				PreviewKind:   "text",
				Preview:       "first line",
			},
		},
	}

	prompt, err := buildComplaintDraftPrompt(source, testUSDistrictProfile())
	if err != nil {
		t.Fatalf("buildComplaintDraftPrompt: %v", err)
	}
	want := []string{
		"Situation markdown follows.",
		"# Situation",
		"Linked local references:",
		"original_name: instructions.txt",
		"reference_path: ./instructions.txt",
		"preview_kind: text",
		"first line",
	}
	for _, needle := range want {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("buildComplaintDraftPrompt missing %q\n%s", needle, prompt)
		}
	}
}

func TestValidateComplaintDraftRequiresLinkWhenSourceHasLinks(t *testing.T) {
	t.Parallel()

	source := ComplaintInput{
		OriginalPath: "/tmp/example/situation.md",
		LinkedFiles: []LinkedFile{
			{ReferencePath: "./confession.txt", OriginalPath: "/tmp/example/confession.txt"},
		},
	}
	text := "# Complaint\n\n## Parties\n\nA.\n\n## Jurisdiction\n\nB.\n\n## Facts\n\nC.\n\n## Claim\n\nBreach of contract and misrepresentation.\n\n## Relief Requested\n\nD.\n"
	err := validateComplaintDraft(text, source, testUSDistrictProfile())
	if err == nil {
		t.Fatalf("validateComplaintDraft error = nil, want error")
	}
	if !strings.Contains(err.Error(), "must cite linked files") {
		t.Fatalf("validateComplaintDraft error = %v", err)
	}
}

func TestValidateComplaintDraftRequiresAllSourceLinks(t *testing.T) {
	t.Parallel()

	source := []LinkedFile{
		{ReferencePath: "./instructions.txt"},
		{ReferencePath: "./session-summary.txt"},
	}
	draft := []LinkedFile{
		{ReferencePath: "./instructions.txt"},
	}
	err := validateComplaintDraftLinks(source, draft)
	if err == nil {
		t.Fatalf("validateComplaintDraftLinks error = nil, want error")
	}
	if !strings.Contains(err.Error(), "omitted referenced file: ./session-summary.txt") {
		t.Fatalf("validateComplaintDraftLinks error = %v", err)
	}
}

func TestSourceSupportsDiversityJurisdiction(t *testing.T) {
	t.Parallel()

	markdown := strings.Join([]string{
		"Peter is a citizen of Texas.",
		"Samantha is a citizen of Massachusetts.",
		"Peter seeks $108,000 in damages.",
	}, "\n")
	if !sourceSupportsDiversityJurisdiction(markdown, 75000) {
		t.Fatalf("sourceSupportsDiversityJurisdiction = false, want true")
	}
}

func TestValidateComplaintDraftRejectsDiversityDenial(t *testing.T) {
	t.Parallel()

	source := ComplaintInput{
		OriginalPath: "/tmp/example/situation.md",
		Markdown: strings.Join([]string{
			"Peter is a citizen of Texas.",
			"Samantha is a citizen of Massachusetts.",
			"Peter seeks $108,000 in damages.",
		}, "\n"),
	}
	text := strings.Join([]string{
		"# Complaint",
		"",
		"## Parties",
		"",
		"Plaintiff Peter is a citizen of Texas.  Defendant Samantha is a citizen of Massachusetts.",
		"",
		"## Jurisdiction",
		"",
		"This Court lacks a basis for federal subject-matter jurisdiction based on the facts provided.",
		"",
		"## Facts",
		"",
		"A.",
		"",
		"## Claim",
		"",
		"Breach of contract and misrepresentation.",
		"",
		"## Relief Requested",
		"",
		"$108,000.",
	}, "\n")
	err := validateComplaintDraft(text, source, testUSDistrictProfile())
	if err == nil {
		t.Fatalf("validateComplaintDraft error = nil, want error")
	}
	if !strings.Contains(err.Error(), "denied federal jurisdiction") {
		t.Fatalf("validateComplaintDraft error = %v", err)
	}
}

func TestValidateComplaintDraftRequiresExpressDiversityPleading(t *testing.T) {
	t.Parallel()

	source := ComplaintInput{
		OriginalPath: "/tmp/example/situation.md",
		Markdown: strings.Join([]string{
			"Peter is a citizen of Texas.",
			"Samantha is a citizen of Massachusetts.",
			"Peter seeks $108,000 in damages.",
		}, "\n"),
	}
	text := strings.Join([]string{
		"# Complaint",
		"",
		"## Parties",
		"",
		"Plaintiff Peter is a citizen of Texas.  Defendant Samantha is a citizen of Massachusetts.",
		"",
		"## Jurisdiction",
		"",
		"The amount in controversy exceeds $75,000.",
		"",
		"## Facts",
		"",
		"A.",
		"",
		"## Claim",
		"",
		"Breach of contract and misrepresentation.",
		"",
		"## Relief Requested",
		"",
		"$108,000.",
	}, "\n")
	err := validateComplaintDraft(text, source, testUSDistrictProfile())
	if err == nil {
		t.Fatalf("validateComplaintDraft error = nil, want error")
	}
	if !strings.Contains(err.Error(), "must plead diversity jurisdiction") {
		t.Fatalf("validateComplaintDraft error = %v", err)
	}
}

func TestValidateComplaintDraftRejectsFederalScreeningForInternationalClaw(t *testing.T) {
	t.Parallel()

	source := ComplaintInput{
		OriginalPath: "/tmp/example/situation.md",
		Markdown:     "Peter lives in Texas.  Samantha lives in Massachusetts.  Peter seeks $108.",
	}
	text := strings.Join([]string{
		"# Complaint",
		"",
		"## Parties",
		"",
		"Peter and Samantha.",
		"",
		"## Jurisdiction",
		"",
		"This Court lacks a basis for federal subject-matter jurisdiction.",
		"",
		"## Facts",
		"",
		"A.",
		"",
		"## Claim",
		"",
		"Breach of contract.",
		"",
		"## Relief Requested",
		"",
		"$108.",
	}, "\n")
	err := validateComplaintDraft(text, source, testInternationalClawProfile())
	if err == nil {
		t.Fatalf("validateComplaintDraft error = nil, want error")
	}
	if !strings.Contains(err.Error(), "used federal subject-matter jurisdiction screening") {
		t.Fatalf("validateComplaintDraft error = %v", err)
	}
}
