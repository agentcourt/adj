# Agent Arbitration Degree Manual

## Overview

Agent Arbitration Degree, or AARD, runs an arbitration about one question and returns degree answers.  A complaint states the question, two lawyers build and argue the record, and each council member answers with an integer from 0 through 100 under the configured judgment standard.  The runtime enforces the procedure, stores the record, and writes a packet for later inspection.

`aard case` runs one case and exposes HTTP APIs for lawyer, observer, and council clients.  The executable can call council models directly, or it can wait for external council clients through the Council API.  External processes provide the lawyers in both modes.

The other commands prepare complaints and deterministic case packets or verify a completed case.  They share the complaint parser and proceeding implementation used by `aard case`.  Commands in this manual assume the working directory is `arbd/` unless stated otherwise.

## Operating Model

A case process owns the arbitration, including phase, turn order, deadlines, attempt budgets, evidence, work notes, council roster, and final output.  Lawyer, observer, and council clients read the case and act through its HTTP APIs.  Clients do not need access to the case output directory.  The case process writes every accepted procedural action to the durable record.

The case command samples and checks the council before it starts the HTTP listener.  With the default `direct` council backend, the process calls each selected council model when deliberation begins.  With the `councilapi` backend, external clients read deliberation opportunities and submit answers through the same case process.

## Choosing A Command

| Goal | Command |
| --- | --- |
| Run one case and expose its Lawyer and Council APIs. | `aard case --complaint FILE --out-dir DIR`. |
| Build a deterministic case packet for an external service. | `aard case-packet --complaint FILE --packet case.tar.gz --manifest case-packet.json`. |
| Normalize or check a complaint file. | `aard complain` and `aard validate`. |
| Check a completed packet against its recorded engine actions. | `aard verify-certificate --dir DIR`. |

## Core Concepts

An AARD case begins with a complaint containing one question.  The plaintiff argues for a higher score when the record supports it, while the defendant tests the evidence, identifies gaps, develops contrary evidence, and argues for a lower score or a narrower supported range.  The case proceeds through openings, arguments, rebuttals, surrebuttals, closings, and council deliberation.  Each council member submits one integer answer from 0 through 100 and a rationale grounded in the admitted record.

The record contains lawyer filings, admitted evidence, technical reports, and council answers.  Initial case files enter the record when the case starts, and lawyers may submit further evidence during arguments, rebuttals, and surrebuttals.  Openings and closings may read evidence but may not submit new evidence.  Work notes remain outside the evidentiary record in `work-notes.ndjson`.

## Operator Guidance

