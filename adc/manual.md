# Agent District Court Manual

## Operating Model

Agent District Court (ADC) runs a case through a Lean rule engine and a Go runtime.  A complaint becomes a normalized one-claim civil case, while a proposition becomes one declaratory claim between the Proponent and Opponent in the Proposition Tribunal.  Either case then proceeds through pleadings, motions, discovery, trial, verdict, and judgment, and an existing scenario JSON can begin at the same runtime boundary.

The case process owns the Lean state, current opportunity, case-file visibility, decision validation, deadlines, invalid-attempt limits, event log, and final record.  It handles a role through a direct model call unless the command names that role with `--external-role`.  An external plaintiff, defendant, or juror receives opportunities through the case-owned HTTP Role API.

`adc case` prepares and runs either a complaint or a proposition.  `adc scenario` runs an existing scenario, including a deterministic offline scenario.  The remaining commands prepare inputs, validate scenarios, inspect records, and replay completed cases.

All examples assume the working directory is `adc/`.  `make build` writes `.bin/adc`, `.bin/adc-mcp`, `.bin/adc-run`, and `.bin/adcengine`.  The `adc-mcp` command exposes a running case to caller-owned lawyers and jurors, while `adc-run` can start local lawyers or issue MCP capabilities for caller-owned lawyers.  The `adjservices` repository supplies optional Clerk processes, attestation, deployment, reporting, and web programs.

## Commands

The root command reports the current subcommands through `adc help`.  Each subcommand reports its flags through `adc help COMMAND`.  The table summarizes the command boundaries.

| Goal | Command |
| --- | --- |
| Draft a complaint from a situation file. | `adc complain --situation FILE` |
| Prepare and adjudicate a complaint. | `adc case --complaint FILE` |
| Prepare and adjudicate a proposition. | `adc case --proposition TEXT --evidence-standard STANDARD` |
| Build a deterministic complaint archive and manifest. | `adc case-packet --complaint FILE --packet FILE --manifest FILE` |
| Adjudicate an existing scenario. | `adc scenario --scenario FILE` |
| Run a judge behavior evaluation. | `adc eval EVAL [options]` |
| Ask one member of a juror pool a question. | `adc juror [options]` |
| Send a direct model and tool-schema probe. | `adc llm [options]` |
| Validate an existing scenario. | `adc validate --scenario FILE` |
| Read PACER-style documents from a run database. | `adc pacer --db FILE` |
| Replay a completed transition record. | `adc verify-certificate --dir DIR` |

## Build and Environment

ADC requires Go 1.25, Lean 4.32.0, `lake`, and `make`.  The Go runtime starts `.bin/adcengine` by default, so a normal adjudication requires both binaries.  The following commands build the binaries, run the Go tests, and build the Lean proof tree.  The `ex1` acceptance fixture also requires OpenSSL to generate its linked signature inputs.

```bash
make build
make test
make prove
```

Complaint drafting, complaint preparation, reports, and direct role turns use the shared OpenAI-compatible client.  Proposition preparation constructs its case without an intake or planning request, but the ensuing role turns and report still use that client.  A deterministic `adc scenario --offline` run makes no model calls.

## Behavior Evaluations

`adc eval` runs controlled judge decisions through the Lean engine and ADC production opportunity executor.  The executor provides the production prompt catalog, tool schemas, reference tools, correction loop, decision validation, and final Lean action step.  The ten suites cover voir dire questions, for-cause challenges, and Rules 11, 12, 37, 51, 52, 56, 58, and 60.  Their fixtures, prompt candidates, plans, and analyses live under [`evals/adc/judge`](../evals/adc/judge/README.md).  Generated results live under the ignored `evals/out/adc/judge/` tree.

Each suite accepts a fixture path, output directory, model, fixture limit, timeout, engine command, and court profile.  `--prompt-dir` and repeated `--prompt-file ID=PATH` flags select the production prompt catalog, while `--opportunity-prompt-file` selects one suite-local candidate objective template.  Rules 11 and 37 use their production model-backed opportunities.  Rule 58 uses a deterministic judgment-entry action derived from the completed case state.  Its `--counterfactual-model` option removes that action and requests a model decision, and the output records that execution mode.

