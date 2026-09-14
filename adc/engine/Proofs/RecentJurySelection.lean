import ADC.Core

namespace ADCProofs.RecentJurySelection

def jurorIdentityProjection (juror : JurorRecord) : String × String × String × String × String :=
  (juror.juror_id, juror.name, juror.note, juror.model, juror.persona_filename)

def sampleVoirDirePanel : List JurorRecord :=
  [ { juror_id := "J1", name := "J1", status := "candidate", note := "n1", model := "m1", persona_filename := "p1" }
  , { juror_id := "J2", name := "J2", status := "candidate", note := "n2", model := "m2", persona_filename := "p2" }
  , { juror_id := "J3", name := "J3", status := "excused_for_cause", note := "n3", model := "m3", persona_filename := "p3" }
  , { juror_id := "J4", name := "J4", status := "candidate", note := "n4", model := "m4", persona_filename := "p4" }
  ]

def sampleSelectedJurors : List String :=
  ["J2", "J4"]

def readyQuestionnaireResponse (jurorId : String) : JurorQuestionnaireResponse :=
  { juror_id := jurorId
  , answers := [{ question_id := "q1", answer := s!"answer-{jurorId}" }]
  , submitted_at := "2026-03-15"
  }

def answeredVoirDireExchange (jurorId askedBy exchangeId : String) : VoirDireExchange :=
  { exchange_id := exchangeId
  , juror_id := jurorId
  , asked_by := askedBy
  , question := s!"question-{askedBy}-{jurorId}"
  , judge_allowed := some true
  , ruling_reason := "allowed"
  , response := s!"response-{askedBy}-{jurorId}"
  , asked_at := "2026-03-15"
  , ruled_at := some "2026-03-15"
  , answered_at := some "2026-03-15"
  }

def readyVoirDireJurors : List JurorRecord :=
  [ { juror_id := "J1", name := "J1", status := "candidate", model := "m1", persona_filename := "p1" }
  , { juror_id := "J2", name := "J2", status := "candidate", model := "m2", persona_filename := "p2" }
  , { juror_id := "J3", name := "J3", status := "candidate", model := "m3", persona_filename := "p3" }
  , { juror_id := "J4", name := "J4", status := "candidate", model := "m4", persona_filename := "p4" }
  , { juror_id := "J5", name := "J5", status := "candidate", model := "m5", persona_filename := "p5" }
  , { juror_id := "J6", name := "J6", status := "candidate", model := "m6", persona_filename := "p6" }
  ]

def readyVoirDireCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-ready-jury"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "voir_dire"
    jury_configuration := some { juror_count := 6, unanimous_required := false, minimum_concurring := 4 }
    juror_questionnaire := [{ question_id := "q1", question := "Can you follow the court's instructions?" }]
    jurors := readyVoirDireJurors
    juror_questionnaire_responses := readyVoirDireJurors.map (fun juror => readyQuestionnaireResponse juror.juror_id)
    voir_dire_exchanges :=
      [ answeredVoirDireExchange "J1" "plaintiff" "px-0"
      , answeredVoirDireExchange "J2" "plaintiff" "px-1"
      , answeredVoirDireExchange "J3" "plaintiff" "px-2"
      , answeredVoirDireExchange "J4" "plaintiff" "px-3"
      , answeredVoirDireExchange "J5" "plaintiff" "px-4"
      , answeredVoirDireExchange "J6" "plaintiff" "px-5"
      , answeredVoirDireExchange "J1" "defendant" "dx-0"
      , answeredVoirDireExchange "J2" "defendant" "dx-1"
      , answeredVoirDireExchange "J3" "defendant" "dx-2"
      , answeredVoirDireExchange "J4" "defendant" "dx-3"
      , answeredVoirDireExchange "J5" "defendant" "dx-4"
      , answeredVoirDireExchange "J6" "defendant" "dx-5"
      ]
  }

def readyVoirDireState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    policy :=
      { max_voir_dire_questions_per_side_per_juror := 1
      , max_for_cause_challenges_per_side := 1
      , max_peremptory_challenges_per_side := 1
      }
    case := readyVoirDireCase
  }

def readyVoirDireReq : OpportunityRequest :=
  { state := readyVoirDireState
  , roles :=
      [ { role := "judge", allowed_tools := ["empanel_jury"] }
      , { role := "plaintiff", allowed_tools := ["record_voir_dire_question"] }
      , { role := "defendant", allowed_tools := ["record_voir_dire_question"] }
      ]
  , max_steps_per_turn := 3
  }

