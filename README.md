# Adjudication Core

This repository contains the ADC, ARB, and AARD adjudication procedures.  Each procedure has a Lean engine, proofs, a Go one-case runtime, a command-line program, case-owned participant APIs, durable records, and replay verification.  Multi-case services, MCP adapters, local-agent launchers, deployment programs, and web applications live in the `adjservices` repository.

## Procedures

| Procedure | Command | Result |
| --- | --- | --- |
| [Agent District Court](adc/README.md) | `adc` | Civil litigation through pleadings, motions, discovery, trial, verdict, and judgment. |
| [Agent Arbitration](arb/README.md) | `aar` | A binary decision on whether one proposition has been demonstrated. |
| [Agent Arbitration Degree](arbd/README.md) | `aard` | Council answers from 0 through 100 for one degree question. |

The Lean engine controls procedural phases, opportunities, accepted actions, and terminal states.  The Go runtime manages one case, enforces deadlines and attempt limits, controls evidence custody and role visibility, and writes the durable record.  A case process can perform roles through configured model providers or expose selected roles through its private HTTP API.

## Build and Test

Go 1.25 builds the runtimes, and Lean 4.32.0 builds the engines and proof trees.  Each procedure Makefile builds its command and engine into that procedure's `.bin/` directory.  The test and proof targets verify the Go runtime and Lean proof library separately.

```bash
make -C adc build test prove
make -C arb build test prove
make -C arbd build test prove
```

The shared `common/` tree contains the model-request, provider, and persona packages required by the three runtimes.  It also contains the default juror and council request-spec pool, the persona named by that pool, and a [persona corpus](common/etc/personas/README.md) for custom pools.  The Go module remains at the repository root because all three procedure commands use these packages.

## Command-Line Cases

Each command provides `help` for its retained subcommands.  ADC can start from a complaint or a prepared scenario, while ARB and AARD start from complaints.  Model-provider credentials depend on the internal roles and council request specifications selected for a case.  The ADC example signing script requires OpenSSL and creates the two linked signature inputs before complaint drafting.

```bash
cd adc
examples/ex1/sign.sh
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
.bin/adc case --complaint examples/ex1/complaint.md --out-dir out/ex1

cd ../arb
.bin/aar case --complaint examples/ex01/complaint.md --out-dir out/ex01

cd ../arbd
.bin/aard case --complaint examples/ex1/complaint.md --out-dir out/ex1
```

The core commands can expose live participant opportunities through their case-owned HTTP APIs.  Callers select the listen address, external roles, and council backend through the procedure command flags.  The [core process interface](docs/service-interface.md) records the command, private HTTP, and artifact interface used by operational consumers.

## Durable Record

Every procedure writes `case-manifest.json` when a run begins, then writes `run.json`, `state.json`, `certificate.json`, `events.ndjson`, `transcript.md`, `digest.md`, work notes, and evidence records as the run progresses.  Keep the files in one output directory because the verifier and inspection commands treat that directory as one case packet.  Each procedure manual defines its complete file set and terminal behavior.

## Documentation

The [ADC manual](adc/manual.md), [ARB manual](arb/manual.md), and [AARD manual](arbd/manual.md) document their core commands, case APIs, records, failure rules, and certificate verification.  The procedure `docs/` directories contain governing rules, practice guides, engine notes, and proof references.  The [cross-procedure proof status](docs/proof-notes.md) summarizes the maintained formal results.

## License

The software is released under the MIT License in [LICENSE](LICENSE).  Trademark and related notice terms are in [NOTICES.md](NOTICES.md).  Both documents apply to the three retained core procedures.
