import Proofs.StepPreservation
import Proofs.Reachability

namespace ArbProofs

/-
This file begins Stage 5 of the verification plan: record provenance and
monotonicity.

The aggregate-limit proofs already establish that the record does not grow past
the configured caps.  That is only part of the story.  A skeptical reader
should also be able to ask two direct questions.

First: where did each admitted exhibit or technical report come from?

Second: once an item enters the record, can a later public step rewrite or
delete it?

The engine stores enough information to answer both questions.  Each admitted
item records its `phase` and `role`, and the executable step functions add
supplemental materials in only one place: `recordMeritsSubmission`.  The
theorems below turn those implementation facts into two global statements.

`recordProvenance` says that every admitted item in a reachable state has one
of the allowed phase-role origins.

`materialsExtend` says that a later state can differ from an earlier state only
by appending more admitted items.  A successful step may append new items, or
it may leave both lists unchanged.  It does not rewrite prior entries.
-/

def materialOriginAllowed (phase role : String) : Prop :=
  (phase = "arguments" ∧ (role = "plaintiff" ∨ role = "defendant")) ∨
    (phase = "rebuttals" ∧ role = "plaintiff") ∨
    (phase = "surrebuttals" ∧ role = "defendant")

def offeredEvidenceOriginAllowed (item : OfferedEvidence) : Prop :=
  materialOriginAllowed item.phase item.role

def technicalReportOriginAllowed (item : TechnicalReport) : Prop :=
  materialOriginAllowed item.phase item.role

def recordProvenance (s : ArbitrationState) : Prop :=
  (∀ item ∈ s.case.offered_evidence, offeredEvidenceOriginAllowed item) ∧
    ∀ item ∈ s.case.technical_reports, technicalReportOriginAllowed item

def materialsExtend (s t : ArbitrationState) : Prop :=
  ∃ offered : List OfferedEvidence, ∃ reports : List TechnicalReport,
    t.case.offered_evidence = s.case.offered_evidence ++ offered ∧
      t.case.technical_reports = s.case.technical_reports ++ reports

/--
The two allowed material origins are exactly the two filing phases that admit
supplemental materials.
-/
theorem materialOriginAllowed_of_arguments
    {role : String}
    (hRole : role = "plaintiff" ∨ role = "defendant") :
    materialOriginAllowed "arguments" role := by
  exact Or.inl ⟨rfl, hRole⟩

theorem materialOriginAllowed_of_rebuttals :
    materialOriginAllowed "rebuttals" "plaintiff" := by
  exact Or.inr <| Or.inl ⟨rfl, rfl⟩

theorem materialOriginAllowed_of_surrebuttals :
    materialOriginAllowed "surrebuttals" "defendant" := by
  exact Or.inr <| Or.inr ⟨rfl, rfl⟩

/--
Every successfully parsed offered-file entry carries the phase supplied to the
parser.
-/
theorem parseOfferedEvidenceEntry_phase
    (entry : Lean.Json)
    (phase role : String)
    (item : OfferedEvidence)
    (hParse : parseOfferedEvidenceEntry entry phase role = .ok item) :
    item.phase = phase := by
  unfold parseOfferedEvidenceEntry at hParse
  cases hFileId : getString entry "evidence_id" with
  | error err =>
      rw [hFileId] at hParse
      cases hParse
  | ok rawFileId =>
      rw [hFileId] at hParse
      have hParse' :
          (if trimString rawFileId = "" then
              (Except.error "offered_evidence entry requires evidence_id" : Except String OfferedEvidence)
            else
              Except.ok
                { phase := phase
                  role := role
                  evidence_id := trimString rawFileId
                  label := getOptionalString entry "label" }) = .ok item := by
        simpa [Bind.bind, Except.bind] using hParse
      by_cases hEmpty : trimString rawFileId = ""
      · have : False := by
          have hBad :
              (Except.error "offered_evidence entry requires evidence_id" : Except String OfferedEvidence) = .ok item := by
            simp [hEmpty] at hParse'
          cases hBad
        contradiction
      · have hOk :
            (Except.ok
              { phase := phase
                role := role
                evidence_id := trimString rawFileId
                label := getOptionalString entry "label" } : Except String OfferedEvidence) = .ok item := by
          simpa [hEmpty] using hParse'
        cases hOk
        rfl

