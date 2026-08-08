import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "filed",
    phase := "none"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def transitionAction (nextStatus : String) : CourtAction :=
  { action_type := "transition_case"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [ ("next_status", Lean.Json.str nextStatus) ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_transition_case_rejects_invalid_status :
    stepErrorMessage (step (stateOf) (transitionAction "archived")) =
      "invalid status: archived" := by
  native_decide

theorem step_transition_case_rejects_invalid_transition :
    stepErrorMessage (step (stateOf) (transitionAction "trial")) =
      "invalid transition from filed to trial" := by
  native_decide

theorem step_transition_case_filed_to_pretrial_succeeds :
    (match step (stateOf) (transitionAction "pretrial") with
      | .ok s' => s'.case.status
      | .error _ => "") = "pretrial" := by
  native_decide

theorem step_transition_case_trial_to_judgment_requires_verdict :
    let c := { baseCase with status := "trial", phase := "post_verdict", jury_verdict := none, hung_jury := none }
    stepErrorMessage (step (stateOf c) (transitionAction "judgment_entered")) =
      "cannot transition to judgment_entered without jury verdict" := by
  native_decide

theorem step_transition_case_trial_to_judgment_rejects_hung :
    let c := { baseCase with
      status := "trial",
      phase := "post_verdict",
      jury_verdict := some { verdict_for := "plaintiff", votes_for_verdict := 6, required_votes := 6, damages := 100.0 },
      hung_jury := some { claim_id := "claim-1", note := "deadlock" } }
    stepErrorMessage (step (stateOf c) (transitionAction "judgment_entered")) =
      "cannot transition to judgment_entered after hung jury" := by
  native_decide

theorem step_transition_case_trial_to_judgment_succeeds_with_verdict :
    let c := { baseCase with
      status := "trial",
      phase := "post_verdict",
      jury_verdict := some { verdict_for := "plaintiff", votes_for_verdict := 6, required_votes := 6, damages := 100.0 },
      hung_jury := none }
    (match step (stateOf c) (transitionAction "judgment_entered") with
      | .ok s' => s'.case.status
      | .error _ => "") = "judgment_entered" := by
  native_decide
