import Proofs.NoStuck
import Proofs.ReachableMaterialLimits
import Proofs.Samples

namespace ArbProofs

structure TextLimitsPositive (p : ArbitrationPolicy) : Prop where
  opening : 0 < p.max_opening_chars
  argument : 0 < p.max_argument_chars
  rebuttal : 0 < p.max_rebuttal_chars
  surrebuttal : 0 < p.max_surrebuttal_chars
  closing : 0 < p.max_closing_chars

def councilMemberIdsCanonicalList (members : List CouncilMember) : Prop :=
  ∀ member ∈ members, trimString member.member_id = member.member_id ∧ member.member_id ≠ ""

def councilMemberIdsCanonical (c : ArbitrationCase) : Prop :=
  councilMemberIdsCanonicalList c.council_members

theorem trimString_x :
    trimString "x" = "x" := by
  native_decide

theorem trimString_plaintiff :
    trimString "plaintiff" = "plaintiff" := by
  native_decide

theorem trimString_defendant :
    trimString "defendant" = "defendant" := by
  native_decide

theorem trimString_council :
    trimString "council" = "council" := by
  native_decide

theorem trimString_demonstrated :
    trimString "demonstrated" = "demonstrated" := by
  native_decide

theorem trimString_empty :
    trimString "" = "" := by
  native_decide

theorem selectedPartyRole_trim (b : Bool) :
    trimString (if b then "plaintiff" else "defendant") =
      (if b then "plaintiff" else "defendant") := by
  cases b <;> simp [trimString_plaintiff, trimString_defendant]

theorem requireTextWithinLimit_x
    (label : String)
    (limit : Nat)
    (hLimit : 0 < limit) :
    requireTextWithinLimit label "x" limit = .ok PUnit.unit := by
  unfold requireTextWithinLimit
  have hLen : "x".length = 1 := by
    native_decide
  have hNotOver : ¬ "x".length > limit := by
    rw [hLen]
    omega
  simp [trimString_x, hNotOver]
  rfl

theorem requireRole_self_of_trim
    (role : String)
    (hTrim : trimString role = role) :
    requireRole role role = .ok PUnit.unit := by
  unfold requireRole
  simp [hTrim]
  rfl

theorem requireRole_selectedParty
    (b : Bool) :
    requireRole (if b then "plaintiff" else "defendant")
      (if b then "plaintiff" else "defendant") = .ok PUnit.unit := by
  exact requireRole_self_of_trim
    (if b then "plaintiff" else "defendant")
    (selectedPartyRole_trim b)

theorem requireRole_selectedPartyProp
    (p : Prop)
    [Decidable p] :
    requireRole (if p then "plaintiff" else "defendant")
      (if p then "plaintiff" else "defendant") = .ok PUnit.unit := by
  by_cases hp : p
  · simp [hp, requireRole_self_of_trim, trimString_plaintiff]
  · simp [hp, requireRole_self_of_trim, trimString_defendant]

theorem requireRole_council :
    requireRole "council" "council" = .ok PUnit.unit := by
  exact requireRole_self_of_trim "council" trimString_council

theorem getString_textPayload
    (text : String) :
    getString (textPayload text) "text" = .ok text := by
  rfl

theorem getString_meritsPayload
    (text : String) :
    getString (meritsPayload text) "text" = .ok text := by
  rfl

theorem parseOfferedEvidence_meritsPayload
    (text phase role : String) :
    parseOfferedEvidence (meritsPayload text) phase role = .ok [] := by
  rfl

theorem parseTechnicalReports_meritsPayload
    (text phase role : String) :
    parseTechnicalReports (meritsPayload text) phase role = .ok [] := by
  rfl

theorem getOptionalArray_textPayload_offered
    (text : String) :
    getOptionalArray (textPayload text) "offered_evidence" = .ok [] := by
  rfl

theorem getOptionalArray_textPayload_reports
    (text : String) :
    getOptionalArray (textPayload text) "technical_reports" = .ok [] := by
  rfl

theorem getString_councilVoteJson_member
    (memberId vote rationale : String) :
    getString
      (Lean.Json.mkObj
        [ ("member_id", Lean.Json.str memberId)
        , ("vote", Lean.Json.str vote)
        , ("rationale", Lean.Json.str rationale)
        ])
      "member_id" = .ok memberId := by
  rfl

