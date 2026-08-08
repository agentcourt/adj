import Proofs.OrchestrationCore

def applyDecisionErrorCode (r : Except StepErr ApplyDecisionOk) : String :=
  match r with
  | .error err => err.code
  | .ok _ => ""

/--
If the case is closed and the request uses the current state version,
`applyDecision` rejects the request with `NO_CURRENT_OPPORTUNITY`.

The proof plan is short and generic.  `applyDecision` first computes the
current open opportunity for the request-shaped `OpportunityRequest`.  The
already proved theorem `currentOpenOpportunity_none_when_case_closed` says that
this selector returns `none` whenever the case status is `closed`.  Once that
rewrite is in place, the `applyDecision` match reduces directly to the public
`NO_CURRENT_OPPORTUNITY` error shape.
-/
theorem applyDecision_closed_case_returns_no_current_opportunity
    (req : ApplyDecisionRequest)
    (hclosed : req.state.case.status = "closed")
    (hversion : req.state.state_version = req.state_version) :
    applyDecisionErrorCode (applyDecision req) = "NO_CURRENT_OPPORTUNITY" := by
  let baseReq : OpportunityRequest :=
    { state := req.state
    , roles := req.roles
    , max_steps_per_turn := req.max_steps_per_turn
    }
  have hcurrent : currentOpenOpportunity? baseReq = none :=
    currentOpenOpportunity_none_when_case_closed baseReq hclosed
  simp [baseReq] at hcurrent
  have happly :
      applyDecision req =
        Except.error
          (mkStepErr
            "no current opportunity"
            "NO_CURRENT_OPPORTUNITY"
            Lean.Json.null
            false
            "No current opportunity is open for decision.") := by
    unfold applyDecision
    simp [hversion, hcurrent]
    rfl
  rw [happly]
  rfl

/-
This theorem fills a real gap in the boundary story.  The suite already proved
that closed cases have no available opportunities and make `nextOpportunity`
terminal.  The public decision boundary should say the same thing from the
other side once the request is not already stale: after closure, every later
decision attempt fails before any role, tool, or payload logic matters.

The proof is short because the API is well shaped.  That is a feature, not a
problem.  A longer proof here would likely mean the boundary itself had become
harder to reason about than it should be.
-/

/--
A pass decision at a fixed opportunity records a pass and returns no action.

The proof plan unfolds `applyDecisionAtOpportunity` and rewrites the pass
branch directly.  Once the decision kind is `pass` and the opportunity allows
passing, the helper returns the `pass_recorded` result with an updated state and
no action.  This is the first half of the decision-confinement boundary:
accepted passes do not fabricate executable actions.
-/
theorem applyDecisionAtOpportunity_pass_success_shape
    (state : CourtState)
    (opportunity : OpportunitySpec)
    (decision : DecisionSpec)
    (hpass : decision.kind.trimAscii.toString = "pass")
    (hmay : opportunity.may_pass = true) :
    applyDecisionAtOpportunity state opportunity decision =
      Except.ok
        { result_kind := "pass_recorded"
        , state := some (recordOpportunityPassFor state opportunity)
        , action := none } := by
  simp [applyDecisionAtOpportunity, hpass, hmay]
  rfl

/-
This theorem states the exact successful pass behavior.  It is worth keeping
because the helper is the point where an accepted decision becomes either a
state update or an executable action.  A pass must stay on the state-update
side only.
-/

/--
If a request names the current open opportunity and the request is a valid
pass, `applyDecision` returns exactly the pass-recording result and no action.

The proof plan matches the tool-side public theorem below.  The outer guards in
`applyDecision` are discharged by the current-opportunity, state-version,
opportunity-id, and role hypotheses.  The remaining goal is exactly the helper
theorem `applyDecisionAtOpportunity_pass_success_shape`.
-/
theorem applyDecision_pass_success_shape
    (req : ApplyDecisionRequest)
    (opportunity : OpportunitySpec)
    (hcurrent :
      currentOpenOpportunity?
        { state := req.state
        , roles := req.roles
        , max_steps_per_turn := req.max_steps_per_turn } = some opportunity)
    (hversion : req.state.state_version = req.state_version)
    (hid : opportunity.opportunity_id = req.opportunity_id)
    (hrole : normalizePartyToken req.role = normalizePartyToken opportunity.role)
    (hpass : req.decision.kind.trimAscii.toString = "pass")
    (hmay : opportunity.may_pass = true) :
    applyDecision req =
      Except.ok
        { result_kind := "pass_recorded"
        , state := some (recordOpportunityPassFor req.state opportunity)
        , action := none } := by
  simpa [applyDecision, hcurrent, hversion, hid, hrole] using
    applyDecisionAtOpportunity_pass_success_shape
      req.state opportunity req.decision hpass hmay

