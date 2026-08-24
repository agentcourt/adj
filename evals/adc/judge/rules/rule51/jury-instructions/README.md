# Rule 51 Jury Instructions

This suite evaluates `settle_jury_instructions`.  The fixture set covers required instruction content, prohibited argument, evidence contamination, burden-shifting language, and limiting-instruction boundaries.  The scorer combines expected disposition checks with required-content, prohibited-content, and deterministic reason-tag checks.

## Files

| File | Use |
| --- | --- |
| [Rule 51 Analysis](analysis.md) | Results, failure analysis, recommendation, and next fixture work. |
| [Rule 51 Plan](plan.md) | Fixture set, scoring design, prompt-iteration plan, and extension notes. |
| [Rule 51 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule51` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule51` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
