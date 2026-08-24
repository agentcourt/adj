# OpenRouter Model Inventory

`tools/model_inventory.py` records the OpenRouter catalog and the provider endpoints advertised for selected model IDs.  It writes one normalized endpoint-variant row for each endpoint returned by OpenRouter.  The script performs no inference requests.

## Catalog Inventory

Run the script from `model-pool/`.  Supply one or more `--model-id` values to inventory named models, or use `--sample-models` and `--sample-seed` to choose a deterministic sample from the catalog.  The script requires `OPENROUTER_API_KEY` in the environment or `secrets/openrouter.api.txt`.

```bash
uv run --script tools/model_inventory.py \
  --run-id model-roots-10-YYYYMMDDTHHMMSSZ \
  --model-id deepseek/deepseek-v4-flash
```

The script fetches `/api/v1/models`, followed by `/api/v1/models/{author}/{slug}/endpoints` for each selected model.  A failed catalog or endpoint request aborts the inventory after the configured retries.  `--request-timeout`, `--retries`, and `--sleep` control request timing without changing which endpoint rows the script records.

| File | Contents |
| --- | --- |
| `raw/models.json` | Raw `/api/v1/models` response. |
| `raw/endpoints/*.json` | One raw endpoint response per selected model ID.  Percent-encoded model IDs provide readable, collision-free filenames. |
| `endpoint_variants.jsonl` | Canonical normalized endpoint rows. |
| `endpoint_variants.csv` | Inspection table for the normalized rows. |
| `summary.json` | Snapshot times, selected model IDs, row counts, endpoint fetches, and provider, quantization, and status counts. |

Each row records the catalog snapshot, OpenRouter model identity, model architecture and limits, provider and endpoint identity, quantization, endpoint limits, supported parameters, pricing, status, and paths to the raw catalog responses.  `endpoint_variant_id` is a readable identifier built from the model ID, endpoint tag or provider, and quantization.  Duplicate endpoint identifiers abort the inventory because downstream eval, clustering, and sampling stages use that field as the endpoint key.

An endpoint with `quantization: "unknown"` remains a distinct endpoint variant.  Provider, endpoint tag, endpoint name, limits, supported parameters, pricing, status, and raw metadata continue to distinguish that row.  The inventory does not infer an unreported quantization or combine endpoints that share the `unknown` value.

## Exact Endpoint Requests

`tools/run_variant_batch.py` passes each endpoint row to `tools/run_eval.py`, which converts it into an exact OpenRouter request specification.  The request uses `endpoint_tag` in `provider.only` when present and otherwise uses `provider_name`.  It sets `allow_fallbacks: false` and `require_parameters: true`, and it sends `provider.quantizations` when the catalog supplies a known quantization.

```json
{
  "model": "<openrouter_model_id>",
  "provider": {
    "only": ["<endpoint_tag-or-provider>"],
    "allow_fallbacks": false,
    "require_parameters": true,
    "quantizations": ["<known-quantization>"]
  }
}
```

For an endpoint whose catalog quantization is `unknown`, the request omits `provider.quantizations` and retains the provider constraint.  Eval result metadata records the requested route, quantization constraint, fallback policy, parameter policy, and request parameters.  It also retains the normalized endpoint fields from the inventory row under `variant_metadata`.

## Route Metadata

Exact-endpoint requests set `X-OpenRouter-Experimental-Metadata: enabled`.  Eval result rows retain the response ID, returned model, inline OpenRouter metadata, usage, finish reasons, latency, provider errors, requested provider constraints, and request parameters.  The runner also queries `/api/v1/generation?id=<generation_id>` after completion and retries briefly because OpenRouter can delay that record.

The generation record can supply provider-response endpoint IDs, model permaslugs, upstream IDs, token counts, cost, latency, and native finish reasons.  Compare the selected route in that metadata with the requested provider constraint and the inventory snapshot.  Missing generation metadata remains an explicit error field in the result row.

The inventory and route metadata describe the OpenRouter product served during the recorded requests.  They do not identify unreported model weights, serving software, hardware, cache precision, provider prompt changes, or later provider changes.  Research that depends on those properties requires a controlled deployment or a provider attestation.
