package spec

import "testing"

func TestParseComplaintMarkdown(t *testing.T) {
	raw := `# Proposition

The claimant will argue that the proposition is substantially true.
`
	got, err := ParseComplaintMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseComplaintMarkdown error = %v", err)
	}
	if got.Proposition == "" {
		t.Fatalf("unexpected parsed complaint: %#v", got)
	}
}

func TestParseComplaintMarkdownAcceptsPlainText(t *testing.T) {
	raw := "The claimant will argue that the proposition is substantially true.\n"
	got, err := ParseComplaintMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseComplaintMarkdown error = %v", err)
	}
	if got.Proposition != "The claimant will argue that the proposition is substantially true." {
		t.Fatalf("Proposition = %q", got.Proposition)
	}
}

func TestParseComplaintMarkdownRejectsEmptyInput(t *testing.T) {
	_, err := ParseComplaintMarkdown(" \n\t\n")
	if err == nil {
		t.Fatal("ParseComplaintMarkdown returned nil error, want failure")
	}
}

func TestParseComplaintMarkdownRejectsEmptyPropositionSection(t *testing.T) {
	_, err := ParseComplaintMarkdown("# Proposition\n\n")
	if err == nil {
		t.Fatal("ParseComplaintMarkdown returned nil error, want failure")
	}
}

func TestComplaintMarkdownRoundTrip(t *testing.T) {
	in := Complaint{
		Proposition: "P",
	}
	got, err := ParseComplaintMarkdown(ComplaintMarkdown(in))
	if err != nil {
		t.Fatalf("round trip parse error = %v", err)
	}
	if got != in {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, in)
	}
}
