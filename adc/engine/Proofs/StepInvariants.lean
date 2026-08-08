import Main

def statusRank : String → Nat
  | "filed" => 0
  | "pretrial" => 1
  | "trial" => 2
  | "judgment_entered" => 3
  | "closed" => 4
  | _ => 99

theorem checkTransition_closed_false (next : String) :
    checkTransition "closed" next = false := by
  simp [checkTransition]

theorem checkTransition_judgment_entered_eq_closed (next : String) :
    checkTransition "judgment_entered" next = (next = "closed") := by
  simp [checkTransition]

theorem checkTransition_filed_characterization (next : String) :
    checkTransition "filed" next = (next = "pretrial" || next = "closed") := by
  simp [checkTransition]

theorem checkTransition_pretrial_characterization (next : String) :
    checkTransition "pretrial" next = (next = "trial" || next = "closed") := by
  simp [checkTransition]

theorem checkTransition_trial_characterization (next : String) :
    checkTransition "trial" next = (next = "judgment_entered" || next = "closed") := by
  simp [checkTransition]

theorem checkTransition_filed_not_trial :
    checkTransition "filed" "trial" = false := by
  simp [checkTransition]

theorem checkTransition_filed_true_implies_allowed_next (next : String) :
    checkTransition "filed" next = true -> allowedStatuses.contains next = true := by
  intro h
  have hnext : next = "pretrial" ∨ next = "closed" := by
    simpa [checkTransition] using h
  cases hnext with
  | inl hp =>
      simp [allowedStatuses, hp]
  | inr hc =>
      simp [allowedStatuses, hc]

theorem checkTransition_pretrial_true_implies_allowed_next (next : String) :
    checkTransition "pretrial" next = true -> allowedStatuses.contains next = true := by
  intro h
  have hnext : next = "trial" ∨ next = "closed" := by
    simpa [checkTransition] using h
  cases hnext with
  | inl ht =>
      simp [allowedStatuses, ht]
  | inr hc =>
      simp [allowedStatuses, hc]

theorem checkTransition_trial_true_implies_allowed_next (next : String) :
    checkTransition "trial" next = true -> allowedStatuses.contains next = true := by
  intro h
  have hnext : next = "judgment_entered" ∨ next = "closed" := by
    simpa [checkTransition] using h
  cases hnext with
  | inl hj =>
      simp [allowedStatuses, hj]
  | inr hc =>
      simp [allowedStatuses, hc]

theorem checkTransition_judgment_entered_true_implies_allowed_next (next : String) :
    checkTransition "judgment_entered" next = true -> allowedStatuses.contains next = true := by
  intro h
  have hnext : next = "closed" := by
    simpa [checkTransition] using h
  simp [allowedStatuses, hnext]

theorem checkTransition_true_implies_current_not_closed (current next : String) :
    checkTransition current next = true -> current ≠ "closed" := by
  intro h
  intro hc
  subst hc
  simp [checkTransition] at h

theorem checkTransition_true_implies_current_allowed (current next : String) :
    checkTransition current next = true -> allowedStatuses.contains current = true := by
  intro h
  by_cases hf : current = "filed"
  · simp [allowedStatuses, hf]
  · by_cases hp : current = "pretrial"
    · simp [allowedStatuses, hp]
    · by_cases ht : current = "trial"
      · simp [allowedStatuses, ht]
      · by_cases hj : current = "judgment_entered"
        · simp [allowedStatuses, hj]
        · have hfalse : checkTransition current next = false := by
            simp [checkTransition, hf, hp, ht, hj]
          rw [hfalse] at h
          cases h

theorem checkTransition_true_implies_statusRank_lt (current next : String) :
    checkTransition current next = true -> statusRank current < statusRank next := by
  intro h
  by_cases hf : current = "filed"
  · subst hf
    have hnext : next = "pretrial" ∨ next = "closed" := by
      simpa [checkTransition] using h
    cases hnext with
    | inl hp =>
        simp [statusRank, hp]
    | inr hc =>
        simp [statusRank, hc]
  · by_cases hp : current = "pretrial"
    · subst hp
      have hnext : next = "trial" ∨ next = "closed" := by
        simpa [checkTransition] using h
      cases hnext with
      | inl ht =>
          simp [statusRank, ht]
      | inr hc =>
          simp [statusRank, hc]
    · by_cases ht : current = "trial"
      · subst ht
        have hnext : next = "judgment_entered" ∨ next = "closed" := by
          simpa [checkTransition] using h
        cases hnext with
        | inl hj =>
            simp [statusRank, hj]
        | inr hc =>
            simp [statusRank, hc]
      · by_cases hj : current = "judgment_entered"
        · subst hj
          have hnext : next = "closed" := by
            simpa [checkTransition] using h
          simp [statusRank, hnext]
        · have hfalse : checkTransition current next = false := by
            simp [checkTransition, hf, hp, ht, hj]
          rw [hfalse] at h
          cases h

