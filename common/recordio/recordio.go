package recordio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func WriteJSON(path string, value any) error {
	wire, err := marshalJSON(path, value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, wire, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func WriteJSONAtomic(path string, value any) (err error) {
	wire, err := marshalJSON(path, value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	closed := false
	renamed := false
	defer func() {
		if !closed {
			if closeErr := tmp.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close %s: %w", tmpName, closeErr))
			}
		}
		if !renamed {
			if removeErr := os.Remove(tmpName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove %s: %w", tmpName, removeErr))
			}
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return fmt.Errorf("set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(wire); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	closed = true
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	renamed = true
	return nil
}

func AppendJSONLine(path string, value any) (err error) {
	wire, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal record for %s: %w", path, err)
	}
	wire = append(wire, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
	}()
	if _, err := file.Write(wire); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func marshalJSON(path string, value any) ([]byte, error) {
	wire, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", path, err)
	}
	return append(wire, '\n'), nil
}
