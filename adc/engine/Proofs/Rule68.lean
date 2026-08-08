import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "pretrial",
    phase := "discovery"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def makeRule68Action (offeree : String) : CourtAction :=
  { action_type := "make_rule68_offer"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("offeree", Lean.Json.str offeree)
      , ("amount", Lean.Json.num 100000)
      , ("offer_id", Lean.Json.str "offer-1")
      ]
  }

def acceptRule68ByIndexAction (actorRole : String) (idx : Nat) : CourtAction :=
  { action_type := "accept_rule68_offer"
  , actor_role := actorRole
  , payload := Lean.Json.mkObj [ ("offer_index", Lean.Json.num idx) ]
  }

def evaluateRule68ByIndexAction (idx : Nat) (amount : Nat) : CourtAction :=
  { action_type := "evaluate_rule68_cost_shift"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("offer_index", Lean.Json.num idx)
      , ("amount", Lean.Json.num amount)
      ]
  }

def pendingOfferCase : CaseState :=
  { baseCase with
    rule68_offers :=
      [ { offer_id := "offer-1"
        , offeror := "defendant"
        , offeree := "plaintiff"
        , amount := 100000.0
        , status := "pending"
        , terms := ""
        , claim_scope := ""
        , served_at := "2026-01-01"
        , expires_at := none
        , accepted_at := none
        , expired_at := none
        } ]
  }

def expiredOfferCase : CaseState :=
  { baseCase with
    rule68_offers :=
      [ { offer_id := "offer-1"
        , offeror := "defendant"
        , offeree := "plaintiff"
        , amount := 100000.0
        , status := "expired"
        , terms := ""
        , claim_scope := ""
        , served_at := "2026-01-01"
        , expires_at := none
        , accepted_at := none
        , expired_at := some "2026-01-20"
        } ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_make_rule68_rejects_invalid_offeree :
    stepErrorMessage (step (stateOf) (makeRule68Action "third_party")) =
      "invalid offeree: third_party" := by
  native_decide

theorem step_accept_rule68_requires_index_or_id :
    let action : CourtAction :=
      { action_type := "accept_rule68_offer", actor_role := "plaintiff", payload := Lean.Json.mkObj [] }
    stepErrorMessage (step (stateOf pendingOfferCase) action) =
      "accept_rule68_offer requires offer_index or offer_id" := by
  native_decide

theorem step_accept_rule68_only_offeree_may_accept :
    stepErrorMessage (step (stateOf pendingOfferCase) (acceptRule68ByIndexAction "defendant" 0)) =
      "only the offeree may accept rule68 offer" := by
  native_decide

theorem step_accept_rule68_success_sets_status_judgment_entered :
    (match step (stateOf pendingOfferCase) (acceptRule68ByIndexAction "plaintiff" 0) with
      | .ok s' => s'.case.status
      | .error _ => "") = "judgment_entered" := by
  native_decide

theorem step_evaluate_rule68_requires_index_or_id :
    let action : CourtAction :=
      { action_type := "evaluate_rule68_cost_shift", actor_role := "judge", payload := Lean.Json.mkObj [] }
    stepErrorMessage (step (stateOf expiredOfferCase) action) =
      "evaluate_rule68_cost_shift requires offer_index or offer_id" := by
  native_decide

theorem step_evaluate_rule68_records_cost_shift_docket :
    (match step (stateOf expiredOfferCase) (evaluateRule68ByIndexAction 0 70000) with
      | .ok s' => hasDocketTitle s'.case "Rule 68 Cost Shift Evaluation"
      | .error _ => false) = true := by
  native_decide
