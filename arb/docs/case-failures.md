# AAR Case Failures

## Ownership

The AAR process owns the case.  It owns the current opportunity, phase, deadline, remaining attempts, evidence state, filings, council state, votes, events, final result, and failure state.  External clients act through the case-owned HTTP APIs, while the case process determines every arbitration consequence.

An opportunity is the unit that can fail because a participant did not act correctly.  The case process detects deadline expiration and exhausted invalid-attempt budgets, while an external client can report its own failure through the applicable API.  AAR records these procedural facts in case state rather than treating them as process faults.

## Rules

A lawyer opportunity failure fails the case.  This applies to plaintiff and defendant opportunities.  When the lawyer misses the deadline or exhausts attempts, AAR records the failed opportunity and sets the case to a terminal failed state.

A council-member opportunity failure fails that member, not the whole case.  When a council member misses the deadline, exhausts attempts, exits before completing the active opportunity, or exceeds the supervised output byte limit, AAR records the failed opportunity, marks or removes that council member with failed status, and continues the case if the arbitration rules allow it.  That member's role API should report `status: "failed"` after the member can no longer act.

System failure is different from participant failure.  Storage failure, invalid internal state, Lean execution failure, API startup failure, and similar faults mean the process could not run AAR correctly.  Those failures should stop the process or put the case into a system-failed state, depending on where the failure occurs.

## Lean Engine

Go detects opportunity failure.  The role API owns deadlines, invalid-attempt counting, stale opportunity checks, role checks, request-size limits, and tool-argument validation.  When a deadline expires or attempts reach zero, Go sends a procedural transition to Lean with the opportunity id, role, phase, failure reason, and supporting details.

Lean owns the case-state transition.  For a lawyer failure, Lean should accept an action that records the failed opportunity and moves the case to `status: "failed"`.  For a council-member failure, Lean should accept an action that records the failed opportunity, marks or removes the council member with failed status, and returns the next state.

Lean rejection of a valid procedural failure request is a system error.  The Go process should not invent a fallback state after Lean rejects the transition.  The process should report the engine rejection as a process/runtime failure because AAR could not advance the case under its rules.

## API Reporting

Invalid tool calls that still have attempts left should return `ok: false`, a precise error object, the active turn, remaining time, and remaining attempts.  The opportunity remains active in that case.  The client should be able to retry without asking any other endpoint what changed.

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
