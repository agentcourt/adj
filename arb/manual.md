# Agent Arbitration Manual

## Overview

Agent Arbitration, or AAR, runs a dispute about one proposition.  A complaint states the proposition, two lawyers build and argue the record, and a council decides whether the proposition has been demonstrated under the configured evidence standard.  The runtime keeps the case record, enforces turn order and limits, stores admitted evidence, records private lawyer work notes outside the record, and writes a final packet for later inspection.

`aar case` runs one case and exposes HTTP APIs for lawyer, observer, and council clients.  The executable can call council models directly, or it can wait for external council clients through the Council API.  External processes provide the lawyers in both modes.

The remaining commands prepare complaints and case packets or verify a completed case.  They share the complaint parser and proceeding implementation used by `aar case`.  Commands in this document assume the working directory is `arb/` unless stated otherwise.

## Operating Model

A case process owns the arbitration.  It owns the current phase, turn order, deadlines, attempt budgets, evidence registry, work-note log, council roster, and final output packet.  Lawyer, observer, and council clients read the case and act through its HTTP APIs.

The case command selects and checks the council before it starts the HTTP listener.  With the default `direct` council backend, the process calls each selected council model when deliberation begins.  With the `councilapi` backend, external clients read deliberation opportunities and submit votes through the same case process.

## Choosing A Command

| Goal | Command |
| --- | --- |
| Run one case and expose its Lawyer and Council APIs. | `aar case --complaint FILE --out-dir DIR`. |
| Build a deterministic case packet for an external service. | `aar case-packet --complaint FILE --packet case.tar.gz --manifest case-packet.json`. |
| Normalize or check a complaint file. | `aar complain` and `aar validate`. |
| Check a completed packet against its recorded engine actions. | `aar verify-certificate --dir DIR`. |

## Core Concepts

An AAR case begins with a complaint.  The complaint contains one proposition, usually written under a `# Proposition` heading.  The plaintiff lawyer tries to demonstrate that proposition, and the defendant lawyer tests the proof, identifies gaps, develops contrary evidence when available, and argues that the proposition has not been demonstrated.

The record contains lawyer filings, admitted evidence, technical reports, and council votes.  Initial case files enter the record when the case starts, and the engine receives their evidence identifiers, sizes, and canonical SHA-256 commitments.  Lawyers may also submit evidence during arguments, rebuttals, and surrebuttals.  Each filing may offer items from the initial catalog or from submissions already in the record.  Openings and closings may read evidence but may not submit new evidence.

The case proceeds through lawyer phases and then deliberation.  The lawyer phases are openings, arguments, rebuttals, surrebuttals, and closings.  The final council phase asks council members to vote `demonstrated` or `not_demonstrated` and explain their vote.  A configured vote threshold determines the final resolution.

The runtime distinguishes record evidence from work notes.  Evidence is part of the case record and may be cited by `evidence_id`.  Work notes are private operator-facing notes sent by lawyers through `send_work_notes`; they are stored in `work-notes.ndjson` and are not evidence, filings, or case events.

## Operator Guidance

Use AAR when the proceeding should decide whether one proposition has been demonstrated under an evidence standard.  Keep the proposition narrow enough that lawyers can search for evidence, test provenance, and argue the record within the configured filing limits.  Use AARD when the desired output is a numeric answer or supported degree; AAR reduces the merits question to `demonstrated` or `not_demonstrated`.

