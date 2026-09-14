# Agent Arbitration Degree Manual

## Overview

Agent Arbitration Degree, or AARD, runs an arbitration about one question and returns degree answers.  A complaint states the question, two lawyers build and argue the record, and each council member answers with an integer from 0 through 100 under the configured judgment standard.  The runtime enforces the procedure, stores the record, and writes a packet for later inspection.

`aard case` runs one case and exposes HTTP APIs for lawyer and observer clients.  With the `councilapi` backend, the same listener also exposes the Council API for external council clients.  With the `direct` backend, the executable calls council models.  External processes provide the lawyers in both modes.

The other commands prepare complaints and deterministic case packets or verify a completed case.  They share the complaint parser and proceeding implementation used by `aard case`.  Commands in this manual assume the working directory is `arbd/` unless stated otherwise.

## Operating Model

A case process owns the arbitration, including phase, turn order, deadlines, attempt budgets, evidence, work notes, council roster, and final output.  Lawyer and observer clients read the case and act through its HTTP APIs.  Council clients use those APIs only under the `councilapi` backend, while the direct backend calls council models from the case process.  Clients do not need access to the case output directory.  The case process writes every accepted procedural action to the durable record.

The case command selects the council before it starts the HTTP listener.  With the default `direct` council backend, it checks each selected model and calls that model when deliberation begins.  With the `councilapi` backend, external clients own model execution, read deliberation opportunities, and submit answers through the case process.

## Choosing A Command

| Goal | Command |
| --- | --- |
| Run one case and expose its Lawyer and optional Council APIs. | `aard case --complaint FILE --out-dir DIR`. |
| Build a deterministic case packet for an external service. | `aard case-packet --complaint FILE --packet case.tar.gz --manifest case-packet.json`. |
| Normalize or check a complaint file. | `aard complain` and `aard validate`. |
| Check a completed packet against its recorded engine actions. | `aard verify-certificate --dir DIR`. |

## Core Concepts

An AARD case begins with a complaint containing one question.  The plaintiff argues for a higher score when the record supports it, while the defendant tests the evidence, identifies gaps, develops contrary evidence, and argues for a lower score or a narrower supported range.  The case proceeds through openings, arguments, rebuttals, surrebuttals, closings, and council deliberation.  Each council member submits one integer answer from 0 through 100 and a rationale grounded in the admitted record.

The record contains lawyer filings, admitted evidence, technical reports, and council answers.  Initial case files enter an immutable catalog containing `evidence_id`, SHA-256, and byte-size commitments before the first opening.  Lawyers may submit further evidence during arguments, rebuttals, and surrebuttals, and those phases may offer evidence and technical reports.

A filing may offer only an initial or submitted identifier visible in that filing action's source state.  Derived evidence names a complete parent identifier, parent digest, and derivation method tied to an initial or earlier submitted item.  Openings and closings may read evidence but cannot add submissions, offered evidence, or technical reports, while work notes remain outside the evidentiary record in `work-notes.ndjson`.

## Operator Guidance

Use AARD for a question whose supported answer falls on a scale from 0 through 100.  The question should permit lawyers to identify evidence, challenge provenance, and argue an answer range within the filing limits.  Preserve the complete output directory, including any configuration, state, replay, council-turn, event, and evidence-custody records it contains.

The packet records the outcome, engine state, procedural sequence, off-record planning, and admitted evidence for one case.  `aard verify-certificate` checks that accepted actions reproduce the recorded terminal state under the selected engine.  It does not rehash stored evidence files, so byte custody requires separate inspection.

## Repository Layout

| Path | Meaning |
| --- | --- |
| `runtime/cmd/aard/` | Go command-line package for `aard`. |
| `runtime/proceeding/` | Case runner, Lawyer API, Council API, evidence storage, rendering, and policy logic. |
| `runtime/lean/` | Go client for the Lean engine. |
| `runtime/spec/` | Complaint parser. |
| `engine/AARD/Core.lean` | Lean state, policy, opportunities, transitions, initialization, and replay. |
| `engine/Main.lean` | JSON protocol and executable entry point. |
| `engine/Proofs/` | Namespaced Lean proofs over the executable core. |
| `etc/policy.json` | Default case policy. |
| `../prompts/arbd/` | Complete editable AARD prompt set. |
| `examples/` | Complaints and initial case files. |

## Build And Environment

Build from `arbd/` with `make build`.  The target builds the Lean engine and the Go command into `.bin/`, and `make test` depends on that build before running the Go tests.  A direct Go build can rebuild the command after a Go-only change.

```bash
make build
go build -o .bin/aard ./runtime/cmd/aard
```

The Go test target covers the runtime, while the Lean build checks the proof tree.  The direct commands below expose each test separately.  Both should pass before distributing a core build.