theorem getString_councilVoteJson_vote
    (memberId vote rationale : String) :
    getString
      (Lean.Json.mkObj
        [ ("member_id", Lean.Json.str memberId)
        , ("vote", Lean.Json.str vote)
        , ("rationale", Lean.Json.str rationale)
        ])
      "vote" = .ok vote := by
  rfl

theorem getOptionalString_councilVoteJson_rationale
    (memberId vote rationale : String) :
    getOptionalString
      (Lean.Json.mkObj
        [ ("member_id", Lean.Json.str memberId)
        , ("vote", Lean.Json.str vote)
        , ("rationale", Lean.Json.str rationale)
        ])
      "rationale" = trimString rationale := by
  rfl

theorem validatePolicy_ok_implies_textLimitsPositive
    (p : ArbitrationPolicy)
    (hPolicy : validatePolicy p = .ok PUnit.unit) :
    TextLimitsPositive p := by
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
      · by_cases hVotesTooHigh : p.required_votes_for_decision > p.council_size
        · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh] at hPolicy
          cases hPolicy
        · by_cases hNonStrict : 2 * p.required_votes_for_decision ≤ p.council_size
          · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict] at hPolicy
            cases hPolicy
          · by_cases hRounds : p.max_deliberation_rounds = 0
            · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict, hRounds] at hPolicy
              cases hPolicy
            · by_cases hOpening : p.max_opening_chars = 0
              · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict, hRounds,
                  hOpening] at hPolicy
                cases hPolicy
              · by_cases hArgument : p.max_argument_chars = 0
                · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict, hRounds,
                    hOpening, hArgument] at hPolicy
                  cases hPolicy
                · by_cases hRebuttal : p.max_rebuttal_chars = 0
                  · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict, hRounds,
                      hOpening, hArgument, hRebuttal] at hPolicy
                    cases hPolicy
                  · by_cases hSurrebuttal : p.max_surrebuttal_chars = 0
                    · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict,
                        hRounds, hOpening, hArgument, hRebuttal, hSurrebuttal] at hPolicy
                      cases hPolicy
                    · by_cases hClosing : p.max_closing_chars = 0
                      · simp [hCouncil, hEvidence, hVotes, hVotesTooHigh, hNonStrict,
                          hRounds, hOpening, hArgument, hRebuttal, hSurrebuttal,
                          hClosing] at hPolicy
                        cases hPolicy
                      · exact {
                          opening := Nat.pos_of_ne_zero hOpening
                          argument := Nat.pos_of_ne_zero hArgument
                          rebuttal := Nat.pos_of_ne_zero hRebuttal
                          surrebuttal := Nat.pos_of_ne_zero hSurrebuttal
                          closing := Nat.pos_of_ne_zero hClosing
                        }

theorem initializeCase_validates_policy
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    validatePolicy req.state.policy = .ok PUnit.unit := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error err =>
      simp [hPolicy] at hInit
      cases hInit
  | ok okv =>
      cases okv
      rfl

theorem initializeCase_establishes_textLimitsPositive
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    TextLimitsPositive s.policy := by
  have hFrame := initializeCase_establishes_caseFrame req s hInit
  rcases hFrame with ⟨_hProp, hPolicyEq, _hMembers⟩
  have hValid := initializeCase_validates_policy req s hInit
  have hText := validatePolicy_ok_implies_textLimitsPositive req.state.policy hValid
  simpa [hPolicyEq] using hText

theorem step_preserves_textLimitsPositive
    (s t : ArbitrationState)
    (action : CourtAction)
    (hText : TextLimitsPositive s.policy)
    (hStep : step { state := s, action := action } = .ok t) :
    TextLimitsPositive t.policy := by
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
  simpa [hPolicyEq] using hText

theorem reachable_textLimitsPositive
    (s : ArbitrationState)
    (hs : Reachable s) :
    TextLimitsPositive s.policy := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_textLimitsPositive req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_textLimitsPositive s t action ih hStep

