package report

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jsmorph/adj/adc/runtime/runner"
	"github.com/jsmorph/adj/common/openai"
)

func WriteTranscript(path string, result runner.Result) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	caseObj := getMap(result.FinalState["case"])
	caption := strOr(caseObj["caption"], "Unknown caption")
	court := strOr(result.FinalState["court_name"], "Unknown court")
	docket := getSlice(caseObj, "docket")

	var b strings.Builder
	b.WriteString("# Trial Transcript\n\n")
	b.WriteString(fmt.Sprintf("Case: %s\n\n", caption))
	b.WriteString(fmt.Sprintf("Court: %s\n\n", court))
	b.WriteString("## External Agent Activity\n\n")
	b.WriteString(renderExternalActivityTable(result.TurnLogs))
	b.WriteString("\n")
	b.WriteString("## Proceedings\n\n")

	count := 0
	for _, raw := range docket {
		entry := getMap(raw)
		title := strOr(entry["title"], "")
		desc := strOr(entry["description"], "")
		if !isTranscriptSection(title) {
			continue
		}
		if strings.TrimSpace(desc) == "" {
			continue
		}
		b.WriteString("### ")
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(desc)
		b.WriteString("\n\n")
		count++
	}
	if count == 0 {
		b.WriteString("No courtroom argument sections were recorded in the docket.\n")
	}
	b.WriteString("\n## Deliberation\n\n")
	b.WriteString(renderJurorRounds(caseObj))
	b.WriteString("\n## Judgment\n\n")
	b.WriteString(renderTranscriptJudgment(caseObj, docket))
	return writeFile(path, b.String())
}

func WriteDigest(path string, result runner.Result) error {
	return WriteDigestWithClient(path, result, "", nil)
}

func WriteDigestWithModel(path string, result runner.Result, model string) error {
	return WriteDigestWithClient(path, result, model, nil)
}

func WriteDigestWithClient(path string, result runner.Result, model string, client *openai.Client) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	state := result.FinalState
	caseObj := getMap(state["case"])
	docket := getSlice(caseObj, "docket")
	jury := getMap(caseObj["jury_verdict"])

	caption := strOr(caseObj["caption"], "Unknown caption")
	caseID := strOr(caseObj["case_id"], "")
	status := strOr(caseObj["status"], "")
	trialMode := strOr(caseObj["trial_mode"], "")
	phase := strOr(caseObj["phase"], "")

	var b strings.Builder
	b.WriteString("# Case Digest\n\n")
	b.WriteString("## Snapshot\n\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("|---|---|\n")
	b.WriteString(fmt.Sprintf("| Case | %s |\n", escapePipe(caption)))
	b.WriteString(fmt.Sprintf("| Case ID | %s |\n", escapePipe(caseID)))
	b.WriteString(fmt.Sprintf("| Status | %s |\n", escapePipe(status)))
	b.WriteString(fmt.Sprintf("| Trial mode | %s |\n", escapePipe(trialMode)))
	b.WriteString(fmt.Sprintf("| Phase | %s |\n", escapePipe(phase)))
	b.WriteString(fmt.Sprintf("| Docket entries | %d |\n", len(docket)))
	b.WriteString(fmt.Sprintf("| Turn logs | %d |\n", len(result.TurnLogs)))

	b.WriteString("\n## External Agent Activity\n\n")
	b.WriteString(renderExternalActivityTable(result.TurnLogs))

	b.WriteString("\n## Important Agent Bash Executions\n\n")
	b.WriteString(renderImportantAgentBashExecutions(result.TurnLogs))

	b.WriteString("\n## Complaint and Background\n\n")
	b.WriteString(complaintAndBackground(caseObj, docket))

	b.WriteString("\n## Pretrial Activity\n\n")
	b.WriteString(pretrialNarrative(caseObj, docket))

	b.WriteString("\n## Voir Dire\n\n")
	b.WriteString(renderVoirDireSection(state, caseObj, docket))

	b.WriteString("\n## Verdict\n\n")
	rounds := jurorVoteRounds(caseObj)
	if len(jury) == 0 {
		hung := getMap(caseObj["hung_jury"])
		if len(hung) == 0 {
			b.WriteString("No jury verdict recorded.\n")
		} else {
			b.WriteString("| Field | Value |\n")
			b.WriteString("|---|---|\n")
			b.WriteString(fmt.Sprintf("| Result | Hung jury |\n"))
			if len(rounds) > 0 {
				b.WriteString(fmt.Sprintf("| Deliberation rounds | %d |\n", len(rounds)))
			}
			b.WriteString(fmt.Sprintf("| Note | %s |\n", escapePipe(strOr(hung["note"], ""))))
		}
	} else {
		b.WriteString("| Field | Value |\n")
		b.WriteString("|---|---|\n")
		b.WriteString(fmt.Sprintf("| For | %s |\n", escapePipe(strOr(jury["verdict_for"], ""))))
		if len(rounds) > 0 {
			b.WriteString(fmt.Sprintf("| Deliberation rounds | %d |\n", len(rounds)))
		}
		b.WriteString(fmt.Sprintf("| Votes | %d/%d |\n", asInt(jury["votes_for_verdict"]), asInt(jury["required_votes"])))
		b.WriteString(fmt.Sprintf("| Damages | %s |\n", escapePipe(fmt.Sprintf("%v", jury["damages"]))))
	}

	b.WriteString("\n## Juror Votes\n\n")
	b.WriteString(renderJurorRounds(caseObj))

	b.WriteString("\n## Evidence Table\n\n")
	b.WriteString(renderEvidenceTable(caseObj, docket))

	b.WriteString("\n## Reports\n\n")
	b.WriteString(renderTechnicalReportsTable(caseObj))

	b.WriteString("\n## Key Courtroom Arguments\n\n")
	args := collectArgumentLines(docket)
	if len(args) == 0 {
		b.WriteString("No opening, trial-theory, or closing entries were found.\n")
	} else {
		for _, line := range args {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n## Side Argument Summaries\n\n")
	summary, err := summarizeArgumentsBySide(caseObj, docket, model, client)
	if err != nil {
		return fmt.Errorf("generate side argument summaries: %w", err)
	}
	b.WriteString(fmt.Sprintf("Summary source: `%s`\n\n", summary.Source))
	b.WriteString("### Plaintiff\n\n")
	b.WriteString(summary.Plaintiff)
	b.WriteString("\n\n")
	b.WriteString("### Defendant\n\n")
	b.WriteString(summary.Defendant)
	b.WriteString("\n")

	b.WriteString("\n## Files and Exhibits\n\n")
	filesInRecord, admitted := countFileAndExhibitEvents(caseObj, docket)
	b.WriteString(fmt.Sprintf("- Evidence files in record: %d\n", filesInRecord))
	b.WriteString(fmt.Sprintf("- Admitted exhibits: %d\n", admitted))

	return writeFile(path, b.String())
}

func writeFile(path string, body string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create report directory: %w", err)
		}
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}
	return nil
}

