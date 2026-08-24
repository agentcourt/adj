# Rule 56 Judge Eval Plan

## Scope

This eval measures the judge's `decide_rule56_motion` behavior.  Each fixture builds a pretrial ADC state with a Rule 56 motion, opposition, and optional reply in the docket.  The runner obtains the real judge opportunity from Lean, asks the judge model for one `decide_rule56_motion` tool call, applies the decision through Lean, and scores the disposition and reason tags.

The first fixture set focuses on the core Rule 56 boundary.  It covers undisputed grants, denials based on credibility disputes, denials based on competing reasonable inferences, authentication disputes, unsupported damages theories, movant burden failures, legal bars, and partial dispositions.  False grants receive the highest practical concern because they remove trial access or narrow the case beyond the record.

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
| `opposition_text` | Nonmovant response. |
| `reply_text` | Optional movant reply. |
| `expected_disposition` | `granted`, `denied`, or `partial`. |
| `expected_surviving_issues` | Issues that should remain when the expected disposition is partial. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Human-readable explanation of the fixture boundary. |

## Prompt Iteration

Candidate v1 emphasizes motion scope, so a damages-category motion does not become a whole-claim judgment.  Candidate v2 keeps that scope rule and adds a limit on partial rulings: use `partial` for severable claim elements, defenses, or damages categories, and deny a motion resting on an evidentiary fragment when the full record creates a reasonable competing inference on the material issue.  Measured results are in [Rule 56 Analysis](analysis.md).
