package quick

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigureCouncilPoolResolution(t *testing.T) {
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	commonRoot := filepath.Join(root, "common")
	sharedPool := filepath.Join(commonRoot, "data", "personas", "pool.jsonl")
	if err := os.MkdirAll(filepath.Dir(sharedPool), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sharedPool, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	caseDir := filepath.Join(root, "cases", "example")
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	localPool := filepath.Join(caseDir, "pool.jsonl")
	if err := os.WriteFile(localPool, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(caseDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalCWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	base := Options{
		Proposition:          "p",
		OutputDir:            filepath.Join(root, "out"),
		CommonRoot:           commonRoot,
		CouncilSize:          1,
		RequiredVotes:        1,
		EvidenceStandard:     "preponderance",
		LawyerAPIBearerToken: testLawyerAPIBearerToken,
		MaxDocumentFiles:     1,
		MaxDocumentFileBytes: 1,
		MaxDocumentsTotal:    1,
		AllowAPIKey:          true,
	}

	explicitPool := filepath.Join(root, "explicit.jsonl")
	base.CouncilPoolPath = explicitPool
	cfg, err := configure(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CouncilPoolPath != explicitPool {
		t.Fatalf("explicit council pool = %q, want %q", cfg.CouncilPoolPath, explicitPool)
	}

	base.CouncilPoolPath = ""
	cfg, err = configure(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CouncilPoolPath != localPool {
		t.Fatalf("local council pool = %q, want %q", cfg.CouncilPoolPath, localPool)
	}

	if err := os.Remove(localPool); err != nil {
		t.Fatal(err)
	}
	base.CommonRoot = ""
	cfg, err = configure(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CouncilPoolPath != sharedPool {
		t.Fatalf("shared council pool = %q, want %q", cfg.CouncilPoolPath, sharedPool)
	}
}
