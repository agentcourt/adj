import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "judgment_entered",
    phase := "post_verdict"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def fileRule59Action (judgmentDateOpt : Option String) (filedAt : String) : CourtAction :=
  let base := Lean.Json.mkObj [ ("filed_at", Lean.Json.str filedAt) ]
  let payload :=
    match judgmentDateOpt with
    | some d => base.mergeObj (Lean.Json.mkObj [ ("last_judgment_date", Lean.Json.str d) ])
    | none => base
  { action_type := "file_rule59_motion", actor_role := "defendant", payload := payload }

def resolveRule59Action (motionIndex : Nat) : CourtAction :=
  { action_type := "resolve_rule59_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num motionIndex)
      , ("granted", Lean.Json.bool false)
      , ("order_text", Lean.Json.str "Denied")
      ]
  }

def fileRule60Action (judgmentDateOpt : Option String) (ground filedAt : String) : CourtAction :=
  let base := Lean.Json.mkObj [ ("ground", Lean.Json.str ground), ("filed_at", Lean.Json.str filedAt) ]
  let payload :=
    match judgmentDateOpt with
    | some d => base.mergeObj (Lean.Json.mkObj [ ("last_judgment_date", Lean.Json.str d) ])
    | none => base
  { action_type := "file_rule60_motion", actor_role := "defendant", payload := payload }

def resolveRule60Action (motionIndex : Nat) : CourtAction :=
  { action_type := "resolve_rule60_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num motionIndex)
      , ("granted", Lean.Json.bool false)
      , ("relief_summary", Lean.Json.str "No relief")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_file_rule59_requires_judgment_date :
    stepErrorMessage (step (stateOf) (fileRule59Action none "2026-01-15")) =
      "rule 59 motion requires entered judgment" := by
  native_decide

theorem step_file_rule59_enforces_timeliness :
    stepErrorMessage (step (stateOf) (fileRule59Action (some "2026-01-01") "2026-03-15")) =
      "rule 59 motion is untimely" := by
  native_decide

theorem step_resolve_rule59_requires_existing_motion :
    stepErrorMessage (step (stateOf) (resolveRule59Action 0)) =
      "rule 59 motion index out of range" := by
  native_decide

theorem step_resolve_rule59_must_be_in_order :
    let c := { baseCase with docket := [
      { title := "Rule 59 Motion", description := "filed" },
      { title := "Rule 59 Motion", description := "filed" }
    ] }
    stepErrorMessage (step (stateOf c) (resolveRule59Action 1)) =
      "rule 59 motions must be resolved in order" := by
  native_decide

theorem step_file_rule60_requires_judgment_date :
    stepErrorMessage (step (stateOf) (fileRule60Action none "60b1_mistake" "2026-01-15")) =
      "rule 60 motion requires entered judgment" := by
  native_decide

theorem step_file_rule60_enforces_one_year_window_for_60b1_to_60b3 :
    stepErrorMessage (step (stateOf) (fileRule60Action (some "2026-01-01") "60b1_mistake" "2027-03-01")) =
      "rule 60(b)(1)-(3) motion is untimely" := by
  native_decide

theorem step_resolve_rule60_records_order_when_valid :
    let c := { baseCase with docket := [ { title := "Rule 60 Motion", description := "filed" } ] }
    (match step (stateOf c) (resolveRule60Action 0) with
      | .ok s' => hasDocketTitle s'.case "Rule 60 Order"
      | .error _ => false) = true := by
  native_decide
