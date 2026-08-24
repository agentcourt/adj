# Development Notes

## 2026-08-23 Restoration to adj

Restored `model-pool/` from adjudication commit `dde9b3fe3b83e0139534da340cb17d5e4522f256`.  The restored tools remain at the repository root and continue to read personas from `common/etc/personas/`.  Current commands use `adc case`, `aar case`, and `aard case`, and OpenRouter request metadata names `agentcourt/adj` as the repository.

Added `--prompt` to `tools/run_eval.py`, `tools/run_variant_batch.py`, and `tools/run_end_to_end.py`.  Relative paths resolve from `model-pool/`, and each runner passes or records the selected prompt path.  The default remains `prompts/juror-single.md`.

Verification: Python compilation, both item validators, the repository audit, all fifteen command help paths, and the end-to-end dry run passed.  A twenty-item mock run using `prompts/council-member.md` scored `1.0`, and a second run confirmed that changing `--prompt` changes the rendered prompt and recorded prompt path.  `uv` retrieved the scripts' declared NumPy, Matplotlib, SciPy, and scikit-learn dependencies during these tests; no dependency declaration changed.

## 2026-08-23 Run Integrity and Persona Packaging

Added `tools/run_record.py` for immutable run manifests and exact input copies.  Variant batches now require explicit `--resume`, reject nonempty output directories for new runs, copy endpoint, question, evidence, and prompt sources, compare current sources byte-for-byte before resume writes, and validate progress rows against the recorded variant order and identity.  The end-to-end runner applies the same input and parameter checks, uses its saved question, prompt, gene, and persona files for child stages, validates stage contents before skipping them, and passes `--resume` only to an existing incomplete variant batch.

Tuple-pool sampling now copies every selected persona into `personas/` beside the output pool and emits a relative path to that copy.  Preflight reads all selected persona sources before writing outputs, rejects one persona ID associated with different contents, assigns deterministic filenames after normalization collisions, and permits an existing destination only when its bytes match.  `--persona-root` supplies an explicit base for relative source paths.

Focused verification compiled the five changed tools and passed thirteen standard-library tests.  The tests cover question and evidence snapshots, source and parameter mismatch rejection, preservation of run files after a rejected resume, strict progress identity, index, JSON-key, and variant-spec validation, live-child detection, an executed batch with explicit resume, end-to-end dry-run resume checks, persona packaging, and persona-ID conflicts.  A separate dry run copied all four Core20 evidence records into a run outside the repository, and a twenty-item mock eval read the copied questions and evidence before scoring `1.0`.  Sampling two rows from the active 25-row pool copied the shared generic persona beside the output and emitted `personas/generic.md` for both rows.

## 2026-07-16 Eval Directory Reorganization

Moved the model-pool eval system under `model-pool/` as part of the repository-wide eval layout.  The Python tools still run from that directory and write generated output under `results/`.  ADC judge behavior eval assets moved into `evals/adc/judge/`, while ADC judge run output moved under ignored `evals/out/adc/judge/`.

## 2026-07-16 README Consolidation

Consolidated the duplicate eval README content into `README.md` and removed `README-better.md`.  The README now describes stable scope, credentials, documentation, validation commands, deterministic local testing, layout, and run-data boundaries.  Run-specific counts, endpoint identities, pass rates, dated filter details, and run IDs remain in the manual, runbooks, variant summaries, analysis notes, or generated run artifacts instead of the README.

## 2026-06-19 Without-Replacement Pool Sampling

Added `--without-replacement` to `tools/sample-tuple-pool.py`.  The flag keeps tuple-uniform sampling but removes a selected row from the available frame, so a pool cannot repeat an endpoint representative.  With equivalent-endpoint deduplication enabled, the maximum pool size is the number of equivalence classes.

