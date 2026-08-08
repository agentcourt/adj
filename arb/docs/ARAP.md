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

The parties present merits arguments during the arguments phase.  They may offer visible evidence as exhibits and submit technical reports during arguments.  The claimant may also offer exhibits and submit technical reports during rebuttal when those materials answer the respondent's argument.  The respondent may offer exhibits and submit technical reports during surrebuttal when those materials answer the rebuttal.  All admitted materials become part of the record considered by the council.

## Rule 6: Closings

Each side has one closing statement.  The claimant closes first, and the respondent closes second.  Closings summarize the record and apply the stated standard of evidence to the disputed proposition.

## Rule 7: Deliberation

After closings, the council deliberates in rounds.  Each council member votes on whether the proposition is substantially true under the stated standard of evidence.  The configured vote threshold resolves the arbitration.  If no side reaches that threshold within the allowed number of rounds, the matter ends without a majority decision.
