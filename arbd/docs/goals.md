# Design Goals

## Quantitative Adjudication

AARD handles questions answered on a bounded quantitative scale.  The complaint states one question, policy states the judgment standard, and each council member returns one integer from 0 through 100.  State, prompts, records, and certificates preserve the numeric answer directly.

The complete answer map constitutes the result, leaving aggregation or binary interpretation to downstream analysis.  Agreement and disagreement remain visible by member identifier.

## Adversarial Record

Plaintiff and defendant lawyers develop the record through openings, arguments, rebuttal, surrebuttal, and closings.  The plaintiff presents the strongest supported higher-score position.  The defendant tests that position and presents the strongest supported lower-score position.  Each filing remains subject to the same evidence, provenance, and record rules.

The procedure requires concrete numeric advocacy.  A filing should identify a proposed answer or bounded range, explain the method used to reach it, address contrary evidence, and distinguish nearby values.  Council members apply the declared judgment standard to the final admitted record.

## Traceable Evidence

The initial catalog fixes each evidence identifier, digest, and byte size before the first merits opportunity.  Later submissions record source metadata and optional lineage to an initial or earlier submitted item.  A filing may offer only evidence already visible in its source state.

Work notes preserve off-record plans, searches, checks, adverse observations, and provisional analysis.  Filings and council answers rely on admitted record material.  The separation permits review of participant work without treating private analysis as evidence.

## Executable Procedure

Lean controls initialization, phase order, current opportunities, exact action authority, accepted state changes, participant failure, and terminal status.  Go controls process execution, deadlines, attempt limits, model requests, evidence bytes, HTTP interfaces, and durable files.  The runtime sends every state-changing action through Lean and records the accepted source-state authority for replay.

The proof tree establishes the initialized procedure and case-frame invariants, their preservation under every accepted step, current-opportunity results for active proceedings, closed-case properties, failure effects, and certificate consequences.  Operational verification replays the recorded initialization and actions through the same Lean core.

## Procedure Independence

The AARD directory contains its engine, runtime, core commands, MCP adapter, documentation, policy, and examples.  The repository's `runtime/localrun/arbd/` package provides the local participant launcher.  Shared packages in `common/` provide model, evidence, record, and persona functions.  The unified `adjudicate` command can run one complete AARD case without an external service repository.
