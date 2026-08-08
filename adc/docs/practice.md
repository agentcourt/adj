# Agent District Court Practice Guide

## Purpose and Sources

This guide teaches agent lawyers how to litigate in Agent District Court, or ADC.  It covers case planning, factual investigation, evidence development, motion practice, trial presentation, jury work, and post-verdict review.  Like Moore's Federal Practice and Procedure and similar practice guides, it explains how advocates use procedure to build proof, preserve the record, and sequence tactical choices.

The related [Agent District Court Manual](../manual.md) supplies the operating reference for commands, flags, the Role API, output records, and replay verification.  The [Agent Rules of Civil Procedure](ARCP.md) supply the civil procedure rules, and the [Local Rules Limits Guide](limits.md) supplies local limits and override concepts.  Those documents define the exact syntax, API shape, and rule text.

ADC practice depends on three records at once.  The pleadings and docket define the procedural case, the case files and admitted exhibits define the trial record, and the private work notes explain how agent lawyers searched, analyzed, and prepared.  Keep those records separate: private notes do not prove facts, case files do not become trial exhibits without the proper legal act, and docket entries control procedural history.

## Core Orientation

A complaint starts the action, the defendant answers or raises threshold objections, the court manages pretrial work, the parties develop the record, and the case resolves by bench decision, jury verdict, judgment, or another authorized disposition.  Complaint-driven runs currently normalize the dispute into a focused claim packet before live litigation begins, so the first task is to understand the claim elements, the defenses, the requested relief, and the files attached to the complaint.

The case process owns the litigation.  It owns the Lean state, the current phase, role opportunities, deadlines, invalid-attempt limits, file visibility, docket updates, event logs, work-note logging, verdict derivation, and final output.  Lawyers and jurors acting through the Role API can act only within the opportunity returned by that process.

ADC separates legal acts from investigation.  Legal acts are court-facing actions accepted through `submit_decision`, such as filing an answer, serving discovery, submitting a technical report, offering an exhibit, delivering a closing argument, proposing a jury instruction, or casting a juror vote.  Investigation uses support tools and the lawyer's ordinary computer resources to inspect files, search sources, extract text, verify signatures, prepare reports, and decide what the court-facing act should say.

## Procedure Map

Each phase determines what the lawyer can do and what the lawyer prepares for the next phase.  The current opportunity identifies the permitted legal tools.  Lean selects that tool set from the case state rather than from a static client list.

| Phase or stage | Main actors | Practice function |
|---|---|---|
| Complaint setup | plaintiff, planner, clerk | Convert the dispute and linked files into the initial case packet, claim theory, and private strategy memos. |
| Pleadings | plaintiff, defendant | Define the claim, admissions, denials, defenses, jurisdiction, and threshold objections. |
| Initial disclosures | parties | Identify key sources, custodians, damages material, and case files needed for discovery and trial. |
| Written discovery | parties | Use interrogatories, production requests, and requests for admission to close proof gaps and remove needless disputes. |
| Discovery disputes | parties, judge | Use Rule 37 motions and orders to resolve material failures, not to repeat ordinary disagreement. |
| Dispositive motions | parties, judge | Test threshold legal failure under Rule 12 or factual sufficiency under Rule 56. |
| Trial setup | clerk, judge | Resolve trial mode, enter the pretrial order, configure the jury, and set trial control terms. |
| Voir dire | judge, parties, jurors | Test juror suitability, preserve challenges, and empanel the jury unless voir dire is skipped by case policy. |
| Openings | parties | Frame the evidence path and disputed elements without adding proof. |
| Trial theory | parties | State the merits theory from the current record and prepare the evidence sequence. |
| Evidence phases | parties | Offer case files as exhibits, submit permitted reports, and rest when the record is complete. |
| Rebuttal and surrebuttal | parties | Answer the other side's new or central points with narrow analysis and, when allowed, targeted evidence. |
| Charge conference | parties, judge | Propose instructions, object to faulty language, and preserve charge issues. |
| Closings | parties | Apply the admitted record to each element, defense, and damages question. |
| Deliberation | jurors | Vote from the trial transcript, instructions, visible case files, admitted exhibits, and the case view. |
| Judgment | judge | Enter judgment from the verdict or bench decision and create the final case result. |

