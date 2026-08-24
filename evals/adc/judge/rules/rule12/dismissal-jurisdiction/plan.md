# Rule 12 Evaluation Plan

## Scope

This evaluation measures decisions produced for `decide_rule12_motion`.  Each fixture builds a filed-stage ADC state with one complaint, one Rule 12 motion, one opposition, and an optional reply.  The runner obtains the judge opportunity, executes the opportunity, applies the decision through Lean, and scores disposition, procedural posture, ground-specific fields, and explanation tags.

## Fixture Coverage

The fixture file contains eighteen rows, with six rows in each of three difficulty tiers.  Ten rows expect dismissal, and eight expect denial.  Severity 5 applies to the eight expected denials and the one expected dismissal with prejudice, while every leave-to-amend row has severity 3.

| Ground | Rows | Ground-specific fields |
| --- | ---: | --- |
| `failure_to_state_a_claim` | 7 | `missing_elements` |
| `lack_subject_matter_jurisdiction` | 3 | `jurisdiction_basis_rejected` |
| `no_standing` | 4 | `injury_missing`, `traceability_missing`, `redressability_missing` |
| `not_ripe` | 2 | Disposition and posture. |
| `moot` | 2 | Disposition and posture. |

The paired fixtures distinguish a pleading defect from a dispute over proof, credibility, or evidentiary support.  Dismissal is expected when the complaint omits a required claim element, jurisdictional basis, standing component, or live controversy.  Denial is expected when the complaint pleads the required facts and the motion contests their truth or likely proof.

## Fixture Shape

| Field | Meaning |
| --- | --- |
| `id` | Stable row identifier. |
| `tier` | Difficulty tier. |
| `issue_family` | Procedural issue used for summary slices. |
| `case_theme` | Short factual setting placed in ADC state. |
| `ground` | Rule 12 or jurisdiction ground presented by the motion. |
| `complaint_text` | Allegations evaluated under the filed ground. |
| `motion_text` | Movant's dismissal argument. |
| `opposition_text` | Opposition to dismissal. |
| `reply_text` | Optional reply. |
| `jurisdictional_allegations` | Optional structured jurisdiction facts placed in state. |
| `expected_disposition` | `granted` or `denied`. |
| `expected_with_prejudice` | Expected prejudice posture. |
| `expected_leave_to_amend` | Expected amendment posture. |
| `expected_missing_elements` | Claim elements required from a granted pleading-sufficiency ruling. |
| `expected_jurisdiction_basis_rejected` | Jurisdictional basis required from a granted jurisdiction ruling. |
| `expected_injury_missing` | Whether injury must be marked missing on a granted standing ruling. |
| `expected_traceability_missing` | Whether traceability must be marked missing on a granted standing ruling. |
| `expected_redressability_missing` | Whether redressability must be marked missing on a granted standing ruling. |
| `expected_reason_tags` | Reason categories used by deterministic explanation scoring. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Explanation of the procedural boundary tested by the row. |

## Scoring

The scorer accepts exactly one `decide_rule12_motion` call and requires `motion_index` zero and the fixture's normalized ground.  It rejects an unrecognized disposition or empty reasoning, then compares disposition, prejudice, and leave to amend with the fixture label.  For a granted motion, it also checks the ground-specific fields listed above.  A denied motion uses the disposition and posture checks.

The evaluator accepts equivalent wording for expected missing elements and jurisdictional bases.  Outcome correctness also requires Lean acceptance and an accepted runner step.  The summary records false dismissals, false denials, posture mismatches, invalid responses, aggregate accuracy, weighted accuracy, and slices by reason tag, issue family, ground, and tier.

## Execution and Prompt Selection

The command resolves the configured court and model, then gives the resulting response client and ADC state to the opportunity runner.  The runner uses the objective supplied by the judge opportunity by default.  A selected template replaces that objective after fixture substitution.  The evaluator records prompt source, prompt name, per-fixture records, and aggregate metrics in `results.jsonl` and `summary.json`.

```bash
adc eval judge-rule12 \
  --opportunity-prompt-file evals/adc/judge/rules/rule12/dismissal-jurisdiction/prompts/candidate-v2.md \
  --opportunity-prompt-name candidate-v2 \
  --out-dir evals/out/adc/judge/rule12-candidate-v2
```