theorem hasInvalidCouncilMemberIds_false_implies_canonical
    (members : List CouncilMember)
    (hInvalid : hasInvalidCouncilMemberIds members = false) :
    councilMemberIdsCanonicalList members := by
  intro member hMem
  have hPredFalse :
      (trimString member.member_id = "" ||
        trimString member.member_id != member.member_id) = false := by
    cases hPred :
        (trimString member.member_id = "" ||
          trimString member.member_id != member.member_id) with
    | false =>
        rfl
    | true =>
        have hAnyTrue : hasInvalidCouncilMemberIds members = true := by
          unfold hasInvalidCouncilMemberIds
          exact List.any_eq_true.mpr ⟨member, hMem, hPred⟩
        rw [hInvalid] at hAnyTrue
        cases hAnyTrue
  have hTrim : trimString member.member_id = member.member_id := by
    by_cases hEq : trimString member.member_id = member.member_id
    · exact hEq
    · simp [hEq] at hPredFalse
  have hNonempty : member.member_id ≠ "" := by
    intro hEmpty
    have hTrimEmpty : trimString member.member_id = "" := by
      simpa [hEmpty] using hTrim
    simp [hTrimEmpty] at hPredFalse
  exact ⟨hTrim, hNonempty⟩

theorem councilMemberIdsCanonicalList_of_ids_eq
    {source target : List CouncilMember}
    (hSource : councilMemberIdsCanonicalList source)
    (hIds : councilMemberIds target = councilMemberIds source) :
    councilMemberIdsCanonicalList target := by
  intro member hMem
  have hTargetId : member.member_id ∈ councilMemberIds target := by
    simpa [councilMemberIds] using
      (List.mem_map.mpr ⟨member, hMem, rfl⟩ :
        member.member_id ∈ target.map (fun item => item.member_id))
  have hSourceId : member.member_id ∈ councilMemberIds source := by
    simpa [hIds] using hTargetId
  rcases (List.mem_map.mp (by simpa [councilMemberIds] using hSourceId)) with
    ⟨sourceMember, hSourceMem, hSourceMemberId⟩
  have hCanonical := hSource sourceMember hSourceMem
  constructor
  · simpa [hSourceMemberId] using hCanonical.1
  · intro hEmpty
    exact hCanonical.2 (hSourceMemberId.trans hEmpty)

theorem initializeCase_no_invalid_councilMemberIds
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    hasInvalidCouncilMemberIds req.council_members = false := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error err =>
      simp [hPolicy] at hInit
      cases hInit
  | ok okv =>
      cases okv
      by_cases hProposition : trimString req.proposition = ""
      · simp [hPolicy, hProposition] at hInit
        cases hInit
      · by_cases hEvidence : trimString req.state.policy.evidence_standard = ""
        · simp [hPolicy, hProposition, hEvidence] at hInit
          cases hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hProposition, hEvidence, hEmpty] at hInit
            cases hInit
          · by_cases hLength : req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength] at hInit
              cases hInit
            · cases hInvalid : hasInvalidCouncilMemberIds req.council_members with
              | false =>
                  rfl
              | true =>
                  simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid] at hInit
                  cases hInit

theorem initializeCase_establishes_councilMemberIdsCanonical
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    councilMemberIdsCanonical s.case := by
  have hFrame := initializeCase_establishes_caseFrame req s hInit
  rcases hFrame with ⟨_hProp, _hPolicy, hIds⟩
  have hInvalid := initializeCase_no_invalid_councilMemberIds req s hInit
  have hReqCanonical :=
    hasInvalidCouncilMemberIds_false_implies_canonical req.council_members hInvalid
  exact councilMemberIdsCanonicalList_of_ids_eq hReqCanonical hIds

theorem step_preserves_councilMemberIdsCanonical
    (s t : ArbitrationState)
    (action : CourtAction)
    (hCanonical : councilMemberIdsCanonical s.case)
    (hStep : step { state := s, action := action } = .ok t) :
    councilMemberIdsCanonical t.case := by
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
  rcases hFrame' with ⟨_hProp, _hPolicy, hIds⟩
  exact councilMemberIdsCanonicalList_of_ids_eq hCanonical hIds

theorem reachable_councilMemberIdsCanonical
    (s : ArbitrationState)
    (hs : Reachable s) :
    councilMemberIdsCanonical s.case := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_councilMemberIdsCanonical req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_councilMemberIdsCanonical s t action ih hStep

