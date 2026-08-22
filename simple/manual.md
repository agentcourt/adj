# Simple Adjudication Manual

The `simple` executable evaluates one proposition with one model request.  The caller states the evidence standard, supplies optional documents, and selects one provider request specification.  A valid response contains exactly one `submit_simple_decision` call with a `demonstrated` or `not_demonstrated` decision and a nonempty rationale.

## Command

The executable has one subcommand:

```text
simple case [flags]
```

| Flag | Meaning |
| --- | --- |
| `--proposition TEXT` | Proposition presented for decision. |
| `--evidence-standard TEXT` | Rule the model applies when deciding whether the evidence demonstrates the proposition. |
| `--documents DIR` | Optional directory imported recursively as case documents. |
| `--out-dir DIR` | Empty output directory for the case record. |
| `--case-id ID` | Case identifier.  The default is `simple-1`. |
| `--run-id ID` | Run identifier.  The command generates one when omitted. |
| `--request-spec FILE` | Request-spec JSON file.  This flag excludes `--model`. |
| `--model endpoint://model` | Direct model reference without a query or fragment.  This flag excludes `--request-spec`. |
| `--reasoning-effort VALUE` | Optional requested reasoning effort: `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, or `max`. |
| `--max-output-tokens N` | Optional positive output-token cap.  Zero uses the request specification or the 4096-token procedure default. |
| `--max-tool-calls N` | Optional positive cap on built-in tool calls.  Zero uses the request specification or provider default. |
| `--prompt-dir DIR` | Uses a complete Simple prompt directory. |
| `--prompt-file ID=PATH` | Overrides one prompt by catalog ID.  The flag may be repeated. |
| `--allow-api-key` | Explicitly permits a provider request authenticated by an API key. |
| `--web-search BOOL` | Makes provider-hosted web search available to the model.  The default is `true`; use `--web-search=false` to disable it. |
| `--max-documents N` | Required maximum document count. |
| `--max-document-bytes N` | Required maximum bytes for one document. |
| `--max-documents-bytes N` | Required maximum bytes across all documents. |
| `--timeout-seconds N` | Provider timeout.  The default is 240 seconds. |

The command rejects a missing proposition, evidence standard, output directory, authentication directive, or limit before it creates a response client.  It also rejects both request-spec flags together and rejects a run with neither source.  An output directory may be absent or empty, which prevents a case from overwriting another record.

`openai://` models use `OPENAI_API_KEY`, while `openrouter://` models use `OPENROUTER_API_KEY`.  The command rejects a model reference containing a query or fragment without reproducing the rejected value.  It reads credential values only after `--allow-api-key` has authorized a provider request.  A missing or rejected credential produces a typed `provider_authentication` error and a nonzero exit status.

Hosted web search is available by default.  The provider request uses the current Responses API `web_search` tool, and the model decides whether the proposition requires a search.  `--web-search=false` removes that tool from the request.

