# Sampling Runbook

This runbook starts with OpenRouter root-model sampling and ends with a JSONL pool sampled from variant/persona cluster vectors.  Run every command from `model-pool/`.

## Scope

The sampling frame is the OpenRouter model catalog from `/api/v1/models`.  A root model is an OpenRouter model ID, such as `deepseek/deepseek-v4-flash`.  A model variant is one provider endpoint row from `/api/v1/models/{author}/{slug}/endpoints`.

Endpoint variants remain separate evaluation units.  Preserve provider, endpoint tag, quantization, context limits, supported parameters, pricing, status fields, and raw OpenRouter JSON.  Treat `quantization: "unknown"` as endpoint-specific unknown state, because two unknown-quantization endpoints can differ in provider, serving engine, limits, wrappers, or behavior.

This runbook uses `tools/sample-tuple-pool.py` for the final pool.  Other pool samplers are outside this procedure.  The runbook assumes `OPENROUTER_API_KEY` and `OPENAI_API_KEY` are available through the environment or ignored files under `secrets/`.

## Output Names

Use timestamped run IDs and write all generated run artifacts under `results/`.  Keep stable, human-readable prefixes so downstream paths make the stage clear.

```bash
ROOT_SAMPLE_RUN=results/model-roots-10-YYYYMMDDTHHMMSSZ
EVAL_RUN=results/model-roots-10-eval-YYYYMMDDTHHMMSSZ
FILTERED_DIR=variants/filtered-20260529
GENE_PREFIX=results/gene
CLUSTER_RUN=results/gene-clusters-YYYYMMDDTHHMMSSZ
VECTOR_RUN=results/variant-persona-clusters-YYYYMMDDTHHMMSSZ
POOL_RUN=results/sample-tuple-pool-YYYYMMDDTHHMMSSZ
```

## End-To-End Runner

Use `tools/run_end_to_end.py` when the whole pipeline should run as one job.  The runner copies the question file, referenced evidence records, prompt, gene file, and persona into `inputs/`, records normalized result parameters and source paths in an immutable `manifest.json`, records every command in `commands.jsonl`, and writes a final `summary.json`.  It supports `--resume`, `--dry-run`, and `--stop-after`.

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

The default runner shape is a small test run.  Increase `--eval-trials`, `--gene-count`, `--samples-per-gene`, and `--pool-size` for a production pool.  The runner caps PCA dimensions to the available embedding rows unless `--strict-pca-dimensions` is set.  The eval stage uses `--timeout` per request and `--eval-no-progress-timeout` per variant child process.

Use `--resume` only with an existing recorded run.  Before writing, the runner requires the current result parameters and original input paths to match the manifest, then compares each current input byte-for-byte with its saved copy.  It also validates the command log, progress records, completed stage records, pending provider-stage records, and every completed stage's derivation from its recorded inputs.  Inventory, eval, and gene stages retain validated provider results and continue missing work.  An interrupted local filter, PCA, clustering, aggregation, or pool attempt is moved under `interrupted-local-stages/` and recomputed from its recorded inputs.  A directory lock prevents a second runner from writing the same run while the runner or one of its child processes remains active.

## Root Sampling

Fetch or reuse a saved OpenRouter `/api/v1/models` catalog.  Sort model IDs lexicographically, choose roots with a deterministic `random.Random(seed).sample(...)`, and record the seed, source catalog, exclusion set, selected roots, command, run ID, and output files.  For incremental samples, exclude roots already present in the active combined root-model catalog unless the run requires overlap.

Use explicit `--model-id` arguments for the inventory run after selecting roots.  `tools/model_inventory.py --sample-models` can sample roots, but it does not accept an exclusion set.  Explicit IDs make incremental sampling reproducible.

