# Development Notes

## Repository Scope

This repository owns the ADC, ARB, AARD, simple, and quick procedures.  Each procedure includes the rules, one-case Go runtime, command-line program, durable record, and tests that its design requires, while ADC, ARB, and AARD also include Lean engines and proofs.  Operational consumers use the documented process interface without importing procedure implementation packages.

The shared `common/` tree contains code required by more than one procedure.  New shared packages must have at least two current consumers and a narrower API than the code they replace.  Procedure-specific behavior belongs in its procedure tree.

## Error and Command Policy

Commands return every input, output, formatting, storage, HTTP, and shutdown error to their caller.  A command writes diagnostics to standard error, writes machine-readable results to standard output where its manual specifies them, and exits nonzero when command execution fails.  Every command rejects unexpected positional arguments and reports help-output failures.

Long-running case commands derive their execution context from interrupt signals.  Startup output failures cancel the case before the command returns.  Case APIs and runtime helpers preserve the original operation error together with any cleanup error.

## Interface Maintenance

The [core process interface](docs/service-interface.md) defines the executable, private HTTP, and durable-record behavior consumed by `adjservices`.  An interface edit requires corresponding test and documentation edits in both repositories.  The paired interface tests use explicit core binaries and an explicit core checkout.

## Supervised AAR Calls

The AAR command accepts `--council-request-attempts` so a supervising service can own retry policy without multiplying provider calls inside one case attempt.  The default remains four attempts for existing callers, while a supervisor can select one.  Direct provider failures carry stable classes for transient, authentication, request, and response-protocol failures, which removes the need for consumers to infer severity from error text.

The AAR command also accepts `--required-votes` together with its existing council-size and evidence-standard overrides.  A supervising service can therefore apply one common council configuration to AAR and quick adjudication without creating a temporary policy file.  The runtime applies all three overrides before policy validation and council sampling.

`run.json` and the command summary report `council_cost_usd` from successful OpenRouter generation metadata lookups.  The total includes council preflight and voting calls made by the direct council client.  A failed provider call can incur a charge that the generation lookup does not recover, so the reported amount can understate provider billing after a failed request.

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
- [x] Add manifest creation to ADC, ARB, and AARD.
- [x] Document the paired discovery interface in `adj` and `adjservices`.
- [x] Repeat complete core and paired-interface verification.

## Simple Adjudication

The `simple` procedure decides one proposition through one direct provider request and requires a structured `demonstrated` or `not_demonstrated` response with a rationale.  The caller supplies an evidence standard, explicit document limits, one request specification, and explicit permission to use API-key billing.  The procedure neither retries the provider nor invokes a lawyer, council, or Lean engine.

`common/documents` imports regular files in bytewise path order, rejects symbolic links and source changes, and records byte counts, media types, and SHA-256 hashes.  Its verified reader confines each path to the imported root and checks type, size, and digest before returning the bytes to a procedure.  `common/recordio` supplies the JSON, atomic JSON, and append-only JSON-line operations used by the new procedures, while the simple record excludes document contents, encoded media, and request-header values from `model-request.json`.

Focused tests cover document import, record replacement, command argument handling, input-media construction, one-request success, typed provider failure, malformed tool output, and failure records.  Race tests, Go vet, and a command build also passed.  Verification used fake provider clients and made no external provider calls.

## Quick Adjudication

The `quick` procedure gives a proponent and an opponent one sequential argument each, then asks a selected council to vote through direct provider requests.  The opponent receives the proponent's argument, and neither lawyer receives a rebuttal or closing opportunity.  The procedure requires a strict-majority threshold, an evidence standard, explicit document limits, and explicit permission to use API-key billing.

The core samples council records without replacement through `crypto/rand`, giving every remaining pool record equal probability at each seat.  A selected record cannot occupy a second seat in the same case, and selecting the full pool includes every record once.  The sampler accepts an internal random-index function so tests can fix the selection sequence and return entropy errors without replacing production randomness.

