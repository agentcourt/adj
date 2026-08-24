# Rule 11 Sanctions

This suite evaluates `decide_rule11_motion`.  The fixture set covers frivolous legal contentions, factual-contention support, improper purpose, safe-harbor effects, discovery-filing limits, reasonable extension arguments, and sanction proportionality.  The scorer checks the grant or denial, sanction type, amount fields, reason tags, and invalid payloads.

## Files

| File | Use |
| --- | --- |
| [Rule 11 Sanctions Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Rule 11 Sanctions Plan](plan.md) | Fixture shape, scoring design, and prompt-iteration plan. |
| [Rule 11 Sanctions Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule11` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule11` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