theorem currentRoundVotes_any_false_of_not_mem
    (c : ArbitrationCase)
    (memberId : String)
    (hFresh : memberId ∉ currentRoundVoteIds c) :
    (currentRoundVotes c).any (fun vote => vote.member_id = memberId) = false := by
  by_cases hAny : (currentRoundVotes c).any (fun vote => vote.member_id = memberId) = true
  · rcases List.any_eq_true.mp hAny with ⟨vote, hVoteMem, hVoteIdBool⟩
    have hVoteId : vote.member_id = memberId := of_decide_eq_true hVoteIdBool
    have hVoteIdMem : memberId ∈ currentRoundVoteIds c := by
      simpa [currentRoundVoteIds] using
        (List.mem_map.mpr ⟨vote, hVoteMem, hVoteId⟩ :
          memberId ∈ (currentRoundVotes c).map (fun item => item.member_id))
    exact False.elim (hFresh hVoteIdMem)
  · cases hBool : (currentRoundVotes c).any (fun vote => vote.member_id = memberId) with
    | false =>
        rfl
    | true =>
        simp [hBool] at hAny

theorem continueDeliberation_ok
    (s : ArbitrationState)
    (c : ArbitrationCase) :
    ∃ t, continueDeliberation s c = .ok t := by
  unfold continueDeliberation
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        exact ⟨stateWithCase s { c with status := "closed", phase := "closed", resolution := resolution },
          by
            simp [hRoundComplete]
            rfl⟩
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · exact ⟨stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" },
            by
              simp [hRoundComplete, hTooFew]
              rfl⟩
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · exact ⟨stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" },
              by
                simp [hRoundComplete, hTooFew, hLastRound]
                rfl⟩
          · exact ⟨stateWithCase s { c with deliberation_round := c.deliberation_round + 1 },
              by
                simp [hRoundComplete, hTooFew, hLastRound]
                rfl⟩
  · exact ⟨stateWithCase s c, by simp [hRoundComplete]; rfl⟩

theorem recordOpeningStatement_success
    (s : ArbitrationState)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "openings")
    (hLimit : 0 < s.policy.max_opening_chars) :
    ∃ action t, step { state := s, action := action } = .ok t := by
  let role := if s.case.openings = [] then "plaintiff" else "defendant"
  have hRole : requireRole role role = .ok PUnit.unit := by
    exact requireRole_selectedPartyProp (s.case.openings = [])
  have hText := requireTextWithinLimit_x "opening statement" s.policy.max_opening_chars hLimit
  refine ⟨openingAction role "x", stateWithCase s (addFiling s.case "openings" role "x"), ?_⟩
  simp [step, hStatus, stepCore, openingAction, hPhase, role, hRole,
    getString_textPayload, trimString_x, hText, Bind.bind, Except.bind,
    Except.pure, Pure.pure]

theorem submitArgument_success
    (s : ArbitrationState)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "arguments")
    (hMaterials : materialLimitsRespected s)
    (hLimit : 0 < s.policy.max_argument_chars) :
    ∃ action t, step { state := s, action := action } = .ok t := by
  let role := if s.case.arguments = [] then "plaintiff" else "defendant"
  have hRole : requireRole role role = .ok PUnit.unit := by
    exact requireRole_selectedPartyProp (s.case.arguments = [])
  have hText := requireTextWithinLimit_x "argument" s.policy.max_argument_chars hLimit
  have hOfferedCap :
      offeredEvidenceCountForRole s.case.offered_evidence role ≤
        s.policy.max_exhibits_per_side := by
    by_cases hEmpty : s.case.arguments = []
    · have hRoleEq : role = "plaintiff" := by
        simp [role, hEmpty]
      simpa [hRoleEq, offeredEvidenceCountForRole_eq_offeredCount] using hMaterials.1
    · have hRoleEq : role = "defendant" := by
        simp [role, hEmpty]
      simpa [hRoleEq, offeredEvidenceCountForRole_eq_offeredCount] using hMaterials.2.1
  have hReportsCap :
      technicalReportCountForRole s.case.technical_reports role ≤
        s.policy.max_reports_per_side := by
    by_cases hEmpty : s.case.arguments = []
    · have hRoleEq : role = "plaintiff" := by
        simp [role, hEmpty]
      simpa [hRoleEq, technicalReportCountForRole_eq_reportCount] using hMaterials.2.2.1
    · have hRoleEq : role = "defendant" := by
        simp [role, hEmpty]
      simpa [hRoleEq, technicalReportCountForRole_eq_reportCount] using hMaterials.2.2.2
  have hOfferedNotOver :
      ¬ s.policy.max_exhibits_per_side <
        offeredEvidenceCountForRole s.case.offered_evidence role := by
    omega
  have hReportsNotOver :
      ¬ s.policy.max_reports_per_side <
        technicalReportCountForRole s.case.technical_reports role := by
    omega
  refine ⟨argumentAction role "x",
    stateWithCase s (appendSupplementalMaterials (addFiling s.case "arguments" role "x") [] []),
    ?_⟩
  simp [step, hStatus, stepCore, argumentAction, recordMeritsSubmission, hPhase,
    role, hRole, getString_meritsPayload, trimString_x, hText,
    parseOfferedEvidence_meritsPayload, parseTechnicalReports_meritsPayload,
    requireCountWithinLimit, hOfferedNotOver, hReportsNotOver,
    appendSupplementalMaterials, Bind.bind, Except.bind,
    Except.pure, Pure.pure]

