import Main

theorem validateAdvanceTrialPhase_requires_jury_outcome_for_post_verdict
    (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "post_verdict" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hJury : c.trial_mode = "jury")
    (hNoVerdict : c.jury_verdict.isNone = true)
    (hNoHung : c.hung_jury.isNone = true) :
    validateAdvanceTrialPhase c "post_verdict" =
      .error "jury trial requires verdict or hung jury notice before post_verdict phase" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hJury, hNoVerdict, hNoHung, allowedPhases]

theorem validateAdvanceTrialPhase_requires_jury_instructions_before_deliberation
    (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "deliberation" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hJury : c.trial_mode = "jury")
    (hNoInstructions : hasDocketTitle c "Jury instructions delivered" = false) :
    validateAdvanceTrialPhase c "deliberation" =
      .error "jury trial requires delivered jury instructions before deliberation phase" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hJury, hNoInstructions, allowedPhases]

theorem validateAdvanceTrialPhase_deliberation_ok_when_instructions_delivered
    (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "deliberation" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hJury : c.trial_mode = "jury")
    (hInstructions : hasDocketTitle c "Jury instructions delivered" = true) :
    validateAdvanceTrialPhase c "deliberation" = .ok () := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hJury, hInstructions, allowedPhases]
