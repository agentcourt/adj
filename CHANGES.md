# Changes

## August 8, 2026

### Independent core repository

The ADC, ARB, and AARD core source was copied from `adjudication` commit `9cc03c1cd62132bed964acfb1d046c234d90b253` into the independent `adj` repository.  The Go module path is now `github.com/jsmorph/adj`, and operational programs remain in the separate `adjservices` repository.  The repository starts with the three procedure trees, their shared core packages, formal proofs, examples, tests, manuals, and process-interface documentation.

## August 2, 2026

### Core and service separation

The `carve` branch of the former combined repository established the ADC, ARB, and AARD procedural cores.  Each procedure retained its Lean engine, proofs, one-case Go runtime, private participant APIs, durable records, certificate verifier, command-line program, tests, and governing documentation.  The companion `service` branch received multi-case process management, MCP adapters, local participant launchers, web programs, attested execution, and deployment.

The branches communicate through installed core executables, private case HTTP APIs, and documented record formats.  Service imports no core implementation package, and core imports no service package.  Immutable commit pairs and cross-branch process tests record compatibility.

### Lean 4.32.0

ADC, ARB, AARD, and the former VMCP experiment were moved to Lean 4.32.0 before auxiliary material was removed.  The retained proof trees now elaborate under the 4.32.0 `do` elaborator and use explicit reductions where the earlier compiler reduced terms implicitly.  Each retained engine and proof tree built successfully at the migration commit.

### Core reduction

The core extraction removed eval programs, provider inventories, pool-generation experiments, saved research material, scratch files, skills, VMCP, web programs, and operational agent support.  ADC now exposes only its complaint, scenario, packet, record, validation, adjudication, and replay commands.  ARB and AARD expose the corresponding one-case command sets.

Shared core code now consists of model request parsing, model access, persona loading, proof-document tools, and the small runtime pool used by direct juror and council execution.  The Go module names the model client and SQLite as its direct third-party dependencies.  ARB uses the shared pool and persona instead of retaining byte-identical procedure-local copies.  The remaining package inventory contains only `adc`, `arb`, `arbd`, and `common` core packages.

ADC retains `examples/ex1`, ARB retains `examples/ex01`, and AARD retains `examples/ex1`.  These compact fixtures cover each procedure's complaint and case-file input without preserving duplicate scenarios or downloaded source collections.  Each example documents the core build, test, proof, validation or complaint-generation, one-case execution, durable records, and certificate check.
