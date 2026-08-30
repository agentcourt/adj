# AARD Process and HTTP Specification

## Scope

This specification defines the external behavior of one `aard case` process and its certificate verifier.  It covers startup, standard streams, private Lawyer and Council APIs, evidence custody, durable records, and participant failures.  The `adjservices` repository owns managed case admission and public routing.

## Process Model

`aard case` runs one degree arbitration from a complaint question to a terminal council answer map or recorded case failure.  Its flags select the complaint, identifiers, new or empty output directory, private API address, policy, initial evidence, council pool, prompts, runtime limits, engine, and council backend.  The process owns case state, active opportunities, evidence registries, upload sessions, council membership, replay actions, and output until termination, with a default 30-second limit on each engine invocation.

Startup claims the output directory before writing any case record.  The command rejects a nonempty directory, creates `.aard-output-claim` exclusively, writes `case-manifest.json`, and removes the claim after that first publication.  An abrupt stop before claim removal can leave a claim that requires manual inspection and cleanup.

The command captures the initial evidence catalog, samples and validates the council, initializes Lean, and then starts the private API.  It writes `caseapi listening on BASE_URL` to standard error after the listener and initialized case are ready.  It writes one JSON summary line to standard output when the case ends or when a process error is reported.

A normal procedural result uses `status: "ok"` and includes the member answer map.  A recorded plaintiff or defendant opportunity failure uses `status: "failed"` and may still exit zero after writing a terminal packet.  Startup, configuration, engine, storage, publication, and other process failures return a nonzero exit status.

## State, Question, and Authority

The complaint supplies one quantitative `question`, while policy supplies `judgment_standard`.  Lean stores both in state and preserves independent `CouncilAnswer.answer` values as natural numbers accepted only from 0 through 100.  The case closes when every still-seated council member has answered in the current round, and the result remains the map from `member_id` to answer.

The Lean state schema remains `v1` with additive defaults for the immutable `evidence_catalog` and submitted-evidence lineage.  Every state-changing action carries authority containing `opportunity_id`, `expected_state_version`, `role`, `phase`, and `member_id`.  Go derives that authority from the current state and opportunity, while Lean requires exact equality and checks that the opportunity allows the requested action type.

Participants supply `case_id`, role or member identity, `opportunity_id`, tool, and arguments through the role APIs.  They do not supply the engine authority object.  The runtime records the derived authority with each accepted replay action.

## Common HTTP Rules

The private API listens only for the running case.  Every request identifies the configured `case_id`, and every participant mutation identifies the current opportunity.  Case mismatch, identity mismatch, inactive turn, wrong actor, and stale opportunity are rejected before tool execution.

POST bodies have finite byte limits and must contain exactly one JSON value.  Tool argument objects and filing payloads reject unknown keys, wrong types, malformed numeric values, and inconsistent evidence fields.  The APIs accept the optional `call_id` where supported, and work-note records preserve it, but it does not deduplicate a retry or identify a cached result.

Role API responses are JSON objects, and successful operations report `ok`.  Participant-controlled argument and payload errors use `tool_failed` and may consume an invalid-attempt allowance, while storage, engine, event, and other infrastructure errors use `runtime_failure`.  Routing and turn-selection errors retain specific codes such as `unknown_case`, `not_current_turn`, and `stale_opportunity` and do not consume a legal attempt.

## Lawyer API

