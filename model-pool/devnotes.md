# Development Notes

## Shared runtime model requests

The direct tool-use screen calls the shared model executor.  The Pi screen uses the formal runtimes' local model gateway and the same executor.  The host applies the request specification and retains endpoint credentials.  Pi receives a local model alias and token.  Provider request records, usage, and observed cost accompany the Pi transcript and MCP calls.

Live checks passed for OpenRouter DeepSeek V4 Flash at Alibaba FP8, Anthropic Claude Sonnet 5, and Google Gemini 3.5 Flash.  The DeepSeek and Claude checks included a rejected tool argument followed by an accepted correction.  The Python inventory and partition coordinator continue to accept OpenRouter inventory rows, while the Go command accepts an individual configuration for any registered endpoint.

## 2026-08-30 Default Pool Installation

Copied `results/pool-20260828-score-gt-070-r2/pool/pool.jsonl` to `common/data/personas/pool.jsonl`.  The shared runtime default now contains 100 distinct model IDs, 30 providers, 100 endpoint variants, and the `personas/generic.md` persona reference.  Quick, ARB, ARBD, and ADC use this file when the caller supplies neither an explicit pool nor a working-directory `pool.jsonl`.

Updated the Quick example settings to use the shared pool with five council members and three required votes.  Live runs of `ex06`, `ex08`, and `ex11` recorded the shared pool path, and all 15 selected endpoint variants matched installed pool rows.  `ex06` and `ex11` received five votes; `ex08` received three votes after StreamLake returned HTTP 429 and AionLabs returned HTTP 400.  The three runs recorded $0.1671622148 in OpenRouter costs.

## 2026-08-29 Model-Pool Generation

Run `pool-20260828-score-gt-070-r2` reused the completed endpoint screen and evaluation results, retained 283 configurations with no provider errors and a deliberation score greater than `0.70`, and evaluated each configuration five times on each of 14 genes.  The runner used two gene processes.  It wrote all 19,810 expected records: 19,494 successful completions, 316 completion errors, 19,494 embeddings, and no embedding errors.  Three successful completions lacked a cost observation.  The observed completion cost was `$25.780984833363004`.

The cross-gene filter excluded all 18 configurations that had at least one error, leaving 265 configurations and 1,325 records per gene.  Five excluded configurations had only rate-limit errors, twelve had only provider errors, and one had both.  The eligible set contained 104 distinct model IDs, so the requested one-per-model pool of 100 remained possible.

PCA used eight dimensions for every gene.  Clustering selected `k=2` for twelve genes, `k=19` for gene 5, and `k=10` for gene 10.  Aggregation produced 265 configuration/persona rows.  Tuple-uniform sampling with seed `0`, endpoint-equivalence deduplication, no replacement, and one row per model produced 100 rows containing 100 distinct model IDs, 30 providers, 100 endpoint variants, and 69 distinct cluster tuples.

## 2026-08-28 Gene-Error Exclusion

The end-to-end runner now lets each gene command finish every configured sample and record per-request completion or embedding errors.  After every selected gene finishes, it excludes a configuration from the common downstream set when any gene has an error, missing sample, or missing embedding for that configuration.  It writes the eligible variants, excluded variants with failure details, and eligible records for each gene under `gene-filter/`.  PCA, clustering, aggregation, and sampling use those files, and the runner checks one-per-model pool capacity again after the exclusion.

The standalone gene command retains its nonzero exit status for record errors unless the caller passes `--allow-record-errors`.  The end-to-end runner passes that option so record-level endpoint failures do not terminate another gene process.  Missing output records and command failures still stop the run.

## 2026-08-25 Runtime Tool-Use Screening

Added a screen between endpoint inventory and question-set evaluation.  Each endpoint configuration must complete a Quick-style direct `submit_council_vote` call and an ARB-style Pi/MCP sequence containing `wait_for_opportunity` and `submit_council_vote`.  The screen uses the same model, provider route, fallback policy, parameter limits, Pi image, and MCP proxy extension that the runtimes use.  MCP acceptance completes the Pi check, and the screen stops a test container that remains active after a five-second exit period.

Pi's OpenRouter Chat Completions client sends `store` and chooses a maximum-token field from its model definition.  The screen and the ARB, ARBD, and ADC runtimes derive those settings from the endpoint's advertised parameters, which prevents exact routing from excluding an otherwise valid endpoint.  They also convert OpenRouter's per-token endpoint prices to Pi's per-million-token cost fields, allowing Pi to report request costs from its observed usage.

