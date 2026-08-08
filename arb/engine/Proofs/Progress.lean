import Proofs.CaseFrame
import Proofs.RecordProvenance
import Proofs.CouncilStatus

namespace ArbProofs

def phaseRank : String → Nat
  | "openings" => 0
  | "arguments" => 1
  | "rebuttals" => 2
  | "surrebuttals" => 3
  | "closings" => 4
  | "deliberation" => 5
  | "closed" => 6
  | _ => 7

def fixedFrameProgress (s t : ArbitrationState) : Prop :=
  caseFrameMatches s.case.proposition s.policy (councilMemberIds s.case.council_members) t ∧
    materialsExtend s t ∧
    seatedCouncilMemberIdsShrink s t ∧
    phaseRank s.case.phase ≤ phaseRank t.case.phase ∧
    s.case.deliberation_round ≤ t.case.deliberation_round

theorem caseFrameMatches_refl
    (s : ArbitrationState) :
    caseFrameMatches
      s.case.proposition
      s.policy
      (councilMemberIds s.case.council_members)
      s := by
  simp [caseFrameMatches]

theorem caseFrameMatches_trans
    {proposition : String}
    {policy : ArbitrationPolicy}
    {memberIds : List String}
    {t u : ArbitrationState}
    (hT : caseFrameMatches proposition policy memberIds t)
    (hU : caseFrameMatches
      t.case.proposition
      t.policy
      (councilMemberIds t.case.council_members)
      u) :
    caseFrameMatches proposition policy memberIds u := by
  rcases hT with ⟨hTProp, hTPolicy, hTMembers⟩
  rcases hU with ⟨hUProp, hUPolicy, hUMembers⟩
  constructor
  · calc
      u.case.proposition = t.case.proposition := hUProp
      _ = proposition := hTProp
  constructor
  · calc
      u.policy = t.policy := hUPolicy
      _ = policy := hTPolicy
  · calc
      councilMemberIds u.case.council_members =
          councilMemberIds t.case.council_members := hUMembers
      _ = memberIds := hTMembers

theorem advanceAfterMerits_phaseRank_mono
    (c : ArbitrationCase) :
    phaseRank c.phase ≤ phaseRank (advanceAfterMerits c).phase := by
  by_cases hOpenings : c.phase = "openings"
  · unfold advanceAfterMerits
    by_cases hOpen : c.openings.length >= 2
    · simp [hOpenings, hOpen, phaseRank]
    · simp [hOpenings, hOpen, phaseRank]
  · by_cases hArguments : c.phase = "arguments"
    · unfold advanceAfterMerits
      by_cases hArg : c.arguments.length >= 2
      · simp [hArguments, hArg, phaseRank]
      · simp [hArguments, hArg, phaseRank]
    · by_cases hRebuttals : c.phase = "rebuttals"
      · unfold advanceAfterMerits
        by_cases hRebuttal : c.rebuttals.length >= 1
        · simp [hRebuttals, hRebuttal, phaseRank]
        · simp [hRebuttals, hRebuttal, phaseRank]
      · by_cases hSurrebuttals : c.phase = "surrebuttals"
        · unfold advanceAfterMerits
          by_cases hSurrebuttal : c.surrebuttals.length >= 1
          · simp [hSurrebuttals, hSurrebuttal, phaseRank]
          · simp [hSurrebuttals, hSurrebuttal, phaseRank]
        · by_cases hClosings : c.phase = "closings"
          · unfold advanceAfterMerits
            by_cases hClosing : c.closings.length >= 2
            · simp [hClosings, hClosing, phaseRank]
            · simp [hClosings, hClosing, phaseRank]
          · simp [advanceAfterMerits, hOpenings, hArguments, hRebuttals, hSurrebuttals,
              hClosings, phaseRank]

theorem addFiling_phaseRank_mono
    (c : ArbitrationCase)
    (phase role text : String) :
    phaseRank c.phase ≤ phaseRank (addFiling c phase role text).phase := by
  by_cases hOpenings : phase = "openings"
  · subst hOpenings
    let filing : Filing := { phase := "openings", role := role, text := text }
    let c1 : ArbitrationCase := { c with openings := c.openings.concat filing }
    simpa [addFiling, filing, c1] using advanceAfterMerits_phaseRank_mono c1
  · by_cases hArguments : phase = "arguments"
    · subst hArguments
      let filing : Filing := { phase := "arguments", role := role, text := text }
      let c1 : ArbitrationCase := { c with arguments := c.arguments.concat filing }
      simpa [addFiling, filing, c1] using advanceAfterMerits_phaseRank_mono c1
    · by_cases hRebuttals : phase = "rebuttals"
      · subst hRebuttals
        let filing : Filing := { phase := "rebuttals", role := role, text := text }
        let c1 : ArbitrationCase := { c with rebuttals := c.rebuttals.concat filing }
        simpa [addFiling, filing, c1] using advanceAfterMerits_phaseRank_mono c1
      · by_cases hSurrebuttals : phase = "surrebuttals"
        · subst hSurrebuttals
          let filing : Filing := { phase := "surrebuttals", role := role, text := text }
          let c1 : ArbitrationCase := { c with surrebuttals := c.surrebuttals.concat filing }
          simpa [addFiling, filing, c1] using advanceAfterMerits_phaseRank_mono c1
        · by_cases hClosings : phase = "closings"
          · subst hClosings
            let filing : Filing := { phase := "closings", role := role, text := text }
            let c1 : ArbitrationCase := { c with closings := c.closings.concat filing }
            simpa [addFiling, filing, c1] using advanceAfterMerits_phaseRank_mono c1
          · simpa [addFiling, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]
              using advanceAfterMerits_phaseRank_mono c