theorem parseOfferedEvidenceEntries_all_phase
    (entries : List Lean.Json)
    (phase role : String)
    (offered : List OfferedEvidence)
    (hParse : parseOfferedEvidenceEntries entries phase role = .ok offered) :
    ∀ item ∈ offered, item.phase = phase := by
  revert offered
  induction entries with
  | nil =>
      intro offered hParse item hMem
      unfold parseOfferedEvidenceEntries at hParse
      cases hParse
      simp at hMem
  | cons entry rest ih =>
      intro offered hParse item hMem
      unfold parseOfferedEvidenceEntries at hParse
      cases hEntry : parseOfferedEvidenceEntry entry phase role with
      | error err =>
          rw [hEntry] at hParse
          cases hParse
      | ok first =>
          have hFirstPhase : first.phase = phase := by
            exact parseOfferedEvidenceEntry_phase entry phase role first hEntry
          rw [hEntry] at hParse
          cases hRest : parseOfferedEvidenceEntries rest phase role with
          | error err =>
              rw [hRest] at hParse
              cases hParse
          | ok tail =>
              rw [hRest] at hParse
              cases hParse
              simp at hMem
              rcases hMem with rfl | hTailMem
              · exact hFirstPhase
              · exact ih tail hRest item hTailMem

theorem parseOfferedEvidence_all_phase
    (payload : Lean.Json)
    (phase role : String)
    (offered : List OfferedEvidence)
    (hParse : parseOfferedEvidence payload phase role = .ok offered) :
    ∀ item ∈ offered, item.phase = phase := by
  unfold parseOfferedEvidence at hParse
  cases hEntries : getOptionalArray payload "offered_evidence" with
  | error err =>
      rw [hEntries] at hParse
      cases hParse
  | ok entries =>
      rw [hEntries] at hParse
      exact parseOfferedEvidenceEntries_all_phase entries phase role offered hParse

/--
Every successfully parsed technical report entry carries the phase supplied to
the parser.
-/
theorem parseTechnicalReportEntry_phase
    (entry : Lean.Json)
    (phase role : String)
    (item : TechnicalReport)
    (hParse : parseTechnicalReportEntry entry phase role = .ok item) :
    item.phase = phase := by
  unfold parseTechnicalReportEntry at hParse
  cases hTitle : getString entry "title" with
  | error err =>
      rw [hTitle] at hParse
      cases hParse
  | ok rawTitle =>
      rw [hTitle] at hParse
      cases hSummary : getString entry "summary" with
      | error err =>
          rw [hSummary] at hParse
          cases hParse
      | ok rawSummary =>
          rw [hSummary] at hParse
          have hParse' :
              (if trimString rawTitle = "" then
                  (Except.error "technical_reports entry requires title" : Except String TechnicalReport)
                else if trimString rawSummary = "" then
                  Except.error "technical_reports entry requires summary"
                else
                  Except.ok
                    { phase := phase
                      role := role
                      title := trimString rawTitle
                      summary := trimString rawSummary }) = .ok item := by
            simpa [Bind.bind, Except.bind] using hParse
          by_cases hTitleEmpty : trimString rawTitle = ""
          · have : False := by
              have hBad :
                  (Except.error "technical_reports entry requires title" : Except String TechnicalReport) = .ok item := by
                simp [hTitleEmpty] at hParse'
              cases hBad
            contradiction
          · by_cases hSummaryEmpty : trimString rawSummary = ""
            · have : False := by
                have hBad :
                    (Except.error "technical_reports entry requires summary" : Except String TechnicalReport) = .ok item := by
                  simp [hTitleEmpty, hSummaryEmpty] at hParse'
                cases hBad
              contradiction
            · have hOk :
                  (Except.ok
                    { phase := phase
                      role := role
                      title := trimString rawTitle
                      summary := trimString rawSummary } : Except String TechnicalReport) = .ok item := by
                simpa [hTitleEmpty, hSummaryEmpty] using hParse'
              cases hOk
              rfl

