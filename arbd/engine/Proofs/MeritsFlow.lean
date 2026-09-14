import Proofs.Samples

namespace ArbdProofs

def afterPlaintiffOpening : Except String ArbitrationState := do
  step { state := initializedState, action := openingAction initializedState "plaintiff" "Plaintiff opening." }

def afterTwoOpenings : Except String ArbitrationState := do
  let s1 ← afterPlaintiffOpening
  step { state := s1, action := openingAction s1 "defendant" "Defendant opening." }

def afterPlaintiffArgument : Except String ArbitrationState := do
  let s1 ← afterTwoOpenings
  step { state := s1, action := argumentAction s1 "plaintiff" "Plaintiff argument." }

def afterPlaintiffSubmittedEvidence : Except String ArbitrationState := do
  let s1 ← afterTwoOpenings
  step { state := s1, action := submittedEvidenceAction s1 "plaintiff" }

def afterTwoArguments : Except String ArbitrationState := do
  let s1 ← afterPlaintiffArgument
  step { state := s1, action := argumentAction s1 "defendant" "Defendant argument." }

def afterPlaintiffRebuttal : Except String ArbitrationState := do
  let s1 ← afterTwoArguments
  step { state := s1, action := rebuttalAction s1 "Plaintiff rebuttal." }

def afterDefendantSurrebuttal : Except String ArbitrationState := do
  let s1 ← afterPlaintiffRebuttal
  step { state := s1, action := surrebuttalAction s1 "Defendant surrebuttal." }

def afterPlaintiffClosing : Except String ArbitrationState := do
  let s1 ← afterDefendantSurrebuttal
  step { state := s1, action := closingAction s1 "plaintiff" "Plaintiff closing." }

def afterTwoClosings : Except String ArbitrationState := do
  let s1 ← afterPlaintiffClosing
  step { state := s1, action := closingAction s1 "defendant" "Defendant closing." }

def afterPassedRebuttal : Except String ArbitrationState := do
  let s1 ← afterTwoArguments
  step { state := s1, action := passAction s1 "plaintiff" }

def afterPassedSurrebuttal : Except String ArbitrationState := do
  let s1 ← afterPassedRebuttal
  step { state := s1, action := passAction s1 "defendant" }

def nextOpportunityPhaseAfter (result : Except String ArbitrationState) : String :=
  match result with
  | .ok state => stateNextPhase (nextOpportunity state)
  | .error _ => ""

theorem nextOpportunity_starts_with_plaintiff_opening :
    stateNextRole (nextOpportunity initializedState) = "plaintiff" ∧
      stateNextPhase (nextOpportunity initializedState) = "openings" ∧
      stateNextTool (nextOpportunity initializedState) = "record_opening_statement" := by
  native_decide

theorem second_opening_advances_to_arguments :
    statePhase afterTwoOpenings = "arguments" := by
  native_decide

theorem second_argument_advances_to_rebuttals :
    statePhase afterTwoArguments = "rebuttals" := by
  native_decide

theorem submitted_evidence_stays_in_arguments :
    statePhase afterPlaintiffSubmittedEvidence = "arguments" ∧
      stateSubmittedEvidenceCount afterPlaintiffSubmittedEvidence = 1 := by
  native_decide

theorem filed_rebuttal_and_surrebuttal_advance_the_case :
    statePhase afterPlaintiffRebuttal = "surrebuttals" ∧
      statePhase afterDefendantSurrebuttal = "closings" := by
  native_decide

theorem pass_actions_advance_optional_merits_phases :
    statePhase afterPassedRebuttal = "surrebuttals" ∧
      statePhase afterPassedSurrebuttal = "closings" := by
  native_decide

theorem second_closing_opens_deliberation :
    statePhase afterTwoClosings = "deliberation" ∧
      nextOpportunityPhaseAfter afterTwoClosings = "deliberation" := by
  native_decide

end ArbdProofs