The Python coordinator processes one partition sequentially and retains one result directory per configuration.  The end-to-end runner partitions configurations by model and starts separate coordinator processes, with eight processes set by `make pool`.  Candidate failures remain in `rejected_variants.jsonl`, while credential, container, and transcript failures stop the affected partition.

OpenRouter returned HTTP 401 for one exact AtlasCloud route with `provider_name` set to AtlasCloud and `is_byok` set to false.  The same OpenRouter key continued to serve other routes.  OpenRouter's [provider documentation](https://openrouter.ai/docs/guides/community/for-providers) identifies provider 401 responses as endpoint-uptime failures.  The screen rejects a route when OpenRouter attributes an authentication error to a shared provider, while an OpenRouter or BYOK authentication error stops the partition.

OpenRouter also returned HTTP 403 for a model restricted to approved agent applications.  OpenRouter documents 403 for [guardrail and access restrictions](https://openrouter.ai/docs/guides/features/guardrails/overview).  The screen therefore rejects an OpenRouter configuration that returns 403.  Missing credentials and unattributed 401 responses continue to stop the partition.

A Pi container exited successfully during the grace period after its accepted vote, after the screen had decided to issue `stop` but before the container runtime processed that command.  Because the container uses `--rm`, the runtime returned exit status 125 and “no such container.”  The screen now accepts the successful process exit in that race.  A failed process exit or a failed stop against a running process remains an error.

## 2026-08-23 Restoration to adj

Restored `model-pool/` from adjudication commit `dde9b3fe3b83e0139534da340cb17d5e4522f256`.  The restored tools remain at the repository root and continue to read personas from `common/etc/personas/`.  Current commands use `adc case`, `aar case`, and `aard case`, and OpenRouter request metadata names `agentcourt/adj` as the repository.

Added `--prompt` to `tools/run_eval.py`, `tools/run_variant_batch.py`, and `tools/run_end_to_end.py`.  Relative paths resolve from `model-pool/`, and each runner passes or records the selected prompt path.  The default remains `prompts/juror-single.md`.

Verification: Python compilation, both item validators, the repository audit, and all fifteen command help paths passed.  A twenty-item mock run using `prompts/council-member.md` scored `1.0`, and a second run confirmed that changing `--prompt` changes the rendered prompt and recorded prompt path.  `uv` retrieved the scripts' declared NumPy, Matplotlib, SciPy, and scikit-learn dependencies during these tests; no dependency declaration changed.

## 2026-08-23 Evaluation Runner Simplification

Deleted the separate run-record module, its recovery and integrity tests, and documentation for that behavior.  Restored the affected commands to the source model-pool behavior while retaining the adj OpenRouter metadata and the configurable prompt path through the direct, batch, and end-to-end eval commands.  Collapsed the unused evidence-tool error subclasses into `ToolError`.  Verification compiled the affected commands, checked the prompt-bearing help paths, completed a one-item mock run with `prompts/council-member.md`, and exercised evidence listing and reading.

## 2026-08-23 Persona Packaging Removal

Restored `tools/sample-tuple-pool.py` to the source model-pool behavior, which preserves the persona paths in its input rows.  Deleted the five persona-packaging tests and the documentation for copied persona files.  Verification compiled the script, checked its help output, and sampled two rows while preserving both input persona paths.

## 2026-08-23 End-To-End Execution Simplification

Removed the end-to-end options that skipped execution or reused existing stage output.  Each run requires a new directory, and stage stops remain available for bounded runs.  The run root contains stage directories and `summary.json`; command events go to standard output.  Python compilation and command help passed.  A live one-model inventory stage completed with three endpoint variants.

## 2026-08-24 Variant Batch Continuation Removal

Removed implicit continuation from `tools/run_variant_batch.py`.  Each invocation requires an absent or empty output directory and evaluates every input variant.  `variant_summary.csv` contains the terminal result for each variant, while standard-output events report live progress.  Removed the control files while retaining per-request, no-progress, and per-variant timeouts.  Removed the stale batch-output entries for snapshot files that the command does not create.

Verification compiled the command and checked its help output.  A zero-variant invocation completed in an empty directory, and a second invocation against that populated directory failed before execution.  A live one-variant, one-item batch completed and scored `1.0`.  Focused child-process runs exercised both timeout kinds, returned exit code 124, and terminated the children.

## 2026-08-24 Gene Runner Simplification

Removed gene-run continuation, record reuse, and temporary-file replacement.  Each invocation requires an absent or empty output directory, requests every configured completion and embedding, and writes `records.jsonl` directly.  Request timeouts and bounded retries remain.

A live one-variant, one-sample run completed one OpenRouter response and one OpenAI embedding.  It wrote one successful record directly, created no temporary record file, and rejected a second invocation against the populated output directory.  Python compilation and command help passed.

An implementation and user-documentation search found none of the removed continuation, temporary-output, PID-file, stop-file, or end-to-end manifest code.  `git diff --check` passed.  The development journal retains the removed feature names as the record of this cleanup.

## 2026-08-24 Output and Identity Reduction

The evaluator now keeps response rows in `raw_results.jsonl`, and the scorer reads that file and writes `scores.json`.  Batch progress uses standard-output events and `variant_summary.csv`.  The end-to-end runner keeps stage outputs and `summary.json`.  Filtering keeps accepted endpoint rows, removed endpoint rows, its CSV view, and its summary.  Gene records omit duplicate run and gene-hash fields, and end-to-end validation rejects incomplete completions or embeddings before PCA.  Aggregation writes one JSONL pool input and its summary.

Inventory aborts on an endpoint-fetch failure.  Endpoint raw filenames percent-encode the model ID, and request-selectable route IDs use `openrouter:<model>@<route>#<quantization>`.  When OpenRouter returns multiple catalog rows with the same route, inventory retains each row, adds a readable `~catalog-row-<index>` suffix to its variant ID, and marks every member of the route group as ambiguous.  The endpoint-evaluation stage rejects those rows before model calls because OpenRouter's documented [Chat API](https://openrouter.ai/docs/api/api-reference/chat/send-chat-completion-request?explorer=true) and [provider routing](https://openrouter.ai/docs/guides/routing/provider-selection) can select the model, provider tag, and quantization but cannot select one row within the group.  Different provider tags remain independently routable.  The checked-in filtered variants and default pool use the readable IDs consistently across root, representative, variant, and equivalent-endpoint fields.  Removed response hashes from the normalized inventory and runtime metadata.

Deleted the unused pool samplers, duplicate filter artifacts and checked-in specs, duplicate aggregate JSON, duplicate scorer and evaluator outputs, inventory Markdown summary, gene manifest, and obsolete result schema.  Local verification compiled every model-pool tool, validated both item sets, passed the repository audit, completed a four-row mock evaluator run and scoring pass, and confirmed that the evaluator directory contains only `raw_results.jsonl` and `scores.json`.

A current OpenRouter inventory for `openai/gpt-4o-mini` returned three routes and wrote `openai%2Fgpt-4o-mini.json`; its output contained the normalized JSONL and CSV, raw catalog responses, and `summary.json`.  A complete live run for one Relace Search route evaluated one question, retained one survivor, completed one gene response and embedding, and finished PCA, clustering, aggregation, and one-row pool sampling.  Its file tree matched the retained output set.  The focused Quick and model-request Go tests passed against the migrated default pool.

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

OpenRouter rate limits and local read failures such as `IncompleteRead(...)` interrupted the first gene run.  The gene runner filters request parameters against endpoint metadata and retries transient completion failures with a bounded policy.  Each invocation starts a new output directory and requests every record.

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

Added `tools/run_end_to_end.py`, a uv-runnable command for the endpoint-variant pool pipeline.  It calls the existing inventory, eval, filter, gene inference, PCA, clustering, aggregation, and tuple-pool tools, and writes stage subdirectories and `summary.json` beneath one run directory.  The runner supports explicit `--model-id` values, sampled roots, stage stops, configurable filter criteria, configurable genes and samples, PCA dimension capping, and pool sampling parameters.

Validation: `uv run --script tools/run_end_to_end.py --help` passed.

Path-handling fix: generalized `tools/run_embedding_pca.py`, `tools/run_first_gene_inference_embeddings.py`, and `tools/run_variant_batch.py` so output and input paths outside the repository root can be reported without `Path.relative_to(ROOT)` failures.  `tools/run_gene_pca_clustering.py` and `tools/aggregate_variant_persona_clusters.py` already received the same display-path treatment during the small-run generalization.

Variant-timeout fix: `tools/run_variant_batch.py` redirects each child eval run to `variant-runs/*/run_eval.log`, monitors raw-result and log progress without blocking on child stdout, and terminates a variant that exceeds `--no-progress-timeout` or `--variant-timeout`.  Timed-out variants appear in `variant_summary.csv`, and the batch exits with status 0 when timeout is the only variant failure.  `tools/run_end_to_end.py` passes the no-progress timeout through the eval stage and writes `filtered/removed_variants.jsonl` so timed-out variants are removed before gene inference.
