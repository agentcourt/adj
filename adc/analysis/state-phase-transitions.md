# State and phase transitions

Mermaid source: [`state-phase-transitions.mmd`](state-phase-transitions.mmd).

This diagram isolates two ordered transition systems in the engine: case status and trial phase.  Case status has only the listed edges.  A case can move from `filed` to `pretrial` or `closed`, from `pretrial` to `trial` or `closed`, from `trial` to `judgment_entered` or `closed`, and from `judgment_entered` to `closed`.  The note on the right states the rule that matters: any status transition outside those edges is invalid.

Trial phase has a ranked order from `none` through `post_verdict`.  The canonical path is `voir_dire`, `openings`, `plaintiff_case`, `plaintiff_evidence`, `defense_case`, `defense_evidence`, `plaintiff_rebuttal`, `plaintiff_rebuttal_evidence`, `defense_surrebuttal`, `defense_surrebuttal_evidence`, `charge_conference`, `closings`, `jury_charge`, `deliberation`, `verdict_return`, and `post_verdict`.  The phase rule allows the engine to stay in the same phase or move to a later-ranked phase.  It rejects backward moves.

The additional notes capture constraints that do not fit into the raw arrows.  The engine rejects advance to `voir_dire` unless the candidate panel is full.  It rejects advance to `openings` unless the jury is empaneled.  It rejects advance to `deliberation` unless the judge has delivered jury instructions.  In a bench trial, it rejects advance to `post_verdict` unless the docket contains a bench opinion.  In a jury trial, it rejects advance to `post_verdict` unless the record already contains a verdict or a hung-jury notice.  Rule 56 is not a phase or status transition at all.  It is a pretrial opportunity window whose pass semantics and reopening rules are separate.  Deliberation is also annotated separately because the engine derives a verdict or a hung jury from rounds of individual sworn-juror votes before the case reaches `post_verdict`.

This diagram is a transition reference.  It does not show the full opportunity generator.  It shows the legal edges and the small set of guard conditions that matter when the engine changes the case phase or status.
