import Main

def filedCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "filed",
    trial_mode := "unset",
    phase := "none",
    filed_on := "2026-01-01"
  }

def pretrialCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "pretrial",
    trial_mode := "jury",
    phase := "discovery",
    filed_on := "2026-01-01"
  }

def reqWithRoles (c : CaseState) (roles : List RolePolicy) : OpportunityRequest :=
  { state := { (default : CourtState) with court_name := "Test Court", case := c },
    roles := roles,
    max_steps_per_turn := 3
  }

theorem filedCandidates_offers_file_complaint_when_missing :
    let c := filedCase
    let facts : TurnFacts := default
    let req := reqWithRoles c [{ role := "plaintiff", allowed_tools := ["file_complaint"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "plaintiff" ∧ t.allowed_tools = ["file_complaint"]) = true := by
  native_decide

theorem filedCandidates_offers_enter_default_when_answer_missing :
    let c := filedCase
    let facts : TurnFacts := { (default : TurnFacts) with hasComplaint := true, hasAnswer := false, hasDefaultEntered := false }
    let req := reqWithRoles c [{ role := "judge", allowed_tools := ["enter_default"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "judge" ∧ t.allowed_tools = ["enter_default"]) = true := by
  native_decide

theorem filedCandidates_offers_enter_default_judgment_after_default :
    let c := filedCase
    let facts : TurnFacts := { (default : TurnFacts) with hasDefaultEntered := true, hasDefaultJudgment := false }
    let req := reqWithRoles c [{ role := "judge", allowed_tools := ["enter_default_judgment"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "judge" ∧ t.allowed_tools = ["enter_default_judgment"]) = true := by
  native_decide

/--
When the complaint has been filed and no answer or Rule 12 motion exists yet,
`filedCandidates` includes the defendant's Rule 12 opportunity if the role
policy allows that tool.

The proof plan is concrete because this file tracks candidate-generation facts
at the builder boundary.  Instantiate the filed-phase facts for the ordinary
unanswered-complaint posture, give the defendant the Rule 12 tool, and compute
the candidate list.  The theorem checks that the expected Rule 12 opportunity
appears in that list.
-/
theorem filedCandidates_offers_rule12_when_complaint_unanswered :
    let c := filedCase
    let facts : TurnFacts := { (default : TurnFacts) with hasComplaint := true, hasRule12Motion := false, hasAnswer := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule12_motion"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["file_rule12_motion"]) = true := by
  native_decide

/-
This is a small support lemma, not a headline result.  It exists because the
next ordering theorem needs the party-side filed candidate to be explicit.
There is no value in making this proof harder than the candidate builder
itself.
-/

theorem filedCandidates_rule11_motion_requires_notice_and_no_correction :
    let c := { filedCase with auto_rule11 := true }
    let facts : TurnFacts := { (default : TurnFacts) with hasRule11Notice := true, hasRule11Correction := false, hasRule11Motion := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule11_motion"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["file_rule11_motion"]) = true := by
  native_decide

theorem filedCandidates_rule11_motion_not_offered_after_correction :
    let c := { filedCase with auto_rule11 := true }
    let facts : TurnFacts := { (default : TurnFacts) with hasRule11Notice := true, hasRule11Correction := true, hasRule11Motion := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule11_motion"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["file_rule11_motion"]) = false := by
  native_decide

theorem pretrialCandidates_offers_respond_rfp_when_served_pending :
    let c := pretrialCase
    let facts : TurnFacts := { (default : TurnFacts) with hasRfpServed := true, hasRfpResponses := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["respond_request_for_production"] }]
    (pretrialCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["respond_request_for_production"]) = true := by
  native_decide

theorem pretrialCandidates_offers_decide_rule37_when_motion_pending :
    let c := pretrialCase
    let facts : TurnFacts := { (default : TurnFacts) with hasRule37Motion := true, hasRule37Order := false }
    let req := reqWithRoles c [{ role := "judge", allowed_tools := ["decide_rule37_motion"] }]
    (pretrialCandidates req c facts 3).any
      (fun t => t.role = "judge" ∧ t.allowed_tools = ["decide_rule37_motion"]) = true := by
  native_decide
