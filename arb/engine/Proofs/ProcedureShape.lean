import Proofs.Reachability

namespace ArbProofs

/-
This file proves a fairness property at the engine boundary.

The earlier proof files described the linear merits sequence with sample steps.
This file moves one level outward, to `nextOpportunityForPhase`.  That function
decides which side may act next.  If it allocates the optional phases
incorrectly, no later runner logic can repair the procedure.

The two claims below are the core of the optional-phase fairness rule.

First: when the case is in rebuttals, the only live opportunity belongs to the
plaintiff.  The defendant never receives a rebuttal opportunity.

Second: when the case is in surrebuttals, the only live opportunity belongs to
the defendant.  The plaintiff never receives a surrebuttal opportunity.

The engine also treats those phases as single-turn opportunities.  Once the
relevant list is nonempty, `nextOpportunityForPhase` returns a terminal result
for that phase instead of offering a second turn.
-/

/-
The file also connects the abstract phase-shape predicate to a simpler fairness
invariant over counts.

`phaseShape` says much more than parity.  It describes the whole merits
sequence, including which later lists must still be empty in each phase.
`proceduralParity` extracts the part that matters for bilateral fairness: in
openings, arguments, and closings, the defendant is never ahead of the
plaintiff and neither side has more than one filing; rebuttal remains a single
plaintiff filing; surrebuttal remains a single defendant filing.

That theorem matters because later proofs can use the smaller invariant without
carrying the full phase machine everywhere.
-/

theorem bilateralStarted_implies_bilateral_parity
    (phase : String)
    (xs : List Filing)
    (h : bilateralStarted phase xs) :
    filingCount xs "plaintiff" ≤ 1 ∧
      filingCount xs "defendant" ≤ 1 ∧
      filingCount xs "defendant" ≤ filingCount xs "plaintiff" := by
  cases xs with
  | nil =>
      simp [filingCount]
  | cons head tail =>
      cases tail with
      | nil =>
          simp [bilateralStarted, filingCount] at h ⊢
          rcases h with ⟨_, hRole⟩
          simp [hRole]
      | cons next rest =>
          simp [bilateralStarted] at h

theorem bilateralComplete_implies_bilateral_parity
    (phase : String)
    (xs : List Filing)
    (h : bilateralComplete phase xs) :
    filingCount xs "plaintiff" ≤ 1 ∧
      filingCount xs "defendant" ≤ 1 ∧
      filingCount xs "defendant" ≤ filingCount xs "plaintiff" := by
  cases xs with
  | nil =>
      simp [bilateralComplete] at h
  | cons first tail =>
      cases tail with
      | nil =>
          simp [bilateralComplete] at h
      | cons second rest =>
          cases rest with
          | nil =>
              simp [bilateralComplete, filingCount] at h ⊢
              rcases h with ⟨_, hFirstRole, _, hSecondRole⟩
              simp [hFirstRole, hSecondRole]
          | cons third rest =>
              simp [bilateralComplete] at h

theorem plaintiffOptionalSequence_implies_optional_parity
    (phase : String)
    (xs : List Filing)
    (h : plaintiffOptionalSequence phase xs) :
    filingCount xs "plaintiff" ≤ 1 ∧
      filingCount xs "defendant" = 0 := by
  cases xs with
  | nil =>
      simp [filingCount]
  | cons head tail =>
      cases tail with
      | nil =>
          simp [plaintiffOptionalSequence, filingCount] at h ⊢
          rcases h with ⟨_, hRole⟩
          simp [hRole]
      | cons next rest =>
          simp [plaintiffOptionalSequence] at h

theorem defendantOptionalSequence_implies_optional_parity
    (phase : String)
    (xs : List Filing)
    (h : defendantOptionalSequence phase xs) :
    filingCount xs "plaintiff" = 0 ∧
      filingCount xs "defendant" ≤ 1 := by
  cases xs with
  | nil =>
      simp [filingCount]
  | cons head tail =>
      cases tail with
      | nil =>
          simp [defendantOptionalSequence, filingCount] at h ⊢
          rcases h with ⟨_, hRole⟩
          simp [hRole]
      | cons next rest =>
          simp [defendantOptionalSequence] at h

