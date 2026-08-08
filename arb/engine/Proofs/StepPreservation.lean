import Proofs.GlobalInvariants

namespace ArbProofs

/-
This file proves preservation at the `step` boundary.

The earlier files established two families of invariants:

1. `phaseShape`: the merits sequence is still in the intended order.
2. `materialLimitsRespected`: the admitted exhibits and reports still respect
   the policy's side-level caps.

The missing link is the public engine entry point.  `Reachable` is defined in
terms of successful calls to `step`, not in terms of the smaller helper
functions such as `addFiling` or `continueDeliberation`.  The induction
therefore needs one layer that translates from successful `step` calls into the
lower-level preservation lemmas already proved.

This file starts with the concrete `step` cases.  That keeps the proofs close
to the transition logic that readers actually inspect in `Main.lean`.
-/

/--
A successful opening-statement step has the expected result state.

The proof follows the executable logic line by line.  Success rules out the
error branches: openings must still be open, the acting side must be the one
the engine expects next, and the text must satisfy the opening limit.  Once
those branches disappear, the result is exactly `stateWithCase` applied to the
case produced by `addFiling`.
-/
theorem step_record_opening_statement_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ rawText : String,
      t =
        stateWithCase s
          (addFiling s.case "openings"
            (if s.case.openings.isEmpty then "plaintiff" else "defendant")
            (trimString rawText)) := by
  have hPhase : s.case.phase = "openings" := by
    by_cases hOpen : s.case.phase = "openings"
    · exact hOpen
    · have hClosed : s.case.phase != "openings" := by simpa using hOpen
      simp [stepCore, hType, hClosed] at hStep
      cases hStep
  let role := if s.case.openings.isEmpty then "plaintiff" else "defendant"
  have hStep' :
      (do
        requireRole action.actor_role (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        let rawText ← getString action.payload "text"
        let text := trimString rawText
        requireTextWithinLimit "opening statement" text s.policy.max_opening_chars
        pure <| stateWithCase s
          (addFiling s.case "openings"
            (if s.case.openings.isEmpty then "plaintiff" else "defendant") text)) = .ok t := by
            simpa [stepCore, hType, hPhase] using hStep
  cases hRoleCheck : requireRole action.actor_role role with
  | error err =>
      rw [hRoleCheck] at hStep'
      simp at hStep'
      cases hStep'
  | ok _ =>
      rw [hRoleCheck] at hStep'
      simp at hStep'
      cases hText : getString action.payload "text" with
      | error err =>
          rw [hText] at hStep'
          cases hStep'
      | ok rawText =>
          rw [hText] at hStep'
          have hStep'' :
              (do
                let text := trimString rawText
                requireTextWithinLimit "opening statement" text s.policy.max_opening_chars
                pure <| stateWithCase s
                  (addFiling s.case "openings"
                    (if s.case.openings.isEmpty then "plaintiff" else "defendant") text)) = .ok t := by
            simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
              Pure.pure, Except.pure] using hStep'
          cases hTextCheck :
              requireTextWithinLimit "opening statement" (trimString rawText) s.policy.max_opening_chars with
          | error err =>
              have hImpossible := hStep''
              simp [hTextCheck, Functor.map, Except.map] at hImpossible
          | ok okv =>
              cases okv
              have hDone :
                  stateWithCase s
                    (addFiling s.case "openings"
                      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
                      (trimString rawText)) = t := by
                simpa [hTextCheck, Functor.map, Except.map] using hStep''
              exact ⟨rawText, hDone.symm⟩

/--
A successful opening-statement step preserves the global filing shape.

The previous theorem identifies the exact state update performed by a
successful opening step.  Once that update is explicit, the preservation claim
reduces to the earlier helper theorem about `addFiling`.
-/
theorem step_record_opening_statement_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  have hPhase : s.case.phase = "openings" := by
    by_cases hOpen : s.case.phase = "openings"
    · exact hOpen
    · have hClosed : s.case.phase != "openings" := by simpa using hOpen
      simp [stepCore, hType, hClosed] at hStep
      cases hStep
  exact stateWithCase_preserves_phaseShape s _
    (addOpening_preserves_phaseShape s.case (trimString rawText) hShape hPhase)

/--
A successful opening-statement step preserves the aggregate material limits.

Openings add only a filing.  They never add exhibits or technical reports.
Once the previous theorem makes the resulting state explicit, the material-limit
claim reduces to `addFiling_preserves_offered_evidence`,
`addFiling_preserves_technical_reports`, and
`stateWithCase_preserves_material_limits`.
-/
theorem step_record_opening_statement_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact stateWithCase_preserves_material_limits s _ hLimits
    (addFiling_preserves_offered_evidence s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_technical_reports s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))

/--
A successful closing-statement step has the expected result state.

This is the closing-phase analogue of the opening result theorem above.  The
engine must still be in closings, the acting side must be the one currently
entitled to close, the text must satisfy the closing limit, and the closing
payload must not contain supplemental evidence fields.
-/
theorem step_deliver_closing_statement_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ rawText : String,
      t =
        stateWithCase s
          (addFiling s.case "closings"
            (if s.case.closings.isEmpty then "plaintiff" else "defendant")
            (trimString rawText)) := by
  have hPhase : s.case.phase = "closings" := by
    by_cases hOpen : s.case.phase = "closings"
    · exact hOpen
    · have hClosed : s.case.phase != "closings" := by simpa using hOpen
      simp [stepCore, hType, hClosed] at hStep
      cases hStep
  let role := if s.case.closings.isEmpty then "plaintiff" else "defendant"
  have hStep' :
      (do
        requireRole action.actor_role (if s.case.closings.isEmpty then "plaintiff" else "defendant")
        let rawText ← getString action.payload "text"
        let text := trimString rawText
        requireTextWithinLimit "closing statement" text s.policy.max_closing_chars
        requireNoSupplementalMaterials action.payload
        pure <| stateWithCase s
          (addFiling s.case "closings"
            (if s.case.closings.isEmpty then "plaintiff" else "defendant") text)) = .ok t := by
    simpa [stepCore, hType, hPhase] using hStep
  cases hRoleCheck : requireRole action.actor_role role with
  | error err =>
      rw [hRoleCheck] at hStep'
      simp at hStep'
      cases hStep'
  | ok _ =>
      rw [hRoleCheck] at hStep'
      simp at hStep'
      cases hText : getString action.payload "text" with
      | error err =>
          rw [hText] at hStep'
          cases hStep'
      | ok rawText =>
          rw [hText] at hStep'
          have hStep'' :
              (do
                let text := trimString rawText
                requireTextWithinLimit "closing statement" text s.policy.max_closing_chars
                requireNoSupplementalMaterials action.payload
                pure <| stateWithCase s
                  (addFiling s.case "closings"
                    (if s.case.closings.isEmpty then "plaintiff" else "defendant") text)) = .ok t := by
            simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
              Pure.pure, Except.pure] using hStep'
          cases hTextCheck :
              requireTextWithinLimit "closing statement" (trimString rawText) s.policy.max_closing_chars with
          | error err =>
              dsimp at hStep''
              rw [hTextCheck] at hStep''
              cases hStep''
          | ok okv =>
              cases okv
              cases hNoSupplemental : requireNoSupplementalMaterials action.payload with
              | error err =>
                  dsimp at hStep''
                  rw [hTextCheck] at hStep''
                  rw [hNoSupplemental] at hStep''
                  cases hStep''
              | ok okv =>
                  cases okv
                  have hDone :
                      stateWithCase s
                        (addFiling s.case "closings"
                          (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                          (trimString rawText)) = t := by
                    dsimp at hStep''
                    rw [hTextCheck] at hStep''
                    rw [hNoSupplemental] at hStep''
                    change
                      (Except.ok
                        (stateWithCase s
                          (addFiling s.case "closings"
                            (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                            (trimString rawText))) : Except String ArbitrationState) = .ok t at hStep''
                    cases hStep''
                    rfl
                  exact ⟨rawText, hDone.symm⟩

/--
A successful closing-statement step preserves the global filing shape.

The result theorem identifies the exact state change.  The substantive claim
then reduces to the earlier closing-phase helper.
-/
theorem step_deliver_closing_statement_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  have hPhase : s.case.phase = "closings" := by
    by_cases hOpen : s.case.phase = "closings"
    · exact hOpen
    · have hClosed : s.case.phase != "closings" := by simpa using hOpen
      simp [stepCore, hType, hClosed] at hStep
      cases hStep
  exact stateWithCase_preserves_phaseShape s _
    (addClosing_preserves_phaseShape s.case (trimString rawText) hShape hPhase)

/--
A successful closing-statement step preserves the aggregate material limits.

Closings do not add exhibits or reports.  The same two list-preservation lemmas
used for openings apply here as well.
-/
theorem step_deliver_closing_statement_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  exact stateWithCase_preserves_material_limits s _ hLimits
    (addFiling_preserves_offered_evidence s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))
    (addFiling_preserves_technical_reports s.case "closings"
      (if s.case.closings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText))

/--
Successful count checks establish the corresponding numerical inequality.

`requireCountWithinLimit` is the engine's executable guard for every aggregate
material cap.  The proofs below use this lemma to recover the arithmetic fact
from the successful check.
-/
theorem requireCountWithinLimit_ok_implies_le
    (label : String)
    (count limit : Nat)
    (hCheck : requireCountWithinLimit label count limit = .ok ()) :
    count ≤ limit := by
  unfold requireCountWithinLimit at hCheck
  by_cases hTooLarge : count > limit
  · simp [hTooLarge] at hCheck
  · exact Nat.le_of_not_gt hTooLarge

/--
Every successfully parsed offered-file entry carries the role supplied to the
parser.
-/
theorem parseOfferedEvidenceEntry_role
    (entry : Lean.Json)
    (phase role : String)
    (item : OfferedEvidence)
    (hParse : parseOfferedEvidenceEntry entry phase role = .ok item) :
    item.role = role := by
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
          have hBad : (Except.error "offered_evidence entry requires evidence_id" : Except String OfferedEvidence) = .ok item := by
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

theorem parseOfferedEvidenceEntries_all_role
    (entries : List Lean.Json)
    (phase role : String)
    (offered : List OfferedEvidence)
    (hParse : parseOfferedEvidenceEntries entries phase role = .ok offered) :
    ∀ item ∈ offered, item.role = role := by
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
          have hFirstRole : first.role = role := by
            exact parseOfferedEvidenceEntry_role entry phase role first hEntry
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
              · exact hFirstRole
              · exact ih tail hRest item hTailMem

/--
Every offered file in a successfully parsed batch carries the acting side.
-/
theorem parseOfferedEvidence_all_role
    (payload : Lean.Json)
    (phase role : String)
    (offered : List OfferedEvidence)
    (hParse : parseOfferedEvidence payload phase role = .ok offered) :
    ∀ item ∈ offered, item.role = role := by
  unfold parseOfferedEvidence at hParse
  cases hEntries : getOptionalArray payload "offered_evidence" with
  | error err =>
      rw [hEntries] at hParse
      cases hParse
  | ok entries =>
      rw [hEntries] at hParse
      exact parseOfferedEvidenceEntries_all_role entries phase role offered hParse

/--
Every successfully parsed technical report entry carries the role supplied to
the parser.
-/
theorem parseTechnicalReportEntry_role
    (entry : Lean.Json)
    (phase role : String)
    (item : TechnicalReport)
    (hParse : parseTechnicalReportEntry entry phase role = .ok item) :
    item.role = role := by
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
              have hBad : (Except.error "technical_reports entry requires title" : Except String TechnicalReport) = .ok item := by
                simp [hTitleEmpty] at hParse'
              cases hBad
            contradiction
          · by_cases hSummaryEmpty : trimString rawSummary = ""
            · have : False := by
                have hBad : (Except.error "technical_reports entry requires summary" : Except String TechnicalReport) = .ok item := by
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

theorem parseTechnicalReportEntries_all_role
    (entries : List Lean.Json)
    (phase role : String)
    (reports : List TechnicalReport)
    (hParse : parseTechnicalReportEntries entries phase role = .ok reports) :
    ∀ item ∈ reports, item.role = role := by
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
          have hFirstRole : first.role = role := by
            exact parseTechnicalReportEntry_role entry phase role first hEntry
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
              · exact hFirstRole
              · exact ih tail hRest item hTailMem

/--
Every technical report in a successfully parsed batch carries the acting side.
-/
theorem parseTechnicalReports_all_role
    (payload : Lean.Json)
    (phase role : String)
    (reports : List TechnicalReport)
    (hParse : parseTechnicalReports payload phase role = .ok reports) :
    ∀ item ∈ reports, item.role = role := by
  unfold parseTechnicalReports at hParse
  cases hEntries : getOptionalArray payload "technical_reports" with
  | error err =>
      rw [hEntries] at hParse
      cases hParse
  | ok entries =>
      rw [hEntries] at hParse
      exact parseTechnicalReportEntries_all_role entries phase role reports hParse

/--
Arguments and rebuttals share one engine helper with supplemental materials.

For the phase-shape theorem, the important fact is not the contents of the
incoming exhibits or reports.  It is the shape of the resulting case:

1. one merits filing is added; then
2. the new exhibits and reports are appended.

`phaseShape` ignores those appended material lists.  The structural proof
therefore needs only this result theorem and
`appendSupplementalMaterials_preserves_phaseShape`.
-/
theorem recordMeritsSubmission_with_materials_result
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit true payload = .ok t) :
    ∃ rawText : String, ∃ offered : List OfferedEvidence, ∃ reports : List TechnicalReport,
      t =
        stateWithCase s
          (appendSupplementalMaterials
            (addFiling s.case phase expectedRole (trimString rawText))
            offered
            reports) := by
  have hPhase : s.case.phase = phase := by
    by_cases hOpen : s.case.phase = phase
    · exact hOpen
    · have hClosed : s.case.phase != phase := by simpa using hOpen
      simp [recordMeritsSubmission, hClosed] at hSubmit
      cases hSubmit
  have hSubmit' :
      (do
        requireRole actorRole expectedRole
        let rawText ← getString payload "text"
        let text := trimString rawText
        requireTextWithinLimit textLabel text limit
        let offered ← parseOfferedEvidence payload phase expectedRole
        let reports ← parseTechnicalReports payload phase expectedRole
        requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
        requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
        let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
        let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
        requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
        requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
        pure <| stateWithCase s
          (appendSupplementalMaterials
            (addFiling s.case phase expectedRole (trimString rawText))
            offered
            reports)) = .ok t := by
    simpa [recordMeritsSubmission, hPhase] using hSubmit
  cases hRole : requireRole actorRole expectedRole with
  | error err =>
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hSubmit'
  | ok okv =>
      cases okv
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hText : getString payload "text" with
      | error err =>
          rw [hText] at hSubmit'
          cases hSubmit'
      | ok rawText =>
          rw [hText] at hSubmit'
          have hSubmit'' :
              (do
                let text := trimString rawText
                requireTextWithinLimit textLabel text limit
                let offered ← parseOfferedEvidence payload phase expectedRole
                let reports ← parseTechnicalReports payload phase expectedRole
                requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
                requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
                let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
                let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
                requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
                requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
                pure <| stateWithCase s
                  (appendSupplementalMaterials
                    (addFiling s.case phase expectedRole (trimString rawText))
                    offered
                    reports)) = .ok t := by
            simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
              Pure.pure, Except.pure] using hSubmit'
          cases hTextCheck : requireTextWithinLimit textLabel (trimString rawText) limit with
          | error err =>
              simp [hTextCheck] at hSubmit''
              cases hSubmit''
          | ok okv =>
              cases okv
              simp [hTextCheck] at hSubmit''
              cases hOffered : parseOfferedEvidence payload phase expectedRole with
              | error err =>
                  rw [hOffered] at hSubmit''
                  cases hSubmit''
              | ok offered =>
                  rw [hOffered] at hSubmit''
                  cases hReports : parseTechnicalReports payload phase expectedRole with
                  | error err =>
                      rw [hReports] at hSubmit''
                      cases hSubmit''
                  | ok reports =>
                      rw [hReports] at hSubmit''
                      have hSubmit''' :
                          (do
                            requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
                            requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
                            let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
                            let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
                            requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
                            requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
                            pure <| stateWithCase s
                              (appendSupplementalMaterials
                                (addFiling s.case phase expectedRole (trimString rawText))
                                offered
                                reports)) = .ok t := by
                        simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
                          Pure.pure, Except.pure] using hSubmit''
                      cases hOfferedPer :
                          requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing with
                      | error err =>
                          simp [hOfferedPer] at hSubmit'''
                          cases hSubmit'''
                      | ok okv =>
                          cases okv
                          simp [hOfferedPer] at hSubmit'''
                          cases hReportsPer :
                              requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing with
                          | error err =>
                              simp [hReportsPer] at hSubmit'''
                              cases hSubmit'''
                          | ok okv =>
                              cases okv
                              simp [hReportsPer] at hSubmit'''
                              let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
                              let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
                              have hSubmit'''' :
                                  (do
                                    requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
                                    requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
                                    pure <| stateWithCase s
                                      (appendSupplementalMaterials
                                        (addFiling s.case phase expectedRole (trimString rawText))
                                        offered
                                        reports)) = .ok t := by
                                simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
                                  Pure.pure, Except.pure] using hSubmit'''
                              cases hOfferedSide :
                                  requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side with
                              | error err =>
                                  simp [hOfferedSide] at hSubmit''''
                                  cases hSubmit''''
                              | ok okv =>
                                  cases okv
                                  simp [hOfferedSide] at hSubmit''''
                                  cases hReportsSide :
                                      requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side with
                                  | error err =>
                                      simp [hReportsSide] at hSubmit''''
                                      cases hSubmit''''
                                  | ok okv =>
                                      cases okv
                                      simp [hReportsSide] at hSubmit''''
                                      cases hSubmit''''
                                      exact ⟨rawText, offered, reports, rfl⟩

