import Main

theorem parseCaseStatusV1_roundtrip_filed :
    parseCaseStatusV1 "filed" = some .filed := by
  simp [parseCaseStatusV1]

theorem parseCaseStatusV1_roundtrip_pretrial :
    parseCaseStatusV1 "pretrial" = some .pretrial := by
  simp [parseCaseStatusV1]

theorem parseCaseStatusV1_roundtrip_trial :
    parseCaseStatusV1 "trial" = some .trial := by
  simp [parseCaseStatusV1]

theorem parseCaseStatusV1_roundtrip_judgment_entered :
    parseCaseStatusV1 "judgment_entered" = some .judgmentEntered := by
  simp [parseCaseStatusV1]

theorem parseCaseStatusV1_roundtrip_closed :
    parseCaseStatusV1 "closed" = some .closed := by
  simp [parseCaseStatusV1]

theorem parseCaseStatusV1_toString_roundtrip (s : CaseStatusV1) :
    parseCaseStatusV1 (caseStatusV1ToString s) = some s := by
  cases s <;> simp [caseStatusV1ToString, parseCaseStatusV1]

theorem allowedStatus_contains_implies_parseCaseStatusV1_some
    (status : String) :
    allowedStatuses.contains status = true ->
      ∃ s : CaseStatusV1, parseCaseStatusV1 status = some s := by
  intro h
  have hstatus :
      status = "filed" ∨
      status = "pretrial" ∨
      status = "trial" ∨
      status = "judgment_entered" ∨
      status = "closed" := by
    simpa [allowedStatuses] using h
  rcases hstatus with hf | hp | ht | hj | hc
  · subst hf; exact ⟨.filed, by simp [parseCaseStatusV1]⟩
  · subst hp; exact ⟨.pretrial, by simp [parseCaseStatusV1]⟩
  · subst ht; exact ⟨.trial, by simp [parseCaseStatusV1]⟩
  · subst hj; exact ⟨.judgmentEntered, by simp [parseCaseStatusV1]⟩
  · subst hc; exact ⟨.closed, by simp [parseCaseStatusV1]⟩

theorem allowedPhases_contains_implies_parseTrialPhaseV1_some
    (phase : String) :
    allowedPhases.contains phase = true ->
      ∃ p : TrialPhaseV1, parseTrialPhaseV1 phase = some p := by
  intro h
  have hphase :
      phase = "none" ∨
      phase = "voir_dire" ∨
      phase = "openings" ∨
      phase = "plaintiff_case" ∨
      phase = "defense_case" ∨
      phase = "plaintiff_rebuttal" ∨
      phase = "defense_surrebuttal" ∨
      phase = "charge_conference" ∨
      phase = "closings" ∨
      phase = "jury_charge" ∨
      phase = "deliberation" ∨
      phase = "verdict_return" ∨
      phase = "post_verdict" := by
    simpa [allowedPhases] using h
  rcases hphase with hnone | hvd | hopen | hpl | hdef | hpr | hds | hcc | hcl | hjc | hdel | hvret | hpost
  · subst hnone; exact ⟨.none, by simp [parseTrialPhaseV1]⟩
  · subst hvd; exact ⟨.voirDire, by simp [parseTrialPhaseV1]⟩
  · subst hopen; exact ⟨.openings, by simp [parseTrialPhaseV1]⟩
  · subst hpl; exact ⟨.plaintiffCase, by simp [parseTrialPhaseV1]⟩
  · subst hdef; exact ⟨.defenseCase, by simp [parseTrialPhaseV1]⟩
  · subst hpr; exact ⟨.plaintiffRebuttal, by simp [parseTrialPhaseV1]⟩
  · subst hds; exact ⟨.defenseSurrebuttal, by simp [parseTrialPhaseV1]⟩
  · subst hcc; exact ⟨.chargeConference, by simp [parseTrialPhaseV1]⟩
  · subst hcl; exact ⟨.closings, by simp [parseTrialPhaseV1]⟩
  · subst hjc; exact ⟨.juryCharge, by simp [parseTrialPhaseV1]⟩
  · subst hdel; exact ⟨.deliberation, by simp [parseTrialPhaseV1]⟩
  · subst hvret; exact ⟨.verdictReturn, by simp [parseTrialPhaseV1]⟩
  · subst hpost; exact ⟨.postVerdict, by simp [parseTrialPhaseV1]⟩

theorem checkTransition_matches_typed_on_enums (current next : CaseStatusV1) :
    checkTransition (caseStatusV1ToString current) (caseStatusV1ToString next) =
      canTransitionStatusV1 current next := by
  cases current <;> cases next <;>
    simp [checkTransition, caseStatusV1ToString, canTransitionStatusV1]

theorem canTransitionStatusV1_closed_false (next : CaseStatusV1) :
    canTransitionStatusV1 .closed next = false := by
  cases next <;> simp [canTransitionStatusV1]

theorem canTransitionStatusV1_judgment_entered_eq_closed (next : CaseStatusV1) :
    canTransitionStatusV1 .judgmentEntered next = (next = .closed) := by
  cases next <;> simp [canTransitionStatusV1]

theorem canTransitionStatusV1_irrefl (s : CaseStatusV1) :
    canTransitionStatusV1 s s = false := by
  cases s <;> simp [canTransitionStatusV1]

theorem canTransitionStatusV1_true_implies_ne (current next : CaseStatusV1) :
    canTransitionStatusV1 current next = true -> current ≠ next := by
  intro h
  intro heq
  rw [heq] at h
  simp [canTransitionStatusV1] at h

theorem canTransitionStatusV1_to_closed_iff_not_closed (current : CaseStatusV1) :
    canTransitionStatusV1 current .closed = (current ≠ .closed) := by
  cases current <;> simp [canTransitionStatusV1]

theorem canTransitionStatusV1_true_implies_next_not_filed
    (current next : CaseStatusV1) :
    canTransitionStatusV1 current next = true -> next ≠ .filed := by
  intro h
  intro hn
  rw [hn] at h
  cases current <;> simp [canTransitionStatusV1] at h

