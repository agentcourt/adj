import Proofs.ProcedureInvariants

namespace ArbdProofs

theorem initializeCase_result
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s = stateWithCase req.state (initializedCase req) := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error error => simp [hPolicy, Bind.bind, Except.bind] at hInit
  | ok value =>
      cases value
      by_cases hQuestion : trimString req.question = ""
      · simp [hPolicy, hQuestion, Bind.bind, Except.bind] at hInit
      · by_cases hStandard : trimString req.state.policy.judgment_standard = ""
        · simp [hPolicy, hQuestion, hStandard, Bind.bind, Except.bind] at hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hQuestion, hStandard, hEmpty, Bind.bind, Except.bind] at hInit
          · by_cases hLength : req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, Bind.bind,
                Except.bind] at hInit
            · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
              · simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, hDuplicate,
                  Bind.bind, Except.bind] at hInit
              · cases hCatalog : validateEvidenceCatalog req.state.evidence_catalog with
                | error error =>
                    simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, hDuplicate,
                      hCatalog, Bind.bind, Except.bind] at hInit
                | ok value =>
                    cases value
                    simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, hDuplicate,
                      hCatalog, Bind.bind, Except.bind] at hInit
                    cases hInit
                    rfl

theorem initializeCase_establishes_phaseShape
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    phaseShape s.case := by
  rw [initializeCase_result req s hInit]
  simp [initializedCase, stateWithCase, phaseShape, bilateralStarted]

theorem initializeCase_establishes_councilIdsUnique
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    councilIdsUnique s.case := by
  have hNoDuplicate : hasDuplicateCouncilMemberIds req.council_members = false := by
    unfold initializeCase at hInit
    cases hPolicy : validatePolicy req.state.policy with
    | error error => simp [hPolicy, Bind.bind, Except.bind] at hInit
    | ok value =>
        cases value
        by_cases hQuestion : trimString req.question = ""
        · simp [hPolicy, hQuestion, Bind.bind, Except.bind] at hInit
        · by_cases hStandard : trimString req.state.policy.judgment_standard = ""
          · simp [hPolicy, hQuestion, hStandard, Bind.bind, Except.bind] at hInit
          · by_cases hEmpty : req.council_members.isEmpty
            · simp [hPolicy, hQuestion, hStandard, hEmpty, Bind.bind, Except.bind] at hInit
            · by_cases hLength : req.council_members.length != req.state.policy.council_size
              · simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, Bind.bind,
                  Except.bind] at hInit
              · cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members with
                | true =>
                    simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, hDuplicate,
                      Bind.bind, Except.bind] at hInit
                | false => rfl
  rw [initializeCase_result req s hInit]
  apply hasDuplicateStrings_eq_false_implies_nodup
  simpa [stateWithCase, initializedCase, councilIdsUnique, councilMemberIds,
    hasDuplicateCouncilMemberIds, List.map_map, Function.comp_def] using hNoDuplicate

theorem initializeCase_establishes_answerIntegrity
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    answerIntegrity s.case := by
  rw [initializeCase_result req s hInit]
  simp [stateWithCase, initializedCase, answerIntegrity, currentRoundAnswerIds,
    currentRoundAnswers, seatedCouncilMemberIds]

theorem initializeCase_establishes_frame
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    initializedCaseFrame req s := by
  rw [initializeCase_result req s hInit]
  simp [initializedCaseFrame, caseFrameMatches, stateWithCase, initializedCase,
    councilMemberIdentities]

theorem initializeCase_establishes_procedureInvariant
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    ProcedureInvariant s :=
  { phase_shape := initializeCase_establishes_phaseShape req s hInit
    council_ids_unique := initializeCase_establishes_councilIdsUnique req s hInit
    answers := initializeCase_establishes_answerIntegrity req s hInit
    record := initializeCase_establishes_recordIntegrity req s hInit }

theorem initializeCase_establishes_runInvariant
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    InitializedRunInvariant req s :=
  { procedure := initializeCase_establishes_procedureInvariant req s hInit
    frame := initializeCase_establishes_frame req s hInit }

theorem setPhase_preserves_answerIntegrity
    (c : ArbitrationCase)
    (phase : String)
    (hIntegrity : answerIntegrity c) :
    answerIntegrity { c with phase := phase } :=
  answerIntegrity_congr (c := c) (d := { c with phase := phase }) rfl rfl rfl hIntegrity

theorem setPhase_preserves_councilIdsUnique
    (c : ArbitrationCase)
    (phase : String)
    (hUnique : councilIdsUnique c) :
    councilIdsUnique { c with phase := phase } :=
  councilIdsUnique_of_member_ids_eq
    (c := c) (d := { c with phase := phase }) rfl hUnique

theorem advanceAfterMerits_preserves_answerIntegrity
    (c : ArbitrationCase)
    (hIntegrity : answerIntegrity c) :
    answerIntegrity (advanceAfterMerits c) := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 && c.phase = "openings"
  · simpa [hOpen] using setPhase_preserves_answerIntegrity c "arguments" hIntegrity
  · by_cases hArgument : c.arguments.length >= 2 && c.phase = "arguments"
    · simpa [hOpen, hArgument] using setPhase_preserves_answerIntegrity c "rebuttals" hIntegrity
    · by_cases hRebuttal : c.rebuttals.length >= 1 && c.phase = "rebuttals"
      · simpa [hOpen, hArgument, hRebuttal] using
          setPhase_preserves_answerIntegrity c "surrebuttals" hIntegrity
      · by_cases hSurrebuttal : c.surrebuttals.length >= 1 && c.phase = "surrebuttals"
        · simpa [hOpen, hArgument, hRebuttal, hSurrebuttal] using
            setPhase_preserves_answerIntegrity c "closings" hIntegrity
        · by_cases hClosing : c.closings.length >= 2 && c.phase = "closings"
          · simpa [hOpen, hArgument, hRebuttal, hSurrebuttal, hClosing] using
              setPhase_preserves_answerIntegrity c "deliberation" hIntegrity
          · simpa [hOpen, hArgument, hRebuttal, hSurrebuttal, hClosing] using hIntegrity

theorem addFiling_preserves_answerIntegrity
    (c : ArbitrationCase)
    (phase role text : String)
    (hIntegrity : answerIntegrity c) :
    answerIntegrity (addFiling c phase role text) := by
  unfold addFiling
  split <;> exact advanceAfterMerits_preserves_answerIntegrity _ hIntegrity

theorem appendSupplementalMaterials_preserves_answerIntegrity
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hIntegrity : answerIntegrity c) :
    answerIntegrity (appendSupplementalMaterials c offered reports) :=
  answerIntegrity_congr rfl rfl rfl hIntegrity

theorem appendSubmittedEvidence_preserves_answerIntegrity
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence)
    (hIntegrity : answerIntegrity c) :
    answerIntegrity (appendSubmittedEvidence c evidence) :=
  answerIntegrity_congr rfl rfl rfl hIntegrity

theorem appendSupplementalMaterials_preserves_councilIdsUnique
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hUnique : councilIdsUnique c) :
    councilIdsUnique (appendSupplementalMaterials c offered reports) :=
  councilIdsUnique_of_member_ids_eq
    (c := c) (d := appendSupplementalMaterials c offered reports) rfl hUnique