theorem appendSupplementalMaterials_preserves_phaseRank
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport) :
    phaseRank (appendSupplementalMaterials c offered reports).phase = phaseRank c.phase := by
  simp [appendSupplementalMaterials, phaseRank]

theorem continueDeliberation_phaseRank_mono
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hPhase : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t) :
    phaseRank c.phase ≤ phaseRank t.case.phase := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        simp [hPhase, phaseRank]
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          simp [hPhase, phaseRank]
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp [hPhase, phaseRank]
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp [hPhase, phaseRank]
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simp [hPhase, phaseRank]

theorem continueDeliberation_deliberation_round_mono
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hCont : continueDeliberation s c = .ok t) :
    c.deliberation_round ≤ t.case.deliberation_round := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        simp
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          simp
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact Nat.le_succ c.deliberation_round
  · simp [hRoundComplete] at hCont
    cases hCont
    exact Nat.le_refl c.deliberation_round

theorem step_pass_phase_opportunity_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    (s.case.phase = "rebuttals" ∧
      t = stateWithCase s { s.case with phase := "surrebuttals" }) ∨
    (s.case.phase = "surrebuttals" ∧
      t = stateWithCase s { s.case with phase := "closings" }) := by
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · left
    constructor
    · exact hRebuttals
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
              rfl
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · right
      constructor
      · exact hSurrebuttals
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
                rfl
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

/--
`fixedFrameProgress` packages the monotone state-to-state coordinates
that the library already proves separately.

The destination state must stay inside the source case frame.  Within that
frame, admitted materials may only grow by suffix, the seated council roster
may only shrink, the phase rank may not move backward, and the deliberation
round may not decrease.  This is the first proof-side preorder that describes
successful public execution abstractly instead of theorem by theorem.
-/
theorem fixedFrameProgress_refl
    (s : ArbitrationState) :
    fixedFrameProgress s s := by
  exact ⟨caseFrameMatches_refl s, materialsExtend_refl s, seatedCouncilMemberIdsShrink_refl s,
    Nat.le_refl _, Nat.le_refl _⟩

theorem fixedFrameProgress_trans
    {s t u : ArbitrationState}
    (hST : fixedFrameProgress s t)
    (hTU : fixedFrameProgress t u) :
    fixedFrameProgress s u := by
  rcases hST with ⟨hFrameST, hMaterialsST, hSeatsST, hPhaseST, hRoundST⟩
  rcases hTU with ⟨hFrameTU, hMaterialsTU, hSeatsTU, hPhaseTU, hRoundTU⟩
  exact ⟨caseFrameMatches_trans hFrameST hFrameTU,
    materialsExtend_trans s t u hMaterialsST hMaterialsTU,
    seatedCouncilMemberIdsShrink_trans hSeatsST hSeatsTU,
    Nat.le_trans hPhaseST hPhaseTU,
    Nat.le_trans hRoundST hRoundTU⟩

