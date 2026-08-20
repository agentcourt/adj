import Proofs.DecisionSummary
import Proofs.DecisionRuleFacts
import Proofs.RecordIntegrity

namespace ArbProofs

structure ClosedCertificateFacts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState) : Prop where
  replay_exact :
    replayInitialized req actions = .ok claimed
  authority_conforming :
    ∃ start,
      initializeCase req = .ok start ∧
        AuthorityConformingReplay start actions
  merits_offer_chronology :
    InitializedMeritsOfferChronology req actions
  reachable :
    Reachable claimed
  record_integrity :
    RecordIntegrity claimed
  evidence_catalog_fixed :
    claimed.evidence_catalog = req.state.evidence_catalog
  length_bound :
    ∃ start,
      initializeCase req = .ok start ∧
        actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
          8 + start.policy.max_deliberation_rounds * start.policy.council_size
  terminal_accounted :
    claimed.case.status = "closed" ∧
      claimed.case.phase = "closed" ∧
        (claimed.case.resolution = "demonstrated" ∨
          claimed.case.resolution = "not_demonstrated" ∨
            claimed.case.resolution = "no_majority")
  outcome_sound :
    (claimed.case.resolution = "demonstrated" ∧ demonstratedOutcomeSound claimed) ∨
      (claimed.case.resolution = "not_demonstrated" ∧ notDemonstratedOutcomeSound claimed) ∨
        (claimed.case.resolution = "no_majority" ∧ noMajorityOutcomeSound claimed)
  decision_rule :
    DecisionRuleFacts claimed
  merits_complete :
    bilateralComplete "openings" claimed.case.openings ∧
      bilateralComplete "arguments" claimed.case.arguments ∧
        plaintiffOptionalSequence "rebuttals" claimed.case.rebuttals ∧
          defendantOptionalSequence "surrebuttals" claimed.case.surrebuttals ∧
            bilateralComplete "closings" claimed.case.closings
  filing_counts :
    filingCount claimed.case.openings "plaintiff" = 1 ∧
      filingCount claimed.case.openings "defendant" = 1 ∧
        filingCount claimed.case.arguments "plaintiff" = 1 ∧
          filingCount claimed.case.arguments "defendant" = 1 ∧
            filingCount claimed.case.closings "plaintiff" = 1 ∧
              filingCount claimed.case.closings "defendant" = 1 ∧
                filingCount claimed.case.rebuttals "plaintiff" ≤ 1 ∧
                  filingCount claimed.case.rebuttals "defendant" = 0 ∧
                    filingCount claimed.case.surrebuttals "plaintiff" = 0 ∧
                      filingCount claimed.case.surrebuttals "defendant" ≤ 1
  decision_summary_replayed :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        decisionSummary replayed = decisionSummary claimed

structure FailedCertificateFacts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState) : Prop where
  replay_exact :
    replayInitialized req actions = .ok claimed
  authority_conforming :
    ∃ start,
      initializeCase req = .ok start ∧
        AuthorityConformingReplay start actions
  merits_offer_chronology :
    InitializedMeritsOfferChronology req actions
  reachable :
    Reachable claimed
  record_integrity :
    RecordIntegrity claimed
  evidence_catalog_fixed :
    claimed.evidence_catalog = req.state.evidence_catalog
  length_bound :
    ∃ start,
      initializeCase req = .ok start ∧
        actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
          8 + start.policy.max_deliberation_rounds * start.policy.council_size
  status_failed :
    claimed.case.status = "failed"
  failure_record :
    ∃ failure,
      claimed.case.failure = some failure ∧
        failure.failure_type = "opportunity_failed" ∧
          (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
            failure.phase = claimed.case.phase
  decision_summary_replayed :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        decisionSummary replayed = decisionSummary claimed

def TerminalCertificateFacts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState) : Prop :=
  ClosedCertificateFacts req actions claimed ∨
    FailedCertificateFacts req actions claimed

