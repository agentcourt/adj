# Prompt Authoring

The five procedures resolve model-facing instructions and tool descriptions through procedure prompt catalogs.  The public procedure names are Simple, Quick, ARB, ARBD, and ADC.  The `arb` and `arbd` selections use the AAR and AARD core commands.  Each procedure has a checked-in catalog beneath `prompts/` and compiled fallbacks, while Quick, AAR, AARD, and ADC also provide standalone MCP adapters in this repository, so `adj` can run every core and adapter without `adjservices`.

The command pairs below build from this repository and communicate through each core's private case API.  A caller starts the selected core, starts its adapter against the published case API address, and gives each participant an MCP capability for its assignment.  Simple makes one model request inside its core and therefore has no participant adapter.

| Procedure | Core command | Standalone MCP command |
| --- | --- | --- |
| Simple | `simple` | — |
| Quick | `quick` | `quick-mcp` |
| AAR | `aar` | `aar-mcp` |
| AARD | `aard` | `aard-mcp` |
| ADC | `adc` | `adc-mcp` |

## Resolution and rendering

Resolution has four levels, listed here from lowest to highest precedence: the compiled fallback, the conventional path beneath the process working directory, the corresponding file in `--prompt-dir DIR`, and an individual `--prompt-file ID=PATH`.  An individual override therefore wins over the complete directory, and the complete directory wins over the conventional file.  A missing conventional file selects the compiled fallback, while a missing file selected through `--prompt-dir` or `--prompt-file` returns an error.

`--prompt-file` may repeat and supplies a partial set.  `--prompt-dir` supplies a complete catalog: every catalog file that lacks an individual override must exist at its relative path beneath that directory.  The core and its MCP adapter can receive the same procedure directory because core entries occupy paths such as `lawyers/` or `attorney/`, while adapter entries occupy `mcp/`.

All relative command-line paths resolve from the working directory of the command that receives them.  Conventional core paths begin at `prompts/PROCEDURE/`, and conventional adapter paths begin at `prompts/PROCEDURE/mcp/`.  Running a command from the repository root therefore selects the checked-in prompt tree without an explicit path, while running it from a procedure subdirectory requires `--prompt-dir ../prompts/PROCEDURE` to select that tree.

Prompt rendering performs literal string replacement.  A token has the exact form `{{TOKEN}}`, with the spelling declared for that catalog entry.  Go template actions, functions, conditionals, loops, and field expressions have no meaning.  A source may omit a declared token, but an undeclared complete token, an unmatched opening `{{`, or an empty rendered prompt returns an error.

The loader validates the source before inserting runtime values.  A proposition, document, record, error message, or other runtime value that contains text such as `{{EXAMPLE}}` remains literal data and does not become a second template pass.  Closing braces without an opening `{{` also remain ordinary prompt text.

## Core catalogs

The following tables list every Simple and Quick core entry.  Paths are relative to the procedure directory supplied through `--prompt-dir`, and the conventional path prepends `prompts/simple/` or `prompts/quick/`.  The core catalogs contain six Simple entries, 23 Quick entries, 45 AAR entries, 45 AARD entries, and 210 ADC entries.  A dash in either table means that the entry accepts no replacement token.

| Simple ID | Relative path | Available tokens |
| --- | --- | --- |
| `decision` | `decision.md` | `{{EVIDENCE_STANDARD}}`, `{{WEB_SEARCH}}` |
| `search.enabled` | `search/on.md` | `{{EVIDENCE_STANDARD}}` |
| `search.disabled` | `search/off.md` | `{{EVIDENCE_STANDARD}}` |
| `case` | `case.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}` |
| `document` | `document.md` | `{{DOCUMENT_PATH}}`, `{{MEDIA_TYPE}}`, `{{SIZE_BYTES}}`, `{{SHA256}}` |
| `tool.decision` | `tools/decision.md` | — |