func isTranscriptSection(title string) bool {
	return strings.HasPrefix(title, "Technical report") ||
		strings.HasPrefix(title, "Opening statement") ||
		strings.HasPrefix(title, "Trial theory") ||
		strings.HasPrefix(title, "Rebuttal presentation") ||
		strings.HasPrefix(title, "Surrebuttal presentation") ||
		strings.HasPrefix(title, "Closing argument") ||
		strings.HasPrefix(title, "Closing rebuttal") ||
		strings.HasPrefix(title, "Proposed jury instruction") ||
		strings.HasPrefix(title, "Jury instruction objection") ||
		title == "Jury instructions settled" ||
		title == "Jury instructions delivered" ||
		title == "Jury supplemental instruction" ||
		title == "Jury deliberation note" ||
		title == "Jury verdict derived" ||
		title == "Judgment entered" ||
		title == "Bench Opinion"
}

func collectArgumentLines(docket []any) []string {
	lines := make([]string, 0)
	for _, raw := range docket {
		entry := getMap(raw)
		title := strOr(entry["title"], "")
		desc := strOr(entry["description"], "")
		if !(strings.HasPrefix(title, "Opening statement") || strings.HasPrefix(title, "Trial theory") || strings.HasPrefix(title, "Closing argument")) {
			continue
		}
		desc = strings.TrimSpace(strings.ReplaceAll(desc, "\n", " "))
		if desc == "" {
			continue
		}
		desc = shortenAtWord(desc, 360)
		lines = append(lines, fmt.Sprintf("**%s**: %s", title, desc))
	}
	return lines
}

type sideSummaryResult struct {
	Plaintiff string
	Defendant string
	Source    string
}

func summarizeArgumentsBySide(caseObj map[string]any, docket []any, model string, client *openai.Client) (sideSummaryResult, error) {
	plaintiffText, defendantText := collectSideArguments(docket)
	courtroomContext := collectCourtroomContext(docket)
	evidenceContext := collectEvidenceContext(caseObj, docket)
	if strings.TrimSpace(plaintiffText) == "" && strings.TrimSpace(defendantText) == "" {
		msg := "No courtroom argument text was available for summary."
		return sideSummaryResult{
			Plaintiff: msg,
			Defendant: msg,
			Source:    "none",
		}, nil
	}

	plaintiffLLM, defendantLLM, err := summarizeArgumentsBySideLLM(plaintiffText, defendantText, courtroomContext, evidenceContext, model, client)
	if err == nil {
		return sideSummaryResult{
			Plaintiff: plaintiffLLM,
			Defendant: defendantLLM,
			Source:    "llm",
		}, nil
	}
	return sideSummaryResult{}, err
}

func resolveSummaryModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("OPENAI_REPORT_MODEL"))
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return model
}

func collectSideArguments(docket []any) (string, string) {
	var plaintiffParts []string
	var defendantParts []string
	for _, raw := range docket {
		entry := getMap(raw)
		title := strings.TrimSpace(strOr(entry["title"], ""))
		desc := strings.TrimSpace(strOr(entry["description"], ""))
		if title == "" || desc == "" || !isTranscriptSection(title) {
			continue
		}
		lower := strings.ToLower(title)
		block := fmt.Sprintf("%s:\n%s", title, desc)
		if strings.HasSuffix(lower, "- plaintiff") {
			plaintiffParts = append(plaintiffParts, block)
			continue
		}
		if strings.HasSuffix(lower, "- defendant") {
			defendantParts = append(defendantParts, block)
			continue
		}
	}
	return strings.Join(plaintiffParts, "\n\n"), strings.Join(defendantParts, "\n\n")
}

func collectCourtroomContext(docket []any) string {
	var sections []string
	for _, raw := range docket {
		entry := getMap(raw)
		title := strings.TrimSpace(strOr(entry["title"], ""))
		desc := strings.TrimSpace(strOr(entry["description"], ""))
		if title == "" || desc == "" {
			continue
		}
		if !(isTranscriptSection(title) || strings.HasPrefix(title, "Trial objection") || strings.HasPrefix(title, "Exhibit ")) {
			continue
		}
		sections = append(sections, fmt.Sprintf("%s:\n%s", title, desc))
	}
	return strings.Join(sections, "\n\n")
}

func collectEvidenceContext(caseObj map[string]any, docket []any) string {
	var lines []string
	for _, raw := range getSlice(caseObj, "case_files") {
		f := getMap(raw)
		fileID := strings.TrimSpace(strOr(f["file_id"], ""))
		label := strings.TrimSpace(strOr(f["label"], ""))
		orig := strings.TrimSpace(strOr(f["original_name"], ""))
		if fileID == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("case_file %s: %s (%s)", fileID, label, orig))
	}
	for _, raw := range docket {
		entry := getMap(raw)
		title := strings.TrimSpace(strOr(entry["title"], ""))
		if !strings.HasPrefix(title, "Exhibit ") {
			continue
		}
		desc := strings.TrimSpace(strOr(entry["description"], ""))
		lines = append(lines, fmt.Sprintf("%s: %s", title, desc))
	}
	return strings.Join(lines, "\n")
}

