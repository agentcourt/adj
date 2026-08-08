import Main

def filedComplaintCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "filed",
    phase := "none",
    decision_traces := [{ action := "file_complaint", outcome := "filed", citations := ["FRCP 3"] }]
  }

def chargeConferenceCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-2",
    filed_on := "2026-01-01",
    status := "trial",
    trial_mode := "jury",
    phase := "charge_conference"
  }

def pretrialRule56Case : CaseState :=
  { (default : CaseState) with
    case_id := "case-4",
    filed_on := "2026-01-01",
    status := "pretrial",
    phase := "discovery",
    decision_traces := [
      { action := "file_complaint", outcome := "filed", citations := ["FRCP 3"] },
      { action := "file_answer", outcome := "filed", citations := ["FRCP 8(b)"] }
    ],
    docket := [
      { title := "Interrogatory Responses", description := "defendant: served" },
      { title := "Responses to Requests for Production", description := "defendant: served" },
      { title := "Responses to Requests for Admission", description := "defendant: served" }
    ]
  }

def stateOf (c : CaseState) (version : Nat := 0) (passed : List String := []) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    court_name := "Test Court",
    case := c,
    state_version := version,
    passed_opportunities := passed
  }

def optionalRule12Req : OpportunityRequest :=
  { state := stateOf filedComplaintCase
  , roles := [{ role := "defendant", allowed_tools := ["file_rule12_motion"] }]
  , max_steps_per_turn := 3
  }

def optionalRule12Opportunity : OpportunitySpec :=
  match currentOpenOpportunity? optionalRule12Req with
  | some opportunity => opportunity
  | none => default

def chargeConferenceReq : OpportunityRequest :=
  { state := stateOf chargeConferenceCase
  , roles := [{ role := "plaintiff", allowed_tools := ["propose_jury_instruction"] }]
  , max_steps_per_turn := 3
  }

def optionalRule56Req : OpportunityRequest :=
  { state := stateOf pretrialRule56Case
  , roles := [{ role := "defendant", allowed_tools := ["file_rule56_motion"] }]
  , max_steps_per_turn := 3
  }

def chargeConferenceOpportunity : OpportunitySpec :=
  match currentOpenOpportunity? chargeConferenceReq with
  | some opportunity => opportunity
  | none => default

def optionalRule56Opportunity : OpportunitySpec :=
  match currentOpenOpportunity? optionalRule56Req with
  | some opportunity => opportunity
  | none => default

def closedReq : ApplyDecisionRequest :=
  { state := stateOf { (default : CaseState) with case_id := "case-3", status := "closed", phase := "post_verdict" }
  , state_version := 0
  , opportunity_id := "o1"
  , role := "plaintiff"
  , decision := { kind := "pass", reason := some "nothing open" }
  , roles := [{ role := "plaintiff", allowed_tools := ["file_complaint"] }]
  , max_steps_per_turn := 3
  }

def stalePassReq : ApplyDecisionRequest :=
  { state := optionalRule12Req.state
  , state_version := 1
  , opportunity_id := optionalRule12Opportunity.opportunity_id
  , role := "defendant"
  , decision := { kind := "pass", reason := some "decline" }
  , roles := optionalRule12Req.roles
  , max_steps_per_turn := optionalRule12Req.max_steps_per_turn
  }

def wrongRolePassReq : ApplyDecisionRequest :=
  { state := optionalRule12Req.state
  , state_version := optionalRule12Req.state.state_version
  , opportunity_id := optionalRule12Opportunity.opportunity_id
  , role := "plaintiff"
  , decision := { kind := "pass", reason := some "not my turn" }
  , roles := optionalRule12Req.roles
  , max_steps_per_turn := optionalRule12Req.max_steps_per_turn
  }

def validPassReq : ApplyDecisionRequest :=
  { state := optionalRule12Req.state
  , state_version := optionalRule12Req.state.state_version
  , opportunity_id := optionalRule12Opportunity.opportunity_id
  , role := "defendant"
  , decision := { kind := "pass", reason := some "decline" }
  , roles := optionalRule12Req.roles
  , max_steps_per_turn := optionalRule12Req.max_steps_per_turn
  }

