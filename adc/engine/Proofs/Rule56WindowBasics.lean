import ADC.Core

namespace ADCProofs.Rule56WindowBasics

/--
Closing a Rule 56 window marks that party as closed.

The proof plan is structural.  Unfold `closeRule56WindowFor` and
`rule56WindowClosedFor`.  Under the only meaningful hypothesis, namely
that the normalized party token is non-empty, the function either leaves a
previously closed window unchanged or appends the normalized token to the
closed-window list.  In either branch, the queried party is closed.
-/
theorem closeRule56WindowFor_marks_party_closed
    (c : CaseState) (party : String)
    (hparty : normalizePartyToken party ≠ "") :
    rule56WindowClosedFor (closeRule56WindowFor c party) party = true := by
  unfold rule56WindowClosedFor closeRule56WindowFor
  by_cases hmem : normalizePartyToken party ∈ c.rule56_window_closed_for
  · simp [hparty, hmem]
  · simp [hparty, hmem]

/--
Reopening Rule 56 windows clears the closure state for every party.

The proof plan is direct.  `reopenRule56Windows` resets the closed-window
list to `[]`, and `rule56WindowClosedFor` checks membership in that list.
No case-specific hypotheses are needed.
-/
theorem reopenRule56Windows_clears_party
    (c : CaseState) (party : String) :
    rule56WindowClosedFor (reopenRule56Windows c) party = false := by
  unfold rule56WindowClosedFor reopenRule56Windows
  simp

/--
Passing a non-Rule-56 opportunity does not change the Rule 56 closure set.

The proof plan is again structural.  Unfold `recordOpportunityPassFor`.
Every pass effect other than `rule56` either records a trace without changing
the closure set or records an ordinary opportunity pass.
-/
private theorem applyOpportunityPassToCase_non_rule56_preserves_window
    (c : CaseState) (opportunity : OpportunitySpec)
    (h : opportunity.pass_effect ≠ "rule56") :
    match applyOpportunityPassToCase? c opportunity with
    | some next => next.rule56_window_closed_for = c.rule56_window_closed_for
    | none => True := by
  unfold applyOpportunityPassToCase?
  by_cases hrule37 : opportunity.pass_effect = "rule37"
  · simp [hrule37, appendTrace]
  · by_cases hvoir : opportunity.pass_effect = "voir_dire_question"
    · simp [hvoir]
      cases requiredPayloadString? opportunity.constraints "asked_by" <;>
        cases requiredPayloadString? opportunity.constraints "juror_id" <;>
        simp [appendTrace]
    · by_cases hcause : opportunity.pass_effect = "for_cause_challenge"
      · simp [hcause, appendTrace]
      · by_cases hperemptory : opportunity.pass_effect = "peremptory_challenge"
        · simp [hperemptory, appendTrace]
        · simp [hrule37, h, hvoir, hcause, hperemptory]

theorem recordOpportunityPassFor_non_rule56_preserves_window
    (s : CourtState) (opportunity : OpportunitySpec)
    (h : opportunity.pass_effect ≠ "rule56") :
    (recordOpportunityPassFor s opportunity).case.rule56_window_closed_for =
      s.case.rule56_window_closed_for := by
  unfold recordOpportunityPassFor
  cases happly : applyOpportunityPassToCase? s.case opportunity with
  | none =>
      simp [recordOpportunityPass, bumpStateVersion]
  | some next =>
      have hpreserve :=
        applyOpportunityPassToCase_non_rule56_preserves_window s.case opportunity h
      rw [happly] at hpreserve
      simp [updateCase, clearPassedOpportunities, bumpStateVersion, hpreserve]

/--
Passing a Rule 56 opportunity closes the Rule 56 window for that role.

The proof plan combines the previous ideas.  Unfold
`recordOpportunityPassFor`, rewrite with the Rule 56 pass-effect hypothesis, and
reduce the goal to the helper lemma embodied in
`closeRule56WindowFor_marks_party_closed`.
-/
theorem recordOpportunityPassFor_rule56_closes_window
    (s : CourtState) (opportunity : OpportunitySpec)
    (heffect : opportunity.pass_effect = "rule56")
    (hrole : normalizePartyToken opportunity.role ≠ "") :
    rule56WindowClosedFor (recordOpportunityPassFor s opportunity).case opportunity.role = true := by
  have happly :
      applyOpportunityPassToCase? s.case opportunity =
        some (closeRule56WindowFor s.case opportunity.role) := by
    simp [applyOpportunityPassToCase?, heffect]
  unfold recordOpportunityPassFor
  rw [happly]
  change rule56WindowClosedFor (closeRule56WindowFor s.case opportunity.role) opportunity.role = true
  exact closeRule56WindowFor_marks_party_closed s.case opportunity.role hrole

end ADCProofs.Rule56WindowBasics
