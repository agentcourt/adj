# Development Notes

## Repository Scope

This repository owns the ADC, ARB, AARD, simple, and quick procedures.  Each procedure includes the rules, one-case Go runtime, command-line program, durable record, and tests that its design requires, while ADC, ARB, and AARD also include Lean engines and proofs.  Operational consumers use the documented process interface without importing procedure implementation packages.

The shared `common/` tree contains code required by more than one procedure.  New shared packages must have at least two current consumers and a narrower API than the code they replace.  Procedure-specific behavior belongs in its procedure tree.

## Error and Command Policy

Commands return every input, output, formatting, storage, HTTP, and shutdown error to their caller.  A command writes diagnostics to standard error, writes machine-readable results to standard output where its manual specifies them, and exits nonzero when command execution fails.  Every command rejects unexpected positional arguments and reports help-output failures.

Long-running case commands derive their execution context from interrupt signals.  Startup output failures cancel the case before the command returns.  Case APIs and runtime helpers preserve the original operation error together with any cleanup error.

## Interface Maintenance

The [core process interface](docs/service-interface.md) defines the executable, private HTTP, and durable-record behavior consumed by `adjservices`.  An interface edit requires corresponding test and documentation edits in both repositories.  The paired interface tests use explicit core binaries and an explicit core checkout.

## AAR Proof Strengthening

The `aar-proof-strengthening` branch strengthens AAR before further ADC or AARD proof work.  Completed work binds every state-changing action to the exact current opportunity and proves record integrity, evidence-catalog preservation, and filing-time offer chronology as implications of the Lean certificate checker.  The remaining agenda includes constructive terminal-run existence, outcome stability, council symmetry, broader certificate consequences, and independent count-rule properties.  Engine, runtime, certificate, proof, and documentation changes remain inside AAR except for this development record.  The [AAR record-integrity and runtime update](arb/docs/update.md) consolidates the completed design and the adaptation plan for AARD and ADC.

- [x] Bind actions to the current opportunity id, state version, role, phase, and council member.
- [x] Make closed and failed states reject state-changing actions.
- [x] Bind evidence commitments, submission provenance, and technical-report byte limits into Lean and the runtime.
- [ ] Prove that every valid initialization admits a terminal run within the existing bound.
- [ ] Prove that a reached substantive threshold determines every terminal continuation.
- [ ] Prove whole-run council renaming and independent-vote order invariance.
- [ ] Derive authorization, global invariants, due process, and outcome soundness from terminal certificate verification.
- [ ] Replace the count-rule characterization assumptions with independent rule properties.
- [ ] Reconcile the proof root, theorem index, and verification documentation.

Each action now carries the opportunity id, expected state version, role, phase, and scheduled council-member id supplied by `next_opportunity`.  The public Lean step checks that authority against the current state before applying the action, and the Go runtime records the same authority in `aar.replay-certificate.v1`.  A successful certificate replay yields an `AuthorityConformingReplay`, which states the source opportunity and exact authority for every accepted action.  Separate theorems state that closed and failed cases reject every action.

The runner creates a missing output directory or accepts an existing empty directory, then claims it through an exclusive `.aar-output-claim` file.  It checks the directory again after acquiring the claim, which prevents a starter from relying on an emptiness observation made before another starter published its case manifest.  A competing starter fails without removing the first starter's claim.  The runner keeps the claim through initial case-manifest publication, then removes it while returning any publication or cleanup errors.  A process exit before cleanup can leave the claim, and a later attempt rejects that directory as nonempty.

The automatic case-file scan excludes the configured complaint by file identity and rejects non-regular sources without opening them.  It does not read text from the source path because the runtime rebuilds readable text from the stored snapshot.  Explicit file selection retains its deliberate inclusion behavior but applies the same regular-file requirement; a symbolic link to a regular source remains accepted while attestation and source-path security remain deferred.

The runtime copies every initial case file into the verified content-addressed evidence store before council selection and Lean initialization.  It opens each regular source once, hashes it, rewinds the same descriptor, publishes from that descriptor, and rebuilds each runtime case-file view from the stored snapshot.  It supplies Lean with a sorted catalog containing each evidence identifier, SHA-256 digest, and byte count.  Lean requires each digest to contain exactly 64 lowercase hexadecimal characters, validates unique normalized catalog identifiers at initialization, and retains the catalog unchanged through every successful public step.  `evidenceCatalogValid_ids_nodup` exposes identifier uniqueness from the catalog invariant.