The phase sequence determines time allocation.  Pleadings and early discovery identify the proof problem.  Trial evidence phases build the record the jury or judge will use.  Closings cannot fix missing exhibits, incomplete verification, or a source-chain gap that should have been preserved earlier.

## Case Theory and Element Work

Every ADC lawyer needs an element table.  For the plaintiff, the table states each element, the fact that proves it, the file or exhibit that supports it, the expected defense answer, and the fallback source if the primary proof fails.  For the defense, the same table becomes a defect table: which element is legally insufficient, factually unproven, unsupported by admissible material, or defeated by an affirmative defense.

The element table should stay stable from complaint through closing.  If a document proves contract formation, the lawyer should use the same document when drafting discovery, preparing the pretrial order, offering exhibits, and closing to the jury.  If a later source changes the theory, the work notes and filing should explain that change rather than letting the case drift.

Damages need their own proof path.  A damages exhibit may show an amount, but it may not show causation, reasonableness, mitigation, or recoverability.  Damages analysis separates arithmetic from legal entitlement and ties each dollar category to a specific breach, reliance event, or loss mechanism.

| Planning item | Plaintiff question | Defense question |
|---|---|---|
| Element | What must be proved to win? | Which element can fail? |
| Source | Which file or outside source proves it? | Which source contradicts or narrows it? |
| Authentication | How will the source be tied to a party, system, date, or event? | What provenance or custody gap affects weight? |
| Causation | What connects the act to the loss or requested relief? | What intervening choice, missing timeline, or alternative cause breaks the chain? |
| Damages | Which item proves amount and reasonableness? | Which item shows only claimed spending or unsupported allocation? |

## The Record Model

ADC record practice centers on case files, exhibits, technical reports, docket entries, and transcripts.  A case file has a `file_id`, original name, label, size, hash, storage path, and recorded uses.  A file attached to the complaint is visible as a case file, but trial use requires offering it as an exhibit during the proper evidence phase.

An exhibit is a trial-facing use of a case file.  The act `offer_case_file_as_exhibit` tells the court that a party relies on a visible file for a stated purpose.  The returned exhibit identity, such as P-1 or D-1, becomes the short trial citation, while the same `file_id` remains the source identity.

Technical reports explain source work.  They are appropriate for signature verification, hash comparison, OCR, transcript preparation, metadata review, source-chain reconstruction, archive inspection, or a search ledger that affects weight.  The report should identify inputs by `file_id` or exhibit when possible, state the method, give the result, and state limits that matter to the factfinder.

Private work notes are outside the case record.  The `send_work_notes` tool records plans, work logs, search history, extraction steps, source URLs, tool errors, and reasoning for later evaluation, but the jury and judge decide from filed material, admitted exhibits, technical reports, and docketed argument.  If a work-note fact affects the case outcome, counsel must move the relevant source, extraction, or report into the record through an allowed legal tool.

## Role API and External Lawyering

The Role API gives a lawyer the current opportunity, prompt, legal-tool specifications, support operations, remaining time, and attempts left.  The lawyer calls `wait_for_opportunity`, reads an active opportunity, inspects the case with `case_status`, `get_case`, and `list_case_files`, and completes one legal act for that opportunity.  A response that names another active role requires the lawyer to wait for its own turn.

External lawyers do not need access to the case output directory.  They read the visible record through support operations: `list_case_files` for file identity and uses, `read_case_text_file` for readable `.md`, `.txt`, `.pem`, and `.b64` files, `request_case_file` when a provider can attach the raw file to the next model turn, and `read_case_file_bytes` when byte-level inspection affects analysis.  The lawyer's research and analysis run in its own environment, while court filings pass through ADC legal tools.

An external client translates its own interface into Role API requests.  The same practice duties apply to every client because the case process owns opportunity identity, tool authority, and validation.  An invalid submission returns a precise error while leaving the opportunity active when attempts remain.

## Full Computer Use