Treat the output directory as the case record for one run.  Preserve `case-manifest.json`, `run.json`, `state.json`, `certificate.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, `evidence-manifest.json`, `evidence-store/`, and `council-turns/` together.  Use `events.ndjson` to reconstruct process sequence, `transcript.md` to read the record, `digest.md` to check the outcome, `certificate.json` to replay-check the accepted state transitions, and `work-notes.ndjson` to review lawyer planning that stayed outside the evidentiary record.

## Repository Layout

| Path | Meaning |
| --- | --- |
| `runtime/cmd/aar/` | Go command-line package for `aar`. |
| `runtime/proceeding/` | Case runner, Lawyer API, Council API, evidence storage, rendering, and policy logic. |
| `runtime/lean/` | Go client for the Lean arbitration engine. |
| `runtime/spec/` | Complaint parsing. |
| `engine/` | Lean arbitration engine used by the Go runtime. |
| `etc/policy.json` | Default case policy. |
| `../prompts/arb/` | Complete editable AAR prompt set. |
| `../examples/` | Shared example cases.  Each example has a `complaint.md` and may have supporting case files. |
| `../common/data/personas/pool.jsonl` and `../common/etc/personas/` | Default direct-council request specifications and personas. |
| `../common/` | Shared model-request, provider, and persona packages used by the core procedures. |

## Build And Environment

Build from `arb/`.  `make build` builds the Lean engine and Go executable into `.bin/`.  The executable looks for `aarengine` in its directory unless `--engine` specifies another path.

```bash
make build
```

The direct Go build is:

```bash
go build -o .bin/aar ./runtime/cmd/aar
```

The Go runtime tests do not build the Lean proof tree.  The real-engine cases require a current `.bin/aarengine`; `make test` builds that binary before running the tests.  After `make aarengine`, the tests may also run directly from `arb/`:

```bash
go test -count=1 ./runtime/...
```

The council pool is a JSONL file containing request specifications and persona paths.  A local `pool.jsonl` takes precedence when `--council-pool` is omitted, followed by `<common-root>/data/personas/pool.jsonl`.  Relative persona paths resolve from the pool's base directory, and the output packet records the selected council roster.

Direct council calls support the registered Anthropic, DeepSeek, Google, Hugging Face, OpenAI, OpenRouter, and xAI endpoints.  The case command checks selected council members before it opens the case API listener, including when external clients will submit the final votes.  Provider failures carry one of four machine-readable classes: `provider_transient`, `provider_authentication`, `provider_request`, or `provider_protocol`.  Each provider-failure removal event records its class, and the terminal result records the last class.  The [model-endpoint guide](../docs/model-endpoints.md) defines credentials, pool records, request support, and endpoint-balanced selection.

## Complaint Files

A complaint is markdown.  If it contains a heading named `Proposition`, AAR uses that section as the proposition.  If no such heading exists, AAR treats the whole trimmed file as the proposition.

```markdown
# Proposition

The defendant sent the signed confession attached to the case packet.
```

Use `aar validate` before running a case if the complaint was written or transformed by another tool:

```bash
.bin/aar validate --complaint ../examples/ex01/complaint.md
```

On success, `aar validate` prints `ok`.

Use `aar complain` to normalize a situation file into canonical complaint form.  The command parses the input the same way as a complaint and writes a `# Proposition` complaint file.

```bash
.bin/aar complain \
  --situation work/my-case/situation.md \
  --out work/my-case/complaint.md
```

On success, `aar complain` prints the output path.

## Initial Evidence

When `aar case` starts without `--file`, the case runner scans the complaint directory for initial case files.  It identifies the configured complaint by file identity and skips it, a situation file, `README.md`, editor backup files ending in `~`, signing evidence, and directories.  Every selected source must be a regular file; a symbolic link to a regular file is accepted.  Before Lean initializes the case, the runtime copies each selected file into the evidence store and verifies the copy.  The runtime constructs the case's file view and immutable evidence catalog from that stored snapshot.  Files ending in `.txt`, `.md`, `.pem`, or `.b64` enter that view as readable text; other file types enter as byte-bearing evidence.

Use `--file` to provide explicit initial evidence.  The flag may be repeated.  Supplying any `--file` value replaces automatic complaint-directory scanning, so list every initial evidence file that belongs in the starting packet.  Explicit selection requires a regular source but includes the complaint when the caller names it.

```bash
.bin/aar case \
  --complaint work/my-case/complaint.md \
  --file work/my-case/source-a.pdf \
  --file 'work/my-case/captures/*.png' \
  --out-dir out/my-case
```

`aar case-packet` packages the same complaint and initial evidence selection into the service input format.  It writes a deterministic `case.tar.gz` and a `case-packet.json` manifest, using the proceeding package's automatic scan and explicit-file validation.  An external service can invoke this command before transporting the packet without importing the proceeding implementation.

