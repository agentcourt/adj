import Main

theorem sumContemptCounts_incrementContemptCount (counts : List ContemptCounter) (targetRole : String) :
    sumContemptCounts (incrementContemptCount counts targetRole) = sumContemptCounts counts + 1 := by
  induction counts with
  | nil =>
      simp [incrementContemptCount, sumContemptCounts]
  | cons x rest ih =>
      by_cases h : x.role = targetRole
      · calc
          sumContemptCounts (incrementContemptCount (x :: rest) targetRole)
              = x.count + 1 + sumContemptCounts rest := by
                  simp [incrementContemptCount, h, sumContemptCounts]
          _ = x.count + sumContemptCounts rest + 1 := by
                  omega
      · calc
          sumContemptCounts (incrementContemptCount (x :: rest) targetRole)
              = x.count + sumContemptCounts (incrementContemptCount rest targetRole) := by
                  simp [incrementContemptCount, h, sumContemptCounts]
          _ = x.count + (sumContemptCounts rest + 1) := by
                  simp [ih]
          _ = x.count + sumContemptCounts rest + 1 := by
                  omega

theorem incrementContemptCount_length (counts : List ContemptCounter) (targetRole : String) :
    (incrementContemptCount counts targetRole).length =
      if counts.any (fun c => c.role = targetRole) then counts.length else counts.length + 1 := by
  induction counts with
  | nil =>
      simp [incrementContemptCount]
  | cons x rest ih =>
      by_cases h : x.role = targetRole
      · simp [incrementContemptCount, h]
      · calc
          (incrementContemptCount (x :: rest) targetRole).length
              = (incrementContemptCount rest targetRole).length + 1 := by
                  simp [incrementContemptCount, h]
          _ = (if rest.any (fun c => c.role = targetRole) then rest.length else rest.length + 1) + 1 := by
                  simp [ih]
          _ = if (x :: rest).any (fun c => c.role = targetRole) then (x :: rest).length else (x :: rest).length + 1 := by
                  by_cases hr : rest.any (fun c => c.role = targetRole)
                  · simp [h, hr]
                  · simp [h, hr, Nat.add_assoc]

theorem contemptCountFor_target_increment (counts : List ContemptCounter) (targetRole : String) :
    contemptCountFor (incrementContemptCount counts targetRole) targetRole =
      contemptCountFor counts targetRole + 1 := by
  induction counts with
  | nil =>
      simp [incrementContemptCount, contemptCountFor]
  | cons x rest ih =>
      by_cases h : x.role = targetRole
      · simp [incrementContemptCount, contemptCountFor, h, Nat.add_assoc, Nat.add_left_comm, Nat.add_comm]
      · simp [incrementContemptCount, contemptCountFor, h, ih]

theorem contemptCountFor_other_unchanged (counts : List ContemptCounter) (targetRole otherRole : String)
    (hOther : otherRole ≠ targetRole) :
    contemptCountFor (incrementContemptCount counts targetRole) otherRole =
      contemptCountFor counts otherRole := by
  induction counts with
  | nil =>
      have hto : targetRole ≠ otherRole := by
        intro hEq
        exact hOther hEq.symm
      simp [incrementContemptCount, contemptCountFor, hto]
  | cons x rest ih =>
      by_cases h : x.role = targetRole
      · have hto : targetRole ≠ otherRole := by
          intro hEq
          exact hOther hEq.symm
        simp [incrementContemptCount, contemptCountFor, h, hto]
      · simp [incrementContemptCount, contemptCountFor, h, ih]

theorem contemptCountFor_incrementContemptCount (counts : List ContemptCounter) (targetRole role : String) :
    contemptCountFor (incrementContemptCount counts targetRole) role =
      contemptCountFor counts role + (if role = targetRole then 1 else 0) := by
  by_cases h : role = targetRole
  · simp [h, contemptCountFor_target_increment]
  · simp [h, contemptCountFor_other_unchanged counts targetRole role h]

theorem contemptCountFor_le_sumContemptCounts (counts : List ContemptCounter) (role : String) :
    contemptCountFor counts role ≤ sumContemptCounts counts := by
  induction counts with
  | nil =>
      simp [contemptCountFor, sumContemptCounts]
  | cons c rest ih =>
      by_cases h : c.role = role
      · simp [contemptCountFor, sumContemptCounts, h, ih]
      · simp [contemptCountFor, sumContemptCounts, h]
        exact Nat.le_trans ih (Nat.le_add_left (sumContemptCounts rest) c.count)

theorem contemptCountFor_incrementContemptCount_le_sum_plus_one
    (counts : List ContemptCounter) (targetRole role : String) :
    contemptCountFor (incrementContemptCount counts targetRole) role ≤ sumContemptCounts counts + 1 := by
  calc
    contemptCountFor (incrementContemptCount counts targetRole) role
        ≤ sumContemptCounts (incrementContemptCount counts targetRole) := by
            exact contemptCountFor_le_sumContemptCounts (incrementContemptCount counts targetRole) role
    _ = sumContemptCounts counts + 1 := by
            exact sumContemptCounts_incrementContemptCount counts targetRole

theorem contemptCountFor_target_positive_after_increment
    (counts : List ContemptCounter) (targetRole : String) :
    contemptCountFor (incrementContemptCount counts targetRole) targetRole > 0 := by
  have hinc :
      contemptCountFor (incrementContemptCount counts targetRole) targetRole =
        contemptCountFor counts targetRole + 1 :=
    contemptCountFor_target_increment counts targetRole
  rw [hinc]
  omega

theorem sumContemptCounts_positive_after_increment
    (counts : List ContemptCounter) (targetRole : String) :
    sumContemptCounts (incrementContemptCount counts targetRole) > 0 := by
  have hsum :
      sumContemptCounts (incrementContemptCount counts targetRole) =
        sumContemptCounts counts + 1 :=
    sumContemptCounts_incrementContemptCount counts targetRole
  rw [hsum]
  omega

theorem contemptCountFor_incrementContemptCount_ge
    (counts : List ContemptCounter) (targetRole role : String) :
    contemptCountFor (incrementContemptCount counts targetRole) role ≥
      contemptCountFor counts role := by
  have hbase :
      contemptCountFor (incrementContemptCount counts targetRole) role =
        contemptCountFor counts role + (if role = targetRole then 1 else 0) :=
    contemptCountFor_incrementContemptCount counts targetRole role
  rw [hbase]
  by_cases h : role = targetRole
  · simp [h]
  · simp [h]

theorem sumContemptCounts_incrementContemptCount_gt
    (counts : List ContemptCounter) (targetRole : String) :
    sumContemptCounts (incrementContemptCount counts targetRole) >
      sumContemptCounts counts := by
  have hsum :
      sumContemptCounts (incrementContemptCount counts targetRole) =
        sumContemptCounts counts + 1 :=
    sumContemptCounts_incrementContemptCount counts targetRole
  rw [hsum]
  omega