theorem canTransitionStatusV1_true_implies_current_not_closed
    (current next : CaseStatusV1) :
    canTransitionStatusV1 current next = true -> current ≠ .closed := by
  intro h
  intro hc
  rw [hc] at h
  cases next <;> simp [canTransitionStatusV1] at h

theorem canTransitionStatusV1_filed_true_iff (next : CaseStatusV1) :
    canTransitionStatusV1 .filed next = true ↔
      next = .pretrial ∨ next = .closed := by
  cases next <;> simp [canTransitionStatusV1]

theorem canTransitionStatusV1_pretrial_true_iff (next : CaseStatusV1) :
    canTransitionStatusV1 .pretrial next = true ↔
      next = .trial ∨ next = .closed := by
  cases next <;> simp [canTransitionStatusV1]

theorem canTransitionStatusV1_trial_true_iff (next : CaseStatusV1) :
    canTransitionStatusV1 .trial next = true ↔
      next = .judgmentEntered ∨ next = .closed := by
  cases next <;> simp [canTransitionStatusV1]

theorem canAdvancePhaseV1_refl (phase : TrialPhaseV1) :
    canAdvancePhaseV1 phase phase = true := by
  simp [canAdvancePhaseV1]

theorem canAdvancePhaseV1_true_iff_rank_le (a b : TrialPhaseV1) :
    canAdvancePhaseV1 a b = true ↔
      trialPhaseRankV1 a ≤ trialPhaseRankV1 b := by
  simp [canAdvancePhaseV1]

theorem canAdvancePhaseV1_trans (a b c : TrialPhaseV1)
    (hab : canAdvancePhaseV1 a b = true)
    (hbc : canAdvancePhaseV1 b c = true) :
    canAdvancePhaseV1 a c = true := by
  have hab' : trialPhaseRankV1 a ≤ trialPhaseRankV1 b := by
    simpa [canAdvancePhaseV1] using hab
  have hbc' : trialPhaseRankV1 b ≤ trialPhaseRankV1 c := by
    simpa [canAdvancePhaseV1] using hbc
  have hac' : trialPhaseRankV1 a ≤ trialPhaseRankV1 c := Nat.le_trans hab' hbc'
  simpa [canAdvancePhaseV1] using hac'

theorem canAdvancePhaseV1_antisymm_true (a b : TrialPhaseV1)
    (hab : canAdvancePhaseV1 a b = true)
    (hba : canAdvancePhaseV1 b a = true) :
    a = b := by
  have hab' : trialPhaseRankV1 a ≤ trialPhaseRankV1 b := by
    simpa [canAdvancePhaseV1] using hab
  have hba' : trialPhaseRankV1 b ≤ trialPhaseRankV1 a := by
    simpa [canAdvancePhaseV1] using hba
  have hrank : trialPhaseRankV1 a = trialPhaseRankV1 b := Nat.le_antisymm hab' hba'
  cases a <;> cases b <;> simp [trialPhaseRankV1] at hrank <;> cases hrank <;> rfl

