# Model Pool Manual

`model-pool/` selects OpenRouter provider endpoints for juror and council model pools.  It evaluates endpoints with JSON-scored question sets, filters completed endpoint summaries by provider-error count and deliberation score, applies behavior prompts to accepted endpoints, clusters response embeddings, and samples endpoint/persona records.  Scripts write generated files under `results/`, while checked-in procedure inputs live in `sets/`, `schemas/`, `rubrics/`, `prompts/`, `genes.json`, `sampled-genes.json`, and `variants/`.  `config/` contains reference-only model and pool lists, and persona text comes from `../common/etc/personas/`.

Use `model-pool/` as the working directory unless a command says otherwise.  OpenRouter calls require `OPENROUTER_API_KEY` in the environment or an ignored `secrets/openrouter.api.txt` file containing `OPENROUTER_API_KEY=<key>` or `export OPENROUTER_API_KEY=<key>`.  Gene-response embedding calls also require `OPENAI_API_KEY` in the environment or an ignored `secrets/openai.api.txt` file using the corresponding `OPENAI_API_KEY` assignment.  A bare token in either file is rejected.

The runner requests strict JSON responses and records raw outputs, parsed responses, tool traces, provider metadata, timing data, and cost data.  Generated results stay under `results/`, while the checked-in accepted endpoint set lives under `variants/filtered-20260529/`.  Claims about current provider behavior require a refreshed inventory and fresh evals.

## Purpose

The eval tools serve two functions for adjudication model pools.  They check whether a model follows the adjudication response format, uses bounded evidence, cites records, and answers the knowledge, quantitative, reasoning, and juror-deliberation items.  They also build endpoint-variant and persona-cluster records for juror and council selection in `adc/`, `arb/`, and `arbd/`.

The eval sets are sized for manual review.  A high score shows that a model behaved acceptably on these items under the recorded provider route, request parameters, and prompt wrapper.  A different route, endpoint, model release, or prompt wrapper requires another evaluation.

## Task Guide