```bash
go test -count=1 ./runtime/...
cd engine
lake build Proofs
```

Direct council calls support the registered Anthropic, DeepSeek, Google, Hugging Face, OpenAI, OpenRouter, and xAI endpoints.  Each selected endpoint requires its canonical credential.  The default pool path is a local `pool.jsonl` when present, followed by `<common-root>/data/personas/pool.jsonl`.  Persona paths resolve from the pool directory and then from the shared common tree.  The [model-endpoint guide](../docs/model-endpoints.md) defines credentials, pool records, request support, and endpoint-balanced selection.

## Complaint Files

A complaint is Markdown containing the degree question.  A `Question` heading identifies the question section when present.  Without that heading, AARD uses the complete trimmed document.

```markdown
# Question

How strongly does the case record support the claim that the defendant sent the signed confession?
```

`aard validate` parses a complaint and prints `ok` on success.  `aard complain` parses a situation file and writes a canonical complaint under a `# Question` heading.  Both commands use the parser used during case initialization.

```bash
.bin/aard validate --complaint examples/ex1/complaint.md
.bin/aard complain --situation work/my-case/situation.md --out work/my-case/complaint.md
```

## Initial Evidence

When `aard case` starts without `--file`, it scans the complaint directory for initial case files and skips directories and names ending in `~`.  It excludes the configured complaint by file identity and the exact names `.gitignore`, `README.md`, `complaint.md`, `situation.md`, `sign.sh`, `confession.sig`, and `samantha_private.pem`.  Every selected input must be a regular file, and text-like files enter as readable evidence while other files enter as byte-bearing evidence.

The repeatable `--file` flag selects explicit initial evidence.  Supplying any `--file` value replaces automatic directory scanning.  Every file required in the initial record must therefore appear in the explicit selection.

For each input, the runtime checks that the path and opened descriptor identify the same regular file.  It reads and hashes that one descriptor, rewinds it for publication, and rejects replacement, size drift, or digest drift.  The runtime publishes the verified bytes into `evidence-store/` and builds the Lean initial catalog before council selection or initialization.

```bash
.bin/aard case --complaint work/my-case/complaint.md --file work/my-case/source-a.pdf --file 'work/my-case/captures/*.png' --out-dir out/my-case
```

`aard case-packet` applies the same complaint and initial-evidence selection without starting a case.  It writes a deterministic gzip archive and a JSON manifest using schema `aard.case-packet.v0`.  The service can transport those files without importing the proceeding implementation.

```bash
.bin/aard case-packet --complaint work/my-case/complaint.md --packet /tmp/case.tar.gz --manifest /tmp/case-packet.json
```

## Commands

| Command | Purpose |
| --- | --- |
| `aard complain` | Write a canonical complaint from a situation Markdown file. |
| `aard validate` | Validate that a complaint parses. |
| `aard case-packet` | Build deterministic `case.tar.gz` and `case-packet.json` inputs for an external service. |
| `aard case` | Run one case and expose the private Lawyer and optional Council APIs. |
| `aard verify-certificate` | Replay `certificate.json` against `state.json` using the Lean engine. |

Command help reports the current flags and defaults.  Each subcommand accepts `-h`, and the root command accepts `help SUBCOMMAND`.  The following commands cover the main core interfaces.

```bash
.bin/aard help case
.bin/aard help case-packet
.bin/aard help verify-certificate
```

## `aard verify-certificate`

`aard verify-certificate` checks a completed packet's `aard.replay-certificate.v1` replay certificate.  It rejects older schema values, requires procedure `aard`, replays initialization and every accepted public action through the selected Lean engine, and compares the result with the claimed final-state hash.  It also requires `state.json` to match that hash and the replayed state to be terminal.

The certificate contains the engine-visible transition record.  Its `initialize_request` contains the initial state, degree question, and council roster, while each ordered action contains the operation, payload, source-state version, and exact opportunity authority.  Evidence actions preserve digest, size, and lineage metadata, and `claimed_final_state` plus `claimed_final_state_sha256` identify the asserted terminal state.

The verifier requires one nonblank case id across the certificate, initialization state, claimed state, packet state, and replayed state.  It accepts only final status `closed` or `failed`, and it checks council-answer payload membership against the action authority.  The file carries no signature, so authenticating the recorded history requires an independent custody or attestation mechanism.

| Check | Failure reported |
| --- | --- |
| Schema is `aard.replay-certificate.v1`, procedure is `aard`, and case ids agree. | A schema, procedure, or case-id error. |
| The claimed final-state hash matches `claimed_final_state`. | `certificate final state hash mismatch` |
| The packet's `state.json` matches the claimed final-state hash. | `packet final state mismatch` |
| Every action has exact authority for its source version, and the Lean engine accepts it. | An authority error, `initialize_case rejected`, or `certificate action N (...) rejected`. |
| Replaying the actions yields the claimed final state. | `replayed final state mismatch` |
| The claimed and replayed status is terminal. | A nonterminal final-state error. |

