import Proofs.DeliberationSummaryCore

namespace ArbProofs

def DeliberationSummary.demonstratedViable (d : DeliberationSummary) : Prop :=
  d.required_votes ≤ d.demonstrated_count + d.uncast_vote_count

def DeliberationSummary.notDemonstratedViable (d : DeliberationSummary) : Prop :=
  d.required_votes ≤ d.not_demonstrated_count + d.uncast_vote_count

def DeliberationSummary.noSubstantiveOutcomeViable (d : DeliberationSummary) : Prop :=
  ¬ d.demonstratedViable ∧ ¬ d.notDemonstratedViable

def DeliberationSummary.afterDemonstratedVote (d : DeliberationSummary) : DeliberationSummary :=
  { d with
    current_round_vote_count := d.current_round_vote_count + 1
    demonstrated_count := d.demonstrated_count + 1 }

def DeliberationSummary.afterNotDemonstratedVote (d : DeliberationSummary) : DeliberationSummary :=
  { d with
    current_round_vote_count := d.current_round_vote_count + 1
    not_demonstrated_count := d.not_demonstrated_count + 1 }

def DeliberationSummary.afterSeatedMemberRemoval (d : DeliberationSummary) : DeliberationSummary :=
  { d with seated_count := d.seated_count - 1 }

def DeliberationSummary.flipSubstantiveCounts (d : DeliberationSummary) : DeliberationSummary :=
  { d with
    demonstrated_count := d.not_demonstrated_count
    not_demonstrated_count := d.demonstrated_count }

def DeliberationSummary.noMajorityClosureReason (d : DeliberationSummary) : Prop :=
  d.seated_count < d.required_votes ∨
    (d.current_round_vote_count = d.seated_count ∧
      d.deliberation_round ≥ d.max_deliberation_rounds)

def DeliberationSummary.noMajoritySound (d : DeliberationSummary) : Prop :=
  d.demonstrated_count < d.required_votes ∧
    d.not_demonstrated_count < d.required_votes ∧
    d.noMajorityClosureReason

def DeliberationSummary.noMajorityClosure (d : DeliberationSummary) : Prop :=
  d.current_round_vote_count = d.seated_count ∧
    d.noSubstantiveOutcomeViable ∧
    d.noMajorityClosureReason

noncomputable def DeliberationSummary.closedResolution? (d : DeliberationSummary) : Option String := by
  classical
  exact
    if hRoundComplete : d.current_round_vote_count = d.seated_count then
      match d.currentResolution? with
      | some resolution => some resolution
      | none =>
          if hReason : d.noMajorityClosureReason then
            some "no_majority"
          else
            none
    else
      none

theorem DeliberationSummary.demonstrated_count_lt_required_of_not_viable
    (d : DeliberationSummary)
    (hNot : ¬ d.demonstratedViable) :
    d.demonstrated_count < d.required_votes := by
  unfold DeliberationSummary.demonstratedViable at hNot
  have hLt : d.demonstrated_count + d.uncast_vote_count < d.required_votes :=
    Nat.lt_of_not_ge hNot
  exact Nat.lt_of_le_of_lt (Nat.le_add_right _ _) hLt

theorem DeliberationSummary.not_demonstrated_count_lt_required_of_not_viable
    (d : DeliberationSummary)
    (hNot : ¬ d.notDemonstratedViable) :
    d.not_demonstrated_count < d.required_votes := by
  unfold DeliberationSummary.notDemonstratedViable at hNot
  have hLt : d.not_demonstrated_count + d.uncast_vote_count < d.required_votes :=
    Nat.lt_of_not_ge hNot
  exact Nat.lt_of_le_of_lt (Nat.le_add_right _ _) hLt

theorem DeliberationSummary.currentResolution_none_of_noSubstantiveOutcomeViable
    (d : DeliberationSummary)
    (hNone : d.noSubstantiveOutcomeViable) :
    d.currentResolution? = none := by
  rcases hNone with ⟨hDem, hNot⟩
  have hDemLt := d.demonstrated_count_lt_required_of_not_viable hDem
  have hNotLt := d.not_demonstrated_count_lt_required_of_not_viable hNot
  have hDemFalse : ¬ d.demonstrated_count ≥ d.required_votes :=
    Nat.not_le.mpr hDemLt
  have hNotFalse : ¬ d.not_demonstrated_count ≥ d.required_votes :=
    Nat.not_le.mpr hNotLt
  simp [DeliberationSummary.currentResolution?, hDemFalse, hNotFalse]

