# Rule 51 Judge Eval Plan

## Procedure

This eval measures the judge's settlement of jury instructions through `settle_jury_instructions`.  For each fixture, the runner constructs a jury-trial state in the `jury_charge` phase and asks Lean for the current judge opportunity.  It executes the judge turn through the ADC prompt and tool path, then checks both the returned summary and the resulting Lean state transition.

The constructed docket contains the claim, trial-evidence summary, both proposed instructions, any objections, and completed closing arguments.  The state records the corresponding proposal, objection, and closing decision traces and supplies six sworn jurors.  Each result retains the constructed state, judge view, opportunity, prompt input, response exchanges, provider accounting, extracted payload, final state, and deterministic scoring details.

## Fixture Set

The fixture file contains 16 rows across three difficulty tiers.  Each row identifies its legal issue, proposals, objections, evidence context, required and prohibited concepts, reason tags, and severity weight.  The set tests neutral burden language, burden shifting, claim-element completeness, argumentative wording, disputed facts, evidence restrictions, damages, credibility, adverse inferences, authenticated electronic evidence, sympathy, and complete neutral settlement summaries.

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

The scorer requires exactly one `settle_jury_instructions` tool call with a nonempty `summary`.  It marks the substance correct when all required terms or accepted equivalents appear and no prohibited term appears in an affirmative, unrejected context.  Lean acceptance, successful action execution, and the absence of a procedural execution error are also required for a correct row.

The prohibited-term check excludes quoted language when nearby text sustains an objection, rejects or refuses the proposal, denies the instruction, or directs the jury not to consider the language.  Reason tags are matched against the summary and reported separately from substantive correctness.  The aggregate output reports accuracy, severity-weighted accuracy, invalid rate, missing-required rate, prohibited-inclusion rate, and slices by reason tag, issue family, and tier.

## Prompt Selection

The runner uses the objective supplied by the Lean opportunity by default.  [Candidate v1](prompts/candidate-v1.md) supplies the claim, evidence, proposals, and objections and states general settlement rules, while the expected terms and reason tags remain scorer inputs.  The template renderer supports the opportunity fields and fixture fields defined in `renderJudgeRule51PromptTemplate`, and it rejects unknown placeholders.

## Outputs

The runner writes one JSON object per fixture to `results.jsonl` and an aggregate `summary.json` in the selected output directory.  The `--limit` option selects the first specified number of fixture rows, while model, court, timeout, temperature, prompt catalog, and Lean engine options control execution.  The `--rescore-results` option uses saved Rule 51 result data to write a newly scored result file and summary.
