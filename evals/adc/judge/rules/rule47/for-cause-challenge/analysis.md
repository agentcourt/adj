# Rule 47 For-Cause Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `decide_juror_for_cause_challenge`.  The fixture set contains 16 pending challenges across fixed bias, follow-law refusal, damages precommitment, digital-evidence refusal, relationship interest, sympathy bias, language or attention limitations, hardship, lawful attitudes, and rehabilitation.  Each row builds a real ADC voir dire state with an answered exchange and a pending challenge, then runs the judge through the Lean opportunity and tool schema.

The eval uses deterministic scoring.  Outcome scoring checks the grant or denial decision and required payload identifiers.  Explanation scoring checks reason tags in `ruling_reason`, and Lean acceptance confirms that the returned payload can be applied to the constructed ADC state.

## Results

| Prompt | Run | Correct | Reason Matches | False Grants | False Denials | Invalid | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| production | live 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | dry 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |

## Findings

Production made no outcome errors on the first for-cause set.  The initial live production summary reported 14 explanation matches because the scorer did not recognize ordinary wording for rehabilitation and lawful preference.  Rescoring fixed those deterministic vocabulary gaps by accepting phrases such as “rehabilitation,” “assurance,” “follow the instructions,” “general preference,” and “not disqualifying.”

Candidate v1 also made no outcome errors.  The candidate prompt states the central for-cause boundary more directly, but the current fixture set does not show an improvement over production.  Its value is as an eval-local reference for future hard rows, especially rows that mix awkward first answers, later assurances, and party attempts to convert lawful skepticism into cause.

## Recommendation

Do not change production opportunity text based on this set.  Production and candidate v1 both scored perfectly after deterministic explanation rescoring, and the candidate did not expose a distinct improvement.  The next Rule 47 expansion should add harder rehabilitation pairs, juror answers that hedge on following limiting instructions, and challenges based on unpopular but lawful attitudes toward damages or corporate parties.
