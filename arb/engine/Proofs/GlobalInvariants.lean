import Proofs.ProcedureShape
import Proofs.AggregateLimits

namespace ArbProofs

/-
This file lifts the local proofs into state invariants.

The earlier files proved three kinds of facts.

First, initialization produces the intended starting state.

Second, `phaseShape` summarizes the merits sequence, and `proceduralParity`
extracts the fairness meaning of that structure.

Third, the admitted-material caps are preserved when one side appends a batch
of exhibits and reports that still fits within the remaining budget.

The next step is to connect those facts to `Reachable`.  That means proving two
things about successful transitions:

1. The filing sequence remains well-formed.
2. The cumulative exhibit and report caps remain respected.

This file begins with the structure lemmas that the full induction needs.
-/

/--
Successful initialization starts in the opening-shape state.

Initialization does not merely set `phase := "openings"`.  It also clears all
later filings.  That combination is exactly the opening branch of `phaseShape`.
-/
theorem initializeCase_establishes_phaseShape
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    phaseShape s.case := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error err =>
      simp [hPolicy] at hInit
      cases hInit
  | ok okv =>
      cases okv
      by_cases hProposition : trimString req.proposition = ""
      · simp [hPolicy, hProposition] at hInit
        cases hInit
      · by_cases hEvidence : trimString req.state.policy.evidence_standard = ""
        · simp [hPolicy, hProposition, hEvidence] at hInit
          cases hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hProposition, hEvidence, hEmpty] at hInit
            cases hInit
          · by_cases hLength : req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength] at hInit
              cases hInit
            · by_cases hInvalid : hasInvalidCouncilMemberIds req.council_members
              · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid] at hInit
                cases hInit
              · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate] at hInit
                  cases hInit
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate, Pure.pure] at hInit
                  cases hInit
                  simp [phaseShape, bilateralStarted, stateWithCase]

/--
Successful initialization also starts with empty admitted-material lists.

`initializeCase` clears `offered_evidence` and `technical_reports`.  Because the
side-level limits are natural-number upper bounds, empty lists satisfy them
immediately.
-/
theorem initializeCase_establishes_materialLimits
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    materialLimitsRespected s := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error err =>
      simp [hPolicy] at hInit
      cases hInit
  | ok okv =>
      cases okv
      by_cases hProposition : trimString req.proposition = ""
      · simp [hPolicy, hProposition] at hInit
        cases hInit
      · by_cases hEvidence : trimString req.state.policy.evidence_standard = ""
        · simp [hPolicy, hProposition, hEvidence] at hInit
          cases hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hProposition, hEvidence, hEmpty] at hInit
            cases hInit
          · by_cases hLength : req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength] at hInit
              cases hInit
            · by_cases hInvalid : hasInvalidCouncilMemberIds req.council_members
              · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid] at hInit
                cases hInit
              · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate] at hInit
                  cases hInit
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate, Pure.pure] at hInit
                  cases hInit
                  simp [materialLimitsRespected, offeredCount, reportCount, stateWithCase]

/--
Appending supplemental materials does not affect the filing shape.

`phaseShape` depends only on the merits lists and the current phase.  The
append function changes neither.
-/
theorem appendSupplementalMaterials_preserves_phaseShape
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hShape : phaseShape c) :
    phaseShape (appendSupplementalMaterials c offered reports) := by
  cases hPhase : c.phase <;> simpa [appendSupplementalMaterials, phaseShape, hPhase] using hShape

theorem appendSubmittedEvidence_preserves_phaseShape
    (c : ArbitrationCase)
    (evidence : SubmittedEvidence)
    (hShape : phaseShape c) :
    phaseShape (appendSubmittedEvidence c evidence) := by
  cases hPhase : c.phase <;> simpa [appendSubmittedEvidence, phaseShape, hPhase] using hShape

/--
Adding an opening to a well-shaped opening state preserves the global shape.

There are only two live opening configurations.

If no opening exists, the added filing must be the plaintiff's, and the case
remains in the `openings` branch of `phaseShape`.

If one opening exists, that opening must already be the plaintiff's.  The new
filing is the defendant's, and the case advances to `arguments`.
-/
theorem addOpening_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "openings") :
    phaseShape (addFiling c "openings" (if c.openings.isEmpty then "plaintiff" else "defendant") text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArgs, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.openings with
  | nil =>
      simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralStarted,
        hArgs, hRebuttals, hSurrebuttals, hClosings]
  | cons first rest =>
      cases rest with
      | nil =>
          have hOne : first.phase = "openings" ∧ first.role = "plaintiff" := by
            simpa [bilateralStarted, hList] using hOpenings
          rcases hOne with ⟨hFirstPhase, hFirstRole⟩
          simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralComplete, bilateralStarted,
            hArgs, hRebuttals, hSurrebuttals, hClosings, hFirstPhase, hFirstRole]
      | cons second tail =>
          have : False := by
            simp [bilateralStarted, hList] at hOpenings
          exact False.elim this

