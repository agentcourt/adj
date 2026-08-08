import Main

theorem swearAvailableLoop_length (remaining : Nat) (jurors : List JurorRecord) :
    (swearAvailableLoop remaining jurors).length = jurors.length := by
  induction jurors generalizing remaining with
  | nil =>
      simp [swearAvailableLoop]
  | cons j rest ih =>
      by_cases h : remaining > 0 && j.status = "available"
      · simp [swearAvailableLoop, h, ih]
      · simp [swearAvailableLoop, h, ih]

theorem swearAvailable_length (jurors : List JurorRecord) (needed : Nat) :
    (swearAvailable jurors needed).length = jurors.length := by
  simpa [swearAvailable] using swearAvailableLoop_length needed jurors

theorem swearAvailableLoop_zero (jurors : List JurorRecord) :
    swearAvailableLoop 0 jurors = jurors := by
  induction jurors with
  | nil =>
      simp [swearAvailableLoop]
  | cons j rest ih =>
      simp [swearAvailableLoop, ih]

theorem swearAvailable_zero (jurors : List JurorRecord) :
    swearAvailable jurors 0 = jurors := by
  simpa [swearAvailable] using swearAvailableLoop_zero jurors

theorem countAvailableFold_bound (acc : Nat) (jurors : List JurorRecord) :
    jurors.foldl (fun n j => if j.status = "available" then n + 1 else n) acc ≤ acc + jurors.length := by
  induction jurors generalizing acc with
  | nil =>
      simp
  | cons j rest ih =>
      by_cases havail : j.status = "available"
      · have hrest :
          rest.foldl (fun n j => if j.status = "available" then n + 1 else n) (acc + 1) ≤
            (acc + 1) + rest.length := ih (acc + 1)
        have hgoal :
            rest.foldl (fun n j => if j.status = "available" then n + 1 else n) (acc + 1) ≤
              acc + (rest.length + 1) := by
          omega
        simpa [List.foldl, havail] using hgoal
      · have hrest :
          rest.foldl (fun n j => if j.status = "available" then n + 1 else n) acc ≤
            acc + rest.length := ih acc
        have hgoal :
            rest.foldl (fun n j => if j.status = "available" then n + 1 else n) acc ≤
              acc + (rest.length + 1) := by
          omega
        simpa [List.foldl, havail] using hgoal

theorem countAvailable_le_length (jurors : List JurorRecord) :
    countAvailable jurors ≤ jurors.length := by
  have hbound : countAvailable jurors ≤ 0 + jurors.length :=
    countAvailableFold_bound 0 jurors
  simpa [countAvailable] using hbound

theorem countAvailable_swearAvailable_le_length (jurors : List JurorRecord) (needed : Nat) :
    countAvailable (swearAvailable jurors needed) ≤ jurors.length := by
  have hbound : countAvailable (swearAvailable jurors needed) ≤ (swearAvailable jurors needed).length :=
    countAvailable_le_length (swearAvailable jurors needed)
  have hlen : (swearAvailable jurors needed).length = jurors.length :=
    swearAvailable_length jurors needed
  rw [hlen] at hbound
  exact hbound