| Quick ID | Relative path | Available tokens |
| --- | --- | --- |
| `lawyer.common` | `lawyers/common.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{DOCUMENT_NOTICE}}` |
| `lawyer.proponent` | `lawyers/for.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{PROPONENT_ARGUMENT}}`, `{{DOCUMENT_NOTICE}}`, `{{LAWYER}}`, `{{WEB_SEARCH}}` |
| `lawyer.opponent` | `lawyers/against.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{PROPONENT_ARGUMENT}}`, `{{DOCUMENT_NOTICE}}`, `{{LAWYER}}`, `{{WEB_SEARCH}}` |
| `lawyer.document_notice` | `lawyers/document-notice.md` | `{{DOCUMENT_COUNT}}` |
| `search.enabled` | `search/on.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}` |
| `search.disabled` | `search/off.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}` |
| `observer` | `observer.md` | `{{CASE_ID}}`, `{{RUN_ID}}`, `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}` |
| `council.system` | `council/system.md` | `{{MEMBER_ID}}`, `{{PERSONA}}` |
| `council.preflight` | `council/preflight.md` | `{{MEMBER_ID}}`, `{{MODEL}}`, `{{PERSONA_FILE}}` |
| `council.case` | `council/case.md` | `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{PROPONENT_ARGUMENT}}`, `{{OPPONENT_ARGUMENT}}` |
| `council.document` | `council/document.md` | `{{DOCUMENT_PATH}}`, `{{MEDIA_TYPE}}`, `{{SIZE_BYTES}}`, `{{SHA256}}` |
| `council.no_documents` | `council/no-documents.md` | — |
| `council.submit` | `council/submit.md` | — |
| `council.repair` | `council/repair.md` | `{{ERROR}}` |
| `tool.submit_argument` | `tools/submit-argument.md` | — |
| `tool.send_work_notes` | `tools/send-work-notes.md` | — |
| `tool.get_case` | `tools/get-case.md` | — |
| `tool.get_case_result` | `tools/get-case-result.md` | — |
| `tool.list_evidence` | `tools/list-evidence.md` | — |
| `tool.stat_evidence` | `tools/stat-evidence.md` | — |
| `tool.read_evidence_range` | `tools/read-evidence-range.md` | — |
| `tool.case_status` | `tools/case-status.md` | — |
| `tool.council_vote` | `tools/council-vote.md` | — |

Simple enables provider-hosted search by default, renders `search.enabled` into `decision` through `{{WEB_SEARCH}}`, and removes the search tool while rendering `search.disabled` when `--web-search=false`.  Quick enables lawyer search by default and composes each lawyer prompt from the common, role, document, and selected search entries.  Quick uses `council.preflight` to test candidate availability and vote-tool compliance before the lawyer turns; its voting prompts remain separate and receive no search tool.

Quick's shared lawyer instruction treats propositions, arguments, documents, research queries, and tool results as material for legal analysis.  The enabled-search instruction treats queries and returned content as adjudicative research and requires source assessment.  This framing applies across tool calls while preserving the lawyer's authority to use search, browser, computer, filesystem, execution, and installation tools when the analysis requires them.

