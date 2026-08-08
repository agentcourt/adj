import Proofs.CertificateFacts
import Proofs.Samples

namespace ArbdProofs

def sampleClosedCertificateActions : List CourtAction :=
  [ openingAction "plaintiff" "Plaintiff opening."
  , openingAction "defendant" "Defendant opening."
  , argumentAction "plaintiff" "Plaintiff argument."
  , argumentAction "defendant" "Defendant argument."
  , passAction "plaintiff"
  , passAction "defendant"
  , closingAction "plaintiff" "Plaintiff closing."
  , closingAction "defendant" "Defendant closing."
  , councilAnswerAction "C1" 72 "first answer"
  , councilAnswerAction "C2" 55 "second answer"
  , councilAnswerAction "C3" 18 "third answer"
  ]

def sampleClosedCertificateState : ArbitrationState :=
  match replayInitialized initRequest sampleClosedCertificateActions with
  | .ok state => state
  | .error _ => default

def certificateCheckAccepted : Except String Unit → Bool
  | .ok () => true
  | .error _ => false

theorem certificateCheckAccepted_eq_true
    (result : Except String Unit) :
    certificateCheckAccepted result = true ↔ result = .ok () := by
  cases result with
  | ok value =>
      cases value
      simp [certificateCheckAccepted]
  | error err =>
      simp [certificateCheckAccepted]

theorem sample_closed_certificate_check_bool :
    certificateCheckAccepted
      (checkReplayCertificate
        initRequest
        sampleClosedCertificateActions
        sampleClosedCertificateState) = true := by
  native_decide

theorem sample_closed_certificate_check :
    checkReplayCertificate
      initRequest
      sampleClosedCertificateActions
      sampleClosedCertificateState = .ok () := by
  exact (certificateCheckAccepted_eq_true _).1 sample_closed_certificate_check_bool

theorem sample_closed_certificate_status :
    sampleClosedCertificateState.case.status = "closed" ∧
      sampleClosedCertificateState.case.phase = "closed" := by
  native_decide

theorem sample_closed_certificate_answer_pairs :
    reportedAnswerPairs sampleClosedCertificateState =
      [("C1", 72), ("C2", 55), ("C3", 18)] := by
  native_decide

theorem sample_closed_certificate_facts :
    ClosedCertificateFacts
      initRequest
      sampleClosedCertificateActions
      sampleClosedCertificateState := by
  exact checkReplayCertificate_status_closed_facts
    initRequest
    sampleClosedCertificateActions
    sampleClosedCertificateState
    sample_closed_certificate_check
    sample_closed_certificate_status.1

def sampleFailedCertificateActions : List CourtAction :=
  [ failOpportunityAction
      "openings:plaintiff"
      "plaintiff"
      "openings"
      "agent_error"
      "plaintiff agent failed"
      "model-p"
  ]

def sampleFailedCertificateState : ArbitrationState :=
  match replayInitialized initRequest sampleFailedCertificateActions with
  | .ok state => state
  | .error _ => default

theorem sample_failed_certificate_check_bool :
    certificateCheckAccepted
      (checkReplayCertificate
        initRequest
        sampleFailedCertificateActions
        sampleFailedCertificateState) = true := by
  native_decide

theorem sample_failed_certificate_check :
    checkReplayCertificate
      initRequest
      sampleFailedCertificateActions
      sampleFailedCertificateState = .ok () := by
  exact (certificateCheckAccepted_eq_true _).1 sample_failed_certificate_check_bool

theorem sample_failed_certificate_status :
    sampleFailedCertificateState.case.status = "failed" ∧
      sampleFailedCertificateState.case.phase = "openings" ∧
        (nextOpportunity sampleFailedCertificateState).terminal = true ∧
          (nextOpportunity sampleFailedCertificateState).reason = "agent_error" := by
  native_decide

theorem sample_failed_certificate_failure_record :
    reportedFailureRecord sampleFailedCertificateState =
      some
        { failure_type := "opportunity_failed"
        , role := "plaintiff"
        , phase := "openings"
        , opportunity_id := "openings:plaintiff"
        , reason := "agent_error"
        , message := "plaintiff agent failed"
        , member_id := ""
        , model := "model-p"
        } := by
  native_decide

theorem sample_failed_certificate_facts :
    FailedCertificateFacts
      initRequest
      sampleFailedCertificateActions
      sampleFailedCertificateState := by
  exact checkReplayCertificate_status_failed_facts
    initRequest
    sampleFailedCertificateActions
    sampleFailedCertificateState
    sample_failed_certificate_check
    sample_failed_certificate_status.1

end ArbdProofs
