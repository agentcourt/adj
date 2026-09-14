import ADC.Core

namespace ADCProofs.VoirDire

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1"
    filed_on := "2026-01-01"
    status := "trial"
    phase := "voir_dire"
    jurors := [
      { juror_id := "J1", name := "Juror One", status := "candidate" },
      { juror_id := "J2", name := "Juror Two", status := "candidate" }
    ] }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    case := c }

def recordVoirDireAction (jurorId : String) : CourtAction :=
  { action_type := "record_voir_dire_question"
    actor_role := "plaintiff"
    payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("question", Lean.Json.str "Can you be fair and impartial?") ] }

def decideQuestionAction (allowed : Bool) : CourtAction :=
  { action_type := "decide_voir_dire_question"
    actor_role := "judge"
    payload := Lean.Json.mkObj
      [ ("exchange_id", Lean.Json.str "vdq-1")
      , ("juror_id", Lean.Json.str "J1")
      , ("allowed", Lean.Json.bool allowed)
      , ("ruling_reason", Lean.Json.str "proper question") ] }

def answerQuestionAction : CourtAction :=
  { action_type := "answer_voir_dire_question"
    actor_role := "juror"
    payload := Lean.Json.mkObj
      [ ("exchange_id", Lean.Json.str "vdq-1")
      , ("juror_id", Lean.Json.str "J1")
      , ("response", Lean.Json.str "Yes") ] }

def questionCase (allowed : Option Bool) (response : String := "") : CaseState :=
  { baseCase with
    voir_dire_exchanges := [{
      exchange_id := "vdq-1"
      juror_id := "J1"
      asked_by := "plaintiff"
      question := "Can you be fair and impartial?"
      judge_allowed := allowed
      response := response
      asked_at := "2026-01-01" }] }

def readyCase : CaseState :=
  { baseCase with
    juror_questionnaire_responses := [
      { juror_id := "J1", answers := [], submitted_at := "2026-01-01" },
      { juror_id := "J2", answers := [], submitted_at := "2026-01-01" }
    ]
    decision_traces := [
      { action := "pass_voir_dire_question", outcome := "plaintiff:J1", citations := [] },
      { action := "pass_voir_dire_question", outcome := "defendant:J1", citations := [] },
      { action := "pass_voir_dire_question", outcome := "plaintiff:J2", citations := [] },
      { action := "pass_voir_dire_question", outcome := "defendant:J2", citations := [] }
    ] }

def challengeForCauseAction (jurorId : String) : CourtAction :=
  { action_type := "challenge_juror_for_cause"
    actor_role := "plaintiff"
    payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("grounds", Lean.Json.str "expressed inability to follow instructions") ] }

def pendingChallengeCase : CaseState :=
  { readyCase with
    for_cause_challenges := [{
      challenge_id := "vdc-1"
      juror_id := "J1"
      by_party := "plaintiff"
      grounds := "expressed inability to follow instructions"
      requested_at := "2026-01-01" }] }

def decideChallengeAction (granted : Bool) : CourtAction :=
  { action_type := "decide_juror_for_cause_challenge"
    actor_role := "judge"
    payload := Lean.Json.mkObj
      [ ("challenge_id", Lean.Json.str "vdc-1")
      , ("juror_id", Lean.Json.str "J1")
      , ("by_party", Lean.Json.str "plaintiff")
      , ("granted", Lean.Json.bool granted)
      , ("ruling_reason", Lean.Json.str "record supports excusal") ] }

def peremptoryStrikeAction (jurorId : String) : CourtAction :=
  { action_type := "strike_juror_peremptorily"
    actor_role := "plaintiff"
    payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str jurorId)
      , ("party", Lean.Json.str "plaintiff") ] }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_record_voir_dire_requires_trial_status :
    let c := { baseCase with status := "pretrial" }
    stepErrorMessage (step (stateOf c) (recordVoirDireAction "J1")) =
      "record voir dire question requires trial status" := by
  native_decide

theorem step_record_voir_dire_requires_voir_dire_phase :
    let c := { baseCase with phase := "openings" }
    stepErrorMessage (step (stateOf c) (recordVoirDireAction "J1")) =
      "record voir dire question requires voir_dire phase; current phase is openings" := by
  native_decide

theorem step_record_voir_dire_rejects_unknown_juror :
    stepErrorMessage (step (stateOf) (recordVoirDireAction "J9")) =
      "unknown juror_id: J9" := by
  native_decide

theorem step_record_voir_dire_adds_pending_exchange :
    (match step (stateOf) (recordVoirDireAction "J1") with
      | .ok s' => s'.case.voir_dire_exchanges.map (fun exchange =>
          (exchange.exchange_id, exchange.juror_id, exchange.judge_allowed))
      | .error _ => []) = [("vdq-1", "J1", none)] := by
  native_decide

theorem step_decide_voir_dire_question_records_ruling :
    (match step (stateOf (questionCase none)) (decideQuestionAction true) with
      | .ok s' => s'.case.voir_dire_exchanges.map (fun exchange => exchange.judge_allowed)
      | .error _ => []) = [some true] := by
  native_decide

theorem step_answer_voir_dire_question_records_response :
    (match step (stateOf (questionCase (some true))) answerQuestionAction with
      | .ok s' => s'.case.voir_dire_exchanges.map (fun exchange => exchange.response)
      | .error _ => []) = ["Yes"] := by
  native_decide

theorem step_challenge_for_cause_adds_pending_challenge :
    (match step (stateOf readyCase) (challengeForCauseAction "J1") with
      | .ok s' => s'.case.for_cause_challenges.map (fun challenge =>
          (challenge.challenge_id, challenge.juror_id, challenge.granted))
      | .error _ => []) = [("vdc-1", "J1", none)] := by
  native_decide

theorem step_decide_challenge_for_cause_granted_marks_excused :
    (match step (stateOf pendingChallengeCase) (decideChallengeAction true) with
      | .ok s' => (s'.case.jurors.find? (fun juror => juror.juror_id = "J1")).map (fun juror => juror.status)
      | .error _ => none) = some "excused_for_cause" := by
  native_decide

theorem step_peremptory_strike_marks_struck :
    (match step (stateOf readyCase) (peremptoryStrikeAction "J2") with
      | .ok s' => (s'.case.jurors.find? (fun juror => juror.juror_id = "J2")).map (fun juror => juror.status)
      | .error _ => none) = some "struck_peremptory" := by
  native_decide

end ADCProofs.VoirDire
