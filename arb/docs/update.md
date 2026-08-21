# AAR Record-Integrity and Runtime Update

This document records the AAR work completed in commit `eec612a` (`Strengthen AAR record integrity`), built on the exact-opportunity authority work in `7061d30`.  It explains the failure modes that led to each change, the resulting engine and runtime rules, and the verification performed on the final source.  It also defines an adaptation plan that treats the distinct AARD and ADC state models and runtimes explicitly.

| Reference | Value |
|---|---|
| Baseline | `7061d30 Bind AAR actions to exact opportunities` |
| Completed change | `eec612af2b1c44f54a4c96ad1be8dca99cfd8b44` |
| Change size | 69 files, 11,737 insertions, 2,361 deletions |
| Lean toolchain | Lean 4.32.0 |
| Proof inventory | 39 files, 721 theorem or lemma declarations, 22,858 lines, 1,048,811 bytes |
| Selected theorem catalog | 298 declarations |

## Design objectives

The work binds the legal record in Lean to the bytes and metadata held by the Go runtime.  It gives each mutable case record one synchronization boundary and gives every engine call a finite lifetime.  It also separates participant mistakes from failures in storage, process execution, state publication, or event recording.

| Prior failure mode | Correction |
|---|---|
| Lean initialization had no commitment to the initial case files. | The runtime snapshots every initial file before initialization and supplies a sorted `evidence_catalog` containing the identifier, SHA-256 digest, and byte count. |
| Initial file text, size, hash, and stored bytes could come from different versions of a changing source. | Initial ingestion hashes and copies one opened descriptor, then rebuilds the runtime view from the verified stored object. |
| Offered evidence was checked through a mutable rendering map rather than the authoritative record registry. | Go resolves offers through visible record metadata, and Lean checks the catalog plus submissions already present in the source state. |
| Parent provenance could be incomplete, caller-forged, or lost while the manifest was built. | The public API accepts a parent identifier and derivation method, derives the parent digest from the record, and stores the complete parent triple in state, events, manifests, and replay actions. |
| A replay action could be appended before evidence publication or state validation finished. | Engine evaluation returns a candidate response and replay action without mutating the run, and publication and state changes occur through one ordered commit path. |
| Lawyer and Council API locks protected overlapping views of one mutable case. | One `runContext` mutex owns the case state, record indexes, events, API turn state, versions, and terminal state. |
| Storage and engine failures consumed invalid-attempt allowances. | One participant-input marker identifies caller-controlled errors, and every unmarked tool error is a runtime failure that ends the turn without charging an attempt. |
| Lean subprocesses had no deadline and could keep descendants alive. | Every engine call receives a context deadline, runs in its own process group, checks for remaining group members, and reports exit, parsing, cancellation, stderr, and cleanup errors. |
| Tool schemas rejected unknown fields on paper while handlers accepted them. | Handlers enforce exact key sets and strict field types for tool arguments and nested payloads, accept one JSON value per request body, and pass the validated normalized strings to Lean. |
| Evidence reads verified one pathname open and could return bytes from another. | A read opens one regular-file descriptor, verifies the complete digest and size, and extracts the requested range during that same pass. |
| Concurrent starters could reuse one output directory, and case-packet publication could leave one of two final files. | Startup claims an empty directory through exclusive creation, while case-packet publication reserves both output paths and reports any failure to remove an owned path after publication fails. |

## Cross-layer invariants

The implementation treats the Lean state, replay action list, evidence manifest, and stored bytes as different representations of one case record.  Each representation has a defined owner and publication point.  A later procedure should state these invariants before choosing data types or copying code.

| Invariant | Lean responsibility | Go responsibility |
|---|---|---|
| Initial evidence identity | Validate unique normalized identifiers and canonical SHA-256 strings and retain the catalog unchanged. | Snapshot exact bytes, calculate the digest and size, publish the content-addressed object, and construct the catalog from that snapshot. |
| Submitted evidence identity | Require a canonical digest, positive bounded size, fresh identifier, allowed origin, and catalog disjointness. | Derive the identifier and digest from accepted bytes, reject collisions, and publish matching metadata and files. |
| Parent lineage | Require all three lineage fields together and match the parent identifier and digest against the catalog or an earlier submission. | Accept only the parent identifier and derivation method, derive the parent digest from visible record evidence, and reject caller-supplied `parent_sha256`. |
| Offer validity | Resolve every offered identifier against the initial catalog or the submissions in the filing's source state, subject to the exhibit byte limit. | Resolve the same identifier through `evidenceByID`, restrict it to record-visible evidence, and use manifest size rather than a cached file view. |
| Technical-report limits | Count title and summary lengths as UTF-8 bytes. | Apply the same byte calculation before calling Lean. |
| Accepted transition shape | Return a state whose case object exists and whose version is exactly the source version plus one. | Validate both conditions before publishing state or a replay action. |
| Submission commit | Accept the proposed record transition. | Publish and verify files, write the candidate manifest, then replace in-memory state, indexes, and replay actions together. |
| API state | Model one serialized state transition at a time. | Publish state, events, versions, turn completion, and terminal state under one mutex, then encode responses after releasing it. |
| Failure ownership | Reject invalid actions without changing state. | Charge only participant-controlled tool errors and propagate process and record failures through the case run. |
| Engine lifetime | Produce one protocol response per request. | Apply case, turn, and engine deadlines, request process-group termination, verify cleanup within the adapter's limits, and preserve all relevant errors. |

## Exact action authority

The preceding authority change supplies the source-state identity used by every transaction and replay proof in this update.  `OpportunityAuthority` contains `opportunity_id`, `expected_state_version`, `role`, `phase`, and `member_id`, with an empty member only where the opportunity has no council actor.  The runtime derives this object from the current Lean state and returned opportunity rather than accepting authority fields from a participant.

