# Rule 52 Judge Eval Analysis

## Scope

The fixture file contains 16 completed bench-trial states, balanced between eight plaintiff judgments and eight defense judgments.  Four rows are tier 1, six are tier 2, and six are tier 3.  The issue families cover contract formation, breach, credibility, causation, damages proof, damages limits, excluded evidence, authentication, agency, and contractual notice.

Each state places the pleadings, trial theories, admitted and excluded evidence, rests, and closing arguments in the docket.  Lean receives matching traces for filing, trial mode, exhibits, rests, closings, and advancement to the `verdict_return` phase.  The resulting judge opportunity permits `file_bench_opinion` and the runner executes the accepted action through the ordinary ADC turn path.

## Decision Boundary

The suite scores the opinion's terms and substance separately.  The `verdict_for` payload field supplies the winner, while the opinion text must contain the expected amount when the fixture specifies one, express all required concepts, and contain none of the prohibited concepts or aliases.  A separate check requires the normalized text to contain `finding`, `conclusion`, and `judgment`.

Several fixtures test the evidence boundary.  Party theories and closing arguments appear in the state, but they do not constitute proof, and excluded exhibits may appear in the docket only as excluded material.  The required and prohibited concept lists therefore test whether the opinion grounds its findings in admitted evidence and keeps the judgment within the proved elements and damages.

## Scoring Boundary

A valid response contains exactly one `file_bench_opinion` tool call, a `verdict_for` value of `plaintiff` or `defendant`, and nonempty `text`.  Substantive correctness requires the expected winner, the expected amount, all required concepts, no prohibited concepts, and the three required opinion terms.  Lean acceptance, successful execution, and the absence of a procedural execution error are required before the runner counts the row as correct.

Reason tags provide a separate diagnostic, while the winner, amount, concept, and term-presence checks determine substantive correctness.  The aggregate summary reports accuracy, winner accuracy, severity-weighted accuracy, invalid responses, component counts, and slices by reason tag, issue family, tier, and expected winner.  False-plaintiff and false-defense counts distinguish the direction of an incorrect judgment.

## Limits

The amount and concept checks use deterministic textual forms and configured aliases.  The prohibited-concept matcher treats a literal concept or alias as present even when the opinion negates, quotes, or rejects it.  A legally equivalent expression outside the configured forms can fail a check, and the opinion-term check establishes only that the three normalized words appear somewhere in the text.  The fixture scope is one contract claim resolved through a single `file_bench_opinion` action.
