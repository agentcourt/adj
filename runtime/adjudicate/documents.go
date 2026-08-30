package adjudicate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DocumentManifestSchemaVersion = "adjudicate.documents.v1"

type DocumentManifest struct {
	SchemaVersion string     `json:"schema_version"`
	Root          string     `json:"root,omitempty"`
	Documents     []Document `json:"documents"`
	TotalBytes    int64      `json:"total_bytes"`
}

type Document struct {
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	MediaType string `json:"media_type,omitempty"`
	SHA256    string `json:"sha256"`
}

func ImportDocuments(sourceRoot, inputsDir string, limits DocumentLimits) (manifest DocumentManifest, returnErr error) {
	if limits.Count <= 0 || limits.PerFile <= 0 || limits.Total <= 0 {
		return DocumentManifest{}, fmt.Errorf("document limits must be positive")
	}
	if limits.PerFile > limits.Total {
		return DocumentManifest{}, fmt.Errorf("document per-file limit exceeds total limit")
	}
	manifest = DocumentManifest{
		SchemaVersion: DocumentManifestSchemaVersion,
		Documents:     []Document{},
	}
	destination := filepath.Join(inputsDir, "documents")
	if sourceRoot == "" {
		if err := os.Mkdir(destination, 0o755); err != nil {
			return DocumentManifest{}, fmt.Errorf("create empty document directory: %w", err)
		}
		return manifest, nil
	}

	absRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return DocumentManifest{}, fmt.Errorf("resolve document root %q: %w", sourceRoot, err)
	}
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return DocumentManifest{}, fmt.Errorf("inspect document root %q: %w", absRoot, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return DocumentManifest{}, fmt.Errorf("document root %q is a symbolic link", absRoot)
	}
	if !rootInfo.IsDir() {
		return DocumentManifest{}, fmt.Errorf("document root %q is not a directory", absRoot)
	}

	paths, err := collectDocumentPaths(absRoot, limits.Count)
	if err != nil {
		return DocumentManifest{}, err
	}
	temporary, err := os.MkdirTemp(inputsDir, ".documents-")
	if err != nil {
		return DocumentManifest{}, fmt.Errorf("create temporary document directory: %w", err)
	}
	keepTemporary := false
	defer func() {
		if !keepTemporary {
			if cleanupErr := os.RemoveAll(temporary); cleanupErr != nil {
				returnErr = joinOperationError(returnErr, cleanupErr, "remove temporary document directory")
			}
		}
	}()

	manifest.Root = absRoot
	for _, relative := range paths {
		document, err := copyDocument(absRoot, relative, temporary, limits.PerFile, limits.Total-manifest.TotalBytes)
		if err != nil {
			return DocumentManifest{}, err
		}
		manifest.Documents = append(manifest.Documents, document)
		manifest.TotalBytes += document.Bytes
	}
	if err := os.Rename(temporary, destination); err != nil {
		return DocumentManifest{}, fmt.Errorf("publish staged documents: %w", err)
	}
	keepTemporary = true
	return manifest, nil
}

