# ADC Proofs

The ADC Lean package separates the reusable procedure from the executable protocol.  `engine/ADC/Core.lean` defines state, validation, opportunity generation, decisions, and transitions.  `engine/Main.lean` parses JSON requests, calls the core, and writes JSON responses.  Every proof module reaches `ADC.Core` through its import graph and belongs to a namespace under `ADCProofs`.

`engine/Proofs.lean` imports every maintained proof module.  A successful build of that target checks the complete maintained proof tree.  Individual proof files remain buildable for focused work.

```bash
cd adc/engine
lake build Proofs
lake build Proofs.Rule56
```

## Proof boundary

The proof tree establishes properties of the encoded procedure.  Its main subjects are validation, role and opportunity authority, state preservation, phase progression, docket effects, decision confinement, replay, certificate facts, and representative terminal outcomes.  The proofs establish consequences of the Lean definitions and the certificate input.  They do not authenticate a certificate, establish the truth of evidence, or prove that a participant reached a sound legal or factual judgment.

The modules use three principal proof forms:

| Form | Use |
| --- | --- |
| Symbolic theorem | Establish a property for arbitrary values satisfying stated hypotheses. |
| Computed theorem using `native_decide` | Check an exact finite state, transition, validator result, or certificate example. |
| Certificate theorem | Derive state and outcome facts from successful replay or certificate checking. |

Computed theorems provide exact checks for concrete states.  They do not generalize beyond the values in their statements.  The theorem catalog identifies each declaration by its name and source file.

## Organization

The proof files group related results rather than reproduce the engine's source order.  The main groups cover foundational limits and time functions, procedural domains such as discovery and jury instructions, opportunity generation and decision application, transition invariants, replay, and certificate consequences.

Every public declaration has a fully qualified name such as `ADCProofs.Rule56.step_decide_rule56_records_order`.  The [theorem catalog](theorems.md) lists all public theorem and lemma declarations.  [Proof statistics](proofstats.md) report counts by file and broad category.  The [ARCP matrix](ARCP-matrix.md) records executable rule coverage.  The [proving notes](proving.md) describe current proof-maintenance decisions, and the [Lean proving guide](provingguide.md) summarizes the available proof methods.

## Maintenance

Run the proof target after changing `ADC/Core.lean` or a proof module.  Regenerate the derived documents from the `adc/` directory after adding, renaming, moving, or deleting proof declarations.

```bash
../common/tools/proofstats.sh
uv run python ../common/tools/gentheorems.py --sync-proofs engine/Proofs
```

The synchronization command removes catalog rows whose declarations no longer exist, adds current public declarations, qualifies each declaration with its module namespace, and preserves annotations attached to declarations that remain in the same file.
