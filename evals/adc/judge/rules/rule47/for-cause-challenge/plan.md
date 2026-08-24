# Rule 47 For-Cause Judge Eval Plan

## Scope

This eval measures the judge's `decide_juror_for_cause_challenge` behavior after voir dire answers have been recorded.  Each fixture builds a jury-trial ADC state in the voir dire phase with one candidate, one answered voir dire exchange, and one pending for-cause challenge.  The runner obtains the real Lean judge opportunity, asks the judge model for one tool call, applies the decision through Lean, and scores the ruling against deterministic fixture labels.

The first fixture set focuses on the line between disqualifying inability and lawful unfavorable attitudes.  It covers fixed bias, refusal to follow the burden of proof, fixed damages floors or caps, refusal to consider digital evidence, direct financial interest, sympathy bias, language or attention limitations, hardship, remote relationships, and rehabilitation by later answers.  False denials receive high weight when the record shows inability to follow law or decide from the record, and false grants receive high weight when the record shows only a lawful attitude that voir dire can examine.

## Fixture Shape

| Field | Meaning |
|---|---|
| `id` | Stable row identifier. |
| `tier` | Difficulty level, with tier 3 for close rehabilitation and lawful-attitude boundaries. |
| `issue_family` | Summary slice for the juror-answer family. |
| `case_theme` | Short factual setting placed in the case caption and docket. |
| `challenged_by` | Party requesting the for-cause strike. |
| `juror_id` | Candidate id used in the pending challenge. |
| `voir_dire_record` | Answer history placed in the questionnaire response, voir dire exchange, and docket. |
| `challenge_grounds` | Party's stated basis for the challenge. |
| `expected_granted` | Expected grant or denial. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Human-readable explanation of the fixture boundary. |

## Scoring

The scorer requires exactly one `decide_juror_for_cause_challenge` tool call.  It checks `challenge_id`, `juror_id`, `by_party`, `granted`, `ruling_reason`, reason tags, and Lean acceptance.  Summary output reports total accuracy, weighted accuracy, invalid rate, false-grant rate, false-denial rate, and slices by reason tag, issue family, tier, and challenging party.

The explanation scorer uses deterministic phrase matching rather than model grading.  It accepts ordinary legal wording for the same reason category, including rehabilitation, credible assurance, refusal to follow instructions, direct interest, lawful skepticism, and fixed damages commitments.  Scorer corrections can be applied through `--rescore-results`, which preserves live model output while improving deterministic label interpretation.

## Prompt Iteration

Candidate v1 adds explicit for-cause boundary language.  It names grant categories for inability to be impartial or to follow law, and denial categories for lawful attitudes, inconvenience, remote relationships, and credible rehabilitation.  Measured results are in [Rule 47 For-Cause Analysis](analysis.md).
