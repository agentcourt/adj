# Development Notes

## Repository Scope

This repository owns the ADC, ARB, and AARD procedures.  Each procedure includes its rules, Lean engine and proofs, one-case Go runtime, command-line program, private participant API, durable record, examples, and tests.  Operational consumers use the documented process interface without importing procedure implementation packages.

The shared `common/` tree contains code required by more than one procedure.  New shared packages must have at least two current consumers and a narrower API than the code they replace.  Procedure-specific behavior belongs in its procedure tree.

## Error and Command Policy

Commands return every input, output, formatting, storage, HTTP, and shutdown error to their caller.  A command writes diagnostics to standard error, writes machine-readable results to standard output where its manual specifies them, and exits nonzero when command execution fails.  Every command rejects unexpected positional arguments and reports help-output failures.

Long-running case commands derive their execution context from interrupt signals.  Startup output failures cancel the case before the command returns.  Case APIs and runtime helpers preserve the original operation error together with any cleanup error.

## Interface Maintenance

The [core process interface](docs/service-interface.md) defines the executable, private HTTP, and durable-record behavior consumed by `adjservices`.  An interface edit requires corresponding test and documentation edits in both repositories.  The paired interface tests use explicit core binaries and an explicit core checkout.

## Supervised AAR Calls

The AAR command accepts `--council-request-attempts` so a supervising service can own retry policy without multiplying provider calls inside one case attempt.  The default remains four attempts for existing callers, while a supervisor can select one.  Direct provider failures carry stable classes for transient, authentication, request, and response-protocol failures, which removes the need for consumers to infer severity from error text.

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

## Verification

Verification begins by building each procedure's engine and proof targets with Lean 4.32.0 through the configured resource-limited runner.  The complete Go suite runs after the engine builds because an ADC integration test executes the engine.  Go verification also includes `go vet ./...` and builds of the three command packages, while documentation verification checks every relative Markdown link against the repository tree.

- [x] Run the complete Go test suite.
- [x] Run the complete Go vet suite.
- [x] Build the ADC, ARB, and AARD commands.
- [x] Build the three Lean engines and proof trees.
- [x] Run the paired `adjservices` interface tests.
- [x] Verify relative Markdown links.
- [x] Repeat the builds and paired tests from clean Git archives.
