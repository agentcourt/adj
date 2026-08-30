package adjudicate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Time      time.Time `json:"time"`
	Type      string    `json:"type"`
	Procedure Procedure `json:"procedure,omitempty"`
	CaseID    string    `json:"case_id,omitempty"`
	RunID     string    `json:"run_id,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}

type EventLog struct {
	mu     sync.Mutex
	path   string
	events []Event
}

func NewEventLog(path string) *EventLog {
	return &EventLog{path: path}
}

func (l *EventLog) Append(event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	next := append(append([]Event(nil), l.events...), event)
	if err := writeNDJSONAtomic(l.path, next); err != nil {
		return err
	}
	l.events = next
	return nil
}

func WriteJSONAtomic(path string, value any) error {
	return writeAtomic(path, 0o644, func(f *os.File) error {
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		return encoder.Encode(value)
	})
}

func writeNDJSONAtomic(path string, values []Event) error {
	return writeAtomic(path, 0o644, func(f *os.File) error {
		encoder := json.NewEncoder(f)
		encoder.SetEscapeHTML(false)
		for _, value := range values {
			if err := encoder.Encode(value); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeAtomic(path string, mode os.FileMode, write func(*os.File) error) (returnErr error) {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-")
	if err != nil {
		return fmt.Errorf("create temporary record for %q: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if returnErr != nil {
			if cleanupErr := os.Remove(temporaryPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				returnErr = joinOperationError(returnErr, cleanupErr, "remove temporary record")
			}
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		closeErr := temporary.Close()
		return joinOperationError(fmt.Errorf("set temporary record mode for %q: %w", path, err), closeErr, "close temporary record")
	}
	if err := write(temporary); err != nil {
		closeErr := temporary.Close()
		return joinOperationError(fmt.Errorf("write temporary record for %q: %w", path, err), closeErr, "close temporary record")
	}
	if err := temporary.Sync(); err != nil {
		closeErr := temporary.Close()
		return joinOperationError(fmt.Errorf("sync temporary record for %q: %w", path, err), closeErr, "close temporary record")
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary record for %q: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish record %q: %w", path, err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open record directory %q: %w", dir, err)
	}
	if err := directory.Sync(); err != nil {
		closeErr := directory.Close()
		return joinOperationError(fmt.Errorf("sync record directory %q: %w", dir, err), closeErr, "close record directory")
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close record directory %q: %w", dir, err)
	}
	return nil
}
