# Sampling Runbook

This procedure starts with OpenRouter model IDs and produces a JSONL pool of provider-endpoint and persona records.  Run each command from `model-pool/`, and place generated files under `results/`.  OpenRouter requests read `OPENROUTER_API_KEY` from the environment or an ignored `secrets/openrouter.api.txt` file containing `OPENROUTER_API_KEY=<key>` or `export OPENROUTER_API_KEY=<key>`.  Gene-response embeddings read `OPENAI_API_KEY` from the environment or the corresponding assignments in ignored `secrets/openai.api.txt`.  A bare token in either credential file is rejected.

## End-to-End Runner

`tools/run_end_to_end.py` runs inventory, runtime tool-use screening, endpoint eval, filtering, gene inference, PCA, clustering, cluster aggregation, and tuple sampling in order.  It accepts explicit repeated `--model-id` values or samples `--root-count` catalog models with `--root-seed`.  Each stage writes beneath `results/<run-id>/`, and the top-level `summary.json` identifies the completed stages and their summaries.

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

The command defaults to five root models, one screening process, one eval trial, two genes, one sample per endpoint/gene pair, three PCA dimensions, a cluster range from `2` through `10`, and twenty pool rows.  `make pool` sets eight screening processes and eight eval processes.  Set the model IDs or root sample, question file, prompt, process counts, trial count, filter criteria, genes, sample count, PCA dimensions, cluster range, pool size, and seeds for the intended pool.  `--stop-after` accepts `inventory`, `screen`, `eval`, `filter`, `genes`, `pca`, `clusters`, `aggregate`, or `pool` and returns after writing that stage's summary.

The eval stage applies a per-attempt `--timeout` and a per-child `--eval-no-progress-timeout`.  Completion requests use the council client's four-attempt policy for HTTP 408, 409, 429, 5xx, timeout, and network failures, with delays of zero, five, and thirty seconds.  `--eval-variant-timeout` adds an absolute limit for one endpoint eval and must be at least the no-progress timeout.  Each gene command finishes every configured sample and records request errors.  After all genes finish, the runner excludes a configuration from every gene when any of its completion or embedding requests failed.  PCA and later stages use the common eligible set.  The runner caps the requested PCA dimensions at the eligible embedding-row count unless `--strict-pca-dimensions` makes the mismatch an error.

## Staged Procedure

The commands below run the individual data-producing stages.  Filtering runs through `tools/run_end_to_end.py`.  Each later command accepts the output paths shown by the preceding stages.

### Endpoint Inventory

Inventory explicit model IDs when the root set has already been chosen.  The script fetches the model catalog and the endpoint response for every selected model, writes raw responses, and normalizes one row per provider endpoint.  HTTP 408, 429, 500, 502, 503, and 504 responses, URL errors, and timeouts use the configured retries.  Other HTTP failures and malformed successful responses abort immediately.

```bash
uv run --no-cache --script tools/model_inventory.py \
  --run-id model-roots-10-YYYYMMDDTHHMMSSZ \
  --model-id root/model-a \
  --model-id root/model-b
```

The inventory directory contains `raw/models.json`, percent-encoded files under `raw/endpoints/`, `endpoint_variants.jsonl`, `endpoint_variants.csv`, and `summary.json`.  Keep unknown-quantization endpoints separate because the provider, endpoint tag, limits, pricing, supported parameters, and runtime behavior can differ.  Inspect `summary.json` for the selected IDs and endpoint count before screening.

### Runtime Tool-Use Screen

Build `.bin/model-config-screen`, then pass the inventory rows to the Python coordinator.  Each configuration receives a Quick-style direct `submit_council_vote` check and a Pi/MCP check using the ARB council tool sequence.  Both checks use the shared model executor.  Pi reaches it through the runtime's local model gateway, and `model-requests.jsonl` records those provider calls.  The output directory must be absent or empty.