/--
The structural phase predicate implies the simpler fairness invariant.

This theorem compresses the engine's full sequence predicate into the facts a
reader usually wants first.  At every well-shaped point in the merits process:

1. In openings, arguments, and closings, the defendant never has more filings
   than the plaintiff, and neither side has more than one filing.
2. Rebuttal remains plaintiff-only and at most once.
3. Surrebuttal remains defendant-only and at most once.

The theorem does not say that `phaseShape` is unnecessary.  The stronger
predicate still carries the full sequence order.  It says that the stronger
predicate entails a clean fairness summary.
-/
theorem phaseShape_implies_proceduralParity
    (c : ArbitrationCase)
    (hShape : phaseShape c) :
    proceduralParity c := by
  by_cases hOpen : c.phase = "openings"
  · simp [phaseShape, hOpen] at hShape
    rcases hShape with ⟨hOpenings, hArgs, hRebuttals, hSurrebuttals, hClosings⟩
    rcases bilateralStarted_implies_bilateral_parity "openings" c.openings hOpenings with
      ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
    refine ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, ?_, ?_, ?_, ?_, ?_, ?_, ?_, ?_, ?_⟩
    all_goals simp [filingCount, hArgs, hRebuttals, hSurrebuttals, hClosings]
  · by_cases hArguments : c.phase = "arguments"
    · simp [phaseShape, hArguments] at hShape
      rcases hShape with ⟨hOpenings, hArgumentsShape, hRebuttals, hSurrebuttals, hClosings⟩
      rcases bilateralComplete_implies_bilateral_parity "openings" c.openings hOpenings with
        ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
      rcases bilateralStarted_implies_bilateral_parity "arguments" c.arguments hArgumentsShape with
        ⟨hArgPlaintiff, hArgDefendant, hArgOrder⟩
      refine ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, hArgPlaintiff, hArgDefendant, hArgOrder,
        ?_, ?_, ?_, ?_, ?_, ?_⟩
      all_goals simp [filingCount, hRebuttals, hSurrebuttals, hClosings]
    · by_cases hRebuttalsPhase : c.phase = "rebuttals"
      · simp [phaseShape, hRebuttalsPhase] at hShape
        rcases hShape with ⟨hOpenings, hArgumentsShape, hRebuttals, hSurrebuttals, hClosings⟩
        rcases bilateralComplete_implies_bilateral_parity "openings" c.openings hOpenings with
          ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
        rcases bilateralComplete_implies_bilateral_parity "arguments" c.arguments hArgumentsShape with
          ⟨hArgPlaintiff, hArgDefendant, hArgOrder⟩
        refine ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, hArgPlaintiff, hArgDefendant, hArgOrder,
          ?_, ?_, ?_, ?_, ?_, ?_⟩
        all_goals simp [filingCount, hRebuttals, hSurrebuttals, hClosings]
      · by_cases hSurrebuttalsPhase : c.phase = "surrebuttals"
        · simp [phaseShape, hSurrebuttalsPhase] at hShape
          rcases hShape with ⟨hOpenings, hArgumentsShape, hRebuttals, hSurrebuttals, hClosings⟩
          rcases bilateralComplete_implies_bilateral_parity "openings" c.openings hOpenings with
            ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
          rcases bilateralComplete_implies_bilateral_parity "arguments" c.arguments hArgumentsShape with
            ⟨hArgPlaintiff, hArgDefendant, hArgOrder⟩
          rcases plaintiffOptionalSequence_implies_optional_parity "rebuttals" c.rebuttals hRebuttals with
            ⟨hRebPlaintiff, hRebDefendant⟩
          refine ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, hArgPlaintiff, hArgDefendant, hArgOrder,
            ?_, ?_, ?_, hRebPlaintiff, hRebDefendant, ?_⟩
          · simp [filingCount, hClosings]
          · simp [filingCount, hClosings]
          · simp [filingCount, hClosings]
          · simp [filingCount, hSurrebuttals]
        · by_cases hClosingsPhase : c.phase = "closings"
          · simp [phaseShape, hClosingsPhase] at hShape
            rcases hShape with ⟨hOpenings, hArgumentsShape, hRebuttals, hSurrebuttals, hClosings⟩
            rcases bilateralComplete_implies_bilateral_parity "openings" c.openings hOpenings with
              ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
            rcases bilateralComplete_implies_bilateral_parity "arguments" c.arguments hArgumentsShape with
              ⟨hArgPlaintiff, hArgDefendant, hArgOrder⟩
            rcases bilateralStarted_implies_bilateral_parity "closings" c.closings hClosings with
              ⟨hClosePlaintiff, hCloseDefendant, hCloseOrder⟩
            rcases plaintiffOptionalSequence_implies_optional_parity "rebuttals" c.rebuttals hRebuttals with
              ⟨hRebPlaintiff, hRebDefendant⟩
            rcases defendantOptionalSequence_implies_optional_parity "surrebuttals" c.surrebuttals hSurrebuttals with
              ⟨hSurPlaintiff, hSurDefendant⟩
            exact ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, hArgPlaintiff, hArgDefendant, hArgOrder,
              hClosePlaintiff, hCloseDefendant, hCloseOrder, hRebPlaintiff, hRebDefendant, hSurPlaintiff, hSurDefendant⟩
          · by_cases hDeliberationPhase : c.phase = "deliberation"
            · simp [phaseShape, hDeliberationPhase] at hShape
              rcases hShape with ⟨hOpenings, hArgumentsShape, hRebuttals, hSurrebuttals, hClosings⟩
              rcases bilateralComplete_implies_bilateral_parity "openings" c.openings hOpenings with
                ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
              rcases bilateralComplete_implies_bilateral_parity "arguments" c.arguments hArgumentsShape with
                ⟨hArgPlaintiff, hArgDefendant, hArgOrder⟩
              rcases bilateralComplete_implies_bilateral_parity "closings" c.closings hClosings with
                ⟨hClosePlaintiff, hCloseDefendant, hCloseOrder⟩
              rcases plaintiffOptionalSequence_implies_optional_parity "rebuttals" c.rebuttals hRebuttals with
                ⟨hRebPlaintiff, hRebDefendant⟩
              rcases defendantOptionalSequence_implies_optional_parity "surrebuttals" c.surrebuttals hSurrebuttals with
                ⟨hSurPlaintiff, hSurDefendant⟩
              exact ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, hArgPlaintiff, hArgDefendant, hArgOrder,
                hClosePlaintiff, hCloseDefendant, hCloseOrder, hRebPlaintiff, hRebDefendant, hSurPlaintiff, hSurDefendant⟩
            · by_cases hClosedPhase : c.phase = "closed"
              · simp [phaseShape, hClosedPhase] at hShape
                rcases hShape with ⟨hOpenings, hArgumentsShape, hRebuttals, hSurrebuttals, hClosings⟩
                rcases bilateralComplete_implies_bilateral_parity "openings" c.openings hOpenings with
                  ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder⟩
                rcases bilateralComplete_implies_bilateral_parity "arguments" c.arguments hArgumentsShape with
                  ⟨hArgPlaintiff, hArgDefendant, hArgOrder⟩
                rcases bilateralComplete_implies_bilateral_parity "closings" c.closings hClosings with
                  ⟨hClosePlaintiff, hCloseDefendant, hCloseOrder⟩
                rcases plaintiffOptionalSequence_implies_optional_parity "rebuttals" c.rebuttals hRebuttals with
                  ⟨hRebPlaintiff, hRebDefendant⟩
                rcases defendantOptionalSequence_implies_optional_parity "surrebuttals" c.surrebuttals hSurrebuttals with
                  ⟨hSurPlaintiff, hSurDefendant⟩
                exact ⟨hOpenPlaintiff, hOpenDefendant, hOpenOrder, hArgPlaintiff, hArgDefendant, hArgOrder,
                  hClosePlaintiff, hCloseDefendant, hCloseOrder, hRebPlaintiff, hRebDefendant, hSurPlaintiff, hSurDefendant⟩
              · simp [phaseShape] at hShape

