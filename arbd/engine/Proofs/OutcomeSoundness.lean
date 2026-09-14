import Proofs.Progress

namespace ArbdProofs

structure ClosedCaseSound
    (req : InitializeCaseRequest)
    (s : ArbitrationState) : Prop where
  merits_complete :
    bilateralComplete "openings" s.case.openings ∧
      bilateralComplete "arguments" s.case.arguments ∧
      plaintiffOptionalSequence "rebuttals" s.case.rebuttals ∧
      defendantOptionalSequence "surrebuttals" s.case.surrebuttals ∧
      bilateralComplete "closings" s.case.closings
  council_ids_unique : councilIdsUnique s.case
  answers_valid : answerIntegrity s.case
  record_integrity : RecordIntegrity s
  frame : initializedCaseFrame req s

theorem initialized_run_closed_case_sound
    (req : InitializeCaseRequest)
    (start target : ArbitrationState)
    (hInit : initializeCase req = .ok start)
    (hRun : StepReachableFrom start target)
    (hClosed : target.case.phase = "closed") :
    ClosedCaseSound req target := by
  have hInvariant := initializedRun_reachable_invariant req start target hInit hRun
  exact
    { merits_complete := phaseShape_closed_merits_complete target.case
        hInvariant.procedure.phase_shape hClosed
      council_ids_unique := hInvariant.procedure.council_ids_unique
      answers_valid := hInvariant.procedure.answers
      record_integrity := hInvariant.procedure.record
      frame := hInvariant.frame }

