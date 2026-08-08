# Updating `analysis/`

`analysis/` holds Mermaid source files and paired Markdown explanations.  The `.mmd` files are the source of truth.  Rendered PNGs are derived local evidence and are not checked into this repository.

## Contents

| Source | Local render | Subject |
|---|---|---|
| `lean-simple-flow.mmd` | `lean-simple-flow.png` | minimal Lean request loop: initialize, role view, next opportunity, apply decision, step |
| `lean-complete-flow.mmd` | `lean-complete-flow.png` | detailed Lean control flow: request routing, filing, pretrial, trial, post-judgment, and guards |
| `formal-pipeline.mmd` | `formal-pipeline.png` | complaint-or-scenario entry path into the Go runner and Lean loop |
| `demo-lifecycle.mmd` | `demo-lifecycle.png` | end-to-end demo lifecycle: complaint staging, memos, scenario, direct or external turns, and final result |
| `role-view-enforcement.mmd` | `role-view-enforcement.png` | role visibility and decision validation loop for local or external agents |
| `state-phase-transitions.mmd` | `state-phase-transitions.png` | allowed case-status and trial-phase transitions, plus Rule 56 and bench-only notes |

## What to change

Update the Mermaid source that matches the concept you changed.  Do not edit rendered PNGs directly.

The filenames already divide the topics cleanly:

- Lean request and control flow: `lean-simple-flow.mmd`, `lean-complete-flow.mmd`
- Go-to-Lean execution path: `formal-pipeline.mmd`, `demo-lifecycle.mmd`
- Visibility and agent interaction: `role-view-enforcement.mmd`
- State and phase transitions: `state-phase-transitions.mmd`

If a change affects more than one layer, update every affected diagram.  Example: a new opportunity-validation step may affect both `lean-complete-flow.mmd` and `role-view-enforcement.mmd`.

## Regeneration

Requirements:

- `mmdc` in `PATH`
- Chromium or Chrome in `PATH`

The repository helper is [`../common/tools/gendiagram.sh`](../../common/tools/gendiagram.sh).  It takes one Mermaid file and one PNG output path.

Regenerate one diagram:

```bash
../common/tools/gendiagram.sh analysis/lean-simple-flow.mmd analysis/lean-simple-flow.png
```

Regenerate all diagrams in `analysis/`:

```bash
for f in analysis/*.mmd; do
  ../common/tools/gendiagram.sh "$f" "${f%.mmd}.png"
done
```

The helper defaults to `MMDC_SCALE=4`.  The analysis diagrams are dense, and the lower scales produce PNGs that are too small to read comfortably.

If the rendered text is too small or too large, rerun with `MMDC_SCALE`:

```bash
MMDC_SCALE=4 ../common/tools/gendiagram.sh analysis/lean-complete-flow.mmd analysis/lean-complete-flow.png
```

## What to verify

After regeneration:

1. Open each PNG and check for clipped labels, overlapping edges, and unreadable text.
2. If you rendered PNGs locally, check that every `.mmd` still has a matching `.png`.
3. If you changed Lean control-flow diagrams, compare the labels against the current request names and action families in `engine/Main.lean` and `runtime/runner`.
4. If you changed agent or visibility diagrams, compare the labels against the current Role API operations in `runtime/runner/roleapi.go`.

## Downstream docs

`analysis/` contains the canonical Mermaid sources and their paired explanations.  [`docs/logic.md`](../docs/logic.md) links to those analysis pages rather than to rendered image files.