/--
In rebuttals, an empty rebuttal list yields exactly one live plaintiff
opportunity.

The theorem states the precise contract exposed by the engine: the role is
`plaintiff`, the phase is `rebuttals`, the opportunity may be passed, and the
allowed tools are exactly rebuttal submission and phase pass.
-/
theorem nextOpportunityForPhase_offers_rebuttal_only_to_plaintiff
    (s : ArbitrationState)
    (hPhase : s.case.phase = "rebuttals")
    (hEmpty : s.case.rebuttals = []) :
    let result := nextOpportunityForPhase s
    result.terminal = false ∧
      result.reason = "" ∧
      result.opportunity = some {
        opportunity_id := "rebuttals:plaintiff"
        role := "plaintiff"
        phase := "rebuttals"
        may_pass := true
        objective := "plaintiff rebuttal"
        allowed_tools := ["submit_evidence", "submit_rebuttal", "pass_phase_opportunity"]
      } := by
  simp [nextOpportunityForPhase, hPhase, hEmpty]

/--
Once a rebuttal exists, the engine offers no second rebuttal turn.

This is the single-turn part of the fairness rule.  The plaintiff may submit
one rebuttal or pass.  After that, rebuttals are over.
-/
theorem nextOpportunityForPhase_closes_rebuttals_after_one_turn
    (s : ArbitrationState)
    (hPhase : s.case.phase = "rebuttals")
    (hFilled : s.case.rebuttals ≠ []) :
    let result := nextOpportunityForPhase s
    result.terminal = true ∧
      result.reason = "no_rebuttal_opportunity" ∧
      result.opportunity = none := by
  have hEmpty : s.case.rebuttals.isEmpty = false := by
    cases hList : s.case.rebuttals with
    | nil =>
        exfalso
        exact hFilled hList
    | cons x xs =>
        simp
  simp [nextOpportunityForPhase, hPhase, hEmpty]

