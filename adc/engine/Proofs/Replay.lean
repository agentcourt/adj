import Proofs.Reachability

def replayTransitions :
    CourtState → List ReplayTransition → Except String CourtState
  | state, [] => .ok state
  | state, transition :: rest => do
      let next ← replayTransition state transition
      replayTransitions next rest

def replayCertificate
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition) :
    Except String CourtState := do
  let start ← replayInitial init
  replayTransitions start transitions

def AcceptedReplayCertificate
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop :=
  replayCertificate init transitions = .ok claimed

theorem replayTransitions_concat_ok
    (start middle target : CourtState)
    (transitions : List ReplayTransition)
    (transition : ReplayTransition)
    (hReplay : replayTransitions start transitions = .ok middle)
    (hTransition : replayTransition middle transition = .ok target) :
    replayTransitions start (transitions.concat transition) = .ok target := by
  induction transitions generalizing start with
  | nil =>
      simp [replayTransitions] at hReplay
      cases hReplay
      simp [replayTransitions, hTransition]
      rfl
  | cons first rest ih =>
      simp [replayTransitions] at hReplay ⊢
      cases hFirst : replayTransition start first with
      | error err =>
          rw [hFirst] at hReplay
          cases hReplay
      | ok next =>
          rw [hFirst] at hReplay
          change replayTransitions next (rest ++ [transition]) = .ok target
          simpa only [List.concat_eq_append] using ih next hReplay

theorem replayTransitions_success_reachableFrom_of_base
    (base current target : CourtState)
    (transitions : List ReplayTransition)
    (hBase : ReplayReachableFrom base current)
    (hReplay : replayTransitions current transitions = .ok target) :
    ReplayReachableFrom base target := by
  induction transitions generalizing current with
  | nil =>
      simp [replayTransitions] at hReplay
      cases hReplay
      exact hBase
  | cons transition rest ih =>
      simp [replayTransitions] at hReplay
      cases hTransition : replayTransition current transition with
      | error err =>
          rw [hTransition] at hReplay
          cases hReplay
      | ok next =>
          rw [hTransition] at hReplay
          exact ih next
            (ReplayReachableFrom.transition current next transition hBase hTransition)
            hReplay

theorem replayTransitions_success_reachableFrom
    (start target : CourtState)
    (transitions : List ReplayTransition)
    (hReplay : replayTransitions start transitions = .ok target) :
    ReplayReachableFrom start target := by
  exact replayTransitions_success_reachableFrom_of_base
    start start target transitions ReplayReachableFrom.refl hReplay

theorem replayReachableFrom_replayTransitions_exists
    (start target : CourtState)
    (hReachable : ReplayReachableFrom start target) :
    ∃ transitions, replayTransitions start transitions = .ok target := by
  induction hReachable with
  | refl =>
      exact ⟨[], rfl⟩
  | transition current next transition _ hTransition ih =>
      rcases ih with ⟨transitions, hReplay⟩
      exact ⟨transitions.concat transition,
        replayTransitions_concat_ok start current next transitions transition hReplay hTransition⟩

theorem replayCertificate_success_components
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (target : CourtState)
    (hReplay : replayCertificate init transitions = .ok target) :
    ∃ start,
      replayInitial init = .ok start ∧
        replayTransitions start transitions = .ok target := by
  unfold replayCertificate at hReplay
  cases hInitial : replayInitial init with
  | error err =>
      simp [hInitial] at hReplay
      cases hReplay
  | ok start =>
      have hTransitions : replayTransitions start transitions = .ok target := by
        rw [hInitial] at hReplay
        change replayTransitions start transitions = .ok target at hReplay
        exact hReplay
      exact ⟨start, rfl, hTransitions⟩

theorem replayCertificate_success_reachableFrom
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (target : CourtState)
    (hReplay : replayCertificate init transitions = .ok target) :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start target := by
  rcases replayCertificate_success_components init transitions target hReplay with
    ⟨start, hInitial, hTransitions⟩
  exact ⟨start, hInitial,
    replayTransitions_success_reachableFrom start target transitions hTransitions⟩

theorem acceptedReplayCertificate_reachableFrom
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed) :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed := by
  exact replayCertificate_success_reachableFrom init transitions claimed hAccepted
