# Agent Arbitration Degree

Agent Arbitration Degree (AARD) decides one degree question through an adversarial record and a council answer map.  A complaint states the question, plaintiff and defendant lawyers build and argue the record, and each council member submits one integer answer from 0 through 100 with a rationale.  The runtime stores filings, admitted evidence, work notes, council answers, transcripts, event logs, and final state in one case packet.

## Documentation

The manual documents the core commands, case-owned HTTP APIs, outputs, and certificate verification.  The practice guide covers lawyer and council work within a case.  The rules define the procedure implemented by the Go runtime and Lean engine.

| Document | Use |
| --- | --- |
| [Agent Arbitration Degree Manual](manual.md) | Core commands, case-owned APIs, outputs, failure behavior, and certificate verification. |
| [Agent Arbitration Degree Practice Guide](docs/practice.md) | Lawyer and council practice for degree questions. |
| [Agent Rules for Arbitration Degree Procedure](docs/ARAP.md) | Governing AARD procedure. |
| [AARD Implementation Reference](docs/update.md) | State, authority, invariants, custody, replay, and runtime structure. |
| [Prompt Authoring Guide](../docs/prompt-authoring.md) | Prompt-file resolution, literal replacement, available tokens, and evaluation practice. |

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the AARD runtime. |
| Lean `4.32.0` and `lake` | Build the Lean engine and proof tree. |
| Model-provider key | Direct council calls require the environment variable named by the selected pool endpoint. |

## Build

Build from `arbd/` with the targets below.  `make build` writes `.bin/aard`, `.bin/aard-mcp`, `.bin/aard-run`, and `.bin/aardengine`.  `aard-mcp` exposes the running core's participant APIs to caller-owned lawyers and council members.  `aard-run` starts the core, MCP adapter, council, and an independently selected OpenClaw, Pi, Codex, or Claude runner for each automatic lawyer role.  It issues MCP capabilities for caller-owned lawyer roles.  The [prompt-authoring guide](../docs/prompt-authoring.md#mcp-capabilities) documents MCP key creation, assignment capabilities, and server startup.  `make test` depends on that build and therefore rebuilds `aardengine` before the Go tests.  `make prove` checks the Lean proof tree separately.

```bash
make build
make test
make prove
```

## First Run

Start one case process from `arbd/`.  The command requires an absent or empty output directory and claims it exclusively during initial publication.  It writes the Case API base address to stderr and waits for participant clients.  The listener always includes the Lawyer and Observer APIs, while `--council-backend councilapi` also includes the Council API.  Its output directory contains the durable case record and replay certificate.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --prompt-dir ../prompts/arbd \
  --out-dir out/ex1
```

## Prompt Configuration

AARD owns the complete prompt catalog used by its lawyers, council members, preflight checks, observer, repair turns, and tool descriptions.  A repeatable `--prompt-file ID=PATH` option replaces individual catalog entries, while `--prompt-dir DIR` supplies a complete prompt set.  The [manual](manual.md#prompt-configuration) lists every identifier, filename, and replacement token.

The Lawyer and Council APIs belong to the case process and accept clients implemented in any harness.  An MCP adapter can act as one of those clients without changing the AARD core.  Running AARD requires no `adjservices` package or process.

## Layout

| Path | Purpose |
| --- | --- |
| [Agent Arbitration Degree Manual](manual.md) | Commands, APIs, outputs, and troubleshooting. |
| `docs/` | Rules, practice guide, evidence handling, policy notes, and council references. |
| `engine/` | Lean degree-arbitration engine and proofs. |
| `runtime/` | Go command, case runtime, and case-owned HTTP APIs. |
| `examples/` | Example complaints and case files. |
| `../prompts/arbd/` | Complete editable AARD prompt set. |

## Output

A terminal packet contains `complaint.md`, `case-manifest.json`, `policy.json`, `runtime.json`, `run.json`, `state.json`, `certificate.json`, `council.json`, `transcript.md`, `digest.md`, `events.ndjson`, and `evidence-manifest.json`.  `work-notes.ndjson` appears after the first submitted work note, and `submitted-evidence/` appears after the first accepted lawyer submission.  `evidence-store/` appears when the record contains stored evidence bytes, while each council turn adds records under `council-turns/`.  An earlier process error can leave only the files published before the error.  Keep the complete directory together because `run.json` describes the final result and the sibling files carry the certificate, stored bytes, prompts, events, and human-readable record.

## License

The repository-level MIT License appears in [the license](../LICENSE).  Trademark and related terms appear in [the notices](../NOTICES.md).  Both documents apply to this directory.