def missingToolNameReq : ApplyDecisionRequest :=
  { state := optionalRule12Req.state
  , state_version := optionalRule12Req.state.state_version
  , opportunity_id := optionalRule12Opportunity.opportunity_id
  , role := "defendant"
  , decision := { kind := "tool", payload := some Lean.Json.null }
  , roles := optionalRule12Req.roles
  , max_steps_per_turn := optionalRule12Req.max_steps_per_turn
  }

def disallowedToolReq : ApplyDecisionRequest :=
  { state := optionalRule12Req.state
  , state_version := optionalRule12Req.state.state_version
  , opportunity_id := optionalRule12Opportunity.opportunity_id
  , role := "defendant"
  , decision := { kind := "tool", tool_name := some "file_answer", payload := some Lean.Json.null }
  , roles := optionalRule12Req.roles
  , max_steps_per_turn := optionalRule12Req.max_steps_per_turn
  }

def proposeInstructionDecision : DecisionSpec :=
  { kind := "tool"
  , tool_name := some "propose_jury_instruction"
  , payload := some (Lean.Json.mkObj [("instruction_text", Lean.Json.str "Misrepresentation requires proof by a preponderance of the evidence.")])
  }

def proposeInstructionReq : ApplyDecisionRequest :=
  { state := chargeConferenceReq.state
  , state_version := chargeConferenceReq.state.state_version
  , opportunity_id := chargeConferenceOpportunity.opportunity_id
  , role := "plaintiff"
  , decision := proposeInstructionDecision
  , roles := chargeConferenceReq.roles
  , max_steps_per_turn := chargeConferenceReq.max_steps_per_turn
  }

def validRule56PassReq : ApplyDecisionRequest :=
  { state := optionalRule56Req.state
  , state_version := optionalRule56Req.state.state_version
  , opportunity_id := optionalRule56Opportunity.opportunity_id
  , role := "defendant"
  , decision := { kind := "pass", reason := some "decline summary judgment" }
  , roles := optionalRule56Req.roles
  , max_steps_per_turn := optionalRule56Req.max_steps_per_turn
  }

def conflictingInstructionDecision : DecisionSpec :=
  { kind := "tool"
  , tool_name := some "propose_jury_instruction"
  , payload := some (Lean.Json.mkObj
      [ ("instruction_id", Lean.Json.str "BAD-ID")
      , ("instruction_text", Lean.Json.str "Bad instruction id payload.") ])
  }

def conflictingInstructionReq : ApplyDecisionRequest :=
  { state := chargeConferenceReq.state
  , state_version := chargeConferenceReq.state.state_version
  , opportunity_id := chargeConferenceOpportunity.opportunity_id
  , role := "plaintiff"
  , decision := conflictingInstructionDecision
  , roles := chargeConferenceReq.roles
  , max_steps_per_turn := chargeConferenceReq.max_steps_per_turn
  }

def applyErrorCode (r : Except StepErr ApplyDecisionOk) : String :=
  match r with
  | .error err => err.code
  | .ok _ => ""

def applyResultKind (r : Except StepErr ApplyDecisionOk) : String :=
  match r with
  | .ok ok => ok.result_kind
  | .error _ => ""

def applyStateVersion (r : Except StepErr ApplyDecisionOk) : Nat :=
  match r with
  | .ok ok =>
      match ok.state with
      | some s => s.state_version
      | none => 0
  | .error _ => 0

def applyPassedOpportunities (r : Except StepErr ApplyDecisionOk) : List String :=
  match r with
  | .ok ok =>
      match ok.state with
      | some s => s.passed_opportunities
      | none => []
  | .error _ => []

def applyRule56WindowClosedFor (r : Except StepErr ApplyDecisionOk) : List String :=
  match r with
  | .ok ok =>
      match ok.state with
      | some s => s.case.rule56_window_closed_for
      | none => []
  | .error _ => []

def applyRule56PassLeavesNoOpenOpportunity (r : Except StepErr ApplyDecisionOk) : Bool :=
  match r with
  | .ok ok =>
      match ok.state with
      | some s =>
          let req : OpportunityRequest :=
            { state := s
            , roles := optionalRule56Req.roles
            , max_steps_per_turn := optionalRule56Req.max_steps_per_turn
            }
          currentOpenOpportunity? req = none
      | none => false
  | .error _ => false

def applyHasState (r : Except StepErr ApplyDecisionOk) : Bool :=
  match r with
  | .ok ok => ok.state.isSome
  | .error _ => false

