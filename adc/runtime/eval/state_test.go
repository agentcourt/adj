package eval

import "testing"

func TestJudgeEvalStatesIncludeResolution(t *testing.T) {
	t.Parallel()

	states := map[string]map[string]any{
		"voir-dire": BuildJudgeVoirDireState(JudgeVoirDireFixture{}),
		"for-cause": BuildJudgeForCauseState(JudgeForCauseFixture{}),
		"rule11":    BuildJudgeRule11State(JudgeRule11Fixture{}),
		"rule12":    BuildJudgeRule12State(JudgeRule12Fixture{}),
		"rule37":    BuildJudgeRule37State(JudgeRule37Fixture{}),
		"rule51":    BuildJudgeRule51State(JudgeRule51Fixture{}),
		"rule52":    BuildJudgeRule52State(JudgeRule52Fixture{}),
		"rule56":    BuildJudgeRule56State(JudgeRule56Fixture{}),
		"rule58":    BuildJudgeRule58State(JudgeRule58Fixture{}),
		"rule60":    BuildJudgeRule60State(JudgeRule60Fixture{}),
	}
	for name, state := range states {
		caseState, ok := state["case"].(map[string]any)
		if !ok {
			t.Fatalf("%s case state = %#v", name, state["case"])
		}
		if got := caseState["resolution"]; got != "pending" {
			t.Errorf("%s resolution = %#v, want pending", name, got)
		}
	}
}
