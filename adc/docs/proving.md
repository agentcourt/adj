# Proving Notes

This document records proof-oriented design notes for the current engine.  It
is not a general Lean tutorial.  Use it for theorem targets, proof boundaries,
and deferred proof work that depends on later design changes.

## Opportunity identity

The engine uses deterministic ids derived from opportunity content.  The ids
are stable for the same opportunity content, but they are not formally
injective.  A theorem about exact optional-pass isolation therefore has to
account for possible id collisions.

That distinction controls the proof shape.  The strongest pass-isolation theorem
would say: passing one optional opportunity suppresses exactly that
opportunity and preserves every other distinct opportunity in the same agenda.
The current hashed scheme does not support that theorem cleanly, because the
proof would still leave hash collisions outside the formal boundary.

The deferred replacement is state-local index assignment over the full
`availableOpportunities` list before filtering `passed_opportunities`.  That
design would make ids pairwise distinct within one state, keep them stable
across pass filtering in that same state, and let the proof say exactly what
the engine means.  We are not changing the implementation yet.

## Maintained proof target

`make prove` should cover maintained theorems about the current engine.  The
current proof root imports maintained theorem modules for current behavior.

The current maintained set covers:

- court-profile theorems about jurisdiction screening
- federal-side contrast theorems for defective diversity pleading
- jury-selection theorems about representative empanelment effects
- representative selection-count theorems for for-cause and peremptory removal
- a voir-dire boundary theorem on a representative ready panel
- verdict-derivation theorems about missing ballots, damages averaging,
  defense verdicts, stable-split hung juries, and round advancement
- a judgment-entry theorem on a representative jury-verdict state
- order invariance for a representative plaintiff-majority vote list

One important theorem remains deferred.  The strong pass-isolation theorem
belongs in the maintained set only after opportunity ids become injective
within one state.  Until then, the honest boundary is narrower.
