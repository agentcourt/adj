# Rule 58 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `enter_judgment`.  The fixture set contains 16 eligible judgment-entry postures across jury verdicts and bench opinions.  The eval uses real ADC state, the real Lean opportunity, the production judge prompt or an eval-local prompt candidate, deterministic scoring, opportunity validation, and a Lean step that records the judgment state.

The scorer checks both payload correctness and the post-step case state.  It requires the correct claim id and basis, rejects prohibited basis concepts, then executes the accepted action and verifies `status=judgment_entered` and the expected `monetary_judgment`.  This structure matches Rule 58 in the engine, where the payload identifies the claim and basis while the amount comes from the jury verdict or existing bench judgment state.

## Results

| Prompt | Run | Correct | Claim Correct | Basis Correct | Amount Correct | Status Correct | Invalid | Lean Rejected | Step Rejected | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| production | live 16 | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | dry 16 | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 16 | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |

## Findings

Production performed cleanly on the measured Rule 58 set.  It returned valid `enter_judgment` payloads for all 16 live rows, used `claim-1`, used the expected `jury verdict` or `bench verdict` basis, and passed the opportunity constraints.  The executed Lean step entered `judgment_entered` and preserved the expected monetary amount in every row.

Candidate v1 preserved production behavior but did not improve measured results.  It states the judgment-entry boundary more directly, including the instruction not to revisit liability or damages, but the production prompt already handled the fixture set.  The live comparison therefore shows no prompt failure cluster.

The implementation exposed a useful harness distinction.  `apply_decision` validates and normalizes the model's opportunity decision, but it does not execute the returned action.  Rule 58 therefore adds a second Lean `step` call after acceptance so the scorer can verify final status and monetary judgment.

## Recommendation

Do not update the production Rule 58 opportunity prompt from this eval alone.  Production and candidate v1 both scored 16/16 with no invalid payloads, no Lean rejections, and no post-step state mismatches.  The next eval should move to Rule 59 or Rule 60, where the judge must decide whether to disturb an entered judgment rather than enter a mechanically determined judgment.

## Next Work

Future Rule 58 rows should focus on multiple claims, partial judgment, Rule 68 acceptance, default judgment under Rule 55, and unavailable-opportunity states such as pre-verdict or hung-jury postures.  Post-judgment work should continue in Rule 60 and Rule 59, where the judge must decide whether to disturb an entered judgment rather than enter a mechanically determined one.  Rule 58 should keep scoring the post-step state because judgment entry correctness depends on the state transition as well as the payload.
