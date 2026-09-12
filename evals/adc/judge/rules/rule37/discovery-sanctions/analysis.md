# Rule 37 Evaluation Analysis

## Evaluation Coverage

The evaluation exercises `decide_rule37_motion` across sixteen discovery-stage states.  Seven fixtures expect a grant, and nine expect a denial.  The issue families cover nonresponse, complete and evasive responses, justified objections, overbreadth, proportionality, harmless cure, disclosure failure, discovery-order violation, requests for admission, premature motions, grants without fees, and fee-only requests without a discovery failure.

## Decision Boundaries

The fixtures distinguish a concrete discovery failure from a complete response, a substantially justified objection, and an overbroad or disproportionate request.  They also test cured defects, premature motion practice, and the Rule 36 consequence that an unanswered request for admission is admitted rather than compelled under Rule 37.  Grant fixtures separate an order compelling discovery from a fee award by testing harmlessness, justification, available fee amounts, and whether fees would be unjust.

## Payload and Scoring

Each fixture constructs ADC state, obtains the Lean judge opportunity, and executes the opportunity's action through the runner and Lean.  The scorer requires one `decide_rule37_motion` payload with motion index zero, a Boolean grant decision, a recognized `sanction_type`, and nonempty reasoning.  A denial requires `sanction_type: none`, while `fees` requires a grant and a positive `sanction_amount`.  The `none` sanction type rejects a nonzero amount.

Outcome correctness combines the grant decision, expected sanction, `lean_accepted`, and `step_accepted`.  The runner obtains `lean_accepted` from `apply_decision` and `step_accepted` from execution of the accepted action.  Explanation scoring searches both `reasoning` and `order_text` for deterministic reason tags.  The summary reports accuracy, grant accuracy, severity-weighted accuracy, invalid responses, false grants, false denials, sanction mismatches, and slices by issue family, reason tag, tier, movant, and expected sanction.

## Prompt Templates

The [prompt directory](prompts/) contains two opportunity templates.  Both state the discovery-failure boundary and the conditions for awarding fees, while `candidate-v2.md` enumerates required payload fields and denial categories.  The evaluator substitutes fixture fields and `{{production_objective}}` before sending a selected template through the production model-backed opportunity.