The [AAR manual](../arb/manual.md#prompt-configuration) lists all 45 AAR core IDs, relative paths, and tokens.  The [AARD manual](../arbd/manual.md#prompt-configuration) provides the corresponding 45-entry AARD catalog, including its question, judgment-standard, and council-answer entries.  Both catalogs cover attorney composition, phase instructions, evidence and text limits, direct and external council prompts, the five direct-council correction components, observers, every core tool description, and the `send_work_notes` property description.

The [ADC prompt catalog](../adc/docs/prompts.md) lists all 210 ADC IDs, paths, and tokens through its 66-entry fixed table, 92-name direct-tool set, and 52 role-specific or shared tool cards.  Its fixed entries include the `strategy.proposition.proponent` and `strategy.proposition.opponent` prompts, the `probe.juror.identity` and `probe.juror.tool-check` probe prompts, corrections, tool-result guidance, case-file attachment text, and the three `import_case_file` schema-property descriptions.  The catalog also covers complaint and case preparation, generated roles, direct and external turns, reports, tool descriptions, and tool guidance.

ADC behavior evals use the same catalog and accept the same `--prompt-dir` and repeated `--prompt-file ID=PATH` options.  A suite candidate supplied through `--opportunity-prompt-file` replaces the objective in a cloned opportunity before the current `runtime.opportunity` and `runtime.turn` entries render the model request.  Rules 11, 37, and 58 require `--counterfactual-model` for candidate evaluation because their production Lean opportunities specify deterministic judge actions; every other suite uses its production model opportunity by default.

### ADC eval candidate templates

An eval candidate uses literal `{{token}}` replacement and may omit any supported token.  Every suite supports `{{production_objective}}`, `{{actor_message}}`, `{{phase}}`, `{{allowed_tools}}`, `{{fixture_id}}`, `{{tier}}`, `{{case_theme}}`, and `{{context_notes}}`; the table lists its additional tokens.  ADC rejects an unknown token, an unmatched `{{`, or an empty rendered candidate before making the fixture's model request.

| Eval suite | Additional candidate tokens |
| --- | --- |
| Voir dire question | `{{question_family}}`, `{{asked_by}}`, `{{juror_id}}`, `{{question}}`, `{{exchange_id}}` |
| For-cause challenge | `{{issue_family}}`, `{{challenged_by}}`, `{{juror_id}}`, `{{voir_dire_record}}`, `{{challenge_grounds}}` |
| Rule 11 | `{{issue_family}}`, `{{movant}}`, `{{target_party}}`, `{{challenged_filing}}`, `{{filing_text}}`, `{{notice_text}}`, `{{notice_served_at}}`, `{{motion_filed_at}}`, `{{correction_text}}`, `{{motion_text}}`, `{{opposition_text}}` |
| Rule 12 | `{{issue_family}}`, `{{ground}}`, `{{complaint_text}}`, `{{motion_text}}`, `{{opposition_text}}`, `{{reply_text}}` |
| Rule 37 | `{{issue_family}}`, `{{movant}}`, `{{target_party}}`, `{{discovery_type}}`, `{{set_index}}`, `{{request_text}}`, `{{response_text}}`, `{{meet_and_confer_text}}`, `{{motion_text}}`, `{{opposition_text}}`, `{{reply_text}}` |
| Rule 51 | `{{issue_family}}`, `{{claim_summary}}`, `{{plaintiff_instruction}}`, `{{defendant_instruction}}`, `{{plaintiff_objection}}`, `{{defendant_objection}}`, `{{evidence_summary}}` |
| Rule 52 | `{{issue_family}}`, `{{complaint_text}}`, `{{answer_text}}`, `{{plaintiff_theory}}`, `{{defendant_theory}}`, `{{admitted_evidence}}`, `{{excluded_evidence}}`, `{{plaintiff_closing}}`, `{{defendant_closing}}` |
| Rule 56 | `{{issue_family}}`, `{{moving_party}}`, `{{opposing_party}}`, `{{motion_scope}}`, `{{request_text}}`, `{{statement_of_undisputed_facts}}`, `{{evidence_refs}}`, `{{opposition_text}}`, `{{reply_text}}` |
| Rule 58 | `{{issue_family}}`, `{{trial_mode}}`, `{{verdict_for}}`, `{{verdict_damages}}`, `{{bench_opinion_text}}`, `{{bench_judgment_amount}}`, `{{expected_claim_id}}`, `{{expected_basis}}`, `{{expected_amount}}` |
| Rule 60 | `{{issue_family}}`, `{{judgment_summary}}`, `{{motion_ground}}`, `{{motion_text}}`, `{{opposition_text}}` |

`{{production_objective}}` contains the unmodified Lean opportunity objective, while the remaining values come from the same opportunity or the selected fixture.  The renderer validates the candidate source before inserting those runtime values, so braces within a question, pleading, evidence record, or other fixture field remain literal data.  The rendered candidate becomes the `{{OBJECTIVE}}` value for `runtime.opportunity`; the production role, court rules, view, tool schemas, and tool guidance remain under their catalog IDs.

## Standalone MCP catalogs

`quick-mcp`, `aar-mcp`, `aard-mcp`, and `adc-mcp` expose procedure APIs as MCP servers.  Each adapter owns model-facing session instructions, wait-state guidance, and MCP tool descriptions, all identified by an `mcp.` prefix.  The adapters translate MCP requests to the case-owned HTTP APIs and hold no procedural state.

Every adapter uses the five wait entries shown below.  These entries and all tool-description entries accept no replacement token.  The session entry accepts the listed identity tokens and renders once for each MCP session.

| Procedures | ID | Relative path | Available tokens |
| --- | --- | --- | --- |
| Quick, AAR, AARD | `mcp.session.instructions` | `mcp/session.md` | `{{CASE_ID}}`, `{{ASSIGNMENT_TYPE}}`, `{{PRINCIPAL_ID}}` |
| ADC | `mcp.session.instructions` | `mcp/session.md` | `{{CASE_ID}}`, `{{ASSIGNMENT_TYPE}}`, `{{ROLE_ID}}`, `{{PRINCIPAL_ID}}` |
| All four | `mcp.wait.ready` | `mcp/wait/ready.md` | — |
| All four | `mcp.wait.waiting` | `mcp/wait/waiting.md` | — |
| All four | `mcp.wait.done` | `mcp/wait/done.md` | — |
| All four | `mcp.wait.failed` | `mcp/wait/failed.md` | — |
| All four | `mcp.wait.error` | `mcp/wait/error.md` | — |

Each tool name in the next table defines one complete catalog entry.  Its ID is `mcp.tool.NAME`, its relative path is `mcp/tools/NAME.md`, and it accepts no replacement token.  The table lists every tool-description entry for each adapter, including names shared by several procedures.

| Procedure | Total MCP entries | Complete tool-name set |
| --- | ---: | --- |
| Quick | 16 | `get_current_opportunity`, `wait_for_opportunity`, `case_status`, `get_case`, `get_case_result`, `list_evidence`, `stat_evidence`, `read_evidence_range`, `send_work_notes`, `submit_decision` |
| AAR | 23 | `get_current_opportunity`, `wait_for_opportunity`, `case_status`, `get_case`, `get_case_result`, `get_turn`, `list_events`, `send_work_notes`, `list_evidence`, `stat_evidence`, `read_evidence_range`, `begin_evidence_upload`, `write_evidence_chunk`, `commit_evidence_upload`, `submit_evidence`, `submit_decision`, `submit_council_vote` |
| AARD | 23 | `get_current_opportunity`, `wait_for_opportunity`, `case_status`, `get_case`, `get_case_result`, `get_turn`, `list_events`, `send_work_notes`, `list_evidence`, `stat_evidence`, `read_evidence_range`, `begin_evidence_upload`, `write_evidence_chunk`, `commit_evidence_upload`, `submit_evidence`, `submit_decision`, `submit_council_answer` |
| ADC | 20 | `get_current_opportunity`, `wait_for_opportunity`, `case_status`, `get_case`, `get_case_result`, `explain_decisions`, `list_case_files`, `read_case_text_file`, `request_case_file`, `read_case_file_bytes`, `get_juror_context`, `send_work_notes`, `submit_decision`, `report_failure` |

An adapter validates its complete MCP subset when it starts.  Repeated `--prompt-file ID=PATH` flags supply a partial MCP set, and `--prompt-file mcp.session.instructions=PATH` replaces the session entry.  `--prompt-dir DIR` supplies every non-overridden MCP entry beneath `DIR/mcp/`.

### MCP capabilities

Each MCP command has `keygen`, `issue`, and `serve` modes.  `keygen --signing-key-file PATH` creates a new private key file with mode `0600` and fails when the path exists.  `serve` requires the same `--signing-key-file PATH`, while `--api-bearer-token` remains available for authentication from the adapter to its case API.

A key file contains `adjmcpkey1.` followed by the unpadded base64url encoding of at least 32 key bytes and one final newline.  A capability has the form `adjmcp1.PAYLOAD.SIGNATURE`, where the payload is the unpadded base64url encoding of JSON fields `version`, `audience`, `case_id`, `assignment_type`, and `principal_id` in that order, with `version` fixed at `adj.mcp.capability.v1`.  The signature is HMAC-SHA-256 over `adj.mcp.capability.v1`, one newline, and the encoded payload segment.  A capability remains valid for the lifetime of the server process that loaded its signing key, and stopping that process revokes the capability.  A managed launcher removes the key file after server readiness, while a standalone caller removes it after shutdown to prevent reuse.

The signing-key file grants authority to issue every participant capability and must remain outside participant-readable filesystems.  Mode `0600` does not isolate processes running under the same Unix account.  Managed launchers remove the private key file before participants start.

`issue` prints one bearer capability bound to a procedure, case, assignment type, and principal.  Quick accepts `--role-id plaintiff`, `--role-id defendant`, or `--role-id observer`.  AAR and AARD accept those role IDs or `--member-id ID` for a council member, while ADC accepts a plaintiff, defendant, or observer role, or `--role-id juror --principal-id ID`.

Every participant connects to the exact URL `http://HOST:PORT/mcp` and sends its capability as `Authorization: Bearer TOKEN`.  The server rejects identity query parameters.  Initialization binds the MCP session to the verified assignment, and every later POST, notification, or DELETE requires a capability for that same assignment.  `GET /health` remains public.

## Command-line use

The core examples below show complete prompt-directory selection and a partial override in the context of each procedure's normal command.  They assume binaries on `PATH` and commands started from the repository root.  The procedure manuals define policy, model, engine, and record details outside prompt selection.

```bash
simple case \
  --proposition "The inspection occurred on 1 July 2026." \
  --evidence-standard preponderance_of_the_evidence \
  --documents ./examples/ex01 \
  --out-dir ./out/simple-ex01 \
  --model openai://gpt-5-mini \
  --allow-api-key \
  --max-documents 100 \
  --max-document-bytes 1048576 \
  --max-documents-bytes 8388608 \
  --prompt-dir ./prompts/simple \
  --prompt-file decision=./prompt-work/simple-decision.md

quick case \
  --proposition "The inspection occurred on 1 July 2026." \
  --documents ./examples/ex01 \
  --out-dir ./out/quick-ex01 \
  --caseapi-addr 127.0.0.1:19001 \
  --lawyerapi-bearer-token-file ./private/quick-caseapi.token \
  --council-size 3 \
  --required-votes 2 \
  --evidence-standard preponderance_of_the_evidence \
  --max-document-files 100 \
  --max-document-file-bytes 1048576 \
  --max-documents-total-bytes 8388608 \
  --allow-api-key \
  --prompt-dir ./prompts/quick \
  --prompt-file lawyer.common=./prompt-work/quick-lawyer.md

aar case \
  --complaint ./examples/ex01/complaint.md \
  --out-dir ./out/aar-ex01 \
  --caseapi-addr 127.0.0.1:19002 \
  --prompt-dir ./prompts/arb \
  --prompt-file attorney.phase.arguments=./prompt-work/aar-arguments.md

aard case \
  --complaint ./arbd/examples/ex1/complaint.md \
  --out-dir ./out/aard-ex01 \
  --caseapi-addr 127.0.0.1:19003 \
  --prompt-dir ./prompts/arbd \
  --prompt-file attorney.phase.arguments=./prompt-work/aard-arguments.md

adc case \
  --proposition "The inspection occurred on 1 July 2026." \
  --evidence-standard preponderance_of_the_evidence \
  --documents ./examples/ex01 \
  --trial-mode bench \
  --external-role plaintiff \
  --external-role defendant \
  --caseapi-addr 127.0.0.1:19004 \
  --max-document-files 100 \
  --max-document-file-bytes 1048576 \
  --max-documents-total-bytes 8388608 \
  --out-dir ./out/adc-ex01 \
  --prompt-dir ./prompts/adc \
  --prompt-file runtime.opportunity=./prompt-work/adc-opportunity.md
```

The adapter examples attach to already running case APIs.  Each command receives the same procedure directory used by its core, plus one partial MCP override.  These commands create one signing key for each adapter, start the adapter, and issue representative participant capabilities.

```bash
quick-mcp keygen --signing-key-file ./out/quick-mcp.key
quick-mcp serve \
  --caseapi-base http://127.0.0.1:19001 \
  --signing-key-file ./out/quick-mcp.key \
  --caseapi-bearer-token-file ./private/quick-caseapi.token \
  --prompt-dir ./prompts/quick \
  --prompt-file mcp.wait.ready=./prompt-work/quick-ready.md
quick-mcp issue \
  --signing-key-file ./out/quick-mcp.key \
  --case-id quick-1 \
  --role-id plaintiff

aar-mcp keygen --signing-key-file ./out/aar-mcp.key
aar-mcp serve \
  --caseapi-base http://127.0.0.1:19002 \
  --signing-key-file ./out/aar-mcp.key \
  --prompt-dir ./prompts/arb \
  --prompt-file mcp.wait.ready=./prompt-work/aar-ready.md
aar-mcp issue \
  --signing-key-file ./out/aar-mcp.key \
  --case-id aar-1 \
  --member-id council-1

aard-mcp keygen --signing-key-file ./out/aard-mcp.key
aard-mcp serve \
  --caseapi-base http://127.0.0.1:19003 \
  --signing-key-file ./out/aard-mcp.key \
  --prompt-dir ./prompts/arbd \
  --prompt-file mcp.wait.ready=./prompt-work/aard-ready.md
aard-mcp issue \
  --signing-key-file ./out/aard-mcp.key \
  --case-id aard-1 \
  --role-id observer

adc-mcp keygen --signing-key-file ./out/adc-mcp.key
adc-mcp serve \
  --caseapi-base http://127.0.0.1:19004 \
  --signing-key-file ./out/adc-mcp.key \
  --prompt-dir ./prompts/adc \
  --prompt-file mcp.wait.ready=./prompt-work/adc-ready.md
adc-mcp issue \
  --signing-key-file ./out/adc-mcp.key \
  --case-id adc-1 \
  --role-id juror \
  --principal-id juror-1
```

## Catalog boundary

The catalogs own text that a model reads as instructions or descriptions.  This includes role prompts, assembled case and document text, search guidance, correction and tool-result guidance, case-file attachment text, council and juror prompts, MCP session and wait-state guidance, core and MCP tool descriptions, and model-facing schema-property descriptions.  Replacing that text changes how a participant understands the procedure without changing the procedure's authority.

Code owns tool names, JSON-schema structure, enum values, HTTP and MCP routes, authentication, role and case binding, opportunity identity, turn order, deadlines, attempt accounting, evidence visibility, state transitions, record formats, and validation.  API and protocol errors also remain code-owned, even when cataloged guidance explains the resulting state to a model.  A prompt cannot grant a tool, relax a limit, admit evidence, alter a vote rule, or make an invalid action valid.

## Maintaining prompt sets

A complete experimental set preserves the catalog directory layout and uses one directory per revision.  A partial experiment names only the changed IDs through repeated `--prompt-file` flags, leaving the remaining entries to a complete directory, the conventional tree, or compiled fallbacks.  The run record and experiment notes retain the command, prompt paths, models, documents, procedure settings, tool activity, source use, accepted filings, decision rationale, elapsed time, token use, and provider cost.

Prompt changes require representative cases whose evidence demands differ.  Useful cases include ordinary established facts, time-dependent public facts, conflicting sources, local files, signatures, PDFs, images, audiovisual material, and evidence that benefits from computation or transformation.  Evaluation examines whether the participant used the available sources and tools, read the supplied files, respected the evidentiary boundary, filed the required action, and stopped at the correct terminal condition.