func collectDocumentPaths(root string, countLimit int) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk documents at %q: %w", path, walkErr)
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve document path %q: %w", path, err)
		}
		if !confinedRelativePath(relative) {
			return fmt.Errorf("document path %q escapes root", path)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("document path %q is a symbolic link", relative)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect document path %q: %w", relative, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("document path %q is not a regular file", relative)
		}
		paths = append(paths, filepath.ToSlash(relative))
		if len(paths) > countLimit {
			return fmt.Errorf("document count exceeds limit of %d", countLimit)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func copyDocument(root, relative, destinationRoot string, perFileLimit, remainingTotal int64) (Document, error) {
	if remainingTotal < 0 {
		return Document{}, fmt.Errorf("document total exceeds limit")
	}
	sourcePath := filepath.Join(root, filepath.FromSlash(relative))
	if !pathWithin(root, sourcePath) {
		return Document{}, fmt.Errorf("document path %q escapes root", relative)
	}
	before, err := os.Lstat(sourcePath)
	if err != nil {
		return Document{}, fmt.Errorf("inspect document %q: %w", relative, err)
	}
	if !before.Mode().IsRegular() {
		return Document{}, fmt.Errorf("document %q changed before copying", relative)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return Document{}, fmt.Errorf("open document %q: %w", relative, err)
	}
	opened, err := source.Stat()
	if err != nil {
		closeErr := source.Close()
		return Document{}, joinOperationError(fmt.Errorf("inspect open document %q: %w", relative, err), closeErr, "close document")
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		closeErr := source.Close()
		return Document{}, joinOperationError(fmt.Errorf("document %q changed before copying", relative), closeErr, "close document")
	}

	destinationPath := filepath.Join(destinationRoot, filepath.FromSlash(relative))
	if !pathWithin(destinationRoot, destinationPath) {
		closeErr := source.Close()
		return Document{}, joinOperationError(fmt.Errorf("staged document path %q escapes destination", relative), closeErr, "close document")
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		closeErr := source.Close()
		return Document{}, joinOperationError(fmt.Errorf("create directory for document %q: %w", relative, err), closeErr, "close document")
	}
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		closeErr := source.Close()
		return Document{}, joinOperationError(fmt.Errorf("create staged document %q: %w", relative, err), closeErr, "close document")
	}

	limit := perFileLimit
	if remainingTotal < limit {
		limit = remainingTotal
	}
	hash := sha256.New()
	readLimit := limit
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	written, copyErr := io.Copy(io.MultiWriter(destination, hash), io.LimitReader(source, readLimit))
	destinationCloseErr := destination.Close()
	sourceCloseErr := source.Close()
	if copyErr != nil {
		return Document{}, errorsForCopy(relative, copyErr, destinationCloseErr, sourceCloseErr)
	}
	if destinationCloseErr != nil {
		return Document{}, joinOperationError(fmt.Errorf("close staged document %q: %w", relative, destinationCloseErr), sourceCloseErr, "close source document")
	}
	if sourceCloseErr != nil {
		return Document{}, fmt.Errorf("close source document %q: %w", relative, sourceCloseErr)
	}
	if written > perFileLimit {
		return Document{}, fmt.Errorf("document %q exceeds per-file limit of %d bytes", relative, perFileLimit)
	}
	if written > remainingTotal {
		return Document{}, fmt.Errorf("document %q exceeds remaining total limit of %d bytes", relative, remainingTotal)
	}
	after, err := os.Lstat(sourcePath)
	if err != nil {
		return Document{}, fmt.Errorf("inspect copied document %q: %w", relative, err)
	}
	if !os.SameFile(before, after) || after.Size() != written || after.ModTime() != before.ModTime() {
		return Document{}, fmt.Errorf("document %q changed while copying", relative)
	}
	return Document{
		Path:      relative,
		Bytes:     written,
		MediaType: mediaType(relative),
		SHA256:    hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func errorsForCopy(relative string, copyErr, destinationCloseErr, sourceCloseErr error) error {
	err := fmt.Errorf("copy document %q: %w", relative, copyErr)
	err = joinOperationError(err, destinationCloseErr, "close staged document")
	return joinOperationError(err, sourceCloseErr, "close source document")
}

func joinOperationError(operation, cleanup error, cleanupName string) error {
	if cleanup == nil {
		return operation
	}
	if operation == nil {
		return fmt.Errorf("%s: %w", cleanupName, cleanup)
	}
	return errors.Join(operation, fmt.Errorf("%s: %w", cleanupName, cleanup))
}

func mediaType(path string) string {
	return mime.TypeByExtension(filepath.Ext(path))
}

func confinedRelativePath(path string) bool {
	if path == "" || path == "." || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && confinedRelativePath(relative)
}
