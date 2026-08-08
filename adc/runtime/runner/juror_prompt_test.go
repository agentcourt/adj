package runner

import (
	"strings"
	"testing"

	"github.com/jsmorph/adj/adc/runtime/spec"
)

func TestBuildJurorSystemPromptForVoteRound(t *testing.T) {
	t.Parallel()

	role := spec.RoleSpec{
		Name:           "juror",
		Instructions:   "Decide the case under the instructions.",
		PromptPreamble: "Take the record seriously.",
		AllowedActions: []string{"submit_juror_vote"},
	}
	opportunity := leanOpportunity{
		AllowedTools: []string{"submit_juror_vote"},
	}
	caseObj := map[string]any{
		"deliberation_round": 2,
		"docket": []any{
			map[string]any{"title": "Voir dire answer", "description": "Should not appear."},
			map[string]any{"title": "Opening statement by plaintiff", "description": "Plaintiff opening."},
			map[string]any{"title": "Trial theory - plaintiff", "description": "Plaintiff theory."},
			map[string]any{"title": "Exhibit confession.txt - admitted", "description": "Signed confession."},
			map[string]any{"title": "Exhibit draft-notes.txt - offered", "description": "Should not appear."},
			map[string]any{"title": "Closing argument - defendant", "description": "Defense closing."},
			map[string]any{"title": "Jury instructions delivered", "description": "Use the preponderance standard."},
			map[string]any{"title": "Jury supplemental instruction", "description": "Do not surrender an honestly held view."},
		},
		"jurors": []any{
			map[string]any{"juror_id": "J1", "status": "sworn"},
			map[string]any{"juror_id": "J2", "status": "sworn"},
			map[string]any{"juror_id": "J3", "status": "candidate"},
		},
		"juror_votes": []any{
			map[string]any{"juror_id": "J1", "round": 1, "vote": "plaintiff", "damages": 108, "confidence": "high", "explanation": "Confession proves falsity."},
			map[string]any{"juror_id": "J2", "round": 1, "vote": "defendant", "damages": 0, "confidence": "medium", "explanation": "Missing reliance | proof."},
			map[string]any{"juror_id": "J3", "round": 1, "vote": "plaintiff", "damages": 108, "confidence": "low", "explanation": "Candidate should not appear."},
		},
	}

	prompt := buildJurorSystemPrompt(role, opportunity, "You are skeptical of unsigned drafts.", caseObj)
	want := []string{
		"Role: juror",
		"Role prompt preamble: Take the record seriously.",
		"Allowed actions: submit_juror_vote",
		"Juror identity:",
		"You are skeptical of unsigned drafts.",
		"Deliberation round: 2",
		"Opening statement by plaintiff:",
		"Trial theory - plaintiff:",
		"Exhibit confession.txt - admitted:",
		"Closing argument - defendant:",
		"Evidence review:",
		"Use the case-view and case-file tools",
		"Judge's instructions:",
		"Use the preponderance standard.",
		"Do not surrender an honestly held view.",
		"Prior ballot round:",
		"| J1 | plaintiff | 108 | high | Confession proves falsity. |",
		"| J2 | defendant | 0 | medium | Missing reliance \\| proof. |",
	}
	for _, needle := range want {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("buildJurorSystemPrompt missing %q\n%s", needle, prompt)
		}
	}
	if strings.Contains(prompt, "draft-notes.txt") ||
		strings.Contains(prompt, "Voir dire answer") ||
		strings.Contains(prompt, "Candidate should not appear.") {
		t.Fatalf("buildJurorSystemPrompt leaked non-jury record\n%s", prompt)
	}
}

func TestBuildJurorSystemPromptForNonVoteOmitsTranscript(t *testing.T) {
	t.Parallel()

	prompt := buildJurorSystemPrompt(
		spec.RoleSpec{Name: "juror", Instructions: "Answer the question.", AllowedActions: []string{"answer_questionnaire"}},
		leanOpportunity{AllowedTools: []string{"submit_voir_dire_answer"}},
		"Identity text",
		map[string]any{"docket": []any{map[string]any{"title": "Opening statement by plaintiff", "description": "Should not appear"}}},
	)
	if strings.Contains(prompt, "Trial transcript:") || strings.Contains(prompt, "Judge's instructions:") {
		t.Fatalf("non-vote prompt included deliberation material\n%s", prompt)
	}
}

func TestJuryFacingTranscriptAndInstructionsFallbacks(t *testing.T) {
	t.Parallel()

	if got := juryFacingTrialTranscript(map[string]any{}); got != "(no recorded trial transcript)" {
		t.Fatalf("juryFacingTrialTranscript fallback = %q", got)
	}
	if got := juryInstructionsText(map[string]any{}); got != "(no delivered jury instructions recorded)" {
		t.Fatalf("juryInstructionsText fallback = %q", got)
	}
	if got := priorDeliberationRoundPacket(map[string]any{}, 0); got != "(no prior ballot round)" {
		t.Fatalf("priorDeliberationRoundPacket fallback = %q", got)
	}
}