theorem allowedPhases_contains_implies_phaseOrder_le_twelve (phase : String) :
    allowedPhases.contains phase = true -> phaseOrder phase ≤ 12 := by
  intro h
  have hphase :
      phase = "none" ∨
      phase = "voir_dire" ∨
      phase = "openings" ∨
      phase = "plaintiff_case" ∨
      phase = "defense_case" ∨
      phase = "plaintiff_rebuttal" ∨
      phase = "defense_surrebuttal" ∨
      phase = "charge_conference" ∨
      phase = "closings" ∨
      phase = "jury_charge" ∨
      phase = "deliberation" ∨
      phase = "verdict_return" ∨
      phase = "post_verdict" := by
    simpa [allowedPhases] using h
  rcases hphase with hnone | hvd | hopen | hpl | hdef | hpr | hds | hcc | hcl | hjc | hdel | hvret | hpost
  · simp [phaseOrder, hnone]
  · simp [phaseOrder, hvd]
  · simp [phaseOrder, hopen]
  · simp [phaseOrder, hpl]
  · simp [phaseOrder, hdef]
  · simp [phaseOrder, hpr]
  · simp [phaseOrder, hds]
  · simp [phaseOrder, hcc]
  · simp [phaseOrder, hcl]
  · simp [phaseOrder, hjc]
  · simp [phaseOrder, hdel]
  · simp [phaseOrder, hvret]
  · simp [phaseOrder, hpost]

theorem allowedPhases_contains_implies_phaseOrder_ne_fallback (phase : String) :
    allowedPhases.contains phase = true -> phaseOrder phase ≠ 999 := by
  intro h
  have hle : phaseOrder phase ≤ 12 := allowedPhases_contains_implies_phaseOrder_le_twelve phase h
  intro h999
  rw [h999] at hle
  omega

theorem allowedStatuses_contains_implies_statusRank_le_four (status : String) :
    allowedStatuses.contains status = true -> statusRank status ≤ 4 := by
  intro h
  have hstatus :
      status = "filed" ∨
      status = "pretrial" ∨
      status = "trial" ∨
      status = "judgment_entered" ∨
      status = "closed" := by
    simpa [allowedStatuses] using h
  rcases hstatus with hf | hp | ht | hj | hc
  · simp [statusRank, hf]
  · simp [statusRank, hp]
  · simp [statusRank, ht]
  · simp [statusRank, hj]
  · simp [statusRank, hc]

theorem allowedStatuses_contains_implies_statusRank_ne_fallback (status : String) :
    allowedStatuses.contains status = true -> statusRank status ≠ 99 := by
  intro h
  have hle : statusRank status ≤ 4 := allowedStatuses_contains_implies_statusRank_le_four status h
  intro h99
  rw [h99] at hle
  omega

theorem checkTransition_true_implies_next_allowed (current next : String) :
    checkTransition current next = true -> allowedStatuses.contains next = true := by
  intro h
  by_cases hf : current = "filed"
  · subst hf
    exact checkTransition_filed_true_implies_allowed_next next h
  · by_cases hp : current = "pretrial"
    · subst hp
      exact checkTransition_pretrial_true_implies_allowed_next next h
    · by_cases ht : current = "trial"
      · subst ht
        exact checkTransition_trial_true_implies_allowed_next next h
      · by_cases hj : current = "judgment_entered"
        · subst hj
          exact checkTransition_judgment_entered_true_implies_allowed_next next h
        · have hfalse : checkTransition current next = false := by
            simp [checkTransition, hf, hp, ht, hj]
          rw [hfalse] at h
          cases h

theorem checkTransition_true_implies_both_allowed (current next : String) :
    checkTransition current next = true ->
      allowedStatuses.contains current = true ∧ allowedStatuses.contains next = true := by
  intro h
  exact And.intro
    (checkTransition_true_implies_current_allowed current next h)
    (checkTransition_true_implies_next_allowed current next h)

theorem checkTransition_true_implies_current_ne_next (current next : String) :
    checkTransition current next = true -> current ≠ next := by
  intro h
  have hlt : statusRank current < statusRank next :=
    checkTransition_true_implies_statusRank_lt current next h
  intro heq
  rw [heq] at hlt
  exact Nat.lt_irrefl _ hlt

theorem checkTransition_true_implies_next_not_filed (current next : String) :
    checkTransition current next = true -> next ≠ "filed" := by
  intro h
  have hlt : statusRank current < statusRank next :=
    checkTransition_true_implies_statusRank_lt current next h
  intro hnext
  rw [hnext] at hlt
  simp [statusRank] at hlt

theorem checkTransition_true_implies_reverse_false (current next : String) :
    checkTransition current next = true -> checkTransition next current = false := by
  intro h
  have hlt : statusRank current < statusRank next :=
    checkTransition_true_implies_statusRank_lt current next h
  by_cases hrev : checkTransition next current = true
  · have hltRev : statusRank next < statusRank current :=
      checkTransition_true_implies_statusRank_lt next current hrev
    have : False := Nat.lt_asymm hlt hltRev
    exact False.elim this
  · cases hval : checkTransition next current with
    | false =>
        rfl
    | true =>
        exfalso
        exact hrev hval
