# Unified Adjudication Command

The `adjudicate` command provides one entry point for the core adjudication procedures.  A request selects a procedure, supplies the matter and its documents, and selects a reusable settings file.  The command validates and resolves the request before it starts a procedure or contacts a provider.

## Ownership

`adj` owns `adjudicate`, the five procedure cores, their MCP adapters, local participant launchers, prompts, records, and verification.  The supported procedure identifiers are `simple`, `quick`, `arbd`, `arb`, and `adc`.  One `adj` checkout executes every supported one-case procedure.

The public procedure identifier `arbd` selects the `aard` core implementation, while `arb` selects `aar`.  The `adc` identifier selects the ADC implementation.  The `simple` identifier selects one direct model decision, while `quick` selects one proponent argument, one opponent argument, and direct council voting.  The optional `adjservices` repository provides managed multi-case services, deployment, web applications, and reporting by executing installed `adj` commands.

## Command Interface

The command accepts one procedure, one proposition, and one settings file.  A document root is optional.  Case, run, and output identifiers may be supplied or generated.

Build all procedure and launcher commands from the repository root.  `make build` writes `adjudicate` to `.bin/` and writes each procedure's commands to its own `.bin/` directory.  The settings example below assumes that `mysettings.json` is in the repository root and names each command and working directory explicitly.

```bash
make build
```

```text
.bin/adjudicate \
  --proc arb \
  --proposition "The sky is blue" \
  --documents ./documents \
  --settings mysettings.json
```

| Flag | Meaning |
| --- | --- |
| `--proc` | Procedure identifier: `simple`, `quick`, `arbd`, `arb`, or `adc`. |
| `--proposition` | Proposition presented for adjudication. |
| `--documents` | Optional document-root directory imported as initial case material. |
| `--settings` | Settings file containing common values and procedure-specific values. |
| `--case-id` | Optional case identifier.  The command generates one when omitted. |
| `--run-id` | Optional run identifier.  The command generates one when omitted. |
| `--out-dir` | Optional output directory.  The command derives one beneath the configured output root when omitted. |

The command-line flags create a canonical adjudication request under `inputs/adjudicate-request.json`.  Paths refer to the machine running `adjudicate`, and relative paths in the settings file resolve from the settings file's directory.  The command rejects positional arguments and writes one JSON result to standard output.

## Request Model

The common matter contains a proposition and zero or more documents.  The selected procedure converts that matter into its native case input and records the conversion.  Procedure settings contain policy, participant, provider, and runtime choices.  The canonical JSON request encodes those values and the selected procedure.

```json
{
  "schema_version": "adjudicate.request.v1",
  "procedure": "arb",
  "case_id": "case-1",
  "run_id": "run-1",
  "proposition": "The sky is blue",
  "documents": {
    "root": "./documents"
  },
  "settings_file": "mysettings.json",
  "out_dir": "out/case-1/run-1"
}
```

`--proposition` supplies the matter directly.  `arbd` converts it into an AARD question, `arb` converts it into a proposition complaint, ADC converts it into a declaratory claim between Proponent and Opponent, and simple and quick use the text as their direct proposition.  The stored request preserves the original text before procedure conversion.

## Settings

One settings file contains common defaults and a separate object for each configured procedure.  Changing `--proc` selects another procedure object from the same file.  A settings file may contain any subset of procedure objects.