def readyVoirDireBoundarySummary : Bool :=
  allAvailableJurorsReadyForEmpanelment readyVoirDireCase 1 3 &&
  nextCandidateWithoutQuestionnaireResponse? readyVoirDireCase = none &&
  nextAvailableJurorNeedingQuestionFrom? readyVoirDireCase "plaintiff" 1 = none &&
  nextAvailableJurorNeedingQuestionFrom? readyVoirDireCase "defendant" 1 = none &&
  (availableOpportunities readyVoirDireReq).any (fun opportunity =>
    opportunity.role = "judge" && opportunity.allowed_tools = ["empanel_jury"])

def candidateCountState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    policy :=
      { max_voir_dire_questions_per_side_per_juror := 1
      , max_for_cause_challenges_per_side := 1
      , max_peremptory_challenges_per_side := 1
      }
    case :=
      { (default : CaseState) with
        case_id := "case-selection-counts"
        filed_on := "2026-03-15"
        status := "trial"
        trial_mode := "jury"
        phase := "voir_dire"
        jurors :=
          [ { juror_id := "J1", name := "J1", status := "candidate", model := "m1", persona_filename := "p1" }
          , { juror_id := "J2", name := "J2", status := "candidate", model := "m2", persona_filename := "p2" }
          ]
        juror_questionnaire_responses :=
          [ readyQuestionnaireResponse "J1"
          , readyQuestionnaireResponse "J2"
          ]
        voir_dire_exchanges :=
          [ answeredVoirDireExchange "J1" "plaintiff" "px-0"
          , answeredVoirDireExchange "J1" "defendant" "dx-0"
          , answeredVoirDireExchange "J2" "plaintiff" "px-1"
          , answeredVoirDireExchange "J2" "defendant" "dx-1"
          ]
        for_cause_challenges :=
          [ { challenge_id := "vdc-1"
            , juror_id := "J1"
            , by_party := "plaintiff"
            , grounds := "Juror admitted inability to follow the court's instructions."
            , requested_at := "2026-03-15"
            }
          ]
      }
  }

def decideForCauseGrantedAction : CourtAction :=
  { action_type := "decide_juror_for_cause_challenge"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("challenge_id", Lean.Json.str "vdc-1")
      , ("juror_id", Lean.Json.str "J1")
      , ("by_party", Lean.Json.str "plaintiff")
      , ("granted", Lean.Json.bool true)
      , ("ruling_reason", Lean.Json.str "Juror cannot be impartial.")
      ]
  }

def peremptoryStrikeAction : CourtAction :=
  { action_type := "strike_juror_peremptorily"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str "J2")
      , ("party", Lean.Json.str "plaintiff")
      , ("reason", Lean.Json.str "exercise peremptory")
      ]
  }

def grantedForCauseSummary : Bool :=
  match step candidateCountState decideForCauseGrantedAction with
  | .ok nextState =>
      countCandidates nextState.case.jurors + 1 = countCandidates candidateCountState.case.jurors &&
      (jurorById? nextState.case.jurors "J1").map (fun juror => juror.status) = some "excused_for_cause"
  | .error _ => false

def peremptoryStrikeSummary : Bool :=
  match step candidateCountState peremptoryStrikeAction with
  | .ok nextState =>
      countCandidates nextState.case.jurors + 1 = countCandidates candidateCountState.case.jurors &&
      (jurorById? nextState.case.jurors "J2").map (fun juror => juror.status) = some "struck_peremptory"
  | .error _ => false

def voirDireQuestionPassCase : CaseState :=
  { readyVoirDireCase with
    case_id := "case-voir-dire-pass"
    jury_configuration := some { juror_count := 1, unanimous_required := false, minimum_concurring := 1 }
    jurors := [{ juror_id := "J1", name := "J1", status := "candidate", model := "m1", persona_filename := "p1" }]
    juror_questionnaire_responses := [readyQuestionnaireResponse "J1"]
    voir_dire_exchanges := [answeredVoirDireExchange "J1" "plaintiff" "px-0"]
  }

def voirDireQuestionPassState : CourtState :=
  { readyVoirDireState with case := voirDireQuestionPassCase }

def voirDireQuestionPassReq : OpportunityRequest :=
  { state := voirDireQuestionPassState
  , roles :=
      [ { role := "judge", allowed_tools := ["empanel_jury"] }
      , { role := "plaintiff", allowed_tools := ["challenge_juror_for_cause", "strike_juror_peremptorily"] }
      , { role := "defendant", allowed_tools := ["record_voir_dire_question", "challenge_juror_for_cause", "strike_juror_peremptorily"] }
      ]
  , max_steps_per_turn := 3
  }

