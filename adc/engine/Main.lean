import Lean

open Lean

structure DocketEntry where
  title : String
  description : String
  deriving Inhabited, ToJson, FromJson

structure DecisionTrace where
  action : String
  outcome : String
  citations : List String
  deriving Inhabited, ToJson, FromJson

structure JuryConfiguration where
  juror_count : Nat
  unanimous_required : Bool
  minimum_concurring : Nat
  deriving Inhabited, ToJson, FromJson

structure JurorRecord where
  juror_id : String
  name : String
  status : String
  note : String := ""
  model : String := ""
  persona_filename : String := ""
  deriving Inhabited, ToJson, FromJson

structure JurorQuestionnaireItem where
  question_id : String
  question : String
  deriving Inhabited, ToJson, FromJson

structure JurorQuestionnaireAnswer where
  question_id : String
  answer : String
  deriving Inhabited, ToJson, FromJson

structure JurorQuestionnaireResponse where
  juror_id : String
  answers : List JurorQuestionnaireAnswer
  submitted_at : String
  deriving Inhabited, ToJson, FromJson

structure VoirDireExchange where
  exchange_id : String
  juror_id : String
  asked_by : String
  question : String
  judge_allowed : Option Bool := none
  ruling_reason : String := ""
  response : String := ""
  asked_at : String
  ruled_at : Option String := none
  answered_at : Option String := none
  deriving Inhabited, ToJson, FromJson

structure ForCauseChallenge where
  challenge_id : String
  juror_id : String
  by_party : String
  grounds : String
  requested_at : String
  ruling_reason : String := ""
  granted : Option Bool := none
  decided_at : Option String := none
  deriving Inhabited, ToJson, FromJson

structure JuryVerdict where
  verdict_for : String
  votes_for_verdict : Nat
  required_votes : Nat
  damages : Float := 0.0
  deriving Inhabited, ToJson, FromJson

structure HungJury where
  claim_id : String
  note : String
  deriving Inhabited, ToJson, FromJson

structure ContemptCounter where
  role : String
  count : Nat
  deriving Inhabited, ToJson, FromJson

structure LocalRuleOverrideV1 where
  ordered_at : String
  override_id : String
  limit_key : String
  new_value : Nat
  scope_party : Option String := none
  scope_phase : Option String := none
  ordered_by : String
  reason : String
  active : Bool := true
  expires_at : Option String := none
  deriving Inhabited, ToJson, FromJson

structure LimitUsageV1 where
  limit_key : String
  actor : String
  phase : String
  value : Nat
  deriving Inhabited, ToJson, FromJson

structure ProtectiveOrder where
  entered_at : String
  order_id : String
  scope : String
  target : String
  allowed_roles : List String
  note : String
  active : Bool := true
  lifted_at : Option String := none
  deriving Inhabited, ToJson, FromJson

structure BenchFinding where
  issue : String
  finding : String
  entered_at : String
  deriving Inhabited, ToJson, FromJson

structure BenchConclusion where
  issue : String
  conclusion : String
  entered_at : String
  deriving Inhabited, ToJson, FromJson

structure JurorExplanation where
  juror_id : String
  vote : String
  confidence : String := ""
  explanation : String
  submitted_at : String
  deriving Inhabited, ToJson, FromJson

structure JurorVote where
  juror_id : String
  round : Nat := 1
  vote : String
  damages : Float := 0.0
  confidence : String := ""
  explanation : String
  submitted_at : String
  deriving Inhabited, ToJson, FromJson

structure Rule68Offer where
  offer_id : String
  offeror : String
  offeree : String
  amount : Float
  status : String := "pending"
  terms : String := ""
  claim_scope : String := ""
  served_at : String := ""
  expires_at : Option String := none
  accepted_at : Option String := none
  expired_at : Option String := none
  deriving Inhabited, ToJson, FromJson

structure TechnicalReport where
  report_id : String
  party : String
  title : String
  summary : String
  method_notes : String := ""
  limitations : String := ""
  file_id : String := ""
  submitted_at : String
  deriving Inhabited, ToJson, FromJson

structure CaseState where
  case_id : String
  caption : String
  judge : String
  filed_on : String
  auto_rule11 : Bool := false
  status : String := "filed"
  trial_mode : String := "unset"
  phase : String := "none"
  last_pleading_served_on : String := ""
  jury_demanded_on : String := ""
  jury_configuration : Option JuryConfiguration := none
  single_claim : Option Json := none
  jurisdictional_allegations : Option Json := none
  jurors : List JurorRecord := []
  juror_questionnaire : List JurorQuestionnaireItem := []
  juror_questionnaire_responses : List JurorQuestionnaireResponse := []
  voir_dire_exchanges : List VoirDireExchange := []
  for_cause_challenges : List ForCauseChallenge := []
  deliberation_round : Nat := 1
  juror_votes : List JurorVote := []
  jury_verdict : Option JuryVerdict := none
  hung_jury : Option HungJury := none
  contempt_counts : List ContemptCounter := []
  protective_orders : List ProtectiveOrder := []
  bench_findings : List BenchFinding := []
  bench_conclusions : List BenchConclusion := []
  juror_explanations : List JurorExplanation := []
  local_rule_overrides : List LocalRuleOverrideV1 := []
  limit_usage : List LimitUsageV1 := []
  rule56_window_closed_for : List String := []
  case_files : List Json := []
  file_events : List Json := []
  rule68_offers : List Rule68Offer := []
  technical_reports : List TechnicalReport := []
  monetary_judgment : Float := 0.0
  docket : List DocketEntry := []
  decision_traces : List DecisionTrace := []
  deriving Inhabited, ToJson, FromJson

structure CourtPolicy where
  max_opening_chars : Nat := 6000
  max_trial_theory_chars : Nat := 4000
  max_closing_chars : Nat := 8000
  max_exhibits_per_side : Nat := 20
  max_support_tool_calls_per_opportunity : Nat := 30
  max_jury_note_chars : Nat := 3000
  skip_voir_dire : Nat := 0
  jury_juror_count : Nat := 6
  jury_unanimous_required : Nat := 1
  jury_minimum_concurring : Nat := 6
  voir_dire_candidate_count : Nat := 10
  max_voir_dire_questions_per_side_per_juror : Nat := 1
  max_disallowed_voir_dire_questions_per_side : Nat := 3
  max_for_cause_challenges_per_side : Nat := 1
  max_peremptory_challenges_per_side : Nat := 1
  max_deliberation_rounds : Nat := 3
  max_dispositive_motions_per_side_pretrial : Nat := 2
  max_interrogatories_per_set : Nat := 5
  max_interrogatory_sets_per_side : Nat := 2
  max_rfp_requests_per_set : Nat := 40
  max_rfp_sets_per_side : Nat := 2
  max_rfa_requests_per_set : Nat := 40
  max_rfa_sets_per_side : Nat := 2
  max_discovery_response_deadline_days : Nat := 30
  max_rule12_summary_chars : Nat := 5000
  max_rule56_summary_chars : Nat := 10000
  max_rule56_reply_chars : Nat := 4000
  max_technical_reports_per_side : Nat := 3
  max_technical_report_summary_chars : Nat := 5000
  deriving Inhabited, ToJson, FromJson

structure CourtState where
  schema_version : String := "v1"
  court_name : String
  court_profile : Option Json := none
  case : CaseState
  policy : CourtPolicy := {}
  state_version : Nat := 0
  passed_opportunities : List String := []
  deriving Inhabited, ToJson, FromJson

structure CourtAction where
  action_type : String
  actor_role : String
  payload : Json
  deriving Inhabited, ToJson, FromJson

structure StepRequest where
  state : CourtState
  action : CourtAction
  deriving Inhabited, ToJson, FromJson

structure ViewRequest where
  state : CourtState
  role : String
  deriving Inhabited, ToJson, FromJson

structure StepOk where
  ok : Bool := true
  state : CourtState
  deriving Inhabited, ToJson

structure StepErr where
  ok : Bool := false
  error : String
  code : String := ""
  details : Json := Json.null
  retryable : Bool := false
  actor_message : String := ""
  deriving Inhabited, ToJson

structure ViewOk where
  ok : Bool := true
  view : Json
  deriving Inhabited, ToJson

structure RolePolicy where
  role : String
  allowed_tools : List String
  deriving Inhabited, ToJson, FromJson

structure OpportunitySpec where
  opportunity_id : String := ""
  role : String
  phase : String := ""
  kind : String := ""
  may_pass : Bool := false
  actor_message : String := ""
  objective : String
  allowed_tools : List String
  step_budget : Nat
  priority : Nat := 100
  constraints : Json := Json.null
  deterministic_action : Option Json := none
  deriving Inhabited, ToJson

structure NextOpportunityOk where
  ok : Bool := true
  terminal : Bool := false
  reason : String := ""
  state_version : Nat := 0
  opportunity : Option OpportunitySpec := none
  deriving Inhabited, ToJson

structure OpportunityRequest where
  state : CourtState
  roles : List RolePolicy
  max_steps_per_turn : Nat := 3
  deriving Inhabited, ToJson, FromJson

structure AgendaOk where
  ok : Bool := true
  state_version : Nat := 0
  terminal : Bool := false
  opportunities : List OpportunitySpec := []
  deriving Inhabited, ToJson

structure DecisionSpec where
  kind : String
  tool_name : Option String := none
  payload : Option Json := none
  reason : Option String := none
  deriving Inhabited, ToJson, FromJson

structure ApplyDecisionRequest where
  state : CourtState
  state_version : Nat
  opportunity_id : String
  role : String
  decision : DecisionSpec
  roles : List RolePolicy
  max_steps_per_turn : Nat := 3
  deriving Inhabited, ToJson, FromJson

structure ApplyDecisionOk where
  ok : Bool := true
  result_kind : String
  state : Option CourtState := none
  action : Option CourtAction := none
  deriving Inhabited, ToJson

structure ComplaintAttachmentSeed where
  file_id : String
  label : String := ""
  original_name : String
  storage_relpath : String := ""
  sha256 : String := ""
  size_bytes : Nat := 0
  deriving Inhabited, ToJson, FromJson

structure InitializeCaseRequest where
  state : CourtState
  complaint_summary : String
  filed_by : String := "plaintiff"
  jury_demanded_on : String := ""
  jurisdictional_allegations : Option Json := none
  attachments : List ComplaintAttachmentSeed := []
  deriving Inhabited, ToJson, FromJson

inductive CaseStatusV1 where
  | filed
  | pretrial
  | trial
  | judgmentEntered
  | closed
  deriving Inhabited, Repr, DecidableEq

def caseStatusV1ToString : CaseStatusV1 → String
  | .filed => "filed"
  | .pretrial => "pretrial"
  | .trial => "trial"
  | .judgmentEntered => "judgment_entered"
  | .closed => "closed"

def parseCaseStatusV1 : String → Option CaseStatusV1
  | "filed" => some .filed
  | "pretrial" => some .pretrial
  | "trial" => some .trial
  | "judgment_entered" => some .judgmentEntered
  | "closed" => some .closed
  | _ => none

def canTransitionStatusV1 : CaseStatusV1 → CaseStatusV1 → Bool
  | .filed, .pretrial => true
  | .filed, .closed => true
  | .pretrial, .trial => true
  | .pretrial, .closed => true
  | .trial, .judgmentEntered => true
  | .trial, .closed => true
  | .judgmentEntered, .closed => true
  | _, _ => false

inductive TrialPhaseV1 where
  | none
  | voirDire
  | openings
  | plaintiffCase
  | plaintiffEvidence
  | defenseCase
  | defenseEvidence
  | plaintiffRebuttal
  | plaintiffRebuttalEvidence
  | defenseSurrebuttal
  | defenseSurrebuttalEvidence
  | chargeConference
  | closings
  | juryCharge
  | deliberation
  | verdictReturn
  | postVerdict
  deriving Inhabited, Repr, DecidableEq

def parseTrialPhaseV1 : String → Option TrialPhaseV1
  | "none" => some .none
  | "voir_dire" => some .voirDire
  | "openings" => some .openings
  | "plaintiff_case" => some .plaintiffCase
  | "plaintiff_evidence" => some .plaintiffEvidence
  | "defense_case" => some .defenseCase
  | "defense_evidence" => some .defenseEvidence
  | "plaintiff_rebuttal" => some .plaintiffRebuttal
  | "plaintiff_rebuttal_evidence" => some .plaintiffRebuttalEvidence
  | "defense_surrebuttal" => some .defenseSurrebuttal
  | "defense_surrebuttal_evidence" => some .defenseSurrebuttalEvidence
  | "charge_conference" => some .chargeConference
  | "closings" => some .closings
  | "jury_charge" => some .juryCharge
  | "deliberation" => some .deliberation
  | "verdict_return" => some .verdictReturn
  | "post_verdict" => some .postVerdict
  | _ => none

def trialPhaseRankV1 : TrialPhaseV1 → Nat
  | .none => 0
  | .voirDire => 1
  | .openings => 2
  | .plaintiffCase => 3
  | .plaintiffEvidence => 4
  | .defenseCase => 5
  | .defenseEvidence => 6
  | .plaintiffRebuttal => 7
  | .plaintiffRebuttalEvidence => 8
  | .defenseSurrebuttal => 9
  | .defenseSurrebuttalEvidence => 10
  | .chargeConference => 11
  | .closings => 12
  | .juryCharge => 13
  | .deliberation => 14
  | .verdictReturn => 15
  | .postVerdict => 16

def canAdvancePhaseV1 (current next : TrialPhaseV1) : Bool :=
  trialPhaseRankV1 current ≤ trialPhaseRankV1 next

inductive TrialActionV1 where
  | recordOpeningStatement
  | submitPresentation
  | offerExhibit
  | restCase
  | deliverClosingArgument
  | issueJurorQuestionnaire
  | answerJurorQuestionnaire
  | decideVoirDireQuestion
  | answerVoirDireQuestion
  | decideJurorForCauseChallenge
  | empanelJury
  | submitJurorVote
  deriving Inhabited, Repr, DecidableEq

def phaseAllowsActionV1 : TrialActionV1 → TrialPhaseV1 → Bool
  | .recordOpeningStatement, .openings => true
  | .submitPresentation, .plaintiffCase => true
  | .submitPresentation, .defenseCase => true
  | .submitPresentation, .plaintiffRebuttal => true
  | .submitPresentation, .defenseSurrebuttal => true
  | .offerExhibit, .plaintiffEvidence => true
  | .offerExhibit, .defenseEvidence => true
  | .offerExhibit, .plaintiffRebuttalEvidence => true
  | .offerExhibit, .defenseSurrebuttalEvidence => true
  | .restCase, .plaintiffEvidence => true
  | .restCase, .defenseEvidence => true
  | .restCase, .plaintiffRebuttalEvidence => true
  | .restCase, .defenseSurrebuttalEvidence => true
  | .deliverClosingArgument, .closings => true
  | .issueJurorQuestionnaire, .voirDire => true
  | .answerJurorQuestionnaire, .voirDire => true
  | .decideVoirDireQuestion, .voirDire => true
  | .answerVoirDireQuestion, .voirDire => true
  | .decideJurorForCauseChallenge, .voirDire => true
  | .empanelJury, .voirDire => true
  | .submitJurorVote, .deliberation => true
  | _, _ => false

def parseCurrentPhaseV1 (c : CaseState) : Except String TrialPhaseV1 := do
  match parseTrialPhaseV1 c.phase with
  | some p => pure p
  | none => throw s!"invalid current phase: {c.phase}"

inductive VerdictSide where
  | plaintiff
  | defendant
  deriving Inhabited, Repr, DecidableEq

def parseVerdictSide : String → Option VerdictSide
  | "plaintiff" => some .plaintiff
  | "defendant" => some .defendant
  | _ => none

inductive ClaimDispositionV1 where
  | pending
  | verdictPlaintiff
  | verdictDefendant
  | hung
  | judgmentEntered
  deriving Inhabited, Repr, DecidableEq

def canEnterJudgmentFromClaimDispositionV1 : ClaimDispositionV1 → Bool
  | .verdictPlaintiff => true
  | .verdictDefendant => true
  | _ => false

def claimDispositionFromCaseStateV1 (c : CaseState) : Except String ClaimDispositionV1 := do
  if c.hung_jury.isSome then
    pure .hung
  else
    match c.jury_verdict with
    | none => pure .pending
    | some verdict =>
        match parseVerdictSide verdict.verdict_for with
        | some .plaintiff => pure .verdictPlaintiff
        | some .defendant => pure .verdictDefendant
        | none => throw s!"invalid verdict_for value: {verdict.verdict_for}"

def judgmentEligibleFromCaseStateV1 (c : CaseState) : Bool :=
  if c.trial_mode = "bench" then
    c.hung_jury.isNone
  else
    match claimDispositionFromCaseStateV1 c with
    | .ok disposition => canEnterJudgmentFromClaimDispositionV1 disposition
    | .error _ => false

def judgmentAmountFromCaseState (c : CaseState) : Float :=
  match c.jury_verdict with
  | some verdict => verdict.damages
  | none => c.monetary_judgment

def allowedStatuses : List String := ["filed", "pretrial", "trial", "judgment_entered", "closed"]

def allowedPhases : List String := [
  "none", "voir_dire", "openings", "plaintiff_case", "plaintiff_evidence",
  "defense_case", "defense_evidence", "plaintiff_rebuttal", "plaintiff_rebuttal_evidence",
  "defense_surrebuttal", "defense_surrebuttal_evidence", "charge_conference",
  "closings", "jury_charge", "deliberation", "verdict_return", "post_verdict"
]

def phaseOrder : String -> Nat
  | "none" => 0
  | "voir_dire" => 1
  | "openings" => 2
  | "plaintiff_case" => 3
  | "plaintiff_evidence" => 4
  | "defense_case" => 5
  | "defense_evidence" => 6
  | "plaintiff_rebuttal" => 7
  | "plaintiff_rebuttal_evidence" => 8
  | "defense_surrebuttal" => 9
  | "defense_surrebuttal_evidence" => 10
  | "charge_conference" => 11
  | "closings" => 12
  | "jury_charge" => 13
  | "deliberation" => 14
  | "verdict_return" => 15
  | "post_verdict" => 16
  | _ => 999

def requireRole (a : CourtAction) (roles : List String) : Except String Unit :=
  if roles.contains a.actor_role then
    .ok ()
  else
    .error s!"role {a.actor_role} not permitted for {a.action_type}"

def getString (j : Json) (k : String) : Except String String := do
  let v ← j.getObjVal? k
  v.getStr?

def getStringOpt (j : Json) (k : String) : Except String (Option String) :=
  match j.getObjVal? k with
  | .ok v =>
      match v.getStr? with
      | .ok s => .ok (some s)
      | .error e => .error e
  | .error _ => .ok none

def getNat (j : Json) (k : String) : Except String Nat := do
  let v ← j.getObjVal? k
  v.getNat?

def getBoolD (j : Json) (k : String) (d : Bool) : Except String Bool :=
  match j.getObjVal? k with
  | .ok v => v.getBool?
  | .error _ => .ok d

def getBoolOpt (j : Json) (k : String) : Except String (Option Bool) :=
  match j.getObjVal? k with
  | .ok v =>
      match v.getBool? with
      | .ok b => .ok (some b)
      | .error e => .error e
  | .error _ => .ok none

def getFloatD (j : Json) (k : String) (d : Float) : Except String Float :=
  match j.getObjVal? k with
  | .ok v =>
      match v with
      | Json.num n => .ok n.toFloat
      | _ => .error s!"payload field {k} must be number"
  | .error _ => .ok d

def getFloatOpt (j : Json) (k : String) : Except String (Option Float) :=
  match j.getObjVal? k with
  | .ok v =>
      match v with
      | Json.num n => .ok (some n.toFloat)
      | _ => .error s!"payload field {k} must be number"
  | .error _ => .ok none

def getNatOpt (j : Json) (k : String) : Except String (Option Nat) :=
  match j.getObjVal? k with
  | .ok v =>
      match v.getNat? with
      | .ok n => .ok (some n)
      | .error e => .error e
  | .error _ => .ok none

def getArrayLenOpt (j : Json) (k : String) : Except String (Option Nat) :=
  match j.getObjVal? k with
  | .ok v =>
      match v with
      | Json.arr xs => .ok (some xs.size)
      | _ => .error s!"payload field {k} must be array"
  | .error _ => .ok none

def getStringList (j : Json) (k : String) : Except String (List String) :=
  match j.getObjVal? k with
  | .ok v =>
      match v with
      | Json.arr xs =>
          xs.foldr
            (fun item acc =>
              match item.getStr?, acc with
              | .ok s, .ok rest => .ok (s :: rest)
              | .error e, _ => .error e
              | _, .error e => .error e)
            (.ok [])
      | _ => .error s!"payload field {k} must be array"
  | .error _ => .error s!"payload field {k} missing"

def getJurorQuestionnaireAnswers (j : Json) (k : String) : Except String (List JurorQuestionnaireAnswer) :=
  match j.getObjVal? k with
  | .ok v =>
      match v with
      | Json.arr xs =>
          xs.foldr
            (fun item acc =>
              match item, acc with
              | Json.obj _, .ok rest => do
                  let questionId ← getString item "question_id"
                  let answer ← getString item "answer"
                  pure ({ question_id := questionId, answer := answer } :: rest)
              | _, .ok _ => .error s!"payload field {k} entries must be objects"
              | _, .error e => .error e)
            (.ok [])
      | _ => .error s!"payload field {k} must be array"
  | .error _ => .error s!"payload field {k} missing"

def trimString (s : String) : String :=
  s.trimAscii.toString

def normalizePartyToken (s : String) : String :=
  let t := trimString s |>.toLower
  if t = "plaintiff" || t = "claimant" || t.contains "plaintiff" || t.contains "claimant" then "plaintiff"
  else if t = "defendant" || t = "defense" || t = "defence" || t.contains "defendant" || t.contains "defense" || t.contains "defence" then "defendant"
  else t

def getValidatedActorPartyField (a : CourtAction) (fieldName : String) : Except String String := do
  let actor := normalizePartyToken a.actor_role
  if actor != "plaintiff" && actor != "defendant" then
    throw s!"invalid party role: {a.actor_role}"
  let partyRaw ← getString a.payload fieldName
  let party := normalizePartyToken partyRaw
  if party != "plaintiff" && party != "defendant" then
    throw s!"invalid {fieldName}: {partyRaw}"
  if party != actor then
    throw s!"{fieldName} payload must match actor role: payload={partyRaw}, actor={a.actor_role}"
  pure actor

def appendDocket (c : CaseState) (title desc : String) : CaseState :=
  { c with docket := c.docket.concat { title := title, description := desc } }

def appendTrace (c : CaseState) (action outcome : String) (citations : List String) : CaseState :=
  { c with decision_traces := c.decision_traces.concat { action := action, outcome := outcome, citations := citations } }

def clearPassedOpportunities (s : CourtState) : CourtState :=
  { s with passed_opportunities := [] }

def bumpStateVersion (s : CourtState) : CourtState :=
  { s with state_version := s.state_version + 1 }

def updateCase (s : CourtState) (c : CaseState) : CourtState :=
  clearPassedOpportunities { (bumpStateVersion s) with case := c }

def initializeComplaintAttachment (c : CaseState) (filedBy filedOn : String) (attachment : ComplaintAttachmentSeed) : Except String CaseState := do
  let fileId := trimString attachment.file_id
  if fileId = "" then
    throw "complaint attachment missing file_id"
  if c.case_files.any (fun entry =>
      match entry.getObjVal? "file_id" with
      | .ok value =>
          match value.getStr? with
          | .ok s => s = fileId
          | .error _ => false
      | .error _ => false) then
    throw s!"duplicate complaint attachment file_id: {fileId}"
  let originalName := trimString attachment.original_name
  if originalName = "" then
    throw s!"complaint attachment {fileId} missing original_name"
  let record := Json.mkObj [
    ("file_id", toJson fileId),
    ("imported_at", toJson filedOn),
    ("imported_by", toJson filedBy),
    ("label", toJson (trimString attachment.label)),
    ("original_name", toJson originalName),
    ("storage_relpath", toJson (trimString attachment.storage_relpath)),
    ("sha256", toJson (trimString attachment.sha256)),
    ("size_bytes", toJson attachment.size_bytes)
  ]
  let c1 := { c with case_files := c.case_files.concat record }
  let event := Json.mkObj [
    ("recorded_at", toJson filedOn),
    ("action", toJson "filed_with_complaint"),
    ("file_id", toJson fileId),
    ("actor", toJson filedBy),
    ("details", toJson s!"complaint_attachment={originalName}")
  ]
  let c2 := { c1 with file_events := c1.file_events.concat event }
  let desc :=
    if trimString attachment.label = "" || trimString attachment.label = originalName then
      s!"{filedBy}: {fileId} {originalName}"
    else
      s!"{filedBy}: {fileId} {trimString attachment.label} / {originalName}"
  pure <| appendDocket c2 "Complaint attachment filed" desc

def initializeCase (req : InitializeCaseRequest) : Except String CourtState := do
  let c := req.state.case
  if c.decision_traces.any (fun t => t.action = "file_complaint") then
    throw "case already initialized"
  let filedBy := normalizePartyToken req.filed_by
  if filedBy != "plaintiff" then
    throw s!"invalid filed_by for case initialization: {req.filed_by}"
  let summary := trimString req.complaint_summary
  if summary = "" then
    throw "complaint_summary is required"
  let juryDemandedOn :=
    if trimString req.jury_demanded_on = "" then c.jury_demanded_on else trimString req.jury_demanded_on
  let jurisdictionalAllegations :=
    match req.jurisdictional_allegations with
    | some value => some value
    | none => c.jurisdictional_allegations
  let c1 := appendTrace (appendDocket { c with
      jury_demanded_on := juryDemandedOn
      jurisdictional_allegations := jurisdictionalAllegations
    } "Complaint filed" summary) "file_complaint" "filed" ["FRCP 3", "FRCP 8(a)"]
  let c2 ← req.attachments.foldlM (fun acc attachment => initializeComplaintAttachment acc filedBy c.filed_on attachment) c1
  pure <| updateCase req.state c2

def recordOpportunityPass (s : CourtState) (opportunityId : String) : CourtState :=
  { (bumpStateVersion s) with
      passed_opportunities := s.passed_opportunities.concat opportunityId
  }

def requiredPayloadString? (constraints : Json) (field : String) : Option String :=
  match constraints.getObjVal? "required_payload" with
  | .ok required =>
      match required.getObjVal? field with
      | .ok value =>
          match value.getStr? with
          | .ok s => some (trimString s)
          | .error _ => none
      | .error _ => none
  | .error _ => none

def closeRule56WindowFor (c : CaseState) (party : String) : CaseState :=
  let normalized := normalizePartyToken party
  if normalized = "" || c.rule56_window_closed_for.contains normalized then
    c
  else
    { c with rule56_window_closed_for := c.rule56_window_closed_for.concat normalized }

def reopenRule56Windows (c : CaseState) : CaseState :=
  { c with rule56_window_closed_for := [] }

def applyOpportunityPassToCase? (c : CaseState) (opportunity : OpportunitySpec) : Option CaseState :=
  if opportunity.allowed_tools = ["file_rule56_motion"] then
    some (closeRule56WindowFor c opportunity.role)
  else if opportunity.allowed_tools = ["record_voir_dire_question"] then
    match requiredPayloadString? opportunity.constraints "asked_by",
          requiredPayloadString? opportunity.constraints "juror_id" with
    | some askedBy, some jurorId =>
        some (appendTrace c "pass_voir_dire_question" s!"{normalizePartyToken askedBy}:{jurorId}" ["FRCP 47(a)"])
    | _, _ => none
  else if opportunity.allowed_tools = ["challenge_juror_for_cause"] then
    some (appendTrace c "pass_juror_for_cause_challenge" (normalizePartyToken opportunity.role) ["FRCP 47(a)"])
  else if opportunity.allowed_tools = ["strike_juror_peremptorily"] then
    some (appendTrace c "pass_juror_peremptory_strike" (normalizePartyToken opportunity.role) ["FRCP 47"])
  else
    none

def recordOpportunityPassFor (s : CourtState) (opportunity : OpportunitySpec) : CourtState :=
  if let some c1 := applyOpportunityPassToCase? s.case opportunity then
    updateCase s c1
  else
    recordOpportunityPass s opportunity.opportunity_id

def jsonObjectPairs (j : Json) : List (String × Json) :=
  match j.getObj? with
  | .ok kvs => (kvs.foldl (init := []) fun acc k v => (k, v) :: acc).reverse
  | .error _ => []

def constraintObjectPairs (constraints : Json) (field : String) : List (String × Json) :=
  match constraints.getObjVal? field with
  | .ok value => jsonObjectPairs value
  | .error _ => []

def applyPayloadDefaults (payload constraints : Json) : Json :=
  let payloadPairs := jsonObjectPairs payload
  let payloadJson := Json.mkObj payloadPairs
  let defaults := constraintObjectPairs constraints "payload_defaults"
  let missingDefaults :=
    defaults.foldl (init := []) fun acc (k, v) =>
      match payloadJson.getObjVal? k with
      | .ok _ => acc
      | .error _ => acc.concat (k, v)
  Json.mkObj (payloadPairs ++ missingDefaults)

def firstRequiredPayloadViolation? (payload constraints : Json) : Option (String × Json) :=
  let required := constraintObjectPairs constraints "required_payload"
  required.find? fun (k, expected) =>
    match payload.getObjVal? k with
    | .ok actual => actual != expected
    | .error _ => true

def checkTransition (current next : String) : Bool :=
  (current = "filed" && (next = "pretrial" || next = "closed")) ||
  (current = "pretrial" && (next = "trial" || next = "closed")) ||
  (current = "trial" && (next = "judgment_entered" || next = "closed")) ||
  (current = "judgment_entered" && next = "closed")

def parseMinConcurring (payload : Json) : Nat :=
  match payload.getObjVal? "minimum_concurring" with
  | .ok v =>
      match v.getNat? with
      | .ok n => n
      | .error _ => 0
  | .error _ => 0

def countCandidates (jurors : List JurorRecord) : Nat :=
  jurors.foldl (fun acc j => if j.status = "candidate" then acc + 1 else acc) 0

def swearAvailableLoop (remaining : Nat) (jurors : List JurorRecord) : List JurorRecord :=
  match jurors with
  | [] => []
  | j :: rest =>
      if remaining > 0 && j.status = "candidate" then
        { j with status := "sworn" } :: swearAvailableLoop (remaining - 1) rest
      else
        j :: swearAvailableLoop remaining rest

def swearAvailable (jurors : List JurorRecord) (needed : Nat) : List JurorRecord :=
  swearAvailableLoop needed jurors

def jurorExists (jurors : List JurorRecord) (jurorId : String) : Bool :=
  jurors.any (fun j => j.juror_id = jurorId)

def setJurorStatus (jurors : List JurorRecord) (jurorId status : String) : List JurorRecord :=
  jurors.map (fun j => if j.juror_id = jurorId then { j with status := status } else j)

