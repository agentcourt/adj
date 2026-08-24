# Rule 51 Jury Instructions

This suite evaluates the judge's use of `settle_jury_instructions`.  Its 16 fixtures cover required instruction content, argumentative or fact-assuming language, evidence boundaries, burden shifting, damages, credibility, and adverse inferences.  The scorer checks the returned summary, Lean acceptance, and execution of the accepted action.

## Files

| File | Use |
| --- | --- |
| [Rule 51 Analysis](analysis.md) | Fixture coverage, scoring boundaries, and limitations. |
| [Rule 51 Plan](plan.md) | Fixture schema, state construction, execution, and scoring procedure. |
| [Rule 51 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt templates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule51` or `go run ./adc/runtime/cmd/adc eval judge-rule51`.  The command reads this directory's fixture file by default and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule51-latest/`.  Use `--opportunity-prompt-file` to select an eval-local template or `--rescore-results` to apply the deterministic scorer to a result file.