def applyHasAction (r : Except StepErr ApplyDecisionOk) : Bool :=
  match r with
  | .ok ok => ok.action.isSome
  | .error _ => false

def applyActionType (r : Except StepErr ApplyDecisionOk) : String :=
  match r with
  | .ok ok =>
      match ok.action with
      | some action => action.action_type
      | none => ""
  | .error _ => ""

def applyActorRole (r : Except StepErr ApplyDecisionOk) : String :=
  match r with
  | .ok ok =>
      match ok.action with
      | some action => action.actor_role
      | none => ""
  | .error _ => ""

def applyActionField (r : Except StepErr ApplyDecisionOk) (field : String) : String :=
  match r with
  | .ok ok =>
      match ok.action with
      | some action =>
          match action.payload.getObjVal? field with
          | .ok value =>
              match value.getStr? with
              | .ok s => s
              | .error _ => ""
          | .error _ => ""
      | none => ""
  | .error _ => ""

theorem optionalRule12Opportunity_is_defendant_passable :
    optionalRule12Opportunity.role = "defendant" ∧
      optionalRule12Opportunity.allowed_tools = ["file_rule12_motion"] ∧
      optionalRule12Opportunity.may_pass = true := by
  native_decide

theorem optionalRule56Opportunity_is_defendant_passable :
    optionalRule56Opportunity.role = "defendant" ∧
      optionalRule56Opportunity.allowed_tools = ["file_rule56_motion"] ∧
      optionalRule56Opportunity.may_pass = true := by
  native_decide

theorem applyDecision_stale_state_version_returns_stale_code :
    applyErrorCode (applyDecision stalePassReq) = "STALE_OPPORTUNITY" := by
  native_decide

theorem applyDecision_without_current_opportunity_returns_no_current_code :
    applyErrorCode (applyDecision closedReq) = "NO_CURRENT_OPPORTUNITY" := by
  native_decide

theorem applyDecision_wrong_role_returns_wrong_role_code :
    applyErrorCode (applyDecision wrongRolePassReq) = "WRONG_ROLE" := by
  native_decide

theorem applyDecision_valid_pass_records_state :
    applyResultKind (applyDecision validPassReq) = "pass_recorded" ∧
      applyHasState (applyDecision validPassReq) = true ∧
      applyHasAction (applyDecision validPassReq) = false ∧
      applyStateVersion (applyDecision validPassReq) = 1 ∧
      applyPassedOpportunities (applyDecision validPassReq) = [optionalRule12Opportunity.opportunity_id] := by
  native_decide

theorem applyDecision_rule56_pass_closes_window :
    applyResultKind (applyDecision validRule56PassReq) = "pass_recorded" ∧
      applyRule56WindowClosedFor (applyDecision validRule56PassReq) = ["defendant"] ∧
      applyRule56PassLeavesNoOpenOpportunity (applyDecision validRule56PassReq) = true := by
  native_decide

theorem applyDecision_missing_tool_name_returns_missing_tool_name_code :
    applyErrorCode (applyDecision missingToolNameReq) = "MISSING_TOOL_NAME" := by
  native_decide

theorem applyDecision_disallowed_tool_returns_tool_not_allowed_code :
    applyErrorCode (applyDecision disallowedToolReq) = "TOOL_NOT_ALLOWED" := by
  native_decide

theorem applyDecision_tool_applies_fixed_payload_defaults :
    applyResultKind (applyDecision proposeInstructionReq) = "execute_tool" ∧
      applyHasState (applyDecision proposeInstructionReq) = false ∧
      applyHasAction (applyDecision proposeInstructionReq) = true ∧
      applyActionType (applyDecision proposeInstructionReq) = "propose_jury_instruction" ∧
      applyActorRole (applyDecision proposeInstructionReq) = "plaintiff" ∧
      applyActionField (applyDecision proposeInstructionReq) "party" = "plaintiff" ∧
      applyActionField (applyDecision proposeInstructionReq) "instruction_id" = "PI-1" := by
  native_decide

theorem applyDecision_conflicting_required_payload_returns_constraint_code :
    applyErrorCode (applyDecision conflictingInstructionReq) = "PAYLOAD_CONSTRAINT_VIOLATION" := by
  native_decide
