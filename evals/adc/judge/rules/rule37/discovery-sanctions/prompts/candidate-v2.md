Decide the pending Rule 37 motion.

Production objective:
{{production_objective}}

Fixture context:
- Movant: {{movant}}
- Target party: {{target_party}}
- Discovery type: {{discovery_type}}
- Request: {{request_text}}
- Response: {{response_text}}
- Meet and confer: {{meet_and_confer_text}}
- Motion: {{motion_text}}
- Opposition: {{opposition_text}}
- Reply: {{reply_text}}

Grant Rule 37 relief when the record shows a concrete discovery failure: no interrogatory response, an evasive or incomplete response, unsupported boilerplate objections, refusal to produce responsive files, failure to supplement required disclosures, or violation of a prior discovery order.  Deny the motion when the response is complete, the objection is substantially justified, the request is overbroad or disproportionate, the defect has been cured without prejudice, the motion is premature, or unanswered RFAs are already admitted under Rule 36.  Do not use Rule 37 to decide the merits of the claim.

Every `decide_rule37_motion` payload must include `motion_index`, `granted`, `sanction_type`, and `reasoning`.  Use `motion_index` 0.  Set `sanction_type` to `none` whenever `granted` is false, including fee-only motions and motions denied after cure, completion, privilege, proportionality, overbreadth, premature filing, or Rule 36 admission.

Set `sanction_type` to `fees` only when the motion is granted, the failure was not substantially justified or harmless, and the record gives a requested fee amount.  Use that requested amount when it is identified as reasonable.  Set `sanction_type` to `none` when a grant is warranted but fees would be unjust.  Do not omit `sanction_type`; do not use `fees` on a denied motion.

Explain the discovery failure or reason for denial in `reasoning`, and put any order to compel or fee award in `order_text`.
