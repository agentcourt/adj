import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "trial",
    phase := "voir_dire"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def setJuryConfigurationAction (jurorCount : Nat) (unanimous : Bool) (minimum : Nat) : CourtAction :=
  { action_type := "set_jury_configuration"
  , actor_role := "clerk"
  , payload := Lean.Json.mkObj
      [ ("juror_count", Lean.Json.num jurorCount)
      , ("unanimous_required", Lean.Json.bool unanimous)
      , ("minimum_concurring", Lean.Json.num minimum)
      ]
  }

def addJurorAction (jurorId name : String) : CourtAction :=
  { action_type := "add_juror"
  , actor_role := "clerk"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("name", Lean.Json.str name)
      ]
  }

def swearJuryAction : CourtAction :=
  { action_type := "swear_jury", actor_role := "clerk", payload := Lean.Json.mkObj [] }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_set_jury_configuration_rejects_size_too_small :
    stepErrorMessage (step (stateOf) (setJuryConfigurationAction 5 true 0)) =
      "jury size must be between 6 and 12" := by
  native_decide

theorem step_set_jury_configuration_rejects_invalid_minimum :
    stepErrorMessage (step (stateOf) (setJuryConfigurationAction 8 false 5)) =
      "minimum concurring jurors must be between 6 and jury size" := by
  native_decide

theorem step_add_juror_rejects_duplicate_id :
    let c := { baseCase with jurors := [{ juror_id := "J1", name := "Juror One", status := "available", note := "" }] }
    stepErrorMessage (step (stateOf c) (addJurorAction "J1" "Duplicate")) =
      "duplicate juror id: J1" := by
  native_decide

theorem step_swear_jury_requires_configuration :
    stepErrorMessage (step (stateOf) swearJuryAction) =
      "jury configuration required before swearing jury" := by
  native_decide

theorem step_swear_jury_requires_enough_available_jurors :
    let c := { baseCase with
      jury_configuration := some { juror_count := 6, unanimous_required := true, minimum_concurring := 6 },
      jurors := [{ juror_id := "J1", name := "Juror One", status := "available", note := "" }] }
    stepErrorMessage (step (stateOf c) swearJuryAction) =
      "insufficient available jurors to swear jury" := by
  native_decide

theorem step_swear_jury_marks_required_count_sworn :
    let c := { baseCase with
      jury_configuration := some { juror_count := 2, unanimous_required := true, minimum_concurring := 2 },
      jurors := [
        { juror_id := "J1", name := "Juror One", status := "available", note := "" },
        { juror_id := "J2", name := "Juror Two", status := "available", note := "" },
        { juror_id := "J3", name := "Juror Three", status := "available", note := "" }
      ] }
    (match step (stateOf c) swearJuryAction with
      | .ok s' => (s'.case.jurors.filter (fun j => j.status = "sworn")).length
      | .error _ => 0) = 2 := by
  native_decide
