# Simple Adjudication

Simple adjudication asks one model to decide one proposition under a stated evidence standard.  The model uses the proposition, relevant established knowledge, and any staged documents, then must call `submit_simple_decision` exactly once.  The Go runtime records the decision, provider response, request settings, and document hashes as its verification evidence.

The command requires explicit document limits and explicit permission to use API-key authentication.  It accepts one request specification through `--request-spec` or one model reference through `--model endpoint://model`, and it rejects a model reference containing a query or fragment.  Optional command-line settings select the requested reasoning effort, output-token cap, and built-in-tool-call cap, overriding corresponding request-spec values.  Provider-hosted web search is available by default for OpenAI and OpenRouter models, while `--web-search=false` disables it.  Simple sends OpenAI's `web_search` tool to OpenAI and OpenRouter's `openrouter:web_search` server tool to OpenRouter.  The command makes one provider attempt and treats malformed decision output as a provider-protocol error.

Build and test the command from this directory:

```sh
make build
make test
```

Run one case with an environment credential already configured for the selected endpoint:

```sh
.bin/simple case \
  --proposition "The sky is blue" \
  --evidence-standard "preponderance of the evidence" \
  --documents ./documents \
  --out-dir ./out/example \
  --model openai://MODEL \
  --allow-api-key \
  --max-documents 100 \
  --max-document-bytes 10485760 \
  --max-documents-bytes 104857600
```

The output directory contains the imported bytes, hashes, model request description, raw provider response, parsed decision, terminal state, events, transcript, digest, and atomic `run.json`.  The model-response record retains normalized web-search calls, source URLs, URL citations, per-response usage, and cost; the run record contains web-search counts, effective request controls, and the common provider-accounting object.  The model-request record contains the resolved web-search setting, reasoning effort, output-token cap, built-in-tool-call cap, effective tool list, document references, hashes, and redacted request-header values, and [the manual](manual.md) defines the flags, accepted media, error behavior, and record files.
