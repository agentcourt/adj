import Main

theorem rule60GroundHasOneYearLimit_true_iff (ground : String) :
    rule60GroundHasOneYearLimit ground = true ↔
      ground = "60b1_mistake" ∨ ground = "60b2_new_evidence" ∨ ground = "60b3_fraud" := by
  unfold rule60GroundHasOneYearLimit
  by_cases h1 : ground = "60b1_mistake"
  · simp [h1]
  · by_cases h2 : ground = "60b2_new_evidence"
    · simp [h2]
    · by_cases h3 : ground = "60b3_fraud"
      · simp [h3]
      · simp [h1, h2, h3]

theorem isRule59Timely_of_elapsedDaysBetween_ok
    (judgmentDate filedAt : String)
    (elapsed : Nat)
    (helapsed : elapsedDaysBetween judgmentDate filedAt = .ok elapsed) :
    isRule59Timely judgmentDate filedAt = .ok (elapsed ≤ rule59WindowDays) := by
  unfold isRule59Timely
  rw [helapsed]
  rfl

theorem isRule59Timely_true_iff_elapsed_le
    (judgmentDate filedAt : String)
    (elapsed : Nat)
    (helapsed : elapsedDaysBetween judgmentDate filedAt = .ok elapsed) :
    isRule59Timely judgmentDate filedAt = .ok true ↔ elapsed ≤ rule59WindowDays := by
  have h := isRule59Timely_of_elapsedDaysBetween_ok judgmentDate filedAt elapsed helapsed
  constructor
  · intro hok
    rw [h] at hok
    simp at hok
    exact hok
  · intro hle
    rw [h]
    simp [hle]

theorem isRule59Timely_false_iff_elapsed_gt
    (judgmentDate filedAt : String)
    (elapsed : Nat)
    (helapsed : elapsedDaysBetween judgmentDate filedAt = .ok elapsed) :
    isRule59Timely judgmentDate filedAt = .ok false ↔ rule59WindowDays < elapsed := by
  have h := isRule59Timely_of_elapsedDaysBetween_ok judgmentDate filedAt elapsed helapsed
  constructor
  · intro hfalse
    have hEq : (elapsed ≤ rule59WindowDays) = false := by
      rw [h] at hfalse
      simpa using hfalse
    have hnotle : ¬ elapsed ≤ rule59WindowDays := by
      intro hle
      simp [hle] at hEq
    exact Nat.lt_of_not_ge hnotle
  · intro hgt
    rw [h]
    have hnotle : ¬ elapsed ≤ rule59WindowDays := Nat.not_le.mpr hgt
    simp [hnotle]

theorem isRule60Timely_unlimited_ground_true
    (judgmentDate filedAt ground : String)
    (hground : rule60GroundHasOneYearLimit ground = false)
    (elapsed : Nat)
    (helapsed : elapsedDaysBetween judgmentDate filedAt = .ok elapsed) :
    isRule60Timely judgmentDate filedAt ground = .ok true := by
  unfold isRule60Timely
  rw [helapsed]
  rw [hground]
  rfl

theorem isRule60Timely_limited_ground_of_elapsedDaysBetween_ok
    (judgmentDate filedAt ground : String)
    (hground : rule60GroundHasOneYearLimit ground = true)
    (elapsed : Nat)
    (helapsed : elapsedDaysBetween judgmentDate filedAt = .ok elapsed) :
    isRule60Timely judgmentDate filedAt ground = .ok (elapsed ≤ rule60OneYearDays) := by
  unfold isRule60Timely
  rw [helapsed]
  rw [hground]
  rfl
