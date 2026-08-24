# Model Pool

`model-pool/` builds the juror and council model pools that `adc case`, `aar case`, and `aard case` draw from.  The tools evaluate models and pinned OpenRouter provider endpoints against JSON-scored question sets, build endpoint inventories, collect behavior-prompt responses, cluster embeddings, and sample endpoint/persona pools into a `pool.jsonl` of request-spec records.  Generated run files belong under `results/`; versioned inputs belong under `sets/`, `schemas/`, `rubrics/`, `prompts/`, `config/`, `genes.json`, `sampled-genes.json`, and `variants/`.  Persona text comes from `../common/etc/personas/`, the same corpus the runtimes read when a pool record names a persona.

A provider endpoint is the unit of evaluation, because one OpenRouter model ID can route to several endpoints that differ in provider, quantization, context limit, supported parameters, pricing, and behavior.  Behavior evals of the adjudication actors themselves are a separate concern and live under [Evals](../evals/README.md).

Use `model-pool/` as the working directory unless a command says otherwise.  OpenRouter calls require `OPENROUTER_API_KEY` or ignored `secrets/openrouter.api.txt`.  Embedding runs require `OPENAI_API_KEY` or ignored `secrets/openai.api.txt`.

## Documentation

| Document | Use |
| --- | --- |
| [Model Pool Manual](manual.md) | Command reference, terminology, scoring model, endpoint-variant procedures, pool construction, and troubleshooting. |
| [Documentation Index](docs/README.md) | Sampling runbook and model inventory notes. |
| [Core20 Rubric](rubrics/core20.md) | Response schemas, deterministic checks, deliberation score, and operational metrics. |
| [Development Notes](devnotes.md) | Development journal, rationale, and follow-up notes. |

## Quick Checks

Run these commands from `model-pool/` before changing items, prompts, schemas, configs, or sampling inputs.  The validation commands check question records and fixture references.  The audit checks repository consistency for the eval inputs and tool references.

```bash
uv run tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
uv run tools/score_eval.py validate-items --questions sets/deliberation/questions.jsonl
uv run tools/audit_eval.py --json
```

Use the mock runner for a deterministic local test that does not call OpenRouter.  The first command writes a local run under `results/`.  The second command scores that run with the deterministic scorer.

```bash
uv run tools/run_eval.py --prompt prompts/juror-single.md --mock perfect --models mock:perfect --out results/mock-perfect
uv run tools/score_eval.py score --run results/mock-perfect
```

## Run Data

Model responses, score files, provider inventories, sampled pools, and stage summaries belong under `results/`, or under an intentional snapshot directory in `variants/` when the repository needs a checked-in survivor set.  `results/` is ignored except for `results/.gitkeep`, and credentials are ignored under `secrets/`.  README content stays limited to stable workflow, file locations, and entry points; run IDs, endpoint counts, pass rates, accepted endpoint lists, and dated filter details belong in generated results, snapshot summaries, analysis notes, or the manual.

## Layout

| Path | Contents |
| --- | --- |
| `sets/` | Checked-in question sets and fixtures. |
| `schemas/` | JSON schemas for items and responses. |
| `rubrics/` | Deterministic scoring rules and metric definitions. |
| `prompts/` | Prompt text used by eval or pool-construction tools. |
| `config/` | Retained model lists and pool selections from earlier runs, kept as reference sets.  No tool reads them. |
| `genes.json`, `sampled-genes.json` | Behavior-prompt source data and sampled prompt sets. |
| `variants/` | Checked-in provider-endpoint snapshot files. |
| `tools/run_eval.py`, `tools/score_eval.py`, `tools/audit_eval.py` | Question-set execution, validation, scoring, and repository checks. |
| `tools/model_inventory.py`, `tools/run_variant_batch.py`, `tools/run_end_to_end.py` | Provider inventory, endpoint-variant evaluation, and full pool-pipeline execution. |
| `tools/run_first_gene_inference_embeddings.py`, `tools/run_embedding_pca.py`, `tools/run_gene_pca_clustering.py` | Gene-response collection, embedding reduction, and clustering. |
| `tools/aggregate_variant_persona_clusters.py`, `tools/sample-tuple-pool.py` | Cluster aggregation and tuple-uniform pool sampling. |
| `tools/clusters-graph.py` | Renders per-gene cluster rows as a faceted chart. |
| `results/` | Generated eval and pool-construction files. |
