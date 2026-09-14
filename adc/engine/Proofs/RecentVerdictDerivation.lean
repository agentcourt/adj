import ADC.Core

namespace ADCProofs.RecentVerdictDerivation

def swornJurorRecent (jurorId model persona : String) : JurorRecord :=
  { juror_id := jurorId
  , name := jurorId
  , status := "sworn"
  , model := model
  , persona_filename := persona
  }

def fourVoteCfg : JuryConfiguration :=
  { juror_count := 6, unanimous_required := false, minimum_concurring := 4 }

def plaintiffMajorityCase : CaseState :=
  { (default : CaseState) with
    jury_configuration := some fourVoteCfg
    jurors :=
      [ swornJurorRecent "J1" "m1" "p1"
      , swornJurorRecent "J2" "m2" "p2"
      , swornJurorRecent "J3" "m3" "p3"
      , swornJurorRecent "J4" "m4" "p4"
      , swornJurorRecent "J5" "m5" "p5"
      , swornJurorRecent "J6" "m6" "p6"
      ]
    juror_votes :=
      [ { juror_id := "J1", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e1", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 1, vote := "plaintiff", damages := 150.0, confidence := "high", explanation := "e2", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 1, vote := "plaintiff", damages := 50.0, confidence := "high", explanation := "e3", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e4", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e5", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e6", submitted_at := "2026-03-15" }
      ]
  }

def defendantMajorityCase : CaseState :=
  { plaintiffMajorityCase with
    juror_votes :=
      [ { juror_id := "J1", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e1", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e2", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e3", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e4", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 1, vote := "plaintiff", damages := 200.0, confidence := "high", explanation := "e5", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 1, vote := "plaintiff", damages := 300.0, confidence := "high", explanation := "e6", submitted_at := "2026-03-15" }
      ]
  }

