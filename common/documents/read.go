package documents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ReadVerified(root string, document File) (raw []byte, err error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("document root is required")
	}
	if err := validateRelativePath(document.Path); err != nil {
		return nil, fmt.Errorf("invalid document path %q: %w", document.Path, err)
	}
	if document.SizeBytes < 0 {
		return nil, fmt.Errorf("document %q has negative recorded size %d", document.Path, document.SizeBytes)
	}
	wantHash, err := hex.DecodeString(strings.TrimSpace(document.SHA256))
	if err != nil || len(wantHash) != sha256.Size {
		return nil, fmt.Errorf("document %q has invalid recorded SHA-256", document.Path)
	}
	if int64(int(document.SizeBytes)) != document.SizeBytes {
		return nil, fmt.Errorf("document %q recorded size %d exceeds addressable memory", document.Path, document.SizeBytes)
	}

	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("stat document root %s: %w", root, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, fmt.Errorf("document root %s must be a directory and must not be a symbolic link", root)
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open document root %s: %w", root, err)
	}
	defer func() {
		if closeErr := rootHandle.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close document root %s: %w", root, closeErr))
		}
	}()
	openedRootInfo, err := rootHandle.Stat(".")
	if err != nil {
		return nil, fmt.Errorf("stat open document root %s: %w", root, err)
	}
	if !os.SameFile(rootInfo, openedRootInfo) {
		return nil, fmt.Errorf("document root %s changed while opening", root)
	}

	relative := filepath.ToSlash(document.Path)
	components := strings.Split(relative, "/")
	var finalInfo os.FileInfo
	for index := range components {
		path := strings.Join(components[:index+1], "/")
		info, err := rootHandle.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("stat imported document path %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("imported document path %s is a symbolic link", path)
		}
		if index < len(components)-1 {
			if !info.IsDir() {
				return nil, fmt.Errorf("imported document path %s is not a directory", path)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("imported document %s is not a regular file", relative)
		}
		finalInfo = info
	}

	file, err := rootHandle.Open(relative)
	if err != nil {
		return nil, fmt.Errorf("open imported document %s: %w", relative, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close imported document %s: %w", relative, closeErr))
		}
	}()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat open imported document %s: %w", relative, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(finalInfo, openedInfo) {
		return nil, fmt.Errorf("imported document %s changed while opening", relative)
	}
	if openedInfo.Size() != document.SizeBytes {
		return nil, fmt.Errorf("imported document %s has %d bytes, recorded size is %d", relative, openedInfo.Size(), document.SizeBytes)
	}

	raw = make([]byte, int(document.SizeBytes))
	read, readErr := io.ReadFull(file, raw)
	if readErr != nil {
		return nil, fmt.Errorf("read imported document %s: read %d of %d bytes: %w", relative, read, document.SizeBytes, readErr)
	}
	var extra [1]byte
	extraRead, extraErr := file.Read(extra[:])
	if extraRead != 0 {
		return nil, fmt.Errorf("imported document %s contains bytes beyond recorded size %d", relative, document.SizeBytes)
	}
	if extraErr != nil && !errors.Is(extraErr, io.EOF) {
		return nil, fmt.Errorf("check imported document %s length: %w", relative, extraErr)
	}

	afterInfo, err := rootHandle.Lstat(relative)
	if err != nil {
		return nil, fmt.Errorf("stat imported document %s after read: %w", relative, err)
	}
	if afterInfo.Mode()&os.ModeSymlink != 0 || !afterInfo.Mode().IsRegular() || !os.SameFile(openedInfo, afterInfo) || afterInfo.Size() != document.SizeBytes {
		return nil, fmt.Errorf("imported document %s changed during read", relative)
	}
	gotHash := sha256.Sum256(raw)
	if !bytes.Equal(gotHash[:], wantHash) {
		return nil, fmt.Errorf("imported document %s SHA-256 does not match its manifest", relative)
	}
	return raw, nil
}
