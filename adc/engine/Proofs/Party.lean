import ADC.Core

namespace ADCProofs.Party

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
  unfold normalizePartyToken
  dsimp
  split
  · exact Or.inl rfl
  · split
    · exact Or.inr (Or.inl rfl)
    · exact Or.inr (Or.inr rfl)

end ADCProofs.Party
