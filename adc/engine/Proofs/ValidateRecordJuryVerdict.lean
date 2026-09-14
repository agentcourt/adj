import ADC.Core

namespace ADCProofs.ValidateRecordJuryVerdict

def swornJuror (jurorId : String) : JurorRecord :=
  { juror_id := jurorId, name := jurorId, status := "sworn" }

def plaintiffVote (jurorId : String) (round damages : Nat) : JurorVote :=
  {
    juror_id := jurorId
    round := round
    vote := "plaintiff"
    damages := Float.ofNat damages
    confidence := "high"
    explanation := "Plaintiff proved the claim."
    submitted_at := "2026-01-01"
  }

def defendantVote (jurorId : String) (round : Nat) : JurorVote :=
  {
    juror_id := jurorId
    round := round
    vote := "defendant"
    damages := 0.0
    confidence := "high"
    explanation := "Plaintiff did not prove the claim."
    submitted_at := "2026-01-01"
  }

def baseCase : CaseState :=
  { (default : CaseState) with
    case_id := "case-1"
    filed_on := "2026-01-01"
    status := "trial"
    trial_mode := "jury"
    phase := "deliberation"
    jury_configuration := some {
      juror_count := 2
      unanimous_required := true
      minimum_concurring := 2
    }
    jurors := [swornJuror "J1", swornJuror "J2"]
  }

def stateOf (c : CaseState := baseCase) (version : Nat := 0) : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    case := c
    state_version := version
  }

def submitVoteAction
    (jurorId vote confidence : String)
    (damages : Nat) : CourtAction :=
  { action_type := "submit_juror_vote"
  , actor_role := "juror"
  , payload := Lean.Json.mkObj [
      ("juror_id", Lean.Json.str jurorId),
      ("vote", Lean.Json.str vote),
      ("damages", Lean.Json.num damages),
      ("confidence", Lean.Json.str confidence),
      ("explanation", Lean.Json.str "The record supports this vote.")
    ]
  }

def stepErrorMessage (result : Except String CourtState) : String :=
  match result with
  | .error message => message
  | .ok _ => ""

theorem submit_juror_vote_requires_deliberation_phase :
    let c := { baseCase with phase := "verdict_return" }
    stepErrorMessage (step (stateOf c) (submitVoteAction "J1" "plaintiff" "high" 100)) =
      "juror vote requires deliberation phase; current phase is verdict_return" := by
  native_decide

theorem submit_juror_vote_requires_known_juror :
    stepErrorMessage (step (stateOf) (submitVoteAction "J3" "plaintiff" "high" 100)) =
      "unknown juror_id: J3" := by
  native_decide

theorem submit_juror_vote_requires_sworn_juror :
    let c := { baseCase with jurors := [
      { swornJuror "J1" with status := "candidate" },
      swornJuror "J2"
    ] }
    stepErrorMessage (step (stateOf c) (submitVoteAction "J1" "plaintiff" "high" 100)) =
      "juror J1 is not sworn" := by
  native_decide

theorem submit_juror_vote_rejects_duplicate_in_current_round :
    let c := { baseCase with juror_votes := [plaintiffVote "J1" 1 100] }
    stepErrorMessage (step (stateOf c) (submitVoteAction "J1" "plaintiff" "high" 100)) =
      "juror vote already submitted for round 1: J1" := by
  native_decide

theorem submit_juror_vote_rejects_invalid_side :
    stepErrorMessage (step (stateOf) (submitVoteAction "J1" "abstain" "high" 0)) =
      "invalid juror vote: abstain" := by
  native_decide

theorem submit_juror_vote_requires_zero_damages_for_defendant :
    stepErrorMessage (step (stateOf) (submitVoteAction "J1" "defendant" "high" 100)) =
      "juror vote damages must be zero on a defense vote" := by
  native_decide

theorem submit_juror_vote_rejects_invalid_confidence :
    stepErrorMessage (step (stateOf) (submitVoteAction "J1" "plaintiff" "certain" 100)) =
      "invalid confidence: certain" := by
  native_decide

theorem submit_juror_vote_appends_current_round_vote :
    (match step (stateOf) (submitVoteAction "J1" "plaintiff" "high" 100) with
    | .error _ => false
    | .ok next =>
        next.case.juror_votes.any (fun vote =>
          vote.juror_id = "J1" &&
          vote.round = 1 &&
          vote.vote = "plaintiff")) = true := by
  native_decide

theorem submit_juror_vote_increments_state_version :
    (match step (stateOf baseCase 7) (submitVoteAction "J1" "plaintiff" "high" 100) with
    | .error _ => 0
    | .ok next => next.state_version) = 8 := by
  native_decide

theorem incomplete_ballot_does_not_derive_verdict :
    (match step (stateOf) (submitVoteAction "J1" "plaintiff" "high" 100) with
    | .error _ => false
    | .ok next => next.case.jury_verdict.isNone) = true := by
  native_decide

def plaintiffVerdictAfterFinalVote : Option JuryVerdict :=
  let c := { baseCase with juror_votes := [plaintiffVote "J1" 1 100] }
  match step (stateOf c) (submitVoteAction "J2" "plaintiff" "high" 300) with
  | .error _ => none
  | .ok next => next.case.jury_verdict

theorem final_concurring_vote_derives_plaintiff_verdict :
    (match plaintiffVerdictAfterFinalVote with
    | some verdict =>
        verdict.verdict_for = "plaintiff" &&
        verdict.votes_for_verdict = 2 &&
        verdict.required_votes = 2
    | none => false) = true := by
  native_decide

theorem plaintiff_verdict_uses_mean_plaintiff_damages :
    (match plaintiffVerdictAfterFinalVote with
    | some verdict => verdict.damages.toBits = (200.0).toBits
    | none => false) = true := by
  native_decide

def defendantVerdictAfterFinalVote : Option JuryVerdict :=
  let c := { baseCase with juror_votes := [defendantVote "J1" 1] }
  match step (stateOf c) (submitVoteAction "J2" "defendant" "high" 0) with
  | .error _ => none
  | .ok next => next.case.jury_verdict

theorem final_concurring_vote_derives_defendant_verdict :
    (match defendantVerdictAfterFinalVote with
    | some verdict =>
        verdict.verdict_for = "defendant" &&
        verdict.votes_for_verdict = 2 &&
        verdict.required_votes = 2
    | none => false) = true := by
  native_decide

theorem defendant_verdict_has_zero_damages :
    (match defendantVerdictAfterFinalVote with
    | some verdict => verdict.damages.toBits = (0.0).toBits
    | none => false) = true := by
  native_decide

theorem prior_round_vote_does_not_block_current_round_vote :
    let c := { baseCase with
      deliberation_round := 2
      juror_votes := [plaintiffVote "J1" 1 100]
    }
    (match step (stateOf c) (submitVoteAction "J1" "plaintiff" "high" 100) with
    | .error _ => false
    | .ok next =>
        next.case.juror_votes.any (fun vote => vote.juror_id = "J1" && vote.round = 2)) = true := by
  native_decide

end ADCProofs.ValidateRecordJuryVerdict