## Commands

| Command | Purpose |
| --- | --- |
| `aar complain` | Write a canonical complaint from a situation markdown file. |
| `aar validate` | Validate that a complaint parses. |
| `aar case-packet` | Build deterministic `case.tar.gz` and `case-packet.json` inputs for an external service. |
| `aar case` | Run one case and expose the private Lawyer and optional Council APIs. |
| `aar verify-certificate` | Replay-check `certificate.json` against `state.json` using the Lean engine. |

Command help reports the current flags and defaults.  Each subcommand accepts `-h`, and the root command accepts `help SUBCOMMAND`.  The case and certificate help are available as follows:

```bash
.bin/aar help case
.bin/aar help case-packet
.bin/aar help verify-certificate
```

## `aar case`

`aar case` runs one case and waits for lawyers and, when configured, council members to act through HTTP APIs.  It writes the private case API base URL to stderr in the form `caseapi listening on http://127.0.0.1:PORT`.  It writes one JSON summary to stdout after the case ends.  That summary reports success, result, vote counts, run id, output directory, or a failure message.

Basic command:

```bash
.bin/aar case \
  --complaint ../examples/ex01/complaint.md \
  --prompt-dir ../prompts/arb \
  --out-dir out/ex01-direct
```

Important flags:

| Flag | Meaning |
| --- | --- |
| `--complaint` | Complaint markdown file.  Required. |
| `--out-dir` | Output directory for the run packet.  Required. |
| `--file` | Initial evidence file or glob.  May repeat. |
| `--policy` | Policy JSON file.  Defaults to `./etc/policy.json` when present. |
| `--council-size` | Override `policy.council_size`. |
| `--required-votes` | Override `policy.required_votes_for_decision`. |
| `--evidence-standard` | Override `policy.evidence_standard`. |
| `--prompt-file` | Prompt override in `ID=PATH` form.  May repeat. |
| `--prompt-dir` | Complete prompt directory.  Every non-overridden catalog file is required. |
| `--common-root` | Shared `common/` tree for council pool and personas. |
| `--council-pool` | Council JSONL request-spec pool.  Use an absolute path unless the pool lives under the common root. |
| `--council-endpoint` | Allowed council endpoint.  May repeat. |
| `--minimum-distinct-council-endpoints` | Minimum endpoint names represented in the completed council. |
| `--caseapi-addr` | Private Case API listen address.  Default: `127.0.0.1:0`. |
| `--council-backend` | `direct` or `councilapi`. |
| `--timeout-seconds` | Council LLM timeout override for direct council. |
| `--council-request-attempts` | Provider attempts per direct council request, from 1 through 4. |
| `--lawyer-timeout-seconds` | Lawyer turn timeout override. |
| `--engine-timeout-seconds` | Lean engine call timeout override. |
| `--max-response-bytes` | Parsed response byte limit override. |
| `--invalid-attempt-limit` | Invalid tool-call attempt limit override. |
| `--engine` | Lean engine binary. |
| `--run-id` | Run id override. |
| `--case-id` | Case id override.  Default: `arb-1` for direct `aar case`. |

The default council backend is `direct`.  In direct mode, the case runner selects council members and calls their configured model endpoints.  In `councilapi` mode, the case runner exposes `/councilapi/v1` and waits for external council agents to connect, read the record, and submit votes.

The private Case API has a health endpoint at `/health`.  It returns HTTP `200` with JSON containing `ok`, `case_id`, and `run_id` after the case process has bound the listener.  The listener address appears on stderr when it becomes available.

## Prompt Configuration

Prompt resolution follows four levels: an individual file override, the complete `--prompt-dir` set, the conventional file under the process working directory, and the compiled fallback.  `--prompt-file ID=PATH` may repeat and provides partial overrides.  A supplied prompt directory must contain every catalog filename that an individual override does not replace.

Conventional files live under `prompts/arb/` relative to the process working directory.  The checked-in set has that layout from the repository root; a command run from `arb/` selects it with `--prompt-dir ../prompts/arb`.  Each source undergoes token validation before runtime values are inserted, so unknown `{{...}}` sequences fail while brace text inside a runtime value remains literal.