theorem parseTechnicalReportEntries_all_phase
    (entries : List Lean.Json)
    (phase role : String)
    (reports : List TechnicalReport)
    (hParse : parseTechnicalReportEntries entries phase role = .ok reports) :
    ∀ item ∈ reports, item.phase = phase := by
  revert reports
  induction entries with
  | nil =>
      intro reports hParse item hMem
      unfold parseTechnicalReportEntries at hParse
      cases hParse
      simp at hMem
  | cons entry rest ih =>
      intro reports hParse item hMem
      unfold parseTechnicalReportEntries at hParse
      cases hEntry : parseTechnicalReportEntry entry phase role with
      | error err =>
          rw [hEntry] at hParse
          cases hParse
      | ok first =>
          have hFirstPhase : first.phase = phase := by
            exact parseTechnicalReportEntry_phase entry phase role first hEntry
          rw [hEntry] at hParse
          cases hRest : parseTechnicalReportEntries rest phase role with
          | error err =>
              rw [hRest] at hParse
              cases hParse
          | ok tail =>
              rw [hRest] at hParse
              cases hParse
              simp at hMem
              rcases hMem with rfl | hTailMem
              · exact hFirstPhase
              · exact ih tail hRest item hTailMem

theorem parseTechnicalReports_all_phase
    (payload : Lean.Json)
    (phase role : String)
    (reports : List TechnicalReport)
    (hParse : parseTechnicalReports payload phase role = .ok reports) :
    ∀ item ∈ reports, item.phase = phase := by
  unfold parseTechnicalReports at hParse
  cases hEntries : getOptionalArray payload "technical_reports" with
  | error err =>
      rw [hEntries] at hParse
      cases hParse
  | ok entries =>
      rw [hEntries] at hParse
      exact parseTechnicalReportEntries_all_phase entries phase role reports hParse

/--
Appending lists preserves a pointwise property when both inputs satisfy it.
-/
theorem append_preserves_membership_property
    {α : Type}
    (p : α → Prop)
    (xs ys : List α)
    (hxs : ∀ item ∈ xs, p item)
    (hys : ∀ item ∈ ys, p item) :
    ∀ item ∈ xs ++ ys, p item := by
  intro item hMem
  rcases List.mem_append.mp hMem with hLeft | hRight
  · exact hxs item hLeft
  · exact hys item hRight

/--
The initialized case starts with an empty admitted-material record.
-/
theorem initializeCase_establishes_recordProvenance
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    recordProvenance s := by
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
                  simp [recordProvenance, stateWithCase]

/--
If a replacement case keeps the admitted-material lists unchanged, it preserves
record provenance.
-/
theorem stateWithCase_preserves_recordProvenance
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports)
    (hProv : recordProvenance s) :
    recordProvenance (stateWithCase s c) := by
  simpa [recordProvenance, stateWithCase, hOffered, hReports] using hProv

/--
Appending newly admitted materials preserves record provenance when the new
items have allowed origins.
-/
theorem appendSupplementalMaterials_preserves_recordProvenance
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports)
    (hProv : recordProvenance s)
    (hOfferedNew : ∀ item ∈ offered, offeredEvidenceOriginAllowed item)
    (hReportsNew : ∀ item ∈ reports, technicalReportOriginAllowed item) :
    recordProvenance (stateWithCase s (appendSupplementalMaterials c offered reports)) := by
  rcases hProv with ⟨hOldOffered, hOldReports⟩
  refine ⟨?_, ?_⟩
  · intro item hMem
    simp [stateWithCase, appendSupplementalMaterials, hOffered] at hMem
    simp [offeredEvidenceOriginAllowed]
    rcases hMem with hOld | hNew
    · exact hOldOffered item hOld
    · exact hOfferedNew item hNew
  · intro item hMem
    simp [stateWithCase, appendSupplementalMaterials, hReports] at hMem
    simp [technicalReportOriginAllowed]
    rcases hMem with hOld | hNew
    · exact hOldReports item hOld
    · exact hReportsNew item hNew

