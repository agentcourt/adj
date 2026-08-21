import Proofs.OpportunityAgreement

namespace ArbdProofs

def materialOriginAllowed (phase role : String) : Prop :=
  (phase = "arguments" ∧ (role = "plaintiff" ∨ role = "defendant")) ∨
    (phase = "rebuttals" ∧ role = "plaintiff") ∨
    (phase = "surrebuttals" ∧ role = "defendant")

def EvidenceCatalogValid (catalog : List EvidenceCommitment) : Prop :=
  hasDuplicateEvidenceCommitmentIds catalog = false ∧
    hasInvalidEvidenceCommitments catalog = false

inductive SubmittedEvidenceHistoryValid
    (catalog : List EvidenceCommitment)
    (maxBytes : Nat) : List SubmittedEvidence → Prop where
  | nil : SubmittedEvidenceHistoryValid catalog maxBytes []
  | snoc
      (prior : List SubmittedEvidence)
      (item : SubmittedEvidence)
      (history : SubmittedEvidenceHistoryValid catalog maxBytes prior)
      (origin : materialOriginAllowed item.phase item.role)
      (entry : submittedEvidenceEntryValid catalog prior maxBytes item = true) :
      SubmittedEvidenceHistoryValid catalog maxBytes (prior.concat item)

structure RecordIntegrity (s : ArbitrationState) : Prop where
  catalog : EvidenceCatalogValid s.evidence_catalog
  submitted : SubmittedEvidenceHistoryValid
    s.evidence_catalog s.policy.max_submitted_evidence_bytes s.case.submitted_evidence
  offered : offeredEvidenceBatchValid
    s.evidence_catalog s.case.submitted_evidence
    s.policy.max_exhibit_bytes s.case.offered_evidence = true
  reports : technicalReportBatchValid
    s.policy.max_report_title_bytes s.policy.max_report_summary_bytes
    s.case.technical_reports = true

structure MeritsOffersUsePriorRecord
    (s : ArbitrationState)
    (action : CourtAction) : Prop where
  argument :
    action.action_type = "submit_argument" →
      ∃ offered,
        parseOfferedEvidence action.payload "arguments"
            (if s.case.arguments.isEmpty then "plaintiff" else "defendant") =
          .ok offered ∧
        offeredEvidenceBatchValid
            s.evidence_catalog s.case.submitted_evidence
            s.policy.max_exhibit_bytes offered = true
  rebuttal :
    action.action_type = "submit_rebuttal" →
      ∃ offered,
        parseOfferedEvidence action.payload "rebuttals" "plaintiff" = .ok offered ∧
        offeredEvidenceBatchValid
            s.evidence_catalog s.case.submitted_evidence
            s.policy.max_exhibit_bytes offered = true
  surrebuttal :
    action.action_type = "submit_surrebuttal" →
      ∃ offered,
        parseOfferedEvidence action.payload "surrebuttals" "defendant" = .ok offered ∧
        offeredEvidenceBatchValid
            s.evidence_catalog s.case.submitted_evidence
            s.policy.max_exhibit_bytes offered = true

theorem validateEvidenceCatalog_ok
    (catalog : List EvidenceCommitment)
    (hValid : validateEvidenceCatalog catalog = .ok ()) :
    EvidenceCatalogValid catalog := by
  unfold validateEvidenceCatalog at hValid
  cases hDuplicate : hasDuplicateEvidenceCommitmentIds catalog with
  | true => simp [hDuplicate, Bind.bind, Except.bind] at hValid
  | false =>
      cases hInvalid : hasInvalidEvidenceCommitments catalog with
      | true => simp [hDuplicate, hInvalid] at hValid
      | false => exact ⟨hDuplicate, hInvalid⟩

theorem validateOfferedEvidenceBatch_ok
    (catalog : List EvidenceCommitment)
    (submitted : List SubmittedEvidence)
    (maxBytes : Nat)
    (offered : List OfferedEvidence)
    (hValid : validateOfferedEvidenceBatch catalog submitted maxBytes offered = .ok ()) :
    offeredEvidenceBatchValid catalog submitted maxBytes offered = true := by
  unfold validateOfferedEvidenceBatch at hValid
  cases hBatch : offeredEvidenceBatchValid catalog submitted maxBytes offered <;>
    simp [hBatch] at hValid ⊢

theorem validateTechnicalReportBatch_ok
    (maxTitleBytes maxSummaryBytes : Nat)
    (reports : List TechnicalReport)
    (hValid : validateTechnicalReportBatch maxTitleBytes maxSummaryBytes reports = .ok ()) :
    technicalReportBatchValid maxTitleBytes maxSummaryBytes reports = true := by
  unfold validateTechnicalReportBatch at hValid
  cases hBatch : technicalReportBatchValid maxTitleBytes maxSummaryBytes reports <;>
    simp [hBatch] at hValid ⊢

theorem validateSubmittedEvidenceParent_ok
    (catalog : List EvidenceCommitment)
    (prior : List SubmittedEvidence)
    (item : SubmittedEvidence)
    (hValid : validateSubmittedEvidenceParent catalog prior item = .ok ()) :
    submittedEvidenceParentValid catalog prior item = true := by
  unfold validateSubmittedEvidenceParent at hValid
  cases hEmpty :
      (item.parent_evidence_id = "" && item.parent_sha256 = "" &&
        item.derivation_method = "") with
  | true => simp [submittedEvidenceParentValid, hEmpty]
  | false =>
      cases hIncomplete :
          (item.parent_evidence_id = "" || item.parent_sha256 = "" ||
            item.derivation_method = "") with
      | true =>
          simp [hEmpty, hIncomplete, Bind.bind, Except.bind] at hValid
      | false =>
          cases hCanonical : isCanonicalSHA256 item.parent_sha256 with
          | false =>
              simp [hEmpty, hIncomplete, hCanonical, Bind.bind, Except.bind] at hValid
          | true =>
              by_cases hSelf : item.parent_evidence_id = item.evidence_id
              · have hParentIdNonempty : item.parent_evidence_id ≠ "" := by
                  intro hParentIdEmpty
                  have hIncompleteTrue :
                      (item.parent_evidence_id = "" || item.parent_sha256 = "" ||
                        item.derivation_method = "") = true := by
                    simp [hParentIdEmpty]
                  rw [hIncomplete] at hIncompleteTrue
                  cases hIncompleteTrue
                have hParentShaNonempty : item.parent_sha256 ≠ "" := by
                  intro hParentShaEmpty
                  have hIncompleteTrue :
                      (item.parent_evidence_id = "" || item.parent_sha256 = "" ||
                        item.derivation_method = "") = true := by
                    simp [hParentShaEmpty]
                  rw [hIncomplete] at hIncompleteTrue
                  cases hIncompleteTrue
                have hMethodNonempty : item.derivation_method ≠ "" := by
                  intro hMethodEmpty
                  have hIncompleteTrue :
                      (item.parent_evidence_id = "" || item.parent_sha256 = "" ||
                        item.derivation_method = "") = true := by
                    simp [hMethodEmpty]
                  rw [hIncomplete] at hIncompleteTrue
                  cases hIncompleteTrue
                have hChildIdNonempty : item.evidence_id ≠ "" := by
                  simpa [hSelf] using hParentIdNonempty
                simp [hParentShaNonempty, hMethodNonempty,
                  hChildIdNonempty, hCanonical, hSelf, Bind.bind, Except.bind] at hValid
              · cases hCommitment : evidenceCommitmentExists
                    catalog prior item.parent_evidence_id item.parent_sha256 with
                | false =>
                    simp [hEmpty, hIncomplete, hCanonical, hSelf, hCommitment] at hValid
                | true =>
                    simp [submittedEvidenceParentValid, hEmpty, hIncomplete,
                      hCanonical, hSelf, hCommitment]

