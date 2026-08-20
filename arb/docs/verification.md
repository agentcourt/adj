# Verification

## Purpose

This document summarizes the current Lean verification surface for AAR.  The theorem index catalogs selected declarations, while the proof statistics count the complete proof tree.  This page explains what the principal declarations prove about the procedure.

## Current Results

The Lean library proves AAR properties over every reachable execution.  The results cover phase order, procedural parity, case-frame stability, material limits, outcome soundness, live-state liveness, council integrity, record provenance, fixed-frame progress, bounded termination, and deliberation neutrality.  Replay-specific predicates also prove that each accepted merits filing offered only evidence present in that filing's source-state record.

| Area | Main theorem family | Meaning |
|---|---|---|
| Merits structure | `reachable_phaseShape` | Every reachable state preserves the intended merits sequence. |
| Procedural parity | `reachable_proceduralParity` | The engine does not give one side extra merits turns. |
| Case immutability | `initialized_run_preserves_caseFrame` | A successful run keeps one proposition, one policy, and one council identity set. |
| Aggregate material caps | `reachable_materialLimitsRespected` | Reachable states respect cumulative exhibit and report limits. |
| Outcome soundness | `reachable_closed_demonstrated_sound`, `reachable_closed_not_demonstrated_sound`, `reachable_closed_no_majority_sound` | Closed outcomes follow from recorded deliberation state and executable closure conditions. |
| Exact action authority | `step_ok_matches_currentOpportunity` | Every accepted public action carries the exact current opportunity id, state version, role, phase, and scheduled council member, and the opportunity authorizes the requested operation. |
| Live-state liveness | `reachable_active_has_nextOpportunity` | A reachable active case always has a next public opportunity. |
| Council integrity and status monotonicity | `reachable_councilVoteIntegrity`, `step_shrinks_seatedCouncilMemberIds`, `step_introduces_newCouncilVotes_only_from_seated` | Stored votes stay well formed, seated membership only shrinks, and new votes come from seated members in the source state. |
| State record integrity | `reachable_recordIntegrity`, `initialized_run_preserves_evidenceCatalog`, `evidenceCatalogValid_ids_nodup` | The initial catalog remains fixed, uses unique identifiers and canonical 64-character lowercase hexadecimal SHA-256 commitments, and stays disjoint from unique submitted identifiers.  Derived submissions refer to an initial or earlier parent with the committed hash, accumulated offered evidence resolves within its size limit in the current state, and technical reports satisfy their UTF-8 byte limits. |
| Replay-time offer chronology | `replaySteps_success_meritsOfferChronology`, `replayInitialized_success_meritsOfferChronology`, `checkReplayCertificate_ok_meritsOfferChronology` | Each accepted argument, rebuttal, or surrebuttal resolves every offered identifier against the immutable catalog or the submitted-evidence list in the source state before that filing. |
| Record provenance and append-only growth | `reachable_recordProvenance`, `stepReachableFrom_materialsExtend` | Admitted materials come from allowed phase-role origins, and later successful steps only append to those lists. |
| Fixed-frame progress | `fixedFrameProgress`, `step_establishes_fixedFrameProgress`, `initialized_run_progresses_in_initial_frame` | Successful steps stay inside one case frame while admitted materials only append, seated council identifiers only shrink, phase rank never falls, and deliberation round never decreases. |
| Bounded termination | `stepPath_length_le_initializedBudget` | Successful public runs from initialization are finite, with an explicit procedural upper bound. |
| Deliberation neutrality | `reachable_currentResolution_is_neutral_under_vote_flip` | Flipping every current-round substantive vote flips the substantive outcome in the same way. |
| Replay certificates | `checkReplayCertificate_terminal_facts`, `checkReplayCertificate_status_closed_facts`, `checkReplayCertificate_status_failed_facts` | Accepted terminal certificate facts include exact replay, each action's exact opportunity authority, replay-time offer chronology, reachability, bounded length, decision-summary replay, and either closed-case soundness or an accounted failed-opportunity record. |
| Failure resilience | `step_fail_opportunity_same_round_resilience` | Same-round failure steps preserve stored council votes, preserve no-substantive-outcome viability, and block a new substantive current resolution under that premise. |