A template may omit any available token, and replacement uses literal strings rather than Go template evaluation.  The `--prompt-dir` paths in the tables are relative to the supplied directory.  The case-runtime catalog contains 45 IDs: 27 instruction entries and 18 tool or schema-description entries.  Every entry in the second table accepts `{{TOOL_NAME}}` and `{{READ_ONLY}}`; the checked-in files contain the complete description and need neither token.

| Prompt ID | `--prompt-dir` path | Available replacement tokens |
| --- | --- | --- |
| `attorney.wrapper` | `attorney/wrapper.md` | `{{ATTORNEY_STANDING}}`, `{{ATTORNEY_COMMON}}`, `{{ATTORNEY_PHASE}}`, `{{ATTORNEY_FINALIZE}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.common` | `attorney/common.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OBJECTIVE}}`, `{{OPPORTUNITY_ID}}`, `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{MODEL_CAPABILITIES_SECTION}}`, `{{CURRENT_RECORD}}`, `{{LIMITS_SECTION}}`, `{{COUNCIL}}`, `{{VISIBLE_CASE_FILES_SECTION}}`, `{{WORKSPACE_SECTION}}`, `{{WORK_PRODUCT_SECTION}}`, `{{DECISION_TOOLS}}` |
| `attorney.standing` | `attorney/standing.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.capabilities` | `attorney/capabilities.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.workspace` | `attorney/workspace.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.limits.wrapper` | `attorney/limits/wrapper.md` | `{{TEXT_LIMITS_SECTION}}`, `{{EVIDENCE_LIMITS_SECTION}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.limits.text` | `attorney/limits/text.md` | `{{TEXT_CHAR_LIMIT}}`, `{{TARGET_TEXT_CHAR_LIMIT}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.limits.evidence` | `attorney/limits/evidence.md` | `{{MAX_EXHIBITS_PER_FILING}}`, `{{USED_EXHIBITS_FOR_SIDE}}`, `{{MAX_EXHIBITS_PER_SIDE}}`, `{{REMAINING_EXHIBITS_FOR_SIDE}}`, `{{MAX_REPORTS_PER_FILING}}`, `{{USED_REPORTS_FOR_SIDE}}`, `{{MAX_REPORTS_PER_SIDE}}`, `{{REMAINING_REPORTS_FOR_SIDE}}`, `{{MAX_SUBMITTED_EVIDENCE_BYTES}}`, `{{MAX_DIRECT_SUBMITTED_EVIDENCE_BYTES}}`, `{{MAX_EVIDENCE_UPLOAD_BYTES}}`, `{{MAX_EVIDENCE_CHUNK_BYTES}}`, `{{USED_SUBMITTED_EVIDENCE_FOR_SIDE}}`, `{{MAX_SUBMITTED_EVIDENCE_PER_SIDE}}`, `{{REMAINING_SUBMITTED_EVIDENCE_FOR_SIDE}}`, `{{MAX_EVIDENCE_READ_BYTES}}`, `{{MAX_EVIDENCE_READS_PER_OPPORTUNITY}}`, `{{MAX_EVIDENCE_READ_BYTES_PER_OPPORTUNITY}}`, `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}` |
| `attorney.phase.openings` | `attorney/phase/openings.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OBJECTIVE}}`, `{{OPPORTUNITY_ID}}`, `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{DECISION_TOOLS}}` |
| `attorney.phase.arguments` | `attorney/phase/arguments.md` | Same as `attorney.phase.openings`. |
| `attorney.phase.rebuttals` | `attorney/phase/rebuttals.md` | Same as `attorney.phase.openings`. |
| `attorney.phase.surrebuttals` | `attorney/phase/surrebuttals.md` | Same as `attorney.phase.openings`. |
| `attorney.phase.closings` | `attorney/phase/closings.md` | Same as `attorney.phase.openings`. |
| `attorney.finalize` | `attorney/finalize.md` | `{{ROLE}}`, `{{PHASE}}`, `{{OPPORTUNITY_ID}}`, `{{DECISION_TOOLS}}` |
| `council.system` | `council/system.md` | `{{MEMBER_ID}}`, `{{DELIBERATION_ROUND}}`, `{{PROPOSITION}}`, `{{EVIDENCE_STANDARD}}`, `{{PERSONA_SECTION}}`, `{{RECORD}}`, `{{OPPORTUNITY_ID}}`, `{{OBJECTIVE}}` |
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
| `tool.submit_council_vote` | `tools/submit-council-vote.md` |
| `tool.send_work_notes.property.notes` | `tools/send-work-notes/property-notes.md` |