def voirDireQuestionPassLeavesFurtherOpportunity : Bool :=
  match currentOpenOpportunity? voirDireQuestionPassReq with
  | some opportunity =>
      match applyDecision
          { state := voirDireQuestionPassReq.state
          , state_version := voirDireQuestionPassReq.state.state_version
          , opportunity_id := opportunity.opportunity_id
          , role := opportunity.role
          , decision := { kind := "pass" }
          , roles := voirDireQuestionPassReq.roles
          , max_steps_per_turn := voirDireQuestionPassReq.max_steps_per_turn
          } with
      | .ok ok =>
          match ok.state with
          | some nextState =>
              nextAvailableJurorNeedingQuestionFrom? nextState.case "defendant" 1 = none &&
              match currentOpenOpportunity?
                  { state := nextState
                  , roles := voirDireQuestionPassReq.roles
                  , max_steps_per_turn := voirDireQuestionPassReq.max_steps_per_turn
                  } with
              | some _ => true
              | none => false
          | none => false
      | .error _ => false
  | none => false

/--
Empanelment preserves juror identity fields on a representative mixed panel.

The sample includes selected candidates, an unselected candidate, and a juror
already removed for cause.  The theorem checks that empanelment changes only
status, not id, name, note, model, or persona file.
-/
theorem empanelSelectedJurors_preserves_identity_projection_on_sample :
    (empanelSelectedJurors sampleVoirDirePanel sampleSelectedJurors).map jurorIdentityProjection =
      sampleVoirDirePanel.map jurorIdentityProjection := by
  native_decide

/--
Selected jurors become sworn on the sample panel.

This theorem checks one of the selected jurors directly through the same
`jurorById?` helper the runtime uses for later jury operations.
-/
theorem empanelSelectedJurors_marks_J2_sworn_on_sample :
    (jurorById? (empanelSelectedJurors sampleVoirDirePanel sampleSelectedJurors) "J2").map
        (fun juror => juror.status) =
      some "sworn" := by
  native_decide

/--
The second selected juror also becomes sworn on the sample panel.

The sample selects two jurors so the theorem set does not depend on a single
position or a single id.
-/
theorem empanelSelectedJurors_marks_J4_sworn_on_sample :
    (jurorById? (empanelSelectedJurors sampleVoirDirePanel sampleSelectedJurors) "J4").map
        (fun juror => juror.status) =
      some "sworn" := by
  native_decide

/--
An unselected candidate becomes `excused_after_voir_dire` on the sample panel.

Empanelment records that a remaining candidate was left off the jury instead of
dropping the juror from state.
-/
theorem empanelSelectedJurors_marks_J1_excused_on_sample :
    (jurorById? (empanelSelectedJurors sampleVoirDirePanel sampleSelectedJurors) "J1").map
        (fun juror => juror.status) =
      some "excused_after_voir_dire" := by
  native_decide

/--
Once every remaining candidate has a questionnaire response and one answered
oral question from each side, the selection record is ready for empanelment.

Counsel may still choose challenges or peremptories.  The theorem establishes
that every candidate has completed questionnaire work and party questioning,
and that the judge's empanelment opportunity is available.
-/
theorem ready_voir_dire_panel_exposes_empanelment_boundary :
    readyVoirDireBoundarySummary = true := by
  native_decide

theorem passing_last_voir_dire_question_still_leaves_next_opportunity :
    voirDireQuestionPassLeavesFurtherOpportunity = true := by
  native_decide

/--
Granting a pending for-cause challenge removes exactly one candidate from the
available panel on the representative voir-dire state.

The proof checks both effects that matter.  The candidate count drops by one,
and the targeted juror's status becomes `excused_for_cause`.
-/
theorem decide_juror_for_cause_challenge_granted_reduces_candidate_count_on_sample :
    grantedForCauseSummary = true := by
  native_decide

/--
A peremptory strike removes exactly one candidate from the available panel on
the representative voir-dire state.

The proof checks the same two effects as the for-cause theorem: one fewer
candidate remains, and the targeted juror's status becomes
`struck_peremptory`.
-/
theorem strike_juror_peremptorily_reduces_candidate_count_on_sample :
    peremptoryStrikeSummary = true := by
  native_decide

end ADCProofs.RecentJurySelection
