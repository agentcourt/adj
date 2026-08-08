import Main

open Lean

def internationalClawProfileRecent : Lean.Json :=
  Lean.Json.mkObj
    [ ("name", Lean.Json.str "International Claw District")
    , ("rules_markdown", Lean.Json.str "General civil jurisdiction applies.")
    , ("jurisdiction_screen", Lean.Json.bool false)
    , ("allowed_jurisdiction_bases", Lean.toJson (["general_civil"] : List String))
    , ("preferred_jurisdiction_basis", Lean.Json.str "general_civil")
    , ("require_jurisdiction_statement", Lean.Json.bool true)
    , ("require_diversity_citizenship", Lean.Json.bool false)
    , ("require_amount_in_controversy", Lean.Json.bool false)
    , ("minimum_amount_in_controversy", Lean.toJson (0 : Nat))
    ]

def unitedStatesDistrictProfileRecent : Lean.Json :=
  Lean.Json.mkObj
    [ ("name", Lean.Json.str "United States District")
    , ("rules_markdown", Lean.Json.str "Federal civil procedure applies.")
    , ("jurisdiction_screen", Lean.Json.bool true)
    , ("allowed_jurisdiction_bases", Lean.toJson (["federal_question", "diversity"] : List String))
    , ("preferred_jurisdiction_basis", Lean.Json.str "diversity")
    , ("require_jurisdiction_statement", Lean.Json.bool true)
    , ("require_diversity_citizenship", Lean.Json.bool true)
    , ("require_amount_in_controversy", Lean.Json.bool true)
    , ("minimum_amount_in_controversy", Lean.toJson (75000 : Nat))
    ]

def internationalClawFiledCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1"
    status := "filed"
    jurisdictional_allegations := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis", Lean.Json.str "general_civil")
      , ("jurisdictional_statement", Lean.Json.str "This Court has jurisdiction over this matter on a general civil basis.")
      , ("amount_in_controversy", Lean.Json.str "108")
      ]
    decision_traces := [{ action := "file_complaint", outcome := "filed", citations := [] }]
  }

def internationalClawFiledState : CourtState :=
  { (default : CourtState) with
    court_name := "International Claw District"
    court_profile := some internationalClawProfileRecent
    case := internationalClawFiledCase
  }

def internationalClawFiledReq : OpportunityRequest :=
  { state := internationalClawFiledState
  , roles :=
      [ { role := "judge", allowed_tools := ["dismiss_for_lack_of_subject_matter_jurisdiction"] }
      , { role := "defendant", allowed_tools := ["file_rule12_motion", "file_answer"] }
      ]
  , max_steps_per_turn := 3
  }

def unitedStatesDistrictFiledCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-2"
    status := "filed"
    jurisdictional_allegations := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis", Lean.Json.str "diversity")
      , ("jurisdictional_statement", Lean.Json.str "This Court has diversity jurisdiction.")
      , ("plaintiff_residence", Lean.Json.str "Texas")
      , ("defendant_residence", Lean.Json.str "Massachusetts")
      , ("amount_in_controversy", Lean.Json.str "108")
      ]
    decision_traces := [{ action := "file_complaint", outcome := "filed", citations := [] }]
  }

def unitedStatesDistrictFiledState : CourtState :=
  { (default : CourtState) with
    court_name := "United States District"
    court_profile := some unitedStatesDistrictProfileRecent
    case := unitedStatesDistrictFiledCase
  }

def unitedStatesDistrictFiledReq : OpportunityRequest :=
  { state := unitedStatesDistrictFiledState
  , roles :=
      [ { role := "judge", allowed_tools := ["dismiss_for_lack_of_subject_matter_jurisdiction"] }
      , { role := "defendant", allowed_tools := ["file_rule12_motion", "file_answer"] }
      ]
  , max_steps_per_turn := 3
  }

/--
The International Claw District disables the subject-matter-jurisdiction Rule 12 ground.

This matters because the same Rule 12 path runs in both courts.  The active court
profile must remove the federal jurisdiction ground when the court does not use a
jurisdiction screen.
-/
theorem validRule12Ground_internationalClaw_disables_subject_matter_jurisdiction :
    validRule12Ground internationalClawFiledState "lack_subject_matter_jurisdiction" = false := by
  native_decide

/--
The Rule 12 ground summary for the International Claw District omits
subject-matter jurisdiction.

This theorem states the exact defendant-facing summary text that the engine uses
when it generates the optional Rule 12 opportunity in that court.
-/
theorem rule12GroundSummary_internationalClaw_omits_subject_matter_jurisdiction :
    rule12GroundSummary internationalClawFiledState =
      "no standing, not ripe, moot, or failure to state a claim" := by
  native_decide

/--
In a filed International Claw case, the next opportunity is the defendant's
optional Rule 12 motion rather than a judge jurisdiction dismissal.

This is the operational effect of disabling the jurisdiction screen.  The engine
still exposes Rule 12, but it does not insert a dismissal opportunity ahead of
the defendant's pleading choice.
-/
theorem nextOpportunity_internationalClaw_filed_case_selects_defendant_rule12 :
    (nextOpportunity internationalClawFiledReq).opportunity.map
        (fun opportunity => (opportunity.role, opportunity.allowed_tools, opportunity.kind)) =
      some ("defendant", ["file_rule12_motion"], "optional") := by
  native_decide

/--
The open opportunity set in the International Claw filed case contains no
subject-matter-jurisdiction dismissal tool.

This theorem rules out the unwanted federal screen directly at the opportunity
level rather than only through helper functions.
-/
theorem openOpportunities_internationalClaw_have_no_jurisdiction_dismissal :
    (openOpportunities internationalClawFiledReq).any
        (fun opportunity => opportunity.allowed_tools.contains "dismiss_for_lack_of_subject_matter_jurisdiction") =
      false := by
  native_decide

/--
The federal profile still treats defective diversity pleading as a facial
subject-matter-jurisdiction defect.

This is the contrast with `International Claw District`.  Residence allegations
and a `$108` amount do not satisfy the federal diversity screen.
-/
theorem subjectMatterJurisdictionFaciallyDefective_unitedStatesDistrict_detects_defective_diversity :
    subjectMatterJurisdictionFaciallyDefective unitedStatesDistrictFiledState = true := by
  native_decide

/--
In the federal profile, the next opportunity is the judge's jurisdiction
dismissal when diversity pleading is facially defective.

This theorem proves the other half of the court split: the federal profile
still inserts the Rule 12(h)(3) dismissal opportunity ahead of the defendant's
pleading choice.
-/
theorem nextOpportunity_unitedStatesDistrict_filed_case_selects_jurisdiction_dismissal :
    (nextOpportunity unitedStatesDistrictFiledReq).opportunity.map
        (fun opportunity => (opportunity.role, opportunity.allowed_tools, opportunity.kind)) =
      some ("judge", ["dismiss_for_lack_of_subject_matter_jurisdiction"], "optional") := by
  native_decide

/-
These court-profile theorems prove the operational effect of the new court
split.

They do not stop at static configuration values.  They prove that the profile
changes the actual opportunity stream seen by the judge and defendant.
-/