theorem appendSubmittedEvidence_preserves_councilIdsUnique
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence)
    (hUnique : councilIdsUnique c) :
    councilIdsUnique (appendSubmittedEvidence c evidence) :=
  councilIdsUnique_of_member_ids_eq
    (c := c) (d := appendSubmittedEvidence c evidence) rfl hUnique

theorem continueDeliberation_preserves_answerIntegrity
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hIntegrity : answerIntegrity c)
    (hContinue : continueDeliberation s c = .ok t) :
    answerIntegrity t.case := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · simp [hComplete] at hContinue
    cases hContinue
    exact answerIntegrity_congr rfl rfl rfl hIntegrity
  · simp [hComplete] at hContinue
    cases hContinue
    exact answerIntegrity_congr rfl rfl rfl hIntegrity

theorem advanceAfterMerits_preserves_councilIdsUnique
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c) :
    councilIdsUnique (advanceAfterMerits c) := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 && c.phase = "openings"
  · simpa [hOpen] using setPhase_preserves_councilIdsUnique c "arguments" hUnique
  · by_cases hArgument : c.arguments.length >= 2 && c.phase = "arguments"
    · simpa [hOpen, hArgument] using setPhase_preserves_councilIdsUnique c "rebuttals" hUnique
    · by_cases hRebuttal : c.rebuttals.length >= 1 && c.phase = "rebuttals"
      · simpa [hOpen, hArgument, hRebuttal] using
          setPhase_preserves_councilIdsUnique c "surrebuttals" hUnique
      · by_cases hSurrebuttal : c.surrebuttals.length >= 1 && c.phase = "surrebuttals"
        · simpa [hOpen, hArgument, hRebuttal, hSurrebuttal] using
            setPhase_preserves_councilIdsUnique c "closings" hUnique
        · by_cases hClosing : c.closings.length >= 2 && c.phase = "closings"
          · simpa [hOpen, hArgument, hRebuttal, hSurrebuttal, hClosing] using
              setPhase_preserves_councilIdsUnique c "deliberation" hUnique
          · simpa [hOpen, hArgument, hRebuttal, hSurrebuttal, hClosing] using hUnique

theorem addFiling_preserves_councilIdsUnique
    (c : ArbitrationCase)
    (phase role text : String)
    (hUnique : councilIdsUnique c) :
    councilIdsUnique (addFiling c phase role text) := by
  unfold addFiling
  split <;> exact advanceAfterMerits_preserves_councilIdsUnique _ hUnique

theorem continueDeliberation_preserves_councilIdsUnique
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hContinue : continueDeliberation s c = .ok t) :
    councilIdsUnique t.case := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · simp [hComplete] at hContinue
    cases hContinue
    exact councilIdsUnique_of_member_ids_eq rfl hUnique
  · simp [hComplete] at hContinue
    cases hContinue
    exact councilIdsUnique_of_member_ids_eq rfl hUnique

theorem continueDeliberation_preserves_caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hCaseId : c.case_id = s.case.case_id)
    (hCaption : c.caption = s.case.caption)
    (hQuestion : c.question = s.case.question)
    (hMembers : councilMemberIdentities c.council_members =
      councilMemberIdentities s.case.council_members)
    (hContinue : continueDeliberation s c = .ok t) :
    caseFrameMatches caseId caption question policy catalog members t := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · simp [hComplete] at hContinue
    cases hContinue
    exact stateWithCase_preserves_caseFrameMatches
      caseId caption question policy catalog members s
      { c with status := "closed", phase := "closed" }
      hFrame hCaseId hCaption hQuestion hMembers
  · simp [hComplete] at hContinue
    cases hContinue
    exact stateWithCase_preserves_caseFrameMatches
      caseId caption question policy catalog members s c
      hFrame hCaseId hCaption hQuestion hMembers

theorem continueDeliberation_preserves_phaseShapeInvariant
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "deliberation")
    (hContinue : continueDeliberation s c = .ok t) :
    phaseShape t.case := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · simp [hComplete] at hContinue
    cases hContinue
    simpa [stateWithCase, phaseShape, hPhase] using hShape
  · simp [hComplete] at hContinue
    cases hContinue
    simpa [stateWithCase] using hShape

theorem updateUnansweredCouncilMember_preserves_invariants
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s t : ArbitrationState)
    (memberId : String)
    (update : CouncilMember → CouncilMember)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hPhase : s.case.phase = "deliberation")
    (hFresh : memberId ∉ currentRoundAnswerIds s.case)
    (hOther : ∀ member, member.member_id ≠ memberId → update member = member)
    (hId : ∀ member, (update member).member_id = member.member_id)
    (hIdentity : ∀ member,
      ((update member).member_id, (update member).model,
        (update member).persona_filename) =
      (member.member_id, member.model, member.persona_filename))
    (hContinue : continueDeliberation s
      { s.case with council_members := s.case.council_members.map update } = .ok t) :
    phaseShape t.case ∧ councilIdsUnique t.case ∧ answerIntegrity t.case ∧
      caseFrameMatches caseId caption question policy catalog members t := by
  let c1 := { s.case with council_members := s.case.council_members.map update }
  have hC1Shape : phaseShape c1 := by
    simpa [c1, phaseShape, hPhase] using hShape
  have hC1Unique : councilIdsUnique c1 := by
    apply councilIdsUnique_of_member_ids_eq
      (c := s.case) (d := c1) _ hUnique
    exact councilMemberIds_map_preserving_ids s.case.council_members update hId
  have hC1Answers : answerIntegrity c1 := by
    exact updateUnansweredCouncilMember_preserves_answerIntegrity
      s.case memberId update hAnswers hFresh hOther
  have hC1Frame : councilMemberIdentities c1.council_members =
      councilMemberIdentities s.case.council_members := by
    exact councilMemberIdentities_map_preserving_identity
      s.case.council_members update hIdentity
  exact ⟨
    continueDeliberation_preserves_phaseShapeInvariant s t c1 hC1Shape
      (by simpa [c1] using hPhase) hContinue,
    continueDeliberation_preserves_councilIdsUnique s t c1 hC1Unique hContinue,
    continueDeliberation_preserves_answerIntegrity s t c1 hC1Answers hContinue,
    continueDeliberation_preserves_caseFrameMatches
      caseId caption question policy catalog members s t c1 hFrame
      rfl rfl rfl hC1Frame hContinue⟩

