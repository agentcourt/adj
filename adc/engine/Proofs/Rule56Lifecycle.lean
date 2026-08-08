import Proofs.ApplyDecision

def rule56PassStateForLifecycle : CourtState :=
  match applyDecision validRule56PassReq with
  | .ok ok =>
      match ok.state with
      | some s => s
      | none => default
  | .error _ => default

def serveInitialDisclosuresAfterRule56PassAction : CourtAction :=
  { action_type := "serve_initial_disclosures"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("party", Lean.Json.str "plaintiff")
      , ("summary", Lean.Json.str "Plaintiff discloses damages computation and supporting documents.")
      ]
  }

def amendComplaintAfterRule56PassAction : CourtAction :=
  { action_type := "file_amended_complaint"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("summary", Lean.Json.str "Plaintiff files an amended complaint with additional factual allegations.")
      ]
  }

def rule56ReqAtState (s : CourtState) : OpportunityRequest :=
  { state := s
  , roles := optionalRule56Req.roles
  , max_steps_per_turn := optionalRule56Req.max_steps_per_turn
  }

def rule56OpportunityAfterInitialDisclosures : Option OpportunitySpec :=
  match step rule56PassStateForLifecycle serveInitialDisclosuresAfterRule56PassAction with
  | .ok s' => currentOpenOpportunity? (rule56ReqAtState s')
  | .error _ => some default

def rule56WindowAfterInitialDisclosures : List String :=
  match step rule56PassStateForLifecycle serveInitialDisclosuresAfterRule56PassAction with
  | .ok s' => s'.case.rule56_window_closed_for
  | .error _ => ["error"]

def rule56OpportunityAfterAmendedComplaint : Option OpportunitySpec :=
  match step rule56PassStateForLifecycle serveInitialDisclosuresAfterRule56PassAction with
  | .error _ => some default
  | .ok s1 =>
      match step s1 amendComplaintAfterRule56PassAction with
      | .ok s2 => currentOpenOpportunity? (rule56ReqAtState s2)
      | .error _ => some default

def reopenedRule56OpportunityAfterAmendedComplaintMatches : Bool :=
  match rule56OpportunityAfterAmendedComplaint with
  | some opportunity =>
      opportunity.role = "defendant" &&
      opportunity.allowed_tools = ["file_rule56_motion"] &&
      opportunity.kind = "optional" &&
      opportunity.may_pass = true &&
      opportunity.phase = "pretrial" &&
      opportunity.objective = "For case 0, optionally file Rule 56 motion if no genuine dispute of material fact."
  | none => false

/--
Serving initial disclosures after a valid Rule 56 pass does not reopen the
closed Rule 56 window.

The proof plan is concrete, but the fact is meaningful.  Start from the public
post-pass state already produced by `applyDecision validRule56PassReq`.  Step
that state with an unrelated pretrial action, `serve_initial_disclosures`.
Then inspect the resulting `rule56_window_closed_for` field.  Because the step
branch for initial disclosures appends docket and trace entries without
touching the Rule 56 window, the closed-window list should remain
`["defendant"]`.
-/
theorem step_initial_disclosures_after_rule56_pass_preserves_closed_window :
    rule56WindowAfterInitialDisclosures = ["defendant"] := by
  native_decide

/--
Serving initial disclosures after a valid Rule 56 pass leaves no current Rule
56 opportunity for the defendant.

The proof plan combines the same concrete post-pass state with the defendant's
Rule 56-only opportunity request.  After the unrelated initial-disclosures
step, the closed Rule 56 window still disables eligibility for the defendant.
Because the request exposes no other tool, `currentOpenOpportunity?` must
remain `none`.
-/
theorem step_initial_disclosures_after_rule56_pass_keeps_rule56_unavailable :
    rule56OpportunityAfterInitialDisclosures = none := by
  native_decide

/--
An amended complaint reopens the defendant's Rule 56 opportunity even after an
intervening unrelated pretrial step.

The proof plan continues the same concrete flow.  First step the post-pass
state with initial disclosures, which should leave the closed Rule 56 window in
place.  Then step that result with `file_amended_complaint`.  The amended
complaint branch calls `reopenRule56Windows`, so the defendant's optional Rule
56 opportunity should become current again under the same Rule 56-only request.
-/
theorem step_amended_complaint_after_initial_disclosures_reopens_rule56 :
    reopenedRule56OpportunityAfterAmendedComplaintMatches = true := by
  native_decide

/--
The Rule 56 window records procedural memory: a valid Rule 56 pass closes the
window, an unrelated pretrial step does not reopen it, and an amended
complaint does reopen it.

The proof plan composes earlier results instead of recomputing the whole flow.
The first conjunct reuses `applyDecision_rule56_pass_closes_window`, which
already proves the public post-pass effect.  The second and third conjuncts use
the two concrete step theorems above.  The result is a short lifecycle theorem
at the public boundary: Rule 56 does not reappear because of ordinary pretrial
activity, but it does reappear after the specific procedural event that the
Rules treat as reopening the window.
-/
theorem rule56_pass_persists_until_amended_complaint :
    applyRule56WindowClosedFor (applyDecision validRule56PassReq) = ["defendant"] ∧
      applyRule56PassLeavesNoOpenOpportunity (applyDecision validRule56PassReq) = true ∧
      rule56OpportunityAfterInitialDisclosures = none ∧
      reopenedRule56OpportunityAfterAmendedComplaintMatches = true := by
  refine ⟨?_, ?_, ?_, ?_⟩
  · exact applyDecision_rule56_pass_closes_window.2.1
  · exact applyDecision_rule56_pass_closes_window.2.2
  · exact step_initial_disclosures_after_rule56_pass_keeps_rule56_unavailable
  · exact step_amended_complaint_after_initial_disclosures_reopens_rule56

/-
This is the right Rule 56 theorem for the current engine.  It is more
interesting than a one-step guard check because it follows the opportunity
through a small procedural history.  The theorem still stays objective.  It
does not say whether summary judgment would be correct.  It says that Lean
remembers the pass, ignores unrelated pretrial churn, and reopens the window
only when the amended-complaint step says it should.

The proof is intentionally mixed.  The lifecycle theorem itself composes prior
theorems.  The two intervening-step theorems use `native_decide` because they
are fully closed concrete flows, and forcing them into a longer manual proof
would not reveal more structure.  The next useful step is a more generic
pretrial-memory theorem that covers other unrelated `updateCase` steps, not
only `serve_initial_disclosures`.
-/
