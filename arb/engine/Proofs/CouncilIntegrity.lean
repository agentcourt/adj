import Proofs.CaseFrame

namespace ArbProofs

open List

def councilIdsUnique (c : ArbitrationCase) : Prop :=
  (councilMemberIds c.council_members).Nodup

def currentRoundVoteIds (c : ArbitrationCase) : List String :=
  (currentRoundVotes c).map (·.member_id)

def seatedCouncilMemberIds (c : ArbitrationCase) : List String :=
  councilMemberIds (seatedCouncilMembers c)

def currentRoundVoteIdsDistinct (c : ArbitrationCase) : Prop :=
  (currentRoundVoteIds c).Nodup

def currentRoundVotesFromSeatedMembers (c : ArbitrationCase) : Prop :=
  ∀ vote ∈ currentRoundVotes c, vote.member_id ∈ seatedCouncilMemberIds c

def councilVoteRoundsBounded (c : ArbitrationCase) : Prop :=
  ∀ vote ∈ c.council_votes, vote.round ≤ c.deliberation_round

def councilVoteIntegrity (c : ArbitrationCase) : Prop :=
  currentRoundVoteIdsDistinct c ∧
    currentRoundVotesFromSeatedMembers c ∧
    councilVoteRoundsBounded c

/-
This file establishes the basic integrity of the deliberation record.

Stage 3 of the verification plan asks for a liveness theorem: every reachable
non-closed state has a next opportunity.  The deliberation branch of that
theorem needs one concrete fact that the earlier files did not state.

If a deliberation round is still live, the stored current-round votes should
describe distinct council members who are still seated in the case.  Otherwise
the arithmetic test in `continueDeliberation` would not line up with the
search performed by `nextCouncilMember?`.

The first half of the file proves those integrity facts for every reachable
state.  The second half derives the one consequence that Stage 3 needs:
whenever a live deliberation state has not yet completed the round, the engine
can still find a seated council member who has not voted in that round.
-/

theorem hasDuplicateStrings_eq_true_of_not_nodup :
    ∀ {xs : List String}, ¬ xs.Nodup → hasDuplicateStrings xs = true
  | [] => by
      intro hNot
      exact False.elim (hNot List.nodup_nil)
  | x :: xs => by
      intro hNot
      by_cases hMem : x ∈ xs
      · have hAnyTrue : xs.any (fun y => y = x) = true := by
          exact List.any_eq_true.mpr ⟨x, hMem, by simp⟩
        simp [hasDuplicateStrings, hAnyTrue]
      · have hTailNotNodup : ¬ xs.Nodup := by
          intro hTail
          exact hNot (List.nodup_cons.mpr ⟨hMem, hTail⟩)
        have hTailDup : hasDuplicateStrings xs = true :=
          hasDuplicateStrings_eq_true_of_not_nodup hTailNotNodup
        simp [hasDuplicateStrings, hTailDup]

theorem hasDuplicateStrings_eq_false_of_nodup
    {xs : List String}
    (hNodup : xs.Nodup) :
    hasDuplicateStrings xs = false := by
  induction xs with
  | nil =>
      rfl
  | cons x xs ih =>
      have hInfo := List.nodup_cons.mp hNodup
      have hNotMem : x ∉ xs := hInfo.1
      have hTail : xs.Nodup := hInfo.2
      have hAnyFalse : xs.any (fun y => y = x) = false := by
        by_cases hAny : xs.any (fun y => y = x) = true
        · rcases List.any_eq_true.mp hAny with ⟨y, hy, hEq⟩
          have hEq' : y = x := by
            exact of_decide_eq_true hEq
          exact False.elim (hNotMem (hEq' ▸ hy))
        · cases hBool : xs.any (fun y => y = x) with
          | false =>
              rfl
          | true =>
              simp [hBool] at hAny
      simp [hasDuplicateStrings, hAnyFalse, ih hTail]

theorem seatedCouncilMemberIds_sublist_councilMemberIds
    (c : ArbitrationCase) :
    seatedCouncilMemberIds c <+ councilMemberIds c.council_members := by
  simpa [seatedCouncilMemberIds, seatedCouncilMembers, councilMemberIds] using
    (show (List.filter memberIsSeated c.council_members).map (·.member_id) <+
        c.council_members.map (·.member_id) from
      (List.filter_sublist (p := memberIsSeated) (l := c.council_members)).map (·.member_id))

theorem seatedCouncilMemberIds_nodup
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c) :
    (seatedCouncilMemberIds c).Nodup := by
  unfold councilIdsUnique at hUnique
  exact List.Nodup.sublist
    (l₁ := seatedCouncilMemberIds c)
    (l₂ := councilMemberIds c.council_members)
    (seatedCouncilMemberIds_sublist_councilMemberIds c)
    hUnique

theorem initializeCase_establishes_councilIdsUnique
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    councilIdsUnique s.case := by
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
                · have hNoDupIds :
                    (req.council_members.map (·.member_id)).Nodup := by
                    unfold hasDuplicateCouncilMemberIds at hDuplicate
                    exact by
                      by_cases hNodup : (req.council_members.map (·.member_id)).Nodup
                      · exact hNodup
                      · have hDupTrue :
                            hasDuplicateStrings (req.council_members.map (·.member_id)) = true :=
                            hasDuplicateStrings_eq_true_of_not_nodup hNodup
                        simp [hDupTrue] at hDuplicate
                  simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate, stateWithCase] at hInit
                  cases hInit
                  simpa [councilIdsUnique, councilMemberIds, Function.comp_def] using hNoDupIds

