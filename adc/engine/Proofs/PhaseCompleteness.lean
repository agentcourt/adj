import ADC.Core

namespace ADCProofs.PhaseCompleteness

theorem phaseAllowsActionV1_openings_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .openings = true ↔
      action = .recordOpeningStatement := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_plaintiffCase_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .plaintiffCase = true ↔
      action = .submitPresentation := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_plaintiffEvidence_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .plaintiffEvidence = true ↔
      action = .offerExhibit ∨ action = .restCase := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_defenseCase_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .defenseCase = true ↔
      action = .submitPresentation := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_defenseEvidence_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .defenseEvidence = true ↔
      action = .offerExhibit ∨ action = .restCase := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_plaintiffRebuttal_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .plaintiffRebuttal = true ↔
      action = .submitPresentation := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_plaintiffRebuttalEvidence_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .plaintiffRebuttalEvidence = true ↔
      action = .offerExhibit ∨ action = .restCase := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_defenseSurrebuttal_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .defenseSurrebuttal = true ↔
      action = .submitPresentation := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_defenseSurrebuttalEvidence_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .defenseSurrebuttalEvidence = true ↔
      action = .offerExhibit ∨ action = .restCase := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_closings_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .closings = true ↔
      action = .deliverClosingArgument := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_deliberation_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .deliberation = true ↔
      action = .submitJurorVote := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_verdictReturn_false
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .verdictReturn = false := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_postVerdict_false
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .postVerdict = false := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_none_false
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .none = false := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_voirDire_true_iff
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .voirDire = true ↔
      action = .issueJurorQuestionnaire ∨
      action = .answerJurorQuestionnaire ∨
      action = .decideVoirDireQuestion ∨
      action = .answerVoirDireQuestion ∨
      action = .decideJurorForCauseChallenge ∨
      action = .empanelJury := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_chargeConference_false
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .chargeConference = false := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_juryCharge_false
    (action : TrialActionV1) :
    phaseAllowsActionV1 action .juryCharge = false := by
  cases action <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_true_implies_phase_not_none
    (action : TrialActionV1) (phase : TrialPhaseV1) :
    phaseAllowsActionV1 action phase = true -> phase ≠ .none := by
  intro h
  cases phase <;> simp [phaseAllowsActionV1] at h <;> simp

end ADCProofs.PhaseCompleteness