Generated a new pool from `results/e2e-root40-pool30-20260618T172234Z/variant-persona-clusters/variant-persona-clusters.jsonl` without running model calls.  The output directory is `results/e2e-root40-pool30-20260618T172234Z/pool-without-replacement/`.  The result has 25 pool rows, 25 diagnostics rows, 25 equivalence rows, 25 unique endpoint-variant IDs, and 21 unique cluster tuples.  A 26-row run fails before writing output because the deduplicated frame has 25 rows.

Promoted the without-replacement pool to the active default pool paths, replacing the previous 19-row pools.  `common/data/personas/pool.jsonl` and `arb/pool.jsonl` both contain the 25-row without-replacement pool.  `arbd/` has no local `pool.jsonl`, so its default remains the shared common pool.

## 2026-06-19 Persona Clustering Tool Move

Moved the legacy CSV persona-clustering pipeline into `evals/`.  The moved files are `tools/filter-models.py`, `tools/model-speed.sh`, `tools/cluster-personas.py`, `tools/clusters-graph.py`, `tools/select-council.py`, `tools/generate-council.py`, and `docs/jury-pool-generation.md`.  The generated corpus and pool data remain under `common/data/personas/` because the existing runtimes read shared model-pool data from that location.

The moved runbook now includes the `clusters-graph.py` command used to render a faceted PCA and cluster chart.  The command reads PCA rows such as `common/data/personas/pca-cluster.csv` or `common/data/personas/personas-pca.csv` and writes a PNG to the requested output path.

## 2026-06-19 Root-40 Pool-30 Run Failure

Run `e2e-root40-pool30-20260618T172234Z` completed inventory, eval, filtering, and four gene inference stages, then failed before PCA.  Inventory sampled 40 root models and produced 103 endpoint variants.  Eval completed all variants; the filter kept 33 survivors.  Gene inference wrote 99 records per gene, but completion errors remained: 16 for gene 0, 17 for gene 1, 14 for gene 2, and 11 for gene 3.

The deterministic failure class is unsupported request parameters during exact provider routing.  `tools/run_first_gene_inference_embeddings.py` sends `temperature`, `top_p`, and `max_tokens` for every survivor while also setting `provider.require_parameters` to `true`.  DigitalOcean `nvidia/nemotron-3-super-120b-a12b`, BaseTen `openai/gpt-oss-120b`, and Poolside `poolside/laguna-xs.2` do not advertise `top_p`, and OpenRouter rejects those exact-provider requests with `404 No endpoints found that can handle the requested parameters`.

The transient failure classes are OpenRouter rate limits and local read failures such as `IncompleteRead(...)`.  The gene runner currently records those failures without retrying.  The proposed fix is to send only parameters supported by the endpoint metadata and to retry rate-limit and read failures with a bounded policy.  Recovery also needs `--resume` to rerun failed gene stages instead of treating any existing `summary.json` as completed.

Follow-up: implemented endpoint-supported request-parameter filtering, bounded completion retries, and record-level gene resume.  The saved run resumed from the existing inventory, eval, and filter outputs.  Gene 0 reused 83 records and recovered 16 records; gene 1 reused 82 and recovered 17; gene 2 reused 85 and recovered 14; gene 3 reused 88 and recovered 11.  All four gene stages finished with 99 records, 99 embeddings, and zero completion or embedding errors.  PCA, clustering, aggregation, and pool sampling completed.  The final pool has 30 rows, 30 diagnostics rows, and 25 equivalence rows.

## 2026-06-18 Equivalent Endpoint Deduplication

Added equivalent-endpoint deduplication to tuple-uniform pool sampling.  Endpoint variants still remain separate through inventory, eval, filtering, gene responses, PCA, clustering, and aggregation, so provider-route behavior remains auditable.  The final pool sampler now groups by model identity, quantization, and modalities, selects one concrete provider endpoint by deterministic operational and capacity ranking, writes that representative to `pool.jsonl`, and writes the full provider set to `equivalence.jsonl`.

## 2026-06-17 Manual Split