theorem currentRoundVoteIntegrity_empty (c : ArbitrationCase) :
    councilVoteIntegrity { c with council_votes := [] } := by
  simp [councilVoteIntegrity, currentRoundVoteIdsDistinct,
    currentRoundVotesFromSeatedMembers, councilVoteRoundsBounded,
    currentRoundVoteIds, currentRoundVotes]

/--
The deliberation-record invariant depends only on council votes, council
members, and the active deliberation round.

This congruence lemma lets the later proofs ignore unrelated fields such as the
merits filings, the current phase marker, or the closing resolution text.
-/
theorem councilVoteIntegrity_congr
    {c d : ArbitrationCase}
    (hVotes : d.council_votes = c.council_votes)
    (hMembers : d.council_members = c.council_members)
    (hRound : d.deliberation_round = c.deliberation_round)
    (hIntegrity : councilVoteIntegrity c) :
    councilVoteIntegrity d := by
  unfold councilVoteIntegrity currentRoundVoteIdsDistinct
    currentRoundVotesFromSeatedMembers councilVoteRoundsBounded
    currentRoundVoteIds currentRoundVotes seatedCouncilMemberIds
    seatedCouncilMembers councilMemberIds at *
  simpa [hVotes, hMembers, hRound] using hIntegrity

theorem currentRoundVotes_congr
    {c d : ArbitrationCase}
    (hVotes : d.council_votes = c.council_votes)
    (hRound : d.deliberation_round = c.deliberation_round) :
    currentRoundVotes d = currentRoundVotes c := by
  unfold currentRoundVotes
  rw [hVotes, hRound]

theorem seatedCouncilMemberCount_congr
    {c d : ArbitrationCase}
    (hMembers : d.council_members = c.council_members) :
    seatedCouncilMemberCount d = seatedCouncilMemberCount c := by
  unfold seatedCouncilMemberCount seatedCouncilMembers
  rw [hMembers]

theorem setPhase_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (phase : String)
    (hIntegrity : councilVoteIntegrity c) :
    councilVoteIntegrity { c with phase := phase } := by
  exact councilVoteIntegrity_congr rfl rfl rfl hIntegrity

/--
Advancing from one merits phase to the next does not disturb deliberation data.

`advanceAfterMerits` changes only the phase marker.  The vote list, council
membership, and deliberation round all remain fixed.
-/
theorem advanceAfterMerits_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (hIntegrity : councilVoteIntegrity c) :
    councilVoteIntegrity (advanceAfterMerits c) := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 && c.phase = "openings"
  · simpa [hOpen] using setPhase_preserves_councilVoteIntegrity c "arguments" hIntegrity
  · by_cases hArg : c.arguments.length >= 2 && c.phase = "arguments"
    · simpa [hOpen, hArg] using setPhase_preserves_councilVoteIntegrity c "rebuttals" hIntegrity
    · by_cases hRebuttal : c.rebuttals.length >= 1 && c.phase = "rebuttals"
      · simpa [hOpen, hArg, hRebuttal] using
          setPhase_preserves_councilVoteIntegrity c "surrebuttals" hIntegrity
      · by_cases hSurrebuttal : c.surrebuttals.length >= 1 && c.phase = "surrebuttals"
        · simpa [hOpen, hArg, hRebuttal, hSurrebuttal] using
            setPhase_preserves_councilVoteIntegrity c "closings" hIntegrity
        · by_cases hClosing : c.closings.length >= 2 && c.phase = "closings"
          · simpa [hOpen, hArg, hRebuttal, hSurrebuttal, hClosing] using
              setPhase_preserves_councilVoteIntegrity c "deliberation" hIntegrity
          · simpa [hOpen, hArg, hRebuttal, hSurrebuttal, hClosing] using hIntegrity

/--
Adding a merits filing preserves deliberation-record integrity.

`addFiling` changes only the filing lists, then possibly advances the phase.
Neither operation rewrites votes, council members, or the active round.
-/
theorem addFiling_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (phase role text : String)
    (hIntegrity : councilVoteIntegrity c) :
    councilVoteIntegrity (addFiling c phase role text) := by
  unfold addFiling
  split <;> exact advanceAfterMerits_preserves_councilVoteIntegrity _ hIntegrity