Lean compares the action authority with `authorityForOpportunity state opportunity` and checks that the opportunity allows the requested action type.  A stale version, wrong phase, wrong role, wrong opportunity, or wrong scheduled council member rejects the action before its body runs.  The same authority object is stored with every `ReplayAction`, so the Lean replay theorems derive both state evolution and the authorization of each accepted action.

`AuthorityConformingReplay` records the source opportunity, exact authority equality, operation authorization, accepted step, and conforming remainder.  The runtime's `authorityForOpportunity` also checks that the cached opportunity version equals the current state version before it invokes Lean.  AARD needs this authority model before it can expose the same certificate facts, while ADC must reconcile it with the version, opportunity, and role fields already carried by `apply_decision`.

## Lean engine

### State and JSON model

The [AAR engine](../engine/Main.lean) adds `EvidenceCommitment` and stores `evidence_catalog : List EvidenceCommitment` at the top level of `ArbitrationState`.  `EvidenceCommitment.size_bytes` is required in JSON, which preserves valid zero-byte initial evidence while rejecting a missing size.  `SubmittedEvidence` now carries `parent_evidence_id`, `parent_sha256`, and `derivation_method` in addition to its own digest and size.

| Definition | Rule |
|---|---|
| `isCanonicalSHA256` | Exactly 64 UTF-8 bytes, each a lowercase hexadecimal character. |
| `validateEvidenceCatalog` | Unique `evidence_id` values, nonempty trimmed identifiers, and canonical digests. |
| `evidenceCommitmentExists` | Exact identifier-and-digest match in the initial catalog or prior submissions. |
| `evidenceReferenceWithinLimit` | Identifier exists in the record and its committed size fits the exhibit limit. |
| `submittedEvidenceParentValid` | Empty lineage or a complete, canonical, non-self parent triple that resolves in the prior record. |
| `submittedEvidenceEntryValid` | Valid metadata, bounded positive size, fresh identifier, catalog disjointness, and valid lineage. |
| `materialOriginAllowed` | The item's phase and role identify an action that may submit evidence. |
| `technicalReportBatchValid` | Every title and summary fits its configured UTF-8 byte limit. |

`getOptionalString` now returns `Except String String`.  A missing field produces the empty value used by the existing optional representation, while a present value must be a string.  This rule covers evidence labels, submitted-evidence source fields, council rationale, and optional failure metadata, so a malformed value cannot disappear during parsing.

The initialization decoder validates the evidence catalog before it creates an active case.  The transition code validates evidence submissions against the prior submission prefix, validates offers against the source-state record, and validates report byte limits before recording a merits filing.  Opening and closing statements reject offered evidence and technical reports, which prevents a successful action from discarding material that the action type cannot record.

### Record integrity

The new [record-integrity proof unit](../engine/Proofs/RecordIntegrity.lean) separates state closure from filing-time chronology.  `RecordIntegrity` describes one state, while `SubmittedEvidenceHistoryValid` describes each submitted item relative to the prefix that existed before it.  This prefix relation proves parent ordering, identifier uniqueness, and disjointness between submitted identifiers and the immutable initial catalog.

| Declaration | Result |
|---|---|
| `EvidenceCatalogValid` | The catalog has no duplicate or malformed commitments. |
| `SubmittedEvidenceHistoryValid` | Every submitted item has an allowed phase-role origin and is valid against its prior history. |
| `RecordIntegrity` | Catalog validity, valid ordered submissions, final-state offer closure, and report byte limits hold together. |
| `evidenceCatalogValid_ids_nodup` | Catalog identifier uniqueness follows from catalog validity. |
| `step_preserves_evidenceCatalog` | Every accepted public action retains the exact catalog. |
| `step_preserves_recordIntegrity` | Every accepted public action preserves the complete state invariant. |
| `reachable_recordIntegrity` | Every reachable state satisfies `RecordIntegrity`. |
| `initialized_run_preserves_evidenceCatalog` | Every run from a successful initialization retains the initialization request's exact catalog. |

`RecordIntegrity.offered` checks the accumulated offers against the complete submitted-evidence list in the same final state.  That property establishes referential closure but cannot express whether an offer preceded the submission that later supplied its identifier.  The replay proof therefore adds a second predicate over source states rather than weakening the state invariant.

### Replay-time chronology and certificates

`MeritsOffersUsePriorRecord` describes one argument, rebuttal, or surrebuttal action and checks its parsed offers against the source state's catalog and submitted-evidence list.  `MeritsOfferChronology` is an inductive replay predicate whose `cons` case stores that pre-step fact, the accepted step, and the same predicate for the remaining actions.  This structure establishes that a later submission cannot justify an earlier filing.

| Declaration | Result |
|---|---|
| `step_ok_meritsOffersUsePriorRecord` | A successful merits step used only evidence available before that step. |
| `replaySteps_success_meritsOfferChronology` | Every successful action replay preserves the pre-step evidence rule at each merits action. |
| `replayInitialized_success_meritsOfferChronology` | The same chronology follows from initialization plus replay. |
| `checkReplayCertificate_ok_meritsOfferChronology` | An accepted replay certificate carries the chronology result. |
| `checkReplayCertificate_ok_recordIntegrity` | An accepted replay certificate yields final-state record integrity and exact catalog equality. |

The [certificate fact structures](../engine/Proofs/CertificateFacts.lean) now include `merits_offer_chronology`, `record_integrity`, and `evidence_catalog_fixed` for both closed and failed cases.  The [replay proofs](../engine/Proofs/Replay.lean) carry exact action authority and evidence chronology through the same accepted action list.  In Lean, a successful `checkReplayCertificate` therefore implies that the claimed final state retains the initialized catalog and that every merits filing used the record available before that filing.