func summarizeArgumentsBySideLLM(plaintiffText, defendantText, courtroomContext, evidenceContext, model string, client *openai.Client) (string, string, error) {
	var err error
	if client == nil {
		client, err = openai.NewFromEnv(false, 90*time.Second)
		if err != nil {
			return "", "", fmt.Errorf("create llm client: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	model = resolveSummaryModel(model)
	temp := 0.2
	input := []map[string]any{
		{
			"role":    "system",
			"content": "You write high-quality civil trial digests. Summarize argument content precisely, with concrete issues, burdens, evidentiary references, and vulnerabilities. Do not invent facts.",
		},
		{
			"role":    "user",
			"content": buildSummaryPrompt(plaintiffText, defendantText, courtroomContext, evidenceContext),
		},
	}
	resp, err := client.CreateResponse(ctx, model, input, nil, "", &temp)
	if err != nil {
		return "", "", fmt.Errorf("request summary: %w", err)
	}
	out, err := parseSideSummaryPayload(strings.TrimSpace(resp.Text))
	if err != nil {
		fixPrompt := []map[string]any{
			{
				"role":    "system",
				"content": "Convert the provided text into strict JSON only. Do not include any prose outside JSON.",
			},
			{
				"role":    "user",
				"content": "Return strict JSON with keys plaintiff_summary and defendant_summary from this text. Keep citation anchors in square brackets.\n\n" + strings.TrimSpace(resp.Text),
			},
		}
		fixResp, fixErr := client.CreateResponse(ctx, model, fixPrompt, nil, "", &temp)
		if fixErr != nil {
			return "", "", fmt.Errorf("summary parse failed (%v) and repair failed: %w", err, fixErr)
		}
		out, err = parseSideSummaryPayload(strings.TrimSpace(fixResp.Text))
		if err != nil {
			return "", "", fmt.Errorf("summary parse failed after repair: %w", err)
		}
	}
	out.PlaintiffSummary = strings.TrimSpace(out.PlaintiffSummary)
	out.DefendantSummary = strings.TrimSpace(out.DefendantSummary)
	if out.PlaintiffSummary == "" || out.DefendantSummary == "" {
		return "", "", fmt.Errorf("empty side summary fields")
	}
	if !strings.Contains(out.PlaintiffSummary, "[") || !strings.Contains(out.PlaintiffSummary, "]") {
		return "", "", fmt.Errorf("plaintiff summary missing citation anchors")
	}
	if !strings.Contains(out.DefendantSummary, "[") || !strings.Contains(out.DefendantSummary, "]") {
		return "", "", fmt.Errorf("defendant summary missing citation anchors")
	}
	return out.PlaintiffSummary, out.DefendantSummary, nil
}

func buildSummaryPrompt(plaintiffText, defendantText, courtroomContext, evidenceContext string) string {
	var b strings.Builder
	b.WriteString("Summarize each side's courtroom arguments from this trial record.\n")
	b.WriteString("Output requirements:\n")
	b.WriteString("1. Return strict JSON only with keys plaintiff_summary and defendant_summary.\n")
	b.WriteString("2. Each value should be 1 to 2 detailed paragraphs.\n")
	b.WriteString("3. Cover legal theory, burden framing, strongest support, weaknesses, and treatment of opposing points.\n")
	b.WriteString("4. Use only the provided text.\n")
	b.WriteString("5. For each major point, include a citation anchor in square brackets using a docket title exactly as given, for example [Opening statement - plaintiff].\n\n")
	b.WriteString("Full courtroom record context:\n")
	if strings.TrimSpace(courtroomContext) == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(shortenAtWord(courtroomContext, 22000))
		b.WriteString("\n")
	}
	b.WriteString("\nEvidence and exhibit context:\n")
	if strings.TrimSpace(evidenceContext) == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(shortenAtWord(evidenceContext, 5000))
		b.WriteString("\n")
	}
	b.WriteString("\nPrimary side texts:\n")
	b.WriteString("Plaintiff courtroom text:\n")
	if strings.TrimSpace(plaintiffText) == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(shortenAtWord(plaintiffText, 12000))
		b.WriteString("\n")
	}
	b.WriteString("\nDefendant courtroom text:\n")
	if strings.TrimSpace(defendantText) == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(shortenAtWord(defendantText, 12000))
		b.WriteString("\n")
	}
	return b.String()
}

func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

type sideSummaryPayload struct {
	PlaintiffSummary string `json:"plaintiff_summary"`
	DefendantSummary string `json:"defendant_summary"`
}

func parseSideSummaryPayload(raw string) (sideSummaryPayload, error) {
	var out sideSummaryPayload
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out, nil
	}
	obj := extractJSONObject(raw)
	if obj == "" {
		return out, fmt.Errorf("no json object found")
	}
	if err := json.Unmarshal([]byte(obj), &out); err != nil {
		return out, fmt.Errorf("invalid json object: %w", err)
	}
	return out, nil
}

func countFileAndExhibitEvents(caseObj map[string]any, docket []any) (int, int) {
	filesInRecord := len(getSlice(caseObj, "case_files"))
	admitted := 0
	for _, raw := range docket {
		title := strOr(getMap(raw)["title"], "")
		if strings.HasPrefix(title, "Exhibit ") && strings.HasSuffix(title, " - admitted") {
			admitted++
		}
	}
	return filesInRecord, admitted
}

type externalTurnActivity struct {
	Turn           int
	Role           string
	Phase          string
	Methods        []string
	ContainerTools []string
	LegalResult    string
}

func collectExternalActivities(turnLogs []runner.TurnLog) []externalTurnActivity {
	activities := make([]externalTurnActivity, 0)
	for i, log := range turnLogs {
		methodsSeen := map[string]bool{}
		containerSeen := map[string]bool{}
		methods := make([]string, 0)
		containerTools := make([]string, 0)
		legalResult := ""
		hasExternal := false
		for _, raw := range log.Transcript {
			if method := strings.TrimSpace(strOr(raw["custom_method"], "")); method != "" {
				hasExternal = true
				if !methodsSeen[method] {
					methodsSeen[method] = true
					methods = append(methods, method)
				}
				continue
			}
			if toolCall := getMap(raw["agent_tool_call"]); len(toolCall) > 0 {
				hasExternal = true
				title := strings.TrimSpace(strOr(toolCall["title"], ""))
				if title != "" && !strings.HasPrefix(title, "adc_") && !containerSeen[title] {
					containerSeen[title] = true
					containerTools = append(containerTools, title)
				}
				continue
			}
			action := strings.TrimSpace(strOr(raw["action"], ""))
			if action == "" {
				continue
			}
			if action == "pass_turn" {
				hasExternal = true
				legalResult = action
				continue
			}
			if isExternalReferenceAction(action) {
				hasExternal = true
				continue
			}
			legalResult = action
		}
		if !hasExternal {
			continue
		}
		activities = append(activities, externalTurnActivity{
			Turn:           i + 1,
			Role:           log.Role,
			Phase:          log.OpportunityPhase,
			Methods:        methods,
			ContainerTools: containerTools,
			LegalResult:    legalResult,
		})
	}
	return activities
}