theorem step_phaseRank_mono
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    phaseRank s.case.phase ≤ phaseRank t.case.phase := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with ⟨rawText, rfl⟩
    simpa [stateWithCase] using
      addFiling_phaseRank_mono s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText)
  · by_cases hArgument : action.action_type = "submit_argument"
    · have hSubmit :
          recordMeritsSubmission
            s
            "arguments"
            action.actor_role
            (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
            "argument"
            s.policy.max_argument_chars
            true
            action.payload = .ok t := by
        simpa [stepCore, hOpening, hArgument] using hStepCore
      rcases recordMeritsSubmission_with_materials_result
          s t "arguments" action.actor_role
          (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          "argument" s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, rfl⟩
      have hPhase :
          phaseRank s.case.phase ≤
            phaseRank (addFiling s.case "arguments"
              (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
              (trimString rawText)).phase :=
        addFiling_phaseRank_mono s.case "arguments"
          (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          (trimString rawText)
      simpa [stateWithCase, appendSupplementalMaterials_preserves_phaseRank] using hPhase
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
          simpa [stepCore, hOpening, hArgument, hRebuttal] using hStepCore
        rcases recordMeritsSubmission_with_materials_result
            s t "rebuttals" action.actor_role "plaintiff"
            "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
          ⟨rawText, offered, reports, rfl⟩
        have hPhase :
            phaseRank s.case.phase ≤
              phaseRank (addFiling s.case "rebuttals" "plaintiff" (trimString rawText)).phase :=
          addFiling_phaseRank_mono s.case "rebuttals" "plaintiff" (trimString rawText)
        simpa [stateWithCase, appendSupplementalMaterials_preserves_phaseRank] using hPhase
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
            simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal] using hStepCore
          rcases recordMeritsSubmission_with_materials_result
              s t "surrebuttals" action.actor_role "defendant"
              "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
            ⟨rawText, offered, reports, rfl⟩
          have hPhase :
              phaseRank s.case.phase ≤
                phaseRank (addFiling s.case "surrebuttals" "defendant" (trimString rawText)).phase :=
            addFiling_phaseRank_mono s.case "surrebuttals" "defendant" (trimString rawText)
          simpa [stateWithCase, appendSupplementalMaterials_preserves_phaseRank] using hPhase
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with ⟨rawText, rfl⟩
            simpa [stateWithCase] using
              addFiling_phaseRank_mono s.case "closings"
                (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                (trimString rawText)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · rcases step_pass_phase_opportunity_result s t action hPass hStepCore with
                (⟨hPhase, rfl⟩ | ⟨hPhase, rfl⟩)
              · simp [stateWithCase, hPhase, phaseRank]
              · simp [stateWithCase, hPhase, phaseRank]
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
                  simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hClosing,
                    hPass, hEvidence] using hStepCore
                rcases submitEvidence_result s t action.actor_role action.payload hSubmit with
                  ⟨evidence, rfl⟩
                simp [stateWithCase, appendSubmittedEvidence]
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, hPhase, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  have hPhase1 : c1.phase = "deliberation" := by
                    simpa [c1] using hPhase
                  exact continueDeliberation_phaseRank_mono s t c1 hPhase1 hCont
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemoval hStepCore with
                      ⟨memberId, status, hPhase, hCont⟩
                    let c1 := {
                      s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                        if member.member_id = memberId then
                          { member with status := trimString status }
                        else
                          member)
                    }
                    have hPhase1 : c1.phase = "deliberation" := by
                      simpa [c1] using hPhase
                    exact continueDeliberation_phaseRank_mono s t c1 hPhase1 hCont
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailOp := step_fail_opportunity_result s t action hFail hStepCore
                      rcases failOpportunity_result s t action.payload hFailOp with hCouncil | hParty
                      · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                          c1, hC1, hPhase, _hSeated, _hFresh, hCont⟩
                        have hPhase1 : c1.phase = "deliberation" := by
                          rw [hC1]
                          simpa using hPhase
                        have hRank := continueDeliberation_phaseRank_mono s t c1 hPhase1 hCont
                        simpa [hC1] using hRank
                      · rcases hParty with ⟨_failure, rfl, _hNotClosed,
                          _hNotDeliberation, _hFailureType, _hFailureRole,
                          _hFailurePhase⟩
                        simp [stateWithCase]
                    · simp [stepCore] at hStepCore

theorem step_deliberation_round_mono
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    s.case.deliberation_round ≤ t.case.deliberation_round := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with ⟨rawText, rfl⟩
    simpa [stateWithCase] using Nat.le_of_eq
      (addFiling_preserves_deliberation_round s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText)).symm
  · by_cases hArgument : action.action_type = "submit_argument"
    · have hSubmit :
          recordMeritsSubmission
            s
            "arguments"
            action.actor_role
            (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
            "argument"
            s.policy.max_argument_chars
            true
            action.payload = .ok t := by
        simpa [stepCore, hOpening, hArgument] using hStepCore
      rcases recordMeritsSubmission_with_materials_result
          s t "arguments" action.actor_role
          (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          "argument" s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, rfl⟩
      have hRound :
          s.case.deliberation_round =
            (addFiling s.case "arguments"
              (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
              (trimString rawText)).deliberation_round :=
        (addFiling_preserves_deliberation_round s.case "arguments"
          (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          (trimString rawText)).symm
      simpa [stateWithCase, appendSupplementalMaterials_preserves_deliberation_round] using
        Nat.le_of_eq hRound
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
          simpa [stepCore, hOpening, hArgument, hRebuttal] using hStepCore
        rcases recordMeritsSubmission_with_materials_result
            s t "rebuttals" action.actor_role "plaintiff"
            "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
          ⟨rawText, offered, reports, rfl⟩
        have hRound :
            s.case.deliberation_round =
              (addFiling s.case "rebuttals" "plaintiff" (trimString rawText)).deliberation_round :=
          (addFiling_preserves_deliberation_round s.case "rebuttals" "plaintiff" (trimString rawText)).symm
        simpa [stateWithCase, appendSupplementalMaterials_preserves_deliberation_round] using
          Nat.le_of_eq hRound
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
            simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal] using hStepCore
          rcases recordMeritsSubmission_with_materials_result
              s t "surrebuttals" action.actor_role "defendant"
              "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
            ⟨rawText, offered, reports, rfl⟩
          have hRound :
              (addFiling s.case "surrebuttals" "defendant" (trimString rawText)).deliberation_round =
                s.case.deliberation_round :=
            addFiling_preserves_deliberation_round s.case "surrebuttals" "defendant" (trimString rawText)
          simpa [stateWithCase, appendSupplementalMaterials_preserves_deliberation_round] using
            Nat.le_of_eq hRound.symm
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with ⟨rawText, rfl⟩
            have hRound :
                (addFiling s.case "closings"
                  (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                  (trimString rawText)).deliberation_round = s.case.deliberation_round :=
              addFiling_preserves_deliberation_round s.case "closings"
                (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                (trimString rawText)
            simpa [stateWithCase] using Nat.le_of_eq hRound.symm
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · rcases step_pass_phase_opportunity_result s t action hPass hStepCore with
                (⟨_hPhase, rfl⟩ | ⟨_hPhase, rfl⟩)
              · simp [stateWithCase]
              · simp [stateWithCase]
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
                  simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hClosing,
                    hPass, hEvidence] using hStepCore
                rcases submitEvidence_result s t action.actor_role action.payload hSubmit with
                  ⟨evidence, rfl⟩
                simp [stateWithCase, appendSubmittedEvidence]
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, _hPhase, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  have hRoundEq : c1.deliberation_round = s.case.deliberation_round := by
                    simp [c1]
                  have hMono : c1.deliberation_round ≤ t.case.deliberation_round :=
                    continueDeliberation_deliberation_round_mono s t c1 hCont
                  simpa [hRoundEq] using hMono
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
                    have hRoundEq : c1.deliberation_round = s.case.deliberation_round := by
                      simp [c1]
                    have hMono : c1.deliberation_round ≤ t.case.deliberation_round :=
                      continueDeliberation_deliberation_round_mono s t c1 hCont
                    simpa [hRoundEq] using hMono
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailOp := step_fail_opportunity_result s t action hFail hStepCore
                      rcases failOpportunity_result s t action.payload hFailOp with hCouncil | hParty
                      · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                          c1, hC1, _hPhase, _hSeated, _hFresh, hCont⟩
                        have hRoundEq : c1.deliberation_round = s.case.deliberation_round := by
                          rw [hC1]
                        have hMono : c1.deliberation_round ≤ t.case.deliberation_round :=
                          continueDeliberation_deliberation_round_mono s t c1 hCont
                        simpa [hRoundEq] using hMono
                      · rcases hParty with ⟨_failure, rfl, _hNotClosed,
                          _hNotDeliberation, _hFailureType, _hFailureRole,
                          _hFailurePhase⟩
                        simp [stateWithCase]
                    · simp [stepCore] at hStepCore

theorem step_establishes_fixedFrameProgress
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    fixedFrameProgress s t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  have hFrame :
      caseFrameMatches
        s.case.proposition
        s.policy
        (councilMemberIds s.case.council_members)
        s :=
    caseFrameMatches_refl s
  exact ⟨step_preserves_caseFrame s t action
      s.case.proposition
      s.policy
      (councilMemberIds s.case.council_members)
      hFrame
      hStep,
    step_extends_materials s t action hStepCore,
    step_shrinks_seatedCouncilMemberIds s t action hStep,
    step_phaseRank_mono s t action hStep,
    step_deliberation_round_mono s t action hStep⟩

theorem stepReachableFrom_fixedFrameProgress
    (start s : ArbitrationState)
    (hRun : StepReachableFrom start s) :
    fixedFrameProgress start s := by
  induction hRun with
  | refl =>
      exact fixedFrameProgress_refl start
  | step u t action hu hStep ih =>
      exact fixedFrameProgress_trans ih
        (step_establishes_fixedFrameProgress u t action hStep)

theorem initialized_run_progresses_in_initial_frame
    (req : InitializeCaseRequest)
    (start s : ArbitrationState)
    (hInit : initializeCase req = .ok start)
    (hRun : StepReachableFrom start s) :
    caseFrameMatches
      (trimString req.proposition)
      req.state.policy
      (councilMemberIds req.council_members)
      s ∧
    fixedFrameProgress start s := by
  exact ⟨initialized_run_preserves_caseFrame req start s hInit hRun,
    stepReachableFrom_fixedFrameProgress start s hRun⟩

end ArbProofs
