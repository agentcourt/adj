import Proofs.DeliberationSummary
import Proofs.ViableOutcomesCore

namespace ArbProofs

/-
This file proves Stage 7 of the verification plan: deliberation neutrality.

The aggregation rule in `currentResolution?` checks `demonstrated` before
`not_demonstrated`.  That ordering is neutral only when both substantive
outcomes cannot reach the configured threshold at the same time.  The policy
validator now enforces exactly that condition through the strict-majority
requirement.

The proof still needs one additional invariant from reachable states.  The
current round cannot contain more distinct votes than there are seated council
members, and seated membership cannot exceed `policy.council_size`.  Those are
the facts that let the strict-majority arithmetic rule out the dual-threshold
case.
-/

def flipOutcomeLabel (value : String) : String :=
  let cleaned := trimString value
  if cleaned = "demonstrated" then
    "not_demonstrated"
  else if cleaned = "not_demonstrated" then
    "demonstrated"
  else
    value

def flipCouncilVote (vote : CouncilVote) : CouncilVote :=
  { vote with vote := flipOutcomeLabel vote.vote }

def flipCaseVotes (c : ArbitrationCase) : ArbitrationCase :=
  { c with council_votes := c.council_votes.map flipCouncilVote }

def flipResolution : Option String → Option String
  | some value => some (flipOutcomeLabel value)
  | none => none

@[simp] private theorem trimString_demonstrated :
    trimString "demonstrated" = "demonstrated" := by
  native_decide

@[simp] private theorem trimString_not_demonstrated :
    trimString "not_demonstrated" = "not_demonstrated" := by
  native_decide

@[simp] private theorem flipOutcomeLabel_demonstrated :
    flipOutcomeLabel "demonstrated" = "not_demonstrated" := by
  simp [flipOutcomeLabel]

@[simp] private theorem flipOutcomeLabel_not_demonstrated :
    flipOutcomeLabel "not_demonstrated" = "demonstrated" := by
  simp [flipOutcomeLabel]

private theorem currentRoundVotes_flipCaseVotes
    (c : ArbitrationCase) :
    currentRoundVotes (flipCaseVotes c) = (currentRoundVotes c).map flipCouncilVote := by
  unfold currentRoundVotes flipCaseVotes
  induction c.council_votes with
  | nil =>
      simp
  | cons vote votes ih =>
      by_cases hRound : vote.round = c.deliberation_round
      · simp [flipCouncilVote, hRound, ih]
      · simp [flipCouncilVote, hRound, ih]

