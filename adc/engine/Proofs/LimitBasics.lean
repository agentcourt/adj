import Main

theorem enforceMeasuredLimit_ok_implies_attempted_le
    (s : CourtState) (actor phase nowIso limitKey detail : String)
    (attempted allowed : Nat)
    (h :
      enforceMeasuredLimit s actor phase nowIso limitKey detail attempted = .ok allowed) :
    attempted ≤ allowed := by
  unfold enforceMeasuredLimit at h
  cases heff : effectiveLimitValue s limitKey actor phase nowIso with
  | error err =>
      simp [heff] at h
  | ok lim =>
      by_cases hle : attempted ≤ lim
      · simp [heff, hle] at h
        cases h
        exact hle
      · simp [heff, hle] at h

theorem enforceMeasuredLimit_ok_of_effectiveLimitValue_ok
    (s : CourtState) (actor phase nowIso limitKey detail : String)
    (attempted allowed : Nat)
    (heff : effectiveLimitValue s limitKey actor phase nowIso = .ok allowed)
    (hle : attempted ≤ allowed) :
    enforceMeasuredLimit s actor phase nowIso limitKey detail attempted = .ok allowed := by
  simp [enforceMeasuredLimit, heff, hle]

theorem enforceMeasuredLimit_error_of_effectiveLimitValue_ok_gt
    (s : CourtState) (actor phase nowIso limitKey detail : String)
    (attempted allowed : Nat)
    (heff : effectiveLimitValue s limitKey actor phase nowIso = .ok allowed)
    (hgt : allowed < attempted) :
    ∃ msg : String, enforceMeasuredLimit s actor phase nowIso limitKey detail attempted = .error msg := by
  refine ⟨limitViolationMessage limitKey actor phase attempted allowed detail, ?_⟩
  have hnotle : ¬ attempted ≤ allowed := Nat.not_le.mpr hgt
  simp [enforceMeasuredLimit, heff, hnotle]