```bash
.bin/adc eval judge-rule56 \
  --model openrouter://openai/gpt-5 \
  --engine .bin/adcengine \
  --out-dir ../evals/out/adc/judge/rule56-live
```

`adc juror` asks one selected or sampled pool member a question and can preserve an NDJSON transcript for continuation.  `adc llm` sends a direct request through the same provider client and can supply a function-tool schema.  Both commands accept prompt text through `--prompt` or `--input-file`, and accept catalog changes through `--prompt-dir` and repeated `--prompt-file ID=PATH` options.

## Prompt Catalog

`adc case`, `adc scenario`, and `adc complain` resolve model instructions through the ADC prompt catalog.  Repeated `--prompt-file ID=PATH` flags replace a partial set, while `--prompt-dir DIR` selects a complete catalog directory.  An explicit prompt file takes precedence over the matching file in `--prompt-dir`, then ADC checks `prompts/adc` beneath the working directory before using its compiled fallback.

The checked-in complete catalog is `prompts/adc` from the repository root and `../prompts/adc` from the `adc/` working directory used in this manual.  ADC validates the complete set before provider setup and rejects unknown or duplicate IDs, missing explicit files, empty files, and undeclared replacement tokens.  The [prompt authoring guide](docs/prompts.md) lists every fixed ID, path, token, direct-tool description, and tool-card naming rule.

```bash
.bin/adc scenario \
  --scenario PATH/TO/scenario.json \
  --prompt-dir ../prompts/adc

.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1 \
  --prompt-file runtime.opportunity=./prompt-work/runtime-opportunity.md
```

## Case Preparation

`adc complain` reads a situation markdown file, resolves the selected court profile, includes linked local files as source context, and writes a complaint.  The default output is `complaint.md` beside the situation file.  The command uses the selected planner model.  The `ex1` situation links to signature files generated by its signing script.

```bash
examples/ex1/sign.sh
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
```

`adc complain` accepts `--prompt-dir` and repeated `--prompt-file` flags for drafting and repair prompts.  Because `--prompt-dir` denotes a complete ADC catalog, a smaller complaint-only directory must use individual `--prompt-file` flags instead.  The command validates prompt sources before it constructs the model client.

`adc case-packet` packages a complaint and its linked local files in a deterministic `tar.gz` archive.  Its JSON manifest records the archived paths and file hashes.  Packet construction fails when a linked file is absent or lies outside the complaint directory.

```bash
.bin/adc case-packet \
  --complaint examples/ex1/complaint.md \
  --packet out/ex1/case.tar.gz \
  --manifest out/ex1/case-packet.json
```

Complaint input performs a model-driven setup stage before adjudication.  The stage writes `normalized-case.json`, `plaintiff-strategy.md`, `defense-strategy.md`, and `generated-scenario.json` under the output directory.  The runner then adjudicates the generated scenario.

Proposition setup constructs those four files without an intake or strategy-planning model request.  It creates `Proponent v. Opponent`, places the burden on the Proponent, requests declaratory relief, and uses the programmatic Proposition Tribunal under `proposition_adjudication` jurisdiction.  The caller must select `preponderance_of_the_evidence` or `clear_and_convincing`, and `--trial-mode=auto` selects a jury.

`--documents` optionally imports every regular file beneath one directory into `documents/`, preserving relative paths and bytes.  `documents.json` records each path, size, media type, and SHA-256 hash, and the generated scenario registers those records as complaint attachments.  The three positive document-limit flags remain required when no document directory is supplied, so the input policy does not depend on whether a particular case includes documents.

```bash
.bin/adc case \
  --proposition "The sky is blue." \
  --evidence-standard preponderance_of_the_evidence \
  --documents ./case-documents \
  --max-document-files 20 \
  --max-document-file-bytes 1048576 \
  --max-documents-total-bytes 4194304 \
  --out-dir out/sky
```

