# Agent Rules for Arbitration Degree Procedure

## Rule 1: Scope

These rules govern a simplified adversarial procedure for resolving a disputed quantitative question before a council.  The procedure omits pretrial motion practice, voir dire, the judge, and the clerk.  The council answers the stated question with one integer from `0` through `100`, using the record and the stated judgment standard.

## Rule 2: Complaint

The complaint states the question to be decided.  The question should ask for a bounded quantitative judgment that can be answered from the record, such as how much one work reused another.  The judgment standard is a case parameter supplied by policy or case configuration, and the council applies that standard during deliberation.

## Rule 3: Council

The court constitutes a council before the merits begin.  The case seats the configured number of members and records each selected `member_id`.  Council members deliberate after the parties finish the merits phases, and each member records one answer with a rationale.

## Rule 4: Merits Phases

The arbitration proceeds in this order: openings, arguments, rebuttals, surrebuttals, closings, and deliberation.  Each side has one opening, and the plaintiff argues before the defendant.  The plaintiff may rebut, and the defendant may surrebut.

## Rule 5: Arguments and Record Material

The initial evidence catalog fixes the identifier, canonical 64-character lowercase SHA-256 digest, and byte size of each initial case file when the engine initializes the case.  An admitted submission has a positive size within the submitted-evidence limit, a canonical digest, source metadata, and an identifier distinct from every initial or earlier submitted item.  The initial catalog remains unchanged throughout the case.

A derived submission identifies its parent with a complete `parent_evidence_id`, `parent_sha256`, and nonblank `derivation_method`, or leaves all three fields empty.  The parent must be an initial item or an earlier submitted item visible in the source state, and the parent digest must match that record item.  A submission cannot name itself as its parent.

The parties may offer visible evidence, submit new source evidence, and submit technical reports during arguments.  The plaintiff has the same permissions during rebuttal, and the defendant has them during surrebuttal.  Every offered identifier must appear in the filing's source state and remain within the exhibit-size and filing-count limits, while every technical-report title and summary must remain within its UTF-8 byte limits.

## Rule 6: Closings

Each side has one closing statement.  The plaintiff closes first, and the defendant closes second.  Closings summarize the record, explain how the stated judgment standard applies, and advocate a concrete answer or a narrow numeric range.  A closing should explain why that advocated number fits the record better than nearby alternatives.

## Rule 7: Deliberation and Result

After closings, the council deliberates.  Each seated council member answers the question once for the current round with one integer from `0` through `100` and a brief rationale.  The arbitration result is the full answer map keyed by `member_id`.

## Rule 8: Opportunity Authority and Failure

Every state-changing participant action carries the current opportunity identifier, expected state version, role, phase, and council member when applicable.  The engine requires exact authority and permits only an operation assigned to that opportunity.  A stale, mismatched, or unauthorized action does not alter the state.

A plaintiff or defendant opportunity failure records the role, phase, opportunity, reason, and message, then ends the case with status `failed`.  A council-member failure marks that scheduled unanswered member `failed`, preserves earlier answers, and leaves the remaining seated members to answer.  The case closes after such a council failure when every member still eligible to answer has answered.
