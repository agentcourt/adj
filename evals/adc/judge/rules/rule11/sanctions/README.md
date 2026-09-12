# Rule 11 Sanctions

This suite evaluates `decide_rule11_motion`.  Its sixteen fixtures cover unsupported legal and factual contentions, improper purpose, reasonable inquiry, safe-harbor effects, discovery-filing limits, and sanction proportionality.  The scorer checks the grant decision, sanction fields, explanation tags, payload validity, and action execution.

## Files

| File | Use |
| --- | --- |
| [Rule 11 Evaluation Analysis](analysis.md) | Fixture coverage, decision boundaries, and scoring. |
| [Rule 11 Evaluation Plan](plan.md) | Fixture schema, scoring rules, and execution procedure. |
| [Rule 11 Fixtures](fixtures.jsonl) | Sixteen fixture rows. |
| [Prompt Candidates](prompts/) | Opportunity prompt templates for model-backed evaluation. |

## Runner

Run the default suite from the repository root with `adc eval judge-rule11` or `go run ./adc/runtime/cmd/adc eval judge-rule11`.  The command reads this directory's fixture file and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule11-latest`.  The `--fixtures` and `--out-dir` options select alternate paths, and `--opportunity-prompt-file` selects a candidate objective template.
