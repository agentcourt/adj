# Development Notes

## Repository Scope

This repository owns complete one-case execution for ADC, ARB, AARD, simple, and quick.  It contains the unified `adjudicate` command, the formal `adc-run`, `aar-run`, and `aard-run` commands, local participant launchers, MCP adapters, prompt catalogs, retained participant state, and native case records.  ADC, ARB, and AARD also include their Lean engines and proofs.

The shared `common/` tree contains code required by more than one procedure.  The `runtime/` tree contains common one-case supervision and procedure adapters, while `internal/` contains launcher implementation that no external Go package imports.  New shared packages must have at least two current consumers and a narrower API than the code they replace.

The optional `adjservices` repository manages multiple cases, deployment, attestation, artifact publication, reporting, and web applications.  Its service programs execute installed `adj` commands and consume documented private HTTP and artifact interfaces.  No `adj` package imports `adjservices`.

## Error and Command Policy

Commands return every input, output, formatting, storage, HTTP, and shutdown error to their caller.  A command writes diagnostics to standard error, writes machine-readable results to standard output where its manual specifies them, and exits nonzero when command execution fails.  Every command rejects unexpected positional arguments and reports help-output failures.

Long-running case commands derive their execution context from interrupt signals.  Startup output failures cancel the case before the command returns.  Case APIs and runtime helpers preserve the original operation error together with any cleanup error.

## Interface Maintenance

The [core process interface](docs/service-interface.md) defines the executable, private HTTP, and durable-record behavior consumed by `adjservices`.  An interface edit requires corresponding test and documentation edits in both repositories.  The paired interface tests use explicit core binaries and an explicit core checkout.

## AAR Proof Strengthening

The `aar-proof-strengthening` branch established the AAR authority, custody, replay, and record-integrity model and then applied it to AARD.  AAR binds every state-changing action to the exact current opportunity and proves record integrity, evidence-catalog preservation, and filing-time offer chronology as implications of the Lean certificate checker.  AARD now carries the corresponding authority, catalog, lineage, chronology, runtime transaction, and certificate facts while retaining its numeric answer model.  The remaining formal agenda includes ADC adaptation, constructive terminal-run existence, outcome stability, council symmetry, broader certificate consequences, and independent count-rule properties.  The [AAR record-integrity and runtime update](arb/docs/update.md) records the completed AAR and AARD designs and the pending ADC decisions.

- [x] Bind actions to the current opportunity id, state version, role, phase, and council member.
- [x] Make closed and failed states reject state-changing actions.
- [x] Bind evidence commitments, submission provenance, and technical-report byte limits into Lean and the runtime.
- [ ] Prove that every valid initialization admits a terminal run within the existing bound.
- [ ] Prove that a reached substantive threshold determines every terminal continuation.
- [ ] Prove whole-run council renaming and independent-vote order invariance.
- [ ] Derive authorization, global invariants, due process, and outcome soundness from terminal certificate verification.
- [ ] Replace the count-rule characterization assumptions with independent rule properties.
- [x] Reconcile the proof root, theorem index, and verification documentation.

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

The five procedures are simple, quick, AARD (`aard`, with service package `arbd`), AAR (`aar`, with service package `arb`), and ADC (`adc`).  The adjacent direct AAR and AARD services leave each selected output directory to the core and store new child streams beneath `RegistryDir/logs/<case_id>/`.  The service reserves case identifiers across active direct cases, active Clerk cases, direct records, and persisted Clerk records before log creation, while unique temporary files publish direct service records without following a fixed temporary path.  Log creation uses confined directory descriptors, exclusive files, exact registry paths, and identity-checked cleanup.  Every direct AAR or AARD terminal consumer accepts a packet only when its case id, run id, top-level result, and final Lean case status agree, covering process exit, completed reads, result retrieval, detached reconciliation, and read-only proxy fallback.

The `aar-run`, `aard-run`, and `adc-run` launchers own an outer service run directory and pass a dedicated `aar-output`, `aard-output`, or `adc-output` child to the core, while simple and quick retain their supervised paths.  The launchers require empty core and log children, reject symbolic-link substitutions, and reject a child whose directory identity changes during validation.  Local Clerk paths and the unified `adjudicate` command's AAR, AARD, and ADC adapters use the same separation.  Formal-procedure readers resolve core results only beneath the named core child, so an outer-root `run.json` has no core-result meaning.  Each container-backed participant receives a private runtime cidfile, and cleanup accepts only a complete 64-character lowercase hexadecimal container identifier from that file.  Cleanup removes that exact identifier; if the runtime never records one, the launcher terminates and reaps the client and retries without removing a container by its requested name.

For a manual role, the launcher writes a mode-`0600` remote-lawyer skill containing that role's MCP capability and removes it during cleanup.  Attested execution archives the service summary, component logs, and nested core record and excludes Pi homes, staged Codex homes, and remote-lawyer skills.  A verified attested AAR or AARD result must come from the exact nested success path, match the Clerk case and run identifiers, and pair `ok` with `closed` or `failed` with `failed`.  A verified attested ADC result must come from `adc-run/adc-output`, carry a valid ADC case manifest with matching case and run identifiers, and match `final_state.case.case_id` with case status `judgment_entered` or `closed`.  A partial attested archive remains available for diagnostic reads and cannot complete a detached case.  The service reporter descends past outer `local-run.json`, `service-case.json`, `clerk.json`, and live `events.ndjson` files to the named core child.  When no nested core marker exists, it reports the outer launcher or service failure or incomplete state instead of interpreting an outer record as a core result.

The S3 output prefix and expected workload image identity still require attestation protocol decisions.  The S3 publisher requires a fresh output prefix because no atomic owner record binds the prefix to a case and run before artifact publication.  A future protocol could claim an owner object through a conditional write or define another atomic prefix-ownership operation; selecting either operation requires approval.  The attested manifest records the container image identifier and image-tar SHA-384 supplied by the workload, while verification has no independently supplied expected values for either field.  A future protocol could carry explicit expected values or resolve them through a trusted versioned binding; selecting either binding requires approval.

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