theorem recordCouncilAnswer_preserves_invariants
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s t : ArbitrationState)
    (memberId : String)
    (answer : Nat)
    (rationale : String)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hRecord : recordCouncilAnswer s memberId answer rationale = .ok t) :
    phaseShape t.case ∧ councilIdsUnique t.case ∧ answerIntegrity t.case ∧
      caseFrameMatches caseId caption question policy catalog members t := by
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold recordCouncilAnswer at hRecord
    rw [if_neg (by simp [hPhase])] at hRecord
    cases hKnown : s.case.council_members.any
        (fun member => member.member_id = memberId) with
    | false =>
        rw [if_pos (by simp [hKnown])] at hRecord
        cases hRecord
    | true =>
        rw [if_neg (by simp [hKnown])] at hRecord
        cases hSeated : s.case.council_members.any
            (fun member => member.member_id = memberId && memberIsSeated member) with
        | false =>
            rw [if_pos (by simp [hSeated])] at hRecord
            cases hRecord
        | true =>
            rw [if_neg (by simp [hSeated])] at hRecord
            by_cases hAnswer : answer > 100
            · rw [if_pos hAnswer] at hRecord
              cases hRecord
            · rw [if_neg hAnswer] at hRecord
              by_cases hRationale : trimString rationale = ""
              · rw [if_pos hRationale] at hRecord
                cases hRecord
              · rw [if_neg hRationale] at hRecord
                cases hPrior : (currentRoundAnswers s.case).any
                    (fun existing => existing.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [hPrior])] at hRecord
                    cases hRecord
                | false =>
                    rw [if_neg (by simp [hPrior])] at hRecord
                    let newAnswer : CouncilAnswer := {
                      member_id := memberId
                      round := s.case.deliberation_round
                      answer := answer
                      rationale := trimString rationale
                    }
                    let c1 := { s.case with
                      council_answers := s.case.council_answers.concat newAnswer }
                    have hSeatedMember : memberId ∈ seatedCouncilMemberIds s.case := by
                      rcases List.any_eq_true.mp hSeated with ⟨member, hMember, hTest⟩
                      simp at hTest
                      rcases hTest with ⟨hMemberId, hMemberStatus⟩
                      have hFiltered : member ∈ seatedCouncilMembers s.case := by
                        exact List.mem_filter.mpr ⟨hMember, hMemberStatus⟩
                      simpa [seatedCouncilMemberIds, councilMemberIds, hMemberId] using
                        (show ∃ candidate,
                            candidate ∈ seatedCouncilMembers s.case ∧
                              candidate.member_id = memberId from
                          ⟨member, hFiltered, hMemberId⟩)
                    have hFresh : memberId ∉ currentRoundAnswerIds s.case := by
                      intro hMember
                      rcases (show ∃ stored,
                          stored ∈ currentRoundAnswers s.case ∧ stored.member_id = memberId from
                        by simpa [currentRoundAnswerIds] using hMember) with
                        ⟨stored, hStored, hStoredId⟩
                      have hAny : (currentRoundAnswers s.case).any
                          (fun existing => existing.member_id = memberId) = true :=
                        List.any_eq_true.mpr ⟨stored, hStored, by simp [hStoredId]⟩
                      rw [hPrior] at hAny
                      cases hAny
                    have hC1Answers : answerIntegrity c1 := by
                      simpa [c1, newAnswer] using
                        appendCurrentRoundAnswer_preserves_answerIntegrity
                          s.case memberId answer rationale hAnswers hSeatedMember hFresh
                          (Nat.le_of_not_gt hAnswer) hRationale
                    have hC1Shape : phaseShape c1 := by
                      simpa [c1, phaseShape, hPhase] using hShape
                    have hC1Unique : councilIdsUnique c1 := by
                      exact councilIdsUnique_of_member_ids_eq rfl hUnique
                    have hC1Frame :
                        councilMemberIdentities c1.council_members =
                          councilMemberIdentities s.case.council_members := rfl
                    exact ⟨
                      continueDeliberation_preserves_phaseShapeInvariant s t c1 hC1Shape
                        (by simpa [c1] using hPhase) hRecord,
                      continueDeliberation_preserves_councilIdsUnique s t c1 hC1Unique hRecord,
                      continueDeliberation_preserves_answerIntegrity s t c1 hC1Answers hRecord,
                      continueDeliberation_preserves_caseFrameMatches
                        caseId caption question policy catalog members s t c1 hFrame
                        rfl rfl rfl hC1Frame hRecord⟩
  · unfold recordCouncilAnswer at hRecord
    rw [if_pos (by simpa using hPhase)] at hRecord
    cases hRecord

theorem removeCouncilMember_preserves_invariants
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s t : ArbitrationState)
    (memberId status : String)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hRemove : removeCouncilMember s memberId status = .ok t) :
    phaseShape t.case ∧ councilIdsUnique t.case ∧ answerIntegrity t.case ∧
      caseFrameMatches caseId caption question policy catalog members t := by
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold removeCouncilMember at hRemove
    rw [if_neg (by simp [hPhase])] at hRemove
    cases hKnown : s.case.council_members.any
        (fun member => member.member_id = memberId) with
    | false =>
        rw [if_pos (by simp [hKnown])] at hRemove
        cases hRemove
    | true =>
        rw [if_neg (by simp [hKnown])] at hRemove
        cases hSeated : s.case.council_members.any
            (fun member => member.member_id = memberId && memberIsSeated member) with
        | false =>
            rw [if_pos (by simp [hSeated])] at hRemove
            cases hRemove
        | true =>
            rw [if_neg (by simp [hSeated])] at hRemove
            by_cases hStatusEmpty : trimString status = ""
            · rw [if_pos hStatusEmpty] at hRemove
              cases hRemove
            · rw [if_neg hStatusEmpty] at hRemove
              by_cases hSeatedStatus : trimString status = "seated"
              · rw [if_pos hSeatedStatus] at hRemove
                cases hRemove
              · rw [if_neg hSeatedStatus] at hRemove
                cases hAnswered : (currentRoundAnswers s.case).any
                    (fun existing => existing.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [hAnswered])] at hRemove
                    cases hRemove
                | false =>
                    rw [if_neg (by simp [hAnswered])] at hRemove
                    let update := fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with status := trimString status }
                      else member
                    have hFresh : memberId ∉ currentRoundAnswerIds s.case := by
                      intro hMember
                      rcases (show ∃ stored,
                          stored ∈ currentRoundAnswers s.case ∧ stored.member_id = memberId from
                        by simpa [currentRoundAnswerIds] using hMember) with
                        ⟨stored, hStored, hStoredId⟩
                      have hAny : (currentRoundAnswers s.case).any
                          (fun existing => existing.member_id = memberId) = true :=
                        List.any_eq_true.mpr ⟨stored, hStored, by simp [hStoredId]⟩
                      rw [hAnswered] at hAny
                      cases hAny
                    apply updateUnansweredCouncilMember_preserves_invariants
                      caseId caption question policy catalog members s t memberId update
                      hShape hUnique hAnswers hFrame hPhase hFresh
                    · intro member hNe
                      simp [update, hNe]
                    · intro member
                      by_cases hEq : member.member_id = memberId <;>
                        simp [update, hEq]
                    · intro member
                      by_cases hEq : member.member_id = memberId <;>
                        simp [update, hEq]
                    · simpa [update] using hRemove
  · unfold removeCouncilMember at hRemove
    rw [if_pos (by simpa using hPhase)] at hRemove
    cases hRemove

