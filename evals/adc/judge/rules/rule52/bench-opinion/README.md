# Rule 52 Bench Opinion

This suite evaluates the judge's use of `file_bench_opinion`.  Its 16 fixtures cover contract formation, breach proof, conflicting testimony, causation and damages gaps, excluded evidence, authentication, agency, notice, and damages limits.  The scorer checks the verdict, amount, required and prohibited concepts, the presence of findings, conclusions, and judgment language, Lean acceptance, and execution of the accepted action.

## Files

| File | Use |
| --- | --- |
| [Rule 52 Analysis](analysis.md) | Fixture coverage, scoring boundaries, and limitations. |
| [Rule 52 Plan](plan.md) | Fixture schema, state construction, execution, and scoring procedure. |
| [Rule 52 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt templates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule52` or `go run ./adc/runtime/cmd/adc eval judge-rule52`.  The command reads this directory's fixture file by default and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule52-latest/`.  Use `--opportunity-prompt-file` to select an eval-local template or `--rescore-results` to apply the deterministic scorer to a result file.
