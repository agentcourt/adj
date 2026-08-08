import Main

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

/-
This theorem is small, but it is not an evaluator check.  It captures the
intended postcondition of the closure helper and avoids repeating list
reasoning in later orchestration proofs.
-/

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

/-
This lemma gives the reopening side of the same helper story.  Together,
the two lemmas isolate the Rule 56 window mechanism from the surrounding
opportunity engine.
-/

/--
Passing a non-Rule-56 opportunity does not change the Rule 56 closure set.

The proof plan is again structural.  Unfold `recordOpportunityPassFor`.
When the opportunity does not expose only `file_rule56_motion`, the
function returns the bumped state `s1` unchanged on the case field.  The
resulting closed-window list is therefore exactly the original one.
-/
theorem recordOpportunityPassFor_non_rule56_preserves_window
    (s : CourtState) (opportunity : OpportunitySpec)
    (h : opportunity.allowed_tools ≠ ["file_rule56_motion"]) :
    (recordOpportunityPassFor s opportunity).case.rule56_window_closed_for =
      s.case.rule56_window_closed_for := by
  unfold recordOpportunityPassFor
  simp [h, bumpStateVersion]

/-
This is the first lemma that connects the helper functions to the
opportunity engine.  It states that ordinary optional opportunities do not
silently affect the Rule 56 window.
-/

/--
Passing a Rule 56 opportunity closes the Rule 56 window for that role.

The proof plan combines the previous ideas.  Unfold
`recordOpportunityPassFor`, rewrite with the Rule 56 tool hypothesis, and
reduce the goal to the helper lemma embodied in
`closeRule56WindowFor_marks_party_closed`.
-/
theorem recordOpportunityPassFor_rule56_closes_window
    (s : CourtState) (opportunity : OpportunitySpec)
    (htools : opportunity.allowed_tools = ["file_rule56_motion"])
    (hrole : normalizePartyToken opportunity.role ≠ "") :
    rule56WindowClosedFor (recordOpportunityPassFor s opportunity).case opportunity.role = true := by
  unfold recordOpportunityPassFor
  unfold rule56WindowClosedFor closeRule56WindowFor
  by_cases hmem : normalizePartyToken opportunity.role ∈ s.case.rule56_window_closed_for
  · simp [htools, hrole, hmem, bumpStateVersion]
  · simp [htools, hrole, hmem, bumpStateVersion]

/-
This theorem states the core procedural effect of a Rule 56 pass without
building a concrete case.  Later orchestration theorems can use it instead
of re-evaluating a particular `ApplyDecisionRequest`.
-/
