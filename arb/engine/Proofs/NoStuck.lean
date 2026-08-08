import Proofs.CouncilIntegrity
import Proofs.ReachableInvariants
import Proofs.OutcomeSoundness

namespace ArbProofs

def meritsPhase (phase : String) : Prop :=
  phase = "openings" ∨
    phase = "arguments" ∨
    phase = "rebuttals" ∨
    phase = "surrebuttals" ∨
    phase = "closings"

def pristineCouncilState (c : ArbitrationCase) : Prop :=
  c.council_votes = [] ∧
    c.deliberation_round = 1 ∧
    ∀ member ∈ c.council_members, memberIsSeated member = true

/-
This file begins Stage 3 of the verification plan: a reachable active case
should not become stuck without a next opportunity.

The proof has two different parts.

For the merits phases, the result is mostly structural.  `phaseShape` already
states the exact filing prefixes that are permitted in openings, arguments,
rebuttals, surrebuttals, and closings.  The operational question is then
simple: do those shapes line up with what `nextOpportunityForPhase` actually
returns?  The answer should be yes.

Deliberation is different.  There the engine computes the next actor from the
stored vote list and the seated council roster.  The earlier files therefore
proved two separate invariants: council identifiers remain unique, and the
current-round votes stay well formed.  Those are the facts this file needs to
turn "the round is still live" into "some seated council member has not yet
voted."

The first theorem batch here proves the direct operational helpers that the
later no-stuck proof will reuse.
-/

/--
A validated policy requires a positive decision threshold.

The deliberation liveness proof needs a simple arithmetic fact: when the
engine says that too few seated members remain for a decision, the threshold it
compares against is not zero.  That fact comes from `validatePolicy`.
-/
theorem validatePolicy_ok_implies_required_votes_positive
    (p : ArbitrationPolicy)
    (hPolicy : validatePolicy p = .ok PUnit.unit) :
    0 < p.required_votes_for_decision := by
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
      · exact Nat.pos_of_ne_zero hVotes

/--
Successful initialization inherits the positive decision threshold from the
validated policy.
-/
theorem initializeCase_establishes_required_votes_positive
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    0 < s.policy.required_votes_for_decision := by
  have hFrame := initializeCase_establishes_caseFrame req s hInit
  rcases hFrame with ⟨_hProp, hPolicyEq, _hMembers⟩
  have hValid : validatePolicy req.state.policy = .ok PUnit.unit := by
    unfold initializeCase at hInit
    cases hPolicy : validatePolicy req.state.policy with
    | error err =>
      simp [hPolicy] at hInit
      cases hInit
    | ok okv =>
      cases okv
      rfl
  have hPos : 0 < req.state.policy.required_votes_for_decision :=
    validatePolicy_ok_implies_required_votes_positive req.state.policy hValid
  simpa [hPolicyEq] using hPos

/--
Successful public steps preserve positivity of the decision threshold because
they preserve the case policy.
-/
theorem step_preserves_required_votes_positive
    (s t : ArbitrationState)
    (action : CourtAction)
    (hPositive : 0 < s.policy.required_votes_for_decision)
    (hStep : step { state := s, action := action } = .ok t) :
    0 < t.policy.required_votes_for_decision := by
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
  simpa [hPolicyEq] using hPositive

/--
Every reachable state carries a positive decision threshold.
-/
theorem reachable_required_votes_positive
    (s : ArbitrationState)
    (hs : Reachable s) :
    0 < s.policy.required_votes_for_decision := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_required_votes_positive req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_required_votes_positive s t action ih hStep

/--
Advancing the deliberation round clears the new current-round vote list.

This is the operational fact behind the round-advance branch of
`continueDeliberation`: the vote list is not rewritten, but every stored vote
belongs to an earlier round.
-/
theorem currentRoundVotes_empty_after_round_increment
    (c : ArbitrationCase)
    (hBounded : councilVoteRoundsBounded c) :
    currentRoundVotes { c with deliberation_round := c.deliberation_round + 1 } = [] := by
  rw [List.eq_nil_iff_forall_not_mem]
  intro vote hVote
  have hVoteMem : vote ∈ c.council_votes := by
    exact (List.mem_filter.mp hVote).1
  have hVoteRound : vote.round = c.deliberation_round + 1 := by
    exact of_decide_eq_true (List.mem_filter.mp hVote).2
  have hLe : vote.round ≤ c.deliberation_round := hBounded vote hVoteMem
  exact Nat.ne_of_lt (Nat.lt_succ_of_le hLe) hVoteRound

/--
If the new current round is empty and at least one seated member remains, the
engine can identify a next council voter.
-/
theorem nextCouncilMember_some_of_empty_currentRoundVotes
    (c : ArbitrationCase)
    (hNoVotes : currentRoundVotes c = [])
    (hSeated : seatedCouncilMembers c ≠ []) :
    ∃ member, nextCouncilMember? c = some member := by
  cases hMembers : seatedCouncilMembers c with
  | nil =>
      exact False.elim (hSeated hMembers)
  | cons member rest =>
      refine ⟨member, ?_⟩
      simp [nextCouncilMember?, hMembers, hNoVotes]

/--
If a reachable state has closed the phase machine, it has also closed the
status flag.

`nextOpportunity` checks `status` first, while several proof statements talk in
terms of `phase`.  The no-stuck theorem needs those two views to agree on what
it means for a case to be closed.
-/
theorem continueDeliberation_phase_closed_implies_status_closed
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed") :
    t.case.status = "closed" := by
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
            simp [hDeliberation] at hClosed
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simp [hDeliberation] at hClosed

