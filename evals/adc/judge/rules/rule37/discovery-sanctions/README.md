# Rule 37 Discovery Sanctions

This suite evaluates `decide_rule37_motion`.  The fixture set covers discovery failures, justified objections, proportionality disputes, harmlessness, prior-order violations, and sanction payload constraints.  The scorer checks grant or denial, sanction type, fee amount, reason tags, and invalid payloads.

## Files

| File | Use |
| --- | --- |
| [Rule 37 Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Rule 37 Plan](plan.md) | Fixture shape, scoring design, and prompt-iteration plan. |
| [Rule 37 Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule37` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule37` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