/-
This theorem lifts the exact helper pass result to the public boundary.  It is
the companion to the tool-side theorem below.  Together they state the two
shapes that a successful public decision may take: state-only pass recording,
or a single confined executable action.
-/

/--
A valid tool decision at a fixed opportunity returns exactly the confined action
for that opportunity.

The proof plan is again direct.  Unfold `applyDecisionAtOpportunity`, rewrite
the decision kind to `tool`, rewrite the non-empty tool name and allowed-tool
check, and rewrite the payload-constraint check to `none`.  The helper then
reduces to a single `execute_tool` result whose action type, actor role, and
payload are determined entirely by the opportunity and the defaulted payload.
-/
theorem applyDecisionAtOpportunity_tool_success_exact_action
    (state : CourtState)
    (opportunity : OpportunitySpec)
    (decision : DecisionSpec)
    (htool : decision.kind.trimAscii.toString = "tool")
    (hempty :
      (match decision.tool_name with | some name => name.trimAscii.toString | none => "").isEmpty = false)
    (hallowed :
      opportunity.allowed_tools.contains
        (match decision.tool_name with | some name => name.trimAscii.toString | none => "") = true)
    (hviol :
      firstRequiredPayloadViolation?
        (applyPayloadDefaults (decision.payload.getD Lean.Json.null) opportunity.constraints)
        opportunity.constraints = none) :
    applyDecisionAtOpportunity state opportunity decision =
      Except.ok
        { result_kind := "execute_tool"
        , state := none
        , action := some
            { action_type := (match decision.tool_name with | some name => name.trimAscii.toString | none => "")
            , actor_role := opportunity.role
            , payload := applyPayloadDefaults (decision.payload.getD Lean.Json.null) opportunity.constraints } } := by
  cases hname : decision.tool_name with
  | none =>
      simp [hname] at hempty
      cases hempty
  | some name =>
      simp [hname] at hempty hallowed
      have hnotpass : decision.kind.trimAscii.toString ≠ "pass" := by
        intro hpass
        simp [htool] at hpass
      simp [applyDecisionAtOpportunity, htool, hname, hempty, hallowed, hviol]
      rfl

/-
This is the core positive confinement theorem.  Once the objective guards are
satisfied, Lean emits exactly one executable action, and that action is
confined to the current opportunity's role and allowed tool set.  The helper
does not invent a different role, tool, or payload shape.
-/

/--
Any successful tool decision at a fixed opportunity returns an action confined
to that opportunity's role and allowed tool set.

The proof plan combines the exact-action theorem with the action fields
themselves.  This is the compact public statement of the helper's
confinement behavior.
-/
theorem applyDecisionAtOpportunity_tool_success_confined
    (state : CourtState)
    (opportunity : OpportunitySpec)
    (decision : DecisionSpec)
    (hkind : decision.kind.trimAscii.toString = "tool")
    (hempty :
      (match decision.tool_name with | some name => name.trimAscii.toString | none => "").isEmpty = false)
    (hallowed :
      opportunity.allowed_tools.contains
        (match decision.tool_name with | some name => name.trimAscii.toString | none => "") = true)
    (hviol :
      firstRequiredPayloadViolation?
        (applyPayloadDefaults (decision.payload.getD Lean.Json.null) opportunity.constraints)
        opportunity.constraints = none) :
    let result := applyDecisionAtOpportunity state opportunity decision
    match result with
    | Except.ok ok =>
        ok.action = some
          { action_type := (match decision.tool_name with | some name => name.trimAscii.toString | none => "")
          , actor_role := opportunity.role
          , payload := applyPayloadDefaults (decision.payload.getD Lean.Json.null) opportunity.constraints }
    | Except.error _ => False := by
  rw [applyDecisionAtOpportunity_tool_success_exact_action state opportunity decision hkind hempty hallowed hviol]