/--
In every reachable state, `phase = "closed"` implies `status = "closed"`.
-/
theorem reachable_phase_closed_implies_status_closed
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed") :
    s.case.status = "closed" := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step u t action hu hStep _ =>
      have hStepCore := stepCore_ok_of_step_ok u t action hStep
      by_cases hOpening : action.action_type = "record_opening_statement"
      · exact False.elim ((step_record_opening_statement_phase_ne_closed u t action hOpening hStepCore) hClosed)
      · by_cases hArgument : action.action_type = "submit_argument"
        · exact False.elim ((step_submit_argument_phase_ne_closed u t action hArgument hStepCore) hClosed)
        · by_cases hRebuttal : action.action_type = "submit_rebuttal"
          · exact False.elim ((step_submit_rebuttal_phase_ne_closed u t action hRebuttal hStepCore) hClosed)
          · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
            · exact False.elim ((step_submit_surrebuttal_phase_ne_closed u t action hSurrebuttal hStepCore) hClosed)
            · by_cases hClosing : action.action_type = "deliver_closing_statement"
              · exact False.elim ((step_deliver_closing_statement_phase_ne_closed u t action hClosing hStepCore) hClosed)
              · by_cases hPass : action.action_type = "pass_phase_opportunity"
                · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed u t action hPass hStepCore) hClosed)
                · by_cases hEvidence : action.action_type = "submit_evidence"
                  · exact False.elim ((step_submit_evidence_phase_ne_closed u t action hEvidence hStepCore) hClosed)
                  · by_cases hVote : action.action_type = "submit_council_vote"
                    · rcases step_submit_council_vote_result u t action hVote hStepCore with
                      ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                      let c1 := { u.case with council_votes := u.case.council_votes.concat {
                        member_id := memberId
                        round := u.case.deliberation_round
                        vote := trimString vote
                        rationale := trimString rationale
                      } }
                      exact continueDeliberation_phase_closed_implies_status_closed u t c1 (by
                        simpa [c1] using hDeliberation) (by
                        simpa [c1] using hCont) hClosed
                    · by_cases hRemoval : action.action_type = "remove_council_member"
                      · rcases step_remove_council_member_result u t action hRemoval hStepCore with
                        ⟨memberId, status, hDeliberation, hCont⟩
                        let c1 := {
                        u.case with council_members := u.case.council_members.map (fun (member : CouncilMember) =>
                          if member.member_id = memberId then
                            { member with status := trimString status }
                          else
                            member)
                        }
                        exact continueDeliberation_phase_closed_implies_status_closed u t c1 (by
                          simpa [c1] using hDeliberation) (by
                          simpa [c1] using hCont) hClosed
                      · by_cases hFail : action.action_type = "fail_opportunity"
                        · have hFailOp := step_fail_opportunity_result u t action hFail hStepCore
                          rcases failOpportunity_result u t action.payload hFailOp with hCouncil | hParty
                          · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                              c1, hC1, hDeliberation, _hSeated, _hFresh, hCont⟩
                            have hDeliberation1 : c1.phase = "deliberation" := by
                              rw [hC1]
                              simpa using hDeliberation
                            exact continueDeliberation_phase_closed_implies_status_closed
                              u t c1 hDeliberation1 hCont hClosed
                          · rcases hParty with ⟨_failure, _hEq, hNotClosed,
                              _hNotDeliberation, _hFailureType, _hFailureRole,
                              _hFailurePhase⟩
                            exact False.elim (hNotClosed hClosed)
                        · simp [stepCore] at hStepCore

/--
A successful opening step never reaches deliberation.

Openings can remain in `openings` or advance to `arguments`.  They do not skip
the intervening merits phases.
-/
theorem step_record_opening_statement_phase_ne_deliberation
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "deliberation" := by
  have hPhase : s.case.phase = "openings" := by
    by_cases hOpen : s.case.phase = "openings"
    · exact hOpen
    · have hClosed : s.case.phase != "openings" := by simpa using hOpen
      simp [stepCore, hType, hClosed] at hStep
      cases hStep
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  by_cases hAdvance : 1 ≤ s.case.openings.length
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]

/--
A successful argument step never reaches deliberation.

Arguments can remain in `arguments` or advance to `rebuttals`.
-/
theorem step_submit_argument_phase_ne_deliberation
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "deliberation" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "arguments"
        action.actor_role
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        "argument"
        s.policy.max_argument_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  have hPhase : s.case.phase = "arguments" := by
    unfold recordMeritsSubmission at hSubmit
    by_cases hArg : s.case.phase = "arguments"
    · exact hArg
    · have hClosed : s.case.phase != "arguments" := by simpa using hArg
      simp [hClosed] at hSubmit
      cases hSubmit
  rcases recordMeritsSubmission_with_materials_result
      s t "arguments" action.actor_role
      (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
      "argument" s.policy.max_argument_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  by_cases hAdvance : 1 ≤ s.case.arguments.length
  · simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase, hAdvance]
  · simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase, hAdvance]

/--
A successful rebuttal step never reaches deliberation.

Rebuttals can remain in `rebuttals` or advance to `surrebuttals`.
-/
theorem step_submit_rebuttal_phase_ne_deliberation
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "deliberation" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "rebuttals"
        action.actor_role
        "plaintiff"
        "rebuttal"
        s.policy.max_rebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  have hPhase : s.case.phase = "rebuttals" := by
    unfold recordMeritsSubmission at hSubmit
    by_cases hRebuttal : s.case.phase = "rebuttals"
    · exact hRebuttal
    · have hClosed : s.case.phase != "rebuttals" := by simpa using hRebuttal
      simp [hClosed] at hSubmit
      cases hSubmit
  rcases recordMeritsSubmission_with_materials_result
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase]

/--
A successful surrebuttal step never reaches deliberation.

Surrebuttal can remain in `surrebuttals` or advance to `closings`.
-/
theorem step_submit_surrebuttal_phase_ne_deliberation
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "deliberation" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "surrebuttals"
        action.actor_role
        "defendant"
        "surrebuttal"
        s.policy.max_surrebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  have hPhase : s.case.phase = "surrebuttals" := by
    unfold recordMeritsSubmission at hSubmit
    by_cases hSurrebuttal : s.case.phase = "surrebuttals"
    · exact hSurrebuttal
    · have hClosed : s.case.phase != "surrebuttals" := by simpa using hSurrebuttal
      simp [hClosed] at hSubmit
      cases hSubmit
  rcases recordMeritsSubmission_with_materials_result
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase]

/--
Passing an optional phase never reaches deliberation.

Passing rebuttal advances to `surrebuttals`.  Passing surrebuttal advances to
`closings`.
-/
theorem step_pass_phase_opportunity_phase_ne_deliberation
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "deliberation" := by
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
            simp [stateWithCase]
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
              simp [stateWithCase]
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem submitEvidence_source_meritsPhase
    (s t : ArbitrationState)
    (actorRole : String)
    (payload : Lean.Json)
    (hSubmit : submitEvidence s actorRole payload = .ok t) :
    meritsPhase s.case.phase := by
  by_cases hArgs : s.case.phase = "arguments"
  · exact Or.inr <| Or.inl hArgs
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · cases hEmpty : s.case.rebuttals.isEmpty with
      | true =>
          exact Or.inr <| Or.inr <| Or.inl hRebuttals
      | false =>
          simp [submitEvidence, hRebuttals, hEmpty] at hSubmit
          change Except.error "rebuttal evidence is closed" = .ok t at hSubmit
          cases hSubmit
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · cases hEmpty : s.case.surrebuttals.isEmpty with
        | true =>
            exact Or.inr <| Or.inr <| Or.inr <| Or.inl hSurrebuttals
        | false =>
            simp [submitEvidence, hSurrebuttals, hEmpty] at hSubmit
            change Except.error "surrebuttal evidence is closed" = .ok t at hSubmit
            cases hSubmit
      · simp [submitEvidence] at hSubmit
        change Except.error
          "submitted evidence is allowed only in arguments, rebuttals, and surrebuttals" = .ok t at hSubmit
        cases hSubmit

theorem step_submit_evidence_phase_eq_source
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase = s.case.phase := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  simp [stateWithCase, appendSubmittedEvidence]

