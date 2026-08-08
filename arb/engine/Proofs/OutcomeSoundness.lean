import Proofs.ViableOutcomesCore

namespace ArbProofs

/-
This file begins the outcome-soundness proof layer.

The earlier global invariants show that reachable states keep the procedure in
order, preserve side-to-side parity, respect cumulative material limits, and
keep the initialized case frame fixed.  Those theorems still leave the most
important public question open.

When the engine closes a case with `demonstrated`, `not_demonstrated`, or
`no_majority`, does that result follow from the recorded deliberation state?

The executable closing logic is concentrated in `continueDeliberation`.  That
is the right place to start.  This file proves that the three closing branches
of `continueDeliberation` are sound with respect to the recorded votes, the
configured threshold, and the round and seating conditions.

This is not yet the final reachable-state theorem recorded in
`docs/verification.md`.  It is the first layer that the global theorem will
need.  The result says that whenever deliberation closes a case, the closure is
justified by the deliberation record that the engine carries in state.
-/

def demonstratedOutcomeSound (s : ArbitrationState) : Prop :=
  voteCountFor (currentRoundVotes s.case) "demonstrated" ≥
    s.policy.required_votes_for_decision

def notDemonstratedOutcomeSound (s : ArbitrationState) : Prop :=
  voteCountFor (currentRoundVotes s.case) "not_demonstrated" ≥
    s.policy.required_votes_for_decision

def noMajorityOutcomeSound (s : ArbitrationState) : Prop :=
  voteCountFor (currentRoundVotes s.case) "demonstrated" <
      s.policy.required_votes_for_decision ∧
    voteCountFor (currentRoundVotes s.case) "not_demonstrated" <
      s.policy.required_votes_for_decision ∧
    (seatedCouncilMemberCount s.case < s.policy.required_votes_for_decision ∨
      ((currentRoundVotes s.case).length = seatedCouncilMemberCount s.case ∧
        s.case.deliberation_round ≥ s.policy.max_deliberation_rounds))