Lawyer agents use the computer resources available to them when those resources can find, test, or explain material evidence.  Those resources can include web search, source-page fetches, browser sessions, computer-use tools, local shell commands, OCR, PDF extraction, image inspection, video or audio transcript tools, metadata tools, hash tools, signature tools, archive tools, public APIs, and short programs.  Counsel should use these resources when the current legal act depends on exact source content, provenance, chronology, authenticity, or extraction quality.

Search results are leads, not evidence.  A search result, snippet, answer box, model summary, or index entry can identify a source target, but counsel should follow the lead to the source page, record, PDF, image, video, archive capture, API response, or other artifact before relying on it.  If the source supports a filing, counsel should import or produce the source as a case file when the current phase permits it, or explain why the source cannot be preserved and how that affects the argument.

Browser use applies when a source depends on rendering, layout, scrolling, session state, embedded media, or interactive controls.  A browser can reveal visible timestamps, author identity, surrounding context, media attachments, repost structure, and whether a basic text fetch missed material content.  When visual context affects meaning, counsel should preserve screenshots or source captures through the court-facing file path, and use a technical report to explain what the browser showed.

Local programs matter when ordinary reading is inadequate.  OCR can turn scans and screenshots into text; PDF tools can reveal hidden text, images, or forms; media tools can extract frames and transcripts; archive tools can list contents and paths; metadata tools can show file dates and formats; hash and signature tools can test integrity.  If counsel installs tools or writes scripts, the work notes should record what was installed or written, and a technical report should state the method when the result affects the case.

## Evidence Search

Evidence search begins with the element table.  Counsel identifies the likely primary source class for each material fact: official records, party communications, signed documents, invoices, logs, regulator pages, public filings, source code, repository history, API records, original media, archive captures, screenshots, transcripts, or contemporaneous notes.  Secondary sources can locate those materials, but proof should rest on the original or most direct preserved source when available.

Search terms should be planned, varied, and recorded.  Effective searches use party names, project names, dates, exact phrases, identifiers, repository paths, filenames, account handles, statutory names, event labels, hash values, and expected file types.  Counsel should search both for supporting material and for adverse material that would weaken or defeat the assigned side's theory.

Adverse search tests the assigned side's theory.  The defense should look for facts that make plaintiff's proof less complete, less material, less causal, or less reliable.  The plaintiff should look for defenses, alternative causes, later corrections, missing context, and source-chain weaknesses before making a claim that the record is decisive.

A search stops for a stated reason.  The reason may be that decisive sources were found and preserved, that remaining leads are cumulative, that a source class cannot be reached within the deadline, or that available access does not permit retrieval.  When a missing source affects proof, counsel should document queries, repositories, URLs or identifiers, retrieval methods, response codes or errors, sources found, sources preserved, and remaining gaps in work notes and, if material, in a technical report.

## Source-Chain and Browser Work

Modern evidence often arrives through chains.  A screenshot may show a post, a report may quote a statement, a clip may excerpt a longer video, and an archive may capture a page after edits.  Counsel should identify the original source, the publisher or author, the publication time, the relationship among reposts or mirrors, the full context around a clip, and any later correction that changes the inference.

Screenshots and clips require careful claims.  A screenshot may prove that a representation circulated, but it may not prove who authored the original statement, when the original appeared, whether it was edited, or whether the surrounding context changes meaning.  A clipped video may prove that a speaker said a sentence, but it may not prove the full exchange, the event date, or the absence of qualifying language.

Archive captures show prior versions when current pages have changed.  Counsel should preserve the capture URL, capture time, target URL, and visible content, and should compare current and archived versions when the difference affects proof.  A technical report should separate observed differences from the legal or factual inference drawn from those differences.

## Evidence Analysis

Evidence analysis states what each item proves, what it does not prove, and why the fact follows.  A document may prove that a party made a statement without proving that the statement was true.  An invoice may prove a charge without proving causation, reasonableness, or reliance.

Weight depends on provenance, custody, independence, completeness, and fit.  A primary source often carries more weight than a summary, but a primary source can be ambiguous, incomplete, unauthenticated, superseded, or disconnected from the element being argued.  Independent confirmation carries weight when multiple sources could share the same initial error, excerpt, dataset, or unsourced claim.

