import ADC.Core

namespace ADCProofs.ValidateHungAndBench

def noSwornJurorCase : CaseState :=
  { (default : CaseState) with
    deliberation_round := 1
    jury_configuration := some {
      juror_count := 6
      unanimous_required := true
      minimum_concurring := 6
    }
  }

def pendingJurorVoteCase : CaseState :=
  { (default : CaseState) with
    deliberation_round := 1
    jury_configuration := some {
      juror_count := 1
      unanimous_required := true
      minimum_concurring := 1
    }
    jurors := [{ juror_id := "J1", name := "Juror One", status := "sworn" }]
  }

def splitJuryCase : CaseState :=
  { (default : CaseState) with
    deliberation_round := 1
    jury_configuration := some {
      juror_count := 2
      unanimous_required := true
      minimum_concurring := 2
    }
    jurors := [
      { juror_id := "J1", name := "Juror One", status := "sworn" },
      { juror_id := "J2", name := "Juror Two", status := "sworn" }
    ]
    juror_votes := [
      {
        juror_id := "J1"
        round := 1
        vote := "plaintiff"
        damages := 100.0
        confidence := "high"
        explanation := "Plaintiff proved the claim."
        submitted_at := "2026-01-01"
      },
      {
        juror_id := "J2"
        round := 1
        vote := "defendant"
        damages := 0.0
        confidence := "high"
        explanation := "Plaintiff did not prove the claim."
        submitted_at := "2026-01-01"
      }
    ]
  }

theorem deriveVerdict_without_configuration_returns_none
    (policy : CourtPolicy) (c : CaseState)
    (hConfiguration : c.jury_configuration = none) :
    deriveVerdictFromJurorVotes? policy c = none := by
  simp [deriveVerdictFromJurorVotes?, hConfiguration]

theorem deriveVerdict_without_sworn_jurors_returns_hung_jury :
    (match deriveVerdictFromJurorVotes? (default : CourtPolicy) noSwornJurorCase with
    | some (none, some _, none, none) => true
    | _ => false) = true := by
  native_decide

theorem deriveVerdict_waits_for_each_sworn_juror :
    deriveVerdictFromJurorVotes? (default : CourtPolicy) pendingJurorVoteCase = none := by
  native_decide

theorem applyDerivedDeliberationOutcome_stores_hung_jury
    (policy : CourtPolicy) (c : CaseState) (hung : HungJury)
    (hDerived :
      deriveVerdictFromJurorVotes? policy c = some (none, some hung, none, none)) :
    (applyDerivedDeliberationOutcome policy c).hung_jury = some hung := by
  simp [applyDerivedDeliberationOutcome, hDerived, appendDocket, appendDocketWithFields]

theorem applyDerivedDeliberationOutcome_stores_verdict
    (policy : CourtPolicy) (c : CaseState) (verdict : JuryVerdict)
    (hDerived :
      deriveVerdictFromJurorVotes? policy c = some (some verdict, none, none, none)) :
    (applyDerivedDeliberationOutcome policy c).jury_verdict = some verdict := by
  simp [applyDerivedDeliberationOutcome, hDerived, appendDocket, appendDocketWithFields]

theorem split_jury_advances_to_second_round :
    (applyDerivedDeliberationOutcome (default : CourtPolicy) splitJuryCase).deliberation_round = 2 := by
  native_decide

theorem split_jury_at_final_round_returns_hung_jury :
    let policy := { (default : CourtPolicy) with max_deliberation_rounds := 1 }
    (applyDerivedDeliberationOutcome policy splitJuryCase).hung_jury.isSome = true := by
  native_decide

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
    (hEmpty : text.trimAscii.isEmpty = true) :
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
    (hNonEmpty : text.trimAscii.isEmpty = false) :
    validateBenchOpinion c text = .ok () := by
  unfold validateBenchOpinion
  simp [hTrial, hPhase, hGate, hBench, hNonEmpty]

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

end ADCProofs.ValidateHungAndBench
