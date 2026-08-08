import Proofs.DecisionConfinement
import Proofs.JurisdictionDismissal
import Proofs.OrchestrationCore


def dismissalToolForFlow : List String :=
  ["dismiss_for_lack_of_subject_matter_jurisdiction"]

def defectiveFiledCaseForFlow : CaseState :=
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

def defectiveFiledStateForFlow : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    court_name := "Test Court",
    case := defectiveFiledCaseForFlow
  }

def defectiveFiledRolesForFlow : List RolePolicy :=
  [ { role := "defendant", allowed_tools := ["file_rule12_motion", "file_answer"] }
  , { role := "judge", allowed_tools := dismissalToolForFlow }
  ]

def defectiveFiledReqForFlow : OpportunityRequest :=
  { state := defectiveFiledStateForFlow
  , roles := defectiveFiledRolesForFlow
  , max_steps_per_turn := 3
  }

def defectiveFiledOpportunityForFlow : OpportunitySpec :=
  match currentOpenOpportunity? defectiveFiledReqForFlow with
  | some opportunity => opportunity
  | none => default

def defectiveFiledDecisionForFlow : DecisionSpec :=
  { kind := "tool"
  , tool_name := some "dismiss_for_lack_of_subject_matter_jurisdiction"
  , payload := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis_rejected", Lean.Json.str "diversity")
      , ("reasoning", Lean.Json.str "The complaint does not adequately allege the amount in controversy.")
      ]
  }

def defectiveFiledApplyReqForFlow : ApplyDecisionRequest :=
  { state := defectiveFiledStateForFlow
  , state_version := defectiveFiledStateForFlow.state_version
  , opportunity_id := defectiveFiledOpportunityForFlow.opportunity_id
  , role := "judge"
  , decision := defectiveFiledDecisionForFlow
  , roles := defectiveFiledRolesForFlow
  , max_steps_per_turn := 3
  }

def defectiveFiledRolesWithoutJudgeDismissalForFlow : List RolePolicy :=
  [{ role := "defendant", allowed_tools := ["file_rule12_motion", "file_answer"] }]

def defectiveFiledReqWithoutJudgeDismissalForFlow : OpportunityRequest :=
  { state := defectiveFiledStateForFlow
  , roles := defectiveFiledRolesWithoutJudgeDismissalForFlow
  , max_steps_per_turn := 3
  }

def defectiveFiledWrongRoleRule12DecisionForFlow : DecisionSpec :=
  { kind := "tool"
  , tool_name := some "file_rule12_motion"
  , payload := some <| Lean.Json.mkObj
      [ ("ground", Lean.Json.str "failure_to_state_a_claim")
      , ("summary", Lean.Json.str "The complaint does not plausibly state a claim.")
      ]
  }

def defectiveFiledWrongRoleRule12ReqForFlow : ApplyDecisionRequest :=
  { state := defectiveFiledStateForFlow
  , state_version := defectiveFiledStateForFlow.state_version
  , opportunity_id := defectiveFiledOpportunityForFlow.opportunity_id
  , role := "defendant"
  , decision := defectiveFiledWrongRoleRule12DecisionForFlow
  , roles := defectiveFiledRolesForFlow
  , max_steps_per_turn := 3
  }

def applyDecisionErrorCodeForFlow (r : Except StepErr ApplyDecisionOk) : String :=
  match r with
  | .error err => err.code
  | .ok _ => ""

def defectiveFiledDismissReasonForFlow : String :=
  "The complaint does not adequately allege the amount in controversy."

def defectiveFiledDismissActionForFlow : CourtAction :=
  dismissForLackOfSubjectMatterJurisdictionAction defectiveFiledDismissReasonForFlow

def defectiveFiledDismissedStateForFlow : CourtState :=
  match step defectiveFiledStateForFlow defectiveFiledDismissActionForFlow with
  | .ok s' => s'
  | .error _ => default

def defectiveFiledPostDismissRule12ReqForFlow : ApplyDecisionRequest :=
  { state := defectiveFiledDismissedStateForFlow
  , state_version := defectiveFiledDismissedStateForFlow.state_version
  , opportunity_id := defectiveFiledOpportunityForFlow.opportunity_id
  , role := "defendant"
  , decision := defectiveFiledWrongRoleRule12DecisionForFlow
  , roles := defectiveFiledRolesForFlow
  , max_steps_per_turn := 3
  }