theorem failCouncilMemberOpportunity_preserves_invariants
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s t : ArbitrationState)
    (memberId reason opportunityId message : String)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hFail : failCouncilMemberOpportunity
      s memberId reason opportunityId message = .ok t) :
    phaseShape t.case ∧ councilIdsUnique t.case ∧ answerIntegrity t.case ∧
      caseFrameMatches caseId caption question policy catalog members t := by
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold failCouncilMemberOpportunity at hFail
    rw [if_neg (by simp [hPhase])] at hFail
    by_cases hMemberEmpty : memberId = ""
    · rw [if_pos hMemberEmpty] at hFail
      cases hFail
    · rw [if_neg hMemberEmpty] at hFail
      by_cases hReasonEmpty : reason = ""
      · rw [if_pos hReasonEmpty] at hFail
        cases hFail
      · rw [if_neg hReasonEmpty] at hFail
        cases hKnown : s.case.council_members.any
            (fun member => member.member_id = memberId) with
        | false =>
            rw [if_pos (by simp [hKnown])] at hFail
            cases hFail
        | true =>
            rw [if_neg (by simp [hKnown])] at hFail
            cases hSeated : s.case.council_members.any
                (fun member => member.member_id = memberId && memberIsSeated member) with
            | false =>
                rw [if_pos (by simp [hSeated])] at hFail
                cases hFail
            | true =>
                rw [if_neg (by simp [hSeated])] at hFail
                cases hAnswered : (currentRoundAnswers s.case).any
                    (fun existing => existing.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [hAnswered])] at hFail
                    cases hFail
                | false =>
                    rw [if_neg (by simp [hAnswered])] at hFail
                    let update := fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with
                          status := "failed"
                          failure_reason := reason
                          failure_opportunity_id := opportunityId
                          failure_message := message
                        }
                      else member
                    have hFresh : memberId ∉ currentRoundAnswerIds s.case := by
                      intro hMember
                      rcases (show ∃ stored,
                          stored ∈ currentRoundAnswers s.case ∧ stored.member_id = memberId from
                        by simpa [currentRoundAnswerIds] using hMember) with
                        ⟨stored, hStored, hStoredId⟩
                      have hAny : (currentRoundAnswers s.case).any
                          (fun existing => existing.member_id = memberId) = true :=
                        List.any_eq_true.mpr ⟨stored, hStored, by simp [hStoredId]⟩
                      rw [hAnswered] at hAny
                      cases hAny
                    apply updateUnansweredCouncilMember_preserves_invariants
                      caseId caption question policy catalog members s t memberId update
                      hShape hUnique hAnswers hFrame hPhase hFresh
                    · intro member hNe
                      simp [update, hNe]
                    · intro member
                      by_cases hEq : member.member_id = memberId <;>
                        simp [update, hEq]
                    · intro member
                      by_cases hEq : member.member_id = memberId <;>
                        simp [update, hEq]
                    · simpa [update] using hFail
  · unfold failCouncilMemberOpportunity at hFail
    rw [if_pos (by simpa using hPhase)] at hFail
    cases hFail

theorem markCaseFailed_preserves_invariants
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState)
    (failure : OpportunityFailure)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s) :
    let t := stateWithCase s
      { s.case with status := "failed", failure := some failure }
    phaseShape t.case ∧ councilIdsUnique t.case ∧ answerIntegrity t.case ∧
      caseFrameMatches caseId caption question policy catalog members t := by
  dsimp
  refine ⟨?_, ?_, ?_, ?_⟩
  · cases hPhase : s.case.phase <;>
      simpa [stateWithCase, phaseShape, hPhase] using hShape
  · exact councilIdsUnique_of_member_ids_eq rfl hUnique
  · exact answerIntegrity_congr rfl rfl rfl hAnswers
  · exact stateWithCase_preserves_caseFrameMatches
      caseId caption question policy catalog members s
      { s.case with status := "failed", failure := some failure }
      hFrame rfl rfl rfl rfl

theorem failOpportunity_preserves_invariants
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hFail : failOpportunity s payload = .ok t) :
    phaseShape t.case ∧ councilIdsUnique t.case ∧ answerIntegrity t.case ∧
      caseFrameMatches caseId caption question policy catalog members t := by
  unfold failOpportunity at hFail
  cases hOpportunityId : getString payload "opportunity_id" with
  | error error =>
      simp only [hOpportunityId] at hFail
      cases hFail
  | ok rawOpportunityId =>
      simp only [hOpportunityId] at hFail
      cases hRole : getString payload "role" with
      | error error =>
          simp only [hRole] at hFail
          cases hFail
      | ok rawRole =>
          simp only [hRole] at hFail
          cases hPhase : getString payload "phase" with
          | error error =>
              simp only [hPhase] at hFail
              cases hFail
          | ok rawPhase =>
              simp only [hPhase] at hFail
              cases hReason : getString payload "reason" with
              | error error =>
                  simp only [hReason] at hFail
                  cases hFail
              | ok rawReason =>
                  simp only [hReason] at hFail
                  cases hMessage : getOptionalString payload "message" with
                  | error error =>
                      simp only [hMessage] at hFail
                      cases hFail
                  | ok message =>
                      simp only [hMessage] at hFail
                      cases hMember : getOptionalString payload "member_id" with
                      | error error =>
                          simp only [hMember] at hFail
                          cases hFail
                      | ok memberId =>
                          simp only [hMember] at hFail
                          cases hModel : getOptionalString payload "model" with
                          | error error =>
                              simp only [hModel] at hFail
                              cases hFail
                          | ok model =>
                              simp only [hModel] at hFail
                              let opportunityId := trimString rawOpportunityId
                              let role := trimString rawRole
                              let phase := trimString rawPhase
                              let reason := trimString rawReason
                              change
                                (if opportunityId = "" then
                                    throw "opportunity failure requires opportunity_id"
                                  else if role = "" then
                                    throw "opportunity failure requires role"
                                  else if phase = "" then
                                    throw "opportunity failure requires phase"
                                  else if reason = "" then
                                    throw "opportunity failure requires reason"
                                  else
                                    do
                                      let opportunity ←
                                        match (nextOpportunity s).opportunity with
                                        | none => throw "no active opportunity can fail"
                                        | some opportunity => pure opportunity
                                      if opportunity.opportunity_id != opportunityId then
                                        throw s!"opportunity_id {opportunityId} does not match current opportunity {opportunity.opportunity_id}"
                                      else if opportunity.role != role then
                                        throw s!"opportunity role {role} does not match current role {opportunity.role}"
                                      else if opportunity.phase != phase then
                                        throw s!"opportunity phase {phase} does not match current phase {opportunity.phase}"
                                      else if role = "council" then
                                        failCouncilMemberOpportunity
                                          s memberId reason opportunityId message
                                      else if role = "plaintiff" || role = "defendant" then
                                        let failure : OpportunityFailure := {
                                          failure_type := "opportunity_failed"
                                          role := role
                                          phase := phase
                                          opportunity_id := opportunityId
                                          reason := reason
                                          message := message
                                          member_id := memberId
                                          model := model
                                        }
                                        pure <| stateWithCase s
                                          { s.case with
                                            status := "failed"
                                            failure := some failure }
                                      else
                                        throw s!"unsupported opportunity failure role: {role}") =
                                  .ok t at hFail
                              by_cases hOpportunityEmpty : opportunityId = ""
                              · rw [if_pos hOpportunityEmpty] at hFail
                                cases hFail
                              · by_cases hRoleEmpty : role = ""
                                · rw [if_neg hOpportunityEmpty, if_pos hRoleEmpty] at hFail
                                  cases hFail
                                · by_cases hPhaseEmpty : phase = ""
                                  · rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                      if_pos hPhaseEmpty] at hFail
                                    cases hFail
                                  · by_cases hReasonEmpty : reason = ""
                                    · rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                        if_neg hPhaseEmpty, if_pos hReasonEmpty] at hFail
                                      cases hFail
                                    · cases hNext : (nextOpportunity s).opportunity with
                                      | none =>
                                          rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                            if_neg hPhaseEmpty, if_neg hReasonEmpty] at hFail
                                          simp only [hNext] at hFail
                                          cases hFail
                                      | some opportunity =>
                                          rw [if_neg hOpportunityEmpty, if_neg hRoleEmpty,
                                            if_neg hPhaseEmpty, if_neg hReasonEmpty] at hFail
                                          simp only [hNext, Bind.bind, Except.bind,
                                            Pure.pure, Except.pure] at hFail
                                          by_cases hOpportunityMismatch :
                                              opportunity.opportunity_id != opportunityId
                                          · rw [if_pos hOpportunityMismatch] at hFail
                                            cases hFail
                                          · rw [if_neg hOpportunityMismatch] at hFail
                                            by_cases hRoleMismatch : opportunity.role != role
                                            · rw [if_pos hRoleMismatch] at hFail
                                              cases hFail
                                            · rw [if_neg hRoleMismatch] at hFail
                                              by_cases hPhaseMismatch : opportunity.phase != phase
                                              · rw [if_pos hPhaseMismatch] at hFail
                                                cases hFail
                                              · rw [if_neg hPhaseMismatch] at hFail
                                                by_cases hCouncil : role = "council"
                                                · rw [if_pos hCouncil] at hFail
                                                  exact
                                                    failCouncilMemberOpportunity_preserves_invariants
                                                      caseId caption question policy catalog members
                                                      s t memberId reason opportunityId message
                                                      hShape hUnique hAnswers hFrame hFail
                                                · rw [if_neg hCouncil] at hFail
                                                  by_cases hParty :
                                                      role = "plaintiff" || role = "defendant"
                                                  · rw [if_pos hParty] at hFail
                                                    let failure : OpportunityFailure := {
                                                      failure_type := "opportunity_failed"
                                                      role := role
                                                      phase := phase
                                                      opportunity_id := opportunityId
                                                      reason := reason
                                                      message := message
                                                      member_id := memberId
                                                      model := model
                                                    }
                                                    have hResult :
                                                        t = stateWithCase s
                                                          { s.case with
                                                            status := "failed"
                                                            failure := some failure } := by
                                                      simpa [failure] using hFail.symm
                                                    rw [hResult]
                                                    exact markCaseFailed_preserves_invariants
                                                      caseId caption question policy catalog members
                                                      s failure hShape hUnique hAnswers hFrame
                                                  · rw [if_neg hParty] at hFail
                                                    cases hFail

