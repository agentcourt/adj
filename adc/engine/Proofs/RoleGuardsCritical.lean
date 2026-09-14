import ADC.Core

namespace ADCProofs.RoleGuardsCritical

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "trial",
    phase := "post_verdict"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def enterJudgmentActionAs (role : String) : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "jury verdict")
      ]
  }

def submitJurorVoteActionAs (role : String) : CourtAction :=
  { action_type := "submit_juror_vote"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str "J1")
      , ("vote", Lean.Json.str "plaintiff")
      , ("damages", Lean.Json.num 100)
      , ("confidence", Lean.Json.str "high")
      , ("explanation", Lean.Json.str "The evidence supports the verdict.")
      ]
  }

def empanelJuryActionAs (role : String) : CourtAction :=
  { action_type := "empanel_jury"
  , actor_role := role
  , payload := Lean.Json.mkObj []
  }

def resolveRule59ActionAs (role : String) : CourtAction :=
  { action_type := "resolve_rule59_motion"
  , actor_role := role
  , payload := Lean.Json.mkObj
      [ ("motion_index", Lean.Json.num 0)
      , ("granted", Lean.Json.bool false)
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_enter_judgment_rejects_non_judge_role :
    stepErrorMessage (step (stateOf) (enterJudgmentActionAs "plaintiff")) =
      "role plaintiff not permitted for enter_judgment" := by
  native_decide

theorem step_submit_juror_vote_rejects_non_juror_role :
    stepErrorMessage (step (stateOf) (submitJurorVoteActionAs "judge")) =
      "role judge not permitted for submit_juror_vote" := by
  native_decide

theorem step_empanel_jury_rejects_non_judge_role :
    stepErrorMessage (step (stateOf) (empanelJuryActionAs "defendant")) =
      "role defendant not permitted for empanel_jury" := by
  native_decide

theorem step_resolve_rule59_rejects_non_judge_role :
    let c := { baseCase with status := "judgment_entered" }
    stepErrorMessage (step (stateOf c) (resolveRule59ActionAs "defendant")) =
      "role defendant not permitted for resolve_rule59_motion" := by
  native_decide

end ADCProofs.RoleGuardsCritical
