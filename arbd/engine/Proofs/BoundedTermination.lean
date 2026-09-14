import Proofs.OutcomeSoundness
import Proofs.Replay

namespace ArbdProofs

theorem addOpening_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (role text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "openings") :
    remainingMeritsSteps (addFiling c "openings" role text) + 1 =
      remainingMeritsSteps c := by
  have hShape' :
      bilateralStarted "openings" c.openings ∧
        c.arguments = [] ∧ c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = [] := by
    simpa [phaseShape, hPhase] using hShape
  rcases hShape' with ⟨hStarted, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.openings with
  | nil =>
      simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hList,
        hArguments, hRebuttals, hSurrebuttals, hClosings]
  | cons head tail =>
      cases tail with
      | nil =>
          simp [bilateralStarted, hList] at hStarted
          simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hList,
            hArguments, hRebuttals, hSurrebuttals, hClosings]
      | cons next rest =>
          simp [bilateralStarted, hList] at hStarted


theorem addArgument_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (role text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "arguments") :
    remainingMeritsSteps (addFiling c "arguments" role text) + 1 =
      remainingMeritsSteps c := by
  have hShape' :
      bilateralComplete "openings" c.openings ∧
        bilateralStarted "arguments" c.arguments ∧
        c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = [] := by
    simpa [phaseShape, hPhase] using hShape
  rcases hShape' with ⟨_hOpenings, hStarted, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.arguments with
  | nil =>
      simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hList,
        hRebuttals, hSurrebuttals, hClosings]
  | cons head tail =>
      cases tail with
      | nil =>
          simp [bilateralStarted, hList] at hStarted
          simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hList,
            hRebuttals, hSurrebuttals, hClosings]
      | cons next rest =>
          simp [bilateralStarted, hList] at hStarted


theorem addRebuttal_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "rebuttals") :
    remainingMeritsSteps (addFiling c "rebuttals" "plaintiff" text) + 1 =
      remainingMeritsSteps c := by
  have hShape' :
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = [] := by
    simpa [phaseShape, hPhase] using hShape
  rcases hShape' with ⟨_hOpenings, _hArguments, hRebuttals, _hSurrebuttals, _hClosings⟩
  simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hRebuttals]


theorem passRebuttal_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (hPhase : c.phase = "rebuttals") :
    remainingMeritsSteps { c with phase := "surrebuttals" } + 1 =
      remainingMeritsSteps c := by
  simp [remainingMeritsSteps, hPhase]


theorem addSurrebuttal_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "surrebuttals") :
    remainingMeritsSteps (addFiling c "surrebuttals" "defendant" text) + 1 =
      remainingMeritsSteps c := by
  have hShape' :
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        c.surrebuttals = [] ∧ c.closings = [] := by
    simpa [phaseShape, hPhase] using hShape
  rcases hShape' with ⟨_hOpenings, _hArguments, _hRebuttals, hSurrebuttals, hClosings⟩
  simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hSurrebuttals, hClosings]


theorem passSurrebuttal_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "surrebuttals") :
    remainingMeritsSteps { c with phase := "closings" } + 1 =
      remainingMeritsSteps c := by
  have hShape' :
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        c.surrebuttals = [] ∧ c.closings = [] := by
    simpa [phaseShape, hPhase] using hShape
  rcases hShape' with ⟨_hOpenings, _hArguments, _hRebuttals, _hSurrebuttals, hClosings⟩
  simp [remainingMeritsSteps, hPhase, hClosings]


theorem addClosing_decreases_remainingMeritsSteps
    (c : ArbitrationCase)
    (role text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "closings") :
    remainingMeritsSteps (addFiling c "closings" role text) + 1 =
      remainingMeritsSteps c := by
  have hShape' :
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
          defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
          bilateralStarted "closings" c.closings := by
    simpa [phaseShape, hPhase] using hShape
  rcases hShape' with ⟨_hOpenings, _hArguments, _hRebuttals, _hSurrebuttals, hStarted⟩
  cases hList : c.closings with
      | nil =>
          simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hList]
      | cons head tail =>
          cases tail with
          | nil =>
              simp [bilateralStarted, hList] at hStarted
              simp [remainingMeritsSteps, addFiling, advanceAfterMerits, hPhase, hList]
          | cons next rest =>
              simp [bilateralStarted, hList] at hStarted

theorem list_length_le_of_nodup_subset
    {α : Type}
    [BEq α]
    [LawfulBEq α]
    {xs ys : List α}
    (hxs : xs.Nodup)
    (hys : ys.Nodup)
    (hSub : xs ⊆ ys) :
    xs.length ≤ ys.length := by
  induction xs generalizing ys with
  | nil =>
      simp
  | cons x xs ih =>
      have hXsInfo := List.nodup_cons.mp hxs
      have hNotMem : x ∉ xs := hXsInfo.1
      have hXsTail : xs.Nodup := hXsInfo.2
      have hMemY : x ∈ ys := hSub (by simp)
      have hYsErase : (ys.erase x).Nodup := hys.erase x
      have hTailSub : xs ⊆ ys.erase x := by
        intro z hz
        have hzY : z ∈ ys := hSub (by simp [hz])
        have hzNe : z ≠ x := by
          intro hEq
          subst hEq
          exact hNotMem hz
        exact (List.Nodup.mem_erase_iff hys).mpr ⟨hzNe, hzY⟩
      have hLen : xs.length ≤ (ys.erase x).length := ih hXsTail hYsErase hTailSub
      calc
        (x :: xs).length = xs.length + 1 := by simp
        _ ≤ (ys.erase x).length + 1 := Nat.add_le_add_right hLen 1
        _ = ys.length := by
          rw [List.length_erase_of_mem hMemY]
          exact Nat.sub_add_cancel (Nat.succ_le_of_lt (List.length_pos_of_mem hMemY))


