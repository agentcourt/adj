# Judge Eval Plan

## Scope

The judge eval system measures ten ADC judge actions across nine ARCP rule directories.  Each fixture describes a controlled procedural posture, expected decision fields, accepted reason tags, and a severity weight.  The runner constructs ADC state from that fixture and scores the decision produced by the corresponding production opportunity.

The fixture and prompt assets live under `evals/adc/judge/rules/`, while the Go runners live under `adc/runtime/eval`.  `adc eval` exposes one subcommand per suite and supplies default fixture paths.  Generated `results.jsonl` and `summary.json` files belong under `evals/out/adc/judge/`.

## Execution

For each fixture, the runner validates the row, constructs the ADC state and roles, applies the selected court profile, and asks the Lean engine for the current opportunity.  A model-backed turn renders the judge role, role view, objective, allowed tool, constraints, and tool schema.  It processes the model decision through correction requests, Lean `apply_decision`, and Lean `step`.  A deterministic production turn executes the action supplied by the opportunity through `step` without rendering a role view or calling a model, the correction loop, or `apply_decision`.

Rules 11 and 37 receive model-backed production opportunities.  Rule 58 receives a deterministic judgment-entry action derived from the completed case state.  Its default eval mode executes that action, while `--counterfactual-model` removes it and requests a model decision.  The Rule 58 command accepts an eval-local opportunity prompt only in counterfactual mode.

The other nine suites use the selected model in their default mode.  `--opportunity-prompt-file` replaces the opportunity objective with a validated suite template, and `--opportunity-prompt-name` records its name.  The objective override retains the production role view, allowed tool, constraints, tool schema, and direct-turn execution.

A procedural invalid-attempt or decision-budget failure produces an invalid result that the scorer can count.  Fixture validation, provider setup, prompt rendering, Lean process, and other execution errors abort the run.  Every result row contains `lean_accepted` and `step_accepted`.  On a deterministic production turn, a successful step also sets `lean_accepted` because that path has no `apply_decision` stage.

## Results and Scoring

Each `results.jsonl` row records fixture fields, constructed state, Lean opportunity, response, turn transcript, final state, provider accounting, extracted tool payload, score fields, and execution fields.  Model-backed rows also record the role view, rendered input, and every response exchange, including corrections.  Deterministic production rows have no role view, rendered input, or response exchanges, and encode the opportunity's action and payload in the response fields.

Each `summary.json` records the suite, model, prompt source, file paths, row counts, accuracy, weighted accuracy, invalid rate, and suite-specific errors.  The Rule 58 summary also records whether execution used the production action or a counterfactual model.  The summaries divide results by fields such as tier, issue family, party, reason tag, or expected disposition when the suite defines that division.  A correct substantive label counts as a correct outcome only when the tool payload is valid and Lean accepts its execution.

## Suites

| Rule | Suite | CLI command | Judge tool | Fixtures | Prompt candidates | Default execution |
| --- | --- | --- | --- | ---: | ---: | --- |
| 11 | [Sanctions](rules/rule11/sanctions/README.md) | `judge-rule11` | `decide_rule11_motion` | 16 | 2 | Model |
| 12 | [Dismissal and Jurisdiction](rules/rule12/dismissal-jurisdiction/README.md) | `judge-rule12` | `decide_rule12_motion` | 18 | 2 | Model |
| 37 | [Discovery Sanctions](rules/rule37/discovery-sanctions/README.md) | `judge-rule37` | `decide_rule37_motion` | 16 | 2 | Model |
| 47 | [Voir Dire Question Screening](rules/rule47/voir-dire-question/README.md) | `judge-voir-dire` | `decide_voir_dire_question` | 60 baseline, 30 hard | 3 | Model |
| 47 | [For-Cause Challenges](rules/rule47/for-cause-challenge/README.md) | `judge-for-cause` | `decide_juror_for_cause_challenge` | 16 | 1 | Model |
| 51 | [Jury Instructions](rules/rule51/jury-instructions/README.md) | `judge-rule51` | `settle_jury_instructions` | 16 | 1 | Model |
| 52 | [Bench Opinion](rules/rule52/bench-opinion/README.md) | `judge-rule52` | `file_bench_opinion` | 16 | 1 | Model |
| 56 | [Summary Judgment](rules/rule56/summary-judgment/README.md) | `judge-rule56` | `decide_rule56_motion` | 30 | 2 | Model |
| 58 | [Judgment Entry](rules/rule58/judgment-entry/README.md) | `judge-rule58` | `enter_judgment` | 16 | 1 | Deterministic action |
| 60 | [Relief from Judgment](rules/rule60/relief-from-judgment/README.md) | `judge-rule60` | `resolve_rule60_motion` | 16 | 1 | Model |

The suite README identifies its fixture categories and scorer fields.  Every suite contains `analysis.md`, and nine suite directories contain a local `plan.md`.  Rule 47 voir dire uses this cross-rule plan.  The [Judge Rule Index](rules/README.md) provides the rule-level directory map.

## Prompt Templates

Candidate templates live in the suite's `prompts/` directory.  Each runner exposes replacements for its fixture fields and rejects unknown or unresolved tokens.  The [prompt-authoring guide](../../../docs/prompt-authoring.md#adc-eval-candidate-templates) lists the tokens accepted by every suite.
