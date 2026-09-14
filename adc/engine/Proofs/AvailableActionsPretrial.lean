import ADC.Core

namespace ADCProofs.AvailableActionsPretrial

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

def plaintiffDiscoveryCase : CaseState :=
  { pretrialCase with
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
      { action := "serve_interrogatories", outcome := "served", citations := ["FRCP 33"] },
      { action := "respond_interrogatories", outcome := "served", citations := ["FRCP 33(b)"] },
      { action := "serve_request_for_production", outcome := "served", citations := ["FRCP 34"] },
      { action := "respond_request_for_production", outcome := "served", citations := ["FRCP 34"] },
      { action := "serve_requests_for_admission", outcome := "served", citations := ["FRCP 36"] },
      { action := "respond_requests_for_admission", outcome := "served", citations := ["FRCP 36"] }
    ]
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

theorem filedCandidates_rule11_motion_requires_notice_and_no_correction :
    let c := { filedCase with
      auto_rule11 := true,
      docket := [docketEntryWithFields "Rule 11 Safe Harbor Notice"
        "served_by=defendant target_party=plaintiff challenged_filing=complaint served_at=2026-01-01"
        [("served_by", "defendant"), ("target_party", "plaintiff"),
         ("challenged_filing", "complaint"), ("served_at", "2026-01-01")]]
    }
    let facts : TurnFacts := { (default : TurnFacts) with hasRule11Notice := true, hasRule11Correction := false, hasRule11Motion := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule11_motion"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["file_rule11_motion"] ∧
        t.deterministic_action.isNone) = true := by
  native_decide

theorem filedCandidates_rule11_motion_not_offered_after_correction :
    let c := { filedCase with
      auto_rule11 := true,
      docket := [
        docketEntryWithFields "Rule 11 Safe Harbor Notice"
          "served_by=defendant target_party=plaintiff challenged_filing=complaint served_at=2026-01-01"
          [("served_by", "defendant"), ("target_party", "plaintiff"),
           ("challenged_filing", "complaint"), ("served_at", "2026-01-01")],
        docketEntryWithFields "Withdrawal or Correction" "notice_index=0 by_party=plaintiff"
          [("notice_index", "0"), ("by_party", "plaintiff")]
      ]
    }
    let facts : TurnFacts := { (default : TurnFacts) with hasRule11Notice := true, hasRule11Correction := true, hasRule11Motion := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule11_motion"] }]
    (filedCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["file_rule11_motion"]) = false := by
  native_decide

theorem pretrialCandidates_offers_respond_rfp_when_served_pending :
    let c := { pretrialCase with docket := [
      docketEntryWithFields "Requests for Production Served" "plaintiff: served_on=defendant set_index=0 requests=[]"
        [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "0")]
    ] }
    let facts : TurnFacts := { (default : TurnFacts) with hasRfpServed := true, hasRfpResponses := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["respond_request_for_production"] }]
    (pretrialCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["respond_request_for_production"]) = true := by
  native_decide

theorem pretrialCandidates_offers_decide_rule37_when_motion_pending :
    let c := { pretrialCase with docket := [
      docketEntryWithFields "Rule 37 Motion"
        "movant=plaintiff target_party=defendant discovery_type=interrogatories set_index=0 discovery_generation=0"
        [("movant", "plaintiff"), ("target_party", "defendant"),
         ("discovery_type", "interrogatories"), ("set_index", "0"),
         ("discovery_generation", "0")]
    ] }
    let facts : TurnFacts := { (default : TurnFacts) with hasRule37Motion := true, hasRule37Order := false }
    let req := reqWithRoles c [{ role := "judge", allowed_tools := ["decide_rule37_motion"] }]
    (pretrialCandidates req c facts 3).any
      (fun t => t.role = "judge" ∧ t.allowed_tools = ["decide_rule37_motion"] ∧
        t.deterministic_action.isNone) = true := by
  native_decide

theorem pretrialCandidates_does_not_repeat_rule37_after_pass :
    let c := { plaintiffDiscoveryCase with decision_traces := plaintiffDiscoveryCase.decision_traces ++ [
      { action := "pass_rule37_motion", outcome := "plaintiff:6", citations := ["FRCP 37"] }
    ] }
    let facts : TurnFacts := { (default : TurnFacts) with hasRule37Motion := false }
    let req := reqWithRoles c [{ role := "plaintiff", allowed_tools := ["file_rule37_motion"] }]
    (pretrialCandidates req c facts 3).any
      (fun t => t.role = "plaintiff" ∧ t.allowed_tools = ["file_rule37_motion"]) = false := by
  native_decide

theorem pretrialCandidates_reopens_rule37_after_discovery_changes :
    let passed := { plaintiffDiscoveryCase with
      docket := plaintiffDiscoveryCase.docket ++ [
        docketEntryWithFields "Interrogatories Served" "plaintiff: served_on=defendant set_index=1 questions=[]"
          [("served_by", "plaintiff"), ("served_on", "defendant"), ("set_index", "1")]
      ],
      decision_traces := plaintiffDiscoveryCase.decision_traces ++ [
        { action := "pass_rule37_motion", outcome := "plaintiff:6", citations := ["FRCP 37"] }
      ]
    }
    let facts : TurnFacts := default
    let req := reqWithRoles passed [{ role := "plaintiff", allowed_tools := ["file_rule37_motion"] }]
    (pretrialCandidates req passed facts 3).any
      (fun t => t.role = "plaintiff" ∧ t.allowed_tools = ["file_rule37_motion"]) = true := by
  native_decide

end ADCProofs.AvailableActionsPretrial
