package prompts

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jsmorph/adj/common/promptfile"
)

const ConventionalDir = "prompts/adc"

const (
	ComplaintSystemID              = "complaint.system"
	ComplaintUserID                = "complaint.user"
	ComplaintRepairID              = "complaint.repair"
	CasePacketSystemID             = "case-packet.system"
	CasePacketUserID               = "case-packet.user"
	CasePacketRepairID             = "case-packet.repair"
	PlaintiffStrategySystemID      = "strategy.plaintiff.system"
	PlaintiffStrategyUserID        = "strategy.plaintiff.user"
	DefenseStrategySystemID        = "strategy.defense.system"
	DefenseStrategyUserID          = "strategy.defense.user"
	PropositionProponentStrategyID = "strategy.proposition.proponent"
	PropositionOpponentStrategyID  = "strategy.proposition.opponent"
	PlaintiffInstructionsID        = "role.plaintiff.instructions"
	PlaintiffRuntimeID             = "role.plaintiff.runtime"
	DefendantInstructionsID        = "role.defendant.instructions"
	DefendantRuntimeID             = "role.defendant.runtime"
	ClerkInstructionsID            = "role.clerk.instructions"
	ClerkRuntimeID                 = "role.clerk.runtime"
	JudgeInstructionsID            = "role.judge.instructions"
	JudgeRuntimeID                 = "role.judge.runtime"
	JurorInstructionsID            = "role.juror.instructions"
	JurorRuntimeID                 = "role.juror.runtime"
	RuntimeSystemID                = "runtime.system"
	RuntimeOpportunityID           = "runtime.opportunity"
	RuntimeJurorID                 = "runtime.juror"
	RuntimeJurorDeliberationID     = "runtime.juror-deliberation"
	RuntimeTurnID                  = "runtime.turn"
	RuntimeCorrectionID            = "runtime.correction"
	RuntimeCorrectionLocalRuleID   = "runtime.correction.local-rule-limit"
	RuntimeCorrectionFieldsID      = "runtime.correction.required-fields"
	RuntimeCorrectionCompleteID    = "runtime.correction.already-complete"
	RuntimeCorrectionNoToolID      = "runtime.correction.no-tool"
	RuntimeCorrectionMultipleID    = "runtime.correction.multiple-tools"
	RuntimeCorrectionDisallowedID  = "runtime.correction.disallowed-tool"
	RuntimeCorrectionSupportID     = "runtime.correction.support-budget-exhausted"
	RuntimeCorrectionFixedID       = "runtime.correction.fixed-fields"
	RuntimeCorrectionMalformedID   = "runtime.correction.malformed-arguments"
	RuntimeResultImportUploadID    = "runtime.result.import-upload-fields"
	RuntimeResultUnknownFileID     = "runtime.result.unknown-case-file"
	RuntimeResultTextExtensionID   = "runtime.result.unsupported-case-text-extension"
	RuntimeResultUnreadableID      = "runtime.result.case-file-missing-readable-path"
	RuntimeResultNonUTF8ID         = "runtime.result.case-file-not-utf8"
	RuntimeResultUnattachableID    = "runtime.result.case-file-missing-attachment-path"
	RuntimeResultAlreadyOfferedID  = "runtime.result.case-file-already-offered"
	RuntimeCaseFileAttachmentID    = "runtime.case-file-attachment"
	RuntimeExternalRoleID          = "runtime.external-role"
	ReportSummarySystemID          = "report.summary.system"
	ReportSummaryUserID            = "report.summary.user"
	ReportRepairSystemID           = "report.repair.system"
	ReportRepairUserID             = "report.repair.user"
	DirectToolDescriptionPrefix    = "tool-description.direct."
	ImportSourceFilenameID         = "tool-description.direct.import_case_file.property.source_filename"
	ImportOriginalNameID           = "tool-description.direct.import_case_file.property.original_name"
	ImportContentBase64ID          = "tool-description.direct.import_case_file.property.content_base64"
	RoleAPICaseStatusID            = "tool-description.role-api.case-status"
	RoleAPIWorkNotesID             = "tool-description.role-api.send-work-notes"
	RoleAPIReadFileBytesID         = "tool-description.role-api.read-case-file-bytes"
	RoleAPISubmitDecisionID        = "tool-description.role-api.submit-decision"
	RoleAPIGetCaseID               = "tool-description.role-api.get-case"
	RoleAPIExplainDecisionsID      = "tool-description.role-api.explain-decisions"
	RoleAPIListCaseFilesID         = "tool-description.role-api.list-case-files"
	RoleAPIReadCaseTextFileID      = "tool-description.role-api.read-case-text-file"
	RoleAPIRequestCaseFileID       = "tool-description.role-api.request-case-file"
	RoleAPIGetJurorContextID       = "tool-description.role-api.get-juror-context"
	GenericToolCardID              = "tool-card.generic"
)

