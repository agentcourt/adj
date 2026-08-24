package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultADCPathStopsAtModuleRoot(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "work", "adj")
	start := filepath.Join(moduleRoot, "adc", "runtime", "cli")
	writePathTestFile(t, filepath.Join(moduleRoot, "go.mod"))
	writePathTestFile(t, filepath.Join(root, "adc", ".bin", "adcengine"))
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll start: %v", err)
	}

	got := defaultADCPathFrom(start, ".bin", "adcengine")
	want := filepath.Join(".bin", "adcengine")
	if got != want {
		t.Fatalf("defaultADCPathFrom ancestor asset = %q, want %q", got, want)
	}
}

func TestDefaultADCPathFindsModuleAsset(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "adj")
	start := filepath.Join(moduleRoot, "adc", "runtime", "cli")
	asset := filepath.Join(moduleRoot, "adc", ".bin", "adcengine")
	writePathTestFile(t, filepath.Join(moduleRoot, "go.mod"))
	writePathTestFile(t, asset)
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll start: %v", err)
	}

	if got := defaultADCPathFrom(start, ".bin", "adcengine"); got != asset {
		t.Fatalf("defaultADCPathFrom = %q, want %q", got, asset)
	}
}

func TestDefaultADCPathWithoutModuleInspectsOnlyStart(t *testing.T) {
	root := newNoModuleTestDir(t)
	start := filepath.Join(root, "child")
	writePathTestFile(t, filepath.Join(root, "adc", ".bin", "adcengine"))
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll start: %v", err)
	}

	want := filepath.Join(".bin", "adcengine")
	if got := defaultADCPathFrom(start, ".bin", "adcengine"); got != want {
		t.Fatalf("defaultADCPathFrom parent asset = %q, want %q", got, want)
	}
	localAsset := filepath.Join(start, "adc", ".bin", "adcengine")
	writePathTestFile(t, localAsset)
	if got := defaultADCPathFrom(start, ".bin", "adcengine"); got != localAsset {
		t.Fatalf("defaultADCPathFrom local asset = %q, want %q", got, localAsset)
	}
}

func TestLocateCommonRootStopsAtModuleRoot(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "work", "adj")
	start := filepath.Join(moduleRoot, "adc", "runtime", "cli")
	writePathTestFile(t, filepath.Join(moduleRoot, "go.mod"))
	writePathTestFile(t, filepath.Join(root, "common", "data", "personas", "pool.jsonl"))
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll start: %v", err)
	}

	want := filepath.Clean(filepath.Join(start, "../common"))
	if got := locateCommonRootFrom(start); got != want {
		t.Fatalf("locateCommonRootFrom ancestor asset = %q, want %q", got, want)
	}
}

func TestLocateCommonRootFindsModuleAsset(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "adj")
	start := filepath.Join(moduleRoot, "adc", "runtime", "cli")
	commonRoot := filepath.Join(moduleRoot, "common")
	writePathTestFile(t, filepath.Join(moduleRoot, "go.mod"))
	writePathTestFile(t, filepath.Join(commonRoot, "data", "personas", "pool.jsonl"))
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll start: %v", err)
	}

	if got := locateCommonRootFrom(start); got != commonRoot {
		t.Fatalf("locateCommonRootFrom = %q, want %q", got, commonRoot)
	}
}

func TestLocateCommonRootWithoutModuleInspectsOnlyStart(t *testing.T) {
	root := newNoModuleTestDir(t)
	start := filepath.Join(root, "child")
	writePathTestFile(t, filepath.Join(root, "common", "data", "personas", "pool.jsonl"))
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll start: %v", err)
	}

	wantFallback := filepath.Clean(filepath.Join(start, "../common"))
	if got := locateCommonRootFrom(start); got != wantFallback {
		t.Fatalf("locateCommonRootFrom parent asset = %q, want %q", got, wantFallback)
	}
	localCommon := filepath.Join(start, "common")
	writePathTestFile(t, filepath.Join(localCommon, "data", "personas", "pool.jsonl"))
	if got := locateCommonRootFrom(start); got != localCommon {
		t.Fatalf("locateCommonRootFrom local asset = %q, want %q", got, localCommon)
	}
}

func writePathTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func newNoModuleTestDir(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "adj-cli-path-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("RemoveAll %s: %v", root, err)
		}
	})
	if moduleRoot := nearestGoModuleRoot(root); moduleRoot != "" {
		t.Fatalf("system temporary directory is inside Go module %s", moduleRoot)
	}
	return root
}
