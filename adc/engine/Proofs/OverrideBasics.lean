import Main

theorem overrideSpecificity_le_two (o : LocalRuleOverrideV1) :
    overrideSpecificity o ≤ 2 := by
  unfold overrideSpecificity
  split <;> split <;> omega

theorem chooseOverride_none_is_candidate
    (candidate : LocalRuleOverrideV1) :
    chooseOverride none candidate = some candidate := by
  simp [chooseOverride]

theorem chooseOverride_isSome
    (current : Option LocalRuleOverrideV1)
    (candidate : LocalRuleOverrideV1) :
    (chooseOverride current candidate).isSome = true := by
  cases current with
  | none =>
      simp [chooseOverride]
  | some existing =>
      by_cases hgt : overrideSpecificity candidate > overrideSpecificity existing
      · simp [chooseOverride, hgt]
      · by_cases hlt : overrideSpecificity candidate < overrideSpecificity existing
        · simp [chooseOverride, hgt, hlt]
        · by_cases htime : existing.ordered_at ≤ candidate.ordered_at
          · simp [chooseOverride, hgt, hlt, htime]
          · simp [chooseOverride, hgt, hlt, htime]

theorem chooseOverride_prefers_higher_specificity
    (existing candidate : LocalRuleOverrideV1)
    (hgt : overrideSpecificity candidate > overrideSpecificity existing) :
    chooseOverride (some existing) candidate = some candidate := by
  simp [chooseOverride, hgt]

theorem chooseOverride_keeps_existing_on_lower_specificity
    (existing candidate : LocalRuleOverrideV1)
    (hlt : overrideSpecificity candidate < overrideSpecificity existing) :
    chooseOverride (some existing) candidate = some existing := by
  have hnotgt : ¬ overrideSpecificity candidate > overrideSpecificity existing :=
    Nat.not_lt.mpr (Nat.le_of_lt hlt)
  simp [chooseOverride, hnotgt, hlt]

theorem chooseOverride_tie_prefers_candidate_when_not_older
    (existing candidate : LocalRuleOverrideV1)
    (heq : overrideSpecificity candidate = overrideSpecificity existing)
    (htime : existing.ordered_at ≤ candidate.ordered_at) :
    chooseOverride (some existing) candidate = some candidate := by
  have hnotgt : ¬ overrideSpecificity candidate > overrideSpecificity existing := by
    rw [heq]
    exact Nat.lt_irrefl _
  have hnotlt : ¬ overrideSpecificity candidate < overrideSpecificity existing := by
    rw [heq]
    exact Nat.lt_irrefl _
  simp [chooseOverride, hnotgt, hnotlt, htime]

theorem chooseOverride_tie_prefers_existing_when_candidate_older
    (existing candidate : LocalRuleOverrideV1)
    (heq : overrideSpecificity candidate = overrideSpecificity existing)
    (hold : ¬ existing.ordered_at ≤ candidate.ordered_at) :
    chooseOverride (some existing) candidate = some existing := by
  have hnotgt : ¬ overrideSpecificity candidate > overrideSpecificity existing := by
    rw [heq]
    exact Nat.lt_irrefl _
  have hnotlt : ¬ overrideSpecificity candidate < overrideSpecificity existing := by
    rw [heq]
    exact Nat.lt_irrefl _
  simp [chooseOverride, hnotgt, hnotlt, hold]