/--
Successful merits submission with supplemental materials exposes the parsed
material lists and the side-level cap facts that justified accepting them.

The shape theorem above is enough for `phaseShape`.  The aggregate material
theorems also need to know two more things:

1. which exhibit and report lists were parsed from the payload; and
2. that the side-level count checks succeeded for those lists.
-/
theorem recordMeritsSubmission_with_materials_details
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit true payload = .ok t) :
    ∃ rawText : String, ∃ offered : List OfferedEvidence, ∃ reports : List TechnicalReport,
      parseOfferedEvidence payload phase expectedRole = .ok offered ∧
      parseTechnicalReports payload phase expectedRole = .ok reports ∧
      offeredCount s.case.offered_evidence expectedRole + offered.length ≤ s.policy.max_exhibits_per_side ∧
      reportCount s.case.technical_reports expectedRole + reports.length ≤ s.policy.max_reports_per_side ∧
      t =
        stateWithCase s
          (appendSupplementalMaterials
            (addFiling s.case phase expectedRole (trimString rawText))
            offered
            reports) := by
  have hPhase : s.case.phase = phase := by
    by_cases hOpen : s.case.phase = phase
    · exact hOpen
    · have hClosed : s.case.phase != phase := by simpa using hOpen
      simp [recordMeritsSubmission, hClosed] at hSubmit
      cases hSubmit
  have hSubmit' :
      (do
        requireRole actorRole expectedRole
        let rawText ← getString payload "text"
        let text := trimString rawText
        requireTextWithinLimit textLabel text limit
        let offered ← parseOfferedEvidence payload phase expectedRole
        let reports ← parseTechnicalReports payload phase expectedRole
        requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
        requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
        let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
        let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
        requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
        requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
        pure <| stateWithCase s
          (appendSupplementalMaterials
            (addFiling s.case phase expectedRole (trimString rawText))
            offered
            reports)) = .ok t := by
    simpa [recordMeritsSubmission, hPhase] using hSubmit
  cases hRole : requireRole actorRole expectedRole with
  | error err =>
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hSubmit'
  | ok okv =>
      cases okv
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hText : getString payload "text" with
      | error err =>
          rw [hText] at hSubmit'
          cases hSubmit'
      | ok rawText =>
          rw [hText] at hSubmit'
          have hSubmit'' :
              (do
                let text := trimString rawText
                requireTextWithinLimit textLabel text limit
                let offered ← parseOfferedEvidence payload phase expectedRole
                let reports ← parseTechnicalReports payload phase expectedRole
                requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
                requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
                let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
                let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
                requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
                requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
                pure <| stateWithCase s
                  (appendSupplementalMaterials
                    (addFiling s.case phase expectedRole (trimString rawText))
                    offered
                    reports)) = .ok t := by
            simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
              Pure.pure, Except.pure] using hSubmit'
          cases hTextCheck : requireTextWithinLimit textLabel (trimString rawText) limit with
          | error err =>
              simp [hTextCheck] at hSubmit''
              cases hSubmit''
          | ok okv =>
              cases okv
              simp [hTextCheck] at hSubmit''
              cases hOffered : parseOfferedEvidence payload phase expectedRole with
              | error err =>
                  rw [hOffered] at hSubmit''
                  cases hSubmit''
              | ok offered =>
                  rw [hOffered] at hSubmit''
                  cases hReports : parseTechnicalReports payload phase expectedRole with
                  | error err =>
                      rw [hReports] at hSubmit''
                      cases hSubmit''
                  | ok reports =>
                      rw [hReports] at hSubmit''
                      have hSubmit''' :
                          (do
                            requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
                            requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
                            let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
                            let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
                            requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
                            requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
                            pure <| stateWithCase s
                              (appendSupplementalMaterials
                                (addFiling s.case phase expectedRole (trimString rawText))
                                offered
                                reports)) = .ok t := by
                        simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
                          Pure.pure, Except.pure] using hSubmit''
                      cases hOfferedPer :
                          requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing with
                      | error err =>
                          simp [hOfferedPer] at hSubmit'''
                          cases hSubmit'''
                      | ok okv =>
                          cases okv
                          simp [hOfferedPer] at hSubmit'''
                          cases hReportsPer :
                              requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing with
                          | error err =>
                              simp [hReportsPer] at hSubmit'''
                              cases hSubmit'''
                          | ok okv =>
                              cases okv
                              simp [hReportsPer] at hSubmit'''
                              let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
                              let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
                              have hSubmit'''' :
                                  (do
                                    requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
                                    requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
                                    pure <| stateWithCase s
                                      (appendSupplementalMaterials
                                        (addFiling s.case phase expectedRole (trimString rawText))
                                        offered
                                        reports)) = .ok t := by
                                simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
                                  Pure.pure, Except.pure] using hSubmit'''
                              cases hOfferedSide :
                                  requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side with
                              | error err =>
                                  simp [hOfferedSide] at hSubmit''''
                                  cases hSubmit''''
                              | ok okv =>
                                  cases okv
                                  simp [hOfferedSide] at hSubmit''''
                                  cases hReportsSide :
                                      requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side with
                                  | error err =>
                                      simp [hReportsSide] at hSubmit''''
                                      cases hSubmit''''
                                  | ok okv =>
                                      cases okv
                                      have hOfferedCap :
                                          totalOffered ≤ s.policy.max_exhibits_per_side := by
                                        exact requireCountWithinLimit_ok_implies_le
                                          "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side hOfferedSide
                                      have hReportsCap :
                                          totalReports ≤ s.policy.max_reports_per_side := by
                                        exact requireCountWithinLimit_ok_implies_le
                                          "technical_reports for this side" totalReports s.policy.max_reports_per_side hReportsSide
                                      have hOfferedCap' :
                                          offeredCount s.case.offered_evidence expectedRole + offered.length ≤
                                            s.policy.max_exhibits_per_side := by
                                        simpa [totalOffered, offeredEvidenceCountForRole_eq_offeredCount] using hOfferedCap
                                      have hReportsCap' :
                                          reportCount s.case.technical_reports expectedRole + reports.length ≤
                                            s.policy.max_reports_per_side := by
                                        simpa [totalReports, technicalReportCountForRole_eq_reportCount] using hReportsCap
                                      simp [hReportsSide] at hSubmit''''
                                      cases hSubmit''''
                                      exact ⟨rawText, offered, reports,
                                        rfl,
                                        rfl,
                                        hOfferedCap',
                                        hReportsCap',
                                        rfl⟩