/--
Adding an argument preserves the filing shape.

The opening phase is already complete when arguments begin.  The argument list
is therefore either empty or contains the plaintiff's single argument.  The new
filing keeps the case in `arguments` or advances it to `rebuttals`.
-/
theorem addArgument_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "arguments") :
    phaseShape (addFiling c "arguments" (if c.arguments.isEmpty then "plaintiff" else "defendant") text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.arguments with
  | nil =>
      simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralStarted,
        hOpenings, hRebuttals, hSurrebuttals, hClosings]
  | cons first rest =>
      cases rest with
      | nil =>
          have hOne : first.phase = "arguments" ∧ first.role = "plaintiff" := by
            simpa [bilateralStarted, hList] using hArguments
          rcases hOne with ⟨hFirstPhase, hFirstRole⟩
          simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape]
          refine ⟨hOpenings, ?_, hRebuttals, hSurrebuttals, hClosings⟩
          simp [bilateralComplete, hFirstPhase, hFirstRole]
      | cons second tail =>
          have : False := by
            simp [bilateralStarted, hList] at hArguments
          exact False.elim this

/--
Adding the single rebuttal advances the case to surrebuttals.

The rebuttal branch of `phaseShape` states that the rebuttal list is empty.
The first plaintiff rebuttal therefore completes the whole phase.
-/
theorem addRebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "rebuttals") :
    phaseShape (addFiling c "rebuttals" "plaintiff" text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  simpa [addFiling, advanceAfterMerits, hPhase, phaseShape, hRebuttals, plaintiffOptionalSequence] using
    (show bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals"
          ([{ phase := "rebuttals", role := "plaintiff", text := text }]) ∧
        c.surrebuttals = [] ∧
        c.closings = [] from
      ⟨hOpenings, hArguments, by simp [plaintiffOptionalSequence], hSurrebuttals, hClosings⟩)

/--
Adding the single surrebuttal advances the case to closings.

The surrebuttal branch mirrors rebuttal.  The list is empty before the filing.
After one defendant surrebuttal, the case opens the closing phase.
-/
theorem addSurrebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "surrebuttals") :
    phaseShape (addFiling c "surrebuttals" "defendant" text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  simpa [addFiling, advanceAfterMerits, hPhase, phaseShape, hSurrebuttals, defendantOptionalSequence] using
    (show bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        defendantOptionalSequence "surrebuttals"
          ([{ phase := "surrebuttals", role := "defendant", text := text }]) ∧
        bilateralStarted "closings" c.closings from
      ⟨hOpenings, hArguments, hRebuttals, by simp [defendantOptionalSequence], by simp [bilateralStarted, hClosings]⟩)

/--
Adding a closing preserves the global shape.

The closing branch is the second bilateral phase after the optional filings.
As with openings and arguments, the list is either empty or contains the
plaintiff's closing.  The second closing moves the case to deliberation.
-/
theorem addClosing_preserves_phaseShape
    (c : ArbitrationCase)
    (text : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "closings") :
    phaseShape (addFiling c "closings" (if c.closings.isEmpty then "plaintiff" else "defendant") text) := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  cases hList : c.closings with
  | nil =>
      simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape, bilateralStarted,
        hOpenings, hArguments, hRebuttals, hSurrebuttals]
  | cons first rest =>
      cases rest with
      | nil =>
          have hOne : first.phase = "closings" ∧ first.role = "plaintiff" := by
            simpa [bilateralStarted, hList] using hClosings
          rcases hOne with ⟨hFirstPhase, hFirstRole⟩
          simp [addFiling, advanceAfterMerits, hPhase, hList, phaseShape]
          refine ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, ?_⟩
          simp [bilateralComplete, hFirstPhase, hFirstRole]
      | cons second tail =>
          have : False := by
            simp [bilateralStarted, hList] at hClosings
          exact False.elim this

/--
Passing the rebuttal opportunity preserves the shape and advances the phase.

The rebuttal branch of `phaseShape` already says the rebuttal list is empty.
Passing therefore changes only the phase marker.
-/
theorem passRebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "rebuttals") :
    phaseShape { c with phase := "surrebuttals" } := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  refine ⟨hOpenings, hArguments, ?_, ?_, ?_⟩
  · simp [plaintiffOptionalSequence, hRebuttals]
  · simp [hSurrebuttals]
  · simp [hClosings]

