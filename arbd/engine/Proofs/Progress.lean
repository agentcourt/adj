import Proofs.StepPreservation

namespace ArbdProofs

theorem currentOpportunity_exists_of_not_none
    (s : ArbitrationState)
    (hSome : (nextOpportunity s).opportunity ≠ none) :
    ∃ opportunity, currentOpportunity s = .ok opportunity := by
  cases hNext : (nextOpportunity s).opportunity with
  | none => exact False.elim (hSome hNext)
  | some opportunity =>
      exact ⟨opportunity, by simp [currentOpportunity, hNext]⟩

theorem merits_phase_has_currentOpportunity
    (s : ArbitrationState)
    (hShape : phaseShape s.case)
    (hNotClosed : s.case.status ≠ "closed")
    (hNotFailed : s.case.status ≠ "failed")
    (hMerits :
      s.case.phase = "openings" ∨
      s.case.phase = "arguments" ∨
      s.case.phase = "rebuttals" ∨
      s.case.phase = "surrebuttals" ∨
      s.case.phase = "closings") :
    ∃ opportunity, currentOpportunity s = .ok opportunity := by
  rcases hMerits with hPhase | hPhase | hPhase | hPhase | hPhase
  · have hStarted : bilateralStarted "openings" s.case.openings := by
      have hShape' : bilateralStarted "openings" s.case.openings ∧
          s.case.arguments = [] ∧ s.case.rebuttals = [] ∧
          s.case.surrebuttals = [] ∧ s.case.closings = [] := by
        simpa [phaseShape, hPhase] using hShape
      exact hShape'.1
    cases hOpenings : s.case.openings with
    | nil =>
        apply currentOpportunity_exists_of_not_none
        simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
          hPhase, plaintiffThenDefendant, hOpenings]
    | cons plaintiff rest =>
        cases rest with
        | nil =>
            apply currentOpportunity_exists_of_not_none
            simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
              hPhase, plaintiffThenDefendant, hOpenings]
        | cons defendant tail =>
            simp [bilateralStarted, hOpenings] at hStarted
  · have hStarted : bilateralStarted "arguments" s.case.arguments := by
      have hShape' : bilateralComplete "openings" s.case.openings ∧
          bilateralStarted "arguments" s.case.arguments ∧
          s.case.rebuttals = [] ∧ s.case.surrebuttals = [] ∧
          s.case.closings = [] := by
        simpa [phaseShape, hPhase] using hShape
      exact hShape'.2.1
    cases hArguments : s.case.arguments with
    | nil =>
        apply currentOpportunity_exists_of_not_none
        simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
          hPhase, plaintiffThenDefendant, hArguments]
    | cons plaintiff rest =>
        cases rest with
        | nil =>
            apply currentOpportunity_exists_of_not_none
            simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
              hPhase, plaintiffThenDefendant, hArguments]
        | cons defendant tail =>
            simp [bilateralStarted, hArguments] at hStarted
  · have hShape' := hShape
    simp [phaseShape, hPhase] at hShape'
    have hEmpty : s.case.rebuttals = [] := hShape'.2.2.1
    apply currentOpportunity_exists_of_not_none
    simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
      hPhase, hEmpty]
  · have hShape' := hShape
    simp [phaseShape, hPhase] at hShape'
    have hEmpty : s.case.surrebuttals = [] := hShape'.2.2.2.1
    apply currentOpportunity_exists_of_not_none
    simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
      hPhase, hEmpty]
  · have hShape' := hShape
    simp [phaseShape, hPhase] at hShape'
    have hStarted : bilateralStarted "closings" s.case.closings :=
      hShape'.2.2.2.2
    cases hClosings : s.case.closings with
    | nil =>
        apply currentOpportunity_exists_of_not_none
        simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
          hPhase, plaintiffThenDefendant, hClosings]
    | cons plaintiff rest =>
        cases rest with
        | nil =>
            apply currentOpportunity_exists_of_not_none
            simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
              hPhase, plaintiffThenDefendant, hClosings]
        | cons defendant tail =>
            simp [bilateralStarted, hClosings] at hStarted

