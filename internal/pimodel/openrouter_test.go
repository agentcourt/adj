package pimodel

import (
	"math"
	"testing"
)

func TestOpenRouterModelSettings(t *testing.T) {
	metadata := map[string]any{
		"supported_parameters":      []any{"max_tokens", "tools"},
		"pricing_prompt":            "0.0000008",
		"pricing_completion":        "0.0000016",
		"pricing_input_cache_read":  "0.0000002",
		"pricing_input_cache_write": nil,
	}
	compat := OpenRouterCompat(map[string]any{"only": []any{"aion-labs"}}, metadata)
	if compat["supportsStore"] != false || compat["maxTokensField"] != "max_tokens" {
		t.Fatalf("compat = %#v", compat)
	}
	cost := OpenRouterCost(metadata)
	if !near(cost["input"], 0.8) || !near(cost["output"], 1.6) || !near(cost["cacheRead"], 0.2) || !near(cost["cacheWrite"], 0.8) {
		t.Fatalf("cost = %#v", cost)
	}
}

func near(value any, want float64) bool {
	got, ok := value.(float64)
	return ok && math.Abs(got-want) < 1e-12
}
