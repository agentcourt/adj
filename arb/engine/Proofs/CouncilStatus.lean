import Proofs.NoStuck

namespace ArbProofs

/-
This file finishes the part of Stage 4 that the earlier council-integrity file
left open.

`CouncilIntegrity.lean` already proves that reachable states preserve unique
member identifiers, that current-round vote identifiers are distinct, that
current-round votes belong to seated members, and that stored vote rounds stay
within the active deliberation round.  Those are the integrity facts that the
no-stuck theorem needed.

The remaining public statement is about change over time.

Two properties matter here.

First: seated membership is monotone.  A successful public step may leave the
seated roster unchanged or shrink it, but it never restores a non-seated
member to `seated`.

Second: when the engine records a new council vote, that vote comes from a
member who was seated in the source state.  This is a state-to-state theorem,
not only a theorem about the final stored record.
-/

def seatedCouncilMemberIdsShrink (s t : ArbitrationState) : Prop :=
  seatedCouncilMemberIds t.case ⊆ seatedCouncilMemberIds s.case

def newCouncilVotesComeFromSeated (s t : ArbitrationState) : Prop :=
  ∀ vote ∈ t.case.council_votes, vote ∉ s.case.council_votes →
    vote.round = s.case.deliberation_round ∧ vote.member_id ∈ seatedCouncilMemberIds s.case

theorem seatedCouncilMemberIdsShrink_refl
    (s : ArbitrationState) :
    seatedCouncilMemberIdsShrink s s := by
  intro memberId hMem
  exact hMem

theorem seatedCouncilMemberIdsShrink_trans
    {s t u : ArbitrationState}
    (hST : seatedCouncilMemberIdsShrink s t)
    (hTU : seatedCouncilMemberIdsShrink t u) :
    seatedCouncilMemberIdsShrink s u := by
  intro memberId hMem
  exact hST (hTU hMem)

theorem seatedCouncilMemberIdsShrink_of_same_members
    {s t : ArbitrationState}
    (hMembers : t.case.council_members = s.case.council_members) :
    seatedCouncilMemberIdsShrink s t := by
  intro memberId hMem
  simpa [seatedCouncilMemberIds, seatedCouncilMembers, hMembers] using hMem

theorem newCouncilVotesComeFromSeated_of_same_votes
    {s t : ArbitrationState}
    (hVotes : t.case.council_votes = s.case.council_votes) :
    newCouncilVotesComeFromSeated s t := by
  intro vote hVote hFresh
  have hOld : vote ∈ s.case.council_votes := by
    simpa [hVotes] using hVote
  exact False.elim (hFresh hOld)

/--
`continueDeliberation` never rewrites the stored vote list.

The function may close the case, advance the deliberation round, or leave the
case in the same round.  In every branch it keeps the stored vote list exactly
as it found it.
-/
theorem continueDeliberation_preserves_council_votes
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hCont : continueDeliberation s c = .ok t) :
    t.case.council_votes = c.council_votes := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        rfl
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          rfl
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            rfl
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            rfl
  · simp [hRoundComplete] at hCont
    cases hCont
    rfl

/--
`continueDeliberation` never rewrites the council roster.

The council roster changes only through `removeCouncilMember`.  The later
deliberation transition may close the case or advance the round, but it leaves
the stored council member list in place.
-/
theorem continueDeliberation_preserves_council_members
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hCont : continueDeliberation s c = .ok t) :
    t.case.council_members = c.council_members := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        rfl
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          rfl
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            rfl
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            rfl
  · simp [hRoundComplete] at hCont
    cases hCont
    rfl

theorem mem_concat_last_of_not_mem_prefix
    {xs : List α}
    {x y : α}
    (hMem : y ∈ xs.concat x)
    (hFresh : y ∉ xs) :
    y = x := by
  have hMem' : y ∈ xs ++ [x] := by
    simpa [List.concat_eq_append] using hMem
  rcases List.mem_append.mp hMem' with hOld | hTail
  · exact False.elim (hFresh hOld)
  · simp at hTail
    exact hTail

