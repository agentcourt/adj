package quick

import (
	"fmt"
	"strings"

	"github.com/jsmorph/adj/common/promptfile"
)

const (
	defaultLawyerPrompt = `Address each required part of the proposition and identify the evidence supporting it. Support factual claims with case documents or identified sources. Address authentication when relevant. Use an available method when it can resolve a material question, and report the method and result.`

	defaultSearchPromptOn = `Web search is enabled. Search when a material public fact is missing from the case record, and identify the source for each fact used. Use available browser or computer tools when they help assess relevant source content.`

	defaultSearchPromptOff = `Native web search is disabled. Base the argument on the immutable case documents and the opposing argument when available. Available methods may inspect, transform, or test the supplied evidence.`

	defaultProponentPrompt = `Quick adjudication case

Proposition:
{{PROPOSITION}}

Evidence standard:
{{EVIDENCE_STANDARD}}

{{LAWYER}}

{{WEB_SEARCH}}

You represent the proponent. Submit one argument; no later argument is available.

{{DOCUMENT_NOTICE}}

Call submit_decision exactly once with kind=tool, tool_name=submit_argument, and payload.text containing your argument.`

	defaultOpponentPrompt = `Quick adjudication case

Proposition:
{{PROPOSITION}}

Evidence standard:
{{EVIDENCE_STANDARD}}

{{LAWYER}}

{{WEB_SEARCH}}

You represent the opponent. Test each required part of the proposition and the proponent's supporting evidence. Identify missing proof, contradictions, source limits, authentication limits, timing problems, and unsupported inferences. You need not demonstrate the inverse proposition. Submit one argument; no later argument is available.

Proponent argument:
{{PROPONENT_ARGUMENT}}

{{DOCUMENT_NOTICE}}

Call submit_decision exactly once with kind=tool, tool_name=submit_argument, and payload.text containing your argument.`

	defaultDocumentNoticePrompt = `The immutable case documents are available through list_evidence, stat_evidence, and read_evidence_range.`
	defaultObserverPrompt       = `Observe quick adjudication case {{CASE_ID}}, run {{RUN_ID}}. The proposition is: {{PROPOSITION}}`
	defaultCouncilSystemPrompt  = `You are council member {{MEMBER_ID}} in a quick adjudication. Act as a neutral factfinder. Decide whether the evidence satisfies the stated standard for each required part of the proposition. Treat the proposition and lawyer arguments as claims. Explain the decisive evidence or evidentiary gap in the rationale.

{{PERSONA}}`
	defaultCouncilCasePrompt = `Evidence standard:
{{EVIDENCE_STANDARD}}

Proposition:
{{PROPOSITION}}

Proponent argument:
{{PROPONENT_ARGUMENT}}

Opponent argument:
{{OPPONENT_ARGUMENT}}

Immutable case documents:`
	defaultCouncilDocumentPrompt    = `Document "{{DOCUMENT_PATH}}" ({{MEDIA_TYPE}}, {{SIZE_BYTES}} bytes, SHA-256 {{SHA256}}):`
	defaultCouncilNoDocumentsPrompt = `No documents were provided.`
	defaultCouncilSubmitPrompt      = `Call submit_council_vote exactly once with vote=demonstrated or vote=not_demonstrated and a concise rationale.`
	defaultCouncilRepairPrompt      = `The previous response was invalid: {{ERROR}}. Call submit_council_vote exactly once with a valid vote and concise rationale.`
	defaultCouncilPreflightPrompt   = `Availability check for council seat {{MEMBER_ID}}, model {{MODEL}}, persona file {{PERSONA_FILE}}. Call submit_council_vote exactly once with vote=demonstrated and rationale=READY.`
	defaultSubmitArgumentToolPrompt = `Submit the one argument allowed for this quick adjudication turn.`
	defaultWorkNotesToolPrompt      = `Record private lawyer work notes.`
	defaultGetCaseToolPrompt        = `Return the visible quick adjudication record.`
	defaultGetResultToolPrompt      = `Return the final result or pending status.`
	defaultListEvidenceToolPrompt   = `List immutable case documents.`
	defaultStatEvidenceToolPrompt   = `Return document metadata.`
	defaultReadEvidenceToolPrompt   = `Read a document byte range as base64.`
	defaultCaseStatusToolPrompt     = `Return the current case phase and turn.`
	defaultCouncilVoteToolPrompt    = `Submit this council member's vote.`
)