The Lawyer API lives under `/lawyerapi/v1`.  Lawyer roles are `plaintiff` and `defendant`, while `observer` provides read-only access.  Every lawyer request includes the case id and applicable role id.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/lawyerapi/v1/get` | Return current role status and a ready turn when available. |
| `GET` | `/lawyerapi/v1/wait` | Wait for a role state change or bounded timeout. |
| `GET` | `/lawyerapi/v1/status` | Return compact case and turn status. |
| `GET` | `/lawyerapi/v1/result` | Return the terminal result or pending status. |
| `POST` | `/lawyerapi/v1/do` | Execute one support, evidence, work-note, or legal operation. |

A ready turn includes opportunity id, role, phase, prompt, tools, limits, deadline information, and remaining attempts.  A successful filing or pass advances the case and completes that opportunity, while accepted evidence submission updates the source state and leaves the lawyer able to file during the same turn.  Attempt exhaustion records a terminal lawyer opportunity failure through Lean.

Lawyer operations include case reads, private work notes, evidence listing, metadata, bounded evidence reads, direct submission, chunked upload, and legal-decision submission.  Evidence reads remain available throughout openings, arguments, rebuttals, surrebuttals, and closings.  Evidence submission remains limited to arguments, rebuttals, and surrebuttals.

## Observer API

Observer calls use the Lawyer API with `role_id: "observer"`.  Observer tools include case status, case view, current turn, bounded event listing, evidence listing, evidence metadata, and bounded evidence reads.  Observers cannot submit filings, evidence, work notes, answers, or failures.

Observer reads use verified file descriptors but do not consume participant read budgets or write participant evidence-read events.  A resolved Observer tool-argument error returns `tool_failed`.  Malformed JSON, request-envelope errors, and routing errors retain their specific request codes, while a storage fault returns `runtime_failure` to that request.  An Observer storage fault does not terminate the active case.

## Council API

The Council API is active when `--council-backend councilapi` selects external council members.  Requests under `/councilapi/v1` identify the case and bound member, and each seated member receives one current opportunity during deliberation.  Direct and external council modes use the same Lean answer and failure transitions.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/councilapi/v1/get` | Return member status and a ready deliberation turn. |
| `GET` | `/councilapi/v1/wait` | Wait for a member state change or bounded timeout. |
| `POST` | `/councilapi/v1/do` | Read evidence or submit one council answer. |
| `POST` | `/councilapi/v1/fail` | Record failure for the active member opportunity. |

Council operations include case reads, evidence listing, metadata, bounded descriptor-verified reads, and `submit_council_answer`.  The advertised tool schema requires an integer answer from 0 through 100 and a nonblank rationale.  The Go handler accepts a whole JSON number or numeric string, normalizes it to an integer in that range, and supplies trusted member identity.  A successful answer completes that member's participation and closes the case when all still-seated members have answered.

A council-member failure marks the scheduled unanswered member failed and preserves prior answers.  Remaining members continue when another answer is required, and the ordinary completion rule can close the case after removal.  The failure does not convert the whole case to status `failed`.

## Evidence Record and Custody

The runtime captures initial case files before council sampling and Lean initialization.  Each captured item receives an `ev_<sha-prefix>_<slug>` identifier, full SHA-256, size, and immutable store location, and these commitments form the initial Lean catalog.  The catalog remains fixed through every accepted state transition.

Submitted evidence carries source metadata, digest, size, and optional lineage.  A lineage names an initial or earlier submitted parent by identifier and digest and records a nonblank derivation method.  Go derives the parent digest from verified record evidence, while Lean rejects incomplete lineage, self-parenting, unknown parents, identifier collisions, malformed digests, and oversized submissions.

Arguments, rebuttals, and surrebuttals may include offered evidence and technical reports.  Each offered identifier must resolve in the catalog or submitted-evidence list in the filing's source state, which prevents a later submission from validating an earlier offer.  Openings and closings reject supplemental evidence and report arrays.

Lawyer and council file reads reserve budgets under the case mutex, release the mutex for input, open one regular-file descriptor, verify complete size and SHA-256 while collecting the requested range, and revalidate the active turn after reacquiring the mutex.  Successful participant reads record `evidence_read`.  Failed or stale reads restore the reservation when applicable.  Path replacement, nonregular files, size changes, and digest changes are runtime failures.

Direct and chunked submissions share one commit path.  The path obtains an accepted Lean transition, publishes the immutable store object and submitted copy, writes a candidate evidence manifest, rechecks the deadline, and then commits state, registries, and replay action together under the case mutex.  No journal repairs a machine crash between related file publications, and a failed late publication can leave an unreferenced file even though in-memory state remains unchanged.

## Durable Records and Schemas