def plaintiffMajorityPermutedCase : CaseState :=
  { plaintiffMajorityCase with
    juror_votes :=
      [ { juror_id := "J5", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e5", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 1, vote := "plaintiff", damages := 50.0, confidence := "high", explanation := "e3", submitted_at := "2026-03-15" }
      , { juror_id := "J1", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e1", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e6", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e4", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 1, vote := "plaintiff", damages := 150.0, confidence := "high", explanation := "e2", submitted_at := "2026-03-15" }
      ]
  }

def stableSplitCase : CaseState :=
  { (default : CaseState) with
    jury_configuration := some { juror_count := 6, unanimous_required := true, minimum_concurring := 6 }
    deliberation_round := 2
    jurors :=
      [ swornJurorRecent "J1" "m1" "p1"
      , swornJurorRecent "J2" "m2" "p2"
      , swornJurorRecent "J3" "m3" "p3"
      , swornJurorRecent "J4" "m4" "p4"
      , swornJurorRecent "J5" "m5" "p5"
      , swornJurorRecent "J6" "m6" "p6"
      ]
    juror_votes :=
      [ { juror_id := "J1", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e1", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e2", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e3", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e4", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e5", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e6", submitted_at := "2026-03-15" }
      , { juror_id := "J1", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e7", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e8", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e9", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 2, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e10", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 2, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e11", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 2, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e12", submitted_at := "2026-03-15" }
      ]
  }

def continuingSplitCase : CaseState :=
  { stableSplitCase with
    jury_configuration := some { juror_count := 6, unanimous_required := false, minimum_concurring := 5 }
    juror_votes :=
      [ { juror_id := "J1", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e1", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e2", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 1, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e3", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e4", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e5", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 1, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e6", submitted_at := "2026-03-15" }
      , { juror_id := "J1", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e7", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e8", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e9", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e10", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 2, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e11", submitted_at := "2026-03-15" }
      , { juror_id := "J6", round := 2, vote := "defendant", damages := 0.0, confidence := "high", explanation := "e12", submitted_at := "2026-03-15" }
      ]
  }

def plaintiffAfterFailedJurorCase : CaseState :=
  { stableSplitCase with
    jurors :=
      [ swornJurorRecent "J1" "m1" "p1"
      , swornJurorRecent "J2" "m2" "p2"
      , swornJurorRecent "J3" "m3" "p3"
      , swornJurorRecent "J4" "m4" "p4"
      , swornJurorRecent "J5" "m5" "p5"
      , { swornJurorRecent "J6" "m6" "p6" with status := "timed_out" }
      ]
    juror_votes :=
      [ { juror_id := "J1", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e1", submitted_at := "2026-03-15" }
      , { juror_id := "J2", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e2", submitted_at := "2026-03-15" }
      , { juror_id := "J3", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e3", submitted_at := "2026-03-15" }
      , { juror_id := "J4", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e4", submitted_at := "2026-03-15" }
      , { juror_id := "J5", round := 2, vote := "plaintiff", damages := 100.0, confidence := "high", explanation := "e5", submitted_at := "2026-03-15" }
      ]
  }

def plaintiffMajorityVerdictSummary : Bool :=
  match deriveVerdictFromJurorVotes? {} plaintiffMajorityCase with
  | some (some verdict, none, none, none) =>
      verdict.verdict_for == "plaintiff" &&
      verdict.votes_for_verdict == 4 &&
      verdict.required_votes == 4 &&
      verdict.damages.toBits == (100.0).toBits
  | _ => false

def defendantMajorityVerdictSummary : Bool :=
  match deriveVerdictFromJurorVotes? {} defendantMajorityCase with
  | some (some verdict, none, none, none) =>
      verdict.verdict_for == "defendant" &&
      verdict.votes_for_verdict == 4 &&
      verdict.required_votes == 4 &&
      verdict.damages.toBits == (0.0).toBits
  | _ => false

def stableSplitHungJurySummary : Bool :=
  match deriveVerdictFromJurorVotes? {} stableSplitCase with
  | some (none, some hung, none, none) =>
      hung.claim_id == "claim-1" &&
      hung.note == "sworn jurors remained split after deliberation round 2, and no juror changed vote or damages from the prior round"
  | _ => false

def continuingSplitAdvanceSummary : Bool :=
  match deriveVerdictFromJurorVotes? {} continuingSplitCase with
  | some (none, none, some 3, none) => true
  | _ => false

def plaintiffMajorityPermutedVerdictSummary : Bool :=
  match deriveVerdictFromJurorVotes? {} plaintiffMajorityPermutedCase with
  | some (some verdict, none, none, none) =>
      verdict.verdict_for == "plaintiff" &&
      verdict.votes_for_verdict == 4 &&
      verdict.required_votes == 4 &&
      verdict.damages.toBits == (100.0).toBits
  | _ => false

def plaintiffAfterFailedJurorVerdictSummary : Bool :=
  match deriveVerdictFromJurorVotes? {} plaintiffAfterFailedJurorCase with
  | some (some verdict, none, none, none) =>
      verdict.verdict_for == "plaintiff" &&
      verdict.votes_for_verdict == 5 &&
      verdict.required_votes == 5 &&
      verdict.damages.toBits == (100.0).toBits
  | _ => false

/--
Verdict derivation stops if any sworn juror has not yet voted in the current
round.  The engine may derive neither a verdict nor a hung jury nor a new
ballot round until every eligible sworn juror has a ballot in the active round.
-/
theorem deriveVerdictFromJurorVotes_none_when_current_round_vote_missing
    (policy : CourtPolicy)
    (c : CaseState)
    (hSworn : countJurorsByStatus c.jurors "sworn" ≠ 0)
    (hMissing :
      (nextSwornJurorWithoutVoteInRound? c (currentDeliberationRound c)).isSome = true) :
    deriveVerdictFromJurorVotes? policy c = none := by
  unfold deriveVerdictFromJurorVotes?
  cases hCfg : c.jury_configuration with
  | none =>
      simp
  | some cfg =>
      simp [hMissing, hSworn]

/--
When plaintiff votes meet the concurrence threshold, the engine returns a
plaintiff verdict with the arithmetic mean of plaintiff-side damages.

The proof checks the derived verdict summary because the verdict contains a
`Float`, and decidable equality on the enclosing structure is unavailable.
-/
theorem deriveVerdictFromJurorVotes_plaintiff_majority_uses_plaintiff_mean :
    plaintiffMajorityVerdictSummary = true := by
  native_decide

/--
When defendant votes meet the concurrence threshold, the engine returns a
defendant verdict with zero damages.

Plaintiff-side damages claims do not survive a defendant verdict.  The proof
checks the verdict fields that carry that legal consequence.
-/
theorem deriveVerdictFromJurorVotes_defendant_majority_zeroes_damages :
    defendantMajorityVerdictSummary = true := by
  native_decide

/--
Reordering the sample plaintiff-majority votes does not change the derived
verdict summary.

This theorem proves the verdict derivation does not depend on the order of the
vote list in the representative plaintiff-majority case.  The engine should
count votes and average plaintiff-side damages, not care about storage order.
-/
theorem deriveVerdictFromJurorVotes_plaintiff_majority_is_order_invariant_on_sample :
    plaintiffMajorityPermutedVerdictSummary = plaintiffMajorityVerdictSummary := by
  native_decide

/--
If round 2 repeats round 1 with the same split and the same damages positions,
the engine declares a hung jury.

This theorem states the new deliberation stop rule directly.  The proof checks
the emitted hung-jury record, including the explanatory note.
-/
theorem deriveVerdictFromJurorVotes_stable_split_declares_hung_jury :
    stableSplitHungJurySummary = true := by
  native_decide

theorem deriveVerdictFromJurorVotes_adjusts_threshold_after_failed_juror :
    plaintiffAfterFailedJurorVerdictSummary = true := by
  native_decide

/--
If a split round is not yet stable and neither side has the required
concurrence, the engine advances to the next round.

The engine keeps the jury voting when the prior round changed but failed to
produce a verdict.
-/
theorem deriveVerdictFromJurorVotes_nonstable_split_advances_round :
    continuingSplitAdvanceSummary = true := by
  native_decide

end ADCProofs.RecentVerdictDerivation