A submitted item has a unique identifier disjoint from the initial catalog, an allowed phase-role origin, a positive size within the submission limit, complete normalized metadata, and a canonical SHA-256 commitment.  Its optional lineage consists of a parent identifier, parent SHA-256 digest, and derivation method supplied together; the parent must match an initial commitment or an earlier submission and cannot name the submitted item.  The Lean JSON parser rejects a present non-string lineage member and requires a canonical parent digest.  The runtime accepts the parent identifier and derivation method, obtains the parent digest from visible record evidence, and rejects a caller-supplied parent digest.

Evidence submission first evaluates the proposed Lean step without appending a replay action.  Every accepted engine transition must return a nonempty case object and a state version exactly one greater than its source state before the runtime publishes state or a certificate action.  The runtime then publishes and verifies the content-addressed object and submitted-evidence copy.  For a chunked upload, it removes the staging file and points the upload session at the submitted copy before writing the candidate evidence manifest atomically.  A manifest-write failure leaves the Lean state, certificate actions, and evidence indexes unchanged, while the lower-level publication operation can reuse matching content-addressed files and a retained upload session.  A public participant handler reports the same failure as `runtime_failure`, preserves the invalid-attempt count, and ends the active turn and run.  After the manifest write succeeds, the runtime replaces the state and evidence indexes, appends the certificate action, and removes any completed upload session; atomic recovery across the manifest, root state, and replay certificate remains a separate design problem.

One `runContext` mutex now protects the Lean state, replay actions, evidence indexes and upload sessions, event list, turn counter, provider error class, and both role APIs' mutable turn records.  The Lawyer and Council API condition variables use that mutex, so a successful action publishes its state and wakes both APIs without an intervening view of mixed state.  Lean transitions remain serialized under this mutex, with each call bounded by case cancellation, the earlier opportunity deadline when one applies, and the configured engine timeout.  Provider requests, condition and channel waits, HTTP response encoding, and final record output run after releasing the mutex; final output uses a deep snapshot captured under the mutex.

Role evidence reads reserve their count and requested byte allowance under the case mutex, open the stored regular file once after releasing the mutex, and verify its full size and digest while capturing the requested range from the same descriptor.  The runtime rebuilds a readable case-file body through the same one-descriptor verification.  An I/O failure or an opportunity that ends during the read restores the reservation, while Observer reads use no opportunity budget.  Successful Lawyer and Council range reads create events; Observer reads do not.  Successful role reads and nonterminal invalid attempts notify both role APIs, and each API derives `done` or `failed` directly from a terminal Lean state before the run loop publishes its final reason.

Role API errors now distinguish participant input from runtime failure with one internal error marker.  HTTP-envelope, identity, stale-turn, and deadline responses do not consume attempts; invalid tools and arguments on a resolved active turn consume one attempt; and unmarked execution, storage, integrity, publication, and event errors complete the turn without an attempt charge and return through the turn result.  Go validates strict Lawyer decision payload types and Council vote values before calling Lean, after which Engine errors, Lean rejection, and malformed accepted state are runtime failures.  Council request-spec conversion returns JSON encoding and decoding errors; initialization rejects an invalid seat before Lean state creation, and case-status generation reports the same error instead of omitting `request_spec`.

Observer tool validation uses the same HTTP error codes without changing a participant turn.  Observer evidence storage and read errors remain request-scoped because the Observer API has no run-level error channel.  Adding case termination for an Observer read failure requires a separate decision about whether a read-only client may stop an otherwise healthy proceeding.

The Lawyer and Council `/do` request types accept `call_id` but do not deduplicate mutations or retain completed results by identifier.  A lost mutation response therefore remains indeterminate to the caller.  Mutation idempotency remains future work.

The Lawyer `/do` request-body limit reserves `runtime.max_response_bytes` for the JSON envelope and metadata, then adds the standard-base64 length of `policy.max_direct_submitted_evidence_bytes`.  The derived limit lets a direct submission carry the maximum decoded evidence size without reducing its metadata budget.  The calculation returns a configuration error if the sum cannot fit in the `int64` limit accepted by `http.MaxBytesReader`; Council request limits remain unchanged.

