# Common Tools

This directory contains shared documentation and proof-maintenance tools for the core procedures.  Each program reads procedure source or documentation and writes a derived document or diagram.  The runtime and command packages do not invoke these programs.

## Files

| File | Role |
| --- | --- |
| `gendiagram.sh` | Generate diagrams from source files. |
| `gentheorems.py` | Sort a theorem catalog and generate its Markdown table.  `--sync-proofs DIR` first replaces the catalog with the public theorem and lemma declarations under `DIR`, retaining annotations for declarations that remain. |
| `proofstats.sh` | Summarize proof statistics. |

## Theorem catalogs

Run `gentheorems.py` from a procedure directory.  With no positional paths, it reads `theorems.tsv` and writes `docs/theorems.md`.  The synchronization form scans the direct `*.lean` children of the proof directory, requires a namespace in every file, records top-level `theorem` and `lemma` declarations, and retains importance and comment fields when the filename and local declaration name still match.

```bash
uv run python ../common/tools/gentheorems.py --sync-proofs engine/Proofs
```

Supplying `TSV MARKDOWN` selects other input and output paths.  Omitting `--sync-proofs` sorts the existing TSV and regenerates its Markdown table without scanning Lean source.