The Go `aar verify-certificate` command runs the executable initialization and action replay, then compares the claimed state and hashes.  It does not call an exported Lean proof checker or return the theorem facts above as protocol data.  A later operational verifier must check or reconstruct every input condition assumed by its Lean certificate theorem, and its documentation must list any theorem premise that the command does not establish.

### Existing proof repairs

Adding catalog validation to initialization and material validation to accepted actions changed the normal form of many existing proofs.  The work updated initialization decompositions, action-result lemmas, actor-role proofs, realizability witnesses, bounded-termination branches, council invariants, outcome proofs, and record-provenance proofs.  The principal files are [step preservation](../engine/Proofs/StepPreservation.lean), [record provenance](../engine/Proofs/RecordProvenance.lean), [bounded termination](../engine/Proofs/BoundedTermination.lean), [opportunity agreement](../engine/Proofs/OpportunityAgreement.lean), and [certificate facts](../engine/Proofs/CertificateFacts.lean).

The proof root imports the new unit, and the selected theorem catalog maps every listed declaration to its current source file.  The generated proof statistics classify `RecordIntegrity.lean` as an invariant unit.  The update retained existing theorem signatures where the stronger implementation still supported them, and stronger detail lemmas now supply weaker result lemmas where both forms are needed.

## Go runtime

### Initial evidence capture

The runtime now completes initial evidence custody before council sampling, initial-state construction, certificate initialization, or the first Lean call.  Automatic discovery excludes the configured complaint by file identity rather than by one conventional filename.  Discovery records path and media information, while the evidence registry creates the authoritative byte snapshot.

| Stage | Operation |
|---|---|
| Discover | Require a regular-file source and retain its path, name, media type, and readability classification. |
| Open | Compare the pathname's file identity with the opened descriptor and require a regular file. |
| Hash | Read the descriptor once, compute SHA-256, and require the byte count to match the opened file size. |
| Publish | Rewind that descriptor and publish through a temporary file in the content-store directory. |
| Verify | Check the temporary or existing hash-addressed object against the expected digest and size. |
| Rebuild | Read the stored object through the verified reader and construct `CaseFile`, `fileByID`, and readable text from that snapshot. |
| Initialize | Sort `EvidenceCommitment` values by identifier and pass them in `initialState`. |

Zero-byte initial evidence remains valid because catalog size and submitted-evidence size have different rules.  An initial commitment may have size zero, while a submitted item must have positive size.  Existing hash-addressed files are reusable only when their type, digest, and size match the requested object.

### Evidence reads

The runtime takes an immutable evidence metadata snapshot under `runContext.mu`, reserves the role's read budget, and releases the lock before file I/O.  The reader opens one regular-file descriptor, verifies the complete SHA-256 digest and byte count, and collects the requested range during that pass.  After reacquiring the mutex, the handler revalidates the exact turn, case context, and deadline before finalizing the reservation or restoring it.

This design prevents pathname replacement between verification and returned bytes.  It also prevents large initial files from holding the case mutex while the runtime hashes them.  Lawyer and Council reads consume per-opportunity count and byte budgets and record events, while Observer reads use the same custody checks without a participant budget.

### Submitted evidence and lineage

Direct and chunked submissions produce the same `SubmittedEvidenceMeta`, engine payload, evidence metadata, event fields, and certificate action fields.  The runtime derives `parent_sha256` from a visible `case_packet` or `submitted_evidence` record and revalidates the complete triple before commit.  A parent must predate its child because Lean validates each child against the prior submission prefix.

Chunked uploads bind the session to the role and phase that created it.  A commit-time expected digest may fill an omitted begin-time digest, but it cannot replace a different begin-time commitment.  Commit rechecks the per-side submission count, staged-file type, exact length, digest, authority, lineage, and identifier collision before it calls Lean.

Chunk writes use the verified stored length as the authoritative next offset.  `WriteAt` writes at that offset, and the runtime records a partial prefix before returning any write, short-write, sync, close, or reconciliation errors.  A retry receives the next valid offset, while a truncated or non-regular staging object is a runtime integrity failure.

### Evidence submission transaction

`commitEvidenceSubmissionLocked` handles direct and chunked admission through one ordered path.  The path separates candidate construction from publication and in-memory commit.  Every fallible step that precedes manifest publication leaves the prior Lean state, replay list, and evidence indexes unchanged.

| Order | Commit operation | Published state after failure |
|---:|---|---|
| 1 | Validate origin, identifier, lineage, metadata, and candidate registry. | Prior state remains authoritative. |
| 2 | Evaluate the bounded Lean `submit_evidence` action without appending a replay action. | Prior state remains authoritative. |
| 3 | Require `ok:true`, a nonempty case object, and `state_version = source + 1`. | Prior state remains authoritative. |
| 4 | Publish and verify the content-addressed object and submitted-evidence copy. | Matching files may remain as reusable, unreferenced artifacts. |
| 5 | Build candidate case-file, evidence, and submitted-evidence indexes. | Prior in-memory indexes remain authoritative. |
| 6 | Atomically replace `evidence-manifest.json` with the candidate manifest. | The new manifest is the durable admission boundary. |
| 7 | Recheck case cancellation and the turn deadline, restoring the prior manifest if the action can no longer commit. | The prior manifest remains authoritative when restoration succeeds. |
| 8 | Replace state and indexes, append the replay action, remove the completed upload session, and notify both role APIs. | The evidence and Lean transition are committed together in memory. |
| 9 | Append the submitted-evidence event. | An append failure leaves the accepted record committed, ends the turn, and returns a runtime failure. |

Publication accepts an existing final file only when its digest and size match, which makes an internal retry after a manifest failure safe.  A chunked session that has moved from staging to the verified submitted copy retains that path until the manifest commit succeeds.  The code returns cleanup errors together with the primary failure rather than replacing either error.

### State and replay publication