/--
In surrebuttals, an empty surrebuttal list yields exactly one live defendant
opportunity.

This is the mirror of the rebuttal theorem.  The engine never offers a
surrebuttal opportunity to the plaintiff.
-/
theorem nextOpportunityForPhase_offers_surrebuttal_only_to_defendant
    (s : ArbitrationState)
    (hPhase : s.case.phase = "surrebuttals")
    (hEmpty : s.case.surrebuttals = []) :
    let result := nextOpportunityForPhase s
    result.terminal = false ∧
      result.reason = "" ∧
      result.opportunity = some {
        opportunity_id := "surrebuttals:defendant"
        role := "defendant"
        phase := "surrebuttals"
        may_pass := true
        objective := "defendant surrebuttal"
        allowed_tools := ["submit_evidence", "submit_surrebuttal", "pass_phase_opportunity"]
      } := by
  simp [nextOpportunityForPhase, hPhase, hEmpty]

/--
Once a surrebuttal exists, the engine offers no second surrebuttal turn.

This closes the optional-phase pair.  After one defendant surrebuttal, or after
the defendant passes, the next live phase is closings rather than another
surrebuttal.
-/
theorem nextOpportunityForPhase_closes_surrebuttals_after_one_turn
    (s : ArbitrationState)
    (hPhase : s.case.phase = "surrebuttals")
    (hFilled : s.case.surrebuttals ≠ []) :
    let result := nextOpportunityForPhase s
    result.terminal = true ∧
      result.reason = "no_surrebuttal_opportunity" ∧
      result.opportunity = none := by
  have hEmpty : s.case.surrebuttals.isEmpty = false := by
    cases hList : s.case.surrebuttals with
    | nil =>
        exfalso
        exact hFilled hList
    | cons x xs =>
        simp
  simp [nextOpportunityForPhase, hPhase, hEmpty]

end ArbProofs
