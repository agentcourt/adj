import Main

structure ReplayInitializeCaseRequest where
  complaint_summary : String
  filed_by : String := "plaintiff"
  jury_demanded_on : String := ""
  jurisdictional_allegations : Option Lean.Json := none
  attachments : List ComplaintAttachmentSeed := []
  deriving Inhabited

structure ReplayInitializeRequest where
  state : CourtState
  initialize_case : Option ReplayInitializeCaseRequest := none
  deriving Inhabited

structure ReplayApplyDecisionTransition where
  state_version : Nat
  opportunity_id : String
  role : String
  decision : DecisionSpec
  roles : List RolePolicy
  max_steps_per_turn : Nat := 3
  deriving Inhabited

inductive ReplayTransition where
  | step (action : CourtAction)
  | applyDecision (transition : ReplayApplyDecisionTransition)

def ReplayInitializeCaseRequest.toInitializeCaseRequest
    (state : CourtState)
    (req : ReplayInitializeCaseRequest) :
    InitializeCaseRequest :=
  { state := state
  , complaint_summary := req.complaint_summary
  , filed_by := req.filed_by
  , jury_demanded_on := req.jury_demanded_on
  , jurisdictional_allegations := req.jurisdictional_allegations
  , attachments := req.attachments
  }

def replayInitial (req : ReplayInitializeRequest) : Except String CourtState :=
  match req.initialize_case with
  | none => .ok req.state
  | some init => initializeCase (init.toInitializeCaseRequest req.state)

def ReplayApplyDecisionTransition.toRequest
    (state : CourtState)
    (transition : ReplayApplyDecisionTransition) :
    ApplyDecisionRequest :=
  { state := state
  , state_version := transition.state_version
  , opportunity_id := transition.opportunity_id
  , role := transition.role
  , decision := transition.decision
  , roles := transition.roles
  , max_steps_per_turn := transition.max_steps_per_turn
  }

def replayApplyDecisionTransition
    (state : CourtState)
    (transition : ReplayApplyDecisionTransition) :
    Except String CourtState :=
  match applyDecision (transition.toRequest state) with
  | .error err => .error err.error
  | .ok resp =>
      if resp.result_kind = "pass_recorded" then
        match resp.state with
        | some next => .ok next
        | none => .error "apply_decision returned empty state"
      else
        .error "apply_decision returned unsupported result_kind"

def replayTransition
    (state : CourtState)
    (transition : ReplayTransition) :
    Except String CourtState :=
  match transition with
  | .step action => step state action
  | .applyDecision decision => replayApplyDecisionTransition state decision

inductive ReplayReachableFrom (start : CourtState) : CourtState → Prop where
  | refl : ReplayReachableFrom start start
  | transition (current next : CourtState) (transition : ReplayTransition)
      (hcurrent : ReplayReachableFrom start current)
      (htransition : replayTransition current transition = .ok next) :
      ReplayReachableFrom start next
