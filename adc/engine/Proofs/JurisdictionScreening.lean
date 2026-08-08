import Main

def dismissalTool : List String :=
  ["dismiss_for_lack_of_subject_matter_jurisdiction"]

def usDistrictProfile : Lean.Json :=
  Lean.Json.mkObj
    [ ("name", Lean.Json.str "United States District")
    , ("rules_markdown", Lean.Json.str "Federal civil rules apply.")
    , ("jurisdiction_screen", Lean.Json.bool true)
    , ("allowed_jurisdiction_bases", toJson (["federal_question", "diversity", "unspecified"] : List String))
    , ("require_jurisdiction_statement", Lean.Json.bool true)
    , ("require_diversity_citizenship", Lean.Json.bool true)
    , ("require_amount_in_controversy", Lean.Json.bool true)
    , ("minimum_amount_in_controversy", toJson (75000 : Nat))
    ]

def internationalClawProfile : Lean.Json :=
  Lean.Json.mkObj
    [ ("name", Lean.Json.str "International Claw District")
    , ("rules_markdown", Lean.Json.str "General civil jurisdiction applies.")
    , ("jurisdiction_screen", Lean.Json.bool false)
    , ("allowed_jurisdiction_bases", toJson (["general_civil"] : List String))
    , ("preferred_jurisdiction_basis", Lean.Json.str "general_civil")
    , ("require_jurisdiction_statement", Lean.Json.bool true)
    , ("require_diversity_citizenship", Lean.Json.bool false)
    , ("require_amount_in_controversy", Lean.Json.bool false)
    , ("minimum_amount_in_controversy", toJson (0 : Nat))
    ]

def completeDiversityCase : CaseState :=
  { (default : CaseState) with
    jurisdictional_allegations := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis", Lean.Json.str "diversity")
      , ("jurisdictional_statement", Lean.Json.str "The parties are citizens of different States and the amount in controversy exceeds $75,000.")
      , ("plaintiff_citizenship", Lean.Json.str "Texas")
      , ("defendant_citizenship", Lean.Json.str "New York")
      , ("amount_in_controversy", Lean.Json.str "96700")
      ]
  }

def missingAmountDiversityCase : CaseState :=
  { completeDiversityCase with
    jurisdictional_allegations := some <| Lean.Json.mkObj
      [ ("jurisdiction_basis", Lean.Json.str "diversity")
      , ("jurisdictional_statement", Lean.Json.str "The parties are citizens of different States and the amount in controversy exceeds $75,000.")
      , ("plaintiff_citizenship", Lean.Json.str "Texas")
      , ("defendant_citizenship", Lean.Json.str "New York")
      , ("amount_in_controversy", Lean.Json.str "")
      ]
  }

def completeDiversityState : CourtState :=
  { (default : CourtState) with
    court_name := "United States District"
    court_profile := some usDistrictProfile
    case := completeDiversityCase
  }

def missingAmountDiversityState : CourtState :=
  { completeDiversityState with case := missingAmountDiversityCase }

def noScreenState : CourtState :=
  { completeDiversityState with
    court_name := "International Claw District"
    court_profile := some internationalClawProfile
    case := missingAmountDiversityCase
  }

def judgeDismissReq (state : CourtState) : OpportunityRequest :=
  { state := state
  , roles := [{ role := "judge", allowed_tools := dismissalTool }]
  , max_steps_per_turn := 3
  }

/--
A complete diversity allegation in the United States District profile is not
facially defective.

The proof unfolds the federal screen and reduces every required field check on
this concrete state.
-/
theorem subjectMatterJurisdictionFaciallyDefective_complete_diversity_false :
    subjectMatterJurisdictionFaciallyDefective completeDiversityState = false := by
  unfold subjectMatterJurisdictionFaciallyDefective
    courtUsesJurisdictionScreen courtProfileBoolD
    hasJurisdictionalAllegations jurisdictionalFieldD jsonStringFieldD
    courtRequiresJurisdictionStatement courtRequiresDiversityCitizenship
    courtRequiresAmountInControversy courtMinimumAmountInControversy
    courtProfileNatD amountInControversyNat?
    completeDiversityState completeDiversityCase trimString
  native_decide

/-
This theorem fixes the baseline federal screen.  The selected court profile
matters now, so the proof works over the full `CourtState`.
-/

/--
An otherwise complete diversity allegation is facially defective in the United
States District profile when the amount-in-controversy allegation is blank.

The proof unfolds the same definitions and reduces the amount requirement to
`true`.
-/
theorem subjectMatterJurisdictionFaciallyDefective_missing_amount_true :
    subjectMatterJurisdictionFaciallyDefective missingAmountDiversityState = true := by
  unfold subjectMatterJurisdictionFaciallyDefective
    courtUsesJurisdictionScreen courtProfileBoolD
    hasJurisdictionalAllegations jurisdictionalFieldD jsonStringFieldD
    courtRequiresJurisdictionStatement courtRequiresDiversityCitizenship
    courtRequiresAmountInControversy courtMinimumAmountInControversy
    courtProfileNatD amountInControversyNat?
    missingAmountDiversityState missingAmountDiversityCase trimString
  native_decide

