import Proofs.Replay

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
  reachable :
    Reachable claimed
  step_reachable :
    ∃ start,
      initializeCase req = .ok start ∧
        StepReachableFrom start claimed
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
  reachable :
    Reachable claimed
  step_reachable :
    ∃ start,
      initializeCase req = .ok start ∧
        StepReachableFrom start claimed
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
      reachable :=
        checkReplayCertificate_ok_reachable req actions claimed hCheck
      step_reachable :=
        checkReplayCertificate_ok_stepReachableFrom req actions claimed hCheck
      terminal_accounted :=
        terminalClosedAccounted_of_status_closed claimed hStatus
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
      reachable :=
        checkReplayCertificate_ok_reachable req actions claimed hCheck
      step_reachable :=
        checkReplayCertificate_ok_stepReachableFrom req actions claimed hCheck
      terminal_accounted :=
        terminalFailedAccounted_of_status_failed claimed hStatus
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
