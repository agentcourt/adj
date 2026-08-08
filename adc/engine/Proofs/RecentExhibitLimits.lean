import Main
import Proofs.EffectiveLimitBasics

open Lean

theorem policyLimitValue_exhibits_eq_max (p : CourtPolicy) :
    policyLimitValue p "trial.exhibits_offered_per_side" = some p.max_exhibits_per_side := by
  simp [policyLimitValue]

theorem enforceMeasuredLimit_exhibits_ok_of_no_overrides
    (s : CourtState) (actor phase nowIso detail : String)
    (attempted : Nat)
    (hOverrides : s.case.local_rule_overrides = [])
    (hle : attempted ≤ s.policy.max_exhibits_per_side) :
    enforceMeasuredLimit s actor phase nowIso "trial.exhibits_offered_per_side" detail attempted =
      .ok s.policy.max_exhibits_per_side := by
  exact enforceMeasuredLimit_no_overrides_ok_of_policy
    s "trial.exhibits_offered_per_side" actor phase nowIso detail attempted
    s.policy.max_exhibits_per_side hOverrides (policyLimitValue_exhibits_eq_max s.policy) hle

theorem enforceMeasuredLimit_exhibits_error_of_no_overrides_gt
    (s : CourtState) (actor phase nowIso detail : String)
    (attempted : Nat)
    (hOverrides : s.case.local_rule_overrides = [])
    (hgt : s.policy.max_exhibits_per_side < attempted) :
    enforceMeasuredLimit s actor phase nowIso "trial.exhibits_offered_per_side" detail attempted =
      .error
        (limitViolationMessage
          "trial.exhibits_offered_per_side"
          actor
          phase
          attempted
          s.policy.max_exhibits_per_side
          detail) := by
  unfold enforceMeasuredLimit
  rw [effectiveLimitValue_no_overrides_eq_policy
    s "trial.exhibits_offered_per_side" actor phase nowIso s.policy.max_exhibits_per_side
    hOverrides (policyLimitValue_exhibits_eq_max s.policy)]
  have hnotle : ¬ attempted ≤ s.policy.max_exhibits_per_side := Nat.not_le.mpr hgt
  simp [hnotle]

def exhibitCaseFile (fileId title : String) : Json :=
  Json.mkObj
    [ ("file_id", Json.str fileId)
    , ("title", Json.str title)
    ]

def offeredExhibitFileEvent (fileId actor : String) : Json :=
  Json.mkObj
    [ ("recorded_at", Json.str "2026-03-16T00:00:00Z")
    , ("action", Json.str "offer_case_file_as_exhibit")
    , ("file_id", Json.str fileId)
    , ("actor", Json.str actor)
    , ("details", Json.str "sample exhibit offer")
    ]

def exhibitEvidenceCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-exhibit-limit"
    filed_on := "2026-03-16"
    status := "trial"
    trial_mode := "jury"
    phase := "plaintiff_evidence"
    case_files :=
      [ exhibitCaseFile "f1" "instructions.txt"
      , exhibitCaseFile "f2" "printing-invoice.txt"
      ]
    file_events := [offeredExhibitFileEvent "f1" "plaintiff"]
    docket :=
      [ { title := "Exhibit PX-1 - admitted"
        , description := "plaintiff: instructions.txt"
        }
      ]
  }

def exhibitEvidenceState (maxExhibits : Nat) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    policy := { max_exhibits_per_side := maxExhibits }
    case := exhibitEvidenceCase
  }

def exhibitEvidenceReq (maxExhibits : Nat) : OpportunityRequest :=
  { state := exhibitEvidenceState maxExhibits
  , roles :=
      [ { role := "plaintiff", allowed_tools := ["offer_case_file_as_exhibit", "rest_case"] }
      , { role := "judge", allowed_tools := ["advance_trial_phase"] }
      ]
  , max_steps_per_turn := 3
  }

def offerPlaintiffExhibitAction : CourtAction :=
  { action_type := "offer_exhibit"
  , actor_role := "plaintiff"
  , payload := Json.mkObj
      [ ("party", Json.str "plaintiff")
      , ("file_id", Json.str "f2")
      , ("exhibit_id", Json.str "PX-2")
      , ("description", Json.str "printing-invoice.txt")
      , ("admitted", Json.bool true)
      , ("offered_at", Json.str "2026-03-16T01:00:00Z")
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

def offerExhibitWithinLimitSummary : Bool :=
  match step (exhibitEvidenceState 2) offerPlaintiffExhibitAction with
  | .ok nextState =>
      (countExhibitsOfferedByParty nextState.case "plaintiff" == 2) &&
      caseFileOfferedByParty nextState.case "plaintiff" "f2" &&
      match nextState.case.limit_usage with
      | [usage] =>
          usage.limit_key == "trial.exhibits_offered_per_side" &&
          usage.actor == "plaintiff" &&
          usage.phase == "plaintiff_evidence" &&
          usage.value == 2
      | _ => false
  | .error _ => false

theorem plaintiff_evidence_exposes_offer_or_rest_below_limit :
    (availableOpportunities (exhibitEvidenceReq 2)).any (fun opportunity =>
      opportunity.role = "plaintiff" &&
      opportunity.phase = "plaintiff_evidence" &&
      opportunity.allowed_tools = ["offer_case_file_as_exhibit", "rest_case"]) := by
  native_decide

theorem plaintiff_evidence_removes_offer_at_limit :
    !(availableOpportunities (exhibitEvidenceReq 1)).any (fun opportunity =>
      opportunity.role = "plaintiff" &&
      opportunity.phase = "plaintiff_evidence" &&
      opportunity.allowed_tools = ["offer_case_file_as_exhibit", "rest_case"]) := by
  native_decide

theorem plaintiff_evidence_exposes_rest_only_at_limit :
    (availableOpportunities (exhibitEvidenceReq 1)).any (fun opportunity =>
      opportunity.role = "plaintiff" &&
      opportunity.phase = "plaintiff_evidence" &&
      opportunity.allowed_tools = ["rest_case"]) := by
  native_decide

theorem step_offer_exhibit_increments_count_within_limit_on_sample :
    offerExhibitWithinLimitSummary := by
  native_decide

theorem step_offer_exhibit_rejects_when_limit_reached_on_sample :
    stepErrorMessage (step (exhibitEvidenceState 1) offerPlaintiffExhibitAction) =
      "LOCAL_RULE_LIMIT_EXCEEDED|limit_key=trial.exhibits_offered_per_side|actor=plaintiff|phase=plaintiff_evidence|attempted=2|allowed=1|detail=exhibits_offered" := by
  native_decide
