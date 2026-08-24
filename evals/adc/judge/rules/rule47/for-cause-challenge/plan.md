# Rule 47 For-Cause Evaluation Plan

## Scope

This evaluation measures decisions produced for `decide_juror_for_cause_challenge` after a voir dire answer has been recorded.  Each fixture builds a jury-trial ADC state in the voir dire phase with one candidate, one answered exchange, and one pending challenge.  The runner obtains the judge opportunity, executes the opportunity, applies the ruling through Lean, and scores the decision and payload.

## Fixture Coverage

The fixture file contains sixteen rows across three difficulty tiers.  Nine rows expect a grant, and seven expect a denial.  The `severity` field weights each row when the summary computes weighted accuracy.

The fixtures test fixed bias, refusal to follow the burden of proof or court instructions, damages floors or caps, refusal to consider an evidence category, direct interests, sympathy, communication and attention limits, hardship, lawful skepticism, remote relationships, and rehabilitation.  Grant labels require a record showing inability to serve impartially or competently.  Denial labels require a record showing a lawful attitude, manageable inconvenience, remote connection, or credible assurance that the candidate can follow the law.

## Fixture Shape

| Field | Meaning |
| --- | --- |
| `id` | Stable row identifier. |
| `tier` | Difficulty tier. |
| `issue_family` | Juror-answer family used for summary slices. |
| `case_theme` | Short factual setting placed in ADC state. |
| `challenged_by` | Party seeking the for-cause strike. |
| `juror_id` | Candidate identifier used in the pending challenge. |
| `voir_dire_record` | Recorded answers placed in the questionnaire, exchange, and docket. |
| `challenge_grounds` | Party's stated basis for the challenge. |
| `expected_granted` | Expected grant or denial. |
| `expected_reason_tags` | Reason categories used by deterministic explanation scoring. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Explanation of the for-cause boundary tested by the row. |

## Scoring

The scorer accepts exactly one `decide_juror_for_cause_challenge` call.  It requires challenge id `fc-1`, the fixture's `juror_id`, the normalized `challenged_by` party in `by_party`, a Boolean `granted` value, and nonempty `ruling_reason`.  It compares the grant decision with the fixture label and uses the ruling reason for deterministic explanation matching.

Outcome correctness also requires Lean acceptance and an accepted runner step.  The summary reports aggregate accuracy, weighted accuracy, invalid rate, false-grant rate, false-denial rate, and explanation matches.  It also provides slices by reason tag, issue family, tier, and challenging party.

## Execution and Prompt Selection

The command resolves the configured court and model, then gives the resulting response client and ADC state to the opportunity runner.  The runner uses the objective supplied by the judge opportunity by default.  A selected template replaces that objective after fixture substitution.  The evaluator records prompt source, prompt name, per-fixture records, and aggregate metrics in `results.jsonl` and `summary.json`.

```bash
adc eval judge-for-cause \
  --opportunity-prompt-file evals/adc/judge/rules/rule47/for-cause-challenge/prompts/candidate-v1.md \
  --opportunity-prompt-name candidate-v1 \
  --out-dir evals/out/adc/judge/for-cause-candidate-v1
```
