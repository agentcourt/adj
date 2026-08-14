# Core Process Interface

## Scope

This document defines the process interface supplied by the core ADC, ARB, and AARD procedures.  Core owns each case's procedural state, participant opportunities, evidence custody, durable record, and terminal result.  `adjservices` owns multi-case admission, process supervision, public routing, deployment, and record presentation.

An interface edit requires paired tests and corresponding documentation edits in both repositories.  Service and core changes may proceed together during first-release development.  The paired tests identify the exact core binaries and source tree under test.

## Executables and Process Behavior

Service invokes installed `adc`, `aar`, and `aard` executables.  Each invocation receives an explicit output directory and, for a live participant API, an explicit loopback listen address chosen by service.  Service captures standard output and standard error in the case output directory, sets an explicit working directory when the core installation still resolves resource defaults there, and supervises the process until exit.

A core process returns a nonzero exit status for startup, configuration, input, engine, storage, or other process failures.  A procedure that records an opportunity failure may return zero after writing a terminal failed case record.  The service parses the last nonempty JSON object on standard output as the process summary and reconciles the final status from `run.json`.

| Procedure | Direct one-case invocation | Required service-facing flags |
| --- | --- | --- |
| ADC complaint | `adc case` | `--complaint`, `--out-dir`, `--case-id`, `--run-id`, `--caseapi-addr`, and requested runtime overrides. |
| ADC scenario | `adc scenario` | `--scenario`, `--output`, `--runtime`, `--events`, `--db`, `--transcript`, `--digest`, `--report-model`, `--allow-assertion-failures`, `--case-id`, `--run-id`, `--caseapi-addr`, and requested runtime overrides. |
| ARB | `aar case` | `--case-id`, `--run-id`, `--complaint`, `--out-dir`, `--caseapi-addr`, `--council-backend`, and any requested policy or runtime override. |
| AARD | `aard case` | `--case-id`, `--run-id`, `--complaint`, `--out-dir`, `--caseapi-addr`, `--council-backend`, and any requested policy or runtime override. |

The service-owned `adc-run`, `aar-run`, and `aard-run` commands start a direct core case through this interface and use the corresponding service-owned MCP adapter.  `adc-run` selects `adc case` for a complaint and `adc scenario` for a prepared scenario.  Each launcher preserves the complete core `run.json` object, records core standard streams beneath `logs/`, and rejects a result file that the new process did not replace.

The `adj` repository retains `validate`, `verify-certificate`, and deterministic `case-packet` construction for operator and service use.  Attested drivers live in `adjservices`, invoke the installed core packet command, and identify the exact core source or artifacts placed in a workload image.  This keeps complaint and case-file selection under the procedure that later validates and runs those inputs.

## Private Case APIs

Service may reach a private case API only through its configured loopback address.  HTTP 204 from `/health` marks a child ready for participant traffic.  Service may proxy the procedure-specific Role API without interpreting or changing procedural requests and responses.

| Procedure | Paths owned by core |
| --- | --- |
| ADC | `/health`; `/roleapi/v1/get`; `/roleapi/v1/wait_for_opportunity`; `/roleapi/v1/status`; `/roleapi/v1/result`; `/roleapi/v1/do`; `/roleapi/v1/fail`. |
| ARB | `/health`; `/lawyerapi/v1/get`; `/lawyerapi/v1/wait`; `/lawyerapi/v1/status`; `/lawyerapi/v1/result`; `/lawyerapi/v1/do`; `/councilapi/v1/get`; `/councilapi/v1/wait`; `/councilapi/v1/do`; `/councilapi/v1/fail`. |
| AARD | `/health`; `/lawyerapi/v1/get`; `/lawyerapi/v1/wait`; `/lawyerapi/v1/status`; `/lawyerapi/v1/result`; `/lawyerapi/v1/do`; `/councilapi/v1/get`; `/councilapi/v1/wait`; `/councilapi/v1/do`; `/councilapi/v1/fail`. |