theorem checkReplayCertificate_ok_meritsOfferChronology
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    InitializedMeritsOfferChronology req actions := by
  exact replayInitialized_success_meritsOfferChronology req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck)

theorem checkReplayCertificate_ok_recordIntegrity
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    RecordIntegrity claimed ∧
      claimed.evidence_catalog = req.state.evidence_catalog := by
  constructor
  · exact reachable_recordIntegrity claimed
      (checkReplayCertificate_ok_reachable req actions claimed hCheck)
  · have hReplay : replayInitialized req actions = .ok claimed :=
      (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
    rcases replayInitialized_success_components req actions claimed hReplay with
      ⟨start, hInit, hSteps⟩
    exact initialized_run_preserves_evidenceCatalog req start claimed hInit
      (replaySteps_success_stepReachableFrom start claimed actions hSteps)

theorem checkReplayCertificate_ok_evidenceCatalog_fixed
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    claimed.evidence_catalog = req.state.evidence_catalog :=
  (checkReplayCertificate_ok_recordIntegrity req actions claimed hCheck).2

theorem checkReplayCertificate_status_closed_facts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "closed") :
    ClosedCertificateFacts req actions claimed := by
  have hReplay : replayInitialized req actions = .ok claimed :=
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
  have hReachable : Reachable claimed :=
    checkReplayCertificate_ok_reachable req actions claimed hCheck
  have hPhase : claimed.case.phase = "closed" :=
    reachable_status_closed_implies_phase_closed claimed hReachable hStatus
  have hResolution :
      claimed.case.resolution = "demonstrated" ∨
        claimed.case.resolution = "not_demonstrated" ∨
          claimed.case.resolution = "no_majority" :=
    reachable_closed_resolution_enum claimed hReachable hPhase
  have hSound :
      (claimed.case.resolution = "demonstrated" ∧ demonstratedOutcomeSound claimed) ∨
        (claimed.case.resolution = "not_demonstrated" ∧ notDemonstratedOutcomeSound claimed) ∨
          (claimed.case.resolution = "no_majority" ∧ noMajorityOutcomeSound claimed) := by
    rcases hResolution with hDemonstrated | hRest
    · exact Or.inl ⟨hDemonstrated,
        checkReplayCertificate_closed_demonstrated_sound
          req actions claimed hCheck hPhase hDemonstrated⟩
    · rcases hRest with hNotDemonstrated | hNoMajority
      · exact Or.inr (Or.inl ⟨hNotDemonstrated,
          checkReplayCertificate_closed_not_demonstrated_sound
            req actions claimed hCheck hPhase hNotDemonstrated⟩)
      · exact Or.inr (Or.inr ⟨hNoMajority,
          checkReplayCertificate_closed_no_majority_sound
            req actions claimed hCheck hPhase hNoMajority⟩)
  exact
    { replay_exact := hReplay
      authority_conforming :=
        checkReplayCertificate_ok_authorityConforming
          req actions claimed hCheck
      merits_offer_chronology :=
        checkReplayCertificate_ok_meritsOfferChronology
          req actions claimed hCheck
      reachable := hReachable
      record_integrity :=
        (checkReplayCertificate_ok_recordIntegrity req actions claimed hCheck).1
      evidence_catalog_fixed :=
        checkReplayCertificate_ok_evidenceCatalog_fixed req actions claimed hCheck
      length_bound :=
        checkReplayCertificate_ok_length_le_initializedBudget
          req actions claimed hCheck
      terminal_accounted := ⟨hStatus, hPhase, hResolution⟩
      outcome_sound := hSound
      decision_rule :=
        reachable_decisionRuleFacts claimed hReachable
      merits_complete :=
        checkReplayCertificate_status_closed_merits_complete
          req actions claimed hCheck hStatus
      filing_counts :=
        checkReplayCertificate_status_closed_filing_counts
          req actions claimed hCheck hStatus
      decision_summary_replayed :=
        checkReplayCertificate_ok_decisionSummary_replayed
          req actions claimed hCheck }