/--
Surrebuttal uses the same helper without supplemental materials.

That produces a simpler result shape: one filing and no appended materials.
-/
theorem recordMeritsSubmission_without_materials_result
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit false payload = .ok t) :
    ∃ rawText : String,
      t = stateWithCase s (addFiling s.case phase expectedRole (trimString rawText)) := by
  have hPhase : s.case.phase = phase := by
    by_cases hOpen : s.case.phase = phase
    · exact hOpen
    · have hClosed : s.case.phase != phase := by simpa using hOpen
      simp [recordMeritsSubmission, hClosed] at hSubmit
      cases hSubmit
  have hSubmit' :
      (do
        requireRole actorRole expectedRole
        let rawText ← getString payload "text"
        let text := trimString rawText
        requireTextWithinLimit textLabel text limit
        requireNoSupplementalMaterials payload
        pure <| stateWithCase s (addFiling s.case phase expectedRole (trimString rawText))) = .ok t := by
    simpa [recordMeritsSubmission, hPhase] using hSubmit
  cases hRole : requireRole actorRole expectedRole with
  | error err =>
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hSubmit'
  | ok okv =>
      cases okv
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hText : getString payload "text" with
      | error err =>
          rw [hText] at hSubmit'
          cases hSubmit'
      | ok rawText =>
          rw [hText] at hSubmit'
          have hSubmit'' :
              (do
                let text := trimString rawText
                requireTextWithinLimit textLabel text limit
                requireNoSupplementalMaterials payload
                pure <| stateWithCase s (addFiling s.case phase expectedRole (trimString rawText))) = .ok t := by
            simpa [SeqRight.seqRight, Bind.bind, Except.bind, Functor.map, Except.map,
              Pure.pure, Except.pure] using hSubmit'
          cases hTextCheck : requireTextWithinLimit textLabel (trimString rawText) limit with
          | error err =>
              simp [hTextCheck] at hSubmit''
              cases hSubmit''
          | ok okv =>
              cases okv
              simp [hTextCheck] at hSubmit''
              cases hNoMaterials : requireNoSupplementalMaterials payload with
              | error err =>
                  simp [hNoMaterials] at hSubmit''
                  cases hSubmit''
              | ok okv =>
                  cases okv
                  simp [hNoMaterials] at hSubmit''
                  cases hSubmit''
                  exact ⟨rawText, rfl⟩

/--
A successful argument step preserves the merits-sequence invariant.

Arguments may append exhibits and technical reports, but those lists are
orthogonal to `phaseShape`.  The structural effect is the plaintiff or
defendant argument filing itself.
-/
theorem step_submit_argument_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  have hSubmit :
      recordMeritsSubmission
        s
        "arguments"
        action.actor_role
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        "argument"
        s.policy.max_argument_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_result
      s t "arguments" action.actor_role
      (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
      "argument" s.policy.max_argument_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  have hPhase : s.case.phase = "arguments" := by
    by_cases hArg : s.case.phase = "arguments"
    · exact hArg
    · have hClosed : s.case.phase != "arguments" := by simpa using hArg
      simp [recordMeritsSubmission, hClosed] at hSubmit
      cases hSubmit
  exact stateWithCase_preserves_phaseShape s _
    (appendSupplementalMaterials_preserves_phaseShape _ _ _
      (addArgument_preserves_phaseShape s.case (trimString rawText) hShape hPhase))

/--
A successful argument step preserves the aggregate material limits.

This theorem depends on two facts that are separate from the phase-shape proof:
the parser tags every new exhibit and report with the acting side, and the
side-level budget checks in `recordMeritsSubmission` match the append arithmetic
proved in `AggregateLimits.lean`.
-/
theorem step_submit_argument_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
  have hSubmit :
      recordMeritsSubmission
        s
        "arguments"
        action.actor_role
        role
        "argument"
        s.policy.max_argument_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType, role] using hStep
  rcases recordMeritsSubmission_with_materials_details
      s t "arguments" action.actor_role role
      "argument" s.policy.max_argument_chars action.payload hSubmit with
    ⟨rawText, offered, reports, hOfferedParse, hReportsParse, hOfferedCap, hReportCap, rfl⟩
  let c1 := addFiling s.case "arguments" role (trimString rawText)
  let s1 := stateWithCase s c1
  have hBase1 : materialLimitsRespected s1 := by
    exact stateWithCase_preserves_material_limits s c1 hLimits
      (addFiling_preserves_offered_evidence s.case "arguments" role (trimString rawText))
      (addFiling_preserves_technical_reports s.case "arguments" role (trimString rawText))
  have hOfferedRole : ∀ item ∈ offered, item.role = role := by
    exact parseOfferedEvidence_all_role action.payload "arguments" role offered hOfferedParse
  have hReportRole : ∀ item ∈ reports, item.role = role := by
    exact parseTechnicalReports_all_role action.payload "arguments" role reports hReportsParse
  have hOfferedCap1 :
      offeredCount s1.case.offered_evidence role + offered.length ≤ s1.policy.max_exhibits_per_side := by
    simpa [s1, c1, stateWithCase, addFiling_preserves_offered_evidence] using hOfferedCap
  have hReportCap1 :
      reportCount s1.case.technical_reports role + reports.length ≤ s1.policy.max_reports_per_side := by
    simpa [s1, c1, stateWithCase, addFiling_preserves_technical_reports] using hReportCap
  simpa [s1, c1, stateWithCase] using
    appendSupplementalMaterials_preserves_material_limits
      s1 offered reports role hBase1 hOfferedRole hReportRole hOfferedCap1 hReportCap1

/--
A successful rebuttal step preserves the merits-sequence invariant.

Rebuttal shares the same submission helper as arguments.  The structural change
is the plaintiff's single rebuttal filing, after which the case moves to
surrebuttals.
-/
theorem step_submit_rebuttal_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  have hSubmit :
      recordMeritsSubmission
        s
        "rebuttals"
        action.actor_role
        "plaintiff"
        "rebuttal"
        s.policy.max_rebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_result
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  have hPhase : s.case.phase = "rebuttals" := by
    by_cases hReb : s.case.phase = "rebuttals"
    · exact hReb
    · have hClosed : s.case.phase != "rebuttals" := by simpa using hReb
      simp [recordMeritsSubmission, hClosed] at hSubmit
      cases hSubmit
  exact stateWithCase_preserves_phaseShape s _
    (appendSupplementalMaterials_preserves_phaseShape _ _ _
      (addRebuttal_preserves_phaseShape s.case (trimString rawText) hShape hPhase))

/--
A successful rebuttal step preserves the aggregate material limits.
-/
theorem step_submit_rebuttal_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  have hSubmit :
      recordMeritsSubmission
        s
        "rebuttals"
        action.actor_role
        "plaintiff"
        "rebuttal"
        s.policy.max_rebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_details
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, hOfferedParse, hReportsParse, hOfferedCap, hReportCap, rfl⟩
  let c1 := addFiling s.case "rebuttals" "plaintiff" (trimString rawText)
  let s1 := stateWithCase s c1
  have hBase1 : materialLimitsRespected s1 := by
    exact stateWithCase_preserves_material_limits s c1 hLimits
      (addFiling_preserves_offered_evidence s.case "rebuttals" "plaintiff" (trimString rawText))
      (addFiling_preserves_technical_reports s.case "rebuttals" "plaintiff" (trimString rawText))
  have hOfferedRole : ∀ item ∈ offered, item.role = "plaintiff" := by
    exact parseOfferedEvidence_all_role action.payload "rebuttals" "plaintiff" offered hOfferedParse
  have hReportRole : ∀ item ∈ reports, item.role = "plaintiff" := by
    exact parseTechnicalReports_all_role action.payload "rebuttals" "plaintiff" reports hReportsParse
  have hOfferedCap1 :
      offeredCount s1.case.offered_evidence "plaintiff" + offered.length ≤ s1.policy.max_exhibits_per_side := by
    simpa [s1, c1, stateWithCase, addFiling_preserves_offered_evidence] using hOfferedCap
  have hReportCap1 :
      reportCount s1.case.technical_reports "plaintiff" + reports.length ≤ s1.policy.max_reports_per_side := by
    simpa [s1, c1, stateWithCase, addFiling_preserves_technical_reports] using hReportCap
  simpa [s1, c1, stateWithCase] using
    appendSupplementalMaterials_preserves_material_limits
      s1 offered reports "plaintiff" hBase1 hOfferedRole hReportRole hOfferedCap1 hReportCap1