theorem step_submit_evidence_phase_ne_deliberation
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "deliberation" := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  have hMerits : meritsPhase s.case.phase :=
    submitEvidence_source_meritsPhase s t action.actor_role action.payload hSubmit
  have hEq := step_submit_evidence_phase_eq_source s t action hType hStep
  intro hDeliberation
  rw [hEq] at hDeliberation
  simp [meritsPhase, hDeliberation] at hMerits

/--
A started bilateral phase still has a next role.

`plaintiffThenDefendant` is the executable selector for openings, arguments,
and closings.  The `bilateralStarted` predicate is the proof-layer summary of
the admissible prefixes in those phases.  This theorem says that the two views
agree.
-/
theorem plaintiffThenDefendant_some_of_bilateralStarted
    (phase plaintiffLabel defendantLabel : String)
    (xs : List Filing)
    (hStarted : bilateralStarted phase xs) :
    ∃ role, plaintiffThenDefendant xs plaintiffLabel defendantLabel = some role := by
  cases xs with
  | nil =>
      exact ⟨plaintiffLabel, by simp [plaintiffThenDefendant]⟩
  | cons filing rest =>
      cases rest with
      | nil =>
          exact ⟨defendantLabel, by simp [plaintiffThenDefendant]⟩
      | cons next tail =>
          simp [bilateralStarted] at hStarted

/--
If a deliberation round is not complete, the engine can still identify a next
voter.

The proof is the contrapositive of `nextCouncilMember_none_implies_round_complete`.
That earlier theorem established the hard direction.  This one packages the
forward consequence that the no-stuck theorem will use.
-/
theorem nextCouncilMember_some_of_incomplete_round
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hIntegrity : councilVoteIntegrity c)
    (hIncomplete : (currentRoundVotes c).length ≠ seatedCouncilMemberCount c) :
    ∃ member, nextCouncilMember? c = some member := by
  by_cases hNone : nextCouncilMember? c = none
  · have hComplete :=
      nextCouncilMember_none_implies_round_complete c hUnique hIntegrity hNone
    exact False.elim (hIncomplete hComplete)
  · cases hMember : nextCouncilMember? c with
    | none =>
        exact False.elim (hNone hMember)
    | some member =>
        refine ⟨member, ?_⟩
        rfl

/--
During deliberation, `continueDeliberation` leaves the phase in only two
places: it either keeps the case in deliberation or closes it.

This is the minimal structural fact needed to rule council actions out of the
merits-phase invariant below.
-/
theorem continueDeliberation_phase_deliberation_or_closed
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t) :
    t.case.phase = "deliberation" ∨ t.case.phase = "closed" := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        exact Or.inr rfl
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          exact Or.inr rfl
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact Or.inr rfl
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact Or.inl hDeliberation
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    exact Or.inl hDeliberation

/--
`pristineCouncilState` depends only on the council roster, the stored votes,
and the deliberation round.

Merits actions change none of those fields.  This congruence lemma packages
that observation so the action-specific proofs can stay short.
-/
theorem pristineCouncilState_congr
    {c d : ArbitrationCase}
    (hVotes : d.council_votes = c.council_votes)
    (hRound : d.deliberation_round = c.deliberation_round)
    (hMembers : d.council_members = c.council_members)
    (hPristine : pristineCouncilState c) :
    pristineCouncilState d := by
  rcases hPristine with ⟨hVotesNil, hRoundOne, hSeated⟩
  refine ⟨?_, ?_, ?_⟩
  · simpa [hVotes] using hVotesNil
  · simpa [hRound] using hRoundOne
  · simpa [hMembers] using hSeated

/--
Advancing between merits phases does not change stored council votes.

`advanceAfterMerits` changes only the phase marker.  The council record remains
verbatim.
-/
theorem advanceAfterMerits_preserves_council_votes
    (c : ArbitrationCase) :
    (advanceAfterMerits c).council_votes = c.council_votes := by
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

/--
Advancing between merits phases does not change the deliberation round.
-/
theorem advanceAfterMerits_preserves_deliberation_round
    (c : ArbitrationCase) :
    (advanceAfterMerits c).deliberation_round = c.deliberation_round := by
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

/--
Advancing between merits phases does not change the council roster.
-/
theorem advanceAfterMerits_preserves_council_members
    (c : ArbitrationCase) :
    (advanceAfterMerits c).council_members = c.council_members := by
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

/--
Adding a merits filing preserves the stored council votes.

The helper updates one filing list, then runs `advanceAfterMerits`.  Neither
step rewrites the council vote list.
-/
theorem addFiling_preserves_council_votes
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).council_votes = c.council_votes := by
  by_cases hOpenings : phase = "openings"
  · subst hOpenings
    let filing : Filing := { phase := "openings", role := role, text := text }
    let c1 : ArbitrationCase := { c with openings := c.openings.concat filing }
    simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_votes c1
  · by_cases hArguments : phase = "arguments"
    · subst hArguments
      let filing : Filing := { phase := "arguments", role := role, text := text }
      let c1 : ArbitrationCase := { c with arguments := c.arguments.concat filing }
      simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_votes c1
    · by_cases hRebuttals : phase = "rebuttals"
      · subst hRebuttals
        let filing : Filing := { phase := "rebuttals", role := role, text := text }
        let c1 : ArbitrationCase := { c with rebuttals := c.rebuttals.concat filing }
        simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_votes c1
      · by_cases hSurrebuttals : phase = "surrebuttals"
        · subst hSurrebuttals
          let filing : Filing := { phase := "surrebuttals", role := role, text := text }
          let c1 : ArbitrationCase := { c with surrebuttals := c.surrebuttals.concat filing }
          simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_votes c1
        · by_cases hClosings : phase = "closings"
          · subst hClosings
            let filing : Filing := { phase := "closings", role := role, text := text }
            let c1 : ArbitrationCase := { c with closings := c.closings.concat filing }
            simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_votes c1
          · simpa [addFiling, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]
              using advanceAfterMerits_preserves_council_votes c

/--
Adding a merits filing preserves the deliberation round.
-/
theorem addFiling_preserves_deliberation_round
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).deliberation_round = c.deliberation_round := by
  by_cases hOpenings : phase = "openings"
  · subst hOpenings
    let filing : Filing := { phase := "openings", role := role, text := text }
    let c1 : ArbitrationCase := { c with openings := c.openings.concat filing }
    simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_deliberation_round c1
  · by_cases hArguments : phase = "arguments"
    · subst hArguments
      let filing : Filing := { phase := "arguments", role := role, text := text }
      let c1 : ArbitrationCase := { c with arguments := c.arguments.concat filing }
      simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_deliberation_round c1
    · by_cases hRebuttals : phase = "rebuttals"
      · subst hRebuttals
        let filing : Filing := { phase := "rebuttals", role := role, text := text }
        let c1 : ArbitrationCase := { c with rebuttals := c.rebuttals.concat filing }
        simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_deliberation_round c1
      · by_cases hSurrebuttals : phase = "surrebuttals"
        · subst hSurrebuttals
          let filing : Filing := { phase := "surrebuttals", role := role, text := text }
          let c1 : ArbitrationCase := { c with surrebuttals := c.surrebuttals.concat filing }
          simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_deliberation_round c1
        · by_cases hClosings : phase = "closings"
          · subst hClosings
            let filing : Filing := { phase := "closings", role := role, text := text }
            let c1 : ArbitrationCase := { c with closings := c.closings.concat filing }
            simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_deliberation_round c1
          · simpa [addFiling, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]
              using advanceAfterMerits_preserves_deliberation_round c

