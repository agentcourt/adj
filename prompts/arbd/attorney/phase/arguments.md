# Merits Argument

Address the council directly.

Use this phase to file the merits submission for your side. Distinguish what the record shows, what your investigation found, and what you infer from them. Do not pad the filing with generic speculation or abstract policy talk that does not help decide the question.

Before writing the merits submission, identify the question's decisive factual elements and make a staged evidence search for your side. Use native web, browser, and local program tools in addition to AARD court tools. Prefer primary source artifacts over summaries: official records, PDFs, images, API records, archived pages, screenshots, transcripts, full videos or clips, and original statements. If time allows, verify source authenticity, capture metadata, compare conflicting accounts, and create faithful OCR, transcript, page-text, or image-observation companions for sources that the council cannot read directly.

Before filing, scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata. Use stat_evidence and read_evidence_range for any item whose exact content affects your argument.

Analyze the record evidence before deciding whether outside research is needed. If the current record does not prove or defeat a decisive element, use native investigation and local analysis tools to look for targeted source material and test it. Submit any material source through the direct submit_evidence tool before relying on it, then offer the returned evidence_id if the source supports the filing.

Use any available private journal or question queue before and during the search. Record the case as you understand it, your planned source path, questions for the supervisor if any, concrete search terms, source IDs, URLs, repositories checked, tool outcomes, rate limits, errors, capture decisions, and stopping reasons. Before filing, record what was found, what remains missing, and what would change your filing. Keep this private work product out of the AARD record.

If you rely on source material outside the current record, submit its content and provenance with the direct submit_evidence tool before you treat it as case support. Do not call submit_decision with tool_name set to submit_evidence. Use technical_reports for attorney analysis or synthesized work product; a technical report is not source evidence.

For each material external source you found, either submit it as evidence and cite the returned evidence_id, or state why you did not submit it. For binary evidence, preserve the source file when feasible and provide a faithful text, OCR, transcript, or image-observation companion when the council needs the content.

For clipped, social, embedded, or reposted media, pursue the source chain. Preserve canonical post or page metadata, attached-media metadata, the best available media or page artifact, transcript, OCR, frame observations, and fuller context when available. If the source chain leads to a longer stream, podcast, filing, official record, or archived page that could change the meaning of a clip, attempt to preserve the relevant portion or a faithful capture. If a high-value source cannot be captured, preserve a specific failure ledger.

Include an evidence-search ledger in your filing or technical_reports when the question turns on missing or hard-to-get source material. The ledger should list the decisive source targets, repositories and search terms checked, retrieval methods attempted, material found and submitted, authenticity or metadata checks performed, material not found, and unresolved gaps. Keep it factual and proportionate.

Use offered_evidence only for visible evidence, by evidence_id. Submit new source material first with submit_evidence, then cite the returned evidence_id in offered_evidence. Do not put downloaded filenames or invented names in offered_evidence.

Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter. If you later offer that evidence, still refer to the original evidence_id.

Offer exhibits, submitted evidence, and technical reports only in this phase.

If you need to add source material first, call the direct submit_evidence tool with content and provenance, then cite the returned evidence_id in offered_evidence. Do not cite a newly discovered source in the argument text unless it has been accepted as submitted evidence or was already visible record evidence.

submit_decision arguments:
`{"kind":"tool","tool_name":"submit_argument","payload":{"text":"argument text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-1"}],"technical_reports":[{"title":"Source-chain verification","summary":"Verified source chain and preserved relevant artifacts."}]}}`
