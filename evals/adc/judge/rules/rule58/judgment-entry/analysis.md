# Rule 58 Judge Eval Analysis

## Scope

The fixture file contains 16 eligible post-verdict states: nine jury verdicts and seven bench opinions.  Four rows are tier 1, six are tier 2, and six are tier 3.  The cases include plaintiff and defense outcomes, a zero-dollar plaintiff verdict, limited and larger awards, authentication, agency, notice, credibility, and rejection of excluded or unsupported proof.

Jury fixtures place a unanimous verdict and damages amount in state.  Bench fixtures place a `Bench Opinion` docket entry and the adjudicated amount in `monetary_judgment`.  Both postures reach the `post_verdict` phase with traces that allow Lean to offer the required `enter_judgment` action.

## Execution Modes

The default execution mode uses the deterministic single-tool action embedded in the Lean opportunity.  This path verifies opportunity generation, action validation, step execution, and state effects through that action.  The summary records this path as `production` with `counterfactual_model: false`.

The `--counterfactual-model` option first verifies that Lean supplied a deterministic action, removes that action from the eval opportunity, and asks the selected model for a decision.  That path uses the configured ADC prompts and any eval-local opportunity template.  The summary records this path as `counterfactual_model` with `counterfactual_model: true`.

## Scoring Boundary

A valid response contains exactly one `enter_judgment` call with a nonempty `claim_id` and `basis`.  Correctness requires the fixture's claim id, all required basis concepts, no prohibited basis concepts, a matching reason tag, and successful action execution.  Model-backed mode obtains `lean_accepted` from `apply_decision`.  Deterministic production mode has no decision stage and sets `lean_accepted` when the step succeeds.  After execution, the case must have `status=judgment_entered` and the expected `monetary_judgment`.

The `enter_judgment` payload has no amount field.  Jury judgment amounts come from `jury_verdict.damages`, while bench judgment amounts already reside in the case state before entry.  State-derived amount scoring therefore verifies that the engine carries the adjudicated amount through the Rule 58 transition.

The aggregate summary reports accuracy, severity-weighted accuracy, invalid rate, rejection counters, component counts, and slices by reason tag, issue family, tier, and trial mode.  The `lean_rejected` and `step_rejected` counters include only rows without an `invalid_reason`.  In counterfactual-model mode, a decision or step rejection that exhausts the correction or decision budget becomes invalid and does not increment those counters.  A deterministic step failure or other nonprocedural execution error aborts the run.  Per-row results retain the attempted acceptance fields, state before and after execution, opportunity, response, extracted payload, and each component score.

## Limits

Every fixture has one claim and an available post-verdict judgment-entry opportunity.  The fixture scope covers judgment after a jury verdict or bench opinion.  The scorer's basis checks use configured textual concepts to determine whether the payload identifies the adjudicative basis already present in state.
