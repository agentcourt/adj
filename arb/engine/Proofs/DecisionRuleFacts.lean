import Proofs.Neutrality
import Proofs.ThresholdMonotonicity
import Proofs.VoteOrder

namespace ArbProofs

structure DecisionRuleFacts (s : ArbitrationState) : Prop where
  current_resolution_anonymous :
    ∀ (c : ArbitrationCase) (requiredVotes : Nat),
      List.Perm (currentRoundVotes s.case) (currentRoundVotes c) →
        currentResolution? s.case requiredVotes = currentResolution? c requiredVotes
  closed_resolution_anonymous :
    ∀ (c : ArbitrationCase) (requiredVotes maxRounds : Nat),
      List.Perm (currentRoundVotes s.case) (currentRoundVotes c) →
        seatedCouncilMemberCount s.case = seatedCouncilMemberCount c →
          s.case.deliberation_round = c.deliberation_round →
            (deliberationSummaryForCase s.case requiredVotes maxRounds).closedResolution? =
              (deliberationSummaryForCase c requiredVotes maxRounds).closedResolution?
  current_resolution_neutral :
    currentResolution? (flipCaseVotes s.case) s.policy.required_votes_for_decision =
      flipResolution (currentResolution? s.case s.policy.required_votes_for_decision)
  demonstrated_quota_monotone :
    ∀ {lower higher : Nat},
      lower ≤ higher →
        currentResolution? s.case higher = some "demonstrated" →
          currentResolution? s.case lower = some "demonstrated"
  not_demonstrated_quota_monotone :
    ∀ {lower higher : Nat},
      lower ≤ higher →
        voteCountFor (currentRoundVotes s.case) "demonstrated" < lower →
          currentResolution? s.case higher = some "not_demonstrated" →
            currentResolution? s.case lower = some "not_demonstrated"
  none_quota_monotone :
    ∀ {lower higher : Nat},
      lower ≤ higher →
        currentResolution? s.case lower = none →
          currentResolution? s.case higher = none
  no_majority_quota_monotone :
    ∀ {lower higher maxRounds : Nat},
      lower ≤ higher →
        (deliberationSummaryForCase s.case lower maxRounds).closedResolution? =
          some "no_majority" →
            (deliberationSummaryForCase s.case higher maxRounds).closedResolution? =
              some "no_majority"

theorem reachable_decisionRuleFacts
    (s : ArbitrationState)
    (hs : Reachable s) :
    DecisionRuleFacts s := by
  exact
    { current_resolution_anonymous := by
        intro c requiredVotes hVotes
        exact currentResolution_eq_of_currentRoundVotes_perm
          s.case c requiredVotes hVotes
      closed_resolution_anonymous := by
        intro c requiredVotes maxRounds hVotes hSeated hRound
        exact deliberationSummaryForCase_closedResolution_eq_of_currentRoundVotes_perm
          s.case c requiredVotes maxRounds hVotes hSeated hRound
      current_resolution_neutral :=
        reachable_currentResolution_is_neutral_under_vote_flip s hs
      demonstrated_quota_monotone := by
        intro lower higher hLe hResolution
        exact currentResolution_demonstrated_of_requiredVotes_le
          s.case hLe hResolution
      not_demonstrated_quota_monotone := by
        intro lower higher hLe hDemBelow hResolution
        exact currentResolution_not_demonstrated_of_requiredVotes_le
          s.case hLe hDemBelow hResolution
      none_quota_monotone := by
        intro lower higher hLe hResolution
        exact currentResolution_none_of_requiredVotes_le
          s.case hLe hResolution
      no_majority_quota_monotone := by
        intro lower higher maxRounds hLe hClosed
        exact deliberationSummaryForCase_closedResolution_no_majority_of_requiredVotes_le
          s.case hLe hClosed }

end ArbProofs
