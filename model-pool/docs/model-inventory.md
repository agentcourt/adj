# OpenRouter model inventory procedure

## Goal

Build a high-fidelity inventory of OpenRouter model variants for evaluation work. The inventory must distinguish provider endpoints for the same OpenRouter model ID, including differences in quantization, context limits, parameter support, pricing, latency, fallback behavior, moderation behavior where observable, and provider-specific runtime behavior.

The evaluated unit is an OpenRouter routed endpoint variant, not an OpenRouter model ID alone:

```text
catalog_snapshot_id
+ openrouter_model_id
+ provider_name
+ endpoint tag or endpoint identifier, if exposed
+ quantization
+ context and completion limits
+ supported-parameter set
+ routing constraints
+ request parameters
+ eval suite version
+ eval timestamp
```

OpenRouter can inventory and observe the routed product. It cannot prove exact weights, Hugging Face commit hash, serving engine, GPU type, tensor-parallel layout, CUDA/kernel version, KV-cache precision, hidden provider prompt template, moderation implementation, or unannounced provider-side changes. Use self-hosting or dedicated provider attestations when the target is a controlled model artifact rather than the product served through OpenRouter.

## Inventory layers

Use two layers.

1. Static endpoint inventory: every model/provider/endpoint variant that OpenRouter advertises through its catalog APIs.
2. Observed inference inventory: a small deterministic probe against selected endpoint variants, recording the provider and routing metadata OpenRouter returns at request time.

The static inventory is broad coverage. The observed inventory tests whether an advertised endpoint can serve now and records request-level fields that the catalog does not capture.

## Static endpoint inventory

First fetch the model catalog:

```bash
curl https://openrouter.ai/api/v1/models \
  -H "Authorization: Bearer $OPENROUTER_API_KEY"
```

Store the raw response. Then fetch endpoint variants for every model ID:

```bash
curl https://openrouter.ai/api/v1/models/{author}/{slug}/endpoints \
  -H "Authorization: Bearer $OPENROUTER_API_KEY"
```

For model ID `deepseek/deepseek-v4-flash`, the endpoint path is:

```text
/api/v1/models/deepseek/deepseek-v4-flash/endpoints
```

Normalize one row per endpoint variant. Preserve the raw JSON for every model and endpoint response.

Suggested normalized fields:

```text
catalog_snapshot_id
snapshot_timestamp
openrouter_model_id
canonical_slug
model_name
model_created
hugging_face_id
knowledge_cutoff
modality
input_modalities
output_modalities
tokenizer
instruct_type
model_context_length
model_supported_parameters
model_default_parameters
model_pricing
provider_name
endpoint_name
endpoint_tag
endpoint_id, if exposed
endpoint_model_id
endpoint_model_name
endpoint_model_permaslug, if exposed
quantization
context_length
max_prompt_tokens
max_completion_tokens
supported_parameters
supports_implicit_caching
pricing_prompt
pricing_completion
pricing_input_cache_read
pricing_discount
status
uptime_last_5m
uptime_last_30m
uptime_last_1d
latency_last_30m
throughput_last_30m
raw_model_json
raw_endpoint_json
```

The endpoint row is the primary inventory object. For example, one OpenRouter model ID can expose multiple provider variants:

```text
deepseek/deepseek-v4-flash / DeepInfra / deepinfra/fp4 / fp4
deepseek/deepseek-v4-flash / GMICloud / gmicloud/fp8 / fp8
deepseek/deepseek-v4-flash / Baidu / baidu/fp8 / fp8
deepseek/deepseek-v4-flash / Alibaba / unknown
deepseek/deepseek-v4-flash / DeepSeek / unknown
```

`quantization` comes from the endpoint catalog. It is a routing and catalog field, not a cryptographic attestation of the exact runtime artifact.

Treat `unknown` as an endpoint-specific unknown, not as a shared quantization class. Two endpoints with `quantization: "unknown"` may have different weight precision, kernels, cache behavior, safety wrappers, or serving engines. Do not collapse unknown-quantization endpoints into one variant. Preserve the provider, endpoint tag, endpoint name, context limits, supported parameters, pricing, status fields, and raw endpoint JSON as the differentiating configuration.

## Observed endpoint probe

For endpoint variants that matter, run a tiny deterministic inference request. Enable router metadata and prevent silent fallback.

Example:

```bash
curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-OpenRouter-Experimental-Metadata: enabled" \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "messages": [
      {
        "role": "user",
        "content": "For this metadata probe, reply with exactly: OK"
      }
    ],
    "temperature": 0,
    "top_p": 1,
    "max_tokens": 32,
    "seed": 1,
    "provider": {
      "only": ["deepinfra/fp4"],
      "allow_fallbacks": false,
      "require_parameters": true,
      "quantizations": ["fp4"]
    }
  }'
```

Use the most specific endpoint tag available in `provider.only` when OpenRouter accepts it. Use a broad provider name only when no endpoint tag is exposed or accepted. Set `allow_fallbacks: false` so a failing endpoint does not silently become another provider. Set `require_parameters: true` so endpoints that cannot honor the requested parameters are excluded instead of receiving a degraded request.