def jurorById? (jurors : List JurorRecord) (jurorId : String) : Option JurorRecord :=
  jurors.find? (fun j => j.juror_id = jurorId)

def nextVoirDireExchangeId (exchanges : List VoirDireExchange) : String :=
  s!"vdq-{exchanges.length + 1}"

def defaultJurorQuestionnaire (_c : CaseState) : List JurorQuestionnaireItem :=
  [
    { question_id := "Q1", question := "Can you give close attention to the evidence, follow the judge's instructions, and decide the case fairly and impartially?" },
    { question_id := "Q2", question := "Do you have any hardship, scheduling, health, caregiving, or work concern that would make it difficult to serve as a juror through verdict?" },
    { question_id := "Q3", question := "Can you apply the preponderance-of-the-evidence standard even if you would personally prefer a higher burden of proof?" },
    { question_id := "Q4", question := "Do you have any strong views about evaluating documentary, electronic, or signed evidence, including digital signatures or authenticated records, that could affect your impartiality?" },
    { question_id := "Q5", question := "Do you have any strong views about whether people or businesses may rely on another person's statements in a civil case involving alleged misrepresentation or reliance?" },
    { question_id := "Q6", question := "Do you have any strong views about misrepresentation, deception, intent, or credibility that could affect your evaluation of the evidence in this case?" },
    { question_id := "Q7", question := "Can you follow instructions on damages and award damages if proven, or deny damages if not proven?" },
    { question_id := "Q8", question := "Do you have prior experience with consulting, freelance, contract, or business-service disputes that could affect your impartiality in this case?" }
  ]

def jurorQuestionnaireIssued (c : CaseState) : Bool :=
  !c.juror_questionnaire.isEmpty

def questionnaireQuestionIds (items : List JurorQuestionnaireItem) : List String :=
  items.map (fun item => item.question_id)

def questionnaireAnswerIds (answers : List JurorQuestionnaireAnswer) : List String :=
  answers.map (fun answer => answer.question_id)

def questionnaireAnswersMatch (questions : List JurorQuestionnaireItem) (answers : List JurorQuestionnaireAnswer) : Bool :=
  let expected := questionnaireQuestionIds questions
  let actual := questionnaireAnswerIds answers
  expected.length = actual.length &&
  expected.all (fun questionId =>
    actual.contains questionId &&
    answers.any (fun answer => answer.question_id = questionId && trimString answer.answer != ""))

def hasQuestionnaireResponseFor (c : CaseState) (jurorId : String) : Bool :=
  c.juror_questionnaire_responses.any (fun response => response.juror_id = jurorId)

def nextCandidateWithoutQuestionnaireResponse? (c : CaseState) : Option JurorRecord :=
  c.jurors.find? (fun juror =>
    juror.status = "candidate" &&
    !hasQuestionnaireResponseFor c juror.juror_id)

def jurorAvailableForVoirDire (c : CaseState) (jurorId : String) : Bool :=
  match jurorById? c.jurors jurorId with
  | some juror => juror.status = "candidate"
  | none => false

def countAnsweredVoirDireQuestionsFrom (c : CaseState) (jurorId askedBy : String) : Nat :=
  c.voir_dire_exchanges.foldl (fun acc x =>
    if x.juror_id = jurorId &&
        normalizePartyToken x.asked_by = normalizePartyToken askedBy &&
        trimString x.response != "" then
      acc + 1
    else
      acc) 0

def nextPendingVoirDireExchange? (c : CaseState) : Option VoirDireExchange :=
  c.voir_dire_exchanges.find? (fun x =>
    x.judge_allowed = some true &&
    trimString x.response = "" &&
    jurorAvailableForVoirDire c x.juror_id)

def nextPendingVoirDireRuling? (c : CaseState) : Option VoirDireExchange :=
  c.voir_dire_exchanges.find? (fun x =>
    x.judge_allowed.isNone &&
    jurorAvailableForVoirDire c x.juror_id)

def countDisallowedVoirDireQuestionsFrom (c : CaseState) (askedBy : String) : Nat :=
  c.voir_dire_exchanges.foldl (fun acc x =>
    if normalizePartyToken x.asked_by = normalizePartyToken askedBy &&
        x.judge_allowed = some false then
      acc + 1
    else
      acc) 0

def hasPendingVoirDireQuestionFrom (c : CaseState) (jurorId askedBy : String) : Bool :=
  c.voir_dire_exchanges.any (fun exchange =>
    exchange.juror_id = jurorId &&
    normalizePartyToken exchange.asked_by = normalizePartyToken askedBy &&
    (exchange.judge_allowed.isNone ||
      (exchange.judge_allowed = some true && trimString exchange.response = "")))

def hasDecisionTraceOutcome (c : CaseState) (action outcome : String) : Bool :=
  c.decision_traces.any (fun t => t.action = action && t.outcome = outcome)

def hasVoirDireQuestionPassBy (c : CaseState) (jurorId askedBy : String) : Bool :=
  hasDecisionTraceOutcome c "pass_voir_dire_question" s!"{normalizePartyToken askedBy}:{jurorId}"

def nextAvailableJurorNeedingQuestionFrom? (c : CaseState) (askedBy : String) (maxQuestions : Nat) : Option JurorRecord :=
  c.jurors.find? (fun juror =>
    juror.status = "candidate" &&
    hasQuestionnaireResponseFor c juror.juror_id &&
    !hasVoirDireQuestionPassBy c juror.juror_id askedBy &&
    countAnsweredVoirDireQuestionsFrom c juror.juror_id askedBy < maxQuestions)

def partyFinishedVoirDireForJuror (c : CaseState) (jurorId askedBy : String) (maxQuestions maxDisallowed : Nat) : Bool :=
  hasVoirDireQuestionPassBy c jurorId askedBy ||
  countAnsweredVoirDireQuestionsFrom c jurorId askedBy >= maxQuestions ||
  countDisallowedVoirDireQuestionsFrom c askedBy >= maxDisallowed

def jurorReadyForEmpanelment (c : CaseState) (jurorId : String) (maxQuestions maxDisallowed : Nat) : Bool :=
  hasQuestionnaireResponseFor c jurorId &&
  partyFinishedVoirDireForJuror c jurorId "plaintiff" maxQuestions maxDisallowed &&
  partyFinishedVoirDireForJuror c jurorId "defendant" maxQuestions maxDisallowed

def allAvailableJurorsReadyForEmpanelment (c : CaseState) (maxQuestions maxDisallowed : Nat) : Bool :=
  c.jurors.all (fun juror =>
    juror.status != "candidate" || jurorReadyForEmpanelment c juror.juror_id maxQuestions maxDisallowed)

def nextForCauseChallengeId (challenges : List ForCauseChallenge) : String :=
  s!"vdc-{challenges.length + 1}"

def nextPendingForCauseChallenge? (c : CaseState) : Option ForCauseChallenge :=
  c.for_cause_challenges.find? (fun challenge =>
    challenge.granted.isNone &&
    jurorAvailableForVoirDire c challenge.juror_id)

def hasForCauseChallengeRequestBy (c : CaseState) (party jurorId : String) : Bool :=
  c.for_cause_challenges.any (fun challenge =>
    normalizePartyToken challenge.by_party = normalizePartyToken party &&
    challenge.juror_id = jurorId)

def countForCauseChallengesRequestedBy (c : CaseState) (party : String) : Nat :=
  c.for_cause_challenges.foldl (fun acc challenge =>
    if normalizePartyToken challenge.by_party = normalizePartyToken party then acc + 1 else acc) 0

def nextAvailableJurorWithoutForCauseRequestBy? (c : CaseState) (party : String) : Option JurorRecord :=
  c.jurors.find? (fun juror =>
    juror.status = "candidate" &&
    !hasForCauseChallengeRequestBy c party juror.juror_id)

def nextAvailableJurorWithoutPeremptoryStrikeBy? (c : CaseState) (party : String) : Option JurorRecord :=
  c.jurors.find? (fun juror =>
    juror.status = "candidate" &&
    !c.decision_traces.any (fun trace =>
      trace.action = "strike_juror_peremptorily" &&
      trace.outcome = s!"{normalizePartyToken party}:{juror.juror_id}"))

def countJurorsByStatus (jurors : List JurorRecord) (status : String) : Nat :=
  jurors.foldl (fun acc juror => if juror.status = status then acc + 1 else acc) 0

def currentDeliberationRound (c : CaseState) : Nat :=
  if c.deliberation_round = 0 then 1 else c.deliberation_round

def jurorVotesForRound (c : CaseState) (round : Nat) : List JurorVote :=
  c.juror_votes.filter (fun v => v.round = round)

def hasJurorVoteForRound (c : CaseState) (round : Nat) (jurorId : String) : Bool :=
  (jurorVotesForRound c round).any (fun v => v.juror_id = jurorId)

def nextSwornJurorWithoutVoteInRound? (c : CaseState) (round : Nat) : Option JurorRecord :=
  c.jurors.find? (fun j => j.status = "sworn" && !hasJurorVoteForRound c round j.juror_id)

def findJurorVoteForRound? (c : CaseState) (round : Nat) (jurorId : String) : Option JurorVote :=
  (jurorVotesForRound c round).find? (fun vote => vote.juror_id = jurorId)

def deliberationRoundStableComparedToPrevious (c : CaseState) (round : Nat) : Bool :=
  if round <= 1 then
    false
  else
    let previousRound := round - 1
    let currentVotes := jurorVotesForRound c round
    let previousVotes := jurorVotesForRound c previousRound
    if currentVotes.length != previousVotes.length then
      false
    else
      currentVotes.all (fun currentVote =>
        match previousVotes.find? (fun vote => vote.juror_id = currentVote.juror_id) with
        | none => false
        | some previousVote =>
            trimString currentVote.vote = trimString previousVote.vote &&
            currentVote.damages == previousVote.damages)

def countVotesFor (votes : List JurorVote) (side : String) : Nat :=
  votes.foldl (fun acc vote =>
    if trimString vote.vote = trimString side then acc + 1 else acc) 0

def meanDamagesForVotes (votes : List JurorVote) : Float :=
  if votes.isEmpty then
    0.0
  else
    let total := votes.foldl (fun acc vote => acc + vote.damages) 0.0
    total / Float.ofNat votes.length

def effectiveMinimumConcurring (cfg : JuryConfiguration) (swornCount : Nat) : Nat :=
  if swornCount = 0 then
    0
  else if swornCount < cfg.minimum_concurring then
    swornCount
  else
    cfg.minimum_concurring

def deriveVerdictFromJurorVotes? (policy : CourtPolicy) (c : CaseState) :
    Option (Option JuryVerdict × Option HungJury × Option Nat × Option String) :=
  let round := currentDeliberationRound c
  match c.jury_configuration with
  | none => none
  | some cfg =>
      let swornCount := countJurorsByStatus c.jurors "sworn"
      let required := effectiveMinimumConcurring cfg swornCount
      if swornCount = 0 then
        some (none, some {
          claim_id := "claim-1"
          note := s!"no sworn jurors remained eligible to deliberate in round {round}"
        }, none, none)
      else if nextSwornJurorWithoutVoteInRound? c round |>.isSome then
        none
      else
        let currentVotes := jurorVotesForRound c round
        let plaintiffVotes := currentVotes.filter (fun vote => trimString vote.vote = "plaintiff")
        let defendantVotes := currentVotes.filter (fun vote => trimString vote.vote = "defendant")
        let plaintiffCount := plaintiffVotes.length
        let defendantCount := defendantVotes.length
        if plaintiffCount >= required && defendantCount < required then
          some (some {
            verdict_for := "plaintiff"
            votes_for_verdict := plaintiffCount
            required_votes := required
            damages := meanDamagesForVotes plaintiffVotes
          }, none, none, none)
        else if defendantCount >= required && plaintiffCount < required then
          some (some {
            verdict_for := "defendant"
            votes_for_verdict := defendantCount
            required_votes := required
            damages := 0.0
          }, none, none, none)
        else if round >= policy.max_deliberation_rounds then
          some (none, some {
            claim_id := "claim-1"
            note := s!"sworn jurors remained split after {round} deliberation rounds"
          }, none, none)
        else if deliberationRoundStableComparedToPrevious c round then
          some (none, some {
            claim_id := "claim-1"
            note := s!"sworn jurors remained split after deliberation round {round}, and no juror changed vote or damages from the prior round"
          }, none, none)
        else
          let supplemental :=
            if round = 1 then
              some "Review the evidence and the court's instructions again. Consider the other jurors' stated reasons with care. Do not surrender an honestly held view solely to reach a verdict."
            else
              none
          some (none, none, some (round + 1), supplemental)

def applyDerivedDeliberationOutcome (policy : CourtPolicy) (c : CaseState) : CaseState :=
  let round := currentDeliberationRound c
  match deriveVerdictFromJurorVotes? policy c with
  | some (some verdict, none, none, none) =>
      appendDocket { c with jury_verdict := some verdict } "Jury verdict derived"
        s!"verdict_for={verdict.verdict_for} votes_for_verdict={verdict.votes_for_verdict} required_votes={verdict.required_votes} damages={verdict.damages}"
  | some (none, some hung, none, none) =>
      appendDocket { c with hung_jury := some hung } "Hung jury notice" hung.note
  | some (none, none, some nextRound, supplemental) =>
      let cRound := { c with deliberation_round := nextRound }
      let cRound := appendDocket cRound "Jury ballot round completed"
        s!"round={round} plaintiff_votes={countVotesFor (jurorVotesForRound c round) "plaintiff"} defendant_votes={countVotesFor (jurorVotesForRound c round) "defendant"}"
      match supplemental with
      | some text =>
          appendDocket cRound "Jury supplemental instruction" text
      | none =>
          appendDocket cRound "Jury deliberation continues" s!"advance to round {nextRound}"
  | _ => c

def empanelSelectedJurors (jurors : List JurorRecord) (selected : List String) : List JurorRecord :=
  jurors.map (fun juror =>
    if selected.contains juror.juror_id then
      { juror with status := "sworn" }
    else if juror.status = "candidate" then
      { juror with status := "excused_after_voir_dire" }
    else
      juror)

def assignProtectiveOrderId (orders : List ProtectiveOrder) : String :=
  s!"po-{orders.length + 1}"

def liftProtectiveOrder (orders : List ProtectiveOrder) (orderId note liftedAt : String) :
    Option (List ProtectiveOrder) :=
  let rec loop (remaining : List ProtectiveOrder) : Bool → List ProtectiveOrder → Option (List ProtectiveOrder)
    | found, acc =>
      match remaining with
      | [] =>
        if found then some acc.reverse else none
      | o :: rest =>
        if o.order_id = orderId then
          let updated :=
            { o with
                active := false
                lifted_at := some liftedAt
                note := if note = "" then o.note else note }
          loop rest true (updated :: acc)
        else
          loop rest found (o :: acc)
  loop orders false []

def setRule68OfferAt : List Rule68Offer → Nat → Rule68Offer → List Rule68Offer
  | [], _, _ => []
  | _ :: rest, 0, next => next :: rest
  | x :: rest, idx + 1, next => x :: setRule68OfferAt rest idx next

def getRule68OfferAt : List Rule68Offer → Nat → Option Rule68Offer
  | [], _ => none
  | x :: _, 0 => some x
  | _ :: rest, idx + 1 => getRule68OfferAt rest idx

def findRule68OfferIndexById (offers : List Rule68Offer) (offerId : String) : Option Nat :=
  let rec loop (remaining : List Rule68Offer) (idx : Nat) : Option Nat :=
    match remaining with
    | [] => none
    | x :: rest =>
        if x.offer_id = offerId then some idx else loop rest (idx + 1)
  loop offers 0

def incrementContemptCount (counts : List ContemptCounter) (targetRole : String) : List ContemptCounter :=
  match counts with
  | [] => [{ role := targetRole, count := 1 }]
  | x :: rest =>
      if x.role = targetRole then
        { x with count := x.count + 1 } :: rest
      else
        x :: incrementContemptCount rest targetRole

def sumContemptCounts : List ContemptCounter → Nat
  | [] => 0
  | c :: rest => c.count + sumContemptCounts rest

def contemptCountFor : List ContemptCounter → String → Nat
  | [], _ => 0
  | c :: rest, role =>
      (if c.role = role then c.count else 0) + contemptCountFor rest role

def requireSingleClaimMetadata (c : CaseState) : Except String Unit := do
  let claim ← match c.single_claim with
    | some j => pure j
    | none => throw "single_claim metadata required before verdict"
  let claimId ← getString claim "claim_id"
  if claimId = "" then
    throw "single_claim.claim_id must be non-empty"
  let _label ← getString claim "label"
  let _theory ← getString claim "legal_theory"
  let standard ← getString claim "standard_of_proof"
  if !(standard = "preponderance_of_the_evidence" || standard = "clear_and_convincing") then
    throw s!"unsupported single_claim.standard_of_proof: {standard}"
  let burdenHolder ← getString claim "burden_holder"
  let burden := normalizePartyToken burdenHolder
  if !(burden = "plaintiff" || burden = "defendant") then
    throw s!"invalid single_claim.burden_holder: {burdenHolder}"
  let _elements ← claim.getObjVal? "elements"
  let _defenses ← claim.getObjVal? "defenses"
  let _damages ← getString claim "damages_question"
  pure ()

def getSingleClaimId (c : CaseState) : Except String String := do
  let claim ← match c.single_claim with
    | some j => pure j
    | none => throw "single_claim metadata required before verdict"
  getString claim "claim_id"

def requireClaimIdMatch (c : CaseState) (payload : Json) : Except String Unit := do
  let payloadClaimId ← getString payload "claim_id"
  let expectedClaimId ← getSingleClaimId c
  if payloadClaimId != expectedClaimId then
    throw s!"claim_id mismatch: payload={payloadClaimId}, expected={expectedClaimId}"
  pure ()

def hasSwornJuror (jurors : List JurorRecord) : Bool :=
  jurors.any (fun j => j.status = "sworn")

def validateComparativeFault (payload : Json) : Except String Unit := do
  let usedOpt ← getBoolOpt payload "comparative_fault_used"
  let plaintiffOpt ← getNatOpt payload "plaintiff_fault_pct"
  let defendantOpt ← getNatOpt payload "defendant_fault_pct"
  match usedOpt with
  | some true =>
      let plaintiff ← match plaintiffOpt with
        | some n => pure n
        | none => throw "comparative fault requires plaintiff_fault_pct"
      let defendant ← match defendantOpt with
        | some n => pure n
        | none => throw "comparative fault requires defendant_fault_pct"
      if plaintiff + defendant != 100 then
        throw "comparative fault percentages must total 100"
      pure ()
  | some false =>
      if plaintiffOpt.isSome || defendantOpt.isSome then
        throw "comparative fault percentages not allowed when comparative_fault_used is false"
      pure ()
  | none =>
      if plaintiffOpt.isSome || defendantOpt.isSome then
        throw "comparative fault percentages require comparative_fault_used"
      pure ()

def roleOwnDocketEntries (docket : List DocketEntry) (role : String) : List DocketEntry :=
  docket.filter (fun e =>
    e.title.startsWith s!"Opening statement - {role}" ||
    e.title.startsWith s!"Trial theory - {role}" ||
    e.title.startsWith s!"Rebuttal presentation - {role}" ||
    e.title.startsWith s!"Surrebuttal presentation - {role}" ||
    e.title.startsWith s!"Closing argument - {role}" ||
    e.title.startsWith s!"Closing rebuttal - {role}" ||
    e.description.startsWith s!"{role}:")

def jsonStringFieldD (entry : Json) (key fallback : String) : String :=
  match entry.getObjVal? key with
  | .ok value =>
      match value.getStr? with
      | .ok s => s
      | .error _ => fallback
  | .error _ => fallback

def jsonNatFieldD (entry : Json) (key : String) (fallback : Nat) : Nat :=
  match entry.getObjVal? key with
  | .ok value =>
      match value.getNat? with
      | .ok n => n
      | .error _ => fallback
  | .error _ => fallback

def courtProfileStringD (s : CourtState) (key fallback : String) : String :=
  match s.court_profile with
  | some entry => jsonStringFieldD entry key fallback
  | none => fallback

def courtProfileNatD (s : CourtState) (key : String) (fallback : Nat) : Nat :=
  match s.court_profile with
  | some entry => jsonNatFieldD entry key fallback
  | none => fallback

def courtProfileBoolD (s : CourtState) (key : String) (fallback : Bool) : Bool :=
  match s.court_profile with
  | some entry =>
      match entry.getObjVal? key with
      | .ok value =>
          match value.getBool? with
          | .ok b => b
          | .error _ => fallback
      | .error _ => fallback
  | none => fallback

def courtUsesJurisdictionScreen (s : CourtState) : Bool :=
  courtProfileBoolD s "jurisdiction_screen" true

def courtRequiresJurisdictionStatement (s : CourtState) : Bool :=
  courtProfileBoolD s "require_jurisdiction_statement" true

def courtRequiresDiversityCitizenship (s : CourtState) : Bool :=
  courtProfileBoolD s "require_diversity_citizenship" true

def courtRequiresAmountInControversy (s : CourtState) : Bool :=
  courtProfileBoolD s "require_amount_in_controversy" true

def courtMinimumAmountInControversy (s : CourtState) : Nat :=
  courtProfileNatD s "minimum_amount_in_controversy" 75000

def amountInControversyNat? (raw : String) : Option Nat :=
  let whole := ((trimString raw).splitOn ".").headD ""
  let digits := whole.foldl (fun acc ch => if ch.isDigit then acc.push ch else acc) ""
  if digits = "" then none else digits.toNat?

def jurisdictionalFieldD (c : CaseState) (key fallback : String) : String :=
  match c.jurisdictional_allegations with
  | some entry => jsonStringFieldD entry key fallback
  | none => fallback

def hasJurisdictionalAllegations (c : CaseState) : Bool :=
  c.jurisdictional_allegations.isSome

def subjectMatterJurisdictionFaciallyDefective (s : CourtState) : Bool :=
  let c := s.case
  if !courtUsesJurisdictionScreen s then
    false
  else if !hasJurisdictionalAllegations c then
    false
  else
    let basis := jurisdictionalFieldD c "jurisdiction_basis" "" |> trimString
    let statement := jurisdictionalFieldD c "jurisdictional_statement" "" |> trimString
    if basis = "" || basis = "unspecified" then
      true
    else if courtRequiresJurisdictionStatement s && statement = "" then
      true
    else if basis = "diversity" then
      let plaintiffCitizenship := jurisdictionalFieldD c "plaintiff_citizenship" "" |> trimString
      let defendantCitizenship := jurisdictionalFieldD c "defendant_citizenship" "" |> trimString
      let amountInControversy := jurisdictionalFieldD c "amount_in_controversy" "" |> trimString
      let citizenshipDefective :=
        courtRequiresDiversityCitizenship s &&
          (plaintiffCitizenship = "" || defendantCitizenship = "")
      let amountDefective :=
        if !courtRequiresAmountInControversy s then
          false
        else
          amountInControversy = "" ||
            match amountInControversyNat? amountInControversy with
            | some amount => amount <= courtMinimumAmountInControversy s
            | none => true
      citizenshipDefective || amountDefective
    else
      false

def subjectMatterJurisdictionFaciallyDefectiveCase (s : CourtState) (c : CaseState) : Bool :=
  subjectMatterJurisdictionFaciallyDefective { s with case := c }

def rule56WindowClosedFor (c : CaseState) (party : String) : Bool :=
  c.rule56_window_closed_for.contains (normalizePartyToken party)

def publicCaseFileJson (entry : Json) : Json :=
  Json.mkObj [
    ("file_id", toJson (jsonStringFieldD entry "file_id" "")),
    ("imported_at", toJson (jsonStringFieldD entry "imported_at" "")),
    ("imported_by", toJson (jsonStringFieldD entry "imported_by" "")),
    ("label", toJson (jsonStringFieldD entry "label" "")),
    ("original_name", toJson (jsonStringFieldD entry "original_name" "")),
    ("sha256", toJson (jsonStringFieldD entry "sha256" "")),
    ("size_bytes", toJson (jsonNatFieldD entry "size_bytes" 0))
  ]

def publicCaseJson (c : CaseState) : Json :=
  Json.mkObj [
    ("case_id", toJson c.case_id),
    ("caption", toJson c.caption),
    ("judge", toJson c.judge),
    ("filed_on", toJson c.filed_on),
    ("status", toJson c.status),
    ("trial_mode", toJson c.trial_mode),
    ("phase", toJson c.phase),
    ("last_pleading_served_on", toJson c.last_pleading_served_on),
    ("jury_demanded_on", toJson c.jury_demanded_on),
    ("jury_configuration", toJson c.jury_configuration),
    ("single_claim", toJson c.single_claim),
    ("jurisdictional_allegations", toJson c.jurisdictional_allegations),
    ("jury_verdict", toJson c.jury_verdict),
    ("hung_jury", toJson c.hung_jury),
    ("local_rule_overrides", toJson c.local_rule_overrides),
    ("limit_usage", toJson c.limit_usage),
    ("case_files", toJson (c.case_files.map publicCaseFileJson)),
    ("docket", toJson c.docket),
    ("decision_traces", toJson c.decision_traces)
  ]

def viewForRole (s : CourtState) (role : String) : Except String Json := do
  if s.schema_version != "v1" then
    throw "unsupported schema version"
  let normalizedRole := normalizePartyToken role
  if normalizedRole = "judge" || normalizedRole = "clerk" then
    pure <| Json.mkObj [
      ("role", toJson normalizedRole),
      ("state", toJson s),
      ("redactions", toJson ([] : List String)),
      ("role_private", Json.mkObj [])
    ]
  else if normalizedRole = "plaintiff" || normalizedRole = "defendant" then
    pure <| Json.mkObj [
      ("role", toJson normalizedRole),
      ("state", Json.mkObj [
        ("schema_version", toJson s.schema_version),
        ("court_name", toJson s.court_name),
        ("court_profile", toJson s.court_profile),
        ("policy", toJson s.policy),
        ("case", publicCaseJson s.case)
      ]),
      ("redactions", toJson ["case.jurors", "case.voir_dire_exchanges", "case.for_cause_challenges", "case.juror_votes", "case.juror_explanations", "case.contempt_counts"]),
      ("role_private", Json.mkObj [("own_docket_entries", toJson (roleOwnDocketEntries s.case.docket normalizedRole))])
    ]
  else
    let redactions :=
      ["case.jurors", "case.voir_dire_exchanges", "case.for_cause_challenges", "case.juror_votes", "case.juror_explanations", "case.contempt_counts"]
    pure <| Json.mkObj [
      ("role", toJson normalizedRole),
      ("state", Json.mkObj [
        ("schema_version", toJson s.schema_version),
        ("court_name", toJson s.court_name),
        ("policy", toJson s.policy),
        ("case", publicCaseJson s.case)
      ]),
      ("redactions", toJson redactions),
      ("role_private", Json.mkObj [])
    ]

def hasDecisionTraceAction (c : CaseState) (action : String) : Bool :=
  c.decision_traces.any (fun t => t.action = action)

def countDecisionTraceOutcomesWithPrefix (c : CaseState) (action : String) (prefixText : String) : Nat :=
  c.decision_traces.foldl (fun acc trace =>
    if trace.action = action && trace.outcome.startsWith prefixText then acc + 1 else acc) 0

def hasJurorExplanationFor (c : CaseState) (jurorId : String) : Bool :=
  c.juror_explanations.any (fun j => j.juror_id = jurorId)

def nextSwornJurorWithoutExplanation? (c : CaseState) : Option JurorRecord :=
  c.jurors.find? (fun j => j.status = "sworn" && !hasJurorExplanationFor c j.juror_id)

def countForCauseChallengesBy (c : CaseState) (party : String) : Nat :=
  countForCauseChallengesRequestedBy c party

def countPeremptoryChallengesBy (c : CaseState) (party : String) : Nat :=
  countDecisionTraceOutcomesWithPrefix c "strike_juror_peremptorily" s!"{normalizePartyToken party}:"

def hasForCauseChallengePassBy (c : CaseState) (party : String) : Bool :=
  hasDecisionTraceOutcome c "pass_juror_for_cause_challenge" (normalizePartyToken party)

def hasPeremptoryChallengePassBy (c : CaseState) (party : String) : Bool :=
  hasDecisionTraceOutcome c "pass_juror_peremptory_strike" (normalizePartyToken party)

def hasDocketTitle (c : CaseState) (title : String) : Bool :=
  c.docket.any (fun e => e.title = title)

def hasAnyDocketTitlePrefix (c : CaseState) (titlePrefix : String) : Bool :=
  c.docket.any (fun e => e.title.startsWith titlePrefix)

def effectiveVoirDireQuestionsPerSide (policy : CourtPolicy) : Nat :=
  if policy.max_voir_dire_questions_per_side_per_juror = 0 then 1 else policy.max_voir_dire_questions_per_side_per_juror

def skipVoirDire (policy : CourtPolicy) : Bool :=
  policy.skip_voir_dire != 0

def voirDireCandidateTarget (policy : CourtPolicy) (c : CaseState) : Nat :=
  match c.jury_configuration with
  | none => policy.voir_dire_candidate_count
  | some cfg =>
      let minimumNeeded :=
        cfg.juror_count + 2 * policy.max_for_cause_challenges_per_side + 2 * policy.max_peremptory_challenges_per_side
      if policy.voir_dire_candidate_count >= minimumNeeded then policy.voir_dire_candidate_count else minimumNeeded

def voirDirePanelReady (policy : CourtPolicy) (c : CaseState) : Bool :=
  countCandidates c.jurors >= voirDireCandidateTarget policy c

def juryEmpaneled (c : CaseState) : Bool :=
  match c.jury_configuration with
  | none => false
  | some cfg => countJurorsByStatus c.jurors "sworn" >= cfg.juror_count

def validateAdvanceTrialPhase (policy : CourtPolicy) (c : CaseState) (phase : String) : Except String Unit :=
  if c.status != "trial" then
    .error "trial phase advancement requires trial status"
  else if !(allowedPhases.contains phase) then
    .error s!"invalid phase: {phase}"
  else
    match parseTrialPhaseV1 c.phase with
    | none => .error s!"invalid current phase: {c.phase}"
    | some currentPhase =>
        match parseTrialPhaseV1 phase with
        | none => .error s!"invalid phase: {phase}"
        | some nextPhase =>
            if !(canAdvancePhaseV1 currentPhase nextPhase) then
              .error s!"cannot move backward from phase {c.phase} to {phase}"
            else if phase = "voir_dire" && c.trial_mode = "jury" &&
                !voirDirePanelReady policy c then
              .error "jury trial requires a full prospective juror panel before voir dire"
            else if phase = "openings" && c.trial_mode = "jury" &&
                !juryEmpaneled c then
              .error "jury trial requires an empaneled jury before openings"
            else if phase = "post_verdict" && c.trial_mode = "bench" && !hasDocketTitle c "Bench Opinion" then
              .error "bench trial requires Bench Opinion before post_verdict phase"
            else if phase = "post_verdict" && c.trial_mode = "jury" &&
                c.jury_verdict.isNone && c.hung_jury.isNone then
              .error "jury trial requires verdict or hung jury notice before post_verdict phase"
            else if phase = "deliberation" && c.trial_mode = "jury" &&
                !hasDocketTitle c "Jury instructions delivered" then
              .error "jury trial requires delivered jury instructions before deliberation phase"
            else
              .ok ()

