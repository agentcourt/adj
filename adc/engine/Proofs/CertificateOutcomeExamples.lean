import Proofs.CertificateOutcomeFacts

def certificateOutcomeSwornJuror (jurorId : String) : JurorRecord :=
  { juror_id := jurorId
  , name := jurorId
  , status := "sworn"
  }

def certificateOutcomeVote
    (jurorId vote : String)
    (damages : Float)
    (explanation : String) :
    JurorVote :=
  { juror_id := jurorId
  , round := 1
  , vote := vote
  , damages := damages
  , confidence := "high"
  , explanation := explanation
  , submitted_at := "2026-03-15"
  }

def certificateVerdictInitialCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-verdict"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "deliberation"
    jury_configuration :=
      some { juror_count := 6, unanimous_required := false, minimum_concurring := 4 }
    jurors :=
      [ certificateOutcomeSwornJuror "J1"
      , certificateOutcomeSwornJuror "J2"
      , certificateOutcomeSwornJuror "J3"
      , certificateOutcomeSwornJuror "J4"
      , certificateOutcomeSwornJuror "J5"
      , certificateOutcomeSwornJuror "J6"
      ]
    juror_votes :=
      [ certificateOutcomeVote "J1" "plaintiff" 100.0 "J1 explanation"
      , certificateOutcomeVote "J2" "plaintiff" 100.0 "J2 explanation"
      , certificateOutcomeVote "J3" "plaintiff" 100.0 "J3 explanation"
      , certificateOutcomeVote "J4" "defendant" 0.0 "J4 explanation"
      , certificateOutcomeVote "J5" "defendant" 0.0 "J5 explanation"
      ]
  }

def certificateVerdictInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := certificateVerdictInitialCase
  }

def certificateVerdictFinalVoteAction : CourtAction :=
  { action_type := "submit_juror_vote"
  , actor_role := "juror"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str "J6")
      , ("vote", Lean.Json.str "plaintiff")
      , ("damages", Lean.Json.num 100)
      , ("confidence", Lean.Json.str "high")
      , ("explanation", Lean.Json.str "J6 explanation")
      ]
  }

def sampleVerdictCertificateInit : ReplayInitializeRequest :=
  { state := certificateVerdictInitialState
  , initialize_case := none
  }

def sampleVerdictCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateVerdictFinalVoteAction]

def sampleVerdictCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions)

theorem sample_verdict_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleVerdictCertificateInit
        sampleVerdictCertificateTransitions) = true := by
  native_decide

theorem sample_verdict_certificate_replay :
    AcceptedReplayCertificate
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions
      sampleVerdictCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions)
    sample_verdict_certificate_replay_bool

theorem sample_verdict_certificate_accounted :
    juryVerdictAccounted sampleVerdictCertificateState = true := by
  native_decide

theorem sample_verdict_certificate_facts :
    VerdictCertificateFacts
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions
      sampleVerdictCertificateState := by
  exact acceptedReplayCertificate_verdict_facts
    sampleVerdictCertificateInit
    sampleVerdictCertificateTransitions
    sampleVerdictCertificateState
    sample_verdict_certificate_replay
    sample_verdict_certificate_accounted

theorem sample_verdict_certificate_outcome_facts :
    OutcomeCertificateFacts
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions
      sampleVerdictCertificateState := by
  exact OutcomeCertificateFacts.verdict
    sample_verdict_certificate_facts

def certificateJudgmentClaim : Lean.Json :=
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

def certificateJudgmentInitialCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-judgment"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "post_verdict"
    single_claim := some certificateJudgmentClaim
    jury_verdict :=
      some
        { verdict_for := "plaintiff"
        , votes_for_verdict := 6
        , required_votes := 4
        , damages := 125.0
        }
  }

def certificateJudgmentInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := certificateJudgmentInitialCase
  }

def certificateEnterJudgmentAction : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "jury verdict")
      ]
  }

def sampleJudgmentCertificateInit : ReplayInitializeRequest :=
  { state := certificateJudgmentInitialState
  , initialize_case := none
  }

def sampleJudgmentCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateEnterJudgmentAction]

def sampleJudgmentCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions)

theorem sample_judgment_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleJudgmentCertificateInit
        sampleJudgmentCertificateTransitions) = true := by
  native_decide

theorem sample_judgment_certificate_replay :
    AcceptedReplayCertificate
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions
      sampleJudgmentCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions)
    sample_judgment_certificate_replay_bool

theorem sample_judgment_certificate_accounted :
    judgmentFromVerdictAccounted sampleJudgmentCertificateState = true := by
  native_decide

theorem sample_judgment_certificate_facts :
    JudgmentCertificateFacts
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions
      sampleJudgmentCertificateState := by
  exact acceptedReplayCertificate_judgment_facts
    sampleJudgmentCertificateInit
    sampleJudgmentCertificateTransitions
    sampleJudgmentCertificateState
    sample_judgment_certificate_replay
    sample_judgment_certificate_accounted

theorem sample_judgment_certificate_outcome_facts :
    OutcomeCertificateFacts
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions
      sampleJudgmentCertificateState := by
  exact OutcomeCertificateFacts.judgment
    sample_judgment_certificate_facts

def certificateTimeoutInitialCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-juror-timeout"
    filed_on := "2026-06-06"
    status := "trial"
    trial_mode := "jury"
    phase := "deliberation"
    jury_configuration :=
      some { juror_count := 6, unanimous_required := true, minimum_concurring := 6 }
    jurors :=
      [ certificateOutcomeSwornJuror "J1"
      , certificateOutcomeSwornJuror "J2"
      , certificateOutcomeSwornJuror "J3"
      , certificateOutcomeSwornJuror "J4"
      , certificateOutcomeSwornJuror "J5"
      , certificateOutcomeSwornJuror "J6"
      ]
    juror_votes :=
      [ certificateOutcomeVote "J2" "plaintiff" 100.0 "J2 explanation"
      , certificateOutcomeVote "J3" "plaintiff" 100.0 "J3 explanation"
      , certificateOutcomeVote "J4" "plaintiff" 100.0 "J4 explanation"
      , certificateOutcomeVote "J5" "plaintiff" 100.0 "J5 explanation"
      , certificateOutcomeVote "J6" "plaintiff" 100.0 "J6 explanation"
      ]
  }

def certificateTimeoutInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := certificateTimeoutInitialCase
  }

def certificateJurorTimeoutAction : CourtAction :=
  { action_type := "process_juror_timeout"
  , actor_role := "system"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str "J1") ]
  }

def sampleJurorTimeoutCertificateInit : ReplayInitializeRequest :=
  { state := certificateTimeoutInitialState
  , initialize_case := none
  }

def sampleJurorTimeoutCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateJurorTimeoutAction]

def sampleJurorTimeoutCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleJurorTimeoutCertificateInit
      sampleJurorTimeoutCertificateTransitions)

theorem sample_juror_timeout_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleJurorTimeoutCertificateInit
        sampleJurorTimeoutCertificateTransitions) = true := by
  native_decide

theorem sample_juror_timeout_certificate_replay :
    AcceptedReplayCertificate
      sampleJurorTimeoutCertificateInit
      sampleJurorTimeoutCertificateTransitions
      sampleJurorTimeoutCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleJurorTimeoutCertificateInit
      sampleJurorTimeoutCertificateTransitions)
    sample_juror_timeout_certificate_replay_bool

theorem sample_juror_timeout_certificate_transition_recorded :
    replayTransitionsContainJurorTimeout
      sampleJurorTimeoutCertificateTransitions
      "J1" = true := by
  native_decide

theorem sample_juror_timeout_certificate_accounted :
    jurorFailureVerdictAccounted
      sampleJurorTimeoutCertificateState
      "J1" = true := by
  native_decide

