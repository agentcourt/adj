import ADC.Core

namespace ADCProofs.DeclaratoryDamages

def declaratoryDamagesClaim (declaratoryOnly : Bool) : Lean.Json :=
  Lean.Json.mkObj
    [ ("claim_id", Lean.Json.str "claim-1")
    , ("label", Lean.Json.str "Declaratory claim")
    , ("legal_theory", Lean.Json.str "declaratory_relief")
    , ("standard_of_proof", Lean.Json.str "preponderance_of_the_evidence")
    , ("burden_holder", Lean.Json.str "plaintiff")
    , ("elements", Lean.Json.arr #[])
    , ("defenses", Lean.Json.arr #[])
    , ("damages_question", Lean.Json.str "Damages must be zero.")
    , ("declaratory_only", Lean.Json.bool declaratoryOnly)
    ]

def declaratoryDamagesLegacyClaim : Lean.Json :=
  Lean.Json.mkObj
    [ ("claim_id", Lean.Json.str "claim-1")
    , ("label", Lean.Json.str "Civil claim")
    , ("legal_theory", Lean.Json.str "civil_claim")
    , ("standard_of_proof", Lean.Json.str "preponderance_of_the_evidence")
    , ("burden_holder", Lean.Json.str "plaintiff")
    , ("elements", Lean.Json.arr #[])
    , ("defenses", Lean.Json.arr #[])
    , ("damages_question", Lean.Json.str "What damages are proven?")
    ]

def declaratoryDamagesCase (status : String := "pretrial") : CaseState :=
  { (default : CaseState) with
    case_id := "case-1"
    filed_on := "2026-08-18"
    status := status
    phase := "discovery"
    single_claim := some (declaratoryDamagesClaim true)
  }

def declaratoryDamagesState (c : CaseState := declaratoryDamagesCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    case := c
  }

def declaratoryDamagesError (result : Except String CourtState) : String :=
  match result with
  | .error message => message
  | .ok _ => ""

def declaratoryDamagesExpectedError : String :=
  "declaratory-only claim requires zero damages"

def declaratoryOnlyDefaultsFalse : Bool :=
  match singleClaimDeclaratoryOnly
      { (default : CaseState) with single_claim := some declaratoryDamagesLegacyClaim } with
  | .ok false => true
  | _ => false

def civilPositiveDamagesAllowed : Bool :=
  match validateDeclaratoryDamages
      { (default : CaseState) with single_claim := some declaratoryDamagesLegacyClaim }
      100.0 with
  | .ok () => true
  | .error _ => false

def declaratoryZeroDamagesAllowed : Bool :=
  match validateDeclaratoryDamages declaratoryDamagesCase 0.0 with
  | .ok () => true
  | .error _ => false

def declaratoryPartialJudgmentAction : CourtAction :=
  { action_type := "enter_partial_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("issues_resolved", Lean.Json.arr #[Lean.Json.str "liability"])
      , ("amount", Lean.Json.num 100)
      ]
  }

def declaratoryRule68OfferAction : CourtAction :=
  { action_type := "make_rule68_offer"
  , actor_role := "defendant"
  , payload := Lean.Json.mkObj
      [ ("offeree", Lean.Json.str "plaintiff")
      , ("amount", Lean.Json.num 100)
      ]
  }

def declaratoryPendingOfferCase : CaseState :=
  { declaratoryDamagesCase with
    rule68_offers :=
      [ { offer_id := "offer-1"
        , offeror := "defendant"
        , offeree := "plaintiff"
        , amount := 100.0
        , status := "pending"
        } ]
  }

def declaratoryExpiredOfferCase : CaseState :=
  { declaratoryDamagesCase with
    rule68_offers :=
      [ { offer_id := "offer-1"
        , offeror := "defendant"
        , offeree := "plaintiff"
        , amount := 100.0
        , status := "expired"
        } ]
  }

def declaratoryAcceptRule68Action : CourtAction :=
  { action_type := "accept_rule68_offer"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj [("offer_index", Lean.Json.num 0)]
  }

def declaratoryEvaluateRule68Action : CourtAction :=
  { action_type := "evaluate_rule68_cost_shift"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("offer_index", Lean.Json.num 0)
      , ("amount", Lean.Json.num 100)
      ]
  }

def declaratoryDefaultJudgmentAction : CourtAction :=
  { action_type := "enter_default_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("against_party", Lean.Json.str "defendant")
      , ("monetary_amount", Lean.Json.num 100)
      ]
  }

def declaratorySettlementAction : CourtAction :=
  { action_type := "enter_settlement"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [("amount", Lean.Json.num 100)]
  }

def declaratorySwornJuror : JurorRecord :=
  { juror_id := "J1", name := "Juror 1", status := "sworn" }

def declaratoryJurorVoteCase : CaseState :=
  { declaratoryDamagesCase "trial" with
    trial_mode := "jury"
    phase := "deliberation"
    jurors := [declaratorySwornJuror]
  }

def declaratoryPositiveJurorVoteAction : CourtAction :=
  { action_type := "submit_juror_vote"
  , actor_role := "juror"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str "J1")
      , ("vote", Lean.Json.str "plaintiff")
      , ("damages", Lean.Json.num 100)
      , ("confidence", Lean.Json.str "high")
      , ("explanation", Lean.Json.str "The claim was proven.")
      ]
  }

