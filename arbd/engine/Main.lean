import Lean

open Lean

structure CouncilMember where
  member_id : String
  model : String := ""
  persona_filename : String := ""
  status : String := "seated"
  failure_reason : String := ""
  failure_opportunity_id : String := ""
  failure_message : String := ""
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure Filing where
  phase : String
  role : String
  text : String
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure OfferedEvidence where
  phase : String
  role : String
  evidence_id : String
  label : String := ""
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure TechnicalReport where
  phase : String
  role : String
  title : String
  summary : String
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure SubmittedEvidence where
  phase : String
  role : String
  evidence_id : String
  title : String
  source_url : String := ""
  source_description : String := ""
  mime_type : String := ""
  retrieval_timestamp : String := ""
  relevance : String := ""
  sha256 : String := ""
  size_bytes : Nat := 0
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure CouncilAnswer where
  member_id : String
  round : Nat
  answer : Nat
  rationale : String := ""
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure OpportunityFailure where
  failure_type : String := "opportunity_failed"
  role : String
  phase : String
  opportunity_id : String
  reason : String
  message : String := ""
  member_id : String := ""
  model : String := ""
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure ArbitrationCase where
  case_id : String
  caption : String
  question : String := ""
  status : String := "draft"
  phase : String := "draft"
  council_members : List CouncilMember := []
  openings : List Filing := []
  arguments : List Filing := []
  rebuttals : List Filing := []
  surrebuttals : List Filing := []
  closings : List Filing := []
  offered_evidence : List OfferedEvidence := []
  technical_reports : List TechnicalReport := []
  submitted_evidence : List SubmittedEvidence := []
  deliberation_round : Nat := 1
  council_answers : List CouncilAnswer := []
  failure : Option OpportunityFailure := none
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure ArbitrationPolicy where
  council_size : Nat := 5
  judgment_standard : String := ""
  max_opening_chars : Nat := 4000
  max_argument_chars : Nat := 6000
  max_rebuttal_chars : Nat := 4000
  max_surrebuttal_chars : Nat := 4000
  max_closing_chars : Nat := 5000
  max_exhibits_per_filing : Nat := 9
  max_exhibits_per_side : Nat := 12
  max_exhibit_bytes : Nat := 131072
  max_reports_per_filing : Nat := 3
  max_reports_per_side : Nat := 4
  max_report_title_bytes : Nat := 256
  max_report_summary_bytes : Nat := 8192
  max_submitted_evidence_per_side : Nat := 8
  max_submitted_evidence_bytes : Nat := 131072
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure ArbitrationState where
  schema_version : String := "v1"
  forum_name : String
  case : ArbitrationCase
  policy : ArbitrationPolicy := {}
  state_version : Nat := 0
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure CourtAction where
  action_type : String
  actor_role : String
  payload : Json
  deriving Inhabited, ToJson, FromJson

structure StepRequest where
  state : ArbitrationState
  action : CourtAction
  deriving Inhabited, ToJson, FromJson

structure InitializeCaseRequest where
  state : ArbitrationState
  question : String
  council_members : List CouncilMember := []
  deriving Inhabited, ToJson, FromJson, DecidableEq

structure OpportunitySpec where
  opportunity_id : String
  role : String
  phase : String
  may_pass : Bool := false
  objective : String
  allowed_tools : List String
  deriving Inhabited, ToJson, DecidableEq

structure StepOk where
  ok : Bool := true
  state : ArbitrationState
  deriving Inhabited, ToJson, DecidableEq

structure StepErr where
  ok : Bool := false
  error : String
  deriving Inhabited, ToJson, DecidableEq

structure NextOpportunityOk where
  ok : Bool := true
  terminal : Bool := false
  reason : String := ""
  state_version : Nat := 0
  opportunity : Option OpportunitySpec := none
  deriving Inhabited, ToJson, DecidableEq

def trimString (s : String) : String :=
  s.trimAscii.toString

def getString (j : Json) (k : String) : Except String String := do
  let v ← j.getObjVal? k
  v.getStr?

