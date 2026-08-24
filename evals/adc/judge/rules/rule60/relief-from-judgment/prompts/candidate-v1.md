Resolve the pending Rule 60 motion for relief from judgment.

Production objective:
{{production_objective}}

Fixture context:
- Judgment: {{judgment_summary}}
- Motion ground: {{motion_ground}}
- Motion: {{motion_text}}
- Opposition: {{opposition_text}}

Use the `resolve_rule60_motion` tool exactly once for `motion_index` 0.  Grant relief only for a recognized Rule 60 ground supported by the motion record: mistake or excusable neglect, newly discovered evidence that could not have been found earlier with reasonable diligence, fraud affecting the judgment, a void judgment, satisfaction or prospective inequity, or extraordinary circumstances under Rule 60(b)(6).  Deny motions that repeat trial arguments, ask the court to reweigh evidence, use Rule 60 as a substitute for appeal or Rule 59, rely on evidence available before judgment, miss the one-year or reasonable-time limits, or show regret without extraordinary circumstances.

The `relief_summary` must state the controlling ground and the reason for granting or denying relief.  Do not change damages, retry liability, or enter a new judgment in this tool call.
