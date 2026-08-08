package runner

import "testing"

func TestSupportToolBudgetDefaultsToThirty(t *testing.T) {
	t.Parallel()

	if got := supportToolBudget(map[string]any{}); got != 30 {
		t.Fatalf("supportToolBudget() = %d, want 30", got)
	}
}

func TestSupportToolBudgetUsesExplicitPolicyLimit(t *testing.T) {
	t.Parallel()

	state := map[string]any{
		"policy": map[string]any{
			"max_support_tool_calls_per_opportunity": 42,
		},
	}
	if got := supportToolBudget(state); got != 42 {
		t.Fatalf("supportToolBudget() = %d, want 42", got)
	}
}