theorem continueDeliberation_closes_only_with_complete_answers
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hPhase : c.phase = "deliberation")
    (hContinue : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed") :
    (currentRoundAnswers c).length = seatedCouncilMemberCount c := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · exact hComplete
  · simp [hComplete] at hContinue
    cases hContinue
    simp [stateWithCase, hPhase] at hClosed

def OpportunityFailureEffect (s t : ArbitrationState) : Prop :=
  (∃ failure : OpportunityFailure,
    t = stateWithCase s
      { s.case with status := "failed", failure := some failure }) ∨
  (∃ memberId reason opportunityId message,
    failCouncilMemberOpportunity
      s memberId reason opportunityId message = .ok t)

theorem failOpportunity_success_effect
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hFail : failOpportunity s payload = .ok t) :
    OpportunityFailureEffect s t := by
  unfold failOpportunity at hFail
  cases hOpportunityId : getString payload "opportunity_id" with
  | error error =>
      simp only [hOpportunityId] at hFail
      cases hFail
  | ok rawOpportunityId =>
      simp only [hOpportunityId] at hFail
      cases hRole : getString payload "role" with
      | error error =>
          simp only [hRole] at hFail
          cases hFail
      | ok rawRole =>
          simp only [hRole] at hFail
          cases hPhase : getString payload "phase" with
          | error error =>
              simp only [hPhase] at hFail
              cases hFail
          | ok rawPhase =>
              simp only [hPhase] at hFail
              cases hReason : getString payload "reason" with
              | error error =>
                  simp only [hReason] at hFail
                  cases hFail
              | ok rawReason =>
                  simp only [hReason] at hFail
                  cases hMessage : getOptionalString payload "message" with
                  | error error =>
                      simp only [hMessage] at hFail
                      cases hFail
                  | ok message =>
                      simp only [hMessage] at hFail
                      cases hMember : getOptionalString payload "member_id" with
                      | error error =>
                          simp only [hMember] at hFail
                          cases hFail
                      | ok memberId =>
                          simp only [hMember] at hFail
                          cases hModel : getOptionalString payload "model" with
                          | error error =>
                              simp only [hModel] at hFail
                              cases hFail
                          | ok model =>
                              simp only [hModel] at hFail
                              let opportunityId := trimString rawOpportunityId
                              let role := trimString rawRole
                              let phase := trimString rawPhase
                              let reason := trimString rawReason
                              change
                                (if opportunityId = "" then
                                    throw "opportunity failure requires opportunity_id"
                                  else if role = "" then
                                    throw "opportunity failure requires role"
                                  else if phase = "" then
                                    throw "opportunity failure requires phase"
                                  else if reason = "" then
                                    throw "opportunity failure requires reason"
                                  else
                                    do
                                      let opportunity ←
                                        match (nextOpportunity s).opportunity with
                                        | none => throw "no active opportunity can fail"
                                        | some opportunity => pure opportunity
                                      if opportunity.opportunity_id != opportunityId then
                                        throw s!"opportunity_id {opportunityId} does not match current opportunity {opportunity.opportunity_id}"
                                      else if opportunity.role != role then
                                        throw s!"opportunity role {role} does not match current role {opportunity.role}"
                                      else if opportunity.phase != phase then
                                        throw s!"opportunity phase {phase} does not match current phase {opportunity.phase}"
                                      else if role = "council" then
                                        failCouncilMemberOpportunity
                                          s memberId reason opportunityId message
                                      else if role = "plaintiff" || role = "defendant" then
                                        let failure : OpportunityFailure := {
                                          failure_type := "opportunity_failed"
                                          role := role
                                          phase := phase
                                          opportunity_id := opportunityId
                                          reason := reason
                                          message := message
                                          member_id := memberId
                                          model := model
                                        }
                                        pure <| stateWithCase s
                                          { s.case with
                                            status := "failed"
                                            failure := some failure }
                                      else
                                        throw s!"unsupported opportunity failure role: {role}") =
                                  .ok t at hFail
                              by_cases hOpportunityEmpty : opportunityId = ""
                              · rw [if_pos hOpportunityEmpty] at hFail
                                cases hFail
                              · by_cases hRoleEmpty : role = ""
                                · rw [if_neg hOpportunityEmpty, if_pos hRoleEmpty] at hFail
                                  cases hFail
                                · by_cases hPhaseEmpty : phase = ""
                                  · rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                      if_pos hPhaseEmpty] at hFail
                                    cases hFail
                                  · by_cases hReasonEmpty : reason = ""
                                    · rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                        if_neg hPhaseEmpty, if_pos hReasonEmpty] at hFail
                                      cases hFail
                                    · cases hNext : (nextOpportunity s).opportunity with
                                      | none =>
                                          rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                            if_neg hPhaseEmpty, if_neg hReasonEmpty] at hFail
                                          simp only [hNext] at hFail
                                          cases hFail
                                      | some opportunity =>
                                          rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                            if_neg hPhaseEmpty, if_neg hReasonEmpty] at hFail
                                          simp only [hNext, Bind.bind, Except.bind,
                                            Pure.pure, Except.pure] at hFail
                                          by_cases hOpportunityMismatch :
                                              opportunity.opportunity_id != opportunityId
                                          · rw [if_pos hOpportunityMismatch] at hFail
                                            cases hFail
                                          · rw [if_neg hOpportunityMismatch] at hFail
                                            by_cases hRoleMismatch : opportunity.role != role
                                            · rw [if_pos hRoleMismatch] at hFail
                                              cases hFail
                                            · rw [if_neg hRoleMismatch] at hFail
                                              by_cases hPhaseMismatch : opportunity.phase != phase
                                              · rw [if_pos hPhaseMismatch] at hFail
                                                cases hFail
                                              · rw [if_neg hPhaseMismatch] at hFail
                                                by_cases hCouncil : role = "council"
                                                · rw [if_pos hCouncil] at hFail
                                                  exact Or.inr
                                                    ⟨memberId, reason, opportunityId, message, hFail⟩
                                                · rw [if_neg hCouncil] at hFail
                                                  by_cases hParty :
                                                      role = "plaintiff" || role = "defendant"
                                                  · rw [if_pos hParty] at hFail
                                                    let failure : OpportunityFailure := {
                                                      failure_type := "opportunity_failed"
                                                      role := role
                                                      phase := phase
                                                      opportunity_id := opportunityId
                                                      reason := reason
                                                      message := message
                                                      member_id := memberId
                                                      model := model
                                                    }
                                                    exact Or.inl ⟨failure, by
                                                      simpa [failure] using hFail.symm⟩
                                                  · rw [if_neg hParty] at hFail
                                                    cases hFail

end ArbdProofs
