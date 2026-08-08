# ADC Acceptance Example

This directory contains a compact documentary civil case.  Peter alleges that Samantha misrepresented having read assigned source material before completing paid writing work.  The record includes the assignment, admission, signature material, invoices, work orders, and time records.

The example exercises complaint drafting, case-file staging, signature analysis, trial presentation, and judgment.  Its files remain small enough for repeated command-line runs.  The case still requires each role to inspect and reason from documentary evidence.

## Inputs

| File | Purpose |
| --- | --- |
| `situation.md` | Narrative source for complaint drafting. |
| `instructions.txt` | Assignment record. |
| `confession.txt` | Samantha's written admission. |
| `confession.sig.b64` | Base64-encoded detached signature. |
| `samantha_public.pem` | Public key for signature verification. |
| `printing-invoice.txt` | Printing charges. |
| `distribution-work-order.txt` | Packaging and distribution charges. |
| `time-and-token-log.txt` | Cleanup time and model usage. |
| `damages-breakdown.txt` | Claimed damages. |

## Run

Run these commands from `adc/`.  The signing script regenerates the key and detached signature material.  Complaint drafting and adjudication use the configured OpenAI-compatible model client.

```bash
make build
make test
make prove
examples/ex1/sign.sh
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1
```

The output directory contains complaint-preparation files and the complete adjudication record.  `run.json` and `state.json` preserve the terminal result, while `events.ndjson`, `run.db`, `transcript.md`, and `digest.md` provide event, database, and written views.  `certificate.json` can be checked with `.bin/adc verify-certificate --dir out/ex1`.

The signature verifies that `confession.sig.b64` authenticates the bytes in `confession.txt` under `samantha_public.pem`.  Attribution of that public key to Samantha depends on the rest of the case record.  A legal filing or technical report must preserve that distinction.