The Go API exposes the same partial map as `Options.PromptFiles` and `Config.PromptFiles`; `PromptIDs` returns the accepted identifiers.  The case-owned Lawyer API supplies all lawyer prompts and tools to external clients, including clients reached through an MCP adapter.  Neither prompt resolution nor role execution depends on `adjservices`; the broader [prompt authoring guide](../docs/prompt-authoring.md) covers editing and evaluation practice.

## Lawyer API

The Lawyer API is available at `/lawyerapi/v1` on the `aar case` listener.  Lawyer roles are `plaintiff` and `defendant`; the `observer` role is read-only.  Every request includes `case_id`, and lawyer requests include `role_id`.

Use `GET /lawyerapi/v1/get` to read the current status.  If a turn is ready, the response includes a prompt, a `turn` object, available tool specs, limits, remaining time, attempts left, and an `opportunity_id`.  Use `GET /lawyerapi/v1/wait` to wait up to the requested timeout for a ready turn or state change.  Use `GET /lawyerapi/v1/status` for role status and `GET /lawyerapi/v1/result` for final result information.

Tool calls use `POST /lawyerapi/v1/do`.  Lawyer tool calls must include the current `opportunity_id` unless the tool is `case_status`.  A successful final filing consumes the opportunity and advances the case.  Tool-specific validation failures can count against the turn's invalid-attempt limit; identity errors, missing opportunity ids, and wrong-turn calls return errors without consuming a turn attempt.

Example read:

```bash
BASE=http://127.0.0.1:21345/lawyerapi/v1

curl -sS "$BASE/wait?case_id=arb-1&role_id=plaintiff&timeout_ms=30000"
```

Example work notes:

```bash
curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "arb-1",
    "role_id": "plaintiff",
    "opportunity_id": "arguments:plaintiff",
    "tool": "send_work_notes",
    "arguments": {
      "notes": "Plan: verify the signature file, compare it to the confession, then submit any missing provenance before filing."
    }
  }'
```

Example final filing:

```bash
curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "arb-1",
    "role_id": "plaintiff",
    "opportunity_id": "openings:plaintiff",
    "tool": "submit_decision",
    "arguments": {
      "kind": "tool",
      "tool_name": "record_opening_statement",
      "payload": {
        "text": "The record will show that the defendant sent the signed confession.",
        "offered_evidence": [],
        "technical_reports": []
      }
    }
  }'
```

Lawyer tools include `case_status`, `get_case`, `send_work_notes`, `list_evidence`, `stat_evidence`, `read_evidence_range`, `submit_evidence`, `begin_evidence_upload`, `write_evidence_chunk`, `commit_evidence_upload`, and `submit_decision`.  Evidence submission and upload tools are available during arguments, rebuttals, and surrebuttals.  The legal filing actions passed through `submit_decision` are `record_opening_statement`, `submit_argument`, `submit_rebuttal`, `submit_surrebuttal`, `deliver_closing_statement`, and `pass_phase_opportunity` when that pass action is allowed.  Observer tools include `case_status`, `get_case`, `get_turn`, `list_events`, `list_evidence`, `stat_evidence`, and `read_evidence_range`.  `get_turn` reports the current role, phase, deadline, and remaining attempts when a lawyer turn is active.

Successful Lawyer and Council evidence range reads create `evidence_read` events.  Observer range reads do not create events.  A storage, integrity, publication, engine, or event-write error during an active participant operation returns `runtime_failure`, preserves the attempt count, and ends that turn and case run.

