# Rule 47 Voir Dire Question Screening

This suite evaluates `decide_voir_dire_question`.  The baseline fixture set covers allowed bias and rule-following probes, along with prohibited merits argument, assumed disputed facts, liability precommitment, damages precommitment, specific-evidence sufficiency, inadmissible-material references, and disguised vote forecasts.  The hard fixture set adds tier-3 boundary pairs for damages ranges, digital-evidence sufficiency, limiting-instruction phrasing, missing-witness sufficiency, insurance references, and "could you still find" formulations.

## Files

| File | Use |
| --- | --- |
| [Rule 47 Voir Dire Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Baseline Fixtures](fixtures.jsonl) | Sixty committed baseline fixture rows. |
| [Hard Fixtures](hard-fixtures.jsonl) | Thirty committed hard fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the baseline suite from the repository root with `adc eval judge-voir-dire` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-voir-dire` form.  Use `--fixtures evals/adc/judge/rules/rule47/voir-dire-question/hard-fixtures.jsonl` for the hard fixture set.  Generated results belong under `evals/out/adc/judge/`.
