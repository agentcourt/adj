import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "pretrial",
    phase := "discovery"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def fileRule56Action : CourtAction :=
  { action_type := "file_rule56_motion"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("movant", Lean.Json.str "defendant")
      , ("scope", Lean.Json.str "liability")
      , ("statement_of_undisputed_facts", Lean.Json.str "No genuine dispute on record evidence.")
      ]
  }

def decideRule56Action (disposition : String) : CourtAction :=
  { action_type := "decide_rule56_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("disposition", Lean.Json.str disposition)
      , ("reasoning", Lean.Json.str "The Rule 56 disposition follows from the summary-judgment record.")
      ]
  }

def amendComplaintAction : CourtAction :=
  { action_type := "file_amended_complaint"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("summary", Lean.Json.str "Plaintiff files an amended complaint with additional allegations.")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

def closedRule56Case : CaseState :=
  { baseCase with
    decision_traces := [
      { action := "file_complaint", outcome := "filed", citations := ["FRCP 3"] },
      { action := "file_answer", outcome := "filed", citations := ["FRCP 8(b)"] }
    ],
    docket := [
      { title := "Interrogatory Responses", description := "defendant: served" },
      { title := "Responses to Requests for Production", description := "defendant: served" },
      { title := "Responses to Requests for Admission", description := "defendant: served" }
    ],
    rule56_window_closed_for := ["defendant"]
  }

def rule56Roles : List RolePolicy :=
  [{ role := "defendant", allowed_tools := ["file_rule56_motion"] }]

def reopenedRule56Opportunity : Option OpportunitySpec :=
  match step (stateOf closedRule56Case) amendComplaintAction with
  | .ok s' =>
      currentOpenOpportunity?
        { state := s'
        , roles := rule56Roles
        , max_steps_per_turn := 3
        }
  | .error _ => none

def amendedComplaintRule56WindowClosedFor : List String :=
  match step (stateOf closedRule56Case) amendComplaintAction with
  | .ok s' => s'.case.rule56_window_closed_for
  | .error _ => ["error"]

def reopenedRule56OpportunityMatches : Bool :=
  match reopenedRule56Opportunity with
  | some opportunity =>
      opportunity.opportunity_id = "o1" &&
      opportunity.role = "defendant" &&
      opportunity.phase = "pretrial" &&
      opportunity.kind = "optional" &&
      opportunity.may_pass = true &&
      opportunity.step_budget = 3 &&
      opportunity.allowed_tools = ["file_rule56_motion"] &&
      opportunity.actor_message = "Current pretrial opportunity for defendant: consider this objective and either act now or pass." &&
      opportunity.objective = "For case 0, optionally file Rule 56 motion if no genuine dispute of material fact."
  | none => false

theorem step_file_rule56_requires_pretrial :
    let c := { baseCase with status := "trial" }
    stepErrorMessage (step (stateOf c) fileRule56Action) =
      "rule 56 motion requires pretrial status" := by
  native_decide

theorem step_file_rule56_rejects_when_already_decided :
    let c := { baseCase with docket := [{ title := "Rule 56 Order", description := "disposition=denied" }] }
    stepErrorMessage (step (stateOf c) fileRule56Action) =
      "rule 56 motion already decided" := by
  native_decide

theorem step_decide_rule56_requires_prior_motion :
    stepErrorMessage (step (stateOf) (decideRule56Action "denied")) =
      "cannot decide rule 56 motion before filing" := by
  native_decide

theorem step_decide_rule56_rejects_invalid_disposition :
    let c := { baseCase with docket := [{ title := "Rule 56 Motion", description := "defendant: filed" }] }
    stepErrorMessage (step (stateOf c) (decideRule56Action "vacated")) =
      "invalid rule 56 disposition: vacated" := by
  native_decide

theorem step_decide_rule56_records_order :
    let c := { baseCase with docket := [{ title := "Rule 56 Motion", description := "defendant: filed" }] }
    (match step (stateOf c) (decideRule56Action "denied") with
      | .ok s' => hasDocketTitle s'.case "Rule 56 Order"
      | .error _ => false) = true := by
  native_decide

/--
An amended complaint clears any previously closed Rule 56 window.

The proof plan is direct.  `file_amended_complaint` calls
`reopenRule56Windows`, so a closed-window pretrial case should step to a
state whose `rule56_window_closed_for` field is empty.  The supporting
definition isolates that field from the successful step result, which
keeps the theorem narrow and avoids re-proving the whole step shape.
-/
theorem amendedComplaint_clears_closed_rule56_windows :
    amendedComplaintRule56WindowClosedFor = [] := by
  native_decide

/-
This theorem started as a weaker state-field check.  That was still
useful, but the public consequence matters more than the internal list
update.  The supporting definition therefore checks the exact reopened
opportunity rather than stopping at the cleared field.
-/

/--
An amended complaint reopens the defendant's Rule 56 opportunity when the
ordinary pretrial prerequisites remain satisfied.

The proof plan is again direct, but it proves more than the preceding
state lemma.  Start from a pretrial case whose Rule 56 window is closed
for the defendant and whose discovery record otherwise makes Rule 56
available.  Step that case with `file_amended_complaint`.  Then ask
`currentOpenOpportunity?` for the defendant's Rule 56 opportunity.  The
result should match the finalized public opportunity shape, including its
phase, objective, and actor message.
-/
theorem amendedComplaint_reopens_rule56_window :
    reopenedRule56OpportunityMatches = true := by
  native_decide

/-
The first version of this theorem failed because it compared against the
pre-finalization shape from `mkTurn`.  `currentOpenOpportunity?` returns
finalized opportunities, so the theorem had to be stated against the
actual public API surface: `o1`, phase `pretrial`, the generic actor
message, and the Rule 56 objective string.
-/
