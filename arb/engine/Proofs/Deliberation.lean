import Proofs.Samples

open ArbProofs

/-
This file proves the current deliberation rules.

The merits sequence is linear and easy to follow.  Deliberation carries the
main decision logic of the engine.  The important questions are precise.

What happens when one side reaches the required number of votes?

What happens when timeouts or removals make the threshold impossible before the
round is complete?

What happens when every seated member votes in a round and no side reaches the
threshold?

Which council member receives the next opportunity when some members have
already voted and others are no longer seated?

The theorems below answer those questions directly on small explicit states.
They state the current meaning of the engine's deliberation code.
-/

def activeDeliberationCase : ArbitrationCase :=
  { baseCase with
    status := "active"
    phase := "deliberation"
    council_members := seatedCouncil
    deliberation_round := 1
  }

def activeDeliberationState : ArbitrationState :=
  { baseState with case := activeDeliberationCase }

def oneDemonstratedVoteState : ArbitrationState :=
  { activeDeliberationState with
    case := { activeDeliberationCase with
      council_votes :=
        [ { member_id := "C1"
          , round := 1
          , vote := "demonstrated"
          , rationale := "first vote"
          } ]
    } }

def afterSecondDemonstratedVote : Except String ArbitrationState :=
  step
    { state := oneDemonstratedVoteState
    , action := councilVoteAction "C2" "demonstrated" "second vote"
    }

def afterThirdDemonstratedVote : Except String ArbitrationState :=
  step
    { state :=
        match afterSecondDemonstratedVote with
        | .ok state => state
        | .error _ => default
    , action := councilVoteAction "C3" "demonstrated" "third vote"
    }

/-
This sample round uses the three-vote policy and enters the third vote with no
possible winner yet.

The point of the sample is simple.  After the final seated member votes, the
round is complete, but neither side has the required three votes.  The engine
should therefore advance to round two instead of closing the case.
-/
def splitRoundState : ArbitrationState :=
  { forum_name := baseState.forum_name
  , case := { activeDeliberationCase with
      council_votes :=
        [ { member_id := "C1", round := 1, vote := "demonstrated", rationale := "r1" }
        , { member_id := "C2", round := 1, vote := "not_demonstrated", rationale := "r2" }
        ]
    }
  , policy := supermajorityPolicy
  , state_version := baseState.state_version
  }

def afterFullSplitRound : Except String ArbitrationState :=
  step
    { state := splitRoundState
    , action := councilVoteAction "C3" "not_demonstrated" "r3"
    }

/-
This sample removal state uses the same three-vote policy.

The case begins with three seated members and requires all three votes for a
decision.  Removing one member makes that threshold impossible, but the engine
now keeps deliberation open until the remaining seated members finish the
round.
-/
def impossibleAfterRemovalState : ArbitrationState :=
  { forum_name := baseState.forum_name
  , case := activeDeliberationCase
  , policy := supermajorityPolicy
  , state_version := baseState.state_version
  }

def afterRemovalMakesThresholdImpossible : Except String ArbitrationState :=
  step
    { state := impossibleAfterRemovalState
    , action := removeCouncilMemberAction "C3" "timed_out"
    }

def removeVotedMemberState : ArbitrationState :=
  oneDemonstratedVoteState

def afterRemovingCurrentRoundVoter : Except String ArbitrationState :=
  step
    { state := removeVotedMemberState
    , action := removeCouncilMemberAction "C1" "timed_out"
    }

/-
This sample opportunity state is for member selection.

One member has already voted in the current round.  One member is no longer
seated.  The remaining seated member should receive the next deliberation
opportunity.
-/
def votedAndTimedOutCase : ArbitrationCase :=
  { activeDeliberationCase with
    council_members :=
      [ sampleMember "C1" "m1" "p1" "seated"
      , sampleMember "C2" "m2" "p2" "timed_out"
      , sampleMember "C3" "m3" "p3" "seated"
      ]
    council_votes :=
      [ { member_id := "C1"
        , round := 1
        , vote := "demonstrated"
        , rationale := "r1"
        } ]
  }

def votedAndTimedOutState : ArbitrationState :=
  { activeDeliberationState with case := votedAndTimedOutCase }

/--
Reaching the vote threshold before the round ends does not close the case.

The sample policy requires two votes.  The sample state already contains one
`demonstrated` vote.  The second `demonstrated` vote establishes the eventual
resolution, but the engine now keeps deliberation open until the final seated
member votes.
-/
theorem second_demonstrated_vote_keeps_deliberation_open :
    stateStatus afterSecondDemonstratedVote = "active" ∧
      statePhase afterSecondDemonstratedVote = "deliberation" ∧
      stateResolution afterSecondDemonstratedVote = "" := by
  native_decide

/--
Completing the round closes the case with the already-determined resolution.

Once the remaining seated member votes, the round is complete.  The existing
two-vote `demonstrated` majority then closes the case.
-/
theorem third_demonstrated_vote_closes_the_case :
    stateStatus afterThirdDemonstratedVote = "closed" ∧
      statePhase afterThirdDemonstratedVote = "closed" ∧
      stateResolution afterThirdDemonstratedVote = "demonstrated" := by
  native_decide

/--
Removing a seated member below the threshold does not close deliberation early.

The engine still seeks the votes of the remaining seated members.  It keeps the
case active in `deliberation`, with no resolution yet recorded.
-/
theorem removal_that_breaks_the_threshold_keeps_deliberation_open :
    stateStatus afterRemovalMakesThresholdImpossible = "active" ∧
      statePhase afterRemovalMakesThresholdImpossible = "deliberation" ∧
      stateResolution afterRemovalMakesThresholdImpossible = "" := by
  native_decide

/--
The engine rejects removal of a member who already voted in the current round.

This guard preserves the meaning of round completion.  Once a current-round
vote exists for a seated member, that member cannot be removed until the round
ends.
-/
theorem removal_of_current_round_voter_is_rejected :
    initErrorMessage afterRemovingCurrentRoundVoter =
      "cannot remove council member after current-round vote: C1" := by
  native_decide

/--
A complete round with no decision advances deliberation to the next round.

The sample policy requires three votes.  After the third vote in the sample
round, neither side reaches three.  The current round is complete, but a
decision is still possible in a later round.  The engine should therefore keep
the case active, keep the phase as `deliberation`, clear no resolution, and
increment the round counter.
-/
theorem full_split_round_advances_deliberation :
    stateStatus afterFullSplitRound = "active" ∧
      statePhase afterFullSplitRound = "deliberation" ∧
      stateRound afterFullSplitRound = 2 ∧
      stateResolution afterFullSplitRound = "" := by
  native_decide

/--
Opportunity selection skips members who already voted and members who are not
seated.

This theorem states the meaning of the selection logic for the next
deliberation opportunity.  The engine should choose the remaining seated member
who has not yet voted in the current round.
-/
theorem nextOpportunity_skips_voted_and_nonseated_members :
    stateNextOpportunityId (nextOpportunity votedAndTimedOutState) =
      "deliberation:1:C3" := by
  native_decide