theorem passRebuttal_success
    (s : ArbitrationState)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "rebuttals")
    (hEmpty : s.case.rebuttals = []) :
    ∃ action t, step { state := s, action := action } = .ok t := by
  have hRole : requireRole "plaintiff" "plaintiff" = .ok PUnit.unit := by
    exact requireRole_self_of_trim "plaintiff" trimString_plaintiff
  refine ⟨passAction "plaintiff", stateWithCase s { s.case with phase := "surrebuttals" }, ?_⟩
  simp [step, hStatus, stepCore, passAction, hPhase, hEmpty, hRole,
    Except.pure, Bind.bind, Except.bind, Pure.pure]

theorem passSurrebuttal_success
    (s : ArbitrationState)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "surrebuttals")
    (hEmpty : s.case.surrebuttals = []) :
    ∃ action t, step { state := s, action := action } = .ok t := by
  have hRole : requireRole "defendant" "defendant" = .ok PUnit.unit := by
    exact requireRole_self_of_trim "defendant" trimString_defendant
  refine ⟨passAction "defendant", stateWithCase s { s.case with phase := "closings" }, ?_⟩
  simp [step, hStatus, stepCore, passAction, hPhase, hEmpty, hRole,
    Except.pure, Bind.bind, Except.bind, Pure.pure]

theorem deliverClosingStatement_success
    (s : ArbitrationState)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "closings")
    (hLimit : 0 < s.policy.max_closing_chars) :
    ∃ action t, step { state := s, action := action } = .ok t := by
  let role := if s.case.closings = [] then "plaintiff" else "defendant"
  have hRole : requireRole role role = .ok PUnit.unit := by
    exact requireRole_selectedPartyProp (s.case.closings = [])
  have hText := requireTextWithinLimit_x "closing statement" s.policy.max_closing_chars hLimit
  refine ⟨closingAction role "x", stateWithCase s (addFiling s.case "closings" role "x"), ?_⟩
  simp [step, hStatus, stepCore, closingAction, hPhase, role, hRole,
    getString_textPayload, trimString_x, hText, requireNoSupplementalMaterials,
    getOptionalArray_textPayload_offered, getOptionalArray_textPayload_reports,
    Bind.bind, Except.bind, Except.pure, Pure.pure]

