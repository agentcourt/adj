# Sampling Runbook

This procedure starts with OpenRouter model IDs and produces a JSONL pool of provider-endpoint and persona records.  Run each command from `model-pool/`, and place generated files under `results/`.  OpenRouter requests require `OPENROUTER_API_KEY`, while gene-response embeddings require `OPENAI_API_KEY`.

## End-To-End Runner

`tools/run_end_to_end.py` runs inventory, endpoint eval, filtering, gene inference, PCA, clustering, cluster aggregation, and tuple sampling in order.  It accepts explicit repeated `--model-id` values or samples `--root-count` catalog models with `--root-seed`.  Each stage writes beneath `results/<run-id>/`, and the top-level `summary.json` identifies the completed stages and their summaries.

```bash
uv run --script tools/run_end_to_end.py \
  --run-id e2e-YYYYMMDDTHHMMSSZ \
  --root-count 5 \
  --root-seed 0 \
  --prompt prompts/juror-single.md \
  --eval-trials 1 \
  --gene-count 2 \
  --samples-per-gene 1 \
  --pca-dimensions 3 \
  --min-k 2 \
  --max-k 4 \
  --pool-size 5
```

The default sizes support a bounded test of the procedure.  Set the model IDs or root sample, question file, prompt, trial count, filter criteria, genes, sample count, PCA dimensions, cluster range, pool size, and seeds for the intended pool.  `--stop-after` accepts `inventory`, `eval`, `filter`, `genes`, `pca`, `clusters`, `aggregate`, or `pool` and returns after writing that stage's summary.

The eval stage applies a per-request `--timeout` and a per-child `--eval-no-progress-timeout`.  `--eval-variant-timeout` adds an absolute limit for one endpoint eval and must be at least the no-progress timeout.  The runner rejects an incomplete gene stage before PCA, including missing records, completion errors, embedding errors, or missing embeddings.

## Staged Procedure

The following commands expose each data-producing tool for inspection and focused runs.  The end-to-end runner provides the filter stage because the repository has no separate filter command.  Paths in later commands must refer to outputs from the same selection run.

### Endpoint Inventory

Inventory explicit model IDs when the root set has already been chosen.  The script fetches the model catalog and the endpoint response for every selected model, writes raw responses, and normalizes one row per provider endpoint.  A failed catalog or endpoint request aborts the inventory after the configured retries.

```bash
uv run --script tools/model_inventory.py \
  --run-id model-roots-10-YYYYMMDDTHHMMSSZ \
  --model-id root/model-a \
  --model-id root/model-b
```

The inventory directory contains `raw/models.json`, percent-encoded files under `raw/endpoints/`, `endpoint_variants.jsonl`, `endpoint_variants.csv`, and `summary.json`.  Keep unknown-quantization endpoints separate because the provider, endpoint tag, limits, pricing, supported parameters, and runtime behavior can differ.  Inspect `summary.json` for the selected IDs and endpoint count before starting evals.

### Endpoint Evals

The batch runner creates one exact request spec and one eval directory for each endpoint variant.  It pins the endpoint route, disables fallback, requires supported parameters, and includes a known quantization constraint.  The output directory must be absent or empty.

```bash
uv run --script tools/run_variant_batch.py \
  --variants results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-eval-YYYYMMDDTHHMMSSZ \
  --questions sets/core20/questions.jsonl \
  --prompt prompts/juror-single.md \
  --trials 3 \
  --timeout 90
```

A successful directory under `variant-runs/` contains `raw_results.jsonl`, `scores.json`, and `run_eval.log`.  The batch directory also contains request files under `specs/`, per-endpoint results in `variant_summary.csv`, and aggregate counts in `summary.json`.  Timed-out endpoints remain in `variant_summary.csv` so the filter can reject them, while a child command or scoring failure gives the batch a nonzero exit status.

### Endpoint Filtering

The end-to-end filter joins inventory rows to `variant_summary.csv` by source index.  It rejects nonzero eval exit codes, a provider-error count different from `--filter-provider-error-count`, and a deliberation score below `--filter-min-deliberation-score`.  The default criteria require zero provider errors and a deliberation score of at least `0.90`.

Run the end-to-end command with `--stop-after filter` to produce a filtered set.  The filter directory contains accepted rows in `endpoint_variants.jsonl` and `endpoint_variants.csv`, rejection records in `removed_variants.jsonl`, and counts, source paths, criteria, and accepted source indexes in `summary.json`.  A filter that accepts no endpoints fails the run.

