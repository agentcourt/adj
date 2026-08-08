import Proofs.Samples
import Proofs.MeritsFlow

open ArbdProofs

def activeDeliberationCase : ArbitrationCase :=
  { baseCase with
    status := "active"
    phase := "deliberation"
    council_members := mixedStatusCouncil.map (fun member => { member with status := "seated" })
    question := initRequest.question
  }

def activeDeliberationState : ArbitrationState :=
  { baseState with case := activeDeliberationCase }

def oneAnswerState : ArbitrationState :=
  { activeDeliberationState with
    case := { activeDeliberationCase with
      council_answers :=
        [ { member_id := "C1"
          , round := 1
          , answer := 72
          , rationale := "first answer"
          } ]
    } }

def afterSecondAnswer : Except String ArbitrationState :=
  step
    { state := oneAnswerState
    , action := councilAnswerAction "C2" 55 "second answer"
    }

def afterThirdAnswer : Except String ArbitrationState :=
  step
    { state :=
        match afterSecondAnswer with
        | .ok state => state
        | .error _ => default
    , action := councilAnswerAction "C3" 18 "third answer"
    }

def afterRemovingUnansweredMember : Except String ArbitrationState :=
  step
    { state := oneAnswerState
    , action := removeCouncilMemberAction "C3" "timed_out"
    }

def twoAnswersState : ArbitrationState :=
  match afterSecondAnswer with
  | .ok state => state
  | .error _ => default

def afterRemovingLastUnansweredMember : Except String ArbitrationState :=
  step
    { state := twoAnswersState
    , action := removeCouncilMemberAction "C3" "timed_out"
    }

def afterRemovingAnsweredMember : Except String ArbitrationState :=
  step
    { state := oneAnswerState
    , action := removeCouncilMemberAction "C1" "timed_out"
    }

def afterOutOfRangeAnswer : Except String ArbitrationState :=
  step
    { state := activeDeliberationState
    , action := councilAnswerAction "C1" 101 "too high"
    }

def votedAndTimedOutCase : ArbitrationCase :=
  { activeDeliberationCase with
    council_members :=
      [ sampleMember "C1" "m1" "p1" "seated"
      , sampleMember "C2" "m2" "p2" "timed_out"
      , sampleMember "C3" "m3" "p3" "seated"
      ]
    council_answers :=
      [ { member_id := "C1"
        , round := 1
        , answer := 72
        , rationale := "r1"
        } ]
  }

def votedAndTimedOutState : ArbitrationState :=
  { activeDeliberationState with case := votedAndTimedOutCase }

theorem second_answer_keeps_deliberation_open :
    stateStatus afterSecondAnswer = "active" ∧
      statePhase afterSecondAnswer = "deliberation" ∧
      stateAnswerCount afterSecondAnswer = 2 := by
  native_decide

theorem third_answer_closes_the_case :
    stateStatus afterThirdAnswer = "closed" ∧
      statePhase afterThirdAnswer = "closed" ∧
      stateAnswerFor "C3" afterThirdAnswer = 18 := by
  native_decide

theorem removing_one_unanswered_member_keeps_deliberation_open :
    stateStatus afterRemovingUnansweredMember = "active" ∧
      statePhase afterRemovingUnansweredMember = "deliberation" := by
  native_decide

theorem removing_last_unanswered_member_closes_the_case :
    stateStatus afterRemovingLastUnansweredMember = "closed" ∧
      statePhase afterRemovingLastUnansweredMember = "closed" ∧
      stateAnswerCount afterRemovingLastUnansweredMember = 2 := by
  native_decide

theorem removal_of_current_round_answerer_is_rejected :
    initErrorMessage afterRemovingAnsweredMember =
      "cannot remove council member after current-round answer: C1" := by
  native_decide

theorem out_of_range_answer_is_rejected :
    initErrorMessage afterOutOfRangeAnswer =
      "council answer must be between 0 and 100" := by
  native_decide

theorem nextOpportunity_skips_answered_and_nonseated_members :
    stateNextOpportunityId (nextOpportunity votedAndTimedOutState) =
      "deliberation:1:C3" := by
  native_decide
