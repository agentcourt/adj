# Rule 56 Summary Judgment

This suite evaluates the judge's use of `decide_rule56_motion`.  Its 30 fixtures cover undisputed records, credibility disputes, competing inferences, authentication disputes, missing elements, legal bars, unsupported damages, movant burden failures, and partial dispositions.  The scorer checks disposition, surviving issues, reason tags, false grants, false denials, invalid payloads, Lean acceptance, and execution of the accepted action.

## Files

| File | Use |
| --- | --- |
| [Rule 56 Analysis](analysis.md) | Fixture coverage, scoring boundaries, and limitations. |
| [Rule 56 Plan](plan.md) | Fixture schema, state construction, execution, and scoring procedure. |
| [Rule 56 Fixtures](fixtures.jsonl) | Thirty committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt templates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule56` or `go run ./adc/runtime/cmd/adc eval judge-rule56`.  The command reads this directory's fixture file by default and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule56-latest/`.  Use `--opportunity-prompt-file` and `--opportunity-prompt-name` to select and identify an eval-local template.