theorem canAdvancePhaseV1_true_and_ne_implies_rank_lt (a b : TrialPhaseV1)
    (hab : canAdvancePhaseV1 a b = true)
    (hne : a ≠ b) :
    trialPhaseRankV1 a < trialPhaseRankV1 b := by
  have hle : trialPhaseRankV1 a ≤ trialPhaseRankV1 b := by
    simpa [canAdvancePhaseV1] using hab
  have hneRank : trialPhaseRankV1 a ≠ trialPhaseRankV1 b := by
    intro heq
    have hab' : canAdvancePhaseV1 a b = true := hab
    have hbaRank : trialPhaseRankV1 b ≤ trialPhaseRankV1 a := by
      rw [← heq]
      exact Nat.le_refl _
    have hba : canAdvancePhaseV1 b a = true := by
      simpa [canAdvancePhaseV1] using hbaRank
    exact hne (canAdvancePhaseV1_antisymm_true a b hab' hba)
  exact Nat.lt_of_le_of_ne hle hneRank

theorem phaseAllowsActionV1_opening_only_openings (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .recordOpeningStatement phase = true -> phase = .openings := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_opening_true_iff_openings (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .recordOpeningStatement phase = true ↔ phase = .openings := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_verdict_only_verdictReturn (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .recordJuryVerdict phase = true -> phase = .verdictReturn := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_verdict_true_iff_verdictReturn (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .recordJuryVerdict phase = true ↔ phase = .verdictReturn := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_poll_only_postVerdict (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .pollJury phase = true -> phase = .postVerdict := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_poll_true_iff_postVerdict (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .pollJury phase = true ↔ phase = .postVerdict := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_hung_implies_delib_or_verdictReturn (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .declareHungJury phase = true ->
      phase = .deliberation ∨ phase = .verdictReturn := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_offerExhibit_true_iff_party_case (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .offerExhibit phase = true ↔
      phase = .plaintiffCase ∨
      phase = .defenseCase ∨
      phase = .plaintiffRebuttal ∨
      phase = .defenseSurrebuttal := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem phaseAllowsActionV1_hung_true_iff_delib_or_verdictReturn (phase : TrialPhaseV1) :
    phaseAllowsActionV1 .declareHungJury phase = true ↔
      phase = .deliberation ∨ phase = .verdictReturn := by
  cases phase <;> simp [phaseAllowsActionV1]

theorem parseVerdictSide_plaintiff :
    parseVerdictSide "plaintiff" = some .plaintiff := by
  simp [parseVerdictSide]

theorem parseVerdictSide_defendant :
    parseVerdictSide "defendant" = some .defendant := by
  simp [parseVerdictSide]

theorem parseVerdictSide_some_implies_token
    (s : String) (side : VerdictSide) :
    parseVerdictSide s = some side ->
      (s = "plaintiff" ∧ side = .plaintiff) ∨
      (s = "defendant" ∧ side = .defendant) := by
  intro h
  cases side with
  | plaintiff =>
      by_cases hpl : s = "plaintiff"
      · exact Or.inl ⟨hpl, rfl⟩
      · by_cases hdef : s = "defendant"
        · subst hdef
          simp [parseVerdictSide] at h
        · simp [parseVerdictSide] at h
  | defendant =>
      by_cases hpl : s = "plaintiff"
      · simp [parseVerdictSide, hpl] at h
      · by_cases hdef : s = "defendant"
        · exact Or.inr ⟨hdef, rfl⟩
        · simp [parseVerdictSide] at h

theorem legalVerdictToken_implies_parseVerdict_some
    (s : String) :
    (s = "plaintiff" ∨ s = "defendant") ->
      ∃ side : VerdictSide, parseVerdictSide s = some side := by
  intro h
  rcases h with hpl | hdef
  · subst hpl
    exact ⟨.plaintiff, by simp [parseVerdictSide]⟩
  · subst hdef
    exact ⟨.defendant, by simp [parseVerdictSide]⟩

theorem canEnterJudgmentFromClaimDispositionV1_hung_false :
    canEnterJudgmentFromClaimDispositionV1 .hung = false := by
  simp [canEnterJudgmentFromClaimDispositionV1]

theorem canEnterJudgmentFromClaimDispositionV1_pending_false :
    canEnterJudgmentFromClaimDispositionV1 .pending = false := by
  simp [canEnterJudgmentFromClaimDispositionV1]

theorem canEnterJudgmentFromClaimDispositionV1_verdictPlaintiff_true :
    canEnterJudgmentFromClaimDispositionV1 .verdictPlaintiff = true := by
  simp [canEnterJudgmentFromClaimDispositionV1]

theorem canEnterJudgmentFromClaimDispositionV1_verdictDefendant_true :
    canEnterJudgmentFromClaimDispositionV1 .verdictDefendant = true := by
  simp [canEnterJudgmentFromClaimDispositionV1]

theorem canEnterJudgmentFromClaimDispositionV1_true_iff_verdict (d : ClaimDispositionV1) :
    canEnterJudgmentFromClaimDispositionV1 d = true ↔
      d = .verdictPlaintiff ∨ d = .verdictDefendant := by
  cases d <;> simp [canEnterJudgmentFromClaimDispositionV1]

theorem canEnterJudgmentFromClaimDispositionV1_false_iff_nonverdict (d : ClaimDispositionV1) :
    canEnterJudgmentFromClaimDispositionV1 d = false ↔
      d = .pending ∨ d = .hung ∨ d = .judgmentEntered := by
  cases d <;> simp [canEnterJudgmentFromClaimDispositionV1]

theorem claimDispositionFromCaseStateV1_hung (c : CaseState)
    (h : c.hung_jury.isSome = true) :
    claimDispositionFromCaseStateV1 c = pure .hung := by
  simp [claimDispositionFromCaseStateV1, h]

theorem claimDispositionFromCaseStateV1_pending (c : CaseState)
    (hh : c.hung_jury = none) (hv : c.jury_verdict = none) :
    claimDispositionFromCaseStateV1 c = pure .pending := by
  simp [claimDispositionFromCaseStateV1, hh, hv]

theorem claimDispositionFromCaseStateV1_plaintiff_verdict (c : CaseState)
    (hh : c.hung_jury = none) :
    claimDispositionFromCaseStateV1
      { c with jury_verdict := some { verdict_for := "plaintiff", votes_for_verdict := 6, required_votes := 6, damages := 0.0 } }
      = pure .verdictPlaintiff := by
  simp [claimDispositionFromCaseStateV1, hh, parseVerdictSide]

theorem claimDispositionFromCaseStateV1_defendant_verdict (c : CaseState)
    (hh : c.hung_jury = none) :
    claimDispositionFromCaseStateV1
      { c with jury_verdict := some { verdict_for := "defendant", votes_for_verdict := 6, required_votes := 6, damages := 0.0 } }
      = pure .verdictDefendant := by
  simp [claimDispositionFromCaseStateV1, hh, parseVerdictSide]

theorem claimDispositionFromCaseStateV1_ne_judgmentEntered (c : CaseState) :
    claimDispositionFromCaseStateV1 c ≠ pure .judgmentEntered := by
  intro h
  cases hh : c.hung_jury with
  | some hj =>
      have hhung : claimDispositionFromCaseStateV1 c = pure .hung := by
        simp [claimDispositionFromCaseStateV1, hh]
      rw [hhung] at h
      cases h
  | none =>
      cases hv : c.jury_verdict with
      | none =>
          have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
            simp [claimDispositionFromCaseStateV1, hh, hv]
          rw [hpending] at h
          cases h
      | some v =>
          cases hp : parseVerdictSide v.verdict_for with
          | none =>
              have herr : claimDispositionFromCaseStateV1 c =
                  throw s!"invalid verdict_for value: {v.verdict_for}" := by
                simp [claimDispositionFromCaseStateV1, hh, hv, hp]
              rw [herr] at h
              cases h
          | some side =>
              cases side with
              | plaintiff =>
                  have hpl : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff := by
                    simp [claimDispositionFromCaseStateV1, hh, hv, hp]
                  rw [hpl] at h
                  cases h
              | defendant =>
                  have hdef : claimDispositionFromCaseStateV1 c = pure .verdictDefendant := by
                    simp [claimDispositionFromCaseStateV1, hh, hv, hp]
                  rw [hdef] at h
                  cases h

theorem claimDispositionFromCaseStateV1_pending_iff (c : CaseState) :
    claimDispositionFromCaseStateV1 c = pure .pending ↔
      c.hung_jury = none ∧ c.jury_verdict = none := by
  constructor
  · intro h
    have hh : c.hung_jury = none := by
      cases hhung : c.hung_jury with
      | none => rfl
      | some hj =>
          have hhung : claimDispositionFromCaseStateV1 c = pure .hung := by
            simp [claimDispositionFromCaseStateV1, hhung]
          rw [hhung] at h
          cases h
    have hv : c.jury_verdict = none := by
      cases hvørd : c.jury_verdict with
      | none => rfl
      | some v =>
          have hfalse : False := by
            cases hp : parseVerdictSide v.verdict_for with
            | none =>
                simp [claimDispositionFromCaseStateV1, hh, hvørd, hp] at h
                cases h
            | some side =>
                cases side <;> (simp [claimDispositionFromCaseStateV1, hh, hvørd, hp] at h; cases h)
          exact False.elim hfalse
    exact And.intro hh hv
  · intro h
    rcases h with ⟨hh, hv⟩
    simp [claimDispositionFromCaseStateV1, hh, hv]

theorem claimDisposition_allowsJudgment_implies_no_hung
    (c : CaseState) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d)
    (hj : canEnterJudgmentFromClaimDispositionV1 d = true) :
    c.hung_jury = none := by
  cases hh : c.hung_jury with
  | none => rfl
  | some hrec =>
      have hdisp : claimDispositionFromCaseStateV1 c = pure .hung := by
        simp [claimDispositionFromCaseStateV1, hh]
      rw [hdisp] at hd
      have hd' : d = .hung := by
        cases hd
        rfl
      rw [hd'] at hj
      simp [canEnterJudgmentFromClaimDispositionV1] at hj

theorem claimDisposition_allowsJudgment_implies_verdict_present
    (c : CaseState) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d)
    (hj : canEnterJudgmentFromClaimDispositionV1 d = true) :
    c.jury_verdict.isSome = true := by
  have hh : c.hung_jury = none :=
    claimDisposition_allowsJudgment_implies_no_hung c d hd hj
  cases hv : c.jury_verdict with
  | none =>
      have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
        simp [claimDispositionFromCaseStateV1, hh, hv]
      rw [hpending] at hd
      have hd' : d = .pending := by
        cases hd
        rfl
      rw [hd'] at hj
      simp [canEnterJudgmentFromClaimDispositionV1] at hj
  | some v =>
      simp

theorem claimDisposition_allowsJudgment_implies_parseVerdict_some
    (c : CaseState) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d)
    (hj : canEnterJudgmentFromClaimDispositionV1 d = true) :
    ∃ v : JuryVerdict, ∃ side : VerdictSide,
      c.jury_verdict = some v ∧ parseVerdictSide v.verdict_for = some side := by
  have hh : c.hung_jury = none :=
    claimDisposition_allowsJudgment_implies_no_hung c d hd hj
  cases hv : c.jury_verdict with
  | none =>
      have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
        simp [claimDispositionFromCaseStateV1, hh, hv]
      rw [hpending] at hd
      have hd' : d = .pending := by
        cases hd
        rfl
      rw [hd'] at hj
      simp [canEnterJudgmentFromClaimDispositionV1] at hj
  | some v =>
      cases hp : parseVerdictSide v.verdict_for with
      | none =>
          have hnone : claimDispositionFromCaseStateV1 c =
              throw s!"invalid verdict_for value: {v.verdict_for}" := by
            simp [claimDispositionFromCaseStateV1, hh, hv, hp]
          rw [hnone] at hd
          cases hd
      | some side =>
          refine ⟨v, side, ?_, hp⟩
          simp

theorem validVerdictSideState_implies_claimDisposition_allowsJudgment
    (c : CaseState) (v : JuryVerdict) (side : VerdictSide)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hp : parseVerdictSide v.verdict_for = some side) :
    ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true := by
  cases side with
  | plaintiff =>
      refine ⟨.verdictPlaintiff, ?_, ?_⟩
      · simp [claimDispositionFromCaseStateV1, hh, hv, hp]
      · simp [canEnterJudgmentFromClaimDispositionV1]
  | defendant =>
      refine ⟨.verdictDefendant, ?_, ?_⟩
      · simp [claimDispositionFromCaseStateV1, hh, hv, hp]
      · simp [canEnterJudgmentFromClaimDispositionV1]

theorem claimDisposition_invalidVerdict_for_is_error
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hp : parseVerdictSide v.verdict_for = none) :
    claimDispositionFromCaseStateV1 c =
      throw s!"invalid verdict_for value: {v.verdict_for}" := by
  simp [claimDispositionFromCaseStateV1, hh, hv, hp]

theorem claimDisposition_invalidVerdict_for_not_pure
    (c : CaseState) (v : JuryVerdict) (d : ClaimDispositionV1)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hp : parseVerdictSide v.verdict_for = none) :
    claimDispositionFromCaseStateV1 c ≠ pure d := by
  intro h
  have herr : claimDispositionFromCaseStateV1 c =
      throw s!"invalid verdict_for value: {v.verdict_for}" :=
    claimDisposition_invalidVerdict_for_is_error c v hh hv hp
  rw [herr] at h
  cases h

theorem invalidVerdictToken_blocks_judgment_eligibility
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hp : parseVerdictSide v.verdict_for = none) :
    ¬ ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true := by
  intro hex
  rcases hex with ⟨d, hd, _⟩
  exact claimDisposition_invalidVerdict_for_not_pure c v d hh hv hp hd

theorem judgmentEligibility_iff_wellFormedVerdict (c : CaseState) :
    (∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true) ↔
    (c.hung_jury = none ∧
      ∃ v : JuryVerdict, ∃ side : VerdictSide,
        c.jury_verdict = some v ∧ parseVerdictSide v.verdict_for = some side) := by
  constructor
  · intro h
    rcases h with ⟨d, hd, hj⟩
    have hh : c.hung_jury = none :=
      claimDisposition_allowsJudgment_implies_no_hung c d hd hj
    rcases claimDisposition_allowsJudgment_implies_parseVerdict_some c d hd hj with
      ⟨v, side, hv, hp⟩
    exact ⟨hh, v, side, hv, hp⟩
  · intro h
    rcases h with ⟨hh, v, side, hv, hp⟩
    rcases validVerdictSideState_implies_claimDisposition_allowsJudgment c v side hh hv hp with
      ⟨d, hd, hj⟩
    exact ⟨d, hd, hj⟩

theorem hungJury_blocks_judgment_eligibility
    (c : CaseState) (hhang : c.hung_jury.isSome = true) :
    ¬ ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true := by
  intro hex
  rcases hex with ⟨d, hd, hj⟩
  have hdHung : claimDispositionFromCaseStateV1 c = pure .hung :=
    claimDispositionFromCaseStateV1_hung c hhang
  rw [hdHung] at hd
  have dEq : d = .hung := by
    cases hd
    rfl
  rw [dEq] at hj
  simp [canEnterJudgmentFromClaimDispositionV1] at hj

theorem noVerdict_noHung_blocks_judgment_eligibility
    (c : CaseState) (hh : c.hung_jury = none) (hv : c.jury_verdict = none) :
    ¬ ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true := by
  intro hex
  rcases hex with ⟨d, hd, hj⟩
  have hdPending : claimDispositionFromCaseStateV1 c = pure .pending :=
    claimDispositionFromCaseStateV1_pending c hh hv
  rw [hdPending] at hd
  have dEq : d = .pending := by
    cases hd
    rfl
  rw [dEq] at hj
  simp [canEnterJudgmentFromClaimDispositionV1] at hj

theorem claimDisposition_allowsJudgment_implies_verdict_for_legal_token
    (c : CaseState) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d)
    (hj : canEnterJudgmentFromClaimDispositionV1 d = true) :
    ∃ v : JuryVerdict,
      c.jury_verdict = some v ∧
        (v.verdict_for = "plaintiff" ∨ v.verdict_for = "defendant") := by
  rcases claimDisposition_allowsJudgment_implies_parseVerdict_some c d hd hj with
    ⟨v, side, hv, hp⟩
  have htok :
      (v.verdict_for = "plaintiff" ∧ side = .plaintiff) ∨
      (v.verdict_for = "defendant" ∧ side = .defendant) :=
    parseVerdictSide_some_implies_token v.verdict_for side hp
  refine ⟨v, hv, ?_⟩
  rcases htok with hpl | hdef
  · exact Or.inl hpl.1
  · exact Or.inr hdef.1

theorem noHung_and_legalVerdictToken_implies_judgment_eligibility
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (htok : v.verdict_for = "plaintiff" ∨ v.verdict_for = "defendant") :
    ∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true := by
  rcases legalVerdictToken_implies_parseVerdict_some v.verdict_for htok with
    ⟨side, hp⟩
  exact validVerdictSideState_implies_claimDisposition_allowsJudgment c v side hh hv hp

theorem noHung_verdictTokenPlaintiff_implies_dispositionPlaintiff
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hpl : v.verdict_for = "plaintiff") :
    claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff := by
  have hp : parseVerdictSide v.verdict_for = some .plaintiff := by
    simpa [hpl] using parseVerdictSide_plaintiff
  simp [claimDispositionFromCaseStateV1, hh, hv, hp]

theorem noHung_verdictTokenDefendant_implies_dispositionDefendant
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hdef : v.verdict_for = "defendant") :
    claimDispositionFromCaseStateV1 c = pure .verdictDefendant := by
  have hp : parseVerdictSide v.verdict_for = some .defendant := by
    simpa [hdef] using parseVerdictSide_defendant
  simp [claimDispositionFromCaseStateV1, hh, hv, hp]

theorem noHung_verdictTokenPlaintiff_implies_exact_judgment_witness
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hpl : v.verdict_for = "plaintiff") :
    claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff ∧
      canEnterJudgmentFromClaimDispositionV1 .verdictPlaintiff = true := by
  refine And.intro ?_ ?_
  · exact noHung_verdictTokenPlaintiff_implies_dispositionPlaintiff c v hh hv hpl
  · simp [canEnterJudgmentFromClaimDispositionV1]

theorem noHung_verdictTokenDefendant_implies_exact_judgment_witness
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hdef : v.verdict_for = "defendant") :
    claimDispositionFromCaseStateV1 c = pure .verdictDefendant ∧
      canEnterJudgmentFromClaimDispositionV1 .verdictDefendant = true := by
  refine And.intro ?_ ?_
  · exact noHung_verdictTokenDefendant_implies_dispositionDefendant c v hh hv hdef
  · simp [canEnterJudgmentFromClaimDispositionV1]

