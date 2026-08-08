import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "trial",
    phase := "voir_dire",
    jurors := [
      { juror_id := "J1", name := "Juror One", status := "available", note := "" },
      { juror_id := "J2", name := "Juror Two", status := "available", note := "" }
    ]
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def recordVoirDireAction (jurorId : String) : CourtAction :=
  { action_type := "record_voir_dire_question"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("question", Lean.Json.str "Can you be fair and impartial?")
      , ("response", Lean.Json.str "Yes")
      ]
  }

def challengeForCauseAction (jurorId : String) (granted : Bool) : CourtAction :=
  { action_type := "challenge_juror_for_cause"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("party", Lean.Json.str "plaintiff")
      , ("grounds", Lean.Json.str "expressed inability to follow instructions")
      , ("granted", Lean.Json.bool granted)
      ]
  }

def peremptoryStrikeAction (jurorId : String) : CourtAction :=
  { action_type := "strike_juror_peremptorily"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("party", Lean.Json.str "plaintiff")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_record_voir_dire_requires_trial_status :
    let c := { baseCase with status := "pretrial" }
    stepErrorMessage (step (stateOf c) (recordVoirDireAction "J1")) =
      "record voir dire question requires trial status" := by
  native_decide

theorem step_record_voir_dire_requires_voir_dire_phase :
    let c := { baseCase with phase := "openings" }
    stepErrorMessage (step (stateOf c) (recordVoirDireAction "J1")) =
      "record voir dire question requires voir_dire phase; current phase is openings" := by
  native_decide

theorem step_record_voir_dire_rejects_unknown_juror :
    stepErrorMessage (step (stateOf) (recordVoirDireAction "J9")) =
      "unknown juror_id: J9" := by
  native_decide

theorem step_challenge_for_cause_granted_marks_excused :
    (match step (stateOf) (challengeForCauseAction "J1" true) with
      | .ok s' => (s'.case.jurors.find? (fun j => j.juror_id = "J1")).map (fun j => j.status)
      | .error _ => none) = some "excused_for_cause" := by
  native_decide

theorem step_peremptory_strike_marks_struck :
    (match step (stateOf) (peremptoryStrikeAction "J2") with
      | .ok s' => (s'.case.jurors.find? (fun j => j.juror_id = "J2")).map (fun j => j.status)
      | .error _ => none) = some "struck_peremptory" := by
  native_decide