/--
Passing the surrebuttal opportunity preserves the shape and advances the phase.

The same reasoning applies one phase later.  The surrebuttal list is still
empty, so the pass changes only the phase marker.
-/
theorem passSurrebuttal_preserves_phaseShape
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "surrebuttals") :
    phaseShape { c with phase := "closings" } := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  refine ⟨hOpenings, hArguments, hRebuttals, ?_, ?_⟩
  · simp [defendantOptionalSequence, hSurrebuttals]
  · simp [bilateralStarted, hClosings]

/--
`stateWithCase` preserves any case-level invariant already proved for the new case.

The helper increments `state_version` and swaps in the supplied case.  It does
not transform the case again.  For `phaseShape`, the proof is therefore direct.
-/
theorem stateWithCase_preserves_phaseShape
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hShape : phaseShape c) :
    phaseShape (stateWithCase s c).case := by
  simpa [stateWithCase] using hShape

/--
Closing a deliberating case does not disturb the merits structure.

The `deliberation` and `closed` branches of `phaseShape` impose the same
requirements on openings, arguments, rebuttals, surrebuttals, and closings.
The only difference is the phase label itself.  Once the case reaches
deliberation with the full merits sequence complete, closing it preserves the
same structural fact.
-/
theorem closeDeliberation_preserves_phaseShape
    (c : ArbitrationCase)
    (resolution : String)
    (hShape : phaseShape c)
    (hPhase : c.phase = "deliberation") :
    phaseShape { c with status := "closed", phase := "closed", resolution := resolution } := by
  simp [phaseShape, hPhase] at hShape
  rcases hShape with ⟨hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings⟩
  simp [phaseShape, hOpenings, hArguments, hRebuttals, hSurrebuttals, hClosings]

/--
`continueDeliberation` preserves the merits structure.

This function decides whether the current votes already determine the outcome,
whether the remaining seated members are too few to reach the required vote
threshold, whether the round should advance, or whether deliberation should
continue in place.  None of those branches rewrite the merits filings.

The proof is therefore a case split on the same four possibilities.  The only
nontrivial branch is closure, where the phase changes from `deliberation` to
`closed`.  `closeDeliberation_preserves_phaseShape` isolates that fact.
-/
theorem continueDeliberation_preserves_phaseShape
    (s t : ArbitrationState)
    (hShape : phaseShape s.case)
    (hPhase : s.case.phase = "deliberation")
    (hCont : continueDeliberation s s.case = .ok t) :
    phaseShape t.case := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes s.case).length = seatedCouncilMemberCount s.case
  · cases hResolution : currentResolution? s.case s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        exact stateWithCase_preserves_phaseShape s _ <|
          closeDeliberation_preserves_phaseShape s.case resolution hShape hPhase
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount s.case < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          exact stateWithCase_preserves_phaseShape s _ <|
            closeDeliberation_preserves_phaseShape s.case "no_majority" hShape hPhase
        · by_cases hLastRound : s.case.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_phaseShape s _ <|
              closeDeliberation_preserves_phaseShape s.case "no_majority" hShape hPhase
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_phaseShape s _ <|
              (by simpa [phaseShape, hPhase] using hShape)
  · simp [hRoundComplete] at hCont
    cases hCont
    exact stateWithCase_preserves_phaseShape s _ <|
      (by simpa [phaseShape, hPhase] using hShape)

/--
`continueDeliberation` preserves the cumulative material limits.

Deliberation changes vote state, council seating, round number, status, phase,
and resolution.  It never rewrites `offered_evidence` or `technical_reports`.
That makes the proof simpler than the filing-shape version above.  Every branch
reduces to `stateWithCase_preserves_material_limits` with reflexive equalities
for the material lists.
-/
theorem continueDeliberation_preserves_material_limits
    (s t : ArbitrationState)
    (hBase : materialLimitsRespected s)
    (hCont : continueDeliberation s s.case = .ok t) :
    materialLimitsRespected t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes s.case).length = seatedCouncilMemberCount s.case
  · cases hResolution : currentResolution? s.case s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        exact stateWithCase_preserves_material_limits s _ hBase rfl rfl
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount s.case < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          exact stateWithCase_preserves_material_limits s _ hBase rfl rfl
        · by_cases hLastRound : s.case.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_material_limits s _ hBase rfl rfl
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_material_limits s _ hBase rfl rfl
  · simp [hRoundComplete] at hCont
    cases hCont
    exact stateWithCase_preserves_material_limits s _ hBase rfl rfl

end ArbProofs