type Options struct {
	PromptDir   string
	PromptFiles map[string]string
}

type Entry struct {
	ID           string
	RelativePath string
	Tokens       []string
}

type definition struct {
	Entry
	Fallback string
}

type Catalog struct {
	sources map[string]string
}

var fixedDefinitions = []definition{
	def(ComplaintSystemID, "complaint/system.md", "Draft one supported civil complaint as Markdown.", nil),
	def(ComplaintUserID, "complaint/user.md", "Selected court profile:\n{{COURT}}\n\nSituation markdown follows.\n\n{{SOURCE}}\n\nLinked local references:\n{{LINKED_FILES}}", []string{"{{COURT}}", "{{SOURCE}}", "{{LINKED_FILES}}"}),
	def(ComplaintRepairID, "complaint/repair.md", "Rewrite the complaint as valid Markdown. Correct this error: {{ERROR}}", []string{"{{ERROR}}"}),
	def(CasePacketSystemID, "case-packet/system.md", "Return one supported civil case packet as strict JSON.", nil),
	def(CasePacketUserID, "case-packet/user.md", "Court:\n{{COURT}}\n\nComplaint:\n{{COMPLAINT}}\n\nLinked files:\n{{LINKED_FILES}}", []string{"{{COURT}}", "{{COMPLAINT}}", "{{LINKED_FILES}}"}),
	def(CasePacketRepairID, "case-packet/repair.md", "Return corrected strict JSON for this error: {{ERROR}}", []string{"{{ERROR}}"}),
	def(PlaintiffStrategySystemID, "strategy/plaintiff/system.md", "Prepare a concrete litigation plan for plaintiff's counsel as Markdown.", nil),
	def(PlaintiffStrategyUserID, "strategy/plaintiff/user.md", strategyFallback("plaintiff"), strategyTokens()),
	def(DefenseStrategySystemID, "strategy/defense/system.md", "Prepare a concrete litigation plan for defense counsel as Markdown.", nil),
	def(DefenseStrategyUserID, "strategy/defense/user.md", strategyFallback("defense"), strategyTokens()),
	def(PropositionProponentStrategyID, "strategy/proposition/proponent.md", "# Proponent Strategy\n\nProposition:\n\n{{PROPOSITION}}\n\nEvidence standard: `{{EVIDENCE_STANDARD}}`.  The Proponent bears the burden to demonstrate the proposition.  Use the imported documents and the trial record to address the proposition, and seek declaratory judgment with no monetary damages.", propositionStrategyTokens()),
	def(PropositionOpponentStrategyID, "strategy/proposition/opponent.md", "# Opponent Strategy\n\nProposition:\n\n{{PROPOSITION}}\n\nEvidence standard: `{{EVIDENCE_STANDARD}}`.  The Opponent may prevail by showing that the Proponent has not met the burden.  Use the imported documents and the trial record to address the proposition, and seek declaratory judgment with no monetary damages.", propositionStrategyTokens()),
	def(PlaintiffInstructionsID, "roles/plaintiff/instructions.md", "Act as plaintiff's counsel within the available procedures.", nil),
	def(PlaintiffRuntimeID, "roles/plaintiff/runtime.md", "Represent the plaintiff using these court rules and strategy.\n\nCourt rules:\n{{COURT_RULES}}\n\nStrategy:\n{{STRATEGY}}", []string{"{{COURT_RULES}}", "{{STRATEGY}}"}),
	def(DefendantInstructionsID, "roles/defendant/instructions.md", "Act as defense counsel within the available procedures.", nil),
	def(DefendantRuntimeID, "roles/defendant/runtime.md", "Represent the defendant using these court rules and strategy.\n\nCourt rules:\n{{COURT_RULES}}\n\nStrategy:\n{{STRATEGY}}", []string{"{{COURT_RULES}}", "{{STRATEGY}}"}),
	def(ClerkInstructionsID, "roles/clerk/instructions.md", "Act as the clerk for case administration.", nil),
	def(ClerkRuntimeID, "roles/clerk/runtime.md", "Administer the case under these court rules:\n{{COURT_RULES}}", []string{"{{COURT_RULES}}"}),
	def(JudgeInstructionsID, "roles/judge/instructions.md", "Act as the judge for rulings, trial control, and judgment.", nil),
	def(JudgeRuntimeID, "roles/judge/runtime.md", "Decide the case under these court rules:\n{{COURT_RULES}}", []string{"{{COURT_RULES}}"}),
	def(JurorInstructionsID, "roles/juror/instructions.md", "Act as one juror for voir dire and deliberation.", nil),
	def(JurorRuntimeID, "roles/juror/runtime.md", "Act as one juror and decide from the admitted record and the court's instructions.", nil),
	def(RuntimeSystemID, "runtime/system.md", "Role: {{ROLE}}\nRole prompt preamble: {{PREAMBLE}}\nInstructions: {{INSTRUCTIONS}}\nAllowed actions: {{ALLOWED_ACTIONS}}\nUse only listed tools with precise payloads. When you decide to act, call exactly one tool rather than replying with prose.\nCurrent view:\n{{VIEW}}", []string{"{{ROLE}}", "{{PREAMBLE}}", "{{INSTRUCTIONS}}", "{{ALLOWED_ACTIONS}}", "{{VIEW}}"}),
	def(RuntimeOpportunityID, "runtime/opportunity.md", "Current opportunity:\n{{ACTOR_MESSAGE}}\nObjective: {{OBJECTIVE}}\nPhase: {{PHASE}}\nAllowed actions: {{ALLOWED_ACTIONS}}\nReference tools: {{REFERENCE_TOOLS}}\nConstraints: {{CONSTRAINTS}}\nPass action: {{PASS_ACTION}}", []string{"{{ACTOR_MESSAGE}}", "{{OBJECTIVE}}", "{{PHASE}}", "{{ALLOWED_ACTIONS}}", "{{REFERENCE_TOOLS}}", "{{CONSTRAINTS}}", "{{PASS_ACTION}}"}),
	def(RuntimeJurorID, "runtime/juror.md", "Role: {{ROLE}}\nRole prompt preamble: {{PREAMBLE}}\nInstructions: {{INSTRUCTIONS}}\nAllowed actions: {{ALLOWED_ACTIONS}}\nUse only listed tools with precise payloads. When you decide to act, call exactly one tool rather than replying with prose.\nAssigned juror: {{JUROR_ID}}\nJuror identity:\n{{PERSONA}}\n{{DELIBERATION}}", []string{"{{ROLE}}", "{{PREAMBLE}}", "{{INSTRUCTIONS}}", "{{ALLOWED_ACTIONS}}", "{{JUROR_ID}}", "{{PERSONA}}", "{{DELIBERATION}}"}),
	def(RuntimeJurorDeliberationID, "runtime/juror-deliberation.md", "Deliberation round: {{ROUND}}\n\nTrial transcript:\n{{TRANSCRIPT}}\n\nEvidence review:\nUse the transcript as the starting point. Use the case-view and case-file tools to inspect admitted exhibits and visible files when their contents or provenance affect the verdict. Use local tools to analyze visible record material. Base the verdict on the record and the court's instructions.\n\nJudge's instructions:\n{{JURY_INSTRUCTIONS}}\n\nPrior ballot round:\n{{PRIOR_BALLOT}}", []string{"{{ROUND}}", "{{TRANSCRIPT}}", "{{JURY_INSTRUCTIONS}}", "{{PRIOR_BALLOT}}"}),
	def(RuntimeTurnID, "runtime/turn.md", "{{BASE_PROMPT}}\n\nTool payloads:\n{{TOOL_SCHEMAS}}\n\nApplicable tool guidance:\n{{TOOL_GUIDANCE}}", []string{"{{BASE_PROMPT}}", "{{TOOL_SCHEMAS}}", "{{TOOL_GUIDANCE}}"}),
	def(RuntimeCorrectionID, "runtime/correction.md", "Your prior tool call was rejected. You are acting as {{ROLE}}.\n\nRejected actions:\n{{ISSUES}}\n\nCorrection guidance:\n{{GUIDANCE}}\n\nRequired fields:\n{{REQUIRED_FIELDS}}\n\nCall one allowed action. Allowed actions: {{ALLOWED_ACTIONS}}. Pass action: {{PASS_ACTION}}.", []string{"{{ROLE}}", "{{ISSUES}}", "{{GUIDANCE}}", "{{REQUIRED_FIELDS}}", "{{ALLOWED_ACTIONS}}", "{{PASS_ACTION}}"}),
	def(RuntimeCorrectionLocalRuleID, "runtime/correction/local-rule-limit.md", "A local-rule limit blocked that action. Pick a different legal action for this turn.", nil),
	def(RuntimeCorrectionFieldsID, "runtime/correction/required-fields.md", "Provide every required argument exactly as defined for the tool.", nil),
	def(RuntimeCorrectionCompleteID, "runtime/correction/already-complete.md", "That step is already complete in this case. Choose the next procedural step.", nil),
	def(RuntimeCorrectionNoToolID, "runtime/correction/no-tool.md", "Choose one allowed action or use a reference tool now. Pass action: {{PASS_ACTION}}.", []string{"{{PASS_ACTION}}"}),
	def(RuntimeCorrectionMultipleID, "runtime/correction/multiple-tools.md", "Call exactly one tool for this opportunity.", nil),
	def(RuntimeCorrectionDisallowedID, "runtime/correction/disallowed-tool.md", "Choose one allowed action for this opportunity, or use a listed reference tool.", nil),
	def(RuntimeCorrectionSupportID, "runtime/correction/support-budget-exhausted.md", "You have inspected enough record material for this opportunity. Submit a legal decision now, or pass if passing is allowed.", nil),
	def(RuntimeCorrectionFixedID, "runtime/correction/fixed-fields.md", "This opportunity fixes {{FIXED_FIELDS}}. Keep those values and supply only the remaining fields.", []string{"{{FIXED_FIELDS}}"}),
	def(RuntimeCorrectionMalformedID, "runtime/correction/malformed-arguments.md", "Your previous tool-call arguments were malformed. Call the tool again with valid JSON arguments.", nil),
	def(RuntimeResultImportUploadID, "runtime/result/import-upload-fields.md", "To import a new file, submit original_name and base64-encoded content in content_base64. Do not refer to a host path.", nil),
	def(RuntimeResultUnknownFileID, "runtime/result/unknown-case-file.md", "Use a case file identifier, not a filename. {{FILE_ID}} is not a known file_id. Available file_id values: {{AVAILABLE_FILE_IDS}}.", []string{"{{FILE_ID}}", "{{AVAILABLE_FILE_IDS}}"}),
	def(RuntimeResultTextExtensionID, "runtime/result/unsupported-case-text-extension.md", "read_case_text_file only supports .md, .txt, .pem, and .b64 files. {{CASE_FILE}} has extension {{EXTENSION}}.", []string{"{{CASE_FILE}}", "{{EXTENSION}}"}),
	def(RuntimeResultUnreadableID, "runtime/result/case-file-missing-readable-path.md", "This case file has no stored path and cannot be read.", nil),
	def(RuntimeResultNonUTF8ID, "runtime/result/case-file-not-utf8.md", "{{CASE_FILE}} could not be read as UTF-8 text.", []string{"{{CASE_FILE}}"}),
	def(RuntimeResultUnattachableID, "runtime/result/case-file-missing-attachment-path.md", "This case file has no stored path and cannot be attached.", nil),
	def(RuntimeResultAlreadyOfferedID, "runtime/result/case-file-already-offered.md", "Choose a case file that this side has not already offered as an exhibit.", nil),
	def(RuntimeCaseFileAttachmentID, "runtime/case-file-attachment.md", "Requested case file {{FILENAME}}. Review it and continue with the current opportunity.", []string{"{{FILENAME}}"}),
	def(RuntimeExternalRoleID, "runtime/external-role.md", "{{SYSTEM_PROMPT}}\n\n{{OPPORTUNITY_PROMPT}}\n\nUse the ADC role API for this opportunity. Inspect the visible case and files when the facts affect the decision. Record work through send_work_notes. Submit one legal act through submit_decision, placing legal-tool arguments in payload.\nDeadline: submit this turn before {{DEADLINE}}. The turn started with {{TIMEOUT}}. The remaining_time_ms field in each response is live.\nDecision attempts: {{DECISION_ATTEMPTS}}.\nSupport tool calls: {{SUPPORT_BUDGET}} per turn.\nAllowed legal tools: {{LEGAL_TOOLS}}\nConstraints: {{CONSTRAINTS}}\nPass action: {{PASS_ACTION}}\n\nLegal tool payloads:\n{{LEGAL_TOOL_SCHEMAS}}\n\nLegal tool guidance:\n{{LEGAL_TOOL_GUIDANCE}}\n\nAvailable support tools:\n{{SUPPORT_TOOLS}}", []string{"{{SYSTEM_PROMPT}}", "{{OPPORTUNITY_PROMPT}}", "{{DEADLINE}}", "{{TIMEOUT}}", "{{DECISION_ATTEMPTS}}", "{{SUPPORT_BUDGET}}", "{{LEGAL_TOOLS}}", "{{CONSTRAINTS}}", "{{PASS_ACTION}}", "{{LEGAL_TOOL_SCHEMAS}}", "{{LEGAL_TOOL_GUIDANCE}}", "{{SUPPORT_TOOLS}}"}),
	def(ReportSummarySystemID, "report/summary/system.md", "Summarize each side's civil-trial arguments precisely from the supplied record.", nil),
	def(ReportSummaryUserID, "report/summary/user.md", "Return strict JSON with plaintiff_summary and defendant_summary. Cite supplied docket titles in square brackets.\n\nCourtroom context:\n{{COURTROOM_CONTEXT}}\n\nEvidence context:\n{{EVIDENCE_CONTEXT}}\n\nPlaintiff text:\n{{PLAINTIFF_TEXT}}\n\nDefendant text:\n{{DEFENDANT_TEXT}}", []string{"{{COURTROOM_CONTEXT}}", "{{EVIDENCE_CONTEXT}}", "{{PLAINTIFF_TEXT}}", "{{DEFENDANT_TEXT}}"}),
	def(ReportRepairSystemID, "report/repair/system.md", "Convert the supplied text to strict JSON with no outside prose.", nil),
	def(ReportRepairUserID, "report/repair/user.md", "Return strict JSON with plaintiff_summary and defendant_summary. Preserve citation anchors in square brackets.\n\n{{MODEL_OUTPUT}}", []string{"{{MODEL_OUTPUT}}"}),
	def(ImportSourceFilenameID, "tool-descriptions/direct/import_case_file/properties/source_filename.md", "Host path for local runner use only", nil),
	def(ImportOriginalNameID, "tool-descriptions/direct/import_case_file/properties/original_name.md", "Original filename when uploading file content", nil),
	def(ImportContentBase64ID, "tool-descriptions/direct/import_case_file/properties/content_base64.md", "Base64-encoded file content", nil),
	def(RoleAPICaseStatusID, "tool-descriptions/role-api/case-status.md", "Report the current case status and active opportunity.", nil),
	def(RoleAPIWorkNotesID, "tool-descriptions/role-api/send-work-notes.md", "Record private work notes outside the case record.", nil),
	def(RoleAPIReadFileBytesID, "tool-descriptions/role-api/read-case-file-bytes.md", "Read a visible case file as base64 bytes by file_id.", nil),
	def(RoleAPISubmitDecisionID, "tool-descriptions/role-api/submit-decision.md", "Submit one legal decision for the current opportunity. Pass action: {{PASS_ACTION}}.", []string{"{{PASS_ACTION}}"}),
	def(RoleAPIGetCaseID, "tool-descriptions/role-api/get-case.md", "Fetch the current visible case view.", nil),
	def(RoleAPIExplainDecisionsID, "tool-descriptions/role-api/explain-decisions.md", "Fetch decision traces visible to this role.", nil),
	def(RoleAPIListCaseFilesID, "tool-descriptions/role-api/list-case-files.md", "List visible case-file identifiers and metadata.", nil),
	def(RoleAPIReadCaseTextFileID, "tool-descriptions/role-api/read-case-text-file.md", "Read a visible text case file by file_id.", nil),
	def(RoleAPIRequestCaseFileID, "tool-descriptions/role-api/request-case-file.md", "Fetch a visible case file as model content items.", nil),
	def(RoleAPIGetJurorContextID, "tool-descriptions/role-api/get-juror-context.md", "Fetch questionnaire and voir dire context for one juror.", nil),
	def(GenericToolCardID, "tool-cards/generic.md", "Use {{TOOL_NAME}} only when it fits the current opportunity and record.", []string{"{{TOOL_NAME}}", "{{ROLE}}"}),
}

