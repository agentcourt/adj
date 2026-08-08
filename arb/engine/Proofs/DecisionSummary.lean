import Proofs.CertificateSoundness
import Proofs.VoteOrder

namespace ArbProofs

structure DecisionSummary where
  status : String
  phase : String
  resolution : String
  required_votes : Nat
  seated_count : Nat
  current_round_vote_count : Nat
  demonstrated_count : Nat
  not_demonstrated_count : Nat
  deliberation_round : Nat
  max_deliberation_rounds : Nat
  deriving Inhabited, DecidableEq, Repr

@[ext] theorem DecisionSummary.ext
    (d e : DecisionSummary)
    (hStatus : d.status = e.status)
    (hPhase : d.phase = e.phase)
    (hResolution : d.resolution = e.resolution)
    (hRequired : d.required_votes = e.required_votes)
    (hSeated : d.seated_count = e.seated_count)
    (hRoundVotes : d.current_round_vote_count = e.current_round_vote_count)
    (hDem : d.demonstrated_count = e.demonstrated_count)
    (hNot : d.not_demonstrated_count = e.not_demonstrated_count)
    (hRound : d.deliberation_round = e.deliberation_round)
    (hMaxRounds : d.max_deliberation_rounds = e.max_deliberation_rounds) :
    d = e := by
  cases d
  cases e
  simp at hStatus hPhase hResolution hRequired hSeated hRoundVotes hDem hNot hRound hMaxRounds
  simp [hStatus, hPhase, hResolution, hRequired, hSeated, hRoundVotes,
    hDem, hNot, hRound, hMaxRounds]

def DecisionSummary.toDeliberationSummary (d : DecisionSummary) : DeliberationSummary :=
  { required_votes := d.required_votes
    seated_count := d.seated_count
    current_round_vote_count := d.current_round_vote_count
    demonstrated_count := d.demonstrated_count
    not_demonstrated_count := d.not_demonstrated_count
    deliberation_round := d.deliberation_round
    max_deliberation_rounds := d.max_deliberation_rounds }

noncomputable def DecisionSummary.closedResolution? (d : DecisionSummary) : Option String :=
  d.toDeliberationSummary.closedResolution?

def decisionSummary (s : ArbitrationState) : DecisionSummary :=
  let d := deliberationSummary s
  { status := s.case.status
    phase := s.case.phase
    resolution := s.case.resolution
    required_votes := d.required_votes
    seated_count := d.seated_count
    current_round_vote_count := d.current_round_vote_count
    demonstrated_count := d.demonstrated_count
    not_demonstrated_count := d.not_demonstrated_count
    deliberation_round := d.deliberation_round
    max_deliberation_rounds := d.max_deliberation_rounds }

theorem decisionSummary_toDeliberationSummary
    (s : ArbitrationState) :
    (decisionSummary s).toDeliberationSummary = deliberationSummary s := by
  rfl

theorem decisionSummary_eq_of_currentRoundVotes_perm
    (s t : ArbitrationState)
    (hStatus : s.case.status = t.case.status)
    (hPhase : s.case.phase = t.case.phase)
    (hResolution : s.case.resolution = t.case.resolution)
    (hRequired :
      s.policy.required_votes_for_decision =
        t.policy.required_votes_for_decision)
    (hMaxRounds :
      s.policy.max_deliberation_rounds =
        t.policy.max_deliberation_rounds)
    (hVotes : List.Perm (currentRoundVotes s.case) (currentRoundVotes t.case))
    (hSeated : seatedCouncilMemberCount s.case = seatedCouncilMemberCount t.case)
    (hRound : s.case.deliberation_round = t.case.deliberation_round) :
    decisionSummary s = decisionSummary t := by
  apply DecisionSummary.ext
  · exact hStatus
  · exact hPhase
  · exact hResolution
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using hRequired
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using hSeated
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using hVotes.length_eq
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using
      voteCountFor_perm "demonstrated" hVotes
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using
      voteCountFor_perm "not_demonstrated" hVotes
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using hRound
  · simpa [decisionSummary, deliberationSummary, deliberationSummaryForCase] using hMaxRounds

theorem decisionSummary_closedResolution_eq_of_currentRoundVotes_perm
    (s t : ArbitrationState)
    (hStatus : s.case.status = t.case.status)
    (hPhase : s.case.phase = t.case.phase)
    (hResolution : s.case.resolution = t.case.resolution)
    (hRequired :
      s.policy.required_votes_for_decision =
        t.policy.required_votes_for_decision)
    (hMaxRounds :
      s.policy.max_deliberation_rounds =
        t.policy.max_deliberation_rounds)
    (hVotes : List.Perm (currentRoundVotes s.case) (currentRoundVotes t.case))
    (hSeated : seatedCouncilMemberCount s.case = seatedCouncilMemberCount t.case)
    (hRound : s.case.deliberation_round = t.case.deliberation_round) :
    (decisionSummary s).closedResolution? =
      (decisionSummary t).closedResolution? := by
  rw [decisionSummary_eq_of_currentRoundVotes_perm
    s t hStatus hPhase hResolution hRequired hMaxRounds hVotes hSeated hRound]

theorem checkReplayCertificate_ok_decisionSummary_replayed
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        decisionSummary replayed = decisionSummary claimed := by
  exact ⟨claimed,
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck,
    rfl⟩

theorem checkReplayCertificate_ok_decisionSummary_produced
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (summary : DecisionSummary)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hSummary : decisionSummary claimed = summary) :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        decisionSummary replayed = summary := by
  exact ⟨claimed,
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck,
    hSummary⟩

theorem checkReplayCertificate_status_closed_decisionSummary
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "closed") :
    (decisionSummary claimed).status = "closed" ∧
      (decisionSummary claimed).phase = "closed" := by
  have hReachable := checkReplayCertificate_ok_reachable req actions claimed hCheck
  have hPhase : claimed.case.phase = "closed" :=
    reachable_status_closed_implies_phase_closed claimed hReachable hStatus
  simp [decisionSummary, hStatus, hPhase]

end ArbProofs