| Task | Use |
| --- | --- |
| Check one model or pinned provider endpoint against a question set. | [Question-Set Evaluation](#question-set-evaluation) |
| Build a pool from OpenRouter model IDs. | [Full Selection Procedure](#full-selection-procedure) |
| Refresh or inspect the accepted provider endpoint set. | [Provider Endpoint Selection](#provider-endpoint-selection) |
| Understand behavior prompts, embeddings, PCA, clusters, and pool sampling. | [Behavior Clustering and Pool Sampling](#behavior-clustering-and-pool-sampling) |
| Render a chart of the per-gene clusters. | [Behavior Clustering and Pool Sampling](#behavior-clustering-and-pool-sampling) |
| Interpret score fields. | [Score Model](#score-model) |
| Diagnose failed validation, endpoint, batch, or clustering runs. | [Troubleshooting](#troubleshooting) |
| Find the meaning and purpose of a repository term. | [Glossary](#glossary) |
| Find the file that stores a specific record. | [File Reference](#file-reference) |

## Operating Rules

Validate checked-in inputs before OpenRouter calls.  `tools/score_eval.py validate-items` checks item files, and `tools/audit_eval.py --json` checks repository consistency.  If either command fails, fix the item, schema, fixture, or repository reference before running network calls.

Preserve each stage directory as a unit.  Raw result rows show model behavior, score files show deterministic evaluation, logs show provider or runner failures, and batch spec files record exact endpoint-routing requests.  Moving one file without its related results makes later comparison harder and can hide provider-route differences.

Refresh endpoint inventories when a claim depends on current provider behavior.  OpenRouter providers can change routing, availability, pricing, metadata, and serving behavior after the checked-in snapshot.  Rebuild the inventory, tool-use screen, batch eval, filtered endpoint set, gene responses, PCA records, clusters, aggregate records, and sampled pool when producing a new pool for current runs.

## Full Selection Procedure

The procedure accepts OpenRouter model IDs and produces `pool.jsonl`, a JSONL file of endpoint/persona records.  A provider endpoint is evaluated as its own unit because one OpenRouter model ID can route to multiple provider endpoints with different provider tags, quantization, context limits, supported parameters, pricing, and behavior.  Each output row supplies a runtime model request and persona reference.

The gene stage stores the persona path supplied through `--persona`, and aggregation and sampling preserve that value.  With the default option, a generated row contains `../common/etc/personas/generic.md`, a path relative to `model-pool/`.  A runtime resolves a relative persona path beside the selected pool file and then under `<pool-dir>/../../etc/`, so it cannot resolve that default value from the nested result directories used by the documented sampler and end-to-end commands.  Pool construction preserves that path unchanged.  The installed default at `common/data/personas/pool.jsonl` instead contains `personas/generic.md`, which resolves through the shared-tree path.

| Step | Input | Script | Output |
| --- | --- | --- | --- |
| Select root models | Explicit OpenRouter model IDs, or `--root-count` plus `--root-seed` | `tools/run_end_to_end.py` | Selected OpenRouter model IDs |
| Inventory provider endpoints | Selected OpenRouter model IDs | `tools/model_inventory.py` | Provider endpoint rows and raw OpenRouter catalog files |
| Screen runtime tool use | Inventory endpoint rows | `tools/run_model_screen.py` and `.bin/model-config-screen` | Accepted rows, rejected rows, direct-call results, Pi transcripts, MCP calls, usage, and costs |
| Evaluate provider endpoints | Screened endpoint rows and a question file | `tools/run_variant_batch.py`, which calls `tools/run_eval.py` and `tools/score_eval.py` | Response files, score files, exact request specs, and per-endpoint summary rows |
| Filter endpoints | Provider endpoint rows and evaluation summaries | `tools/run_end_to_end.py` filter stage | Accepted endpoint rows, rejected endpoint records, and filter summary |
| Collect behavior responses | Accepted endpoint rows, sampled genes, persona file, sample count | `tools/run_first_gene_inference_embeddings.py` | Gene completions, OpenRouter metadata, embeddings, and per-gene summary |
| Filter gene errors | Every selected gene's records and accepted endpoint rows | `tools/run_end_to_end.py` | One shared eligible endpoint set, excluded endpoint records, and eligible records for each gene |
| Reduce embeddings | Eligible gene completion records | `tools/run_embedding_pca.py` | PCA coordinates and PCA summary |
| Cluster responses | Per-gene PCA records | `tools/run_gene_pca_clustering.py` | Cluster assignments and clustering summary |
| Aggregate cluster labels | Cluster assignments, cluster fit, and accepted endpoint rows | `tools/aggregate_variant_persona_clusters.py` | Endpoint/persona cluster records |
| Sample pool | Endpoint/persona cluster records | `tools/sample-tuple-pool.py` | `pool.jsonl`, sampling diagnostics, and equivalence records |

`tools/run_end_to_end.py` executes those stages in one command.  It prints structured stage and command events while it runs.  Stage directories and `summary.json` contain the results.

Run `make pool RUN_ID=pool-YYYYMMDDTHHMMSSZ` from `model-pool/` to use the full-catalog Makefile configuration.  That configuration uses eight screening processes and eight evaluation processes, runs three evaluation trials, samples fourteen genes with five responses per endpoint and gene, requests eight PCA dimensions, searches cluster counts from `2` through `20`, and selects 100 records without replacement with at most one record per model.  The completed pool appears at `results/<run-id>/pool/pool.jsonl`, and Make variables override each setting.

```bash
uv run --no-cache --script tools/run_end_to_end.py \
  --run-id e2e-YYYYMMDDTHHMMSSZ \
  --root-count 5 \
  --root-seed 0 \
  --screen-processes 8 \
  --prompt prompts/juror-single.md \
  --eval-trials 1 \
  --gene-count 2 \
  --samples-per-gene 1 \
  --pca-dimensions 3 \
  --min-k 2 \
  --max-k 4 \
  --pool-size 5
```

This example screens the endpoint inventory in eight separate processes, evaluates the accepted configurations with one trial per question, samples two genes, collects one response per accepted endpoint/gene pair, requests three PCA dimensions, searches K-means values from `2` through `4`, and writes five pool entries.  Each screening process handles its assigned configurations sequentially.  Each gene process writes every configured sample, including request failures.  After all genes finish, the runner removes a configuration from every gene when any selected gene has a completion error, embedding error, missing sample, or missing embedding for that configuration.  PCA and later stages use that shared eligible set.  The runner caps the PCA dimensions at the eligible embedding-row count unless `--strict-pca-dimensions` makes the mismatch an error.  For a specified pool, set the root models or root sampling parameters, question file, trial count, filter criteria, gene selection, sample count, PCA dimensions, clustering range, pool size, and random seeds explicitly.  `--gene-processes` starts that many separate gene commands at a time, and `--stop-after` stops after a named stage.

Completed screen and evaluation files can begin a new run at filtering:

```bash
uv run --no-cache --script tools/run_end_to_end.py \
  --run-id pool-from-eval \
  --start-at filter \
  --existing-screen-variants results/source/endpoint_variants.jsonl \
  --existing-eval-summary results/source/variant_summary.csv \
  --filter-provider-error-count 0 \
  --filter-deliberation-score-gt 0.70 \
  --genes genes.json \
  --gene-count 14 \
  --gene-processes 2 \
  --samples-per-gene 5 \
  --pca-dimensions 8 \
  --strict-pca-dimensions \
  --min-k 2 \
  --max-k 20 \
  --pool-size 100 \
  --without-replacement \
  --one-per-model
```

The command requires both source files and joins them by `combined_index`.  `--filter-deliberation-score-gt` uses strict greater-than comparison.  The new run directory contains endpoint filtering, gene inference, gene-error filtering, PCA, clustering, aggregation, and sampling output.

## Glossary

| Term | Meaning | Purpose | Main files |
| --- | --- | --- | --- |
| Question | One ordinary question or record-based adjudication task. | Tests answer quality, JSON compliance, or record use. | `sets/*/questions.jsonl` |
| Evidence record | Local evidence for a record-based adjudication question. | Limits what the model may cite when answering a record-based question. | `sets/core20/fixtures/*` |
| Response | Strict JSON returned by a model for one question. | Gives the scorer fixed fields for answer, confidence, rationale, and evidence citations. | `schemas/response.schema.json` |
| Evaluation output | Raw and parsed responses for a model or provider endpoint over questions and trials. | Preserves model output, tool trace, provider metadata, timing data, and errors before scoring. | `results/*/raw_results.jsonl` |
| Score | Deterministic checks and aggregate metrics for an evaluation output. | Separates answer quality from formatting failures, provider errors, tool failures, latency, and cost. | `results/*/scores.json` |
| Provider endpoint | One OpenRouter provider endpoint for one OpenRouter model ID. | Keeps provider routing, quantization, context limits, pricing, and supported parameters separate during evaluation. | `endpoint_variants.jsonl` |
| Accepted endpoint | A provider endpoint that passed the recorded operational and deliberation filters. | Supplies eligible endpoints for behavior sampling and pool construction. | `variants/filtered-20260529/*` |
| Gene | A behavior-eliciting prompt applied after endpoint filtering. | Produces response variation used to compare accepted endpoints beyond question-set scores. | `genes.json`, `sampled-genes.json` |
| Persona | Role text used while sampling gene responses. | Holds the role constant while comparing endpoint behavior on genes and supplies persona text when a sampled pool entry is selected. | `../common/etc/personas/` |
| Cluster assignment | One sampled completion assigned to a per-gene PCA cluster. | Records the behavior group for one endpoint response to one gene. | `clusters.jsonl` |
| Cluster record | One endpoint/persona row with one cluster label per sampled gene. | Summarizes endpoint/persona behavior for pool sampling. | `variant-persona-clusters.jsonl` |
| Pool entry | One selected endpoint/persona row for a model pool. | Provides a model request specification and persona reference to pool code. | `pool.jsonl` |

## Question-Set Evaluation

Question-set evaluation tests whether a model or pinned provider endpoint answers the question set correctly and returns the required JSON.  It records formatting failures, request errors, tool failures, latency, and cost as separate fields.  Endpoint filtering uses each variant's run exit code, combined provider-error count, and deliberation score.  Completion count, latency, timeout count, schema violations, tool failures, invalid votes, malformed JSON, context-limit errors, and cost remain in the score output but do not independently affect that filter.

| Input | Meaning |
| --- | --- |
| Question file | `sets/core20/questions.jsonl` or `sets/deliberation/questions.jsonl` |
| Prompt file | `--prompt`, default `prompts/juror-single.md`.  Relative paths resolve from `model-pool/`. |
| Target endpoint | `--models openrouter://...`, `--model-spec ...`, `--model-spec-jsonl ...`, or a mock model |
| Trial count | `--trials`, default `3` |
| Evidence records | `sets/core20/fixtures/*` for record-based adjudication questions |
| Output directory | `--out results/<name>` |

| Output | Contents |
| --- | --- |
| `raw_results.jsonl` | One response row per target, trial, and question |
| `scores.json` | Per-response scores and aggregate summaries by model or provider endpoint |

Ordinary questions must return `answer`, `confidence`, `rationale`, and `evidence_ids`, and the scorer requires an empty evidence list for every ordinary item.  Record-based adjudication questions must return `vote`, `confidence`, `rationale`, and `evidence_ids`.  Their cited evidence IDs must come from the evidence record.

```json
{"answer":"A","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":[]}
```

```json
{"vote":"demonstrated","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":["E1"]}
```

`sets/core20/questions.jsonl` contains 20 questions: four human-knowledge questions, four science or quantitative questions, four reasoning questions, four instruction-following questions, and four record-based adjudication questions.  `sets/deliberation/questions.jsonl` contains the first twelve knowledge, science, and reasoning questions from `core20` plus eight juror-deliberation questions.  The juror-deliberation questions test burden of proof, evidentiary sufficiency, source reliability, conflicting records, temporal precision, alternative explanations, confidence calibration, and scope control.

### Local Verification Without API Keys

```bash
uv run --no-cache tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
uv run --no-cache tools/score_eval.py validate-items --questions sets/deliberation/questions.jsonl
uv run --no-cache tools/audit_eval.py --json
uv run --no-cache tools/run_eval.py --prompt prompts/juror-single.md --mock perfect --models mock:perfect --out results/mock-perfect
uv run --no-cache tools/score_eval.py score --run results/mock-perfect
```

### OpenRouter Check

```bash
uv run --no-cache tools/run_eval.py \
  --prompt prompts/juror-single.md \
  --models openrouter://openai/gpt-4.1-mini \
  --limit 2 \
  --tool-mode function \
  --out results/openrouter-test

uv run --no-cache tools/score_eval.py score --run results/openrouter-test
```

An exact provider-endpoint evaluation starts from a JSON spec.  For an inventory-derived row, `tools/run_eval.py` adds the OpenRouter metadata header, pins the provider endpoint, disables fallbacks, and records returned generation metadata when OpenRouter provides it.  Batch eval directories retain the exact spec used for each endpoint request.

```bash
uv run --no-cache tools/run_eval.py \
  --prompt prompts/juror-single.md \
  --model-spec results/<batch-run>/specs/<variant>.json \
  --limit 2 \
  --tool-mode function \
  --out results/openrouter-provider-endpoint-test

uv run --no-cache tools/score_eval.py score --run results/openrouter-provider-endpoint-test
```

### Endpoint JSONL

```bash
uv run --no-cache tools/run_eval.py \
  --prompt prompts/juror-single.md \
  --model-spec-jsonl variants/filtered-20260529/endpoint_variants.jsonl \
  --limit 2 \
  --tool-mode function \
  --out results/openrouter-variant-jsonl-test

uv run --no-cache tools/score_eval.py score --run results/openrouter-variant-jsonl-test
```

### Function-Tool Check

```bash
uv run --no-cache tools/run_eval.py \
  --prompt prompts/juror-single.md \
  --questions sets/core20/questions.jsonl \
  --item-id core20.tool.001 \
  --tool-mode function \
  --models openrouter://openai/gpt-4.1-mini \
  --out results/tool-function-openrouter-test

uv run --no-cache tools/score_eval.py score \
  --questions sets/core20/questions.jsonl \
  --run results/tool-function-openrouter-test
```

### Deliberation Evaluation

```bash
uv run --no-cache tools/run_eval.py \
  --prompt prompts/juror-single.md \
  --questions sets/deliberation/questions.jsonl \
  --models openrouter://openai/gpt-4.1-mini \
  --out results/deliberation-openrouter-test

uv run --no-cache tools/score_eval.py score \
  --questions sets/deliberation/questions.jsonl \
  --run results/deliberation-openrouter-test
```

Use `--trials 1` for a single-pass test run.  The default of three trials supports stability fields in the scorer.  Score comparisons require the same trial count.

`deliberation_score` is the mean, across trials that contain a completed deliberation row, of the fraction of completed deliberation rows answered correctly on the substantive issue.  Rows with metadata errors, including timeouts and provider errors, do not enter its denominator, and the scorer omits a trial with no completed deliberation rows.  Operational metrics report latency, request errors, malformed JSON, schema violations, invalid votes, tool-call failures, context-limit errors, and cost.  `provider_error_count` combines provider, rate-limit, credential, and runner errors.  The checked-in accepted endpoint set uses `provider_error_count == 0` and `deliberation_score >= 0.90`.

## Provider Endpoint Selection

Provider endpoint selection records the provider endpoints available for selected OpenRouter model IDs, evaluates each endpoint separately, and applies explicit filter criteria.  `tools/model_inventory.py` fetches the OpenRouter model catalog and endpoint metadata, then writes one normalized row per provider endpoint.  It saves the raw responses separately and records their paths with the provider name, endpoint tag, quantization, limits, supported parameters, pricing, and status fields.

```bash
uv run --no-cache tools/model_inventory.py \
  --run-id model-roots-10-YYYYMMDDTHHMMSSZ \
  --model-id deepseek/deepseek-v4-flash
```

### Inventory Outputs

| File | Contents |
| --- | --- |
| `endpoint_variants.jsonl` | One provider endpoint row per OpenRouter endpoint |
| `endpoint_variants.csv` | Inspection table for endpoint rows |
| `summary.json` | Counts, selected model IDs, endpoint fetches, and provider, quantization, and status counts |
| `raw/models.json` | Raw OpenRouter model catalog response |
| `raw/endpoints/*.json` | Raw OpenRouter endpoint metadata responses |

### Runtime Tool-Use Screen

The screen runs two checks for every endpoint configuration.  The direct check sends the Quick council preflight prompt through the Responses API and requires one valid `submit_council_vote` call.  It uses Quick's 20-second preflight limit and three provider attempts.  The Pi check starts the Pi container and MCP proxy extension used by ARB, ARBD, and ADC, then requires `wait_for_opportunity` and `submit_council_vote` through MCP.

Both checks use the inventory row's model, exact provider route, request parameters, and disabled-fallback policy.  Endpoint capability metadata controls the Chat Completions parameter names used by Pi, while endpoint prices populate Pi's token accounting.  A model passes only when both checks submit a schema-valid vote.  After MCP accepts the Pi vote, the screen allows five seconds for Pi to exit and then stops the test container because ARB has completed the council opportunity at that point.

```bash
make screen-command
uv run --no-cache --script tools/run_model_screen.py \
  --variants results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-screen-YYYYMMDDTHHMMSSZ \
  --screen-command ../.bin/model-config-screen
```

The coordinator rejects rows whose metadata lacks a model, provider route, text input, text output, or tool support.  A model or provider refusal rejects that configuration and allows the next configuration to run.  Missing credentials, container startup failures, and malformed Pi transcripts stop the partition as infrastructure errors.  Each rejection record identifies the failing check and its returned error.

| File | Contents |
| --- | --- |
| `endpoint_variants.jsonl` | Configurations that passed both checks. |
| `rejected_variants.jsonl` | Inventory rows with their rejection records. |
| `results.jsonl` | One summary row per configuration. |
| `configurations/*/result.json` | Direct and Pi status, tool calls, token usage, and cost. |
| `configurations/*/pi.stdout.jsonl` | Pi event transcript. |
| `configurations/*/pi.stderr.log`, `configurations/*/mcp.log` | Pi and MCP diagnostics. |
| `summary.json` | Counts, cumulative cost, and output paths. |

### Request Fields

| Field | Role |
| --- | --- |
| `openrouter_model_id` | `model` |
| `endpoint_tag` | `provider.only[0]` when present |
| `provider_name` | Fallback `provider.only` value when `endpoint_tag` is absent |
| `quantization` | `provider.quantizations` when the value is known |
| `supported_parameters` | Endpoint capability metadata retained with the result |

Provider-endpoint evaluations use exact routing constraints.  For known quantization, the request includes `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, and `provider.quantizations`.  For `quantization: "unknown"`, the request still pins the provider endpoint and omits the quantization list.

```json
{
  "model": "<openrouter_model_id>",
  "provider": {
    "only": ["<endpoint_tag>"],
    "allow_fallbacks": false,
    "require_parameters": true,
    "quantizations": ["<quantization>"]
  }
}
```

### Unknown Quantization

```json
{
  "model": "<openrouter_model_id>",
  "provider": {
    "only": ["<endpoint_tag-or-provider>"],
    "allow_fallbacks": false,
    "require_parameters": true
  }
}
```

### Route Metadata Header

```text
X-OpenRouter-Experimental-Metadata: enabled
```

Eval result rows record the routed endpoint from response metadata and `/api/v1/generation?id=<generation_id>` when OpenRouter returns it.  The metadata can include provider, endpoint, usage, cost, latency, native token counts, and upstream IDs.  The request measures the routed OpenRouter endpoint product, while exact weights, serving engine, GPU type, KV-cache precision, and provider prompts require provider attestations or controlled deployments.

Use `tools/run_variant_batch.py` to evaluate an endpoint inventory one variant at a time.  The batch runner writes one exact spec file per variant, calls `tools/run_eval.py`, scores each completed run with `tools/score_eval.py`, and writes per-variant summary rows.  The output directory must be absent or empty.

```bash
uv run --no-cache --script tools/run_variant_batch.py \
  --variants results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-eval-YYYYMMDDTHHMMSSZ \
  --questions sets/core20/questions.jsonl \
  --prompt prompts/juror-single.md \
  --trials 3 \
  --timeout 90
```

### Batch Outputs

| File | Contents |
| --- | --- |
| `specs/*.json` | Exact OpenRouter variant specs used for requests. |
| `variant-runs/*/run_eval.log` | Child-process output for one variant. |
| `variant-runs/*/raw_results.jsonl` | Raw eval rows for one variant. |
| `variant-runs/*/scores.json` | Scored summary for a variant whose scoring command succeeded. |
| `variant_summary.csv` | Tabular per-variant summary. |
| `summary.json` | Batch status and aggregate counts. |

The batch runner accepts `--no-progress-timeout` and `--variant-timeout`.  `--timeout` sets the timeout for each completion attempt.  Completion requests use the council client's four-attempt policy for HTTP 408, 409, 429, 5xx, timeout, and network failures, with delays of zero, five, and thirty seconds.  `--no-progress-timeout` terminates a child process that stops writing output or result rows.  A timed-out variant remains in `variant_summary.csv`, and downstream filtering excludes it before gene inference.

The checked-in accepted endpoint set is `variants/filtered-20260529/`.  It contains 32 accepted provider endpoints from a 72-endpoint source set.  Its filter criteria are recorded in `summary.json`: `provider_error_count == 0` and `deliberation_score >= 0.90`.

### Filter Outputs

| File | Contents |
| --- | --- |
| `endpoint_variants.jsonl` | Full provider endpoint rows for accepted endpoints |
| `endpoint_variants.csv` | Inspection table for accepted endpoints |
| `removed_variants.jsonl` | Rejected endpoint identities, filter reasons, and reason-specific exit or score fields |
| `summary.json` | Filter criteria, source paths, accepted endpoint count, and selected source indexes |

The checked-in snapshot contains `endpoint_variants.jsonl`, `endpoint_variants.csv`, and `summary.json`.  Its summary records the filter criteria, total endpoint count, accepted endpoint count, and accepted source indexes.  Generated filter directories also contain rejected rows in `removed_variants.jsonl`.

## Behavior Clustering and Pool Sampling

Behavior clustering compares accepted endpoints on behavior-eliciting prompts after question-set filtering.  A gene is one behavior prompt, and `../common/etc/personas/generic.md` is the default persona.  For each `gene + provider endpoint + persona`, `tools/run_first_gene_inference_embeddings.py` collects one or more completions and embeds the response text.

| Input | Meaning |
| --- | --- |
| Accepted endpoints | `variants/filtered-20260529/endpoint_variants.jsonl` or another filtered endpoint file whose rows contain `endpoint_tag` |
| Genes | `sampled-genes.json` or another sampled gene file |
| Persona | `../common/etc/personas/generic.md` unless `--persona` names another file |
| Samples per gene | `--samples` for gene inference and `--expected-samples-per-gene` for aggregation |

| File | Contents |
| --- | --- |
| `records.jsonl` | Gene completions, request and route metadata, errors, and embedding vectors |
| `summary.json` | Expected and written record counts, status counts, and completion and embedding error counts |
| `pca-records.jsonl` | PCA coordinates for one gene's embedded responses |
| `clusters.jsonl` | One cluster assignment per sampled completion |
| `variant-persona-clusters.jsonl` | One endpoint/persona record with cluster labels ordered by `gene_index` |
| `pool.jsonl` | Sampled endpoint/persona records selected from cluster-label tuples |
| `diagnostics.jsonl` | Selected tuple and source row for each pool record |
| `equivalence.jsonl` | Endpoint-equivalence classes and selected representatives |

PCA is computed separately for each gene because each gene has its own response distribution.  Per-gene clustering assigns each sampled completion to a selected cluster, using K-means when valid candidates exist and a single fallback cluster otherwise.  Aggregation converts sample-level labels into one cluster record per endpoint/persona row, ordered by ascending `gene_index`.

### Gene Inference and Embedding

```bash
uv run --no-cache --script tools/run_first_gene_inference_embeddings.py \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --genes sampled-genes.json \
  --persona ../common/etc/personas/generic.md \
  --samples 3 \
  --gene-index 0 \
  --out results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ
```

Check `summary.json` before passing the records to PCA.  `records_written` and `embedding_count` must equal `expected_records`, while both error counts must be zero.  The gene command writes diagnostic rows and its summary before returning a nonzero exit status for a completion or embedding failure, and the end-to-end runner stops on that failed stage.

### PCA Reduction

```bash
uv run --no-cache --script tools/run_embedding_pca.py \
  --records results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ/records.jsonl \
  --out results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ \
  --dimensions 3
```

### Per-Gene Clustering

```bash
uv run --no-cache --script tools/run_gene_pca_clustering.py \
  --pca-records results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-2-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-3-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-4-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ \
  --expected-rows-per-gene 96 \
  --expected-variants-per-gene 32 \
  --expected-samples-per-variant 3
```

### Cluster Aggregation

```bash
uv run --no-cache --script tools/aggregate_variant_persona_clusters.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.jsonl \
  --cluster-fit results/gene-clusters-YYYYMMDDTHHMMSSZ/cluster-fit.json \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --out results/variant-persona-clusters-YYYYMMDDTHHMMSSZ \
  --expected-samples-per-gene 3
```

### Tuple-Uniform Sampling

```bash
uv run --no-cache --script tools/sample-tuple-pool.py \
  results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl \
  --out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/pool.jsonl \
  --diagnostics-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/diagnostics.jsonl \
  --equivalence-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/equivalence.jsonl \
  --pool-size 20 \
  --seed 0
```

`tools/sample-tuple-pool.py` deduplicates equivalent provider endpoints before sampling.  It groups rows by OpenRouter model ID, endpoint model ID, canonical slug, Hugging Face ID, quantization, and modalities.  The grouping excludes provider name, endpoint tag, context limits, prompt and completion limits, supported parameters, price, latency, and uptime, because those fields describe provider-route capability or serving behavior rather than model-configuration identity.  For each group, it ranks representatives by fewer provider errors, higher deliberation score, fewer schema violations, fewer timeouts and context-limit errors, higher context and token capacity, higher uptime, lower latency, lower price, then stable endpoint identifiers.  Missing error counts rank as zero.  Missing deliberation score, capacity, and uptime rank below known values, while missing latency or price ranks after known values.  Standard end-to-end survivor rows contain the filter's provider-error count and deliberation score but omit schema-violation, timeout, and context-limit-error counts, so those three counts rank as zero and do not distinguish rows from that pipeline.

Each emitted row names one concrete provider endpoint.  `equivalence.jsonl` records every provider endpoint in each equivalent group, including the selected representative, provider name, endpoint tag, quantization, limits, operational fields, and cluster vector.  Use `--no-dedupe-equivalent-endpoints` only when the pool is meant to compare provider routes for the same model configuration.

After deduplication, the sampler chooses one unique cluster tuple uniformly at random, then chooses one representative row uniformly from rows with that tuple.  Sampling uses replacement by default, so repeated rows can appear in `pool.jsonl`.  The diagnostics file records the selected tuple, source row, model ID, provider, endpoint tag, quantization, equivalence class, and cumulative counts.  The diagnostic counts describe the deduplicated sampling frame unless `--no-dedupe-equivalent-endpoints` was used.

Pass `--without-replacement` when each row from the sampling frame may appear at most once.  With deduplication enabled, the maximum `--pool-size` is the number of equivalence classes.  The command fails before writing output when the requested size exceeds that frame.

## Score Model

| Field | Meaning |
| --- | --- |
| `deliberation_score` | Mean trial score over completed substantive knowledge, science, reasoning, and juror-deliberation rows. |
| `trial_scores` | Per-trial deliberation scores. |
| `deliberation_score_stddev` | Population standard deviation over trial scores. |
| `deliberation_score_min` and `deliberation_score_max` | Trial-score range. |
| `item_variation_count` | Count of items with differing outcomes, schema validity, or response values across trials. |
| `operational_metrics` | Latency, timeouts, provider errors, malformed JSON, schema violations, invalid votes, tool-call failures, context-limit errors, and cost. |

Endpoint filtering uses each variant row's `run_exit_code`, `provider_error_count`, and `deliberation_score`.  The provider-error field combines provider, rate-limit, credential, and runner errors, while the deliberation score excludes every row with a metadata error.  Equivalent-endpoint representative ranking uses the score, error, capacity, uptime, latency, price, and identifier fields present in its input.  Missing error counts rank as zero, missing deliberation score, capacity, and uptime rank below known values, and missing latency or price ranks after known values.  Standard end-to-end survivor rows omit schema-violation, timeout, and context-limit-error counts, so those fields rank as zero.  Trial scores, score spread, and item variation do not affect filtering or representative selection.

## Troubleshooting

If a validation command fails, inspect the reported item ID and schema path first.  The core item schema, response schema, fixtures, and rubric must agree before a run can produce scores.  A fixture-backed tool item should have a manifest and evidence files under the matching `sets/core20/fixtures/` directory.

If an OpenRouter run fails before it writes result rows, check credentials, model IDs, provider constraints, and endpoint availability.  The tools read `OPENROUTER_API_KEY` from the environment first and then from ignored `secrets/openrouter.api.txt`, which must contain `OPENROUTER_API_KEY=<key>` or `export OPENROUTER_API_KEY=<key>`.  A bare token is rejected.  A provider-side endpoint change can invalidate the route named by an exact-variant spec.

If a batch run stops making progress, inspect `variant-runs/*/run_eval.log`, `variant_summary.csv`, and the timeout fields in the command.  The per-request timeout controls one model call, while `--no-progress-timeout` controls a child process that stops writing output or result rows.  A child crash or scoring failure indicates an eval-tool problem that must be diagnosed before continuing.

If gene-response, PCA, clustering, aggregation, or pool sampling fails, check row counts against the command expectations.  The clustering and aggregation tools validate expected variants, samples per variant, and samples per gene so incomplete upstream data cannot produce a pool without an error.  Set those expected-count flags to the input shape for each run.

## File Reference

| Path | Contents |
| --- | --- |
| `sets/core20/questions.jsonl` | 20-question core set with knowledge, science, reasoning, instruction-following, and record-based adjudication questions |
| `sets/deliberation/questions.jsonl` | 20-question deliberation set with eight juror-deliberation questions |
| `sets/core20/fixtures/` | Evidence records for record-based questions |
| `schemas/` | JSON schemas for questions and responses |
| `rubrics/core20.md` | Deterministic checks, deliberation score, and operational metrics |
| `prompts/` | Single-juror and council-member prompt wrappers |
| `../common/etc/personas/generic.md` | Generic persona for gene-response sampling |
| `../common/etc/personas/` | Named persona text files, also read by the runtimes when a pool record names one |
| `config/` | Reference-only model lists and pool selections |
| `tools/run_eval.py` | Model call script for mock models, OpenRouter model IDs, and exact provider specs |
| `tools/score_eval.py` | Question validation and deterministic scoring |
| `tools/audit_eval.py` | Repository consistency audit |
| `tools/tool_server.py` | Local read-only evidence tools used by the runner |
| `tools/model_inventory.py` | OpenRouter model and provider endpoint inventory |
| `tools/run_model_screen.py` | Sequential direct and Pi/MCP screening for one endpoint partition |
| `tools/run_variant_batch.py` | Provider-endpoint batch evaluation |
| `tools/run_end_to_end.py` | Full selection procedure from root models to `pool.jsonl` |
| `tools/run_first_gene_inference_embeddings.py` | Gene completion and embedding collection |
| `tools/run_embedding_pca.py` | PCA reduction for embedded gene responses |
| `tools/run_gene_pca_clustering.py` | Per-gene K-means clustering |
| `tools/aggregate_variant_persona_clusters.py` | Aggregation from sample-level clusters to endpoint/persona cluster records |
| `tools/sample-tuple-pool.py` | Tuple-uniform sampler for variant/persona cluster rows |
| `tools/clusters-graph.py` | Renders `clusters.csv` from the per-gene clustering stage as a faceted chart |
| `variants/filtered-20260529/` | Checked-in accepted endpoint snapshot |
| `genes.json` and `sampled-genes.json` | Source gene list and sampled gene subset used by the clustering procedure |
| `results/` | Generated evaluation and pool-construction files.  Git ignores this directory except for `.gitkeep`. |

## Scope

The core eval set checks JSON format, instruction following, knowledge, quantitative reasoning, record use, and evidence citations.  The endpoint-variant tooling evaluates OpenRouter routed products under explicit provider and quantization constraints.  Full adjudication runs are in `adc/`, `arb/`, and `arbd/`.

## Detailed References

[Sampling Runbook](docs/sampling-runbook.md) contains the staged procedure from OpenRouter root sampling through tuple-uniform pool sampling.  [OpenRouter Inventory Procedure](docs/model-inventory.md) explains provider-endpoint identity, routing constraints, metadata fields, and interpretation limits.  [Core20 Rubric](rubrics/core20.md) defines response schemas, deterministic checks, deliberation scoring, and operational metrics.
