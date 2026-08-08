import Proofs.Progress
import Proofs.OutcomeSoundness
import Proofs.ViableOutcomes

namespace ArbProofs

/-
This file connects the fixed-frame progress layer to the executable
same-round viability transport results.

`fixedFrameProgress` describes the monotone structural coordinates of a
successful public step.  `ViableOutcomes.lean` now describes what same-round
deliberation updates do to substantive viability and impossibility.  The
theorems here pair those two views at the public step boundary and package the
result as a separate same-round progress relation.
-/

def viableOutcomesShrink (s t : ArbitrationState) : Prop :=
  ((deliberationSummary t).demonstratedViable →
      (deliberationSummary s).demonstratedViable) ∧
    ((deliberationSummary t).notDemonstratedViable →
      (deliberationSummary s).notDemonstratedViable)

theorem viableOutcomesShrink_refl
    (s : ArbitrationState) :
    viableOutcomesShrink s s := by
  constructor <;> intro hViable <;> exact hViable

theorem viableOutcomesShrink_trans
    {s t u : ArbitrationState}
    (hST : viableOutcomesShrink s t)
    (hTU : viableOutcomesShrink t u) :
    viableOutcomesShrink s u := by
  constructor
  · intro hViable
    exact hST.1 (hTU.1 hViable)
  · intro hViable
    exact hST.2 (hTU.2 hViable)

theorem viableOutcomesShrink_preserves_noSubstantiveOutcomeViable
    {s t : ArbitrationState}
    (hShrink : viableOutcomesShrink s t)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable) :
    (deliberationSummary t).noSubstantiveOutcomeViable := by
  constructor
  · intro hViable
    exact hNoViable.1 (hShrink.1 hViable)
  · intro hViable
    exact hNoViable.2 (hShrink.2 hViable)

def sameRoundDeliberationProgress (s t : ArbitrationState) : Prop :=
  fixedFrameProgress s t ∧
    t.case.deliberation_round = s.case.deliberation_round ∧
    viableOutcomesShrink s t

structure SameRoundFailureResilience (s t : ArbitrationState) : Prop where
  fixed_frame :
    fixedFrameProgress s t
  same_round :
    t.case.deliberation_round = s.case.deliberation_round
  council_votes_preserved :
    t.case.council_votes = s.case.council_votes
  no_substantive_outcome_preserved :
    (deliberationSummary s).noSubstantiveOutcomeViable →
      (deliberationSummary t).noSubstantiveOutcomeViable
  blocks_substantive_current_resolution :
    (deliberationSummary s).noSubstantiveOutcomeViable →
      currentResolution? t.case t.policy.required_votes_for_decision = none

private theorem fixedFrameProgress_preserves_councilIdsUnique
    {s t : ArbitrationState}
    (hProgress : fixedFrameProgress s t)
    (hUnique : councilIdsUnique s.case) :
    councilIdsUnique t.case := by
  rcases hProgress with ⟨hFrame, _hMaterials, _hSeats, _hPhase, _hRound⟩
  rcases hFrame with ⟨_hProp, _hPolicy, hIds⟩
  unfold councilIdsUnique at hUnique ⊢
  simpa [hIds] using hUnique

private theorem fixedFrameProgress_required_votes_eq
    {s t : ArbitrationState}
    (hProgress : fixedFrameProgress s t) :
    (deliberationSummary t).required_votes =
      (deliberationSummary s).required_votes := by
  rcases hProgress with ⟨hFrame, _hMaterials, _hSeats, _hPhase, _hRound⟩
  rcases hFrame with ⟨_hProp, hPolicy, _hIds⟩
  simp [deliberationSummary, deliberationSummaryForCase, hPolicy]

private theorem fixedFrameProgress_max_deliberation_rounds_eq
    {s t : ArbitrationState}
    (hProgress : fixedFrameProgress s t) :
    (deliberationSummary t).max_deliberation_rounds =
      (deliberationSummary s).max_deliberation_rounds := by
  rcases hProgress with ⟨hFrame, _hMaterials, _hSeats, _hPhase, _hRound⟩
  rcases hFrame with ⟨_hProp, hPolicy, _hIds⟩
  simp [deliberationSummary, deliberationSummaryForCase, hPolicy]

private theorem fixedFrameProgress_seated_count_le
    {s t : ArbitrationState}
    (hProgress : fixedFrameProgress s t)
    (hUnique : councilIdsUnique s.case) :
    (deliberationSummary t).seated_count ≤
      (deliberationSummary s).seated_count := by
  have hUniqueT : councilIdsUnique t.case :=
    fixedFrameProgress_preserves_councilIdsUnique hProgress hUnique
  rcases hProgress with ⟨_hFrame, _hMaterials, hSeats, _hPhase, _hRound⟩
  have hLen :
      (seatedCouncilMemberIds t.case).length ≤
        (seatedCouncilMemberIds s.case).length :=
    list_length_le_of_nodup_subset
      (seatedCouncilMemberIds_nodup t.case hUniqueT)
      (seatedCouncilMemberIds_nodup s.case hUnique)
      hSeats
  simpa [deliberationSummary, deliberationSummaryForCase,
    seatedCouncilMemberCount, seatedCouncilMemberIds, councilMemberIds] using hLen

