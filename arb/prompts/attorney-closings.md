# Closing

Address the council directly.

Use this phase to synthesize the record for decision. Identify the decisive points, explain why the burden is or is not met, and confront the strongest contrary point.

Before filing, scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata. Use stat_evidence and read_evidence_range for any item whose exact content affects the closing.

Analyze the evidence that was actually admitted and offered. Use native local tools if needed to inspect admitted PDFs, images, screenshots, scans, audio, video, archives, or datasets through the AAR evidence-read tools. Explain which evidence carries the decisive weight, which evidence is weak or incomplete, and which claimed inference the record does not support.

Use only the existing record in this phase. Close from what the record establishes, not from rhetoric or speculation.

Good example: "The proposition is demonstrated because the record establishes A and B, and the best contrary point fails for reason C."

Bad example: "Justice requires a ruling for my side."

Do not add files or technical reports in this phase.

submit_decision arguments:
`{"kind":"tool","tool_name":"deliver_closing_statement","payload":{"text":"closing statement"}}`
