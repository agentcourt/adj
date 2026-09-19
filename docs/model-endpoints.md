# Council and Jury Model Endpoints

Quick, ARB, ARBD, and ADC select council or juror model configurations from a JSONL pool.  One shared executor sends those requests to OpenAI, Anthropic, Google, xAI, DeepSeek, Hugging Face Inference Providers, or OpenRouter.  The endpoint named in each pool record determines the provider and credential.  The executor does not change endpoints after selection.

Simple has a separate direct-model interface.  It continues to accept `openai://MODEL` and `openrouter://MODEL` because its optional web search uses those providers' Responses APIs.

## Pool Records and Selection

Each nonblank pool line contains one request specification.  The `model` value omits the endpoint prefix because `endpoint` is a separate field.

```jsonl
{"endpoint":"openai","model":"gpt-5.6-luna","request":{"reasoning_effort":"high","max_output_tokens":4096},"persona":"generic.md"}
{"endpoint":"anthropic","model":"claude-sonnet-5","request":{"reasoning_effort":"high","max_output_tokens":4096},"persona":"generic.md"}
{"endpoint":"google","model":"gemini-3.5-flash","request":{"reasoning_effort":"high","max_output_tokens":4096},"persona":"generic.md"}
```

| Field | Meaning |
| --- | --- |
| `endpoint` | Registered endpoint name.  Required. |
| `model` | Model identifier sent to that endpoint.  Required. |
| `persona` | Persona file resolved from the pool directory and then from the shared common tree.  Required by the procedure pool loaders. |
| `request` | Optional `temperature`, `top_p`, `max_tokens`, `max_output_tokens`, `max_tool_calls`, and `reasoning_effort` values. |
| `provider` | Optional OpenRouter routing constraints.  Other endpoints reject this field. |
| `headers` | Optional request headers for an OpenAI-compatible endpoint.  The executor rejects authorization and host headers.  Native Anthropic, Google, and DeepSeek requests reject all custom headers. |
| `variant_metadata` | Optional route, quantization, compatibility, price, and inventory metadata retained with the selected configuration. |

The pool may contain the same configuration more than once, and one configuration may occupy more than one seat.  For each seat, the selector chooses an eligible endpoint with the fewest assigned seats, then a configuration for that endpoint with the fewest assigned seats.  It uses cryptographic randomness to break ties.  This ordering balances endpoints before models while preserving variation among equivalent choices.

The default `common/data/personas/pool.jsonl` contains the generated OpenRouter pool.  The optional `common/data/personas/direct-lab-pool.jsonl` contains `openai://gpt-5.6-luna`, `anthropic://claude-sonnet-5`, and `google://gemini-3.5-flash` configurations that completed Quick's availability and vote requests through the named lab services.  Select either file through the procedure's pool option.

Quick and ARB run a bounded availability request before accepting a configuration.  ARBD does the same in direct mode.  ADC runs a `submit_juror_vote` request through the selected configuration before assigning an automatically generated candidate juror.  A failed configuration becomes ineligible.  A missing endpoint credential makes every configuration for that endpoint ineligible.  A configuration that passed an earlier availability request may occupy another seat without a repeated check.  A rejection ends selection when it leaves too few endpoints to meet the configured minimum.  Quick, ARB, and ARBD fail before participant work begins when they cannot fill the requested council.  ADC fails the candidate-assignment action when it cannot select an available configuration.

ARBD's Council API mode selects the roster without making provider requests because the external council clients own those requests.  The complete local runner checks the credentials for the selected endpoints before it starts its Pi council agents.  It records each model request when deliberation begins.

The core commands and complete local-run commands accept these flags:

| Flag | Meaning |
| --- | --- |
| `--council-pool PATH` | Quick, ARB, and ARBD JSONL request-specification pool. |
| `--juror-personas PATH` | ADC JSONL request-specification pool. |
| `--council-endpoint NAME` | Permit one endpoint.  Repeat the flag to permit several endpoints.  Omitting it permits every endpoint represented in the pool. |
| `--minimum-distinct-council-endpoints N` | Require at least `N` endpoint names in a completed Quick, ARB, or ARBD council.  Zero imposes no minimum. |

