# Lean request loop

Mermaid source: [`lean-simple-flow.mmd`](lean-simple-flow.mmd).

This diagram is the short version of the Lean runtime.  The runner holds the current authoritative `CourtState` and uses a small request vocabulary against the Lean engine.  `initialize_case` seeds the complaint, attachments, and court profile.  `role_view` returns the role-scoped case view.  `next_opportunity` derives one current opportunity.  `apply_decision` validates a pass or legal act.  `step` applies the resulting action state and returns the updated state.

The lower loop shows the ordinary execution cycle.  An agent sees the current role view and one constrained opportunity, submits a pass or decision, and Lean validates it.  An invalid submission produces an error with `actor_message` and leaves the state unchanged.  A valid submission becomes an accepted action or a recorded pass.  The runner executes the accepted act, receives the resulting state from `step`, and asks for the next opportunity.

This diagram omits the detailed action families, opportunity families, and role-specific helper flows.  It exists to show the minimal formal loop without the rest of the machinery.
