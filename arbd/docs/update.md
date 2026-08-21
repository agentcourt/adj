# AARD Record-Integrity and Runtime Update

## Scope and Status

This document records the AARD authority, record-integrity, runtime, and certificate work in the current source tree.  It explains the rules implemented by the Lean engine and Go runtime and preserves the procedure-specific choices needed for later ports.  The [Verification Results](#verification-results) section records the completed in-tree and paired service checks.

The implementation keeps AARD centered on one quantitative question and one configured judgment standard.  Each council member still submits an independent natural-number answer from 0 through 100, and the result remains the answer map keyed by `member_id`.  The work adds record and replay guarantees without introducing an aggregate answer, a binary outcome, or a renamed deciding body.

## Durable Formats

The update changes the replay certificate version while retaining the other established format names.  The Lean state keeps schema `v1` because the catalog and lineage fields have decoding defaults and extend the existing state representation.  Each format has its own compatibility boundary and should change only when its serialized meaning changes.

| Record | Schema | Current rule |
|---|---|---|
| Lean arbitration state | `v1` | Adds a defaulted immutable `evidence_catalog` and defaulted submitted-evidence lineage fields. |
| Replay certificate | `aard.replay-certificate.v1` | Requires exact opportunity authority on every recorded action and rejects the prior v0 certificate. |
| Evidence manifest | `aar.evidence-manifest.v0` | Uses the shared AAR/AARD evidence metadata format. |
| Case packet | `aard.case-packet.v0` | Retains the deterministic AARD archive and manifest format. |
| Council turn snapshot | `aard.council-turn-snapshot.v0` | Retains the complaint and state, policy and runtime, prompt, tool schema, limits, evidence, and request specification. |

Evidence identifiers retain the `ev_<sha-prefix>_<slug>` form generated from the stored SHA-256 digest and normalized source name.  The current prefix contains the first 12 hexadecimal digest characters, while the complete digest remains in the catalog and manifest.  A future identifier change would require explicit migration and replay rules because filings and certificates store these identifiers.

## Exact Opportunity Authority

Every state-changing `CourtAction` now carries `OpportunityAuthority`.  The authority contains `opportunity_id`, `expected_state_version`, `role`, `phase`, and `member_id`.  Council opportunities require the member field, while lawyer opportunities require it to be empty.  Go derives the value from the current Lean state and returned opportunity rather than accepting authority fields from a participant.

Lean reconstructs the expected authority from the source state and current opportunity, requires exact equality, and checks that the opportunity authorizes the requested action type.  A stale state version, wrong opportunity, wrong role, wrong phase, wrong council member, or disallowed operation rejects the action before its body runs.  Closed and failed cases reject every later public step before authorization.

The runtime stores the same authority with every accepted `ReplayAction`.  Certificate replay checks the recorded expected state version before each engine call and checks council member payloads against council authority.  The proof library carries exact authority through successful action lists and accepted certificates.

## Lean Record Model

Initialization supplies an immutable `evidence_catalog` whose entries contain `evidence_id`, canonical lowercase SHA-256, and `size_bytes`.  Lean rejects blank or untrimmed identifiers, duplicate identifiers, malformed digests, and invalid commitments before the case begins.  Successful public steps preserve the catalog exactly.

Submitted evidence stores normalized metadata, SHA-256, size, and optional lineage.  A lineage is either empty or contains `parent_evidence_id`, `parent_sha256`, and a nonempty `derivation_method`.  The parent must be an initial commitment or an earlier submitted item with the recorded digest.  The child identifier must differ from every catalog identifier and earlier submitted identifier, which makes the submission history prefix-valid.

Arguments, rebuttals, and surrebuttals may submit evidence, offer visible evidence, and include technical reports.  Openings and closings reject nonempty `offered_evidence` and `technical_reports` arrays, while surrebuttal now follows ARAP and the Go tool surface by permitting both classes of supplemental material.  Lean validates offered identifiers and committed sizes and measures report titles and summaries in UTF-8 bytes.

Final-state record validity alone cannot show when an offered item entered the record.  The source-state predicate checks each accepted argument, rebuttal, or surrebuttal against the catalog and submitted-evidence list present before that filing.  Replay chronology carries this predicate across the action list, so a later submission cannot validate an earlier offer.

The [proof overview](verification.md), [selected theorem catalog](theorems.md), and [complete proof statistics](proofstats.md) describe the formal results.  The principal results establish exact authority, initialization and step preservation, reachable record integrity, source-state offer chronology, replay, and closed and failed certificate fact packages.  Concrete certificate examples cover one complete three-answer case and one terminal lawyer-opportunity failure.

## Evidence Custody

The runtime captures every initial case file before initialization and builds the Lean catalog from the captured metadata.  It opens a regular source file, verifies that the descriptor still names the inspected path, hashes the complete descriptor contents, and publishes those bytes into `evidence-store/`.  Council sampling, provider preflight, and Lean initialization therefore operate only after the immutable initial catalog exists.

Lawyer and council reads reserve their count and byte budgets while holding `runContext.mu`, then release the mutex for file input.  The reader verifies one opened regular-file descriptor, hashes and counts the complete file while collecting the requested range, and rejects path replacement, size disagreement, or digest disagreement.  After input completes, the handler reacquires the mutex, revalidates the active opportunity and deadline, finalizes or restores the reservation, records the read event, and returns the bytes.

Observer reads use the same verified-descriptor path without participant read budgets.  An Observer storage fault returns `runtime_failure` to that request and does not terminate the case.  This request-scoped treatment remains an explicit limit rather than a general rule for participant storage faults.

Direct submission and chunked-upload commit enter one admission path.  The path validates identity, metadata, lineage, and candidate registry state.  It obtains an accepted Lean `submit_evidence` transition, publishes the immutable store object and submitted-evidence copy, writes the candidate manifest, and rechecks the deadline.  It then commits state, registries, and the replay action under the mutex.  A failure before that commit leaves the in-memory record unchanged, while a later event-write failure or machine stop can leave committed state or unreferenced files that require inspection.

The caller supplies `parent_evidence_id` and `derivation_method`, while Go derives `parent_sha256` from verified record evidence.  Go verifies that parent bytes still match the recorded digest before submission, and Lean checks the parent against the immutable catalog or prior submission prefix.  Neither layer proves that the child bytes implement the stated derivation method.

## Request and Error Classes

Role API request bodies have finite byte limits and must contain exactly one JSON value.  Tool argument and filing payload validators reject unknown keys, wrong value types, and inconsistent evidence fields before a state transition.  Identity, case, active-turn, and opportunity checks return their specific request codes without treating the request as a legal action.

Participant-controlled tool or payload errors carry the `tool_failed` class and can consume an invalid-attempt allowance.  Storage, engine, event, and other infrastructure errors carry `runtime_failure`.  An active participant turn ends when such an error makes continued execution unsafe.  Deadline expiration and attempt exhaustion record `fail_opportunity` through Lean when that transition succeeds.

`call_id` remains metadata on role requests, and work-note records preserve it.  The runtime does not keep a completed-call table or return a prior result for a repeated `call_id`.  A client retry can therefore repeat a non-idempotent operation and must use current opportunity and state responses to determine whether the first request committed.

## State Ownership and Engine Calls

One `runContext.mu` owns mutable case state, evidence and file registries, upload sessions, events, turn number, certificate actions, and both role API turn records.  Methods ending in `Locked` require that mutex, and condition variables for Lawyer and Council state changes use it.  A separate small mutex protects only accumulated HTTP response-write errors and has no authority over case state.

Provider calls and HTTP response writes run without the case mutex.  Evidence reads also release the mutex after reserving a source snapshot and budget, then revalidate the turn after input completes.  State-changing Lean calls remain under `runContext.mu`, but each call has the shorter of its engine timeout and participant-turn deadline.

The [Lake configuration](../engine/lakefile.toml) passes `-j1` to each Lean compiler process through `weakLeanArgs`, limiting that process to one compiler worker unless a stronger per-process setting applies.  This argument does not limit Lake's build scheduler to one module job.  Runtime engine serialization, compiler worker limits, and build-job concurrency govern different execution paths.  The verification record identifies the bounded runner and exact serialized Lean commands used for the completed checks.

The Lean adapter starts each engine invocation in a new operating-system process group.  Context cancellation or timeout kills that process group, and post-exit cleanup checks for descendants that remained in the group.  The adapter treats a rejection as protocol data only when stdout contains a valid `{ "ok": false, "error": ... }` object, the command exits with status 1, and the process group is gone.  Every other nonzero exit is a process failure.

## Output and Publication

`aard case` requires a new or empty output directory.  Startup creates `.aard-output-claim` with exclusive creation, confirms that it is the only directory entry, writes `case-manifest.json`, and then removes the claim.  Another starter therefore cannot claim the same directory during startup, and a nonempty completed or partial directory cannot be reused.

The direct AARD service leaves that output directory to the core and stores new child standard streams beneath `RegistryDir/logs/<case_id>/`.  Its artifact API retains the logical `service-logs/aard.stdout` and `service-logs/aard.stderr` names and accepts only exact registry paths recorded for the case.  The service reserves case identifiers against active and persisted direct and Clerk records, creates confined log directories and files exclusively, publishes its record through a unique temporary file, and verifies directory identity before failed-create cleanup.  Every direct terminal consumer uses one reader that requires matching case and run identifiers and consistent top-level and final Lean case statuses.  The `aard-run` launcher owns an outer service run directory, passes its dedicated `aard-output` child to the core, and keeps component logs and `local-run.json` outside that child.  It requires empty core and log children, rejects symbolic-link substitutions, and rejects a child whose directory identity changes during validation.  Each container-backed participant writes its complete runtime container identifier to a private cidfile, and cleanup removes only that verified identifier.  Local and attested Clerk readers resolve core artifacts through the nested core path; an outer-root `run.json` has no core-result role.  The reporter descends through the outer service records and reports their failure or incomplete state when no nested core marker exists.  For a manual role, the launcher writes a bearer-token remote-lawyer skill in the outer directory and removes it during cleanup.  Attested event reads prefer the nested core copy from a verified success archive and otherwise use the live or downloaded top-level event object; a partial event copy remains diagnostic.  The attested archive retains the launcher summary, component logs, and core record and excludes Pi homes, staged Codex homes, and bearer-token remote-lawyer skills.  The Python driver publishes an extracted service root only after safe extraction into a sibling temporary directory, cleans remote temporary directories after failed transfers, and preserves cleanup errors.  The exec entrypoint stops before upload or manifest creation when archive creation, hashing, or sizing fails.  A trusted attested result comes only from `aard-run/aard-output/run.json`, matches the Clerk case and run identifiers, and pairs `ok` with `closed` or `failed` with `failed`.  A partial attested archive cannot complete a case.

An abrupt stop before claim removal can leave `.aard-output-claim`.  The runtime has no age, owner, or liveness rule for reclaiming that file, so cleanup remains a manual operator decision after confirming that no case process owns the directory.  The claim prevents concurrent startup but does not provide crash recovery for later packet writes.

Final output takes owned copies of nested state, events, evidence metadata, file indexes, work-product paths, initialization data, and replay actions while holding the mutex.  Rendering occurs from that snapshot after the lock is released.  `run.json` contains identifiers, times, status, error and failure data, phase, complaint, judgment standard, backend, answer map, attorney and case-file data, submissions, evidence, council, events, final state, and final reason.  It has no schema or generated-artifacts field.

`aard case-packet` retains schema `aard.case-packet.v0` and verifies each source through one open regular-file descriptor while writing the deterministic archive.  It builds the archive and manifest in temporary files, reserves both final paths with exclusive creation, and publishes both by rename with rollback on an ordinary error.  A machine crash between the two renames can still leave one output without the other because the pair has no journal or directory transaction.

## Replay Certificates

Terminal runs write `certificate.json` with schema `aard.replay-certificate.v1`.  The file contains the initialization request, ordered accepted public actions with authority, claimed final state, and canonical JSON SHA-256 of that state.  The schema change distinguishes authority-bearing actions from the earlier certificate form.

The Go verifier accepts only procedure `aard` and the v1 schema.  It requires one nonempty certificate `case_id`, requires that identifier in the initialization state, claimed state, packet `state.json`, and replayed state, and rejects a replayed status other than `closed` or `failed`.  It hashes the claimed state and `state.json`, replays initialization and every action through bounded engine calls, hashes the replayed state, and requires all three hashes to agree.

Certificate replay validates each recorded authority shape, checks its expected state version against the replay source, and checks council payload membership where the action uses a council member.  The Lean engine then performs the complete exact-opportunity and operation check during each step.  Verification does not read work notes, event logs, council snapshots, or evidence bytes.

## Formal and Operational Boundary

Lean defines the executable predicate `checkReplayCertificate` and proves that its success implies exact replay, authority-conforming actions, source-state offer chronology, reachability, record integrity, and fixed catalog equality.  The closed and failed certificate fact theorems also require the corresponding claimed-state status premise.  The Go `aard verify-certificate` command implements initialization, action replay, case binding, terminal checks, and JSON hashing through the engine protocol.  It does not invoke the Lean theorem as an exported proof checker or return the theorem's derived facts as protocol data.

The operational command and formal predicate therefore have related but distinct acceptance paths.  Documentation must state both paths and must not describe Go's success response as a proof object.  A later theorem-backed verifier would need an explicit engine API that checks or reconstructs every premise required by the formal certificate facts.

## Failure Semantics

A plaintiff or defendant opportunity failure records `failure_type: "opportunity_failed"`, the role, phase, opportunity id, reason, message, and optional model information, then changes case status to `failed`.  The next-opportunity API reports a terminal case, final output records the failure, and the command may finish normally with a failed procedural result.  Later public steps are rejected because failed state is terminal.

A council opportunity failure has different semantics.  Lean marks the scheduled unanswered member `failed`, preserves existing answers, and applies the ordinary complete-answer rule to the remaining seated council.  Remaining members continue when another answer is required, while the case closes if every still-seated member has already answered.

## Verification Results

Lean verification ran through `./tmp/leanrun-aar` with Lean 4.32.0, one runner job, one compiler worker per process, a 900-second timeout, 100 percent CPU, `MemoryHigh=4G`, `MemoryMax=6G`, 1 GiB swap, and `TasksMax=64`.  The narrow `RecordIntegrity.lean` check, complete proof-root build, and executable build all exited zero without diagnostics.  The built and installed engine executables are byte-identical and have SHA-256 `f4f38c5b123d745799c44b172ff59f4a2fcdedf41d266d8cc335fec38436db0f`.

| Lean check | Exact command | Result |
|---|---|---|
| Record-integrity unit | `./tmp/leanrun-aar lake --dir arbd/engine env lean -Dweak.warning.unusedSimpArgs=false arbd/engine/Proofs/RecordIntegrity.lean` | Exit 0 in 6.73 seconds. |
| Complete proof root | `./tmp/leanrun-aar lake --dir arbd/engine build Proofs` | Exit 0 in 8.56 seconds, 14 jobs. |
| Executable | `./tmp/leanrun-aar lake --dir arbd/engine build aardengine` | Exit 0 in 4.76 seconds, four jobs. |

Go verification used `GOMAXPROCS=2`, `LEAN_NUM_THREADS=1`, and repository-local `TMPDIR` and `GOCACHE` values.  The complete runtime test, proceeding race test, runtime vet, focused process and record tests, and twenty repeated concurrency, deadline, publication, and certificate checks passed.  The tests used fake engines, local HTTP handlers, or the rebuilt engine and made no provider call.  Go repeated the existing equal-GOPATH-and-GOROOT warning, and the command build reported that it could not update a read-only shared module stat cache while still exiting zero.

| Go check | Exact command | Result |
|---|---|---|
| Complete runtime | `env TMPDIR=$PWD/tmp/go-tmp GOCACHE=$PWD/tmp/go-cache GOMAXPROCS=2 LEAN_NUM_THREADS=1 go test -count=1 ./arbd/runtime/...` | Exit 0 for command, adapter, proceeding, and specification packages. |
| Proceeding race | `env TMPDIR=$PWD/tmp/go-tmp GOCACHE=$PWD/tmp/go-cache GOMAXPROCS=2 LEAN_NUM_THREADS=1 go test -race -count=1 ./arbd/runtime/proceeding` | Exit 0 in 2.30 seconds. |
| Runtime vet | `env TMPDIR=$PWD/tmp/go-tmp GOCACHE=$PWD/tmp/go-cache GOMAXPROCS=2 LEAN_NUM_THREADS=1 go vet ./arbd/runtime/...` | Exit 0 without diagnostics. |
| Command build | `env TMPDIR=$PWD/tmp/go-tmp GOCACHE=$PWD/tmp/go-cache GOMAXPROCS=2 LEAN_NUM_THREADS=1 CGO_ENABLED=0 go build -o arbd/.bin/aard ./arbd/runtime/cmd/aard` | Exit 0.  The binary SHA-256 is `0d8fb93369987876b838af32b4e46341b92d5065073da3d0aada4ad387d67438`. |

The repeated boundary command names every selected test explicitly.  The set covers shared notifications, concurrent commits, lock release, deadline rollback, evidence publication, error ownership, packet publication, and certificate boundaries.  All twenty runs passed in 0.78 seconds.

```sh
env TMPDIR=$PWD/tmp/go-tmp GOCACHE=$PWD/tmp/go-cache GOMAXPROCS=2 LEAN_NUM_THREADS=1 go test -count=20 -run '^(TestRoleAPIWaitersShareCaseNotification|TestRoleAPIsPublishTerminalStateTogether|TestConcurrentDuplicateLawyerDecisionCommitsOnce|TestConcurrentSameOffsetEvidenceChunksWriteOnce|TestRoleAPIHTTPWritesDoNotHoldCaseMutex|TestDirectCouncilProviderCallDoesNotHoldCaseMutex|TestEvidenceFileIODoesNotHoldCaseMutex|TestBlockedEvidenceReadsDoNotBlockDeadlinesAndRollback|TestInitialEvidenceCatalogUsesStoredSnapshots|TestSubmittedEvidenceLineageDerivesParentCommitment|TestDirectEvidencePublicationCanRetryAfterManifestFailure|TestUploadPublicationCanRetryAfterManifestFailure|TestSubmittedCopyPublicationFailureDoesNotCommit|TestAtomicPublicationRejectsChangedSourceCommitment|TestSubmittedEvidenceLineagePersistsInStateManifestEventAndCertificateAction|TestRoleAPIParticipantInputConsumesOneAttempt|TestRoleAPIEngineFailuresDoNotConsumeAttempts|TestLawyerRoleAPIRuntimeFailuresCompleteTurn|TestCouncilAPIRuntimeFailurePropagatesFromOpportunity|TestObserverRoleAPIClassifiesToolAndStorageErrors|TestCouncilAcceptedAnswerEventFailureCompletesWithoutAttemptCharge|TestMalformedAcceptedFailureStateIsRuntimeFailure|TestFailureTransitionRejectsIncompleteOrWrongVersionState|TestWriteCasePacketRejectsPreexistingOutput|TestPublishCasePacketOutputsRemovesPacketAfterManifestFailure|TestWriteCasePacketOutputsRejectsChangedSource|TestVerifyReplayCertificateRejectsV0Schema|TestVerifyReplayCertificateRejectsInitializationOnlyNonterminalReplay|TestVerifyReplayCertificateRejectsCaseIDBoundaryMismatch|TestVerifyReplayCertificateRejectsTrailingJSONData)$' ./arbd/runtime/proceeding
```

Generated-document checks reproduced `docs/proofstats.md` from `common/tools/proofstats.sh` and `docs/theorems.md` from `common/tools/gentheorems.py`.  The selected catalog contains 52 unique declarations, each in its listed file, while the complete proof tree contains 10 files, 98 theorem or lemma declarations, 2,800 lines, and 122,878 bytes.  Local Markdown links resolve, the prose checks reported no diagnostics, and `git diff --check` passes.

The adjacent service correction passed the complete `service/arbd` package with and without the race detector and passed the selected path, collision, persistence, cleanup, and terminal-transition tests in twenty consecutive runs.  The complete adjacent-service test, vet, and build checks also passed.  The paired `adjservices/service/compat/arbd` package then passed against the rebuilt service commands and the current AARD binaries, including the direct output-directory boundary and MCP terminal-read path.

## Deferred Limits

The completed work leaves the limits below explicit.  Each limit names current behavior rather than an implied future commitment.  A later change should define authority, recovery, and compatibility before altering any of these boundaries.

| Limit | Current behavior |
|---|---|
| `call_id` idempotency | Role APIs carry `call_id`, but do not deduplicate calls or cache completed results. |
| Multi-file crash recovery | Ordinary errors clean temporary or reserved outputs where implemented, but no journal repairs a crash between related file publications. |
| Stale output claims | An abandoned `.aard-output-claim` requires manual inspection and cleanup. |
| Observer storage faults | Observer file faults remain request-scoped `runtime_failure` responses and do not stop the case. |
| Certificate file custody | Certificate verification hashes JSON states and replays actions but does not rehash `evidence-store/`. |
| Derivation semantics | Lineage binds a child to a prior identifier and digest plus a nonempty method string, but does not prove the transformation. |
| Process-group scope | Cleanup covers the process group created for one engine call, not a descendant that deliberately leaves that group. |
| Mutex during Lean | State-changing engine calls remain serialized while holding `runContext.mu`, subject to finite engine and turn deadlines. |

## Guide for Later Ports

A later port should preserve the sequence below because each stage establishes the inputs needed by the next.  Procedure-specific record nouns and terminal rules should replace AARD terms where required.  Format names, failure behavior, and runtime ownership require explicit decisions before implementation.

| Stage | Work | Required check |
|---:|---|---|
| 0 | Establish the bounded Lean runner, per-process compiler worker limit, build-job limit, and serialized verification commands. | One narrow target completes inside recorded CPU, memory, task, and time limits. |
| 1 | Identify the question or claim, decision rule, participant roles, terminal states, and existing failure classes. | Written procedure mapping against current engine and runtime fields. |
| 2 | Define immutable initial commitments, submitted or generated records, lineage, and source-state reference rules. | Examples for initial, prior-submission, later-only, collision, and invalid-parent cases. |
| 3 | Add exact opportunity authority to every state-changing participant action and replay record. | Wrong id, version, role, phase, member, and operation are rejected. |
| 4 | Validate catalog, submission metadata, lineage, references, and byte limits in the executable engine. | Focused engine checks cover each rejection and one nonempty accepted history. |
| 5 | Prove initialization, step preservation, reachable integrity, source-state chronology, and replay authority. | One serialized proof target passes before broader proof work. |
| 6 | Build initial custody before initialization and derive the engine catalog from verified descriptors. | Source replacement, digest, size, collision, and empty-file cases are covered. |
| 7 | Put mutable case state and both role turn records under one ownership rule. | Concurrent actions, timeouts, reads, final snapshots, and notifications are covered. |
| 8 | Classify participant input separately from runtime failure and make request schemas finite and strict. | Each error class has asserted response, attempt, turn, and state effects. |
| 9 | Bound engine process groups and preserve semantic rejection separately from process failure. | Timeout, cancellation, malformed output, exit status, and surviving descendants are covered. |
| 10 | Add exclusive output claims, verified publication, paired artifacts, and owned final snapshots. | Concurrent starters, existing targets, rename failure, and partial output are covered. |
| 11 | Version replay certificates and bind verification to procedure, case id, terminal state, actions, and final-state hashes. | Tampered schema, case ids, authority, actions, state, and nonterminal claims are rejected. |
| 12 | Update manuals, procedure rules, proof references, format tables, and deferred limits from the implemented source. | Links resolve, generated references match their sources, and final verification results are recorded. |

ADC cannot copy the AARD state definitions because its court record, participant decision path, and terminal outcomes differ.  It must first decide how attachments, generated case files, exhibits, reports, and arbitrary replay starts enter one typed integrity predicate.  It must also reconcile exact authority with both `apply_decision` and raw engine steps before adopting the replay and certificate fact structure described here.
