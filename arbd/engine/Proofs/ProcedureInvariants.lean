import Proofs.RecordIntegrity

namespace ArbdProofs

open List

def bilateralStarted (phase : String) : List Filing → Prop
  | [] => True
  | [plaintiff] => plaintiff.phase = phase ∧ plaintiff.role = "plaintiff"
  | _ => False

def bilateralComplete (phase : String) : List Filing → Prop
  | [plaintiff, defendant] =>
      plaintiff.phase = phase ∧ plaintiff.role = "plaintiff" ∧
        defendant.phase = phase ∧ defendant.role = "defendant"
  | _ => False

def plaintiffOptionalSequence (phase : String) : List Filing → Prop
  | [] => True
  | [plaintiff] => plaintiff.phase = phase ∧ plaintiff.role = "plaintiff"
  | _ => False

def defendantOptionalSequence (phase : String) : List Filing → Prop
  | [] => True
  | [defendant] => defendant.phase = phase ∧ defendant.role = "defendant"
  | _ => False

def phaseShape (c : ArbitrationCase) : Prop :=
  match c.phase with
  | "openings" =>
      bilateralStarted "openings" c.openings ∧
        c.arguments = [] ∧ c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = []
  | "arguments" =>
      bilateralComplete "openings" c.openings ∧
        bilateralStarted "arguments" c.arguments ∧
        c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = []
  | "rebuttals" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = []
  | "surrebuttals" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        c.surrebuttals = [] ∧ c.closings = []
  | "closings" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
        bilateralStarted "closings" c.closings
  | "deliberation" | "closed" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
        bilateralComplete "closings" c.closings
  | _ => False

def councilMemberIds (members : List CouncilMember) : List String :=
  members.map (fun member => member.member_id)

def councilMemberIdentities (members : List CouncilMember) : List (String × String × String) :=
  members.map (fun member => (member.member_id, member.model, member.persona_filename))

def councilIdsUnique (c : ArbitrationCase) : Prop :=
  (councilMemberIds c.council_members).Nodup

def currentRoundAnswerIds (c : ArbitrationCase) : List String :=
  (currentRoundAnswers c).map (fun answer => answer.member_id)

def seatedCouncilMemberIds (c : ArbitrationCase) : List String :=
  councilMemberIds (seatedCouncilMembers c)

def answerIntegrity (c : ArbitrationCase) : Prop :=
  (currentRoundAnswerIds c).Nodup ∧
    (∀ answer ∈ c.council_answers, answer.round = c.deliberation_round) ∧
    (∀ answer ∈ c.council_answers, answer.answer ≤ 100) ∧
    (∀ answer ∈ c.council_answers, answer.rationale ≠ "") ∧
    (∀ answer ∈ currentRoundAnswers c,
      answer.member_id ∈ seatedCouncilMemberIds c)

def caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState) : Prop :=
  s.case.case_id = caseId ∧
    s.case.caption = caption ∧
    s.case.question = question ∧
    s.policy = policy ∧
    s.evidence_catalog = catalog ∧
    councilMemberIdentities s.case.council_members = members

def caseFrameFieldsMatch (source target : ArbitrationCase) : Prop :=
  target.case_id = source.case_id ∧
    target.caption = source.caption ∧
    target.question = source.question ∧
    councilMemberIdentities target.council_members =
      councilMemberIdentities source.council_members

def initializedCaseFrame (req : InitializeCaseRequest) (s : ArbitrationState) : Prop :=
  caseFrameMatches
    req.state.case.case_id
    req.state.case.caption
    (trimString req.question)
    req.state.policy
    req.state.evidence_catalog
    (councilMemberIdentities req.council_members)
    s

def initializedCase (req : InitializeCaseRequest) : ArbitrationCase :=
  { req.state.case with
    question := trimString req.question
    council_members := req.council_members.map (fun member => { member with status := "seated" })
    status := "active"
    phase := "openings"
    openings := []
    arguments := []
    rebuttals := []
    surrebuttals := []
    closings := []
    offered_evidence := []
    technical_reports := []
    submitted_evidence := []
    deliberation_round := 1
    council_answers := []
    failure := none
  }

structure ProcedureInvariant (s : ArbitrationState) : Prop where
  phase_shape : phaseShape s.case
  council_ids_unique : councilIdsUnique s.case
  answers : answerIntegrity s.case
  record : RecordIntegrity s

structure InitializedRunInvariant
    (req : InitializeCaseRequest)
    (s : ArbitrationState) : Prop where
  procedure : ProcedureInvariant s
  frame : initializedCaseFrame req s