Use AARD for a question whose supported answer falls on a scale from 0 through 100.  The question should permit lawyers to identify evidence, challenge provenance, and argue an answer range within the filing limits.  Preserve `run.json`, `state.json`, `certificate.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, `evidence-manifest.json`, and `evidence-store/` together.  These files record the outcome, engine state, procedural sequence, off-record planning, and admitted evidence for one case.  Use `aard verify-certificate` to check that the accepted actions reproduce the recorded final state.

## Repository Layout

| Path | Meaning |
| --- | --- |
| `runtime/cmd/aard/` | Go command-line package for `aard`. |
| `runtime/proceeding/` | Case runner, Lawyer API, Council API, evidence storage, rendering, and policy logic. |
| `runtime/lean/` | Go client for the Lean engine. |
| `runtime/spec/` | Complaint parser. |
| `engine/` | Lean degree-arbitration engine and proofs. |
| `etc/policy.json` | Default case policy. |
| `prompts/` | Lawyer and council prompt files used by the case runner. |
| `attorney-instructions/` | Standing instructions supplied to lawyer clients. |
| `examples/` | Complaints and initial case files. |

## Build And Environment

Build from `arbd/` with `make build`.  The target builds the Lean engine and the Go command into `.bin/`.  A direct Go build can rebuild the command after a Go-only change.

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

Direct council calls require the credential named by each selected request specification.  Current OpenAI and OpenRouter endpoints use `OPENAI_API_KEY` and `OPENROUTER_API_KEY`, respectively.  The default pool path is a local `pool.jsonl` when present, followed by `<common-root>/data/personas/pool.jsonl`.  Persona paths resolve from the pool directory and then from the shared common tree.

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

When `aard case` starts without `--file`, it scans the complaint directory for initial case files.  The scan skips the complaint, a situation file, `README.md`, editor backups ending in `~`, signing evidence, and directories.  Text-like files enter the record as readable text evidence, while other files enter as byte-bearing evidence.

The repeatable `--file` flag selects explicit initial evidence.  Supplying any `--file` value replaces automatic directory scanning.  Every file required in the initial record must therefore appear in the explicit selection.

```bash
.bin/aard case --complaint work/my-case/complaint.md --file work/my-case/source-a.pdf --file 'work/my-case/captures/*.png' --out-dir out/my-case
```

`aard case-packet` applies the same complaint and initial-evidence selection without starting a case.  It writes a deterministic gzip archive and a JSON manifest for an external service.  The service can transport those files without importing the proceeding implementation.

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

`aard verify-certificate` checks a completed packet's replay certificate.  It reads `certificate.json`, replays initialization and every accepted public action through the selected Lean engine, and compares the result with the claimed final-state hash.  It also requires `state.json` to match that hash.

The certificate contains the engine-visible transition record.  Its `initialize_request` field contains the initial state, degree question, and council roster sent to `initialize_case`.  Its `actions` field contains the accepted public actions in order.  Its `claimed_final_state` and `claimed_final_state_sha256` fields identify the asserted terminal state.

The file carries no signature or endorsement.  A passing check establishes that the claimed outcome follows from the recorded actions under the selected engine.  Authenticating the recorded history requires an independent custody or attestation mechanism.

| Check | Failure reported |
| --- | --- |
| The claimed final-state hash matches `claimed_final_state`. | `certificate final state hash mismatch` |
| The packet's `state.json` matches the claimed final-state hash. | `packet final state mismatch` |
| The Lean engine accepts initialization and every recorded action. | `initialize_case rejected` or `certificate action N (...) rejected` |
| Replaying the actions yields the claimed final state. | `replayed final state mismatch` |

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

Successful verification prints a JSON object containing `status: "ok"`, case and run identifiers, the accepted-action count, and the final-state hash.  A failed check exits with an error that identifies the first mismatch or rejected engine action.  The command checks engine transitions and does not inspect work notes, logs, or evidence bytes.

## `aard case`

`aard case` initializes one case and waits for its lawyer clients.  It writes the private Case API base URL to stderr as `caseapi listening on http://127.0.0.1:PORT`.  After termination, it writes one JSON summary to stdout.  The summary reports the answers and output path or a structured failure.

### Example

```bash
.bin/aard case --complaint examples/ex1/complaint.md --out-dir out/ex1-direct
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
| `--attorney-instructions` | Standing attorney instructions file. |
| `--prompt-dir` | Prompt directory override. |
| `--attorney-common-prompt` | Attorney common prompt file override. |
| `--attorney-arguments-prompt` | Attorney arguments prompt file override. |
| `--attorney-rebuttals-prompt` | Attorney rebuttals prompt file override. |
| `--common-root` | Shared `common/` tree for council pool and personas. |
| `--council-pool` | Council JSONL request-spec pool. |
| `--caseapi-addr` | Private Case API listen address.  Default: `127.0.0.1:0`. |
| `--council-backend` | `direct` or `councilapi`. |
| `--timeout-seconds` | Direct council model timeout override. |
| `--lawyer-timeout-seconds` | Lawyer turn timeout override. |
| `--max-response-bytes` | Parsed response byte limit override. |
| `--invalid-attempt-limit` | Invalid tool-call attempt limit override. |
| `--engine` | Lean engine binary. |
| `--run-id` | Run identifier override. |
| `--case-id` | Case identifier override.  Default: `arbd-1`. |

The default council backend is `direct`.  Direct mode samples council members and calls their configured model endpoints after lawyer closings.  `councilapi` mode exposes council opportunities for external clients while retaining the same roster and engine procedure.

The Case API provides `GET /health` on the listener.  It returns HTTP `204` after the process has bound its address.  The listener address printed to stderr becomes the base for both role APIs.

## Lawyer API

The Lawyer API is available at `/lawyerapi/v1` on the `aard case` listener.  Lawyer roles are `plaintiff` and `defendant`, while `observer` is read-only.  Every request includes `case_id`, and lawyer requests also include `role_id`.

`GET /lawyerapi/v1/get` reads current role state and returns a ready turn when available.  `GET /lawyerapi/v1/wait` waits for a ready turn or state change, while `GET /lawyerapi/v1/status` returns role status.  `GET /lawyerapi/v1/result` returns terminal result information.  A ready turn includes the prompt, tool specifications, limits, remaining time, remaining attempts, and `opportunity_id`.

Tool calls use `POST /lawyerapi/v1/do`.  Each opportunity-bound call includes the current `opportunity_id`; `case_status` is exempt.  A successful final filing consumes the opportunity and advances the case.  Tool validation failures can consume an attempt, while identity and turn-selection errors do not.

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
      "text": "The score will turn on the attribution and provenance of the confession.",
      "offered_evidence": [],
      "technical_reports": []
    }
  }
}'
```

