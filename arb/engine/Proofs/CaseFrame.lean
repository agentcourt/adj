import Proofs.StepPreservation

namespace ArbProofs

/-
This file proves that initialization fixes the identity of the case.

The current global invariants say that reachable states preserve structural
properties: phase order, side-to-side parity, and aggregate material limits.
Those theorems do not yet say whether the engine might quietly change the case
being adjudicated.

That risk has three parts.

First, the proposition should remain fixed.  The engine should not begin by
adjudicating one claim and then silently switch to another.

Second, the governing policy should remain fixed.  The vote threshold,
character limits, and material caps should not drift after initialization.

Third, the set of council identities should remain fixed.  Member status may
change in deliberation, but the case should not replace one decision maker with
another mid-run.

These three fields form the case frame.  The theorems below show that a
successful initialization establishes that frame and that every successful
public `step` preserves it.
-/

theorem advanceAfterMerits_preserves_proposition
    (c : ArbitrationCase) :
    (advanceAfterMerits c).proposition = c.proposition := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 && c.phase = "openings"
  · simp [hOpen]
  · by_cases hArg : c.arguments.length >= 2 && c.phase = "arguments"
    · simp [hOpen, hArg]
    · by_cases hRebuttal : c.rebuttals.length >= 1 && c.phase = "rebuttals"
      · simp [hOpen, hArg, hRebuttal]
      · by_cases hSurrebuttal : c.surrebuttals.length >= 1 && c.phase = "surrebuttals"
        · simp [hOpen, hArg, hRebuttal, hSurrebuttal]
        · by_cases hClosing : c.closings.length >= 2 && c.phase = "closings"
          · simp [hOpen, hArg, hRebuttal, hSurrebuttal, hClosing]
          · simp [hOpen, hArg, hRebuttal, hSurrebuttal, hClosing]

theorem advanceAfterMerits_preserves_councilMemberIds
    (c : ArbitrationCase) :
    councilMemberIds (advanceAfterMerits c).council_members =
      councilMemberIds c.council_members := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 && c.phase = "openings"
  · simp [hOpen, councilMemberIds]
  · by_cases hArg : c.arguments.length >= 2 && c.phase = "arguments"
    · simp [hOpen, hArg, councilMemberIds]
    · by_cases hRebuttal : c.rebuttals.length >= 1 && c.phase = "rebuttals"
      · simp [hOpen, hArg, hRebuttal, councilMemberIds]
      · by_cases hSurrebuttal : c.surrebuttals.length >= 1 && c.phase = "surrebuttals"
        · simp [hOpen, hArg, hRebuttal, hSurrebuttal, councilMemberIds]
        · by_cases hClosing : c.closings.length >= 2 && c.phase = "closings"
          · simp [hOpen, hArg, hRebuttal, hSurrebuttal, hClosing, councilMemberIds]
          · simp [hOpen, hArg, hRebuttal, hSurrebuttal, hClosing, councilMemberIds]

theorem addFiling_preserves_proposition
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).proposition = c.proposition := by
  by_cases hOpenings : phase = "openings"
  · subst hOpenings
    let filing : Filing := { phase := "openings", role := role, text := text }
    let c1 : ArbitrationCase := { c with openings := c.openings.concat filing }
    simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_proposition c1
  · by_cases hArguments : phase = "arguments"
    · subst hArguments
      let filing : Filing := { phase := "arguments", role := role, text := text }
      let c1 : ArbitrationCase := { c with arguments := c.arguments.concat filing }
      simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_proposition c1
    · by_cases hRebuttals : phase = "rebuttals"
      · subst hRebuttals
        let filing : Filing := { phase := "rebuttals", role := role, text := text }
        let c1 : ArbitrationCase := { c with rebuttals := c.rebuttals.concat filing }
        simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_proposition c1
      · by_cases hSurrebuttals : phase = "surrebuttals"
        · subst hSurrebuttals
          let filing : Filing := { phase := "surrebuttals", role := role, text := text }
          let c1 : ArbitrationCase := { c with surrebuttals := c.surrebuttals.concat filing }
          simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_proposition c1
        · by_cases hClosings : phase = "closings"
          · subst hClosings
            let filing : Filing := { phase := "closings", role := role, text := text }
            let c1 : ArbitrationCase := { c with closings := c.closings.concat filing }
            simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_proposition c1
          · simpa [addFiling, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]
              using advanceAfterMerits_preserves_proposition c

