# Proof Work Status

This note records the maintained proof trees and their operational boundaries.  The ARB results cover realisability, maximal-run terminal accounting, opportunity agreement, and certificate replay.  ADC and AARD provide procedure-specific proof trees.

## Current proof surfaces

| Procedure | Proof files | Theorem or lemma declarations | Lines |
| --- | ---: | ---: | ---: |
| ARB | 39 | 721 | 22,858 |
| AARD | 15 | 209 | 6,093 |
| ADC | 71 | 611 | 10,830 |

The maintained roots contain no `sorry`, `axiom`, or `unsafe` declaration.  ARB has the broadest theorem surface:

| Area | Anchor theorem or file | Status |
| --- | --- | --- |
| Reachability | `Reachable`, `StepReachableFrom` | Executions are modeled as successful initialization followed by successful public steps. |
| Phase order and parity | `reachable_phaseShape`, `reachable_proceduralParity` | Merits filings preserve the required order and side-to-side parity. |
| Case frame | `initialized_run_preserves_caseFrame` | Proposition, policy, and council identity are fixed across a run. |
| Record integrity | `reachable_recordIntegrity`, `step_ok_meritsOffersUsePriorRecord`, `initialized_run_preserves_evidenceCatalog` | Initial commitments remain fixed.  Submitted evidence has validated metadata, size, identity, and lineage.  Each merits offer refers to evidence available before that filing. |
| Supplemental-material limits and provenance | `reachable_materialLimitsRespected`, `reachable_recordProvenance`, `stepReachableFrom_materialsExtend` | Offered evidence references and technical reports respect caps, have allowed filing origins, and grow by suffix. |
| Outcome soundness | `reachable_closed_demonstrated_sound`, `reachable_closed_not_demonstrated_sound`, `reachable_closed_no_majority_sound` | Closed outcomes follow from current-round votes, the threshold, and executable closure conditions. |
| Liveness and realisability | `reachable_active_has_nextOpportunity`, `reachable_active_has_successful_step` | Every reachable active state exposes an opportunity and admits at least one successful public action. |
| Maximal paths | `initializedStepPathMaximal_terminal_accounted` | A maximal successful path from initialization ends closed with an enumerated resolution or failed with an accounted party-opportunity failure. |
| Opportunity authority | `reachable_actor_step_matches_nextOpportunity`, `checkReplayCertificate_ok_authorityConforming` | Accepted actor-facing actions match the advertised role and tool.  Replayed actions carry the exact opportunity id, state version, role, phase, and council member required at each step. |
| Replay certificates | `checkReplayCertificate_terminal_facts` | Accepted terminal replay certificates expose exact replay, exact opportunity authority, filing-time evidence chronology, reachability, record integrity, a fixed initial evidence catalog, bounded length, decision-summary replay, and closed or failed terminal facts. |
| Failure resilience | `step_fail_opportunity_same_round_resilience` | Same-round opportunity failure preserves stored council votes and does not create a substantive result once no substantive outcome remains viable. |
| Decision rule | `DecisionRuleFacts`, `DecisionRuleCharacterization` | The executable threshold rule has permutation invariance, neutrality, quota monotonicity, and a count-level characterization. |
| Due process | `reachable_status_closed_merits_complete`, certificate due-process facts | Closed cases and accepted closed certificates carry the ordered mandatory merits filings and filing-count facts. |

## Certificate Work

The term "certificate" in these systems names a package of a run's input, its accepted-action record, and its claimed final state, bound together by hashes.  The package carries no signature and no endorsement, and verification is recomputation: the verifier replays the recorded actions through the engine and compares the result with the claim.  The word is borrowed from complexity theory, where a certificate is a witness that makes a claim checkable without search.  The check here re-executes every engine transition and saves work because the recorded actions remove search and the model calls are not repeated.  Readers who expect the ordinary sense of an authority's attestation will be misled.  A passing package shows that the claimed outcome follows from the recorded history under the rules.  Attested execution addresses whether the recorded history corresponds to the execution that produced it.

All three formal procedures write runtime certificates and provide explicit verifier commands.  ARB remains the reference implementation for record-integrity proofs.  Services list and fetch certificate artifacts without running replay verification during case creation, listing, polling, or artifact reads.

| System | Runtime boundary | Proof boundary |
| --- | --- | --- |
| ARB | Writes `certificate.json`; `aar verify-certificate` replays against `state.json`. | Accepted terminal certificates expose exact replay, exact opportunity authority, filing-time evidence chronology, reachability, record integrity, a fixed initial evidence catalog, bounded length, decision-summary replay, and either closed outcome and due-process facts or failed-opportunity facts. |
| ADC | Writes `state.json` and `certificate.json` using `adc.replay-certificate.v1`; `adc verify-certificate` checks final-state hashes and replays deterministic steps and opportunity decisions.  An opportunity tool transition reruns `apply_decision`, compares its authorized action with the recorded executed step, and applies that step only after equality succeeds. | Accepted certificates expose exact replay, replay-start reachability, closed-terminal accounting, verdict facts, juror-failure verdict facts, juror-failure hung-jury facts, judgment facts, a combined outcome package, and concrete replayed examples.  The Lean replay relation includes direct steps and authority-checked `applyDecision` transitions. |
| AARD | Writes `state.json` and `certificate.json`; `aard verify-certificate` replays initialization and accepted actions. | Successful initialization and every reachable step preserve phase shape, council and answer integrity, record integrity, and fixed case data.  Progress results cover active merits and deliberation opportunities.  Every accepted action strictly decreases a budget bounded by twice the per-side evidence limit plus eight merits actions and the council size.  Accepted terminal certificates add exact replay, authority, chronology, bounded length, terminal accounting, answer-pair or failure-record replay, and checked examples. |

## Remaining Direction

Future proof work should support operational or adjudicative claims that the system already exposes.  The current branch does not need a new ARB removal-interleaving theorem.  ARB already exposes the relevant council-failure boundary through ordered accepted actions, current-round-voter protection, failure recording, and rule-governed continuation after failure.

| Area | Direction | Reason |
| --- | --- | --- |
| ARB council failure and removal | Defer a step-commutation theorem. | `ClosedCertificateFacts.closed_resolution_agrees_with_matched_case` covers the matched-state decision-rule claim.  Existing step theorems cover accepted failure recording, current-round-voter protection, vote preservation, seated-set shrinkage, and terminal outcome soundness.  A commutation theorem should wait until the runtime or API intentionally promises order independence. |

AARD covers initialized-run invariants, active-case opportunities, closed-case soundness, failure effects, and both terminal certificate forms.  ADC covers both verdict and hung-jury outcomes that derive from a deliberating-juror timeout.  ARB closed certificates carry the decision-rule package and expose a matched-case closed-resolution theorem: when another case has the same current-round vote multiset, seated count, and deliberation round, the executable closed-resolution summary agrees for the same required-vote and max-round values.

The matched-case theorem supports a narrow operational statement about removals: after the removal effects have been matched at the decision-rule inputs, ordering artifacts do not change the closed-resolution summary.  A step-level theorem for action order would require a different claim and tighter hypotheses.  If ARB later promises order independence, the theorem should specify the accepted action pair, distinct member ids, no current-round vote for the removed member, the same final seated set, and the same final current-round vote multiset.

## Current Limits

The proof surface verifies the executable procedure and stored records.  It does not prove that a lawyer searched well, that a council member reasoned well, or that the proposition is true.  Lean verifies the procedure, the certificate binds a packet to the engine transition sequence, and the record remains the source for human review of advocacy and evidence quality.
