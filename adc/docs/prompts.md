# ADC Prompt Authoring

ADC resolves every model-facing instruction through one prompt catalog.  The catalog covers complaint and case generation, proposition strategies, generated role instructions, direct and external-role turns, corrections, tool results and case-file attachments, reports, tool descriptions, schema-property descriptions, and tool guidance.  Its 208 entries comprise 64 fixed definitions, 92 direct-tool descriptions, and 52 role-specific or shared tool cards.  Court and case data, scenario `RoleSpec` values, schema structure, enum values, validation errors, and record rendering remain runtime data rather than prompt source.

## Resolution and overrides

Each prompt has a stable ID, a relative catalog path, allowed replacement tokens, and a compiled fallback.  ADC first reads a path supplied by `--prompt-file ID=PATH`, then reads `DIR/<relative path>` when `--prompt-dir DIR` is present, then checks `prompts/adc/<relative path>` beneath the process working directory, and finally uses the compiled fallback when the conventional file is absent.  An explicit file or `--prompt-dir` file must exist, while the checked-in `prompts/adc` tree contains the complete catalog.

`--prompt-file` is repeatable and supplies a partial set.  `--prompt-dir` supplies a complete set, so every catalog file without an individual override must exist beneath that directory even when one command uses only part of the catalog.  ADC rejects an unknown ID, a duplicate command-line ID, an unreadable explicit file, an empty prompt, or an undeclared `{{TOKEN}}` before making a model request.

The manual runs commands from `adc/`, where the checked-in complete catalog is `../prompts/adc`.  These examples replace one prompt and then select a complete prompt directory.  `--prompt-file` takes precedence when both forms name the same prompt.

```bash
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --prompt-file complaint.system=./my-prompts/complaint-system.md

.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1 \
  --prompt-dir ../prompts/adc
```

The same flags apply to `adc case`, `adc scenario`, and `adc complain`.  Programmatic callers use `PromptDir` and `PromptFiles map[string]string` on `casegen.PlanningOptions`, `casegen.ScenarioOptions`, `casegen.ComplaintDraftOptions`, `runner.Config`, and `report.DigestOptions`.  `casegen.CreatePropositionPlanWithOptions` applies `PlanningOptions` to the generated Proponent and Opponent strategies.  A scenario's existing role instructions and prompt preambles remain scenario data and pass through the catalog's runtime wrappers.

## Replacement syntax

ADC performs literal string replacement rather than Go template execution.  A source file may contain only the tokens declared for its catalog entry, with the exact uppercase spelling and braces shown below.  Replacement values may contain text that resembles `{{TOKEN}}`; ADC checks the source before inserting runtime values, so such data remains literal.

An author can omit an allowed token from a custom source.  ADC still supplies every declared value at the call site, which keeps one ID's runtime interface stable across prompt revisions.  Adding a new token requires a catalog and call-site change because an undeclared marker causes prompt loading to fail.

## Fixed catalog entries

Paths in this table are relative to a `--prompt-dir` directory and to the conventional `prompts/adc` directory.  A dash in the token column means that the prompt accepts no replacement token.  Tool-description and tool-card families follow the table because their IDs derive from tool names.