`aar case-packet` opens each source while constructing the embedded manifest and confirms that the pathname and descriptor identify the same regular file.  During archive construction, it repeats that identity check, hashes and counts the exact bytes copied to each tar member, and compares both values with the embedded manifest.  A mismatch aborts publication and removes both temporary outputs.  Publication reserves both final paths through exclusive creation, so an existing packet or manifest remains unchanged.  If either rename fails, publication removes every reservation or published output that it owns and returns the publication and cleanup errors together.

`RecordIntegrity` combines catalog validity, ordered submitted-evidence history, final-state offered-evidence reference and size checks, and UTF-8 byte limits for report titles and summaries.  `SubmittedEvidenceHistoryValid` records each submission as a suffix whose origin and entry validation refer to the prior history, and its derived theorems prove unique submitted identifiers and disjointness from the catalog.  `MeritsOffersUsePriorRecord` checks one accepted merits action against its source-state record, while `MeritsOfferChronology` carries that property through every action in a successful replay.  The replay-time predicate establishes that an offer referred to the initial catalog or a submission already present when the filing occurred; the final-state predicate establishes referential closure against the complete record in one state.  `replaySteps_success_meritsOfferChronology` and `replayInitialized_success_meritsOfferChronology` derive chronology from successful replay, while `checkReplayCertificate_ok_meritsOfferChronology` derives it from certificate acceptance.  The closed and failed certificate fact structures now include exact opportunity authority, offer chronology, final-state record integrity, and exact equality between the claimed and initialized catalogs.

The final record-integrity proof checks used `./tmp/leanrun-aar` with Lean 4.32.0, `LEAN_NUM_THREADS=1`, `LEANRUN_JOBS=1`, 100 percent CPU, 4 GiB memory high, 6 GiB memory maximum, 1 GiB swap, 64 tasks, and a 900-second timeout.  The AAR Lake package passed `-j1` to each Lean compiler process through `weakLeanArgs`.  `./tmp/leanrun-aar lake --dir arb/engine build Proofs` built 43 jobs without diagnostics, and `./tmp/leanrun-aar lake --dir arb/engine build aarengine` built 4 jobs without diagnostics.  The rebuilt Lake executable and `arb/.bin/aarengine` had the same SHA-256 digest.

Focused Go verification covers exhaustion and deadline responses, bounded Lean calls, process-group cleanup, case and client cancellation, terminal status after an accepted closing vote, output-directory preservation, complaint identity and special-file scanning, single-descriptor evidence reads, source replacement, empty evidence, reservation rollback and concurrent budget enforcement, event-error publication, participant-input accounting, runtime-error propagation, council request-spec conversion, strict decision and vote validation, and direct Council retry boundaries.  Compile-only, focused, ordinary, and race-enabled proceeding tests passed, and the selected concurrency tests passed in twenty consecutive runs.  The complete runtime test, runtime vet, AAR command build, and diff check also passed.  Every Go command used two scheduler threads, a repository-local temporary directory, and a repository-local build cache.  The checks used the rebuilt local engine or bounded fake engines and HTTP handlers, made no provider call, and repeated the existing GOPATH/GOROOT equality warning.

## AARD Record Integrity

The AARD work ports the approved AAR authority, record-integrity, custody, transaction, synchronization, and bounded-engine rules while retaining AARD's numeric council answers and question-based case model.  Lean permits offered evidence, technical reports, and submitted evidence during surrebuttal, matching ARAP, the Go API, prompts, schemas, and tests.  The replay certificate uses `aard.replay-certificate.v1`, while the state remains `v1`, evidence identifiers retain `ev_<sha-prefix>_<slug>`, and `aar.evidence-manifest.v0` remains the shared AAR/AARD manifest format.

The existing `fail_opportunity` and failed-member representation remain unchanged.  Accepted actions remain committed when event recording fails, Observer storage errors remain request-scoped, and a stale `.aard-output-claim` requires operator cleanup.  These choices keep the AARD answer model and durable record identifiers stable while giving authority-bearing replay actions a distinct certificate version.