The Go verifier validates envelope facts, replays actions, and compares canonical JSON hashes.  Lean's `checkReplayCertificate` is a formal function rather than an exported Go verification call.  The Lean proofs establish that success of that function implies exact replay and authority, reachability, record integrity, source-state offer chronology, and fixed initial-catalog equality.  The closed and failed certificate fact packages additionally require the corresponding claimed-state status premise.

### Example

```bash
.bin/aard verify-certificate --dir out/ex1-direct
```

| Flag | Meaning |
| --- | --- |
| `--dir` | Packet directory containing `certificate.json` and `state.json`. |
| `--certificate` | Certificate path override. |
| `--state` | Final-state path override. |
| `--engine` | Lean engine binary. |
| `--engine-timeout-seconds` | Maximum seconds for each Lean invocation.  Default: `30`. |

Successful verification prints a JSON object containing `status: "ok"`, the case identifier, the run identifier when present, the accepted-action count, and the final-state hash.  A failed check exits with an error that identifies the first mismatch or rejected engine action, and every engine call is subject to the configured finite timeout.  The command does not inspect work notes, events, council snapshots, human-readable reports, or evidence bytes, and it does not rehash `evidence-store/`.

## `aard case`

`aard case` initializes one case and waits for its lawyer clients.  It writes the private Case API base URL to stderr as `caseapi listening on http://127.0.0.1:PORT`.  After termination, it writes one JSON summary to stdout.  The summary reports the answers and output path or a structured failure.

### Example

```bash
.bin/aard case --complaint examples/ex1/complaint.md --prompt-dir ../prompts/arbd --out-dir out/ex1-direct
```

### Flags

| Flag | Meaning |
| --- | --- |
| `--complaint` | Complaint Markdown file.  Required. |
| `--out-dir` | Output directory for the case packet.  Required. |
| `--file` | Initial evidence file or glob.  May repeat. |
| `--policy` | Policy JSON file.  Defaults to `./etc/policy.json` when present. |
| `--council-size` | Override `policy.council_size`. |
| `--judgment-standard` | Override `policy.judgment_standard`. |
| `--prompt-file` | Prompt override in `ID=PATH` form.  May repeat. |
| `--prompt-dir` | Complete prompt directory.  Every non-overridden catalog file is required. |
| `--common-root` | Shared `common/` tree for council pool and personas. |
| `--council-pool` | Council JSONL request-spec pool. |
| `--council-endpoint` | Allowed council endpoint.  May repeat. |
| `--minimum-distinct-council-endpoints` | Minimum endpoint names represented in the completed council. |
| `--caseapi-addr` | Private Case API listen address.  Default: `127.0.0.1:0`. |
| `--council-backend` | `direct` or `councilapi`. |
| `--timeout-seconds` | Council opportunity timeout override for either backend. |
| `--lawyer-timeout-seconds` | Lawyer turn timeout override. |
| `--engine-timeout-seconds` | Maximum seconds for one Lean engine call. |
| `--max-response-bytes` | Parsed response byte limit override. |
| `--invalid-attempt-limit` | Invalid tool-call attempt limit override. |
| `--engine` | Lean engine binary. |
| `--run-id` | Run identifier override. |
| `--case-id` | Case identifier override.  Default: `arbd-1`. |

The default council backend is `direct`.  Direct mode passes the rendered final record to each selected model and exposes only `submit_council_answer` for that model call.  `councilapi` mode gives an external member live case and evidence reads plus the same answer operation.

Both backends write operator records under `council-turns/`.  Each `input.json` uses schema `aard.council-turn-snapshot.v0`, and the sibling `prompt.txt` contains the rendered prompt.  A direct model receives the prompt through its provider request rather than reading these snapshot files.

The Case API provides `GET /health` on the listener.  It returns HTTP `200` with JSON containing `ok`, `case_id`, and `run_id` after the process has bound its address.  The listener address printed to stderr becomes the base for both role APIs.

## Prompt Configuration

Prompt resolution follows four levels: an individual file override, the complete `--prompt-dir` set, the conventional file under the process working directory, and the compiled fallback.  `--prompt-file ID=PATH` may repeat and provides partial overrides.  A supplied prompt directory must contain every catalog filename that an individual override does not replace.

Conventional files live under `prompts/arbd/` relative to the process working directory.  The checked-in set has that layout from the repository root; a command run from `arbd/` selects it with `--prompt-dir ../prompts/arbd`.  Each source undergoes token validation before runtime values are inserted, so unknown `{{...}}` sequences fail while brace text inside a runtime value remains literal.