Conflicts should be named and resolved.  If sources disagree, counsel should compare source quality, chronology, custody, specificity, and context, then state why the standard of proof favors one source or leaves the fact unproven.  A filing that omits a known conflict weakens the rest of the argument because the factfinder has to reconstruct the analysis on its own.

Absence of evidence needs restraint.  A failed search may support an inference when the missing item would normally appear in a complete repository, under a known publication practice, or in an official record.  The inference is weaker when access is limited, the repository is incomplete, the event may be private, or publication practice is uncertain.

## Technical Reports and Extraction Work

Technical reports should make difficult evidence intelligible.  A report can explain how counsel verified a signature, decoded a base64 file, extracted text from a PDF, ran OCR on an image, inspected file metadata, prepared a transcript, compared hashes, reviewed an archive, or searched a source repository.  The report should give enough method detail for the judge or jury to understand the result without burying the reader in tool logs.

The source and the extraction should remain distinct.  The source file, page capture, image, video, audio, archive, or API response belongs in the case record when it can be preserved.  The technical report explains what counsel did with that source and what limits affect the result.

Reports separate observation from inference.  "OCR reads the timestamp as 2026-04-12 14:03 UTC" is an extraction observation.  "That timestamp places the statement after reliance occurred" is an argument tied to the report and the record.

| Report type | Proper use |
|---|---|
| Search ledger | Repositories, queries, URLs or identifiers checked, source hits, failed retrievals, and stopping reasons. |
| Extraction report | OCR, PDF text extraction, transcript preparation, frame notes, archive listings, or spreadsheet parsing. |
| Verification report | Hash comparison, signature verification, certificate review, metadata inspection, or source-file integrity checks. |
| Source-chain report | Relationship among original source, repost, clip, screenshot, archive capture, and later correction. |
| Comparison report | Differences among versions, statements, figures, timestamps, model outputs, or file contents. |

## Argument Writing and Record Use

Every argument should state a proof path.  The path begins with the legal element or procedural requirement, identifies the record item that bears on it, explains what that item proves, addresses the central adverse point, and states the requested ruling or verdict consequence.  This structure applies to motions, trial theories, exhibit descriptions, objections, rebuttal, closings, and post-judgment motions.

Argument should not ask the judge or jury to infer the source work.  If counsel used web search, a browser, OCR, metadata inspection, signature verification, or local programs to reach a conclusion, the record should contain the source file and any technical report needed to understand that work.  The argument then cites the case file, exhibit, or report and explains how it affects the governing element, defense, damages issue, or procedural rule.

Argument must account for proof limits.  Counsel should distinguish what the record establishes, what remains uncertain, and why the uncertainty does or does not defeat the assigned side's position.  Treatment of source limits, extraction errors, missing context, and adverse evidence gives the factfinder a decision path instead of a summary that depends on trust.

## Pleadings and Early Case Work

Complaint practice starts with jurisdiction, element facts, and relief.  The complaint should state the claim in numbered factual allegations that can later be admitted, denied, tested in discovery, and proved at trial.  Complaint attachments should be chosen for their element value, not because they are convenient or voluminous.

Answer practice requires disciplined admissions.  The defendant should admit what the record requires, deny the disputed point precisely, and state defenses that target elements or damages.  The answer preserves the defense theory without becoming trial briefing.

Rule 12 practice should stay narrow.  A Rule 12 motion should identify the pleading defect, the rule ground, the count or element affected, and the exact relief sought.  If the real dispute requires facts outside the complaint, counsel should preserve the point for discovery, Rule 56, or trial rather than forcing it into a threshold motion.

## Discovery and Pretrial Motions

Discovery should follow the element table.  Interrogatories identify positions, timeline, custodians, and the documents or communications that matter.  Production requests seek source files, metadata, complete threads, logs, and version history.  Requests for admission lock down authentication, dates, undisputed facts, and narrowed trial issues.

Discovery responses should build the trial record and protect credibility.  A targeted admission focuses the dispute.  An overbroad denial or context-free objection creates Rule 37 risk and gives the other side an easy credibility point.

Rule 37 motions depend on a chronology.  The moving party should show the original request, the response, the deficiency, the attempted cure, and the relief needed.  The responding party should show what was produced, why any objection was proportional or justified, and what cure was offered.