Record the request body, request headers except secrets, request timestamp, response timestamp, raw response JSON, and parsed metadata fields.

Important inline response fields:

```text
generation_id
returned_model
system_fingerprint, if returned
finish_reason
native_finish_reason, if returned
usage
openrouter_metadata.requested
openrouter_metadata.strategy
openrouter_metadata.region
openrouter_metadata.summary
openrouter_metadata.attempt
openrouter_metadata.is_byok
openrouter_metadata.endpoints
raw openrouter_metadata
prompt_hash
completion_hash
```

The inline `openrouter_metadata` may identify the selected provider and endpoint candidates, but it may not restate `quantization`. Join the response back to the endpoint snapshot taken immediately before the run.

## Post-hoc generation metadata

After the inference response returns, call the generation endpoint with the returned generation ID:

```bash
curl -G https://openrouter.ai/api/v1/generation \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -d id=gen-1234567890
```

The generation record can appear after a short delay. Retry a few times with a small backoff. Store both raw JSON and parsed fields.

Suggested parsed fields:

```text
generation_id
request_id
created_at
api_type
streamed
cancelled
provider_name
provider_responses[].provider_name
provider_responses[].endpoint_id
provider_responses[].model_permaslug
provider_responses[].status
provider_responses[].latency
provider_responses[].id
upstream_id
model
origin
user_agent
is_byok
router
finish_reason
native_finish_reason
latency
generation_time
moderation_latency
tokens_prompt
tokens_completion
native_tokens_prompt
native_tokens_completion
native_tokens_reasoning
native_tokens_cached
total_cost
usage
cache_discount
upstream_inference_cost
raw_generation_json
```

The generation metadata is usually stronger for provider, upstream ID, usage, cost, token accounting, and latency. It still does not fully attest the provider runtime.

## Storage model

Use three core tables or equivalent JSONL files.

### `openrouter_model_snapshots`

One row per model in `/api/v1/models` per snapshot.

Primary key:

```text
catalog_snapshot_id + openrouter_model_id
```

Purpose: preserve model-level catalog state and support endpoint joins.

### `openrouter_endpoint_variants`

One row per endpoint in `/api/v1/models/{author}/{slug}/endpoints` per snapshot.

Recommended stable key:

```text
catalog_snapshot_id
+ openrouter_model_id
+ provider_name
+ endpoint_tag
+ endpoint_name
+ quantization
```

If OpenRouter exposes a durable endpoint ID in the endpoint catalog, include it in the key. If the endpoint ID appears only in `/generation`, store it in the probe table and join by provider, tag, model permaslug, quantization, and snapshot time.

Purpose: enumerate all advertised provider variants, including quantization and supported-parameter differences. For `quantization = "unknown"`, the row still represents a distinct variant when provider, endpoint tag, endpoint name, limits, parameter support, pricing, status fields, or raw endpoint metadata differ.

### `openrouter_variant_probes`

One row per probe request.

Primary key:

```text
probe_run_id
```

Foreign keys:

```text
catalog_snapshot_id
openrouter_model_id
provider_name
endpoint_tag
quantization
```

Purpose: record observed routability and request-level behavior for one endpoint variant at one time.

## Probe policy

Do not probe every endpoint on every run by default. Full probing can be expensive and can create rate-limit pressure.

Use tiers:

1. Catalog-only inventory for all models and all endpoints.
2. Daily or pre-eval probes for endpoint variants in the active evaluation pool.
3. Full sweep only when building or refreshing a provider baseline, and with rate limits, backoff, and resumable state.

For active evals, snapshot endpoint metadata immediately before the eval run and probe the endpoint variants that will be used by the run. The eval result should link to that snapshot.

## Eval request policy

For fixed-provider evals, always specify provider routing controls:

```json
{
  "provider": {
    "only": ["<endpoint tag or provider>"],
    "allow_fallbacks": false,
    "require_parameters": true,
    "quantizations": ["<catalog quantization>"]
  }
}
```

Also set generation parameters explicitly:

```json
{
  "temperature": 0,
  "top_p": 1,
  "max_tokens": 1024
}
```

Set `seed` if the target endpoint supports it. For structured-output or tool-use evals, include `response_format`, `tools`, and `tool_choice` as needed, and keep `require_parameters: true`.

## Reconstruction rule

For every eval output, record enough information to reconstruct what was evaluated:

```text
openrouter_model_id
+ returned_model or model_permaslug
+ requested provider constraint
+ selected provider from metadata
+ endpoint tag or provider response endpoint ID
+ requested quantization constraint
+ catalog quantization from the endpoint snapshot
+ fallback policy
+ supported-parameter policy
+ request parameters
+ eval suite version
+ eval timestamp
```

Quantization is reconstructed from the endpoint snapshot and the request constraint. Do not assume the inference response will contain it. If catalog quantization is `unknown`, reconstruct it as `unknown for this specific endpoint`, not as a fungible class shared with other unknown endpoints.

## Interpretation rule

Use OpenRouter metadata to measure the routed OpenRouter product. Use controlled deployment to measure a controlled model artifact.

Collapse results across providers only when the research question is OpenRouter default routing behavior. For model-quality comparisons, keep provider endpoint variants separate unless there is a deliberate aggregation rule.
