import ADC.Core

open Lean

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
