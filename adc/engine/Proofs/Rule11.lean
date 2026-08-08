import Main

def baseRule11Case : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "pretrial",
    phase := "pleadings",
    docket := [{ title := "Rule 11 Motion", description := "filed" }]
  }

def rule11State (c : CaseState := baseRule11Case) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def decideRule11DeniedWithSanctionAction : CourtAction :=
  { action_type := "decide_rule11_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num 0)
      , ("granted", Lean.Json.bool false)
      , ("sanction_type", Lean.Json.str "admonition")
      , ("reasoning", Lean.Json.str "The motion fails, so sanctions are not available.")
      ]
  }

def decideRule11GrantedMonetaryNoAmountAction : CourtAction :=
  { action_type := "decide_rule11_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num 0)
      , ("granted", Lean.Json.bool true)
      , ("sanction_type", Lean.Json.str "monetary_penalty")
      , ("reasoning", Lean.Json.str "A monetary sanction is warranted.")
      ]
  }

def decideRule11DeniedNoSanctionAction : CourtAction :=
  { action_type := "decide_rule11_motion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num 0)
      , ("granted", Lean.Json.bool false)
      , ("reasoning", Lean.Json.str "The motion is denied on the merits.")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_decide_rule11_denied_cannot_include_sanction :
    stepErrorMessage (step (rule11State) decideRule11DeniedWithSanctionAction) =
      "denied rule 11 motion cannot include sanctions" := by
  native_decide

theorem step_decide_rule11_granted_monetary_requires_amount :
    stepErrorMessage (step (rule11State) decideRule11GrantedMonetaryNoAmountAction) =
      "monetary sanctions require positive sanction amount" := by
  native_decide

theorem step_decide_rule11_denied_without_sanction_records_order :
    (match step (rule11State) decideRule11DeniedNoSanctionAction with
      | .ok s' => hasDocketTitle s'.case "Rule 11 Order"
      | .error _ => false) = true := by
  native_decide
