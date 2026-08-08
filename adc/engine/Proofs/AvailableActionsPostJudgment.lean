import Main

def judgmentCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "judgment_entered",
    phase := "post_verdict"
  }

def reqWithRoles (c : CaseState) (roles : List RolePolicy) : OpportunityRequest :=
  { state := { (default : CourtState) with court_name := "Test Court", case := c },
    roles := roles,
    max_steps_per_turn := 3
  }

theorem postJudgmentCandidates_offers_rule59_when_missing :
    let c := judgmentCase
    let facts : TurnFacts := default
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule59_motion"] }]
    (postJudgmentCandidates req c facts 3).any
      (fun t => t.role = "defendant" ∧ t.allowed_tools = ["file_rule59_motion"]) = true := by
  native_decide

theorem postJudgmentCandidates_offers_resolve_rule59_when_pending :
    let c := judgmentCase
    let facts : TurnFacts := { (default : TurnFacts) with hasRule59Motion := true, hasRule59Order := false }
    let req := reqWithRoles c [{ role := "judge", allowed_tools := ["resolve_rule59_motion"] }]
    (postJudgmentCandidates req c facts 3).any
      (fun t => t.role = "judge" ∧ t.allowed_tools = ["resolve_rule59_motion"]) = true := by
  native_decide

theorem postJudgmentCandidates_offers_rule60_default_judgment_track :
    let c := judgmentCase
    let facts : TurnFacts := { (default : TurnFacts) with hasDefaultJudgment := true, hasRule60Motion := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule60_motion"] }]
    (postJudgmentCandidates req c facts 3).any
      (fun t =>
        t.role = "defendant" ∧
        t.objective = "For case 0, if relief from default judgment is sought, file Rule 60 motion with ground and a timely filed_at date." ∧
        t.allowed_tools = ["file_rule60_motion"]) = true := by
  native_decide

theorem postJudgmentCandidates_offers_rule60_general_track :
    let c := judgmentCase
    let facts : TurnFacts := { (default : TurnFacts) with hasDefaultJudgment := false, hasRule60Motion := false }
    let req := reqWithRoles c [{ role := "defendant", allowed_tools := ["file_rule60_motion"] }]
    (postJudgmentCandidates req c facts 3).any
      (fun t =>
        t.role = "defendant" ∧
        t.objective = "For case 0, if post-judgment relief is sought, file Rule 60 motion with ground and timely filed_at date." ∧
        t.allowed_tools = ["file_rule60_motion"]) = true := by
  native_decide

theorem postJudgmentCandidates_offers_supersedeas_then_stay_then_lift :
    let c := judgmentCase
    let req := reqWithRoles c [{ role := "judge", allowed_tools := ["post_supersedeas_bond", "order_discretionary_stay", "lift_stay"] }]
    let a0 := postJudgmentCandidates req c ({ (default : TurnFacts) with hasSupersedeasBond := false }) 3
    let a1 := postJudgmentCandidates req c ({ (default : TurnFacts) with hasSupersedeasBond := true, hasDiscretionaryStay := false }) 3
    let a2 := postJudgmentCandidates req c ({ (default : TurnFacts) with hasSupersedeasBond := true, hasDiscretionaryStay := true, hasStayLift := false }) 3
    a0.any (fun t => t.allowed_tools = ["post_supersedeas_bond"]) = true ∧
      a1.any (fun t => t.allowed_tools = ["order_discretionary_stay"]) = true ∧
      a2.any (fun t => t.allowed_tools = ["lift_stay"]) = true := by
  native_decide
