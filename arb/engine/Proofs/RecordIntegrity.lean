import Proofs.RecordProvenance
import Proofs.CouncilIntegrity

namespace ArbProofs

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

/--
`MeritsOffersUsePriorRecord` states the temporal reference property for one
action.  Each merits payload parses to an offered-evidence list whose IDs
resolve within the catalog or submitted-evidence list in that action's source
state, subject to the source state's exhibit-size limit.  Its three fields
cover every action type that accepts offered evidence.
-/
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

theorem evidenceCatalogValid_ids_nodup
    (catalog : List EvidenceCommitment)
    (hValid : EvidenceCatalogValid catalog) :
    (catalog.map (fun item => item.evidence_id)).Nodup := by
  by_cases hNodup : (catalog.map (fun item => item.evidence_id)).Nodup
  · exact hNodup
  · have hDuplicate :
        hasDuplicateStrings (catalog.map (fun item => item.evidence_id)) = true :=
      hasDuplicateStrings_eq_true_of_not_nodup hNodup
    have hNoDuplicate :
        hasDuplicateStrings (catalog.map (fun item => item.evidence_id)) = false := by
      simpa [hasDuplicateEvidenceCommitmentIds] using hValid.1
    rw [hDuplicate] at hNoDuplicate
    cases hNoDuplicate

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

theorem submittedEvidenceEntryValid_prior_fresh
    (catalog : List EvidenceCommitment)
    (prior : List SubmittedEvidence)
    (maxBytes : Nat)
    (item : SubmittedEvidence)
    (hEntry : submittedEvidenceEntryValid catalog prior maxBytes item = true) :
    prior.any (fun previous => previous.evidence_id = item.evidence_id) = false := by
  simp only [submittedEvidenceEntryValid, Bool.and_eq_true] at hEntry
  have hFresh :
      Bool.not (prior.any (fun previous =>
        previous.evidence_id = item.evidence_id)) = true :=
    hEntry.1.2
  cases hAny : prior.any (fun previous =>
      previous.evidence_id = item.evidence_id) with
  | false => rfl
  | true =>
      rw [hAny] at hFresh
      cases hFresh

theorem submittedEvidenceEntryValid_catalog_fresh
    (catalog : List EvidenceCommitment)
    (prior : List SubmittedEvidence)
    (maxBytes : Nat)
    (item : SubmittedEvidence)
    (hEntry : submittedEvidenceEntryValid catalog prior maxBytes item = true) :
    catalog.any (fun commitment => commitment.evidence_id = item.evidence_id) = false := by
  simp only [submittedEvidenceEntryValid, Bool.and_eq_true] at hEntry
  have hFresh :
      Bool.not (catalog.any (fun commitment =>
        commitment.evidence_id = item.evidence_id)) = true :=
    hEntry.1.1.2
  cases hAny : catalog.any (fun commitment =>
      commitment.evidence_id = item.evidence_id) with
  | false => rfl
  | true =>
      rw [hAny] at hFresh
      cases hFresh

theorem submittedEvidenceHistoryValid_ids_nodup
    (catalog : List EvidenceCommitment)
    (maxBytes : Nat)
    (items : List SubmittedEvidence)
    (hHistory : SubmittedEvidenceHistoryValid catalog maxBytes items) :
    (items.map (fun item => item.evidence_id)).Nodup := by
  induction hHistory with
  | nil => simp
  | snoc prior item _history _origin entry ih =>
      have hFreshAny := submittedEvidenceEntryValid_prior_fresh
        catalog prior maxBytes item entry
      have hFresh : item.evidence_id ∉ prior.map (fun previous => previous.evidence_id) := by
        intro hMember
        rcases List.mem_map.mp hMember with ⟨previous, hPrevious, hId⟩
        have hAbsent := List.any_eq_false.mp hFreshAny previous hPrevious
        exact hAbsent (by simpa using hId)
      rw [List.map_concat, List.concat_eq_append, List.nodup_append]
      refine ⟨ih, by simp, ?_⟩
      intro priorId hPriorId itemId hItemId
      simp at hItemId
      rcases hItemId with rfl
      intro hEqual
      exact hFresh (hEqual ▸ hPriorId)