Rule 56 practice should be evidence-indexed.  The moving party should state each material fact, cite the record, and explain why no genuine dispute remains.  The opposing party should identify the specific record conflict, missing inference, credibility issue, or legal reason the fact cannot carry judgment.

## Trial Preparation

Trial preparation begins before trial setup.  Counsel should prepare an exhibit ranking, an objection chart, stipulations, proposed jury instructions, a verdict-form plan, and an element-by-element proof outline.  The pretrial order should preserve the issues and exhibits that matter, because trial phases will not give unlimited chances to repair a poor trial plan.

Exhibit ranking controls trial sequence because ADC evidence phases proceed one file at a time.  A party should know which file to offer first, what fact it proves, and whether the file is cumulative, foundational, or decisive.  When all material files have been admitted, resting is better than duplicating exhibits or weakening the record with marginal material.

Objection planning should be concise.  Counsel should prepare short grounds for relevance, foundation, authentication, scope, prejudice, hearsay-like concerns when applicable, and limits on technical reports.  The response to an objection should state the exhibit's element function and the foundation already in the record.

## Voir Dire and Jury Work

Voir dire tests whether jurors can apply the rules to the case.  Counsel should ask questions tied to treatment of admissions, views about agent conduct, comfort with technical evidence, burden-of-proof discipline, damages skepticism, and ability to separate authentication from merits.  The questions identify jurors who cannot apply the rules without previewing the whole case.

Cause challenges need concrete grounds.  A juror who says that all agent-generated documents are worthless regardless of authentication presents a different problem from a juror who wants careful proof.  Peremptory strikes should be reserved for residual risk after cause challenges have done their work.

If voir dire is skipped, trial presentation carries more explanatory load.  Counsel cannot rely on tailored juror questioning to test assumptions, so openings, evidence descriptions, jury instructions, and closings must explain the proof path with extra care.  Jurors still receive the trial transcript, instructions, visible case view, admitted exhibits, and visible case files during deliberation.

## Openings and Trial Theory

Openings state what the evidence will show.  They should identify the claim, burden, key elements, central evidence, and the expected defense answer without overstating facts that have not been admitted or preserved.  An opening gives the judge or jury a proof outline that later exhibits will confirm.

Trial theory submissions function as proof briefs.  They should apply the visible record to the contested elements, identify admitted facts, cite exhibits or case files, and state why the opponent's theory does not defeat the claim or defense.  A trial theory should not repeat every docket event, because the factfinder needs the path from evidence to result.

The defendant's trial theory should concede damaging facts when the record requires it.  A defense that accepts a narrow admission can still win on materiality, causation, reliance, mitigation, or damages.  Precision makes the defense more credible than a blanket denial that the record will not support.

## Evidence Phases

Evidence phases turn case files into trial exhibits.  Counsel should use `list_case_files` to confirm file identity and prior uses, read or request the files that matter, and offer one file per opportunity when the current tool set permits it.  The description should state the exhibit's purpose, not argue the whole case.

An exhibit description identifies the source and its element function.  "Invoice for 1,000 printed briefing packages, offered to prove the printing-cost component of claimed damages" is more precise than a generic label.  When a file supports authentication rather than merits, the description should say that and avoid implying broader proof.

Resting is an affirmative trial act.  A party rests when the necessary record has been admitted, when no unoffered material file remains, or when another exhibit would only duplicate the opponent's admitted evidence.  Resting does not prevent counsel from arguing from opponent-admitted exhibits in later phases.

## Rebuttal and Surrebuttal

Rebuttal and surrebuttal are answer phases.  Plaintiff rebuttal should answer the defense's central new points, and defense surrebuttal should answer plaintiff's rebuttal without rearguing the entire case.  The lawyer identifies one or two decisive claims and tests whether the admitted record supports them.

Targeted evidence can be appropriate when the phase and legal tools allow it.  A rebuttal source might be the full page behind a clipped screenshot, a corrected version of a document, a metadata report answering an authentication challenge, or an exhibit that supplies missing context.  The filing should explain why the new material responds to the preceding argument rather than expanding the case for its own sake.

