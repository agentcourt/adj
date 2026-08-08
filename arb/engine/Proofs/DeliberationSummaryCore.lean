import Proofs.CouncilIntegrity

namespace ArbProofs

theorem seatedCouncilMemberCount_le_memberCount
    (c : ArbitrationCase) :
    seatedCouncilMemberCount c ≤ c.council_members.length := by
  unfold seatedCouncilMemberCount seatedCouncilMembers
  induction c.council_members with
  | nil =>
      simp
  | cons head tail ih =>
      cases hSeat : memberIsSeated head with
      | false =>
          simpa [hSeat] using
            Nat.le_trans ih (Nat.le_succ tail.length)
      | true =>
          simpa [hSeat] using Nat.succ_le_succ ih

/--
Successful initialization fixes the council roster length at `policy.council_size`.
-/
theorem initializeCase_establishes_councilMemberCount_eq_policySize
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.case.council_members.length = s.policy.council_size := by
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
          · by_cases hLength : req.council_members.length = req.state.policy.council_size
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
                  simpa using hLength
            · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength] at hInit
              cases hInit

/--
Successful public steps preserve the council-roster length.
-/
theorem step_preserves_councilMemberCount_eq_policySize
    (s t : ArbitrationState)
    (action : CourtAction)
    (hCount : s.case.council_members.length = s.policy.council_size)
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.council_members.length = t.policy.council_size := by
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
  rcases hFrame' with ⟨_hProp, hPolicyEq, hIdsEq⟩
  have hLenEq : t.case.council_members.length = s.case.council_members.length := by
    calc
      t.case.council_members.length = (councilMemberIds t.case.council_members).length := by
        simp [councilMemberIds]
      _ = (councilMemberIds s.case.council_members).length := by
        simp [hIdsEq]
      _ = s.case.council_members.length := by
        simp [councilMemberIds]
  calc
    t.case.council_members.length = s.case.council_members.length := hLenEq
    _ = s.policy.council_size := hCount
    _ = t.policy.council_size := by simp [hPolicyEq]

/--
Every reachable state keeps the initialized council-roster length.
-/
theorem reachable_councilMemberCount_eq_policySize
    (s : ArbitrationState)
    (hs : Reachable s) :
    s.case.council_members.length = s.policy.council_size := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_councilMemberCount_eq_policySize req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_councilMemberCount_eq_policySize s t action ih hStep

/--
Every reachable state has at most `policy.council_size` seated members.
-/
theorem reachable_seatedCouncilMemberCount_le_councilSize
    (s : ArbitrationState)
    (hs : Reachable s) :
    seatedCouncilMemberCount s.case ≤ s.policy.council_size := by
  calc
    seatedCouncilMemberCount s.case ≤ s.case.council_members.length :=
      seatedCouncilMemberCount_le_memberCount s.case
    _ = s.policy.council_size :=
      reachable_councilMemberCount_eq_policySize s hs

/--
For nodup lists, subset membership bounds length.
-/
theorem list_length_le_of_nodup_subset
    {α : Type}
    [BEq α]
    [LawfulBEq α]
    {xs ys : List α}
    (hxs : xs.Nodup)
    (hys : ys.Nodup)
    (hSub : xs ⊆ ys) :
    xs.length ≤ ys.length := by
  induction xs generalizing ys with
  | nil =>
      simp
  | cons x xs ih =>
      have hXsInfo := List.nodup_cons.mp hxs
      have hNotMem : x ∉ xs := hXsInfo.1
      have hXsTail : xs.Nodup := hXsInfo.2
      have hMemY : x ∈ ys := hSub (by simp)
      have hYsErase : (ys.erase x).Nodup := hys.erase x
      have hTailSub : xs ⊆ ys.erase x := by
        intro z hz
        have hzY : z ∈ ys := hSub (by simp [hz])
        have hzNe : z ≠ x := by
          intro hEq
          subst hEq
          exact hNotMem hz
        exact (List.Nodup.mem_erase_iff hys).mpr ⟨hzNe, hzY⟩
      have hLen : xs.length ≤ (ys.erase x).length := ih hXsTail hYsErase hTailSub
      calc
        (x :: xs).length = xs.length + 1 := by simp
        _ ≤ (ys.erase x).length + 1 := Nat.add_le_add_right hLen 1
        _ = ys.length := by
          rw [List.length_erase_of_mem hMemY]
          exact Nat.sub_add_cancel (Nat.succ_le_of_lt (List.length_pos_of_mem hMemY))