```bash
uv run python - <<'PY'
import json
import random
from pathlib import Path

catalog_path = Path("results/PRIOR-RUN/raw/models.json")
exclude_summary_path = Path("results/ACTIVE-COMBINED-CATALOG/summary.json")
sample_size = 10
seed = 2

models = json.loads(catalog_path.read_text())["data"]
exclude = set(json.loads(exclude_summary_path.read_text())["model_roots"])
ordered = sorted(model["id"] for model in models)
eligible = [model_id for model_id in ordered if model_id not in exclude]

sample = sorted(random.Random(seed).sample(eligible, sample_size))
print("\n".join(sample))
PY
```

If the exclusion source is a single inventory run rather than a combined catalog, use `selected_model_ids` instead of `model_roots`.

Record the exact source catalog path, excluded roots, seed, selected roots, and reason that overlap was or was not allowed.  That record is the audit trail for the random choice.

## Endpoint Inventory

Run a static OpenRouter endpoint inventory over the selected root model IDs.  The inventory fetches the catalog, fetches endpoint metadata for each selected root, preserves raw JSON, and emits one normalized row per endpoint variant.  The inventory step does not run inference.

```bash
uv run --script tools/model_inventory.py \
  --run-id model-roots-10-YYYYMMDDTHHMMSSZ \
  --model-id root/model-a \
  --model-id root/model-b
```

Expected files:

| File | Purpose |
| --- | --- |
| `raw/models.json` | Raw `/api/v1/models` response for the snapshot. |
| `raw/endpoints/*.json` | Raw endpoint responses for selected roots. |
| `endpoint_variants.jsonl` | Canonical normalized endpoint-variant rows. |
| `endpoint_variants.csv` | Tabular inspection view. |
| `summary.json` | Counts, selected root IDs, provider counts, and errors. |
| `summary.md` | Human-readable inventory summary. |

Verify the inventory before using it.  The root count should equal the selected root count, and endpoint fetch errors should be zero or explicitly documented.  Unknown quantization rows are valid endpoint variants, not errors.

```bash
jq '{selected_model_count, endpoint_variant_count, endpoint_fetch_error_count, selected_model_ids}' \
  results/model-roots-10-YYYYMMDDTHHMMSSZ/summary.json
```

## Variant Evals

Evaluate endpoint variants with exact OpenRouter routing constraints.  Each variant row must become the request spec for that variant: `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, and `provider.quantizations` when quantization is known.  `tools/run_eval.py` handles that policy when called through `tools/run_variant_batch.py`.

```bash
uv run --script tools/run_variant_batch.py \
  --variants results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-eval-YYYYMMDDTHHMMSSZ \
  --questions sets/core20/questions.jsonl \
  --prompt prompts/juror-single.md \
  --trials 3 \
  --timeout 90