/--
`materialsExtend` is reflexive.
-/
theorem materialsExtend_refl (s : ArbitrationState) : materialsExtend s s := by
  refine ⟨[], [], ?_, ?_⟩ <;> simp

/--
If one step extends the admitted-material lists, and the next step extends them
again, the composed run still extends them by appending a larger suffix.
-/
theorem materialsExtend_trans
    (s t u : ArbitrationState)
    (hST : materialsExtend s t)
    (hTU : materialsExtend t u) :
    materialsExtend s u := by
  rcases hST with ⟨offeredST, reportsST, hOfferedST, hReportsST⟩
  rcases hTU with ⟨offeredTU, reportsTU, hOfferedTU, hReportsTU⟩
  refine ⟨offeredST ++ offeredTU, reportsST ++ reportsTU, ?_, ?_⟩
  · calc
      u.case.offered_evidence = t.case.offered_evidence ++ offeredTU := hOfferedTU
      _ = (s.case.offered_evidence ++ offeredST) ++ offeredTU := by rw [hOfferedST]
      _ = s.case.offered_evidence ++ (offeredST ++ offeredTU) := by simp [List.append_assoc]
  · calc
      u.case.technical_reports = t.case.technical_reports ++ reportsTU := hReportsTU
      _ = (s.case.technical_reports ++ reportsST) ++ reportsTU := by rw [hReportsST]
      _ = s.case.technical_reports ++ (reportsST ++ reportsTU) := by simp [List.append_assoc]

/--
If a replacement case keeps the admitted-material lists unchanged, the new
state extends the old one by an empty suffix.
-/
theorem stateWithCase_extends_materials
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports) :
    materialsExtend s (stateWithCase s c) := by
  refine ⟨[], [], ?_, ?_⟩ <;> simp [stateWithCase, hOffered, hReports]

/--
Appending admitted materials extends the old record by exactly those appended
suffixes.
-/
theorem appendSupplementalMaterials_extends_materials
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports) :
    materialsExtend s (stateWithCase s (appendSupplementalMaterials c offered reports)) := by
  refine ⟨offered, reports, ?_, ?_⟩
  · simp [stateWithCase, appendSupplementalMaterials, hOffered]
  · simp [stateWithCase, appendSupplementalMaterials, hReports]

/--
`continueDeliberation` never rewrites the admitted-material lists.  It changes
only phase, status, resolution, votes, or the deliberation round.
-/
theorem continueDeliberation_preserves_recordProvenance_for
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hProv : recordProvenance s)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports)
    (hCont : continueDeliberation s c = .ok t) :
    recordProvenance t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        exact stateWithCase_preserves_recordProvenance s _ hOffered hReports hProv
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          exact stateWithCase_preserves_recordProvenance s _ hOffered hReports hProv
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact stateWithCase_preserves_recordProvenance s _ hOffered hReports hProv
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact stateWithCase_preserves_recordProvenance s _ hOffered hReports hProv
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    exact stateWithCase_preserves_recordProvenance s _ hOffered hReports hProv

/--
`continueDeliberation` also preserves the admitted-material lists by extension
with an empty suffix.
-/
theorem continueDeliberation_extends_materials_for
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports)
    (hCont : continueDeliberation s c = .ok t) :
    materialsExtend s t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution, stateWithCase] at hCont
        cases hCont
        exact stateWithCase_extends_materials s _ hOffered hReports
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew, stateWithCase] at hCont
          cases hCont
          exact stateWithCase_extends_materials s _ hOffered hReports
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact stateWithCase_extends_materials s _ hOffered hReports
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound, stateWithCase] at hCont
            cases hCont
            exact stateWithCase_extends_materials s _ hOffered hReports
  · simp [hRoundComplete, stateWithCase] at hCont
    cases hCont
    exact stateWithCase_extends_materials s _ hOffered hReports