The Lawyer and Council `/do` request formats accept `call_id` but do not use it to deduplicate a mutation or recover a prior response.  A caller cannot resolve a lost mutation response by resending the same `call_id`.  Mutation idempotency remains future work.

## Council API

The Council API is available when a case starts with `--council-backend councilapi`.  Calls go to `/councilapi/v1` and include `case_id` and `member_id`.  A council member uses `GET /councilapi/v1/wait` or `GET /councilapi/v1/get` to receive its deliberation opportunity, reads the record through evidence tools, and submits one vote through `POST /councilapi/v1/do`.

Council tools include `get_case`, `list_evidence`, `stat_evidence`, `read_evidence_range`, and `submit_council_vote`.  A council vote payload has `vote` and `rationale`.  The vote must be `demonstrated` or `not_demonstrated`, and the rationale should explain the vote from the admitted record.  After `submit_council_vote` succeeds, that council member is finished and should stop; it should not wait for later council opportunities assigned to other members.

The `POST /councilapi/v1/fail` endpoint lets a supervising process report council-member failure for an active opportunity.  The request reason must be `agent_exited` or `agent_output_limit_exceeded`.  An accepted failure dismisses that member, records the failure, and continues the case under the council rules.

## `aar verify-certificate`

`aar verify-certificate` checks a completed packet's replay certificate.  The command reads `certificate.json`, replays its initialization request and accepted public actions through the configured Lean engine, and compares the replayed final state to the certificate's claimed final-state hash.  It applies the configured engine timeout separately to initialization and each recorded action.  It also reads `state.json` from the same packet and requires that file to match the certificate hash.

The certificate contains the engine-visible transition record.  `initialize_request` contains the exact initial state, proposition, council roster, and initial evidence commitments sent to `initialize_case`.  `actions` contains the public actions the engine accepted, in order, with `action_type`, `actor_role`, `authority`, and `payload`; submitted-evidence payloads include their byte commitment and complete parent lineage when applicable.  Catalog, submission, and parent SHA-256 commitments contain exactly 64 lowercase hexadecimal characters, while the optional lineage members must be JSON strings when present.  The authority records the opportunity id, expected state version, role, phase, and scheduled council-member id checked for that transition, while `claimed_final_state_sha256` hashes the compact JSON encoding of `claimed_final_state`.

The name "certificate" overstates what this file is.  It is a package of the run's input, its accepted-action record, and its claimed final state, with hashes tying the package to the packet.  It carries no signature and no endorsement.  The word is borrowed from complexity theory, where a certificate is a witness that makes a claim checkable without search; here the check is a full re-execution of every engine transition, and it saves work only because the recorded actions remove any search and the model calls are not repeated.  A passing verification shows that the claimed outcome follows from the recorded history under the engine's rules.  It does not show that the recorded history is what actually happened: any internally legal history yields a passing package.  Establishing that the record is genuine requires attested execution or records held by the participants themselves.

Verification checks four conditions:

| Check | Failure reported |
| --- | --- |
| The claimed final-state hash matches `claimed_final_state`. | `certificate final state hash mismatch` |
| The packet's `state.json` matches the claimed final-state hash. | `packet final state mismatch` |
| The Lean engine accepts `initialize_case` and every recorded action. | `initialize_case rejected` or `certificate action N (...) rejected` |
| Replaying the recorded actions yields the claimed final state. | `replayed final state mismatch` |

Basic command:

```bash
.bin/aar verify-certificate --dir out/ex01-direct
```

Important flags:

| Flag | Meaning |
| --- | --- |
| `--dir` | Output packet directory containing `certificate.json` and `state.json`. |
| `--certificate` | Certificate path override. |
| `--state` | Final state path override. |
| `--engine` | Lean engine binary. |
| `--engine-timeout-seconds` | Maximum time for each Lean initialization or action call.  Default: 30 seconds. |