Moved the detailed evals README content into `manual.md` and kept `README.md` as a short index.  The manual contains purpose, operator guidance, command reference material, endpoint-variant procedures, scoring model, scope, and troubleshooting.  The README links to the manual, sampling runbook, and model-inventory notes, and keeps validation commands, a deterministic local test, credential notes, and the main file map.

## 2026-05-31 End-To-End Sampling Test

Goal: run a small end-to-end sampling test over five OpenRouter root models.  The test shape uses one eval trial, two genes, one sample per gene, the generic persona, PCA with dimensions capped by available rows, per-gene clustering, variant/persona aggregation, and tuple-uniform pool sampling.

Implementation note: generalized `tools/run_gene_pca_clustering.py` so the expected variant count and expected samples per variant are flags rather than historical constants.  Generalized `tools/aggregate_variant_persona_clusters.py` so one-sample test runs can aggregate without the historical three-sample requirement.  Synthetic validation passed with two variants, two genes, one sample per gene, and two PCA dimensions.

Run result: sampled five root models with seed `0`: `anthropic/claude-opus-4.7`, `mistralai/mistral-large-2407`, `openai/gpt-5.1`, `openai/gpt-audio`, and `qwen/qwen3-32b`.  Inventory found 18 endpoint variants and zero endpoint-fetch errors.  The one-trial core eval completed all 18 variant runs with no runner failures; 13 variants had provider errors, and five survived the historical filter `provider_error_count == 0` and `deliberation_score >= 0.90`.

Survivors: Mistral Large 2407 on Mistral, Qwen3 32B on DeepInfra, Qwen3 32B on Nebius, Qwen3 32B on AtlasCloud, and Qwen3 32B on Groq.  The two gene runs used `What do you think about religion?` and `What causes climate change?`; each wrote five records and five embeddings with no completion or embedding errors.  PCA used three dimensions for each gene.  Per-gene clustering selected `k=2` for both genes, aggregation wrote five variant/persona rows, and tuple-uniform sampling emitted five pool rows from two unique cluster tuples.

## 2026-05-31 End-To-End Runner

Added `tools/run_end_to_end.py`, a uv-runnable command for the endpoint-variant pool pipeline.  It calls the existing inventory, eval, filter, gene inference, PCA, clustering, aggregation, and tuple-pool tools, and writes a single run directory with `manifest.json`, `commands.jsonl`, stage subdirectories, and `summary.json`.  The runner supports explicit `--model-id` values, sampled roots, resume, dry-run, stage stops, configurable filter criteria, configurable genes and samples, PCA dimension capping, and pool sampling parameters.

Validation: `uv run --script tools/run_end_to_end.py --help` passed.  Dry-run over `/tmp/adjudication-evals-orchestrator-dry/dry` wrote the manifest and recorded the inventory command without calling OpenRouter.  Resume validation over `/tmp/adjudication-evals-orchestrator-resume-test/run` reused today’s completed inventory, eval, and gene inference artifacts, then ran filter, PCA, clustering, aggregation, and pool sampling to completion without live API calls.

Path-handling fix: generalized `tools/run_embedding_pca.py`, `tools/run_first_gene_inference_embeddings.py`, and `tools/run_variant_batch.py` so output and input paths outside the repository root can be reported without `Path.relative_to(ROOT)` failures.  `tools/run_gene_pca_clustering.py` and `tools/aggregate_variant_persona_clusters.py` already received the same display-path treatment during the small-run generalization.

Variant-timeout fix: `tools/run_variant_batch.py` now redirects each child eval run to `variant-runs/*/run_eval.log`, monitors raw-result and log progress without blocking on child stdout, and terminates a variant that exceeds `--no-progress-timeout` or `--variant-timeout`.  Timed-out variants are recorded in `progress.jsonl` and `variant_summary.csv`, and the batch exits with status 0 when timeout is the only variant failure.  `tools/run_end_to_end.py` passes the no-progress timeout through the eval stage and writes `filtered/removed_variants.jsonl` so timed-out variants are removed before gene inference.