theorem step_record_opening_statement_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact stateWithCase_preserves_recordProvenance s _
    (addFiling_preserves_offered_evidence s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_technical_reports s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    hProv

theorem step_record_opening_statement_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact stateWithCase_extends_materials s _
    (addFiling_preserves_offered_evidence s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_technical_reports s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))

theorem step_submit_argument_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
  rcases recordMeritsSubmission_with_materials_details
      s t "arguments" action.actor_role role
      "argument" s.policy.max_argument_chars action.payload
      (by simpa [stepCore, hType, role] using hStep) with
    ⟨rawText, offered, reports, hOfferedParse, hReportsParse, _hOfferedCap, _hReportsCap, rfl⟩
  have hRoleCase : role = "plaintiff" ∨ role = "defendant" := by
    unfold role
    by_cases hEmpty : s.case.arguments.isEmpty <;> simp [hEmpty]
  have hOfferedNew : ∀ item ∈ offered, offeredEvidenceOriginAllowed item := by
    intro item hMem
    have hItemPhase : item.phase = "arguments" :=
      parseOfferedEvidence_all_phase action.payload "arguments" role offered hOfferedParse item hMem
    have hItemRole : item.role = role :=
      parseOfferedEvidence_all_role action.payload "arguments" role offered hOfferedParse item hMem
    have hAllowed : materialOriginAllowed "arguments" role :=
      materialOriginAllowed_of_arguments hRoleCase
    simpa [offeredEvidenceOriginAllowed, hItemPhase, hItemRole] using hAllowed
  have hReportsNew : ∀ item ∈ reports, technicalReportOriginAllowed item := by
    intro item hMem
    have hItemPhase : item.phase = "arguments" :=
      parseTechnicalReports_all_phase action.payload "arguments" role reports hReportsParse item hMem
    have hItemRole : item.role = role :=
      parseTechnicalReports_all_role action.payload "arguments" role reports hReportsParse item hMem
    have hAllowed : materialOriginAllowed "arguments" role :=
      materialOriginAllowed_of_arguments hRoleCase
    simpa [technicalReportOriginAllowed, hItemPhase, hItemRole] using hAllowed
  exact appendSupplementalMaterials_preserves_recordProvenance s
    (addFiling s.case "arguments" role (trimString rawText))
    offered reports
    (addFiling_preserves_offered_evidence s.case "arguments" role (trimString rawText))
    (addFiling_preserves_technical_reports s.case "arguments" role (trimString rawText))
    hProv hOfferedNew hReportsNew

theorem step_submit_argument_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
  rcases recordMeritsSubmission_with_materials_result
      s t "arguments" action.actor_role role
      "argument" s.policy.max_argument_chars action.payload
      (by simpa [stepCore, hType, role] using hStep) with
    ⟨rawText, offered, reports, rfl⟩
  exact appendSupplementalMaterials_extends_materials s
    (addFiling s.case "arguments" role (trimString rawText))
    offered reports
    (addFiling_preserves_offered_evidence s.case "arguments" role (trimString rawText))
    (addFiling_preserves_technical_reports s.case "arguments" role (trimString rawText))

theorem step_submit_rebuttal_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  rcases recordMeritsSubmission_with_materials_details
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload
      (by simpa [stepCore, hType] using hStep) with
    ⟨rawText, offered, reports, hOfferedParse, hReportsParse, _hOfferedCap, _hReportsCap, rfl⟩
  have hOfferedNew : ∀ item ∈ offered, offeredEvidenceOriginAllowed item := by
    intro item hMem
    have hItemPhase : item.phase = "rebuttals" :=
      parseOfferedEvidence_all_phase action.payload "rebuttals" "plaintiff" offered hOfferedParse item hMem
    have hItemRole : item.role = "plaintiff" :=
      parseOfferedEvidence_all_role action.payload "rebuttals" "plaintiff" offered hOfferedParse item hMem
    simpa [offeredEvidenceOriginAllowed, hItemPhase, hItemRole] using materialOriginAllowed_of_rebuttals
  have hReportsNew : ∀ item ∈ reports, technicalReportOriginAllowed item := by
    intro item hMem
    have hItemPhase : item.phase = "rebuttals" :=
      parseTechnicalReports_all_phase action.payload "rebuttals" "plaintiff" reports hReportsParse item hMem
    have hItemRole : item.role = "plaintiff" :=
      parseTechnicalReports_all_role action.payload "rebuttals" "plaintiff" reports hReportsParse item hMem
    simpa [technicalReportOriginAllowed, hItemPhase, hItemRole] using materialOriginAllowed_of_rebuttals
  exact appendSupplementalMaterials_preserves_recordProvenance s
    (addFiling s.case "rebuttals" "plaintiff" (trimString rawText))
    offered reports
    (addFiling_preserves_offered_evidence s.case "rebuttals" "plaintiff" (trimString rawText))
    (addFiling_preserves_technical_reports s.case "rebuttals" "plaintiff" (trimString rawText))
    hProv hOfferedNew hReportsNew

