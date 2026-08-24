# Rule 47 Voir Dire Question Screening

This suite evaluates `decide_voir_dire_question`.  The sixty-row baseline file balances permissible screening questions against merits argument, assumed facts, liability or damages precommitment, specific-evidence sufficiency, inadmissible material, and compound precommitment.  The thirty-row hard file contains tier-3 boundary pairs for damages, digital evidence, limiting instructions, missing witnesses, insurance references, and conditional-verdict phrasing.

## Files

| File | Use |
| --- | --- |
| [Rule 47 Voir Dire Analysis](analysis.md) | Fixture coverage, decision boundaries, scoring, and execution procedure. |
| [Baseline Fixtures](fixtures.jsonl) | Sixty fixture rows. |
| [Hard Fixtures](hard-fixtures.jsonl) | Thirty tier-3 fixture rows. |
| [Prompt Candidates](prompts/) | Three eval-local opportunity prompt templates. |

## Runner

Run the baseline suite from the repository root with `adc eval judge-voir-dire` or `go run ./adc/runtime/cmd/adc eval judge-voir-dire`.  The command writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/latest`.  The `--out-dir` option selects another destination.  Select the hard file with `--fixtures evals/adc/judge/rules/rule47/voir-dire-question/hard-fixtures.jsonl`, and supply a candidate with `--opportunity-prompt-file` and `--opportunity-prompt-name`.
