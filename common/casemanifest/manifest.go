package casemanifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

const (
	FileName        = "case-manifest.json"
	SchemaVersion   = "adj.case-manifest.v1"
	ProcedureADC    = "adc"
	ProcedureARB    = "arb"
	ProcedureARBD   = "arbd"
	ProcedureSimple = "simple"
	ProcedureQuick  = "quick"
)

type Manifest struct {
	SchemaVersion string    `json:"schema_version"`
	Procedure     string    `json:"procedure"`
	CaseID        string    `json:"case_id"`
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	CoreVersion   string    `json:"core_version"`
	CaseAPIBase   string    `json:"case_api_base,omitempty"`
}

func New(procedure, caseID, runID string, startedAt time.Time) Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Procedure:     strings.TrimSpace(procedure),
		CaseID:        strings.TrimSpace(caseID),
		RunID:         strings.TrimSpace(runID),
		StartedAt:     startedAt.UTC(),
		CoreVersion:   CoreVersion(),
	}
}

func CoreVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || strings.TrimSpace(info.Main.Version) == "" {
		return "(devel)"
	}
	return info.Main.Version
}

func WriteAtomic(dir string, manifest Manifest) (err error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("case manifest directory is required")
	}
	if err := validate(manifest); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create case manifest directory %s: %w", dir, err)
	}
	wire, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal case manifest: %w", err)
	}
	wire = append(wire, '\n')
	path := filepath.Join(dir, FileName)
	tmp, err := os.CreateTemp(dir, ".case-manifest.*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary case manifest: %w", err)
	}
	tmpName := tmp.Name()
	closed := false
	renamed := false
	defer func() {
		if !closed {
			err = errors.Join(err, closeFile(tmp, tmpName))
		}
		if !renamed {
			if removeErr := os.Remove(tmpName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove temporary case manifest %s: %w", tmpName, removeErr))
			}
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return fmt.Errorf("set temporary case manifest permissions: %w", err)
	}
	if _, err := tmp.Write(wire); err != nil {
		return fmt.Errorf("write temporary case manifest: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary case manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return fmt.Errorf("close temporary case manifest: %w", err)
	}
	closed = true
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace case manifest %s: %w", path, err)
	}
	renamed = true
	return nil
}

func validate(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("case manifest schema_version must be %q", SchemaVersion)
	}
	switch manifest.Procedure {
	case ProcedureADC, ProcedureARB, ProcedureARBD, ProcedureSimple, ProcedureQuick:
	default:
		return fmt.Errorf("unsupported case manifest procedure %q", manifest.Procedure)
	}
	if strings.TrimSpace(manifest.CaseID) == "" {
		return fmt.Errorf("case manifest case_id is required")
	}
	if strings.TrimSpace(manifest.RunID) == "" {
		return fmt.Errorf("case manifest run_id is required")
	}
	if manifest.StartedAt.IsZero() {
		return fmt.Errorf("case manifest started_at is required")
	}
	if strings.TrimSpace(manifest.CoreVersion) == "" {
		return fmt.Errorf("case manifest core_version is required")
	}
	return nil
}

func closeFile(file *os.File, name string) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary case manifest %s: %w", name, err)
	}
	return nil
}
