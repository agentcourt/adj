# Practice Manual

## Purpose and Use

This manual teaches agent lawyers how to litigate arbs in Agent Arbitration.  It covers case planning, factual investigation, evidence preservation, filings, and presentation to the council.  The [Agent Arbitration Manual](../manual.md) defines exact commands, private HTTP routes, role operations, and storage paths.

The related [Agent Arbitration Manual](../manual.md) supplies the procedural and technical reference: commands, APIs, flags, role tools, runtime limits, and output artifacts.  This manual instead explains how an agent lawyer uses the procedure to build and test a case.  Its function is closer to Moore's Federal Practice and Procedure and similar practice treatises than to a command reference.

AAR litigation turns on disciplined fact work.  The council decides one proposition under one evidence standard, so advocacy depends on identifying the decisive factual elements, finding the strongest sources, preserving them in the record, and explaining the inference from source to result.  The lawyer's native computer tools determine how much evidence the lawyer can search, inspect, extract, verify, and preserve before the procedural window closes.

## Core Orientation

The complaint does one thing.  It states the proposition to be decided.  The standard of evidence comes from policy or case configuration, not from the pleading.  The governing rules remain [Agent Rules for Arbitration Procedure](ARAP.md).  The runtime role names are `plaintiff` and `defendant`; ARAP's claimant and respondent correspond to those roles.  There are no claims, counts, defenses, motions, discovery requests, or evidentiary hearings separate from the merits sequence.

The procedure runs in one line: openings, arguments, rebuttals, surrebuttals, closings, and deliberation.  Plaintiff goes first in openings, arguments, rebuttals, and closings.  Defendant goes second in openings and arguments, may answer with a surrebuttal, and closes last.  Deliberation then proceeds in rounds until one side reaches the configured vote threshold or the final round ends without one.

Each phase has a distinct job.  Openings frame the factual dispute and the expected proof.  Arguments build the main record and present the merits theory.  Rebuttal and surrebuttal answer the other side's strongest points.  Closings tell the council what the admitted record proves under the stated burden.

## Phase Map

The phase sequence controls advocacy and evidence work.  Each phase determines whether counsel can add record material, argue from material already admitted, or answer a new point with a targeted response.  The table maps each phase to its practice function.  The [Agent Arbitration Manual](../manual.md) states the current tool and API details.

| Phase | Who acts | Practice function |
|---|---|---|
| `openings` | plaintiff, then defendant | Frame the proposition, identify decisive factual elements, preview the proof, and mark missing sources or extraction work without submitting new evidence. |
| `arguments` | plaintiff, then defendant | Build the main record, submit source evidence, offer exhibits by `evidence_id`, add technical reports, and present the merits theory. |
| `rebuttals` | plaintiff | Answer the defendant's strongest argument with targeted evidence, reports, and analysis, or pass when the phase allows it. |
| `surrebuttals` | defendant | Answer the rebuttal with a narrow response, targeted evidence or reports, or pass when the phase allows it. |
| `closings` | plaintiff, then defendant | Apply the full admitted record to the proposition and standard of evidence without adding new material. |
| `deliberation` | council members | Vote from the record, evidence, reports, and filings. |

The phase map controls time allocation.  Arguments carry the main investigation and preservation burden, so counsel finishes decisive source work before drafting the argument.  Rebuttal and surrebuttal repair only targeted gaps created by the preceding filing, and closings cannot repair missing evidence.

## Theory of Proof

AAR case planning starts by translating the proposition into factual elements.  Counsel identifies what must be true for the proposition to satisfy the standard, what fact would defeat or narrow that result, and what source could establish each point.  The element map gives the search plan structure and prevents the filing from becoming a collection of disconnected observations.

The standard of evidence shapes both search depth and argument.  A preponderance case may turn on the best explanation of an incomplete record, while a higher standard may require stronger source custody, independent confirmation, or narrower factual claims.  The filing states how each factual element fits the standard, because council members vote on the proposition as configured rather than on a looser impression of who seems more persuasive.

Role advocacy does not license false precision.  If investigation produces adverse facts, counsel identifies them, explains their significance, and states the best truthful theory that remains for the assigned side.  When no strong pro-side theory survives, the filing says why the side's position is thin and preserves the best available argument without inventing facts, sources, or tests.

## Full Computer Use

AAR court tools control record acts.  Native computer tools control investigation capacity.  Agent lawyers use the full computer available to them when it can find, preserve, or test material evidence: web search, web fetch, browser sessions, local file tools, shell commands, OCR, PDF extraction, image and media inspection, metadata tools, hash tools, signature tools, archive tools, public APIs, and short programs.  Operator instructions, credentials, network access, installed programs, and turn deadlines set the practical limits, and counsel records those limits when they affect the proof.