theorem step_submit_rebuttal_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  rcases recordMeritsSubmission_with_materials_result
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload
      (by simpa [stepCore, hType] using hStep) with
    ⟨rawText, offered, reports, rfl⟩
  exact appendSupplementalMaterials_extends_materials s
    (addFiling s.case "rebuttals" "plaintiff" (trimString rawText))
    offered reports
    (addFiling_preserves_offered_evidence s.case "rebuttals" "plaintiff" (trimString rawText))
    (addFiling_preserves_technical_reports s.case "rebuttals" "plaintiff" (trimString rawText))

theorem step_submit_surrebuttal_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  rcases recordMeritsSubmission_with_materials_details
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload
      (by simpa [stepCore, hType] using hStep) with
    ⟨rawText, offered, reports, hOfferedParse, hReportsParse, _hOfferedCap, _hReportsCap, rfl⟩
  have hOfferedNew : ∀ item ∈ offered, offeredEvidenceOriginAllowed item := by
    intro item hMem
    have hItemPhase : item.phase = "surrebuttals" :=
      parseOfferedEvidence_all_phase action.payload "surrebuttals" "defendant" offered hOfferedParse item hMem
    have hItemRole : item.role = "defendant" :=
      parseOfferedEvidence_all_role action.payload "surrebuttals" "defendant" offered hOfferedParse item hMem
    simpa [offeredEvidenceOriginAllowed, hItemPhase, hItemRole] using materialOriginAllowed_of_surrebuttals
  have hReportsNew : ∀ item ∈ reports, technicalReportOriginAllowed item := by
    intro item hMem
    have hItemPhase : item.phase = "surrebuttals" :=
      parseTechnicalReports_all_phase action.payload "surrebuttals" "defendant" reports hReportsParse item hMem
    have hItemRole : item.role = "defendant" :=
      parseTechnicalReports_all_role action.payload "surrebuttals" "defendant" reports hReportsParse item hMem
    simpa [technicalReportOriginAllowed, hItemPhase, hItemRole] using materialOriginAllowed_of_surrebuttals
  exact appendSupplementalMaterials_preserves_recordProvenance s
    (addFiling s.case "surrebuttals" "defendant" (trimString rawText))
    offered reports
    (addFiling_preserves_offered_evidence s.case "surrebuttals" "defendant" (trimString rawText))
    (addFiling_preserves_technical_reports s.case "surrebuttals" "defendant" (trimString rawText))
    hProv hOfferedNew hReportsNew

theorem step_submit_surrebuttal_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  rcases recordMeritsSubmission_with_materials_result
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload
      (by simpa [stepCore, hType] using hStep) with
    ⟨rawText, offered, reports, rfl⟩
  exact appendSupplementalMaterials_extends_materials s
    (addFiling s.case "surrebuttals" "defendant" (trimString rawText))
    offered reports
    (addFiling_preserves_offered_evidence s.case "surrebuttals" "defendant" (trimString rawText))
    (addFiling_preserves_technical_reports s.case "surrebuttals" "defendant" (trimString rawText))

theorem step_submit_evidence_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  exact stateWithCase_preserves_recordProvenance s _
    (by simp [appendSubmittedEvidence])
    (by simp [appendSubmittedEvidence])
    hProv

