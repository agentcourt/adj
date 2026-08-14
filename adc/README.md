# Agent District Court

Agent District Court (ADC) implements civil adjudication with a Lean rule engine and a Go runtime.  The engine validates procedural state transitions under the Agent Rules for Civil Procedure.  The runtime prepares cases, obtains role decisions, stores the record, and writes replay material.

ADC accepts either a complaint or a scenario JSON file.  Complaint intake produces a normalized one-claim case, private party strategies, and a generated scenario before adjudication begins.  A scenario can instead define deterministic turns or model-driven roles directly.

The command can handle roles through direct model calls or expose plaintiff, defendant, and juror opportunities through its HTTP Role API.  The process retains ownership of the Lean state, deadlines, validation, case-file visibility, and final record in both modes.  Agent launchers, MCP adapters, Clerk services, attestation, and deployment material live in the `adjservices` repository.

## Documentation

The manual documents the command-line interface, Role API, records, and replay verification.  The practice guide describes work performed by each procedural role.  The rules state the civil procedure enforced by the runtime and Lean engine.

| Document | Use |
| --- | --- |
| [Agent District Court Manual](manual.md) | Commands, Role API, records, verification, and failure diagnosis. |
| [Agent District Court Practice Guide](docs/practice.md) | Pleadings, discovery, evidence, trial, and deliberation. |
| [Agent Rules for Civil Procedure](docs/ARCP.md) | Governing ADC procedure. |

## Requirements

ADC builds with Go 1.25 and Lean 4.32.0.  The Lean build uses `lake`.  The Makefile supplies the standard build, test, proof, and example targets.  The `ex1` acceptance fixture also requires OpenSSL to generate its linked signature inputs.

## Build

Run the build from `adc/`.  It writes the command to `.bin/adc` and the Lean engine to `.bin/adcengine`.  The test and proof targets check the Go runtime and Lean proof tree separately.

```bash
make build
make test
make prove
```

## Command-Line Use

The complaint path uses model calls for complaint drafting, intake, strategy preparation, and procedural roles.  It therefore requires the OpenAI-compatible credentials accepted by the shared model client.  Use `adc help` or `adc help COMMAND` for the complete current flag set.

```bash
examples/ex1/sign.sh
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md

.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1
```

A deterministic scenario can run without model access when every turn specifies its action.  The `--offline` flag enforces that condition.  The command writes each requested record to the supplied path.

```bash
.bin/adc scenario \
  --scenario PATH/TO/scenario.json \
  --offline \
  --output out/scenario/run.json \
  --runtime out/scenario/runtime.json \
  --events out/scenario/events.ndjson \
  --db out/scenario/run.db
```

## Role API

`adc case` and `adc scenario` can expose selected roles with repeated `--external-role` flags and `--caseapi-addr`.  External clients wait for an opportunity, inspect the role-visible record, submit work notes, and submit one permitted legal decision through `/roleapi/v1`.  The [manual](manual.md#role-api) defines the endpoints and request shapes.

## Repository Layout

The ADC directory contains the complete procedure-specific implementation.  Shared model, record, and diagram code remains under the repository-level `common/` directory.  The table identifies the principal ADC components.

| Path | Purpose |
| --- | --- |
| `engine/` | Lean rule engine, proofs, and Lake project. |
| `runtime/` | Go command, case preparation, runner, Role API, reports, and storage. |
| `etc/` | Court profiles. |
| `examples/` | Example case inputs. |
| `docs/` | Rules, practice material, proof notes, and procedure analysis. |
| `analysis/` | Procedure and state diagrams. |

## Records

A complaint-driven run writes `normalized-case.json`, party strategies, and `generated-scenario.json` before adjudication.  The adjudication record includes `case-manifest.json`, `run.json`, `state.json`, `certificate.json`, `runtime.json`, `events.ndjson`, `run.db`, `transcript.md`, `digest.md`, and `work-notes.ndjson`.  `adc verify-certificate` replays the accepted Lean transitions and compares the result with the recorded terminal state.

## License

The repository-level [MIT License](../LICENSE) covers the software.  [Notices](../NOTICES.md) contain the trademark and related terms.  Those files govern the ADC sources in this directory.
