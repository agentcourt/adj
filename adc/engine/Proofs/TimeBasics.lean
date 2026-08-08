import Main

theorem elapsedDaysBetween_of_ordinalDay_ok
    (servedIso respondedIso : String)
    (servedOrd respondedOrd : Nat)
    (hserved : ordinalDay servedIso = .ok servedOrd)
    (hresponded : ordinalDay respondedIso = .ok respondedOrd) :
    elapsedDaysBetween servedIso respondedIso =
      .ok (if respondedOrd >= servedOrd then respondedOrd - servedOrd else 0) := by
  unfold elapsedDaysBetween
  rw [hserved, hresponded]
  rfl

theorem elapsedDaysBetween_self_of_ordinalDay_ok
    (isoDate : String)
    (ord : Nat)
    (hord : ordinalDay isoDate = .ok ord) :
    elapsedDaysBetween isoDate isoDate = .ok 0 := by
  have h := elapsedDaysBetween_of_ordinalDay_ok isoDate isoDate ord ord hord hord
  simpa using h

theorem elapsedDaysBetween_zero_when_response_precedes_service
    (servedIso respondedIso : String)
    (servedOrd respondedOrd : Nat)
    (hserved : ordinalDay servedIso = .ok servedOrd)
    (hresponded : ordinalDay respondedIso = .ok respondedOrd)
    (hprecedes : respondedOrd < servedOrd) :
    elapsedDaysBetween servedIso respondedIso = .ok 0 := by
  have h := elapsedDaysBetween_of_ordinalDay_ok servedIso respondedIso servedOrd respondedOrd hserved hresponded
  have hnotge : ¬ respondedOrd >= servedOrd := Nat.not_le.mpr hprecedes
  simpa [hnotge] using h

theorem elapsedDaysBetween_diff_when_response_not_precedes_service
    (servedIso respondedIso : String)
    (servedOrd respondedOrd : Nat)
    (hserved : ordinalDay servedIso = .ok servedOrd)
    (hresponded : ordinalDay respondedIso = .ok respondedOrd)
    (hnotprecedes : ¬ respondedOrd < servedOrd) :
    elapsedDaysBetween servedIso respondedIso = .ok (respondedOrd - servedOrd) := by
  have h := elapsedDaysBetween_of_ordinalDay_ok servedIso respondedIso servedOrd respondedOrd hserved hresponded
  have hge : respondedOrd >= servedOrd := Nat.le_of_not_lt hnotprecedes
  simpa [hge] using h

theorem elapsedDaysBetween_diff_when_service_le_response
    (servedIso respondedIso : String)
    (servedOrd respondedOrd : Nat)
    (hserved : ordinalDay servedIso = .ok servedOrd)
    (hresponded : ordinalDay respondedIso = .ok respondedOrd)
    (hle : servedOrd ≤ respondedOrd) :
    elapsedDaysBetween servedIso respondedIso = .ok (respondedOrd - servedOrd) := by
  exact
    elapsedDaysBetween_diff_when_response_not_precedes_service
      servedIso respondedIso servedOrd respondedOrd hserved hresponded
      (Nat.not_lt_of_ge hle)

theorem elapsedDaysBetween_self_of_ordinalDay_exists
    (isoDate : String)
    (hexists : ∃ ord : Nat, ordinalDay isoDate = .ok ord) :
    elapsedDaysBetween isoDate isoDate = .ok 0 := by
  rcases hexists with ⟨ord, hord⟩
  exact elapsedDaysBetween_self_of_ordinalDay_ok isoDate ord hord
