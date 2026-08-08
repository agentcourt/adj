import Proofs.ReachableInvariants

namespace ArbProofs

theorem bilateralComplete_filingCount_eq_one
    (phase : String)
    (filings : List Filing)
    (hComplete : bilateralComplete phase filings) :
    filingCount filings "plaintiff" = 1 ∧
      filingCount filings "defendant" = 1 := by
  cases filings with
  | nil =>
      simp [bilateralComplete] at hComplete
  | cons first rest =>
      cases rest with
      | nil =>
          simp [bilateralComplete] at hComplete
      | cons second tail =>
          cases tail with
          | nil =>
              simp [bilateralComplete, filingCount] at hComplete ⊢
              rcases hComplete with ⟨_hFirstPhase, hFirstRole,
                _hSecondPhase, hSecondRole⟩
              simp [hFirstRole, hSecondRole]
          | cons third tail =>
              simp [bilateralComplete] at hComplete

theorem phaseShape_closed_merits_complete
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hClosed : c.phase = "closed") :
    bilateralComplete "openings" c.openings ∧
      bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
          defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
            bilateralComplete "closings" c.closings := by
  simpa [phaseShape, hClosed] using hShape

theorem reachable_closed_merits_complete
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed") :
    bilateralComplete "openings" s.case.openings ∧
      bilateralComplete "arguments" s.case.arguments ∧
        plaintiffOptionalSequence "rebuttals" s.case.rebuttals ∧
          defendantOptionalSequence "surrebuttals" s.case.surrebuttals ∧
            bilateralComplete "closings" s.case.closings := by
  exact phaseShape_closed_merits_complete s.case
    (reachable_phaseShape s hs)
    hClosed

theorem reachable_closed_filing_counts
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed") :
    filingCount s.case.openings "plaintiff" = 1 ∧
      filingCount s.case.openings "defendant" = 1 ∧
        filingCount s.case.arguments "plaintiff" = 1 ∧
          filingCount s.case.arguments "defendant" = 1 ∧
            filingCount s.case.closings "plaintiff" = 1 ∧
              filingCount s.case.closings "defendant" = 1 ∧
                filingCount s.case.rebuttals "plaintiff" ≤ 1 ∧
                  filingCount s.case.rebuttals "defendant" = 0 ∧
                    filingCount s.case.surrebuttals "plaintiff" = 0 ∧
                      filingCount s.case.surrebuttals "defendant" ≤ 1 := by
  rcases reachable_closed_merits_complete s hs hClosed with
    ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  rcases bilateralComplete_filingCount_eq_one "openings" s.case.openings hOpenings with
    ⟨hOpenPlaintiff, hOpenDefendant⟩
  rcases bilateralComplete_filingCount_eq_one "arguments" s.case.arguments hArguments with
    ⟨hArgPlaintiff, hArgDefendant⟩
  rcases bilateralComplete_filingCount_eq_one "closings" s.case.closings hClosings with
    ⟨hClosePlaintiff, hCloseDefendant⟩
  rcases plaintiffOptionalSequence_implies_optional_parity
      "rebuttals" s.case.rebuttals hRebuttals with
    ⟨hRebPlaintiff, hRebDefendant⟩
  rcases defendantOptionalSequence_implies_optional_parity
      "surrebuttals" s.case.surrebuttals hSurrebuttals with
    ⟨hSurPlaintiff, hSurDefendant⟩
  exact ⟨hOpenPlaintiff, hOpenDefendant, hArgPlaintiff, hArgDefendant,
    hClosePlaintiff, hCloseDefendant, hRebPlaintiff, hRebDefendant,
    hSurPlaintiff, hSurDefendant⟩

end ArbProofs