theorem validateSubmittedEvidenceEntry_ok
    (catalog : List EvidenceCommitment)
    (prior : List SubmittedEvidence)
    (maxBytes : Nat)
    (item : SubmittedEvidence)
    (hValid : validateSubmittedEvidenceEntry catalog prior maxBytes item = .ok ()) :
    submittedEvidenceEntryValid catalog prior maxBytes item = true := by
  unfold validateSubmittedEvidenceEntry at hValid
  cases hMetadata : submittedEvidenceMetadataValid item with
  | false =>
      rw [hMetadata] at hValid
      simp [Bind.bind, Except.bind] at hValid
  | true =>
      rw [hMetadata] at hValid
      cases hCatalog : catalog.any (fun commitment =>
          commitment.evidence_id = item.evidence_id) with
      | true =>
          rw [hCatalog] at hValid
          simp [Bind.bind, Except.bind] at hValid
      | false =>
          rw [hCatalog] at hValid
          cases hPrior : prior.any (fun previous =>
              previous.evidence_id = item.evidence_id) with
          | true =>
              rw [hPrior] at hValid
              simp [Bind.bind, Except.bind] at hValid
          | false =>
              rw [hPrior] at hValid
              by_cases hSize : item.size_bytes > maxBytes
              · simp [hSize, Bind.bind, Except.bind] at hValid
              · have hParentCall :
                    validateSubmittedEvidenceParent catalog prior item = .ok () := by
                    simpa [hSize, Bind.bind, Except.bind] using hValid
                have hParent := validateSubmittedEvidenceParent_ok
                  catalog prior item hParentCall
                have hLe : item.size_bytes ≤ maxBytes := Nat.le_of_not_gt hSize
                simp [submittedEvidenceEntryValid, hMetadata, hCatalog, hPrior,
                  hLe, hParent]

theorem offeredEvidenceBatchValid_append
    (catalog : List EvidenceCommitment)
    (submitted : List SubmittedEvidence)
    (maxBytes : Nat)
    (left right : List OfferedEvidence)
    (hLeft : offeredEvidenceBatchValid catalog submitted maxBytes left = true)
    (hRight : offeredEvidenceBatchValid catalog submitted maxBytes right = true) :
    offeredEvidenceBatchValid catalog submitted maxBytes (left ++ right) = true := by
  simp only [offeredEvidenceBatchValid, List.all_eq_true] at hLeft hRight ⊢
  intro item hItem
  rcases List.mem_append.mp hItem with hItemLeft | hItemRight
  · exact hLeft item hItemLeft
  · exact hRight item hItemRight

theorem technicalReportBatchValid_append
    (maxTitleBytes maxSummaryBytes : Nat)
    (left right : List TechnicalReport)
    (hLeft : technicalReportBatchValid maxTitleBytes maxSummaryBytes left = true)
    (hRight : technicalReportBatchValid maxTitleBytes maxSummaryBytes right = true) :
    technicalReportBatchValid maxTitleBytes maxSummaryBytes (left ++ right) = true := by
  simp only [technicalReportBatchValid, List.all_eq_true] at hLeft hRight ⊢
  intro report hReport
  rcases List.mem_append.mp hReport with hReportLeft | hReportRight
  · exact hLeft report hReportLeft
  · exact hRight report hReportRight

theorem evidenceReferenceWithinLimit_submitted_concat
    (catalog : List EvidenceCommitment)
    (prior : List SubmittedEvidence)
    (maxBytes : Nat)
    (evidenceId : String)
    (item : SubmittedEvidence)
    (hReference : evidenceReferenceWithinLimit catalog prior maxBytes evidenceId = true) :
    evidenceReferenceWithinLimit catalog (prior.concat item) maxBytes evidenceId = true := by
  unfold evidenceReferenceWithinLimit at hReference ⊢
  cases hCatalog : catalog.any (fun catalogItem =>
      catalogItem.evidence_id = evidenceId && decide (catalogItem.size_bytes ≤ maxBytes)) with
  | true => rfl
  | false =>
      cases hPrior : prior.any (fun priorItem =>
          priorItem.evidence_id = evidenceId && decide (priorItem.size_bytes ≤ maxBytes)) with
      | false =>
          rw [hCatalog, hPrior] at hReference
          cases hReference
      | true =>
          rw [List.concat_eq_append, List.any_append, hPrior]
          rfl

theorem offeredEvidenceBatchValid_submitted_concat
    (catalog : List EvidenceCommitment)
    (prior : List SubmittedEvidence)
    (maxBytes : Nat)
    (offered : List OfferedEvidence)
    (item : SubmittedEvidence)
    (hOffered : offeredEvidenceBatchValid catalog prior maxBytes offered = true) :
    offeredEvidenceBatchValid catalog (prior.concat item) maxBytes offered = true := by
  simp only [offeredEvidenceBatchValid, List.all_eq_true] at hOffered ⊢
  intro offeredItem hMember
  exact evidenceReferenceWithinLimit_submitted_concat
    catalog prior maxBytes offeredItem.evidence_id item (hOffered offeredItem hMember)

theorem advanceAfterMerits_preserves_record
    (c : ArbitrationCase) :
    (advanceAfterMerits c).submitted_evidence = c.submitted_evidence ∧
      (advanceAfterMerits c).offered_evidence = c.offered_evidence ∧
        (advanceAfterMerits c).technical_reports = c.technical_reports := by
  unfold advanceAfterMerits
  split <;> try { exact ⟨rfl, rfl, rfl⟩ }
  split <;> try { exact ⟨rfl, rfl, rfl⟩ }
  split <;> try { exact ⟨rfl, rfl, rfl⟩ }
  split <;> try { exact ⟨rfl, rfl, rfl⟩ }
  split <;> exact ⟨rfl, rfl, rfl⟩

theorem addFiling_preserves_record
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).submitted_evidence = c.submitted_evidence ∧
      (addFiling c phase role text).offered_evidence = c.offered_evidence ∧
        (addFiling c phase role text).technical_reports = c.technical_reports := by
  unfold addFiling
  split <;> simp [advanceAfterMerits_preserves_record]

theorem stateWithCase_preserves_recordIntegrity
    (s : ArbitrationState)
    (c : ArbitrationCase)
    (hIntegrity : RecordIntegrity s)
    (hSubmitted : c.submitted_evidence = s.case.submitted_evidence)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports) :
    RecordIntegrity (stateWithCase s c) := by
  exact
    { catalog := by simpa [stateWithCase] using hIntegrity.catalog
      submitted := by simpa [stateWithCase, hSubmitted] using hIntegrity.submitted
      offered := by simpa [stateWithCase, hSubmitted, hOffered] using hIntegrity.offered
      reports := by simpa [stateWithCase, hReports] using hIntegrity.reports }

