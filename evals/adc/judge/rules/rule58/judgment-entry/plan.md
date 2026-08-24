# Rule 58 Judge Eval Plan

## Procedure

This eval measures the judge's entry of judgment under Rule 58 through `enter_judgment`.  Each fixture constructs an eligible post-verdict state after either a jury verdict or a filed bench opinion.  Lean then supplies a deterministic single-tool judgment-entry opportunity, and the runner validates and executes the action through the ADC turn path.

Jury states contain `jury_verdict`, and bench states contain a `Bench Opinion` docket entry and an adjudicated amount in `monetary_judgment`.  The default path uses the opportunity's deterministic action, while `--counterfactual-model` removes that action from the eval opportunity and requests a decision from the selected judge model.  Every result records the fixture, state, opportunity, applied state, provider accounting, extracted payload, and component scores.  Counterfactual-model results also record the role view, prompt input, and response exchanges.  Deterministic production results leave those fields empty.

## Fixture Set

The fixture file contains 16 rows across three difficulty tiers.  Nine rows use jury verdicts, and seven use bench opinions.  The set checks plaintiff money judgments, defense judgments, a zero-dollar plaintiff verdict, limited damages, larger awards, authentication, agency, notice, credibility, and bench opinions that reject excluded or unsupported proof.

| Theme | Scored Boundary |
|---|---|
| Jury plaintiff verdict | Judgment follows the jury verdict and uses the verdict damages |
| Jury defense verdict | Judgment enters for defendant with zero monetary judgment |
| Jury zero-dollar plaintiff verdict | Judgment enters from the verdict without inventing damages |
| Bench plaintiff opinion | Judgment follows the bench opinion and the state amount |
| Bench defense opinion | Judgment enters for defendant with zero monetary judgment |
| Limited damages | Judgment preserves the limited amount already determined |
| Record finality | Judgment entry preserves the adjudicated liability and damages |

## Scoring

The scorer requires exactly one `enter_judgment` tool call with a nonempty `claim_id` and `basis`.  It checks claim id, basis concepts, prohibited basis concepts, reason tags, `lean_accepted`, `step_accepted`, final case status, and final monetary judgment.  In model-backed mode, `lean_accepted` records `apply_decision`.  In deterministic production mode, a successful step sets both acceptance fields.  The final amount is scored from the post-step state, because the Rule 58 payload does not carry an amount field.

The summary reports total accuracy, weighted accuracy, invalid rate, rejection counters, claim correctness, basis correctness, reason correctness, final status correctness, final amount correctness, and slices by trial mode, issue family, tier, and reason tag.  The `lean_rejected` and `step_rejected` counters exclude rows with an `invalid_reason`, including correction-exhausted decision or step rejections.  Amount scoring reads the case state after execution because the `enter_judgment` payload contains only the claim id and basis.  The engine derives the judgment amount from the jury verdict or the amount already recorded for the bench opinion.

## Prompt Selection

The default path executes the opportunity's deterministic action directly.  In `--counterfactual-model` mode, [candidate v1](prompts/candidate-v1.md) supplies the trial result and the claim and basis required by the fixture and directs the model to preserve the adjudicated merits when entering judgment.  The ADC prompt catalog and any `--prompt-dir` or `--prompt-file` overrides determine the surrounding judge prompt.

## Outputs

The runner writes one JSON object per fixture to `results.jsonl` and an aggregate `summary.json` in the selected output directory.  The `--limit`, court, timeout, prompt-catalog, and Lean-engine options apply to both execution modes.  Model, temperature, and opportunity-prompt options govern the model request in `--counterfactual-model` mode.  The output records `execution_mode` and `counterfactual_model` so deterministic and model-backed results remain distinguishable.
