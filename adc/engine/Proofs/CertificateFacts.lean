import Proofs.Replay
import Proofs.OrchestrationCore

def terminalClosedAccounted (state : CourtState) : Prop :=
  state.case.status = "closed" ∧
    ∀ roles maxSteps,
      (nextOpportunity
        { state := state
        , roles := roles
        , max_steps_per_turn := maxSteps }).terminal = true

structure ClosedCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  terminal_accounted :
    terminalClosedAccounted claimed

def TerminalCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop :=
  ClosedCertificateFacts init transitions claimed

def certificateReplayAccepted : Except String CourtState → Bool
  | .ok _ => true
  | .error _ => false

def certificateStateOrDefault : Except String CourtState → CourtState
  | .ok state => state
  | .error _ => default

theorem certificateStateOrDefault_ok
    (result : Except String CourtState)
    (hAccepted : certificateReplayAccepted result = true) :
    result = .ok (certificateStateOrDefault result) := by
  cases result with
  | ok state =>
      rfl
  | error err =>
      simp [certificateReplayAccepted] at hAccepted

theorem terminalClosedAccounted_of_status_closed
    (state : CourtState)
    (hStatus : state.case.status = "closed") :
    terminalClosedAccounted state := by
  constructor
  · exact hStatus
  · intro roles maxSteps
    exact nextOpportunity_terminal_when_case_closed
      { state := state
      , roles := roles
      , max_steps_per_turn := maxSteps }
      hStatus

theorem acceptedReplayCertificate_status_closed_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hStatus : claimed.case.status = "closed") :
    ClosedCertificateFacts init transitions claimed := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      terminal_accounted :=
        terminalClosedAccounted_of_status_closed claimed hStatus }

theorem acceptedReplayCertificate_terminal_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hStatus : claimed.case.status = "closed") :
    TerminalCertificateFacts init transitions claimed :=
  acceptedReplayCertificate_status_closed_facts
    init transitions claimed hAccepted hStatus
