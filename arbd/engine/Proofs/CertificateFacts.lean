import Proofs.BoundedTermination

namespace ArbdProofs

def reportedAnswerPairs (s : ArbitrationState) : List (String × Nat) :=
  s.case.council_answers.map (fun answer => (answer.member_id, answer.answer))

def reportedFailureRecord (s : ArbitrationState) : Option OpportunityFailure :=
  s.case.failure

def terminalClosedAccounted (s : ArbitrationState) : Prop :=
  s.case.status = "closed" ∧
    (nextOpportunity s).terminal = true ∧
      (nextOpportunity s).reason = "answers_complete"

def terminalFailedAccounted (s : ArbitrationState) : Prop :=
  s.case.status = "failed" ∧
    (nextOpportunity s).terminal = true

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
  step_reachable :
    ∃ start,
      initializeCase req = .ok start ∧
        StepReachableFrom start claimed
  run_invariant :
    InitializedRunInvariant req claimed
  bounded_length :
    actions.length ≤
      (req.state.policy.max_submitted_evidence_per_side +
        req.state.policy.max_submitted_evidence_per_side) + 8 + req.state.policy.council_size
  terminal_accounted :
    terminalClosedAccounted claimed
  answer_pairs_replayed :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        reportedAnswerPairs replayed = reportedAnswerPairs claimed

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
  step_reachable :
    ∃ start,
      initializeCase req = .ok start ∧
        StepReachableFrom start claimed
  run_invariant :
    InitializedRunInvariant req claimed
  bounded_length :
    actions.length ≤
      (req.state.policy.max_submitted_evidence_per_side +
        req.state.policy.max_submitted_evidence_per_side) + 8 + req.state.policy.council_size
  terminal_accounted :
    terminalFailedAccounted claimed
  failure_record_replayed :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        reportedFailureRecord replayed = reportedFailureRecord claimed

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

theorem checkReplayCertificate_ok_runInvariant
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    InitializedRunInvariant req claimed := by
  have hReplay : replayInitialized req actions = .ok claimed :=
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
  rcases replayInitialized_success_components req actions claimed hReplay with
    ⟨start, hInit, hSteps⟩
  exact initializedRun_reachable_invariant req start claimed hInit
    (replaySteps_success_stepReachableFrom start claimed actions hSteps)

theorem terminalClosedAccounted_of_status_closed
    (s : ArbitrationState)
    (hStatus : s.case.status = "closed") :
    terminalClosedAccounted s := by
  unfold terminalClosedAccounted
  simp [nextOpportunity, hStatus]

theorem terminalFailedAccounted_of_status_failed
    (s : ArbitrationState)
    (hStatus : s.case.status = "failed") :
    terminalFailedAccounted s := by
  unfold terminalFailedAccounted
  cases hFailure : s.case.failure <;> simp [nextOpportunity, hStatus, hFailure]

theorem checkReplayCertificate_status_closed_facts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "closed") :
    ClosedCertificateFacts req actions claimed := by
  have hReplay : replayInitialized req actions = .ok claimed :=
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
  exact
    { replay_exact := hReplay
      authority_conforming :=
        checkReplayCertificate_ok_authorityConforming
          req actions claimed hCheck
      merits_offer_chronology :=
        checkReplayCertificate_ok_meritsOfferChronology
          req actions claimed hCheck
      reachable :=
        checkReplayCertificate_ok_reachable req actions claimed hCheck
      record_integrity :=
        (checkReplayCertificate_ok_recordIntegrity req actions claimed hCheck).1
      evidence_catalog_fixed :=
        checkReplayCertificate_ok_evidenceCatalog_fixed req actions claimed hCheck
      step_reachable :=
        checkReplayCertificate_ok_stepReachableFrom req actions claimed hCheck
      run_invariant :=
        checkReplayCertificate_ok_runInvariant req actions claimed hCheck
      terminal_accounted :=
        terminalClosedAccounted_of_status_closed claimed hStatus
      bounded_length := checkReplayCertificate_length_bound req actions claimed hCheck
      answer_pairs_replayed := ⟨claimed, hReplay, rfl⟩ }

theorem checkReplayCertificate_status_failed_facts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "failed") :
    FailedCertificateFacts req actions claimed := by
  exact
    { replay_exact :=
        (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
      authority_conforming :=
        checkReplayCertificate_ok_authorityConforming
          req actions claimed hCheck
      merits_offer_chronology :=
        checkReplayCertificate_ok_meritsOfferChronology
          req actions claimed hCheck
      reachable :=
        checkReplayCertificate_ok_reachable req actions claimed hCheck
      record_integrity :=
        (checkReplayCertificate_ok_recordIntegrity req actions claimed hCheck).1
      evidence_catalog_fixed :=
        checkReplayCertificate_ok_evidenceCatalog_fixed req actions claimed hCheck
      step_reachable :=
        checkReplayCertificate_ok_stepReachableFrom req actions claimed hCheck
      run_invariant :=
        checkReplayCertificate_ok_runInvariant req actions claimed hCheck
      terminal_accounted :=
        terminalFailedAccounted_of_status_failed claimed hStatus
      bounded_length := checkReplayCertificate_length_bound req actions claimed hCheck
      failure_record_replayed := ⟨claimed,
        (checkReplayCertificate_ok_iff req actions claimed).1 hCheck, rfl⟩ }

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

theorem ClosedCertificateFacts.answer_pairs_replayed_eq
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed) :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        reportedAnswerPairs replayed = reportedAnswerPairs claimed :=
  facts.answer_pairs_replayed

theorem FailedCertificateFacts.failure_record_replayed_eq
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : FailedCertificateFacts req actions claimed) :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        reportedFailureRecord replayed = reportedFailureRecord claimed :=
  facts.failure_record_replayed

end ArbdProofs
