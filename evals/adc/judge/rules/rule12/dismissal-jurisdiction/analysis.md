# Rule 12 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Results

The Rule 12 eval has 18 fixtures and uses the real `decide_rule12_motion` opportunity.  The dry run scored 18/18 and confirmed fixture loading, state construction, Lean opportunity generation, template rendering, scoring, report writing, and Lean acceptance.  The production live run initially scored 15/18 under the first scorer, but two misses reflected scorer precision rather than model behavior.

The scorer now accepts equivalent missing-element labels and jurisdiction-basis wording.  A payload identifying `facts constituting breach` satisfies an expected `breach` element, and a payload rejecting both federal-question and diversity jurisdiction satisfies an expected omitted jurisdiction basis.  After rescoring the same production live result file, production scored 17/18, with 18/18 reason matches, no false dismissals, no false denials, no invalid responses, and one posture mismatch.

| Prompt | Run | Correct | Reason Matches | Invalid | False Dismissals | False Denials | Posture Mismatches |
|---|---|---:|---:|---:|---:|---:|---:|
| Production | dry | 18/18 | 18/18 | 0 | 0 | 0 | 0 |
| Production | live, rescored | 17/18 | 18/18 | 0 | 0 | 0 | 1 |
| Candidate v1 | dry | 18/18 | 18/18 | 0 | 0 | 0 | 0 |
| Candidate v1 | live | 15/18 | 18/18 | 0 | 0 | 0 | 3 |
| Candidate v2 | dry | 18/18 | 18/18 | 0 | 0 | 0 | 0 |
| Candidate v2 | live | 18/18 | 18/18 | 0 | 0 | 0 | 0 |

## Failure Analysis

Production’s remaining live failure was `r12-014`.  The model correctly granted dismissal as moot because the complaint alleged full payment before filing and sought only the same payment.  It incorrectly set `leave_to_amend` to true, even though the pleaded facts showed no remaining live relief on the current complaint.

Candidate v1 fixed `r12-014` but overcorrected.  It told the judge to deny leave when the pleaded facts showed no live controversy, which the model applied to curable standing redressability, ripeness, and diversity-amount defects.  The failed rows were `r12-010`, `r12-011`, and `r12-017`, all of which expected dismissal with leave to amend.

Candidate v2 is the best measured Rule 12 prompt.  It preserves leave to amend for curable jurisdiction, standing, ripeness, and claim-element defects, and it denies leave only for mootness when the complaint admits complete prefiling satisfaction of every requested form of relief.  It scored 18/18 on the full live fixture set without invalid responses or reason-tag failures.

## Recommendation

Candidate v2 should be the production prompt candidate if ADC updates the Rule 12 opportunity text.  The measured improvement is narrow: it fixes complete prefiling satisfaction without changing the treatment of amendable jurisdiction, standing, ripeness, or pleading defects.  The next test set should add more mootness and amendment rows before changing production text, because the current live evidence for v2 comes from one full 18-row pass.

## Next Work

The next Rule 12 set should add more mootness and amendment-posture rows before prompt promotion.  Paired Rule 12 and Rule 56 rows would also test whether the judge preserves the pleading-stage standard when the same factual setting later appears as an evidence-sufficiency problem.  Court-driven subject-matter jurisdiction screening should be added if ADC exposes a judge opportunity for dismissal without a party Rule 12 motion.