def validateEnterJudgment (c : CaseState) : Except String Unit :=
  if c.status != "trial" then
    .error "judgment entry requires trial status"
  else if c.trial_mode = "bench" then
    if c.hung_jury.isSome then
      .error "cannot enter judgment after hung jury"
    else if !hasDocketTitle c "Bench Opinion" then
      .error "bench trial requires Bench Opinion before judgment"
    else
      .ok ()
  else
    match claimDispositionFromCaseStateV1 c with
    | .error e => .error e
    | .ok disposition =>
        if !canEnterJudgmentFromClaimDispositionV1 disposition then
          if disposition = ClaimDispositionV1.hung then
            .error "cannot enter judgment after hung jury"
          else
            .error "jury verdict required before judgment"
        else
          .ok ()

def validateBenchOpinion (c : CaseState) (text : String) : Except String Unit :=
  if c.status != "trial" then
    .error "bench opinion requires trial status"
  else
    match parseCurrentPhaseV1 c with
    | .error e => .error e
    | .ok currentPhase =>
        if !(currentPhase = TrialPhaseV1.verdictReturn || currentPhase = TrialPhaseV1.postVerdict) then
          .error s!"bench opinion requires verdict_return or post_verdict phase; current phase is {c.phase}"
        else if c.trial_mode != "bench" then
          .error "bench opinion is only available in bench trials"
        else if text.trimAscii.toString = "" then
          .error "bench opinion text must be non-empty"
        else
          .ok ()

def validateTrialActionPhase
    (c : CaseState)
    (action : TrialActionV1)
    (message : String) : Except String Unit :=
  match parseCurrentPhaseV1 c with
  | .error e => .error e
  | .ok currentPhase =>
      if !(phaseAllowsActionV1 action currentPhase) then
        .error message
      else
        .ok ()

def trialPresentationTitle (phase party : String) : Except String String := do
  if phase = "plaintiff_case" || phase = "defense_case" then
    pure s!"Trial theory - {party}"
  else if phase = "plaintiff_rebuttal" then
    pure s!"Rebuttal presentation - {party}"
  else if phase = "defense_surrebuttal" then
    pure s!"Surrebuttal presentation - {party}"
  else
    throw s!"presentation requires plaintiff_case, defense_case, plaintiff_rebuttal, or defense_surrebuttal phase; current phase is {phase}"

def applyAdvanceTrialPhase (s : CourtState) (phase : String) : Except String CourtState := do
  validateAdvanceTrialPhase s.policy s.case phase
  let c1 := appendDocket { s.case with phase := phase } s!"Phase: {phase}" s!"Court set phase to {phase}"
  pure <| updateCase s c1

def runAdvanceTrialPhase (s : CourtState) (payload : Json) : Except String CourtState := do
  let phase ← getString payload "phase"
  applyAdvanceTrialPhase s phase

def countExhibitsOfferedByParty (c : CaseState) (party : String) : Nat :=
  c.docket.foldl
    (fun acc e =>
      if e.title.startsWith "Exhibit " && e.description.startsWith s!"{party}:"
      then acc + 1
      else acc)
    0

def countDispositiveMotionsByParty (c : CaseState) (party : String) : Nat :=
  c.docket.foldl
    (fun acc e =>
      if e.title = "Rule 12 Motion" || e.title = "Rule 56 Motion" then
        if e.description.startsWith s!"{party}:" then acc + 1 else acc
      else
        acc)
    0

def countTechnicalReportsByParty (c : CaseState) (party : String) : Nat :=
  c.technical_reports.foldl
    (fun acc r => if normalizePartyToken r.party = party then acc + 1 else acc)
    0

def countDocketTitle (c : CaseState) (title : String) : Nat :=
  c.docket.foldl
    (fun acc e => if e.title = title then acc + 1 else acc)
    0

def countDocketTitleByPartyPrefix (c : CaseState) (title party : String) : Nat :=
  c.docket.foldl
    (fun acc e =>
      if e.title = title && e.description.startsWith party
      then acc + 1
      else acc)
    0

def jsonFieldEqString (entry : Json) (key : String) (expected : String) : Bool :=
  match entry.getObjVal? key with
  | .ok value =>
      match value.getStr? with
      | .ok s => s = expected
      | .error _ => false
  | .error _ => false

def hasCaseFileId (c : CaseState) (fileId : String) : Bool :=
  c.case_files.any (fun entry => jsonFieldEqString entry "file_id" fileId)

def appendFileEvent (c : CaseState) (recordedAt action fileId actor details : String) : CaseState :=
  let event := Json.mkObj [
    ("recorded_at", toJson recordedAt),
    ("action", toJson action),
    ("file_id", toJson fileId),
    ("actor", toJson actor),
    ("details", toJson details)
  ]
  { c with file_events := c.file_events.concat event }

def caseFileOfferedByParty (c : CaseState) (party fileId : String) : Bool :=
  c.file_events.any (fun entry =>
    jsonFieldEqString entry "action" "offer_case_file_as_exhibit" &&
    jsonFieldEqString entry "file_id" fileId &&
    normalizePartyToken (jsonStringFieldD entry "actor" "") = party)

def remainingCaseFilesForParty (c : CaseState) (party : String) : List Json :=
  c.case_files.filter (fun entry =>
    let fileId := jsonStringFieldD entry "file_id" ""
    fileId != "" && !caseFileOfferedByParty c party fileId)

def partyCanOfferMoreExhibits (c : CaseState) (party : String) (maxExhibits : Nat) : Bool :=
  countExhibitsOfferedByParty c party < maxExhibits &&
    !(remainingCaseFilesForParty c party).isEmpty

def evidenceRestTitle (phase : String) : Except String String := do
  match phase with
  | "plaintiff_evidence" => pure "Plaintiff case rested"
  | "defense_evidence" => pure "Defense case rested"
  | "plaintiff_rebuttal_evidence" => pure "Plaintiff rebuttal rested"
  | "defense_surrebuttal_evidence" => pure "Defense surrebuttal rested"
  | _ => throw s!"rest_case requires an evidence phase; current phase is {phase}"

def normalizePhaseForLimit (phase : String) : String :=
  if phase = "" then "none" else phase

def datePrefix (s : String) : String :=
  match s.splitOn "T" with
  | [] => s
  | head :: _ => head

def parseIsoDateParts (s : String) : Except String (Nat × Nat × Nat) := do
  let parts := (datePrefix s).splitOn "-"
  match parts with
  | [yRaw, mRaw, dRaw] =>
      let y ← match yRaw.toNat? with
        | some n => pure n
        | none => throw s!"invalid date year: {yRaw}"
      let m ← match mRaw.toNat? with
        | some n => pure n
        | none => throw s!"invalid date month: {mRaw}"
      let d ← match dRaw.toNat? with
        | some n => pure n
        | none => throw s!"invalid date day: {dRaw}"
      if m = 0 || m > 12 then
        throw s!"invalid date month: {m}"
      if d = 0 || d > 31 then
        throw s!"invalid date day: {d}"
      pure (y, m, d)
  | _ => throw s!"invalid ISO date: {s}"

def isLeapYear (y : Nat) : Bool :=
  (y % 400 = 0) || ((y % 4 = 0) && (y % 100 != 0))

def daysBeforeMonth (m : Nat) : Nat :=
  match m with
  | 1 => 0
  | 2 => 31
  | 3 => 59
  | 4 => 90
  | 5 => 120
  | 6 => 151
  | 7 => 181
  | 8 => 212
  | 9 => 243
  | 10 => 273
  | 11 => 304
  | 12 => 334
  | _ => 0

def ordinalDay (isoDate : String) : Except String Nat := do
  let (y, m, d) ← parseIsoDateParts isoDate
  let y1 := y - 1
  let leapAdj := if isLeapYear y && m > 2 then 1 else 0
  pure <| y1 * 365 + y1 / 4 - y1 / 100 + y1 / 400 + daysBeforeMonth m + leapAdj + d

def elapsedDaysBetween (servedIso respondedIso : String) : Except String Nat := do
  let servedOrd ← ordinalDay servedIso
  let respondedOrd ← ordinalDay respondedIso
  pure <| if respondedOrd >= servedOrd then respondedOrd - servedOrd else 0

def rule59WindowDays : Nat := 28

def rule60OneYearDays : Nat := 365

def rule60GroundHasOneYearLimit (ground : String) : Bool :=
  ground = "60b1_mistake" || ground = "60b2_new_evidence" || ground = "60b3_fraud"

def isRule59Timely (judgmentDate filedAt : String) : Except String Bool := do
  let elapsed ← elapsedDaysBetween judgmentDate filedAt
  pure <| elapsed ≤ rule59WindowDays

def isRule60Timely (judgmentDate filedAt ground : String) : Except String Bool := do
  let elapsed ← elapsedDaysBetween judgmentDate filedAt
  if rule60GroundHasOneYearLimit ground then
    pure <| elapsed ≤ rule60OneYearDays
  else
    pure true

def validateRule59Timing (judgmentDate filedAt : String) : Except String Unit :=
  match isRule59Timely judgmentDate filedAt with
  | .error e => .error e
  | .ok true => .ok ()
  | .ok false => .error "rule 59 motion is untimely"

def validateRule60Timing (judgmentDate filedAt ground : String) : Except String Unit :=
  match isRule60Timely judgmentDate filedAt ground with
  | .error e => .error e
  | .ok true => .ok ()
  | .ok false => .error "rule 60(b)(1)-(3) motion is untimely"

def stringContains (hay needle : String) : Bool :=
  if needle = "" then true else (hay.splitOn needle).length > 1

def isDiscoveryFilingText (s : String) : Bool :=
  let t := s.toLower
  stringContains t "discovery" || stringContains t "rule 26"

def setLimitUsage
    (usage : List LimitUsageV1)
    (limitKey actor phase : String)
    (value : Nat) : List LimitUsageV1 :=
  let rec loop (remaining : List LimitUsageV1) : List LimitUsageV1 :=
    match remaining with
    | [] => [{ limit_key := limitKey, actor := actor, phase := phase, value := value }]
    | x :: rest =>
        if x.limit_key = limitKey && x.actor = actor && x.phase = phase then
          { x with value := value } :: rest
        else
          x :: loop rest
  loop usage

def policyLimitValue (p : CourtPolicy) (limitKey : String) : Option Nat :=
  if limitKey = "text.opening_chars_per_side" then some p.max_opening_chars
  else if limitKey = "text.trial_theory_chars_per_side" then some p.max_trial_theory_chars
  else if limitKey = "text.closing_chars_per_side" then some p.max_closing_chars
  else if limitKey = "trial.exhibits_offered_per_side" then some p.max_exhibits_per_side
  else if limitKey = "motions.dispositive_motions_per_side_pretrial" then some p.max_dispositive_motions_per_side_pretrial
  else if limitKey = "discovery.interrogatories_per_set" then some p.max_interrogatories_per_set
  else if limitKey = "discovery.interrogatory_sets_per_side" then some p.max_interrogatory_sets_per_side
  else if limitKey = "discovery.rfp_requests_per_set" then some p.max_rfp_requests_per_set
  else if limitKey = "discovery.rfp_sets_per_side" then some p.max_rfp_sets_per_side
  else if limitKey = "discovery.rfa_requests_per_set" then some p.max_rfa_requests_per_set
  else if limitKey = "discovery.rfa_sets_per_side" then some p.max_rfa_sets_per_side
  else if limitKey = "discovery.response_deadline_days" then some p.max_discovery_response_deadline_days
  else if limitKey = "text.rule12_summary_chars" then some p.max_rule12_summary_chars
  else if limitKey = "text.rule56_summary_chars" then some p.max_rule56_summary_chars
  else if limitKey = "text.rule56_reply_chars" then some p.max_rule56_reply_chars
  else if limitKey = "reports.per_side_count" then some p.max_technical_reports_per_side
  else if limitKey = "reports.summary_chars_per_report" then some p.max_technical_report_summary_chars
  else none

def overrideApplies (o : LocalRuleOverrideV1) (actor phase nowIso : String) : Bool :=
  o.active &&
    (match o.expires_at with
    | some expiry => nowIso ≤ expiry
    | none => true) &&
    (match o.scope_party with
    | some p => normalizePartyToken p = actor
    | none => true) &&
    (match o.scope_phase with
    | some p => p = phase
    | none => true)

def overrideSpecificity (o : LocalRuleOverrideV1) : Nat :=
  (if o.scope_party.isSome then 1 else 0) + (if o.scope_phase.isSome then 1 else 0)

def chooseOverride (current : Option LocalRuleOverrideV1) (candidate : LocalRuleOverrideV1) : Option LocalRuleOverrideV1 :=
  match current with
  | none => some candidate
  | some existing =>
      let cSpec := overrideSpecificity candidate
      let eSpec := overrideSpecificity existing
      if cSpec > eSpec then
        some candidate
      else if cSpec < eSpec then
        some existing
      else if existing.ordered_at ≤ candidate.ordered_at then
        some candidate
      else
        some existing

def effectiveLimitValue
    (s : CourtState)
    (limitKey actor phase nowIso : String) : Except String Nat := do
  let base ← match policyLimitValue s.policy limitKey with
    | some n => pure n
    | none => throw s!"unknown local-rule limit key: {limitKey}"
  let winner :=
    s.case.local_rule_overrides.foldl
      (fun acc o =>
        if o.limit_key = limitKey && overrideApplies o actor phase nowIso then
          chooseOverride acc o
        else
          acc)
      (none : Option LocalRuleOverrideV1)
  match winner with
  | some o => pure o.new_value
  | none => pure base

def limitViolationMessage
    (limitKey actor phase : String)
    (attempted allowed : Nat)
    (detail : String) : String :=
  s!"LOCAL_RULE_LIMIT_EXCEEDED|limit_key={limitKey}|actor={actor}|phase={phase}|attempted={attempted}|allowed={allowed}|detail={detail}"

def enforceMeasuredLimit
    (s : CourtState)
    (actor phase nowIso limitKey detail : String)
    (attempted : Nat) : Except String Nat :=
  match effectiveLimitValue s limitKey actor phase nowIso with
  | .error e => .error e
  | .ok allowed =>
      if attempted ≤ allowed then
        .ok allowed
      else
        .error (limitViolationMessage limitKey actor phase attempted allowed detail)

def rolePolicyFor (roles : List RolePolicy) (role : String) : Option RolePolicy :=
  roles.find? (fun rp => normalizePartyToken rp.role = role)

def roleAllowsAll (roles : List RolePolicy) (role : String) (tools : List String) : Bool :=
  match rolePolicyFor roles role with
  | none => false
  | some rp => tools.all (fun tool => rp.allowed_tools.contains tool)

def policyJuryJurorCount (policy : CourtPolicy) : Nat :=
  if policy.jury_juror_count = 0 then 6 else policy.jury_juror_count

def policyJuryUnanimousRequired (policy : CourtPolicy) : Bool :=
  policy.jury_unanimous_required != 0

def policyJuryMinimumConcurring (policy : CourtPolicy) (jurorCount : Nat) : Nat :=
  if policyJuryUnanimousRequired policy then
    jurorCount
  else if policy.jury_minimum_concurring = 0 then
    6
  else
    policy.jury_minimum_concurring

def requiredJuryVotes (c : CaseState) : Nat :=
  match c.jury_configuration with
  | some cfg => cfg.minimum_concurring
  | none => 6

def mkDeterministicSingleTool (actionType : String) (payload : Json) : Json :=
  Json.mkObj [
    ("kind", toJson "single_tool"),
    ("action_type", toJson actionType),
    ("payload", payload)
  ]

def mkDeterministicClerkPanelSetup (caseId : String) (candidateCount : Nat) : Json :=
  Json.mkObj [
    ("kind", toJson "clerk_panel_setup"),
    ("case_id", toJson caseId),
    ("candidate_count", toJson candidateCount)
  ]

def mkDeterministicRandomEmpanelJury (caseId : String) (jurorCount : Nat) : Json :=
  Json.mkObj [
    ("kind", toJson "random_empanel_jury"),
    ("case_id", toJson caseId),
    ("juror_count", toJson jurorCount)
  ]

def mkDeterministicJuryConfiguration (caseId : String) (policy : CourtPolicy) : Json :=
  let jurorCount := policyJuryJurorCount policy
  let unanimous := policyJuryUnanimousRequired policy
  let minimumConcurring := policyJuryMinimumConcurring policy jurorCount
  mkDeterministicSingleTool "set_jury_configuration"
    (Json.mkObj [
      ("case_id", toJson caseId),
      ("juror_count", toJson jurorCount),
      ("unanimous_required", toJson unanimous),
      ("minimum_concurring", toJson minimumConcurring)
    ])

def mandatoryOpportunityTool (tool : String) : Bool :=
  tool = "decide_rule11_motion" ||
  tool = "evaluate_rule68_cost_shift" ||
  tool = "enter_judgment" ||
  tool = "resolve_trial_mode" ||
  tool = "set_last_pleading_served_on" ||
  tool = "settle_jury_instructions" ||
  tool = "deliver_jury_instructions" ||
  tool = "answer_juror_questionnaire" ||
  tool = "decide_voir_dire_question" ||
  tool = "answer_voir_dire_question" ||
  tool = "decide_juror_for_cause_challenge" ||
  tool = "empanel_jury" ||
  tool = "submit_juror_vote" ||
  tool = "decide_rule12_motion" ||
  tool = "decide_rule37_motion" ||
  tool = "decide_rule56_motion" ||
  tool = "resolve_rule59_motion"

def inferMayPass (allowedTools : List String) (requireSuccess : Bool) (deterministicAction : Option Json) : Bool :=
  if deterministicAction.isSome then
    false
  else if requireSuccess then
    false
  else
    !(allowedTools.any mandatoryOpportunityTool)

def currentOpportunityPhase (c : CaseState) : String :=
  if c.status = "trial" then c.phase else c.status

def mkOpportunityMessage (role phase : String) (mayPass : Bool) : String :=
  let phaseText := if phase.trimAscii.toString.isEmpty then "current" else phase
  if mayPass then
    s!"Current {phaseText} opportunity for {role}: consider this objective and either act now or pass."
  else
    s!"Current {phaseText} opportunity for {role}: act on this objective now."

def mkTurn (role objective : String) (allowedTools : List String) (requireSuccess : Bool) (maxSteps : Nat)
    (deterministicAction : Option Json := none) (priority : Nat := 100) : OpportunitySpec :=
  let mayPass := inferMayPass allowedTools requireSuccess deterministicAction
  { opportunity_id := ""
  , role := role
  , phase := ""
  , kind := if mayPass then "optional" else "required"
  , may_pass := mayPass
  , actor_message := ""
  , objective := objective
  , allowed_tools := allowedTools
  , step_budget := maxSteps
  , priority := priority
  , constraints := Json.null
  , deterministic_action := deterministicAction
  }

def fixedPayloadConstraints (pairs : List (String × Json)) : Json :=
  let scopedObj := Json.mkObj pairs
  Json.mkObj [
    ("payload_defaults", scopedObj),
    ("required_payload", scopedObj)
  ]

def partyScopedPayloadConstraints (party : String) (pairs : List (String × Json)) : Json :=
  fixedPayloadConstraints (("party", toJson party) :: pairs)

/--
Opportunity ids are currently deterministic hashes of opportunity content.
That fixes the earlier positional-id bug, where passing `o1` in one
unchanged state could suppress a different later opportunity that also
received `o1`.

These ids are not injective.  They are only deterministic.  A stronger proof
about pass isolation needs injective ids inside one agenda: passing one
optional opportunity should suppress exactly that opportunity and preserve all
other distinct opportunities in the same state.

The clean candidate design is state-local index assignment over the full
`availableOpportunities` list before filtering `passed_opportunities`.  That
would make ids pairwise distinct within a state, keep them stable across pass
filtering in the same state, and avoid proving properties about hash
collisions.  We are not changing the implementation yet.  Keep that design
constraint in mind when adding proofs or changing agenda generation.
-/
def assignOpportunityIds (actions : List OpportunitySpec) : List OpportunitySpec :=
  let makeOpportunityId (action : OpportunitySpec) : String :=
    let seed := Json.compress <| Json.mkObj [
      ("role", toJson action.role),
      ("phase", toJson action.phase),
      ("kind", toJson action.kind),
      ("objective", toJson action.objective),
      ("allowed_tools", toJson action.allowed_tools),
      ("step_budget", toJson action.step_budget),
      ("priority", toJson action.priority),
      ("constraints", action.constraints),
      ("deterministic_action", toJson action.deterministic_action)
    ]
    s!"o{hash seed}"
  actions.map (fun action => { action with opportunity_id := makeOpportunityId action })

def finalizeOpportunities (c : CaseState) (actions : List OpportunitySpec) : List OpportunitySpec :=
  let phase := currentOpportunityPhase c
  actions.map (fun action =>
    { action with
        phase := phase
        actor_message :=
          if action.actor_message.trimAscii.toString.isEmpty then
            mkOpportunityMessage action.role phase action.may_pass
          else
            action.actor_message
    })

structure TurnFacts where
  hasComplaint : Bool
  hasAnswer : Bool
  hasAmendedComplaint : Bool
  hasOpeningPlaintiff : Bool
  hasOpeningDefendant : Bool
  hasTheoryPlaintiff : Bool
  hasTheoryDefendant : Bool
  hasRebuttalPlaintiff : Bool
  hasSurrebuttalDefendant : Bool
  hasClosingPlaintiff : Bool
  hasClosingDefendant : Bool
  hasClosingRebuttalPlaintiff : Bool
  hasDeliberationNote : Bool
  hasBenchOpinion : Bool
  hasJuryPoll : Bool
  hasRule12Motion : Bool
  hasRule12Opposition : Bool
  hasRule12Reply : Bool
  hasRule12Order : Bool
  hasRule56Motion : Bool
  hasRule56Opposition : Bool
  hasRule56Reply : Bool
  hasRule56Order : Bool
  hasPartialJudgment : Bool
  hasPretrialOrder : Bool
  hasRule41Dismissal : Bool
  hasRule59Motion : Bool
  hasRule59Order : Bool
  hasRule37Motion : Bool
  hasRule37Order : Bool
  hasInterrogatoryResponses : Bool
  hasRule11Notice : Bool
  hasRule11Correction : Bool
  hasRule11Motion : Bool
  hasRule11Order : Bool
  hasRule68Offer : Bool
  hasPendingRule68Offer : Bool
  hasAcceptedRule68Offer : Bool
  hasExpiredRule68Offer : Bool
  hasRule68CostShift : Bool
  hasSupersedeasBond : Bool
  hasDiscretionaryStay : Bool
  hasStayLift : Bool
  hasRfpServed : Bool
  hasRfpResponses : Bool
  hasRfaServed : Bool
  hasRfaResponses : Bool
  hasAnyCaseFile : Bool
  hasCaseFileImported : Bool
  hasCaseFileProduced : Bool
  hasPleadingService : Bool
  hasJuryDemand : Bool
  hasDefaultEntered : Bool
  hasDefaultJudgment : Bool
  hasRule60Motion : Bool
  hasRule60Order : Bool
  hasActiveProtectiveOrder : Bool
  hasProtectiveOrderEver : Bool
  hasTechnicalReportPlaintiff : Bool
  hasTechnicalReportDefendant : Bool
  hasJuryInstructionProposalPlaintiff : Bool
  hasJuryInstructionProposalDefendant : Bool
  hasJuryInstructionObjectionPlaintiff : Bool
  hasJuryInstructionObjectionDefendant : Bool
  hasJuryInstructionsSettled : Bool
  hasJuryInstructionsDelivered : Bool
  deriving Inhabited

def collectTurnFacts (c : CaseState) : TurnFacts :=
  { hasComplaint := hasDecisionTraceAction c "file_complaint"
  , hasAnswer := hasDecisionTraceAction c "file_answer"
  , hasAmendedComplaint := hasDecisionTraceAction c "file_amended_complaint"
  , hasOpeningPlaintiff := hasDocketTitle c "Opening statement - plaintiff"
  , hasOpeningDefendant := hasDocketTitle c "Opening statement - defendant"
  , hasTheoryPlaintiff := hasDocketTitle c "Trial theory - plaintiff"
  , hasTheoryDefendant := hasDocketTitle c "Trial theory - defendant"
  , hasRebuttalPlaintiff := hasDocketTitle c "Rebuttal presentation - plaintiff"
  , hasSurrebuttalDefendant := hasDocketTitle c "Surrebuttal presentation - defendant"
  , hasClosingPlaintiff := hasDocketTitle c "Closing argument - plaintiff"
  , hasClosingDefendant := hasDocketTitle c "Closing argument - defendant"
  , hasClosingRebuttalPlaintiff := hasDocketTitle c "Closing rebuttal - plaintiff"
  , hasDeliberationNote := hasDocketTitle c "Jury deliberation note"
  , hasBenchOpinion := hasDocketTitle c "Bench Opinion"
  , hasJuryPoll := hasDocketTitle c "Jury poll"
  , hasRule12Motion := hasDocketTitle c "Rule 12 Motion"
  , hasRule12Opposition := hasDocketTitle c "Rule 12 Opposition"
  , hasRule12Reply := hasDocketTitle c "Rule 12 Reply"
  , hasRule12Order := hasDocketTitle c "Rule 12 Order"
  , hasRule56Motion := hasDocketTitle c "Rule 56 Motion"
  , hasRule56Opposition := hasDocketTitle c "Rule 56 Opposition"
  , hasRule56Reply := hasDocketTitle c "Rule 56 Reply"
  , hasRule56Order := hasDocketTitle c "Rule 56 Order"
  , hasPartialJudgment := hasDecisionTraceAction c "enter_partial_judgment"
  , hasPretrialOrder := hasDecisionTraceAction c "enter_pretrial_order"
  , hasRule41Dismissal := hasDecisionTraceAction c "dismiss_case_rule41"
  , hasRule59Motion := hasDecisionTraceAction c "file_rule59_motion"
  , hasRule59Order := hasDecisionTraceAction c "resolve_rule59_motion"
  , hasRule37Motion := hasDocketTitle c "Rule 37 Motion"
  , hasRule37Order := hasDocketTitle c "Rule 37 Order"
  , hasInterrogatoryResponses := hasDocketTitle c "Interrogatory Responses"
  , hasRule11Notice := hasDocketTitle c "Rule 11 Safe Harbor Notice"
  , hasRule11Correction := hasDecisionTraceAction c "withdraw_or_correct_filing"
  , hasRule11Motion := hasDocketTitle c "Rule 11 Motion"
  , hasRule11Order := hasDocketTitle c "Rule 11 Order"
  , hasRule68Offer := c.rule68_offers.length > 0
  , hasPendingRule68Offer := c.rule68_offers.any (fun o => o.status = "pending")
  , hasAcceptedRule68Offer := c.rule68_offers.any (fun o => o.status = "accepted")
  , hasExpiredRule68Offer := c.rule68_offers.any (fun o => o.status = "expired")
  , hasRule68CostShift := hasDocketTitle c "Rule 68 Cost Shift Evaluation"
  , hasSupersedeasBond := hasDecisionTraceAction c "post_supersedeas_bond"
  , hasDiscretionaryStay := hasDecisionTraceAction c "order_discretionary_stay"
  , hasStayLift := hasDecisionTraceAction c "lift_stay"
  , hasRfpServed := hasDocketTitle c "Requests for Production Served"
  , hasRfpResponses := hasDocketTitle c "Responses to Requests for Production"
  , hasRfaServed := hasDocketTitle c "Requests for Admission Served"
  , hasRfaResponses := hasDocketTitle c "Responses to Requests for Admission"
  , hasAnyCaseFile := !c.case_files.isEmpty
  , hasCaseFileImported := hasDecisionTraceAction c "import_case_file"
  , hasCaseFileProduced := hasDecisionTraceAction c "produce_case_file"
  , hasPleadingService := c.last_pleading_served_on != ""
  , hasJuryDemand := c.jury_demanded_on != ""
  , hasDefaultEntered := hasDecisionTraceAction c "enter_default"
  , hasDefaultJudgment := hasDecisionTraceAction c "enter_default_judgment"
  , hasRule60Motion := hasDecisionTraceAction c "file_rule60_motion"
  , hasRule60Order := hasDecisionTraceAction c "resolve_rule60_motion"
  , hasActiveProtectiveOrder := c.protective_orders.any (fun o => o.active)
  , hasProtectiveOrderEver := hasDecisionTraceAction c "enter_protective_order"
  , hasTechnicalReportPlaintiff := c.technical_reports.any (fun r => normalizePartyToken r.party = "plaintiff")
  , hasTechnicalReportDefendant := c.technical_reports.any (fun r => normalizePartyToken r.party = "defendant")
  , hasJuryInstructionProposalPlaintiff := hasDocketTitle c "Proposed jury instruction - plaintiff"
  , hasJuryInstructionProposalDefendant := hasDocketTitle c "Proposed jury instruction - defendant"
  , hasJuryInstructionObjectionPlaintiff := hasDocketTitle c "Jury instruction objection - plaintiff"
  , hasJuryInstructionObjectionDefendant := hasDocketTitle c "Jury instruction objection - defendant"
  , hasJuryInstructionsSettled := hasDocketTitle c "Jury instructions settled"
  , hasJuryInstructionsDelivered := hasDocketTitle c "Jury instructions delivered"
  }

def rule12LeaveToAmendPending (c : CaseState) : Bool :=
  c.docket.any (fun e =>
    e.title = "Rule 12 Order" &&
    e.description.contains "disposition=granted" &&
    e.description.contains "leave_to_amend=true") &&
  !hasDecisionTraceAction c "file_amended_complaint"

def latestRule12MotionGround? (c : CaseState) : Option String :=
  let motions := c.docket.filter (fun e => e.title = "Rule 12 Motion")
  match motions.reverse.head? with
  | none => none
  | some entry =>
      let marker := "ground="
      match entry.description.splitOn marker with
      | _ :: rest :: _ =>
          match rest.splitOn " " with
          | ground :: _ =>
              let trimmed := trimString ground
              if trimmed = "" then none else some trimmed
          | [] => none
      | _ => none

def validRule12Ground (s : CourtState) (ground : String) : Bool :=
  (courtUsesJurisdictionScreen s && ground = "lack_subject_matter_jurisdiction") ||
  ground = "no_standing" ||
  ground = "not_ripe" ||
  ground = "moot" ||
  ground = "failure_to_state_a_claim"

def rule12GroundSummary (s : CourtState) : String :=
  if courtUsesJurisdictionScreen s then
    "lack of subject-matter jurisdiction, no standing, not ripe, moot, or failure to state a claim"
  else
    "no standing, not ripe, moot, or failure to state a claim"

def rule56WindowEligible (c : CaseState) (facts : TurnFacts) (party : String) : Bool :=
  !facts.hasRule56Motion &&
    !facts.hasRule56Order &&
    !facts.hasPretrialOrder &&
    facts.hasInterrogatoryResponses &&
    facts.hasRfpResponses &&
    facts.hasRfaResponses &&
    (!facts.hasRule37Motion || facts.hasRule37Order) &&
    !rule56WindowClosedFor c party