theorem DeliberationSummary.demonstratedViable_of_currentResolution
    (d : DeliberationSummary)
    (hResolution : d.currentResolution? = some "demonstrated") :
    d.demonstratedViable := by
  unfold DeliberationSummary.currentResolution? at hResolution
  by_cases hDem : d.demonstrated_count ≥ d.required_votes
  · unfold DeliberationSummary.demonstratedViable
    exact Nat.le_trans hDem (Nat.le_add_right _ _)
  · simp [hDem] at hResolution

theorem DeliberationSummary.notDemonstratedViable_of_currentResolution
    (d : DeliberationSummary)
    (hResolution : d.currentResolution? = some "not_demonstrated") :
    d.notDemonstratedViable := by
  unfold DeliberationSummary.currentResolution? at hResolution
  by_cases hDem : d.demonstrated_count ≥ d.required_votes
  · simp [hDem] at hResolution
  · by_cases hNot : d.not_demonstrated_count ≥ d.required_votes
    · unfold DeliberationSummary.notDemonstratedViable
      exact Nat.le_trans hNot (Nat.le_add_right _ _)
    · simp [hDem, hNot] at hResolution

theorem DeliberationSummary.noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
    (d : DeliberationSummary)
    (hResolution : d.currentResolution? = none)
    (hRoundComplete : d.current_round_vote_count = d.seated_count) :
    d.noSubstantiveOutcomeViable := by
  have hBelow := d.currentResolution_none_implies_below_threshold hResolution
  constructor
  · intro hViable
    unfold DeliberationSummary.demonstratedViable DeliberationSummary.uncast_vote_count at hViable
    omega
  · intro hViable
    unfold DeliberationSummary.notDemonstratedViable DeliberationSummary.uncast_vote_count at hViable
    omega

theorem DeliberationSummary.noMajoritySound_of_noSubstantiveOutcomeViable
    (d : DeliberationSummary)
    (hNoViable : d.noSubstantiveOutcomeViable)
    (hReason : d.noMajorityClosureReason) :
    d.noMajoritySound := by
  have hDem := d.demonstrated_count_lt_required_of_not_viable hNoViable.1
  have hNot := d.not_demonstrated_count_lt_required_of_not_viable hNoViable.2
  exact ⟨hDem, hNot, hReason⟩

theorem DeliberationSummary.noMajoritySound_of_noMajorityClosure
    (d : DeliberationSummary)
    (hClosure : d.noMajorityClosure) :
    d.noMajoritySound := by
  rcases hClosure with ⟨_hRoundComplete, hNoViable, hReason⟩
  exact d.noMajoritySound_of_noSubstantiveOutcomeViable hNoViable hReason

theorem DeliberationSummary.closedResolution_eq_of_currentResolution
    (d : DeliberationSummary)
    (resolution : String)
    (hRoundComplete : d.current_round_vote_count = d.seated_count)
    (hResolution : d.currentResolution? = some resolution) :
    d.closedResolution? = some resolution := by
  simp [DeliberationSummary.closedResolution?, hRoundComplete, hResolution]

theorem DeliberationSummary.closedResolution_eq_no_majority_of_currentResolution_none
    (d : DeliberationSummary)
    (hRoundComplete : d.current_round_vote_count = d.seated_count)
    (hResolution : d.currentResolution? = none)
    (hReason : d.noMajorityClosureReason) :
    d.closedResolution? = some "no_majority" := by
  simp [DeliberationSummary.closedResolution?, hRoundComplete, hResolution, hReason]

theorem DeliberationSummary.closedResolution_eq_no_majority_of_noMajorityClosure
    (d : DeliberationSummary)
    (hClosure : d.noMajorityClosure) :
    d.closedResolution? = some "no_majority" := by
  rcases hClosure with ⟨hRoundComplete, hNoViable, hReason⟩
  have hNone : d.currentResolution? = none :=
    d.currentResolution_none_of_noSubstantiveOutcomeViable hNoViable
  simp [DeliberationSummary.closedResolution?, hRoundComplete, hNone, hReason]