/-
This theorem is the short headline for the helper boundary.  A valid tool
decision does not merely succeed.  It succeeds by producing the exact confined
action that the current opportunity permits.
-/

/--
If a request names the current open opportunity and satisfies the helper's
objective guards, `applyDecision` returns exactly the confined executable
action for that opportunity.

The proof plan is short.  `applyDecision` checks four objective outer guards:
state version, current open opportunity, opportunity id, and role.  The
hypotheses discharge those guards directly.  The remaining goal is exactly the
helper theorem `applyDecisionAtOpportunity_tool_success_exact_action`.
-/
theorem applyDecision_tool_success_exact_action
    (req : ApplyDecisionRequest)
    (opportunity : OpportunitySpec)
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
        opportunity.constraints = none) :
    applyDecision req =
      Except.ok
        { result_kind := "execute_tool"
        , state := none
        , action := some
            { action_type := (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "")
            , actor_role := opportunity.role
            , payload := applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) opportunity.constraints } } := by
  simpa [applyDecision, hcurrent, hversion, hid, hrole] using
    applyDecisionAtOpportunity_tool_success_exact_action
      req.state opportunity req.decision htool hempty hallowed hviol

/-
This theorem lifts the exact helper result to the public `applyDecision`
boundary.  It is the right statement for the current stage.  It keeps all
semantic questions out of Lean and proves the formal boundary instead: once the
objective preconditions are met, the public API returns exactly the confined
action that the current opportunity permits.
-/

/--
Under the same objective preconditions, the executable action returned by
`applyDecision` is confined to the current opportunity's role and allowed tool
set.

The proof plan is immediate from the exact-action theorem.  Rewrite
`applyDecision req` to the exact success result, then read the `actor_role` and
`action_type` fields from that result.
-/
theorem applyDecision_tool_success_confined
    (req : ApplyDecisionRequest)
    (opportunity : OpportunitySpec)
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
        opportunity.constraints = none) :
    let result := applyDecision req
    match result with
    | Except.ok ok =>
        ok.action = some
          { action_type := (match req.decision.tool_name with | some name => name.trimAscii.toString | none => "")
          , actor_role := opportunity.role
          , payload := applyPayloadDefaults (req.decision.payload.getD Lean.Json.null) opportunity.constraints }
    | Except.error _ => False := by
  rw [applyDecision_tool_success_exact_action req opportunity hcurrent hversion hid hrole htool hempty hallowed hviol]

/-
This is the short public corollary.  The formal result is not merely that a
request can succeed.  It succeeds by returning the exact action fixed by the
current opportunity and the request payload defaults.
-/

/--
Under the same objective preconditions, a successful public tool decision does
not update state inside `applyDecision`.

The proof plan is immediate from `applyDecision_tool_success_exact_action`.  In
the tool-success case, `applyDecision` validates the decision and returns the
executable action, leaving state mutation for the later `step`.
-/
theorem applyDecision_tool_success_has_no_state_update
    (req : ApplyDecisionRequest)
    (opportunity : OpportunitySpec)
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
        opportunity.constraints = none) :
    let result := applyDecision req
    match result with
    | Except.ok ok => ok.state = none
    | Except.error _ => False := by
  rw [applyDecision_tool_success_exact_action req opportunity hcurrent hversion hid hrole htool hempty hallowed hviol]

/-
This is the state-mutation half of the public boundary.  Successful tool
validation does not mutate authoritative case state.  It emits an executable
action for `step` to handle later.
-/

/--
Under the same objective preconditions, a successful public pass decision
returns no executable action.

