# Demo lifecycle

Mermaid source: [`demo-lifecycle.mmd`](demo-lifecycle.mmd).

This diagram shows the end-to-end path for the standard example run.  The run starts from `situation.md` and its linked files, regenerates the key material and signature with `sign.sh`, and drafts `complaint.md` with `adc complain`.  `adc case` then prepares and adjudicates the complaint with direct or external roles.

The middle of the diagram shows the runtime path inside `adc case`.  The runner stages the complaint attachments, generates the plaintiff and defense strategies, builds a scenario with `case_init` and no case-specific turns, and asks Lean to initialize the case state.  From there the run enters the `next_opportunity` loop.

The loop shows the decision path for each opportunity.  The runner logs the role, phase, reason, and allowed tools, then obtains a direct model response or waits for an external Role API response.  Lean validates the resulting decision, and an invalid decision returns a correction while preserving the opportunity.  An accepted decision updates the SQLite record and event stream before the runner advances or writes the final result.

This diagram is operational.  It does not try to describe every prompt field, tool call, or formal action family.  It shows the control path of a live demo run.