private theorem sameRoundDeliberationProgress_deliberation_round_eq
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t) :
    (deliberationSummary t).deliberation_round =
      (deliberationSummary s).deliberation_round := by
  simpa [deliberationSummary, deliberationSummaryForCase] using hProgress.2.1

theorem sameRoundDeliberationProgress_refl
    (s : ArbitrationState) :
    sameRoundDeliberationProgress s s := by
  exact ⟨fixedFrameProgress_refl s, rfl, viableOutcomesShrink_refl s⟩

theorem sameRoundDeliberationProgress_trans
    {s t u : ArbitrationState}
    (hST : sameRoundDeliberationProgress s t)
    (hTU : sameRoundDeliberationProgress t u) :
    sameRoundDeliberationProgress s u := by
  rcases hST with ⟨hProgressST, hRoundST, hShrinkST⟩
  rcases hTU with ⟨hProgressTU, hRoundTU, hShrinkTU⟩
  exact ⟨fixedFrameProgress_trans hProgressST hProgressTU,
    hRoundTU.trans hRoundST,
    viableOutcomesShrink_trans hShrinkST hShrinkTU⟩

theorem sameRoundDeliberationProgress_preserves_noSubstantiveOutcomeViable
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable) :
    (deliberationSummary t).noSubstantiveOutcomeViable := by
  exact viableOutcomesShrink_preserves_noSubstantiveOutcomeViable hProgress.2.2 hNoViable

theorem sameRoundDeliberationProgress_blocks_substantive_currentResolution
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable) :
    currentResolution? t.case t.policy.required_votes_for_decision = none := by
  exact deliberationSummary_noSubstantiveOutcomeViable_implies_currentResolution_none t
    (sameRoundDeliberationProgress_preserves_noSubstantiveOutcomeViable hProgress hNoViable)

theorem sameRoundDeliberationProgress_preserves_noMajorityClosureReason_of_round_complete
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t)
    (hUnique : councilIdsUnique s.case)
    (hReason : (deliberationSummary s).noMajorityClosureReason)
    (hRoundComplete :
      (deliberationSummary t).current_round_vote_count =
        (deliberationSummary t).seated_count) :
    (deliberationSummary t).noMajorityClosureReason := by
  rcases hReason with hTooFew | ⟨_hSourceRoundComplete, hLastRound⟩
  · left
    have hSeatedLe :
        (deliberationSummary t).seated_count ≤
          (deliberationSummary s).seated_count :=
      fixedFrameProgress_seated_count_le hProgress.1 hUnique
    have hReqEq :
        (deliberationSummary t).required_votes =
          (deliberationSummary s).required_votes :=
      fixedFrameProgress_required_votes_eq hProgress.1
    rw [hReqEq]
    exact Nat.lt_of_le_of_lt hSeatedLe hTooFew
  · right
    constructor
    · exact hRoundComplete
    · rw [sameRoundDeliberationProgress_deliberation_round_eq hProgress,
        fixedFrameProgress_max_deliberation_rounds_eq hProgress.1]
      exact hLastRound

theorem sameRoundDeliberationProgress_forces_noMajorityClosure
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t)
    (hUnique : councilIdsUnique s.case)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable)
    (hReason : (deliberationSummary s).noMajorityClosureReason)
    (hRoundComplete :
      (deliberationSummary t).current_round_vote_count =
        (deliberationSummary t).seated_count) :
    (deliberationSummary t).noMajorityClosure := by
  exact ⟨hRoundComplete,
    sameRoundDeliberationProgress_preserves_noSubstantiveOutcomeViable hProgress hNoViable,
    sameRoundDeliberationProgress_preserves_noMajorityClosureReason_of_round_complete
      hProgress hUnique hReason hRoundComplete⟩

theorem sameRoundDeliberationProgress_forces_closedResolution_no_majority
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t)
    (hUnique : councilIdsUnique s.case)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable)
    (hReason : (deliberationSummary s).noMajorityClosureReason)
    (hRoundComplete :
      (deliberationSummary t).current_round_vote_count =
        (deliberationSummary t).seated_count) :
    (deliberationSummary t).closedResolution? = some "no_majority" := by
  exact (deliberationSummary t).closedResolution_eq_no_majority_of_noMajorityClosure
    (sameRoundDeliberationProgress_forces_noMajorityClosure
      hProgress hUnique hNoViable hReason hRoundComplete)

