import Proofs.Samples

open ArbProofs

/-
This file proves the ordered merits sequence of `agentarbitration`.

The arbitration engine is intentionally linear.  That design choice should be
easy to state and easy to check.  The useful questions are these.

Who receives the first opening opportunity?

What closes a bilateral phase such as openings or arguments?

What closes a single-sided phase such as rebuttal or surrebuttal?

What does a pass do in those optional phases?

When does the case stop generating attorney opportunities and begin council
deliberation?

The proofs below answer those questions by running the real `step` function on
small sample states.  That keeps the theorems close to the actual engine
behavior instead of proving claims about a hand-written approximation of the
state machine.
-/

def afterPlaintiffOpening : Except String ArbitrationState := do
  step { state := initializedState, action := openingAction "plaintiff" "Plaintiff opening." }

def afterTwoOpenings : Except String ArbitrationState := do
  let s1 ← afterPlaintiffOpening
  step { state := s1, action := openingAction "defendant" "Defendant opening." }

def afterPlaintiffArgument : Except String ArbitrationState := do
  let s1 ← afterTwoOpenings
  step { state := s1, action := argumentAction "plaintiff" "Plaintiff argument." }

def afterTwoArguments : Except String ArbitrationState := do
  let s1 ← afterPlaintiffArgument
  step { state := s1, action := argumentAction "defendant" "Defendant argument." }

def afterPlaintiffRebuttal : Except String ArbitrationState := do
  let s1 ← afterTwoArguments
  step { state := s1, action := rebuttalAction "Plaintiff rebuttal." }

def afterDefendantSurrebuttal : Except String ArbitrationState := do
  let s1 ← afterPlaintiffRebuttal
  step { state := s1, action := surrebuttalAction "Defendant surrebuttal." }

def afterPlaintiffClosing : Except String ArbitrationState := do
  let s1 ← afterDefendantSurrebuttal
  step { state := s1, action := closingAction "plaintiff" "Plaintiff closing." }

def afterTwoClosings : Except String ArbitrationState := do
  let s1 ← afterPlaintiffClosing
  step { state := s1, action := closingAction "defendant" "Defendant closing." }

def afterPassedRebuttal : Except String ArbitrationState := do
  let s1 ← afterTwoArguments
  step { state := s1, action := passAction "plaintiff" }

def afterPassedSurrebuttal : Except String ArbitrationState := do
  let s1 ← afterPassedRebuttal
  step { state := s1, action := passAction "defendant" }

def nextOpportunityPhaseAfter (result : Except String ArbitrationState) : String :=
  match result with
  | .ok state => stateNextPhase (nextOpportunity state)
  | .error _ => ""

/--
The initialized case begins with the plaintiff opening opportunity.

This theorem fixes the first ordering choice in the procedure.  The engine
begins with the plaintiff, in the openings phase, and offers the opening
statement tool.
-/
theorem nextOpportunity_starts_with_plaintiff_opening :
    stateNextRole (nextOpportunity initializedState) = "plaintiff" ∧
      stateNextPhase (nextOpportunity initializedState) = "openings" ∧
      stateNextTool (nextOpportunity initializedState) = "record_opening_statement" := by
  native_decide

/--
The second opening closes openings and moves the case to arguments.

This theorem states the basic rule for the bilateral phases: once both sides
file, the phase ends immediately.  There is no separate phase-advance action.
-/
theorem second_opening_advances_to_arguments :
    statePhase afterTwoOpenings = "arguments" := by
  native_decide

/--
The second merits argument closes arguments and moves the case to rebuttals.

This is the same bilateral rule one phase later.  The procedure should treat
arguments as complete once both sides have filed.
-/
theorem second_argument_advances_to_rebuttals :
    statePhase afterTwoArguments = "rebuttals" := by
  native_decide

/--
Submitting a rebuttal and a surrebuttal moves the case through the two
single-sided merits phases.

These phases do not wait for a second filing.  One plaintiff rebuttal opens
surrebuttal.  One defendant surrebuttal opens closings.
-/
theorem filed_rebuttal_and_surrebuttal_advance_the_case :
    statePhase afterPlaintiffRebuttal = "surrebuttals" ∧
      statePhase afterDefendantSurrebuttal = "closings" := by
  native_decide

/--
Passing in rebuttal or surrebuttal also advances the case.

The optional phases still need an exact procedural meaning when a side declines
to file.  A pass is the action that closes the current optional phase and opens
the next one.
-/
theorem pass_actions_advance_optional_merits_phases :
    statePhase afterPassedRebuttal = "surrebuttals" ∧
      statePhase afterPassedSurrebuttal = "closings" := by
  native_decide

/--
The second closing ends the attorney sequence and opens deliberation.

This theorem states the point where the engine stops generating attorney
opportunities.  After both closings, the case enters deliberation and the next
opportunity is a deliberation opportunity.
-/
theorem second_closing_opens_deliberation :
    statePhase afterTwoClosings = "deliberation" ∧
      nextOpportunityPhaseAfter afterTwoClosings = "deliberation" := by
  native_decide