- [x] Bind every AARD action to the exact opportunity, state version, role, phase, and council member.
- [x] Add the immutable initial evidence catalog, submitted-evidence lineage, and filing-time offer chronology to Lean.
- [x] Prove initialization, step preservation, reachability, catalog equality, authority-conforming replay, chronology, and certificate facts.
- [x] Capture initial bytes before council sampling and Lean initialization, then unify direct and chunked evidence admission.
- [x] Replace overlapping role-API locks with one case mutex and preserve AARD answer and member-failure output.
- [x] Add strict request validation, participant/runtime error ownership, accepted-state checks, and bounded Lean processes.
- [x] Add output-directory ownership, verified case-packet publication, final snapshots, and focused race tests.
- [x] Generate AARD proof statistics, theorem references, verification notes, and the implementation guide.
- [x] Verify the proof root and engine through leanrunner, then run serialized Go, race, vet, command, packet, and certificate checks.
- [x] Reconcile the adjacent service's output-directory ownership with the exclusive core output claim, then repeat the paired AARD compatibility test.

Final Lean verification used `./tmp/leanrun-aar` with Lean 4.32.0, `LEAN_NUM_THREADS=1`, `LEANRUN_JOBS=1`, 100 percent CPU, 4 GiB `MemoryHigh`, 6 GiB `MemoryMax`, 1 GiB swap, `TasksMax=64`, and a 900-second timeout.  The narrow `RecordIntegrity.lean` check completed in 6.73 seconds, `lake --dir arbd/engine build Proofs` completed 14 jobs in 8.56 seconds, and `lake --dir arbd/engine build aardengine` completed four jobs in 4.76 seconds, all without diagnostics.  The Lake executable and installed `arbd/.bin/aardengine` have SHA-256 `f4f38c5b123d745799c44b172ff59f4a2fcdedf41d266d8cc335fec38436db0f`.

Final in-tree Go verification passed the complete `./arbd/runtime/...` test, the complete proceeding test with and without the race detector, runtime vet, the adapter process-control tests, focused record, transaction, error-classification, packet, and certificate tests, and twenty consecutive runs of the selected concurrency and publication set.  Every Go command used two scheduler threads and repository-local temporary and build-cache directories, while tests used bounded fake engines, HTTP handlers, or the rebuilt local engine and made no provider call.  Go repeated the existing equal-GOPATH-and-GOROOT warning, and the command build reported that it could not update a read-only shared module stat cache while still exiting zero.  The rebuilt `arbd/.bin/aard` has SHA-256 `0d8fb93369987876b838af32b4e46341b92d5065073da3d0aada4ad387d67438`.

The adjacent direct AAR and AARD services now leave each selected output directory to the core and store new child streams beneath `RegistryDir/logs/<case_id>/`.  Case identifiers are reserved across active direct cases, active Clerk cases, direct records, and persisted Clerk records before log creation, while unique temporary files publish direct service records without following a fixed temporary path.  Log creation uses confined directory descriptors, exclusive files, exact recorded paths, and identity-checked cleanup.  Every direct terminal consumer accepts a packet only when its case id, run id, top-level result, and final Lean case status agree, covering process exit, completed reads, result retrieval, detached reconciliation, and read-only proxy fallback.  The complete service tests, both direct-service race tests, twenty focused repetitions, service vet and builds, and paired AAR and AARD compatibility packages pass.  The `aar-run`, `aard-run`, and attested Clerk launchers still place their own logs within the directory passed to their core child, so their output-ownership change remains separate.

## Supervised AAR Calls

The AAR command accepts `--council-request-attempts` so a supervising service can own retry policy without multiplying provider calls inside one case attempt.  The default remains four attempts for existing callers, while a supervisor can select one.  Direct provider failures carry stable classes for transient, authentication, request, and response-protocol failures, which removes the need for consumers to infer severity from error text.

The AAR command also accepts `--required-votes` together with its existing council-size and evidence-standard overrides.  A supervising service can therefore apply one common council configuration to AAR and quick adjudication without creating a temporary policy file.  The runtime applies all three overrides before policy validation and council sampling.

`run.json` and the command summary contain the shared `provider` object.  It counts logical council calls, including preflight and voting requests, and separately counts responses that supplied usage or cost.  A failed provider call can incur a charge that neither response source returns, so observed cost can understate provider billing after a failed request.

Verification covered the complete ARB Go test suite, the root and ARB Go vet suites, and the ARB command and Lean engine build.  Focused tests check the default and one-attempt settings, provider error classification, OpenRouter cost parsing, and command-summary fields.  The verification made no provider calls.

Council preflight preserves the provider error in `CouncilPreflightError`, whose `Unwrap` method carries the typed cause through `proceeding.Run` and the command summary.  Replacement records retain the cause string for durable output.  Run cancellation returns the context error before council-candidate handling.