A template may omit any available token, and replacement uses literal strings rather than Go template evaluation.  The `--prompt-dir` paths in the tables are relative to the supplied directory.  The case-runtime catalog contains 45 IDs: 27 instruction entries and 18 tool or schema-description entries.  Every entry in the second table accepts `{{TOOL_NAME}}` and `{{READ_ONLY}}`; the checked-in files contain the complete description and need neither token.

| Prompt ID | `--prompt-dir` path | Available replacement tokens |
| --- | --- | --- |
| `attorney.wrapper` | `attorney/wrapper.md` | `{{ATTORNEY_STANDING}}`, `{{ATTORNEY_COMMON}}`, `{{ATTORNEY_PHASE}}`, `{{ATTORNEY_FINALIZE}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.common` | `attorney/common.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OBJECTIVE}}`, `{{OPPORTUNITY_ID}}`, `{{QUESTION}}`, `{{JUDGMENT_STANDARD}}`, `{{MODEL_CAPABILITIES_SECTION}}`, `{{CURRENT_RECORD}}`, `{{LIMITS_SECTION}}`, `{{COUNCIL}}`, `{{VISIBLE_CASE_FILES_SECTION}}`, `{{WORKSPACE_SECTION}}`, `{{WORK_PRODUCT_SECTION}}`, `{{DECISION_TOOLS}}` |
| `attorney.standing` | `attorney/standing.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.capabilities` | `attorney/capabilities.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.workspace` | `attorney/workspace.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.limits.wrapper` | `attorney/limits/wrapper.md` | `{{TEXT_LIMITS_SECTION}}`, `{{EVIDENCE_LIMITS_SECTION}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.limits.text` | `attorney/limits/text.md` | `{{TEXT_CHAR_LIMIT}}`, `{{TARGET_TEXT_CHAR_LIMIT}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.limits.evidence` | `attorney/limits/evidence.md` | `{{MAX_EXHIBITS_PER_FILING}}`, `{{USED_EXHIBITS_FOR_SIDE}}`, `{{MAX_EXHIBITS_PER_SIDE}}`, `{{REMAINING_EXHIBITS_FOR_SIDE}}`, `{{MAX_REPORTS_PER_FILING}}`, `{{USED_REPORTS_FOR_SIDE}}`, `{{MAX_REPORTS_PER_SIDE}}`, `{{REMAINING_REPORTS_FOR_SIDE}}`, `{{MAX_SUBMITTED_EVIDENCE_BYTES}}`, `{{MAX_DIRECT_SUBMITTED_EVIDENCE_BYTES}}`, `{{MAX_EVIDENCE_UPLOAD_BYTES}}`, `{{MAX_EVIDENCE_CHUNK_BYTES}}`, `{{USED_SUBMITTED_EVIDENCE_FOR_SIDE}}`, `{{MAX_SUBMITTED_EVIDENCE_PER_SIDE}}`, `{{REMAINING_SUBMITTED_EVIDENCE_FOR_SIDE}}`, `{{MAX_EVIDENCE_READ_BYTES}}`, `{{MAX_EVIDENCE_READS_PER_OPPORTUNITY}}`, `{{MAX_EVIDENCE_READ_BYTES_PER_OPPORTUNITY}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.phase.openings` | `attorney/phase/openings.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OBJECTIVE}}`, `{{OPPORTUNITY_ID}}`, `{{QUESTION}}`, `{{JUDGMENT_STANDARD}}`, `{{DECISION_TOOLS}}` |
| `attorney.phase.arguments` | `attorney/phase/arguments.md` | Same as `attorney.phase.openings`. |
| `attorney.phase.rebuttals` | `attorney/phase/rebuttals.md` | Same as `attorney.phase.openings`. |
| `attorney.phase.surrebuttals` | `attorney/phase/surrebuttals.md` | Same as `attorney.phase.openings`. |
| `attorney.phase.closings` | `attorney/phase/closings.md` | Same as `attorney.phase.openings`. |
| `attorney.finalize` | `attorney/finalize.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}`, `{{DECISION_TOOLS}}` |
| `council.system` | `council/system.md` | `{{MEMBER_ID}}`, `{{DELIBERATION_ROUND}}`, `{{QUESTION}}`, `{{JUDGMENT_STANDARD}}`, `{{PERSONA_SECTION}}`, `{{RECORD}}`, `{{OPPORTUNITY_ID}}`, `{{OBJECTIVE}}` |
| `council.persona` | `council/persona.md` | `{{PERSONA}}`, `{{MEMBER_ID}}`, `{{MODEL}}`, `{{PERSONA_FILE}}`, `{{OPPORTUNITY_ID}}` |
| `council.direct.request` | `council/direct/request.md` | `{{COUNCIL_TOOL}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}` |
| `council.direct.repair` | `council/direct/repair.md` | `{{CORRECTION}}`, `{{REPAIR_KIND}}`, `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}`, `{{SIZE_BYTES}}`, `{{LIMIT_BYTES}}` |
| `council.direct.repair.malformed_arguments` | `council/direct/repair/malformed-arguments.md` | `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}` |
| `council.direct.repair.response_too_large` | `council/direct/repair/response-too-large.md` | `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}`, `{{SIZE_BYTES}}`, `{{LIMIT_BYTES}}` |
| `council.direct.repair.tool_call_count` | `council/direct/repair/tool-call-count.md` | `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}` |
| `council.direct.repair.wrong_tool` | `council/direct/repair/wrong-tool.md` | `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}` |
| `council.direct.repair.invalid_arguments` | `council/direct/repair/invalid-arguments.md` | `{{REASON}}`, `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}` |
| `council.api` | `council/api.md` | `{{COUNCIL_SYSTEM}}`, `{{COUNCIL_TOOL}}`, `{{SUBMISSION_FIELDS}}`, `{{MEMBER_ID}}`, `{{OPPORTUNITY_ID}}` |
| `council.preflight.system` | `council/preflight/system.md` | `{{MEMBER_ID}}`, `{{MODEL}}`, `{{PERSONA_FILE}}` |
| `council.preflight.user` | `council/preflight/user.md` | `{{MEMBER_ID}}`, `{{MODEL}}`, `{{PERSONA_FILE}}` |
| `observer` | `observer.md` | `{{CASE_ID}}` |

