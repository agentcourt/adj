import Proofs.StepPreservation

namespace ArbProofs

/-
This file states the larger procedural theorem that the earlier files were
preparing to prove.

`Reachable` is the engine's own notion of a valid history: one successful
initialization followed by zero or more successful public `step` calls.  The
question now is no longer local.  It is global.

Do those public transitions preserve the intended procedural structure across
every state the engine can produce?

For the first global theorem, the answer is yes.  Every reachable state keeps
the merits sequence in order.  The proof is an induction on `Reachable`.  The
base case is initialization.  The step case is a case split on the public
action type, delegating to the step-preservation theorems proved earlier.

Once that theorem is in place, the fairness summary follows immediately.
`proceduralParity` was designed as a smaller consequence of `phaseShape`, not
as an independent property with its own separate induction.
-/

/--
Every reachable state preserves the intended merits sequence.

This theorem is the first full engine invariant.  It no longer speaks about
one helper function or one sample trace.  It speaks about every state the Lean
engine can produce through successful public transitions.
-/
theorem reachable_phaseShape
    (s : ArbitrationState)
    (hs : Reachable s) :
    phaseShape s.case := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_phaseShape req s hInit
  | step s t action hs hStep ih =>
      have hStepCore := stepCore_ok_of_step_ok s t action hStep
      by_cases hOpening : action.action_type = "record_opening_statement"
      · exact step_record_opening_statement_preserves_phaseShape s t action hOpening ih hStepCore
      · by_cases hArgument : action.action_type = "submit_argument"
        · exact step_submit_argument_preserves_phaseShape s t action hArgument ih hStepCore
        · by_cases hRebuttal : action.action_type = "submit_rebuttal"
          · exact step_submit_rebuttal_preserves_phaseShape s t action hRebuttal ih hStepCore
          · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
            · exact step_submit_surrebuttal_preserves_phaseShape s t action hSurrebuttal ih hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact step_submit_evidence_preserves_phaseShape s t action hEvidence ih hStepCore
              · by_cases hClosing : action.action_type = "deliver_closing_statement"
                · exact step_deliver_closing_statement_preserves_phaseShape s t action hClosing ih hStepCore
                · by_cases hPass : action.action_type = "pass_phase_opportunity"
                  · exact step_pass_phase_opportunity_preserves_phaseShape s t action hPass ih hStepCore
                  · by_cases hVote : action.action_type = "submit_council_vote"
                    · exact step_submit_council_vote_preserves_phaseShape s t action hVote ih hStepCore
                    · by_cases hRemoval : action.action_type = "remove_council_member"
                      · exact step_remove_council_member_preserves_phaseShape s t action hRemoval ih hStepCore
                      · by_cases hFail : action.action_type = "fail_opportunity"
                        · exact step_fail_opportunity_preserves_phaseShape s t action hFail ih hStepCore
                        · simp [stepCore] at hStepCore

/--
Every reachable state satisfies the procedural parity summary.

This theorem is intentionally short.  The real work is in
`reachable_phaseShape`.  `proceduralParity` then follows from the structural
theorem already proved in `ProcedureShape.lean`.
-/
theorem reachable_proceduralParity
    (s : ArbitrationState)
    (hs : Reachable s) :
    proceduralParity s.case := by
  exact phaseShape_implies_proceduralParity s.case (reachable_phaseShape s hs)

end ArbProofs