An authentication failure ends council preflight after its first candidate because another model cannot repair the credential.  Request, protocol, and transient failures can remain candidate-specific, so preflight can replace those candidates.  The failed-preflight tests check both the returned class and the number of candidate checks.

A provider failure during deliberation removes the affected council member, and the engine can close the case with `no_majority` while the result status remains `ok`.  The terminal result therefore carries the last council provider class independently of the structured case-failure object.  Each removal event retains its own class, and nonzero request-attempt overrides outside the documented range fail before case setup.

Follow-up verification ran `go test ./...`, `go vet ./...`, `go build ./...`, the ARB engine and proof build through Lean 4.32.0, the ARB paired compatibility tests against `adjservices` at `cb9caac`, and the relative Markdown-link check.  Every check passed.  The verification made no external provider call.

The default council pool stores endpoint capability metadata inside each record's `variant` object.  Request-spec parsing now copies the selected nested metadata, and direct council preflight excludes a pinned endpoint when its recorded `supported_parameters` list omits `tools`.  Capability metadata that is absent or malformed remains unknown and proceeds to the existing provider availability check.

Verification ran the focused model-request and proceeding tests, `go test ./...`, `go vet ./...`, `go build ./...`, and the ARB paired compatibility suite against `adjservices` at `cb9caac`.  Every check passed, and the affected documentation links resolve inside the repository.  The verification made no external provider call.

## Case Discovery

Every durable one-case run writes `case-manifest.json` through `common/casemanifest`.  ADC, ARB, and AARD write the identity record when the run starts and replace it with the bound private API address after the listener starts.  The shared package validates required fields and returns atomic-write and cleanup errors to the procedure command.

- [x] Add the versioned manifest schema and atomic writer.
- [x] Add manifest creation to all five procedures.
- [x] Document the paired discovery interface in `adj` and `adjservices`.
- [x] Repeat complete core and paired-interface verification.

## Simple Adjudication

The `simple` procedure decides one proposition through one direct provider request and requires a structured `demonstrated` or `not_demonstrated` response with a rationale.  The caller supplies an evidence standard, explicit document limits, one request specification, and explicit permission to use API-key billing.  The decision uses the proposition, relevant established knowledge, and any supplied documents, so an empty document set does not determine the result.

`common/documents` imports regular files in bytewise path order, rejects symbolic links and source changes, and records byte counts, media types, and SHA-256 hashes.  Its verified reader confines each path to the imported root and checks type, size, and digest before returning the bytes to a procedure; simple now uses that reader instead of maintaining a second verifier.  `common/modelinput` constructs the provider content item used by both simple and quick for UTF-8 text, `image/*`, and PDF documents.  Request-spec metadata does not describe model input modalities, so local media acceptance does not predict whether a provider or model will accept an encoded image or PDF.  The replacement test creates a distinct file before renaming it over the source, avoiding a filesystem-dependent assumption that immediate removal and recreation receive different inode numbers.  `common/recordio` supplies the JSON, atomic JSON, and append-only JSON-line operations used by the new procedures, while the simple record excludes document contents, encoded media, and request-header values from `model-request.json`.

The shared provider response retains input, cached-input, output, reasoning, and total token counts from the completed Responses payload.  It reads OpenRouter's inline `usage.cost` before considering the generation-metadata endpoint, which can return 404 while OpenRouter indexes a completed generation.  The common accounting object records request count, observed-value counts, and sums, allowing every procedure to retain partial usage and cost observations without representing an unknown value as zero.

Focused tests cover document import, record replacement, command argument handling, input-media construction, one-request success, typed provider failure, malformed tool output, and failure records.  Race tests, Go vet, and a command build also passed.  Verification used fake provider clients and made no external provider calls.

A live OpenRouter simple case first returned 831 tokens and $0.0001018248 in its completed response, but the old client ignored both fields and wrote zero after an immediate generation-metadata request returned HTTP 404.  After correction, the same proposition completed in 2.6 seconds and recorded 385 input, 164 output, 53 reasoning, and 549 total tokens together with $0.0000578956.  The second run made no generation-metadata request because the completed response supplied its cost.

A final OpenAI GPT-5 mini case used the committed code and one 189-byte text document.  It completed in 9.8 seconds and recorded 204 input, 597 output, 448 reasoning, and 801 total tokens in the response, terminal result, and provider-response event.  OpenAI supplied no monetary cost, and each record omitted the cost field.

