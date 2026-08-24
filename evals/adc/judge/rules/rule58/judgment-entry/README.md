# Rule 58 Judgment Entry

This suite evaluates the judge's use of `enter_judgment` after a jury verdict or bench opinion.  Its 16 fixtures cover plaintiff and defense outcomes, zero and nonzero awards, limited damages, authentication, agency, notice, credibility, and excluded evidence.  The scorer checks the claim, basis, reason tags, action execution, final status, and monetary judgment.

## Files

| File | Use |
| --- | --- |
| [Rule 58 Analysis](analysis.md) | Fixture coverage, execution modes, scoring boundaries, and limitations. |
| [Rule 58 Plan](plan.md) | Fixture schema, state construction, execution, and scoring procedure. |
| [Rule 58 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt templates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule58` or `go run ./adc/runtime/cmd/adc eval judge-rule58`.  By default, the runner exercises the deterministic action supplied by the Lean opportunity.  The `--counterfactual-model` option removes that action from the eval opportunity and requests a decision from the selected model.  The command reads this directory's fixture file by default and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule58-latest/`.