Every accepted action now passes through `acceptedStepState`.  That validator requires a nonempty state, a nonempty case object, and exactly one state-version increment.  Lawyer decisions, Council votes, submitted evidence, and procedural failures publish the validated state and prepared replay action together.

`fail_opportunity` builds authoritative `type`, `role`, `phase`, `opportunity_id`, `reason`, `message`, and council-member fields before it accepts optional details.  Caller details cannot replace those fields.  The failure transition uses the same bounded engine evaluation and accepted-state checks as ordinary actions.

### Shared mutable state

One mutex in `runContext` owns all mutable case-wide data and both role APIs' turn records.  API-specific condition variables use that mutex, which gives Lawyer and Council waiters one publication order.  The server publishes both API pointers before it begins serving requests.

| Protected by `runContext.mu` | Kept outside the mutex |
|---|---|
| Lean state and terminal reason | Immutable configuration and complaint |
| Replay actions and certificate initialization state | Provider network calls |
| Evidence registry, lookup maps, submissions, and upload sessions | Condition and channel waits |
| Events, turn counter, and provider error class | HTTP response encoding and writes |
| Lawyer and Council active turns, attempts, versions, and conditions | Evidence file verification after a reservation |
| Final result inputs until the deep snapshot is complete | Final packet rendering from the owned snapshot |

Successful actions and failures notify both role APIs after publishing their state.  Nonterminal invalid-attempt changes and successful Lawyer or Council evidence reads also increment observable versions and notify waiters.  Final output captures owned copies of nested state, events, evidence metadata, file indexes, work-product paths, and replay actions under the mutex before performing file output.

The chosen synchronization model keeps Lean transitions serialized under the mutex.  A separate in-flight reservation system could permit reads during an engine step, but it would require supersession tokens and duplicate-failure arbitration.  The implementation instead bounds every engine call and keeps the state commit rule direct.

### API validation and error ownership

The Lawyer and Council POST decoders accept exactly one JSON value and apply bounded request bodies.  Tool argument objects, decision payloads, offered-evidence entries, technical-report entries, and Council votes reject unknown keys in agreement with their published schemas.  The runtime validates caller fields before inserting trusted fields such as the active council member identifier.

Go normalizes accepted Lawyer strings before sending them to Lean.  This keeps Go's `strings.TrimSpace` checks from accepting a value that Lean's ASCII trimming would interpret differently.  The Lawyer request limit adds the standard Base64-encoded length of `max_direct_submitted_evidence_bytes` to the general response-sized metadata allowance, with overflow checked before conversion to `int64`.

| Error class | Examples | Turn effect |
|---|---|---|
| HTTP envelope or routing | Bad JSON, wrong case, missing role, stale opportunity, or no active turn. | Return an HTTP/API error without charging the active opportunity. |
| Participant input | Unknown tool, wrong argument type, unknown record identifier, invalid range, bad upload offset, incomplete upload, invalid vote, or policy-limit violation. | Consume one invalid attempt and keep the turn active while attempts remain. |
| Procedural deadline | The opportunity deadline expires before a valid action commits. | Record one `deadline_expired` Lean transition using a fresh engine-call limit. |
| Runtime failure | Storage, hashing, staging, event, engine execution, Lean rejection after Go validation, malformed accepted state, or publication failure. | Preserve the attempt allowance, complete the turn with the error, and return the error from the case run. |

The single `participantInputError` marker defines this split.  Validation code marks only errors attributable to the resolved participant request, so an unclassified error defaults to a runtime failure.  Council execution propagates a completed turn's runtime error directly instead of converting it into an exhausted-attempt member failure.

Observer validation uses the same response codes without changing a participant turn.  An Observer storage error is returned to that request because the Observer API has no run-level error channel.  This behavior requires a separate policy decision before another procedure allows a read-only client to terminate its case.

### Bounded Lean processes

The [Lean process adapter](../runtime/lean/engine.go) accepts `context.Context` for initialization, steps, and opportunity queries.  The runtime default is 30 seconds through `runtime.engine_call_timeout_seconds`, with a positive bounded CLI override.  Certificate replay applies the same explicit limit to initialization and every replayed action.

| Condition | Result |
|---|---|
| Case context ends first | Request process-group termination and return the case cancellation cause together with any cleanup failure. |
| Opportunity deadline ends first | Return the distinct turn-deadline cause and record one fresh `deadline_expired` transition. |
| Engine limit ends first | Return a runtime engine timeout.  Process cleanup that crosses the later turn deadline does not change the cause. |
| HTTP client disconnects after a valid POST was accepted | Continue the mutation under the case context and suppress the abandoned response write. |
| Engine exits 1 with `{ "ok": false, "error": "..." }` and no surviving process group | Treat the response as a normal Lean rejection. |
| Engine exits with another status, a signal, malformed JSON, incomplete rejection data, or a surviving process-group member | Return the process error together with stderr, parse, rejection, cancellation, and cleanup information that applies. |

Each command starts in its own process group.  After the parent returns, the adapter checks that group, requests termination of remaining group members, verifies the result available to it, and reports cleanup failures.  Tests cover clean protocol rejection, exit status 7, signal termination, malformed JSON, custom cancellation causes, deadline expiry, pipe-drain timeout, and process-group cleanup after zero and nonzero exits.

### Output and case-packet publication

`aar case` accepts an absent or empty output directory and creates an exclusive `.aar-output-claim` before the first case artifact.  It reads the directory again after acquiring the claim, which closes the stale emptiness-check race between two starters.  Initial case-manifest publication releases the claim and returns any publication or cleanup error.

