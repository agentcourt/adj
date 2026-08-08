import Proofs.DecisionConfinement
import Proofs.OpportunitySelection

/--
If `availableOpportunities` ends with a strictly lower-priority opportunity and
the state has no passed opportunities, then any request from a different role
against that current opportunity is rejected with `WRONG_ROLE`.

The proof plan composes two existing boundaries.  First,
`currentOpenOpportunity_of_available_append_last_if_no_passes` proves that the
lower-priority trailing opportunity is current.  Then
`applyDecision_wrong_role_of_current_opportunity_returns_wrong_role` applies at
the public decision boundary.  The result is the generic confinement fact that
matters for the overall architecture: opportunity ordering and role ownership
work together, so another role cannot bypass the selected current move.
-/
theorem applyDecision_wrong_role_when_append_last_target_is_current
    (req : OpportunityRequest)
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (requestedRole : String)
    (decision : DecisionSpec)
    (havailable : availableOpportunities req = actions ++ [target])
    (hpasses : req.state.passed_opportunities = [])
    (hlower : ∀ a, a ∈ actions -> target.priority < a.priority)
    (hrole : normalizePartyToken requestedRole ≠ normalizePartyToken target.role) :
    let applyReq : ApplyDecisionRequest :=
      { state := req.state
      , state_version := req.state.state_version
      , opportunity_id := target.opportunity_id
      , role := requestedRole
      , decision := decision
      , roles := req.roles
      , max_steps_per_turn := req.max_steps_per_turn
      }
    applyDecisionErrorCode (applyDecision applyReq) = "WRONG_ROLE" := by
  have hcurrent :
      currentOpenOpportunity? req = some target :=
    currentOpenOpportunity_of_available_append_last_if_no_passes
      req actions target havailable hpasses hlower
  exact
    applyDecision_wrong_role_of_current_opportunity_returns_wrong_role
      req.state
      req.roles
      req.max_steps_per_turn
      target
      requestedRole
      decision
      hcurrent
      hrole

/-
This theorem is the first generic confinement result that actually spans the
two relevant layers.  The selector alone only says which opportunity is
current.  The decision boundary alone only says what happens if the current
opportunity belongs to someone else.  Together they state the important system
property: once formal priority makes an opportunity current, another role
cannot seize that move.
-/

/--
If `availableOpportunities` ends with a strictly lower-priority opportunity and
the state has no passed opportunities, then a valid tool decision from that
opportunity's role produces exactly the confined executable action for that
opportunity.

The proof plan is the positive companion to the theorem above.  First,
`currentOpenOpportunity_of_available_append_last_if_no_passes` proves that the
trailing lower-priority opportunity is current.  Then
`applyDecision_tool_success_exact_action` applies at the public decision
boundary.  The conclusion is the full positive confinement statement: once an
opportunity is current, the matching role can act only through the exact tool
and defaulted payload that the current opportunity permits.
-/
theorem applyDecision_tool_success_when_append_last_target_is_current
    (req : OpportunityRequest)
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (decision : DecisionSpec)
    (havailable : availableOpportunities req = actions ++ [target])
    (hpasses : req.state.passed_opportunities = [])
    (hlower : ∀ a, a ∈ actions -> target.priority < a.priority)
    (htool : decision.kind.trimAscii.toString = "tool")
    (hempty :
      (match decision.tool_name with | some name => name.trimAscii.toString | none => "").isEmpty = false)
    (hallowed :
      target.allowed_tools.contains
        (match decision.tool_name with | some name => name.trimAscii.toString | none => "") = true)
    (hviol :
      firstRequiredPayloadViolation?
        (applyPayloadDefaults (decision.payload.getD Lean.Json.null) target.constraints)
        target.constraints = none) :
    let applyReq : ApplyDecisionRequest :=
      { state := req.state
      , state_version := req.state.state_version
      , opportunity_id := target.opportunity_id
      , role := target.role
      , decision := decision
      , roles := req.roles
      , max_steps_per_turn := req.max_steps_per_turn
      }
    applyDecision applyReq =
      Except.ok
        { result_kind := "execute_tool"
        , state := none
        , action := some
            { action_type := (match decision.tool_name with | some name => name.trimAscii.toString | none => "")
            , actor_role := target.role
            , payload := applyPayloadDefaults (decision.payload.getD Lean.Json.null) target.constraints } } := by
  have hcurrent :
      currentOpenOpportunity? req = some target :=
    currentOpenOpportunity_of_available_append_last_if_no_passes
      req actions target havailable hpasses hlower
  exact
    applyDecision_tool_success_exact_action
      { state := req.state
      , state_version := req.state.state_version
      , opportunity_id := target.opportunity_id
      , role := target.role
      , decision := decision
      , roles := req.roles
      , max_steps_per_turn := req.max_steps_per_turn
      }
      target
      hcurrent
      rfl
      rfl
      rfl
      htool
      hempty
      hallowed
      hviol

/-
This theorem is the positive counterpart to the wrong-role result above.  It
shows the exact shape of the authorized path once a lower-priority opportunity
is current: the matching role does not get arbitrary power.  Lean still emits
only the exact tool action that the current opportunity allows, and it leaves
authoritative state unchanged until `step` runs.
-/

/--
If `availableOpportunities` ends with a strictly lower-priority current
opportunity, then the same tool decision succeeds for that opportunity's role
and is rejected for any different role.

The proof plan is to combine the two generic theorems above.  The positive
theorem gives the exact successful `execute_tool` result for the owning role.
The negative theorem gives `WRONG_ROLE` for a different role on the same
opportunity id.  The value of this theorem is not the conjunction itself.  It
is the architectural statement: once formal priority chooses the current
opportunity, role ownership determines whether a proposed tool decision can
cross the public boundary at all.
-/
theorem applyDecision_current_role_partition_when_append_last_target_is_current
    (req : OpportunityRequest)
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (decision : DecisionSpec)
    (otherRole : String)
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
        target.constraints = none) :
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
    applyDecision applyReqGood =
      Except.ok
        { result_kind := "execute_tool"
        , state := none
        , action := some
            { action_type := (match decision.tool_name with | some name => name.trimAscii.toString | none => "")
            , actor_role := target.role
            , payload := applyPayloadDefaults (decision.payload.getD Lean.Json.null) target.constraints } } ∧
    applyDecisionErrorCode (applyDecision applyReqBad) = "WRONG_ROLE" := by
  constructor
  · exact
      applyDecision_tool_success_when_append_last_target_is_current
        req actions target decision havailable hpasses hlower htool hempty hallowed hviol
  · exact
      applyDecision_wrong_role_when_append_last_target_is_current
        req actions target otherRole decision havailable hpasses hlower hother

/-
This theorem is the clean generic statement of role partition at the public
boundary.  The same opportunity and the same decision payload admit exactly two
interesting outcomes: the owning role gets the confined action, and any other
role gets `WRONG_ROLE`.  That is a strong statement of the system's design,
and it builds directly on the smaller selector and confinement theorems rather
than recomputing the opportunity machinery.
-/
