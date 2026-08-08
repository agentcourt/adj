import Proofs.CertificateFacts

def juryVerdictAccounted (state : CourtState) : Bool :=
  match state.case.jury_verdict with
  | none => false
  | some verdict =>
      (verdict.verdict_for == "plaintiff" || verdict.verdict_for == "defendant") &&
        decide (verdict.required_votes ≤ verdict.votes_for_verdict) &&
          (!(verdict.verdict_for == "defendant") ||
            verdict.damages.toBits == (0.0).toBits)

def judgmentFromVerdictAccounted (state : CourtState) : Bool :=
  match state.case.jury_verdict with
  | none => false
  | some verdict =>
      state.case.status == "judgment_entered" &&
        state.case.monetary_judgment.toBits == verdict.damages.toBits &&
          hasDecisionTraceAction state.case "enter_judgment"

def replayTransitionProcessJurorTimeoutFor
    (transition : ReplayTransition)
    (jurorId : String) :
    Bool :=
  match transition with
  | ReplayTransition.step action =>
      action.action_type == "process_juror_timeout" &&
        action.actor_role == "system" &&
          match getString action.payload "juror_id" with
          | .ok actual => actual == jurorId
          | .error _ => false
  | ReplayTransition.applyDecision _ => false

def replayTransitionsContainJurorTimeout
    (transitions : List ReplayTransition)
    (jurorId : String) :
    Bool :=
  transitions.any (fun transition =>
    replayTransitionProcessJurorTimeoutFor transition jurorId)

def timedOutJurorRecorded (state : CourtState) (jurorId : String) : Bool :=
  state.case.jurors.any (fun juror =>
    juror.juror_id == jurorId && juror.status == "timed_out")

def verdictUsesEffectiveConcurrence (state : CourtState) : Bool :=
  match state.case.jury_configuration, state.case.jury_verdict with
  | some cfg, some verdict =>
      verdict.required_votes ==
        effectiveMinimumConcurring cfg (countJurorsByStatus state.case.jurors "sworn")
  | _, _ => false

def jurorFailureVerdictAccounted (state : CourtState) (jurorId : String) : Bool :=
  timedOutJurorRecorded state jurorId &&
    verdictUsesEffectiveConcurrence state &&
      juryVerdictAccounted state

def hungJuryAccounted (state : CourtState) : Bool :=
  match state.case.hung_jury with
  | none => false
  | some hung =>
      state.case.jury_verdict.isNone &&
        hung.claim_id != "" &&
          hung.note != ""

def jurorFailureHungJuryAccounted (state : CourtState) (jurorId : String) : Bool :=
  timedOutJurorRecorded state jurorId &&
    hungJuryAccounted state

structure VerdictCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  verdict_accounted :
    juryVerdictAccounted claimed = true

structure JudgmentCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  judgment_accounted :
    judgmentFromVerdictAccounted claimed = true

structure JurorFailureVerdictCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  timeout_transition_recorded :
    replayTransitionsContainJurorTimeout transitions jurorId = true
  juror_failure_verdict_accounted :
    jurorFailureVerdictAccounted claimed jurorId = true

structure JurorFailureHungJuryCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  timeout_transition_recorded :
    replayTransitionsContainJurorTimeout transitions jurorId = true
  juror_failure_hung_jury_accounted :
    jurorFailureHungJuryAccounted claimed jurorId = true

inductive OutcomeCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop where
  | closed :
      ClosedCertificateFacts init transitions claimed →
        OutcomeCertificateFacts init transitions claimed
  | verdict :
      VerdictCertificateFacts init transitions claimed →
        OutcomeCertificateFacts init transitions claimed
  | jurorFailureVerdict :
      (jurorId : String) →
        JurorFailureVerdictCertificateFacts init transitions claimed jurorId →
          OutcomeCertificateFacts init transitions claimed
  | jurorFailureHungJury :
      (jurorId : String) →
        JurorFailureHungJuryCertificateFacts init transitions claimed jurorId →
          OutcomeCertificateFacts init transitions claimed
  | judgment :
      JudgmentCertificateFacts init transitions claimed →
        OutcomeCertificateFacts init transitions claimed

theorem acceptedReplayCertificate_status_closed_outcome_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hStatus : claimed.case.status = "closed") :
    OutcomeCertificateFacts init transitions claimed := by
  exact OutcomeCertificateFacts.closed
    (acceptedReplayCertificate_status_closed_facts
      init transitions claimed hAccepted hStatus)

theorem acceptedReplayCertificate_verdict_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hVerdict : juryVerdictAccounted claimed = true) :
    VerdictCertificateFacts init transitions claimed := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      verdict_accounted := hVerdict }

theorem acceptedReplayCertificate_verdict_outcome_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hVerdict : juryVerdictAccounted claimed = true) :
    OutcomeCertificateFacts init transitions claimed := by
  exact OutcomeCertificateFacts.verdict
    (acceptedReplayCertificate_verdict_facts
      init transitions claimed hAccepted hVerdict)

theorem acceptedReplayCertificate_judgment_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hJudgment : judgmentFromVerdictAccounted claimed = true) :
    JudgmentCertificateFacts init transitions claimed := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      judgment_accounted := hJudgment }

theorem acceptedReplayCertificate_judgment_outcome_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hJudgment : judgmentFromVerdictAccounted claimed = true) :
    OutcomeCertificateFacts init transitions claimed := by
  exact OutcomeCertificateFacts.judgment
    (acceptedReplayCertificate_judgment_facts
      init transitions claimed hAccepted hJudgment)

theorem acceptedReplayCertificate_juror_failure_verdict_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hTimeout : replayTransitionsContainJurorTimeout transitions jurorId = true)
    (hVerdict : jurorFailureVerdictAccounted claimed jurorId = true) :
    JurorFailureVerdictCertificateFacts init transitions claimed jurorId := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      timeout_transition_recorded := hTimeout
      juror_failure_verdict_accounted := hVerdict }

theorem acceptedReplayCertificate_juror_failure_verdict_outcome_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hTimeout : replayTransitionsContainJurorTimeout transitions jurorId = true)
    (hVerdict : jurorFailureVerdictAccounted claimed jurorId = true) :
    OutcomeCertificateFacts init transitions claimed := by
  exact OutcomeCertificateFacts.jurorFailureVerdict jurorId
    (acceptedReplayCertificate_juror_failure_verdict_facts
      init transitions claimed jurorId hAccepted hTimeout hVerdict)

theorem acceptedReplayCertificate_juror_failure_hung_jury_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hTimeout : replayTransitionsContainJurorTimeout transitions jurorId = true)
    (hHung : jurorFailureHungJuryAccounted claimed jurorId = true) :
    JurorFailureHungJuryCertificateFacts init transitions claimed jurorId := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      timeout_transition_recorded := hTimeout
      juror_failure_hung_jury_accounted := hHung }

theorem acceptedReplayCertificate_juror_failure_hung_jury_outcome_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hTimeout : replayTransitionsContainJurorTimeout transitions jurorId = true)
    (hHung : jurorFailureHungJuryAccounted claimed jurorId = true) :
    OutcomeCertificateFacts init transitions claimed := by
  exact OutcomeCertificateFacts.jurorFailureHungJury jurorId
    (acceptedReplayCertificate_juror_failure_hung_jury_facts
      init transitions claimed jurorId hAccepted hTimeout hHung)
