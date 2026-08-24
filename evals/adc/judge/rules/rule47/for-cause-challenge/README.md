# Rule 47 For-Cause Challenges

This suite evaluates `decide_juror_for_cause_challenge`.  Its sixteen fixtures cover inability to follow law, fixed bias, damages commitments, evidence refusal, direct interests, sympathy, rehabilitation, lawful attitudes, language or attention limits, and hardship.  The scorer checks the ruling, required identifiers, explanation tags, payload validity, and Lean acceptance.

## Files

| File | Use |
| --- | --- |
| [Rule 47 For-Cause Analysis](analysis.md) | Fixture coverage, decision boundaries, and scoring. |
| [Rule 47 For-Cause Plan](plan.md) | Fixture schema, scoring rules, and execution procedure. |
| [For-Cause Fixtures](fixtures.jsonl) | Sixteen fixture rows. |
| [Prompt Candidate](prompts/candidate-v1.md) | Eval-local opportunity prompt template. |

## Runner

Run the suite from the repository root with `adc eval judge-for-cause` or `go run ./adc/runtime/cmd/adc eval judge-for-cause`.  The command reads this directory's fixture file and writes `results.jsonl` and `summary.json` under `evals/out/adc/judge/for-cause-latest`.  The `--fixtures` and `--out-dir` options select alternate paths.  Supply the candidate with `--opportunity-prompt-file` and identify it with `--opportunity-prompt-name`.
