# Rule 60 Relief From Judgment

This suite evaluates `resolve_rule60_motion`.  The fixture set covers mistake, newly discovered evidence, fraud, void judgments, changed circumstances, timeliness, unsupported requests, and repeated trial arguments.  The scorer checks grant or denial, recognized ground, reason tags, status effects, and Lean acceptance.

## Files

| File | Use |
| --- | --- |
| [Rule 60 Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Rule 60 Plan](plan.md) | Fixture set, scoring design, prompt-iteration plan, and extension notes. |
| [Rule 60 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule60` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule60` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