theorem DeliberationSummary.currentResolution_eq_of_closedResolution_ne_no_majority
    (d : DeliberationSummary)
    (resolution : String)
    (hClosed : d.closedResolution? = some resolution)
    (hNe : resolution ≠ "no_majority") :
    d.currentResolution? = some resolution := by
  classical
  unfold DeliberationSummary.closedResolution? at hClosed
  by_cases hRoundComplete : d.current_round_vote_count = d.seated_count
  · simp [hRoundComplete] at hClosed
    cases hCurrent : d.currentResolution? with
    | some chosen =>
        have hChosen : chosen = resolution := by
          simpa [hCurrent] using hClosed
        subst chosen
        rfl
    | none =>
        by_cases hReason : d.noMajorityClosureReason
        · simp [hCurrent, hReason] at hClosed
          exact False.elim (hNe hClosed.symm)
        · simp [hCurrent, hReason] at hClosed
  · simp [hRoundComplete] at hClosed

theorem DeliberationSummary.noMajorityClosure_of_closedResolution_no_majority
    (d : DeliberationSummary)
    (hClosed : d.closedResolution? = some "no_majority") :
    d.noMajorityClosure := by
  classical
  unfold DeliberationSummary.closedResolution? at hClosed
  by_cases hRoundComplete : d.current_round_vote_count = d.seated_count
  · cases hCurrent : d.currentResolution? with
    | some resolution =>
        simp [hRoundComplete, hCurrent] at hClosed
        subst resolution
        unfold DeliberationSummary.currentResolution? at hCurrent
        by_cases hDem : d.demonstrated_count ≥ d.required_votes
        · simp [hDem] at hCurrent
        · by_cases hNot : d.not_demonstrated_count ≥ d.required_votes
          · simp [hDem, hNot] at hCurrent
          · simp [hDem, hNot] at hCurrent
    | none =>
        by_cases hReason : d.noMajorityClosureReason
        · have hNoViable : d.noSubstantiveOutcomeViable :=
            d.noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
              hCurrent hRoundComplete
          exact ⟨hRoundComplete, hNoViable, hReason⟩
        · simp [hRoundComplete, hCurrent, hReason] at hClosed
  · simp [hRoundComplete] at hClosed

theorem DeliberationSummary.afterDemonstratedVote_demonstratedViable_iff
    (d : DeliberationSummary)
    (hCapacity : d.current_round_vote_count < d.seated_count) :
    (d.afterDemonstratedVote).demonstratedViable ↔ d.demonstratedViable := by
  simp [DeliberationSummary.demonstratedViable,
    DeliberationSummary.afterDemonstratedVote,
    DeliberationSummary.uncast_vote_count]
  omega

theorem DeliberationSummary.afterDemonstratedVote_notDemonstratedViable_implies
    (d : DeliberationSummary)
    (hCapacity : d.current_round_vote_count < d.seated_count)
    (hViable : (d.afterDemonstratedVote).notDemonstratedViable) :
    d.notDemonstratedViable := by
  have hViable' :
      d.required_votes ≤ d.not_demonstrated_count +
        (d.seated_count - (d.current_round_vote_count + 1)) := by
    simpa [DeliberationSummary.notDemonstratedViable,
      DeliberationSummary.afterDemonstratedVote,
      DeliberationSummary.uncast_vote_count] using hViable
  unfold DeliberationSummary.notDemonstratedViable DeliberationSummary.uncast_vote_count
  omega

theorem DeliberationSummary.afterNotDemonstratedVote_notDemonstratedViable_iff
    (d : DeliberationSummary)
    (hCapacity : d.current_round_vote_count < d.seated_count) :
    (d.afterNotDemonstratedVote).notDemonstratedViable ↔ d.notDemonstratedViable := by
  simp [DeliberationSummary.notDemonstratedViable,
    DeliberationSummary.afterNotDemonstratedVote,
    DeliberationSummary.uncast_vote_count]
  omega

theorem DeliberationSummary.afterNotDemonstratedVote_demonstratedViable_implies
    (d : DeliberationSummary)
    (hCapacity : d.current_round_vote_count < d.seated_count)
    (hViable : (d.afterNotDemonstratedVote).demonstratedViable) :
    d.demonstratedViable := by
  have hViable' :
      d.required_votes ≤ d.demonstrated_count +
        (d.seated_count - (d.current_round_vote_count + 1)) := by
    simpa [DeliberationSummary.demonstratedViable,
      DeliberationSummary.afterNotDemonstratedVote,
      DeliberationSummary.uncast_vote_count] using hViable
  unfold DeliberationSummary.demonstratedViable DeliberationSummary.uncast_vote_count
  omega