theorem addFiling_preserves_councilMemberIds
    (c : ArbitrationCase)
    (phase role text : String) :
    councilMemberIds (addFiling c phase role text).council_members =
      councilMemberIds c.council_members := by
  by_cases hOpenings : phase = "openings"
  · subst hOpenings
    let filing : Filing := { phase := "openings", role := role, text := text }
    let c1 : ArbitrationCase := { c with openings := c.openings.concat filing }
    simpa [addFiling, filing, c1, councilMemberIds]
      using advanceAfterMerits_preserves_councilMemberIds c1
  · by_cases hArguments : phase = "arguments"
    · subst hArguments
      let filing : Filing := { phase := "arguments", role := role, text := text }
      let c1 : ArbitrationCase := { c with arguments := c.arguments.concat filing }
      simpa [addFiling, filing, c1, councilMemberIds]
        using advanceAfterMerits_preserves_councilMemberIds c1
    · by_cases hRebuttals : phase = "rebuttals"
      · subst hRebuttals
        let filing : Filing := { phase := "rebuttals", role := role, text := text }
        let c1 : ArbitrationCase := { c with rebuttals := c.rebuttals.concat filing }
        simpa [addFiling, filing, c1, councilMemberIds]
          using advanceAfterMerits_preserves_councilMemberIds c1
      · by_cases hSurrebuttals : phase = "surrebuttals"
        · subst hSurrebuttals
          let filing : Filing := { phase := "surrebuttals", role := role, text := text }
          let c1 : ArbitrationCase := { c with surrebuttals := c.surrebuttals.concat filing }
          simpa [addFiling, filing, c1, councilMemberIds]
            using advanceAfterMerits_preserves_councilMemberIds c1
        · by_cases hClosings : phase = "closings"
          · subst hClosings
            let filing : Filing := { phase := "closings", role := role, text := text }
            let c1 : ArbitrationCase := { c with closings := c.closings.concat filing }
            simpa [addFiling, filing, c1, councilMemberIds]
              using advanceAfterMerits_preserves_councilMemberIds c1
          · simpa [addFiling, councilMemberIds, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]
              using advanceAfterMerits_preserves_councilMemberIds c

theorem appendSupplementalMaterials_preserves_proposition
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport) :
    (appendSupplementalMaterials c offered reports).proposition = c.proposition := by
  simp [appendSupplementalMaterials]

theorem appendSupplementalMaterials_preserves_councilMemberIds
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport) :
    councilMemberIds (appendSupplementalMaterials c offered reports).council_members =
      councilMemberIds c.council_members := by
  simp [appendSupplementalMaterials, councilMemberIds]

theorem appendSubmittedEvidence_preserves_proposition
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence) :
    (appendSubmittedEvidence c evidence).proposition = c.proposition := by
  simp [appendSubmittedEvidence]

theorem appendSubmittedEvidence_preserves_councilMemberIds
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence) :
    councilMemberIds (appendSubmittedEvidence c evidence).council_members =
      councilMemberIds c.council_members := by
  simp [appendSubmittedEvidence, councilMemberIds]

theorem councilMemberIds_failure_update
    (members : List CouncilMember)
    (memberId reason opportunityId message : String) :
    councilMemberIds
      (List.map (fun (member : CouncilMember) =>
        if member.member_id = memberId then
          { member with
            status := "failed"
            failure_reason := reason
            failure_opportunity_id := opportunityId
            failure_message := message }
        else
          member) members) =
      councilMemberIds members := by
  unfold councilMemberIds
  induction members with
  | nil =>
      simp
  | cons head tail ih =>
      by_cases h : head.member_id = memberId
      · simp [h, ih]
      · simp [h, ih]

theorem stateWithCase_preserves_caseFrame
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hProp : c.proposition = s.case.proposition)
    (hMembers : councilMemberIds c.council_members = councilMemberIds s.case.council_members) :
    caseFrameMatches proposition policy memberIds (stateWithCase s c) := by
  rcases hFrame with ⟨hStateProp, hStatePolicy, hStateMembers⟩
  constructor
  · simp [stateWithCase, hProp, hStateProp]
  constructor
  · simp [stateWithCase, hStatePolicy]
  · simp [stateWithCase, hMembers, hStateMembers]

/--
Successful initialization fixes the initial case frame.

Initialization trims and stores the proposition, copies the policy from the
draft state, and reseats the supplied council list without changing member
identifiers.
-/
theorem initializeCase_establishes_caseFrame
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    caseFrameMatches
      (trimString req.proposition)
      req.state.policy
      (councilMemberIds req.council_members)
      s := by
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
            · by_cases hInvalid : hasInvalidCouncilMemberIds req.council_members
              · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid] at hInit
                cases hInit
              · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate] at hInit
                  cases hInit
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate, stateWithCase] at hInit
                  cases hInit
                  simp [caseFrameMatches, councilMemberIds]