`RecordIntegrity` describes one state and checks the accumulated offered-evidence list against that state's complete submitted-evidence list.  `MeritsOffersUsePriorRecord` checks one accepted argument, rebuttal, or surrebuttal against the record in its source state.  `MeritsOfferChronology` carries that action-level property through a successful replay.  The chronology proof therefore establishes that a later submission cannot supply the reference for an earlier offer.

Together, these theorems show that the engine keeps the procedure in order, preserves record integrity, and closes cases only in ways justified by the stored deliberation state.  They also show that the engine neither strands a live case nor admits an infinite successful public run.  The replay predicates bind accepted actions to their exact opportunities and bind merits offers to the record available at filing time.

## Proof Structure

The proof library has three organizing layers.  Reachability and preservation theorems describe valid public executions.  Deliberation-summary and viable-outcome theorems isolate the vote arithmetic used for substantive closure and `no_majority`.  Fixed-frame progress theorems describe monotone movement inside one initialized case frame.

`engine/Proofs/DeliberationSummaryCore.lean` carries the summary definition, direct correspondence with the executable resolution rule, and council arithmetic independent of reachability.  `engine/Proofs/ViableOutcomesCore.lean` carries the pure viability language, closure language for `no_majority`, and monotonicity lemmas.  `engine/Proofs/OutcomeSoundness.lean` and `engine/Proofs/NoStuck.lean` use those layers to prove current outcome and liveness claims over reachable states.

`engine/Proofs/Progress.lean` defines `fixedFrameProgress`, a source-anchored state relation that packages frame preservation, append-only admitted materials, shrinking seated-member identifiers, nondecreasing phase rank, and nondecreasing deliberation round.  `engine/Proofs/ProgressViability.lean` adds same-round deliberation progress, which combines fixed-frame progress with viability shrinkage for same-round council actions.

## Replay Certificates

AAR now has a packet-level replay certificate path.  The runtime writes `certificate.json` with the initialization request, accepted public actions and their opportunity authorities, claimed final state, and compact final-state hash.  The `aar verify-certificate` command checks that artifact against `state.json` and the Lean engine, while services list and serve the artifact through ordinary artifact routes.

The Lean certificate package covers both terminal outcomes.  Closed packets carry exact replay, exact opportunity authority, replay-time offer chronology, reachability, terminal accounting, record integrity, catalog immutability, ordered merits completion, filing counts, decision-rule facts, matched-case closed-resolution agreement, decision-summary replay, and resolution-specific soundness.  Failed packets carry exact replay, exact opportunity authority, replay-time offer chronology, reachability, record integrity, catalog immutability, the initialized action-length bound, decision-summary replay, and an `opportunity_failed` record identifying a plaintiff or defendant role and the failed phase.  `ClosedCertificateFacts` and `FailedCertificateFacts` expose the chronology as `merits_offer_chronology`.  Both packages derive final-state record facts from `checkReplayCertificate_ok_recordIntegrity` and filing-time chronology from `checkReplayCertificate_ok_meritsOfferChronology`.

ADC and AARD now follow the same operator boundary with procedure-specific certificate schemas.  ADC writes `state.json` and `certificate.json`, verifies them with `adc verify-certificate`, and proves accepted-certificate replay, closed-terminal accounting, verdict facts, deliberating-juror timeout verdict facts, deliberating-juror timeout hung-jury facts, judgment facts, a combined outcome fact package, and concrete replayed examples.  AARD writes `state.json` and `certificate.json`, verifies them with `aard verify-certificate`, and proves exact replay, reachability, terminal accounting, answer-pair replay for closed certificates, and failure-record replay for failed certificates.

Services expose certificate artifacts through the existing artifact APIs.  They list and fetch the files when a terminal packet contains them.  They do not run replay verification as part of case creation, listing, polling, or artifact reads.

## Limits

Lean proves properties of the executable procedure and the evidence commitments stored in engine state.  The Go runtime establishes the relationship between those commitments and files in `evidence-store/`; certificate replay does not read or hash the files.  The proof does not establish the truth of the proposition, the adequacy of a lawyer's search or reasoning, or a formal semantics for the evidence-standard string.
