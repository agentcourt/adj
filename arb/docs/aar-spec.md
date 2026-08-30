# AAR Process and HTTP Specification

## Scope

This specification defines the external behavior of one `aar case` process.  It covers process startup, standard streams, the private Lawyer and Council APIs, records, results, and participant failures.  The `adjservices` repository owns managed case admission and public routing.

## Process Model

`aar case` runs one arbitration from a complaint to a terminal state.  Its flags select the complaint, identifiers, output directory, private API address, policy, evidence files, council pool, prompts, limits, engine, and council backend.  The process owns the case state and every participant opportunity until completion.  Each Lean initialization, transition, or opportunity query has a 30-second default limit, which `--engine-timeout-seconds` overrides with a positive number of seconds.  The output path may be absent or an existing empty directory; startup rejects a file or nonempty directory before writing any record.  Startup claims an accepted directory with an exclusive `.aar-output-claim` file until it publishes the initial case manifest.  A concurrent starter fails, while an abrupt process exit may leave a claim that causes a later start to reject the directory as nonempty.

When the private API starts, the command writes a diagnostic line to standard error with the prefix `caseapi listening on `.  The suffix contains the base URL without a role path.  A caller that supplies the listen address can instead poll `/health` for readiness.

The command writes one JSON summary line to standard output when the case ends.  A normal result uses `status: "ok"` and reports the resolution, while a recorded lawyer failure uses `status: "failed"` and can still exit zero.  Startup, configuration, engine, storage, and other process failures return a nonzero exit status.  An expired engine-call limit terminates that engine process group and returns an engine error.

## Common HTTP Rules

The private API listens only for the running case.  Requests identify the configured `case_id`, and mutating participant requests include the current opportunity id.  The case process binds the resulting engine action to that opportunity's current state version, role, phase, and scheduled council member, then validates identity, deadline, attempt budget, evidence rules, and the requested operation.

JSON responses include `ok` unless an endpoint serves bytes.  Request failures use an HTTP error status and a structured error object.  A procedurally invalid tool call can return HTTP 200 with `ok: false` because the case process received and rejected the operation.

Lawyer and Council `/do` request objects may contain `call_id`, but the current process does not deduplicate calls or retain a prior result by that identifier.  A lost mutation response therefore leaves the caller unable to determine from `call_id` whether the operation completed.  Mutation idempotency remains future work.

The process snapshots every initial evidence file into its verified content store before creating the Lean initial state.  That state contains a sorted catalog of evidence identifier, size, and canonical SHA-256 commitments, where each digest consists of exactly 64 lowercase hexadecimal characters.  Submitted-item and parent commitments use the same digest representation.  The catalog remains unchanged throughout every accepted transition, while later filings may offer only a catalog item or an earlier submitted item.  The public evidence-submission schemas expose `parent_evidence_id` and `derivation_method` as optional strings, and derived evidence supplies both as nonempty values.  A public request containing `parent_sha256` fails because the runtime resolves that digest from the named record item.  The internal Lean action contains all three nonempty lineage strings, and its parser rejects a present non-string lineage member.

## Lawyer API

The Lawyer API lives under `/lawyerapi/v1`.  Lawyer roles are `plaintiff` and `defendant`, and `observer` provides read-only access.  Every request includes the case id and applicable role id.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/lawyerapi/v1/get` | Return current role status, prompt, tools, limits, and turn. |
| `GET` | `/lawyerapi/v1/wait` | Wait for a role state change or timeout. |
| `GET` | `/lawyerapi/v1/status` | Return compact case and turn status. |
| `GET` | `/lawyerapi/v1/result` | Return the terminal result or pending status. |
| `POST` | `/lawyerapi/v1/do` | Execute one support, evidence, work-note, or legal operation. |

A ready turn includes its opportunity id, phase, prompt, available tools, limits, time remaining, and attempts remaining.  A mutating request supplies that opportunity id with the tool name and arguments.  Waiting, completed, and failed cases report `waiting`, `done`, and `failed` status values.

Lawyer operations include case reads, private work notes, evidence listing, evidence metadata, bounded evidence reads, direct evidence submission, chunked upload, and legal-decision submission.  Evidence reads remain available across lawyer phases.  Evidence submission remains limited to the phases defined by the procedure.

## Council API

The Council API is active when `--council-backend councilapi` selects external council members.  Requests under `/councilapi/v1` identify the case and bound member.  Each seated member receives an opportunity during deliberation and can read admitted evidence before voting.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/councilapi/v1/get` | Return member status, prompt, tools, limits, and turn. |
| `GET` | `/councilapi/v1/wait` | Wait for a member state change or timeout. |
| `POST` | `/councilapi/v1/do` | Read evidence or submit a vote. |
| `POST` | `/councilapi/v1/fail` | Record failure for the active member opportunity. |

Council operations include case reads, evidence listing, evidence metadata, bounded evidence reads, and `submit_council_vote`.  A vote states `demonstrated` or `not_demonstrated` and supplies a rationale.  The runtime supplies the trusted member identity rather than accepting authority from the vote payload.

## Results and Failures

`run.json` records the final state, resolution, council roster, votes, admitted evidence, final reason, the council cost reported by successful OpenRouter generation lookups, and the last provider error class when a provider failure removed a council member.  `state.json`, `certificate.json`, `events.ndjson`, `evidence-manifest.json`, `transcript.md`, and `digest.md` are sibling files in the durable case packet.  Evidence submission publishes verified bytes and a candidate manifest before the runtime advances its state and replay-action list, so a publication error cannot create an accepted certificate action without its evidence record.  The publication layer can reuse a matching immutable file during a later internal attempt, while a publication failure returned by a participant handler ends the active turn and process run without consuming an invalid-attempt allowance.

A lawyer failure ends the arbitration and records a structured failure.  The failure identifies the role, phase, opportunity, reason, and message.  A council-member failure dismisses that member and permits the remaining seated council to continue when the policy allows it.

## Test Obligations

Process tests start the real command and communicate through the private HTTP APIs.  They verify startup, opportunity identity, deadlines, attempt exhaustion, evidence custody, participant failure, terminal summaries, and durable records.  Replay tests verify that `certificate.json` reproduces the recorded terminal state through the selected Lean engine.
