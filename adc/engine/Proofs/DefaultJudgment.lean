import Main

def defaultCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "pretrial",
    phase := "pleadings"
  }

def defaultState (c : CaseState := defaultCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def defaultJudgmentAction (againstParty : String) : CourtAction :=
  let payload := Lean.Json.mkObj [
    ("against_party", Lean.Json.str againstParty),
    ("reason", Lean.Json.str "failure to plead or defend")
  ]
  { action_type := "enter_default_judgment"
  , actor_role := "judge"
  , payload := payload
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_enter_default_judgment_rejects_invalid_against_party :
    stepErrorMessage (step (defaultState) (defaultJudgmentAction "third_party")) =
      "invalid against_party: third_party" := by
  native_decide

theorem step_enter_default_judgment_success_sets_status :
    (match step (defaultState) (defaultJudgmentAction "defendant") with
      | .ok s' => s'.case.status
      | .error _ => "") = "judgment_entered" := by
  native_decide

theorem step_enter_default_judgment_success_records_docket :
    (match step (defaultState) (defaultJudgmentAction "defendant") with
      | .ok s' => hasDocketTitle s'.case "Default judgment entered"
      | .error _ => false) = true := by
  native_decide
