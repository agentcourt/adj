import Proofs.Reachability
namespace ArbProofs

/-
This file proves the arithmetic fact behind the cumulative material limits.

The engine stores admitted exhibits and technical reports in append-only lists.
The policy then places side-level caps on those lists.  The dynamic property we
want is simple to state: if the current state already respects the side caps,
and if the next batch of materials belongs to one side and still fits within
that side's remaining budget, then appending the new materials preserves the
caps.

This theorem is more useful than a sample check and more tractable than an
immediate theorem about the whole `step` function.  It isolates the exact
counting argument that later step-level proofs will need.
-/

theorem offeredCount_eq_length_of_all_role
    (items : List OfferedEvidence)
    (role : String)
    (hAll : ∀ item ∈ items, item.role = role) :
    offeredCount items role = items.length := by
  induction items with
  | nil =>
      simp [offeredCount]
  | cons head tail ih =>
      have hHead : head.role = role := hAll head (by simp)
      have hTail : ∀ item, item ∈ tail → item.role = role := by
        intro item hItem
        exact hAll item (by simp [List.mem_cons, hItem])
      have hTailCount : offeredCount tail role = tail.length := ih hTail
      simpa [offeredCount, hHead] using congrArg Nat.succ hTailCount

theorem offeredCount_zero_of_all_other_role
    (items : List OfferedEvidence)
    (role other : String)
    (hAll : ∀ item ∈ items, item.role = role)
    (hNe : role ≠ other) :
    offeredCount items other = 0 := by
  induction items with
  | nil =>
      simp [offeredCount]
  | cons head tail ih =>
      have hHead : head.role = role := hAll head (by simp)
      have hTailRole : ∀ item, item ∈ tail → item.role = role := by
        intro item hItem
        exact hAll item (by simp [List.mem_cons, hItem])
      have hTailNe : ∀ item, item ∈ tail → ¬ item.role = other := by
        intro item hItem hEq
        exact hNe ((hTailRole item hItem).symm.trans hEq)
      have hHeadNe : head.role ≠ other := by
        intro hEq
        exact hNe (hHead.symm.trans hEq)
      have hTailCount : offeredCount tail other = 0 := ih hTailRole
      simpa [offeredCount, hHeadNe] using hTailCount

theorem reportCount_eq_length_of_all_role
    (items : List TechnicalReport)
    (role : String)
    (hAll : ∀ item ∈ items, item.role = role) :
    reportCount items role = items.length := by
  induction items with
  | nil =>
      simp [reportCount]
  | cons head tail ih =>
      have hHead : head.role = role := hAll head (by simp)
      have hTail : ∀ item, item ∈ tail → item.role = role := by
        intro item hItem
        exact hAll item (by simp [List.mem_cons, hItem])
      have hTailCount : reportCount tail role = tail.length := ih hTail
      simpa [reportCount, hHead] using congrArg Nat.succ hTailCount

theorem reportCount_zero_of_all_other_role
    (items : List TechnicalReport)
    (role other : String)
    (hAll : ∀ item ∈ items, item.role = role)
    (hNe : role ≠ other) :
    reportCount items other = 0 := by
  induction items with
  | nil =>
      simp [reportCount]
  | cons head tail ih =>
      have hHead : head.role = role := hAll head (by simp)
      have hTailRole : ∀ item, item ∈ tail → item.role = role := by
        intro item hItem
        exact hAll item (by simp [List.mem_cons, hItem])
      have hTailNe : ∀ item, item ∈ tail → ¬ item.role = other := by
        intro item hItem hEq
        exact hNe ((hTailRole item hItem).symm.trans hEq)
      have hHeadNe : head.role ≠ other := by
        intro hEq
        exact hNe (hHead.symm.trans hEq)
      have hTailCount : reportCount tail other = 0 := ih hTailRole
      simpa [reportCount, hHeadNe] using hTailCount

/--
Appending one side's new materials preserves the cumulative side caps.

