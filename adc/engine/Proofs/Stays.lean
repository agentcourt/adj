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

def postBondAction : CourtAction :=
  { action_type := "post_supersedeas_bond"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("effective_until", Lean.Json.str "2026-12-31")
      , ("note", Lean.Json.str "Bond posted")
      ]
  }

def orderStayAction : CourtAction :=
  { action_type := "order_discretionary_stay"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("start_on", Lean.Json.str "2026-01-05")
      , ("end_on", Lean.Json.str "2026-06-30")
      , ("reason", Lean.Json.str "Preserve status quo pending motion")
      ]
  }

def liftStayAction (idx : Nat) : CourtAction :=
  { action_type := "lift_stay"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("stay_index", Lean.Json.num idx)
      , ("reason", Lean.Json.str "Conditions no longer support stay")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_post_supersedeas_bond_records_docket :
    (match step (stateOf) postBondAction with
      | .ok s' => hasDocketTitle s'.case "Supersedeas bond posted"
      | .error _ => false) = true := by
  native_decide

theorem step_order_discretionary_stay_records_docket :
    (match step (stateOf) orderStayAction with
      | .ok s' => hasDocketTitle s'.case "Discretionary Stay Order"
      | .error _ => false) = true := by
  native_decide

theorem step_lift_stay_requires_existing_stay :
    stepErrorMessage (step (stateOf) (liftStayAction 0)) =
      "stay index out of range" := by
  native_decide

theorem step_lift_stay_rejects_already_lifted :
    let c := { baseCase with docket := [
      { title := "Discretionary Stay Order", description := "stay_index=0 start_on=2026-01-05 end_on=2026-06-30 reason=test" },
      { title := "Stay Lifted", description := "stay_index=0 reason=test" }
    ] }
    stepErrorMessage (step (stateOf c) (liftStayAction 0)) =
      "stay already lifted" := by
  native_decide

theorem step_lift_stay_requires_in_order_resolution :
    let c := { baseCase with docket := [
      { title := "Discretionary Stay Order", description := "stay_index=0 start_on=2026-01-05 end_on=2026-06-30 reason=test" },
      { title := "Discretionary Stay Order", description := "stay_index=1 start_on=2026-01-06 end_on=2026-07-01 reason=test" }
    ] }
    stepErrorMessage (step (stateOf c) (liftStayAction 1)) =
      "stays must be lifted in order" := by
  native_decide

theorem step_lift_stay_records_lift_entry_when_valid :
    let c := { baseCase with docket := [
      { title := "Discretionary Stay Order", description := "stay_index=0 start_on=2026-01-05 end_on=2026-06-30 reason=test" }
    ] }
    (match step (stateOf c) (liftStayAction 0) with
      | .ok s' => hasDocketTitle s'.case "Stay Lifted"
      | .error _ => false) = true := by
  native_decide
