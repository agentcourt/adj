package documents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = "adj.documents.v1"

type Limits struct {
	MaxFiles      int   `json:"max_files"`
	MaxFileBytes  int64 `json:"max_file_bytes"`
	MaxTotalBytes int64 `json:"max_total_bytes"`
}

type File struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type,omitempty"`
}

type Manifest struct {
	SchemaVersion string `json:"schema_version"`
	TotalBytes    int64  `json:"total_bytes"`
	Files         []File `json:"files"`
}

func Import(root, destination string, limits Limits) (manifest Manifest, err error) {
	if limits.MaxFiles <= 0 {
		return Manifest{}, fmt.Errorf("max files must be positive")
	}
	if limits.MaxFileBytes <= 0 {
		return Manifest{}, fmt.Errorf("max file bytes must be positive")
	}
	if limits.MaxTotalBytes <= 0 {
		return Manifest{}, fmt.Errorf("max total bytes must be positive")
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return Manifest{}, fmt.Errorf("document root is required")
	}
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return Manifest{}, fmt.Errorf("document destination is required")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("stat document root %s: %w", root, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, fmt.Errorf("document root %s is a symbolic link", root)
	}
	if !rootInfo.IsDir() {
		return Manifest{}, fmt.Errorf("document root %s is not a directory", root)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve document root %s: %w", root, err)
	}

	paths, err := collectPaths(absRoot, rootInfo, limits)
	if err != nil {
		return Manifest{}, err
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create document destination parent %s: %w", parent, err)
	}
	if _, err := os.Lstat(destination); err == nil {
		return Manifest{}, fmt.Errorf("document destination %s already exists", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Manifest{}, fmt.Errorf("stat document destination %s: %w", destination, err)
	}
	staging, err := os.MkdirTemp(parent, ".documents.*.tmp")
	if err != nil {
		return Manifest{}, fmt.Errorf("create document staging directory: %w", err)
	}
	renamed := false
	defer func() {
		if !renamed {
			if removeErr := os.RemoveAll(staging); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("remove document staging directory %s: %w", staging, removeErr))
			}
		}
	}()

	manifest = Manifest{SchemaVersion: SchemaVersion, Files: make([]File, 0, len(paths))}
	for _, path := range paths {
		file, copyErr := copyFile(absRoot, staging, path.relative, path.info, limits)
		if copyErr != nil {
			return Manifest{}, copyErr
		}
		if manifest.TotalBytes > limits.MaxTotalBytes-file.SizeBytes {
			return Manifest{}, fmt.Errorf("documents exceed total byte limit %d", limits.MaxTotalBytes)
		}
		manifest.TotalBytes += file.SizeBytes
		manifest.Files = append(manifest.Files, file)
	}
	if err := os.Rename(staging, destination); err != nil {
		return Manifest{}, fmt.Errorf("install documents at %s: %w", destination, err)
	}
	renamed = true
	return manifest, nil
}

type sourcePath struct {
	relative string
	info     os.FileInfo
}

func collectPaths(root string, originalRoot os.FileInfo, limits Limits) ([]sourcePath, error) {
	var paths []sourcePath
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk document path %s: %w", path, walkErr)
		}
		if path == root {
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("stat document root %s during import: %w", root, err)
			}
			if entry.Type()&os.ModeSymlink != 0 || !info.IsDir() || !os.SameFile(originalRoot, info) {
				return fmt.Errorf("document root %s changed during import", root)
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("document path %s is a symbolic link", path)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat document path %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("document path %s is not a regular file", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve document path %s: %w", path, err)
		}
		relative = filepath.ToSlash(relative)
		if err := validateRelativePath(relative); err != nil {
			return fmt.Errorf("invalid document path %s: %w", path, err)
		}
		if info.Size() > limits.MaxFileBytes {
			return fmt.Errorf("document %s has %d bytes, exceeding file byte limit %d", relative, info.Size(), limits.MaxFileBytes)
		}
		if total > limits.MaxTotalBytes-info.Size() {
			return fmt.Errorf("documents exceed total byte limit %d", limits.MaxTotalBytes)
		}
		total += info.Size()
		if len(paths) >= limits.MaxFiles {
			return fmt.Errorf("documents exceed file count limit %d", limits.MaxFiles)
		}
		paths = append(paths, sourcePath{relative: relative, info: info})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(paths, func(i, j int) bool {
		return bytes.Compare([]byte(paths[i].relative), []byte(paths[j].relative)) < 0
	})
	return paths, nil
}

