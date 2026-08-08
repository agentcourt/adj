package runner

func supportToolBudget(state map[string]any) int {
	policy, _ := state["policy"].(map[string]any)
	if policy == nil {
		return 30
	}
	if value := toInt(policy["max_support_tool_calls_per_opportunity"]); value > 0 {
		return value
	}
	return 30
}
