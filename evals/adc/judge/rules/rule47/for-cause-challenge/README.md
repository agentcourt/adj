# Rule 47 For-Cause Challenges

This suite evaluates `decide_juror_for_cause_challenge`.  The fixture set covers fixed bias, refusal to follow law, damages precommitment, digital-evidence refusal, relationship interest, sympathy bias, language or attention limits, hardship, lawful attitudes, and rehabilitation.  The scorer checks grant or denial, challenge identifiers, juror identifiers, reason tags, and Lean acceptance.

## Files

| File | Use |
| --- | --- |
| [Rule 47 For-Cause Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [For-Cause Plan](plan.md) | Fixture shape, scoring design, and prompt-iteration plan. |
| [For-Cause Fixtures](fixtures.jsonl) | Sixteen committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-for-cause` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-for-cause` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
