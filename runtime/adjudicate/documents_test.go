package adjudicate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestImportDocumentsRecursesSortsAndHashesStagedBytes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"z.txt":           "last",
		".hidden":         "hidden",
		"nested/a.json":   `{"a":1}`,
		"nested/.private": "private",
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := filepath.Join(t.TempDir(), "inputs")
	if err := os.Mkdir(inputs, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest, err := ImportDocuments(root, inputs, DocumentLimits{Count: 10, PerFile: 100, Total: 1_000})
	if err != nil {
		t.Fatal(err)
	}
	var gotPaths []string
	for _, document := range manifest.Documents {
		gotPaths = append(gotPaths, document.Path)
		contents := []byte(files[document.Path])
		hash := sha256.Sum256(contents)
		if document.SHA256 != hex.EncodeToString(hash[:]) {
			t.Fatalf("hash for %q = %q", document.Path, document.SHA256)
		}
		staged, err := os.ReadFile(filepath.Join(inputs, "documents", filepath.FromSlash(document.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(staged, contents) {
			t.Fatalf("staged %q = %q", document.Path, staged)
		}
	}
	wantPaths := []string{".hidden", "nested/.private", "nested/a.json", "z.txt"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", gotPaths, wantPaths)
	}
}

func TestImportDocumentsRejectsSymlinksAndSpecialFiles(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		inputs := newInputsDir(t)
		_, err := ImportDocuments(root, inputs, DocumentLimits{Count: 10, PerFile: 100, Total: 1_000})
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("ImportDocuments() error = %v", err)
		}
	})

	t.Run("named pipe", func(t *testing.T) {
		root := t.TempDir()
		pipe := filepath.Join(root, "pipe")
		if err := makeNamedPipe(pipe); err != nil {
			t.Skipf("named pipes unavailable: %v", err)
		}
		inputs := newInputsDir(t)
		_, err := ImportDocuments(root, inputs, DocumentLimits{Count: 10, PerFile: 100, Total: 1_000})
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("ImportDocuments() error = %v", err)
		}
	})
}

func TestImportDocumentsEnforcesLimitsWhileCopying(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]string
		limits DocumentLimits
		want   string
	}{
		{
			name:   "count",
			files:  map[string]string{"a": "a", "b": "b"},
			limits: DocumentLimits{Count: 1, PerFile: 10, Total: 10},
			want:   "document count exceeds limit",
		},
		{
			name:   "per file",
			files:  map[string]string{"a": "1234"},
			limits: DocumentLimits{Count: 1, PerFile: 3, Total: 10},
			want:   "exceeds per-file limit",
		},
		{
			name:   "total",
			files:  map[string]string{"a": "123", "b": "456"},
			limits: DocumentLimits{Count: 2, PerFile: 4, Total: 5},
			want:   "exceeds remaining total limit",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for name, contents := range test.files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := ImportDocuments(root, newInputsDir(t), test.limits)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ImportDocuments() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestImportDocumentsRejectsInvalidLimitsAndPathEscapes(t *testing.T) {
	inputs := newInputsDir(t)
	_, err := ImportDocuments("", inputs, DocumentLimits{})
	if err == nil || !strings.Contains(err.Error(), "document limits must be positive") {
		t.Fatalf("ImportDocuments() error = %v", err)
	}
	for _, path := range []string{"", ".", "..", "../outside", "/absolute"} {
		if confinedRelativePath(path) {
			t.Fatalf("confinedRelativePath(%q) = true", path)
		}
	}
}

func TestImportDocumentsAllowsEmptyDirectory(t *testing.T) {
	manifest, err := ImportDocuments(t.TempDir(), newInputsDir(t), DocumentLimits{Count: 1, PerFile: 1, Total: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Documents) != 0 || manifest.TotalBytes != 0 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func newInputsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "inputs")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
