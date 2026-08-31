package caserecord

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const PublishedIndexName = "case-record.json"

func Publish(record Record, target string) error {
	sources, err := validatePublishRecord(record)
	if err != nil {
		return err
	}
	target, err = preparePublishTarget(record, target)
	if err != nil {
		return err
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		return fmt.Errorf("create publication directory %s: %w", target, err)
	}
	if err := os.Mkdir(filepath.Join(target, "files"), 0o755); err != nil {
		return fmt.Errorf("create publication files directory: %w", err)
	}
	for _, artifact := range record.Artifacts {
		source := sources[artifact.SourceID]
		from := filepath.Join(source.Path, filepath.FromSlash(artifact.Path))
		to := filepath.Join(target, "files", source.ID, filepath.FromSlash(artifact.Path))
		if err := copyArtifact(from, to, artifact); err != nil {
			return err
		}
	}

	published := record
	published.Sources = append([]Source(nil), record.Sources...)
	for index := range published.Sources {
		published.Sources[index].Path = filepath.ToSlash(filepath.Join("files", published.Sources[index].ID))
		published.Sources[index].PathBase = SourcePathBaseIndex
	}
	if err := writePublishedIndex(filepath.Join(target, PublishedIndexName), published); err != nil {
		return err
	}
	return nil
}

func preparePublishTarget(record Record, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("publication directory is required")
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve publication directory %s: %w", target, err)
	}
	if _, err := os.Lstat(absTarget); err == nil {
		return "", fmt.Errorf("publication directory %s already exists", absTarget)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect publication directory %s: %w", absTarget, err)
	}
	parent := filepath.Dir(absTarget)
	parentInfo, err := os.Lstat(parent)
	if err != nil {
		return "", fmt.Errorf("inspect publication parent %s: %w", parent, err)
	}
	if parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return "", fmt.Errorf("publication parent %s is not a regular directory", parent)
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("resolve publication parent %s: %w", parent, err)
	}
	resolvedTarget := filepath.Join(resolvedParent, filepath.Base(absTarget))
	for _, source := range record.Sources {
		sourcePath, err := filepath.Abs(source.Path)
		if err != nil {
			return "", fmt.Errorf("resolve source %s: %w", source.ID, err)
		}
		if pathInside(sourcePath, absTarget) {
			return "", fmt.Errorf("publication directory must be outside source %s", source.ID)
		}
		if !source.Available {
			continue
		}
		resolvedSource, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			return "", fmt.Errorf("resolve source %s: %w", source.ID, err)
		}
		if pathInside(resolvedSource, resolvedTarget) {
			return "", fmt.Errorf("publication directory must be outside source %s", source.ID)
		}
	}
	return resolvedTarget, nil
}

func validatePublishRecord(record Record) (map[string]Source, error) {
	sources := make(map[string]Source, len(record.Sources))
	for _, source := range record.Sources {
		if source.ID == "" || !filepath.IsLocal(source.ID) || filepath.Base(source.ID) != source.ID {
			return nil, fmt.Errorf("invalid source ID %q", source.ID)
		}
		if source.PathBase != SourcePathBaseAbsolute || !filepath.IsAbs(source.Path) {
			return nil, fmt.Errorf("source %s does not have an absolute source path", source.ID)
		}
		if _, exists := sources[source.ID]; exists {
			return nil, fmt.Errorf("duplicate source ID %q", source.ID)
		}
		sources[source.ID] = source
	}
	for _, artifact := range record.Artifacts {
		source, exists := sources[artifact.SourceID]
		if !exists {
			return nil, fmt.Errorf("artifact %s refers to unknown source %q", artifact.Path, artifact.SourceID)
		}
		if !source.Available {
			return nil, fmt.Errorf("artifact %s refers to unavailable source %s", artifact.Path, artifact.SourceID)
		}
		path := filepath.FromSlash(artifact.Path)
		if artifact.Path == "" || !filepath.IsLocal(path) || path == "." {
			return nil, fmt.Errorf("artifact has invalid path %q", artifact.Path)
		}
	}
	return sources, nil
}

func copyArtifact(from, to string, artifact Artifact) (returnErr error) {
	before, err := os.Lstat(from)
	if err != nil {
		return fmt.Errorf("inspect artifact %s: %w", from, err)
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("artifact %s is not a regular file", from)
	}
	input, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("open artifact %s: %w", from, err)
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close artifact %s: %w", from, closeErr))
		}
	}()
	opened, err := input.Stat()
	if err != nil {
		return fmt.Errorf("inspect open artifact %s: %w", from, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return fmt.Errorf("artifact %s changed while publication started", from)
	}
	if opened.Size() != artifact.SizeBytes || !opened.ModTime().UTC().Equal(artifact.Timestamp) {
		return fmt.Errorf("artifact %s changed after the case record was read", from)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("create artifact directory for %s: %w", to, err)
	}
	output, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, opened.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create published artifact %s: %w", to, err)
	}
	defer func() {
		if closeErr := output.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close published artifact %s: %w", to, closeErr))
		}
	}()
	written, err := io.Copy(output, input)
	if err != nil {
		return fmt.Errorf("copy artifact %s: %w", from, err)
	}
	if written != artifact.SizeBytes {
		return fmt.Errorf("copy artifact %s: wrote %d bytes, expected %d", from, written, artifact.SizeBytes)
	}
	if err := os.Chtimes(to, artifact.Timestamp, artifact.Timestamp); err != nil {
		return fmt.Errorf("set published artifact time %s: %w", to, err)
	}
	return nil
}

func writePublishedIndex(path string, record Record) (returnErr error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create published case record %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close published case record %s: %w", path, closeErr))
		}
	}()
	return WriteJSON(file, record)
}

func pathInside(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