A rebuilt OpenRouter case decided “A square has four sides” without documents after the established-knowledge prompt clarification.  It completed in 6.0 seconds, returned `demonstrated`, and recorded 407 input, 236 output, 110 reasoning, and 643 total tokens.  The completed response supplied a cost of $0.000067683.

## Quick Adjudication

The `quick` procedure gives a proponent and an opponent one sequential argument each, then asks a selected council to vote through direct provider requests.  The opponent receives the proponent's argument, and neither lawyer receives a rebuttal or closing opportunity.  The procedure requires a strict-majority threshold, an evidence standard, explicit document limits, and explicit permission to use API-key billing.

The core samples council records without replacement through `crypto/rand`, giving every remaining pool record equal probability at each seat.  A selected record cannot occupy a second seat in the same case, and selecting the full pool includes every record once.  The sampler accepts an internal random-index function so tests can fix the selection sequence and return entropy errors without replacing production randomness.

The private lawyer API uses the AAR lawyer paths and tool shapes so `adjservices` can present either procedure through the same MCP adapter.  Each lawyer can inspect immutable staged documents through list, stat, and range-read operations, and the shared verified reader rejects a changed file before returning its bytes.  The council request uses the same `common/modelinput` encoding as simple after the two accepted arguments; another binary media type fails during initialization.  Each selected member must submit one structured `demonstrated` or `not_demonstrated` vote with a rationale.

Production startup initializes every distinct selected council endpoint before the case API opens, which detects a missing credential or unsupported endpoint without sending a provider request.  Each vote records available provider token usage and cost, while the terminal result sums observed values and records their counts beside the total request count.  The durable record also contains resolved input, runtime identity, document metadata, ordered events, private lawyer work notes, both arguments, provider response identifiers, and the terminal result.

Request headers remain in memory and do not enter council metadata or other durable records.  Model references containing a query or fragment fail with a generic validation error that does not reproduce the rejected value.  A provider-metadata retrieval error remains attached to its vote when the completed response supplies no inline cost.  Callers can therefore distinguish an unknown cost from a zero cost and inspect the retrieval failure without searching process logs.

Council requests run sequentially by default.  The optional parallel mode starts the selected requests together, cancels outstanding request contexts after the first observed failure, waits for every started request to return, and records successful votes in roster order.  `input.json` and the `council_started` event record the resolved mode.

Focused tests cover lawyer-turn deadlines, cancellation, concurrent submissions, durable event order, document verification, majority results, complete pool validation before sampling, endpoint tool support, explicit billing authorization, typed protocol errors, error redaction, terminal shutdown and response-write failures, complete early command results, and short output writes.  Council-mode tests verify sequential execution by default, concurrent starts, roster-order persistence after reverse-order completion, and outstanding-request cancellation after a provider failure.  Focused ordinary and race tests, Go vet, a command build, repeated concurrency tests, and the diff check passed; verification used fake provider clients and local HTTP calls and made no external provider request.

ADC, ARB, AARD, and quick health responses now return HTTP 200 JSON with the process's case and run identifiers.  A supervisor or catalog can compare those values with the expected invocation instead of treating any successful response at a reused address as proof of identity.  Focused tests check the exact response fields for all four private APIs.

A live quick case imported one 189-byte text document, and both Codex lawyers used the stat and range-read operations before filing.  The sequential one-member council recorded 909 input, 439 output, 279 reasoning, and 1,348 total tokens together with $0.0001426026.  The complete service run took 72.5 seconds and returned `not_demonstrated`, consistent with the document naming Valve K-17 rather than the proposition's east pump.

## ADC Proposition Input

`adc case` accepts exactly one complaint or proposition.  Proposition setup creates `Proponent v. Opponent` in the programmatic Proposition Tribunal, assigns the burden to the Proponent, and requests a declaration whether the proposition has been demonstrated.  The Tribunal accepts `proposition_adjudication` jurisdiction without subject-matter screening, and `--trial-mode=auto` selects a jury.

The caller selects either `preponderance_of_the_evidence` or `clear_and_convincing` because the Lean engine accepts those two claim standards.  Setup imports an optional document tree through `common/documents`, records the manifest, verifies each imported file before scenario construction, and preserves the manifest size and SHA-256 values in the case attachments.  The runtime verifies the formal attachment path, regular-file type, exact size, and SHA-256 before it exports evidence, returns bytes through the role API, or adds file contents to a model prompt.  Positive count, per-file, and total-byte limits remain required when the document tree is absent.