theorem hasDuplicateStrings_eq_true_of_not_nodup :
    ∀ {items : List String}, ¬ items.Nodup → hasDuplicateStrings items = true
  | [] => by
      intro h
      exact False.elim (h List.nodup_nil)
  | item :: rest => by
      intro h
      by_cases hMember : item ∈ rest
      · have hAny : rest.any (fun candidate => candidate = item) = true :=
          List.any_eq_true.mpr ⟨item, hMember, by simp⟩
        simp [hasDuplicateStrings, hAny]
      · have hRest : ¬ rest.Nodup := by
          intro hNodup
          exact h (List.nodup_cons.mpr ⟨hMember, hNodup⟩)
        simp [hasDuplicateStrings, hasDuplicateStrings_eq_true_of_not_nodup hRest]

theorem hasDuplicateStrings_eq_false_implies_nodup
    {items : List String}
    (hDuplicate : hasDuplicateStrings items = false) :
    items.Nodup := by
  by_cases hNodup : items.Nodup
  · exact hNodup
  · have hTrue := hasDuplicateStrings_eq_true_of_not_nodup hNodup
    rw [hDuplicate] at hTrue
    cases hTrue

theorem seatedCouncilMemberIds_sublist (c : ArbitrationCase) :
    seatedCouncilMemberIds c <+ councilMemberIds c.council_members := by
  simpa [seatedCouncilMemberIds, seatedCouncilMembers, councilMemberIds] using
    (show (List.filter memberIsSeated c.council_members).map (fun member => member.member_id) <+
        c.council_members.map (fun member => member.member_id) from
      (List.filter_sublist (p := memberIsSeated) (l := c.council_members)).map
        (fun member => member.member_id))

theorem seatedCouncilMemberIds_nodup
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c) :
    (seatedCouncilMemberIds c).Nodup := by
  exact hUnique.sublist (seatedCouncilMemberIds_sublist c)

theorem answerIntegrity_congr
    {c d : ArbitrationCase}
    (hAnswers : d.council_answers = c.council_answers)
    (hMembers : d.council_members = c.council_members)
    (hRound : d.deliberation_round = c.deliberation_round)
    (hIntegrity : answerIntegrity c) :
    answerIntegrity d := by
  unfold answerIntegrity currentRoundAnswerIds currentRoundAnswers
    seatedCouncilMemberIds seatedCouncilMembers councilMemberIds at *
  simpa [hAnswers, hMembers, hRound] using hIntegrity

theorem appendCurrentRoundAnswer_preserves_answerIntegrity
    (c : ArbitrationCase)
    (memberId : String)
    (answer : Nat)
    (rationale : String)
    (hIntegrity : answerIntegrity c)
    (hSeated : memberId ∈ seatedCouncilMemberIds c)
    (hFresh : memberId ∉ currentRoundAnswerIds c)
    (hBound : answer ≤ 100)
    (hRationale : trimString rationale ≠ "") :
    answerIntegrity
      { c with council_answers := c.council_answers.concat {
          member_id := memberId
          round := c.deliberation_round
          answer := answer
          rationale := trimString rationale
        } } := by
  let newAnswer : CouncilAnswer := {
    member_id := memberId
    round := c.deliberation_round
    answer := answer
    rationale := trimString rationale
  }
  have hCurrent :
      currentRoundAnswers
          { c with council_answers := c.council_answers.concat newAnswer } =
        currentRoundAnswers c ++ [newAnswer] := by
    simp [currentRoundAnswers, newAnswer, List.concat_eq_append]
  have hDistinct : (currentRoundAnswerIds c ++ [memberId]).Nodup := by
    rw [List.nodup_append]
    refine ⟨hIntegrity.1, by simp, ?_⟩
    intro left hLeft right hRight
    simp at hRight
    rcases hRight with rfl
    intro hEqual
    exact hFresh (hEqual ▸ hLeft)
  refine ⟨?_, ?_, ?_, ?_, ?_⟩
  · change
      (currentRoundAnswers
          { c with council_answers := c.council_answers.concat newAnswer }).map
        (fun stored => stored.member_id) |>.Nodup
    rw [hCurrent, List.map_append]
    simpa only [currentRoundAnswerIds, List.map_cons, List.map_nil] using hDistinct
  · intro stored hStored
    have hStored' : stored ∈ c.council_answers ++ [newAnswer] := by
      simpa [newAnswer, List.concat_eq_append] using hStored
    rcases List.mem_append.mp hStored' with hOld | hNew
    · exact hIntegrity.2.1 stored hOld
    · simp [newAnswer] at hNew
      rcases hNew with rfl
      rfl
  · intro stored hStored
    have hStored' : stored ∈ c.council_answers ++ [newAnswer] := by
      simpa [newAnswer, List.concat_eq_append] using hStored
    rcases List.mem_append.mp hStored' with hOld | hNew
    · exact hIntegrity.2.2.1 stored hOld
    · simp [newAnswer] at hNew
      rcases hNew with rfl
      exact hBound
  · intro stored hStored
    have hStored' : stored ∈ c.council_answers ++ [newAnswer] := by
      simpa [newAnswer, List.concat_eq_append] using hStored
    rcases List.mem_append.mp hStored' with hOld | hNew
    · exact hIntegrity.2.2.2.1 stored hOld
    · simp [newAnswer] at hNew
      rcases hNew with rfl
      exact hRationale
  · intro current hCurrentMember
    rw [hCurrent] at hCurrentMember
    rcases List.mem_append.mp hCurrentMember with hOld | hNew
    · exact hIntegrity.2.2.2.2 current hOld
    · simp [newAnswer] at hNew
      rcases hNew with rfl
      exact hSeated

