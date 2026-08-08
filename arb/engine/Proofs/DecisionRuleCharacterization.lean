import Proofs.DecisionRuleFacts

namespace ArbProofs

structure DecisionCounts where
  required_votes : Nat
  seated_count : Nat
  demonstrated_count : Nat
  not_demonstrated_count : Nat
  deriving Inhabited, DecidableEq, Repr

def DecisionCounts.engineResolution? (d : DecisionCounts) : Option String :=
  if d.demonstrated_count ≥ d.required_votes then
    some "demonstrated"
  else if d.not_demonstrated_count ≥ d.required_votes then
    some "not_demonstrated"
  else
    none

def DecisionCounts.flip (d : DecisionCounts) : DecisionCounts :=
  { d with
    demonstrated_count := d.not_demonstrated_count
    not_demonstrated_count := d.demonstrated_count }

def DecisionCounts.admissible (d : DecisionCounts) : Prop :=
  d.demonstrated_count + d.not_demonstrated_count ≤ d.seated_count ∧
    d.seated_count < 2 * d.required_votes

def DecisionCounts.ofDeliberationSummary (d : DeliberationSummary) : DecisionCounts :=
  { required_votes := d.required_votes
    seated_count := d.seated_count
    demonstrated_count := d.demonstrated_count
    not_demonstrated_count := d.not_demonstrated_count }

@[simp] theorem DecisionCounts.engineResolution_ofDeliberationSummary
    (d : DeliberationSummary) :
    (DecisionCounts.ofDeliberationSummary d).engineResolution? =
      d.currentResolution? := by
  simp [DecisionCounts.ofDeliberationSummary,
    DecisionCounts.engineResolution?,
    DeliberationSummary.currentResolution?]

theorem DecisionCounts.ofDeliberationSummary_admissible
    (d : DeliberationSummary)
    (hCount : d.substantive_vote_count ≤ d.seated_count)
    (hMajority : d.seated_count < 2 * d.required_votes) :
    (DecisionCounts.ofDeliberationSummary d).admissible := by
  constructor
  · simpa [DecisionCounts.ofDeliberationSummary,
      DecisionCounts.admissible,
      DeliberationSummary.substantive_vote_count] using hCount
  · simpa [DecisionCounts.ofDeliberationSummary,
      DecisionCounts.admissible] using hMajority

theorem DecisionCounts.flip_admissible
    (d : DecisionCounts)
    (hAdmissible : d.admissible) :
    d.flip.admissible := by
  rcases hAdmissible with ⟨hCount, hMajority⟩
  constructor
  · simp [DecisionCounts.flip]
    omega
  · simpa [DecisionCounts.flip] using hMajority

theorem DecisionCounts.not_both_thresholds
    (d : DecisionCounts)
    (hAdmissible : d.admissible) :
    ¬ (d.demonstrated_count ≥ d.required_votes ∧
      d.not_demonstrated_count ≥ d.required_votes) := by
  rcases hAdmissible with ⟨hCount, hMajority⟩
  intro hBoth
  rcases hBoth with ⟨hDemonstrated, hNotDemonstrated⟩
  omega

@[simp] private theorem trimString_demonstrated_characterization :
    trimString "demonstrated" = "demonstrated" := by
  native_decide

@[simp] private theorem trimString_not_demonstrated_characterization :
    trimString "not_demonstrated" = "not_demonstrated" := by
  native_decide

@[simp] private theorem flipOutcomeLabel_demonstrated_characterization :
    flipOutcomeLabel "demonstrated" = "not_demonstrated" := by
  simp [flipOutcomeLabel]

@[simp] private theorem flipOutcomeLabel_not_demonstrated_characterization :
    flipOutcomeLabel "not_demonstrated" = "demonstrated" := by
  simp [flipOutcomeLabel]

theorem DecisionCounts.engineResolution_flip
    (d : DecisionCounts)
    (hAdmissible : d.admissible) :
    d.flip.engineResolution? =
      flipResolution d.engineResolution? := by
  have hNoDual := d.not_both_thresholds hAdmissible
  by_cases hDemonstrated : d.demonstrated_count ≥ d.required_votes
  · have hNotDemonstratedFalse :
        ¬ d.not_demonstrated_count ≥ d.required_votes := by
      intro hNotDemonstrated
      exact hNoDual ⟨hDemonstrated, hNotDemonstrated⟩
    simp [DecisionCounts.engineResolution?, DecisionCounts.flip,
      flipResolution, hDemonstrated, hNotDemonstratedFalse]
  · by_cases hNotDemonstrated : d.not_demonstrated_count ≥ d.required_votes
    · simp [DecisionCounts.engineResolution?, DecisionCounts.flip,
        flipResolution, hDemonstrated, hNotDemonstrated]
    · simp [DecisionCounts.engineResolution?, DecisionCounts.flip,
        flipResolution, hDemonstrated, hNotDemonstrated]

structure CountDecisionRuleSpec
    (rule : DecisionCounts → Option String) : Prop where
  result_range :
    ∀ d,
      rule d = some "demonstrated" ∨
        rule d = some "not_demonstrated" ∨
          rule d = none
  neutral :
    ∀ d, d.admissible →
      rule d.flip = flipResolution (rule d)
  demonstrated_threshold :
    ∀ d, d.admissible →
      d.demonstrated_count ≥ d.required_votes →
        rule d = some "demonstrated"
  below_threshold_none :
    ∀ d, d.admissible →
      d.demonstrated_count < d.required_votes →
        d.not_demonstrated_count < d.required_votes →
          rule d = none

