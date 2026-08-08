import Proofs.MaximalRuns

namespace ArbProofs

def actorFacingAction (action : CourtAction) : Prop :=
  action.action_type ≠ "remove_council_member" ∧
    action.action_type ≠ "fail_opportunity"

theorem requireRole_ok_implies_trim
    (actual expected : String)
    (value : Unit)
    (hRole : requireRole actual expected = .ok value) :
    trimString actual = expected := by
  unfold requireRole at hRole
  by_cases hTrim : trimString actual = expected
  · exact hTrim
  · simp [hTrim] at hRole

theorem plaintiffThenDefendant_some_eq_if_isEmpty
    (xs : List Filing)
    (plaintiffLabel defendantLabel role : String)
    (hRole : plaintiffThenDefendant xs plaintiffLabel defendantLabel = some role) :
    role = if xs.isEmpty then plaintiffLabel else defendantLabel := by
  cases xs with
  | nil =>
      simp [plaintiffThenDefendant] at hRole
      cases hRole
      simp
  | cons first rest =>
      cases rest with
      | nil =>
          simp [plaintiffThenDefendant] at hRole
          cases hRole
          simp
      | cons second tail =>
          simp [plaintiffThenDefendant] at hRole

theorem step_record_opening_statement_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "openings" := by
  by_cases hPhase : s.case.phase = "openings"
  · exact hPhase
  · have hClosed : s.case.phase != "openings" := by simpa using hPhase
    simp [stepCore, hType, hClosed] at hStep
    cases hStep

theorem recordMeritsSubmission_source_phase
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (allowSupplementalMaterials : Bool)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit allowSupplementalMaterials payload = .ok t) :
    s.case.phase = phase := by
  by_cases hPhase : s.case.phase = phase
  · exact hPhase
  · have hClosed : s.case.phase != phase := by simpa using hPhase
    simp [recordMeritsSubmission, hClosed] at hSubmit
    cases hSubmit

theorem recordMeritsSubmission_actor_role
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (allowSupplementalMaterials : Bool)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit allowSupplementalMaterials payload = .ok t) :
    trimString actorRole = expectedRole := by
  have hPhase := recordMeritsSubmission_source_phase
    s t phase actorRole expectedRole textLabel limit allowSupplementalMaterials payload hSubmit
  have hCore :
      (do
        requireRole actorRole expectedRole
        let rawText ← getString payload "text"
        let text := trimString rawText
        requireTextWithinLimit textLabel text limit
        if allowSupplementalMaterials then
          let offered ← parseOfferedEvidence payload phase expectedRole
          let reports ← parseTechnicalReports payload phase expectedRole
          requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
          requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
          let totalOffered := offeredEvidenceCountForRole s.case.offered_evidence expectedRole + offered.length
          let totalReports := technicalReportCountForRole s.case.technical_reports expectedRole + reports.length
          requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
          requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
          let c1 := addFiling s.case phase expectedRole text
          let c2 := appendSupplementalMaterials c1 offered reports
          pure <| stateWithCase s c2
        else
          requireNoSupplementalMaterials payload
          pure <| stateWithCase s (addFiling s.case phase expectedRole text)) = .ok t := by
    simpa [recordMeritsSubmission, hPhase] using hSubmit
  cases hRole : requireRole actorRole expectedRole with
  | error err =>
      rw [hRole] at hCore
      cases hCore
  | ok okv =>
      exact requireRole_ok_implies_trim actorRole expectedRole okv hRole

theorem step_submit_argument_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "arguments" := by
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
  exact recordMeritsSubmission_source_phase s t "arguments" action.actor_role role
    "argument" s.policy.max_argument_chars true action.payload hSubmit

theorem step_submit_argument_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    trimString action.actor_role =
      (if s.case.arguments.isEmpty then "plaintiff" else "defendant") := by
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
  exact recordMeritsSubmission_actor_role s t "arguments" action.actor_role role
    "argument" s.policy.max_argument_chars true action.payload hSubmit

