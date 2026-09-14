# AARD Implementation Reference

## Procedure and Components

AARD decides one quantitative question.  Plaintiff and defendant lawyers create an adversarial record through openings, arguments, rebuttal, surrebuttal, and closings.  Each eligible council member then submits one integer answer from 0 through 100 and a nonblank rationale.  The result preserves the complete map from `member_id` to answer.

The implementation has four principal parts.  `engine/AARD/Core.lean` defines the state, policy, opportunities, transitions, initialization, and replay functions.  `engine/Main.lean` implements the JSON protocol and executable entry point.  `runtime/proceeding` owns one case, participant APIs, council execution, evidence custody, records, and certificate creation.  `runtime/lean` invokes the Lean executable with finite timeouts and translates its JSON responses.

The AARD command and runtime have no dependency on `adjservices`.  The unified `adjudicate` command in this repository can start the AARD core, MCP adapter, automatic lawyers, and council processes for one complete case.

## State and Authority

The Lean state uses schema `v1`.  It contains the case, policy, immutable initial evidence catalog, and monotonically increasing `state_version`.  The case stores its identity, question, status, phase, council roster, filings, evidence references, reports, submitted evidence, deliberation round, council answers, and optional failure.

Every state-changing action carries `OpportunityAuthority`: `opportunity_id`, `expected_state_version`, `role`, `phase`, and `member_id`.  Council opportunities require a member identifier; lawyer opportunities require an empty member identifier.  Lean reconstructs the expected authority from the source state and rejects a stale version, wrong opportunity, wrong role, wrong phase, wrong council member, or operation absent from the current opportunity.

The Go runtime derives authority from the opportunity returned by Lean.  After an accepted step, Go requires a nonempty case state and an exact one-step increase in `state_version`.  It records the same authority and payload in the replay certificate.

## Procedure Invariants

`Proofs/ProcedureInvariants.lean` defines the invariant package used for initialized runs.  `phaseShape` specifies the exact filing sequence admitted in each phase.  `councilIdsUnique` requires unique council identifiers.  `answerIntegrity` requires unique current-round answer owners, an answer from 0 through 100, a nonempty stored rationale, the current round number, and membership in the seated council.  `caseFrameMatches` fixes the case identifier, caption, question, policy, evidence catalog, and council identities from initialization.

`Proofs/StepPreservation.lean` proves that successful initialization establishes these properties and that every accepted public step preserves them.  The preservation proof covers all merits filings and passes, evidence submission, council answers, council removal, council failure, and party failure.  `initializedRun_reachable_invariant` lifts the result to every state reachable from a successful initialization.

`Proofs/Progress.lean` proves that every active merits state satisfying the phase invariant has a current opportunity.  It proves the corresponding result for deliberation while an eligible member remains unanswered.  It also defines a remaining-step measure and bounds its merits, evidence-submission, and deliberation components from the policy and council size.

`Proofs/OutcomeSoundness.lean` packages the properties of an initialized closed case: completed merits, unique council identifiers, valid answers, record integrity, and the initialized case frame.  It proves that deliberation closes only after every eligible council member has answered.  Its failure theorem classifies an accepted `fail_opportunity` transition as either the recorded case-level party failure or the delegated failure of the scheduled unanswered council member.

## Record Integrity

Initialization validates the initial evidence catalog.  Each entry has a trimmed nonempty identifier, canonical lowercase SHA-256, and byte size, and identifiers are unique.  Successful public steps preserve the catalog exactly.

Submitted evidence stores normalized metadata, a canonical digest, a positive bounded size, and optional lineage.  Lineage contains all of `parent_evidence_id`, `parent_sha256`, and `derivation_method`, or none of them.  A parent must occur in the initial catalog or earlier submitted-evidence prefix with the recorded digest.  A submitted identifier cannot collide with the initial catalog or an earlier submission.

Arguments, rebuttals, and surrebuttals may submit evidence, offer visible evidence, and include technical reports.  Lean resolves offered identifiers against the filing action's source state, enforcing the chronological rule that later evidence cannot validate an earlier offer.  It also enforces filing limits, exhibit counts and sizes, report counts, and UTF-8 title and summary limits.

Go captures initial files before initialization, derives Lean commitments from verified descriptors, and publishes the bytes under `evidence-store/`.  Evidence reads verify the complete stored descriptor against its recorded digest and size before returning a bounded range.  Direct and chunked submissions use the same validation and admission path.

## Council Answers and Failure

The runtime normalizes a direct or Council API answer to a whole number from 0 through 100 and trims its rationale.  Lean independently requires the scheduled seated member, a fresh current-round answer, the numeric bound, and a nonempty rationale.  The engine selects the first seated member without a current-round answer and closes after every eligible member has answered.

A plaintiff or defendant opportunity failure records its role, phase, opportunity, reason, and message, then sets case status to `failed`.  A council opportunity failure changes the scheduled unanswered member from `seated` to `failed`, preserves existing answers, and continues with the remaining eligible members.  The ordinary completion rule closes the case when no eligible member remains unanswered.

## Replay Certificates

Terminal cases write `certificate.json` using schema `aard.replay-certificate.v1`.  The certificate contains the initialization request, ordered accepted actions with their source-state authority, and the claimed final state.  The Go verifier checks procedure and case identity, terminal status, action authority shape and source version, packet-state equality, and exact replay through the configured Lean engine.

Lean's `checkReplayCertificate` accepts exactly when initialization followed by the recorded action list produces the claimed state.  The certificate fact packages derive replay agreement, authority conformance, source-state offer chronology, reachability, record integrity, fixed initialization data, and the initialized-run invariant.  Closed facts also preserve the member-answer pairs.  Failed facts preserve the recorded opportunity failure.

The Go verifier checks recorded transitions and JSON state equality.  It does not rehash `evidence-store/`, validate a claimed derivation against source bytes, or return Lean theorem values through the process protocol.

## Prompt and Participant Interfaces

AARD owns its prompt catalog in `runtime/proceeding`, with editable files under `prompts/arbd`.  `--prompt-file ID=PATH` replaces individual entries, and `--prompt-dir DIR` supplies a complete catalog.  Resolution then checks the working-directory convention and compiled fallback.  Token insertion uses validated literal replacement.

The case process exposes the Lawyer API and Observer API.  The `councilapi` backend adds the Council API, while the default `direct` backend calls each selected council request specification.  External lawyers can use any harness through `aard-mcp`; `aard-run` and the unified `adjudicate` command can start OpenClaw, Pi, Codex, or Claude lawyers.

## Durable Formats

| Record | Schema or rule |
|---|---|
| Lean state | Additive schema `v1`. |
| Replay certificate | `aard.replay-certificate.v1`. |
| Evidence manifest | `aar.evidence-manifest.v0`. |
| Case packet | `aard.case-packet.v0`. |
| Council turn snapshot | `aard.council-turn-snapshot.v0`. |

The [manual](../manual.md) defines commands, flags, APIs, records, and failure reporting.  The [process specification](aard-spec.md) defines the external process and HTTP behavior.  The [verification guide](verification.md) defines the proved properties and the boundary between Lean results and operational certificate verification.
