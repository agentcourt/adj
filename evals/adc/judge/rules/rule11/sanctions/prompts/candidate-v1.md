Decide the pending Rule 11 motion.

Production objective:
{{production_objective}}

Fixture context:
- Movant: {{movant}}
- Target party: {{target_party}}
- Challenged filing: {{challenged_filing}}
- Filing text: {{filing_text}}
- Safe-harbor notice: {{notice_text}}
- Notice served: {{notice_served_at}}
- Motion filed: {{motion_filed_at}}
- Correction: {{correction_text}}
- Motion: {{motion_text}}
- Opposition: {{opposition_text}}

Grant Rule 11 only for a filing abuse: a legal contention not warranted by existing law or a nonfrivolous extension argument, a factual contention with no evidentiary support after reasonable inquiry, a denial that contradicts records available before filing, or a filing presented for improper purpose.  Deny sanctions for weak merits positions, disputed inferences from identified records, reasonable extension arguments, allegations likely to have support after discovery, reasonable prefiling inquiry, safe-harbor defects, timely correction or withdrawal, and discovery disputes that belong under Rule 37.

Payload rules are strict.  A denied Rule 11 motion must set `granted` to false, set `sanction_detail` to the empty string, and omit `sanction_type` and `sanction_amount`.  A granted Rule 11 motion must set `granted` to true, include a nonempty `sanction_type`, include a nonempty `sanction_detail` identifying the filing defect, and include a positive `sanction_amount` only for `monetary_penalty` or `fee_shift`.

Choose the least severe sanction sufficient for deterrence.  Use `admonition` for a limited legal defect, `non_monetary_directive` for withdrawal or correction, `monetary_penalty` for improper purpose or repeated abuse, and `fee_shift` only when the fixture record identifies a fee amount tied to the motion.  Explain the decisive Rule 11 reason in `reasoning`.
