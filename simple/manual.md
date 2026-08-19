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
| `--allow-api-key` | Explicitly permits a provider request authenticated by an API key. |
| `--max-documents N` | Required maximum document count. |
| `--max-document-bytes N` | Required maximum bytes for one document. |
| `--max-documents-bytes N` | Required maximum bytes across all documents. |
| `--timeout-seconds N` | Provider timeout.  The default is 240 seconds. |

The command rejects a missing proposition, evidence standard, output directory, authentication directive, or limit before it creates a response client.  It also rejects both request-spec flags together and rejects a run with neither source.  An output directory may be absent or empty, which prevents a case from overwriting another record.

`openai://` models use `OPENAI_API_KEY`, while `openrouter://` models use `OPENROUTER_API_KEY`.  The command rejects a model reference containing a query or fragment without reproducing the rejected value.  It reads credential values only after `--allow-api-key` has authorized a provider request.  A missing or rejected credential produces a typed `provider_authentication` error and a nonzero exit status.

## Documents and Requests

The importer walks the document root recursively, includes hidden regular files, and sorts relative paths by their byte representation.  It rejects symbolic links, special files, path escapes, file-count violations, per-file byte violations, total-byte violations, and a source file that changes during import.  The record preserves each relative path, exact byte count, media type, SHA-256 hash, and copied bytes beneath `documents/`.

Simple adjudication sends UTF-8 text as `input_text`, an `image/*` document as an `input_image` data URL, and an `application/pdf` document as `input_file` data.  It rejects other media before creating the response client.  The provider request tells the model to apply the evidence standard to the proposition, any supplied documents, and relevant established knowledge; the absence of documents does not determine the decision.

The tool schema requires `decision` and `rationale`, rejects other properties, and restricts the decision values to `demonstrated` and `not_demonstrated`.  The runtime validates the returned call independently of the schema.  Zero calls, multiple calls, another tool name, malformed JSON, an unknown field, an invalid decision, or an empty rationale produces `provider_protocol` without another model request.

## Record

| Path | Contents |
| --- | --- |
| `case-manifest.json` | Procedure, case, run, start time, and core version. |
| `input.json` | Proposition, evidence standard, identifiers, and document-presence flag. |
| `runtime.json` | Evidence standard, authentication directive, limits, timeout, and one-attempt policy. |
| `documents.json` | Ordered document paths, media types, sizes, and hashes. |
| `documents/` | Imported document bytes with their original hierarchy. |
| `model-request.json` | Redacted request specification, fixed prompt, proposition, document references, and tool schema. |
| `model-response.json` | Raw response, response identifier, parsed calls, provider metadata, token usage, available provider cost, and typed error. |
| `decision.json` | Parsed decision or terminal decision error. |
| `state.json` | Terminal procedure state. |
| `events.ndjson` | Ordered initialization, provider, decision, and failure events. |
| `transcript.md` | Human-readable proposition, document index, and decision. |
| `digest.md` | Human-readable case summary. |
| `run.json` | Atomic terminal result, including the common provider-accounting object. |

The request record redacts every request-header value because a custom header may contain a credential.  It refers to imported documents by path, size, media type, and hash instead of copying their text or data URLs.  The document directory and manifest retain the exact request material.

Provider usage records input, cached-input, output, reasoning, and total token counts from the completed response.  OpenRouter cost comes from the completed response when present and otherwise from available generation metadata.  The `provider` object records one logical request and uses observed-value counts to distinguish unavailable usage or cost from zero.

The command writes one JSON result to standard output.  Exit status zero identifies a valid terminal decision, while configuration, input, storage, provider, cancellation, and response-protocol errors return a nonzero status.  Once the output directory exists, document, media, and provider failures write terminal state and `run.json` when the filesystem permits those records.
