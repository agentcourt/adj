import Proofs.Reachability

namespace ArbdProofs

def actorFacingAction (action : CourtAction) : Prop :=
  action.action_type ≠ "remove_council_member" ∧
    action.action_type ≠ "fail_opportunity"

theorem requireOpportunityAuthority_ok_implies_eq
    (actual expected : OpportunityAuthority)
    (value : Unit)
    (hRequired : requireOpportunityAuthority actual expected = .ok value) :
    actual = expected := by
  by_cases hEqual : actual = expected
  · exact hEqual
  · simp [requireOpportunityAuthority, hEqual] at hRequired

theorem currentOpportunity_ok_implies_next
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hCurrent : currentOpportunity s = .ok opportunity) :
    (nextOpportunity s).opportunity = some opportunity := by
  unfold currentOpportunity at hCurrent
  cases hNext : (nextOpportunity s).opportunity with
  | none =>
      simp [hNext] at hCurrent
  | some current =>
      rw [hNext] at hCurrent
      cases hCurrent
      rfl

theorem authorizeAction_ok_matches_currentOpportunity
    (s : ArbitrationState)
    (action : CourtAction)
    (opportunity : OpportunitySpec)
    (hAuthorize : authorizeAction s action = .ok opportunity) :
    (nextOpportunity s).opportunity = some opportunity ∧
      action.authority = authorityForOpportunity s opportunity ∧
        authorizeOpportunityAction opportunity action = .ok () := by
  unfold authorizeAction at hAuthorize
  cases hCurrent : currentOpportunity s with
  | error err =>
      simp [hCurrent, Bind.bind, Except.bind] at hAuthorize
  | ok current =>
      simp only [hCurrent, Bind.bind, Except.bind] at hAuthorize
      cases hRequirement :
          requireOpportunityAuthority action.authority
            (authorityForOpportunity s current) with
      | error err =>
          simp [hRequirement] at hAuthorize
      | ok authorityValue =>
          simp only [hRequirement] at hAuthorize
          cases hOperation : authorizeOpportunityAction current action with
          | error err =>
              simp [hOperation] at hAuthorize
          | ok operationValue =>
              simp only [hOperation] at hAuthorize
              cases operationValue
              have hNext :=
                currentOpportunity_ok_implies_next s current hCurrent
              have hAuthority :=
                requireOpportunityAuthority_ok_implies_eq
                  action.authority (authorityForOpportunity s current)
                  authorityValue hRequirement
              cases hAuthorize
              exact ⟨hNext, hAuthority, hOperation⟩

theorem step_ok_matches_currentOpportunity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    ∃ opportunity,
      (nextOpportunity s).opportunity = some opportunity ∧
        action.authority = authorityForOpportunity s opportunity ∧
          authorizeOpportunityAction opportunity action = .ok () := by
  by_cases hClosed : s.case.status = "closed"
  · simp [step, hClosed] at hStep
  · by_cases hFailed : s.case.status = "failed"
    · simp [step, hFailed] at hStep
    · cases hAuthorize : authorizeAction s action with
      | error err =>
          simp [step, hClosed, hFailed, hAuthorize,
            Bind.bind, Except.bind] at hStep
      | ok opportunity =>
          exact ⟨opportunity,
            authorizeAction_ok_matches_currentOpportunity
              s action opportunity hAuthorize⟩

theorem closed_step_rejected
    (s : ArbitrationState)
    (action : CourtAction)
    (hClosed : s.case.status = "closed") :
    step { state := s, action := action } = .error "case is closed" := by
  simp [step, hClosed]

theorem failed_step_rejected
    (s : ArbitrationState)
    (action : CourtAction)
    (hFailed : s.case.status = "failed") :
    step { state := s, action := action } = .error "case has failed" := by
  simp [step, hFailed]

end ArbdProofs