def getNat (j : Json) (k : String) : Except String Nat := do
  let v ← j.getObjVal? k
  v.getNat?

def getOptionalString (j : Json) (k : String) : String :=
  match j.getObjVal? k with
  | .ok value =>
      match value.getStr? with
      | .ok s => trimString s
      | .error _ => ""
  | .error _ => ""

def getOptionalArray (j : Json) (k : String) : Except String (List Json) := do
  match j.getObjVal? k with
  | .error _ => pure []
  | .ok (.arr xs) => pure xs.toList
  | .ok _ => throw s!"{k} must be an array"

def stateWithCase (s : ArbitrationState) (c : ArbitrationCase) : ArbitrationState :=
  { s with case := c, state_version := s.state_version + 1 }

def hasDuplicateStrings : List String → Bool
  | [] => false
  | x :: xs => xs.any (fun y => y = x) || hasDuplicateStrings xs

def hasDuplicateCouncilMemberIds (members : List CouncilMember) : Bool :=
  hasDuplicateStrings (members.map (fun member => member.member_id))

def plaintiffThenDefendant (xs : List Filing) (plaintiffLabel defendantLabel : String) : Option String :=
  if xs.length = 0 then
    some plaintiffLabel
  else if xs.length = 1 then
    some defendantLabel
  else
    none

def currentRoundAnswers (c : ArbitrationCase) : List CouncilAnswer :=
  c.council_answers.filter (fun answer => answer.round = c.deliberation_round)

def memberIsSeated (member : CouncilMember) : Bool :=
  member.status = "seated"

def seatedCouncilMembers (c : ArbitrationCase) : List CouncilMember :=
  c.council_members.filter memberIsSeated

def seatedCouncilMemberCount (c : ArbitrationCase) : Nat :=
  (seatedCouncilMembers c).length

def offeredEvidenceCountForRole (items : List OfferedEvidence) (role : String) : Nat :=
  items.foldl (fun acc item => if item.role = role then acc + 1 else acc) 0

def technicalReportCountForRole (items : List TechnicalReport) (role : String) : Nat :=
  items.foldl (fun acc item => if item.role = role then acc + 1 else acc) 0

def submittedEvidenceCountForRole (items : List SubmittedEvidence) (role : String) : Nat :=
  items.foldl (fun acc item => if item.role = role then acc + 1 else acc) 0

def nextCouncilMember? (c : ArbitrationCase) : Option CouncilMember :=
  let answers := currentRoundAnswers c
  (seatedCouncilMembers c).find? (fun member =>
    !(answers.any (fun answer => answer.member_id = member.member_id)))