| Tool or schema-description prompt ID | `--prompt-dir` path |
| --- | --- |
| `tool.case_status` | `tools/case-status.md` |
| `tool.lawyer.get_case` | `tools/lawyer/get-case.md` |
| `tool.observer.get_case` | `tools/observer/get-case.md` |
| `tool.council.get_case` | `tools/council/get-case.md` |
| `tool.send_work_notes` | `tools/send-work-notes.md` |
| `tool.get_turn` | `tools/get-turn.md` |
| `tool.list_events` | `tools/list-events.md` |
| `tool.list_evidence` | `tools/list-evidence.md` |
| `tool.stat_evidence` | `tools/stat-evidence.md` |
| `tool.observer.stat_evidence` | `tools/observer/stat-evidence.md` |
| `tool.read_evidence_range` | `tools/read-evidence-range.md` |
| `tool.begin_evidence_upload` | `tools/begin-evidence-upload.md` |
| `tool.write_evidence_chunk` | `tools/write-evidence-chunk.md` |
| `tool.commit_evidence_upload` | `tools/commit-evidence-upload.md` |
| `tool.submit_evidence` | `tools/submit-evidence.md` |
| `tool.submit_decision` | `tools/submit-decision.md` |
| `tool.submit_council_answer` | `tools/submit-council-answer.md` |
| `tool.send_work_notes.property.notes` | `tools/send-work-notes/property-notes.md` |

The Go API exposes the same partial map as `Options.PromptFiles` and `Config.PromptFiles`; `PromptIDs` returns the accepted identifiers.  The case-owned Lawyer API supplies all lawyer prompts and tools to external clients, including clients reached through an MCP adapter.  Neither prompt resolution nor role execution depends on `adjservices`; the broader [prompt authoring guide](../docs/prompt-authoring.md) covers editing and evaluation practice.

## Lawyer API

The Lawyer API is available at `/lawyerapi/v1` on the `aard case` listener.  Lawyer roles are `plaintiff` and `defendant`, while `observer` is read-only.  Every request includes `case_id`, and lawyer requests also include `role_id`.

`GET /lawyerapi/v1/get` reads current role state and returns a ready turn when available.  `GET /lawyerapi/v1/wait` waits for a ready turn or state change, while `GET /lawyerapi/v1/status` returns role status.  `GET /lawyerapi/v1/result` returns terminal result information.  A ready turn includes the prompt, tool specifications, limits, remaining time, remaining attempts, and `opportunity_id`.

Tool calls use `POST /lawyerapi/v1/do`.  Each opportunity-bound call includes the current `opportunity_id`.  `case_status` is exempt.  A successful final filing consumes the opportunity and advances the case.

POST request bodies have a finite byte limit and must contain exactly one JSON value.  Tool arguments and nested filing payloads reject unknown keys, wrong types, and inconsistent evidence or report fields.  The API accepts an optional `call_id` as request metadata, and work-note records preserve it, but it does not deduplicate retries or retrieve a cached result.