Search tools provide leads.  They do not provide evidence.  A search result, snippet, model summary, or index card can identify a source target, but the lawyer must follow the lead to the source page, document, media item, official record, archive capture, API response, or other preserved artifact before relying on it.  If the source supports the filing, counsel submits the source content or a faithful captured form through the AAR evidence tools before citing it as support.

Browser use is necessary when a page is dynamic, visual, interactive, or dependent on client-side rendering.  The browser can confirm what a user would see, reveal embedded media, expose timestamps or surrounding context, and show whether a source page differs from text retrieved by a basic fetch tool.  When visual presentation affects meaning, counsel preserves screenshots, page text, source URLs, retrieval time, and a concise description of visible state.

Local programs are necessary when a source is too complex for ordinary reading.  PDFs may need text extraction, images may need OCR and metadata inspection, videos may need transcripts or frame notes, archives may need file listings, and signed material may need hash or signature checks.  When the environment permits installation, counsel may add programs that improve extraction or verification, and the filing reports the method, version-sensitive limits when relevant, and any error that affects weight.

## Evidence Search

Evidence search begins with a source map.  Counsel identifies the likely primary sources for each element: official records, court filings, regulator releases, original PDFs, original images, canonical posts, API records, archive captures, transcripts, full videos or clips, and statements by the relevant actor.  Secondary reporting can locate these materials, but the search usually continues to the primary source.

Effective searches use names, dates, exact phrases, source classes, repositories, identifiers, and expected file types.  A lawyer investigating a public statement searches the speaker's official channels, archived pages, video platforms, transcript sources, and reliable reports that quote or link the original.  A lawyer investigating a market condition, statutory event, corporate act, or government action searches the official rule text, regulator or agency pages, filings, press releases, and archived public records.

Adverse search is part of proof.  Counsel looks for the strongest source that would defeat, limit, or contextualize the assigned side's position.  The council can weigh the record only if the lawyer has checked contrary primary material, later corrections, full context around clipped media, conflicting timestamps, and source-chain breaks.

Search stops for a reason.  A lawyer may stop because the decisive sources have been found and preserved, because further search would be cumulative, because the remaining target cannot be reached within the turn, or because the available tools cannot access the repository.  The filing or a technical report includes a search ledger when missing or hard-to-get evidence affects the result: queries, repositories, URLs or identifiers checked, retrieval methods, tool errors, sources found, sources submitted, and unresolved gaps.

## Browser and Source-Chain Work

Many modern sources do not survive as one clean text document.  Social posts, short videos, embedded clips, live pages, comment threads, and reposted screenshots often require source-chain analysis.  Counsel identifies the canonical post or page, author or publisher, publication time, attached media, quoted or reposted source, shortlinks, archive captures, mirrored copies, fuller recordings, and later corrections when those facts could change the inference.

Use a browser for public pages whose content depends on rendering, session state, scrolling, media controls, or layout.  Counsel inspects the page enough to know whether a text fetch missed relevant material, whether a video or image is embedded, whether a timestamp or author identity appears only in the rendered page, and whether a capture needs visual context.  Browser observations become record evidence when they matter, through a screenshot, saved page, extracted text, or technical report tied to preserved source material.

Clips and screenshots require source-chain work.  A cropped image, excerpted video, or screenshot of a post may prove that someone circulated an assertion, but it may not prove the event, date, author, or context.  Counsel locates the original item where possible, preserves the best available artifact, and explains whether the preserved copy establishes the content, the source, the publication context, or only the existence of a secondary representation.

## Evidence Preservation and Admission

The record needs preserved source material.  Lawyer description cannot substitute for it.  A lawyer who finds material outside the current record submits it through `submit_evidence` or the chunked upload tools before relying on it, when the current phase allows evidence submission.  The returned `evidence_id` is the record identity, and `offered_evidence` cites visible evidence by that identifier rather than by a local filename, URL, or invented exhibit name.

Submission and offering serve different functions.  `submit_evidence` or chunked upload preserves source bytes and makes the item visible as record evidence.  `offered_evidence` tells the council which visible evidence the filing relies on as an exhibit.  In arguments, rebuttals, and surrebuttals, counsel submits material outside sources first, then offers the returned `evidence_id` when the filing relies on that source.

Evidence and analysis are different materials.  The source document, image, page capture, transcript, API response, or media file belongs in evidence when it supports a factual point.  The lawyer's search ledger, extraction notes, OCR explanation, hash comparison, source-chain analysis, or inferential synthesis belongs in a technical report or filing text.  A technical report can make source evidence intelligible, but it cannot replace the source when the source can be preserved.

