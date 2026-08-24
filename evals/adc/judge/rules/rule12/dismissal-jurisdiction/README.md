# Rule 12 Dismissal and Jurisdiction

This suite evaluates `decide_rule12_motion` and subject-matter jurisdiction dismissal behavior.  The fixture set covers missing elements, conclusory allegations, allegations that must be accepted at the pleading stage, amount-in-controversy defects, citizenship defects, standing defects, amendable pleadings, and prejudice decisions.  The scorer checks disposition, ground, closure status, leave to amend, prejudice, reason tags, and invalid payloads.

## Files

| File | Use |
| --- | --- |
| [Rule 12 Analysis](analysis.md) | Results, failure analysis, recommendation, and next fixture work. |
| [Rule 12 Plan](plan.md) | Fixture set, scoring design, prompt-iteration plan, and extension notes. |
| [Rule 12 Fixtures](fixtures.jsonl) | Eighteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule12` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule12` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
