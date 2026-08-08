import Main

theorem normalizePartyToken_claimant :
    normalizePartyToken "claimant" = "plaintiff" := by
  native_decide

theorem normalizePartyToken_defense :
    normalizePartyToken "defense" = "defendant" := by
  native_decide

theorem normalizePartyToken_defence :
    normalizePartyToken "defence" = "defendant" := by
  native_decide

theorem normalizePartyToken_plaintiff_fixed :
    normalizePartyToken "plaintiff" = "plaintiff" := by
  native_decide

theorem normalizePartyToken_defendant_fixed :
    normalizePartyToken "defendant" = "defendant" := by
  native_decide

theorem normalizePartyToken_idempotent_on_normalized (s : String)
    (h : normalizePartyToken s = "plaintiff" ∨ normalizePartyToken s = "defendant") :
    normalizePartyToken (normalizePartyToken s) = normalizePartyToken s := by
  cases h with
  | inl hp =>
      rw [hp]
      exact normalizePartyToken_plaintiff_fixed
  | inr hd =>
      rw [hd]
      exact normalizePartyToken_defendant_fixed

theorem normalizePartyToken_output_classification (s : String) :
    normalizePartyToken s = "plaintiff" ∨
      normalizePartyToken s = "defendant" ∨
      normalizePartyToken s = (trimString s).toLower := by
  by_cases hpl :
      (((trimString s).toLower = "plaintiff" ∨ (trimString s).toLower = "claimant") ∨
          (trimString s).toLower.contains "plaintiff" = true) ∨
        (trimString s).toLower.contains "claimant" = true
  · exact Or.inl (by simp [normalizePartyToken, hpl])
  · by_cases hdef :
      (((((trimString s).toLower = "defendant" ∨ (trimString s).toLower = "defense") ∨
              (trimString s).toLower = "defence") ∨
            (trimString s).toLower.contains "defendant" = true) ∨
          (trimString s).toLower.contains "defense" = true) ∨
        (trimString s).toLower.contains "defence" = true
    · exact Or.inr (Or.inl (by simp [normalizePartyToken, hpl, hdef]))
    · exact Or.inr (Or.inr (by simp [normalizePartyToken, hpl, hdef]))