theorem noHung_verdictTokenPlaintiff_not_dispositionDefendant
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hpl : v.verdict_for = "plaintiff") :
    claimDispositionFromCaseStateV1 c ≠ pure .verdictDefendant := by
  have hdisp : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff :=
    noHung_verdictTokenPlaintiff_implies_dispositionPlaintiff c v hh hv hpl
  intro h
  rw [hdisp] at h
  cases h

theorem noHung_verdictTokenDefendant_not_dispositionPlaintiff
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (hdef : v.verdict_for = "defendant") :
    claimDispositionFromCaseStateV1 c ≠ pure .verdictPlaintiff := by
  have hdisp : claimDispositionFromCaseStateV1 c = pure .verdictDefendant :=
    noHung_verdictTokenDefendant_implies_dispositionDefendant c v hh hv hdef
  intro h
  rw [hdisp] at h
  cases h

theorem dispositionPlaintiff_iff_noHung_and_tokenPlaintiff (c : CaseState) :
    claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff ↔
      (c.hung_jury = none ∧
        ∃ v : JuryVerdict, c.jury_verdict = some v ∧ v.verdict_for = "plaintiff") := by
  constructor
  · intro hd
    have hh : c.hung_jury = none := by
      cases hhang : c.hung_jury with
      | none => rfl
      | some hj =>
          have hhung : claimDispositionFromCaseStateV1 c = pure .hung := by
            simp [claimDispositionFromCaseStateV1, hhang]
          rw [hhung] at hd
          cases hd
    cases hv : c.jury_verdict with
    | none =>
        have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
          simp [claimDispositionFromCaseStateV1, hh, hv]
        rw [hpending] at hd
        cases hd
    | some v =>
        cases hp : parseVerdictSide v.verdict_for with
        | none =>
            have herr : claimDispositionFromCaseStateV1 c =
                throw s!"invalid verdict_for value: {v.verdict_for}" := by
              simp [claimDispositionFromCaseStateV1, hh, hv, hp]
            rw [herr] at hd
            cases hd
        | some side =>
            cases side with
            | plaintiff =>
                refine ⟨hh, v, ?_, ?_⟩
                · simp
                have htok :
                    (v.verdict_for = "plaintiff" ∧ VerdictSide.plaintiff = .plaintiff) ∨
                    (v.verdict_for = "defendant" ∧ VerdictSide.plaintiff = .defendant) :=
                  parseVerdictSide_some_implies_token v.verdict_for .plaintiff hp
                rcases htok with hpl | hdef
                · exact hpl.1
                · cases hdef.2
            | defendant =>
                have hdefdisp : claimDispositionFromCaseStateV1 c = pure .verdictDefendant := by
                  simp [claimDispositionFromCaseStateV1, hh, hv, hp]
                rw [hdefdisp] at hd
                cases hd
  · intro h
    rcases h with ⟨hh, v, hv, hpl⟩
    exact noHung_verdictTokenPlaintiff_implies_dispositionPlaintiff c v hh hv hpl

