# Development Notes

## 2026-08-09: Persona corpus restoration

The core runtime retained the shared persona loader, default request-spec pool, and generic persona during repository extraction.  This change restores the 17 attorney prompts, 21 philosopher prompts, and 14 synthetic-person prompts formerly stored under `common/etc/personas`.  The restored paths let custom ADC juror and ARB or AARD council pools select the prompts through the existing loader.

The former `common/etc/personas.csv` combined the 14 synthetic people with 182 historical model identifiers per person, while `evals/model-pool/personas/experiments` duplicated the canonical prompt files.  This restoration preserves the canonical texts and documents their request-spec paths without restoring generated model inventories or evaluation output.  The distributed default pool remains unchanged.

### Verification

- [x] Compare all 52 restored files with combined-repository commit `1f62a56f66da3a476a7f4064a86a580a2970fadc`.
- [x] Load one restored attorney, philosopher, and synthetic-person prompt through the shared persona package.
- [x] Run the shared persona package tests.
- [x] Check the repository diff and the new Markdown link.

## 2026-08-08: Independent repository

The `adj` repository derives from `adjudication` commit `9cc03c1cd62132bed964acfb1d046c234d90b253` on its `carve` branch.  The initial snapshot preserves the ADC, ARB, and AARD procedure implementations, Lean proofs, one-case command-line programs, shared core packages, tests, examples, and documentation.  The completed branch-extraction plan and retention ledger remain in the source repository, where their branch-specific history can be read in context.

The Go module path changed from `adjudication` to `github.com/jsmorph/adj`, with every internal import changed accordingly.  The companion operational code derives from `adjudication` commit `26a7911599f8f1d947dbee7ecce722155f05024c` and now lives in `adjservices`.  Cross-repository verification builds fresh commands from both repositories and runs the service compatibility packages against the corresponding core executables and Lean engines.

The first race-enabled Go suite encountered a signal-7 crash in Lean 4.32.0's bundled `clang` while a runner test caused an on-demand `adcengine` build.  The Lean runner then built `adcengine` successfully under its verified local limits, and the exact failed Go test passed.  The complete race-enabled Go suite passed after the engine build, while all three proof libraries and executable targets built successfully through the same runner.

### Verification

- [x] Run `gofmt` over every Go source file.
- [x] Run `go test -race -buildvcs=false -count=1 ./...`.
- [x] Run `go vet ./...`.
- [x] Build the ADC, ARB, and AARD Lean proof trees with Lean 4.32.0.
- [x] Build `adcengine`, `aarengine`, and `aardengine` with Lean 4.32.0.
- [x] Run the `adjservices` compatibility packages against fresh `adj` and `adjservices` binaries.
