import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "trial",
    phase := "plaintiff_case"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def enterOverrideActionAs (role : String) (overrideId : String := "") : CourtAction :=
  { action_type := "enter_local_rule_override"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("limit_key", Lean.Json.str "text.closing_chars_per_side")
      , ("new_value", Lean.Json.num 7000)
      , ("ordered_by", Lean.Json.str "judge")
      , ("reason", Lean.Json.str "case complexity")
      , ("override_id", Lean.Json.str overrideId)
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_enter_local_rule_override_rejects_non_judge :
    stepErrorMessage (step (stateOf) (enterOverrideActionAs "plaintiff")) =
      "role plaintiff not permitted for enter_local_rule_override" := by
  native_decide

theorem step_enter_local_rule_override_assigns_default_id_when_blank :
    (match step (stateOf) (enterOverrideActionAs "judge" "") with
      | .ok s' => (s'.case.local_rule_overrides.head?.map (fun o => o.override_id))
      | .error _ => none) = some "lro-1" := by
  native_decide

theorem step_enter_local_rule_override_records_docket :
    (match step (stateOf) (enterOverrideActionAs "judge" "") with
      | .ok s' => hasDocketTitle s'.case "Local Rule Override lro-1"
      | .error _ => false) = true := by
  native_decide

theorem step_enter_local_rule_override_respects_explicit_id :
    (match step (stateOf) (enterOverrideActionAs "judge" "override-xyz") with
      | .ok s' => (s'.case.local_rule_overrides.head?.map (fun o => o.override_id))
      | .error _ => none) = some "override-xyz" := by
  native_decide
