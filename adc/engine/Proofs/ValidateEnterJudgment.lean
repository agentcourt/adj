import Main

theorem validateEnterJudgment_requires_trial_status
    (c : CaseState)
    (hNotTrial : c.status ≠ "trial") :
    validateEnterJudgment c = .error "judgment entry requires trial status" := by
  unfold validateEnterJudgment
  simp [hNotTrial]

theorem validateEnterJudgment_bench_hung_error
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hBench : c.trial_mode = "bench")
    (hHung : c.hung_jury.isSome = true) :
    validateEnterJudgment c = .error "cannot enter judgment after hung jury" := by
  unfold validateEnterJudgment
  simp [hTrial, hBench, hHung]

theorem validateEnterJudgment_bench_requires_opinion
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hBench : c.trial_mode = "bench")
    (hNoHung : c.hung_jury.isSome = false)
    (hNoOpinion : hasDocketTitle c "Bench Opinion" = false) :
    validateEnterJudgment c = .error "bench trial requires Bench Opinion before judgment" := by
  unfold validateEnterJudgment
  simp [hTrial, hBench, hNoHung, hNoOpinion]

theorem validateEnterJudgment_bench_ok
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hBench : c.trial_mode = "bench")
    (hNoHung : c.hung_jury.isSome = false)
    (hOpinion : hasDocketTitle c "Bench Opinion" = true) :
    validateEnterJudgment c = .ok () := by
  unfold validateEnterJudgment
  simp [hTrial, hBench, hNoHung, hOpinion]

theorem validateEnterJudgment_jury_disposition_hung_error
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hNotBench : c.trial_mode ≠ "bench")
    (hDisp : claimDispositionFromCaseStateV1 c = .ok ClaimDispositionV1.hung) :
    validateEnterJudgment c = .error "cannot enter judgment after hung jury" := by
  unfold validateEnterJudgment
  simp [hTrial, hNotBench, hDisp, canEnterJudgmentFromClaimDispositionV1]

theorem validateEnterJudgment_jury_disposition_pending_error
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hNotBench : c.trial_mode ≠ "bench")
    (hDisp : claimDispositionFromCaseStateV1 c = .ok ClaimDispositionV1.pending) :
    validateEnterJudgment c = .error "jury verdict required before judgment" := by
  unfold validateEnterJudgment
  simp [hTrial, hNotBench, hDisp, canEnterJudgmentFromClaimDispositionV1]

theorem validateEnterJudgment_jury_disposition_verdict_plaintiff_ok
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hNotBench : c.trial_mode ≠ "bench")
    (hDisp : claimDispositionFromCaseStateV1 c = .ok ClaimDispositionV1.verdictPlaintiff) :
    validateEnterJudgment c = .ok () := by
  unfold validateEnterJudgment
  simp [hTrial, hNotBench, hDisp, canEnterJudgmentFromClaimDispositionV1]

theorem validateEnterJudgment_jury_disposition_verdict_defendant_ok
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hNotBench : c.trial_mode ≠ "bench")
    (hDisp : claimDispositionFromCaseStateV1 c = .ok ClaimDispositionV1.verdictDefendant) :
    validateEnterJudgment c = .ok () := by
  unfold validateEnterJudgment
  simp [hTrial, hNotBench, hDisp, canEnterJudgmentFromClaimDispositionV1]

theorem validateEnterJudgment_ok_implies_trial_status
    (c : CaseState)
    (hOk : validateEnterJudgment c = .ok ()) :
    c.status = "trial" := by
  unfold validateEnterJudgment at hOk
  by_cases hTrial : c.status = "trial"
  · exact hTrial
  · simp [hTrial] at hOk

theorem validateEnterJudgment_bench_ok_implies_no_hung
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hBench : c.trial_mode = "bench")
    (hOk : validateEnterJudgment c = .ok ()) :
    c.hung_jury.isSome = false := by
  unfold validateEnterJudgment at hOk
  by_cases hHung : c.hung_jury.isSome
  · simp [hTrial, hBench, hHung] at hOk
  · simp [hHung]

theorem validateEnterJudgment_bench_ok_implies_bench_opinion
    (c : CaseState)
    (hTrial : c.status = "trial")
    (hBench : c.trial_mode = "bench")
    (hOk : validateEnterJudgment c = .ok ()) :
    hasDocketTitle c "Bench Opinion" = true := by
  unfold validateEnterJudgment at hOk
  by_cases hHung : c.hung_jury.isSome
  · simp [hTrial, hBench, hHung] at hOk
  · by_cases hOpinion : hasDocketTitle c "Bench Opinion"
    · simp [hOpinion]
    · simp [hTrial, hBench, hHung, hOpinion] at hOk