Successful verification prints a JSON object with `status: "ok"`, the case id, the run id when present, the accepted action count, and the final-state hash.  A certificate hash mismatch, packet-state mismatch, rejected replay action, or replayed final-state mismatch exits with an error.  The executable transitions check exact action authority, filing-time evidence references, canonical commitments, submission lineage, exhibit sizes, and report byte limits.  The Lean certificate theorems imply replay-time offer chronology and final-state `RecordIntegrity` from an accepted replay; the command does not execute `RecordIntegrity` as a separate predicate.  The command does not rehash `evidence-store/` or inspect lawyer work notes and logs.

`aar verify-certificate` performs this check explicitly after the case ends.  The case command writes the certificate but does not verify it before returning.  A later check can specify alternate certificate, state, or engine paths through command flags.

## Output Packet

Every completed or failed case writes a run packet under its output directory.  The directory may be absent or empty when the command starts.  The command creates a missing directory and rejects a file or nonempty directory without changing its contents.  It holds an exclusive `.aar-output-claim` file until it publishes the initial case manifest, so concurrent starters cannot accept the same directory.  An abrupt process exit may leave that claim, which makes the directory nonempty on a later attempt.  The exact file set depends on how far the case progressed.  These files constitute the main record:

| File | Contents |
| --- | --- |
| `complaint.md` | Canonical complaint. |
| `case-manifest.json` | Run identity, start time, core version, and bound case API address. |
| `policy.json` | Effective policy values. |
| `runtime.json` | Effective runtime limits. |
| `run.json` | Final structured result, including provider request, usage, and cost accounting and the last council `error_class`. |
| `state.json` | Final case state. |
| `certificate.json` | Initialization request, accepted public actions, claimed final state, and final-state hash for replay checking. |
| `council.json` | Council roster and related council metadata. |
| `digest.md` | Human-readable summary. |
| `transcript.md` | Human-readable transcript with filings and council votes. |
| `events.ndjson` | Event log.  Timestamps are UTC. |
| `work-notes.ndjson` | Off-record lawyer work notes. |
| `evidence-manifest.json` | Evidence metadata and custody information. |
| `evidence-store/` | Stored evidence bytes. |
| `submitted-evidence/` | Accepted lawyer-submitted evidence copies. |
| `council-turns/` | Council turn snapshots written before each council member acts. |

Useful inspection commands:

```bash
jq '{status, phase, resolution, final_reason, failure}' "$out/run.json"
jq '.final_state.case.council_votes' "$out/run.json"
jq '{case_id, run_id, actions:(.actions|length), claimed_final_state_sha256}' "$out/certificate.json"
jq -r '[.timestamp,.role,.phase,.event_type] | @tsv' "$out/events.ndjson"
jq -r '[.timestamp,.role,.phase,(.notes|length)] | @tsv' "$out/work-notes.ndjson"
```

Use `transcript.md` when reading the full procedural record.  Use `digest.md` when checking the final outcome and vote tally.  Use `certificate.json` with `aar verify-certificate` when checking that the recorded accepted actions replay to the packet's final state.  Use `evidence-manifest.json` and `evidence-store/` when exact source bytes, hashes, or evidence custody matter.

## Policy And Limits

The default policy has five council members, a preponderance evidence standard, and three required votes for a decision.  It allows three deliberation rounds, limits lawyer filings, offered exhibits, and technical reports, and sets evidence custody limits.  A policy JSON file can override those fields.  The complaint controls the proposition, while command flags set runtime limits such as turn deadlines, response bytes, and invalid attempts.  The output packet records the effective policy in `policy.json` and runtime limits in `runtime.json`.

Policy validation rejects a zero council size, a zero decision threshold, a threshold above the council size, and any threshold that is not a strict majority.  The strict-majority rule prevents one vote distribution from satisfying both substantive outcomes.  The engine enforces rules that change the legal state, including phase order, filing limits, vote thresholds, deliberation rounds, and admitted-material counts.  Go enforces transport limits and byte-transfer budgets before material reaches the engine.

