# Rule 51 Judge Eval Analysis

## Coverage

The fixture file contains 16 jury-charge states in three tiers: four tier-1 rows, six tier-2 rows, and six tier-3 rows.  The issue families cover burden standards, burden shifting, claim elements, argumentative language, assumptions of disputed fact, excluded evidence, limiting instructions, damages, credibility, adverse inferences, digital evidence, sympathy, and a complete neutral charge.  Each state supplies competing proposed instructions, any objections, an evidence summary, completed closings, and a six-person jury configuration.

The suite scores the settlement summary produced by `settle_jury_instructions`.  Required concepts include fixture-specific claim elements and limiting rules, while prohibited concepts identify language that would make the settled charge argumentative, unsupported, or legally incorrect.  Complete-charge structure and delivery belong to the separate `deliver_jury_instructions` action.

## Scoring Boundary

A valid response contains exactly one `settle_jury_instructions` tool call with a nonempty `summary`.  A row is substantively correct when the summary contains every required term or accepted equivalent and contains no prohibited term in an affirmative final-instruction context.  The runner also requires Lean to accept and execute the decision before counting the row as correct.

The prohibited-term check examines nearby language so that a ruling may quote a defective proposal while rejecting it.  Context indicating that an objection was sustained or that language was rejected, refused, denied, negated, excluded, or forbidden prevents that quotation from counting as final-charge contamination.  Accepted required-term equivalents include formulations such as `breach caused` for causation, `evidence admitted at trial` for admitted evidence, and permissive language equivalent to `may infer`.

Reason-tag matching is a separate diagnostic.  The tool carries its explanation in `summary`, and the scorer derives tags from that text.  Required- and prohibited-concept checks determine substantive correctness.  The aggregate summary reports accuracy, severity-weighted accuracy, invalid responses, missing required concepts, prohibited inclusions, and slices by reason tag, issue family, and tier.

## Limits

The deterministic checks use configured terms, aliases, and local negation patterns.  A semantically valid formulation outside those aliases can miss a required concept, and an unusual quotation structure can affect prohibited-term classification.  The saved per-row result includes the state, role view, opportunity, prompt input, response exchanges, final state, provider accounting, extracted payload, and individual scoring fields for inspection.
