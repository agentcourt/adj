import Main

theorem validateRecordJuryVerdict_invalid_current_phase
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (msg : String)
    (hPhase : parseCurrentPhaseV1 c = .error msg) :
    validateRecordJuryVerdict c verdictFor votes damages = .error msg := by
  unfold validateRecordJuryVerdict
  simp [hPhase]

theorem validateRecordJuryVerdict_phase_gate_error
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = false) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error s!"jury verdict requires verdict_return phase; current phase is {c.phase}" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate]

theorem validateRecordJuryVerdict_hung_error
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = true) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error "cannot record verdict after hung jury is declared" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate, hHung]

theorem validateRecordJuryVerdict_invalid_verdict_for
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = false)
    (hSide : parseVerdictSide verdictFor = none) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error s!"invalid verdict_for value: {verdictFor}" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate, hHung, hSide]

theorem validateRecordJuryVerdict_negative_damages
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (side : VerdictSide)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = false)
    (hSide : parseVerdictSide verdictFor = some side)
    (hNeg : damages < 0) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error "damages must be nonnegative" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate, hHung, hSide, hNeg]

theorem validateRecordJuryVerdict_defendant_nonzero_damages
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = false)
    (hSide : parseVerdictSide verdictFor = some VerdictSide.defendant)
    (hNonNeg : ¬ damages < 0)
    (hNonZero : damages != 0.0) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error "damages must be zero on defendant verdict" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate, hHung, hSide, hNonNeg, hNonZero]

theorem validateRecordJuryVerdict_missing_jury_configuration
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (side : VerdictSide)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = false)
    (hSide : parseVerdictSide verdictFor = some side)
    (hNonNeg : ¬ damages < 0)
    (hDefCheck : (side = VerdictSide.defendant && damages != 0.0) = false)
    (hCfg : c.jury_configuration = none) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error "jury configuration required before verdict" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate, hHung, hSide, hNonNeg, hDefCheck, hCfg]

theorem validateRecordJuryVerdict_insufficient_votes
    (c : CaseState)
    (verdictFor : String)
    (votes required : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (side : VerdictSide)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = false)
    (hSide : parseVerdictSide verdictFor = some side)
    (hNonNeg : ¬ damages < 0)
    (hDefCheck : (side = VerdictSide.defendant && damages != 0.0) = false)
    (hCfg : c.jury_configuration = some { juror_count := 6, unanimous_required := true, minimum_concurring := required })
    (hVotes : votes < required) :
    validateRecordJuryVerdict c verdictFor votes damages =
      .error "insufficient concurring votes for verdict" := by
  unfold validateRecordJuryVerdict
  simp [hPhase, hGate, hHung, hSide, hNonNeg, hDefCheck, hCfg, hVotes]

theorem validateRecordJuryVerdict_ok
    (c : CaseState)
    (verdictFor : String)
    (votes required : Nat)
    (damages : Float)
    (currentPhase : TrialPhaseV1)
    (side : VerdictSide)
    (hPhase : parseCurrentPhaseV1 c = .ok currentPhase)
    (hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true)
    (hHung : c.hung_jury.isSome = false)
    (hSide : parseVerdictSide verdictFor = some side)
    (hNonNeg : ¬ damages < 0)
    (hDefCheck : (side = VerdictSide.defendant && damages != 0.0) = false)
    (hCfg : c.jury_configuration = some { juror_count := 6, unanimous_required := true, minimum_concurring := required })
    (hVotes : required ≤ votes) :
    validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required) := by
  unfold validateRecordJuryVerdict
  have hNotLt : ¬ votes < required := Nat.not_lt.mpr hVotes
  simp [hPhase, hGate, hHung, hSide, hNonNeg, hDefCheck, hCfg, hNotLt]

theorem validateRecordJuryVerdict_ok_implies_no_hung
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    c.hung_jury.isSome = false := by
  unfold validateRecordJuryVerdict at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      by_cases hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = false
      · simp [hPhase, hGate] at hOk
      · by_cases hHung : c.hung_jury.isSome
        · simp [hPhase, hGate, hHung] at hOk
        · simp [hHung]

theorem validateRecordJuryVerdict_ok_implies_has_jury_configuration
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    ∃ cfg : JuryConfiguration, c.jury_configuration = some cfg ∧ required = cfg.minimum_concurring := by
  unfold validateRecordJuryVerdict at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      by_cases hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = false
      · simp [hPhase, hGate] at hOk
      · by_cases hHung : c.hung_jury.isSome
        · simp [hPhase, hGate, hHung] at hOk
        · cases hSide : parseVerdictSide verdictFor with
          | none =>
              simp [hPhase, hGate, hHung, hSide] at hOk
          | some verdictSide =>
              by_cases hNeg : damages < 0
              · simp [hPhase, hGate, hHung, hSide, hNeg] at hOk
              · by_cases hDefCheck : (verdictSide = VerdictSide.defendant && damages != 0.0) = true
                · simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck] at hOk
                · cases hCfg : c.jury_configuration with
                  | none =>
                      simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hCfg] at hOk
                  | some cfg =>
                      by_cases hVotesLt : votes < cfg.minimum_concurring
                      · simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hCfg, hVotesLt] at hOk
                      · have hEq : (verdictSide, cfg.minimum_concurring) = (side, required) := by
                          simpa [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hCfg, hVotesLt] using hOk
                        have hReq : required = cfg.minimum_concurring := by
                          exact congrArg Prod.snd hEq.symm
                        exact ⟨cfg, rfl, hReq⟩