The private lawyer API uses the AAR lawyer paths and tool shapes so `adjservices` can present either procedure through the same MCP adapter.  Each lawyer can inspect immutable staged documents through list, stat, and range-read operations, and the shared verified reader rejects a changed file before returning its bytes.  The council prompt contains the two accepted arguments and verified document contents, and each selected member must submit one structured `demonstrated` or `not_demonstrated` vote with a rationale.

Production startup initializes every distinct selected council endpoint before the case API opens, which detects a missing credential or unsupported endpoint without sending a provider request.  The durable record contains resolved input, runtime identity, document metadata, ordered events, private lawyer work notes, both arguments, council votes, provider response identifiers, recovered cost, and the terminal result.  Request headers and query parameters remain in memory and do not enter council metadata or other durable records.

Council requests run sequentially by default.  The optional parallel mode starts the selected requests together, cancels outstanding request contexts after the first observed failure, waits for every started request to return, and records successful votes in roster order.  `input.json` and the `council_started` event record the resolved mode.

Focused tests cover lawyer-turn deadlines, cancellation, concurrent submissions, durable event order, document verification, majority results, complete pool validation before sampling, endpoint tool support, explicit billing authorization, typed protocol errors, error redaction, terminal shutdown and response-write failures, complete early command results, and short output writes.  Council-mode tests verify sequential execution by default, concurrent starts, roster-order persistence after reverse-order completion, and outstanding-request cancellation after a provider failure.  Focused ordinary and race tests, Go vet, a command build, repeated concurrency tests, and the diff check passed; verification used fake provider clients and local HTTP calls and made no external provider request.

## ADC Proposition Input

`adc case` accepts exactly one complaint or proposition.  Proposition setup creates `Proponent v. Opponent` in the programmatic Proposition Tribunal, assigns the burden to the Proponent, and requests a declaration whether the proposition has been demonstrated.  The Tribunal accepts `proposition_adjudication` jurisdiction without subject-matter screening, and `--trial-mode=auto` selects a jury.

The caller selects either `preponderance_of_the_evidence` or `clear_and_convincing` because the Lean engine accepts those two claim standards.  Setup imports an optional document tree through `common/documents`, records the manifest, verifies each imported file before scenario construction, and preserves the manifest size and SHA-256 values in the case attachments.  The runtime verifies the formal attachment path, regular-file type, exact size, and SHA-256 before it exports evidence, returns bytes through the role API, or adds file contents to a model prompt.  Positive count, per-file, and total-byte limits remain required when the document tree is absent.

Proposition setup constructs the normalized claim, both role strategies, and the scenario without an intake or planner model request.  The existing ADC runtime then handles pleadings, discovery, trial, verdict, and judgment with the selected direct or external roles.  Focused tests cover the Tribunal profile, deterministic case fields, accepted standards, document import and hash preservation, changed-document rejection, size drift, symbolic-link replacement, role API reads, model-prompt attachments, the proposition input flags, empty document records, and the jury default.

Proposition claims set `declaratory_only` to true, while an absent field defaults to false for existing civil claims.  The Lean engine rejects a positive damages amount at juror voting, monetary judgment, Rule 68, settlement, and final jury or bench disposition boundaries.  Focused race tests, Go vet, and the ADC engine and maintained proof builds passed with Lean 4.32.0 through the local resource-limited runner.

## Verification

Verification begins by building each procedure's engine and proof targets with Lean 4.32.0 through the configured resource-limited runner.  The complete Go suite runs after the engine builds because an ADC integration test executes the engine.  Go verification also includes `go vet ./...` and builds of the three command packages, while documentation verification checks every relative Markdown link against the repository tree.

- [x] Run the complete Go test suite.
- [x] Run the complete Go vet suite.
- [x] Build the ADC, ARB, and AARD commands.
- [x] Build the three Lean engines and proof trees.
- [x] Run the paired `adjservices` interface tests.
- [x] Verify relative Markdown links.
- [x] Repeat the builds and paired tests from clean Git archives.
