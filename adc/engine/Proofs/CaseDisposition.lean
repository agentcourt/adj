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

def enterPartialJudgmentAction (issues : Array Lean.Json) (amount : Nat) : CourtAction :=
  { action_type := "enter_partial_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("issues_resolved", Lean.Json.arr issues)
      , ("amount", Lean.Json.num amount)
      , ("basis", Lean.Json.str "no genuine dispute")
      ]
  }

def dismissRule41Action (withPrejudice : Bool) : CourtAction :=
  { action_type := "dismiss_case_rule41"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("with_prejudice", Lean.Json.bool withPrejudice)
      , ("reason", Lean.Json.str "stipulated dismissal")
      ]
  }

def enterSettlementAction (amount : Nat) (consent : Bool) : CourtAction :=
  { action_type := "enter_settlement"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("amount", Lean.Json.num amount)
      , ("consent_judgment", Lean.Json.bool consent)
      , ("summary", Lean.Json.str "settlement agreement reached")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_enter_partial_judgment_requires_pretrial :
    let c := { baseCase with status := "trial" }
    stepErrorMessage (step (stateOf c) (enterPartialJudgmentAction #[Lean.Json.str "liability"] 50000)) =
      "partial judgment requires pretrial status" := by
  native_decide

theorem step_enter_partial_judgment_requires_nonempty_issues :
    stepErrorMessage (step (stateOf) (enterPartialJudgmentAction #[] 50000)) =
      "issues_resolved must contain at least one issue" := by
  native_decide

theorem step_enter_partial_judgment_sets_positive_monetary_amount :
    (match step (stateOf) (enterPartialJudgmentAction #[Lean.Json.str "liability"] 50000) with
      | .ok s' => s'.case.monetary_judgment > 0.0
      | .error _ => false) = true := by
  native_decide

theorem step_dismiss_rule41_closes_case :
    (match step (stateOf) (dismissRule41Action false) with
      | .ok s' => s'.case.status
      | .error _ => "") = "closed" := by
  native_decide

theorem step_enter_settlement_zero_without_consent_closes_case :
    (match step (stateOf) (enterSettlementAction 0 false) with
      | .ok s' => s'.case.status
      | .error _ => "") = "closed" := by
  native_decide

theorem step_enter_settlement_positive_enters_judgment :
    (match step (stateOf) (enterSettlementAction 1000 false) with
      | .ok s' => s'.case.status
      | .error _ => "") = "judgment_entered" := by
  native_decide

theorem step_enter_settlement_consent_enters_judgment_even_zero :
    (match step (stateOf) (enterSettlementAction 0 true) with
      | .ok s' => s'.case.status
      | .error _ => "") = "judgment_entered" := by
  native_decide