theorem list_length_eq_of_nodup_same_members
    {left right : List String}
    (hLeft : left.Nodup)
    (hRight : right.Nodup)
    (hLeftRight : left ⊆ right)
    (hRightLeft : right ⊆ left) :
    left.length = right.length := by
  induction left generalizing right with
  | nil => exact (List.subset_nil.mp hRightLeft).symm ▸ rfl
  | cons item rest ih =>
      have hLeftInfo := List.nodup_cons.mp hLeft
      have hItemRight : item ∈ right := hLeftRight (by simp)
      have hRightErase : (right.erase item).Nodup := hRight.erase item
      have hRestRight : rest ⊆ right.erase item := by
        intro candidate hCandidate
        have hCandidateRight : candidate ∈ right := hLeftRight (by simp [hCandidate])
        have hCandidateNe : candidate ≠ item := by
          intro hEqual
          subst hEqual
          exact hLeftInfo.1 hCandidate
        exact (List.Nodup.mem_erase_iff hRight).mpr ⟨hCandidateNe, hCandidateRight⟩
      have hRightRest : right.erase item ⊆ rest := by
        intro candidate hCandidate
        have hInfo := (List.Nodup.mem_erase_iff hRight).mp hCandidate
        have hCandidateLeft : candidate ∈ item :: rest := hRightLeft hInfo.2
        simpa [hInfo.1] using hCandidateLeft
      have hTailLength : rest.length = (right.erase item).length :=
        ih hLeftInfo.2 hRightErase hRestRight hRightRest
      calc
        (item :: rest).length = rest.length + 1 := by simp
        _ = (right.erase item).length + 1 := by rw [hTailLength]
        _ = right.length := by
          rw [List.length_erase_of_mem hItemRight]
          exact Nat.sub_add_cancel (Nat.succ_le_of_lt (List.length_pos_of_mem hItemRight))

theorem currentRoundAnswerIds_length_eq_seatedCouncilMemberIds_length
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hIntegrity : answerIntegrity c)
    (hCover : seatedCouncilMemberIds c ⊆ currentRoundAnswerIds c) :
    (currentRoundAnswerIds c).length = (seatedCouncilMemberIds c).length := by
  have hAnswerSubset : currentRoundAnswerIds c ⊆ seatedCouncilMemberIds c := by
    intro memberId hMember
    rcases (show ∃ answer,
        answer ∈ currentRoundAnswers c ∧ answer.member_id = memberId from
      by simpa [currentRoundAnswerIds] using hMember) with
      ⟨answer, hAnswer, hAnswerId⟩
    simpa [hAnswerId] using hIntegrity.2.2.2.2 answer hAnswer
  exact list_length_eq_of_nodup_same_members
    hIntegrity.1 (seatedCouncilMemberIds_nodup c hUnique)
    hAnswerSubset hCover

theorem nextCouncilMember_none_implies_answers_complete
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hIntegrity : answerIntegrity c)
    (hNone : nextCouncilMember? c = none) :
    (currentRoundAnswers c).length = seatedCouncilMemberCount c := by
  have hCover : seatedCouncilMemberIds c ⊆ currentRoundAnswerIds c := by
    intro memberId hMember
    have hFindNone :
        (seatedCouncilMembers c).find? (fun member =>
          !(currentRoundAnswers c).any
            (fun answer => answer.member_id = member.member_id)) = none := by
      simpa [nextCouncilMember?] using hNone
    have hMemberExists : ∃ member,
        member ∈ seatedCouncilMembers c ∧ member.member_id = memberId := by
      simpa [seatedCouncilMemberIds, councilMemberIds] using hMember
    rcases hMemberExists with ⟨member, hMemberSeat, hMemberId⟩
    have hRejected := List.find?_eq_none.mp hFindNone member hMemberSeat
    have hAny : (currentRoundAnswers c).any
        (fun answer => answer.member_id = memberId) = true := by
      cases hAnswer : (currentRoundAnswers c).any
          (fun answer => answer.member_id = memberId) with
      | false =>
          simp [hMemberId, hAnswer] at hRejected
      | true => rfl
    rcases List.any_eq_true.mp hAny with ⟨answer, hAnswerMember, hAnswerId⟩
    have hStored : answer.member_id = memberId := of_decide_eq_true hAnswerId
    simpa [currentRoundAnswerIds] using
      (show ∃ stored,
          stored ∈ currentRoundAnswers c ∧ stored.member_id = memberId from
        ⟨answer, hAnswerMember, hStored⟩)
  have hLengths := currentRoundAnswerIds_length_eq_seatedCouncilMemberIds_length
    c hUnique hIntegrity hCover
  simpa [currentRoundAnswerIds, seatedCouncilMemberIds, seatedCouncilMemberCount,
    councilMemberIds] using hLengths

theorem deliberation_has_currentOpportunity
    (s : ArbitrationState)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : answerIntegrity s.case)
    (hNotClosed : s.case.status ≠ "closed")
    (hNotFailed : s.case.status ≠ "failed")
    (hPhase : s.case.phase = "deliberation")
    (hIncomplete :
      (currentRoundAnswers s.case).length ≠ seatedCouncilMemberCount s.case) :
    ∃ opportunity, currentOpportunity s = .ok opportunity := by
  cases hNext : nextCouncilMember? s.case with
  | none =>
      exact False.elim
        (hIncomplete (nextCouncilMember_none_implies_answers_complete
          s.case hUnique hIntegrity hNext))
  | some member =>
      apply currentOpportunity_exists_of_not_none
      simp [nextOpportunity, hNotClosed, hNotFailed, nextOpportunityForPhase,
        hPhase, hNext]

