package mcpcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ReadyPublication struct {
	path    string
	enabled bool
	owner   os.FileInfo
}

type ReadyRecord struct {
	Address string `json:"address"`
}

func NewReadyPublication(path string) (*ReadyPublication, error) {
	path = strings.TrimSpace(path)
	publication := &ReadyPublication{path: path, enabled: path != ""}
	if !publication.enabled {
		return publication, nil
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("ready file %s already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect ready file %s: %w", path, err)
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return nil, fmt.Errorf("inspect ready-file directory %s: %w", parent, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("ready-file parent %s is not a directory", parent)
	}
	return publication, nil
}

func (p *ReadyPublication) Publish(address string) (returnErr error) {
	if p == nil || !p.enabled {
		return nil
	}
	if p.owner != nil {
		return fmt.Errorf("ready file %s was already published", p.path)
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("ready-file address is empty")
	}
	raw, err := json.Marshal(ReadyRecord{Address: address})
	if err != nil {
		return fmt.Errorf("encode ready file: %w", err)
	}
	raw = append(raw, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(p.path), "."+filepath.Base(p.path)+".tmp-")
	if err != nil {
		return fmt.Errorf("create ready-file temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			returnErr = errors.Join(returnErr, temporary.Close())
		}
		if temporaryPath != "" {
			removeErr := os.Remove(temporaryPath)
			if !errors.Is(removeErr, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, removeErr)
			}
		}
		if returnErr != nil && p.owner != nil {
			returnErr = errors.Join(returnErr, p.Remove())
		}
	}()
	written, err := temporary.Write(raw)
	if written != len(raw) {
		err = errors.Join(err, io.ErrShortWrite)
	}
	if err != nil {
		return fmt.Errorf("write ready-file temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync ready-file temporary file: %w", err)
	}
	owner, err := temporary.Stat()
	if err != nil {
		return fmt.Errorf("inspect ready-file temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close ready-file temporary file: %w", err)
	}
	closed = true
	if err := os.Link(temporaryPath, p.path); err != nil {
		return fmt.Errorf("publish ready file %s without replacement: %w", p.path, err)
	}
	p.owner = owner
	current, err := os.Lstat(p.path)
	if err != nil {
		return fmt.Errorf("inspect published ready file %s: %w", p.path, err)
	}
	if !os.SameFile(owner, current) {
		return fmt.Errorf("published ready file %s has unexpected identity", p.path)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove ready-file temporary link: %w", err)
	}
	temporaryPath = ""
	return nil
}

func (p *ReadyPublication) Remove() error {
	if p == nil || !p.enabled || p.owner == nil {
		return nil
	}
	current, err := os.Lstat(p.path)
	if errors.Is(err, os.ErrNotExist) {
		p.owner = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect ready file %s before removal: %w", p.path, err)
	}
	if !os.SameFile(p.owner, current) {
		return fmt.Errorf("ready file %s ownership changed; leaving it in place", p.path)
	}
	if err := os.Remove(p.path); err != nil {
		return fmt.Errorf("remove ready file %s: %w", p.path, err)
	}
	p.owner = nil
	return nil
}
