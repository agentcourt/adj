# Quick adjudication

Quick adjudication gives a proponent and an opponent one argument each, then asks a council to vote on the proposition.  The procedure does not invoke Lean or add later argument rounds.  The caller must state the council size, required majority, evidence standard, document limits, and council request-spec pool.  The core samples distinct council records from that pool with cryptographic randomness and records the selected endpoint, model, and persona metadata.

The core exposes one local HTTP API for the lawyer turns.  Its paths match the lawyer portion of the AAR case API, allowing the adjservices AAR MCP adapter to present the turns to external agents.  Council members run through direct Responses-compatible provider calls, which require `--allow-api-key`.  Before opening a lawyer turn, the core checks every selected endpoint for supported configuration and required credentials without sending a provider request.  A request-spec model reference may not contain a query or fragment.

## Command

From the module root, build `./quick/cmd/quick` with an explicit output path or run it with `go run ./quick/cmd/quick`.  This example states every procedure and document-acceptance parameter.  Omitting `--documents` creates an empty immutable document set, but the limits remain required so the same request can accept documents without changing its policy.

```text
quick case \
  --proposition "The sky is blue" \
  --documents ./case-documents \
  --out-dir ./quick-case \
  --council-pool ./pool.jsonl \
  --council-size 3 \
  --required-votes 2 \
  --evidence-standard "preponderance of the evidence" \
  --max-document-files 20 \
  --max-document-file-bytes 1048576 \
  --max-documents-total-bytes 4194304 \
  --allow-api-key
```

The two lawyer agents use role IDs `plaintiff` and `defendant`.  Each calls `submit_decision` with `kind=tool`, `tool_name=submit_argument`, and an argument in `payload.text`.  The opponent receives the proponent's accepted argument, and each lawyer can read the immutable documents through the evidence-list, stat, and range-read tools.

Council requests run sequentially by default.  `--parallel-council` starts all selected council requests together and cancels outstanding requests after the first observed failure.  The procedure waits for every started request to return, then records successful votes in council-roster order rather than response-arrival order.

Each council request contains the proposition and accepted arguments followed by verified documents in path order.  UTF-8 documents use text content items, `image/*` documents use image data URLs, and PDFs use file content items.  Another binary media type fails during initialization, before the lawyer API opens or a provider request begins.  Request-spec metadata does not describe input modalities, so a provider or model can reject an encoded image or PDF through the normal provider error path.

## Records

The output directory must be empty.  The core writes the current result as the case progresses and leaves a complete terminal record.  `case-manifest.json` supports case discovery by adjservices.

| Path | Contents |
| --- | --- |
| `case-manifest.json` | Procedure, case, run, version, start time, and case API address. |
| `input.json` | Resolved proposition, policy, limits, identifiers, and council mode. |
| `runtime.json` | Local case API address and runtime identity. |
| `documents.json` and `documents/` | Imported document metadata, hashes, and bytes. |
| `events.ndjson` | Ordered procedure events. |
| `work-notes.jsonl` | Private lawyer notes. |
| `transcript.json` | The two arguments and council votes. |
| `run.json` | Current or terminal result and provider accounting. |

Council records contain the selected endpoint/model reference, persona filename, and available provider token usage and cost for each vote.  The terminal `provider` object counts logical council requests and separately counts responses that supplied usage or cost, allowing its totals to represent partial observations.  Provider request headers remain in memory for the provider call and do not appear in the durable council, transcript, event, or run records.

Imported files reside under `documents/`, retain relative paths, and carry byte counts and SHA-256 digests in `documents.json`.  The council reads each file through the shared verified reader, which rejects type, size, digest, or path changes.  A provider-metadata error remains on its vote when the completed response omits inline cost and the metadata lookup fails.  This record distinguishes unknown cost from zero cost without treating metadata retrieval as an adjudication failure.
