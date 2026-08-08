import Main

open Lean

def recentSingleClaim : Lean.Json :=
  Lean.Json.mkObj
    [ ("claim_id", Lean.Json.str "claim-1")
    , ("label", Lean.Json.str "Misrepresentation")
    , ("legal_theory", Lean.Json.str "misrepresentation")
    , ("standard_of_proof", Lean.Json.str "preponderance_of_the_evidence")
    , ("burden_holder", Lean.Json.str "plaintiff")
    , ("elements", Lean.Json.mkObj [])
    , ("defenses", Lean.Json.mkObj [])
    , ("damages_question", Lean.Json.str "What damages, if any, did plaintiff prove?")
    ]

def recentJuryVerdictCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-judgment"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "post_verdict"
    single_claim := some recentSingleClaim
    jury_verdict := some
      { verdict_for := "plaintiff"
      , votes_for_verdict := 6
      , required_votes := 4
      , damages := 125.0
      }
    hung_jury := none
  }

def recentJuryVerdictState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := recentJuryVerdictCase
  }

def recentEnterJudgmentAction : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "jury verdict")
      ]
  }

def recentEnterJudgmentSummary : Bool :=
  match step recentJuryVerdictState recentEnterJudgmentAction with
  | .ok nextState =>
      nextState.case.status = "judgment_entered" &&
      nextState.case.monetary_judgment.toBits = (125.0).toBits &&
      hasDocketTitle nextState.case "Judgment entered" &&
      hasDecisionTraceAction nextState.case "enter_judgment"
  | .error _ => false

/--
When a jury verdict already exists and the judge enters judgment on that
verdict, the engine carries the verdict damages into the monetary judgment and
sets the case status to `judgment_entered`.

The proof plan uses one concrete post-verdict state with the required claim
metadata and a plaintiff jury verdict.  The theorem then checks the exact
postconditions that matter: status, amount, docket entry, and decision trace.
-/
theorem step_enter_judgment_from_jury_verdict_sets_amount_and_status :
    recentEnterJudgmentSummary = true := by
  native_decide

/-
This theorem closes the path from verdict derivation to final judgment in the
maintained proof set.

It proves more than validator success.  The engine does the two concrete
things judgment entry must do: carry the verdict amount into
`monetary_judgment`, and move the case into `judgment_entered`.
-/

def recentHungJuryCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-hung"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "post_verdict"
    single_claim := some recentSingleClaim
    jury_verdict := none
    hung_jury := some
      { claim_id := "claim-1"
      , note := "sworn jurors remained split after deliberation round 2"
      }
  }

def recentHungJuryReq : OpportunityRequest :=
  { state :=
      { (default : CourtState) with
        schema_version := "v1"
        court_name := "Test Court"
        case := recentHungJuryCase
      }
  , roles := [{ role := "judge", allowed_tools := ["transition_case", "enter_judgment"] }]
  , max_steps_per_turn := 3
  }

/--
When a jury deadlocks and the case reaches `post_verdict`, the next judicial
step is to close the case rather than to enter judgment.

This is the missing terminal path for hung juries.  Without it, the engine can
record the deadlock but cannot finish the case.
-/
theorem nextOpportunity_hung_jury_post_verdict_closes_case :
    (nextOpportunity recentHungJuryReq).opportunity.map
        (fun opportunity => (opportunity.role, opportunity.allowed_tools, opportunity.kind)) =
      some ("judge", ["transition_case"], "required") := by
  native_decide