Every completed or procedurally failed case writes a terminal packet, while an earlier process error can leave only the files published before that error.  `run.json` contains identifiers, times, status, error and failure data, phase, complaint, judgment standard, backend, answer map, attorneys, case files, submitted evidence, evidence, council, events, final state, and final reason.  It has no schema or generated-artifacts field.  Stage-specific directories and records appear only when the corresponding evidence, work-note, or council-turn operation occurs.

| Record | Schema or role |
|---|---|
| `complaint.md` | Canonical complaint. |
| `case-manifest.json` | Case and run identity, start time, core version, and bound API address. |
| `policy.json` | Effective procedure and evidence-custody policy. |
| `runtime.json` | Effective deadlines, engine timeout, response limit, attempt limit, and council output limit. |
| `run.json` | Final structured result. |
| `state.json` | Final additive Lean state schema `v1`. |
| `certificate.json` | Replay schema `aard.replay-certificate.v1`. |
| `council.json` | Final roster, member status, request specifications, and failures. |
| `council-turns/*/input.json` | Council snapshot schema `aard.council-turn-snapshot.v0`, created for each council turn. |
| `council-turns/*/prompt.txt` | Rendered prompt created for the corresponding council turn. |
| `evidence-manifest.json` | Shared evidence schema `aar.evidence-manifest.v0`. |
| `evidence-store/` | Immutable content-addressed evidence bytes, present when at least one item is stored. |
| `submitted-evidence/` | Copies created for accepted lawyer submissions. |
| `events.ndjson` | Ordered process events. |
| `work-notes.ndjson` | Off-record lawyer notes, created on the first submitted note. |
| `transcript.md`, `digest.md` | Human-readable proceeding and answer views. |

`aard case-packet` writes a deterministic archive and manifest under schema `aard.case-packet.v0`.  It verifies each source descriptor while constructing the archive, builds both outputs in temporary files, reserves both final names exclusively, and renames both with cleanup on an ordinary error.  It has no crash journal for a stop between the two final renames.

## Replay Certificate Verification

`aard verify-certificate` accepts only schema `aard.replay-certificate.v1` and procedure `aard`.  It requires a nonblank certificate case id and requires the same id in the initialization state, claimed state, packet `state.json`, and replayed state.  It also requires replay to finish with case status `closed` or `failed`.

The verifier hashes the claimed final state, compares `state.json` with that hash, replays initialization and every action through bounded Lean engine calls, and hashes the replayed state.  It validates each authority shape and source version before the engine step and checks council member payloads against council authority.  Successful verification requires the claimed, packet, and replay hashes to agree.

The Go verifier replays and hashes but does not invoke Lean's theorem `checkReplayCertificate` as an exported proof checker.  In Lean, successful `checkReplayCertificate` implies exact replay, exact authority, source-state offer chronology, reachability, record integrity, and fixed catalog equality.  The closed and failed certificate facts additionally require the corresponding claimed-state status premise.  The [verification guide](verification.md) describes this formal and operational boundary.

Certificate verification does not inspect `evidence-store/`, work notes, events, council snapshots, transcript, or digest.  It therefore verifies recorded engine transitions and JSON state equality rather than packet-wide evidence-byte custody.  A separate custody verifier would be required to rehash stored evidence files.

## Concurrency and Engine Lifecycle

One `runContext.mu` owns mutable case state, both role API turn records, evidence and upload registries, events, and certificate actions.  Provider calls, HTTP writes, and descriptor reads run without that mutex, while state-changing Lean calls hold it.  Final output clones an owned snapshot under the mutex and renders from the clone after releasing the lock.

Every Lean invocation has a finite engine timeout, and a participant step also observes the participant-turn deadline.  The adapter starts the engine in a new process group, kills that group on cancellation or timeout, and checks for remaining group members after command exit.  Only a valid rejection object paired with exit status 1 constitutes semantic rejection.  Other nonzero exits, malformed output, and surviving descendants are process failures.

## Test Obligations

Process tests use the real command and private APIs where the behavior crosses package boundaries.  They cover startup claims, opportunity identity, request strictness, error classes, deadlines, attempt exhaustion, evidence custody, submission publication, participant failure, final snapshots, durable records, process cleanup, and certificate tampering.  The [implementation record](update.md#verification-results) gives the exact in-tree proof, runtime, formatting, and documentation checks.
