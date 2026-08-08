import Proofs.StepPreservation

namespace ArbProofs

/-
This file states the second global invariant.

The earlier arithmetic proof in `AggregateLimits.lean` showed what must happen
when one side appends a new batch of exhibits and reports that still fits
within the remaining budget.  The step-level proofs then have to connect that
fact to the public engine boundary.

The global theorem is the same pattern as `reachable_phaseShape`: induction on
`Reachable`, with one preservation theorem for each successful public action.

The important point is now global rather than local.  The earlier files proved
the counting argument and then proved each public transition preserves that
argument.  This file closes the loop.  Every reachable state, not just every
sample trace or helper result, respects the cumulative exhibit and report caps.
-/

/--
Every reachable state respects the aggregate exhibit and report caps.
-/
theorem reachable_materialLimitsRespected
    (s : ArbitrationState)
    (hs : Reachable s) :
    materialLimitsRespected s := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_materialLimits req s hInit
  | step s t action hs hStep ih =>
      have hStepCore := stepCore_ok_of_step_ok s t action hStep
      by_cases hOpening : action.action_type = "record_opening_statement"
      · exact step_record_opening_statement_preserves_material_limits s t action hOpening ih hStepCore
      · by_cases hArgument : action.action_type = "submit_argument"
        · exact step_submit_argument_preserves_material_limits s t action hArgument ih hStepCore
        · by_cases hRebuttal : action.action_type = "submit_rebuttal"
          · exact step_submit_rebuttal_preserves_material_limits s t action hRebuttal ih hStepCore
          · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
            · exact step_submit_surrebuttal_preserves_material_limits s t action hSurrebuttal ih hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact step_submit_evidence_preserves_material_limits s t action hEvidence ih hStepCore
              · by_cases hClosing : action.action_type = "deliver_closing_statement"
                · exact step_deliver_closing_statement_preserves_material_limits s t action hClosing ih hStepCore
                · by_cases hPass : action.action_type = "pass_phase_opportunity"
                  · exact step_pass_phase_opportunity_preserves_material_limits s t action hPass ih hStepCore
                  · by_cases hVote : action.action_type = "submit_council_vote"
                    · exact step_submit_council_vote_preserves_material_limits s t action hVote ih hStepCore
                    · by_cases hRemoval : action.action_type = "remove_council_member"
                      · exact step_remove_council_member_preserves_material_limits s t action hRemoval ih hStepCore
                      · by_cases hFail : action.action_type = "fail_opportunity"
                        · exact step_fail_opportunity_preserves_material_limits s t action hFail ih hStepCore
                        · simp [stepCore] at hStepCore

end ArbProofs
