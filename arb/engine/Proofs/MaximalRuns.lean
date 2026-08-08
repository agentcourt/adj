import Proofs.Realizability
import Proofs.Progress
import Proofs.TerminalStates

namespace ArbProofs

def terminalStatusInvariant (s : ArbitrationState) : Prop :=
  s.case.status = "active" ∨
    (s.case.status = "closed" ∧ s.case.phase = "closed") ∨
      s.case.status = "failed"

def stepBlocked (s : ArbitrationState) : Prop :=
  ∀ action t, step { state := s, action := action } ≠ .ok t

def StepPathMaximal (start : ArbitrationState) (n : Nat) (s : ArbitrationState) : Prop :=
  StepPath start n s ∧ stepBlocked s

theorem step_ok_source_status_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    s.case.status ≠ "closed" := by
  unfold step at hStep
  by_cases hClosed : s.case.status = "closed"
  · simp [hClosed] at hStep
  · exact hClosed

theorem continueDeliberation_preserves_terminalStatusInvariant_of_active
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hActive : c.status = "active")
    (hCont : continueDeliberation s c = .ok t) :
    terminalStatusInvariant t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        exact Or.inr (Or.inl ⟨rfl, rfl⟩)
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          exact Or.inr (Or.inl ⟨rfl, rfl⟩)
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact Or.inr (Or.inl ⟨rfl, rfl⟩)
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact Or.inl (by simpa [stateWithCase] using hActive)
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    exact Or.inl (by simpa [stateWithCase] using hActive)

theorem step_preserves_terminalStatusInvariant
    (s t : ArbitrationState)
    (action : CourtAction)
    (hInvariant : terminalStatusInvariant s)
    (hStep : step { state := s, action := action } = .ok t) :
    terminalStatusInvariant t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  have hSourceNotClosed := step_ok_source_status_ne_closed s t action hStep
  have hSourceNotFailed := step_ok_source_status_ne_failed s t action hStep
  have hSourceActive : s.case.status = "active" := by
    rcases hInvariant with hActive | hTerminal
    · exact hActive
    · rcases hTerminal with hClosed | hFailed
      · exact False.elim (hSourceNotClosed hClosed.1)
      · exact False.elim (hSourceNotFailed hFailed)
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with
      ⟨rawText, rfl⟩
    exact Or.inl (by
      simp [stateWithCase, addFiling_preserves_status, hSourceActive])
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
      exact Or.inl (by
        simp [stateWithCase, appendSupplementalMaterials_preserves_status,
          addFiling_preserves_status, hSourceActive])
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
        exact Or.inl (by
          simp [stateWithCase, appendSupplementalMaterials_preserves_status,
            addFiling_preserves_status, hSourceActive])
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
          exact Or.inl (by
            simp [stateWithCase, appendSupplementalMaterials_preserves_status,
              addFiling_preserves_status, hSourceActive])
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit :
                submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hEvidence]
                using hStepCore
            rcases submitEvidence_result s t action.actor_role action.payload hSubmit with
              ⟨evidence, rfl⟩
            exact Or.inl (by
              simp [stateWithCase, appendSubmittedEvidence_preserves_status, hSourceActive])
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with
                ⟨rawText, rfl⟩
              exact Or.inl (by
                simp [stateWithCase, addFiling_preserves_status, hSourceActive])
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_result s t action hPass hStepCore with hResult | hResult
                · rcases hResult with ⟨_hPhase, rfl⟩
                  exact Or.inl (by simp [stateWithCase, hSourceActive])
                · rcases hResult with ⟨_hPhase, rfl⟩
                  exact Or.inl (by simp [stateWithCase, hSourceActive])
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, _hPhase, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  exact continueDeliberation_preserves_terminalStatusInvariant_of_active
                    s t c1
                    (by simp [c1, hSourceActive])
                    (by simpa [c1] using hCont)
                · by_cases hRemove : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemove hStepCore with
                      ⟨memberId, status, _hPhase, hCont⟩
                    let c1 := { s.case with
                      council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                        if member.member_id = memberId then
                          { member with status := trimString status }
                        else
                          member) }
                    exact continueDeliberation_preserves_terminalStatusInvariant_of_active
                      s t c1
                      (by simp [c1, hSourceActive])
                      (by simpa [c1] using hCont)
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailStep :
                          failOpportunity s action.payload = .ok t := by
                        have hCore :
                            (do
                              requireRole action.actor_role "system"
                              failOpportunity s action.payload) = .ok t := by
                          simpa [stepCore, hFail] using hStepCore
                        cases hRole : requireRole action.actor_role "system" with
                        | error err =>
                            rw [hRole] at hCore
                            cases hCore
                        | ok okv =>
                            cases okv
                            rw [hRole] at hCore
                            simpa [SeqRight.seqRight, Bind.bind, Except.bind] using hCore
                      rcases failOpportunity_result s t action.payload hFailStep with hCouncil | hParty
                      · rcases hCouncil with ⟨memberId, reason, opportunityId, message, c1,
                          hC1, _hPhase, _hSeated, _hFresh, hCont⟩
                        have hC1Active : c1.status = "active" := by
                          rw [hC1]
                          simp [hSourceActive]
                        exact continueDeliberation_preserves_terminalStatusInvariant_of_active
                          s t c1 hC1Active hCont
                      · rcases hParty with ⟨failure, rfl, _hNotClosed, _hNotDeliberation,
                          _hFailureType, _hFailureRole, _hFailurePhase⟩
                        exact Or.inr (Or.inr (by simp [stateWithCase]))
                    · simp [stepCore] at hStepCore