theorem sample_juror_timeout_certificate_facts :
    JurorFailureVerdictCertificateFacts
      sampleJurorTimeoutCertificateInit
      sampleJurorTimeoutCertificateTransitions
      sampleJurorTimeoutCertificateState
      "J1" := by
  exact acceptedReplayCertificate_juror_failure_verdict_facts
    sampleJurorTimeoutCertificateInit
    sampleJurorTimeoutCertificateTransitions
    sampleJurorTimeoutCertificateState
    "J1"
    sample_juror_timeout_certificate_replay
    sample_juror_timeout_certificate_transition_recorded
    sample_juror_timeout_certificate_accounted

theorem sample_juror_timeout_certificate_outcome_facts :
    OutcomeCertificateFacts
      sampleJurorTimeoutCertificateInit
      sampleJurorTimeoutCertificateTransitions
      sampleJurorTimeoutCertificateState := by
  exact OutcomeCertificateFacts.jurorFailureVerdict "J1"
    sample_juror_timeout_certificate_facts

def certificateHungJuryTimeoutInitialCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-juror-timeout-hung"
    filed_on := "2026-06-06"
    status := "trial"
    trial_mode := "jury"
    phase := "deliberation"
    jury_configuration :=
      some { juror_count := 1, unanimous_required := true, minimum_concurring := 1 }
    jurors := [certificateOutcomeSwornJuror "J1"]
  }

def certificateHungJuryTimeoutInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := certificateHungJuryTimeoutInitialCase
  }

def sampleJurorTimeoutHungJuryCertificateInit : ReplayInitializeRequest :=
  { state := certificateHungJuryTimeoutInitialState
  , initialize_case := none
  }

def sampleJurorTimeoutHungJuryCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateJurorTimeoutAction]

def sampleJurorTimeoutHungJuryCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleJurorTimeoutHungJuryCertificateInit
      sampleJurorTimeoutHungJuryCertificateTransitions)

theorem sample_juror_timeout_hung_jury_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleJurorTimeoutHungJuryCertificateInit
        sampleJurorTimeoutHungJuryCertificateTransitions) = true := by
  native_decide

theorem sample_juror_timeout_hung_jury_certificate_replay :
    AcceptedReplayCertificate
      sampleJurorTimeoutHungJuryCertificateInit
      sampleJurorTimeoutHungJuryCertificateTransitions
      sampleJurorTimeoutHungJuryCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleJurorTimeoutHungJuryCertificateInit
      sampleJurorTimeoutHungJuryCertificateTransitions)
    sample_juror_timeout_hung_jury_certificate_replay_bool

theorem sample_juror_timeout_hung_jury_certificate_transition_recorded :
    replayTransitionsContainJurorTimeout
      sampleJurorTimeoutHungJuryCertificateTransitions
      "J1" = true := by
  native_decide

theorem sample_juror_timeout_hung_jury_certificate_accounted :
    jurorFailureHungJuryAccounted
      sampleJurorTimeoutHungJuryCertificateState
      "J1" = true := by
  native_decide

theorem sample_juror_timeout_hung_jury_certificate_facts :
    JurorFailureHungJuryCertificateFacts
      sampleJurorTimeoutHungJuryCertificateInit
      sampleJurorTimeoutHungJuryCertificateTransitions
      sampleJurorTimeoutHungJuryCertificateState
      "J1" := by
  exact acceptedReplayCertificate_juror_failure_hung_jury_facts
    sampleJurorTimeoutHungJuryCertificateInit
    sampleJurorTimeoutHungJuryCertificateTransitions
    sampleJurorTimeoutHungJuryCertificateState
    "J1"
    sample_juror_timeout_hung_jury_certificate_replay
    sample_juror_timeout_hung_jury_certificate_transition_recorded
    sample_juror_timeout_hung_jury_certificate_accounted

theorem sample_juror_timeout_hung_jury_certificate_outcome_facts :
    OutcomeCertificateFacts
      sampleJurorTimeoutHungJuryCertificateInit
      sampleJurorTimeoutHungJuryCertificateTransitions
      sampleJurorTimeoutHungJuryCertificateState := by
  exact OutcomeCertificateFacts.jurorFailureHungJury "J1"
    sample_juror_timeout_hung_jury_certificate_facts
