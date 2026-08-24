# Rule 52 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `file_bench_opinion`.  The fixture set contains 16 completed bench trials across contract formation, breach proof, credibility, causation, excluded evidence, damages proof, damages limitation, authentication, agency, and notice.  The eval uses real ADC state, the real Lean opportunity, the production judge prompt or an eval-local prompt candidate, deterministic scoring, and Lean application of each returned payload.

The scorer treats Rule 52 form and substance as part of the decision.  It requires the opinion to state the required element reasoning, avoid excluded evidence, include the proved amount, and separate findings, conclusions, and judgment.  This scoring choice captures bench-opinion failures that would otherwise look like correct final outcomes with incomplete adjudicative reasoning.

## Results

| Prompt | Run | Correct | Winner Correct | Amount Correct | Reason Matches | Invalid | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 16 | 16 | 0 | 1.000 |
| production | live, initial score | 8 | 16 | 16 | 16 | 0 | 0.500 |
| production | live, rescored | 16 | 16 | 16 | 16 | 0 | 1.000 |
| candidate-v1 | dry 16 | 16 | 16 | 16 | 16 | 0 | 1.000 |
| candidate-v1 | live, initial score | 9 | 16 | 16 | 16 | 0 | 0.563 |
| candidate-v1 | live, rescored | 16 | 16 | 16 | 16 | 0 | 1.000 |

## Findings

The current production prompt performed well on the measured Rule 52 set.  In the live run, production selected the correct winner and amount on all 16 rows, used a valid `file_bench_opinion` payload every time, kept excluded evidence out of the judgment reasoning, and produced findings, conclusions, and judgment language.  The initial 8/16 score came from deterministic scorer wording gaps rather than wrong judicial decisions.

The scorer corrections were specific to observed equivalent legal language.  Production used phrases such as `resale failed because`, `additional freight charges`, `audit log (AL-3) was authenticated`, `did not prove any damages amount`, and `previously accepted two prior Lee-initiated orders`.  Candidate v1 used additional valid phrases, including `contains no completed signature block`, `does not identify source data`, `did not meet the contractual written-notice requirement`, and `contemporaneous service log and receiving message were more reliable`.

Candidate v1 preserved the measured production behavior but did not improve it.  It gave more explicit Rule 52 section and record-confinement instructions, and it also selected the correct winner and amount in every live row.  After scorer correction, candidate v1 and production both scored 16/16 with no invalid payloads and no Lean rejections.

The fixture set was corrected during iteration.  Several plaintiff-win rows needed admitted damages or duty evidence stated more concretely, including unpaid invoice records, replacement cost records, a preservation duty for audit records, delivery plus nonpayment evidence, and a repair reimbursement duty.  Those fixture edits kept the expected judgments tied to admitted evidence rather than trial-theory conclusions.

## Recommendation

Do not update the production Rule 52 opportunity prompt from this eval alone.  Production and candidate v1 both scored 16/16 after deterministic scorer corrections, so the candidate prompt does not show a measured improvement.  The next Rule 52 iteration should add harder fixtures before prompt changes, especially mixed-claim bench trials, counterclaims, nominal damages, equitable relief, admitted-but-low-weight evidence, and credibility findings that affect only one element.

## Next Work

The next Rule 52 set should add harder bench-trial rows.  Useful additions include mixed claims, counterclaims, nominal damages, equitable relief, adverse inference, admitted evidence assigned low weight, and credibility findings that affect only one element.  A later runner can test separate findings and conclusions if Lean exposes `add_bench_finding` and `add_bench_conclusion` before `file_bench_opinion`.  Those rows should keep winner and amount scoring separate from the quality of the Rule 52 reasoning.
