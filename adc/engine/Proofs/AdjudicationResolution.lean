import ADC.Core

namespace ADCProofs.AdjudicationResolution

def adjudicationResolutionClaim (declaratoryOnly : Bool := true) : Lean.Json :=
  Lean.Json.mkObj
    [ ("claim_id", Lean.Json.str "claim-1")
    , ("label", Lean.Json.str "Proposition")
    , ("legal_theory", Lean.Json.str "proposition_adjudication")
    , ("standard_of_proof", Lean.Json.str "preponderance_of_the_evidence")
    , ("burden_holder", Lean.Json.str "plaintiff")
    , ("elements", Lean.Json.arr #[])
    , ("defenses", Lean.Json.arr #[])
    , ("damages_question", Lean.Json.str "Damages must be zero.")
    , ("declaratory_only", Lean.Json.bool declaratoryOnly)
    ]

def adjudicationResolutionCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1"
    filed_on := "2026-08-18"
    status := "trial"
    phase := "post_verdict"
    trial_mode := "jury"
    single_claim := some adjudicationResolutionClaim
  }

def adjudicationResolutionState (c : CaseState := adjudicationResolutionCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    case := c
  }

def adjudicationResolutionOf (result : Except String CourtState) : String :=
  match result with
  | .ok state => state.case.resolution
  | .error message => message

def adjudicationWinnerResolutionIs (c : CaseState) (winner expected : String) : Bool :=
  match resolveDeclaratoryForWinner c winner with
  | .ok resolved => resolved.resolution == expected
  | .error _ => false

def adjudicationJuryJudgmentAction : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "jury verdict")
      ]
  }

def adjudicationBenchOpinionAction (verdictFor : String) : CourtAction :=
  { action_type := "file_bench_opinion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("text", Lean.Json.str "Findings and conclusions support judgment.")
      , ("verdict_for", Lean.Json.str verdictFor)
      ]
  }

def adjudicationSettlementAction : CourtAction :=
  { action_type := "enter_settlement"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("summary", Lean.Json.str "The parties resolved the dispute.")
      , ("consent_judgment", Lean.Json.bool false)
      ]
  }

def adjudicationHungCloseAction : CourtAction :=
  { action_type := "transition_case"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [("next_status", Lean.Json.str "closed")]
  }

theorem resolveDeclaratoryForWinner_maps_burden_holder_to_demonstrated :
    adjudicationWinnerResolutionIs adjudicationResolutionCase
      "plaintiff" "demonstrated" = true := by
  native_decide

theorem resolveDeclaratoryForWinner_maps_opponent_to_not_demonstrated :
    adjudicationWinnerResolutionIs adjudicationResolutionCase
      "defendant" "not_demonstrated" = true := by
  native_decide

theorem resolveDeclaratoryForWinner_preserves_ordinary_cases :
    adjudicationWinnerResolutionIs
      { adjudicationResolutionCase with
        single_claim := some (adjudicationResolutionClaim false) }
      "plaintiff" "pending" = true := by
  native_decide

theorem step_jury_judgment_records_demonstrated :
    adjudicationResolutionOf
      (step
        (adjudicationResolutionState
          { adjudicationResolutionCase with
            jury_verdict := some
              { verdict_for := "plaintiff"
              , votes_for_verdict := 6
              , required_votes := 6
              , damages := 0.0
              } })
        adjudicationJuryJudgmentAction) = "demonstrated" := by
  native_decide

theorem step_jury_judgment_records_not_demonstrated :
    adjudicationResolutionOf
      (step
        (adjudicationResolutionState
          { adjudicationResolutionCase with
            jury_verdict := some
              { verdict_for := "defendant"
              , votes_for_verdict := 6
              , required_votes := 6
              , damages := 0.0
              } })
        adjudicationJuryJudgmentAction) = "not_demonstrated" := by
  native_decide

theorem step_jury_status_transition_records_demonstrated :
    adjudicationResolutionOf
      (step
        (adjudicationResolutionState
          { adjudicationResolutionCase with
            jury_verdict := some
              { verdict_for := "plaintiff"
              , votes_for_verdict := 6
              , required_votes := 6
              , damages := 0.0
              } })
        { action_type := "transition_case"
        , actor_role := "judge"
        , payload := Lean.Json.mkObj
            [("next_status", Lean.Json.str "judgment_entered")]
        }) = "demonstrated" := by
  native_decide

theorem step_bench_opinion_records_structured_resolution :
    adjudicationResolutionOf
      (step
        (adjudicationResolutionState
          { adjudicationResolutionCase with
            trial_mode := "bench"
            phase := "verdict_return" })
        (adjudicationBenchOpinionAction "plaintiff")) = "demonstrated" := by
  native_decide

theorem step_settlement_records_no_decision :
    adjudicationResolutionOf
      (step adjudicationResolutionState adjudicationSettlementAction) = "no_decision" := by
  native_decide

theorem step_hung_jury_close_records_no_decision :
    adjudicationResolutionOf
      (step
        (adjudicationResolutionState
          { adjudicationResolutionCase with
            hung_jury := some { claim_id := "claim-1", note := "No verdict." } })
        adjudicationHungCloseAction) = "no_decision" := by
  native_decide

end ADCProofs.AdjudicationResolution
