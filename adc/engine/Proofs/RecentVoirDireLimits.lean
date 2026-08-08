import Main

open Lean

def voirDireReadyQuestionnaireResponse (jurorId : String) : JurorQuestionnaireResponse :=
  { juror_id := jurorId
  , answers := [{ question_id := "q1", answer := s!"answer-{jurorId}" }]
  , submitted_at := "2026-03-16"
  }

def disallowedExchange (exchangeId jurorId askedBy reason : String) : VoirDireExchange :=
  { exchange_id := exchangeId
  , juror_id := jurorId
  , asked_by := askedBy
  , question := s!"question-{exchangeId}"
  , judge_allowed := some false
  , ruling_reason := reason
  , asked_at := "2026-03-16"
  , ruled_at := some "2026-03-16"
  }

def pendingExchange (exchangeId jurorId askedBy question : String) : VoirDireExchange :=
  { exchange_id := exchangeId
  , juror_id := jurorId
  , asked_by := askedBy
  , question := question
  , asked_at := "2026-03-16"
  }

def limitedVoirDireCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-voir-dire-limit"
    filed_on := "2026-03-16"
    status := "trial"
    trial_mode := "jury"
    phase := "voir_dire"
    juror_questionnaire := [{ question_id := "q1", question := "Can you follow the court's instructions?" }]
    jurors := [{ juror_id := "J1", name := "J1", status := "candidate", model := "m1", persona_filename := "p1" }]
    juror_questionnaire_responses := [voirDireReadyQuestionnaireResponse "J1"]
    voir_dire_exchanges :=
      [ disallowedExchange "vx-1" "J1" "plaintiff" "Asked for a precommitment on sufficiency."
      , disallowedExchange "vx-2" "J1" "plaintiff" "Argued disputed facts."
      , disallowedExchange "vx-3" "J1" "plaintiff" "Asked about a specific damages award."
      ]
  }

def limitedVoirDireState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    policy :=
      { max_voir_dire_questions_per_side_per_juror := 1
      , max_disallowed_voir_dire_questions_per_side := 3
      }
    case := limitedVoirDireCase
  }

def limitedVoirDireReq : OpportunityRequest :=
  { state := limitedVoirDireState
  , roles :=
      [ { role := "plaintiff", allowed_tools := ["record_voir_dire_question"] }
      , { role := "defendant", allowed_tools := ["record_voir_dire_question"] }
      ]
  , max_steps_per_turn := 3
  }

def proposePlaintiffVoirDireAction : CourtAction :=
  { action_type := "record_voir_dire_question"
  , actor_role := "plaintiff"
  , payload := Json.mkObj
      [ ("juror_id", Json.str "J1")
      , ("question", Json.str "Would this signed confession be enough to prove liability?")
      ]
  }

def pendingRulingCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-voir-dire-ruling"
    filed_on := "2026-03-16"
    status := "trial"
    trial_mode := "jury"
    phase := "voir_dire"
    juror_questionnaire := [{ question_id := "q1", question := "Can you follow the court's instructions?" }]
    jurors := [{ juror_id := "J1", name := "J1", status := "candidate", model := "m1", persona_filename := "p1" }]
    juror_questionnaire_responses := [voirDireReadyQuestionnaireResponse "J1"]
    voir_dire_exchanges :=
      [ disallowedExchange "vx-1" "J1" "plaintiff" "Asked for a precommitment on sufficiency."
      , disallowedExchange "vx-2" "J1" "plaintiff" "Argued disputed facts."
      , pendingExchange "vx-3" "J1" "plaintiff"
          "Would you require live testimony rather than authenticated digital records?" ]
  }

def pendingRulingState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    policy :=
      { max_voir_dire_questions_per_side_per_juror := 1
      , max_disallowed_voir_dire_questions_per_side := 3
      }
    case := pendingRulingCase
  }

def disallowPendingQuestionAction : CourtAction :=
  { action_type := "decide_voir_dire_question"
  , actor_role := "judge"
  , payload := Json.mkObj
      [ ("exchange_id", Json.str "vx-3")
      , ("juror_id", Json.str "J1")
      , ("allowed", Json.bool false)
      , ("ruling_reason", Json.str "This asks the juror to precommit on evidentiary sufficiency in this case.")
      ]
  }

def voirDireStepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

def disallowRulingSummary : Bool :=
  match step pendingRulingState disallowPendingQuestionAction with
  | .ok nextState =>
      countDisallowedVoirDireQuestionsFrom nextState.case "plaintiff" == 3 &&
      match nextState.case.voir_dire_exchanges.find? (fun exchange => exchange.exchange_id = "vx-3") with
      | some exchange =>
          exchange.judge_allowed == some false &&
          exchange.ruling_reason ==
            "This asks the juror to precommit on evidentiary sufficiency in this case."
      | none => false
  | .error _ => false

theorem countDisallowedVoirDireQuestionsFrom_counts_rejected_questions_on_sample :
    countDisallowedVoirDireQuestionsFrom limitedVoirDireCase "plaintiff" = 3 := by
  native_decide

theorem step_record_voir_dire_question_rejects_when_disallow_limit_reached_on_sample :
    voirDireStepErrorMessage (step limitedVoirDireState proposePlaintiffVoirDireAction) =
      "voir dire disallow limit reached for plaintiff" := by
  native_decide

theorem step_decide_voir_dire_question_disallowed_increments_count_and_stores_reason_on_sample :
    disallowRulingSummary := by
  native_decide

theorem voir_dire_opportunities_remove_plaintiff_question_after_disallow_limit_on_sample :
    !(availableOpportunities limitedVoirDireReq).any (fun opportunity =>
      opportunity.role = "plaintiff" &&
      opportunity.phase = "voir_dire" &&
      opportunity.allowed_tools = ["record_voir_dire_question"]) := by
  native_decide

theorem voir_dire_opportunities_leave_defendant_question_below_its_disallow_limit_on_sample :
    (availableOpportunities limitedVoirDireReq).any (fun opportunity =>
      opportunity.role = "defendant" &&
      opportunity.phase = "voir_dire" &&
      opportunity.allowed_tools = ["record_voir_dire_question"]) := by
  native_decide