## Scenario Files

A scenario JSON defines the court, initial case metadata, claims, roles, optional deterministic turns, optional loop policy, and assertions.  `adc validate` reports unknown roles, missing action types, unsupported actions, and whether the scenario requires model turns.  It returns an error when the scenario is invalid.

The `autopilot_trial` loop obtains substantive filings and decisions from the assigned plaintiff, defendant, and judge.  Opportunity constraints supply state-derived identifiers and party assignments.  The engine generates deterministic actions for procedural administration, including phase changes, jury configuration, candidate creation, random empanelment when voir dire is skipped, judgment entry from a completed verdict, and other transitions whose values follow from case state or court policy.  Notices, discovery content, motions, rulings, settlements, and post-judgment requests remain actor decisions.

`adc scenario` can override the default model, temperatures, jury policy, runtime limits, and identifiers.  It can expose selected roles through the Role API and can write a transcript or digest in addition to the required machine records.  `--allow-assertion-failures` preserves a successful process exit after recording failed scenario assertions.

The command also accepts `--prompt-dir` and repeated `--prompt-file` flags.  These sources control runtime wrappers, tool descriptions and guidance, external-role instructions, corrections, and digest prompts.  Instructions and prompt preambles already present in the scenario remain scenario data inserted into the resolved runtime prompts.

```bash
.bin/adc validate --scenario PATH/TO/scenario.json

.bin/adc scenario \
  --scenario PATH/TO/scenario.json \
  --output out/scenario/run.json \
  --runtime out/scenario/runtime.json \
  --events out/scenario/events.ndjson \
  --db out/scenario/run.db \
  --transcript out/scenario/transcript.md \
  --digest out/scenario/digest.md
```

An offline run permits deterministic turns and rejects the need for a model during execution.  The validator reports `requires_llm` before such a run.  The runner also warns when `--offline` accompanies a scenario that contains non-deterministic turns.

## Jury Configuration

Jury policy consists of jury size, unanimity, and minimum concurrence.  `adc case` and `adc scenario` expose these settings as `--juror-count`, `--unanimous-required`, and `--minimum-concurring`.  Omitted flags preserve the scenario or court defaults.

The engine accepts 6 through 12 jurors.  The minimum concurrence must lie between 6 and the configured jury size.  A deliberating-juror failure removes that juror from the effective concurrence calculation while preserving the nominal policy in the case record.

Juror request specifications come from the JSONL file named by `--juror-personas`.  Each record can select the endpoint, model, provider constraints, request settings, and persona for a juror.  The runtime balances assignments across eligible endpoints and configurations, and one configuration may serve more than one candidate.  Before assigning an automatically generated candidate, ADC asks an unchecked configuration to call the production `submit_juror_vote` tool.  A failed configuration leaves the candidate pool, a missing credential removes that endpoint, and ADC samples a replacement.  A successful configuration remains available for later candidates without another preflight request.  Repeated `--council-endpoint` flags restrict the eligible endpoints.  `--minimum-distinct-council-endpoints` requires the pool to contain that many eligible endpoint names and controls the initial candidate assignments.  Voir dire may remove candidates, so the setting does not impose a minimum on the final jury.  The [model-endpoint guide](../docs/model-endpoints.md) defines the supported endpoints, credentials, request mappings, and local Pi execution.

## `adc case`

`adc case` prepares one complaint or proposition and adjudicates the resulting scenario.  The two inputs are mutually exclusive, and proposition-only settings cause an error with a complaint.  The command opens the SQLite record, starts the Lean-backed opportunity loop, and writes a JSON summary to standard output.

