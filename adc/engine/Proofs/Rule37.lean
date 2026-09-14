import ADC.Core

namespace ADCProofs.Rule37

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
      , ("relief_sought", Lean.Json.str "Complete responses")
      , ("summary", Lean.Json.str "The response remains incomplete.")
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

def completedPlaintiffDiscovery : CaseState :=
  { baseCase with
    docket := [
      docketEntryWithFields "Interrogatories Served" "plaintiff: served_on=defendant set_index=0 questions=[]"
        [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "0")],
      docketEntryWithFields "Interrogatory Responses" "defendant: responding_party=defendant set_index=0 answers=[]"
        [("responding_party", "defendant"), ("set_index", "0")],
      docketEntryWithFields "Requests for Production Served" "plaintiff: served_on=defendant set_index=0 requests=[]"
        [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "0")],
      docketEntryWithFields "Responses to Requests for Production" "defendant: responding_party=defendant set_index=0 responses=[]"
        [("responding_party", "defendant"), ("set_index", "0")],
      docketEntryWithFields "Requests for Admission Served" "plaintiff: served_on=defendant set_index=0 requests=[]"
        [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "0")],
      docketEntryWithFields "Responses to Requests for Admission" "defendant: responding_party=defendant set_index=0 responses=[]"
        [("responding_party", "defendant"), ("set_index", "0")]
    ],
    decision_traces := [
      { action := "pass_rule37_motion", outcome := "plaintiff:6", citations := ["FRCP 37"] }
    ]
  }

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
      "rule 37 motion does not identify discovery served by the movant on the target party" := by
  native_decide

theorem step_decide_rule37_requires_existing_motion :
    stepErrorMessage (step (stateOf) (decideRule37Action 0 false "none" none)) =
      "rule 37 motion is not the pending motion" := by
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

theorem rule37_pass_survives_opposing_party_discovery :
    let c := { completedPlaintiffDiscovery with docket := completedPlaintiffDiscovery.docket ++ [
      docketEntryWithFields "Interrogatories Served" "defendant: served_on=plaintiff set_index=1 questions=[]"
        [("served_by", "defendant"), ("served_on", "plaintiff"), ("set_index", "1")],
      docketEntryWithFields "Interrogatory Responses" "plaintiff: responding_party=plaintiff set_index=1 answers=[]"
        [("responding_party", "plaintiff"), ("set_index", "1")]
    ] }
    hasRule37PassForCurrentDiscovery c "plaintiff" = true := by
  native_decide

theorem rule37_pass_expires_after_same_party_discovery :
    let c := { completedPlaintiffDiscovery with docket := completedPlaintiffDiscovery.docket ++ [
      docketEntryWithFields "Interrogatories Served" "plaintiff: served_on=defendant set_index=1 questions=[]"
        [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "1")]
    ] }
    hasRule37PassForCurrentDiscovery c "plaintiff" = false := by
  native_decide

def decidedRule37Case : CaseState :=
  let c := { completedPlaintiffDiscovery with decision_traces := [] }
  match step (stateOf c) (fileRule37Action "defendant" "interrogatories" 0 1) with
  | .error _ => default
  | .ok filed =>
      match step filed (decideRule37Action 0 false "none" none) with
      | .error _ => default
      | .ok decided => decided.case

theorem decided_rule37_motion_blocks_same_discovery_generation :
    let facts : TurnFacts := default
    let req : OpportunityRequest :=
      { state := stateOf decidedRule37Case
      , roles := [{ role := "plaintiff", allowed_tools := ["file_rule37_motion"] }]
      , max_steps_per_turn := 3
      }
    (pretrialCandidates req decidedRule37Case facts 3).any
      (fun opportunity =>
        opportunity.role = "plaintiff" &&
        opportunity.allowed_tools = ["file_rule37_motion"]) = false := by
  native_decide

theorem new_discovery_generation_allows_later_rule37_motion :
    let c := { decidedRule37Case with docket := decidedRule37Case.docket ++ [
      docketEntryWithFields "Interrogatories Served" "plaintiff: served_on=defendant set_index=1 questions=[]"
        [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "1")]
    ] }
    let facts : TurnFacts := default
    let req : OpportunityRequest :=
      { state := stateOf c
      , roles := [{ role := "plaintiff", allowed_tools := ["file_rule37_motion"] }]
      , max_steps_per_turn := 3
      }
    (pretrialCandidates req c facts 3).any
      (fun opportunity =>
        opportunity.role = "plaintiff" &&
        opportunity.allowed_tools = ["file_rule37_motion"]) = true := by
  native_decide

end ADCProofs.Rule37
