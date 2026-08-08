import Main

def mkAdvanceTrialPhaseJudgeAction (phase : String) : CourtAction :=
  { action_type := "advance_trial_phase"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [("phase", Lean.Json.str phase)]
  }

def mkEnterJudgmentJudgeAction (claimId basis : String) : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [
      ("claim_id", Lean.Json.str claimId),
      ("basis", Lean.Json.str basis)
    ]
  }

theorem mkAdvanceTrialPhaseJudgeAction_action_type (phase : String) :
    (mkAdvanceTrialPhaseJudgeAction phase).action_type = "advance_trial_phase" := by
  simp [mkAdvanceTrialPhaseJudgeAction]

theorem mkAdvanceTrialPhaseJudgeAction_actor_role (phase : String) :
    (mkAdvanceTrialPhaseJudgeAction phase).actor_role = "judge" := by
  simp [mkAdvanceTrialPhaseJudgeAction]


theorem mkEnterJudgmentJudgeAction_action_type (claimId basis : String) :
    (mkEnterJudgmentJudgeAction claimId basis).action_type = "enter_judgment" := by
  simp [mkEnterJudgmentJudgeAction]

theorem mkEnterJudgmentJudgeAction_actor_role (claimId basis : String) :
    (mkEnterJudgmentJudgeAction claimId basis).actor_role = "judge" := by
  simp [mkEnterJudgmentJudgeAction]

theorem step_enter_judgment_propagates_validator_error
    (s : CourtState)
    (claimId basis : String)
    (hSchema : s.schema_version = "v1")
    (msg : String)
    (hValidate : validateEnterJudgment s.case = .error msg) :
    step s (mkEnterJudgmentJudgeAction claimId basis) = .error msg := by
  unfold step mkEnterJudgmentJudgeAction
  simp [hSchema, requireRole]
  rw [hValidate]
  rfl

theorem step_advance_trial_phase_propagates_validator_error
    (s : CourtState)
    (payload : Lean.Json)
    (decodedPhase : String)
    (hSchema : s.schema_version = "v1")
    (msg : String)
    (hGet : getString payload "phase" = .ok decodedPhase)
    (hValidate : validateAdvanceTrialPhase s.case decodedPhase = .error msg) :
    step s { action_type := "advance_trial_phase", actor_role := "judge", payload := payload } = .error msg := by
  unfold step
  simp [hSchema, requireRole]
  unfold runAdvanceTrialPhase
  rw [hGet]
  change applyAdvanceTrialPhase s decodedPhase = .error msg
  unfold applyAdvanceTrialPhase
  rw [hValidate]
  rfl
