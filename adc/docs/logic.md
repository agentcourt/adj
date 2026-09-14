# Procedure Execution

ADC divides formal procedure from runtime work.  [`engine/ADC/Core.lean`](../engine/ADC/Core.lean) defines the court state, request and action types, validators, role views, opportunity generation, decision authorization, and state transitions.  [`engine/Main.lean`](../engine/Main.lean) implements the line-oriented JSON protocol and executable entry point.  The Go runtime owns model and Role API interaction, support operations, deadlines, attempt limits, file storage, event recording, and output publication.

## Lean requests

| Request | Result |
| --- | --- |
| `initialize_case` | Validates the complaint inputs and constructs the initial court state. |
| `role_view` | Returns the portion of the state visible to one role. |
| `next_opportunity` | Returns the lowest-priority open opportunity or a terminal result. |
| `agenda` | Returns every open opportunity after pass filtering. |
| `apply_decision` | Validates one pass or legal-tool decision against the current opportunity. |
| `step` | Applies one authorized or deterministic `CourtAction` to the supplied state. |

Opportunity generation constructs the complete agenda, assigns the positional identifiers `o1`, `o2`, and so on, and then removes passed opportunities.  Each opportunity states its role, phase, priority, pass behavior, allowed tools, payload constraints, and optional deterministic action.  The identifiers remain stable when pass filtering removes an earlier entry from the same agenda.

`apply_decision` checks the supplied state version, opportunity identifier, role, decision kind, tool name, and fixed payload constraints.  A pass changes state according to the opportunity's `pass_effect`.  A tool decision returns the exact `CourtAction` that the runtime may execute.  `step` checks that action against the state and returns either an updated state or a structured rejection.  A docket entry separates its display description from typed fields.  Procedural matching and limits use the typed fields for parties, discovery generations, and motion indexes.

## Runtime execution

The runner retains the authoritative state and requests one opportunity at a time.  For a direct role, it builds the role prompt and accepts support-tool calls until the role submits a decision.  For an external role, the Role API exposes the same visible state, opportunity, support operations, and legal-tool schemas.  The runner sends each submitted decision to `apply_decision` before changing state.

Support operations read role-visible case data, store work notes, or manage case files.  Legal actions enter the Lean transition path.  An invalid legal decision leaves the opportunity open while its attempt allowance remains.  An accepted pass updates the state returned by `apply_decision`.  An accepted tool decision produces a `CourtAction`, which the runner executes through `step`.

## Replay

The `adc.replay-certificate.v1` record distinguishes deterministic steps from opportunity decisions.  A deterministic transition stores its `CourtAction`.  An opportunity pass stores the complete `apply_decision` request and no executed step.  An opportunity tool transition stores that request and the exact executed step.  Replay calls `apply_decision`, requires the returned action to equal the recorded step, and then applies the step.  The verifier compares the replayed state with the certificate claim and `state.json`.

The maintained proof tree imports `ADC.Core` rather than the JSON executable.  Its theorems cover validation, opportunity and role authority, state preservation, replay reachability, and selected terminal consequences.  Evidence truth, legal reasoning quality, external execution attestation, and file-byte custody require evidence outside the Lean transition proof.

The [short request-loop diagram](../analysis/lean-simple-flow.md) and [complete control-flow diagram](../analysis/lean-complete-flow.md) show these boundaries.
