package proceeding

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jsmorph/adj/common/promptfile"
)

const (
	promptAttorney               = "attorney.wrapper"
	promptAttorneyCommon         = "attorney.common"
	promptAttorneyStanding       = "attorney.standing"
	promptAttorneyCapabilities   = "attorney.capabilities"
	promptAttorneyWorkspace      = "attorney.workspace"
	promptAttorneyLimits         = "attorney.limits.wrapper"
	promptAttorneyTextLimits     = "attorney.limits.text"
	promptAttorneyEvidenceLimits = "attorney.limits.evidence"
	promptAttorneyOpenings       = "attorney.phase.openings"
	promptAttorneyArguments      = "attorney.phase.arguments"
	promptAttorneyRebuttals      = "attorney.phase.rebuttals"
	promptAttorneySurrebuttals   = "attorney.phase.surrebuttals"
	promptAttorneyClosings       = "attorney.phase.closings"
	promptAttorneyFinalize       = "attorney.finalize"
	promptCouncilSystem          = "council.system"
	promptCouncilPersona         = "council.persona"
	promptCouncilDirectRequest   = "council.direct.request"
	promptCouncilDirectRepair    = "council.direct.repair"
	promptCouncilRepairMalformed = "council.direct.repair.malformed_arguments"
	promptCouncilRepairOversize  = "council.direct.repair.response_too_large"
	promptCouncilRepairCallCount = "council.direct.repair.tool_call_count"
	promptCouncilRepairWrongTool = "council.direct.repair.wrong_tool"
	promptCouncilRepairArguments = "council.direct.repair.invalid_arguments"
	promptCouncilAPI             = "council.api"
	promptCouncilPreflightSystem = "council.preflight.system"
	promptCouncilPreflightUser   = "council.preflight.user"
	promptObserver               = "observer"
)

type promptDefinition struct {
	id           string
	relativePath string
	fallback     string
	tokens       []string
}

type toolDescriptionDefinition struct {
	id           string
	name         string
	relativePath string
	description  string
	readOnly     bool
}

var toolDescriptionDefinitions = []toolDescriptionDefinition{
	{id: "tool.case_status", name: "case_status", relativePath: "tools/case-status.md", description: "Return the current case phase, active turn, role status, and case counts.", readOnly: true},
	{id: "tool.lawyer.get_case", name: "get_case", relativePath: "tools/lawyer/get-case.md", description: "Return the current visible arbitration record.", readOnly: true},
	{id: "tool.observer.get_case", name: "get_case", relativePath: "tools/observer/get-case.md", description: "Return the current arbitration record.", readOnly: true},
	{id: "tool.council.get_case", name: "get_case", relativePath: "tools/council/get-case.md", description: "Return the current visible arbitration record for this council member.", readOnly: true},
	{id: "tool.send_work_notes", name: "send_work_notes", relativePath: "tools/send-work-notes.md", description: "Send private work notes for off-record operator analysis. This does not create evidence, a filing, a technical report, or a case event."},
	{id: "tool.get_turn", name: "get_turn", relativePath: "tools/get-turn.md", description: "Return the current turn role, phase, deadline, and attempts.", readOnly: true},
	{id: "tool.list_events", name: "list_events", relativePath: "tools/list-events.md", description: "List recorded case events.", readOnly: true},
	{id: "tool.list_evidence", name: "list_evidence", relativePath: "tools/list-evidence.md", description: "List visible immutable record evidence.", readOnly: true},
	{id: "tool.stat_evidence", name: "stat_evidence", relativePath: "tools/stat-evidence.md", description: "Return metadata and read limits for one visible evidence item.", readOnly: true},
	{id: "tool.observer.stat_evidence", name: "stat_evidence", relativePath: "tools/observer/stat-evidence.md", description: "Return metadata for one visible evidence item.", readOnly: true},
	{id: "tool.read_evidence_range", name: "read_evidence_range", relativePath: "tools/read-evidence-range.md", description: "Read a bounded byte range from one visible evidence item as base64.", readOnly: true},
	{id: "tool.begin_evidence_upload", name: "begin_evidence_upload", relativePath: "tools/begin-evidence-upload.md", description: "Begin a chunked evidence upload."},
	{id: "tool.write_evidence_chunk", name: "write_evidence_chunk", relativePath: "tools/write-evidence-chunk.md", description: "Write one base64 chunk into an upload session."},
	{id: "tool.commit_evidence_upload", name: "commit_evidence_upload", relativePath: "tools/commit-evidence-upload.md", description: "Verify and admit a completed evidence upload."},
	{id: "tool.submit_evidence", name: "submit_evidence", relativePath: "tools/submit-evidence.md", description: "Submit source evidence with provenance."},
	{id: "tool.submit_decision", name: "submit_decision", relativePath: "tools/submit-decision.md", description: "Submit the legal act for the current opportunity."},
	{id: "tool.submit_council_answer", name: "submit_council_answer", relativePath: "tools/submit-council-answer.md", description: "Submit one council answer for the current deliberation opportunity."},
	{id: "tool.send_work_notes.property.notes", name: "send_work_notes", relativePath: "tools/send-work-notes/property-notes.md", description: "Accumulated private work notes for this lawyer turn."},
}