var quickPromptSpecs = []promptfile.Spec{
	{ID: "lawyer.common", Name: "quick shared lawyer", ConventionalPath: "prompts/quick/lawyers/common.md", RelativePath: "lawyers/common.md", Fallback: defaultLawyerPrompt, Tokens: []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}", "{{DOCUMENT_NOTICE}}"}},
	{ID: "lawyer.proponent", Name: "quick proponent", ConventionalPath: "prompts/quick/lawyers/for.md", RelativePath: "lawyers/for.md", Fallback: defaultProponentPrompt, Tokens: []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}", "{{PROPONENT_ARGUMENT}}", "{{DOCUMENT_NOTICE}}", "{{LAWYER}}", "{{WEB_SEARCH}}"}},
	{ID: "lawyer.opponent", Name: "quick opponent", ConventionalPath: "prompts/quick/lawyers/against.md", RelativePath: "lawyers/against.md", Fallback: defaultOpponentPrompt, Tokens: []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}", "{{PROPONENT_ARGUMENT}}", "{{DOCUMENT_NOTICE}}", "{{LAWYER}}", "{{WEB_SEARCH}}"}},
	{ID: "lawyer.document_notice", Name: "quick lawyer document notice", ConventionalPath: "prompts/quick/lawyers/document-notice.md", RelativePath: "lawyers/document-notice.md", Fallback: defaultDocumentNoticePrompt, Tokens: []string{"{{DOCUMENT_COUNT}}"}},
	{ID: "search.enabled", Name: "quick search enabled", ConventionalPath: "prompts/quick/search/on.md", RelativePath: "search/on.md", Fallback: defaultSearchPromptOn, Tokens: []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}"}},
	{ID: "search.disabled", Name: "quick search disabled", ConventionalPath: "prompts/quick/search/off.md", RelativePath: "search/off.md", Fallback: defaultSearchPromptOff, Tokens: []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}"}},
	{ID: "observer", Name: "quick observer", ConventionalPath: "prompts/quick/observer.md", RelativePath: "observer.md", Fallback: defaultObserverPrompt, Tokens: []string{"{{CASE_ID}}", "{{RUN_ID}}", "{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}"}},
	{ID: "council.system", Name: "quick council", ConventionalPath: "prompts/quick/council/system.md", RelativePath: "council/system.md", Fallback: defaultCouncilSystemPrompt, Tokens: []string{"{{MEMBER_ID}}", "{{PERSONA}}"}},
	{ID: "council.case", Name: "quick council case", ConventionalPath: "prompts/quick/council/case.md", RelativePath: "council/case.md", Fallback: defaultCouncilCasePrompt, Tokens: []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}", "{{PROPONENT_ARGUMENT}}", "{{OPPONENT_ARGUMENT}}"}},
	{ID: "council.document", Name: "quick council document", ConventionalPath: "prompts/quick/council/document.md", RelativePath: "council/document.md", Fallback: defaultCouncilDocumentPrompt, Tokens: []string{"{{DOCUMENT_PATH}}", "{{MEDIA_TYPE}}", "{{SIZE_BYTES}}", "{{SHA256}}"}},
	{ID: "council.no_documents", Name: "quick council no documents", ConventionalPath: "prompts/quick/council/no-documents.md", RelativePath: "council/no-documents.md", Fallback: defaultCouncilNoDocumentsPrompt},
	{ID: "council.submit", Name: "quick council submission", ConventionalPath: "prompts/quick/council/submit.md", RelativePath: "council/submit.md", Fallback: defaultCouncilSubmitPrompt},
	{ID: "council.repair", Name: "quick council repair", ConventionalPath: "prompts/quick/council/repair.md", RelativePath: "council/repair.md", Fallback: defaultCouncilRepairPrompt, Tokens: []string{"{{ERROR}}"}},
	{ID: "council.preflight", Name: "quick council preflight", ConventionalPath: "prompts/quick/council/preflight.md", RelativePath: "council/preflight.md", Fallback: defaultCouncilPreflightPrompt, Tokens: []string{"{{MEMBER_ID}}", "{{MODEL}}", "{{PERSONA_FILE}}"}},
	{ID: "tool.submit_argument", Name: "quick submit-argument tool", ConventionalPath: "prompts/quick/tools/submit-argument.md", RelativePath: "tools/submit-argument.md", Fallback: defaultSubmitArgumentToolPrompt},
	{ID: "tool.send_work_notes", Name: "quick work-notes tool", ConventionalPath: "prompts/quick/tools/send-work-notes.md", RelativePath: "tools/send-work-notes.md", Fallback: defaultWorkNotesToolPrompt},
	{ID: "tool.get_case", Name: "quick get-case tool", ConventionalPath: "prompts/quick/tools/get-case.md", RelativePath: "tools/get-case.md", Fallback: defaultGetCaseToolPrompt},
	{ID: "tool.get_case_result", Name: "quick get-result tool", ConventionalPath: "prompts/quick/tools/get-case-result.md", RelativePath: "tools/get-case-result.md", Fallback: defaultGetResultToolPrompt},
	{ID: "tool.list_evidence", Name: "quick list-evidence tool", ConventionalPath: "prompts/quick/tools/list-evidence.md", RelativePath: "tools/list-evidence.md", Fallback: defaultListEvidenceToolPrompt},
	{ID: "tool.stat_evidence", Name: "quick stat-evidence tool", ConventionalPath: "prompts/quick/tools/stat-evidence.md", RelativePath: "tools/stat-evidence.md", Fallback: defaultStatEvidenceToolPrompt},
	{ID: "tool.read_evidence_range", Name: "quick read-evidence tool", ConventionalPath: "prompts/quick/tools/read-evidence-range.md", RelativePath: "tools/read-evidence-range.md", Fallback: defaultReadEvidenceToolPrompt},
	{ID: "tool.case_status", Name: "quick case-status tool", ConventionalPath: "prompts/quick/tools/case-status.md", RelativePath: "tools/case-status.md", Fallback: defaultCaseStatusToolPrompt},
	{ID: "tool.council_vote", Name: "quick council-vote tool", ConventionalPath: "prompts/quick/tools/council-vote.md", RelativePath: "tools/council-vote.md", Fallback: defaultCouncilVoteToolPrompt},
}

