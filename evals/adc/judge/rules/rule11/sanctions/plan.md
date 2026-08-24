# Rule 11 Evaluation Plan

## Scope

This evaluation measures decisions produced for `decide_rule11_motion`.  Each fixture builds a filed-stage ADC state containing a complaint, the challenged filing, safe-harbor notice and dates, an optional correction, the sanctions motion, and the opposition.  The runner obtains the judge opportunity, executes its action through Lean, and scores the legal outcome and payload.

## Fixture Coverage

The fixture file contains sixteen rows across three difficulty tiers.  The rows test both enforcement and restraint, with seven expected grants and nine expected denials.  Every row has severity 5, so weighted accuracy equals ordinary accuracy.

## Fixture Shape

| Field | Meaning |
| --- | --- |
| `id` | Stable row identifier. |
| `tier` | Difficulty tier. |
| `issue_family` | Rule 11 issue used for summary slices. |
| `case_theme` | Short factual setting placed in ADC state. |
| `movant` | Party seeking sanctions. |
| `target_party` | Party whose filing is challenged. |
| `challenged_filing` | Filing or paper under review. |
| `filing_text` | Content of the challenged filing. |
| `notice_text` | Safe-harbor notice. |
| `notice_served_at` | Safe-harbor service date. |
| `motion_filed_at` | Sanctions-motion filing date. |
| `correction_text` | Optional withdrawal or correction. |
| `motion_text` | Motion and requested sanction. |
| `opposition_text` | Opposition and asserted sanction limits. |
| `expected_granted` | Expected grant or denial. |
| `expected_sanction_type` | `none`, `admonition`, `non_monetary_directive`, `monetary_penalty`, or `fee_shift`. |
| `expected_sanction_amount` | Expected amount for a monetary sanction. |
| `expected_reason_tags` | Reason categories used by deterministic explanation scoring. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Explanation of the legal boundary tested by the row. |

## Scoring

The scorer accepts exactly one `decide_rule11_motion` call and requires `motion_index` zero.  It validates `granted`, `sanction_type`, `sanction_amount`, `sanction_detail`, and `reasoning` before comparing the grant decision and sanction to the fixture label.  It then combines those comparisons with the acceptance fields to determine outcome correctness.  In deterministic production mode, a successful step sets both `step_accepted` and `lean_accepted`.  Counterfactual model mode obtains `lean_accepted` from `apply_decision`.

Denied motions use an empty or omitted `sanction_type`, a zero or omitted `sanction_amount`, and an empty `sanction_detail`.  Granted motions require a recognized sanction type and nonempty detail, with a positive amount for `monetary_penalty` or `fee_shift` and a zero or omitted amount for `admonition` or `non_monetary_directive`.  The summary records aggregate rates and slices by reason tag, issue family, tier, movant, and expected sanction type.

## Execution and Prompt Selection

The default command executes the deterministic action supplied by the judge opportunity.  The `--counterfactual-model` flag removes that action and obtains a decision from `--model`.  Candidate prompt execution uses counterfactual-model mode.  The evaluator records prompt source, prompt name, execution mode, per-fixture records, and aggregate metrics in `results.jsonl` and `summary.json`.

```bash
adc eval judge-rule11 \
  --counterfactual-model \
  --opportunity-prompt-file evals/adc/judge/rules/rule11/sanctions/prompts/candidate-v2.md \
  --opportunity-prompt-name candidate-v2 \
  --out-dir evals/out/adc/judge/rule11-candidate-v2
```
