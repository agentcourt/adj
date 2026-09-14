# Agent Arbitration

Agent Arbitration (AAR) decides one proposition through an adversarial record and a council vote.  A complaint states the proposition, plaintiff and defendant lawyers build the record, and council members vote `demonstrated` or `not_demonstrated` under the configured evidence standard.  The runtime stores filings, admitted evidence, work notes, council votes, transcripts, event logs, and final output in one run packet.

## Documentation

The manual documents the core commands, case-owned HTTP APIs, outputs, and certificate verification.  The practice guide covers lawyer and council work within a case.  The rules define the procedure implemented by the Go runtime and Lean engine.

| Document | Use |
| --- | --- |
| [Agent Arbitration Manual](manual.md) | Core commands, case-owned APIs, outputs, failure behavior, and certificate verification. |
| [ARB Documentation Index](docs/README.md) | Rules, evidence handling, council selection, and proof references. |
| [Agent Arbitration Practice Guide](docs/practice.md) | Lawyer and council practice: phase work, evidence search, source preservation, technical reports, work notes, and council deliberation. |
| [Agent Rules for Arbitration Procedure](docs/ARAP.md) | Governing AAR procedure. |
| [Prompt Authoring Guide](../docs/prompt-authoring.md) | Prompt-file resolution, literal replacement, available tokens, and evaluation practice. |

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the AAR runtime. |
| Lean `4.32.0` and `lake` | Builds the Lean engine and proof tree. |
| Model-provider credentials | The selected pool determines credentials.  The [model-endpoint guide](../docs/model-endpoints.md) covers Anthropic, DeepSeek, Google, Hugging Face, OpenAI, OpenRouter, and xAI.  The default pool requires `OPENROUTER_API_KEY`. |
| Rootless Podman and the [Pi image](../containers/pi/README.md) | `aar-run` uses Pi for its council and for lawyers whose profiles select Pi. |

## Build

Build from `arb/`:

```bash
make build
make test
make prove
```

`make build` writes `.bin/aar`, `.bin/aar-mcp`, `.bin/aar-run`, and `.bin/aarengine`.  `aar-mcp` exposes the running core's participant APIs to caller-owned lawyers and council members.  `aar-run` starts the core, MCP adapter, selected local lawyers, and council, or issues MCP capabilities for caller-owned lawyers.  The [prompt-authoring guide](../docs/prompt-authoring.md#mcp-capabilities) documents MCP key creation, assignment capabilities, and server startup.  `make test` rebuilds `.bin/aarengine` and then runs the Go runtime tests against that binary.  `make prove` builds the Lean proof tree.

## First Run

The repository's [complete-case example](../README.md#complete-case) builds the participant image and starts two Pi lawyers and a five-member council through `aar-run`.  The [local-runner guide](../runtime/localrun/arb/README.md) describes participant configuration and output.

## Core API

Start a core process from `arb/` with the command below when supplying external lawyer clients.  It writes the private Case API address to stderr and waits for those clients to act.  Its output directory contains the durable case record and replay certificate.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aar case \
  --complaint ../examples/ex01/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --prompt-dir ../prompts/arb \
  --out-dir out/ex01
```

## Prompt Configuration

AAR owns the complete prompt catalog used by its lawyers, council members, preflight checks, observer, repair turns, and tool descriptions.  A repeatable `--prompt-file ID=PATH` option replaces individual catalog entries, while `--prompt-dir DIR` supplies a complete prompt set.  The [manual](manual.md#prompt-configuration) lists every identifier, filename, and replacement token.

The Lawyer and Council APIs belong to the case process and accept clients implemented in any harness.  An MCP adapter can act as one of those clients without changing the AAR core.  Running AAR requires no `adjservices` package or process.

## Layout

| Path | Purpose |
| --- | --- |
| [Agent Arbitration Manual](manual.md) | Commands, APIs, outputs, and troubleshooting. |
| `docs/` | Rules, practice guide, API/process specs, evidence handling, policy notes, and proof references. |
| `engine/` | Lean arbitration engine and proofs. |
| `runtime/` | Go CLI, case runtime, and case-owned HTTP APIs. |
| `../examples/` | Shared example complaints and case packets. |
| `../prompts/arb/` | Complete editable AAR prompt set. |

## Output

Run output contains `case-manifest.json`, `run.json`, `state.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, `evidence-manifest.json`, `evidence-store/`, and `certificate.json`.  Council turn snapshots live under `council-turns/` as deliberation begins.  Keep these files together as the durable record of one case.

## License

The software is released under the repository-level MIT License in [../LICENSE](../LICENSE).  Trademark and related notice terms are in [../NOTICES.md](../NOTICES.md).
