# ARB Engine Reviews

Reviews in this file cover the ARB Lean engine under `engine/`.  Each entry states the code and proof areas examined, the verification boundary checked, and remaining technical work.  Generated proof counts come from `docs/proofstats.md` at the review date.

## 2026-08-20

This review examined the current 1,197-line `Main.lean`, the replay and verification path in `Proofs/Replay.lean`, `Proofs/CertificateFacts.lean`, `runtime/cmd/aar/verify_certificate.go`, and `runtime/proceeding/certificate.go`, and the record-integrity work in `Proofs/RecordIntegrity.lean` and the Go evidence runtime.  It sampled the dependent initialization, preservation, liveness, termination, and outcome proofs affected by catalog validation rather than re-reading every proof file.  The generated inventory contains 39 proof files, 721 theorem or lemma declarations, and 22,858 lines.

### Strengths

The engine keeps transition and verification logic in pure functions behind a thin stdin/stdout JSON boundary.  Public-step theorems reason about the executable `stepCore` functions and their `Json` payloads, while replay soundness composes reachability with outcome soundness through `checkReplayCertificate`.  A source scan found no `sorry`, `axiom`, or `unsafe` declaration in the proof tree.

The engine validates report title and summary limits against their stored UTF-8 encodings.  It validates each offered exhibit against the size commitment in the immutable initial catalog or the submitted-evidence list present before the filing.  `RecordIntegrity` proves final-state closure, while `MeritsOfferChronology` proves that each accepted merits filing used the record available in its source state.

### Remaining Findings

1. Run the proven verifier.  `checkReplayCertificate` carries the soundness theorems but the engine command does not execute it.  The operational verifier is a Go loop, `replayCertificateActions`, that drives the engine binary once per action and compares canonical-JSON SHA-256 hashes.  Its trusted base therefore includes the Go loop, per-step orchestration, and hash canonicalization.  A `verify_certificate` request in `Main.lean` could decode the certificate and run `checkReplayCertificate` in one engine invocation, while Go retains the hash binding to `state.json` and stored artifacts.

2. Prove JSON round trips.  States and certificates cross the process boundary as JSON while the proofs concern structural values.  Lemmas establishing `FromJson (ToJson s) = s` for `ArbitrationState` and the request types would cover this boundary.

| Further candidate | Substance |
| --- | --- |
| `decide` over `native_decide` | Seven proof files trust the compiled evaluator for small concrete samples.  The kernel evaluator may handle them and reduce the trusted base. |
| Inductive phase, status, vote, and role types | Strings make illegal states representable and carry string hypotheses through the proof tree.  This change would require a broad rewrite, although codecs could preserve the wire format. |
| One turn-order function | `nextOpportunityForPhase` and each `stepCore` handler encode turn selection separately.  `OpportunityAgreement` proves their agreement; sharing one definition could retire those lemmas. |

### Boundary Notes

The public Lawyer API accepts `parent_evidence_id` and `derivation_method` for derived evidence and rejects a caller-supplied `parent_sha256`.  The Go runtime hashes the submitted bytes, resolves the named parent's committed digest from visible record evidence, and sends the Lean engine a `submit_evidence` payload containing the child size and digest plus all three lineage strings.  Lean checks canonical digest syntax, identifier freshness, lineage completeness, parent commitment equality, and filing-time exhibit references against engine state.  Certificate replay proves that engine-visible history, while byte-level custody inspection uses the evidence manifest and stored files.