```

The batch runner writes one model-spec file per variant and one run directory per attempted variant.  A new run refuses a nonempty output directory and saves exact copies of the endpoint inventory, questions, referenced evidence records, and prompt under `inputs/`.  A timed-out variant stays in the eval records, and the filtering stage removes it before gene inference.

Continuation requires the same command plus `--resume`.  The runner first compares the current sources with the saved copies, then validates every progress row's JSON, index, endpoint identity, status, exit code, exact variant spec, log, raw results, `run.json`, and score files.  It retries `command_failed` variants from evaluation and retries `score_failed` variants from scoring without repeating the validated provider requests.  A `timed_out` variant remains a terminal evaluated outcome.  If evaluation or scoring completed before the parent recorded progress, resume adopts the validated artifacts and continues from the next unfinished operation.  A run with a current command or scoring failure exits nonzero even though a later explicit resume can retry it.

Expected files:

| File | Purpose |
| --- | --- |
| `manifest.json` | Immutable normalized parameters and source-to-snapshot records. |
| `inputs/` | Exact endpoint, question, evidence, and prompt copies. |
| `specs/*.json` | Exact OpenRouter variant specs used for requests. |
| `variant-runs/*/run_eval.log` | Child-process output for one variant. |
| `variant-runs/*/raw_results.jsonl` | Raw eval results for one variant. |
| `variant-runs/*/scores.json` | Scored summary for one variant. |
| `progress.jsonl` | Per-variant completion records. |
| `variant_summary.csv` | Tabular per-variant summary. |
| `summary.json` | Batch status and success counts. |

Verify that every attempted variant has a progress row.  Scored variants have `scores.json`; timed-out or failed variants have `run_exit_code != 0` or no deliberation score.  Inspect provider errors, schema violations, timeouts, context-limit errors, and deliberation score as separate facts.

## Combine Incremental Runs

When a sample is incremental, combine the previous and new endpoint catalogs before filtering.  Preserve one flat combined variant list and one flat eval directory.  Renumber combined eval indexes only as display indexes; keep the original endpoint-variant records and source run IDs.

For combined runs, write a `summary.json` that records source catalogs, source eval runs, combined root count, combined variant count, copied eval run count, and aggregate eval counts.  Verify that every combined variant has one eval summary row.  Keep source directories unchanged.

## Filter Variants

Filter endpoint variants after evals, using explicit operational and deliberation criteria.  The current checked-in survivor set uses `provider_error_count == 0` and `deliberation_score >= 0.90`.  The filter output uses `variants/filtered-20260529/` as the active survivor set for gene inference unless a new run replaces it.

The current repository contains `variants/filtered-20260529/` as the active survivor set.  For a new run, create the same files from the endpoint-variant catalog, eval summaries, and exact spec files.  Each survivor variant row should include `combined_index`, `filter_provider_error_count`, and `filter_deliberation_score`.  Keep removed-variant records for timed-out, failed, provider-error, and low-score variants.

Use this command for a normal inventory/eval pair produced by `tools/model_inventory.py` and `tools/run_variant_batch.py`.  For a combined run, set `variant_path`, `eval_summary_path`, and `specs_dir` to the combined paths; the command accepts either CSV or JSONL eval summaries.  If preserving the existing `variants/filtered-20260529/` directory, set `out` to a timestamped path and use that path in later commands.

```bash
uv run python - <<'PY'
import csv
import json
import shutil
from pathlib import Path

variant_path = Path("results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl")
eval_summary_path = Path("results/model-roots-10-eval-YYYYMMDDTHHMMSSZ/variant_summary.csv")
specs_dir = Path("results/model-roots-10-eval-YYYYMMDDTHHMMSSZ/specs")
out = Path("variants/filtered-20260529")
min_score = 0.90
required_provider_errors = 0

def load_jsonl(path):
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]

def load_eval_rows(path):
    if path.suffix == ".csv":
        with path.open(newline="") as handle:
            return list(csv.DictReader(handle))
    return load_jsonl(path)

def row_index(row, fallback):
    value = row.get("combined_index") or row.get("index") or fallback
    return int(value)

def int_field(row, key):
    value = row.get(key)
    return int(value) if value not in (None, "") else 0

def float_field(row, key):
    value = row.get(key)
    if value in (None, ""):
        return None
    return float(value)

variants = load_jsonl(variant_path)
summaries = load_eval_rows(eval_summary_path)
summary_by_index = {row_index(row, None): row for row in summaries}

survivor_variants = []
survivor_summaries = []
survivor_manifest = []
removed_variants = []

def removed_row(variant, index, reason, **extra):
    row = {
        "combined_index": index,
        "openrouter_model_id": variant.get("openrouter_model_id"),
        "provider_name": variant.get("provider_name"),
        "endpoint_tag": variant.get("endpoint_tag"),
        "quantization": variant.get("quantization"),
        "reason": reason,
    }
    row.update(extra)
    return row

