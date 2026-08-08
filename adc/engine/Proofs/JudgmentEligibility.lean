import Main

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1",
    status := "trial",
    phase := "post_verdict"
  }

theorem judgmentEligible_bench_without_hung_true :
    let c := { baseCase with trial_mode := "bench", hung_jury := none }
    judgmentEligibleFromCaseStateV1 c = true := by
  native_decide

theorem judgmentEligible_bench_with_hung_false :
    let c := { baseCase with trial_mode := "bench", hung_jury := some { claim_id := "claim-1", note := "deadlock" } }
    judgmentEligibleFromCaseStateV1 c = false := by
  native_decide

theorem judgmentEligible_jury_pending_false :
    let c := { baseCase with trial_mode := "jury", jury_verdict := none, hung_jury := none }
    judgmentEligibleFromCaseStateV1 c = false := by
  native_decide

theorem judgmentEligible_jury_hung_false :
    let c := { baseCase with
      trial_mode := "jury",
      jury_verdict := none,
      hung_jury := some { claim_id := "claim-1", note := "deadlock" } }
    judgmentEligibleFromCaseStateV1 c = false := by
  native_decide

theorem judgmentEligible_jury_plaintiff_verdict_true :
    let c := { baseCase with
      trial_mode := "jury",
      jury_verdict := some { verdict_for := "plaintiff", votes_for_verdict := 6, required_votes := 6, damages := 108.0 },
      hung_jury := none }
    judgmentEligibleFromCaseStateV1 c = true := by
  native_decide

theorem judgmentEligible_jury_defendant_verdict_true :
    let c := { baseCase with
      trial_mode := "jury",
      jury_verdict := some { verdict_for := "defendant", votes_for_verdict := 6, required_votes := 6, damages := 0.0 },
      hung_jury := none }
    judgmentEligibleFromCaseStateV1 c = true := by
  native_decide

theorem judgmentEligible_jury_invalid_verdict_token_false :
    let c := { baseCase with
      trial_mode := "jury",
      jury_verdict := some { verdict_for := "invalid-side", votes_for_verdict := 6, required_votes := 6, damages := 0.0 },
      hung_jury := none }
    judgmentEligibleFromCaseStateV1 c = false := by
  native_decide