/--
Replacing one member status with a non-seated value can only shrink the seated
roster.

This is the status-monotonicity fact the public theorem needs.  The update may
remove the targeted member from the seated roster.  It cannot add a new seated
member, because the only changed status is explicitly non-seated.
-/
theorem setMemberStatus_shrinks_seatedCouncilMemberIds
    (c : ArbitrationCase)
    (memberId status : String)
    (hStatus : trimString status ≠ "seated") :
    councilMemberIds
        ((c.council_members.map (fun member =>
            if member.member_id = memberId then
              { member with status := trimString status }
            else
              member)).filter memberIsSeated)
      ⊆ councilMemberIds (c.council_members.filter memberIsSeated) := by
  intro target hTarget
  have hWitness :
      ∃ member,
        member ∈
            (c.council_members.map (fun member =>
              if member.member_id = memberId then
                { member with status := trimString status }
              else
                member)).filter memberIsSeated ∧
          member.member_id = target := by
    simpa [councilMemberIds] using hTarget
  rcases hWitness with ⟨member, hMemberMem, rfl⟩
  have hMemberMap :
      member ∈
        c.council_members.map (fun member =>
          if member.member_id = memberId then
            { member with status := trimString status }
          else
            member) := by
    exact (List.mem_filter.mp hMemberMem).1
  have hMemberSeated : memberIsSeated member = true := by
    exact (List.mem_filter.mp hMemberMem).2
  rcases List.mem_map.mp hMemberMap with ⟨source, hSourceMem, hSourceEq⟩
  by_cases hSourceId : source.member_id = memberId
  · have hUpdatedSeated :
        memberIsSeated { source with status := trimString status } = true := by
      have hUpdatedEq : { source with status := trimString status } = member := by
        simpa [hSourceId] using hSourceEq
      simpa [hUpdatedEq] using hMemberSeated
    simp [memberIsSeated, hStatus] at hUpdatedSeated
  · have hMemberEq : member = source := by
      simpa [hSourceId] using hSourceEq.symm
    have hSourceSeated : memberIsSeated source = true := by
      simpa [hMemberEq] using hMemberSeated
    have hSourceSeatMem : source ∈ c.council_members.filter memberIsSeated := by
      exact List.mem_filter.mpr ⟨hSourceMem, hSourceSeated⟩
    simpa [councilMemberIds, hMemberEq] using
      (show ∃ candidate, candidate ∈ c.council_members.filter memberIsSeated ∧ candidate.member_id = source.member_id from
        ⟨source, hSourceSeatMem, rfl⟩)

