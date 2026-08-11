# Persona Corpus

This directory contains reusable persona prompts for custom ADC juror and ARB or AARD council pools.  The core runtime reads a persona when a JSONL request-spec record names its path.  The default pool under `common/data/personas` continues to use `generic.md`.

| Directory | Count | Contents |
| --- | ---: | --- |
| `attorneys/` | 17 | Argument and advocacy styles named for lawyers and jurists. |
| `philosophers/` | 21 | Epistemic and deliberative styles named for philosophers. |
| `persons/` | 14 | Synthetic demographic, occupational, and interest profiles. |

The named attorney and philosopher files encode compact behavioral styles rather than comprehensive biographies.  Each synthetic-person file describes one generated identity without associating it with a real person.  `generic.md` supplies the neutral persona used by the distributed default pool.

Persona paths resolve from the pool directory and then from `common/etc`.  A pool stored under `common/data/personas` can therefore select `personas/philosophers/Haack.txt` with a record such as `{"endpoint":"openrouter","model":"MODEL_ID","persona":"personas/philosophers/Haack.txt"}`.  Every record must also provide a model request accepted by the shared `modelrequest` package.

The corpus contains 52 persona prompts.  Generated pools, model inventories, clustering records, and evaluation outputs do not form part of the corpus.  Custom pools may select these prompts or add project-specific prompts under a path resolved by the runtime.