def jurisdictionDismissalCandidates (req : OpportunityRequest) (c : CaseState) (maxSteps : Nat) : List OpportunitySpec := Id.run do
  let mut actions : List OpportunitySpec := []
  if c.status = "judgment_entered" || c.status = "closed" then
    return actions
  if !subjectMatterJurisdictionFaciallyDefectiveCase req.state c then
    return actions
  if hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction" then
    return actions
  if roleAllowsAll req.roles "judge" ["dismiss_for_lack_of_subject_matter_jurisdiction"] then
    actions := actions.concat
      ({ (mkTurn "judge"
        "For case 0, if the complaint does not adequately allege subject-matter jurisdiction, dismiss for lack of subject-matter jurisdiction and state the rejected basis."
        ["dismiss_for_lack_of_subject_matter_jurisdiction"] false maxSteps) with
          priority := 10
       })
  actions

def filedCandidates (req : OpportunityRequest) (c : CaseState) (facts : TurnFacts) (maxSteps : Nat) : List OpportunitySpec := Id.run do
  let mut actions : List OpportunitySpec := []
  if !facts.hasComplaint && roleAllowsAll req.roles "plaintiff" ["file_complaint"] then
    actions := actions.concat (mkTurn "plaintiff" "For case 0, file complaint clearly stating claim, liability theory, causation, and damages requested." ["file_complaint"] true maxSteps)
  if facts.hasComplaint && !facts.hasRule12Motion && !facts.hasAnswer && roleAllowsAll req.roles "defendant" ["file_rule12_motion"] then
    actions := actions.concat (mkTurn "defendant" s!"For case 0, optionally file a Rule 12 motion only if one supported ground fits the complaint as pleaded: {rule12GroundSummary req.state}. Otherwise file answer." ["file_rule12_motion"] false maxSteps)
  if facts.hasRule12Motion && !facts.hasRule12Opposition && roleAllowsAll req.roles "plaintiff" ["oppose_rule12_motion"] then
    actions := actions.concat (mkTurn "plaintiff" "For case 0, file Rule 12 opposition for motion_index 0." ["oppose_rule12_motion"] false maxSteps)
  if facts.hasRule12Opposition && !facts.hasRule12Reply && roleAllowsAll req.roles "defendant" ["reply_rule12_motion"] then
    actions := actions.concat (mkTurn "defendant" "For case 0, file Rule 12 reply for motion_index 0." ["reply_rule12_motion"] false maxSteps)
  if facts.hasRule12Motion && !facts.hasRule12Order && roleAllowsAll req.roles "judge" ["decide_rule12_motion"] then
    let ground := latestRule12MotionGround? c |>.getD "failure_to_state_a_claim"
    actions := actions.concat
      ({ (mkTurn "judge" s!"For case 0, decide Rule 12 motion_index 0 on the ground {ground}. Apply the standard for that ground. Grant only if that ground is established on the pleadings or jurisdictional allegations. If granted, set with_prejudice and leave_to_amend consistently and state the decisive reason." ["decide_rule12_motion"] false maxSteps) with
          constraints := Json.mkObj [("required_payload", Json.mkObj [("motion_index", toJson (0 : Nat)), ("ground", toJson ground)])]
       })
  if !facts.hasAnswer && !rule12LeaveToAmendPending c && roleAllowsAll req.roles "defendant" ["file_answer"] then
    actions := actions.concat (mkTurn "defendant" "For case 0, file answer responding to allegations and asserting defenses." ["file_answer"] true maxSteps)
  if facts.hasRule12Order && !facts.hasAmendedComplaint &&
      roleAllowsAll req.roles "plaintiff" ["file_amended_complaint"] then
    actions := actions.concat (mkTurn "plaintiff" "For case 0, file amended complaint adding concrete facts that cure pleading deficiencies identified in the Rule 12 order." ["file_amended_complaint"] true maxSteps)
  if c.auto_rule11 && facts.hasComplaint && !facts.hasRule11Notice &&
      roleAllowsAll req.roles "defendant" ["serve_rule11_safe_harbor_notice"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, if Rule 11 concerns exist, serve a safe-harbor notice on plaintiff." ["serve_rule11_safe_harbor_notice"] false 1
        (some
          (mkDeterministicSingleTool "serve_rule11_safe_harbor_notice"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("served_by", toJson "defendant"),
              ("target_party", toJson "plaintiff")
            ]))))
  if c.auto_rule11 && facts.hasRule11Notice && !facts.hasRule11Correction &&
      roleAllowsAll req.roles "plaintiff" ["withdraw_or_correct_filing"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, if safe-harbor issues are curable, withdraw or correct the challenged filing on notice_index 0." ["withdraw_or_correct_filing"] false 1
        (some
          (mkDeterministicSingleTool "withdraw_or_correct_filing"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("notice_index", toJson (0 : Nat)),
              ("by_party", toJson "plaintiff")
            ]))))
  if c.auto_rule11 && facts.hasRule11Notice && !facts.hasRule11Correction && !facts.hasRule11Motion &&
      roleAllowsAll req.roles "defendant" ["file_rule11_motion"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, if safe-harbor was not cured, file Rule 11 motion_index 0." ["file_rule11_motion"] false 1
        (some
          (mkDeterministicSingleTool "file_rule11_motion"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("movant", toJson "defendant"),
              ("notice_index", toJson (0 : Nat))
            ]))))
  if c.auto_rule11 && facts.hasRule11Motion && !facts.hasRule11Order &&
      roleAllowsAll req.roles "judge" ["decide_rule11_motion"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, decide Rule 11 motion_index 0 and state sanctions if granted." ["decide_rule11_motion"] false 1
        (some
          (mkDeterministicSingleTool "decide_rule11_motion"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("motion_index", toJson (0 : Nat)),
              ("granted", toJson true),
              ("sanction_type", toJson "admonition"),
              ("sanction_amount", toJson (0 : Nat)),
              ("sanction_detail", toJson "Pleadings must remain grounded in available evidence."),
              ("reasoning", toJson "The challenged filing warranted a limited sanction because safe-harbor notice did not lead to correction and an admonition is proportionate.")
            ]))))
  if facts.hasComplaint && !facts.hasAnswer && !facts.hasDefaultEntered &&
      roleAllowsAll req.roles "judge" ["enter_default"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, if defendant failed to plead, enter default against defendant with a short reason." ["enter_default"] false maxSteps)
  if facts.hasDefaultEntered && !facts.hasDefaultJudgment &&
      roleAllowsAll req.roles "judge" ["enter_default_judgment"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, if default is established, enter default judgment against defendant with supported monetary_amount and reason." ["enter_default_judgment"] false maxSteps)
  if facts.hasComplaint && facts.hasAnswer && !facts.hasRule68Offer &&
      roleAllowsAll req.roles "defendant" ["make_rule68_offer"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, make a Rule 68 offer of judgment to plaintiff." ["make_rule68_offer"] false 1
        (some
          (mkDeterministicSingleTool "make_rule68_offer"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("offer_id", toJson "offer-0001"),
              ("offeree", toJson "plaintiff"),
              ("amount", toJson (100000 : Nat)),
              ("terms", toJson "inclusive of taxable costs"),
              ("served_by", toJson "defendant"),
              ("served_at", toJson c.filed_on)
            ]))))
  if facts.hasPendingRule68Offer && !facts.hasAcceptedRule68Offer &&
      roleAllowsAll req.roles "plaintiff" ["accept_rule68_offer"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, accept the pending Rule 68 offer." ["accept_rule68_offer"] false 1
        (some
          (mkDeterministicSingleTool "accept_rule68_offer"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("offer_index", toJson (0 : Nat)),
              ("accepted_at", toJson c.filed_on)
            ]))))
  if facts.hasPendingRule68Offer && !facts.hasAcceptedRule68Offer &&
      roleAllowsAll req.roles "clerk" ["expire_rule68_offers"] then
    actions := actions.concat
      (mkTurn "clerk" "For case 0, expire pending Rule 68 offers." ["expire_rule68_offers"] false 1
        (some
          (mkDeterministicSingleTool "expire_rule68_offers"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("as_of", toJson c.filed_on)
            ]))))
  if facts.hasExpiredRule68Offer && !facts.hasRule68CostShift &&
      roleAllowsAll req.roles "judge" ["evaluate_rule68_cost_shift"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, evaluate Rule 68(d) cost-shift after offer expiry." ["evaluate_rule68_cost_shift"] false 1
        (some
          (mkDeterministicSingleTool "evaluate_rule68_cost_shift"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("offer_index", toJson (0 : Nat)),
              ("awarded_to", toJson "plaintiff"),
              ("amount", toJson (70000 : Nat)),
              ("reason", toJson "final recovery did not exceed unaccepted offer")
            ]))))
  if facts.hasAnswer && !facts.hasPleadingService && roleAllowsAll req.roles "clerk" ["set_last_pleading_served_on"] then
    actions := actions.concat (mkTurn "clerk" "For case 0, set the last pleading service date." ["set_last_pleading_served_on"] true 1
      (some (mkDeterministicSingleTool "set_last_pleading_served_on"
        (Json.mkObj [("case_id", toJson c.case_id), ("served_on", toJson c.filed_on)]))))
  if facts.hasPleadingService && !facts.hasJuryDemand && roleAllowsAll req.roles "clerk" ["record_jury_demand"] then
    actions := actions.concat (mkTurn "clerk" "For case 0, record a jury demand date." ["record_jury_demand"] false maxSteps)
  if c.trial_mode = "unset" && roleAllowsAll req.roles "judge" ["resolve_trial_mode"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, resolve trial mode with parties_stipulate_nonjury false and court_orders_jury true when jury demand exists." ["resolve_trial_mode"] true 1
        (some
          (mkDeterministicSingleTool "resolve_trial_mode"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("parties_stipulate_nonjury", toJson false),
              ("court_orders_jury", toJson facts.hasJuryDemand)
            ]))))
  if roleAllowsAll req.roles "judge" ["transition_case"] then
    actions := actions.concat (mkTurn "judge" "For case 0, transition case to pretrial only." ["transition_case"] true 1
      (some (mkDeterministicSingleTool "transition_case"
        (Json.mkObj [("case_id", toJson c.case_id), ("next_status", toJson "pretrial")]))))
  actions

def pretrialCandidates (req : OpportunityRequest) (c : CaseState) (facts : TurnFacts) (maxSteps : Nat) : List OpportunitySpec := Id.run do
  let mut actions : List OpportunitySpec := []
  if !facts.hasProtectiveOrderEver && roleAllowsAll req.roles "judge" ["enter_protective_order"] then
    let payload := Json.mkObj [
      ("case_id", toJson c.case_id),
      ("order_id", toJson "po-0001"),
      ("scope", toJson "category"),
      ("target", toJson "discovery"),
      ("allowed_roles", toJson ["plaintiff", "defendant", "judge"]),
      ("note", toJson "Protective order for discovery materials.")
    ]
    actions := actions.concat
      (mkTurn "judge" "For case 0, optionally enter a narrowly tailored protective order for discovery with order_id po-0001." ["enter_protective_order"] false 1
        (some (mkDeterministicSingleTool "enter_protective_order" payload)))
  if !facts.hasPartialJudgment && roleAllowsAll req.roles "judge" ["enter_partial_judgment"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, if fewer than all claims or issues are fully resolved, enter a Rule 54(b) partial judgment with reasons." ["enter_partial_judgment"] false 1
        (some
          (mkDeterministicSingleTool "enter_partial_judgment"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("issues_resolved", toJson ["liability"]),
              ("amount", toJson (50000 : Nat)),
              ("basis", toJson "no genuine dispute of material fact on liability")
            ]))))
  if !hasDocketTitle c "Initial Disclosures" && roleAllowsAll req.roles "plaintiff" ["serve_initial_disclosures"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, serve initial disclosures." ["serve_initial_disclosures"] true 1
        (some
          (mkDeterministicSingleTool "serve_initial_disclosures"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("party", toJson "plaintiff"),
              ("summary", toJson "initial disclosures served")
            ]))))
  if !hasDocketTitle c "Initial Disclosures" && roleAllowsAll req.roles "defendant" ["serve_initial_disclosures"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, serve initial disclosures." ["serve_initial_disclosures"] true 1
        (some
          (mkDeterministicSingleTool "serve_initial_disclosures"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("party", toJson "defendant"),
              ("summary", toJson "initial disclosures served")
            ]))))
  if !facts.hasTechnicalReportPlaintiff && roleAllowsAll req.roles "plaintiff" ["submit_technical_report"] then
    actions := actions.concat
      ({ (mkTurn "plaintiff" "For case 0, submit a plaintiff technical report with report_id TR-P1, title, summary, and concise limitations." ["submit_technical_report"] false maxSteps) with
          constraints := partyScopedPayloadConstraints "plaintiff" [("report_id", toJson "TR-P1")]
       })
  if !facts.hasTechnicalReportDefendant && roleAllowsAll req.roles "defendant" ["submit_technical_report"] then
    actions := actions.concat
      ({ (mkTurn "defendant" "For case 0, submit a defense technical report with report_id TR-D1, title, summary, and concise limitations." ["submit_technical_report"] false maxSteps) with
          constraints := partyScopedPayloadConstraints "defendant" [("report_id", toJson "TR-D1")]
       })
  if !facts.hasAnyCaseFile && roleAllowsAll req.roles "defendant" ["import_case_file"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, import a key case file that may be produced in discovery." ["import_case_file"] true 1
        (some
          (mkDeterministicSingleTool "import_case_file"
            (Json.mkObj [
              ("imported_by", toJson "defendant"),
              ("source_filename", toJson "scenarios/assets/supply_chain_delay_notice.txt"),
              ("label", toJson "Delay notice"),
            ]))))
  if !hasDocketTitle c "Interrogatories Served" && roleAllowsAll req.roles "plaintiff" ["serve_interrogatories"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, serve a focused first interrogatory set with at most five questions. Use served_by=plaintiff and served_on=defendant. Use an in-case service date and do not backdate." ["serve_interrogatories"] true 1
        (some
          (mkDeterministicSingleTool "serve_interrogatories"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("served_by", toJson "plaintiff"),
              ("served_on", toJson "defendant"),
              ("questions", toJson ["Identify all recipients of the disputed document.", "State when the disclosure occurred.", "Describe safeguards in place.", "Identify all versions transmitted.", "State the basis for asserting no damages."])
            ]))))
  if hasDocketTitle c "Interrogatories Served" && !hasDocketTitle c "Interrogatory Responses" then
    if roleAllowsAll req.roles "defendant" ["respond_interrogatories"] then
      actions := actions.concat
        (mkTurn "defendant" "For case 0, serve verified interrogatory responses for set_index 0." ["respond_interrogatories"] true 1
          (some
            (mkDeterministicSingleTool "respond_interrogatories"
              (Json.mkObj [
                ("case_id", toJson c.case_id),
                ("set_index", toJson (0 : Nat)),
                ("responding_party", toJson "defendant")
              ]))))
    else if roleAllowsAll req.roles "defendant" ["respond_interrogatory_item", "finalize_interrogatory_responses"] then
      actions := actions.concat (mkTurn "defendant" "For case 0, draft interrogatory responses one question at a time for set_index 0. Draft only missing question indices. If all questions are already drafted, call finalize_interrogatory_responses immediately with verified=true and do not include responded_at." ["respond_interrogatory_item", "finalize_interrogatory_responses"] false maxSteps)
  if !facts.hasRfpServed && roleAllowsAll req.roles "plaintiff" ["serve_request_for_production"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, serve first request for production set." ["serve_request_for_production"] true 1
        (some
          (mkDeterministicSingleTool "serve_request_for_production"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("served_by", toJson "plaintiff"),
              ("requests", toJson ["Produce all versions of the disputed specification.", "Produce all transmission logs for the disputed file."])
            ]))))
  if facts.hasCaseFileImported && facts.hasRfpServed && !facts.hasCaseFileProduced &&
      roleAllowsAll req.roles "defendant" ["produce_case_file"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, produce imported file file-0001 to plaintiff as responsive production." ["produce_case_file"] true 1
        (some
          (mkDeterministicSingleTool "produce_case_file"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("file_id", toJson "file-0001"),
              ("produced_by", toJson "defendant"),
              ("produced_to", toJson "plaintiff"),
              ("request_ref", toJson "RFP-0"),
              ("produced_at", toJson c.filed_on)
            ]))))
  if facts.hasRfpServed && !facts.hasRfpResponses && roleAllowsAll req.roles "defendant" ["respond_request_for_production"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, serve responses to request-for-production set_index 0." ["respond_request_for_production"] true 1
        (some
          (mkDeterministicSingleTool "respond_request_for_production"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("set_index", toJson (0 : Nat)),
              ("responding_party", toJson "defendant")
            ]))))
  if !facts.hasRfaServed && roleAllowsAll req.roles "plaintiff" ["serve_requests_for_admission"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, serve first requests-for-admission set." ["serve_requests_for_admission"] true 1
        (some
          (mkDeterministicSingleTool "serve_requests_for_admission"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("served_by", toJson "plaintiff"),
              ("requests", toJson ["Admit receipt of the confidential package.", "Admit no written authorization existed.", "Admit transmission to at least one third party."])
            ]))))
  if facts.hasRfaServed && !facts.hasRfaResponses && roleAllowsAll req.roles "defendant" ["respond_requests_for_admission"] then
    actions := actions.concat
      (mkTurn "defendant" "For case 0, serve responses to requests-for-admission set_index 0." ["respond_requests_for_admission"] true 1
        (some
          (mkDeterministicSingleTool "respond_requests_for_admission"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("set_index", toJson (0 : Nat)),
              ("responding_party", toJson "defendant")
            ]))))
  if !facts.hasRule37Motion && roleAllowsAll req.roles "plaintiff" ["file_rule37_motion"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, file Rule 37 motion to compel based on interrogatories set_index 0 with relief sought and summary." ["file_rule37_motion"] true 1
        (some
          (mkDeterministicSingleTool "file_rule37_motion"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("movant", toJson "plaintiff"),
              ("target_party", toJson "defendant"),
              ("discovery_type", toJson "interrogatories"),
              ("set_index", toJson (0 : Nat)),
              ("discovery_set_count", toJson (1 : Nat)),
              ("relief_summary", toJson "order complete verified interrogatory responses")
            ]))))
  if facts.hasRule37Motion && !facts.hasRule37Order && roleAllowsAll req.roles "judge" ["decide_rule37_motion"] then
    let grantRule37 := !facts.hasInterrogatoryResponses
    let sanctionType := if grantRule37 then "fees" else "none"
    let sanctionAmount : Nat := if grantRule37 then 750 else 0
    let orderText :=
      if grantRule37 then
        "motion granted; compel complete interrogatory responses and award fees"
      else
        "motion denied"
    actions := actions.concat
      (mkTurn "judge" "For case 0, decide Rule 37 motion_index 0 and include sanction decision." ["decide_rule37_motion"] true 1
        (some
          (mkDeterministicSingleTool "decide_rule37_motion"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("motion_index", toJson (0 : Nat)),
              ("granted", toJson grantRule37),
              ("sanction_type", toJson sanctionType),
              ("sanction_amount", toJson sanctionAmount),
              ("order_text", toJson orderText),
              ("reasoning", toJson (if grantRule37 then
                "The record shows the requested discovery response remains incomplete, so an order compelling responses and awarding fees is warranted."
              else
                "The record shows no unresolved discovery failure that warrants compulsion or sanctions."))
            ]))))
  if rule56WindowEligible c facts "defendant" && roleAllowsAll req.roles "defendant" ["file_rule56_motion"] then
    actions := actions.concat (mkTurn "defendant" "For case 0, optionally file Rule 56 motion if no genuine dispute of material fact." ["file_rule56_motion"] false maxSteps)
  if facts.hasRule56Motion && !facts.hasRule56Opposition && roleAllowsAll req.roles "plaintiff" ["oppose_rule56_motion"] then
    actions := actions.concat (mkTurn "plaintiff" "For case 0, file Rule 56 opposition for motion_index 0." ["oppose_rule56_motion"] false maxSteps)
  if facts.hasRule56Opposition && !facts.hasRule56Reply && roleAllowsAll req.roles "defendant" ["reply_rule56_motion"] then
    actions := actions.concat (mkTurn "defendant" "For case 0, file Rule 56 reply for motion_index 0." ["reply_rule56_motion"] false maxSteps)
  if facts.hasRule56Motion && !facts.hasRule56Order && roleAllowsAll req.roles "judge" ["decide_rule56_motion"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, decide Rule 56 motion_index 0 with disposition granted, denied, or partial, and explain the decisive record-based reason." ["decide_rule56_motion"] true maxSteps)
  if facts.hasActiveProtectiveOrder && facts.hasRfpResponses && facts.hasRfaResponses &&
      roleAllowsAll req.roles "judge" ["lift_protective_order"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, if confidentiality restrictions are no longer required, lift protective order po-0001." ["lift_protective_order"] false maxSteps)
  if facts.hasAnswer && !facts.hasRule41Dismissal && roleAllowsAll req.roles "plaintiff" ["dismiss_case_rule41"] then
    actions := actions.concat
      (mkTurn "plaintiff" "For case 0, if the parties stipulate, dismiss the action under Rule 41." ["dismiss_case_rule41"] false 1
        (some
          (mkDeterministicSingleTool "dismiss_case_rule41"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("with_prejudice", toJson false),
              ("reason", toJson "stipulated dismissal")
            ]))))
  if facts.hasAnswer && roleAllowsAll req.roles "judge" ["enter_settlement"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, if parties have reached agreement, enter settlement with amount and consent_judgment." ["enter_settlement"] false 1
        (some
          (mkDeterministicSingleTool "enter_settlement"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("amount", toJson (185000 : Nat)),
              ("consent_judgment", toJson true),
              ("summary", toJson "Settlement entered after pleadings and motion practice.")
            ]))))
  if !facts.hasPretrialOrder && roleAllowsAll req.roles "judge" ["enter_pretrial_order"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, enter a Rule 16(e) pretrial order defining claims and exhibits for trial control." ["enter_pretrial_order"] false 1
        (some
          (mkDeterministicSingleTool "enter_pretrial_order"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("text", toJson "pretrial_order: claim=claim-1 documentary exhibits preserved for trial")
            ]))))
  if roleAllowsAll req.roles "judge" ["transition_case"] then
    actions := actions.concat (mkTurn "judge" "For case 0, transition case to trial only." ["transition_case"] true 1
      (some (mkDeterministicSingleTool "transition_case"
        (Json.mkObj [("case_id", toJson c.case_id), ("next_status", toJson "trial")]))))
  actions

