# Core Process Interface

## Scope

This document defines five procedure interfaces: simple, quick, AARD (`aard`, with service package `arbd`), AAR (`aar`, with service package `arb`), and ADC (`adc`).  Core owns each procedure's case state, input custody, participant opportunities where present, durable record, and terminal result.  `adjservices` owns procedure selection, external-participant launch, multi-case admission, process supervision, public routing, deployment, and record presentation.

An interface edit requires paired tests and corresponding documentation edits in both repositories.  Service and core changes may proceed together during first-release development.  The paired tests identify the exact core binaries and source tree under test.

## Executables and Process Behavior

Service invokes installed `adc`, `aar`, `aard`, `simple`, and `quick` executables.  Each invocation receives an explicit output directory and, for a live participant API, an explicit loopback listen address chosen by service.  The direct AAR and AARD services store child standard streams beneath their registry directories and leave the selected core output directory to the core process.  Service records the physical log paths, sets an explicit working directory when the core installation resolves resource defaults there, and supervises the process until exit.

A core process returns a nonzero exit status for startup, configuration, input, engine, storage, or other process failures.  A procedure that records an opportunity failure may return zero after writing a terminal failed case record.  The service parses the last nonempty JSON object on standard output as the process summary and reconciles the final status from `run.json`.

| Procedure | Direct one-case invocation | Required service-facing flags |
| --- | --- | --- |
| ADC complaint | `adc case` | `--complaint`, `--out-dir`, `--case-id`, `--run-id`, `--caseapi-addr`, and requested runtime overrides. |
| ADC proposition | `adc case` | `--proposition`, `--documents`, `--evidence-standard`, document limits, `--trial-mode`, `--out-dir`, `--case-id`, `--run-id`, and requested runtime overrides. |
| ADC scenario | `adc scenario` | `--scenario`, `--output`, `--runtime`, `--events`, `--db`, `--transcript`, `--digest`, `--report-model`, `--allow-assertion-failures`, `--case-id`, `--run-id`, `--caseapi-addr`, and requested runtime overrides. |
| AAR (`arb`) | `aar case` | `--case-id`, `--run-id`, `--complaint`, `--out-dir`, `--caseapi-addr`, `--council-backend`, and any requested policy or runtime override. |
| AARD | `aard case` | `--case-id`, `--run-id`, `--complaint`, `--out-dir`, `--caseapi-addr`, `--council-backend`, and any requested policy or runtime override. |
| Simple | `simple case` | `--proposition`, `--documents`, `--evidence-standard`, `--model` or `--request-spec`, `--reasoning-effort`, `--max-output-tokens`, `--max-tool-calls`, `--web-search=BOOL`, document limits, `--allow-api-key`, `--out-dir`, `--case-id`, and `--run-id`. |
| Quick | `quick case` | `--proposition`, `--documents`, `--evidence-standard`, `--lawyer-web-search=BOOL`, council size and vote threshold, optional `--council-pool` and `--common-root`, document limits, `--allow-api-key`, `--caseapi-addr`, `--lawyerapi-bearer-token-file`, `--out-dir`, `--case-id`, and `--run-id`. |

Simple's resolved web-search setting controls the provider-hosted Responses tool.  Its optional reasoning-effort, maximum-output-token, and maximum-tool-call flags override request-spec values, while omission preserves the request-spec or applicable default.  The tool-call limit applies to provider built-in tools across the response and leaves the custom decision function available.  Quick selects an explicit council pool, then a working-directory `pool.jsonl`, then the shared pool under `common`; it records the resolved path.  Quick selects search-enabled or search-disabled lawyer instructions and records the resolved setting, while an external lawyer client supplies the corresponding search capability.  Candidate preflight receives an availability prompt and the vote tool; the later vote receives the proposition, documents, arguments, and decision tool.  A missing endpoint credential excludes candidates for that endpoint, while a request-specific authentication failure advances to another candidate because request headers can differ.  The durable roster and candidate-rejection events record safe endpoint-variant and provider-routing fields.

