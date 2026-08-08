import Proofs.OutcomeSoundness

namespace ArbProofs

theorem voteCountFor_perm
    {votes1 votes2 : List CouncilVote}
    (value : String)
    (hPerm : List.Perm votes1 votes2) :
    voteCountFor votes1 value = voteCountFor votes2 value := by
  let countStep := fun acc (vote : CouncilVote) =>
    if trimString vote.vote = value then acc + 1 else acc
  have hComm :
      ∀ x ∈ votes1, ∀ y ∈ votes1, ∀ z,
        countStep (countStep z x) y = countStep (countStep z y) x := by
    intro x _hx y _hy z
    by_cases hx : trimString x.vote = value
    · by_cases hy : trimString y.vote = value
      · simp [countStep, hx, hy, Nat.add_comm]
      · simp [countStep, hx, hy]
    · by_cases hy : trimString y.vote = value
      · simp [countStep, hx, hy]
      · simp [countStep, hx, hy]
  simpa [voteCountFor, countStep] using
    (List.Perm.foldl_eq' hPerm hComm 0)

theorem currentResolution_eq_of_currentRoundVotes_perm
    (c d : ArbitrationCase)
    (requiredVotes : Nat)
    (hVotes : List.Perm (currentRoundVotes c) (currentRoundVotes d)) :
    currentResolution? c requiredVotes = currentResolution? d requiredVotes := by
  have hDemonstrated :
      voteCountFor (currentRoundVotes c) "demonstrated" =
        voteCountFor (currentRoundVotes d) "demonstrated" :=
    voteCountFor_perm "demonstrated" hVotes
  have hNotDemonstrated :
      voteCountFor (currentRoundVotes c) "not_demonstrated" =
        voteCountFor (currentRoundVotes d) "not_demonstrated" :=
    voteCountFor_perm "not_demonstrated" hVotes
  simp [currentResolution?, hDemonstrated, hNotDemonstrated]

theorem deliberationSummaryForCase_eq_of_currentRoundVotes_perm
    (c d : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hVotes : List.Perm (currentRoundVotes c) (currentRoundVotes d))
    (hSeated : seatedCouncilMemberCount c = seatedCouncilMemberCount d)
    (hRound : c.deliberation_round = d.deliberation_round) :
    deliberationSummaryForCase c requiredVotes maxRounds =
      deliberationSummaryForCase d requiredVotes maxRounds := by
  apply DeliberationSummary.ext
  · rfl
  · simpa [deliberationSummaryForCase] using hSeated
  · simpa [deliberationSummaryForCase] using hVotes.length_eq
  · simpa [deliberationSummaryForCase] using
      voteCountFor_perm "demonstrated" hVotes
  · simpa [deliberationSummaryForCase] using
      voteCountFor_perm "not_demonstrated" hVotes
  · simpa [deliberationSummaryForCase] using hRound
  · rfl

theorem deliberationSummaryForCase_closedResolution_eq_of_currentRoundVotes_perm
    (c d : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hVotes : List.Perm (currentRoundVotes c) (currentRoundVotes d))
    (hSeated : seatedCouncilMemberCount c = seatedCouncilMemberCount d)
    (hRound : c.deliberation_round = d.deliberation_round) :
    (deliberationSummaryForCase c requiredVotes maxRounds).closedResolution? =
      (deliberationSummaryForCase d requiredVotes maxRounds).closedResolution? := by
  rw [deliberationSummaryForCase_eq_of_currentRoundVotes_perm
    c d requiredVotes maxRounds hVotes hSeated hRound]

theorem continueDeliberation_closes_same_resolution_of_currentRoundVotes_perm
    (s : ArbitrationState)
    (c d : ArbitrationCase)
    (resolution : String)
    (hVotes : List.Perm (currentRoundVotes c) (currentRoundVotes d))
    (hSeated : seatedCouncilMemberCount c = seatedCouncilMemberCount d)
    (hRound : c.deliberation_round = d.deliberation_round)
    (hClosed :
      (deliberationSummaryForCase
        c
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds).closedResolution? = some resolution) :
    continueDeliberation s d =
      .ok (stateWithCase s { d with status := "closed", phase := "closed", resolution := resolution }) := by
  have hClosedD :
      (deliberationSummaryForCase
        d
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds).closedResolution? = some resolution := by
    have hEq :=
      deliberationSummaryForCase_closedResolution_eq_of_currentRoundVotes_perm
        c
        d
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds
        hVotes
        hSeated
        hRound
    simpa [hEq] using hClosed
  exact continueDeliberation_closed_of_summary_closedResolution s d resolution hClosedD

end ArbProofs
