package cli

import "testing"

func TestDefaultComplaintOutputPath(t *testing.T) {
	t.Parallel()

	got := defaultComplaintOutputPath("/tmp/example/situation.md")
	want := "/tmp/example/complaint.md"
	if got != want {
		t.Fatalf("defaultComplaintOutputPath() = %q, want %q", got, want)
	}
}