Proposition setup constructs the normalized claim, both role strategies, and the scenario without an intake or planner model request.  The existing ADC runtime then handles pleadings, discovery, trial, verdict, and judgment with the selected direct or external roles.  Focused tests cover the Tribunal profile, deterministic case fields, accepted standards, document import and hash preservation, changed-document rejection, size drift, symbolic-link replacement, role API reads, model-prompt attachments, the proposition input flags, empty document records, and the jury default.

ADC records logical requests made by its direct provider clients, including digest generation, and sums every observed usage and cost value.  The runner writes its initial aggregate with the terminal procedure result, then the command refreshes `run.json` and the SQLite evidence result after generating the digest.  The refresh returns both file and database errors and runs before the command reports success.

Proposition claims set `declaratory_only` to true, while an absent field defaults to false for existing civil claims.  The Lean engine rejects a positive damages amount at juror voting, monetary judgment, Rule 68, settlement, and final jury or bench disposition boundaries.  Focused race tests, Go vet, and the ADC engine and maintained proof builds passed with Lean 4.32.0 through the local resource-limited runner.

The Lean case state records a declaratory `resolution` as `pending`, `demonstrated`, `not_demonstrated`, or `no_decision`.  Jury and default judgments derive a merits resolution from the winning party and the claim's burden holder, while a bench opinion supplies a required structured winner.  Hung juries, settlements, accepted offers, and procedural dismissals record `no_decision`, and ordinary civil claims retain `pending` because these values describe proposition adjudication.

Verification built `adcengine` and the maintained `Proofs` target with Lean 4.32.0 through the local resource-limited runner.  The complete ADC runtime race tests, ADC runtime vet checks, command build, and diff check passed.  The timeout integration tests now execute the prebuilt engine binary, preserving the required runner boundary for every Lean build.

## Documentation

The repository overview now distinguishes the five core procedures from the three formal procedures that use Lean and replay certificates.  The core process guide covers the simple and quick executables, private APIs, discovery manifests, records, and service tests beside the ADC, ARB, and AARD interface.  The shared-package, proof, simple, and quick references use the same procedure names and record descriptions.

Documentation checks validated every fenced JSON block and relative Markdown link, with the two files generated by the ADC signing example treated as generated inputs.  The simple and quick help output matches the documented flags, and the common, simple, and quick Go tests pass.  The socket-dependent quick tests used the checkout-local test wrapper because the default sandbox rejects loopback listeners.

## Verification

Verification begins by building each formal procedure's engine and proof targets with Lean 4.32.0 through the configured resource-limited runner.  The complete Go suite runs after the engine builds because an ADC integration test executes the engine.  Go verification also includes `go vet ./...` and builds of all five command packages, while documentation verification checks every relative Markdown link against the repository tree.

The current verification pass built `Main`, `Proofs`, and the executable target for ADC, ARB, and AARD with Lean 4.32.0.  The local runner used a 900-second timeout, 4 GiB memory high, 6 GiB memory maximum, 1 GiB swap, 100% CPU, and one job slot; it reported successful completion without persistent job identifiers.  The complete Go tests, vet checks, package builds, repeated document-replacement tests, and relative-link checks passed.

The cleanup pass ran the complete Go suite, full Go vet and package builds, and race tests for the changed common, simple, quick, ADC, ARB, and AARD packages.  Documentation checks parsed every fenced JSON block and checked relative links, excluding the generated ADC signature files and the literal complaint-template link placeholder.  The pass used fake provider clients and made no provider call.  No Lean source changed, so the previously verified Lean 4.32.0 engine and proof builds were not repeated.

The post-review correction pass ran the complete Go suite, full Go vet and package builds, and focused race tests for `common/modelinput`, simple request construction, and quick adjudication.  Documentation checks parsed every fenced JSON block and resolved every relative link in both repositories.  Verification made no provider call, and no Lean source changed.

- [x] Run the complete Go test suite.
- [x] Run the complete Go vet suite.
- [x] Build the ADC, ARB, AARD, simple, and quick commands.
- [x] Build the three Lean engines and proof trees.
- [x] Run the paired `adjservices` interface tests.
- [x] Verify relative Markdown links.
- [x] Repeat the builds and paired tests from clean Git archives.
