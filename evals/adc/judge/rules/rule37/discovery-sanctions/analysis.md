# Rule 37 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `decide_rule37_motion`.  The fixture set contains 16 Rule 37 motions across discovery nonresponse, complete response, evasive response, justified objections, overbroad requests, proportionality, harmless cure, disclosure failure, order violation, RFA nonresponse under Rule 36, premature filing, grant without fees, and fee-only requests.  The eval uses real ADC state, the real Lean opportunity, the production judge prompt or an eval-local prompt candidate, deterministic scoring, and Lean application of each returned payload.

The scorer treats sanction fields as part of the decision.  A correct grant-or-denial result is not enough if the payload uses `fees` on a denied motion, omits `sanction_type`, or omits a positive amount for a fee award.  This scoring choice matches Lean's Rule 37 validation and captures failures that would otherwise appear as legally correct reasoning with an unusable tool payload.

## Results

| Prompt | Run | Correct | Grant Correct | Reason Matches | Invalid | False Grants | False Denials | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| production | live 16 | 9 | 9 | 9 | 7 | 0 | 0 | 0.584 |
| candidate-v1 | dry 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 16 | 15 | 15 | 15 | 1 | 0 | 0 | 0.935 |
| candidate-v2 | dry 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v2 | live 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |

## Findings

Production repeatedly denied motions for correct substantive reasons while setting `sanction_type` to `fees`.  Those payloads are invalid because Lean rejects sanctions on denied Rule 37 motions.  The failures clustered on complete response, justified objection, overbreadth, proportionality, harmless cure, premature motion, and work-product objection rows.

The first production run also showed that the original RFA fixture label was wrong.  ARCP Rule 36 says a matter is admitted if no timely answer or objection is served, so a motion to compel RFA responses should be denied rather than granted.  Relabeling that row removed a false-denial result and left the sanction-type defect as the real production failure cluster.

Candidate v1 addressed the main sanction-type defect but still omitted `sanction_type` on the fee-only denial row.  Candidate v2 added a direct payload rule requiring `sanction_type` in every tool call and requiring `none` for every denied motion.  That change removed the final invalid payload without adding false grants, false denials, or sanction mismatches.

## Recommendation

Candidate v2 is the best measured Rule 37 opportunity prompt on this fixture set.  It improves production from 9/16 to 16/16 live and eliminates the invalid denied-motion sanction payloads.  Keep the candidate eval-local until the project makes a separate production prompt update decision, but treat its payload rule as the current best text for Rule 37 prompt iteration.

## Next Work

The next Rule 37 set should test close fee awards, partial grants, mixed justified and unjustified objections, repeated violation of a prior discovery order, and proportional sanctions after late cure.  Those rows would determine whether the prompt still handles sanction limits when the grant decision and the fee decision diverge.  They should also keep the denied-motion payload rule under pressure, because that rule was the main production failure.
