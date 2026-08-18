package recordio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteJSONAtomicReplacesRecord(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "run.json")
	if err := WriteJSONAtomic(path, map[string]int{"value": 1}); err != nil {
		t.Fatalf("write first record: %v", err)
	}
	if err := WriteJSONAtomic(path, map[string]int{"value": 2}); err != nil {
		t.Fatalf("replace record: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if got["value"] != 2 {
		t.Fatalf("value = %d, want 2", got["value"])
	}
	if names, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".run.json.*.tmp")); err != nil || len(names) != 0 {
		t.Fatalf("temporary files = %v, error = %v", names, err)
	}
}

func TestAppendJSONLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "events.ndjson")
	if err := AppendJSONLine(path, map[string]int{"sequence": 1}); err != nil {
		t.Fatalf("append first record: %v", err)
	}
	if err := AppendJSONLine(path, map[string]int{"sequence": 2}); err != nil {
		t.Fatalf("append second record: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read records: %v", err)
	}
	if lines := strings.Split(strings.TrimSpace(string(raw)), "\n"); len(lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(lines))
	}
}