```json
{
  "schema_version": "adjudicate.settings.v1",
  "common": {
    "output_root": "out",
    "agent_state_root": "out/.agents",
    "lawyer_profile": "openclaw-lawyers",
    "evidence_standard": "preponderance_of_the_evidence",
    "council_pool": "common/data/personas/pool.jsonl",
    "council_size": 3,
    "required_votes": 2,
    "document_limits": {
      "count": 100,
      "per_file_bytes": 1048576,
      "total_bytes": 8388608
    },
    "allow_api_key": true,
    "provider_credentials": {
      "openai": {
        "source": "api_key",
        "environment_variable": "OPENAI_API_KEY"
      },
      "openrouter": {
        "source": "api_key",
        "environment_variable": "OPENROUTER_API_KEY"
      }
    }
  },
  "agent_profiles": {
    "openclaw-lawyers": {
      "runner": "openclaw",
      "reasoning_effort": "xhigh"
    }
  },
  "procedures": {
    "simple": {
      "core_command": "simple/.bin/simple",
      "core_working_directory": ".",
      "model": "openrouter://MODEL",
      "reasoning_effort": "xhigh",
      "max_output_tokens": 16384,
      "max_tool_calls": 8,
      "prompt_dir": "prompts/simple",
      "prompt_files": {
        "decision": "prompts/simple/decision.md"
      }
    },
    "quick": {
      "core_command": "quick/.bin/quick",
      "core_working_directory": ".",
      "mcp_command": "quick/.bin/quick-mcp",
      "mcp_working_directory": ".",
      "auto_lawyers": "both",
      "mcp_listen": "127.0.0.1:0",
      "prompt_dir": "prompts/quick",
      "prompt_files": {
        "lawyer.common": "prompts/quick/lawyers/common.md",
        "mcp.session.instructions": "prompts/quick/mcp/session.md"
      },
      "launcher_prompt_dir": "prompts/quick",
      "launcher_prompt_files": {
        "participant": "prompts/quick/participants/default.md"
      }
    },
    "arbd": {
      "core_command": "arbd/.bin/aard",
      "core_working_directory": ".",
      "mcp_command": "arbd/.bin/aard-mcp",
      "mcp_working_directory": ".",
      "auto_lawyers": "both",
      "judgment_standard": "Score the evidence from 0 through 100.",
      "web_search": true,
      "prompt_dir": "prompts/arbd",
      "prompt_files": {
        "attorney.common": "prompts/arbd/attorney/common.md",
        "mcp.session.instructions": "prompts/arbd/mcp/session.md"
      },
      "launcher_prompt_dir": "prompts/arbd"
    },
    "arb": {
      "core_command": "arb/.bin/aar",
      "core_working_directory": ".",
      "mcp_command": "arb/.bin/aar-mcp",
      "mcp_working_directory": ".",
      "auto_lawyers": "both",
      "web_search": true,
      "prompt_dir": "prompts/arb",
      "prompt_files": {
        "attorney.common": "prompts/arb/attorney/common.md",
        "mcp.session.instructions": "prompts/arb/mcp/session.md"
      },
      "launcher_prompt_dir": "prompts/arb"
    },
    "adc": {
      "core_command": "adc/.bin/adc",
      "core_working_directory": ".",
      "mcp_command": "adc/.bin/adc-mcp",
      "mcp_working_directory": ".",
      "auto_lawyers": "both",
      "trial_mode": "bench",
      "web_search": true,
      "prompt_dir": "prompts/adc",
      "prompt_files": {
        "runtime.opportunity": "prompts/adc/runtime/opportunity.md",
        "mcp.session.instructions": "prompts/adc/mcp/session.md"
      },
      "launcher_prompt_dir": "prompts/adc"
    }
  }
}
```

Every procedure object accepts `prompt_dir` for a complete `adj` prompt catalog and `prompt_files` for a partial ID-to-path map.  Simple sends all entries to its core, while Quick, AAR, AARD, and ADC send IDs beginning with `mcp.` to their procedure-specific MCP child and all remaining IDs to the core.  Both children receive the complete directory, allowing one procedure tree to contain core entries and the adapter's `mcp/` subtree.

Quick, AAR, AARD, and ADC separately accept `launcher_prompt_dir` and `launcher_prompt_files` for participant bootstrap text and remote-lawyer skills.  Every configured prompt path resolves from the settings file, while an omitted setting leaves conventional process-working-directory paths and compiled fallbacks available.  The [prompt-authoring guide](docs/prompt-authoring.md) defines precedence, every launcher ID and token, and the boundary among procedure, MCP, and launcher prompts.

The common evidence standard applies when the file configures `simple`, `quick`, `arb`, or `adc`, using `preponderance_of_the_evidence` or `clear_and_convincing`.  The council pool and size also apply to `arbd`, while its required `judgment_standard` defines its scoring policy.  `arbd` does not consume the common evidence standard or required-vote threshold.  The required-vote threshold applies to `arb`, `quick`, and jury or automatic `adc`; a jury or automatic `adc` run requires six through twelve jurors and at least six concurring votes.

The `adjudicate.settings.v1` schema defines the common fields, agent profiles, and five procedure objects.  The loader rejects unknown fields, invalid values, and missing required values before it creates a case directory or contacts a provider.  It resolves each configured procedure once with the common settings, selects the requested procedure from those resolved values, and writes the selection to `inputs/resolved-settings.json`.  It requires a home directory only when a configured subscription profile or home-relative path needs one.

Settings identify models, providers, agent profiles, timeouts, byte limits, and output roots.  Direct-provider credentials use the common source metadata, while agent subscription credentials use the selected agent profile.  The resolved record stores credential paths and environment-variable names without storing credential values.

| Common field | Rule |
| --- | --- |
| `output_root` | Defaults to `out` relative to the settings file. |
| `agent_state_root` | Defaults to `<output_root>/.agents`. |
| `lawyer_profile` | Optional fallback for an omitted plaintiff or defendant profile name. |
| `evidence_standard` | Required when `simple`, `quick`, `arb`, or `adc` is configured.  Accepts `preponderance_of_the_evidence` or `clear_and_convincing`. |
| `council_pool`, `council_size` | Required when any configured `arbd`, `arb`, `quick`, jury ADC, or automatic ADC object needs a council. |
| `required_votes` | Required for `arb`, `quick`, jury ADC, and automatic ADC.  `arbd` does not consume it. |
| `document_limits` | Required positive count, per-file byte, and total-byte limits. |
| `allow_api_key` | Required and true because the implemented procedures use direct provider requests. |
| `provider_credentials` | Required metadata for each direct provider used by any configured procedure object. |