theorem submitCouncilVote_success
    (s : ArbitrationState)
    (hs : Reachable s)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "deliberation") :
    ∃ action t, step { state := s, action := action } = .ok t := by
  rcases reachable_deliberation_has_nextCouncilMember s hs hPhase hStatus with
    ⟨member, hNext⟩
  have hFind :
      (seatedCouncilMembers s.case).find?
        (fun candidate =>
          !(currentRoundVotes s.case).any (fun vote => vote.member_id = candidate.member_id)) =
        some member := by
    simpa [nextCouncilMember?] using hNext
  have hMemberSeatedMem : member ∈ seatedCouncilMembers s.case := by
    exact List.mem_of_find?_eq_some hFind
  have hMemberMem : member ∈ s.case.council_members := by
    unfold seatedCouncilMembers at hMemberSeatedMem
    exact (List.mem_filter.mp hMemberSeatedMem).1
  have hMemberSeated : memberIsSeated member = true := by
    unfold seatedCouncilMembers at hMemberSeatedMem
    exact (List.mem_filter.mp hMemberSeatedMem).2
  have hCanonical := reachable_councilMemberIdsCanonical s hs member hMemberMem
  have hKnown :
      s.case.council_members.any (fun candidate => candidate.member_id = member.member_id) =
        true := by
    exact List.any_eq_true.mpr ⟨member, hMemberMem, by simp⟩
  have hSeated :
      s.case.council_members.any
        (fun candidate => candidate.member_id = member.member_id && memberIsSeated candidate) =
        true := by
    exact List.any_eq_true.mpr ⟨member, hMemberMem, by simp [hMemberSeated]⟩
  rcases nextCouncilMember_some_implies_seated_and_fresh s.case member hNext with
    ⟨_hSeatedId, hFresh⟩
  have hAlready :
      (currentRoundVotes s.case).any (fun vote => vote.member_id = member.member_id) = false := by
    exact currentRoundVotes_any_false_of_not_mem s.case member.member_id hFresh
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
      member_id := member.member_id
      round := s.case.deliberation_round
      vote := "demonstrated"
      rationale := ""
    } }
  rcases continueDeliberation_ok s c1 with ⟨t, hCont⟩
  have hRecord :
      recordCouncilVote s member.member_id "demonstrated" "" = .ok t := by
    unfold recordCouncilVote
    simp [hStatus, hPhase, hKnown, hSeated, trimString_demonstrated, trimString_empty,
      hAlready]
    simpa [c1, hStatus, hPhase, List.concat_eq_append] using hCont
  have hStepCore :
      stepCore
        { state := s
          action := councilVoteAction member.member_id "demonstrated" "" } = .ok t := by
    simp [stepCore, councilVoteAction, requireRole_council,
      getString_councilVoteJson_member, getString_councilVoteJson_vote,
      getOptionalString_councilVoteJson_rationale, hCanonical.1,
      trimString_demonstrated, trimString_empty, hRecord,
      Bind.bind, Except.bind]
  refine ⟨councilVoteAction member.member_id "demonstrated" "", t, ?_⟩
  simp [step, hStatus, hStepCore]

theorem reachable_active_has_successful_step
    (s : ArbitrationState)
    (hs : Reachable s)
    (hStatus : s.case.status = "active") :
    ∃ action t, step { state := s, action := action } = .ok t := by
  have hShape : phaseShape s.case := reachable_phaseShape s hs
  have hLimits := reachable_textLimitsPositive s hs
  have hMaterials := reachable_materialLimitsRespected s hs
  by_cases hOpenings : s.case.phase = "openings"
  · exact recordOpeningStatement_success s hStatus hOpenings hLimits.opening
  · by_cases hArguments : s.case.phase = "arguments"
    · exact submitArgument_success s hStatus hArguments hMaterials hLimits.argument
    · by_cases hRebuttals : s.case.phase = "rebuttals"
      · have hRebuttalShape :
            bilateralComplete "openings" s.case.openings ∧
              bilateralComplete "arguments" s.case.arguments ∧
              s.case.rebuttals = [] ∧
              s.case.surrebuttals = [] ∧
              s.case.closings = [] := by
          simpa [phaseShape, hRebuttals] using hShape
        exact passRebuttal_success s hStatus hRebuttals hRebuttalShape.2.2.1
      · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
        · have hSurrebuttalShape :
              bilateralComplete "openings" s.case.openings ∧
                bilateralComplete "arguments" s.case.arguments ∧
                plaintiffOptionalSequence "rebuttals" s.case.rebuttals ∧
                s.case.surrebuttals = [] ∧
                s.case.closings = [] := by
            simpa [phaseShape, hSurrebuttals] using hShape
          exact passSurrebuttal_success s hStatus hSurrebuttals hSurrebuttalShape.2.2.2.1
        · by_cases hClosings : s.case.phase = "closings"
          · exact deliverClosingStatement_success s hStatus hClosings hLimits.closing
          · by_cases hDeliberation : s.case.phase = "deliberation"
            · exact submitCouncilVote_success s hs hStatus hDeliberation
            · have hPhaseNotClosed : s.case.phase ≠ "closed" := by
                intro hClosed
                have hClosedStatus := reachable_phase_closed_implies_status_closed s hs hClosed
                rw [hClosedStatus] at hStatus
                simp at hStatus
              have hImpossible : False := by
                simp [phaseShape] at hShape
              exact False.elim hImpossible

end ArbProofs