/--
`voteCountFor` never exceeds the length of the vote list it counts.
-/
theorem voteCountFor_fold_le_length
    (votes : List CouncilVote)
    (value : String)
    (acc : Nat) :
    votes.foldl (fun acc vote => if trimString vote.vote = value then acc + 1 else acc) acc ≤
      acc + votes.length := by
  induction votes generalizing acc with
  | nil =>
      simp
  | cons vote rest ih =>
      simp only [List.foldl_cons, List.length_cons]
      by_cases hMatch : trimString vote.vote = value
      · simp [hMatch]
        simpa [Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using ih (acc + 1)
      · simp [hMatch]
        exact Nat.le_trans (ih acc) (Nat.le_succ (acc + rest.length))

theorem voteCountFor_le_length
    (votes : List CouncilVote)
    (value : String) :
    voteCountFor votes value ≤ votes.length := by
  simpa [voteCountFor] using voteCountFor_fold_le_length votes value 0

/--
Successful initialization always opens the live case in the `openings` phase.

This generic fact is useful in the reachable-state wrapper below.  The
initialization branch cannot already be a closed outcome.
-/
theorem initializeCase_phase_openings
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.case.phase = "openings" := by
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
                  simp

/--
The summary-side `noMajorityClosure` predicate is sufficient for
`continueDeliberation` to close as `no_majority`.

This theorem packages the executable branch condition in the same compact
summary language used by the later closure soundness proof.
-/
theorem continueDeliberation_closed_no_majority_of_summary_noMajorityClosure
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hClosure :
      (deliberationSummaryForCase
        c
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds).noMajorityClosure) :
    continueDeliberation s c =
      .ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" }) := by
  rcases hClosure with ⟨hRoundComplete, hNoViable, hReason⟩
  have hRoundComplete' : (currentRoundVotes c).length = seatedCouncilMemberCount c := by
    simpa [deliberationSummaryForCase] using hRoundComplete
  have hSummaryNone :
      (deliberationSummaryForCase
        c
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds).currentResolution? = none := by
    exact
      (deliberationSummaryForCase
        c
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds).currentResolution_none_of_noSubstantiveOutcomeViable
        hNoViable
  have hCurrent :
      currentResolution? c s.policy.required_votes_for_decision = none := by
    exact (deliberationSummaryForCase_currentResolution
      c
      s.policy.required_votes_for_decision
      s.policy.max_deliberation_rounds).symm.trans hSummaryNone
  unfold continueDeliberation
  rcases hReason with hTooFew | ⟨hSummaryRoundComplete, hLastRound⟩
  · have hTooFew' : seatedCouncilMemberCount c < s.policy.required_votes_for_decision := by
      simpa [deliberationSummaryForCase] using hTooFew
    simp [hRoundComplete', hCurrent, hTooFew', stateWithCase]
    rfl
  · have hLastRound' : c.deliberation_round ≥ s.policy.max_deliberation_rounds := by
      simpa [deliberationSummaryForCase] using hLastRound
    have hRoundComplete'' : (currentRoundVotes c).length = seatedCouncilMemberCount c := by
      simpa [deliberationSummaryForCase] using hSummaryRoundComplete
    by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
    · simp [hRoundComplete'', hCurrent, hTooFew, stateWithCase]
      rfl
    · simp [hRoundComplete'', hCurrent, hTooFew, hLastRound', stateWithCase]
      rfl

/--
If `continueDeliberation` closes as `no_majority`, the source deliberation
state satisfied the summary-side closure predicate for that branch.
-/
theorem continueDeliberation_closed_no_majority_implies_summary_noMajorityClosure
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    (deliberationSummaryForCase
      c
      s.policy.required_votes_for_decision
      s.policy.max_deliberation_rounds).noMajorityClosure := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hCurrent : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hCurrent, stateWithCase] at hCont
        cases hCont
        unfold currentResolution? at hCurrent
        by_cases hDem : voteCountFor (currentRoundVotes c) "demonstrated" ≥ s.policy.required_votes_for_decision
        · simp [hDem] at hCurrent
          have hImpossible : False := by
            have hEq : "demonstrated" = "no_majority" := by
              simpa [hCurrent] using hResolution
            simp at hEq
          exact hImpossible.elim
        · by_cases hNot : voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ s.policy.required_votes_for_decision
          · simp [hDem, hNot] at hCurrent
            have hImpossible : False := by
              have hEq : "not_demonstrated" = "no_majority" := by
                simpa [hCurrent] using hResolution
              simp at hEq
            exact hImpossible.elim
          · simp [hDem, hNot] at hCurrent
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hCurrent, hTooFew, stateWithCase] at hCont
          cases hCont
          exact
            deliberationSummaryForCase_noMajorityClosure_of_currentResolution_none_of_round_complete
              c
              s.policy.required_votes_for_decision
              s.policy.max_deliberation_rounds
              hCurrent
              hRoundComplete
              (Or.inl hTooFew)
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact
              deliberationSummaryForCase_noMajorityClosure_of_currentResolution_none_of_round_complete
                c
                s.policy.required_votes_for_decision
                s.policy.max_deliberation_rounds
                hCurrent
                hRoundComplete
                (Or.inr hLastRound)
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp [hDeliberation] at hClosed
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simp [hDeliberation] at hClosed

/--
The summary-side `closedResolution?` value determines the exact closed
`continueDeliberation` result.

