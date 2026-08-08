# Council Constitution

`arb` constitutes its deciding body during `aar case`.  The procedure uses a council, which is the term the code and the run evidence use throughout.  Council constitution begins after the complaint has been parsed and the case policy has been loaded, and it finishes before the Lean engine accepts the case and opens the first attorney turn.

The runtime draws council members from a pool file, converts the draw into Lean input, and then asks the Lean engine to initialize the case with that exact list.  The engine performs a second set of checks on the incoming council and rewrites each member into a seated state before the case becomes active.  Once initialization succeeds, the constituted council is recorded immediately in the event stream and later in the final evidence.

| Stage | Code path | Effect |
|---|---|---|
| CLI configuration | [case command](../runtime/cmd/aar/case.go) | Loads policy, applies `--council-size` and `--council-pool`, and builds proceeding options. |
| Pool loading | [persona loader](../../common/persona/persona.go) | Reads the pool file, validates model ids, resolves persona files, and loads persona text. |
| Sampling | [proceeding helpers](../runtime/proceeding/helpers.go) | Draws `council_size` records without replacement and assigns seat ids in draw order. |
| Engine initialization | [Lean engine](../engine/Main.lean) | Requires exact council length, requires unique member ids, rewrites all members to `seated`, and opens the case. |
| Recording | [main proceeding](../runtime/proceeding/run.go) and [renderer](../runtime/proceeding/render.go) | Writes the constituted council into the initialization event and the final run evidence. |
| Deliberation order | [Lean engine](../engine/Main.lean) | Selects the first seated member who has not yet voted in the current round. |

## Entry Point

`aar case` loads the complaint, resolves the shared `common` tree, loads the arbitration policy, and applies the explicit CLI overrides before it touches the council pool.  The policy controls council size and the decision threshold, so those values must be fixed before sampling begins.  The default policy in [the proceeding policy layer](../runtime/proceeding/policy.go) and [the repository policy file](../etc/policy.json) sets `council_size` to `5` and `required_votes_for_decision` to `3`.

The case command accepts `--council-size` as a direct override, and then validates the resulting policy before it starts the run.  [Policy validation](../runtime/proceeding/policy.go) requires a positive council size, a positive threshold, a threshold no greater than the council size, and a strict-majority relation `2 * required_votes_for_decision > council_size`.  That validation determines the shape of the deciding body before the runtime reads a single council record from the pool.

## Pool File

The pool file comes from `--council-pool` when the caller supplies it.  Otherwise the CLI uses `./pool.jsonl` when that file exists, falling back to `<common-root>/data/personas/pool.jsonl`.  The file is newline-delimited JSON, and each usable line must be a request spec with `openrouter_model_id` or explicit `endpoint` and `model`, provider routing fields such as `endpoint_tag` and `quantization` when available, and a persona path.

Each usable line becomes one independent sampleable record.  The loader resolves the persona filename relative to the pool file, reads the persona text immediately, and requires that text to be non-empty.  If the pool file repeats a line, the runtime treats each repeated line as a separate entry in the sampling pool, because the loader preserves record multiplicity rather than collapsing identical records.

## Sampling

[The council sampler](../runtime/proceeding/helpers.go) receives the parsed pool and the already-validated `council_size`.  It requires `council_size <= len(specs)`, builds an index list over the pool records, and draws from that index list with `crypto/rand`.  Each successful draw removes one index from the remaining set, so sampling proceeds without replacement across the pool records for that run.

The draw order determines the seat ids.  The first sampled record becomes `C1`, the second becomes `C2`, and the sequence continues until the runtime has drawn `council_size` records.  Each drawn seat carries four runtime values at this stage: the synthetic `member_id`, the selected model id, the persona filename, and the loaded persona text.

The runtime keeps the persona text for prompting, but the public council metadata carries only the seat id, model id, and persona filename.  That separation appears directly in [the council seat type](../runtime/proceeding/types.go), where `PersonaText` is excluded from JSON output.  The draw therefore produces both the public description of the council and the private prompt material that the council runtime will later feed to each model.