/--
In a filed case with a facially defective diversity allegation, the
subject-matter-jurisdiction dismissal opportunity preempts the defendant's Rule
12 and answer opportunities.

The proof plan is concrete because the point is the full orchestration choice,
not a local helper.  Build a filed case that already has a complaint, gives the
defendant both ordinary pleading tools, and gives the judge the dismissal tool.
Then compute `nextOpportunity` and check the public facts that matter: the
response is not terminal, the selected role is the judge, the allowed tool set
is the dismissal tool, and the priority is `10`.  That shows Lean chooses the
mandatory jurisdiction screen before party-controlled pleading moves.
-/
theorem nextOpportunity_defective_filed_case_selects_judge_dismissal :
    let resp := nextOpportunity defectiveFiledReqForFlow
    resp.terminal = false ∧
    resp.opportunity.map (fun o => (o.role, o.allowed_tools, o.priority)) =
      some ("judge", dismissalToolForFlow, 10) := by
  native_decide

/-
This theorem is the first realistic jurisdiction-flow result.  The case is not
judge-only.  The defendant has genuine pleading choices, but the engine still
puts the court's jurisdiction duty first.  That is a concrete demonstration of
the overall approach: agent choices persist, but formal procedure controls the
order in which those choices may matter.
-/

/--
In that same filed case, the defendant's Rule 12 opportunity remains available
in the open set even though the judge's dismissal opportunity is current.

The proof plan checks `openOpportunities` directly.  The theorem looks for two
facts at once: one open opportunity for the judge with the dismissal tool, and
one open opportunity for the defendant with the Rule 12 tool.  This is the
important complement to the selection theorem above.  The engine does not erase
party choices.  It orders them.
-/
theorem openOpportunities_defective_filed_case_keep_rule12_while_selecting_judge :
    let opportunities := openOpportunities defectiveFiledReqForFlow
    opportunities.any (fun o => o.role = "judge" && o.allowed_tools = dismissalToolForFlow) = true ∧
    opportunities.any (fun o => o.role = "defendant" && o.allowed_tools = ["file_rule12_motion"]) = true := by
  native_decide

/-
This is the strongest theorem in the file so far.  It states the key ordering
property of the whole approach in concrete form.  The party's pleading option
persists in the open opportunity set.  Lean still chooses the judge's
jurisdiction duty as the current move.
-/

/--
If the same defective filed case omits the judge's dismissal tool, the engine
falls back to the defendant's Rule 12 opportunity.

The proof plan is the contrast case for the previous theorem.  Keep the same
defective complaint and the same defendant pleading tools, but remove the
judge's dismissal tool from the role policy.  Then compute `nextOpportunity`
and check that the selected role is now the defendant with the Rule 12 tool.
This shows that the earlier preemption result comes from formal procedure and
tool policy, not from a hard-coded preference for judges.
-/
theorem nextOpportunity_defective_filed_case_without_judge_dismissal_selects_rule12 :
    let resp := nextOpportunity defectiveFiledReqWithoutJudgeDismissalForFlow
    resp.terminal = false ∧
    resp.opportunity.map (fun o => (o.role, o.allowed_tools)) =
      some ("defendant", ["file_rule12_motion"]) := by
  native_decide

/-
This contrast theorem makes the selection story sharper.  The engine does not
always hand the case to the judge.  It does so here because the jurisdiction
screen is available and outranks the ordinary pleading path.  Remove that tool,
and the defendant's Rule 12 opportunity becomes current.
-/

/--
In the defective filed case, the public `currentOpenOpportunity?` boundary
agrees with the already proved `nextOpportunity` selection and returns the
judge's jurisdiction-dismissal opportunity.