var promptDefinitions = func() []promptDefinition {
	definitions := []promptDefinition{
		{
			id:           promptAttorney,
			relativePath: "attorney/wrapper.md",
			fallback:     "{{ATTORNEY_STANDING}}\n\n{{ATTORNEY_COMMON}}\n\n{{ATTORNEY_PHASE}}\n\n{{ATTORNEY_FINALIZE}}",
			tokens:       []string{"ATTORNEY_STANDING", "ATTORNEY_COMMON", "ATTORNEY_PHASE", "ATTORNEY_FINALIZE", "ROLE", "PHASE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptAttorneyCommon,
			relativePath: "attorney/common.md",
			fallback:     "You represent {{ROLE}} in an Agent Arbitration Degree proceeding.\n\nQuestion: {{QUESTION}}\nJudgment standard: {{JUDGMENT_STANDARD}}\nPhase: {{PHASE}}\nObjective: {{OBJECTIVE}}\nOpportunity ID: {{OPPORTUNITY_ID}}\n\nRecord:\n{{CURRENT_RECORD}}\n\nLimits:\n{{LIMITS_SECTION}}\n\nCouncil:\n{{COUNCIL}}\n\n{{VISIBLE_CASE_FILES_SECTION}}{{WORKSPACE_SECTION}}{{WORK_PRODUCT_SECTION}}\n\n{{MODEL_CAPABILITIES_SECTION}}\n\nAllowed final filing actions: {{DECISION_TOOLS}}",
			tokens: []string{
				"ROLE", "PHASE", "OBJECTIVE", "OPPORTUNITY_ID", "QUESTION", "JUDGMENT_STANDARD",
				"MODEL_CAPABILITIES_SECTION", "CURRENT_RECORD", "LIMITS_SECTION", "COUNCIL",
				"VISIBLE_CASE_FILES_SECTION", "WORKSPACE_SECTION", "WORK_PRODUCT_SECTION", "DECISION_TOOLS",
			},
		},
		{
			id:           promptAttorneyStanding,
			relativePath: "attorney/standing.md",
			fallback:     "Represent {{ROLE}}. Follow the current opportunity and the Lawyer API instructions.",
			tokens:       []string{"ROLE", "PHASE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptAttorneyCapabilities,
			relativePath: "attorney/capabilities.md",
			fallback:     "Use the Lawyer API as role {{ROLE}}. GET returns the current prompt, available tools, opportunity id, live deadline, and attempts left. POST executes one tool call and must include the current turn.opportunity_id. For this turn, opportunity_id is {{OPPORTUNITY_ID}}.",
			tokens:       []string{"ROLE", "PHASE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptAttorneyWorkspace,
			relativePath: "attorney/workspace.md",
			fallback:     "Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter. Do not reconstruct byte-sensitive evidence by hand. Use evidence_id plus hash as record identity.",
			tokens:       []string{"ROLE", "PHASE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptAttorneyLimits,
			relativePath: "attorney/limits/wrapper.md",
			fallback:     "{{TEXT_LIMITS_SECTION}}\n{{EVIDENCE_LIMITS_SECTION}}",
			tokens:       []string{"TEXT_LIMITS_SECTION", "EVIDENCE_LIMITS_SECTION", "ROLE", "PHASE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptAttorneyTextLimits,
			relativePath: "attorney/limits/text.md",
			fallback:     "Text limit for this submission: {{TEXT_CHAR_LIMIT}} characters.\nTarget length for the first submission: {{TARGET_TEXT_CHAR_LIMIT}} characters or less.",
			tokens:       []string{"TEXT_CHAR_LIMIT", "TARGET_TEXT_CHAR_LIMIT", "ROLE", "PHASE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptAttorneyEvidenceLimits,
			relativePath: "attorney/limits/evidence.md",
			fallback:     "Exhibits: at most {{MAX_EXHIBITS_PER_FILING}} in this filing. This side has used {{USED_EXHIBITS_FOR_SIDE}} of {{MAX_EXHIBITS_PER_SIDE}} total, with {{REMAINING_EXHIBITS_FOR_SIDE}} left.\nTechnical reports: at most {{MAX_REPORTS_PER_FILING}} in this filing. This side has used {{USED_REPORTS_FOR_SIDE}} of {{MAX_REPORTS_PER_SIDE}} total, with {{REMAINING_REPORTS_FOR_SIDE}} left.\nSubmitted evidence: admitted items may be at most {{MAX_SUBMITTED_EVIDENCE_BYTES}} bytes. Direct submit_evidence items may be at most {{MAX_DIRECT_SUBMITTED_EVIDENCE_BYTES}} bytes; chunked evidence uploads may be at most {{MAX_EVIDENCE_UPLOAD_BYTES}} bytes with {{MAX_EVIDENCE_CHUNK_BYTES}}-byte chunks. This side has submitted {{USED_SUBMITTED_EVIDENCE_FOR_SIDE}} of {{MAX_SUBMITTED_EVIDENCE_PER_SIDE}} total, with {{REMAINING_SUBMITTED_EVIDENCE_FOR_SIDE}} left.\nEvidence reads: at most {{MAX_EVIDENCE_READ_BYTES}} bytes per read, {{MAX_EVIDENCE_READS_PER_OPPORTUNITY}} reads per opportunity, and {{MAX_EVIDENCE_READ_BYTES_PER_OPPORTUNITY}} bytes total per opportunity.\nUse only visible case evidence_id values in offered_evidence. Submit new source material first with submit_evidence, then cite the returned evidence_id in offered_evidence. Use evidence_id and hash for custody checks and exact byte inspection.\nUse technical_reports for attorney analysis or synthesized work product, not as a substitute for source evidence when exact source content matters.",
			tokens: []string{
				"MAX_EXHIBITS_PER_FILING", "USED_EXHIBITS_FOR_SIDE", "MAX_EXHIBITS_PER_SIDE", "REMAINING_EXHIBITS_FOR_SIDE",
				"MAX_REPORTS_PER_FILING", "USED_REPORTS_FOR_SIDE", "MAX_REPORTS_PER_SIDE", "REMAINING_REPORTS_FOR_SIDE",
				"MAX_SUBMITTED_EVIDENCE_BYTES", "MAX_DIRECT_SUBMITTED_EVIDENCE_BYTES", "MAX_EVIDENCE_UPLOAD_BYTES", "MAX_EVIDENCE_CHUNK_BYTES",
				"USED_SUBMITTED_EVIDENCE_FOR_SIDE", "MAX_SUBMITTED_EVIDENCE_PER_SIDE", "REMAINING_SUBMITTED_EVIDENCE_FOR_SIDE",
				"MAX_EVIDENCE_READ_BYTES", "MAX_EVIDENCE_READS_PER_OPPORTUNITY", "MAX_EVIDENCE_READ_BYTES_PER_OPPORTUNITY",
				"ROLE", "PHASE", "OPPORTUNITY_ID",
			},
		},
		{id: promptAttorneyOpenings, relativePath: "attorney/phase/openings.md", fallback: "Give an opening statement grounded in the current record.", tokens: attorneyPhaseTokens()},
		{id: promptAttorneyArguments, relativePath: "attorney/phase/arguments.md", fallback: "Submit the strongest truthful merits argument for {{ROLE}} and support it with record evidence.", tokens: attorneyPhaseTokens()},
		{id: promptAttorneyRebuttals, relativePath: "attorney/phase/rebuttals.md", fallback: "Answer the opposing argument's strongest material point. Pass when no useful rebuttal is available.", tokens: attorneyPhaseTokens()},
		{id: promptAttorneySurrebuttals, relativePath: "attorney/phase/surrebuttals.md", fallback: "Answer the strongest new point in rebuttal. Pass when no useful surrebuttal is available.", tokens: attorneyPhaseTokens()},
		{id: promptAttorneyClosings, relativePath: "attorney/phase/closings.md", fallback: "Synthesize the admitted record and explain the answer supported by the judgment standard.", tokens: attorneyPhaseTokens()},
		{
			id:           promptAttorneyFinalize,
			relativePath: "attorney/finalize.md",
			fallback:     "Submit the legal act with submit_decision before the deadline.",
			tokens:       []string{"ROLE", "PHASE", "OPPORTUNITY_ID", "DECISION_TOOLS"},
		},
		{
			id:           promptCouncilSystem,
			relativePath: "council/system.md",
			fallback:     "You are council member {{MEMBER_ID}} in deliberation round {{DELIBERATION_ROUND}}. Answer the question on the 0 through 100 scale under the stated judgment standard.\n\nQuestion: {{QUESTION}}\nJudgment standard: {{JUDGMENT_STANDARD}}\n{{PERSONA_SECTION}}\nRecord:\n{{RECORD}}",
			tokens:       []string{"MEMBER_ID", "DELIBERATION_ROUND", "QUESTION", "JUDGMENT_STANDARD", "PERSONA_SECTION", "RECORD", "OPPORTUNITY_ID", "OBJECTIVE"},
		},
		{
			id:           promptCouncilPersona,
			relativePath: "council/persona.md",
			fallback:     "Persona:\n{{PERSONA}}",
			tokens:       []string{"PERSONA", "MEMBER_ID", "MODEL", "PERSONA_FILE", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilDirectRequest,
			relativePath: "council/direct/request.md",
			fallback:     "Call {{COUNCIL_TOOL}} exactly once for this opportunity.",
			tokens:       []string{"COUNCIL_TOOL", "MEMBER_ID", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilDirectRepair,
			relativePath: "council/direct/repair.md",
			fallback:     "{{CORRECTION}}",
			tokens:       []string{"CORRECTION", "REPAIR_KIND", "COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID", "SIZE_BYTES", "LIMIT_BYTES"},
		},
		{
			id:           promptCouncilRepairMalformed,
			relativePath: "council/direct/repair/malformed-arguments.md",
			fallback:     "The previous tool call arguments were malformed. Call {{COUNCIL_TOOL}} exactly once with valid JSON arguments and keep the rationale brief.",
			tokens:       []string{"COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilRepairOversize,
			relativePath: "council/direct/repair/response-too-large.md",
			fallback:     "Your response payload was {{SIZE_BYTES}} bytes; the limit is {{LIMIT_BYTES}} bytes. Call {{COUNCIL_TOOL}} exactly once with only {{SUBMISSION_FIELDS}} and a concise rationale. Do not include analysis outside the tool call.",
			tokens:       []string{"COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID", "SIZE_BYTES", "LIMIT_BYTES"},
		},
		{
			id:           promptCouncilRepairCallCount,
			relativePath: "council/direct/repair/tool-call-count.md",
			fallback:     "Call {{COUNCIL_TOOL}} exactly once.",
			tokens:       []string{"COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilRepairWrongTool,
			relativePath: "council/direct/repair/wrong-tool.md",
			fallback:     "The only allowed tool is {{COUNCIL_TOOL}}.",
			tokens:       []string{"COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilRepairArguments,
			relativePath: "council/direct/repair/invalid-arguments.md",
			fallback:     "Correct the arguments and call {{COUNCIL_TOOL}} exactly once. Validation error: {{REASON}}",
			tokens:       []string{"REASON", "COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilAPI,
			relativePath: "council/api.md",
			fallback:     "{{COUNCIL_SYSTEM}}\n\nCouncil API instructions:\nYou are a council member. Answer the question from the admitted record.\nYou may examine admitted evidence through read-only tools when exact bytes, metadata, or exhibit contents matter.\nDo not search the web, introduce new facts, create new evidence, or upload evidence.\nWhen ready, call {{COUNCIL_TOOL}} exactly once with {{SUBMISSION_FIELDS}} and a concise rationale.",
			tokens:       []string{"COUNCIL_SYSTEM", "COUNCIL_TOOL", "SUBMISSION_FIELDS", "MEMBER_ID", "OPPORTUNITY_ID"},
		},
		{
			id:           promptCouncilPreflightSystem,
			relativePath: "council/preflight/system.md",
			fallback:     "You are being checked for availability as an Agent Arbitration Degree council member. Reply with the exact word ready.",
			tokens:       []string{"MEMBER_ID", "MODEL", "PERSONA_FILE"},
		},
		{
			id:           promptCouncilPreflightUser,
			relativePath: "council/preflight/user.md",
			fallback:     "Availability check. Reply ready.",
			tokens:       []string{"MEMBER_ID", "MODEL", "PERSONA_FILE"},
		},
		{
			id:           promptObserver,
			relativePath: "observer.md",
			fallback:     "Observe the arbitration record. Observer tools are read-only.",
			tokens:       []string{"CASE_ID"},
		},
	}
	for _, tool := range toolDescriptionDefinitions {
		definitions = append(definitions, promptDefinition{
			id:           tool.id,
			relativePath: tool.relativePath,
			fallback:     tool.description,
			tokens:       []string{"TOOL_NAME", "READ_ONLY"},
		})
	}
	return definitions
}()

func attorneyPhaseTokens() []string {
	return []string{"ROLE", "PHASE", "OBJECTIVE", "OPPORTUNITY_ID", "QUESTION", "JUDGMENT_STANDARD", "DECISION_TOOLS"}
}

func PromptIDs() []string {
	ids := make([]string, 0, len(promptDefinitions))
	for _, definition := range promptDefinitions {
		ids = append(ids, definition.id)
	}
	return ids
}

func (cfg Config) renderPromptFile(id string, values map[string]string) (string, error) {
	definition, err := promptDefinitionByID(id)
	if err != nil {
		return "", err
	}
	source, ok := cfg.promptSources[id]
	if !ok {
		resolved, resolveErr := cfg.resolvePromptSources()
		if resolveErr != nil {
			return "", resolveErr
		}
		source = resolved[id]
	}
	replacements := make([]string, 0, len(definition.tokens)*2)
	for _, token := range definition.tokens {
		replacements = append(replacements, "{{"+token+"}}", values[token])
	}
	return promptfile.Render(definition.id, source, replacements...)
}

func (cfg Config) preparePrompts() (Config, error) {
	resolved, err := cfg.resolvePromptSources()
	if err != nil {
		return Config{}, err
	}
	cfg.promptSources = resolved
	cfg.toolDescriptions = make(map[string]string, len(toolDescriptionDefinitions))
	for _, definition := range toolDescriptionDefinitions {
		description, err := cfg.renderPromptFile(definition.id, map[string]string{
			"TOOL_NAME": definition.name,
			"READ_ONLY": fmt.Sprintf("%t", definition.readOnly),
		})
		if err != nil {
			return Config{}, err
		}
		cfg.toolDescriptions[definition.description] = description
	}
	observerPrompt, err := cfg.renderPromptFile(promptObserver, map[string]string{"CASE_ID": normalizeCaseID(cfg.CaseID)})
	if err != nil {
		return Config{}, err
	}
	cfg.observerPrompt = observerPrompt
	return cfg, nil
}

func (cfg Config) modelToolDescription(defaultDescription string) string {
	if description := strings.TrimSpace(cfg.toolDescriptions[defaultDescription]); description != "" {
		return description
	}
	return defaultDescription
}

func (cfg Config) resolvePromptSources() (map[string]string, error) {
	overrides := make(map[string]string, len(cfg.PromptFiles))
	for id, path := range cfg.PromptFiles {
		overrides[id] = path
	}
	specs := make([]promptfile.Spec, 0, len(promptDefinitions))
	for _, definition := range promptDefinitions {
		tokens := make([]string, 0, len(definition.tokens))
		for _, token := range definition.tokens {
			tokens = append(tokens, "{{"+token+"}}")
		}
		specs = append(specs, promptfile.Spec{
			ID:               definition.id,
			Name:             definition.id,
			ConventionalPath: filepath.Join("prompts", "arbd", definition.relativePath),
			RelativePath:     definition.relativePath,
			Fallback:         definition.fallback,
			Tokens:           tokens,
		})
	}
	return promptfile.Resolve(specs, overrides, cfg.PromptDir)
}

func promptDefinitionByID(id string) (promptDefinition, error) {
	id = strings.TrimSpace(id)
	for _, definition := range promptDefinitions {
		if definition.id == id {
			return definition, nil
		}
	}
	return promptDefinition{}, fmt.Errorf("unknown prompt ID %q; valid IDs: %s", id, strings.Join(PromptIDs(), ", "))
}

func validatePromptFileIDs(files map[string]string) error {
	ids := make([]string, 0, len(files))
	for id := range files {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := promptDefinitionByID(id); err != nil {
			return err
		}
		if strings.TrimSpace(files[id]) == "" {
			return fmt.Errorf("prompt file path for %s is empty", id)
		}
	}
	return nil
}

func attorneyPromptID(phase string) (string, error) {
	switch phase {
	case "openings":
		return promptAttorneyOpenings, nil
	case "arguments":
		return promptAttorneyArguments, nil
	case "rebuttals":
		return promptAttorneyRebuttals, nil
	case "surrebuttals":
		return promptAttorneySurrebuttals, nil
	case "closings":
		return promptAttorneyClosings, nil
	default:
		return "", fmt.Errorf("unsupported attorney prompt phase %q", phase)
	}
}