def trialCandidates (req : OpportunityRequest) (c : CaseState) (facts : TurnFacts) (maxSteps : Nat) : List OpportunitySpec := Id.run do
  let mut actions : List OpportunitySpec := []
  let claimId :=
    match c.single_claim with
    | some claim =>
        match claim.getObjVal? "claim_id" with
        | .ok claimIdJson =>
            match claimIdJson.getStr? with
            | .ok s => s
            | .error _ => "claim-1"
        | .error _ => "claim-1"
    | none => "claim-1"
  let voirDireQuestionsPerSide := effectiveVoirDireQuestionsPerSide req.state.policy
  let maxDisallowedVoirDirePerSide := req.state.policy.max_disallowed_voir_dire_questions_per_side
  let candidateTarget := voirDireCandidateTarget req.state.policy c
  let maxForCausePerSide := req.state.policy.max_for_cause_challenges_per_side
  let maxPeremptoryPerSide := req.state.policy.max_peremptory_challenges_per_side
  if c.trial_mode = "jury" && c.jury_configuration.isNone && roleAllowsAll req.roles "clerk" ["set_jury_configuration"] then
    let jurorCount := policyJuryJurorCount req.state.policy
    let minimumConcurring := policyJuryMinimumConcurring req.state.policy jurorCount
    actions := actions.concat
      (mkTurn "clerk"
        s!"For case 0, set jury configuration to {jurorCount} jurors with minimum concurring {minimumConcurring}."
        ["set_jury_configuration"] true maxSteps
        (some (mkDeterministicJuryConfiguration c.case_id req.state.policy)))
  if c.trial_mode = "jury" && c.phase = "none" && !voirDirePanelReady req.state.policy c &&
      roleAllowsAll req.roles "clerk" ["add_juror"] then
    actions := actions.concat (mkTurn "clerk" s!"For case 0, add any missing prospective jurors to reach a voir dire panel of {candidateTarget} candidates." ["add_juror"] true maxSteps
      (some (mkDeterministicClerkPanelSetup c.case_id candidateTarget)))
  if c.phase = "none" && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
    let nextPhase := if c.trial_mode = "jury" then "voir_dire" else "openings"
    actions := actions.concat (mkTurn "judge" s!"For case 0, advance phase to {nextPhase}." ["advance_trial_phase"] true 1
      (some (mkDeterministicSingleTool "advance_trial_phase"
        (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson nextPhase)]))))
  if c.phase = "voir_dire" then
    if juryEmpaneled c && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to openings." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "openings")]))))
    else if skipVoirDire req.state.policy then
      match c.jury_configuration with
      | some cfg =>
          if voirDirePanelReady req.state.policy c &&
              countCandidates c.jurors >= cfg.juror_count &&
              roleAllowsAll req.roles "judge" ["empanel_jury"] then
            actions := actions.concat
              (mkTurn "judge"
                s!"For case 0, skip voir dire and randomly empanel exactly {cfg.juror_count} jurors from the current candidate panel."
                ["empanel_jury"] true 1
                (some (mkDeterministicRandomEmpanelJury c.case_id cfg.juror_count)))
      | none => ()
    else
      if !jurorQuestionnaireIssued c then
        if roleAllowsAll req.roles "judge" ["issue_juror_questionnaire"] then
          actions := actions.concat (mkTurn "judge" "For case 0, issue the standard court juror questionnaire for the current candidate panel." ["issue_juror_questionnaire"] true 1
            (some (mkDeterministicSingleTool "issue_juror_questionnaire" (Json.mkObj []))))
      else
        match nextCandidateWithoutQuestionnaireResponse? c with
        | some juror =>
            if roleAllowsAll req.roles "juror" ["answer_juror_questionnaire"] then
              actions := actions.concat
                ({ (mkTurn "juror"
                    s!"For case 0, answer the court's juror questionnaire for juror_id {juror.juror_id}. Answer each question truthfully from your own perspective."
                    ["answer_juror_questionnaire"] false maxSteps) with
                      constraints := Json.mkObj [
                        ("required_payload", Json.mkObj [("juror_id", toJson juror.juror_id)]),
                        ("questionnaire", toJson c.juror_questionnaire)
                      ]
                 })
        | none =>
          match nextPendingVoirDireRuling? c with
          | some exchange =>
              if roleAllowsAll req.roles "judge" ["decide_voir_dire_question"] then
                actions := actions.concat
                  ({ (mkTurn "judge"
                      s!"For case 0, rule on the pending voir dire question by {exchange.asked_by} to juror_id {exchange.juror_id}. Allow a narrow question that tests bias, burden of proof discipline, attitudes toward documentary or digital evidence, or the ability to follow instructions. Disallow a question that argues the merits, assumes disputed facts, asks for a precommitment on liability or damages, or asks whether specific proof would be enough. If you disallow it, explain the problem briefly so counsel can adapt."
                      ["decide_voir_dire_question"] false maxSteps) with
                        constraints := fixedPayloadConstraints [
                          ("exchange_id", toJson exchange.exchange_id),
                          ("juror_id", toJson exchange.juror_id),
                          ("asked_by", toJson exchange.asked_by)
                        ]
                   })
          | none =>
            match nextPendingForCauseChallenge? c with
            | some challenge =>
                if roleAllowsAll req.roles "judge" ["decide_juror_for_cause_challenge"] then
                  actions := actions.concat
                    ({ (mkTurn "judge"
                        s!"For case 0, decide the pending for-cause challenge by {challenge.by_party} to juror_id {challenge.juror_id}. Grant it if the record shows the candidate cannot be impartial or cannot follow the court's instructions.  Otherwise deny it and explain why the candidate can still serve."
                        ["decide_juror_for_cause_challenge"] false maxSteps) with
                          constraints := fixedPayloadConstraints [
                            ("challenge_id", toJson challenge.challenge_id),
                            ("juror_id", toJson challenge.juror_id),
                            ("by_party", toJson challenge.by_party)
                          ]
                     })
            | none =>
              match nextPendingVoirDireExchange? c with
              | some exchange =>
                  if roleAllowsAll req.roles "juror" ["answer_voir_dire_question"] then
                    actions := actions.concat
                      ({ (mkTurn "juror"
                          s!"For case 0, answer the pending voir dire question for juror_id {exchange.juror_id} from {exchange.asked_by}: {exchange.question}"
                          ["answer_voir_dire_question"] false maxSteps) with
                            constraints := fixedPayloadConstraints [
                              ("exchange_id", toJson exchange.exchange_id),
                              ("juror_id", toJson exchange.juror_id)
                            ]
                       })
              | none =>
                match if countDisallowedVoirDireQuestionsFrom c "plaintiff" < maxDisallowedVoirDirePerSide then
                  nextAvailableJurorNeedingQuestionFrom? c "plaintiff" voirDireQuestionsPerSide
                else
                  none with
                | some juror =>
                    if roleAllowsAll req.roles "plaintiff" ["record_voir_dire_question"] then
                      actions := actions.concat
                        ({ (mkTurn "plaintiff"
                            s!"For case 0, propose one voir dire question for juror {juror.juror_id} on matters relevant to impartiality, credibility, attention, bias, and the ability to follow the court's instructions. The judge will screen the question before the juror sees it."
                            ["record_voir_dire_question"] false maxSteps) with
                              constraints := fixedPayloadConstraints [
                                ("juror_id", toJson juror.juror_id),
                                ("asked_by", toJson "plaintiff")
                              ]
                         })
                | none =>
                    match if countDisallowedVoirDireQuestionsFrom c "defendant" < maxDisallowedVoirDirePerSide then
                      nextAvailableJurorNeedingQuestionFrom? c "defendant" voirDireQuestionsPerSide
                    else
                      none with
                    | some juror =>
                        if roleAllowsAll req.roles "defendant" ["record_voir_dire_question"] then
                          actions := actions.concat
                            ({ (mkTurn "defendant"
                                s!"For case 0, propose one voir dire question for juror {juror.juror_id} on matters relevant to impartiality, burden of proof, damages, bias, and the ability to follow the court's instructions. The judge will screen the question before the juror sees it."
                                ["record_voir_dire_question"] false maxSteps) with
                                  constraints := fixedPayloadConstraints [
                                    ("juror_id", toJson juror.juror_id),
                                    ("asked_by", toJson "defendant")
                                  ]
                             })
                    | none =>
                        if countForCauseChallengesBy c "plaintiff" < maxForCausePerSide &&
                            !hasForCauseChallengePassBy c "plaintiff" then
                          match nextAvailableJurorWithoutForCauseRequestBy? c "plaintiff" with
                          | some _ =>
                              if roleAllowsAll req.roles "plaintiff" ["challenge_juror_for_cause"] then
                                actions := actions.concat
                                  (mkTurn "plaintiff"
                                    "For case 0, if any candidate said something that shows they cannot be impartial or cannot follow the court's instructions, challenge that candidate for cause and state the disqualifying ground. Otherwise pass."
                                    ["challenge_juror_for_cause"] false maxSteps)
                          | none => ()
                        if countForCauseChallengesBy c "defendant" < maxForCausePerSide &&
                            !hasForCauseChallengePassBy c "defendant" then
                          match nextAvailableJurorWithoutForCauseRequestBy? c "defendant" with
                          | some _ =>
                              if roleAllowsAll req.roles "defendant" ["challenge_juror_for_cause"] then
                                actions := actions.concat
                                  (mkTurn "defendant"
                                    "For case 0, if any candidate said something that shows they cannot be impartial or cannot follow the court's instructions, challenge that candidate for cause and state the disqualifying ground. Otherwise pass."
                                    ["challenge_juror_for_cause"] false maxSteps)
                          | none => ()
                        if countPeremptoryChallengesBy c "plaintiff" < maxPeremptoryPerSide &&
                            !hasPeremptoryChallengePassBy c "plaintiff" then
                          match nextAvailableJurorWithoutPeremptoryStrikeBy? c "plaintiff" with
                          | some _ =>
                              if roleAllowsAll req.roles "plaintiff" ["strike_juror_peremptorily"] then
                                actions := actions.concat
                                  ({ (mkTurn "plaintiff"
                                      "For case 0, if you want to use a peremptory strike, choose one remaining candidate juror_id and strike that juror. Otherwise pass."
                                      ["strike_juror_peremptorily"] false maxSteps) with
                                        constraints := fixedPayloadConstraints [("party", toJson "plaintiff")]
                                   })
                          | none => ()
                        if countPeremptoryChallengesBy c "defendant" < maxPeremptoryPerSide &&
                            !hasPeremptoryChallengePassBy c "defendant" then
                          match nextAvailableJurorWithoutPeremptoryStrikeBy? c "defendant" with
                          | some _ =>
                              if roleAllowsAll req.roles "defendant" ["strike_juror_peremptorily"] then
                                actions := actions.concat
                                  ({ (mkTurn "defendant"
                                      "For case 0, if you want to use a peremptory strike, choose one remaining candidate juror_id and strike that juror. Otherwise pass."
                                      ["strike_juror_peremptorily"] false maxSteps) with
                                        constraints := fixedPayloadConstraints [("party", toJson "defendant")]
                                   })
                          | none => ()
                        match c.jury_configuration with
                        | some cfg =>
                            if allAvailableJurorsReadyForEmpanelment c voirDireQuestionsPerSide maxDisallowedVoirDirePerSide &&
                                countCandidates c.jurors >= cfg.juror_count &&
                                roleAllowsAll req.roles "judge" ["empanel_jury"] then
                              actions := actions.concat
                                (mkTurn "judge"
                                  s!"For case 0, empanel exactly {cfg.juror_count} jurors from the remaining candidate panel based on the questionnaire record and oral voir dire."
                                  ["empanel_jury"] false maxSteps)
                        | none => ()
  if c.phase = "openings" then
    if !facts.hasOpeningPlaintiff && roleAllowsAll req.roles "plaintiff" ["record_opening_statement"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, record plaintiff opening statement." ["record_opening_statement"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" []
         })
    if !facts.hasOpeningDefendant && roleAllowsAll req.roles "defendant" ["record_opening_statement"] then
      actions := actions.concat
        ({ (mkTurn "defendant" "For case 0, record defense opening statement." ["record_opening_statement"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "defendant" []
         })
    if roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to plaintiff_case." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "plaintiff_case")]))))
  if c.phase = "plaintiff_case" then
    if !facts.hasTheoryPlaintiff && roleAllowsAll req.roles "plaintiff" ["submit_trial_theory"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, present plaintiff case by submitting trial theory." ["submit_trial_theory"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" []
         })
    if roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to plaintiff_evidence." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "plaintiff_evidence")]))))
  if c.phase = "plaintiff_evidence" then
    let maxExhibits := req.state.policy.max_exhibits_per_side
    let rested := hasDocketTitle c "Plaintiff case rested"
    if !rested then
      if partyCanOfferMoreExhibits c "plaintiff" maxExhibits &&
          roleAllowsAll req.roles "plaintiff" ["offer_case_file_as_exhibit", "rest_case"] then
        actions := actions.concat
          (mkTurn "plaintiff"
            s!"For case 0, plaintiff evidence phase: either offer one remaining case file as the next plaintiff exhibit or rest plaintiff case. Plaintiff may offer up to {maxExhibits} exhibits total."
            ["offer_case_file_as_exhibit", "rest_case"] true maxSteps)
      else if roleAllowsAll req.roles "plaintiff" ["rest_case"] then
        actions := actions.concat
          (mkTurn "plaintiff"
            "For case 0, no unoffered case files remain for plaintiff within the exhibit limit. Rest plaintiff case now."
            ["rest_case"] true 1)
    if rested && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to defense_case." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "defense_case")]))))
  if c.phase = "defense_case" then
    if !facts.hasTheoryDefendant && roleAllowsAll req.roles "defendant" ["submit_trial_theory"] then
      actions := actions.concat
        ({ (mkTurn "defendant" "For case 0, present defense case by submitting trial theory." ["submit_trial_theory"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "defendant" []
         })
    if roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to defense_evidence." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "defense_evidence")]))))
  if c.phase = "defense_evidence" then
    let maxExhibits := req.state.policy.max_exhibits_per_side
    let rested := hasDocketTitle c "Defense case rested"
    if !rested then
      if partyCanOfferMoreExhibits c "defendant" maxExhibits &&
          roleAllowsAll req.roles "defendant" ["offer_case_file_as_exhibit", "rest_case"] then
        actions := actions.concat
          (mkTurn "defendant"
            s!"For case 0, defense evidence phase: either offer one remaining case file as the next defense exhibit or rest defense case. Defendant may offer up to {maxExhibits} exhibits total."
            ["offer_case_file_as_exhibit", "rest_case"] true maxSteps)
      else if roleAllowsAll req.roles "defendant" ["rest_case"] then
        actions := actions.concat
          (mkTurn "defendant"
            "For case 0, no unoffered case files remain for defendant within the exhibit limit. Rest defense case now."
            ["rest_case"] true 1)
    if rested && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to plaintiff_rebuttal." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "plaintiff_rebuttal")]))))
  if c.phase = "plaintiff_rebuttal" then
    if !facts.hasRebuttalPlaintiff && roleAllowsAll req.roles "plaintiff" ["submit_trial_theory"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, present plaintiff rebuttal, limited to defense points." ["submit_trial_theory"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" []
         })
    if roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to plaintiff_rebuttal_evidence." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "plaintiff_rebuttal_evidence")]))))
  if c.phase = "plaintiff_rebuttal_evidence" then
    let maxExhibits := req.state.policy.max_exhibits_per_side
    let rested := hasDocketTitle c "Plaintiff rebuttal rested"
    if !rested then
      if partyCanOfferMoreExhibits c "plaintiff" maxExhibits &&
          roleAllowsAll req.roles "plaintiff" ["offer_case_file_as_exhibit", "rest_case"] then
        actions := actions.concat
          (mkTurn "plaintiff"
            s!"For case 0, plaintiff rebuttal evidence phase: either offer one remaining responsive case file as the next plaintiff exhibit or rest plaintiff rebuttal. Plaintiff may offer up to {maxExhibits} exhibits total."
            ["offer_case_file_as_exhibit", "rest_case"] true maxSteps)
      else if roleAllowsAll req.roles "plaintiff" ["rest_case"] then
        actions := actions.concat
          (mkTurn "plaintiff"
            "For case 0, no unoffered case files remain for plaintiff within the exhibit limit. Rest plaintiff rebuttal now."
            ["rest_case"] true 1)
    if rested && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to defense_surrebuttal." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "defense_surrebuttal")]))))
  if c.phase = "defense_surrebuttal" then
    if !facts.hasSurrebuttalDefendant && roleAllowsAll req.roles "defendant" ["submit_trial_theory"] then
      actions := actions.concat
        ({ (mkTurn "defendant" "For case 0, present defense surrebuttal, limited to new rebuttal points." ["submit_trial_theory"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "defendant" []
         })
    if roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to defense_surrebuttal_evidence." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "defense_surrebuttal_evidence")]))))
  if c.phase = "defense_surrebuttal_evidence" then
    let maxExhibits := req.state.policy.max_exhibits_per_side
    let rested := hasDocketTitle c "Defense surrebuttal rested"
    if !rested then
      if partyCanOfferMoreExhibits c "defendant" maxExhibits &&
          roleAllowsAll req.roles "defendant" ["offer_case_file_as_exhibit", "rest_case"] then
        actions := actions.concat
          (mkTurn "defendant"
            s!"For case 0, defense surrebuttal evidence phase: either offer one remaining responsive case file as the next defense exhibit or rest defense surrebuttal. Defendant may offer up to {maxExhibits} exhibits total."
            ["offer_case_file_as_exhibit", "rest_case"] true maxSteps)
      else if roleAllowsAll req.roles "defendant" ["rest_case"] then
        actions := actions.concat
          (mkTurn "defendant"
            "For case 0, no unoffered case files remain for defendant within the exhibit limit. Rest defense surrebuttal now."
            ["rest_case"] true 1)
    if rested && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to charge_conference." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "charge_conference")]))))
  if c.phase = "charge_conference" then
    if !facts.hasJuryInstructionProposalPlaintiff && roleAllowsAll req.roles "plaintiff" ["propose_jury_instruction"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, propose a plaintiff jury instruction with instruction_id PI-1 and concise instruction text." ["propose_jury_instruction"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" [("instruction_id", toJson "PI-1")]
         })
    if !facts.hasJuryInstructionProposalDefendant && roleAllowsAll req.roles "defendant" ["propose_jury_instruction"] then
      actions := actions.concat
        ({ (mkTurn "defendant" "For case 0, propose a defense jury instruction with instruction_id DI-1 and concise instruction text." ["propose_jury_instruction"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "defendant" [("instruction_id", toJson "DI-1")]
         })
    if facts.hasJuryInstructionProposalDefendant && !facts.hasJuryInstructionObjectionPlaintiff &&
        roleAllowsAll req.roles "plaintiff" ["object_jury_instruction"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, if appropriate, object to instruction_id DI-1 with concise legal grounds." ["object_jury_instruction"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" [("instruction_id", toJson "DI-1")]
         })
    if facts.hasJuryInstructionProposalPlaintiff && !facts.hasJuryInstructionObjectionDefendant &&
        roleAllowsAll req.roles "defendant" ["object_jury_instruction"] then
      actions := actions.concat
        ({ (mkTurn "defendant" "For case 0, if appropriate, object to instruction_id PI-1 with concise legal grounds." ["object_jury_instruction"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "defendant" [("instruction_id", toJson "PI-1")]
         })
    if facts.hasJuryInstructionProposalPlaintiff && facts.hasJuryInstructionProposalDefendant &&
        roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to closings." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "closings")]))))
  if c.phase = "closings" then
    if !facts.hasClosingPlaintiff && roleAllowsAll req.roles "plaintiff" ["deliver_closing_argument"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, deliver plaintiff closing argument." ["deliver_closing_argument"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" []
         })
    if facts.hasClosingPlaintiff && !facts.hasClosingDefendant && roleAllowsAll req.roles "defendant" ["deliver_closing_argument"] then
      actions := actions.concat
        ({ (mkTurn "defendant" "For case 0, deliver defense closing argument." ["deliver_closing_argument"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "defendant" []
         })
    if facts.hasClosingPlaintiff && facts.hasClosingDefendant &&
        !facts.hasClosingRebuttalPlaintiff &&
        roleAllowsAll req.roles "plaintiff" ["deliver_closing_argument"] then
      actions := actions.concat
        ({ (mkTurn "plaintiff" "For case 0, optionally deliver a plaintiff rebuttal closing argument limited to points raised in defense closing." ["deliver_closing_argument"] false maxSteps) with
            constraints := partyScopedPayloadConstraints "plaintiff" []
         })
    if facts.hasClosingPlaintiff && facts.hasClosingDefendant &&
        roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      if c.trial_mode = "jury" then
        actions := actions.concat (mkTurn "judge" "For case 0, advance phase to jury_charge." ["advance_trial_phase"] true 1
          (some (mkDeterministicSingleTool "advance_trial_phase"
            (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "jury_charge")]))))
      else
        actions := actions.concat (mkTurn "judge" "For case 0, advance phase to verdict_return." ["advance_trial_phase"] true 1
          (some (mkDeterministicSingleTool "advance_trial_phase"
            (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "verdict_return")]))))
  if c.phase = "jury_charge" then
    if !facts.hasJuryInstructionsSettled && roleAllowsAll req.roles "judge" ["settle_jury_instructions"] then
      actions := actions.concat (mkTurn "judge" "For case 0, settle jury instructions with a concise summary of the final instruction set." ["settle_jury_instructions"] true 1)
    if facts.hasJuryInstructionsSettled && !facts.hasJuryInstructionsDelivered &&
        roleAllowsAll req.roles "judge" ["deliver_jury_instructions"] then
      actions := actions.concat (mkTurn "judge" "For case 0, deliver the final jury instructions text in direct neutral language, without ceremonial address." ["deliver_jury_instructions"] true 1)
    if facts.hasJuryInstructionsDelivered && roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to deliberation." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "deliberation")]))))
  if c.phase = "deliberation" then
    if c.trial_mode = "jury" then
      let round := currentDeliberationRound c
      match nextSwornJurorWithoutVoteInRound? c round with
      | some juror =>
          if roleAllowsAll req.roles "juror" ["submit_juror_vote"] then
            actions := actions.concat
              ({ (mkTurn "juror"
                  s!"For case 0, deliberation round {round}: juror_id {juror.juror_id} must cast an individual verdict vote, state any damages amount if voting for plaintiff, give a confidence level, and explain the vote with reference to trial evidence."
                  ["submit_juror_vote"] false maxSteps) with
                    constraints := fixedPayloadConstraints [("juror_id", toJson juror.juror_id)]
               })
      | none => ()
      if (c.jury_verdict.isSome || c.hung_jury.isSome) &&
          roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
        actions := actions.concat (mkTurn "judge" "For case 0, advance phase to post_verdict." ["advance_trial_phase"] true 1
          (some (mkDeterministicSingleTool "advance_trial_phase"
            (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "post_verdict")]))))
    else if roleAllowsAll req.roles "judge" ["advance_trial_phase"] then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to verdict_return." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "verdict_return")]))))
  if c.phase = "verdict_return" then
    if c.trial_mode = "bench" && !facts.hasBenchOpinion &&
        roleAllowsAll req.roles "judge" ["file_bench_opinion"] then
      actions := actions.concat (mkTurn "judge" "For case 0, file a bench opinion explaining findings of fact, conclusions of law, and why judgment should be entered." ["file_bench_opinion"] true maxSteps)
    if roleAllowsAll req.roles "judge" ["advance_trial_phase"] &&
        ((c.trial_mode = "jury" && (c.jury_verdict.isSome || c.hung_jury.isSome)) ||
          (c.trial_mode = "bench" && facts.hasBenchOpinion)) then
      actions := actions.concat (mkTurn "judge" "For case 0, advance phase to post_verdict." ["advance_trial_phase"] true 1
        (some (mkDeterministicSingleTool "advance_trial_phase"
          (Json.mkObj [("case_id", toJson c.case_id), ("phase", toJson "post_verdict")]))))
  if c.phase = "post_verdict" then
    if c.hung_jury.isSome && roleAllowsAll req.roles "judge" ["transition_case"] then
      actions := actions.concat
        (mkTurn "judge" "For case 0, close the case after the hung jury." ["transition_case"] true 1
          (some (mkDeterministicSingleTool "transition_case"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("next_status", toJson "closed")
            ]))))
    else if judgmentEligibleFromCaseStateV1 c &&
        (c.trial_mode = "jury" || facts.hasBenchOpinion) &&
        roleAllowsAll req.roles "judge" ["enter_judgment"] then
      let basis := if c.trial_mode = "jury" then "jury verdict" else "bench verdict"
      actions := actions.concat (mkTurn "judge" s!"For case 0, enter judgment with basis {basis}." ["enter_judgment"] true 1
        (some (mkDeterministicSingleTool "enter_judgment"
          (Json.mkObj [
            ("case_id", toJson c.case_id),
            ("claim_id", toJson claimId),
            ("basis", toJson basis)
          ]))))
  actions

def postJudgmentCandidates (req : OpportunityRequest) (c : CaseState) (facts : TurnFacts) (maxSteps : Nat) : List OpportunitySpec := Id.run do
  let mut actions : List OpportunitySpec := []
  if c.status = "judgment_entered" && !facts.hasRule59Motion &&
      roleAllowsAll req.roles "defendant" ["file_rule59_motion"] then
    let payload := Json.mkObj [
      ("case_id", toJson c.case_id),
      ("last_judgment_date", toJson c.filed_on),
      ("filed_at", toJson c.filed_on)
    ]
    actions := actions.concat
      (mkTurn "defendant" "For case 0, if a new trial or alteration is sought, file a timely Rule 59 motion." ["file_rule59_motion"] false 1
        (some (mkDeterministicSingleTool "file_rule59_motion" payload)))
  if c.status = "judgment_entered" && facts.hasRule59Motion && !facts.hasRule59Order &&
      roleAllowsAll req.roles "judge" ["resolve_rule59_motion"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, resolve Rule 59 motion_index 0 as granted or denied." ["resolve_rule59_motion"] true 1
        (some
          (mkDeterministicSingleTool "resolve_rule59_motion"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("motion_index", toJson (0 : Nat)),
              ("granted", toJson false),
              ("order_text", toJson "Rule 59 motion denied")
            ]))))
  if c.status = "judgment_entered" && facts.hasDefaultJudgment && !facts.hasRule60Motion &&
      roleAllowsAll req.roles "defendant" ["file_rule60_motion"] then
    let payload := Json.mkObj [
      ("case_id", toJson c.case_id),
      ("ground", toJson "60b1_mistake"),
      ("ground_description", toJson "service-routing error and prompt corrective appearance"),
      ("last_judgment_date", toJson c.filed_on),
      ("filed_at", toJson c.filed_on)
    ]
    actions := actions.concat
      (mkTurn "defendant" "For case 0, if relief from default judgment is sought, file Rule 60 motion with ground and a timely filed_at date." ["file_rule60_motion"] false 1
        (some (mkDeterministicSingleTool "file_rule60_motion" payload)))
  if c.status = "judgment_entered" && !facts.hasDefaultJudgment && !facts.hasRule60Motion &&
      roleAllowsAll req.roles "defendant" ["file_rule60_motion"] then
    let payload := Json.mkObj [
      ("case_id", toJson c.case_id),
      ("ground", toJson "60b6_other"),
      ("ground_description", toJson "post-judgment relief requested"),
      ("last_judgment_date", toJson c.filed_on),
      ("filed_at", toJson c.filed_on)
    ]
    actions := actions.concat
      (mkTurn "defendant" "For case 0, if post-judgment relief is sought, file Rule 60 motion with ground and timely filed_at date." ["file_rule60_motion"] false 1
        (some (mkDeterministicSingleTool "file_rule60_motion" payload)))
  if c.status = "judgment_entered" && facts.hasRule60Motion && !facts.hasRule60Order &&
      roleAllowsAll req.roles "judge" ["resolve_rule60_motion"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, resolve Rule 60 motion_index 0 as granted or denied with a short relief summary." ["resolve_rule60_motion"] true maxSteps)
  if c.status = "judgment_entered" && !facts.hasSupersedeasBond &&
      roleAllowsAll req.roles "judge" ["post_supersedeas_bond"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, post supersedeas bond to secure judgment pending further proceedings." ["post_supersedeas_bond"] false 1
        (some
          (mkDeterministicSingleTool "post_supersedeas_bond"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("effective_until", toJson c.filed_on),
              ("note", toJson "bond posted")
            ]))))
  if c.status = "judgment_entered" && facts.hasSupersedeasBond && !facts.hasDiscretionaryStay &&
      roleAllowsAll req.roles "judge" ["order_discretionary_stay"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, order discretionary stay pending post-judgment motion resolution." ["order_discretionary_stay"] false 1
        (some
          (mkDeterministicSingleTool "order_discretionary_stay"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("start_on", toJson c.filed_on),
              ("end_on", toJson c.filed_on),
              ("reason", toJson "temporary stay")
            ]))))
  if c.status = "judgment_entered" && facts.hasDiscretionaryStay && !facts.hasStayLift &&
      roleAllowsAll req.roles "judge" ["lift_stay"] then
    actions := actions.concat
      (mkTurn "judge" "For case 0, lift discretionary stay when grounds no longer support continuation." ["lift_stay"] false 1
        (some
          (mkDeterministicSingleTool "lift_stay"
            (Json.mkObj [
              ("case_id", toJson c.case_id),
              ("stay_index", toJson (0 : Nat)),
              ("reason", toJson "stay lifted")
            ]))))
  actions

def availableOpportunities (req : OpportunityRequest) : List OpportunitySpec := Id.run do
  let c := req.state.case
  let maxSteps := if req.max_steps_per_turn = 0 then 1 else req.max_steps_per_turn
  let facts := collectTurnFacts c
  if c.status = "closed" then
    return []
  let actions :=
    if c.status = "filed" then
      filedCandidates req c facts maxSteps
    else if c.status = "pretrial" then
      pretrialCandidates req c facts maxSteps
    else if c.status = "trial" then
      trialCandidates req c facts maxSteps
    else if c.status = "judgment_entered" then
      postJudgmentCandidates req c facts maxSteps
    else
      []
  let allActions := actions ++ jurisdictionDismissalCandidates req c maxSteps
  return assignOpportunityIds (finalizeOpportunities c allActions)

def openOpportunities (req : OpportunityRequest) : List OpportunitySpec :=
  (availableOpportunities req).filter (fun action =>
    !(req.state.passed_opportunities.elem action.opportunity_id))

def selectLowestPriorityOpportunity? (actions : List OpportunitySpec) : Option OpportunitySpec :=
  actions.foldl
    (fun acc action =>
      match acc with
      | none => some action
      | some current =>
          if action.priority < current.priority then some action else acc)
    none

def nextOpportunity (req : OpportunityRequest) : NextOpportunityOk :=
  let actions := openOpportunities req
  match selectLowestPriorityOpportunity? actions with
  | some opportunity =>
      { state_version := req.state.state_version, opportunity := some opportunity }
  | none =>
      { terminal := true, reason := "no_eligible_opportunity", state_version := req.state.state_version }

def currentOpenOpportunity? (req : OpportunityRequest) : Option OpportunitySpec :=
  selectLowestPriorityOpportunity? (openOpportunities req)

def mkStepErr
    (message : String)
    (code : String := "")
    (details : Json := Json.null)
    (retryable : Bool := false)
    (actorMessage : String := "") : StepErr :=
  { ok := false
  , error := message
  , code := code
  , details := details
  , retryable := retryable
  , actor_message := if actorMessage.trimAscii.toString.isEmpty then message else actorMessage
  }

def applyDecisionAtOpportunity
    (state : CourtState)
    (opportunity : OpportunitySpec)
    (decision : DecisionSpec) : Except StepErr ApplyDecisionOk := do
  let decisionKind := decision.kind.trimAscii.toString
  if decisionKind = "pass" then
    if !opportunity.may_pass then
      throw <| mkStepErr
        s!"opportunity {opportunity.opportunity_id} does not allow pass"
        "PASS_NOT_ALLOWED"
        Json.null
        true
        "This opportunity requires action now.  Choose one of the allowed tools."
    let state := recordOpportunityPassFor state opportunity
    pure { result_kind := "pass_recorded", state := some state }
  else if decisionKind = "tool" then
    let toolName := match decision.tool_name with
      | some name => name.trimAscii.toString
      | none => ""
    if toolName.isEmpty then
      throw <| mkStepErr
        "tool decision missing tool_name"
        "MISSING_TOOL_NAME"
        Json.null
        true
        "Choose one allowed tool for this opportunity."
    if !(opportunity.allowed_tools.contains toolName) then
      throw <| mkStepErr
        s!"tool {toolName} is not allowed for opportunity {opportunity.opportunity_id}"
        "TOOL_NOT_ALLOWED"
        Json.null
        true
        s!"Tool {toolName} is not allowed here.  Choose one of the allowed tools."
    let payload := applyPayloadDefaults (decision.payload.getD Json.null) opportunity.constraints
    if let some (field, expected) := firstRequiredPayloadViolation? payload opportunity.constraints then
      throw <| mkStepErr
        s!"payload field {field} does not satisfy opportunity requirement"
        "PAYLOAD_CONSTRAINT_VIOLATION"
        (Json.mkObj [("field", toJson field), ("expected", expected)])
        true
        s!"For this opportunity, set {field} to {Json.compress expected}."
    pure {
      result_kind := "execute_tool"
      action := some {
        action_type := toolName
        actor_role := opportunity.role
        payload := payload
      }
    }
  else
    throw <| mkStepErr
      s!"unsupported decision kind: {decision.kind}"
      "UNSUPPORTED_DECISION_KIND"
      Json.null
      true
      "Submit either a tool decision or a pass decision."

def applyDecision (req : ApplyDecisionRequest) : Except StepErr ApplyDecisionOk := do
  if req.state.state_version != req.state_version then
    throw <| mkStepErr
      s!"stale opportunity state_version={req.state_version} current={req.state.state_version}"
      "STALE_OPPORTUNITY"
      Json.null
      false
      "That opportunity is no longer current.  Ask for the current opportunity and decide again."
  let baseReq : OpportunityRequest := { state := req.state, roles := req.roles, max_steps_per_turn := req.max_steps_per_turn }
  let opportunity ← match currentOpenOpportunity? baseReq with
    | some opportunity => pure opportunity
    | none =>
        throw <| mkStepErr
          "no current opportunity"
          "NO_CURRENT_OPPORTUNITY"
          Json.null
          false
          "No current opportunity is open for decision."
  if opportunity.opportunity_id != req.opportunity_id then
    throw <| mkStepErr
      s!"unexpected opportunity_id {req.opportunity_id}; current opportunity is {opportunity.opportunity_id}"
      "STALE_OPPORTUNITY"
      Json.null
      false
      "That opportunity is no longer current.  Ask for the current opportunity and decide again."
  let requestedRole := normalizePartyToken req.role
  let expectedRole := normalizePartyToken opportunity.role
  if requestedRole != expectedRole then
    throw <| mkStepErr
      s!"opportunity {req.opportunity_id} belongs to {opportunity.role}, not {req.role}"
      "WRONG_ROLE"
      Json.null
      false
      s!"This opportunity belongs to {opportunity.role}.  Only that role may act on it."
  applyDecisionAtOpportunity req.state opportunity req.decision