The proof plan reuses two earlier results rather than recomputing the
selection.  `nextOpportunity_defective_filed_case_selects_judge_dismissal`
already proves the selected tuple at the `nextOpportunity` boundary.
`nextOpportunity_opportunity_eq_currentOpenOpportunity` then identifies that
selected opportunity with `currentOpenOpportunity?`.
-/
theorem currentOpenOpportunity_defective_filed_case_selects_judge_dismissal :
    (currentOpenOpportunity? defectiveFiledReqForFlow).map
      (fun o => (o.role, o.allowed_tools, o.priority)) =
      some ("judge", dismissalToolForFlow, 10) := by
  simpa [nextOpportunity_opportunity_eq_currentOpenOpportunity defectiveFiledReqForFlow] using
    nextOpportunity_defective_filed_case_selects_judge_dismissal.2

/-
This theorem turns the concrete `nextOpportunity` computation into a fact about
the public selector boundary itself.  That is the right shape for later
decision-boundary proofs, because `applyDecision` is keyed to
`currentOpenOpportunity?`, not to `nextOpportunity` directly.
-/

/--
The named defective-filed opportunity is the current open opportunity.

The proof plan unfolds the helper definition once and rules out the `none`
branch by contradiction.  If `currentOpenOpportunity? defectiveFiledReqForFlow`
were `none`, the previous theorem would say its mapped tuple is `some (...)`,
which is impossible.
-/
theorem currentOpenOpportunity_defective_filed_case_eq_named_opportunity :
    currentOpenOpportunity? defectiveFiledReqForFlow = some defectiveFiledOpportunityForFlow := by
  unfold defectiveFiledOpportunityForFlow
  cases hcurrent : currentOpenOpportunity? defectiveFiledReqForFlow with
  | none =>
      have hselected := currentOpenOpportunity_defective_filed_case_selects_judge_dismissal
      simp [hcurrent] at hselected
  | some opportunity =>
      simp

/-
This is a small but necessary bridge theorem.  The file uses a named
opportunity value in the later `ApplyDecisionRequest`.  To apply the generic
decision-boundary theorems, the file needs that named value to agree with the
actual current opportunity.
-/

/--
The named defective-filed opportunity has the selected judge-dismissal shape.

The proof plan rewrites the current-opportunity tuple theorem to the named
opportunity and then extracts the resulting equality from `Option.some.inj`.
That one tuple equality is more useful than separate ad hoc field facts because
later proofs can read the role, tool set, and priority from a single source.
-/
theorem defectiveFiledOpportunityForFlow_shape :
    (defectiveFiledOpportunityForFlow.role,
      defectiveFiledOpportunityForFlow.allowed_tools,
      defectiveFiledOpportunityForFlow.priority) =
      ("judge", dismissalToolForFlow, 10) := by
  have hshape :
      (some defectiveFiledOpportunityForFlow).map
        (fun o => (o.role, o.allowed_tools, o.priority)) =
        some ("judge", dismissalToolForFlow, 10) := by
    simpa [currentOpenOpportunity_defective_filed_case_eq_named_opportunity] using
      currentOpenOpportunity_defective_filed_case_selects_judge_dismissal
  exact Option.some.inj hshape

/-
This theorem replaces a few narrower field lemmas with one reusable tuple fact.
That makes the later proofs shorter and clearer.
-/

/--
In that same defective filed case, the defendant's Rule 12 opportunity remains
open but is not actionable while the judge's jurisdiction-dismissal opportunity
is current.

The proof plan keeps the two public facts together.  First, reuse the earlier
open-opportunity theorem to confirm that the defendant's Rule 12 path still
appears in the open set.  Then submit a defendant Rule 12 decision against the
current opportunity id, which belongs to the judge.  `applyDecision` must
reject that request with `WRONG_ROLE`.  This is the architecture in one step:
party options may remain open, but formal priority still determines who may act
now.
-/
theorem defective_filed_case_rule12_available_but_not_actionable :
    let opportunities := openOpportunities defectiveFiledReqForFlow
    opportunities.any (fun o => o.role = "defendant" && o.allowed_tools = ["file_rule12_motion"]) = true ∧
    applyDecisionErrorCodeForFlow (applyDecision defectiveFiledWrongRoleRule12ReqForFlow) = "WRONG_ROLE" := by
  constructor
  · exact (openOpportunities_defective_filed_case_keep_rule12_while_selecting_judge).2
  · simpa [defectiveFiledWrongRoleRule12ReqForFlow, applyDecisionErrorCodeForFlow] using
      applyDecision_wrong_role_of_current_opportunity_returns_wrong_role
        defectiveFiledStateForFlow
        defectiveFiledRolesForFlow
        3
        defectiveFiledOpportunityForFlow
        "defendant"
        defectiveFiledWrongRoleRule12DecisionForFlow
        currentOpenOpportunity_defective_filed_case_eq_named_opportunity
        (by
          rw [show defectiveFiledOpportunityForFlow.role = "judge" by
            exact congrArg Prod.fst defectiveFiledOpportunityForFlow_shape]
          native_decide)

