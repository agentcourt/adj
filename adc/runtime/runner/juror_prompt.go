package runner

import (
	"fmt"
	"strings"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
)

func (r *Runner) buildJurorSystemPrompt(role spec.RoleSpec, opportunity leanOpportunity, personaText string, caseObj map[string]any) (string, error) {
	deliberation := "(none for this opportunity)"
	if jurorVoteOpportunity(opportunity) {
		round := jurorVoteRound(caseObj)
		prior := "(no prior ballot round)"
		if round > 1 {
			prior = priorDeliberationRoundPacket(caseObj, round-1)
		}
		var err error
		deliberation, err = r.prompts.Render(adcprompts.RuntimeJurorDeliberationID, map[string]string{
			"{{ROUND}}":             fmt.Sprintf("%d", round),
			"{{TRANSCRIPT}}":        juryFacingTrialTranscript(caseObj),
			"{{JURY_INSTRUCTIONS}}": juryInstructionsText(caseObj),
			"{{PRIOR_BALLOT}}":      prior,
		})
		if err != nil {
			return "", err
		}
	}
	return r.prompts.Render(adcprompts.RuntimeJurorID, map[string]string{
		"{{ROLE}}":            strings.TrimSpace(role.Name),
		"{{PREAMBLE}}":        promptValue(role.PromptPreamble),
		"{{INSTRUCTIONS}}":    promptValue(role.Instructions),
		"{{ALLOWED_ACTIONS}}": promptList(role.EffectiveAllowedActions()),
		"{{JUROR_ID}}":        promptValue(targetJurorIDForOpportunity(opportunity)),
		"{{PERSONA}}":         promptValue(personaText),
		"{{DELIBERATION}}":    deliberation,
	})
}

func jurorVoteOpportunity(opportunity leanOpportunity) bool {
	for _, tool := range opportunity.AllowedTools {
		if strings.TrimSpace(tool) == "submit_juror_vote" {
			return true
		}
	}
	return false
}

func juryFacingTrialTranscript(caseObj map[string]any) string {
	docket, _ := caseObj["docket"].([]any)
	if len(docket) == 0 {
		return "(no recorded trial transcript)"
	}
	start, end := juryFacingTrialTranscriptBounds(docket)
	if start < 0 || end < start {
		return "(no recorded trial transcript)"
	}
	sections := make([]string, 0)
	for i := start; i <= end; i++ {
		entry, _ := docket[i].(map[string]any)
		if entry == nil {
			continue
		}
		title := strings.TrimSpace(stringOrDefault(entry["title"], ""))
		desc := strings.TrimSpace(stringOrDefault(entry["description"], ""))
		if title == "" || desc == "" || !juryFacingTranscriptTitle(title) {
			continue
		}
		sections = append(sections, fmt.Sprintf("%s:\n%s", title, desc))
	}
	if len(sections) == 0 {
		return "(no recorded trial transcript)"
	}
	return strings.Join(sections, "\n\n")
}

func juryFacingTrialTranscriptBounds(docket []any) (int, int) {
	start := -1
	end := -1
	for i, raw := range docket {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		title := strings.TrimSpace(stringOrDefault(entry["title"], ""))
		if start < 0 && strings.HasPrefix(title, "Opening statement") {
			start = i
		}
		if strings.HasPrefix(title, "Closing argument") || strings.HasPrefix(title, "Closing rebuttal") {
			end = i
		}
	}
	return start, end
}

func juryFacingTranscriptTitle(title string) bool {
	title = strings.TrimSpace(title)
	return strings.HasPrefix(title, "Opening statement") ||
		strings.HasPrefix(title, "Trial theory") ||
		strings.HasPrefix(title, "Rebuttal presentation") ||
		strings.HasPrefix(title, "Surrebuttal presentation") ||
		strings.HasPrefix(title, "Closing argument") ||
		strings.HasPrefix(title, "Closing rebuttal") ||
		strings.HasPrefix(title, "Technical report") ||
		strings.HasPrefix(title, "Exhibit ") && strings.HasSuffix(title, " - admitted")
}

func juryInstructionsText(caseObj map[string]any) string {
	docket, _ := caseObj["docket"].([]any)
	sections := make([]string, 0, 2)
	for _, raw := range docket {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		title := strings.TrimSpace(stringOrDefault(entry["title"], ""))
		if title != "Jury instructions delivered" && title != "Jury supplemental instruction" {
			continue
		}
		text := strings.TrimSpace(stringOrDefault(entry["description"], ""))
		if text != "" {
			sections = append(sections, text)
		}
	}
	if len(sections) == 0 {
		return "(no delivered jury instructions recorded)"
	}
	return strings.Join(sections, "\n\n")
}

func jurorVoteRound(caseObj map[string]any) int {
	round := toInt(caseObj["deliberation_round"])
	if round <= 0 {
		return 1
	}
	return round
}

func priorDeliberationRoundPacket(caseObj map[string]any, round int) string {
	if round <= 0 {
		return "(no prior ballot round)"
	}
	jurors, _ := caseObj["jurors"].([]any)
	votesByJuror := map[string]map[string]any{}
	rawVotes, _ := caseObj["juror_votes"].([]any)
	for _, raw := range rawVotes {
		vote, _ := raw.(map[string]any)
		if vote == nil || toInt(vote["round"]) != round {
			continue
		}
		jurorID := strings.TrimSpace(stringOrDefault(vote["juror_id"], ""))
		if jurorID == "" {
			continue
		}
		votesByJuror[jurorID] = vote
	}
	if len(votesByJuror) == 0 {
		return "(no prior ballot round)"
	}
	lines := []string{
		fmt.Sprintf("Round %d results:", round),
		"| Juror ID | Vote | Damages | Confidence | Explanation |",
		"|---|---|---:|---|---|",
	}
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if juror == nil || strings.TrimSpace(stringOrDefault(juror["status"], "")) != "sworn" {
			continue
		}
		jurorID := strings.TrimSpace(stringOrDefault(juror["juror_id"], ""))
		if jurorID == "" {
			continue
		}
		vote := votesByJuror[jurorID]
		if vote == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"| %s | %s | %v | %s | %s |",
			jurorID,
			escapePromptPipe(strings.TrimSpace(stringOrDefault(vote["vote"], ""))),
			vote["damages"],
			escapePromptPipe(strings.TrimSpace(stringOrDefault(vote["confidence"], ""))),
			escapePromptPipe(strings.TrimSpace(stringOrDefault(vote["explanation"], ""))),
		))
	}
	return strings.Join(lines, "\n")
}

func escapePromptPipe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "n/a"
	}
	return strings.ReplaceAll(s, "|", "\\|")
}