The proof plan is immediate from `applyDecision_pass_success_shape`.  In the
pass-success case, `applyDecision` updates only the pass bookkeeping state and
returns no action for later execution.
-/
theorem applyDecision_pass_success_has_no_action
    (req : ApplyDecisionRequest)
    (opportunity : OpportunitySpec)
    (hcurrent :
      currentOpenOpportunity?
        { state := req.state
        , roles := req.roles
        , max_steps_per_turn := req.max_steps_per_turn } = some opportunity)
    (hversion : req.state.state_version = req.state_version)
    (hid : opportunity.opportunity_id = req.opportunity_id)
    (hrole : normalizePartyToken req.role = normalizePartyToken opportunity.role)
    (hpass : req.decision.kind.trimAscii.toString = "pass")
    (hmay : opportunity.may_pass = true) :
    let result := applyDecision req
    match result with
    | Except.ok ok => ok.action = none
    | Except.error _ => False := by
  rw [applyDecision_pass_success_shape req opportunity hcurrent hversion hid hrole hpass hmay]

/-
This is the action-side half of the public boundary for pass decisions.  A
valid pass updates bookkeeping only.  It does not fabricate an executable
action.
-/

/--
If a request names the current opportunity and the current opportunity belongs
to another role, `applyDecision` rejects the request with `WRONG_ROLE`.

The proof plan unfolds `applyDecision` only far enough to reach the role check.
The stale-state, no-current-opportunity, and stale-opportunity branches all
collapse under the hypotheses.  The remaining branch is the explicit
`WRONG_ROLE` throw.  That branch does not depend on the content of the
requested decision.  It depends only on ownership of the current opportunity.
-/
theorem applyDecision_wrong_role_of_current_opportunity_returns_wrong_role
    (state : CourtState)
    (roles : List RolePolicy)
    (maxSteps : Nat)
    (opportunity : OpportunitySpec)
    (requestedRole : String)
    (decision : DecisionSpec)
    (hcurrent :
      currentOpenOpportunity?
        { state := state
        , roles := roles
        , max_steps_per_turn := maxSteps } = some opportunity)
    (hrole : normalizePartyToken requestedRole ≠ normalizePartyToken opportunity.role) :
    let req : ApplyDecisionRequest :=
      { state := state
      , state_version := state.state_version
      , opportunity_id := opportunity.opportunity_id
      , role := requestedRole
      , decision := decision
      , roles := roles
      , max_steps_per_turn := maxSteps
      }
    applyDecisionErrorCode (applyDecision req) = "WRONG_ROLE" := by
  unfold applyDecisionErrorCode applyDecision
  simp [hcurrent, hrole]
  have hthrow :
      (do
        let y : PUnit ←
          throw
            (mkStepErr
              (toString "opportunity " ++ toString opportunity.opportunity_id ++ toString " belongs to " ++
                    toString opportunity.role ++ toString ", not " ++ toString requestedRole)
              "WRONG_ROLE" Lean.Json.null false
              (toString "This opportunity belongs to " ++ toString opportunity.role ++
                toString ".  Only that role may act on it."))
        applyDecisionAtOpportunity state opportunity decision) =
      Except.error
        (mkStepErr
          (toString "opportunity " ++ toString opportunity.opportunity_id ++ toString " belongs to " ++
                toString opportunity.role ++ toString ", not " ++ toString requestedRole)
          "WRONG_ROLE" Lean.Json.null false
          (toString "This opportunity belongs to " ++ toString opportunity.role ++
            toString ".  Only that role may act on it.")) := by
    rfl
  rw [hthrow]
  simp [mkStepErr]

/-
This theorem lifts the wrong-role guard to the public API in general form.  It
is a stronger statement than the earlier concrete examples because it says the
decision content is irrelevant.  Once the current opportunity belongs to a
different role, the engine rejects the request before it examines the proposed
act.
-/

def defectiveJurisdictionCaseForDecision : CaseState :=
  { (default : CaseState) with
    case_id := "case-jurisdiction-flow",
    filed_on := "2026-01-01",
    status := "filed",
    phase := "pleadings",
    decision_traces := [{ action := "file_complaint", outcome := "filed", citations := ["FRCP 3"] }],
    jurisdictional_allegations := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis", Lean.Json.str "diversity")
      , ("jurisdictional_statement", Lean.Json.str "The parties are citizens of different States and the amount in controversy exceeds $75,000.")
      , ("plaintiff_citizenship", Lean.Json.str "Texas")
      , ("defendant_citizenship", Lean.Json.str "New York")
      , ("amount_in_controversy", Lean.Json.str "")
      ]
  }

