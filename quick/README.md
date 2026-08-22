# Quick adjudication

Quick adjudication gives a proponent and an opponent one argument each, then asks a council to vote on the proposition.  The procedure does not invoke Lean or add later argument rounds.  The caller must state the council size, required majority, evidence standard, and document limits.  The core resolves a council request-spec pool, samples distinct records from it with cryptographic randomness, and records the selected endpoint, model, and persona metadata.

The core exposes one local HTTP API for the lawyer turns.  The procedure-specific `quick-mcp` adapter translates external participant requests to that API, while `aar-mcp` handles AAR's wider lawyer and council APIs.  Every `/lawyerapi/v1` request requires the bearer token supplied through `--lawyerapi-bearer-token-file`; `/health` remains public for process supervision.  The token file must be a regular file with no group or other access, and the core omits its contents from logs and records.  `quick-mcp` reads the same token through `--caseapi-bearer-token-file`, separate from the signing key and role capabilities described in the [prompt-authoring guide](../docs/prompt-authoring.md#mcp-capabilities).  Council members run through direct Responses-compatible provider calls, which require `--allow-api-key`.  Before opening a lawyer turn, the core checks candidate endpoints with bounded Responses requests and replaces candidates that are unavailable.  A request-spec model reference may not contain a query or fragment.

## Command

Run `make -C quick build` from the module root to build `quick/.bin/quick` and `quick/.bin/quick-mcp`.  This example states every procedure and document-acceptance parameter.  Omitting `--documents` creates an empty immutable document set, but the limits remain required so the same request can accept documents without changing its policy.  The prompt-file options and their conventional paths appear in the [prompt-authoring guide](../docs/prompt-authoring.md).

```text
quick case \
  --proposition "The sky is blue" \
  --documents ./case-documents \
  --out-dir ./quick-case \
  --council-size 3 \
  --required-votes 2 \
  --evidence-standard "preponderance of the evidence" \
  --lawyerapi-bearer-token-file ./private/quick-caseapi.token \
  --lawyer-web-search=true \
  --max-document-files 20 \
  --max-document-file-bytes 1048576 \
  --max-documents-total-bytes 4194304 \
  --allow-api-key
```

An explicit `--council-pool` selects that file.  Without that option, Quick selects `./pool.jsonl` when it exists, then `<common-root>/data/personas/pool.jsonl`.  `--common-root` overrides the shared `common` directory, whose default comes from the working directory and its ancestors.

The two lawyer agents use role IDs `plaintiff` and `defendant`.  Each calls `submit_decision` with `kind=tool`, `tool_name=submit_argument`, and an argument in `payload.text`.  The opponent receives the proponent's accepted argument, and each lawyer can read the immutable documents through the evidence-list, stat, and range-read tools.

Quick enables lawyer web search by default.  `--lawyer-web-search=false` selects the disabled search instructions, while repeated `--prompt-file ID=PATH` options override selected prompts.  The [prompt-authoring guide](../docs/prompt-authoring.md) lists the prompt IDs and conventional paths.  The council receives only the proposition, documents, arguments, and decision tool.

Council requests run sequentially by default.  `--parallel-council` starts all selected council requests together and cancels outstanding requests after the first observed failure.  The procedure waits for every started request to return, then records successful votes in council-roster order rather than response-arrival order.

Quick excludes a pool record before sampling when its endpoint metadata states that the model does not support `tools`, which the vote request requires.  It shuffles the eligible records and requires each candidate to return a valid `submit_council_vote` call in a bounded availability request.  A request, protocol, transient, or request-specific authentication failure advances to the next candidate.  A missing endpoint credential excludes the remaining candidates for that provider endpoint while candidates for other endpoints remain eligible.  `council_candidate_rejected` events record the safe route identity of each failed candidate and its replacement, and provider accounting includes every availability request.

Each council request contains the proposition and accepted arguments followed by verified documents in path order.  UTF-8 documents use text content items, `image/*` documents use image data URLs, and PDFs use file content items.  Another binary media type fails during initialization, before the lawyer API opens or a provider request begins.  Request-spec metadata does not describe input modalities, so a provider or model can reject an encoded image or PDF through the normal provider error path.

## Records

The output directory must be empty.  The core writes the current result as the case progresses and leaves a complete terminal record.  `case-manifest.json` supports case discovery by adjservices.

| Path | Contents |
| --- | --- |
| `case-manifest.json` | Procedure, case, run, version, start time, and case API address. |
| `input.json` | Resolved proposition, policy, lawyer web-search setting, limits, identifiers, and council mode. |
| `runtime.json` | Local case API address and runtime identity. |
| `documents.json` and `documents/` | Imported document metadata, hashes, and bytes. |
| `events.ndjson` | Ordered procedure events. |
| `work-notes.jsonl` | Private lawyer notes. |
| `transcript.json` | The two arguments and council votes. |
| `run.json` | Current or terminal result and provider accounting. |

Council records contain the selected endpoint/model reference, persona filename, endpoint-variant identifier, provider name and tag, quantization, and effective provider restrictions.  The terminal `provider` object counts logical council requests and separately counts responses that supplied usage or cost, allowing its totals to represent partial observations.  Provider request headers remain in memory for the provider call and do not appear in the durable council, transcript, event, or run records.

Imported files reside under `documents/`, retain relative paths, and carry byte counts and SHA-256 digests in `documents.json`.  The council reads each file through the shared verified reader, which rejects type, size, digest, or path changes.  A provider-metadata error remains on its vote when the completed response omits inline cost and the metadata lookup fails.  This record distinguishes unknown cost from zero cost without treating metadata retrieval as an adjudication failure.