Preservation includes provenance.  A complete source submission names the URL, canonical identifier, publisher or author when known, retrieval time, retrieval method, MIME type or file type, and relationship to any parent source or derived extraction.  When exact custody affects weight, counsel includes SHA-256 or other hash information and compares it to the AAR evidence metadata when available.

Binary or hard-to-read evidence often needs a faithful companion.  If the source is a scan, screenshot, image, video, audio file, archive, spreadsheet, or PDF with difficult text, counsel preserves the source artifact and submits or reports the extraction the council needs.  OCR, transcripts, page text, frame notes, metadata tables, archive listings, and image observations identify their source evidence and method so the council can separate the artifact from the interpretation.

Failed capture attempts can carry evidentiary weight.  If a high-value source cannot be retrieved, counsel records the URL or identifier, retrieval method, time, response code or error, rate-limit information when available, and the next-best preserved source.  A precise failed capture tells the council what was attempted and prevents a later filing from implying that the source was ignored.

## Evidence Analysis

Evidence analysis turns admitted material into proof.  The lawyer states what each item shows, what it does not show, and what inferential step connects it to the proposition.  A source that proves an event occurred may not prove timing, authorship, legal effect, authenticity, or completeness, and the argument keeps those distinctions visible.

Record inspection begins before advocacy in every lawyer phase.  Counsel scans the current record and evidence list, then uses `stat_evidence` and `read_evidence_range` for any item whose contents, custody, size, MIME type, or source metadata affects the filing.  Evidence-read budgets and filing caps favor early triage: inspect decisive evidence first, then use reports to explain extraction or verification work that the council cannot reproduce from the filing alone.

Weight depends on provenance, custody, independence, and fit.  Primary sources usually carry more weight than summaries, but a primary source can still be incomplete, unauthenticated, superseded, or ambiguous.  Independent confirmation carries weight when one source might share the same error, quote, edited clip, feed, or unaudited dataset as another.

Conflicts require analysis.  If two sources disagree, counsel identifies the conflict, compares source quality, chronology, custody, and specificity, and explains why the standard of evidence resolves the conflict one way or leaves the proposition unproven.  A filing that ignores a known conflict invites the council to distrust the rest of the proof.

Absence of evidence requires care.  A failed search may support an inference when the missing source would normally exist in a known repository, under a known publication practice, or in a complete official record.  The inference weakens when the repository is incomplete, the search access is limited, the event might have been private, or the source class has uncertain publication habits.

## Technical Reports

Technical reports are attorney work product submitted as part of a filing when the phase permits them.  They document work that the council needs to understand a source or a search result: extraction, verification, source-chain reconstruction, metadata comparison, hash calculation, OCR, transcript preparation, archive inspection, API sampling, or a search ledger.  They identify inputs by `evidence_id` when possible, state the method, give the result, and describe limits or errors that affect weight.

A report must be short enough for the council to use.  It omits long source material when the source is available as evidence.  It gives the result instead of burying it under tool logs.  The filing can cite the report for the method and conclusion, while the evidence provides the record basis.

Reports are strongest when they distinguish observation from inference.  "OCR of PX-2 reads the timestamp as 2025-05-31 14:03 UTC" is an observation about an extraction.  "That timestamp places the post after the deadline" is an inference that belongs in the argument, even if the report supplies the extraction that supports it.

| Report type | Proper use |
|---|---|
| Search ledger | Documents targeted searches, source repositories, queries, URLs or identifiers checked, material found, capture failures, and stopping reasons. |
| Extraction report | Explains OCR, transcript, PDF text, frame notes, archive listings, or other conversions from preserved source material. |
| Metadata or hash report | Records file metadata, hashes, signatures, certificates, timestamps, or comparisons that bear on custody or authenticity. |
| Source-chain report | Reconstructs relationships among a clip, screenshot, repost, original source, archive capture, and fuller context. |
| Comparison report | Compares conflicting sources, versions, statements, figures, or timestamps and identifies the material differences. |

## Openings

Openings are short, accurate, and forward-looking.  At that point the record may already include case-packet evidence, but the lawyer may not submit new evidence, offer evidence, or file technical reports.  The opening states the factual theory at a high level, identifies the proof that will matter, and tells the council why those expected facts matter under the burden of proof.

The opening frames the case and plans the investigation.  Counsel may use native tools to inspect available case materials, identify likely source targets, test whether public records exist, and decide which extraction or preservation work belongs in arguments.  If the opening depends on a source not yet in the record, the opening names the source target or search path rather than stating the missing fact as established.

An opening preserves credibility by staying within known proof.  It does not quote from unseen files, describe unperformed tests, or assert that a public source says something counsel has not preserved or read.  Credibility suffers when the same council later compares the opening to a thinner record.

## Arguments and Record Building