Lawyer tools include `case_status`, `get_case`, `send_work_notes`, evidence inspection and upload tools, and `submit_decision`.  Filing actions are `record_opening_statement`, `submit_argument`, `submit_rebuttal`, `submit_surrebuttal`, `deliver_closing_statement`, and `pass_phase_opportunity` when the procedure permits a pass.  Evidence submission is available during arguments, rebuttals, and surrebuttals.

Observer tools include `case_status`, `get_case`, `get_turn`, `list_events`, and evidence inspection tools.  They cannot submit filings, evidence, or work notes.  `get_turn` reports the active role, phase, deadline, and remaining attempts.

## Council API

The Council API is available when `aard case` starts with `--council-backend councilapi`.  Calls use `/councilapi/v1` and identify both `case_id` and `member_id`.  A member receives its deliberation opportunity through `GET /councilapi/v1/wait` or `GET /councilapi/v1/get`.

Council tools include `get_case`, evidence inspection tools, and `submit_council_answer`.  An answer contains integer `answer` from 0 through 100 and string `rationale` grounded in the admitted record.  A successful submission completes that member's participation.

`POST /councilapi/v1/fail` reports a council-member failure for an active opportunity.  The request reason must identify an agent exit or output-limit failure accepted by the API.  The case records the dismissal and continues with the remaining seated members.

## Output Packet

Every completed or procedurally failed case writes a packet under its output directory.  The exact file set depends on how far the case progressed.  The following files constitute the durable core record.

| File | Contents |
| --- | --- |
| `complaint.md` | Canonical complaint. |
| `policy.json` | Effective policy values. |
| `runtime.json` | Effective runtime limits. |
| `run.json` | Final structured result. |
| `state.json` | Final case state. |
| `certificate.json` | Initialization, accepted public actions, claimed final state, and final-state hash. |
| `council.json` | Council roster, final member statuses, request specifications, and failure details. |
| `digest.md` | Human-readable summary. |
| `transcript.md` | Human-readable transcript with filings and council answers. |
| `events.ndjson` | UTC event log. |
| `work-notes.ndjson` | Off-record lawyer work notes. |
| `evidence-manifest.json` | Evidence metadata and custody information. |
| `evidence-store/` | Stored evidence bytes. |
| `submitted-evidence/` | Accepted lawyer-submitted evidence copies. |

### Inspection

```bash
jq '{status, phase, answers, final_reason, failure}' "$out/run.json"
jq '.final_state.case.council_answers' "$out/run.json"
jq '{case_id, run_id, actions:(.actions|length), claimed_final_state_sha256}' "$out/certificate.json"
jq -r '[.timestamp,.role,.phase,.event_type] | @tsv' "$out/events.ndjson"
jq -r '[.timestamp,.role,.phase,(.notes|length)] | @tsv' "$out/work-notes.ndjson"
```

