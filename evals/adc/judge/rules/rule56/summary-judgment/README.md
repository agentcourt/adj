# Rule 56 Summary Judgment

This suite evaluates `decide_rule56_motion`.  The fixture set covers no-dispute grants, denials for genuine factual disputes, credibility disputes, competing inferences, element-specific failures, unsupported damages theories, and authentication disputes.  The scorer checks disposition, partial-judgment mismatches, reason tags, false grants, false denials, and invalid payloads.

## Files

| File | Use |
| --- | --- |
| [Rule 56 Analysis](analysis.md) | Results, findings, recommendation, and next fixture work. |
| [Rule 56 Plan](plan.md) | Fixture shape and prompt-iteration plan. |
| [Rule 56 Fixtures](fixtures.jsonl) | Thirty committed fixture rows. |
| [Prompt Candidates](prompts/) | Eval-local opportunity prompt candidates. |

## Runner

Run the suite from the repository root with `adc eval judge-rule56` or the corresponding `go run ./adc/runtime/cmd/adc eval judge-rule56` form.  The CLI default fixture path points to this directory.  Generated results belong under `evals/out/adc/judge/`.