theorem step_record_opening_statement_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    trimString action.actor_role =
      (if s.case.openings.isEmpty then "plaintiff" else "defendant") := by
  have hPhase := step_record_opening_statement_source_phase s t action hType hStep
  let role := if s.case.openings.isEmpty then "plaintiff" else "defendant"
  have hCore :
      (do
        requireRole action.actor_role role
        let rawText ← getString action.payload "text"
        let text := trimString rawText
        requireTextWithinLimit "opening statement" text s.policy.max_opening_chars
        pure <| stateWithCase s (addFiling s.case "openings" role text)) = .ok t := by
    simpa [stepCore, hType, hPhase, role] using hStep
  cases hRole : requireRole action.actor_role role with
  | error err =>
      rw [hRole] at hCore
      cases hCore
  | ok okv =>
      exact requireRole_ok_implies_trim action.actor_role role okv hRole

theorem step_submit_rebuttal_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "rebuttals" := by
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
  exact recordMeritsSubmission_source_phase s t "rebuttals" action.actor_role "plaintiff"
    "rebuttal" s.policy.max_rebuttal_chars true action.payload hSubmit

theorem step_submit_rebuttal_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    trimString action.actor_role = "plaintiff" := by
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
  exact recordMeritsSubmission_actor_role s t "rebuttals" action.actor_role "plaintiff"
    "rebuttal" s.policy.max_rebuttal_chars true action.payload hSubmit

theorem step_submit_surrebuttal_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "surrebuttals" := by
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
  exact recordMeritsSubmission_source_phase s t "surrebuttals" action.actor_role "defendant"
    "surrebuttal" s.policy.max_surrebuttal_chars true action.payload hSubmit

theorem step_submit_surrebuttal_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    trimString action.actor_role = "defendant" := by
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
  exact recordMeritsSubmission_actor_role s t "surrebuttals" action.actor_role "defendant"
    "surrebuttal" s.policy.max_surrebuttal_chars true action.payload hSubmit

theorem step_deliver_closing_statement_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "closings" := by
  by_cases hPhase : s.case.phase = "closings"
  · exact hPhase
  · have hClosed : s.case.phase != "closings" := by simpa using hPhase
    simp [stepCore, hType, hClosed] at hStep
    cases hStep

theorem step_deliver_closing_statement_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    trimString action.actor_role =
      (if s.case.closings.isEmpty then "plaintiff" else "defendant") := by
  have hPhase := step_deliver_closing_statement_source_phase s t action hType hStep
  let role := if s.case.closings.isEmpty then "plaintiff" else "defendant"
  have hCore :
      (do
        requireRole action.actor_role role
        let rawText ← getString action.payload "text"
        let text := trimString rawText
        requireTextWithinLimit "closing statement" text s.policy.max_closing_chars
        requireNoSupplementalMaterials action.payload
        pure <| stateWithCase s (addFiling s.case "closings" role text)) = .ok t := by
    simpa [stepCore, hType, hPhase, role] using hStep
  cases hRole : requireRole action.actor_role role with
  | error err =>
      rw [hRole] at hCore
      cases hCore
  | ok okv =>
      exact requireRole_ok_implies_trim action.actor_role role okv hRole

theorem submitEvidence_source_phase
    (s t : ArbitrationState)
    (actorRole : String)
    (payload : Lean.Json)
    (hSubmit : submitEvidence s actorRole payload = .ok t) :
    s.case.phase = "arguments" ∨
      s.case.phase = "rebuttals" ∨
        s.case.phase = "surrebuttals" := by
  by_cases hArguments : s.case.phase = "arguments"
  · exact Or.inl hArguments
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · exact Or.inr (Or.inl hRebuttals)
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · exact Or.inr (Or.inr hSurrebuttals)
      · simp [submitEvidence] at hSubmit
        cases hSubmit

theorem submitEvidence_core_actor_role
    (s t : ArbitrationState)
    (actorRole expectedRole : String)
    (payload : Lean.Json)
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
    trimString actorRole = expectedRole := by
  cases hRole : requireRole actorRole expectedRole with
  | error err =>
      rw [hRole] at hCore
      cases hCore
  | ok okv =>
      exact requireRole_ok_implies_trim actorRole expectedRole okv hRole

