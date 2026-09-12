# Judge Evals

Judge evals are grouped by ARCP rule and judge action.  Each suite keeps its fixtures, prompt candidates, and analysis together.  Nine suites also contain a local `plan.md`, while Rule 47 voir dire uses the cross-rule [Judge Eval Plan](plan.md).  Rule 47 has separate suites for voir dire question screening and for-cause challenges because they use different ADC tools and scorers.

The Go runners live under `adc/runtime/eval`, with CLI defaults in `adc/runtime/cli/eval.go`.  Generated reports belong under ignored `evals/out/adc/judge/` paths.  The cross-rule plan is [Judge Eval Plan](plan.md), and the rule-grouped suite index is [Judge Rule Index](rules/README.md).

Each runner constructs ADC state from a fixture and obtains the corresponding Lean opportunity.  Model-backed turns render the judge view, request a model decision, handle corrections, call Lean `apply_decision`, and execute the accepted `step`.  Rule 58 production turns execute the state-derived judgment action through `step` without a model request or `apply_decision` call.  A procedural attempt-limit or decision-budget failure produces an invalid result row, while other execution errors abort the run.

Rules 11 and 37 use the same model-backed opportunities as ADC cases.  Rule 58 supplies a deterministic action because its claim, basis, and judgment amount derive from the completed verdict state.  `adc eval judge-rule58 --counterfactual-model` removes that action and requests a model decision.  Rule 58 accepts an eval-local opportunity prompt only in counterfactual mode.

Prompt candidates live under each suite's `prompts/` and replace the opportunity objective.  The runner retains the opportunity's allowed tool, constraints, role view, tool schema, and direct-turn semantics, and records the prompt source in its results.  The [prompt-authoring guide](../../../docs/prompt-authoring.md#adc-eval-candidate-templates) defines the candidate tokens and validation rules.

## Suites

| Rule | Suite | Runner | Analysis |
| --- | --- | --- | --- |
| Rule 11 | [Sanctions](rules/rule11/sanctions/README.md) | `judge-rule11` | [Rule 11 Sanctions Analysis](rules/rule11/sanctions/analysis.md) |
| Rule 12 | [Dismissal and Jurisdiction](rules/rule12/dismissal-jurisdiction/README.md) | `judge-rule12` | [Rule 12 Analysis](rules/rule12/dismissal-jurisdiction/analysis.md) |
| Rule 37 | [Discovery Sanctions](rules/rule37/discovery-sanctions/README.md) | `judge-rule37` | [Rule 37 Analysis](rules/rule37/discovery-sanctions/analysis.md) |
| Rule 47 | [Voir Dire Question Screening](rules/rule47/voir-dire-question/README.md) | `judge-voir-dire` | [Rule 47 Voir Dire Analysis](rules/rule47/voir-dire-question/analysis.md) |
| Rule 47 | [For-Cause Challenges](rules/rule47/for-cause-challenge/README.md) | `judge-for-cause` | [Rule 47 For-Cause Analysis](rules/rule47/for-cause-challenge/analysis.md) |
| Rule 51 | [Jury Instructions](rules/rule51/jury-instructions/README.md) | `judge-rule51` | [Rule 51 Analysis](rules/rule51/jury-instructions/analysis.md) |
| Rule 52 | [Bench Opinion](rules/rule52/bench-opinion/README.md) | `judge-rule52` | [Rule 52 Analysis](rules/rule52/bench-opinion/analysis.md) |
| Rule 56 | [Summary Judgment](rules/rule56/summary-judgment/README.md) | `judge-rule56` | [Rule 56 Analysis](rules/rule56/summary-judgment/analysis.md) |
| Rule 58 | [Judgment Entry](rules/rule58/judgment-entry/README.md) | `judge-rule58` | [Rule 58 Analysis](rules/rule58/judgment-entry/analysis.md) |
| Rule 60 | [Relief from Judgment](rules/rule60/relief-from-judgment/README.md) | `judge-rule60` | [Rule 60 Analysis](rules/rule60/relief-from-judgment/analysis.md) |
