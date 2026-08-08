# Proof Work Status

This note records the proof agenda after the July 2026 certificate and ARB proof work.  It supersedes the earlier ARB proof review that treated realisability, maximal-run terminal accounting, opportunity agreement, and certificate replay as open items.  The current branch has those results in Lean, so the current theorem surface is the starting point for the next proof pass.

## Current ARB Surface

ARB is the most complete proof target.  Its proof library has 38 proof files, 665 theorem or lemma declarations, and 21,297 lines according to `arb/docs/proofstats.md`.  A targeted scan found no `sorry`, no `axiom` declarations, and no `unsafe` declarations in the ARB, ADC, or AARD engine proof trees; the word `admit` appears only in prose.

| Area | Anchor theorem or file | Status |
| --- | --- | --- |
| Reachability | `Reachable`, `StepReachableFrom` | Executions are modeled as successful initialization followed by successful public steps. |
| Phase order and parity | `reachable_phaseShape`, `reachable_proceduralParity` | Merits filings preserve the required order and side-to-side parity. |
| Case frame | `initialized_run_preserves_caseFrame` | Proposition, policy, and council identity are fixed across a run. |
| Material limits and provenance | `reachable_materialLimitsRespected`, `reachable_recordProvenance`, `stepReachableFrom_materialsExtend` | Admitted materials respect caps, have allowed origins, and grow by suffix. |
| Outcome soundness | `reachable_closed_demonstrated_sound`, `reachable_closed_not_demonstrated_sound`, `reachable_closed_no_majority_sound` | Closed outcomes follow from current-round votes, the threshold, and executable closure conditions. |
| Liveness and realisability | `reachable_nonclosed_has_nextOpportunity`, `reachable_active_has_successful_step` | Reachable live states expose an opportunity and admit at least one successful public action. |
| Maximal paths | `initializedStepPathMaximal_terminal_accounted` | A maximal successful path from initialization ends closed with an enumerated resolution or failed with an accounted party-opportunity failure. |
| Opportunity agreement | `accepted_actor_action_matches_current_opportunity` | Accepted actor-facing actions match the advertised role and allowed-tool list. |
| Replay certificates | `checkReplayCertificate_terminal_facts` | Accepted terminal replay certificates expose exact replay, reachability, bounded length, decision-summary replay, and closed or failed terminal facts. |
| Failure resilience | `step_fail_opportunity_same_round_resilience` | Same-round opportunity failure preserves stored council votes and does not create a substantive result once no substantive outcome remains viable. |
| Decision rule | `DecisionRuleFacts`, `DecisionRuleCharacterization` | The executable threshold rule has permutation invariance, neutrality, quota monotonicity, and a count-level characterization. |
| Due process | `reachable_status_closed_merits_complete`, certificate due-process facts | Closed cases and accepted closed certificates carry the ordered mandatory merits filings and filing-count facts. |

## Certificate Work

The term "certificate" in these systems names a package of a run's input, its accepted-action record, and its claimed final state, bound together by hashes.  The package carries no signature and no endorsement, and verification is recomputation: the verifier replays the recorded actions through the engine and compares the result with the claim.  The word is borrowed from complexity theory, where a certificate is a witness that makes a claim checkable without search; the check here re-executes every engine transition, and it saves work only because the recorded actions remove any search and the model calls are not repeated.  Readers who expect the ordinary sense of an authority's attestation will be misled.  A passing package shows the claimed outcome follows from the recorded history under the rules; whether the recorded history is what actually happened is outside its scope and is covered, when needed, by attested execution.

The certificate plan has been carried across all three current adjudication procedures.  ARB remains the reference implementation because its proof package is deepest, but ADC and AARD now have runtime certificates, explicit verifier commands, service artifact exposure, and Lean replay facts.  Services list and fetch certificate artifacts; they do not run replay verification during case creation, listing, polling, or artifact reads.

| System | Runtime boundary | Proof boundary |
| --- | --- | --- |
| ARB | Writes `certificate.json`; `aar verify-certificate` replays against `state.json`. | Accepted terminal certificates expose exact replay, reachability, bounded length, closed outcome soundness, decision-rule facts, due-process facts, decision-summary replay, or failed-opportunity facts. |
| ADC | Writes `state.json` and `certificate.json`; `adc verify-certificate` checks final-state hashes and replays accepted transitions. | Accepted certificates expose exact replay, replay-start reachability, closed-terminal accounting, verdict facts, juror-failure verdict facts, juror-failure hung-jury facts, judgment facts, a combined outcome package, and concrete replayed examples. |
| AARD | Writes `state.json` and `certificate.json`; `aard verify-certificate` replays initialization and accepted actions. | Accepted terminal certificates expose exact replay, reachability, terminal accounting, closed answer-pair replay, failed-case failure-record replay, and checked closed and failed examples. |

## Remaining Direction

Future proof work should support operational or adjudicative claims that the system already exposes.  The current branch does not need a new ARB removal-interleaving theorem.  ARB already exposes the relevant council-failure boundary through ordered accepted actions, current-round-voter protection, failure recording, and rule-governed continuation after failure.

| Area | Direction | Reason |
| --- | --- | --- |
| ARB council failure and removal | Defer a step-commutation theorem. | `ClosedCertificateFacts.closed_resolution_agrees_with_matched_case` covers the matched-state decision-rule claim.  Existing step theorems cover accepted failure recording, current-round-voter protection, vote preservation, seated-set shrinkage, and terminal outcome soundness.  A commutation theorem should wait until the runtime or API intentionally promises order independence. |

AARD now covers the current certificate report boundary for both terminal shapes.  ADC now covers both verdict and hung-jury outcomes that derive from a deliberating-juror timeout.  ARB closed certificates now carry the existing decision-rule package and expose a matched-case closed-resolution theorem: when another case has the same current-round vote multiset, seated count, and deliberation round, the executable closed-resolution summary agrees for the same required-vote and max-round values.

The matched-case theorem supports a narrow operational statement about removals: after the removal effects have been matched at the decision-rule inputs, ordering artifacts do not change the closed-resolution summary.  A step-level theorem for action order would require a different claim and tighter hypotheses.  If ARB later promises order independence, the theorem should specify the accepted action pair, distinct member ids, no current-round vote for the removed member, the same final seated set, and the same final current-round vote multiset.

## Current Limits

The proof surface verifies the executable procedure and stored records.  It does not prove that a lawyer searched well, that a council member reasoned well, or that the underlying proposition is true.  Those limits are appropriate for the current system: Lean verifies the procedure, the certificate binds a packet to the engine transition sequence, and the record remains the source for human review of advocacy and evidence quality.
