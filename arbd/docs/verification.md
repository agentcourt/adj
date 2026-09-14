# Verification

## Scope

The AARD Lean library proves properties of the executable state transition functions in `engine/AARD/Core.lean`.  The proof tree covers initialization, exact action authority, procedural sequence, council and answer integrity, fixed case data, evidence-record integrity, progress, terminal outcomes, replay, and certificate facts.  `engine/Main.lean` implements the JSON protocol around that core.

All proof declarations belong to the `ArbdProofs` namespace.  `engine/Proofs.lean` imports every proof module directly.  The [theorem catalog](theorems.md) lists every theorem and lemma declaration, while the [proof statistics](proofstats.md) count all proof files and declarations.

## Initialized Runs

`InitializedRunInvariant` combines `ProcedureInvariant` with `initializedCaseFrame`.  The procedure component contains four properties:

| Property | Requirement |
|---|---|
| `phaseShape` | Each phase contains the exact completed and partial filing sequences allowed by the procedure. |
| `councilIdsUnique` | Council member identifiers remain unique. |
| `answerIntegrity` | Current-round answer owners are unique; every stored answer belongs to the current round, lies from 0 through 100, has a nonempty rationale, and belongs to a seated member. |
| `RecordIntegrity` | The catalog, submitted-evidence history, offers, lineage, and technical reports satisfy their recorded limits and reference rules. |

The frame component fixes the initialized case identifier, caption, trimmed question, policy, initial evidence catalog, and council identities.  Council status may change after a member failure, but each member's identifier, model, and persona filename remain fixed.

`initializeCase_establishes_runInvariant` proves that every successful initialization establishes this package.  `step_preserves_runInvariant` proves preservation by every successful public step.  `initializedRun_reachable_invariant` extends the result to every state connected to a successful initialization by `StepReachableFrom`.

## Authority and Record History

The engine computes one current opportunity from the source state.  An accepted action must carry the same opportunity identifier, state version, role, phase, and scheduled council member, and its operation must occur in the opportunity's allowed operation list.

`RecordIntegrity` validates the immutable initial catalog, each submitted-evidence prefix, accumulated offers, lineage, and report byte limits.  `MeritsOffersUsePriorRecord` adds the chronological condition: an argument, rebuttal, or surrebuttal may offer only evidence present before that filing step.  Replay preserves both the accumulated invariant and this source-state condition.

| Area | Principal declarations | Result |
|---|---|---|
| Exact opportunity authority | `step_ok_matches_currentOpportunity`, `authorizeAction_ok_matches_currentOpportunity` | Every accepted action matches the source state's current opportunity and uses an allowed operation. |
| Terminal rejection | `closed_step_rejected`, `failed_step_rejected` | Closed and failed cases reject later public steps. |
| Record initialization | `initializeCase_establishes_recordIntegrity_and_catalog` | Initialization establishes record integrity and copies the initial catalog unchanged. |
| Record preservation | `step_preserves_recordIntegrity_and_catalog`, `reachable_recordIntegrity` | Accepted steps and reachable states retain record integrity and the initialized catalog. |
| Filing chronology | `step_ok_meritsOffersUsePriorRecord`, `replayInitialized_success_meritsOfferChronology` | Merits offers resolve against evidence present in the action's source state. |
| Full invariant | `initializeCase_establishes_runInvariant`, `step_preserves_runInvariant`, `initializedRun_reachable_invariant` | Initialization establishes the procedural and frame invariants, and every accepted run preserves them. |

## Progress and Outcomes

The progress results concern states that satisfy the initialized-run invariants.  `merits_phase_has_currentOpportunity` proves that an active state in openings, arguments, rebuttals, surrebuttals, or closings has a current opportunity.  `deliberation_has_currentOpportunity` proves the same result while an eligible council member remains unanswered.  `accepted_step_has_currentOpportunity` derives the exact source opportunity and its authorization facts from any successful public step.

The remaining-step functions count open merits positions, available evidence submissions, and unanswered seated council members.  `remainingStepBudget_finite_bound` bounds their sum by eight merits positions, twice the per-side evidence-submission limit, and the configured council size.  The eight positions include the optional rebuttal and surrebuttal.

`step_decreases_remainingStepBudget` proves strict decrease under every accepted public action from a state satisfying `ProcedureInvariant`.  `replaySteps_length_add_budget_le` bounds the accepted action count plus the final budget by the initial budget.  `replayInitialized_length_bound` therefore bounds every successful initialized run by `2 × max_submitted_evidence_per_side + 8 + council_size` actions.  `no_infinite_initialized_run` excludes an infinite sequence of accepted actions.  These bounds concern accepted Lean transitions.  Model-request retries, invalid submissions, work notes, evidence reads, and elapsed time remain governed by runtime limits.

`initialized_run_closed_case_sound` proves that a reachable initialized state in the closed phase has complete merits, unique council identifiers, valid answers, record integrity, and the initialized case frame.  `continueDeliberation_closes_only_with_complete_answers` proves that the deliberation continuation function enters the closed phase only when the current-round answer count equals the seated-member count.

`failOpportunity_success_effect` classifies every successful failure transition.  A party failure creates the recorded case-level failure state.  A council failure delegates to `failCouncilMemberOpportunity` for the scheduled unanswered member.

## Replay Certificates

The certificate schema is `aard.replay-certificate.v1`.  Its initialization request contains the source state, question, and council roster.  Each recorded action contains its operation, actor role, exact source-state authority, and payload.  The certificate also contains the claimed final state.

`checkReplayCertificate_ok_iff` states that Lean accepts a certificate exactly when initialization followed by the recorded actions produces the claimed state.  Certificate acceptance implies authority conformance, source-state offer chronology, reachability, record integrity, fixed initial catalog, `InitializedRunInvariant`, and the accepted-action bound.  Both terminal certificate structures include `bounded_length`.  Closed certificates preserve the answer pairs, while failed certificates preserve the opportunity-failure record.

The concrete certificate examples cover one closed case with three council answers and one case-level plaintiff opportunity failure.  They evaluate the executable certificate predicate and instantiate the corresponding fact structures.

## Operational Verification

`aard verify-certificate` accepts the v1 schema and procedure `aard`.  It requires one nonblank case identifier across the certificate, initialization state, claimed state, packet `state.json`, and replayed state.  It validates each recorded authority and source version, replays initialization and every action through the configured Lean executable, requires a terminal status, and compares the resulting JSON state with the claimed and packet states.

The command executes the same Lean initialization and step functions used during a case.  It does not expose `checkReplayCertificate` or return theorem values through the protocol.  It also does not inspect work notes, events, council snapshots, transcript files, or evidence bytes.  Evidence-file custody therefore requires a separate comparison of the manifest commitments with `evidence-store/`.

## Limits

The proofs establish properties of Lean state, transitions, replay, and recorded commitments.  They do not establish the truth of the case question, the quality of a lawyer's work, or an aggregate derived from the independent council answers.  Evidence lineage proves the recorded parent relationship and digest agreement; it does not prove that child bytes result from the stated transformation.
