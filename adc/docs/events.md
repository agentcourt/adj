# Event Records

ADC writes structured case events to `events.ndjson` and the SQLite `events` table.  External roles can also write private planning to `work-notes.ndjson`.  The final result, state, transcript, and digest provide terminal views of the same run.

## Files

| Record | Shape | Purpose |
| --- | --- | --- |
| `events.ndjson` | JSON lines | Ordered legal actions and selected runtime events. |
| `run.db` | SQLite | Queryable case, action, and event data. |
| `work-notes.ndjson` | JSON lines | Private external-role notes outside the case record. |
| `run.json` | JSON | Terminal machine-readable result. |
| `state.json` | JSON | Terminal Lean state. |
| `transcript.md` | Markdown | Human-readable case transcript. |
| `digest.md` | Markdown | Human-readable case summary. |

## Structured Events

Action events pass through `persistActionEvent` in `runtime/runner/io.go`.  Each event records the run, turn, step, actor role, action type, payload, response, and timestamp.  The action family includes legal acts and record-access operations that the runtime elects to preserve.

Agent events pass through `persistAgentEvent` in the same package.  The Role API uses them for API errors and model-completion results.  Negative step indexes derived from the per-turn sequence distinguish these records from accepted legal-action steps.

Both event families use the same two sinks.  `appendEventLine` appends an object to `events.ndjson`, and `Store.AppendEvent` inserts a row into the SQLite table.  The duplicate representations support sequential review and database queries without changing event meaning.

## Work Notes

`send_work_notes` appends private notes to `work-notes.ndjson`.  A note contains the case, run, role, optional juror principal, opportunity, and complete note text.  Work notes never become a legal act or part of the case state.

The notes can preserve plans, source searches, extraction steps, failed approaches, and turn summaries.  They support later evaluation of an external role's work.  A role must submit relevant evidence or argument through a permitted legal tool before the judge or jury can rely on it.

## Review

`run.json`, `digest.md`, and `transcript.md` provide the terminal outcome and written account.  `events.ndjson` or the SQLite `events` table provides the exact recorded sequence.  `work-notes.ndjson` supplies the private planning record when external roles submitted notes.