theorem stateWithCase_addFiling_preserves_caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState)
    (phase role text : String)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s) :
    caseFrameMatches caseId caption question policy catalog members
      (stateWithCase s (addFiling s.case phase role text)) := by
  have hFields := addFiling_preserves_caseFrameFields s.case phase role text
  apply stateWithCase_preserves_caseFrameMatches
    caseId caption question policy catalog members s _ hFrame
  · exact hFields.1
  · exact hFields.2.1
  · exact hFields.2.2.1
  · exact hFields.2.2.2

theorem stateWithCase_appendMaterials_preserves_caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState)
    (phase role text : String)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s) :
    caseFrameMatches caseId caption question policy catalog members
      (stateWithCase s
        (appendSupplementalMaterials
          (addFiling s.case phase role text) offered reports)) := by
  have hFields := appendSupplementalMaterials_preserves_caseFrameFields
    s.case (addFiling s.case phase role text) offered reports
    (addFiling_preserves_caseFrameFields s.case phase role text)
  apply stateWithCase_preserves_caseFrameMatches
    caseId caption question policy catalog members s _ hFrame
  · exact hFields.1
  · exact hFields.2.1
  · exact hFields.2.2.1
  · exact hFields.2.2.2

theorem stateWithCase_appendEvidence_preserves_caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState)
    (evidence : SubmittedEvidence)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s) :
    caseFrameMatches caseId caption question policy catalog members
      (stateWithCase s (appendSubmittedEvidence s.case evidence)) := by
  have hFields := appendSubmittedEvidence_preserves_caseFrameFields s.case evidence
  exact stateWithCase_preserves_caseFrameMatches
    caseId caption question policy catalog members s
    (appendSubmittedEvidence s.case evidence) hFrame
    hFields.1 hFields.2.1 hFields.2.2.1 hFields.2.2.2

theorem stateWithCase_setPhase_preserves_caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState)
    (phase : String)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s) :
    caseFrameMatches caseId caption question policy catalog members
      (stateWithCase s { s.case with phase := phase }) := by
  exact stateWithCase_preserves_caseFrameMatches
    caseId caption question policy catalog members s
    { s.case with phase := phase } hFrame rfl rfl rfl rfl

theorem recordMeritsSubmission_ok_phase
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (allowSupplementalMaterials : Bool)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission s phase actorRole expectedRole textLabel
      limit allowSupplementalMaterials payload = .ok t) :
    s.case.phase = phase := by
  by_cases hPhase : s.case.phase = phase
  · exact hPhase
  · unfold recordMeritsSubmission at hSubmit
    rw [if_pos (by simpa using hPhase)] at hSubmit
    cases hSubmit

theorem appendSupplementalMaterials_preserves_phaseShape
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hShape : phaseShape c) :
    phaseShape (appendSupplementalMaterials c offered reports) := by
  cases hPhase : c.phase <;>
    simpa [appendSupplementalMaterials, phaseShape, hPhase] using hShape

theorem appendSubmittedEvidence_preserves_phaseShape
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence)
    (hShape : phaseShape c) :
    phaseShape (appendSubmittedEvidence c evidence) := by
  cases hPhase : c.phase <;>
    simpa [appendSubmittedEvidence, phaseShape, hPhase] using hShape

theorem addOpening_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "openings") :
    phaseShape
      (addFiling c "openings"
        (if c.openings.isEmpty then "plaintiff" else "defendant") text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.openings with
  | nil =>
      simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralStarted,
        hArguments, hRebuttals, hSurrebuttals, hClosings]
  | cons first rest =>
      cases rest with
      | nil =>
          have hOne : first.phase = "openings" ∧ first.role = "plaintiff" := by
            simpa [bilateralStarted, hList] using hOpenings
          rcases hOne with ⟨hFirstPhase, hFirstRole⟩
          simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape,
            bilateralComplete, bilateralStarted, hArguments, hRebuttals,
            hSurrebuttals, hClosings, hFirstPhase, hFirstRole]
      | cons second tail =>
          simp [bilateralStarted, hList] at hOpenings

theorem addArgument_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "arguments") :
    phaseShape
      (addFiling c "arguments"
        (if c.arguments.isEmpty then "plaintiff" else "defendant") text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.arguments with
  | nil =>
      simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralStarted,
        hOpenings, hRebuttals, hSurrebuttals, hClosings]
  | cons first rest =>
      cases rest with
      | nil =>
          have hOne : first.phase = "arguments" ∧ first.role = "plaintiff" := by
            simpa [bilateralStarted, hList] using hArguments
          rcases hOne with ⟨hFirstPhase, hFirstRole⟩
          simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape]
          exact ⟨hOpenings, by simp [bilateralComplete, hFirstPhase, hFirstRole],
            hRebuttals, hSurrebuttals, hClosings⟩
      | cons second tail =>
          simp [bilateralStarted, hList] at hArguments

