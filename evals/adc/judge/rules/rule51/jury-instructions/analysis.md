# Rule 51 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Results

The Rule 51 eval has 16 fixtures and uses the real `settle_jury_instructions` opportunity.  The dry run scored 16/16 and confirmed fixture loading, charge-state construction, Lean opportunity generation, prompt rendering, deterministic scoring, report writing, and Lean acceptance.  The first production live score was 8/16 under an overly literal prohibited-term scorer that treated quoted rejected language as if it appeared in the final charge.

The scorer now distinguishes final-charge contamination from a ruling section that rejects a defective proposal.  It ignores prohibited phrases when the local context shows the instruction was sustained against, rejected, refused, denied, negated, or excluded.  It also accepts equivalent required wording, including `breach caused` for causation, `evidence admitted at trial` for admitted evidence, and `may but are not required to infer` for a permissive adverse inference.

| Prompt | Run | Correct | Reason Matches | Invalid | Missing Required | Prohibited Included |
|---|---|---:|---:|---:|---:|---:|
| Production | dry | 16/16 | 16/16 | 0 | 0 | 0 |
| Production | live, rescored | 16/16 | 16/16 | 0 | 0 | 0 |
| Candidate v1 | dry | 16/16 | 16/16 | 0 | 0 | 0 |
| Candidate v1 | live, rescored | 16/16 | 16/16 | 0 | 0 | 0 |

## Failure Analysis

The initial production failures were not prompt failures.  The model often wrote a ruling section that quoted the rejected party proposal, then wrote a neutral final instruction summary.  The first scorer searched the whole summary for prohibited phrases and therefore marked rejected language as if the final charge had adopted it.

Two initial misses were required-term equivalence problems.  The production summary used `breach caused` rather than the abstract word `causation`, and it used `must not draw any adverse inference` rather than `no adverse inference`.  The candidate summary used `evidence admitted at trial` and `may but are not required to infer`, which satisfy the same required concepts.

Candidate v1 preserved production behavior but did not improve measured results.  It provides more explicit instruction-settlement guidance, but the current fixture set does not show a production failure after scorer correction.  The measured result therefore supports keeping production prompt text unchanged until a harder Rule 51 set exposes a real failure cluster.

## Recommendation

Do not update the production Rule 51 opportunity prompt from this eval alone.  Production and candidate v1 both scored 16/16 on the measured live fixture set after deterministic scorer corrections.  The next iteration should add a hard set before any prompt change, especially rows where ruling language and final-charge language are close enough to stress the scorer and the judge prompt.

## Next Work

The next Rule 51 set should include close ruling-versus-final-charge language, excluded settlement communications, mitigation instructions, verdict-threshold errors, and limiting instructions that quote defective proposals near the final charge summary.  A later `deliver_jury_instructions` eval should score complete charge text after the settlement-summary scorer remains stable on harder fixtures.  That second eval should verify full-charge structure, neutral wording, and consistency with the settled instruction summary.
