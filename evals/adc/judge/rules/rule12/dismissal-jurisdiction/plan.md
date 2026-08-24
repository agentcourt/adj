# Rule 12 Judge Eval Plan

## Scope

This eval measures judge decisions on Rule 12 motions and closely related jurisdiction dismissals.  It uses real ADC state, the Lean judge opportunity, the `decide_rule12_motion` tool schema, deterministic scoring, and eval-local opportunity prompt candidates.  The first version focuses on disposition, ground, amendment posture, prejudice posture, missing claim elements, jurisdiction-basis rejection, and standing component fields.

The eval covers pleading sufficiency, subject-matter jurisdiction, standing, ripeness, and mootness.  These decisions can close a case before evidence development, so the fixture set gives high weight to false dismissals and wrong prejudice decisions.  The first implementation keeps the state compact: one complaint, one Rule 12 motion, one opposition, and an optional reply.

## Fixture Set

The first fixture file contains 18 rows across three difficulty tiers.  Tier 1 rows test clean grants and denials, tier 2 rows test contextual pleading and jurisdiction issues, and tier 3 rows test adversarial framing.  Each row states the motion ground, the complaint, the motion, the opposition, expected tool fields, reason tags, severity, and a short note about the boundary being tested.

| Category | Rows | Scored Fields |
|---|---:|---|
| Failure to state a claim | 7 | Disposition, `leave_to_amend`, `with_prejudice`, `missing_elements` |
| Subject-matter jurisdiction | 3 | Disposition, `leave_to_amend`, `jurisdiction_basis_rejected` |
| Standing | 4 | Disposition, `leave_to_amend`, missing standing components |
| Ripeness | 2 | Disposition and amendment posture |
| Mootness | 2 | Disposition and amendment posture |

The paired rows distinguish pleading defects from factual disputes.  The judge should deny a Rule 12 motion when the complaint pleads concrete facts and the defendant contests proof, credibility, or later evidentiary support.  The judge should grant a motion when the complaint omits a required element, omits jurisdictional facts, pleads no standing component, presents only a contingent dispute, or admits that the requested relief was already satisfied before filing.

## Scoring

The scorer requires exactly one `decide_rule12_motion` tool call with `motion_index` 0 and the fixture ground.  It marks the disposition incorrect if the grant or denial differs from the fixture, if `with_prejudice` differs, if `leave_to_amend` differs, or if a granted motion omits required ground-specific fields.  It reports false dismissals, false denials, posture mismatches, invalid responses, weighted accuracy, and slices by reason tag, issue family, ground, and tier.

Ground-specific scoring checks the fields that ADC expects the judge to supply.  Failure-to-state-a-claim rows require the expected missing elements, with equivalent labels accepted when the model identifies the same element in more specific wording.  Jurisdiction rows require the rejected basis, and standing rows require the missing component booleans.

## Prompt Iteration

Candidate v1 addresses mootness amendment posture by stating when leave to amend should be refused.  Candidate v2 narrows that rule to mootness cases where the complaint admits full prefiling satisfaction of all requested relief and seeks no remaining live relief, which leaves curable standing, ripeness, and diversity-amount defects amendable.  Measured results are in [Rule 12 Analysis](analysis.md).

## Next Extensions

The next fixture expansion should add paired Rule 12 and Rule 56 themes.  The same factual setting should appear once as a pleading-sufficiency row and once as an evidentiary-sufficiency row, which tests whether the judge applies the procedural standard instead of using a general case-strength judgment.  The expansion should also add court-driven subject-matter jurisdiction screening if ADC exposes a separate judge opportunity for dismissal without a party Rule 12 motion.
