# Rule 60 Relief from Judgment

This suite evaluates the judge's use of `resolve_rule60_motion`.  Its 16 fixtures cover mistake, excusable neglect, newly discovered evidence, fraud, void and satisfied judgments, prospective inequity, extraordinary circumstances, timeliness, and attempts to reargue the merits.  The scorer checks grant or denial, required and prohibited concepts, reason tags, Lean acceptance, and execution of the accepted action.

## Files

| File | Use |
| --- | --- |
| [Rule 60 Analysis](analysis.md) | Fixture coverage, scoring boundaries, and limitations. |
| [Rule 60 Plan](plan.md) | Fixture schema, state construction, execution, and scoring procedure. |
| [Rule 60 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt templates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule60` or `go run ./adc/runtime/cmd/adc eval judge-rule60`.  The command reads this directory's fixture file by default and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule60-latest/`.  Use `--opportunity-prompt-file` to select an eval-local template or `--rescore-results` to apply the deterministic scorer to a result file.