theorem appendMaterials_preserves_recordIntegrity
    (s : ArbitrationState)
    (phase role text : String)
    (offered : List OfferedEvidence)
    (reports : List TechnicalReport)
    (hIntegrity : RecordIntegrity s)
    (hOffered : offeredEvidenceBatchValid
      s.evidence_catalog s.case.submitted_evidence s.policy.max_exhibit_bytes offered = true)
    (hReports : technicalReportBatchValid
      s.policy.max_report_title_bytes s.policy.max_report_summary_bytes reports = true) :
    RecordIntegrity
      (stateWithCase s
        (appendSupplementalMaterials
          (addFiling s.case phase role text) offered reports)) := by
  have hRecord := addFiling_preserves_record s.case phase role text
  exact
    { catalog := by simpa [stateWithCase] using hIntegrity.catalog
      submitted := by
        simpa [stateWithCase, appendSupplementalMaterials, hRecord.1] using
          hIntegrity.submitted
      offered := by
        simpa [stateWithCase, appendSupplementalMaterials, hRecord.1, hRecord.2.1] using
          offeredEvidenceBatchValid_append
            s.evidence_catalog s.case.submitted_evidence s.policy.max_exhibit_bytes
            s.case.offered_evidence offered hIntegrity.offered hOffered
      reports := by
        simpa [stateWithCase, appendSupplementalMaterials, hRecord.2.2] using
          technicalReportBatchValid_append
            s.policy.max_report_title_bytes s.policy.max_report_summary_bytes
            s.case.technical_reports reports hIntegrity.reports hReports }

theorem appendSubmittedEvidence_preserves_recordIntegrity
    (s : ArbitrationState)
    (item : SubmittedEvidence)
    (hIntegrity : RecordIntegrity s)
    (hOrigin : materialOriginAllowed item.phase item.role)
    (hEntry : submittedEvidenceEntryValid
      s.evidence_catalog s.case.submitted_evidence
      s.policy.max_submitted_evidence_bytes item = true) :
    RecordIntegrity (stateWithCase s (appendSubmittedEvidence s.case item)) := by
  exact
    { catalog := by simpa [stateWithCase] using hIntegrity.catalog
      submitted := by
        simpa [stateWithCase, appendSubmittedEvidence] using
          SubmittedEvidenceHistoryValid.snoc
            s.case.submitted_evidence item hIntegrity.submitted hOrigin hEntry
      offered := by
        simpa [stateWithCase, appendSubmittedEvidence] using
          offeredEvidenceBatchValid_submitted_concat
            s.evidence_catalog s.case.submitted_evidence s.policy.max_exhibit_bytes
            s.case.offered_evidence item hIntegrity.offered
      reports := by
        simpa [stateWithCase, appendSubmittedEvidence] using hIntegrity.reports }

theorem initializeCase_establishes_recordIntegrity_and_catalog
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    RecordIntegrity s ∧ s.evidence_catalog = req.state.evidence_catalog := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error err => simp [hPolicy, Bind.bind, Except.bind] at hInit
  | ok okv =>
      cases okv
      by_cases hQuestion : trimString req.question = ""
      · simp [hPolicy, hQuestion, Bind.bind, Except.bind] at hInit
      · by_cases hStandard : trimString req.state.policy.judgment_standard = ""
        · simp [hPolicy, hQuestion, hStandard, Bind.bind, Except.bind] at hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hQuestion, hStandard, hEmpty, Bind.bind,
              Except.bind] at hInit
          · by_cases hLength :
                req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hQuestion, hStandard, hEmpty, hLength, Bind.bind,
                Except.bind] at hInit
            · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
              · simp [hPolicy, hQuestion, hStandard, hEmpty, hLength,
                  hDuplicate, Bind.bind, Except.bind] at hInit
              · cases hCatalog : validateEvidenceCatalog req.state.evidence_catalog with
                | error err =>
                    simp [hPolicy, hQuestion, hStandard, hEmpty, hLength,
                      hDuplicate, hCatalog, Bind.bind, Except.bind] at hInit
                | ok okv =>
                    cases okv
                    simp [hPolicy, hQuestion, hStandard, hEmpty, hLength,
                      hDuplicate, hCatalog, stateWithCase, Bind.bind, Except.bind] at hInit
                    cases hInit
                    constructor
                    · exact
                        { catalog := validateEvidenceCatalog_ok
                            req.state.evidence_catalog hCatalog
                          submitted := SubmittedEvidenceHistoryValid.nil
                          offered := by simp [offeredEvidenceBatchValid]
                          reports := by simp [technicalReportBatchValid] }
                    · rfl

theorem initializeCase_establishes_recordIntegrity
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    RecordIntegrity s :=
  (initializeCase_establishes_recordIntegrity_and_catalog req s hInit).1

theorem initializeCase_preserves_evidenceCatalog
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.evidence_catalog = req.state.evidence_catalog :=
  (initializeCase_establishes_recordIntegrity_and_catalog req s hInit).2

theorem continueDeliberation_preserves_recordIntegrity_and_catalog
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hIntegrity : RecordIntegrity s)
    (hSubmitted : c.submitted_evidence = s.case.submitted_evidence)
    (hOffered : c.offered_evidence = s.case.offered_evidence)
    (hReports : c.technical_reports = s.case.technical_reports)
    (hContinue : continueDeliberation s c = .ok t) :
    RecordIntegrity t ∧ t.evidence_catalog = s.evidence_catalog := by
  unfold continueDeliberation at hContinue
  by_cases hComplete : (currentRoundAnswers c).length = seatedCouncilMemberCount c
  · simp [hComplete] at hContinue
    cases hContinue
    exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
      (by simpa using hSubmitted) (by simpa using hOffered) (by simpa using hReports),
      by simp [stateWithCase]⟩
  · simp [hComplete] at hContinue
    cases hContinue
    exact ⟨stateWithCase_preserves_recordIntegrity s c hIntegrity
      hSubmitted hOffered hReports, by simp [stateWithCase]⟩