The direct AAR service leaves that output directory to the core and stores new child standard streams beneath `RegistryDir/logs/<case_id>/`.  Its artifact API retains the logical `service-logs/aar.stdout` and `service-logs/aar.stderr` names and accepts exact legacy paths recorded beneath an older case output directory.  The service reserves case identifiers against active and persisted direct and Clerk records, creates confined log directories and files exclusively, publishes its record through a unique temporary file, and verifies directory identity before failed-create cleanup.  Every direct terminal consumer uses one reader that requires matching case and run identifiers and consistent top-level and final Lean case statuses.  The service-owned `aar-run` and attested Clerk layouts require a separate output-ownership change.

The automatic case-file scan excludes the configured complaint through file identity and rejects non-regular sources.  The case-packet command binds each manifest digest and archive member to one opened descriptor, checking file identity, size, byte count, and SHA-384.  A source replacement or change between manifest construction and archive writing aborts publication.

Case-packet publication creates temporary packet and manifest files, calculates the final packet hash and size before publication, and reserves both final paths with exclusive creation.  A failure to reserve or rename either path attempts to remove every output path owned by that invocation and returns cleanup errors with the publication error.  Existing packet or manifest targets remain unchanged when the operation returns success or when rollback succeeds.

## Verification

Lean builds ran only through the repository's resource-limited wrapper.  The wrapper used one Lean thread, one Lake job, `CPUQuota=100%`, `MemoryHigh=4 GiB`, `MemoryMax=6 GiB`, `MemorySwapMax=1 GiB`, `TasksMax=64`, and a 900-second command timeout.  Both final targets completed without diagnostics.

```text
./tmp/leanrun-aar lake --dir arb/engine build Proofs
./tmp/leanrun-aar lake --dir arb/engine build aarengine
```

The first command completed 43 jobs, and the second completed 4 jobs.  The rebuilt Lake executable and `arb/.bin/aarengine` had the same SHA-256 digest: `bff1b813dfb5323caabb70091f92f8daed79dd06fb2470b511c32cb3e46a818c`.  The real-engine Go tests exercised catalog validation, UTF-8 report limits, strict optional fields, lineage, identifier collisions, prior-parent use, and later offers of submitted evidence.

Go commands ran one at a time with `GOMAXPROCS=2`, a repository-local temporary directory, and a repository-local build cache.  The following package-level commands passed.  Verification made no external provider, container, or network request.

```text
go test -count=1 ./arb/runtime/lean
go test -count=1 ./arb/runtime/proceeding
go test -race -count=1 ./arb/runtime/proceeding
go test -count=1 ./arb/runtime/...
go vet ./arb/runtime/...
```

The [development journal](../../devnotes.md) records the focused coverage and final status: selected concurrency tests passed in 20 consecutive runs, the engine process and protocol tests passed, and the AAR command build completed.  The focused files include `concurrency_test.go`, `record_integrity_test.go`, `lawyerapi_test.go`, `councilapi_test.go`, and the Lean adapter's `engine_test.go`.  Generated proof statistics and the selected theorem Markdown matched their source scripts and TSV catalog, while `git diff --check` passed before commit.  The journal does not preserve one final combined regular expression for those selected tests.

The adjacent direct-service correction passed the complete `service/arb` package with and without the race detector and passed the selected path, collision, persistence, cleanup, and terminal-transition tests in twenty consecutive runs.  The complete adjacent-service test, vet, and build checks also passed.  The paired `adjservices/service/compat/arb` package passed against the rebuilt service commands and the current AAR binaries, including the direct output-directory boundary and MCP terminal-read path.

### Documentation and generated references

The theorem catalog now maps each selected declaration to the file that defines it, including the declarations moved into `DeliberationSummaryCore.lean` and the new record-integrity results.  The theorem generator omits trailing empty TSV fields, and the proof-statistics script classifies `RecordIntegrity.lean` with the invariant proofs.  The generated Markdown, proof notes, verification summary, engine review, manual, process specification, evidence guide, and failure guide were updated against the final source.

The AAR Makefile now separates the `aarengine` target from the Go command build and makes the engine a prerequisite of its test target.  Its Lean invocations set `LEAN_NUM_THREADS=1`, while workstation builds still use the stricter leanrunner limits recorded above.  `make test` therefore rebuilds the engine before the real-engine Go tests run.

## Current limits

These limits describe the completed AAR implementation and should remain visible during adaptation.  Some require a design choice because the correct behavior depends on the procedure and its operator model.  Copying the current behavior without that choice would preserve an unresolved boundary.