This theorem packages the three executable closing branches behind one summary
predicate: threshold closure for either substantive outcome, or `no_majority`
closure when the round is complete and the executable closure reason holds.
-/
theorem continueDeliberation_closed_of_summary_closedResolution
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (resolution : String)
    (hClosedResolution :
      (deliberationSummaryForCase
        c
        s.policy.required_votes_for_decision
        s.policy.max_deliberation_rounds).closedResolution? = some resolution) :
    continueDeliberation s c =
      .ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := resolution }) := by
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · have hSummaryRoundComplete :
        (deliberationSummaryForCase
          c
          s.policy.required_votes_for_decision
          s.policy.max_deliberation_rounds).current_round_vote_count =
          (deliberationSummaryForCase
            c
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).seated_count := by
      simpa [deliberationSummaryForCase] using hRoundComplete
    cases hCurrent : currentResolution? c s.policy.required_votes_for_decision with
    | some chosen =>
        have hChosen : chosen = resolution := by
          simpa [DeliberationSummary.closedResolution?, hSummaryRoundComplete, hCurrent]
            using hClosedResolution
        subst chosen
        unfold continueDeliberation
        simp [hRoundComplete, hCurrent, stateWithCase]
        rfl
    | none =>
        have hReason :
            (deliberationSummaryForCase
              c
              s.policy.required_votes_for_decision
              s.policy.max_deliberation_rounds).noMajorityClosureReason := by
          by_cases hReason :
              (deliberationSummaryForCase
                c
                s.policy.required_votes_for_decision
                s.policy.max_deliberation_rounds).noMajorityClosureReason
          · exact hReason
          · simp [DeliberationSummary.closedResolution?, hSummaryRoundComplete, hCurrent, hReason]
              at hClosedResolution
        have hNoViable :
            (deliberationSummaryForCase
              c
              s.policy.required_votes_for_decision
              s.policy.max_deliberation_rounds).noSubstantiveOutcomeViable := by
          exact
            deliberationSummaryForCase_noSubstantiveOutcomeViable_of_currentResolution_none_of_round_complete
              c
              s.policy.required_votes_for_decision
              s.policy.max_deliberation_rounds
              hCurrent
              hRoundComplete
        have hClosure :
            (deliberationSummaryForCase
              c
              s.policy.required_votes_for_decision
              s.policy.max_deliberation_rounds).noMajorityClosure := by
          exact ⟨hSummaryRoundComplete, hNoViable, hReason⟩
        have hNoMajority :
            continueDeliberation s c =
              .ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" }) := by
          exact continueDeliberation_closed_no_majority_of_summary_noMajorityClosure s c hClosure
        have hChosen : resolution = "no_majority" := by
          have hEq : "no_majority" = resolution := by
            simpa [DeliberationSummary.closedResolution?, hSummaryRoundComplete, hCurrent, hReason]
              using hClosedResolution
          simpa using hEq.symm
        subst resolution
        exact hNoMajority
  · simp [DeliberationSummary.closedResolution?, deliberationSummaryForCase, hRoundComplete]
      at hClosedResolution

/--
A closed `continueDeliberation` result records exactly the summary-side
`closedResolution?` value of the source deliberation state.
-/
theorem continueDeliberation_closed_implies_summary_closedResolution
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed") :
    (deliberationSummaryForCase
      c
      s.policy.required_votes_for_decision
      s.policy.max_deliberation_rounds).closedResolution? = some t.case.resolution := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · have hSummaryRoundComplete :
        (deliberationSummaryForCase
          c
          s.policy.required_votes_for_decision
          s.policy.max_deliberation_rounds).current_round_vote_count =
          (deliberationSummaryForCase
            c
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).seated_count := by
      simpa [deliberationSummaryForCase] using hRoundComplete
    cases hCurrent : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hCurrent, stateWithCase] at hCont
        cases hCont
        exact
          (deliberationSummaryForCase
            c
            s.policy.required_votes_for_decision
            s.policy.max_deliberation_rounds).closedResolution_eq_of_currentResolution
              resolution
              hSummaryRoundComplete
              (by simpa using hCurrent)
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hCurrent, hTooFew, stateWithCase] at hCont
          cases hCont
          exact
            (deliberationSummaryForCase
              c
              s.policy.required_votes_for_decision
              s.policy.max_deliberation_rounds).closedResolution_eq_no_majority_of_noMajorityClosure
                (deliberationSummaryForCase_noMajorityClosure_of_currentResolution_none_of_round_complete
                  c
                  s.policy.required_votes_for_decision
                  s.policy.max_deliberation_rounds
                  hCurrent
                  hRoundComplete
                  (Or.inl hTooFew))
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact
              (deliberationSummaryForCase
                c
                s.policy.required_votes_for_decision
                s.policy.max_deliberation_rounds).closedResolution_eq_no_majority_of_noMajorityClosure
                  (deliberationSummaryForCase_noMajorityClosure_of_currentResolution_none_of_round_complete
                    c
                    s.policy.required_votes_for_decision
                    s.policy.max_deliberation_rounds
                    hCurrent
                    hRoundComplete
                    (Or.inr hLastRound))
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            simp [hDeliberation] at hClosed
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    simp [hDeliberation] at hClosed