/--
Appending exhibits or technical reports preserves deliberation-record integrity.

Supplemental materials belong to the evidentiary record.  They do not affect
the council roster, the stored votes, or the round counter.
-/
theorem appendSupplementalMaterials_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hIntegrity : councilVoteIntegrity c) :
    councilVoteIntegrity (appendSupplementalMaterials c offered reports) := by
  apply councilVoteIntegrity_congr (c := c)
  · rfl
  · rfl
  · rfl
  · exact hIntegrity

/--
Appending a fresh current-round vote preserves deliberation-record integrity.

The public vote action proves two facts before this theorem is used: the
voting member is seated, and that member does not already appear in the current
round's vote list.  Those are exactly the facts needed here.
-/
theorem appendCurrentRoundVote_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (memberId vote rationale : String)
    (hIntegrity : councilVoteIntegrity c)
    (hSeated : memberId ∈ seatedCouncilMemberIds c)
    (hFresh : memberId ∉ currentRoundVoteIds c) :
    councilVoteIntegrity
      { c with council_votes := c.council_votes.concat {
          member_id := memberId
          round := c.deliberation_round
          vote := trimString vote
          rationale := trimString rationale
        } } := by
  let newVote : CouncilVote := {
    member_id := memberId
    round := c.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  }
  have hDistinct :
      (currentRoundVoteIds c ++ [memberId]).Nodup := by
    rw [List.nodup_append]
    refine ⟨hIntegrity.1, by simp, ?_⟩
    intro a hMemA b hMemB
    simp at hMemB
    rcases hMemB with rfl
    intro hEq
    exact hFresh (hEq ▸ hMemA)
  have hFromSeated :
      currentRoundVotesFromSeatedMembers
        { c with council_votes := c.council_votes.concat newVote } := by
    intro currentVote hVote
    have hVote' :
        currentVote ∈ currentRoundVotes c ++ [newVote] := by
      simpa [currentRoundVotes, newVote, List.concat_eq_append] using hVote
    rcases List.mem_append.mp hVote' with hOld | hNew
    · simpa [seatedCouncilMemberIds, seatedCouncilMembers, councilMemberIds] using
        hIntegrity.2.1 currentVote hOld
    · simp [newVote, seatedCouncilMemberIds] at hNew ⊢
      rcases hNew with rfl
      exact hSeated
  have hBounded :
      councilVoteRoundsBounded
        { c with council_votes := c.council_votes.concat newVote } := by
    intro storedVote hVote
    have hVote' : storedVote ∈ c.council_votes ++ [newVote] := by
      simpa [newVote, List.concat_eq_append] using hVote
    rcases List.mem_append.mp hVote' with hOld | hNew
    · exact hIntegrity.2.2 storedVote hOld
    · simp [newVote] at hNew
      rcases hNew with rfl
      exact Nat.le_refl c.deliberation_round
  refine ⟨?_, hFromSeated, hBounded⟩
  simpa [currentRoundVoteIdsDistinct, currentRoundVoteIds, currentRoundVotes,
    newVote, List.concat_eq_append] using hDistinct

/--
Removing an unvoted seated member preserves deliberation-record integrity.