| Limit | Current behavior | Later decision or work |
|---|---|---|
| Mutation idempotency | Lawyer and Council `/do` accept `call_id` but retain no completed-result index. | Define durable or in-memory request identity before retries can resolve a lost response. |
| Top-level HTTP fields | Strict key checks cover tool arguments and nested payloads, while the envelope decoder accepts unknown top-level fields. | Decide whether each procedure needs strict envelope rejection before documenting that broader rule. |
| Final packet crash recovery | Evidence admission has an atomic manifest boundary, while `run.json`, `state.json`, `certificate.json`, and sibling files publish separately. | Choose a versioned record directory or one authoritative root object before claiming crash-atomic final output. |
| Accepted-action event failure | The accepted state, replay action, and evidence admission remain committed, while the turn and run end with a runtime error. | Preserve this rule or introduce a transactional event authority before adapting it. |
| Observer storage failure | The request receives `runtime_failure`, while the case continues. | Decide whether a read-only client may stop the procedure. |
| File-path attestation | Explicit case-file selection accepts a symbolic link whose resolved target is regular, while stored bytes receive full custody checks. | Add source-path attestation only if the source pathname has evidentiary meaning. |
| Cancellation during file mutation | Case cancellation stops bounded Lean calls, while a file operation may leave a verified but unreferenced artifact in a partial output directory. | Add cleanup or resumable publication if the target procedure requires reuse of a canceled run directory. |
| Certificate byte verification | Replay verifies metadata, transitions, final-state equality, and hashes of JSON state. | Rehash `evidence-store/` in a separate packet-custody verifier when byte presence must be checked during certificate verification. |
| Operational theorem boundary | The Go verifier replays the executable engine protocol and compares state hashes.  It does not invoke Lean's theorem `checkReplayCertificate` as an exported checker. | Keep the executable replay checks and proved implications aligned, or expose a theorem-backed checker through a deliberate engine API. |
| Derivation semantics | Lineage proves the identity and digest of a prior parent and records a nonempty method. | Verify that the child bytes result from the stated transformation when the procedure requires that claim. |
| JSON representation proofs | Real-engine tests exercise catalog commitments, strict optional fields, submitted lineage, prior-parent references, later offers, and UTF-8 report limits across the Go-to-Lean boundary. | Add JSON round-trip lemmas when the formal boundary needs a proof that every serialized value decodes to the intended Lean structure. |
| Formal nonvacuity example | Real-engine tests exercise a nonempty submitted item, derived child, and later offer. | Add a Lean witness theorem if the formal library needs an explicit inhabited chronology branch. |
| Submitted-history run theorem | Step results construct submissions by suffix and the history invariant validates every prefix. | Add a named whole-run prefix theorem when later proofs need direct append-only submission history. |
| Terminal-run existence | `stepPath_length_plus_budget_le` bounds a successful path, and `initializedStepPathMaximal_terminal_accounted` classifies an initialized maximal path's endpoint. | Prove that every valid initialization admits a bounded maximal path before claiming constructive terminal-run existence. |
| Legacy removal action | Lean still accepts the system action `remove_council_member`, while the Go runtime records member failure through `fail_opportunity`. | Remove the unused action only after reviewing its proof and replay compatibility costs. |
| Mutex availability during Lean | The case mutex remains held during a bounded engine call. | Add in-flight transition reservations only if measured engine latency requires concurrent reads during evaluation. |
| Output-directory crash residue | An abrupt exit can leave `.aar-output-claim`, and later startup rejects the nonempty directory. | Define claim recovery only with an ownership and stale-process rule. |

## Adaptation to AARD and ADC

The adaptation should preserve the invariants and failure boundaries while using each procedure's record vocabulary.  AARD has the closest code structure and can adopt much of the AAR implementation after its engine schema changes.  ADC has a larger state model, separate `step` and `apply_decision` paths, existing attachment commitments, and one multi-role API, so its record definition must come before runtime edits.

### Shared sequence

The order below keeps the executable model, runtime record, and proof claims aligned.  Each stage has a verification gate before the next stage changes another representation.  A later implementation should retain this order unless a procedure-specific dependency requires a documented change.

| Stage | Work | Gate |
|---:|---|---|
| 0 | Establish the procedure's leanrunner, `weakLeanArgs = ["-j1"]`, one-job build settings, and command timeout before any Lean verification. | One narrow engine or proof target completes inside the recorded CPU, memory, swap, task, and time limits. |
| 1 | Define the procedure's record items, visibility rules, immutable initial commitments, submitted or generated items, derivation relation, and filing-time reference rule. | English invariant review against the procedure rules and current state fields. |
| 2 | Add exact opportunity authority to every state-changing action that lacks it. | Engine tests reject stale version, wrong opportunity, role, phase, and member or juror identity. |
| 3 | Add catalog and lineage types to Lean, then validate them at initialization and admission. | Focused engine tests cover empty files, malformed digests, duplicate identifiers, collisions, incomplete lineage, self-parenting, and unknown parents. |
| 4 | Add source-state offer or citation validation and UTF-8 byte limits to every filing action that records those values. | Real-engine tests cover initial references, prior submissions, later-only submissions, oversized items, and multibyte text. |
| 5 | Prove initialization, one-step preservation, reachability, catalog equality, prior-parent ordering, and replay-time chronology. | Narrow proof target, complete proof root, and executable build pass through the procedure's leanrunner. |
| 6 | Snapshot initial bytes and build the Lean catalog from the verified store before council sampling, provider preflight, or initialization. | Source replacement, configured-complaint exclusion, zero-byte, regular-file, digest, cached-text, and existing-object tests pass. |
| 7 | Replace direct mutation with candidate evaluation and an ordered file-manifest-state-replay commit. | Injected publication and manifest failures leave the prior state and replay list unchanged, and matching artifacts support retry. |
| 8 | Establish one mutable-state ownership rule for the runner and role APIs. | Race tests cover both API directions, final state, duplicate actions, evidence budgets, HTTP writes, provider waits, and file I/O. |
| 9 | Add strict request decoding and participant/runtime error classification. | Handler tests assert response code, attempt count, turn completion, returned run error, and no discarded fields. |
| 10 | Add context-aware engine calls, process-group cleanup, accepted-state checks, and replay timeouts. | Tests cover a blocked process, case cancellation, unexpected exit status, malformed protocol response, a surviving process-group member, opportunity expiry, and HTTP client disconnection. |
| 11 | Add exclusive output claims, paired artifact publication, and owned final snapshots. | Tests cover two starters, a stale directory read, source replacement, existing targets, rename failure, and returned cleanup errors. |
| 12 | Update process specifications, evidence notes, failure rules, manuals, proof summaries, theorem indexes, and development notes. | Generated documents match their source catalogs, links resolve, and every stated command has a recorded successful run. |

### AARD mapping

The `arbd` tree has corresponding files for evidence storage, Lawyer and Council APIs, case packets, replay certificates, and a smaller Lean proof library.  Its [engine](../../arbd/engine/Main.lean) stores submitted evidence but has no initial `evidence_catalog`, exact `OpportunityAuthority`, record-integrity predicate, or replay-time offer chronology.  Its [proceeding runtime](../../arbd/runtime/proceeding/run.go) samples the council and initializes Lean before it builds the evidence registry, uses separate API synchronization, permits unbounded engine calls, and mutates evidence through the earlier publication path.

