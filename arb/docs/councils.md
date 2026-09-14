# Council Constitution

`arb` constitutes its deciding body during `aar case`.  The procedure uses a council, which is the term the code and the run evidence use throughout.  Council constitution begins after the complaint has been parsed and the case policy has been loaded, and it finishes before the Lean engine accepts the case and opens the first attorney turn.

The runtime draws council members from a pool file, converts the draw into Lean input, and then asks the Lean engine to initialize the case with that exact list.  The engine performs a second set of checks on the incoming council and rewrites each member into a seated state before the case becomes active.  Once initialization succeeds, the constituted council is recorded immediately in the event stream and later in the final evidence.

| Stage | Code path | Effect |
|---|---|---|
| CLI configuration | [case command](../runtime/cmd/aar/case.go) | Loads policy, applies the council-size, required-vote, and pool overrides, and builds proceeding options. |
| Pool loading | [persona loader](../../common/persona/persona.go) | Reads the pool file, validates model ids, resolves persona files, and loads persona text. |
| Sampling and preflight | [council preflight](../runtime/proceeding/council_preflight.go) and [shared selector](../../common/councilsample/selector.go) | Balances endpoints and configurations, checks availability, and assigns seat ids. |
| Engine initialization | [Lean engine](../engine/Main.lean) | Requires exact council length, requires unique member ids, rewrites all members to `seated`, and opens the case. |
| Recording | [main proceeding](../runtime/proceeding/run.go) and [renderer](../runtime/proceeding/render.go) | Writes the constituted council into the initialization event and the final run evidence. |
| Deliberation order | [Lean engine](../engine/Main.lean) | Selects the first seated member who has not yet voted in the current round. |

## Entry Point

`aar case` loads the complaint, resolves the shared `common` tree, loads the arbitration policy, and applies the explicit CLI overrides before it touches the council pool.  The policy controls council size and the decision threshold, so those values must be fixed before sampling begins.  The default policy in [the proceeding policy layer](../runtime/proceeding/policy.go) and [the repository policy file](../etc/policy.json) sets `council_size` to `5` and `required_votes_for_decision` to `3`.

The case command accepts `--council-size` and `--required-votes` as direct overrides, and then validates the resulting policy before it starts the run.  [Policy validation](../runtime/proceeding/policy.go) requires a positive council size, a positive threshold, a threshold no greater than the council size, and a strict-majority relation `2 * required_votes_for_decision > council_size`.  That validation determines the shape of the deciding body before the runtime reads a single council record from the pool.

## Pool File

The pool file comes from `--council-pool` when the caller supplies it.  Otherwise the CLI uses `./pool.jsonl` when that file exists, falling back to `<common-root>/data/personas/pool.jsonl`.  The file is newline-delimited JSON, and each usable line must be a request spec with `openrouter_model_id` or explicit `endpoint` and `model`, provider routing fields such as `endpoint_tag` and `quantization` when available, and a persona path.

Each usable line becomes one sampleable record.  The loader resolves the persona filename from the pool directory and then through the shared common tree, reads the persona text, and requires non-empty text.  Repeated lines remain separate entries.  A single entry can also occupy several seats.

The direct council request requires the `tools` parameter for `submit_council_vote`.  When a pool record contains endpoint `supported_parameters`, preflight excludes that candidate if the list omits `tools` and records the replacement cause.  A record without endpoint capability metadata proceeds to the provider availability check.

## Sampling

[Council preflight](../runtime/proceeding/council_preflight.go) passes the pool endpoints, council size, allowed endpoints, and minimum distinct endpoint count to the shared selector.  For each seat, the selector chooses an eligible endpoint with the fewest assigned seats, then a record for that endpoint with the fewest assigned seats.  It breaks ties with cryptographic randomness.  Configurations remain eligible after acceptance, so a council can have more seats than the pool has records.

