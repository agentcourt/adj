package caserecord

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishCopiesCatalogAndWritesPortableIndex(t *testing.T) {
	root := t.TempDir()
	recordDir := filepath.Join(root, "record")
	writeTestManifest(t, recordDir, "simple")
	writeTestFile(t, filepath.Join(recordDir, "documents", "record.txt"), "evidence")
	writeTestJSON(t, filepath.Join(recordDir, "documents.json"), map[string]any{
		"files": []any{map[string]any{"path": "record.txt", "media_type": "text/plain"}},
	})
	sessionDir := t.TempDir()
	writeTestFile(t, filepath.Join(sessionDir, "session.jsonl"), "session")
	writeTestJSON(t, filepath.Join(recordDir, "run.json"), map[string]any{
		"management": map[string]any{"participants": []any{map[string]any{"role": "analyst", "state_dir": sessionDir}}},
	})

	record, err := Build(Options{Dir: recordDir, IncludeSessions: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if record.Sources[0].PathBase != SourcePathBaseAbsolute {
		t.Fatalf("direct source path base = %q", record.Sources[0].PathBase)
	}
	target := filepath.Join(root, "published")
	if err := Publish(record, target); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(target, PublishedIndexName))
	if err != nil {
		t.Fatalf("read published index: %v", err)
	}
	var published Record
	if err := json.Unmarshal(raw, &published); err != nil {
		t.Fatalf("decode published index: %v", err)
	}
	if len(published.Sources) != 2 || published.Sources[0].Path != "files/record" || published.Sources[1].Path != "files/session-1" || published.Sources[0].PathBase != SourcePathBaseIndex || published.Sources[1].PathBase != SourcePathBaseIndex {
		t.Fatalf("published sources = %#v", published.Sources)
	}
	sourceRoots := make(map[string]string, len(record.Sources))
	for _, source := range record.Sources {
		sourceRoots[source.ID] = source.Path
	}
	for _, artifact := range record.Artifacts {
		want, err := os.ReadFile(filepath.Join(sourceRoots[artifact.SourceID], filepath.FromSlash(artifact.Path)))
		if err != nil {
			t.Fatalf("read source artifact %s: %v", artifact.Path, err)
		}
		got, err := os.ReadFile(filepath.Join(target, "files", artifact.SourceID, filepath.FromSlash(artifact.Path)))
		if err != nil {
			t.Fatalf("read published artifact %s: %v", artifact.Path, err)
		}
		if string(got) != string(want) {
			t.Fatalf("published artifact %s differs", artifact.Path)
		}
	}
	if err := Publish(record, target); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing target error = %v", err)
	}
}

func TestPublishRejectsTargetInsideSourceAndChangedArtifact(t *testing.T) {
	root := t.TempDir()
	recordDir := filepath.Join(root, "record")
	writeTestManifest(t, recordDir, "simple")
	artifactPath := filepath.Join(recordDir, "document.txt")
	writeTestFile(t, artifactPath, "first")
	record, err := Build(Options{Dir: recordDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	inside := filepath.Join(recordDir, "published")
	if err := Publish(record, inside); err == nil || !strings.Contains(err.Error(), "outside source record") {
		t.Fatalf("inside target error = %v", err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("inside target was created: %v", err)
	}

	writeTestFile(t, artifactPath, "changed contents")
	target := filepath.Join(root, "changed-source")
	if err := Publish(record, target); err == nil || !strings.Contains(err.Error(), "changed after") {
		t.Fatalf("changed artifact error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, PublishedIndexName)); !os.IsNotExist(err) {
		t.Fatalf("index exists after failed copy: %v", err)
	}
}