| AAR source | AARD target | Adaptation |
|---|---|---|
| [Engine model and validation](../engine/Main.lean) | `arbd/engine/Main.lean` | Add commitments, authority, strict optional parsing, lineage, offer validation, report byte checks, and phase-specific supplemental-material validation after resolving the surrebuttal conflict.  Retain numeric `CouncilAnswer`. |
| [Record-integrity proofs](../engine/Proofs/RecordIntegrity.lean) | New `arbd/engine/Proofs/RecordIntegrity.lean` | Replace vote-specific branches with answer-specific branches and prove preservation for every AARD action. |
| [Replay and certificate facts](../engine/Proofs/Replay.lean) | `arbd/engine/Proofs/Replay.lean`, `CertificateFacts.lean` | Add authority-conforming replay, offer chronology, final-state integrity, and fixed catalog facts to closed and failed certificates. |
| [Lean process adapter](../runtime/lean/engine.go) | `arbd/runtime/lean/engine.go` | Add contexts, deadlines, process-group cleanup, exit-1 rejection semantics, and joined parse/process errors. |
| [Proceeding runtime](../runtime/proceeding/run.go) | `arbd/runtime/proceeding/run.go`, `types.go`, `helpers.go`, `case_packet.go` | Ingest the verified catalog before council sampling, provider preflight, and initialization, exclude the configured complaint by file identity in run and packet discovery, add one case mutex, and capture an owned final snapshot. |
| [Evidence custody](../runtime/proceeding/evidence.go) | `arbd/runtime/proceeding/evidence.go`, `lawyerapi.go` | Port single-descriptor capture and reads, lineage derivation, chunk reconciliation, candidate manifest publication, and one commit path. |
| [Role API rules](../runtime/proceeding/roleapi_errors.go) | `arbd/runtime/proceeding/lawyerapi.go`, `councilapi.go`, new error helper | Port strict schemas, error ownership, shared notifications, accepted-state checks, and deadline behavior. |
| [Case-packet publication](../runtime/proceeding/case_packet.go) | `arbd/runtime/proceeding/case_packet.go` | Port source identity checks, prepublication packet hashing, paired reservations, and rollback. |

AARD needs a small set of decisions before implementation.  Its `CouncilAnswer` values range from 0 through 100, and its supplemental-material rule is currently inconsistent: ARAP and the Go API allow surrebuttal material, while Lean calls `recordMeritsSubmission` with `allowSupplementalMaterials = false`.  The choices below remain open and require approval before the affected state or durable formats change.

| AARD decision | Options and consequences |
|---|---|
| Surrebuttal materials | Permit them in Lean to match ARAP and Go, which expands the filing-origin proofs, or reject them in ARAP and Go, which changes the public procedure. |
| Certificate schema | Extend the current schema in place, which preserves one format name but changes its accepted fields, or introduce a version, which preserves old replay behavior at the cost of version handling. |
| Evidence identifiers | Preserve the current digest-prefixed `ev_<sha[:12]>_<slug>` scheme and add commitments around it, which favors packet compatibility, or adopt a full-digest, digest-only, or otherwise revised scheme, which requires migration and replay rules. |
| Failed council members | Preserve the current `fail_opportunity` representation and prove its interaction with record actions, which leaves the durable schema unchanged, or add an explicit member-removal record, which changes state and certificate formats and requires replay compatibility or a schema version. |

Keeping the answer model unchanged isolates record work from substantive judgment semantics.  A real-engine test should include a derived submission and a later offer because that path exercises catalog, prior history, lineage, source-state chronology, runtime publication, and replay together.  The test should compare the accepted engine state, stored evidence manifest, and replay certificate for the same parent and offer identifiers.

### ADC mapping

ADC already records attachment identifiers, SHA-256 digests, and sizes, and `common/documents` verifies complaint attachment bytes.  Its [Lean `CaseState`](../../adc/engine/Main.lean) stores `case_files`, `file_events`, and `technical_reports`, while [Go initialization](../../adc/runtime/runner/state_init.go) adds presentation and runtime fields such as `filing_documents` outside that typed Lean state.  The first ADC task is a domain decision that classifies complaint attachments, `import_case_file`, `produce_case_file`, `offer_exhibit`, and report file references within the formal court record.

| ADC operation | Current record effect | Record-integrity treatment |
|---|---|---|
| Complaint attachment initialization | Creates a `case_files` entry and a `filed_with_complaint` event. | Treat as the initial catalog after validating identifier uniqueness, canonical digest, size, and stored bytes. |
| `import_case_file` | Creates a byte-bearing `case_files` entry and an `import_case_file` event. | Treat as dynamic admission with freshness, origin, byte custody, and optional lineage. |
| `produce_case_file` | Appends a visibility event for an existing `file_id`. | Preserve the existing commitment rather than creating a second submitted item. |
| Go `offer_case_file_as_exhibit` | Requires an existing nonempty `file_id`, emits `offer_exhibit`, and records the file-backed offer. | Prove that the emitted reference existed in the source-state record. |
| Raw Lean `offer_exhibit` | Permits an empty `file_id`.  That branch does not append a file event. | Decide whether the empty form is a distinct non-file exhibit or an invalid participant action, then align Go, Lean, replay, and proofs. |
| `submit_technical_report` | Appends a typed report whose `file_id` may be empty or unchecked. | Require every nonempty source identifier to resolve in the source-state record. |
| File read operations | Read an existing attachment or dynamic case file without a court-state transition. | Retain `common/documents` for initial attachments and add digest-and-size verification for later dynamic files. |