Participants supply case, role, opportunity, tool, and arguments rather than Lean authority.  The runtime derives `opportunity_id`, `expected_state_version`, `role`, `phase`, and `member_id` from the owned state and current opportunity.  Lean requires exact equality for those fields and checks that the opportunity permits the requested operation.

Case identity, role identity, current-turn, and stale-opportunity errors retain specific response codes and do not consume an attempt.  Participant-controlled tool or payload errors return `tool_failed` and may consume an invalid-attempt allowance.  Storage, engine, event, and related infrastructure errors return `runtime_failure` and stop the affected active turn rather than being counted as participant input.

### Read A Turn

```bash
BASE=http://127.0.0.1:21345/lawyerapi/v1
curl -sS "$BASE/wait?case_id=arbd-1&role_id=plaintiff&timeout_ms=30000"
```

### Record Work Notes

```bash
curl -sS -X POST "$BASE/do" -H 'content-type: application/json' --data '{
  "case_id": "arbd-1",
  "role_id": "plaintiff",
  "opportunity_id": "arguments:plaintiff",
  "tool": "send_work_notes",
  "arguments": {
    "notes": "Inspect the case files, identify decisive facts, submit missing source material, and argue the supported score."
  }
}'
```

### Submit A Filing

```bash
curl -sS -X POST "$BASE/do" -H 'content-type: application/json' --data '{
  "case_id": "arbd-1",
  "role_id": "plaintiff",
  "opportunity_id": "openings:plaintiff",
  "tool": "submit_decision",
  "arguments": {
    "kind": "tool",
    "tool_name": "record_opening_statement",
    "payload": {
      "text": "The score will turn on the attribution and provenance of the confession."
    }
  }
}'
```

Lawyer tools include `case_status`, `get_case`, `send_work_notes`, evidence inspection and upload tools, and `submit_decision`.  Filing actions are `record_opening_statement`, `submit_argument`, `submit_rebuttal`, `submit_surrebuttal`, `deliver_closing_statement`, and `pass_phase_opportunity` when the procedure permits a pass.  Evidence submission, offered evidence, and technical reports are available during arguments, rebuttals, and surrebuttals.

Evidence reads verify a regular-file descriptor against the committed full digest and size before returning a bounded range.  The runtime rechecks the opportunity after file input and rejects a stale result without committing its budget or procedural effects.  Successful Lawyer reads create `evidence_read` events.

Direct submission and chunked upload share one admission path.  The runtime obtains the Lean transition, publishes the store object and submitted copy, writes a candidate manifest, rechecks the deadline, and then commits state, registries, and replay action together.  Failures before commit preserve the earlier in-memory state, while a later event-write failure or machine stop can leave committed state or unreferenced files because publication has no multi-file crash journal.

Observer tools include `case_status`, `get_case`, `get_turn`, `list_events`, and evidence inspection tools.  They cannot submit filings, evidence, or work notes, and their successful evidence reads create no participant read event or budget charge.  An Observer storage fault returns `runtime_failure` to that request without ending the active case.

## Council API

The Council API is available when `aard case` starts with `--council-backend councilapi`.  Calls use `/councilapi/v1` and identify both `case_id` and `member_id`.  A member receives its deliberation opportunity through `GET /councilapi/v1/wait` or `GET /councilapi/v1/get`.

Council tools include `get_case`, evidence inspection tools, and `submit_council_answer`.  Council POST bodies use the same one-value and byte-bounded JSON rule as the Lawyer API, and operation arguments reject unknown fields.  The advertised answer schema requires an integer from 0 through 100 and a string rationale grounded in the admitted record.  Go accepts a whole JSON number or numeric string, normalizes it to an integer in that range, and sends that normalized value to Lean.

The runtime derives the member's exact opportunity authority, and Lean checks that authority before recording the answer or failure.  A successful answer completes that member's participation, while a stale or mismatched request leaves state unchanged.  Successful Council API evidence reads consume the member's opportunity budget and create `evidence_read` events.

`POST /councilapi/v1/fail` reports a council-member failure for an active opportunity.  The request reason must identify an agent exit or output-limit failure accepted by the API.  The case marks that unanswered member failed and continues with the remaining seated members unless the complete-answer rule closes the case.

## Output Packet

`aard case` creates a missing output directory or accepts an existing empty directory.  It rejects a file or nonempty directory without changing the existing contents, then creates `.aard-output-claim` exclusively and confirms that the claim is the directory's sole entry.  The runtime removes the claim after publishing `case-manifest.json`.  An abrupt stop before removal can leave a stale claim that requires manual inspection.

Every completed or procedurally failed case writes a terminal packet under that claimed directory.  A process error can leave only the files published before the error, so the exact file set depends on how far initialization or execution progressed.  The following entries constitute the durable record when their stage has completed.