| ID | Relative path | Tokens |
| --- | --- | --- |
| `complaint.system` | `complaint/system.md` | — |
| `complaint.user` | `complaint/user.md` | `{{COURT}}`, `{{SOURCE}}`, `{{LINKED_FILES}}` |
| `complaint.repair` | `complaint/repair.md` | `{{ERROR}}` |
| `case-packet.system` | `case-packet/system.md` | — |
| `case-packet.user` | `case-packet/user.md` | `{{COURT}}`, `{{COMPLAINT}}`, `{{LINKED_FILES}}` |
| `case-packet.repair` | `case-packet/repair.md` | `{{ERROR}}` |
| `strategy.plaintiff.system` | `strategy/plaintiff/system.md` | — |
| `strategy.plaintiff.user` | `strategy/plaintiff/user.md` | `{{COURT}}`, `{{CASE_PACKET}}`, `{{COMPLAINT}}`, `{{LINKED_FILES}}`, `{{PLAINTIFF_TOOLS}}`, `{{DEFENDANT_TOOLS}}` |
| `strategy.defense.system` | `strategy/defense/system.md` | — |
| `strategy.defense.user` | `strategy/defense/user.md` | `{{COURT}}`, `{{CASE_PACKET}}`, `{{COMPLAINT}}`, `{{LINKED_FILES}}`, `{{PLAINTIFF_TOOLS}}`, `{{DEFENDANT_TOOLS}}` |
| `strategy.proposition.proponent` | `strategy/proposition/proponent.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}` |
| `strategy.proposition.opponent` | `strategy/proposition/opponent.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}` |
| `role.plaintiff.instructions` | `roles/plaintiff/instructions.md` | — |
| `role.plaintiff.runtime` | `roles/plaintiff/runtime.md` | `{{COURT_RULES}}`, `{{STRATEGY}}` |
| `role.defendant.instructions` | `roles/defendant/instructions.md` | — |
| `role.defendant.runtime` | `roles/defendant/runtime.md` | `{{COURT_RULES}}`, `{{STRATEGY}}` |
| `role.clerk.instructions` | `roles/clerk/instructions.md` | — |
| `role.clerk.runtime` | `roles/clerk/runtime.md` | `{{COURT_RULES}}` |
| `role.judge.instructions` | `roles/judge/instructions.md` | — |
| `role.judge.runtime` | `roles/judge/runtime.md` | `{{COURT_RULES}}` |
| `role.juror.instructions` | `roles/juror/instructions.md` | — |
| `role.juror.runtime` | `roles/juror/runtime.md` | — |
| `runtime.system` | `runtime/system.md` | `{{ROLE}}`, `{{PREAMBLE}}`, `{{INSTRUCTIONS}}`, `{{ALLOWED_ACTIONS}}`, `{{VIEW}}` |
| `runtime.opportunity` | `runtime/opportunity.md` | `{{ACTOR_MESSAGE}}`, `{{OBJECTIVE}}`, `{{PHASE}}`, `{{ALLOWED_ACTIONS}}`, `{{REFERENCE_TOOLS}}`, `{{CONSTRAINTS}}`, `{{PASS_ACTION}}` |
| `runtime.juror` | `runtime/juror.md` | `{{ROLE}}`, `{{PREAMBLE}}`, `{{INSTRUCTIONS}}`, `{{ALLOWED_ACTIONS}}`, `{{JUROR_ID}}`, `{{PERSONA}}`, `{{DELIBERATION}}` |
| `runtime.juror-deliberation` | `runtime/juror-deliberation.md` | `{{ROUND}}`, `{{TRANSCRIPT}}`, `{{JURY_INSTRUCTIONS}}`, `{{PRIOR_BALLOT}}` |
| `runtime.turn` | `runtime/turn.md` | `{{BASE_PROMPT}}`, `{{TOOL_SCHEMAS}}`, `{{TOOL_GUIDANCE}}` |
| `runtime.correction` | `runtime/correction.md` | `{{ROLE}}`, `{{ISSUES}}`, `{{GUIDANCE}}`, `{{REQUIRED_FIELDS}}`, `{{ALLOWED_ACTIONS}}`, `{{PASS_ACTION}}` |
| `runtime.correction.local-rule-limit` | `runtime/correction/local-rule-limit.md` | — |
| `runtime.correction.required-fields` | `runtime/correction/required-fields.md` | — |
| `runtime.correction.already-complete` | `runtime/correction/already-complete.md` | — |
| `runtime.correction.no-tool` | `runtime/correction/no-tool.md` | `{{PASS_ACTION}}` |
| `runtime.correction.multiple-tools` | `runtime/correction/multiple-tools.md` | — |
| `runtime.correction.disallowed-tool` | `runtime/correction/disallowed-tool.md` | — |
| `runtime.correction.support-budget-exhausted` | `runtime/correction/support-budget-exhausted.md` | — |
| `runtime.correction.fixed-fields` | `runtime/correction/fixed-fields.md` | `{{FIXED_FIELDS}}` |
| `runtime.correction.malformed-arguments` | `runtime/correction/malformed-arguments.md` | — |
| `runtime.result.import-upload-fields` | `runtime/result/import-upload-fields.md` | — |
| `runtime.result.unknown-case-file` | `runtime/result/unknown-case-file.md` | `{{FILE_ID}}`, `{{AVAILABLE_FILE_IDS}}` |
| `runtime.result.unsupported-case-text-extension` | `runtime/result/unsupported-case-text-extension.md` | `{{CASE_FILE}}`, `{{EXTENSION}}` |
| `runtime.result.case-file-missing-readable-path` | `runtime/result/case-file-missing-readable-path.md` | — |
| `runtime.result.case-file-not-utf8` | `runtime/result/case-file-not-utf8.md` | `{{CASE_FILE}}` |
| `runtime.result.case-file-missing-attachment-path` | `runtime/result/case-file-missing-attachment-path.md` | — |
| `runtime.result.case-file-already-offered` | `runtime/result/case-file-already-offered.md` | — |
| `runtime.case-file-attachment` | `runtime/case-file-attachment.md` | `{{FILENAME}}` |
| `runtime.external-role` | `runtime/external-role.md` | `{{SYSTEM_PROMPT}}`, `{{OPPORTUNITY_PROMPT}}`, `{{DEADLINE}}`, `{{TIMEOUT}}`, `{{DECISION_ATTEMPTS}}`, `{{SUPPORT_BUDGET}}`, `{{LEGAL_TOOLS}}`, `{{CONSTRAINTS}}`, `{{PASS_ACTION}}`, `{{LEGAL_TOOL_SCHEMAS}}`, `{{LEGAL_TOOL_GUIDANCE}}`, `{{SUPPORT_TOOLS}}` |
| `report.summary.system` | `report/summary/system.md` | — |
| `report.summary.user` | `report/summary/user.md` | `{{COURTROOM_CONTEXT}}`, `{{EVIDENCE_CONTEXT}}`, `{{PLAINTIFF_TEXT}}`, `{{DEFENDANT_TEXT}}` |
| `report.repair.system` | `report/repair/system.md` | — |
| `report.repair.user` | `report/repair/user.md` | `{{MODEL_OUTPUT}}` |
| `tool-description.direct.import_case_file.property.source_filename` | `tool-descriptions/direct/import_case_file/properties/source_filename.md` | — |
| `tool-description.direct.import_case_file.property.original_name` | `tool-descriptions/direct/import_case_file/properties/original_name.md` | — |
| `tool-description.direct.import_case_file.property.content_base64` | `tool-descriptions/direct/import_case_file/properties/content_base64.md` | — |
| `tool-description.role-api.case-status` | `tool-descriptions/role-api/case-status.md` | — |
| `tool-description.role-api.send-work-notes` | `tool-descriptions/role-api/send-work-notes.md` | — |
| `tool-description.role-api.read-case-file-bytes` | `tool-descriptions/role-api/read-case-file-bytes.md` | — |
| `tool-description.role-api.submit-decision` | `tool-descriptions/role-api/submit-decision.md` | `{{PASS_ACTION}}` |
| `tool-description.role-api.get-case` | `tool-descriptions/role-api/get-case.md` | — |
| `tool-description.role-api.explain-decisions` | `tool-descriptions/role-api/explain-decisions.md` | — |
| `tool-description.role-api.list-case-files` | `tool-descriptions/role-api/list-case-files.md` | — |
| `tool-description.role-api.read-case-text-file` | `tool-descriptions/role-api/read-case-text-file.md` | — |
| `tool-description.role-api.request-case-file` | `tool-descriptions/role-api/request-case-file.md` | — |
| `tool-description.role-api.get-juror-context` | `tool-descriptions/role-api/get-juror-context.md` | — |
| `tool-card.generic` | `tool-cards/generic.md` | `{{ROLE}}`, `{{TOOL_NAME}}` |