`--reasoning-effort` overrides `request.reasoning_effort` in a request-spec file.  When both sources omit the value, Simple omits the [Responses API `reasoning` field](https://developers.openai.com/api/reference/cli/resources/responses/methods/create) and preserves the provider and model default.  Simple accepts the union of current Responses API effort values because each model supports a different subset, so the provider rejects a value that the selected model does not support.

`--max-output-tokens` overrides `request.max_output_tokens` or the legacy `request.max_tokens` value in a request-spec file.  A request-spec value takes effect when the command-line option is zero.  When neither source supplies a cap, Simple requests 4096 maximum output tokens.

`--max-tool-calls` overrides `request.max_tool_calls` in a request-spec file.  A request-spec value takes effect when the command-line option is zero, while omission from both sources leaves the provider default in effect.  The [Responses API field](https://developers.openai.com/api/reference/cli/resources/responses/methods/create) limits the total calls to built-in tools across the response, including web-search actions, and does not count the custom decision function.

The [prompt-authoring guide](../docs/prompt-authoring.md) lists the prompt catalog, conventional paths, and replacement tokens.  A complete prompt directory supplies every catalog file.  A repeated `--prompt-file` option supplies selected files by catalog ID.

OpenAI executes the [hosted Responses web-search tool](https://developers.openai.com/api/docs/guides/tools-web-search) under the configured OpenAI account.  OpenRouter accepts the same `web_search` declaration through its [hosted web-search interface](https://openrouter.ai/docs/guides/routing/model-variants/online).  The endpoint's existing credential authorizes the model and search request, and provider search charges apply to that account.

## Documents and Requests

The importer walks the document root recursively, includes hidden regular files, and sorts relative paths by their byte representation.  It rejects symbolic links, special files, path escapes, file-count violations, per-file byte violations, total-byte violations, and a source file that changes during import.  The record preserves each relative path, exact byte count, media type, SHA-256 hash, and copied bytes beneath `documents/`.

Simple adjudication sends UTF-8 text as `input_text`, an `image/*` document as an `input_image` data URL, and an `application/pdf` document as `input_file` data.  It rejects other media before creating the response client.  Request-spec metadata does not describe input modalities, so a provider or model can reject an encoded image or PDF through the normal provider error path.  The provider request tells the model to apply the evidence standard to the proposition, any supplied documents, and relevant established knowledge; the absence of documents does not determine the decision.

The decision tool schema requires `decision` and `rationale`, rejects other properties, and restricts the decision values to `demonstrated` and `not_demonstrated`.  The runtime requires exactly one `submit_simple_decision` function call while allowing any provider-hosted web-search calls that precede it.  Zero decision calls, multiple decision calls, another function name, malformed JSON, an unknown field, an invalid decision, or an empty rationale produces `provider_protocol` without another model request.

## Record

| Path | Contents |
| --- | --- |
| `case-manifest.json` | Procedure, case, run, start time, and core version. |
| `input.json` | Proposition, evidence standard, identifiers, and document-presence flag. |
| `runtime.json` | Evidence standard, authentication directive, resolved web-search setting, reasoning effort, output-token cap, built-in-tool-call cap, document limits, timeout, and one-attempt policy. |
| `documents.json` | Ordered document paths, media types, sizes, and hashes. |
| `documents/` | Imported document bytes with their original hierarchy. |
| `model-request.json` | Redacted request specification, fixed prompt, proposition, document references, resolved web-search setting, reasoning effort, output-token cap, built-in-tool-call cap, and tool schemas. |
| `model-response.json` | Raw response, response identifier, parsed decision and search calls, search sources, URL citations, provider metadata, token usage, available provider cost, and typed error. |
| `decision.json` | Parsed decision or terminal decision error. |
| `state.json` | Terminal procedure state, including the requested reasoning effort, output-token cap, and built-in-tool-call cap. |
| `events.ndjson` | Ordered initialization, provider, decision, and failure events, including the effective request controls. |
| `transcript.md` | Human-readable proposition, document index, decision, web-search counts, and model-request controls. |
| `digest.md` | Human-readable case, web-search, and model-request summary. |
| `run.json` | Atomic terminal result, including provider accounting, web-search counts, requested reasoning effort, output-token cap, and built-in-tool-call cap. |

The request record redacts every request-header value because a custom header may contain a credential.  It refers to imported documents by path, size, media type, and hash instead of copying their text or data URLs.  The document directory and manifest retain the exact request material.

Provider usage records input, cached-input, output, reasoning, and total token counts from the completed response.  OpenRouter cost comes from the completed response when present and otherwise from available generation metadata.  The `provider` object records one logical request and uses observed-value counts to distinguish unavailable usage or cost from zero.

The request asks the provider to return `web_search_call.action.sources`.  The response record preserves each search action, query, source URL, and output `url_citation` with its title and text offsets, while the raw provider JSON remains available for fields outside the normalized record.  The terminal records count search calls, returned sources, and URL citations; zero counts mean that the model received the tool but did not use it or that the provider returned no corresponding data.

The command writes one JSON result to standard output.  Exit status zero identifies a valid terminal decision, while configuration, input, storage, provider, cancellation, and response-protocol errors return a nonzero status.  Once the output directory exists, document, media, and provider failures write terminal state and `run.json` when the filesystem permits those records.
