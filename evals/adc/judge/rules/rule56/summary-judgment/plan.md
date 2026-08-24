# Rule 56 Judge Eval Plan

## Procedure

This eval measures the judge's `decide_rule56_motion` behavior.  Each fixture builds a pretrial ADC state with a Rule 56 motion, opposition, and optional reply in the docket.  The runner obtains the current judge opportunity from Lean, executes one judge turn through the ADC prompt and tool path, applies the accepted decision, and scores the disposition and surviving issues.

The fixture text appears in docket entries rather than a separate evaluator-only message.  The model therefore receives the motion record through the same judge view used by ADC, while an eval-local opportunity template may restate selected fixture fields.  Each result records the state, view, opportunity, prompt input, response exchanges, final state, provider accounting, extracted payload, and scoring fields.

## Fixture Shape

| Field | Meaning |
|---|---|
| `id` | Stable row identifier. |
| `tier` | Difficulty level, with tier 3 for adversarial phrasing and close boundaries. |
| `issue_family` | Summary slice for the legal boundary tested. |
| `case_theme` | Short factual setting placed in the case caption and docket. |
| `moving_party` | Party filing the Rule 56 motion. |
| `motion_scope` | Claim, defense, element, issue, or damages category requested. |
| `request_text` | Motion request. |
| `statement_of_undisputed_facts` | Movant's asserted undisputed record facts. |
| `evidence_refs` | Record references supporting or contesting the asserted facts. |
| `opposition_text` | Nonmovant response. |
| `reply_text` | Optional movant reply. |
| `expected_disposition` | `granted`, `denied`, or `partial`. |
| `expected_surviving_issues` | Expected remaining issues when the disposition is partial. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Human-readable explanation of the fixture boundary. |

## Scoring

The scorer requires exactly one `decide_rule56_motion` call with `motion_index: 0`, a valid disposition, and nonempty reasoning.  It compares the disposition and the normalized set of surviving issues with the fixture expectations.  It also requires Lean acceptance, successful action execution, and a completed turn without a procedural error.

Reason-tag matching is reported separately from outcome correctness.  Aggregate error classification distinguishes false grants, false denials, and partial mismatches, with a grant or partial disposition against an expected denial counted as a false grant.  The summary reports accuracy, severity-weighted accuracy, invalid rate, false-grant and false-denial rates, and slices by reason tag, issue family, tier, and moving party.

## Prompt Selection

The runner uses the objective supplied by the Lean opportunity by default.  [Candidate v1](prompts/candidate-v1.md) emphasizes motion scope and severable partial relief, while [candidate v2](prompts/candidate-v2.md) also addresses selective quotations and evidentiary fragments.  Both templates receive the moving and opposing parties, requested scope, motion text, asserted facts, evidence references, opposition, and reply, while expected scoring fields remain scorer inputs.

## Outputs

The runner writes one JSON object per fixture to `results.jsonl` and an aggregate `summary.json` in the selected output directory.  The `--limit` option selects the first specified number of fixture rows, while model, court, timeout, temperature, prompt catalog, and Lean engine options control execution.  The command always uses the opportunity executor.  Rule 56 has no rescore mode.