/--
Adding a merits filing preserves the council roster.
-/
theorem addFiling_preserves_council_members
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).council_members = c.council_members := by
  by_cases hOpenings : phase = "openings"
  · subst hOpenings
    let filing : Filing := { phase := "openings", role := role, text := text }
    let c1 : ArbitrationCase := { c with openings := c.openings.concat filing }
    simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_members c1
  · by_cases hArguments : phase = "arguments"
    · subst hArguments
      let filing : Filing := { phase := "arguments", role := role, text := text }
      let c1 : ArbitrationCase := { c with arguments := c.arguments.concat filing }
      simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_members c1
    · by_cases hRebuttals : phase = "rebuttals"
      · subst hRebuttals
        let filing : Filing := { phase := "rebuttals", role := role, text := text }
        let c1 : ArbitrationCase := { c with rebuttals := c.rebuttals.concat filing }
        simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_members c1
      · by_cases hSurrebuttals : phase = "surrebuttals"
        · subst hSurrebuttals
          let filing : Filing := { phase := "surrebuttals", role := role, text := text }
          let c1 : ArbitrationCase := { c with surrebuttals := c.surrebuttals.concat filing }
          simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_members c1
        · by_cases hClosings : phase = "closings"
          · subst hClosings
            let filing : Filing := { phase := "closings", role := role, text := text }
            let c1 : ArbitrationCase := { c with closings := c.closings.concat filing }
            simpa [addFiling, filing, c1] using advanceAfterMerits_preserves_council_members c1
          · simpa [addFiling, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]
              using advanceAfterMerits_preserves_council_members c

/--
Adding exhibits or technical reports does not change council data.
-/
theorem appendSupplementalMaterials_preserves_council_votes
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport) :
    (appendSupplementalMaterials c offered reports).council_votes = c.council_votes := by
  simp [appendSupplementalMaterials]

theorem appendSupplementalMaterials_preserves_deliberation_round
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport) :
    (appendSupplementalMaterials c offered reports).deliberation_round = c.deliberation_round := by
  simp [appendSupplementalMaterials]

theorem appendSupplementalMaterials_preserves_council_members
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport) :
    (appendSupplementalMaterials c offered reports).council_members = c.council_members := by
  simp [appendSupplementalMaterials]

/--
Successful initialization starts with no council votes, round one, and every
member seated.

This is stronger than the case-frame theorem.  It identifies the full
deliberation metadata that should persist unchanged until the case reaches the
first council turn.
-/
theorem initializeCase_establishes_pristineCouncilState
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    pristineCouncilState s.case := by
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
                · simp [stateWithCase, hPolicy, hProposition, hEvidence, hEmpty, hLength,
                    hInvalid, hDuplicate] at hInit
                  cases hInit
                  refine ⟨rfl, rfl, ?_⟩
                  intro member hMem
                  simp at hMem
                  rcases hMem with ⟨source, hSourceMem, rfl⟩
                  simp [memberIsSeated]

