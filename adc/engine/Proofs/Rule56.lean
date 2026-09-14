import ADC.Core

namespace ADCProofs.Rule56

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
      [ ("motion_index", Lean.Json.num 0)
      , ("disposition", Lean.Json.str disposition)
      , ("reasoning", Lean.Json.str "The Rule 56 disposition follows from the summary-judgment record.")
      ]
  }

def opposeRule56Action (motionIndex : Nat) (party : String) : CourtAction :=
  { action_type := "oppose_rule56_motion"
  , actor_role := party
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num motionIndex)
      , ("party", Lean.Json.str party)
      , ("summary", Lean.Json.str "The record contains a genuine dispute.")
      ]
  }

def replyRule56Action (motionIndex : Nat) (party : String) : CourtAction :=
  { action_type := "reply_rule56_motion"
  , actor_role := party
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num motionIndex)
      , ("party", Lean.Json.str party)
      , ("summary", Lean.Json.str "The opposition does not establish a genuine dispute.")
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
      docketEntryWithFields "Interrogatories Served" "defendant: served_on=plaintiff set_index=0 questions=[]"
        [("served_by", "defendant"), ("served_on", "plaintiff"), ("set_index", "0")],
      docketEntryWithFields "Interrogatory Responses" "plaintiff: responding_party=plaintiff set_index=0 answers=[]"
        [("responding_party", "plaintiff"), ("set_index", "0")],
      docketEntryWithFields "Requests for Production Served" "defendant: served_on=plaintiff set_index=0 requests=[]"
        [("served_by", "defendant"), ("served_on", "plaintiff"), ("set_index", "0")],
      docketEntryWithFields "Responses to Requests for Production" "plaintiff: responding_party=plaintiff set_index=0 responses=[]"
        [("responding_party", "plaintiff"), ("set_index", "0")],
      docketEntryWithFields "Requests for Admission Served" "defendant: served_on=plaintiff set_index=0 requests=[]"
        [("served_by", "defendant"), ("served_on", "plaintiff"), ("set_index", "0")],
      docketEntryWithFields "Responses to Requests for Admission" "plaintiff: responding_party=plaintiff set_index=0 responses=[]"
        [("responding_party", "plaintiff"), ("set_index", "0")]
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
      opportunity.objective = "For case 0, file a Rule 56 motion only if the record establishes that no genuine dispute of material fact requires trial. Otherwise pass."
  | none => false

theorem step_file_rule56_requires_pretrial :
    let c := { baseCase with status := "trial" }
    stepErrorMessage (step (stateOf c) fileRule56Action) =
      "rule 56 motion requires pretrial status" := by
  native_decide

theorem step_file_rule56_rejects_second_motion_by_party :
    let c := { baseCase with docket := [
      docketEntryWithFields "Rule 56 Motion"
        "defendant: motion_index=0 scope=liability statement_of_undisputed_facts=record"
        [("movant", "defendant"), ("motion_index", "0")],
      docketEntryWithFields "Rule 56 Order" "motion_index=0 disposition=denied"
        [("motion_index", "0"), ("disposition", "denied")]
    ] }
    stepErrorMessage (step (stateOf c) fileRule56Action) =
      "rule 56 motion already filed by defendant" := by
  native_decide

theorem step_decide_rule56_requires_prior_motion :
    stepErrorMessage (step (stateOf) (decideRule56Action "denied")) =
      "rule 56 motion is not the pending motion" := by
  native_decide

theorem step_decide_rule56_rejects_invalid_disposition :
    let c := { baseCase with docket := [docketEntryWithFields "Rule 56 Motion"
      "defendant: motion_index=0 scope=liability statement_of_undisputed_facts=record"
      [("movant", "defendant"), ("motion_index", "0")]] }
    stepErrorMessage (step (stateOf c) (decideRule56Action "vacated")) =
      "invalid rule 56 disposition: vacated" := by
  native_decide

theorem step_decide_rule56_records_order :
    let c := { baseCase with docket := [docketEntryWithFields "Rule 56 Motion"
      "defendant: motion_index=0 scope=liability statement_of_undisputed_facts=record"
      [("movant", "defendant"), ("motion_index", "0")]] }
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

def secondRule56MotionCase : CaseState :=
  { baseCase with docket := [
      docketEntryWithFields "Rule 56 Motion" "defendant: motion_index=0"
        [("movant", "defendant"), ("motion_index", "0")],
      docketEntryWithFields "Rule 56 Order" "motion_index=0 disposition=denied"
        [("motion_index", "0"), ("disposition", "denied")],
      docketEntryWithFields "Rule 56 Motion" "plaintiff: motion_index=1"
        [("movant", "plaintiff"), ("motion_index", "1")]
    ] }

theorem second_rule56_motion_accepts_one_opposition_and_rejects_duplicate :
    (match step (stateOf secondRule56MotionCase) (opposeRule56Action 1 "defendant") with
    | .error _ => "first opposition rejected"
    | .ok opposed => stepErrorMessage (step opposed (opposeRule56Action 1 "defendant"))) =
      "rule 56 opposition already filed" := by
  native_decide

theorem rule56_reply_requires_opposition_for_same_motion :
    let c := { secondRule56MotionCase with docket := secondRule56MotionCase.docket ++ [
      docketEntryWithFields "Rule 56 Opposition" "plaintiff: motion_index=0"
        [("party", "plaintiff"), ("motion_index", "0")]
    ] }
    stepErrorMessage (step (stateOf c) (replyRule56Action 1 "plaintiff")) =
      "cannot reply before the rule 56 opposition is filed" := by
  native_decide

theorem dispositive_motion_count_uses_movant_field :
    let c := { baseCase with docket := [
      docketEntryWithFields "Rule 56 Motion" "plaintiff: misleading description"
        [("movant", "defendant"), ("motion_index", "0")]
    ] }
    countDispositiveMotionsByParty c "plaintiff" = 0 ∧
      countDispositiveMotionsByParty c "defendant" = 1 := by
  native_decide

end ADCProofs.Rule56
