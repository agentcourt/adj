import Main

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

/-
This theorem is the bridge between the helper lemmas in
`Rule56WindowBasics.lean` and the higher-level opportunity machinery.  It
states the exact semantic role of the closed-window predicate in the
eligibility test.
-/

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
    (hmotion : facts.hasRule56Motion = false)
    (horder : facts.hasRule56Order = false)
    (hpretrial : facts.hasPretrialOrder = false)
    (hint : facts.hasInterrogatoryResponses = true)
    (hrfp : facts.hasRfpResponses = true)
    (hrfa : facts.hasRfaResponses = true)
    (hrule37 : !facts.hasRule37Motion || facts.hasRule37Order)
    (hclosed : rule56WindowClosedFor c party = false) :
    rule56WindowEligible c facts party = true := by
  unfold rule56WindowEligible
  simp [hmotion, horder, hpretrial, hint, hrfp, hrfa, hrule37, hclosed]

/-
This theorem states the positive side of the same eligibility rule.  It
gives later proofs a clean target when they need to show that reopening
the window actually matters.
-/

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
    (hmotion : facts.hasRule56Motion = false)
    (horder : facts.hasRule56Order = false)
    (hpretrial : facts.hasPretrialOrder = false)
    (hint : facts.hasInterrogatoryResponses = true)
    (hrfp : facts.hasRfpResponses = true)
    (hrfa : facts.hasRfaResponses = true)
    (hrule37 : !facts.hasRule37Motion || facts.hasRule37Order) :
    rule56WindowEligible (reopenRule56Windows c) facts party = true := by
  have hclosed : rule56WindowClosedFor (reopenRule56Windows c) party = false := by
    unfold rule56WindowClosedFor reopenRule56Windows
    simp
  apply rule56WindowEligible_true_when_prerequisites_hold
  · exact hmotion
  · exact horder
  · exact hpretrial
  · exact hint
  · exact hrfp
  · exact hrfa
  · exact hrule37
  · exact hclosed

/-
This is the better theorem to keep at this stage.  It avoids overfitting to
the full pretrial opportunity pipeline, but it still captures the legal
consequence of reopening: with the same discovery record, Rule 56 becomes
available again because the window itself is no longer closed.
-/