| File | Contents |
| --- | --- |
| `complaint.md` | Canonical complaint. |
| `case-manifest.json` | Run identity, start time, core version, and bound case API address. |
| `policy.json` | Effective policy values. |
| `runtime.json` | Effective runtime limits. |
| `run.json` | Final structured result.  This file has no schema field or artifact list. |
| `state.json` | Final additive Lean state schema `v1`. |
| `certificate.json` | Replay schema `aard.replay-certificate.v1`. |
| `council.json` | Council roster, final member statuses, request specifications, and failure details. |
| `council-turns/*/input.json` | Operator turn snapshot schema `aard.council-turn-snapshot.v0`, created for each council turn. |
| `council-turns/*/prompt.txt` | Exact rendered prompt created for that council turn. |
| `digest.md` | Human-readable summary. |
| `transcript.md` | Human-readable transcript with filings and council answers. |
| `events.ndjson` | UTC event log. |
| `work-notes.ndjson` | Off-record lawyer work notes, created on the first submitted note. |
| `evidence-manifest.json` | Shared evidence schema `aar.evidence-manifest.v0`. |
| `evidence-store/` | Stored evidence bytes, present when at least one item is stored. |
| `submitted-evidence/` | Copies created for accepted lawyer submissions. |

`run.json` contains case and run identifiers, times, status, error and failure data, phase, complaint, judgment standard, council backend, the member answer map, and direct-provider request accounting.  It also contains attorney, case-file, submitted-evidence, evidence, council, and event data, followed by `final_state` and `final_reason`.  The provider object counts direct availability and answer requests and sums usage or cost reported by those responses.  Generated packet files remain siblings in the output directory rather than entries in `run.json`.

State schema `v1` retains compatibility through defaulted additions for `evidence_catalog` and submitted-evidence lineage.  The initial catalog remains fixed, while accepted submissions and filings extend the case record.  Final rendering clones state and related mutable records while holding the case mutex, then writes from that owned snapshot after releasing the mutex.

### Inspection

```bash
jq '{status, phase, answers, final_reason, failure, provider}' "$out/run.json"
jq '.final_state.case.council_answers' "$out/run.json"
jq '{case_id, run_id, actions:(.actions|length), claimed_final_state_sha256}' "$out/certificate.json"
jq -r '[.timestamp,.role,.phase,.type] | @tsv' "$out/events.ndjson"
test ! -f "$out/work-notes.ndjson" || jq -r '[.timestamp,.role,.phase,(.notes|length)] | @tsv' "$out/work-notes.ndjson"
```

Use `transcript.md` to read the procedural record and `digest.md` to inspect the final answer set.  Use `certificate.json` with `aard verify-certificate` to replay accepted actions, and use the evidence manifest and store to inspect exact source bytes and custody metadata.  Use `events.ndjson` to reconstruct process sequence, while accounting for an event-write failure that can occur after an accepted state and certificate action commit.

## Policy And Limits

The default policy seats five council members and asks each for one integer from 0 through 100 with a short explanation.  It limits filing lengths, exhibits, technical reports, evidence submissions, uploads, and reads.  A policy JSON file can override those fields.  The output packet records the effective policy in `policy.json`.

The default runtime allows 900 seconds per lawyer turn, 240 seconds per council opportunity under either backend, 30 seconds per Lean engine call, a 128 KiB parsed response, three invalid attempts per opportunity, and 4096 direct council output tokens.  Command flags override the lawyer deadline, council timeout, engine timeout, response byte limit, and invalid-attempt limit.  A request specification may override the direct council output-token limit.

The Lean engine enforces phase order, exact authority, evidence commitments, lineage, source-state offers, filing and report limits, and council answers.  Go enforces the same overlapping commitments against stored bytes along with transport sizes, evidence-read budgets, deadlines, model calls, and filesystem custody.  The packet records all effective policy values in `policy.json` and the six runtime values in `runtime.json`.

## Runtime Ownership

One `runContext.mu` owns mutable case state, both role turn records, evidence and upload registries, events, and certificate actions.  Provider calls, HTTP response writes, and descriptor reads run outside that mutex.  Provider calls and descriptor reads revalidate the current opportunity before committing state or budget effects, while HTTP response writes do not mutate case state.  State-changing Lean calls remain serialized while holding the mutex and use the shorter of the engine timeout and any participant deadline.

The engine adapter starts each Lean invocation in a new process group.  Cancellation, timeout, and post-exit cleanup target that group, although a descendant that deliberately leaves the group falls outside this scope.  Only a valid rejection object with exit status 1 counts as a semantic rejection.  Malformed output, other nonzero exits, and surviving group members are process failures.

## Failure And Status