theorem dispositionDefendant_iff_noHung_and_tokenDefendant (c : CaseState) :
    claimDispositionFromCaseStateV1 c = pure .verdictDefendant ↔
      (c.hung_jury = none ∧
        ∃ v : JuryVerdict, c.jury_verdict = some v ∧ v.verdict_for = "defendant") := by
  constructor
  · intro hd
    have hh : c.hung_jury = none := by
      cases hhang : c.hung_jury with
      | none => rfl
      | some hj =>
          have hhung : claimDispositionFromCaseStateV1 c = pure .hung := by
            simp [claimDispositionFromCaseStateV1, hhang]
          rw [hhung] at hd
          cases hd
    cases hv : c.jury_verdict with
    | none =>
        have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
          simp [claimDispositionFromCaseStateV1, hh, hv]
        rw [hpending] at hd
        cases hd
    | some v =>
        cases hp : parseVerdictSide v.verdict_for with
        | none =>
            have herr : claimDispositionFromCaseStateV1 c =
                throw s!"invalid verdict_for value: {v.verdict_for}" := by
              simp [claimDispositionFromCaseStateV1, hh, hv, hp]
            rw [herr] at hd
            cases hd
        | some side =>
            cases side with
            | plaintiff =>
                have hpldisp : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff := by
                  simp [claimDispositionFromCaseStateV1, hh, hv, hp]
                rw [hpldisp] at hd
                cases hd
            | defendant =>
                refine ⟨hh, v, ?_, ?_⟩
                · simp
                have htok :
                    (v.verdict_for = "plaintiff" ∧ VerdictSide.defendant = .plaintiff) ∨
                    (v.verdict_for = "defendant" ∧ VerdictSide.defendant = .defendant) :=
                  parseVerdictSide_some_implies_token v.verdict_for .defendant hp
                rcases htok with hpl | hdef
                · cases hpl.2
                · exact hdef.1
  · intro h
    rcases h with ⟨hh, v, hv, hdef⟩
    exact noHung_verdictTokenDefendant_implies_dispositionDefendant c v hh hv hdef