```bash
make screen-command
uv run --no-cache --script tools/run_model_screen.py \
  --variants results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-screen-YYYYMMDDTHHMMSSZ \
  --screen-command ../.bin/model-config-screen
```

The screen preserves the exact provider route, fallback policy, request parameters, Pi transcript, MCP calls, usage, and cost for each configuration.  After MCP accepts the Pi vote, the screen allows five seconds for Pi to exit and then stops the test container because ARB has completed the council opportunity at that point.  It writes passing rows to `endpoint_variants.jsonl`, failures to `rejected_variants.jsonl`, per-configuration summaries to `results.jsonl`, and aggregate counts to `summary.json`.  Standard-output events report cumulative cost after every configuration and report an in-progress configuration every sixty seconds.

### Endpoint Evals

The batch runner creates one exact request spec and one eval directory for each screened endpoint variant.  It pins the endpoint route, disables fallback, sets `require_parameters: true`, and includes the quantization constraint when the inventory reports a known value.  The output directory must be absent or empty.

```bash
uv run --no-cache --script tools/run_variant_batch.py \
  --variants results/model-roots-10-screen-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-eval-YYYYMMDDTHHMMSSZ \
  --questions sets/core20/questions.jsonl \
  --prompt prompts/juror-single.md \
  --trials 3 \
  --timeout 90 \
  --tool-mode function
```

A successful directory under `variant-runs/` contains `raw_results.jsonl`, `scores.json`, and `run_eval.log`.  The evaluator requests JSON-object responses for ordinary questions.  Record questions expose the evidence functions without a response-format parameter, matching the direct-council request, and the scorer validates the final JSON answer.  The batch directory also contains request files under `specs/`, per-endpoint results in `variant_summary.csv`, and aggregate counts in `summary.json`.  Timed-out endpoints remain in `variant_summary.csv` so the filter can reject them, while a child command or scoring failure gives the batch a nonzero exit status.

### Endpoint Filtering

The end-to-end filter joins inventory rows to `variant_summary.csv` by source index.  It rejects a nonzero per-variant `run_exit_code`, a `provider_error_count` different from `--filter-provider-error-count`, and a deliberation score outside the selected threshold.  `--filter-min-deliberation-score` applies an inclusive minimum, while `--filter-deliberation-score-gt` applies a strict greater-than threshold.  The provider-error count includes provider, rate-limit, credential, and runner errors.  The deliberation score excludes rows with metadata errors, including timeouts, and omits a trial that has no completed deliberation rows.  The default criteria require zero counted errors and a deliberation score of at least `0.90`.

Run the end-to-end command with `--stop-after filter` to produce a filtered set.  The filter directory contains accepted rows in `endpoint_variants.jsonl` and `endpoint_variants.csv`, rejection records in `removed_variants.jsonl`, and counts, source paths, criteria, and accepted source indexes in `summary.json`.  A filter that accepts no endpoints fails the run.

To continue from completed screening and evaluation data, pass `--start-at filter`, `--existing-screen-variants`, and `--existing-eval-summary`.  The two source files must contain matching `combined_index` values.  The command creates a new run directory for the filter and every later stage.

The checked-in snapshot under `variants/filtered-20260529/` contains `endpoint_variants.jsonl`, `endpoint_variants.csv`, and `summary.json`.  Run the end-to-end command for claims about current provider behavior because routes, availability, pricing, and behavior can change.  Pass the resulting filtered `endpoint_variants.jsonl` to every downstream stage.

### Gene Inference and Embeddings

Run one gene index at a time against the accepted endpoint set.  Each sample uses the endpoint's exact route policy and records the completion, request parameters, route metadata, status, and embedding.  The command writes directly to `records.jsonl` and summarizes expected rows, status counts, errors, and embeddings in `summary.json`.  Its output directory must be absent or empty.

```bash
uv run --no-cache --script tools/run_first_gene_inference_embeddings.py \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --genes sampled-genes.json \
  --persona ../common/etc/personas/generic.md \
  --samples 3 \
  --gene-index 0 \
  --out results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ
```