Core validates case identifiers, principal identifiers, opportunity identifiers, legal tool calls, attempts, deadlines, file visibility, and evidence access.  Service selects a case process and forwards bytes, status, and relevant HTTP headers.  MCP adapters translate MCP requests to these APIs but hold no procedural state.

## Discovery Manifest

Each one-case command writes `case-manifest.json` atomically in its output directory when the run begins.  The command replaces the manifest after the private listener starts so `case_api_base` contains the address that the operating system assigned.  A filesystem observer can discover the directory before terminal artifacts exist and can test the `/health` route beneath the recorded base URL for current liveness.

```json
{
  "schema_version": "adj.case-manifest.v1",
  "procedure": "arb",
  "case_id": "case-1",
  "run_id": "run-1",
  "started_at": "2026-08-14T12:30:00.000000123Z",
  "core_version": "v0.1.0",
  "case_api_base": "http://127.0.0.1:21345"
}
```

| Field | Meaning |
| --- | --- |
| `schema_version` | Manifest schema, currently `adj.case-manifest.v1`. |
| `procedure` | `adc`, `arb`, or `arbd`. |
| `case_id` | Case identifier supplied to the core command. |
| `run_id` | Run identifier supplied to the core command. |
| `started_at` | UTC start time in RFC 3339 format with available fractional precision. |
| `core_version` | Go main-module version, or `(devel)` for an unversioned development build. |
| `case_api_base` | Bound private HTTP base URL.  ADC omits it when its case API is disabled. |

The manifest declares startup identity and addressing.  `run.json` records terminal status and result data, while `events.ndjson` records progress.  Failure to write or replace the manifest terminates the core command and returns the write error.

## Durable Record

Core writes the durable adjudication record beneath the selected output directory.  Service may list, read, range-serve, and render documented files without changing them.  The certificate verifier on core remains the authority for replaying accepted actions and comparing the replayed terminal state with the recorded state.

| File or directory | Owner and use |
| --- | --- |
| `case-manifest.json` | Core startup identity, version, start time, and bound private API address used for discovery. |
| `run.json` | Core terminal result and run metadata consumed by service status reconciliation. |
| `state.json` | Core terminal state consumed by verification and reporting. |
| `certificate.json` | Core accepted-action replay certificate. |
| `events.ndjson` | Core append-only case event stream used for monitoring. |
| `work-notes.ndjson` | Core off-record participant work notes when the procedure enables them. |
| `evidence-manifest.json` and `evidence-store/` | Core evidence identifiers, metadata, hashes, and stored bytes. |
| `transcript.md` and `digest.md` | Core human-readable record and summary. |
| `runtime.json` and `run.db` | ADC runtime configuration and database when the selected ADC path writes them. |
| `local-run.json` | Service-owned local-agent orchestration record after launcher extraction. |
| `service-logs/` and `clerk.json` | Service-owned process logs and multi-case record. |

A service must treat an unreadable or missing `run.json` after process exit as a failed or incomplete execution.  It may reconcile a detached service record from a readable terminal `run.json`.  Artifact access must confine paths to the recorded output directory and expose only the service's explicit allowlist.

## Interface Tests

Service unit tests use fake core executables to verify argument construction, startup, failure, process cleanup, record reconciliation, proxying, and artifact access.  Paired tests use built core executables and a specified `adj` source tree to verify command help, required flags, private API startup, one direct case per procedure, and terminal record consumption.  These tests run against the core and service versions intended for the same release.

The `service/compat/adc`, `service/compat/arb`, and `service/compat/arbd` packages in `adjservices` start the standalone Clerk and MCP executables against selected core binaries.  The ADC package completes a bench case with external parties, an internal judge, digest generation, terminal record reconciliation, and certificate replay.  The ARB and AARD packages cover participant failures, deadlines, complete MCP cases, terminal records, and service reconciliation.  All three packages accept explicit service binary, core binary, and core checkout paths through test flags.

Changes to command names, required flags, private routes, consumed request or response fields, exit behavior, or record names require coordinated edits.  The core and service documentation must describe the same interface.  The paired tests must pass before either repository is prepared for release.