/--
If deliberation closes a case as `demonstrated`, the closed state records
enough `demonstrated` votes for the configured threshold.

The theorem assumes that deliberation started from a deliberation-phase case.
That is exactly the condition under which the public council actions call
`continueDeliberation`.
-/
theorem continueDeliberation_closed_demonstrated_sound
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  let d := deliberationSummaryForCase
    c
    s.policy.required_votes_for_decision
    s.policy.max_deliberation_rounds
  have hSummaryClosed : d.closedResolution? = some "demonstrated" := by
    simpa [d, hResolution] using
      continueDeliberation_closed_implies_summary_closedResolution s t c
        hDeliberation hCont hClosed
  have hSummaryCurrent : d.currentResolution? = some "demonstrated" := by
    exact d.currentResolution_eq_of_closedResolution_ne_no_majority
      "demonstrated"
      hSummaryClosed
      (by simp)
  have hSound :
      voteCountFor (currentRoundVotes c) "demonstrated" ≥
        s.policy.required_votes_for_decision := by
    exact currentResolution_demonstrated_implies_sound c
      s.policy.required_votes_for_decision
      (by simpa [d] using hSummaryCurrent)
  have hExact :
      continueDeliberation s c =
        .ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "demonstrated" }) := by
    exact continueDeliberation_closed_of_summary_closedResolution s c "demonstrated"
      (by simpa [d] using hSummaryClosed)
  have hState :
      t = stateWithCase s { c with status := "closed", phase := "closed", resolution := "demonstrated" } := by
    have hOk :
        Except.ok t =
          Except.ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "demonstrated" }) :=
      hCont.symm.trans hExact
    simpa using hOk
  subst t
  unfold demonstratedOutcomeSound
  change voteCountFor
    (currentRoundVotes { c with status := "closed", phase := "closed", resolution := "demonstrated" })
      "demonstrated" ≥ s.policy.required_votes_for_decision
  have hVotes :
      currentRoundVotes { c with status := "closed", phase := "closed", resolution := "demonstrated" } =
        currentRoundVotes c := by
    rfl
  rw [hVotes]
  exact hSound

/--
If deliberation closes a case as `not_demonstrated`, the closed state records
enough `not_demonstrated` votes for the configured threshold.
-/
theorem continueDeliberation_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  let d := deliberationSummaryForCase
    c
    s.policy.required_votes_for_decision
    s.policy.max_deliberation_rounds
  have hSummaryClosed : d.closedResolution? = some "not_demonstrated" := by
    simpa [d, hResolution] using
      continueDeliberation_closed_implies_summary_closedResolution s t c
        hDeliberation hCont hClosed
  have hSummaryCurrent : d.currentResolution? = some "not_demonstrated" := by
    exact d.currentResolution_eq_of_closedResolution_ne_no_majority
      "not_demonstrated"
      hSummaryClosed
      (by simp)
  have hSound :
      voteCountFor (currentRoundVotes c) "not_demonstrated" ≥
        s.policy.required_votes_for_decision := by
    exact currentResolution_not_demonstrated_implies_sound c
      s.policy.required_votes_for_decision
      (by simpa [d] using hSummaryCurrent)
  have hExact :
      continueDeliberation s c =
        .ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "not_demonstrated" }) := by
    exact continueDeliberation_closed_of_summary_closedResolution s c "not_demonstrated"
      (by simpa [d] using hSummaryClosed)
  have hState :
      t = stateWithCase s { c with status := "closed", phase := "closed", resolution := "not_demonstrated" } := by
    have hOk :
        Except.ok t =
          Except.ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "not_demonstrated" }) :=
      hCont.symm.trans hExact
    simpa using hOk
  subst t
  unfold notDemonstratedOutcomeSound
  change voteCountFor
    (currentRoundVotes { c with status := "closed", phase := "closed", resolution := "not_demonstrated" })
      "not_demonstrated" ≥ s.policy.required_votes_for_decision
  have hVotes :
      currentRoundVotes { c with status := "closed", phase := "closed", resolution := "not_demonstrated" } =
        currentRoundVotes c := by
    rfl
  rw [hVotes]
  exact hSound