var toolCardPaths = []string{
	"clerk/add_juror", "clerk/record_jury_demand", "clerk/set_jury_configuration", "clerk/set_last_pleading_served_on", "clerk/swear_jury",
	"defendant/file_answer", "defendant/file_rule11_motion", "defendant/file_rule12_motion", "defendant/file_rule56_motion", "defendant/reply_rule12_motion", "defendant/reply_rule56_motion", "defendant/respond_interrogatories", "defendant/respond_request_for_production", "defendant/respond_requests_for_admission", "defendant/serve_rule11_safe_harbor_notice",
	"judge/advance_trial_phase", "judge/decide_juror_for_cause_challenge", "judge/decide_rule11_motion", "judge/decide_rule12_motion", "judge/decide_rule37_motion", "judge/decide_rule56_motion", "judge/deliver_jury_instructions", "judge/dismiss_for_lack_of_subject_matter_jurisdiction", "judge/enter_judgment", "judge/enter_pretrial_order", "judge/file_bench_opinion", "judge/resolve_trial_mode", "judge/settle_jury_instructions", "judge/transition_case",
	"plaintiff/file_amended_complaint", "plaintiff/file_rule37_motion", "plaintiff/offer_case_file_as_exhibit", "plaintiff/oppose_rule12_motion", "plaintiff/oppose_rule56_motion", "plaintiff/serve_interrogatories", "plaintiff/serve_request_for_production", "plaintiff/serve_requests_for_admission", "plaintiff/withdraw_or_correct_filing",
	"shared/challenge_juror_for_cause", "shared/deliver_closing_argument", "shared/get_case", "shared/list_case_files", "shared/object_jury_instruction", "shared/offer_case_file_as_exhibit", "shared/propose_jury_instruction", "shared/read_case_text_file", "shared/record_opening_statement", "shared/request_case_file", "shared/rest_case", "shared/serve_initial_disclosures", "shared/submit_technical_report", "shared/submit_trial_theory",
}