def nextOpportunityForPhase (s : ArbitrationState) : NextOpportunityOk :=
  let c := s.case
  match c.phase with
  | "openings" =>
      match plaintiffThenDefendant c.openings "plaintiff" "defendant" with
      | some role =>
          { state_version := s.state_version
            opportunity := some {
              opportunity_id := s!"openings:{role}"
              role := role
              phase := "openings"
              objective := s!"{role} opening statement"
              allowed_tools := ["record_opening_statement"]
            } }
      | none =>
          { terminal := true, reason := "no_opening_opportunity", state_version := s.state_version }
  | "arguments" =>
      match plaintiffThenDefendant c.arguments "plaintiff" "defendant" with
      | some role =>
          { state_version := s.state_version
            opportunity := some {
              opportunity_id := s!"arguments:{role}"
              role := role
              phase := "arguments"
              objective := s!"{role} merits argument"
              allowed_tools := ["submit_evidence", "submit_argument"]
            } }
      | none =>
          { terminal := true, reason := "no_argument_opportunity", state_version := s.state_version }
  | "rebuttals" =>
      if c.rebuttals.isEmpty then
        { state_version := s.state_version
          opportunity := some {
            opportunity_id := "rebuttals:plaintiff"
            role := "plaintiff"
            phase := "rebuttals"
            may_pass := true
            objective := "plaintiff rebuttal"
            allowed_tools := ["submit_evidence", "submit_rebuttal", "pass_phase_opportunity"]
          } }
      else
        { terminal := true, reason := "no_rebuttal_opportunity", state_version := s.state_version }
  | "surrebuttals" =>
      if c.surrebuttals.isEmpty then
        { state_version := s.state_version
          opportunity := some {
            opportunity_id := "surrebuttals:defendant"
            role := "defendant"
            phase := "surrebuttals"
            may_pass := true
            objective := "defendant surrebuttal"
            allowed_tools := ["submit_surrebuttal", "pass_phase_opportunity"]
          } }
      else
        { terminal := true, reason := "no_surrebuttal_opportunity", state_version := s.state_version }
  | "closings" =>
      match plaintiffThenDefendant c.closings "plaintiff" "defendant" with
      | some role =>
          { state_version := s.state_version
            opportunity := some {
              opportunity_id := s!"closings:{role}"
              role := role
              phase := "closings"
              objective := s!"{role} closing statement"
              allowed_tools := ["deliver_closing_statement"]
            } }
      | none =>
          { terminal := true, reason := "no_closing_opportunity", state_version := s.state_version }
  | "deliberation" =>
      match nextCouncilMember? c with
      | some member =>
          { state_version := s.state_version
            opportunity := some {
              opportunity_id := s!"deliberation:{c.deliberation_round}:{member.member_id}"
              role := "council"
              phase := "deliberation"
              objective := s!"council answer by {member.member_id}"
              allowed_tools := ["submit_council_answer"]
            } }
      | none =>
          { terminal := true, reason := "awaiting_deliberation_completion", state_version := s.state_version }
  | "closed" =>
      { terminal := true, reason := "answers_complete", state_version := s.state_version }
  | _ =>
      { terminal := true, reason := s!"invalid_phase:{c.phase}", state_version := s.state_version }

def nextOpportunity (s : ArbitrationState) : NextOpportunityOk :=
  let c := s.case
  if c.status = "closed" then
    { terminal := true, reason := "answers_complete", state_version := s.state_version }
  else if c.status = "failed" then
    match c.failure with
    | some failure =>
        { terminal := true, reason := if failure.reason = "" then "case_failed" else failure.reason, state_version := s.state_version }
    | none =>
        { terminal := true, reason := "case_failed", state_version := s.state_version }
  else
    nextOpportunityForPhase s

def requireRole (actual expected : String) : Except String Unit :=
  if trimString actual = expected then
    pure ()
  else
    throw s!"invalid actor role: expected {expected}, got {actual}"

def requireTextWithinLimit (label text : String) (limit : Nat) : Except String Unit := do
  let cleaned := trimString text
  if cleaned = "" then
    throw s!"{label} text is required"
  if cleaned.length > limit then
    throw s!"{label} exceeds character limit of {limit}"

def requireCountWithinLimit (label : String) (count limit : Nat) : Except String Unit := do
  if count > limit then
    throw s!"{label} exceed limit of {limit}"

def validatePolicy (p : ArbitrationPolicy) : Except String Unit := do
  if p.council_size = 0 then
    throw "policy.council_size must be positive"
  if trimString p.judgment_standard = "" then
    throw "policy.judgment_standard is required"
  if p.max_opening_chars = 0 then
    throw "policy.max_opening_chars must be positive"
  if p.max_argument_chars = 0 then
    throw "policy.max_argument_chars must be positive"
  if p.max_rebuttal_chars = 0 then
    throw "policy.max_rebuttal_chars must be positive"
  if p.max_surrebuttal_chars = 0 then
    throw "policy.max_surrebuttal_chars must be positive"
  if p.max_closing_chars = 0 then
    throw "policy.max_closing_chars must be positive"
  if p.max_exhibits_per_filing > p.max_exhibits_per_side then
    throw "policy.max_exhibits_per_filing exceeds max_exhibits_per_side"
  if p.max_exhibit_bytes = 0 then
    throw "policy.max_exhibit_bytes must be positive"
  if p.max_reports_per_filing > p.max_reports_per_side then
    throw "policy.max_reports_per_filing exceeds max_reports_per_side"
  if p.max_report_title_bytes = 0 then
    throw "policy.max_report_title_bytes must be positive"
  if p.max_report_summary_bytes = 0 then
    throw "policy.max_report_summary_bytes must be positive"
  if p.max_submitted_evidence_bytes = 0 then
    throw "policy.max_submitted_evidence_bytes must be positive"