def declaratoryPositiveJuryVerdictCase : CaseState :=
  { declaratoryDamagesCase "trial" with
    trial_mode := "jury"
    phase := "post_verdict"
    jury_verdict := some
      { verdict_for := "plaintiff"
      , votes_for_verdict := 6
      , required_votes := 6
      , damages := 100.0
      }
  }

def declaratoryPositiveBenchCase : CaseState :=
  { declaratoryDamagesCase "trial" with
    trial_mode := "bench"
    phase := "post_verdict"
    monetary_judgment := 100.0
    docket := [{ title := "Bench Opinion", description := "Plaintiff prevails." }]
  }

def declaratoryEnterJudgmentAction : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "trial outcome")
      ]
  }

def declaratoryTransitionJudgmentAction : CourtAction :=
  { action_type := "transition_case"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [("next_status", Lean.Json.str "judgment_entered")]
  }

theorem singleClaimDeclaratoryOnly_defaults_false_when_field_absent :
    declaratoryOnlyDefaultsFalse = true := by
  native_decide

theorem validateDeclaratoryDamages_preserves_civil_positive_damages :
    civilPositiveDamagesAllowed = true := by
  native_decide

theorem validateDeclaratoryDamages_accepts_zero :
    declaratoryZeroDamagesAllowed = true := by
  native_decide

theorem step_declaratory_juror_vote_rejects_positive_damages :
    declaratoryDamagesError
      (step (declaratoryDamagesState declaratoryJurorVoteCase)
        declaratoryPositiveJurorVoteAction) = declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_partial_judgment_rejects_positive_damages :
    declaratoryDamagesError
      (step declaratoryDamagesState declaratoryPartialJudgmentAction) =
      declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_rule68_offer_rejects_positive_damages :
    declaratoryDamagesError
      (step declaratoryDamagesState declaratoryRule68OfferAction) =
      declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_rule68_acceptance_rejects_positive_damages :
    declaratoryDamagesError
      (step (declaratoryDamagesState declaratoryPendingOfferCase)
        declaratoryAcceptRule68Action) = declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_rule68_evaluation_rejects_positive_damages :
    declaratoryDamagesError
      (step (declaratoryDamagesState declaratoryExpiredOfferCase)
        declaratoryEvaluateRule68Action) = declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_default_judgment_rejects_positive_damages :
    declaratoryDamagesError
      (step declaratoryDamagesState declaratoryDefaultJudgmentAction) =
      declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_settlement_rejects_positive_damages :
    declaratoryDamagesError
      (step declaratoryDamagesState declaratorySettlementAction) =
      declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_jury_judgment_rejects_positive_damages :
    declaratoryDamagesError
      (step (declaratoryDamagesState declaratoryPositiveJuryVerdictCase)
        declaratoryEnterJudgmentAction) = declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_bench_judgment_rejects_positive_damages :
    declaratoryDamagesError
      (step (declaratoryDamagesState declaratoryPositiveBenchCase)
        declaratoryEnterJudgmentAction) = declaratoryDamagesExpectedError := by
  native_decide

theorem step_declaratory_status_transition_rejects_positive_jury_damages :
    declaratoryDamagesError
      (step (declaratoryDamagesState declaratoryPositiveJuryVerdictCase)
        declaratoryTransitionJudgmentAction) = declaratoryDamagesExpectedError := by
  native_decide

end ADCProofs.DeclaratoryDamages
