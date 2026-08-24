# Rule 52 Bench Opinion

This suite evaluates `file_bench_opinion`.  The fixture set covers admitted-document proof, conflicting testimony, missing causation, damages proof gaps, credibility explanation, unadmitted exhibit references, and conclusions that omit elements.  The scorer checks winner selection, amount, required element reasoning, prohibited reliance on excluded proof, fact-law-judgment separation, and Lean acceptance.

## Files

| File | Use |
| --- | --- |
| [Rule 52 Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Rule 52 Plan](plan.md) | Fixture set, scoring design, prompt-iteration plan, and extension notes. |
| [Rule 52 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule52` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule52` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
