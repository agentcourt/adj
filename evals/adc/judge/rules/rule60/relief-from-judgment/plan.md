# Rule 60 Judge Eval Plan

## Procedure

This eval measures the judge's resolution of Rule 60 motions through `resolve_rule60_motion`.  Each fixture builds a judgment-entered ADC state with one pending motion and opposition, then obtains the current judge opportunity from Lean.  The runner executes the judge turn through the ADC prompt and tool path and applies the accepted decision through Lean.

The state records a $12,000 judgment, the fixture's judgment description, the Rule 60 ground and motion, the opposition, and decision traces that expose the motion to Lean.  Default-judgment rows also record the default judgment in the docket and decision traces.  Each result retains the fixture, state, role view, opportunity, prompt input, response exchanges, final state, provider accounting, extracted payload, and component scores.

## Fixture Set

The fixture file contains 16 rows across three difficulty tiers and balances eight expected grants against eight expected denials.  Each row supplies the judgment, motion ground, motion and opposition text, expected disposition, required and prohibited concepts, reason tags, severity, and optional default-judgment posture.  The set covers mistake or excusable neglect, void judgment, satisfaction, newly discovered evidence, fraud, timeliness, prospective inequity, extraordinary circumstances, settlements, regret, and ordinary reargument.

| Theme | Scored Boundary |
|---|---|
| Excusable neglect after default | Relief can be granted when neglect is excusable and the party acts promptly |
| Ordinary reargument | Denial when the motion asks the judge to reweigh credibility or evidence |
| Void judgment | Lack of valid service supports relief from a default judgment |
| Satisfied judgment | Full payment or satisfaction supports limited relief |
| Newly discovered evidence | Relief requires evidence unavailable earlier despite diligence and material to the result |
| Fraud or misconduct | Relief for fraud affecting the judgment, and denial for impeachment-only or known issues |
| Timeliness | Rule 60(b)(1) timing and reasonable-time limits can bar relief |
| Extraordinary prospective change | Prospective enforcement can become inequitable after an external legal change |

## Scoring

The scorer accepts exactly one `resolve_rule60_motion` tool call with `motion_index: 0` and a nonempty `relief_summary`.  It reads a Boolean `granted` value but treats a missing or non-Boolean value as false instead of invalid.  It checks the resulting grant or denial, required concepts, prohibited concepts, reason tags, decision acceptance, and action execution.  The summary reports accuracy, weighted accuracy, invalid rate, false-grant and false-denial rates, rejection counters, and slices by issue family, tier, expected disposition, and reason tag.  The `lean_rejected` and `step_rejected` counters exclude rows with an `invalid_reason`.

The prohibited-concept scorer is negation-aware.  Rule 60 denials often state the absent ground, such as "no extraordinary circumstances," "does not show fraud," or a list of grounds that are not shown.  The scorer treats those forms as correct denials rather than assertions of the prohibited ground, while still flagging a summary that relies on an improper ground affirmatively.

## Prompt Selection

The runner uses the objective supplied by the Lean opportunity by default.  [Candidate v1](prompts/candidate-v1.md) supplies the judgment, asserted ground, motion, and opposition and states the recognized grounds and common denial boundaries.  The expected disposition, required and prohibited concepts, reason tags, severity, and context notes remain scorer inputs.

## Outputs

The runner writes one JSON object per fixture to `results.jsonl` and an aggregate `summary.json` in the selected output directory.  The `--limit` option selects the first specified number of fixture rows, while model, court, timeout, temperature, prompt catalog, and Lean engine options control execution.  The `--rescore-results` option uses saved Rule 60 result data to write a newly scored result file and summary.
