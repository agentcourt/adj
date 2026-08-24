# Rule 37 Evaluation Plan

## Scope

This evaluation measures decisions produced for `decide_rule37_motion`.  Each fixture builds a pretrial ADC state in the discovery phase with a discovery request, response record, optional meet-and-confer text, motion, opposition, and optional reply.  The runner obtains the judge opportunity, executes its action through Lean, and scores the legal outcome and sanction payload.

## Fixture Coverage

The fixture file contains sixteen rows across three difficulty tiers.  Seven rows expect a grant, and nine expect a denial.  The `severity` field weights each row when the summary computes weighted accuracy.

The fixtures cover interrogatories, requests for production, initial disclosures, and requests for admission.  They test missing, complete, and evasive responses; privilege and work-product objections; overbreadth and proportionality; cure; disclosure and discovery-order violations; premature motions; and the separation between compelled relief and fees.  The expected sanction types are `fees` and `none`.

## Fixture Shape

| Field | Meaning |
| --- | --- |
| `id` | Stable row identifier. |
| `tier` | Difficulty tier. |
| `issue_family` | Discovery issue used for summary slices. |
| `case_theme` | Short factual setting placed in ADC state. |
| `movant` | Party seeking Rule 37 relief. |
| `target_party` | Party whose discovery conduct is challenged. |
| `discovery_type` | `interrogatories`, `rfp`, `rfa`, or `initial_disclosures`. |
| `set_index` | Discovery-set index placed in ADC state. |
| `request_text` | Discovery request or disclosure obligation. |
| `response_text` | Response, objection, cure, or nonresponse record. |
| `meet_and_confer_text` | Optional meet-and-confer chronology. |
| `motion_text` | Motion and requested relief. |
| `opposition_text` | Opposition and asserted sanction limits. |
| `reply_text` | Optional reply. |
| `expected_granted` | Expected grant or denial. |
| `expected_sanction_type` | `none` or `fees`. |
| `expected_sanction_amount` | Expected amount when fees are awarded. |
| `expected_reason_tags` | Reason categories used by deterministic explanation scoring. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Explanation of the discovery boundary tested by the row. |

## Scoring

The scorer accepts exactly one `decide_rule37_motion` call and requires `motion_index` zero.  It validates `granted`, `sanction_type`, `sanction_amount`, and `reasoning` before comparing the grant decision and sanction to the fixture label.  It uses `order_text` together with `reasoning` for deterministic explanation matching.

A denied motion must use `sanction_type: none`, and a nonzero amount is invalid with that sanction type.  A fee award requires `sanction_type: fees` and a positive amount, while a granted motion may use `none` when fees would be unjust.  Outcome correctness also requires both acceptance fields, and the summary records aggregate rates and slices by reason tag, issue family, tier, movant, and expected sanction type.  In deterministic production mode, a successful step sets both `step_accepted` and `lean_accepted`.  Counterfactual model mode obtains `lean_accepted` from `apply_decision`.

## Execution and Prompt Selection

The default command executes the deterministic action supplied by the judge opportunity.  The `--counterfactual-model` flag removes that action and obtains a decision from `--model`.  Candidate prompt execution uses counterfactual-model mode.  The evaluator records prompt source, prompt name, execution mode, per-fixture records, and aggregate metrics in `results.jsonl` and `summary.json`.

```bash
adc eval judge-rule37 \
  --counterfactual-model \
  --opportunity-prompt-file evals/adc/judge/rules/rule37/discovery-sanctions/prompts/candidate-v2.md \
  --opportunity-prompt-name candidate-v2 \
  --out-dir evals/out/adc/judge/rule37-candidate-v2
```