/--
If deliberation closes a case as `no_majority`, neither side reached the vote
threshold, and closure happened for one of the two executable reasons:

1. too few seated members remained to make the threshold possible; or
2. the round was complete, the final round had been reached, and no side had
   yet reached the threshold.
-/
theorem continueDeliberation_closed_no_majority_sound
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  let d := deliberationSummaryForCase
    c
    s.policy.required_votes_for_decision
    s.policy.max_deliberation_rounds
  have hSummaryClosed : d.closedResolution? = some "no_majority" := by
    simpa [d, hResolution] using
      continueDeliberation_closed_implies_summary_closedResolution s t c
        hDeliberation hCont hClosed
  have hSummaryClosure : d.noMajorityClosure :=
    d.noMajorityClosure_of_closedResolution_no_majority hSummaryClosed
  have hSummarySound : d.noMajoritySound :=
    d.noMajoritySound_of_noMajorityClosure hSummaryClosure
  have hExact :
      continueDeliberation s c =
        .ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" }) := by
    exact continueDeliberation_closed_of_summary_closedResolution s c "no_majority"
      (by simpa [d] using hSummaryClosed)
  have hState :
      t = stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" } := by
    have hOk :
        Except.ok t =
          Except.ok (stateWithCase s { c with status := "closed", phase := "closed", resolution := "no_majority" }) :=
      hCont.symm.trans hExact
    simpa using hOk
  subst t
  unfold noMajorityOutcomeSound
  let closedCase := { c with status := "closed", phase := "closed", resolution := "no_majority" }
  change voteCountFor (currentRoundVotes closedCase) "demonstrated" <
      s.policy.required_votes_for_decision ∧
    voteCountFor (currentRoundVotes closedCase) "not_demonstrated" <
        s.policy.required_votes_for_decision ∧
      (seatedCouncilMemberCount closedCase < s.policy.required_votes_for_decision ∨
        (currentRoundVotes closedCase).length = seatedCouncilMemberCount closedCase ∧
          closedCase.deliberation_round ≥ s.policy.max_deliberation_rounds)
  have hVotes : currentRoundVotes closedCase = currentRoundVotes c := by
    rfl
  have hSeated : seatedCouncilMemberCount closedCase = seatedCouncilMemberCount c := by
    rfl
  rw [hVotes, hSeated]
  simpa [DeliberationSummary.noMajoritySound,
    DeliberationSummary.noMajorityClosureReason, d, deliberationSummaryForCase] using hSummarySound

/--
An opening-statement step never closes the case.

Openings can keep the case in `openings` or advance it to `arguments`.  They
do not jump directly to `closed`.
-/
theorem step_record_opening_statement_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
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
An argument step never closes the case.

Arguments can keep the case in `arguments` or advance it to `rebuttals`.
-/
theorem step_submit_argument_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
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
A rebuttal step never closes the case.

Rebuttals can keep the case in `rebuttals` or advance it to `surrebuttals`.
-/
theorem step_submit_rebuttal_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
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
A surrebuttal step never closes the case.

Surrebuttal can keep the case in `surrebuttals` or advance it to `closings`.
-/
theorem step_submit_surrebuttal_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
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
A closing-statement step never closes the case.

Closings can keep the case in `closings` or advance it to `deliberation`.
-/
theorem step_deliver_closing_statement_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  have hPhase : s.case.phase = "closings" := by
    by_cases hClosing : s.case.phase = "closings"
    · exact hClosing
    · have hClosed : s.case.phase != "closings" := by simpa using hClosing
      simp [stepCore, hType, hClosed] at hStep
      cases hStep
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  by_cases hAdvance : 1 ≤ s.case.closings.length
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]

/--
Passing an optional phase never closes the case.

Passing rebuttal advances to `surrebuttals`.  Passing surrebuttal advances to
`closings`.
-/
theorem step_pass_phase_opportunity_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
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