## Tool-description and guidance families

Every direct-runtime action has its own description ID `tool-description.direct.NAME` and path `tool-descriptions/direct/NAME.md`.  These prompts accept no token because each file belongs to one named action.  The three `import_case_file` schema-property descriptions use the fixed IDs and nested paths listed above, and the runtime inserts their resolved text wherever it presents that schema to a model.  The current action names are:

```text
accept_rule68_offer add_bench_conclusion add_bench_finding add_juror
advance_trial_phase answer_juror_questionnaire answer_voir_dire_question
challenge_juror_for_cause decide_juror_for_cause_challenge decide_rule11_motion
decide_rule12_motion decide_rule37_motion decide_rule56_motion
decide_voir_dire_question deliver_closing_argument deliver_jury_instructions
dismiss_case_rule41 dismiss_for_lack_of_subject_matter_jurisdiction empanel_jury
enter_default enter_default_judgment enter_judgment enter_local_rule_override
enter_partial_judgment enter_pretrial_order enter_protective_order enter_settlement
evaluate_rule68_cost_shift expire_rule68_offers explain_decisions
file_amended_complaint file_answer file_bench_opinion file_complaint
file_rule11_motion file_rule12_motion file_rule37_motion file_rule56_motion
file_rule59_motion file_rule60_motion finalize_interrogatory_responses get_case
get_juror_context hold_in_contempt import_case_file issue_juror_questionnaire
lift_protective_order lift_stay list_case_files make_rule68_offer
object_jury_instruction object_to_evidence offer_case_file_as_exhibit offer_exhibit
oppose_rule12_motion oppose_rule56_motion order_discretionary_stay pass_turn
post_supersedeas_bond produce_case_file propose_jury_instruction read_case_text_file
record_general_verdict_with_interrogatories record_jury_demand record_offer_of_proof
record_opening_statement record_voir_dire_question reply_rule12_motion
reply_rule56_motion request_case_file resolve_rule59_motion resolve_rule60_motion
resolve_trial_mode respond_interrogatories respond_interrogatory_item
respond_request_for_production respond_requests_for_admission rest_case
serve_initial_disclosures serve_interrogatories serve_request_for_production
serve_requests_for_admission serve_rule11_safe_harbor_notice set_jury_configuration
set_last_pleading_served_on settle_jury_instructions strike_juror_peremptorily
submit_juror_vote submit_technical_report submit_trial_theory transition_case
withdraw_or_correct_filing
```

A checked-in tool card has ID `tool-card.ROLE.NAME` and path `tool-cards/ROLE/NAME.md`, where `ROLE` is `plaintiff`, `defendant`, `clerk`, `judge`, or `shared`.  Every checked-in card accepts `{{ROLE}}` and `{{TOOL_NAME}}`, although a card need not include either token.  A tool without a role-specific or shared card uses `tool-card.generic`, which keeps uncatalogued guidance concise while the tool schema remains runtime data.

The direct and external-role paths resolve the same `runtime.system`, `runtime.opportunity`, `runtime.juror`, `runtime.juror-deliberation`, correction, tool-description, schema-property-description, and tool-card sources.  The external path adds `runtime.external-role` around those resolved components because its client acts through the Role API rather than direct function calls.  The prompt catalog therefore changes wording without changing procedural authority, schema structure, case visibility, or Lean validation.