| AAR source | ADC target | Adaptation |
|---|---|---|
| `EvidenceCommitment` and initialization catalog | `adc/engine/Main.lean`, `adc/runtime/runner/state_init.go` | Reuse complaint attachment commitments or define a broader `RecordCommitment`.  Make the committed catalog part of `CourtState` rather than an initialization-only argument. |
| Submitted-evidence history | `adc/engine/Main.lean` and filing or exhibit actions | Define append-only admitted exhibits, generated filings, and derivations using ADC's docket and filing vocabulary. |
| Filing-time offer chronology | `adc/engine/Proofs/Replay.lean` and a new record proof unit | State the property over ADC actions that cite files, exhibits, reports, or docket entries, using each replay source state. |
| Exact action authority | `adc/engine/Main.lean`, `adc/runtime/lean/engine.go`, `adc/runtime/runner/opportunity_*` | Reconcile direct `step` actions with the existing `apply_decision` request, which already carries version, opportunity, and role outside the action payload. |
| Candidate replay action | `adc/runtime/runner/certificate.go` | Stop appending a successful `step` transition before the surrounding state or file operation commits.  Handle `step` and `apply_decision` through one publication rule. |
| Shared mutable state | `adc/runtime/runner/runner.go`, `roleapi.go`, state and turn helpers | Replace the separate runner/API ownership model with one documented mutex or an actor model chosen before edits. |
| Bounded engine calls | `adc/runtime/lean/engine.go`, `runtime_limits.go`, command flags, certificate verifier | Add contexts and the explicit engine-call limit to initialization, views, opportunity queries, steps, decisions, and replay. |
| Byte custody | `adc/runtime/runner/attachment_reader.go`, `local_actions.go`, `adc/runtime/store`, `adc/runtime/casepacket` | Retain `common/documents` for complaint attachments.  Snapshot inline and host-path `import_case_file` bytes into owned storage before commit, then verify recorded digest and size during later reads. |
| Strict Role API | `adc/runtime/runner/roleapi.go`, tool and validation files | Enforce exact request shapes, one JSON value, trusted identity insertion, error ownership, bounded bodies, and response writes outside the case lock. |

ADC's multi-role and juror flows require procedure-specific authority rather than AAR's plaintiff-defendant-council cases.  Court actions include deterministic local steps, external-role decisions, juror actions, and `apply_decision` transitions, so the authority and commit abstraction must cover each state-changing route.  The proof should name ADC's record categories before it claims catalog disjointness, parent ordering, or temporal citation validity.

ADC's `applyDecision` already validates state version, opportunity identifier, role, allowed tool, and decision constraints.  A pass mutates state through `apply_decision` and records that transition.  An `execute_tool` result emits a `CourtAction`, after which `step` performs the mutation and the certificate records only that raw step.  This split loses the authority that produced a participant tool action, so the certificate design must either record a combined decision-plus-step transition that reruns `applyDecision` and compares the emitted action exactly, or carry and verify equivalent authority on the emitted step.  Accepted-state validation then applies to the complete `CourtState` and the version relation of the chosen replay form.

ADC currently stores `case_files` and `file_events` as `List Json`, which leaves identifier, digest, size, origin, and lineage facts behind repeated decoders or untyped predicates.  The representation decision belongs before proof implementation because it determines serialized-state compatibility and the size of every preservation proof.  The pending ADC decisions are collected below so the port does not select them through incidental implementation choices.

| ADC decision | Options and consequences |
|---|---|
| Record representation | The options are (a) replace the JSON lists with typed records, which creates one formal source of truth and changes serialized state, (b) prove total predicates over the existing lists, which preserves the format and adds decoder obligations, or (c) add a typed sidecar, which reduces the initial conversion and creates synchronization proofs. |
| Participant replay authority | Record a combined decision-plus-step transition, which preserves the existing raw action and adds a replay form, or add authority to the emitted step, which changes the action and certificate formats. |
| Other replay classes | Give deterministic scenario actions and system failures distinct constructors, which makes replay authority explicit, or retain one raw-step constructor with a classifier and proof that participant actions cannot enter it. |
| Empty exhibit identifier | Treat it as a non-file exhibit with separate semantics, or reject it and change the Lean action rule. |
| Arbitrary replay start | Add an executable engine validator for the supplied start, which expands the protocol and keeps certificate acceptance self-contained, or require `RecordIntegrity start` as a theorem premise supplied outside the certificate, which leaves the protocol unchanged and enlarges the trusted boundary. |
| Runtime ownership | Use one runner mutex, which keeps commits serialized, or use one actor loop, which changes request and timeout dispatch. |

`offer_exhibit` currently permits an empty `file_id`, and `submit_technical_report` can name a source file.  ADC must decide whether an empty exhibit reference represents a non-file-backed exhibit or invalid record data, then state the report reference rule in the same source-state chronology.  Its report limits currently use characters and support local-rule overrides, so the chronology predicate must calculate the applicable limit from the action's source state unless a separate approved design adds an effective-limit field to serialized state.

ADC replay supports both an initialization request and an arbitrary supplied starting state.  Catalog preservation therefore needs one theorem for a successful `initialize_case` request and another theorem relating a replay target to an arbitrary replay start.  Certificate facts should select the theorem that matches the certificate's initialization form.

ADC's engine currently returns semantic `{ "ok": false }` responses with process exit status zero.  Contexts, process groups, cleanup, and error joining can follow the AAR adapter, while ADC should continue treating every nonzero exit as a process failure unless its engine protocol changes first.  Protocol-exit behavior must therefore remain a procedure-specific test rather than a copied AAR constant.

### Completion rule

The gates in the shared sequence are the adaptation acceptance criteria.  A port is complete when its executable model, runtime publication rules, operational certificate verifier, Lean checker implications, tests, and documentation describe the same procedure-specific record and failure semantics.  A procedure-specific difference changes the invariant and its gate before implementation begins.