theorem list_length_lt_of_nodup_subset_and_fresh
    {α : Type}
    [BEq α]
    [LawfulBEq α]
    {xs ys : List α}
    {x : α}
    (hxs : xs.Nodup)
    (hys : ys.Nodup)
    (hSub : xs ⊆ ys)
    (hMem : x ∈ ys)
    (hFresh : x ∉ xs) :
    xs.length < ys.length := by
  have hConsSub : x :: xs ⊆ ys := by
    intro z hz
    rcases List.mem_cons.mp hz with hEq | hzXs
    · simpa [hEq] using hMem
    · exact hSub hzXs
  have hConsLen : (x :: xs).length ≤ ys.length :=
    list_length_le_of_nodup_subset
      (List.nodup_cons.mpr ⟨hFresh, hxs⟩)
      hys
      hConsSub
  exact Nat.lt_of_add_one_le hConsLen


theorem answers_lt_seated_of_fresh
    (c : ArbitrationCase) (memberId : String)
    (hUnique : councilIdsUnique c) (hIntegrity : answerIntegrity c)
    (hSeated : memberId ∈ seatedCouncilMemberIds c)
    (hFresh : memberId ∉ currentRoundAnswerIds c) :
    (currentRoundAnswers c).length < seatedCouncilMemberCount c := by
  have hSubset : currentRoundAnswerIds c ⊆ seatedCouncilMemberIds c := by
    intro id hId
    rcases List.mem_map.mp hId with ⟨answer, hAnswer, rfl⟩
    exact hIntegrity.2.2.2.2 answer hAnswer
  have hLt := list_length_lt_of_nodup_subset_and_fresh
    hIntegrity.1 (seatedCouncilMemberIds_nodup c hUnique) hSubset hSeated hFresh
  simpa [currentRoundAnswerIds, seatedCouncilMemberIds, councilMemberIds,
    seatedCouncilMemberCount] using hLt

theorem budget_le_components (s : ArbitrationState) :
    remainingStepBudget s ≤ remainingSubmittedEvidenceSteps s +
      remainingMeritsSteps s.case + remainingDeliberationSteps s := by
  unfold remainingStepBudget
  split <;> omega

theorem budget_eq_components (s : ArbitrationState)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed") :
    remainingStepBudget s = remainingSubmittedEvidenceSteps s +
      remainingMeritsSteps s.case + remainingDeliberationSteps s := by
  simp [remainingStepBudget, hClosed, hFailed]

theorem addFiling_budget_fields (c : ArbitrationCase) (phase role text : String) :
    (addFiling c phase role text).submitted_evidence = c.submitted_evidence ∧
    (addFiling c phase role text).council_members = c.council_members ∧
    (addFiling c phase role text).council_answers = c.council_answers ∧
    (addFiling c phase role text).deliberation_round = c.deliberation_round := by
  unfold addFiling
  have advance (d : ArbitrationCase) :
      (advanceAfterMerits d).submitted_evidence = d.submitted_evidence ∧
      (advanceAfterMerits d).council_members = d.council_members ∧
      (advanceAfterMerits d).council_answers = d.council_answers ∧
      (advanceAfterMerits d).deliberation_round = d.deliberation_round := by
    unfold advanceAfterMerits
    repeat first | split | exact ⟨rfl, rfl, rfl, rfl⟩
  split <;> exact advance _

theorem filing_decreases_budget
    (s : ArbitrationState) (phase role text : String)
    (offered : List OfferedEvidence) (reports : List TechnicalReport)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hMerits : remainingMeritsSteps (addFiling s.case phase role text) + 1 =
      remainingMeritsSteps s.case) :
    remainingStepBudget (stateWithCase s
      (appendSupplementalMaterials (addFiling s.case phase role text) offered reports)) <
      remainingStepBudget s := by
  have hFields := addFiling_budget_fields s.case phase role text
  let t := stateWithCase s
    (appendSupplementalMaterials (addFiling s.case phase role text) offered reports)
  have hEvidence : remainingSubmittedEvidenceSteps t = remainingSubmittedEvidenceSteps s := by
    simp only [t, remainingSubmittedEvidenceSteps, stateWithCase,
      appendSupplementalMaterials, hFields.1]
  have hCouncil : remainingDeliberationSteps t = remainingDeliberationSteps s := by
    simp only [t, remainingDeliberationSteps, stateWithCase, appendSupplementalMaterials,
      currentRoundAnswers, seatedCouncilMemberCount, seatedCouncilMembers,
      hFields.2.1, hFields.2.2.1, hFields.2.2.2]
  have hMeritsEqual : remainingMeritsSteps t.case =
      remainingMeritsSteps (addFiling s.case phase role text) := rfl
  have hBound := budget_le_components t
  rw [hEvidence, hCouncil, hMeritsEqual] at hBound
  change remainingStepBudget t < remainingStepBudget s
  rw [budget_eq_components s hClosed hFailed]
  omega

