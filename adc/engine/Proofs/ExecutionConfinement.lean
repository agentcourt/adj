import Proofs.DecisionConfinement

namespace ADCProofs.ExecutionConfinement

open ADCProofs.DecisionConfinement

/--
If a valid current tool decision emits an action whose execution closes the
case, later decisions on the successor state fail with
`NO_CURRENT_OPPORTUNITY`.

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
        (applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) (decisionConstraints opportunity req.decision))
        (decisionConstraints opportunity req.decision) = none)
    (hstep :
      step req.state
        { action_type := (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "")
        , actor_role := opportunity.role
        , payload := applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) (decisionConstraints opportunity req.decision) } =
          Except.ok closedState)
    (hclosed : closedState.case.status = "closed") :
    let emittedAction : CourtAction :=
      { action_type := (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "")
      , actor_role := opportunity.role
      , payload := applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) (decisionConstraints opportunity req.decision) }
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
  · rw [hstep]
    apply applyDecision_closed_case_returns_no_current_opportunity
    · exact hclosed
    · rfl

end ADCProofs.ExecutionConfinement
