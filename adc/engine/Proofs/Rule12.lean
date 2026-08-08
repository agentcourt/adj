import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "filed",
    phase := "pleadings"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def filedRule12Case (ground : String) : CaseState :=
  { baseCase with
    docket := [{ title := "Rule 12 Motion", description := s!"defendant: ground={ground} summary=Complaint lacks plausible allegations." }]
  }

def fileRule12Action (ground : String := "failure_to_state_a_claim") : CourtAction :=
  { action_type := "file_rule12_motion"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("movant", Lean.Json.str "defendant")
      , ("ground", Lean.Json.str ground)
      , ("summary", Lean.Json.str "Complaint lacks plausible allegations.")
      ]
  }

def decideRule12Action
    (ground disposition : String)
    (withPrejudice leaveToAmend : Bool)
    (extra : List (String × Lean.Json) := []) : CourtAction :=
  { action_type := "decide_rule12_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj <|
      [ ("ground", Lean.Json.str ground)
      , ("disposition", Lean.Json.str disposition)
      , ("with_prejudice", Lean.Json.bool withPrejudice)
      , ("leave_to_amend", Lean.Json.bool leaveToAmend)
      , ("reasoning", Lean.Json.str "The Rule 12 disposition follows from the pleadings.")
      ] ++ extra
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_file_rule12_requires_filed_or_pretrial :
    let c := { baseCase with status := "trial" }
    stepErrorMessage (step (stateOf c) (fileRule12Action)) =
      "rule 12 motion requires filed or pretrial status" := by
  native_decide

theorem step_file_rule12_unavailable_after_answer :
    let c := { baseCase with decision_traces := [{ action := "file_answer", outcome := "filed", citations := [] }] }
    stepErrorMessage (step (stateOf c) (fileRule12Action)) =
      "rule 12 motion unavailable after answer is filed" := by
  native_decide

theorem step_decide_rule12_requires_motion :
    stepErrorMessage (step (stateOf) (decideRule12Action "failure_to_state_a_claim" "denied" false false)) =
      "cannot decide rule 12 motion before filing" := by
  native_decide

theorem step_decide_rule12_rejects_invalid_disposition :
    stepErrorMessage (step (stateOf (filedRule12Case "failure_to_state_a_claim"))
      (decideRule12Action "failure_to_state_a_claim" "partial" false false)) =
        "invalid rule 12 disposition: partial" := by
  native_decide

theorem step_decide_rule12_rejects_conflicting_prejudice_and_amend :
    stepErrorMessage (step (stateOf (filedRule12Case "failure_to_state_a_claim"))
      (decideRule12Action "failure_to_state_a_claim" "granted" true true
        [("missing_elements", Lean.Json.arr #[Lean.Json.str "reliance"]) ])) =
        "rule 12 order cannot be both with_prejudice and leave_to_amend" := by
  native_decide

theorem step_decide_rule12_requires_matching_ground :
    stepErrorMessage (step (stateOf (filedRule12Case "no_standing"))
      (decideRule12Action "moot" "denied" false false)) =
        "rule 12 ruling ground must match filed motion ground: filed=no_standing, ruling=moot" := by
  native_decide

theorem step_decide_rule12_requires_jurisdiction_basis_rejected :
    stepErrorMessage (step (stateOf (filedRule12Case "lack_subject_matter_jurisdiction"))
      (decideRule12Action "lack_subject_matter_jurisdiction" "granted" false false)) =
        "rule 12 jurisdiction dismissal requires jurisdiction_basis_rejected" := by
  native_decide

theorem step_decide_rule12_requires_standing_component :
    stepErrorMessage (step (stateOf (filedRule12Case "no_standing"))
      (decideRule12Action "no_standing" "granted" false false)) =
        "standing dismissal requires at least one missing standing component" := by
  native_decide

theorem step_decide_rule12_requires_missing_elements :
    stepErrorMessage (step (stateOf (filedRule12Case "failure_to_state_a_claim"))
      (decideRule12Action "failure_to_state_a_claim" "granted" false true)) =
        "failure_to_state_a_claim dismissal requires missing_elements" := by
  native_decide

theorem step_decide_rule12_granted_failure_to_state_a_claim_closes_case :
    (match step (stateOf (filedRule12Case "failure_to_state_a_claim"))
      (decideRule12Action "failure_to_state_a_claim" "granted" true false
        [("missing_elements", Lean.Json.arr #[Lean.Json.str "reliance"]) ]) with
      | .ok s' => s'.case.status
      | .error _ => "") = "closed" := by
  native_decide