theorem submittedEvidenceHistoryValid_disjoint_catalog
    (catalog : List EvidenceCommitment)
    (maxBytes : Nat)
    (items : List SubmittedEvidence)
    (hHistory : SubmittedEvidenceHistoryValid catalog maxBytes items) :
    ∀ submitted ∈ items, ∀ commitment ∈ catalog,
      submitted.evidence_id ≠ commitment.evidence_id := by
  induction hHistory with
  | nil => simp
  | snoc prior item _history _origin entry ih =>
      intro submitted hSubmitted commitment hCommitment
      have hSubmitted' : submitted ∈ prior ++ [item] := by
        simpa [List.concat_eq_append] using hSubmitted
      rcases List.mem_append.mp hSubmitted' with hPrior | hItem
      · exact ih submitted hPrior commitment hCommitment
      · have hFreshAny := submittedEvidenceEntryValid_catalog_fresh
          catalog prior maxBytes item entry
        simp at hItem
        rcases hItem with rfl
        have hAbsent := List.any_eq_false.mp hFreshAny commitment hCommitment
        intro hEqual
        exact hAbsent (by simpa using hEqual.symm)

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

theorem advanceAfterMerits_preserves_submitted_record
    (c : ArbitrationCase) :
    (advanceAfterMerits c).submitted_evidence = c.submitted_evidence := by
  unfold advanceAfterMerits
  split
  · rfl
  · split
    · rfl
    · split
      · rfl
      · split
        · rfl
        · split <;> rfl

