# Rule 52 Judge Eval Plan

## Procedure

This eval measures the judge's bench-trial opinion under Rule 52 through `file_bench_opinion`.  Each fixture constructs a case with `status=trial`, `trial_mode=bench`, and `phase=verdict_return`, then obtains the current judge opportunity from Lean.  The runner executes the judge turn through the ADC prompt and tool path and applies the accepted action to the state.

The docket contains the complaint, answer, both trial theories, admitted and excluded evidence, party rests, and closing arguments.  Decision traces establish the pleadings, bench-trial designation, exhibit treatment, rests, closings, and verdict-return phase.  Each result records the fixture, state, role view, opportunity, prompt input, response exchanges, final state, provider accounting, extracted opinion, and component scores.

## Fixture Set

The fixture file contains 16 rows across three difficulty tiers and divides evenly between expected plaintiff and defense judgments.  Every row supplies pleadings, party theories, admitted evidence, any excluded evidence, closings, the expected winner and amount, required and prohibited concepts, reason tags, and a severity weight.  The legal boundaries include contract formation, breach, credibility, causation, damages proof, damages limitation, authentication, agency authority, and contractual notice.

| Theme | Scored Boundary |
|---|---|
| Contract formation and breach proof | Contract, breach, delivery, nonpayment, and damages support |
| Causation and damages gaps | No award when causation or damages proof fails |
| Excluded or unauthenticated evidence | No reliance on excluded screenshots or text messages |
| Damages limitation | Direct damages only when consequential damages lack support |
| Authentication and technical records | Reliance on admitted authenticated records only |
| Agency and authority | Actual or apparent authority versus independent-contractor limits |
| Contractual notice | Timely notice versus failed notice condition |
| Credibility | Contemporaneous records and explained credibility choices |

## Scoring

The scorer requires exactly one `file_bench_opinion` tool call with a valid `verdict_for` and nonempty `text`.  It checks the payload winner, any expected amount in the text, required concepts tied to admitted proof, prohibited concepts, and the presence of `finding`, `conclusion`, and `judgment` in the normalized text.  Lean must accept and execute the decision, and the execution path must finish without a procedural error.

Reason tags such as `breach_proved`, `causation_gap`, `damages_limited`, `excluded_evidence`, `authentication`, `agency`, `notice`, `credibility`, and `fact_law_separation` are reported separately.  The aggregate summary includes component counts, accuracy, winner accuracy, weighted accuracy, invalid rate, and slices by reason tag, issue family, tier, and expected winner.  The `--rescore-results` option applies these deterministic checks to an existing Rule 52 result file without making model calls.

## Prompt Selection

The runner uses the objective supplied by the Lean opportunity by default.  [Candidate v1](prompts/candidate-v1.md) supplies the pleadings, party theories, evidence, and closings and requires labeled `Findings of Fact`, `Conclusions of Law`, and `Judgment` sections.  The expected winner, amount, concepts, and reason tags remain scorer inputs.

## Outputs

The runner writes one JSON object per fixture to `results.jsonl` and an aggregate `summary.json` in the selected output directory.  The `--limit` option selects the first specified number of fixture rows, while model, court, timeout, temperature, prompt catalog, and Lean engine options control execution.  Each result row includes component scores for winner, amount, required concepts, prohibited concepts, opinion terms, decision acceptance, and step execution.
