# Rule 51 Judge Eval Plan

## Scope

This eval measures the judge's settlement of jury instructions under Rule 51.  The first version targets `settle_jury_instructions`, because that tool produces a compact summary that can be scored deterministically.  A later eval should target `deliver_jury_instructions` after the project has a stricter full-charge scorer.

The eval constructs a jury-charge ADC state with party proposals, objections, an evidence summary, completed closings, and a jury configuration.  It obtains the real Lean judge opportunity and applies the model's `settle_jury_instructions` tool call back through Lean.  Each result records the fixture, constructed state, role view, opportunity, prompt input, raw response, extracted summary, deterministic score, and Lean acceptance.

## Fixture Set

The first fixture file contains 16 rows across three difficulty tiers.  The rows test neutral burden language, burden shifting, claim-element completeness, argumentative wording, assumptions of disputed fact, excluded evidence, damages, credibility, limiting-purpose evidence, adverse inference, authenticated electronic evidence, sympathy, and a neutral complete charge summary.  The scorer checks whether required concepts appear and whether prohibited final-charge language appears.

| Category | Rows | Scored Boundary |
|---|---:|---|
| Burden standard and burden shifting | 3 | Correct civil burden and no burden shift |
| Claim elements | 2 | Contract, breach, causation, and damages coverage |
| Argumentative or fact-assuming wording | 2 | Neutral wording and conditional fact language |
| Excluded or limited-purpose evidence | 2 | Record-only and limiting-purpose treatment |
| Damages and credibility | 2 | Compensatory damages and jury credibility role |
| Adverse inference | 2 | Permissive inference or no inference, depending on predicate |
| Digital evidence and sympathy | 2 | Neutral treatment of authenticated records and no sympathy bias |
| Neutral complete charge | 1 | Burden, damages, and neutral final wording |

## Scoring

The scorer requires exactly one `settle_jury_instructions` tool call with a nonempty `summary`.  It marks a row correct when all required terms or accepted equivalents appear and no prohibited term appears in an unrejected, unnegated context.  The prohibited-term scorer ignores bad language when the summary quotes it only to sustain an objection, reject a proposal, deny an instruction, or state that the jury must not consider it.

The summary output reports total accuracy, weighted accuracy, invalid rate, missing-required rate, prohibited-inclusion rate, and slices by reason tag, issue family, and tier.  Reason tags are matched against the settled summary text rather than a separate explanation field, because `settle_jury_instructions` has only one substantive payload field.  The CLI includes `--rescore-results` so deterministic scoring corrections can be applied to existing live result files.

## Prompt Iteration

Candidate v1 gives the judge explicit fixture context and general rules for neutral settlement, while withholding the expected required and prohibited terms so the scorer still tests the judgment rather than recall.  It exists to measure whether instruction-specific guidance improves settlement summaries.  Measured results are in [Rule 51 Analysis](analysis.md).

## Next Extensions

The next Rule 51 set should add harder rows before changing production prompt text.  Useful additions include instructions that misuse verdict thresholds, instructions that embed excluded settlement communications, instructions that misstate mitigation, and limiting instructions whose rejected language appears close to the final instruction summary.  A `deliver_jury_instructions` eval should then score full charge text for required sections, prohibited language, neutral tone, and consistency with the settled summary.
