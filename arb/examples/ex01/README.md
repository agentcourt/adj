# ARB Acceptance Example

This directory contains a compact documentary arbitration.  The proposition concerns Samantha's representation about when an essay would be complete and the claimant's reliance on that representation.  The record includes the assignment, admission, messages, approvals, invoices, work orders, time records, damages, and detached-signature material.

Run these commands from `arb/`.  Validation checks the complaint and every referenced case file before adjudication starts.  Adjudication and council deliberation use the configured OpenAI-compatible model client.

```bash
make build
make test
make prove
.bin/aar validate --complaint examples/ex01/complaint.md
.bin/aar case \
  --complaint examples/ex01/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex01
```

The output directory contains the complete arbitration record.  `run.json` and `state.json` preserve the terminal result, while `events.ndjson`, `work-notes.ndjson`, `transcript.md`, and `digest.md` provide event and written views.  Check `certificate.json` with `.bin/aar verify-certificate --dir out/ex01`.
