# Design Goals

`arbd` exists to answer a narrow class of questions that `arb` does not fit well.  Those questions ask for a bounded quantitative judgment rather than a binary determination, and the classic example is how much one work reused another.  The design goal is to support that quantitative form without refactoring the sibling procedures or weakening the adversarial structure.

## Bounded Divergence From `arb`

The first goal is to keep `arbd` close to `arb` wherever the binary outcome model is not the issue.  The merits sequence, the council constitution path, the complaint drafting flow, the runtime packaging, and the attorney backend remain recognizably the same.  That bounded divergence keeps the code review surface small, lowers the risk to `arb`, and makes it clear which changes are intrinsic to degree adjudication rather than incidental refactors.

This is why `arbd` keeps the existing council and `member_id` machinery.  The procedure did not need a renamed deciding body or a shared abstraction layer to support degree questions.  It needed a new complaint shape, a new council action, a different closure rule, and a different final evidence.

## Numeric Judgment Rather Than Binary Outcome

The second goal is to make the question itself primary.  The complaint states one question, and the policy states how the council should answer it.  Degree adjudication should therefore remain a numeric process from prompt to state to final evidence, because that is the information the procedure is meant to collect.

This goal affects both prompting and state.  Attorneys are expected to argue for concrete numbers or narrow numeric ranges, and council members are expected to answer with one bounded integer from `0` through `100`.  The Lean state therefore stores numeric answers directly rather than storing labels and reconstructing a number later.

## Independent Member Answers

The third goal is to preserve the full council answer set.  `arbd` leaves aggregation out of scope and records each seated member's answer independently.  The arbitration result is the map from `member_id` to answer.

This design records the procedure's product directly.  When the case is close, disagreement across members is part of the result rather than noise to be hidden by an aggregate.  When the case is easy, the answer map will still show that convergence without the engine needing a second, derived output concept.

## Role-Bound Advocacy

The fourth goal is to keep advocacy role-bound even when the evidence points away from the side's preferred number.  The claimant should still file the strongest truthful high-number case, and the respondent should still file the strongest truthful low-number case.  Degree questions often turn on weighting, discounting, and calibration, so role-bound advocacy needs to show its method instead of assuming a binary outcome frame.

This does not authorize exaggeration.  A side may have to concede that the best surviving case supports a narrower range than it wanted at the outset, and the filing should say so.  What `arbd` needs from advocacy is disciplined position-taking: a concrete number, a method for getting there, and an account of why nearby alternatives fit the record less well.

## Transparent Methods

The fifth goal is methodological transparency.  Degree questions invite hidden weighting choices, vague use of similarity language, and silent discounting of adverse facts.  `arbd` should therefore encourage explicit scoring methods, identified anchors, and clear explanations of what facts moved the advocated number up or down.

This goal appears in the example sonnet case and should remain visible in later examples.  A good `arbd` filing should explain why a score of `82` differs from `92`, not just announce that one of them feels right.  The council can then disagree on the number while still engaging the same recorded method and the same record facts.
