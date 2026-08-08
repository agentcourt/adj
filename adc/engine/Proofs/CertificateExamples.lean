import Proofs.CertificateFacts

def certificateExampleBaseCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-case-1"
    filed_on := "2026-01-01"
    status := "pretrial"
    phase := "discovery"
  }

def certificateExampleInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    case := certificateExampleBaseCase
  }

def certificateExampleDismissRule41Action : CourtAction :=
  { action_type := "dismiss_case_rule41"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("with_prejudice", Lean.Json.bool false)
      , ("reason", Lean.Json.str "stipulated dismissal")
      ]
  }

def sampleClosedCertificateInit : ReplayInitializeRequest :=
  { state := certificateExampleInitialState
  , initialize_case := none
  }

def sampleClosedCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateExampleDismissRule41Action]

def sampleClosedCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleClosedCertificateInit
      sampleClosedCertificateTransitions)

theorem sample_closed_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleClosedCertificateInit
        sampleClosedCertificateTransitions) = true := by
  native_decide

theorem sample_closed_certificate_replay :
    AcceptedReplayCertificate
      sampleClosedCertificateInit
      sampleClosedCertificateTransitions
      sampleClosedCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleClosedCertificateInit
      sampleClosedCertificateTransitions)
    sample_closed_certificate_replay_bool

theorem sample_closed_certificate_status :
    sampleClosedCertificateState.case.status = "closed" := by
  native_decide

theorem sample_closed_certificate_terminal :
    terminalClosedAccounted sampleClosedCertificateState := by
  exact terminalClosedAccounted_of_status_closed
    sampleClosedCertificateState
    sample_closed_certificate_status

theorem sample_closed_certificate_facts :
    ClosedCertificateFacts
      sampleClosedCertificateInit
      sampleClosedCertificateTransitions
      sampleClosedCertificateState := by
  exact acceptedReplayCertificate_status_closed_facts
    sampleClosedCertificateInit
    sampleClosedCertificateTransitions
    sampleClosedCertificateState
    sample_closed_certificate_replay
    sample_closed_certificate_status