theorem addFiling_preserves_submitted_record
    (c : ArbitrationCase)
    (phase role text : String) :
    (addFiling c phase role text).submitted_evidence = c.submitted_evidence := by
  unfold addFiling
  split <;> simp [advanceAfterMerits_preserves_submitted_record]

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
  let c1 := addFiling s.case phase role text
  have hSubmitted1 : c1.submitted_evidence = s.case.submitted_evidence :=
    addFiling_preserves_submitted_record s.case phase role text
  have hOffered1 : c1.offered_evidence = s.case.offered_evidence :=
    addFiling_preserves_offered_evidence s.case phase role text
  have hReports1 : c1.technical_reports = s.case.technical_reports :=
    addFiling_preserves_technical_reports s.case phase role text
  exact
    { catalog := by simpa [stateWithCase] using hIntegrity.catalog
      submitted := by
        simpa [stateWithCase, appendSupplementalMaterials, c1, hSubmitted1] using
          hIntegrity.submitted
      offered := by
        simpa [stateWithCase, appendSupplementalMaterials, c1, hSubmitted1,
          hOffered1] using
          offeredEvidenceBatchValid_append
            s.evidence_catalog s.case.submitted_evidence s.policy.max_exhibit_bytes
            s.case.offered_evidence offered hIntegrity.offered hOffered
      reports := by
        simpa [stateWithCase, appendSupplementalMaterials, c1, hReports1] using
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
      by_cases hProposition : trimString req.proposition = ""
      · simp [hPolicy, hProposition, Bind.bind, Except.bind] at hInit
      · by_cases hEvidence : trimString req.state.policy.evidence_standard = ""
        · simp [hPolicy, hProposition, hEvidence, Bind.bind, Except.bind] at hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hProposition, hEvidence, hEmpty, Bind.bind,
              Except.bind] at hInit
          · by_cases hLength :
                req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, Bind.bind,
                Except.bind] at hInit
            · by_cases hInvalid : hasInvalidCouncilMemberIds req.council_members
              · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                  Bind.bind, Except.bind] at hInit
              · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
                · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                    hDuplicate, Bind.bind, Except.bind] at hInit
                · cases hCatalog : validateEvidenceCatalog req.state.evidence_catalog with
                  | error err =>
                      simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
                        hDuplicate, hCatalog, Bind.bind, Except.bind] at hInit
                  | ok okv =>
                      cases okv
                      simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hInvalid,
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
  by_cases hRoundComplete :
      (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hContinue
        cases hContinue
        exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
          (by simpa using hSubmitted) (by simpa using hOffered) (by simpa using hReports),
          by simp [stateWithCase]⟩
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hContinue
          cases hContinue
          exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
            (by simpa using hSubmitted) (by simpa using hOffered) (by simpa using hReports),
            by simp [stateWithCase]⟩
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hContinue
            cases hContinue
            exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
              (by simpa using hSubmitted) (by simpa using hOffered) (by simpa using hReports),
              by simp [stateWithCase]⟩
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hContinue
            cases hContinue
            exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
              (by simpa using hSubmitted) (by simpa using hOffered) (by simpa using hReports),
              by simp [stateWithCase]⟩
  · simp [hRoundComplete] at hContinue
    cases hContinue
    exact ⟨stateWithCase_preserves_recordIntegrity s c hIntegrity
      hSubmitted hOffered hReports, by simp [stateWithCase]⟩

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
        hValid, _hReportsValid, _hOfferedCap, _hReportsCap, _hResult⟩
    exact ⟨offered, hParse, validateOfferedEvidenceBatch_ok _ _ _ _ hValid⟩
  · intro hType
    have hSubmit : recordMeritsSubmission s "rebuttals" action.actor_role
        "plaintiff" "rebuttal" s.policy.max_rebuttal_chars true action.payload = .ok t := by
      simpa [stepCore, hType] using hStepCore
    rcases recordMeritsSubmission_with_materials_record_details
        s t "rebuttals" action.actor_role "plaintiff" "rebuttal"
        s.policy.max_rebuttal_chars action.payload hSubmit with
      ⟨_rawText, offered, _reports, hParse, _hReportsParse,
        hValid, _hReportsValid, _hOfferedCap, _hReportsCap, _hResult⟩
    exact ⟨offered, hParse, validateOfferedEvidenceBatch_ok _ _ _ _ hValid⟩
  · intro hType
    have hSubmit : recordMeritsSubmission s "surrebuttals" action.actor_role
        "defendant" "surrebuttal" s.policy.max_surrebuttal_chars true action.payload = .ok t := by
      simpa [stepCore, hType] using hStepCore
    rcases recordMeritsSubmission_with_materials_record_details
        s t "surrebuttals" action.actor_role "defendant" "surrebuttal"
        s.policy.max_surrebuttal_chars action.payload hSubmit with
      ⟨_rawText, offered, _reports, hParse, _hReportsParse,
        hValid, _hReportsValid, _hOfferedCap, _hReportsCap, _hResult⟩
    exact ⟨offered, hParse, validateOfferedEvidenceBatch_ok _ _ _ _ hValid⟩

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
    exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
      (addFiling_preserves_submitted_record s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
      (addFiling_preserves_offered_evidence s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText))
      (addFiling_preserves_technical_reports s.case "openings"
        (if s.case.openings.isEmpty then "plaintiff" else "defendant")
        (trimString rawText)), by simp [stateWithCase]⟩
  · by_cases hArgument : action.action_type = "submit_argument"
    · let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
      have hSubmit : recordMeritsSubmission s "arguments" action.actor_role role
          "argument" s.policy.max_argument_chars true action.payload = .ok t := by
        simpa [stepCore, hArgument, role] using hStepCore
      rcases recordMeritsSubmission_with_materials_record_details
          s t "arguments" action.actor_role role "argument"
          s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, _hOfferedParse, _hReportsParse,
          hOfferedValid, hReportsValid, _hOfferedCap, _hReportsCap, rfl⟩
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
            hOfferedValid, hReportsValid, _hOfferedCap, _hReportsCap, rfl⟩
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
              hOfferedValid, hReportsValid, _hOfferedCap, _hReportsCap, rfl⟩
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
              (by simpa [materialOriginAllowed] using hOrigin)
              (validateSubmittedEvidenceEntry_ok _ _ _ _ hValid),
              by simp [stateWithCase]⟩
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with
                ⟨rawText, rfl⟩
              exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity
                (addFiling_preserves_submitted_record s.case "closings"
                  (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                  (trimString rawText))
                (addFiling_preserves_offered_evidence s.case "closings"
                  (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                  (trimString rawText))
                (addFiling_preserves_technical_reports s.case "closings"
                  (if s.case.closings.isEmpty then "plaintiff" else "defendant")
                  (trimString rawText)), by simp [stateWithCase]⟩
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_record_result
                  s t action hPass hStepCore with hResult | hResult
                · rw [hResult]
                  exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity rfl rfl rfl,
                    by simp [stateWithCase]⟩
                · rw [hResult]
                  exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity rfl rfl rfl,
                    by simp [stateWithCase]⟩
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, _hPhase, hContinue⟩
                  exact continueDeliberation_preserves_recordIntegrity_and_catalog s t _
                    hIntegrity (by rfl) (by rfl) (by rfl) hContinue
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemoval hStepCore with
                      ⟨memberId, status, _hPhase, hContinue⟩
                    exact continueDeliberation_preserves_recordIntegrity_and_catalog s t _
                      hIntegrity (by rfl) (by rfl) (by rfl) hContinue
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailCore : failOpportunity s action.payload = .ok t := by
                        have hCore :
                            (do
                              requireRole action.actor_role "system"
                              failOpportunity s action.payload) = .ok t := by
                          simpa [stepCore, hFail] using hStepCore
                        cases hRole : requireRole action.actor_role "system" with
                        | error err =>
                            rw [hRole] at hCore
                            cases hCore
                        | ok okv =>
                            cases okv
                            rw [hRole] at hCore
                            simpa [SeqRight.seqRight, Bind.bind, Except.bind] using hCore
                      rcases failOpportunity_result s t action.payload hFailCore with hCouncil | hParty
                      · rcases hCouncil with
                          ⟨_memberId, _reason, _opportunityId, _message, c1, hC1,
                            _hPhase, _hSeated, _hFresh, hContinue⟩
                        exact continueDeliberation_preserves_recordIntegrity_and_catalog s t c1
                          hIntegrity (by rw [hC1]) (by rw [hC1]) (by rw [hC1]) hContinue
                      · rcases hParty with ⟨failure, rfl, _hNotClosed, _hNotDeliberation,
                          _hFailureType, _hRole, _hPhase⟩
                        exact ⟨stateWithCase_preserves_recordIntegrity s _ hIntegrity rfl rfl rfl,
                          by simp [stateWithCase]⟩
                    · simp [stepCore] at hStepCore

theorem step_preserves_recordIntegrity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hIntegrity : RecordIntegrity s)
    (hStep : step { state := s, action := action } = .ok t) :
    RecordIntegrity t :=
  (step_preserves_recordIntegrity_and_catalog s t action hIntegrity hStep).1

theorem continueDeliberation_preserves_evidenceCatalog
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hContinue : continueDeliberation s c = .ok t) :
    t.evidence_catalog = s.evidence_catalog := by
  unfold continueDeliberation at hContinue
  by_cases hRoundComplete :
      (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hResolution : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hResolution] at hContinue
        cases hContinue
        simp [stateWithCase]
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hResolution, hTooFew] at hContinue
          cases hContinue
          simp [stateWithCase]
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hContinue
            cases hContinue
            simp [stateWithCase]
          · simp [hRoundComplete, hResolution, hTooFew, hLastRound] at hContinue
            cases hContinue
            simp [stateWithCase]
  · simp [hRoundComplete] at hContinue
    cases hContinue
    simp [stateWithCase]