theorem submitEvidence_actor_role
    (s t : ArbitrationState)
    (actorRole : String)
    (payload : Lean.Json)
    (hSubmit : submitEvidence s actorRole payload = .ok t) :
    (s.case.phase = "arguments" ∧
      trimString actorRole =
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")) ∨
      (s.case.phase = "rebuttals" ∧ trimString actorRole = "plaintiff") ∨
        (s.case.phase = "surrebuttals" ∧ trimString actorRole = "defendant") := by
  by_cases hArguments : s.case.phase = "arguments"
  · have hCore :
        (do
          requireRole actorRole (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          let parsedEvidence ← parseSubmittedEvidence payload s.case.phase
            (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
          let evidence :=
            { parsedEvidence with role := (if s.case.arguments.isEmpty then "plaintiff" else "defendant") }
          if s.case.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
            throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
          else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
            throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
          else
            let total := submittedEvidenceCountForRole s.case.submitted_evidence
              (if s.case.arguments.isEmpty then "plaintiff" else "defendant") + 1
            requireCountWithinLimit "submitted_evidence for this side" total
              s.policy.max_submitted_evidence_per_side
            pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
      simpa [submitEvidence, hArguments] using hSubmit
    exact Or.inl ⟨hArguments,
      submitEvidence_core_actor_role s t actorRole
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant") payload hCore⟩
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · cases hEmpty : s.case.rebuttals.isEmpty with
      | false =>
          simp [submitEvidence, hRebuttals, hEmpty] at hSubmit
          cases hSubmit
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
                  requireCountWithinLimit "submitted_evidence for this side" total
                    s.policy.max_submitted_evidence_per_side
                  pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
            simpa [submitEvidence, hArguments, hRebuttals, hEmpty] using hSubmit
          exact Or.inr (Or.inl ⟨hRebuttals,
            submitEvidence_core_actor_role s t actorRole "plaintiff" payload hCore⟩)
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · cases hEmpty : s.case.surrebuttals.isEmpty with
        | false =>
            simp [submitEvidence, hSurrebuttals, hEmpty] at hSubmit
            cases hSubmit
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
                    requireCountWithinLimit "submitted_evidence for this side" total
                      s.policy.max_submitted_evidence_per_side
                    pure <| stateWithCase s (appendSubmittedEvidence s.case evidence)) = .ok t := by
              simpa [submitEvidence, hArguments, hRebuttals, hSurrebuttals, hEmpty] using hSubmit
            exact Or.inr (Or.inr ⟨hSurrebuttals,
              submitEvidence_core_actor_role s t actorRole "defendant" payload hCore⟩)
      · simp [submitEvidence] at hSubmit
        cases hSubmit

theorem step_pass_phase_opportunity_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    (s.case.phase = "rebuttals" ∧ trimString action.actor_role = "plaintiff") ∨
      (s.case.phase = "surrebuttals" ∧ trimString action.actor_role = "defendant") := by
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
        cases hPass
    | ok okv =>
        exact Or.inl ⟨hRebuttals,
          requireRole_ok_implies_trim action.actor_role "plaintiff" okv hRole⟩
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
          cases hPass
      | ok okv =>
          exact Or.inr ⟨hSurrebuttals,
            requireRole_ok_implies_trim action.actor_role "defendant" okv hRole⟩
    · simp [stepCore, hType, hRebuttals, hSurrebuttals] at hStep

theorem step_submit_council_vote_actor_role
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    trimString action.actor_role = "council" := by
  have hCore :
      (do
        requireRole action.actor_role "council"
        let memberId := trimString (← getString action.payload "member_id")
        let vote := trimString (← getString action.payload "vote")
        let rationale := getOptionalString action.payload "rationale"
        recordCouncilVote s memberId vote rationale) = .ok t := by
    simpa [stepCore, hType] using hStep
  cases hRole : requireRole action.actor_role "council" with
  | error err =>
      rw [hRole] at hCore
      cases hCore
  | ok okv =>
      exact requireRole_ok_implies_trim action.actor_role "council" okv hRole

theorem nextOpportunity_role_openings
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "openings")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.role =
      (if s.case.openings.isEmpty then "plaintiff" else "defendant") := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.openings "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      exact plaintiffThenDefendant_some_eq_if_isEmpty
        s.case.openings "plaintiff" "defendant" role hRole

theorem nextOpportunity_role_arguments
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "arguments")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.role =
      (if s.case.arguments.isEmpty then "plaintiff" else "defendant") := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.arguments "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      exact plaintiffThenDefendant_some_eq_if_isEmpty
        s.case.arguments "plaintiff" "defendant" role hRole

theorem nextOpportunity_role_rebuttals
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "rebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.role = "plaintiff" := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hEmpty : s.case.rebuttals.isEmpty with
  | false =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
  | true =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
      cases hNext
      simp

theorem nextOpportunity_role_surrebuttals
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.role = "defendant" := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hEmpty : s.case.surrebuttals.isEmpty with
  | false =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
  | true =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
      cases hNext
      simp

theorem nextOpportunity_role_closings
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "closings")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.role =
      (if s.case.closings.isEmpty then "plaintiff" else "defendant") := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.closings "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      exact plaintiffThenDefendant_some_eq_if_isEmpty
        s.case.closings "plaintiff" "defendant" role hRole