/--
If a nodup subset omits a known element of the larger nodup list, the subset is
strictly shorter.
-/
theorem list_length_lt_of_nodup_subset_and_fresh
    {α : Type}
    [BEq α]
    [LawfulBEq α]
    {xs ys : List α}
    {x : α}
    (hxs : xs.Nodup)
    (hys : ys.Nodup)
    (hSub : xs ⊆ ys)
    (hMem : x ∈ ys)
    (hFresh : x ∉ xs) :
    xs.length < ys.length := by
  have hConsSub : x :: xs ⊆ ys := by
    intro z hz
    rcases List.mem_cons.mp hz with hEq | hzXs
    · simpa [hEq] using hMem
    · exact hSub hzXs
  have hConsLen : (x :: xs).length ≤ ys.length :=
    list_length_le_of_nodup_subset
      (List.nodup_cons.mpr ⟨hFresh, hxs⟩)
      hys
      hConsSub
  exact Nat.lt_of_add_one_le hConsLen

/--
Current-round vote identifiers are always a subset of the seated council-member
identifiers.
-/
theorem currentRoundVoteIds_subset_seatedCouncilMemberIds
    (c : ArbitrationCase)
    (hIntegrity : councilVoteIntegrity c) :
    currentRoundVoteIds c ⊆ seatedCouncilMemberIds c := by
  intro memberId hMem
  have hVote :
      ∃ vote, vote ∈ currentRoundVotes c ∧ vote.member_id = memberId := by
    simpa [currentRoundVoteIds] using hMem
  rcases hVote with ⟨vote, hVoteMem, hVoteId⟩
  simpa [hVoteId] using hIntegrity.2.1 vote hVoteMem

/--
A seated member who has not yet voted implies that the current round still has
one unit of vote capacity left.
-/
theorem currentRoundVotes_length_lt_seatedCouncilMemberCount_of_fresh_seated
    (c : ArbitrationCase)
    (memberId : String)
    (hUnique : councilIdsUnique c)
    (hIntegrity : councilVoteIntegrity c)
    (hSeated : memberId ∈ seatedCouncilMemberIds c)
    (hFresh : memberId ∉ currentRoundVoteIds c) :
    (currentRoundVotes c).length < seatedCouncilMemberCount c := by
  have hLt :
      (currentRoundVoteIds c).length < (seatedCouncilMemberIds c).length := by
    exact list_length_lt_of_nodup_subset_and_fresh
      hIntegrity.1
      (seatedCouncilMemberIds_nodup c hUnique)
      (currentRoundVoteIds_subset_seatedCouncilMemberIds c hIntegrity)
      hSeated
      hFresh
  simpa [currentRoundVoteIds, seatedCouncilMemberIds, seatedCouncilMemberCount,
    councilMemberIds] using hLt

structure DeliberationSummary where
  required_votes : Nat
  seated_count : Nat
  current_round_vote_count : Nat
  demonstrated_count : Nat
  not_demonstrated_count : Nat
  deliberation_round : Nat
  max_deliberation_rounds : Nat
  deriving Inhabited, DecidableEq, Repr

