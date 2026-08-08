# Example 1

This example asks an authorship-and-reuse question about two sonnets.  The first file states that its author wrote the first sonnet in 2024.  The second file states that its author wrote a very similar but not identical sonnet in 2025.

The case asks how much of the second sonnet was really the first sonnet.  The record is narrow on purpose, because the point of the example is to force the council to reason from the texts themselves rather than from side issues.  The run exercises the `Question` complaint format, the case-file scan, and the final `member_id -> answer` map.

Run these commands from `arbd/`.  Validation checks the complaint and both referenced sonnets before adjudication starts.  Adjudication and council deliberation use the configured OpenAI-compatible model client.

```bash
make build
make test
make prove
.bin/aard validate --complaint examples/ex1/complaint.md
.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --council-pool ../common/data/personas/pool.jsonl \
  --out-dir out/ex1
```

The output directory contains the complete degree-adjudication record.  `run.json` and `state.json` preserve the terminal result, while `events.ndjson`, `work-notes.ndjson`, `transcript.md`, and `digest.md` provide event and written views.  Check `certificate.json` with `.bin/aard verify-certificate --dir out/ex1`.
