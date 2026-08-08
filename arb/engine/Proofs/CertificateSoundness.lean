import Proofs.Replay
import Proofs.OutcomeSoundness
import Proofs.DueProcess

namespace ArbProofs

theorem checkReplayCertificate_closed_demonstrated_sound
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution : claimed.case.resolution = "demonstrated") :
    demonstratedOutcomeSound claimed := by
  exact reachable_closed_demonstrated_sound claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed
    hResolution

theorem checkReplayCertificate_closed_not_demonstrated_sound
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution : claimed.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound claimed := by
  exact reachable_closed_not_demonstrated_sound claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed
    hResolution

theorem checkReplayCertificate_closed_no_majority_sound
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution : claimed.case.resolution = "no_majority") :
    noMajorityOutcomeSound claimed := by
  exact reachable_closed_no_majority_sound claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed
    hResolution

theorem checkReplayCertificate_closed_substantive_threshold
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution :
      claimed.case.resolution = "demonstrated" ∨
        claimed.case.resolution = "not_demonstrated") :
    (claimed.case.resolution = "demonstrated" ∧ demonstratedOutcomeSound claimed) ∨
      (claimed.case.resolution = "not_demonstrated" ∧ notDemonstratedOutcomeSound claimed) := by
  rcases hResolution with hDemonstrated | hNotDemonstrated
  · exact Or.inl ⟨hDemonstrated,
      checkReplayCertificate_closed_demonstrated_sound
        req actions claimed hCheck hClosed hDemonstrated⟩
  · exact Or.inr ⟨hNotDemonstrated,
      checkReplayCertificate_closed_not_demonstrated_sound
        req actions claimed hCheck hClosed hNotDemonstrated⟩

theorem checkReplayCertificate_closed_filing_counts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed") :
    filingCount claimed.case.openings "plaintiff" = 1 ∧
      filingCount claimed.case.openings "defendant" = 1 ∧
        filingCount claimed.case.arguments "plaintiff" = 1 ∧
          filingCount claimed.case.arguments "defendant" = 1 ∧
            filingCount claimed.case.closings "plaintiff" = 1 ∧
              filingCount claimed.case.closings "defendant" = 1 ∧
                filingCount claimed.case.rebuttals "plaintiff" ≤ 1 ∧
                  filingCount claimed.case.rebuttals "defendant" = 0 ∧
                    filingCount claimed.case.surrebuttals "plaintiff" = 0 ∧
                      filingCount claimed.case.surrebuttals "defendant" ≤ 1 := by
  exact reachable_closed_filing_counts claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed

theorem checkReplayCertificate_closed_merits_complete
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed") :
    bilateralComplete "openings" claimed.case.openings ∧
      bilateralComplete "arguments" claimed.case.arguments ∧
        plaintiffOptionalSequence "rebuttals" claimed.case.rebuttals ∧
          defendantOptionalSequence "surrebuttals" claimed.case.surrebuttals ∧
            bilateralComplete "closings" claimed.case.closings := by
  exact reachable_closed_merits_complete claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed

theorem checkReplayCertificate_status_closed_filing_counts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "closed") :
    filingCount claimed.case.openings "plaintiff" = 1 ∧
      filingCount claimed.case.openings "defendant" = 1 ∧
        filingCount claimed.case.arguments "plaintiff" = 1 ∧
          filingCount claimed.case.arguments "defendant" = 1 ∧
            filingCount claimed.case.closings "plaintiff" = 1 ∧
              filingCount claimed.case.closings "defendant" = 1 ∧
                filingCount claimed.case.rebuttals "plaintiff" ≤ 1 ∧
                  filingCount claimed.case.rebuttals "defendant" = 0 ∧
                    filingCount claimed.case.surrebuttals "plaintiff" = 0 ∧
                      filingCount claimed.case.surrebuttals "defendant" ≤ 1 := by
  have hReachable := checkReplayCertificate_ok_reachable req actions claimed hCheck
  exact checkReplayCertificate_closed_filing_counts req actions claimed hCheck
    (reachable_status_closed_implies_phase_closed claimed hReachable hStatus)

theorem checkReplayCertificate_status_closed_merits_complete
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "closed") :
    bilateralComplete "openings" claimed.case.openings ∧
      bilateralComplete "arguments" claimed.case.arguments ∧
        plaintiffOptionalSequence "rebuttals" claimed.case.rebuttals ∧
          defendantOptionalSequence "surrebuttals" claimed.case.surrebuttals ∧
            bilateralComplete "closings" claimed.case.closings := by
  have hReachable := checkReplayCertificate_ok_reachable req actions claimed hCheck
  exact checkReplayCertificate_closed_merits_complete req actions claimed hCheck
    (reachable_status_closed_implies_phase_closed claimed hReachable hStatus)

end ArbProofs
