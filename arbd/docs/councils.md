# Council Constitution

`arbd` constitutes its deciding body during `aard case`.  The procedure uses a council, and the code uses that term in state, events, prompts, and final run artifacts.  Constitution begins after the complaint and policy have been loaded, and it finishes before the Lean engine opens the first lawyer turn.

The Go runtime draws council members from a pool file, assigns seat ids, and sends that exact list to the Lean engine.  The engine checks the council length against `policy.council_size`, requires unique member ids, and rewrites each incoming member to `seated`.  Once initialization succeeds, the event stream and final packet record the constituted council.

| Stage | Code path | Effect |
|---|---|---|
| Command configuration | [`runtime/cmd/aard/case.go`](../runtime/cmd/aard/case.go) | Parses `--council-size`, `--council-pool`, and `--council-backend`, then builds proceeding options. |
| Policy validation | [`runtime/proceeding/policy.go`](../runtime/proceeding/policy.go) | Requires a positive council size and a non-empty judgment standard. |
| Pool loading | [`common/persona/persona.go`](../../common/persona/persona.go) | Reads pool records, resolves persona files, and loads persona text. |
| Sampling | [`runtime/proceeding/council_preflight.go`](../runtime/proceeding/council_preflight.go) | Draws records without replacement and assigns `C1`, `C2`, and later seat ids. |
| Engine initialization | [`engine/Main.lean`](../engine/Main.lean) | Checks the council, stores seated members, and opens the case. |
| Recording | [`runtime/proceeding/run.go`](../runtime/proceeding/run.go) and [`runtime/proceeding/render.go`](../runtime/proceeding/render.go) | Writes initialization events and final run artifacts. |

## Pool File

The pool file comes from `--council-pool` when the caller supplies it.  Otherwise `arbd` checks for a local `pool.jsonl`, then uses the shared default under `common`.  Each usable line must be a JSON request-spec record with model, provider, optional quantization, request settings, and persona information.

Each usable line becomes one sampleable record.  A JSON record supplies provider, model, optional quantization, persona file, and request settings through the shared `modelrequest` parser.  The persona loader resolves the persona file relative to the pool file, reads the persona text immediately, and rejects empty persona text.

## Sampling

The sampler shuffles the usable pool records with `crypto/rand` and draws without replacement.  The first selected record becomes `C1`, the second becomes `C2`, and the sequence continues until the runtime has drawn `policy.council_size` records.  Each selected seat carries public metadata, private persona text, and the full model request specification used by the direct runtime.

`council-backend=direct` checks selected council models before seating them.  `council-backend=councilapi` makes no model call during startup because external council clients connect through the Council API.  Both paths validate the pool size before the engine sees the case.

## Lean State

After sampling, the runtime converts the drawn seats into Lean input and calls `initialize_case`.  Each mapped member includes `member_id`, `model`, `persona_filename`, and `status`, with `status` set to `seated` before the request.  The Lean initializer checks the council and resets the deliberation round, answer list, case status, case phase, and failure fields for the new case.

The Lean state is the source of truth after initialization.  The Go runtime may have sampled and labeled the seats, but lawyer turns, council turns, answer recording, member failure, and closure all operate against the initialized state.  The final packet therefore includes both the sampled public council metadata and the final member statuses from Lean.

## Answer Order And Failure

When the case reaches deliberation, the Lean engine chooses the first seated member who has not yet answered in the current round.  The underlying list order is the original sampling order, so the first round calls `C1`, then `C2`, then `C3`, and so on.  AARD closes after one complete round of seated-member answers and returns the member answer map without aggregation.

Council member failure is handled inside the case state.  A Council API client can report failure for the active member, and the engine marks that member `failed` while deliberation continues with the remaining seated members.  Lawyer failure has different consequences: the engine marks the case `failed` with a detailed reason, and the process reports that failure through the case result.