Repeat the command with a distinct output directory for every selected gene index.  `--gene-processes` controls how many separate gene commands the end-to-end runner starts together.  It waits for each group before starting the next group.  A standalone gene command returns a nonzero exit status when a completion or embedding failed.  The end-to-end runner passes `--allow-record-errors`, which makes the command return success after writing every expected record.  Once all genes finish, the runner writes the shared eligible endpoint set, excluded endpoint records with their errors, and per-gene eligible records under `gene-filter/`.  A configuration enters PCA only when every selected gene contains the expected sample indexes, successful statuses, and embeddings for that configuration.

### PCA

PCA runs separately for each gene because each prompt produces its own response distribution.  `pca-records.jsonl` contains the projected rows, `pca-fit.json` contains the fitted components and variance data, and `summary.json` reports source counts and dimensions.  The requested dimension count cannot exceed the number of usable embedding rows.

```bash
uv run --no-cache --script tools/run_embedding_pca.py \
  --records results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ/records.jsonl \
  --out results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ \
  --dimensions 3
```

### Per-Gene Clustering

The clustering tool validates row counts, endpoint coverage, samples per endpoint, and PCA dimensions when their expected values are supplied.  It evaluates K-means candidates separately for each gene and selects the candidate with the highest silhouette score.  Too few rows or no valid candidate produces a single fallback cluster.  The tool writes `clusters.jsonl`, `clusters.csv`, `cluster-fit.json`, and `summary.json`, and cluster labels have meaning only within their gene index.

```bash
uv run --no-cache --script tools/run_gene_pca_clustering.py \
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
env MPLBACKEND=Agg uv run --no-cache --script tools/clusters-graph.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.csv \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.png
```

### Cluster Vector Aggregation

Aggregation produces one cluster vector for each endpoint and persona.  For each gene, it chooses a unanimous label, the unique highest-count label, or the label of the sample nearest its assigned cluster center when the highest counts tie.  The output record calls a unique nonunanimous winner `majority`, including a plurality, and the output directory contains `variant-persona-clusters.jsonl` and `summary.json`.

```bash
uv run --no-cache --script tools/aggregate_variant_persona_clusters.py \
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
uv run --no-cache --script tools/sample-tuple-pool.py \
  results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl \
  --out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/pool.jsonl \
  --diagnostics-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/diagnostics.jsonl \
  --equivalence-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/equivalence.jsonl \
  --pool-size 20 \
  --seed 0
```

The equivalence key uses OpenRouter model ID, endpoint model ID, canonical slug, Hugging Face ID, quantization, and input and output modalities.  Representative selection ranks provider-error count, deliberation score, capacity, uptime, latency, price, and stable endpoint identifiers for standard end-to-end rows.  It can also rank schema violations, timeouts, and context-limit errors when the sampler input contains those fields.  Missing error counts rank as zero.  Missing deliberation score, capacity, and uptime rank below known values, while missing latency or price ranks after known values.  `equivalence.jsonl` records the members and selected representative of every class.  `--no-dedupe-equivalent-endpoints` retains provider routes as separate sampling rows when the pool is intended to compare those routes.

`pool.jsonl` contains the sampled request-spec records, and `diagnostics.jsonl` identifies the tuple and source row selected for each output row.  With `--without-replacement`, the requested pool size cannot exceed the post-deduplication sampling frame.  The sampler validates the input rows and frame size before opening the output files.

The sampled rows preserve the gene stage's `persona_path`.  With the default `--persona`, a generated row contains `../common/etc/personas/generic.md`, a path relative to `model-pool/`.  A runtime resolves a relative persona path beside `pool.jsonl` and then under `<pool-dir>/../../etc/`, so it cannot resolve that default value from the nested result directories used by the documented sampler and end-to-end commands.  Pool construction preserves that path unchanged.  The installed default at `common/data/personas/pool.jsonl` instead contains `personas/generic.md`, which resolves through the shared-tree path.
