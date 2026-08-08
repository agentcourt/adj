# AAR Case-Failure Testing

## Scope

These tests cover the behavior specified in [AAR Process And HTTP Specification](aar-spec.md).  Process tests start `aar case`, interact through its private Role APIs, inspect its output, and check its exit status.  Focused API and proceeding tests exercise council-member failure without a service or Clerk process.

The tests distinguish participant failure from process failure and verify that output artifacts preserve the same facts reported through HTTP and stdout.  Lawyer failure ends the case while allowing `aar case` to exit `0`.  Council-member failure records the member failure and permits the proceeding to continue when the council rules allow it.

## Test Setup

A process test starts `.bin/aar case` with `--caseapi-addr 127.0.0.1:0`, reads stderr until it finds `caseapi listening on ...`, and appends `/lawyerapi/v1` or `/councilapi/v1` for Role API calls.  The fixture records stdout, stderr, request and response JSON, and the output directory in a temporary directory.  A failed test retains that directory and prints its path for inspection.

The fixture uses a small complaint, short bounded timeouts, and a one-attempt budget where an invalid request must end an opportunity.  A fake model endpoint supplies deterministic council candidates without network access.  Direct API tests call the HTTP handlers over `httptest` and inspect the resulting case state and events.

## Test Matrix

| ID | Level | Case | Required Result |
| --- | --- | --- | --- |
| LF-1 | Process | Lawyer exhausts attempts | Child exits `0`; stdout and `run.json` report a failed case with reason `attempts_exhausted`; events contain `opportunity_failed`. |
| LF-2 | Process | Lawyer deadline expires | Child exits `0`; stdout and `run.json` report a failed case with reason `deadline_expired`; events contain `opportunity_failed`. |
| CF-1 | Council API | Council member reports agent failure | The active turn completes; the member records `status: "failed"`; the response identifies the member, opportunity, and reason. |
| CF-2 | Council API | Failed member waits again | The response has `status: "failed"`, no mutating tools, and the stored failure object. |
| CF-3 | Proceeding | Council member exhausts response attempts | The member records `attempts_exhausted`; events preserve the invalid-response cause. |
| CF-4 | Proceeding | Council member deadline expires | The member records `deadline_expired`; events preserve the failed opportunity and member removal. |
| RF-1 | Process | Startup input is invalid | Child exits nonzero; stdout reports `status: "error"`; no Role API starts. |

## Lawyer Failure Tests

LF-1 starts `aar case` with a one-attempt budget and waits for the plaintiff turn through `GET /lawyerapi/v1/wait`.  It submits `submit_decision` with the active opportunity id and an invalid opening-statement payload.  The Role API returns `ok: false`, and the case reaches terminal failed status because the invalid-attempt budget is exhausted.

The test waits for the child process and requires exit status `0`.  The final stdout object and `run.json` must report `status: "failed"`, `failure.role: "plaintiff"`, and `failure.reason: "attempts_exhausted"`.  `events.ndjson` must contain `opportunity_failed`.

LF-2 starts the same direct process with a one-second lawyer timeout, observes an active plaintiff turn, and submits no decision.  The process must exit `0` after the deadline.  Stdout, `run.json`, and `events.ndjson` must record `deadline_expired` for the plaintiff opportunity.

## Council-Member Failure Tests

CF-1 calls the direct Council API failure route for the active member and opportunity.  The handler must complete the turn, mark that member failed, and return a structured failure with the recorded reason.  Tests must also reject a failure report from another member or for a stale opportunity without completing the active turn.

CF-2 asks the direct Council API to wait after case state records a failed member.  The response must report failed status, identify the member and reason, and expose no tools.  This response lets an external council client terminate without interpreting the event stream.

CF-3 drives the proceeding's council executor with repeated oversized responses until it exhausts the invalid-attempt budget.  The executor must mark the member failed and record the cause in the failure events.  CF-4 invokes the timeout removal path and requires the same state and event structure with reason `deadline_expired`.

## Runtime-Failure Test

RF-1 starts `aar case` with a missing complaint path.  The process must exit nonzero and write a compact stdout summary with `status: "error"`.  Stderr must not announce a case API listener because startup failed before the process could accept Role API calls.

This test preserves the distinction between runtime failure and participant failure.  Missed deadlines and exhausted participant attempts produce procedural state transitions and terminal case artifacts.  Invalid startup configuration produces a nonzero process exit.

## Verification

The process cases live in `runtime/cmd/aar/blackbox_test.go`; the focused Council API and proceeding cases live under `runtime/proceeding`.  `make build` under `arb/` creates the binaries required by the black-box fixture before `go test ./runtime/...`.  The complete repository test also runs these cases through `go test ./...` from the repository root.

All test inputs remain local and deterministic.  Tests use returned opportunity ids and observe active turns before asserting attempt or deadline behavior.  Network access and live model credentials are outside this test set.