@[ext] theorem DeliberationSummary.ext
    (d e : DeliberationSummary)
    (hRequired : d.required_votes = e.required_votes)
    (hSeated : d.seated_count = e.seated_count)
    (hRoundVotes : d.current_round_vote_count = e.current_round_vote_count)
    (hDem : d.demonstrated_count = e.demonstrated_count)
    (hNot : d.not_demonstrated_count = e.not_demonstrated_count)
    (hRound : d.deliberation_round = e.deliberation_round)
    (hMaxRounds : d.max_deliberation_rounds = e.max_deliberation_rounds) :
    d = e := by
  cases d
  cases e
  simp at hRequired hSeated hRoundVotes hDem hNot hRound hMaxRounds
  simp [hRequired, hSeated, hRoundVotes, hDem, hNot, hRound, hMaxRounds]

def DeliberationSummary.substantive_vote_count (d : DeliberationSummary) : Nat :=
  d.demonstrated_count + d.not_demonstrated_count

def DeliberationSummary.uncast_vote_count (d : DeliberationSummary) : Nat :=
  d.seated_count - d.current_round_vote_count

def DeliberationSummary.rounds_remaining (d : DeliberationSummary) : Nat :=
  d.max_deliberation_rounds - d.deliberation_round

def DeliberationSummary.currentResolution? (d : DeliberationSummary) : Option String :=
  if d.demonstrated_count >= d.required_votes then
    some "demonstrated"
  else if d.not_demonstrated_count >= d.required_votes then
    some "not_demonstrated"
  else
    none

def deliberationSummaryForCase
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat) : DeliberationSummary :=
  let votes := currentRoundVotes c
  { required_votes := requiredVotes
    seated_count := seatedCouncilMemberCount c
    current_round_vote_count := votes.length
    demonstrated_count := voteCountFor votes "demonstrated"
    not_demonstrated_count := voteCountFor votes "not_demonstrated"
    deliberation_round := c.deliberation_round
    max_deliberation_rounds := maxRounds }

def deliberationSummary (s : ArbitrationState) : DeliberationSummary :=
  deliberationSummaryForCase
    s.case
    s.policy.required_votes_for_decision
    s.policy.max_deliberation_rounds