def step (s : CourtState) (a : CourtAction) : Except String CourtState := do
  if s.schema_version != "v1" then
    throw "unsupported schema version"
  let c := s.case
  match a.action_type with
  | "file_complaint" =>
      requireRole a ["plaintiff"]
      if hasDecisionTraceAction c "file_complaint" then
        throw "complaint already filed"
      if c.status != "filed" then
        throw "complaint filing requires filed status"
      let summary ← getString a.payload "summary"
      let filedByRaw ← match (← getStringOpt a.payload "filed_by") with
        | some v => pure v
        | none => pure "plaintiff"
      let filedBy := normalizePartyToken filedByRaw
      if filedBy != "plaintiff" then
        throw s!"invalid filed_by for complaint: {filedByRaw}"
      let juryDemandedOn := match (← getStringOpt a.payload "jury_demanded_on") with
        | some v => v
        | none => c.jury_demanded_on
      let c1 := appendTrace (appendDocket { c with jury_demanded_on := juryDemandedOn } "Complaint filed" summary) "file_complaint" "filed" ["FRCP 3", "FRCP 8(a)"]
      pure <| updateCase s c1
  | "file_answer" =>
      requireRole a ["defendant"]
      if !hasDecisionTraceAction c "file_complaint" then
        throw "cannot file answer before complaint is filed"
      if hasDecisionTraceAction c "file_answer" then
        throw "answer already filed"
      if c.status != "filed" then
        throw "answer filing requires filed status"
      let summary ← getString a.payload "summary"
      let filedByRaw ← match (← getStringOpt a.payload "filed_by") with
        | some v => pure v
        | none => pure "defendant"
      let filedBy := normalizePartyToken filedByRaw
      if filedBy != "defendant" then
        throw s!"invalid filed_by for answer: {filedByRaw}"
      let servedOn := match (← getStringOpt a.payload "served_on") with
        | some v => v
        | none => c.filed_on
      let c1 := appendTrace (appendDocket { c with last_pleading_served_on := servedOn } "Answer filed" summary) "file_answer" "filed" ["FRCP 8(b)", "FRCP 12(a)"]
      pure <| updateCase s c1
  | "file_amended_complaint" =>
      requireRole a ["plaintiff"]
      if !hasDecisionTraceAction c "file_complaint" then
        throw "cannot file amended complaint before complaint is filed"
      if c.status != "filed" && c.status != "pretrial" then
        throw "amended complaint filing requires filed or pretrial status"
      let summary ← getString a.payload "summary"
      let filedByRaw ← match (← getStringOpt a.payload "filed_by") with
        | some v => pure v
        | none => pure "plaintiff"
      let filedBy := normalizePartyToken filedByRaw
      if filedBy != "plaintiff" then
        throw s!"invalid filed_by for amended complaint: {filedByRaw}"
      let c1 := appendTrace (appendDocket (reopenRule56Windows c) "Amended complaint filed" summary) "file_amended_complaint" "filed" ["FRCP 15(a)"]
      pure <| updateCase s c1
  | "serve_initial_disclosures" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "pretrial" then
        throw "initial disclosures require pretrial status"
      let party ← getValidatedActorPartyField a "party"
      if countDocketTitleByPartyPrefix c "Initial Disclosures" party > 0 then
        throw s!"initial disclosures already served by {party}"
      let summary ← getString a.payload "summary"
      let c1 := appendTrace (appendDocket c "Initial Disclosures" s!"{party}: {summary}") "serve_initial_disclosures" "served" ["FRCP 26(a)(1)"]
      pure <| updateCase s c1
  | "import_case_file" =>
      requireRole a ["plaintiff", "defendant"]
      let fileId ← getString a.payload "file_id"
      if hasCaseFileId c fileId then
        throw s!"case file already exists: {fileId}"
      let importedAt ← getString a.payload "imported_at"
      let importedBy ← getValidatedActorPartyField a "imported_by"
      let label := match getStringOpt a.payload "label" with
        | .ok (some s) => s
        | _ => ""
      let originalName ← getString a.payload "original_name"
      let storageRelpath ← getString a.payload "storage_relpath"
      let sha256 ← getString a.payload "sha256"
      let sizeBytes ← getNat a.payload "size_bytes"
      let record := Json.mkObj [
        ("file_id", toJson fileId),
        ("imported_at", toJson importedAt),
        ("imported_by", toJson importedBy),
        ("label", toJson label),
        ("original_name", toJson originalName),
        ("storage_relpath", toJson storageRelpath),
        ("sha256", toJson sha256),
        ("size_bytes", toJson sizeBytes)
      ]
      let c1 := { c with case_files := c.case_files.concat record }
      let c2 := appendFileEvent c1 importedAt "import_case_file" fileId importedBy s!"source={storageRelpath}"
      let c3 := appendTrace (appendDocket c2 "Case File Imported" s!"{importedBy}: {fileId} {originalName}")
        "import_case_file" "imported" ["FRCP 26(a)(1)(A)(ii)"]
      pure <| updateCase s c3
  | "produce_case_file" =>
      requireRole a ["plaintiff", "defendant"]
      let fileId ← getString a.payload "file_id"
      if !hasCaseFileId c fileId then
        throw s!"unknown file_id: {fileId}"
      let producedBy ← getValidatedActorPartyField a "produced_by"
      let producedToRaw ← getString a.payload "produced_to"
      let producedTo := normalizePartyToken producedToRaw
      if !(producedTo = "plaintiff" || producedTo = "defendant") then
        throw s!"invalid produced_to: {producedToRaw}"
      if producedTo = producedBy then
        throw "file production requires opposing party recipient"
      let requestRef := match getStringOpt a.payload "request_ref" with
        | .ok (some s) => s
        | _ => ""
      let producedAt := match getStringOpt a.payload "produced_at" with
        | .ok (some s) => s
        | _ => c.filed_on
      let detail :=
        if requestRef = "" then s!"to={producedTo}" else s!"to={producedTo} request_ref={requestRef}"
      let c1 := appendFileEvent c producedAt "produce_case_file" fileId producedBy detail
      let c2 := appendTrace (appendDocket c1 "Case File Produced" s!"{producedBy} produced {fileId} to {producedTo}")
        "produce_case_file" "produced" ["FRCP 34", "FRCP 26(e)"]
      pure <| updateCase s c2
  | "set_last_pleading_served_on" =>
      requireRole a ["clerk"]
      let servedOn ← getString a.payload "served_on"
      pure <| updateCase s { c with last_pleading_served_on := servedOn }
  | "record_jury_demand" =>
      requireRole a ["clerk"]
      let demandedOn ← getString a.payload "demanded_on"
      pure <| updateCase s { c with jury_demanded_on := demandedOn }
  | "resolve_trial_mode" =>
      requireRole a ["judge"]
      let partiesStipulateNonjury ← getBoolD a.payload "parties_stipulate_nonjury" false
      let courtOrdersJury ← getBoolD a.payload "court_orders_jury" false
      let mode :=
        if partiesStipulateNonjury then
          "bench"
        else if courtOrdersJury then
          "jury"
        else if c.jury_demanded_on = "" then
          "bench"
        else
          "jury"
      let c1 := appendTrace { c with trial_mode := mode } "resolve_trial_mode" mode ["FRCP 38", "FRCP 39"]
      pure <| updateCase s c1
  | "transition_case" =>
      requireRole a ["judge"]
      let nextStatus ← getString a.payload "next_status"
      if !(allowedStatuses.contains nextStatus) then
        throw s!"invalid status: {nextStatus}"
      let currentStatus ← match parseCaseStatusV1 c.status with
        | some cs => pure cs
        | none => throw s!"invalid current status: {c.status}"
      let nextStatusV1 ← match parseCaseStatusV1 nextStatus with
        | some ns => pure ns
        | none => throw s!"invalid status: {nextStatus}"
      if !(canTransitionStatusV1 currentStatus nextStatusV1) then
        throw s!"invalid transition from {c.status} to {nextStatus}"
      if currentStatus = CaseStatusV1.trial && nextStatusV1 = CaseStatusV1.judgmentEntered then
        if c.jury_verdict.isNone then
          throw "cannot transition to judgment_entered without jury verdict"
        if c.hung_jury.isSome then
          throw "cannot transition to judgment_entered after hung jury"
      pure <| updateCase s { c with status := nextStatus }
  | "set_jury_configuration" =>
      requireRole a ["clerk"]
      if hasSwornJuror c.jurors then
        throw "cannot change jury configuration after jury is sworn"
      let jurorCount ← getNat a.payload "juror_count"
      let unanimous ← getBoolD a.payload "unanimous_required" true
      let minimumArg := parseMinConcurring a.payload
      if jurorCount < 6 || jurorCount > 12 then
        throw "jury size must be between 6 and 12"
      let required := if unanimous then jurorCount else if minimumArg = 0 then 6 else minimumArg
      if required < 6 || required > jurorCount then
        throw "minimum concurring jurors must be between 6 and jury size"
      let cfg : JuryConfiguration := { juror_count := jurorCount, unanimous_required := unanimous, minimum_concurring := required }
      let c1 := appendTrace { c with jury_configuration := some cfg } "set_jury_configuration" s!"{required}_of_{jurorCount}" ["FRCP 48(a)", "FRCP 48(b)"]
      pure <| updateCase s c1
  | "add_juror" =>
      requireRole a ["clerk"]
      if hasSwornJuror c.jurors then
        throw "cannot add juror after jury is sworn"
      let jurorId ← getString a.payload "juror_id"
      let name ← getString a.payload "name"
      let model := match getStringOpt a.payload "model" with
        | .ok (some v) => v
        | _ => ""
      let personaFilename := match getStringOpt a.payload "persona_filename" with
        | .ok (some v) => v
        | _ => ""
      if c.jurors.any (fun j => j.juror_id = jurorId) then
        throw s!"duplicate juror id: {jurorId}"
      let juror : JurorRecord := {
        juror_id := jurorId,
        name := name,
        status := "candidate",
        model := model,
        persona_filename := personaFilename
      }
      let c1 := appendTrace { c with jurors := c.jurors.concat juror } "add_juror" "candidate" ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "issue_juror_questionnaire" =>
      requireRole a ["judge"]
      validateTrialActionPhase c .issueJurorQuestionnaire
        s!"issuing the juror questionnaire requires voir_dire phase; current phase is {c.phase}"
      if jurorQuestionnaireIssued c then
        throw "juror questionnaire already issued"
      if countCandidates c.jurors = 0 then
        throw "cannot issue juror questionnaire without candidate jurors"
      let questionnaire := defaultJurorQuestionnaire c
      let c1 := appendTrace
        (appendDocket { c with juror_questionnaire := questionnaire } "Juror questionnaire issued"
          s!"questions={questionnaire.length}")
        "issue_juror_questionnaire" s!"{questionnaire.length}_questions" ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "answer_juror_questionnaire" =>
      requireRole a ["juror"]
      validateTrialActionPhase c .answerJurorQuestionnaire
        s!"juror questionnaire answer requires voir_dire phase; current phase is {c.phase}"
      let jurorId ← getString a.payload "juror_id"
      let answers ← getJurorQuestionnaireAnswers a.payload "answers"
      if !jurorQuestionnaireIssued c then
        throw "juror questionnaire has not been issued"
      match jurorById? c.jurors jurorId with
      | none => throw s!"unknown juror_id: {jurorId}"
      | some juror =>
          if juror.status != "candidate" then
            throw s!"juror {jurorId} is not a current candidate"
          pure ()
      if hasQuestionnaireResponseFor c jurorId then
        throw s!"juror questionnaire already answered: {jurorId}"
      if !questionnaireAnswersMatch c.juror_questionnaire answers then
        throw "juror questionnaire answers must answer each issued question exactly once"
      let response : JurorQuestionnaireResponse := {
        juror_id := jurorId
        answers := answers
        submitted_at := c.filed_on
      }
      let c1 := appendTrace
        (appendDocket { c with juror_questionnaire_responses := c.juror_questionnaire_responses.concat response }
          "Juror questionnaire answered" s!"{jurorId}: completed")
        "answer_juror_questionnaire" jurorId ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "swear_jury" =>
      requireRole a ["clerk"]
      let cfg ← match c.jury_configuration with
        | some cfg => pure cfg
        | none => throw "jury configuration required before swearing jury"
      let candidates := countCandidates c.jurors
      if candidates < cfg.juror_count then
        throw "insufficient candidate jurors to swear jury"
      let swornJurors := swearAvailable c.jurors cfg.juror_count
      let c1 := appendTrace { c with jurors := swornJurors } "swear_jury" s!"{cfg.juror_count}_sworn" ["FRCP 47(b)"]
      pure <| updateCase s c1
  | "record_voir_dire_question" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "trial" then
        throw "record voir dire question requires trial status"
      if c.phase != "voir_dire" then
        throw s!"record voir dire question requires voir_dire phase; current phase is {c.phase}"
      let jurorId ← getString a.payload "juror_id"
      if !jurorAvailableForVoirDire c jurorId then
        throw s!"unknown juror_id: {jurorId}"
      let question ← getString a.payload "question"
      let askedBy := normalizePartyToken a.actor_role
      let maxQuestions := effectiveVoirDireQuestionsPerSide s.policy
      if countDisallowedVoirDireQuestionsFrom c askedBy >= s.policy.max_disallowed_voir_dire_questions_per_side then
        throw s!"voir dire disallow limit reached for {askedBy}"
      if countAnsweredVoirDireQuestionsFrom c jurorId askedBy >= maxQuestions then
        throw s!"voir dire questioning limit reached for {askedBy} and {jurorId}"
      if hasPendingVoirDireQuestionFrom c jurorId askedBy then
        throw s!"pending voir dire question already exists for {askedBy} and {jurorId}"
      let exchange : VoirDireExchange := {
        exchange_id := nextVoirDireExchangeId c.voir_dire_exchanges
        juror_id := jurorId
        asked_by := askedBy
        question := question
        asked_at := c.filed_on
      }
      let c1 := appendTrace
        (appendDocket { c with voir_dire_exchanges := c.voir_dire_exchanges.concat exchange } "Voir dire question proposed"
          s!"{askedBy} to {jurorId}: {question}")
        "record_voir_dire_question" s!"{askedBy}:{jurorId}" ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "decide_voir_dire_question" =>
      requireRole a ["judge"]
      validateTrialActionPhase c .decideVoirDireQuestion
        s!"voir dire ruling requires voir_dire phase; current phase is {c.phase}"
      let exchangeId ← getString a.payload "exchange_id"
      let jurorId ← getString a.payload "juror_id"
      let allowed ← match (← getBoolOpt a.payload "allowed") with
        | some value => pure value
        | none => throw "decide_voir_dire_question requires boolean field allowed"
      let rulingReason ← getString a.payload "ruling_reason"
      let exchange ← match c.voir_dire_exchanges.find? (fun item =>
          item.exchange_id = exchangeId &&
          item.juror_id = jurorId &&
          item.judge_allowed.isNone) with
        | some item => pure item
        | none => throw s!"unknown pending voir dire exchange for ruling: {exchangeId}"
      let updatedExchanges := c.voir_dire_exchanges.map (fun item =>
        if item.exchange_id = exchangeId then
          { item with
              judge_allowed := some allowed
              ruling_reason := rulingReason
              ruled_at := some c.filed_on }
        else
          item)
      let rulingTitle := if allowed then "Voir dire question allowed" else "Voir dire question disallowed"
      let rulingDesc :=
        if allowed then
          s!"judge allowed {exchange.asked_by} question to {jurorId}: {rulingReason}"
        else
          s!"judge disallowed {exchange.asked_by} question to {jurorId}: {rulingReason}"
      let c1 := appendTrace
        (appendDocket { c with voir_dire_exchanges := updatedExchanges } rulingTitle rulingDesc)
        "decide_voir_dire_question" s!"{exchange.asked_by}:{if allowed then "allowed" else "disallowed"}:{jurorId}" ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "answer_voir_dire_question" =>
      requireRole a ["juror"]
      validateTrialActionPhase c .answerVoirDireQuestion
        s!"voir dire answer requires voir_dire phase; current phase is {c.phase}"
      let exchangeId ← getString a.payload "exchange_id"
      let jurorId ← getString a.payload "juror_id"
      let response ← getString a.payload "response"
      if !jurorAvailableForVoirDire c jurorId then
        throw s!"unknown juror_id: {jurorId}"
      let exchange ← match c.voir_dire_exchanges.find? (fun item =>
          item.exchange_id = exchangeId &&
          item.juror_id = jurorId &&
          item.judge_allowed = some true &&
          trimString item.response = "") with
        | some item => pure item
        | none => throw s!"unknown pending voir dire exchange: {exchangeId}"
      let updatedExchanges := c.voir_dire_exchanges.map (fun item =>
        if item.exchange_id = exchangeId then
          { item with response := response, answered_at := some c.filed_on }
        else
          item)
      let c1 := appendTrace
        (appendDocket { c with voir_dire_exchanges := updatedExchanges } "Voir dire answer"
          s!"{jurorId} to {exchange.asked_by}: {response}")
        "answer_voir_dire_question" jurorId ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "challenge_juror_for_cause" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "trial" then
        throw "challenge for cause requires trial status"
      if c.phase != "voir_dire" then
        throw s!"challenge for cause requires voir_dire phase; current phase is {c.phase}"
      let jurorId ← getString a.payload "juror_id"
      if !jurorAvailableForVoirDire c jurorId then
        throw s!"unknown juror_id: {jurorId}"
      let byParty := normalizePartyToken a.actor_role
      let grounds ← getString a.payload "grounds"
      if !jurorReadyForEmpanelment c jurorId
          (effectiveVoirDireQuestionsPerSide s.policy)
          s.policy.max_disallowed_voir_dire_questions_per_side then
        throw s!"cannot seek excusal for cause of {jurorId} before both sides finish voir dire questioning"
      if nextPendingForCauseChallenge? c |>.isSome then
        throw "judge must decide the pending for-cause challenge before another is filed"
      if countForCauseChallengesBy c byParty >= s.policy.max_for_cause_challenges_per_side then
        throw s!"for-cause challenge limit reached for {byParty}"
      if hasForCauseChallengeRequestBy c byParty jurorId then
        throw s!"for-cause challenge already requested by {byParty} for {jurorId}"
      let challenge : ForCauseChallenge := {
        challenge_id := nextForCauseChallengeId c.for_cause_challenges
        juror_id := jurorId
        by_party := byParty
        grounds := grounds
        requested_at := c.filed_on
      }
      let c1 := appendTrace
        (appendDocket { c with for_cause_challenges := c.for_cause_challenges.concat challenge } "For-cause challenge requested"
          s!"{byParty} requested excusal of {jurorId}: {grounds}")
        "challenge_juror_for_cause" s!"{byParty}:requested:{jurorId}" ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "decide_juror_for_cause_challenge" =>
      requireRole a ["judge"]
      validateTrialActionPhase c .decideJurorForCauseChallenge
        s!"for-cause challenge decision requires voir_dire phase; current phase is {c.phase}"
      let challengeId ← getString a.payload "challenge_id"
      let jurorId ← getString a.payload "juror_id"
      let byPartyRaw ← getString a.payload "by_party"
      let byParty := normalizePartyToken byPartyRaw
      let granted ← getBoolD a.payload "granted" false
      let rulingReason ← getString a.payload "ruling_reason"
      let challenge ← match c.for_cause_challenges.find? (fun item =>
          item.challenge_id = challengeId &&
          item.juror_id = jurorId &&
          normalizePartyToken item.by_party = byParty &&
          item.granted.isNone) with
        | some item => pure item
        | none => throw s!"unknown pending for-cause challenge: {challengeId}"
      let updatedChallenges := c.for_cause_challenges.map (fun item =>
        if item.challenge_id = challengeId then
          { item with granted := some granted, decided_at := some c.filed_on, ruling_reason := rulingReason }
        else
          item)
      let updatedJurors :=
        if granted then
          setJurorStatus c.jurors jurorId "excused_for_cause"
        else
          c.jurors
      let c1 := appendTrace
        (appendDocket { c with jurors := updatedJurors, for_cause_challenges := updatedChallenges } "For-cause challenge decided"
          s!"{byParty} challenge to {jurorId}: granted={granted}; reason={rulingReason}")
        "decide_juror_for_cause_challenge" s!"{byParty}:{if granted then "granted" else "denied"}:{challenge.juror_id}" ["FRCP 47(a)"]
      pure <| updateCase s c1
  | "strike_juror_peremptorily" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "trial" then
        throw "peremptory strike requires trial status"
      if c.phase != "voir_dire" then
        throw s!"peremptory strike requires voir_dire phase; current phase is {c.phase}"
      let jurorId ← getString a.payload "juror_id"
      if !jurorAvailableForVoirDire c jurorId then
        throw s!"unknown juror_id: {jurorId}"
      let byParty ← getValidatedActorPartyField a "party"
      if countPeremptoryChallengesBy c byParty >= s.policy.max_peremptory_challenges_per_side then
        throw s!"peremptory challenge limit reached for {byParty}"
      if !jurorReadyForEmpanelment c jurorId
          (effectiveVoirDireQuestionsPerSide s.policy)
          s.policy.max_disallowed_voir_dire_questions_per_side then
        throw s!"cannot strike {jurorId} before both sides finish voir dire questioning"
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some r) => r
        | _ => ""
      let nextJurors := setJurorStatus c.jurors jurorId "struck_peremptory"
      let c1 := appendTrace
        (appendDocket { c with jurors := nextJurors } "Peremptory strike"
          s!"{byParty} struck {jurorId}; reason={reason}")
        "strike_juror_peremptorily" s!"{byParty}:{jurorId}" ["FRCP 47"]
      pure <| updateCase s c1
  | "empanel_jury" =>
      requireRole a ["judge"]
      validateTrialActionPhase c .empanelJury
        s!"jury empanelment requires voir_dire phase; current phase is {c.phase}"
      let cfg ← match c.jury_configuration with
        | some value => pure value
        | none => throw "jury configuration required before empanelment"
      let maxQuestions := effectiveVoirDireQuestionsPerSide s.policy
      let maxDisallowed := s.policy.max_disallowed_voir_dire_questions_per_side
      if !skipVoirDire s.policy then
        if nextPendingVoirDireRuling? c |>.isSome then
          throw "cannot empanel jury while a voir dire ruling is pending"
        if nextPendingVoirDireExchange? c |>.isSome then
          throw "cannot empanel jury while a voir dire answer is pending"
        if nextPendingForCauseChallenge? c |>.isSome then
          throw "cannot empanel jury while a for-cause challenge is pending"
        if !allAvailableJurorsReadyForEmpanelment c maxQuestions maxDisallowed then
          throw "cannot empanel jury before both sides finish questioning the candidate panel"
      let selectedJurorIds ← getStringList a.payload "juror_ids"
      if selectedJurorIds.length != cfg.juror_count then
        throw s!"empanel_jury requires exactly {cfg.juror_count} juror_ids"
      let rec hasDuplicate (seen : List String) (remaining : List String) : Bool :=
        match remaining with
        | [] => false
        | head :: tail => if seen.contains head then true else hasDuplicate (head :: seen) tail
      if hasDuplicate [] selectedJurorIds then
        throw "empanel_jury juror_ids must be unique"
      selectedJurorIds.forM (fun jurorId => do
        match jurorById? c.jurors jurorId with
        | none => throw s!"unknown juror_id: {jurorId}"
        | some juror =>
            if juror.status != "candidate" then
              throw s!"juror {jurorId} is not available for empanelment"
            if !skipVoirDire s.policy && !jurorReadyForEmpanelment c jurorId maxQuestions maxDisallowed then
              throw s!"juror {jurorId} is not ready for empanelment"
            pure ())
      if countCandidates c.jurors < cfg.juror_count then
        throw "insufficient candidate jurors to empanel the jury"
      let updatedJurors := empanelSelectedJurors c.jurors selectedJurorIds
      let c1 := appendTrace
        (appendDocket { c with jurors := updatedJurors } "Jury empaneled"
          s!"selected jurors: {String.intercalate ", " selectedJurorIds}")
        "empanel_jury" (String.intercalate "," selectedJurorIds) ["FRCP 47(b)", "FRCP 48(a)"]
      pure <| updateCase s c1
  | "object_to_evidence" =>
      requireRole a ["judge", "plaintiff", "defendant"]
      if c.status != "trial" then
        throw "objection to evidence requires trial status"
      let issue ← getString a.payload "issue"
      let grounds ← getString a.payload "grounds"
      let ruling ← getString a.payload "ruling"
      let c1 := appendTrace (appendDocket c "Trial objection" s!"{issue}: {grounds} ({ruling})")
        "record_trial_objection" ruling ["FRE 103", "FRCP 46"]
      pure <| updateCase s c1
  | "enter_protective_order" =>
      requireRole a ["judge"]
      let scope ← getString a.payload "scope"
      let target ← getString a.payload "target"
      let note := match getStringOpt a.payload "note" with
        | .ok (some v) => v
        | _ => ""
      let allowedRolesRaw ← getStringList a.payload "allowed_roles"
      let allowedRoles := allowedRolesRaw.map trimString |>.filter (fun r => r != "")
      if allowedRoles.isEmpty then
        throw "allowed_roles must contain at least one role"
      let orderId := match getStringOpt a.payload "order_id" with
        | .ok (some v) =>
          let trimmed := trimString v
          if trimmed = "" then assignProtectiveOrderId c.protective_orders else trimmed
        | _ => assignProtectiveOrderId c.protective_orders
      if c.protective_orders.any (fun o => o.order_id = orderId) then
        throw s!"duplicate protective order id: {orderId}"
      let order : ProtectiveOrder := {
        entered_at := c.filed_on
        order_id := orderId
        scope := trimString scope
        target := trimString target
        allowed_roles := allowedRoles
        note := trimString note
        active := true
        lifted_at := none
      }
      let c1 := appendTrace
        (appendDocket { c with protective_orders := c.protective_orders.concat order }
          s!"Protective Order {orderId}" s!"scope={order.scope} target={order.target}")
        "enter_protective_order" orderId ["FRCP 26(c)"]
      pure <| updateCase s c1
  | "lift_protective_order" =>
      requireRole a ["judge"]
      let orderIdRaw ← getString a.payload "order_id"
      let orderId := trimString orderIdRaw
      if orderId = "" then
        throw "order_id is required"
      let note := match getStringOpt a.payload "note" with
        | .ok (some v) => trimString v
        | _ => ""
      let liftedAt := c.filed_on
      let updatedOrders ← match liftProtectiveOrder c.protective_orders orderId note liftedAt with
        | some orders => pure orders
        | none => throw s!"unknown protective order: {orderId}"
      let c1 := appendTrace
        (appendDocket { c with protective_orders := updatedOrders }
          s!"Protective Order {orderId} lifted" note)
        "lift_protective_order" orderId ["FRCP 26(c)"]
      pure <| updateCase s c1
  | "add_bench_finding" =>
      requireRole a ["judge"]
      if c.status != "trial" then
        throw "bench finding requires trial status"
      let issue ← getString a.payload "issue"
      let finding ← getString a.payload "finding"
      let entry : BenchFinding := {
        issue := issue
        finding := finding
        entered_at := c.filed_on
      }
      let c1 := appendTrace { c with bench_findings := c.bench_findings.concat entry }
        "add_bench_finding" issue ["FRCP 52(a)"]
      pure <| updateCase s c1
  | "add_bench_conclusion" =>
      requireRole a ["judge"]
      if c.status != "trial" then
        throw "bench conclusion requires trial status"
      let issue ← getString a.payload "issue"
      let conclusion ← getString a.payload "conclusion"
      let entry : BenchConclusion := {
        issue := issue
        conclusion := conclusion
        entered_at := c.filed_on
      }
      let c1 := appendTrace { c with bench_conclusions := c.bench_conclusions.concat entry }
        "add_bench_conclusion" issue ["FRCP 52(a)"]
      pure <| updateCase s c1
  | "submit_juror_vote" =>
      requireRole a ["juror"]
      validateTrialActionPhase c .submitJurorVote
        s!"juror vote requires deliberation phase; current phase is {c.phase}"
      let round := currentDeliberationRound c
      let jurorId ← getString a.payload "juror_id"
      let vote ← getString a.payload "vote"
      let damages ← getFloatD a.payload "damages" 0.0
      let confidence ← getString a.payload "confidence"
      let explanation ← getString a.payload "explanation"
      match jurorById? c.jurors jurorId with
      | none =>
        throw s!"unknown juror_id: {jurorId}"
      | some juror =>
          if juror.status != "sworn" then
            throw s!"juror {jurorId} is not sworn"
          pure ()
      if hasJurorVoteForRound c round jurorId then
        throw s!"juror vote already submitted for round {round}: {jurorId}"
      if !(vote = "plaintiff" || vote = "defendant") then
        throw s!"invalid juror vote: {vote}"
      if damages < 0.0 then
        throw "juror vote damages must be nonnegative"
      if vote = "defendant" && damages != 0.0 then
        throw "juror vote damages must be zero on a defense vote"
      if !(confidence = "high" || confidence = "medium" || confidence = "low") then
        throw s!"invalid confidence: {confidence}"
      let entry : JurorVote := {
        juror_id := jurorId
        round := round
        vote := vote
        damages := damages
        confidence := confidence
        explanation := explanation
        submitted_at := c.filed_on
      }
      let c1 := appendTrace
        (appendDocket { c with juror_votes := c.juror_votes.concat entry }
          "Juror vote filed" s!"round={round} {jurorId}: vote={vote} damages={damages} confidence={confidence}")
        "submit_juror_vote" jurorId ["FRCP 48(b)", "FRCP 48(c)"]
      let c2 := applyDerivedDeliberationOutcome s.policy c1
      pure <| updateCase s c2
  | "process_juror_timeout" =>
      if a.actor_role != "system" then
        throw s!"role {a.actor_role} not permitted for {a.action_type}"
      let jurorId ← getString a.payload "juror_id"
      match jurorById? c.jurors jurorId with
      | none =>
          throw s!"unknown juror_id: {jurorId}"
      | some juror =>
          if juror.status = "candidate" then
            if c.phase != "voir_dire" then
              throw s!"candidate juror timeout requires voir_dire phase; current phase is {c.phase}"
            let c1 := appendTrace
              (appendDocket { c with jurors := setJurorStatus c.jurors jurorId "timed_out" }
                "Juror timed out" s!"{jurorId} timed out during voir dire and was excused from the candidate panel")
              "process_juror_timeout" s!"candidate:{jurorId}" ["FRCP 47(a)"]
            pure <| updateCase s c1
          else if juror.status = "sworn" then
            if c.phase != "deliberation" then
              throw s!"sworn juror timeout requires deliberation phase; current phase is {c.phase}"
            let c1 := appendTrace
              (appendDocket { c with jurors := setJurorStatus c.jurors jurorId "timed_out" }
                "Juror timed out" s!"{jurorId} timed out during deliberation and will not participate further")
              "process_juror_timeout" s!"sworn:{jurorId}" ["FRCP 48(b)", "FRCP 48(c)"]
            let c2 := applyDerivedDeliberationOutcome s.policy c1
            pure <| updateCase s c2
          else
            throw s!"juror {jurorId} is neither a current candidate nor a sworn juror"
  | "enter_pretrial_order" =>
      requireRole a ["judge"]
      if c.status != "pretrial" then
        throw "pretrial order requires pretrial status"
      let text ← getString a.payload "text"
      if trimString text = "" then
        throw "pretrial order text is required"
      let c1 := appendTrace (appendDocket c "Pretrial Order" s!"pretrial_order {text}")
        "enter_pretrial_order" "entered" ["FRCP 16(e)"]
      pure <| updateCase s c1
  | "advance_trial_phase" =>
      requireRole a ["judge"]
      runAdvanceTrialPhase s a.payload
  | "record_opening_statement" =>
      requireRole a ["plaintiff", "defendant"]
      validateTrialActionPhase c .recordOpeningStatement
        s!"opening statement requires openings phase; current phase is {c.phase}"
      let party ← getValidatedActorPartyField a "party"
      let summary ← getString a.payload "summary"
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s party phase c.filed_on "text.opening_chars_per_side"
        "opening_statement_chars" summary.length
      let usage := setLimitUsage c.limit_usage "text.opening_chars_per_side" party phase summary.length
      pure <| updateCase s (appendDocket { c with limit_usage := usage } s!"Opening statement - {party}" summary)
  | "submit_trial_theory" =>
      requireRole a ["plaintiff", "defendant"]
      validateTrialActionPhase c .submitPresentation
        s!"presentation requires plaintiff_case, defense_case, plaintiff_rebuttal, or defense_surrebuttal phase; current phase is {c.phase}"
      let party ← getValidatedActorPartyField a "party"
      let theory ← getString a.payload "theory"
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s party phase c.filed_on "text.trial_theory_chars_per_side"
        "trial_theory_chars" theory.length
      let usage := setLimitUsage c.limit_usage "text.trial_theory_chars_per_side" party phase theory.length
      let title ← trialPresentationTitle c.phase party
      pure <| updateCase s (appendDocket { c with limit_usage := usage } title theory)
  | "submit_technical_report" =>
      requireRole a ["plaintiff", "defendant"]
      let party ← getValidatedActorPartyField a "party"
      let reportId ← getString a.payload "report_id"
      let title ← getString a.payload "title"
      let summary ← getString a.payload "summary"
      let methodNotes := match getStringOpt a.payload "method_notes" with
        | .ok (some v) => v
        | _ => ""
      let limitations := match getStringOpt a.payload "limitations" with
        | .ok (some v) => v
        | _ => ""
      let fileId := match getStringOpt a.payload "file_id" with
        | .ok (some v) => v
        | _ => ""
      if c.status = "pretrial" then
        pure ()
      else if c.status = "trial" && (c.phase = "plaintiff_case" || c.phase = "defense_case") then
        pure ()
      else
        throw s!"technical report submission requires pretrial or party case-in-chief phase; current status={c.status}, phase={c.phase}"
      let phase := normalizePhaseForLimit c.phase
      let reportsAttempted := countTechnicalReportsByParty c party + 1
      let _ ← enforceMeasuredLimit s party phase c.filed_on "reports.per_side_count"
        "technical_reports_submitted" reportsAttempted
      let _ ← enforceMeasuredLimit s party phase c.filed_on "reports.summary_chars_per_report"
        "technical_report_summary_chars" summary.length
      let usage1 := setLimitUsage c.limit_usage "reports.per_side_count" party phase reportsAttempted
      let usage := setLimitUsage usage1 "reports.summary_chars_per_report" party phase summary.length
      let report : TechnicalReport := {
        report_id := reportId
        party := party
        title := title
        summary := summary
        method_notes := methodNotes
        limitations := limitations
        file_id := fileId
        submitted_at := c.filed_on
      }
      let c1 := appendTrace
        (appendDocket { c with technical_reports := c.technical_reports.concat report, limit_usage := usage }
          s!"Technical report - {party}"
          s!"report_id={reportId} title={title} summary={summary} method_notes={methodNotes} limitations={limitations} file_id={fileId}")
        "submit_technical_report" reportId ["FRCP 26(a)(1)(A)(ii)", "FRCP 26(e)"]
      pure <| updateCase s c1
  | "propose_jury_instruction" =>
      requireRole a ["plaintiff", "defendant"]
      if c.phase != "charge_conference" then
        throw s!"jury instruction proposal requires charge_conference phase; current phase is {c.phase}"
      let party ← getValidatedActorPartyField a "party"
      let instructionId ← getString a.payload "instruction_id"
      let text ← getString a.payload "text"
      let rationale := match getStringOpt a.payload "rationale" with
        | .ok (some v) => v
        | _ => ""
      let c1 := appendTrace
        (appendDocket c s!"Proposed jury instruction - {party}"
          s!"instruction_id={instructionId} text={text} rationale={rationale}")
        "propose_jury_instruction" instructionId ["FRCP 51(a)"]
      pure <| updateCase s c1
  | "object_jury_instruction" =>
      requireRole a ["plaintiff", "defendant"]
      if c.phase != "charge_conference" then
        throw s!"jury instruction objection requires charge_conference phase; current phase is {c.phase}"
      let party ← getValidatedActorPartyField a "party"
      let instructionId ← getString a.payload "instruction_id"
      let grounds ← getString a.payload "grounds"
      let c1 := appendTrace
        (appendDocket c s!"Jury instruction objection - {party}"
          s!"instruction_id={instructionId} grounds={grounds}")
        "object_jury_instruction" instructionId ["FRCP 51(c)"]
      pure <| updateCase s c1
  | "settle_jury_instructions" =>
      requireRole a ["judge"]
      if c.phase != "jury_charge" then
        throw s!"settle jury instructions requires jury_charge phase; current phase is {c.phase}"
      if !hasAnyDocketTitlePrefix c "Proposed jury instruction - " then
        throw "cannot settle jury instructions before any proposed instructions are filed"
      let summary ← getString a.payload "summary"
      let c1 := appendTrace
        (appendDocket c "Jury instructions settled" summary)
        "settle_jury_instructions" "settled" ["FRCP 51(b)"]
      pure <| updateCase s c1
  | "deliver_jury_instructions" =>
      requireRole a ["judge"]
      if c.phase != "jury_charge" then
        throw s!"deliver jury instructions requires jury_charge phase; current phase is {c.phase}"
      if !hasDocketTitle c "Jury instructions settled" then
        throw "cannot deliver jury instructions before settlement"
      let text ← getString a.payload "text"
      let c1 := appendTrace
        (appendDocket c "Jury instructions delivered" text)
        "deliver_jury_instructions" "delivered" ["FRCP 51(b)"]
      pure <| updateCase s c1
  | "offer_exhibit" =>
      requireRole a ["plaintiff", "defendant"]
      validateTrialActionPhase c .offerExhibit
        s!"offer exhibit requires evidence phase; current phase is {c.phase}"
      let party ← getValidatedActorPartyField a "party"
      let offeredCount := countExhibitsOfferedByParty c party
      let nextCount := offeredCount + 1
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s party phase c.filed_on "trial.exhibits_offered_per_side"
        "exhibits_offered" nextCount
      let exhibitId ← getString a.payload "exhibit_id"
      let description ← getString a.payload "description"
      let admitted ← getBoolD a.payload "admitted" true
      let fileId := trimString ((← getStringOpt a.payload "file_id").getD "")
      let offeredAt := trimString ((← getStringOpt a.payload "offered_at").getD c.filed_on)
      if fileId != "" && !hasCaseFileId c fileId then
        throw s!"unknown case file: {fileId}"
      let usage := setLimitUsage c.limit_usage "trial.exhibits_offered_per_side" party phase nextCount
      let c1 := { c with limit_usage := usage }
      let c2 :=
        if fileId = "" then c1
        else appendFileEvent c1 offeredAt "offer_case_file_as_exhibit" fileId party s!"exhibit_id={exhibitId} admitted={admitted}"
      pure <| updateCase s (appendDocket c2 s!"Exhibit {exhibitId} - {if admitted then "admitted" else "excluded"}" s!"{party}: {description}")
  | "rest_case" =>
      requireRole a ["plaintiff", "defendant"]
      validateTrialActionPhase c .restCase
        s!"rest_case requires evidence phase; current phase is {c.phase}"
      let title ← evidenceRestTitle c.phase
      let party := normalizePartyToken a.actor_role
      pure <| updateCase s (appendDocket c title s!"{party}: rested")
  | "deliver_closing_argument" =>
      requireRole a ["plaintiff", "defendant"]
      validateTrialActionPhase c .deliverClosingArgument
        s!"closing argument requires closings phase; current phase is {c.phase}"
      let party ← getValidatedActorPartyField a "party"
      let argument ← getString a.payload "argument"
      let hasClosingPlaintiff := hasDocketTitle c "Closing argument - plaintiff"
      let hasClosingDefendant := hasDocketTitle c "Closing argument - defendant"
      let hasClosingRebuttalPlaintiff := hasDocketTitle c "Closing rebuttal - plaintiff"
      let title ←
        if party = "plaintiff" then
          if hasClosingPlaintiff then
            if hasClosingDefendant && !hasClosingRebuttalPlaintiff then
              pure "Closing rebuttal - plaintiff"
            else
              throw "plaintiff closing already recorded"
          else
            pure "Closing argument - plaintiff"
        else
          if !hasClosingPlaintiff then
            throw "defendant closing requires plaintiff closing first"
          else if hasClosingDefendant then
            throw "defendant closing already recorded"
          else
            pure "Closing argument - defendant"
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s party phase c.filed_on "text.closing_chars_per_side"
        "closing_argument_chars" argument.length
      let usage := setLimitUsage c.limit_usage "text.closing_chars_per_side" party phase argument.length
      pure <| updateCase s (appendDocket { c with limit_usage := usage } title argument)
  | "serve_rule11_safe_harbor_notice" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status = "closed" then
        throw "cannot serve rule 11 notice on closed case"
      let servedBy ← getValidatedActorPartyField a "served_by"
      let targetPartyRaw ← getString a.payload "target_party"
      let targetParty := normalizePartyToken targetPartyRaw
      if !(targetParty = "plaintiff" || targetParty = "defendant") then
        throw s!"invalid target_party: {targetPartyRaw}"
      if targetParty = servedBy then
        throw "rule 11 notice target must be the opposing party"
      let c1 := appendTrace (appendDocket c "Rule 11 Safe Harbor Notice" "served")
        "serve_rule11_safe_harbor_notice" "served" ["FRCP 11(c)(2)"]
      pure <| updateCase s c1
  | "withdraw_or_correct_filing" =>
      requireRole a ["plaintiff", "defendant"]
      let noticeIndex ← getNat a.payload "notice_index"
      let noticeCount := countDocketTitle c "Rule 11 Safe Harbor Notice"
      if noticeIndex >= noticeCount then
        throw "rule 11 notice index out of range"
      let _byParty ← getValidatedActorPartyField a "by_party"
      let noticeResolved ← getBoolD a.payload "notice_resolved" false
      if noticeResolved then
        throw "rule 11 notice already resolved"
      let c1 := appendTrace (appendDocket c "Withdrawal or Correction" "resolved")
        "withdraw_or_correct_filing" "resolved_safe_harbor" ["FRCP 11(c)(2)"]
      pure <| updateCase s c1
  | "file_rule11_motion" =>
      requireRole a ["plaintiff", "defendant"]
      let _movant ← getValidatedActorPartyField a "movant"
      let noticeIndex ← getNat a.payload "notice_index"
      let noticeCount := countDocketTitle c "Rule 11 Safe Harbor Notice"
      if noticeIndex >= noticeCount then
        throw "rule 11 notice index out of range"
      let noticeResolved ← getBoolD a.payload "notice_resolved" false
      if noticeResolved then
        throw "cannot file rule 11 motion after withdrawal or correction"
      let challengedFiling := match getStringOpt a.payload "notice_challenged_filing" with
        | .ok (some s) => s
        | _ => ""
      if isDiscoveryFilingText challengedFiling then
        throw "rule 11 does not apply to discovery matters"
      let noticeServedAtOpt ← getStringOpt a.payload "notice_served_at"
      let filedAtOpt ← getStringOpt a.payload "filed_at"
      match noticeServedAtOpt, filedAtOpt with
      | some noticeServedAt, some filedAt =>
          let elapsed ← elapsedDaysBetween noticeServedAt filedAt
          if elapsed < 21 then
            throw "rule 11 safe harbor period has not elapsed"
      | _, _ => pure ()
      let c1 := appendTrace (appendDocket c "Rule 11 Motion" "filed")
        "file_rule11_motion" "filed" ["FRCP 11(c)(2)"]
      pure <| updateCase s c1
  | "decide_rule11_motion" =>
      requireRole a ["judge"]
      let motionIndex ← getNat a.payload "motion_index"
      let motionCount := countDocketTitle c "Rule 11 Motion"
      if motionIndex >= motionCount then
        throw "rule 11 motion index out of range"
      let disposition := match getStringOpt a.payload "rule11_motion_disposition" with
        | .ok (some s) => s
        | _ => "pending"
      if disposition != "pending" then
        throw "rule 11 motion already decided"
      let granted ← getBoolD a.payload "granted" false
      let sanctionTypeOpt ← getStringOpt a.payload "sanction_type"
      let sanctionAmountOpt ← getFloatOpt a.payload "sanction_amount"
      let sanctionDetail := match getStringOpt a.payload "sanction_detail" with
        | .ok (some s) => s
        | _ => ""
      let reasoning ← getString a.payload "reasoning"
      if reasoning.trimAscii.toString = "" then
        throw "rule 11 order requires reasoning"
      match sanctionTypeOpt with
      | some t =>
          if !(t = "none" || t = "admonition" || t = "non_monetary_directive" || t = "monetary_penalty" || t = "fee_shift") then
            throw "invalid sanction_type for rule 11 order"
      | none => pure ()
      if granted then
        let sanctionType ← match sanctionTypeOpt with
          | some t => pure t
          | none => throw "granted rule 11 motion requires sanction type"
        if sanctionType = "monetary_penalty" || sanctionType = "fee_shift" then
          match sanctionAmountOpt with
          | some n =>
              if n <= 0 then
                throw "monetary sanctions require positive sanction amount"
          | none => throw "monetary sanctions require positive sanction amount"
        if sanctionType = "admonition" || sanctionType = "non_monetary_directive" then
          match sanctionAmountOpt with
          | some n =>
              if n != 0 then
                throw "non-monetary sanctions cannot include monetary amount"
          | _ => pure ()
      else
        match sanctionTypeOpt with
        | some t =>
            if t.trimAscii.toString != "" then
              throw "denied rule 11 motion cannot include sanctions"
        | _ => pure ()
        match sanctionAmountOpt with
        | some n =>
            if n != 0 then
              throw "denied rule 11 motion cannot include sanctions"
        | _ => pure ()
        if sanctionDetail.trimAscii.toString != "" then
          throw "denied rule 11 motion cannot include sanctions"
      let disposition := if granted then "granted" else "denied"
      let sanctionType := match sanctionTypeOpt with
        | some t => t
        | none => "none"
      let sanctionAmount := match sanctionAmountOpt with
        | some n => toString n
        | none => "0"
      let orderDesc := s!"motion_index={motionIndex} disposition={disposition} sanction_type={sanctionType} sanction_amount={sanctionAmount} sanction_detail: {sanctionDetail} reasoning: {reasoning}"
      let c1 := appendTrace (appendDocket c "Rule 11 Order" orderDesc)
        "decide_rule11_motion" disposition ["FRCP 11(c)(1)", "FRCP 11(c)(4)"]
      pure <| updateCase s c1
  | "file_rule37_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if !(c.status = "filed" || c.status = "pretrial") then
        throw "rule 37 motion is unavailable after pretrial"
      let movant ← getValidatedActorPartyField a "movant"
      let targetPartyRaw ← getString a.payload "target_party"
      let targetParty := normalizePartyToken targetPartyRaw
      if !(targetParty = "plaintiff" || targetParty = "defendant") then
        throw s!"invalid target_party: {targetPartyRaw}"
      if targetParty = movant then
        throw "rule 37 motion target must be the opposing party"
      let discoveryTypeRaw ← getString a.payload "discovery_type"
      let discoveryType := discoveryTypeRaw.trimAscii.toString.toLower
      if !(discoveryType = "interrogatories" || discoveryType = "rfp" || discoveryType = "rfa" || discoveryType = "initial_disclosures") then
        throw "discovery_type must be interrogatories, rfp, rfa, or initial_disclosures"
      let setIndex ← getNat a.payload "set_index"
      let availableSetCount := match getNatOpt a.payload "discovery_set_count" with
        | .ok (some n) => n
        | _ => 0
      if discoveryType != "initial_disclosures" && setIndex >= availableSetCount then
        if discoveryType = "interrogatories" then
          throw "interrogatory set index out of range"
        else if discoveryType = "rfp" then
          throw "request-for-production set index out of range"
        else
          throw "request-for-admission set index out of range"
      let c1 := appendTrace (appendDocket c "Rule 37 Motion" "filed")
        "file_rule37_motion" discoveryType ["FRCP 37(a)"]
      pure <| updateCase s c1
  | "decide_rule37_motion" =>
      requireRole a ["judge"]
      let motionIndex ← getNat a.payload "motion_index"
      let motionCount := countDocketTitle c "Rule 37 Motion"
      if motionIndex >= motionCount then
        throw "rule 37 motion index out of range"
      let disposition := match getStringOpt a.payload "rule37_motion_disposition" with
        | .ok (some s) => s
        | _ => "pending"
      if disposition != "pending" then
        throw "rule 37 motion already decided"
      let granted ← getBoolD a.payload "granted" false
      let sanctionType := match getStringOpt a.payload "sanction_type" with
        | .ok (some s) => s
        | _ => "none"
      let reasoning ← getString a.payload "reasoning"
      if reasoning.trimAscii.toString = "" then
        throw "rule 37 order requires reasoning"
      let orderText := match getStringOpt a.payload "order_text" with
        | .ok (some s) => s
        | _ => ""
      if !(sanctionType = "none" || sanctionType = "fees") then
        throw "invalid sanction_type for rule 37 order"
      let sanctionAmountOpt ← getFloatOpt a.payload "sanction_amount"
      if !granted && sanctionType != "none" then
        throw "denied rule 37 motion cannot include sanctions"
      match sanctionAmountOpt with
      | some n =>
          if n < 0 then
            throw "sanction_amount must be >= 0"
      | _ => pure ()
      if sanctionType = "none" then
        match sanctionAmountOpt with
        | some n =>
            if n != 0 then
              throw "no-sanction disposition cannot include sanction amount"
        | _ => pure ()
      if sanctionType = "fees" then
        match sanctionAmountOpt with
        | some n =>
            if n <= 0 then
              throw "fees sanction requires positive sanction_amount"
        | none => throw "fees sanction requires positive sanction_amount"
      let disposition := if granted then "granted" else "denied"
      let sanctionAmount := match sanctionAmountOpt with
        | some n => toString n
        | none => "0"
      let orderTextSuffix :=
        if orderText.trimAscii.toString = "" then
          ""
        else
          s!" order_text: {orderText}"
      let orderDesc := s!"motion_index={motionIndex} disposition={disposition} sanction_type={sanctionType} sanction_amount={sanctionAmount}{orderTextSuffix} reasoning: {reasoning}"
      let c1 := appendTrace (appendDocket c "Rule 37 Order" orderDesc)
        "decide_rule37_motion" disposition ["FRCP 37(a)(5)", "FRCP 37(b)(2)"]
      pure <| updateCase s c1
  | "file_rule12_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if !(c.status = "filed" || c.status = "pretrial") then
        throw "rule 12 motion requires filed or pretrial status"
      if hasDecisionTraceAction c "file_answer" then
        throw "rule 12 motion unavailable after answer is filed"
      if hasDocketTitle c "Rule 12 Order" then
        throw "rule 12 motion already decided"
      let movant ← getValidatedActorPartyField a "movant"
      let ground ← getString a.payload "ground"
      if !validRule12Ground s ground then
        throw s!"invalid rule 12 ground: {ground}"
      let summary ← getString a.payload "summary"
      let _ ← enforceMeasuredLimit s movant (normalizePhaseForLimit c.phase) c.filed_on "text.rule12_summary_chars"
        "rule12_summary_chars" summary.length
      let attempted := countDispositiveMotionsByParty c movant + 1
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s movant phase c.filed_on "motions.dispositive_motions_per_side_pretrial"
        "dispositive_motion_count" attempted
      let usage1 := setLimitUsage c.limit_usage "text.rule12_summary_chars" movant phase summary.length
      let usage := setLimitUsage usage1 "motions.dispositive_motions_per_side_pretrial" movant phase attempted
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Rule 12 Motion" s!"{movant}: ground={ground} summary={summary}")
        "file_rule12_motion" "filed" ["FRCP 12(b)"]
      pure <| updateCase s c1
  | "oppose_rule12_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if !hasDocketTitle c "Rule 12 Motion" then
        throw "cannot oppose rule 12 motion before a motion is filed"
      if hasDocketTitle c "Rule 12 Order" then
        throw "cannot oppose rule 12 motion after order is entered"
      if hasDocketTitle c "Rule 12 Opposition" then
        throw "rule 12 opposition already filed"
      let party ← getValidatedActorPartyField a "party"
      let summary ← getString a.payload "summary"
      let c1 := appendTrace (appendDocket c "Rule 12 Opposition" s!"{party}: {summary}")
        "oppose_rule12_motion" "filed" ["FRCP 12"]
      pure <| updateCase s c1
  | "reply_rule12_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if !hasDocketTitle c "Rule 12 Motion" then
        throw "cannot reply on rule 12 motion before a motion is filed"
      if !hasDocketTitle c "Rule 12 Opposition" then
        throw "cannot reply on rule 12 motion before opposition is filed"
      if hasDocketTitle c "Rule 12 Order" then
        throw "cannot reply on rule 12 motion after order is entered"
      if hasDocketTitle c "Rule 12 Reply" then
        throw "rule 12 reply already filed"
      let party ← getValidatedActorPartyField a "party"
      let summary ← getString a.payload "summary"
      let c1 := appendTrace (appendDocket c "Rule 12 Reply" s!"{party}: {summary}")
        "reply_rule12_motion" "filed" ["FRCP 12"]
      pure <| updateCase s c1
  | "decide_rule12_motion" =>
      requireRole a ["judge"]
      if !hasDocketTitle c "Rule 12 Motion" then
        throw "cannot decide rule 12 motion before filing"
      if hasDocketTitle c "Rule 12 Order" then
        throw "rule 12 motion already decided"
      let motionGround := latestRule12MotionGround? c |>.getD ""
      if motionGround = "" then
        throw "rule 12 motion ground missing from filed motion"
      let ground ← getString a.payload "ground"
      if !validRule12Ground s ground then
        throw s!"invalid rule 12 ground: {ground}"
      if ground != motionGround then
        throw s!"rule 12 ruling ground must match filed motion ground: filed={motionGround}, ruling={ground}"
      let disposition ← getString a.payload "disposition"
      if !(disposition = "granted" || disposition = "denied") then
        throw s!"invalid rule 12 disposition: {disposition}"
      let withPrejudice ← getBoolD a.payload "with_prejudice" false
      let leaveToAmend ← getBoolD a.payload "leave_to_amend" false
      let jurisdictionBasisRejectedOpt ← getStringOpt a.payload "jurisdiction_basis_rejected"
      let injuryMissing := (← getBoolOpt a.payload "injury_missing").getD false
      let traceabilityMissing := (← getBoolOpt a.payload "traceability_missing").getD false
      let redressabilityMissing := (← getBoolOpt a.payload "redressability_missing").getD false
      let missingElements ←
        match getStringList a.payload "missing_elements" with
        | .ok xs => pure xs
        | .error _ => pure []
      let reasoning ← getString a.payload "reasoning"
      if reasoning.trimAscii.toString = "" then
        throw "rule 12 order requires reasoning"
      if withPrejudice && leaveToAmend then
        throw "rule 12 order cannot be both with_prejudice and leave_to_amend"
      if ground != "failure_to_state_a_claim" && withPrejudice then
        throw "only failure_to_state_a_claim may be dismissed with prejudice in this initial model"
      if disposition = "granted" && ground = "lack_subject_matter_jurisdiction" then
        if (match jurisdictionBasisRejectedOpt with | some v => trimString v | none => "") = "" then
          throw "rule 12 jurisdiction dismissal requires jurisdiction_basis_rejected"
      if disposition = "granted" && ground = "no_standing" then
        if !injuryMissing && !traceabilityMissing && !redressabilityMissing then
          throw "standing dismissal requires at least one missing standing component"
      if disposition = "granted" && ground = "failure_to_state_a_claim" then
        if missingElements.isEmpty then
          throw "failure_to_state_a_claim dismissal requires missing_elements"
      let cResolved :=
        if disposition = "granted" then
          { c with status := "closed" }
        else
          c
      let c1 := appendTrace (appendDocket cResolved "Rule 12 Order"
        s!"ground={ground} disposition={disposition} with_prejudice={if withPrejudice then "true" else "false"} leave_to_amend={if leaveToAmend then "true" else "false"} reasoning: {reasoning}")
        "decide_rule12_motion" disposition ["FRCP 12"]
      pure <| updateCase s c1
  | "dismiss_for_lack_of_subject_matter_jurisdiction" =>
      requireRole a ["judge"]
      if !courtUsesJurisdictionScreen s then
        throw "current court does not use subject-matter jurisdiction dismissal"
      if c.status = "judgment_entered" then
        throw "cannot dismiss for lack of subject-matter jurisdiction after final judgment in this initial model"
      if c.status = "closed" then
        throw "case already closed"
      if hasDecisionTraceAction c "dismiss_for_lack_of_subject_matter_jurisdiction" then
        throw "subject-matter-jurisdiction dismissal already entered"
      let jurisdictionBasisRejected ← getString a.payload "jurisdiction_basis_rejected"
      if trimString jurisdictionBasisRejected = "" then
        throw "subject-matter-jurisdiction dismissal requires jurisdiction_basis_rejected"
      let reasoning ← getString a.payload "reasoning"
      if trimString reasoning = "" then
        throw "subject-matter-jurisdiction dismissal requires reasoning"
      let leaveToAmend := (← getBoolOpt a.payload "leave_to_amend").getD false
      let c1 := appendTrace (appendDocket { c with status := "closed" } "Subject-Matter Jurisdiction Dismissal"
        s!"jurisdiction_basis_rejected={jurisdictionBasisRejected} leave_to_amend={if leaveToAmend then "true" else "false"} reasoning: {reasoning}")
        "dismiss_for_lack_of_subject_matter_jurisdiction" "dismissed" ["FRCP 12(h)(3)"]
      pure <| updateCase s c1
  | "file_rule56_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "pretrial" then
        throw "rule 56 motion requires pretrial status"
      if hasDocketTitle c "Rule 56 Order" then
        throw "rule 56 motion already decided"
      let movant ← getValidatedActorPartyField a "movant"
      let scope ← getString a.payload "scope"
      let statement ← getString a.payload "statement_of_undisputed_facts"
      let combined := if scope = "" then statement else s!"{scope}\n{statement}"
      let _ ← enforceMeasuredLimit s movant (normalizePhaseForLimit c.phase) c.filed_on "text.rule56_summary_chars"
        "rule56_summary_chars" combined.length
      let attempted := countDispositiveMotionsByParty c movant + 1
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s movant phase c.filed_on "motions.dispositive_motions_per_side_pretrial"
        "dispositive_motion_count" attempted
      let usage1 := setLimitUsage c.limit_usage "text.rule56_summary_chars" movant phase combined.length
      let usage := setLimitUsage usage1 "motions.dispositive_motions_per_side_pretrial" movant phase attempted
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Rule 56 Motion" s!"{movant}: filed")
        "file_rule56_motion" "filed" ["FRCP 56(a)"]
      pure <| updateCase s c1
  | "oppose_rule56_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if !hasDocketTitle c "Rule 56 Motion" then
        throw "cannot oppose rule 56 motion before a motion is filed"
      if hasDocketTitle c "Rule 56 Order" then
        throw "cannot oppose rule 56 motion after order is entered"
      if hasDocketTitle c "Rule 56 Opposition" then
        throw "rule 56 opposition already filed"
      let party ← getValidatedActorPartyField a "party"
      let summary ← getString a.payload "summary"
      let c1 := appendTrace (appendDocket c "Rule 56 Opposition" s!"{party}: {summary}")
        "oppose_rule56_motion" "filed" ["FRCP 56(c)"]
      pure <| updateCase s c1
  | "reply_rule56_motion" =>
      requireRole a ["plaintiff", "defendant"]
      if !hasDocketTitle c "Rule 56 Motion" then
        throw "cannot reply on rule 56 motion before a motion is filed"
      if !hasDocketTitle c "Rule 56 Opposition" then
        throw "cannot reply on rule 56 motion before opposition is filed"
      if hasDocketTitle c "Rule 56 Order" then
        throw "cannot reply on rule 56 motion after order is entered"
      if hasDocketTitle c "Rule 56 Reply" then
        throw "rule 56 reply already filed"
      let party ← getValidatedActorPartyField a "party"
      let summary ← getString a.payload "summary"
      let phase := normalizePhaseForLimit c.phase
      let _ ← enforceMeasuredLimit s party phase c.filed_on "text.rule56_reply_chars"
        "rule56_reply_chars" summary.length
      let usage := setLimitUsage c.limit_usage "text.rule56_reply_chars" party phase summary.length
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Rule 56 Reply" s!"{party}: {summary}")
        "reply_rule56_motion" "filed" ["FRCP 56(c)"]
      pure <| updateCase s c1
  | "decide_rule56_motion" =>
      requireRole a ["judge"]
      if !hasDocketTitle c "Rule 56 Motion" then
        throw "cannot decide rule 56 motion before filing"
      if hasDocketTitle c "Rule 56 Order" then
        throw "rule 56 motion already decided"
      let disposition ← getString a.payload "disposition"
      if !(disposition = "granted" || disposition = "denied" || disposition = "partial") then
        throw s!"invalid rule 56 disposition: {disposition}"
      let reasoning ← getString a.payload "reasoning"
      if reasoning.trimAscii.toString = "" then
        throw "rule 56 order requires reasoning"
      let c1 := appendTrace (appendDocket c "Rule 56 Order" s!"disposition={disposition} reasoning: {reasoning}")
        "decide_rule56_motion" disposition ["FRCP 56"]
      pure <| updateCase s c1
  | "serve_interrogatories" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "pretrial" then
        throw "interrogatories require pretrial status"
      let servedBy ← getValidatedActorPartyField a "served_by"
      let questionsLen ← match (← getArrayLenOpt a.payload "questions") with
        | some n => pure n
        | none => throw "payload field questions must be array"
      if questionsLen = 0 then
        throw "interrogatories require at least one question"
      let phase := normalizePhaseForLimit c.phase
      let setAttempted := countDocketTitleByPartyPrefix c "Interrogatories Served" servedBy + 1
      let _ ← enforceMeasuredLimit s servedBy phase c.filed_on "discovery.interrogatory_sets_per_side"
        "interrogatory_set_count" setAttempted
      let _ ← enforceMeasuredLimit s servedBy phase c.filed_on "discovery.interrogatories_per_set"
        "interrogatory_count" questionsLen
      let usage1 := setLimitUsage c.limit_usage "discovery.interrogatory_sets_per_side" servedBy phase setAttempted
      let usage2 := setLimitUsage usage1 "discovery.interrogatories_per_set" servedBy phase questionsLen
      let c1 := appendTrace (appendDocket { c with limit_usage := usage2 } "Interrogatories Served" s!"{servedBy}: served")
        "serve_interrogatories" "served" ["FRCP 33"]
      pure <| updateCase s c1
  | "respond_interrogatories" =>
      requireRole a ["plaintiff", "defendant"]
      let setIndex ← getNat a.payload "set_index"
      let servedCount := countDocketTitle c "Interrogatories Served"
      if setIndex >= servedCount then
        throw "cannot respond to interrogatories before service"
      let responseCount := countDocketTitle c "Interrogatory Responses"
      if setIndex < responseCount then
        throw "interrogatory responses already filed"
      let responding ← getValidatedActorPartyField a "responding_party"
      let servedOnDateOpt ← getStringOpt a.payload "served_on_date"
      let respondedAtOpt ← getStringOpt a.payload "responded_at"
      let usage := c.limit_usage
      let usage ← match servedOnDateOpt, respondedAtOpt with
        | some servedOn, some respondedAt =>
            let servedOrd ← ordinalDay servedOn
            let respondedOrd ← ordinalDay respondedAt
            let elapsed := if respondedOrd >= servedOrd then respondedOrd - servedOrd else 0
            let phase := normalizePhaseForLimit c.phase
            let _ ← enforceMeasuredLimit s responding phase c.filed_on "discovery.response_deadline_days"
              "discovery_response_days_elapsed" elapsed
            pure <| setLimitUsage usage "discovery.response_deadline_days" responding phase elapsed
        | _, _ => pure usage
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Interrogatory Responses" s!"{responding}: served")
        "respond_interrogatories" "served" ["FRCP 33(b)"]
      pure <| updateCase s c1
  | "respond_interrogatory_item" =>
      requireRole a ["plaintiff", "defendant"]
      let setIndex ← getNat a.payload "set_index"
      let servedCount := countDocketTitle c "Interrogatories Served"
      if setIndex >= servedCount then
        throw "cannot draft interrogatory response before service"
      if hasDocketTitle c "Interrogatory Responses" then
        throw "interrogatory responses already finalized"
      let respondingRaw ←
        match (← getStringOpt a.payload "responding_party") with
        | some v => pure v
        | none => pure a.actor_role
      let responding := normalizePartyToken respondingRaw
      if !(responding = "plaintiff" || responding = "defendant") then
        throw s!"invalid responding_party: {respondingRaw}"
      let itemIndex ←
        match (← getNatOpt a.payload "item_index") with
        | some n => pure n
        | none =>
            match (← getNatOpt a.payload "question_index") with
            | some n => pure n
            | none => throw "payload must include item_index or question_index"
      let _answerOpt ← getStringOpt a.payload "answer"
      let _objectionOpt ← getStringOpt a.payload "objection"
      let detail := s!"{responding}: set_index={setIndex} item_index={itemIndex}"
      let c1 := appendTrace (appendDocket c "Interrogatory Response Draft" detail)
        "respond_interrogatory_item" "drafted" ["FRCP 33(b)"]
      pure <| updateCase s c1
  | "finalize_interrogatory_responses" =>
      requireRole a ["plaintiff", "defendant"]
      let setIndex ← getNat a.payload "set_index"
      let servedCount := countDocketTitle c "Interrogatories Served"
      if setIndex >= servedCount then
        throw "cannot finalize interrogatory responses before service"
      let responseCount := countDocketTitle c "Interrogatory Responses"
      if setIndex < responseCount then
        throw "interrogatory responses already filed"
      let respondingRaw ←
        match (← getStringOpt a.payload "responding_party") with
        | some v => pure v
        | none => pure a.actor_role
      let responding := normalizePartyToken respondingRaw
      if !(responding = "plaintiff" || responding = "defendant") then
        throw s!"invalid responding_party: {respondingRaw}"
      let hasDraft :=
        c.docket.any (fun entry =>
          entry.title = "Interrogatory Response Draft" &&
          entry.description.contains s!"{responding}:" &&
          entry.description.contains s!"set_index={setIndex}")
      if !hasDraft then
        throw "no interrogatory response draft items available to finalize"
      let servedOnDateOpt ← getStringOpt a.payload "served_on_date"
      let respondedAtOpt ← getStringOpt a.payload "responded_at"
      let usage := c.limit_usage
      let usage ← match servedOnDateOpt, respondedAtOpt with
        | some servedOn, some respondedAt =>
            let servedOrd ← ordinalDay servedOn
            let respondedOrd ← ordinalDay respondedAt
            let elapsed := if respondedOrd >= servedOrd then respondedOrd - servedOrd else 0
            let phase := normalizePhaseForLimit c.phase
            let _ ← enforceMeasuredLimit s responding phase c.filed_on "discovery.response_deadline_days"
              "discovery_response_days_elapsed" elapsed
            pure <| setLimitUsage usage "discovery.response_deadline_days" responding phase elapsed
        | _, _ => pure usage
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Interrogatory Responses" s!"{responding}: served")
        "finalize_interrogatory_responses" "served" ["FRCP 33(b)"]
      pure <| updateCase s c1
  | "serve_request_for_production" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "pretrial" then
        throw "requests for production require pretrial status"
      let servedBy ← getValidatedActorPartyField a "served_by"
      let requestLen ← match (← getArrayLenOpt a.payload "requests") with
        | some n => pure n
        | none => throw "payload field requests must be array"
      if requestLen = 0 then
        throw "request for production set must include at least one request"
      let phase := normalizePhaseForLimit c.phase
      let setAttempted := countDocketTitleByPartyPrefix c "Requests for Production Served" servedBy + 1
      let _ ← enforceMeasuredLimit s servedBy phase c.filed_on "discovery.rfp_sets_per_side"
        "rfp_set_count" setAttempted
      let _ ← enforceMeasuredLimit s servedBy phase c.filed_on "discovery.rfp_requests_per_set"
        "rfp_request_count" requestLen
      let usage1 := setLimitUsage c.limit_usage "discovery.rfp_sets_per_side" servedBy phase setAttempted
      let usage2 := setLimitUsage usage1 "discovery.rfp_requests_per_set" servedBy phase requestLen
      let c1 := appendTrace (appendDocket { c with limit_usage := usage2 } "Requests for Production Served" s!"{servedBy}: served")
        "serve_request_for_production" "served" ["FRCP 34"]
      pure <| updateCase s c1
  | "respond_request_for_production" =>
      requireRole a ["plaintiff", "defendant"]
      let setIndex ← getNat a.payload "set_index"
      let servedCount := countDocketTitle c "Requests for Production Served"
      if setIndex >= servedCount then
        throw "cannot respond to requests for production before service"
      let responseCount := countDocketTitle c "Responses to Requests for Production"
      if setIndex < responseCount then
        throw "responses to requests for production already filed"
      let responding ← getValidatedActorPartyField a "responding_party"
      let servedOnDateOpt ← getStringOpt a.payload "served_on_date"
      let respondedAtOpt ← getStringOpt a.payload "responded_at"
      let usage := c.limit_usage
      let usage ← match servedOnDateOpt, respondedAtOpt with
        | some servedOn, some respondedAt =>
            let servedOrd ← ordinalDay servedOn
            let respondedOrd ← ordinalDay respondedAt
            let elapsed := if respondedOrd >= servedOrd then respondedOrd - servedOrd else 0
            let phase := normalizePhaseForLimit c.phase
            let _ ← enforceMeasuredLimit s responding phase c.filed_on "discovery.response_deadline_days"
              "discovery_response_days_elapsed" elapsed
            pure <| setLimitUsage usage "discovery.response_deadline_days" responding phase elapsed
        | _, _ => pure usage
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Responses to Requests for Production" s!"{responding}: served")
        "respond_request_for_production" "served" ["FRCP 34"]
      pure <| updateCase s c1
  | "serve_requests_for_admission" =>
      requireRole a ["plaintiff", "defendant"]
      if c.status != "pretrial" then
        throw "requests for admission require pretrial status"
      let servedBy ← getValidatedActorPartyField a "served_by"
      let requestLen ← match (← getArrayLenOpt a.payload "requests") with
        | some n => pure n
        | none => throw "payload field requests must be array"
      if requestLen = 0 then
        throw "requests for admission set must include at least one request"
      let phase := normalizePhaseForLimit c.phase
      let setAttempted := countDocketTitleByPartyPrefix c "Requests for Admission Served" servedBy + 1
      let _ ← enforceMeasuredLimit s servedBy phase c.filed_on "discovery.rfa_sets_per_side"
        "rfa_set_count" setAttempted
      let _ ← enforceMeasuredLimit s servedBy phase c.filed_on "discovery.rfa_requests_per_set"
        "rfa_request_count" requestLen
      let usage1 := setLimitUsage c.limit_usage "discovery.rfa_sets_per_side" servedBy phase setAttempted
      let usage2 := setLimitUsage usage1 "discovery.rfa_requests_per_set" servedBy phase requestLen
      let c1 := appendTrace (appendDocket { c with limit_usage := usage2 } "Requests for Admission Served" s!"{servedBy}: served")
        "serve_requests_for_admission" "served" ["FRCP 36"]
      pure <| updateCase s c1
  | "respond_requests_for_admission" =>
      requireRole a ["plaintiff", "defendant"]
      let setIndex ← getNat a.payload "set_index"
      let servedCount := countDocketTitle c "Requests for Admission Served"
      if setIndex >= servedCount then
        throw "cannot respond to requests for admission before service"
      let responseCount := countDocketTitle c "Responses to Requests for Admission"
      if setIndex < responseCount then
        throw "responses to requests for admission already filed"
      let responding ← getValidatedActorPartyField a "responding_party"
      let servedOnDateOpt ← getStringOpt a.payload "served_on_date"
      let respondedAtOpt ← getStringOpt a.payload "responded_at"
      let usage := c.limit_usage
      let usage ← match servedOnDateOpt, respondedAtOpt with
        | some servedOn, some respondedAt =>
            let servedOrd ← ordinalDay servedOn
            let respondedOrd ← ordinalDay respondedAt
            let elapsed := if respondedOrd >= servedOrd then respondedOrd - servedOrd else 0
            let phase := normalizePhaseForLimit c.phase
            let _ ← enforceMeasuredLimit s responding phase c.filed_on "discovery.response_deadline_days"
              "discovery_response_days_elapsed" elapsed
            pure <| setLimitUsage usage "discovery.response_deadline_days" responding phase elapsed
        | _, _ => pure usage
      let c1 := appendTrace (appendDocket { c with limit_usage := usage } "Responses to Requests for Admission" s!"{responding}: served")
        "respond_requests_for_admission" "served" ["FRCP 36"]
      pure <| updateCase s c1
  | "enter_partial_judgment" =>
      requireRole a ["judge"]
      if c.status != "pretrial" then
        throw "partial judgment requires pretrial status"
      let issuesResolved ← getStringList a.payload "issues_resolved"
      if issuesResolved.length = 0 then
        throw "issues_resolved must contain at least one issue"
      let amount ← getFloatD a.payload "amount" 0.0
      if amount < 0.0 then
        throw "amount must be >= 0"
      let basis := match getStringOpt a.payload "basis" with
        | .ok (some value) => value
        | _ => ""
      let cJudged := { c with monetary_judgment := amount }
      let c1 := appendTrace (appendDocket cJudged "Partial judgment entered"
        s!"issues_resolved={issuesResolved.length} amount={amount} basis={basis}")
        "enter_partial_judgment" "entered" ["FRCP 54(b)", "FRCP 58"]
      pure <| updateCase s c1
  | "make_rule68_offer" =>
      requireRole a ["defendant"]
      let offereeRaw ← getString a.payload "offeree"
      let offeree := normalizePartyToken offereeRaw
      if !(offeree = "plaintiff" || offeree = "defendant") then
        throw s!"invalid offeree: {offereeRaw}"
      let amount ← getFloatD a.payload "amount" 0.0
      if amount < 0.0 then
        throw "amount must be >= 0"
      let offerId :=
        match getStringOpt a.payload "offer_id" with
        | .ok (some value) =>
            let trimmed := trimString value
            if trimmed = "" then s!"offer-{c.rule68_offers.length + 1}" else trimmed
        | _ => s!"offer-{c.rule68_offers.length + 1}"
      if c.rule68_offers.any (fun offer => offer.offer_id = offerId) then
        throw s!"rule68 offer_id already exists: {offerId}"
      let servedAt := match getStringOpt a.payload "served_at" with
        | .ok (some value) =>
            let trimmed := trimString value
            if trimmed = "" then c.filed_on else trimmed
        | _ => c.filed_on
      let terms := match getStringOpt a.payload "terms" with
        | .ok (some value) => value
        | _ => ""
      let claimScope := match getStringOpt a.payload "claim_scope" with
        | .ok (some value) => value
        | _ => ""
      let offer : Rule68Offer :=
        { offer_id := offerId
        , offeror := "defendant"
        , offeree := offeree
        , amount := amount
        , status := "pending"
        , terms := terms
        , claim_scope := claimScope
        , served_at := servedAt
        , expires_at := none
        , accepted_at := none
        , expired_at := none
        }
      let c1 := appendTrace (appendDocket { c with rule68_offers := c.rule68_offers.concat offer } "Rule 68 Offer"
        s!"offer_id={offerId} offeror=defendant offeree={offeree} amount={amount}")
        "make_rule68_offer" "pending" ["FRCP 68(a)"]
      pure <| updateCase s c1
  | "accept_rule68_offer" =>
      requireRole a ["plaintiff", "defendant"]
      let offerIndexOpt ← getNatOpt a.payload "offer_index"
      let offerIdOpt ← getStringOpt a.payload "offer_id"
      let idx ←
        match offerIndexOpt with
        | some n => pure n
        | none =>
            match offerIdOpt with
            | some offerId =>
                match findRule68OfferIndexById c.rule68_offers offerId with
                | some n => pure n
                | none => throw s!"unknown offer_id: {offerId}"
            | none => throw "accept_rule68_offer requires offer_index or offer_id"
      let offer ← match getRule68OfferAt c.rule68_offers idx with
        | some o => pure o
        | none => throw "rule68 offer index out of range"
      if offer.status != "pending" then
        throw "rule68 offer is not pending"
      let actor := normalizePartyToken a.actor_role
      if actor != offer.offeree then
        throw "only the offeree may accept rule68 offer"
      let acceptedAt := match getStringOpt a.payload "accepted_at" with
        | .ok (some value) =>
            let trimmed := trimString value
            if trimmed = "" then c.filed_on else trimmed
        | _ => c.filed_on
      let updatedOffer :=
        { offer with
            status := "accepted"
            accepted_at := some acceptedAt }
      let updatedOffers := setRule68OfferAt c.rule68_offers idx updatedOffer
      let cAccepted := { c with rule68_offers := updatedOffers, status := "judgment_entered", monetary_judgment := offer.amount }
      let c1 := appendTrace (appendDocket cAccepted "Rule 68 Offer Accepted"
        s!"offer_id={offer.offer_id} accepted_by={actor} amount={offer.amount}")
        "accept_rule68_offer" "accepted" ["FRCP 68(a)"]
      pure <| updateCase s c1
  | "expire_rule68_offers" =>
      requireRole a ["clerk", "judge"]
      let asOf := match getStringOpt a.payload "as_of" with
        | .ok (some value) =>
            let trimmed := trimString value
            if trimmed = "" then c.filed_on else trimmed
        | _ => c.filed_on
      let (expiredCount, updatedRev) := c.rule68_offers.foldl
        (fun acc offer =>
          let expiredCount := acc.fst
          let rev := acc.snd
          if offer.status = "pending" then
            (expiredCount + 1, { offer with status := "expired", expired_at := some asOf } :: rev)
          else
            (expiredCount, offer :: rev))
        (0, [])
      let updatedOffers := updatedRev.reverse
      let c1 := appendTrace (appendDocket { c with rule68_offers := updatedOffers } "Rule 68 Offers Expired"
        s!"as_of={asOf} expired_count={expiredCount}")
        "expire_rule68_offers" "expired" ["FRCP 68(b)"]
      pure <| updateCase s c1
  | "evaluate_rule68_cost_shift" =>
      requireRole a ["judge"]
      let offerIndexOpt ← getNatOpt a.payload "offer_index"
      let offerIdOpt ← getStringOpt a.payload "offer_id"
      let idx ←
        match offerIndexOpt with
        | some n => pure n
        | none =>
            match offerIdOpt with
            | some offerId =>
                match findRule68OfferIndexById c.rule68_offers offerId with
                | some n => pure n
                | none => throw s!"unknown offer_id: {offerId}"
            | none => throw "evaluate_rule68_cost_shift requires offer_index or offer_id"
      let offer ← match getRule68OfferAt c.rule68_offers idx with
        | some o => pure o
        | none => throw "rule68 offer index out of range"
      let finalAmount ← getFloatD a.payload "amount" 0.0
      if finalAmount < 0.0 then
        throw "amount must be >= 0"
      let awardedToRaw := match getStringOpt a.payload "awarded_to" with
        | .ok (some value) => value
        | _ => "plaintiff"
      let awardedTo := normalizePartyToken awardedToRaw
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some value) => value
        | _ => ""
      let applies := offer.status = "expired" && finalAmount <= offer.amount
      let c1 := appendTrace (appendDocket c "Rule 68 Cost Shift Evaluation"
        s!"offer_id={offer.offer_id} offer_status={offer.status} awarded_to={awardedTo} final_amount={finalAmount} offer_amount={offer.amount} applies={applies} reason={reason}")
        "evaluate_rule68_cost_shift" (if applies then "applies" else "does_not_apply") ["FRCP 68(d)"]
      pure <| updateCase s c1
  | "file_rule59_motion" =>
      requireRole a ["plaintiff", "defendant"]
      let judgmentDateOpt ← getStringOpt a.payload "last_judgment_date"
      let judgmentDate ← match judgmentDateOpt with
        | some d => pure d
        | none => throw "rule 59 motion requires entered judgment"
      let filedAt ← getString a.payload "filed_at"
      validateRule59Timing judgmentDate filedAt
      let c1 := appendTrace (appendDocket c "Rule 59 Motion" "filed")
        "file_rule59_motion" "filed" ["FRCP 59(b)", "FRCP 59(e)"]
      pure <| updateCase s c1
  | "resolve_rule59_motion" =>
      requireRole a ["judge"]
      let motionIndex ← getNat a.payload "motion_index"
      let motionCount := countDocketTitle c "Rule 59 Motion"
      if motionIndex >= motionCount then
        throw "rule 59 motion index out of range"
      let orderCount := countDocketTitle c "Rule 59 Order"
      if motionIndex < orderCount then
        throw "rule 59 motion already resolved"
      if motionIndex > orderCount then
        throw "rule 59 motions must be resolved in order"
      let granted ← getBoolD a.payload "granted" false
      let orderText := match getStringOpt a.payload "order_text" with
        | .ok (some s) => s
        | _ => ""
      let disposition := if granted then "granted" else "denied"
      let c1 := appendTrace (appendDocket c "Rule 59 Order"
        s!"motion_index={motionIndex} disposition={disposition} order_text={orderText}")
        "resolve_rule59_motion" disposition ["FRCP 59"]
      pure <| updateCase s c1
  | "file_rule60_motion" =>
      requireRole a ["plaintiff", "defendant"]
      let judgmentDateOpt ← getStringOpt a.payload "last_judgment_date"
      let judgmentDate ← match judgmentDateOpt with
        | some d => pure d
        | none => throw "rule 60 motion requires entered judgment"
      let ground ← getString a.payload "ground"
      let filedAt ← getString a.payload "filed_at"
      validateRule60Timing judgmentDate filedAt ground
      let c1 := appendTrace (appendDocket c "Rule 60 Motion" "filed")
        "file_rule60_motion" "filed" ["FRCP 60(b)"]
      pure <| updateCase s c1
  | "enter_default" =>
      requireRole a ["clerk", "judge"]
      let againstPartyRaw ← getString a.payload "against_party"
      let againstParty := normalizePartyToken againstPartyRaw
      if !(againstParty = "plaintiff" || againstParty = "defendant") then
        throw s!"invalid against_party: {againstPartyRaw}"
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some s) => s
        | _ => ""
      let c1 := appendTrace (appendDocket c "Default entered" s!"against={againstParty} reason={reason}")
        "enter_default" "entered" ["FRCP 55(a)"]
      pure <| updateCase s c1
  | "enter_default_judgment" =>
      requireRole a ["judge"]
      let againstPartyRaw ← getString a.payload "against_party"
      let againstParty := normalizePartyToken againstPartyRaw
      if !(againstParty = "plaintiff" || againstParty = "defendant") then
        throw s!"invalid against_party: {againstPartyRaw}"
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some s) => s
        | _ => ""
      let amountOpt ← getFloatOpt a.payload "monetary_amount"
      match amountOpt with
        | some n =>
            if n < 0 then throw "monetary_amount must be >= 0" else pure ()
        | none => pure ()
      let amount := match amountOpt with
        | some n => n
        | none => 0.0
      let cUpdated := { c with status := "judgment_entered", monetary_judgment := amount }
      let amountDesc := match amountOpt with
        | some n => toString n
        | none => "0"
      let c1 := appendTrace (appendDocket cUpdated "Default judgment entered" s!"against={againstParty} amount={amountDesc} reason={reason}")
        "enter_default_judgment" "entered" ["FRCP 55(b)"]
      pure <| updateCase s c1
  | "dismiss_case_rule41" =>
      requireRole a ["plaintiff", "judge"]
      let withPrejudice ← getBoolD a.payload "with_prejudice" false
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some s) => s
        | _ => ""
      let cClosed := { c with status := "closed" }
      let c1 := appendTrace (appendDocket cClosed "Case dismissed (Rule 41)"
        s!"with_prejudice={withPrejudice} reason={reason}")
        "dismiss_case_rule41" (if withPrejudice then "with_prejudice" else "without_prejudice") ["FRCP 41(a)"]
      pure <| updateCase s c1
  | "enter_settlement" =>
      requireRole a ["judge"]
      let summary := match getStringOpt a.payload "summary" with
        | .ok (some s) => s
        | _ => ""
      let amountOpt ← getFloatOpt a.payload "amount"
      match amountOpt with
        | some n =>
            if n < 0 then throw "amount must be >= 0" else pure ()
        | none => pure ()
      let consentJudgment ← getBoolD a.payload "consent_judgment" false
      let amount := match amountOpt with
        | some n => n
        | none => 0.0
      let nextStatus := if consentJudgment || amount > 0.0 then "judgment_entered" else "closed"
      let amountDesc := match amountOpt with
        | some n => toString n
        | none => "0"
      let cSettled := { c with status := nextStatus, monetary_judgment := amount }
      let c1 := appendTrace (appendDocket cSettled "Settlement entered"
        s!"amount={amountDesc} consent_judgment={consentJudgment} summary={summary}")
        "enter_settlement" "entered" ["FRCP 41(a)"]
      pure <| updateCase s c1
  | "resolve_rule60_motion" =>
      requireRole a ["judge"]
      let motionIndex ← getNat a.payload "motion_index"
      let motionCount := countDocketTitle c "Rule 60 Motion"
      if motionIndex >= motionCount then
        throw "rule 60 motion index out of range"
      let orderCount := countDocketTitle c "Rule 60 Order"
      if motionIndex < orderCount then
        throw "rule 60 motion already resolved"
      if motionIndex > orderCount then
        throw "rule 60 motions must be resolved in order"
      let granted ← getBoolD a.payload "granted" false
      let reliefSummary := match getStringOpt a.payload "relief_summary" with
        | .ok (some s) => s
        | _ => ""
      let outcome := if granted then "granted" else "denied"
      let orderDesc := s!"motion_index={motionIndex} disposition={outcome} relief_summary={reliefSummary}"
      let c1 := appendTrace (appendDocket c "Rule 60 Order" orderDesc)
        "resolve_rule60_motion" outcome ["FRCP 60"]
      pure <| updateCase s c1
  | "post_supersedeas_bond" =>
      requireRole a ["judge", "defendant"]
      let effectiveUntil := match getStringOpt a.payload "effective_until" with
        | .ok (some s) => s
        | _ => c.filed_on
      let note := match getStringOpt a.payload "note" with
        | .ok (some s) => s
        | _ => ""
      let c1 := appendTrace (appendDocket c "Supersedeas bond posted"
        s!"effective_until={effectiveUntil} note={note}")
        "post_supersedeas_bond" "posted" ["FRCP 62(b)"]
      pure <| updateCase s c1
  | "order_discretionary_stay" =>
      requireRole a ["judge"]
      let startOn := match getStringOpt a.payload "start_on" with
        | .ok (some s) => s
        | _ => c.filed_on
      let endOn := match getStringOpt a.payload "end_on" with
        | .ok (some s) => s
        | _ => c.filed_on
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some s) => s
        | _ => ""
      let stayIndex := countDocketTitle c "Discretionary Stay Order"
      let c1 := appendTrace (appendDocket c "Discretionary Stay Order"
        s!"stay_index={stayIndex} start_on={startOn} end_on={endOn} reason={reason}")
        "order_discretionary_stay" "entered" ["FRCP 62"]
      pure <| updateCase s c1
  | "lift_stay" =>
      requireRole a ["judge"]
      let stayIndex ← getNat a.payload "stay_index"
      let stayCount := countDocketTitle c "Discretionary Stay Order"
      if stayIndex >= stayCount then
        throw "stay index out of range"
      let liftedCount := countDocketTitle c "Stay Lifted"
      if stayIndex < liftedCount then
        throw "stay already lifted"
      if stayIndex > liftedCount then
        throw "stays must be lifted in order"
      let reason := match getStringOpt a.payload "reason" with
        | .ok (some s) => s
        | _ => ""
      let c1 := appendTrace (appendDocket c "Stay Lifted"
        s!"stay_index={stayIndex} reason={reason}")
        "lift_stay" "lifted" ["FRCP 62"]
      pure <| updateCase s c1
  | "enter_local_rule_override" =>
      requireRole a ["judge"]
      let limitKey ← getString a.payload "limit_key"
      let newValue ← getNat a.payload "new_value"
      let orderedBy ← getString a.payload "ordered_by"
      let reason ← getString a.payload "reason"
      let overrideId :=
        match getString a.payload "override_id" with
        | .ok value =>
            if value.trimAscii.toString = "" then s!"lro-{c.local_rule_overrides.length + 1}" else value
        | .error _ => s!"lro-{c.local_rule_overrides.length + 1}"
      let scopeParty :=
        match getString a.payload "scope_party" with
        | .ok value =>
            if value.trimAscii.toString = "" then none else some (normalizePartyToken value)
        | .error _ => none
      let scopePhase :=
        match getString a.payload "scope_phase" with
        | .ok value =>
            if value.trimAscii.toString = "" then none else some value
        | .error _ => none
      let expiresAt :=
        match getString a.payload "expires_at" with
        | .ok value => if value.trimAscii.toString = "" then none else some value
        | .error _ => none
      let overrideEntry : LocalRuleOverrideV1 :=
        { ordered_at := c.filed_on
        , override_id := overrideId
        , limit_key := limitKey
        , new_value := newValue
        , scope_party := scopeParty
        , scope_phase := scopePhase
        , ordered_by := orderedBy
        , reason := reason
        , active := true
        , expires_at := expiresAt
        }
      let c1 := appendTrace { c with local_rule_overrides := c.local_rule_overrides.concat overrideEntry }
        "enter_local_rule_override" overrideId ["Local Rule 1", "FRCP 83"]
      let c2 := appendDocket c1 s!"Local Rule Override {overrideId}"
        s!"limit_key={limitKey} new_value={newValue}"
      pure <| updateCase s c2
  | "file_bench_opinion" =>
      requireRole a ["judge"]
      let text ← getString a.payload "text"
      validateBenchOpinion c text
      let c1 := appendTrace (appendDocket c "Bench Opinion" text) "file_bench_opinion" "entered" ["FRCP 52(a)(1)"]
      pure <| updateCase s c1
  | "enter_judgment" =>
      requireRole a ["judge"]
      validateEnterJudgment c
      requireSingleClaimMetadata c
      requireClaimIdMatch c a.payload
      let basis ← getString a.payload "basis"
      let amount := judgmentAmountFromCaseState c
      let c1 := appendTrace { c with monetary_judgment := amount } "enter_judgment" basis ["FRCP 58"]
      let c2 := appendDocket { c1 with status := "judgment_entered" } "Judgment entered" basis
      pure <| updateCase s c2
  | "hold_in_contempt" =>
      requireRole a ["judge"]
      if c.status != "trial" then
        throw "contempt finding requires trial status"
      let targetRoleRaw ← getString a.payload "target_role"
      let targetRole := normalizePartyToken targetRoleRaw
      if !(targetRole = "plaintiff" || targetRole = "defendant" || targetRole = "clerk" || targetRole = "juror") then
        throw s!"invalid contempt target role: {targetRoleRaw}"
      let reason ← getString a.payload "reason"
      let severityRaw ← getString a.payload "severity"
      let severity := severityRaw.toLower
      if !(severity = "warning" || severity = "contempt") then
        throw s!"invalid contempt severity: {severityRaw}"
      let c1 :=
        if severity = "contempt" then
          { c with contempt_counts := incrementContemptCount c.contempt_counts targetRole }
        else
          c
      pure <| updateCase s (appendDocket c1 s!"Court {severity} for {targetRole}" reason)
  | _ =>
      throw s!"unknown action_type: {a.action_type}"