theorem validateEnterJudgment_ok_implies_no_hung
    (c : CaseState)
    (hOk : validateEnterJudgment c = .ok ()) :
    c.hung_jury.isSome = false := by
  have hTrial : c.status = "trial" := validateEnterJudgment_ok_implies_trial_status c hOk
  by_cases hBench : c.trial_mode = "bench"
  · exact validateEnterJudgment_bench_ok_implies_no_hung c hTrial hBench hOk
  · by_cases hHung : c.hung_jury.isSome
    · exfalso
      have hDisp : claimDispositionFromCaseStateV1 c = .ok ClaimDispositionV1.hung := by
        unfold claimDispositionFromCaseStateV1
        simp [hHung]
        rfl
      have hErr :
          validateEnterJudgment c = .error "cannot enter judgment after hung jury" :=
        validateEnterJudgment_jury_disposition_hung_error c hTrial hBench hDisp
      cases hOk.symm.trans hErr
    · simp [hHung]

theorem validateEnterJudgment_ok_implies_jury_disposition_allows
    (c : CaseState)
    (hOk : validateEnterJudgment c = .ok ())
    (hNotBench : c.trial_mode ≠ "bench") :
    ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = .ok d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true := by
  have hTrial : c.status = "trial" :=
    validateEnterJudgment_ok_implies_trial_status c hOk
  by_cases hBench : c.trial_mode = "bench"
  · exact (hNotBench hBench).elim
  · unfold validateEnterJudgment at hOk
    simp [hTrial, hBench] at hOk
    cases hDisp : claimDispositionFromCaseStateV1 c with
    | error e =>
        simp [hDisp] at hOk
    | ok d =>
        by_cases hAllow : canEnterJudgmentFromClaimDispositionV1 d = true
        · exact ⟨d, rfl, hAllow⟩
        · have hAllowFalse : canEnterJudgmentFromClaimDispositionV1 d = false :=
            Bool.eq_false_iff.mpr hAllow
          have hErrEq :
              (if d = ClaimDispositionV1.hung then
                (Except.error "cannot enter judgment after hung jury" : Except String Unit)
               else
                (Except.error "jury verdict required before judgment" : Except String Unit)) = .ok () := by
            simpa [hDisp, hAllowFalse] using hOk
          by_cases hdHung : d = ClaimDispositionV1.hung
          · simp [hdHung] at hErrEq
          · simp [hdHung] at hErrEq

theorem validateEnterJudgment_ok_implies_jury_disposition_is_verdict
    (c : CaseState)
    (hOk : validateEnterJudgment c = .ok ())
    (hNotBench : c.trial_mode ≠ "bench") :
    ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = .ok d ∧
      (d = ClaimDispositionV1.verdictPlaintiff ∨ d = ClaimDispositionV1.verdictDefendant) := by
  rcases
      validateEnterJudgment_ok_implies_jury_disposition_allows
        c hOk hNotBench
    with ⟨d, hDisp, hAllow⟩
  cases d with
  | pending =>
      simp [canEnterJudgmentFromClaimDispositionV1] at hAllow
  | hung =>
      simp [canEnterJudgmentFromClaimDispositionV1] at hAllow
  | verdictPlaintiff =>
      exact ⟨ClaimDispositionV1.verdictPlaintiff, hDisp, Or.inl rfl⟩
  | verdictDefendant =>
      exact ⟨ClaimDispositionV1.verdictDefendant, hDisp, Or.inr rfl⟩
  | judgmentEntered =>
      simp [canEnterJudgmentFromClaimDispositionV1] at hAllow

theorem validateEnterJudgment_ok_jury_implies_not_pending_or_hung
    (c : CaseState)
    (hOk : validateEnterJudgment c = .ok ())
    (hNotBench : c.trial_mode ≠ "bench") :
    ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = .ok d ∧
      d ≠ ClaimDispositionV1.pending ∧
      d ≠ ClaimDispositionV1.hung := by
  rcases
      validateEnterJudgment_ok_implies_jury_disposition_is_verdict
        c hOk hNotBench
    with ⟨d, hDisp, hVerdict⟩
  refine ⟨d, hDisp, ?_, ?_⟩
  · intro hPending
    rcases hVerdict with hPl | hDef
    · cases hPl
      cases hPending
    · cases hDef
      cases hPending
  · intro hHung
    rcases hVerdict with hPl | hDef
    · cases hPl
      cases hHung
    · cases hDef
      cases hHung