The checked-in snapshot under `variants/filtered-20260529/` retains `endpoint_variants.jsonl`, `endpoint_variants.csv`, and `summary.json`.  Use a new end-to-end run for current-provider claims because routes, availability, pricing, and behavior can change.  Pass that run's filtered `endpoint_variants.jsonl` to every downstream stage.

### Gene Inference And Embeddings

Run one gene index at a time against the accepted endpoint set.  Each sample uses the endpoint's exact route policy and records the completion, request parameters, route metadata, status, and embedding.  The command writes directly to `records.jsonl` and summarizes expected rows, status counts, errors, and embeddings in `summary.json`.

```bash
uv run --script tools/run_first_gene_inference_embeddings.py \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --genes sampled-genes.json \
  --persona ../common/etc/personas/generic.md \
  --samples 3 \
  --gene-index 0 \
  --out results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ
```

Repeat the command with a distinct output directory for every selected gene index.  Before PCA, require `records_written == expected_records`, `embedding_count == expected_records`, and zero completion and embedding errors.  The end-to-end runner enforces those conditions and stops on the first incomplete gene stage.
The gene command writes its diagnostic rows and summary, then returns a nonzero exit status when a completion or embedding failed.

### PCA

PCA runs separately for each gene because each prompt produces its own response distribution.  `pca-records.jsonl` contains the projected rows, `pca-fit.json` contains the fitted components and variance data, and `summary.json` reports source counts and dimensions.  The requested dimension count cannot exceed the number of usable embedding rows.

```bash
uv run --script tools/run_embedding_pca.py \
  --records results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ/records.jsonl \
  --out results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ \
  --dimensions 3
```

### Per-Gene Clustering

The clustering tool validates row counts, endpoint coverage, samples per endpoint, and PCA dimensions when their expected values are supplied.  It fits K-means separately for each gene, selects a candidate by silhouette score, and writes `clusters.jsonl`, `clusters.csv`, `cluster-fit.json`, and `summary.json`.  Cluster labels have meaning only within their gene index.

```bash
uv run --script tools/run_gene_pca_clustering.py \
  --pca-records results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-2-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-3-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-4-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ \
  --expected-rows-per-gene 96 \
  --expected-variants-per-gene 32 \
  --expected-samples-per-variant 3 \
  --pca-dimensions 3 \
  --min-k 3 \
  --max-k 10
```

`tools/clusters-graph.py` renders the inspection CSV as a faceted chart.  Each provider occupies one row, each gene occupies one column, and model markers show `pc1` against `pc2`.  Set a noninteractive Matplotlib backend for file-only rendering.

```bash
env MPLBACKEND=Agg uv run --script tools/clusters-graph.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.csv \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.png
```

### Cluster Vector Aggregation

Aggregation produces one cluster vector for each endpoint and persona.  For each gene, it chooses a unanimous label, a majority label, or the label of the sample nearest its assigned cluster center when all samples differ.  The output directory contains the canonical `variant-persona-clusters.jsonl` and `summary.json`.

```bash
uv run --script tools/aggregate_variant_persona_clusters.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.jsonl \
  --cluster-fit results/gene-clusters-YYYYMMDDTHHMMSSZ/cluster-fit.json \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --out results/variant-persona-clusters-YYYYMMDDTHHMMSSZ \
  --expected-samples-per-gene 3
```

Check that every accepted endpoint has one aggregate row and that every `clusters` array follows ascending `gene_index`.  `summary.json` reports the input row count, output row count, gene order, and aggregation-method counts.  An incomplete grouped gene or sample set, a referenced endpoint absent from the variant file, or a missing fit record aborts aggregation.

### Tuple-Uniform Pool Sampling

The tuple sampler deduplicates equivalent provider endpoints before sampling.  It chooses a distinct cluster tuple uniformly, then chooses a representative endpoint/persona row uniformly within that tuple.  Sampling uses replacement unless `--without-replacement` is present.

```bash
uv run --script tools/sample-tuple-pool.py \
  results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl \
  --out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/pool.jsonl \
  --diagnostics-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/diagnostics.jsonl \
  --equivalence-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/equivalence.jsonl \
  --pool-size 20 \
  --seed 0
```

The equivalence key uses model identity, quantization, and modalities.  Representative selection ranks operational results, capacity, availability, latency, price, and stable endpoint identifiers, while `equivalence.jsonl` records the members and selected representative of every class.  `--no-dedupe-equivalent-endpoints` retains provider routes as separate sampling rows when the pool is intended to compare those routes.

`pool.jsonl` contains the sampled request-spec records, and `diagnostics.jsonl` identifies the tuple and source row selected for each output row.  With `--without-replacement`, the requested pool size cannot exceed the post-deduplication sampling frame.  The sampler validates the input rows and frame size before opening the output files.
