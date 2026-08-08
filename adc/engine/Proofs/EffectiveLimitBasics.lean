import Main

theorem effectiveLimitValue_no_overrides_eq_policy
    (s : CourtState) (limitKey actor phase nowIso : String) (n : Nat)
    (hOverrides : s.case.local_rule_overrides = [])
    (hPolicy : policyLimitValue s.policy limitKey = some n) :
    effectiveLimitValue s limitKey actor phase nowIso = .ok n := by
  unfold effectiveLimitValue
  rw [hPolicy]
  simp [hOverrides]
  rfl

theorem effectiveLimitValue_unknown_key_error
    (s : CourtState) (limitKey actor phase nowIso : String)
    (hPolicy : policyLimitValue s.policy limitKey = none) :
    effectiveLimitValue s limitKey actor phase nowIso =
      .error s!"unknown local-rule limit key: {limitKey}" := by
  unfold effectiveLimitValue
  rw [hPolicy]
  rfl

theorem enforceMeasuredLimit_no_overrides_ok_of_policy
    (s : CourtState) (limitKey actor phase nowIso detail : String)
    (attempted n : Nat)
    (hOverrides : s.case.local_rule_overrides = [])
    (hPolicy : policyLimitValue s.policy limitKey = some n)
    (hle : attempted ≤ n) :
    enforceMeasuredLimit s actor phase nowIso limitKey detail attempted = .ok n := by
  have heff :
      effectiveLimitValue s limitKey actor phase nowIso = .ok n :=
    effectiveLimitValue_no_overrides_eq_policy s limitKey actor phase nowIso n hOverrides hPolicy
  unfold enforceMeasuredLimit
  rw [heff]
  simp [hle]

theorem effectiveLimitValue_single_override_applies_eq_override
    (s : CourtState) (limitKey actor phase nowIso : String) (base : Nat)
    (o : LocalRuleOverrideV1)
    (hOverrides : s.case.local_rule_overrides = [o])
    (hPolicy : policyLimitValue s.policy limitKey = some base)
    (hKey : o.limit_key = limitKey)
    (hApplies : overrideApplies o actor phase nowIso = true) :
    effectiveLimitValue s limitKey actor phase nowIso = .ok o.new_value := by
  unfold effectiveLimitValue
  rw [hPolicy]
  simp [hOverrides, hKey, hApplies, chooseOverride]
  rfl

theorem effectiveLimitValue_single_override_mismatched_key_eq_policy
    (s : CourtState) (limitKey actor phase nowIso : String) (base : Nat)
    (o : LocalRuleOverrideV1)
    (hOverrides : s.case.local_rule_overrides = [o])
    (hPolicy : policyLimitValue s.policy limitKey = some base)
    (hKey : o.limit_key ≠ limitKey) :
    effectiveLimitValue s limitKey actor phase nowIso = .ok base := by
  unfold effectiveLimitValue
  rw [hPolicy]
  simp [hOverrides, hKey]
  rfl

theorem effectiveLimitValue_single_override_not_applicable_eq_policy
    (s : CourtState) (limitKey actor phase nowIso : String) (base : Nat)
    (o : LocalRuleOverrideV1)
    (hOverrides : s.case.local_rule_overrides = [o])
    (hPolicy : policyLimitValue s.policy limitKey = some base)
    (hKey : o.limit_key = limitKey)
    (hNotApplies : overrideApplies o actor phase nowIso = false) :
    effectiveLimitValue s limitKey actor phase nowIso = .ok base := by
  unfold effectiveLimitValue
  rw [hPolicy]
  simp [hOverrides, hKey, hNotApplies]
  rfl

theorem effectiveLimitValue_two_overrides_second_wins_on_higher_specificity
    (s : CourtState) (limitKey actor phase nowIso : String) (base : Nat)
    (o1 o2 : LocalRuleOverrideV1)
    (hOverrides : s.case.local_rule_overrides = [o1, o2])
    (hPolicy : policyLimitValue s.policy limitKey = some base)
    (hKey1 : o1.limit_key = limitKey)
    (hApplies1 : overrideApplies o1 actor phase nowIso = true)
    (hKey2 : o2.limit_key = limitKey)
    (hApplies2 : overrideApplies o2 actor phase nowIso = true)
    (hSpec : overrideSpecificity o2 > overrideSpecificity o1) :
    effectiveLimitValue s limitKey actor phase nowIso = .ok o2.new_value := by
  unfold effectiveLimitValue
  rw [hPolicy]
  simp [hOverrides, hKey1, hApplies1, hKey2, hApplies2, chooseOverride, hSpec]
  rfl

theorem effectiveLimitValue_two_overrides_first_kept_when_second_lower_specificity
    (s : CourtState) (limitKey actor phase nowIso : String) (base : Nat)
    (o1 o2 : LocalRuleOverrideV1)
    (hOverrides : s.case.local_rule_overrides = [o1, o2])
    (hPolicy : policyLimitValue s.policy limitKey = some base)
    (hKey1 : o1.limit_key = limitKey)
    (hApplies1 : overrideApplies o1 actor phase nowIso = true)
    (hKey2 : o2.limit_key = limitKey)
    (hApplies2 : overrideApplies o2 actor phase nowIso = true)
    (hSpec : overrideSpecificity o2 < overrideSpecificity o1) :
    effectiveLimitValue s limitKey actor phase nowIso = .ok o1.new_value := by
  unfold effectiveLimitValue
  rw [hPolicy]
  have hnotgt : ¬ overrideSpecificity o2 > overrideSpecificity o1 :=
    Nat.not_lt.mpr (Nat.le_of_lt hSpec)
  simp [hOverrides, hKey1, hApplies1, hKey2, hApplies2, chooseOverride, hSpec, hnotgt]
  rfl