A pass fits the phase when the opponent's filing adds no material point or the record and closing will answer it better.  A pass should be deliberate, with work notes recording the reason.

## Jury Instructions and Closings

Charge conference practice should start from the elements and defenses.  A proposed instruction should state the burden, the elements, the damages question, and any limiting rule the jury needs to avoid misuse of the evidence.  An objection should quote or identify the faulty language, state the legal problem, and request a concrete correction.

Closings synthesize the record.  They should state the burden, walk the factfinder through each contested element, handle the central adverse point, and request a verdict or judgment that matches the instructions and verdict form.  A closing should cite admitted exhibits and reports, not private notes or searches that never became record material.

Closing distinguishes proof from argument.  The exhibit proves a document, amount, date, admission, or source condition.  The closing explains why that proof satisfies or fails an element under the governing burden.

## Deliberation and Juror Practice

Jurors decide from the record exposed to them.  A deliberating juror receives the trial transcript from openings through closings, the final instructions, the visible case view, and operations for inspecting admitted exhibits and visible case files.  The juror uses those materials to vote on the claim, damages, confidence, and explanation.

Juror analysis should track the court's instructions.  A vote should state the element findings, the exhibits supporting each finding, any limits on the evidence, and the damages reasoning when voting for plaintiff.  A conclusory vote does not show how the record satisfies or fails the legal elements.

Juror failure is separate from disagreement.  If a deliberating juror opportunity fails, ADC removes that juror from the effective concurrence threshold and continues when the remaining jury can decide.  A hung jury follows from unresolved disagreement among eligible jurors, exhausted deliberation rounds, or the absence of any eligible juror able to form a verdict.

## Bench Trials

Bench trial practice uses the same evidence record, but the audience changes.  Counsel should write as if preparing proposed findings and conclusions under Rule 52 from the first trial act.  Each exhibit should support a proposed finding, and each finding should support a conclusion or remedy.

Bench arguments may use legal structure directly.  The judge can use element tables, chronology, and rule citations more directly than a jury, but factual proof still depends on admitted files and reports.  A bench closing gives the court proposed findings rather than a jury-style appeal to common sense.

## Judgment and Post-Judgment Work

Judgment work starts before the verdict returns.  Counsel should know what judgment language follows from each possible verdict, what post-verdict motions may be preserved, and what deadlines start at entry.  If a jury verdict is ambiguous, incomplete, or inconsistent, counsel should raise the issue before judgment language hardens around the wrong result.

Post-judgment motions require record discipline.  Rule 59 and Rule 60 arguments should identify the order, ruling, verdict, evidence, or newly discovered material at issue, then explain why the governing rule permits relief.  A general complaint about the result is not a post-judgment theory.

Enforcement and stays should be planned in exact terms.  The prevailing party should identify the relief granted, enforcement mechanism, and factual predicate for enforcement.  The losing party should identify the stay authority, requested duration, security or bond position, and effect on the judgment.

## Practical Method

ADC practice has four stages.  First, map the claim, defenses, damages, burden, and likely source classes.  Second, search and inspect with the full computer resources available, preserving sources and extraction work through the allowed record path.  Third, offer exhibits, file technical reports, and make legal submissions in the phases where the tools permit those acts.

The fourth stage is writing the argument.  The argument should read as a proof path from record to result: element, evidence, weight, adverse point, and conclusion.  Counsel should keep private work notes throughout, but the final filing must stand on the admitted record and the legal tools accepted by the case process.

The factfinder needs primary sources, adverse-source checks, preserved files, extraction limits, and element-level argument.  Search snippets, private notes, unsupported summaries, and broad narrative leave the factfinder without the material needed to decide.

## References

| Resource | Use |
|---|---|
| [Agent District Court Manual](../manual.md) | Commands, Role API, output records, and replay verification. |
| [Agent Rules of Civil Procedure](ARCP.md) | Governing civil procedure rules. |
| [Local Rules Limits Guide](limits.md) | Local limits, overrides, character budgets, invalid-action policy, and discovery controls. |
| [Juries](juries.md) | Jury configuration, pool behavior, voir dire, verdicts, and failure handling. |
| [Protective Orders](protectiveorders.md) | Confidentiality, access limits, and controlled processing. |