theorem continueDeliberation_preserves_caseFrame_for
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hProp : c.proposition = s.case.proposition)
    (hMembers : councilMemberIds c.council_members = councilMemberIds s.case.council_members)
    (hCont : continueDeliberation s c = .ok t) :
    caseFrameMatches proposition policy memberIds t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame
          (by simp [hProp])
          (by simpa [councilMemberIds] using hMembers)
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame
            (by simp [hProp])
            (by simpa [councilMemberIds] using hMembers)
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame
              (by simp [hProp])
              (by simpa [councilMemberIds] using hMembers)
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame
              (by simp [hProp])
              (by simpa [councilMemberIds] using hMembers)
  · simp [hRoundComplete] at hCont
    cases hCont
    exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame
      (by simp [hProp])
      (by simpa [councilMemberIds] using hMembers)

theorem step_pass_phase_opportunity_preserves_caseFrame
    (s t : ArbitrationState)
    (action : CourtAction)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hType : action.action_type = "pass_phase_opportunity")
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    caseFrameMatches proposition policy memberIds t := by
  rcases hFrame with ⟨hProp, hPolicy, hMembers⟩
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · have hPass :
        (do
          requireRole action.actor_role "plaintiff"
          if !s.case.rebuttals.isEmpty then
            throw "rebuttal already submitted"
          pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
      simpa [stepCore, hType, hRebuttals] using hStep
    cases hRole : requireRole action.actor_role "plaintiff" with
    | error err =>
        rw [hRole] at hPass
        simp at hPass
        cases hPass
    | ok okv =>
        cases okv
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            have hFrame' : caseFrameMatches proposition policy memberIds s :=
              ⟨hProp, hPolicy, hMembers⟩
            exact stateWithCase_preserves_caseFrame s
              { s.case with phase := "surrebuttals" }
              proposition policy memberIds
              hFrame'
              (by simp)
              (by simp [councilMemberIds])
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · have hPass :
          (do
            requireRole action.actor_role "defendant"
            if !s.case.surrebuttals.isEmpty then
              throw "surrebuttal already submitted"
            pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
        simpa [stepCore, hType, hRebuttals, hSurrebuttals] using hStep
      cases hRole : requireRole action.actor_role "defendant" with
      | error err =>
          rw [hRole] at hPass
          simp at hPass
          cases hPass
      | ok okv =>
          cases okv
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              have hFrame' : caseFrameMatches proposition policy memberIds s :=
                ⟨hProp, hPolicy, hMembers⟩
              exact stateWithCase_preserves_caseFrame s
                { s.case with phase := "closings" }
                proposition policy memberIds
                hFrame'
                (by simp)
                (by simp [councilMemberIds])
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem step_submit_evidence_preserves_caseFrame
    (s t : ArbitrationState)
    (action : CourtAction)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hType : action.action_type = "submit_evidence")
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    caseFrameMatches proposition policy memberIds t := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame
    (appendSubmittedEvidence_preserves_proposition s.case evidence)
    (appendSubmittedEvidence_preserves_councilMemberIds s.case evidence)

theorem failOpportunity_preserves_caseFrame
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hFail : failOpportunity s payload = .ok t) :
    caseFrameMatches proposition policy memberIds t := by
  rcases failOpportunity_result s t payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨memberId, reason, opportunityId, message, c1, hC1,
      _hDelib, _hSeated, _hFresh, hCont⟩
    exact continueDeliberation_preserves_caseFrame_for s t c1 proposition policy memberIds
      hFrame
      (by rw [hC1])
      (by
        rw [hC1]
        exact councilMemberIds_failure_update s.case.council_members memberId reason
          opportunityId message)
      hCont
  · rcases hParty with ⟨_failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact stateWithCase_preserves_caseFrame s _ proposition policy memberIds hFrame rfl rfl

theorem step_fail_opportunity_preserves_caseFrame
    (s t : ArbitrationState)
    (action : CourtAction)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hType : action.action_type = "fail_opportunity")
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    caseFrameMatches proposition policy memberIds t := by
  have hStep' :
      (do
        requireRole action.actor_role "system"
        failOpportunity s action.payload) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "system" with
  | error err =>
      rw [hRole] at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      exact failOpportunity_preserves_caseFrame s t action.payload proposition policy memberIds
        hFrame hStep'

/--
Every successful public step preserves the case frame.

