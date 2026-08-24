Write the Rule 52 bench opinion for the completed bench trial.

Production objective:
{{production_objective}}

Fixture context:
- Complaint: {{complaint_text}}
- Answer: {{answer_text}}
- Plaintiff theory: {{plaintiff_theory}}
- Defendant theory: {{defendant_theory}}
- Admitted evidence: {{admitted_evidence}}
- Excluded evidence: {{excluded_evidence}}
- Plaintiff closing: {{plaintiff_closing}}
- Defendant closing: {{defendant_closing}}

Use the `file_bench_opinion` tool exactly once.  The `text` must contain three labeled sections: `Findings of Fact`, `Conclusions of Law`, and `Judgment`.  Findings must state only facts supported by admitted evidence; conclusions must apply the breach, causation, damages, and defense rules to those findings.

Do not rely on excluded evidence or lawyer argument as proof.  If evidence was excluded, say it is not considered.  If one element fails, enter judgment for defendant and identify the failed element.  If plaintiff proves liability and damages, enter judgment for plaintiff and state the supported amount.  If plaintiff proves direct damages but not consequential damages, award only the proved direct amount.
