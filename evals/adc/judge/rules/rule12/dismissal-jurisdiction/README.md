# Rule 12 Dismissal and Jurisdiction

This suite evaluates `decide_rule12_motion`.  Its eighteen fixtures cover failure to state a claim, subject-matter jurisdiction, standing, ripeness, and mootness, including amendment and prejudice decisions.  The scorer checks disposition, posture, ground-specific fields, explanation tags, payload validity, and Lean acceptance.

## Files

| File | Use |
| --- | --- |
| [Rule 12 Evaluation Analysis](analysis.md) | Fixture coverage, decision boundaries, and scoring. |
| [Rule 12 Evaluation Plan](plan.md) | Fixture schema, scoring rules, and execution procedure. |
| [Rule 12 Fixtures](fixtures.jsonl) | Eighteen fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt templates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule12` or `go run ./adc/runtime/cmd/adc eval judge-rule12`.  The command reads this directory's fixture file and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule12-latest`.  The `--fixtures` and `--out-dir` options select alternate paths, while `--opportunity-prompt-file` and `--opportunity-prompt-name` select and identify a candidate.