The theorem uses explicit role-uniformity assumptions for the new material
lists.  That is deliberate.  The arithmetic fact should not depend on how the
lists were produced.  It says: once the incoming exhibits and reports all
belong to one side, and once the caller has shown that side's new totals fit
within the policy, appending those lists preserves the full bilateral limit
invariant.
-/
theorem appendSupplementalMaterials_preserves_material_limits
    (s : ArbitrationState)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (role : String)
    (hBase : materialLimitsRespected s)
    (hOfferedRole : ∀ item ∈ offered, item.role = role)
    (hReportRole : ∀ item ∈ reports, item.role = role)
    (hOfferedCap :
      offeredCount s.case.offered_evidence role + offered.length ≤ s.policy.max_exhibits_per_side)
    (hReportCap :
      reportCount s.case.technical_reports role + reports.length ≤ s.policy.max_reports_per_side) :
    materialLimitsRespected { s with case := appendSupplementalMaterials s.case offered reports } := by
  rcases hBase with ⟨hPlaintiffOff, hDefendantOff, hPlaintiffRep, hDefendantRep⟩
  by_cases hPlaintiff : role = "plaintiff"
  · have hOfferedSelf : offeredCount offered "plaintiff" = offered.length := by
      simpa [hPlaintiff] using offeredCount_eq_length_of_all_role offered role hOfferedRole
    have hOfferedOther : offeredCount offered "defendant" = 0 := by
      apply offeredCount_zero_of_all_other_role offered role "defendant" hOfferedRole
      simp [hPlaintiff]
    have hReportSelf : reportCount reports "plaintiff" = reports.length := by
      simpa [hPlaintiff] using reportCount_eq_length_of_all_role reports role hReportRole
    have hReportOther : reportCount reports "defendant" = 0 := by
      apply reportCount_zero_of_all_other_role reports role "defendant" hReportRole
      simp [hPlaintiff]
    refine ⟨?_, ?_⟩
    · simpa [appendSupplementalMaterials, offeredCount_append, hOfferedSelf, hPlaintiff] using hOfferedCap
    · refine ⟨?_, ?_⟩
      · simpa [appendSupplementalMaterials, offeredCount_append, hOfferedOther] using hDefendantOff
      · refine ⟨?_, ?_⟩
        · simpa [appendSupplementalMaterials, reportCount_append, hReportSelf, hPlaintiff] using hReportCap
        · simpa [appendSupplementalMaterials, reportCount_append, hReportOther] using hDefendantRep
  · by_cases hDefendant : role = "defendant"
    · have hOfferedSelf : offeredCount offered "defendant" = offered.length := by
        simpa [hDefendant] using offeredCount_eq_length_of_all_role offered role hOfferedRole
      have hOfferedOther : offeredCount offered "plaintiff" = 0 := by
        apply offeredCount_zero_of_all_other_role offered role "plaintiff" hOfferedRole
        simp [hDefendant]
      have hReportSelf : reportCount reports "defendant" = reports.length := by
        simpa [hDefendant] using reportCount_eq_length_of_all_role reports role hReportRole
      have hReportOther : reportCount reports "plaintiff" = 0 := by
        apply reportCount_zero_of_all_other_role reports role "plaintiff" hReportRole
        simp [hDefendant]
      refine ⟨?_, ?_⟩
      · simpa [appendSupplementalMaterials, offeredCount_append, hOfferedOther] using hPlaintiffOff
      · refine ⟨?_, ?_⟩
        · simpa [appendSupplementalMaterials, offeredCount_append, hOfferedSelf, hDefendant] using hOfferedCap
        · refine ⟨?_, ?_⟩
          · simpa [appendSupplementalMaterials, reportCount_append, hReportOther] using hPlaintiffRep
          · simpa [appendSupplementalMaterials, reportCount_append, hReportSelf, hDefendant] using hReportCap
    · have hOfferedPlaintiff : offeredCount offered "plaintiff" = 0 := by
        apply offeredCount_zero_of_all_other_role offered role "plaintiff" hOfferedRole
        exact hPlaintiff
      have hOfferedDefendant : offeredCount offered "defendant" = 0 := by
        apply offeredCount_zero_of_all_other_role offered role "defendant" hOfferedRole
        exact hDefendant
      have hReportPlaintiff : reportCount reports "plaintiff" = 0 := by
        apply reportCount_zero_of_all_other_role reports role "plaintiff" hReportRole
        exact hPlaintiff
      have hReportDefendant : reportCount reports "defendant" = 0 := by
        apply reportCount_zero_of_all_other_role reports role "defendant" hReportRole
        exact hDefendant
      refine ⟨?_, ?_⟩
      · simpa [appendSupplementalMaterials, offeredCount_append, hOfferedPlaintiff] using hPlaintiffOff
      · refine ⟨?_, ?_⟩
        · simpa [appendSupplementalMaterials, offeredCount_append, hOfferedDefendant] using hDefendantOff
        · refine ⟨?_, ?_⟩
          · simpa [appendSupplementalMaterials, reportCount_append, hReportPlaintiff] using hPlaintiffRep
          · simpa [appendSupplementalMaterials, reportCount_append, hReportDefendant] using hDefendantRep