The service-owned `adc-run`, `aar-run`, and `aard-run` commands start a formal core case through this interface and supervise the corresponding `adc-mcp`, `aar-mcp`, or `aard-mcp` command from this repository.  Each launcher owns its selected run directory and gives the core a procedure-specific child named `adc-output`, `aar-output`, or `aard-output`; component logs and `local-run.json` remain in the outer directory.  The launcher requires empty core and log children, rejects symbolic-link substitutions, and rejects a child whose directory identity changes during validation.  For a manual role, the launcher writes a mode-`0600` remote-lawyer skill containing that role's MCP capability and the common `/mcp` URL, treats the skill as a secret, and removes it during cleanup.  The unified `adjudicate` command supplies its record, core, and log directories to its AAR, AARD, and ADC adapters through the same launcher API, while quick and simple retain their separate supervised paths.  Each path preserves the complete core result and rejects a result file that the new process did not replace.

Each container-backed participant receives a private runtime cidfile.  Cleanup accepts only a complete 64-character lowercase hexadecimal container identifier from that file and asks the runtime to remove that exact identifier.  When no valid identifier exists, the launcher terminates and reaps the client and retries without removing a container by its requested name.

The `adj` repository retains `validate`, `verify-certificate`, and deterministic `case-packet` construction for operator and service use.  Attested drivers live in `adjservices`, invoke the installed core packet command, and identify the exact core source or artifacts placed in a workload image.  This keeps complaint and case-file selection under the procedure that later validates and runs those inputs.

## Private Case APIs

Service may reach a private case API only through its configured loopback address.  A successful `/health` response is HTTP 200 JSON containing the exact `case_id` and `run_id` for the child.  Service may proxy the procedure-specific Role API without interpreting or changing procedural requests and responses.

| Procedure | Paths owned by core |
| --- | --- |
| ADC | `/health`; `/roleapi/v1/get`; `/roleapi/v1/wait_for_opportunity`; `/roleapi/v1/status`; `/roleapi/v1/result`; `/roleapi/v1/do`; `/roleapi/v1/fail`. |
| ARB | `/health`; `/lawyerapi/v1/get`; `/lawyerapi/v1/wait`; `/lawyerapi/v1/status`; `/lawyerapi/v1/result`; `/lawyerapi/v1/do`; `/councilapi/v1/get`; `/councilapi/v1/wait`; `/councilapi/v1/do`; `/councilapi/v1/fail`. |
| AARD | `/health`; `/lawyerapi/v1/get`; `/lawyerapi/v1/wait`; `/lawyerapi/v1/status`; `/lawyerapi/v1/result`; `/lawyerapi/v1/do`; `/councilapi/v1/get`; `/councilapi/v1/wait`; `/councilapi/v1/do`; `/councilapi/v1/fail`. |
| Quick | `/health`; `/lawyerapi/v1/get`; `/lawyerapi/v1/wait`; `/lawyerapi/v1/status`; `/lawyerapi/v1/result`; `/lawyerapi/v1/do`; `/lawyerapi/v1/fail`. |
| Simple | No private HTTP API. |

Core validates case identifiers, principal identifiers, opportunity identifiers, legal tool calls, attempts, deadlines, file visibility, and evidence access.  Quick requires one bearer token on every `/lawyerapi/v1` request and leaves `/health` available to the supervisor.  The Quick core and `quick-mcp` read that separate token from owner-only files, while MCP clients receive signed role capabilities without the core token.  Service selects a case process and forwards bytes, status, and relevant HTTP headers.  MCP adapters translate MCP requests to these APIs but hold no procedural state.

## Discovery Manifest

Each one-case command writes `case-manifest.json` atomically in its output directory when the run begins.  The command replaces the manifest after the private listener starts so `case_api_base` contains the address that the operating system assigned.  A filesystem observer can discover the directory before terminal artifacts exist and can test the `/health` route beneath the recorded base URL, requiring matching case and run identifiers before treating the process as the manifest's live instance.

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
| `procedure` | `adc`, `arb`, `arbd`, `simple`, or `quick`. |
| `case_id` | Case identifier supplied to the core command. |
| `run_id` | Run identifier supplied to the core command. |
| `started_at` | UTC start time in RFC 3339 format with available fractional precision. |
| `core_version` | Go main-module version, or `(devel)` for an unversioned development build. |
| `case_api_base` | Bound private HTTP base URL.  Simple omits it, and ADC omits it when its case API is disabled. |

The manifest declares startup identity and addressing.  `run.json` records terminal status and result data, while `events.ndjson` records progress.  Failure to write or replace the manifest terminates the core command and returns the write error.

## Durable Record

