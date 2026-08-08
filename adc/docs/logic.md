# Procedure execution

This page explains the execution approach at a high level.  The system treats civil procedure as a constrained state-transition process: each action is checked against rule constraints, valid actions produce explicit state changes, and invalid actions produce explicit rejection reasons.  The relevant diagrams are described in [Lean request loop](../analysis/lean-simple-flow.md) and [Lean control flow](../analysis/lean-complete-flow.md).  The Mermaid sources live in [`analysis/`](../analysis/index.md).

These diagrams show the control logic of the system from case state through action selection, rule validation, state transition, and recorded output.  The core idea is separation of authority: adjudicative logic decides whether an action is valid and what legal consequence follows, while runtime logic executes requests and records results.

At a high level, each step begins from a current case state and a proposed procedural action.  The adjudicative layer evaluates that action against rule constraints, phase constraints, and any active judicial limits.  If the action is valid, the system produces a new state that reflects the legal effect of that step.  If the action is invalid, the system returns structured rejection reasons and preserves state.

That loop repeats until the proceeding reaches a terminal posture such as judgment, dismissal, or another rule-defined endpoint.  Because each transition is explicit, the process forms a verifiable chain rather than a sequence of informal updates.  The same inputs should produce the same legal transition, which supports predictable behavior and meaningful audit.

Record access, orchestration, and user interfaces can change without altering adjudicative semantics.  At the same time, rule revisions can be made without rewriting operational surfaces.  This boundary keeps the legal core stable while allowing operational components to evolve.

In summary, the system works as a constrained transition machine for civil procedure.  It takes a state and an action, applies rule-governed logic, emits a new state or a rejection, and records the result.  The design goal is not to automate discretion away.  The design goal is to ensure that discretion operates inside explicit procedural bounds.
