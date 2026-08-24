# Rule 56 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `decide_rule56_motion`.  The fixture set contains 30 Rule 56 motions split evenly across tiers 1, 2, and 3.  The set includes clean grants, clean denials, partial dispositions, credibility disputes, competing inferences, authentication disputes, unsupported damages theories, movant burden failures, and legal bars.

The eval uses real ADC state and the real Lean opportunity.  Fixture text enters the judge's view through docket entries for the motion, opposition, and reply.  The scorer checks the tool payload disposition, deterministic reason tags, invalid response modes, and Lean acceptance.

## Results

| Prompt | Run | Correct | Reason Matches | False Grants | False Denials | Invalid | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|
| production | dry 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| production | live 30 | 29 | 30 | 1 | 0 | 0 | 0.968 |
| candidate-v1 | dry 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 30 | 29 | 29 | 1 | 0 | 0 | 0.960 |
| candidate-v2 | dry 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| candidate-v2 | live 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |

## Findings

Production failed `r56-009`.  The motion sought summary judgment only on consequential lost profits while direct contract damages and liability remained supported.  The model reasoned correctly about the unsupported lost-profit category but returned `granted` instead of `partial`, which overresolved the case relative to the motion's scope.

Candidate v1 fixed `r56-009` by emphasizing motion scope and partial dispositions.  It then failed `r56-027` by granting a partial ruling on a single deposition subpoint.  The full deposition created a genuine dispute on reliance, so the correct disposition was denial rather than a partial grant on an immaterial fragment.

Candidate v2 fixed both observed clusters.  It preserved the rule that damages-only or issue-only motions should not become whole-claim judgments.  It also added a limit on partial rulings: do not grant partial summary judgment on a single document sentence, deposition answer, or subfact when the full record supports a reasonable competing inference on the material issue.

## Recommendation

Candidate v2 is the best measured Rule 56 opportunity prompt on this fixture set.  It improved production from 29/30 to 30/30 live without introducing invalid responses, false denials, or explanation-score loss.  Keep it eval-local until broader judge evals test whether the same scope language interacts well with Rule 12, jury instructions, and for-cause challenge decisions.

## Next Work

The next Rule 56 set should test partial summary judgment across multiple claims, affirmative defenses, damages categories, and issue-only motions.  Paired Rule 12 and Rule 56 themes should also be added to verify that the judge applies different procedural standards to the same factual setting.  Those rows should preserve the current emphasis on false grants, because a wrong grant remains the highest-risk Rule 56 failure.
