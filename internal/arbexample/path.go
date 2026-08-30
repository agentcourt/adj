package arbexample

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ComplaintPath(workingDir string, example string) (string, error) {
	effectiveDir := strings.TrimSpace(workingDir)
	if effectiveDir == "" {
		var err error
		effectiveDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get AAR working directory: %w", err)
		}
	}
	effectiveDir, err := filepath.Abs(filepath.Clean(effectiveDir))
	if err != nil {
		return "", fmt.Errorf("resolve AAR working directory: %w", err)
	}
	examplesDir := filepath.Join(effectiveDir, "examples")
	if filepath.Base(effectiveDir) == "arb" {
		examplesDir = filepath.Join(filepath.Dir(effectiveDir), "examples")
	}
	return filepath.Join(examplesDir, example, "complaint.md"), nil
}