theorem appendSubmittedEvidence_preserves_material_limits
    (s : ArbitrationState)
    (evidence : SubmittedEvidence)
    (hBase : materialLimitsRespected s) :
    materialLimitsRespected { s with case := appendSubmittedEvidence s.case evidence } := by
  simpa [materialLimitsRespected, appendSubmittedEvidence] using hBase

/-
The lemmas below lift the arithmetic fact into procedure-level invariants.

`appendSupplementalMaterials_preserves_material_limits` proves the counting
argument itself.  The remaining work is structural.  Most transition helpers do
not touch the admitted-material lists at all.  The proofs make that explicit.

That distinction matters.  If a later edit breaks the invariant, the failure
should point to one of two precise causes: either the append arithmetic was
wrong, or a transition unexpectedly changed the material lists.
-/

theorem advanceAfterMerits_preserves_offered_evidence
    (c : ArbitrationCase) :
    (advanceAfterMerits c).offered_evidence = c.offered_evidence := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 ∧ c.phase = "openings"
  · simp [hOpen]
  · simp [hOpen]
    by_cases hArg : c.arguments.length >= 2 ∧ c.phase = "arguments"
    · simp [hArg]
    · simp [hArg]
      by_cases hRebut : c.rebuttals.length >= 1 ∧ c.phase = "rebuttals"
      · simp [hRebut]
      · simp [hRebut]
        by_cases hSur : c.surrebuttals.length >= 1 ∧ c.phase = "surrebuttals"
        · simp [hSur]
        · simp [hSur]
          by_cases hClose : c.closings.length >= 2 ∧ c.phase = "closings"
          · simp [hClose]
          · simp [hClose]

theorem advanceAfterMerits_preserves_technical_reports
    (c : ArbitrationCase) :
    (advanceAfterMerits c).technical_reports = c.technical_reports := by
  unfold advanceAfterMerits
  by_cases hOpen : c.openings.length >= 2 ∧ c.phase = "openings"
  · simp [hOpen]
  · simp [hOpen]
    by_cases hArg : c.arguments.length >= 2 ∧ c.phase = "arguments"
    · simp [hArg]
    · simp [hArg]
      by_cases hRebut : c.rebuttals.length >= 1 ∧ c.phase = "rebuttals"
      · simp [hRebut]
      · simp [hRebut]
        by_cases hSur : c.surrebuttals.length >= 1 ∧ c.phase = "surrebuttals"
        · simp [hSur]
        · simp [hSur]
          by_cases hClose : c.closings.length >= 2 ∧ c.phase = "closings"
          · simp [hClose]
          · simp [hClose]

theorem addFiling_preserves_offered_evidence
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).offered_evidence = c.offered_evidence := by
  unfold addFiling
  split <;> simp [advanceAfterMerits_preserves_offered_evidence]

theorem addFiling_preserves_technical_reports
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).technical_reports = c.technical_reports := by
  unfold addFiling
  split <;> simp [advanceAfterMerits_preserves_technical_reports]

theorem stateWithCase_preserves_material_limits
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hBase : materialLimitsRespected s)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports) :
    materialLimitsRespected (stateWithCase s c) := by
  simpa [materialLimitsRespected, stateWithCase, hOffered, hReports] using hBase

end ArbProofs
