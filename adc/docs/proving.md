# ADC Proving Notes

## Maintained root

`engine/Proofs.lean` imports every maintained ADC proof module.  Each module reaches `ADC.Core` through its import graph and uses its own namespace below `ADCProofs`.  This structure detects duplicate declaration names and prevents a proof module from depending on the JSON executable in `Main.lean`.

The maintained tree contains symbolic results, finite computed checks, replay examples, and certificate consequences.  `make prove` builds the root for ordinary repository use.  Resource-constrained development environments can run the same Lake target through their Lean process runner.

## Opportunity identity and passes

`availableOpportunities` constructs the complete agenda, finalizes its entries, and assigns the identifiers `o1`, `o2`, and so on before it removes passed opportunities.  The identifiers are distinct within one agenda.  Filtering a passed opportunity therefore preserves the identifiers of the remaining entries from that agenda.

An opportunity declares its pass behavior through `pass_effect`.  A transient pass records the opportunity identifier.  Rule 37 and Rule 56 passes update their party-specific procedural windows.  Voir dire question, for-cause challenge, and peremptory challenge passes update the corresponding selection record.  The pass proofs state these effects separately from executable tool decisions.

## Decision and replay authority

`applyDecision` validates the state version, current opportunity identifier, role, decision kind, allowed tool, and fixed payload constraints.  A pass may return an updated state.  A tool decision returns the exact authorized `CourtAction` without applying it.

Replay certificates use `adc.replay-certificate.v1`.  A recorded opportunity tool decision contains the decision request and the exact executed step.  Replay calls `applyDecision`, compares the authorized action with that step, and applies the step only when they are equal.  A recorded pass must omit an executed step and replays the state returned by `applyDecision`.  Deterministic runtime actions remain direct `step` transitions.

This certificate structure establishes that each recorded opportunity action came from the recorded current opportunity under the recorded role policy.  The certificate still relies on its supplied initial state, role policies, and transition data.  Cryptographic authentication and external evidence custody lie outside the Lean replay checker.

## Further proof work

General certificate theorems derive authority, procedural invariants, and terminal outcome facts from successful certificate checking.  Several current files establish those results for component functions or representative states.  A smaller number lift them through replay.  New proofs should state precise initial-state assumptions and separate general implications from computed examples.

The [proof overview](proofs.md) defines the proof forms and maintenance commands.  The [theorem catalog](theorems.md) and [proof statistics](proofstats.md) describe the current tree.