theorem DeliberationSummary.afterSeatedMemberRemoval_demonstratedViable_implies
    (d : DeliberationSummary)
    (hCapacity : d.current_round_vote_count < d.seated_count)
    (hViable : (d.afterSeatedMemberRemoval).demonstratedViable) :
    d.demonstratedViable := by
  have hViable' :
      d.required_votes ≤ d.demonstrated_count +
        ((d.seated_count - 1) - d.current_round_vote_count) := by
    simpa [DeliberationSummary.demonstratedViable,
      DeliberationSummary.afterSeatedMemberRemoval,
      DeliberationSummary.uncast_vote_count] using hViable
  unfold DeliberationSummary.demonstratedViable DeliberationSummary.uncast_vote_count
  omega

theorem DeliberationSummary.afterSeatedMemberRemoval_notDemonstratedViable_implies
    (d : DeliberationSummary)
    (hCapacity : d.current_round_vote_count < d.seated_count)
    (hViable : (d.afterSeatedMemberRemoval).notDemonstratedViable) :
    d.notDemonstratedViable := by
  have hViable' :
      d.required_votes ≤ d.not_demonstrated_count +
        ((d.seated_count - 1) - d.current_round_vote_count) := by
    simpa [DeliberationSummary.notDemonstratedViable,
      DeliberationSummary.afterSeatedMemberRemoval,
      DeliberationSummary.uncast_vote_count] using hViable
  unfold DeliberationSummary.notDemonstratedViable DeliberationSummary.uncast_vote_count
  omega

theorem DeliberationSummary.flipSubstantiveCounts_demonstratedViable_iff
    (d : DeliberationSummary) :
    (d.flipSubstantiveCounts).demonstratedViable ↔ d.notDemonstratedViable := by
  simp [DeliberationSummary.flipSubstantiveCounts, DeliberationSummary.demonstratedViable,
    DeliberationSummary.notDemonstratedViable, DeliberationSummary.uncast_vote_count]

theorem DeliberationSummary.flipSubstantiveCounts_notDemonstratedViable_iff
    (d : DeliberationSummary) :
    (d.flipSubstantiveCounts).notDemonstratedViable ↔ d.demonstratedViable := by
  simp [DeliberationSummary.flipSubstantiveCounts, DeliberationSummary.demonstratedViable,
    DeliberationSummary.notDemonstratedViable, DeliberationSummary.uncast_vote_count]

theorem DeliberationSummary.flipSubstantiveCounts_noSubstantiveOutcomeViable_iff
    (d : DeliberationSummary) :
    (d.flipSubstantiveCounts).noSubstantiveOutcomeViable ↔ d.noSubstantiveOutcomeViable := by
  constructor <;> intro h <;>
    exact ⟨h.2, h.1⟩

theorem deliberationSummaryForCase_noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hResolution : currentResolution? c requiredVotes = none)
    (hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c) :
    (deliberationSummaryForCase c requiredVotes maxRounds).noSubstantiveOutcomeViable := by
  let d := deliberationSummaryForCase c requiredVotes maxRounds
  have hSummary : (deliberationSummaryForCase c requiredVotes maxRounds).currentResolution? = none := by
    simpa [d] using (deliberationSummaryForCase_currentResolution c requiredVotes maxRounds).trans hResolution
  have hSummaryRound :
      (deliberationSummaryForCase c requiredVotes maxRounds).current_round_vote_count =
        (deliberationSummaryForCase c requiredVotes maxRounds).seated_count := by
    simpa [d, deliberationSummaryForCase] using hRoundComplete
  exact d.noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
    (by simpa [d] using hSummary)
    (by simpa [d] using hSummaryRound)

theorem deliberationSummaryForCase_noSubstantiveOutcomeViable_implies_below_threshold
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hNoViable : (deliberationSummaryForCase c requiredVotes maxRounds).noSubstantiveOutcomeViable) :
    voteCountFor (currentRoundVotes c) "demonstrated" < requiredVotes ∧
      voteCountFor (currentRoundVotes c) "not_demonstrated" < requiredVotes := by
  let d := deliberationSummaryForCase c requiredVotes maxRounds
  have hDem :
      (deliberationSummaryForCase c requiredVotes maxRounds).demonstrated_count <
        (deliberationSummaryForCase c requiredVotes maxRounds).required_votes :=
    d.demonstrated_count_lt_required_of_not_viable (by simpa [d] using hNoViable.1)
  have hNot :
      (deliberationSummaryForCase c requiredVotes maxRounds).not_demonstrated_count <
        (deliberationSummaryForCase c requiredVotes maxRounds).required_votes :=
    d.not_demonstrated_count_lt_required_of_not_viable (by simpa [d] using hNoViable.2)
  simpa [d, deliberationSummaryForCase] using And.intro hDem hNot

