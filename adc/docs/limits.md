# Local Rules Limits Guide

This document proposes court-configurable procedural limits for this system.  The goal is disciplined proceedings, predictable resource use, and fair opportunity to be heard without turning limits into an arbitrary barrier to merits adjudication.

The frame is FRCP-compatible local practice.  FRCP Rule 83 supports local rules and case-specific orders.  The system should treat limits as configurable local rules applied per case, with transparent waivers and explicit judicial overrides.

## Design principles

Limits should preserve adversarial fairness and reduce procedural noise.  They should not pre-decide merits.  A good limit reduces abuse while still letting parties develop a complete record for dispositive motions and trial.

Limits should be explicit, machine-checkable, and visible to all parties at case start or by later court order.  Every enforcement action should generate a traceable rule citation and a concrete violation reason.

Limits should distinguish hard caps from default caps.  Hard caps are strict unless modified by order.  Default caps can be exceeded by leave of court.  This is closer to actual local practice and reduces needless sanctions disputes.

## Limit taxonomy

| Category | What is constrained | Typical risk addressed |
|---|---|---|
| Text volume | word, character, or page limits | prolix briefing, prompt stuffing, latency/cost spikes |
| Filing counts | motions, requests, objections, amendments | serial motion practice, harassment, docket clutter |
| Evidence volume | number and size of exhibits/files | document dumps, low-signal records |
| Discovery volume | interrogatories, RFPs, RFAs, disclosures | disproportional discovery burden |
| Timing | deadlines, response windows, extension caps | delay tactics, deadline games |
| Hearing/trial cadence | opening/closing duration, witness/exhibit slots | runaway trial phases |
| Retry behavior | repeated rejected tool calls or invalid filings | denial-of-service style loops |

## Recommended first-wave limits

These are the highest-value limits for this codebase now.  They map directly to flows already implemented.

| Area | Limit | Recommended default | Type |
|---|---|---|---|
| Opening statement | max chars per side | 6,000 | hard cap |
| Closing argument | max chars per side | 8,000 | hard cap |
| Trial theory statement | max chars per side | 4,000 | hard cap |
| Rule 12 motion summary | max chars | 6,000 | default cap |
| Rule 56 motion summary + SUMF text | max chars | 10,000 | default cap |
| Rule 12/56 replies | max chars | 4,000 | default cap |
| Motions per side, per phase | count | 1 dispositive motion track unless leave granted | default cap |
| Interrogatories per set | count | 25 | hard cap |
| RFP requests per set | count | 40 | default cap |
| RFA requests per set | count | 40 | default cap |
| Evidence exhibits per side at trial | count | 40 | default cap |
| Uploaded file size | bytes per file | 25 MB | hard cap |
| Total produced file volume | bytes per side | 500 MB | default cap |
| Trial objections per side per phase | count | 30 | default cap |
| Invalid action retries in a turn | count | 2 | hard cap |

The numeric defaults should be treated as starting points, not doctrine.  They are intended to keep live runs tractable while preserving realistic litigation behavior.

## Discovery-specific limits

Discovery is where proportionality failures appear first.  Interrogatories already have a well-known federal baseline, so a hard cap at 25 is a natural anchor.  RFP and RFA counts should be defaults with judicial adjustment, because complex commercial cases legitimately exceed simple fixed numbers.

Initial disclosures should enforce completeness fields rather than volume limits.  The higher-value control is required structure: witness list, document categories, damages computation summary, and insurance disclosure marker if applicable.

Requests and responses should enforce one-to-one cardinality where required.  If a set has N requests, the response set should have N response slots, with each slot designated as admit, deny, produce, object, or partially comply where applicable.

## Motion practice limits

A single dispositive-motion track per side before trial should be the default.  Serial Rule 12 or Rule 56 filings without changed circumstances are usually abusive in this environment and create low-signal churn.

If a party seeks additional motions, the court should require a leave motion with a narrow statement of new basis.  The leave decision should be explicit and docketed so later enforcement is deterministic.

For Rule 11, limit repeated safe-harbor notices on the same target filing without new grounds.  This prevents harassment cycles while preserving legitimate sanctions practice.

## Trial-phase limits

Trial needs limits that preserve courtroom sequence integrity.  The useful limits are per-side statement length, exhibit count, and objection count, plus strict phase gating.  Counting limits should reset by phase only where that matches real procedure.

