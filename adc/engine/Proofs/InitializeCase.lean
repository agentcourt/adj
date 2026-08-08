import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "filed",
    phase := "none"
  }

def baseState : CourtState :=
  { (default : CourtState) with
    court_name := "Test Court",
    case := baseCase,
    state_version := 7,
    passed_opportunities := ["o9", "o10"]
  }

def initializedState : CourtState :=
  { baseState with
    case := { baseCase with decision_traces := [{ action := "file_complaint", outcome := "filed", citations := ["FRCP 3"] }] }
  }

def oneAttachment : ComplaintAttachmentSeed :=
  { file_id := "file-1"
  , label := "confession"
  , original_name := "confession.txt"
  , storage_relpath := "input-files/01-confession.txt"
  , sha256 := "abc123"
  , size_bytes := 42
  }

def baseInitReq : InitializeCaseRequest :=
  { state := baseState
  , complaint_summary := "Complaint for breach of contract and misrepresentation."
  , filed_by := "plaintiff"
  , jury_demanded_on := "2026-01-05"
  , attachments := []
  }

def attachmentInitReq : InitializeCaseRequest :=
  { baseInitReq with attachments := [oneAttachment] }

def emptySummaryReq : InitializeCaseRequest :=
  { baseInitReq with complaint_summary := "   " }

def invalidFiledByReq : InitializeCaseRequest :=
  { baseInitReq with filed_by := "defendant" }

def alreadyInitializedReq : InitializeCaseRequest :=
  { baseInitReq with state := initializedState }

def initErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

def initStateVersion (r : Except String CourtState) : Nat :=
  match r with
  | .ok s => s.state_version
  | .error _ => 0

def initPassedOpportunities (r : Except String CourtState) : List String :=
  match r with
  | .ok s => s.passed_opportunities
  | .error _ => []

def initJuryDemandedOn (r : Except String CourtState) : String :=
  match r with
  | .ok s => s.case.jury_demanded_on
  | .error _ => ""

def initHasComplaintTrace (r : Except String CourtState) : Bool :=
  match r with
  | .ok s => hasDecisionTraceAction s.case "file_complaint"
  | .error _ => false

def initHasComplaintDocket (r : Except String CourtState) : Bool :=
  match r with
  | .ok s => hasDocketTitle s.case "Complaint filed"
  | .error _ => false

def initHasAttachmentFile (r : Except String CourtState) (fileId : String) : Bool :=
  match r with
  | .ok s => hasCaseFileId s.case fileId
  | .error _ => false

def initHasAttachmentDocket (r : Except String CourtState) : Bool :=
  match r with
  | .ok s => hasDocketTitle s.case "Complaint attachment filed"
  | .error _ => false

theorem initializeCase_requires_nonempty_summary :
    initErrorMessage (initializeCase emptySummaryReq) = "complaint_summary is required" := by
  native_decide

theorem initializeCase_requires_plaintiff_filed_by :
    initErrorMessage (initializeCase invalidFiledByReq) =
      "invalid filed_by for case initialization: defendant" := by
  native_decide

theorem initializeCase_rejects_already_initialized_case :
    initErrorMessage (initializeCase alreadyInitializedReq) = "case already initialized" := by
  native_decide

theorem initializeCase_success_records_core_complaint_effects :
    initStateVersion (initializeCase baseInitReq) = baseState.state_version + 1 ∧
      initPassedOpportunities (initializeCase baseInitReq) = [] ∧
      initJuryDemandedOn (initializeCase baseInitReq) = "2026-01-05" ∧
      initHasComplaintTrace (initializeCase baseInitReq) = true ∧
      initHasComplaintDocket (initializeCase baseInitReq) = true := by
  native_decide

theorem initializeCase_success_seeds_attachment_record :
    initHasAttachmentFile (initializeCase attachmentInitReq) "file-1" = true ∧
      initHasAttachmentDocket (initializeCase attachmentInitReq) = true := by
  native_decide
