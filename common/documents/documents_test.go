package documents

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestImportPreservesBytesHierarchyAndOrder(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	root := filepath.Join(base, "source")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"z.txt":         []byte("last\n"),
		"nested/a.json": []byte("{\"first\":true}\n"),
		".hidden":       {0, 1, 2, 3},
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(base, "record", "documents")
	manifest, err := Import(root, destination, Limits{MaxFiles: 10, MaxFileBytes: 100, MaxTotalBytes: 300})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	wantOrder := []string{".hidden", "nested/a.json", "z.txt"}
	if len(manifest.Files) != len(wantOrder) {
		t.Fatalf("file count = %d, want %d", len(manifest.Files), len(wantOrder))
	}
	var total int64
	for index, wantPath := range wantOrder {
		got := manifest.Files[index]
		if got.Path != wantPath {
			t.Fatalf("file %d path = %q, want %q", index, got.Path, wantPath)
		}
		wantBytes := files[wantPath]
		copied, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(wantPath)))
		if err != nil {
			t.Fatalf("read copied %s: %v", wantPath, err)
		}
		if string(copied) != string(wantBytes) {
			t.Fatalf("copied %s bytes differ", wantPath)
		}
		hash := sha256.Sum256(wantBytes)
		if got.SHA256 != hex.EncodeToString(hash[:]) {
			t.Fatalf("%s hash = %q", wantPath, got.SHA256)
		}
		total += int64(len(wantBytes))
	}
	if manifest.TotalBytes != total {
		t.Fatalf("total bytes = %d, want %d", manifest.TotalBytes, total)
	}
}

func TestImportRejectsInvalidEntriesAndLimits(t *testing.T) {
	t.Parallel()

	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(t.TempDir(), "target")
		if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		_, err := Import(root, filepath.Join(t.TempDir(), "documents"), Limits{MaxFiles: 10, MaxFileBytes: 100, MaxTotalBytes: 100})
		if err == nil {
			t.Fatal("Import accepted symlink")
		}
	})

	t.Run("file limit", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "large"), []byte("12345"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Import(root, filepath.Join(t.TempDir(), "documents"), Limits{MaxFiles: 10, MaxFileBytes: 4, MaxTotalBytes: 10})
		if err == nil {
			t.Fatal("Import accepted oversized file")
		}
	})

	t.Run("total limit", func(t *testing.T) {
		root := t.TempDir()
		for _, name := range []string{"a", "b"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("123"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := Import(root, filepath.Join(t.TempDir(), "documents"), Limits{MaxFiles: 10, MaxFileBytes: 4, MaxTotalBytes: 5})
		if err == nil {
			t.Fatal("Import accepted oversized total")
		}
	})

	t.Run("file count", func(t *testing.T) {
		root := t.TempDir()
		for _, name := range []string{"a", "b"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := Import(root, filepath.Join(t.TempDir(), "documents"), Limits{MaxFiles: 1, MaxFileBytes: 10, MaxTotalBytes: 10})
		if err == nil {
			t.Fatal("Import accepted too many files")
		}
	})

	t.Run("invalid limits", func(t *testing.T) {
		_, err := Import(t.TempDir(), filepath.Join(t.TempDir(), "documents"), Limits{})
		if err == nil {
			t.Fatal("Import accepted invalid limits")
		}
	})
}

func TestCopyFileRejectsReplacement(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "document.txt")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = copyFile(root, t.TempDir(), "document.txt", original, Limits{MaxFiles: 1, MaxFileBytes: 10, MaxTotalBytes: 10})
	if err == nil {
		t.Fatal("copyFile accepted a replacement file")
	}
}