| Procedure object | Fields and defaults |
| --- | --- |
| `arb` | `core_command` defaults to `aar`, while `core_working_directory` is optional.  `mcp_command` defaults to `aar-mcp`, and `mcp_working_directory` defaults to the core working directory.  The object accepts plaintiff and defendant profiles, defaults `auto_lawyers` to `both` and `web_search` to true, and accepts optional MCP address, prompt, and timeout fields. |
| `arbd` | `core_command` defaults to `aard`, while `core_working_directory` is optional.  `mcp_command` defaults to `aard-mcp`, and `mcp_working_directory` defaults to the core working directory.  The object requires `judgment_standard`, accepts plaintiff and defendant profiles, defaults `auto_lawyers` to `both` and `web_search` to true, and accepts optional MCP address, prompt, and timeout fields. |
| `adc` | `core_command` defaults to `adc`, while `core_working_directory` is optional.  `mcp_command` defaults to `adc-mcp`, and `mcp_working_directory` defaults to the core working directory.  The object accepts plaintiff and defendant profiles and defaults `trial_mode` to `auto`, `auto_lawyers` to `both`, and `web_search` to true.  It also accepts optional MCP address, prompt, and timeout fields. |
| `simple` | `core_command` defaults to `simple`, while `core_working_directory` is optional.  The object requires `model`, defaults `web_search` to true, and accepts optional `reasoning_effort`, `max_output_tokens`, and `max_tool_calls`.  It also accepts optional `prompt_dir`, `prompt_files`, and `timeout`. |
| `quick` | `core_command` defaults to `quick`, while `core_working_directory` is optional.  `mcp_command` defaults to `quick-mcp`, and `mcp_working_directory` defaults to the core working directory.  The object accepts plaintiff and defendant profiles, defaults `auto_lawyers` to `both`, `web_search` to true, and `parallel_council` to false, and accepts optional MCP address, prompt, and timeout fields. |

Quick starts one MCP child for each case.  The controller creates a per-run signing key and issues separate bearer capabilities bound to the case and the plaintiff or defendant assignment.  Both lawyers connect to the same `/mcp` URL, while their capabilities supply the identity used by the adapter.

For Quick, AAR, AARD, and ADC, `auto_lawyers` accepts `both`, `plaintiff`, `defendant`, or `none`.  Automatic roles require a compatible profile, while every other role receives a generated remote-lawyer skill containing its MCP capability.  `mcp_listen` selects the adapter listen address, and `mcp_public_base_url` supplies the externally reachable base URL inserted into remote skills; a public URL is required when a wildcard listener serves a remote lawyer.

A caller using a manual lawyer should supply `--out-dir` because `adjudicate` writes its JSON result only after the run finishes.  After the MCP adapter becomes ready, the launcher creates `<OUT>/openclaw-plaintiff-lawyer-skill.md` or `<OUT>/openclaw-defendant-lawyer-skill.md` with mode `0600`, and the run waits for the manual participant to complete its assigned opportunities.  Each skill contains a role-bound bearer capability, remains usable while that run's MCP adapter is active, and is removed when `adjudicate` returns after success, failure, or cancellation.

The five `web_search` settings resolve independently.  Omitting the field or setting it to `true` enables search for the Simple model or the lawyers used by Quick, AAR, AARD, or ADC.  An explicit `false` disables local lawyer search and tells an externally supplied MCP lawyer to avoid web search.  Council members and jurors remain offline.

Durations use Go duration strings such as `90s` or `15m`.  A configured timeout must be positive, and procedure adapters that pass integer-second flags require a whole number of seconds.  ADC accepts `auto`, `jury`, or `bench`.  `auto` uses the proposition procedure's jury recommendation.

## Document Import

`--documents DIR` imports every regular file beneath `DIR` recursively.  The importer preserves each path relative to `DIR`, includes hidden regular files, and sorts paths by their byte representation before writing the manifest.  It copies the exact file bytes into the case input area and records each relative path, byte count, media type when known, and SHA-256 hash.

The importer rejects an unreadable entry, symbolic link, special file, path escape, duplicate relative path, per-file limit violation, or total-size limit violation.  Validation and copying finish before procedure startup, so every participant reads the staged bytes identified by the manifest.  An empty directory produces an empty document manifest.

Availability means registration in the procedure's document or evidence store.  `arbd`, `arb`, and `adc` receive every validated staged document path as a native case file, and quick exposes immutable documents through its lawyer API before including verified contents in council prompts.  Simple and quick encode UTF-8 text, `image/*`, and PDF documents and reject another media type before a provider request.  The request-spec metadata does not describe input modalities, so a provider or model can reject an encoded image or PDF through the normal provider error path.

## Procedure Specifications

The procedure registry assigns each identifier a runner and capability set.  Settings validation resolves the procedure's command, working directory, participant profiles, policy, and provider sources before dispatch.  Each adapter converts the common proposition and documents into native inputs and maps the native result into the common result.