def advanceAfterMerits (c : ArbitrationCase) : ArbitrationCase :=
  if c.openings.length >= 2 && c.phase = "openings" then
    { c with phase := "arguments" }
  else if c.arguments.length >= 2 && c.phase = "arguments" then
    { c with phase := "rebuttals" }
  else if c.rebuttals.length >= 1 && c.phase = "rebuttals" then
    { c with phase := "surrebuttals" }
  else if c.surrebuttals.length >= 1 && c.phase = "surrebuttals" then
    { c with phase := "closings" }
  else if c.closings.length >= 2 && c.phase = "closings" then
    { c with phase := "deliberation" }
  else
    c

def addFiling (c : ArbitrationCase) (phase role text : String) : ArbitrationCase :=
  let filing := { phase := phase, role := role, text := text }
  let c1 :=
    match phase with
    | "openings" => { c with openings := c.openings.concat filing }
    | "arguments" => { c with arguments := c.arguments.concat filing }
    | "rebuttals" => { c with rebuttals := c.rebuttals.concat filing }
    | "surrebuttals" => { c with surrebuttals := c.surrebuttals.concat filing }
    | "closings" => { c with closings := c.closings.concat filing }
    | _ => c
  advanceAfterMerits c1

def appendSupplementalMaterials
  (c : ArbitrationCase)
  (offered : List OfferedEvidence)
  (reports : List TechnicalReport) : ArbitrationCase :=
  { c with
    offered_evidence := c.offered_evidence ++ offered
    technical_reports := c.technical_reports ++ reports
  }

def appendSubmittedEvidence
  (c : ArbitrationCase)
  (evidence : SubmittedEvidence) : ArbitrationCase :=
  { c with submitted_evidence := c.submitted_evidence.concat evidence }

def parseOfferedEvidenceEntry (entry : Json) (phase role : String) : Except String OfferedEvidence := do
  let rawEvidenceId ← getString entry "evidence_id"
  let evidenceId := trimString rawEvidenceId
  if evidenceId = "" then
    .error "offered_evidence entry requires evidence_id"
  else
    .ok {
      phase := phase
      role := role
      evidence_id := evidenceId
      label := getOptionalString entry "label"
    }

def parseOfferedEvidenceEntries (entries : List Json) (phase role : String) : Except String (List OfferedEvidence) := do
  match entries with
  | [] => pure []
  | entry :: rest =>
      let first ← parseOfferedEvidenceEntry entry phase role
      let tail ← parseOfferedEvidenceEntries rest phase role
      pure (first :: tail)

def parseOfferedEvidence (payload : Json) (phase role : String) : Except String (List OfferedEvidence) := do
  let entries ← getOptionalArray payload "offered_evidence"
  parseOfferedEvidenceEntries entries phase role

def parseTechnicalReportEntry (entry : Json) (phase role : String) : Except String TechnicalReport := do
  let rawTitle ← getString entry "title"
  let rawSummary ← getString entry "summary"
  let title := trimString rawTitle
  let summary := trimString rawSummary
  if title = "" then
    .error "technical_reports entry requires title"
  else if summary = "" then
    .error "technical_reports entry requires summary"
  else
    .ok {
      phase := phase
      role := role
      title := title
      summary := summary
    }

def parseTechnicalReportEntries
    (entries : List Json)
    (phase role : String) : Except String (List TechnicalReport) := do
  match entries with
  | [] => pure []
  | entry :: rest =>
      let first ← parseTechnicalReportEntry entry phase role
      let tail ← parseTechnicalReportEntries rest phase role
      pure (first :: tail)

def parseTechnicalReports (payload : Json) (phase role : String) : Except String (List TechnicalReport) := do
  let entries ← getOptionalArray payload "technical_reports"
  parseTechnicalReportEntries entries phase role

