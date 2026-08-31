# Adjudication Core

The [`agentcourt/adj`](https://github.com/agentcourt/adj) repository contains five one-case adjudication procedures: ADC, ARB, AARD, simple, and quick.  It owns complete one-case execution, including procedure runtimes, local participant launchers, MCP adapters, prompts, records, and verification.  ADC, ARB, and AARD use Lean engines and replay proofs, while simple and quick provide direct model decisions and one-round adversarial decisions.  The optional [`agentcourt/adjservices`](https://github.com/agentcourt/adjservices) repository provides managed multi-case services, deployment, web applications, and reporting by executing commands installed from this repository.

## Procedures

| Procedure | Command | Result |
| --- | --- | --- |
| [Agent District Court](adc/README.md) | `adc` | Civil litigation through pleadings, motions, discovery, trial, verdict, and judgment. |
| [Agent Arbitration](arb/README.md) | `aar` | A binary decision on whether one proposition has been demonstrated. |
| [Agent Arbitration Degree](arbd/README.md) | `aard` | Council answers from 0 through 100 for one degree question. |
| [Simple Adjudication](simple/README.md) | `simple` | One model returns a binary decision and rationale. |
| [Quick Adjudication](quick/README.md) | `quick` | Two one-shot arguments followed by direct council voting. |

The three formal procedures use Lean to control procedural phases, opportunities, accepted actions, and terminal states.  Their Go runtimes enforce deadlines and attempt limits, control evidence custody and role visibility, and write replayable records.  Quick exposes two lawyer opportunities through a private HTTP API and then calls its council directly, while simple makes one direct provider request without a participant API.

## Build and Test

Go 1.25 builds all five procedures, and Lean 4.32.0 builds the ADC, ARB, and AARD engines and proof trees.  The root Makefile builds the procedure commands, the three formal local-run commands, and `.bin/adjudicate`.  Each procedure Makefile writes its commands beneath that procedure's `.bin/` directory, including `aar-run`, `aard-run`, or `adc-run` for a complete formal case with local or remote lawyers.

```bash
make build
make test

make -C adc build test prove
make -C arb build test prove
make -C arbd build test prove
make -C simple build test
make -C quick build test
```

The shared `common/` tree contains document import and verification, record writing, case manifests, model requests, provider clients, and persona loading used across the procedures.  It also contains the default juror and council request-spec pool, the persona named by that pool, and a [persona corpus](common/etc/personas/README.md) for custom pools.  One root Go module keeps these shared packages and all five commands together.

## Evaluations and Model Pools

The [behavior evals](evals/README.md) place an ADC actor in controlled Lean states, run the production opportunity executor, and score the resulting legal action against committed fixtures.  ADC provides ten judge suites through `adc eval`, with fixtures, candidate prompts, plans, and analyses under `evals/adc/judge/`.  Generated behavior-eval records belong under the ignored `evals/out/` directory.

The [model-pool tools](model-pool/README.md) inventory provider endpoints, evaluate and score models, collect behavior responses, calculate embeddings, cluster the results, and sample request-specification pools.  Quick, ARB, AARD, and ADC load installed JSONL pool records through their council or juror pool options, resolving each relative persona path beside the pool or through the shared common-tree layout.  Generated pool runs belong under the ignored `model-pool/results/` directory, while the installed runtime default remains `common/data/personas/pool.jsonl`.

## Command-Line Cases

Each command provides `help` for its subcommands.  ADC can start from a complaint, proposition, or prepared scenario.  ARB and AARD start from complaints, while simple and quick start from propositions.  Model-provider credentials depend on the roles, direct model, and council request specifications selected for a case.  The ADC example signing script requires OpenSSL and creates the two linked signature inputs before complaint drafting.

```bash
cd adc
examples/ex1/sign.sh
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
.bin/adc case --complaint examples/ex1/complaint.md --out-dir out/ex1

cd ../arb
.bin/aar case --complaint ../examples/ex01/complaint.md --out-dir out/ex01

cd ../arbd
.bin/aard case --complaint examples/ex1/complaint.md --out-dir out/ex1

cd ../simple
.bin/simple case \
  --proposition "The sky is blue" \
  --evidence-standard preponderance_of_the_evidence \
  --out-dir out/example \
  --model openai://MODEL \
  --allow-api-key \
  --max-documents 100 \
  --max-document-bytes 1048576 \
  --max-documents-bytes 8388608

cd ..
quick/.bin/quick case \
  --proposition "The sky is blue" \
  --out-dir quick/out/example \
  --council-size 3 \
  --required-votes 2 \
  --evidence-standard preponderance_of_the_evidence \
  --lawyerapi-bearer-token-file ./private/quick-caseapi.token \
  --max-document-files 100 \
  --max-document-file-bytes 1048576 \
  --max-documents-total-bytes 8388608 \
  --allow-api-key
```

This command waits for external plaintiff and defendant clients to connect through `quick-mcp`.  The [Quick guide](quick/README.md) describes the lawyer API and client setup.

The unified `.bin/adjudicate` command selects any of the five procedures from one settings file.  It starts the selected core, MCP adapter, automatic participants, and council or jury processes required for one case.  A procedure can instead set `auto_lawyers` to `plaintiff`, `defendant`, or `none` and supply the remaining lawyer through the generated MCP capability.  The [unified command reference](adjudication-cli.md) defines its request, settings, participant profiles, and result.

Quick uses `./pool.jsonl` when present, then the shared `common/data/personas/pool.jsonl`.  `--council-pool` and `--common-root` override those paths.  The resolved pool path appears in `input.json`.

ADC, ARB, AARD, and quick can expose live participant opportunities through case-owned HTTP APIs.  Callers select the listen address, external roles, or council backend through procedure flags where the procedure supports those choices.  The [core process interface](docs/service-interface.md) records the command, private HTTP, and artifact interface used by operational consumers.

## Durable Record

Every procedure writes `case-manifest.json`, `run.json`, and an event record in its output directory.  ADC, ARB, and AARD add engine state, replay certificates, evidence custody records, transcripts, digests, and procedure-specific files.  Simple records its imported documents, model request and response, parsed decision, state, transcript, and digest, while quick records its imported documents, private lawyer notes, two arguments, selected council, votes, and provider accounting.  Keep each procedure's files together because inspection and verification use the output directory as one case record.

`adjudicate case-record --dir RUN_DIR` reads any of these records and writes a chronological JSON docket with a physical artifact catalog.  Optional flags add work notes or retained participant sessions and process logs.  `--publish-dir TARGET` copies the selected artifacts and a portable index into a new directory.  The [unified command reference](adjudication-cli.md#case-record-index) defines the command, publication layout, and JSON fields.

## Documentation

The [unified command reference](adjudication-cli.md) documents one-case execution across the five procedures, and the [prompt-authoring guide](docs/prompt-authoring.md) covers core, MCP, and launcher prompt catalogs.  The [ADC manual](adc/manual.md), [ARB manual](arb/manual.md), and [AARD manual](arbd/manual.md) document procedure commands, case APIs, records, failure rules, and certificate verification.  The [simple manual](simple/manual.md) defines its direct-model request and record, while the [quick guide](quick/README.md) defines its lawyer API, council execution, and record.  The formal-procedure `docs/` directories contain governing rules, practice guides, engine notes, and proof references, and the [cross-procedure proof status](docs/proof-notes.md) summarizes the maintained Lean results.

## License

The software is released under the MIT License in [LICENSE](LICENSE).  Trademark and related notice terms are in [NOTICES.md](NOTICES.md).  Both documents apply to the repository's five procedures and shared packages.