def parseJsonInput (input : String) : Except String Json := do
  Json.parse input

def parseStepRequest (j : Json) : Except String StepRequest := do
  fromJson? j

def parseViewRequest (j : Json) : Except String ViewRequest := do
  fromJson? j

def parseOpportunityRequest (j : Json) : Except String OpportunityRequest := do
  fromJson? j

def parseApplyDecisionRequest (j : Json) : Except String ApplyDecisionRequest := do
  fromJson? j

def parseInitializeCaseRequest (j : Json) : Except String InitializeCaseRequest := do
  let stateJson ←
    match j.getObjVal? "state" with
    | .ok v => pure v
    | .error _ => throw "payload field state missing"
  let state : CourtState ← fromJson? stateJson
  let complaintSummary ← getString j "complaint_summary"
  let filedBy :=
    match (← getStringOpt j "filed_by") with
    | some v => v
    | none => "plaintiff"
  let juryDemandedOn :=
    match (← getStringOpt j "jury_demanded_on") with
    | some v => v
    | none => ""
  let jurisdictionalAllegations :=
    match j.getObjVal? "jurisdictional_allegations" with
    | .ok v => some v
    | .error _ => none
  let attachments ←
    match j.getObjVal? "attachments" with
    | .ok v => fromJson? v
    | .error _ => pure []
  pure {
    state := state
    complaint_summary := complaintSummary
    filed_by := filedBy
    jury_demanded_on := juryDemandedOn
    jurisdictional_allegations := jurisdictionalAllegations
    attachments := attachments
  }