theorem nextOpportunity_role_deliberation
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "deliberation")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    opportunity.role = "council" := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hMember : nextCouncilMember? s.case with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hMember] at hNext
  | some member =>
      simp [nextOpportunityForPhase, hPhase, hMember] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_record_opening_statement
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "openings")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "record_opening_statement" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.openings "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_submit_argument
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "arguments")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_argument" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.arguments "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_submit_evidence
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "arguments" ∨
      s.case.phase = "rebuttals" ∨
        s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_evidence" ∈ opportunity.allowed_tools := by
  rcases hPhase with hArguments | hRest
  · unfold nextOpportunity at hNext
    simp [hStatus] at hNext
    cases hRole : plaintiffThenDefendant s.case.arguments "plaintiff" "defendant" with
    | none =>
        simp [nextOpportunityForPhase, hArguments, hRole] at hNext
    | some role =>
        simp [nextOpportunityForPhase, hArguments, hRole] at hNext
        cases hNext
        simp
  · rcases hRest with hRebuttals | hSurrebuttals
    · unfold nextOpportunity at hNext
      simp [hStatus] at hNext
      cases hEmpty : s.case.rebuttals.isEmpty with
      | false =>
          simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
      | true =>
          simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
          cases hNext
          simp
    · unfold nextOpportunity at hNext
      simp [hStatus] at hNext
      cases hEmpty : s.case.surrebuttals.isEmpty with
      | false =>
          simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
      | true =>
          simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
          cases hNext
          simp

theorem nextOpportunity_allows_submit_rebuttal
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "rebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_rebuttal" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hEmpty : s.case.rebuttals.isEmpty with
  | false =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
  | true =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_submit_surrebuttal
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_surrebuttal" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hEmpty : s.case.surrebuttals.isEmpty with
  | false =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
  | true =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_deliver_closing_statement
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "closings")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "deliver_closing_statement" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.closings "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_pass_phase_opportunity
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "rebuttals" ∨ s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "pass_phase_opportunity" ∈ opportunity.allowed_tools := by
  rcases hPhase with hRebuttals | hSurrebuttals
  · unfold nextOpportunity at hNext
    simp [hStatus] at hNext
    cases hEmpty : s.case.rebuttals.isEmpty with
    | false =>
        simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
    | true =>
        simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
        cases hNext
        simp
  · unfold nextOpportunity at hNext
    simp [hStatus] at hNext
    cases hEmpty : s.case.surrebuttals.isEmpty with
    | false =>
        simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
    | true =>
        simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
        cases hNext
        simp

theorem nextOpportunity_allows_submit_council_vote
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "deliberation")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_council_vote" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hMember : nextCouncilMember? s.case with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hMember] at hNext
  | some member =>
      simp [nextOpportunityForPhase, hPhase, hMember] at hNext
      cases hNext
      simp

