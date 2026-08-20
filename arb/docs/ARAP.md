# Agent Rules for Arbitration Procedure

## Rule 1: Scope

These rules govern a simplified adversarial procedure for resolving a disputed proposition before a council.  The procedure omits pretrial motion practice, voir dire, the judge, and the clerk.  The council determines whether the proposition is substantially true under the stated standard of evidence.

## Rule 2: Complaint

The complaint states the proposition to be decided.  The standard of evidence is a case parameter supplied by policy or case configuration, and the council applies that burden in deliberation.

## Rule 3: Council

The court constitutes a council before the merits begin.  Council members are selected through the same controlled sourcing process used for jurors in `agentcourt`, but without voir dire.  Council members deliberate and vote after the parties finish the merits phases.

## Rule 4: Merits Phases

The arbitration proceeds in this order: openings, arguments, rebuttals, surrebuttals, closings, and deliberation.  Each side has one opening.  The claimant argues first, the respondent second.  The claimant may rebut, and the respondent may surrebut.

## Rule 5: Arguments and Record Material

The parties present merits arguments during the arguments phase.  They may submit source evidence, offer visible evidence as exhibits, and submit technical reports during arguments.  An exhibit must identify an item in the initial evidence catalog or an earlier submitted item, and the item's committed size must fit the exhibit limit.  A derived submission must identify an initial or earlier submitted parent and state the derivation method.  AAR records the parent's committed SHA-256 with the derived submission, and every SHA-256 commitment contains exactly 64 lowercase hexadecimal characters.  The claimant may add these materials during rebuttal when they answer the respondent's argument, and the respondent may do so during surrebuttal when they answer the rebuttal.  Technical-report titles and summaries must fit their UTF-8 byte limits, and every accepted material becomes part of the record considered by the council.

## Rule 6: Closings

Each side has one closing statement.  The claimant closes first, and the respondent closes second.  Closings summarize the record and apply the stated standard of evidence to the disputed proposition.

## Rule 7: Deliberation

After closings, the council deliberates in rounds.  Each council member votes on whether the proposition is substantially true under the stated standard of evidence.  The configured vote threshold resolves the arbitration.  If no side reaches that threshold within the allowed number of rounds, the matter ends without a majority decision.