theorem addRebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "rebuttals") :
    phaseShape (addFiling c "rebuttals" "plaintiff" text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  simpa [addFiling, advanceAfterMerits, hPhase, phaseShape, hRebuttals,
    plaintiffOptionalSequence] using
      (show bilateralComplete "openings" c.openings ∧
          bilateralComplete "arguments" c.arguments ∧
          plaintiffOptionalSequence "rebuttals"
            [{ phase := "rebuttals", role := "plaintiff", text := text }] ∧
          c.surrebuttals = [] ∧ c.closings = [] from
        ⟨hOpenings, hArguments, by simp [plaintiffOptionalSequence],
          hSurrebuttals, hClosings⟩)

theorem addSurrebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "surrebuttals") :
    phaseShape (addFiling c "surrebuttals" "defendant" text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  simpa [addFiling, advanceAfterMerits, hPhase, phaseShape, hSurrebuttals,
    defendantOptionalSequence] using
      (show bilateralComplete "openings" c.openings ∧
          bilateralComplete "arguments" c.arguments ∧
          plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
          defendantOptionalSequence "surrebuttals"
            [{ phase := "surrebuttals", role := "defendant", text := text }] ∧
          bilateralStarted "closings" c.closings from
        ⟨hOpenings, hArguments, hRebuttals,
          by simp [defendantOptionalSequence], by simp [bilateralStarted, hClosings]⟩)

theorem addClosing_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "closings") :
    phaseShape
      (addFiling c "closings"
        (if c.closings.isEmpty then "plaintiff" else "defendant") text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.closings with
  | nil =>
      simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralStarted,
        hOpenings, hArguments, hRebuttals, hSurrebuttals]
  | cons first rest =>
      cases rest with
      | nil =>
          have hOne : first.phase = "closings" ∧ first.role = "plaintiff" := by
            simpa [bilateralStarted, hList] using hClosings
          rcases hOne with ⟨hFirstPhase, hFirstRole⟩
          simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape]
          exact ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals,
            by simp [bilateralComplete, hFirstPhase, hFirstRole]⟩
      | cons second tail =>
          simp [bilateralStarted, hList] at hClosings

theorem passRebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "rebuttals") :
    phaseShape { c with phase := "surrebuttals" } := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  exact ⟨hOpenings, hArguments, by simp [plaintiffOptionalSequence, hRebuttals],
    by simp [hSurrebuttals], by simp [hClosings]⟩

theorem passSurrebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "surrebuttals") :
    phaseShape { c with phase := "closings" } := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  exact ⟨hOpenings, hArguments, hRebuttals,
    by simp [defendantOptionalSequence, hSurrebuttals],
    by simp [bilateralStarted, hClosings]⟩

theorem stateWithCase_preserves_phaseShape
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hShape : phaseShape c) :
    phaseShape (stateWithCase s c).case := by
  simpa [stateWithCase] using hShape

theorem setCouncilAnswers_preserves_phaseShape
    (c : ArbitrationCase)
    (answers : List CouncilAnswer)
    (hShape : phaseShape c) :
    phaseShape { c with council_answers := answers } := by
  cases hPhase : c.phase <;> simpa [phaseShape, hPhase] using hShape

theorem setCouncilMembers_preserves_phaseShape
    (c : ArbitrationCase)
    (members : List CouncilMember)
    (hShape : phaseShape c) :
    phaseShape { c with council_members := members } := by
  cases hPhase : c.phase <;> simpa [phaseShape, hPhase] using hShape

theorem continueDeliberation_preserves_phaseShape
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "deliberation")
    (hContinue : continueDeliberation s c = .ok t) :
    phaseShape t.case := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · simp [hComplete] at hContinue
    cases hContinue
    simpa [stateWithCase, phaseShape, hPhase] using hShape
  · simp [hComplete] at hContinue
    cases hContinue
    exact stateWithCase_preserves_phaseShape s c hShape

theorem initializedRunInvariant_of_components
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hShape : phaseShape s.case)
    (hUnique : councilIdsUnique s.case)
    (hAnswers : answerIntegrity s.case)
    (hRecord : RecordIntegrity s)
    (hFrame : caseFrameMatches
      req.state.case.case_id
      req.state.case.caption
      (trimString req.question)
      req.state.policy
      req.state.evidence_catalog
      (councilMemberIdentities req.council_members)
      s) :
    InitializedRunInvariant req s :=
  { procedure := {
      phase_shape := hShape
      council_ids_unique := hUnique
      answers := hAnswers
      record := hRecord }
    frame := by simpa [initializedCaseFrame] using hFrame }

