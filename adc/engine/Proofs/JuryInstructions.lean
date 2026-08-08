import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "trial",
    phase := "charge_conference"
  }

def mkState (c : CaseState) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def proposeInstructionAction : CourtAction :=
  { action_type := "propose_jury_instruction"
  , actor_role := "plaintiff"
  , payload := Lean.Json.mkObj
      [ ("party", Lean.Json.str "plaintiff")
      , ("instruction_id", Lean.Json.str "PI-1")
      , ("text", Lean.Json.str "Apply preponderance of the evidence.")
      ]
  }

def settleInstructionsAction : CourtAction :=
  { action_type := "settle_jury_instructions"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [ ("summary", Lean.Json.str "Final instruction set adopted.") ]
  }

def deliverInstructionsAction : CourtAction :=
  { action_type := "deliver_jury_instructions"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj [ ("text", Lean.Json.str "Members of the jury, you must apply these instructions to the facts.") ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_propose_jury_instruction_requires_charge_conference :
    let c := { baseCase with phase := "closings" }
    stepErrorMessage (step (mkState c) proposeInstructionAction) =
      "jury instruction proposal requires charge_conference phase; current phase is closings" := by
  native_decide

theorem step_settle_jury_instructions_requires_jury_charge :
    stepErrorMessage (step (mkState baseCase) settleInstructionsAction) =
      "settle jury instructions requires jury_charge phase; current phase is charge_conference" := by
  native_decide

theorem step_settle_jury_instructions_requires_proposal :
    let c := { baseCase with phase := "jury_charge" }
    stepErrorMessage (step (mkState c) settleInstructionsAction) =
      "cannot settle jury instructions before any proposed instructions are filed" := by
  native_decide

theorem step_deliver_jury_instructions_requires_settlement :
    let c := { baseCase with phase := "jury_charge" }
    stepErrorMessage (step (mkState c) deliverInstructionsAction) =
      "cannot deliver jury instructions before settlement" := by
  native_decide

theorem step_settle_jury_instructions_records_docket :
    let c := { baseCase with
      phase := "jury_charge",
      docket := [{ title := "Proposed jury instruction - plaintiff", description := "instruction_id=PI-1" }]
    }
    (match step (mkState c) settleInstructionsAction with
      | .ok s' => hasDocketTitle s'.case "Jury instructions settled"
      | .error _ => false) = true := by
  native_decide

theorem step_deliver_jury_instructions_records_docket :
    let c := { baseCase with
      phase := "jury_charge",
      docket := [{ title := "Jury instructions settled", description := "Final instruction set adopted." }]
    }
    (match step (mkState c) deliverInstructionsAction with
      | .ok s' => hasDocketTitle s'.case "Jury instructions delivered"
      | .error _ => false) = true := by
  native_decide