theorem sameRoundDeliberationProgress_forces_continueDeliberation_closed_no_majority
    {s t : ArbitrationState}
    (hProgress : sameRoundDeliberationProgress s t)
    (hUnique : councilIdsUnique s.case)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable)
    (hReason : (deliberationSummary s).noMajorityClosureReason)
    (hRoundComplete :
      (deliberationSummary t).current_round_vote_count =
        (deliberationSummary t).seated_count) :
    continueDeliberation t t.case =
      .ok (stateWithCase t { t.case with status := "closed", phase := "closed", resolution := "no_majority" }) := by
  have hClosed :
      (deliberationSummary t).closedResolution? = some "no_majority" :=
    sameRoundDeliberationProgress_forces_closedResolution_no_majority
      hProgress hUnique hNoViable hReason hRoundComplete
  simpa [deliberationSummary] using
    continueDeliberation_closed_of_summary_closedResolution t t.case "no_majority" hClosed

theorem step_submit_council_vote_same_round_establishes_viableOutcomesShrink
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    viableOutcomesShrink s t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_submit_council_vote_details_with_valid_vote s t action hType hStepCore with
    ⟨memberId, vote, rationale, _hPhase, hSeated, hFresh, hVote, hCont⟩
  constructor
  · intro hViable
    rcases hVote with hVote | hVote
    · have hCont' :
          let c1 := { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := "demonstrated"
            rationale := trimString rationale
          } }
          continueDeliberation s c1 = .ok t := by
        simpa [hVote] using hCont
      exact (continueDeliberation_after_append_demonstrated_vote_same_round_demonstratedViable_iff
        s t memberId rationale hUnique hIntegrity hSeated hFresh hCont' hSameRound).mp hViable
    · have hCont' :
          let c1 := { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := "not_demonstrated"
            rationale := trimString rationale
          } }
          continueDeliberation s c1 = .ok t := by
        simpa [hVote] using hCont
      exact continueDeliberation_after_append_not_demonstrated_vote_same_round_demonstratedViable_implies
        s t memberId rationale hUnique hIntegrity hSeated hFresh hCont' hSameRound hViable
  · intro hViable
    rcases hVote with hVote | hVote
    · have hCont' :
          let c1 := { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := "demonstrated"
            rationale := trimString rationale
          } }
          continueDeliberation s c1 = .ok t := by
        simpa [hVote] using hCont
      exact continueDeliberation_after_append_demonstrated_vote_same_round_notDemonstratedViable_implies
        s t memberId rationale hUnique hIntegrity hSeated hFresh hCont' hSameRound hViable
    · have hCont' :
          let c1 := { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := "not_demonstrated"
            rationale := trimString rationale
          } }
          continueDeliberation s c1 = .ok t := by
        simpa [hVote] using hCont
      exact (continueDeliberation_after_append_not_demonstrated_vote_same_round_notDemonstratedViable_iff
        s t memberId rationale hUnique hIntegrity hSeated hFresh hCont' hSameRound).mp hViable

theorem step_remove_council_member_same_round_establishes_viableOutcomesShrink
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    viableOutcomesShrink s t := by
  constructor
  · intro hViable
    exact step_remove_council_member_same_round_demonstratedViable_implies
      s t action hType hStep hSameRound hUnique hIntegrity hViable
  · intro hViable
    exact step_remove_council_member_same_round_notDemonstratedViable_implies
      s t action hType hStep hSameRound hUnique hIntegrity hViable

theorem step_submit_council_vote_same_round_progress_and_transport
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    ∃ vote, fixedFrameProgress s t ∧ sameRoundVoteTransport vote s t := by
  rcases step_submit_council_vote_same_round_supports_submitted_outcome
      s t action hType hStep hSameRound hUnique hIntegrity with
    ⟨vote, hTransport⟩
  exact ⟨vote, step_establishes_fixedFrameProgress s t action hStep, hTransport⟩

theorem step_submit_council_vote_same_round_establishes_sameRoundDeliberationProgress
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    sameRoundDeliberationProgress s t := by
  exact ⟨step_establishes_fixedFrameProgress s t action hStep,
    hSameRound,
    step_submit_council_vote_same_round_establishes_viableOutcomesShrink
      s t action hType hStep hSameRound hUnique hIntegrity⟩

theorem step_submit_council_vote_same_round_preserves_noSubstantiveOutcomeViable
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable) :
    (deliberationSummary t).noSubstantiveOutcomeViable := by
  exact sameRoundDeliberationProgress_preserves_noSubstantiveOutcomeViable
    (step_submit_council_vote_same_round_establishes_sameRoundDeliberationProgress
      s t action hType hStep hSameRound hUnique hIntegrity)
    hNoViable