Use `transcript.md` to read the procedural record and `digest.md` to inspect the final answer set.  Use `certificate.json` with `aard verify-certificate` to replay accepted actions.  Use the evidence manifest and store to inspect exact source bytes and custody metadata.  Use `events.ndjson` to reconstruct process sequence.

## Policy And Limits

The default policy seats five council members and asks each for one integer from 0 through 100 with a short explanation.  It limits filing lengths, exhibits, technical reports, evidence submissions, uploads, and reads.  A policy JSON file can override those fields.  The output packet records the effective policy in `policy.json`.

The default runtime allows 900 seconds per lawyer turn, 240 seconds per direct council model call, a 128 KiB parsed response, three invalid attempts per opportunity, and 4096 council output tokens.  Command flags override the lawyer deadline, council timeout, response byte limit, and invalid-attempt limit.  A request specification may override the direct council output-token limit.

The Lean engine enforces phase order and accepted procedural actions.  Go enforces transport sizes, evidence byte budgets, deadlines, model calls, and filesystem custody before an action reaches the engine.  The packet records the effective runtime values in `runtime.json`.

## Failure And Status

Case status can be `draft`, an active phase, `closed`, or `failed`.  A recorded procedural failure exits with status zero after the command writes the packet and a stdout summary containing `status: "failed"`.  Lawyer deadline expiration and invalid-attempt exhaustion are examples of recorded procedural failures.

A process-level error exits nonzero.  Examples include an unreadable complaint, invalid policy, unavailable pool, missing provider credentials, or unavailable engine.  The command writes a JSON error summary to stdout and a diagnostic to stderr.

A council-member failure dismisses that member and records the reason.  The remaining seated members continue under the degree-arbitration procedure.  The final packet reports member status and any resulting answer set.

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

The `councilapi` backend uses external clients for final answers.  It exposes the Council API on the same address as the Lawyer API.  The case still samples and validates the configured roster before opening its listener.

```bash
.bin/aard case --complaint examples/ex1/complaint.md --council-backend councilapi --council-pool ../common/data/personas/pool.jsonl --out-dir out/ex1-councilapi
```

### Case Packet

Case-packet construction does not start a case or call a model.  It uses the same complaint and initial-evidence selection as `aard case`.  Repeating the command over identical inputs produces identical packet bytes and manifest content.

```bash
.bin/aard case-packet --complaint examples/ex1/complaint.md --packet /tmp/ex1-case.tar.gz --manifest /tmp/ex1-case-packet.json
```

## Troubleshooting

If `aard case` fails before reporting its listener address, inspect the JSON error summary and stderr diagnostic.  Common causes include an invalid complaint or policy, unreadable pool or persona, missing provider credentials, failed council preflight, and an unavailable Lean engine.  Early failures can leave a partial output directory because initialization writes files before council sampling.

If the case remains in a lawyer phase, query `/lawyerapi/v1/status` and `/lawyerapi/v1/get` for each lawyer.  The response identifies the active role, opportunity, deadline, and remaining attempts.  The case waits until the assigned client files, passes where permitted, or reaches its deadline.

If a lawyer fails by deadline or invalid attempts, inspect `events.ndjson`, `work-notes.ndjson`, and the failure object in `run.json`.  The failure records role, phase, opportunity identifier, reason, and message.  A lawyer failure terminates the case.

If an external council client fails, inspect `events.ndjson` for `council_member_removed` and related opportunity events.  A supervising client can report the active member's failure through `/councilapi/v1/fail`.  The remaining members continue under the configured council rules.

Use a distinct output directory for each invocation.  Reusing a directory can mix files from different cases because `aard case` writes its packet in place.  A partial directory after a process error records only the files written before that error.

If certificate verification fails, read the reported check before inspecting other files.  Hash mismatches identify disagreement inside the packet, while an engine rejection identifies the numbered accepted action that failed replay.  An alternate engine path can determine whether the result depends on the engine build.