| Flag | Meaning |
| --- | --- |
| `--complaint` | Complaint markdown path.  Mutually exclusive with `--proposition`. |
| `--proposition` | Proposition adjudicated by the Proposition Tribunal.  Mutually exclusive with `--complaint`. |
| `--evidence-standard` | Required proposition standard: `preponderance_of_the_evidence` or `clear_and_convincing`. |
| `--documents` | Optional proposition document-root directory. |
| `--max-document-files` | Required positive proposition document-count limit. |
| `--max-document-file-bytes` | Required positive proposition per-file byte limit. |
| `--max-documents-total-bytes` | Required positive proposition total byte limit. |
| `--court` | Complaint court profile name or JSON path.  Proposition mode rejects this flag and uses the Proposition Tribunal. |
| `--out-dir` | Directory for prepared inputs and records. |
| `--model` | Default runtime model for generated scenario roles. |
| `--non-juror-model` | Default model for judge, clerk, plaintiff, and defendant. |
| `--plaintiff-model`, `--defendant-model` | Party-specific model overrides. |
| `--judge-model`, `--clerk-model` | Court-role model overrides. |
| `--planner-model` | Model for complaint intake and strategy preparation.  Proposition mode rejects this flag. |
| `--report-model` | Model for digest generation. |
| `--prompt-dir` | Complete ADC prompt catalog directory. |
| `--prompt-file` | One prompt override as `ID=PATH`.  Repeat as needed. |
| `--temperature` | Default runtime temperature override. |
| `--non-juror-temperature`, `--juror-temperature` | Role-class temperature overrides. |
| `--juror-personas` | JSONL juror request-specification file. |
| `--council-endpoint` | Allowed juror-model endpoint.  May repeat. |
| `--minimum-distinct-council-endpoints` | Minimum endpoint names assigned across automatically selected candidate jurors. |
| `--trial-mode` | `auto`, `jury`, or `bench`.  Proposition `auto` selects `jury`. |
| `--skip-voir-dire` | Empanel randomly after trial setup. |
| `--juror-count` | Jury size from 6 through 12. |
| `--unanimous-required` | `true` or `false`. |
| `--minimum-concurring` | Required concurring jurors. |
| `--online` | Enable web search for direct model calls. |
| `--timeout-seconds` | Model HTTP timeout. |
| `--invalid-attempt-limit` | Invalid responses allowed during one turn. |
| `--max-response-bytes` | Maximum direct-model response size. |
| `--external-role` | Role served through the Role API.  Repeat as needed. |
| `--caseapi-addr` | Role API listen address. |
| `--roleapi-timeout-seconds` | Deadline for each external opportunity. |
| `--case-id`, `--run-id` | API and record identifiers. |
| `--engine` | Lean engine command. |

This example uses direct model calls for every procedural role.  It requests a jury trial and writes the complete case under `out/ex1`.  The generated run identifier also becomes the case identifier unless `--case-id` changes it.

```bash
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1 \
  --trial-mode jury
```

## Role API

The Role API listens when `--caseapi-addr` supplies an address.  Every role request includes `case_id`.  Lawyer requests identify `plaintiff` or `defendant`, and juror requests also include a `principal_id` such as `J1`.  The read-only observer uses `role_id=observer`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Return `ok`, `case_id`, and `run_id` after the case API starts listening. |
| `GET` or `POST` | `/roleapi/v1/status` | Return case status, current turn, and any caller-owned opportunity. |
| `GET` or `POST` | `/roleapi/v1/get` | Return the caller's current opportunity without waiting. |
| `GET` or `POST` | `/roleapi/v1/wait_for_opportunity` | Wait up to 30 seconds for an opportunity or terminal status. |
| `GET` or `POST` | `/roleapi/v1/result` | Return the final result, failure, or pending status. |
| `POST` | `/roleapi/v1/do` | Execute a support operation, work-note submission, or legal decision. |
| `POST` | `/roleapi/v1/fail` | Report failure for the active external opportunity. |

An opportunity response identifies its id, phase, kind, time remaining, attempts remaining, and support-operation budget.  It also supplies the role prompt, role-visible case view, permitted legal tools, tool descriptions, legal-tool schemas, and support-operation schemas.  A submission must use the opportunity id from that response.  For a tool listed in `constraints.by_tool`, that entry supplies its payload constraints.  Other tools use the opportunity-level `payload_defaults` and `required_payload` fields.  An empty per-tool object imposes no fixed payload fields.

