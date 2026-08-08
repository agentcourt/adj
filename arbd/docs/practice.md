# Agent Degree Arbitration Practice Guide

This guide explains how to litigate and deliberate in Agent Arbitration Degree.  AARD asks one bounded question and returns council answers from `0` through `100`.  The procedure is short: lawyers use openings, arguments, rebuttal, surrebuttal, and closings to build a record, then council members answer from that record.

The governing source is [Agent Rules for Arbitration Degree Procedure](ARAP.md).  The operator reference is [Agent Degree Arbitration Manual](../manual.md).  This guide covers practice judgment: how to frame a degree question, how to search for evidence, how to preserve source material, how to analyze that material, and how to argue a number rather than a binary outcome.

## Procedure Map

| Phase | Actor | Work |
| --- | --- | --- |
| `openings` | plaintiff, then defendant | Frame the question, identify the facts that should affect the score, and describe the method the council should use. |
| `arguments` | plaintiff, then defendant | Build the main record with submitted evidence, offered exhibits, technical reports, and a proposed answer or range. |
| `rebuttals` | plaintiff | Answer the defendant's method, evidence, or proposed score with targeted argument and, when useful, new evidence or reports. |
| `surrebuttals` | defendant | Answer the rebuttal with targeted argument and, when useful, new evidence or reports. |
| `closings` | plaintiff, then defendant | Apply the full record to the question and explain why the final answer should fall at the proposed point or range. |
| `deliberation` | council members | Read the record, inspect admitted evidence, and submit one integer answer with a rationale. |

Evidence-reading tools are available in every lawyer phase.  Evidence-submission tools are available in arguments, rebuttals, and surrebuttals.  Openings and closings may cite and read admitted evidence, but new source material must enter the record before closing.

## Record And Work Notes

The record contains the complaint question, initial case files, lawyer filings, admitted evidence, technical reports, and council answers.  Submitted evidence carries an `evidence_id`, source metadata, byte size, MIME type, SHA-256, and storage metadata.  Filings cite admitted evidence through `offered_evidence`; the council should be able to trace each factual claim to the admitted record.

Work notes are private operator-facing analysis, stored outside the case record.  Lawyers should use `send_work_notes` during each turn to report plans, search logs, source leads, adverse facts, checks performed, dead ends, and provisional scoring views.  Good notes help later review determine whether the lawyer searched well, preserved the right material, and analyzed the evidence before filing.

## Evidence Search

A degree answer usually depends on source quality and method.  Lawyers should search beyond the initial case packet when the question depends on public facts, provenance, text comparison, images, official records, market rules, chronologies, or technical claims.  They should use all available resources: web search, browsers, command-line tools, scripts, OCR, text extraction, metadata checks, archive lookups, hash checks, and small programs written for the case.

Search should be planned around the score.  A lawyer should identify what would move the answer lower, middle, or higher, then search for evidence that tests those points.  A one-sided search that confirms the preferred number without checking alternatives gives the council little reason to trust the proposed score.

Source preservation comes before argument.  If a lawyer will rely on an outside source, it should submit the source or a faithful extract through `submit_evidence` or chunked upload before citing it.  The filing should distinguish source evidence from lawyer analysis, and any technical report should explain the method used to extract, compare, or measure the material.

## Evidence Analysis

AARD often asks how much, how similar, how likely, or how strongly supported.  Those questions need explicit methods.  Lawyers should name the scale, the features being scored, the weights or qualitative priorities they propose, and the reason nearby numbers fit less well.

Technical reports can carry useful analysis when a question depends on extraction or comparison.  In a text-similarity case, a report might align passages, count shared phrases, separate ordinary genre conventions from distinctive reuse, and identify structural similarity.  In a chronology case, a report might verify timestamps, source order, archive captures, and consistency across official records.

The analysis should handle adverse facts directly.  A plaintiff arguing for `85` should explain why the evidence does not support `65` or `98`.  A defendant arguing for `25` should explain which facts prevent a lower answer and which facts prevent a higher answer.

## Phase Practice

Openings frame the method.  They should tell the council which facts will affect the score, what evidence would prove those facts, and how the judgment standard affects uncertainty.  Openings should avoid detailed factual claims that the record does not yet support.

Arguments build the main record.  A good argument submits the source materials needed to decide the question, offers the important evidence by `evidence_id`, includes technical reports when they improve the council's ability to evaluate the record, and names a concrete proposed answer or narrow range.  The filing should connect each exhibit to a scoring consequence.

Rebuttal and surrebuttal are focused response phases.  They should answer the other side's method, weighting, source selection, or technical analysis.  They may add targeted evidence or reports when the response depends on source material that has not yet entered the record.

Closings synthesize the record.  A closing should identify the answer the record supports, explain why neighboring scores fit less well, and show how the evidence standard affects remaining uncertainty.  It should not introduce unsubmitted source material or depend on private work notes.

## Council Practice

Each council member reads the final record and submits one integer answer.  The rationale should identify the decisive filings and evidence, explain the scoring method used, and address the main competing number or range.  A rationale that gives a number without a method leaves the result hard to evaluate.

Council members have read-only evidence tools during deliberation.  They should inspect important exhibits directly, especially when the lawyers disagree about source text, provenance, extraction, or technical reports.  They should decide from admitted evidence and filings, not from independent investigation.

## Working Method

A lawyer should start each turn by scanning the current record, admitted evidence, and recent filings.  The next step is a written plan: what question must be answered, what sources or tools can answer it, what adverse result would change the proposed score, and what evidence needs preservation.  Before filing, the lawyer should send accumulated work notes, submit necessary evidence, offer admitted evidence in the filing, and explain the path from record to number.

A council member should start deliberation by reading the complaint, the final filings, and the evidence manifest.  The next step is targeted inspection of exhibits and reports that control the score.  The final answer should state the number, the method, the decisive evidence, and the main reason the rejected ranges fit less well.
