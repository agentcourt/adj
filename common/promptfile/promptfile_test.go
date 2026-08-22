package promptfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndRender(t *testing.T) {
	root := t.TempDir()
	conventional := filepath.Join(root, "conventional.md")
	source, err := Load("test", "", conventional, "fallback {{VALUE}}")
	if err != nil {
		t.Fatal(err)
	}
	if rendered, err := Render("test", source, "{{VALUE}}", "one"); err != nil || rendered != "fallback one" {
		t.Fatalf("fallback render = %q, %v", rendered, err)
	}
	if err := os.WriteFile(conventional, []byte("conventional {{VALUE}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err = Load("test", "", conventional, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if rendered, err := Render("test", source, "{{VALUE}}", "two {{literal}}"); err != nil || rendered != "conventional two {{literal}}" {
		t.Fatalf("conventional render = %q, %v", rendered, err)
	}
	if rendered, err := Render("test", `{"outer":{"value":"{{VALUE}}"}}`, "{{VALUE}}", "three"); err != nil || rendered != `{"outer":{"value":"three"}}` {
		t.Fatalf("JSON render = %q, %v", rendered, err)
	}
	if _, err := Load("test", filepath.Join(root, "missing.md"), conventional, "fallback"); err == nil || !strings.Contains(err.Error(), "read test prompt") {
		t.Fatalf("missing explicit prompt error = %v", err)
	}
	if err := Validate("test", "{{UNKNOWN}}", "{{VALUE}}"); err == nil || !strings.Contains(err.Error(), "unresolved token {{UNKNOWN}}") {
		t.Fatalf("unresolved token error = %v", err)
	}
	if err := Validate("test", "{{UNKNOWN", "{{VALUE}}"); err == nil || !strings.Contains(err.Error(), "unmatched template delimiter") {
		t.Fatalf("unmatched delimiter error = %v", err)
	}
	if err := Validate("test", " \n", "{{VALUE}}"); err == nil || !strings.Contains(err.Error(), "prompt is empty") {
		t.Fatalf("empty prompt error = %v", err)
	}

	var assignments Assignments
	if err := assignments.Set("case=" + conventional); err != nil {
		t.Fatal(err)
	}
	if err := assignments.Set("case=other.md"); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("duplicate assignment error = %v", err)
	}
	resolved, err := Resolve([]Spec{{ID: "case", Name: "case", ConventionalPath: conventional, RelativePath: "case.md", Fallback: "fallback", Tokens: []string{"{{VALUE}}"}}}, assignments.Values(), "")
	if err != nil || resolved["case"] != "conventional {{VALUE}}" {
		t.Fatalf("resolved prompt = %q, %v", resolved["case"], err)
	}
	if _, err := Resolve([]Spec{{ID: "case", ConventionalPath: conventional, RelativePath: "case.md", Fallback: "fallback"}}, map[string]string{"unknown": conventional}, ""); err == nil || !strings.Contains(err.Error(), "unknown prompt ID") {
		t.Fatalf("unknown prompt error = %v", err)
	}
}
