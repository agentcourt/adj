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

## Unified ARB complaint isolation

The unified ARB adapter writes its generated complaint to
`inputs/arb/complaint.md`.  AAR treats every sibling of a complaint as an
initial case file when the caller supplies no explicit `--file` option.
The former `inputs/arb-complaint.md` path therefore caused the unified
command's `adjudicate-request.json`, `resolved-settings.json`, and
`documents.json` control records to enter the case evidence when the
matter contained no imported document.  The dedicated directory
preserves AAR's automatic case-file behavior and contains no common
control record.  Imported documents continue through explicit case-file
paths.

## Unified ARB terminal cleanup

A complete ARB case can end while automatic lawyers still wait for another MCP
opportunity and after council containers have exited and removed themselves.
The local runner cancels those remaining clients after the core publishes its
result.  Participant finalization now skips terminal-usage parsing when the
runner canceled the participant, because the forced stop can leave an
incomplete OpenClaw JSON stream.  Cleanup, participant-record, and process-record
errors still return to the caller.  The council cleanup path accepts a missing
container ID for a runtime client that had already exited, while retaining the
missing-ID error when cleanup must kill a live client whose container ownership
was never recorded.

## OpenRouter Simple web search

OpenRouter's current [web-search server-tool
documentation](https://openrouter.ai/docs/guides/features/server-tools/web-search)
requires `{"type":"openrouter:web_search"}` for Responses requests.  It lists
GPT-4.1 among the models with native search and permits the server tool beside a
user-defined function tool.  The common Responses client previously sent
OpenAI's `{"type":"web_search"}` declaration to both endpoints, which
OpenRouter rejected with `400 invalid_prompt`.  Tool conversion now selects the
endpoint's documented declaration.  OpenAI requests retain the standard
`include` field for search-action sources; OpenRouter requests omit that
OpenAI-specific field and obtain citations through OpenRouter's response
annotations.

An isolated Reconometrics run exercised the rebuilt unified and procedure
binaries.  ARB completed a full automatic proceeding with six demonstrated
votes, seven observed council requests, a closed Lean-replay record, and no
terminal-cleanup error.  Simple completed an OpenRouter GPT-4.1 Responses
request with hosted search enabled and returned `ok/demonstrated`.  The
complete `common/openai` and `internal/lawyer` test packages passed, as did the
focused ARB adapter and container-lifecycle tests and vet for all changed Go
packages.  The restricted sandbox prevented the complete
`runtime/localrun/arb` package from opening its `httptest` listener; the live
run exercised that listener and the changed cleanup path.

## AAR Proof Strengthening

The `aar-proof-strengthening` branch established the AAR authority, custody, replay, and record-integrity model and then applied it to AARD.  AAR binds every state-changing action to the exact current opportunity and proves record integrity, evidence-catalog preservation, and filing-time offer chronology as implications of the Lean certificate checker.  AARD carries the corresponding authority, catalog, lineage, chronology, runtime transaction, and certificate facts while retaining its numeric answer model.  ADC now binds opportunity decisions to their executed actions through certificate replay, while its file and record-integrity adaptation remains open.  The remaining formal agenda includes that ADC record work, constructive terminal-run existence, outcome stability, council symmetry, broader certificate consequences, and independent count-rule properties.  The [AAR record-integrity and runtime update](arb/docs/update.md) records the completed AAR and AARD designs and the remaining ADC decisions.

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

`--publish-dir` creates a new portable directory, copies every selected artifact under `files/SOURCE_ID/`, and writes `case-record.json` after the copies complete.  Published source paths are relative to the index.  Direct indexes retain absolute source paths.  `path_base` distinguishes the two forms.  The publication target must remain outside every source root.  Copy errors leave the incomplete directory without a completed index.

- [x] Add the common case-record reader and JSON schema.
- [x] Add the `adjudicate case-record` command and external output-file handling.
- [x] Add portable publication with explicit source-path bases.
- [x] Test Simple, Quick, ARB, ARBD, and ADC record extraction.
- [x] Run focused tests, vet, build, and retained Simple and Quick examples.
- [x] Document accepted layouts, selection rules, JSON fields, timestamp sources, and unavailable-session behavior.
- [x] Align the ADC and ARB Pi authentication fixtures with the shared validator.

## Provider-neutral council and jury execution

The current change adds one provider-neutral model executor for council members and jurors.  Quick calls the executor in its process.  ARB, ARBD, and ADC use the same executor for direct model calls and expose each selected configuration to a Pi council member through a loopback OpenAI Chat Completions server.  The upstream provider credential remains in the core or local-run process.  Pi receives a per-member bearer token, a random model alias, and the loopback address.

The approved endpoint set is `anthropic`, `deepseek`, `google`, `huggingface`, `openai`, `openrouter`, and `xai`.  Direct Anthropic requests use the Messages API.  Direct Google requests use GenerateContent.  Direct DeepSeek requests use Chat Completions.  Hugging Face, OpenAI, OpenRouter, and xAI use OpenAI Responses-compatible endpoints.  The fixed base URLs and credentials are:

| Endpoint | Base URL | Credential |
| --- | --- | --- |
| `anthropic` | `https://api.anthropic.com/v1/messages` | `ANTHROPIC_API_KEY` |
| `deepseek` | `https://api.deepseek.com/chat/completions` or `/beta/chat/completions` for strict tools | `DEEPSEEK_API_KEY` |
| `google` | `https://generativelanguage.googleapis.com/v1beta/models/MODEL:generateContent` | `GEMINI_API_KEY` |
| `huggingface` | `https://router.huggingface.co/v1` | `HF_TOKEN` |
| `openai` | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| `openrouter` | `https://openrouter.ai/api/v1` | `OPENROUTER_API_KEY` |
| `xai` | `https://api.x.ai/v1` | `XAI_API_KEY` |

Provider routing constraints apply only to OpenRouter.  Native Anthropic, Google, and DeepSeek adapters reject request headers and `max_tool_calls`, which those adapters do not implement.  Every endpoint rejects request-spec attempts to replace its authorization or host headers.  No endpoint falls back to OpenRouter.

The pool selector permits the same request specification to occupy more than one seat.  It selects among the least-used eligible endpoints, then among the least-used configurations for that endpoint, using cryptographic random selection for ties.  A failed configuration leaves the pool.  A missing endpoint credential removes all configurations for that endpoint.  The endpoint allowlist and minimum endpoint count apply to the selected council.  ADC applies them to the candidate panel because voir dire can remove candidates before the final jury forms.

`common/modelapi` contains the provider-neutral response, tool-call, usage, accounting, and provider-error types.  `common/openai` aliases those types and retains the OpenAI-compatible request implementation.  `common/modelgateway` contains the executor, native provider adapters, bounded raw HTTP client, content conversion, Pi loopback server, and focused protocol tests.  `common/councilsample` contains the reusable selector.

The Anthropic adapter preserves returned thinking blocks when continuing a tool exchange, groups consecutive tool results into one user message, uses adaptive thinking with `output_config.effort`, and counts cache-created, cache-read, and thinking tokens.  The Google adapter preserves `thoughtSignature`, groups consecutive function responses, maps reasoning levels to the uppercase REST values `MINIMAL`, `LOW`, `MEDIUM`, and `HIGH`, and preserves usage metadata.  The DeepSeek adapter preserves `reasoning_content`, uses the documented `thinking` toggle and `reasoning_effort` field, maps `medium` and `xhigh` to `high`, and uses the beta endpoint when a tool requires strict schema enforcement.

The Pi server accepts `POST /v1/chat/completions`, supports streamed and non-streamed replies, converts Pi's OpenAI-format messages and tools into the provider-neutral request form, and enforces append-only conversation history.  The implementation matches Pi 0.72.1 behavior in `/home/somebody/.npm-global/lib/node_modules/@mariozechner/pi-coding-agent/node_modules/@mariozechner/pi-ai/dist/providers/openai-completions.js`, including assistant `content: null` when a tool-call response has no text.  It writes one JSONL request record with timestamps, endpoint, model, returned model, response ID, usage, and provider failure data.  Handler write failures are returned from server shutdown.

Quick, ARB, and ARBD accept repeatable `--council-endpoint` and `--minimum-distinct-council-endpoints`.  ADC accepts the same flags for juror candidates.  Unified settings add `common.council_allowed_endpoints` and `common.council_minimum_distinct_endpoints`.  The unified runners and formal local-run commands pass both settings to their cores.

ARB, ARBD, and ADC local runs start the loopback model server when Pi council members or jurors need it.  Each Pi process receives a fresh token and alias.  Provider credentials are removed from the Pi environment.  The request records are `logs/council-model-requests.jsonl` for ARB and ARBD and `logs/juror-model-requests.jsonl` for ADC.  These Pi-path request records contain usage, but the current formal core provider-accounting object does not incorporate that usage.  Existing Pi council calls also occurred outside the formal core accounting.  This limitation requires documentation or a separately approved accounting design.

Simple continues to support its existing `openai://` and `openrouter://` direct models.  The current change concerns councils and juries.  `runtime/adjudicate/simple_runner.go` uses the shared credential-name registry only for credential lookup and participant-environment filtering.

The changed production areas are Quick council execution; ARB and ARBD council selection and preflight; ADC juror assignment and response clients; the three formal local-run packages; the unified settings and runners; the shared OpenAI response types; and the three new shared packages.  The checkout has about 1,477 added and 1,182 deleted tracked lines before this journal entry.  No commit contains this work.

Focused protocol tests cover Anthropic thinking-block replay and grouped tool results, Google thought-signature replay and grouped function responses, DeepSeek reasoning and tool parsing, request-spec restrictions, Pi continuation history, and endpoint-balanced duplicate selection.  The following command passed for every listed package except Quick's listener-dependent tests:

```bash
env GOCACHE=/tmp/adj-go-cache go test \
  ./common/modelapi ./common/modelgateway ./common/councilsample \
  ./common/openai ./quick ./arb/runtime/proceeding \
  ./arbd/runtime/proceeding ./adc/runtime/runner ./adc/runtime/cli
```

`common/modelgateway`, `common/councilsample`, `common/openai`, both arbitration proceeding packages, `adc/runtime/runner`, and `adc/runtime/cli` passed.  Four Quick tests failed because the restricted test process could not open a loopback listener.  Three timed out waiting for `runtime.json`; one reported `listen tcp 127.0.0.1:0: socket: operation not permitted`.  Earlier focused local-run argument tests and unified settings and runner tests passed.  `gofmt` has run over every changed Go path.  The complete suite, build, vet, and diff checks remain to run after the live tests and documentation edits.

The configured environment contains OpenAI, Anthropic, Google, xAI, and OpenRouter credential variables.  It lacks `DEEPSEEK_API_KEY` and `HF_TOKEN`.  Live model discovery found `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna` on OpenAI; `claude-opus-5`, `claude-sonnet-5`, `claude-fable-5-1`, `claude-opus-4-8`, and other current models on Anthropic; and `gemini-2.5-flash`, `gemini-2.5-pro`, `gemini-3.1-pro-preview`, `gemini-3.5-flash`, and other current models on Google.  The xAI model-list request returned HTTP 401 with `{"code":"unauthenticated:bad-credentials","error":"Bad credentials."}`.  The xAI adapter has protocol tests but lacks a successful live test with the current credential.

The first live Quick command failed while creating its output under the checkout because `/dev/sdd1` had no available blocks.  The second run wrote its record to `/tmp/adj-gateway-quick-out`.  Its one-member pool used `openai://gpt-5.6-luna`, low reasoning, the generic persona, and the production `submit_council_vote` preflight.  The preflight succeeded with 157 input tokens, 44 output tokens, 14 reasoning tokens, and 201 total tokens.  The core then opened `http://127.0.0.1:38451` and waited for the plaintiff.  No lawyer submission was sent, so the case ended after the configured two-minute lawyer timeout.  This proves the direct OpenAI executor and Quick preflight path reached a valid tool response.  It does not complete the Quick case test.

Ignored live-test inputs are under `tmp/live-gateway/`.  `pool.jsonl` currently names `openai://gpt-5.6-luna`, and `token` contains the local Quick API token with mode 0600.  The finished live record is `/tmp/adj-gateway-quick-out`.  The provider model-list responses are `/tmp/adj-openai-models.json`, `/tmp/adj-anthropic-models.json`, and `/tmp/adj-google-models.json`; the xAI error is `/tmp/adj-xai-models-error.json`.  None contains an API key.

The reusable approval for `quick/.bin/quick case` permits further live Quick runs with loopback and provider access.  Individual model-list `curl` commands received exact approvals.  Further testing should use the procedure executables so one reusable command approval covers their provider requests.

The first live Quick command exposed a full home filesystem.  Inspection attributed most new use to Go build caches and unrelated Lean builds rather than this checkout.  The user freed the disk before testing resumed.  Live case records remained under `/tmp` so generated procedure output did not enter the working tree.

The [model-endpoint guide](docs/model-endpoints.md) records endpoint credentials and protocols, duplicate configurations, selection order, endpoint controls, Pi loopback execution, request records, native request restrictions, and the ADC candidate-panel limitation.  The Quick, ARB, ARBD, ADC, unified-command, AARD council, and AAR local-run guides now use the same behavior.

Complete Quick cases passed through direct OpenAI, Anthropic, and Google requests.  The OpenAI run used `gpt-5.6-luna` with low reasoning and returned `not_demonstrated`; its availability and vote requests reported 620 input, 160 output, 37 reasoning, and 780 total tokens.  The Anthropic run used `claude-sonnet-5` with low reasoning and returned `not_demonstrated`; its requests reported 1,855 input, 372 output, and 2,227 total tokens.  The Google run used `gemini-3.5-flash` with low reasoning and returned `not_demonstrated`; its requests reported 669 input, 131 output, 522 reasoning, and 1,322 total tokens.

The first Anthropic Quick request exposed an invalid repair exchange: the continued request omitted a tool result for rejected tool calls.  Quick, ARB, and ARBD now add one `function_call_output` for every rejected call and preserve the previous response identifier before repairing an oversized response.  Focused tests cover the correction input.  The first Google availability request exposed that GenerateContent's `parameters` field rejected the complete submission schema; the adapter now uses the documented `parametersJsonSchema` field.

A complete ARB local run exercised the Pi path with one `anthropic://claude-sonnet-5` council member, two Codex subscription lawyers at `xhigh`, live lawyer search, the real Lean engine, the AAR MCP adapter, the Pi container, and the loopback model server.  The case closed `not_demonstrated` after 18 minutes and 30 seconds.  Pi called `aar_wait_for_opportunity`, then `aar_submit_council_vote`.  The two successful upstream Anthropic calls reported 4,484 input and 89 output tokens, then 22,048 input and 320 output tokens.  Pi began one post-tool continuation after the accepted vote; core closure canceled it, and the request log retained the resulting `context canceled` row.  The case's Pi container was absent after launcher cleanup.

The formal run found two launcher defects.  ARB and ARBD local option validation still required `OPENROUTER_API_KEY` for Pi councils even when the selected pool used another endpoint; that obsolete requirement was removed.  Codex rejected its assigned `/tmp` work directory because it was outside a Git repository; new and resumed Codex invocations now pass `--skip-git-repo-check`.  The focused local-run and agent tests cover both corrections.

The fixed OpenAI service URL invalidated two existing AAR black-box tests that had redirected production execution with `OPENAI_BASE_URL`.  Those tests now call the current `runCase` path in the test process and install a scoped HTTP transport that redirects only `api.openai.com` requests to their fake Responses server.  They retain the real Lean engine, provider request construction, Case API, command result, and durable record.  The separate runtime-failure test still executes the built `aar` binary and checks its nonzero exit.

The final credential review found that unified participant environments removed only three canonical provider variables unless a settings entry named another source.  They now remove every canonical provider credential registered by the shared executor before adding the selected lawyer profile's credential.  The environment-separation test includes an undeclared ambient Google credential.

The shared settings validator originally limited `council_minimum_distinct_endpoints` to `council_size` for every procedure.  That bound applies to Quick, ARB, and ARBD because they enforce the minimum across a completed council.  ADC enforces the minimum across candidate assignments before voir dire, so an ADC-only settings file may specify a minimum greater than the final jury size.  A focused settings test covers that case.

The documentation review corrected AARD Council API behavior.  Direct AARD councils receive availability requests before seating.  Council API mode selects the roster without a provider request because external clients own model execution, while the complete local runner checks the selected endpoint credentials before it starts Pi.  AARD direct calls use the shared executor, but the current AARD result schema has no provider-accounting field.  Adding that field requires a separate schema decision.

Quick's current selection path no longer uses its former random loader or a preliminary shuffle.  The shared selector supplies random tie-breaking after balancing endpoints and configurations.  The obsolete loader, shuffle, and their tests were removed.

One attempted formal run started two Claude subscription lawyers concurrently.  One authenticated while the other failed because its staged OAuth session could not refresh.  The evidence indicates concurrent refresh-token use across staged credential copies.  This limitation remains unresolved.  It does not affect the completed Codex-lawyer test.

The endpoint implementation follows the official [Anthropic Messages](https://platform.claude.com/docs/en/api/messages/create), [Anthropic tool-use](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview), [Google GenerateContent](https://ai.google.dev/api/generate-content), [DeepSeek Chat Completions](https://api-docs.deepseek.com/api/create-chat-completion/), [DeepSeek thinking-mode](https://api-docs.deepseek.com/guides/thinking_mode/), [Hugging Face Responses](https://huggingface.co/docs/inference-providers/en/guides/responses-api), [OpenRouter Responses](https://openrouter.ai/docs/api/api-reference/responses/create-responses), [OpenAI Responses](https://developers.openai.com/api/reference/cli/resources/responses/methods/create), and [xAI tool](https://docs.x.ai/developers/tools/overview) documentation.

The final protocol review aligned DeepSeek thinking requests with its documented `thinking` object and `reasoning_effort` field, added Anthropic's reported thinking tokens to normalized accounting, and replaced the obsolete xAI documentation link with its Responses reference.  The complete Go test suite, `go vet -p=1 ./...`, `go build -p=1 ./...`, the scoped Markdown and JSON check for every changed guide, and `git diff --check` passed.  The repository-wide Markdown checker also inspected retained participant workspaces under `data/examples`; third-party skill files there and existing example and prompt placeholders produce unrelated failures.

The current xAI credential still returns HTTP 401.  `DEEPSEEK_API_KEY` and `HF_TOKEN` remain unavailable.  Those three adapters have focused protocol tests but no successful live provider test.  A permanent direct-lab pool has not been selected; its composition requires separate approval.

One unresolved protocol question is Pi's optional `tool_choice` field.  The loopback server parses it but does not forward it.  Pi omits the field in the current council path unless its caller supplies an option.  Confirm the production call behavior before deciding whether to reject or translate a supplied value.

- [x] Add the provider-neutral response and accounting types.
- [x] Add the shared executor and native Anthropic, Google, and DeepSeek adapters.
- [x] Add the authenticated loopback Pi model server.
- [x] Permit duplicate configurations and balance selection across endpoints.
- [x] Pass endpoint controls through the core commands, local runs, and unified settings.
- [x] Pass focused protocol, selection, procedure, and settings tests.
- [x] Complete a live OpenAI Quick preflight through the production path.
- [x] Complete live Quick cases through OpenAI, Anthropic, and Google.
- [x] Complete one Pi council case through a formal procedure.
- [ ] Correct or replace the xAI credential and test xAI.
- [ ] Obtain credentials before testing DeepSeek and Hugging Face.
- [ ] Review the tested direct configurations and confirm a permanent lab pool.
- [x] Correct and re-read the affected manuals.
- [x] Run the complete tests, build, vet, and diff checks.

## Direct-lab follow-up

The optional `common/data/personas/direct-lab-pool.jsonl` contains the three configurations that completed Quick availability and voting requests through OpenAI, Anthropic, and Google.  The generated OpenRouter pool remains the runtime default.

The selector checks the configured minimum endpoint count after removing a configuration or endpoint.  Quick, ARB, AARD, and ADC retain the rejected provider error when that removal makes the minimum impossible.  ADC checks each previously untested pool record through the production `submit_juror_vote` schema before assigning it to an automatically generated candidate.  It caches successful checks, rejects a failed record, rejects an endpoint after a missing-credential error, and samples another eligible record.

The Pi loopback server rejects a non-null `tool_choice` value.  Every ARB, AARD, and ADC Pi opportunity token is removed after its process exits, including a process canceled during case cleanup.  Preparation and process-start errors remove the token before returning.

AARD now copies the shared executor accounting into its native result, local-run result, unified outcome, and unified core error.  Direct availability and answer requests contribute to that accounting.  Pi council calls remain in `logs/council-model-requests.jsonl` because they execute in the local runner rather than the formal core.

Two AARD black-box tests had depended on `OPENAI_BASE_URL`, which the fixed OpenAI endpoint no longer reads.  The tests now execute the current `runCase` path with an HTTP transport that redirects only `api.openai.com` to the test server.  They continue to exercise the Lean engine, provider request construction, Case API, command result, and durable record.

The complete Go test suite, vet, build, scoped Markdown-link check, JSONL parse, and diff check pass.  A focused rerun covers the corrected Quick endpoint-minimum failure path.

- [x] Add the direct-lab pool.
- [x] Reject unusable ADC candidate configurations before assignment.
- [x] Enforce endpoint minima after candidate removal.
- [x] Reject unsupported Pi `tool_choice` requests.
- [x] Revoke Pi opportunity tokens after process exit.
- [x] Add AARD direct-provider accounting.
- [x] Run the complete tests, vet, build, and documentation checks.
- [ ] Run live procedure tests.

## Direct-lab live procedure tests

A complete Quick case selected five members from the direct-lab pool with all three endpoints represented.  The run completed `not_demonstrated` by five votes to zero.  Its eight availability and voting requests reported 4,745 input, 805 output, 280 reasoning, and 5,806 total tokens.

A complete ARB case used the real Lean engine, eight external Lawyer API filings, and a five-member direct council containing all three endpoints.  The run completed `not_demonstrated` by five votes to zero.  Its eight provider requests reported 4,204 input, 458 output, 260 reasoning, and 4,922 total tokens.  Certificate replay passed with thirteen actions.

A complete AARD case used the real Lean engine, eight external Lawyer API filings, and a five-member direct council containing all three endpoints.  The council returned 0, 30, 45, 0, and 10.  The new provider field recorded eight requests, 4,515 input, 631 output, 986 reasoning, and 6,037 total tokens.  Certificate replay passed with thirteen actions.

The ADC proposition test stopped during pretrial before candidate-juror assignment.  `pretrialCandidates`, now in `adc/engine/ADC/Core.lean`, emitted a required deterministic `import_case_file` action with `source_filename` set to `scenarios/assets/supply_chain_delay_notice.txt`.  That file does not exist in the repository.  The same engine function contained case-specific deterministic discovery about a disputed document, confidential package, transmission logs, written authorization, third-party disclosure, and damages, plus fixed monetary values.  Proposition-case generation added the generic Proposition Tribunal after this autopilot code and used the same `autopilot_trial` loop.  The resulting generic case therefore received facts and actions from an unrelated example.

The failed ADC run retained five completed non-juror model-response events but no terminal result or provider accounting.  The completed Quick, ARB, and AARD runs reported 24 requests and 16,765 tokens.  An earlier restricted-network ARB attempt recorded one failed request with no usage.

## Domain-neutral ADC autopilot

The generic `autopilot_trial` loop now reserves deterministic actions for values derived from case state or court policy.  Plaintiff, defendant, and judge models supply Rule 11 filings and rulings, discovery content and responses, Rule 37 motions and rulings, Rule 68 offers and acceptances, pretrial orders, settlements, partial judgments, and post-judgment filings and rulings.  Rule 68 cost-shift evaluation runs after judgment and compares the expired offer with the entered judgment amount.  Phase transitions, jury setup, skipped-voir-dire empanelment, and judgment entry remain deterministic.

The docket now retains the substantive payloads for Rule 11 notices, corrections, and motions; Rule 37 motions; interrogatories and responses; production requests and responses; admission requests and responses; and Rules 59 and 60 motions.  The participant tool schemas require those payloads.  Separate docket entries track each party's initial disclosures.

The generic file-import opportunity was removed.  Proposition documents enter as complaint attachments before adjudication, and the internal actor had no source file from which to perform a later import.  A passed import opportunity otherwise returned after each state change because ordinary pass identifiers are transient.  Rule 37 passes now write a decision trace after discovery responses are complete, preventing repeated model calls on an unchanged discovery record.

Rules 11 and 37 now expose model-backed production judge opportunities.  Their eval commands run those opportunities directly and accept candidate objective templates without a counterfactual flag.  Rule 58 retains its state-derived production judgment action and its counterfactual-model option.

The report summary request no longer supplies a fixed temperature.  A live case using `gpt-5.6-luna` reached judgment but failed during digest generation because that model rejects the former `temperature: 0.2` parameter.  The report package test and rebuilt ADC command passed after removing the parameter.

One-fixture Rule 11 and Rule 37 production evals completed through `gpt-5.6-luna`, `apply_decision`, and `step`, with correct fixture scores.  They reported two requests and 14,803 tokens.  The ADC engine, proof target, explicit `ApplyDecision` proof, and explicit pretrial-action proof passed through `leanrunner`.

A complete proposition case at `/tmp/adj-direct-lab-e2e-20260912/adc-9` used the real Lean engine, web-enabled OpenAI litigation actors, six jurors selected from the direct OpenAI, Anthropic, and Google pool, and all three required candidate endpoints.  It reached `judgment_entered` after 65 turns and resolved the proposition as `not_demonstrated`.  The run recorded one Rule 37 opportunity, both initial disclosures, and the complete discovery payloads.  Certificate replay passed all 74 transitions.  The run reported 48 requests, 520,413 input tokens, 12,857 output tokens, 2,676 reasoning tokens, and 533,752 total tokens.  Completed procedure and eval records together report 74 requests and 565,320 tokens.  Several stopped ADC attempts contain successful requests that did not reach terminal provider accounting, so the experiment total exceeds the recorded total.  No completed record contains provider cost data.

The complete Go test suite, `go vet -p=1 ./...`, `go build -p=1 ./...`, `gofmt`, and `git diff --check` passed.  `lake build Proofs adcengine` and the changed pretrial and post-judgment proof modules passed through `leanrunner`.  A later proof audit found that eleven files excluded from the cleanup root contained useful coverage; the ADC Lean cleanup section records their restoration and repair.

- [x] Replace case-specific deterministic ADC actions with state-derived or model-backed opportunities.
- [x] Preserve substantive filing and discovery payloads in the docket.
- [x] Run live Rule 11 and Rule 37 production evals.
- [x] Complete a six-juror, three-endpoint ADC proposition case.
- [x] Verify the live certificate and inspect the resulting record.
- [x] Complete repository-wide tests, vet, build, documentation checks, and final review.

## ADC Lean cleanup

The formal state machine and procedure API now reside in `ADC/Core.lean`; `Main.lean` contains the JSON protocol and executable entry point.  Every proof module imports the reusable core through its dependency graph and uses a namespace below `ADCProofs`.  The maintained `Proofs.lean` root imports all 71 proof files.

The cleanup originally removed eleven proof modules after treating their subjects as obsolete.  Review showed that those modules contained current coverage or coverage that needed adaptation to the current procedure.  The modules were restored.  Jury status and setup proofs now use candidate jurors, phase-completeness proofs cover the current action family, and role-guard proofs cover current juror-vote and empanelment actions.  The retired manual jury-verdict and hung-jury validators were replaced with proofs of `submit_juror_vote`, verdict derivation, split-jury continuation, final-round hung-jury derivation, and stored outcomes.  Voir dire now covers the staged question, ruling, answer, challenge, challenge decision, and peremptory-strike transitions.

Opportunity identifiers use positions in the complete state agenda.  Fixed juror questionnaire items remain general qualification questions, while counsel supplies case-specific questions through the voir dire procedure.  A replay transition for an opportunity decision validates its state version, opportunity identifier, role, decision, allowed tool, and exact executed action.  Deterministic runtime actions remain direct replay transitions.

Procedural docket entries now separate display descriptions from typed fields.  Discovery matching, Rule 37 generation tracking, Rule 56 motion-party and response matching, exhibit counts, and dispositive-motion counts use exact fields.  A decided Rule 37 motion blocks another motion for the same party and discovery generation.  Later discovery from that party creates a new generation.  Rule 56 opposition and reply checks use the pending motion index.  Focused computed proofs cover these cases and show that misleading description text does not affect the typed exhibit or motion counters.

The runtime supplies RFC 3339 event times for `produce_case_file` and `offer_exhibit`.  Opportunity execution replaces participant-supplied event times; direct scenario actions retain an explicit time and receive the runtime time when the field is absent.

The generated ADC inventory contains 71 proof files, 611 public theorem or lemma declarations, and 10,830 lines.  Every ADC proof target and every ARB proof target passed in a separate `leanrunner` invocation with one Lean job.  The ADC and ARB executables also passed separate builds.  The ARB audit found no deleted proof file or unimported proof source: `Reachability.lean` and `Samples.lean` enter the root through dependent modules.  Its Lean `step` validates exact current-opportunity authority before every accepted action, and Go certificate replay submits each recorded authority to that same transition function.  No ARB engine or runtime repair was required.

The complete Go test suite, `go vet -p=1 ./...`, `go build -p=1 ./...`, focused race tests for the ADC runner and ARB proceeding, and `git diff --check` passed after the runtime, proof, and documentation repairs.  A source scan found no `sorry`, `axiom`, or `unsafe` declaration in the ADC, ARB, or AARD Lean proof trees.

- [x] Replace hashed opportunity identifiers with state-agenda indices.
- [x] Replace case-specific fixed juror questions with general qualification questions.
- [x] Make Rule 37 pass closure depend on the party and discovery generation.
- [x] Generate party-specific discovery, Rule 37, and Rule 56 opportunities for both parties.
- [x] Bind accepted opportunity actions to their opportunity in replay certificates.
- [x] Put every maintained proof in one namespace and one build target.
- [x] Restore and update every proof module removed during cleanup.
- [x] Update and regenerate the ADC proof documents.
- [x] Split `Main.lean` at the core and executable boundary.
- [x] Run focused Lean and Go tests, the complete test suites, and live ADC cases.

The documentation describes opportunity decisions for both direct-model and external Role API execution.  The theorem catalog and proof statistics derive from the current proof files.

## AARD proof and documentation update

The AARD state machine and procedure API now reside in `AARD/Core.lean`; `Main.lean` contains the JSON protocol and executable entry point.  The 14 proof modules use the `ArbdProofs` namespace and enter the build through direct imports in `Proofs.lean`.  The generated inventory contains 177 public theorem or lemma declarations and 5,235 lines.

`ProcedureInvariants.lean`, `StepPreservation.lean`, `Progress.lean`, and `OutcomeSoundness.lean` prove the general case invariant, preservation by successful steps, active-case opportunity results, finite action-capacity bounds, and terminal outcome properties.  `CertificateFacts.lean` exposes the run invariant with closed and failed replay packages.  Review found no Go transition that differed from the proved state machine.

Every proof module, the aggregate proof import, and the AARD executable passed in separate `leanrun` invocations with one Lean job.  The AARD proceeding, local-run, and command tests passed.  The rewritten AARD documents describe the current source and omit prior test transcripts and temporary paths.

The first complete local test exposed two continuation errors.  Pi sometimes represented an assistant tool call with absent, null, or empty content on successive Chat Completions requests.  The local gateway now normalizes those equivalent representations before enforcing append-only history.  OpenRouter rejected a non-null `previous_response_id` on a stateless Responses request.  The shared OpenRouter client now retains the input and output items for each response and sends the full history with the next tool result, following OpenRouter's [Go agent implementation](https://github.com/OpenRouterTeam/go-agent/blob/main/model_result.go).  Focused tests cover both exchanges.

The second local test reached council deliberation.  Four short-lived Pi council processes exited before final cleanup.  Podman's automatic removal also removed their container-ID files, and ARBD reported their absence as a cleanup error despite recorded exit code 0.  ARB already accepted an absent ID after a completed process.  ARBD now uses the same condition, with a focused test for a completed, automatically removed container.

Further log review showed that three council members still failed the gateway's history comparison in that run.  Pi parses tool-call arguments and uses `JSON.stringify` when it includes them in the next request, changing their textual representation.  The gateway now compares parsed argument values and continues to reject changed values.  The shared executor already removes previously submitted input before continuing a provider conversation.

The following run stopped during the defendant's closing because Codex reported that its usage limit had been reached.  Pi recorded the error and exited zero.  The shared Pi output reader now returns settled provider errors as well as refusals, and ARBD reads that result before recording process completion.  A focused test preserves the quota message and partial usage.  The user restored Codex authentication before the next run.

The remaining-step counter has a finite upper bound.  The current AARD proofs do not establish strict decrease or bounded run length.  The verification guide states that limit.

The manual's opening example now omits its empty supplemental arrays.  The validator accepts those arrays when empty and rejects nonempty ones; the specification now states that condition.  Go tests, vet, build, and the changed documentation's 47 local links pass.

The model-pool screen's Pi test still calls OpenRouter Chat Completions directly, while the current formal local runners use the shared gateway and its Responses client.  The screening command was left unchanged during this AARD update.

The restored-auth run completed all eight lawyer filings and 22 work notes using Pi, Codex subscription authentication, `gpt-5.6-sol`, `xhigh`, and enabled search.  Both lawyers used local programs and found no need for outside sources for the two-sonnet comparison.  Pi retried one Codex processing error and recorded a successful retry.  Kimi K2, DeepSeek V4 Flash, GLM 4.5V, and Qwen3 Max Thinking submitted answers of 88, 87, 88, and 86.  The history-comparison error did not recur.  IBM Granite 4.1 8B returned HTTP 404, and OpenRouter's endpoint listing returned an empty endpoint array for that model.  The installed pool remains unchanged.

That run exposed an error in the new Pi diagnostic handling: adding a provider error to the operating-system process-exit error made cleanup reject a council failure that the core had already recorded and handled.  ARBD now includes the Pi error in process status and participant-failure handling while preserving the operating-system exit result for cleanup.  A second live test submitted the completed eight filings through the Lawyer API and ran the same five model configurations through Pi and MCP.  The unavailable Granite configuration failed again.  The other members answered 88, 90, 86, and 89, and the unified command returned `status: "ok"`, `phase: "closed"`, and exit status zero with the member failure retained.

Certificate replay passed all 13 actions for both runs.  Case-record generation with work notes and sessions passed.  The full-lawyer index contains 42 chronological docket entries and 99 artifacts, with both retained session roots available.  No container with the test name prefix remained after cleanup.  Test inputs, records, sessions, work directories, and external indexes remain under `/tmp/adj-arbd-current.9Vq4G9/`; the completed full-lawyer record is `run-5/`, and the corrected-launcher test is `council-test-2/`.

- [x] Move the AARD state machine and procedure API to `AARD/Core.lean`.
- [x] Put every proof file in `ArbdProofs` and import every proof file from `Proofs.lean`.
- [x] Add general procedure, preservation, progress, action-capacity, and outcome proofs.
- [x] Extend the terminal certificate fact packages.
- [x] Compare the Go runtime with the formal properties and correct any mismatch.
- [x] Regenerate the theorem catalog and proof statistics.
- [x] Rewrite the AARD implementation and verification documents for the current source.
- [x] Run every Lean proof file separately through `leanrun`.
- [x] Run the focused and complete Go tests.
- [x] Run Go vet, build, and final diff checks.
- [x] Run AARD from `adj`, test council failure handling, verify the certificates, and generate the case-record listings.

## AARD follow-up

The installed pool excludes `ibm-granite/granite-4.1-8b` at `coreweave/bf16`.  Two live AARD council tests returned HTTP 404 for that configuration, and OpenRouter returned an empty endpoint list for the model.  The generated pool records remain available in Git history and the generation output.

- [x] Remove the unavailable Granite configuration from the installed pool.
- [x] Route the model-pool Pi screen through the shared runtime model gateway.
- [x] Prove strict decrease and a bound on successful AARD run length.
- [x] Complete focused tests, a live council test, and the documentation updates.

The screening command now uses the shared executor for its Quick-style request and the formal runtimes' local gateway for Pi requests.  The host retains provider credentials and applies the selected request specification.  Pi receives a local alias and token.  The command records upstream response IDs, usage, errors, and observed costs, while retaining the Pi transcript and MCP calls.

Live direct and Pi/MCP checks passed for OpenRouter DeepSeek V4 Flash at Alibaba FP8, Anthropic Claude Sonnet 5, and Google Gemini 3.5 Flash.  The DeepSeek and Claude runs corrected a rejected tool argument and submitted an accepted vote.  OpenRouter reported five cost observations totaling `$0.0009014716`.  Anthropic and Google returned usage without cost observations.  The records remain under `/tmp/adj-arbd-current.9Vq4G9/screen-deepseek`, `screen-anthropic`, and `screen-google`.  No screening container remained after the tests.

The next live AARD council sampled two more unavailable routes: KAT Coder Pro v2.5 at StreamLake and GLM-5 at AtlasCloud FP8.  OpenRouter returned HTTP 404 with the requested and available routes.  Its endpoint listings confirmed that KAT Coder Pro v2.5 was available only at AtlasCloud and that GLM-5 had eight routes, excluding AtlasCloud.  The installed pool excludes those two configurations and now contains 97 entries.  Their case failures and provider responses remain in `council-test-3/`.

That council test completed with answers of 89, 86, and 87 and two recorded member failures.  The unified command returned `status: "ok"`, `phase: "closed"`, and exit status zero.  Certificate verification passed all thirteen actions, and case-record generation included work notes and retained sessions.  The pool removals concern the three configurations that failed these live tests.  The remaining 97 configurations have not undergone another complete availability screen.

The final screening command also passed the direct vote and Pi/MCP tests for OpenAI `gpt-5.6-luna`.  Across all four configurations, the tests recorded 19 provider responses and 55,338 total tokens.  The reported cost remains `$0.0009014716`, with five OpenRouter observations and no cost observations from the direct services.  The OpenAI record is `screen-openai/` beside the other screening records.  No screening or AARD test container remained after cleanup.

`Proofs/BoundedTermination.lean` proves strict decrease of `remainingStepBudget` under every accepted public action from a state satisfying `ProcedureInvariant`.  Successful initialized runs contain at most `2 × max_submitted_evidence_per_side + 8 + council_size` accepted actions, and an infinite sequence of accepted actions is impossible.  Both terminal certificate fact packages now include the accepted-action bound.  The evidence-submission helper exposes the validated per-side count needed by the decrease proof, while preserving the original theorem and its statement.

The maintained AARD tree contains 15 proof files, 209 theorem or lemma declarations, and 6,093 lines.  The changed record-integrity, bounded-termination, certificate-fact, and certificate-example targets passed in separate `leanrunner` invocations, followed by the explicit proof-root check.  Each invocation used one Lean job, a 6 GiB memory limit, and a 100% CPU quota.  The proof tree contains no `sorry`, `axiom`, or `unsafe` declaration.  The complete Go tests, `go vet -p=1 ./...`, `go build -p=1 ./...`, affected-document link check, and `git diff --check` passed.  The verification guide distinguishes the accepted-transition bound from runtime request and elapsed-time limits.

## Prompt and documentation review: 2026-09-14

The review identified conflicting lawyer tool and work-note instructions, omitted ARBD advocacy roles, incorrect evidence-phase guidance, fixed search prose outside the launcher catalogs, and outdated operating examples.  The approved work updates those prompts and their compiled fallbacks, adds search prompt entries through the existing resolver, and corrects the documentation against the current source.

The search entries are `search.local.enabled`, `search.local.disabled`, `search.remote.enabled`, and `search.remote.disabled`.  Formal launchers use all four.  Quick uses the remote pair because its automatic-lawyer search instructions already belong to its core catalog.  The web-search setting selects the entry.  File selection and token validation use the existing launcher prompt rules.

The ADC limits reference follows [policy initialization](adc/runtime/runner/state_init.go), [limit and override rules](adc/engine/ADC/Core.lean), and [runtime limits](adc/runtime/runner/runtime_limits.go).  Inspection also found that the Go override tool schema requires `override_value`, while Lean requires `new_value` and `ordered_by`.  Approval to correct that additional mismatch is pending.

- [x] Correct lawyer tool scope, ARBD roles, evidence phases, and work notes.
- [x] Make local and remote search instructions replaceable.
- [x] Complete current limits, pool, prompt-authoring, and example references.
- [x] Run all Go tests, vet, build, and edited-document link checks.
- [x] Complete the live ARBD test and review its record.

`go test -p=1 ./...`, `go vet -p=1 ./...`, and `go build -p=1 ./...` passed.  The edited Markdown contains 52 local file links, all resolvable.  Launcher tests exercise enabled and disabled search selection through all four source levels, and generated remote-skill tests check individual search overrides.  The ARBD rendered-prompt test checks the higher-score and lower-score assignments.  No Lean source or proof changed.

The live test used the two sonnets from `arbd/examples/ex1`, two Pi lawyers using `openai/gpt-5.6-sol` at `xhigh` through Codex authentication, web search enabled, a custom `search.local.enabled` file, and five Pi/MCP council members selected from the direct-lab pool.  Its inputs, settings, notes, sessions, and case files are retained under `tmp/prompt-review-20260914/`.  Both lawyers received the custom search text, wrote and ran comparison programs, retained their work, and completed all eight filings.  They sent 23 notes across those opportunities.  Their final proposed scores were 94 and 88.  Neither used web search for this self-contained text comparison.  The plaintiff corrected a Python import error, and the defendant corrected an unavailable `python` command and missing evidence-file paths.

The case closed with four integer answers of 90 and one recorded council failure.  Google returned HTTP 429 for `GenerateRequestsPerDayPerProjectPerModel-FreeTier`, reporting a daily limit of 20 requests for `gemini-3.5-flash`.  The 13-action certificate passed `aard verify-certificate`.  All case processes stopped, and `podman ps` showed no running containers.  After shutdown, the retained test directory occupies 23 MiB, and the completed lawyer sessions report an estimated $9.54 in usage.  That estimate is not a subscription billing total.  The council gateway recorded 25 logical requests, including three failed requests, and supplied token usage without cost records.

Two council agents called tools after their answers were accepted: C2 made seven more calls, and C4 made three.  Their launcher tells them to stop after acceptance, while the shared MCP session tells participants to wait until the case reaches a terminal state.  These instructions need alignment before another test.  That follow-up and the ADC override-schema correction require approval.  This run tested ARBD end to end.  ADC, ARB, and Quick changes passed their Go tests without an additional live case in this review.

## Documentation corrections: 2026-09-14

The approved documentation review covers complete-case setup, council sampling, Lean dependencies, protective orders, provider credentials, navigation, and index descriptions.  The README now gives a complete two-Pi-lawyer ARB invocation with five council members, three required votes, Codex subscription authentication, `xhigh` reasoning, and enabled search.  The build instructions distinguish command compilation from the Pi image build.

The council guide follows the [shared selector](common/councilsample/selector.go) and [ARB preflight](arb/runtime/proceeding/council_preflight.go): balance endpoints, balance records within an endpoint, and allow repeated configurations.  The protective-order guide follows the [ADC core](adc/engine/ADC/Core.lean) and [tool schemas](adc/runtime/runner/tools.go), distinguishing recorded order terms from enforcement.  The proving guide follows ADC's [Lake configuration](adc/engine/lakefile.toml) and [empty package manifest](adc/engine/lake-manifest.json).  The Pi guide uses the credential boundary documented in the [model-endpoint reference](docs/model-endpoints.md).

- [x] Correct the seven documentation findings and ARB's Pi authentication help text.
- [x] Check edited-document links and run the launcher and shared-selector tests.
- [x] Complete the README case command and inspect its output.
- [x] Commit and push the changes on `main`.

`go test -p=1 ./cmd/aar-run ./common/councilsample` and the ARB launcher build passed.  The initial edited-document check covered 146 local links with no missing target or heading.  The live case uses `arb/out/first-case/` and the existing Pi image and Lean engine binary.  Its first startup encountered the sandbox's socket restriction.  Execution with the approved launcher permission started both lawyers and the case API.

The final documentation check covered 160 local links, including the development journal, with all targets and headings resolved.  `git diff --check` and the launcher help check passed.  Commit `ad87800` contains the documentation corrections and two CLI help-string changes and was pushed to `origin/main`.

The README command completed in 32 minutes and 6 seconds with exit status zero, `status: "ok"`, `phase: "closed"`, and resolution `demonstrated`.  Both Pi lawyers completed their four filings and sent twenty work notes in total.  They used local execution and retained files, including signature verification.  Search was enabled, with zero recorded search-tool calls.  Five models from the default pool voted: Kimi K2 and Muse Glimmer voted `not_demonstrated`, while GPT-4 Turbo, Gemini 2.5 Pro, and Aion 3.0 Mini voted `demonstrated`.  `aar verify-certificate` accepted all thirteen recorded actions.

The council log contains fifteen successful requests and 123,758 total tokens.  It provides no dollar-cost observations.  The core's five availability requests reported $0.00080588, and the Pi lawyer sessions report $9.524578 in estimated usage.  Subscription billing is separate from that estimate.  After shutdown, Podman listed zero running containers, both lawyer credential files were absent, and the retained case directory occupied 31 MiB.  The case record, participant work, and logs remain under `arb/out/first-case/`.

## Israel–Syria ARB test: 2026-09-18

The approved test uses the unchanged [Israel–Syria proposition](examples/ex04/complaint.md) and [market rules and resolution rationale](examples/ex04/market-rules.md).  Both Pi lawyers use `openai/gpt-5.6-sol`, `xhigh` reasoning, Codex subscription credentials, and enabled web search.  Nine council members are sampled from `common/data/personas/pool.jsonl`, with five votes required under preponderance of the evidence.  Lawyer and council turns each have a 1,500-second timeout.  The checked-in policy and prompts otherwise remain unchanged.

The output directory is `arb/out/ex04-20260918T144715Z/`.  The initial checkout is commit `7016150` on `main`.  Complaint validation passed.  Existing ARB executables and the Pi image are available.  Before starting, Podman reports no running containers and the repository filesystem has 13 GiB available.  The supplied evidence includes Polymarket's Yes resolution and rationale.  Review must distinguish that supplied account from sources the lawyers retrieve.  The known council stop-instruction conflict also requires observation.

- [x] Complete the case while observing notes, source retrieval, evidence admission, filings, council calls, and errors.
- [x] Inspect the result and replay certificate, record usage, and confirm case-process and credential cleanup.
- [x] Record findings and retained artifact locations.

The case closed `demonstrated` after 47 minutes and 22 seconds.  Eight council members voted `demonstrated`; Qwen Plus through Alibaba failed before voting.  Its Pi transcript contains an empty-name tool call, followed by `Tool not found` and an HTTP 400 on the next provider request.  The parser and model server accept and forward the empty parsed name.  The saved logs omit the raw upstream response, so the origin of that name remains unresolved.  During initial selection, ARB also replaced an unavailable GLM-5.2/Mistral route and a rate-limited Llama 3.1 70B/DeepInfra configuration.

Both lawyers completed all four filings, made 19 search calls containing 56 queries, and submitted five new source items.  They sent 24 work notes.  Each lawyer made one prohibited opening-phase evidence action, received a rejection, and corrected its submission.  The longest opening research gaps between notes were approximately seven and ten minutes.  The defendant corrected its construction comparison in the accepted surrebuttal after the plaintiff challenged it.  D-2's lawyer-supplied retrieval time is approximately 52 seconds later than its recorded submission, an inconsistent provenance timestamp.  All eight council voters made zero tool calls after acceptance; the final member's subsequent model continuation was canceled on case closure.

Certificate verification passed all 22 actions.  The lawyer sessions report $12.846562 in estimated usage under subscription authentication, including repeated cached context.  The core reports $0.000623665 for availability checks.  The council log records 33 requests and 417,207 tokens without dollar-cost observations.  Search-extension usage is outside those totals.  Podman reports zero running containers, the case-process check found zero matching processes, and staged lawyer authentication and MCP configuration files are absent.  The retained run occupies 46 MiB.  The [test report](arb/out/ex04-20260918T144715Z/review.md) records configuration, evidence, votes, failures, prompt observations, and artifact locations.  Runtime code, prompts, and pool entries remain unchanged.

## Israel–Syria ARB technical report: 2026-09-18

The [case-study report](reports/marxiv/israel-syria-arb/paper.pdf) explains adj's five procedures, ADC's court roles, ARB's opportunity sequence, pool generation and case-time sampling, and the Lean verification boundary.  It then follows the completed case through research, evidence, the defense's narrowing position, the construction correction, closings, and council decisions.  It includes all eight accepted council rationales and attributed excerpts from filings and high-level work notes.  The [report directory](reports/marxiv/israel-syria-arb/README.md) documents the source, build commands, and local record locations.

Review used the accepted transcript, notes, events, evidence metadata, council snapshots, participant tool logs, request accounting, and maintained documentation at `7016150`.  The report distinguishes that source revision from the executable's recorded build identity.  It also distinguishes the original 100-entry generated pool from the 97 configurations installed for this case.  Architectural references include [Council Constitution](arb/docs/councils.md), [ARB Verification](arb/docs/verification.md), [Proof Work Status](docs/proof-notes.md), and the [Sampling Runbook](model-pool/docs/sampling-runbook.md).

Two further observations emerged during report review.  The council case view exposes earlier votes, although the first-round prompts omit them and none of the successful agents called `get_case`.  C6 attributed the secondary report's takeover/seizure wording to the United States without preserving the source qualification.  The paper discusses both observations without claiming that they changed the outcome.

- [x] Check the narrative and quotations against the case record, including all eight complete council explanations.
- [x] Recompute lawyer request, token, cost-estimate, and search-query totals from the logs.
- [x] Review factual scope and prose, read the extracted PDF text, and inspect representative rendered pages.
- [x] Compile the report with resolved references and no layout warnings.

marXiv accepted the report on September 18, 2026, as [2609.00009](http://127.0.0.1:8405/abs/2609.00009), following submission [226dddcac47b](http://127.0.0.1:8405/status/226dddcac47b).  The [editorial review](reports/marxiv/israel-syria-arb/review.md) records the decision and remarks: define endpoint equivalence and representative selection, identify Line A at first use, and revise imprecise or redundant prose.  Report preparation made no runtime, prompt, or pool changes and required no new adjudication or Lean build.

## Israel–Syria ADC test: 2026-09-18

The approved ADC test uses the same proposition and initial market-rules document as the ARB case, with fresh participant sessions and no ARB filings or findings supplied to the participants.  Both Pi lawyers use `openai/gpt-5.6-sol`, `xhigh` reasoning, Codex subscription authentication, and enabled web search.  The jury has nine members from the installed 97-configuration pool, with six concurring votes required.  The evidence standard is preponderance of the evidence.  Model, lawyer, and juror timeouts are 1,500 seconds.  ADC retains its default GPT-5.4 judge and clerk, voir dire, policies, and checked-in prompts.

The retained directory is `adc/out/ex04-20260918T164600Z/`.  The unified command converts the proposition into a declaratory claim between Proponent and Opponent.  The initial evidence includes the supplied Polymarket Yes resolution and rationale.  Before startup, Podman reports no running containers and the filesystem has 12 GiB available.

- [ ] Complete the case while observing research, work notes, pleadings, judicial decisions, jury selection, and voting.
- [ ] Review the result, verify the replay certificate, account for usage, and check process and credential cleanup.
- [ ] Write and review a standalone technical report from the ADC record.

The run was stopped at 17:14:00 UTC during the defense's document-production response, after 27 minutes and 54 seconds.  Both lawyers researched public sources and saved material in their workspaces.  They filed technical reports and answered interrogatories, but the plaintiff's production response could identify only the original `file-0001`.  It explained that retrieved sources lacked case-file IDs.  The defense's last note proposed producing its retained source extracts when a production opportunity became available.

Inspection found that `adc/engine/ADC/Core.lean` implements `import_case_file` but never offers it in the procedural agenda.  `adc/runtime/runner/reference_tools.go` omits import, and the Role API's support-tool handler accepts only those reference tools and file-byte reads.  The production opportunity also requires an earlier import trace.  This leaves automatic lawyer research disconnected from court-file registration.  The user subsequently approved correcting the ADC problem.

The [run review](adc/out/ex04-20260918T164600Z/review.md) retains the diagnosis and completion plan.  The [ADC report draft](reports/marxiv/israel-syria-adc/README.md) contains system background and the early proceeding, marked incomplete.  The unified result records cancellation.  The interrupted core did not publish a final run or replay certificate.  The record retains 17 events and 33 work notes.  The lawyer logs contain 161 completed assistant responses and `$12.901453` in estimated usage, separate from subscription billing and search-extension usage.  The retained directory occupies 38 MiB.  Podman and process checks found no remaining case processes or containers, and staged lawyer authentication and MCP configuration files are absent.

## ADC file-import correction: 2026-09-18

Lawyer opportunities now permit `import_case_file` alongside the pending legal action when the scenario grants import to that role.  The pending filing remains available after an import.  Each unproduced case file has a production opportunity.  The prior production condition stopped after the first file and excluded initial attachments unless a later import occurred.  Imports remain authorized court actions, submitted through the same decision API as other filings.

History identifies commit `1c7f5d6`, “Make ADC autopilot domain-neutral,” as the removal of the import opportunity.  The removed step imported a hardcoded supply-chain file for the defendant when the case had no files.  The change supplied no general replacement.  The new runner test exercises scheduler-generated opportunities before calling the lawyer decision API.

The Go runtime now prepares file metadata before opportunity authorization.  Previously it authorized upload arguments but executed generated metadata, which would fail the replay check requiring equality between the authorized and executed payloads.  The replay check remains unchanged.  The import tool description and manual document workspace uploads, court registration, production, and exhibit offers.

- [x] Test two workspace uploads through the lawyer decision API, production of both files, a trial-phase upload, an exhibit offer, and juror text access using the compiled Lean engine.  Replay each accepted decision and compare its resulting state.
- [x] Run all ADC Go tests and the ADC launcher tests.
- [x] Build the engine and check `AvailableActionsPretrial`, `OpportunityConfinement`, and `ApplyDecision` separately through leanrunner: local, 4 GiB memory high, 6 GiB maximum, 1 GiB swap, 100% CPU, 900-second timeout.  All passed.
- [ ] Complete a fresh live Israel–Syria case using the approved nine-juror, six-vote configuration and verify its certificate.

The fresh run started at 18:30:31 UTC with core version `v0.0.0-20260914232303-7016150f2ddc+dirty` and the rebuilt ADC engine.  It uses separate workspaces and sessions.  The initial proposition and document are unchanged.

Codex stopped this run at 19:04:22 UTC after it exposed a defect in the new scheduling: each file production clears transient passes, reopening the separate import opportunity even when no new information exists.  Four imports and four productions succeeded.  The completed lawyer responses total `$45.194167` in token-price estimates under Codex subscription authentication.  This figure does not include separate search-extension usage.  The run occupies 52 MiB, retains no staged `auth.json` or `mcp.json`, and left no case containers.  No verdict or final certificate was produced.

The user approved adding import to ordinary lawyer opportunities.  Lean and Go now select `constraints.by_tool[tool_name]` when present, with the opportunity-level constraints as the fallback.  The import entry is empty, so report, production, and other filing fields apply only to their respective actions.  The integration test imports two files during a report opportunity, completes the report, produces both files without import-only passes, and verifies trial import, exhibit admission, juror access, and exact replay.  It also rejects a report that supplies another party's fixed fields.

All ADC runtime and launcher Go tests passed.  The revised `DecisionConfinement`, `OpportunityConfinement`, `OpportunityClosure`, and `ExecutionConfinement` statements retain their guarantees using the selected tool constraints.  Leanrunner checked those dependencies and the `ApplyDecision`, `AvailableActionsPretrial`, and `OpportunitySelection` targets on the local host with the limits above.  All passed.  `go vet` passed for the changed runner and prompt packages.

- [x] Implement the approved import scheduling and tool-specific constraints.
- [x] Check the affected Go code and Lean proofs.
- [x] Complete the live Pi/MCP import-and-production test and verify its certificate.

The live test uses the existing ADC local-runner Go API with one Pi lawyer at `xhigh`, Codex subscription authentication, and web search enabled.  The scenario asks the lawyer to create two invoice-calculation documents, import them, file a report, and produce them against a pending request.  It stops at the transition to trial.  The fixture and invocation are retained under `tmp/adc-import-live/`, with output under `adc/out/import-live-20260918-02/`.  The first startup failed because the fixture omitted a required court-profile field.  That attempt made no model requests.  The corrected fixture uses a fresh output directory.

The live lawyer completed two imports, a technical report, and two productions, followed by the deterministic transition to trial.  No import-only pass occurred.  Certificate verification passed all nine state transitions, and both uploaded files match the retained workspace files byte for byte.  The lawyer sent 28 completed responses with `$1.223326` in token-price estimates.  The retained directory occupies 3.8 MiB.  Staged authentication and MCP configuration files are absent, and Podman reports zero running containers.

The first upload attempt used a container path as `source_filename`, then succeeded using base64.  Inspection showed that `legal_tool_specs` omitted the descriptions used by direct model calls.  The Role API now obtains those descriptions and schemas from the existing tool builder.  The source-filename prompt also identifies the court host and directs container and remote-workspace uploads to `original_name` and `content_base64`.  A second live run under `adc/out/import-live-20260918-03/` checks this guidance.

The first run exited with a digest error after writing its state, transcript, and certificate: `defendant summary missing citation anchors`.  The summary collector includes the plaintiff's technical report but finds no defense argument.  The generator still requests both summaries, and its validator requires square brackets in both.  The suggested correction records an explicit absence for a side without argument text and requires citations for summaries with source arguments.  That reporting change awaits approval.  The Israel–Syria rerun remains pending.

The second live run received the full import description and revised source-filename guidance through MCP.  Both imports used base64 without a host-path rejection.  Two imports, a report, two productions, and the trial transition completed with no import-only passes.  Certificate verification passed all nine state transitions.  Final digest generation failed on the same absent-defense citation condition.

The second run also exposed a model-side copying error: the explanation document contained `independently` in the workspace, while the model-submitted base64 decoded to `indepently`.  The stored file matches those submitted bytes.  The lawyer noticed the typo when reading the imported file but retained it.  The first document matches its workspace original.  The two original documents in the first live run also match their uploads.  This observation requires attention before using model-constructed base64 for larger evidentiary files.  No file-transfer design change has been made.

The second test recorded 26 completed lawyer responses with `$1.081359` in token-price estimates, for `$2.304685` across the two live tests.  These figures exclude the direct digest-model requests.  Its retained directory occupies 3.6 MiB.  No staged authentication or MCP configuration file remains, and Podman reports zero running containers.  The final ADC Go tests, runner and prompt vet checks, and `git diff --check` passed.  The two outstanding observations above remain unresolved.

## ADC absent-side digest correction: 2026-09-18

The user approved correcting digest generation for a side with no recorded argument text.  The [report generator](adc/runtime/report/report.go) now checks each side's source text before validating its summary.  An empty source produces the existing sentence, "No courtroom argument text was available for summary."  A nonempty source requires a nonempty summary with citation brackets.  The check uses the record even if the model returns prose for an absent side.  The existing path for two absent sides still avoids a summary-model request.

The checked-in summary prompt and its fallback ask for an empty JSON string for an absent side.  The manual describes the generated absence statement and citation requirement.  The focused test covers absent source text, model text returned for an absent side, a cited summary, an empty summary for an existing argument, and an uncited summary for an existing argument.

- [x] Implement and document absent-side handling.
- [x] Run the focused report, prompt, and CLI tests.
- [x] Complete the live Pi/MCP test, inspect its digest, and verify the certificate.
- [x] Commit and push the correction on `main`.

The live run uses the unchanged invoice scenario and participant settings from the preceding tests, with a fresh output directory at `adc/out/import-live-20260918-04/`.  The model-side file-copying observation remains separate and unresolved.

The live test exited with status zero after two imports, a technical report, two productions, the transition to trial, and digest generation.  The digest contains a cited plaintiff summary and the fixed absence statement for the defendant.  All nine certificate transitions verified.  Both uploaded files match their retained workspace originals.  The ADC runtime and launcher Go tests, report and prompt vet checks, and `git diff --check` passed.  This correction changes no Lean source or proof.

The lawyer recorded 32 completed responses and `$1.354129` in token-price estimates.  The digest request recorded 819 input tokens and 300 output tokens without a dollar-cost observation.  The retained directory occupies 4.1 MiB.  Staged authentication and MCP configuration files are absent, and Podman reports zero running containers.

Digest review found a separate wording error: the plaintiff summary calls the two imported documents "exhibits," while the same digest records zero admitted exhibits.  The absent-side correction leaves that model-generated wording unchanged.  This observation and the earlier model-side file-copying error remain open before the Israel–Syria rerun.

## ADC digest evidence status and file transfer: 2026-09-18

The digest's summary input now includes registered-file statuses, production events, exhibit entries, and explicit file and admission counts.  Its system prompt distinguishes registration, disclosure, and exhibit rulings, and attributes verification work to the reporting participant.  The fallback prompt carries the same instructions.  A focused test checks imported and produced files with no exhibits, then admitted and excluded exhibit entries.  The manual describes the summary input.

- [x] Update the summary input, prompts, documentation, and focused test.
- [x] Run ADC runtime, launcher, and CLI Go tests, and report and prompt vet checks.
- [x] Complete the live Pi/MCP test and inspect its digest.
- [x] Commit and push the digest correction.

The live test at `adc/out/import-live-20260918-05/` stopped after both imports and the technical report.  Pi returned `Codex error: Unable to verify Daybreak Blue access. Please try again.`  Six court actions were recorded, including the three deterministic setup actions.  The lawyer log contains 16 successful assistant responses and one error response, with `$0.574791` in token-price estimates.  Both uploaded files match their workspace originals.  No case container or staged authentication or MCP configuration file remains.  The [official authentication documentation](https://developers.openai.com/codex/auth) does not explain the provider's error.  One unchanged retry uses `adc/out/import-live-20260918-06/`.

Inspection of the installed Pi MCP adapter, version 2.27.0, found that its script interface exposes MCP calls but no filesystem access.  It therefore cannot read and upload workspace bytes through that interface.  File transfer still requires a design choice.  The proposed `adc-mcp import-file --file PATH` command would read the file on the participant's machine and submit the existing import decision through MCP.  A court-side workspace mapping would cover locally launched participants only.  The user has been asked to choose.  No transfer mechanism has changed.

The retry exited with status zero after two imports, a technical report, both productions, the trial transition, and digest generation.  The plaintiff summary identifies the documents as case files.  The digest records zero admitted exhibits and the absence of defense argument text.  Both uploads match their workspace originals byte for byte.  All nine certificate transitions verified under local leanrunner limits: 4 GiB memory high, 6 GiB maximum, 1 GiB swap, 100% CPU, and a 900-second timeout.  The command was `adc/.bin/adc verify-certificate --dir adc/out/import-live-20260918-06/adc-output`, run from the repository root.  It used the existing compiled engine.  No Lean source or proof changed.

The retry recorded 22 completed lawyer responses with `$1.051705` in token-price estimates, making `$1.626496` across these two attempts.  The digest request used 903 input tokens and 214 output tokens without a dollar-cost observation.  The failed and successful runs occupy 2.5 MiB and 3.2 MiB.  No case process, container, or staged authentication or MCP configuration file remains.

Review also found that the digest's external-activity and Bash sections omit Pi activity.  They read `custom_method` and `agent_tool_call` entries in core turn transcripts, whereas these MCP lawyer turns contain decision acceptance and legal actions.  The Pi session and process logs retain the tool calls.  The resulting "No external role activity recorded" statement is misleading for this run.  This reporting issue remains open separately from evidence-status wording.

## ADC participant file-upload command: 2026-09-18

The user approved `adc-mcp import-file --file PATH`.  It reads the participant's file, base64-encodes its bytes, and submits the existing `import_case_file` decision through that participant's MCP capability.  The command returns registered file metadata as JSON.  It supports an optional label, an explicit MCP URL and capability file, environment-provided connection settings, and a configurable timeout.  The existing 4 MiB MCP request limit includes the encoded file and JSON overhead.

The client follows the MCP 2025-06-18 [HTTP transport](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports) and [initialization sequence](https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle) used by adj's JSON-response server.  It initializes a session, sends the initialized notification, calls one tool, and deletes the session.  Errors return to the caller.  The client sends each request once.  After an interrupted upload response, the caller must inspect the case before deciding whether another submission is needed.

The ADC launcher supplies its configured MCP executable to automatic lawyers.  Native processes receive its absolute path.  Pi and OpenClaw receive a read-only executable mount.  `ADJ_MCP_COMMAND`, `ADJ_MCP_URL`, and `ADJ_MCP_BEARER_TOKEN` provide the command path and existing assignment credentials.  The local participant prompts, remote-lawyer instructions, import descriptions, compiled fallbacks, manual, and prompt-authoring guide describe the command and opportunity sequence.  No dependency was added.

- [x] Implement the command and launcher support.
- [x] Update prompts and documentation.
- [x] Test MCP client calls against the shared server, including a tool error, and test Pi command mounting and environment delivery.
- [x] Run ADC Go tests, all formal local-runner tests, shared launcher tests, and affected vet checks.
- [x] Complete the live Pi/MCP upload test and compare each stored file with its workspace original.
- [x] Verify the certificate, inspect process and credential cleanup, and record usage.
- [x] Commit and push on `main`.

The live test uses the unchanged invoice scenario and Pi lawyer settings with output under `adc/out/import-live-20260918-07/`.  The summary generator's separate Pi-activity omission remains outside this file-transfer change.

The test exited with status zero.  The lawyer invoked the command twice through Bash, importing a CSV calculation table and a Markdown explanation.  Both stored files match their workspace originals byte for byte.  The two upload clients deleted their MCP sessions.  The lawyer then filed its report and produced both files, and the court advanced to trial and generated its digest.  Certificate verification passed all nine transitions through local leanrunner: 4 GiB memory high, 6 GiB maximum, 1 GiB swap, 100% CPU, and a 900-second timeout.  The command was `adc/.bin/adc verify-certificate --dir adc/out/import-live-20260918-07/adc-output`, run from the repository root.  It used the existing compiled engine.  No Lean source or proof changed.

The run recorded 22 completed lawyer responses and `$0.705226` in token-price estimates.  Its digest request used 979 input tokens and 266 output tokens without a dollar-cost observation.  The retained directory occupies 2.7 MiB.  No case process, container, or staged authentication or MCP configuration file remains.

Live model testing covered Pi.  The shared launcher supplies the command to the other runners, with unit coverage for the container mount and environment.  The installed Codex CLI is 0.155.0.  Its [documented shell environment policy](https://developers.openai.com/codex/config-advanced#shell-environment-policy) preserves token-named variables by default, so this implementation leaves its settings unchanged.

## ADC Pi activity reporting: 2026-09-18

The report read legacy tool events from core turn transcripts, while Pi's native tool events remained in process logs.  External and direct-model opportunity turns also shared the same decision-acceptance shape.  External Role API turns now carry an explicit `external` field, allowing reports to identify their submissions without misclassifying direct-model decisions.

After successful execution and participant shutdown, the ADC launcher appends a Pi activity section to the existing digest.  The report parses `tool_execution_start` and `tool_execution_end` events from closed logs, identifies each source, counts results by tool, and renders Bash excerpts.  It excludes thinking, work-note contents, and MCP response bodies.  It makes no additional model request and assigns no inferred court turn.  The parser returns read and format errors.  Native tool activity remains outside the Lean state and replay certificate.

- [x] Add external-turn identification and Pi activity reporting.
- [x] Document report contents and limits.
- [x] Run affected Go tests and review the changes.
- [x] Run the live Pi/MCP case, inspect its reports, and verify its certificate.
- [x] Record usage, cleanup, and results.

The live Pi/MCP test under `adc/out/import-live-20260918-08/` completed two imports, a report, two productions, the transition to trial, and digest generation.  The transcript and digest identify the five external filing turns.  The digest's native activity counts match all 21 tool calls in the Pi log: two Bash calls, three writes, and sixteen MCP calls.  Its Bash excerpts contain the arithmetic program's output and both upload commands.  The final `wait_for_opportunity` returned `fetch failed` after the final production.  The report counts that failed result.  The launcher cancels MCP on case completion.  Both uploaded documents match their workspace originals byte for byte.

Certificate verification passed all nine transitions using the existing compiled engine through local leanrunner, from the repository root: `adc/.bin/adc verify-certificate --dir adc/out/import-live-20260918-08/adc-output`.  Limits were 4 GiB memory high, 6 GiB maximum, 1 GiB swap, 100% CPU, and a 900-second timeout.  ADC runtime, local-runner, and command Go tests passed, as did affected vet checks.  No Lean source or proof changed.

The test recorded 21 completed lawyer responses and `$0.750207` in token-price estimates.  The digest request used 1,172 input tokens and 252 output tokens without a dollar-cost observation.  The retained directory occupies 2.8 MiB.  No test process, container, or staged authentication or MCP configuration file remains.  Live coverage used one Pi lawyer.  Juror log selection is implemented but has not received a live jury test in this change.

## Israel–Syria ADC study: 2026-09-18

The full Israel–Syria case started at 22:28:34 UTC under `adc/out/ex04-20260918T223000Z/`, using source revision `0e2a080` and rebuilt Go commands.  The approved settings remain nine jurors, six required votes, preponderance of the evidence, 25-minute turn limits, default voir dire, and the installed 97-configuration pool.  Both Pi lawyers use `openai/gpt-5.6-sol` with `xhigh`, Codex subscription authentication, and enabled search.  Their separate state directories begin with fresh sessions.  The initial document matches `examples/ex04/market-rules.md`.  Earlier case filings and results are absent from the inputs.

Both lawyers completed their first technical reports by 22:57 UTC, and interrogatories began at 22:58.  The plaintiff imported a source-analysis document as `file-0002`.  Its stored copy matches the workspace original.  The defendant imported the extracted text of Israel's UN letter S/2024/887 as `file-0003` and retained the official PDF in its workspace.  Research encountered blocked pages, empty downloads, and unavailable commands, but both lawyers obtained source text through subsequent tool calls.  No runtime or prompt changed during this case.  The report draft is under `reports/marxiv/israel-syria-adc/` and remains incomplete pending judgment and verification.

Discovery produced six registered files, including both lawyers' source archives and the official Israeli letter PDF.  The plaintiff decoded and extracted the defense archive with local tools, obtaining Syria's contemporaneous letter and the later UNDOF report.  Both sides answered five interrogatories, their document requests, and all admission requests: 34 served by the plaintiff and 30 by the defense.  They declined Rule 37 motions.  At 23:27:53 UTC, the judge denied the plaintiff's Rule 56 motion because the record permitted competing reasonable inferences concerning offensive character and intent at commencement.  Those issues remain unresolved.  The current draft covers the proceeding through that ruling.

- [ ] Observe the case through judgment and record failures as they occur.
- [ ] Inspect the verdict, jury selection, research, filings, notes, and accounting.
- [ ] Verify the replay certificate and participant cleanup.
- [ ] Complete the standalone ADC technical report and review its quotations and claims.
- [ ] Submit the PDF to marXiv and follow editorial review.
- [ ] Commit and push the completed report and documentation.

The case stopped at 23:33 UTC during jury selection.  J2 and J4 ended their Pi sessions without submitting questionnaire answers.  The launcher reported J2's exit but suppressed J4's because its failure map used only the state-local opportunity ID `o1`.  Process matching, model bindings, and directory names had the same identity defect.  The fix uses the existing state version together with principal and opportunity ID.  The Role API exposes the version and checks supplied identities on submissions and failure reports.  The MCP adapter sends both fields.  No Lean rule or proof changes are required.

Shutdown also reported missing container ID files after completed Pi processes.  Podman removes those files when it removes the container, as documented under [`--cidfile`](https://docs.podman.io/en/stable/markdown/podman-run.1.html).  Cleanup now permits an absent ID file for an already exited client while retaining process and finalization errors.  No container remained after the stopped case.

- [x] Test repeated opportunity IDs, stale submissions, and successive live Pi juror turns.
- [x] Commit the verified runtime correction and restart the full case with fresh lawyer sessions.

The live Pi/MCP test `adc/out/juror-turns-20260918-08/` completed six questionnaires and two further answers by J1.  All eight opportunities used `o1`, while J1's three processes used state versions 16, 22, and 23.  Replay verification passed 24 transitions through local leanrunner with its default limits.  No container or staged secret remains.  Setup attempts exposed two fixture errors (missing court profile and insufficient candidate panel), an eight-turn limit that prevented finalization, and a runtime copy function that converted an empty string slice to JSON `null`.  That copy now preserves an empty list.  A focused test covers it.  ADC runtime, launcher, command tests, and affected vet checks pass.

The successful test also exposed incomplete juror activity reporting: stopping a process removed the process record used to locate its log.  The launcher now retains the log metadata independently of live process records.  The full-case run will verify that all juror logs appear in the digest.  The stopped Israel–Syria case recorded 366 lawyer responses and about $134.71 in token-price estimates, excluding search subrequests and court-model requests.

Revision `543624c` was committed and pushed to `origin/main`.  After rebuilding the Go commands, the fresh full case began at 00:34:39 UTC on September 19 under `adc/out/ex04-20260919-01/`.  The approved settings and original market document are unchanged.  The new agent-state directory gives both lawyers fresh sessions.  The earlier draft is retained as `report-draft.tex` beside the stopped case.  The manuscript under `reports/marxiv/israel-syria-adc/` now follows the new run.

Both technical reports were filed by 00:59 UTC.  The plaintiff's stored assessment matches its workspace copy.  Discovery added the two official UN-letter PDFs, five interrogatories from each party, document production, and admission requests.  The plaintiff's answers narrowed its theory to the ground advance and retention of positions, with the strike campaign as context.  Both parties disclosed the absence of original geospatial analysis, examined witnesses, and internal operational orders.  The plaintiff's technical-report research contained a thirteen-minute, forty-one-second gap between submitted work notes despite continuing tool activity.  The report records that limitation.  No runtime, prompt, pool, or case input has changed during this run.

At 01:33:46 UTC, the judge denied the plaintiff's Rule 56 motion because the admitted conduct supported competing reasonable inferences about offensive character and intent.  The plaintiff corrected a reply that exceeded the 4,000-character limit.  Both sides declined Rule 37 motions and the defense declined a Rule 56 cross-motion.  Jury selection began with thirteen candidates for nine seats.  Gemma 4 31B through Crusoe returned two upstream 429 responses during screening before its configuration was assigned to J10.

Review during jury setup found that case generation fixes `loop_policy.max_turns` at 180 (`adc/runtime/casegen/casegen.go`).  `runAutopilot` returns an error if the case reaches that limit before judgment (`adc/runtime/runner/autopilot.go`).  Thirteen questionnaires and one allowed question per side per candidate can consume 91 opportunities, putting this case at risk of exhausting the limit during trial.  The running process cannot reload it, and the CLI provides no case continuation.  The user has been asked whether to stop, increase the generated limit to 500, and rerun.  The current case remains unchanged pending that decision.

At 01:39 UTC, J8 (DeepSeek V3.2) and J9 (Qwen3 30B A3B Instruct) exited after local model-gateway errors.  The launcher handled both failures and assigned J14 (MiniMax M2.7) and J15 (GLM-5 Turbo), confirming that separate failures sharing `o1` no longer suppress each other.  The event name `agent_timeout` also covers these process exits.  Neither event represents an elapsed 25-minute deadline.

J8's third assistant tool-call message contained only two newline characters as text.  Pi removes whitespace-only assistant blocks in its OpenAI Chat Completions conversion.  The gateway normalizes only empty strings and null, so its prefix comparison rejected the next request as `chat request changed prior message 6`.  J9's streamed arguments were `{"tool": "adc_wait_for_opportunity", "args": {}"}`.  Pi's streaming JSON parser repaired that into an object and the MCP read succeeded.  The gateway then tried to parse the original malformed argument string while comparing the next request's history and rejected the continuation.  The relevant gateway functions are `checkMessagePrefix` and `historyMessageJSON` in `common/modelgateway/server.go`.  The installed Pi source documents its behavior in `pi-ai/dist/providers/openai-completions.js` and `pi-ai/dist/utils/json-parse.js`.  No gateway change has been made during this case.

The case was stopped at 01:44:30 UTC after confirming those gateway defects.  J10 had also encountered repeated upstream rate limits.  Seven questionnaire responses had been accepted, and the case had reached opportunity 67.  It produced no verdict or completed replay certificate.  Its records remain under the same run directory.  No container or staged authentication/MCP configuration file remains.  The lawyer logs contain 440 completed assistant responses and $127.05 in token-price estimates, excluding court requests and search subrequests.  The report remains an incomplete draft and has not been submitted.  Approval is pending for correction of the gateway compatibility behavior, a higher turn limit, and another full run.