def parseSubmittedEvidence (payload : Json) (phase role : String) : Except String SubmittedEvidence := do
  let rawEvidenceId ← getString payload "evidence_id"
  let rawTitle ← getString payload "title"
  let rawMimeType ← getString payload "mime_type"
  let rawRelevance ← getString payload "relevance"
  let rawSha256 ← getString payload "sha256"
  let evidenceId := trimString rawEvidenceId
  let title := trimString rawTitle
  let mimeType := trimString rawMimeType
  let relevance := trimString rawRelevance
  let sha256 := trimString rawSha256
  let sourceUrl := getOptionalString payload "source_url"
  let sourceDescription := getOptionalString payload "source_description"
  let retrievalTimestamp := getOptionalString payload "retrieval_timestamp"
  let sizeBytes ← payload.getObjValAs? Nat "size_bytes"
  if evidenceId = "" then
    throw "submitted evidence requires evidence_id"
  if title = "" then
    throw "submitted evidence requires title"
  if sourceUrl = "" && sourceDescription = "" then
    throw "submitted evidence requires source_url or source_description"
  if mimeType = "" then
    throw "submitted evidence requires mime_type"
  if relevance = "" then
    throw "submitted evidence requires relevance"
  if sha256 = "" then
    throw "submitted evidence requires sha256"
  if sizeBytes = 0 then
    throw "submitted evidence size_bytes must be positive"
  pure {
    phase := phase
    role := role
    evidence_id := evidenceId
    title := title
    source_url := sourceUrl
    source_description := sourceDescription
    mime_type := mimeType
    retrieval_timestamp := retrievalTimestamp
    relevance := relevance
    sha256 := sha256
    size_bytes := sizeBytes
  }

def submitEvidence
  (s : ArbitrationState)
  (actorRole : String)
  (payload : Json) : Except String ArbitrationState := do
  let c := s.case
  let expectedRole ←
    match c.phase with
    | "arguments" =>
        pure (if c.arguments.isEmpty then "plaintiff" else "defendant")
    | "rebuttals" =>
        if c.rebuttals.isEmpty then
          pure "plaintiff"
        else
          throw "rebuttal evidence is closed"
    | _ => throw "submitted evidence is allowed only in arguments and rebuttals"
  requireRole actorRole expectedRole
  let parsedEvidence ← parseSubmittedEvidence payload c.phase expectedRole
  let evidence := { parsedEvidence with role := expectedRole }
  if c.submitted_evidence.any (fun item => item.evidence_id = evidence.evidence_id) then
    throw s!"duplicate submitted evidence_id: {evidence.evidence_id}"
  else if evidence.size_bytes > s.policy.max_submitted_evidence_bytes then
    throw s!"submitted evidence exceeds byte limit of {s.policy.max_submitted_evidence_bytes}"
  else
    let total := submittedEvidenceCountForRole c.submitted_evidence expectedRole + 1
    requireCountWithinLimit "submitted_evidence for this side" total s.policy.max_submitted_evidence_per_side
    pure <| stateWithCase s (appendSubmittedEvidence c evidence)

def requireNoSupplementalMaterials (payload : Json) : Except String Unit := do
  let offered ← getOptionalArray payload "offered_evidence"
  if !offered.isEmpty then
    throw "offered_evidence are allowed only in arguments and rebuttals"
  let reports ← getOptionalArray payload "technical_reports"
  if !reports.isEmpty then
    throw "technical_reports are allowed only in arguments and rebuttals"

