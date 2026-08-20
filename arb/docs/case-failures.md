# AAR Case Failures

## Ownership

The AAR process owns the case.  It owns the current opportunity, phase, deadline, remaining attempts, evidence state, filings, council state, votes, events, final result, and failure state.  External clients act through the case-owned HTTP APIs, while the case process determines every arbitration consequence.

An opportunity is the unit that can fail because a participant did not act correctly.  The case process detects deadline expiration and exhausted invalid-attempt budgets, while an external client can report its own failure through the applicable API.  AAR records these procedural facts in case state rather than treating them as process faults.

## Rules

A lawyer opportunity failure fails the case.  This applies to plaintiff and defendant opportunities.  When the lawyer misses the deadline or exhausts attempts, AAR records the failed opportunity and sets the case to a terminal failed state.

A council-member opportunity failure fails that member, not the whole case.  When a council member misses the deadline, exhausts attempts, exits before completing the active opportunity, or exceeds the supervised output byte limit, AAR records the failed opportunity, marks or removes that council member with failed status, and continues the case if the arbitration rules allow it.  That member's role API should report `status: "failed"` after the member can no longer act.

System failure is different from participant failure.  Storage failure, invalid internal state, Lean execution failure, API startup failure, and similar faults mean the process could not run AAR correctly.  Those failures should stop the process or put the case into a system-failed state, depending on where the failure occurs.

## Lean Engine

Go detects opportunity failure.  The role API owns deadlines, invalid-attempt counting, stale opportunity checks, role checks, request-size limits, and tool-argument validation.  When a deadline expires or attempts reach zero, Go sends a procedural transition to Lean with the opportunity id, role, phase, failure reason, and supporting details.  Every Lean call has the configured engine-call timeout.

An action call uses the earlier of the opportunity deadline and the engine-call timeout.  If the action reaches the opportunity deadline, AAR discards its response and sends one `deadline_expired` transition through a fresh engine call.  The deadline transition uses the case context and the full engine-call timeout.  An engine timeout before the opportunity deadline is a runtime failure.

Lean owns the case-state transition.  For a lawyer failure, Lean should accept an action that records the failed opportunity and moves the case to `status: "failed"`.  For a council-member failure, Lean should accept an action that records the failed opportunity, marks or removes the council member with failed status, and returns the next state.

Lean rejection of a valid procedural failure request is a system error.  The Go process should not invent a fallback state after Lean rejects the transition.  The process should report the engine rejection as a process/runtime failure because AAR could not advance the case under its rules.

## API Reporting

Invalid tool calls that still have attempts left should return `ok: false`, a precise error object, the active turn, remaining time, and remaining attempts.  The opportunity remains active in that case.  The client should be able to retry without asking any other endpoint what changed.

HTTP parsing, case and role identity, active-turn selection, stale opportunity ids, and deadline checks do not consume an invalid attempt.  After a valid `/do` request resolves to the active opportunity, an unknown tool or invalid tool argument is participant input and consumes one attempt.  The API reports those errors as `tool_failed`, and successful exhaustion completes the procedural failure transition without returning a runtime error through the turn.

The case context owns a parsed and identity-validated POST mutation.  A client disconnect suppresses its response while the mutation continues to completion.  Case cancellation stops an active Lean call and returns a runtime failure to a connected client.

An execution, storage, integrity, publication, manifest, event-recording, or internal-state error consumes no attempt and completes the active turn.  The API reports `runtime_failure`, and the same error passes through the turn result to the case runner.  A Lean execution error, a Lean rejection after Go has validated the action, or an accepted Lean response without a valid next state belongs to this class.

The direct council backend retries malformed provider responses and Go-invalid vote payloads within the member's invalid-attempt budget.  Once Go validates a vote, a Lean execution error or rejection stops the proceeding as a runtime error rather than consuming another provider attempt or failing the member.  This division keeps participant attribution at the input boundary and treats Go/Lean disagreement as a process fault.

Observer tools have no active turn or invalid-attempt budget.  Observer input validation returns `tool_failed`, while evidence storage, hash, or read failure returns `runtime_failure` to that HTTP request.  An Observer runtime failure does not currently terminate the case because the Observer API has no run-level fatal-error channel.

When a lawyer opportunity failure makes the case terminal, every role API reports the failed case.  `get`, `wait`, `status`, and `result` return `status: "failed"` and include the same structured failure object.  An external client can therefore stop without interpreting the event stream.

When a council member fails, that member's API should report `status: "failed"` with the failure object and no mutating tools.  Other council members and lawyers should see the case as running unless AAR has reached a separate terminal rule.  Observers should see the member failure in the case status and events.

## Process Reporting

The `aar case` process should report procedural case failure as a normal terminal case result on stdout and exit `0`.  The stdout object should use `status: "failed"` and include both a short `error` string and a structured `failure` object.  A nonzero process exit should mean the process failed to run AAR correctly, not that a participant failed an opportunity.

The `error` string should be factual and specific.  It should name the role, phase or opportunity id, and failure reason.  The structured `failure` object should carry machine-readable fields for the same fact.

```json
{
  "case_id": "case-123",
  "run_id": "run-case-123",
  "status": "failed",
  "phase": "arguments",
  "error": "Plaintiff lawyer opportunity arguments:plaintiff failed because the deadline expired.",
  "failure": {
    "type": "opportunity_failed",
    "role": "plaintiff",
    "phase": "arguments",
    "opportunity_id": "arguments:plaintiff",
    "reason": "deadline_expired"
  },
  "final_state": {
    "case": {
      "status": "failed",
      "phase": "arguments"
    }
  }
}
```

Council-member failure should not produce a terminal failed case result unless the rules later make the case fail for an independent reason.  The process should record the member failure in events, record the member's failed status in final or current case state, and continue to the next opportunity.  If the case later closes, the final stdout object should describe the arbitration result and include the council-member failure in the events and final state.