var directToolNames = []string{
	"accept_rule68_offer", "add_bench_conclusion", "add_bench_finding", "add_juror", "advance_trial_phase",
	"answer_juror_questionnaire", "answer_voir_dire_question", "challenge_juror_for_cause", "decide_juror_for_cause_challenge", "decide_rule11_motion",
	"decide_rule12_motion", "decide_rule37_motion", "decide_rule56_motion", "decide_voir_dire_question", "deliver_closing_argument",
	"deliver_jury_instructions", "dismiss_case_rule41", "dismiss_for_lack_of_subject_matter_jurisdiction", "empanel_jury", "enter_default",
	"enter_default_judgment", "enter_judgment", "enter_local_rule_override", "enter_partial_judgment", "enter_pretrial_order",
	"enter_protective_order", "enter_settlement", "evaluate_rule68_cost_shift", "expire_rule68_offers", "explain_decisions",
	"file_amended_complaint", "file_answer", "file_bench_opinion", "file_complaint", "file_rule11_motion",
	"file_rule12_motion", "file_rule37_motion", "file_rule56_motion", "file_rule59_motion", "file_rule60_motion",
	"finalize_interrogatory_responses", "get_case", "get_juror_context", "hold_in_contempt", "import_case_file",
	"issue_juror_questionnaire", "lift_protective_order", "lift_stay", "list_case_files", "make_rule68_offer",
	"object_jury_instruction", "object_to_evidence", "offer_case_file_as_exhibit", "offer_exhibit", "oppose_rule12_motion",
	"oppose_rule56_motion", "order_discretionary_stay", "pass_turn", "post_supersedeas_bond", "produce_case_file",
	"propose_jury_instruction", "read_case_text_file", "record_general_verdict_with_interrogatories", "record_jury_demand", "record_offer_of_proof",
	"record_opening_statement", "record_voir_dire_question", "reply_rule12_motion", "reply_rule56_motion", "request_case_file",
	"resolve_rule59_motion", "resolve_rule60_motion", "resolve_trial_mode", "respond_interrogatories", "respond_interrogatory_item",
	"respond_request_for_production", "respond_requests_for_admission", "rest_case", "serve_initial_disclosures", "serve_interrogatories",
	"serve_request_for_production", "serve_requests_for_admission", "serve_rule11_safe_harbor_notice", "set_jury_configuration", "set_last_pleading_served_on",
	"settle_jury_instructions", "strike_juror_peremptorily", "submit_juror_vote", "submit_technical_report", "submit_trial_theory",
	"transition_case", "withdraw_or_correct_filing",
}