/-
This remains the smallest concrete federal defect worth proving.  The only
missing fact is the amount allegation.
-/

/--
The International Claw District profile disables the federal jurisdiction
screen, so the same defective diversity allegation is not facially defective.
-/
theorem subjectMatterJurisdictionFaciallyDefective_no_screen_false :
    subjectMatterJurisdictionFaciallyDefective noScreenState = false := by
  unfold subjectMatterJurisdictionFaciallyDefective courtUsesJurisdictionScreen
    courtProfileBoolD noScreenState
  native_decide

/-
This theorem captures the whole point of the second court profile.  The engine
does not run a federal subject-matter screen there.
-/

/--
If the complaint is not facially defective under the active court profile, the
jurisdiction-dismissal generator returns no candidates.
-/
theorem jurisdictionDismissalCandidates_nil_of_not_defective
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hdef : subjectMatterJurisdictionFaciallyDefectiveCase req.state c = false) :
    jurisdictionDismissalCandidates req c maxSteps = [] := by
  unfold jurisdictionDismissalCandidates
  simp [hdef]

/-
This is the basic negative screen theorem after the court-profile split.  The
candidate generator now depends on the court profile in `req.state`.
-/

/--
If a subject-matter-jurisdiction dismissal already appears in the trace, the
generator returns no new dismissal candidate.
-/
theorem jurisdictionDismissalCandidates_nil_of_prior_dismissal
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hjudgment : c.status ≠ "judgment_entered")
    (hclosed : c.status ≠ "closed")
    (hdef : subjectMatterJurisdictionFaciallyDefectiveCase req.state c = true)
    (htrace : hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction" = true) :
    jurisdictionDismissalCandidates req c maxSteps = [] := by
  unfold jurisdictionDismissalCandidates
  simp [hjudgment, hclosed, hdef, htrace]

/-
This still isolates the idempotence guard on the dismissal path.
-/

/--
If the complaint is facially defective under the active court profile, no prior
dismissal exists, and the judge has the dismissal tool, the generator returns
the singleton dismissal candidate.
-/
theorem jurisdictionDismissalCandidates_singleton_when_enabled
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hjudgment : c.status ≠ "judgment_entered")
    (hclosed : c.status ≠ "closed")
    (hdef : subjectMatterJurisdictionFaciallyDefectiveCase req.state c = true)
    (htrace : hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction" = false)
    (hallow : roleAllowsAll req.roles "judge" dismissalTool = true) :
    jurisdictionDismissalCandidates req c maxSteps =
      [{ (mkTurn "judge"
          "For case 0, if the complaint does not adequately allege subject-matter jurisdiction, dismiss for lack of subject-matter jurisdiction and state the rejected basis."
          dismissalTool false maxSteps) with
            priority := 10 }] := by
  unfold jurisdictionDismissalCandidates
  simp [hjudgment, hclosed, hdef, htrace]
  rw [show roleAllowsAll req.roles "judge" ["dismiss_for_lack_of_subject_matter_jurisdiction"] = true by
    simpa [dismissalTool] using hallow]
  rfl

/-
This states the exact candidate emitted when the active court profile still
uses the jurisdiction screen.
-/

/--
If the complaint is facially defective but the judge lacks the dismissal tool,
the generator returns no candidate.
-/
theorem jurisdictionDismissalCandidates_nil_when_judge_disabled
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hjudgment : c.status ≠ "judgment_entered")
    (hclosed : c.status ≠ "closed")
    (hdef : subjectMatterJurisdictionFaciallyDefectiveCase req.state c = true)
    (htrace : hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction" = false)
    (hallow : roleAllowsAll req.roles "judge" dismissalTool = false) :
    jurisdictionDismissalCandidates req c maxSteps = [] := by
  unfold jurisdictionDismissalCandidates
  simp [hjudgment, hclosed, hdef, htrace]
  rw [show roleAllowsAll req.roles "judge" ["dismiss_for_lack_of_subject_matter_jurisdiction"] = false by
    simpa [dismissalTool] using hallow]
  rfl

/-
This theorem still matters at the integration boundary.  The tool surface can
disable the screen even when the complaint is defective.
-/

/--
A complete diversity complaint in the United States District profile yields no
jurisdiction-dismissal candidate.
-/
theorem completeDiversityCase_has_no_jurisdictionDismissalCandidate :
    jurisdictionDismissalCandidates (judgeDismissReq completeDiversityState) completeDiversityCase 3 = [] := by
  native_decide

/--
A diversity complaint with no amount allegation in the United States District
profile yields the singleton jurisdiction-dismissal candidate.
-/
theorem missingAmountDiversityCase_emits_jurisdictionDismissalCandidate :
    jurisdictionDismissalCandidates (judgeDismissReq missingAmountDiversityState) missingAmountDiversityCase 3 =
      [{ (mkTurn "judge"
          "For case 0, if the complaint does not adequately allege subject-matter jurisdiction, dismiss for lack of subject-matter jurisdiction and state the rejected basis."
          dismissalTool false 3) with
            priority := 10 }] := by
  apply jurisdictionDismissalCandidates_singleton_when_enabled
  · native_decide
  · native_decide
  · exact subjectMatterJurisdictionFaciallyDefective_missing_amount_true
  · native_decide
  · native_decide

/--
The same defective diversity allegation yields no jurisdiction-dismissal
candidate in the International Claw District profile.
-/
theorem noScreenCase_has_no_jurisdictionDismissalCandidate :
    jurisdictionDismissalCandidates (judgeDismissReq noScreenState) missingAmountDiversityCase 3 = [] := by
  native_decide

/-
This is the concrete profile split: the federal court screens, the Claw court
does not.
-/

/--
If the jurisdiction-dismissal generator produces a candidate, the complaint is
facially defective under the active court profile.
-/
theorem jurisdictionDismissalCandidates_nonempty_implies_defective
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hnonempty : jurisdictionDismissalCandidates req c maxSteps ≠ []) :
    subjectMatterJurisdictionFaciallyDefectiveCase req.state c = true := by
  cases hscreen : subjectMatterJurisdictionFaciallyDefectiveCase req.state c
  · exfalso
    apply hnonempty
    exact jurisdictionDismissalCandidates_nil_of_not_defective req c maxSteps hscreen
  · simp