Arguments are the center of the case.  This is the main phase in which a side submits source evidence, offers exhibits, files technical reports, and presents the merits theory.  Counsel treats this phase as both record assembly and merits briefing, because later phases cannot replace a missing argument record.

The strongest argument is selective.  It offers evidence material to the proposition and the standard of evidence, not every source the search found.  Each exhibit serves a named inferential step: a document may establish the statement at issue, a page capture may establish publication, a transcript may establish the relevant words, and a metadata report may support authenticity or timing.

Argument text reads like a proof path.  It states the proposition, identifies the decisive factual elements, cites the record evidence for each element, addresses adverse evidence, and explains why the standard of evidence resolves the dispute.  The argument explains each cited source's role instead of leaving that inference to the council.

Record building comes before final drafting.  Counsel submits admissible source material first, confirms the returned `evidence_id`, prepares any technical reports, then writes the argument around the accepted record.  A lawyer who writes first often discovers too late that the decisive source cannot be captured, exceeds a limit, lacks provenance, or does not say what the search result implied.

## Rebuttal and Surrebuttal

Rebuttal and surrebuttal are response phases.  They answer the preceding filing's strongest points, not the whole merits case.  A focused response identifies the decisive opposing claim, tests the evidence behind it, and shows why the claim fails, narrows, or leaves the standard unsatisfied.

Response evidence is targeted.  If the other side relies on a clipped source, counsel may need to preserve the full source, later correction, original post, official record, or missing context.  If the other side relies on a technical assertion, counsel may need a focused extraction, metadata check, hash comparison, or source-chain report.

Passing can be sound when the phase permits it.  A pass is stronger than a repetitive filing when the record already contains the answer and no targeted response would help.  Counsel passes only when further response would add nothing material or would distract from closing.

## Closings

Closings synthesize the record.  By then the council has the filings, admitted evidence, offered exhibits, and technical reports.  The closing states what the record proves, what it fails to prove, and how the standard of evidence resolves the proposition.

A closing cannot add evidence or reports.  It must make record citations easy to follow and explain why missing proof changes the result.  If the side's case depends on an inference from absence, a source conflict, or a failed capture, the closing ties that point to the earlier search ledger or report rather than introducing a new factual account.

The strongest closing gives the council a decision path.  It identifies the one or two factual questions that decide the case, states the record answer to each, handles the strongest adverse point, and ends with the requested resolution.  It omits sources and procedural events that do not affect the vote.

## Work Notes and Search Ledgers

Work notes are private lawyer work product.  They contain the plan, issue outline, search log, source targets, URLs or identifiers, tools used, scripts or programs written, browser observations, OCR or extraction work, errors, adverse checks, reasoning, draft theory, decisions, and unresolved gaps.  They are not evidence, filings, technical reports, or legal support.

A search ledger becomes record-facing only when counsel includes it in a filing or technical report.  A record-facing ledger is concise and tied to a material issue.  It lists the decisive targets, repositories and queries checked, retrieval methods, material found and submitted, material not found, tool errors, and stopping reasons.

The council decides from the record.  Private notes can discipline the lawyer's work and let an operator inspect the investigation, but the council cannot treat those notes as proof.  If a search fact affects the decision, counsel moves the relevant source, extraction, or report into the record through an allowed filing path.

## Council Deliberation

The council reads the record directly after closings.  Each member casts an individual vote with a rationale, and the case resolves if either side reaches the configured vote threshold in a round.  The runtime vote labels are `demonstrated` and `not_demonstrated`; ARAP describes the same question as whether the proposition is substantially true under the configured standard.  No judge repairs the record, consolidates the theories, or supplies missing factual links.

The deliberation structure requires council-ready evidence.  If the decisive material is a binary file, an image, an audio clip, a spreadsheet, or a long PDF, counsel gives the council the extraction or report needed to understand it.  If the decisive inference depends on source custody, timing, or a search failure, counsel preserves that work in a report or filing before closing.

Arguments that depend on undocumented leaps usually fail here.  A council member may accept a narrow inference from a well-preserved record, but the filing cannot require the member to reconstruct the search, guess what a screenshot shows, or infer why a missing source changes the result.  Counsel builds a record complete enough for a careful council member to vote without independent investigation.

## Practical Method

A sound working method has four stages.  First, define the proposition, standard, decisive factual elements, and strongest adverse theory.  Second, search for primary sources and adverse sources with the full computer tools available, preserving source material and extraction work as the evidence rules allow.  Third, submit source material, technical reports, and offered evidence in the proper phase.  Fourth, write the filing as an inference path from admitted record to requested result.

The procedure rewards concentration.  Because there is no separate motion practice, discovery phase, or evidentiary hearing outside the merits sequence, every filing must do visible work.  Strong cases in this forum are compact, supported, candid about uncertainty, and explicit about the path from source to proposition.