Simple now makes provider-hosted web search available by default and accepts `--web-search=false` for a run without hosted search.  The request uses the current Responses API `web_search` tool documented in the [OpenAI web-search guide](https://developers.openai.com/api/docs/guides/tools-web-search), requests `web_search_call.action.sources`, and permits the model to choose whether to search.  OpenRouter accepts the same tool declaration through its [online model interface](https://openrouter.ai/docs/guides/routing/model-variants/online), so each endpoint uses its existing credential.

The shared client moved from `openai-go` v1.12.0 to `openai-go/v3` v3.52.0 because v1 serializes only the legacy `web_search_preview` tool.  The v3 module requires Go 1.25.0 and raises the selected `github.com/tidwall/gjson` and `golang.org/x/sys` versions to 1.19.0 and 0.47.0.  Simple uses the selected endpoint's hosted search service and existing credential.

`simple.runtime.v2` records the resolved search setting, and `simple.model-request.v2` records the effective tool list.  `simple.model-response.v2` normalizes search actions, queries, source URLs, and URL citations with titles and text offsets while preserving the raw response.  `simple.run.v2` and `simple.state.v2` record whether search was enabled and count calls, sources, and citations; built-in search calls do not change the requirement for exactly one `submit_simple_decision` function call.

Verification ran the shared OpenAI and Simple focused tests, the complete Go test suite, focused Go vet, and `go build ./...`.  Every verification command passed after source formatting.  Tests used provider fixtures and made no external provider request.

Focused tests cover document import, record replacement, command argument handling, input-media construction, one-request success, typed provider failure, malformed tool output, and failure records.  Race tests, Go vet, and a command build also passed.  Verification used fake provider clients and made no external provider calls.

A live OpenRouter simple case first returned 831 tokens and $0.0001018248 in its completed response, but the old client ignored both fields and wrote zero after an immediate generation-metadata request returned HTTP 404.  After correction, the same proposition completed in 2.6 seconds and recorded 385 input, 164 output, 53 reasoning, and 549 total tokens together with $0.0000578956.  The second run made no generation-metadata request because the completed response supplied its cost.

A final OpenAI GPT-5 mini case used the committed code and one 189-byte text document.  It completed in 9.8 seconds and recorded 204 input, 597 output, 448 reasoning, and 801 total tokens in the response, terminal result, and provider-response event.  OpenAI supplied no monetary cost, and each record omitted the cost field.

A rebuilt OpenRouter case decided “A square has four sides” without documents after the established-knowledge prompt clarification.  It completed in 6.0 seconds, returned `demonstrated`, and recorded 407 input, 236 output, 110 reasoning, and 643 total tokens.  The completed response supplied a cost of $0.000067683.

Simple now accepts an optional reasoning effort, output-token cap, and built-in-tool-call cap through its Go options and command line.  The request specification can provide each value, an explicit command-line option takes precedence, and the existing 4096-token cap remains the output fallback.  An omitted tool-call cap leaves the provider default in effect.  The [Responses API reference](https://developers.openai.com/api/reference/cli/resources/responses/methods/create) defines the reasoning request object and `max_tool_calls`, while current [model guidance](https://developers.openai.com/api/docs/guides/latest-model) shows that each model supports a subset of the accepted effort values; the provider rejects a valid value that the selected model does not support.

The OpenAI Responses request maps the resolved effort to `reasoning.effort` and a positive tool-call cap to `max_tool_calls`.  Runtime, request, state, run, event, transcript, and digest records preserve the requested effort, effective output-token cap, and tool-call cap.  An empty recorded effort or zero recorded tool-call cap means that the request omitted that field and retained the provider default.

Focused ordinary tests, race tests, and Go vet passed for the model-request, OpenAI client, Simple proceeding, and Simple command packages.  The complete Go test suite also passed.  The first package build omitted `-o`, so Go refused to replace the repository's `simple/` directory with a binary; the repeated build used an explicit temporary output file and passed.

- [x] Run focused model-request, OpenAI client, Simple proceeding, and Simple command tests.
- [x] Run focused race tests and Go vet.
- [x] Build the Simple command.

## Quick Adjudication

The `quick` procedure gives a proponent and an opponent one sequential argument each, then asks a selected council to vote through direct provider requests.  The opponent receives the proponent's argument, and neither lawyer receives a rebuttal or closing opportunity.  The procedure requires a strict-majority threshold, an evidence standard, explicit document limits, and explicit permission to use API-key billing.

Simple and Quick retain concise compiled prompt defaults and look for conventional prompt files under the process working directory.  Explicit prompt-file flags take precedence, while a missing conventional file selects the compiled default and a missing explicit file fails the run.  Prompt rendering uses literal token replacement and rejects empty templates and unrecognized `{{...}}` tokens; focused tests cover fallback loading, file overrides, token values, and validation without provider calls.

Quick assembles each role prompt from shared lawyer guidance, role-specific instructions, and the selected search instructions.  `--lawyer-web-search` defaults to true, and `adj.quick.input.v2` records the resolved value as `lawyer_web_search_enabled`.  The council prompt remains separate and receives the closed case record.

The core excludes a council record when its endpoint metadata states that it cannot accept the `tools` parameter required by the vote request.  It samples the remaining records without replacement through `crypto/rand`, giving every eligible record equal probability at each seat.  A selected record cannot occupy a second seat in the same case.  The sampler accepts an internal random-index function so tests can fix the selection sequence and return entropy errors without replacing production randomness.

Quick resolves its council pool in the same order as ARB: an explicit pool path, `./pool.jsonl` when present, then `<common-root>/data/personas/pool.jsonl`.  The command accepts `--common-root`, and library callers can set `Options.CommonRoot`.  The resolved pool path remains in `input.json`.

Live default-pool tests exposed three failure classes before completion: endpoint metadata omitted `tools`; a pinned model endpoint returned 404; and an available model returned prose instead of the vote tool.  Quick now excludes candidates known to lack tool support, shuffles the eligible pool, and requires a valid `submit_council_vote` call in a bounded availability request before assigning a seat.  Candidate-specific failures, including authentication failures returned by a request, advance to another record because request headers can differ.  A missing credential detected while initializing a provider client excludes the remaining candidates for that endpoint while leaving other endpoint types eligible.  Rejection events identify the complete safe route for failed and replacement records, provider accounting includes the preflight requests, and the durable roster records endpoint-variant and provider-routing fields without request headers.  The final live case at `/tmp/quick-pool-default-e2e.S5uuRX/record-live-8` omitted `--council-pool`, recorded the shared pool path, rejected six candidates, seated five endpoint variants, collected five votes, and closed with 16 provider requests and $0.01017181 in observed cost.

The private lawyer API uses the AAR lawyer paths and tool shapes so `quick-mcp` can expose the same case-operation pattern through Quick's procedure-specific adapter.  Every lawyer API route requires a separate bearer token, while `/health` remains available to the process supervisor.  The core and adapter read the token from owner-only regular files and omit its contents from process arguments, prompts, role capabilities, work directories, logs, and records.  Each lawyer can inspect immutable staged documents through list, stat, and range-read operations, and the shared verified reader rejects a changed file before returning its bytes.  The council request uses the same `common/modelinput` encoding as simple after the two accepted arguments; another binary media type fails during initialization.  Each selected member must submit one structured `demonstrated` or `not_demonstrated` vote with a rationale.

Production startup initializes every distinct selected council endpoint before the case API opens, which detects a missing credential or unsupported endpoint without sending a provider request.  Each vote records available provider token usage and cost, while the terminal result sums observed values and records their counts beside the total request count.  The durable record also contains resolved input, runtime identity, document metadata, ordered events, private lawyer work notes, both arguments, provider response identifiers, and the terminal result.

Request headers remain in memory and do not enter council metadata or other durable records.  Model references containing a query or fragment fail with a generic validation error that does not reproduce the rejected value.  A provider-metadata retrieval error remains attached to its vote when the completed response supplies no inline cost.  Callers can therefore distinguish an unknown cost from a zero cost and inspect the retrieval failure without searching process logs.

Council requests run sequentially by default.  The optional parallel mode starts the selected requests together, cancels outstanding request contexts after the first observed failure, waits for every started request to return, and records successful votes in roster order.  `input.json` and the `council_started` event record the resolved mode.

Focused tests cover lawyer-turn deadlines, cancellation, concurrent submissions, durable event order, document verification, majority results, complete pool validation before sampling, endpoint tool support, explicit billing authorization, typed protocol errors, error redaction, terminal shutdown and response-write failures, complete early command results, and short output writes.  Council-mode tests verify sequential execution by default, concurrent starts, roster-order persistence after reverse-order completion, and outstanding-request cancellation after a provider failure.  Focused ordinary and race tests, Go vet, a command build, repeated concurrency tests, and the diff check passed; verification used fake provider clients and local HTTP calls and made no external provider request.

ADC, ARB, AARD, and quick health responses now return HTTP 200 JSON with the process's case and run identifiers.  A supervisor or catalog can compare those values with the expected invocation instead of treating any successful response at a reused address as proof of identity.  Focused tests check the exact response fields for all four private APIs.

A live quick case imported one 189-byte text document, and both Codex lawyers used the stat and range-read operations before filing.  The sequential one-member council recorded 909 input, 439 output, 279 reasoning, and 1,348 total tokens together with $0.0001426026.  The complete service run took 72.5 seconds and returned `not_demonstrated`, consistent with the document naming Valve K-17 rather than the proposition's east pump.

A seven-member live Quick run received malformed JSON in one council member's `submit_council_vote` arguments.  Quick already rendered a correction prompt and retained the preceding response identifier, but its default invalid-attempt limit of one ended the loop before a correction request could occur.  Quick now permits three invalid submissions by default, matching AAR, AARD, and ADC and giving the existing bounded loop two correction opportunities within the same turn timeout.  Focused tests cover recovery after malformed arguments and failure after all three attempts.

A later live Quick council response supplied `vote=demonstrated` while its rationale concluded that the proposition was not demonstrated.  The council system, submission, and tool-description prompts named both enum values without defining their relationship to the proposition, so the structured response was syntactically valid despite the semantic inversion.  The configurable prompts and compiled fallbacks now define `demonstrated` as satisfying the stated evidence standard for every required part of the proposition, define `not_demonstrated` as the converse, and require the rationale to support the selected vote.  The request-construction test checks that both the assembled council instructions and tool description contain this mapping.

A direct control request then sent the revised instructions to the same Relace Search model and BF16 provider route.  The control record contradicted its proposition, and the model returned `not_demonstrated` with a rationale that identified the contradiction and found the standard unsatisfied.  The request and raw response remain with the experiment records.  The focused Quick prompt and request tests and the complete Quick package suite passed after the edit.

Review of the shared Responses conversion found that function-tool descriptions were omitted when the generic tool maps became SDK parameters.  The prompt catalog therefore resolved these descriptions, but a provider never received them.  The converter now copies a nonempty description into the SDK function-tool parameter, and its JSON-serialization test checks the exact description beside the existing name, schema, strictness, and hosted-search fields.

A replacement live run later failed when a council endpoint passed availability preflight and then returned an upstream HTTP 429 during its substantive vote.  The shared provider client already classifies 429 responses as transient and implements bounded backoff, but Quick configured one provider attempt by default and therefore never used that recovery path.  Quick now defaults to three provider attempts per council response, allowing two retries for transient transport, rate-limit, and server failures while retaining the existing four-attempt maximum and per-member council deadline.  The resolved-options test checks the new default.

A later full council request received an intermittent OpenRouter 400 `invalid_prompt` from a pinned SiliconFlow route.  An exact replay of the complete request, including the same route, arguments, documents, tool schema, and pool request specification, succeeded and returned one valid vote, so the saved failure does not establish a malformed request.  Inspection found that Quick left substantive council output unbounded when the pool record omitted a limit, although its preflight uses 1,024 tokens and ARB and AARD apply a 4,096-token fallback to substantive council requests.  Quick now applies the same 4,096-token fallback while preserving an explicit pool limit across the initial request and any repair requests.  The lawyer guidance also reserves authenticity work for a concrete dispute or material reliability question and tells lawyers not to repeat file hashes or compute checksums otherwise; the default council document label omits its hash.

## ADC Proposition Input

`adc case` accepts exactly one complaint or proposition.  Proposition setup creates `Proponent v. Opponent` in the programmatic Proposition Tribunal, assigns the burden to the Proponent, and requests a declaration whether the proposition has been demonstrated.  The Tribunal accepts `proposition_adjudication` jurisdiction without subject-matter screening, and `--trial-mode=auto` selects a jury.

The caller selects either `preponderance_of_the_evidence` or `clear_and_convincing` because the Lean engine accepts those two claim standards.  Setup imports an optional document tree through `common/documents`, records the manifest, verifies each imported file before scenario construction, and preserves the manifest size and SHA-256 values in the case attachments.  The runtime verifies the formal attachment path, regular-file type, exact size, and SHA-256 before it exports evidence, returns bytes through the role API, or adds file contents to a model prompt.  Positive count, per-file, and total-byte limits remain required when the document tree is absent.

Proposition setup constructs the normalized claim, both role strategies, and the scenario without an intake or planner model request.  The existing ADC runtime then handles pleadings, discovery, trial, verdict, and judgment with the selected direct or external roles.  Focused tests cover the Tribunal profile, deterministic case fields, accepted standards, document import and hash preservation, changed-document rejection, size drift, symbolic-link replacement, role API reads, model-prompt attachments, the proposition input flags, empty document records, and the jury default.

ADC records logical requests made by its direct provider clients, including digest generation, and sums every observed usage and cost value.  The runner writes its initial aggregate with the terminal procedure result, then the command refreshes `run.json` and the SQLite evidence result after generating the digest.  The refresh returns both file and database errors and runs before the command reports success.

Proposition claims set `declaratory_only` to true, while an absent field defaults to false for existing civil claims.  The Lean engine rejects a positive damages amount at juror voting, monetary judgment, Rule 68, settlement, and final jury or bench disposition boundaries.  Focused race tests, Go vet, and the ADC engine and maintained proof builds passed with Lean 4.32.0 through the local resource-limited runner.

The Lean case state records a declaratory `resolution` as `pending`, `demonstrated`, `not_demonstrated`, or `no_decision`.  Jury and default judgments derive a merits resolution from the winning party and the claim's burden holder, while a bench opinion supplies a required structured winner.  Hung juries, settlements, accepted offers, and procedural dismissals record `no_decision`, and ordinary civil claims retain `pending` because these values describe proposition adjudication.

Verification built `adcengine` and the maintained `Proofs` target with Lean 4.32.0 through the local resource-limited runner.  The complete ADC runtime race tests, ADC runtime vet checks, command build, and diff check passed.  The timeout integration tests now execute the prebuilt engine binary, preserving the required runner boundary for every Lean build.

## Prompt Catalogs and MCP Adapters

All five procedures resolve model-facing instructions and tool descriptions through catalogs with stable IDs, relative paths, declared replacement tokens, and compiled fallbacks.  Resolution gives an individual file the highest precedence, followed by a complete prompt directory, the conventional process-working-directory path, and the fallback.  This design supports complete prompt revisions and small experiments without making repository-relative files a runtime dependency.

Rendering uses literal `{{TOKEN}}` replacement because prompt authors need substitution rather than a programming language.  Each loader validates its source before inserting runtime values, which preserves token-looking text in propositions, records, documents, and error messages.  Tool names, schema structure, validation rules, authority, record visibility, and protocol errors remain code-owned, while the catalogs own the instructions and descriptions shown to models.

Quick, AAR, AARD, and ADC now have procedure-specific MCP adapters in this repository.  Their `mcp.*` catalogs cover session instructions, wait-state guidance, and every exposed MCP tool description beneath the same procedure prompt directory used by the core.  The [prompt-authoring guide](docs/prompt-authoring.md) defines the catalogs and command-line interface, and each adapter can run beside its core without an `adjservices` process.

The first MCP interface used one server-wide bearer and selected the participant from URL query parameters.  Any bearer holder could therefore initialize a session for another participant.  The replacement capability binds the procedure audience, case, assignment type, and principal under an HMAC-SHA-256 signature, and the verified assignment supplies all participant identity.  A session stores that assignment and accepts later requests only from the same verified assignment.

All four MCP commands use signed assignment capabilities.  Each command provides `keygen`, `issue`, and `serve` modes; the server verifies the procedure audience and assignment on initialization and requires the same verified assignment for later session requests.  A capability uses HMAC-SHA-256 with a private key file, and identity no longer comes from MCP URL parameters.  A managed service child sets a zero session TTL to disable session expiry and removes its private key file after readiness; child exit ends capability validity because the key remains only in child and supervisor memory.  The formal procedure Makefiles build `aar-mcp`, `aard-mcp`, and `adc-mcp` beside their core commands.

The cross-repository compatibility vector uses key bytes `00` through `1f`, audience `quick`, case `case-alpha`, assignment type `lawyer`, and principal `plaintiff`.  Focused tests cover the fixed token, signature tampering, a wrong audience, identity-query rejection, a participant mismatch on POST and DELETE, exclusive key creation, command dispatch, and Quick capability issuance.  The complete Go test suite, focused race tests, focused vet checks, source formatting check, and diff check passed without a provider request.

AAR and AARD direct councils resolve malformed arguments, oversized responses, wrong call counts, wrong tools, and invalid arguments through five separate repair components before inserting the selected component into the repair wrapper.  Their Lawyer API catalogs also own the `tool.send_work_notes.property.notes` schema-property description.  ADC owns both proposition-strategy prompts, model-facing correction components, tool-result guidance, case-file attachment text, and the three `import_case_file` property descriptions.  AAR, AARD, and ADC insert those components into larger runtime prompts or schemas through literal replacement.

- [x] Give every core prompt a stable ID, checked-in conventional path, and compiled fallback.
- [x] Add literal source validation and partial or complete command-line overrides.
- [x] Add procedure-specific standalone MCP adapters and `mcp.*` prompt catalogs.
- [x] Document every catalog directly or through the authoritative procedure catalog.
- [x] Run the final focused prompt and MCP tests after all concurrent source edits finish.
- [x] Run the complete Go tests, vet checks, builds, and whitespace check.
- [x] Check relative Markdown links after the documentation edits finish.

## Documentation

The repository overview now distinguishes the five core procedures from the three formal procedures that use Lean and replay certificates.  The core process guide covers the simple and quick executables, private APIs, discovery manifests, records, and service tests beside the ADC, ARB, and AARD interface.  The shared-package, proof, simple, and quick references use the same procedure names and record descriptions.

Documentation checks validated every fenced JSON block and relative Markdown link, with the two files generated by the ADC signing example treated as generated inputs.  The simple and quick help output matches the documented flags, and the common, simple, and quick Go tests pass.  The socket-dependent quick tests used the checkout-local test wrapper because the default sandbox rejects loopback listeners.

## Verification

Verification begins by building each formal procedure's engine and proof targets with Lean 4.32.0 through the configured resource-limited runner.  The complete Go suite runs after the engine builds because an ADC integration test executes the engine.  Go verification also includes `go vet ./...` and builds of all five command packages, while documentation verification checks every relative Markdown link against the repository tree.

The current verification pass built `Main`, `Proofs`, and the executable target for ADC, ARB, and AARD with Lean 4.32.0.  The local runner used a 900-second timeout, 4 GiB memory high, 6 GiB memory maximum, 1 GiB swap, 100% CPU, and one job slot; it reported successful completion without persistent job identifiers.  The complete Go tests, vet checks, package builds, repeated document-replacement tests, and relative-link checks passed.

The cleanup pass ran the complete Go suite, full Go vet and package builds, and race tests for the changed common, simple, quick, ADC, ARB, and AARD packages.  Documentation checks parsed every fenced JSON block and checked relative links, excluding the generated ADC signature files and the literal complaint-template link placeholder.  The pass used fake provider clients and made no provider call.  No Lean source changed, so the previously verified Lean 4.32.0 engine and proof builds were not repeated.

The post-review correction pass ran the complete Go suite, full Go vet and package builds, and focused race tests for `common/modelinput`, simple request construction, and quick adjudication.  Documentation checks parsed every fenced JSON block and resolved every relative link in both repositories.  Verification made no provider call, and no Lean source changed.

Parallel repetition exposed `ETXTBSY` in ADC tests that directly executed a shell fixture immediately after writing it.  The fixture now runs as an input to `/bin/sh`, which preserves the tests of standard input, standard output, standard error, exit status, and command arguments while avoiding direct execution of a newly written file.  The failing test passed 100 consecutive repetitions after the edit, and the complete Go suite and focused vet checks passed.

The complete test command later exposed a termination-check race in `arbd/runtime/lean`.  Linux procfs can return `ESRCH` when a process exits while the test opens `/proc/PID/stat`, but the duplicated ARB and AARD helper recognized only `ENOENT` as completed termination.  Both helpers now recognize `ESRCH`, and the failing AARD test passed 20 consecutive runs after the correction.

- [x] Run the complete Go test suite.
- [x] Run the complete Go vet suite.
- [x] Build the ADC, ARB, AARD, simple, and quick commands.
- [x] Build the three Lean engines and proof trees.
- [x] Run the paired `adjservices` interface tests.
- [x] Verify relative Markdown links.
- [x] Repeat the builds and paired tests from clean Git archives.

## Quick legal-analysis framing

A live Quick lawyer using Claude Code with Claude Opus 4.8 read a legal case record and then received Anthropic's `cyber` refusal before it could file an argument.  The case asks the council to decide a proposition about conduct described in the evidence; it does not direct the lawyer to perform that conduct.  The shared lawyer prompt now identifies propositions, arguments, and case materials as claims and evidence for analysis and identifies references to conduct as case facts or allegations.  The instruction applies to every case and preserves the lawyer's access to search, local execution, installed tools, and evidentiary tests.

The checked-in shared prompt and its compiled fallback contain the same text.  A focused test compares them and verifies that both lawyer roles receive the framing.

A later Pi defendant received an Anthropic cyber refusal after web search returned research about conduct described in the case.  The earlier core framing stopped at the proposition, arguments, and case materials; research queries and tool results entered the conversation without an explicit legal-analysis label.  The shared lawyer and enabled-search prompts now identify queries and returned tool content as legal research, require source assessment, and direct both lawyers to use available tools when those tools can improve the analysis.

The checked-in shared-lawyer and enabled-search prompts match their compiled fallbacks.  The focused prompt test checks both pairs.  It also checks that the plaintiff and defendant receive the legal-research and enabled-search instructions in their assembled prompts.  `../verification/go-test -count=1 ./quick -run '^TestDefaultLawyerPromptsFrameLegalResearch$'` and `../verification/go-test -count=1 ./quick` passed.

- [x] Run the Quick package tests.
- [x] Run the revised Quick prompt test.
- [x] Run the complete Go tests, vet, build, and whitespace checks.
- [ ] Repeat the live Claude Opus 4.8 Quick condition from a fresh record directory.

The fresh Claude Opus 4.8 run completed both lawyer turns, but its sequential council stopped after C1 voted because C2 received HTTP 400 with code `invalid_prompt` and message `Invalid Responses API request`.  Quick had configured three provider attempts, but the shared client treated every HTTP 400 response as a permanent request failure and therefore sent C2 only one substantive request.  Candidate-check HTTP 429 responses and deadlines occurred before the case began; Quick rejected those candidates and seated replacements.

An exact reconstruction sent the same C2 model, pinned Novita route, accepted arguments, documents, tool schema, and output limit, and it returned a valid `not_demonstrated` vote.  A prior experiment produced the same HTTP 400 signature on a different OpenRouter model and provider, and its unchanged replay also succeeded.  OpenRouter identifies its [Responses API as beta](https://openrouter.ai/docs/api/reference/responses/basic-usage), which accords with the observed intermittent response but does not explain its internal cause.

The shared client now retries only the OpenRouter Responses error whose status, code, and message match the observed signature.  The existing provider-attempt limit and request deadline bound those retries, and exhaustion reports the error as `provider_transient`.  Other HTTP 400 responses retain their existing request-error behavior, and tests cover the exact retry, host restriction, response-path restriction, message restriction, code restriction, and final classification.

- [x] Replay the failed C2 request without changing its contents or provider route.
- [x] Test the OpenRouter `invalid_prompt` retry condition.
- [x] Repeat the complete Claude Opus 4.8 Quick run with the corrected client.

The fresh Quick run completed both direct Anthropic Claude Opus 4.8 lawyer turns and seven sequential council votes, resolving `not_demonstrated` by seven votes to zero.  Candidate checks replaced three stale routes that returned HTTP 404, while every seated council member completed its substantive request.  This run did not receive the intermittent `invalid_prompt` response, so the unchanged exact replay and the bounded transport tests provide the direct evidence for the new retry path.

A later Quick run completed both OpenClaw lawyer turns and recorded four `not_demonstrated` votes before C5 received the same OpenRouter `invalid_prompt` response on all three configured requests.  An unchanged replay later returned a valid `not_demonstrated` vote from the same model and pinned SiliconFlow route.  The request was valid, but Quick failed the entire case because one substantive council request failed after preflight.

Quick now applies ARB's council-member failure rule.  A provider failure, member deadline, or exhausted invalid-response limit produces a durable council-member failure and allows the other members to vote.  The original council size and required vote count remain fixed, and the procedure resolves after every seat has produced a vote or a failure.  Parent cancellation, prompt construction, document verification, record writing, and other procedure errors still fail the run.  The result and transcript schema versions advance to version 2 and include `council_failures`; each record contains the member identity, `failed` status, failure reason, message, provider error class when available, and failure time.

- [x] Test sequential member failure with a 4–2 verdict from the six completed votes.
- [x] Test a 3–3 split with one member failure and a `no_majority` result.
- [x] Test deadline, parent cancellation, and local procedure-error classification.
- [x] Test parallel member failure without sibling cancellation and procedure failure with sibling cancellation.
- [x] Test the terminal result, transcript, event, and lawyer API failure records.
- [x] Repeat the interrupted OpenClaw GPT-5.6 Quick condition with a fresh output directory.

Review found that a provider could return a successful response after parent cancellation or the member deadline and Quick could record that late vote.  Quick now checks the parent and member request contexts after every provider return, before returning a parsed vote, and before committing an outcome.  Timeout classification now matches ARB by recognizing request-context deadlines, wrapped deadline errors, `net.Error` timeouts, and standard timeout messages.  Tests cover success returned after parent cancellation, success returned after the member deadline, plain network timeouts, and provider-wrapped network timeouts.  The removal event also supplies ARB's `cause` field while retaining the structured failure record's `message` field.

## Canonical repository location

The canonical repository is `github.com/agentcourt/adj`.  The Go module declaration and every internal import use that path.  A tracked-file search found no remaining reference to the former module path.

`../verification/go-test -count=1 ./...`, `go vet -p=1 ./...`, and `go build -p=1 ./...` passed after the module-path change.  Go printed the existing warning that `GOPATH` and `GOROOT` both name `/home/somebody/go`.  Source formatting and `git diff --check` complete the repository verification.

## Evaluation-system restoration

The repository split deleted the behavior-eval and model-pool systems before the first `adj` commit.  The deletion also removed 63 evaluator tests and marked the planned assertion preservation complete without a corresponding migration.  The restoration source is old `adjudication` commit `dde9b3fe3b83e0139534da340cb17d5e4522f256` on `tidy`, the last maintained form of both systems.

The maintained layout separates actor behavior evaluation from provider and pool construction.  `evals/` contains ADC fixtures, prompt candidates, plans, and analyses, while `adc/runtime/eval/` contains the Lean-backed runners and scorers.  `model-pool/` contains provider inventory, model evaluation, deterministic scoring, embeddings, clustering, and pool sampling, and the runtime default remains `common/data/personas/pool.jsonl` until a generated replacement receives separate review.

Current ADC changed after the recovered evaluators were written.  The port uses the current prompt catalog and the same opportunity executor as an ADC case, including reference-tool calls, correction turns, Lean `apply_decision`, and Lean `step`.  Each result records every provider exchange, the turn log, final state, provider usage and cost accounting, and separate Lean and Step acceptance.  Provider, prompt, Lean-process, filesystem, and persistence errors abort a run, while a bounded invalid model turn produces a typed procedural failure that the scorer marks invalid.  The recovered `adc juror` and `adc llm` commands provide the pool-member and direct-request probes used beside the eval suites.  The eval adapter fixes the Lean opportunity limit at three steps and the isolated execution turn at one, matching every suite, and omits configuration and result fields with no production consumer.  The eval-level deterministic-action test covers the same path as the deleted runner-level duplicate; the focused runner and eval tests passed after these removals.

Rules 11, 37, and 58 provide deterministic actions in their production Lean opportunities.  Their default evals execute those actions without initializing a model provider, and each suite rejects a missing deterministic action.  The explicit `--counterfactual-model` option removes the action from a cloned opportunity for prompt research and records that execution mode in every result and summary.

ADC judge evals expose production execution only.  Model-backed suites initialize their configured provider, while Rules 11, 37, and 58 execute their Lean-provided deterministic action unless the caller selects counterfactual model mode.  The removed synthetic path comprised the command flags, output fields, canned gold responses, production scripted-response client, associated test fixtures, and documentation.

`go test ./adc/runtime/eval`, `go test ./adc/runtime/cli`, `go test ./adc/runtime/runner`, and `go test ./adc/...` passed after the removal.  A source search found no remaining synthetic-execution names in `adc/`, `evals/adc/`, or this journal, and `git diff --check` passed.

The model-pool end-to-end runner executes every stage in a new run directory and reports command starts and finishes on standard output.  It writes stage directories and one run summary.  Stage stops remain for bounded runs.

The endpoint-variant batch runner requires an absent or empty output directory and evaluates every input variant.  `variant_summary.csv` contains one terminal row per variant, while the event stream reports live progress.  Per-request, no-progress, and per-variant timeouts remain and terminate a timed-out child process.

The direct evaluator writes response rows to `raw_results.jsonl`, and the scorer writes the row scores and aggregates to `scores.json`.  The filter retains accepted endpoint rows, removed endpoint rows, its CSV view, and its summary.  The gene runner requires an absent or empty output directory and writes each result to `records.jsonl`; the end-to-end runner rejects any completion error, embedding error, missing record, or missing embedding before PCA.  Request timeouts and bounded retries remain.

Inventory requests now abort the run when any selected model's endpoint request fails.  Route IDs use `openrouter:<model>@<route>#<quantization>`, and inventory rejects collisions before writing normalized rows.  The checked-in filtered variants and default pool use those IDs consistently, including nested equivalent endpoints.  Inventory raw files use percent-encoded model IDs as filenames; response hashes were removed from normalized metadata.

Removed the unused pool samplers and retained `sample-tuple-pool.py`.  Removed duplicate aggregate JSON, filter copies, evaluator and scorer outputs, inventory Markdown, command and progress journals, gene manifests, and the result schema whose only object was the removed evaluator aggregate.

Python compilation and command help passed.  A live end-to-end inventory stage produced three endpoint variants and no run manifest.  A live one-variant batch completed one item with a score of `1.0`, and focused runs exercised both retained timeout kinds.  A live one-variant gene run completed one response and one embedding, wrote its record directly, and rejected the populated output directory on a second invocation.  The first child-process test had failed because the sandbox's default `uv` cache is read-only; setting `UV_CACHE_DIR` to a writable cache corrected that test environment.

After the output reduction, a current one-model inventory returned three readable route IDs and one percent-encoded raw endpoint filename.  A complete one-route live run evaluated one question, filtered the route, completed one gene response and embedding, and finished PCA, clustering, aggregation, and pool sampling.  The output tree contained only the retained stage files.  The focused Quick and shared model-request tests passed against the migrated default pool.

An implementation and user-documentation search found none of the removed continuation, temporary-output, PID-file, stop-file, or end-to-end manifest code.  `git diff --check` passed.  The development journals retain the removed feature names as the record of this cleanup.

Prompt catalog construction does not resolve the default court.  A supplied nonzero court profile is validated during renderer construction, while `JudgeRole` and Rule 12 schema construction resolve an omitted court and return lookup failures.  Juror and LLM probes can therefore load and override their catalog prompts from a working directory that has no ADC court asset.

The complete Go test suite, the race-enabled evaluator, runner, Lean, and CLI suites, `go vet -p=1 ./...`, `go build -p=1 ./...`, and `make -C adc prove` passed before the current cleanup.  All sixteen candidate prompts completed their full fixture sets, and the 30 hard voir-dire fixtures passed.  The deterministic-rule candidates used counterfactual execution.  All eight supported rescore paths passed without runtime initialization, and a prompt-catalog override appeared in the recorded model input.  A live one-fixture voir-dire run completed through OpenRouter with one accepted tool call, one accepted Lean decision, one accepted Step, and complete provider accounting.

- [x] Restore the maintained source trees and related probe commands.
- [x] Port evaluator prompts, schemas, command context, and prompt overrides.
- [x] Pass focused and complete Go verification.
- [x] Pass focused evaluator, CLI, runner, and rescore tests after removing synthetic execution.
- [ ] Pass model-pool validation, audit, mock scoring, and pipeline construction.
- [ ] Pass bounded live ADC and complete model-pool runs.
- [ ] Review the final file set, documentation, and generated ignored output.

## Evaluation documentation

The behavior-eval documentation describes the ten registered ADC judge suites, their checked-in fixtures and prompt templates, the production opportunity runner, deterministic execution for Rules 11, 37, and 58, scoring, and generated outputs.  The rule-specific documents state their fixture distributions, state construction, payload requirements, summary fields, execution modes, rescore support, and limits from the Go implementation and JSONL fixtures.  The Rule 58 candidate identifies the fixture context as the source of `claim_id` and `basis` and the tool schema as the source of their requirement.  The model-pool operator documents describe the inventory, evaluation, filtering, gene inference, PCA, clustering, aggregation, and sampling commands and their retained files.  They also record the persona-path mismatch between generated pools under `results/` and the runtime loaders.

A one-fixture Rule 11 command completed through the ADC engine with the production deterministic action, Lean acceptance, step acceptance, and a correct score.  The focused evaluator and CLI Go tests passed, and the model-pool repository audit and both question-set validations reported no issues.  A mock Core20 evaluation wrote and scored 60 rows with a deliberation score of 1.0 and no operational errors.  Relative Markdown links, prose structure, terminology, and whitespace checks passed across the edited operator documents.

- [x] Check every ADC judge suite document against its runner and fixtures.
- [x] Check the model-pool operator documents against the command implementations.
- [x] Run one ADC eval through the production execution path.
- [x] Run the focused Go tests and local model-pool validation commands.
- [x] Verify local links, prose structure, terminology, and whitespace.

## ADC payload validation and model-pool accounting

The runner validates the required Rule 60 `granted` Boolean after applying opportunity payload defaults and before calling `ApplyDecision`.  The shared payload boundary covers internal model turns and external role submissions.  Fresh scoring and rescoring both derive `Granted` from the preserved tool payload and classify a missing or non-Boolean value as `malformed_granted`.

The model-pool tool loop accumulates usage and cost across provider rounds.  A later provider failure carries completed-round metadata and tool traces into the written result row while retaining the original error for classification and OpenRouter error details.  The scorer treats either a positive `tool_error_count` or an error in a tool-trace result as a tool-call failure.

`go test ./adc/...`, `uv run python -m unittest discover -s tests -p 'test_*.py' -v`, and `git diff --check` passed.  The Go run covered the production payload boundary and Rule 60 rescore behavior.  The Python tests covered aggregate usage and cost, preservation after a later timeout, result-row persistence, and both metadata- and trace-based tool failures.

- [x] Reject malformed Rule 60 decisions before engine execution.
- [x] Validate preserved Rule 60 payloads during rescoring.
- [x] Preserve completed provider-round accounting and tool traces after a later failure.
- [x] Document and run the model-pool unit-test command.

## Standalone procedure execution

The `adj` repository owns complete one-case execution for Simple, Quick, ARB, ARBD, and ADC.  It contains the unified `adjudicate` command, the formal-procedure run commands, local lawyer launchers, launcher prompt catalogs, and the local Pi lawyer image recipe.  `adjservices` starts installed `adj` commands when a managed service needs a case and retains deployment, attestation, artifact, report, and web responsibilities.

The unified command guide uses the root `make build` target and names every procedure command, MCP command, and working directory in its settings example.  It records the location, permissions, active lifetime, and cleanup of each manual-lawyer skill, while the prompt guide distinguishes same-host loopback access from remote access through a public MCP base URL.  The AAR and AARD specifications assign managed case admission and public routing to `adjservices`, and the one-case runner documentation and command help use launcher terminology.

A live ARB run started both Claude lawyers from `adj`, issued their MCP capabilities, retained their working directories, and enabled native web search and local execution.  The plaintiff read all eleven exhibits, reconstructed their bytes, verified the supplied signature with OpenSSL, searched the web, downloaded a primary text, and sent two work-note updates before its opening.  The run exposed a fixed `/home/user/work-product` journal path in the court prompt even though the headless launcher assigned a different retained directory.  The ARB and ARBD standing prompts now place `work-product/case-notes.md` inside the workspace assigned by the launcher or external harness, and their compiled fallbacks carry the same instruction.

The selected council endpoint returned HTTP 503 maintenance responses from DigitalOcean during that run.  The request client applied its bounded retry policy.  The next live test will use an explicit council model to separate standalone-runner verification from provider-pool availability.

Active temporary-directory prefixes, helper environment variables, MCP bearer-token variable names, and the Codex usage-checkpoint schema now use the `adj` namespace.  Historic experiment outputs retain the namespace recorded when they ran.  Focused tests cover MCP-child startup, process helpers, participant configuration, resumed usage accounting, and unified runner behavior under the current names.

The complete Go test suite, build, vet, and whitespace checks pass.  The full service-side tests, builds, vet, dependency listing, and paired ADC, ARB, and AARD compatibility suites also pass against explicit binaries from this checkout.  These checks reused the existing formal engines and did not rebuild Lean.

- [x] Confirm retained lawyer files, local execution, web search, and incremental work notes in a live ARB run.
- [x] Remove the fixed journal path from ARB and ARBD prompts and fallbacks.
- [ ] Complete a fresh standalone ARB case with a working council provider.
- [x] Run the complete Go tests, builds, and whitespace checks for the repository split.

## ARBD lawyer profiles

ARBD resolves separate plaintiff and defendant profiles for OpenClaw, Pi, Codex, and Claude.  Unified settings use `plaintiff_profile` and `defendant_profile`, while `lawyer_profile` supplies any omitted automatic role.  The `aard-run` command exposes the same per-role runner, provider, model, command, reasoning, authentication, credential, API-key source, and resume settings.

Pi profiles keep the public OpenAI provider and `openai/gpt-5.6-sol` model names.  Subscription authentication reads Codex credentials and the shared agent runtime selects Pi's `openai-codex` provider internally.  Each role receives its own environment, retained work directory, state directory, authentication source, MCP assignment, and participant prompt.

- [x] Add independent ARBD plaintiff and defendant profiles.
- [x] Add ARBD headless and Pi launcher prompts with compiled fallbacks.
- [x] Test Pi subscription, role credential separation, settings resolution, command flags, and launcher prompt resolution.
- [x] Run the focused Go tests, vet checks, and `aard-run` build.

## Case record index

The cross-procedure case-record reader derives its docket from each procedure's retained files.  It accepts direct core records and the documented unified and formal-launcher directory layouts.  One JSON object contains a chronological logical docket and a catalog of regular files.  Sources identify the case root and retained participant state roots listed in the unified management result.

Procedure events supply ARB, ARBD, and ADC action times.  Quick arguments, Quick votes, Simple decisions, evidence metadata, and work notes carry their own times.  The case manifest supplies the initial filing time.  File modification time supplies the remaining timestamps because the portable Go file API does not expose file creation time.  Every item records the source of its timestamp.

The default access set contains the case record.  `--include-work-notes` adds work-note records.  `--include-sessions` adds process logs, formal-launcher state under `agents/`, and participant state directories recorded by the unified command.  The index excludes participant work directories, exported work product, and ADC strategy files under both options.  The reader skips symbolic links and copies hash metadata from existing document and evidence manifests.

- [x] Add the common case-record reader and JSON schema.
- [x] Add the `adjudicate case-record` command and external output-file handling.
- [x] Test Simple, Quick, ARB, ARBD, and ADC record extraction.
- [x] Run focused tests, vet, build, and retained Simple and Quick examples.
- [x] Document accepted layouts, selection rules, JSON fields, timestamp sources, and unavailable-session behavior.
- [ ] Resolve the existing ADC and ARB Pi-authentication test mismatch before the complete Go test command can pass.
