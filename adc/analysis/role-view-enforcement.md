# Role-view enforcement

Mermaid source: [`role-view-enforcement.mmd`](role-view-enforcement.mmd).

This diagram describes the enforcement loop that keeps a direct or external role inside its permitted view of the case.  The runner asks Lean for `role_view(state, role)`, then assembles the prompt from the returned view, role preamble, tool cards, and current opportunity.  External roles receive the same role-visible information through the Role API.

Inside the loop, the agent may call support tools before it submits a legal act.  The current support set includes `get_case`, `get_juror_context`, `list_case_files`, `read_case_text_file`, `request_case_file`, `read_case_file_bytes`, and `explain_decisions`.  Those calls do not bypass the role view.  They go back through Lean-backed visibility checks, and Lean returns only what that role may see.

When the agent submits a pass or a legal tool call through `submit_decision`, the runner asks Lean to apply the decision to the current opportunity.  If Lean rejects the decision, it returns a `StepErr` with an actor-facing correction message.  The runner returns that correction to the same agent, and the same opportunity remains open.  If Lean accepts the decision, the runner applies the accepted action with `step` and receives the updated state.

The diagram separates prompt construction from formal enforcement.  The role view constrains both prompt assembly and helper-operation access.  Lean validates the resulting decision before the state can change.
