# Rule 60 Judge Eval Analysis

## Scope

The fixture file contains 16 judgment-entered states with a pending Rule 60 motion and opposition.  The set divides evenly between eight expected grants and eight expected denials, and three rows arise from default judgments.  Four rows are tier 1, six are tier 2, and six are tier 3.

The issue families cover clerical mistake, excusable neglect, newly discovered evidence, void and satisfied judgments, fraud, diligence, timeliness, prospective inequity, extraordinary circumstances, settlement, buyer's remorse, and ordinary merits reargument.  Each state records the judgment, motion ground, motion text, opposition, and traces for judgment entry and the Rule 60 filing.  Default-judgment fixtures also include a default-judgment docket entry and decision trace.

## Decision Boundary

Grant fixtures require a recognized Rule 60 ground supported by the motion record.  The newly discovered evidence row states diligence, unavailability, materiality, and a likely effect on the judgment, while the fraud and voidness rows distinguish operative defects from unsupported accusations.  Denial fixtures cover known evidence, impeachment-only material, ordinary legal error or credibility reargument, missed time limits, and regret without extraordinary circumstances.

The suite also distinguishes relief under Rule 60 from other procedural devices.  Full payment and a post-judgment settlement followed by full payment support relief based on satisfaction, while regret over a consent judgment does not.  Attempts to substitute Rule 60 for Rule 59 or appeal are denied.  The tool resolves the pending motion and leaves damages and the entered judgment unchanged.

## Scoring Boundary

A scored response contains exactly one `resolve_rule60_motion` call with `motion_index: 0` and a nonempty `relief_summary`.  The scorer reads a Boolean `granted` value but treats a missing or non-Boolean value as false instead of marking the payload invalid.  A malformed denial payload can therefore pass the grant component when its other fields and execution pass.  Correctness also requires every required concept, no affirmative prohibited concept, at least one expected reason tag, Lean acceptance, and successful execution.

The prohibited-concept check is sensitive to negation.  A denial may state that the movant showed `no extraordinary circumstances` or `does not show fraud` without being treated as affirmative reliance on that ground.  The scorer maps each configured concept to its accepted alternative phrases before matching.

The aggregate summary reports accuracy, weighted accuracy, false-grant and false-denial rates, invalid rate, rejection counters, component counts, and slices by reason tag, issue family, tier, and expected disposition.  The `lean_rejected` and `step_rejected` counters include only rows without an `invalid_reason`.  An uncorrected decision or step rejection becomes invalid and does not increment those counters.  A nonprocedural execution error aborts the run.  The per-row result preserves the attempted acceptance fields, complete state, judge view, opportunity, prompt input, response exchanges, final state, provider accounting, payload, and scoring details.

## Limits

The runner requires one Rule 60 opportunity for `motion_index` 0.  The fixture scope covers a single motion seeking grant or denial of relief.  The scorer's deterministic concept and reason checks depend on configured phrases and aliases, so per-row output identifies legally equivalent formulations that fall outside those forms.