theorem recordMeritsSubmission_with_materials_record_details
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit true payload = .ok t) :
    ∃ rawText : String, ∃ offered : List OfferedEvidence, ∃ reports : List TechnicalReport,
      parseOfferedEvidence payload phase expectedRole = .ok offered ∧
      parseTechnicalReports payload phase expectedRole = .ok reports ∧
      validateOfferedEvidenceBatch
          s.evidence_catalog s.case.submitted_evidence s.policy.max_exhibit_bytes offered = .ok () ∧
      validateTechnicalReportBatch
          s.policy.max_report_title_bytes s.policy.max_report_summary_bytes reports = .ok () ∧
      t = stateWithCase s
        (appendSupplementalMaterials
          (addFiling s.case phase expectedRole (trimString rawText)) offered reports) := by
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
        requireTextWithinLimit textLabel (trimString rawText) limit
        let offered ← parseOfferedEvidence payload phase expectedRole
        let reports ← parseTechnicalReports payload phase expectedRole
        requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
        requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
        let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
        let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
        requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
        requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
        validateOfferedEvidenceBatch
          s.evidence_catalog s.case.submitted_evidence s.policy.max_exhibit_bytes offered
        validateTechnicalReportBatch
          s.policy.max_report_title_bytes s.policy.max_report_summary_bytes reports
        pure <| stateWithCase s
          (appendSupplementalMaterials
            (addFiling s.case phase expectedRole (trimString rawText)) offered reports)) = .ok t := by
    simpa [recordMeritsSubmission, hPhase] using hSubmit
  cases hRole : requireRole actorRole expectedRole with
  | error err =>
      rw [hRole] at hSubmit'
      simp at hSubmit'
      cases hSubmit'
  | ok roleValue =>
      cases roleValue
      simp only [hRole, Bind.bind, Except.bind] at hSubmit'
      cases hText : getString payload "text" with
      | error err =>
          rw [hText] at hSubmit'
          cases hSubmit'
      | ok rawText =>
          simp only [hText] at hSubmit'
          cases hTextLimit : requireTextWithinLimit textLabel (trimString rawText) limit with
          | error err =>
              rw [hTextLimit] at hSubmit'
              cases hSubmit'
          | ok textValue =>
              cases textValue
              simp only [hTextLimit] at hSubmit'
              cases hOffered : parseOfferedEvidence payload phase expectedRole with
              | error err =>
                  rw [hOffered] at hSubmit'
                  cases hSubmit'
              | ok offered =>
                  simp only [hOffered] at hSubmit'
                  cases hReports : parseTechnicalReports payload phase expectedRole with
                  | error err =>
                      rw [hReports] at hSubmit'
                      cases hSubmit'
                  | ok reports =>
                      simp only [hReports] at hSubmit'
                      cases hOfferedPer : requireCountWithinLimit
                          "offered_evidence" offered.length s.policy.max_exhibits_per_filing with
                      | error err =>
                          simp [hOfferedPer] at hSubmit'
                      | ok offeredPerValue =>
                          cases offeredPerValue
                          simp only [hOfferedPer] at hSubmit'
                          cases hReportsPer : requireCountWithinLimit
                              "technical_reports" reports.length s.policy.max_reports_per_filing with
                          | error err =>
                              simp [hReportsPer] at hSubmit'
                          | ok reportsPerValue =>
                              cases reportsPerValue
                              simp only [hReportsPer] at hSubmit'
                              let totalOffered := offeredEvidenceCountForRole
                                s.case.offered_evidence expectedRole + offered.length
                              let totalReports := technicalReportCountForRole
                                s.case.technical_reports expectedRole + reports.length
                              cases hOfferedSide : requireCountWithinLimit
                                  "offered_evidence for this side" totalOffered
                                  s.policy.max_exhibits_per_side with
                              | error err =>
                                  simp [totalOffered, hOfferedSide] at hSubmit'
                              | ok offeredSideValue =>
                                  cases offeredSideValue
                                  simp only [totalOffered, hOfferedSide] at hSubmit'
                                  cases hReportsSide : requireCountWithinLimit
                                      "technical_reports for this side" totalReports
                                      s.policy.max_reports_per_side with
                                  | error err =>
                                      simp [totalReports, hReportsSide] at hSubmit'
                                  | ok reportsSideValue =>
                                      cases reportsSideValue
                                      simp only [totalReports, hReportsSide] at hSubmit'
                                      cases hOfferedValid : validateOfferedEvidenceBatch
                                          s.evidence_catalog s.case.submitted_evidence
                                          s.policy.max_exhibit_bytes offered with
                                      | error err =>
                                          simp [hOfferedValid] at hSubmit'
                                      | ok offeredValidValue =>
                                          cases offeredValidValue
                                          simp only [hOfferedValid] at hSubmit'
                                          cases hReportsValid : validateTechnicalReportBatch
                                              s.policy.max_report_title_bytes
                                              s.policy.max_report_summary_bytes reports with
                                          | error err =>
                                              simp [hReportsValid] at hSubmit'
                                          | ok reportsValidValue =>
                                              cases reportsValidValue
                                              simp only [hReportsValid] at hSubmit'
                                              cases hSubmit'
                                              exact ⟨rawText, offered, reports,
                                                rfl, rfl, hOfferedValid,
                                                hReportsValid, rfl⟩

