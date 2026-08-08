# Agents

ADC can handle a procedural role through a direct model call or its case-owned HTTP Role API.  The case process owns Lean state, role-visible views, active opportunities, deadlines, invalid-attempt counts, file access, and final results.  A role receives the current opportunity and can propose one permitted legal act.

## Roles

Plaintiff, defendant, and juror roles can run internally or through the Role API.  Judge and clerk roles remain internal because they perform court-controlled procedural work in the current runtime.  The observer role reads status and final results without receiving authority to act.

| Role | Procedural function |
| --- | --- |
| Plaintiff | Plead, conduct discovery, present evidence, argue, and request relief. |
| Defendant | Answer, raise defenses, conduct discovery, present evidence, and argue. |
| Juror | Answer selection questions and vote during deliberation. |
| Judge | Decide motions, control trial, instruct the jury, and enter judgment. |
| Clerk | Record administrative acts, configure the jury, and advance stages. |
| Observer | Read current status and terminal results. |

## Role API

The Role API lives under `/roleapi/v1` on the address supplied through `--caseapi-addr`.  Every request identifies the case and role, while juror requests also identify the juror principal.  Repeated `--external-role` flags select the roles whose opportunities wait for API decisions.

| Endpoint | Purpose |
| --- | --- |
| `GET /health` | Report listener readiness. |
| `GET` or `POST /roleapi/v1/status` | Return case status and current-turn information. |
| `GET` or `POST /roleapi/v1/get` | Return the caller's opportunity without waiting. |
| `GET` or `POST /roleapi/v1/wait_for_opportunity` | Wait for an opportunity or terminal status. |
| `GET` or `POST /roleapi/v1/result` | Return a final, failed, or pending result. |
| `POST /roleapi/v1/do` | Execute a support operation, submit work notes, or submit a decision. |
| `POST /roleapi/v1/fail` | Report failure for the active opportunity. |

An active response includes the current prompt, role-visible case view, opportunity identity, time remaining, attempts remaining, and support-operation budget.  It also reports the legal tools Lean permits for that turn and the schemas for legal and support operations.  The caller must return the active opportunity id with work notes, decisions, or failures.

## Legal Decisions

Legal acts use the `submit_decision` operation.  A legal-tool decision supplies `kind=tool`, `tool_name`, and `payload`, while an allowed pass supplies `kind=pass` and `reason`.  Lean validates the state version, opportunity id, role, permitted tool set, and payload before accepting the transition.

An invalid decision leaves the same opportunity active while its attempt budget remains.  The response explains the failed validation so that the role can submit a corrected decision.  Exhausting the budget invokes the configured failure behavior for that role.

## Record Access and Work Notes

Support operations expose only material visible to the current role.  They include case reads, decision explanations, case-file listing, bounded file reads, provider file requests, and juror context when applicable.  Lean-backed visibility checks govern these reads independently of prompt construction.

`send_work_notes` stores private planning outside the adjudication record.  Each entry records the case, run, role, optional juror principal, opportunity, and note text in `work-notes.ndjson`.  Facts that affect an adjudicative result must enter the record through a permitted legal act rather than through private notes.

## Failures

Plaintiff or defendant failure ends the case because the active party opportunity cannot continue.  Juror failure follows the procedural replacement and dismissal rules, including candidate replacement during selection when available.  A failed deliberating juror leaves the effective concurrence calculation while the nominal jury policy remains in the record.
