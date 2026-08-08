import Main

theorem canAdvance_none_to_openings :
    canAdvancePhaseV1 .none .openings = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_openings_to_plaintiffCase :
    canAdvancePhaseV1 .openings .plaintiffCase = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_plaintiffCase_to_defenseCase :
    canAdvancePhaseV1 .plaintiffCase .defenseCase = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_defenseCase_to_chargeConference :
    canAdvancePhaseV1 .defenseCase .chargeConference = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_chargeConference_to_closings :
    canAdvancePhaseV1 .chargeConference .closings = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_closings_to_juryCharge :
    canAdvancePhaseV1 .closings .juryCharge = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_closings_to_verdictReturn :
    canAdvancePhaseV1 .closings .verdictReturn = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_juryCharge_to_deliberation :
    canAdvancePhaseV1 .juryCharge .deliberation = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_deliberation_to_verdictReturn :
    canAdvancePhaseV1 .deliberation .verdictReturn = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem canAdvance_verdictReturn_to_postVerdict :
    canAdvancePhaseV1 .verdictReturn .postVerdict = true := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem cannot_advance_postVerdict_to_deliberation :
    canAdvancePhaseV1 .postVerdict .deliberation = false := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem cannot_advance_verdictReturn_to_plaintiffCase :
    canAdvancePhaseV1 .verdictReturn .plaintiffCase = false := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem cannot_advance_defenseCase_to_openings :
    canAdvancePhaseV1 .defenseCase .openings = false := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]

theorem cannot_advance_trial_to_voirDire :
    canAdvancePhaseV1 .openings .voirDire = false := by
  simp [canAdvancePhaseV1, trialPhaseRankV1]
