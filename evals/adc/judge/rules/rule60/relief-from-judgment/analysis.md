# Rule 60 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Scope

This analysis covers the judge eval for `resolve_rule60_motion`.  The fixture set contains 16 judgment-entered postures with pending Rule 60 motions and oppositions.  The eval uses real ADC state, the real Lean opportunity, the production judge prompt or an eval-local prompt candidate, deterministic scoring, opportunity validation, and a Lean step that executes the accepted Rule 60 action.

The scorer checks both the tool payload and the accepted engine path.  It requires the correct `motion_index`, the expected grant or denial, a reason-bearing `relief_summary`, required concepts, prohibited-concept absence with negation handling, reason tags, Lean acceptance, and step acceptance.  The negation handling was needed because correct Rule 60 denials often state the absent ground, such as "no extraordinary circumstances" or "does not show fraud."

## Results

| Prompt | Run | Correct | Grant Correct | Required Correct | Prohibited Correct | Reason Correct | Invalid | Lean Rejected | Step Rejected | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| production | live 16, rescored | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | dry 16 | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 16, rescored | 16 | 16 | 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |

## Findings

Production made the correct Rule 60 grant or denial decision on all 16 live rows.  The initial live score was lower because the scorer treated correct denial language as prohibited-ground reliance and missed ordinary synonyms such as "lack of valid service," "newly discovered," "not reasonably obtainable before judgment," and "amend the judgment to correct."  After scorer correction, the saved production responses scored 16/16 with no invalid payloads, no Lean rejections, and no step rejections.

The fixture set was also corrected during iteration.  The first new-evidence grant row stated diligence and unavailability, but it did not state materiality or likely effect on the judgment.  The production model denied that row for a real Rule 60(b)(2) reason, so the fixture was revised to state that the logs bear directly on the access issue, liability, and damages.

Candidate v1 matched production on the measured decision behavior.  It made all 16 grant or denial decisions correctly, and its lower pre-rescore score came from thinner but legally adequate `relief_summary` wording.  The candidate prompt remains useful as an eval-local checklist, but the measured results do not justify copying it into production.

## Recommendation

Do not update the production Rule 60 opportunity prompt from this eval alone.  The production prompt reached 16/16 after the deterministic scorer and the under-specified fixture were corrected.  The next Rule 60 work should add harder rows before any prompt promotion decision, especially multiple motions, mixed prospective and monetary relief, jurisdictional voidness, fraud on the court, and Rule 60(b)(6) attempts to avoid Rule 59 or appeal deadlines.

## Next Work

The next Rule 60 set should add harder finality rows, including multiple motions, partial relief, mistake versus legal error, independent action language, fraud on the court, jurisdictional voidness, and prospective relief with mixed monetary and nonmonetary terms.  Rows where Lean should produce no Rule 60 opportunity should live in a separate transition eval because this runner assumes an available required judge opportunity.  The scorer should keep its negation-aware prohibited-concept checks, since correct denials often name the absent Rule 60 ground.