The repair to `removeCouncilMember` forbids removal after a current-round vote.
That restriction is what makes the preservation theorem true: the removed
member cannot already appear in the current round's stored vote list.
-/
theorem removeUnvotedCouncilMember_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (memberId status : String)
    (hIntegrity : councilVoteIntegrity c)
    (hFresh : memberId ∉ currentRoundVoteIds c) :
    councilVoteIntegrity
      { c with council_members := c.council_members.map (fun member =>
          if member.member_id = memberId then
            { member with status := trimString status }
          else
            member) } := by
  let members :=
    c.council_members.map (fun member =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  have hFromSeated :
      currentRoundVotesFromSeatedMembers { c with council_members := members } := by
    intro currentVote hVote
    have hOld : currentVote ∈ currentRoundVotes c := by
      simpa [currentRoundVotes, members] using hVote
    have hOldSeat : currentVote.member_id ∈ seatedCouncilMemberIds c :=
      hIntegrity.2.1 currentVote hOld
    have hNotRemoved : currentVote.member_id ≠ memberId := by
      intro hEq
      apply hFresh
      have hMem : currentVote.member_id ∈ currentRoundVoteIds c := by
        simpa [currentRoundVoteIds] using
          (show ∃ vote, vote ∈ currentRoundVotes c ∧ vote.member_id = currentVote.member_id from
            ⟨currentVote, hOld, rfl⟩)
      simpa [hEq] using hMem
    rcases (show ∃ member, member ∈ seatedCouncilMembers c ∧ member.member_id = currentVote.member_id from
      by simpa [seatedCouncilMemberIds, councilMemberIds] using hOldSeat) with
      ⟨member, hMemberSeat, hMemberId⟩
    have hMemberMem : member ∈ c.council_members := by
      exact (List.mem_filter.mp hMemberSeat).1
    have hMemberStillSeated : memberIsSeated member := by
      exact (List.mem_filter.mp hMemberSeat).2
    have hMemberNe : member.member_id ≠ memberId := by
      simpa [hMemberId] using hNotRemoved
    have hMemberMapped : member ∈ members := by
      apply List.mem_map.mpr
      exact ⟨member, hMemberMem, by simp [hMemberNe]⟩
    have hSeatNew : member ∈ seatedCouncilMembers { c with council_members := members } := by
      unfold seatedCouncilMembers
      exact List.mem_filter.mpr ⟨hMemberMapped, by simpa using hMemberStillSeated⟩
    simpa [seatedCouncilMemberIds, councilMemberIds] using
      (show ∃ candidate,
          candidate ∈ seatedCouncilMembers { c with council_members := members } ∧
            candidate.member_id = currentVote.member_id from
        ⟨member, hSeatNew, hMemberId⟩)
  refine ⟨?_, hFromSeated, ?_⟩
  · simpa [currentRoundVoteIdsDistinct, currentRoundVoteIds, currentRoundVotes, members] using hIntegrity.1
  · simpa [councilVoteRoundsBounded, members] using hIntegrity.2.2

theorem failUnvotedCouncilMember_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (memberId reason opportunityId message : String)
    (hIntegrity : councilVoteIntegrity c)
    (hFresh : memberId ∉ currentRoundVoteIds c) :
    councilVoteIntegrity
      { c with council_members := List.map (fun (member : CouncilMember) =>
          if member.member_id = memberId then
            { member with
              status := "failed"
              failure_reason := reason
              failure_opportunity_id := opportunityId
              failure_message := message }
          else
            member) c.council_members } := by
  let members :=
    List.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with
          status := "failed"
          failure_reason := reason
          failure_opportunity_id := opportunityId
          failure_message := message }
      else
        member) c.council_members
  have hFromSeated :
      currentRoundVotesFromSeatedMembers { c with council_members := members } := by
    intro currentVote hVote
    have hOld : currentVote ∈ currentRoundVotes c := by
      simpa [currentRoundVotes, members] using hVote
    have hOldSeat : currentVote.member_id ∈ seatedCouncilMemberIds c :=
      hIntegrity.2.1 currentVote hOld
    have hNotFailed : currentVote.member_id ≠ memberId := by
      intro hEq
      apply hFresh
      have hMem : currentVote.member_id ∈ currentRoundVoteIds c := by
        simpa [currentRoundVoteIds] using
          (show ∃ vote, vote ∈ currentRoundVotes c ∧ vote.member_id = currentVote.member_id from
            ⟨currentVote, hOld, rfl⟩)
      simpa [hEq] using hMem
    rcases (show ∃ member, member ∈ seatedCouncilMembers c ∧ member.member_id = currentVote.member_id from
      by simpa [seatedCouncilMemberIds, councilMemberIds] using hOldSeat) with
      ⟨member, hMemberSeat, hMemberId⟩
    have hMemberMem : member ∈ c.council_members := by
      exact (List.mem_filter.mp hMemberSeat).1
    have hMemberStillSeated : memberIsSeated member := by
      exact (List.mem_filter.mp hMemberSeat).2
    have hMemberNe : member.member_id ≠ memberId := by
      simpa [hMemberId] using hNotFailed
    have hMemberMapped : member ∈ members := by
      apply List.mem_map.mpr
      exact ⟨member, hMemberMem, by simp [hMemberNe]⟩
    have hSeatNew : member ∈ seatedCouncilMembers { c with council_members := members } := by
      unfold seatedCouncilMembers
      exact List.mem_filter.mpr ⟨hMemberMapped, by simpa using hMemberStillSeated⟩
    simpa [seatedCouncilMemberIds, councilMemberIds] using
      (show ∃ candidate,
          candidate ∈ seatedCouncilMembers { c with council_members := members } ∧
            candidate.member_id = currentVote.member_id from
        ⟨member, hSeatNew, hMemberId⟩)
  refine ⟨?_, hFromSeated, ?_⟩
  · simpa [currentRoundVoteIdsDistinct, currentRoundVoteIds, currentRoundVotes, members] using hIntegrity.1
  · simpa [councilVoteRoundsBounded, members] using hIntegrity.2.2

/--
Advancing to the next deliberation round preserves deliberation-record
integrity.

