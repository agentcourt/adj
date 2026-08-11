# Development Notes

## Repository Scope

This repository owns the ADC, ARB, and AARD procedures.  Each procedure includes its rules, Lean engine and proofs, one-case Go runtime, command-line program, private participant API, durable record, examples, and tests.  Operational consumers use the documented process interface without importing procedure implementation packages.

The shared `common/` tree contains code required by more than one procedure.  New shared packages must have at least two current consumers and a narrower API than the code they replace.  Procedure-specific behavior belongs in its procedure tree.

## Error and Command Policy

Commands return every input, output, formatting, storage, HTTP, and shutdown error to their caller.  A command writes diagnostics to standard error, writes machine-readable results to standard output where its manual specifies them, and exits nonzero when command execution fails.  Every command rejects unexpected positional arguments and reports help-output failures.

Long-running case commands derive their execution context from interrupt signals.  Startup output failures cancel the case before the command returns.  Case APIs and runtime helpers preserve the original operation error together with any cleanup error.

## Interface Maintenance

The [core process interface](docs/service-interface.md) defines the executable, private HTTP, and durable-record behavior consumed by `adjservices`.  An interface edit requires corresponding test and documentation edits in both repositories.  The paired interface tests use explicit core binaries and an explicit core checkout.

## Verification

Verification begins by building each procedure's engine and proof targets with Lean 4.32.0 through the configured resource-limited runner.  The complete Go suite runs after the engine builds because an ADC integration test executes the engine.  Go verification also includes `go vet ./...` and builds of the three command packages, while documentation verification checks every relative Markdown link against the repository tree.

- [x] Run the complete Go test suite.
- [x] Run the complete Go vet suite.
- [x] Build the ADC, ARB, and AARD commands.
- [x] Build the three Lean engines and proof trees.
- [x] Run the paired `adjservices` interface tests.
- [x] Verify relative Markdown links.
- [x] Repeat the builds and paired tests from clean Git archives.