type quickPromptSet map[string]string

func loadQuickPrompts(cfg Config) (quickPromptSet, error) {
	resolved, err := promptfile.Resolve(quickPromptSpecs, cfg.PromptFiles, cfg.PromptDir)
	if err != nil {
		return nil, err
	}
	observer, err := promptfile.Render(
		"quick observer",
		resolved["observer"],
		"{{CASE_ID}}", cfg.CaseID,
		"{{RUN_ID}}", cfg.RunID,
		"{{PROPOSITION}}", cfg.Proposition,
		"{{EVIDENCE_STANDARD}}", cfg.EvidenceStandard,
	)
	if err != nil {
		return nil, err
	}
	resolved["observer.rendered"] = observer
	return quickPromptSet(resolved), nil
}

func (r *runner) renderPrompt(id string, replacements ...string) (string, error) {
	source := r.promptSource(id)
	if source == "" {
		return "", fmt.Errorf("quick prompt %q is unavailable", id)
	}
	return promptfile.Render("quick "+id, source, replacements...)
}

func (r *runner) promptSource(id string) string {
	if source := r.cfg.prompts[id]; source != "" {
		return source
	}
	for _, spec := range quickPromptSpecs {
		if spec.ID == id {
			return spec.Fallback
		}
	}
	return ""
}

func (r *runner) lawyerPromptLocked(role string) (string, error) {
	id := "lawyer.proponent"
	if role == "defendant" {
		id = "lawyer.opponent"
	}
	proponentArgument := ""
	if len(r.transcript.Arguments) > 0 {
		proponentArgument = r.transcript.Arguments[0].Text
	}
	documentNotice := ""
	if len(r.documents.Files) > 0 {
		var err error
		documentNotice, err = r.renderPrompt("lawyer.document_notice", "{{DOCUMENT_COUNT}}", fmt.Sprintf("%d", len(r.documents.Files)))
		if err != nil {
			return "", err
		}
	}
	lawyer, err := r.renderPrompt(
		"lawyer.common",
		"{{PROPOSITION}}", r.cfg.Proposition,
		"{{EVIDENCE_STANDARD}}", r.cfg.EvidenceStandard,
		"{{DOCUMENT_NOTICE}}", documentNotice,
	)
	if err != nil {
		return "", err
	}
	searchID := "search.disabled"
	if r.cfg.LawyerWebSearchEnabled {
		searchID = "search.enabled"
	}
	search, err := r.renderPrompt(
		searchID,
		"{{PROPOSITION}}", r.cfg.Proposition,
		"{{EVIDENCE_STANDARD}}", r.cfg.EvidenceStandard,
	)
	if err != nil {
		return "", err
	}
	return r.renderPrompt(
		id,
		"{{PROPOSITION}}", r.cfg.Proposition,
		"{{EVIDENCE_STANDARD}}", r.cfg.EvidenceStandard,
		"{{PROPONENT_ARGUMENT}}", proponentArgument,
		"{{DOCUMENT_NOTICE}}", documentNotice,
		"{{LAWYER}}", lawyer,
		"{{WEB_SEARCH}}", search,
	)
}

func (r *runner) observerPrompt() string {
	if rendered := strings.TrimSpace(r.cfg.prompts["observer.rendered"]); rendered != "" {
		return rendered
	}
	return strings.TrimSpace(strings.NewReplacer(
		"{{CASE_ID}}", r.cfg.CaseID,
		"{{RUN_ID}}", r.cfg.RunID,
		"{{PROPOSITION}}", r.cfg.Proposition,
		"{{EVIDENCE_STANDARD}}", r.cfg.EvidenceStandard,
	).Replace(defaultObserverPrompt))
}

func (r *runner) councilPrompt(member CouncilMember) (string, error) {
	return r.renderPrompt(
		"council.system",
		"{{MEMBER_ID}}", member.MemberID,
		"{{PERSONA}}", strings.TrimSpace(member.PersonaText),
	)
}

func (r *runner) toolPrompt(id string) string {
	return strings.TrimSpace(r.promptSource("tool." + id))
}