theorem reachable_actor_step_allowed_by_nextOpportunity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hs : Reachable s)
    (hStatus : s.case.status = "active")
    (hActorFacing : actorFacingAction action)
    (hStep : step { state := s, action := action } = .ok t) :
    ∃ opportunity,
      (nextOpportunity s).opportunity = some opportunity ∧
        action.action_type ∈ opportunity.allowed_tools := by
  rcases reachable_active_has_nextOpportunity s hs hStatus with ⟨_hLive, hSome⟩
  rcases hSome with ⟨opportunity, hNext⟩
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · have hPhase := step_record_opening_statement_source_phase s t action hOpening hStepCore
    exact ⟨opportunity, hNext, by
      simpa [hOpening] using
        nextOpportunity_allows_record_opening_statement s opportunity hStatus hPhase hNext⟩
  · by_cases hArgument : action.action_type = "submit_argument"
    · have hPhase := step_submit_argument_source_phase s t action hArgument hStepCore
      exact ⟨opportunity, hNext, by
        simpa [hArgument] using
          nextOpportunity_allows_submit_argument s opportunity hStatus hPhase hNext⟩
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hPhase := step_submit_rebuttal_source_phase s t action hRebuttal hStepCore
        exact ⟨opportunity, hNext, by
          simpa [hRebuttal] using
            nextOpportunity_allows_submit_rebuttal s opportunity hStatus hPhase hNext⟩
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hPhase := step_submit_surrebuttal_source_phase s t action hSurrebuttal hStepCore
          exact ⟨opportunity, hNext, by
            simpa [hSurrebuttal] using
              nextOpportunity_allows_submit_surrebuttal s opportunity hStatus hPhase hNext⟩
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hEvidence]
                using hStepCore
            have hPhase := submitEvidence_source_phase s t action.actor_role action.payload hSubmit
            exact ⟨opportunity, hNext, by
              simpa [hEvidence] using
                nextOpportunity_allows_submit_evidence s opportunity hStatus hPhase hNext⟩
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · have hPhase := step_deliver_closing_statement_source_phase s t action hClosing hStepCore
              exact ⟨opportunity, hNext, by
                simpa [hClosing] using
                  nextOpportunity_allows_deliver_closing_statement s opportunity hStatus hPhase hNext⟩
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_result s t action hPass hStepCore with hResult | hResult
                · exact ⟨opportunity, hNext, by
                    simpa [hPass] using
                      nextOpportunity_allows_pass_phase_opportunity s opportunity hStatus
                        (Or.inl hResult.1) hNext⟩
                · exact ⟨opportunity, hNext, by
                    simpa [hPass] using
                      nextOpportunity_allows_pass_phase_opportunity s opportunity hStatus
                        (Or.inr hResult.1) hNext⟩
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨_memberId, _vote, _rationale, hPhase, _hCont⟩
                  exact ⟨opportunity, hNext, by
                    simpa [hVote] using
                      nextOpportunity_allows_submit_council_vote s opportunity hStatus hPhase hNext⟩
                · by_cases hRemove : action.action_type = "remove_council_member"
                  · exact False.elim (hActorFacing.1 hRemove)
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact False.elim (hActorFacing.2 hFail)
                    · simp [stepCore] at hStepCore