## Lean Initialization

After sampling, the runtime converts the drawn seats into Lean input with [the council mapper](../runtime/proceeding/helpers.go).  Each mapped entry includes `member_id`, `model`, `persona_filename`, and `status`, with `status` set to `seated` before the request is sent.  The Go bridge in [the Lean engine wrapper](../runtime/lean/engine.go) packages that list into an `initialize_case` request together with the proposition and the current policy state.

[The Lean initializer](../engine/Main.lean) then performs its own constitution checks.  It requires a non-empty council, requires the incoming list length to match `policy.council_size`, and requires unique `member_id` values.  When those checks pass, it rewrites every incoming member to `status := "seated"`, resets `deliberation_round` to `1`, clears `council_votes`, sets case status to `active`, and moves case phase to `openings`.

That second initialization step fixes the authoritative council state inside the engine.  The Go runtime may have sampled and labeled the seats, but the Lean state becomes the source of truth for who is seated and which round is current.  From that point forward, attorney turns, council turns, vote recording, and removal all operate against the initialized Lean state rather than against the original pool file or the pre-init draw structure.

## Recording

The constituted council is recorded as soon as initialization succeeds.  [The main proceeding](../runtime/proceeding/run.go) appends a `run_initialized` event that includes the complaint, the evidence standard, the attorney model configuration, and the full council list.  That event is the first durable record of which members were seated in that run.

The same council list appears again in the completion evidence written by [the renderer](../runtime/proceeding/render.go).  `run.json` carries the council in the top-level `council` field, and `council.json` writes the same list on its own.  Those evidence preserve the constituted body exactly as it existed at initialization, which means the council metadata is not reconstructed later from the vote log.

## Vote Order

When the case reaches deliberation, the Lean engine chooses the next voting member from the initialized `council_members` list.  [The selection function](../engine/Main.lean) filters that list to members whose status remains `seated` and then chooses the first seated member who has not yet voted in the current round.  Because the underlying list order is the original sampling order, the first round calls members in the sampled seat order: `C1`, then `C2`, then `C3`, and so on.

The default council backend calls the selected provider directly and gives the model a rendered record plus one function, `submit_council_vote`.  With `--council-backend councilapi`, the runtime waits for an external council member to call the Council API.  The external path preserves the same Lean vote transition and gives the member read-only evidence tools during deliberation: get case, list evidence, stat evidence, bounded byte-range read, and vote submission.  Council members do not receive upload or evidence-submission tools.  The procedural boundary is: lawyers build the record; council members decide from the admitted record.

The procedure now waits for all seated members to vote in a round before resolving that round.  [The deliberation continuation rule](../engine/Main.lean) compares the current-round vote count to the number of seated members, and only then resolves to `demonstrated`, `not_demonstrated`, or `no_majority`, or advances to the next round.  The strict-majority policy check established earlier keeps the two substantive outcomes mutually exclusive within a valid policy.

## Status Changes

The council can shrink during deliberation through explicit status change.  The current runtime path for that change is council opportunity failure in [the council runtime](../runtime/proceeding/council.go), which calls Lean `fail_opportunity` and marks the member `failed`.  Lean permits that transition only during deliberation, only for a known seated member, and only before that member has cast a vote in the current round.

After failure, the member remains present in `council_members`, but the member no longer counts as seated.  Later calls to the next-member selector therefore skip that seat, and later rounds use the smaller seated body.  The runtime records that change as an `opportunity_failed` event and a `council_member_removed` event, so the event stream shows both the originally constituted council and the later status transition.

If a council model keeps returning malformed or invalid votes until it exhausts the runtime invalid-attempt limit, AAR records a council-member opportunity failure.  The failed member is dismissed, and the case continues if the remaining seated council members can still proceed under the policy.
