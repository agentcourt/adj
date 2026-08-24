# Rule 58 Judgment Entry

This suite evaluates `enter_judgment`.  The fixture set covers plaintiff verdicts, defense verdicts, damages mismatch, bench decisions, premature judgment attempts, and final status checks.  The scorer checks tool choice, judgment amount, status transition, reason tags, and Lean acceptance.

## Files

| File | Use |
| --- | --- |
| [Rule 58 Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Rule 58 Plan](plan.md) | Fixture set, scoring design, prompt-iteration plan, and extension notes. |
| [Rule 58 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule58` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule58` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