def countThresholdRule (d : DecisionCounts) : Option String :=
  d.engineResolution?

theorem countThresholdRule_spec :
    CountDecisionRuleSpec countThresholdRule := by
  exact
    { result_range := by
        intro d
        unfold countThresholdRule DecisionCounts.engineResolution?
        by_cases hDemonstrated : d.demonstrated_count ≥ d.required_votes
        · simp [hDemonstrated]
        · by_cases hNotDemonstrated : d.not_demonstrated_count ≥ d.required_votes
          · simp [hDemonstrated, hNotDemonstrated]
          · simp [hDemonstrated, hNotDemonstrated]
      neutral := by
        intro d hAdmissible
        exact DecisionCounts.engineResolution_flip d hAdmissible
      demonstrated_threshold := by
        intro d _hAdmissible hDemonstrated
        simp [countThresholdRule, DecisionCounts.engineResolution?, hDemonstrated]
      below_threshold_none := by
        intro d _hAdmissible hDemonstratedLt hNotDemonstratedLt
        have hDemonstratedFalse :
            ¬ d.demonstrated_count ≥ d.required_votes :=
          Nat.not_le.mpr hDemonstratedLt
        have hNotDemonstratedFalse :
            ¬ d.not_demonstrated_count ≥ d.required_votes :=
          Nat.not_le.mpr hNotDemonstratedLt
        simp [countThresholdRule, DecisionCounts.engineResolution?,
          hDemonstratedFalse, hNotDemonstratedFalse] }

theorem CountDecisionRuleSpec.not_demonstrated_threshold
    {rule : DecisionCounts → Option String}
    (hRule : CountDecisionRuleSpec rule)
    (d : DecisionCounts)
    (hAdmissible : d.admissible)
    (hNotDemonstrated : d.not_demonstrated_count ≥ d.required_votes) :
    rule d = some "not_demonstrated" := by
  have hFlipAdmissible : d.flip.admissible :=
    d.flip_admissible hAdmissible
  have hFlipDemonstrated :
      d.flip.demonstrated_count ≥ d.flip.required_votes := by
    simpa [DecisionCounts.flip] using hNotDemonstrated
  have hFlipRule : rule d.flip = some "demonstrated" :=
    hRule.demonstrated_threshold d.flip hFlipAdmissible hFlipDemonstrated
  have hNeutral := hRule.neutral d hAdmissible
  rw [hNeutral] at hFlipRule
  rcases hRule.result_range d with hDemonstrated | hRest
  · rw [hDemonstrated] at hFlipRule
    simp [flipResolution] at hFlipRule
  · rcases hRest with hNotDemonstratedResult | hNone
    · exact hNotDemonstratedResult
    · rw [hNone] at hFlipRule
      simp [flipResolution] at hFlipRule

theorem CountDecisionRuleSpec.eq_engineResolution
    {rule : DecisionCounts → Option String}
    (hRule : CountDecisionRuleSpec rule)
    (d : DecisionCounts)
    (hAdmissible : d.admissible) :
    rule d = d.engineResolution? := by
  by_cases hDemonstrated : d.demonstrated_count ≥ d.required_votes
  · have hRuleDemonstrated :
        rule d = some "demonstrated" :=
      hRule.demonstrated_threshold d hAdmissible hDemonstrated
    simp [DecisionCounts.engineResolution?, hDemonstrated, hRuleDemonstrated]
  · have hDemonstratedLt :
        d.demonstrated_count < d.required_votes :=
      Nat.lt_of_not_ge hDemonstrated
    by_cases hNotDemonstrated : d.not_demonstrated_count ≥ d.required_votes
    · have hRuleNotDemonstrated :
          rule d = some "not_demonstrated" :=
        hRule.not_demonstrated_threshold
          d hAdmissible hNotDemonstrated
      simp [DecisionCounts.engineResolution?, hDemonstrated,
        hNotDemonstrated, hRuleNotDemonstrated]
    · have hNotDemonstratedLt :
          d.not_demonstrated_count < d.required_votes :=
        Nat.lt_of_not_ge hNotDemonstrated
      have hRuleNone : rule d = none :=
        hRule.below_threshold_none
          d hAdmissible hDemonstratedLt hNotDemonstratedLt
      simp [DecisionCounts.engineResolution?, hDemonstrated,
        hNotDemonstrated, hRuleNone]

theorem CountDecisionRuleSpec.eq_currentResolution_of_summary
    {rule : DecisionCounts → Option String}
    (hRule : CountDecisionRuleSpec rule)
    (d : DeliberationSummary)
    (hCount : d.substantive_vote_count ≤ d.seated_count)
    (hMajority : d.seated_count < 2 * d.required_votes) :
    rule (DecisionCounts.ofDeliberationSummary d) =
      d.currentResolution? := by
  have hAdmissible :=
    DecisionCounts.ofDeliberationSummary_admissible d hCount hMajority
  simpa using
    hRule.eq_engineResolution
      (DecisionCounts.ofDeliberationSummary d)
      hAdmissible

end ArbProofs
