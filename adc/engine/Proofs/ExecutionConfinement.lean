import Proofs.DecisionConfinement

/--
If a valid current tool decision emits an action whose execution closes the
case, then the engine seals itself against later decisions on the successor
state.

The proof plan composes the existing public-boundary theorems instead of
recomputing the whole flow.  First, `applyDecision_tool_success_exact_action`
proves that the original request emits the exact confined action fixed by the
current opportunity.  Second, rewrite the `step` match with the supplied
closed-state execution hypothesis.  The remaining goal is exactly the generic
closed-case theorem `applyDecision_closed_case_returns_no_current_opportunity`
applied to the follow-up request on the successor state.
-/
theorem tool_execution_closing_case_blocks_followup_decisions
    (req : ApplyDecisionRequest)
    (opportunity : OpportunitySpec)
    (closedState : CourtState)
    (followOpportunityId : String)
    (followRole : String)
    (followDecision : DecisionSpec)
    (followRoles : List RolePolicy)
    (followMaxSteps : Nat)
    (hcurrent :
      currentOpenOpportunity?
        { state := req.state
        , roles := req.roles
        , max_steps_per_turn := req.max_steps_per_turn } = some opportunity)
    (hversion : req.state.state_version = req.state_version)
    (hid : opportunity.opportunity_id = req.opportunity_id)
    (hrole : normalizePartyToken req.role = normalizePartyToken opportunity.role)
    (htool : req.decision.kind.trimAscii.toString = "tool")
    (hempty :
      (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "").isEmpty = false)
    (hallowed :
      opportunity.allowed_tools.contains
        (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "") = true)
    (hviol :
      firstRequiredPayloadViolation?
        (applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) opportunity.constraints)
        opportunity.constraints = none)
    (hstep :
      step req.state
        { action_type := (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "")
        , actor_role := opportunity.role
        , payload := applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) opportunity.constraints } =
          Except.ok closedState)
    (hclosed : closedState.case.status = "closed") :
    let emittedAction : CourtAction :=
      { action_type := (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "")
      , actor_role := opportunity.role
      , payload := applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) opportunity.constraints }
    applyDecision req =
      Except.ok
        { result_kind := "execute_tool"
        , state := none
        , action := some emittedAction } ∧
    (match step req.state emittedAction with
      | .ok s' =>
          let followReq : ApplyDecisionRequest :=
            { state := s'
            , state_version := s'.state_version
            , opportunity_id := followOpportunityId
            , role := followRole
            , decision := followDecision
            , roles := followRoles
            , max_steps_per_turn := followMaxSteps
            }
          applyDecisionErrorCode (applyDecision followReq) = "NO_CURRENT_OPPORTUNITY"
      | .error _ => False) := by
  constructor
  · exact
      applyDecision_tool_success_exact_action
        req opportunity hcurrent hversion hid hrole htool hempty hallowed hviol
  · simp [hstep]
    apply applyDecision_closed_case_returns_no_current_opportunity
    · exact hclosed
    · rfl

/-
This theorem is the generic closure-seal result the proof suite needed.  The
earlier concrete jurisdiction theorem already showed the pattern in one filed
case.  This theorem states it at the public boundary without mentioning any
particular rule.  Once a valid current tool decision emits an action and that
action steps to a closed case, later decisions cannot cross the boundary on the
successor state.

The theorem still stays on the objective side.  It does not say that the
underlying legal decision was correct.  It says that the formal engine confines
what follows from a closing execution.  That is one of the central claims of
the whole approach.
-/