theorem submitEvidence_decreases_budget
    (s t : ArbitrationState) (role : String) (payload : Lean.Json)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hSubmit : submitEvidence s role payload = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  rcases submitEvidence_record_details_with_count s t role payload hSubmit with
    ⟨evidence, hOrigin, _, hCount, rfl⟩
  have hRole : evidence.role = "plaintiff" ∨ evidence.role = "defendant" := by
    rcases hOrigin with ⟨_, h⟩ | ⟨_, h⟩ | ⟨_, h⟩
    · exact h
    · exact Or.inl h
    · exact Or.inr h
  have hBound := budget_le_components (stateWithCase s (appendSubmittedEvidence s.case evidence))
  have hMerits : remainingMeritsSteps (appendSubmittedEvidence s.case evidence) =
      remainingMeritsSteps s.case := rfl
  change remainingStepBudget (stateWithCase s (appendSubmittedEvidence s.case evidence)) <
    remainingStepBudget s
  rw [budget_eq_components s hClosed hFailed]
  simp only [stateWithCase] at hBound
  rw [hMerits] at hBound
  rcases hRole with hRole | hRole <;>
    simp [remainingSubmittedEvidenceSteps, submittedEvidenceCountForRole,
      appendSubmittedEvidence, List.concat_eq_append, List.foldl_append,
      remainingDeliberationSteps, seatedCouncilMemberCount, seatedCouncilMembers,
      currentRoundAnswers, stateWithCase, hRole] at hBound hCount ⊢ <;> omega

theorem filter_length_decreases {α : Type} (xs : List α) (p q : α → Bool)
    (hSubset : ∀ x ∈ xs, q x = true → p x = true) :
    (xs.filter q).length ≤ (xs.filter p).length ∧
      ((∃ x ∈ xs, p x = true ∧ q x = false) →
        (xs.filter q).length < (xs.filter p).length) := by
  induction xs with
  | nil => simp
  | cons x xs ih =>
      have hTail := ih (fun y hy => hSubset y (by simp [hy]))
      have hHead := hSubset x (by simp)
      cases hp : p x <;> cases hq : q x <;>
        simp_all
      all_goals first
        | exact hTail.2
        | (constructor <;> omega)
        | (intro y hy hp hq; have := hTail.2 y hy hp hq; omega)

theorem seated_count_decreases
    (c : ArbitrationCase) (memberId : String) (update : CouncilMember → CouncilMember)
    (hSeated : c.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true)
    (hSame : ∀ member, member.member_id ≠ memberId →
      memberIsSeated (update member) = memberIsSeated member)
    (hRemoved : ∀ member, member.member_id = memberId →
      memberIsSeated (update member) = false) :
    seatedCouncilMemberCount { c with council_members := c.council_members.map update } <
      seatedCouncilMemberCount c := by
  have hSub : ∀ member ∈ c.council_members,
      memberIsSeated (update member) = true → memberIsSeated member = true := by
    intro member _ h
    by_cases hid : member.member_id = memberId
    · simp [hRemoved member hid] at h
    · simpa [hSame member hid] using h
  have hWitness : ∃ member ∈ c.council_members,
      memberIsSeated member = true ∧ memberIsSeated (update member) = false := by
    rcases List.any_eq_true.mp hSeated with ⟨member, hMem, h⟩
    have hId : member.member_id = memberId := by simpa using (Bool.and_eq_true_iff.mp h).1
    exact ⟨member, hMem, (Bool.and_eq_true_iff.mp h).2, hRemoved member hId⟩
  have hLt := (filter_length_decreases c.council_members memberIsSeated
    (fun member => memberIsSeated (update member)) hSub).2 hWitness
  simpa [seatedCouncilMemberCount, seatedCouncilMembers, List.filter_map,
    Function.comp_def] using hLt

