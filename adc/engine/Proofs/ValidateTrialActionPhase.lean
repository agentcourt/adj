import Main

theorem validateTrialActionPhase_invalid_current_phase
    (c : CaseState)
    (action : TrialActionV1)
    (message : String)
    (err : String)
    (hPhase : parseCurrentPhaseV1 c = .error err) :
    validateTrialActionPhase c action message = .error err := by
  unfold validateTrialActionPhase
  simp [hPhase]

theorem validateTrialActionPhase_gate_error
    (c : CaseState)
    (action : TrialActionV1)
    (message : String)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 action currentPhase = false) :
    validateTrialActionPhase c action message = .error message := by
  unfold validateTrialActionPhase
  simp [hPhase, hGate]

theorem validateTrialActionPhase_ok
    (c : CaseState)
    (action : TrialActionV1)
    (message : String)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 action currentPhase = true) :
    validateTrialActionPhase c action message = .ok () := by
  unfold validateTrialActionPhase
  simp [hPhase, hGate]

theorem validateTrialActionPhase_ok_implies_phase_parse_success
    (c : CaseState)
    (action : TrialActionV1)
    (message : String)
    (hOk : validateTrialActionPhase c action message = .ok ()) :
    ∃ currentPhase : TrialPhaseV1, parseCurrentPhaseV1 c = .ok currentPhase := by
  unfold validateTrialActionPhase at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error err =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      exact ⟨currentPhase, rfl⟩

theorem validateTrialActionPhase_ok_implies_gate_true
    (c : CaseState)
    (action : TrialActionV1)
    (message : String)
    (hOk : validateTrialActionPhase c action message = .ok ()) :
    ∃ currentPhase : TrialPhaseV1,
      parseCurrentPhaseV1 c = .ok currentPhase ∧
      phaseAllowsActionV1 action currentPhase = true := by
  rcases validateTrialActionPhase_ok_implies_phase_parse_success c action message hOk with
    ⟨currentPhase, hPhase⟩
  unfold validateTrialActionPhase at hOk
  by_cases hGate : phaseAllowsActionV1 action currentPhase = false
  · simp [hPhase, hGate] at hOk
  · have hGateTrue : phaseAllowsActionV1 action currentPhase = true := by
      cases hGateBool : phaseAllowsActionV1 action currentPhase with
      | false =>
          exact (hGate hGateBool).elim
      | true =>
          rfl
    exact ⟨currentPhase, hPhase, hGateTrue⟩
