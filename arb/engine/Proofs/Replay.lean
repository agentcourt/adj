import Proofs.MaximalRuns

namespace ArbProofs

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
  | step s t action hs hStep ih =>
      rcases ih with ⟨actions, hReplay⟩
      exact ⟨actions.concat action, replaySteps_concat_ok start s t actions action hReplay hStep⟩

theorem stepReachableFrom_reachable
    (start target : ArbitrationState)
    (hStart : Reachable start)
    (hRun : StepReachableFrom start target) :
    Reachable target := by
  induction hRun with
  | refl =>
      exact hStart
  | step s t action hs hStep ih =>
      exact Reachable.step s t action ih hStep

theorem replaySteps_success_reachable
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hStart : Reachable start)
    (hReplay : replaySteps start actions = .ok target) :
    Reachable target := by
  exact stepReachableFrom_reachable start target hStart
    (replaySteps_success_stepReachableFrom start target actions hReplay)

theorem replaySteps_success_stepPath_of_base
    (base current target : ArbitrationState)
    (n : Nat)
    (actions : List CourtAction)
    (hBase : StepPath base n current)
    (hReplay : replaySteps current actions = .ok target) :
    StepPath base (n + actions.length) target := by
  induction actions generalizing current n with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      simpa using hBase
  | cons action rest ih =>
      simp [replaySteps] at hReplay
      cases hStep : step { state := current, action := action } with
      | error err =>
          rw [hStep] at hReplay
          cases hReplay
      | ok next =>
          rw [hStep] at hReplay
          have hBaseNext : StepPath base (n + 1) next :=
            StepPath.step n current next action hBase hStep
          have hRest := ih next (n + 1) hBaseNext hReplay
          simpa [Nat.add_assoc, Nat.add_comm, Nat.add_left_comm] using hRest

theorem replaySteps_success_stepPath
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hReplay : replaySteps start actions = .ok target) :
    StepPath start actions.length target := by
  simpa using replaySteps_success_stepPath_of_base
    start start target 0 actions StepPath.refl hReplay

theorem stepPath_replaySteps_exists
    (start target : ArbitrationState)
    (n : Nat)
    (hPath : StepPath start n target) :
    ∃ actions, actions.length = n ∧ replaySteps start actions = .ok target := by
  induction hPath with
  | refl =>
      exact ⟨[], rfl, rfl⟩
  | step n s t action hs hStep ih =>
      rcases ih with ⟨actions, hLength, hReplay⟩
      exact ⟨actions.concat action, by simp [hLength],
        replaySteps_concat_ok start s t actions action hReplay hStep⟩

theorem replaySteps_length_le_initializedBudget
    (req : InitializeCaseRequest)
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hInit : initializeCase req = .ok start)
    (hReplay : replaySteps start actions = .ok target) :
    actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
      8 + start.policy.max_deliberation_rounds * start.policy.council_size := by
  exact stepPath_length_le_initializedBudget req start target actions.length hInit
    (replaySteps_success_stepPath start target actions hReplay)

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

theorem replayInitialized_success_stepPath
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target) :
    ∃ start,
      initializeCase req = .ok start ∧
        StepPath start actions.length target := by
  rcases replayInitialized_success_components req actions target hReplay with
    ⟨start, hInit, hSteps⟩
  exact ⟨start, hInit, replaySteps_success_stepPath start target actions hSteps⟩

theorem replayInitialized_length_le_initializedBudget
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target) :
    ∃ start,
      initializeCase req = .ok start ∧
        actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
          8 + start.policy.max_deliberation_rounds * start.policy.council_size := by
  rcases replayInitialized_success_components req actions target hReplay with
    ⟨start, hInit, hSteps⟩
  exact ⟨start, hInit, replaySteps_length_le_initializedBudget
    req start target actions hInit hSteps⟩

theorem replayInitialized_blocked_terminal_accounted
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target)
    (hBlocked : stepBlocked target) :
    (target.case.status = "closed" ∧
        target.case.phase = "closed" ∧
          (target.case.resolution = "demonstrated" ∨
            target.case.resolution = "not_demonstrated" ∨
              target.case.resolution = "no_majority")) ∨
      (target.case.status = "failed" ∧
        ∃ failure,
          target.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = target.case.phase) := by
  rcases replayInitialized_success_components req actions target hReplay with
    ⟨start, hInit, hSteps⟩
  exact initializedStepPathMaximal_terminal_accounted
    req start target actions.length hInit
    ⟨replaySteps_success_stepPath start target actions hSteps, hBlocked⟩

theorem replayInitialized_terminal_status_accounted
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (target : ArbitrationState)
    (hReplay : replayInitialized req actions = .ok target)
    (hTerminal : target.case.status = "closed" ∨ target.case.status = "failed") :
    (target.case.status = "closed" ∧
        target.case.phase = "closed" ∧
          (target.case.resolution = "demonstrated" ∨
            target.case.resolution = "not_demonstrated" ∨
              target.case.resolution = "no_majority")) ∨
      (target.case.status = "failed" ∧
        ∃ failure,
          target.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = target.case.phase) := by
  have hReachable := replayInitialized_success_reachable req actions target hReplay
  rcases hTerminal with hClosed | hFailed
  · have hPhase := reachable_status_closed_implies_phase_closed target hReachable hClosed
    have hResolution := reachable_closed_resolution_enum target hReachable hPhase
    exact Or.inl ⟨hClosed, hPhase, hResolution⟩
  · have hFailure := reachable_failed_has_failure target hReachable hFailed
    exact Or.inr ⟨hFailed, hFailure⟩

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

theorem checkReplayCertificate_ok_stepPath
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    ∃ start,
      initializeCase req = .ok start ∧
        StepPath start actions.length claimed := by
  exact replayInitialized_success_stepPath req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck)

theorem checkReplayCertificate_ok_length_le_initializedBudget
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ()) :
    ∃ start,
      initializeCase req = .ok start ∧
        actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
          8 + start.policy.max_deliberation_rounds * start.policy.council_size := by
  exact replayInitialized_length_le_initializedBudget req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck)

theorem checkReplayCertificate_ok_blocked_terminal_accounted
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hBlocked : stepBlocked claimed) :
    (claimed.case.status = "closed" ∧
        claimed.case.phase = "closed" ∧
          (claimed.case.resolution = "demonstrated" ∨
            claimed.case.resolution = "not_demonstrated" ∨
              claimed.case.resolution = "no_majority")) ∨
      (claimed.case.status = "failed" ∧
        ∃ failure,
          claimed.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = claimed.case.phase) := by
  exact replayInitialized_blocked_terminal_accounted req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck) hBlocked

theorem checkReplayCertificate_ok_terminal_status_accounted
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hTerminal : claimed.case.status = "closed" ∨ claimed.case.status = "failed") :
    (claimed.case.status = "closed" ∧
        claimed.case.phase = "closed" ∧
          (claimed.case.resolution = "demonstrated" ∨
            claimed.case.resolution = "not_demonstrated" ∨
              claimed.case.resolution = "no_majority")) ∨
      (claimed.case.status = "failed" ∧
        ∃ failure,
          claimed.case.failure = some failure ∧
            failure.failure_type = "opportunity_failed" ∧
              (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
                failure.phase = claimed.case.phase) := by
  exact replayInitialized_terminal_status_accounted req actions claimed
    ((checkReplayCertificate_ok_iff req actions claimed).1 hCheck) hTerminal

end ArbProofs