var definitions = buildDefinitions()

func def(id, relativePath, fallback string, tokens []string) definition {
	return definition{Entry: Entry{ID: id, RelativePath: relativePath, Tokens: tokens}, Fallback: fallback}
}

func strategyTokens() []string {
	return []string{"{{COURT}}", "{{CASE_PACKET}}", "{{COMPLAINT}}", "{{LINKED_FILES}}", "{{PLAINTIFF_TOOLS}}", "{{DEFENDANT_TOOLS}}"}
}

func propositionStrategyTokens() []string {
	return []string{"{{PROPOSITION}}", "{{EVIDENCE_STANDARD}}"}
}

func strategyFallback(side string) string {
	return "Prepare the " + side + " strategy from this record.\nCourt:\n{{COURT}}\nCase packet:\n{{CASE_PACKET}}\nComplaint:\n{{COMPLAINT}}\nFiles:\n{{LINKED_FILES}}\nPlaintiff tools: {{PLAINTIFF_TOOLS}}\nDefendant tools: {{DEFENDANT_TOOLS}}"
}

func buildDefinitions() map[string]definition {
	out := make(map[string]definition, len(fixedDefinitions)+len(toolCardPaths)+len(directToolNames))
	for _, entry := range fixedDefinitions {
		if _, exists := out[entry.ID]; exists {
			panic("duplicate ADC prompt definition: " + entry.ID)
		}
		out[entry.ID] = entry
	}
	for _, path := range toolCardPaths {
		id := "tool-card." + strings.ReplaceAll(path, "/", ".")
		out[id] = def(id, "tool-cards/"+path+".md", "Use {{TOOL_NAME}} only when it fits the current opportunity and record.", []string{"{{TOOL_NAME}}", "{{ROLE}}"})
	}
	for _, toolName := range directToolNames {
		id := DirectToolDescriptionPrefix + toolName
		out[id] = def(id, "tool-descriptions/direct/"+toolName+".md", "Execute `"+toolName+"` with a payload matching its schema.", nil)
	}
	return out
}