theorem updateUnansweredCouncilMember_preserves_answerIntegrity
    (c : ArbitrationCase)
    (memberId : String)
    (update : CouncilMember → CouncilMember)
    (hIntegrity : answerIntegrity c)
    (hFresh : memberId ∉ currentRoundAnswerIds c)
    (hOther : ∀ member, member.member_id ≠ memberId → update member = member) :
    answerIntegrity
      { c with council_members := c.council_members.map update } := by
  have hOwnership :
      ∀ current ∈ currentRoundAnswers c,
        current.member_id ∈
          seatedCouncilMemberIds
            { c with council_members := c.council_members.map update } := by
    intro current hCurrent
    have hOldSeat := hIntegrity.2.2.2.2 current hCurrent
    have hNotTarget : current.member_id ≠ memberId := by
      intro hEqual
      apply hFresh
      have hMember : current.member_id ∈ currentRoundAnswerIds c := by
        simpa [currentRoundAnswerIds] using
          (show ∃ stored,
              stored ∈ currentRoundAnswers c ∧ stored.member_id = current.member_id from
            ⟨current, hCurrent, rfl⟩)
      simpa [hEqual] using hMember
    rcases (show ∃ member,
        member ∈ seatedCouncilMembers c ∧ member.member_id = current.member_id from
      by simpa [seatedCouncilMemberIds, councilMemberIds] using hOldSeat) with
      ⟨member, hMemberSeat, hMemberId⟩
    have hMemberMem : member ∈ c.council_members := (List.mem_filter.mp hMemberSeat).1
    have hMemberStillSeated : memberIsSeated member := (List.mem_filter.mp hMemberSeat).2
    have hMemberNe : member.member_id ≠ memberId := by
      simpa [hMemberId] using hNotTarget
    have hMapped : member ∈ c.council_members.map update := by
      apply List.mem_map.mpr
      exact ⟨member, hMemberMem, by simp [hOther member hMemberNe]⟩
    have hSeatNew :
        member ∈ seatedCouncilMembers
          { c with council_members := c.council_members.map update } := by
      exact List.mem_filter.mpr ⟨hMapped, hMemberStillSeated⟩
    simpa [seatedCouncilMemberIds, councilMemberIds] using
      (show ∃ candidate,
          candidate ∈ seatedCouncilMembers
            { c with council_members := c.council_members.map update } ∧
            candidate.member_id = current.member_id from
        ⟨member, hSeatNew, hMemberId⟩)
  refine ⟨?_, ?_, ?_, ?_, hOwnership⟩
  · simpa [currentRoundAnswerIds, currentRoundAnswers] using hIntegrity.1
  · simpa using hIntegrity.2.1
  · simpa using hIntegrity.2.2.1
  · simpa using hIntegrity.2.2.2.1

theorem councilMemberIds_map_preserving_ids
    (members : List CouncilMember)
    (update : CouncilMember → CouncilMember)
    (hId : ∀ member, (update member).member_id = member.member_id) :
    councilMemberIds (members.map update) = councilMemberIds members := by
  induction members with
  | nil => rfl
  | cons member rest ih =>
      change
        (update member).member_id :: councilMemberIds (rest.map update) =
          member.member_id :: councilMemberIds rest
      rw [hId member, ih]