theorem submitEvidence_result
    (s t : ArbitrationState)
    (actorRole : String)
    (payload : Lean.Json)
    (hSubmit : submitEvidence s actorRole payload = .ok t) :
    ∃ evidence,
      t = stateWithCase s (appendSubmittedEvidence s.case evidence) := by
  have handle (expectedRole : String)
      (hCore :
        (do
          requireRole actorRole expectedRole
          let parsedEvidence ← parseSubmittedEvidence payload s.case.phase expectedRole
          let evidence := { parsedEvidence with role := expectedRole }
          if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
            throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
          else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
            throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
          else
            let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
            requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
            pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t) :
      ∃ evidence,
        t = stateWithCase s (appendSubmittedEvidence s.case evidence) := by
    cases hRole : requireRole actorRole expectedRole with
    | error err =>
        rw [hRole] at hCore
        simp at hCore
        cases hCore
    | ok okv =>
        cases okv
        rw [hRole] at hCore
        simp at hCore
        cases hEvidence : parseSubmittedEvidence payload s.case.phase expectedRole with
        | error err =>
            rw [hEvidence] at hCore
            cases hCore
        | ok parsedEvidence =>
            rw [hEvidence] at hCore
            let evidence : SubmittedEvidence := { parsedEvidence with role := expectedRole }
            change
              (if ∃ x, x ∈ s.case.submitted_evidence ∧ x.evidence_id = evidence.evidence_id then
                throw (toString "duplicate submitted evidence_id: " ++ toString evidence.evidence_id)
              else if s.policy.max_submitted_evidence_bytes < evidence.size_bytes then
                throw (toString "submitted evidence exceeds byte limit of " ++
                  toString s.policy.max_submitted_evidence_bytes)
              else
                (fun _ => stateWithCase s (appendSubmittedEvidence s.case evidence)) <$>
                  requireCountWithinLimit "submitted_evidence for this side"
                    (submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1)
                    s.policy.max_submitted_evidence_per_side) = .ok t at hCore
            by_cases hDup : ∃ x, x ∈ s.case.submitted_evidence ∧ x.evidence_id = evidence.evidence_id
            · simp [hDup] at hCore
            · simp [hDup] at hCore
              by_cases hSize : s.policy.max_submitted_evidence_bytes < evidence.size_bytes
              · simp [hSize] at hCore
              · simp [hSize] at hCore
                let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
                cases hCount : requireCountWithinLimit "submitted_evidence for this side"
                    total s.policy.max_submitted_evidence_per_side with
                | error err =>
                    simp [total, hCount] at hCore
                    cases hCore
                | ok okv =>
                    cases okv
                    simp [total, hCount] at hCore
                    cases hCore
                    exact ⟨evidence, rfl⟩
  by_cases hArgs : s.case.phase = "arguments"
  · have hCore :
        (do
          requireRole actorRole (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          let parsedEvidence ← parseSubmittedEvidence payload s.case.phase (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          let evidence := { parsedEvidence with role := (if s.case.arguments.isEmpty then "plaintiff" else "defendant") }
          if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
            throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
          else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
            throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
          else
            let total := submittedEvidenceCountForRole s.case.submitted_evidence (if s.case.arguments.isEmpty then "plaintiff" else "defendant") + 1
            requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
            pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
        simpa [submitEvidence, hArgs] using hSubmit
    exact handle (if s.case.arguments.isEmpty then "plaintiff" else "defendant") hCore
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · cases hEmpty : s.case.rebuttals.isEmpty with
      | true =>
        have hCore :
            (do
              requireRole actorRole "plaintiff"
              let parsedEvidence ← parseSubmittedEvidence payload s.case.phase "plaintiff"
              let evidence := { parsedEvidence with role := "plaintiff" }
              if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
                throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
              else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
                throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
              else
                let total := submittedEvidenceCountForRole s.case.submitted_evidence "plaintiff" + 1
                requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
                pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
            simpa [submitEvidence, hArgs, hRebuttals, hEmpty] using hSubmit
        exact handle "plaintiff" hCore
      | false =>
        have hClosed :
            (do
              let expectedRole ← (throw "rebuttal evidence is closed" : Except String String)
              requireRole actorRole expectedRole
              let parsedEvidence ← parseSubmittedEvidence payload s.case.phase expectedRole
              let evidence := { parsedEvidence with role := expectedRole }
              if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
                throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
              else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
                throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
              else
                let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
                requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
                pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
          simpa [submitEvidence, hArgs, hRebuttals, hEmpty] using hSubmit
        change Except.error "rebuttal evidence is closed" = .ok t at hClosed
        cases hClosed
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · cases hEmpty : s.case.surrebuttals.isEmpty with
        | true =>
          have hCore :
              (do
                requireRole actorRole "defendant"
                let parsedEvidence ← parseSubmittedEvidence payload s.case.phase "defendant"
                let evidence := { parsedEvidence with role := "defendant" }
                if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
                  throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
                else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
                  throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
                else
                  let total := submittedEvidenceCountForRole s.case.submitted_evidence "defendant" + 1
                  requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
                  pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
              simpa [submitEvidence, hArgs, hRebuttals, hSurrebuttals, hEmpty] using hSubmit
          exact handle "defendant" hCore
        | false =>
          have hClosed :
              (do
                let expectedRole ← (throw "surrebuttal evidence is closed" : Except String String)
                requireRole actorRole expectedRole
                let parsedEvidence ← parseSubmittedEvidence payload s.case.phase expectedRole
                let evidence := { parsedEvidence with role := expectedRole }
                if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
                  throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
                else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
                  throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
                else
                  let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
                  requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
                  pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
            simpa [submitEvidence, hArgs, hRebuttals, hSurrebuttals, hEmpty] using hSubmit
          change Except.error "surrebuttal evidence is closed" = .ok t at hClosed
          cases hClosed
      · have hClosed :
            (do
              let expectedRole ← (throw "submitted evidence is allowed only in arguments, rebuttals, and surrebuttals" : Except String String)
              requireRole actorRole expectedRole
              let parsedEvidence ← parseSubmittedEvidence payload s.case.phase expectedRole
              let evidence := { parsedEvidence with role := expectedRole }
              if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
                throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
              else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
                throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
              else
                let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
                requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
                pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
          simpa [submitEvidence, hArgs, hRebuttals, hSurrebuttals] using hSubmit
        change Except.error "submitted evidence is allowed only in arguments, rebuttals, and surrebuttals" = .ok t at hClosed
        cases hClosed

theorem step_submit_evidence_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  exact stateWithCase_preserves_phaseShape s _
    (appendSubmittedEvidence_preserves_phaseShape s.case evidence hShape)

theorem step_submit_evidence_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_evidence")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases submitEvidence_result s t action.actor_role action.payload hSubmit with ⟨evidence, rfl⟩
  simpa [stateWithCase, materialLimitsRespected] using
    appendSubmittedEvidence_preserves_material_limits s evidence hLimits

/--
A successful surrebuttal step preserves the merits-sequence invariant.

Surrebuttal shares the supplemental-material path with arguments and rebuttal.
The structural filing change remains the single defendant surrebuttal filing.
-/
theorem step_submit_surrebuttal_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  have hSubmit :
      recordMeritsSubmission
        s
        "surrebuttals"
        action.actor_role
        "defendant"
        "surrebuttal"
        s.policy.max_surrebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_result
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  have hPhase : s.case.phase = "surrebuttals" := by
    by_cases hSur : s.case.phase = "surrebuttals"
    · exact hSur
    · have hClosed : s.case.phase != "surrebuttals" := by simpa using hSur
      simp [recordMeritsSubmission, hClosed] at hSubmit
      cases hSubmit
  exact stateWithCase_preserves_phaseShape s _
    (appendSupplementalMaterials_preserves_phaseShape _ _ _
      (addSurrebuttal_preserves_phaseShape s.case (trimString rawText) hShape hPhase))

/--
A successful surrebuttal step preserves the aggregate material limits.

Surrebuttal uses the same material-limit path as argument and rebuttal
submissions.
-/
theorem step_submit_surrebuttal_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  have hSubmit :
      recordMeritsSubmission
        s
        "surrebuttals"
        action.actor_role
        "defendant"
        "surrebuttal"
        s.policy.max_surrebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  rcases recordMeritsSubmission_with_materials_details
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, hOfferedParse, hReportsParse, hOfferedCap, hReportCap, rfl⟩
  let c1 := addFiling s.case "surrebuttals" "defendant" (trimString rawText)
  let s1 := stateWithCase s c1
  have hBase1 : materialLimitsRespected s1 := by
    exact stateWithCase_preserves_material_limits s c1 hLimits
      (addFiling_preserves_offered_evidence s.case "surrebuttals" "defendant" (trimString rawText))
      (addFiling_preserves_technical_reports s.case "surrebuttals" "defendant" (trimString rawText))
  have hOfferedRole : ∀ item ∈ offered, item.role = "defendant" := by
    exact parseOfferedEvidence_all_role action.payload "surrebuttals" "defendant" offered hOfferedParse
  have hReportRole : ∀ item ∈ reports, item.role = "defendant" := by
    exact parseTechnicalReports_all_role action.payload "surrebuttals" "defendant" reports hReportsParse
  have hOfferedCap1 :
      offeredCount s1.case.offered_evidence "defendant" + offered.length ≤ s1.policy.max_exhibits_per_side := by
    simpa [s1, c1, stateWithCase, addFiling_preserves_offered_evidence] using hOfferedCap
  have hReportCap1 :
      reportCount s1.case.technical_reports "defendant" + reports.length ≤ s1.policy.max_reports_per_side := by
    simpa [s1, c1, stateWithCase, addFiling_preserves_technical_reports] using hReportCap
  simpa [s1, c1, stateWithCase] using
    appendSupplementalMaterials_preserves_material_limits
      s1 offered reports "defendant" hBase1 hOfferedRole hReportRole hOfferedCap1 hReportCap1

/--
Passing an optional phase preserves the global filing shape.

The engine allows passing only in rebuttals and surrebuttals.  In either
branch, the corresponding filing list is still empty when the pass succeeds.
The pass therefore changes only the phase marker.
-/
theorem step_pass_phase_opportunity_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · have hPass :
        (do
          requireRole action.actor_role "plaintiff"
          if !s.case.rebuttals.isEmpty then
            throw "rebuttal already submitted"
          pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
      simpa [stepCore, hType, hRebuttals] using hStep
    have hEmpty : s.case.rebuttals = [] := by
      simp [phaseShape, hRebuttals] at hShape
      exact hShape.2.2.1
    cases hRole : requireRole action.actor_role "plaintiff" with
    | error err =>
        rw [hRole] at hPass
        simp at hPass
        cases hPass
    | ok _ =>
        rw [hRole] at hPass
        simp [hEmpty] at hPass
        cases hPass
        simpa [stateWithCase, hEmpty] using
          passRebuttal_preserves_phaseShape s.case hShape hRebuttals
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · have hPass :
          (do
            requireRole action.actor_role "defendant"
            if !s.case.surrebuttals.isEmpty then
              throw "surrebuttal already submitted"
            pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
        simpa [stepCore, hType, hRebuttals, hSurrebuttals] using hStep
      have hEmpty : s.case.surrebuttals = [] := by
        simp [phaseShape, hSurrebuttals] at hShape
        exact hShape.2.2.2.1
      cases hRole : requireRole action.actor_role "defendant" with
      | error err =>
          rw [hRole] at hPass
          simp at hPass
          cases hPass
      | ok _ =>
          rw [hRole] at hPass
          simp [hEmpty] at hPass
          cases hPass
          simpa [stateWithCase, hEmpty] using
            passSurrebuttal_preserves_phaseShape s.case hShape hSurrebuttals
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

/--
Passing an optional phase preserves the aggregate material limits.

The pass action changes only the phase marker.  It does not change the
admitted-material lists.
-/
theorem step_pass_phase_opportunity_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
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
    | ok _ =>
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            exact stateWithCase_preserves_material_limits s _ hLimits rfl rfl
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
      | ok _ =>
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              exact stateWithCase_preserves_material_limits s _ hLimits rfl rfl
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

/--
Changing council votes does not affect the merits-sequence predicate.

`phaseShape` ignores the deliberation metadata.  It depends only on the phase
marker and the merits filing lists.
-/
theorem setCouncilVotes_preserves_phaseShape
    (c : ArbitrationCase)
    (votes : List CouncilVote)
    (hShape : phaseShape c) :
    phaseShape { c with council_votes := votes } := by
  cases hPhase : c.phase <;> simpa [phaseShape, hPhase] using hShape

/--
Changing council-member status does not affect the merits-sequence predicate.
-/
theorem setCouncilMembers_preserves_phaseShape
    (c : ArbitrationCase)
    (members : List CouncilMember)
    (hShape : phaseShape c) :
    phaseShape { c with council_members := members } := by
  cases hPhase : c.phase <;> simpa [phaseShape, hPhase] using hShape

/--
`continueDeliberation` preserves `phaseShape` for any deliberating case whose
merits lists already have the right shape.

The earlier theorem in `GlobalInvariants` fixes the case argument to `s.case`.
The council actions first construct an intermediate case and then call
`continueDeliberation` on that case.  This generalized form states the same
fact at the level those actions need.
-/
theorem continueDeliberation_preserves_phaseShape_for
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hShape : phaseShape c)
    (hPhase : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t) :
    phaseShape t.case := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        exact stateWithCase_preserves_phaseShape s _
          (closeDeliberation_preserves_phaseShape c resolution hShape hPhase)
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          exact stateWithCase_preserves_phaseShape s _
            (closeDeliberation_preserves_phaseShape c "no_majority" hShape hPhase)
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_phaseShape s _
              (closeDeliberation_preserves_phaseShape c "no_majority" hShape hPhase)
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_phaseShape s _
              (by simpa [phaseShape, hPhase] using hShape)
  · simp [hRoundComplete] at hCont
    cases hCont
    exact stateWithCase_preserves_phaseShape s _
      (by simpa [phaseShape, hPhase] using hShape)

/--
`continueDeliberation` preserves the aggregate material limits for any case
that still carries the same admitted materials as the current state.

Deliberation does not rewrite exhibits or reports.  The theorem therefore needs
only the equalities that connect the intermediate deliberation case back to the
state's admitted-material lists.
-/
theorem continueDeliberation_preserves_material_limits_for
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hBase : materialLimitsRespected s)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports)
    (hCont : continueDeliberation s c = .ok t) :
    materialLimitsRespected t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hCont
        cases hCont
        exact stateWithCase_preserves_material_limits s _ hBase hOffered hReports
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hCont
          cases hCont
          exact stateWithCase_preserves_material_limits s _ hBase hOffered hReports
        · by_cases hLastRound : c.deliberation_round >= s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_material_limits s _ hBase hOffered hReports
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hCont
            cases hCont
            exact stateWithCase_preserves_material_limits s _ hBase hOffered hReports
  · simp [hRoundComplete] at hCont
    cases hCont
    exact stateWithCase_preserves_material_limits s _ hBase hOffered hReports

/--
A successful council-vote step reaches `continueDeliberation` from the case
with one appended vote.

The public `step` wrapper contributes only its own actor-role check and the two
required payload fields.  Once those checks succeed, the remaining work is the
engine's `recordCouncilVote` path.  The theorem exposes that intermediate case
directly so that the later preservation theorems can focus on what changes and
what does not.
-/
theorem step_submit_council_vote_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ memberId vote rationale,
      s.case.phase = "deliberation" ∧
      continueDeliberation s
        { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := trimString vote
            rationale := trimString rationale
          } } = .ok t := by
  have hStep' :
      (do
        requireRole action.actor_role "council"
        let memberId := trimString (← getString action.payload "member_id")
        let vote := trimString (← getString action.payload "vote")
        let rationale := getOptionalString action.payload "rationale"
        recordCouncilVote s memberId vote rationale) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "council" with
  | error err =>
      rw [hRole] at hStep'
      simp at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      simp at hStep'
      cases hMember : getString action.payload "member_id" with
      | error err =>
          rw [hMember] at hStep'
          cases hStep'
      | ok rawMemberId =>
          rw [hMember] at hStep'
          cases hVoteText : getString action.payload "vote" with
          | error err =>
              rw [hVoteText] at hStep'
              cases hStep'
          | ok rawVote =>
              rw [hVoteText] at hStep'
              let memberId := trimString rawMemberId
              let vote := trimString rawVote
              let rationale := getOptionalString action.payload "rationale"
              have hRecord :
                  recordCouncilVote s memberId vote rationale = .ok t := by
                simpa [memberId, vote, rationale, SeqRight.seqRight, Bind.bind, Except.bind] using hStep'
              have hPhase : s.case.phase = "deliberation" := by
                by_cases hDelib : s.case.phase = "deliberation"
                · exact hDelib
                · have hClosed : s.case.phase != "deliberation" := by simpa using hDelib
                  simp [recordCouncilVote, hClosed] at hRecord
                  cases hRecord
              unfold recordCouncilVote at hRecord
              rw [if_neg (by simp [hPhase])] at hRecord
              cases hKnown :
                  s.case.council_members.any (fun member => member.member_id = memberId) with
              | false =>
                  rw [if_pos (by simp [hKnown])] at hRecord
                  cases hRecord
              | true =>
                  rw [if_neg (by simp [hKnown])] at hRecord
                  cases hSeated :
                      s.case.council_members.any
                        (fun member => member.member_id = memberId && memberIsSeated member) with
                  | false =>
                      rw [if_pos (by simp [hSeated])] at hRecord
                      cases hRecord
                  | true =>
                      rw [if_neg (by simp [hSeated])] at hRecord
                      cases hVoteValid :
                          (trimString vote != "demonstrated" &&
                            trimString vote != "not_demonstrated") with
                      | true =>
                          rw [if_pos (by simp [hVoteValid])] at hRecord
                          cases hRecord
                      | false =>
                          rw [if_neg (by simp [hVoteValid])] at hRecord
                          let votes := currentRoundVotes s.case
                          cases hAlready : votes.any (fun existing => existing.member_id = memberId) with
                          | true =>
                              rw [if_pos (by simp [votes, hAlready])] at hRecord
                              cases hRecord
                          | false =>
                              rw [if_neg (by simp [votes, hAlready])] at hRecord
                              exact ⟨memberId, vote, rationale, hPhase, hRecord⟩

/--
A successful council-vote step also identifies a seated member who had not yet
voted in the current round.

The earlier result theorem exposed the intermediate case sent to
`continueDeliberation`.  The integrity proofs need two more facts from the same
successful path:

1. the voting member was seated in the source case; and
2. that member did not already appear in the current round's vote list.
-/
private theorem step_submit_council_vote_details_core
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ memberId vote rationale,
      s.case.phase = "deliberation" ∧
      memberId ∈ councilMemberIds (seatedCouncilMembers s.case) ∧
      memberId ∉ (currentRoundVotes s.case).map (·.member_id) ∧
      (trimString vote = "demonstrated" ∨ trimString vote = "not_demonstrated") ∧
      continueDeliberation s
        { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := trimString vote
            rationale := trimString rationale
          } } = .ok t := by
  have hStep' :
      (do
        requireRole action.actor_role "council"
        let memberId := trimString (← getString action.payload "member_id")
        let vote := trimString (← getString action.payload "vote")
        let rationale := getOptionalString action.payload "rationale"
        recordCouncilVote s memberId vote rationale) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "council" with
  | error err =>
      rw [hRole] at hStep'
      simp at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      simp at hStep'
      cases hMember : getString action.payload "member_id" with
      | error err =>
          rw [hMember] at hStep'
          cases hStep'
      | ok rawMemberId =>
          rw [hMember] at hStep'
          cases hVoteText : getString action.payload "vote" with
          | error err =>
              rw [hVoteText] at hStep'
              cases hStep'
          | ok rawVote =>
              rw [hVoteText] at hStep'
              let memberId := trimString rawMemberId
              let vote := trimString rawVote
              let rationale := getOptionalString action.payload "rationale"
              have hRecord :
                  recordCouncilVote s memberId vote rationale = .ok t := by
                simpa [memberId, vote, rationale, SeqRight.seqRight, Bind.bind, Except.bind] using hStep'
              have hPhase : s.case.phase = "deliberation" := by
                by_cases hDelib : s.case.phase = "deliberation"
                · exact hDelib
                · have hClosed : s.case.phase != "deliberation" := by simpa using hDelib
                  simp [recordCouncilVote, hClosed] at hRecord
                  cases hRecord
              unfold recordCouncilVote at hRecord
              rw [if_neg (by simp [hPhase])] at hRecord
              cases hKnown :
                  s.case.council_members.any (fun member => member.member_id = memberId) with
              | false =>
                  rw [if_pos (by simp [hKnown])] at hRecord
                  cases hRecord
              | true =>
                  rw [if_neg (by simp [hKnown])] at hRecord
                  cases hSeated :
                      s.case.council_members.any
                        (fun member => member.member_id = memberId && memberIsSeated member) with
                  | false =>
                      rw [if_pos (by simp [hSeated])] at hRecord
                      cases hRecord
                  | true =>
                      rw [if_neg (by simp [hSeated])] at hRecord
                      cases hVoteValid :
                          (trimString vote != "demonstrated" &&
                            trimString vote != "not_demonstrated") with
                      | true =>
                          rw [if_pos (by simp [hVoteValid])] at hRecord
                          cases hRecord
                      | false =>
                          rw [if_neg (by simp [hVoteValid])] at hRecord
                          let votes := currentRoundVotes s.case
                          cases hAlready : votes.any (fun existing => existing.member_id = memberId) with
                          | true =>
                              rw [if_pos (by simp [votes, hAlready])] at hRecord
                              cases hRecord
                          | false =>
                              rw [if_neg (by simp [votes, hAlready])] at hRecord
                              have hMemberSeated :
                                  memberId ∈ councilMemberIds (seatedCouncilMembers s.case) := by
                                rcases List.any_eq_true.mp hSeated with ⟨member, hMemberMem, hSeatPair⟩
                                have hSeatParts :
                                    decide (member.member_id = memberId) = true ∧
                                      memberIsSeated member = true := by
                                  simpa using hSeatPair
                                have hMemberId : member.member_id = memberId := by
                                  exact of_decide_eq_true hSeatParts.1
                                have hSeat : memberIsSeated member = true := by
                                  exact hSeatParts.2
                                have hSeatMem : member ∈ seatedCouncilMembers s.case := by
                                  unfold seatedCouncilMembers
                                  simp [hMemberMem, hSeat]
                                have hSeatId :
                                    ∃ candidate, candidate ∈ seatedCouncilMembers s.case ∧
                                      candidate.member_id = memberId := by
                                  exact ⟨member, hSeatMem, hMemberId⟩
                                simpa [councilMemberIds] using hSeatId
                              have hNotAlready :
                                  memberId ∉ (currentRoundVotes s.case).map (·.member_id) := by
                                intro hMemId
                                have hVoteMem :
                                    ∃ vote : CouncilVote,
                                      vote ∈ currentRoundVotes s.case ∧ vote.member_id = memberId := by
                                  simpa using hMemId
                                rcases hVoteMem with ⟨existing, hExistingMem, hExistingId⟩
                                have hAnyTrue :
                                    votes.any (fun vote => vote.member_id = memberId) = true := by
                                  exact List.any_eq_true.mpr ⟨existing, by simpa [votes] using hExistingMem,
                                    by simp [hExistingId]⟩
                                simp [hAlready] at hAnyTrue
                              have hVoteCases :
                                  trimString vote = "demonstrated" ∨
                                    trimString vote = "not_demonstrated" := by
                                by_cases hDem : trimString vote = "demonstrated"
                                · exact Or.inl hDem
                                · by_cases hNot : trimString vote = "not_demonstrated"
                                  · exact Or.inr hNot
                                  · have hInvalid :
                                        (trimString vote != "demonstrated" &&
                                          trimString vote != "not_demonstrated") = true := by
                                      simp [hDem, hNot]
                                    simp [hVoteValid] at hInvalid
                              exact ⟨memberId, vote, rationale, hPhase, hMemberSeated,
                                hNotAlready, hVoteCases, hRecord⟩

theorem step_submit_council_vote_details
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ memberId vote rationale,
      s.case.phase = "deliberation" ∧
      memberId ∈ councilMemberIds (seatedCouncilMembers s.case) ∧
      memberId ∉ (currentRoundVotes s.case).map (·.member_id) ∧
      continueDeliberation s
        { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := trimString vote
            rationale := trimString rationale
          } } = .ok t := by
  rcases step_submit_council_vote_details_core s t action hType hStep with
    ⟨memberId, vote, rationale, hPhase, hSeated, hFresh, _hVote, hCont⟩
  exact ⟨memberId, vote, rationale, hPhase, hSeated, hFresh, hCont⟩

theorem step_submit_council_vote_details_with_valid_vote
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ memberId vote rationale,
      s.case.phase = "deliberation" ∧
      memberId ∈ councilMemberIds (seatedCouncilMembers s.case) ∧
      memberId ∉ (currentRoundVotes s.case).map (·.member_id) ∧
      (trimString vote = "demonstrated" ∨ trimString vote = "not_demonstrated") ∧
      continueDeliberation s
        { s.case with council_votes := s.case.council_votes.concat {
            member_id := memberId
            round := s.case.deliberation_round
            vote := trimString vote
            rationale := trimString rationale
          } } = .ok t := by
  exact step_submit_council_vote_details_core s t action hType hStep

/--
A successful council-removal step reaches `continueDeliberation` from the case
with one member's seating status changed.

As with the vote wrapper, the public `step` function adds only a small amount
of parsing and role checking.  The substantive transition is still
`removeCouncilMember`.
-/
theorem step_remove_council_member_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ memberId status,
      s.case.phase = "deliberation" ∧
      continueDeliberation s
        { s.case with
            council_members := s.case.council_members.map (fun (member : CouncilMember) =>
              if member.member_id = memberId then
                { member with status := trimString status }
              else
                member) } = .ok t := by
  have hStep' :
      (do
        requireRole action.actor_role "system"
        let memberId := trimString (← getString action.payload "member_id")
        let status := (← getString action.payload "status")
        removeCouncilMember s memberId status) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "system" with
  | error err =>
      rw [hRole] at hStep'
      simp at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      simp at hStep'
      cases hMember : getString action.payload "member_id" with
      | error err =>
          rw [hMember] at hStep'
          cases hStep'
      | ok rawMemberId =>
          rw [hMember] at hStep'
          cases hStatusText : getString action.payload "status" with
          | error err =>
              rw [hStatusText] at hStep'
              cases hStep'
          | ok rawStatus =>
              rw [hStatusText] at hStep'
              let memberId := trimString rawMemberId
              let status := rawStatus
              have hRemove :
                  removeCouncilMember s memberId status = .ok t := by
                simpa [memberId, status, SeqRight.seqRight, Bind.bind, Except.bind] using hStep'
              have hPhase : s.case.phase = "deliberation" := by
                by_cases hDelib : s.case.phase = "deliberation"
                · exact hDelib
                · have hClosed : s.case.phase != "deliberation" := by simpa using hDelib
                  simp [removeCouncilMember, hClosed] at hRemove
                  cases hRemove
              unfold removeCouncilMember at hRemove
              rw [if_neg (by simp [hPhase])] at hRemove
              cases hKnown :
                  s.case.council_members.any (fun member => member.member_id = memberId) with
              | false =>
                  rw [if_pos (by simp [hKnown])] at hRemove
                  cases hRemove
              | true =>
                  rw [if_neg (by simp [hKnown])] at hRemove
                  cases hSeated :
                      s.case.council_members.any
                        (fun member => member.member_id = memberId && memberIsSeated member) with
                  | false =>
                      rw [if_pos (by simp [hSeated])] at hRemove
                      cases hRemove
                  | true =>
                      rw [if_neg (by simp [hSeated])] at hRemove
                      let votes := currentRoundVotes s.case
                      cases hAlready : votes.any (fun existing => existing.member_id = memberId) with
                      | true =>
                          by_cases hEmpty : trimString status = ""
                          · simp [hEmpty, votes, hAlready] at hRemove
                            cases hRemove
                          · by_cases hSeatedStatus : trimString status = "seated"
                            · simp [hSeatedStatus, votes, hAlready] at hRemove
                              cases hRemove
                            · simp [hEmpty, hSeatedStatus, votes, hAlready] at hRemove
                              cases hRemove
                      | false =>
                          by_cases hEmpty : trimString status = ""
                          · simp [hEmpty, votes, hAlready] at hRemove
                            cases hRemove
                          · by_cases hSeatedStatus : trimString status = "seated"
                            · simp [hSeatedStatus, votes, hAlready] at hRemove
                              cases hRemove
                            · simp [hEmpty, hSeatedStatus, votes, hAlready] at hRemove
                              exact ⟨memberId, status, hPhase, hRemove⟩

/--
A successful council-removal step identifies a seated member who had not yet
voted in the current round.

This is the removal analogue of `step_submit_council_vote_details`.  The
integrity proofs need exactly the same two facts: the targeted member was
seated, and that member was not already represented in the current round's
votes.
-/
theorem step_remove_council_member_details
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ memberId status,
      s.case.phase = "deliberation" ∧
      memberId ∈ councilMemberIds (seatedCouncilMembers s.case) ∧
      memberId ∉ (currentRoundVotes s.case).map (·.member_id) ∧
      trimString status ≠ "seated" ∧
      continueDeliberation s
        { s.case with
            council_members := s.case.council_members.map (fun (member : CouncilMember) =>
              if member.member_id = memberId then
                { member with status := trimString status }
              else
                member) } = .ok t := by
  have hStep' :
      (do
        requireRole action.actor_role "system"
        let memberId := trimString (← getString action.payload "member_id")
        let status := (← getString action.payload "status")
        removeCouncilMember s memberId status) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "system" with
  | error err =>
      rw [hRole] at hStep'
      simp at hStep'
      cases hStep'
  | ok okv =>
      cases okv
      rw [hRole] at hStep'
      simp at hStep'
      cases hMember : getString action.payload "member_id" with
      | error err =>
          rw [hMember] at hStep'
          cases hStep'
      | ok rawMemberId =>
          rw [hMember] at hStep'
          cases hStatusText : getString action.payload "status" with
          | error err =>
              rw [hStatusText] at hStep'
              cases hStep'
          | ok rawStatus =>
              rw [hStatusText] at hStep'
              let memberId := trimString rawMemberId
              let status := rawStatus
              have hRemove :
                  removeCouncilMember s memberId status = .ok t := by
                simpa [memberId, status, SeqRight.seqRight, Bind.bind, Except.bind] using hStep'
              have hPhase : s.case.phase = "deliberation" := by
                by_cases hDelib : s.case.phase = "deliberation"
                · exact hDelib
                · have hClosed : s.case.phase != "deliberation" := by simpa using hDelib
                  simp [removeCouncilMember, hClosed] at hRemove
                  cases hRemove
              unfold removeCouncilMember at hRemove
              rw [if_neg (by simp [hPhase])] at hRemove
              cases hKnown :
                  s.case.council_members.any (fun member => member.member_id = memberId) with
              | false =>
                  rw [if_pos (by simp [hKnown])] at hRemove
                  cases hRemove
              | true =>
                  rw [if_neg (by simp [hKnown])] at hRemove
                  cases hSeated :
                      s.case.council_members.any
                        (fun member => member.member_id = memberId && memberIsSeated member) with
                  | false =>
                      rw [if_pos (by simp [hSeated])] at hRemove
                      cases hRemove
                  | true =>
                      rw [if_neg (by simp [hSeated])] at hRemove
                      let votes := currentRoundVotes s.case
                      cases hAlready : votes.any (fun existing => existing.member_id = memberId) with
                      | true =>
                          by_cases hEmpty : trimString status = ""
                          · simp [hEmpty, votes, hAlready] at hRemove
                            cases hRemove
                          · by_cases hSeatedStatus : trimString status = "seated"
                            · simp [hSeatedStatus, votes, hAlready] at hRemove
                              cases hRemove
                            · simp [hEmpty, hSeatedStatus, votes, hAlready] at hRemove
                              cases hRemove
                      | false =>
                          by_cases hEmpty : trimString status = ""
                          · simp [hEmpty, votes, hAlready] at hRemove
                            cases hRemove
                          · by_cases hSeatedStatus : trimString status = "seated"
                            · simp [hSeatedStatus, votes, hAlready] at hRemove
                              cases hRemove
                            · simp [hEmpty, hSeatedStatus, votes, hAlready] at hRemove
                              have hMemberSeated :
                                  memberId ∈ councilMemberIds (seatedCouncilMembers s.case) := by
                                rcases List.any_eq_true.mp hSeated with ⟨member, hMemberMem, hSeatPair⟩
                                have hSeatParts :
                                    decide (member.member_id = memberId) = true ∧
                                      memberIsSeated member = true := by
                                  simpa using hSeatPair
                                have hMemberId : member.member_id = memberId := by
                                  exact of_decide_eq_true hSeatParts.1
                                have hSeat : memberIsSeated member = true := by
                                  exact hSeatParts.2
                                have hSeatMem : member ∈ seatedCouncilMembers s.case := by
                                  unfold seatedCouncilMembers
                                  simp [hMemberMem, hSeat]
                                have hSeatId :
                                    ∃ candidate, candidate ∈ seatedCouncilMembers s.case ∧
                                      candidate.member_id = memberId := by
                                  exact ⟨member, hSeatMem, hMemberId⟩
                                simpa [councilMemberIds] using hSeatId
                              have hNotAlready :
                                  memberId ∉ (currentRoundVotes s.case).map (·.member_id) := by
                                intro hMemId
                                have hVoteMem :
                                    ∃ vote : CouncilVote,
                                      vote ∈ currentRoundVotes s.case ∧ vote.member_id = memberId := by
                                  simpa using hMemId
                                rcases hVoteMem with ⟨existing, hExistingMem, hExistingId⟩
                                have hAnyTrue :
                                    votes.any (fun vote => vote.member_id = memberId) = true := by
                                  exact List.any_eq_true.mpr ⟨existing, by simpa [votes] using hExistingMem,
                                    by simp [hExistingId]⟩
                                simp [hAlready] at hAnyTrue
                              exact ⟨memberId, status, hPhase, hMemberSeated, hNotAlready,
                                hSeatedStatus, hRemove⟩

theorem failCouncilMemberOpportunity_result
    (s t : ArbitrationState)
    (memberId reason opportunityId message : String)
    (hFail :
      failCouncilMemberOpportunity s memberId reason opportunityId message = .ok t) :
    ∃ c1,
      c1 =
        { s.case with
          council_members := s.case.council_members.map (fun (member : CouncilMember) =>
            if member.member_id = memberId then
              { member with
                status := "failed"
                failure_reason := reason
                failure_opportunity_id := opportunityId
                failure_message := message
              }
            else
          member) } ∧
      s.case.phase = "deliberation" ∧
      memberId ∈ councilMemberIds (seatedCouncilMembers s.case) ∧
      memberId ∉ (currentRoundVotes s.case).map (·.member_id) ∧
      continueDeliberation s c1 = .ok t := by
  let c1 : ArbitrationCase :=
    { s.case with
      council_members := s.case.council_members.map (fun (member : CouncilMember) =>
        if member.member_id = memberId then
          { member with
            status := "failed"
            failure_reason := reason
            failure_opportunity_id := opportunityId
            failure_message := message
          }
        else
          member) }
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold failCouncilMemberOpportunity at hFail
    rw [if_neg (by simp [hPhase])] at hFail
    by_cases hMemberEmpty : memberId = ""
    · rw [if_pos hMemberEmpty] at hFail
      change (throw "council opportunity failure requires member_id" :
        Except String ArbitrationState) = .ok t at hFail
      cases hFail
    · rw [if_neg hMemberEmpty] at hFail
      by_cases hReasonEmpty : reason = ""
      · rw [if_pos hReasonEmpty] at hFail
        change (throw "opportunity failure requires reason" :
          Except String ArbitrationState) = .ok t at hFail
        cases hFail
      · rw [if_neg hReasonEmpty] at hFail
        cases hKnown : s.case.council_members.any (fun member => member.member_id = memberId) with
        | false =>
            rw [if_pos (by simp [hKnown])] at hFail
            change (throw (toString "unknown council member: " ++ toString memberId) :
              Except String ArbitrationState) = .ok t at hFail
            cases hFail
        | true =>
            rw [if_neg (by simp [hKnown])] at hFail
            cases hSeated :
                s.case.council_members.any
                  (fun member => member.member_id = memberId && memberIsSeated member) with
            | false =>
                rw [if_pos (by simp [hSeated])] at hFail
                change (throw (toString "council member is not seated: " ++
                  toString memberId) : Except String ArbitrationState) = .ok t at hFail
                cases hFail
            | true =>
                rw [if_neg (by simp [hSeated])] at hFail
                let votes := currentRoundVotes s.case
                cases hAlready : votes.any (fun vote => vote.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [votes, hAlready])] at hFail
                    change (throw (toString "cannot fail council member after current-round vote: " ++
                      toString memberId) : Except String ArbitrationState) = .ok t at hFail
                    cases hFail
                | false =>
                    rw [if_neg (by simp [votes, hAlready])] at hFail
                    have hMemberSeated :
                        memberId ∈ councilMemberIds (seatedCouncilMembers s.case) := by
                      rcases List.any_eq_true.mp hSeated with ⟨member, hMemberMem, hSeatPair⟩
                      have hSeatParts :
                          decide (member.member_id = memberId) = true ∧
                            memberIsSeated member = true := by
                        simpa using hSeatPair
                      have hMemberId : member.member_id = memberId := by
                        exact of_decide_eq_true hSeatParts.1
                      have hSeat : memberIsSeated member = true := by
                        exact hSeatParts.2
                      have hSeatMem : member ∈ seatedCouncilMembers s.case := by
                        unfold seatedCouncilMembers
                        simp [hMemberMem, hSeat]
                      have hSeatId :
                          ∃ candidate, candidate ∈ seatedCouncilMembers s.case ∧
                            candidate.member_id = memberId := by
                        exact ⟨member, hSeatMem, hMemberId⟩
                      simpa [councilMemberIds] using hSeatId
                    have hNotAlready :
                        memberId ∉ (currentRoundVotes s.case).map (·.member_id) := by
                      intro hMemId
                      have hVoteMem :
                          ∃ vote : CouncilVote,
                            vote ∈ currentRoundVotes s.case ∧ vote.member_id = memberId := by
                        simpa using hMemId
                      rcases hVoteMem with ⟨existing, hExistingMem, hExistingId⟩
                      have hAnyTrue :
                          votes.any (fun vote => vote.member_id = memberId) = true := by
                        exact List.any_eq_true.mpr ⟨existing, by simpa [votes] using hExistingMem,
                          by simp [hExistingId]⟩
                      simp [hAlready] at hAnyTrue
                    refine ⟨c1, rfl, hPhase, hMemberSeated, hNotAlready, ?_⟩
                    simpa [c1] using hFail
  · have hNotDelib : (s.case.phase != "deliberation") = true := by
      simpa using hPhase
    unfold failCouncilMemberOpportunity at hFail
    rw [if_pos hNotDelib] at hFail
    change (throw "council opportunity failure is allowed only in deliberation" :
      Except String ArbitrationState) = .ok t at hFail
    cases hFail

theorem failCase_preserves_phaseShape
    (c : ArbitrationCase)
    (failure : OpportunityFailure)
    (hShape : phaseShape c) :
    phaseShape
      { c with
        status := "failed"
        failure := some failure } := by
  cases hPhase : c.phase <;> simpa [phaseShape, hPhase] using hShape

theorem failCase_preserves_material_limits
    (s : ArbitrationState)
    (failure : OpportunityFailure)
    (hLimits : materialLimitsRespected s) :
    materialLimitsRespected
      (stateWithCase s
        { s.case with
          status := "failed"
          failure := some failure }) := by
  exact stateWithCase_preserves_material_limits s _ hLimits rfl rfl

set_option linter.unusedSimpArgs false in
theorem nextOpportunity_phase_eq
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.phase = s.case.phase := by
  unfold nextOpportunity at hNext
  by_cases hClosed : s.case.status = "closed"
  · simp [hClosed] at hNext
  · by_cases hFailed : s.case.status = "failed"
    · cases hFailure : s.case.failure with
      | none =>
          simp [hFailed, hFailure] at hNext
      | some failure =>
          simp [hFailed, hFailure] at hNext
    · simp [hClosed, hFailed] at hNext
      by_cases hOpenings : s.case.phase = "openings"
      · cases hRole : plaintiffThenDefendant s.case.openings "plaintiff" "defendant" with
        | none =>
            simp [nextOpportunityForPhase, hOpenings, hRole] at hNext
        | some role =>
            simp [nextOpportunityForPhase, hOpenings, hRole] at hNext
            cases hNext
            simp [hOpenings]
      · by_cases hArguments : s.case.phase = "arguments"
        · cases hRole : plaintiffThenDefendant s.case.arguments "plaintiff" "defendant" with
          | none =>
              simp [nextOpportunityForPhase, hOpenings, hArguments, hRole] at hNext
          | some role =>
              simp [nextOpportunityForPhase, hOpenings, hArguments, hRole] at hNext
              cases hNext
              simp [hArguments]
        · by_cases hRebuttals : s.case.phase = "rebuttals"
          · cases hEmpty : s.case.rebuttals.isEmpty with
            | false =>
                simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals, hEmpty] at hNext
            | true =>
                simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals, hEmpty] at hNext
                cases hNext
                simp [hRebuttals]
          · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
            · cases hEmpty : s.case.surrebuttals.isEmpty with
              | false =>
                  simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                    hSurrebuttals, hEmpty] at hNext
              | true =>
                  simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                    hSurrebuttals, hEmpty] at hNext
                  cases hNext
                  simp [hSurrebuttals]
            · by_cases hClosings : s.case.phase = "closings"
              · cases hRole : plaintiffThenDefendant s.case.closings "plaintiff" "defendant" with
                | none =>
                    simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                      hSurrebuttals, hClosings, hRole] at hNext
                | some role =>
                    simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                      hSurrebuttals, hClosings, hRole] at hNext
                    cases hNext
                    simp [hClosings]
              · by_cases hDeliberation : s.case.phase = "deliberation"
                · cases hMember : nextCouncilMember? s.case with
                  | none =>
                      simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                        hSurrebuttals, hClosings, hDeliberation, hMember] at hNext
                  | some member =>
                      simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                        hSurrebuttals, hClosings, hDeliberation, hMember] at hNext
                      cases hNext
                      simp [hDeliberation]
                · by_cases hClosedPhase : s.case.phase = "closed"
                  · simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                      hSurrebuttals, hClosings, hDeliberation, hClosedPhase] at hNext
                  · simp [nextOpportunityForPhase, hOpenings, hArguments, hRebuttals,
                      hSurrebuttals, hClosings, hDeliberation, hClosedPhase] at hNext

