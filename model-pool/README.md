# Model Pool

`model-pool/` constructs juror and council model-pool records for `adc case`, `aar case`, and `aard case`.  The tools inventory pinned OpenRouter provider endpoints, screen each configuration through direct and Pi/MCP tool calls, evaluate accepted configurations against JSON-scored question sets, collect behavior-prompt responses, cluster embeddings, and sample endpoint/persona records into `pool.jsonl`.  Generated run files belong under `results/`.  Versioned procedure inputs belong under `sets/`, `schemas/`, `rubrics/`, `prompts/`, `genes.json`, `sampled-genes.json`, and `variants/`.  `config/` contains reference-only model and pool lists, and persona text comes from `../common/etc/personas/`.

A provider endpoint is the unit of evaluation, because one OpenRouter model ID can route to several endpoints that differ in provider, quantization, context limit, supported parameters, pricing, and behavior.  Pool records retain endpoint identity so runtime requests can preserve the selected route constraints.  Behavior evals of the adjudication actors live under [Evals](../evals/README.md).

Use `model-pool/` as the working directory unless a command says otherwise.  OpenRouter calls require `OPENROUTER_API_KEY` in the environment or an ignored `secrets/openrouter.api.txt` file containing `OPENROUTER_API_KEY=<key>` or `export OPENROUTER_API_KEY=<key>`.  Embedding runs require `OPENAI_API_KEY` in the environment or an ignored `secrets/openai.api.txt` file using the corresponding `OPENAI_API_KEY` assignment.  A bare token in either file is rejected.

Run `make pool RUN_ID=pool-YYYYMMDDTHHMMSSZ` from `model-pool/` to generate the configured 100-entry pool.  The Makefile inventories the full OpenRouter catalog, uses eight screening processes and eight evaluation processes, runs three evaluation trials, samples fourteen genes with five responses per endpoint and gene, requests eight PCA dimensions, searches cluster counts from `2` through `20`, and selects at most one record per model without replacement.  Make variables in `model-pool/Makefile` override those settings, and the completed pool appears at `results/<run-id>/pool/pool.jsonl`.

## Documentation

| Document | Use |
| --- | --- |
| [Model Pool Manual](manual.md) | Command reference, terminology, scoring model, endpoint-variant procedures, pool construction, and troubleshooting. |
| [Documentation Index](docs/README.md) | Sampling runbook and model inventory reference. |
| [Core20 Rubric](rubrics/core20.md) | Response schemas, deterministic checks, deliberation score, and operational metrics. |
| [Development Notes](devnotes.md) | Development journal, design rationale, and verification record. |

## Quick Checks

Run these commands from `model-pool/` before changing items, prompts, schemas, or sampling inputs.  The validation commands check question records and fixture references.  The audit checks question modes, schema assumptions, prompt contracts, exact-route defaults, and endpoint filename uniqueness.

```bash
uv run --no-cache tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
uv run --no-cache tools/score_eval.py validate-items --questions sets/deliberation/questions.jsonl
uv run --no-cache tools/audit_eval.py --json
```

Use the mock runner for a deterministic local test that does not call OpenRouter.  The first command writes a local run under `results/`.  The second command scores that run with the deterministic scorer.

```bash
uv run --no-cache tools/run_eval.py --prompt prompts/juror-single.md --mock perfect --models mock:perfect --out results/mock-perfect
uv run --no-cache tools/score_eval.py score --run results/mock-perfect
```

## Run Data

Generated model responses, score files, provider inventories, sampled pools, and stage summaries belong under `results/`.  Git ignores that directory except for `results/.gitkeep`, and it ignores credentials under `secrets/`.  The checked-in provider-endpoint snapshot lives under `variants/filtered-20260529/`.

Gene inference reads persona text from `--persona`.  `--persona-record-path` selects the path written into generated records.  `make pool` and the end-to-end command default that stored path to `personas/generic.md`, which resolves when the pool is installed at `common/data/personas/pool.jsonl`.  A pool used from another directory needs a persona path resolvable there.  The [manual](manual.md#full-selection-procedure) defines path resolution and standalone gene-command defaults.

## Layout

| Path | Contents |
| --- | --- |
| `sets/` | Checked-in question sets and fixtures. |
| `schemas/` | JSON schemas for items and responses. |
| `rubrics/` | Deterministic scoring rules and metric definitions. |
| `prompts/` | Prompt text used by eval or pool-construction tools. |
| `config/` | Reference-only model lists and pool selections. |
| `genes.json`, `sampled-genes.json` | Behavior-prompt source data and sampled prompt sets. |
| `variants/` | Checked-in provider-endpoint snapshot files. |
| `tools/run_eval.py`, `tools/score_eval.py`, `tools/audit_eval.py` | Question-set execution, validation, scoring, and repository checks. |
| `tools/model_inventory.py`, `tools/run_model_screen.py`, `tools/run_variant_batch.py`, `tools/run_end_to_end.py` | Provider inventory, runtime tool-use screening, endpoint evaluation, and full pool construction. |
| `tools/run_first_gene_inference_embeddings.py`, `tools/run_embedding_pca.py`, `tools/run_gene_pca_clustering.py` | Gene-response collection, embedding reduction, and clustering. |
| `tools/aggregate_variant_persona_clusters.py`, `tools/sample-tuple-pool.py` | Cluster aggregation and tuple-uniform pool sampling. |
| `tools/clusters-graph.py` | Renders per-gene cluster rows as a faceted chart. |
| `results/` | Generated eval and pool-construction files. |
