# Agent Rules for Arbitration Degree Procedure

## Rule 1: Scope

These rules govern a simplified adversarial procedure for resolving a disputed quantitative question before a council.  The procedure omits pretrial motion practice, voir dire, the judge, and the clerk.  The council answers the stated question with one integer from `0` through `100`, using the record and the stated judgment standard.

## Rule 2: Complaint

The complaint states the question to be decided.  The question should ask for a bounded quantitative judgment that can be answered from the record, such as how much one work reused another.  The judgment standard is a case parameter supplied by policy or case configuration, and the council applies that standard during deliberation.

## Rule 3: Council

The court constitutes a council before the merits begin.  Council members are selected through the same controlled sourcing process used in the sibling procedures, but this procedure keeps the existing council and `member_id` machinery rather than renaming the deciding body.  Council members deliberate after the parties finish the merits phases, and each member records one answer with a rationale.

## Rule 4: Merits Phases

The arbitration proceeds in this order: openings, arguments, rebuttals, surrebuttals, closings, and deliberation.  Each side has one opening.  The claimant argues first, and the respondent argues second.  The claimant may rebut, and the respondent may surrebut.

## Rule 5: Arguments and Record Material

The parties present merits arguments during the arguments phase.  They may offer visible evidence as exhibits, submit new source evidence, and submit technical reports during arguments.  The claimant may also offer exhibits, submit new source evidence, and submit technical reports during rebuttal when those materials answer the respondent's argument.  The respondent may offer exhibits, submit new source evidence, and submit technical reports during surrebuttal when those materials answer the rebuttal.  Source evidence preserves content and provenance for material outside the initial case file set; technical reports record attorney analysis, measurements, comparisons, or synthesized work product.  All admitted materials become part of the record considered by the council.

## Rule 6: Closings

Each side has one closing statement.  The claimant closes first, and the respondent closes second.  Closings summarize the record, explain how the stated judgment standard applies, and advocate a concrete answer or a narrow numeric range.  A closing should explain why that advocated number fits the record better than nearby alternatives.

## Rule 7: Deliberation and Result

After closings, the council deliberates.  Each seated council member answers the question once for the current round with one integer from `0` through `100` and a brief rationale.  The arbitration result is the full answer map keyed by `member_id`.
