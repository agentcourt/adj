import Proofs.Reachability

namespace ArbdProofs

def replaySteps : ArbitrationState → List CourtAction → Except String ArbitrationState
  | s, [] => .ok s
  | s, action :: rest => do
      let t ← step { state := s, action := action }
      replaySteps t rest

def replayInitialized
    (req : InitializeCaseRequest)
    (actions : List CourtAction) :
    Except String ArbitrationState := do
  let start ← initializeCase req
  replaySteps start actions

def checkReplayCertificate
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState) :
    Except String Unit := do
  let replayed ← replayInitialized req actions
  if replayed = claimed then
    pure ()
  else
    throw "final state mismatch"

theorem initializeCase_deterministic
    (req : InitializeCaseRequest)
    (s t : ArbitrationState)
    (hs : initializeCase req = .ok s)
    (ht : initializeCase req = .ok t) :
    s = t := by
  rw [hs] at ht
  cases ht
  rfl

theorem step_deterministic
    (s t u : ArbitrationState)
    (action : CourtAction)
    (ht : step { state := s, action := action } = .ok t)
    (hu : step { state := s, action := action } = .ok u) :
    t = u := by
  rw [ht] at hu
  cases hu
  rfl

theorem replaySteps_concat_ok
    (start middle target : ArbitrationState)
    (actions : List CourtAction)
    (action : CourtAction)
    (hReplay : replaySteps start actions = .ok middle)
    (hStep : step { state := middle, action := action } = .ok target) :
    replaySteps start (actions.concat action) = .ok target := by
  induction actions generalizing start with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      simp [replaySteps, hStep]
      rfl
  | cons first rest ih =>
      simp [replaySteps] at hReplay ⊢
      cases hFirst : step { state := start, action := first } with
      | error err =>
          rw [hFirst] at hReplay
          cases hReplay
      | ok next =>
          rw [hFirst] at hReplay
          change replaySteps next (rest ++ [action]) = .ok target
          simpa only [List.concat_eq_append] using ih next hReplay

theorem replaySteps_success_stepReachableFrom_of_base
    (base current target : ArbitrationState)
    (actions : List CourtAction)
    (hBase : StepReachableFrom base current)
    (hReplay : replaySteps current actions = .ok target) :
    StepReachableFrom base target := by
  induction actions generalizing current with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      exact hBase
  | cons action rest ih =>
      simp [replaySteps] at hReplay
      cases hStep : step { state := current, action := action } with
      | error err =>
          rw [hStep] at hReplay
          cases hReplay
      | ok next =>
          rw [hStep] at hReplay
          exact ih next (StepReachableFrom.step current next action hBase hStep) hReplay

theorem replaySteps_success_stepReachableFrom
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hReplay : replaySteps start actions = .ok target) :
    StepReachableFrom start target := by
  exact replaySteps_success_stepReachableFrom_of_base
    start start target actions StepReachableFrom.refl hReplay

theorem stepReachableFrom_replaySteps_exists
    (start target : ArbitrationState)
    (hRun : StepReachableFrom start target) :
    ∃ actions, replaySteps start actions = .ok target := by
  induction hRun with
  | refl =>
      exact ⟨[], rfl⟩
  | step s t action _ hStep ih =>
      rcases ih with ⟨actions, hReplay⟩
      exact ⟨actions.concat action, replaySteps_concat_ok start s t actions action hReplay hStep⟩

theorem replaySteps_success_reachable
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hStart : Reachable start)
    (hReplay : replaySteps start actions = .ok target) :
    Reachable target := by
  exact stepReachableFrom_reachable start target hStart
    (replaySteps_success_stepReachableFrom start target actions hReplay)

theorem replayInitialized_success_components
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target) :
    ∃ start,
      initializeCase req = .ok start ∧
        replaySteps start actions = .ok target := by
  unfold replayInitialized at hReplay
  cases hInit : initializeCase req with
  | error err =>
      simp [hInit] at hReplay
      cases hReplay
  | ok start =>
      have hSteps : replaySteps start actions = .ok target := by
        rw [hInit] at hReplay
        change replaySteps start actions = .ok target at hReplay
        exact hReplay
      exact ⟨start, rfl, hSteps⟩

theorem replayInitialized_success_reachable
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target) :
    Reachable target := by
  rcases replayInitialized_success_components req actions target hReplay with
    ⟨start, hInit, hSteps⟩
  exact replaySteps_success_reachable start target actions
    (Reachable.init req start hInit) hSteps

theorem replayInitialized_success_stepReachableFrom
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target) :
    ∃ start,
      initializeCase req = .ok start ∧
        StepReachableFrom start target := by
  rcases replayInitialized_success_components req actions target hReplay with
    ⟨start, hInit, hSteps⟩
  exact ⟨start, hInit, replaySteps_success_stepReachableFrom start target actions hSteps⟩

theorem checkReplayCertificate_ok_iff
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState) :
    checkReplayCertificate req actions claimed = .ok () ↔
      replayInitialized req actions = .ok claimed := by
  constructor
  · intro hCheck
    unfold checkReplayCertificate at hCheck
    cases hReplay : replayInitialized req actions with
    | error err =>
        rw [hReplay] at hCheck
        cases hCheck
    | ok replayed =>
        rw [hReplay] at hCheck
        by_cases hEq : replayed = claimed
        · cases hEq
          rfl
        · change (if replayed = claimed then (Except.ok () : Except String Unit)
            else Except.error "final state mismatch") = Except.ok () at hCheck
          simp [hEq] at hCheck
  · intro hReplay
    unfold checkReplayCertificate
    rw [hReplay]
    change (if claimed = claimed then (Except.ok () : Except String Unit)
      else Except.error "final state mismatch") = Except.ok ()
    simp

theorem checkReplayCertificate_ok_reachable
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    Reachable claimed := by
  exact replayInitialized_success_reachable req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck)

theorem checkReplayCertificate_ok_stepReachableFrom
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    ∃ start,
      initializeCase req = .ok start ∧
        StepReachableFrom start claimed := by
  exact replayInitialized_success_stepReachableFrom req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck)

end ArbdProofs
