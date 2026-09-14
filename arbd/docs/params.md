# Parameter Surface

AARD separates four classes of input: complaint content, procedural policy, evidence-custody policy, and runtime limits.  The complaint states the question, procedural policy defines the engine-visible arbitration, custody policy bounds byte transport and inspection, and runtime limits bound the processes that conduct the case.  Procedure and custody fields share `policy.json`, but only the procedural subset enters the Lean policy.

The [case command](../runtime/cmd/aard/case.go) loads these values and constructs the [proceeding options](../runtime/proceeding/types.go).  The [policy layer](../runtime/proceeding/policy.go) supplies defaults and rejects invalid combinations before initialization.  The [Lean core](../engine/AARD/Core.lean) checks the engine-visible policy again while enforcing each accepted transition.

## Parameter Groups

| Group | Parameter | Purpose | Primary enforcement |
|---|---|---|---|
| Complaint | `question` | Quantitative question decided by the council. | Parser and Lean state |
| Procedure | `council_size` | Number of council members seated for the case. | Go startup and Lean |
| Procedure | `judgment_standard` | Standard applied to the record and 0–100 answer. | Go startup, Lean, and prompts |
| Procedure | `max_opening_chars` | Opening text limit. | Lean |
| Procedure | `max_argument_chars` | Argument text limit. | Lean |
| Procedure | `max_rebuttal_chars` | Rebuttal text limit. | Lean |
| Procedure | `max_surrebuttal_chars` | Surrebuttal text limit. | Lean |
| Procedure | `max_closing_chars` | Closing text limit. | Lean |
| Procedure | `max_exhibits_per_filing` | Offered items in one filing. | Go and Lean |
| Procedure | `max_exhibits_per_side` | Offered items by one side across the case. | Lean |
| Procedure | `max_exhibit_bytes` | Bytes in one offered item. | Go and Lean |
| Procedure | `max_reports_per_filing` | Technical reports in one filing. | Go and Lean |
| Procedure | `max_reports_per_side` | Technical reports by one side across the case. | Lean |
| Procedure | `max_report_title_bytes` | UTF-8 bytes in one report title. | Go and Lean |
| Procedure | `max_report_summary_bytes` | UTF-8 bytes in one report summary. | Go and Lean |
| Procedure | `max_submitted_evidence_per_side` | Admitted submissions by one side across the case. | Go and Lean |
| Procedure | `max_submitted_evidence_bytes` | Bytes in one admitted submission. | Go and Lean |
| Custody | `max_direct_submitted_evidence_bytes` | Bytes in one JSON or base64 submission. | Go |
| Custody | `max_evidence_upload_bytes` | Bytes in one chunked upload. | Go |
| Custody | `max_evidence_chunk_bytes` | Bytes in one upload chunk. | Go |
| Custody | `max_evidence_read_bytes` | Bytes returned by one evidence read. | Go |
| Custody | `max_evidence_reads_per_opportunity` | Evidence reads during one participant opportunity. | Go |
| Custody | `max_evidence_read_bytes_per_opportunity` | Total returned evidence bytes during one opportunity. | Go |
| Runtime | `council_llm_timeout_seconds` | Total deadline for one direct or Council API opportunity. | Go |
| Runtime | `lawyer_turn_timeout_seconds` | Lawyer opportunity deadline. | Go |
| Runtime | `engine_call_timeout_seconds` | Maximum duration of one Lean invocation. | Go |
| Runtime | `max_response_bytes` | Parsed participant or model response bytes. | Go |
| Runtime | `invalid_attempt_limit` | Invalid participant attempts before failure. | Go |
| Runtime | `council_max_output_tokens` | Default direct council output-token limit. | Go and request specification |

`judgment_standard` belongs to procedural policy because it governs how every council member answers the same question.  The complaint contains the question without embedding council size, filing limits, or runtime deadlines.  This separation allows one complaint to run under different declared procedures without changing its text.

## Enforcement Boundary

Lean enforces phase order, exact opportunity authority, filing text limits, exhibit counts and byte commitments, technical-report counts and UTF-8 byte limits, submitted-evidence counts and byte commitments, lineage, source-state offer chronology, council answers from 0 through 100, and closure after complete answering.  Go verifies file custody, enforces direct upload and read limits, supplies trusted authority and parent digests, and rejects transport errors before invoking the engine.  Both layers enforce the exhibit, submitted-evidence, and report commitments where their representations overlap.

The byte commitment in Lean concerns recorded metadata, while Go owns the stored file descriptor and hashes its complete contents.  Replay therefore proves facts about accepted identifiers, digests, sizes, lineage, references, and phase history.  Certificate verification does not rehash `evidence-store/`, so stored-byte custody remains a separate operational check.

## Persistence

`complaint.md` stores the canonical complaint, and the Lean state stores its parsed `question`.  `policy.json` stores every effective procedure and custody field, while `runtime.json` stores the six runtime fields.  `run.json` includes the complaint, judgment standard, backend, participant and evidence metadata, events, answer map, and final state.

The engine-visible policy appears inside `state.json` and the final state in `run.json`.  Custody-only policy fields do not enter the Lean state, and the evidence manifest does not duplicate policy or runtime settings.  Events record actions and process facts rather than a copy of the complete configuration.

## Closure Rule

The case closes when every council member still eligible to answer has answered once in the current round.  The result is the answer map keyed by `member_id`, with no threshold, aggregate answer, or substantive outcome label.  A failed council member leaves the remaining members subject to the same completion rule.

The current engine fixes the closure rule rather than exposing it as a policy field.  Aggregation or multiple rounds would change the legal state and proof obligations.  Such a change would require an engine-visible policy decision before implementation.

## Configuration and Defaults

The main configuration surface is one policy file plus the complaint and runtime overrides.  `--council-size` and `--judgment-standard` provide narrow procedure overrides, while timeout, response, attempt, engine, identifier, and output flags control execution.  The final packet records the resulting complaint, policy, and runtime values in their separate files.

The checked-in defaults seat five council members, use the judgment standard in [`etc/policy.json`](../etc/policy.json), allow 900 seconds for a lawyer turn, and allow 240 seconds for a council opportunity under either backend.  One Lean engine call has a 30-second default, the parsed response limit is 128 KiB, the invalid-attempt limit is three, and the direct council output default is 4096 tokens.  The current engine closes after one complete set of eligible council answers.
