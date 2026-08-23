package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
)

func TestToolSchemasAndPromptCatalog(t *testing.T) {
	t.Parallel()

	schema := toolSchema("file_bench_opinion")
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required = %#v", schema["required"])
	}
	want := map[string]bool{"text": true, "verdict_for": true}
	for _, name := range required {
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("required fields omit %#v", want)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", schema["properties"])
	}
	verdict, ok := properties["verdict_for"].(map[string]any)
	if !ok {
		t.Fatalf("verdict_for = %#v", properties["verdict_for"])
	}
	values, ok := verdict["enum"].([]string)
	if !ok || len(values) != 2 || values[0] != "plaintiff" || values[1] != "defendant" {
		t.Fatalf("verdict_for enum = %#v", verdict["enum"])
	}
	if got, want := adcprompts.DirectToolNames(), supportedActions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("direct tool-description prompts = %#v, want %#v", got, want)
	}

	overridePath := filepath.Join(t.TempDir(), "source-filename.md")
	if err := os.WriteFile(overridePath, []byte("Custom source filename guidance."), 0o644); err != nil {
		t.Fatalf("write schema-property prompt: %v", err)
	}
	promptCatalog, err := adcprompts.Load(adcprompts.Options{PromptFiles: map[string]string{
		adcprompts.ImportSourceFilenameID: overridePath,
	}})
	if err != nil {
		t.Fatalf("load prompt catalog: %v", err)
	}
	descriptions, err := loadSchemaPropertyDescriptions(promptCatalog)
	if err != nil {
		t.Fatalf("load schema-property descriptions: %v", err)
	}
	importSchema := toolSchemaWithDescriptions("import_case_file", toolSchema("import_case_file"), descriptions)
	importProperties := importSchema["properties"].(map[string]any)
	sourceFilename := importProperties["source_filename"].(map[string]any)
	if got := sourceFilename["description"]; got != "Custom source filename guidance." {
		t.Fatalf("source_filename description = %#v", got)
	}
}