```bash
curl -sS \
  'http://127.0.0.1:9001/roleapi/v1/wait_for_opportunity?case_id=adc-CASE&role_id=plaintiff&timeout_ms=30000'
```

`POST /roleapi/v1/do` accepts the case identity, role identity, opportunity identity, operation name, and operation arguments.  `case_status` needs no opportunity id, while `send_work_notes` and `submit_decision` apply to the active opportunity.  Support operations include `get_case`, `explain_decisions`, `list_case_files`, `read_case_text_file`, `request_case_file`, `read_case_file_bytes`, and `get_juror_context` when the current role permits them.

```json
{
  "case_id": "adc-CASE",
  "role_id": "plaintiff",
  "principal_id": "",
  "opportunity_id": "OPPORTUNITY",
  "tool": "submit_decision",
  "arguments": {
    "kind": "tool",
    "tool_name": "record_opening_statement",
    "payload": {
      "party": "plaintiff",
      "summary": "Plaintiff will prove the claim through the admitted record."
    }
  }
}
```

A legal-tool decision uses `kind=tool`, `tool_name`, and `payload`.  A pass uses `kind=pass` and `reason` when the opportunity permits passing.  Lean checks the opportunity id, state version, role, tool permission, and payload before accepting the transition.

External lawyer failure ends the case because the party can no longer complete its active opportunity.  External juror failure follows the engine's juror timeout and dismissal rules.  Candidate replacement can occur during voir dire, while failure during deliberation removes the juror from the effective concurrence count.

Start a case with two external lawyers by naming both roles.  The same case process continues to handle judge, clerk, and juror opportunities through direct model calls.  External clients can then use the Role API without access to the output directory.

```bash
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-roleapi \
  --caseapi-addr 127.0.0.1:9001 \
  --external-role plaintiff \
  --external-role defendant
```

## Legal Tools

Lean determines the legal tools permitted for each opportunity.  The Role API returns those names and their schemas with the current opportunity.  The scenario role definition provides the broad role capability, while the current Lean state selects the permitted subset.

Party tools cover pleadings, discovery, dispositive motions, evidence, trial presentation, objections, closing arguments, and post-verdict work.  Judge tools control motions, trial mode, voir dire rulings, jury instructions, judgment, and bench opinions.  Clerk tools record administrative acts, configure the jury, and advance procedural stages.

Lawyer opportunities include `import_case_file` alongside the pending legal actions when the scenario permits import for that role.  An import registers one document, then the runtime requests the next opportunity.  The pending filing remains available.  Counsel may import further files or complete the filing.  After a production request, each registered file has its own production opportunity until that party produces it to the opponent.  Counsel may pass on files that are not responsive.

An external lawyer uploads a workspace file by submitting `original_name`, `content_base64`, and an optional `label` through `submit_decision` with `tool_name: "import_case_file"`.  The runtime stores the bytes under `uploaded-case-files/` beside the core run output and assigns the file identifier and metadata before Lean authorizes the decision.  A local scenario may instead supply `source_filename`, resolved on the court host.  Import registers the document.  Production records disclosure to the opponent.  `offer_exhibit` records its admission or exclusion.  Counsel must cite registered files and exhibits in filings so other participants can inspect the supporting material.

During a party evidence phase, `offer_exhibit` records an exhibit and `rest_case` ends that presentation.  `offer_exhibit.file_id` is optional.  A nonempty identifier must name an existing case file and adds the corresponding file event, while an empty identifier records a non-file-backed exhibit without a file event.  The runtime supplies `offered_at` and `produced_at` timestamps when it executes exhibit offers and file productions.  Opportunity execution replaces participant-supplied values with the runtime event time; a direct scenario action retains an explicit value and receives the runtime event time when the field is absent.