Once the round increments, the new current-round vote list is empty.  The
bounded-round clause from the old state is exactly what justifies that claim.
-/
theorem advanceDeliberationRound_preserves_councilVoteIntegrity
    (c : ArbitrationCase)
    (hIntegrity : councilVoteIntegrity c) :
    councilVoteIntegrity { c with deliberation_round := c.deliberation_round + 1 } := by
  have hNoVotes :
      currentRoundVotes { c with deliberation_round := c.deliberation_round + 1 } = [] := by
    rw [List.eq_nil_iff_forall_not_mem]
    intro vote hVote
    have hVoteMem : vote ∈ c.council_votes := by
      exact (List.mem_filter.mp hVote).1
    have hVoteRound : vote.round = c.deliberation_round + 1 := by
      exact of_decide_eq_true (List.mem_filter.mp hVote).2
    have hLe : vote.round ≤ c.deliberation_round := hIntegrity.2.2 vote hVoteMem
    exact Nat.ne_of_lt (Nat.lt_succ_of_le hLe) hVoteRound
  refine ⟨?_, ?_, ?_⟩
  · simp [currentRoundVoteIdsDistinct, currentRoundVoteIds, hNoVotes]
  · simp [currentRoundVotesFromSeatedMembers, hNoVotes]
  · intro vote hVote
    have hVoteMem : vote ∈ c.council_votes := by simpa using hVote
    exact Nat.le_succ_of_le (hIntegrity.2.2 vote hVoteMem)

theorem currentRoundVoteIdsDistinct_set_votes
    (c : ArbitrationCase)
    (votes : List CouncilVote)
    (hDistinct : (votes.filter (fun vote => vote.round = c.deliberation_round)).map (·.member_id) |>.Nodup) :
    currentRoundVoteIds { c with council_votes := votes } |>.Nodup := by
  simpa [currentRoundVoteIds, currentRoundVotes]
    using hDistinct

theorem currentRoundVotesFromSeatedMembers_set_votes
    (c : ArbitrationCase)
    (votes : List CouncilVote)
    (members : List CouncilMember)
    (hMembers :
      ∀ vote ∈ votes.filter (fun vote => vote.round = c.deliberation_round),
        vote.member_id ∈ councilMemberIds (members.filter memberIsSeated)) :
    currentRoundVotesFromSeatedMembers
      { c with council_votes := votes, council_members := members } := by
  intro vote hVote
  simpa [currentRoundVotes, seatedCouncilMemberIds, seatedCouncilMembers, councilMemberIds]
    using hMembers vote hVote

theorem list_length_eq_of_nodup_same_members
    {xs ys : List String}
    (hXs : xs.Nodup)
    (hYs : ys.Nodup)
    (hXY : xs ⊆ ys)
    (hYX : ys ⊆ xs) :
    xs.length = ys.length := by
  induction xs generalizing ys with
  | nil =>
      exact (List.subset_nil.mp hYX).symm ▸ rfl
  | cons x xs ih =>
      have hXsInfo := List.nodup_cons.mp hXs
      have hNotMem : x ∉ xs := hXsInfo.1
      have hXsTail : xs.Nodup := hXsInfo.2
      have hMemY : x ∈ ys := hXY (by simp)
      have hYsErase : (ys.erase x).Nodup := hYs.erase x
      have hXsSub : xs ⊆ ys.erase x := by
        intro z hz
        have hzY : z ∈ ys := hXY (by simp [hz])
        have hzNe : z ≠ x := by
          intro hEq
          subst hEq
          exact hNotMem hz
        exact (List.Nodup.mem_erase_iff hYs).mpr ⟨hzNe, hzY⟩
      have hEraseSub : ys.erase x ⊆ xs := by
        intro z hz
        have hzInfo := (List.Nodup.mem_erase_iff hYs).mp hz
        have hzXs : z ∈ x :: xs := hYX hzInfo.2
        simpa [hzInfo.1] using hzXs
      have hTailLen : xs.length = (ys.erase x).length := ih hXsTail hYsErase hXsSub hEraseSub
      calc
        (x :: xs).length = xs.length + 1 := by simp
        _ = (ys.erase x).length + 1 := by simp [hTailLen]
        _ = ys.length := by
          rw [List.length_erase_of_mem hMemY]
          exact Nat.sub_add_cancel (Nat.succ_le_of_lt (List.length_pos_of_mem hMemY))

theorem currentRoundVoteIds_length_eq_seatedCouncilMemberIds_length_of_same_members
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hIntegrity : councilVoteIntegrity c)
    (hCover : seatedCouncilMemberIds c ⊆ currentRoundVoteIds c) :
    (currentRoundVoteIds c).length = (seatedCouncilMemberIds c).length := by
  have hVoteSubset : currentRoundVoteIds c ⊆ seatedCouncilMemberIds c := by
    intro memberId hMem
    have hVote :
        ∃ vote, vote ∈ currentRoundVotes c ∧ vote.member_id = memberId := by
      simpa [currentRoundVoteIds] using hMem
    rcases hVote with ⟨vote, hVoteMem, hVoteId⟩
    simpa [hVoteId] using hIntegrity.2.1 vote hVoteMem
  exact list_length_eq_of_nodup_same_members
    hIntegrity.1
    (seatedCouncilMemberIds_nodup c hUnique)
    hVoteSubset
    hCover

