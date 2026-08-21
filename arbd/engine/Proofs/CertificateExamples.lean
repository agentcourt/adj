import Proofs.CertificateFacts
import Proofs.Samples

namespace ArbdProofs

def appendAcceptedAction
    (state : ArbitrationState)
    (actions : List CourtAction)
    (action : CourtAction) : Except String (ArbitrationState × List CourtAction) := do
  let next ← step { state := state, action := action }
  pure (next, actions.concat action)

def sampleClosedCertificateRun : Except String (ArbitrationState × List CourtAction) := do
  let s0 ← initializeCase initRequest
  let (s1, a1) ← appendAcceptedAction s0 []
    (openingAction s0 "plaintiff" "Plaintiff opening.")
  let (s2, a2) ← appendAcceptedAction s1 a1
    (openingAction s1 "defendant" "Defendant opening.")
  let (s3, a3) ← appendAcceptedAction s2 a2
    (argumentAction s2 "plaintiff" "Plaintiff argument.")
  let (s4, a4) ← appendAcceptedAction s3 a3
    (argumentAction s3 "defendant" "Defendant argument.")
  let (s5, a5) ← appendAcceptedAction s4 a4 (passAction s4 "plaintiff")
  let (s6, a6) ← appendAcceptedAction s5 a5 (passAction s5 "defendant")
  let (s7, a7) ← appendAcceptedAction s6 a6
    (closingAction s6 "plaintiff" "Plaintiff closing.")
  let (s8, a8) ← appendAcceptedAction s7 a7
    (closingAction s7 "defendant" "Defendant closing.")
  let (s9, a9) ← appendAcceptedAction s8 a8
    (councilAnswerAction s8 "C1" 72 "first answer")
  let (s10, a10) ← appendAcceptedAction s9 a9
    (councilAnswerAction s9 "C2" 55 "second answer")
  appendAcceptedAction s10 a10
    (councilAnswerAction s10 "C3" 18 "third answer")

def sampleClosedCertificateActions : List CourtAction :=
  match sampleClosedCertificateRun with
  | .ok (_, actions) => actions
  | .error _ => []

def sampleClosedCertificateState : ArbitrationState :=
  match sampleClosedCertificateRun with
  | .ok (state, _) => state
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

def sampleFailedCertificateRun : Except String (ArbitrationState × List CourtAction) := do
  let start ← initializeCase initRequest
  appendAcceptedAction start []
    (failOpportunityAction
      start
      "openings:plaintiff"
      "plaintiff"
      "openings"
      "agent_error"
      "plaintiff agent failed"
      "model-p")

def sampleFailedCertificateActions : List CourtAction :=
  match sampleFailedCertificateRun with
  | .ok (_, actions) => actions
  | .error _ => []

def sampleFailedCertificateState : ArbitrationState :=
  match sampleFailedCertificateRun with
  | .ok (state, _) => state
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