theorem failOpportunity_result
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hFail : failOpportunity s payload = .ok t) :
    (∃ memberId reason opportunityId message c1,
      c1 =
        { s.case with
          council_members := List.map (fun (member : CouncilMember) =>
            if member.member_id = memberId then
              { member with
                status := "failed"
                failure_reason := reason
                failure_opportunity_id := opportunityId
                failure_message := message }
            else member) s.case.council_members } ∧
      s.case.phase = "deliberation" ∧
      memberId ∈ councilMemberIds (seatedCouncilMembers s.case) ∧
      memberId ∉ (currentRoundVotes s.case).map (·.member_id) ∧
      continueDeliberation s c1 = .ok t) ∨
    (∃ failure : OpportunityFailure,
      t = stateWithCase s
        { s.case with
          status := "failed"
          failure := some failure } ∧
      t.case.phase ≠ "closed" ∧
      s.case.phase ≠ "deliberation" ∧
      failure.failure_type = "opportunity_failed" ∧
      (failure.role = "plaintiff" ∨ failure.role = "defendant") ∧
      failure.phase = s.case.phase) := by
  unfold failOpportunity at hFail
  cases hOpportunityIdText : getString payload "opportunity_id" with
  | error err =>
      rw [hOpportunityIdText] at hFail
      cases hFail
  | ok rawOpportunityId =>
      rw [hOpportunityIdText] at hFail
      cases hRoleText : getString payload "role" with
      | error err =>
          rw [hRoleText] at hFail
          cases hFail
      | ok rawRole =>
          rw [hRoleText] at hFail
          cases hPhaseText : getString payload "phase" with
          | error err =>
              rw [hPhaseText] at hFail
              cases hFail
          | ok rawPhase =>
              rw [hPhaseText] at hFail
              cases hReasonText : getString payload "reason" with
              | error err =>
                  rw [hReasonText] at hFail
                  cases hFail
              | ok rawReason =>
                  rw [hReasonText] at hFail
                  let opportunityId := trimString rawOpportunityId
                  let role := trimString rawRole
                  let phase := trimString rawPhase
                  let reason := trimString rawReason
                  let message := getOptionalString payload "message"
                  let memberId := getOptionalString payload "member_id"
                  let model := getOptionalString payload "model"
                  by_cases hOpportunityEmpty : opportunityId = ""
                  · simp [opportunityId, hOpportunityEmpty] at hFail
                  · by_cases hRoleEmpty : role = ""
                    · simp [opportunityId, role, hOpportunityEmpty, hRoleEmpty] at hFail
                    · by_cases hPhaseEmpty : phase = ""
                      · simp [opportunityId, role, phase, hOpportunityEmpty, hRoleEmpty,
                          hPhaseEmpty] at hFail
                      · by_cases hReasonEmpty : reason = ""
                        · simp [opportunityId, role, phase, reason, hOpportunityEmpty, hRoleEmpty,
                            hPhaseEmpty, hReasonEmpty] at hFail
                        · cases hNext : (nextOpportunity s).opportunity with
                          | none =>
                              simp [opportunityId, role, phase, reason, hOpportunityEmpty,
                                hRoleEmpty, hPhaseEmpty, hReasonEmpty, hNext] at hFail
                          | some opportunity =>
                              by_cases hOpportunityMismatch :
                                  opportunity.opportunity_id != opportunityId
                              · simp [opportunityId, role, phase, reason, hOpportunityEmpty,
                                  hRoleEmpty, hPhaseEmpty, hReasonEmpty, hNext,
                                  hOpportunityMismatch] at hFail
                              · by_cases hRoleMismatch : opportunity.role != role
                                · simp [opportunityId, role, phase, reason, hOpportunityEmpty,
                                    hRoleEmpty, hPhaseEmpty, hReasonEmpty, hNext,
                                    hOpportunityMismatch, hRoleMismatch] at hFail
                                · by_cases hPhaseMismatch : opportunity.phase != phase
                                  · simp [opportunityId, role, phase, reason, hOpportunityEmpty,
                                      hRoleEmpty, hPhaseEmpty, hReasonEmpty, hNext,
                                      hOpportunityMismatch, hRoleMismatch, hPhaseMismatch] at hFail
                                  · have hOpportunityMatch :
                                        (opportunity.opportunity_id != opportunityId) = false := by
                                      simpa using hOpportunityMismatch
                                    have hRoleMatch : (opportunity.role != role) = false := by
                                      simpa using hRoleMismatch
                                    have hPhaseMatch : (opportunity.phase != phase) = false := by
                                      simpa using hPhaseMismatch
                                    have hOpportunityEq :
                                        opportunity.opportunity_id = opportunityId := by
                                      by_cases hEq : opportunity.opportunity_id = opportunityId
                                      · exact hEq
                                      · have hTrue :
                                            (opportunity.opportunity_id != opportunityId) = true := by
                                          simp [hEq]
                                        simp [hOpportunityMatch] at hTrue
                                    have hRoleEq : opportunity.role = role := by
                                      by_cases hEq : opportunity.role = role
                                      · exact hEq
                                      · have hTrue : (opportunity.role != role) = true := by
                                          simp [hEq]
                                        simp [hRoleMatch] at hTrue
                                    have hPhaseEq : opportunity.phase = phase := by
                                      by_cases hEq : opportunity.phase = phase
                                      · exact hEq
                                      · have hTrue : (opportunity.phase != phase) = true := by
                                          simp [hEq]
                                        simp [hPhaseMatch] at hTrue
                                    by_cases hCouncil : role = "council"
                                    · have hFailCouncil :
                                          failCouncilMemberOpportunity s memberId reason opportunityId message =
                                            .ok t := by
                                        simpa [opportunityId, role, phase, reason, message, memberId,
                                          model, hOpportunityEmpty, hRoleEmpty, hPhaseEmpty,
                                          hReasonEmpty, hNext, hOpportunityMatch, hRoleMatch,
                                          hPhaseMatch, hOpportunityEq, hRoleEq, hPhaseEq, hCouncil]
                                          using hFail
                                      rcases failCouncilMemberOpportunity_result s t memberId reason
                                          opportunityId message hFailCouncil with
                                        ⟨c1, hC1, hDelib, hSeated, hFresh, hCont⟩
                                      exact Or.inl ⟨memberId, reason, opportunityId, message, c1,
                                        hC1, hDelib, hSeated, hFresh, hCont⟩
                                    · by_cases hParty : role = "plaintiff" || role = "defendant"
                                      · let failure : OpportunityFailure := {
                                          failure_type := "opportunity_failed"
                                          role := role
                                          phase := phase
                                          opportunity_id := opportunityId
                                          reason := reason
                                          message := message
                                          member_id := memberId
                                          model := model
                                        }
                                        have hEq :
                                            t = stateWithCase s
                                              { s.case with
                                                status := "failed"
                                                failure := some failure } := by
                                          simpa [opportunityId, role, phase, reason, message, memberId,
                                            model, failure, hOpportunityEmpty, hRoleEmpty, hPhaseEmpty,
                                            hReasonEmpty, hNext, hOpportunityMatch, hRoleMatch,
                                            hPhaseMatch, hOpportunityEq, hRoleEq, hPhaseEq, hCouncil,
                                            hParty] using hFail.symm
                                        have hSourceNotClosed : s.case.phase ≠ "closed" := by
                                          intro hClosed
                                          unfold nextOpportunity at hNext
                                          by_cases hStatusClosed : s.case.status = "closed"
                                          · simp [hStatusClosed] at hNext
                                          · by_cases hStatusFailed : s.case.status = "failed"
                                            · cases hFailure : s.case.failure with
                                              | none =>
                                                  simp [hStatusFailed, hFailure] at hNext
                                              | some failure =>
                                                  simp [hStatusFailed, hFailure] at hNext
                                            · simp [hStatusClosed, hStatusFailed, nextOpportunityForPhase,
                                                hClosed] at hNext
                                        have hClosedT : t.case.phase ≠ "closed" := by
                                          rw [hEq]
                                          simpa [stateWithCase] using hSourceNotClosed
                                        have hSourceNotDeliberation :
                                            s.case.phase ≠ "deliberation" := by
                                          intro hDeliberation
                                          unfold nextOpportunity at hNext
                                          by_cases hStatusClosed : s.case.status = "closed"
                                          · simp [hStatusClosed] at hNext
                                          · by_cases hStatusFailed : s.case.status = "failed"
                                            · cases hFailure : s.case.failure with
                                              | none =>
                                                  simp [hStatusFailed, hFailure] at hNext
                                              | some failure =>
                                                  simp [hStatusFailed, hFailure] at hNext
                                            · cases hMember : nextCouncilMember? s.case with
                                              | none =>
                                                  simp [hStatusClosed, hStatusFailed, nextOpportunityForPhase,
                                                    hDeliberation, hMember] at hNext
                                              | some member =>
                                                  simp [hStatusClosed, hStatusFailed, nextOpportunityForPhase,
                                                    hDeliberation, hMember] at hNext
                                                  have hRoleCouncil : opportunity.role = "council" := by
                                                    cases hNext
                                                    rfl
                                                  have hRoleIsCouncil : role = "council" := by
                                                    rw [← hRoleEq]
                                                    exact hRoleCouncil
                                                  exact hCouncil hRoleIsCouncil
                                        have hFailureType :
                                            failure.failure_type = "opportunity_failed" := by
                                          simp [failure]
                                        have hFailureRole :
                                            failure.role = "plaintiff" ∨
                                              failure.role = "defendant" := by
                                          have hRoleParty :
                                              role = "plaintiff" ∨ role = "defendant" := by
                                            simpa using hParty
                                          rcases hRoleParty with hPlaintiff | hDefendant
                                          · exact Or.inl (by simp [failure, hPlaintiff])
                                          · exact Or.inr (by simp [failure, hDefendant])
                                        have hFailurePhase :
                                            failure.phase = s.case.phase := by
                                          have hOpportunityPhase :
                                              opportunity.phase = s.case.phase :=
                                            nextOpportunity_phase_eq s opportunity hNext
                                          calc
                                            failure.phase = phase := by simp [failure]
                                            _ = opportunity.phase := hPhaseEq.symm
                                            _ = s.case.phase := hOpportunityPhase
                                        exact Or.inr ⟨failure, hEq, hClosedT,
                                          hSourceNotDeliberation, hFailureType, hFailureRole,
                                          hFailurePhase⟩
                                      · simp [opportunityId, role, phase, reason, hOpportunityEmpty,
                                          hRoleEmpty, hPhaseEmpty, hReasonEmpty, hNext, hOpportunityEq,
                                          hRoleEq, hPhaseEq, hCouncil, hParty] at hFail

