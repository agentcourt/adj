package runner

import (
	"strings"
	"testing"
)

func TestResultMapRejectsUnsupportedValues(t *testing.T) {
	_, err := resultMap(Result{FinalState: map[string]any{"bad": make(chan struct{})}})
	if err == nil {
		t.Fatal("resultMap accepted an unsupported value")
	}
	if !strings.Contains(err.Error(), "marshal run result for storage") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResultMapPreservesResult(t *testing.T) {
	got, err := resultMap(Result{Scenario: "example", FinalState: map[string]any{"count": 2}})
	if err != nil {
		t.Fatalf("resultMap: %v", err)
	}
	if got["scenario"] != "example" {
		t.Fatalf("scenario = %v", got["scenario"])
	}
	state, ok := got["final_state"].(map[string]any)
	if !ok || state["count"] != float64(2) {
		t.Fatalf("final_state = %#v", got["final_state"])
	}
}
