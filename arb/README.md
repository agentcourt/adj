# Agent Arbitration

Agent Arbitration (AAR) decides one proposition through an adversarial record and a council vote.  A complaint states the proposition, plaintiff and defendant lawyers build the record, and council members vote `demonstrated` or `not_demonstrated` under the configured evidence standard.  The runtime stores filings, admitted evidence, work notes, council votes, transcripts, event logs, and final output in one run packet.

## Documentation

The manual documents the core commands, case-owned HTTP APIs, outputs, and certificate verification.  The practice guide covers lawyer and council work within a case.  The rules define the procedure implemented by the Go runtime and Lean engine.

| Document | Use |
| --- | --- |
| [Agent Arbitration Manual](manual.md) | Core commands, case-owned APIs, outputs, failure behavior, and certificate verification. |
| [Agent Arbitration Practice Guide](docs/practice.md) | Lawyer and council practice: phase work, evidence search, source preservation, technical reports, work notes, and council deliberation. |
| [Agent Rules for Arbitration Procedure](docs/ARAP.md) | Governing AAR procedure. |

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the AAR runtime. |
| Lean `4.32.0` and `lake` | Builds the Lean engine and proof tree. |
| Model-provider key | Direct council calls require the environment variable named by the selected pool endpoints: `OPENAI_API_KEY` or `OPENROUTER_API_KEY`. |

## Build

Build from `arb/`:

```bash
make build
make test
make prove
```

`make build` writes `.bin/aar` and `.bin/aarengine`.  `make test` runs the Go tests for the runtime.  `make prove` builds the Lean proof tree.

## First Run

Start one case process from `arb/`.  The command writes the private Lawyer and Council API address to stderr and waits for participants to act.  Its output directory contains the durable case record and replay certificate.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aar case \
  --complaint examples/ex01/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex01
```

## Layout

| Path | Purpose |
| --- | --- |
| [Agent Arbitration Manual](manual.md) | Commands, APIs, outputs, and troubleshooting. |
| `docs/` | Rules, practice guide, API/process specs, evidence handling, policy notes, and proof references. |
| `engine/` | Lean arbitration engine and proofs. |
| `runtime/` | Go CLI, case runtime, and case-owned HTTP APIs. |
| `examples/` | Example complaints and case packets. |
| `prompts/` | Prompt templates used by the case runtime. |

## Output

Run output contains `run.json`, `state.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, `evidence-manifest.json`, `evidence-store/`, and `certificate.json`.  Council turn snapshots live under `council-turns/` as deliberation begins.  Keep these files together as the durable record of one case.

## License

The software is released under the repository-level MIT License in [../LICENSE](../LICENSE).  Trademark and related notice terms are in [../NOTICES.md](../NOTICES.md).