theorem step_preserves_runInvariant
    (req : InitializeCaseRequest)
    (s t : ArbitrationState)
    (action : CourtAction)
    (hInvariant : InitializedRunInvariant req s)
    (hStep : step { state := s, action := action } = .ok t) :
    InitializedRunInvariant req t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  have hRecord := step_preserves_recordIntegrity
    s t action hInvariant.procedure.record hStep
  have hFrame : caseFrameMatches
      req.state.case.case_id
      req.state.case.caption
      (trimString req.question)
      req.state.policy
      req.state.evidence_catalog
      (councilMemberIdentities req.council_members)
      s := by
    simpa [initializedCaseFrame] using hInvariant.frame
  by_cases hOpening : action.action_type = "record_opening_statement"
  · have hPhase : s.case.phase = "openings" := by
      by_cases hOpen : s.case.phase = "openings"
      · exact hOpen
      · have hClosed : s.case.phase != "openings" := by simpa using hOpen
        simp [stepCore, hOpening, hClosed, Bind.bind, Except.bind] at hStepCore
    rcases step_record_opening_statement_result s t action hOpening hStepCore with
      ⟨rawText, rfl⟩
    apply initializedRunInvariant_of_components req
    · exact stateWithCase_preserves_phaseShape s _
        (addOpening_preserves_phaseShape s.case (trimString rawText)
          hInvariant.procedure.phase_shape hPhase)
    · simpa [stateWithCase] using
        addFiling_preserves_councilIdsUnique s.case "openings"
          (if s.case.openings.isEmpty then "plaintiff" else "defendant")
          (trimString rawText) hInvariant.procedure.council_ids_unique
    · simpa [stateWithCase] using
        addFiling_preserves_answerIntegrity s.case "openings"
          (if s.case.openings.isEmpty then "plaintiff" else "defendant")
          (trimString rawText) hInvariant.procedure.answers
    · exact hRecord
    · exact stateWithCase_addFiling_preserves_caseFrameMatches
        req.state.case.case_id req.state.case.caption (trimString req.question)
        req.state.policy req.state.evidence_catalog
        (councilMemberIdentities req.council_members) s "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText) hFrame
  · by_cases hArgument : action.action_type = "submit_argument"
    · let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
      have hSubmit : recordMeritsSubmission s "arguments" action.actor_role role
          "argument" s.policy.max_argument_chars true action.payload = .ok t := by
        simpa [stepCore, hArgument, role] using hStepCore
      have hPhase : s.case.phase = "arguments" :=
        recordMeritsSubmission_ok_phase s t "arguments" action.actor_role role
          "argument" s.policy.max_argument_chars true action.payload hSubmit
      rcases recordMeritsSubmission_with_materials_record_details
          s t "arguments" action.actor_role role "argument"
          s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
          _hOfferedValid, _hReportsValid, rfl⟩
      apply initializedRunInvariant_of_components req
      · exact stateWithCase_preserves_phaseShape s _
          (appendSupplementalMaterials_preserves_phaseShape _ offered reports
            (addArgument_preserves_phaseShape s.case (trimString rawText)
              hInvariant.procedure.phase_shape hPhase))
      · simpa [stateWithCase] using
          appendSupplementalMaterials_preserves_councilIdsUnique
            (addFiling s.case "arguments" role (trimString rawText)) offered reports
            (addFiling_preserves_councilIdsUnique s.case "arguments" role
              (trimString rawText) hInvariant.procedure.council_ids_unique)
      · simpa [stateWithCase] using
          appendSupplementalMaterials_preserves_answerIntegrity
            (addFiling s.case "arguments" role (trimString rawText)) offered reports
            (addFiling_preserves_answerIntegrity s.case "arguments" role
              (trimString rawText) hInvariant.procedure.answers)
      · exact hRecord
      · exact stateWithCase_appendMaterials_preserves_caseFrameMatches
          req.state.case.case_id req.state.case.caption (trimString req.question)
          req.state.policy req.state.evidence_catalog
          (councilMemberIdentities req.council_members) s "arguments" role
          (trimString rawText) offered reports hFrame
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hSubmit : recordMeritsSubmission s "rebuttals" action.actor_role
            "plaintiff" "rebuttal" s.policy.max_rebuttal_chars true action.payload = .ok t := by
          simpa [stepCore, hRebuttal] using hStepCore
        have hPhase : s.case.phase = "rebuttals" :=
          recordMeritsSubmission_ok_phase s t "rebuttals" action.actor_role
            "plaintiff" "rebuttal" s.policy.max_rebuttal_chars true action.payload hSubmit
        rcases recordMeritsSubmission_with_materials_record_details
            s t "rebuttals" action.actor_role "plaintiff" "rebuttal"
            s.policy.max_rebuttal_chars action.payload hSubmit with
          ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
            _hOfferedValid, _hReportsValid, rfl⟩
        apply initializedRunInvariant_of_components req
        · exact stateWithCase_preserves_phaseShape s _
            (appendSupplementalMaterials_preserves_phaseShape _ offered reports
              (addRebuttal_preserves_phaseShape s.case (trimString rawText)
                hInvariant.procedure.phase_shape hPhase))
        · simpa [stateWithCase] using
            appendSupplementalMaterials_preserves_councilIdsUnique
              (addFiling s.case "rebuttals" "plaintiff" (trimString rawText))
              offered reports
              (addFiling_preserves_councilIdsUnique s.case "rebuttals" "plaintiff"
                (trimString rawText) hInvariant.procedure.council_ids_unique)
        · simpa [stateWithCase] using
            appendSupplementalMaterials_preserves_answerIntegrity
              (addFiling s.case "rebuttals" "plaintiff" (trimString rawText))
              offered reports
              (addFiling_preserves_answerIntegrity s.case "rebuttals" "plaintiff"
                (trimString rawText) hInvariant.procedure.answers)
        · exact hRecord
        · exact stateWithCase_appendMaterials_preserves_caseFrameMatches
            req.state.case.case_id req.state.case.caption (trimString req.question)
            req.state.policy req.state.evidence_catalog
            (councilMemberIdentities req.council_members) s "rebuttals" "plaintiff"
            (trimString rawText) offered reports hFrame
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hSubmit : recordMeritsSubmission s "surrebuttals" action.actor_role
              "defendant" "surrebuttal" s.policy.max_surrebuttal_chars true action.payload =
              .ok t := by
            simpa [stepCore, hSurrebuttal] using hStepCore
          have hPhase : s.case.phase = "surrebuttals" :=
            recordMeritsSubmission_ok_phase s t "surrebuttals" action.actor_role
              "defendant" "surrebuttal" s.policy.max_surrebuttal_chars true
              action.payload hSubmit
          rcases recordMeritsSubmission_with_materials_record_details
              s t "surrebuttals" action.actor_role "defendant" "surrebuttal"
              s.policy.max_surrebuttal_chars action.payload hSubmit with
            ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
              _hOfferedValid, _hReportsValid, rfl⟩
          apply initializedRunInvariant_of_components req
          · exact stateWithCase_preserves_phaseShape s _
              (appendSupplementalMaterials_preserves_phaseShape _ offered reports
                (addSurrebuttal_preserves_phaseShape s.case (trimString rawText)
                  hInvariant.procedure.phase_shape hPhase))
          · simpa [stateWithCase] using
              appendSupplementalMaterials_preserves_councilIdsUnique
                (addFiling s.case "surrebuttals" "defendant" (trimString rawText))
                offered reports
                (addFiling_preserves_councilIdsUnique s.case "surrebuttals" "defendant"
                  (trimString rawText) hInvariant.procedure.council_ids_unique)
          · simpa [stateWithCase] using
              appendSupplementalMaterials_preserves_answerIntegrity
                (addFiling s.case "surrebuttals" "defendant" (trimString rawText))
                offered reports
                (addFiling_preserves_answerIntegrity s.case "surrebuttals" "defendant"
                  (trimString rawText) hInvariant.procedure.answers)
          · exact hRecord
          · exact stateWithCase_appendMaterials_preserves_caseFrameMatches
              req.state.case.case_id req.state.case.caption (trimString req.question)
              req.state.policy req.state.evidence_catalog
              (councilMemberIdentities req.council_members) s "surrebuttals" "defendant"
              (trimString rawText) offered reports hFrame
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hEvidence] using hStepCore
            rcases submitEvidence_record_details s t action.actor_role action.payload hSubmit with
              ⟨item, _hOrigin, _hValid, rfl⟩
            apply initializedRunInvariant_of_components req
            · exact stateWithCase_preserves_phaseShape s _
                (appendSubmittedEvidence_preserves_phaseShape s.case item
                  hInvariant.procedure.phase_shape)
            · simpa [stateWithCase] using
                appendSubmittedEvidence_preserves_councilIdsUnique s.case item
                  hInvariant.procedure.council_ids_unique
            · simpa [stateWithCase] using
                appendSubmittedEvidence_preserves_answerIntegrity s.case item
                  hInvariant.procedure.answers
            · exact hRecord
            · exact stateWithCase_appendEvidence_preserves_caseFrameMatches
                req.state.case.case_id req.state.case.caption (trimString req.question)
                req.state.policy req.state.evidence_catalog
                (councilMemberIdentities req.council_members) s item hFrame
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · have hPhase : s.case.phase = "closings" := by
                by_cases hOpen : s.case.phase = "closings"
                · exact hOpen
                · have hClosed : s.case.phase != "closings" := by simpa using hOpen
                  simp [stepCore, hClosing, hClosed, Bind.bind, Except.bind] at hStepCore
              rcases step_deliver_closing_statement_result
                  s t action hClosing hStepCore with ⟨rawText, rfl⟩
              apply initializedRunInvariant_of_components req
              · exact stateWithCase_preserves_phaseShape s _
                  (addClosing_preserves_phaseShape s.case (trimString rawText)
                    hInvariant.procedure.phase_shape hPhase)
              · simpa [stateWithCase] using
                  addFiling_preserves_councilIdsUnique s.case "closings"
                    (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                    (trimString rawText) hInvariant.procedure.council_ids_unique
              · simpa [stateWithCase] using
                  addFiling_preserves_answerIntegrity s.case "closings"
                    (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                    (trimString rawText) hInvariant.procedure.answers
              · exact hRecord
              · exact stateWithCase_addFiling_preserves_caseFrameMatches
                  req.state.case.case_id req.state.case.caption (trimString req.question)
                  req.state.policy req.state.evidence_catalog
                  (councilMemberIdentities req.council_members) s "closings"
                  (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                  (trimString rawText) hFrame
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · have hSourcePhase :
                    s.case.phase = "rebuttals" ∨ s.case.phase = "surrebuttals" := by
                  by_cases hRebuttals : s.case.phase = "rebuttals"
                  · exact Or.inl hRebuttals
                  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
                    · exact Or.inr hSurrebuttals
                    · simp [stepCore, hPass, hRebuttals, hSurrebuttals] at hStepCore
                rcases step_pass_phase_opportunity_record_result
                    s t action hPass hStepCore with hResult | hResult
                · rw [hResult] at hRecord ⊢
                  apply initializedRunInvariant_of_components req
                  · apply stateWithCase_preserves_phaseShape
                    rcases hSourcePhase with hRebuttals | hSurrebuttals
                    · exact passRebuttal_preserves_phaseShape s.case
                        hInvariant.procedure.phase_shape hRebuttals
                    · simpa [phaseShape, hSurrebuttals] using
                        hInvariant.procedure.phase_shape
                  · simpa [stateWithCase] using
                      setPhase_preserves_councilIdsUnique s.case "surrebuttals"
                        hInvariant.procedure.council_ids_unique
                  · simpa [stateWithCase] using
                      setPhase_preserves_answerIntegrity s.case "surrebuttals"
                        hInvariant.procedure.answers
                  · exact hRecord
                  · exact stateWithCase_setPhase_preserves_caseFrameMatches
                      req.state.case.case_id req.state.case.caption (trimString req.question)
                      req.state.policy req.state.evidence_catalog
                      (councilMemberIdentities req.council_members) s "surrebuttals" hFrame
                · rw [hResult] at hRecord ⊢
                  apply initializedRunInvariant_of_components req
                  · apply stateWithCase_preserves_phaseShape
                    rcases hSourcePhase with hRebuttals | hSurrebuttals
                    · have hSurShape := passRebuttal_preserves_phaseShape s.case
                        hInvariant.procedure.phase_shape hRebuttals
                      simpa using passSurrebuttal_preserves_phaseShape
                        { s.case with phase := "surrebuttals" } hSurShape rfl
                    · exact passSurrebuttal_preserves_phaseShape s.case
                        hInvariant.procedure.phase_shape hSurrebuttals
                  · simpa [stateWithCase] using
                      setPhase_preserves_councilIdsUnique s.case "closings"
                        hInvariant.procedure.council_ids_unique
                  · simpa [stateWithCase] using
                      setPhase_preserves_answerIntegrity s.case "closings"
                        hInvariant.procedure.answers
                  · exact hRecord
                  · exact stateWithCase_setPhase_preserves_caseFrameMatches
                      req.state.case.case_id req.state.case.caption (trimString req.question)
                      req.state.policy req.state.evidence_catalog
                      (councilMemberIdentities req.council_members) s "closings" hFrame
              · by_cases hAnswer : action.action_type = "submit_council_answer"
                · have hCore :
                      (do
                        requireRole action.actor_role "council"
                        let memberId := trimString (← getString action.payload "member_id")
                        let answer := (← getNat action.payload "answer")
                        let rationale := trimString (← getString action.payload "rationale")
                        recordCouncilAnswer s memberId answer rationale) = .ok t := by
                    simpa [stepCore, hAnswer] using hStepCore
                  cases hRole : requireRole action.actor_role "council" with
                  | error error =>
                      rw [hRole] at hCore
                      simp at hCore
                      cases hCore
                  | ok value =>
                      cases value
                      simp only [hRole, Bind.bind, Except.bind] at hCore
                      cases hMember : getString action.payload "member_id" with
                      | error error =>
                          rw [hMember] at hCore
                          cases hCore
                      | ok rawMemberId =>
                          simp only [hMember] at hCore
                          cases hAnswerValue : getNat action.payload "answer" with
                          | error error =>
                              rw [hAnswerValue] at hCore
                              cases hCore
                          | ok answer =>
                              simp only [hAnswerValue] at hCore
                              cases hRationale : getString action.payload "rationale" with
                              | error error =>
                                  rw [hRationale] at hCore
                                  cases hCore
                              | ok rawRationale =>
                                  simp only [hRationale] at hCore
                                  have hLocal := recordCouncilAnswer_preserves_invariants
                                    req.state.case.case_id req.state.case.caption
                                    (trimString req.question) req.state.policy
                                    req.state.evidence_catalog
                                    (councilMemberIdentities req.council_members)
                                    s t (trimString rawMemberId) answer
                                    (trimString rawRationale)
                                    hInvariant.procedure.phase_shape
                                    hInvariant.procedure.council_ids_unique
                                    hInvariant.procedure.answers hFrame hCore
                                  exact initializedRunInvariant_of_components req t
                                    hLocal.1 hLocal.2.1 hLocal.2.2.1 hRecord hLocal.2.2.2
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · have hCore :
                        (do
                          requireRole action.actor_role "system"
                          let memberId := trimString (← getString action.payload "member_id")
                          let status := (← getString action.payload "status")
                          removeCouncilMember s memberId status) = .ok t := by
                      simpa [stepCore, hRemoval] using hStepCore
                    cases hRole : requireRole action.actor_role "system" with
                    | error error =>
                        rw [hRole] at hCore
                        cases hCore
                    | ok value =>
                        cases value
                        simp only [hRole, Bind.bind, Except.bind] at hCore
                        cases hMember : getString action.payload "member_id" with
                        | error error =>
                            rw [hMember] at hCore
                            cases hCore
                        | ok rawMemberId =>
                            simp only [hMember] at hCore
                            cases hStatus : getString action.payload "status" with
                            | error error =>
                                rw [hStatus] at hCore
                                cases hCore
                            | ok status =>
                                simp only [hStatus] at hCore
                                have hLocal := removeCouncilMember_preserves_invariants
                                  req.state.case.case_id req.state.case.caption
                                  (trimString req.question) req.state.policy
                                  req.state.evidence_catalog
                                  (councilMemberIdentities req.council_members)
                                  s t (trimString rawMemberId) status
                                  hInvariant.procedure.phase_shape
                                  hInvariant.procedure.council_ids_unique
                                  hInvariant.procedure.answers hFrame hCore
                                exact initializedRunInvariant_of_components req t
                                  hLocal.1 hLocal.2.1 hLocal.2.2.1 hRecord hLocal.2.2.2
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hCore :
                          (do
                            requireRole action.actor_role "system"
                            failOpportunity s action.payload) = .ok t := by
                        simpa [stepCore, hFail] using hStepCore
                      cases hRole : requireRole action.actor_role "system" with
                      | error error =>
                          rw [hRole] at hCore
                          cases hCore
                      | ok value =>
                          cases value
                          simp only [hRole, Bind.bind, Except.bind] at hCore
                          have hLocal := failOpportunity_preserves_invariants
                            req.state.case.case_id req.state.case.caption
                            (trimString req.question) req.state.policy
                            req.state.evidence_catalog
                            (councilMemberIdentities req.council_members)
                            s t action.payload hInvariant.procedure.phase_shape
                            hInvariant.procedure.council_ids_unique
                            hInvariant.procedure.answers hFrame hCore
                          exact initializedRunInvariant_of_components req t
                            hLocal.1 hLocal.2.1 hLocal.2.2.1 hRecord hLocal.2.2.2
                    · simp [stepCore] at hStepCore

theorem initializedRun_reachable_invariant
    (req : InitializeCaseRequest)
    (start target : ArbitrationState)
    (hInit : initializeCase req = .ok start)
    (hRun : StepReachableFrom start target) :
    InitializedRunInvariant req target := by
  induction hRun with
  | refl => exact initializeCase_establishes_runInvariant req start hInit
  | step s t action _hReachable hStep ih =>
      exact step_preserves_runInvariant req s t action ih hStep

end ArbdProofs
