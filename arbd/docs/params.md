# Parameter Surface

`arbd` separates three classes of input that serve different purposes: procedural policy, complaint content, and runtime limits.  Procedural policy determines what the arbitration procedure allows and how council members are instructed to answer.  Complaint content states the question for one case.  Runtime limits constrain the infrastructure that runs the case, not the legal procedure itself.

The current implementation already keeps those classes separate in the code and in the run record.  [The `aard case` command](../runtime/cmd/aard/case.go) loads the complaint and policy independently, then builds [the proceeding options](../runtime/proceeding/options.go).  [The proceeding policy layer](../runtime/proceeding/policy.go) validates procedural policy, evidence custody limits, and runtime limits before case initialization, and [the Lean engine](../engine/Main.lean) carries the procedural policy into the legal state.

## Parameter Groups

| Group | Parameter | Purpose | Home | Primary enforcement |
|---|---|---|---|---|
| Procedure | `council_size` | Number of council members seated for the case | policy | Go at startup, Lean in state |
| Procedure | `judgment_standard` | The standard the council applies to the question | policy | Go at startup, Lean in state, prompts |
| Procedure | `max_opening_chars` | Opening text limit | policy | Lean |
| Procedure | `max_argument_chars` | Argument text limit | policy | Lean |
| Procedure | `max_rebuttal_chars` | Rebuttal text limit | policy | Lean |
| Procedure | `max_surrebuttal_chars` | Surrebuttal text limit | policy | Lean |
| Procedure | `max_closing_chars` | Closing text limit | policy | Lean |
| Procedure | `max_exhibits_per_filing` | Maximum offered evidence in one filing | policy | Lean and Go |
| Procedure | `max_exhibits_per_side` | Maximum offered evidence by one side across the whole case | policy | Lean |
| Procedure | `max_exhibit_bytes` | Maximum bytes for one offered evidence item. The default matches the submitted-evidence upload limit. | policy | Go before submission |
| Procedure | `max_submitted_evidence_per_side` | Maximum submitted evidence admitted by one side across the whole case | policy | Lean and Go |
| Procedure | `max_submitted_evidence_bytes` | Maximum bytes for one admitted submitted evidence item | policy | Lean and Go |
| Procedure | `max_direct_submitted_evidence_bytes` | Maximum bytes for direct JSON/base64 evidence submission | policy | Go |
| Procedure | `max_evidence_upload_bytes` | Maximum bytes for chunked evidence upload | policy | Go |
| Runtime | `max_evidence_chunk_bytes` | Maximum bytes in one upload chunk | policy | Go |
| Runtime | `max_evidence_read_bytes` | Maximum bytes returned by one evidence range read | policy | Go |
| Runtime | `max_evidence_reads_per_opportunity` | Maximum evidence reads in one attorney opportunity | policy | Go |
| Runtime | `max_evidence_read_bytes_per_opportunity` | Maximum total evidence bytes returned in one attorney opportunity | policy | Go |
| Procedure | `max_reports_per_filing` | Maximum technical reports in one filing | policy | Lean and Go |
| Procedure | `max_reports_per_side` | Maximum technical reports by one side across the whole case | policy | Lean |
| Procedure | `max_report_title_bytes` | Maximum title size for one report | policy | Lean and Go |
| Procedure | `max_report_summary_bytes` | Maximum summary size for one report | policy | Lean and Go |
| Complaint | `question` | The disputed quantitative question | complaint | complaint parser |
| Runtime | `council_llm_timeout_seconds` | Timeout for council turns | proceeding config | Go |
| Runtime | `lawyer_turn_timeout_seconds` | Timeout for lawyer turns | proceeding config | Go |
| Runtime | `max_response_bytes` | Maximum raw model response size accepted from a turn | proceeding config | Go |
| Runtime | `invalid_attempt_limit` | Maximum invalid attempts before a turn fails | proceeding config | Go |

`judgment_standard` belongs in policy, not in the complaint.  It is a case parameter no different in kind from `council_size` or a filing limit.  The complaint should state only the disputed question, and the policy or case configuration should supply the standard the council applies to that question.

## Closure Rule

The case closes when every seated council member has answered once in the current round, and the result is the answer map itself.  The current implementation therefore needs only complete-answering rules and the answer map in state.  It does not need a vote threshold, a substantive outcome label, or an aggregate-answer field.

That closure rule is part of the procedural surface, even though it is not currently configurable.  If a later version adds aggregation or multi-round convergence, that change belongs in policy and in the Lean engine rather than in complaint parsing or runtime transport.  The present code keeps the simpler rule visible by leaving those omitted fields out of both [the policy type](../runtime/proceeding/policy.go) and [the case state](../engine/Main.lean).

## Enforcement Split

Lean should continue to enforce procedural rules that affect the legal state: phase ordering, text limits, counts of exhibits, submitted evidence, technical reports, bounded council answers, and closure on complete answering.  Go should enforce byte-based custody limits and transport limits before material reaches the engine.  A byte limit is about what the runtime will carry and persist.  A phase rule is about what the procedure allows.  They are different constraints and should stay in different layers.

This split also determines persistence.  Policy values that affect the legal case are written into the arbitration state and therefore into record artifacts such as `run.json`, `state.json`, `evidence-manifest.json`, and the event log.  Runtime limits stay in proceeding config and appear in `runtime.json` rather than in the legal state.

## Configuration Surface

The main configuration surface should remain one policy file, not a long list of unrelated CLI flags.  A single `--policy FILE` argument is enough for procedural policy.  The existing CLI can keep a small number of operational flags such as timeout values and output paths, plus narrow policy overrides such as `--council-size` and `--judgment-standard` when they are useful for testing.

That separation lets the same complaint run under different procedural policies without rewriting the complaint, and it lets the same policy run under different timeout settings without changing the legal state.  The final run packet can then show exactly which question, which policy, and which runtime limits produced the recorded council answers.  That record boundary is part of the procedure's audit trail.

## Defaults

The initial defaults should preserve the current implementation's working behavior.  That means a five-member council, the checked-in judgment standard from [`etc/policy.json`](../etc/policy.json), and the current filing-size and material-limit fields.  It also means one deliberation round in practice, because the engine closes after the first complete set of seated-member answers.
