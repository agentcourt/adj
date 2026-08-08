# Lean Proving Guide

This guide is for proving mathematics in Lean, from first contact with tactic mode to custom tactics and packages you may add later.  It is written for this repository's current toolchain, but its scope is deliberately wider than the packages installed here.

Lean changes.  Always prefer the reference manual and current package docs when exact syntax is relevant.  Use this guide as the working manual and the cited sources as the final authority.

## Scope and sources

| Source | Type | What it covers |
| --- | --- | --- |
| [Lean reference manual](https://lean-lang.org/doc/reference/latest/) | Authoritative | The language, elaboration, proof mode, tactics, simplifier, attributes, macros, metaprogramming, and tooling. |
| [Tactic proofs](https://lean-lang.org/doc/reference/latest/Tactic-Proofs/) | Authoritative | Core tactic mode syntax, control flow, cases, induction, rewriting, simplification, and proof structure. |
| [Tactic reference](https://lean-lang.org/doc/reference/latest/Tactic-Proofs/Tactic-Reference/) | Authoritative | Core tactic commands and their exact behavior. |
| [The simplifier](https://lean-lang.org/doc/reference/latest/The-Simplifier/) | Authoritative | How `simp` works, rewrite rule orientation, simp attributes, congruence, and tracing. |
| [Conversion tactics](https://lean-lang.org/doc/reference/latest/Tactic-Proofs/Targeted-Rewriting-with--conv/) | Authoritative | `conv` mode and localized rewriting. |
| [Custom tactics](https://lean-lang.org/doc/reference/latest/Tactic-Proofs/Custom-Tactics/) | Authoritative | Syntax extensions, tactic elaborators, and tactic implementation patterns. |
| [Theorem Proving in Lean 4](https://lean-lang.org/theorem_proving_in_lean4/) | Supporting | The best pedagogical path through term proofs, tactic proofs, induction, rewriting, simplification, and structured proofs. |
| [Metaprogramming in Lean 4](https://leanprover-community.github.io/lean4-metaprogramming-book/) | Supporting | The right guide when you move from using tactics to building tactics. |
| [mathlib docs index](https://leanprover-community.github.io/mathlib4_docs/) | Authoritative for mathlib | Searchable declarations, docstrings, and import paths for mathlib tactics and lemmas. |
| [mathlib package page](https://reservoir.lean-lang.org/@leanprover-community/mathlib) | Authoritative for packaging | Current package metadata, dependency instructions, and source entry points. |
| [Reservoir](https://reservoir.lean-lang.org/) | Authoritative for packages | Package registry for Lean libraries and tactics not currently in this repository. |
| [Duper](https://github.com/leanprover-community/duper) | Primary package source | Example of a substantial external tactic package that is not part of this repository by default. |

The installed tactic surface in this repository comes from three places: core Lean, mathlib, and the packages mathlib pulls in.  The local directories that matter are `Lean/Elab/Tactic`, `Init/Tactics.lean`, and `engine/.lake/packages/mathlib/Mathlib/Tactic`.

## First principles

Lean proofs are terms.  Tactic mode is a term construction language with proof-state feedback.  The most important practical consequence is that the best proof is not the cleverest tactic script.  It is the script that leaves behind the clearest term, uses the smallest amount of automation consistent with robustness, and makes later refactors cheap.

The fastest route through a proof usually has four stages: normalize the goal, expose the structure, solve the routine arithmetic or rewriting, and close the remaining conceptual gap with a short local lemma.  When a tactic script becomes opaque, step back and rewrite the theorem statement or add a named helper lemma.

## Proving workflow

| Situation | First tools | Escalation path |
| --- | --- | --- |
| Definitional equality | `rfl`, `simp`, `simpa` | `change`, `unfold`, `delta`, `conv` |
| One implication or universal quantifier | `intro`, `rintro`, `intros` | `revert`, `generalize`, `rename_i` |
| Case split or data decomposition | `cases`, `rcases`, `obtain`, `constructor` | `induction`, `casesm`, `match` |
| Rewriting by named lemmas | `rw`, `rwa`, `nth_rewrite` | `simp_rw`, `conv`, `erw` |
| Simplification by a stable rewrite base | `simp`, `simpa`, `simp_all` | custom simp lemmas, `dsimp`, simp tracing |
| Linear arithmetic over naturals or integers | `omega`, `linarith` | `nlinarith`, `zify`, local inequalities |
| Polynomial or semiring normalization | `ring_nf`, `ring`, `abel` | `linear_combination`, `polyrith` |
| Field calculations | `field_simp`, `field` | explicit nonzero hypotheses, `ring_nf` |
| Cast-heavy arithmetic | `norm_cast`, `push_cast`, `zify` | `qify`, `rify`, manual coercion lemmas |
| Search for a finishing lemma | `exact?`, `apply?`, `rw?`, `library_search` | `aesop`, `grind`, `hint` |
| Localized rewriting inside a term | `conv` | custom simp sets, manual `calc` chain |
| Proof automation over local hypotheses | `aesop`, `solve_by_elim`, `grind` | explicitly curated rule sets |

## Core tactic language

### Entering and shaping proof mode

| Tactic or construct | Use | Notes |
| --- | --- | --- |
| `by` | Start tactic mode | The proof term is synthesized from the script. |
| `case` | Focus a named goal after `cases` or `induction` | Use it aggressively.  It keeps branch structure explicit. |
| `next` | Start the next branch in term-style tactic blocks | Useful in `match`-like tactic structure. |
| `all_goals` | Run the same script on all goals | Safe when the subgoals are truly uniform. |
| `first \| ...` | Try alternatives in order | Good for small fallback trees. |
| `try` | Attempt a tactic without failure | Use sparingly.  Overuse hides proof structure. |
| `repeat` | Repeat a tactic until failure | Safe only for idempotent or strongly shrinking steps. |
| `<;>` | Apply the right-hand tactic to every new subgoal | Very useful after `constructor`, `cases`, `simp`, and `induction`. |
| `done` | Assert that no goals remain | Good near the end of dense automation. |
| `skip` | Do nothing | Mostly useful in tactic combinators or placeholders. |

### Managing the local context

| Tactic | Use | Notes |
| --- | --- | --- |
| `intro`, `intros` | Introduce hypotheses and binders | Use `intro h` when names matter. |
| `rintro` | Intro plus immediate pattern matching | Usually the best start for conjunctions, existentials, and inductive hypotheses. |
| `have` | Create a local lemma or intermediate fact | Prefer this to repeating a hard subproof inline. |
| `let` | Introduce a local definition | Use when a term is repeated structurally, not propositionally. |
| `suffices` | Replace the goal with a cleaner subgoal | Excellent when the current target is not in the right shape. |
| `show` | Replace the target with a definitionally equal target | Good for clarifying intent before automation. |
| `change` | Force a new target shape by definitional equality | One of the most useful and underused tactics. |
| `specialize` | Apply a hypothesis to arguments | Good when Lean does not infer the intended instantiation cleanly. |
| `generalize` | Replace a subterm with a named variable | Useful before induction or when you need an invariant. |
| `revert` | Move hypotheses back into the goal | Often the right move before induction. |
| `rename_i` | Rename inaccessible or generated variables | Do this early after `cases` or `induction`. |
| `clear`, `clear_except` | Remove irrelevant hypotheses | Use to keep proof states readable. |
| `subst`, `substs` | Substitute equal variables away | Strong cleanup after `cases`, `injection`, or constructor equalities. |

## Introduction, elimination, and structured reasoning

| Goal shape | Main tactics | Technique |
| --- | --- | --- |
| Implication or universal statement | `intro`, `rintro`, `exact` | Name hypotheses well.  Proofs become readable immediately. |
| Conjunction | `constructor`, `refine ⟨_, _⟩` | Use `constructor` if both branches are tactic-heavy. |
| Disjunction | `left`, `right`, `constructor` | Prefer explicit branch choice. |
| Existential | `use`, `refine ⟨w, _⟩` | `use` is often the clearest. |
| Equality of constructors | `rfl`, `constructor`, `cases` | Lean's definitional equality often does more than expected. |
| Sigma, tuple, or record decomposition | `cases`, `rcases`, `obtain` | `rcases h with ⟨a, b, h₁, h₂⟩` is the normal workhorse. |
| Induction on naturals or inductives | `induction`, `cases`, `case` | Revert dependent hypotheses first if the induction hypothesis is too weak. |
| Functional extensionality | `funext`, `ext` | `ext` is usually the better first move for structures and maps. |
| Chain of equalities or inequalities | `calc` | Use `calc` when a proof is primarily a readable algebraic or order chain. |

A good Lean proof often alternates between short tactic bursts and explicit `calc` blocks.  Use tactics to expose structure.  Use `calc` to present the mathematical argument once the structure is visible.

## Rewriting and simplification

### Rewriting

| Tactic | Use | Notes |
| --- | --- | --- |
| `rw` | Rewrite the goal or hypotheses with equalities or iff lemmas | Deterministic and local.  Start here before heavier automation. |
| `rwa` | `rw` followed by `assumption` | Good for short closing moves. |
| `erw` | Like `rw`, but uses a looser reducibility mode | Useful around dependent equalities and casts. |
| `nth_rewrite` | Rewrite only a chosen occurrence | Excellent when `rw` hits the wrong copy of a term. |
| `simp_rw` | Repeat `rw`-style rewriting in a simp loop | Use for systematic theorem-directed rewriting. |
| `conv` | Rewrite inside a selected subexpression | Best tool for under-binder or focused rewriting. |

### Simplification

| Tactic | Use | Notes |
| --- | --- | --- |
| `simp` | Simplify by definitional reduction and `[simp]` rules | The main normalization tactic in Lean. |
| `simpa` | `simp` plus exact comparison with a known term | Often clearer than a manual `have` plus `simp`. |
| `simp_all` | Simplify the target and every hypothesis | Very strong.  Use when the local context is small and coherent. |
| `dsimp` | Definition unfolding without arbitrary rewriting | Good when you want reduction but not theorem-directed simplification. |
| `simp?` | Ask Lean for a suggested `simp` call | Useful for discovering the right simp set. |
| `unfold`, `delta` | Unfold definitions explicitly | Prefer targeted unfolding over global unfolding. |
| `native_decide`, `decide` | Close decidable propositions by computation | Excellent for finite, exact checks. |

The most important `simp` technique is curation.  Add `[simp]` only to lemmas that are canonical, terminating, and broadly correct in either direction.  Keep problem-specific rewrites local by writing `simp [localLemma, defName]`.

### `simp` discipline

| Technique | Reason |
| --- | --- |
| Prefer `simp [defs]` over repeated `rw` when the rewrite base is canonical. | It makes proofs shorter and more stable. |
| Prefer `rw` over `simp` when only one directed rewrite should happen. | It keeps proof intent explicit. |
| Use `simpa using h` when a proof is already present in a slightly different normal form. | It avoids fragile replay of the proof. |
| Inspect simplifier behavior with trace options when `simp` surprises you. | Blindly adding lemmas to `[simp]` is how simp sets become unusable. |

## Equality, congruence, and extensionality

| Tactic | Use | Notes |
| --- | --- | --- |
| `rfl` | Close a goal by reflexivity after reduction | Always try this first on definitional equalities. |
| `congr` | Reduce an equality goal to equality of parts | Good for structured terms. |
| `congr!` | Stronger congruence with additional simplification | Useful, but inspect the resulting goals. |
| `gcongr` | Monotone congruence for inequalities and order goals | One of mathlib's best higher-level tactics. |
| `apply_fun` | Apply a function to both sides of an equality | Good when an injective or monotone map is the right bridge. |
| `funext` | Prove equality of functions pointwise | Standard functional extensionality. |
| `ext` | Apply extensionality lemmas for structures, sets, maps, and more | Often the shortest route to equality of complex objects. |
| `subsingleton` | Use proof irrelevance or subsingleton structure | Good for uniqueness goals. |
| `convert` | Change the target by a nearby equal expression and leave side goals | Powerful, but only if the conversion gap is obvious. |

For set-, function-, and structure-valued goals, `ext` is usually the right first move.  For order goals, `gcongr` often turns a nontrivial monotonicity argument into simpler component goals.

## Induction and recursive proofs

| Technique | When to use it | Main tools |
| --- | --- | --- |
| Structural induction | Data is already in the right inductive form | `induction`, `case`, `simp`, local helper lemmas |
| Strong induction on naturals | The step depends on all smaller cases | `induction'` style patterns, well-founded lemmas, `have` |
| Revert-then-induct | Dependent hypotheses make the induction hypothesis too weak | `revert`, `generalize`, `induction` |
| Functional induction | Proof follows a recursive function's branches | equation lemmas, pattern matching, `simp` |
| Termination proofs | Recursive definitions need a decreasing measure | `termination_by`, `decreasing_by`, `simp_wf`, arithmetic automation |

When induction feels painful, the first suspect is the theorem statement.  Generalize constants that should vary.  Revert hypotheses that depend on the induction variable.  State lemmas at the right level of generality before beginning induction.

## Arithmetic and normalization tactics

| Tactic | Best domain | Strength | Caveat |
| --- | --- | --- | --- |
| `norm_num` | Concrete arithmetic, numerals, small algebraic facts | Fast and reliable | It is not a general symbolic solver. |
| `positivity` | Positivity and nonnegativity goals | Excellent side-condition generator | Needs the expression to be in a recognized form. |
| `omega` | Presburger arithmetic with `+`, `-`, inequalities, divisibility, `%`, `/` in supported forms | Very strong over `Nat` and `Int` | Do not expect it to solve nonlinear ring identities. |
| `linarith` | Linear arithmetic over ordered semirings or rings | Good for inequality closures from hypotheses | Assumes linearity. |
| `nlinarith` | Nonlinear arithmetic after normalization | Strong for polynomial inequalities | Can be slower and more brittle. |
| `ring` | Commutative semiring equality by normalization | Good for exact algebraic identities | Prefer `ring_nf` when you want the normal form visible. |
| `ring_nf` | Normalize both sides of a ring expression | Excellent in the middle of a proof | Stronger as a rewriting step than as a final closer. |
| `abel` | Additive commutative group normalization | Best for additive rearrangements | Not for multiplicative structure. |
| `field_simp` | Clear denominators in field expressions | Standard first step for rational expressions | Requires and generates nonzero side conditions. |
| `field` | Field normalization | Good after side conditions are present | Less transparent than `field_simp` plus `ring_nf`. |
| `linear_combination` | Derive linear or algebraic combinations of equations | Good for hand-guided elimination | Requires a clear target plan. |
| `polyrith` | Polynomial certificate search | Good for polynomial identities and inequalities | Heavier than routine arithmetic. |
| `norm_cast` | Normalize casts in arithmetic expressions | Essential when coercions clutter the goal | Works best if casts are already placed sanely. |
| `push_cast` | Push casts inward | Good preprocessing for arithmetic automation | Often paired with `norm_num` or `ring_nf`. |
| `zify` | Move natural-number arithmetic to integer arithmetic | Very useful before `omega` or `linarith` | Check the generated side conditions. |
| `qify`, `rify` | Move arithmetic into rationals or reals | Useful when field tactics expect those domains | These are preprocessing steps, not finishers. |
| `bound` | Bound expressions using order facts | Useful for inequality proofs with monotonic structure | Needs a useful library of monotonicity lemmas. |

For Collatz-style proofs, the usual arithmetic stack is `rw` or `simp` to expose the branch, `ring_nf` for explicit formulas, and `omega` or `linarith` for the remaining inequalities.  That is often better than reaching for one giant automation step.

## Search and automation

| Tactic | Use | Notes |
| --- | --- | --- |
| `exact?` | Ask Lean for a term that solves the goal | Often finds the right library lemma immediately. |
| `apply?` | Ask Lean for lemmas that can be applied to the goal | Good when you know the shape but not the theorem name. |
| `rw?` | Ask Lean for rewrite candidates | Good for discovering the right normalizing lemma. |
| `library_search` | Search the environment for a theorem that closes the goal | Helpful, but can be noisy on weakly shaped goals. |
| `aesop` | Goal-directed proof search with a rule set | Excellent for structural proof search and local automation. |
| `solve_by_elim` | Finish from local hypotheses and available elimination rules | Lighter and more predictable than `aesop` in many small goals. |
| `grind` | Saturation and simplification style automation in core Lean | Stronger than simple rewriting when the logic is relational and local. |
| `tauto`, `itauto` | Propositional and intuitionistic propositional reasoning | Use when the problem is logical, not algebraic. |
| `hint` | Ask for tactic suggestions | Good for exploration. |

Automation is only good when the goal is already shaped correctly.  If `aesop` or `grind` stalls, the right response is usually to rewrite or split the goal, not to pile on more automation.

## `conv` and targeted rewriting

`conv` is the right tool when you want to rewrite one part of an expression and leave the rest untouched.  It shines under binders, inside nested algebraic expressions, and in places where `rw` would pick the wrong occurrence.

Typical patterns:

```lean
conv =>
  lhs
  simp [Collatz.step]
```

```lean
conv in (3 * (?x) + 1) =>
  rw [someLemma]
```

```lean
conv =>
  enter [1, x]
  simp
```

Use `conv` when you are saying, "the proof is obvious once this exact subterm is normalized."  Do not use it as a replacement for ordinary `rw` or `simp`.

## Diagnostics, debugging, and proof engineering

| Tool or technique | Use | Notes |
| --- | --- | --- |
| `set_option pp.all true` | Show the fully elaborated goal | Useful when coercions or implicit arguments hide the real problem. |
| `show_term` | Display the term produced by a tactic block | Good for understanding automation. |
| `guard_target`, `guard_hyp` | Assert the shape of the goal or a hypothesis | Useful in stable custom tactics and regression-resistant proofs. |
| `extract_goal` | Pull a goal into a standalone theorem skeleton | Very good for debugging large proof states. |
| `try_this` | Accept machine-suggested replacements | Good for simplifying scripts after experimentation. |
| `recover` | Explore proof repair around failing steps | Useful during refactors. |
| `haveI` | Install local instances | Essential when typeclass search needs a local hint. |
| `set_option trace.Meta.Tactic.simp.rewrite true` | Trace simp rewrites | One of the most useful debugging traces. |
| `set_option diagnostics true` | See elaboration diagnostics | Good for instance search and elaboration issues. |
| `min_imports` | Find a smaller import surface | Use after the proof is stable. |

Good proof engineering habits matter more than any one tactic.  Normalize early.  Name nontrivial local lemmas.  Avoid brittle occurrence-specific rewrites when a lemma can encode the normalization cleanly.  Prefer a short custom simp lemma over a long pile of repeated rewrites.

## Custom tactics and metaprogramming

You should move from ordinary tactic scripts to custom tactics only when a proof pattern is repeated often enough to justify an abstraction.  The normal path has three levels.

| Level | Mechanism | Use |
| --- | --- | --- |
| Syntax sugar | `macro_rules` and tactic syntax extensions | Good when an existing tactic sequence deserves a better surface syntax. |
| Tactic elaboration | `elab` on tactic syntax | Good when you need context-sensitive behavior or tactic generation. |
| Meta programming | `Lean.Meta`, `Lean.Elab.Tactic`, `withMainContext`, `getMainGoal` | Necessary for serious automation, proof search, and proof-state transformations. |

Guidelines for custom tactics:

| Rule | Reason |
| --- | --- |
| Build the tactic on a small number of explicit invariants. | Otherwise debugging becomes hopeless. |
| Use `guard_target` and `guard_hyp` in tests. | Tactic regressions are usually shape regressions. |
| Keep fallback behavior predictable. | Silent backtracking makes proofs hard to trust. |
| Separate syntax, elaboration, and meta logic. | This keeps tactics maintainable. |
| Use local helper lemmas before custom tactics. | Many repeated proof patterns are really missing lemmas, not missing automation. |

## Mathlib tactic families

The installed mathlib tactic tree is large.  The useful way to understand it is by family, not by alphabetized file list.

| Family | Representative tactics or modules | Use |
| --- | --- | --- |
| Arithmetic and normalization | `NormNum`, `Ring`, `Field`, `FieldSimp`, `Positivity`, `Linarith`, `LinearCombination`, `Polyrith`, `Zify`, `Qify`, `Rify`, `ModCases`, `IntervalCases`, `ReduceModChar`, `Bound` | Numeric, algebraic, order, and modular arithmetic proofs. |
| Equality and congruence | `ApplyCongr`, `CongrExclamation`, `CongrM`, `GCongr`, `GRewrite`, `Ext`, `ToFun`, `ToLevel`, `TermCongr` | Rewriting, monotonicity, congruence, extensionality, and structured equality proofs. |
| Context and proof management | `ByCases`, `ByContra`, `Change`, `Choose`, `Convert`, `Generalize`, `HaveI`, `Observe`, `Recall`, `RSuffices`, `Set`, `Use`, `WLOG`, `Substs`, `SwapVar`, `ScopedNS`, `Variable` | Shaping the context and goal before heavier tactics. |
| Search and automation | `Aesop`, `Hint`, `Find`, `ExtractGoal`, `TryThis`, `Says`, `Recover`, `Linter`, `TacticAnalysis`, `MinImports` | Search, suggestions, linting, and proof maintenance. |
| Simplification support | `SimpRw`, `SimpIntro`, `SplitIfs`, `Push`, `NormCast`, `CancelDenoms`, `ClearExclamation`, `Clean` | Preprocessing for later automation. |
| Logic and finite structure | `Tauto`, `ITauto`, `Finiteness`, `FinCases`, `CasesM`, `MkIffOfInductiveProp`, `Subsingleton` | Propositional, inductive, and finite combinatorial goals. |
| Domain-specific families | `CategoryTheory/*`, `Continuity`, `Measurability`, `Monotonicity`, `FunProp`, `ContinuousFunctionalCalculus` | Specialist tactics for specific mathematical areas. |

The right way to learn a family is to read its docstring, then inspect two or three proofs that use it well.  mathlib's docs index is the fastest entry point.

## Beyond the current install

This repository currently has core Lean plus mathlib and the packages mathlib pulls in.  That is already a large proving surface.  Still, you may later want tactics that are not part of the present dependency set.

| Package or source | What it adds | When to consider it |
| --- | --- | --- |
| [Reservoir](https://reservoir.lean-lang.org/) | Registry view of add-on packages | Use this to survey available packages before adding anything. |
| [Duper](https://github.com/leanprover-community/duper) | First-order superposition prover | Consider it when goals are first-order, equational, and lemma search is the bottleneck. |
| Package README and source docstrings | Package-specific semantics and examples | Treat these as the primary source for non-core, non-mathlib tactics. |

Rules for evaluating external tactic packages:

| Question | Reason |
| --- | --- |
| Does the package solve a repeated proof bottleneck, or does it only look impressive? | Add packages for repeated proof needs, not novelty. |
| Is the package actively maintained and compatible with the current Lean release? | Version skew is the fastest way to lose time. |
| Are the tactics predictable enough for long-lived proofs? | Heavy automation with unstable output is expensive later. |
| Can the same gain be achieved by adding two or three lemmas instead? | New dependencies should beat simple library work. |

## Core Lean tactic catalog

The following table groups the core tactic modules available in this toolchain.  Some are user-facing tactics.  Some are support modules behind the user-facing surface.  The point of this catalog is to show the whole proving surface, not to recommend every entry equally.

| Group | Core modules |
| --- | --- |
| Basic proof mode and control | `Basic`, `BuiltinTactic`, `Calc`, `Do`, `Lets`, `Repeat`, `Try`, `Show`, `ShowTerm`, `Doc`, `Config`, `ConfigSetter` |
| Rewriting and simplification | `Rewrite`, `Rewrites`, `Simp`, `Simpa`, `SimpArith`, `Simproc`, `Conv`, `Cbv`, `Change`, `Delta`, `Unfold`, `NormCast`, `BoolToPropSimps` |
| Structure and elimination | `RCases`, `Cases`, `Induction`, `Injection`, `Split`, `Match`, `Generalize`, `RenameInaccessibles` |
| Search and automation | `LibrarySearch`, `SolveByElim`, `Grind`, `Omega`, `BVDecide`, `Decide`, `Monotonicity`, `FalseOrByContra` |
| Equality and extensionality | `Congr`, `Rfl`, `Symm`, `Ext`, `AsAuxLemma` |
| Debugging and guards | `Guard`, `ExposeNames`, `Meta`, `TreeTacAttr` |

## mathlib tactic catalog

This is the broad mathlib proving surface you should expect to encounter or consider.  The list is grouped by function, not by file tree.

| Group | Representative mathlib tactics and modules |
| --- | --- |
| Arithmetic and semiring normalization | `Abel`, `Algebraize`, `ArithMult`, `CancelDenoms`, `Field`, `FieldSimp`, `Linarith`, `LinearCombination`, `NormNum`, `NoncommRing`, `Polyrith`, `Positivity`, `Ring`, `Zify`, `Qify`, `Rify`, `ReduceModChar`, `ModCases`, `IntervalCases`, `Bound` |
| Goal shaping and context control | `ApplyAt`, `ApplyCongr`, `ApplyFun`, `ApplyWith`, `ByCases`, `ByContra`, `Change`, `Choose`, `Clean`, `ClearExcept`, `ClearExclamation`, `Clear_`, `Convert`, `DefEqAbuse`, `DefEqTransformations`, `ExtractGoal`, `Generalize`, `HaveI`, `Inhabit`, `Observe`, `Recall`, `Recover`, `Rename`, `RSuffices`, `Set`, `ScopedNS`, `SwapVar`, `UnsetOption`, `Use`, `Variable`, `WLOG` |
| Simplification, rewriting, and congruence | `CongrExclamation`, `CongrM`, `DSimpPercent`, `DepRewrite`, `Ext`, `GCongr`, `GRewrite`, `NthRewrite`, `Push`, `SimpIntro`, `SimpRw`, `SplitIfs`, `Substs`, `TermCongr` |
| Search, automation, and advice | `Aesop`, `Find`, `FindSyntax`, `Hint`, `Linter`, `MinImports`, `Observe`, `Propose`, `Says`, `SuccessIfFailWithMsg`, `TacticAnalysis`, `TryThis` |
| Logic and finite structure | `CasesM`, `Constructor`, `Ext`, `FinCases`, `Finiteness`, `ITauto`, `MkIffOfInductiveProp`, `Subsingleton`, `TFAE`, `Tauto` |
| Domain and specialist families | `CategoryTheory/*`, `Continuity`, `ContinuousFunctionalCalculus`, `FunProp/*`, `Measurability`, `Monotonicity/*`, `Order/*`, `Widget/*` |

## Techniques that matter more than tactic names

| Technique | What it looks like in practice |
| --- | --- |
| Normalize before searching | Rewrite definitions, simplify, and cast-manage before calling `library_search`, `aesop`, or `grind`. |
| State helper lemmas at the right abstraction level | A reusable normalization lemma is usually better than a long tactic script. |
| Prefer `simpa using` to replaying a proof | If the proof exists in a nearby form, normalize the target to it. |
| Separate conceptual steps from arithmetic closure | Do the mathematical step with `have` or `calc`, then finish arithmetic with `omega`, `linarith`, or `ring_nf`. |
| Keep automation local and typed | `aesop` is best when the local context is already well-shaped. |
| Revert dependent hypotheses before induction | This is the standard repair when an induction hypothesis is too weak. |
| Use `conv` for one subterm, not the whole goal | Targeted rewriting is what `conv` is for. |
| Trim imports after the proof stabilizes | Use `min_imports` or local inspection to reduce the future compile and maintenance cost. |

## A short proving playbook

When a new goal appears, work in this order.

1. Read the target and context without touching tactics yet.
2. Decide whether the proof is structural, rewriting-based, arithmetic, extensional, or search-driven.
3. Put the goal in the right shape with `intro`, `rintro`, `cases`, `change`, `rw`, or `simp`.
4. If arithmetic remains, choose the narrowest solver that fits: `norm_num`, `omega`, `linarith`, `ring_nf`, `field_simp`, or `positivity`.
5. If a local abstraction repeats, stop and prove a helper lemma.
6. Only then reach for `aesop`, `grind`, or package-level automation.

If you follow that order, most Lean proofs become short, local, and repairable.  If you skip it, tactic scripts often become long, global, and fragile.
