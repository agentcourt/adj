# Development Notes

## 2026-08-05: Concurrent fake-engine execution

Parallel council tests intermittently received `ETXTBSY` when they executed shell files created in temporary directories.  The Go standard library's Linux `os/exec` test documents the cause in [issue 22315](https://go.dev/issue/22315): a concurrent fork can inherit the writable descriptor before its close-on-exec flag takes effect.  The council fixtures now pass their fake engine program to `/bin/sh -c`, eliminating the generated executable while preserving a subprocess that reads the engine request from standard input.

Both ARB and AARD passed 100 repetitions of the two parallel council-removal tests.  The complete core Go suite, build, and vet checks also pass.  Commit `ef540ff` contains the symmetric fixture correction.

## 2026-08-02: Acceptance example reduction

ARB retains `examples/ex01` as its command-line acceptance case.  The removed examples duplicated the same input interface or preserved run-specific market research and source captures.  The retained example now documents validation, one-case execution, output records, and certificate verification.

The complete Go test, build, and vet checks pass after the reduction.  `aar validate --complaint examples/ex01/complaint.md` accepts the retained complaint and its case-file references.  A live case continues to require the model credentials documented by the example.

The procedure-local `pool.jsonl` and `personas/generic.md` were byte-identical to the retained files under `common/`.  ARB now uses the same shared defaults as ADC and AARD, while an explicit `--council-pool` or a caller-supplied local pool still takes precedence.  Three unlinked Mermaid files also left `arb/analysis` because they described obsolete resolution labels and prohibited surrebuttal evidence that the current procedure accepts.

## 2026-08-02: Core reduction

The unexposed council replay implementation and its operator document were removed.  The live runtime still writes council-turn snapshots as part of the durable record, so the snapshot writer and shared JSON helpers now live in core-specific files.  The complete Go test identified that shared dependency before the removal was committed.

The attorney prompt now describes a service-independent private journal path and the case-owned Role API.  Core process, failure, evidence, council, and practice documents no longer describe Clerk, MCP, OpenClaw, or Pi operation.  The obsolete pool snapshot, wrapper script, MCP plans, and authentication notes were also removed.

Verification completed with `go test -buildvcs=false -count=1 ./...`, `go build -buildvcs=false ./...`, `go vet ./...`, command help checks, and `git diff --check`.  The retained tests cover direct and external lawyer work, council work, evidence custody, records, and certificate replay.  The snapshot schema remains `aar.council-turn-snapshot.v0`.

## 2026-08-02

### Lean 4.32.0