theorem nextCouncilMember_none_implies_round_complete
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hIntegrity : councilVoteIntegrity c)
    (hNone : nextCouncilMember? c = none) :
    (currentRoundVotes c).length = seatedCouncilMemberCount c := by
  have hNoGap :
      seatedCouncilMemberIds c ⊆ currentRoundVoteIds c := by
    intro memberId hMem
    have hFindNone :
        (seatedCouncilMembers c).find?
          (fun member => !(currentRoundVotes c).any (fun vote => vote.member_id = member.member_id)) =
        none := by
      simpa [nextCouncilMember?] using hNone
    have hForall :=
      (List.find?_eq_none.mp hFindNone)
    have hMember :
        ∃ member, member ∈ seatedCouncilMembers c ∧ member.member_id = memberId := by
      simpa [seatedCouncilMemberIds, councilMemberIds] using hMem
    rcases hMember with ⟨member, hMemberMem, hMemberId⟩
    have hMemberVotes :
        ¬ !(currentRoundVotes c).any (fun vote => vote.member_id = memberId) = true := by
      simpa [hMemberId] using hForall member hMemberMem
    have hAnyTrue : (currentRoundVotes c).any (fun vote => vote.member_id = memberId) = true := by
      cases hAny : (currentRoundVotes c).any (fun vote => vote.member_id = memberId) with
      | false =>
          simp [hAny] at hMemberVotes
      | true =>
          rfl
    rcases List.any_eq_true.mp hAnyTrue with ⟨vote, hVoteMem, hVoteId⟩
    have hVote :
        ∃ vote, vote ∈ currentRoundVotes c ∧ vote.member_id = memberId := by
      exact ⟨vote, hVoteMem, of_decide_eq_true hVoteId⟩
    simpa [currentRoundVoteIds] using hVote
  have hLengths :
      (currentRoundVoteIds c).length = (seatedCouncilMemberIds c).length :=
    currentRoundVoteIds_length_eq_seatedCouncilMemberIds_length_of_same_members
      c hUnique hIntegrity hNoGap
  simpa [currentRoundVoteIds, currentRoundVotes, seatedCouncilMemberIds,
    seatedCouncilMemberCount, seatedCouncilMembers, councilMemberIds] using hLengths

/--
Initialization establishes deliberation-record integrity immediately.

The initialized case starts with no stored council votes.  That trivial base
case is the root of the later reachable-state theorem.
-/
theorem initializeCase_establishes_councilVoteIntegrity
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    councilVoteIntegrity s.case := by
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
                  simp [councilVoteIntegrity, currentRoundVoteIdsDistinct,
                    currentRoundVotesFromSeatedMembers, councilVoteRoundsBounded,
                    currentRoundVoteIds, currentRoundVotes]

/--
`continueDeliberation` preserves deliberation-record integrity.

The function either closes the case with the same deliberation data, advances
to the next round, or leaves the case in the same round.  The earlier local
lemmas cover exactly those three shapes.
-/
theorem continueDeliberation_preserves_councilVoteIntegrity_for
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hIntegrity : councilVoteIntegrity c)
    (hCont : continueDeliberation s c = .ok t) :
    councilVoteIntegrity t.case := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        apply councilVoteIntegrity_congr (c := c)
        · rfl
        · rfl
        · rfl
        · exact hIntegrity
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          apply councilVoteIntegrity_congr (c := c)
          · rfl
          · rfl
          · rfl
          · exact hIntegrity
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            apply councilVoteIntegrity_congr (c := c)
            · rfl
            · rfl
            · rfl
            · exact hIntegrity
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact advanceDeliberationRound_preserves_councilVoteIntegrity c hIntegrity
  · simp [hRoundComplete] at hCont
    cases hCont
    apply councilVoteIntegrity_congr (c := c)
    · rfl
    · rfl
    · rfl
    · exact hIntegrity

/--
The merits-submission helper preserves deliberation-record integrity when
supplemental materials are allowed.
-/
theorem recordMeritsSubmission_with_materials_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (payload : Lean.Json)
    (hIntegrity : councilVoteIntegrity s.case)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit true payload = .ok t) :
    councilVoteIntegrity t.case := by
  rcases recordMeritsSubmission_with_materials_result
      s t phase actorRole expectedRole textLabel limit payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  exact appendSupplementalMaterials_preserves_councilVoteIntegrity
    (addFiling s.case phase expectedRole (trimString rawText))
    offered
    reports
    (addFiling_preserves_councilVoteIntegrity s.case phase expectedRole (trimString rawText) hIntegrity)

