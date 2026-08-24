# Rule 52 Judge Eval Plan

## Scope

This eval measures the judge's bench-trial opinion under Rule 52.  The current ADC posture offers `file_bench_opinion` at `status=trial`, `trial_mode=bench`, and `phase=verdict_return`, so the first eval scores the filed opinion text rather than a multi-step sequence of separate findings and conclusions.  The runner still checks the Rule 52 concerns that motivated the plan: findings grounded in admitted evidence, conclusions tied to the claim elements, exclusion of unadmitted proof, and judgment consistent with the record.

The eval constructs a completed bench-trial ADC state with complaint and answer text, opening theories, admitted bench evidence, excluded evidence when applicable, rests, and closing arguments.  It obtains the real Lean judge opportunity and applies the model's `file_bench_opinion` tool call back through Lean.  Each result records the fixture, constructed state, role view, opportunity, prompt input, raw response, extracted opinion text, deterministic score, and Lean acceptance.

## Fixture Set

The first fixture file contains 16 rows across three difficulty tiers.  The rows test clean plaintiff and defense judgments, contract formation, credibility, causation, excluded evidence, damages proof, damages limitation, authentication, agency authority, and contractual notice.  The set is balanced between plaintiff and defendant judgments so the scorer can separate wrong winner selection from missing reasoning detail.

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

The scorer requires exactly one `file_bench_opinion` tool call with nonempty `text`.  It detects the judgment winner, checks any expected damages amount, requires concepts tied to the admitted record, rejects prohibited reliance on excluded or unsupported proof, and requires the opinion to contain findings, conclusions, and judgment language.  It also matches reason tags such as `breach_proved`, `causation_gap`, `damages_limited`, `excluded_evidence`, `authentication`, `agency`, `notice`, `credibility`, and `fact_law_separation`.

The scorer includes `--rescore-results` because bench opinions use varied but valid legal language.  The first live runs exposed deterministic scorer strictness rather than wrong decisions: opinions used phrases such as `additional freight charges`, `audit log AL-3 was authenticated`, and `did not meet the contractual written-notice requirement`.  The final scorer accepts those equivalent formulations while still rejecting wrong winners, wrong amounts, invalid payloads, prohibited proof, and opinions without Rule 52 section separation.

## Prompt Iteration

Candidate v1 gives the judge explicit fixture context and asks for labeled `Findings of Fact`, `Conclusions of Law`, and `Judgment` sections.  It states record-confinement rules for excluded evidence, lawyer argument, failed elements, proved damages, and damages limited to direct losses.  Measured results are in [Rule 52 Analysis](analysis.md).

## Next Extensions

The next Rule 52 set should add harder rows before any production prompt change.  Useful additions include multiple claims with mixed winners, counterclaims, equitable relief, nominal damages, adverse inference from evidence loss, witness impeachment that affects only one issue, and a bench opinion that must explain why an admitted exhibit receives little weight.  A later runner can test multi-step findings and conclusions if the Lean opportunity sequence exposes `add_bench_finding` and `add_bench_conclusion` before `file_bench_opinion`.