The runtime checks a selected record's availability before accepting it and caches success for later seats.  A failed record becomes ineligible.  A credential failure excludes its endpoint.  Selection fails if the remaining choices cannot fill the council or satisfy `--minimum-distinct-council-endpoints`.  Repeated `--council-endpoint` flags restrict eligible endpoints.  The [model-endpoint guide](../../docs/model-endpoints.md) defines the shared pool format and credential rules.

The accepted order determines the seat ids.  The first accepted record becomes `C1`, the second becomes `C2`, and the sequence continues until preflight seats `council_size` records.  Each seat carries the synthetic `member_id`, selected model id, request spec, persona filename, and loaded persona text.

The [council seat type](../runtime/proceeding/types.go) serializes the member id, model, persona filename, and request specification.  It keeps loaded persona text in memory for prompting.

## Lean Initialization

After sampling, the runtime converts the drawn seats into Lean input with [the council mapper](../runtime/proceeding/helpers.go).  Each mapped entry includes `member_id`, `model`, `persona_filename`, and `status`, with `status` set to `seated` before the request is sent.  The Go bridge in [the Lean engine wrapper](../runtime/lean/engine.go) packages that list into an `initialize_case` request together with the proposition and the current policy state.

[The Lean initializer](../engine/Main.lean) requires a non-empty council, a list length matching `policy.council_size`, and unique `member_id` values.  When those checks pass, it rewrites every incoming member to `status := "seated"`, resets `deliberation_round` to `1`, clears `council_votes`, sets case status to `active`, and moves case phase to `openings`.

That second initialization step fixes the authoritative council state inside the engine.  The Go runtime may have sampled and labeled the seats, but the Lean state becomes the source of truth for who is seated and which round is current.  From that point forward, attorney turns, council turns, vote recording, and removal all operate against the initialized Lean state rather than against the original pool file or the pre-init draw structure.

## Recording

The constituted council is recorded as soon as initialization succeeds.  [The main proceeding](../runtime/proceeding/run.go) appends a `run_initialized` event that includes the complaint, the evidence standard, the attorney model configuration, and the full council list.  That event is the first durable record of which members were seated in that run.

The [renderer](../runtime/proceeding/render.go) writes the council list to the top-level `council` field in `run.json` and to `council.json`.  Both files preserve the roster selected at initialization.

## Vote Order

When the case reaches deliberation, [the Lean selection function](../engine/Main.lean) chooses the first seated member in `council_members` who has yet to vote in the current round.  The first round therefore calls members in sampled seat order: `C1`, then `C2`, then `C3`, and so on.

The default council backend calls the selected provider directly and gives the model a rendered record plus one function, `submit_council_vote`.  With `--council-backend councilapi`, the runtime waits for an external council member to call the Council API.  The external path preserves the same Lean vote transition and gives the member read-only evidence tools during deliberation: get case, list evidence, stat evidence, bounded byte-range read, and vote submission.  Council members do not receive upload or evidence-submission tools.  The procedural boundary is: lawyers build the record; council members decide from the admitted record.

The procedure waits for all seated members to vote in a round before resolving that round.  [The deliberation continuation rule](../engine/Main.lean) compares the current-round vote count to the number of seated members, and then resolves to `demonstrated`, `not_demonstrated`, or `no_majority`, or advances to the next round.  The strict-majority policy check keeps the two substantive outcomes mutually exclusive within a valid policy.

## Status Changes

The council can shrink during deliberation through explicit status change.  The current runtime path for that change is council opportunity failure in [the council runtime](../runtime/proceeding/council.go), which calls Lean `fail_opportunity` and marks the member `failed`.  Lean permits that transition only during deliberation, only for a known seated member, and only before that member has cast a vote in the current round.

After failure, the member remains present in `council_members`, but the member no longer counts as seated.  Later calls to the next-member selector therefore skip that seat, and later rounds use the smaller seated body.  The runtime records that change as an `opportunity_failed` event and a `council_member_removed` event, so the event stream shows both the originally constituted council and the later status transition.

If a council model keeps returning malformed or invalid votes until it exhausts the runtime invalid-attempt limit, AAR records a council-member opportunity failure.  The failed member is dismissed, and the case continues if the remaining seated council members can still proceed under the policy.
