import Proofs.ExecutionConfinement
import Proofs.OpportunityConfinement

/--
If formal priority selects a current opportunity, then the engine partitions the
roles at that boundary and, when the emitted action closes the case, seals the
successor state against later decisions.

The proof plan composes three existing generic theorems.  First,
`applyDecision_current_role_partition_when_append_last_target_is_current`
states the present-tense partition: the owning role gets the exact executable
action, and any different role gets `WRONG_ROLE`.  Second,
`tool_execution_closing_case_blocks_followup_decisions` states the future-tense
closure property: if that exact emitted action steps to a closed case, later
decisions on the successor state fail with `NO_CURRENT_OPPORTUNITY`.  The new
theorem joins those two pieces under one shaped `availableOpportunities` list.
-/
theorem append_last_current_opportunity_partitions_roles_and_seals_after_closure
    (req : OpportunityRequest)
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (decision : DecisionSpec)
    (otherRole : String)
    (closedState : CourtState)
    (followOpportunityId : String)
    (followRole : String)
    (followDecision : DecisionSpec)
    (followRoles : List RolePolicy)
    (followMaxSteps : Nat)
    (havailable : availableOpportunities req = actions ++ [target])
    (hpasses : req.state.passed_opportunities = [])
    (hlower : ∀ a, a ∈ actions -> target.priority < a.priority)
    (hother : normalizePartyToken otherRole ≠ normalizePartyToken target.role)
    (htool : decision.kind.trimAscii.toString = "tool")
    (hempty :
      (match decision.tool_name with | some name => name.trimAscii.toString | none => "").isEmpty = false)
    (hallowed :
      target.allowed_tools.contains
        (match decision.tool_name with | some name => name.trimAscii.toString | none => "") = true)
    (hviol :
      firstRequiredPayloadViolation?
        (applyPayloadDefaults (decision.payload.getD Lean.Json.null) target.constraints)
        target.constraints = none)
    (hstep :
      step req.state
        { action_type := (match decision.tool_name with | some name => name.trimAscii.toString | none => "")
        , actor_role := target.role
        , payload := applyPayloadDefaults (decision.payload.getD Lean.Json.null) target.constraints } =
          Except.ok closedState)
    (hclosed : closedState.case.status = "closed") :
    let applyReqGood : ApplyDecisionRequest :=
      { state := req.state
      , state_version := req.state.state_version
      , opportunity_id := target.opportunity_id
      , role := target.role
      , decision := decision
      , roles := req.roles
      , max_steps_per_turn := req.max_steps_per_turn
      }
    let applyReqBad : ApplyDecisionRequest :=
      { state := req.state
      , state_version := req.state.state_version
      , opportunity_id := target.opportunity_id
      , role := otherRole
      , decision := decision
      , roles := req.roles
      , max_steps_per_turn := req.max_steps_per_turn
      }
    let emittedAction : CourtAction :=
      { action_type := (match decision.tool_name with | some name => name.trimAscii.toString | none => "")
      , actor_role := target.role
      , payload := applyPayloadDefaults (decision.payload.getD Lean.Json.null) target.constraints }
    applyDecision applyReqGood =
      Except.ok
        { result_kind := "execute_tool"
        , state := none
        , action := some emittedAction } ∧
    applyDecisionErrorCode (applyDecision applyReqBad) = "WRONG_ROLE" ∧
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
      (applyDecision_current_role_partition_when_append_last_target_is_current
        req actions target decision otherRole havailable hpasses hlower hother htool hempty hallowed hviol).1
  constructor
  · exact
      (applyDecision_current_role_partition_when_append_last_target_is_current
        req actions target decision otherRole havailable hpasses hlower hother htool hempty hallowed hviol).2
  · exact
      (tool_execution_closing_case_blocks_followup_decisions
        { state := req.state
        , state_version := req.state.state_version
        , opportunity_id := target.opportunity_id
        , role := target.role
        , decision := decision
        , roles := req.roles
        , max_steps_per_turn := req.max_steps_per_turn
        }
        target
        closedState
        followOpportunityId
        followRole
        followDecision
        followRoles
        followMaxSteps
        (currentOpenOpportunity_of_available_append_last_if_no_passes
          req actions target havailable hpasses hlower)
        rfl
        rfl
        rfl
        htool
        hempty
        hallowed
        hviol
        hstep
        hclosed).2

/-
This theorem is the best generic statement in the current proof suite of the
overall architecture.  It keeps three distinct layers in one result:

1. formal priority chooses the current opportunity;
2. the public decision boundary partitions roles into one authorized actor and
   everyone else;
3. if the authorized action closes the case, the successor state is sealed
   against later decisions.

The theorem is still objective.  It says nothing about legal correctness.  It
says that, once the engine's own objective preconditions are met, the resulting
state machine behaves exactly as the architecture claims.
-/
