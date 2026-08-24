# Rule 60 Judge Eval Plan

## Scope

This eval measures the judge's resolution of Rule 60 motions for relief from judgment.  The first version targets `resolve_rule60_motion`, because the current Lean opportunity exposes a required judge decision after judgment when a Rule 60 motion appears in the case record.  The runner builds a judgment-entered ADC state, obtains the real Lean opportunity, asks the production prompt or an eval-local prompt candidate for the tool call, validates the decision through `apply_decision`, and executes the returned action through Lean `step`.

The fixture state records the judgment, the Rule 60 motion, the opposition, and decision traces that make the motion visible to Lean.  Default-judgment rows also record a default judgment trace, because excusable-neglect and void-judgment rows need that posture.  The eval uses text fixtures for the motion record but keeps the ADC state deterministic, so failures can be traced to the judge decision rather than state generation.

## Fixture Set

The first fixture file contains 16 rows across three difficulty tiers.  The set balances eight expected grants and eight expected denials, with rows for mistake or excusable neglect, void judgment, satisfaction, newly discovered evidence, fraud, timeliness, prospective inequity, and ordinary reargument.  The new-evidence grant row states diligence, materiality, and likely effect after iteration showed that diligence alone did not support Rule 60(b)(2) relief.

| Theme | Scored Boundary |
|---|---|
| Excusable neglect after default | Relief can be granted when neglect is excusable and the party acts promptly |
| Ordinary reargument | Rule 60 does not reopen credibility or evidence-weight disputes |
| Void judgment | Lack of valid service supports relief from a default judgment |
| Satisfied judgment | Full payment or satisfaction supports limited relief |
| Newly discovered evidence | Relief requires evidence unavailable earlier despite diligence and material to the result |
| Fraud or misconduct | Fraud affecting the judgment supports relief, but impeachment-only or known issues do not |
| Timeliness | Rule 60(b)(1) timing and reasonable-time limits can bar relief |
| Extraordinary prospective change | Prospective enforcement can become inequitable after an external legal change |

## Scoring

The scorer requires exactly one `resolve_rule60_motion` tool call with `motion_index: 0`, a `granted` boolean, and a nonempty `relief_summary`.  It checks grant or denial, required concepts, prohibited concepts, reason tags, Lean opportunity acceptance, and Lean step acceptance.  The summary reports total accuracy, weighted accuracy, invalid rate, false-grant rate, false-denial rate, Lean rejection, and slices by issue family, tier, expected disposition, and reason tag.

The prohibited-concept scorer is negation-aware.  Rule 60 denials often state the absent ground, such as "no extraordinary circumstances," "does not show fraud," or a list of grounds that are not shown.  The scorer treats those forms as correct denials rather than assertions of the prohibited ground, while still flagging a summary that relies on an improper ground affirmatively.

## Prompt Iteration

Candidate v1 gives the judge a compact Rule 60 checklist: grant only recognized grounds supported by the motion record, deny ordinary reargument and untimely motions, and leave damages and liability alone.  Measured results are in [Rule 60 Analysis](analysis.md).

## Next Extensions

The next Rule 60 set should add harder timeliness and finality rows rather than repeating the current boundaries.  Useful additions include multiple Rule 60 motions, partial relief, mistake versus legal error, independent action language, fraud on the court, voidness based on subject-matter jurisdiction, and prospective relief with mixed monetary and nonmonetary terms.  Rows with unavailable opportunities should live in a separate transition eval, because this runner assumes Lean has produced a required judge Rule 60 opportunity.