Jury-facing content should remain concise and structured.  Length limits on closings and trial theories are practical controls against non-merits verbosity.  They also support consistent juror role prompts and transcript quality.

## Timing and deadline limits

Timing limits should be represented as explicit intervals in case policy, not implicit by runtime clock behavior.  The system should validate deadlines using recorded filing timestamps and policy windows.

Useful timing controls now include response windows for Rule 12 oppositions/replies, Rule 56 oppositions/replies, discovery response windows, and extension-request caps per side.  Extension requests should require reason text and either consent flag or judicial decision.

A practical model is one stipulated extension per item up to a short cap, with further extensions requiring judicial findings.  This mirrors ordinary scheduling-order practice while keeping automation deterministic.

## Sanctions and consequences model

Limit violations should not default to immediate merits-preclusive sanctions.  Start with graduated consequences.

| Violation severity | Typical response |
|---|---|
| Minor first violation | reject filing/action with reason; allow corrected resubmission |
| Repeated technical violation | strike noncompliant filing and require leave to refile |
| Repeated bad-faith violation | monetary or procedural sanction per judge order |
| Phase integrity violation | reject action; preserve phase state unchanged |

Every enforcement should record: violated rule id, measured value, allowed value, actor, timestamp, and remedy applied.

## Waivers, stipulations, and judge overrides

Local-rule limits need explicit escape valves.  Parties should be able to stipulate to certain extensions or count increases where court approval is not mandatory.  The judge should be able to override any limit by order, with optional one-time or case-wide scope.

Overrides must be data, not hidden behavior.  A limit change should be represented as a docketed policy amendment that states old value, new value, reason, scope, and effective interval.

## Anti-gaming controls

Without anti-gaming controls, numeric limits can be bypassed by fragmentation tactics.  The system should define anti-fragmentation semantics before implementation.

A filing should count by substantive unit, not by transport chunk.  Multiple filings within a short window that are materially one brief should be merge-counted unless the court orders otherwise.  Likewise, duplicate exhibits with minor renaming should count once for quota purposes unless there is a valid evidentiary reason.

## Observability and audit requirements

Limit enforcement is only credible if observable.  The system should emit structured events for every accepted, rejected, and overridden limit check.

At minimum, logs and case state should allow a reviewer to answer: which rule applied, what value was measured, why it passed or failed, and who authorized any deviation.

## Suggested implementation order

| Stage | Scope | Why this order |
|---|---|---|
| Stage 1 | text-length limits for trial statements and dispositive filings | lowest complexity, immediate quality gain |
| Stage 2 | discovery cardinality limits and response matching | high procedural value, deterministic checks |
| Stage 3 | motion-count caps and leave-to-file workflow | reduces abuse loops and docket churn |
| Stage 4 | exhibit/file volume caps and anti-duplication counting | controls resource abuse and transcript sprawl |
| Stage 5 | deadline windows and extension policy | highest realism, needs careful timestamp policy |

## Open design questions for discussion

| Question | Options to evaluate |
|---|---|
| Unit for text limits | chars only, words only, or both |
| Scope of motion caps | per side per case, per side per phase, or per claim |
| Exhibit cap scope | per side total or per claim |
| Deadline source | case filed date anchors vs. per-event anchors |
| Override authority | judge only vs. stipulated + judge ratification |
| Enforcement default | reject-and-correct vs. auto-strike after threshold |

## Immediate recommendation

Adopt a conservative local-rules policy with clear defaults, not aggressive hard caps.  Start by enforcing text limits, discovery cardinality, and one dispositive-motion track per side with leave-to-file for extras.  Keep override paths explicit and docketed.

This yields high control with low risk of distorting merits adjudication.

## V1 policy schema

This section defines the concrete configuration shape for initial implementation.

```json
{
  "policy_version": "1.0",
  "effective_on": "2026-01-01",
  "limits": {
    "text": {
      "opening_chars_per_side": {"value": 6000, "kind": "hard"},
      "closing_chars_per_side": {"value": 8000, "kind": "hard"},
      "trial_theory_chars_per_side": {"value": 4000, "kind": "hard"},
      "rule12_summary_chars": {"value": 6000, "kind": "default"},
      "rule56_summary_chars": {"value": 10000, "kind": "default"},
      "rule56_reply_chars": {"value": 4000, "kind": "default"}
    },
    "discovery": {
      "interrogatories_per_set": {"value": 5, "kind": "hard"},
      "rfp_requests_per_set": {"value": 40, "kind": "default"},
      "rfa_requests_per_set": {"value": 40, "kind": "default"}
    },
    "motions": {
      "dispositive_motions_per_side_pretrial": {"value": 1, "kind": "default"}
    },
    "runtime": {
      "invalid_actions_per_turn": {"value": 2, "kind": "hard"}
    }
  },
  "overrides": []
}
```

