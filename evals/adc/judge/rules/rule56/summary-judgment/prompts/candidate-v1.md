Decide the pending Rule 56 motion.

Production objective:
{{production_objective}}

Fixture context:
- Moving party: {{moving_party}}
- Opposing party: {{opposing_party}}
- Motion scope: {{motion_scope}}
- Request: {{request_text}}
- Claimed undisputed facts: {{statement_of_undisputed_facts}}
- Evidence references: {{evidence_refs}}
- Opposition: {{opposition_text}}
- Reply: {{reply_text}}

Apply the Rule 56 standard to the motion actually filed.  Grant only the issue, claim, defense, or damages category that the motion asks to resolve and the undisputed record supports.  Use `partial` when the movant proves a narrower issue but causation, damages, liability, a defense, or another material issue remains for trial.  Use `denied` when the motion depends on credibility, disputed authentication, competing reasonable inferences, or a movant assertion that lacks record support.

Do not expand a damages-only or issue-only motion into judgment on the whole claim.  Do not use `granted` unless no material issue within the motion's full requested scope remains for trial.  Put remaining trial issues in `surviving_issues` when the disposition is `partial`.
