# Local Rules Limits Guide

ADC stores procedural limits in the scenario's `policy` object and the Lean state's `policy` object.  The [runtime initializer](../runtime/runner/state_init.go) supplies defaults, and the [Lean core](../engine/ADC/Core.lean) enforces limits when it accepts an action.  Model timeouts, response-size limits, and invalid-attempt limits belong to the Go runtime.

## Policy Fields

A scenario can override selected limits with a flat `policy` object:

```json
{
  "policy": {
    "max_opening_chars": 6000,
    "max_closing_chars": 8000,
    "max_interrogatories_per_set": 5,
    "max_dispositive_motions_per_side_pretrial": 2
  }
}
```

This object is a fragment of a scenario passed to `adc scenario --scenario FILE`.  Omitted fields receive defaults.  The [manual](../manual.md#scenario-files) describes the surrounding scenario.

### Limits with Judicial Overrides

Each row maps a policy field to the key used by local-rule overrides and violation messages.  Text lengths use Lean's `String.length`.  Count limits measure the accepted items specified in the last column.

| Policy field | Default | Override key | Measurement |
| --- | ---: | --- | --- |
| `max_opening_chars` | 6000 | `text.opening_chars_per_side` | Opening summary length. |
| `max_trial_theory_chars` | 4000 | `text.trial_theory_chars_per_side` | Trial-theory length. |
| `max_closing_chars` | 8000 | `text.closing_chars_per_side` | Closing argument length. |
| `max_exhibits_per_side` | 20 | `trial.exhibits_offered_per_side` | Exhibits offered by the party. |
| `max_dispositive_motions_per_side_pretrial` | 2 | `motions.dispositive_motions_per_side_pretrial` | Rule 12 and Rule 56 motions by the party. |
| `max_interrogatories_per_set` | 5 | `discovery.interrogatories_per_set` | Interrogatories in one set. |
| `max_interrogatory_sets_per_side` | 2 | `discovery.interrogatory_sets_per_side` | Sets served by the party. |
| `max_rfp_requests_per_set` | 40 | `discovery.rfp_requests_per_set` | Production requests in one set. |
| `max_rfp_sets_per_side` | 2 | `discovery.rfp_sets_per_side` | Production-request sets served by the party. |
| `max_rfa_requests_per_set` | 40 | `discovery.rfa_requests_per_set` | Admission requests in one set. |
| `max_rfa_sets_per_side` | 2 | `discovery.rfa_sets_per_side` | Admission-request sets served by the party. |
| `max_discovery_response_deadline_days` | 30 | `discovery.response_deadline_days` | Elapsed days between supplied `served_on_date` and `responded_at` values. |
| `max_rule12_summary_chars` | 5000 | `text.rule12_summary_chars` | Rule 12 motion summary length. |
| `max_rule56_summary_chars` | 10000 | `text.rule56_summary_chars` | Rule 56 motion summary length. |
| `max_rule56_reply_chars` | 4000 | `text.rule56_reply_chars` | Rule 56 reply summary length. |
| `max_technical_reports_per_side` | 3 | `reports.per_side_count` | Reports submitted by the party. |
| `max_technical_report_summary_chars` | 5000 | `reports.summary_chars_per_report` | One report summary's length. |

The engine checks the proposed count or length against the effective limit before accepting the action.  Successful measured actions update `case.limit_usage` with `limit_key`, `actor`, `phase`, and `value`.  Count checks derive their totals from the case's accepted filings or reports.  The phase field identifies where the check occurred.

Discovery response actions check elapsed days only when both date fields are supplied.  A response date before the service date produces an elapsed value of zero.

### Other Procedural Settings

These fields also belong to `policy`.  Judicial local-rule overrides apply to the keys in the preceding table.

| Policy field | Default | Meaning |
| --- | ---: | --- |
| `max_support_tool_calls_per_opportunity` | 30 | Support-operation budget per opportunity. |
| `max_jury_note_chars` | 3000 | Stored jury-note length setting.  The current engine and runtime do not enforce it. |
| `skip_voir_dire` | 0 | Use voir dire by default.  A value of 1 selects random empanelment. |
| `jury_juror_count` | 6 | Nominal jury size. |
| `jury_unanimous_required` | 1 | Require unanimity by default. |
| `jury_minimum_concurring` | 6 | Nominal minimum concurrence. |
| `voir_dire_candidate_count` | 10 | Initial candidate count. |
| `max_voir_dire_questions_per_side_per_juror` | 1 | Question allowance for each side and candidate. |
| `max_disallowed_voir_dire_questions_per_side` | 3 | Disallowed-question limit per side. |
| `max_for_cause_challenges_per_side` | 1 | For-cause challenge allowance per side. |
| `max_peremptory_challenges_per_side` | 1 | Peremptory challenge allowance per side. |
| `max_deliberation_rounds` | 3 | Deliberation-round limit. |

The [jury guide](juries.md) describes candidate selection, challenges, voting, and juror failure.  The [manual](../manual.md#jury-configuration) describes jury command-line overrides and valid size and concurrence ranges.

## Judicial Overrides

The Lean action `enter_local_rule_override` requires the judge role.  Its payload contains `limit_key`, nonnegative integer `new_value`, `ordered_by`, and `reason`.  Optional fields are `override_id`, `scope_party`, `scope_phase`, and `expires_at`.

```json
{
  "type": "enter_local_rule_override",
  "role": "judge",
  "payload": {
    "limit_key": "text.closing_chars_per_side",
    "new_value": 10000,
    "ordered_by": "judge",
    "reason": "Additional length is needed to address the admitted technical reports.",
    "scope_party": "plaintiff",
    "scope_phase": "closings"
  }
}
```

This is an engine action.  The current model-facing tool schema instead requires `override_value` and omits `ordered_by`.  That mismatch prevents a schema-conforming model payload from satisfying the Lean action's required fields.

An accepted override appends an entry to `case.local_rule_overrides`, a docket entry, and a decision trace.  The stored entry includes the payload fields, `active: true`, and `ordered_at` set to `case.filed_on`.  An omitted or blank identifier becomes `lro-N`, where N is the new entry's position.  Blank optional scope or expiry fields mean unrestricted scope or no expiry.

### Selection and Time

An override applies when its key matches, it is active, its optional party and phase match the action, and its expiry comparison succeeds.  More scoped fields give an override priority: party plus phase outranks either field alone, which outranks an unscoped override.  At equal specificity, the later `ordered_at` string wins.  Equal timestamps select the later entry in the list.

Current limit checks supply `case.filed_on` as the comparison time.  Expiry succeeds when that string is less than or equal to `expires_at`.  This compares stored strings and includes the expiry value.  The result therefore depends on the case date and consistent date formatting.  The action's optional expiry value does not impose a wall-clock deadline on a model process.

## Limit Errors and Runtime Budgets

A measured-limit rejection returns a string in this format:

```text
LOCAL_RULE_LIMIT_EXCEEDED|limit_key=text.closing_chars_per_side|actor=plaintiff|phase=closings|attempted=9124|allowed=8000|detail=closing_argument_chars
```

The fields identify the rule, actor, phase, proposed measurement, effective limit, and action-specific detail.  An unknown lookup key returns `unknown local-rule limit key: KEY`.  The rejected action leaves the prior state unchanged.

The Go [runtime limits](../runtime/runner/runtime_limits.go) apply to model and participant execution:

| Runtime field | Default | Command-line option |
| --- | ---: | --- |
| `llm_timeout_seconds` | 180 | `--timeout-seconds` |
| `roleapi_timeout_seconds` | 480 | `--roleapi-timeout-seconds` |
| `max_response_bytes` | 131072 | `--max-response-bytes` |
| `invalid_attempt_limit` | 3 | `--invalid-attempt-limit` |
| `juror_max_output_tokens` | 4096 | Juror request configuration. |

An invalid participant decision consumes an invalid attempt.  Exhaustion ends an external juror's turn under the [juror-failure rules](juries.md).  Other participant failures return an error to the case loop.  The current opportunity reports its permitted actions, deadline, attempt allowance, and support budget.  Lawyers plan investigation and filing within those values.

Generated scenarios set `loop_policy.max_turns` to 500 and `max_steps_per_turn` to 5.  Reaching the turn limit before the configured stopping condition ends the run with an error.  A supplied scenario can set its own loop limits.  These counts are separate from the per-turn timeouts.