### Schema notes

`kind` determines default enforcement posture:

- `hard`: reject when exceeded unless an explicit override exists.
- `default`: reject when exceeded unless an explicit override exists.

In v1, `hard` and `default` have the same runtime behavior.  The distinction is policy intent and reporting semantics.  It allows later refinement without schema breakage.

Each constrained action must map to one `limit_key` and one `measured_unit`.  Example: `deliver_closing_argument` maps to `text.closing_chars_per_side` and measures UTF-8 character count.

## V1 override record

Overrides are case-scoped records in policy state and should be docketed by linked order.

```json
{
  "override_id": "ovr-0001",
  "limit_key": "motions.dispositive_motions_per_side_pretrial",
  "scope": {
    "case_id": "2026-08-19-0001",
    "party": "defendant",
    "phase": "pretrial"
  },
  "new_value": 2,
  "reason": "Leave granted for newly discovered schedule evidence.",
  "ordered_by": "judge",
  "ordered_on": "2026-10-02T15:00:00Z",
  "expires_on": null
}
```

Override semantics:

1. Overrides are additive and most-specific scope wins.
2. If two overrides are equally specific, the latest `ordered_on` wins.
3. Expired overrides are ignored.

## V1 violation result shape

Every rejected action on limit grounds should return a structured error with machine-stable fields.

```json
{
  "error_code": "LOCAL_RULE_LIMIT_EXCEEDED",
  "limit_key": "text.closing_chars_per_side",
  "measured_value": 9124,
  "allowed_value": 8000,
  "measured_unit": "chars",
  "actor": "plaintiff",
  "case_id": "2026-08-19-0001",
  "action": "deliver_closing_argument",
  "detail": "closing argument exceeds local rule cap"
}
```

## V1 error codes

| Code | Meaning | Typical action |
|---|---|---|
| `LOCAL_RULE_LIMIT_EXCEEDED` | value exceeded and no valid override | reject action |
| `LOCAL_RULE_INVALID_OVERRIDE_SCOPE` | override scope malformed or unsupported | reject override request |
| `LOCAL_RULE_OVERRIDE_NOT_FOUND` | referenced override id missing | reject mutation |
| `LOCAL_RULE_POLICY_MISSING` | required policy key absent | fail fast and log config error |
| `LOCAL_RULE_UNIT_MISMATCH` | measured unit does not match limit unit | fail fast and log bug |

## Enforcement matrix

| Action | Limit key | Unit | Counter scope |
|---|---|---|---|
| `record_opening_statement` | `text.opening_chars_per_side` | chars | case + party |
| `deliver_closing_argument` | `text.closing_chars_per_side` | chars | case + party |
| `submit_trial_theory` | `text.trial_theory_chars_per_side` | chars | case + party |
| `file_rule12_motion` | `text.rule12_summary_chars` | chars | each filing |
| `file_rule56_motion` | `text.rule56_summary_chars` | chars | each filing |
| `reply_rule56_motion` | `text.rule56_reply_chars` | chars | each filing |
| `serve_interrogatories` | `discovery.interrogatories_per_set` | count | each set |
| `serve_request_for_production` | `discovery.rfp_requests_per_set` | count | each set |
| `serve_request_for_admission` | `discovery.rfa_requests_per_set` | count | each set |
| `file_rule12_motion` and `file_rule56_motion` | `motions.dispositive_motions_per_side_pretrial` | count | case + party + pretrial |
| any tool action in a role turn | `runtime.invalid_actions_per_turn` | count | turn |

## Implementation constraint

Do not infer limits from prompts.  Enforce only from structured arguments and state.  Prompt text can guide agents but cannot serve as legal data.

## Test requirements for v1

Minimum required test coverage before release:

1. one passing and one failing test for each enforced limit key;
2. override acceptance and override expiration behavior;
3. precedence resolution when multiple overrides apply;
4. deterministic error payload assertions for every error code;
5. live-scenario regression runs for at least one trial-heavy and one discovery-heavy scenario.
