package prompttext

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	rendered, err := Render("test", "case {{CASE_ID}}", map[string]string{"{{CASE_ID}}": "{{literal}}"})
	if err != nil || rendered != "case {{literal}}" {
		t.Fatalf("rendered = %q, %v", rendered, err)
	}
	rendered, err = Render("json", `{"outer":{"value":"{{CASE_ID}}"}}`, map[string]string{"{{CASE_ID}}": "case-1"})
	if err != nil || rendered != `{"outer":{"value":"case-1"}}` {
		t.Fatalf("JSON render = %q, %v", rendered, err)
	}
	for _, source := range []string{"{{UNKNOWN}}", "{{UNKNOWN", " "} {
		if _, err := Render("test", source, map[string]string{"{{CASE_ID}}": "case"}); err == nil || !strings.Contains(err.Error(), "render test prompt") {
			t.Fatalf("render error for %q = %v", source, err)
		}
	}
}
