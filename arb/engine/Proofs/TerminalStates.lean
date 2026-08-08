import Proofs.BoundedTermination

namespace ArbProofs

theorem currentResolution_some_enum
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (resolution : String)
    (hResolution : currentResolution? c requiredVotes = some resolution) :
    resolution = "demonstrated" ∨ resolution = "not_demonstrated" := by
  unfold currentResolution? at hResolution
  by_cases hDemonstrated :
      voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes
  · simp [hDemonstrated] at hResolution
    exact Or.inl hResolution.symm
  · by_cases hNotDemonstrated :
        voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ requiredVotes
    · simp [hDemonstrated, hNotDemonstrated] at hResolution
      exact Or.inr hResolution.symm
    · simp [hDemonstrated, hNotDemonstrated] at hResolution

theorem continueDeliberation_closed_resolution_enum
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed") :
    t.case.resolution = "demonstrated" ∨
      t.case.resolution = "not_demonstrated" ∨
        t.case.resolution = "no_majority" := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        rcases currentResolution_some_enum
            c s.policy.required_votes_for_decision resolution hResolution with
          hDemonstrated | hNotDemonstrated
        · exact Or.inl hDemonstrated
        · exact Or.inr (Or.inl hNotDemonstrated)
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          exact Or.inr (Or.inr rfl)
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact Or.inr (Or.inr rfl)
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp [hDeliberation] at hClosed
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simp [hDeliberation] at hClosed

theorem continueDeliberation_status_ne_failed
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hStatus : c.status ≠ "failed")
    (hCont : continueDeliberation s c = .ok t) :
    t.case.status ≠ "failed" := by
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
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simpa [stateWithCase] using hStatus
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simpa [stateWithCase] using hStatus

