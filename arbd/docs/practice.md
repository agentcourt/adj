# Agent Arbitration Degree Practice Guide

Agent Arbitration Degree asks one bounded question and returns an independent answer from 0 through 100 for each council member.  Lawyers develop an adversarial record through openings, arguments, rebuttal, surrebuttal, and closings.  Council members then answer under the stated judgment standard.

The governing source is [Agent Rules for Arbitration Degree Procedure](ARAP.md).  The operator reference is the [Agent Arbitration Degree Manual](../manual.md).  This guide addresses question framing, evidence preservation, technical analysis, numeric argument, and council review.

## Procedure Map

| Phase | Actor | Work |
|---|---|---|
| `openings` | plaintiff, then defendant | Frame the question, material facts, proposed method, and expected dispute. |
| `arguments` | plaintiff, then defendant | Build the main record with submissions, offered evidence, reports, and a proposed answer or range. |
| `rebuttals` | plaintiff | Answer the defendant with focused argument and, when needed, new evidence or reports. |
| `surrebuttals` | defendant | Answer the rebuttal with focused argument and, when needed, new evidence or reports. |
| `closings` | plaintiff, then defendant | Apply the complete record and judgment standard to a proposed answer or range. |
| `deliberation` | council members | Review the final record and submit one integer answer with a rationale. |

Evidence-reading tools remain available throughout the lawyer phases.  Evidence-submission tools remain available during arguments, rebuttals, and surrebuttals.  Openings and closings can cite admitted material but cannot add evidence or technical reports.

## Record and Work Notes

The record contains the complaint question, immutable initial evidence commitments, lawyer filings, accepted submissions, offered evidence, technical reports, and council answers.  Each initial commitment fixes an `evidence_id`, SHA-256, and byte size before the first opening.  Each accepted submission adds source metadata and the same byte commitments to the ordered record.

A derived submission names a public parent identifier and derivation method.  The runtime resolves that parent from the current record and supplies its verified SHA-256, while the engine requires the parent to be initial or earlier submitted evidence.  The derivation string records the claimed transformation but does not prove that the child bytes follow from the parent bytes.

Work notes remain outside the evidentiary record in `work-notes.ndjson`.  They can record plans, search logs, source leads, adverse facts, checks, dead ends, and provisional scoring views for later review.  A filing and council answer must rely on admitted record material rather than private notes.

## Evidence Search and Preservation

A degree answer often depends on source quality, provenance, and method.  A useful search plan identifies facts that would move the answer toward the lower, middle, or higher part of the scale, then tests each possibility.  Official records, archived sources, metadata, hashes, text extraction, image inspection, and small case-specific analyses can supply those tests.

Source preservation precedes citation.  A lawyer relying on outside material submits the source or a faithful extract through direct submission or chunked upload before offering its `evidence_id` in a filing.  This order is mandatory because Lean checks each offered identifier against the source state of the filing action.

An extract or transformation should identify its admitted parent and describe the derivation method.  The parent digest comes from the runtime's verified record rather than participant input alone.  A technical report should state the extraction, comparison, measurement, or synthesis method without presenting private working material as source evidence.

## Evidence Analysis

Questions about degree require an explicit scoring method.  A filing can identify the scale, scored features, weights or qualitative priorities, uncertainty treatment, and reasons nearby numbers fit the record less well.  The judgment standard supplies the common instruction under which each council member evaluates that method.

Technical reports can preserve analyses that the council should consider as part of the record.  A text-comparison report can align passages, count shared phrases, distinguish common conventions from distinctive overlap, and compare structure.  A chronology report can identify timestamps, source order, archive captures, and inconsistencies across official records.

Each side should address facts that resist its proposed score.  A plaintiff proposing `85` can explain why the record supports neither `65` nor `98`.  A defendant proposing `25` can identify both the facts that prevent a lower answer and those that prevent a higher answer.

## Phase Practice

Openings frame the method and the expected factual dispute.  They can identify admitted initial evidence but cannot add offered-evidence or report arrays.  Detailed assertions should remain tied to record material that the council can inspect.

Arguments build the main record.  A complete argument submits required source material first, offers important evidence by `evidence_id`, includes bounded technical reports when useful, and relates each item to a concrete answer or narrow range.  The defendant can test the plaintiff's sources, method, weights, and treatment of uncertainty while developing contrary evidence under the same record rules.

Rebuttal and surrebuttal answer the preceding filing.  Both phases permit evidence submission, offered evidence, and technical reports under the same custody, lineage, count, and byte limits used for arguments.  Their narrower sequence favors material that resolves a disputed source, method, or scoring consequence.

Closings apply the final record to the question and judgment standard.  Each closing can identify the supported answer, explain why neighboring values fit less well, and account for remaining uncertainty.  A closing cannot introduce new evidence or depend on private work notes.

## Council Practice

Every council member receives the complaint, final record, judgment standard, and one opportunity to submit an integer answer with a rationale.  The rationale can identify decisive filings and evidence, explain the scoring method, and address the principal competing range.  Each member answers independently, and the result preserves the full map rather than aggregating it.

The `direct` backend passes a rendered record prompt to the selected model and exposes only `submit_council_answer` for that model call.  The runtime writes `council-turns/.../input.json` and `prompt.txt` for the operator under schema `aard.council-turn-snapshot.v0`.  The model does not read those files as an API.  The operator can use the snapshot to inspect the exact record and prompt supplied for the turn.

The `councilapi` backend gives an external member live read-only evidence operations and `submit_council_answer` during the active opportunity.  Such a member can inspect important exhibits through bounded, descriptor-verified reads before answering.  Successful Council API reads create `evidence_read` events and consume that opportunity's read budget.

## Working Method

A lawyer can begin a turn by reading the current state, admitted evidence, and preceding filings.  The resulting plan identifies the unresolved factual question, the source or method that can answer it, the adverse result that would change the score, and any bytes that require admission.  Before filing, the lawyer records useful work notes, commits required evidence, and then offers only identifiers visible in the updated source state.

A council member can begin with the complaint, final filings, judgment standard, and evidence metadata.  The next review isolates the exhibits and reports that control the score under the backend's available interface.  The answer then states the number, method, decisive record material, and principal reason the rejected range fits less well.