theorem failOpportunity_preserves_phaseShape
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hShape : phaseShape s.case)
    (hFail : failOpportunity s payload = .ok t) :
    phaseShape t.case := by
  rcases failOpportunity_result s t payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨memberId, reason, opportunityId, message, c1, hC1,
      hDelib, _hSeated, _hFresh, hCont⟩
    have hShape1 : phaseShape c1 := by
      rw [hC1]
      exact setCouncilMembers_preserves_phaseShape s.case
        (List.map (fun (member : CouncilMember) =>
          if member.member_id = memberId then
            { member with
              status := "failed"
              failure_reason := reason
              failure_opportunity_id := opportunityId
              failure_message := message }
          else member) s.case.council_members) hShape
    have hPhase1 : c1.phase = "deliberation" := by
      rw [hC1]
      simpa using hDelib
    exact continueDeliberation_preserves_phaseShape_for s t c1 hShape1 hPhase1 hCont
  · rcases hParty with ⟨failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact stateWithCase_preserves_phaseShape s _
      (failCase_preserves_phaseShape s.case failure hShape)

theorem failOpportunity_preserves_material_limits
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hLimits : materialLimitsRespected s)
    (hFail : failOpportunity s payload = .ok t) :
    materialLimitsRespected t := by
  rcases failOpportunity_result s t payload hFail with hCouncil | hParty
  · rcases hCouncil with ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
      _hDelib, _hSeated, _hFresh, hCont⟩
    exact continueDeliberation_preserves_material_limits_for s t c1 hLimits
      (by rw [hC1])
      (by rw [hC1])
      hCont
  · rcases hParty with ⟨failure, rfl, _hNotClosed, _hNotDeliberation,
      _hFailureType, _hFailureRole, _hFailurePhase⟩
    exact failCase_preserves_material_limits s failure hLimits