theorem councilMemberIdentities_map_preserving_identity
    (members : List CouncilMember)
    (update : CouncilMember → CouncilMember)
    (hIdentity : ∀ member,
      ((update member).member_id, (update member).model,
        (update member).persona_filename) =
      (member.member_id, member.model, member.persona_filename)) :
    councilMemberIdentities (members.map update) =
      councilMemberIdentities members := by
  induction members with
  | nil => rfl
  | cons member rest ih =>
      change
        ((update member).member_id, (update member).model,
            (update member).persona_filename) ::
              councilMemberIdentities (rest.map update) =
          (member.member_id, member.model, member.persona_filename) ::
            councilMemberIdentities rest
      rw [hIdentity member, ih]

theorem councilIdsUnique_of_member_ids_eq
    {c d : ArbitrationCase}
    (hIds : councilMemberIds d.council_members =
      councilMemberIds c.council_members)
    (hUnique : councilIdsUnique c) :
    councilIdsUnique d := by
  simpa [councilIdsUnique, hIds] using hUnique

theorem stateWithCase_preserves_caseFrameMatches
    (caseId caption question : String)
    (policy : ArbitrationPolicy)
    (catalog : List EvidenceCommitment)
    (members : List (String × String × String))
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hFrame : caseFrameMatches caseId caption question policy catalog members s)
    (hCaseId : c.case_id = s.case.case_id)
    (hCaption : c.caption = s.case.caption)
    (hQuestion : c.question = s.case.question)
    (hMembers : councilMemberIdentities c.council_members =
      councilMemberIdentities s.case.council_members) :
    caseFrameMatches caseId caption question policy catalog members
      (stateWithCase s c) := by
  rcases hFrame with
    ⟨hOldCaseId, hOldCaption, hOldQuestion, hPolicy, hCatalog, hOldMembers⟩
  exact ⟨by simpa [stateWithCase, hCaseId] using hOldCaseId,
    by simpa [stateWithCase, hCaption] using hOldCaption,
    by simpa [stateWithCase, hQuestion] using hOldQuestion,
    by simpa [stateWithCase] using hPolicy,
    by simpa [stateWithCase] using hCatalog,
    by simpa [stateWithCase, hMembers] using hOldMembers⟩

theorem advanceAfterMerits_preserves_caseFrameFields
    (c : ArbitrationCase) :
    caseFrameFieldsMatch c (advanceAfterMerits c) := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 && c.phase = "openings"
  · simp [hOpen, caseFrameFieldsMatch]
  · by_cases hArgument : c.arguments.length >= 2 && c.phase = "arguments"
    · simp [hOpen, hArgument, caseFrameFieldsMatch]
    · by_cases hRebuttal : c.rebuttals.length >= 1 && c.phase = "rebuttals"
      · simp [hOpen, hArgument, hRebuttal, caseFrameFieldsMatch]
      · by_cases hSurrebuttal : c.surrebuttals.length >= 1 && c.phase = "surrebuttals"
        · simp [hOpen, hArgument, hRebuttal, hSurrebuttal, caseFrameFieldsMatch]
        · by_cases hClosing : c.closings.length >= 2 && c.phase = "closings"
          · simp [hOpen, hArgument, hRebuttal, hSurrebuttal, hClosing,
              caseFrameFieldsMatch]
          · simp [hOpen, hArgument, hRebuttal, hSurrebuttal, hClosing,
              caseFrameFieldsMatch]

theorem addFiling_preserves_caseFrameFields
    (c : ArbitrationCase)
    (phase role text : String) :
    caseFrameFieldsMatch c (addFiling c phase role text) := by
  unfold addFiling
  split <;> simpa [caseFrameFieldsMatch] using
    advanceAfterMerits_preserves_caseFrameFields _

theorem appendSupplementalMaterials_preserves_caseFrameFields
    (source c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hFields : caseFrameFieldsMatch source c) :
    caseFrameFieldsMatch source (appendSupplementalMaterials c offered reports) := by
  simpa [caseFrameFieldsMatch, appendSupplementalMaterials] using hFields

theorem appendSubmittedEvidence_preserves_caseFrameFields
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence) :
    caseFrameFieldsMatch c (appendSubmittedEvidence c evidence) := by
  simp [caseFrameFieldsMatch, appendSubmittedEvidence]

theorem phaseShape_closed_merits_complete
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hClosed : c.phase = "closed") :
    bilateralComplete "openings" c.openings ∧
      bilateralComplete "arguments" c.arguments ∧
      plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
      defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
      bilateralComplete "closings" c.closings := by
  simpa [phaseShape, hClosed] using hShape

end ArbdProofs
