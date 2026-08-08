import Main

def closedCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "closed",
    phase := "post_verdict"
  }

def closedReq : OpportunityRequest :=
  { state := { (default : CourtState) with case := closedCase }
  , roles :=
      [ { role := "plaintiff", allowed_tools := ["file_complaint"] }
      , { role := "judge", allowed_tools := ["enter_judgment"] }
      ]
  , max_steps_per_turn := 3
  }

def lowPriorityAction : OpportunitySpec :=
  { opportunity_id := ""
  , role := "plaintiff"
  , objective := "low"
  , allowed_tools := ["t1"]
  , step_budget := 1
  , priority := 200
  , deterministic_action := none
  }

def highPriorityAction : OpportunitySpec :=
  { opportunity_id := ""
  , role := "judge"
  , objective := "high"
  , allowed_tools := ["t2"]
  , step_budget := 1
  , priority := 10
  , deterministic_action := none
  }

theorem availableActions_closed_returns_empty :
    availableOpportunities closedReq = [] := by
  native_decide

theorem nextOpportunity_closed_stops_with_reason :
    let resp := nextOpportunity closedReq
    resp.terminal = true ∧ resp.reason = "no_eligible_opportunity" ∧ resp.opportunity = none := by
  native_decide

theorem selectLowestPriorityOpportunity_empty_none :
    selectLowestPriorityOpportunity? [] = none := by
  native_decide

theorem selectLowestPriorityOpportunity_prefers_lower_priority_value :
    (selectLowestPriorityOpportunity? [lowPriorityAction, highPriorityAction]).map (fun t => t.role) = some "judge" := by
  native_decide

theorem assignOpportunityIds_preserves_length
    (actions : List OpportunitySpec) :
    (assignOpportunityIds actions).length = actions.length := by
  simp [assignOpportunityIds]

theorem nextOpportunity_opportunity_eq_currentOpenOpportunity
    (req : OpportunityRequest) :
    (nextOpportunity req).opportunity = currentOpenOpportunity? req := by
  unfold nextOpportunity currentOpenOpportunity?
  cases hSelect : selectLowestPriorityOpportunity? (openOpportunities req) <;> simp [hSelect]

theorem nextOpportunity_terminal_iff_no_currentOpenOpportunity
    (req : OpportunityRequest) :
    (nextOpportunity req).terminal = true ↔ currentOpenOpportunity? req = none := by
  unfold nextOpportunity currentOpenOpportunity?
  cases hSelect : selectLowestPriorityOpportunity? (openOpportunities req) <;> simp [hSelect]

/--
A closed case has no available opportunities, regardless of roles or phase.

The proof plan is direct.  `availableOpportunities` checks `req.state.case.status`
first and returns the empty list immediately when the status is `closed`.  After
unfolding the definition, `simp` with the status hypothesis finishes the proof.
-/
theorem availableOpportunities_nil_when_case_closed
    (req : OpportunityRequest)
    (hclosed : req.state.case.status = "closed") :
    availableOpportunities req = [] := by
  unfold availableOpportunities
  simp [hclosed]

/-
This is the reusable closed-case theorem that the older concrete `closedReq`
example was pointing at.  It is more useful than the concrete example because
later proofs can apply it to any state that reaches `closed`.
-/

/--
A closed case has no current open opportunity.

The proof plan uses the previous theorem.  `currentOpenOpportunity?` selects
from `openOpportunities`, which in turn filters `availableOpportunities`.  Once
the available-opportunity list is empty, the current open opportunity is `none`.
-/
theorem currentOpenOpportunity_none_when_case_closed
    (req : OpportunityRequest)
    (hclosed : req.state.case.status = "closed") :
    currentOpenOpportunity? req = none := by
  unfold currentOpenOpportunity? openOpportunities
  rw [availableOpportunities_nil_when_case_closed req hclosed]
  simp [selectLowestPriorityOpportunity?]

/-
This theorem is the real bridge to later step-level results.  If a successful
action closes the case, later orchestration proofs can stop at this lemma
instead of re-unfolding the opportunity machinery.
-/

/--
A closed case makes `nextOpportunity` terminal.

The proof plan combines `currentOpenOpportunity_none_when_case_closed` with the
existing equivalence between terminal responses and missing current
opportunities.
-/
theorem nextOpportunity_terminal_when_case_closed
    (req : OpportunityRequest)
    (hclosed : req.state.case.status = "closed") :
    (nextOpportunity req).terminal = true := by
  rw [nextOpportunity_terminal_iff_no_currentOpenOpportunity]
  exact currentOpenOpportunity_none_when_case_closed req hclosed

/-
This is the summary orchestration fact for closed cases.  It states the public
effect of closure directly in terms of `nextOpportunity`, which is the boundary
the Go runner actually uses.
-/