private theorem voteCountFor_foldl_acc
    (votes : List CouncilVote)
    (value : String)
    (acc : Nat) :
    votes.foldl
        (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
        acc =
      acc + voteCountFor votes value := by
  induction votes generalizing acc with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      by_cases hValue : trimString vote.vote = value
      · calc
          (vote :: votes).foldl
              (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
              acc
            = votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                (acc + 1) := by
                  simp [List.foldl, hValue]
          _ = (acc + 1) + voteCountFor votes value := by
                simpa using ih (acc + 1)
          _ = acc + (1 + voteCountFor votes value) := by
                omega
          _ = acc + votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                1 := by
                  have hOne : votes.foldl
                      (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                      1 =
                    1 + voteCountFor votes value := by
                      simpa using ih 1
                  rw [hOne]
          _ = acc + voteCountFor (vote :: votes) value := by
                unfold voteCountFor
                simp [List.foldl, hValue]
      · calc
          (vote :: votes).foldl
              (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
              acc
            = votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                acc := by
                  simp [List.foldl, hValue]
          _ = acc + voteCountFor votes value := by
                simpa using ih acc
          _ = acc + voteCountFor (vote :: votes) value := by
                unfold voteCountFor
                simp [List.foldl, hValue]

private theorem voteCountFor_cons_neutrality
    (vote : CouncilVote)
    (votes : List CouncilVote)
    (value : String) :
    voteCountFor (vote :: votes) value =
      (if trimString vote.vote = value then 1 else 0) + voteCountFor votes value := by
  unfold voteCountFor
  simpa only [voteCountFor, List.foldl] using voteCountFor_foldl_acc votes value
    (if trimString vote.vote = value then 1 else 0)

private theorem flipCouncilVote_demonstrated_increment
    (vote : CouncilVote) :
    (if trimString (flipCouncilVote vote).vote = "demonstrated" then 1 else 0) =
      (if trimString vote.vote = "not_demonstrated" then 1 else 0) := by
  by_cases hDem : trimString vote.vote = "demonstrated"
  · simp [flipCouncilVote, flipOutcomeLabel, hDem]
  · by_cases hNot : trimString vote.vote = "not_demonstrated"
    · simp [flipCouncilVote, flipOutcomeLabel, hNot]
    · simp [flipCouncilVote, flipOutcomeLabel, hDem, hNot]

private theorem flipCouncilVote_not_demonstrated_increment
    (vote : CouncilVote) :
    (if trimString (flipCouncilVote vote).vote = "not_demonstrated" then 1 else 0) =
      (if trimString vote.vote = "demonstrated" then 1 else 0) := by
  by_cases hDem : trimString vote.vote = "demonstrated"
  · simp [flipCouncilVote, flipOutcomeLabel, hDem]
  · by_cases hNot : trimString vote.vote = "not_demonstrated"
    · simp [flipCouncilVote, flipOutcomeLabel, hNot]
    · simp [flipCouncilVote, flipOutcomeLabel, hDem, hNot]

private theorem voteCountFor_flipped_votes_demonstrated
    (votes : List CouncilVote) :
    voteCountFor (votes.map flipCouncilVote) "demonstrated" =
      voteCountFor votes "not_demonstrated" := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      simp only [List.map, voteCountFor_cons_neutrality, ih]
      rw [flipCouncilVote_demonstrated_increment]

private theorem voteCountFor_flipped_votes_not_demonstrated
    (votes : List CouncilVote) :
    voteCountFor (votes.map flipCouncilVote) "not_demonstrated" =
      voteCountFor votes "demonstrated" := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      simp only [List.map, voteCountFor_cons_neutrality, ih]
      rw [flipCouncilVote_not_demonstrated_increment]

private theorem substantive_vote_counts_le_length_neutrality
    (votes : List CouncilVote) :
    voteCountFor votes "demonstrated" +
      voteCountFor votes "not_demonstrated" ≤
      votes.length := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      rw [voteCountFor_cons_neutrality, voteCountFor_cons_neutrality]
      by_cases hDem : trimString vote.vote = "demonstrated"
      · have hNotNe : trimString vote.vote ≠ "not_demonstrated" := by
          intro hEq
          simp [hDem] at hEq
        have hTail := ih
        simp [hDem]
        omega
      · by_cases hNot : trimString vote.vote = "not_demonstrated"
        · have hTail := ih
          simp [hNot]
          omega
        · have hTail := ih
          simp [hDem, hNot]
          omega

private theorem strict_majority_excludes_dual_threshold
    (votes : List CouncilVote)
    (councilSize requiredVotes : Nat)
    (hLength : votes.length ≤ councilSize)
    (hMajority : councilSize < 2 * requiredVotes) :
    ¬ (voteCountFor votes "demonstrated" ≥ requiredVotes ∧
        voteCountFor votes "not_demonstrated" ≥ requiredVotes) := by
  intro hBoth
  have hCounts := substantive_vote_counts_le_length_neutrality votes
  rcases hBoth with ⟨hDem, hNot⟩
  omega

private theorem validatePolicy_ok_implies_strict_majority
    (p : ArbitrationPolicy)
    (hPolicy : validatePolicy p = .ok PUnit.unit) :
    p.council_size < 2 * p.required_votes_for_decision := by
  unfold validatePolicy at hPolicy
  by_cases hCouncil : p.council_size = 0
  · simp [hCouncil] at hPolicy
    cases hPolicy
  · by_cases hEvidence : trimString p.evidence_standard = ""
    · simp [hCouncil, hEvidence] at hPolicy
      cases hPolicy
    · by_cases hVotes : p.required_votes_for_decision = 0
      · simp [hCouncil, hEvidence, hVotes] at hPolicy
        cases hPolicy
      · by_cases hTooLarge : p.required_votes_for_decision > p.council_size
        · simp [hCouncil, hEvidence, hVotes, hTooLarge] at hPolicy
          cases hPolicy
        · by_cases hNotMajority : 2 * p.required_votes_for_decision ≤ p.council_size
          · simp [hCouncil, hEvidence, hVotes, hTooLarge, hNotMajority] at hPolicy
            cases hPolicy
          · omega

private theorem initializeCase_establishes_strict_majority
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.policy.council_size < 2 * s.policy.required_votes_for_decision := by
  have hFrame := initializeCase_establishes_caseFrame req s hInit
  rcases hFrame with ⟨_hProp, hPolicyEq, _hMembers⟩
  have hValid : validatePolicy req.state.policy = .ok PUnit.unit := by
    unfold initializeCase at hInit
    cases hPolicy : validatePolicy req.state.policy with
    | error err =>
        simp [hPolicy] at hInit
        cases hInit
    | ok okv =>
        cases okv
        rfl
  simpa [hPolicyEq] using validatePolicy_ok_implies_strict_majority req.state.policy hValid

private theorem step_preserves_strict_majority
    (s t : ArbitrationState)
    (action : CourtAction)
    (hMajority : s.policy.council_size < 2 * s.policy.required_votes_for_decision)
    (hStep : step { state := s, action := action } = .ok t) :
    t.policy.council_size < 2 * t.policy.required_votes_for_decision := by
  have hFrame : caseFrameMatches
      s.case.proposition
      s.policy
      (councilMemberIds s.case.council_members)
      s := by
    simp [caseFrameMatches]
  have hFrame' := step_preserves_caseFrame
    s t action
    s.case.proposition
    s.policy
    (councilMemberIds s.case.council_members)
    hFrame
    hStep
  rcases hFrame' with ⟨_hProp, hPolicyEq, _hMembers⟩
  simpa [hPolicyEq] using hMajority

private theorem reachable_strict_majority
    (s : ArbitrationState)
    (hs : Reachable s) :
    s.policy.council_size < 2 * s.policy.required_votes_for_decision := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_strict_majority req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_strict_majority s t action ih hStep

private theorem deliberationSummary_flipCaseVotes
    (s : ArbitrationState) :
    deliberationSummary (stateWithCase s (flipCaseVotes s.case)) =
      (deliberationSummary s).flipSubstantiveCounts := by
  apply DeliberationSummary.ext
  · rfl
  · rfl
  · calc
      (deliberationSummaryForCase (flipCaseVotes s.case)
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).current_round_vote_count
        = (currentRoundVotes (flipCaseVotes s.case)).length := by
            simp [deliberationSummaryForCase]
      _ = ((currentRoundVotes s.case).map flipCouncilVote).length := by
            rw [currentRoundVotes_flipCaseVotes]
      _ = (currentRoundVotes s.case).length := by
            simp
      _ = (deliberationSummaryForCase s.case
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).current_round_vote_count := by
            simp [deliberationSummaryForCase]
  · calc
      (deliberationSummaryForCase (flipCaseVotes s.case)
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).demonstrated_count
        = voteCountFor (currentRoundVotes (flipCaseVotes s.case)) "demonstrated" := by
            simp [deliberationSummaryForCase]
      _ = voteCountFor ((currentRoundVotes s.case).map flipCouncilVote) "demonstrated" := by
            rw [currentRoundVotes_flipCaseVotes]
      _ = voteCountFor (currentRoundVotes s.case) "not_demonstrated" := by
            exact voteCountFor_flipped_votes_demonstrated (currentRoundVotes s.case)
      _ = (deliberationSummaryForCase s.case
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).not_demonstrated_count := by
            simp [deliberationSummaryForCase]
  · calc
      (deliberationSummaryForCase (flipCaseVotes s.case)
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).not_demonstrated_count
        = voteCountFor (currentRoundVotes (flipCaseVotes s.case)) "not_demonstrated" := by
            simp [deliberationSummaryForCase]
      _ = voteCountFor ((currentRoundVotes s.case).map flipCouncilVote) "not_demonstrated" := by
            rw [currentRoundVotes_flipCaseVotes]
      _ = voteCountFor (currentRoundVotes s.case) "demonstrated" := by
            exact voteCountFor_flipped_votes_not_demonstrated (currentRoundVotes s.case)
      _ = (deliberationSummaryForCase s.case
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).demonstrated_count := by
            simp [deliberationSummaryForCase]
  · rfl
  · rfl

private theorem currentResolution_flipSubstantiveCounts_of_bound
    (d : DeliberationSummary)
    (hSubstantiveBound : d.substantive_vote_count ≤ d.seated_count)
    (hMajority : d.seated_count < 2 * d.required_votes) :
    (d.flipSubstantiveCounts).currentResolution? =
      flipResolution d.currentResolution? := by
  have hDualThreshold :
      ¬ (d.demonstrated_count ≥ d.required_votes ∧
          d.not_demonstrated_count ≥ d.required_votes) := by
    intro hBoth
    rcases hBoth with ⟨hDem, hNot⟩
    unfold DeliberationSummary.substantive_vote_count at hSubstantiveBound
    omega
  by_cases hDem : d.demonstrated_count ≥ d.required_votes
  · have hNotLt : d.not_demonstrated_count < d.required_votes := by
      apply Nat.lt_of_not_ge
      intro hNot
      exact hDualThreshold ⟨hDem, hNot⟩
    simp [DeliberationSummary.currentResolution?, DeliberationSummary.flipSubstantiveCounts,
      flipResolution, hDem, Nat.not_le.mpr hNotLt]
  · by_cases hNot : d.not_demonstrated_count ≥ d.required_votes
    · simp [DeliberationSummary.currentResolution?, DeliberationSummary.flipSubstantiveCounts,
        flipResolution, hDem, hNot]
    · simp [DeliberationSummary.currentResolution?, DeliberationSummary.flipSubstantiveCounts,
        flipResolution, hDem, hNot]

/--
Flipping every current-round vote in a reachable state swaps the result of
`currentResolution?` in the same way.

This is the public neutrality theorem for the aggregation rule over the full
validated policy space.  It depends on two facts that the earlier proof layers
already established: reachable states preserve a strict-majority threshold, and
reachable deliberation records never contain more distinct current-round votes
than there are council seats available to cast them.
-/
theorem reachable_currentResolution_is_neutral_under_vote_flip
    (s : ArbitrationState)
    (hs : Reachable s) :
    currentResolution? (flipCaseVotes s.case) s.policy.required_votes_for_decision =
      flipResolution (currentResolution? s.case s.policy.required_votes_for_decision) := by
  have hSubstantiveBound :
      (deliberationSummary s).substantive_vote_count ≤
        (deliberationSummary s).seated_count := by
    exact Nat.le_trans
      (deliberationSummary_substantive_vote_count_le_current_round_vote_count s)
      (reachable_deliberationSummary_vote_count_le_seated_count s hs)
  have hSummaryMajority :
      (deliberationSummary s).seated_count <
        2 * (deliberationSummary s).required_votes := by
    exact Nat.lt_of_le_of_lt
      (reachable_deliberationSummary_seated_count_le_council_size s hs)
      (by simpa [deliberationSummary, deliberationSummaryForCase] using
        reachable_strict_majority s hs)
  calc
    currentResolution? (flipCaseVotes s.case) s.policy.required_votes_for_decision
      = (deliberationSummary (stateWithCase s (flipCaseVotes s.case))).currentResolution? := by
          rw [deliberationSummary_currentResolution (stateWithCase s (flipCaseVotes s.case))]
          rfl
    _ = ((deliberationSummary s).flipSubstantiveCounts).currentResolution? := by
          rw [deliberationSummary_flipCaseVotes]
    _ = flipResolution ((deliberationSummary s).currentResolution?) := by
          exact currentResolution_flipSubstantiveCounts_of_bound
            (deliberationSummary s) hSubstantiveBound hSummaryMajority
    _ = flipResolution (currentResolution? s.case s.policy.required_votes_for_decision) := by
          simp [deliberationSummary_currentResolution]

end ArbProofs