theorem dispositionPlaintiff_implies_noHung
    (c : CaseState)
    (hpl : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff) :
    c.hung_jury = none := by
  exact (dispositionPlaintiff_iff_noHung_and_tokenPlaintiff c).mp hpl |>.1

theorem dispositionDefendant_implies_noHung
    (c : CaseState)
    (hdef : claimDispositionFromCaseStateV1 c = pure .verdictDefendant) :
    c.hung_jury = none := by
  exact (dispositionDefendant_iff_noHung_and_tokenDefendant c).mp hdef |>.1

theorem dispositionIsVerdict_implies_noHung
    (c : CaseState)
    (h : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff ∨
      claimDispositionFromCaseStateV1 c = pure .verdictDefendant) :
    c.hung_jury = none := by
  rcases h with hpl | hdef
  · exact dispositionPlaintiff_implies_noHung c hpl
  · exact dispositionDefendant_implies_noHung c hdef

theorem judgmentEligibility_iff_noHung_and_legalToken (c : CaseState) :
    (∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true) ↔
    (c.hung_jury = none ∧
      ∃ v : JuryVerdict,
        c.jury_verdict = some v ∧
          (v.verdict_for = "plaintiff" ∨ v.verdict_for = "defendant")) := by
  constructor
  · intro h
    rcases h with ⟨d, hd, hj⟩
    have hh : c.hung_jury = none :=
      claimDisposition_allowsJudgment_implies_no_hung c d hd hj
    rcases claimDisposition_allowsJudgment_implies_verdict_for_legal_token c d hd hj with
      ⟨v, hv, htok⟩
    exact ⟨hh, v, hv, htok⟩
  · intro h
    rcases h with ⟨hh, v, hv, htok⟩
    exact noHung_and_legalVerdictToken_implies_judgment_eligibility c v hh hv htok

theorem noHung_and_legalVerdictToken_implies_not_pending
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (htok : v.verdict_for = "plaintiff" ∨ v.verdict_for = "defendant") :
    claimDispositionFromCaseStateV1 c ≠ pure .pending := by
  rcases noHung_and_legalVerdictToken_implies_judgment_eligibility c v hh hv htok with
    ⟨d, hd, hj⟩
  intro hpending
  rw [hpending] at hd
  have dEq : d = .pending := by
    cases hd
    rfl
  rw [dEq] at hj
  simp [canEnterJudgmentFromClaimDispositionV1] at hj

theorem noHung_and_legalVerdictToken_implies_not_hung
    (c : CaseState) (v : JuryVerdict)
    (hh : c.hung_jury = none)
    (hv : c.jury_verdict = some v)
    (htok : v.verdict_for = "plaintiff" ∨ v.verdict_for = "defendant") :
    claimDispositionFromCaseStateV1 c ≠ pure .hung := by
  intro hhung
  rcases htok with hpl | hdef
  · have hdisp : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff :=
      noHung_verdictTokenPlaintiff_implies_dispositionPlaintiff c v hh hv hpl
    rw [hdisp] at hhung
    cases hhung
  · have hdisp : claimDispositionFromCaseStateV1 c = pure .verdictDefendant :=
      noHung_verdictTokenDefendant_implies_dispositionDefendant c v hh hv hdef
    rw [hdisp] at hhung
    cases hhung

