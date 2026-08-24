# Rule 56 Judge Eval Analysis

## Scope

The fixture file contains 30 Rule 56 motions, with ten rows in each of three difficulty tiers.  Expected dispositions comprise 17 denials, seven grants, and six partial grants, and the moving party is the defendant in 24 rows and the plaintiff in six.  The issue families cover undisputed records, credibility disputes, competing inferences, authentication disputes, missing elements, unsupported damages, movant burden failures, legal bars, and partial relief.

Each fixture becomes a pretrial ADC state whose docket contains the motion, asserted undisputed facts, evidence references, opposition, and any reply.  Lean supplies the current judge opportunity for `motion_index` 0, and the runner executes the judge turn through the ordinary ADC prompt and tool path.  The accepted decision is applied through Lean before the scorer counts the row as correct.

## Decision Boundary

The fixtures distinguish the motion's requested scope from the full case.  A motion directed only to a damages category, claim element, defense, or other severable issue may warrant `partial`, while a record-wide dispute on the requested material issue warrants `denied`.  A full grant requires an undisputed record for the entire scope of relief requested.

Two rows make the partial-disposition boundary explicit.  Fixture `r56-009` seeks judgment only on consequential lost profits and expects `partial` because direct damages and liability remain, while fixture `r56-027` quotes one deposition answer and expects `denied` because the full testimony supports a competing inference on reliance.  The remaining fixtures apply the same distinction to claims, defenses, elements, damages categories, authentication, credibility, and legal bars.

## Scoring Boundary

A valid response contains exactly one `decide_rule56_motion` call for `motion_index` 0, a disposition of `granted`, `denied`, or `partial`, and nonempty reasoning.  The returned disposition must equal the fixture expectation, and the normalized `surviving_issues` set must match the expected set exactly.  Lean acceptance, successful execution, and the absence of a procedural execution error are also required for a correct row.

Reason tags are matched against the reasoning text and reported separately from disposition correctness.  The aggregate summary classifies incorrect outcomes as false grants, false denials, or partial mismatches and reports severity-weighted accuracy.  It also provides slices by reason tag, issue family, tier, and moving party.

## Prompt Coverage

[Candidate v1](prompts/candidate-v1.md) emphasizes the requested motion scope and the use of `partial` for narrower supported relief.  [Candidate v2](prompts/candidate-v2.md) contains the same scope rule and adds an explicit distinction between a severable legal issue and an evidentiary fragment considered against the full record.  Expected dispositions, surviving issues, reason tags, severity, and context notes remain scorer inputs.

## Limits

The fixture scope is one contract claim decided under Rule 56.  A deterministic textual matcher evaluates reason tags.  The saved result contains the complete state, judge view, opportunity, prompt input, response exchanges, final state, extracted payload, and component scores so a disposition or surviving-issues mismatch can be inspected directly.