theorem accepted_step_has_currentOpportunity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    ∃ opportunity,
      currentOpportunity s = .ok opportunity ∧
      action.authority = authorityForOpportunity s opportunity ∧
      authorizeOpportunityAction opportunity action = .ok () := by
  rcases step_ok_matches_currentOpportunity s t action hStep with
    ⟨opportunity, hNext, hAuthority, hAuthorized⟩
  exact ⟨opportunity, by simp [currentOpportunity, hNext], hAuthority, hAuthorized⟩

def remainingMeritsSteps (c : ArbitrationCase) : Nat :=
  match c.phase with
  | "openings" => if c.openings.isEmpty then 8 else 7
  | "arguments" => if c.arguments.isEmpty then 6 else 5
  | "rebuttals" => 4
  | "surrebuttals" => 3
  | "closings" => if c.closings.isEmpty then 2 else 1
  | _ => 0

def remainingSubmittedEvidenceSteps (s : ArbitrationState) : Nat :=
  (s.policy.max_submitted_evidence_per_side -
      submittedEvidenceCountForRole s.case.submitted_evidence "plaintiff") +
    (s.policy.max_submitted_evidence_per_side -
      submittedEvidenceCountForRole s.case.submitted_evidence "defendant")

def remainingDeliberationSteps (s : ArbitrationState) : Nat :=
  seatedCouncilMemberCount s.case - (currentRoundAnswers s.case).length

def remainingStepBudget (s : ArbitrationState) : Nat :=
  if s.case.status = "failed" || s.case.status = "closed" then 0
  else remainingSubmittedEvidenceSteps s + remainingMeritsSteps s.case +
    remainingDeliberationSteps s

theorem remainingMeritsSteps_le_eight (c : ArbitrationCase) :
    remainingMeritsSteps c ≤ 8 := by
  by_cases hOpenings : c.phase = "openings"
  · cases hEmpty : c.openings.isEmpty <;>
      simp [remainingMeritsSteps, hOpenings, hEmpty]
  · by_cases hArguments : c.phase = "arguments"
    · cases hEmpty : c.arguments.isEmpty <;>
        simp [remainingMeritsSteps, hArguments, hEmpty]
    · by_cases hRebuttals : c.phase = "rebuttals"
      · simp [remainingMeritsSteps, hRebuttals]
      · by_cases hSurrebuttals : c.phase = "surrebuttals"
        · simp [remainingMeritsSteps, hSurrebuttals]
        · by_cases hClosings : c.phase = "closings"
          · cases hEmpty : c.closings.isEmpty <;>
              simp [remainingMeritsSteps, hClosings, hEmpty]
          · simp [remainingMeritsSteps]

theorem remainingSubmittedEvidenceSteps_le (s : ArbitrationState) :
    remainingSubmittedEvidenceSteps s ≤
      s.policy.max_submitted_evidence_per_side +
        s.policy.max_submitted_evidence_per_side := by
  exact Nat.add_le_add (Nat.sub_le _ _) (Nat.sub_le _ _)

theorem remainingDeliberationSteps_le_council_size
    (s : ArbitrationState)
    (hCouncilSize : s.case.council_members.length = s.policy.council_size) :
    remainingDeliberationSteps s ≤ s.policy.council_size := by
  calc
    remainingDeliberationSteps s ≤ seatedCouncilMemberCount s.case := Nat.sub_le _ _
    _ ≤ s.case.council_members.length := by
      exact List.length_filter_le memberIsSeated s.case.council_members
    _ = s.policy.council_size := hCouncilSize

theorem remainingStepBudget_finite_bound
    (s : ArbitrationState)
    (hCouncilSize : s.case.council_members.length = s.policy.council_size) :
    remainingStepBudget s ≤
      (s.policy.max_submitted_evidence_per_side +
        s.policy.max_submitted_evidence_per_side) + 8 + s.policy.council_size := by
  by_cases hTerminal : s.case.status = "failed" || s.case.status = "closed"
  · simp [remainingStepBudget, hTerminal]
  · simp only [remainingStepBudget, hTerminal, Bool.false_eq_true, if_false]
    exact Nat.add_le_add
      (Nat.add_le_add
        (remainingSubmittedEvidenceSteps_le s)
        (remainingMeritsSteps_le_eight s.case))
      (remainingDeliberationSteps_le_council_size s hCouncilSize)

end ArbdProofs