def recordMeritsSubmission
  (s : ArbitrationState)
  (phase actorRole expectedRole textLabel : String)
  (limit : Nat)
  (allowSupplementalMaterials : Bool)
  (payload : Json) : Except String ArbitrationState := do
  let c := s.case
  if c.phase != phase then
    throw s!"{phase} are closed"
  requireRole actorRole expectedRole
  let text := trimString (← getString payload "text")
  requireTextWithinLimit textLabel text limit
  if allowSupplementalMaterials then
    let offered ← parseOfferedEvidence payload phase expectedRole
    let reports ← parseTechnicalReports payload phase expectedRole
    requireCountWithinLimit "offered_evidence" offered.length s.policy.max_exhibits_per_filing
    requireCountWithinLimit "technical_reports" reports.length s.policy.max_reports_per_filing
    let totalOffered := offeredEvidenceCountForRole c.offered_evidence expectedRole + offered.length
    let totalReports := technicalReportCountForRole c.technical_reports expectedRole + reports.length
    requireCountWithinLimit "offered_evidence for this side" totalOffered s.policy.max_exhibits_per_side
    requireCountWithinLimit "technical_reports for this side" totalReports s.policy.max_reports_per_side
    let c1 := addFiling c phase expectedRole text
    let c2 := appendSupplementalMaterials c1 offered reports
    pure <| stateWithCase s c2
  else
    requireNoSupplementalMaterials payload
    pure <| stateWithCase s (addFiling c phase expectedRole text)

def continueDeliberation (s : ArbitrationState) (c : ArbitrationCase) : Except String ArbitrationState := do
  if (currentRoundAnswers c).length = seatedCouncilMemberCount c then
    pure <| stateWithCase s { c with status := "closed", phase := "closed" }
  else
    pure <| stateWithCase s c

def recordCouncilAnswer (s : ArbitrationState) (memberId : String) (answer : Nat) (rationale : String) :
    Except String ArbitrationState := do
  let c := s.case
  if c.phase != "deliberation" then
    throw "council answer is only allowed in deliberation"
  if !(c.council_members.any (fun member => member.member_id = memberId)) then
    throw s!"unknown council member: {memberId}"
  if !(c.council_members.any (fun member => member.member_id = memberId && memberIsSeated member)) then
    throw s!"council member is not seated: {memberId}"
  if answer > 100 then
    throw "council answer must be between 0 and 100"
  let cleanedRationale := trimString rationale
  if cleanedRationale = "" then
    throw "council answer rationale is required"
  let answers := currentRoundAnswers c
  if answers.any (fun existing => existing.member_id = memberId) then
    throw s!"council member already answered this round: {memberId}"
  let c1 := { c with council_answers := c.council_answers.concat {
      member_id := memberId
      round := c.deliberation_round
      answer := answer
      rationale := cleanedRationale
    } }
  continueDeliberation s c1

def removeCouncilMember (s : ArbitrationState) (memberId status : String) : Except String ArbitrationState := do
  let c := s.case
  if c.phase != "deliberation" then
    throw "council member removal is allowed only in deliberation"
  if !(c.council_members.any (fun member => member.member_id = memberId)) then
    throw s!"unknown council member: {memberId}"
  if !(c.council_members.any (fun member => member.member_id = memberId && memberIsSeated member)) then
    throw s!"council member is not seated: {memberId}"
  let cleanedStatus := trimString status
  if cleanedStatus = "" then
    throw "council member removal requires status"
  if cleanedStatus = "seated" then
    throw "council member removal requires a non-seated status"
  let answers := currentRoundAnswers c
  if answers.any (fun answer => answer.member_id = memberId) then
    throw s!"cannot remove council member after current-round answer: {memberId}"
  let c1 := {
    c with council_members := c.council_members.map (fun member =>
      if member.member_id = memberId then
        { member with status := cleanedStatus }
      else
        member)
  }
  continueDeliberation s c1

def failCouncilMemberOpportunity
  (s : ArbitrationState)
  (memberId reason opportunityId message : String) : Except String ArbitrationState := do
  let c := s.case
  if c.phase != "deliberation" then
    throw "council opportunity failure is allowed only in deliberation"
  if memberId = "" then
    throw "council opportunity failure requires member_id"
  if reason = "" then
    throw "opportunity failure requires reason"
  if !(c.council_members.any (fun member => member.member_id = memberId)) then
    throw s!"unknown council member: {memberId}"
  if !(c.council_members.any (fun member => member.member_id = memberId && memberIsSeated member)) then
    throw s!"council member is not seated: {memberId}"
  let answers := currentRoundAnswers c
  if answers.any (fun answer => answer.member_id = memberId) then
    throw s!"cannot fail council member after current-round answer: {memberId}"
  let c1 := {
    c with council_members := c.council_members.map (fun member =>
      if member.member_id = memberId then
        { member with
          status := "failed"
          failure_reason := reason
          failure_opportunity_id := opportunityId
          failure_message := message
        }
      else
        member)
  }
  continueDeliberation s c1

