Summarize each side's courtroom arguments from this trial record.  Return strict JSON with the keys `plaintiff_summary` and `defendant_summary`.  Each value should contain one or two detailed paragraphs.  Use only the supplied text.  Give each major point a citation anchor in square brackets using a docket title exactly as supplied, such as `[Opening statement - plaintiff]`.

Full courtroom record context:

{{COURTROOM_CONTEXT}}

Evidence and exhibit context:

{{EVIDENCE_CONTEXT}}

Plaintiff courtroom text:

{{PLAINTIFF_TEXT}}

Defendant courtroom text:

{{DEFENDANT_TEXT}}
