import Main

theorem validateDeclareHungJury_invalid_current_phase
    (c : CaseState)
    (msg : String)
    (hPhase : parseCurrentPhaseV1 c = .error msg) :
    validateDeclareHungJury c = .error msg := by
  unfold validateDeclareHungJury
  simp [hPhase]

theorem validateDeclareHungJury_phase_gate_error
    (c : CaseState)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .declareHungJury currentPhase = false) :
    validateDeclareHungJury c =
      .error s!"hung jury declaration requires deliberation or verdict_return phase; current phase is {c.phase}" := by
  unfold validateDeclareHungJury
  simp [hPhase, hGate]

theorem validateDeclareHungJury_verdict_already_returned
    (c : CaseState)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .declareHungJury currentPhase = true)
    (hVerdict : c.jury_verdict.isSome = true) :
    validateDeclareHungJury c = .error "cannot declare hung jury after verdict is returned" := by
  unfold validateDeclareHungJury
  simp [hPhase, hGate, hVerdict]

theorem validateDeclareHungJury_ok
    (c : CaseState)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .declareHungJury currentPhase = true)
    (hNoVerdict : c.jury_verdict.isSome = false) :
    validateDeclareHungJury c = .ok () := by
  unfold validateDeclareHungJury
  simp [hPhase, hGate, hNoVerdict]

theorem validateBenchOpinion_requires_trial_status
    (c : CaseState)
    (text : String)
    (hNotTrial : c.status ≠ "trial") :
    validateBenchOpinion c text = .error "bench opinion requires trial status" := by
  unfold validateBenchOpinion
  simp [hNotTrial]

theorem validateBenchOpinion_invalid_current_phase
    (c : CaseState)
    (text : String)
    (hTrial : c.status = "trial")
    (msg : String)
    (hPhase : parseCurrentPhaseV1 c = .error msg) :
    validateBenchOpinion c text = .error msg := by
  unfold validateBenchOpinion
  simp [hTrial, hPhase]

theorem validateBenchOpinion_phase_gate_error
    (c : CaseState)
    (text : String)
    (hTrial : c.status = "trial")
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : !(currentPhase = TrialPhaseV1.verdictReturn || currentPhase = TrialPhaseV1.postVerdict)) :
    validateBenchOpinion c text =
      .error s!"bench opinion requires verdict_return or post_verdict phase; current phase is {c.phase}" := by
  unfold validateBenchOpinion
  simp [hTrial, hPhase, hGate]

theorem validateBenchOpinion_requires_bench_mode
    (c : CaseState)
    (text : String)
    (hTrial : c.status = "trial")
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : currentPhase = TrialPhaseV1.verdictReturn || currentPhase = TrialPhaseV1.postVerdict)
    (hNotBench : c.trial_mode ≠ "bench") :
    validateBenchOpinion c text = .error "bench opinion is only available in bench trials" := by
  unfold validateBenchOpinion
  simp [hTrial, hPhase, hGate, hNotBench]

theorem validateBenchOpinion_requires_nonempty_text
    (c : CaseState)
    (text : String)
    (hTrial : c.status = "trial")
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : currentPhase = TrialPhaseV1.verdictReturn || currentPhase = TrialPhaseV1.postVerdict)
    (hBench : c.trial_mode = "bench")
    (hEmpty : text.trimAscii.toString = "") :
    validateBenchOpinion c text = .error "bench opinion text must be non-empty" := by
  unfold validateBenchOpinion
  simp [hTrial, hPhase, hGate, hBench, hEmpty]

theorem validateBenchOpinion_ok
    (c : CaseState)
    (text : String)
    (hTrial : c.status = "trial")
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : currentPhase = TrialPhaseV1.verdictReturn || currentPhase = TrialPhaseV1.postVerdict)
    (hBench : c.trial_mode = "bench")
    (hNonEmpty : text.trimAscii.toString ≠ "") :
    validateBenchOpinion c text = .ok () := by
  unfold validateBenchOpinion
  simp [hTrial, hPhase, hGate, hBench, hNonEmpty]

theorem validateDeclareHungJury_ok_implies_no_verdict
    (c : CaseState)
    (hOk : validateDeclareHungJury c = .ok ()) :
    c.jury_verdict.isSome = false := by
  unfold validateDeclareHungJury at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      by_cases hGate : phaseAllowsActionV1 .declareHungJury currentPhase = false
      · simp [hPhase, hGate] at hOk
      · by_cases hVerdict : c.jury_verdict.isSome
        · simp [hPhase, hGate, hVerdict] at hOk
        · simp [hVerdict]

theorem validateDeclareHungJury_ok_implies_phase_gate_true
    (c : CaseState)
    (hOk : validateDeclareHungJury c = .ok ()) :
    ∃ currentPhase : TrialPhaseV1,
      parseCurrentPhaseV1 c = .ok currentPhase ∧
      phaseAllowsActionV1 .declareHungJury currentPhase = true := by
  unfold validateDeclareHungJury at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      by_cases hGate : phaseAllowsActionV1 .declareHungJury currentPhase = false
      · simp [hPhase, hGate] at hOk
      · have hGateTrue : phaseAllowsActionV1 .declareHungJury currentPhase = true := by
          cases hGateBool : phaseAllowsActionV1 .declareHungJury currentPhase with
          | false =>
              exact (hGate hGateBool).elim
          | true =>
              rfl
        exact ⟨currentPhase, rfl, hGateTrue⟩

theorem validateDeclareHungJury_ok_implies_phase_is_delib_or_verdict_return
    (c : CaseState)
    (hOk : validateDeclareHungJury c = .ok ()) :
    ∃ currentPhase : TrialPhaseV1,
      parseCurrentPhaseV1 c = .ok currentPhase ∧
      (currentPhase = TrialPhaseV1.deliberation ∨ currentPhase = TrialPhaseV1.verdictReturn) := by
  rcases validateDeclareHungJury_ok_implies_phase_gate_true c hOk with
    ⟨currentPhase, hPhase, hGate⟩
  cases currentPhase <;> simp [phaseAllowsActionV1] at hGate
  · exact ⟨TrialPhaseV1.deliberation, hPhase, Or.inl rfl⟩
  · exact ⟨TrialPhaseV1.verdictReturn, hPhase, Or.inr rfl⟩

theorem validateBenchOpinion_ok_implies_trial_status
    (c : CaseState) (text : String)
    (hOk : validateBenchOpinion c text = .ok ()) :
    c.status = "trial" := by
  unfold validateBenchOpinion at hOk
  by_cases hTrial : c.status = "trial"
  · exact hTrial
  · simp [hTrial] at hOk

theorem validateBenchOpinion_ok_implies_bench_mode
    (c : CaseState) (text : String)
    (hOk : validateBenchOpinion c text = .ok ()) :
    c.trial_mode = "bench" := by
  unfold validateBenchOpinion at hOk
  by_cases hTrial : c.status = "trial"
  · cases hPhase : parseCurrentPhaseV1 c with
    | error e =>
        simp [hTrial, hPhase] at hOk
    | ok currentPhase =>
        by_cases hGate : !(currentPhase = TrialPhaseV1.verdictReturn || currentPhase = TrialPhaseV1.postVerdict)
        · simp [hTrial, hPhase, hGate] at hOk
        · by_cases hBench : c.trial_mode = "bench"
          · exact hBench
          · simp [hTrial, hPhase, hGate, hBench] at hOk
  · simp [hTrial] at hOk
