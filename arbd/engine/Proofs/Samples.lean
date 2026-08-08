import Main

open Lean

namespace ArbdProofs

def sampleMember (memberId model persona status : String) : CouncilMember :=
  { member_id := memberId
  , model := model
  , persona_filename := persona
  , status := status
  }

def mixedStatusCouncil : List CouncilMember :=
  [ sampleMember "C1" "m1" "p1" "timed_out"
  , sampleMember "C2" "m2" "p2" "seated"
  , sampleMember "C3" "m3" "p3" "excused"
  ]

def samplePolicy : ArbitrationPolicy :=
  { council_size := 3
  , judgment_standard := "Answer with one integer from 0 through 100."
  }

def invalidBlankStandardPolicy : ArbitrationPolicy :=
  { samplePolicy with judgment_standard := "" }

def baseCase : ArbitrationCase :=
  { case_id := "arbd-1"
  , caption := "Example degree arbitration"
  }

def baseState : ArbitrationState :=
  { forum_name := "Test Forum"
  , case := baseCase
  , policy := samplePolicy
  , state_version := 10
  }

def initRequest : InitializeCaseRequest :=
  { state := baseState
  , question := "What percentage of work Y is novel in view of work X?"
  , council_members := mixedStatusCouncil
  }

def initializedState : ArbitrationState :=
  match initializeCase initRequest with
  | .ok state => state
  | .error _ => default

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

def stateQuestion : Except String ArbitrationState → String
  | .ok state => state.case.question
  | .error _ => ""

def stateAnswerCount : Except String ArbitrationState → Nat
  | .ok state => state.case.council_answers.length
  | .error _ => 0

def stateAnswerFor (memberId : String) : Except String ArbitrationState → Nat
  | .ok state =>
      match state.case.council_answers.find? (fun answer => answer.member_id = memberId) with
      | some answer => answer.answer
      | none => 0
  | .error _ => 0

def stateSubmittedEvidenceCount : Except String ArbitrationState → Nat
  | .ok state => state.case.submitted_evidence.length
  | .error _ => 0

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

def submittedEvidenceAction (role : String) : CourtAction :=
  { action_type := "submit_evidence"
  , actor_role := role
  , payload := Json.mkObj
      [ ("evidence_id", Json.str "ev_abc123_submitted-evidence-01-plaintiff")
      , ("title", Json.str "Source text")
      , ("source_url", Json.str "https://example.test/source")
      , ("mime_type", Json.str "text/plain")
      , ("retrieval_timestamp", Json.str "2026-05-20T00:00:00Z")
      , ("relevance", Json.str "Shows the source material.")
      , ("sha256", Json.str "abc123")
      , ("size_bytes", toJson (12 : Nat))
      ]
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

def councilAnswerAction (memberId : String) (answer : Nat) (rationale : String) : CourtAction :=
  { action_type := "submit_council_answer"
  , actor_role := "council"
  , payload := Json.mkObj
      [ ("member_id", Json.str memberId)
      , ("answer", toJson answer)
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

def failOpportunityAction
    (opportunityId role phase reason message model : String) : CourtAction :=
  { action_type := "fail_opportunity"
  , actor_role := "system"
  , payload := Json.mkObj
      [ ("opportunity_id", Json.str opportunityId)
      , ("role", Json.str role)
      , ("phase", Json.str phase)
      , ("reason", Json.str reason)
      , ("message", Json.str message)
      , ("model", Json.str model)
      ]
  }

end ArbdProofs
