import ADC.Core

namespace ADCProofs.ValidateAdvancePhaseJury

theorem validateAdvanceTrialPhase_requires_jury_outcome_for_post_verdict
    (policy : CourtPolicy) (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "post_verdict" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hJury : c.trial_mode = "jury")
    (hNoVerdict : c.jury_verdict.isNone = true)
    (hNoHung : c.hung_jury.isNone = true) :
    validateAdvanceTrialPhase policy c "post_verdict" =
      .error "jury trial requires verdict or hung jury notice before post_verdict phase" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hJury, hNoVerdict, hNoHung, allowedPhases]

theorem validateAdvanceTrialPhase_requires_jury_instructions_before_deliberation
    (policy : CourtPolicy) (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "deliberation" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hJury : c.trial_mode = "jury")
    (hNoInstructions : hasDocketTitle c "Jury instructions delivered" = false) :
    validateAdvanceTrialPhase policy c "deliberation" =
      .error "jury trial requires delivered jury instructions before deliberation phase" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hJury, hNoInstructions, allowedPhases]

theorem validateAdvanceTrialPhase_deliberation_ok_when_instructions_delivered
    (policy : CourtPolicy) (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "deliberation" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hJury : c.trial_mode = "jury")
    (hInstructions : hasDocketTitle c "Jury instructions delivered" = true) :
    validateAdvanceTrialPhase policy c "deliberation" = .ok () := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hJury, hInstructions, allowedPhases]

end ADCProofs.ValidateAdvancePhaseJury