for position, variant in enumerate(variants, start=1):
    index = row_index(variant, position)
    eval_row = summary_by_index[index]
    run_exit_code = int_field(eval_row, "run_exit_code")
    if run_exit_code != 0:
        removed_variants.append(removed_row(
            variant,
            index,
            "run_exit_code",
            run_exit_code=run_exit_code,
            variant_status=eval_row.get("variant_status"),
            timeout_kind=eval_row.get("timeout_kind"),
        ))
        continue
    provider_errors = int_field(eval_row, "provider_error_count")
    score = float_field(eval_row, "deliberation_score")
    if provider_errors != required_provider_errors:
        removed_variants.append(removed_row(
            variant,
            index,
            "provider_error_count",
            provider_error_count=provider_errors,
        ))
        continue
    if score is None or score < min_score:
        removed_variants.append(removed_row(
            variant,
            index,
            "deliberation_score",
            deliberation_score=score,
        ))
        continue

    survivor = dict(variant)
    survivor["combined_index"] = index
    survivor["filter_provider_error_count"] = provider_errors
    survivor["filter_deliberation_score"] = score
    survivor_variants.append(survivor)

    summary_row = dict(eval_row)
    summary_row["combined_index"] = index
    summary_row["provider_error_count"] = provider_errors
    summary_row["deliberation_score"] = score
    survivor_summaries.append(summary_row)

    survivor_manifest.append({
        "combined_index": index,
        "endpoint_variant_id": survivor.get("endpoint_variant_id"),
        "openrouter_model_id": survivor.get("openrouter_model_id"),
        "provider_name": survivor.get("provider_name"),
        "endpoint_tag": survivor.get("endpoint_tag"),
        "quantization": survivor.get("quantization"),
        "run_dir": eval_row.get("variant_run_dir") or eval_row.get("run_dir"),
    })

if out.exists():
    raise SystemExit(f"{out} already exists")
spec_out = out / "specs"
spec_out.mkdir(parents=True)

for row in survivor_summaries:
    matches = sorted(specs_dir.glob(f"{int(row['combined_index']):02d}-*.json"))
    if len(matches) != 1:
        raise SystemExit(f"expected one spec for combined index {row['combined_index']}, found {len(matches)}")
    shutil.copy2(matches[0], spec_out / matches[0].name)

def write_jsonl(path, rows):
    with path.open("w") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")

write_jsonl(out / "endpoint_variants.jsonl", survivor_variants)
write_jsonl(out / "variant_summary.jsonl", survivor_summaries)
write_jsonl(out / "manifest.jsonl", survivor_manifest)
write_jsonl(out / "removed_variants.jsonl", removed_variants)

fields = [
    "combined_index",
    "openrouter_model_id",
    "provider_name",
    "endpoint_tag",
    "quantization",
    "endpoint_variant_id",
    "filter_provider_error_count",
    "filter_deliberation_score",
]
with (out / "endpoint_variants.csv").open("w", newline="") as handle:
    writer = csv.DictWriter(handle, fieldnames=fields, extrasaction="ignore")
    writer.writeheader()
    writer.writerows(survivor_variants)