theorem failMemberStatus_shrinks_seatedCouncilMemberIds
    (c : ArbitrationCase)
    (memberId reason opportunityId message : String) :
    councilMemberIds
        ((c.council_members.map (fun member =>
            if member.member_id = memberId then
              { member with
                status := "failed"
                failure_reason := reason
                failure_opportunity_id := opportunityId
                failure_message := message }
            else
              member)).filter memberIsSeated)
      ⊆ councilMemberIds (c.council_members.filter memberIsSeated) := by
  intro target hTarget
  have hWitness :
      ∃ member,
        member ∈
            (c.council_members.map (fun member =>
              if member.member_id = memberId then
                { member with
                  status := "failed"
                  failure_reason := reason
                  failure_opportunity_id := opportunityId
                  failure_message := message }
              else
                member)).filter memberIsSeated ∧
          member.member_id = target := by
    simpa [councilMemberIds] using hTarget
  rcases hWitness with ⟨member, hMemberMem, rfl⟩
  have hMemberMap :
      member ∈
        c.council_members.map (fun member =>
          if member.member_id = memberId then
            { member with
              status := "failed"
              failure_reason := reason
              failure_opportunity_id := opportunityId
              failure_message := message }
          else
            member) := by
    exact (List.mem_filter.mp hMemberMem).1
  have hMemberSeated : memberIsSeated member = true := by
    exact (List.mem_filter.mp hMemberMem).2
  rcases List.mem_map.mp hMemberMap with ⟨source, hSourceMem, hSourceEq⟩
  by_cases hSourceId : source.member_id = memberId
  · have hUpdatedSeated :
        memberIsSeated
          { source with
            status := "failed"
            failure_reason := reason
            failure_opportunity_id := opportunityId
            failure_message := message } = true := by
      have hUpdatedEq :
          { source with
            status := "failed"
            failure_reason := reason
            failure_opportunity_id := opportunityId
            failure_message := message } = member := by
        simpa [hSourceId] using hSourceEq
      simpa [hUpdatedEq] using hMemberSeated
    simp [memberIsSeated] at hUpdatedSeated
  · have hMemberEq : member = source := by
      simpa [hSourceId] using hSourceEq.symm
    have hSourceSeated : memberIsSeated source = true := by
      simpa [hMemberEq] using hMemberSeated
    have hSourceSeatMem : source ∈ c.council_members.filter memberIsSeated := by
      exact List.mem_filter.mpr ⟨hSourceMem, hSourceSeated⟩
    simpa [councilMemberIds, hMemberEq] using
      (show ∃ candidate, candidate ∈ c.council_members.filter memberIsSeated ∧ candidate.member_id = source.member_id from
        ⟨source, hSourceSeatMem, rfl⟩)

/--
Every successful public step can only shrink the seated roster.