func renderExternalActivityTable(turnLogs []runner.TurnLog) string {
	activities := collectExternalActivities(turnLogs)
	if len(activities) == 0 {
		return "No external role activity recorded.\n"
	}
	var b strings.Builder
	b.WriteString("| Turn | Role | Phase | API methods | Agent tools | Legal result |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, item := range activities {
		methods := strings.Join(item.Methods, ", ")
		if strings.TrimSpace(methods) == "" {
			methods = "none"
		}
		containerTools := strings.Join(item.ContainerTools, ", ")
		if strings.TrimSpace(containerTools) == "" {
			containerTools = "none"
		}
		legalResult := item.LegalResult
		if strings.TrimSpace(legalResult) == "" {
			legalResult = "none"
		}
		b.WriteString(fmt.Sprintf(
			"| %d | %s | %s | %s | %s | %s |\n",
			item.Turn,
			escapePipe(item.Role),
			escapePipe(emptyNA(item.Phase)),
			escapePipe(methods),
			escapePipe(containerTools),
			escapePipe(legalResult),
		))
	}
	return b.String()
}

type agentBashExecution struct {
	Turn    int
	Role    string
	Phase   string
	Command string
	Status  string
	Output  string
}

func collectAgentBashExecutions(turnLogs []runner.TurnLog) []agentBashExecution {
	out := make([]agentBashExecution, 0)
	for i, log := range turnLogs {
		pending := map[string]agentBashExecution{}
		order := make([]string, 0)
		for _, raw := range log.Transcript {
			if toolCall := getMap(raw["agent_tool_call"]); len(toolCall) > 0 {
				if strings.TrimSpace(strOr(toolCall["title"], "")) != "bash" {
					continue
				}
				toolCallID := strings.TrimSpace(strOr(toolCall["tool_call_id"], ""))
				if toolCallID == "" {
					continue
				}
				if _, ok := pending[toolCallID]; !ok {
					order = append(order, toolCallID)
				}
				pending[toolCallID] = agentBashExecution{
					Turn:    i + 1,
					Role:    log.Role,
					Phase:   log.OpportunityPhase,
					Command: bashCommandFromRawInput(toolCall["raw_input"]),
					Status:  strings.TrimSpace(strOr(toolCall["status"], "")),
				}
				continue
			}
			update := getMap(raw["agent_tool_update"])
			if len(update) == 0 {
				continue
			}
			toolCallID := strings.TrimSpace(strOr(update["tool_call_id"], ""))
			entry, ok := pending[toolCallID]
			if !ok {
				continue
			}
			if cmd := bashCommandFromRawInput(update["raw_input"]); len(strings.TrimSpace(cmd)) > len(strings.TrimSpace(entry.Command)) {
				entry.Command = cmd
			}
			status := strings.TrimSpace(strOr(update["status"], ""))
			if status != "" {
				entry.Status = status
			}
			if text := bashOutputText(update["raw_output"]); text != "" {
				entry.Output = text
			}
			pending[toolCallID] = entry
		}
		for _, toolCallID := range order {
			entry := pending[toolCallID]
			if !isImportantBashExecution(entry.Command, entry.Output) {
				continue
			}
			out = append(out, entry)
		}
	}
	return out
}

func renderImportantAgentBashExecutions(turnLogs []runner.TurnLog) string {
	executions := collectAgentBashExecutions(turnLogs)
	if len(executions) == 0 {
		return "No important bash executions were recorded.\n"
	}
	var b strings.Builder
	b.WriteString("| Turn | Role | Phase | Status | Command | Output |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, item := range executions {
		status := item.Status
		if strings.TrimSpace(status) == "" {
			status = "unknown"
		}
		output := strings.TrimSpace(item.Output)
		if output == "" {
			output = "n/a"
		} else {
			output = shortenAtWord(strings.ReplaceAll(output, "\n", " "), 160)
		}
		b.WriteString(fmt.Sprintf(
			"| %d | %s | %s | %s | %s | %s |\n",
			item.Turn,
			escapePipe(item.Role),
			escapePipe(emptyNA(item.Phase)),
			escapePipe(status),
			escapePipe(shortenAtWord(item.Command, 160)),
			escapePipe(output),
		))
	}
	return b.String()
}

func bashCommandFromRawInput(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if len(value) == 0 {
			return ""
		}
		if cmd := strings.TrimSpace(strOr(value["cmd"], "")); cmd != "" {
			return cmd
		}
		if cmd := strings.TrimSpace(strOr(value["command"], "")); cmd != "" {
			return cmd
		}
		if argv, ok := value["argv"].([]any); ok {
			parts := make([]string, 0, len(argv))
			for _, item := range argv {
				part := strings.TrimSpace(strOr(item, ""))
				if part != "" {
					parts = append(parts, part)
				}
			}
			return strings.Join(parts, " ")
		}
	}
	wire, err := json.Marshal(raw)
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(raw))
	}
	return strings.TrimSpace(string(wire))
}

func renderTranscriptJudgment(caseObj map[string]any, docket []any) string {
	jury := getMap(caseObj["jury_verdict"])
	hung := getMap(caseObj["hung_jury"])
	status := strings.TrimSpace(strOr(caseObj["status"], ""))
	judgmentDesc := strings.TrimSpace(firstDocketDescriptionByTitle(docket, "Judgment entered"))

	if len(jury) > 0 {
		var b strings.Builder
		b.WriteString("| Field | Value |\n")
		b.WriteString("|---|---|\n")
		b.WriteString(fmt.Sprintf("| Status | %s |\n", escapePipe(emptyNA(status))))
		b.WriteString(fmt.Sprintf("| Verdict | %s |\n", escapePipe(emptyNA(strOr(jury["verdict_for"], "")))))
		b.WriteString(fmt.Sprintf("| Votes | %d/%d |\n", asInt(jury["votes_for_verdict"]), asInt(jury["required_votes"])))
		b.WriteString(fmt.Sprintf("| Damages | %v |\n", jury["damages"]))
		b.WriteString(fmt.Sprintf("| Monetary judgment | %v |\n", caseObj["monetary_judgment"]))
		if judgmentDesc != "" {
			b.WriteString(fmt.Sprintf("| Basis | %s |\n", escapePipe(judgmentDesc)))
		}
		return b.String()
	}

	if len(hung) > 0 {
		var b strings.Builder
		b.WriteString("| Field | Value |\n")
		b.WriteString("|---|---|\n")
		b.WriteString(fmt.Sprintf("| Status | %s |\n", escapePipe(emptyNA(status))))
		b.WriteString("| Result | hung jury |\n")
		b.WriteString(fmt.Sprintf("| Note | %s |\n", escapePipe(emptyNA(strOr(hung["note"], "")))))
		if judgmentDesc != "" {
			b.WriteString(fmt.Sprintf("| Basis | %s |\n", escapePipe(judgmentDesc)))
		}
		return b.String()
	}

	if judgmentDesc == "" {
		return "No judgment record was available.\n"
	}
	return judgmentDesc + "\n"
}

