# Rule 11 Evaluation Analysis

## Evaluation Coverage

The evaluation exercises `decide_rule11_motion` across sixteen filed-stage states.  Seven fixtures expect a grant, and nine expect a denial.  The issue families cover unsupported legal and factual contentions, improper purpose, nonfrivolous extensions, likely evidentiary support, weak merits positions, safe-harbor defects, timely correction, discovery conduct, and reasonable inquiry.

## Decision Boundaries

The fixtures distinguish filing abuse from weak advocacy, contested inferences, and allegations that may obtain evidentiary support through discovery.  They also distinguish Rule 11 conduct from discovery disputes governed by Rule 37 and test whether timely correction or defective safe-harbor procedure defeats sanctions.  Grant fixtures require a proportionate response selected from admonition, non-monetary directive, monetary penalty, or fee shift.

## Payload and Scoring

Each fixture constructs ADC state, obtains the Lean judge opportunity, and executes the opportunity's action through the runner and Lean.  The scorer requires one `decide_rule11_motion` payload with motion index zero, a Boolean grant decision, nonempty reasoning, and a present `sanction_detail` field.  A denial requires empty sanction-type and sanction-detail fields and a zero or omitted amount.  A grant requires a recognized sanction type and a nonempty detail.  Monetary penalties and fee shifts require positive amounts, while admonitions and non-monetary directives use zero or omitted amounts.

Outcome correctness combines the grant decision, expected sanction, `lean_accepted`, and `step_accepted`.  The runner obtains `lean_accepted` from `apply_decision` and `step_accepted` from execution of the accepted action.  Explanation scoring uses deterministic phrase matching against each fixture's reason tags.  The summary reports accuracy, grant accuracy, severity-weighted accuracy, invalid responses, false grants, false denials, sanction mismatches, and slices by issue family, reason tag, tier, movant, and expected sanction.

## Prompt Templates

The [prompt directory](prompts/) contains two opportunity templates.  Both state the Rule 11 decision boundary and payload requirements, while `candidate-v2.md` gives a more specific sanction-selection rule for limited legal defects and correction directives.  The evaluator substitutes fixture fields and `{{production_objective}}` before sending a selected template through the production model-backed opportunity.