/--
A successful council-vote step preserves the merits-sequence invariant.

This step changes the current vote list and then calls `continueDeliberation`.
Neither part rewrites the merits filings.  The remaining proof work is the
public wrapper: strip the role and payload parsing, then appeal to
`continueDeliberation_preserves_phaseShape_for`.
-/
theorem step_submit_council_vote_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, hPhase, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hShape1 : phaseShape c1 := by
    simpa [c1] using
      setCouncilVotes_preserves_phaseShape s.case
        (s.case.council_votes.concat {
          member_id := memberId
          round := s.case.deliberation_round
          vote := trimString vote
          rationale := trimString rationale
        }) hShape
  have hPhase1 : c1.phase = "deliberation" := by
    simpa [c1] using hPhase
  exact continueDeliberation_preserves_phaseShape_for s t c1 hShape1 hPhase1 hCont

/--
A successful council-vote step preserves the aggregate material limits.
-/
theorem step_submit_council_vote_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, _hPhase, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  exact continueDeliberation_preserves_material_limits_for s t c1 hLimits
    (by simp [c1])
    (by simp [c1])
    hCont

/--
A successful council-removal step preserves the merits-sequence invariant.

Removing a timed-out council member changes seating metadata and then calls
`continueDeliberation`.  It does not change the merits filings.
-/
theorem step_remove_council_member_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, hPhase, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hShape1 : phaseShape c1 := by
    simpa [c1] using
      setCouncilMembers_preserves_phaseShape s.case
        (s.case.council_members.map (fun (member : CouncilMember) =>
          if member.member_id = memberId then
            { member with status := trimString status }
          else
            member)) hShape
  have hPhase1 : c1.phase = "deliberation" := by
    simpa [c1] using hPhase
  exact continueDeliberation_preserves_phaseShape_for s t c1 hShape1 hPhase1 hCont

/--
A successful council-removal step preserves the aggregate material limits.
-/
theorem step_remove_council_member_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, _hPhase, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  exact continueDeliberation_preserves_material_limits_for s t c1 hLimits
    (by simp [c1])
    (by simp [c1])
    hCont

theorem step_fail_opportunity_preserves_phaseShape
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hShape : phaseShape s.case)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    phaseShape t.case := by
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
      exact failOpportunity_preserves_phaseShape s t action.payload hShape hStep'

theorem step_fail_opportunity_preserves_material_limits
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "fail_opportunity")
    (hLimits : materialLimitsRespected s)
    (hStep : stepCore { state := s, action := action } = .ok t) :
    materialLimitsRespected t := by
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
      exact failOpportunity_preserves_material_limits s t action.payload hLimits hStep'

end ArbProofs