summary = {
    "source_variant_file": str(variant_path),
    "source_eval_summary_file": str(eval_summary_path),
    "source_specs_dir": str(specs_dir),
    "filter_criteria": {
        "provider_error_count": required_provider_errors,
        "deliberation_score_minimum": min_score,
    },
    "total_variants": len(variants),
    "survivor_count": len(survivor_variants),
    "survivor_combined_indexes": [row["combined_index"] for row in survivor_summaries],
    "removed_count": len(removed_variants),
    "removed_variant_indexes": [row["combined_index"] for row in removed_variants],
    "outputs": [
        "endpoint_variants.jsonl",
        "endpoint_variants.csv",
        "variant_summary.jsonl",
        "manifest.jsonl",
        "removed_variants.jsonl",
        "specs/*.json",
        "summary.json",
    ],
}
(out / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False) + "\n")
print({"survivor_count": len(survivor_variants)})
PY
```

Expected files:

| File | Purpose |
| --- | --- |
| `variants/filtered-20260529/endpoint_variants.jsonl` | Full survivor endpoint-variant records. |
| `variants/filtered-20260529/endpoint_variants.csv` | Survivor inspection table. |
| `variants/filtered-20260529/variant_summary.jsonl` | Survivor eval summaries. |
| `variants/filtered-20260529/manifest.jsonl` | Survivor eval manifest rows. |
| `variants/filtered-20260529/specs/*.json` | Exact survivor variant specs. |
| `variants/filtered-20260529/summary.json` | Criteria, source paths, survivor count, and indexes. |

Verify counts and criteria.

```bash
uv run python - <<'PY'
import json
from pathlib import Path

out = Path("variants/filtered-20260529")
summary = json.loads((out / "summary.json").read_text())
variants = [json.loads(line) for line in (out / "endpoint_variants.jsonl").read_text().splitlines() if line.strip()]
evals = [json.loads(line) for line in (out / "variant_summary.jsonl").read_text().splitlines() if line.strip()]
specs = list((out / "specs").glob("*.json"))

assert len(variants) == len(evals) == len(specs) == summary["survivor_count"]
assert all(int(row["provider_error_count"]) == 0 for row in evals)
assert all(float(row["deliberation_score"]) >= 0.90 for row in evals)
print({"survivor_count": summary["survivor_count"], "criteria_ok": True})
PY
```

## Genes And Samples

Use `genes.json` as the source gene list and `sampled-genes.json` as the sampled gene list for a run.  A gene is a behavior-eliciting prompt.  The current checked-in sampling workflow uses four distinct genes and three completions per `gene + endpoint variant + persona`.

The current persona is `../common/etc/personas/generic.md`.  Keep one persona unless the run design calls for persona variation.  The gene runner derives `persona_id` from the saved persona filename and records the saved persona path in every row.  Record gene count, persona count, endpoint-variant count, samples per combination, and expected completion count in the run output.

For the current 32-survivor set:

```text
32 endpoint variants * 1 persona * 4 genes * 3 samples = 384 completions
```

## Gene Inference And Embeddings

Run one gene index at a time.  The script name contains `first`, but `--gene-index` selects any sampled gene.  Each completion uses the exact endpoint variant request policy, records OpenRouter metadata, and embeds the response with `text-embedding-3-small` by default.

```bash
uv run --script tools/run_first_gene_inference_embeddings.py \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --genes sampled-genes.json \
  --persona ../common/etc/personas/generic.md \
  --samples 3 \
  --gene-index 0 \
  --out results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ
```

Repeat for each gene index in `sampled-genes.json`.  Use distinct output directories for each gene.  The default request parameters are `temperature: 0.7`, `top_p: 1.0`, and `max_tokens: 512`.

The current script expects every survivor row to have a non-empty `endpoint_tag`; it builds `provider.only` from that field.  Each direct gene run records an immutable manifest, exact variant, gene, and persona copies, one atomic row record per completion, and an aggregate `records.jsonl`.  Explicit `--resume` validates those artifacts before any write or provider request.  It reuses successful rows, retries only embedding after an `embedding_error`, and retries completion after a `completion_error` or missing row.

Verify each gene inference run before PCA.

```bash
jq '{records_written, embedding_count, completion_error_count, embedding_error_count, status_counts, gene_index, gene}' \
  results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ/summary.json
```

For a 32-variant, one-persona, three-sample gene run, the expected output is 96 records and 96 embeddings.  If a new run has errors, record the counts and decide whether to rerun that gene or carry the error rows forward.  PCA includes only `status: "ok"` records.

## PCA

Run PCA separately for each gene.  PCA coordinates from different genes are not in one shared coordinate system, so clustering must also run per gene.  The current workflow reduces embeddings to three dimensions.

```bash
uv run --script tools/run_embedding_pca.py \
  --records results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ/records.jsonl \
  --out results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ \
  --dimensions 3
```

Expected files:

| File | Purpose |
| --- | --- |
| `pca-records.jsonl` | One projected row per included embedding. |
| `pca-fit.json` | Mean, components, variance, ratios, and singular values. |
| `summary.json` | Source counts and PCA dimensions. |

Verify row counts, dimensions, and endpoint coverage.

```bash
jq '{included_records, source_records, embedding_dimension, pca_dimensions, explained_variance_ratio_sum}' \
  results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ/summary.json
```

## Per-Gene Clustering

Cluster the completed per-gene PCA rows.  The script validates the expected row count, one gene per PCA file, 32 endpoint variants by default, three samples per endpoint variant, and three-dimensional PCA vectors by default.  It runs K-means for `k = 3..10`, scores valid candidates with silhouette score, and writes one cluster row per sampled completion.

```bash
uv run --script tools/run_gene_pca_clustering.py \
  --pca-records results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-2-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-3-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-4-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ
```

Use `--expected-rows-per-gene`, `--expected-variants-per-gene`, `--expected-samples-per-variant`, `--pca-dimensions`, `--min-k`, and `--max-k` to make the clustering validation match the run shape.

The output cluster labels are local to `gene_index`.  A label `2` for gene 0 has no direct relation to label `2` for gene 1.

Expected files:

| File | Purpose |
| --- | --- |
| `clusters.jsonl` | One sample-level cluster row per completion. |
| `clusters.csv` | Compact inspection table. |
| `summary.json` | Chosen `k`, silhouette scores, counts, and validation data. |
| `cluster-fit.json` | Cluster centers and candidate scores by gene. |

Render `clusters.csv` when a visual check helps.  The chart puts one row per serving provider and one column per gene, colors each point by its cluster, and gives each model within a provider row its own marker.  It plots `pc1` against `pc2`, so it shows two of the three reduced dimensions.

```bash
env MPLBACKEND=Agg uv run --script tools/clusters-graph.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.csv \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.png
```

## Cluster Vector Aggregation

Aggregate sample-level clusters into one variant/persona cluster vector per endpoint variant and persona.  For each variant, persona, and gene, the aggregator chooses a unanimous cluster when all three samples agree, a majority cluster when two samples agree, and the cluster for the sample nearest to its assigned K-means center when all three differ.  The output `clusters` array is ordered by ascending `gene_index`.

```bash
uv run --script tools/aggregate_variant_persona_clusters.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.jsonl \
  --cluster-fit results/gene-clusters-YYYYMMDDTHHMMSSZ/cluster-fit.json \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --out results/variant-persona-clusters-YYYYMMDDTHHMMSSZ \
  --expected-samples-per-gene 3
```

Expected files:

| File | Purpose |
| --- | --- |
| `variant-persona-clusters.jsonl` | Canonical pool-sampling input. |
| `variant-persona-clusters.json` | JSON array version of the same rows. |
| `summary.json` | Row counts, gene order, and aggregation-method counts. |

Verify the aggregate before sampling a pool.

```bash
uv run python - <<'PY'
import json
from pathlib import Path

path = Path("results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl")
rows = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]

print({
    "rows": len(rows),
    "cluster_lengths": sorted({len(row["clusters"]) for row in rows}),
    "all_clusters_int": all(all(isinstance(value, int) for value in row["clusters"]) for row in rows),
    "all_have_variant": all(isinstance(row.get("variant"), dict) for row in rows),
    "all_have_persona": all(isinstance(row.get("persona"), dict) for row in rows),
})
PY
```

## Tuple-Uniform Pool Sampling

Use `tools/sample-tuple-pool.py` to generate the final pool.  The sampler treats each distinct `clusters` vector as a sampling unit.  For each emitted row, it chooses one unique cluster tuple uniformly at random, then chooses one row uniformly from the rows with that tuple.

```bash
mkdir -p results/sample-tuple-pool-YYYYMMDDTHHMMSSZ
uv run --script tools/sample-tuple-pool.py \
  results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl \
  --out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/pool.jsonl \
  --diagnostics-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/diagnostics.jsonl \
  --equivalence-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/equivalence.jsonl \
  --pool-size 20 \
  --seed 0 \
  | tee results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/sample.log
```

The sampler deduplicates equivalent provider endpoints before tuple sampling.  The equivalence key uses model identity, quantization, and modalities; it excludes provider name, endpoint tag, context limits, prompt and completion limits, supported parameters, price, latency, and uptime.  The representative selection rule ranks endpoints by operational results, capacity, and serving metadata, then writes the full provider set to `equivalence.jsonl`.

The sampler copies selected persona files into `personas/` beside `pool.jsonl` and emits confined `personas/...` paths relative to the pool file.  Relative source paths resolve from `model-pool/`, the cluster-input directory, and shared `common/etc/` unless `--persona-root` supplies a base directory.  A source path supplied with `--persona-root` must resolve inside that root.  Before writing output, the sampler validates every selected persona, rejects one persona ID associated with different bytes, rejects symbolic links in the output persona path, and accepts an existing destination only when its bytes match.

Sampling is with replacement by default after deduplication.  Duplicate rows can appear in the final pool, especially when `pool-size` exceeds the number of well-populated tuples or when the random draw revisits a tuple.  The diagnostics file records the selected tuple, source row, cumulative tuple count, cumulative source-row count, model ID, provider, endpoint tag, quantization, endpoint identifier, and equivalence class for each emitted row.

Add `--without-replacement` when the pool must contain each row from the sampling frame at most once.  With the default equivalent-endpoint deduplication, the sampling frame is the representative set, so `--pool-size` cannot exceed the number of equivalence classes.  The command fails before writing output when `--pool-size` is larger than the available frame.

Verify the sampled pool.

```bash
uv run python - <<'PY'
import json
from pathlib import Path

source_path = Path("results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl")
pool_path = Path("results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/pool.jsonl")
diagnostics_path = Path("results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/diagnostics.jsonl")
equivalence_path = Path("results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/equivalence.jsonl")

source_lines = [line for line in source_path.read_text().splitlines() if line.strip()]
source_rows = [json.loads(line) for line in source_lines]
source_ids = {row["endpoint_variant_id"] for row in source_rows}
pool_lines = [line for line in pool_path.read_text().splitlines() if line.strip()]
pool_rows = [json.loads(line) for line in pool_lines]
diagnostics = [json.loads(line) for line in diagnostics_path.read_text().splitlines() if line.strip()]
equivalence = [json.loads(line) for line in equivalence_path.read_text().splitlines() if line.strip()]

source_tuples = {tuple(row["clusters"]) for row in source_rows}
pool_tuples = {tuple(row["clusters"]) for row in pool_rows}

print({
    "output_rows": len(pool_rows),
    "diagnostic_rows": len(diagnostics),
    "equivalence_rows": len(equivalence),
    "unique_output_rows": len(set(pool_lines)),
    "unique_output_tuples": len(pool_tuples),
    "all_representatives_from_input": all(row["endpoint_variant_id"] in source_ids for row in pool_rows),
    "all_tuples_from_input": pool_tuples <= source_tuples,
    "all_clusters_int": all(all(isinstance(value, int) for value in row["clusters"]) for row in pool_rows),
    "all_equivalence_records_have_representative": all("representative_endpoint_variant_id" in row for row in equivalence),
})
PY
```

## Run Record

The batch and end-to-end runners save normalized result parameters and exact input copies before model work.  A new run refuses a nonempty output directory.  Resume occurs only through explicit `--resume` with the original sources and result parameters; neither runner infers continuation from existing files.  Resume validates every reusable artifact before publishing an owner record, appending a command record, deleting a recognized interrupted temporary, or changing a stage output.  Preserve `manifest.json`, `inputs/`, `commands.jsonl`, stage outputs, stage records, progress files, and `summary.json` as one run record.

For root sampling, record the catalog snapshot and exclusion set.  For evals, record operational metrics separately from deliberation score.  For pool generation, record input row count, unique tuple count, output row count, output unique tuple count, output unique row count, seed, and pool path.