theorem step_preserves_evidenceCatalog
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    t.evidence_catalog = s.evidence_catalog := by
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · rcases step_record_opening_statement_result s t action hOpening hStepCore with
      ⟨rawText, rfl⟩
    simp [stateWithCase]
  · by_cases hArgument : action.action_type = "submit_argument"
    · let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
      have hSubmit : recordMeritsSubmission s "arguments" action.actor_role role
          "argument" s.policy.max_argument_chars true action.payload = .ok t := by
        simpa [stepCore, hArgument, role] using hStepCore
      rcases recordMeritsSubmission_with_materials_result s t "arguments"
          action.actor_role role "argument" s.policy.max_argument_chars action.payload hSubmit with
        ⟨rawText, offered, reports, rfl⟩
      simp [stateWithCase]
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hSubmit : recordMeritsSubmission s "rebuttals" action.actor_role
            "plaintiff" "rebuttal" s.policy.max_rebuttal_chars true action.payload = .ok t := by
          simpa [stepCore, hRebuttal] using hStepCore
        rcases recordMeritsSubmission_with_materials_result s t "rebuttals"
            action.actor_role "plaintiff" "rebuttal" s.policy.max_rebuttal_chars
            action.payload hSubmit with ⟨rawText, offered, reports, rfl⟩
        simp [stateWithCase]
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hSubmit : recordMeritsSubmission s "surrebuttals" action.actor_role
              "defendant" "surrebuttal" s.policy.max_surrebuttal_chars true action.payload = .ok t := by
            simpa [stepCore, hSurrebuttal] using hStepCore
          rcases recordMeritsSubmission_with_materials_result s t "surrebuttals"
              action.actor_role "defendant" "surrebuttal" s.policy.max_surrebuttal_chars
              action.payload hSubmit with ⟨rawText, offered, reports, rfl⟩
          simp [stateWithCase]
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hEvidence] using hStepCore
            rcases submitEvidence_result s t action.actor_role action.payload hSubmit with
              ⟨item, rfl⟩
            simp [stateWithCase]
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · rcases step_deliver_closing_statement_result s t action hClosing hStepCore with
                ⟨rawText, rfl⟩
              simp [stateWithCase]
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_record_result s t action hPass hStepCore with
                  hResult | hResult <;> rw [hResult] <;> simp [stateWithCase]
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨memberId, vote, rationale, _hPhase, hContinue⟩
                  exact continueDeliberation_preserves_evidenceCatalog s t _ hContinue
                · by_cases hRemoval : action.action_type = "remove_council_member"
                  · rcases step_remove_council_member_result s t action hRemoval hStepCore with
                      ⟨memberId, status, _hPhase, hContinue⟩
                    exact continueDeliberation_preserves_evidenceCatalog s t _ hContinue
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · have hFailCore : failOpportunity s action.payload = .ok t := by
                        have hCore :
                            (do
                              requireRole action.actor_role "system"
                              failOpportunity s action.payload) = .ok t := by
                          simpa [stepCore, hFail] using hStepCore
                        cases hRole : requireRole action.actor_role "system" with
                        | error err =>
                            rw [hRole] at hCore
                            cases hCore
                        | ok okv =>
                            cases okv
                            rw [hRole] at hCore
                            simpa [SeqRight.seqRight, Bind.bind, Except.bind] using hCore
                      rcases failOpportunity_result s t action.payload hFailCore with hCouncil | hParty
                      · rcases hCouncil with
                          ⟨_memberId, _reason, _opportunityId, _message, c1, _hC1,
                            _hPhase, _hSeated, _hFresh, hContinue⟩
                        exact continueDeliberation_preserves_evidenceCatalog s t c1 hContinue
                      · rcases hParty with ⟨failure, rfl, _hNotClosed, _hNotDeliberation,
                          _hFailureType, _hRole, _hPhase⟩
                        simp [stateWithCase]
                    · simp [stepCore] at hStepCore

theorem reachable_recordIntegrity
    (s : ArbitrationState)
    (hReachable : Reachable s) :
    RecordIntegrity s := by
  induction hReachable with
  | init req s hInit => exact initializeCase_establishes_recordIntegrity req s hInit
  | step s t action _hReachable hStep ih =>
      exact step_preserves_recordIntegrity s t action ih hStep

theorem stepReachableFrom_preserves_evidenceCatalog
    (start s : ArbitrationState)
    (hRun : StepReachableFrom start s) :
    s.evidence_catalog = start.evidence_catalog := by
  induction hRun with
  | refl => rfl
  | step u v action _hu hStep ih =>
      exact (step_preserves_evidenceCatalog u v action hStep).trans ih

theorem initialized_run_preserves_evidenceCatalog
    (req : InitializeCaseRequest)
    (start s : ArbitrationState)
    (hInit : initializeCase req = .ok start)
    (hRun : StepReachableFrom start s) :
    s.evidence_catalog = req.state.evidence_catalog := by
  have hStart := initializeCase_preserves_evidenceCatalog req start hInit
  exact (stepReachableFrom_preserves_evidenceCatalog start s hRun).trans hStart

end ArbProofs
