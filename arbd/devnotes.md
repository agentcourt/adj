# Development Notes

## 2026-08-05: Concurrent fake-engine execution

The council fixtures now pass their fake engine program to `/bin/sh -c` instead of writing and executing a temporary shell file.  This matches the ARB correction for the `ETXTBSY` race documented by the Go standard library in [issue 22315](https://go.dev/issue/22315).  The change retains the subprocess and standard-input boundary exercised by these tests.

Both ARB and AARD passed 100 repetitions of the two parallel council-removal tests.  The complete core Go suite, build, and vet checks also pass.  Commit `ef540ff` contains the symmetric fixture correction.

## 2026-08-02: Acceptance example reduction

AARD retains `examples/ex1` as its command-line acceptance case.  The removed `examples/ex2` and `examples/ex3` supplied additional questions through the same complaint and case-file interface.  The retained example now documents validation, one-case execution, output records, and certificate verification.

The complete Go test, build, and vet checks pass after the reduction.  `aard validate --complaint examples/ex1/complaint.md` accepts the retained complaint and its case-file references.  A live case continues to require the model credentials documented by the example.

## 2026-08-02: Core reduction

The AARD manuals and supporting documents now describe one-case execution and the case-owned Lawyer and Council APIs.  Service, MCP, OpenClaw, Pi, attestation, and deployment instructions remain in the `adjservices` repository.  The Makefile now reports the retained `aard case` command instead of deleted local-agent commands.

The council and evidence documents describe direct model work and external HTTP clients without naming a transport adapter.  The obsolete MCP specification and OpenClaw attorney notes were removed from `carve`.  Procedure rules, practice guidance, policy notes, evidence custody, records, and certificate verification remain.

Verification completed with `go test -buildvcs=false -count=1 ./...`, `go build -buildvcs=false ./...`, `go vet ./...`, command help checks, and `git diff --check`.  The retained tests cover direct and external lawyers, external council members, numeric answers, evidence custody, failures, and replay.  No AARD implementation package imports service code.

## 2026-08-02

### Lean 4.32.0

Reference: [Lean 4.32.0 release notes](https://lean-lang.org/doc/reference/latest/releases/v4.32.0/)

The AARD Lean engine now selects Lean 4.32.0.  Lean 4.32.0 makes the new `do` elaborator the default, which changes the reductions exposed to the replay proof.  The revised proof reduces the monadic operations explicitly and applies `List.concat_eq_append` when comparing replayed action lists.

Verification:

- [x] `lean --version` reports Lean 4.32.0.
- [x] `lake build` completes all 15 AARD engine and proof jobs.

### Core and service branch split

Reference: [Core and service branch plan](https://github.com/jsmorph/adjudication/blob/carve/plan.md)

The `carve` branch retains AARD's Lean engine, proofs, one-case Go runtime, case-owned Lawyer and Council APIs, durable records, and certificate verification.  The `service` branch receives the multi-case Clerk service, MCP adapter, local OpenClaw and Pi launchers, agent templates, attested execution, Docker deployment, web programs, and their operational support.  The branches communicate through documented executable, HTTP, and artifact interfaces, with tested commit pairs recording compatibility.

Service commits `4b4fa1751fa4b8a1e709b3f80ad1cbcbc6eaa581` and `6eaed038b468add7099b77edb766b987ba053dcd` contain the extracted AARD multi-case service and MCP adapter.  Service commit `25dac0e20c08ffa730a661eb4080677bd3bdfaa7` contains `aard-run`, the local-agent launcher, its templates, and the paired process-interface test.  Service commit `48d19263fde43f010312cb446cd4d6970a019c4f` retains the Clerk and MCP cases formerly mixed into the AARD command black-box tests and passes them against the real `carve` executable and Lean engine.  The direct one-case failure tests remain on `carve`, while the service and MCP variants can now leave the core command package.

### Core command black-box boundary

The AARD command package now contains only the direct black-box cases for lawyer attempt exhaustion, lawyer deadline expiration, and runtime failure exit behavior.  The service and MCP cases and their process helpers left `carve` after service commit `48d19263fde43f010312cb446cd4d6970a019c4f` preserved and ran them.  The retained fixture now uses the built `aardengine` filename and reports provider I/O, HTTP body closure, process-log closure, and process-cleanup errors.

Verification:

- [x] `go test -buildvcs=false -count=1 -run '^TestBlackBox' ./arbd/runtime/cmd/aard`
- [x] `go vet -buildvcs=false ./arbd/runtime/cmd/aard`

### Multi-case service removal

The AARD multi-case service package and `aard service` dispatch entry have left `carve` after service commits `4b4fa1751fa4b8a1e709b3f80ad1cbcbc6eaa581` and `48d19263fde43f010312cb446cd4d6970a019c4f` preserved the service-owned implementation and cross-branch tests.  The AARD root help now lists the retained core and temporarily duplicated adapter commands without advertising the Clerk service.  The command package passes its tests and builds without an import of the removed service package.

Verification:

- [x] `go test -buildvcs=false -count=1 ./arbd/runtime/cmd/aard`
- [x] `go test -buildvcs=false -count=1 ./arbd/runtime/...`
- [x] `go vet -buildvcs=false ./arbd/runtime/...`
- [x] `go build -buildvcs=false -o /tmp/carve-service-removal-bins/aard ./arbd/runtime/cmd/aard`

### Operational adapter removal

The AARD `run` and `mcp` commands, their runtime packages, and their OpenClaw and Pi templates have left `carve`.  Service commits `6eaed038b468add7099b77edb766b987ba053dcd` and `25dac0e20c08ffa730a661eb4080677bd3bdfaa7` retain the standalone adapter, launcher, embedded templates, package tests, and paired core-process checks, while `48d19263fde43f010312cb446cd4d6970a019c4f` retains a complete MCP case.  The AARD command now builds and tests without imports of either removed runtime package.

Verification:

- [x] `go test -buildvcs=false -count=1 ./arbd/runtime/...`
- [x] `go vet -buildvcs=false ./arbd/runtime/...`
- [x] `go build -buildvcs=false -o /tmp/carve-core-only-bins/aard ./arbd/runtime/cmd/aard`

### Case-packet ownership

Deterministic AARD case-packet construction remains on `carve` as part of the core input interface.  The builder uses the proceeding package's automatic and explicit case-file selection rules, preserving one implementation of input validation.  Attested service code will invoke the installed `aard case-packet` command without importing or copying that implementation.

### Core and service split: attested execution

Service commit `6a4441006b1f81ae573e9754c76e3c3f185d9e34` moved the AARD attested images, entrypoint, drivers, scripts, tests, and runbooks under `service/attested/arbd`.  Service commit `5bf19d083ea670cdbf767ccf304c81ec712f90d4` corrected the image's council-data copy and built against `carve@489cd9ecdb46f41f036d825fc9f5221f68c13c1a`.  Image `sha256:a35e3e6bdd8090f890ab3cdf9d877fbcc758670cce1b72b5ad25c56243d83264` displayed the service `aard-run` help and validated the embedded `ex1` complaint through the installed core executable.

The core branch retains `aard case-packet`, its deterministic proceeding builder, `aard case`, the case-owned Lawyer and Council APIs, durable records, and certificate verification.  The AARD Dockerfiles, exec entrypoint, attested Python and shell programs, and operational runbooks have left this branch.  The core README and documentation index no longer link to those removed files.

Verification:

- [x] `go test -buildvcs=false -count=1 ./arbd/runtime/...`
- [x] `go vet ./arbd/runtime/...`
- [x] `go build -buildvcs=false -o /tmp/carve-aard-attested-removal ./arbd/runtime/cmd/aard`
- [x] The freshly built command lists only `case`, `case-packet`, `complain`, `validate`, and `verify-certificate`.
- [x] Two fresh `aard case-packet` runs over `arbd/examples/ex1/complaint.md` produced byte-identical archives and manifests, and the archive listed the complaint, two sonnets, case arguments, and embedded manifest.

## 2026-07-13

### Replay certificates

Reference: `runtime/proceeding/certificate.go`, `runtime/cmd/aard/verify_certificate.go`, `runtime/service/service.go`, `manual.md`

AARD now writes `certificate.json` beside `run.json` and `state.json` in terminal output directories.  The certificate records the initial state, degree question, council roster, accepted engine actions, claimed final state, and compact JSON final-state hash.  `aard verify-certificate` reads `certificate.json` and `state.json`, replays the initialization request and accepted actions through the Lean engine, and reports the accepted action count plus final-state hash when replay succeeds.

The service artifact list now includes `state.json` and `certificate.json` for direct case and Clerk artifact routes.  The service still treats verification as an operator action through `aard verify-certificate`; case creation, listing, result polling, and artifact reads do not run replay verification.  Focused tests cover replay acceptance, packet-state mismatch, rejected replay actions, accepted-step recording, and artifact allowlist behavior.

Manual HTTP testing of a real AARD case found that the council prompt path still used the package-level default prompt directory when the process was started from the repository root.  The lawyer prompts already respected `--prompt-dir`, but the council prompt entered deliberation through `renderPromptFile` rather than the `Config` method.  `buildCouncilPrompt` now uses the configured prompt resolver, and a focused test covers a custom council prompt directory.

The live test ran `aard case` with the real `aardengine`, `councilapi`, one council member, and `arbd/examples/ex1`.  Manual Lawyer API calls filed both openings, both arguments, a pass sequence, and both closings; a manual Council API call submitted answer `68`.  The run wrote `/tmp/aard-cert-live-20260713d`, and `aard verify-certificate --dir /tmp/aard-cert-live-20260713d` accepted 9 recorded actions with final-state hash `27c942a1039b9b2b6c2b0eb93fb24f44adef0b1eb64f1f684ce3148622a026b6`.

The AARD proof layer now has certificate modules under `engine/Proofs/`.  `Reachability.lean` defines valid engine histories, `Replay.lean` defines initialized replay and accepted-certificate checking, `CertificateFacts.lean` packages closed and failed terminal facts, and `CertificateExamples.lean` checks complete closed and failed sample certificates.  The closed facts expose the answer pairs from the replayed final state, and the failed facts expose the replayed `OpportunityFailure` record.  Those facts match the runtime's current terminal reports while leaving aggregate-degree rules for later AARD design work.

## 2026-07-10

### Service process record reconciliation

Reference: `runtime/service/service.go`, `runtime/service/clerk.go`, `runtime/service/service_test.go`, `manual.md`

The service now gives child processes direct stdout and stderr log file descriptors instead of copying pipe output through the service process.  This lets the child continue writing logs if the service process exits, and it removes pipe-copy goroutines from the lifecycle path.  Completion still reads the stdout log after the child exits to populate the service summary.

Clerk listing and lookup now reconcile detached active-looking `clerk.json` records before returning them.  If `run.json` exists, the service marks the record completed or failed from that artifact and persists the repaired record.  If no terminal artifact exists, the service marks the record failed with `service restarted and child process is not attached`; it does not reattach to a process.

The direct service registry uses the same restart rule.  Active or previously detached records are repaired from `run.json` when it appears, and the repaired registry record is written back to disk.  Focused tests cover Clerk record repair, detached active Clerk failure, and direct registry repair.

### Service API error cleanup

Clerk create now checks `examples/EXAMPLE/complaint.md` before reserving an output directory or starting a child process.  Missing examples return `unknown_example`, and invalid example names return `invalid_example`.  Artifact reads now distinguish names outside the allowlist from listed artifacts whose files are absent: the first returns `unknown_artifact`, and the second returns `artifact_missing` without host filesystem paths.

### Live evidence manifest routing

Reference: `runtime/proceeding/evidence.go`, `runtime/proceeding/lawyerapi.go`, `runtime/service/service.go`, `runtime/service/clerk.go`

Manual web-console testing of a real Clerk AARD run found that the evidence page could not fetch initial case-packet evidence during the active run.  The Clerk evidence route returned `manifest_missing` because AARD wrote `evidence-manifest.json` only during final packet rendering, while the service route depended on that manifest to map `evidence_id` values to stored bytes.  The service route also still expected the old legacy manifest list shape, so it would have failed against the current terminal manifest after the run completed.

AARD now matches the AAR behavior.  The evidence registry writes `evidence-manifest.json` at initialization, direct evidence submission and chunked upload commits rewrite it after accepted evidence changes, and final rendering uses the same manifest writer.  The service route reads both legacy and current manifest shapes, serves content-addressed `evidence-store/` paths, and reports active missing manifests as `evidence_manifest_pending` instead of terminal `manifest_missing`.

### Verification

- [x] `go test ./arbd/runtime/service`
- [x] `go test ./arb/runtime/... ./arbd/runtime/... ./adc/runtime/...`
- [x] `go test ./arbd/runtime/proceeding ./arbd/runtime/service`

## 2026-06-17

### Manual review

Reference: `manual.md`, `Dockerfile.md`, `docs/attested-dev-host.md`

The manual states when AARD is the right procedure: one question, numeric answers from 0 through 100, and one output directory as the record for one run.  The guidance names the files an operator should preserve together, including `run.json`, `state.json`, `transcript.md`, `digest.md`, `events.ndjson`, `work-notes.ndjson`, and evidence files.  It also distinguishes AARD from AAR where the desired output is a binary demonstrated or not-demonstrated decision under an evidence standard.

The attested Clerk text points to the AARD Docker runbook and dev-host requirements for the image build, S3 layout, expected PCR values, and verification checks.  The troubleshooting section includes attested Clerk diagnosis: inspect the Clerk record, `/attestation/events`, driver logs, the S3 input prefix, the S3 output prefix, and verification files before relying on console output.  The existing events endpoint documentation remains in the Clerk section and has matching troubleshooting guidance.

### README review

The README uses the same documentation table as ADC, AAR, and evals.  It links to the manual, Docker runbook, dev-host requirements, practice guide, and rules.  It also names the main output files and points attested output details to the Docker runbook.

### Attested Clerk AARD test

The first real Clerk-managed attested AARD run used `case_id=clerk-attested-aard-ex1-20260617T005035Z` and `run_id=aard-ex1-20260617T005035Z`.  The exec instance `i-019f74892947bb713` loaded `arbd-glue:poc`, downloaded `auth.json` and `keys.sh` from the S3 input prefix, and then failed before AARD produced `events.ndjson` or a verifiable attestation.  The terminal S3 objects were `run.log` and `aard-partial.tar.gz`.

The failure came from `arbd/attest/exec-container-entrypoint.sh` passing `--openclaw-network host` to `aard run`.  AARD lacked the corresponding AAR option, so the argument parser treated `host` as one example name and later treated `ex1` as a second example name.  The root error was `aard run accepts at most one example name`.  The fix is to add the AAR host-network option to AARD rather than remove it from the attested entrypoint, because the exec topology expects the OpenClaw lawyer containers to use host networking.

The second run used `case_id=clerk-attested-aard-ex1-20260617T010406Z` and `run_id=aard-ex1-20260617T010406Z`.  It passed the argument parser, imported the embedded Pi image, and then failed because `/opt/adjudication/common/data/personas/pool.jsonl` was absent from the image.  The runtime default looks for `./pool.jsonl` first and then `<common-root>/data/personas/pool.jsonl`; the repository only had `arb/pool.jsonl`, whose records refer to `personas/generic.md`.  The fix is to add the JSONL pool under `common/data/personas` and the generic persona under `common/etc/personas`, matching the documented runtime-pool default and avoiding an `arbd` dependency on `arb`.

The third run used `case_id=clerk-attested-aard-ex1-20260617T012752Z` and `run_id=aard-ex1-20260617T012752Z`.  It reached the rebuilt image and failed with `read persona text personas/generic.md: open /opt/adjudication/common/data/personas/personas/generic.md: no such file or directory`.  The loader resolves pool-relative persona paths first under the pool directory and then under `common/etc`, so the prior fix put `generic.md` in the wrong common directory.  The corrected tree stores `generic.md` at `common/etc/personas/generic.md`.

The fourth run used `case_id=clerk-attested-aard-ex1-20260617T013611Z` and `run_id=aard-ex1-20260617T013611Z`.  It reached OpenClaw and failed when the plaintiff container reported `EACCES` while reading `/aard-codex/auth.json`.  AARD still staged the mounted Codex home with mode `0700` and the token file with mode `0600`, while the working AAR path stages the directory as `0777` and `auth.json` as `0666` because OpenClaw reads the mounted files as a different container user.

The fifth run used `case_id=clerk-attested-aard-ex1-20260617T014658Z` and `run_id=aard-ex1-20260617T014658Z`.  The Clerk service completed the case, the driver downloaded `aard-output.tar.gz`, `attestation.b64`, `events.ndjson`, `manifest.json`, `manifest.sha384`, and `run.log`, and verification passed with PCR 4, PCR 7, PCR 12, signature, manifest hash, archive hash, and archive size checks.  The Clerk result reported answers `C1=74`, `C2=74`, `C3=83`, and `C4=76`; `C5` exited during deliberation and the AARD engine removed that council member, then closed the case with the remaining seated answers.  The exec instance `i-08a9c08482ca8408f` terminated after completion.

## 2026-06-04

### Service and agent runtime migration

Reference: AARD service and agent update plan (`../scratch/arbd/update-plan.md`)

`arbd` now has the same current runtime shape as `arb`: a direct `aard` command with `case`, `mcp`, `service`, and `run` subcommands; a private case HTTP API; role-bound Lawyer API and Council API endpoints; an MCP adapter over those APIs; a Clerk service; and a local run path that starts OpenClaw lawyers and Pi council agents.  The implementation keeps AARD degree semantics.  Lawyers argue for numeric answers or answer ranges, council members call `submit_council_answer` with an integer from `0` through `100`, and final result data exposes the answer map rather than a binary resolution.

The Lean engine now represents opportunity failure directly.  A lawyer failure marks the case status as `failed` with a stored reason, while a council member failure marks that member as failed and continues deliberation with the remaining seated members.  The process-level API test started `aard case`, drove both lawyer roles through HTTP, submitted three council answers through the Council API, and produced a successful answer map.

- [x] Add `runtime/proceeding` with private Case API, Lawyer API, Council API, result reporting, work notes, and degree-specific council answers.
- [x] Add `runtime/mcp`, `runtime/service`, `runtime/localrun`, and `agent-instructions`.
- [x] Replace the old `runtime/cmd/aard` dispatcher with direct subcommands.
- [x] Update `Makefile` and `README.md` for the supported API and local-run path.
- [x] Run `go test ./...`, `lake build Proofs`, `lake build aardengine`, a direct case API process test, and a service API process test.

## 2026-06-16

### Attested Clerk execution

AARD now mirrors AAR's attested Clerk execution path.  The service accepts `execution.mode: "attested"` for Clerk-created runs, resolves service-level attestation defaults, rejects unsupported local-run overrides, and requires verification before a Clerk record can reach `completed`.  Attested example input selects a checked-in `arbd/examples/<name>` case inside the image, while attested complaint input uses `aard case-packet` to package `complaint_path` and optional `case_files` into deterministic S3 input objects.

The AARD attested workload uses `run-aard.sh`, `run-arbd-attested.py`, `arbd/attest/exec-container-entrypoint.sh`, `arbd/Dockerfile`, and `arbd/Dockerfile.glue`.  The S3 prefixes use `aard-inputs` and `aard-runs`, the workload archive is `aard-output.tar.gz`, and failed remote runs can leave `aard-partial.tar.gz` for diagnosis.  Clerk artifact, result, evidence, and `attestation/events` routes read from `aard-output/` after verification and from the top-level downloaded attestation files where appropriate.

The AARD documentation uses the same operator document set as AAR: `README.md`, `manual.md`, `Dockerfile.md`, and `docs/attested-dev-host.md`.  The AARD-specific wrapper `tools/run-one-attested-arbd.sh` stages secrets, chooses timestamped S3 prefixes, invokes `run-arbd-attested.py`, and verifies the attestation.  The container proof script `tools/run-container-poc.sh` was copied for the AARD image name so the runbook file table names existing files.

- [x] Add `aard case-packet`.
- [x] Add attested Clerk config, request validation, driver command construction, completion verification, artifact routing, evidence routing, and `attestation/events`.
- [x] Add AARD Dockerfiles, exec entrypoint, local driver, exec runner, one-example wrapper, and container proof script.
- [x] Add AARD service tests for attested example input, attested complaint input, live event reads, unsupported local-run fields, and failed attested execution.
- [x] Run `sh -n` on the AARD shell scripts, `python3 -m py_compile` on `run-arbd-attested.py`, and `go test ./...` under `arbd/runtime`.

## 2026-05-02

### Initial fork from `arb`

`arbd/` began as a bounded fork of `arb/`, but it does not preserve the binary decision model.  The fork keeps the same merits sequence and the same council-member machinery.  It changes the complaint from `Proposition` to `Question`, changes the policy field from standard-of-evidence framing to `judgment_standard`, and changes the deliberation act from a binary vote to one integer answer in `[0,100]`.

The final result is the answer set.  The runtime exports that result as a Go map keyed by `member_id`.  The engine does not compute a threshold outcome, an aggregate, or a `no_majority` result.  Closure now follows one condition: every seated council member has answered once in the round.

### Proof scope

The copied `arb` proof files did not fit `arbd`, because those files proved properties of threshold closure, substantive-outcome viability, and binary neutrality.  The new proof set is narrow and direct.  It proves initialization rules, ordered merits sequence, bounded council answers, answer completeness on closure, and the current removal guards.

### Documentation boundary

This first version keeps the new tree limited.  It has one example, one proof batch, and one Makefile demo path.  It avoids importing `arb`'s binary-vote notes and examples under misleading names.

### Example 1 refresh

The first example now uses two sonnets rather than the earlier placeholder novelty prompt.  The record states that one sonnet was written in 2024 and that a very similar but not identical sonnet was written in 2025.  That makes the example fit the degree model directly, because the council has to answer how much of the later text was really the earlier text.

### Council answer transport

The first live council runs exposed a transport mismatch with the shared council pool, not a Lean defect.  `arb` asks council models for a string-valued tool argument, while the first `arbd` draft asked the same mixed pool for a JSON integer.  That difference was enough to trigger repeated invalid council submissions under the normal `make demo` path.

The current Council API accepts a numeric `answer` for `submit_council_answer` and stores the value as an integer before it calls the Lean engine.  `aard run` now gives Pi council agents full request-spec model configs and a degree-specific MCP instruction.  The engine and final run evidence still store numeric answers.

### Documentation set

`arbd/docs/` now mirrors the core non-proof `arb/docs/` set with procedure-specific replacements: `ARAP.md`, `councils.md`, `goals.md`, `params.md`, and `practice.md`.  The text stays close to the working implementation rather than speculating about later aggregation or convergence designs.

The proof-oriented `arb/docs/` files were omitted.  `arbd` has a smaller proof tree, and the user asked for the procedural and practical documents first.  The documentation review pass focused on direct statement, explicit procedure description, and removal of binary-outcome phrasing that did not fit the degree model.

### Example 2

`arbd/examples/ex2/` now follows the same narrow pattern as `ex1`, but with two short stories instead of two sonnets.  The 2024 story, `first-story.md`, describes a near-future city whose civic AI assigns small mercies.  The 2025 story, `second-story.md`, tracks the same plot, scene order, and motifs with paraphrastic substitutions and relabeled set-pieces.

`arbd/Makefile` now has an `ex2` target that prints the supported local command path.  The live path is `aard run ex2`, because `aard case` waits for Lawyer API and Council API clients.  Earlier live runs on this example produced a high answer spread consistent with its close textual overlap.

### Example 3

`arbd/examples/ex3/` reuses the same 2024 base story as `ex2`, but pairs it with a 2025 story that is only loosely related.  The second story keeps some of the same named places and people, along with the same near-future municipal-AI setting, but changes the conflict, the central mechanism, the scene sequence, and most of the phrasing.  The example tests whether common world-building and cast alone yield a high score.

`arbd/Makefile` now has an `ex3` target as well.  The live path is `aard run ex3`, because the direct case process waits for external clients.  Earlier live runs on this example produced lower answers than `ex2`, which fits the intended design of the example.

### Evidence fidelity and explicit file filtering

The first review pass found two runtime issues worth fixing.  First, the exported `council.json`, `run.json`, digest, and transcript used the initially sampled council list even after the Lean state had marked a member `timed_out` or otherwise removed.  That made the packet misleading in exactly the cases where a reader most needs the status history.

`arbd` now derives the exported council list from the final Lean state.  That keeps the packet aligned with the source of truth and carries each member's final `status` into JSON and the rendered markdown reports.  The runtime still keeps the sampled council list in memory for persona text during live execution.

The same review found that `--file .gitignore` slipped past `validateExplicitCaseFilePath`, because `filepath.Ext(".gitignore")` is empty.  The validator now rejects `.gitignore` by basename before it checks ordinary extensions.  That change affects only the explicit `--file` path, which is where the gap existed.

## 2026-05-04

### Flexible complaint input

Reference: [Complaint parser](runtime/spec/complaint.go)

The degree runtime needs one question string.  The source file no longer has to
carry a literal `# Question` heading for the parser to produce that value.  When
a `Question` section exists, the parser uses that section.  When no such section
exists, the parser treats the whole trimmed file as the question.

The canonical writer still emits a `# Question` heading.  That keeps generated
complaint packets stable and readable.  Empty input fails, and an explicit empty
`Question` section fails, because either case lacks a question.

- [x] Preserve canonical complaint output.
- [x] Accept plain text as complaint input.
- [x] Reject blank complaints and blank explicit sections.
- [x] Cover parser behavior in tests.

## 2026-08-05: Carve documentation and command audit

The historical link to the removed service migration plan now remains as a labeled path without a dead Markdown target.  The AARD tree contains 36 checked local links with no missing target, and every tracked shell example parses under `bash -n`.  The shell check also identified and corrected an unmatched single quote in the documented filing request.

The command audit found that `aard help complain` and `aard help validate` printed usage but returned `flag.ErrHelp`, causing exit status 1.  Both subcommands now treat requested help as success, matching the other retained commands and the manual.  `TestHelpTopicsSucceed` covers every root help topic, and the complete `runtime/cmd/aard` package test passes.

## 2026-08-05: Final carve and service verification

The documented `make build test prove` targets pass with Lean 4.32.0, and the root Go test and vet suites pass.  The paired AARD compatibility package starts `aard case` from `carve@0c4162fcd985fa0888893f1e25088e9600bdb207` through `service@622daf6feeca4d12b1d0a3aae3768e532a0c38af`, completing Clerk and MCP cases and the retained failure cases.  The retained `examples/ex1` complaint passes command-line validation, and the final dependency audit finds no service package in the core module.