theorem dispositionIsVerdict_iff_wellFormedVerdictState (c : CaseState) :
    (claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff ∨
      claimDispositionFromCaseStateV1 c = pure .verdictDefendant) ↔
    (c.hung_jury = none ∧
      ∃ v : JuryVerdict, ∃ side : VerdictSide,
        c.jury_verdict = some v ∧ parseVerdictSide v.verdict_for = some side) := by
  constructor
  · intro h
    rcases h with hpl | hdef
    · have helig : ∃ d : ClaimDispositionV1,
          claimDispositionFromCaseStateV1 c = pure d ∧
          canEnterJudgmentFromClaimDispositionV1 d = true := by
        exact ⟨.verdictPlaintiff, hpl, by simp [canEnterJudgmentFromClaimDispositionV1]⟩
      exact (judgmentEligibility_iff_wellFormedVerdict c).mp helig
    · have helig : ∃ d : ClaimDispositionV1,
          claimDispositionFromCaseStateV1 c = pure d ∧
          canEnterJudgmentFromClaimDispositionV1 d = true := by
        exact ⟨.verdictDefendant, hdef, by simp [canEnterJudgmentFromClaimDispositionV1]⟩
      exact (judgmentEligibility_iff_wellFormedVerdict c).mp helig
  · intro h
    have helig : ∃ d : ClaimDispositionV1,
        claimDispositionFromCaseStateV1 c = pure d ∧
        canEnterJudgmentFromClaimDispositionV1 d = true :=
      (judgmentEligibility_iff_wellFormedVerdict c).mpr h
    rcases helig with ⟨d, hd, hj⟩
    have hdver : d = .verdictPlaintiff ∨ d = .verdictDefendant :=
      (canEnterJudgmentFromClaimDispositionV1_true_iff_verdict d).mp hj
    rcases hdver with hdpl | hddef
    · left
      rw [hdpl] at hd
      exact hd
    · right
      rw [hddef] at hd
      exact hd

theorem judgmentEligibility_iff_dispositionIsVerdict (c : CaseState) :
    (∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true) ↔
    (claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff ∨
      claimDispositionFromCaseStateV1 c = pure .verdictDefendant) := by
  constructor
  · intro h
    rcases h with ⟨d, hd, hj⟩
    have hdver : d = .verdictPlaintiff ∨ d = .verdictDefendant :=
      (canEnterJudgmentFromClaimDispositionV1_true_iff_verdict d).mp hj
    rcases hdver with hdpl | hddef
    · left
      rw [hdpl] at hd
      exact hd
    · right
      rw [hddef] at hd
      exact hd
  · intro h
    rcases h with hpl | hdef
    · exact ⟨.verdictPlaintiff, hpl, by simp [canEnterJudgmentFromClaimDispositionV1]⟩
    · exact ⟨.verdictDefendant, hdef, by simp [canEnterJudgmentFromClaimDispositionV1]⟩

theorem judgmentEligibility_witness_unique
    (c : CaseState) (d1 d2 : ClaimDispositionV1)
    (h1 : claimDispositionFromCaseStateV1 c = pure d1)
    (h2 : claimDispositionFromCaseStateV1 c = pure d2) :
    d1 = d2 := by
  rw [h1] at h2
  cases h2
  rfl

theorem judgmentEligibility_existsUnique_iff_exists (c : CaseState) :
    (∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true) ↔
    (∃ d : ClaimDispositionV1,
      claimDispositionFromCaseStateV1 c = pure d ∧
      canEnterJudgmentFromClaimDispositionV1 d = true ∧
      ∀ d' : ClaimDispositionV1,
        claimDispositionFromCaseStateV1 c = pure d' ∧
        canEnterJudgmentFromClaimDispositionV1 d' = true ->
        d' = d) := by
  constructor
  · intro hex
    rcases hex with ⟨d, hd, hj⟩
    refine ⟨d, hd, hj, ?_⟩
    intro d' hd'
    exact (judgmentEligibility_witness_unique c d d' hd hd'.1).symm
  · intro huniq
    rcases huniq with ⟨d, hd, hj, _⟩
    exact ⟨d, hd, hj⟩

theorem noHung_pureNonPending_iff_parseableVerdictPresent
    (c : CaseState) (hh : c.hung_jury = none) :
    (∃ d : ClaimDispositionV1, claimDispositionFromCaseStateV1 c = pure d ∧ d ≠ .pending) ↔
      ∃ v : JuryVerdict, ∃ side : VerdictSide,
        c.jury_verdict = some v ∧ parseVerdictSide v.verdict_for = some side := by
  constructor
  · intro h
    rcases h with ⟨d, hd, hnp⟩
    cases hv : c.jury_verdict with
    | none =>
        have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
          simp [claimDispositionFromCaseStateV1, hh, hv]
        rw [hpending] at hd
        have dEq : d = .pending := by
          cases hd
          rfl
        exact (hnp dEq).elim
    | some v =>
        cases hp : parseVerdictSide v.verdict_for with
        | none =>
            have herr : claimDispositionFromCaseStateV1 c =
                throw s!"invalid verdict_for value: {v.verdict_for}" := by
              simp [claimDispositionFromCaseStateV1, hh, hv, hp]
            rw [herr] at hd
            cases hd
        | some side =>
            refine ⟨v, side, ?_, hp⟩
            simp
  · intro hparse
    rcases hparse with ⟨v, side, hv, hp⟩
    rcases validVerdictSideState_implies_claimDisposition_allowsJudgment c v side hh hv hp with
      ⟨d, hd, hj⟩
    have hdver : d = .verdictPlaintiff ∨ d = .verdictDefendant :=
      (canEnterJudgmentFromClaimDispositionV1_true_iff_verdict d).mp hj
    refine ⟨d, hd, ?_⟩
    intro hpend
    rw [hpend] at hdver
    rcases hdver with hpl | hdef
    · cases hpl
    · cases hdef

theorem noHung_pureDisposition_partition
    (c : CaseState) (hh : c.hung_jury = none) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d) :
    d = .pending ∨ d = .verdictPlaintiff ∨ d = .verdictDefendant := by
  cases hv : c.jury_verdict with
  | none =>
      have hpending : claimDispositionFromCaseStateV1 c = pure .pending := by
        simp [claimDispositionFromCaseStateV1, hh, hv]
      rw [hpending] at hd
      left
      cases hd
      rfl
  | some v =>
      cases hp : parseVerdictSide v.verdict_for with
      | none =>
          have herr : claimDispositionFromCaseStateV1 c =
              throw s!"invalid verdict_for value: {v.verdict_for}" := by
            simp [claimDispositionFromCaseStateV1, hh, hv, hp]
          rw [herr] at hd
          cases hd
      | some side =>
          cases side with
          | plaintiff =>
              right
              left
              have hpl : claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff := by
                simp [claimDispositionFromCaseStateV1, hh, hv, hp]
              rw [hpl] at hd
              cases hd
              rfl
          | defendant =>
              right
              right
              have hdef : claimDispositionFromCaseStateV1 c = pure .verdictDefendant := by
                simp [claimDispositionFromCaseStateV1, hh, hv, hp]
              rw [hdef] at hd
              cases hd
              rfl

