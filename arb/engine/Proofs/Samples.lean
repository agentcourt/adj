import Main

open Lean

namespace ArbProofs

/-
This file defines the sample states for the first `agentarbitration` proofs.

The first proofs should state what the present engine already means.  Three
parts of the engine matter most at this stage.

First, initialization must turn a draft state plus a policy into a live case
with a seated council and a clean starting point.

Second, the merits sequence is intentionally linear.  The engine is supposed to
move through openings, arguments, rebuttals, surrebuttals, closings, and then
deliberation without any side branches beyond the optional pass actions in
rebuttal and surrebuttal.

Third, deliberation must close when the configured vote threshold is reached,
close only after every seated member has voted in the current round, and use
the completed round to determine whether the threshold was reached or became
impossible.

Small explicit states are enough for those claims.  They keep the later proof
files readable, and they make it clear which fact each theorem is supposed to
capture.
-/

def sampleMember (memberId model persona status : String) : CouncilMember :=
  { member_id := memberId
  , model := model
  , persona_filename := persona
  , status := status
  }

/-
The initialization sample deliberately begins with mixed statuses.

That lets the initialization proofs say something meaningful.  If the input
council were already fully seated, a later theorem about reseating would not
distinguish real normalization from accidental agreement with the input.
-/
def mixedStatusCouncil : List CouncilMember :=
  [ sampleMember "C1" "m1" "p1" "timed_out"
  , sampleMember "C2" "m2" "p2" "seated"
  , sampleMember "C3" "m3" "p3" "excused"
  ]

def seatedCouncil : List CouncilMember :=
  mixedStatusCouncil.map (fun member => { member with status := "seated" })

/-
The base policy is intentionally small.

Three council seats and a two-vote threshold are enough to exhibit the normal
decision path.  A second policy raises the threshold to three votes so that the
proofs can also show a full round with no decision and a removal that makes a
decision impossible without ending the round early.
-/
def samplePolicy : ArbitrationPolicy :=
  { council_size := 3
  , evidence_standard := "Preponderance of the evidence."
  , required_votes_for_decision := 2
  , max_deliberation_rounds := 2
  }

def supermajorityPolicy : ArbitrationPolicy :=
  { samplePolicy with required_votes_for_decision := 3 }

def invalidThresholdPolicy : ArbitrationPolicy :=
  { samplePolicy with required_votes_for_decision := 4 }

def nonStrictMajorityPolicy : ArbitrationPolicy :=
  { samplePolicy with council_size := 4, required_votes_for_decision := 2 }

def baseCase : ArbitrationCase :=
  { case_id := "arb-1"
  , caption := "Example arbitration"
  }

def baseState : ArbitrationState :=
  { forum_name := "Test Forum"
  , case := baseCase
  , policy := samplePolicy
  , state_version := 10
  }

def initRequest : InitializeCaseRequest :=
  { state := baseState
  , proposition := "The claimant demonstrated the proposition."
  , council_members := mixedStatusCouncil
  }

/-
`initializedState` is the successful result of the standard sample request.

The separate theorems in `InitializeCase.lean` still prove that this request
succeeds and that it has the expected effects.  This definition exists so that
the later flow and deliberation proofs can build on a common live case without
repeating the same match expression.
-/
def initializedState : ArbitrationState :=
  match initializeCase initRequest with
  | .ok state => state
  | .error _ => default

/-
The helper functions below deliberately extract only the fields that the
theorems care about.

`ArbitrationState` does not derive `DecidableEq`, and the later theorems do not
need whole-state equality anyway.  The claims are narrower: an error string is
exactly this string, the phase is exactly this phase, the resolution is exactly
this resolution, and so on.
-/
def initErrorMessage : Except String ArbitrationState → String
  | .error msg => msg
  | .ok _ => ""

def policyErrorMessage : Except String Unit → String
  | .error msg => msg
  | .ok _ => ""

def statePhase : Except String ArbitrationState → String
  | .ok state => state.case.phase
  | .error _ => ""

def stateStatus : Except String ArbitrationState → String
  | .ok state => state.case.status
  | .error _ => ""

def stateResolution : Except String ArbitrationState → String
  | .ok state => state.case.resolution
  | .error _ => ""

def stateVersion : Except String ArbitrationState → Nat
  | .ok state => state.state_version
  | .error _ => 0

def stateRound : Except String ArbitrationState → Nat
  | .ok state => state.case.deliberation_round
  | .error _ => 0

def stateCouncilSize : Except String ArbitrationState → Nat
  | .ok state => state.case.council_members.length
  | .error _ => 0

def stateAllCouncilStatusesAre (status : String) : Except String ArbitrationState → Bool
  | .ok state =>
      state.case.council_members.foldl
        (fun acc member => acc && trimString member.status = status)
        true
  | .error _ => false

def stateProposition : Except String ArbitrationState → String
  | .ok state => state.case.proposition
  | .error _ => ""

def stateNextRole (result : NextOpportunityOk) : String :=
  match result.opportunity with
  | some opportunity => opportunity.role
  | none => ""

def stateNextPhase (result : NextOpportunityOk) : String :=
  match result.opportunity with
  | some opportunity => opportunity.phase
  | none => ""

def stateNextTool (result : NextOpportunityOk) : String :=
  match result.opportunity with
  | some opportunity =>
      match opportunity.allowed_tools with
      | tool :: _ => tool
      | [] => ""
  | none => ""

def stateNextOpportunityId (result : NextOpportunityOk) : String :=
  match result.opportunity with
  | some opportunity => opportunity.opportunity_id
  | none => ""

/-
The action constructors keep the proof files focused on procedure rather than
JSON syntax.

The proofs are about which opportunity comes next and what state change a
particular filing produces.  They are not about whether a JSON object was typed
correctly in the proof file.
-/
def textPayload (text : String) : Json :=
  Json.mkObj [("text", Json.str text)]

def meritsPayload (text : String) : Json :=
  Json.mkObj
    [ ("text", Json.str text)
    , ("offered_evidence", Json.arr #[])
    , ("technical_reports", Json.arr #[])
    ]

def openingAction (role text : String) : CourtAction :=
  { action_type := "record_opening_statement"
  , actor_role := role
  , payload := textPayload text
  }

def argumentAction (role text : String) : CourtAction :=
  { action_type := "submit_argument"
  , actor_role := role
  , payload := meritsPayload text
  }

def rebuttalAction (text : String) : CourtAction :=
  { action_type := "submit_rebuttal"
  , actor_role := "plaintiff"
  , payload := meritsPayload text
  }

def surrebuttalAction (text : String) : CourtAction :=
  { action_type := "submit_surrebuttal"
  , actor_role := "defendant"
  , payload := meritsPayload text
  }

def closingAction (role text : String) : CourtAction :=
  { action_type := "deliver_closing_statement"
  , actor_role := role
  , payload := textPayload text
  }

def passAction (role : String) : CourtAction :=
  { action_type := "pass_phase_opportunity"
  , actor_role := role
  , payload := Json.null
  }

def councilVoteAction (memberId vote rationale : String) : CourtAction :=
  { action_type := "submit_council_vote"
  , actor_role := "council"
  , payload := Json.mkObj
      [ ("member_id", Json.str memberId)
      , ("vote", Json.str vote)
      , ("rationale", Json.str rationale)
      ]
  }

def removeCouncilMemberAction (memberId status : String) : CourtAction :=
  { action_type := "remove_council_member"
  , actor_role := "system"
  , payload := Json.mkObj
      [ ("member_id", Json.str memberId)
      , ("status", Json.str status)
      ]
  }

end ArbProofs