/-
This theorem is the clearest jurisdiction-flow boundary result so far.  The
defendant's pleading option persists in the open set.  Lean still bars the
defendant from acting on it because the current move belongs to the judge.  The
system therefore preserves party choices without letting them bypass procedural
priority.
-/

/--
In the defective filed case, the matching judge decision emits one executable
subject-matter-jurisdiction dismissal action with the expected role and payload
fields.

The proof plan applies the generic public decision-boundary theorem
`applyDecision_tool_success_exact_action` and then reads the result fields.
The generic theorem already does the hard work: it proves that the public
decision boundary emits exactly one executable action for the current
opportunity.  This concrete theorem states the public facts that matter for the
flow theorem below: the result kind is `execute_tool`, the state is not updated
yet, the action type is the jurisdiction-dismissal tool, the actor role is the
judge, and the payload preserves the decisive jurisdiction basis and reasoning.
-/
theorem applyDecision_defective_filed_case_emits_judge_dismissal :
    let result := applyDecision defectiveFiledApplyReqForFlow
    (match result with
      | .ok ok =>
          ok.result_kind = "execute_tool" &&
          ok.state.isNone &&
          match ok.action with
          | some action =>
              action.action_type = "dismiss_for_lack_of_subject_matter_jurisdiction" &&
              action.actor_role = "judge" &&
              jsonStringFieldD action.payload "jurisdiction_basis_rejected" "" = "diversity" &&
              jsonStringFieldD action.payload "reasoning" "" = defectiveFiledDismissReasonForFlow
          | none => false
      | .error _ => false) = true := by
  native_decide

/-
This is the positive public-boundary theorem for the filed-case example.  It no
longer asks Lean to recompute the whole example by brute force.  It uses the
selector and confinement lemmas that the proof suite already established.
-/

/--
Once the judge takes that dismissal action in the defective filed case, `step`
closes the case, records the dismissal on the docket, and makes
`nextOpportunity` terminal.

The proof plan stays at the public boundary.  Evaluate the concrete dismissal
action through `step` and read the status and docket facts from the successor
state.  Then use the closed-case orchestration theorem to show that the same
successor state makes `nextOpportunity` terminal.
-/
theorem defective_filed_case_dismissal_then_stops :
    let stepResult := step defectiveFiledStateForFlow defectiveFiledDismissActionForFlow
    (match stepResult with
      | .ok s' =>
          s'.case.status = "closed" &&
          hasDocketTitle s'.case "Subject-Matter Jurisdiction Dismissal" &&
          (nextOpportunity
            { state := s'
            , roles := defectiveFiledRolesForFlow
            , max_steps_per_turn := 3 }).terminal
      | .error _ => false) = true := by
  native_decide

/-
This theorem states the public postconditions that matter.  It does not try to
name the entire successor state.  That is the right level for this file.  The
point is the boundary behavior: the dismissal closes the case, records the
docket entry, and stops the opportunity engine.
-/

/--
After the judge dismisses the defective filed case for lack of subject-matter
jurisdiction, the defendant's former Rule 12 path no longer has a current
opportunity to act on.

The proof plan uses the new generic closed-case decision theorem rather than
recomputing the whole `applyDecision` call.  First identify the concrete
successor state after the dismissal step.  Then show that this successor case
is closed.  The post-dismissal Rule 12 request uses that successor state's own
state version, so the generic theorem applies directly and yields
`NO_CURRENT_OPPORTUNITY`.
-/
theorem defective_filed_case_dismissal_blocks_later_rule12_decision :
    applyDecisionErrorCodeForFlow
      (applyDecision defectiveFiledPostDismissRule12ReqForFlow) = "NO_CURRENT_OPPORTUNITY" := by
  apply applyDecision_closed_case_returns_no_current_opportunity
  · native_decide
  · rfl