func Load(opts Options) (*Catalog, error) {
	ids := KnownIDs()
	specs := make([]promptfile.Spec, 0, len(ids))
	for _, id := range ids {
		entry := definitions[id]
		specs = append(specs, promptfile.Spec{
			ID:               entry.ID,
			Name:             entry.ID,
			ConventionalPath: filepath.Join(filepath.FromSlash(ConventionalDir), filepath.FromSlash(entry.RelativePath)),
			RelativePath:     filepath.FromSlash(entry.RelativePath),
			Fallback:         entry.Fallback,
			Tokens:           entry.Tokens,
		})
	}
	sources, err := promptfile.Resolve(specs, opts.PromptFiles, opts.PromptDir)
	if err != nil {
		return nil, err
	}
	return &Catalog{sources: sources}, nil
}

func (c *Catalog) Render(id string, values map[string]string) (string, error) {
	entry, ok := definitions[id]
	if !ok {
		return "", fmt.Errorf("unknown ADC prompt id %q", id)
	}
	if c == nil {
		return "", fmt.Errorf("ADC prompt catalog is nil")
	}
	source, ok := c.sources[id]
	if !ok {
		return "", fmt.Errorf("ADC prompt %q is not loaded", id)
	}
	replacements := make([]string, 0, len(entry.Tokens)*2)
	for _, token := range entry.Tokens {
		value, exists := values[token]
		if !exists {
			return "", fmt.Errorf("render %s prompt: replacement for %s is missing", id, token)
		}
		replacements = append(replacements, token, value)
	}
	return promptfile.Render(id, source, replacements...)
}