theorem continueDeliberation_decreases_budget
    (s t : ArbitrationState) (c : ArbitrationCase)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hPhase : s.case.phase = "deliberation") (hNewPhase : c.phase = "deliberation")
    (hEvidence : c.submitted_evidence = s.case.submitted_evidence)
    (hDecrease : remainingDeliberationSteps (stateWithCase s c) < remainingDeliberationSteps s)
    (hContinue : continueDeliberation s c = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  rw [budget_eq_components s hClosed hFailed]
  have hMerits : remainingMeritsSteps s.case = 0 := by simp [remainingMeritsSteps, hPhase]
  rw [hMerits]
  unfold continueDeliberation at hContinue
  split at hContinue
  · cases hContinue
    simp only [remainingStepBudget, stateWithCase]
    simp
    omega
  · cases hContinue
    have hBound := budget_le_components (stateWithCase s c)
    have hNewMerits : remainingMeritsSteps c = 0 := by simp [remainingMeritsSteps, hNewPhase]
    change remainingStepBudget (stateWithCase s c) ≤
      remainingSubmittedEvidenceSteps (stateWithCase s c) + remainingMeritsSteps c +
      remainingDeliberationSteps (stateWithCase s c) at hBound
    rw [hNewMerits] at hBound
    have hSameEvidence : remainingSubmittedEvidenceSteps (stateWithCase s c) =
        remainingSubmittedEvidenceSteps s := by
      simp [remainingSubmittedEvidenceSteps, stateWithCase, hEvidence]
    rw [hSameEvidence] at hBound
    omega

theorem seated_and_fresh
    (c : ArbitrationCase) (memberId : String)
    (hSeated : c.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true)
    (hFresh : (currentRoundAnswers c).any
      (fun answer => answer.member_id = memberId) = false) :
    memberId ∈ seatedCouncilMemberIds c ∧ memberId ∉ currentRoundAnswerIds c := by
  constructor
  · rcases List.any_eq_true.mp hSeated with ⟨member, hMem, h⟩
    have hInfo : member.member_id = memberId ∧ memberIsSeated member = true := by simpa using h
    exact List.mem_map.mpr ⟨member, List.mem_filter.mpr ⟨hMem, hInfo.2⟩, hInfo.1⟩
  · intro hId
    rcases List.mem_map.mp hId with ⟨answer, hMem, hId⟩
    have hAny : (currentRoundAnswers c).any
        (fun answer => answer.member_id = memberId) = true :=
      List.any_eq_true.mpr ⟨answer, hMem, by simp [hId]⟩
    simp [hFresh] at hAny

theorem recordCouncilAnswer_decreases_budget
    (s t : ArbitrationState) (memberId : String) (answer : Nat) (rationale : String)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hUnique : councilIdsUnique s.case) (hIntegrity : answerIntegrity s.case)
    (hAnswer : recordCouncilAnswer s memberId answer rationale = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  have hPhase' : s.case.phase = "deliberation" := by
    by_cases h : s.case.phase = "deliberation"
    · exact h
    · simp [recordCouncilAnswer, h, Bind.bind, Except.bind] at hAnswer
  have hKnown : s.case.council_members.any (fun member => member.member_id = memberId) = true := by
    cases h : s.case.council_members.any (fun member => member.member_id = memberId)
    · simp [recordCouncilAnswer, hPhase', h, Bind.bind, Except.bind] at hAnswer
    · rfl
  have hSeated' : s.case.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true := by
    cases h : s.case.council_members.any
        (fun member => member.member_id = memberId && memberIsSeated member)
    · simp [recordCouncilAnswer, hPhase', hKnown, h, Bind.bind, Except.bind] at hAnswer
    · rfl
  have hRange : ¬answer > 100 := by
    intro h
    simp [recordCouncilAnswer, hPhase', hKnown, hSeated', h, Bind.bind, Except.bind] at hAnswer
  have hText : trimString rationale ≠ "" := by
    intro h
    simp [recordCouncilAnswer, hPhase', hKnown, hSeated', hRange, h,
      Bind.bind, Except.bind] at hAnswer
  have hFresh' : (currentRoundAnswers s.case).any
      (fun answer => answer.member_id = memberId) = false := by
    cases h : (currentRoundAnswers s.case).any (fun answer => answer.member_id = memberId)
    · rfl
    · simp [recordCouncilAnswer, hPhase', hKnown, hSeated', hRange, hText, h,
        Bind.bind, Except.bind] at hAnswer
  simp only [recordCouncilAnswer, hPhase', hKnown, hSeated', hRange, hText, hFresh',
    bne_self_eq_false, Bool.not_true, Bool.false_eq_true, if_false, Bind.bind, Except.bind] at hAnswer
  have hInfo := seated_and_fresh s.case memberId hSeated' hFresh'
  have hCapacity := answers_lt_seated_of_fresh s.case memberId hUnique hIntegrity hInfo.1 hInfo.2
  refine continueDeliberation_decreases_budget s t _ hClosed hFailed hPhase' ?_ ?_ ?_ hAnswer
  · rfl
  · rfl
  · simp [remainingDeliberationSteps, stateWithCase, currentRoundAnswers,
      seatedCouncilMemberCount, seatedCouncilMembers, List.concat_eq_append,
      List.filter_append] at hCapacity ⊢
    omega

theorem updatedCouncil_decreases_budget
    (s t : ArbitrationState) (memberId : String) (update : CouncilMember → CouncilMember)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hPhase : s.case.phase = "deliberation")
    (hUnique : councilIdsUnique s.case) (hIntegrity : answerIntegrity s.case)
    (hSeated : s.case.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true)
    (hFresh : (currentRoundAnswers s.case).any
      (fun answer => answer.member_id = memberId) = false)
    (hSame : ∀ member, member.member_id ≠ memberId →
      memberIsSeated (update member) = memberIsSeated member)
    (hRemoved : ∀ member, member.member_id = memberId →
      memberIsSeated (update member) = false)
    (hContinue : continueDeliberation s
      { s.case with council_members := s.case.council_members.map update } = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  have hInfo := seated_and_fresh s.case memberId hSeated hFresh
  have hCapacity := answers_lt_seated_of_fresh s.case memberId hUnique hIntegrity hInfo.1 hInfo.2
  have hCount := seated_count_decreases s.case memberId update hSeated hSame hRemoved
  refine continueDeliberation_decreases_budget s t _ hClosed hFailed hPhase ?_ ?_ ?_ hContinue
  · exact hPhase
  · rfl
  · change seatedCouncilMemberCount
      { s.case with council_members := s.case.council_members.map update } -
      (currentRoundAnswers s.case).length <
      seatedCouncilMemberCount s.case - (currentRoundAnswers s.case).length
    simp only [seatedCouncilMemberCount, seatedCouncilMembers, currentRoundAnswers] at hCount hCapacity ⊢
    omega

theorem removeCouncilMember_decreases_budget
    (s t : ArbitrationState) (memberId status : String)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hUnique : councilIdsUnique s.case) (hIntegrity : answerIntegrity s.case)
    (hResult : removeCouncilMember s memberId status = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  have hPhase : s.case.phase = "deliberation" := by
    by_cases h : s.case.phase = "deliberation"
    · exact h
    · simp [removeCouncilMember, h, Bind.bind, Except.bind] at hResult
  have hKnown : s.case.council_members.any (fun member => member.member_id = memberId) = true := by
    by_cases h : s.case.council_members.any (fun member => member.member_id = memberId) = true
    · exact h
    · simp [removeCouncilMember, hPhase, h, Bind.bind, Except.bind] at hResult
  have hSeated : s.case.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true := by
    by_cases h : s.case.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true
    · exact h
    · simp [removeCouncilMember, hPhase, hKnown, h, Bind.bind, Except.bind] at hResult
  have hText : trimString status ≠ "" := by
    intro h
    simp [removeCouncilMember, hPhase, hKnown, hSeated, h, Bind.bind, Except.bind] at hResult
  have hStatus : trimString status ≠ "seated" := by
    intro h
    simp [removeCouncilMember, hPhase, hKnown, hSeated, h, Bind.bind, Except.bind] at hResult
  have hFresh : (currentRoundAnswers s.case).any
      (fun answer => answer.member_id = memberId) = false := by
    by_cases h : (currentRoundAnswers s.case).any
      (fun answer => answer.member_id = memberId) = false
    · exact h
    · simp [removeCouncilMember, hPhase, hKnown, hSeated, hText, hStatus, h, Bind.bind, Except.bind] at hResult
  simp only [removeCouncilMember, hPhase, hKnown, hSeated, hText, hStatus, hFresh,
    bne_self_eq_false, Bool.not_true, Bool.false_eq_true, if_false, Bind.bind, Except.bind] at hResult
  refine updatedCouncil_decreases_budget s t memberId
    (fun member => if member.member_id = memberId then { member with status := trimString status } else member)
    hClosed hFailed hPhase hUnique hIntegrity hSeated hFresh ?_ ?_ ?_
  · intro member hId
    simp [hId]
  · intro member hId
    simp [hId, memberIsSeated, hStatus]
  · simpa only [hPhase] using hResult

theorem failCouncilMemberOpportunity_decreases_budget
    (s t : ArbitrationState) (memberId reason opportunityId message : String)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hUnique : councilIdsUnique s.case) (hIntegrity : answerIntegrity s.case)
    (hResult : failCouncilMemberOpportunity s memberId reason opportunityId message = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  have hPhase : s.case.phase = "deliberation" := by
    by_cases h : s.case.phase = "deliberation"
    · exact h
    · simp [failCouncilMemberOpportunity, h, Bind.bind, Except.bind] at hResult
  have hMember : memberId ≠ "" := by
    intro h
    simp [failCouncilMemberOpportunity, hPhase, h, Bind.bind, Except.bind] at hResult
  have hReason : reason ≠ "" := by
    intro h
    simp [failCouncilMemberOpportunity, hPhase, hMember, h, Bind.bind, Except.bind] at hResult
  have hKnown : s.case.council_members.any (fun member => member.member_id = memberId) = true := by
    by_cases h : s.case.council_members.any (fun member => member.member_id = memberId) = true
    · exact h
    · simp [failCouncilMemberOpportunity, hPhase, hMember, hReason, h, Bind.bind, Except.bind] at hResult
  have hSeated : s.case.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true := by
    by_cases h : s.case.council_members.any
      (fun member => member.member_id = memberId && memberIsSeated member) = true
    · exact h
    · simp [failCouncilMemberOpportunity, hPhase, hMember, hReason, hKnown, h, Bind.bind, Except.bind] at hResult
  have hFresh : (currentRoundAnswers s.case).any
      (fun answer => answer.member_id = memberId) = false := by
    by_cases h : (currentRoundAnswers s.case).any
      (fun answer => answer.member_id = memberId) = false
    · exact h
    · simp [failCouncilMemberOpportunity, hPhase, hMember, hReason, hKnown, hSeated, h, Bind.bind, Except.bind] at hResult
  simp only [failCouncilMemberOpportunity, hPhase, hMember, hReason, hKnown, hSeated, hFresh,
    bne_self_eq_false, Bool.not_true, Bool.false_eq_true, if_false, Bind.bind, Except.bind] at hResult
  refine updatedCouncil_decreases_budget s t memberId
    (fun member => if member.member_id = memberId then
      { member with
        status := "failed"
        failure_reason := reason
        failure_opportunity_id := opportunityId
        failure_message := message } else member)
    hClosed hFailed hPhase hUnique hIntegrity hSeated hFresh ?_ ?_ ?_
  · intro member hId
    simp [hId]
  · intro member hId
    simp [hId, memberIsSeated]
  · simpa only [hPhase] using hResult

theorem opportunity_implies_positive_budget
    (s : ArbitrationState) (opportunity : OpportunitySpec)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hUnique : councilIdsUnique s.case) (hIntegrity : answerIntegrity s.case)
    (hOpportunity : (nextOpportunity s).opportunity = some opportunity) :
    0 < remainingStepBudget s := by
  rw [budget_eq_components s hClosed hFailed]
  by_cases hMerits : 0 < remainingMeritsSteps s.case
  · omega
  have hopenings : s.case.phase ≠ "openings" := by
    intro h
    simp [remainingMeritsSteps, h] at hMerits
    split at hMerits <;> contradiction
  have harguments : s.case.phase ≠ "arguments" := by
    intro h
    simp [remainingMeritsSteps, h] at hMerits
    split at hMerits <;> contradiction
  have hrebuttals : s.case.phase ≠ "rebuttals" := by
    intro h
    simp [remainingMeritsSteps, h] at hMerits
  have hsurrebuttals : s.case.phase ≠ "surrebuttals" := by
    intro h
    simp [remainingMeritsSteps, h] at hMerits
  have hclosings : s.case.phase ≠ "closings" := by
    intro h
    simp [remainingMeritsSteps, h] at hMerits
    split at hMerits <;> contradiction

  have hPhase : s.case.phase = "deliberation" := by
    by_cases h : s.case.phase = "deliberation"
    · exact h
    · simp only [nextOpportunity, hClosed, hFailed, if_false,
        nextOpportunityForPhase] at hOpportunity
      split at hOpportunity <;> simp_all
  cases hNext : nextCouncilMember? s.case with
  | none =>
      simp [nextOpportunity, nextOpportunityForPhase, hClosed, hFailed, hPhase, hNext] at hOpportunity
  | some member =>
      have hFind : (seatedCouncilMembers s.case).find?
          (fun member => !(currentRoundAnswers s.case).any
            (fun answer => answer.member_id = member.member_id)) = some member := hNext
      have hMember := List.mem_of_find?_eq_some hFind
      have hUnanswered := List.find?_some hFind
      have hSeated : s.case.council_members.any
          (fun m => m.member_id = member.member_id && memberIsSeated m) = true := by
        have hInfo := List.mem_filter.mp hMember
        exact List.any_eq_true.mpr ⟨member, hInfo.1, by simp [hInfo.2]⟩
      have hFresh : (currentRoundAnswers s.case).any
          (fun answer => answer.member_id = member.member_id) = false := by
        simpa using hUnanswered
      have hInfo := seated_and_fresh s.case member.member_id hSeated hFresh
      have hCapacity := answers_lt_seated_of_fresh s.case member.member_id
        hUnique hIntegrity hInfo.1 hInfo.2
      unfold remainingDeliberationSteps
      omega

theorem phase_update_decreases_budget
    (s : ArbitrationState) (phase : String)
    (hClosed : s.case.status ≠ "closed") (hFailed : s.case.status ≠ "failed")
    (hMerits : remainingMeritsSteps { s.case with phase := phase } + 1 =
      remainingMeritsSteps s.case) :
    remainingStepBudget (stateWithCase s { s.case with phase := phase }) <
      remainingStepBudget s := by
  rw [budget_eq_components s hClosed hFailed]
  have hBound := budget_le_components (stateWithCase s { s.case with phase := phase })
  change remainingStepBudget (stateWithCase s { s.case with phase := phase }) ≤
    remainingSubmittedEvidenceSteps s + remainingMeritsSteps { s.case with phase := phase } +
    remainingDeliberationSteps s at hBound
  omega

set_option maxHeartbeats 600000 in
theorem step_decreases_remainingStepBudget
    (s t : ArbitrationState) (action : CourtAction)
    (hInvariant : ProcedureInvariant s)
    (hStep : step { state := s, action := action } = .ok t) :
    remainingStepBudget t < remainingStepBudget s := by
  have hClosed : s.case.status ≠ "closed" := by
    intro h
    rw [closed_step_rejected s action h] at hStep
    contradiction
  have hFailed : s.case.status ≠ "failed" := by
    intro h
    rw [failed_step_rejected s action h] at hStep
    contradiction
  have hCore := stepCore_ok_of_step_ok s t action hStep
  have hShape := hInvariant.phase_shape
  have hCases := hCore
  unfold stepCore at hCases
  split at hCases
  · have hType0 : action.action_type = "record_opening_statement" := by assumption
    have hPhase : s.case.phase = "openings" := by
      by_cases h : s.case.phase = "openings"
      · exact h
      · simp [stepCore, hType0, h, Bind.bind, Except.bind] at hCore
    rcases step_record_opening_statement_result s t action hType0 hCore with ⟨text, rfl⟩
    simpa [appendSupplementalMaterials] using
      filing_decreases_budget s "openings" _ (trimString text) [] [] hClosed hFailed
        (addOpening_decreases_remainingMeritsSteps s.case _ _ hShape hPhase)
  · have hType1 : action.action_type = "submit_argument" := by assumption
    have hSubmit : recordMeritsSubmission s "arguments" action.actor_role
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant") "argument"
        s.policy.max_argument_chars true action.payload = .ok t := by
      simpa [stepCore, hType1] using hCore
    have hPhase := recordMeritsSubmission_ok_phase s t _ _ _ _ _ _ _ hSubmit
    rcases recordMeritsSubmission_with_materials_record_details s t _ _ _ _ _ _ hSubmit with
      ⟨text, offered, reports, _, _, _, _, rfl⟩
    exact filing_decreases_budget s _ _ (trimString text) offered reports hClosed hFailed
      (addArgument_decreases_remainingMeritsSteps s.case _ _ hShape hPhase)
  · have hType2 : action.action_type = "submit_rebuttal" := by assumption
    have hSubmit : recordMeritsSubmission s "rebuttals" action.actor_role "plaintiff"
        "rebuttal" s.policy.max_rebuttal_chars true action.payload = .ok t := by
      simpa [stepCore, hType2] using hCore
    have hPhase := recordMeritsSubmission_ok_phase s t _ _ _ _ _ _ _ hSubmit
    rcases recordMeritsSubmission_with_materials_record_details s t _ _ _ _ _ _ hSubmit with
      ⟨text, offered, reports, _, _, _, _, rfl⟩
    exact filing_decreases_budget s _ _ (trimString text) offered reports hClosed hFailed
      (addRebuttal_decreases_remainingMeritsSteps s.case _ hShape hPhase)
  · have hType3 : action.action_type = "submit_surrebuttal" := by assumption
    have hSubmit : recordMeritsSubmission s "surrebuttals" action.actor_role "defendant"
        "surrebuttal" s.policy.max_surrebuttal_chars true action.payload = .ok t := by
      simpa [stepCore, hType3] using hCore
    have hPhase := recordMeritsSubmission_ok_phase s t _ _ _ _ _ _ _ hSubmit
    rcases recordMeritsSubmission_with_materials_record_details s t _ _ _ _ _ _ hSubmit with
      ⟨text, offered, reports, _, _, _, _, rfl⟩
    exact filing_decreases_budget s _ _ (trimString text) offered reports hClosed hFailed
      (addSurrebuttal_decreases_remainingMeritsSteps s.case _ hShape hPhase)
  · have hType4 : action.action_type = "submit_evidence" := by assumption
    exact submitEvidence_decreases_budget s t action.actor_role action.payload hClosed hFailed
      (by simpa [stepCore, hType4] using hCore)
  · have hType5 : action.action_type = "deliver_closing_statement" := by assumption
    have hPhase : s.case.phase = "closings" := by
      by_cases h : s.case.phase = "closings"
      · exact h
      · simp [stepCore, hType5, h, Bind.bind, Except.bind] at hCore
    rcases step_deliver_closing_statement_result s t action hType5 hCore with ⟨text, rfl⟩
    simpa [appendSupplementalMaterials] using
      filing_decreases_budget s "closings" _ (trimString text) [] [] hClosed hFailed
        (addClosing_decreases_remainingMeritsSteps s.case _ _ hShape hPhase)
  · have hType6 : action.action_type = "pass_phase_opportunity" := by assumption
    by_cases hPhase : s.case.phase = "rebuttals"
    · cases hRole : requireRole action.actor_role "plaintiff" <;>
        cases hEmpty : s.case.rebuttals.isEmpty <;>
        simp [stepCore, hType6, hPhase, hRole, hEmpty, Bind.bind, Except.bind] at hCore
      cases hCore
      exact phase_update_decreases_budget s "surrebuttals" hClosed hFailed
        (passRebuttal_decreases_remainingMeritsSteps s.case hPhase)
    · by_cases hPhase' : s.case.phase = "surrebuttals"
      · cases hRole : requireRole action.actor_role "defendant" <;>
          cases hEmpty : s.case.surrebuttals.isEmpty <;>
          simp [stepCore, hType6, hPhase', hRole, hEmpty, Bind.bind, Except.bind] at hCore
        cases hCore
        exact phase_update_decreases_budget s "closings" hClosed hFailed
          (passSurrebuttal_decreases_remainingMeritsSteps s.case hShape hPhase')
      · simp [stepCore, hType6, hPhase, hPhase'] at hCore
  · have hType7 : action.action_type = "submit_council_answer" := by assumption
    simp only [stepCore, hType7, Bind.bind, Except.bind] at hCore
    split at hCore <;> try contradiction
    split at hCore <;> try contradiction
    split at hCore <;> try contradiction
    split at hCore <;> try contradiction
    exact recordCouncilAnswer_decreases_budget s t _ _ _ hClosed hFailed
      hInvariant.council_ids_unique hInvariant.answers hCore
  · have hType8 : action.action_type = "remove_council_member" := by assumption
    simp only [stepCore, hType8, Bind.bind, Except.bind] at hCore
    split at hCore <;> try contradiction
    split at hCore <;> try contradiction
    split at hCore <;> try contradiction
    exact removeCouncilMember_decreases_budget s t _ _ hClosed hFailed
      hInvariant.council_ids_unique hInvariant.answers hCore
  · have hType9 : action.action_type = "fail_opportunity" := by assumption
    have hPositive : 0 < remainingStepBudget s := by
      rcases step_ok_matches_currentOpportunity s t action hStep with ⟨opportunity, hNext, _, _⟩
      exact opportunity_implies_positive_budget s opportunity hClosed hFailed
        hInvariant.council_ids_unique hInvariant.answers hNext
    have hFailure : failOpportunity s action.payload = .ok t := by
      cases hRole : requireRole action.actor_role "system" <;>
        simp [stepCore, hType9, hRole, Bind.bind, Except.bind] at hCore
      exact hCore
    rcases failOpportunity_success_effect s t action.payload hFailure with
      ⟨failure, rfl⟩ | ⟨memberId, reason, opportunityId, message, hCouncil⟩
    · simpa [remainingStepBudget, stateWithCase] using hPositive
    · exact failCouncilMemberOpportunity_decreases_budget s t _ _ _ _ hClosed hFailed
        hInvariant.council_ids_unique hInvariant.answers hCouncil
  · contradiction

theorem replaySteps_length_add_budget_le
    (req : InitializeCaseRequest) (start target : ArbitrationState)
    (actions : List CourtAction)
    (hInvariant : InitializedRunInvariant req start)
    (hReplay : replaySteps start actions = .ok target) :
    actions.length + remainingStepBudget target ≤ remainingStepBudget start := by
  induction actions generalizing start with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      simp
  | cons action rest ih =>
      cases hStep : step { state := start, action := action } with
      | error err => simp [replaySteps, hStep, Bind.bind, Except.bind] at hReplay
      | ok next =>
          have hRest : replaySteps next rest = .ok target := by
            simpa [replaySteps, hStep, Bind.bind, Except.bind] using hReplay
          have hNext := step_preserves_runInvariant req start next action hInvariant hStep
          have hBound := ih next hNext hRest
          have hDecrease := step_decreases_remainingStepBudget start next action hInvariant.procedure hStep
          simp only [List.length_cons]
          omega

theorem initializeCase_council_size
    (req : InitializeCaseRequest) (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.case.council_members.length = s.policy.council_size := by
  have hLength : req.council_members.length = req.state.policy.council_size := by
    by_cases h : req.council_members.length = req.state.policy.council_size
    · exact h
    unfold initializeCase at hInit
    cases hPolicy : validatePolicy req.state.policy with
    | error err => simp [hPolicy, Bind.bind, Except.bind] at hInit
    | ok value =>
        cases value
        by_cases hQuestion : trimString req.question = ""
        · simp [hPolicy, hQuestion, Bind.bind, Except.bind] at hInit
        by_cases hStandard : trimString req.state.policy.judgment_standard = ""
        · simp [hPolicy, hQuestion, hStandard, Bind.bind, Except.bind] at hInit
        by_cases hEmpty : req.council_members.isEmpty
        · simp [hPolicy, hQuestion, hStandard, hEmpty, Bind.bind, Except.bind] at hInit
        simp [hPolicy, hQuestion, hStandard, hEmpty, h, Bind.bind, Except.bind] at hInit
  rw [initializeCase_result req s hInit]
  simp [stateWithCase, initializedCase, hLength]

theorem replayInitialized_length_bound
    (req : InitializeCaseRequest) (actions : List CourtAction) (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target) :
    actions.length ≤
      (req.state.policy.max_submitted_evidence_per_side +
        req.state.policy.max_submitted_evidence_per_side) + 8 + req.state.policy.council_size := by
  rcases replayInitialized_success_components req actions target hReplay with ⟨start, hInit, hSteps⟩
  have hRun := replaySteps_length_add_budget_le req start target actions
    (initializeCase_establishes_runInvariant req start hInit) hSteps
  have hBound := remainingStepBudget_finite_bound start (initializeCase_council_size req start hInit)
  have hPolicy : start.policy = req.state.policy := by
    rw [initializeCase_result req start hInit]
    rfl
  rw [hPolicy] at hBound
  omega

theorem checkReplayCertificate_length_bound
    (req : InitializeCaseRequest) (actions : List CourtAction) (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    actions.length ≤
      (req.state.policy.max_submitted_evidence_per_side +
        req.state.policy.max_submitted_evidence_per_side) + 8 + req.state.policy.council_size :=
  replayInitialized_length_bound req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck)

theorem no_infinite_initialized_run
    (req : InitializeCaseRequest) (states : Nat → ArbitrationState) (actions : Nat → CourtAction)
    (hInit : initializeCase req = .ok (states 0)) :
    ¬∀ n, step { state := states n, action := actions n } = .ok (states (n + 1)) := by
  intro hSteps
  have hBound (n : Nat) :
      InitializedRunInvariant req (states n) ∧
        remainingStepBudget (states n) + n ≤ remainingStepBudget (states 0) := by
    induction n with
    | zero => exact ⟨initializeCase_establishes_runInvariant req _ hInit, by omega⟩
    | succ n ih =>
        have hStep := hSteps n
        have hNext := step_preserves_runInvariant req _ _ _ ih.1 hStep
        have hDecrease := step_decreases_remainingStepBudget _ _ _ ih.1.procedure hStep
        exact ⟨hNext, by omega⟩
  have := (hBound (remainingStepBudget (states 0) + 1)).2
  omega

end ArbdProofs