def defectiveJurisdictionStateForDecision : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    court_name := "Test Court",
    case := defectiveJurisdictionCaseForDecision
  }

def jurisdictionDismissRolesForDecision : List RolePolicy :=
  [{ role := "judge", allowed_tools := ["dismiss_for_lack_of_subject_matter_jurisdiction"] }]

def jurisdictionDismissReqForDecision : OpportunityRequest :=
  { state := defectiveJurisdictionStateForDecision
  , roles := jurisdictionDismissRolesForDecision
  , max_steps_per_turn := 3
  }

def jurisdictionDismissOpportunityForDecision : OpportunitySpec :=
  match currentOpenOpportunity? jurisdictionDismissReqForDecision with
  | some opportunity => opportunity
  | none => default

def jurisdictionDismissDecisionForDecision : DecisionSpec :=
  { kind := "tool"
  , tool_name := some "dismiss_for_lack_of_subject_matter_jurisdiction"
  , payload := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis_rejected", Lean.Json.str "diversity")
      , ("reasoning", Lean.Json.str "The complaint does not adequately allege the amount in controversy.")
      ]
  }

def jurisdictionDismissApplyReqForDecision : ApplyDecisionRequest :=
  { state := defectiveJurisdictionStateForDecision
  , state_version := defectiveJurisdictionStateForDecision.state_version
  , opportunity_id := jurisdictionDismissOpportunityForDecision.opportunity_id
  , role := "judge"
  , decision := jurisdictionDismissDecisionForDecision
  , roles := jurisdictionDismissRolesForDecision
  , max_steps_per_turn := 3
  }

def jurisdictionDismissResultLooksRight : Except StepErr ApplyDecisionOk → Bool
  | .error _ => false
  | .ok ok =>
      ok.result_kind = "execute_tool" &&
      ok.state.isNone &&
      match ok.action with
      | none => false
      | some action =>
          action.action_type = "dismiss_for_lack_of_subject_matter_jurisdiction" &&
          action.actor_role = "judge" &&
          jsonStringFieldD action.payload "jurisdiction_basis_rejected" "" = "diversity" &&
          jsonStringFieldD action.payload "reasoning" "" =
            "The complaint does not adequately allege the amount in controversy."

/--
For the concrete defective diversity complaint, `applyDecision` emits the
expected subject-matter-jurisdiction dismissal action.

The proof plan is concrete.  Evaluate the public decision request built from
the current opportunity and check the returned result fields that matter:
result kind, absence of state update, action type, actor role, and the two
payload fields.  This theorem is the concrete instance of the generic
confinement result above.
-/
theorem applyDecision_defective_jurisdiction_emits_expected_action :
    jurisdictionDismissResultLooksRight (applyDecision jurisdictionDismissApplyReqForDecision) = true := by
  native_decide

/-
This theorem is the concrete public-boundary instance of the general
confinement result.  It shows the emitted action fields for a real
jurisdiction-screening case without re-proving the general boundary theorem.
-/

/--
For the same defective diversity complaint, the accepted dismissal decision and
the resulting `step` close the case and make `nextOpportunity` terminal.

The proof plan rewrites `applyDecision` to the exact dismissal action from the
previous theorem.  The rest of the statement is a concrete computation of
`step` on that action and `nextOpportunity` on the resulting closed state.
-/
theorem applyDecision_defective_jurisdiction_then_step_stops :
    let result := applyDecision jurisdictionDismissApplyReqForDecision
    (match result with
      | .ok ok =>
          match ok.action with
          | some action =>
              match step defectiveJurisdictionStateForDecision action with
              | .ok s' =>
                  s'.case.status = "closed" &&
                  (nextOpportunity
                    { state := s'
                    , roles := jurisdictionDismissRolesForDecision
                    , max_steps_per_turn := 3 }).terminal
              | .error _ => false
          | none => false
      | .error _ => false) = true := by
  native_decide

/-
This is the end-to-end theorem for the current pass.  It starts at the public
decision boundary, not at a local helper, and follows the accepted dismissal
through `step` to a closed case with no later opportunity.
-/