theorem checkReplayCertificate_status_failed_facts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "failed") :
    FailedCertificateFacts req actions claimed := by
  have hReplay : replayInitialized req actions = .ok claimed :=
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
  have hReachable : Reachable claimed :=
    checkReplayCertificate_ok_reachable req actions claimed hCheck
  exact
    { replay_exact := hReplay
      authority_conforming :=
        checkReplayCertificate_ok_authorityConforming
          req actions claimed hCheck
      merits_offer_chronology :=
        checkReplayCertificate_ok_meritsOfferChronology
          req actions claimed hCheck
      reachable := hReachable
      record_integrity :=
        (checkReplayCertificate_ok_recordIntegrity req actions claimed hCheck).1
      evidence_catalog_fixed :=
        checkReplayCertificate_ok_evidenceCatalog_fixed req actions claimed hCheck
      length_bound :=
        checkReplayCertificate_ok_length_le_initializedBudget
          req actions claimed hCheck
      status_failed := hStatus
      failure_record :=
        reachable_failed_has_failure claimed hReachable hStatus
      decision_summary_replayed :=
        checkReplayCertificate_ok_decisionSummary_replayed
          req actions claimed hCheck }

theorem checkReplayCertificate_terminal_facts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hTerminal :
      claimed.case.status = "closed" ∨
        claimed.case.status = "failed") :
    TerminalCertificateFacts req actions claimed := by
  rcases hTerminal with hClosed | hFailed
  · exact Or.inl
      (checkReplayCertificate_status_closed_facts req actions claimed hCheck hClosed)
  · exact Or.inr
      (checkReplayCertificate_status_failed_facts req actions claimed hCheck hFailed)

theorem ClosedCertificateFacts.demonstrated_sound
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (hResolution : claimed.case.resolution = "demonstrated") :
    demonstratedOutcomeSound claimed := by
  rcases facts.outcome_sound with hDemonstrated | hRest
  · exact hDemonstrated.2
  · rcases hRest with hNotDemonstrated | hNoMajority
    · exact False.elim <| by
        simpa [hResolution] using hNotDemonstrated.1
    · exact False.elim <| by
        simpa [hResolution] using hNoMajority.1

theorem ClosedCertificateFacts.decision_rule_facts
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed) :
    DecisionRuleFacts claimed :=
  facts.decision_rule

theorem ClosedCertificateFacts.closed_resolution_agrees_with_matched_case
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hVotes :
      List.Perm
        (currentRoundVotes claimed.case)
        (currentRoundVotes c))
    (hSeated :
      seatedCouncilMemberCount claimed.case =
        seatedCouncilMemberCount c)
    (hRound :
      claimed.case.deliberation_round =
        c.deliberation_round) :
    (deliberationSummaryForCase
      claimed.case
      requiredVotes
      maxRounds).closedResolution? =
      (deliberationSummaryForCase
        c
        requiredVotes
        maxRounds).closedResolution? :=
  facts.decision_rule.closed_resolution_anonymous
    c requiredVotes maxRounds hVotes hSeated hRound

theorem ClosedCertificateFacts.not_demonstrated_sound
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (hResolution : claimed.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound claimed := by
  rcases facts.outcome_sound with hDemonstrated | hRest
  · exact False.elim <| by
      simpa [hResolution] using hDemonstrated.1
  · rcases hRest with hNotDemonstrated | hNoMajority
    · exact hNotDemonstrated.2
    · exact False.elim <| by
        simpa [hResolution] using hNoMajority.1

theorem ClosedCertificateFacts.no_majority_sound
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (hResolution : claimed.case.resolution = "no_majority") :
    noMajorityOutcomeSound claimed := by
  rcases facts.outcome_sound with hDemonstrated | hRest
  · exact False.elim <| by
      simpa [hResolution] using hDemonstrated.1
  · rcases hRest with hNotDemonstrated | hNoMajority
    · exact False.elim <| by
        simpa [hResolution] using hNotDemonstrated.1
    · exact hNoMajority.2

end ArbProofs
