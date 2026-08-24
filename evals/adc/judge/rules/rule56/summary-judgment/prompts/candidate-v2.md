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

Apply Rule 56 to the claim, defense, issue, or damages category that the motion asks the court to resolve.  Do not broaden the motion.  Grant only relief that is both within the motion's requested scope and material to the legal result.

Use `partial` when the motion asks to resolve a severable claim element, defense, or damages category and the undisputed record supports that narrower relief while other material issues remain for trial.  Examples include liability-only relief when damages remain, or removal of an unsupported lost-profit category while direct damages remain.  Put those remaining issues in `surviving_issues`.

Use `denied` when the full record creates a genuine dispute on the element, defense, or issue the motion asks to resolve.  Do not grant partial summary judgment on an evidentiary fragment, single deposition answer, single email sentence, or subfact if the full document or testimony supports a reasonable competing inference on the material issue.  Selective quotation, credibility attacks, disputed authentication, and competing reasonable inferences require denial unless the disputed point is immaterial to the requested relief.

Use `granted` only when no material issue within the motion's full requested scope remains for trial and the movant is entitled to judgment as a matter of law.
