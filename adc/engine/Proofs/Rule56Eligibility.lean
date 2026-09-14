import ADC.Core

namespace ADCProofs.Rule56Eligibility

/--
A closed Rule 56 window makes Rule 56 ineligible for that party.

The proof plan is symbolic.  Unfold `rule56WindowEligible` and rewrite the
last conjunct with the hypothesis that the party's Rule 56 window is
already closed.  The whole conjunction then collapses to `false`.
-/
theorem rule56WindowEligible_false_when_window_closed
    (c : CaseState) (facts : TurnFacts) (party : String)
    (hclosed : rule56WindowClosedFor c party = true) :
    rule56WindowEligible c facts party = false := by
  unfold rule56WindowEligible
  rw [hclosed]
  simp

/--
If every other Rule 56 prerequisite holds and the window is open, Rule 56
is eligible.

The proof plan is symbolic.  Rewrite each conjunct of
`rule56WindowEligible` with the supplied hypotheses, including the fact
that the Rule 56 window is not closed for the party.  The conjunction then
reduces to `true`.
-/
theorem rule56WindowEligible_true_when_prerequisites_hold
    (c : CaseState) (facts : TurnFacts) (party : String)
    (hpretrial : facts.hasPretrialOrder = false)
    (hrule56 : pendingMotionIndex? c "Rule 56 Motion" "Rule 56 Order" = none)
    (hrule37 : pendingMotionIndex? c "Rule 37 Motion" "Rule 37 Order" = none)
    (hprior : countDocketTitleByField c "Rule 56 Motion" "movant" party = 0)
    (hdiscovery : discoveryCompleteFor c party = true)
    (hclosed : rule56WindowClosedFor c party = false) :
    rule56WindowEligible c facts party = true := by
  unfold rule56WindowEligible
  simp [hpretrial, hrule56, hrule37, hprior, hdiscovery, hclosed]

/--
Reopening a closed Rule 56 window restores Rule 56 eligibility when the
ordinary discovery prerequisites remain satisfied.

The proof plan combines one symbolic helper with the reopening function.
First rewrite `rule56WindowClosedFor (reopenRule56Windows c) party` to
`false`.  Then apply the positive eligibility theorem under the same
preconditions on `facts`.
-/
theorem reopenRule56Windows_restores_eligibility
    (c : CaseState) (facts : TurnFacts) (party : String)
    (hpretrial : facts.hasPretrialOrder = false)
    (hrule56 : pendingMotionIndex? c "Rule 56 Motion" "Rule 56 Order" = none)
    (hrule37 : pendingMotionIndex? c "Rule 37 Motion" "Rule 37 Order" = none)
    (hprior : countDocketTitleByField c "Rule 56 Motion" "movant" party = 0)
    (hdiscovery : discoveryCompleteFor c party = true) :
    rule56WindowEligible (reopenRule56Windows c) facts party = true := by
  have hclosed : rule56WindowClosedFor (reopenRule56Windows c) party = false := by
    unfold rule56WindowClosedFor reopenRule56Windows
    simp
  apply rule56WindowEligible_true_when_prerequisites_hold
  · exact hpretrial
  · change pendingMotionIndex? c "Rule 56 Motion" "Rule 56 Order" = none
    exact hrule56
  · change pendingMotionIndex? c "Rule 37 Motion" "Rule 37 Order" = none
    exact hrule37
  · change countDocketTitleByField c "Rule 56 Motion" "movant" party = 0
    exact hprior
  · change discoveryCompleteFor c party = true
    exact hdiscovery
  · exact hclosed

end ADCProofs.Rule56Eligibility