| Procedure | Core runner | Input conversion | Participants | Record verification | Decision form |
| --- | --- | --- | --- | --- | --- |
| `arbd` | `aard case` | Question complaint plus imported case files | Automatic or caller-owned MCP lawyers with search available by default, plus offline Pi council agents | Lean replay certificate | Stable JSON object of council scores from 0 through 100 |
| `arb` | `aar case` | Proposition complaint plus imported evidence | Automatic or caller-owned MCP lawyers with search available by default, plus offline Pi council agents | Lean replay certificate | `demonstrated`, `not_demonstrated`, or `no_majority` |
| `adc` | `adc case` | Declaratory proposition case plus imported case material | Automatic or caller-owned MCP lawyers with search available by default, court roles, and offline optional jurors | Lean replay certificate | `demonstrated`, `not_demonstrated`, or `no_decision` |
| `simple` | Single-model core runner | Proposition and provider-supported document content | Single model request with provider-hosted search available by default | Recorded request, response, search activity, settings, and document hashes | Binary decision and rationale |
| `quick` | `quick case` | Proposition plus imported documents | One proponent argument and one opponent argument with search available by default, followed by offline direct council requests | Recorded arguments, votes, participant search settings, provider metadata, settings, and document hashes | `demonstrated`, `not_demonstrated`, or `no_majority` |

The required common capabilities are settings validation, document import, typed failures, terminal JSON output, a durable result, and event recording.  Participant APIs and Lean replay depend on the procedure specification.  The common result reports those capabilities so a caller can select the applicable inspection and verification operations.

## Procedure Behavior

### `arb`

The `arb` adapter writes a canonical proposition complaint, stages the imported documents as initial evidence, and starts an AAR core case.  The common council pool, size, vote threshold, and evidence standard control the council.  Procedure settings select automatic roles, participant profiles, MCP addressing, lawyer search policy, and runtime values specific to AAR, with `common.lawyer_profile` supplying both profile names when they are omitted.

### `arbd`

The `arbd` adapter writes a `# Question` complaint, passes each staged document path as a case file, and starts an AARD core case.  Its plaintiff and defendant profile fields select OpenClaw, Pi, Codex, or Claude for each automatic role, with `common.lawyer_profile` supplying an omitted name.  Unselected roles connect through MCP.  The adapter applies the lawyer search policy and uses the common council pool and size for Pi council agents.  A successful native result must match the requested case and run, report `status: "ok"` and `phase: "closed"`, and contain scores from 0 through 100.

### `adc`

The `adc` adapter passes the proposition to ADC's Proposition Tribunal, which creates a declaratory claim between `Proponent` and `Opponent`.  It supplies the staged document directory, explicit document limits, common evidence standard, selected lawyer profiles, lawyer search policy, and trial mode.  Jury and automatic modes use the common council pool as ADC's juror pool, while bench mode omits the juror pool and its OpenRouter credential.

### `simple`

The `simple` runner makes one provider request containing the proposition, decision instructions, and supported imported material.  Its procedure object requires an `openai://MODEL` or `openrouter://MODEL` model reference without a query or fragment.  When search is enabled, the request makes the current Responses `web_search` tool available through the selected endpoint and its existing credential.  The model decides whether to call it.