def failOpportunity (s : ArbitrationState) (payload : Json) : Except String ArbitrationState := do
  let opportunityId := trimString (← getString payload "opportunity_id")
  let role := trimString (← getString payload "role")
  let phase := trimString (← getString payload "phase")
  let reason := trimString (← getString payload "reason")
  let message := getOptionalString payload "message"
  let memberId := getOptionalString payload "member_id"
  let model := getOptionalString payload "model"
  if opportunityId = "" then
    throw "opportunity failure requires opportunity_id"
  if role = "" then
    throw "opportunity failure requires role"
  if phase = "" then
    throw "opportunity failure requires phase"
  if reason = "" then
    throw "opportunity failure requires reason"
  match (nextOpportunity s).opportunity with
  | none => throw "no active opportunity can fail"
  | some opportunity =>
      if opportunity.opportunity_id != opportunityId then
        throw s!"opportunity_id {opportunityId} does not match current opportunity {opportunity.opportunity_id}"
      if opportunity.role != role then
        throw s!"opportunity role {role} does not match current role {opportunity.role}"
      if opportunity.phase != phase then
        throw s!"opportunity phase {phase} does not match current phase {opportunity.phase}"
      if role = "council" then
        failCouncilMemberOpportunity s memberId reason opportunityId message
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
        pure <| stateWithCase s { s.case with status := "failed", failure := some failure }
      else
        throw s!"unsupported opportunity failure role: {role}"

def initializeCase (req : InitializeCaseRequest) : Except String ArbitrationState := do
  let question := trimString req.question
  let standard := trimString req.state.policy.judgment_standard
  validatePolicy req.state.policy
  if question = "" then
    throw "question is required"
  if standard = "" then
    throw "policy.judgment_standard is required"
  if req.council_members.isEmpty then
    throw "at least one council member is required"
  if req.council_members.length != req.state.policy.council_size then
    throw "council_members length does not match policy.council_size"
  if hasDuplicateCouncilMemberIds req.council_members then
    throw "council_members contain duplicate member_id"
  let c := { req.state.case with
    question := question
    council_members := req.council_members.map (fun member => { member with status := "seated" })
    status := "active"
    phase := "openings"
    openings := []
    arguments := []
    rebuttals := []
    surrebuttals := []
    closings := []
    offered_evidence := []
    technical_reports := []
    submitted_evidence := []
    deliberation_round := 1
    council_answers := []
    failure := none
  }
  pure <| stateWithCase req.state c

