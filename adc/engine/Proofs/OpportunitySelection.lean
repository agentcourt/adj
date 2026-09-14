import Proofs.OrchestrationCore

namespace ADCProofs.OpportunitySelection

open ADCProofs.OrchestrationCore

/--
If every opportunity in `actions` has strictly higher priority than `target`,
then folding the selector from any `current` that also has strictly higher
priority still ends at `target` once `target` is appended.

The proof plan is induction on `actions`.  The selector keeps the
lowest-priority opportunity seen so far.  In the inductive step, either `head`
outranks `current` and becomes the new accumulator or it does not.  In both
branches, the hypotheses say that `target` still outranks the surviving
accumulator and every remaining action, so the induction hypothesis applies to
the tail.
-/
private theorem foldl_select_some_current_append_target
    (actions : List OpportunitySpec)
    (current target : OpportunitySpec)
    (hcur : target.priority < current.priority)
    (hall : ∀ a, a ∈ actions -> target.priority < a.priority) :
    List.foldl
      (fun acc action =>
        match acc with
        | none => some action
        | some current =>
            if action.priority < current.priority then some action else acc)
      (some current)
      (actions ++ [target]) = some target := by
  induction actions generalizing current with
  | nil =>
      simp [hcur]
  | cons head tail ih =>
      have hhead : target.priority < head.priority := hall head (by simp)
      have htail : ∀ a, a ∈ tail -> target.priority < a.priority := by
        intro a ha
        exact hall a (by simp [ha])
      by_cases hlt : head.priority < current.priority
      · simpa [hlt] using ih head hhead htail
      · simpa [hlt] using ih current hcur htail

/--
If the last opportunity in a list has strictly lower priority than every
earlier opportunity, `selectLowestPriorityOpportunity?` returns that last
opportunity.

The proof plan reduces the public selector to the accumulator theorem above.
The empty case is immediate.  In the nonempty case, the head becomes the first
accumulator value, and the helper theorem proves that folding over the tail and
the appended target still ends at `target`.
-/
theorem selectLowestPriorityOpportunity_append_last_if_strictly_lower
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (hlower : ∀ a, a ∈ actions -> target.priority < a.priority) :
    selectLowestPriorityOpportunity? (actions ++ [target]) = some target := by
  cases actions with
  | nil =>
      simp [selectLowestPriorityOpportunity?]
  | cons head tail =>
      have hhead : target.priority < head.priority := hlower head (by simp)
      have htail : ∀ a, a ∈ tail -> target.priority < a.priority := by
        intro a ha
        exact hlower a (by simp [ha])
      unfold selectLowestPriorityOpportunity?
      change
        List.foldl
          (fun (acc : Option OpportunitySpec) (action : OpportunitySpec) =>
            match acc with
            | none => some action
            | some current =>
                if action.priority < current.priority then some action else acc)
          (some head)
          (tail ++ [target]) = some target
      exact foldl_select_some_current_append_target tail head target hhead htail

/--
If `availableOpportunities` ends with a strictly lower-priority opportunity and
the state has no passed opportunities, `currentOpenOpportunity?` returns that
last opportunity.

The proof plan is direct.  With no passed opportunities, `openOpportunities`
reduces to `availableOpportunities`.  The selector theorem above then applies
to the resulting appended list.
-/
theorem currentOpenOpportunity_of_available_append_last_if_no_passes
    (req : OpportunityRequest)
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (havailable : availableOpportunities req = actions ++ [target])
    (hpasses : req.state.passed_opportunities = [])
    (hlower : ∀ a, a ∈ actions -> target.priority < a.priority) :
    currentOpenOpportunity? req = some target := by
  have hopen : openOpportunities req = actions ++ [target] := by
    unfold openOpportunities
    rw [hpasses, havailable]
    simp
  unfold currentOpenOpportunity?
  rw [hopen]
  exact
    selectLowestPriorityOpportunity_append_last_if_strictly_lower actions target hlower

/--
Under the same hypotheses, `nextOpportunity` exposes the same selected target
at the public response boundary and remains non-terminal.

The proof plan uses the previous selector theorem together with the public
boundary lemmas from `OrchestrationCore`.  The opportunity field follows from
`nextOpportunity_opportunity_eq_currentOpenOpportunity`.  The non-terminal
field follows because `nextOpportunity_terminal_iff_no_currentOpenOpportunity`
would otherwise force the current open opportunity to be `none`, contradicting
the previous theorem.
-/
theorem nextOpportunity_of_available_append_last_if_no_passes
    (req : OpportunityRequest)
    (actions : List OpportunitySpec)
    (target : OpportunitySpec)
    (havailable : availableOpportunities req = actions ++ [target])
    (hpasses : req.state.passed_opportunities = [])
    (hlower : ∀ a, a ∈ actions -> target.priority < a.priority) :
    (nextOpportunity req).opportunity = some target ∧
    (nextOpportunity req).terminal = false := by
  have hcurrent :
      currentOpenOpportunity? req = some target :=
    currentOpenOpportunity_of_available_append_last_if_no_passes
      req actions target havailable hpasses hlower
  constructor
  · rw [nextOpportunity_opportunity_eq_currentOpenOpportunity]
    exact hcurrent
  · by_cases hterm : (nextOpportunity req).terminal = true
    · have hnone : currentOpenOpportunity? req = none :=
        (nextOpportunity_terminal_iff_no_currentOpenOpportunity req).mp hterm
      simp [hcurrent] at hnone
    · simp [hterm]

end ADCProofs.OpportunitySelection
