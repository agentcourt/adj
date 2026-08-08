package spec

import "testing"

func TestParseComplaintMarkdown(t *testing.T) {
	raw := `# Question

What percentage of artwork Y is novel in view of artwork X?
`
	got, err := ParseComplaintMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseComplaintMarkdown error = %v", err)
	}
	if got.Question == "" {
		t.Fatalf("unexpected parsed complaint: %#v", got)
	}
}

func TestParseComplaintMarkdownAcceptsPlainText(t *testing.T) {
	raw := "What percentage of artwork Y is novel in view of artwork X?\n"
	got, err := ParseComplaintMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseComplaintMarkdown error = %v", err)
	}
	if got.Question != "What percentage of artwork Y is novel in view of artwork X?" {
		t.Fatalf("Question = %q", got.Question)
	}
}

func TestParseComplaintMarkdownRejectsEmptyInput(t *testing.T) {
	_, err := ParseComplaintMarkdown(" \n\t\n")
	if err == nil {
		t.Fatal("ParseComplaintMarkdown returned nil error, want failure")
	}
}

func TestParseComplaintMarkdownRejectsEmptyQuestionSection(t *testing.T) {
	_, err := ParseComplaintMarkdown("# Question\n\n")
	if err == nil {
		t.Fatal("ParseComplaintMarkdown returned nil error, want failure")
	}
}

func TestComplaintMarkdownRoundTrip(t *testing.T) {
	in := Complaint{
		Question: "Q",
	}
	got, err := ParseComplaintMarkdown(ComplaintMarkdown(in))
	if err != nil {
		t.Fatalf("round trip parse error = %v", err)
	}
	if got != in {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, in)
	}
}
