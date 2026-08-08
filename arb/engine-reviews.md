# ARB Engine Reviews

Reviews of the ARB Lean engine (`engine/`).  Each review records its scope, what it verified, and candidate improvements at the time of writing.

## 2026-07-24

Scope: a full read of `Main.lean`, the replay and verification path on both sides of the process boundary (`Proofs/Replay.lean`, `cmd/aar/verify_certificate.go`, `proceeding/certificate.go`), and a sample of the proof tree (`StepPreservation`, `Replay`, `CertificateSoundness`, `NoStuck`, `InitializeCase`).  The proof tree is 21k lines across 40 files; this review read its structure and several files, not all of it.

### Strengths

The engine is a single 945-line pure core: structures, validation, transition functions, and a thin stdin/stdout JSON boundary, with determinism falling out of purity.  The proof tree has no `sorry`, no `axiom`, and no `unsafe`, and it reasons at the real boundary: theorems quantify over `stepCore` on actual `Json` payloads rather than an idealized model, and replay soundness composes reachability with outcome soundness at the `checkReplayCertificate` function.  Proof files carry substantive explanatory docstrings.  `docs/proof-notes.md` matches what this review read where it checked.

### Improvements

1. Run the proven verifier.  `checkReplayCertificate` carries the soundness theorems and never executes.  The operational verifier is a Go loop (`replayCertificateActions`) that drives the engine binary once per action and compares canonical-JSON SHA-256 hashes, so the trusted base for verification includes the Go loop, its per-step orchestration, and its hash canonicalization.  A `verify_certificate` request type in `Main.lean` that decodes the certificate and runs `checkReplayCertificate` in one engine invocation would make the proven function the executable verifier.  The Go side keeps the hash binding to `state.json` and S3 artifacts.  The function exists; the cost is a decoder and a dispatch arm.

2. Close the byte-limit gap.  `max_report_title_bytes` and `max_report_summary_bytes` are validated as policy values in Lean and enforced only in Go (`attorney_tools.go`).  The engine accepts reports of any size, so a replayed certificate can contain material violating the recorded policy while every proof about material limits speaks of counts.  The titles and summaries are in the payload the engine already parses, so enforcement in Lean is direct.  `max_exhibit_bytes` differs: the engine never sees exhibit bytes, so that limit should either leave the engine policy or be documented as runtime-only.

3. JSON round-trip lemmas.  States and certificates cross the process boundary as JSON while the proofs are about structural values.  `FromJson (ToJson s) = s` for `ArbitrationState` and the request types is the unproven seam between the proven model and the wire format.  Mechanical but sizable for derived instances.

| Further candidates | Substance |
| --- | --- |
| `decide` over `native_decide` | Seven proof files trust the compiled evaluator for small concrete samples; the kernel evaluator likely handles them and shrinks the trusted base. |
| Inductive phase, status, vote, and role types | Strings make illegal states representable and force `invalid_phase` branches and string hypotheses through the proof tree.  Real payoff, rewrite-scale churn, wire format preservable through codecs.  A design decision to discuss before attempting. |
| One turn-order function | Turn selection is encoded in both `nextOpportunityForPhase` and each `stepCore` handler; the `OpportunityAgreement` proofs pin them together, so this is deduplication that would retire those lemmas rather than a correctness fix. |
| `getOptionalString` swallows type errors | `{"label": 5}` becomes `""` instead of an error, so a malformed optional field is silently accepted. |
| `failOpportunity` style | A six-level match pyramid where the rest of the file uses `do` notation. |

### Boundary Notes

The engine trusts caller-claimed `size_bytes` and `sha256` on submitted evidence.  It cannot do otherwise, since it never sees file bytes, and the Go layer owns that verification.  That boundary is inherent; the engine documentation should state it.