theorem submitEvidence_record_details
    (s t : ArbitrationState)
    (actorRole : String)
    (payload : Lean.Json)
    (hSubmit : submitEvidence s actorRole payload = .ok t) :
    ∃ evidence,
      materialOriginAllowed evidence.phase evidence.role ∧
      validateSubmittedEvidenceEntry
          s.evidence_catalog s.case.submitted_evidence
          s.policy.max_submitted_evidence_bytes evidence = .ok () ∧
      t = stateWithCase s (appendSubmittedEvidence s.case evidence) := by
  have handle (expectedRole : String)
      (hOrigin : materialOriginAllowed s.case.phase expectedRole)
      (hCore :
        (do
          requireRole actorRole expectedRole
          let parsedEvidence ← parseSubmittedEvidence payload s.case.phase expectedRole
          let evidence := { parsedEvidence with phase := s.case.phase, role := expectedRole }
          validateSubmittedEvidenceEntry
            s.evidence_catalog s.case.submitted_evidence
            s.policy.max_submitted_evidence_bytes evidence
          let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
          requireCountWithinLimit "submitted_evidence for this side" total
            s.policy.max_submitted_evidence_per_side
          pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t) :
      ∃ evidence,
        materialOriginAllowed evidence.phase evidence.role ∧
        validateSubmittedEvidenceEntry
            s.evidence_catalog s.case.submitted_evidence
            s.policy.max_submitted_evidence_bytes evidence = .ok () ∧
        t = stateWithCase s (appendSubmittedEvidence s.case evidence) := by
    cases hRole : requireRole actorRole expectedRole with
    | error err =>
        rw [hRole] at hCore
        simp at hCore
        cases hCore
    | ok roleValue =>
        cases roleValue
        simp only [hRole, Bind.bind, Except.bind] at hCore
        cases hEvidence : parseSubmittedEvidence payload s.case.phase expectedRole with
        | error err =>
            rw [hEvidence] at hCore
            cases hCore
        | ok parsedEvidence =>
            simp only [hEvidence] at hCore
            let evidence : SubmittedEvidence :=
              { parsedEvidence with phase := s.case.phase, role := expectedRole }
            cases hValid : validateSubmittedEvidenceEntry
                s.evidence_catalog s.case.submitted_evidence
                s.policy.max_submitted_evidence_bytes evidence with
            | error err =>
                rw [hValid] at hCore
                cases hCore
            | ok validValue =>
                cases validValue
                simp only [evidence, hValid] at hCore
                let total := submittedEvidenceCountForRole
                  s.case.submitted_evidence expectedRole + 1
                cases hCount : requireCountWithinLimit "submitted_evidence for this side"
                    total s.policy.max_submitted_evidence_per_side with
                | error err =>
                    simp [total, hCount] at hCore
                | ok countValue =>
                    cases countValue
                    simp [total, hCount] at hCore
                    cases hCore
                    exact ⟨evidence, by simpa [evidence] using hOrigin, hValid, rfl⟩
  by_cases hArguments : s.case.phase = "arguments"
  · have hCore :
        (do
          let expectedRole := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
          requireRole actorRole expectedRole
          let parsedEvidence ← parseSubmittedEvidence payload s.case.phase expectedRole
          let evidence := { parsedEvidence with phase := s.case.phase, role := expectedRole }
          validateSubmittedEvidenceEntry
            s.evidence_catalog s.case.submitted_evidence
            s.policy.max_submitted_evidence_bytes evidence
          let total := submittedEvidenceCountForRole s.case.submitted_evidence expectedRole + 1
          requireCountWithinLimit "submitted_evidence for this side" total
            s.policy.max_submitted_evidence_per_side
          pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
      simpa [submitEvidence, hArguments] using hSubmit
    exact handle (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
      (by
        unfold materialOriginAllowed
        left
        exact ⟨hArguments, by split <;> simp⟩) hCore
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · cases hEmpty : s.case.rebuttals.isEmpty with
      | true =>
          have hCore :
              (do
                requireRole actorRole "plaintiff"
                let parsedEvidence ← parseSubmittedEvidence payload s.case.phase "plaintiff"
                let evidence := { parsedEvidence with phase := s.case.phase, role := "plaintiff" }
                validateSubmittedEvidenceEntry
                  s.evidence_catalog s.case.submitted_evidence
                  s.policy.max_submitted_evidence_bytes evidence
                let total := submittedEvidenceCountForRole s.case.submitted_evidence "plaintiff" + 1
                requireCountWithinLimit "submitted_evidence for this side" total
                  s.policy.max_submitted_evidence_per_side
                pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
            simpa [submitEvidence, hArguments, hRebuttals, hEmpty] using hSubmit
          exact handle "plaintiff"
            (by unfold materialOriginAllowed; exact Or.inr (Or.inl ⟨hRebuttals, rfl⟩)) hCore
      | false =>
          simp [submitEvidence, hRebuttals, hEmpty,
            Bind.bind, Except.bind, Pure.pure, Except.pure] at hSubmit
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · cases hEmpty : s.case.surrebuttals.isEmpty with
        | true =>
            have hCore :
                (do
                  requireRole actorRole "defendant"
                  let parsedEvidence ← parseSubmittedEvidence payload s.case.phase "defendant"
                  let evidence := { parsedEvidence with phase := s.case.phase, role := "defendant" }
                  validateSubmittedEvidenceEntry
                    s.evidence_catalog s.case.submitted_evidence
                    s.policy.max_submitted_evidence_bytes evidence
                  let total := submittedEvidenceCountForRole s.case.submitted_evidence "defendant" + 1
                  requireCountWithinLimit "submitted_evidence for this side" total
                    s.policy.max_submitted_evidence_per_side
                  pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
              simpa [submitEvidence, hArguments, hRebuttals, hSurrebuttals, hEmpty] using hSubmit
            exact handle "defendant"
              (by unfold materialOriginAllowed; exact Or.inr (Or.inr ⟨hSurrebuttals, rfl⟩)) hCore
        | false =>
            simp [submitEvidence, hSurrebuttals, hEmpty,
              Bind.bind, Except.bind, Pure.pure, Except.pure] at hSubmit
      · simp [submitEvidence,
          Bind.bind, Except.bind, Pure.pure, Except.pure] at hSubmit

theorem step_record_opening_statement_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ rawText : String,
      t = stateWithCase s
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
        requireRole action.actor_role role
        let rawText ← getString action.payload "text"
        requireTextWithinLimit "opening statement" (trimString rawText)
          s.policy.max_opening_chars
        requireNoSupplementalMaterials action.payload
        pure <| stateWithCase s
          (addFiling s.case "openings" role (trimString rawText))) = .ok t := by
    simpa [stepCore, hType, hPhase, role] using hStep
  cases hRole : requireRole action.actor_role role with
  | error err =>
      rw [hRole] at hStep'
      simp at hStep'
      cases hStep'
  | ok roleValue =>
      cases roleValue
      simp only [hRole, Bind.bind, Except.bind] at hStep'
      cases hText : getString action.payload "text" with
      | error err =>
          rw [hText] at hStep'
          cases hStep'
      | ok rawText =>
          simp only [hText] at hStep'
          cases hLimit : requireTextWithinLimit "opening statement" (trimString rawText)
              s.policy.max_opening_chars with
          | error err =>
              rw [hLimit] at hStep'
              cases hStep'
          | ok limitValue =>
              cases limitValue
              simp only [hLimit] at hStep'
              cases hSupplemental : requireNoSupplementalMaterials action.payload with
              | error err =>
                  rw [hSupplemental] at hStep'
                  cases hStep'
              | ok supplementalValue =>
                  cases supplementalValue
                  simp only [hSupplemental] at hStep'
                  cases hStep'
                  exact ⟨rawText, by simp [role]⟩

theorem step_deliver_closing_statement_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    ∃ rawText : String,
      t = stateWithCase s
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
        requireRole action.actor_role role
        let rawText ← getString action.payload "text"
        requireTextWithinLimit "closing statement" (trimString rawText)
          s.policy.max_closing_chars
        requireNoSupplementalMaterials action.payload
        pure <| stateWithCase s
          (addFiling s.case "closings" role (trimString rawText))) = .ok t := by
    simpa [stepCore, hType, hPhase, role] using hStep
  cases hRole : requireRole action.actor_role role with
  | error err =>
      rw [hRole] at hStep'
      simp at hStep'
      cases hStep'
  | ok roleValue =>
      cases roleValue
      simp only [hRole, Bind.bind, Except.bind] at hStep'
      cases hText : getString action.payload "text" with
      | error err =>
          rw [hText] at hStep'
          cases hStep'
      | ok rawText =>
          simp only [hText] at hStep'
          cases hLimit : requireTextWithinLimit "closing statement" (trimString rawText)
              s.policy.max_closing_chars with
          | error err =>
              rw [hLimit] at hStep'
              cases hStep'
          | ok limitValue =>
              cases limitValue
              simp only [hLimit] at hStep'
              cases hSupplemental : requireNoSupplementalMaterials action.payload with
              | error err =>
                  rw [hSupplemental] at hStep'
                  cases hStep'
              | ok supplementalValue =>
                  cases supplementalValue
                  simp only [hSupplemental] at hStep'
                  cases hStep'
                  exact ⟨rawText, by simp [role]⟩

theorem step_pass_phase_opportunity_record_result
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    t = stateWithCase s { s.case with phase := "surrebuttals" } ∨
      t = stateWithCase s { s.case with phase := "closings" } := by
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · left
    have hPass :
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
    | ok roleValue =>
        cases roleValue
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            rfl
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · right
      have hPass :
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
      | ok roleValue =>
          cases roleValue
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              rfl
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem step_ok_meritsOffersUsePriorRecord
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    MeritsOffersUsePriorRecord s action := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  refine
    { argument := ?_
      rebuttal := ?_
      surrebuttal := ?_ }
  · intro hType
    let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
    have hSubmit : recordMeritsSubmission s "arguments" action.actor_role role
        "argument" s.policy.max_argument_chars true action.payload = .ok t := by
      simpa [stepCore, hType, role] using hStepCore
    rcases recordMeritsSubmission_with_materials_record_details
        s t "arguments" action.actor_role role "argument"
        s.policy.max_argument_chars action.payload hSubmit with
      ⟨_rawText, offered, _reports, hParse, _hReportsParse,
        hValid, _hReportsValid, _hResult⟩
    exact ⟨offered, hParse, validateOfferedEvidenceBatch_ok _ _ _ _ hValid⟩
  · intro hType
    have hSubmit : recordMeritsSubmission s "rebuttals" action.actor_role
        "plaintiff" "rebuttal" s.policy.max_rebuttal_chars true action.payload = .ok t := by
      simpa [stepCore, hType] using hStepCore
    rcases recordMeritsSubmission_with_materials_record_details
        s t "rebuttals" action.actor_role "plaintiff" "rebuttal"
        s.policy.max_rebuttal_chars action.payload hSubmit with
      ⟨_rawText, offered, _reports, hParse, _hReportsParse,
        hValid, _hReportsValid, _hResult⟩
    exact ⟨offered, hParse, validateOfferedEvidenceBatch_ok _ _ _ _ hValid⟩
  · intro hType
    have hSubmit : recordMeritsSubmission s "surrebuttals" action.actor_role
        "defendant" "surrebuttal" s.policy.max_surrebuttal_chars true action.payload = .ok t := by
      simpa [stepCore, hType] using hStepCore
    rcases recordMeritsSubmission_with_materials_record_details
        s t "surrebuttals" action.actor_role "defendant" "surrebuttal"
        s.policy.max_surrebuttal_chars action.payload hSubmit with
      ⟨_rawText, offered, _reports, hParse, _hReportsParse,
        hValid, _hReportsValid, _hResult⟩
    exact ⟨offered, hParse, validateOfferedEvidenceBatch_ok _ _ _ _ hValid⟩

theorem recordCouncilAnswer_preserves_recordIntegrity_and_catalog
    (s t : ArbitrationState)
    (memberId : String)
    (answer : Nat)
    (rationale : String)
    (hIntegrity : RecordIntegrity s)
    (hRecord : recordCouncilAnswer s memberId answer rationale = .ok t) :
    RecordIntegrity t ∧ t.evidence_catalog = s.evidence_catalog := by
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold recordCouncilAnswer at hRecord
    rw [if_neg (by simp [hPhase])] at hRecord
    cases hKnown : s.case.council_members.any
        (fun member => member.member_id = memberId) with
    | false =>
        rw [if_pos (by simp [hKnown])] at hRecord
        cases hRecord
    | true =>
        rw [if_neg (by simp [hKnown])] at hRecord
        cases hSeated : s.case.council_members.any
            (fun member => member.member_id = memberId && memberIsSeated member) with
        | false =>
            rw [if_pos (by simp [hSeated])] at hRecord
            cases hRecord
        | true =>
            rw [if_neg (by simp [hSeated])] at hRecord
            by_cases hAnswer : answer > 100
            · rw [if_pos hAnswer] at hRecord
              cases hRecord
            · rw [if_neg hAnswer] at hRecord
              by_cases hRationale : trimString rationale = ""
              · rw [if_pos hRationale] at hRecord
                cases hRecord
              · rw [if_neg hRationale] at hRecord
                cases hPrior : (currentRoundAnswers s.case).any
                    (fun existing => existing.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [hPrior])] at hRecord
                    cases hRecord
                | false =>
                    rw [if_neg (by simp [hPrior])] at hRecord
                    exact continueDeliberation_preserves_recordIntegrity_and_catalog
                      s t
                      { s.case with
                        council_answers := s.case.council_answers.concat {
                          member_id := memberId
                          round := s.case.deliberation_round
                          answer := answer
                          rationale := trimString rationale
                        } }
                      hIntegrity rfl rfl rfl hRecord
  · unfold recordCouncilAnswer at hRecord
    rw [if_pos (by simpa using hPhase)] at hRecord
    cases hRecord

theorem removeCouncilMember_preserves_recordIntegrity_and_catalog
    (s t : ArbitrationState)
    (memberId status : String)
    (hIntegrity : RecordIntegrity s)
    (hRemove : removeCouncilMember s memberId status = .ok t) :
    RecordIntegrity t ∧ t.evidence_catalog = s.evidence_catalog := by
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold removeCouncilMember at hRemove
    rw [if_neg (by simp [hPhase])] at hRemove
    cases hKnown : s.case.council_members.any
        (fun member => member.member_id = memberId) with
    | false =>
        rw [if_pos (by simp [hKnown])] at hRemove
        cases hRemove
    | true =>
        rw [if_neg (by simp [hKnown])] at hRemove
        cases hSeated : s.case.council_members.any
            (fun member => member.member_id = memberId && memberIsSeated member) with
        | false =>
            rw [if_pos (by simp [hSeated])] at hRemove
            cases hRemove
        | true =>
            rw [if_neg (by simp [hSeated])] at hRemove
            by_cases hStatusEmpty : trimString status = ""
            · rw [if_pos hStatusEmpty] at hRemove
              cases hRemove
            · rw [if_neg hStatusEmpty] at hRemove
              by_cases hSeatedStatus : trimString status = "seated"
              · rw [if_pos hSeatedStatus] at hRemove
                cases hRemove
              · rw [if_neg hSeatedStatus] at hRemove
                cases hAnswered : (currentRoundAnswers s.case).any
                    (fun existing => existing.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [hAnswered])] at hRemove
                    cases hRemove
                | false =>
                    rw [if_neg (by simp [hAnswered])] at hRemove
                    exact continueDeliberation_preserves_recordIntegrity_and_catalog
                      s t
                      { s.case with
                        council_members := s.case.council_members.map
                          (fun (member : CouncilMember) =>
                            if member.member_id = memberId then
                              { member with status := trimString status }
                            else
                              member) }
                      hIntegrity rfl rfl rfl hRemove
  · unfold removeCouncilMember at hRemove
    rw [if_pos (by simpa using hPhase)] at hRemove
    cases hRemove

theorem failCouncilMemberOpportunity_preserves_recordIntegrity_and_catalog
    (s t : ArbitrationState)
    (memberId reason opportunityId message : String)
    (hIntegrity : RecordIntegrity s)
    (hFail : failCouncilMemberOpportunity
      s memberId reason opportunityId message = .ok t) :
    RecordIntegrity t ∧ t.evidence_catalog = s.evidence_catalog := by
  by_cases hPhase : s.case.phase = "deliberation"
  · unfold failCouncilMemberOpportunity at hFail
    rw [if_neg (by simp [hPhase])] at hFail
    by_cases hMember : memberId = ""
    · rw [if_pos hMember] at hFail
      cases hFail
    · rw [if_neg hMember] at hFail
      by_cases hReason : reason = ""
      · rw [if_pos hReason] at hFail
        cases hFail
      · rw [if_neg hReason] at hFail
        cases hKnown : s.case.council_members.any
            (fun member => member.member_id = memberId) with
        | false =>
            rw [if_pos (by simp [hKnown])] at hFail
            cases hFail
        | true =>
            rw [if_neg (by simp [hKnown])] at hFail
            cases hSeated : s.case.council_members.any
                (fun member => member.member_id = memberId && memberIsSeated member) with
            | false =>
                rw [if_pos (by simp [hSeated])] at hFail
                cases hFail
            | true =>
                rw [if_neg (by simp [hSeated])] at hFail
                cases hAnswered : (currentRoundAnswers s.case).any
                    (fun existing => existing.member_id = memberId) with
                | true =>
                    rw [if_pos (by simp [hAnswered])] at hFail
                    cases hFail
                | false =>
                    rw [if_neg (by simp [hAnswered])] at hFail
                    exact continueDeliberation_preserves_recordIntegrity_and_catalog
                      s t
                      { s.case with
                        council_members := s.case.council_members.map
                          (fun (member : CouncilMember) =>
                            if member.member_id = memberId then
                              { member with
                                status := "failed"
                                failure_reason := reason
                                failure_opportunity_id := opportunityId
                                failure_message := message
                              }
                            else
                              member) }
                      hIntegrity rfl rfl rfl hFail
  · unfold failCouncilMemberOpportunity at hFail
    rw [if_pos (by simpa using hPhase)] at hFail
    cases hFail

theorem failOpportunity_preserves_recordIntegrity_and_catalog
    (s t : ArbitrationState)
    (payload : Lean.Json)
    (hIntegrity : RecordIntegrity s)
    (hFail : failOpportunity s payload = .ok t) :
    RecordIntegrity t ∧ t.evidence_catalog = s.evidence_catalog := by
  unfold failOpportunity at hFail
  cases hOpportunityId : getString payload "opportunity_id" with
  | error err =>
      simp only [hOpportunityId] at hFail
      cases hFail
  | ok rawOpportunityId =>
      simp only [hOpportunityId] at hFail
      cases hRole : getString payload "role" with
      | error err =>
          simp only [hRole] at hFail
          cases hFail
      | ok rawRole =>
          simp only [hRole] at hFail
          cases hPhase : getString payload "phase" with
          | error err =>
              simp only [hPhase] at hFail
              cases hFail
          | ok rawPhase =>
              simp only [hPhase] at hFail
              cases hReason : getString payload "reason" with
              | error err =>
                  simp only [hReason] at hFail
                  cases hFail
              | ok rawReason =>
                  simp only [hReason] at hFail
                  cases hMessage : getOptionalString payload "message" with
                  | error err =>
                      simp only [hMessage] at hFail
                      cases hFail
                  | ok message =>
                      simp only [hMessage] at hFail
                      cases hMember : getOptionalString payload "member_id" with
                      | error err =>
                          simp only [hMember] at hFail
                          cases hFail
                      | ok memberId =>
                          simp only [hMember] at hFail
                          cases hModel : getOptionalString payload "model" with
                          | error err =>
                              simp only [hModel] at hFail
                              cases hFail
                          | ok model =>
                              simp only [hModel] at hFail
                              let opportunityId := trimString rawOpportunityId
                              let role := trimString rawRole
                              let phase := trimString rawPhase
                              let reason := trimString rawReason
                              change
                                (if opportunityId = "" then
                                    throw "opportunity failure requires opportunity_id"
                                  else if role = "" then
                                    throw "opportunity failure requires role"
                                  else if phase = "" then
                                    throw "opportunity failure requires phase"
                                  else if reason = "" then
                                    throw "opportunity failure requires reason"
                                  else
                                    do
                                      let opportunity ←
                                        match (nextOpportunity s).opportunity with
                                        | none => throw "no active opportunity can fail"
                                        | some opportunity => pure opportunity
                                      if opportunity.opportunity_id != opportunityId then
                                        throw s!"opportunity_id {opportunityId} does not match current opportunity {opportunity.opportunity_id}"
                                      else if opportunity.role != role then
                                        throw s!"opportunity role {role} does not match current role {opportunity.role}"
                                      else if opportunity.phase != phase then
                                        throw s!"opportunity phase {phase} does not match current phase {opportunity.phase}"
                                      else if role = "council" then
                                        failCouncilMemberOpportunity
                                          s memberId reason opportunityId message
                                      else if role = "plaintiff" || role = "defendant" then
                                        let failure : OpportunityFailure := {
                                          failure_type := "opportunity_failed"
                                          role := role
                                          phase := phase
                                          opportunity_id := opportunityId
                                          reason := reason
                                          message := message
                                          member_id := memberId
                                          model := model
                                        }
                                        pure <| stateWithCase s
                                          { s.case with
                                            status := "failed"
                                            failure := some failure }
                                      else
                                        throw s!"unsupported opportunity failure role: {role}") =
                                  .ok t at hFail
                              by_cases hOpportunityEmpty : opportunityId = ""
                              · rw [if_pos hOpportunityEmpty] at hFail
                                cases hFail
                              · by_cases hRoleEmpty : role = ""
                                · rw [if_neg hOpportunityEmpty] at hFail
                                  rw [if_pos hRoleEmpty] at hFail
                                  cases hFail
                                · by_cases hPhaseEmpty : phase = ""
                                  · rw [if_neg hOpportunityEmpty] at hFail
                                    rw [if_neg hRoleEmpty] at hFail
                                    rw [if_pos hPhaseEmpty] at hFail
                                    cases hFail
                                  · by_cases hReasonEmpty : reason = ""
                                    · rw [if_neg hOpportunityEmpty] at hFail
                                      rw [if_neg hRoleEmpty] at hFail
                                      rw [if_neg hPhaseEmpty] at hFail
                                      rw [if_pos hReasonEmpty] at hFail
                                      cases hFail
                                    · cases hNext : (nextOpportunity s).opportunity with
                                      | none =>
                                          rw [if_neg hOpportunityEmpty] at hFail
                                          rw [if_neg hRoleEmpty] at hFail
                                          rw [if_neg hPhaseEmpty] at hFail
                                          rw [if_neg hReasonEmpty] at hFail
                                          simp only [hNext] at hFail
                                          cases hFail
                                      | some opportunity =>
                                          rw [if_neg hOpportunityEmpty] at hFail
                                          rw [if_neg hRoleEmpty] at hFail
                                          rw [if_neg hPhaseEmpty] at hFail
                                          rw [if_neg hReasonEmpty] at hFail
                                          simp only [hNext, Bind.bind, Except.bind,
                                            Pure.pure, Except.pure] at hFail
                                          by_cases hOpportunityMismatch :
                                              opportunity.opportunity_id != opportunityId
                                          · rw [if_pos hOpportunityMismatch] at hFail
                                            cases hFail
                                          · rw [if_neg hOpportunityMismatch] at hFail
                                            by_cases hRoleMismatch : opportunity.role != role
                                            · rw [if_pos hRoleMismatch] at hFail
                                              cases hFail
                                            · rw [if_neg hRoleMismatch] at hFail
                                              by_cases hPhaseMismatch : opportunity.phase != phase
                                              · rw [if_pos hPhaseMismatch] at hFail
                                                cases hFail
                                              · rw [if_neg hPhaseMismatch] at hFail
                                                by_cases hCouncil : role = "council"
                                                · rw [if_pos hCouncil] at hFail
                                                  exact
                                                    failCouncilMemberOpportunity_preserves_recordIntegrity_and_catalog
                                                      s t memberId reason opportunityId message
                                                      hIntegrity hFail
                                                · rw [if_neg hCouncil] at hFail
                                                  by_cases hParty :
                                                      role = "plaintiff" || role = "defendant"
                                                  · rw [if_pos hParty] at hFail
                                                    let failure : OpportunityFailure := {
                                                      failure_type := "opportunity_failed"
                                                      role := role
                                                      phase := phase
                                                      opportunity_id := opportunityId
                                                      reason := reason
                                                      message := message
                                                      member_id := memberId
                                                      model := model
                                                    }
                                                    have hResult :
                                                        t = stateWithCase s
                                                          { s.case with
                                                            status := "failed"
                                                            failure := some failure } := by
                                                      simpa [failure] using hFail.symm
                                                    rw [hResult]
                                                    exact ⟨stateWithCase_preserves_recordIntegrity
                                                      s _ hIntegrity rfl rfl rfl,
                                                      by simp [stateWithCase]⟩
                                                  · rw [if_neg hParty] at hFail
                                                    cases hFail

theorem step_preserves_recordIntegrity_and_catalog
    (s t : ArbitrationState)
    (action : CourtAction)
    (hIntegrity : RecordIntegrity s)
    (hStep : step { state := s, action := action } = .ok t) :
    RecordIntegrity t ∧ t.evidence_catalog = s.evidence_catalog := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with
      ⟨rawText, rfl⟩
    have hRecord := addFiling_preserves_record s.case "openings"
      (if s.case.openings.isEmpty then "plaintiff" else "defendant")
      (trimString rawText)
    exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
      hRecord.1 hRecord.2.1 hRecord.2.2, by simp [stateWithCase]⟩
  · by_cases hArgument : action.action_type = "submit_argument"
    · let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
      have hSubmit : recordMeritsSubmission s "arguments" action.actor_role role
          "argument" s.policy.max_argument_chars true action.payload = .ok t := by
        simpa [stepCore, hArgument, role] using hStepCore
      rcases recordMeritsSubmission_with_materials_record_details
          s t "arguments" action.actor_role role "argument"
          s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
          hOfferedValid, hReportsValid, rfl⟩
      exact ⟨appendMaterials_preserves_recordIntegrity s "arguments" role
        (trimString rawText) offered reports hIntegrity
        (validateOfferedEvidenceBatch_ok _ _ _ _ hOfferedValid)
        (validateTechnicalReportBatch_ok _ _ _ hReportsValid),
        by simp [stateWithCase]⟩
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hSubmit : recordMeritsSubmission s "rebuttals" action.actor_role
            "plaintiff" "rebuttal" s.policy.max_rebuttal_chars true action.payload = .ok t := by
          simpa [stepCore, hRebuttal] using hStepCore
        rcases recordMeritsSubmission_with_materials_record_details
            s t "rebuttals" action.actor_role "plaintiff" "rebuttal"
            s.policy.max_rebuttal_chars action.payload hSubmit with
          ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
            hOfferedValid, hReportsValid, rfl⟩
        exact ⟨appendMaterials_preserves_recordIntegrity s "rebuttals" "plaintiff"
          (trimString rawText) offered reports hIntegrity
          (validateOfferedEvidenceBatch_ok _ _ _ _ hOfferedValid)
          (validateTechnicalReportBatch_ok _ _ _ hReportsValid),
          by simp [stateWithCase]⟩
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hSubmit : recordMeritsSubmission s "surrebuttals" action.actor_role
              "defendant" "surrebuttal" s.policy.max_surrebuttal_chars true action.payload = .ok t := by
            simpa [stepCore, hSurrebuttal] using hStepCore
          rcases recordMeritsSubmission_with_materials_record_details
              s t "surrebuttals" action.actor_role "defendant" "surrebuttal"
              s.policy.max_surrebuttal_chars action.payload hSubmit with
            ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
              hOfferedValid, hReportsValid, rfl⟩
          exact ⟨appendMaterials_preserves_recordIntegrity s "surrebuttals" "defendant"
            (trimString rawText) offered reports hIntegrity
            (validateOfferedEvidenceBatch_ok _ _ _ _ hOfferedValid)
            (validateTechnicalReportBatch_ok _ _ _ hReportsValid),
            by simp [stateWithCase]⟩
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hEvidence] using hStepCore
            rcases submitEvidence_record_details s t action.actor_role action.payload hSubmit with
              ⟨item, hOrigin, hValid, rfl⟩
            exact ⟨appendSubmittedEvidence_preserves_recordIntegrity s item hIntegrity
              hOrigin (validateSubmittedEvidenceEntry_ok _ _ _ _ hValid),
              by simp [stateWithCase]⟩
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · rcases step_deliver_closing_statement_result
                s t action hClosing hStepCore with ⟨rawText, rfl⟩
              have hRecord := addFiling_preserves_record s.case "closings"
                (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                (trimString rawText)
              exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
                hRecord.1 hRecord.2.1 hRecord.2.2, by simp [stateWithCase]⟩
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_record_result
                  s t action hPass hStepCore with hResult | hResult
                · rw [hResult]
                  exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity rfl rfl rfl,
                    by simp [stateWithCase]⟩
                · rw [hResult]
                  exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity rfl rfl rfl,
                    by simp [stateWithCase]⟩
              · by_cases hAnswer : action.action_type = "submit_council_answer"
                · have hCore :
                      (do
                        requireRole action.actor_role "council"
                        let memberId := trimString (← getString action.payload "member_id")
                        let answer := (← getNat action.payload "answer")
                        let rationale := trimString (← getString action.payload "rationale")
                        recordCouncilAnswer s memberId answer rationale) = .ok t := by
                    simpa [stepCore, hAnswer] using hStepCore
                  cases hRole : requireRole action.actor_role "council" with
                  | error err =>
                      rw [hRole] at hCore
                      simp at hCore
                      cases hCore
                  | ok roleValue =>
                      cases roleValue
                      simp only [hRole, Bind.bind, Except.bind] at hCore
                      cases hMember : getString action.payload "member_id" with
                      | error err =>
                          rw [hMember] at hCore
                          cases hCore
                      | ok rawMemberId =>
                          simp only [hMember] at hCore
                          cases hAnswerValue : getNat action.payload "answer" with
                          | error err =>
                              rw [hAnswerValue] at hCore
                              cases hCore
                          | ok answer =>
                              simp only [hAnswerValue] at hCore
                              cases hRationale : getString action.payload "rationale" with
                              | error err =>
                                  rw [hRationale] at hCore
                                  cases hCore
                              | ok rawRationale =>
                                  simp only [hRationale] at hCore
                                  exact recordCouncilAnswer_preserves_recordIntegrity_and_catalog
                                    s t (trimString rawMemberId) answer
                                    (trimString rawRationale) hIntegrity hCore
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · have hCore :
                        (do
                          requireRole action.actor_role "system"
                          let memberId := trimString (← getString action.payload "member_id")
                          let status := (← getString action.payload "status")
                          removeCouncilMember s memberId status) = .ok t := by
                      simpa [stepCore, hRemoval] using hStepCore
                    cases hRole : requireRole action.actor_role "system" with
                    | error err =>
                        rw [hRole] at hCore
                        simp at hCore
                        cases hCore
                    | ok roleValue =>
                        cases roleValue
                        simp only [hRole, Bind.bind, Except.bind] at hCore
                        cases hMember : getString action.payload "member_id" with
                        | error err =>
                            rw [hMember] at hCore
                            cases hCore
                        | ok rawMemberId =>
                            simp only [hMember] at hCore
                            cases hStatus : getString action.payload "status" with
                            | error err =>
                                rw [hStatus] at hCore
                                cases hCore
                            | ok status =>
                                simp only [hStatus] at hCore
                                exact removeCouncilMember_preserves_recordIntegrity_and_catalog
                                  s t (trimString rawMemberId) status hIntegrity hCore
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hCore :
                          (do
                            requireRole action.actor_role "system"
                            failOpportunity s action.payload) = .ok t := by
                        simpa [stepCore, hFail] using hStepCore
                      cases hRole : requireRole action.actor_role "system" with
                      | error err =>
                          rw [hRole] at hCore
                          cases hCore
                      | ok roleValue =>
                          cases roleValue
                          simp only [hRole, Bind.bind, Except.bind] at hCore
                          exact failOpportunity_preserves_recordIntegrity_and_catalog
                            s t action.payload hIntegrity hCore
                    · simp [stepCore] at hStepCore

theorem step_preserves_recordIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hIntegrity : RecordIntegrity s)
    (hStep : step { state := s, action := action } = .ok t) :
    RecordIntegrity t :=
  (step_preserves_recordIntegrity_and_catalog s t action hIntegrity hStep).1

theorem reachable_recordIntegrity
    (s : ArbitrationState)
    (hReachable : Reachable s) :
    RecordIntegrity s := by
  induction hReachable with
  | init req s hInit => exact initializeCase_establishes_recordIntegrity req s hInit
  | step s t action _hReachable hStep ih =>
      exact step_preserves_recordIntegrity s t action ih hStep

theorem initialized_run_preserves_evidenceCatalog
    (req : InitializeCaseRequest)
    (start target : ArbitrationState)
    (hInit : initializeCase req = .ok start)
    (hRun : StepReachableFrom start target) :
    target.evidence_catalog = req.state.evidence_catalog := by
  have hStartCatalog := initializeCase_preserves_evidenceCatalog req start hInit
  induction hRun with
  | refl => exact hStartCatalog
  | step s t action _hReachable hStep ih =>
      have hSourceReachable : Reachable s := by
        exact stepReachableFrom_reachable start s
          (Reachable.init req start hInit) _hReachable
      have hSourceIntegrity := reachable_recordIntegrity s hSourceReachable
      exact (step_preserves_recordIntegrity_and_catalog
        s t action hSourceIntegrity hStep).2.trans ih

end ArbdProofs
