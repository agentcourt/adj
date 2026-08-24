# Rule 11 Judge Eval Plan

## Scope

This eval measures the judge's `decide_rule11_motion` behavior.  Each fixture builds a filed-stage ADC state with a complaint, a challenged filing, a Rule 11 safe-harbor notice, an optional correction record, a Rule 11 motion, and opposition text.  The runner obtains the real Lean judge opportunity, asks the model for one tool call, applies the payload through Lean, and scores the ruling, sanction fields, explanation tags, and Lean acceptance.

The first fixture set focuses on restraint and enforcement.  It includes frivolous legal contentions, factual contentions with no evidentiary support, factual denials contradicted by records available before filing, improper-purpose filings, nonfrivolous extension arguments, allegations likely to have support after discovery, weak merits positions, safe-harbor defects, timely correction, and discovery conduct that belongs under Rule 37.  The highest-risk failures are false grants against legitimate advocacy, invalid sanction payloads on denied motions, and disproportionate monetary sanctions.

## Fixture Shape

| Field | Meaning |
|---|---|
| `id` | Stable row identifier. |
| `tier` | Difficulty level, with tier 3 for adversarial or close boundaries. |
| `issue_family` | Summary slice for the Rule 11 issue family. |
| `case_theme` | Short factual setting placed in the case caption and docket. |
| `movant` | Party seeking Rule 11 sanctions. |
| `target_party` | Party whose filing is challenged. |
| `challenged_filing` | Name of the filing or paper under Rule 11 review. |
| `filing_text` | Challenged filing content. |
| `notice_text` | Safe-harbor notice text. |
| `notice_served_at` | Safe-harbor service date. |
| `motion_filed_at` | Rule 11 motion filing date. |
| `correction_text` | Optional withdrawal or correction during the safe-harbor period. |
| `motion_text` | Movant's Rule 11 motion and requested sanction. |
| `opposition_text` | Target party's reason for denial or sanction limits. |
| `expected_granted` | Expected grant or denial. |
| `expected_sanction_type` | `none`, `admonition`, `non_monetary_directive`, `monetary_penalty`, or `fee_shift`. |
| `expected_sanction_amount` | Required amount when the expected sanction is monetary. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Human-readable explanation of the fixture boundary. |

## Scoring

The scorer requires exactly one `decide_rule11_motion` tool call.  It checks `motion_index`, `granted`, `sanction_type`, `sanction_amount`, `sanction_detail`, `reasoning`, deterministic reason tags, and Lean acceptance.  Denied motions are invalid if they include a nonempty `sanction_type`, any nonzero `sanction_amount`, or a nonempty `sanction_detail`, because Lean treats those fields as sanctions despite the denial.

Granted motions require a nonempty sanction type and sanction detail.  `monetary_penalty` and `fee_shift` require a positive amount, while `admonition` and `non_monetary_directive` reject monetary amounts.  The summary reports total accuracy, grant accuracy, weighted accuracy, invalid rate, false-grant rate, false-denial rate, sanction mismatches, and slices by reason tag, issue family, tier, movant, and expected sanction type.

## Prompt Iteration

Candidate v1 states the payload rules directly: a denied Rule 11 order carries no sanction fields, and a monetary sanction carries an amount.  Candidate v2 keeps those rules and narrows the sanction ladder, reserving non-monetary directives for correction-focused records or motion-specific requests.  Measured results are in [Rule 11 Sanctions Analysis](analysis.md).