def step (req : StepRequest) : Except String ArbitrationState := do
  let c := req.state.case
  match req.action.action_type with
  | "record_opening_statement" =>
      if c.phase != "openings" then
        throw "opening statements are closed"
      let role := if c.openings.isEmpty then "plaintiff" else "defendant"
      requireRole req.action.actor_role role
      let text := trimString (← getString req.action.payload "text")
      requireTextWithinLimit "opening statement" text req.state.policy.max_opening_chars
      pure <| stateWithCase req.state (addFiling c "openings" role text)
  | "submit_argument" =>
      recordMeritsSubmission
        req.state
        "arguments"
        req.action.actor_role
        (if c.arguments.isEmpty then "plaintiff" else "defendant")
        "argument"
        req.state.policy.max_argument_chars
        true
        req.action.payload
  | "submit_rebuttal" =>
      recordMeritsSubmission
        req.state
        "rebuttals"
        req.action.actor_role
        "plaintiff"
        "rebuttal"
        req.state.policy.max_rebuttal_chars
        true
        req.action.payload
  | "submit_surrebuttal" =>
      recordMeritsSubmission
        req.state
        "surrebuttals"
        req.action.actor_role
        "defendant"
        "surrebuttal"
        req.state.policy.max_surrebuttal_chars
        false
        req.action.payload
  | "submit_evidence" =>
      submitEvidence req.state req.action.actor_role req.action.payload
  | "deliver_closing_statement" =>
      if c.phase != "closings" then
        throw "closings are closed"
      let role := if c.closings.isEmpty then "plaintiff" else "defendant"
      requireRole req.action.actor_role role
      let text := trimString (← getString req.action.payload "text")
      requireTextWithinLimit "closing statement" text req.state.policy.max_closing_chars
      pure <| stateWithCase req.state (addFiling c "closings" role text)
  | "pass_phase_opportunity" =>
      if c.phase = "rebuttals" then
        requireRole req.action.actor_role "plaintiff"
        if !c.rebuttals.isEmpty then
          throw "rebuttal already submitted"
        pure <| stateWithCase req.state { c with phase := "surrebuttals" }
      else if c.phase = "surrebuttals" then
        requireRole req.action.actor_role "defendant"
        if !c.surrebuttals.isEmpty then
          throw "surrebuttal already submitted"
        pure <| stateWithCase req.state { c with phase := "closings" }
      else
        throw "pass is allowed only in rebuttals or surrebuttals"
  | "submit_council_answer" =>
      requireRole req.action.actor_role "council"
      let memberId := trimString (← getString req.action.payload "member_id")
      let answer := (← getNat req.action.payload "answer")
      let rationale := trimString (← getString req.action.payload "rationale")
      recordCouncilAnswer req.state memberId answer rationale
  | "remove_council_member" =>
      requireRole req.action.actor_role "system"
      let memberId := trimString (← getString req.action.payload "member_id")
      let status := (← getString req.action.payload "status")
      removeCouncilMember req.state memberId status
  | "fail_opportunity" =>
      requireRole req.action.actor_role "system"
      failOpportunity req.state req.action.payload
  | _ => throw s!"unknown action type: {req.action.action_type}"

def parseJsonInput (input : String) : Except String Json := do
  Json.parse input

def parseStepRequest (j : Json) : Except String StepRequest := do
  fromJson? j

def parseInitializeCaseRequest (j : Json) : Except String InitializeCaseRequest := do
  fromJson? j

def getRequestType (j : Json) : String :=
  match j.getObjVal? "request_type" with
  | .ok value =>
      match value.getStr? with
      | .ok s => s
      | .error _ => ""
  | .error _ => ""

def printJsonLine (j : Json) : IO Unit :=
  IO.println (toString j.compress)

def main (_args : List String) : IO UInt32 := do
  let stdin ← IO.getStdin
  let input ← stdin.readToEnd
  match parseJsonInput input with
  | .error err =>
      printJsonLine (toJson ({ ok := false, error := err } : StepErr))
      pure 1
  | .ok j =>
      match getRequestType j with
      | "initialize_case" =>
          match parseInitializeCaseRequest j with
          | .error err =>
              printJsonLine (toJson ({ ok := false, error := err } : StepErr))
              pure 1
          | .ok req =>
              match initializeCase req with
              | .error err =>
                  printJsonLine (toJson ({ ok := false, error := err } : StepErr))
                  pure 1
              | .ok state =>
                  printJsonLine (toJson ({ ok := true, state := state } : StepOk))
                  pure 0
      | "next_opportunity" =>
          match j.getObjValAs? ArbitrationState "state" with
          | .error err =>
              printJsonLine (toJson ({ ok := false, error := err } : StepErr))
              pure 1
          | .ok state =>
              printJsonLine (toJson (nextOpportunity state))
              pure 0
      | _ =>
          match parseStepRequest j with
          | .error err =>
              printJsonLine (toJson ({ ok := false, error := err } : StepErr))
              pure 1
          | .ok req =>
              match step req with
              | .error err =>
                  printJsonLine (toJson ({ ok := false, error := err } : StepErr))
                  pure 1
              | .ok state =>
                  printJsonLine (toJson ({ ok := true, state := state } : StepOk))
                  pure 0