theorem reachable_actor_step_role_matches_nextOpportunity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hs : Reachable s)
    (hStatus : s.case.status = "active")
    (hActorFacing : actorFacingAction action)
    (hStep : step { state := s, action := action } = .ok t) :
    ∃ opportunity,
      (nextOpportunity s).opportunity = some opportunity ∧
        trimString action.actor_role = opportunity.role := by
  rcases reachable_active_has_nextOpportunity s hs hStatus with ⟨_hLive, hSome⟩
  rcases hSome with ⟨opportunity, hNext⟩
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · have hPhase := step_record_opening_statement_source_phase s t action hOpening hStepCore
    have hActor := step_record_opening_statement_actor_role s t action hOpening hStepCore
    have hOpportunity := nextOpportunity_role_openings s opportunity hStatus hPhase hNext
    exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
  · by_cases hArgument : action.action_type = "submit_argument"
    · have hPhase := step_submit_argument_source_phase s t action hArgument hStepCore
      have hActor := step_submit_argument_actor_role s t action hArgument hStepCore
      have hOpportunity := nextOpportunity_role_arguments s opportunity hStatus hPhase hNext
      exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hPhase := step_submit_rebuttal_source_phase s t action hRebuttal hStepCore
        have hActor := step_submit_rebuttal_actor_role s t action hRebuttal hStepCore
        have hOpportunity := nextOpportunity_role_rebuttals s opportunity hStatus hPhase hNext
        exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hPhase := step_submit_surrebuttal_source_phase s t action hSurrebuttal hStepCore
          have hActor := step_submit_surrebuttal_actor_role s t action hSurrebuttal hStepCore
          have hOpportunity := nextOpportunity_role_surrebuttals s opportunity hStatus hPhase hNext
          exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hEvidence]
                using hStepCore
            rcases submitEvidence_actor_role s t action.actor_role action.payload hSubmit with
              hArgumentRole | hRest
            · rcases hArgumentRole with ⟨hPhase, hActor⟩
              have hOpportunity := nextOpportunity_role_arguments s opportunity hStatus hPhase hNext
              exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
            · rcases hRest with hRebuttalRole | hSurrebuttalRole
              · rcases hRebuttalRole with ⟨hPhase, hActor⟩
                have hOpportunity := nextOpportunity_role_rebuttals s opportunity hStatus hPhase hNext
                exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
              · rcases hSurrebuttalRole with ⟨hPhase, hActor⟩
                have hOpportunity := nextOpportunity_role_surrebuttals s opportunity hStatus hPhase hNext
                exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · have hPhase := step_deliver_closing_statement_source_phase s t action hClosing hStepCore
              have hActor := step_deliver_closing_statement_actor_role s t action hClosing hStepCore
              have hOpportunity := nextOpportunity_role_closings s opportunity hStatus hPhase hNext
              exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_actor_role s t action hPass hStepCore with
                  hRebuttalPass | hSurrebuttalPass
                · rcases hRebuttalPass with ⟨hPhase, hActor⟩
                  have hOpportunity := nextOpportunity_role_rebuttals s opportunity hStatus hPhase hNext
                  exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
                · rcases hSurrebuttalPass with ⟨hPhase, hActor⟩
                  have hOpportunity := nextOpportunity_role_surrebuttals s opportunity hStatus hPhase hNext
                  exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
              · by_cases hVote : action.action_type = "submit_council_vote"
                · have hActor := step_submit_council_vote_actor_role s t action hVote hStepCore
                  rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨_memberId, _vote, _rationale, hPhase, _hCont⟩
                  have hOpportunity := nextOpportunity_role_deliberation s opportunity hStatus hPhase hNext
                  exact ⟨opportunity, hNext, hActor.trans hOpportunity.symm⟩
                · by_cases hRemove : action.action_type = "remove_council_member"
                  · exact False.elim (hActorFacing.1 hRemove)
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact False.elim (hActorFacing.2 hFail)
                    · simp [stepCore] at hStepCore

theorem reachable_actor_step_matches_nextOpportunity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hs : Reachable s)
    (hStatus : s.case.status = "active")
    (hActorFacing : actorFacingAction action)
    (hStep : step { state := s, action := action } = .ok t) :
    ∃ opportunity,
      (nextOpportunity s).opportunity = some opportunity ∧
        trimString action.actor_role = opportunity.role ∧
          action.action_type ∈ opportunity.allowed_tools := by
  rcases reachable_actor_step_role_matches_nextOpportunity
      s t action hs hStatus hActorFacing hStep with
    ⟨roleOpportunity, hRoleNext, hRole⟩
  rcases reachable_actor_step_allowed_by_nextOpportunity
      s t action hs hStatus hActorFacing hStep with
    ⟨toolOpportunity, hToolNext, hAllowed⟩
  rw [hRoleNext] at hToolNext
  cases hToolNext
  exact ⟨roleOpportunity, hRoleNext, hRole, hAllowed⟩

end ArbProofs
