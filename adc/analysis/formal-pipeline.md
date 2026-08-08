# Formal pipeline

Mermaid source: [`formal-pipeline.mmd`](formal-pipeline.mmd).

This diagram shows the main entry paths into the formal runtime.  The system accepts three kinds of input: `situation.md`, `complaint.md`, and `scenario.json`.  A situation file does not enter the formal system directly.  `adc complain` first resolves the linked files and drafts a complaint.  `adc case` then loads that complaint and its linked attachments.  A prebuilt scenario bypasses complaint drafting and case generation.

The center of the diagram shows where complaint-driven and scenario-driven execution meet.  `adc case` performs case generation: it produces the case packet, the strategies, and `case_init`, then builds a formal scenario.  If `case_init` is present, Lean initializes the authoritative `CourtState`.  Otherwise the scenario already contains the state.

The lower half shows the steady-state loop.  Lean derives the next opportunity, and a terminal case causes the runner to write the database rows, event log, transcript, and digest.  Otherwise Lean produces a role-scoped view, the runner assembles the prompt, and a direct or external role proposes a decision through a model call or the Role API.  Lean rejects an invalid decision with a correction or returns an accepted act whose execution produces the authoritative state for the next opportunity.

This diagram is narrower than the demo lifecycle diagram.  It focuses on how inputs become a formal state and how that state advances, not on the surrounding demo setup.