Reference: [Lean 4.32.0 release notes](https://lean-lang.org/doc/reference/latest/releases/v4.32.0/)

The ARB Lean engine now selects Lean 4.32.0.  Lean 4.32.0 makes the new `do` elaborator the default and exposes reductions that the earlier proof build discharged implicitly.  The revised proofs reduce those monadic operations explicitly and use field-preservation lemmas when a record update leaves the vote summary, council count, or termination measure unchanged.

Verification:

- [x] `lean --version` reports Lean 4.32.0.
- [x] `lake build` completes all 45 ARB engine and proof jobs.

### Core and service branch split

Reference: [Core and service branch plan](https://github.com/jsmorph/adjudication/blob/carve/plan.md)

The `carve` branch retains ARB's Lean engine, proofs, one-case Go runtime, case-owned Lawyer and Council APIs, durable records, and certificate verification.  The `service` branch receives the multi-case Clerk service, MCP adapter, local OpenClaw and Pi launchers, agent templates, attested execution, Docker deployment, web programs, and their operational support.  The branches communicate through documented executable, HTTP, and artifact interfaces, with tested commit pairs recording compatibility.

Service commits `4b4fa1751fa4b8a1e709b3f80ad1cbcbc6eaa581` and `6eaed038b468add7099b77edb766b987ba053dcd` contain the extracted ARB multi-case service and MCP adapter.  Service commit `19b9254442e90c25c6cac21460d80eadb04ba7f3` contains `aar-run`, the local-agent launcher, its templates, and the paired process-interface test.  Service commit `48d19263fde43f010312cb446cd4d6970a019c4f` retains the Clerk and MCP cases formerly mixed into the ARB command black-box tests and passes them against the real `carve` executable and Lean engine.  The direct one-case failure tests remain on `carve`, while the service and MCP variants can now leave the core command package.

### Core command black-box boundary

The ARB command package now contains only the direct black-box cases for lawyer attempt exhaustion, lawyer deadline expiration, and runtime failure exit behavior.  The service and MCP cases and their process helpers left `carve` after service commit `48d19263fde43f010312cb446cd4d6970a019c4f` preserved and ran them.  The retained fixture reports provider I/O, HTTP body closure, process-log closure, and process-cleanup errors.

Verification:

- [x] `go test -buildvcs=false -count=1 -run '^TestBlackBox' ./arb/runtime/cmd/aar`
- [x] `go vet -buildvcs=false ./arb/runtime/cmd/aar`

### Multi-case service removal

The ARB multi-case service package and `aar service` dispatch entry have left `carve` after service commits `4b4fa1751fa4b8a1e709b3f80ad1cbcbc6eaa581` and `48d19263fde43f010312cb446cd4d6970a019c4f` preserved the service-owned implementation and cross-branch tests.  The ARB root help now lists the retained core and temporarily duplicated adapter commands without advertising the Clerk service.  The command package passes its tests and builds without an import of the removed service package.

Verification:

- [x] `go test -buildvcs=false -count=1 ./arb/runtime/cmd/aar`
- [x] `go test -buildvcs=false -count=1 ./arb/runtime/...`
- [x] `go vet -buildvcs=false ./arb/runtime/...`
- [x] `go build -buildvcs=false -o /tmp/carve-service-removal-bins/aar ./arb/runtime/cmd/aar`

### Operational adapter removal

The ARB `run` and `mcp` commands, their runtime packages, and their OpenClaw and Pi templates have left `carve`.  Service commits `6eaed038b468add7099b77edb766b987ba053dcd` and `19b9254442e90c25c6cac21460d80eadb04ba7f3` retain the standalone adapter, launcher, embedded templates, package tests, and paired core-process checks, while `48d19263fde43f010312cb446cd4d6970a019c4f` retains a complete MCP case.  The `council-replay` and `juror-replay` experiments also left the command because the approved service boundary assigns them no operational use.

Verification:

- [x] `go test -buildvcs=false -count=1 ./arb/runtime/...`
- [x] `go vet -buildvcs=false ./arb/runtime/...`
- [x] `go build -buildvcs=false -o /tmp/carve-core-only-bins/aar ./arb/runtime/cmd/aar`

### Case-packet ownership

Deterministic ARB case-packet construction remains on `carve` as part of the core input interface.  The builder uses the proceeding package's automatic and explicit case-file selection rules, preserving one implementation of input validation.  Attested service code will invoke the installed `aar case-packet` command without importing or copying that implementation.

### Core and service split: attested execution

Service commit `dc7c61d61b478dd2bf24fdc7e1e4924d80b37443` moved the ARB attested images, entrypoint, drivers, scripts, tests, and runbooks under `service/attested/arb`.  Its local driver invokes the installed core `aar case-packet` command, while its image builds core and service from full commit IDs.  The image built against `carve@a4cc40d99899721d957c4f370040998306f771e9`, displayed the service `aar-run` help, and validated the embedded `ex01` complaint through the installed core executable.

The core branch retains `aar case-packet`, its deterministic proceeding builder, `aar case`, the case-owned Lawyer and Council APIs, durable records, and certificate verification.  The ARB Dockerfiles, exec entrypoint, attested Python and shell programs, and operational runbooks have left this branch.  The core README and documentation index no longer link to those removed files.

Verification:

- [x] `go test -buildvcs=false -count=1 ./arb/runtime/...`
- [x] `go vet ./arb/runtime/...`
- [x] `go build -buildvcs=false -o /tmp/carve-aar-attested-removal ./arb/runtime/cmd/aar`
- [x] The freshly built command listed only `case`, `case-packet`, `complain`, `validate`, and `verify-certificate`.
- [x] The freshly built `aar case-packet` created and listed a deterministic packet from `arb/examples/ex01/complaint.md`.

## 2026-07-13

### Runtime replay certificates

Reference: `runtime/proceeding/certificate.go`, `runtime/cmd/aar/verify_certificate.go`, `runtime/service/service.go`, `manual.md`

ARB now writes `certificate.json` in terminal output packets.  The certificate records the Lean initialization request, every accepted public action, the claimed final state, and the SHA-256 hash of the compact final-state JSON.  The new `aar verify-certificate` command reads the certificate and `state.json`, replays the certificate through the Lean engine, and requires the certificate hash, packet final-state hash, and replayed final-state hash to agree.

The runtime records actions only after the engine accepts a step, so stale opportunities and rejected tool calls do not enter the certificate.  The service artifact allowlist now includes `certificate.json`, allowing direct service and Clerk artifact routes to list and fetch the file when the run wrote it.  Services do not verify certificates automatically; operators run `aar verify-certificate` when they want the replay check.  Focused verifier tests now cover certificate hash mismatch, packet-state mismatch, missing accepted actions, rejected replay actions, altered action payloads, and successful replay.  A live `examples/ex01` Council API run at `/tmp/aar-cert-live-ex01.C559OC` wrote a 13-action certificate and verified successfully with the rebuilt local CLI.

Verification:

- [x] `go test ./arb/runtime/proceeding`
- [x] `go test ./arb/runtime/cmd/aar`
- [x] `go test ./arb/runtime/service`
- [x] `./.bin/aar verify-certificate --dir /tmp/aar-cert-live-ex01.C559OC --engine .bin/aarengine`

### Certificate and failure proof packaging

Reference: `engine/Proofs/CertificateFacts.lean`, `engine/Proofs/ProgressViability.lean`

The closed-certificate fact package now has resolution-specific accessors for demonstrated, not-demonstrated, and no-majority soundness.  A caller that has `ClosedCertificateFacts` and the recorded resolution can extract the matching soundness theorem directly, without redoing the disjunction split.  The package also carries `DecisionRuleFacts` for the claimed closed state, reusing the reachability fact already established by accepted certificate replay.  `ClosedCertificateFacts.closed_resolution_agrees_with_matched_case` exposes the matched-state decision-rule theorem at the certificate boundary: same current-round vote multiset, seated count, and deliberation round give the same closed-resolution summary for the same required-vote and max-round values.  The package still rests on exact initialized replay and does not change certificate acceptance.

The certificate package now covers failed terminal packets.  `FailedCertificateFacts` packages exact initialized replay, reachability, the initialized action-length bound, failed status, the recorded `opportunity_failed` object with a plaintiff or defendant role and phase, and decision-summary replay.  `checkReplayCertificate_terminal_facts` exposes the terminal boundary as closed facts or failed facts.

The same-round failure package now records the public `fail_opportunity` consequences needed for the council-failure story.  A successful same-round failure step preserves the stored council vote list, preserves no-substantive-outcome viability, and blocks a new substantive current resolution under that premise.  This packages existing viability and progress facts at the public step boundary.

Verification:

- [x] `lake build Proofs.CertificateFacts`
- [x] `lake build Proofs.CertificateFacts Proofs.DecisionRuleFacts Proofs.DecisionRuleCharacterization`
- [x] `lake build Proofs.ProgressViability`
- [x] `lake build Proofs`

### Active-step realizability proof

Reference: `engine/Proofs/Realizability.lean`, `engine/Proofs.lean`, `engine/Main.lean`, `engine/Proofs/NoStuck.lean`

ARB now has a proof that every reachable active state admits at least one successful public `step`.  The proof composes existing reachability invariants with executable witness actions: a one-character merits filing in merits phases, pass actions for optional rebuttal phases, and a council vote for deliberation using the next seated member.  It records the additional invariants needed for those witnesses: validated text limits remain positive, initialized council member IDs are nonempty and trimmed, and those facts persist across successful steps.

Verification:

- [x] `lake build Proofs.Realizability`
- [x] `lake build Proofs`

### Maximal-run terminal proof

Reference: `engine/Proofs/MaximalRuns.lean`, `engine/Proofs/Realizability.lean`, `engine/Proofs/TerminalStates.lean`, `engine/Proofs/BoundedTermination.lean`

ARB now has a run-level theorem for maximal successful public paths.  `StepPathMaximal` combines the existing indexed successful-run relation with the absence of any further successful public `step`.  The terminal theorem proves that any maximal path from a reachable state ends with either a closed case whose phase is closed and whose resolution is one of `demonstrated`, `not_demonstrated`, or `no_majority`, or a failed case with an `opportunity_failed` record identifying the party role and phase.

The proof adds a status invariant for reachable states: a reachable case is active, closed with `phase = "closed"`, or failed.  The maximal result then uses `reachable_active_has_successful_step` to rule out an active endpoint, and it reuses the existing terminal-state facts for closed and failed endpoints.

Verification:

- [x] `lake build Proofs.MaximalRuns`
- [x] `lake build Proofs`

### Surrebuttal opportunity tool list

Reference: `engine/Main.lean`, `engine/Proofs/ProcedureShape.lean`, `engine/Proofs/NoStuck.lean`, `manual.md`

The opportunity API now lists `submit_evidence` during the defendant's surrebuttal opportunity.  The engine already accepted submitted evidence during arguments, rebuttals, and surrebuttals, and the manual already described that behavior.  The opportunity spec now matches the accepted actor-facing action set for that phase.

Verification:

- [x] `lake build Proofs.ProcedureShape`
- [x] `lake build Proofs.NoStuck`
- [x] `lake build Proofs`

### Opportunity allowed-tool agreement

Reference: `engine/Proofs/OpportunityAgreement.lean`, `engine/Proofs/MaximalRuns.lean`, `engine/Proofs/NoStuck.lean`, `engine/Main.lean`

ARB now proves that every accepted actor-facing public action appears in the current `nextOpportunity` allowed-tool list.  The theorem covers reachable active states and excludes the two system actions, `remove_council_member` and `fail_opportunity`, because those are not advertised as actor tools.  The proof uses the existing reachable liveness theorem to obtain the current opportunity, derives the accepted source phase from the successful step, and then checks the phase-specific allowed-tool list.

Verification:

- [x] `lake build Proofs.OpportunityAgreement`
- [x] `lake build Proofs`

### Opportunity actor-role agreement

Reference: `engine/Proofs/OpportunityAgreement.lean`, `engine/Main.lean`

ARB now proves that every accepted actor-facing public action from a reachable active state uses the actor role advertised by the current `nextOpportunity`.  The proof covers merits filings, submitted evidence, optional phase passes, and council votes.  It extracts the role accepted by each `stepCore` branch and matches it against the role returned by the corresponding `nextOpportunityForPhase` branch.  The file also exposes a combined theorem that packages the role match with the allowed-tool match.

Verification:

- [x] `lake build Proofs.OpportunityAgreement`
- [x] `lake build Proofs`

### Replay certificate foundation

Reference: `engine/Proofs/Replay.lean`, `engine/Proofs.lean`, `engine/Proofs/Reachability.lean`, `engine/Proofs/BoundedTermination.lean`

ARB now has a proof-local replay function over lists of public actions.  The replay theorems connect successful replay to `StepReachableFrom`, `Reachable`, and indexed `StepPath`, and they prove that any existing `StepReachableFrom` or `StepPath` has a replaying action list.  Successful replay from initialization also inherits the explicit run-length bound from bounded termination.  This gives the certificate work a small foundation without changing the runtime API or the engine transition function.

Verification:

- [x] `lake build Proofs.Replay`
- [x] `lake build Proofs`

### Initialized replay certificate wrapper

Reference: `engine/Proofs/Replay.lean`, `engine/Proofs/MaximalRuns.lean`

ARB now has a proof-local `replayInitialized` checker that runs `initializeCase` and then replays a public action list.  A successful initialized replay exposes its initialized start state and step replay, proves the final state reachable, yields a `StepPath` with length equal to the action-list length, inherits the initialized length bound, and accounts for terminal endpoints.  Blocked endpoints reuse the maximal-run terminal theorem; endpoints already known to have closed or failed status reuse the reachable terminal-state theorems.

Verification:

- [x] `lake build Proofs.Replay`
- [x] `lake build Proofs`

### Replay certificate acceptance theorem

Reference: `engine/Proofs/Replay.lean`

ARB now has a proof-local `checkReplayCertificate` checker for an initialization request, public action list, and claimed final state.  The main theorem proves that the checker accepts exactly when initialized replay produces the claimed state.  Accepted certificates inherit reachability, an indexed `StepPath`, the initialized length bound, blocked-endpoint terminal accounting, and terminal-state accounting for closed or failed claimed states.

Verification:

- [x] `lake build Proofs.Replay`
- [x] `lake build Proofs`

### Certificate outcome soundness

Reference: `engine/Proofs/CertificateSoundness.lean`, `engine/Proofs/Replay.lean`, `engine/Proofs/OutcomeSoundness.lean`

Accepted replay certificates now inherit the outcome-soundness theorems at the certificate boundary.  A certificate for a closed `demonstrated` state has the demonstrated vote threshold in the final current round, a certificate for a closed `not_demonstrated` state has the not-demonstrated threshold, and a certificate for `no_majority` has the recorded no-majority conditions.  The file also packages the substantive-outcome case as one theorem, so failures or missing votes cannot manufacture either substantive result in an accepted certificate.

Verification:

- [x] `lake build Proofs.CertificateSoundness`
- [x] `lake build Proofs`

### Same-round impossibility preservation

Reference: `engine/Proofs/ProgressViability.lean`, `engine/Proofs/ViableOutcomesCore.lean`

Same-round deliberation progress now exposes the operational consequence of the existing viability-shrink theorem: once neither substantive outcome remains viable, later same-round progress still has no current substantive resolution.  This packages the failure-resilience point in executable terms, because same-round votes and removals cannot turn an impossible substantive outcome back into `demonstrated` or `not_demonstrated`.

Verification:

- [x] `lake build Proofs.ProgressViability`
- [x] `lake build Proofs`

### Council failure resilience

Reference: `engine/Proofs/ViableOutcomes.lean`, `engine/Proofs/ProgressViability.lean`

The same-round failure theorem now covers `fail_opportunity` at the public `step` boundary.  A council opportunity failure removes an unvoted seated member and then runs `continueDeliberation`, while party failure leaves the deliberation summary unchanged.  In either branch, if no substantive outcome was viable before the step and the step stays in the same deliberation round, no substantive outcome becomes viable afterward.

Verification:

- [x] `lake build Proofs.ViableOutcomes`
- [x] `lake build Proofs.ProgressViability`
- [x] `lake build Proofs`

### Vote-order invariance foundation

Reference: `engine/Proofs/VoteOrder.lean`, `engine/Proofs.lean`

ARB now proves the pure counting foundation for vote-order invariance.  `voteCountFor` is invariant under `List.Perm`, and `currentResolution?` is unchanged when two cases have permuted current-round vote lists.  This isolates the aggregation fact from the engine transition path: the current resolution depends on vote counts, not list order.

The proof now lifts that result to `deliberationSummaryForCase` when the seated count and deliberation round match.  Those hypotheses account for the summary fields that do not come from the current-round vote list.  The closure wrapper proves that if the source summary closes with a resolution, `continueDeliberation` closes the permuted destination case with the same resolution.

Verification:

- [x] `lake build Proofs.VoteOrder`
- [x] `lake build Proofs`

### Threshold monotonicity

Reference: `engine/Proofs/ThresholdMonotonicity.lean`, `engine/Proofs.lean`

ARB now proves quota monotonicity for current-round resolutions.  A `demonstrated` current resolution at a higher required-vote quota remains `demonstrated` at a lower quota, and a missing current resolution at a lower quota remains missing at a higher quota.  The `not_demonstrated` theorem includes the rule's ordering caveat: lowering the quota preserves `not_demonstrated` only when the demonstrated count still falls below the lower quota.

The same file now proves the no-majority side for summary closure.  If a case summary closes as `no_majority` at a lower required-vote quota, the same current votes, seating, and round also close as `no_majority` at a higher quota.  The proof factors through two smaller facts: higher quotas preserve the no-substantive-outcome condition, and higher quotas preserve the no-majority closure reason.

Verification:

- [x] `lake build Proofs.ThresholdMonotonicity`
- [x] `lake build Proofs`

### Closed-case due process package

Reference: `engine/Proofs/DueProcess.lean`, `engine/Proofs.lean`

ARB now packages the closed-case merits sequence as a direct theorem over reachable states.  If a reachable case is closed, its openings, arguments, and closings each contain one plaintiff filing and one defendant filing in the engine's required order.  Rebuttal remains plaintiff-only and optional, and surrebuttal remains defendant-only and optional.

Accepted replay certificates now inherit the same ordered merits package and filing-count package for closed claimed states.  The certificate theorems have both phase-closed and status-closed entry points, with the latter deriving the closed phase from existing reachable terminal-state facts.  This puts the due-process fact at the certificate boundary alongside outcome soundness, so a checked closed packet carries both its vote justification and its completed merits sequence.

Verification:

- [x] `lake build Proofs.DueProcess`
- [x] `lake build Proofs.CertificateSoundness`
- [x] `lake build Proofs`

### Decision summary projection

Reference: `engine/Proofs/DecisionSummary.lean`, `engine/Proofs.lean`

ARB now has a proof-side `DecisionSummary` projection for decision-relevant fields: terminal labels, resolution, policy quota, round cap, current-round vote counts, seating, and deliberation round.  The replay certificate checker remains exact: it still accepts only when replay reproduces the claimed final state.  The new certificate theorem proves that an accepted certificate's decision summary is produced by exact initialized replay, and the permutation theorem proves that matching structural fields and permuted current-round votes yield the same decision summary and the same summary-side closed-resolution value.

Verification:

- [x] `lake build Proofs.DecisionSummary`
- [x] `lake build Proofs`

### Closed-certificate fact package

Reference: `engine/Proofs/CertificateFacts.lean`, `engine/Proofs.lean`

ARB now has a single certificate-facing package for accepted closed replay certificates.  `ClosedCertificateFacts` includes exact initialized replay, reachability, the initialized action-length bound, closed terminal accounting with resolution enumeration, outcome soundness, ordered merits completion, filing counts, and decision-summary production by exact replay.  Callers can cite one theorem when they need the replay, terminal-state, merits, filing-count, outcome, and summary facts together.  The checker remains exact; this file only composes facts already proved for accepted certificates.

Verification:

- [x] `lake build Proofs.CertificateFacts`
- [x] `lake build Proofs`

### Decision-rule fact package

Reference: `engine/Proofs/DecisionRuleFacts.lean`, `engine/Proofs.lean`

ARB now packages the existing decision-rule facts for the executable `currentResolution?` rule.  `DecisionRuleFacts` gives a reachable state anonymity under current-round vote permutation, closed-resolution anonymity when seating and round match, vote-flip neutrality under the validated strict-majority policy, and quota monotonicity for `demonstrated`, `not_demonstrated`, `none`, and `no_majority`.  This is a composition layer over the engine rule; it does not define an abstract rule space or prove uniqueness.

Verification:

- [x] `lake build Proofs.DecisionRuleFacts`
- [x] `lake build Proofs`

### Decision-rule characterization

Reference: `engine/Proofs/DecisionRuleCharacterization.lean`, `engine/Proofs.lean`

ARB now has a count-level characterization of the executable threshold rule.  `DecisionCounts` records the quota, seated count, and two substantive vote counts, with admissibility requiring that substantive counts fit within seated membership and that seated membership is below twice the quota.  Any count rule with normalized outputs, vote-flip neutrality, demonstrated-threshold decisiveness, and no-result behavior below both thresholds agrees with the engine rule on every admissible input.  The engine count rule satisfies the same specification, and deliberation summaries can use the theorem when their count bound and strict-majority bound are available.

Verification:

- [x] `lake build Proofs.DecisionRuleCharacterization`
- [x] `lake build Proofs`

## 2026-07-10

### Service process record reconciliation

Reference: `runtime/service/service.go`, `runtime/service/clerk.go`, `runtime/service/service_test.go`, `manual.md`

The service now gives child processes direct stdout and stderr log file descriptors instead of copying pipe output through the service process.  This lets the child continue writing logs if the service process exits, and it removes pipe-copy goroutines from the lifecycle path.  Completion still reads the stdout log after the child exits to populate the service summary.

Clerk listing and lookup now reconcile detached active-looking `clerk.json` records before returning them.  If `run.json` exists, the service marks the record completed or failed from that artifact and persists the repaired record.  If no terminal artifact exists, the service marks the record failed with `service restarted and child process is not attached`; it does not reattach to a process.

The direct service registry uses the same restart rule.  Active or previously detached records are repaired from `run.json` when it appears, and the repaired registry record is written back to disk.  Focused tests cover Clerk record repair, detached active Clerk failure, direct registry repair, and attached active evidence-manifest pending behavior.

### Service API error cleanup

Clerk create now checks `examples/EXAMPLE/complaint.md` before reserving an output directory or starting a child process.  Missing examples return `unknown_example`, and invalid example names return `invalid_example`.  Artifact reads now distinguish names outside the allowlist from listed artifacts whose files are absent: the first returns `unknown_artifact`, and the second returns `artifact_missing` without host filesystem paths.

### Verification

- [x] `go test ./arb/runtime/service`
- [x] `go test ./arb/runtime/... ./arbd/runtime/... ./adc/runtime/...`

## 2026-07-06

### ex13 summary draft

Reference: `out/local-direct-three-per-ex-only-20260629/ex13/summary.md`, `agent-instructions/draft-summary.md`, `out/local-direct-three-per-ex-only-20260629/ex13/run-01`, `out/local-direct-three-per-ex-only-20260629/ex13/run-02`, `out/local-direct-three-per-ex-only-20260629/ex13/run-03`

The ex13 draft summary uses only the historical run artifacts as the record.  It now leads with the proposition and `rules.txt` clarifying document, then gives a concise resolution summary before the procedural explanation.  The draft reports all three run-level outcomes, the admitted evidence sets across the three runs, the parties' merits arguments, every submitted or failed council vote, twelve submitted votes for `not_demonstrated`, one failed council member in each run, and the shared evidentiary basis: no signed or formally adopted U.S.-Iran agreement, no matched official public confirmations, and no permanent-cessation language by the June 15, 2025 deadline.

`agent-instructions/draft-summary.md` records the drafting pattern for future example summaries.  It instructs summary drafters to lead with the dispute and resolution, use the run artifacts as the record unless independent verification is requested, link internal artifacts, report run-level variation and process failures, and keep technical runtime details secondary to the arbitration record.

### Provider-tolerant juror model experiment

Reference: `out/juror-model-experiments/generic-5model-tolerant-20260706T145403Z`, `../common/modelrequest/spec.go`, `../common/modelrequest/spec_test.go`

The generic-persona model experiment uses five model identities while allowing OpenRouter to choose the serving provider for each request.  The copied pool records still retain `provider_name`, `endpoint_tag`, and `quantization` as metadata, but each request config includes `provider: {}` so the request parser does not derive a provider lock from those fields.  The parser test now records that rule: an explicit empty provider object suppresses OpenRouter provider derivation and sends no `provider` body.

The completed rows show the intended request behavior.  `provider_only` is empty in `model-runs.jsonl`, while `provider` records the source config lineage, such as WandB, Novita, Alibaba, Mistral, or OpenRouter-routed Claude.  The run artifacts do not record the final upstream endpoint OpenRouter selected after the provider lock was removed; the Pi logs record the API provider as `openrouter`.

The run stopped at `ex08a/run-02` during row 144 and row 145.  Both failures occurred before model deliberation because the Pi container failed while installing `pi-mcp-adapter`, with npm reporting no matching version for `@aws-sdk/core@^3.974.28`.  A later direct check inside `agentcourt-pi-sandbox:latest` could read `@aws-sdk/core` version `3.974.28` and could install `pi-mcp-adapter`, which points to an npm registry or cache inconsistency during live extension installation rather than a model-provider failure.

The shared Pi image now pins `pi-mcp-adapter@2.11.0` at `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter`.  The AAR, ADC, and AARD local-run defaults use that path, so normal Pi agents load the adapter from the image instead of installing `npm:pi-mcp-adapter` at startup.  The rebuilt local Docker image is `agentcourt-pi-sandbox:latest`, image id `sha256:5ea8953d6b1c2e7194abbc28d16cd025298f7ca62f15189df56f8d6fa44bd5da`.

The direct verification runs used `aar juror-replay` against `ex08a/run-02`, snapshot `turn-000009-C1`, and omitted `--pi-mcp-adapter`, so they exercised the new default.  The `gpt-oss-120b` run wrote `arb/out/pi-adapter-baked-test-ex08a-run02-gpt-oss-120b`, returned `status=ok`, and submitted one council vote.  The exact row-144 model check used `claude-opus-4.8-fast`, wrote `arb/out/pi-adapter-baked-test-ex08a-run02-claude-opus-4.8-fast`, returned `status=ok`, and the normal logs for both runs contain no `npm install`, `ETARGET`, or missing-version message.

Before resuming the long experiment, the active ledgers were repaired for the two affected `ex08a/run-02` rows.  The repair preserved the original ledger files and row directories under `arb/out/juror-model-experiments/generic-5model-tolerant-20260706T145403Z/resume-repair/20260706T202709Z`, removed the Claude terminal error row from `model-runs.jsonl`, removed the Claude and DeepSeek failed-attempt rows from `attempts.jsonl` and `failed-attempts.jsonl`, and moved the corresponding active row directories out of `runs/` and `failed/`.  The remaining active counts before restart were 143 terminal rows, 159 attempt rows, and 20 failed-attempt rows.

After the session interruption, the resumed runner had reached row 232 and then stopped after a burst of OpenRouter connection errors.  The failed Pi logs showed `input:0`, `output:0`, and `errorMessage:"Connection error."` before any model output across several unrelated models, while a fresh Pi-image OpenRouter probe later reached the API.  The connection-error rows were preserved under `arb/out/juror-model-experiments/generic-5model-tolerant-20260706T145403Z/resume-repair/20260706T224929Z`, and the active ledgers removed only the nine terminal connection-error rows and eighteen matching failed-attempt rows.  The remaining active counts before restart were 223 terminal rows, 247 attempt rows, and 29 failed-attempt rows.

## 2026-06-18

### Pi council output accounting and ex08a 9-member run

Reference: `runtime/localrun/localrun.go`, `runtime/localrun/localrun_test.go`, `manual.md`, `out/ex08a-openclaw-pi-20260618004247`

The failed ex08a 9-member run at `out/ex08a-openclaw-pi-20260617220136` showed C5 removed for `agent_output_limit_exceeded` after the local process counted raw Pi JSON stdout before compaction.  Pi emitted repeated accumulated `message_update` records while thinking remained enabled, and the saved stdout log was much smaller because `piTailLogWriter` compacted repeated prefixes after the byte counter had already counted them.  The local runner now places the counter after the Pi compactor for stdout, so the council output limit measures compacted stdout plus stderr while preserving the normal 128 MiB cap.

The focused test `TestPiTailLogWriterCounterCountsFilteredBytes` covers that accounting order.  `go test ./runtime/localrun` and `make build` passed after the change.  The manual now states that `aar run` enforces the Pi council output limit against compacted Pi stdout logs and stderr byte counts.

The next ex08a 9-member run at `out/ex08a-openclaw-pi-20260618004247` did not hit the output limit.  C4, `openrouter://minimax/minimax-m2.7`, failed during deliberation because OpenRouter returned upstream `429` on all three automatic retries.  The case later closed `status=ok`, `resolution=demonstrated`, after enough remaining members voted, but that output is unsuitable for a strict 9-vote comparison because it contains `council_member_removed` events instead of nine council votes.

## 2026-06-17

### Lawyer source-work prompts

Reference: `prompts/attorney-common.md`, `prompts/attorney-arguments.md`, `prompts/attorney-rebuttals.md`, `prompts/attorney-surrebuttals.md`

The prompt review added source-mapping, staged search, browser-use, local-tool-installation, media-extraction, and capture-failure guidance to the AAR lawyer prompts.  The common prompt now tells lawyers to identify decisive facts, likely primary sources, confirming sources, adverse sources, and extraction methods before searching.  It also tells lawyers when to use browser tools, when to install a focused local program, and what to record when installation, capture, or extraction fails.

The argument, rebuttal, and surrebuttal prompts now restate the phase-specific search task in concrete terms.  Arguments focus on the proposition's decisive elements and primary source paths.  Rebuttals start from the opponent's evidence ids, quoted phrases, metadata, and source chain.  Surrebuttals keep the same methods limited to new factual points raised in rebuttal.

### Manual review

Reference: `manual.md`, `Dockerfile.md`, `docs/attested-dev-host.md`

The manual states when AAR is the right procedure: one proposition, a binary demonstrated or not-demonstrated outcome, and one output directory as the record for one run.  The guidance names the files an operator should preserve together, including `run.json`, `state.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, evidence files, and council-turn snapshots.  It also distinguishes AAR from AARD where the desired output is a numeric answer or supported degree.

The Clerk section documents the attested events endpoint that the service exposes.  The attested Clerk text points to the AAR Docker runbook and dev-host requirements for the image build, S3 layout, expected PCR values, and verification checks.  The troubleshooting section includes attested Clerk diagnosis: inspect the Clerk record, `/attestation/events`, driver logs, the S3 input prefix, the S3 output prefix, and verification files before relying on console output.

### README review

The README uses the same documentation table as ADC, AARD, and evals.  It links to the manual, Docker runbook, dev-host requirements, practice guide, and rules.  It also names the main output files and points attested output details to the Docker runbook.

## 2026-06-16

### Attested live event monitoring

Reference: `attest/exec-container-entrypoint.sh`, `runtime/service/clerk.go`, `runtime/service/service_test.go`, `Dockerfile.md`, `docs/attested-dev-host.md`

The exec container now starts `aar run` in the background and refreshes `events.ndjson` at `OUTPUT_PREFIX/events.ndjson` while the run is active.  It uploads that object only when the local event file changes, and it performs a final refresh before uploading the terminal archive.  If the live upload fails, the entrypoint terminates the AAR process, writes the failure into `run.log`, uploads the failed-run archive path, and exits with an error.

The Clerk service now exposes `GET /clerk/v1/cases/{case_id}/attestation/events` for attested AAR runs.  The handler serves `events.ndjson` from extracted local output when that file exists, falls back to the top-level downloaded S3 object, and then reads the live S3 object through the configured `dev` host with the configured AWS region.  The endpoint returns the raw NDJSON stream, so callers can tail the same lifecycle events that `aar run` writes without parsing launcher stdout.

### Attested Clerk case packets

Reference: `runtime/proceeding/case_packet.go`, `runtime/cmd/aar/case_packet.go`, `runtime/service/clerk_attested.go`, `tools/run-arb-attested.py`, `tools/run-aar.sh`, `attest/exec-container-entrypoint.sh`, `Dockerfile.md`

Attested Clerk execution now accepts the same case selectors as local Clerk execution: `example`, or `complaint_path` with optional `case_files`.  The service still rejects runtime overrides that the exec path does not carry yet, including policy paths, council pools, OpenClaw settings, Pi settings, and timeout overrides.  Verification remains mandatory before an attested Clerk record reaches `completed`.

The local attested driver uses `AAR_INPUT_MODE=example` for checked-in examples and `AAR_INPUT_MODE=case-packet` for explicit complaints.  In case-packet mode, it invokes `go run ./arb/runtime/cmd/aar case-packet` from the repository root, then uploads `case.tar.gz` and `case-packet.json` to `INPUT_PREFIX` through `dev` and passes their SHA-384 hashes into the exec AMI.  The packet builder lives in the Go proceeding package, so automatic case-file selection and explicit `case_files` validation use the same functions as non-attested `aar run`.

Explicit `case_files` use the same glob expansion, duplicate-basename rejection, and prohibited extension checks as local `aar run`.  The packet stores explicit files under per-file subdirectories so their basenames remain the evidence ids after extraction.  The exec workload container verifies packet and manifest hashes, extracts the packet, and runs `aar run --complaint` with repeated `--file` arguments only when the original Clerk request used explicit case files.

The attestation manifest now records `input_mode`, `aar_case_id`, `case_packet_key`, `case_packet_sha384`, `case_packet_bytes`, `case_manifest_key`, and `case_manifest_sha384`.  The verifier checks the packet keys and hashes for case-packet mode and continues to check `aar_example` for example mode.  The S3 input prefix still must contain `auth.json` and `keys.sh`; the driver stages case objects but does not stage runtime secrets.

### Attested error handling cleanup

Reference: `attest/exec-container-entrypoint.sh`, `tools/run-arb-attested.py`, `runtime/service/clerk.go`

The exec container entrypoint now rejects empty, absolute, newline-bearing, and bad-segment packet paths with a named validator rather than a literal-newline shell pattern.  It also requires EC2 metadata reads for `instance_id` and `ami_id`, so the attested manifest no longer records empty instance identity fields after an IMDS failure.  IMDSv2 token failure still falls back to tokenless IMDS because the host can permit IMDSv1, but the metadata read must succeed.

The local attested driver no longer treats a failed S3 listing as an empty output prefix.  Remote temporary-directory cleanup and launched-instance termination now return errors; when cleanup fails during another failure, the driver reports both failures in one error.  The Clerk service no longer discards `clerk.json` persistence errors: synchronous paths return them, and asynchronous completion paths mark the in-memory record as failed when the final write fails.

### Attested review fixes

Reference: `runtime/proceeding/case_packet.go`, `runtime/service/clerk_attested.go`, `runtime/service/service_test.go`, `tools/run-arb-attested.py`

The case-packet writer now resolves packet, manifest, and source paths before opening any generated output.  It rejects packet or manifest paths that overlap the complaint, case files, or each other, and it writes generated files through temporary files before publishing them.  The proceeding tests cover the complaint-clobbering case, packet/manifest output collision, and a failed packet path that must not publish a manifest.

The Clerk attestation record now reads the real driver keys `INPUT_PREFIX` and `OUTPUT_PREFIX`, while retaining the older `AAR_INPUT_PREFIX` and `AAR_OUTPUT_PREFIX` names as fallbacks for existing local test artifacts.  The service fake driver now writes the same prefix keys as `tools/run-arb-attested.py`, so the service test catches a mismatch between the driver and the Clerk record parser.  The attested driver also verifies `run_id`, `input_prefix`, and `aar_case_id` from `manifest.json` before accepting a completed attestation.

## 2026-06-12

### Generic attested AAR example runs

Reference: `attest/exec-container-entrypoint.sh`, `tools/run-aar.sh`, `tools/run-arb-attested.py`, `Dockerfile.md`

The exec container entrypoint now accepts `AAR_EXAMPLE`, validates it with the same path-safety boundary as `aar run`, defaults to `ex01`, and records the selected example in `manifest.json`.  If `RUN_ID` is absent in AAR mode, the default run ID is `aar-$AAR_EXAMPLE-$STAMP`; non-AAR modes keep the existing `run-$STAMP` default.  The AAR-owned `tools/run-aar.sh` launcher now passes `AAR_EXAMPLE` into the exec workload container and names default runs from the selected example.

`Dockerfile.md` now documents the current attested execution path end to end.  It covers the base image, attested workload image, dev build and upload sequence, launcher installation, S3 input staging, example input, local complaint packet input, the exec AMI command, artifact download, manifest and archive verification, attestation verification, and the known first-failure checks from the completed runs.  The documented generic path runs any checked-in example under `arb/examples/<name>` or any local complaint plus optional case files through the Go case-packet builder.

`tools/run-arb-attested.py` is now the preferred local runner for this path.  It launches the existing exec AMI through `dev`, polls the S3 output prefix, writes local `progress.log` and `launcher.log`, downloads all terminal artifacts into the requested local directory, extracts the AAR archive, and can verify the manifest, archive hashes, attestation user data, and selected PCR values.  If terminal S3 artifacts appear while `exec.sh` is still polling, the runner terminates only the instance ID that `exec.sh` launched and stops the remote launcher.

The AAR-specific runner scripts are in `tools/`: `run-aar.sh`, `run-arb-attested.py`, and `run-container-poc.sh`.  The `attest` repository keeps generic exec AMI and attestation utilities.  `/home/ec2-user/attest` on `dev` remains a runtime directory that can contain copied scripts from both source repositories.

## 2026-06-11

### AAR S3 archive output

Reference: `attest/exec-container-entrypoint.sh`, AAR run `s3://agentcourt-data/arbattest/aar-runs/aar-ex01-20260611T230151Z`

The old entrypoint success path ran `aws s3 cp --recursive "$aar_out" "$output_prefix/aar/"`.  That copied Pi council working homes into S3.  The obsolete `aar-ex01-20260611T230151Z` prefix reached 92,834 objects because it included package trees under paths such as `aar/pi-C4/pi-extensions/npm/.../node_modules/...`.

The exec container entrypoint now uploads AAR output as one archive object instead of recursively copying the working tree.  A successful AAR run uploads `aar-output.tar.gz`; a failed AAR run uploads `aar-partial.tar.gz` with `run.log` and then exits with the AAR status.  The archive excludes `pi-*` homes and staged `openclaw-*-codex` directories, while retaining the case packet, logs, evidence store, event log, transcript, digest, work notes, and `local-run.json`.

The success manifest now records `aar_archive_key`, `aar_archive_sha384`, and `aar_archive_bytes`.  The manifest hash remains the value passed to `nitro-tpm-attest --user-data`, so the attestation binds the single AAR archive object rather than thousands of individual S3 keys.

The dev rebuild used commit `d338c32`.  The rebuilt AAR image is `sha256:72775dddf4cc1b3dcf77970443801d98c2f9740d6576bf655c4fa33cc41c035f`; the rebuilt attested workload image is `sha256:07ee87e51928468e382851ac72ec92062ea7794116652a312a5c32bfab26c2a1`.  The uploaded tar `s3://agentcourt-data/arbattest/images/arb-glue-poc.tar` has SHA-384 `fbfb459dd3b5b2e73763ac98e424342a56b5a82fe3624bc0c940db7d2e3d95f628a7e9d99e212ab28bb680ad9d040133`.

The attested AAR run `aar-ex01-20260612T001855Z` completed on exec instance `i-028821ebeaaf19674`.  Output prefix `s3://agentcourt-data/arbattest/aar-runs/aar-ex01-20260612T001855Z` contains `run.log`, `manifest.json`, `manifest.sha384`, `attestation.b64`, and `aar-output.tar.gz`.  The run result was `status=ok`, `resolution=demonstrated`, and the archive contains the case packet, logs, evidence store, event log, work notes, transcript, digest, and submitted evidence while excluding `pi-*` homes and staged OpenClaw Codex directories.

The manifest hash is `ae52d9b5acccd76a45ce0e6c8f3cabf8e775ddb20e0761702fa1d73e15dffdcab080a0be859556170aaa3a23e9971f41`, and `sha384sum manifest.json` matches `manifest.sha384`.  The archive hash is `ce42ae939df866a2919f20ff8ccd5ffc86df0ffc0f7376b84811f9ae0a44dac8b664b4aaf0a7913b25677a2a7fc75bb0`, matching `manifest.json`.  The attestation signature and certificate chain validated; attestation user data equals the manifest hash.  PCR4 is `83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4`, PCR7 is `98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35`, and PCR12 is all zeros.

The dev-side `exec.sh` launcher did not detect the successful run from EC2 console output.  S3 had the complete success result, but `get-console-output` did not expose `ATTESTATION END`, and `exec.sh` kept polling.  After verification, the temporary instance was terminated manually and the stale launcher process was killed.  Future launcher work should use S3 artifacts as the success record.

### Exec AMI OpenClaw networking

Reference: `arb-glue:poc`, Docker-enabled exec AMI `ami-011f957fe91cf7b81`, AAR run `s3://agentcourt-data/arbattest/aar-runs/ex01-20260611T212020Z`

The `ex01-20260611T212020Z` AAR exec run failed in the plaintiff OpenClaw container after three retries.  Each attempt failed after about 229 to 233 seconds with `stream disconnected before completion` from `https://chatgpt.com/backend-api/codex/responses`, before the plaintiff submitted an opening.  The failing run uploaded `run.log` and `aar-partial/`, but no manifest or attestation, because the exec container entrypoint writes those artifacts only after a successful AAR run.

The auth placement was checked in the same container topology.  Direct OpenClaw in Docker on `dev` can read `/aar-codex/auth.json`, import the token with `openclaw models auth paste-token`, and complete a one-line `openclaw agent --local` request.  The same check also passes when `arb-glue:poc` starts the child OpenClaw container through the host Docker socket on `dev`, proving the staged path under the shared work root is visible to the child container.

The same nested check failed on the Docker-enabled exec AMI when the child OpenClaw container used Docker bridge networking.  Exec instance `i-031896be76d384d75` mounted `/aar-codex/auth.json`, imported the token, then the one-line `openclaw agent --local` request failed after 227,809 ms with the same stream-disconnect error.  This reproduced the AAR failure without AAR prompts, MCP tools, lawyer concurrency, or council containers.

The host-network variant succeeded on the same exec AMI.  Exec instance `i-0c51749a2ab6e1876` used the same attested workload image tar, AAR input `auth.json`, OpenClaw image digest, and one-line OpenClaw request, but started the child OpenClaw container with `--network host`; the diagnostic completed and the launcher saw `ATTESTATION END`.  The fix adds an explicit `aar run --openclaw-network` option and makes the entrypoint AAR invocation pass `--openclaw-network host`; when that mode is selected and no Docker MCP host is specified, `aar run` uses `127.0.0.1` for Docker-launched OpenClaw containers.

## 2026-06-06

### Root documentation tidy

Reference: [AAR Process and HTTP Specification](docs/aar-spec.md), AAR MCP Specification (`docs/aar-mcp-spec.md`), AAR MCP Test Plan (`docs/aar-mcp-test.md`), [AAR Case Failures](docs/case-failures.md), [AAR Case-Failure Testing](docs/case-failures-testing.md)

Durable AAR specifications and test plans moved from the repository root into `arb/docs/`.  Temporary notes, stale drafts, and one-off OpenClaw instructions moved into root `scratch/`.  Root Markdown now contains only repository-level files: `README.md`, `NOTICES.md`, and `AGENTS.md`.

## 2026-06-06

### Practice manual expansion

Reference: [Practice Manual](docs/practice.md), [Agent Arbitration Manual](manual.md), [Evidence Handling](docs/evidence-handling.md)

`practice.md` now functions as a lawyer-facing practice treatise.  The manual link sends command, MCP, Lawyer API, and output-artifact details to `manual.md`.  The practice text focuses on case planning, source search, evidence preservation, browser and local tools, source submission, technical reports, and record-based argument.

The substance pass aligned the practice manual with runtime role names, distinguished submitted evidence from offered exhibits, added evidence-read triage, and stated the runtime council vote labels.  `ARAP.md` no longer describes surrebuttal as text-only because the current runtime, prompts, and manual allow the defendant to submit targeted surrebuttal evidence and technical reports.

## 2026-06-04

### Pi message-update log filtering

Reference: `adjservices/service/arb/README.md#programs`

The C5 Clerk run produced more than 140 MB of Pi stdout while the agent repeated accumulated `message_update` content without calling the council tools.  The stdout log filter now compacts only Pi council `message_update` lines whose active `thinking` or `text` content is a prefix extension of the previous event for the same response and content index.  The stored log line keeps the event metadata, replaces repeated accumulated content with the tail, and adds `aar_log_filter.message: "earlier repeated message_update events dropped"`.

Invalid JSON, unrelated event types, missing content fields, and non-prefix changes remain unchanged.  The local run process counts compacted Pi stdout log bytes plus stderr bytes for the council output limit, so repeated accumulated telemetry does not consume the cap multiple times.  Non-prefix output, stderr, and other large unfiltered records still count toward the limit.  This filter addresses log amplification from accumulated message fields; it does not classify the agent's reasoning quality or alter case state.

### Case-file scanner cleanup

Reference: [Manual case-file scanning](manual.md#initial-evidence)

The first Clerk `ex04` run exposed a stale backup-file problem in automatic case-file scanning.  The example directory contained `README.md~`, and the scanner admitted it as initial evidence because it skipped `README.md` but not editor backup files ending in `~`.  The scanner now skips those backup files, the proceeding test covers that rule, and `examples/ex04/market-rules.md` carries the market rule text as an intentional case-packet file.

### Live-run wall-clock timestamps

The Clerk `ex06` and `ex01` runs showed event timestamps that appeared to exceed 900-second turn deadlines.  The live shell polling also showed `date -u` jumping forward by several minutes during a 30-second tool wait, and MCP responses still reported substantial `remaining_ms` on accepted turns.  AAR creates turn deadlines with `time.Now().Add(...)` and enforces them with timers and deadline comparisons, so the artifact pattern points to host wall-clock adjustment rather than a timeout-enforcement defect.

## 2026-06-03

### Pi council completion

Reference: Pi council instructions (`agent-instructions/pi-council.md.tmpl`)

The Clerk `ex02` run showed C3 voting successfully and then continuing to call `wait_for_opportunity`.  That extra loop later reached C5's opportunity and produced a rejected tool call, though the case still closed correctly because C5 submitted its own vote.  The Pi council instruction template now states that a member must stop after `submit_council_vote` returns `ok: true`, and the localrun template test checks for that stop rule.

## 2026-06-02

### OpenClaw lawyer authentication

Reference: OpenClaw OAuth-Derived Codex Auth (`docs/openclaw-auth.md`)

`aar run` now supports both OpenClaw lawyer authentication paths.  Automatic mode prefers a readable Codex `auth.json`, stages one copied Codex home per lawyer container, mounts it as `/aar-codex`, and sets `CODEX_HOME=/aar-codex`.  If no readable Codex auth file exists, automatic mode uses `OPENAI_API_KEY`.  Explicit `codex` and `api-key` modes force either path.

The staged Codex homes are deleted when the run exits because `auth.json` contains bearer and refresh credentials.  The implementation does not mount the operator's whole `~/.codex` directory into OpenClaw containers.  The API-key path still passes only the `OPENAI_API_KEY` environment variable into the OpenClaw container.

`aar run` now patches the in-container OpenClaw config before starting each lawyer.  The patch sets `plugins.entries.codex.config.appServer.turnCompletionIdleTimeoutMs` and `postToolRawAssistantCompletionIdleTimeoutMs` to the effective AAR lawyer turn timeout.  The ex01 OpenClaw/Pi run on 2026-06-03 failed before this change because the embedded Codex app server abandoned a provider turn after about 120 seconds during plaintiff opening.  The rerun passed that point, completed all lawyer filings, and closed the case.

`aar run --auto-lawyers` controls which OpenClaw lawyers the local process starts.  The default `both` starts plaintiff and defendant.  `plaintiff` starts only the plaintiff and writes `openclaw-defendant-lawyer-skill.md` for a remote defendant; `defendant` starts only the defendant and writes `openclaw-plaintiff-lawyer-skill.md` for a remote plaintiff.  Manual lawyer mode requires a reachable MCP URL.  Use `--mcp-public-base-url` when `--mcp-listen` binds to a wildcard address.

### AAR opportunity failure

Reference: [AAR Case Failures](docs/case-failures.md)

AAR now treats participant failure as case state.  The Lean engine exposes one procedural action, `fail_opportunity`, which validates the active opportunity before changing state.  Plaintiff or defendant failure sets `case.status` to `failed` and records a typed failure object.  Council-member failure sets that member's status to `failed`, records the reason fields on the member, and lets deliberation continue under the existing council rules.

The Go role APIs detect deadlines and invalid-attempt exhaustion, then call `fail_opportunity`.  Lawyer failure now produces a terminal `Result` with `status: "failed"` and process exit `0`; service/runtime faults still use process errors.  The Lawyer API, Council API, service result endpoint, and MCP `wait_for_opportunity` now report failed case or failed-member states directly.

Verification status: `make build` passes, and focused Go tests for runner, service, CLI, and MCP pass.  `lake build Proofs` still fails in `Proofs.StepPreservation` on existing surrebuttal evidence proof obligations: the proof expects old text-only surrebuttal behavior, while the executable now allows surrebuttal evidence.  That proof repair is separate from the `fail_opportunity` runtime path.

The process and HTTP black-box tests now cover the external AAR failure boundary.  They start `aar case` and `aar service`, drive lawyer and council roles over HTTP, and assert process exit status, stdout summaries, service case records, result endpoints, `run.json`, and event logs for attempt exhaustion and deadline expiration.  The service startup path now binds its listener before printing the readiness line, and the service waits for stdout capture to finish before classifying a child process from its final JSON summary.

The failure specification now distinguishes direct `aar case` terminal artifacts from completed service-backed role reads.  The black-box tests retain per-test process logs and HTTP exchange logs on failure, and service-managed cases assert child exit code, parsed stdout summary, stdout log path, stderr log path, and final service status.

### AAR MCP specification

Reference: AAR MCP Specification (`docs/aar-mcp-spec.md`), AAR MCP Test Plan (`docs/aar-mcp-test.md`)

The MCP behavior now has separate root-level specification and test-plan documents.  The spec treats `aar mcp` as a transport adapter that binds each MCP session to one case-role or case-member assignment, exposes stable assignment tool sets, normalizes wait responses, injects the active opportunity id, and forwards calls to the service role APIs.  AAR remains the authority for case state, role validation, member validation, deadlines, attempts, and terminal case status.

The test plan separates unit, process, and service tests.  It covers session binding, authentication, origin checks, tool lists, wait normalization, opportunity-id injection, forwarding, error propagation, process health, logs, and service-backed lawyer, observer, and council assignments.  OpenClaw and Pi runs remain outside the minimum passing set for this adapter boundary.

The first executable pass now starts `aar mcp` as a subprocess, drives `/mcp` with JSON-RPC over HTTP, and uses fake Lawyer and Council role APIs behind the adapter.  The tests cover invalid startup, health readiness, bearer authentication, origin checks, missing and deleted sessions, lawyer, observer, and council tool sets, wait-state normalization, opportunity-id injection, AAR `ok:false` and non-2xx propagation, outbound service authorization, and log redaction.  Idle-session expiry remains a direct unit test because testing it through the process would depend on wall-clock timing rather than the expiry rule.

### Provider and transport cleanup

Reference: Council API (`../scratch/arb/councilapi.md`), OpenClaw service runbook (`../scratch/arb/running.md`), Pi container README (`../common/pi-container/README.md`)

AAR council calls now use direct provider clients for the `direct` backend.  Council seats carry JSON request specs with endpoint, model, provider, quantization, request parameters, and persona information.  The case runner no longer starts a local provider proxy, and the CLI no longer accepts provider-proxy or removed council-agent flags.

Local service examples now run OpenClaw containers for lawyers and Pi containers for council members.  Council members receive their model and routing configuration through the mounted Pi home and reach the case through the Council API-backed MCP service.  Shared persona-generation tools now probe OpenRouter directly and keep OpenAI embeddings direct.

### Lawyer case results

Reference: Lawyer HTTP API (`../scratch/arb/lawyerapi.md`), OpenClaw service runbook (`../scratch/arb/running.md`)

The Lawyer API now exposes `GET /lawyerapi/v1/result`.  The request uses the same `case_id` and `role_id` shape as the rest of the API.  While the case remains open, the response reports `status: "pending"` and returns the live turn envelope.  After the case closes, it returns the resolution, final reason when known, deliberation round, every stored council vote with rationale, and vote counts by round.

The unified MCP server exposes the same data through the read-only `get_case_result` tool.  This keeps final-result inspection available to lawyers and observers without adding another polling loop or reading output files from the operator's filesystem.  The MCP server does not interpret the vote data; it forwards the case-result JSON returned by AAR.

### Lawyer case status

Reference: Lawyer HTTP API (`../scratch/arb/lawyerapi.md`), OpenClaw service runbook (`../scratch/arb/running.md`)

The Lawyer API now exposes `GET /lawyerapi/v1/status` and the read-only `case_status` tool.  The response reports the role's current status, case phase, case status, active turn, current opportunity details, state version, and compact counts for evidence, filings, events, and council votes.  The unified MCP server exposes `case_status` through the stable tool set and calls the status endpoint directly, so a waiting lawyer can inspect case status without an active `opportunity_id`.

### Lawyer Evidence Tools

Reference: [Evidence Handling](docs/evidence-handling.md), OpenClaw lawyer runbook (`../scratch/arb/running.md`)

The Lawyer API now separates read access from evidence submission.  Read-only evidence tools are available in every active lawyer phase, so a remote lawyer can inspect case-packet files before an opening or closing.  Evidence-submission tools remain limited to arguments, rebuttals, and surrebuttals.

Surrebuttals now use the same exhibit and technical-report validation path as arguments and rebuttals.  This keeps surrebuttal narrow as a response phase while allowing the defendant to preserve and cite targeted source material when the plaintiff's rebuttal makes that necessary.  Openings and closings still file text-only legal acts through `submit_decision`.

Lawyer prompts now tell counsel to inspect the current record, scan the evidence list at each opportunity, analyze relevant evidence before advocating from it, and use targeted search when the record leaves a material gap.  They distinguish AAR court tools from native OpenClaw investigation tools, so a clawyer should use web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, and local analysis tools when those tools can find or test material sources.  They require source-page retrieval after search results, adverse-source checks, and a search ledger when material evidence cannot be found or captured.  Counsel must submit material outside sources through AAR evidence tools before relying on them when evidence submission is available.  Remote clawyers receive case-packet files and later submissions through AAR evidence tools rather than local filesystem access.

`buildAttorneyPrompt` now adds the evidence-read reminder in every lawyer phase.  A test renders openings, arguments, rebuttals, surrebuttals, and closings through the single prompt directory, and checks that each generated prompt includes instructions for work notes, evidence scans, evidence analysis, native tools, browser work, local programs, and evidence-reading tools.

The Lawyer API now exposes `send_work_notes` in every active lawyer turn.  It writes the complete notes string to `work-notes.ndjson` with role, phase, turn, opportunity id, timestamp, and optional call id.  The prompts now describe those notes as a working journal: plans, issue outlines, work logs, sources checked, scripts or programs written, packages installed, browser work, OCR and extraction work, adverse checks, errors, analysis, decisions, and unresolved gaps.  The notes log is outside the record: it does not enter Lean state, `events.ndjson`, transcript output, digest output, evidence manifests, or observer event tools.  The MCP adapter exposes the tool as part of the stable lawyer transport tool set.

The removed OpenClaw attorney adapter no longer belongs to the runtime.  The supported OpenClaw path is now `aar service` plus `aar mcp`, with lawyers and council members acting through service-backed MCP tools.

Repeated OpenClaw runs showed plaintiff finding useful sources but attempting to submit them by calling `submit_decision` with `tool_name: submit_evidence`.  Defendant could submit evidence directly in the same service, so the failure was prompt and schema ambiguity rather than a server-wide submission failure.  The lawyer prompts and runbook assignment text now say that evidence admission uses the direct `submit_evidence` tool, or the direct chunked-upload tools, before the final filing.  They also state that `submit_decision` is only for the final legal act and must not wrap `submit_evidence`.  The `submit_decision` schema now filters the engine action list to final filing actions, so `submit_evidence` is no longer advertised as a valid `submit_decision.tool_name`.

## 2026-06-01

### Council API and MCP adapter

Reference: Council HTTP API (`../scratch/arb/councilapi.md`)

The Council API follows the Lawyer API architecture but binds each active client to `case_id` and `member_id`.  The HTTP server exposes `get`, `wait`, and `do`, and the MCP adapter only brokers those calls over Streamable HTTP.  The API keeps vote validation, deadlines, attempts, and evidence read budgets in AAR rather than moving that state into an agent adapter.

The adapter uses one MCP session per case-member.  A failed or expired MCP session can be re-created with the same URL because AAR remains the source of the active opportunity and turn budget.  The current council tool set can be exposed dynamically from the Council API status without adding adapter-side arbitration rules.

## 2026-06-01

### Lawyer API

Reference: Lawyer HTTP API (`../scratch/arb/lawyerapi.md`)

The lawyer side now uses one HTTP API owned by `aar case`.  The runner starts `/lawyerapi/v1`, publishes one active turn at a time, and blocks until the active lawyer submits a valid `submit_decision` call, exhausts attempts, or reaches the turn deadline.  Plaintiff and defendant integrations now sit outside the runtime and can use curl, a CLI, an MCP server, or another client that speaks this API.

The old local lawyer agent path has been removed from the AAR runtime.  Council support now uses direct provider calls or the Council API.  Shared evidence validation, filing validation, and prompt construction remain in the runner and are called by the HTTP API.

The Lawyer API now treats `opportunity_id` as a per-turn guard on plaintiff and defendant `POST /do` calls.  A lawyer receives the current value from `GET /get` in `turn.opportunity_id` and must send it back with every lawyer tool call for that turn.  Missing or stale values fail before tool execution and do not consume the turn's invalid-attempt budget.

The lawyer prompt templates now match that API.  They distinguish HTTP tools from legal acts submitted through `submit_decision`, state the current opportunity id, and remove old local-agent wording.  The single prompt set now contains the evidence-focused source retrieval, preservation, and work-note guidance that previously lived in a separate prompt override directory.

The handbook now gives remote clawyers one procedural and technical reference.  It treats the Lawyer HTTP API as the governing interface and describes MCP as one shared service process with one MCP session per case-role.  The handbook covers phase order, filing rules, evidence custody, turn budgets, observer use, MCP tool mapping, reconnection, and error handling.

The unified MCP server implements the MCP path described in the handbook.  It serves Streamable HTTP at `/mcp`, binds each MCP session from `case_id` and either `role_id` or `member_id` query parameters, exposes assignment-specific tools, and forwards `tools/call` requests to the service role APIs.  It fetches the live opportunity before every forwarded mutating tool call, injects the active `opportunity_id`, and returns AAR failures as MCP tool results with structured content.  The runner remains the phase authority.

OpenClaw onboarding now uses assignment text plus an MCP server definition.  The remote-user procedure is the same for lawyers and council members: the operator gives OpenClaw the case id, assignment id, MCP URL, and token; the claw records the MCP server definition, verifies `wait_for_opportunity`, and enters the wait-tool operating loop.  The claw does not need a scheduled Gateway job to discover turns.

The Lawyer HTTP API now has `/lawyerapi/v1/wait`.  It returns the same status shape as `/get`, but it blocks until a role has work, case state changes, or the request timeout expires.  The response includes `wait.version`, so a runner can call the endpoint again with `after_version` and avoid choosing its own sleep interval.

The unified MCP server exposes `wait_for_opportunity` as an always-available read-only tool.  The server maps that tool to `/wait`, caps each call at 30 seconds, and normalizes the result to `state: ready`, `state: waiting`, `state: done`, or `state: error`.  The OpenClaw-facing instructions tell a clawyer or council member to call `wait_for_opportunity` repeatedly until it receives work, completion, or an error.

`aar mcp` runs as a shared service for many case-role and case-member sessions.  Each MCP session stores the binding for `case_id` plus one principal id; it does not own case state.  Idle-session expiry can delete stale MCP session records without changing an arb.  A clawyer or council member that loses a session can initialize a new MCP session with the same URL and recover current status from the service role APIs.  The server has a default 30-minute idle TTL, a configurable cleanup interval, and `--session-ttl 0` for deployments that want to disable expiry.

- [x] Add the HTTP Lawyer API server to `aar case`.
- [x] Replace local lawyer execution with turn blocking on HTTP tool calls.
- [x] Remove lawyer model, lawyer agent command, lawyer endpoint, and bridge CLI flags.
- [x] Delete the old OpenClaw lawyer adapter and bridge files.
- [x] Update prompt text to use HTTP tool names.
- [x] Require active opportunity ids on lawyer tool calls.
- [x] Clean up default and evidence-rich lawyer prompt templates.
- [x] Draft the arbitration handbook for remote clawyers.
- [x] Add the shared MCP adapter for OpenClaw lawyer sessions.
- [x] Draft the OpenClaw `arb` skill for self-service clawyer assignment.
- [x] Add `/wait` and MCP `wait_for_opportunity` for bounded turn waits.
- [x] Expire idle MCP sessions in the shared adapter.

## 2026-04-01

### Literate Lean proof pass

Reference: [Literate Lean notes](docs/literate-lean.md)

The first proof batch does not try to prove the whole procedure at once.  It
states a few properties that the present engine already claims to implement and
that are useful enough to stabilize early.

The current proof files are:

| File | Purpose |
|---|---|
| `engine/Proofs/InitializeCase.lean` | Policy and initialization postconditions |
| `engine/Proofs/MeritsFlow.lean` | Ordered phase progression through the merits sequence |
| `engine/Proofs/Deliberation.lean` | Vote threshold, no-majority closure, round advance, and member selection |

The shared sample file, `engine/Proofs/Samples.lean`, exists only to keep the
later files readable.  It collects the small example states and the narrow
field-extraction helpers that the theorems need.

### Why these proofs first

Initialization, phase order, and deliberation are the parts of the engine that
give the procedure its meaning.  The proofs are still sample-based, but they
are not arbitrary tests.  Each theorem states a procedural fact that should
remain true if the engine changes later.

### Initial proof targets

- Prove the symmetric policy facts that motivated shared per-side limits.
- Prove more about opportunity selection in rebuttal, surrebuttal, and
  deliberation.
- Prove cumulative material limits on exhibits and technical reports.
- Consider whether the engine should expose cleaner helper definitions for more
  general theorems about deliberation and closure.

### Reachable-state invariants

The proof set no longer stops at representative examples.  The current files
now prove two global invariants over every Lean state reachable through
successful initialization and successful public `step` transitions.

| File | Purpose |
|---|---|
| `engine/Proofs/ReachableInvariants.lean` | Every reachable state preserves the merits-sequence invariant, and therefore procedural parity |
| `engine/Proofs/ReachableMaterialLimits.lean` | Every reachable state respects the cumulative exhibit and report caps |
| `engine/Proofs/StepPreservation.lean` | Public `step` preservation for openings, arguments, rebuttals, surrebuttals, closings, optional passes, council votes, and council-member removal |

This changed the proof burden.  The hard part is no longer to state the global
theorems.  It is to keep the step-preservation layer readable while it mirrors
the executable branching structure in `Main.lean`.

### Next proof targets

- Prove stronger global facts about council composition and vote thresholds.
- Prove more about opportunity selection from reachable states, not only about
  state preservation after a successful step.
- Simplify some proof files in `StepPreservation.lean` so the executable
  branches and the proof branches line up more directly.

## 2026-04-02

### Deliberation-neutrality policy decision

Reference: [Verification](docs/verification.md)

The proof work exposed a policy problem rather than a coding defect.
`currentResolution?` checks `demonstrated` before `not_demonstrated`.  That is
acceptable only if both outcomes cannot simultaneously satisfy the configured
threshold.  The validator previously allowed that overlap.

The engine now resolves that at the policy boundary.  `validatePolicy` in Lean
and Go requires `2 * required_votes_for_decision > council_size`.  That keeps
the current aggregation rule, removes the dual-threshold cases, and makes the
planned deliberation-neutrality theorem a theorem about the whole validated
validated policy rather than a theorem with an extra side condition.

### Deliberation-neutrality proof

Reference: [Verification](docs/verification.md)

Stage 7 is now complete in `engine/Proofs/Neutrality.lean`.  The proof does
not quantify over arbitrary malformed cases.  It proves neutrality over
reachable states, where the existing integrity layer already guarantees that
current-round votes come from distinct seated members and cannot outgrow the
configured council size.

The key proof shape is simple.  First, define a vote-flip map on council
votes and show that flipping the current round swaps the two substantive vote
counts.  Then combine that with the strict-majority validator and the
reachable seat bound to exclude dual-threshold states.  That is enough to show
that `currentResolution?` commutes with the vote flip on every reachable
state.

## 2026-04-03

### Explicit case-file selection for `aar case`

`aar case` still defaults to loading case files from the complaint directory.
That behavior is convenient for the examples, but it depends on a directory
scan and a skip list.  The CLI now also accepts repeated `--file` arguments,
including glob patterns, and passes the resolved file list into the runner.

The explicit list replaces the directory scan.  That keeps the old
default while giving the caller a precise file boundary for one run.  The CLI
expands globs, rejects unmatched glob patterns, and rejects prohibited
extensions: `.gitignore`, `.sh`, and `.sig`.  The runner then loads exactly
those files and fails on duplicate basenames, because the case record keys
files by visible filename.

### `aar case` summary JSON

`aar case` now writes one JSON object to standard output for execution
results.  On success, the object reports the resolution and the final-round
counts for votes for and against the proposition.  On failure, the object
reports the error string.

The command still exits nonzero on failure.  The CLI wraps those failures in a
reported-error type so the JSON object remains the only case-result payload on
standard output and the binary does not add a second plain-text error line for
that path.

### Attorney web search in removed local-agent runs

The attorney prompts already instructed the model to use native web search when
public investigation mattered, but the old local-agent path did not stage a
search-enabled model into the temporary Pi home.  The attorneys were told to do
work that the runtime had not enabled.

That path has since been deleted.  The current lawyer design puts model
selection outside AAR and keeps AAR responsible for case access, evidence
validation, filing validation, and turn budgets.  The lawyer prompt still
requires source retrieval, evidence preservation, analysis, and a work log.

The old attorney timeout also became too short once public-source
investigation was enabled.  In `ex04`, the plaintiff arguments turn used enough
public-source investigation to exceed 480 seconds before filing.  The default
attorney timeout is now 900 seconds.

### Attorney filing limits in prompts

`ex04` exposed a second prompt defect after web search was enabled.  The
attorneys could now gather the needed material, but the prompt still left key
filing constraints implicit.  The plaintiff rebuttal then burned its retries on
 three avoidable mistakes: a rebuttal that exceeded the text limit, too many
technical reports for the side-wide cap, and earlier attempts to place
workspace filenames in `offered_files`.

The prompt and attorney view now state the hard limits for the current
opportunity.  That includes the text limit for the current filing, the per-file
and per-side exhibit and technical-report caps, the amount already used by the
current side, and the remaining capacity.  The prompt now also states the engine
record rule: `offered_files` may name only visible case files by `file_id`;
outside material enters through `technical_reports`.

Attorney validation errors now carry the attempted count and the remaining
side capacity.  That keeps the model close to the engine rule and avoids
wasting retries on blind correction attempts.

### Retired lawyer model configuration

Older revisions allowed `aar case` to configure lawyer models and local or
remote lawyer agent commands.  The `lawyerapi` branch removed that path and
left lawyer model selection to clients outside the runtime.

The removed plaintiff demo staged a backend Pi home through the same code path
that ordinary attorney runs used.  `aar` exposed two helper commands for that
purpose: one staged the Pi home into a supplied directory, and one printed the
current lawyer tool catalog as JSON.  The demo script used those helpers
instead of carrying its own copies of `settings.json`, `models.json`, and the
tool schema.

## 2026-04-30

### Ignore regenerated signing evidence in `ex01`

Reference: [Example signer](examples/ex01/sign.sh)

`examples/ex01` regenerates `samantha_public.pem` and `confession.sig.b64` from
the ignored source inputs `samantha_private.pem` and `confession.sig`.  Keeping
the derived files tracked leaves the worktree dirty after an ordinary example
run.

The local `.gitignore` in `examples/ex01` now ignores those derived outputs as
well.  The repository index must also stop tracking them, because ignore rules
do not apply to files that Git already tracks.

### Invalid-attempt limit errors now preserve reasons

Reference: [Attorney tool helpers](runtime/proceeding/attorney_tools.go), [Council runner](runtime/proceeding/council.go)

The attorney and council runners previously replaced the decisive validation
message with a generic invalid-attempt ceiling error on the final failed
submission.  That made the failure hard to diagnose, because the run-level
error lost the exact reason that had already been returned to the agent during
the correction loop.

The runner now carries the invalid reasons forward and includes them in the
final limit error in attempt order.  That keeps the stop condition the same,
but it makes the terminal error match the runtime rejection path instead of
hiding it behind a generic summary.

### Invalid submission feedback now explains the next step

Reference: [Attorney tool helpers](runtime/proceeding/attorney_tools.go)

The attorney tool path previously returned only the bare validation error on
each rejected submission.  That told the model what failed, but it did not say
how many invalid submissions remained or what another miss would do to the
run.  The handler now returns structured rejection text with the current
invalid-submission count, the remaining budget for the opportunity, and one
corrective instruction.

Length failures now report submitted and allowed characters, direct the agent
to count characters rather than tokens, and give a resubmission target below
the hard cap.  Final exhausted attempts switch to terminal language and state
that the opportunity has failed and the run is ending with an error.  The
terminal message still includes the ordered invalid-submission history.

That change fixed a real mismatch.  The earlier script omitted the write-file
tool and hand-built the Pi configuration.  After the change, the external
plaintiff opening matched the ordinary local path closely enough to complete:
note file write, opening submission, accepted filing.

It did not fix the plaintiff arguments failure in `ex06`.  The plaintiff still
stalled in the arguments phase.  The failure mode changed, which narrows the
cause.  The old run spent its time rewriting notes around citation formatting
and source packaging.  The new run used the complete tool set and reached the
substance faster.  It still kept rewriting `case-notes.md`, but the content now
tracked the adverse merits directly: the notes concluded that the official
record supports ground entry but likely not the territorial-objective element,
and that the plaintiff's best colorable `YES` theory runs into the explicit
edge-case carveout.  That points to a prompt or role-interface problem about
how plaintiff advocacy should proceed when truthful investigation turns the case
against the assigned side.  It does not point to agent transport or Pi-home
staging any longer.

## 2026-04-08

### Verification document consolidation

Reference: [Verification](docs/verification.md)

The verification material had split into a status note, a stage plan, and a
findings note.  That separation made the current state harder to read, because
a reader had to reconstruct one story from three files.  The documentation now
uses `docs/verification.md` as the canonical record for established results,
the finished stage structure, proof-driven findings, and the limits of what the
Lean engine can prove.

### Abstract verification structures

Reference: [Verification](docs/verification.md)

The next proof work now has a separate note about abstractions that the current
engine already suggests.  The strongest candidates are a progress preorder over
fixed-frame runs, a compact deliberation summary, a viable-outcomes notion for
threshold reachability, the existing vote-flip involution, a lexicographic
termination potential, and a trace semantics for successful runs.  The
recommended first extension is a deliberation-summary layer that isolates
counts, remaining eligible voters, round budget, and outcome attainability from
the full case record.

### Deliberation summary proof layer

Reference: [Verification](docs/verification.md)

The first implementation step now spans
`engine/Proofs/DeliberationSummaryCore.lean` and
`engine/Proofs/DeliberationSummary.lean`.  The core file now carries the
compact proof-side `DeliberationSummary` record, the direct case-level
correspondence with `currentResolution?`, and the lower council arithmetic that
the summary layer needs.  The wrapper file keeps the reachable vote-count,
seated-count, and positive-threshold bounds that rely on later proof layers.

### Summary-core dependency split

Reference: [Verification](docs/verification.md)

The import graph had blocked the next summary-based compression.  `OutcomeSoundness.lean`
and `NoStuck.lean` sat below `DeliberationSummary.lean`, because that file had
been importing `BoundedTermination.lean` for a few local arithmetic lemmas and
for the reachable wrappers.  The summary layer now splits at that boundary:
`DeliberationSummaryCore.lean` sits below `OutcomeSoundness.lean`, while
`DeliberationSummary.lean` keeps only the reachable wrappers above `NoStuck`.

That change pulled the direct `currentResolution?` soundness facts into the
summary core and let `OutcomeSoundness.lean` consume them directly.  The lower
termination file now imports the core arithmetic instead of defining the same
council-length and current-round-capacity lemmas.  The remaining import
pressure is now on the liveness side rather than on outcome soundness.

### Summary-form liveness bridge

Reference: [Verification](docs/verification.md)

The next split now reaches one theorem in `NoStuck.lean`.  The selector fact
that `nextCouncilMember?` returns a seated member who has not yet voted moved
into `DeliberationSummaryCore.lean`, together with the summary-capacity lemma
that turns that fact into `current_round_vote_count < seated_count`.  `NoStuck.lean`
now uses those lower results to prove the summary-form round-capacity theorem
for every reachable live deliberation state.

The split moves one real liveness theorem below `ViableOutcomes.lean` instead
of leaving the whole summary bridge above the existing Stage 3 file.  The remaining pressure is narrower: the viability
and closure facts still sit above `NoStuck.lean`, but the basic summary view
of live deliberation no longer does.

### Viability-core dependency split

Reference: [Verification](docs/verification.md)

The same import pressure then showed up inside the viability layer.  The
summary-level viability definitions and lemmas had been sitting in
`ViableOutcomes.lean` above the executable update correspondences, even though
most of them did not depend on removal arithmetic or on later proof layers.
The viability layer now splits the same way the summary layer did:
`ViableOutcomesCore.lean` carries the pure viability language and the
summary-only theorems, while `ViableOutcomes.lean` keeps the direct vote and
removal update correspondences.

`OutcomeSoundness.lean` now imports `ViableOutcomesCore.lean` and proves the
`no_majority` branch through summary
non-viability instead of reopening the threshold arithmetic directly from
`currentResolution? = none`.  The core file now also carries a summary closure
predicate for `no_majority`, so the lower layer can package the executable
closure reasons with the below-threshold conclusion before `OutcomeSoundness.lean`
translates the result back to the state-level statement.  `OutcomeSoundness.lean`
now also proves the direct bridge in both directions: summary `no_majority`
closure is sufficient for `continueDeliberation` to close that way, and an
executable `no_majority` closure from deliberation implies the same summary
predicate on the source state.  That leaves the higher file responsible only
for the executable update correspondence lemmas that still depend on the later
termination layer.

### Viable outcomes proof layer

Reference: [Verification](docs/verification.md)

The second implementation step now spans `engine/Proofs/ViableOutcomesCore.lean`
and `engine/Proofs/ViableOutcomes.lean`.  The core file defines summary-level
viability for the two substantive outcomes, proves the first shrinkage facts,
and packages the pure summary-side closure lemmas.  A vote for one side
preserves that side's viability and can only shrink the other side's viability.
Removing one seated member can only shrink viability for both sides.  The
higher file then proves that these summary updates match the intermediate
deliberation states produced by direct vote and removal updates before
`continueDeliberation` runs.

### Summary-based public wrappers

Reference: [Verification](docs/verification.md)

The first bridge theorems for the third stage now split across
`engine/Proofs/OutcomeSoundness.lean`, `engine/Proofs/NoStuck.lean`,
`engine/Proofs/ViableOutcomesCore.lean`, and `engine/Proofs/ViableOutcomes.lean`.
The liveness side now proves the summary-form current-round capacity bound in
`NoStuck.lean`.  The closure side now proves the `no_majority` arithmetic
through summary non-viability in `OutcomeSoundness.lean`.  The core viability
file handles the summary-side facts: executable `currentResolution?` implies
the corresponding summary-viability fact, summary-level exhaustion implies
executable non-resolution, and the summary-level count flip swaps the two
substantive outcomes.  The higher viability file then handles the executable
vote and removal update correspondences.  `engine/Proofs/Neutrality.lean` now
uses that lower summary form directly, so the reachable vote-flip theorem is
stated over the same public result but proved through `DeliberationSummary`
instead of through another round of raw vote-count case analysis on the case
record.

### Closed-resolution bridge

Reference: [Verification](docs/verification.md)

The next compression step turned the summary closure language into one uniform
bridge for closed deliberation results.  `ViableOutcomesCore.lean` now defines
the proof-side `DeliberationSummary.closedResolution?` function and proves the
summary equalities that correspond to substantive threshold closure and to
`no_majority` closure.  `OutcomeSoundness.lean` now proves the executable
bridge in both directions: if the source summary reports a closed resolution,
`continueDeliberation` returns exactly that closed result, and if
`continueDeliberation` closes a deliberation-phase case, the source summary
reports the same result.

The summary layer now defines the whole closed-output boundary of
`continueDeliberation`, which is the right granularity for later monotonicity
or inevitability theorems.  The remaining higher work is narrower: the executable vote and removal update correspondences still sit
above this layer, but the closure logic now has one proof-side shape.

### Executable viability transport

Reference: [Verification](docs/verification.md)

The next step converted those remaining executable update correspondences into
real viability statements.  `ViableOutcomes.lean` still uses the summary equalities
for the intermediate vote and removal cases before `continueDeliberation`, but
it now proves what those equalities mean for the engine state.  A vote for
`demonstrated` preserves demonstrated viability and preserves impossibility of
`not_demonstrated`.  A vote for `not_demonstrated` preserves not-demonstrated
viability and preserves impossibility of `demonstrated`.  A seated-member
removal preserves impossibility for both substantive outcomes.

The higher viability file now carries executable impossibility facts that the
later public step theorems can consume without reopening the arithmetic in the
summary core.

### Same-round final-state bridge

Reference: [Verification](docs/verification.md)

The next step carried that transport across the `continueDeliberation` boundary
when the round does not advance.  `ViableOutcomes.lean` now proves a compact
congruence fact for `DeliberationSummary`: if `continueDeliberation` keeps the
same deliberation round, then the final state has the same summary as the
intermediate `stateWithCase s c`.  That is the right bridge because the
function may still close the case in place, but closure changes none of the
summary fields.

That bridge supports two new public same-round results.  First, a successful
council-vote step now yields an existential `sameRoundVoteTransport` theorem:
for the submitted vote label, the final state preserves viability of the voted
side and preserves impossibility of the opposite side.  Second, a successful
council-member removal step now preserves demonstrated impossibility,
not-demonstrated impossibility, and therefore total substantive non-viability
when the round stays fixed.

### Progress-viability bridge

Reference: [Verification](docs/verification.md)

The next step connected those same-round deliberation facts to the structural
progress layer without overstating what `fixedFrameProgress` can prove alone.
`ProgressViability.lean` imports both `Progress.lean` and
`ViableOutcomes.lean` and proves two public bridge theorems.  A successful
same-round council-vote step now yields both `fixedFrameProgress s t` and an
existential `sameRoundVoteTransport` witness.  A successful same-round
council-member removal step now yields `fixedFrameProgress s t` together with
an implication from source total substantive non-viability to target total
substantive non-viability.

The present preorder tracks case frame, materials, seats, phase rank, and
round.  It does not track current-round votes.  The new bridge therefore pairs
progress with viability transport on the concrete same-round deliberation steps
where the vote update is known, instead of claiming a false global monotonicity
theorem for `fixedFrameProgress` alone.

### Same-round deliberation progress

Reference: [Verification](docs/verification.md)

The next step turned that bridge into a proof-side relation.  `ProgressViability.lean`
now defines `viableOutcomesShrink`, which says that target viability for either
substantive outcome implies source viability for that same outcome.  It then
defines `sameRoundDeliberationProgress`, which combines `fixedFrameProgress`,
same-round equality, and that shrink relation.  Both new relations are
reflexive and transitive.

The public step theorems now establish that same-round relation for successful
council-vote and council-removal steps.  The vote side uses a new lower wrapper
in `StepPreservation.lean` that exposes the already-forced vote-label
disjunction from `recordCouncilVote`.  The removal-side non-viability
preservation theorem now follows from `viableOutcomesShrink` instead of sitting
as a separate ad hoc implication.  This is the first abstract relation in the
library that tracks both structural progress and substantive viability
shrinkage without pretending that the global preorder already contains current-round
vote data.

### Same-round closure inevitability

Reference: [Verification](docs/verification.md)

The next step completed that same-round line.  `ProgressViability.lean` now
proves that `sameRoundDeliberationProgress` preserves `no_majority` closure
reasons in the only form that matters for later closure: the target state has
completed the round.  The key structural lemma here is seat-count monotonicity
under `fixedFrameProgress` plus source council-id uniqueness.  That suffices to
carry the "too few seats" closure reason forward, while same-round equality and
fixed policy carry the last-round reason.

The file then packages the main theorem: if the source summary already has no
viable substantive outcome and already has one `no_majority` closure reason,
then any later same-round progress state that completes the round is forced to
summary `no_majority` closure.  The executable corollary is direct through
`OutcomeSoundness.lean`: `continueDeliberation` on that target state must close
as `no_majority`.  The public council-vote and council-removal theorems now
inherit that result.  This finishes the summary, viability, and same-round
progress agenda as a coherent proof line.

### Fixed-frame progress preorder

Reference: [Verification](docs/verification.md)

The next implementation step now lives in `engine/Proofs/Progress.lean`.  The
file defines `fixedFrameProgress`, a state relation anchored to the source
frame and paired with the monotone coordinates that the library had been
proving separately: append-only admitted materials, shrinking seated-member
identifiers, nondecreasing phase rank, and nondecreasing deliberation round.
The first theorem batch proves reflexivity and transitivity, shows that every
successful public step establishes that relation, and packages the initialized
run form as the conjunction of the initialization frame and source-anchored
progress from the initialized state.

### Attorney tool-error handling

The attorney guidance now states that tool errors are authoritative host
feedback and that counsel must change the request before retrying the same
tool.  I added that rule to both the standing attorney instructions and the
always-sent attorney court prompt.  The duplication is deliberate because the
standing file does not travel over every remote client path, while the common
court prompt always does.

### Opening cap and target margin

The next policy change raises `max_opening_chars` from `4000` to `5000` in both
the built-in default policy and the checked-in `etc/policy.json` that `make`
targets load by default.  The target-length guidance now uses 75% of the hard
cap again for both the first-submission prompt target and the retry hint.  That
gives openings a `3750` target under a `5000` cap, while leaving the hard cap
configurable through policy JSON.

## 2026-05-04

### Flexible complaint input

Reference: [Complaint parser](runtime/spec/complaint.go)

The arbitration runtime needs one proposition string.  The source file format
no longer has to carry a literal `# Proposition` heading for the parser to
produce that value.  When a `Proposition` section exists, the parser uses that
section.  When no such section exists, the parser treats the whole trimmed file
as the proposition.

The canonical writer still emits a `# Proposition` heading.  That keeps
generated complaint packets stable and readable.  Empty input fails, and an
explicit empty `Proposition` section fails, because either case lacks a
proposition.

- [x] Preserve canonical complaint output.
- [x] Accept plain text as complaint input.
- [x] Reject blank complaints and blank explicit sections.
- [x] Cover parser behavior in tests.

## 2026-06-02

### Public service startup

Reference: AAR service (`runtime/service/service.go`)

The first `ex01` service run failed during case creation because the public
service waited only thirty seconds for the child runner to announce private
lawyer and council APIs.  The child runner starts those private APIs after
council preflight, and council preflight can spend more than thirty seconds on
external model availability checks.  The public service now returns an accepted
case once the child process starts, keeps the case in `starting`, and lets
public role `wait` calls block within the API wait limit until the private role
API appears.

The corrected path was tested with `ex01` and `ex04` through the public service,
the AAR MCP adapter, OpenClaw lawyer containers, and council members using the
council API.  `ex01` closed as demonstrated with a 4-1 council vote, and `ex04`
closed as demonstrated with a 5-0 council vote.  The searched MCP logs for both
runs showed no HTTP 4xx or 5xx tool calls and no MCP error states.

### Agent lifecycle

The `ex01` OpenClaw/Pi run showed repeated C4 MCP sessions because the example
runner restarted agents and used them to check for work.  That lifecycle was
wrong.  The example runner now starts each lawyer or council agent once and
lets `set -e` fail the run when a command fails.

### Private case API startup

Reference: Service runner (`runtime/service/service.go`)

The public service no longer reads child API URLs from child stderr.  It chooses
one local private address before it starts `aar case`, passes that address as
`--caseapi-addr`, records `caseapi_base`, and polls `GET /health` on that base
until startup succeeds or the configured startup timeout expires.  The child
case API serves `/health`, `/lawyerapi/v1/...`, and `/councilapi/v1/...` on the
same private listener when the Council API backend is active.

The subprocess tests also exposed invalid stdout-pipe ordering.  Both the
service child watcher and the black-box process test code now wait for stdout
capture to finish before calling `cmd.Wait()`, matching Go's `StdoutPipe`
requirements and preserving the final JSON summary for service status.

### Service-backed MCP process test

Reference: MCP process test (`runtime/cmd/aar/mcp_blackbox_test.go`)

The external MCP test now starts `aar service`, starts `aar mcp`, creates a
real service-managed case with the Council API backend, and drives plaintiff,
defendant, observer, and council assignments through MCP JSON-RPC.  The test
checks tool lists, observer rejection of mutating tools, work-note recording,
evidence reading, lawyer filings, council votes, service final result data, and
the case artifacts written under the output directory.

## 2026-06-11

### OpenClaw stream retry

Reference: Local run launcher (`runtime/localrun/localrun.go`)

An attested `ex01` AAR run on the Docker-enabled exec AMI reached the plaintiff
OpenClaw lawyer and failed inside `openclaw agent`.  The plaintiff stderr log
reported `stream disconnected before completion` from the ChatGPT Codex response
endpoint after about 228 seconds.  AAR treated the process exit as fatal before
the plaintiff opening opportunity completed.

The generated OpenClaw container command now retries only that observed stream
disconnect failure.  It keeps the same `AAR_SESSION_KEY`, captures stderr for
classification, and exits immediately for auth, MCP, configuration, or other
OpenClaw failures.  The localrun package test covers the generated command, and
`go test ./arb/runtime/localrun` passes.

### Exec completion marker

Reference: AAR exec container entrypoint (`attest/exec-container-entrypoint.sh`)

The exec launcher waits for an `ATTESTATION END` marker in the application
console output.  The AAR exec entrypoint path wrote the attestation to S3 but printed only
`OUTPUT_PREFIX` and `MANIFEST_SHA384`, which would leave a successful run
waiting until the launcher timeout.  The exec container entrypoint now prints `ATTESTATION END`
after it uploads `run.log`, `manifest.json`, `manifest.sha384`, and
`attestation.b64`.

### Exec retry result

Reference: Local run launcher (`runtime/localrun/localrun.go`)

The rebuilt post-retry attested workload image tar was uploaded to
`s3://agentcourt-data/arbattest/images/arb-glue-poc.tar` with SHA-384
`4586edeca3246f471aa446b536736cbf7d6d6843447f6955a5f2f81016c7784f408f92869eb916adabb7fb624808acb8`.
The follow-up exec run used instance `i-0237488429308a6e0` and wrote partial
artifacts under `s3://agentcourt-data/arbattest/aar-runs/ex01-20260611T212020Z`.
The run reached the OpenClaw lawyers, configured their MCP servers, and stopped
before any plaintiff opening statement was submitted.

The retry code behaved as intended.  Plaintiff attempts 1, 2, and 3 all failed
with `stream disconnected before completion` from
`https://chatgpt.com/backend-api/codex/responses` after about 229 to 233
seconds.  Defendant attempts 1 and 2 hit the same error before AAR stopped
because the plaintiff container exited.

The partial AAR state shows the case waiting at `openings:plaintiff`.  `run.log`
reports `docker process openclaw-plaintiff exited before case completion`, and
the S3 prefix contains `run.log` plus `aar-partial/` only.  No `manifest.json`,
`manifest.sha384`, or `attestation.b64` exists for this run because the exec
container entrypoint creates those files only after `aar run` exits successfully.

The follow-up diagnostics reproduced the failure without AAR prompts, MCP
tools, lawyer concurrency, or council containers.  A one-line nested OpenClaw
request on the same exec AMI failed on Docker bridge networking after 227,809
ms with the same stream-disconnect error.  The same request succeeded when the
child OpenClaw container used Docker host networking.

### Agent cleanup ordering

Reference: Local run launcher (`runtime/localrun/localrun.go`)

The host-network AAR run on instance `i-0fe13af586ae3c639` passed the previous
OpenClaw stream-disconnect point, reached council deliberation, and entered the
success upload path under
`s3://agentcourt-data/arbattest/aar-runs/aar-ex01-20260611T230151Z/aar/`.
During that upload, the console showed child container output interleaved with
S3 copy output.  That led to a cleanup-ordering check in the localrun package.

`stopAgents` stopped live child processes but did not wait for the original
`docker run` or child process to finish `cmd.Wait()` and close redirected
stdout and stderr.  The exec container entrypoint could therefore begin uploading
the AAR output tree while the final process log bytes were still being flushed.  The
uploaded output tree can then race the process log closure.

The process completion channel now carries the process wait error separately
from stdout and stderr close errors.  `stopAgents` waits up to 30 seconds after
stopping each live process, treats the process exit from an intentional stop as
expected, and still reports log-close errors.  `TestStopAgentsWaitsForProcessExit`
covers the ordering.

## 2026-06-12

### Attested `ex03` run

Reference: AAR Docker image runbook (`Dockerfile.md`)

The `ex03` attested run used the checked-in driver at
`tools/run-arb-attested.py` with verification enabled.  The first build command
body was accidentally invoked locally instead of through `ssh dev`; it failed at
`cd /home/ec2-user/adjudication-build-2361886` and changed no remote state.  The
corrected remote build used `/home/ec2-user/adjudication-build-2361886` on
branch `arbattest`, rebuilt `arbattest-aar:dev` and `arb-glue:poc` with Docker
cache disabled, and uploaded `s3://agentcourt-data/arbattest/images/arb-glue-poc.tar`
with SHA-384 `1b3e3a9a1bae75dbe527d12591d95d526b4b4f7a063e72ba1e9239e709e752c7f1f1c5884f722fc5fff94f1cf3695f50`.

The staged input prefix is
`s3://agentcourt-data/arbattest/aar-inputs/ex03-20260612T031231Z`, containing
`auth.json` and `keys.sh`.  The rebuilt image validated
`examples/ex03/complaint.md` before input staging.  The runtime launcher files in
`/home/ec2-user/attest` matched the checked-in `exec.sh`,
`parse_attestation.py`, and `tools/run-aar.sh` by SHA-384 before the EC2 run
started.

The run id is `aar-ex03-20260612T031231Z`.  It used exec AMI
`ami-011f957fe91cf7b81`, instance `i-0f3bb32a380fdd053`, and output prefix
`s3://agentcourt-data/arbattest/aar-runs/aar-ex03-20260612T031231Z`.  The local
output directory is `/media/hd2/src/arbattest/aar-attested/aar-ex03-20260612T031231Z`.

The S3 output prefix contains exactly five success objects: `run.log`,
`manifest.json`, `manifest.sha384`, `attestation.b64`, and
`aar-output.tar.gz`.  The AAR result in `aar-output/local-run.json` is
`status=ok`, `resolution=demonstrated`, with case id
`arb-ex03-20260612031530`.  The manifest reports
`started_at=2026-06-12T03:14:48Z` and `finished_at=2026-06-12T03:33:00Z`.

Verification passed locally.  `manifest.sha384` is
`8a8c4260fbc8657221baba08af1c9f150eac12e728437ef8572e705d998c7170d3f97b7361727befd99e7ed8311dc10e`,
the archive is 3,132,979 bytes with SHA-384
`839565dd0f92ab86fef012f1b80873e3f5cf9653cbcbc1b4ace8cb7463b7acc967f530f63490ad2e1611ce88b2f91ce7`,
and `run.log` SHA-384 is
`02acf50cc09728289099519757884602629c5c597e84ccc32d45648e24342c27c99c0a54682604dbfc50d20af70b6c40`.
The manifest records container image id
`sha256:30858a2901b6f61cd0d4cb5ac96edee2ca34bb82f194c5ab807104064ecc82df`
and container image tar SHA-384
`1b3e3a9a1bae75dbe527d12591d95d526b4b4f7a063e72ba1e9239e709e752c7f1f1c5884f722fc5fff94f1cf3695f50`.

The attestation signature and certificate chain validated.  The attestation
user data equals the manifest hash.  PCR4 matched
`83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4`,
PCR7 matched
`98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35`,
and PCR12 was all zeros.

The local driver terminated instance `i-0f3bb32a380fdd053` after it saw the
complete S3 artifact set, downloaded the artifacts, verified the manifest and
attestation, and extracted the archive.  EC2 reported the instance as
`shutting-down` with reason `Client.UserInitiatedShutdown: User initiated
shutdown` immediately after the run.  The run therefore used S3 artifacts as the
completion record and did not require manual cleanup of the launched instance.

### Attested `ex06` run

Reference: AAR Docker image runbook (`Dockerfile.md`)

The `ex06` attested run used `tools/run-arb-attested.py` with verification
enabled.  The dev-side image validated `examples/ex06/complaint.md` before
input staging.  The runtime launcher files in `/home/ec2-user/attest` matched
the expected runtime path by SHA-384 before the EC2 run started.

The staged input prefix is
`s3://agentcourt-data/arbattest/aar-inputs/ex06-20260612T042346Z`, containing
`auth.json` and `keys.sh`.  The run reused the uploaded attested-workload-image tar at
`s3://agentcourt-data/arbattest/images/arb-glue-poc.tar` with SHA-384
`1b3e3a9a1bae75dbe527d12591d95d526b4b4f7a063e72ba1e9239e709e752c7f1f1c5884f722fc5fff94f1cf3695f50`.

The run id is `aar-ex06-20260612T042346Z`.  It used exec AMI
`ami-011f957fe91cf7b81`, instance `i-00fb5acdf339f2592`, and output prefix
`s3://agentcourt-data/arbattest/aar-runs/aar-ex06-20260612T042346Z`.  The local
output directory is `/media/hd2/src/arbattest/aar-attested/aar-ex06-20260612T042346Z`.

The S3 output prefix contains exactly five success objects: `run.log`,
`manifest.json`, `manifest.sha384`, `attestation.b64`, and
`aar-output.tar.gz`.  The AAR result in `aar-output/local-run.json` is
`status=ok`, `resolution=not_demonstrated`, with case id
`arb-ex06-20260612042615`.  The manifest reports
`started_at=2026-06-12T04:25:34Z` and `finished_at=2026-06-12T04:41:36Z`.

Verification passed locally.  `manifest.sha384` is
`d65c655127b54a8846766b1931f90fddb5182b23aee768aeef27c28f320a32a3ef62b0fa9317f4dff73ddce99453a8e3`,
the archive is 1,166,917 bytes with SHA-384
`cac64144d8da3103f849efc58f6ffe5e6530079593f5578124dd58c24d03b9c61a63011ddc0b573549806f024d6c301f`,
and `run.log` SHA-384 is
`4b3eefaf1be7f5f1db0daded302309da001d31067278d9107166a5443caea138c39ea653e35c55df9aa31a399ba54605`.
The manifest records container image id
`sha256:30858a2901b6f61cd0d4cb5ac96edee2ca34bb82f194c5ab807104064ecc82df`
and container image tar SHA-384
`1b3e3a9a1bae75dbe527d12591d95d526b4b4f7a063e72ba1e9239e709e752c7f1f1c5884f722fc5fff94f1cf3695f50`.

The attestation signature and certificate chain validated.  The attestation
user data equals the manifest hash.  PCR4 matched
`83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4`,
PCR7 matched
`98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35`,
and PCR12 was all zeros.  The local driver terminated instance
`i-00fb5acdf339f2592` after it saw the complete S3 artifact set, downloaded the
artifacts, verified the manifest and attestation, and extracted the archive.

## 2026-06-16

### README runbook references

`arb/README.md` now points operators to the three documents needed for attested Clerk runs: `manual.md`, `Dockerfile.md`, and `docs/attested-dev-host.md`.  The README names the `aar service` and Clerk API sections, the attested Docker image runbook, the S3 artifact layout, live `events.ndjson`, and verification.  The layout table lists `Dockerfile.md` as the attested Docker image and exec runbook.

### Clerk-attested `ex01` run

The Clerk-attested `ex01` run used `aar service` with the checked-in attested driver, verification enabled, exec AMI `ami-011f957fe91cf7b81`, and instance type `m5.4xlarge`.  The accepted case id was `clerk-ex01-20260616T221531Z`, and the accepted run id was `aar-clerk-ex01-20260616T221531Z`.  The input prefix was `s3://agentcourt-data/arbattest/aar-inputs/clerk-ex01-20260616T221531Z`, and the output prefix was `s3://agentcourt-data/arbattest/aar-runs/aar-clerk-ex01-20260616T221531Z`.

The Clerk monitoring endpoint `/clerk/v1/cases/clerk-ex01-20260616T221531Z/attestation/events` read the live `events.ndjson` object while the exec container wrote it to S3.  The endpoint first became available after the event object existed, then returned the live event stream through council deliberation.  The event stream recorded a council-member failure for `C1`, followed by completed votes from `C2`, `C3`, `C4`, and `C5`.

The final Clerk case record reported `status=completed`, process exit code `0`, AAR `status=ok`, and `resolution=no_majority`.  The result endpoint showed `C2` and `C4` voting `demonstrated`, while `C3` and `C5` voted `not_demonstrated`; `C1` exited before completing deliberation and was removed.  The driver terminated instance `i-01d3a6fc4495a2b62` after it saw the complete S3 artifact set.

Verification passed through the Clerk-attested path.  The verification log recorded `ok` for `manifest.sha384`, run id, mode, input mode, input prefix, AAR case id, output prefix, archive key, `run.log` SHA-384, archive SHA-384, archive byte count, container image id, container tar hash, `aar_example`, signature, user data, PCR4, PCR7, and PCR12.  The manifest SHA-384 was `c86a0ce5faae268d6fafb5359a8ce3dc4f72f4d608a679bc75c5d28a56c389423880be966ea4b0132df8ada9fbb82955`, and the extracted output directory was `/tmp/arb-clerk-ex01-20260616T215547Z/clerk-ex01-20260616T221531Z/aar-output`.

Two earlier attempts identified setup defects before the accepted run.  The driver treated an empty S3 output prefix as a fatal error because it used `aws s3 ls`; `tools/run-arb-attested.py` now lists the prefix with `s3api list-objects-v2` and accepts an empty object set.  The runtime launcher on `dev` was stale and passed `ARB_GLUE_MODE=aar`, while the current container entrypoint expects `ARB_EXEC_MODE=aar`; `/home/ec2-user/attest/run-aar.sh` was replaced with the checked-in launcher and verified by SHA-384 before the accepted run.

## 2026-06-17

### Pi container cleanup

A local direct `ex01` run restored the signature and public key evidence, then closed with `status=ok` and `resolution=not_demonstrated`.  C4 looped while trying to submit a Pi MCP vote, exceeded the council output limit, and AAR removed the council member from the case.  The local `podman run --rm` client was killed, but the unnamed Pi container kept running because AAR had no container name to remove during cleanup.

The runner now gives each Pi council container a deterministic AAR name and records that name with the process record.  Cleanup removes named containers through the same runtime command that launched them, even when the local client process has already exited.  The output-limit monitor removes the named container before it kills the local client process, which prevents the client-kill path from orphaning a live Pi container.

The runtime test pass also corrected the `runtime/cmd/aar` service black-box fixture.  The service API requires `out_dir` to be an immediate child of the configured service output root, but those tests passed sibling directories under the broader fixture root.  The fixture now exposes the service output root and derives service case directories from it, so `go test ./runtime/...` exercises the current service path.

## 2026-06-30

### Local direct example batch

The local direct `examples/ex*` batch stopped at `out/local-direct-three-per-ex-only-20260629/ex08a/run-02` because `.bin/aar run` returned exit code `1` after the case had already closed.  The case artifacts show a completed run: `run.json` reports `status=ok`, `phase=closed`, and `resolution=demonstrated`, with council votes from C1 through C5.  The stderr file contained three upstream OpenRouter retry lines, the private case API address, and then `Get "http://127.0.0.1:34231/councilapi/v1/get?case_id=arb-ex08a-20260630053412&member_id=C3": dial tcp 127.0.0.1:34231: connect: connection refused`.

The root cause was a lifecycle race in `runtime/localrun/localrun.go`.  After a Pi council process exited, `handleCouncilProcessExit` checked `/councilapi/v1/get` to decide whether the council member needed a failure report.  In this run, the case had closed and the private case API listener had stopped, but the main runner selected the agent-error channel before it selected the completed case outcome.

The runner now waits briefly for a completed case outcome before treating an agent-error signal as fatal.  If the case outcome arrives, the runner writes the normal run summary and returns the case result.  If no case outcome arrives within the short wait, the runner returns the agent error, so active-case agent failures remain fatal.

## 2026-07-03

### Juror replay persona experiments

`aar juror-replay` now runs one fresh Pi deliberation from an existing AAR output packet with an explicit model config and persona file.  The command uses the existing council replay executor, which preserves the frozen Council API, MCP server, Pi container path, prompt rendering, evidence access controls, and replay output files.  The separate command keeps the experiment interface focused on model-plus-persona replay while leaving `aar council-replay` available for same-spec replay.

The command prefers captured council-turn snapshots when it can identify one from `--snapshot` or a unique `--member-id` match under `council-turns/`.  If the source output predates snapshots, the command uses `reconstructed_first_round`, which rebuilds a first-round deliberation from durable output files.  Ambiguous snapshot selection fails with a specific error so a replay cannot use the wrong saved turn.

The implementation adds a strict persona override to the local replay config loader.  With `--persona`, the loader parses the model config as a JSON request-spec record, reads the supplied persona path, rejects missing or empty persona files, stores the absolute persona path in the request spec, and passes the resulting seat to the existing replay builder.  The command writes `juror-replay.json` with the source output, selected snapshot, model config path, persona path, persona SHA-256, vote, rationale, and tool-call count.

Replay cleanup now runs through a deferred secret cleanup path after the replay run state exists.  This covers successful replay, failed model calls, failed Pi process exit, and Pi startup errors after `writePiConfig` has created `.mcp.json` or Pi auth files.  The focused startup-failure test forces the container command to fail and verifies that generated replay secret files are absent afterward.

Focused tests cover persona override loading, missing and empty persona failures, snapshot discovery by member id, ambiguous snapshot rejection, and fallback to reconstructed replay when no snapshot directory exists.  The first real test should use one existing `ex*` output with `council-turns/`, a model config derived from that run's `council.json`, and one persona from `evals/model-pool/personas/experiments`.

## 2026-07-09

### Live Clerk evidence manifests

Manual service testing found that `/clerk/v1/cases/{case_id}/evidence/{evidence_id}` returned `manifest_missing` during active long Clerk runs after lawyers had submitted evidence.  Submitted evidence bytes entered `evidence-store/` and live run state immediately, but `evidence-manifest.json` was written only during final packet rendering.  The service route uses the manifest to map an evidence id to stored bytes, so it could not fetch live evidence before terminal rendering.

The proceeding now writes `evidence-manifest.json` when the evidence registry initializes and after each accepted submitted-evidence item.  The manifest writer uses a temporary file in the output directory and renames it into place, so the service route does not read partial JSON during a concurrent write.  The Clerk and direct service evidence routes now report an active missing manifest as HTTP `409` with error code `evidence_manifest_pending`; terminal packets without a manifest still return HTTP `404` with error code `manifest_missing`.

## 2026-08-05: Carve documentation and command audit

Historical journal references to deliberately removed service, adapter, launcher, and experimental files now retain their labels and paths without dead Markdown links.  The case-failure test plan now describes the retained direct `aar case`, Council API, and proceeding tests, including their current test names and expected records.  The ARB tree contains 82 checked local links with no missing target, and every tracked shell example parses under `bash -n`.

The command audit found that `aar help complain` and `aar help validate` printed usage but returned `flag.ErrHelp`, causing exit status 1.  Both subcommands now treat requested help as success, matching the other retained commands and the manual.  `TestHelpTopicsSucceed` covers every root help topic, and the complete `runtime/cmd/aar` package test passes.

## 2026-08-05: Final carve and service verification

The documented `make build test prove` targets pass with Lean 4.32.0, and the root Go test and vet suites pass.  The paired ARB compatibility package starts `aar case` from `carve@0c4162fcd985fa0888893f1e25088e9600bdb207` through `service@622daf6feeca4d12b1d0a3aae3768e532a0c38af`, completing Clerk and MCP cases and the retained failure cases.  The retained `examples/ex01` complaint passes command-line validation, and the final dependency audit finds no service package in the core module.