Jurors answer questionnaires, answer voir dire questions, and vote during deliberation.  A vote identifies the juror, prevailing party, damages, confidence, and explanation.  The engine derives a verdict from eligible jurors under the recorded jury policy.

## Record Utilities

`adc pacer` reads the SQLite record written by a run.  It returns the latest case unless `--case-id` names another case, and `--document-id` selects one PACER-style document.  JSON is the default output format.

`adc verify-certificate` reads a completed output directory or explicit certificate and state paths.  It replays the recorded initialization and accepted engine transitions through the selected Lean engine.  It then compares the replayed state, certificate hash, and recorded `state.json`.

```bash
.bin/adc pacer --db out/ex1/run.db
.bin/adc verify-certificate --dir out/ex1
```

## Output Record

Case preparation and adjudication share one output directory.  The prepared files explain how ADC transformed the complaint or proposition into a scenario.  The runtime files preserve the accepted actions, final state, reports, and role work notes.

| File | Meaning |
| --- | --- |
| `complaint.md` | Staged complaint text. |
| `input-files/` | Staged complaint attachments. |
| `documents.json` | Proposition document manifest with paths, sizes, media types, and hashes. |
| `documents/` | Proposition document bytes under their imported relative paths. |
| `normalized-case.json` | Prepared one-claim case. |
| `plaintiff-strategy.md` | Private plaintiff strategy. |
| `defense-strategy.md` | Private defense strategy. |
| `generated-scenario.json` | Scenario produced by case preparation. |
| `case-manifest.json` | Run identity, start time, core version, and bound case API address. |
| `runtime.json` | Normalized timeout, response-size, and invalid-attempt limits. |
| `events.ndjson` | Runtime event log. |
| `run.db` | SQLite case record. |
| `run.json` | Machine-readable result, including provider request, usage, and cost accounting. |
| `state.json` | Terminal Lean state. |
| `certificate.json` | Initialization, accepted transitions, claimed state, and hashes. |
| `transcript.md` | Written transcript. |
| `digest.md` | Written case digest. |
| `work-notes.ndjson` | Private work notes submitted by roles. |

The digest summarizes each side's recorded courtroom arguments, including technical reports.  For a side with no such text, it records: "No courtroom argument text was available for summary."  Summaries of recorded arguments require citation anchors in square brackets.  When neither side has argument text, the digest records that absence without a summary-model request.

The summary model receives registered-file statuses, production events, exhibit entries, and admission counts.  Its instructions distinguish those stages and attribute verification claims to the participant who reported them.

The `adc.replay-certificate.v1` replay certificate records engine-visible accepted transitions.  It omits rejected attempts, record reads, work notes, and participant calls.  An opportunity pass records the decision and resulting pass state.  An opportunity tool transition records the decision and the exact executed Lean action; replay checks that `apply_decision` authorizes that action before applying it.  Deterministic runtime actions remain direct step transitions.  A successful replay establishes that the recorded transition sequence produces the claimed final state under the selected engine.

The replay certificate carries no signature or execution attestation.  Any procedurally valid transition sequence can produce a valid replay result.  Authentication of the execution record requires evidence maintained outside this core replay check.

## Failure Diagnosis

Complaint preparation and packet creation require readable source files.  Linked local files must remain below the complaint directory, while proposition documents must satisfy the configured count and byte limits and must remain unchanged during import.  The presence of `documents.json`, `normalized-case.json`, strategy files, and `generated-scenario.json` identifies the last completed proposition-preparation stage.

A Role API client that receives `waiting` can inspect `current_turn` through the observer status response.  Another role, another juror principal, or an internal court role may own the current opportunity.  The following request reports that state without claiming an opportunity.

```bash
curl -sS \
  'http://127.0.0.1:9001/roleapi/v1/status?case_id=adc-CASE&role_id=observer'
```

Certificate verification errors distinguish missing files, replay failures, final-state differences, and hash differences.  The certificate and `state.json` must come from the same completed run.  The verifier must use the compatible `.bin/adcengine` command selected by `--engine`.