/--
Evidence submission never closes the case.  It is accepted only during the
arguments phase or the open plaintiff rebuttal evidence window, and it preserves
the current phase.
-/
theorem step_submit_evidence_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  by_cases hArgs : s.case.phase = "arguments"
  · rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
    simp [stateWithCase, appendSubmittedEvidence, hArgs]
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · cases hEmpty : s.case.rebuttals.isEmpty with
      | true =>
          rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
          simp [stateWithCase, appendSubmittedEvidence, hRebuttals]
      | false =>
          simp [submitEvidence, hRebuttals, hEmpty, Bind.bind, Except.bind] at hSubmit
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · cases hEmpty : s.case.surrebuttals.isEmpty with
        | true =>
            rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
            simp [stateWithCase, appendSubmittedEvidence, hSurrebuttals]
        | false =>
            simp [submitEvidence, hSurrebuttals, hEmpty,
              Bind.bind, Except.bind] at hSubmit
      · simp [submitEvidence, Bind.bind, Except.bind] at hSubmit

theorem step_fail_opportunity_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    failOpportunity s action.payload = .ok t := by
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
      exact hStep'

theorem step_fail_opportunity_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  have hFail := step_fail_opportunity_result s t action hType hStep
  rcases failOpportunity_result s t action.payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
      hDeliberation, _hSeated, _hFresh, hCont⟩
    have hDeliberation1 : c1.phase = "deliberation" := by
      rw [hC1]
      simpa using hDeliberation
    exact continueDeliberation_closed_demonstrated_sound s t c1
      hDeliberation1 hCont hClosed hResolution
  · rcases hParty with ⟨_failure, _hEq, hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact False.elim (hNotClosed hClosed)

theorem step_fail_opportunity_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  have hFail := step_fail_opportunity_result s t action hType hStep
  rcases failOpportunity_result s t action.payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
      hDeliberation, _hSeated, _hFresh, hCont⟩
    have hDeliberation1 : c1.phase = "deliberation" := by
      rw [hC1]
      simpa using hDeliberation
    exact continueDeliberation_closed_not_demonstrated_sound s t c1
      hDeliberation1 hCont hClosed hResolution
  · rcases hParty with ⟨_failure, _hEq, hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact False.elim (hNotClosed hClosed)

theorem step_fail_opportunity_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  have hFail := step_fail_opportunity_result s t action hType hStep
  rcases failOpportunity_result s t action.payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
      hDeliberation, _hSeated, _hFresh, hCont⟩
    have hDeliberation1 : c1.phase = "deliberation" := by
      rw [hC1]
      simpa using hDeliberation
    exact continueDeliberation_closed_no_majority_sound s t c1
      hDeliberation1 hCont hClosed hResolution
  · rcases hParty with ⟨_failure, _hEq, hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact False.elim (hNotClosed hClosed)

/--
Any successful public step that closes as `demonstrated` is sound.

The non-deliberation actions cannot close the case.  The only closing public
steps are the council vote and council member removal actions, and those now
delegate to the direct soundness theorems above.
-/
theorem step_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim ((step_record_opening_statement_phase_ne_closed s t action hOpening hStepCore) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim ((step_submit_argument_phase_ne_closed s t action hArgument hStepCore) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStepCore) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStepCore) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStepCore) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStepCore) hClosed)
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact False.elim ((step_submit_evidence_phase_ne_closed s t action hEvidence hStepCore) hClosed)
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                  ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  have hDeliberation1 : c1.phase = "deliberation" := by
                    simpa [c1] using hDeliberation
                  exact continueDeliberation_closed_demonstrated_sound s t c1
                    hDeliberation1 hCont hClosed hResolution
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
                    have hDeliberation1 : c1.phase = "deliberation" := by
                      simpa [c1] using hDeliberation
                    exact continueDeliberation_closed_demonstrated_sound s t c1
                      hDeliberation1 hCont hClosed hResolution
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_closed_demonstrated_sound s t action
                        hFail hStepCore hClosed hResolution
                    · simp [stepCore] at hStepCore

