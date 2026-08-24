# Rule 11 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `decide_rule11_motion`.  The fixture set contains 16 Rule 11 motions across frivolous legal contentions, unsupported factual contentions, improper purpose, reasonable legal extensions, factual allegations likely to have discovery support, weak merits positions, safe-harbor defects, timely correction, discovery exclusions, and sanction proportionality.  The eval uses real ADC state, the real Lean opportunity, the production judge prompt or an eval-local prompt candidate, deterministic scoring, and Lean application of each returned payload.

The scorer treats sanction fields as part of the decision.  A correct grant-or-denial result fails if the payload places sanctions on a denied motion, omits a required sanction type on a grant, omits a positive amount for a monetary sanction, or gives a sanction type that does not match the fixture's expected sanction.  This scoring choice matches Lean's Rule 11 validation and captures failures that otherwise look like legally sound reasoning with an unusable tool payload.

## Results

| Prompt | Run | Correct | Grant Correct | Reason Matches | Invalid | False Grants | False Denials | Sanction Mismatches | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 16 | 0 | 0 | 0 | 0 | 1.000 |
| production | live 16 | 5 | 5 | 5 | 11 | 0 | 0 | 0 | 0.313 |
| candidate-v1 | dry 16 | 16 | 16 | 16 | 0 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 16 | 15 | 16 | 16 | 0 | 0 | 0 | 1 | 0.938 |
| candidate-v2 | dry 16 | 16 | 16 | 16 | 0 | 0 | 0 | 0 | 1.000 |
| candidate-v2 | live 16 | 16 | 16 | 16 | 0 | 0 | 0 | 0 | 1.000 |

## Findings

Production's dominant failure was payload validity, not Rule 11 substance.  Nine denied motions returned `sanction_type: "none"` plus explanatory no-sanctions text in `sanction_detail`, which Lean rejects because a denied Rule 11 motion cannot include sanctions.  Two granted rows selected `fee_shift` without a positive amount, even though the fixture record either expected a nonmonetary sanction or lacked a fee amount.

Candidate v1 removed the invalid-payload cluster and preserved every grant-or-denial decision.  Its only miss was `r11-001`, where it granted a motion for a claim foreclosed by an attached release but chose a withdrawal directive rather than the expected admonition.  That result showed the prompt needed a sharper distinction between a first limited legal defect and a correction-focused nonmonetary directive.

Candidate v2 added that distinction.  It uses `admonition` for a first limited legal defect, including a released claim or repleaded dismissed claim, unless the motion record specifically asks for a correction directive.  It reserves `non_monetary_directive` for records that request withdrawal, amendment, or correction of a factual pleading or representation, and it keeps fee shifting limited to records with a tied fee amount.

## Recommendation

Candidate v2 is the best measured Rule 11 opportunity prompt on this fixture set.  It improves production from 5/16 to 16/16 live and removes every invalid sanction payload observed in the production run.  Keep the prompt eval-local until the project makes a separate production prompt update decision, but treat candidate v2 as the current best text for Rule 11 prompt iteration.

## Next Work

The next Rule 11 set should add harder sanction-proportionality rows before any production prompt update.  Useful additions include repeated misconduct, mixed valid and invalid contentions in one filing, attorney-versus-party sanction allocation, and records where fees are requested but only partly tied to the Rule 11 violation.  Those rows would test whether candidate v2 preserves restraint once the sanction choice becomes less mechanical.