The optional `reasoning_effort` field accepts `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, or `max`, subject to support from the selected model.  The optional positive `max_output_tokens` field limits reasoning and visible output together, while the optional positive `max_tool_calls` field limits provider built-in tool calls across the response.  Omission leaves the model's reasoning and tool-call defaults in effect and retains Simple's 4,096-token output default.

The decision values are `demonstrated` and `not_demonstrated`, matching `arb` for direct comparison.  The decision prompt defines the evidence standard, requires a rationale grounded in the supplied material, and requires URLs for relied-on web sources when search is enabled.  Media validation and document-limit checks complete before the provider request.

The native request record contains the effective search setting, tool declaration, requested reasoning effort, output-token limit, and built-in-tool-call limit.  The native response record preserves the raw provider response and normalizes each search action, query, source URL, and URL citation, including its title and text offsets.  Native runtime, state, event, transcript, digest, and terminal records also state the model-request and search settings and summarize search call, source, and citation counts.

### `quick`

The `quick` adapter starts a core case with two sequential lawyer opportunities.  The plaintiff files one argument, the defendant receives that argument and files one response, and neither lawyer receives another turn.  The core samples the configured council pool without replacement, then requests offline votes sequentially by default or concurrently when `parallel_council` is true.

The lawyer HTTP paths and MCP tools match the lawyer subset of `arb`, allowing all four supported agent runners to use the same case-operation pattern.  The core verifies each staged document before a lawyer or council member reads it and checks the selected council endpoints for local credential configuration before opening the first lawyer turn.  Council requests present UTF-8 documents as text, images as image data URLs, and PDFs as file content items; another binary media type fails before the first lawyer turn.  The terminal record contains both arguments, every vote and rationale, the selected council metadata, recovered provider cost, and the resulting majority decision.

| Quick lawyer runner | Enabled search | Disabled search |
| --- | --- | --- |
| Codex | Sets `web_search = "live"`, allows outbound network access in the workspace-write sandbox, and uses OpenAI-hosted Codex search with the selected Codex authentication. | Sets `web_search = "disabled"` while retaining outbound access for local analysis tools. |
| Claude | Loads the default built-in tools, including `WebSearch`, and allows the case MCP tools.  Its sandbox confines writes to the role work directory, protects staged credentials and MCP configuration, and permits Bash to download or install needed tools. | Applies the same local-tool and sandbox configuration and denies only `WebSearch`. |
| OpenClaw | Uses the full tool profile and full execution mode, including files, shell, process, browser, and computer use.  An OpenAI profile enables live hosted Codex search.  An Anthropic profile selects the bundled key-free DuckDuckGo provider and enables its plugin. | Sets `tools.web.search.enabled` to false, deletes retained provider, credential, and native-search fields, and disables the DuckDuckGo plugin.  The full local tool profile remains available. |
| Pi | Loads the case `mcp` proxy, Pi's seven default local-analysis tools, and pinned Pi Web Access with its complete search and content-analysis tool set.  Search tries OpenAI first and key-free Exa MCP after specified failures. | Omits Pi Web Access while retaining `mcp`, `read`, `bash`, `edit`, `write`, `grep`, `find`, and `ls`. |

The lawyer decides whether to use available search and analysis tools.  Pi runs as container root under rootless Podman, while OpenClaw runs as root through its selected Docker command, allowing either participant to install a needed system program during the invocation.  Their retained workspaces preserve user-installed programs and analysis files, and OpenClaw performs a scoped ownership cleanup after either ordinary exit or forced cancellation.  A lawyer that relies on a web source must put its URL and retrieval date in its private work notes and filed argument.  The Quick core receives those filings as case material, while its council receives no search tool.

## Participant Profiles

A named lawyer profile selects OpenClaw, Pi, Codex, or Claude and resolves the runner's command, model, authentication source, and resumption setting.  Automatic `arbd`, `arb`, `adc`, and `quick` lawyers use their role profile or inherit `common.lawyer_profile`.  The settings loader validates profiles only for roles selected by `auto_lawyers` and writes the resolved values to the case record before it starts a participant.

Codex and Claude use subscription credentials from `~/.codex/auth.json` and `~/.claude/.credentials.json` by default.  A Pi profile using the OpenAI provider can use the same Codex subscription file.  API-key authentication requires an explicit profile setting and source environment-variable name.  Claude retains its standard tools in both authentication modes and adds `--no-session-persistence` only when that profile sets `resume: false`.  Lawyer search reuses the selected runner authentication for Codex, Claude, Pi, and an OpenAI OpenClaw profile, while an Anthropic OpenClaw profile uses key-free DuckDuckGo search.  Pi Web Access retains key-free Exa MCP as its second search provider.  A profile never changes from subscription authentication to an ambient API key when its credential file is missing or invalid.

OpenClaw defaults to the OpenAI provider, subscription authentication through `~/.codex/auth.json`, the `gpt-5.5` model, and `resume: false`.  An OpenAI profile accepts an unqualified or `openai/`-qualified model.  An Anthropic profile requires `provider: "anthropic"`, an `anthropic/`-qualified model, and explicit API-key authentication.  Pi, Codex, and Claude default to `resume: true`.  Pi requires a valid provider-qualified model, while Codex and Claude may use the runner's model default.  Only Codex and Claude accept a custom `command` field because the `adj` launchers run OpenClaw and Pi in containers.

Quick, AAR, AARD, and ADC read the optional profile field `reasoning_effort`, which accepts `low`, `medium`, `high`, or `xhigh`.  Codex receives the value as `model_reasoning_effort`, Pi receives `--thinking`, Claude receives `--effort`, and OpenClaw receives `--thinking`.  An omitted value leaves the Codex, Pi, or Claude runner default in effect and preserves OpenClaw's `low` default.

Pi, Codex, and Claude keep role-specific session state under `<agent_state_root>/<case_id>/<procedure>/<profile>/<role>/<runner>`.  OpenClaw keeps its internal state inside its disposable container for the process lifetime, including retries and every lawyer opportunity, while its mounted work directory remains available after exit.  Each run keeps its working directory under the run record, and an exclusive procedure-session lease rejects concurrent use of retained state.  Cleanup removes staged credentials, MCP configuration, assignment capabilities, and secret environment entries while preserving retained participant state, workspace files, and the management record paths needed to locate them.  Process exit, log closure, cleanup, role verification, usage inspection, state measurement, and participant recording must finish before a success marker remains available for resumption.  Cancellation or a finalization failure removes a marker created earlier in that sequence.

Credential values remain outside the settings file.  For local runs, the repository ignores `tmp/`, so an operator can place exported credential variables in `tmp/adjudicate.env`, restrict that file to its owner, and load it before starting the command.  The `environment_variable` fields in settings name those loaded variables.

```bash
chmod 600 tmp/adjudicate.env
. ./tmp/adjudicate.env
.bin/adjudicate --proc simple --proposition "The sky is blue" --settings settings.json
```

## Result Interface

After command-line parsing supplies a complete request and identifiers, the command writes one common result envelope for success and failure.  Help, flag-parsing errors, missing required flags, and identifier-generation failures use standard error because no complete request exists.  The `decision` field provides the closest common interpretation, while `procedure_result` preserves the complete native core result.  Callers use the tagged `decision.kind` before interpreting its value; `arbd` uses `council_answers` and stores the stable JSON answer map in `decision.value`.

```json
{
  "schema_version": "adjudicate.result.v1",
  "procedure": "arb",
  "case_id": "case-1",
  "run_id": "run-1",
  "started_at": "2026-08-18T12:00:00Z",
  "finished_at": "2026-08-18T12:15:00Z",
  "status": "ok",
  "phase": "closed",
  "decision": {
    "kind": "binary",
    "value": "demonstrated",
    "rationale": "The admitted record supports the proposition."
  },
  "capabilities": {
    "documents": true,
    "participant_api": true,
    "lean_replay": true,
    "sessions": true,
    "council": true
  },
  "procedure_result": {},
  "record_dir": "out/case-1/run-1",
  "management": {
    "updated_at": "2026-08-18T12:15:00Z",
    "processes": [
      {
        "name": "adjudicate",
        "kind": "controller",
        "pid": 3001,
        "instance_id": "7cfda0a8-47ed-4a75-8699-bd6ac27be0b5:9428704",
        "state": "completed",
        "started_at": "2026-08-18T12:00:00Z",
        "finished_at": "2026-08-18T12:15:00Z"
      }
    ],
    "participants": [
      {
        "role": "plaintiff",
        "profile": "codex-lawyers",
        "runner": "codex",
        "reasoning_effort": "xhigh",
        "resumed": true,
        "state_dir": "/srv/adjudication/agents/case-1/arb/codex-lawyers/plaintiff/codex",
		"work_dir": "/srv/adjudication/runs/case-1/work/plaintiff",
        "state_bytes": 8192,
        "usage": {
          "input_tokens": 900,
          "cached_input_tokens": 600,
          "output_tokens": 100,
          "total_tokens": 1000
        }
      }
    ],
    "provider": {
      "request_count": 3,
      "usage_observed_count": 2,
      "usage": {
        "input_tokens": 1200,
        "output_tokens": 400,
        "total_tokens": 1600
      },
      "cost_observed_count": 1,
      "cost_usd": 0.00042
    }
  }
}
```

`status` is `running` while `adjudicate` owns an active invocation, `ok` for a completed procedure, `failed` for a terminal procedural failure recorded by the core, and `error` for a process, configuration, input, storage, or provider failure that prevents a valid terminal case.  `failure` carries a structured procedural failure, while `error_class` carries a stable machine-readable process or provider class.  The common envelope preserves the complete native result under `procedure_result`.

`management.processes` records the controller, core, adapters, lawyers, and council or juror processes owned by `adjudicate`, including the operating-system process-instance identity where available.  On Linux, that identity combines the kernel boot ID with the process start ticks from `/proc/<pid>/stat`, so the same PID and tick count after a reboot cannot match an older record.  `management.participants` records each lawyer's role, selected profile and runner, effective reasoning effort, resumption status, work directory, structured token usage, and effective search setting.  It also records the retained-state directory and size for a runner that retains session state.  An enabled participant search object names its mechanism and ordered provider list.  A disabled object contains `enabled: false` without those names.  A participant-output parse or retained-state measurement error appears on the participant and in `management.errors`, and also fails that managed process.

A settled provider refusal appears in `management.participants[].refusal`.  The object records the provider, model, available stop reasons, retry status, optional provider category and explanation, participant session ID, provider request and response IDs, refused-message ID, relative stdout artifact, and any retained session path.  The stdout artifact preserves the runner output, while a retained session artifact can preserve preceding prompts, tool calls, tool results, and model messages without copying that content into `run.json`.  The controller reports `error_class: "provider_refusal"`, removes the invocation's success marker, and ends the run before any later procedure stage begins.

Pi supplies a refusal through the final `agent_end` event confirmed by `agent_settled`; Claude Code supplies `system.model_refusal_no_fallback` and a terminal refusal result; and OpenClaw supplies `meta.completion.refusal: true`.  The controller uses those structured signals and does not infer a refusal from response prose.  Codex `turn.failed` currently supplies an error message without a stable refusal code, so the controller records that event as an ordinary terminal failure.  A failed headless invocation lacks the success marker required for later session resumption, while a new OpenClaw run receives a run-specific session key.

The lawyer search labels are `native` with `providers: ["openai"]` for Codex, `native` with `providers: ["anthropic"]` for direct Claude, `gateway` with `providers: ["openrouter"]` for Claude through OpenRouter, and `pi_web_access` with `providers: ["openai", "exa"]` for Pi.  An OpenAI OpenClaw profile records `native` with `providers: ["openai"]`, while an Anthropic OpenClaw profile records `managed` with `providers: ["duckduckgo"]`.  These fields record the available mechanism and routing order.  The runner's raw standard streams retain the search events it emits and establish whether it used that mechanism.

Codex usage comes from `turn.completed` events, Claude usage comes from the terminal `result` event, and Pi usage comes from assistant `message_end` events.  A resumed Codex event contains a cumulative session counter, so the controller stores the preceding counter in retained state and records its componentwise delta for the current run.  Cache fields preserve each runner's schema, and `total_tokens` follows that schema.  Codex cached input is a subset of input, while Claude and Pi report cache categories separately.  OpenClaw retains its workspace but does not expose structured participant usage through this launcher.

The `provider` object counts logical procedure-provider requests and the subset that supplied usage or cost.  Each reported sum covers only the observed subset.  An absent usage or cost value means that no request supplied it, while a numeric zero means that the provider reported zero.  Participant usage and procedure-provider accounting describe separate model calls and remain separate in the record.

## Durable Record

The run output directory contains common request and management records beside the selected core procedure's record.  `adjudicate` preserves native core files beneath `core/`.  The directory separates staged input, native output, process logs, participant work, events, and the common result.

| Path | Contents |
| --- | --- |
| `inputs/adjudicate-request.json` | Canonical request after command-line parsing. |
| `inputs/resolved-settings.json` | Complete selected settings after default resolution. |
| `inputs/documents.json` | Imported document paths, sizes, media types, and hashes. |
| `inputs/documents/` | Staged document bytes. |
| `run.json` | Live common result and management record, replaced atomically through terminal publication. |
| `events.ndjson` | Common dispatch, import, startup, and terminal events. |
| `core/` | Complete native record written by the selected core procedure. |
| `logs/` | Captured core and participant standard streams. |
| `work/ROLE/` | Per-run participant working directories. |

The native `arbd`, `arb`, and `adc` records retain their state, certificate, evidence, transcript, digest, and work-note files beneath `core/`.  Simple writes its imported documents, request, raw and normalized response, decision, state, transcript, digest, events, and run result there.  Quick writes its imported documents, runtime record, ordered events, private lawyer notes, arguments, council votes, and run result there.  Participant standard streams under `logs/` preserve each runner's emitted search events, while relied-on source URLs and retrieval dates belong in Quick work notes and arguments.  A result is terminal after `adjudicate` reconciles the native record and writes the common `run.json` atomically.

## Case Record Index

`adjudicate case-record` reads a retained case and writes one JSON index.  The input directory must contain exactly one case manifest in a supported location:

| Record layout | Manifest path below `RUN_DIR` |
| --- | --- |
| Direct procedure output | `case-manifest.json` |
| Unified `adjudicate` output | `core/case-manifest.json` |
| `aar-run` output | `aar-output/case-manifest.json` |
| `aard-run` output | `aard-output/case-manifest.json` |
| `adc-run` output | `adc-output/case-manifest.json` |

```text
.bin/adjudicate case-record --dir RUN_DIR
.bin/adjudicate case-record --dir RUN_DIR --output ../case-record.json
.bin/adjudicate case-record --dir RUN_DIR --publish-dir ../published-case
.bin/adjudicate case-record --dir RUN_DIR --include-work-notes
.bin/adjudicate case-record --dir RUN_DIR --include-sessions
```

The default command writes JSON to standard output.  `--output` creates a new file outside the case directory.  Its parent directory must exist, and the destination path must be unused.  `--publish-dir` creates a portable directory containing the index and every selected artifact:

```text
published-case/
  case-record.json
  files/
    record/
      ...
    session-1/
      ...
