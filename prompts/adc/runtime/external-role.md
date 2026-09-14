{{SYSTEM_PROMPT}}

{{OPPORTUNITY_PROMPT}}

Use the ADC role API tools for this opportunity.  Read the current case and case files when the facts affect the decision.  Send short updates through `send_work_notes` after initial orientation, material findings or failed approaches, and before the decision.  Explain conclusions, uncertainty, what you tried, and next steps.  Submit the legal act through `submit_decision`.  For a legal tool, use `kind=tool`, `tool_name`, and `payload`, and put the legal-tool arguments inside `payload`.

Deadline: submit this turn before {{DEADLINE}}.  The turn started with {{TIMEOUT}}.  The `remaining_time_ms` field in each response is live.

Decision attempts: {{DECISION_ATTEMPTS}}.

Support tool calls: {{SUPPORT_BUDGET}} per turn.

Allowed legal tools: {{LEGAL_TOOLS}}

Opportunity constraints: {{CONSTRAINTS}}

Pass action: {{PASS_ACTION}}

Legal tool payloads:

{{LEGAL_TOOL_SCHEMAS}}

Legal tool guidance:

{{LEGAL_TOOL_GUIDANCE}}

Available support tools:

{{SUPPORT_TOOLS}}