/--
The merits-submission helper preserves deliberation-record integrity when no
supplemental materials are allowed.
-/
theorem recordMeritsSubmission_without_materials_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (payload : Lean.Json)
    (hIntegrity : councilVoteIntegrity s.case)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit false payload = .ok t) :
    councilVoteIntegrity t.case := by
  rcases recordMeritsSubmission_without_materials_result
      s t phase actorRole expectedRole textLabel limit payload hSubmit with
    ⟨rawText, rfl⟩
  exact addFiling_preserves_councilVoteIntegrity
    s.case phase expectedRole (trimString rawText) hIntegrity

/--
Opening preserves deliberation-record integrity.
-/
theorem step_record_opening_statement_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact addFiling_preserves_councilVoteIntegrity
    s.case "openings"
    (if s.case.openings.isEmpty then "plaintiff" else "defendant")
    (trimString rawText)
    hIntegrity

/--
Argument preserves deliberation-record integrity.
-/
theorem step_submit_argument_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
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
  exact recordMeritsSubmission_with_materials_preserves_councilVoteIntegrity
    s t "arguments" action.actor_role
    (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
    "argument" s.policy.max_argument_chars action.payload hIntegrity hSubmit

/--
Rebuttal preserves deliberation-record integrity.
-/
theorem step_submit_rebuttal_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
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
  exact recordMeritsSubmission_with_materials_preserves_councilVoteIntegrity
    s t "rebuttals" action.actor_role "plaintiff"
    "rebuttal" s.policy.max_rebuttal_chars action.payload hIntegrity hSubmit

/--
Surrebuttal preserves deliberation-record integrity.
-/
theorem step_submit_surrebuttal_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
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
  exact recordMeritsSubmission_with_materials_preserves_councilVoteIntegrity
    s t "surrebuttals" action.actor_role "defendant"
    "surrebuttal" s.policy.max_surrebuttal_chars action.payload hIntegrity hSubmit

/--
Closing preserves deliberation-record integrity.
-/
theorem step_deliver_closing_statement_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact addFiling_preserves_councilVoteIntegrity
    s.case "closings"
    (if s.case.closings.isEmpty then "plaintiff" else "defendant")
    (trimString rawText)
    hIntegrity

/--
Passing rebuttal or surrebuttal preserves deliberation-record integrity.
-/
theorem step_pass_phase_opportunity_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
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
            apply councilVoteIntegrity_congr (c := s.case)
            · rfl
            · rfl
            · rfl
            · exact hIntegrity
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
              apply councilVoteIntegrity_congr (c := s.case)
              · rfl
              · rfl
              · rfl
              · exact hIntegrity
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

/--
A successful evidence submission preserves deliberation-record integrity.
Evidence submission appends source material, not council votes or membership.
-/
theorem step_submit_evidence_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  apply councilVoteIntegrity_congr (c := s.case)
  · rfl
  · rfl
  · rfl
  · exact hIntegrity

/--
A successful council vote preserves deliberation-record integrity.
-/
theorem step_submit_council_vote_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
  rcases step_submit_council_vote_details s t action hType hStep with
    ⟨memberId, vote, rationale, _hPhase, hSeated, hFresh, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hIntegrity1 : councilVoteIntegrity c1 := by
    exact appendCurrentRoundVote_preserves_councilVoteIntegrity
      s.case memberId vote rationale hIntegrity hSeated hFresh
  exact continueDeliberation_preserves_councilVoteIntegrity_for s t c1 hIntegrity1 hCont

/--
A successful council removal preserves deliberation-record integrity.
-/
theorem step_remove_council_member_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
  rcases step_remove_council_member_details s t action hType hStep with
    ⟨memberId, status, _hPhase, _hSeated, hFresh, _hStatus, hCont⟩
  let c1 := { s.case with
    council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hIntegrity1 : councilVoteIntegrity c1 := by
    simpa [c1] using removeUnvotedCouncilMember_preserves_councilVoteIntegrity
      s.case memberId status hIntegrity hFresh
  exact continueDeliberation_preserves_councilVoteIntegrity_for s t c1 hIntegrity1 (by
    simpa [c1] using hCont)

theorem failOpportunity_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hIntegrity : councilVoteIntegrity s.case)
    (hFail : failOpportunity s payload = .ok t) :
    councilVoteIntegrity t.case := by
  rcases failOpportunity_result s t payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨memberId, reason, opportunityId, message, c1, hC1,
      _hDelib, _hSeated, hFresh, hCont⟩
    have hFreshIds : memberId ∉ currentRoundVoteIds s.case := by
      simpa [currentRoundVoteIds] using hFresh
    have hIntegrity1 : councilVoteIntegrity c1 := by
      rw [hC1]
      exact failUnvotedCouncilMember_preserves_councilVoteIntegrity
        s.case memberId reason opportunityId message hIntegrity hFreshIds
    exact continueDeliberation_preserves_councilVoteIntegrity_for s t c1 hIntegrity1 hCont
  · rcases hParty with ⟨_failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    apply councilVoteIntegrity_congr (c := s.case)
    · rfl
    · rfl
    · rfl
    · exact hIntegrity

theorem step_fail_opportunity_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
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
      exact failOpportunity_preserves_councilVoteIntegrity s t action.payload hIntegrity hStep'

/--
Every successful public step preserves deliberation-record integrity.

The proof follows the public action surface, just as the earlier global
invariant files did.  Merits actions preserve the invariant because they leave
deliberation data alone.  The two council actions preserve it through the local
lemmas proved above.
-/
theorem step_preserves_councilVoteIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hIntegrity : councilVoteIntegrity s.case)
    (hStep : step { state := s, action := action } = .ok t) :
    councilVoteIntegrity t.case := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact step_record_opening_statement_preserves_councilVoteIntegrity
      s t action hOpening hIntegrity hStepCore
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact step_submit_argument_preserves_councilVoteIntegrity
        s t action hArgument hIntegrity hStepCore
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact step_submit_rebuttal_preserves_councilVoteIntegrity
          s t action hRebuttal hIntegrity hStepCore
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact step_submit_surrebuttal_preserves_councilVoteIntegrity
            s t action hSurrebuttal hIntegrity hStepCore
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact step_deliver_closing_statement_preserves_councilVoteIntegrity
              s t action hClosing hIntegrity hStepCore
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact step_pass_phase_opportunity_preserves_councilVoteIntegrity
                s t action hPass hIntegrity hStepCore
            · by_cases hEvidence : action.action_type = "submit_evidence"
              · exact step_submit_evidence_preserves_councilVoteIntegrity
                  s t action hEvidence hIntegrity hStepCore
              · by_cases hVote : action.action_type = "submit_council_vote"
                · exact step_submit_council_vote_preserves_councilVoteIntegrity
                    s t action hVote hIntegrity hStepCore
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · exact step_remove_council_member_preserves_councilVoteIntegrity
                      s t action hRemoval hIntegrity hStepCore
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_preserves_councilVoteIntegrity
                        s t action hFail hIntegrity hStepCore
                    · simp [stepCore] at hStepCore