theorem step_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim ((step_record_opening_statement_phase_ne_closed s t action hOpening hStepCore) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim ((step_submit_argument_phase_ne_closed s t action hArgument hStepCore) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStepCore) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStepCore) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStepCore) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStepCore) hClosed)
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact False.elim ((step_submit_evidence_phase_ne_closed s t action hEvidence hStepCore) hClosed)
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                  ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  have hDeliberation1 : c1.phase = "deliberation" := by
                    simpa [c1] using hDeliberation
                  exact continueDeliberation_closed_not_demonstrated_sound s t c1
                    hDeliberation1 hCont hClosed hResolution
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
                    have hDeliberation1 : c1.phase = "deliberation" := by
                      simpa [c1] using hDeliberation
                    exact continueDeliberation_closed_not_demonstrated_sound s t c1
                      hDeliberation1 hCont hClosed hResolution
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_closed_not_demonstrated_sound s t action
                        hFail hStepCore hClosed hResolution
                    · simp [stepCore] at hStepCore

theorem step_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim ((step_record_opening_statement_phase_ne_closed s t action hOpening hStepCore) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim ((step_submit_argument_phase_ne_closed s t action hArgument hStepCore) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStepCore) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStepCore) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStepCore) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStepCore) hClosed)
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact False.elim ((step_submit_evidence_phase_ne_closed s t action hEvidence hStepCore) hClosed)
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                  ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                  let c1 := { s.case with council_votes := s.case.council_votes.concat {
                    member_id := memberId
                    round := s.case.deliberation_round
                    vote := trimString vote
                    rationale := trimString rationale
                  } }
                  have hDeliberation1 : c1.phase = "deliberation" := by
                    simpa [c1] using hDeliberation
                  exact continueDeliberation_closed_no_majority_sound s t c1
                    hDeliberation1 hCont hClosed hResolution
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
                    have hDeliberation1 : c1.phase = "deliberation" := by
                      simpa [c1] using hDeliberation
                    exact continueDeliberation_closed_no_majority_sound s t c1
                      hDeliberation1 hCont hClosed hResolution
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_closed_no_majority_sound s t action
                        hFail hStepCore hClosed hResolution
                    · simp [stepCore] at hStepCore

/--
Every reachable closed `demonstrated` result is sound.

The base case is impossible because initialization always opens the case.  The
step case reduces immediately to the public step theorem above.
-/
theorem reachable_closed_demonstrated_sound
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed")
    (hResolution : s.case.resolution = "demonstrated") :
    demonstratedOutcomeSound s := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step s t action hs hStep _ =>
      exact step_closed_demonstrated_sound s t action hStep hClosed hResolution

theorem reachable_closed_not_demonstrated_sound
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed")
    (hResolution : s.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound s := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step s t action hs hStep _ =>
      exact step_closed_not_demonstrated_sound s t action hStep hClosed hResolution

theorem reachable_closed_no_majority_sound
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed")
    (hResolution : s.case.resolution = "no_majority") :
    noMajorityOutcomeSound s := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step s t action hs hStep _ =>
      exact step_closed_no_majority_sound s t action hStep hClosed hResolution

/--
A successful council-vote step that closes as `demonstrated` has a sound
outcome.

This theorem lifts the previous result from the raw deliberation helper to the
public `step` boundary for council votes.
-/
theorem step_submit_council_vote_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_submit_council_vote_result s t action hType hStepCore with
    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_submit_council_vote_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_submit_council_vote_result s t action hType hStepCore with
    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_not_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_submit_council_vote_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_submit_council_vote_result s t action hType hStepCore with
    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_no_majority_sound s t c1
    hDeliberation1 hCont hClosed hResolution

/--
A successful council-removal step that closes the case has a sound outcome.

The removal step can close as `no_majority` immediately when the threshold
becomes impossible.  The theorem is stated in the same shape as the vote-step
wrappers so later global theorems can work uniformly over the public step
boundary.
-/
theorem step_remove_council_member_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_remove_council_member_result s t action hType hStepCore with
    ⟨memberId, status, hDeliberation, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_remove_council_member_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_remove_council_member_result s t action hType hStepCore with
    ⟨memberId, status, hDeliberation, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_not_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_remove_council_member_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  rcases step_remove_council_member_result s t action hType hStepCore with
    ⟨memberId, status, hDeliberation, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_no_majority_sound s t c1
    hDeliberation1 hCont hClosed hResolution

end ArbProofs