theorem step_failed_has_failure
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hFailed : t.case.status = "failed") :
    ∃ failure,
      t.case.failure = some failure ∧
        failure.failure_type = "opportunity_failed" ∧
          (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
            failure.phase = t.case.phase := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  have hSourceNotFailed := step_ok_source_status_ne_failed s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with
      ⟨rawText, rfl⟩
    have hSourceFailed : s.case.status = "failed" := by
      simpa [stateWithCase, addFiling_preserves_status] using hFailed
    exact False.elim (hSourceNotFailed hSourceFailed)
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
      have hSourceFailed : s.case.status = "failed" := by
        simpa [stateWithCase, appendSupplementalMaterials_preserves_status,
          addFiling_preserves_status] using hFailed
      exact False.elim (hSourceNotFailed hSourceFailed)
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
        have hSourceFailed : s.case.status = "failed" := by
          simpa [stateWithCase, appendSupplementalMaterials_preserves_status,
            addFiling_preserves_status] using hFailed
        exact False.elim (hSourceNotFailed hSourceFailed)
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
          have hSourceFailed : s.case.status = "failed" := by
            simpa [stateWithCase, appendSupplementalMaterials_preserves_status,
              addFiling_preserves_status] using hFailed
          exact False.elim (hSourceNotFailed hSourceFailed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with
              ⟨rawText, rfl⟩
            have hSourceFailed : s.case.status = "failed" := by
              simpa [stateWithCase, addFiling_preserves_status] using hFailed
            exact False.elim (hSourceNotFailed hSourceFailed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · by_cases hRebuttals : s.case.phase = "rebuttals"
              · have hPassStep :
                    (do
                      requireRole action.actor_role "plaintiff"
                      if !s.case.rebuttals.isEmpty then
                        throw "rebuttal already submitted"
                      pure <| stateWithCase s { s.case with phase := "surrebuttals" }) =
                      .ok t := by
                  simpa [stepCore, hPass, hRebuttals] using hStepCore
                cases hRole : requireRole action.actor_role "plaintiff" with
                | error err =>
                    rw [hRole] at hPassStep
                    simp at hPassStep
                    cases hPassStep
                | ok okv =>
                    cases okv
                    rw [hRole] at hPassStep
                    cases hEmpty : s.case.rebuttals.isEmpty with
                    | false =>
                        simp [hEmpty] at hPassStep
                        cases hPassStep
                    | true =>
                        simp [hEmpty] at hPassStep
                        cases hPassStep
                        have hSourceFailed : s.case.status = "failed" := by
                          simpa [stateWithCase] using hFailed
                        exact False.elim (hSourceNotFailed hSourceFailed)
              · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
                · have hPassStep :
                      (do
                        requireRole action.actor_role "defendant"
                        if !s.case.surrebuttals.isEmpty then
                          throw "surrebuttal already submitted"
                        pure <| stateWithCase s { s.case with phase := "closings" }) =
                        .ok t := by
                    simpa [stepCore, hPass, hRebuttals, hSurrebuttals] using hStepCore
                  cases hRole : requireRole action.actor_role "defendant" with
                  | error err =>
                      rw [hRole] at hPassStep
                      simp at hPassStep
                      cases hPassStep
                  | ok okv =>
                      cases okv
                      rw [hRole] at hPassStep
                      cases hEmpty : s.case.surrebuttals.isEmpty with
                      | false =>
                          simp [hEmpty] at hPassStep
                          cases hPassStep
                      | true =>
                          simp [hEmpty] at hPassStep
                          cases hPassStep
                          have hSourceFailed : s.case.status = "failed" := by
                            simpa [stateWithCase] using hFailed
                          exact False.elim (hSourceNotFailed hSourceFailed)
                · simp [stepCore, hPass, hRebuttals, hSurrebuttals] at hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · have hSubmit :
                    submitEvidence s action.actor_role action.payload = .ok t := by
                  simpa [stepCore, hEvidence] using hStepCore
                rcases submitEvidence_result s t action.actor_role action.payload hSubmit with
                  ⟨evidence, rfl⟩
                have hSourceFailed : s.case.status = "failed" := by
                  simpa [stateWithCase, appendSubmittedEvidence_preserves_status] using hFailed
                exact False.elim (hSourceNotFailed hSourceFailed)
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  have hC1Status : c1.status ≠ "failed" := by
                    simpa [c1] using hSourceNotFailed
                  have hNotFailed :
                      t.case.status ≠ "failed" :=
                    continueDeliberation_status_ne_failed s t c1 hC1Status
                      (by simpa [c1] using hCont)
                  exact False.elim (hNotFailed hFailed)
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemoval hStepCore with
                      ⟨memberId, status, hDeliberation, hCont⟩
                    let c1 := {
                      s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                        if member.member_id = memberId then
                          { member with status := trimString status }
                        else
                          member)
                    }
                    have hC1Status : c1.status ≠ "failed" := by
                      simpa [c1] using hSourceNotFailed
                    have hNotFailed :
                        t.case.status ≠ "failed" :=
                      continueDeliberation_status_ne_failed s t c1 hC1Status
                        (by simpa [c1] using hCont)
                    exact False.elim (hNotFailed hFailed)
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailOp := step_fail_opportunity_result s t action hFail hStepCore
                      rcases failOpportunity_result s t action.payload hFailOp with hCouncil | hParty
                      · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                          c1, hC1, _hDeliberation, _hSeated, _hFresh, hCont⟩
                        have hC1Status : c1.status ≠ "failed" := by
                          rw [hC1]
                          simpa using hSourceNotFailed
                        have hNotFailed :
                            t.case.status ≠ "failed" :=
                          continueDeliberation_status_ne_failed s t c1 hC1Status hCont
                        exact False.elim (hNotFailed hFailed)
                      · rcases hParty with ⟨failure, rfl, _hNotClosed,
                          _hNotDeliberation, hFailureType, hFailureRole,
                          hFailurePhase⟩
                        refine ⟨failure, ?_, hFailureType, hFailureRole, ?_⟩
                        · simp [stateWithCase]
                        · simpa [stateWithCase] using hFailurePhase
                    · simp [stepCore] at hStepCore

theorem step_closed_resolution_enum
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed") :
    t.case.resolution = "demonstrated" ∨
      t.case.resolution = "not_demonstrated" ∨
        t.case.resolution = "no_majority" := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim
      ((step_record_opening_statement_phase_ne_closed s t action hOpening hStepCore) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim
        ((step_submit_argument_phase_ne_closed s t action hArgument hStepCore) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim
          ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStepCore) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim
            ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStepCore) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim
              ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStepCore) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim
                ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStepCore) hClosed)
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact False.elim
                  ((step_submit_evidence_phase_ne_closed s t action hEvidence hStepCore) hClosed)
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  exact continueDeliberation_closed_resolution_enum s t c1
                    (by simpa [c1] using hDeliberation)
                    (by simpa [c1] using hCont)
                    hClosed
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemoval hStepCore with
                      ⟨memberId, status, hDeliberation, hCont⟩
                    let c1 := {
                      s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                        if member.member_id = memberId then
                          { member with status := trimString status }
                        else
                          member)
                    }
                    exact continueDeliberation_closed_resolution_enum s t c1
                      (by simpa [c1] using hDeliberation)
                      (by simpa [c1] using hCont)
                      hClosed
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailOp := step_fail_opportunity_result s t action hFail hStepCore
                      rcases failOpportunity_result s t action.payload hFailOp with hCouncil | hParty
                      · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                          c1, hC1, hDeliberation, _hSeated, _hFresh, hCont⟩
                        exact continueDeliberation_closed_resolution_enum s t c1
                          (by rw [hC1]; simpa using hDeliberation)
                          hCont
                          hClosed
                      · rcases hParty with ⟨_failure, _hEq, hNotClosed,
                          _hNotDeliberation, _hFailureType, _hFailureRole,
                          _hFailurePhase⟩
                        exact False.elim (hNotClosed hClosed)
                    · simp [stepCore] at hStepCore

theorem reachable_closed_resolution_enum
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed") :
    s.case.resolution = "demonstrated" ∨
      s.case.resolution = "not_demonstrated" ∨
        s.case.resolution = "no_majority" := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step u t action hu hStep _ =>
      exact step_closed_resolution_enum u t action hStep hClosed

theorem reachable_failed_has_failure
    (s : ArbitrationState)
    (hs : Reachable s)
    (hFailed : s.case.status = "failed") :
    ∃ failure,
      s.case.failure = some failure ∧
        failure.failure_type = "opportunity_failed" ∧
          (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
            failure.phase = s.case.phase := by
  induction hs with
  | init req s hInit =>
      have hActive := initializeCase_status_active req s hInit
      simp [hActive] at hFailed
  | step u t action hu hStep _ =>
      exact step_failed_has_failure u t action hStep hFailed

end ArbProofs