theorem step_record_opening_statement_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact pristineCouncilState_congr
    (c := s.case)
    (d := addFiling s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_council_votes s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_deliberation_round s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_council_members s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    hPristine

theorem step_submit_argument_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
  have hSubmit :
      recordMeritsSubmission
        s
        "arguments"
        action.actor_role
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        "argument"
        s.policy.max_argument_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_result
      s t "arguments" action.actor_role
      (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
      "argument" s.policy.max_argument_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  exact pristineCouncilState_congr
    (c := s.case)
    (d := appendSupplementalMaterials
      (addFiling s.case "arguments"
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
      offered
      reports)
    (by
      rw [appendSupplementalMaterials_preserves_council_votes]
      exact addFiling_preserves_council_votes s.case "arguments"
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
    (by
      rw [appendSupplementalMaterials_preserves_deliberation_round]
      exact addFiling_preserves_deliberation_round s.case "arguments"
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
    (by
      rw [appendSupplementalMaterials_preserves_council_members]
      exact addFiling_preserves_council_members s.case "arguments"
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
    hPristine

theorem step_submit_rebuttal_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
  have hSubmit :
      recordMeritsSubmission
        s
        "rebuttals"
        action.actor_role
        "plaintiff"
        "rebuttal"
        s.policy.max_rebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_result
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  exact pristineCouncilState_congr
    (c := s.case)
    (d := appendSupplementalMaterials
      (addFiling s.case "rebuttals" "plaintiff" (trimString rawText))
      offered
      reports)
    (by
      rw [appendSupplementalMaterials_preserves_council_votes]
      exact addFiling_preserves_council_votes s.case "rebuttals" "plaintiff" (trimString rawText))
    (by
      rw [appendSupplementalMaterials_preserves_deliberation_round]
      exact addFiling_preserves_deliberation_round s.case "rebuttals" "plaintiff" (trimString rawText))
    (by
      rw [appendSupplementalMaterials_preserves_council_members]
      exact addFiling_preserves_council_members s.case "rebuttals" "plaintiff" (trimString rawText))
    hPristine

theorem step_submit_surrebuttal_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
  have hSubmit :
      recordMeritsSubmission
        s
        "surrebuttals"
        action.actor_role
        "defendant"
        "surrebuttal"
        s.policy.max_surrebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_result
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  exact pristineCouncilState_congr
    (c := s.case)
    (d := appendSupplementalMaterials
      (addFiling s.case "surrebuttals" "defendant" (trimString rawText))
      offered
      reports)
    (by
      rw [appendSupplementalMaterials_preserves_council_votes]
      exact addFiling_preserves_council_votes s.case "surrebuttals" "defendant" (trimString rawText))
    (by
      rw [appendSupplementalMaterials_preserves_deliberation_round]
      exact addFiling_preserves_deliberation_round s.case "surrebuttals" "defendant" (trimString rawText))
    (by
      rw [appendSupplementalMaterials_preserves_council_members]
      exact addFiling_preserves_council_members s.case "surrebuttals" "defendant" (trimString rawText))
    hPristine

theorem step_deliver_closing_statement_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact pristineCouncilState_congr
    (c := s.case)
    (d := addFiling s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_council_votes s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_deliberation_round s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_council_members s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    hPristine

theorem step_pass_phase_opportunity_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
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
    | ok _ =>
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            exact pristineCouncilState_congr
              (c := s.case)
              (d := { s.case with phase := "surrebuttals" })
              rfl rfl rfl hPristine
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
      | ok _ =>
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              exact pristineCouncilState_congr
                (c := s.case)
                (d := { s.case with phase := "closings" })
                rfl rfl rfl hPristine
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem step_submit_evidence_preserves_pristineCouncilState
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hPristine : pristineCouncilState s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    pristineCouncilState t.case := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  exact pristineCouncilState_congr
    (by simp [stateWithCase, appendSubmittedEvidence])
    (by simp [stateWithCase, appendSubmittedEvidence])
    (by simp [stateWithCase, appendSubmittedEvidence])
    hPristine

/--
Every reachable merits-phase state still has pristine deliberation metadata.

The council actions are unavailable before deliberation.  Every other public
action preserves the vote list, the round counter, and council-member seating.
-/
theorem reachable_meritsPhase_pristineCouncilState
    (s : ArbitrationState)
    (hs : Reachable s)
    (hMerits : meritsPhase s.case.phase) :
    pristineCouncilState s.case := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_pristineCouncilState req s hInit
  | step u t action hu hStep ih =>
      have hStepCore := stepCore_ok_of_step_ok u t action hStep
      by_cases hOpening : action.action_type = "record_opening_statement"
      · have hPhase : u.case.phase = "openings" := by
          by_cases hOpen : u.case.phase = "openings"
          · exact hOpen
          · have hClosed : u.case.phase != "openings" := by simpa using hOpen
            simp [stepCore, hOpening, hClosed] at hStepCore
            cases hStepCore
        have hMeritsU : meritsPhase u.case.phase := Or.inl hPhase
        exact step_record_opening_statement_preserves_pristineCouncilState
          u t action hOpening (ih hMeritsU) hStepCore
      · by_cases hArgument : action.action_type = "submit_argument"
        · have hSubmit :
              recordMeritsSubmission
                u
                "arguments"
                action.actor_role
                (if u.case.arguments.isEmpty then "plaintiff" else "defendant")
                "argument"
                u.policy.max_argument_chars
                true
                action.payload = .ok t := by
            simpa [stepCore, hArgument] using hStepCore
          have hPhase : u.case.phase = "arguments" := by
            by_cases hArg : u.case.phase = "arguments"
            · exact hArg
            · unfold recordMeritsSubmission at hSubmit
              simp [hArg] at hSubmit
              cases hSubmit
          have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inl hPhase
          exact step_submit_argument_preserves_pristineCouncilState
            u t action hArgument (ih hMeritsU) hStepCore
        · by_cases hRebuttal : action.action_type = "submit_rebuttal"
          · have hSubmit :
                recordMeritsSubmission
                  u
                  "rebuttals"
                  action.actor_role
                  "plaintiff"
                  "rebuttal"
                  u.policy.max_rebuttal_chars
                  true
                  action.payload = .ok t := by
              simpa [stepCore, hRebuttal] using hStepCore
            have hPhase : u.case.phase = "rebuttals" := by
              by_cases hRebuttalPhase : u.case.phase = "rebuttals"
              · exact hRebuttalPhase
              · unfold recordMeritsSubmission at hSubmit
                simp [hRebuttalPhase] at hSubmit
                cases hSubmit
            have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inr <| Or.inl hPhase
            exact step_submit_rebuttal_preserves_pristineCouncilState
              u t action hRebuttal (ih hMeritsU) hStepCore
          · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
            · have hSubmit :
                  recordMeritsSubmission
                    u
                    "surrebuttals"
                    action.actor_role
                    "defendant"
                    "surrebuttal"
                    u.policy.max_surrebuttal_chars
                    true
                    action.payload = .ok t := by
                simpa [stepCore, hSurrebuttal] using hStepCore
              have hPhase : u.case.phase = "surrebuttals" := by
                by_cases hSurrebuttalPhase : u.case.phase = "surrebuttals"
                · exact hSurrebuttalPhase
                · unfold recordMeritsSubmission at hSubmit
                  simp [hSurrebuttalPhase] at hSubmit
                  cases hSubmit
              have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inr <| Or.inr <| Or.inl hPhase
              exact step_submit_surrebuttal_preserves_pristineCouncilState
                u t action hSurrebuttal (ih hMeritsU) hStepCore
            · by_cases hClosing : action.action_type = "deliver_closing_statement"
              · have hPhase : u.case.phase = "closings" := by
                  by_cases hClosingPhase : u.case.phase = "closings"
                  · exact hClosingPhase
                  · have hClosed : u.case.phase != "closings" := by simpa using hClosingPhase
                    simp [stepCore, hClosing, hClosed] at hStepCore
                    cases hStepCore
                have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inr <| Or.inr <| Or.inr hPhase
                exact step_deliver_closing_statement_preserves_pristineCouncilState
                  u t action hClosing (ih hMeritsU) hStepCore
              · by_cases hPass : action.action_type = "pass_phase_opportunity"
                · by_cases hRebuttals : u.case.phase = "rebuttals"
                  · have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inr <| Or.inl hRebuttals
                    exact step_pass_phase_opportunity_preserves_pristineCouncilState
                      u t action hPass (ih hMeritsU) hStepCore
                  · by_cases hSurrebuttals : u.case.phase = "surrebuttals"
                    · have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inr <| Or.inr <| Or.inl hSurrebuttals
                      exact step_pass_phase_opportunity_preserves_pristineCouncilState
                        u t action hPass (ih hMeritsU) hStepCore
                    · simp [stepCore, hPass, hRebuttals, hSurrebuttals] at hStepCore
                · by_cases hEvidence : action.action_type = "submit_evidence"
                  · have hSubmit : submitEvidence u action.actor_role action.payload = .ok t := by
                      simpa [stepCore, hEvidence] using hStepCore
                    have hMeritsU : meritsPhase u.case.phase :=
                      submitEvidence_source_meritsPhase u t action.actor_role action.payload hSubmit
                    exact step_submit_evidence_preserves_pristineCouncilState
                      u t action hEvidence (ih hMeritsU) hStepCore
                  · by_cases hVote : action.action_type = "submit_council_vote"
                    · rcases step_submit_council_vote_result u t action hVote hStepCore with
                      ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                      let c1 := { u.case with council_votes := u.case.council_votes.concat {
                        member_id := memberId
                        round := u.case.deliberation_round
                        vote := trimString vote
                        rationale := trimString rationale
                      } }
                      have hPhaseOut := continueDeliberation_phase_deliberation_or_closed u t c1
                        (by simpa [c1] using hDeliberation)
                        (by simpa [c1] using hCont)
                      rcases hPhaseOut with hDelibOut | hClosedOut
                      · simp [meritsPhase, hDelibOut] at hMerits
                      · simp [meritsPhase, hClosedOut] at hMerits
                    · by_cases hRemoval : action.action_type = "remove_council_member"
                      · rcases step_remove_council_member_result u t action hRemoval hStepCore with
                        ⟨memberId, status, hDeliberation, hCont⟩
                        let c1 := {
                          u.case with council_members := u.case.council_members.map (fun (member : CouncilMember) =>
                            if member.member_id = memberId then
                              { member with status := trimString status }
                            else
                              member)
                        }
                        have hPhaseOut := continueDeliberation_phase_deliberation_or_closed u t c1
                          (by simpa [c1] using hDeliberation)
                          (by simpa [c1] using hCont)
                        rcases hPhaseOut with hDelibOut | hClosedOut
                        · simp [meritsPhase, hDelibOut] at hMerits
                        · simp [meritsPhase, hClosedOut] at hMerits
                      · by_cases hFail : action.action_type = "fail_opportunity"
                        · have hFailOp := step_fail_opportunity_result u t action hFail hStepCore
                          rcases failOpportunity_result u t action.payload hFailOp with hCouncil | hParty
                          · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message,
                              c1, hC1, hDeliberation, _hSeated, _hFresh, hCont⟩
                            have hPhaseOut := continueDeliberation_phase_deliberation_or_closed u t c1
                              (by
                                rw [hC1]
                                simpa using hDeliberation)
                              hCont
                            rcases hPhaseOut with hDelibOut | hClosedOut
                            · simp [meritsPhase, hDelibOut] at hMerits
                            · simp [meritsPhase, hClosedOut] at hMerits
                          · rcases hParty with ⟨_failure, rfl, _hNotClosed,
                              _hNotDeliberation, _hFailureType, _hFailureRole,
                              _hFailurePhase⟩
                            exact pristineCouncilState_congr rfl rfl rfl (ih (by
                              simpa [meritsPhase, stateWithCase] using hMerits))
                        · simp [stepCore] at hStepCore

/--
A positive seated-member count gives a nonempty seated roster.

This is the small list fact needed in the round-advance branch of the
deliberation liveness proof.
-/
theorem seatedCouncilMembers_nonempty_of_count_positive
    (c : ArbitrationCase)
    (hPositive : 0 < seatedCouncilMemberCount c) :
    seatedCouncilMembers c ≠ [] := by
  cases hMembers : seatedCouncilMembers c with
  | nil =>
      simp [seatedCouncilMemberCount, hMembers] at hPositive
  | cons member rest =>
      simp

/--
If every council member is still seated and the council roster is nonempty, the
seated roster is also nonempty.
-/
theorem seatedCouncilMembers_nonempty_of_pristine
    (c : ArbitrationCase)
    (hPristine : pristineCouncilState c)
    (hNonempty : c.council_members ≠ []) :
    seatedCouncilMembers c ≠ [] := by
  rcases hPristine with ⟨_hVotes, _hRound, hSeated⟩
  cases hMembers : c.council_members with
  | nil =>
      exact False.elim (hNonempty hMembers)
  | cons member rest =>
      have hMemberSeated : memberIsSeated member = true := by
        exact hSeated member (by simp [hMembers])
      simp [seatedCouncilMembers, hMembers, hMemberSeated]

/--
A live result from `continueDeliberation` still has a next council voter.

This theorem isolates the operational content of the deliberation helper.  If
the helper closes the case, the theorem does not apply.  If it returns a live
deliberation state, then one of two things happened.

First: the current round was incomplete, so the helper returned the same case.
In that branch, the existing council-integrity theorem already says that a
seated unvoted member exists.

Second: the round was complete but not final, so the helper advanced the round
counter.  In that branch, the new round starts with no current-round votes.
The only remaining obligation is to show that at least one seated member
remains, which follows from the positive vote threshold and the fact that the
helper did not take the "too few seated members" closing branch.
-/
theorem continueDeliberation_live_has_nextCouncilMember
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hPositive : 0 < s.policy.required_votes_for_decision)
    (hUnique : councilIdsUnique c)
    (hIntegrity : councilVoteIntegrity c)
    (hCont : continueDeliberation s c = .ok t)
    (hPhase : t.case.phase = "deliberation")
    (hStatus : t.case.status = "active") :
    ∃ member, nextCouncilMember? t.case = some member := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        simp at hPhase
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          simp at hStatus
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp at hStatus
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            let c1 : ArbitrationCase := { c with deliberation_round := c.deliberation_round + 1 }
            have hNoVotes : currentRoundVotes c1 = [] := by
              exact currentRoundVotes_empty_after_round_increment c hIntegrity.2.2
            have hEnoughSeated : s.policy.required_votes_for_decision ≤ seatedCouncilMemberCount c := by
              exact Nat.le_of_not_gt hTooFew
            have hSeatedPositive : 0 < seatedCouncilMemberCount c := by
              exact Nat.lt_of_lt_of_le hPositive hEnoughSeated
            have hSeatedNonempty : seatedCouncilMembers c1 ≠ [] := by
              have hSeatedNonempty0 :
                  seatedCouncilMembers c ≠ [] := by
                exact seatedCouncilMembers_nonempty_of_count_positive c hSeatedPositive
              simpa [c1, seatedCouncilMembers] using hSeatedNonempty0
            exact nextCouncilMember_some_of_empty_currentRoundVotes c1 hNoVotes hSeatedNonempty
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simpa using nextCouncilMember_some_of_incomplete_round c hUnique hIntegrity hRoundComplete

/--
A reachable live deliberation state still has a next council voter.

Initialization cannot produce deliberation.  The only successful public steps
that can end in deliberation are: the second closing statement, a council vote,
or a council-member removal.  The first case uses the merits-phase invariant to
show that deliberation starts with an untouched council record.  The latter two
cases use the direct `continueDeliberation` theorem above.
-/
theorem reachable_deliberation_has_nextCouncilMember
    (s : ArbitrationState)
    (hs : Reachable s)
    (hPhase : s.case.phase = "deliberation")
    (hStatus : s.case.status = "active") :
    ∃ member, nextCouncilMember? s.case = some member := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hPhase
  | step u t action hu hStep _ =>
      have hStepCore := stepCore_ok_of_step_ok u t action hStep
      by_cases hOpening : action.action_type = "record_opening_statement"
      · exact False.elim ((step_record_opening_statement_phase_ne_deliberation u t action hOpening hStepCore) hPhase)
      · by_cases hArgument : action.action_type = "submit_argument"
        · exact False.elim ((step_submit_argument_phase_ne_deliberation u t action hArgument hStepCore) hPhase)
        · by_cases hRebuttal : action.action_type = "submit_rebuttal"
          · exact False.elim ((step_submit_rebuttal_phase_ne_deliberation u t action hRebuttal hStepCore) hPhase)
          · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
            · exact False.elim ((step_submit_surrebuttal_phase_ne_deliberation u t action hSurrebuttal hStepCore) hPhase)
            · by_cases hClosing : action.action_type = "deliver_closing_statement"
              · have hPhaseU : u.case.phase = "closings" := by
                  by_cases hClosingPhase : u.case.phase = "closings"
                  · exact hClosingPhase
                  · have hClosed : u.case.phase != "closings" := by simpa using hClosingPhase
                    simp [stepCore, hClosing, hClosed] at hStepCore
                    cases hStepCore
                have hMeritsU : meritsPhase u.case.phase := Or.inr <| Or.inr <| Or.inr <| Or.inr hPhaseU
                have hPristineU : pristineCouncilState u.case :=
                  reachable_meritsPhase_pristineCouncilState u hu hMeritsU
                have hPristineT : pristineCouncilState t.case :=
                  step_deliver_closing_statement_preserves_pristineCouncilState
                    u t action hClosing hPristineU hStepCore
                have hNonemptyCouncil : t.case.council_members ≠ [] := by
                  exact step_preserves_nonempty_council u t action
                    (reachable_nonempty_council u hu) hStep
                have hNoVotes : currentRoundVotes t.case = [] := by
                  rcases hPristineT with ⟨hVotes, hRound, _hSeated⟩
                  simp [currentRoundVotes, hVotes, hRound]
                have hSeatedNonempty : seatedCouncilMembers t.case ≠ [] := by
                  exact seatedCouncilMembers_nonempty_of_pristine t.case hPristineT hNonemptyCouncil
                exact nextCouncilMember_some_of_empty_currentRoundVotes t.case hNoVotes hSeatedNonempty
              · by_cases hPass : action.action_type = "pass_phase_opportunity"
                · exact False.elim ((step_pass_phase_opportunity_phase_ne_deliberation u t action hPass hStepCore) hPhase)
                · by_cases hEvidence : action.action_type = "submit_evidence"
                  · exact False.elim ((step_submit_evidence_phase_ne_deliberation u t action hEvidence hStepCore) hPhase)
                  · by_cases hVote : action.action_type = "submit_council_vote"
                    · rcases step_submit_council_vote_details u t action hVote hStepCore with
                      ⟨memberId, vote, rationale, _hPhaseU, hSeated, hFresh, hCont⟩
                      let c1 := { u.case with council_votes := u.case.council_votes.concat {
                        member_id := memberId
                        round := u.case.deliberation_round
                        vote := trimString vote
                        rationale := trimString rationale
                      } }
                      have hPositive : 0 < u.policy.required_votes_for_decision :=
                        reachable_required_votes_positive u hu
                      have hUniqueU : councilIdsUnique u.case :=
                        reachable_councilIdsUnique u hu
                      have hIntegrityU : councilVoteIntegrity u.case :=
                        reachable_councilVoteIntegrity u hu
                      have hIntegrity1 : councilVoteIntegrity c1 := by
                        exact appendCurrentRoundVote_preserves_councilVoteIntegrity
                          u.case memberId vote rationale hIntegrityU hSeated hFresh
                      have hUnique1 : councilIdsUnique c1 := by
                        simpa [c1, councilIdsUnique, councilMemberIds] using hUniqueU
                      exact continueDeliberation_live_has_nextCouncilMember
                        u t c1 hPositive hUnique1 hIntegrity1
                        (by simpa [c1] using hCont) hPhase hStatus
                    · by_cases hRemoval : action.action_type = "remove_council_member"
                      · rcases step_remove_council_member_details u t action hRemoval hStepCore with
                        ⟨memberId, status, _hPhaseU, _hSeated, hFresh, _hStatus, hCont⟩
                        let c1 := {
                          u.case with council_members := u.case.council_members.map (fun (member : CouncilMember) =>
                            if member.member_id = memberId then
                              { member with status := trimString status }
                            else
                              member)
                        }
                        have hPositive : 0 < u.policy.required_votes_for_decision :=
                          reachable_required_votes_positive u hu
                        have hUniqueU : councilIdsUnique u.case :=
                          reachable_councilIdsUnique u hu
                        have hIntegrityU : councilVoteIntegrity u.case :=
                          reachable_councilVoteIntegrity u hu
                        have hIntegrity1 : councilVoteIntegrity c1 := by
                          simpa [c1] using removeUnvotedCouncilMember_preserves_councilVoteIntegrity
                            u.case memberId status hIntegrityU hFresh
                        have hUnique1 : councilIdsUnique c1 := by
                          unfold councilIdsUnique
                          unfold councilIdsUnique at hUniqueU
                          simpa [c1, councilMemberIds_status_update] using hUniqueU
                        exact continueDeliberation_live_has_nextCouncilMember
                          u t c1 hPositive hUnique1 hIntegrity1
                          (by simpa [c1] using hCont) hPhase hStatus
                      · by_cases hFail : action.action_type = "fail_opportunity"
                        · have hFailOp := step_fail_opportunity_result u t action hFail hStepCore
                          rcases failOpportunity_result u t action.payload hFailOp with hCouncil | hParty
                          · rcases hCouncil with ⟨memberId, reason, opportunityId, message,
                              c1, hC1, hDeliberation, _hSeated, hFresh, hCont⟩
                            have hPositive : 0 < u.policy.required_votes_for_decision :=
                              reachable_required_votes_positive u hu
                            have hUniqueU : councilIdsUnique u.case :=
                              reachable_councilIdsUnique u hu
                            have hIntegrityU : councilVoteIntegrity u.case :=
                              reachable_councilVoteIntegrity u hu
                            have hFreshIds : memberId ∉ currentRoundVoteIds u.case := by
                              simpa [currentRoundVoteIds] using hFresh
                            have hIntegrity1 : councilVoteIntegrity c1 := by
                              rw [hC1]
                              exact failUnvotedCouncilMember_preserves_councilVoteIntegrity
                                u.case memberId reason opportunityId message hIntegrityU hFreshIds
                            have hUnique1 : councilIdsUnique c1 := by
                              rw [hC1]
                              unfold councilIdsUnique
                              unfold councilIdsUnique at hUniqueU
                              simpa [councilMemberIds_failure_update] using hUniqueU
                            exact continueDeliberation_live_has_nextCouncilMember
                              u t c1 hPositive hUnique1 hIntegrity1 hCont hPhase hStatus
                          · rcases hParty with ⟨_failure, rfl, _hNotClosed,
                              _hNotDeliberation, _hFailureType, _hFailureRole,
                              _hFailurePhase⟩
                            simp [stateWithCase] at hStatus
                        · simp [stepCore] at hStepCore

/--
In a reachable live deliberation state, the summary records that the current
round still has a voter slot available.
-/
theorem reachable_live_deliberation_deliberationSummary_has_round_capacity
    (s : ArbitrationState)
    (hs : Reachable s)
    (hPhase : s.case.phase = "deliberation")
    (hStatus : s.case.status = "active") :
    (deliberationSummary s).current_round_vote_count <
      (deliberationSummary s).seated_count := by
  rcases reachable_deliberation_has_nextCouncilMember s hs hPhase hStatus with
    ⟨member, hNext⟩
  have hUnique : councilIdsUnique s.case := reachable_councilIdsUnique s hs
  have hIntegrity : councilVoteIntegrity s.case := reachable_councilVoteIntegrity s hs
  exact nextCouncilMember_some_implies_deliberationSummary_round_capacity
    s member hUnique hIntegrity hNext

/--
Every reachable active state has a next opportunity.

This is the Stage 3 theorem.  It says the engine does not strand a live case.
The proof splits by phase.

For the merits phases, `phaseShape` tells us exactly which filing prefixes are
reachable, and those prefixes line up with the executable selectors in
`nextOpportunityForPhase`.

For deliberation, the previous theorem supplies the next council voter.
-/
theorem reachable_active_has_nextOpportunity
    (s : ArbitrationState)
    (hs : Reachable s)
    (hStatus : s.case.status = "active") :
    (nextOpportunity s).terminal = false ∧
      ∃ opp, (nextOpportunity s).opportunity = some opp := by
  have hShape : phaseShape s.case := reachable_phaseShape s hs
  by_cases hOpenings : s.case.phase = "openings"
  · have hOpeningShape :
        bilateralStarted "openings" s.case.openings ∧
          s.case.arguments = [] ∧
          s.case.rebuttals = [] ∧
          s.case.surrebuttals = [] ∧
          s.case.closings = [] := by
      simpa [phaseShape, hOpenings] using hShape
    have hStarted : bilateralStarted "openings" s.case.openings := hOpeningShape.1
    rcases plaintiffThenDefendant_some_of_bilateralStarted
        "openings" "plaintiff" "defendant" s.case.openings hStarted with
      ⟨role, hRole⟩
    refine ⟨?_, ?_⟩
    · simp [nextOpportunity, hStatus, nextOpportunityForPhase, hOpenings, hRole]
    · refine ⟨{
        opportunity_id := s!"openings:{role}"
        role := role
        phase := "openings"
        objective := s!"{role} opening statement"
        allowed_tools := ["record_opening_statement"]
      }, ?_⟩
      simp [nextOpportunity, hStatus, nextOpportunityForPhase, hOpenings, hRole]
  · by_cases hArguments : s.case.phase = "arguments"
    · have hArgShape :
          bilateralComplete "openings" s.case.openings ∧
            bilateralStarted "arguments" s.case.arguments ∧
            s.case.rebuttals = [] ∧
            s.case.surrebuttals = [] ∧
            s.case.closings = [] := by
        simpa [phaseShape, hArguments] using hShape
      have hStarted : bilateralStarted "arguments" s.case.arguments := hArgShape.2.1
      rcases plaintiffThenDefendant_some_of_bilateralStarted
          "arguments" "plaintiff" "defendant" s.case.arguments hStarted with
        ⟨role, hRole⟩
      refine ⟨?_, ?_⟩
      · simp [nextOpportunity, hStatus, nextOpportunityForPhase, hArguments, hRole]
      · refine ⟨{
          opportunity_id := s!"arguments:{role}"
          role := role
          phase := "arguments"
          objective := s!"{role} merits argument"
          allowed_tools := ["submit_evidence", "submit_argument"]
        }, ?_⟩
        simp [nextOpportunity, hStatus, nextOpportunityForPhase, hArguments, hRole]
    · by_cases hRebuttals : s.case.phase = "rebuttals"
      · have hRebuttalShape :
            bilateralComplete "openings" s.case.openings ∧
              bilateralComplete "arguments" s.case.arguments ∧
              s.case.rebuttals = [] ∧
              s.case.surrebuttals = [] ∧
              s.case.closings = [] := by
          simpa [phaseShape, hRebuttals] using hShape
        have hEmpty : s.case.rebuttals = [] := hRebuttalShape.2.2.1
        refine ⟨?_, ?_⟩
        · simp [nextOpportunity, hStatus, nextOpportunityForPhase, hRebuttals, hEmpty]
        · refine ⟨{
            opportunity_id := "rebuttals:plaintiff"
            role := "plaintiff"
            phase := "rebuttals"
            may_pass := true
            objective := "plaintiff rebuttal"
            allowed_tools := ["submit_evidence", "submit_rebuttal", "pass_phase_opportunity"]
          }, ?_⟩
          simp [nextOpportunity, hStatus, nextOpportunityForPhase, hRebuttals, hEmpty]
      · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
        · have hSurrebuttalShape :
              bilateralComplete "openings" s.case.openings ∧
                bilateralComplete "arguments" s.case.arguments ∧
                plaintiffOptionalSequence "rebuttals" s.case.rebuttals ∧
                s.case.surrebuttals = [] ∧
                s.case.closings = [] := by
            simpa [phaseShape, hSurrebuttals] using hShape
          have hEmpty : s.case.surrebuttals = [] := hSurrebuttalShape.2.2.2.1
          refine ⟨?_, ?_⟩
          · simp [nextOpportunity, hStatus, nextOpportunityForPhase, hSurrebuttals, hEmpty]
          · refine ⟨{
              opportunity_id := "surrebuttals:defendant"
              role := "defendant"
              phase := "surrebuttals"
              may_pass := true
              objective := "defendant surrebuttal"
              allowed_tools := ["submit_evidence", "submit_surrebuttal", "pass_phase_opportunity"]
            }, ?_⟩
            simp [nextOpportunity, hStatus, nextOpportunityForPhase, hSurrebuttals, hEmpty]
        · by_cases hClosings : s.case.phase = "closings"
          · have hClosingShape :
                bilateralComplete "openings" s.case.openings ∧
                  bilateralComplete "arguments" s.case.arguments ∧
                  plaintiffOptionalSequence "rebuttals" s.case.rebuttals ∧
                  defendantOptionalSequence "surrebuttals" s.case.surrebuttals ∧
                  bilateralStarted "closings" s.case.closings := by
              simpa [phaseShape, hClosings] using hShape
            have hStarted : bilateralStarted "closings" s.case.closings := hClosingShape.2.2.2.2
            rcases plaintiffThenDefendant_some_of_bilateralStarted
                "closings" "plaintiff" "defendant" s.case.closings hStarted with
              ⟨role, hRole⟩
            refine ⟨?_, ?_⟩
            · simp [nextOpportunity, hStatus, nextOpportunityForPhase, hClosings, hRole]
            · refine ⟨{
                opportunity_id := s!"closings:{role}"
                role := role
                phase := "closings"
                objective := s!"{role} closing statement"
                allowed_tools := ["deliver_closing_statement"]
              }, ?_⟩
              simp [nextOpportunity, hStatus, nextOpportunityForPhase, hClosings, hRole]
          · by_cases hDeliberation : s.case.phase = "deliberation"
            · rcases reachable_deliberation_has_nextCouncilMember s hs hDeliberation hStatus with
                ⟨member, hMember⟩
              refine ⟨?_, ?_⟩
              · simp [nextOpportunity, hStatus, nextOpportunityForPhase, hDeliberation, hMember]
              · refine ⟨{
                  opportunity_id := s!"deliberation:{s.case.deliberation_round}:{member.member_id}"
                  role := "council"
                  phase := "deliberation"
                  objective := s!"council vote by {member.member_id}"
                  allowed_tools := ["submit_council_vote"]
                }, ?_⟩
                simp [nextOpportunity, hStatus, nextOpportunityForPhase, hDeliberation, hMember]
            · have hPhaseNotClosed : s.case.phase ≠ "closed" := by
                intro hClosed
                have hClosedStatus := reachable_phase_closed_implies_status_closed s hs hClosed
                rw [hClosedStatus] at hStatus
                simp at hStatus
              have hImpossible : False := by
                simp [phaseShape] at hShape
              exact False.elim hImpossible

end ArbProofs