/-
This is the soundness theorem after the court-profile split.  Any emitted
candidate still implies a live defect.
-/

/--
If the generator produces a candidate, the case is not already closed.
-/
theorem jurisdictionDismissalCandidates_nonempty_implies_not_closed
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hnonempty : jurisdictionDismissalCandidates req c maxSteps ≠ []) :
    c.status ≠ "closed" := by
  intro hclosed
  apply hnonempty
  unfold jurisdictionDismissalCandidates
  simp [hclosed]

/--
If the generator produces a candidate, the case is not already at final
judgment.
-/
theorem jurisdictionDismissalCandidates_nonempty_implies_not_judgment_entered
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hnonempty : jurisdictionDismissalCandidates req c maxSteps ≠ []) :
    c.status ≠ "judgment_entered" := by
  intro hjudgment
  apply hnonempty
  unfold jurisdictionDismissalCandidates
  simp [hjudgment]

/--
If the generator produces a candidate, no earlier subject-matter-jurisdiction
dismissal appears in the trace.
-/
theorem jurisdictionDismissalCandidates_nonempty_implies_no_prior_dismissal
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hnonempty : jurisdictionDismissalCandidates req c maxSteps ≠ []) :
    hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction" = false := by
  cases htrace : hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction"
  · simp
  · exfalso
    apply hnonempty
    exact jurisdictionDismissalCandidates_nil_of_prior_dismissal req c maxSteps
      (jurisdictionDismissalCandidates_nonempty_implies_not_judgment_entered req c maxSteps hnonempty)
      (jurisdictionDismissalCandidates_nonempty_implies_not_closed req c maxSteps hnonempty)
      (jurisdictionDismissalCandidates_nonempty_implies_defective req c maxSteps hnonempty)
      htrace

/--
If the generator produces a candidate, the judge has the required dismissal
tool.
-/
theorem jurisdictionDismissalCandidates_nonempty_implies_judge_enabled
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hnonempty : jurisdictionDismissalCandidates req c maxSteps ≠ []) :
    roleAllowsAll req.roles "judge" dismissalTool = true := by
  cases hallow : roleAllowsAll req.roles "judge" dismissalTool
  · exfalso
    apply hnonempty
    exact jurisdictionDismissalCandidates_nil_when_judge_disabled req c maxSteps
      (jurisdictionDismissalCandidates_nonempty_implies_not_judgment_entered req c maxSteps hnonempty)
      (jurisdictionDismissalCandidates_nonempty_implies_not_closed req c maxSteps hnonempty)
      (jurisdictionDismissalCandidates_nonempty_implies_defective req c maxSteps hnonempty)
      (jurisdictionDismissalCandidates_nonempty_implies_no_prior_dismissal req c maxSteps hnonempty)
      hallow
  · simp

/--
If the generator produces any candidate, it produces exactly one candidate.
-/
theorem jurisdictionDismissalCandidates_nonempty_has_length_one
    (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat)
    (hnonempty : jurisdictionDismissalCandidates req c maxSteps ≠ []) :
    (jurisdictionDismissalCandidates req c maxSteps).length = 1 := by
  rw [jurisdictionDismissalCandidates_singleton_when_enabled req c maxSteps
    (jurisdictionDismissalCandidates_nonempty_implies_not_judgment_entered req c maxSteps hnonempty)
    (jurisdictionDismissalCandidates_nonempty_implies_not_closed req c maxSteps hnonempty)
    (jurisdictionDismissalCandidates_nonempty_implies_defective req c maxSteps hnonempty)
    (jurisdictionDismissalCandidates_nonempty_implies_no_prior_dismissal req c maxSteps hnonempty)
    (jurisdictionDismissalCandidates_nonempty_implies_judge_enabled req c maxSteps hnonempty)]
  simp

/-
This remains the endpoint theorem for the screen: when it fires, it emits one
deterministic candidate, and when the court disables the screen, it emits none.
-/