/--
Every reachable state preserves deliberation-record integrity.

Initialization starts from an empty vote list.  Every later successful public
step preserves the invariant.  This is the reachable-state theorem that the
later liveness proof will use.
-/
theorem reachable_councilVoteIntegrity
    (s : ArbitrationState)
    (hs : Reachable s) :
    councilVoteIntegrity s.case := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_councilVoteIntegrity req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_councilVoteIntegrity s t action ih hStep

/--
Successful public steps preserve the uniqueness of council member identifiers.

The case-frame theorem already proved the stronger structural fact: successful
steps preserve the entire list of council member identifiers.  This theorem
extracts the one consequence the later deliberation proofs need.
-/
theorem step_preserves_councilIdsUnique
    (s t : ArbitrationState)
    (action : CourtAction)
    (hUnique : councilIdsUnique s.case)
    (hStep : step { state := s, action := action } = .ok t) :
    councilIdsUnique t.case := by
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
  rcases hFrame' with ⟨_hProp, _hPolicy, hMembers⟩
  unfold councilIdsUnique at hUnique ⊢
  simpa [hMembers] using hUnique

/--
Every reachable state preserves unique council member identifiers.

This is the first reachable deliberation invariant.  The initialization proof
now guarantees unique council identifiers, and the case-frame theorem keeps the
identifier list fixed through every later successful step.
-/
theorem reachable_councilIdsUnique
    (s : ArbitrationState)
    (hs : Reachable s) :
    councilIdsUnique s.case := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_councilIdsUnique req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_councilIdsUnique s t action ih hStep

theorem councilMemberIds_eq_nil_iff
    (members : List CouncilMember) :
    councilMemberIds members = [] ↔ members = [] := by
  cases members with
  | nil =>
      simp [councilMemberIds]
  | cons member rest =>
      simp [councilMemberIds]

theorem initializeCase_establishes_nonempty_council
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.case.council_members ≠ [] := by
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
                  simpa [List.isEmpty_iff] using hEmpty

theorem step_preserves_nonempty_council
    (s t : ArbitrationState)
    (action : CourtAction)
    (hNonempty : s.case.council_members ≠ [])
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.council_members ≠ [] := by
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
  rcases hFrame' with ⟨_hProp, _hPolicy, hMembers⟩
  intro hEmpty
  apply hNonempty
  have hIdsEmptyT : councilMemberIds t.case.council_members = [] := by
    simp [councilMemberIds, hEmpty]
  have hIdsEmptyS : councilMemberIds s.case.council_members = [] := by
    simpa [hMembers] using hIdsEmptyT
  exact (councilMemberIds_eq_nil_iff s.case.council_members).mp hIdsEmptyS

theorem reachable_nonempty_council
    (s : ArbitrationState)
    (hs : Reachable s) :
    s.case.council_members ≠ [] := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_nonempty_council req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_nonempty_council s t action ih hStep

end ArbProofs