func bashOutputText(raw any) string {
	value, _ := raw.(map[string]any)
	if value == nil {
		return ""
	}
	if text := strings.TrimSpace(contentText(getSlice(value, "content"))); text != "" {
		return text
	}
	details := getMap(value["details"])
	stdout := strings.TrimSpace(strOr(details["stdout"], strOr(value["stdout"], strOr(details["output"], strOr(value["output"], "")))))
	stderr := strings.TrimSpace(strOr(details["stderr"], strOr(value["stderr"], "")))
	exitCode := ""
	if code, ok := details["exitCode"]; ok {
		exitCode = fmt.Sprintf("%v", code)
	} else if code, ok := value["exitCode"]; ok {
		exitCode = fmt.Sprintf("%v", code)
	} else if code, ok := details["code"]; ok {
		exitCode = fmt.Sprintf("%v", code)
	} else if code, ok := value["code"]; ok {
		exitCode = fmt.Sprintf("%v", code)
	}
	parts := make([]string, 0, 3)
	if stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr != "" {
		parts = append(parts, "stderr: "+stderr)
	}
	if exitCode != "" {
		parts = append(parts, "exit code: "+exitCode)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func contentText(items []any) string {
	parts := make([]string, 0)
	for _, raw := range items {
		item := getMap(raw)
		text := strings.TrimSpace(strOr(item["text"], ""))
		if text == "" {
			content := getMap(item["content"])
			text = strings.TrimSpace(strOr(content["text"], ""))
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func isImportantBashExecution(command string, output string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	output = strings.ToLower(strings.TrimSpace(output))
	if command == "" {
		return false
	}
	for _, needle := range []string{
		"openssl",
		"base64",
		"sha256",
		" dgst ",
		" -verify ",
		"apt ",
		"apt-get",
		"apk ",
		"pip ",
		"npm ",
		"python",
		"node ",
		"go ",
		"git ",
		"curl ",
		"jq ",
		"|",
		">",
		";",
		"&&",
	} {
		if strings.Contains(command, needle) {
			return true
		}
	}
	if strings.Contains(output, "verified ok") || strings.Contains(output, "exit code:") || strings.Contains(output, "stderr:") {
		return true
	}
	return len(command) >= 60
}

func isExternalReferenceAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "get_case", "list_case_files", "read_case_text_file", "request_case_file", "explain_decisions":
		return true
	default:
		return false
	}
}

func caseFileStatuses(caseObj map[string]any) map[string]string {
	statuses := make(map[string]string)
	for _, raw := range getSlice(caseObj, "file_events") {
		event := getMap(raw)
		fileID := strings.TrimSpace(strOr(event["file_id"], ""))
		if fileID == "" {
			continue
		}
		switch strOr(event["action"], "") {
		case "filed_with_complaint":
			statuses[fileID] = "filed with complaint"
		case "import", "import_case_file":
			if _, ok := statuses[fileID]; !ok {
				statuses[fileID] = "imported"
			}
		}
	}
	return statuses
}

func complaintAndBackground(caseObj map[string]any, docket []any) string {
	claim := getMap(caseObj["single_claim"])
	complaint := firstDocketDescriptionByTitle(docket, "Complaint filed")
	if strings.TrimSpace(complaint) == "" {
		complaint = "No explicit complaint text found in the docket."
	}
	label := strOr(claim["label"], "")
	theory := strOr(claim["legal_theory"], "")
	standard := strOr(claim["standard_of_proof"], "")
	if strings.TrimSpace(label) == "" && strings.TrimSpace(theory) == "" {
		return complaint + "\n"
	}
	return fmt.Sprintf(
		"%s\n\nClaim profile: `%s` under `%s`, standard `%s`.\n",
		complaint,
		emptyNA(label),
		emptyNA(theory),
		emptyNA(standard),
	)
}

func pretrialNarrative(caseObj map[string]any, docket []any) string {
	lines := make([]string, 0)
	for _, raw := range docket {
		entry := getMap(raw)
		title := strOr(entry["title"], "")
		desc := strings.TrimSpace(strOr(entry["description"], ""))
		if title == "" {
			continue
		}
		if strings.HasPrefix(title, "Opening statement") ||
			strings.HasPrefix(title, "Trial theory") ||
			strings.HasPrefix(title, "Rebuttal presentation") ||
			strings.HasPrefix(title, "Surrebuttal presentation") ||
			strings.HasPrefix(title, "Closing argument") ||
			strings.HasPrefix(title, "Closing rebuttal") ||
			title == "Bench Opinion" {
			continue
		}
		if strings.HasPrefix(title, "Phase: ") {
			continue
		}
		if desc == "" {
			lines = append(lines, fmt.Sprintf("- %s", title))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: %s", title, shorten(desc, 260)))
		}
	}
	if len(lines) == 0 {
		return "No pretrial activity summary was available.\n"
	}
	var b strings.Builder
	b.WriteString("Pretrial progression included the following docketed activity:\n\n")
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func renderVoirDireSection(state map[string]any, caseObj map[string]any, docket []any) string {
	jurors := getSlice(caseObj, "jurors")
	if len(jurors) == 0 {
		return "No juror candidates were recorded.\n"
	}
	policy := getMap(state["policy"])
	skipVoirDire := asInt(policy["skip_voir_dire"]) != 0
	exchanges := getSlice(caseObj, "voir_dire_exchanges")
	challenges := getSlice(caseObj, "for_cause_challenges")

	var b strings.Builder
	b.WriteString("### Candidate Panel\n\n")
	b.WriteString("| Juror ID | Status | Model | Persona file |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, raw := range jurors {
		j := getMap(raw)
		b.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s |\n",
			escapePipe(emptyNA(strOr(j["juror_id"], ""))),
			escapePipe(emptyNA(strOr(j["status"], ""))),
			escapePipe(emptyNA(firstNonEmptyString(j, "model", "llm_model"))),
			escapePipe(emptyNA(firstNonEmptyString(j, "persona_filename", "persona_file", "persona_path"))),
		))
	}

	b.WriteString("\n### Questionnaire Record\n\n")
	if skipVoirDire {
		b.WriteString("Voir dire was skipped.  No juror questionnaire was issued.\n")
	} else {
		b.WriteString(renderQuestionnaireRecord(caseObj))
	}

	b.WriteString("\n### Voir Dire Exchanges\n\n")
	if skipVoirDire {
		b.WriteString("Voir dire was skipped.  No lawyer questioning was recorded.\n")
	} else if len(exchanges) == 0 {
		b.WriteString("No voir dire exchanges were recorded.\n")
	} else {
		b.WriteString("| # | Juror ID | Asked by | Ruling | Question | Judge reason | Answer |\n")
		b.WriteString("|---:|---|---|---|---|---|---|\n")
		for i, raw := range exchanges {
			exchange := getMap(raw)
			ruling := "pending"
			if allowed, ok := exchange["judge_allowed"].(bool); ok {
				if allowed {
					ruling = "allowed"
				} else {
					ruling = "disallowed"
				}
			}
			b.WriteString(fmt.Sprintf(
				"| %d | %s | %s | %s | %s | %s | %s |\n",
				i+1,
				escapePipe(emptyNA(strOr(exchange["juror_id"], ""))),
				escapePipe(emptyNA(strOr(exchange["asked_by"], ""))),
				escapePipe(ruling),
				escapePipe(emptyNA(shorten(strings.TrimSpace(strOr(exchange["question"], "")), 220))),
				escapePipe(emptyNA(shorten(strings.TrimSpace(strOr(exchange["ruling_reason"], "")), 220))),
				escapePipe(emptyNA(shorten(strings.TrimSpace(strOr(exchange["response"], "")), 220))),
			))
		}
	}

	b.WriteString("\n### Challenges and Selection\n\n")
	if len(challenges) == 0 && countVoirDireSelectionEvents(docket) == 0 {
		b.WriteString("No for-cause challenges or peremptory strikes were recorded.\n")
	} else {
		b.WriteString("| Event | Juror ID | By | Outcome | Details |\n")
		b.WriteString("|---|---|---|---|---|\n")
		for _, raw := range challenges {
			challenge := getMap(raw)
			outcome := "pending"
			if granted, ok := challenge["granted"].(bool); ok {
				if granted {
					outcome = "granted"
				} else {
					outcome = "denied"
				}
			}
			b.WriteString(fmt.Sprintf(
				"| for-cause challenge | %s | %s | %s | %s |\n",
				escapePipe(emptyNA(strOr(challenge["juror_id"], ""))),
				escapePipe(emptyNA(strOr(challenge["by_party"], ""))),
				escapePipe(outcome),
				escapePipe(emptyNA(shorten(firstNonEmptyString(challenge, "ruling_reason", "grounds"), 220))),
			))
		}
		for _, raw := range docket {
			entry := getMap(raw)
			title := strOr(entry["title"], "")
			if title != "Peremptory strike" && title != "Jury empaneled" {
				continue
			}
			desc := strOr(entry["description"], "")
			jurorID := extractVoirDireJurorID(desc)
			by := extractVoirDireActor(desc)
			outcome := "recorded"
			if title == "Jury empaneled" {
				outcome = "selected"
			}
			b.WriteString(fmt.Sprintf(
				"| %s | %s | %s | %s | %s |\n",
				escapePipe(title),
				escapePipe(emptyNA(jurorID)),
				escapePipe(emptyNA(by)),
				escapePipe(outcome),
				escapePipe(emptyNA(shorten(desc, 220))),
			))
		}
	}

	sworn := swornJurorIDs(caseObj)
	b.WriteString("\nFinal empaneled jurors: ")
	if len(sworn) == 0 {
		b.WriteString("none.\n")
	} else {
		b.WriteString(strings.Join(sworn, ", "))
		b.WriteString(".\n")
	}
	return b.String()
}

func renderJurorRounds(caseObj map[string]any) string {
	jurors := getSlice(caseObj, "jurors")
	if len(jurors) == 0 {
		return "No juror records available (bench trial or no jury data).\n"
	}
	rounds := jurorVoteRounds(caseObj)
	if len(rounds) == 0 {
		return "No sworn juror votes were recorded.\n"
	}
	votesByRound := map[int]map[string]map[string]any{}
	for _, raw := range getSlice(caseObj, "juror_votes") {
		j := getMap(raw)
		round := asInt(j["round"])
		if round <= 0 {
			round = 1
		}
		id := strOr(j["juror_id"], "")
		if strings.TrimSpace(id) == "" {
			continue
		}
		if votesByRound[round] == nil {
			votesByRound[round] = map[string]map[string]any{}
		}
		votesByRound[round][id] = j
	}
	var b strings.Builder
	for idx, round := range rounds {
		if idx > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("### Round %d\n\n", round))
		if supplemental := supplementalInstructionBeforeRound(caseObj, round); supplemental != "" {
			b.WriteString("Supplemental instruction:\n\n")
			b.WriteString(supplemental)
			b.WriteString("\n\n")
		}
		b.WriteString(renderJurorRoundSummary(caseObj, rounds, round, votesByRound))
		b.WriteString("\n")
		b.WriteString("| # | Juror ID | Status | Model | Persona file | Vote | Damages | Confidence | Explanation |\n")
		b.WriteString("|---:|---|---|---|---|---|---:|---|---|\n")
		row := 0
		for _, raw := range jurors {
			j := getMap(raw)
			if strOr(j["status"], "") != "sworn" {
				continue
			}
			jid := strOr(j["juror_id"], "")
			if strings.TrimSpace(jid) == "" {
				jid = fmt.Sprintf("J%d", row+1)
			}
			ex := votesByRound[round][jid]
			if ex == nil {
				continue
			}
			model := firstNonEmptyString(j, "model", "llm_model")
			persona := firstNonEmptyString(j, "persona_filename", "persona_file", "persona_path")
			vote := firstNonEmptyString(ex, "vote", "verdict_for")
			damages := fmt.Sprintf("%v", ex["damages"])
			if strings.TrimSpace(damages) == "" || damages == "<nil>" {
				damages = "n/a"
			}
			conf := firstNonEmptyString(ex, "confidence", "confidence_level")
			expl := firstNonEmptyString(ex, "explanation", "summary")
			row++
			b.WriteString(fmt.Sprintf(
				"| %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
				row,
				escapePipe(jid),
				escapePipe(emptyNA(strOr(j["status"], ""))),
				escapePipe(emptyNA(model)),
				escapePipe(emptyNA(persona)),
				escapePipe(emptyNA(vote)),
				escapePipe(damages),
				escapePipe(emptyNA(conf)),
				escapePipe(emptyNA(shorten(strings.TrimSpace(expl), 220))),
			))
		}
	}
	return b.String()
}

func renderJurorRoundSummary(caseObj map[string]any, rounds []int, round int, votesByRound map[int]map[string]map[string]any) string {
	requiredVotes := requiredJurorVotes(caseObj)
	tally := tallyJurorRound(votesByRound[round])
	parts := []string{
		fmt.Sprintf(
			"Summary: %d for plaintiff, %d for defendant, required votes %d.",
			tally.plaintiff,
			tally.defendant,
			requiredVotes,
		),
	}
	if prevRound := previousJurorVoteRound(rounds, round); prevRound > 0 {
		voteChanges, damagesChanges := compareJurorRounds(votesByRound[prevRound], votesByRound[round])
		parts = append(parts, fmt.Sprintf(
			"Changes from round %d: %d vote changes, %d damages changes.",
			prevRound,
			voteChanges,
			damagesChanges,
		))
	}
	juryVerdict := getMap(caseObj["jury_verdict"])
	hungJury := getMap(caseObj["hung_jury"])
	lastRound := round == rounds[len(rounds)-1]
	switch {
	case lastRound && len(juryVerdict) > 0:
		parts = append(parts, fmt.Sprintf(
			"This round produced a verdict for %s with derived damages %v.",
			emptyNA(strOr(juryVerdict["verdict_for"], "")),
			juryVerdict["damages"],
		))
	case lastRound && len(hungJury) > 0:
		parts = append(parts, "This round ended in a hung jury.")
	default:
		parts = append(parts, "No verdict was reached in this round. Deliberation continued.")
	}
	return strings.Join(parts, " ") + "\n\n"
}

func jurorVoteRounds(caseObj map[string]any) []int {
	seen := map[int]bool{}
	rounds := make([]int, 0)
	for _, raw := range getSlice(caseObj, "juror_votes") {
		vote := getMap(raw)
		round := asInt(vote["round"])
		if round <= 0 {
			round = 1
		}
		if seen[round] {
			continue
		}
		seen[round] = true
		rounds = append(rounds, round)
	}
	sort.Ints(rounds)
	return rounds
}

func supplementalInstructionBeforeRound(caseObj map[string]any, round int) string {
	if round <= 1 {
		return ""
	}
	for _, raw := range getSlice(caseObj, "docket") {
		entry := getMap(raw)
		if strOr(entry["title"], "") != "Jury supplemental instruction" {
			continue
		}
		return strings.TrimSpace(strOr(entry["description"], ""))
	}
	return ""
}

type jurorRoundTally struct {
	plaintiff int
	defendant int
}

func tallyJurorRound(votes map[string]map[string]any) jurorRoundTally {
	var tally jurorRoundTally
	for _, vote := range votes {
		switch strings.TrimSpace(strOr(vote["vote"], "")) {
		case "plaintiff":
			tally.plaintiff++
		case "defendant":
			tally.defendant++
		}
	}
	return tally
}

func previousJurorVoteRound(rounds []int, round int) int {
	for i, candidate := range rounds {
		if candidate != round || i == 0 {
			continue
		}
		return rounds[i-1]
	}
	return 0
}

func compareJurorRounds(previous map[string]map[string]any, current map[string]map[string]any) (int, int) {
	voteChanges := 0
	damagesChanges := 0
	for jurorID, currentVote := range current {
		previousVote := previous[jurorID]
		if len(previousVote) == 0 {
			continue
		}
		currentChoice := strings.TrimSpace(strOr(currentVote["vote"], ""))
		previousChoice := strings.TrimSpace(strOr(previousVote["vote"], ""))
		if currentChoice != previousChoice {
			voteChanges++
			continue
		}
		if currentChoice != "plaintiff" {
			continue
		}
		currentDamages, currentOK := asFloat(currentVote["damages"])
		previousDamages, previousOK := asFloat(previousVote["damages"])
		if currentOK && previousOK && currentDamages != previousDamages {
			damagesChanges++
		}
	}
	return voteChanges, damagesChanges
}

func requiredJurorVotes(caseObj map[string]any) int {
	juryCfg := getMap(caseObj["jury_configuration"])
	if required := asInt(juryCfg["minimum_concurring"]); required > 0 {
		return required
	}
	juryVerdict := getMap(caseObj["jury_verdict"])
	if required := asInt(juryVerdict["required_votes"]); required > 0 {
		return required
	}
	return len(swornJurorIDs(caseObj))
}

func renderQuestionnaireRecord(caseObj map[string]any) string {
	questions := getSlice(caseObj, "juror_questionnaire")
	responses := getSlice(caseObj, "juror_questionnaire_responses")
	if len(questions) == 0 && len(responses) == 0 {
		return "No juror questionnaire record appears in the case state.\n"
	}
	questionTextByID := map[string]string{}
	for _, raw := range questions {
		question := getMap(raw)
		questionID := strings.TrimSpace(strOr(question["question_id"], ""))
		if questionID == "" {
			continue
		}
		questionTextByID[questionID] = strings.TrimSpace(strOr(question["prompt"], ""))
	}
	if len(responses) == 0 {
		return "The court issued a questionnaire, but no candidate responses were recorded.\n"
	}
	var b strings.Builder
	for _, raw := range responses {
		response := getMap(raw)
		jurorID := strings.TrimSpace(strOr(response["juror_id"], ""))
		answers := getSlice(response, "answers")
		b.WriteString("#### ")
		b.WriteString(emptyNA(jurorID))
		b.WriteString("\n\n")
		if len(answers) == 0 {
			b.WriteString("No questionnaire answers recorded.\n\n")
			continue
		}
		b.WriteString("| Question | Answer |\n")
		b.WriteString("|---|---|\n")
		for _, answerRaw := range answers {
			answer := getMap(answerRaw)
			questionID := strings.TrimSpace(strOr(answer["question_id"], ""))
			questionText := strings.TrimSpace(questionTextByID[questionID])
			if questionText == "" {
				questionText = questionID
			}
			b.WriteString(fmt.Sprintf(
				"| %s | %s |\n",
				escapePipe(emptyNA(shorten(questionText, 220))),
				escapePipe(emptyNA(shorten(strings.TrimSpace(strOr(answer["answer"], "")), 220))),
			))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func countVoirDireSelectionEvents(docket []any) int {
	count := 0
	for _, raw := range docket {
		entry := getMap(raw)
		title := strOr(entry["title"], "")
		if title == "Peremptory strike" || title == "Jury empaneled" {
			count++
		}
	}
	return count
}

func swornJurorIDs(caseObj map[string]any) []string {
	ids := make([]string, 0)
	for _, raw := range getSlice(caseObj, "jurors") {
		juror := getMap(raw)
		if strOr(juror["status"], "") != "sworn" {
			continue
		}
		jurorID := strings.TrimSpace(strOr(juror["juror_id"], ""))
		if jurorID == "" {
			continue
		}
		ids = append(ids, jurorID)
	}
	return ids
}

func extractVoirDireJurorID(desc string) string {
	for _, field := range strings.Fields(desc) {
		if strings.HasPrefix(field, "J") {
			return strings.Trim(field, ",;")
		}
	}
	return ""
}

func extractVoirDireActor(desc string) string {
	desc = strings.TrimSpace(desc)
	if strings.HasPrefix(desc, "plaintiff ") || strings.HasPrefix(desc, "plaintiff:") {
		return "plaintiff"
	}
	if strings.HasPrefix(desc, "defendant ") || strings.HasPrefix(desc, "defendant:") {
		return "defendant"
	}
	return ""
}

func renderEvidenceTable(caseObj map[string]any, docket []any) string {
	var b strings.Builder
	b.WriteString("| Source | ID | Status | Details |\n")
	b.WriteString("|---|---|---|---|\n")
	files := getSlice(caseObj, "case_files")
	fileStatuses := caseFileStatuses(caseObj)
	for _, raw := range files {
		f := getMap(raw)
		fileID := strOr(f["file_id"], "")
		label := strOr(f["label"], "")
		orig := strOr(f["original_name"], "")
		details := strings.TrimSpace(strings.Join([]string{label, orig}, " | "))
		status := strings.TrimSpace(fileStatuses[fileID])
		if status == "" {
			status = "recorded"
		}
		b.WriteString(fmt.Sprintf("| case_file | %s | %s | %s |\n", escapePipe(emptyNA(fileID)), escapePipe(status), escapePipe(emptyNA(details))))
	}
	for _, raw := range docket {
		e := getMap(raw)
		title := strOr(e["title"], "")
		if !strings.HasPrefix(title, "Exhibit ") {
			continue
		}
		desc := strOr(e["description"], "")
		status := "excluded"
		if strings.Contains(title, " - admitted") {
			status = "admitted"
		}
		b.WriteString(fmt.Sprintf("| exhibit | %s | %s | %s |\n", escapePipe(title), status, escapePipe(shorten(desc, 240))))
	}
	if len(files) == 0 && countExhibitRows(docket) == 0 {
		return "No evidence files or exhibits were recorded.\n"
	}
	return b.String()
}

func renderTechnicalReportsTable(caseObj map[string]any) string {
	reports := getSlice(caseObj, "technical_reports")
	if len(reports) == 0 {
		return "No technical reports submitted.\n"
	}
	var b strings.Builder
	b.WriteString("| # | Type | Party | Report ID | Title | Summary | Method notes | Limitations | File ID |\n")
	b.WriteString("|---:|---|---|---|---|---|---|---|---|\n")
	for i, raw := range reports {
		r := getMap(raw)
		party := firstNonEmptyString(r, "party")
		reportID := firstNonEmptyString(r, "report_id")
		title := firstNonEmptyString(r, "title")
		summary := firstNonEmptyString(r, "summary")
		methodNotes := firstNonEmptyString(r, "method_notes")
		limitations := firstNonEmptyString(r, "limitations")
		fileID := firstNonEmptyString(r, "file_id")
		b.WriteString(fmt.Sprintf(
			"| %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			i+1,
			"technical",
			escapePipe(emptyNA(party)),
			escapePipe(emptyNA(reportID)),
			escapePipe(emptyNA(title)),
			escapePipe(emptyNA(shorten(summary, 220))),
			escapePipe(emptyNA(shorten(methodNotes, 140))),
			escapePipe(emptyNA(shorten(limitations, 140))),
			escapePipe(emptyNA(fileID)),
		))
	}
	return b.String()
}

func countExhibitRows(docket []any) int {
	n := 0
	for _, raw := range docket {
		title := strOr(getMap(raw)["title"], "")
		if strings.HasPrefix(title, "Exhibit ") {
			n++
		}
	}
	return n
}

func firstDocketDescriptionByTitle(docket []any, title string) string {
	for _, raw := range docket {
		entry := getMap(raw)
		if strOr(entry["title"], "") == title {
			return strOr(entry["description"], "")
		}
	}
	return ""
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(strOr(m[k], "")); s != "" {
			return s
		}
	}
	return ""
}

func emptyNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return "n/a"
	}
	return s
}

func shorten(s string, max int) string {
	t := strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if utf8.RuneCountInString(t) <= max || max <= 3 {
		return t
	}
	return string([]rune(t)[:max-3]) + "..."
}

func shortenAtWord(s string, max int) string {
	t := strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if utf8.RuneCountInString(t) <= max || max <= 3 {
		return t
	}
	end := max - 3
	if end < 1 {
		return "..."
	}
	cut := string([]rune(t)[:end])
	if idx := strings.LastIndexByte(cut, ' '); idx > end/2 {
		cut = cut[:idx]
	}
	return cut + "..."
}

func getMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func getSlice(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	if v == nil {
		return []any{}
	}
	return v
}

func strOr(v any, fallback string) string {
	s, _ := v.(string)
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