```

The publication parent must exist.  The target must be unused and outside every source root.  `--output` and `--publish-dir` are mutually exclusive.  Publication writes no JSON to standard output.

The command writes `case-record.json` after it copies every artifact.  A copy error leaves the incomplete target directory without a completed index.  The caller can inspect or remove that directory before choosing another target.

| Selection | Added material |
| --- | --- |
| Default | Artifacts classified as `case_record` and their logical docket entries. |
| `--include-work-notes` | Procedure work-note files and one `work_note` docket entry for each note.  The entry references the source line; the note text remains in the work-note file. |
| `--include-sessions` | Process logs, formal-launcher participant state under `agents/`, and retained participant state directories listed in the unified `run.json`. |
| Both flags | Work notes, process logs, and retained participant sessions. |
| Outside index scope | Participant work directories, exported work product, and ADC strategy files. |

The two inclusion flags are independent.

The top-level object uses schema `adj.case-record.v1` and contains the following fields:

| Field | Contents |
| --- | --- |
| `schema_version` | `adj.case-record.v1`. |
| `generated_at` | UTC time when the command read the case. |
| `procedure`, `case_id`, `run_id` | Identity from the core case manifest. |
| `sources` | The case directory and each requested retained session root, with availability and role information. |
| `docket` | Logical case documents in chronological order. |
| `artifacts` | Regular files in chronological order. |

Each source contains these fields:

| Field | Contents |
| --- | --- |
| `id` | `record` for the case directory or `session-N` for an external retained-session root. |
| `kind` | `record` or `session`. |
| `path` | Source-root path. |
| `path_base` | `absolute` when the source path is absolute or `index` when the path is relative to `case-record.json`. |
| `role` | Participant role for a retained session, when recorded. |
| `available` | Whether the source root was available when the command read it. |
| `error` | `not found` or `not a regular directory` for an unavailable requested session root. |

Direct standard-output and `--output` indexes use absolute source paths.  Published indexes use `files/record` and `files/session-N`, relative to `case-record.json`.

The command records a missing or unusable external session root in `sources` and continues indexing the available record.  Other filesystem errors and malformed required records fail the command.  A published index preserves an unavailable source entry, although its `files/session-N` directory will be absent.

Each docket item contains these fields:

| Field | Contents |
| --- | --- |
| `id` | Stable identifier within the case index. |
| `sequence` | One-based position after chronological sorting. |
| `timestamp`, `timestamp_source` | UTC chronological time and its source. |
| `phase`, `actor` | Procedure phase and participant when recorded. |
| `kind`, `title` | Entry type and concise label. |
| `description` | Procedure description when recorded. |
| `access` | `case_record` or `work_notes`. |
| `source` | Source location for the logical entry. |
| `artifact_refs` | Physical files represented by the entry, when available. |

Common entries cover complaints, input documents, and evidence.  The procedure-specific entries are:

| Procedure | Additional docket entries |
| --- | --- |
| Simple | Decision. |
| Quick | Arguments, council votes, council failures, and decision. |
| ARB | Filings, technical reports, evidence offers, submitted evidence, council votes, and decision. |
| ARBD | Filings, technical reports, evidence offers, submitted evidence, council answers, and degree decision. |
| ADC | Entries from the ADC case docket. |

A source location contains `source_id` and a path relative to that source root.  It also contains `json_pointer` for an entry inside a JSON document or `line` for an NDJSON entry when applicable.

All timestamps use RFC 3339 UTC form.  `timestamp_source` has one of four values:

| Value | Meaning |
| --- | --- |
| `case_manifest` | Case start time from `case-manifest.json`. |
| `record` | Time stored with the item in a procedure record. |
| `event` | Time stored on the corresponding procedure event. |
| `file_modification` | Modification time of the source file. |

Initial complaints and documents use the case start time.  Recorded item and event times supply later entries when present.  File modification time supplies a logical entry whose record has no item time.  The case start time supplies the final fallback when the corresponding file metadata is unavailable.

Each artifact contains these fields:

| Field | Contents |
| --- | --- |
| `source_id`, `path` | Source root and path relative to that root. |
| `media_type`, `size_bytes` | Media type from the filename or an existing manifest, and file size. |
| `category` | `case_manifest`, `complaint`, `state`, `certificate`, `transcript`, `digest`, `event_log`, `decision`, `model_request`, `model_response`, `run_record`, `council_record`, `evidence_manifest`, `case_input`, `configuration`, `evidence`, `work_notes`, `process_log`, `participant_session`, or `unclassified`. |
| `access` | `case_record`, `work_notes`, or `sessions`. |
| `timestamp`, `timestamp_source` | File modification time and `file_modification`. |
| `recorded_sha256` | Hash copied from an existing document or evidence manifest, when present. |

Go's portable file information supplies modification time rather than creation time.  The artifact walk catalogs regular files and skips symbolic links.

The publication contains the access classes selected by the command flags.  Work notes contain private participant analysis.  Session material can contain provider credentials when participant cleanup did not complete.  A server exposing either class needs access controls suitable for those records.

## Process Behavior

After constructing a complete request, the command writes one JSON result to standard output and diagnostics to standard error.  Exit status zero means that `run.json` contains a complete terminal procedure record, including a recorded procedural failure.  A nonzero exit status reports configuration, input, startup, provider, storage, supervision, or reconciliation failure; engine construction and dispatch failures still produce the common result on standard output.

Interrupt cancellation reaches the core process and every participant started by `adjudicate`.  The supervisor waits for process termination, returns shutdown errors, and records the last complete state available from the core.  It accepts a terminal `run.json` only when the current process replaces that file.

## Adding a Procedure

A new procedure supplies one identifier, core runner, settings specification, input adapter, result adapter, capability declaration, participant launchers where needed, durable-record description, and tests.  The core runner owns one case and follows the common process behavior.  An optional managed service can invoke the published `adj` command without importing the procedure implementation.

The tests exercise settings rejection, input conversion, document import, core startup, participant traffic when present, terminal result reconciliation, process failure, cancellation, and record inspection.  Service integration tests can select exact installed command paths when a managed service adopts the procedure.  A procedure joins the public `--proc` values after its one-case execution path passes those tests.

## ADC Proposition Resolution

ADC's Proposition Tribunal creates one declaratory claim with `Proponent` bearing the burden and `Opponent` opposing it.  A judgment for `Proponent` produces `demonstrated`, a judgment for `Opponent` produces `not_demonstrated`, and a termination without a merits decision produces `no_decision`.  The common result uses `decision.kind: "binary"` for the proposition axis and preserves the third procedural outcome in `decision.value`.

The ADC core records the resolution in its final case state and exposes it through the Role API.  The `adjudicate` result adapter accepts only `judgment_entered` or `closed` case status with one of the three terminal resolutions.  The native ADC result remains available under `procedure_result` for the judgment, docket, evidence, transcript, and replay data.
