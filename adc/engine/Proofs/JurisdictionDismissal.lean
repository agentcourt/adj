import ADC.Core

namespace ADCProofs.JurisdictionDismissal

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-jurisdiction-1",
    filed_on := "2026-01-01",
    status := "filed",
    phase := "pleadings"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def dismissForLackOfSubjectMatterJurisdictionAction
    (reasoning : String := "The complaint does not adequately allege a basis for federal jurisdiction.") :
    CourtAction :=
  { action_type := "dismiss_for_lack_of_subject_matter_jurisdiction"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("jurisdiction_basis_rejected", Lean.Json.str "diversity")
      , ("reasoning", Lean.Json.str reasoning)
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

def reqOf (s : CourtState) : OpportunityRequest :=
  { state := s
  , roles :=
      [ { role := "plaintiff", allowed_tools := ["file_amended_complaint"] }
      , { role := "defendant", allowed_tools := ["file_answer"] }
      , { role := "judge", allowed_tools := ["dismiss_for_lack_of_subject_matter_jurisdiction"] }
      ]
  , max_steps_per_turn := 3
  }

/--
The judge-only subject-matter-jurisdiction dismissal requires non-empty
reasoning.

The proof plan is local.  Build the dismissal action with an empty
reasoning field and confirm that `step` rejects it with the exact guard
message.  The theorem isolates the payload requirement from case closure.
-/
theorem step_jurisdiction_dismissal_requires_reasoning :
    stepErrorMessage
      (step (stateOf) (dismissForLackOfSubjectMatterJurisdictionAction "")) =
        "subject-matter-jurisdiction dismissal requires reasoning" := by
  native_decide

/--
A successful subject-matter-jurisdiction dismissal closes the case and
records the dismissal on the docket.

The proof plan is again local.  Run the dismissal step on a filed case and
check the two effects that matter most for later reasoning: the resulting
case status is `closed`, and the docket contains the dismissal entry.
-/
theorem step_jurisdiction_dismissal_closes_case_and_records_docket :
    let result := step (stateOf) dismissForLackOfSubjectMatterJurisdictionAction
    (match result with
      | .ok s' => s'.case.status = "closed" && hasDocketTitle s'.case "Subject-Matter Jurisdiction Dismissal"
      | .error _ => false) = true := by
  native_decide

/--
A successful subject-matter-jurisdiction dismissal leaves no later opportunity.

The proof plan is concrete but still meaningful.  Run the dismissal step on
the base filed case, build a fresh opportunity request from the resulting
state, and check that `nextOpportunity` is terminal.  This ties the local
dismissal action to the public orchestration boundary the runner uses.
-/
theorem step_jurisdiction_dismissal_blocks_next_opportunity :
    let result := step (stateOf) dismissForLackOfSubjectMatterJurisdictionAction
    (match result with
      | .ok s' => (nextOpportunity (reqOf s')).terminal
      | .error _ => false) = true := by
  native_decide

end ADCProofs.JurisdictionDismissal