theorem step_submit_evidence_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  exact stateWithCase_extends_materials s _
    (by simp [appendSubmittedEvidence])
    (by simp [appendSubmittedEvidence])

theorem step_deliver_closing_statement_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact stateWithCase_preserves_recordProvenance s _
    (addFiling_preserves_offered_evidence s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_technical_reports s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    hProv

theorem step_deliver_closing_statement_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact stateWithCase_extends_materials s _
    (addFiling_preserves_offered_evidence s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_technical_reports s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))

theorem step_pass_phase_opportunity_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · have hPass :
        (do
          requireRole action.actor_role "plaintiff"
          if !s.case.rebuttals.isEmpty then
            throw "rebuttal already submitted"
          pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
      simpa [stepCore, hType, hRebuttals] using hStep
    cases hRole : requireRole action.actor_role "plaintiff" with
    | error err =>
        rw [hRole] at hPass
        simp at hPass
        cases hPass
    | ok okv =>
        cases okv
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            exact stateWithCase_preserves_recordProvenance s _ rfl rfl hProv
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · have hPass :
          (do
            requireRole action.actor_role "defendant"
            if !s.case.surrebuttals.isEmpty then
              throw "surrebuttal already submitted"
            pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
        simpa [stepCore, hType, hRebuttals, hSurrebuttals] using hStep
      cases hRole : requireRole action.actor_role "defendant" with
      | error err =>
          rw [hRole] at hPass
          simp at hPass
          cases hPass
      | ok okv =>
          cases okv
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              exact stateWithCase_preserves_recordProvenance s _ rfl rfl hProv
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem step_pass_phase_opportunity_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · have hPass :
        (do
          requireRole action.actor_role "plaintiff"
          if !s.case.rebuttals.isEmpty then
            throw "rebuttal already submitted"
          pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
      simpa [stepCore, hType, hRebuttals] using hStep
    cases hRole : requireRole action.actor_role "plaintiff" with
    | error err =>
        rw [hRole] at hPass
        simp at hPass
        cases hPass
    | ok okv =>
        cases okv
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            exact stateWithCase_extends_materials s _ rfl rfl
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · have hPass :
          (do
            requireRole action.actor_role "defendant"
            if !s.case.surrebuttals.isEmpty then
              throw "surrebuttal already submitted"
            pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
        simpa [stepCore, hType, hRebuttals, hSurrebuttals] using hStep
      cases hRole : requireRole action.actor_role "defendant" with
      | error err =>
          rw [hRole] at hPass
          simp at hPass
          cases hPass
      | ok okv =>
          cases okv
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              exact stateWithCase_extends_materials s _ rfl rfl
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem step_submit_council_vote_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, _hPhase, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  exact continueDeliberation_preserves_recordProvenance_for s t c1 hProv
    (by simp [c1])
    (by simp [c1])
    hCont

theorem step_submit_council_vote_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, _hPhase, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  exact continueDeliberation_extends_materials_for s t c1
    (by simp [c1])
    (by simp [c1])
    hCont

theorem step_remove_council_member_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, _hPhase, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  exact continueDeliberation_preserves_recordProvenance_for s t c1 hProv
    (by simp [c1])
    (by simp [c1])
    hCont

theorem step_remove_council_member_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, _hPhase, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  exact continueDeliberation_extends_materials_for s t c1
    (by simp [c1])
    (by simp [c1])
    hCont

theorem failOpportunity_preserves_recordProvenance
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hProv : recordProvenance s)
    (hFail : failOpportunity s payload = .ok t) :
    recordProvenance t := by
  rcases failOpportunity_result s t payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
      _hDelib, _hSeated, _hFresh, hCont⟩
    exact continueDeliberation_preserves_recordProvenance_for s t c1 hProv
      (by rw [hC1])
      (by rw [hC1])
      hCont
  · rcases hParty with ⟨_failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact stateWithCase_preserves_recordProvenance s _ rfl rfl hProv

theorem failOpportunity_extends_materials
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hFail : failOpportunity s payload = .ok t) :
    materialsExtend s t := by
  rcases failOpportunity_result s t payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
      _hDelib, _hSeated, _hFresh, hCont⟩
    exact continueDeliberation_extends_materials_for s t c1
      (by rw [hC1])
      (by rw [hC1])
      hCont
  · rcases hParty with ⟨_failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact stateWithCase_extends_materials s _ rfl rfl

theorem step_fail_opportunity_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  have hStep' :
      (do
        requireRole action.actor_role "system"
        failOpportunity s action.payload) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "system" with
  | error err =>
      rw [hRole] at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      exact failOpportunity_preserves_recordProvenance s t action.payload hProv hStep'

theorem step_fail_opportunity_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  have hStep' :
      (do
        requireRole action.actor_role "system"
        failOpportunity s action.payload) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "system" with
  | error err =>
      rw [hRole] at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      exact failOpportunity_extends_materials s t action.payload hStep'

/--
Every successful public step preserves record provenance.
-/
theorem step_preserves_recordProvenance
    (s t : ArbitrationState)
    (action : CourtAction)
    (hProv : recordProvenance s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    recordProvenance t := by
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact step_record_opening_statement_preserves_recordProvenance s t action hOpening hProv hStep
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact step_submit_argument_preserves_recordProvenance s t action hArgument hProv hStep
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact step_submit_rebuttal_preserves_recordProvenance s t action hRebuttal hProv hStep
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact step_submit_surrebuttal_preserves_recordProvenance s t action hSurrebuttal hProv hStep
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · exact step_submit_evidence_preserves_recordProvenance s t action hEvidence hProv hStep
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · exact step_deliver_closing_statement_preserves_recordProvenance s t action hClosing hProv hStep
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · exact step_pass_phase_opportunity_preserves_recordProvenance s t action hPass hProv hStep
              · by_cases hVote : action.action_type = "submit_council_vote"
                · exact step_submit_council_vote_preserves_recordProvenance s t action hVote hProv hStep
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · exact step_remove_council_member_preserves_recordProvenance s t action hRemoval hProv hStep
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_preserves_recordProvenance s t action hFail hProv hStep
                    · simp [stepCore] at hStep

/--
Every successful public step extends the admitted-material lists by appending a
suffix or leaves them unchanged.
-/
theorem step_extends_materials
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialsExtend s t := by
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact step_record_opening_statement_extends_materials s t action hOpening hStep
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact step_submit_argument_extends_materials s t action hArgument hStep
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact step_submit_rebuttal_extends_materials s t action hRebuttal hStep
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact step_submit_surrebuttal_extends_materials s t action hSurrebuttal hStep
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · exact step_submit_evidence_extends_materials s t action hEvidence hStep
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · exact step_deliver_closing_statement_extends_materials s t action hClosing hStep
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · exact step_pass_phase_opportunity_extends_materials s t action hPass hStep
              · by_cases hVote : action.action_type = "submit_council_vote"
                · exact step_submit_council_vote_extends_materials s t action hVote hStep
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · exact step_remove_council_member_extends_materials s t action hRemoval hStep
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact step_fail_opportunity_extends_materials s t action hFail hStep
                    · simp [stepCore] at hStep

/--
Every reachable state satisfies record provenance.
-/
theorem reachable_recordProvenance
    (s : ArbitrationState)
    (hs : Reachable s) :
    recordProvenance s := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_recordProvenance req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_recordProvenance s t action ih
        (stepCore_ok_of_step_ok s t action hStep)

/--
Along any successful public run, the admitted-material lists change only by
appending suffixes.
-/
theorem stepReachableFrom_materialsExtend
    (start s : ArbitrationState)
    (hs : StepReachableFrom start s) :
    materialsExtend start s := by
  induction hs with
  | refl =>
      exact materialsExtend_refl start
  | step u t action hu hStep ih =>
      exact materialsExtend_trans start u t ih
        (step_extends_materials u t action (stepCore_ok_of_step_ok u t action hStep))

end ArbProofs
