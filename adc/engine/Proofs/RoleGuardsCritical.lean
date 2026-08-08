import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "trial",
    phase := "post_verdict"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def enterJudgmentActionAs (role : String) : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "jury verdict")
      ]
  }

def recordJuryVerdictActionAs (role : String) : CourtAction :=
  { action_type := "record_jury_verdict"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("verdict_for", Lean.Json.str "plaintiff")
      , ("votes_for_verdict", Lean.Json.num 6)
      , ("damages", Lean.Json.num 100)
      ]
  }

def declareHungJuryActionAs (role : String) : CourtAction :=
  { action_type := "declare_hung_jury"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("note", Lean.Json.str "deadlock")
      ]
  }

def resolveRule59ActionAs (role : String) : CourtAction :=
  { action_type := "resolve_rule59_motion"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num 0)
      , ("granted", Lean.Json.bool false)
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_enter_judgment_rejects_non_judge_role :
    stepErrorMessage (step (stateOf) (enterJudgmentActionAs "plaintiff")) =
      "role plaintiff not permitted for enter_judgment" := by
  native_decide

theorem step_record_jury_verdict_rejects_non_foreperson_role :
    stepErrorMessage (step (stateOf) (recordJuryVerdictActionAs "judge")) =
      "role judge not permitted for record_jury_verdict" := by
  native_decide

theorem step_declare_hung_jury_rejects_non_foreperson_role :
    stepErrorMessage (step (stateOf) (declareHungJuryActionAs "defendant")) =
      "role defendant not permitted for declare_hung_jury" := by
  native_decide

theorem step_resolve_rule59_rejects_non_judge_role :
    let c := { baseCase with status := "judgment_entered" }
    stepErrorMessage (step (stateOf c) (resolveRule59ActionAs "defendant")) =
      "role defendant not permitted for resolve_rule59_motion" := by
  native_decide
