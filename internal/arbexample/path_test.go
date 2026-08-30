package arbexample

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComplaintPathUsesEffectiveAARWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name       string
		workingDir string
		want       string
	}{
		{
			name:       "arb procedure directory",
			workingDir: filepath.Join(root, "arb", "."),
			want:       filepath.Join(root, "examples", "ex01", "complaint.md"),
		},
		{
			name:       "other working directory",
			workingDir: filepath.Join(root, "runtime"),
			want:       filepath.Join(root, "runtime", "examples", "ex01", "complaint.md"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ComplaintPath(test.workingDir, "ex01")
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("path = %q, want %q", got, test.want)
			}
		})
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	arbDir := filepath.Join(root, "arb")
	if err := os.MkdirAll(arbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(arbDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	got, err := ComplaintPath("", "ex01")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "examples", "ex01", "complaint.md")
	if got != want {
		t.Fatalf("path from process cwd = %q, want %q", got, want)
	}
}
