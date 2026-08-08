# Agent Arbitration Degree

Agent Arbitration Degree (AARD) decides one degree question through an adversarial record and a council answer map.  A complaint states the question, plaintiff and defendant lawyers build and argue the record, and each council member submits one integer answer from 0 through 100 with a rationale.  The runtime stores filings, admitted evidence, work notes, council answers, transcripts, event logs, and final state in one case packet.

## Documentation

The manual documents the core commands, case-owned HTTP APIs, outputs, and certificate verification.  The practice guide covers lawyer and council work within a case.  The rules define the procedure implemented by the Go runtime and Lean engine.

| Document | Use |
| --- | --- |
| [Agent Arbitration Degree Manual](manual.md) | Core commands, case-owned APIs, outputs, failure behavior, and certificate verification. |
| [Agent Arbitration Degree Practice Guide](docs/practice.md) | Lawyer and council practice for degree questions. |
| [Agent Rules for Arbitration Degree Procedure](docs/ARAP.md) | Governing AARD procedure. |

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the AARD runtime. |
| Lean `4.32.0` and `lake` | Build the Lean engine and proof tree. |
| Model-provider key | Direct council calls require the environment variable named by the selected pool endpoint. |

## Build

Build from `arbd/` with the targets below.  `make build` writes `.bin/aard` and `.bin/aardengine`.  The test and proof targets check the Go runtime and Lean proof tree.

```bash
make build
make test
make prove
```

## First Run

Start one case process from `arbd/`.  The command writes the private Lawyer and Council API address to stderr and waits for participant clients.  Its output directory contains the durable case record and replay certificate.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex1
```

## Layout

| Path | Purpose |
| --- | --- |
| [Agent Arbitration Degree Manual](manual.md) | Commands, APIs, outputs, and troubleshooting. |
| `docs/` | Rules, practice guide, evidence handling, policy notes, and council references. |
| `engine/` | Lean degree-arbitration engine and proofs. |
| `runtime/` | Go command, case runtime, and case-owned HTTP APIs. |
| `examples/` | Example complaints and case files. |
| `prompts/` | Prompt templates used by the case runtime. |

## Output

Case output contains `run.json`, `state.json`, `certificate.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, `evidence-manifest.json`, and `evidence-store/`.  Council status and request specifications appear in `council.json`.  Keep these files together as the durable record of one case.

## License

The repository-level MIT License appears in [the license](../LICENSE).  Trademark and related terms appear in [the notices](../NOTICES.md).  Both documents apply to this directory.