The default runtime allows 900 seconds per lawyer turn, 240 seconds per direct council model call, 30 seconds per Lean engine call, four provider attempts per request, a 128 KiB non-content request or response budget, three invalid attempts per opportunity, and 4096 council output tokens.  Lawyer `/do` adds enough request-body allowance for direct evidence at the policy limit after base64 encoding, while Council `/do` request bodies retain the configured byte limit.  A request specification may set its own output-token limit; otherwise the runtime uses the 4096-token value.  `aar case` flags override the lawyer deadline, council timeout, engine timeout, provider attempts, response byte limit, and invalid-attempt limit.

A lawyer failure fails the case.  Examples include deadline expiration and exhausting invalid attempts.  A council member failure dismisses that member, records the failure, and lets the case continue under council rules.

## Failure And Status

Case status can be `draft`, an active phase name, `closed`, or `failed`.  `aar case` exits `0` for a procedural failure after it records the failure and writes the final packet.  Its stdout summary then contains `status: "failed"`, an error, and a structured failure object.

A process-level error exits nonzero.  Examples include an unreadable complaint, an invalid policy, an unavailable council pool, missing model credentials, or an unavailable Lean engine.  The command writes a JSON error summary to stdout and a diagnostic to stderr.

A provider error summary includes `error_class`, while a completed case whose council lost members to provider errors includes the last class.  The `provider` object counts logical requests and separately counts responses that supplied usage or cost.  Its token and cost totals sum the observed values, preserving partial accounting without representing an unknown value as zero.

## Running Examples

The repository examples contain complaint and evidence inputs.  They do not include lawyer clients.  List the available complaint directories with:

```bash
find ../examples -maxdepth 2 -name complaint.md -printf '%h\n' | sort
```

Start `ex01` with the direct council backend after setting the credentials required by the shared council pool.  The command prints the case API base URL to stderr and waits for plaintiff and defendant clients.  Those clients act through the Lawyer API described above.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aar case \
  --complaint ../examples/ex01/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex01-direct
```

The `councilapi` backend uses external clients for the final votes.  It exposes the Council API on the same address as the Lawyer API.  The command still selects and checks the configured council before starting the listener.

```bash
.bin/aar case \
  --complaint ../examples/ex01/complaint.md \
  --council-backend councilapi \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex01-councilapi
```

Case-packet construction can run without starting a case or calling a model.  It uses the same complaint and initial-evidence selection as `aar case`.  The command writes a deterministic archive and manifest:

```bash
.bin/aar case-packet \
  --complaint ../examples/ex01/complaint.md \
  --packet /tmp/ex01-case.tar.gz \
  --manifest /tmp/ex01-case-packet.json
```

## Troubleshooting

If `aar case` fails before reporting its listener address, inspect the JSON error summary and stderr diagnostic.  Common causes are an invalid complaint or policy, an unreadable pool or persona, a missing credential for a selected endpoint, a failed council availability check, and an unavailable Lean engine.  The process creates or accepts an empty output directory and writes the initial case manifest before council selection, so a later initialization failure may leave a partial directory.

If the case remains on one lawyer phase, query `/lawyerapi/v1/status` and `/lawyerapi/v1/get` for each lawyer role.  The response identifies the active role, opportunity, deadline, and remaining attempts.  A case waits until the assigned client makes a final filing or the lawyer deadline expires.

If a lawyer fails by deadline or invalid attempts, inspect `events.ndjson`, `work-notes.ndjson`, and the failure object in `run.json`.  The failure identifies the role, phase, opportunity id, reason, and message.  A lawyer failure fails the case.

If an external council client fails, inspect `events.ndjson` for `council_member_removed` and related opportunity events.  The supervising client can report a current member failure through `/councilapi/v1/fail`.  The remaining members continue under the configured council rules.

Each invocation requires an absent or empty output directory.  A process-level error may leave a partial directory, which can be preserved for diagnosis or moved before a retry uses the original path.  An abrupt exit during initial case-manifest publication can leave `.aar-output-claim` as the only file.  Starting against any such nonempty directory fails before writing another record.

If certificate verification fails, read the reported check before inspecting other artifacts.  Hash mismatches indicate disagreement within the packet, while an engine rejection identifies the numbered accepted action that failed replay.  An alternate engine path can determine whether the failure follows from using a different engine build.
