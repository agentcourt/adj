package runner

import "testing"

func TestBenchOpinionRequiresStructuredVerdict(t *testing.T) {
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
}