func (c *Catalog) Text(id string) (string, error) {
	return c.Render(id, map[string]string{})
}

func (c *Catalog) ToolCard(role, tool string) (string, error) {
	role = strings.TrimSpace(role)
	tool = strings.TrimSpace(tool)
	for _, id := range []string{
		"tool-card." + role + "." + tool,
		"tool-card.shared." + tool,
		GenericToolCardID,
	} {
		if !Known(id) {
			continue
		}
		return c.Render(id, map[string]string{"{{ROLE}}": role, "{{TOOL_NAME}}": tool})
	}
	return "", fmt.Errorf("ADC tool-card prompt is unavailable for role=%s tool=%s", role, tool)
}

func Known(id string) bool {
	_, ok := definitions[strings.TrimSpace(id)]
	return ok
}

func RoleAPIToolDescriptionID(toolName string) (string, bool) {
	id, ok := map[string]string{
		"case_status":          RoleAPICaseStatusID,
		"send_work_notes":      RoleAPIWorkNotesID,
		"read_case_file_bytes": RoleAPIReadFileBytesID,
		"submit_decision":      RoleAPISubmitDecisionID,
		"get_case":             RoleAPIGetCaseID,
		"explain_decisions":    RoleAPIExplainDecisionsID,
		"list_case_files":      RoleAPIListCaseFilesID,
		"read_case_text_file":  RoleAPIReadCaseTextFileID,
		"request_case_file":    RoleAPIRequestCaseFileID,
		"get_juror_context":    RoleAPIGetJurorContextID,
	}[strings.TrimSpace(toolName)]
	return id, ok
}

func DirectToolDescriptionID(toolName string) (string, bool) {
	id := DirectToolDescriptionPrefix + strings.TrimSpace(toolName)
	return id, Known(id)
}

func DirectToolNames() []string {
	return append([]string(nil), directToolNames...)
}

func KnownIDs() []string {
	ids := make([]string, 0, len(definitions))
	for id := range definitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func Entries() []Entry {
	ids := KnownIDs()
	out := make([]Entry, 0, len(ids))
	for _, id := range ids {
		entry := definitions[id].Entry
		entry.Tokens = append([]string(nil), entry.Tokens...)
		out = append(out, entry)
	}
	return out
}