ADC uses the same endpoint and configuration balancing while it assigns automatically generated candidate jurors.  Its minimum setting rejects a pool with fewer than the requested number of eligible endpoints.  The first candidate assignments span the available endpoints before reusing one.  The preflight result is cached by pool record, so later assignments of the same configuration do not repeat it.  Voir dire may remove candidates, so this setting does not impose a minimum endpoint count on the final jury.

The unified settings names are `common.council_allowed_endpoints` and `common.council_minimum_distinct_endpoints`:

```json
{
  "common": {
    "council_pool": "common/data/personas/direct-lab-pool.jsonl",
    "council_size": 5,
    "council_allowed_endpoints": ["openai", "anthropic", "google"],
    "council_minimum_distinct_endpoints": 3
  }
}
```

## Endpoints and Credentials

The executor uses fixed official service URLs.  A request specification cannot replace a base URL, host header, or credential header.

| Endpoint | Request API | Credential |
| --- | --- | --- |
| `anthropic` | [Anthropic Messages](https://platform.claude.com/docs/en/api/messages/create) at `https://api.anthropic.com/v1/messages` | `ANTHROPIC_API_KEY` |
| `deepseek` | [DeepSeek Chat Completions](https://api-docs.deepseek.com/api/create-chat-completion/) at `https://api.deepseek.com/chat/completions`; strict tools use `/beta/chat/completions` | `DEEPSEEK_API_KEY` |
| `google` | [Google GenerateContent](https://ai.google.dev/api/generate-content) at `https://generativelanguage.googleapis.com/v1beta/models/MODEL:generateContent` | `GEMINI_API_KEY` |
| `huggingface` | [Hugging Face Responses](https://huggingface.co/docs/inference-providers/en/guides/responses-api) at `https://router.huggingface.co/v1/responses` | `HF_TOKEN` |
| `openai` | [OpenAI Responses](https://developers.openai.com/api/reference/cli/resources/responses/methods/create) at `https://api.openai.com/v1/responses` | `OPENAI_API_KEY` |
| `openrouter` | [OpenRouter Responses](https://openrouter.ai/docs/api/api-reference/responses/create-responses) at `https://openrouter.ai/api/v1/responses` | `OPENROUTER_API_KEY` |
| `xai` | [xAI Responses](https://docs.x.ai/developers/rest-api-reference/inference/responses) at `https://api.x.ai/v1/responses` | `XAI_API_KEY` |

Direct core commands read the credential variables named above.  `adjudicate` instead reads each configured source variable and passes its value under the canonical variable name only to the process that owns provider calls:

```json
{
  "common": {
    "allow_api_key": true,
    "provider_credentials": {
      "openai": {
        "source": "api_key",
        "environment_variable": "OPENAI_API_KEY"
      },
      "anthropic": {
        "source": "api_key",
        "environment_variable": "ANTHROPIC_API_KEY"
      },
      "google": {
        "source": "api_key",
        "environment_variable": "GEMINI_API_KEY"
      }
    }
  }
}
```

Quick, ARB, ARBD, and jury ADC load every source declared in `common.provider_credentials` when they construct the core provider environment.  Each declared source variable must therefore contain a credential when one of those procedures starts.  The launcher removes the common provider credentials before applying a lawyer profile, so a lawyer receives only the credentials selected by that profile.  A Pi council or juror process receives only its local gateway token.

## Request Support

Every council and juror request uses function tools for its structured submission.  The executor converts the shared tool definitions, message content, continuation state, response text, tool calls, usage, and provider errors into one internal representation.

The OpenAI, Hugging Face, OpenRouter, and xAI clients send the request through their Responses-compatible APIs.  Their supported reasoning values, content types, and request parameters depend on the selected service and model.  The executor forwards configured `temperature`, `top_p`, output-token limits, `max_tool_calls`, reasoning effort, and ordinary custom headers.  Only OpenRouter accepts the request specification's `provider` routing object.  For an OpenRouter tool exchange, the executor retains the response input and output, sends the full history with the next tool result, and omits `previous_response_id`.  OpenRouter's [Go agent implementation](https://github.com/OpenRouterTeam/go-agent/blob/main/model_result.go) uses the same stateless continuation because the live Responses API rejects a non-null `previous_response_id` on such requests.

The native adapters apply these rules:

| Endpoint | Content and tools | Reasoning effort |
| --- | --- | --- |
| Anthropic | Text, base64 images, base64 documents, and function tools.  A continued tool exchange preserves thinking blocks and groups consecutive tool results in one user message.  Usage includes reported thinking tokens. | `none` disables thinking.  `low`, `medium`, `high`, `xhigh`, and `max` use adaptive thinking and the corresponding output effort. |
| Google | Text, base64 images, base64 documents, and function declarations using `parametersJsonSchema`.  A continued tool exchange preserves thought signatures and groups consecutive function responses. | `minimal`, `low`, `medium`, and `high`, sent as the uppercase GenerateContent values. |
| DeepSeek | Text and function tools.  Strict tools use the beta Chat Completions URL.  A continued exchange preserves `reasoning_content`. | `none` disables thinking.  `low`, `high`, and `max` pass through.  `medium` and `xhigh` map to `high`, as specified by DeepSeek. |

All native adapters accept `temperature`, `top_p`, and an output-token limit.  They reject `max_tool_calls` and custom request headers.  Unsupported content or reasoning values produce a request error before the provider call.  A provider can reject a value that its selected model does not support.

Council members and jurors receive no web-search tool.  Their inputs comprise the case material and procedure tools selected by the procedure.

## Direct Calls and Pi Calls

Quick calls the executor in the Quick process.  `aar case` and `aard case` call it in the core process when `--council-backend direct` is selected.  Direct ADC juror execution also calls the executor in the core process.

The complete local runners `aar-run`, `aard-run`, and `adc-run`, including their use through `adjudicate`, start Pi agents for council or juror opportunities.  The local runner keeps the shared executor and exposes a loopback OpenAI Chat Completions interface that Pi can call.  Each opportunity receives a fresh random model alias and bearer token bound to one upstream request specification.  Pi receives neither the upstream model name nor its credential.  The server accepts streamed and non-streamed Pi requests and preserves append-only continuation history.  It rejects a non-null `tool_choice` field because it cannot enforce that field through every upstream endpoint.  The local runner revokes the opportunity token when its Pi process exits.

When checking returned history, the server treats omitted, null, empty, and whitespace-only assistant content as equivalent for messages containing tool calls.  It compares parsed tool arguments because Pi serializes them again between requests.  When the provider's original argument string is malformed JSON, the comparison accepts Pi's parsed argument object while checking the remaining message fields, including tool names and call identifiers.  Continued provider requests retain the original provider response.  These comparison rules do not replace its arguments with Pi's parsed copy.

The local runner writes every Pi-to-provider request to JSONL:

| Procedure | Request record |
| --- | --- |
| ARB | `logs/council-model-requests.jsonl` |
| ARBD | `logs/council-model-requests.jsonl` |
| ADC | `logs/juror-model-requests.jsonl` |

Each row records start and finish times, endpoint, requested and returned model identifiers, response identifier, observed usage, and provider failure data.  A failed or canceled continuation also receives a row.

Direct Quick, ARB, ARBD, and ADC calls contribute to the core provider-accounting object.  Pi council and juror calls record usage in the JSONL request log and do not contribute to the formal core's provider-accounting totals.  Participant-model usage and procedure-provider accounting remain separate in unified records.
