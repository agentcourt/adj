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

def fileRule37Action (targetParty discoveryType : String) (setIndex setCount : Nat) : CourtAction :=
  { action_type := "file_rule37_motion"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("movant", Lean.Json.str "plaintiff")
      , ("target_party", Lean.Json.str targetParty)
      , ("discovery_type", Lean.Json.str discoveryType)
      , ("set_index", Lean.Json.num setIndex)
      , ("discovery_set_count", Lean.Json.num setCount)
      ]
  }

def decideRule37Action (motionIndex : Nat) (granted : Bool) (sanctionType : String) (sanctionAmount : Option Nat) : CourtAction :=
  let base := Lean.Json.mkObj
    [ ("motion_index", Lean.Json.num motionIndex)
    , ("granted", Lean.Json.bool granted)
    , ("sanction_type", Lean.Json.str sanctionType)
    , ("reasoning", Lean.Json.str "The Rule 37 disposition follows from the discovery record.")
    ]
  let payload :=
    match sanctionAmount with
    | some n => base.mergeObj (Lean.Json.mkObj [ ("sanction_amount", Lean.Json.num n) ])
    | none => base
  { action_type := "decide_rule37_motion", actor_role := "judge", payload := payload }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_file_rule37_target_must_be_opposing_party :
    stepErrorMessage (step (stateOf) (fileRule37Action "plaintiff" "interrogatories" 0 1)) =
      "rule 37 motion target must be the opposing party" := by
  native_decide

theorem step_file_rule37_rejects_invalid_discovery_type :
    stepErrorMessage (step (stateOf) (fileRule37Action "defendant" "depositions" 0 1)) =
      "discovery_type must be interrogatories, rfp, rfa, or initial_disclosures" := by
  native_decide

theorem step_file_rule37_checks_interrogatory_set_range :
    stepErrorMessage (step (stateOf) (fileRule37Action "defendant" "interrogatories" 1 1)) =
      "interrogatory set index out of range" := by
  native_decide

theorem step_decide_rule37_requires_existing_motion :
    stepErrorMessage (step (stateOf) (decideRule37Action 0 false "none" none)) =
      "rule 37 motion index out of range" := by
  native_decide

theorem step_decide_rule37_denied_cannot_include_sanctions :
    let c := { baseCase with docket := [{ title := "Rule 37 Motion", description := "filed" }] }
    stepErrorMessage (step (stateOf c) (decideRule37Action 0 false "fees" (some 500))) =
      "denied rule 37 motion cannot include sanctions" := by
  native_decide

theorem step_decide_rule37_fees_requires_positive_amount :
    let c := { baseCase with docket := [{ title := "Rule 37 Motion", description := "filed" }] }
    stepErrorMessage (step (stateOf c) (decideRule37Action 0 true "fees" none)) =
      "fees sanction requires positive sanction_amount" := by
  native_decide

theorem step_decide_rule37_records_order_when_valid :
    let c := { baseCase with docket := [{ title := "Rule 37 Motion", description := "filed" }] }
    (match step (stateOf c) (decideRule37Action 0 true "fees" (some 750)) with
      | .ok s' => hasDocketTitle s'.case "Rule 37 Order"
      | .error _ => false) = true := by
  native_decide