The proof follows the public action boundary rather than the internal helper
functions.  That is the right level for the invariant, because `Reachable` is
defined in terms of successful public `step` calls.
-/
theorem step_preserves_caseFrame
    (s t : ArbitrationState)
    (action : CourtAction)
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (hFrame : caseFrameMatches proposition policy memberIds s)
    (hStep : step { state := s, action := action } = .ok t) :
    caseFrameMatches proposition policy memberIds t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with ⟨rawText, rfl⟩
    exact stateWithCase_preserves_caseFrame s _
      proposition policy memberIds hFrame
      (addFiling_preserves_proposition s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
      (addFiling_preserves_councilMemberIds s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
  · by_cases hArgument : action.action_type = "submit_argument"
    · let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
      have hSubmit :
          recordMeritsSubmission
            s
            "arguments"
            action.actor_role
            role
            "argument"
            s.policy.max_argument_chars
            true
            action.payload = .ok t := by
        simpa [stepCore, hArgument, role] using hStepCore
      rcases recordMeritsSubmission_with_materials_result
          s t "arguments" action.actor_role role
          "argument" s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, rfl⟩
      exact stateWithCase_preserves_caseFrame s _
        proposition policy memberIds hFrame
        (by
          simp [appendSupplementalMaterials_preserves_proposition,
            addFiling_preserves_proposition])
        (by
          simp [appendSupplementalMaterials_preserves_councilMemberIds,
            addFiling_preserves_councilMemberIds])
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hSubmit :
            recordMeritsSubmission
              s
              "rebuttals"
              action.actor_role
              "plaintiff"
              "rebuttal"
              s.policy.max_rebuttal_chars
              true
              action.payload = .ok t := by
          simpa [stepCore, hRebuttal] using hStepCore
        rcases recordMeritsSubmission_with_materials_result
            s t "rebuttals" action.actor_role "plaintiff"
            "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
          ⟨rawText, offered, reports, rfl⟩
        exact stateWithCase_preserves_caseFrame s _
          proposition policy memberIds hFrame
          (by
            simp [appendSupplementalMaterials_preserves_proposition,
              addFiling_preserves_proposition])
          (by
            simp [appendSupplementalMaterials_preserves_councilMemberIds,
              addFiling_preserves_councilMemberIds])
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hSubmit :
              recordMeritsSubmission
                s
                "surrebuttals"
                action.actor_role
                "defendant"
                "surrebuttal"
                s.policy.max_surrebuttal_chars
                true
                action.payload = .ok t := by
            simpa [stepCore, hSurrebuttal] using hStepCore
          rcases recordMeritsSubmission_with_materials_result
              s t "surrebuttals" action.actor_role "defendant"
              "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
            ⟨rawText, offered, reports, rfl⟩
          exact stateWithCase_preserves_caseFrame s _
            proposition policy memberIds hFrame
            (by
              simp [appendSupplementalMaterials_preserves_proposition,
                addFiling_preserves_proposition])
            (by
              simp [appendSupplementalMaterials_preserves_councilMemberIds,
                addFiling_preserves_councilMemberIds])
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with ⟨rawText, rfl⟩
            exact stateWithCase_preserves_caseFrame s _
              proposition policy memberIds hFrame
              (addFiling_preserves_proposition s.case "closings"
                (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                (trimString rawText))
              (addFiling_preserves_councilMemberIds s.case "closings"
                (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                (trimString rawText))
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact step_pass_phase_opportunity_preserves_caseFrame
                s t action proposition policy memberIds hPass hFrame hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact step_submit_evidence_preserves_caseFrame
                  s t action proposition policy memberIds hEvidence hFrame hStepCore
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                  ⟨memberId, vote, rationale, _hPhase, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  exact continueDeliberation_preserves_caseFrame_for s t c1
                    proposition policy memberIds hFrame
                    (by simp [c1])
                    (by simp [c1, councilMemberIds])
                    hCont
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemoval hStepCore with
                    ⟨memberId, status, _hPhase, hCont⟩
                    let c1 := {
                      s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                        if member.member_id = memberId then
                          { member with status := trimString status }
                        else
                          member)
                    }
                    exact continueDeliberation_preserves_caseFrame_for s t c1
                      proposition policy memberIds hFrame
                      (by simp [c1])
                      (by
                        simpa [c1] using
                          councilMemberIds_status_update s.case.council_members memberId (trimString status))
                      hCont
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_preserves_caseFrame s t action
                        proposition policy memberIds hFail hFrame hStepCore
                    · simp [stepCore] at hStepCore

/--
Any run that begins at a successful initialization preserves the initialized
case frame.

This theorem is the public statement of the file.  It connects the
initialization request to every later state in the same successful run.
-/
theorem initialized_run_preserves_caseFrame
    (req : InitializeCaseRequest)
    (start s : ArbitrationState)
    (hInit : initializeCase req = .ok start)
    (hRun : StepReachableFrom start s) :
    caseFrameMatches
      (trimString req.proposition)
      req.state.policy
      (councilMemberIds req.council_members)
      s := by
  have hStart :
      caseFrameMatches
        (trimString req.proposition)
        req.state.policy
        (councilMemberIds req.council_members)
        start := initializeCase_establishes_caseFrame req start hInit
  induction hRun with
  | refl =>
      exact hStart
  | step u v action hu hStep ih =>
      exact step_preserves_caseFrame u v action
        (trimString req.proposition)
        req.state.policy
        (councilMemberIds req.council_members)
        ih
        hStep

end ArbProofs