func copyFile(root, staging, relative string, original os.FileInfo, limits Limits) (file File, err error) {
	source := filepath.Join(root, filepath.FromSlash(relative))
	if err := ensureWithinRoot(root, source); err != nil {
		return File{}, err
	}
	beforeOpen, err := os.Lstat(source)
	if err != nil {
		return File{}, fmt.Errorf("stat document %s before open: %w", relative, err)
	}
	if beforeOpen.Mode()&os.ModeSymlink != 0 || !beforeOpen.Mode().IsRegular() {
		return File{}, fmt.Errorf("document %s changed to a symbolic link or non-regular file", relative)
	}
	if !os.SameFile(original, beforeOpen) {
		return File{}, fmt.Errorf("document %s was replaced during import", relative)
	}
	in, err := os.Open(source)
	if err != nil {
		return File{}, fmt.Errorf("open document %s: %w", relative, err)
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close document %s: %w", relative, closeErr))
		}
	}()
	info, err := in.Stat()
	if err != nil {
		return File{}, fmt.Errorf("stat open document %s: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return File{}, fmt.Errorf("document %s changed to a non-regular file", relative)
	}
	if !os.SameFile(original, info) {
		return File{}, fmt.Errorf("document %s was replaced while opening", relative)
	}
	if info.Size() != original.Size() {
		return File{}, fmt.Errorf("document %s changed size during import", relative)
	}

	destination := filepath.Join(staging, filepath.FromSlash(relative))
	if err := ensureWithinRoot(staging, destination); err != nil {
		return File{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return File{}, fmt.Errorf("create document directory for %s: %w", relative, err)
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return File{}, fmt.Errorf("create imported document %s: %w", relative, err)
	}
	closed := false
	defer func() {
		if !closed {
			if closeErr := out.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close imported document %s: %w", relative, closeErr))
			}
		}
	}()

	hash := sha256.New()
	sniff := make([]byte, 0, 512)
	written, err := io.Copy(io.MultiWriter(out, hash, writerFunc(func(p []byte) (int, error) {
		remaining := 512 - len(sniff)
		if remaining > len(p) {
			remaining = len(p)
		}
		if remaining > 0 {
			sniff = append(sniff, p[:remaining]...)
		}
		return len(p), nil
	})), io.LimitReader(in, limits.MaxFileBytes+1))
	if err != nil {
		return File{}, fmt.Errorf("copy document %s: %w", relative, err)
	}
	if written != original.Size() {
		return File{}, fmt.Errorf("document %s changed during import: copied %d bytes, expected %d", relative, written, original.Size())
	}
	if written > limits.MaxFileBytes {
		return File{}, fmt.Errorf("document %s exceeds file byte limit %d", relative, limits.MaxFileBytes)
	}
	if err := out.Sync(); err != nil {
		return File{}, fmt.Errorf("sync imported document %s: %w", relative, err)
	}
	if err := out.Close(); err != nil {
		closed = true
		return File{}, fmt.Errorf("close imported document %s: %w", relative, err)
	}
	closed = true
	afterCopy, err := os.Lstat(source)
	if err != nil {
		return File{}, fmt.Errorf("stat document %s after copy: %w", relative, err)
	}
	if afterCopy.Mode()&os.ModeSymlink != 0 || !afterCopy.Mode().IsRegular() || !os.SameFile(original, afterCopy) || afterCopy.Size() != original.Size() || !afterCopy.ModTime().Equal(original.ModTime()) {
		return File{}, fmt.Errorf("document %s changed during import", relative)
	}

	return File{
		Path:      relative,
		SizeBytes: written,
		SHA256:    hex.EncodeToString(hash.Sum(nil)),
		MediaType: detectMediaType(relative, sniff),
	}, nil
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func detectMediaType(path string, firstBytes []byte) string {
	sniffed := strings.TrimSpace(strings.Split(http.DetectContentType(firstBytes), ";")[0])
	extension := strings.TrimSpace(strings.Split(mime.TypeByExtension(filepath.Ext(path)), ";")[0])
	if sniffed == "text/plain" && extension != "" {
		return extension
	}
	if sniffed == "application/octet-stream" && extension != "" {
		return extension
	}
	return sniffed
}

func validateRelativePath(path string) error {
	if path == "" || path == "." || strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must be relative")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean != path || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path escapes document root")
	}
	return nil
}

func ensureWithinRoot(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve path %s within %s: %w", path, root, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path %s escapes root %s", path, root)
	}
	return nil
}
