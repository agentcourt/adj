import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    filed_on := "2026-01-01",
    status := "trial",
    trial_mode := "bench",
    phase := "verdict_return"
  }

def stateOf (c : CaseState := baseCase) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1",
    case := c
  }

def addBenchFindingAction : CourtAction :=
  { action_type := "add_bench_finding"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("issue", Lean.Json.str "Breach")
      , ("finding", Lean.Json.str "Defendant breached duty by failing controls")
      ]
  }

def addBenchConclusionAction : CourtAction :=
  { action_type := "add_bench_conclusion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("issue", Lean.Json.str "Liability")
      , ("conclusion", Lean.Json.str "Defendant liable under negligence standard")
      ]
  }

def fileBenchOpinionAction : CourtAction :=
  { action_type := "file_bench_opinion"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("text", Lean.Json.str "Findings and conclusions entered for judgment.") ]
  }

def stepErrorMessage (r : Except String CourtState) : String :=
  match r with
  | .error msg => msg
  | .ok _ => ""

theorem step_add_bench_finding_requires_trial_status :
    let c := { baseCase with status := "pretrial" }
    stepErrorMessage (step (stateOf c) addBenchFindingAction) =
      "bench finding requires trial status" := by
  native_decide

theorem step_add_bench_conclusion_requires_trial_status :
    let c := { baseCase with status := "pretrial" }
    stepErrorMessage (step (stateOf c) addBenchConclusionAction) =
      "bench conclusion requires trial status" := by
  native_decide

theorem step_add_bench_finding_appends_entry :
    (match step (stateOf) addBenchFindingAction with
      | .ok s' => s'.case.bench_findings.length
      | .error _ => 0) = 1 := by
  native_decide

theorem step_add_bench_conclusion_appends_entry :
    (match step (stateOf) addBenchConclusionAction with
      | .ok s' => s'.case.bench_conclusions.length
      | .error _ => 0) = 1 := by
  native_decide

theorem step_file_bench_opinion_records_docket :
    (match step (stateOf) fileBenchOpinionAction with
      | .ok s' => hasDocketTitle s'.case "Bench Opinion"
      | .error _ => false) = true := by
  native_decide