theorem reachable_terminalStatusInvariant
    (s : ArbitrationState)
    (hs : Reachable s) :
    terminalStatusInvariant s := by
  induction hs with
  | init req s hInit =>
      exact Or.inl (initializeCase_status_active req s hInit)
  | step s t action hs hStep ih =>
      exact step_preserves_terminalStatusInvariant s t action ih hStep

theorem reachable_status_closed_implies_phase_closed
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.status = "closed") :
    s.case.phase = "closed" := by
  have hInvariant := reachable_terminalStatusInvariant s hs
  rcases hInvariant with hActive | hTerminal
  · rw [hClosed] at hActive
    simp at hActive
  · rcases hTerminal with hClosedInvariant | hFailed
    · exact hClosedInvariant.2
    · rw [hClosed] at hFailed
      simp at hFailed

theorem maximalStepPath_status_terminal
    (start s : ArbitrationState)
    (n : Nat)
    (hStart : Reachable start)
    (hPath : StepPath start n s)
    (hBlocked : stepBlocked s) :
    s.case.status = "closed" ∨ s.case.status = "failed" := by
  have hs : Reachable s := stepPath_reachable start s n hStart hPath
  have hInvariant := reachable_terminalStatusInvariant s hs
  rcases hInvariant with hActive | hTerminal
  · rcases reachable_active_has_successful_step s hs hActive with ⟨action, t, hStep⟩
    exact False.elim (hBlocked action t hStep)
  · rcases hTerminal with hClosed | hFailed
    · exact Or.inl hClosed.1
    · exact Or.inr hFailed

theorem maximalStepPath_terminal_accounted
    (start s : ArbitrationState)
    (n : Nat)
    (hStart : Reachable start)
    (hPath : StepPath start n s)
    (hBlocked : stepBlocked s) :
    (s.case.status = "closed" ∧
        s.case.phase = "closed" ∧
          (s.case.resolution = "demonstrated" ∨
            s.case.resolution = "not_demonstrated" ∨
              s.case.resolution = "no_majority")) ∨
      (s.case.status = "failed" ∧
        ∃ failure,
          s.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = s.case.phase) := by
  have hs : Reachable s := stepPath_reachable start s n hStart hPath
  rcases maximalStepPath_status_terminal start s n hStart hPath hBlocked with hClosed | hFailed
  · have hPhase : s.case.phase = "closed" :=
      reachable_status_closed_implies_phase_closed s hs hClosed
    have hResolution := reachable_closed_resolution_enum s hs hPhase
    exact Or.inl ⟨hClosed, hPhase, hResolution⟩
  · have hFailure := reachable_failed_has_failure s hs hFailed
    exact Or.inr ⟨hFailed, hFailure⟩

theorem StepPathMaximal_terminal_accounted
    (start s : ArbitrationState)
    (n : Nat)
    (hStart : Reachable start)
    (hMaximal : StepPathMaximal start n s) :
    (s.case.status = "closed" ∧
        s.case.phase = "closed" ∧
          (s.case.resolution = "demonstrated" ∨
            s.case.resolution = "not_demonstrated" ∨
              s.case.resolution = "no_majority")) ∨
      (s.case.status = "failed" ∧
        ∃ failure,
          s.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = s.case.phase) := by
  exact maximalStepPath_terminal_accounted start s n hStart hMaximal.1 hMaximal.2

theorem initializedStepPathMaximal_terminal_accounted
    (req : InitializeCaseRequest)
    (start s : ArbitrationState)
    (n : Nat)
    (hInit : initializeCase req = .ok start)
    (hMaximal : StepPathMaximal start n s) :
    (s.case.status = "closed" ∧
        s.case.phase = "closed" ∧
          (s.case.resolution = "demonstrated" ∨
            s.case.resolution = "not_demonstrated" ∨
              s.case.resolution = "no_majority")) ∨
      (s.case.status = "failed" ∧
        ∃ failure,
          s.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = s.case.phase) := by
  exact StepPathMaximal_terminal_accounted start s n
    (Reachable.init req start hInit)
    hMaximal

end ArbProofs