theorem noHung_pureNonPending_iff_dispositionIsVerdict
    (c : CaseState) (hh : c.hung_jury = none) :
    (∃ d : ClaimDispositionV1, claimDispositionFromCaseStateV1 c = pure d ∧ d ≠ .pending) ↔
      (claimDispositionFromCaseStateV1 c = pure .verdictPlaintiff ∨
        claimDispositionFromCaseStateV1 c = pure .verdictDefendant) := by
  constructor
  · intro h
    rcases noHung_pureNonPending_iff_parseableVerdictPresent c hh |>.mp h with
      ⟨v, side, hv, hp⟩
    rcases validVerdictSideState_implies_claimDisposition_allowsJudgment c v side hh hv hp with
      ⟨d, hd, hj⟩
    exact (judgmentEligibility_iff_dispositionIsVerdict c).mp ⟨d, hd, hj⟩
  · intro h
    rcases h with hpl | hdef
    · exact ⟨.verdictPlaintiff, hpl, by decide⟩
    · exact ⟨.verdictDefendant, hdef, by decide⟩

theorem noHung_pureNonPending_implies_canEnter
    (c : CaseState) (hh : c.hung_jury = none) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d)
    (hnp : d ≠ .pending) :
    canEnterJudgmentFromClaimDispositionV1 d = true := by
  have hpart : d = .pending ∨ d = .verdictPlaintiff ∨ d = .verdictDefendant :=
    noHung_pureDisposition_partition c hh d hd
  rcases hpart with hpend | hpl | hdef
  · exact (hnp hpend).elim
  · rw [hpl]
    simp [canEnterJudgmentFromClaimDispositionV1]
  · rw [hdef]
    simp [canEnterJudgmentFromClaimDispositionV1]

theorem noHung_pureDisposition_ne_hung_and_ne_judgmentEntered
    (c : CaseState) (hh : c.hung_jury = none) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d) :
    d ≠ .hung ∧ d ≠ .judgmentEntered := by
  have hpart : d = .pending ∨ d = .verdictPlaintiff ∨ d = .verdictDefendant :=
    noHung_pureDisposition_partition c hh d hd
  constructor
  · intro hhung
    rcases hpart with hpend | hpl | hdef
    · rw [hpend] at hhung
      cases hhung
    · rw [hpl] at hhung
      cases hhung
    · rw [hdef] at hhung
      cases hhung
  · intro hj
    rcases hpart with hpend | hpl | hdef
    · rw [hpend] at hj
      cases hj
    · rw [hpl] at hj
      cases hj
    · rw [hdef] at hj
      cases hj

theorem noHung_pureDisposition_canEnter_iff_nonpending
    (c : CaseState) (hh : c.hung_jury = none) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d) :
    canEnterJudgmentFromClaimDispositionV1 d = true ↔ d ≠ .pending := by
  constructor
  · intro hcan
    have hpart : d = .pending ∨ d = .verdictPlaintiff ∨ d = .verdictDefendant :=
      noHung_pureDisposition_partition c hh d hd
    rcases hpart with hpend | hpl | hdef
    · intro h
      rw [h] at hcan
      simp [canEnterJudgmentFromClaimDispositionV1] at hcan
    · intro h
      rw [h] at hpl
      cases hpl
    · intro h
      rw [h] at hdef
      cases hdef
  · intro hnp
    exact noHung_pureNonPending_implies_canEnter c hh d hd hnp

theorem noHung_pureDisposition_canEnter_false_iff_pending
    (c : CaseState) (hh : c.hung_jury = none) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d) :
    canEnterJudgmentFromClaimDispositionV1 d = false ↔ d = .pending := by
  constructor
  · intro hfalse
    by_cases hpend : d = .pending
    · exact hpend
    · have htrue : canEnterJudgmentFromClaimDispositionV1 d = true :=
        (noHung_pureDisposition_canEnter_iff_nonpending c hh d hd).mpr hpend
      rw [htrue] at hfalse
      cases hfalse
  · intro hpend
    rw [hpend]
    simp [canEnterJudgmentFromClaimDispositionV1]

theorem noHung_pureDisposition_canEnter_false_implies_not_verdict
    (c : CaseState) (hh : c.hung_jury = none) (d : ClaimDispositionV1)
    (hd : claimDispositionFromCaseStateV1 c = pure d)
    (hfalse : canEnterJudgmentFromClaimDispositionV1 d = false) :
    d ≠ .verdictPlaintiff ∧ d ≠ .verdictDefendant := by
  have hpend : d = .pending :=
    (noHung_pureDisposition_canEnter_false_iff_pending c hh d hd).mp hfalse
  constructor
  · intro hpl
    rw [hpend] at hpl
    cases hpl
  · intro hdef
    rw [hpend] at hdef
    cases hdef

theorem noHung_pureDisposition_verdict_implies_canEnter_true
    (d : ClaimDispositionV1)
    (hver : d = .verdictPlaintiff ∨ d = .verdictDefendant) :
    canEnterJudgmentFromClaimDispositionV1 d = true := by
  rcases hver with hpl | hdef
  · rw [hpl]
    simp [canEnterJudgmentFromClaimDispositionV1]
  · rw [hdef]
    simp [canEnterJudgmentFromClaimDispositionV1]