/-
This theorem completes the filed-case story at the public boundary.  Before the
dismissal, the defendant's Rule 12 path remains open but not actionable.  After
the dismissal, that same path is gone because the case is closed.  The engine
does not leave a stale party opportunity hanging after the court resolves the
mandatory jurisdiction screen.
-/

/--
In the defective filed case, the open set preserves the defendant's Rule 12
option, the current move still belongs to the judge, the defendant cannot act
out of order, the matching judge decision emits the right dismissal action
fields, and the resulting dismissal step closes the case and stops the
engine.

The proof plan is to assemble the earlier concrete and generic theorems into a
single public theorem.  The open-set facts come from the opportunity generator.
The wrong-role rejection comes from the public decision boundary.  The exact
judge-side result comes from the decision-boundary theorem above.  The closure
and terminality facts come from the step theorem above.
-/
theorem defective_filed_case_confinement :
    let opportunities := openOpportunities defectiveFiledReqForFlow
    opportunities.any (fun o => o.role = "judge" && o.allowed_tools = dismissalToolForFlow) = true ∧
    opportunities.any (fun o => o.role = "defendant" && o.allowed_tools = ["file_rule12_motion"]) = true ∧
    currentOpenOpportunity? defectiveFiledReqForFlow = some defectiveFiledOpportunityForFlow ∧
    applyDecisionErrorCodeForFlow (applyDecision defectiveFiledWrongRoleRule12ReqForFlow) = "WRONG_ROLE" ∧
    (let result := applyDecision defectiveFiledApplyReqForFlow
      match result with
      | .ok ok =>
          ok.result_kind = "execute_tool" &&
          ok.state.isNone &&
          match ok.action with
          | some action =>
              action.action_type = "dismiss_for_lack_of_subject_matter_jurisdiction" &&
              action.actor_role = "judge" &&
              jsonStringFieldD action.payload "jurisdiction_basis_rejected" "" = "diversity" &&
              jsonStringFieldD action.payload "reasoning" "" = defectiveFiledDismissReasonForFlow
          | none => false
      | .error _ => false) = true ∧
    (let stepResult := step defectiveFiledStateForFlow defectiveFiledDismissActionForFlow
      match stepResult with
      | .ok s' =>
          s'.case.status = "closed" &&
          hasDocketTitle s'.case "Subject-Matter Jurisdiction Dismissal" &&
          (nextOpportunity
            { state := s'
            , roles := defectiveFiledRolesForFlow
            , max_steps_per_turn := 3 }).terminal
      | .error _ => false) = true ∧
    applyDecisionErrorCodeForFlow
      (applyDecision defectiveFiledPostDismissRule12ReqForFlow) = "NO_CURRENT_OPPORTUNITY" := by
  constructor
  · exact (openOpportunities_defective_filed_case_keep_rule12_while_selecting_judge).1
  constructor
  · exact (openOpportunities_defective_filed_case_keep_rule12_while_selecting_judge).2
  constructor
  · exact currentOpenOpportunity_defective_filed_case_eq_named_opportunity
  constructor
  · exact defective_filed_case_rule12_available_but_not_actionable.2
  constructor
  · exact applyDecision_defective_filed_case_emits_judge_dismissal
  constructor
  · exact defective_filed_case_dismissal_then_stops
  · exact defective_filed_case_dismissal_blocks_later_rule12_decision

/-
This is the strongest jurisdiction-flow theorem in the current suite.  The
theorem is still concrete, but it demonstrates the architecture directly:
multiple options may remain open, formal priority selects the current actor,
out-of-order action is rejected, the matching decision emits the right action
fields, the dismissal step closes the case and stops the engine, and later
party attempts fail because no current opportunity remains.

The next worthwhile generalization is to remove more of the concrete filed-case
scaffolding.  The right direction is a theorem that starts from a shaped
`availableOpportunities` list and a shaped current opportunity, then derives
this same confinement story without naming one fixed case.
-/
