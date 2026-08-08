import Main

def interrogatoryCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "pretrial",
    phase := "discovery"
  }

def interrogatoryState (c : CaseState := interrogatoryCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def serveInterrogatoriesAction : CourtAction :=
  { action_type := "serve_interrogatories"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("served_by", Lean.Json.str "plaintiff")
      , ("served_on", Lean.Json.str "defendant")
      , ("questions", Lean.Json.arr #[Lean.Json.str "Identify all communications with buyer."])
      ]
  }

def respondInterrogatoryItemAction : CourtAction :=
  { action_type := "respond_interrogatory_item"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("served_on", Lean.Json.str "defendant")
      , ("set_index", Lean.Json.num 0)
      , ("question_index", Lean.Json.num 0)
      , ("response", Lean.Json.str "No responsive communications known.")
      ]
  }

def finalizeInterrogatoriesAction : CourtAction :=
  { action_type := "finalize_interrogatory_responses"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("served_on", Lean.Json.str "defendant")
      , ("set_index", Lean.Json.num 0)
      , ("verified", Lean.Json.bool true)
      ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_serve_interrogatories_requires_pretrial :
    let c := { interrogatoryCase with status := "trial" }
    stepErrorMessage (step (interrogatoryState c) serveInterrogatoriesAction) =
      "interrogatories require pretrial status" := by
  native_decide

theorem step_respond_interrogatory_item_requires_prior_service :
    stepErrorMessage (step (interrogatoryState) respondInterrogatoryItemAction) =
      "cannot draft interrogatory response before service" := by
  native_decide

theorem step_finalize_interrogatories_requires_prior_service :
    stepErrorMessage (step (interrogatoryState) finalizeInterrogatoriesAction) =
      "cannot finalize interrogatory responses before service" := by
  native_decide

theorem step_serve_interrogatories_enforces_local_rule_limit :
    stepErrorMessage (step (interrogatoryState) serveInterrogatoriesAction) =
      "LOCAL_RULE_LIMIT_EXCEEDED|limit_key=discovery.interrogatory_sets_per_side|actor=plaintiff|phase=discovery|attempted=1|allowed=0|detail=interrogatory_set_count" := by
  native_decide
