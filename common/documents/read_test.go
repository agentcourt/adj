package documents

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadVerified(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("verified document\n")
	path := filepath.Join(root, "nested", "document.txt")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	document := manifestFile("nested/document.txt", want)
	got, err := ReadVerified(root, document)
	if err != nil {
		t.Fatalf("ReadVerified error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("ReadVerified bytes = %q, want %q", got, want)
	}
}

func TestReadVerifiedRejectsSizeAndDigestDrift(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		recorded []byte
		stored   []byte
	}{
		{name: "size", recorded: []byte("short"), stored: []byte("longer")},
		{name: "digest", recorded: []byte("first"), stored: []byte("other")},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "document"), test.stored, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := ReadVerified(root, manifestFile("document", test.recorded))
			if err == nil {
				t.Fatal("ReadVerified accepted changed content")
			}
		})
	}
}

func TestReadVerifiedRejectsSymlinksAndEscapes(t *testing.T) {
	t.Parallel()

	t.Run("file symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(t.TempDir(), "target")
		raw := []byte("target")
		if err := os.WriteFile(target, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "document")); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadVerified(root, manifestFile("document", raw)); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("ReadVerified error = %v", err)
		}
	})

	t.Run("directory symlink", func(t *testing.T) {
		root := t.TempDir()
		target := t.TempDir()
		raw := []byte("target")
		if err := os.WriteFile(filepath.Join(target, "document"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadVerified(root, manifestFile("linked/document", raw)); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("ReadVerified error = %v", err)
		}
	})

	t.Run("root symlink", func(t *testing.T) {
		target := t.TempDir()
		raw := []byte("target")
		if err := os.WriteFile(filepath.Join(target, "document"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(t.TempDir(), "linked-root")
		if err := os.Symlink(target, root); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadVerified(root, manifestFile("document", raw)); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("ReadVerified error = %v", err)
		}
	})

	t.Run("path escape", func(t *testing.T) {
		raw := []byte("outside")
		if _, err := ReadVerified(t.TempDir(), manifestFile("../outside", raw)); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Fatalf("ReadVerified error = %v", err)
		}
	})
}

func manifestFile(path string, raw []byte) File {
	hash := sha256.Sum256(raw)
	return File{
		Path:      path,
		SizeBytes: int64(len(raw)),
		SHA256:    hex.EncodeToString(hash[:]),
	}
}