private theorem voteCountFor_foldl_acc
    (votes : List CouncilVote)
    (value : String)
    (acc : Nat) :
    votes.foldl
        (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
        acc =
      acc + voteCountFor votes value := by
  induction votes generalizing acc with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      by_cases hValue : trimString vote.vote = value
      · calc
          (vote :: votes).foldl
              (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
              acc
            = votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                (acc + 1) := by
                  simp [List.foldl, hValue]
          _ = (acc + 1) + voteCountFor votes value := by
                simpa using ih (acc + 1)
          _ = acc + (1 + voteCountFor votes value) := by
                omega
          _ = acc + votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                1 := by
                  have hOne :
                      votes.foldl
                          (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                          1 =
                        1 + voteCountFor votes value := by
                    simpa using ih 1
                  rw [hOne]
          _ = acc + voteCountFor (vote :: votes) value := by
                unfold voteCountFor
                simp [List.foldl, hValue]
      · calc
          (vote :: votes).foldl
              (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
              acc
            = votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                acc := by
                  simp [List.foldl, hValue]
          _ = acc + voteCountFor votes value := by
                simpa using ih acc
          _ = acc + voteCountFor (vote :: votes) value := by
                unfold voteCountFor
                simp [List.foldl, hValue]

theorem voteCountFor_cons
    (vote : CouncilVote)
    (votes : List CouncilVote)
    (value : String) :
    voteCountFor (vote :: votes) value =
      (if trimString vote.vote = value then 1 else 0) + voteCountFor votes value := by
  unfold voteCountFor
  simpa only [voteCountFor, List.foldl] using voteCountFor_foldl_acc votes value
    (if trimString vote.vote = value then 1 else 0)

theorem substantive_vote_counts_le_length
    (votes : List CouncilVote) :
    voteCountFor votes "demonstrated" +
      voteCountFor votes "not_demonstrated" ≤
      votes.length := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      rw [voteCountFor_cons, voteCountFor_cons]
      by_cases hDem : trimString vote.vote = "demonstrated"
      · have hTail := ih
        simp [hDem]
        omega
      · by_cases hNot : trimString vote.vote = "not_demonstrated"
        · have hTail := ih
          simp [hNot]
          omega
        · have hTail := ih
          simp [hDem, hNot]
          omega

@[simp] theorem deliberationSummaryForCase_currentResolution
    (c : ArbitrationCase)
    (requiredVotes maxRounds : Nat) :
    (deliberationSummaryForCase c requiredVotes maxRounds).currentResolution? =
      currentResolution? c requiredVotes := by
  simp [deliberationSummaryForCase, DeliberationSummary.currentResolution?, currentResolution?]

@[simp] theorem deliberationSummary_currentResolution
    (s : ArbitrationState) :
    (deliberationSummary s).currentResolution? =
      currentResolution? s.case s.policy.required_votes_for_decision := by
  simp [deliberationSummary]

theorem DeliberationSummary.currentResolution_demonstrated_implies_threshold
    (d : DeliberationSummary)
    (hResolution : d.currentResolution? = some "demonstrated") :
    d.demonstrated_count ≥ d.required_votes := by
  unfold DeliberationSummary.currentResolution? at hResolution
  by_cases hDem : d.demonstrated_count ≥ d.required_votes
  · exact hDem
  · simp [hDem] at hResolution

theorem DeliberationSummary.currentResolution_not_demonstrated_implies_threshold
    (d : DeliberationSummary)
    (hResolution : d.currentResolution? = some "not_demonstrated") :
    d.not_demonstrated_count ≥ d.required_votes := by
  unfold DeliberationSummary.currentResolution? at hResolution
  by_cases hDem : d.demonstrated_count ≥ d.required_votes
  · simp [hDem] at hResolution
  · by_cases hNot : d.not_demonstrated_count ≥ d.required_votes
    · exact hNot
    · simp [hDem, hNot] at hResolution

theorem DeliberationSummary.currentResolution_none_implies_below_threshold
    (d : DeliberationSummary)
    (hResolution : d.currentResolution? = none) :
    d.demonstrated_count < d.required_votes ∧
      d.not_demonstrated_count < d.required_votes := by
  unfold DeliberationSummary.currentResolution? at hResolution
  by_cases hDem : d.demonstrated_count ≥ d.required_votes
  · simp [hDem] at hResolution
  · by_cases hNot : d.not_demonstrated_count ≥ d.required_votes
    · simp [hDem, hNot] at hResolution
    · exact ⟨Nat.lt_of_not_ge hDem, Nat.lt_of_not_ge hNot⟩

theorem currentResolution_demonstrated_implies_sound
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (hResolution : currentResolution? c requiredVotes = some "demonstrated") :
    voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes := by
  let d := deliberationSummaryForCase c requiredVotes 0
  have hSummary : d.currentResolution? = some "demonstrated" := by
    simpa [d] using hResolution
  have hCount : d.demonstrated_count ≥ d.required_votes :=
    d.currentResolution_demonstrated_implies_threshold hSummary
  simpa [d, deliberationSummaryForCase] using hCount

theorem currentResolution_not_demonstrated_implies_sound
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (hResolution : currentResolution? c requiredVotes = some "not_demonstrated") :
    voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ requiredVotes := by
  let d := deliberationSummaryForCase c requiredVotes 0
  have hSummary : d.currentResolution? = some "not_demonstrated" := by
    simpa [d] using hResolution
  have hCount : d.not_demonstrated_count ≥ d.required_votes :=
    d.currentResolution_not_demonstrated_implies_threshold hSummary
  simpa [d, deliberationSummaryForCase] using hCount

theorem currentResolution_none_implies_below_threshold
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (hResolution : currentResolution? c requiredVotes = none) :
    voteCountFor (currentRoundVotes c) "demonstrated" < requiredVotes ∧
      voteCountFor (currentRoundVotes c) "not_demonstrated" < requiredVotes := by
  let d := deliberationSummaryForCase c requiredVotes 0
  have hSummary : d.currentResolution? = none := by
    simpa [d] using hResolution
  have hCounts := d.currentResolution_none_implies_below_threshold hSummary
  simpa [d, deliberationSummaryForCase] using hCounts

theorem deliberationSummary_substantive_vote_count_le_current_round_vote_count
    (s : ArbitrationState) :
    (deliberationSummary s).substantive_vote_count ≤
      (deliberationSummary s).current_round_vote_count := by
  simpa [deliberationSummary, deliberationSummaryForCase,
    DeliberationSummary.substantive_vote_count] using
    substantive_vote_counts_le_length (currentRoundVotes s.case)

theorem deliberationSummary_current_round_vote_count_lt_seated_count_of_fresh_seated
    (s : ArbitrationState)
    (memberId : String)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case)
    (hSeated : memberId ∈ seatedCouncilMemberIds s.case)
    (hFresh : memberId ∉ currentRoundVoteIds s.case) :
    (deliberationSummary s).current_round_vote_count <
      (deliberationSummary s).seated_count := by
  simpa [deliberationSummary, deliberationSummaryForCase] using
    currentRoundVotes_length_lt_seatedCouncilMemberCount_of_fresh_seated
      s.case memberId hUnique hIntegrity hSeated hFresh

/--
If `nextCouncilMember?` identifies a member, that member is seated and fresh
for the current round.
-/
theorem nextCouncilMember_some_implies_seated_and_fresh
    (c : ArbitrationCase)
    (member : CouncilMember)
    (hNext : nextCouncilMember? c = some member) :
    member.member_id ∈ seatedCouncilMemberIds c ∧
      member.member_id ∉ currentRoundVoteIds c := by
  have hFind :
      (seatedCouncilMembers c).find?
        (fun candidate =>
          !(currentRoundVotes c).any (fun vote => vote.member_id = candidate.member_id)) =
        some member := by
    simpa [nextCouncilMember?] using hNext
  have hMem : member ∈ seatedCouncilMembers c := by
    exact List.mem_of_find?_eq_some hFind
  have hFreshBool :
      !(currentRoundVotes c).any (fun vote => vote.member_id = member.member_id) = true := by
    simpa using List.find?_some hFind
  have hSeatedId : member.member_id ∈ seatedCouncilMemberIds c := by
    simpa [seatedCouncilMemberIds, councilMemberIds] using
      (List.mem_map.mpr ⟨member, hMem, rfl⟩ :
        member.member_id ∈ (seatedCouncilMembers c).map (·.member_id))
  have hFreshId : member.member_id ∉ currentRoundVoteIds c := by
    intro hVoteId
    have hVote :
        ∃ vote, vote ∈ currentRoundVotes c ∧ vote.member_id = member.member_id := by
      exact List.mem_map.1 (by simpa [currentRoundVoteIds] using hVoteId)
    have hAnyTrue : (currentRoundVotes c).any (fun vote => vote.member_id = member.member_id) = true := by
      rcases hVote with ⟨vote, hVoteMem, hVoteMember⟩
      exact List.any_eq_true.mpr ⟨vote, hVoteMem, by simp [hVoteMember]⟩
    have hImpossible : False := by
      simp [hAnyTrue] at hFreshBool
    exact hImpossible.elim
  exact ⟨hSeatedId, hFreshId⟩

/--
A chosen next council voter means that the summary still has current-round vote
capacity left.
-/
theorem nextCouncilMember_some_implies_deliberationSummary_round_capacity
    (s : ArbitrationState)
    (member : CouncilMember)
    (hUnique : councilIdsUnique s.case)
    (hIntegrity : councilVoteIntegrity s.case)
    (hNext : nextCouncilMember? s.case = some member) :
    (deliberationSummary s).current_round_vote_count <
      (deliberationSummary s).seated_count := by
  rcases nextCouncilMember_some_implies_seated_and_fresh s.case member hNext with
    ⟨hSeated, hFresh⟩
  exact deliberationSummary_current_round_vote_count_lt_seated_count_of_fresh_seated
    s member.member_id hUnique hIntegrity hSeated hFresh

end ArbProofs
