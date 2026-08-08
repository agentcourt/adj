# Lean control flow

Mermaid source: [`lean-complete-flow.mmd`](lean-complete-flow.mmd).

This diagram describes the full Lean-facing control surface.  The entry point is a tagged request.  Lean handles `initialize_case`, `role_view`, `next_opportunity`, `agenda`, `apply_decision`, and `step`.  The diagram separates those request kinds because they play different roles.  `initialize_case` validates the complaint summary, attachments, and jurisdiction allegations before it seeds the complaint attachments, docket, and traces.  `role_view` partitions the case by role.  Judges and clerks see the full case.  Parties see the public case, visible files, jurisdiction allegations, and docket.  Jurors and other nonparty roles see jury-safe redactions.

The opportunity path begins with `collectTurnFacts`.  Lean then selects candidate opportunities from the current case status: filed, pretrial, trial, or post-judgment.  Jurisdiction-dismissal candidates appear only when the selected court profile enables screening.  Lean then assigns opportunity ids, filters passed opportunities, and returns the lowest-priority open opportunity or a terminal result.  The separate `agenda` request feeds the same finalization path.

The decision path begins with `applyDecision`.  Lean validates the state version, opportunity id, role, allowed tool, and payload constraints.  A failure returns a retryable `StepErr` with an actor-facing correction message.  A successful pass records the pass and may close the current Rule 56 window.  A successful tool call emits an accepted action for runner execution.

The action path begins with `step`.  After a schema check, Lean dispatches the action into one of four families.  Filing handles case initialization, complaint filing, answer or amendment, and trial-mode resolution.  Pretrial handles Rule 11, Rule 12, disclosures, discovery, Rule 37, Rule 56, and the pretrial order.  Trial now includes jury configuration, full voir dire or skip-voir-dire empanelment, openings, separate presentation and evidence phases for each side, closings, a jury-versus-bench split, deliberation rounds, individual juror votes, bench opinions, verdict or hung jury, poll, and judgment.  Post-judgment handles Rules 59 and 60 plus court-profile and limit-enforcement actions.

The notes at the bottom mark three constraints that matter in practice.  Rule 56 is a windowed opportunity family, not a phase change.  Subject-matter-jurisdiction dismissal depends on the court profile.  In jury trials, the result comes from individual sworn-juror votes rather than a separate foreperson actor.

This diagram is the most detailed of the set.  It is still a control-flow summary, not a complete formal specification.  It shows the major request routes and action families that shape a live case.
