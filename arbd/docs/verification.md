# Verification

## Scope

The AARD Lean library proves properties of the executable degree-arbitration engine over initialized and reachable states.  Its selected theorem catalog emphasizes exact action authority, record integrity, filing-time evidence chronology, replay, terminal certificate facts, and concrete closed and failed certificates.  The generated proof statistics count every theorem and lemma declaration in the proof tree, including internal preservation and parsing lemmas omitted from the selected catalog.

## Principal Results

The public `step` theorem binds each accepted action to the opportunity computed from its source state.  Record-integrity theorems establish the initial evidence catalog, validate ordered submissions and lineage, resolve offers, enforce report byte limits, and preserve the catalog through successful runs.  Replay theorems carry the same authority and source-state chronology across the certificate action list.

| Area | Principal declarations | Result |
|---|---|---|
| Exact opportunity authority | `step_ok_matches_currentOpportunity`, `authorizeAction_ok_matches_currentOpportunity` | An accepted action supplies the current opportunity id, source state version, role, phase, scheduled member, and an operation allowed by that opportunity. |
| Terminal rejection | `closed_step_rejected`, `failed_step_rejected` | Closed and failed cases reject later public steps. |
| Record initialization | `initializeCase_establishes_recordIntegrity_and_catalog` | Initialization validates the evidence catalog and starts a state satisfying `RecordIntegrity`. |
| Step preservation | `step_preserves_recordIntegrity_and_catalog` | Every accepted public action preserves record integrity and the exact initial catalog. |
| Reachable record | `reachable_recordIntegrity`, `initialized_run_preserves_evidenceCatalog` | Reachable states retain valid ordered evidence records and the initialized catalog. |
| Source-state offer rule | `step_ok_meritsOffersUsePriorRecord` | Each accepted argument, rebuttal, or surrebuttal resolves offers against the catalog and submissions present before the filing. |
| Replay chronology | `replaySteps_success_meritsOfferChronology`, `replayInitialized_success_meritsOfferChronology` | Successful replay preserves the source-state offer rule at every merits action. |
| Exact replay | `checkReplayCertificate_ok_iff` | Lean certificate acceptance is equivalent to initialized replay producing the claimed state exactly. |
| Accepted certificate facts | `checkReplayCertificate_ok_authorityConforming`, `checkReplayCertificate_ok_meritsOfferChronology`, `checkReplayCertificate_ok_recordIntegrity` | Acceptance implies exact authority, filing-time chronology, reachability, record integrity, and catalog equality. |
| Terminal certificate packages | `checkReplayCertificate_status_closed_facts`, `checkReplayCertificate_status_failed_facts`, `checkReplayCertificate_terminal_facts` | Accepted terminal certificates yield either closed answer-pair facts or failed opportunity-record facts. |
| Concrete certificates | `sample_closed_certificate_facts`, `sample_failed_certificate_facts` | Executable closed and failed examples satisfy their complete formal fact packages. |

## Record Integrity and Chronology

`RecordIntegrity` requires a valid initial catalog, a prefix-valid submitted-evidence history, resolvable accumulated offers, and technical reports within their configured UTF-8 byte limits.  A submitted identifier cannot collide with the catalog or an earlier submission, and a derived item identifies an initial or earlier parent by identifier and SHA-256 commitment.  Successful initialization establishes the predicate, every accepted action preserves it, and every reachable state therefore satisfies it.

Accumulated-state integrity does not establish when an offer first became valid, because a later submission appears in the final record.  `MeritsOffersUsePriorRecord` evaluates one accepted argument, rebuttal, or surrebuttal against the catalog and submitted-evidence list in that action's source state.  `MeritsOfferChronology` carries that property through replay, preventing a later submission from supplying the reference for an earlier filing.

## Replay Certificates

The certificate schema is `aard.replay-certificate.v1`.  Its initialization request carries the AARD question, state, and council members, while each action carries its exact opportunity authority and payload.  The claimed final state preserves AARD's numeric `CouncilAnswer`, whose `answer` field is a natural number and whose executable admission rule accepts only values from 0 through 100.

`checkReplayCertificate_ok_iff` states that Lean acceptance holds exactly when initialization followed by the recorded public actions produces the claimed state.  The accepted-certificate theorems then derive action authority, source-state offer chronology, reachability, final-state record integrity, and fixed-catalog equality.  Closed facts also replay the member-answer pairs, while failed facts replay the stored opportunity-failure record and establish a terminal failed state.

The concrete closed certificate records answers 72, 55, and 18 for members C1, C2, and C3.  The concrete failed certificate records a plaintiff failure at the opening opportunity with `failure_type` equal to `opportunity_failed`.  Both examples prove certificate acceptance and instantiate their respective terminal fact structures.

## Operational Boundary

The Go `aard verify-certificate` command validates the v1 schema and procedure, hashes the claimed final state, compares that hash with `state.json`, replays initialization and every action through the configured Lean engine, and hashes the replayed state.  Lean's `checkReplayCertificate` is the formal executable predicate, and successful evaluation establishes exact replay, authority, chronology, reachability, record integrity, and catalog equality.  The closed and failed fact packages require the corresponding claimed-state status premise.  The Go verifier separately requires the replayed status to be `closed` or `failed`.  It replays and hashes without invoking the Lean theorem as an exported proof checker or returning the derived formal facts as protocol data.

Go preserves the current failure distinction during execution and replay.  A lawyer opportunity failure terminates the case with a structured failure record, while a council-member failure removes that member and permits the remaining seated council to continue when the policy allows it.  The failed certificate theorem concerns a terminal case-level opportunity failure and proves replay agreement for the stored record.

## Limits

The proofs cover the Lean state, executable transitions, recorded commitments, and certificate implications.  Certificate verification hashes JSON states and replays actions, but it does not rehash the files stored under `evidence-store/` or prove that a child's bytes implement its stated derivation method.  The library also does not establish the truth of the degree question, the adequacy of advocacy, or a single aggregate derived from the independent 0–100 member answers.
