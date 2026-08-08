import Main

theorem validateAdvanceTrialPhase_requires_trial_status
    (c : CaseState) (phase : String)
    (hNotTrial : c.status ≠ "trial") :
    validateAdvanceTrialPhase c phase = .error "trial phase advancement requires trial status" := by
  unfold validateAdvanceTrialPhase
  simp [hNotTrial]

theorem validateAdvanceTrialPhase_invalid_phase
    (c : CaseState) (phase : String)
    (hTrial : c.status = "trial")
    (hNotMem : phase ∉ allowedPhases) :
    validateAdvanceTrialPhase c phase = .error s!"invalid phase: {phase}" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hNotMem]

theorem validateAdvanceTrialPhase_invalid_current_phase
    (c : CaseState) (phase : String)
    (hTrial : c.status = "trial")
    (hMem : phase ∈ allowedPhases)
    (hCurrent : parseTrialPhaseV1 c.phase = none) :
    validateAdvanceTrialPhase c phase = .error s!"invalid current phase: {c.phase}" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hMem, hCurrent]

theorem validateAdvanceTrialPhase_backward_transition
    (c : CaseState) (phase : String)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hMem : phase ∈ allowedPhases)
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 phase = some nextPhase)
    (hBackward : canAdvancePhaseV1 currentPhase nextPhase = false) :
    validateAdvanceTrialPhase c phase = .error s!"cannot move backward from phase {c.phase} to {phase}" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hMem, hCurrent, hNext, hBackward]

theorem validateAdvanceTrialPhase_requires_bench_opinion_for_post_verdict
    (c : CaseState)
    (currentPhase nextPhase : TrialPhaseV1)
    (hTrial : c.status = "trial")
    (hCurrent : parseTrialPhaseV1 c.phase = some currentPhase)
    (hNext : parseTrialPhaseV1 "post_verdict" = some nextPhase)
    (hAdvance : canAdvancePhaseV1 currentPhase nextPhase = true)
    (hBench : c.trial_mode = "bench")
    (hNoOpinion : hasDocketTitle c "Bench Opinion" = false) :
    validateAdvanceTrialPhase c "post_verdict" = .error "bench trial requires Bench Opinion before post_verdict phase" := by
  unfold validateAdvanceTrialPhase
  simp [hTrial, hCurrent, hNext, hAdvance, hBench, hNoOpinion, allowedPhases]

theorem validateAdvanceTrialPhase_ok_implies_trial_status
    (c : CaseState) (phase : String)
    (hOk : validateAdvanceTrialPhase c phase = .ok ()) :
    c.status = "trial" := by
  unfold validateAdvanceTrialPhase at hOk
  by_cases hTrial : c.status = "trial"
  · exact hTrial
  · simp [hTrial] at hOk

theorem validateAdvanceTrialPhase_ok_implies_phase_allowed
    (c : CaseState) (phase : String)
    (hOk : validateAdvanceTrialPhase c phase = .ok ()) :
    phase ∈ allowedPhases := by
  unfold validateAdvanceTrialPhase at hOk
  by_cases hTrial : c.status = "trial"
  · by_cases hMem : phase ∈ allowedPhases
    · exact hMem
    · simp [hTrial, hMem] at hOk
  · simp [hTrial] at hOk

theorem validateAdvanceTrialPhase_ok_implies_forward_parse_and_gate
    (c : CaseState) (phase : String)
    (hOk : validateAdvanceTrialPhase c phase = .ok ()) :
    ∃ currentPhase nextPhase : TrialPhaseV1,
      parseTrialPhaseV1 c.phase = some currentPhase ∧
      parseTrialPhaseV1 phase = some nextPhase ∧
      canAdvancePhaseV1 currentPhase nextPhase = true := by
  have hTrial : c.status = "trial" :=
    validateAdvanceTrialPhase_ok_implies_trial_status c phase hOk
  have hAllowed : phase ∈ allowedPhases :=
    validateAdvanceTrialPhase_ok_implies_phase_allowed c phase hOk
  unfold validateAdvanceTrialPhase at hOk
  cases hCurrent : parseTrialPhaseV1 c.phase with
  | none =>
      simp [hTrial, hAllowed, hCurrent] at hOk
  | some currentPhase =>
      cases hNext : parseTrialPhaseV1 phase with
      | none =>
          simp [hTrial, hAllowed, hCurrent, hNext] at hOk
      | some nextPhase =>
          by_cases hAdvance : canAdvancePhaseV1 currentPhase nextPhase = false
          · simp [hTrial, hAllowed, hCurrent, hNext, hAdvance] at hOk
          · have hAdvanceTrue : canAdvancePhaseV1 currentPhase nextPhase = true := by
              cases hAdvBool : canAdvancePhaseV1 currentPhase nextPhase with
              | false =>
                  exact (hAdvance hAdvBool).elim
              | true =>
                  rfl
            exact ⟨currentPhase, nextPhase, rfl, rfl, hAdvanceTrue⟩