Most public actions do not touch the council roster at all.  The removal
action is the only one that can change it, and it changes one seated member to
an explicitly non-seated status.
-/
theorem step_shrinks_seatedCouncilMemberIds
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    seatedCouncilMemberIdsShrink s t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with ⟨rawText, rfl⟩
    exact seatedCouncilMemberIdsShrink_of_same_members <|
      addFiling_preserves_council_members s.case "openings"
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
      have hMembers :
          (addFiling s.case "arguments"
              (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
              (trimString rawText)).council_members =
            s.case.council_members := by
        simpa [List.isEmpty_iff] using
          addFiling_preserves_council_members s.case "arguments"
            (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
            (trimString rawText)
      exact seatedCouncilMemberIdsShrink_of_same_members <| by
        simpa [stateWithCase, appendSupplementalMaterials_preserves_council_members, List.isEmpty_iff] using hMembers
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
        exact seatedCouncilMemberIdsShrink_of_same_members <| by
          simp [stateWithCase, appendSupplementalMaterials_preserves_council_members,
            addFiling_preserves_council_members s.case "rebuttals" "plaintiff" (trimString rawText)]
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
          exact seatedCouncilMemberIdsShrink_of_same_members <| by
            simp [stateWithCase, appendSupplementalMaterials_preserves_council_members,
              addFiling_preserves_council_members s.case "surrebuttals" "defendant" (trimString rawText)]
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with ⟨rawText, rfl⟩
            exact seatedCouncilMemberIdsShrink_of_same_members <|
              by
                simpa [List.isEmpty_iff, stateWithCase] using
                  addFiling_preserves_council_members s.case "closings"
                    (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                    (trimString rawText)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · by_cases hRebuttals : s.case.phase = "rebuttals"
              · have hPassResult :
                    (do
                      requireRole action.actor_role "plaintiff"
                      if !s.case.rebuttals.isEmpty then
                        throw "rebuttal already submitted"
                      pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
                  simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hClosing, hPass, hRebuttals] using hStepCore
                cases hRole : requireRole action.actor_role "plaintiff" with
                | error err =>
                    rw [hRole] at hPassResult
                    simp at hPassResult
                    cases hPassResult
                | ok _ =>
                    rw [hRole] at hPassResult
                    cases hEmpty : s.case.rebuttals.isEmpty with
                    | false =>
                        simp [hEmpty] at hPassResult
                        cases hPassResult
                    | true =>
                        simp [hEmpty] at hPassResult
                        cases hPassResult
                        exact seatedCouncilMemberIdsShrink_of_same_members rfl
              · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
                · have hPassResult :
                      (do
                        requireRole action.actor_role "defendant"
                        if !s.case.surrebuttals.isEmpty then
                          throw "surrebuttal already submitted"
                        pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
                    simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hClosing, hPass, hRebuttals, hSurrebuttals] using hStepCore
                  cases hRole : requireRole action.actor_role "defendant" with
                  | error err =>
                      rw [hRole] at hPassResult
                      simp at hPassResult
                      cases hPassResult
                  | ok _ =>
                      rw [hRole] at hPassResult
                      cases hEmpty : s.case.surrebuttals.isEmpty with
                      | false =>
                          simp [hEmpty] at hPassResult
                          cases hPassResult
                      | true =>
                          simp [hEmpty] at hPassResult
                          cases hPassResult
                          exact seatedCouncilMemberIdsShrink_of_same_members rfl
                · simp [stepCore, hPass, hRebuttals, hSurrebuttals] at hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
                  simpa [stepCore, hEvidence] using hStepCore
                rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
                exact seatedCouncilMemberIdsShrink_of_same_members <| by
                  simp [stateWithCase, appendSubmittedEvidence]
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_details s t action hVote hStepCore with
                  ⟨memberId, vote, rationale, _hPhase, _hSeated, _hFresh, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  exact seatedCouncilMemberIdsShrink_of_same_members <| by
                    have hMembers : t.case.council_members = c1.council_members :=
                      continueDeliberation_preserves_council_members s t c1 hCont
                    simpa [c1] using hMembers
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_details s t action hRemoval hStepCore with
                    ⟨memberId, status, _hPhase, _hSeated, _hFresh, hStatus, hCont⟩
                    let c1 := { s.case with
                    council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with status := trimString status }
                      else
                        member)
                    }
                    intro target hTarget
                    have hTarget1 : target ∈ seatedCouncilMemberIds c1 := by
                      have hMembers : t.case.council_members = c1.council_members :=
                        continueDeliberation_preserves_council_members s t c1 hCont
                      simpa [seatedCouncilMemberIds, seatedCouncilMembers, c1, hMembers] using hTarget
                    exact setMemberStatus_shrinks_seatedCouncilMemberIds s.case memberId status hStatus hTarget1
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailOp := step_fail_opportunity_result s t action hFail hStepCore
                      rcases failOpportunity_result s t action.payload hFailOp with hCouncil | hParty
                      · rcases hCouncil with ⟨memberId, reason, opportunityId, message,
                          c1, hC1, _hDeliberation, _hSeated, _hFresh, hCont⟩
                        intro target hTarget
                        have hTarget1 : target ∈ seatedCouncilMemberIds c1 := by
                          have hMembers : t.case.council_members = c1.council_members :=
                            continueDeliberation_preserves_council_members s t c1 hCont
                          simpa [seatedCouncilMemberIds, seatedCouncilMembers, hMembers] using hTarget
                        rw [hC1] at hTarget1
                        exact failMemberStatus_shrinks_seatedCouncilMemberIds
                          s.case memberId reason opportunityId message hTarget1
                      · rcases hParty with ⟨_failure, rfl, _hNotClosed,
                          _hNotDeliberation, _hFailureType, _hFailureRole,
                          _hFailurePhase⟩
                        exact seatedCouncilMemberIdsShrink_of_same_members rfl
                    · cases hType : action.action_type <;>
                      simp [hType] at hOpening hArgument hRebuttal hSurrebuttal hClosing hPass hEvidence hVote hRemoval hFail <;>
                      simp [stepCore, hType] at hStepCore

/--
Any successful public run can only shrink the seated roster.

This is the monotonicity theorem that the verification plan called for.
Starting from any source state, later successful public steps may remove seated
members.  They do not restore a member to `seated`.
-/
theorem stepReachableFrom_shrinks_seatedCouncilMemberIds
    (start s : ArbitrationState)
    (hs : StepReachableFrom start s) :
    seatedCouncilMemberIdsShrink start s := by
  induction hs with
  | refl =>
      exact seatedCouncilMemberIdsShrink_refl start
  | step u v action hu hStep ih =>
      exact seatedCouncilMemberIdsShrink_trans ih
        (step_shrinks_seatedCouncilMemberIds u v action hStep)

/--
Once a member is non-seated, later successful public steps do not restore that
member to `seated`.

This theorem is just the readable corollary of the shrinking-roster theorem.
It says directly what a skeptical reader would ask about timed-out or removed
members.
-/
theorem stepReachableFrom_nonseated_stays_nonseated
    (start s : ArbitrationState)
    (hs : StepReachableFrom start s)
    {memberId : String}
    (hAbsent : memberId ∉ seatedCouncilMemberIds start.case) :
    memberId ∉ seatedCouncilMemberIds s.case := by
  intro hPresent
  exact hAbsent ((stepReachableFrom_shrinks_seatedCouncilMemberIds start s hs) hPresent)

/--
A successful council-vote step introduces only a seated current-round vote.

This is the step-local statement behind the larger public theorem below.  The
engine records exactly one new vote, that vote belongs to the current source
round, and its member was seated in the source state.
-/
theorem step_submit_council_vote_introduces_only_seated_currentRoundVote
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    newCouncilVotesComeFromSeated s t := by
  rcases step_submit_council_vote_details s t action hType hStep with
    ⟨memberId, vote, rationale, _hPhase, hSeated, _hFresh, hCont⟩
  let newVote : CouncilVote := {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  }
  let c1 := { s.case with council_votes := s.case.council_votes.concat newVote }
  have hVotes : t.case.council_votes = c1.council_votes :=
    continueDeliberation_preserves_council_votes s t c1 hCont
  intro storedVote hStoredVote hFresh
  have hStoredVote1 : storedVote ∈ c1.council_votes := by
    simpa [hVotes] using hStoredVote
  have hEq : storedVote = newVote := by
    apply mem_concat_last_of_not_mem_prefix
    · simpa [c1] using hStoredVote1
    · exact hFresh
  subst hEq
  exact ⟨rfl, hSeated⟩

/--
Every successful public step introduces new council votes only from seated
members in the source state.

Most public actions do not add votes at all.  The council-vote action is the
only exception, and the engine admits that step only after checking that the
member is seated in the source state and has not already voted in the current
round.
-/
theorem step_introduces_newCouncilVotes_only_from_seated
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    newCouncilVotesComeFromSeated s t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with ⟨rawText, rfl⟩
    exact newCouncilVotesComeFromSeated_of_same_votes <|
      addFiling_preserves_council_votes s.case "openings"
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
      have hVotes :
          (addFiling s.case "arguments"
              (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
              (trimString rawText)).council_votes =
            s.case.council_votes := by
        simpa [List.isEmpty_iff] using
          addFiling_preserves_council_votes s.case "arguments"
            (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
            (trimString rawText)
      exact newCouncilVotesComeFromSeated_of_same_votes <| by
        simpa [stateWithCase, appendSupplementalMaterials_preserves_council_votes, List.isEmpty_iff] using hVotes
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
        exact newCouncilVotesComeFromSeated_of_same_votes <| by
          simp [stateWithCase, appendSupplementalMaterials_preserves_council_votes,
            addFiling_preserves_council_votes s.case "rebuttals" "plaintiff" (trimString rawText)]
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
          exact newCouncilVotesComeFromSeated_of_same_votes <| by
            simp [stateWithCase, appendSupplementalMaterials_preserves_council_votes,
              addFiling_preserves_council_votes s.case "surrebuttals" "defendant" (trimString rawText)]
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with ⟨rawText, rfl⟩
            exact newCouncilVotesComeFromSeated_of_same_votes <|
              by
                simpa [List.isEmpty_iff, stateWithCase] using
                  addFiling_preserves_council_votes s.case "closings"
                    (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                    (trimString rawText)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · by_cases hRebuttals : s.case.phase = "rebuttals"
              · have hPassResult :
                    (do
                      requireRole action.actor_role "plaintiff"
                      if !s.case.rebuttals.isEmpty then
                        throw "rebuttal already submitted"
                      pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
                  simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hClosing, hPass, hRebuttals] using hStepCore
                cases hRole : requireRole action.actor_role "plaintiff" with
                | error err =>
                    rw [hRole] at hPassResult
                    simp at hPassResult
                    cases hPassResult
                | ok _ =>
                    rw [hRole] at hPassResult
                    cases hEmpty : s.case.rebuttals.isEmpty with
                    | false =>
                        simp [hEmpty] at hPassResult
                        cases hPassResult
                    | true =>
                        simp [hEmpty] at hPassResult
                        cases hPassResult
                        exact newCouncilVotesComeFromSeated_of_same_votes rfl
              · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
                · have hPassResult :
                      (do
                        requireRole action.actor_role "defendant"
                        if !s.case.surrebuttals.isEmpty then
                          throw "surrebuttal already submitted"
                        pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
                    simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hClosing, hPass, hRebuttals, hSurrebuttals] using hStepCore
                  cases hRole : requireRole action.actor_role "defendant" with
                  | error err =>
                      rw [hRole] at hPassResult
                      simp at hPassResult
                      cases hPassResult
                  | ok _ =>
                      rw [hRole] at hPassResult
                      cases hEmpty : s.case.surrebuttals.isEmpty with
                      | false =>
                          simp [hEmpty] at hPassResult
                          cases hPassResult
                      | true =>
                          simp [hEmpty] at hPassResult
                          cases hPassResult
                          exact newCouncilVotesComeFromSeated_of_same_votes rfl
                · simp [stepCore, hPass, hRebuttals, hSurrebuttals] at hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
                  simpa [stepCore, hEvidence] using hStepCore
                rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
                exact newCouncilVotesComeFromSeated_of_same_votes <| by
                  simp [stateWithCase, appendSubmittedEvidence]
              · by_cases hVote : action.action_type = "submit_council_vote"
                · exact step_submit_council_vote_introduces_only_seated_currentRoundVote
                    s t action hVote hStepCore
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_details s t action hRemoval hStepCore with
                    ⟨memberId, status, _hPhase, _hSeated, _hFresh, _hStatus, hCont⟩
                    let c1 := { s.case with
                    council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with status := trimString status }
                      else
                        member)
                    }
                    exact newCouncilVotesComeFromSeated_of_same_votes <|
                      continueDeliberation_preserves_council_votes s t c1 hCont
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailOp := step_fail_opportunity_result s t action hFail hStepCore
                      rcases failOpportunity_result s t action.payload hFailOp with hCouncil | hParty
                      · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                          c1, hC1, _hDeliberation, _hSeated, _hFresh, hCont⟩
                        exact newCouncilVotesComeFromSeated_of_same_votes <| by
                          have hVotes : t.case.council_votes = c1.council_votes :=
                            continueDeliberation_preserves_council_votes s t c1 hCont
                          rw [hC1] at hVotes
                          simpa using hVotes
                      · rcases hParty with ⟨_failure, rfl, _hNotClosed,
                          _hNotDeliberation, _hFailureType, _hFailureRole,
                          _hFailurePhase⟩
                        exact newCouncilVotesComeFromSeated_of_same_votes rfl
                    · cases hType : action.action_type <;>
                      simp [hType] at hOpening hArgument hRebuttal hSurrebuttal hClosing hPass hEvidence hVote hRemoval hFail <;>
                      simp [stepCore, hType] at hStepCore

end ArbProofs