theorem validateRecordJuryVerdict_ok_implies_votes_meet_required
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    required ≤ votes := by
  have hCfg :
      ∃ cfg : JuryConfiguration, c.jury_configuration = some cfg ∧ required = cfg.minimum_concurring :=
    validateRecordJuryVerdict_ok_implies_has_jury_configuration c verdictFor votes damages side required hOk
  rcases hCfg with ⟨cfg, hCfgEq, hReqEq⟩
  unfold validateRecordJuryVerdict at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      by_cases hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = false
      · simp [hPhase, hGate] at hOk
      · by_cases hHung : c.hung_jury.isSome
        · simp [hPhase, hGate, hHung] at hOk
        · cases hSide : parseVerdictSide verdictFor with
          | none =>
              simp [hPhase, hGate, hHung, hSide] at hOk
          | some verdictSide =>
              by_cases hNeg : damages < 0
              · simp [hPhase, hGate, hHung, hSide, hNeg] at hOk
              · by_cases hDefCheck : (verdictSide = VerdictSide.defendant && damages != 0.0) = true
                · simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck] at hOk
                · rw [hCfgEq] at hOk
                  by_cases hVotesLt : votes < cfg.minimum_concurring
                  · simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hVotesLt] at hOk
                  · have hNotLt : ¬ votes < cfg.minimum_concurring := by
                      simpa using hVotesLt
                    have hLeCfg : cfg.minimum_concurring ≤ votes := Nat.not_lt.mp hNotLt
                    simpa [hReqEq] using hLeCfg

theorem validateRecordJuryVerdict_ok_implies_phase_parse_success
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    ∃ currentPhase : TrialPhaseV1, parseCurrentPhaseV1 c = .ok currentPhase := by
  unfold validateRecordJuryVerdict at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      exact ⟨currentPhase, rfl⟩

theorem validateRecordJuryVerdict_ok_implies_phase_gate_true
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    ∃ currentPhase : TrialPhaseV1,
      parseCurrentPhaseV1 c = .ok currentPhase ∧
      phaseAllowsActionV1 .recordJuryVerdict currentPhase = true := by
  rcases
      validateRecordJuryVerdict_ok_implies_phase_parse_success
        c verdictFor votes damages side required hOk
    with ⟨currentPhase, hPhase⟩
  unfold validateRecordJuryVerdict at hOk
  by_cases hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = false
  · simp [hPhase, hGate] at hOk
  · have hGateTrue : phaseAllowsActionV1 .recordJuryVerdict currentPhase = true := by
      cases hGateBool : phaseAllowsActionV1 .recordJuryVerdict currentPhase with
      | false =>
          exact (hGate hGateBool).elim
      | true =>
          rfl
    exact ⟨currentPhase, hPhase, hGateTrue⟩

theorem validateRecordJuryVerdict_ok_implies_phase_is_verdict_return
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    ∃ currentPhase : TrialPhaseV1,
      parseCurrentPhaseV1 c = .ok currentPhase ∧
      currentPhase = TrialPhaseV1.verdictReturn := by
  rcases
      validateRecordJuryVerdict_ok_implies_phase_gate_true
        c verdictFor votes damages side required hOk
    with ⟨currentPhase, hPhase, hGate⟩
  cases currentPhase <;> simp [phaseAllowsActionV1] at hGate
  · exact ⟨TrialPhaseV1.verdictReturn, hPhase, rfl⟩

theorem validateRecordJuryVerdict_ok_implies_side_parses
    (c : CaseState)
    (verdictFor : String)
    (votes : Nat)
    (damages : Float)
    (side : VerdictSide)
    (required : Nat)
    (hOk : validateRecordJuryVerdict c verdictFor votes damages = .ok (side, required)) :
    parseVerdictSide verdictFor = some side := by
  unfold validateRecordJuryVerdict at hOk
  cases hPhase : parseCurrentPhaseV1 c with
  | error e =>
      simp [hPhase] at hOk
  | ok currentPhase =>
      by_cases hGate : phaseAllowsActionV1 .recordJuryVerdict currentPhase = false
      · simp [hPhase, hGate] at hOk
      · by_cases hHung : c.hung_jury.isSome
        · simp [hPhase, hGate, hHung] at hOk
        · cases hSide : parseVerdictSide verdictFor with
          | none =>
              simp [hPhase, hGate, hHung, hSide] at hOk
          | some parsedSide =>
              by_cases hNeg : damages < 0
              · simp [hPhase, hGate, hHung, hSide, hNeg] at hOk
              · by_cases hDefCheck : (parsedSide = VerdictSide.defendant && damages != 0.0) = true
                · simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck] at hOk
                · cases hCfg : c.jury_configuration with
                  | none =>
                      simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hCfg] at hOk
                  | some cfg =>
                      by_cases hVotesLt : votes < cfg.minimum_concurring
                      · simp [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hCfg, hVotesLt] at hOk
                      · have hEq : (parsedSide, cfg.minimum_concurring) = (side, required) := by
                          simpa [hPhase, hGate, hHung, hSide, hNeg, hDefCheck, hCfg, hVotesLt] using hOk
                        have hSideEq : parsedSide = side := congrArg Prod.fst hEq
                        cases hSideEq
                        rfl
