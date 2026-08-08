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

def enterProtectiveOrderAction (orderId : String) (allowedRoles : Array Lean.Json := #[Lean.Json.str "plaintiff", Lean.Json.str "defendant"]) : CourtAction :=
  { action_type := "enter_protective_order"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("order_id", Lean.Json.str orderId)
      , ("scope", Lean.Json.str "category")
      , ("target", Lean.Json.str "discovery")
      , ("allowed_roles", Lean.Json.arr allowedRoles)
      , ("note", Lean.Json.str "Confidential business records")
      ]
  }

def liftProtectiveOrderAction (orderId : String) : CourtAction :=
  { action_type := "lift_protective_order"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("order_id", Lean.Json.str orderId)
      , ("note", Lean.Json.str "No longer needed")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_enter_protective_order_requires_nonempty_allowed_roles :
    let action := enterProtectiveOrderAction "po-1" #[]
    stepErrorMessage (step (stateOf) action) =
      "allowed_roles must contain at least one role" := by
  native_decide

theorem step_enter_protective_order_rejects_duplicate_order_id :
    let c := { baseCase with
      protective_orders :=
        [ { entered_at := "2026-01-01", order_id := "po-1", scope := "category", target := "discovery", allowed_roles := ["plaintiff"], note := "", active := true, lifted_at := none } ] }
    stepErrorMessage (step (stateOf c) (enterProtectiveOrderAction "po-1")) =
      "duplicate protective order id: po-1" := by
  native_decide

theorem step_enter_protective_order_records_docket :
    (match step (stateOf) (enterProtectiveOrderAction "po-1") with
      | .ok s' => hasDocketTitle s'.case "Protective Order po-1"
      | .error _ => false) = true := by
  native_decide

theorem step_lift_protective_order_requires_order_id :
    stepErrorMessage (step (stateOf) (liftProtectiveOrderAction "")) =
      "order_id is required" := by
  native_decide

theorem step_lift_protective_order_requires_existing_order :
    stepErrorMessage (step (stateOf) (liftProtectiveOrderAction "po-missing")) =
      "unknown protective order: po-missing" := by
  native_decide

theorem step_lift_protective_order_records_docket :
    let c := { baseCase with
      protective_orders :=
        [ { entered_at := "2026-01-01", order_id := "po-1", scope := "category", target := "discovery", allowed_roles := ["plaintiff"], note := "", active := true, lifted_at := none } ] }
    (match step (stateOf c) (liftProtectiveOrderAction "po-1") with
      | .ok s' => hasDocketTitle s'.case "Protective Order po-1 lifted"
      | .error _ => false) = true := by
  native_decide