theorem step_submit_council_vote_same_round_forces_continueDeliberation_closed_no_majority
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable)
    (hReason : (deliberationSummary s).noMajorityClosureReason)
    (hRoundComplete :
      (deliberationSummary t).current_round_vote_count =
        (deliberationSummary t).seated_count) :
    continueDeliberation t t.case =
      .ok (stateWithCase t { t.case with status := "closed", phase := "closed", resolution := "no_majority" }) := by
  exact sameRoundDeliberationProgress_forces_continueDeliberation_closed_no_majority
    (step_submit_council_vote_same_round_establishes_sameRoundDeliberationProgress
      s t action hType hStep hSameRound hUnique hIntegrity)
    hUnique
    hNoViable
    hReason
    hRoundComplete

theorem step_remove_council_member_same_round_progress_and_preserves_noSubstantiveOutcomeViable
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    fixedFrameProgress s t ∧
      ((deliberationSummary s).noSubstantiveOutcomeViable →
        (deliberationSummary t).noSubstantiveOutcomeViable) := by
  constructor
  · exact step_establishes_fixedFrameProgress s t action hStep
  · intro hNoViable
    exact viableOutcomesShrink_preserves_noSubstantiveOutcomeViable
      (step_remove_council_member_same_round_establishes_viableOutcomesShrink
        s t action hType hStep hSameRound hUnique hIntegrity)
      hNoViable

theorem step_remove_council_member_same_round_establishes_sameRoundDeliberationProgress
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    sameRoundDeliberationProgress s t := by
  exact ⟨step_establishes_fixedFrameProgress s t action hStep,
    hSameRound,
    step_remove_council_member_same_round_establishes_viableOutcomesShrink
      s t action hType hStep hSameRound hUnique hIntegrity⟩

theorem step_remove_council_member_same_round_forces_continueDeliberation_closed_no_majority
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case)
    (hNoViable : (deliberationSummary s).noSubstantiveOutcomeViable)
    (hReason : (deliberationSummary s).noMajorityClosureReason)
    (hRoundComplete :
      (deliberationSummary t).current_round_vote_count =
        (deliberationSummary t).seated_count) :
    continueDeliberation t t.case =
      .ok (stateWithCase t { t.case with status := "closed", phase := "closed", resolution := "no_majority" }) := by
  exact sameRoundDeliberationProgress_forces_continueDeliberation_closed_no_majority
    (step_remove_council_member_same_round_establishes_sameRoundDeliberationProgress
      s t action hType hStep hSameRound hUnique hIntegrity)
    hUnique
    hNoViable
    hReason
    hRoundComplete

theorem step_fail_opportunity_same_round_progress_and_preserves_noSubstantiveOutcomeViable
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    fixedFrameProgress s t ∧
      ((deliberationSummary s).noSubstantiveOutcomeViable →
        (deliberationSummary t).noSubstantiveOutcomeViable) := by
  constructor
  · exact step_establishes_fixedFrameProgress s t action hStep
  · intro hNoViable
    have hStepCore := stepCore_ok_of_step_ok s t action hStep
    have hFail := step_fail_opportunity_result s t action hType hStepCore
    exact failOpportunity_same_round_preserves_noSubstantiveOutcomeViable
      s t action.payload hFail hSameRound hUnique hIntegrity hNoViable

theorem step_fail_opportunity_preserves_council_votes
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.council_votes = s.case.council_votes := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  have hFail := step_fail_opportunity_result s t action hType hStepCore
  rcases failOpportunity_result s t action.payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
      c1, hC1, _hDeliberation, _hSeated, _hFresh, hCont⟩
    have hVotes : t.case.council_votes = c1.council_votes :=
      continueDeliberation_preserves_council_votes s t c1 hCont
    rw [hC1] at hVotes
    simpa using hVotes
  · rcases hParty with ⟨_failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    rfl

theorem step_fail_opportunity_same_round_resilience
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : step { state := s, action := action } = .ok t)
    (hSameRound : t.case.deliberation_round = s.case.deliberation_round)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case) :
    SameRoundFailureResilience s t := by
  have hProgress :=
    step_fail_opportunity_same_round_progress_and_preserves_noSubstantiveOutcomeViable
      s t action hType hStep hSameRound hUnique hIntegrity
  exact
    { fixed_frame := hProgress.1
      same_round := hSameRound
      council_votes_preserved :=
        step_fail_opportunity_preserves_council_votes s t action hType hStep
      no_substantive_outcome_preserved := hProgress.2
      blocks_substantive_current_resolution := by
        intro hNoViable
        exact deliberationSummary_noSubstantiveOutcomeViable_implies_currentResolution_none
          t (hProgress.2 hNoViable) }

end ArbProofs