Core writes each durable adjudication record beneath the selected output directory.  Service may list, read, range-serve, and render documented files without changing them.  ADC, ARB, and AARD provide certificate verifiers that replay accepted actions and compare the replayed terminal state with the recorded state.

| File or directory | Owner and use |
| --- | --- |
| `case-manifest.json` | Core startup identity, version, start time, and bound private API address used for discovery. |
| `run.json` | Core terminal result and run metadata consumed by service status reconciliation. |
| `state.json` and `certificate.json` | Formal-procedure state and accepted-action replay certificate.  Simple also writes a terminal state without a replay certificate. |
| `events.ndjson` | Core case event stream used for monitoring. |
| `work-notes.ndjson` or `work-notes.jsonl` | Participant work notes when the procedure enables them. |
| `evidence-manifest.json` and `evidence-store/` | Formal-procedure evidence identifiers, metadata, hashes, and stored bytes. |
| `documents.json` and `documents/` | Simple and quick imported-document metadata, hashes, and bytes. |
| `model-request.json`, `model-response.json`, and `decision.json` | Simple request controls, provider response, parsed decision, normalized web-search actions and sources, and URL citations. |
| `transcript.json` | Quick arguments and council votes. |
| `transcript.md` and `digest.md` | Human-readable records written by the procedures that define them. |
| `runtime.json` | Procedure runtime configuration, including Simple's resolved web-search, reasoning-effort, output-token, and built-in-tool-call settings, and the bound private API address where present. |
| `run.db` | ADC runtime database when the selected ADC path writes it. |
| `local-run.json` | Service-owned local-agent orchestration record in the launcher run directory. |
| Logical `service-logs/` artifacts | Direct AAR and AARD child streams stored under the service registry, and ADC child streams stored in the outer service run directory; all use stable artifact names. |
| `clerk.json` | Service-owned multi-case record. |

A service must treat an unreadable or missing `run.json` after process exit as a failed or incomplete execution.  AAR and AARD direct and attested consumers require matching case and run identifiers and accept only `ok` with a closed final case or `failed` with a failed final case.  ADC local and direct consumers require a matching `run.json.final_state.case.case_id` and final case status `judgment_entered` or `closed`.  The attested ADC consumer also requires an `adj.case-manifest.v1` ADC manifest with the service case and run identifiers.  The attested readers accept only the exact `aar-run/aar-output`, `aard-run/aard-output`, or `adc-run/adc-output` success path after the service record is complete and attestation verification succeeds.  A partial archive remains available for diagnostic artifact reads and cannot complete an attested case.  A read-only proxy may use a direct terminal record after the private listener closes only when the case id, run id, top-level result, and final procedure state identify the supervised invocation.  Artifact access must confine core paths to the recorded output directory, AAR and AARD direct logs to the approved registry path, ADC service logs to the validated outer run directory, and every name to the service's explicit allowlist.

The service reporter descends past outer `local-run.json`, `service-case.json`, `clerk.json`, and live `events.ndjson` files to the named core child.  When the nested core marker is absent, the reporter retains one outer launcher or service summary and reports its failure or incomplete state.  The named core child is the sole source for a formal core `run.json`, so an earlier flat layout cannot supply a terminal result.

## Interface Tests

Service unit tests use fake core executables to verify argument construction, startup, failure, process cleanup, record reconciliation, proxying, and artifact access.  Paired tests use built ADC, ARB, and AARD executables and a specified `adj` source tree to verify command help, required flags, private API startup, direct cases, and terminal record consumption.  Unified-command tests cover five-way settings and registration, ARB, AARD, and ADC adapter request and result mapping, and simple and quick process arguments, private API startup, participant supervision, cancellation, and record freshness.

The `service/compat/adc`, `service/compat/arb`, and `service/compat/arbd` packages in `adjservices` start the standalone Clerk and MCP executables against selected core binaries.  The ADC package completes a bench case with external parties, an internal judge, digest generation, terminal record reconciliation, and certificate replay.  The ARB and AARD packages cover participant failures, deadlines, complete MCP cases, terminal records, and service reconciliation.  All three packages accept explicit service binary, core binary, and core checkout paths through test flags.  Simple and quick use the unified-command test path described above.

Changes to command names, required flags, private routes, consumed request or response fields, exit behavior, or record names require coordinated edits.  The core and service documentation must describe the same interface.  The paired tests must pass before either repository is prepared for release.
