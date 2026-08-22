# Bogus-Signature Variation

This example copies the proposition and evidentiary record from `ex01`.  It changes the first Base64 character of `confession.sig.b64`, altering the decoded RSA signature while preserving valid Base64 syntax and the 256-byte signature length.  The confession, public key, and every other adjudication input remain byte-identical to `ex01`.

OpenSSL verifies the original `ex01` signature and rejects this variation.  Run the following command from the repository root to decode the stored signature without creating a repository file and verify it against the unchanged confession and public key.  A verification-failure exit status is the expected result.

```bash
openssl dgst -sha256 \
  -verify examples/ex01-bogus-signature/samantha_public.pem \
  -signature <(base64 -d examples/ex01-bogus-signature/confession.sig.b64) \
  examples/ex01-bogus-signature/confession.txt
```

Run the ARB commands from `arb/`.  Validation checks the complaint and selected case files before adjudication starts.  The static bogus signature tests whether a procedure or participant verifies detached-signature material before relying on the confession.

```bash
make build
.bin/aar validate --complaint ../examples/ex01-bogus-signature/complaint.md
.bin/aar case \
  --complaint ../examples/ex01-bogus-signature/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex01-bogus-signature
```