def parseLimitErrorToken (token : String) : Option (String × String) :=
  match token.splitOn "=" with
  | [k, v] => some (k, v)
  | _ => none

def parseLimitErrorDetails (msg : String) : Option (List (String × String)) :=
  if msg.startsWith "LOCAL_RULE_LIMIT_EXCEEDED|" then
    some <| (msg.splitOn "|").drop 1 |>.filterMap parseLimitErrorToken
  else
    none

def renderError (msg : String) : String :=
  match parseLimitErrorDetails msg with
  | some kvs =>
      let detailsJson := Json.mkObj (kvs.map (fun (k, v) => (k, toJson v)))
      Json.compress (toJson ({ ok := false, error := msg, code := "LOCAL_RULE_LIMIT_EXCEEDED", details := detailsJson, actor_message := msg } : StepErr))
  | none =>
      Json.compress (toJson ({ ok := false, error := msg, actor_message := msg } : StepErr))

def renderOk (state : CourtState) : String :=
  Json.compress (toJson ({ ok := true, state := state } : StepOk))

def renderViewOk (view : Json) : String :=
  Json.compress (toJson ({ ok := true, view := view } : ViewOk))

def renderStepErr (err : StepErr) : String :=
  Json.compress (toJson err)

def renderNextOpportunityOk (resp : NextOpportunityOk) : String :=
  Json.compress (toJson resp)

def renderAgendaOk (resp : AgendaOk) : String :=
  Json.compress (toJson resp)

def renderApplyDecisionOk (resp : ApplyDecisionOk) : String :=
  Json.compress (toJson resp)

def main (_args : List String) : IO UInt32 := do
  let stdin ← IO.getStdin
  let input ← stdin.readToEnd
  match parseJsonInput input with
  | .error e =>
      IO.println (renderError s!"invalid request: {e}")
  | .ok j =>
      let requestType :=
        match j.getObjVal? "request_type" with
        | .ok v =>
            match v.getStr? with
            | .ok s => s
            | .error _ => ""
        | .error _ => ""
      if requestType = "view_state" || requestType = "role_view" then
        match parseViewRequest j with
        | .error e =>
            IO.println (renderError s!"invalid request: {e}")
        | .ok req =>
            match viewForRole req.state req.role with
            | .error e => IO.println (renderError e)
            | .ok view => IO.println (renderViewOk view)
      else if requestType = "next_opportunity" then
        match parseOpportunityRequest j with
        | .error e =>
            IO.println (renderError s!"invalid request: {e}")
        | .ok req =>
            IO.println (renderNextOpportunityOk (nextOpportunity req))
      else if requestType = "agenda" then
        match parseOpportunityRequest j with
        | .error e =>
            IO.println (renderError s!"invalid request: {e}")
        | .ok req =>
            let opportunities := openOpportunities req
            IO.println (renderAgendaOk { state_version := req.state.state_version, terminal := opportunities.isEmpty, opportunities := opportunities })
      else if requestType = "apply_decision" then
        match parseApplyDecisionRequest j with
        | .error e =>
            IO.println (renderError s!"invalid request: {e}")
        | .ok req =>
            match applyDecision req with
            | .error err => IO.println (renderStepErr err)
            | .ok resp => IO.println (renderApplyDecisionOk resp)
      else if requestType = "initialize_case" then
        match parseInitializeCaseRequest j with
        | .error e =>
            IO.println (renderError s!"invalid request: {e}")
        | .ok req =>
            match initializeCase req with
            | .error e => IO.println (renderError e)
            | .ok state => IO.println (renderOk state)
      else
        match parseStepRequest j with
        | .error e =>
            IO.println (renderError s!"invalid request: {e}")
        | .ok req =>
            match step req.state req.action with
            | .error e => IO.println (renderError e)
            | .ok state => IO.println (renderOk state)
  pure 0