Lean `case.status` is `draft`, `active`, `closed`, or `failed`.  `case.phase` separately records the procedural phase, and a failed case can retain the phase in which failure occurred.  Terminal `run.json` status is `ok` or `failed`, while the generated council-member status records `seated` or `failed` independently.  Request-envelope and routing errors such as a wrong case, actor, turn, or opportunity leave state unchanged and do not consume a participant attempt.  Malformed participant tool arguments or filing payloads return `tool_failed` and may consume an invalid-attempt allowance.

A plaintiff or defendant deadline or invalid-attempt exhaustion records an `opportunity_failed` action through Lean and sets the whole case to `failed`.  The command can then exit zero after writing the terminal packet and a stdout summary containing `status: "failed"`.  The failure record identifies role, phase, opportunity, reason, message, and optional model information.

A council-member deadline, agent exit, request failure, or output failure marks that scheduled unanswered member `failed` and preserves prior answers.  Another eligible member continues when an answer remains due, while the ordinary complete-answer rule can close the case after the failure.  The final packet reports the member status, failure reason, and resulting answer map.

Storage, engine, event, and related infrastructure faults use `runtime_failure` at the role API boundary and stop the affected active operation.  Observer storage faults remain confined to the request and do not end the case.  A startup, configuration, engine-process, storage, publication, or other `aard case` failure exits nonzero and normally writes a JSON error summary to stdout.  After successful JSON reporting, the command suppresses a duplicate stderr diagnostic.  Stderr can contain flag or help output, an unreported error, or a diagnostic when writing the JSON summary fails.

## Running Examples

The repository examples contain complaint and initial-evidence files.  They do not start lawyer clients.  List them with the following command.

```bash
find examples -maxdepth 2 -name complaint.md -printf '%h\n' | sort
```

### Direct Council

Set the credential required by the selected pool before starting the case.  The command prints the Case API address to stderr and waits for plaintiff and defendant clients.  Direct council calls begin after both lawyers complete the closing phase.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY
.bin/aard case --complaint examples/ex1/complaint.md --council-pool ../common/data/personas/pool.jsonl --out-dir out/ex1-direct
```

### External Council

The `councilapi` backend uses external clients for final answers.  It exposes the Council API on the same address as the Lawyer API.  The case selects and validates the configured roster before opening its listener without making provider requests.

```bash
.bin/aard case --complaint examples/ex1/complaint.md --council-backend councilapi --council-pool ../common/data/personas/pool.jsonl --out-dir out/ex1-councilapi
```

### Case Packet

Case-packet construction does not start a case or call a model.  It uses the same verified complaint and initial-evidence selection as `aard case`, and identical inputs produce identical packet bytes and manifest content when each invocation uses fresh final output paths.  Existing packet or manifest targets are rejected because the command reserves both names exclusively before publication.

The command builds both artifacts in temporary files, reserves both final names, renames the packet, and then renames the manifest.  An ordinary error removes the temporary or reserved files owned by that invocation.  No crash journal repairs a machine stop between the two final renames.

```bash
.bin/aard case-packet --complaint examples/ex1/complaint.md --packet /tmp/ex1-case.tar.gz --manifest /tmp/ex1-case-packet.json
```

## Troubleshooting

If `aard case` fails before reporting its listener address, inspect the JSON error summary on stdout.  Stderr may contain flag output or a diagnostic when the command could not write that summary, but an ordinary reported error has no duplicate stderr message.  Common causes include an invalid complaint or policy, unreadable initial file, unavailable pool or persona, missing provider credentials, failed council preflight, and an unavailable Lean engine.  Early failures can leave a partial output directory containing only the configuration, evidence, claim, or initialization records published before the error.

If the case remains in a lawyer phase, query `/lawyerapi/v1/status` and `/lawyerapi/v1/get` for each lawyer.  The response identifies the active role, opportunity, deadline, and remaining attempts.  The case waits until the assigned client files, passes where permitted, or reaches its deadline.

If a lawyer fails by deadline or invalid attempts, inspect `events.ndjson`, the failure object in `run.json`, and `work-notes.ndjson` when that optional file exists.  The failure records role, phase, opportunity identifier, reason, and message.  A lawyer failure terminates the case.

If an external council client fails, inspect `events.ndjson` for `council_member_removed` and related opportunity events.  A supervising client can report the active member's failure through `/councilapi/v1/fail`.  The remaining members continue under the configured council rules.

Use an absent or empty output directory for each invocation.  `aard case` rejects a file or nonempty directory without reusing its contents, and `aard case-packet` rejects existing packet or manifest targets.  A stale `.aard-output-claim` or partial publication requires manual inspection before cleanup or a later run with a fresh path.

If certificate verification fails, read the reported check before inspecting other files.  Hash mismatches identify disagreement inside the packet, while an engine rejection identifies the numbered accepted action that failed replay.  An alternate engine path can determine whether the result depends on the engine build.
