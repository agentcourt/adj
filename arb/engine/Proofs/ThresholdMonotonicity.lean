import Proofs.ViableOutcomesCore

namespace ArbProofs

theorem currentResolution_demonstrated_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher : Nat}
    (hLe : lower ≤ higher)
    (hResolution : currentResolution? c higher = some "demonstrated") :
    currentResolution? c lower = some "demonstrated" := by
  have hHigh :
      voteCountFor (currentRoundVotes c) "demonstrated" ≥ higher :=
    currentResolution_demonstrated_implies_sound c higher hResolution
  have hLower :
      voteCountFor (currentRoundVotes c) "demonstrated" ≥ lower :=
    Nat.le_trans hLe hHigh
  unfold currentResolution?
  simp [hLower]

theorem currentResolution_not_demonstrated_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher : Nat}
    (hLe : lower ≤ higher)
    (hDemBelow :
      voteCountFor (currentRoundVotes c) "demonstrated" < lower)
    (hResolution : currentResolution? c higher = some "not_demonstrated") :
    currentResolution? c lower = some "not_demonstrated" := by
  have hHigh :
      voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ higher :=
    currentResolution_not_demonstrated_implies_sound c higher hResolution
  have hLowerNot :
      voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ lower :=
    Nat.le_trans hLe hHigh
  have hLowerDem :
      ¬ voteCountFor (currentRoundVotes c) "demonstrated" ≥ lower :=
    Nat.not_le.mpr hDemBelow
  unfold currentResolution?
  simp [hLowerDem, hLowerNot]

theorem currentResolution_none_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher : Nat}
    (hLe : lower ≤ higher)
    (hResolution : currentResolution? c lower = none) :
    currentResolution? c higher = none := by
  have hBelow := currentResolution_none_implies_below_threshold c lower hResolution
  have hDemBelow :
      voteCountFor (currentRoundVotes c) "demonstrated" < higher :=
    Nat.lt_of_lt_of_le hBelow.1 hLe
  have hNotBelow :
      voteCountFor (currentRoundVotes c) "not_demonstrated" < higher :=
    Nat.lt_of_lt_of_le hBelow.2 hLe
  have hDemFalse :
      ¬ voteCountFor (currentRoundVotes c) "demonstrated" ≥ higher :=
    Nat.not_le.mpr hDemBelow
  have hNotFalse :
      ¬ voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ higher :=
    Nat.not_le.mpr hNotBelow
  unfold currentResolution?
  simp [hDemFalse, hNotFalse]

theorem deliberationSummaryForCase_noSubstantiveOutcomeViable_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher maxRounds : Nat}
    (hLe : lower ≤ higher)
    (hNoViable :
      (deliberationSummaryForCase c lower maxRounds).noSubstantiveOutcomeViable) :
    (deliberationSummaryForCase c higher maxRounds).noSubstantiveOutcomeViable := by
  constructor
  · intro hHighViable
    have hHigh :
        higher ≤
          voteCountFor (currentRoundVotes c) "demonstrated" +
            (seatedCouncilMemberCount c - (currentRoundVotes c).length) := by
      simpa [deliberationSummaryForCase, DeliberationSummary.demonstratedViable,
        DeliberationSummary.uncast_vote_count] using hHighViable
    have hLow :
        lower ≤
          voteCountFor (currentRoundVotes c) "demonstrated" +
            (seatedCouncilMemberCount c - (currentRoundVotes c).length) :=
      Nat.le_trans hLe hHigh
    exact hNoViable.1 <| by
      simpa [deliberationSummaryForCase, DeliberationSummary.demonstratedViable,
        DeliberationSummary.uncast_vote_count] using hLow
  · intro hHighViable
    have hHigh :
        higher ≤
          voteCountFor (currentRoundVotes c) "not_demonstrated" +
            (seatedCouncilMemberCount c - (currentRoundVotes c).length) := by
      simpa [deliberationSummaryForCase, DeliberationSummary.notDemonstratedViable,
        DeliberationSummary.uncast_vote_count] using hHighViable
    have hLow :
        lower ≤
          voteCountFor (currentRoundVotes c) "not_demonstrated" +
            (seatedCouncilMemberCount c - (currentRoundVotes c).length) :=
      Nat.le_trans hLe hHigh
    exact hNoViable.2 <| by
      simpa [deliberationSummaryForCase, DeliberationSummary.notDemonstratedViable,
        DeliberationSummary.uncast_vote_count] using hLow

theorem deliberationSummaryForCase_noMajorityClosureReason_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher maxRounds : Nat}
    (hLe : lower ≤ higher)
    (hReason :
      (deliberationSummaryForCase c lower maxRounds).noMajorityClosureReason) :
    (deliberationSummaryForCase c higher maxRounds).noMajorityClosureReason := by
  rcases hReason with hTooFew | hLastRound
  · left
    have hTooFewLower : seatedCouncilMemberCount c < lower := by
      simpa [deliberationSummaryForCase,
        DeliberationSummary.noMajorityClosureReason] using hTooFew
    have hTooFewHigher : seatedCouncilMemberCount c < higher :=
      Nat.lt_of_lt_of_le hTooFewLower hLe
    simpa [deliberationSummaryForCase,
      DeliberationSummary.noMajorityClosureReason] using hTooFewHigher
  · right
    rcases hLastRound with ⟨hRoundComplete, hLastRound⟩
    constructor
    · simpa [deliberationSummaryForCase] using hRoundComplete
    · simpa [deliberationSummaryForCase] using hLastRound

theorem deliberationSummaryForCase_noMajorityClosure_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher maxRounds : Nat}
    (hLe : lower ≤ higher)
    (hClosure :
      (deliberationSummaryForCase c lower maxRounds).noMajorityClosure) :
    (deliberationSummaryForCase c higher maxRounds).noMajorityClosure := by
  rcases hClosure with ⟨hRoundComplete, hNoViable, hReason⟩
  exact ⟨by simpa [deliberationSummaryForCase] using hRoundComplete,
    deliberationSummaryForCase_noSubstantiveOutcomeViable_of_requiredVotes_le
      c hLe hNoViable,
    deliberationSummaryForCase_noMajorityClosureReason_of_requiredVotes_le
      c hLe hReason⟩

theorem deliberationSummaryForCase_closedResolution_no_majority_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher maxRounds : Nat}
    (hLe : lower ≤ higher)
    (hClosed :
      (deliberationSummaryForCase c lower maxRounds).closedResolution? =
        some "no_majority") :
    (deliberationSummaryForCase c higher maxRounds).closedResolution? =
      some "no_majority" := by
  have hClosureLow :
      (deliberationSummaryForCase c lower maxRounds).noMajorityClosure :=
    (deliberationSummaryForCase c lower maxRounds).noMajorityClosure_of_closedResolution_no_majority
      hClosed
  exact
    (deliberationSummaryForCase c higher maxRounds).closedResolution_eq_no_majority_of_noMajorityClosure
      (deliberationSummaryForCase_noMajorityClosure_of_requiredVotes_le
        c hLe hClosureLow)

end ArbProofs