theorem deliberationSummaryForCase_noMajoritySound_of_currentResolution_none_of_round_complete
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hResolution : currentResolution? c requiredVotes = none)
    (hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c)
    (hReason : seatedCouncilMemberCount c < requiredVotes ∨ c.deliberation_round ≥ maxRounds) :
    (deliberationSummaryForCase c requiredVotes maxRounds).noMajoritySound := by
  let d := deliberationSummaryForCase c requiredVotes maxRounds
  have hNoViable : d.noSubstantiveOutcomeViable := by
    exact deliberationSummaryForCase_noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
      c requiredVotes maxRounds hResolution hRoundComplete
  have hSummaryReason : d.noMajorityClosureReason := by
    rcases hReason with hTooFew | hLastRound
    · left
      simpa [d, deliberationSummaryForCase] using hTooFew
    · right
      constructor
      · simpa [d, deliberationSummaryForCase] using hRoundComplete
      · simpa [d, deliberationSummaryForCase] using hLastRound
  exact d.noMajoritySound_of_noSubstantiveOutcomeViable hNoViable hSummaryReason

theorem deliberationSummaryForCase_noMajorityClosure_of_currentResolution_none_of_round_complete
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat)
    (hResolution : currentResolution? c requiredVotes = none)
    (hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c)
    (hReason : seatedCouncilMemberCount c < requiredVotes ∨ c.deliberation_round ≥ maxRounds) :
    (deliberationSummaryForCase c requiredVotes maxRounds).noMajorityClosure := by
  let d := deliberationSummaryForCase c requiredVotes maxRounds
  have hNoViable : d.noSubstantiveOutcomeViable := by
    exact deliberationSummaryForCase_noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
      c requiredVotes maxRounds hResolution hRoundComplete
  have hSummaryReason : d.noMajorityClosureReason := by
    rcases hReason with hTooFew | hLastRound
    · left
      simpa [d, deliberationSummaryForCase] using hTooFew
    · right
      constructor
      · simpa [d, deliberationSummaryForCase] using hRoundComplete
      · simpa [d, deliberationSummaryForCase] using hLastRound
  exact ⟨by simpa [d, deliberationSummaryForCase] using hRoundComplete, hNoViable, hSummaryReason⟩

theorem currentResolution_demonstrated_implies_deliberationSummary_demonstratedViable
    (s : ArbitrationState)
    (hResolution : currentResolution? s.case s.policy.required_votes_for_decision = some "demonstrated") :
    (deliberationSummary s).demonstratedViable := by
  have hSummary :
      (deliberationSummary s).currentResolution? = some "demonstrated" := by
    simpa [deliberationSummary_currentResolution] using hResolution
  exact (deliberationSummary s).demonstratedViable_of_currentResolution hSummary

theorem currentResolution_not_demonstrated_implies_deliberationSummary_notDemonstratedViable
    (s : ArbitrationState)
    (hResolution : currentResolution? s.case s.policy.required_votes_for_decision = some "not_demonstrated") :
    (deliberationSummary s).notDemonstratedViable := by
  have hSummary :
      (deliberationSummary s).currentResolution? = some "not_demonstrated" := by
    simpa [deliberationSummary_currentResolution] using hResolution
  exact (deliberationSummary s).notDemonstratedViable_of_currentResolution hSummary

theorem deliberationSummary_noSubstantiveOutcomeViable_implies_currentResolution_none
    (s : ArbitrationState)
    (hNone : (deliberationSummary s).noSubstantiveOutcomeViable) :
    currentResolution? s.case s.policy.required_votes_for_decision = none := by
  have hSummaryNone :
      (deliberationSummary s).currentResolution? = none :=
    (deliberationSummary s).currentResolution_none_of_noSubstantiveOutcomeViable hNone
  simpa [deliberationSummary_currentResolution] using hSummaryNone

end ArbProofs
