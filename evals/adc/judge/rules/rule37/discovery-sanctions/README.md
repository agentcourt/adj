# Rule 37 Discovery Sanctions

This suite evaluates `decide_rule37_motion`.  Its sixteen fixtures cover concrete discovery failures, complete responses, justified objections, overbreadth, proportionality, cure, disclosure duties, discovery-order violations, and fee limits.  The scorer checks the grant decision, sanction fields, explanation tags, payload validity, and action execution.

## Files

| File | Use |
| --- | --- |
| [Rule 37 Evaluation Analysis](analysis.md) | Fixture coverage, decision boundaries, and scoring. |
| [Rule 37 Evaluation Plan](plan.md) | Fixture schema, scoring rules, and execution procedure. |
| [Rule 37 Fixtures](fixtures.jsonl) | Sixteen fixture rows. |
| [Prompt Candidates](prompts/) | Opportunity prompt templates for model-backed evaluation. |

## Runner

Run the default suite from the repository root with `adc eval judge-rule37` or `go run ./adc/runtime/cmd/adc eval judge-rule37`.  The command reads this directory's fixture file and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/rule37-latest`.  The `--fixtures` and `--out-dir` options select alternate paths, and `--opportunity-prompt-file` selects a candidate objective template.
